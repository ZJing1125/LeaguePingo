package engine

import (
	"context"
	"time"

	"leaguepingo/internal/source"
)

// Polling remains the fallback and the source of truth. Only one optional
// phase stream may exist for a connection, with a one-minute failure cooldown.
func (e *Engine) syncPhaseEvents(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil || e.client == nil || e.phaseWatchCancel != nil || (e.phaseWatchVersion == e.clientVersion && time.Now().Before(e.phaseWatchRetry)) {
		return
	}
	stream, ok := e.client.(source.PhaseEventSource)
	if !ok {
		return
	}
	watchCtx, cancel := context.WithCancel(ctx)
	e.phaseWatchCancel = cancel
	e.phaseWatchVersion = e.clientVersion
	e.phaseWatchSeq++
	version, sequence := e.clientVersion, e.phaseWatchSeq
	go func() {
		defer cancel()
		_ = stream.WatchPhase(watchCtx, func(phase string) { e.observePhase(version, sequence, phase) })
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.phaseWatchSeq == sequence && e.clientVersion == version {
			e.phaseWatchCancel = nil
			e.phaseWatchRetry = time.Now().Add(time.Minute)
		}
	}()
}

func (e *Engine) stopPhaseEventsLocked() {
	if e.phaseWatchCancel != nil {
		e.phaseWatchCancel()
	}
	e.phaseWatchCancel = nil
	e.phaseWatchSeq++ // Reject late callbacks, even when the client version is unchanged.
	e.phaseWatchRetry = time.Time{}
}

func (e *Engine) observePhase(version, sequence uint64, phase string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != version || e.phaseWatchSeq != sequence || phase == "" || phase == e.phaseHint {
		return
	}
	e.phaseEpoch++
	e.phaseHint = phase
	if phase != "ChampSelect" {
		e.stopChampSelectEventsLocked()
	}
	if !preparationStage(stageForPhase(phase)) && e.cancel != nil {
		e.cancel() // Stop prefetch before waiting for a confirming HTTP response.
	}
	select {
	case e.phaseWake <- struct{}{}:
	default: // Coalesce notifications; never build an unbounded event queue.
	}
}
