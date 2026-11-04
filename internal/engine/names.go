package engine

import (
	"context"
	"time"

	"leaguepingo/internal/core"
)

// Name enrichment is independent of history freshness. Publish only the name,
// preserving history/profile work that may have completed during the request.
func (e *Engine) resolvePlayerName(ctx context.Context, client Client, version uint64, player core.Participant, manual bool) core.Participant {
	if client == nil || !core.IsKnownPUUID(player.PUUID) || player.Stale {
		return player
	}
	if core.IsPlaceholderName(player.DisplayName) {
		nameCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		name, err := e.loadName(nameCtx, client, player.PUUID, version, manual)
		cancel()
		if err != nil || ctx.Err() != nil || core.IsPlaceholderName(name) {
			e.mu.RLock()
			player.Stale = player.Stale || e.client == nil || e.clientVersion != version
			e.mu.RUnlock()
			return player
		}
		player.DisplayName = name
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != version {
		player.Stale = true
		return player
	}
	if cached, ok := e.players.player(player.PUUID); ok && cached.ConnectionID == version {
		if !core.IsPlaceholderName(cached.DisplayName) {
			player.DisplayName = cached.DisplayName
		} else {
			cached.DisplayName = player.DisplayName
			e.cachePlayerLocked(cached)
		}
	}
	changed := false
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.PUUID == player.PUUID && p.ConnectionID == version && core.IsPlaceholderName(p.DisplayName) {
			p.DisplayName = player.DisplayName
			changed = true
		}
	}
	if changed {
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
	return player
}
