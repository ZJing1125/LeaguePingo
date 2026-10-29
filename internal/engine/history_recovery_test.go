package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func recoveryEngine(t *testing.T) (*Engine, *fakeClient, core.Participant) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	due := time.Now()
	p := core.Participant{Key: "synthetic", PUUID: "synthetic", ConnectionID: 1, IdentityState: core.IdentityKnown, HistoryState: core.HistoryReady, History: []core.Match{{GameID: 10}}, HistoryMeta: core.HistoryMeta{FetchedAt: &old, Service: &core.HistoryServiceState{Code: "ServiceBusy", RetryAt: &due}}, Profile: &core.Profile{RankState: "ready"}}
	f := &fakeClient{}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 1
	e.snapshot = core.Snapshot{Connected: true, Stage: "idle", Phase: "None", ConnectionID: 1}
	e.cachePlayerLocked(p)
	return e, f, p
}

func TestRecoveryQuotaRetainsSnapshotAndSuccessfulReadClearsState(t *testing.T) {
	e, f, p := recoveryEngine(t)
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{Meta: core.HistoryMeta{Service: p.HistoryMeta.Service}}, apperrors.HTTP(apperrors.DataService, 503)
	}
	for range 2 {
		got, err := e.RetryHistory(context.Background(), p.Key, 1)
		if err != nil || len(got.History) != 1 || got.History[0].GameID != 10 || !got.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) || got.Profile == nil {
			t.Fatal("outage discarded local player data")
		}
	}
	if _, err := e.RetryHistory(context.Background(), p.Key, 1); apperrors.CodeOf(err) != apperrors.RecoveryExhausted {
		t.Fatal("automatic quota reset")
	}
	if len(f.calls) != 2 {
		t.Fatalf("unbounded requests: %v", f.calls)
	}
	// Explicit refresh remains available, and does not depend on the auto quota.
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{Matches: []core.Match{{GameID: 11}}}, nil
	}
	got, err := e.RefreshHistory(context.Background(), p.Key, 1)
	if err != nil || got.HistoryMeta.Service != nil || got.History[0].GameID != 11 {
		t.Fatal("manual recovery failed")
	}
}

func TestRecoveryYieldsToManualAndRejectsPhaseOrAccountChanges(t *testing.T) {
	e, f, p := recoveryEngine(t)
	entered := make(chan struct{})
	done := make(chan error, 1)
	f.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		close(entered)
		<-ctx.Done()
		return core.HistoryResult{}, ctx.Err()
	}
	go func() { _, err := e.RetryHistory(context.Background(), p.Key, 1); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := e.scheduler.acquire(ctx, true)
	if err != nil {
		t.Fatal("recovery blocked manual action")
	}
	release()
	if err := <-done; err == nil {
		t.Fatal("preempted recovery applied result")
	}
	if e.historyRecovery.entries[p.Key].attempts != 0 {
		t.Fatal("preemption consumed failure quota")
	}
	for _, stage := range []string{"selection", "in-game"} {
		e.snapshot.Stage = stage
		if _, err := e.RetryHistory(context.Background(), p.Key, 1); err == nil {
			t.Fatal("automatic recovery ran during game preparation/play")
		}
	}
	if _, err := e.RetryHistory(context.Background(), p.Key, 2); err == nil {
		t.Fatal("old identity crossed connection")
	}
}

func TestDegradedLocalFallbackDoesNotEraseOldRemoteHistoryOrWriteDisk(t *testing.T) {
	e, _, p := recoveryEngine(t)
	before := *p.HistoryMeta.FetchedAt
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 1}}, Meta: core.HistoryMeta{Service: p.HistoryMeta.Service}})
	if p.History[0].GameID != 10 || !p.HistoryMeta.FetchedAt.Equal(before) || p.HistoryState != core.HistoryReady || p.HistoryMeta.CheckedAt == nil {
		t.Fatal("fallback fabricated freshness or replaced remote records")
	}
	store, err := openHistoryStore(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	e.historyStore = store
	store.put(p.PUUID, core.HistoryPage{Matches: p.History, Meta: p.HistoryMeta, Offset: 0, NextOffset: 1})
	if _, ok := store.get(p.PUUID, 0, 0); ok {
		t.Fatal("session degradation persisted as fresh history")
	}
}

func TestCachedHomeRefreshPublishesOutageWithoutChangingDataTime(t *testing.T) {
	e, f, p := recoveryEngine(t)
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{Meta: core.HistoryMeta{Service: p.HistoryMeta.Service}}, apperrors.HTTP(apperrors.DataService, 503)
	}
	e.refreshHydratedHistory(f, 0, 1, p)
	select {
	case next := <-e.PlayerUpdates():
		if next.Key != p.Key || next.HistoryMeta.Service == nil || next.Profile == nil || next.History[0].GameID != 10 || !next.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) {
			t.Fatal("idle event dropped retained data or fabricated freshness")
		}
	case <-time.After(time.Second):
		t.Fatal("empty-roster home never received background status")
	}
}

func TestCachedRefreshCoalescesAndRejectsLateAccountResult(t *testing.T) {
	e, f, p := recoveryEngine(t)
	entered, release := make(chan struct{}), make(chan struct{})
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		close(entered)
		<-release
		return core.HistoryResult{Matches: []core.Match{{GameID: 99}}}, nil
	}
	e.refreshHydratedHistory(f, 0, 1, p)
	<-entered
	e.refreshHydratedHistory(f, 0, 1, p)
	e.mu.Lock()
	e.invalidateConnectionLocked()
	e.mu.Unlock()
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		e.scheduler.mu.Lock()
		busy := e.scheduler.busy
		e.scheduler.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-e.PlayerUpdates():
		t.Fatal("late old-account result published")
	default:
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Fatal("duplicate cache refresh was not coalesced")
	}
}

func TestRecoveryCannotBypassWaitOrRetryPermanentFailure(t *testing.T) {
	e, f, p := recoveryEngine(t)
	due := time.Now().Add(time.Minute)
	p.HistoryMeta.Service.RetryAt = &due
	e.cachePlayerLocked(p)
	if _, err := e.RetryHistory(context.Background(), p.Key, 1); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Fatal("early call bypassed cooldown")
	}
	p.HistoryMeta.Service = &core.HistoryServiceState{Code: "AccessDenied", RetryAt: &due}
	e.cachePlayerLocked(p)
	if _, err := e.RetryHistory(context.Background(), p.Key, 1); apperrors.CodeOf(err) != apperrors.RecoveryExhausted {
		t.Fatal("permanent failure allowed automatic retry")
	}
	if len(f.calls) != 0 {
		t.Fatal("permanent failure queried source")
	}
}
