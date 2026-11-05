package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func waitInitialMatch(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		state := initialMatchForTest(&e.initialMatch)
		done := state.started && state.cancel == nil
		e.mu.RUnlock()
		if done {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("initial match preparation did not settle")
}

func TestInitialMatchRunFillsNamesAndHistoryOnce(t *testing.T) {
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	for i := range f.roster {
		f.roster[i].DisplayName = "玩家名称暂不可用"
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	waitInitialMatch(t, e)
	for _, p := range e.Snapshot().Participants {
		if p.DisplayName != "Alice#TEST" || !historyCached(p) {
			t.Fatal("initial match player was not filled")
		}
	}
	f.mu.Lock()
	names, histories := 0, 0
	for _, call := range f.calls {
		if call == "name" {
			names++
		}
		if call == "history" {
			histories++
		}
		if call == "find" {
			t.Error("startup resolved identities again")
		}
	}
	f.mu.Unlock()
	if names != 10 || histories != 10 {
		t.Fatalf("unexpected startup requests: name=%d history=%d", names, histories)
	}
	for i := 0; i < 5; i++ {
		e.poll(context.Background())
	}
	if _, err := e.RefreshRoster(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"Lobby", "GameStart", "InProgress"} {
		f.mu.Lock()
		f.phase = phase
		f.gameID = 43
		f.mu.Unlock()
		e.poll(context.Background())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if call == "history" {
			count++
		}
	}
	if count != 10 {
		t.Fatal("polling or manual roster refresh restarted startup batch")
	}
}

func TestInitialMatchPublishesEachPlayerBeforeBatchCompletes(t *testing.T) {
	secondStarted, releaseSecond := make(chan struct{}), make(chan struct{})
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	f.historyFn = func(ctx context.Context, puuid string) (core.HistoryResult, error) {
		if puuid == f.roster[1].PUUID {
			close(secondStarted)
			select {
			case <-releaseSecond:
			case <-ctx.Done():
				return core.HistoryResult{}, ctx.Err()
			}
		}
		return core.HistoryResult{}, nil
	}
	published := make(chan core.Snapshot, 64)
	e := New(func() (Client, error) { return f, nil }, func(snapshot core.Snapshot) { published <- snapshot })
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		close(releaseSecond)
		t.Fatal("second player request never started")
	}
	if snapshot := e.Snapshot(); snapshot.Participants[0].HistoryState != core.HistoryEmpty || snapshot.Participants[1].HistoryState != core.HistoryLoading {
		close(releaseSecond)
		t.Fatalf("first result waited for the batch: first=%s second=%s", snapshot.Participants[0].HistoryState, snapshot.Participants[1].HistoryState)
	}
	found := false
	for !found {
		select {
		case snapshot := <-published:
			found = snapshot.Participants[0].HistoryState == core.HistoryEmpty && snapshot.Participants[1].HistoryState == core.HistoryLoading
		case <-time.After(2 * time.Second):
			close(releaseSecond)
			t.Fatal("first player result was not published to the UI")
		}
	}
	close(releaseSecond)
	waitInitialMatch(t, e)
}

func TestInitialMatchUsesPartialRosterAfterIdentityBudgetEnds(t *testing.T) {
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()[:9]}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	f.mu.Lock()
	f.rosterErr = errors.New("later roster read unavailable")
	f.mu.Unlock()
	e.poll(context.Background())
	e.poll(context.Background())
	waitInitialMatch(t, e)
	if len(e.Snapshot().Participants) != 9 {
		t.Fatal("partial roster was fabricated or lost")
	}
	histories := 0
	for _, call := range f.calls {
		if call == "history" {
			histories++
		}
	}
	if histories != 9 {
		t.Fatal("partial roster was never filled or exceeded known identities")
	}
}

func TestMatchEntryReusesPreparedAlliesAndFillsNewEnemies(t *testing.T) {
	f := &fakeClient{phase: "ChampSelect", gameID: 42, roster: fullTestRoster()}
	f.roster = f.roster[:5]
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	f.mu.Lock()
	before := len(f.calls)
	f.phase = "GameStart"
	f.roster = fullTestRoster()
	for i := 5; i < 10; i++ {
		f.roster[i].DisplayName = "玩家名称暂不可用"
	}
	f.mu.Unlock()
	e.poll(context.Background())
	if initialMatchForTest(&e.initialMatch).started {
		t.Fatal("loading screen started player requests")
	}
	f.mu.Lock()
	f.phase = "InProgress"
	f.mu.Unlock()
	e.poll(context.Background())
	waitInitialMatch(t, e)
	for _, p := range e.Snapshot().Participants {
		if !historyCached(p) || core.IsPlaceholderName(p.DisplayName) {
			t.Fatal("match entry left known opponents without data")
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	names, histories := 0, 0
	for _, call := range f.calls[before:] {
		if call == "history" {
			histories++
		}
		if call == "name" {
			names++
		}
	}
	if names != 5 || histories != 5 {
		t.Fatalf("entry did not reuse allies and fill exactly five enemies: names=%d histories=%d", names, histories)
	}
}

func TestInitialMatchWaitsForFirstNonemptyPhase(t *testing.T) {
	f := &fakeClient{phase: "", gameID: 42, roster: fullTestRoster()}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	if initialMatchForTest(&e.initialMatch).gameID != 0 {
		t.Fatal("empty phase consumed startup eligibility")
	}
	f.mu.Lock()
	f.phase = "InProgress"
	f.mu.Unlock()
	e.poll(context.Background())
	waitInitialMatch(t, e)
	if !initialMatchForTest(&e.initialMatch).started {
		t.Fatal("first confirmed active phase did not initialise match")
	}
}

func TestEachMatchGetsOneEntryBatchButReconnectDoesNotRetry(t *testing.T) {
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	var calls atomic.Int32
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		calls.Add(1)
		return core.HistoryResult{}, errors.New("synthetic failure")
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	waitInitialMatch(t, e)
	for i := 0; i < 3; i++ {
		e.poll(context.Background())
	}
	if _, err := e.RefreshRoster(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.invalidateConnectionLocked()
	e.mu.Unlock()
	e.poll(context.Background())
	if calls.Load() != 10 {
		t.Fatal("poll, manual refresh or reconnect restarted the same match")
	}
	f.mu.Lock()
	f.phase = "Lobby"
	f.mu.Unlock()
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	// Count only the next entry batch, not permitted room preparation.
	before := calls.Load()
	f.mu.Lock()
	f.phase, f.gameID = "InProgress", 43
	f.mu.Unlock()
	e.poll(context.Background())
	waitInitialMatch(t, e)
	if calls.Load()-before != 10 {
		t.Fatal("new match did not receive exactly one bounded batch")
	}
}

func TestPreviousMatchCompletionCannotCancelNewBatch(t *testing.T) {
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.initialMatch = initialMatchPreparation{version: 2, gameID: 43, token: 2, cancel: cancel, eligible: true}
	e.finishInitialMatch(2, 42, 1)
	if ctx.Err() != nil || initialMatchForTest(&e.initialMatch).cancel == nil || !initialMatchForTest(&e.initialMatch).eligible {
		t.Fatal("previous game's completion cancelled new preparation")
	}
}

func TestInitialMatchWaitsForInProgressAndReusesFreshHistory(t *testing.T) {
	f := &fakeClient{phase: "GameStart", gameID: 42, roster: fullTestRoster()}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	if initialMatchForTest(&e.initialMatch).started {
		t.Fatal("loading screen started player requests")
	}
	e.mu.Lock()
	for _, p := range e.snapshot.Participants {
		core.ApplyHistory(&p, core.HistoryResult{})
		e.cachePlayerLocked(p)
	}
	e.mu.Unlock()
	f.mu.Lock()
	f.phase = "InProgress"
	f.mu.Unlock()
	e.poll(context.Background())
	waitInitialMatch(t, e)
	for _, p := range e.Snapshot().Participants {
		if !historyCached(p) {
			t.Fatal("fresh history was not reused")
		}
	}
	for _, call := range f.calls {
		if call == "history" || call == "name" {
			t.Fatal("cached player was fetched again")
		}
	}
}

func TestInitialMatchTimeoutClearsLoadingAndDoesNotRetry(t *testing.T) {
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	var calls atomic.Int32
	f.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		calls.Add(1)
		<-ctx.Done()
		return core.HistoryResult{}, ctx.Err()
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	e.initialMatch.setParent(ctx)
	e.poll(context.Background())
	waitInitialMatch(t, e)
	for _, p := range e.Snapshot().Participants {
		if p.HistoryState == core.HistoryLoading {
			t.Fatal("timeout left perpetual loading")
		}
	}
	for i := 0; i < 5; i++ {
		e.poll(context.Background())
	}
	if calls.Load() != 1 {
		t.Fatal("timeout batch retried")
	}
}

func TestInitialMatchCapsPlayersAndFailureAttempts(t *testing.T) {
	roster := append(fullTestRoster(), core.Participant{Key: "extra", PUUID: "extra", IdentityState: core.IdentityKnown})
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: roster}
	var calls atomic.Int32
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		calls.Add(1)
		return core.HistoryResult{}, errors.New("synthetic failure")
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	waitInitialMatch(t, e)
	if calls.Load() != initialMatchPlayerLimit {
		t.Fatal("startup exceeded player/attempt cap")
	}
	if e.Snapshot().Participants[10].HistoryState == core.HistoryLoading {
		t.Fatal("unrequested extra player was left loading")
	}
}

func TestInitialMatchCancellationRejectsLateDataAndKeepsDisconnectReason(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	f := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		close(started)
		<-release
		return core.HistoryResult{Matches: []core.Match{{GameID: 99}}}, nil
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	e.initialMatch.setParent(context.Background())
	e.poll(context.Background())
	<-started
	e.mu.Lock()
	e.disconnectLocked("synthetic disconnected")
	e.mu.Unlock()
	close(release)
	waitInitialMatch(t, e)
	s := e.Snapshot()
	if s.Message != "synthetic disconnected；仍可查看已缓存的对局资料" {
		t.Fatal("late startup completion erased disconnect reason")
	}
	for _, p := range s.Participants {
		if len(p.History) > 0 || p.HistoryState == core.HistoryLoading {
			t.Fatal("cancelled startup applied late history or left loading")
		}
	}
}
