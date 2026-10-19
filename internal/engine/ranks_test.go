package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type rankClient struct {
	*basicProfileClient
	rank   core.Profile
	onRank func(context.Context)
}

func (c *rankClient) Ranks(ctx context.Context, _ string) core.Profile {
	c.record("rank")
	if c.onRank != nil {
		c.onRank(ctx)
	}
	return c.rank
}

func TestRankRetryPreservesHistoryAndUntouchedProfileFreshness(t *testing.T) {
	e, b, p := profileEnsureFixture()
	got, err := e.EnsureProfile(context.Background(), p.Key, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.RankState != "unavailable" {
		t.Fatal("full profile did not preserve source rank state")
	}
	before := got.Profile.Fields["level"]
	c := &rankClient{basicProfileClient: b, rank: core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "GOLD"}}}}
	e.client = c
	c.onRank = func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second {
			t.Fatal("rank retry lacks independent budget")
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.cache[p.Key]
		core.ApplyHistory(&current, core.HistoryResult{Matches: []core.Match{{GameID: 77}}})
		e.cache[p.Key] = current
	}
	got, err = e.LoadRanks(context.Background(), p.Key, 0)
	if err != nil || got.History[0].GameID != 77 || got.Profile.Level == nil || got.Profile.ProfileIconData != "synthetic-avatar" {
		t.Fatal("rank load replaced history/basic data", err)
	}
	if got.Profile.Fields["level"] != before || got.Profile.Fields["rank"].State != "ready" {
		t.Fatal("rank load changed unrelated freshness")
	}
	if len(c.calls) != 2 || c.calls[1] != "rank" || e.Snapshot().Participants[0].Profile.RankState != "ready" {
		t.Fatal("rank retry fetched unrelated data or missed roster")
	}
	rankTime := got.Profile.Fields["rank"]
	// A subsequent basic-only operation must not age a rank it never requested.
	merged := mergeProfile(b.basic, got.Profile, "level", "icon", "avatar", "privacy")
	if merged.Fields["rank"] != rankTime || merged.RankState != "ready" {
		t.Fatal("opening details aged rank")
	}
}

func TestRankFailureStateAndExplicitRetryRetainLastVerifiedValue(t *testing.T) {
	e, b, p := profileEnsureFixture()
	c := &rankClient{basicProfileClient: b, rank: core.Profile{RankState: "failed", RankMessage: "段位查询超时，可重试"}}
	e.client = c
	got, err := e.LoadRanks(context.Background(), p.Key, 0)
	if err != nil || got.Profile.RankState != "failed" || got.Profile.Fields["rank"].State != "failed" || len(got.History) != 1 {
		t.Fatal("failure state lost", err)
	}
	c.rank = core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "GOLD"}}}
	got, err = e.LoadRanks(context.Background(), p.Key, 0)
	if err != nil || got.Profile.RankState != "ready" {
		t.Fatal("explicit retry did not recover", err)
	}
	verified := got.Profile.Fields["rank"].FetchedAt
	c.rank = core.Profile{RankState: "failed", RankMessage: "段位查询超时，可重试"}
	got, err = e.LoadRanks(context.Background(), p.Key, 0)
	if err != nil || got.Profile.Ranks[0].Tier != "GOLD" || got.Profile.Fields["rank"].State != "retained" || got.Profile.Fields["rank"].FetchedAt != verified || got.Profile.RankMessage == "" {
		t.Fatal("failed refresh discarded old rank or its age", err)
	}
	if len(c.calls) != 3 {
		t.Fatal("unexpected automatic retry")
	}
}

func TestRankReplyAndManualGateRejectOldConnection(t *testing.T) {
	e, b, p := profileEnsureFixture()
	c := &rankClient{basicProfileClient: b, rank: core.Profile{RankState: "ready"}}
	e.client = c
	_, done, _ := e.beginLookup(context.Background())
	if _, err := e.LoadRanks(context.Background(), p.Key, 0); err == nil || len(c.calls) != 0 {
		t.Fatal("rank request bypassed manual gate")
	}
	done()
	if _, err := e.LoadRanks(context.Background(), p.Key, 1); err == nil || len(c.calls) != 0 {
		t.Fatal("stale identity triggered request")
	}
	c.onRank = func(context.Context) { e.mu.Lock(); defer e.mu.Unlock(); e.invalidateConnectionLocked() }
	if _, err := e.LoadRanks(context.Background(), p.Key, 0); err == nil || len(e.cache) != 0 {
		t.Fatal("late rank crossed connections")
	}
}

func TestRankWaitingForSchedulerRejectsChangedConnection(t *testing.T) {
	e, b, p := profileEnsureFixture()
	c := &rankClient{basicProfileClient: b, rank: core.Profile{RankState: "ready"}}
	e.client = c
	release, err := e.scheduler.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := e.LoadRanks(context.Background(), p.Key, 0); result <- err }()
	waiting := false
	deadline := time.Now().Add(time.Second)
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
		t.Fatal("rank did not wait for scheduler")
	}
	e.mu.Lock()
	e.invalidateConnectionLocked()
	e.mu.Unlock()
	release()
	select {
	case err = <-result:
	case <-time.After(time.Second):
		t.Fatal("queued rank did not stop")
	}
	if err == nil || len(c.calls) != 0 {
		t.Fatal("queued rank crossed connections")
	}
}
