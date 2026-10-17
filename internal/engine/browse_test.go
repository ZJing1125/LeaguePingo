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
	if got, err := e.LoadProfile(context.Background(), p.Key, 0); err != nil || got.PrivacyState != "private" || got.Profile.Fields["privacy"].State != "retained" {
		t.Fatal("failed profile request discarded known privacy or labeled it fresh")
	}
}

func TestPartialProfileUpdatePreservesFieldAgeWithoutReplacingHistory(t *testing.T) {
	level := 100
	old := mergeProfile(core.Profile{Level: &level, RankState: "ready", PrivacyState: "private", Ranks: []core.Rank{{Tier: "GOLD"}}}, nil)
	fake := &profileClient{fakeClient: &fakeClient{}, profile: core.Profile{Level: &level, RankState: "unavailable", PrivacyState: "unknown"}}
	e := New(nil, nil)
	e.client = fake
	p := fullTestRoster()[0]
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 99}}})
	p.Profile = &old
	e.cache[p.Key] = p
	got, err := e.LoadProfile(context.Background(), p.Key, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Fields["level"].State != "ready" || got.Profile.Fields["rank"].State != "retained" || got.Profile.Fields["privacy"].State != "retained" {
		t.Fatal("partial failure not exposed per field")
	}
	if !got.Profile.Fields["rank"].FetchedAt.Equal(*old.Fields["rank"].FetchedAt) || !got.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) {
		t.Fatal("profile retry changed original field/history timestamp")
	}
	fake.profile = core.Profile{RankState: "ready", PrivacyState: "public", Ranks: []core.Rank{{Tier: "NONE"}}}
	got, err = e.LoadProfile(context.Background(), p.Key, 0)
	if err != nil || got.Profile.Fields["privacy"].State != "ready" || got.PrivacyState != "public" || got.Profile.Ranks[0].Tier != "NONE" {
		t.Fatal("explicit new state not replacing retained state")
	}
}

func TestProfileIconChangeCannotRetainPreviousPlayersPicture(t *testing.T) {
	a, b := 123, 456
	old := mergeProfile(core.Profile{ProfileIconID: &a, ProfileIconData: "old-picture"}, nil)
	retained := mergeProfile(core.Profile{}, &old)
	if retained.ProfileIconData != "old-picture" || retained.Fields["avatar"].State != "retained" {
		t.Fatal("same icon was not retained honestly")
	}
	changed := mergeProfile(core.Profile{ProfileIconID: &b}, &old)
	if changed.ProfileIconData != "" || changed.Fields["avatar"].State != "unavailable" {
		t.Fatal("old picture mislabeled as new icon")
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
