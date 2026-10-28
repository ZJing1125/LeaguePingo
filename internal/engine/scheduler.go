package engine

import (
	"context"
	"errors"
	"sync"

	"leaguepingo/internal/core"
)

// One expensive client request at a time. Waiting manual actions take priority.
type requestScheduler struct {
	mu              sync.Mutex
	busy            bool
	manual          int
	automaticCancel context.CancelFunc
	changed         chan struct{}
	flights         map[string]*historyFlight
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
	return s.acquireRequest(ctx, manual, nil)
}
func (s *requestScheduler) acquireAutomatic(ctx context.Context) (context.Context, func(), error) {
	requestCtx, cancel := context.WithCancel(ctx)
	release, err := s.acquireRequest(requestCtx, false, cancel)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return requestCtx, func() { release(); cancel() }, nil
}
func (s *requestScheduler) acquireRequest(ctx context.Context, manual bool, interrupt context.CancelFunc) (func(), error) {
	s.mu.Lock()
	if manual {
		s.manual++
		// A foreground action should not sit behind a speculative page scan.
		// The interrupted worker keeps its cursor and can resume after the action.
		if s.automaticCancel != nil {
			s.automaticCancel()
		}
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
			} else {
				s.automaticCancel = interrupt
			}
			s.mu.Unlock()
			return func() {
				s.mu.Lock()
				s.busy = false
				if !manual {
					s.automaticCancel = nil
				}
				s.signal()
				s.mu.Unlock()
			}, nil
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
		// A manual action or entry batch may outlive a cancelled pregame worker.
		// Retrying that cancelled flight does not retry an actual entry request.
		if f.err != nil && (manual || errors.Is(f.err, context.Canceled)) && ctx.Err() == nil {
			return s.history(ctx, key, manual, load)
		}
		return f.result, f.err
	}
	f := &historyFlight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	requestCtx := ctx
	var release func()
	var err error
	if manual {
		release, err = s.acquire(ctx, true)
	} else {
		requestCtx, release, err = s.acquireAutomatic(ctx)
	}
	if err == nil {
		f.result, f.err = load(requestCtx)
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
