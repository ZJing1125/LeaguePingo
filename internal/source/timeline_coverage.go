package source

import (
	"context"
	"leaguepingo/internal/core"
	"sort"
)

func timelineCategory(event core.TimelineEvent) string {
	switch event.Type {
	case "ITEM_PURCHASED", "ITEM_SOLD", "ITEM_DESTROYED", "ITEM_UNDO":
		return "items"
	case "SKILL_LEVEL_UP":
		return "skills"
	}
	return ""
}
func timelineTarget(detail core.MatchDetail) int {
	for _, p := range detail.Players {
		if p.Target {
			return p.ParticipantID
		}
	}
	return 0
}
func timelineCounts(t core.MatchTimeline, target int) map[string]int {
	counts := map[string]int{}
	for _, e := range t.Events {
		if e.ParticipantID == target {
			counts[timelineCategory(e)]++
			if e.Type == "ITEM_PURCHASED" {
				counts["purchases"]++
			}
		}
	}
	return counts
}
func timelineNeedsEvents(t core.MatchTimeline, d core.MatchDetail) bool {
	for _, p := range d.Players {
		if p.Target && p.Summary.Duration != nil && *p.Summary.Duration >= 180 {
			counts := timelineCounts(t, p.ParticipantID)
			return counts["purchases"] == 0 || counts["skills"] == 0
		}
	}
	return false
}

// Replace a whole transaction/skill sequence, keeping one source's ordering.
func mergeTimelineEvents(local, remote core.MatchTimeline, d core.MatchDetail) core.MatchTimeline {
	target := timelineTarget(d)
	before, after := timelineCounts(local, target), timelineCounts(remote, target)
	replace := map[string]bool{}
	for _, key := range []string{"items", "skills"} {
		replace[key] = after[key] > before[key]
	}
	replace["items"] = after["purchases"] > before["purchases"] || after["purchases"] == before["purchases"] && after["items"] > before["items"]
	events := []core.TimelineEvent{}
	for _, e := range local.Events {
		if e.ParticipantID != target || !replace[timelineCategory(e)] {
			events = append(events, e)
		}
	}
	for _, e := range remote.Events {
		if e.ParticipantID == target && replace[timelineCategory(e)] {
			events = append(events, e)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	local.Events = events
	local.Partial = local.Partial || timelineNeedsEvents(local, d)
	return local
}
func (s *TencentLCU) timelineWithRunes(ctx context.Context, t core.MatchTimeline, d core.MatchDetail, target string) core.MatchTimeline {
	for i, p := range d.Players {
		if !p.Target {
			continue
		}
		shards := p.Summary.RuneShards
		if len(shards) == 3 && shards[0] > 0 && shards[1] > 0 && shards[2] > 0 {
			return t
		}
		wasPartial := t.Partial
		t.Partial = true
		raw, platform, err := s.sgpGameData(ctx, d.GameID, "SUMMARY", "/sgp-detail")
		if err != nil {
			return t
		}
		richer, err := core.ParseSGPMatchDetail(raw, d.GameID, target, platform)
		if err != nil {
			return t
		}
		for _, next := range richer.Players {
			if next.Target && next.ParticipantID == p.ParticipantID && next.TeamID == p.TeamID && next.Summary.ChampionID == p.Summary.ChampionID {
				if len(shards) != 3 {
					shards = make([]int, 3)
				} else {
					shards = append([]int(nil), shards...)
				}
				if len(next.Summary.RuneShards) == 3 {
					for slot, value := range next.Summary.RuneShards {
						if shards[slot] == 0 {
							shards[slot] = value
						}
					}
				}
				d.Players = append([]core.MatchPlayer(nil), d.Players...)
				d.Players[i].Summary.RuneShards = shards
				t.Detail = &d
				if shards[0] > 0 && shards[1] > 0 && shards[2] > 0 {
					t.Partial = wasPartial
				}
			}
		}
	}
	return t
}
