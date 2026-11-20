package engine

import (
	"context"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/source"
)

func (e *Engine) HistoryPageForQueue(ctx context.Context, key string, connection uint64, request core.HistoryPageRequest) (core.HistoryPage, error) {
	if request.Offset < 0 || request.Offset > 980 || request.MixedOffset < 0 || request.MixedOffset > 1000 || request.RemainingPages < 1 || request.RemainingPages > 50 {
		return core.HistoryPage{}, apperrors.New(apperrors.InvalidPage)
	}
	if _, ok := core.ScoutModeFor(request.QueueID); !ok {
		return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
	}
	client, player, err := e.browsePlayer(key, connection)
	if err != nil {
		return core.HistoryPage{}, err
	}
	pager, ok := client.(source.QueueHistorySource)
	if request.MixedOnly || !ok || !pager.SupportsHistoryQueue(request.QueueID) {
		return e.mixedQueueFallback(ctx, key, connection, request.MixedOffset, 1)
	}
	queued := time.Now()
	requestCtx, release, err := e.scheduler.acquireAutomatic(ctx)
	diagnostics.ResponseStage(ctx, "queue", time.Since(queued))
	if err != nil {
		return core.HistoryPage{}, err
	}
	page, err := pager.HistoryPageForQueue(requestCtx, player.PUUID, request.QueueID, request.Offset)
	canceled := requestCtx.Err()
	release()
	if canceled != nil {
		return core.HistoryPage{}, canceled
	}
	if apperrors.CodeOf(err) == apperrors.Unsupported && request.RemainingPages >= 2 {
		return e.mixedQueueFallback(ctx, key, connection, request.MixedOffset, 2)
	}
	if err != nil {
		return core.HistoryPage{}, err
	}
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == connection
	e.mu.RUnlock()
	if !current {
		return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
	}
	page.Requests = 1
	return page, nil
}
func (e *Engine) mixedQueueFallback(ctx context.Context, key string, connection uint64, offset, requests int) (core.HistoryPage, error) {
	if offset == 1000 {
		return core.HistoryPage{Offset: offset, NextOffset: offset, Requests: requests, LimitReason: "已达到最近 1000 条混合记录的查询上限，按已找到的模式样本分析。"}, nil
	}
	page, err := e.HistoryPage(ctx, key, connection, offset)
	page.QueueID, page.Requests = 0, requests
	return page, err
}

func (e *Engine) VerifyHistoryQueue(ctx context.Context) (core.HistoryQueueVerification, error) {
	e.mu.RLock()
	client, version := e.client, e.clientVersion
	phase := snapshotPhase(e.snapshot.Phase)
	e.mu.RUnlock()
	if client == nil {
		return core.HistoryQueueVerification{}, source.ErrNotConnected
	}
	if phase != PhaseNone && phase != PhaseLobby && phase != PhaseEndOfGame && phase != PhasePreEndOfGame && phase != PhaseWaitingForStats {
		return core.HistoryQueueVerification{}, apperrors.New(apperrors.QueryBusy)
	}
	verifier, ok := client.(source.QueueHistorySource)
	if !ok {
		return core.HistoryQueueVerification{}, apperrors.New(apperrors.Unsupported)
	}
	result, err := verifier.VerifyHistoryQueue(ctx, 2400)
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == version
	e.mu.RUnlock()
	if !current {
		return core.HistoryQueueVerification{}, apperrors.New(apperrors.StaleData)
	}
	return result, err
}
