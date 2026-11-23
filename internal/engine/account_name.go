package engine

import (
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// The caller has verified the poll connection and account under the engine lock.
func (e *Engine) acceptAccountNameLocked() {
	names, ok := e.client.(source.CachedAccountNameSource)
	if !ok {
		return
	}
	name := names.CachedAccountName(e.accountIdentity)
	if core.IsPlaceholderName(name) {
		return
	}
	if player, found := e.players.player(e.accountIdentity); found && player.ConnectionID == e.clientVersion && player.DisplayName != name {
		player.DisplayName = name
		e.players.remember(player)
		e.publishPlayerLocked(player)
	}
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.PUUID == e.accountIdentity && p.ConnectionID == e.clientVersion {
			p.DisplayName = name
		}
	}
}
