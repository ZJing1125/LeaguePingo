package core

import (
	_ "embed"
	"encoding/json"
	"math"
	"strconv"
)

const PerformanceVersion = 1

type PerformanceMetric struct {
	Key   string  `json:"key"`
	Score float64 `json:"score"`
	Games int     `json:"games"`
}

// Derived, non-identifying comparison; never a copy of the other players' stats.
type MatchPerformance struct {
	Horse   *HorsePerformance   `json:"horse,omitempty"`
	Version int                 `json:"version"`
	Score   float64             `json:"score"`
	Basis   string              `json:"basis"`
	Metrics []PerformanceMetric `json:"metrics"`
}

// Riot Data Dragon 16.20.1, same version as packaged artwork. A champion class
// is a comparison cohort, never evidence of the player's actual Rift lane.
//
//go:embed champion_roles.json
var championRolesJSON []byte

var championRoles = func() map[string]string {
	var roles map[string]string
	if err := json.Unmarshal(championRolesJSON, &roles); err != nil {
		panic(err)
	}
	return roles
}()

func performanceCohort(game historyGame, target historyParticipant, players []historyParticipant) ([]historyParticipant, string, string) {
	mode, _ := ScoutModeFor(game.QueueID)
	role := historyPosition(game, target)
	class := championRoles[strconv.Itoa(target.ChampionID)]
	var cohort []historyParticipant
	basis := "class"
	if mode.Roles && role != "" {
		basis = "position"
		for _, p := range players {
			if historyPosition(game, p) == role {
				cohort = append(cohort, p)
			}
		}
		if len(cohort) == 2 && cohort[0].TeamID != cohort[1].TeamID {
			return cohort, basis, role
		}
	}
	cohort = nil
	if class == "" {
		return nil, "", ""
	}
	for _, p := range players {
		if championRoles[strconv.Itoa(p.ChampionID)] == class {
			cohort = append(cohort, p)
		}
	}
	if len(cohort) < 2 {
		return nil, "", ""
	}
	return cohort, "class", class
}

func matchPerformance(game historyGame, target historyParticipant) *MatchPerformance {
	mode, ok := ScoutModeFor(game.QueueID)
	if !ok || mode.Arena || !mode.Matches(valueOrZero(game.MapID), "") || len(game.Participants) != 10 || target.Stats.EarlyEnd != nil && *target.Stats.EarlyEnd && game.Duration != nil && *game.Duration <= 300 {
		return nil
	}
	players, seen, counts := []historyParticipant{}, map[int]bool{}, map[int]int{}
	for _, raw := range game.Participants {
		var p historyParticipant
		if json.Unmarshal(raw, &p) != nil {
			return nil
		}
		normalizeHistoryParticipant(&p)
		if p.ChampionID == 0 {
			p.ChampionID = p.Stats.ChampionID
		}
		if p.ParticipantID <= 0 || seen[p.ParticipantID] || p.TeamID != 100 && p.TeamID != 200 {
			return nil
		}
		seen[p.ParticipantID] = true
		counts[p.TeamID]++
		players = append(players, p)
	}
	if counts[100] != 5 || counts[200] != 5 {
		return nil
	}
	if target.ChampionID == 0 {
		target.ChampionID = target.Stats.ChampionID
	}
	cohort, basis, role := performanceCohort(game, target, players)
	if len(cohort) < 2 {
		return nil
	}
	weights := performanceWeights(role, mode.Roles)
	out := MatchPerformance{Version: PerformanceVersion, Basis: basis, Horse: horsePerformance(game, target, players, cohort, role, basis)}
	total, used := 0.0, 0.0
	for _, key := range []string{"damage", "gold", "participation", "survival", "cs", "vision", "taken", "healing", "shielding"} {
		weight := weights[key]
		if weight == 0 {
			continue
		}
		value, valid := performanceValue(target, players, key)
		mean, complete := performanceMean(cohort, players, key)
		if !valid || !complete || mean <= 0 {
			continue
		}
		score := math.Max(0, math.Min(10, value/mean*5))
		out.Metrics = append(out.Metrics, PerformanceMetric{Key: key, Score: score, Games: 1})
		total, used = total+score*weight, used+weight
	}
	if (len(out.Metrics) < 3 || used < .5) && out.Horse == nil {
		return nil
	}
	if used > 0 {
		out.Score = total / used
	}
	return &out
}

func performanceWeights(role string, lanes bool) map[string]float64 {
	if role == "UTILITY" || role == "Support" {
		if lanes {
			return map[string]float64{"damage": .1, "gold": .15, "participation": .25, "survival": .2, "vision": .3}
		}
		return map[string]float64{"damage": .1, "gold": .15, "participation": .25, "survival": .2, "healing": .15, "shielding": .15}
	}
	if role == "Tank" {
		return map[string]float64{"damage": .15, "gold": .15, "participation": .2, "survival": .15, "taken": .35}
	}
	if lanes {
		return map[string]float64{"damage": .25, "gold": .2, "participation": .2, "survival": .2, "cs": .1, "vision": .05}
	}
	return map[string]float64{"damage": .3, "gold": .25, "participation": .25, "survival": .2}
}

func performanceMean(cohort, players []historyParticipant, key string) (float64, bool) {
	total := 0.0
	for _, p := range cohort {
		v, ok := performanceValue(p, players, key)
		if !ok {
			return 0, false
		}
		total += v
	}
	return total / float64(len(cohort)), true
}

func performanceValue(p historyParticipant, players []historyParticipant, key string) (float64, bool) {
	value := func(n *int) (float64, bool) {
		if n == nil || *n < 0 {
			return 0, false
		}
		return float64(*n), true
	}
	s := p.Stats
	switch key {
	case "damage":
		return value(s.Damage)
	case "gold":
		return value(s.Gold)
	case "vision":
		return value(s.VisionScore)
	case "taken":
		return value(s.DamageTaken)
	case "healing":
		return value(s.Healing)
	case "shielding":
		return value(s.Shielding)
	case "cs":
		if s.Minions != nil && s.Neutral != nil && *s.Minions >= 0 && *s.Neutral >= 0 {
			return float64(*s.Minions) + float64(*s.Neutral), true
		}
	case "survival":
		if s.Deaths != nil && *s.Deaths >= 0 {
			return 1 / (float64(*s.Deaths) + 1), true
		}
	case "participation":
		if s.Kills == nil || s.Assists == nil || *s.Kills < 0 || *s.Assists < 0 {
			return 0, false
		}
		total := 0.0
		for _, peer := range players {
			if peer.TeamID != p.TeamID {
				continue
			}
			if peer.Stats.Kills == nil || *peer.Stats.Kills < 0 {
				return 0, false
			}
			total += float64(*peer.Stats.Kills)
		}
		participation := float64(*s.Kills) + float64(*s.Assists)
		if total > 0 && participation <= total {
			return participation / total, true
		}
	}
	return 0, false
}
