package engine

import (
	"context"

	"leaguepingo/internal/core"
)

// A previously verified identity in this connection may display its expired
// history immediately. Refreshing it is a single coalesced, cancellable read,
// with the existing budget; explicit force-refresh keeps its blocking semantics.
func (e *Engine) reuseLookupHistory(ctx context.Context, client Client, generation, version uint64, player core.Participant) (core.Participant, bool) {
	if !historyCached(player) || (client != nil && player.ConnectionID != version) {
		return core.Participant{}, false
	}
	player = cachedPlayer(player, client == nil)
	if client != nil && player.Stale {
		e.refreshHydratedHistory(client, generation, version, player)
		return player, true
	}
	return e.enrichCachedProfile(ctx, player), true
}
