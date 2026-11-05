package engine

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
)

// querySession owns the foreground query slot and the idle home query's child
// cancellation. Engine retains identity/phase/generation checks and scheduling.
// Engine may take mu; this component never calls back into Engine under mu.
type querySession struct {
	mu         sync.Mutex
	gate       chan struct{}
	homeCancel context.CancelFunc
}

func newQuerySession() *querySession {
	return &querySession{gate: make(chan struct{}, 1)}
}

func (q *querySession) begin(parent context.Context) (context.Context, func(), error) {
	select {
	case q.gate <- struct{}{}:
		bounded, cancel := context.WithTimeout(parent, 12*time.Second)
		return bounded, func() { cancel(); <-q.gate }, nil
	default:
		return nil, nil, apperrors.New(apperrors.QueryBusy)
	}
}

// Only a verified, uncached idle account gets this extra cancellation scope.
// Other manual queries share the slot but are not cancelled by leaving idle.
func (q *querySession) startHome(parent context.Context) (context.Context, context.CancelFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	q.homeCancel = cancel
	return ctx, cancel
}

func (q *querySession) stopHome() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.homeCancel != nil {
		q.homeCancel()
	}
}
