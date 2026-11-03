package engine

import (
	"context"
	"sync"
	"time"
)

// eventSubscription owns only stream lifetime and wake coalescing. The engine
// still verifies connection/phase invariants under its own lock before applying
// a hint. Lock order is engine -> subscription; no callback runs under this lock.
type eventSubscription struct {
	mu                sync.Mutex
	cancel            context.CancelFunc
	version, sequence uint64
	retry             time.Time
	wake              chan struct{}
}

func newEventSubscription() *eventSubscription {
	return &eventSubscription{wake: make(chan struct{}, 1)}
}

func (s *eventSubscription) begin(ctx context.Context, version uint64, now time.Time) (context.Context, context.CancelFunc, uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil || s.cancel != nil || (s.version == version && now.Before(s.retry)) {
		return nil, nil, 0, false
	}
	watch, cancel := context.WithCancel(ctx)
	s.cancel, s.version = cancel, version
	s.sequence++
	return watch, cancel, s.sequence, true
}

func (s *eventSubscription) finish(version, sequence uint64, now time.Time, invalidate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version != version || s.sequence != sequence {
		return
	}
	s.cancel = nil
	s.retry = now.Add(time.Minute)
	if invalidate {
		s.sequence++
	}
}

func (s *eventSubscription) stop(resetRetry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
	s.sequence++
	if resetRetry {
		s.retry = time.Time{}
	}
}

func (s *eventSubscription) resetRetry() {
	s.mu.Lock()
	s.retry = time.Time{}
	s.mu.Unlock()
}

func (s *eventSubscription) current(sequence uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sequence == sequence
}

func (s *eventSubscription) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
