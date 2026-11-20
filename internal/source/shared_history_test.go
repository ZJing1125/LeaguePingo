package source

import (
	"context"
	"leaguepingo/internal/core"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedHistoryKeepsIndependentWaitersAndRechecksFinishedHeads(t *testing.T) {
	var reads sharedHistoryPages
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	load := func(ctx context.Context) (core.HistoryPage, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return core.HistoryPage{Offset: 42}, nil
		case <-ctx.Done():
			return core.HistoryPage{}, ctx.Err()
		}
	}
	first, cancel := context.WithCancel(context.Background())
	done1, done2 := make(chan error, 1), make(chan error, 1)
	go func() { _, err := reads.read(first, "same", load); done1 <- err }()
	<-started
	go func() {
		p, err := reads.read(context.Background(), "same", load)
		if err == nil && p.Offset != 42 {
			err = context.Canceled
		}
		done2 <- err
	}()
	deadline := time.After(time.Second)
	for {
		reads.mu.Lock()
		n := reads.flights["same"].users
		reads.mu.Unlock()
		if n == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("second waiter did not join")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if <-done1 != context.Canceled {
		t.Fatal("first consumer not canceled")
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate origin read")
	}
	close(release)
	if err := <-done2; err != nil {
		t.Fatal("one consumer canceled the other")
	}
	if _, err := reads.read(context.Background(), "same", load); err != nil || calls.Load() != 2 {
		t.Fatal("completed head reused without checking freshness")
	}
}

func TestSharedHistoryCancelsLastWaiterAndReset(t *testing.T) {
	for _, reset := range []bool{false, true} {
		var reads sharedHistoryPages
		started, stopped := make(chan struct{}), make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = reads.read(ctx, "old", func(c context.Context) (core.HistoryPage, error) {
				close(started)
				<-c.Done()
				close(stopped)
				return core.HistoryPage{}, c.Err()
			})
		}()
		<-started
		if reset {
			reads.reset()
		} else {
			cancel()
		}
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("unused or reset request kept running")
		}
		cancel()
		<-done
	}
}
