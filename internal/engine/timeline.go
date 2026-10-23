package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// Timeline is explicit, bounded and shares the manual lane with history/details.
// It never runs from roster preparation, render, scrolling or mode changes.
func (e *Engine) MatchTimeline(ctx context.Context, key string, connectionID uint64, gameID int64) (core.MatchTimeline, error) {
	if gameID <= 0 {
		return core.MatchTimeline{}, errors.New("比赛编号无效")
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
		return core.MatchTimeline{}, errors.New("玩家身份无法核验")
	}
	timelineSource, ok := client.(source.MatchTimelineSource)
	if !ok {
		return core.MatchTimeline{}, errors.New("当前数据源未提供时间线")
	}
	cacheKey := fmt.Sprintf("%d:%s", gameID, player.PUUID)
	e.mu.RLock()
	cached, found := e.timelines[cacheKey]
	detail, hasDetail := e.details[cacheKey]
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
		return core.MatchTimeline{}, errors.New("连接已变化，请重新查询")
	}
	if !hasDetail || detail.ConnectionID != connectionID || time.Since(detail.FetchedAt) >= 5*time.Minute {
		loader, ok := client.(source.MatchDetailSource)
		if !ok {
			return core.MatchTimeline{}, errors.New("尚未取得可核验双方资料")
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
		return core.MatchTimeline{}, errors.New("连接或请求已变化，时间线未应用")
	}
	if timeline.GameID != gameID {
		return core.MatchTimeline{}, errors.New("时间线比赛编号不匹配")
	}
	timeline.ConnectionID = connectionID
	detail.ConnectionID = connectionID
	timeline.Detail = &detail
	if e.timelines == nil || len(e.timelines) >= 8 {
		e.timelines = map[string]core.MatchTimeline{}
	}
	if e.details == nil || len(e.details) >= 32 {
		e.details = map[string]core.MatchDetail{}
	}
	e.timelines[cacheKey] = timeline
	e.details[cacheKey] = detail
	return timeline, nil
}
