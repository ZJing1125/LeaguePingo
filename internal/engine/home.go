package engine

import (
	"context"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// The home page prepares only the verified local account, once per connection
// in the UI. It uses the existing manual lane and 12-second budget. During a
// lobby, champion selection or gameplay it only reads the prepared cache.
func (e *Engine) CurrentPlayer(ctx context.Context, connectionID uint64) (core.Participant, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer done()
	e.mu.Lock()
	client, version, generation := e.client, e.clientVersion, e.snapshot.Generation
	key := e.accountIdentity
	if client == nil || version != connectionID || !core.IsKnownPUUID(key) {
		e.mu.Unlock()
		return core.Participant{}, apperrors.New(apperrors.NotLoggedIn)
	}
	player, cached := e.players.player(key)
	for _, p := range e.snapshot.Participants {
		if p.PUUID == key && p.ConnectionID == version {
			player, cached = p, true
			break
		}
	}
	idle := snapshotStage(e.snapshot.Stage) == StageIdle && (e.phaseHint == PhaseUnknown || stageForPhase(e.phaseHint) == StageIdle)
	if !idle || cached {
		e.mu.Unlock()
		if !cached {
			return core.Participant{}, apperrors.New(apperrors.QueryBusy)
		}
		if idle && !cacheFresh(player) {
			e.refreshHydratedHistory(client, generation, version, player)
		}
		return cachedPlayer(player, false), nil
	}
	ctx, cancel := e.queries.startHome(ctx)
	e.mu.Unlock()
	defer cancel()
	if !cached {
		player = core.Participant{Key: key, PUUID: key, ConnectionID: version, IdentityState: core.IdentityKnown, PrivacyState: "unknown", HistoryState: core.HistoryPending}
		if e.hydrateHistory(&player) {
			e.mu.Lock()
			e.cachePlayerLocked(player)
			e.mu.Unlock()
			e.refreshHydratedHistory(client, generation, version, player)
			return player, nil
		}
	}
	result, err := e.loadManualHistoryGuarded(ctx, client, generation, version, player, false, false, func() bool {
		return ctx.Err() != context.Canceled && e.client != nil && e.clientVersion == version && e.accountIdentity == key && snapshotStage(e.snapshot.Stage) == StageIdle && (e.phaseHint == PhaseUnknown || stageForPhase(e.phaseHint) == StageIdle)
	})
	// An optional profile timeout must not discard a completed first history.
	if ctx.Err() != nil && (ctx.Err() == context.Canceled || !historyCached(result)) {
		return core.Participant{}, ctx.Err()
	}
	return result, err
}
