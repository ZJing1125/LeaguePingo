package engine

import (
	"context"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func readRoster(ctx context.Context, client Client, stage Stage) (int64, []core.Participant, error) {
	if stage == StagePreparing {
		if selection, ok := client.(source.ChampSelectSource); ok {
			return selection.ChampSelectRoster(ctx)
		}
	}
	return client.Roster(ctx)
}

func (e *Engine) syncChampSelectEvents(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil || e.client == nil || !e.snapshot.Connected || snapshotStage(e.snapshot.Stage) != StagePreparing || (e.phaseHint != PhaseUnknown && e.phaseHint != PhaseChampSelect) {
		e.stopChampSelectEventsLocked()
		return
	}
	if e.champWatchCancel != nil || (e.champWatchVersion == e.clientVersion && time.Now().Before(e.champWatchRetry)) {
		return
	}
	stream, ok := e.client.(source.ChampSelectEventSource)
	if !ok {
		return
	}
	watchCtx, cancel := context.WithCancel(ctx)
	e.champWatchCancel, e.champWatchVersion = cancel, e.clientVersion
	e.champWatchSeq++
	version, sequence := e.clientVersion, e.champWatchSeq
	go func() {
		defer cancel()
		_ = stream.WatchChampSelect(watchCtx, func() { e.observeChampSelect(version, sequence) })
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.clientVersion == version && e.champWatchSeq == sequence {
			e.champWatchCancel = nil
			e.champWatchRetry = time.Now().Add(time.Minute)
			e.champWatchSeq++
		}
	}()
}

func (e *Engine) stopChampSelectEventsLocked() {
	if e.champWatchCancel != nil {
		e.champWatchCancel()
	}
	e.champWatchCancel = nil
	e.champWatchSeq++
	e.champEpoch++
}

func (e *Engine) observeChampSelect(version, sequence uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != version || e.champWatchSeq != sequence || snapshotStage(e.snapshot.Stage) != StagePreparing || (e.phaseHint != PhaseUnknown && e.phaseHint != PhaseChampSelect) {
		return
	}
	e.champEpoch++
	select {
	case e.champWake <- struct{}{}:
	default:
	}
}

// The event path only verifies phase and the local selection roster. It does
// not perform account/profile queries or reset the prefetch budget on swaps.
func (e *Engine) refreshChampSelect(ctx context.Context) {
	e.mu.RLock()
	client, version, generation, epoch, phaseEpoch := e.client, e.clientVersion, e.snapshot.Generation, e.champEpoch, e.phaseEpoch
	eligible := e.snapshot.Connected && snapshotStage(e.snapshot.Stage) == StagePreparing && (e.phaseHint == PhaseUnknown || e.phaseHint == PhaseChampSelect)
	e.mu.RUnlock()
	if client == nil || !eligible {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	phaseValue, err := client.Phase(requestCtx)
	if err != nil {
		return
	} // Periodic polling remains responsible for connectivity.
	if Phase(phaseValue) != PhaseChampSelect {
		e.mu.Lock()
		if e.clientVersion == version && e.phaseEpoch == phaseEpoch {
			e.stopChampSelectEventsLocked()
			if e.cancel != nil {
				e.cancel()
			}
			select {
			case e.phaseWake <- struct{}{}:
			default:
			}
		}
		e.mu.Unlock()
		return
	}
	e.mu.RLock()
	stillEligible := e.clientVersion == version && e.phaseEpoch == phaseEpoch && e.snapshot.Generation == generation && snapshotStage(e.snapshot.Stage) == StagePreparing
	e.mu.RUnlock()
	if !stillEligible {
		return
	}
	gameID, roster, err := readRoster(requestCtx, client, StagePreparing)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clientVersion != version || e.snapshot.Generation != generation || e.champEpoch != epoch || e.phaseEpoch != phaseEpoch || snapshotStage(e.snapshot.Stage) != StagePreparing {
		return
	}
	if err != nil || len(roster) == 0 {
		return
	} // Never erase a usable roster on a transient failure.
	for i := range roster {
		roster[i].ConnectionID = version
	}
	if gameID == 0 {
		gameID = e.snapshot.GameID
	}
	if gameID != e.snapshot.GameID || rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(StagePreparing, PhaseChampSelect, gameID, roster, "")
	}
}
