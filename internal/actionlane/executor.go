// Package actionlane provides the shared lifetime guard for opt-in client
// mutations. It owns cancellation, stale-session rejection and bounded action
// permits; product controllers still own their state machines and status text.
package actionlane

import (
	"context"
	"sync"
	"time"
)

type Session struct{ generation uint64 }

type Executor struct {
	mu         sync.Mutex
	generation uint64
	cancel     context.CancelFunc
	blocked    bool
	attempts   int
}

func (e *Executor) Start(parent context.Context, timeout time.Duration) (context.Context, Session, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.blocked {
		return nil, Session{}, false
	}
	e.stopLocked()
	var ctx context.Context
	if timeout > 0 {
		ctx, e.cancel = context.WithTimeout(parent, timeout)
	} else {
		ctx, e.cancel = context.WithCancel(parent)
	}
	e.attempts = 0
	return ctx, Session{generation: e.generation}, true
}

func (e *Executor) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

func (e *Executor) Block() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blocked = true
	e.stopLocked()
}

func (e *Executor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blocked = false
	e.stopLocked()
}

func (e *Executor) Blocked() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.blocked
}

func (e *Executor) Current(session Session) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentLocked(session)
}

// Permit linearizes a mutation against Stop, Block and Reset. The caller
// supplies the action window start because a session may wait safely before a
// client invitation is actually actionable.
func (e *Executor) Permit(session Session, openedAt time.Time, window time.Duration, maxAttempts int) (int, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.currentLocked(session) || openedAt.IsZero() || window <= 0 || maxAttempts <= 0 || time.Since(openedAt) > window || e.attempts >= maxAttempts {
		return e.attempts, false
	}
	e.attempts++
	return e.attempts, true
}

func (e *Executor) Finish(session Session) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentLocked(session) {
		e.stopLocked()
	}
}

func (e *Executor) currentLocked(session Session) bool {
	return session.generation != 0 && session.generation == e.generation && e.cancel != nil && !e.blocked
}

func (e *Executor) stopLocked() {
	e.generation++
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.attempts = 0
}
