package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type pagingClient struct {
	*fakeClient
	pageFn   func(context.Context, string, int) (core.HistoryPage, error)
	detailFn func(context.Context, int64, string) (core.MatchDetail, error)
}

type profileClient struct {
	*fakeClient
	profile core.Profile
}

func (p *profileClient) Profile(context.Context, string) core.Profile {
	p.record("profile")
	return p.profile
}

func TestProfileOnlyUpdateUsesCachedPlayerWithoutHistoryRefresh(t *testing.T) {
	fake := &profileClient{fakeClient: &fakeClient{}, profile: core.Profile{RankState: "unavailable", PrivacyState: "private"}}
	e := New(nil, nil)
	e.client = fake
	p := fullTestRoster()[0]
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 99}}})
	e.cache[p.Key] = p
	got, err := e.LoadProfile(context.Background(), p.Key, 0)
	if err != nil || got.PrivacyState != "private" || len(got.History) != 1 || len(fake.calls) != 1 || fake.calls[0] != "profile" {
		t.Fatal("profile-only update fetched or lost history")
	}
	fake.profile = core.Profile{RankState: "unavailable", PrivacyState: "unknown"}
	if _, err := e.LoadProfile(context.Background(), p.Key, 0); err == nil || e.cache[p.Key].PrivacyState != "private" {
		t.Fatal("failed profile request discarded known privacy")
	}
}

func TestHistoryRefreshRetainsPreviouslyVerifiedProfileOnOptionalFailure(t *testing.T) {
	fake := &profileClient{fakeClient: &fakeClient{}, profile: core.Profile{RankState: "unavailable", PrivacyState: "unknown"}}
	e := New(nil, nil)
	e.client = fake
	level := 100
	e.cache["manual"] = core.Participant{Key: "manual", PUUID: "manual", Profile: &core.Profile{Level: &level, PrivacyState: "private", RankState: "ready", Ranks: []core.Rank{{Tier: "GOLD"}}}}
	p, err := e.RefreshLookup(context.Background(), "Manual#TEST")
	if err != nil || p.Profile == nil || p.Profile.Level == nil || *p.Profile.Level != 100 || p.PrivacyState != "private" || len(p.Profile.Ranks) != 1 {
		t.Fatal("optional failure erased verified profile")
	}
}

func (p *pagingClient) MatchDetail(ctx context.Context, gameID int64, target string) (core.MatchDetail, error) {
	return p.detailFn(ctx, gameID, target)
}

func TestMatchDetailCachedAndIsolatedFromConnectionChanges(t *testing.T) {
	fake := &pagingClient{fakeClient: &fakeClient{}}
	e := New(nil, nil)
	e.client = fake
	e.snapshot.Participants = fullTestRoster()[:1]
	calls := 0
	fake.detailFn = func(context.Context, int64, string) (core.MatchDetail, error) {
		calls++
		return core.MatchDetail{GameID: 42, FetchedAt: time.Now()}, nil
	}
	for i := 0; i < 2; i++ {
		if _, err := e.MatchDetail(context.Background(), "player-0", 0, 42); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("completed detail fetched repeatedly")
	}
	fake.detailFn = func(context.Context, int64, string) (core.MatchDetail, error) {
		e.mu.Lock()
		e.invalidateConnectionLocked()
		e.mu.Unlock()
		return core.MatchDetail{GameID: 43}, nil
	}
	if _, err := e.MatchDetail(context.Background(), "player-0", 0, 43); err == nil || len(e.details) != 0 {
		t.Fatal("old detail entered new connection")
	}
}

func (p *pagingClient) HistoryPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	return p.pageFn(ctx, id, offset)
}

func TestPagingRejectsConnectionChangesAndSharesManualGate(t *testing.T) {
	fake := &pagingClient{fakeClient: &fakeClient{}}
	e := New(nil, nil)
	e.client = fake
	e.snapshot.Participants = fullTestRoster()[:1]
	calls := 0
	fake.pageFn = func(context.Context, string, int) (core.HistoryPage, error) {
		calls++
		e.mu.Lock()
		e.invalidateConnectionLocked()
		e.mu.Unlock()
		return core.HistoryPage{Matches: []core.Match{{GameID: 99}}}, nil
	}
	if _, err := e.HistoryPage(context.Background(), "player-0", 0, 20); err == nil {
		t.Fatal("old connection page accepted")
	}
	if _, err := e.HistoryPage(context.Background(), "player-0", 0, 20); err == nil || calls != 1 {
		t.Fatal("stale player reached source")
	}
	_, release, _ := e.beginLookup(context.Background())
	defer release()
	if _, err := e.HistoryPage(context.Background(), "player-0", 1, 20); err == nil {
		t.Fatal("parallel manual browse admitted")
	}
}
