package source

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

// Explicit opt-in, never record live player names, IDs, raw bodies or tokens.
// An explicit developer action may check manual lookup in or out of a match.
func TestLiveManualLookup(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_LOOKUP_TEST") != "1" {
		t.Skip("opt in with LEAGUEPINGO_LIVE_LOOKUP_TEST=1 for one self-account lookup")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal(err)
	}
	defer c.HTTP.CloseIdleConnections()
	s := &TencentLCU{client: c}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	checkPhase := func() {
		t.Helper()
		phase, err := s.Phase(ctx)
		if err != nil {
			t.Fatal("phase check failed")
		}
		if phase == "" {
			t.Fatal("phase unavailable")
		}
	}
	checkPhase()
	raw, err := c.Get(ctx, "/lol-summoner/v1/current-summoner")
	if err != nil {
		t.Fatal("self-profile unavailable")
	}
	var self struct {
		PUUID    string `json:"puuid"`
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}
	if json.Unmarshal(raw, &self) != nil || self.PUUID == "" || self.GameName == "" || self.TagLine == "" {
		t.Fatal("self-profile incomplete (redacted)")
	}
	checkPhase()
	player, err := s.FindByRiotID(ctx, self.GameName+"#"+self.TagLine)
	if err != nil {
		t.Fatal("complete-ID lookup failed (details redacted)")
	}
	if player.PUUID != self.PUUID {
		t.Fatal("lookup returned a different account (redacted)")
	}
	checkPhase()
	history, err := s.History(ctx, player.PUUID)
	if err != nil {
		t.Fatal("history lookup failed (details redacted)")
	}
	t.Logf("live complete-ID lookup matched self-account; history parsed, games=%d (identities redacted)", len(history))
}

func TestLiveRoster(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_ROSTER_TEST") != "1" {
		t.Skip("opt in with LEAGUEPINGO_LIVE_ROSTER_TEST=1")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal(err)
	}
	defer c.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	s := &TencentLCU{client: c}
	phase, err := s.Phase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if phase != "InProgress" && phase != "GameStart" && phase != "Reconnect" {
		t.Skip("not in an active match")
	}
	_, roster, err := s.Roster(ctx)
	if err != nil {
		t.Fatal("local roster unavailable (redacted)")
	}
	allies, enemies, known := 0, 0, 0
	for _, p := range roster {
		if p.Team == "ally" {
			allies++
		} else {
			enemies++
		}
		if p.IdentityState == "known" {
			known++
		}
	}
	if allies != 5 || enemies != 5 || known != 10 {
		t.Fatalf("roster counts: allies=%d enemies=%d known=%d", allies, enemies, known)
	}
	t.Logf("live local roster: allies=%d enemies=%d valid identities=%d; no history requested", allies, enemies, known)
}
