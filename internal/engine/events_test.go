package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type phaseEventClient struct {
	*fakeClient
	watch func(context.Context, func(string)) error
}

func (f *phaseEventClient) WatchPhase(ctx context.Context, notify func(string)) error {
	return f.watch(ctx, notify)
}

func TestPhaseEventsCancelPrefetchAndWakePollingBeforeNormalInterval(t *testing.T) {
	started := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	ready := make(chan struct{})
	trigger := make(chan struct{})
	watchStopped := make(chan struct{})
	fake := &phaseEventClient{fakeClient: &fakeClient{phase: "ChampSelect", roster: fullTestRoster(), gameID: 42}}
	var historyCalls atomic.Int32
	fake.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		if historyCalls.Add(1) != 1 {
			return core.HistoryResult{}, nil
		}
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
		return core.HistoryResult{}, ctx.Err()
	}
	fake.watch = func(ctx context.Context, notify func(string)) error {
		defer close(watchStopped)
		close(ready)
		select {
		case <-trigger:
			fake.mu.Lock()
			fake.phase = "InProgress"
			fake.mu.Unlock()
			notify("InProgress")
		case <-ctx.Done():
			return ctx.Err()
		}
		<-ctx.Done()
		return ctx.Err()
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("engine did not shut down")
		}
	}()
	for _, signal := range []<-chan struct{}{ready, started} {
		select {
		case <-signal:
		case <-time.After(2 * time.Second):
			t.Fatal("phase watch or preparation not started")
		}
	}
	close(trigger)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("phase event did not cancel prefetch")
	}
	deadline := time.Now().Add(time.Second)
	for e.Snapshot().Stage != "in-game" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.Snapshot().Stage != "in-game" {
		t.Fatal("event did not wake phase verification")
	}
	waitInitialMatch(t, e)
	if historyCalls.Load() != 11 {
		t.Fatal("entry did not replace the cancelled pregame flight with one bounded batch")
	}
	for _, p := range e.Snapshot().Participants {
		if !historyCached(p) {
			t.Fatal("cancelled pregame flight left an entry player unprepared")
		}
	}
	cancel()
	select {
	case <-watchStopped:
	case <-time.After(time.Second):
		t.Fatal("watch outlived engine cancellation")
	}
}

func TestFailedPhaseWatchIsCooledDownAndPollingStillWorks(t *testing.T) {
	var attempts atomic.Int32
	fake := &phaseEventClient{fakeClient: &fakeClient{phase: "InProgress", roster: fullTestRoster(), gameID: 42}}
	fake.watch = func(context.Context, func(string)) error {
		attempts.Add(1)
		return errors.New("unsupported")
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	e.syncPhaseEvents(context.Background())
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		failed := !e.phaseEvents.retry.IsZero()
		e.mu.RUnlock()
		if failed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 20; i++ {
		e.syncPhaseEvents(context.Background())
	}
	if attempts.Load() != 1 {
		t.Fatal("failed event source retried without cooldown")
	}
	fake.mu.Lock()
	fake.phase = "EndOfGame"
	fake.mu.Unlock()
	e.poll(context.Background())
	if e.Snapshot().Phase != "EndOfGame" {
		t.Fatal("polling fallback unavailable")
	}
}

func TestPhaseEventsRejectLateConnectionsAndCoalesce(t *testing.T) {
	e := New(nil, nil)
	e.client = &fakeClient{}
	e.clientVersion, e.phaseEvents.sequence = 2, 10
	e.observePhase(1, 10, "InProgress")
	e.observePhase(2, 9, "InProgress")
	if e.phaseEpoch != 0 || len(e.phaseEvents.wake) != 0 {
		t.Fatal("late stream mutated current connection")
	}
	for i := 0; i < 1000; i++ {
		e.observePhase(2, 10, "InProgress")
	}
	if len(e.phaseEvents.wake) != 1 || e.phaseEpoch != 1 {
		t.Fatal("duplicate phases built an unbounded queue")
	}
}

func TestPhaseEventDiscardsOlderInFlightPhaseResponse(t *testing.T) {
	e := New(nil, nil)
	fake := &fakeClient{phase: "ChampSelect"}
	e.client, e.clientVersion, e.phaseEvents.sequence = fake, 2, 10
	fake.phaseFn = func() (string, error) {
		e.observePhase(2, 10, "InProgress")
		return "ChampSelect", nil
	}
	e.poll(context.Background())
	if e.Snapshot().Phase == "ChampSelect" || e.prepareRunning || len(e.phaseEvents.wake) != 1 {
		t.Fatal("old HTTP response restarted preparation after game-start event")
	}
}
