package source

import (
	"context"
	"encoding/json"
	"io"
	"leaguepingo/internal/core"
	"os"
	"runtime"
	"testing"
	"time"
)

// Explicit local audit: only anonymous slot/group labels, never names, IDs,
// credentials, addresses or raw responses. Compare against known party rosters.
func TestLiveCurrentGrouping(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_GROUP_AUDIT") != "1" {
		t.Skip("explicit Windows current-match audit")
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Fatal("requires Windows amd64 game computer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	source, err := DiscoverTencent()
	if err != nil {
		t.Fatal("client unavailable")
	}
	s := source.(*TencentLCU)
	phase, err := s.Phase(ctx)
	if err != nil || (phase != "GameStart" && phase != "InProgress") {
		t.Fatal("run during loading or gameplay")
	}
	id, local, err := s.Roster(ctx)
	if err != nil || id <= 0 {
		t.Fatal("current match not confirmed")
	}
	meta := s.SessionMetadata()
	mode := "other"
	switch meta.GameMode {
	case "ARAM", "ARAM_MAYHEM", "CLASSIC":
		mode = meta.GameMode
	}
	t.Logf("queue=%d map=%d mode=%s", meta.QueueID, meta.MapID, mode)
	extra, remoteErr := s.Grouping(ctx, id)
	if remoteErr != nil {
		t.Log("SGP grouping unavailable; local fields only")
	}
	verifiedPhase, e := s.Phase(ctx)
	if e != nil || (verifiedPhase != "GameStart" && verifiedPhase != "InProgress") {
		t.Fatal("phase changed; discard audit")
	}
	previous := s.CachedAccountIdentity()
	self, e := s.AccountIdentity(ctx)
	if e != nil || self != previous {
		t.Fatal("account changed; discard audit")
	}
	verifyID, _, verifyErr := s.Roster(ctx)
	if verifyErr != nil || verifyID != id {
		t.Fatal("match changed; discard audit")
	}
	for _, item := range []struct {
		name string
		ps   int
	}{{"LCU", len(local)}, {"SGP", len(extra)}} {
		t.Logf("%s participants=%d", item.name, item.ps)
	}
	groups := map[string]int{}
	for i, p := range local {
		g := 0
		if p.PremadeGroup != "" {
			if groups[p.PremadeGroup] == 0 {
				groups[p.PremadeGroup] = len(groups) + 1
			}
			g = groups[p.PremadeGroup]
		}
		t.Logf("LCU slot=%d side=%s field=%t group=%d count=%d", i+1, p.Team, p.GroupToken != "", g, p.PremadeSize)
	}
	groups = map[string]int{}
	for i, p := range extra {
		g := 0
		if p.PremadeGroup != "" {
			if groups[p.PremadeGroup] == 0 {
				groups[p.PremadeGroup] = len(groups) + 1
			}
			g = groups[p.PremadeGroup]
		}
		t.Logf("SGP slot=%d side=%s field=%t group=%d count=%d", i+1, p.Team, p.GroupToken != "", g, p.PremadeSize)
	}
	t.Log("STRUCTURE ONLY: verify both teams against known matchmaking party rosters; absence does not mean solo.")
	if path := os.Getenv("LEAGUEPINGO_GROUP_EXPECTED"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal("expected slot file unavailable")
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, 4097))
		if err != nil || len(raw) > 4096 {
			t.Fatal("expected slot file exceeds bounds")
		}
		var expected struct {
			QueueID int   `json:"queueId"`
			Groups  []int `json:"groups"`
		}
		if json.Unmarshal(raw, &expected) != nil || expected.QueueID != meta.QueueID {
			t.Fatal("expected queue differs; discard comparison")
		}
		merged, err := core.MergeGroupFields(local, extra)
		if err != nil {
			t.Fatal("source conflict; comparison rejected")
		}
		result, err := core.CompareGroupSlots(merged, expected.Groups)
		if err != nil {
			t.Fatal("invalid ten-slot baseline")
		}
		t.Logf("COMPARISON known=%d/10 pairs=%d together=%d wrong_merge=%d wrong_split=%d unknown_pairs=%d", result.KnownPlayers, result.ComparedPairs, result.CorrectTogether, result.WrongMerge, result.WrongSplit, result.UnknownPairs)
		if result.KnownPlayers != 10 || result.WrongMerge != 0 || result.WrongSplit != 0 || result.UnknownPairs != 0 {
			t.Fatal("NOT VERIFIED: incomplete or incorrect partition")
		}
		t.Log("MATCHED supplied baseline for this match only; baseline authenticity and other matches/modes remain unverified.")
	}
}
