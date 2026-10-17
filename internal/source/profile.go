package source

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"leaguepingo/internal/core"
	"net/http"
	"net/url"
)

func (s *TencentLCU) Profile(ctx context.Context, puuid string) (out core.Profile) {
	out = s.BasicProfile(ctx, puuid)
	if ctx.Err() != nil {
		return out
	}
	s.loadRanks(ctx, puuid, &out)
	return out
}

func (s *TencentLCU) BasicProfile(ctx context.Context, puuid string) (out core.Profile) {
	out = core.Profile{RankState: "unavailable", PrivacyState: "unknown", Ranks: []core.Rank{}}
	// This optional local asset shares the explicit profile request's budget.
	// Never load profile pictures from the game loop or arbitrary remote URLs.
	defer func() {
		if out.ProfileIconID != nil && ctx.Err() == nil {
			out.ProfileIconData = s.profileIcon(ctx, *out.ProfileIconID)
		}
	}()
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
	return out
}

func (s *TencentLCU) loadRanks(ctx context.Context, puuid string, out *core.Profile) {
	raw, err := s.client.Get(ctx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(puuid))
	if err != nil {
		return
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
		return
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
}

func (s *TencentLCU) profileIcon(ctx context.Context, id int) string {
	if id < 0 {
		return ""
	}
	raw, err := s.client.Get(ctx, fmt.Sprintf("/lol-game-data/assets/v1/profile-icons/%d.jpg", id))
	if err != nil || len(raw) > 256<<10 {
		return ""
	}
	mime := http.DetectContentType(raw)
	if mime != "image/jpeg" && mime != "image/png" {
		return ""
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 512 || config.Height > 512 {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
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
