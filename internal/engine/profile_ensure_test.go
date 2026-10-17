package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type basicProfileClient struct {
	*profileClient
	basic   core.Profile
	onBasic func()
}

func (c *basicProfileClient) BasicProfile(context.Context, string) core.Profile {
	c.record("basic-profile")
	if c.onBasic != nil {
		c.onBasic()
	}
	return c.basic
}

func profileEnsureFixture() (*Engine, *basicProfileClient, core.Participant) {
	level, icon := 100, 123
	c := &basicProfileClient{profileClient: &profileClient{fakeClient: &fakeClient{}}, basic: core.Profile{Level: &level, ProfileIconID: &icon, ProfileIconData: "synthetic-avatar", PrivacyState: "public", RankState: "unavailable"}}
	e := New(nil, nil)
	e.client = c
	p := core.Participant{Key: "manual", PUUID: "manual", DisplayName: "Manual#TEST", IdentityState: core.IdentityKnown}
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 42}}})
	e.cachePlayerLocked(p)
	e.snapshot.Participants = []core.Participant{p}
	return e, c, p
}

func TestCachedManualLookupsEnrichWithoutIdentityHistoryOrRank(t *testing.T) {
	for _, kind := range []string{"id", "player", "after-resolution"} {
		t.Run(kind, func(t *testing.T) {
			e, c, p := profileEnsureFixture()
			var got core.Participant
			var err error
			switch kind {
			case "id":
				got, err = e.Lookup(context.Background(), "Manual#TEST")
			case "player":
				got, err = e.LookupPlayer(context.Background(), p.Key)
			case "after-resolution":
				got, err = e.loadManualHistory(context.Background(), c, 0, 0, p, false, true)
			}
			if err != nil || got.Profile == nil || got.Profile.ProfileIconData != "synthetic-avatar" || len(c.calls) != 1 || c.calls[0] != "basic-profile" || !got.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) {
				t.Fatal("cached lookup did not enrich independently", err)
			}
			if e.Snapshot().Participants[0].Profile == nil {
				t.Fatal("profile not shared with roster")
			}
			if _, err = e.EnsureProfile(context.Background(), p.Key, 0); err != nil || len(c.calls) != 1 {
				t.Fatal("complete profile reloaded")
			}
		})
	}
}

func TestEnsureProfileFailureCooldownAndExplicitRetry(t *testing.T) {
	e, c, p := profileEnsureFixture()
	c.basic = core.Profile{RankState: "unavailable", PrivacyState: "unknown"}
	if _, err := e.EnsureProfile(context.Background(), p.Key, 0); err == nil {
		t.Fatal("missing profile treated as success")
	}
	for i := 0; i < 5; i++ {
		got, err := e.Lookup(context.Background(), "Manual#TEST")
		if err != nil || len(got.History) != 1 {
			t.Fatal("optional failure lost cached history")
		}
	}
	if len(c.calls) != 1 {
		t.Fatal("failure cooldown ineffective")
	}
	c.profile = c.basic
	_, _ = e.LoadProfile(context.Background(), p.Key, 0)
	if len(c.calls) != 2 || c.calls[1] != "profile" {
		t.Fatal("explicit retry should bypass cooldown")
	}
	e.profileRetry[p.Key] = time.Now().Add(-time.Second)
	_, _ = e.EnsureProfile(context.Background(), p.Key, 0)
	if len(c.calls) != 3 {
		t.Fatal("cooldown never expired")
	}
}

func TestEnsureProfileRejectsLateConnectionAndPreservesNewHistory(t *testing.T) {
	e, c, p := profileEnsureFixture()
	c.onBasic = func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.cache[p.Key]
		core.ApplyHistory(&current, core.HistoryResult{Matches: []core.Match{{GameID: 43}}})
		e.cache[p.Key] = current
	}
	got, err := e.EnsureProfile(context.Background(), p.Key, 0)
	if err != nil || got.History[0].GameID != 43 || e.cache[p.Key].History[0].GameID != 43 {
		t.Fatal("profile overwrote newer history")
	}
	e, c, p = profileEnsureFixture()
	c.onBasic = func() { e.mu.Lock(); e.invalidateConnectionLocked(); e.mu.Unlock() }
	if _, err = e.EnsureProfile(context.Background(), p.Key, 0); err == nil || len(e.cache) != 0 || len(e.profileRetry) != 0 {
		t.Fatal("old profile/cooldown leaked into new connection")
	}
}

func TestEnsureProfileFromRosterOnlyAndManualGate(t *testing.T) {
	e, c, p := profileEnsureFixture()
	e.cache = map[string]core.Participant{}
	_, release, _ := e.beginLookup(context.Background())
	if _, err := e.EnsureProfile(context.Background(), p.Key, 0); err == nil || len(c.calls) != 0 {
		t.Fatal("parallel operation bypassed gate")
	}
	release()
	if _, err := e.EnsureProfile(context.Background(), p.Key, 0); err != nil || e.cache[p.Key].Profile == nil {
		t.Fatal("roster-only profile not cached", err)
	}
	if _, err := e.EnsureProfile(context.Background(), p.Key, 1); err == nil || len(c.calls) != 1 {
		t.Fatal("stale connection issued request")
	}
}

func TestEnsureProfileWaitingForSchedulerRejectsChangedConnection(t *testing.T) {
	e, c, p := profileEnsureFixture()
	release, err := e.scheduler.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := e.EnsureProfile(context.Background(), p.Key, 0); done <- err }()
	deadline := time.Now().Add(time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		e.scheduler.mu.Lock()
		waiting = e.scheduler.manual > 0
		e.scheduler.mu.Unlock()
		if waiting {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !waiting {
		release()
		t.Fatal("profile did not wait for scheduler")
	}
	e.mu.Lock()
	e.invalidateConnectionLocked()
	e.mu.Unlock()
	release()
	select {
	case err = <-done:
	case <-time.After(time.Second):
		t.Fatal("profile request did not stop")
	}
	if err == nil || len(c.calls) != 0 {
		t.Fatal("queued old connection issued profile request")
	}
}
