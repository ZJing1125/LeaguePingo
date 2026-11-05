package engine

import (
	"context"
	"fmt"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// queryState is the Engine boundary for live guards and published state. Reads
// return values; only commit evaluates the optional guard under Engine's lock.
// Query I/O never runs under querySession's lock or inside a state callback.
type queryState interface {
	resolvePlayerName(context.Context, Client, uint64, core.Participant, bool) core.Participant
	enrichCachedProfile(context.Context, core.Participant) core.Participant
	queryCachedPlayer(string, uint64) queryCacheRead
	queryCurrent(uint64) bool
	markProfileAttempt(uint64, string)
	commitManualQuery(uint64, uint64, core.Participant, func() bool) (core.Participant, error)
}

type queryCacheRead struct {
	player         core.Participant
	found, current bool
}

// playerQueries owns foreground query orchestration and its exclusive session.
// The scheduler and guarded state are explicit dependencies, not an Engine
// pointer. Connection/phase/generation mutation remains at the state boundary.
type playerQueries struct {
	*querySession
	scheduler *requestScheduler
	state     queryState
}

func newPlayerQueries(state queryState, scheduler *requestScheduler) *playerQueries {
	return &playerQueries{querySession: newQuerySession(), scheduler: scheduler, state: state}
}

func (q *playerQueries) history(ctx context.Context, client Client, generation, version uint64, player core.Participant, force, enrich bool, validLocked func() bool) (core.Participant, error) {
	// Preserve an already verified complete-ID name before consulting a cache
	// populated from a nameless roster. Both name passes keep their old order.
	if !core.IsPlaceholderName(player.DisplayName) {
		player = q.state.resolvePlayerName(ctx, client, version, player, true)
	}
	cache := q.state.queryCachedPlayer(player.PUUID, version)
	if !force && cache.current && cache.found && cacheFresh(cache.player) {
		if enrich {
			return q.state.enrichCachedProfile(ctx, cachedPlayer(cache.player, false)), nil
		}
		return cachedPlayer(cache.player, false), nil
	}
	player = q.state.resolvePlayerName(ctx, client, version, player, true)
	if player.Stale {
		return core.Participant{}, apperrors.New(apperrors.StaleData)
	}
	hadHistory := historyCached(player)
	player.HistoryState = core.HistoryLoading
	history, err := q.scheduler.history(ctx, fmt.Sprintf("%d:%s", version, player.PUUID), true, func(c context.Context) (core.HistoryResult, error) {
		if !q.state.queryCurrent(version) {
			return core.HistoryResult{}, source.ErrNotConnected
		}
		return client.History(c, player.PUUID)
	})
	player, err = manualHistoryResult(player, cache, hadHistory, history, err)
	if err != nil {
		return core.Participant{}, err
	}
	player.ConnectionID = version
	if enriched, stale := q.profile(ctx, client, version, player, cache, enrich); stale {
		return enriched, nil
	} else {
		player = enriched
	}
	return q.state.commitManualQuery(generation, version, player, validLocked)
}

// A refresh failure keeps verified history; a first failure can still return a
// usable identity and optional profile. Original service/freshness clocks stay.
func manualHistoryResult(player core.Participant, cache queryCacheRead, hadHistory bool, history core.HistoryResult, err error) (core.Participant, error) {
	if err == nil {
		core.ApplyHistory(&player, history)
		return player, nil
	}
	if history.Meta.Service == nil && ((cache.current && cache.found && historyCached(cache.player)) || hadHistory) {
		return core.Participant{}, apperrors.Normalize(err, apperrors.DataUnavailable)
	}
	if cache.current && cache.found && historyCached(cache.player) {
		player = cache.player
	}
	player.HistoryState = core.HistoryFailed
	if len(player.History) > 0 {
		player.HistoryState, player.Cached = core.HistoryReady, true
	}
	checked := time.Now()
	player.HistoryMeta.CheckedAt, player.HistoryMeta.Service = &checked, history.Meta.Service
	player.Error = "已确认玩家身份；" + apperrors.UserMessage(err, apperrors.DataUnavailable)
	return player, nil
}

// Optional profiles use the same manual scheduler and only two seconds of the
// existing caller budget; unsupported or timed-out profiles keep the history.
func (q *playerQueries) profile(ctx context.Context, client Client, version uint64, player core.Participant, cache queryCacheRead, enrich bool) (core.Participant, bool) {
	profiles, supported := client.(source.ProfileSource)
	if !supported || !enrich {
		return player, false
	}
	profileCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	release, err := q.scheduler.acquire(profileCtx, true)
	if err != nil {
		return player, false
	}
	defer release()
	if !q.state.queryCurrent(version) {
		player.Stale = true
		return player, true
	}
	q.state.markProfileAttempt(version, player.Key)
	profile := profiles.Profile(profileCtx, player.PUUID)
	previous := player.Profile
	if previous == nil && cache.current && cache.found {
		previous = cache.player.Profile
	}
	profile = mergeProfile(profile, previous)
	player.Profile, player.PrivacyState = &profile, profile.PrivacyState
	return player, false
}
