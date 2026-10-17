package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// Browsing is always an explicit UI action and shares the manual request lane.
func (e *Engine) browsePlayer(key string, connectionID uint64) (Client, core.Participant, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.client == nil {
		return nil, core.Participant{}, source.ErrNotConnected
	}
	if connectionID != e.clientVersion {
		return nil, core.Participant{}, errors.New("资料来自旧连接，请重新查询玩家")
	}
	if p, ok := e.cache[key]; ok && p.ConnectionID == connectionID {
		return e.client, p, nil
	}
	for _, p := range e.snapshot.Participants {
		if p.Key == key && p.ConnectionID == connectionID && core.IsKnownPUUID(p.PUUID) {
			return e.client, p, nil
		}
	}
	return nil, core.Participant{}, errors.New("玩家资料已失效，请重新查询")
}

func (e *Engine) LoadProfile(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	profiles, ok := client.(source.ProfileSource)
	if !ok {
		return core.Participant{}, errors.New("当前数据源不支持玩家资料")
	}
	profileCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	release, err := e.scheduler.acquire(profileCtx, true)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	profile := profiles.Profile(profileCtx, player.PUUID)
	if profile.Level == nil && profile.RankState != "ready" && profile.PrivacyState != "public" && profile.PrivacyState != "private" {
		return core.Participant{}, errors.New("当前客户端未提供可核验资料，保留上次结果")
	}
	if player.Profile != nil {
		profile = mergeProfile(profile, player.Profile)
	}
	player.Profile = &profile
	player.PrivacyState = profile.PrivacyState
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != connectionID {
		return core.Participant{}, errors.New("连接已变化，请重新查询玩家资料")
	}
	// A profile-only operation must never replace history that completed meanwhile.
	if current, ok := e.cache[key]; ok {
		current.Profile = &profile
		current.PrivacyState = profile.PrivacyState
		player = current
		e.cache[key] = current
	}
	for i, p := range e.snapshot.Participants {
		if p.Key == key && p.ConnectionID == connectionID {
			e.snapshot.Participants[i].Profile = &profile
			e.snapshot.Participants[i].PrivacyState = profile.PrivacyState
		}
	}
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return player, nil
}

func mergeProfile(next core.Profile, previous *core.Profile) core.Profile {
	if previous == nil {
		return next
	}
	if next.Level == nil {
		next.Level = previous.Level
	}
	if next.ProfileIconID == nil {
		next.ProfileIconID = previous.ProfileIconID
	}
	if next.RankState != "ready" {
		next.Ranks = previous.Ranks
		next.RankState = previous.RankState
	}
	if next.PrivacyState != "public" && next.PrivacyState != "private" {
		next.PrivacyState = previous.PrivacyState
	}
	return next
}

func (e *Engine) MatchDetail(ctx context.Context, key string, connectionID uint64, gameID int64) (core.MatchDetail, error) {
	if gameID <= 0 {
		return core.MatchDetail{}, errors.New("比赛编号无效")
	}
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.MatchDetail{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.MatchDetail{}, err
	}
	cacheKey := fmt.Sprintf("%d:%s", gameID, player.PUUID)
	e.mu.RLock()
	cached, ok := e.details[cacheKey]
	e.mu.RUnlock()
	if ok && cached.ConnectionID == connectionID && time.Since(cached.FetchedAt) < 5*time.Minute {
		return cached, nil
	}
	detailSource, ok := client.(source.MatchDetailSource)
	if !ok {
		return core.MatchDetail{}, errors.New("当前数据源不支持整场对局详情")
	}
	release, err := e.scheduler.acquire(ctx, true)
	if err != nil {
		return core.MatchDetail{}, err
	}
	defer release()
	detail, err := detailSource.MatchDetail(ctx, gameID, player.PUUID)
	if err != nil {
		return core.MatchDetail{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != connectionID {
		return core.MatchDetail{}, errors.New("连接已变化，请重新查询详情")
	}
	detail.ConnectionID = connectionID
	if e.details == nil || len(e.details) >= 32 {
		e.details = map[string]core.MatchDetail{}
	}
	e.details[cacheKey] = detail
	return detail, nil
}

func (e *Engine) HistoryPage(ctx context.Context, key string, connectionID uint64, offset int) (core.HistoryPage, error) {
	if offset < 0 || offset > 980 {
		return core.HistoryPage{}, errors.New("分页超出范围")
	}
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.HistoryPage{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.HistoryPage{}, err
	}
	pager, ok := client.(source.HistoryPager)
	if !ok {
		return core.HistoryPage{}, errors.New("当前数据源不支持历史分页")
	}
	release, err := e.scheduler.acquire(ctx, true)
	if err != nil {
		return core.HistoryPage{}, err
	}
	defer release()
	page, err := pager.HistoryPage(ctx, player.PUUID, offset)
	if err != nil {
		return core.HistoryPage{}, err
	}
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == connectionID
	e.mu.RUnlock()
	if !current {
		return core.HistoryPage{}, errors.New("连接已变化，分页结果未应用，请重新查询")
	}
	return page, nil
}
