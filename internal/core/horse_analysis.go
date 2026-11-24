package core

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AnalysisGame stores only the target's bounded evidence. It carries no peer
// identifiers and does not authorize detail requests for history rows.
type AnalysisGame struct {
	GameID          int64    `json:"gameId"`
	ChampionID      int      `json:"championId"`
	TeamID          int      `json:"teamId,omitempty"`
	KDA             float64  `json:"kda"`
	Deaths          int      `json:"deaths"`
	DeathsPerTen    *float64 `json:"deathsPerTen,omitempty"`
	Win             *bool    `json:"win,omitempty"`
	Participation   *float64 `json:"participation,omitempty"`
	DamagePerMinute *float64 `json:"damagePerMinute,omitempty"`
	EfficiencyRatio *float64 `json:"efficiencyRatio,omitempty"`
}

func HorseTier(score float64) string {
	switch {
	case score >= 85:
		return "千里马"
	case score >= 72:
		return "上等马"
	case score >= 55:
		return "中等马"
	case score >= 40:
		return "下等马"
	default:
		return "纯牛马"
	}
}

func horseMatches(matches []Match, mode ScoutMode, cutoff time.Time) []Match {
	out := []Match{}
	seen := map[int64]bool{}
	ordered := append([]Match(nil), matches...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].PlayedAt == nil {
			return false
		}
		return ordered[j].PlayedAt == nil || ordered[i].PlayedAt.After(*ordered[j].PlayedAt)
	})
	for _, m := range ordered {
		if m.GameID <= 0 || seen[m.GameID] || m.QueueID != mode.QueueID || m.PlayedAt == nil || m.PlayedAt.After(cutoff) || m.PlayedAt.Before(cutoff.Add(-30*24*time.Hour)) || !mode.Matches(valueOrZero(m.MapID), "") || IsRemake(m) {
			continue
		}
		seen[m.GameID] = true
		if m.Kills == nil || m.Deaths == nil || m.Assists == nil || *m.Kills < 0 || *m.Deaths < 0 || *m.Assists < 0 {
			continue
		}
		out = append(out, m)
		if len(out) == horseRules.MaximumGames {
			break
		}
	}
	return out
}

func analysisEvidence(m Match) AnalysisGame {
	evidence := AnalysisGame{GameID: m.GameID, ChampionID: m.ChampionID, KDA: float64(*m.Kills+*m.Assists) / float64(max(1, *m.Deaths)), Deaths: *m.Deaths, Win: m.Win, Participation: m.Participation}
	if m.Duration != nil && *m.Duration > 0 {
		v := float64(*m.Deaths) * 600 / float64(*m.Duration)
		evidence.DeathsPerTen = &v
		if m.Damage != nil && *m.Damage >= 0 {
			dpm := float64(*m.Damage) * 60 / float64(*m.Duration)
			evidence.DamagePerMinute = &dpm
		}
	}
	for _, peer := range m.Roster {
		if peer.Target {
			evidence.TeamID = peer.TeamID
			break
		}
	}
	if m.Performance != nil && m.Performance.Horse != nil {
		evidence.EfficiencyRatio = m.Performance.Horse.EfficiencyRatio
	}
	return evidence
}

func buildHorseAnalysis(matches []Match, mode ScoutMode, cutoff time.Time, complete bool) *PlayerAnalysis {
	out := &PlayerAnalysis{Version: 3, State: "insufficient", Basis: "basic", Confidence: "low", Provisional: true, Metrics: []PerformanceMetric{}, Recent: []AnalysisGame{}, Tags: []string{}}
	selected := horseMatches(matches, mode, cutoff)
	totals, weights := map[string]float64{}, map[string]float64{}
	counts := map[string]int{}
	score, totalWeight := 0.0, 0.0
	uncertainPosition := false
	for i, m := range selected {
		evidence := analysisEvidence(m)
		out.Recent = append(out.Recent, evidence)
		weight := math.Max(.8, horseRules.RecentWeight-float64(i)*horseRules.WeightStep)
		anchor := horseRules.KDAAnchorRift
		if !mode.Roles {
			anchor = horseRules.KDAAnchorBrawl
		}
		value := clampScore(evidence.KDA/anchor*horseRules.Baseline, 0, 100)
		metrics := []PerformanceMetric{{Key: "kda", Score: value, Games: 1}}
		if m.Performance != nil && m.Performance.Horse != nil && m.Performance.Horse.Version == 2 {
			h := m.Performance.Horse
			if !math.IsNaN(h.Score) && !math.IsInf(h.Score, 0) && h.Score >= 0 && h.Score <= 100 {
				value = h.Score
				metrics = h.Metrics
				out.ComparedGames++
				if mode.Roles && m.Performance.Basis != "position" {
					uncertainPosition = true
				}
			}
		}
		score += value * weight
		totalWeight += weight
		for _, metric := range metrics {
			totals[metric.Key] += metric.Score * weight
			weights[metric.Key] += weight
			counts[metric.Key]++
		}
	}
	out.Games = len(selected)
	out.ScoredGames = len(selected)
	if len(selected) == 0 {
		if len(matches) == 0 && complete {
			out.State = "empty"
		}
		return out
	}
	value := math.Round(score/totalWeight*10) / 10
	out.Score = &value
	out.State = "sample"
	if len(selected) >= horseRules.MinimumGames {
		out.State = "ready"
	}
	if out.ComparedGames == out.ScoredGames {
		out.Basis = "comparative"
	} else if out.ComparedGames > 0 {
		out.Basis = "mixed"
	}
	out.Provisional = out.State != "ready" || !complete || uncertainPosition || out.ComparedGames*2 < out.ScoredGames
	if !out.Provisional {
		out.Confidence = "medium"
		if len(selected) == 15 && out.ComparedGames == 15 {
			out.Confidence = "high"
		}
	}
	for _, key := range []string{"dpm", "eff", "takenEff", "kda", "participation", "vision", "goldDiff"} {
		if weights[key] > 0 {
			out.Metrics = append(out.Metrics, PerformanceMetric{Key: key, Score: math.Round(totals[key]/weights[key]*10) / 10, Games: counts[key]})
		}
	}
	out.Tags = horseFormTags(out.Recent)
	return out
}

func horseFormTags(games []AnalysisGame) []string {
	tags := []string{}
	if len(games) == 0 {
		return tags
	}
	streak := 0
	if games[0].Win != nil {
		for _, g := range games[:min(5, len(games))] {
			if g.Win == nil || *g.Win != *games[0].Win {
				break
			}
			streak++
		}
	}
	if streak >= 4 && *games[0].Win {
		tags = append(tags, fmt.Sprintf("%d连胜", streak))
	}
	if streak >= 3 && !*games[0].Win {
		tags = append(tags, fmt.Sprintf("%d连败", streak))
	}
	if len(games) < 15 {
		return tags
	}
	recent, previous, rateRecent, ratePrevious := 0.0, 0.0, 0.0, 0.0
	for i, g := range games {
		if g.DeathsPerTen == nil {
			return tags
		}
		if i < 5 {
			recent += g.KDA / 5
			rateRecent += *g.DeathsPerTen / 5
		} else {
			previous += g.KDA / 10
			ratePrevious += *g.DeathsPerTen / 10
		}
	}
	delta := (recent - previous) / math.Max(.1, previous)
	if delta <= -.4 && rateRecent >= ratePrevious*1.3 && rateRecent-ratePrevious >= .5 {
		tags = append([]string{"近期死亡上升"}, tags...)
	}
	return tags
}
