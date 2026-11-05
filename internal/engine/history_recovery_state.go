package engine

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// historyRecovery owns finite retry episodes, coalesced background tasks and
// the local player-update queue. Engine -> mu is the only lock order; no source
// access, storage, Engine publication or callback occurs while mu is held.
type historyRecovery struct {
	mu         sync.RWMutex
	entries    map[string]*historyRecoveryEntry
	updates    chan core.Participant
	background map[string]*historyRecoveryTask
}

type historyRecoveryEntry struct {
	attempts int
	started  time.Time
	task     *historyRecoveryTask
}

type historyRecoveryTask struct {
	cancel   context.CancelFunc
	finished bool
}

type recoveryAttempt struct {
	key   string
	entry *historyRecoveryEntry
	task  *historyRecoveryTask
}

func (r *historyRecovery) init() {
	r.mu.Lock()
	r.updates = make(chan core.Participant, 64)
	r.mu.Unlock()
}

func (r *historyRecovery) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *historyRecovery) stopLocked() {
	for _, v := range r.entries {
		if v.task != nil {
			v.task.cancel()
			v.task = nil
		}
	}
	for _, task := range r.background {
		task.cancel()
	}
}

func (r *historyRecovery) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
	r.entries, r.background = nil, nil
}

func (r *historyRecovery) forget(key string) {
	r.mu.Lock()
	delete(r.entries, key)
	r.mu.Unlock()
}

func (r *historyRecovery) updatesChannel() <-chan core.Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.updates
}

func (r *historyRecovery) publish(player core.Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updates == nil {
		return
	}
	select {
	case r.updates <- player:
	default:
		select {
		case <-r.updates:
		default:
		}
		select {
		case r.updates <- player:
		default:
		}
	}
}

func (r *historyRecovery) beginRetry(parent context.Context, key string, now time.Time) (context.Context, recoveryAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]*historyRecoveryEntry{}
	}
	v := r.entries[key]
	if v == nil {
		if len(r.entries) >= 64 {
			return nil, recoveryAttempt{}, apperrors.New(apperrors.RecoveryExhausted)
		}
		v = &historyRecoveryEntry{started: now}
		r.entries[key] = v
	}
	if v.task != nil {
		return nil, recoveryAttempt{}, apperrors.New(apperrors.QueryBusy)
	}
	if v.attempts >= 2 || now.Sub(v.started) >= time.Minute {
		return nil, recoveryAttempt{}, apperrors.New(apperrors.RecoveryExhausted)
	}
	deadline := now.Add(6 * time.Second)
	if end := v.started.Add(time.Minute); end.Before(deadline) {
		deadline = end
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	v.attempts++
	v.task = &historyRecoveryTask{cancel: cancel}
	return ctx, recoveryAttempt{key: key, entry: v, task: v.task}, nil
}

func (r *historyRecovery) finishRetry(attempt recoveryAttempt, refund bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if attempt.task.finished {
		return
	}
	attempt.task.finished = true
	attempt.task.cancel()
	if r.entries[attempt.key] != attempt.entry {
		return
	}
	// Stopped work may finish after another attempt starts. Refund only its
	// quota; it must not release the newer task's cancellation/occupancy slot.
	if attempt.entry.task == attempt.task {
		attempt.entry.task = nil
	}
	if refund && attempt.entry.attempts > 0 {
		attempt.entry.attempts--
	}
}

func (r *historyRecovery) beginBackground(parent context.Context, key string) (context.Context, *historyRecoveryTask, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.background == nil {
		r.background = map[string]*historyRecoveryTask{}
	}
	if r.background[key] != nil || len(r.background) >= 64 {
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	task := &historyRecoveryTask{cancel: cancel}
	r.background[key] = task
	return ctx, task, true
}

func (r *historyRecovery) finishBackground(key string, task *historyRecoveryTask) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task.cancel()
	if r.background[key] == task {
		delete(r.background, key)
	}
}
