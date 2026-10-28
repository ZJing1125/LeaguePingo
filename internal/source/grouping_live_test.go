package source

import (
	"context"
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
}
