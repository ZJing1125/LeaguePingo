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
	return e.loadProfileWithName(ctx, key, connectionID, false)
}

// EnsureProfile is invoked only by an explicit user opening/query, never by
// roster events or background preparation. The existing manual gate serializes it.
func (e *Engine) EnsureProfile(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer done()
	return e.loadProfileWithName(ctx, key, connectionID, true)
}

// LoadRanks is explicit, bounded and independent of optional names and avatars.
func (e *Engine) LoadRanks(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	if player.IdentityState != core.IdentityKnown || !core.IsKnownPUUID(player.PUUID) {
		return core.Participant{}, errors.New("客户端未提供有效身份，无法查询段位")
	}
	ranks, ok := client.(source.RankSource)
	if !ok {
		return core.Participant{}, errors.New("当前数据源不支持段位查询")
	}
	rankCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	release, err := e.scheduler.acquire(rankCtx, true)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == connectionID
	e.mu.RUnlock()
	if !current {
		return core.Participant{}, errors.New("连接已变化，请重新查询段位")
	}
	profile := ranks.Ranks(rankCtx, player.PUUID)
	return e.storeProfile(player, connectionID, profile, "rank")
}

func (e *Engine) loadProfileWithName(ctx context.Context, key string, connectionID uint64, onlyMissing bool) (core.Participant, error) {
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	e.resolvePlayerName(ctx, client, connectionID, player, true)
	return e.loadProfileData(ctx, key, connectionID, onlyMissing)
}

func (e *Engine) enrichCachedProfile(ctx context.Context, player core.Participant) core.Participant {
	if player.Stale {
		return player
	}
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()
	player = e.resolvePlayerName(ctx, client, player.ConnectionID, player, true)
	if player.Stale {
		return player
	}
	if updated, err := e.loadProfileData(ctx, player.Key, player.ConnectionID, true); err == nil {
		updated.Cached = player.Cached
		return updated
	}
	return player // Optional data must not turn a cached history into a failed query.
}

func basicProfileComplete(p *core.Profile) bool {
	return p != nil && p.Level != nil && p.ProfileIconData != ""
}

func (e *Engine) markProfileAttempt(version uint64, key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clientVersion != version {
		return
	}
	if e.profileRetry == nil || len(e.profileRetry) >= 64 {
		e.profileRetry = map[string]time.Time{}
	}
	e.profileRetry[key] = time.Now().Add(30 * time.Second)
}

func (e *Engine) loadProfileData(ctx context.Context, key string, connectionID uint64, onlyMissing bool) (core.Participant, error) {
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	if onlyMissing {
		e.mu.RLock()
		cooling := time.Now().Before(e.profileRetry[key])
		e.mu.RUnlock()
		if automaticProfileFresh(player.Profile) || cooling {
			return player, nil
		}
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
	e.mu.RLock()
	currentConnection := e.client != nil && e.clientVersion == connectionID
	e.mu.RUnlock()
	if !currentConnection {
		return core.Participant{}, errors.New("连接已变化，请重新查询玩家资料")
	}
	e.markProfileAttempt(connectionID, key)
	profile := profiles.Profile(profileCtx, player.PUUID)
	if onlyMissing && !profileAvailable(profile) && (player.Profile == nil || !profileAvailable(*player.Profile)) {
		return core.Participant{}, errors.New("当前客户端未提供可核验资料，保留上次结果")
	}
	return e.storeProfile(player, connectionID, profile)
}

func (e *Engine) storeProfile(player core.Participant, connectionID uint64, profile core.Profile, fields ...string) (core.Participant, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != connectionID {
		return core.Participant{}, errors.New("连接已变化，请重新查询玩家资料")
	}
	// A profile-only operation must never replace history that completed meanwhile.
	if current, ok := e.cache[player.Key]; ok {
		player = current
	}
	previous := player.Profile
	if previous == nil {
		previous = &core.Profile{PrivacyState: player.PrivacyState}
	}
	profile = mergeProfile(profile, previous, fields...)
	player.Profile = &profile
	player.PrivacyState = profile.PrivacyState
	e.cachePlayerLocked(player)
	for i, p := range e.snapshot.Participants {
		if p.Key == player.Key && p.ConnectionID == connectionID {
			e.snapshot.Participants[i].Profile = &profile
			e.snapshot.Participants[i].PrivacyState = profile.PrivacyState
		}
	}
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return player, nil
}

// RefreshHistory uses an already verified identity and preserves independent
// profile data. It only repairs a missing name; known names, ID lookup and
// optional profile enrichment are not requested again.
func (e *Engine) RefreshHistory(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer done()
	client, player, err := e.browsePlayer(key, connectionID)
	if err != nil {
		return core.Participant{}, err
	}
	e.mu.RLock()
	generation := e.snapshot.Generation
	e.mu.RUnlock()
	return e.loadManualHistory(ctx, client, generation, connectionID, player, true, false)
}

func mergeProfile(next core.Profile, previous *core.Profile, fields ...string) core.Profile {
	requested := func(field string) bool {
		if field == "rank" && next.RankState == "not-requested" {
			return false
		}
		if len(fields) == 0 {
			return true
		}
		for _, f := range fields {
			if f == field {
				return true
			}
		}
		return false
	}
	rankRequested := requested("rank")
	now := time.Now()
	next.UpdatedAt = &now
	next.Fields = make(map[string]core.FieldFreshness)
	track := func(field string, verified, hadValue bool) {
		meta := core.FieldFreshness{State: "unavailable"}
		if verified {
			meta.State, meta.FetchedAt = "ready", &now
		} else if hadValue && previous != nil {
			meta = previous.Fields[field]
			meta.State = "retained"
		}
		next.Fields[field] = meta
	}
	old := core.Profile{}
	if previous != nil {
		old = *previous
	}
	track("level", next.Level != nil, old.Level != nil)
	track("icon", next.ProfileIconID != nil, old.ProfileIconID != nil)
	track("rank", next.RankState == "ready", old.RankState == "ready")
	if next.RankState == "failed" && old.RankState != "ready" {
		next.Fields["rank"] = core.FieldFreshness{State: "failed"}
	}
	privacyKnown := func(p core.Profile) bool { return p.PrivacyState == "public" || p.PrivacyState == "private" }
	track("privacy", privacyKnown(next), privacyKnown(old))
	if next.Level == nil {
		next.Level = old.Level
	}
	if next.ProfileIconID == nil {
		next.ProfileIconID = old.ProfileIconID
	}
	sameIcon := next.ProfileIconID != nil && old.ProfileIconID != nil && *next.ProfileIconID == *old.ProfileIconID
	track("avatar", next.ProfileIconData != "", sameIcon && old.ProfileIconData != "")
	if next.ProfileIconData == "" && sameIcon {
		next.ProfileIconData = old.ProfileIconData
	}
	if next.RankState != "ready" && old.RankState == "ready" {
		next.Ranks, next.RankState = old.Ranks, old.RankState
	}
	if !privacyKnown(next) && privacyKnown(old) {
		next.PrivacyState = old.PrivacyState
	}
	// Fields omitted by this operation retain their values AND freshness.
	for _, field := range []string{"level", "icon", "avatar", "privacy", "rank"} {
		if (field != "rank" && requested(field)) || (field == "rank" && rankRequested) {
			continue
		}
		delete(next.Fields, field)
		if meta, ok := old.Fields[field]; ok {
			next.Fields[field] = meta
		}
		switch field {
		case "level":
			next.Level = old.Level
		case "icon":
			next.ProfileIconID = old.ProfileIconID
		case "avatar":
			next.ProfileIconData = old.ProfileIconData
		case "privacy":
			next.PrivacyState = old.PrivacyState
		case "rank":
			next.Ranks, next.RankState, next.RankMessage = old.Ranks, old.RankState, old.RankMessage
			if next.RankState == "" {
				next.RankState = "not-requested"
			}
		}
	}
	if next.Ranks == nil {
		next.Ranks = []core.Rank{}
	}
	return next
}

func profileAvailable(p core.Profile) bool {
	return p.Level != nil || p.ProfileIconID != nil || p.RankState == "ready" || p.PrivacyState == "public" || p.PrivacyState == "private"
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
