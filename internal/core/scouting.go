package core

import (
	"sort"
	"time"
)

// Scouting is independent of the recent-history list and its browsing filters.
// Reports are immutable after publication. Partial ranges never imply zero games.
type ScoutSeason struct {
	Name  string    `json:"name"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type ChampionMastery struct {
	ChampionID int `json:"championId"`
	Level      int `json:"level"`
	Points     int `json:"points"`
}
type ScoutHeroQueue struct {
	QueueID int       `json:"queueId"`
	Stats   Stats     `json:"stats"`
	Average []float64 `json:"average"`
}
type ScoutChampion struct {
	ChampionID int              `json:"championId"`
	Games      *int             `json:"games"`
	Mastery    *ChampionMastery `json:"mastery,omitempty"`
	Queues     []ScoutHeroQueue `json:"queues"`
}
type ScoutPosition struct {
	Position string `json:"position"`
	Games    int    `json:"games"`
}
type ScoutQueue struct {
	QueueID   int             `json:"queueId"`
	Season    Stats           `json:"season"`
	Twelve    Stats           `json:"twelve"`
	Month     Stats           `json:"month"`
	Positions []ScoutPosition `json:"positions"`
}
type ScoutReport struct {
	State      string          `json:"state"`
	Retained   bool            `json:"retained,omitempty"`
	FetchedAt  time.Time       `json:"fetchedAt"`
	Season     *ScoutSeason    `json:"season,omitempty"`
	Complete   bool            `json:"complete"`
	Complete12 bool            `json:"complete12"`
	Complete30 bool            `json:"complete30"`
	Read       int             `json:"read"`
	Message    string          `json:"message,omitempty"`
	Champions  []ScoutChampion `json:"champions"`
	Queues     []ScoutQueue    `json:"queues"`
}

func BuildScoutReport(matches []Match, masteries []ChampionMastery, season *ScoutSeason, cutoff time.Time, complete, complete12, complete30 bool) ScoutReport {
	r := ScoutReport{State: "ready", FetchedAt: cutoff, Season: season, Complete: complete && season != nil, Complete12: complete12, Complete30: complete30, Read: len(matches), Champions: []ScoutChampion{}, Queues: []ScoutQueue{}}
	valid, seasonal := []Match{}, []Match{}
	seen := map[int64]bool{}
	ids := map[int]bool{}
	for _, m := range matches {
		if seen[m.GameID] || m.GameID <= 0 {
			continue
		}
		seen[m.GameID] = true
		if m.PlayedAt == nil || m.PlayedAt.After(cutoff) {
			continue
		}
		valid = append(valid, m)
		if season != nil && !m.PlayedAt.Before(season.Start) && m.PlayedAt.Before(season.End) {
			seasonal = append(seasonal, m)
			if m.ChampionID > 0 {
				ids[m.ChampionID] = true
			}
		}
	}
	masteryByID := map[int]ChampionMastery{}
	for _, m := range masteries {
		if m.ChampionID > 0 && m.Level >= 0 && m.Points >= 0 {
			masteryByID[m.ChampionID] = m
			ids[m.ChampionID] = true
		}
	}
	ordered := make([]int, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)
	for _, id := range ordered {
		h := ScoutChampion{ChampionID: id, Queues: []ScoutHeroQueue{}}
		if mastery, ok := masteryByID[id]; ok {
			copy := mastery
			h.Mastery = &copy
		}
		all := []Match{}
		for _, m := range seasonal {
			if m.ChampionID == id {
				all = append(all, m)
			}
		}
		if len(all) > 0 || r.Complete {
			n := len(all)
			h.Games = &n
		}
		for _, q := range []int{420, 440} {
			selected := []Match{}
			var k, d, a float64
			for _, m := range all {
				if m.QueueID == q {
					selected = append(selected, m)
					if !IsRemake(m) && m.Kills != nil && m.Deaths != nil && m.Assists != nil {
						k += float64(*m.Kills)
						d += float64(*m.Deaths)
						a += float64(*m.Assists)
					}
				}
			}
			stats := CalculateStats(selected)
			average := []float64{}
			if stats.KDAGames > 0 {
				n := float64(stats.KDAGames)
				average = []float64{k / n, d / n, a / n}
			}
			h.Queues = append(h.Queues, ScoutHeroQueue{QueueID: q, Stats: stats, Average: average})
		}
		r.Champions = append(r.Champions, h)
	}
	for _, q := range []int{420, 440} {
		s, tw, month := []Match{}, []Match{}, []Match{}
		positions := map[string]int{}
		for _, m := range seasonal {
			if m.QueueID == q {
				s = append(s, m)
			}
		}
		for _, m := range valid {
			if m.QueueID != q {
				continue
			}
			if !m.PlayedAt.Before(cutoff.Add(-12 * time.Hour)) {
				tw = append(tw, m)
			}
			if !m.PlayedAt.Before(cutoff.Add(-30 * 24 * time.Hour)) {
				month = append(month, m)
				if IsRemake(m) {
					continue
				}
				if m.MapID != nil && *m.MapID != 11 {
					continue
				}
				switch m.Position {
				case "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY":
					positions[m.Position]++
				}
			}
		}
		p := []ScoutPosition{}
		for _, role := range []string{"TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"} {
			if positions[role] > 0 {
				p = append(p, ScoutPosition{Position: role, Games: positions[role]})
			}
		}
		r.Queues = append(r.Queues, ScoutQueue{QueueID: q, Season: CalculateStats(s), Twelve: CalculateStats(tw), Month: CalculateStats(month), Positions: p})
	}
	return r
}
