package engine

import (
	"context"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
	"time"
)

// Each queue retains its own value and original observation time.
func mergeProfileRanks(next *core.Profile, old core.Profile, requested bool, now time.Time) {
	result := []core.Rank{}
	changedSeason := false
	for _, n := range next.Ranks {
		for _, o := range old.Ranks {
			if n.Queue == o.Queue && n.SeasonID != nil && o.SeasonID != nil && *n.SeasonID != *o.SeasonID {
				changedSeason = true
			}
		}
	}
	for _, queue := range []string{"RANKED_SOLO_5x5", "RANKED_FLEX_SR"} {
		key := "rank/" + queue
		state := core.FieldFreshness{State: "unavailable"}
		if next.RankState == "failed" {
			state.State = "failed"
		}
		found := false
		if requested {
			for _, rank := range next.Ranks {
				if rank.Queue == queue {
					result = append(result, rank)
					state = core.FieldFreshness{State: "ready", FetchedAt: &now}
					found = true
				}
			}
		}
		if !found && !changedSeason {
			for _, rank := range old.Ranks {
				if rank.Queue == queue {
					result = append(result, rank)
					state = old.Fields[key]
					if state.FetchedAt == nil {
						state = old.Fields["rank"]
					}
					if requested {
						state.State = "retained"
					}
				}
			}
		}
		next.Fields[key] = state
	}
	for _, rank := range next.Ranks {
		if rank.Queue != "RANKED_SOLO_5x5" && rank.Queue != "RANKED_FLEX_SR" {
			result = append(result, rank)
		}
	}
	next.Ranks = result
}

// A partial rank must not re-download an already fresh avatar and level.
func readMissingProfile(ctx context.Context, client Client, profiles source.ProfileSource, player core.Participant, onlyMissing bool) (core.Profile, []string) {
	p := player.Profile
	if onlyMissing && basicProfileComplete(p) && p.UpdatedAt != nil && time.Since(*p.UpdatedAt) < 5*time.Minute && (p.PrivacyState == "public" || p.PrivacyState == "private") {
		if ranks, ok := client.(source.RankSource); ok {
			return ranks.Ranks(ctx, player.PUUID), []string{"rank"}
		}
	}
	return profiles.Profile(ctx, player.PUUID), nil
}
