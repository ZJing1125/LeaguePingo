package engine

import (
	"context"
	"errors"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// One finite recovery episode per verified player/connection. Manual refresh
// does not use this quota, but still obeys the source's shared rate-limit gate.
type historyRecovery struct {
	entries    map[string]*historyRecoveryEntry
	updates    chan core.Participant
	background map[string]*historyRecoveryEntry
}
type historyRecoveryEntry struct {
	attempts int
	started  time.Time
	cancel   context.CancelFunc
}

func (r *historyRecovery) stop() {
	for _, v := range r.entries {
		if v.cancel != nil {
			v.cancel()
			v.cancel = nil
		}
	}
	for _, v := range r.background {
		if v.cancel != nil {
			v.cancel()
		}
	}
}

// Local UI update channel only; never logged, persisted or sent to a service.
func (e *Engine) PlayerUpdates() <-chan core.Participant {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.historyRecovery.updates
}
func (e *Engine) publishPlayerLocked(p core.Participant) {
	if e.historyRecovery.updates == nil || e.client == nil || p.ConnectionID != e.clientVersion {
		return
	}
	select {
	case e.historyRecovery.updates <- p:
	default:
		select {
		case <-e.historyRecovery.updates:
		default:
		}
		select {
		case e.historyRecovery.updates <- p:
		default:
		}
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
	if e.snapshot.Stage != "idle" || (e.phaseHint != "" && stageForPhase(e.phaseHint) != "idle") {
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
	if e.historyRecovery.entries == nil {
		e.historyRecovery.entries = map[string]*historyRecoveryEntry{}
	}
	v := e.historyRecovery.entries[key]
	if v == nil {
		if len(e.historyRecovery.entries) >= 64 {
			e.mu.Unlock()
			return player, apperrors.New(apperrors.RecoveryExhausted)
		}
		v = &historyRecoveryEntry{started: time.Now()}
		e.historyRecovery.entries[key] = v
	}
	if v.cancel != nil {
		e.mu.Unlock()
		return player, apperrors.New(apperrors.QueryBusy)
	}
	if v.attempts >= 2 || time.Since(v.started) >= time.Minute {
		e.mu.Unlock()
		return player, apperrors.New(apperrors.RecoveryExhausted)
	}
	request, cancel := context.WithTimeout(ctx, 6*time.Second)
	v.attempts++
	v.cancel = cancel
	epoch := e.phaseEpoch
	e.mu.Unlock()
	defer cancel()
	refund := false
	defer func() {
		e.mu.Lock()
		v.cancel = nil
		if refund && v.attempts > 0 {
			v.attempts--
		}
		e.mu.Unlock()
	}()
	request, release, err := e.scheduler.acquireAutomatic(request)
	if err != nil {
		refund = errors.Is(err, context.Canceled)
		return player, err
	}
	defer release()
	e.mu.RLock()
	valid := e.client != nil && e.clientVersion == connectionID && e.phaseEpoch == epoch && e.snapshot.Stage == "idle" && (e.phaseHint == "" || stageForPhase(e.phaseHint) == "idle")
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
	if e.client == nil || e.clientVersion != connectionID || e.phaseEpoch != epoch || e.snapshot.Stage != "idle" || (e.phaseHint != "" && stageForPhase(e.phaseHint) != "idle") {
		return player, apperrors.New(apperrors.StaleData)
	}
	// Preserve profile, name and any newer manual history completed meanwhile.
	if latest, ok := e.cache[player.PUUID]; ok && latest.ConnectionID == connectionID {
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
		delete(e.historyRecovery.entries, key)
	}
	return player, nil
}
