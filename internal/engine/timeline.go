package engine

import (
	"context"
	"fmt"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// Timeline is explicit, bounded and shares the manual lane with history/details.
// It never runs from roster preparation, render, scrolling or mode changes.
func (e *Engine) MatchTimeline(ctx context.Context, key string, connectionID uint64, gameID int64) (core.MatchTimeline, error) {
	if gameID <= 0 {
		return core.MatchTimeline{}, apperrors.New(apperrors.InvalidMatch)
	}
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.MatchTimeline{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.MatchTimeline{}, err
	}
	if player.IdentityState != core.IdentityKnown || !core.IsKnownPUUID(player.PUUID) {
		return core.MatchTimeline{}, apperrors.New(apperrors.PlayerIdentityUnavailable)
	}
	timelineSource, ok := client.(source.MatchTimelineSource)
	if !ok {
		return core.MatchTimeline{}, apperrors.New(apperrors.Unsupported)
	}
	cacheKey := fmt.Sprintf("%d:%s", gameID, player.PUUID)
	e.mu.RLock()
	cached, found, detail, hasDetail := e.players.analysis(cacheKey)
	e.mu.RUnlock()
	if found && cached.ConnectionID == connectionID && time.Since(cached.FetchedAt) < 5*time.Minute {
		return cached, nil
	}
	release, err := e.scheduler.acquire(ctx, true)
	if err != nil {
		return core.MatchTimeline{}, err
	}
	defer release()
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == connectionID
	e.mu.RUnlock()
	if !current {
		return core.MatchTimeline{}, apperrors.New(apperrors.StaleData)
	}
	if !hasDetail || detail.ConnectionID != connectionID || time.Since(detail.FetchedAt) >= 5*time.Minute {
		loader, ok := client.(source.MatchDetailSource)
		if !ok {
			return core.MatchTimeline{}, apperrors.New(apperrors.DataUnavailable)
		}
		detail, err = loader.MatchDetail(ctx, gameID, player.PUUID)
		if err != nil {
			return core.MatchTimeline{}, err
		}
	}
	timeline, err := timelineSource.MatchTimeline(ctx, detail, player.PUUID)
	if err != nil {
		return core.MatchTimeline{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil || e.client == nil || e.clientVersion != connectionID {
		return core.MatchTimeline{}, apperrors.New(apperrors.StaleData)
	}
	if timeline.GameID != gameID {
		return core.MatchTimeline{}, apperrors.New(apperrors.InvalidResponse)
	}
	timeline.ConnectionID = connectionID
	detail.ConnectionID = connectionID
	timeline.Detail = &detail
	e.players.rememberTimeline(cacheKey, timeline, detail)
	return timeline, nil
}
