package core

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed horse_rules.json
var horseRulesJSON []byte
var horseRules = func() struct {
	Version                                                            int
	Baseline, RatioSlope, GoldDiffSlope, KDAAnchorRift, KDAAnchorBrawl float64
	MinimumGames, MaximumGames                                         int
	RecentWeight, WeightStep                                           float64
} {
	var rules struct {
		Version                                                            int
		Baseline, RatioSlope, GoldDiffSlope, KDAAnchorRift, KDAAnchorBrawl float64
		MinimumGames, MaximumGames                                         int
		RecentWeight, WeightStep                                           float64
	}
	if err := json.Unmarshal(horseRulesJSON, &rules); err != nil {
		panic(err)
	}
	return rules
}()

type HorsePerformance struct {
	Version         int                 `json:"version"`
	Score           float64             `json:"score"`
	Metrics         []PerformanceMetric `json:"metrics"`
	EfficiencyRatio *float64            `json:"efficiencyRatio,omitempty"`
}

func horseWeights(role string, lanes bool) map[string]float64 {
	if !lanes {
		return map[string]float64{"dpm": .45, "kda": .35, "participation": .20}
	}
	switch role {
	case "TOP", "Tank":
		return map[string]float64{"goldDiff": .30, "takenEff": .30, "kda": .25, "participation": .15}
	case "JUNGLE":
		return map[string]float64{"participation": .35, "kda": .30, "goldDiff": .20, "eff": .15}
	case "UTILITY", "Support":
		return map[string]float64{"participation": .40, "vision": .30, "kda": .30}
	default:
		return map[string]float64{"eff": .40, "kda": .30, "goldDiff": .20, "participation": .10}
	}
}

func horseValue(p historyParticipant, players []historyParticipant, key string, duration *int) (float64, bool) {
	s := p.Stats
	positive := func(v *int) bool { return v != nil && *v >= 0 }
	switch key {
	case "kda":
		if positive(s.Kills) && positive(s.Assists) && positive(s.Deaths) {
			return float64(*s.Kills+*s.Assists) / float64(max(1, *s.Deaths)), true
		}
	case "eff", "takenEff":
		if !positive(s.Gold) || *s.Gold < 1000 || !positive(s.Damage) {
			return 0, false
		}
		damage := float64(*s.Damage)
		if key == "takenEff" {
			if !positive(s.DamageTaken) {
				return 0, false
			}
			damage += float64(*s.DamageTaken)
		}
		return damage / float64(*s.Gold), true
	case "dpm", "vision":
		field := s.Damage
		if key == "vision" {
			field = s.VisionScore
		}
		if positive(field) && duration != nil && *duration > 0 {
			return float64(*field) * 60 / float64(*duration), true
		}
	case "participation":
		return performanceValue(p, players, key)
	}
	return 0, false
}

// Same-position cohorts include the direct opponent. Class fallback excludes
// lane gold differences; it never turns a champion class into a lane assignment.
func horsePerformance(game historyGame, target historyParticipant, players, cohort []historyParticipant, role, basis string) *HorsePerformance {
	mode, _ := ScoutModeFor(game.QueueID)
	weights := horseWeights(role, mode.Roles)
	out := &HorsePerformance{Version: 2, Metrics: []PerformanceMetric{}}
	values := map[string]float64{}
	for _, key := range []string{"dpm", "eff", "takenEff", "kda", "participation", "vision", "goldDiff"} {
		if weights[key] == 0 {
			continue
		}
		if key == "goldDiff" {
			if basis != "position" || game.Duration == nil || *game.Duration <= 0 || target.Stats.Gold == nil || *target.Stats.Gold < 0 {
				continue
			}
			for _, peer := range cohort {
				if peer.TeamID != target.TeamID && peer.Stats.Gold != nil && *peer.Stats.Gold >= 0 {
					values[key] = clampScore(horseRules.Baseline+horseRules.GoldDiffSlope*float64(*target.Stats.Gold-*peer.Stats.Gold)*60/float64(*game.Duration), 0, 100)
				}
			}
			continue
		}
		value, ok := horseValue(target, players, key, game.Duration)
		sum, valid := 0.0, ok
		for _, peer := range cohort {
			v, present := horseValue(peer, players, key, game.Duration)
			valid = valid && present
			sum += v
		}
		if !valid || sum <= 0 {
			continue
		}
		ratio := value / (sum / float64(len(cohort)))
		values[key] = clampScore(horseRules.Baseline+(ratio-1)*horseRules.RatioSlope, 0, 100)
		if key == "eff" {
			out.EfficiencyRatio = &ratio
		}
	}
	// A missing lane comparison redistributes only that metric's weight to Eff/KP.
	if _, ok := values["goldDiff"]; !ok && weights["goldDiff"] > 0 {
		original := weights["eff"] + weights["participation"]
		if original > 0 {
			for _, key := range []string{"eff", "participation"} {
				weights[key] += weights["goldDiff"] * weights[key] / original
			}
		}
		delete(weights, "goldDiff")
	}
	total, used := 0.0, 0.0
	for _, key := range []string{"dpm", "eff", "takenEff", "kda", "participation", "vision", "goldDiff"} {
		if value, ok := values[key]; ok {
			total += value * weights[key]
			used += weights[key]
			out.Metrics = append(out.Metrics, PerformanceMetric{Key: key, Score: value, Games: 1})
		}
	}
	if len(out.Metrics) < 2 || used < .5 {
		return nil
	}
	out.Score = total / used
	return out
}

func clampScore(value, low, high float64) float64 { return math.Max(low, math.Min(high, value)) }
