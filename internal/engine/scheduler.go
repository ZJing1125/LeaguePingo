package engine

import (
	"context"
	"leaguepingo/internal/core"
	"sync"
)

// One expensive client request at a time. Waiting manual actions take priority.
type requestScheduler struct {
	mu      sync.Mutex
	busy    bool
	manual  int
	changed chan struct{}
	flights map[string]*historyFlight
}
type historyFlight struct {
	done   chan struct{}
	result core.HistoryResult
	err    error
}

func newScheduler() *requestScheduler {
	return &requestScheduler{changed: make(chan struct{}), flights: map[string]*historyFlight{}}
}
func (s *requestScheduler) signal() { close(s.changed); s.changed = make(chan struct{}) }
func (s *requestScheduler) acquire(ctx context.Context, manual bool) (func(), error) {
	s.mu.Lock()
	if manual {
		s.manual++
	}
	for {
		if err := ctx.Err(); err != nil {
			if manual {
				s.manual--
				s.signal()
			}
			s.mu.Unlock()
			return nil, err
		}
		if !s.busy && (manual || s.manual == 0) {
			s.busy = true
			if manual {
				s.manual--
			}
			s.mu.Unlock()
			return func() { s.mu.Lock(); s.busy = false; s.signal(); s.mu.Unlock() }, nil
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-changed:
		}
		s.mu.Lock()
	}
}
func (s *requestScheduler) history(ctx context.Context, key string, manual bool, load func(context.Context) (core.HistoryResult, error)) (core.HistoryResult, error) {
	s.mu.Lock()
	if f, ok := s.flights[key]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return core.HistoryResult{}, ctx.Err()
		case <-f.done:
		}
		// A manual action may outlive a cancelled pregame worker.
		if f.err != nil && manual && ctx.Err() == nil {
			return s.history(ctx, key, manual, load)
		}
		return f.result, f.err
	}
	f := &historyFlight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	release, err := s.acquire(ctx, manual)
	if err == nil {
		f.result, f.err = load(ctx)
		release()
	} else {
		f.err = err
	}
	s.mu.Lock()
	delete(s.flights, key)
	close(f.done)
	s.mu.Unlock()
	return f.result, f.err
}
