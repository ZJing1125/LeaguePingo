package source

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

type rankedQueue struct {
	Queue    string  `json:"queueType"`
	Tier     *string `json:"tier"`
	Division string  `json:"division"`
	LP       *int    `json:"leaguePoints"`
	Wins     *int    `json:"wins"`
	Losses   *int    `json:"losses"`
}

func (s *TencentLCU) loadRanks(ctx context.Context, puuid string, out *core.Profile) {
	out.RankState, out.RankMessage = "unavailable", "客户端未返回可核验段位，不能据此判断未定级"
	raw, err := s.client.Get(ctx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(puuid))
	if err != nil {
		out.RankState, out.RankMessage = "failed", "段位查询失败，可重试"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			out.RankMessage = "段位查询超时，可重试"
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			out.RankMessage = "段位查询已取消，可重试"
		}
		return
	}
	var response struct {
		PUUID    string                 `json:"puuid"`
		Queues   json.RawMessage        `json:"queues"`
		QueueMap map[string]rankedQueue `json:"queueMap"`
	}
	if json.Unmarshal(raw, &response) != nil {
		out.RankState, out.RankMessage = "failed", "客户端段位响应格式暂不兼容，可重试"
		return
	}
	if response.PUUID != "" && response.PUUID != puuid {
		out.RankState, out.RankMessage = "failed", "段位返回身份不匹配，已忽略本次结果"
		return
	}
	var queues []rankedQueue
	if len(response.Queues) > 0 && string(response.Queues) != "null" {
		if json.Unmarshal(response.Queues, &queues) != nil {
			var mapped map[string]rankedQueue
			if json.Unmarshal(response.Queues, &mapped) != nil {
				out.RankState, out.RankMessage = "failed", "客户端段位响应格式暂不兼容，可重试"
				return
			}
			for key, q := range mapped {
				if q.Queue == "" {
					q.Queue = key
				}
				if q.Queue == key {
					queues = append(queues, q)
				}
			}
		}
	}
	// Both layouts are used by client versions. Map keys never override an
	// explicit, conflicting queueType, and only the two supported queues survive.
	for key, q := range response.QueueMap {
		if q.Queue == "" {
			q.Queue = key
		}
		if q.Queue == key {
			queues = append(queues, q)
		}
	}
	seen := map[string]bool{}
	// An empty response is not proof that the player is unranked.
	for _, q := range queues {
		if q.Queue != "RANKED_SOLO_5x5" && q.Queue != "RANKED_FLEX_SR" {
			continue
		}
		if q.Tier == nil || !validTier(*q.Tier) {
			continue
		}
		if seen[q.Queue] {
			continue
		}
		seen[q.Queue] = true
		out.Ranks = append(out.Ranks, core.Rank{Queue: q.Queue, Tier: *q.Tier, Division: q.Division, LP: validCount(q.LP), Wins: validCount(q.Wins), Losses: validCount(q.Losses)})
	}
	if len(out.Ranks) > 0 {
		out.RankState, out.RankMessage = "ready", ""
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
