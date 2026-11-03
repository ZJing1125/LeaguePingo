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
	if ctx.Err() != nil || e.client == nil {
		return
	}
	stream, ok := e.client.(source.PhaseEventSource)
	if !ok {
		return
	}
	version := e.clientVersion
	watchCtx, cancel, sequence, started := e.phaseEvents.begin(ctx, version, time.Now())
	if !started {
		return
	}
	go func() {
		defer cancel()
		_ = stream.WatchPhase(watchCtx, func(phase string) { e.observePhase(version, sequence, phase) })
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.clientVersion == version {
			e.phaseEvents.finish(version, sequence, time.Now(), false)
		}
	}()
}

func (e *Engine) stopPhaseEventsLocked() {
	e.phaseEvents.stop(true) // Reject late callbacks, including on the same client.
}

func (e *Engine) observePhase(version, sequence uint64, phase string) {
	observed := Phase(phase)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != version || !e.phaseEvents.current(sequence) || observed == PhaseUnknown || observed == e.phaseHint {
		return
	}
	e.phaseEpoch++
	stage := stageForPhase(observed)
	if stage != StageIdle {
		e.historyRecovery.stop()
	}
	e.phaseHint = observed
	e.readiness.Phase(phase, time.Now())
	e.observeAutoAcceptLocked(observed)
	e.postGame.Hint(version, phase)
	if !activeMatch(observed) && e.groupCancel != nil {
		e.groupCancel()
		e.groupCancel = nil
	}
	if stage != snapshotStage(e.snapshot.Stage) || observed == PhaseGameStart {
		e.stopScoutingLocked()
	}
	if stage != StageIdle && e.homeCancel != nil {
		e.homeCancel()
	}
	if !activeMatch(observed) {
		e.stopInitialMatchLocked()
	}
	if observed != PhaseChampSelect {
		e.stopChampSelectEventsLocked()
	}
	if (!preparationStage(stage) || stage != snapshotStage(e.snapshot.Stage)) && e.cancel != nil {
		e.cancel() // Stop prefetch before waiting for a confirming HTTP response.
	}
	e.phaseEvents.notify()
}
