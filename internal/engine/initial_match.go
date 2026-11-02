package engine

import (
	"context"
	"fmt"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

const initialMatchBudget = 45 * time.Second
const initialMatchPlayerLimit = 10
const initialMatchMessage = "正在补齐本局开局资料；每局仅一轮，最多 10 位玩家、45 秒。"

// Run owns this bounded entry batch, both for an in-game startup and for players
// whose identities only become available after champion selection ends.
type initialMatchPreparation struct {
	parent            context.Context
	eligible, started bool
	version           uint64
	token             uint64
	gameID            int64
	cancel            context.CancelFunc
}

func (e *Engine) stopInitialMatchLocked() {
	s := &e.initialMatch
	if s.cancel != nil {
		s.cancel()
		for i := range e.snapshot.Participants {
			p := &e.snapshot.Participants[i]
			if p.ConnectionID == s.version && p.HistoryState == core.HistoryLoading {
				p.HistoryState = core.HistoryPending
				p.Error = "开局补齐已停止，可手动查询"
			}
		}
	}
	s.eligible = false
}

func (e *Engine) startInitialMatchLocked() {
	s := &e.initialMatch
	if s.parent == nil || s.parent.Err() != nil || e.client == nil || !e.snapshot.Connected || !activeMatch(snapshotPhase(e.snapshot.Phase)) || e.snapshot.GameID == 0 || (e.phaseHint != PhaseUnknown && !activeMatch(e.phaseHint)) {
		return
	}
	if s.gameID != e.snapshot.GameID {
		e.stopInitialMatchLocked()
		s.token++
		s.gameID, s.version = e.snapshot.GameID, e.clientVersion
		s.started, s.eligible = false, true
	}
	if !s.eligible || s.started || s.version != e.clientVersion {
		return
	}
	// Wait for actual gameplay and allow bounded identity discovery to finish.
	if snapshotPhase(e.snapshot.Phase) == PhaseGameStart || (!completeRoster(e.snapshot.Participants) && e.matchRosterAttempts < inGameRosterAttemptLimit) {
		return
	}
	s.started = true
	var targets []core.Participant
	known := false
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if !core.IsKnownPUUID(p.PUUID) || len(targets) >= initialMatchPlayerLimit {
			continue
		}
		known = true
		if cached, ok := e.cache[p.PUUID]; ok && cached.ConnectionID == s.version && historyCached(cached) && cacheFresh(cached) {
			core.ApplyHistory(p, core.HistoryResult{Matches: cached.History, Meta: cached.HistoryMeta})
			if core.IsPlaceholderName(p.DisplayName) {
				p.DisplayName = cached.DisplayName
			}
		}
		if historyCached(*p) && cacheFresh(*p) && !core.IsPlaceholderName(p.DisplayName) && !e.needsAutomaticProfileLocked(*p) {
			continue
		}
		if !historyCached(*p) || !cacheFresh(*p) {
			p.HistoryState = core.HistoryLoading
			p.Error = ""
		}
		targets = append(targets, *p)
	}
	if len(targets) == 0 {
		s.eligible = false
		e.snapshot.Message = "已复用本局已有资料；本局不再自动补取，缺失资料可手动查询。"
		if !known {
			e.snapshot.Message = "客户端尚未提供可查询身份；可刷新本地名单或输入完整 ID 查询。"
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
		return
	}
	ctx, cancel := context.WithTimeout(s.parent, initialMatchBudget)
	s.cancel = cancel
	e.snapshot.Message = initialMatchMessage
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	go e.prepareInitialMatch(ctx, s.version, s.gameID, s.token, e.client, targets)
}

func (e *Engine) initialMatchValidLocked(version uint64, gameID int64) bool {
	return e.initialMatch.eligible && e.initialMatch.cancel != nil && e.client != nil && e.clientVersion == version && e.snapshot.Connected && e.snapshot.GameID == gameID && activeMatch(snapshotPhase(e.snapshot.Phase)) && (e.phaseHint == PhaseUnknown || activeMatch(e.phaseHint))
}

func (e *Engine) initialMatchValid(version uint64, gameID int64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.initialMatchValidLocked(version, gameID)
}

func (e *Engine) prepareInitialMatch(ctx context.Context, version uint64, gameID int64, token uint64, client Client, targets []core.Participant) {
	defer e.finishInitialMatch(version, gameID, token)
	for _, target := range targets {
		if ctx.Err() != nil || !e.initialMatchValid(version, gameID) {
			return
		}
		phaseCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		phase, err := client.Phase(phaseCtx)
		cancel()
		if err != nil || !activeMatch(Phase(phase)) {
			return
		}
		if ctx.Err() != nil || !e.initialMatchValid(version, gameID) {
			return
		}
		e.resolvePlayerName(ctx, client, version, target, false)
		e.mu.Lock()
		current := core.Participant{}
		for i := range e.snapshot.Participants {
			p := &e.snapshot.Participants[i]
			if p.PUUID == target.PUUID && p.ConnectionID == version {
				if cached, ok := e.cache[p.PUUID]; ok && cached.ConnectionID == version && historyCached(cached) && cacheFresh(cached) {
					core.ApplyHistory(p, core.HistoryResult{Matches: cached.History, Meta: cached.HistoryMeta})
					e.snapshot.UpdatedAt = time.Now()
					e.publishLocked()
				}
				current = *p
				break
			}
		}
		e.mu.Unlock()
		if current.PUUID == "" {
			continue
		}
		if historyCached(current) && cacheFresh(current) {
			continue
		}
		if ctx.Err() != nil || !e.initialMatchValid(version, gameID) {
			return
		}
		requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		history, err := e.scheduler.history(requestCtx, fmt.Sprintf("%d:%s", version, target.PUUID), false, func(c context.Context) (core.HistoryResult, error) {
			if !e.initialMatchValid(version, gameID) {
				return core.HistoryResult{}, context.Canceled
			}
			return client.History(c, target.PUUID)
		})
		cancel()
		if ctx.Err() != nil {
			return
		}
		e.mu.Lock()
		if !e.initialMatchValidLocked(version, gameID) {
			e.mu.Unlock()
			return
		}
		for i := range e.snapshot.Participants {
			p := &e.snapshot.Participants[i]
			if p.PUUID != target.PUUID || p.ConnectionID != version {
				continue
			}
			// Explicit work completed in the meantime takes precedence.
			if historyCached(*p) && cacheFresh(*p) {
				break
			}
			if err != nil {
				p.HistoryState = core.HistoryFailed
				p.Error = apperrors.UserMessage(err, apperrors.DataUnavailable)
			} else {
				core.ApplyHistory(p, history)
				e.cachePlayerLocked(*p)
			}
			break
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
		e.mu.Unlock()
	}
	// Optional data uses only the remaining one-shot budget, after all histories.
	for _, target := range targets {
		if ctx.Err() != nil || !e.initialMatchValid(version, gameID) {
			return
		}
		e.prepareOptionalProfile(ctx, client, target, func() bool { return e.initialMatchValidLocked(version, gameID) })
	}

}

func (e *Engine) finishInitialMatch(version uint64, gameID int64, token uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := &e.initialMatch
	// A cancelled previous game's worker may finish after the next batch starts.
	if s.version != version || s.gameID != gameID || s.token != token {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.eligible = false
	if e.clientVersion != version || !e.snapshot.Connected || e.snapshot.GameID != gameID || !activeMatch(snapshotPhase(e.snapshot.Phase)) {
		return
	}
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.HistoryState == core.HistoryLoading {
			p.HistoryState = core.HistoryPending
			p.Error = "开局补齐已结束，可手动查询"
		}
	}
	e.snapshot.Message = "本局开局资料补齐已结束；未完成的玩家可手动查询，本局不再自动重试。"
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
}
