package core

import (
	"math"
	"sort"
	"time"
)

const PlayerAnalysisVersion = 1

type PlayerAnalysis struct {
	Version       int                 `json:"version"`
	State         string              `json:"state"`
	Grade         string              `json:"grade,omitempty"`
	Score         *float64            `json:"score"`
	Games         int                 `json:"games"`
	ScoredGames   int                 `json:"scoredGames"`
	ComparedGames int                 `json:"comparedGames"`
	Basis         string              `json:"basis"`
	Provisional   bool                `json:"provisional"`
	Confidence    string              `json:"confidence"`
	Metrics       []PerformanceMetric `json:"metrics"`
}

func analysisGrade(score float64) string {
	switch {
	case score >= 8.5:
		return "S"
	case score >= 7:
		return "A"
	case score >= 5:
		return "B"
	case score >= 3:
		return "C"
	default:
		return "D"
	}
}

// Grades are this assistant's versioned heuristic for recent performance,
// not a percentile, a prediction or the current live match's score.
func BuildPlayerAnalysis(matches []Match, queue int, cutoff time.Time, complete bool) *PlayerAnalysis {
	mode, ok := ScoutModeFor(queue)
	if !ok {
		return nil
	}
	out := &PlayerAnalysis{Version: PlayerAnalysisVersion, State: "insufficient", Basis: "basic", Confidence: "low", Provisional: true, Metrics: []PerformanceMetric{}}
	total, weights, squared := 0.0, 0.0, 0.0
	metrics, metricWeights := map[string]PerformanceMetric{}, map[string]float64{}
	seen := map[int64]bool{}
	uncertainPosition := false
	for _, m := range matches {
		if m.GameID <= 0 || seen[m.GameID] || m.QueueID != queue || m.PlayedAt == nil || m.PlayedAt.After(cutoff) || m.PlayedAt.Before(cutoff.Add(-30*24*time.Hour)) || !mode.Matches(valueOrZero(m.MapID), "") {
			continue
		}
		seen[m.GameID] = true
		out.Games++
		if IsRemake(m) {
			continue
		}
		score, components, compared, valid := analysisMatch(m, mode)
		if !valid {
			continue
		}
		weight := math.Exp(-math.Ln2 * cutoff.Sub(*m.PlayedAt).Hours() / (7 * 24))
		total, weights, squared = total+score*weight, weights+weight, squared+weight*weight
		out.ScoredGames++
		if compared {
			if mode.Roles && m.Performance != nil && m.Performance.Basis != "position" {
				uncertainPosition = true
			}
			out.ComparedGames++
		}
		for _, component := range components {
			old := metrics[component.Key]
			old.Key, old.Score, old.Games = component.Key, old.Score+component.Score*weight, old.Games+1
			metrics[component.Key], metricWeights[component.Key] = old, metricWeights[component.Key]+weight
		}
	}
	if out.ScoredGames == 0 {
		return out
	}
	effective := weights * weights / squared
	trust := effective / (effective + 6)
	score := math.Round((trust*total/weights+(1-trust)*5)*10) / 10
	out.Score, out.Grade, out.State = &score, analysisGrade(score), "ready"
	if mode.Arena {
		out.Basis = "placement"
	} else if out.ComparedGames == out.ScoredGames {
		out.Basis = "comparative"
	} else if out.ComparedGames > 0 {
		out.Basis = "mixed"
	}
	out.Provisional = out.ScoredGames < 5 || !complete || uncertainPosition || !mode.Arena && out.ComparedGames*2 < out.ScoredGames
	if !out.Provisional {
		out.Confidence = "medium"
	}
	if !out.Provisional && out.ScoredGames >= 20 && effective >= 8 && (mode.Arena || out.ComparedGames*5 >= out.ScoredGames*4) {
		out.Confidence = "high"
	}
	for key, value := range metrics {
		value.Score = math.Round(value.Score/metricWeights[key]*10) / 10
		out.Metrics = append(out.Metrics, value)
	}
	sort.Slice(out.Metrics, func(i, j int) bool { return out.Metrics[i].Key < out.Metrics[j].Key })
	return out
}

func analysisMatch(m Match, mode ScoutMode) (float64, []PerformanceMetric, bool, bool) {
	if mode.Arena {
		if m.Arena == nil || m.Arena.TeamCount == nil || *m.Arena.TeamCount < 2 || *m.Arena.TeamCount > 32 || m.Arena.Placement <= 0 || m.Arena.Placement > *m.Arena.TeamCount {
			return 0, nil, false, false
		}
		score := float64(*m.Arena.TeamCount-m.Arena.Placement) / float64(*m.Arena.TeamCount-1) * 10
		return score, []PerformanceMetric{{Key: "placement", Score: score, Games: 1}}, true, true
	}
	if p := m.Performance; p != nil && p.Version == PerformanceVersion && len(p.Metrics) >= 3 && !math.IsNaN(p.Score) && !math.IsInf(p.Score, 0) && p.Score >= 0 && p.Score <= 10 {
		score := p.Score
		if m.Win != nil {
			if *m.Win {
				score = .9*score + 1
			} else {
				score *= .9
			}
		}
		return score, p.Metrics, true, true
	}
	return basicAnalysisMatch(m, mode)
}

// Missing full-game comparisons use an explicitly provisional basic score.
// Anchors are documented initial rule parameters, not sampled population means.
func basicAnalysisMatch(m Match, mode ScoutMode) (float64, []PerformanceMetric, bool, bool) {
	var parts []PerformanceMetric
	total, weight := 0.0, 0.0
	if m.Kills != nil && m.Deaths != nil && m.Assists != nil && *m.Kills >= 0 && *m.Deaths >= 0 && *m.Assists >= 0 {
		kda := (float64(*m.Kills) + float64(*m.Assists)) / float64(max(1, *m.Deaths))
		anchor := 2.5
		if mode.MapID == 12 {
			anchor = 3
		}
		score := 10 * kda / (kda + anchor)
		parts = append(parts, PerformanceMetric{Key: "kda", Score: score, Games: 1})
		total, weight = .4*score, .4
	}
	if m.Win != nil {
		score := 0.0
		if *m.Win {
			score = 10
		}
		parts = append(parts, PerformanceMetric{Key: "outcome", Score: score, Games: 1})
		total, weight = total+.6*score, weight+.6
	}
	if weight == 0 {
		return 0, nil, false, false
	}
	return total / weight, parts, false, true
}
