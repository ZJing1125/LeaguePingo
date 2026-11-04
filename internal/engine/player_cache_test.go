package engine

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func seedPlayerForTest(e *Engine, key string, player core.Participant) {
	e.players.mu.Lock()
	defer e.players.mu.Unlock()
	if e.players.players == nil {
		e.players.players = map[string]core.Participant{}
	}
	e.players.players[key] = player
}

func cachedPlayerForTest(e *Engine, key string) core.Participant {
	player, _ := e.players.player(key)
	return player
}

type cacheCounts struct{ players, profileRetry, details, timelines int }

func cacheCountsForTest(e *Engine) cacheCounts {
	e.players.mu.RLock()
	defer e.players.mu.RUnlock()
	return cacheCounts{len(e.players.players), len(e.players.profileRetry), len(e.players.details), len(e.players.timelines)}
}

func TestPlayerCachePreservesHistoryAgeNameAndHydration(t *testing.T) {
	var cache playerCache
	old := time.Now().Add(-6 * time.Minute)
	player := core.Participant{Key: "self", PUUID: "self", ConnectionID: 7, DisplayName: "Name#TEST", Cached: true, Stale: true,
		History: []core.Match{{GameID: 42}}, HistoryMeta: core.HistoryMeta{FetchedAt: &old}}
	cache.restorePlayer(player)
	if _, ok := cache.byAlias(normalizeID(player.DisplayName)); ok {
		t.Fatal("hydration invented a verified alias")
	}
	cache.remember(player)
	player.DisplayName = "玩家名称暂不可用"
	cache.remember(player)
	got, ok := cache.byAlias(normalizeID("\u2066Name\u2069#\u2066TEST\u2069"))
	if !ok || got.DisplayName != "Name#TEST" || got.History[0].GameID != 42 || !got.HistoryMeta.FetchedAt.Equal(old) || !got.Cached || !got.Stale {
		t.Fatal("cache changed verified name, history age or stale/cached evidence")
	}
	player.ConnectionID = 8
	cache.remember(player)
	if got, _ := cache.player(player.PUUID); got.DisplayName == "Name#TEST" {
		t.Fatal("old connection supplied a new connection's missing name")
	}
}

func TestPlayerCacheResetAndExistingCapacityBoundaries(t *testing.T) {
	var cache playerCache
	stamp := time.Now()
	for i := range 64 {
		key := fmt.Sprintf("p%d", i)
		cache.remember(core.Participant{Key: key, PUUID: key, DisplayName: key + "#TEST"})
		cache.markProfileAttempt(key, stamp)
	}
	cache.remember(core.Participant{Key: "new", PUUID: "new", DisplayName: "New#TEST"})
	cache.markProfileAttempt("new", stamp)
	if _, ok := cache.byAlias(normalizeID("p0#TEST")); ok || cache.profileCooling("p0", stamp) {
		t.Fatal("existing 64-entry rollover retained stale aliases/cooldowns")
	}
	if _, ok := cache.player("new"); !ok || !cache.profileCooling("new", stamp.Add(29*time.Second)) || cache.profileCooling("new", stamp.Add(30*time.Second)) {
		t.Fatal("new record or exact profile cooldown boundary changed")
	}
	for i := range 32 {
		cache.rememberDetail(fmt.Sprint(i), core.MatchDetail{GameID: int64(i + 1)})
	}
	cache.rememberDetail("new", core.MatchDetail{GameID: 99})
	if _, ok := cache.detail("0"); ok {
		t.Fatal("detail capacity rollover changed")
	}
	for i := range 9 {
		cache.rememberTimeline(fmt.Sprint(i), core.MatchTimeline{GameID: int64(i + 1)}, core.MatchDetail{GameID: int64(i + 1)})
	}
	if _, found, detail, hasDetail := cache.analysis("0"); found || !hasDetail || detail.GameID != 1 {
		t.Fatal("timeline rollover erased its independent detail cache")
	}
	cache.reset()
	if _, ok := cache.byAlias(normalizeID("New#TEST")); ok || cache.profileCooling("new", stamp) {
		t.Fatal("reset retained player aliases/cooldowns")
	}
	if _, found, _, hasDetail := cache.analysis("8"); found || hasDetail {
		t.Fatal("reset retained analysis")
	}
}

func TestPlayerCacheConcurrentResetAndReads(t *testing.T) {
	var cache playerCache
	var workers sync.WaitGroup
	for worker := range 4 {
		workers.Go(func() {
			for i := range 80 {
				key := fmt.Sprintf("p%d-%d", worker, i)
				cache.remember(core.Participant{PUUID: key, DisplayName: key + "#TEST"})
				cache.markProfileAttempt(key, time.Now())
				cache.rememberTimeline(key, core.MatchTimeline{GameID: 42}, core.MatchDetail{GameID: 42})
				cache.player(key)
				cache.byAlias(normalizeID(key + "#TEST"))
				cache.analysis(key)
				cache.reset()
			}
		})
	}
	workers.Wait()
	cache.reset()
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if len(cache.players)+len(cache.aliases)+len(cache.profileRetry)+len(cache.details)+len(cache.timelines) != 0 {
		t.Fatal("reset did not clear every owned cache")
	}
}
