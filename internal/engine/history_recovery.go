package engine

import (
	"context"
	"errors"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// Local UI update channel only; never logged, persisted or sent to a service.
func (e *Engine) PlayerUpdates() <-chan core.Participant {
	return e.historyRecovery.updatesChannel()
}
func (e *Engine) publishPlayerLocked(p core.Participant) {
	if e.client != nil && p.ConnectionID == e.clientVersion {
		e.historyRecovery.publish(p)
	}
}

// RetryHistory is a single low-priority first-page read, never a scan, profile
// refresh or identity lookup. The active UI schedules at most two calls; this
// server quota also prevents remounts/multiple windows from resetting that cap.
func (e *Engine) RetryHistory(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	e.mu.Lock()
	if snapshotStage(e.snapshot.Stage) != StageIdle || (e.phaseHint != PhaseUnknown && stageForPhase(e.phaseHint) != StageIdle) {
		e.mu.Unlock()
		return player, apperrors.New(apperrors.RecoveryExhausted)
	}
	if player.HistoryMeta.Service == nil {
		e.mu.Unlock()
		return player, nil
	}
	switch apperrors.Code(player.HistoryMeta.Service.Code) {
	case apperrors.NotLoggedIn, apperrors.RateLimited, apperrors.ServiceBusy, apperrors.NetworkUnavailable, apperrors.RequestTimeout:
	default:
		e.mu.Unlock()
		return player, apperrors.New(apperrors.RecoveryExhausted)
	}
	if player.HistoryMeta.Service.RetryAt == nil {
		e.mu.Unlock()
		return player, apperrors.New(apperrors.RecoveryExhausted)
	}
	if time.Now().Before(*player.HistoryMeta.Service.RetryAt) {
		e.mu.Unlock()
		return player, nil
	}
	request, attempt, err := e.historyRecovery.beginRetry(ctx, key, time.Now())
	if err != nil {
		e.mu.Unlock()
		return player, err
	}
	epoch := e.phaseEpoch
	e.mu.Unlock()
	refund := false
	defer func() { e.historyRecovery.finishRetry(attempt, refund) }()
	request, release, err := e.scheduler.acquireAutomatic(request)
	if err != nil {
		refund = errors.Is(err, context.Canceled)
		return player, err
	}
	defer release()
	e.mu.RLock()
	valid := e.client != nil && e.clientVersion == connectionID && e.phaseEpoch == epoch && snapshotStage(e.snapshot.Stage) == StageIdle && (e.phaseHint == PhaseUnknown || stageForPhase(e.phaseHint) == StageIdle)
	e.mu.RUnlock()
	if !valid {
		return player, apperrors.New(apperrors.StaleData)
	}
	result, loadErr := client.History(request, player.PUUID)
	e.mu.Lock()
	defer e.mu.Unlock()
	if request.Err() != nil {
		refund = request.Err() == context.Canceled
		return player, request.Err()
	}
	if e.client == nil || e.clientVersion != connectionID || e.phaseEpoch != epoch || snapshotStage(e.snapshot.Stage) != StageIdle || (e.phaseHint != PhaseUnknown && stageForPhase(e.phaseHint) != StageIdle) {
		return player, apperrors.New(apperrors.StaleData)
	}
	// Preserve profile, name and any newer manual history completed meanwhile.
	if latest, ok := e.players.player(player.PUUID); ok && latest.ConnectionID == connectionID {
		player = latest
	}
	if loadErr == nil {
		core.ApplyHistory(&player, result)
	} else {
		checked := time.Now()
		player.HistoryMeta.CheckedAt = &checked
		player.HistoryMeta.Service = result.Meta.Service
		if result.Meta.Service == nil {
			player.HistoryMeta.Service = &core.HistoryServiceState{Code: string(apperrors.CodeOf(loadErr)), RetryAt: apperrors.RetryAt(loadErr)}
		}
		player.Error = apperrors.UserMessage(loadErr, apperrors.DataUnavailable)
	}
	e.cachePlayerLocked(player)
	if loadErr == nil && result.Meta.Service == nil {
		e.historyRecovery.forget(key)
	}
	return player, nil
}
