package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"leaguepingo/internal/core"
)

func (s *TencentLCU) ScoutSeason(ctx context.Context) (*core.ScoutSeason, error) {
	// A cold start directly in champion select may not have visited the lobby.
	// Probe queue identity once with this bounded preparation, not on UI renders.
	if s.CurrentQueue() == 0 {
		if session, err := s.client.Get(ctx, "/lol-gameflow/v1/session"); err == nil {
			s.rememberQueue(session, false)
		}
	}
	raw, err := s.client.Get(ctx, "/lol-seasons/v1/season/product/LOL")
	if err != nil {
		return nil, err
	}
	return parseScoutSeason(raw, time.Now())
}
func parseScoutSeason(raw []byte, now time.Time) (*core.ScoutSeason, error) {
	type season struct {
		Start    int64 `json:"seasonStart"`
		End      int64 `json:"seasonEnd"`
		Metadata struct {
			Name string `json:"publicName"`
			Year int    `json:"year"`
		} `json:"metadata"`
	}
	var list []season
	if json.Unmarshal(raw, &list) != nil {
		var one season
		if json.Unmarshal(raw, &one) != nil {
			return nil, errors.New("赛季范围暂未取得")
		}
		list = []season{one}
	}
	var result *core.ScoutSeason
	for _, s := range list {
		start, end := s.Start, s.End
		if start > 0 && start < 100000000000 {
			start *= 1000
		}
		if end > 0 && end < 100000000000 {
			end *= 1000
		}
		a, b := time.UnixMilli(start), time.UnixMilli(end)
		if start <= 0 || end <= start || now.Before(a) || !now.Before(b) {
			continue
		}
		name := s.Metadata.Name
		if name == "" && s.Metadata.Year > 2000 {
			name = fmt.Sprintf("%d 赛季", s.Metadata.Year)
		}
		if name == "" {
			name = "当前赛季"
		}
		if result == nil || a.After(result.Start) {
			result = &core.ScoutSeason{Name: name, Start: a, End: b}
		}
	}
	if result == nil {
		return nil, errors.New("赛季范围暂未取得")
	}
	return result, nil
}

func (s *TencentLCU) ChampionMasteries(ctx context.Context, puuid string) ([]core.ChampionMastery, error) {
	if !core.IsKnownPUUID(puuid) {
		return nil, errors.New("身份未提供")
	}
	raw, err := s.client.Get(ctx, "/lol-champion-mastery/v1/"+url.PathEscape(puuid)+"/champion-mastery")
	if err != nil {
		return nil, err
	}
	var items []struct {
		PUUID  string `json:"puuid"`
		ID     int    `json:"championId"`
		Level  *int   `json:"championLevel"`
		Points *int   `json:"championPoints"`
	}
	if json.Unmarshal(raw, &items) != nil || items == nil || len(items) > 1000 {
		return nil, errors.New("英雄经验格式暂不兼容")
	}
	out := []core.ChampionMastery{}
	seen := map[int]bool{}
	for _, m := range items {
		if m.PUUID != "" && m.PUUID != puuid {
			return nil, errors.New("英雄经验身份不匹配")
		}
		if m.ID > 0 && !seen[m.ID] && m.Level != nil && m.Points != nil && *m.Level >= 0 && *m.Points >= 0 {
			out = append(out, core.ChampionMastery{ChampionID: m.ID, Level: *m.Level, Points: *m.Points})
			seen[m.ID] = true
		}
	}
	return out, nil
}

func (s *TencentLCU) ScoutPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	if !core.IsKnownPUUID(puuid) || offset < 0 || offset >= 20000 {
		return core.HistoryPage{}, errors.New("对局统计范围无效")
	}
	// This dedicated cursor is not the user-facing 20/50-game analysis sample.
	// Do not fall back to a repeated LCU prefix and call it season history.
	return s.sgpSummaryPage(ctx, puuid, offset)
}
