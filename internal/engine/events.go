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
	stream, hasPhase := e.client.(source.PhaseEventSource)
	session, hasSession := e.client.(source.SessionEventSource)
	if !hasPhase && !hasSession {
		return
	}
	version := e.clientVersion
	watchCtx, cancel, sequence, started := e.phaseEvents.begin(ctx, version, time.Now())
	if !started {
		return
	}
	go func() {
		defer cancel()
		phase := func(phase string) { e.observePhase(version, sequence, phase) }
		if hasSession {
			_ = session.WatchSession(watchCtx, phase, func() { e.observeLobby(version, sequence) })
		} else {
			_ = stream.WatchPhase(watchCtx, phase)
		}
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
	previousSelection := e.readiness.SelectionStarted()
	e.readiness.Phase(phase, time.Now())
	if observed == PhaseChampSelect && !previousSelection.Equal(e.readiness.SelectionStarted()) {
		e.beginAnalysisBriefLocked(observed)
	}
	e.observeAutoAcceptLocked(observed)
	e.postGame.Hint(version, phase)
	if !activeMatch(observed) {
		e.grouping.stop()
	}
	if stage != snapshotStage(e.snapshot.Stage) || observed == PhaseGameStart {
		e.stopScoutingLocked()
	}
	if stage != StageIdle {
		e.queries.stopHome()
	}
	if !activeMatch(observed) {
		e.stopInitialMatchLocked()
	}
	if observed != PhaseChampSelect {
		e.observeAnalysisBriefLocked()
		e.stopChampSelectEventsLocked()
	}
	if !preparationStage(stage) || stage != snapshotStage(e.snapshot.Stage) {
		e.preparation.stop() // Stop prefetch before waiting for a confirming HTTP response.
	}
	e.phaseEvents.notify()
}

func (e *Engine) observeLobby(version, sequence uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != version || !e.phaseEvents.current(sequence) ||
		snapshotStage(e.snapshot.Stage) != StageParty || e.phaseHint != PhaseUnknown && stageForPhase(e.phaseHint) != StageParty {
		return
	}
	e.phaseEpoch++ // Reject a lobby result which started before the change.
	select {
	case e.lobbyWake <- struct{}{}:
	default:
	}
}
