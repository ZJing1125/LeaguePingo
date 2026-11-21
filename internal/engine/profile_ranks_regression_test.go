package engine

import (
	"leaguepingo/internal/core"
	"testing"
	"time"
)

func TestPartialRankRefreshKeepsOtherQueueAndItsAge(t *testing.T) {
	old := mergeProfile(core.Profile{RankState: "ready", Ranks: []core.Rank{
		{Queue: "RANKED_SOLO_5x5", Tier: "GOLD"}, {Queue: "RANKED_FLEX_SR", Tier: "SILVER"},
	}}, nil)
	oldTime := old.Fields["rank/RANKED_FLEX_SR"].FetchedAt
	next := mergeProfile(core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "PLATINUM"}}}, &old)
	if len(next.Ranks) != 2 || next.Ranks[1].Tier != "SILVER" || next.Fields["rank/RANKED_FLEX_SR"].State != "retained" || !next.Fields["rank/RANKED_FLEX_SR"].FetchedAt.Equal(*oldTime) {
		t.Fatal("partial queue erased or re-dated the other rank")
	}
	next = mergeProfile(core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_FLEX_SR", Tier: "NONE"}}}, &next)
	if next.Ranks[1].Tier != "NONE" {
		t.Fatal("explicit unranked did not replace old tier")
	}
}

func TestNewKnownSeasonDoesNotKeepMissingOldQueue(t *testing.T) {
	one, two := 1, 2
	old := mergeProfile(core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "GOLD", SeasonID: &one}, {Queue: "RANKED_FLEX_SR", Tier: "GOLD", SeasonID: &one}}}, nil)
	next := mergeProfile(core.Profile{RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "SILVER", SeasonID: &two}}}, &old)
	if len(next.Ranks) != 1 {
		t.Fatal("old season flex rank survived known season transition")
	}
}
func TestRestoreAnonymousHistoryKeepsSessionProfileAndName(t *testing.T) {
	var cache playerCache
	at := time.Now()
	p := core.Participant{Key: "target", PUUID: "target", ConnectionID: 1, DisplayName: "Player#TEST", Profile: &core.Profile{UpdatedAt: &at}}
	cache.remember(p)
	cache.restorePlayer(core.Participant{Key: p.Key, PUUID: p.PUUID, ConnectionID: 1, DisplayName: "玩家名称暂不可用"})
	got, _ := cache.player(p.PUUID)
	if got.Profile != p.Profile || got.DisplayName != p.DisplayName {
		t.Fatal("disk history replaced session identity/profile")
	}
}
