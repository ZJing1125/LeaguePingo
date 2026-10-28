package engine

import (
	"context"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func automaticProfileFresh(p *core.Profile) bool {
	return basicProfileComplete(p) && p.RankState == "ready" && p.UpdatedAt != nil && time.Since(*p.UpdatedAt) < 5*time.Minute
}

func (e *Engine) needsAutomaticProfileLocked(p core.Participant) bool {
	_, supported := e.client.(source.ProfileSource)
	return supported && p.ConnectionID == e.clientVersion && p.IdentityState == core.IdentityKnown && core.IsKnownPUUID(p.PUUID) && !automaticProfileFresh(p.Profile)
}

func (e *Engine) needsPreparedProfileLocked(p core.Participant) bool {
	return len(e.prepareProfiles) < 10 && !e.prepareProfiles[p.PUUID] && e.needsAutomaticProfileLocked(p)
}

// Called only by the existing bounded preparation workers, never by rendering
// or periodic in-game polling. The guard is evaluated while holding e.mu.
func (e *Engine) prepareOptionalProfile(ctx context.Context, client Client, target core.Participant, validLocked func() bool) {
	profiles, ok := client.(source.ProfileSource)
	if !ok || ctx.Err() != nil {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	automaticCtx, release, err := e.scheduler.acquireAutomatic(requestCtx)
	if err != nil {
		return
	}
	defer release()
	e.mu.RLock()
	valid := validLocked()
	var current core.Participant
	for _, p := range e.snapshot.Participants {
		if p.PUUID == target.PUUID && p.ConnectionID == target.ConnectionID {
			current = p
			break
		}
	}
	stage := e.snapshot.Stage
	e.mu.RUnlock()
	if !valid || current.PUUID == "" || automaticProfileFresh(current.Profile) {
		return
	}
	phase, err := client.Phase(automaticCtx)
	if err != nil || stageForPhase(phase) != stage || (stage == "in-game" && !activeMatch(phase)) {
		return
	}
	profile := profiles.Profile(automaticCtx, target.PUUID)
	if automaticCtx.Err() != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !validLocked() {
		return
	}
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.PUUID != target.PUUID || p.ConnectionID != target.ConnectionID {
			continue
		}
		profile = mergeProfile(profile, p.Profile)
		p.Profile, p.PrivacyState = &profile, profile.PrivacyState
		e.cachePlayerLocked(*p)
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
		return
	}
}

// RefreshData is the single explicit refresh action. Optional profile failures
// never discard history, and a history failure still allows the profile update.
func (e *Engine) RefreshData(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
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
	result, historyErr := e.loadManualHistory(ctx, client, generation, connectionID, player, true, false)
	if historyErr != nil {
		result = player
	}
	if _, supported := client.(source.ProfileSource); supported && ctx.Err() == nil {
		if enriched, err := e.loadProfileData(ctx, key, connectionID, false); err == nil {
			result = enriched
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil || e.clientVersion != connectionID {
		return core.Participant{}, source.ErrNotConnected
	}
	if historyErr != nil {
		result.Error = "战绩更新失败，已有记录已保留"
		e.cachePlayerLocked(result)
		for i, p := range e.snapshot.Participants {
			if p.Key == key && p.ConnectionID == connectionID {
				e.snapshot.Participants[i].Error = result.Error
			}
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
	return result, nil
}
