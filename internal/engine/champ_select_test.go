package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type selectionClient struct {
	*fakeClient
	selection    []core.Participant
	selectionErr error
	onRead       func()
	watch        func(context.Context, func()) error
	reads        atomic.Int32
}

func (c *selectionClient) ChampSelectRoster(context.Context) (int64, []core.Participant, error) {
	c.reads.Add(1)
	c.mu.Lock()
	roster := append([]core.Participant(nil), c.selection...)
	err, fn := c.selectionErr, c.onRead
	c.mu.Unlock()
	if fn != nil {
		fn()
	}
	return 42, roster, err
}
func (c *selectionClient) WatchChampSelect(ctx context.Context, notify func()) error {
	if c.watch != nil {
		return c.watch(ctx, notify)
	}
	return errors.New("unsupported")
}

func selectionFixture() (*Engine, *selectionClient) {
	now := time.Now()
	players := fullTestRoster()
	for i := range players {
		players[i].ChampionID = 23
		players[i].ChampionState = "selected"
		players[i].HistoryState = core.HistoryReady
		players[i].HistoryMeta.FetchedAt = &now
	}
	c := &selectionClient{fakeClient: &fakeClient{phase: "ChampSelect", roster: fullTestRoster(), gameID: 42}, selection: players}
	e := New(func() (Client, error) { return c, nil }, nil)
	e.poll(context.Background())
	return e, c
}

func TestSelectionHeroSwapsNeverReloadHistoryOrUseGameflow(t *testing.T) {
	e, c := selectionFixture()
	budget, token := e.prepareDeadline, e.prepareToken
	for _, hero := range []int{24, 25, 0, 23} {
		c.selection[0].ChampionID = hero
		e.refreshChampSelect(context.Background())
		if got := e.Snapshot().Participants[0]; got.ChampionID != hero || got.HistoryState != core.HistoryReady {
			t.Fatal("hero change lost or history discarded")
		}
	}
	if e.prepareDeadline != budget || e.prepareToken != token {
		t.Fatal("hero swap restarted preparation budget")
	}
	for _, call := range c.calls {
		if call != "phase" {
			t.Fatalf("hero swap performed unrelated operation %s", call)
		}
	}
	c.selection[0].ChampionID = 26
	e.poll(context.Background())
	if e.Snapshot().Participants[0].ChampionID != 26 {
		t.Fatal("polling fallback ignored selection")
	}
	c.selection[0].ChampionID = 27
	if _, err := e.RefreshRoster(context.Background()); err != nil || e.Snapshot().Participants[0].ChampionID != 27 {
		t.Fatal("manual refresh ignored selection")
	}
}

func TestSelectionRejectsStalePollManualAndEventResponses(t *testing.T) {
	for _, operation := range []string{"poll", "manual", "event"} {
		t.Run(operation, func(t *testing.T) {
			e, c := selectionFixture()
			c.selection[0].ChampionID = 24
			c.onRead = func() { e.observeChampSelect(e.clientVersion, e.champEvents.sequence) }
			switch operation {
			case "poll":
				e.poll(context.Background())
			case "manual":
				_, _ = e.RefreshRoster(context.Background())
			case "event":
				e.refreshChampSelect(context.Background())
			}
			if e.Snapshot().Participants[0].ChampionID != 23 {
				t.Fatal("older selection response was applied")
			}
			c.onRead = nil
			c.selection[0].ChampionID = 25
			e.refreshChampSelect(context.Background())
			if e.Snapshot().Participants[0].ChampionID != 25 {
				t.Fatal("latest selection did not recover")
			}
		})
	}
}

func TestSelectionRejectsOldSessionsAndStopsOnPhaseHint(t *testing.T) {
	e, c := selectionFixture()
	version, sequence := e.clientVersion, e.champEvents.sequence
	e.observeChampSelect(version-1, sequence)
	e.observeChampSelect(version, sequence-1)
	if len(e.champEvents.wake) != 0 {
		t.Fatal("old connection callback accepted")
	}
	for i := 0; i < 1000; i++ {
		e.observeChampSelect(version, sequence)
	}
	if len(e.champEvents.wake) != 1 {
		t.Fatal("unbounded selection queue")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.champEvents.cancel = cancel
	e.observePhase(e.clientVersion, e.phaseEvents.sequence, "InProgress")
	if ctx.Err() == nil {
		t.Fatal("selection watcher not cancelled by game start")
	}
	before := c.reads.Load()
	e.refreshChampSelect(context.Background())
	if c.reads.Load() != before {
		t.Fatal("selection read started after phase exit")
	}
	e.observeChampSelect(version, sequence)
	if e.champEvents.cancel != nil {
		t.Fatal("late callback restarted watcher")
	}
}

func TestSelectionFailureRetainsDataAndWatchCooldown(t *testing.T) {
	e, c := selectionFixture()
	c.selectionErr = errors.New("temporarily unavailable")
	e.refreshChampSelect(context.Background())
	if len(e.Snapshot().Participants) != 10 {
		t.Fatal("transient failure erased roster")
	}
	var attempts atomic.Int32
	c.watch = func(context.Context, func()) error { attempts.Add(1); return errors.New("unsupported") }
	e.syncChampSelectEvents(context.Background())
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		failed := !e.champEvents.retry.IsZero()
		e.mu.RUnlock()
		if failed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 20; i++ {
		e.syncChampSelectEvents(context.Background())
	}
	if attempts.Load() != 1 {
		t.Fatal("selection watch retries not cooled down")
	}
	c.selectionErr = nil
	c.selection[0].ChampionID = 24
	e.poll(context.Background())
	if e.Snapshot().Participants[0].ChampionID != 24 {
		t.Fatal("watch failure disabled polling")
	}
}

func TestSelectionPhaseVerificationCannotStartReadAfterExitNotification(t *testing.T) {
	e, c := selectionFixture()
	before := c.reads.Load()
	c.phaseFn = func() (string, error) {
		e.observePhase(e.clientVersion, e.phaseEvents.sequence, "InProgress")
		return "ChampSelect", nil
	}
	e.refreshChampSelect(context.Background())
	if c.reads.Load() != before {
		t.Fatal("selection request started after game-start notification")
	}
}

func TestSelectionBurstCoalescesInRunAndCancelsOnShutdown(t *testing.T) {
	e, c := selectionFixture()
	ready, emit, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	c.watch = func(ctx context.Context, notify func()) error {
		defer close(stopped)
		close(ready)
		select {
		case <-emit:
		case <-ctx.Done():
			return ctx.Err()
		}
		c.mu.Lock()
		c.selection[0].ChampionID = 26
		c.mu.Unlock()
		for i := 0; i < 1000; i++ {
			notify()
		}
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("engine shutdown stalled")
		}
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("watch did not start")
	}
	before := c.reads.Load()
	close(emit)
	deadline := time.Now().Add(2 * time.Second)
	for e.Snapshot().Participants[0].ChampionID != 26 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.Snapshot().Participants[0].ChampionID != 26 || c.reads.Load()-before != 1 {
		t.Fatal("burst did not coalesce to latest selection")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("selection watcher outlived engine")
	}
}
