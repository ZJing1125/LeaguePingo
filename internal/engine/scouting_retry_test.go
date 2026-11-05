package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

type flakyScoutClient struct {
	*scoutClient
	calls atomic.Int32
}

func (f *flakyScoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	if f.calls.Add(1) == 1 {
		return core.HistoryPage{}, errors.New("synthetic slow page")
	}
	return f.scoutClient.ScoutPage(ctx, id, offset)
}

// A single slow page used to freeze that player for the rest of the stage. One
// bounded retry keeps the player; only the second failure ends the cursor.
func TestScoutingRetriesOneFailedPage(t *testing.T) {
	f := &flakyScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}}
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
	r := waitScout(t, e)
	if r.Read != 120 || !r.Complete || !r.Complete30 || r.State == "partial" {
		t.Fatalf("one transient page dropped the player: read=%d complete=%v complete30=%v state=%s", r.Read, r.Complete, r.Complete30, r.State)
	}
	if f.pages.Load() != 6 || f.calls.Load() != 7 {
		t.Fatalf("retry changed issued pages: pages=%d calls=%d", f.pages.Load(), f.calls.Load())
	}
	if r.PagesRead != 6 || r.CoverageEndsAt == nil {
		t.Fatalf("coverage fields not published: pages=%d coverage=%v", r.PagesRead, r.CoverageEndsAt)
	}
}

// A second consecutive failure still ends the cursor instead of retrying
// without bound inside the stage scope.
func TestScoutingStopsAfterRepeatedPageFailures(t *testing.T) {
	f := &alwaysFailingScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown, ChampionID: 23}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	r := waitScout(t, e)
	if r.Read != 0 || r.State != "partial" {
		t.Fatalf("repeated failures were not bounded: read=%d state=%s", r.Read, r.State)
	}
	if f.calls.Load() != 2 {
		t.Fatalf("unbounded page retries: %d", f.calls.Load())
	}
	if r.Message == "" {
		t.Fatal("failure reason was not preserved for the interface")
	}
	if r.Reason != string(apperrors.RequestTimeout) {
		t.Fatalf("fixed reason code missing: %q", r.Reason)
	}
}

type alwaysFailingScoutClient struct {
	*scoutClient
	calls atomic.Int32
}

func (f *alwaysFailingScoutClient) ScoutPage(context.Context, string, int) (core.HistoryPage, error) {
	f.calls.Add(1)
	return core.HistoryPage{}, apperrors.New(apperrors.RequestTimeout)
}
