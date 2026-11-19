package engine

import (
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// playerCache owns only in-memory records, aliases, profile cooldowns and match
// caches. Engine -> cache is the nested lock order. No source access, disk I/O
// or engine callback runs under mu. Values keep the existing model-copy rules;
// connection/phase checks remain with Engine before accepting source results.
type playerCache struct {
	mu           sync.RWMutex
	players      map[string]core.Participant
	aliases      map[string]string
	profileRetry map[string]time.Time
	details      map[string]core.MatchDetail
	timelines    map[string]core.MatchTimeline
}

func (c *playerCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.players, c.aliases = nil, nil
	c.profileRetry, c.details, c.timelines = nil, nil, nil
}

func (c *playerCache) player(key string) (core.Participant, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.players[key]
	return p, ok
}

func (c *playerCache) byAlias(alias string) (core.Participant, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.players[c.aliases[alias]]
	return p, ok
}

// restorePlayer restores already-verified disk history without turning it into a
// fresh network result or binding a new name alias. Normal writes use remember.
func (c *playerCache) restorePlayer(p core.Participant) {
	p.TeamRating = nil
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.players == nil {
		c.players = map[string]core.Participant{}
	}
	c.players[p.PUUID] = p
}

func (c *playerCache) remember(p core.Participant) core.Participant {
	p.TeamRating = nil
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.players[p.PUUID]; ok && old.ConnectionID == p.ConnectionID && core.IsPlaceholderName(p.DisplayName) && !core.IsPlaceholderName(old.DisplayName) {
		p.DisplayName = old.DisplayName
	}
	// Keep the existing Engine cache rollover policy, separate from SGP's LRU.
	if c.players == nil || len(c.players) >= 64 {
		c.players, c.aliases = map[string]core.Participant{}, map[string]string{}
	}
	if c.aliases == nil {
		c.aliases = map[string]string{}
	}
	c.players[p.PUUID] = p
	if alias := normalizeID(p.DisplayName); alias != "" {
		c.aliases[alias] = p.PUUID
	}
	return p
}

func (c *playerCache) markProfileAttempt(key string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.profileRetry == nil || len(c.profileRetry) >= 64 {
		c.profileRetry = map[string]time.Time{}
	}
	c.profileRetry[key] = now.Add(30 * time.Second)
}

func (c *playerCache) profileCooling(key string, now time.Time) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return now.Before(c.profileRetry[key])
}

func (c *playerCache) detail(key string) (core.MatchDetail, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	detail, ok := c.details[key]
	return detail, ok
}

func (c *playerCache) rememberDetail(key string, detail core.MatchDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rememberDetailLocked(key, detail)
}

func (c *playerCache) rememberDetailLocked(key string, detail core.MatchDetail) {
	if c.details == nil || len(c.details) >= 32 {
		c.details = map[string]core.MatchDetail{}
	}
	c.details[key] = detail
}

func (c *playerCache) analysis(key string) (core.MatchTimeline, bool, core.MatchDetail, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	timeline, found := c.timelines[key]
	detail, hasDetail := c.details[key]
	return timeline, found, detail, hasDetail
}

func (c *playerCache) rememberTimeline(key string, timeline core.MatchTimeline, detail core.MatchDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timelines == nil || len(c.timelines) >= 8 {
		c.timelines = map[string]core.MatchTimeline{}
	}
	c.timelines[key] = timeline
	c.rememberDetailLocked(key, detail)
}
