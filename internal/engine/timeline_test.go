package engine

import (
	"context"
	"leaguepingo/internal/core"
	"testing"
	"time"
)

type timelineClient struct {
	*pagingClient
	timelineFn func(context.Context, core.MatchDetail, string) (core.MatchTimeline, error)
}

func (c *timelineClient) MatchTimeline(ctx context.Context, d core.MatchDetail, id string) (core.MatchTimeline, error) {
	return c.timelineFn(ctx, d, id)
}
func TestTimelineExplicitCacheAndConnectionIsolation(t *testing.T) {
	fake := &timelineClient{pagingClient: &pagingClient{fakeClient: &fakeClient{}}}
	e := New(nil, nil)
	e.client = fake
	e.snapshot.Participants = fullTestRoster()[:1]
	details, timelines := 0, 0
	fake.detailFn = func(context.Context, int64, string) (core.MatchDetail, error) {
		details++
		return core.MatchDetail{GameID: 42, FetchedAt: time.Now()}, nil
	}
	fake.timelineFn = func(_ context.Context, d core.MatchDetail, target string) (core.MatchTimeline, error) {
		timelines++
		if d.GameID != 42 || target == "" {
			t.Fatal("unverified detail passed to timeline")
		}
		return core.MatchTimeline{GameID: 42, FetchedAt: time.Now()}, nil
	}
	for i := 0; i < 2; i++ {
		result, err := e.MatchTimeline(context.Background(), "player-0", 0, 42)
		if err != nil || result.Detail == nil || result.Detail.GameID != 42 {
			t.Fatal("timeline failed", err)
		}
	}
	if details != 1 || timelines != 1 {
		t.Fatal("cached timeline or verified detail was fetched again", details, timelines)
	}
	if _, err := e.MatchTimeline(context.Background(), "player-0", 99, 42); err == nil || timelines != 1 {
		t.Fatal("old connection reached source")
	}
	fake.detailFn = func(context.Context, int64, string) (core.MatchDetail, error) {
		return core.MatchDetail{GameID: 43, FetchedAt: time.Now()}, nil
	}
	fake.timelineFn = func(context.Context, core.MatchDetail, string) (core.MatchTimeline, error) {
		e.mu.Lock()
		e.invalidateConnectionLocked()
		e.mu.Unlock()
		return core.MatchTimeline{GameID: 43, FetchedAt: time.Now()}, nil
	}
	if _, err := e.MatchTimeline(context.Background(), "player-0", 0, 43); err == nil || cacheCountsForTest(e).timelines != 0 || cacheCountsForTest(e).details != 0 {
		t.Fatal("late timeline crossed connection change")
	}
}
func TestMasteryFreshnessSurvivesPartialProfileFailure(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	old := mergeProfile(core.Profile{Masteries: []core.ChampionMastery{{ChampionID: 23, Level: 10, Points: 1000}}, MasteryFetchedAt: &at}, nil)
	next := mergeProfile(core.Profile{}, &old)
	if len(next.Masteries) != 1 || next.Fields["mastery"].State != "retained" || !next.Fields["mastery"].FetchedAt.Equal(at) {
		t.Fatal("failed optional data discarded mastery or reset age")
	}
	empty := mergeProfile(core.Profile{Masteries: []core.ChampionMastery{}, MasteryFetchedAt: &at}, &old)
	if empty.Masteries == nil || len(empty.Masteries) != 0 || empty.Fields["mastery"].State != "ready" {
		t.Fatal("known-empty mastery could not replace old values")
	}
}
