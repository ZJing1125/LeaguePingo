package source

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

// This explicit test is the candidate verifier. Production keeps mixed-history
// queries until region-specific continuous-pagination evidence is available.
func TestLiveQueueHistoryAudit(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_QUEUE_AUDIT") != "1" {
		t.Skip("explicit Windows self-account queue audit")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal("local client unavailable")
	}
	s := &TencentLCU{client: c}
	defer c.HTTP.CloseIdleConnections()
	defer s.ResetIdentity()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	self, err := s.currentPUUID(ctx)
	if err != nil || !core.IsKnownPUUID(self) {
		t.Fatal("self identity unavailable")
	}
	guard := func() {
		phase, e := s.Phase(ctx)
		if e != nil {
			t.Fatal("phase unavailable")
		}
		switch phase {
		case "None", "Lobby", "EndOfGame", "PreEndOfGame", "WaitingForStats":
		default:
			t.Fatal("audit requires idle, lobby or postgame")
		}
		current, e := s.AccountIdentity(ctx)
		if e != nil || current != self {
			t.Fatal("account changed")
		}
	}
	read := func(offset int, filtered bool) core.HistoryResult {
		guard()
		session, e := s.sgpSession(ctx, false, &s.channels.history)
		if e != nil {
			t.Fatal("session unavailable")
		}
		values := url.Values{"startIndex": {fmt.Sprint(offset)}, "count": {"20"}}
		if filtered {
			values.Set("tag", "q_2400")
		}
		path := "/match-history-query/v1/products/lol/player/" + url.PathEscape(self) + "/SUMMARY?" + values.Encode()
		raw, e := s.getSGPAuthorized(ctx, session, path, "/sgp-history", &s.channels.history)
		if e != nil {
			t.Fatal("history request unavailable")
		}
		result, e := core.ParseSGPHistory(raw, self, session.platform)
		if e != nil || result.Meta.Returned > 20 || result.Meta.Excluded > 0 {
			t.Fatal("history contract invalid")
		}
		guard()
		return result
	}
	var mixed, tagged []core.Match
	for offset := 0; offset < 60; offset += 20 {
		page := read(offset, false)
		mixed = append(mixed, page.Matches...)
		if page.Meta.Returned < 20 {
			break
		}
	}
	for offset := 0; offset < 40; offset += 20 {
		page := read(offset, true)
		tagged = append(tagged, page.Matches...)
		if page.Meta.Returned < 20 {
			break
		}
	}
	check := read(0, false)
	if len(mixed) == 0 || len(check.Matches) == 0 || mixed[0].GameID != check.Matches[0].GameID {
		t.Fatal("history changed during comparison; inconclusive")
	}
	expected := []int64{}
	for _, m := range mixed {
		if m.QueueID == 2400 {
			expected = append(expected, m.GameID)
		}
	}
	if len(expected) < 21 {
		t.Log("QUEUE_AUDIT inconclusive insufficient mixed-history Mayhem samples for two-page comparison")
		return
	}
	n := min(len(expected), 40)
	if len(tagged) < n {
		t.Fatal("filtered history missed expected matches")
	}
	seen := map[int64]bool{}
	for i, m := range tagged {
		if m.QueueID != 2400 || seen[m.GameID] {
			t.Fatal("wrong queue or repeated page")
		}
		seen[m.GameID] = true
		if i < n && expected[i] != m.GameID {
			t.Fatal("filtered history differs from mixed-history reference")
		}
	}
	t.Logf("QUEUE_AUDIT queue=2400 mixed=%d filtered=%d compared=%d pagination=verified current-region-only", len(mixed), len(tagged), n)
}
