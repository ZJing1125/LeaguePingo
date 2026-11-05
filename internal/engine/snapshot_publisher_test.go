package engine

import (
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func captureSnapshotsForTest(e *Engine, capacity int) <-chan core.Snapshot {
	e.publisher = &snapshotPublisher{updates: make(chan core.Snapshot, capacity)}
	return e.publisher.updates
}

func TestSnapshotPublisherSlowCallbackKeepsLatestOrderedSnapshots(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})
	var once sync.Once
	p := newSnapshotPublisher(func(snapshot core.Snapshot) {
		once.Do(func() { close(entered); <-release })
		if snapshot.Generation == 100 {
			close(completed)
		}
	})
	defer func() { close(p.updates) }() // Test owns this queue; all producers end below.
	p.publish(core.Snapshot{})
	<-entered
	done := make(chan struct{})
	go func() {
		for generation := uint64(1); generation <= 100; generation++ {
			p.publish(core.Snapshot{Generation: generation})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		close(release)
		<-done
		t.Fatal("slow callback blocked snapshot publication")
	}
	p.mu.RLock()
	queued := len(p.updates)
	first := <-p.updates
	p.mu.RUnlock()
	if queued != 32 || first.Generation != 69 {
		close(release)
		t.Fatalf("queue did not retain newest 32 in order: queued=%d first=%d", queued, first.Generation)
	}
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("latest snapshot did not reach callback")
	}
}

func TestEnginePublishesPrivateSnapshotAndAllowsCallbackReentry(t *testing.T) {
	e := New(nil, nil)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan core.Snapshot, 1)
	e.publisher = newSnapshotPublisher(func(snapshot core.Snapshot) {
		close(entered)
		<-release
		_ = e.Snapshot() // Callback may safely re-enter Engine.
		_ = e.publisher.enabled()
		done <- snapshot
	})
	defer close(e.publisher.updates)
	e.mu.Lock()
	e.snapshot.Participants = []core.Participant{{Key: "synthetic", History: []core.Match{{GameID: 42}}}}
	e.publishLocked()
	e.mu.Unlock()
	<-entered
	e.mu.Lock()
	e.snapshot.Participants[0].History[0].GameID = 99
	e.mu.Unlock()
	close(release)
	select {
	case snapshot := <-done:
		if snapshot.Participants[0].History[0].GameID != 42 {
			t.Fatal("queued snapshot shared mutable Engine history")
		}
	case <-time.After(time.Second):
		t.Fatal("callback could not re-enter Engine or publisher")
	}
}

func TestSnapshotPublisherDisabledWithoutCallback(t *testing.T) {
	p := newSnapshotPublisher(nil)
	if p.enabled() || p.updates != nil {
		t.Fatal("disabled publisher created a queue")
	}
	p.publish(core.Snapshot{})
}
