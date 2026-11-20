package source

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// Only overlapping reads share work. Finished heads are checked afresh next time.
type sharedHistoryPages struct {
	mu      sync.Mutex
	flights map[string]*sharedHistoryPage
}
type sharedHistoryPage struct {
	done   chan struct{}
	cancel context.CancelFunc
	users  int
	page   core.HistoryPage
	err    error
}

func (s *sharedHistoryPages) read(ctx context.Context, key string, load func(context.Context) (core.HistoryPage, error)) (core.HistoryPage, error) {
	if err := ctx.Err(); err != nil {
		return core.HistoryPage{}, err
	}
	s.mu.Lock()
	if s.flights == nil {
		s.flights = map[string]*sharedHistoryPage{}
	}
	flight := s.flights[key]
	if flight == nil {
		request, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		flight = &sharedHistoryPage{done: make(chan struct{}), cancel: cancel}
		s.flights[key] = flight
		go s.load(request, key, flight, load)
	}
	flight.users++
	s.mu.Unlock()
	defer s.leave(key, flight)
	select {
	case <-ctx.Done():
		return core.HistoryPage{}, ctx.Err()
	case <-flight.done:
		return flight.page, flight.err
	}
}

func (s *sharedHistoryPages) load(ctx context.Context, key string, flight *sharedHistoryPage, load func(context.Context) (core.HistoryPage, error)) {
	defer flight.cancel()
	flight.page, flight.err = load(ctx)
	s.mu.Lock()
	if s.flights[key] == flight {
		delete(s.flights, key)
	}
	close(flight.done)
	s.mu.Unlock()
}

func (s *sharedHistoryPages) leave(key string, flight *sharedHistoryPage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	flight.users--
	if flight.users == 0 {
		if s.flights[key] == flight {
			delete(s.flights, key)
		}
		flight.cancel()
	}
}
func (s *sharedHistoryPages) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, flight := range s.flights {
		flight.cancel()
	}
	s.flights = nil
}
