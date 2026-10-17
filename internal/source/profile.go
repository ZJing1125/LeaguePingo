package source

import (
	"context"
	"encoding/json"
	"leaguepingo/internal/core"
	"net/url"
)

func (s *TencentLCU) Profile(ctx context.Context, puuid string) core.Profile {
	out := core.Profile{RankState: "unavailable", PrivacyState: "unknown", Ranks: []core.Rank{}}
	if raw, err := s.client.Get(ctx, "/lol-summoner/v2/summoners/puuid/"+url.PathEscape(puuid)); err == nil {
		var p struct {
			PUUID   string `json:"puuid"`
			Level   *int   `json:"summonerLevel"`
			Privacy string `json:"privacy"`
			Icon    *int   `json:"profileIconId"`
		}
		if json.Unmarshal(raw, &p) == nil && p.PUUID == puuid {
			out.Level = validCount(p.Level)
			out.ProfileIconID = validCount(p.Icon)
			switch p.Privacy {
			case "PUBLIC":
				out.PrivacyState = "public"
			case "PRIVATE":
				out.PrivacyState = "private"
			}
		}
	}
	raw, err := s.client.Get(ctx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(puuid))
	if err != nil {
		return out
	}
	var response struct {
		Queues []struct {
			Queue    string  `json:"queueType"`
			Tier     *string `json:"tier"`
			Division string  `json:"division"`
			LP       *int    `json:"leaguePoints"`
			Wins     *int    `json:"wins"`
			Losses   *int    `json:"losses"`
		} `json:"queues"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Queues == nil {
		return out
	}
	// An empty response is not proof that the player is unranked.
	for _, q := range response.Queues {
		if q.Queue != "RANKED_SOLO_5x5" && q.Queue != "RANKED_FLEX_SR" {
			continue
		}
		if q.Tier == nil || !validTier(*q.Tier) {
			continue
		}
		out.Ranks = append(out.Ranks, core.Rank{Queue: q.Queue, Tier: *q.Tier, Division: q.Division, LP: validCount(q.LP), Wins: validCount(q.Wins), Losses: validCount(q.Losses)})
	}
	if len(out.Ranks) > 0 {
		out.RankState = "ready"
	}
	return out
}
func validCount(p *int) *int {
	if p != nil && *p < 0 {
		return nil
	}
	return p
}
func validTier(s string) bool {
	switch s {
	case "NONE", "UNRANKED", "IRON", "BRONZE", "SILVER", "GOLD", "PLATINUM", "EMERALD", "DIAMOND", "MASTER", "GRANDMASTER", "CHALLENGER":
		return true
	}
	return false
}
