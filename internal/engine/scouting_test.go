package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type scoutClient struct {
	*fakeClient
	pages    atomic.Int32
	now      time.Time
	repeat   bool
	endEarly bool
}

func (s *scoutClient) CurrentQueue() int { return 440 }
func (s *scoutClient) ScoutSeason(context.Context) (*core.ScoutSeason, error) {
	return &core.ScoutSeason{Name: "test", Start: s.now.Add(-100 * 24 * time.Hour), End: s.now.Add(100 * 24 * time.Hour)}, nil
}
func (s *scoutClient) ChampionMasteries(context.Context, string) ([]core.ChampionMastery, error) {
	return []core.ChampionMastery{{ChampionID: 23, Level: 31, Points: 700000}}, nil
}
func (s *scoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	s.pages.Add(1)
	matches := []core.Match{}
	for i := 0; i < 20; i++ {
		n := offset + i
		if s.repeat {
			n = i
		}
		at := s.now.Add(-time.Duration(n)*24*time.Hour - time.Hour)
		matches = append(matches, core.Match{GameID: int64(n + 1), ChampionID: 23, QueueID: 420, PlayedAt: &at})
	}
	return core.HistoryPage{Offset: offset, NextOffset: offset + 20, Matches: matches, Meta: core.HistoryMeta{Returned: 20, Parsed: 20}, HasMore: !s.endEarly}, nil
}
func startScoutTest(t *testing.T, f *scoutClient) *Engine {
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown, ChampionID: 23, History: []core.Match{{GameID: 999999}}}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	return e
}
func waitScout(t *testing.T, e *Engine) *core.ScoutReport {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		s := e.Snapshot()
		if len(s.Participants) > 0 && s.Participants[0].Scout != nil && s.Participants[0].Scout.State != "loading" {
			return s.Participants[0].Scout
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scouting did not finish")
	return nil
}
func TestScoutingIndependentSeasonPagingAndModeSwitchDoesNotQuery(t *testing.T) {
	f := &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}
	e := startScoutTest(t, f)
	r := waitScout(t, e)
	if !r.Complete || !r.Complete30 || r.Read != 120 || r.Champions[0].Games == nil || *r.Champions[0].Games != 100 || len(e.Snapshot().Participants[0].History) != 1 {
		t.Fatal("season report capped at list sample or browsing history overwritten")
	}
	if e.Snapshot().ScoutQueue != 440 {
		t.Fatal("ranked queue default did not follow session")
	}
	before := f.pages.Load()
	if _, err := e.SetScoutQueue(420); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		e.mu.Lock()
		e.syncScoutingLocked()
		e.mu.Unlock()
	}
	if e.Snapshot().ScoutQueue != 420 || f.pages.Load() != before {
		t.Fatal("mode switching or poll fetched again")
	}
	e.mu.Lock()
	e.stopScoutingLocked()
	e.scouting.scope = "new-game"
	token := e.scouting.token
	e.mu.Unlock()
	if e.saveScout(1, "old-game", token-1, "p", core.ScoutReport{State: "ready"}) {
		t.Fatal("late game report applied")
	}
}
func TestScoutingRepeatedPageCannotBecomeCompleteSeason(t *testing.T) {
	f := &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC(), repeat: true}
	e := startScoutTest(t, f)
	r := waitScout(t, e)
	if r.Complete || r.Complete30 || r.State != "partial" || f.pages.Load() != 2 {
		t.Fatal("repeated page accepted as full history")
	}
}

func TestCachedScoutReportPublishesWithoutAnotherRequest(t *testing.T) {
	f := &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}
	e := New(nil, nil)
	e.onChange = func(core.Snapshot) {}
	e.updates = make(chan core.Snapshot, 8)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(context.Background())
	r := &core.ScoutReport{State: "ready", Complete30: true, FetchedAt: time.Now().UTC()}
	e.scouting.cache = map[string]*core.ScoutReport{"p": r}
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	select {
	case s := <-e.updates:
		if s.Participants[0].Scout != r {
			t.Fatal("cached report was not published")
		}
	default:
		t.Fatal("cache-only statistics never reached UI")
	}
	if f.pages.Load() != 0 {
		t.Fatal("cached report fetched again")
	}
}

func TestSourceExhaustionDoesNotProveEntireSeasonOrMonth(t *testing.T) {
	f := &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC(), endEarly: true}
	e := startScoutTest(t, f)
	r := waitScout(t, e)
	if r.Complete || r.Complete30 || !r.Complete12 || r.State != "partial" || f.pages.Load() != 1 {
		t.Fatal("short retained history was advertised as the full season/month")
	}
}

func TestExpiredSeasonCacheCannotBeReusedAsCurrentSeason(t *testing.T) {
	now := time.Now().UTC()
	f := &scoutClient{fakeClient: &fakeClient{}, now: now, endEarly: true}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client, e.clientVersion = f, 1
	e.initialMatch.setParent(ctx)
	old := &core.ScoutReport{State: "ready", Complete30: true, FetchedAt: now.Add(-time.Minute), Season: &core.ScoutSeason{Name: "old", Start: now.Add(-365 * 24 * time.Hour), End: now.Add(-30 * time.Second)}}
	e.scouting.cache = map[string]*core.ScoutReport{"p": old}
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	r := waitScout(t, e)
	if f.pages.Load() == 0 || r.Season == nil || r.Season.Name == "old" {
		t.Fatal("fresh but expired-season cache was reused")
	}
}

func TestScoutRefreshRetainsCoverageAndOriginalCutoffUntilReplacement(t *testing.T) {
	now := time.Now().UTC()
	season := &core.ScoutSeason{Name: "test", Start: now.Add(-90 * 24 * time.Hour), End: now.Add(90 * 24 * time.Hour)}
	old := &core.ScoutReport{State: "ready", Season: season, Read: 1200, Complete30: true, FetchedAt: now.Add(-time.Minute)}
	e := New(nil, nil)
	e.client = &fakeClient{}
	e.clientVersion = 1
	e.scouting.scope, e.scouting.token = "scope", 1
	e.scouting.cache = map[string]*core.ScoutReport{}
	e.snapshot = core.Snapshot{Connected: true, Participants: []core.Participant{{ConnectionID: 1, PUUID: "p", Scout: old}}}
	for _, incoming := range []core.ScoutReport{
		{State: "loading", Season: season, FetchedAt: now},
		{State: "partial", Season: season, Read: 20, FetchedAt: now},
		{State: "partial", FetchedAt: now}, // season request failed, old season is still active
	} {
		if !e.saveScout(1, "scope", 1, "p", incoming) {
			t.Fatal("current report rejected")
		}
		got := e.Snapshot().Participants[0].Scout
		if !got.Retained || got.Read != 1200 || !got.FetchedAt.Equal(old.FetchedAt) || got.State != incoming.State {
			t.Fatal("refresh discarded earlier coverage or relabelled its cutoff")
		}
	}
	fresh := core.ScoutReport{State: "ready", Season: season, Read: 1250, Complete30: true, FetchedAt: now}
	e.saveScout(1, "scope", 1, "p", fresh)
	got := e.Snapshot().Participants[0].Scout
	if got.Retained || got.Read != 1250 || !got.FetchedAt.Equal(now) {
		t.Fatal("fresh replacement was not applied")
	}
	nextSeason := &core.ScoutSeason{Name: "next", Start: now, End: now.Add(180 * 24 * time.Hour)}
	e.saveScout(1, "scope", 1, "p", core.ScoutReport{State: "loading", Season: nextSeason, FetchedAt: now})
	got = e.Snapshot().Participants[0].Scout
	if got.Retained || got.Read != 0 || got.Season.Name != "next" {
		t.Fatal("previous season was presented as current season")
	}
}

type firstPageScoutClient struct {
	*scoutClient
	secondStarted chan struct{}
	releaseSecond chan struct{}
}

func (s *firstPageScoutClient) ChampionMasteries(ctx context.Context, id string) ([]core.ChampionMastery, error) {
	if id == "second" {
		close(s.secondStarted)
		select {
		case <-s.releaseSecond:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.scoutClient.ChampionMasteries(ctx, id)
}
func TestUnlockedChampionFirstPageArrivesBeforeOtherPlayersMastery(t *testing.T) {
	f := &firstPageScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC(), endEarly: true}, secondStarted: make(chan struct{}), releaseSecond: make(chan struct{})}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.client, e.clientVersion = f, 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "preparing", Phase: "ChampSelect", Participants: []core.Participant{{ConnectionID: 1, Key: "first", PUUID: "first", IdentityState: core.IdentityKnown, ChampionID: 23, ChampionState: "intent"}, {ConnectionID: 1, Key: "second", PUUID: "second", IdentityState: core.IdentityKnown}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	select {
	case <-f.secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second player mastery never started")
	}
	// The first player must not be blocked behind another player's slow
	// mastery: the workers are independent.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := e.Snapshot()
		if len(s.Participants) > 0 && s.Participants[0].Scout != nil && s.Participants[0].Scout.Read == 20 && s.Participants[0].ChampionState == "intent" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	s := e.Snapshot()
	if s.Participants[0].Scout == nil || s.Participants[0].Scout.Read != 20 || s.Participants[0].ChampionState != "intent" {
		t.Fatal("first-page statistics blocked by another player's mastery")
	}
	before := f.pages.Load()
	// A pure lock/hero update reuses the same report and preparation budget.
	e.mu.Lock()
	roster := clone(e.snapshot).Participants
	roster[0].ChampionState = "selected"
	e.replaceLocked("preparing", "ChampSelect", 0, roster, "")
	e.mu.Unlock()
	if f.pages.Load() != before || e.Snapshot().Participants[0].Scout.Read != 20 {
		t.Fatal("locking lost report or restarted fetching")
	}
	close(f.releaseSecond)
}
