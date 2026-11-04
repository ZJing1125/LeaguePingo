package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func (e *Engine) startPreparationLocked() {
	stage := snapshotStage(e.snapshot.Stage)
	if !preparationStage(stage) || e.client == nil || (e.phaseHint != PhaseUnknown && stageForPhase(e.phaseHint) != stage) {
		return
	}
	ctx, cancel, token, started := e.preparation.begin(e.snapshot.Participants, e.clientVersion, e.preparationProfilesLocked(), time.Now())
	if started {
		go func(version uint64, client Client) {
			defer cancel()
			e.prepare(ctx, token, client, version)
		}(e.clientVersion, e.client)
	}
}

func (e *Engine) finishPreparation(token uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.preparation.finish(token) {
		return
	}
	changed := false
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.HistoryState == core.HistoryLoading {
			p.HistoryState = core.HistoryPending
			p.Error = "准备已暂停，可主动查询"
			changed = true
		}
	}
	if changed {
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
}

func (e *Engine) prepare(ctx context.Context, token uint64, client Client, version uint64) {
	defer e.finishPreparation(token)
	for ctx.Err() == nil {
		player, stage, valid := e.nextPreparedHistory(token, version)
		if !valid {
			return
		}
		if player.PUUID == "" {
			if !e.prepareNextProfile(ctx, token, client, version, stage) {
				return
			}
		} else if !e.prepareHistory(ctx, token, client, version, stage, player) {
			return
		}
	}
}

func (e *Engine) nextPreparedHistory(token, version uint64) (core.Participant, Stage, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stage := snapshotStage(e.snapshot.Stage)
	if !e.preparation.current(token) || e.clientVersion != version || !preparationStage(stage) {
		return core.Participant{}, stage, false
	}
	return e.preparation.nextHistory(clone(e.snapshot).Participants, version), stage, true
}

func (e *Engine) prepareNextProfile(ctx context.Context, token uint64, client Client, version uint64, stage Stage) bool {
	e.mu.Lock()
	if !e.preparation.current(token) || e.clientVersion != version || snapshotStage(e.snapshot.Stage) != stage || ctx.Err() != nil {
		e.mu.Unlock()
		return false
	}
	player := e.preparation.claimProfile(token, e.snapshot.Participants, e.preparationProfilesLocked())
	e.mu.Unlock()
	if player.PUUID == "" {
		return false
	}
	e.prepareOptionalProfile(ctx, client, player, func() bool {
		return e.client != nil && e.clientVersion == version && e.preparation.current(token) && snapshotStage(e.snapshot.Stage) == stage && (e.phaseHint == PhaseUnknown || stageForPhase(e.phaseHint) == stage)
	})
	return true
}

func (e *Engine) prepareHistory(ctx context.Context, token uint64, client Client, version uint64, stage Stage, player core.Participant) bool {
	e.mu.Lock()
	claimed := e.preparation.claimHistory(token, player.PUUID)
	e.mu.Unlock()
	if !claimed {
		return false
	}
	e.mu.RLock()
	cached, ok := e.players.player(player.PUUID)
	e.mu.RUnlock()
	if ok && cacheFresh(cached) {
		e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
			if core.IsPlaceholderName(p.DisplayName) && !core.IsPlaceholderName(cached.DisplayName) {
				p.DisplayName = cached.DisplayName
			}
			core.ApplyHistory(p, core.HistoryResult{Matches: cached.History, Meta: cached.HistoryMeta})
		})
		return true
	}
	e.updatePrepared(token, player.PUUID, func(p *core.Participant) { p.HistoryState = core.HistoryLoading; p.Error = "" })
	if !strings.Contains(player.DisplayName, "#") {
		nameCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		name, err := e.loadName(nameCtx, client, player.PUUID, version, false)
		cancel()
		if err == nil && name != "" {
			e.updatePrepared(token, player.PUUID, func(p *core.Participant) { p.DisplayName = name })
		}
	}
	phaseCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	phaseValue, err := client.Phase(phaseCtx)
	cancel()
	if err != nil || stageForPhase(Phase(phaseValue)) != stage || ctx.Err() != nil {
		e.mu.Lock()
		e.preparation.retryAfterPhaseFailure(token, time.Now())
		e.mu.Unlock()
		return false
	}
	requestCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	history, err := e.scheduler.history(requestCtx, fmt.Sprintf("%d:%s", version, player.PUUID), false, func(c context.Context) (core.HistoryResult, error) { return client.History(c, player.PUUID) })
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == nil {
			e.mu.Lock()
			e.preparation.refundHistory(token, player.PUUID)
			e.mu.Unlock()
			e.updatePrepared(token, player.PUUID, func(p *core.Participant) { p.HistoryState = core.HistoryPending; p.Error = "" })
			return true
		}
		e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
			p.HistoryState = core.HistoryFailed
			p.Error = apperrors.UserMessage(err, apperrors.DataUnavailable)
		})
		return true
	}
	e.updatePrepared(token, player.PUUID, func(p *core.Participant) { core.ApplyHistory(p, history) })
	return true
}
