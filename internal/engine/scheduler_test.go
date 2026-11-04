package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestSchedulerManualPriorityAndCancellation(t *testing.T) {
	s := newScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	release, _ := s.acquire(ctx, false)
	order := make(chan string, 2)
	go func() {
		done, err := s.acquire(ctx, true)
		if err == nil {
			order <- "manual"
			done()
		}
	}()
	for {
		s.mu.Lock()
		waiting := s.manual
		s.mu.Unlock()
		if waiting > 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("manual did not queue")
		case <-time.After(time.Millisecond):
		}
	}
	go func() {
		done, err := s.acquire(ctx, false)
		if err == nil {
			order <- "automatic"
			done()
		}
	}()
	release()
	if <-order != "manual" || <-order != "automatic" {
		t.Fatal("manual did not receive priority")
	}
	stop, cancelled := context.WithCancel(ctx)
	cancelled()
	if _, err := s.acquire(stop, false); err == nil {
		t.Fatal("cancelled request admitted")
	}
}

func TestSchedulerManualPreemptsAutomaticRequest(t *testing.T) {
	s := newScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	automaticCtx, releaseAutomatic, err := s.acquireAutomatic(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manualReady := make(chan struct{})
	go func() {
		release, acquireErr := s.acquire(ctx, true)
		if acquireErr == nil {
			close(manualReady)
			release()
		}
	}()
	select {
	case <-automaticCtx.Done():
	case <-ctx.Done():
		t.Fatal("manual action did not interrupt the automatic request")
	}
	releaseAutomatic()
	select {
	case <-manualReady:
	case <-ctx.Done():
		t.Fatal("manual action did not acquire the released request lane")
	}
}

func TestPreparationSurvivesChampionChangeAndStopsAtGameStart(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	var calls atomic.Int32
	fake := &fakeClient{phase: "ChampSelect"}
	fake.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-ctx.Done()
		close(cancelled)
		return core.HistoryResult{}, ctx.Err()
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("preparation did not start")
	}
	token := preparationForTest(e).token
	fake.mu.Lock()
	fake.champion = 42
	fake.mu.Unlock()
	e.poll(context.Background())
	if preparationForTest(e).token != token || calls.Load() != 1 {
		t.Fatal("champion change restarted the identity's query")
	}
	fake.mu.Lock()
	fake.phase = "InProgress"
	fake.mu.Unlock()
	e.poll(context.Background())
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("automatic history was not cancelled")
	}
	if calls.Load() != 1 {
		t.Fatal("new automatic history started in game")
	}
}
