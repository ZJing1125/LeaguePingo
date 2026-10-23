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
	"time"
)

func (s *TencentLCU) Profile(ctx context.Context, puuid string) (out core.Profile) {
	basicCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	out = s.BasicProfile(basicCtx, puuid)
	cancel()
	if ctx.Err() != nil {
		out.RankState, out.RankMessage = "failed", "玩家资料查询超时，可刷新重试"
		return out
	}
	s.loadRanks(ctx, puuid, &out)
	if ctx.Err() == nil {
		masteryCtx, stop := context.WithTimeout(ctx, 300*time.Millisecond)
		items, at, err := s.championMasteries(masteryCtx, puuid)
		stop()
		if err == nil {
			out.Masteries, out.MasteryFetchedAt = items, &at
		}
	}
	return out
}

func (s *TencentLCU) BasicProfile(ctx context.Context, puuid string) (out core.Profile) {
	out = core.Profile{RankState: "not-requested", PrivacyState: "unknown", Ranks: []core.Rank{}}
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

func (s *TencentLCU) Ranks(ctx context.Context, puuid string) (out core.Profile) {
	out = core.Profile{RankState: "not-requested", PrivacyState: "unknown", Ranks: []core.Rank{}}
	if !core.IsKnownPUUID(puuid) {
		out.RankState, out.RankMessage = "unavailable", "客户端未提供有效身份，无法查询段位"
		return
	}
	s.loadRanks(ctx, puuid, &out)
	return
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
