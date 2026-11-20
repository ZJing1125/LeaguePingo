package core

import (
	"sort"
	"time"
)

// Scouting is independent of the recent-history list and its browsing filters.
// Reports are immutable after publication. Partial ranges never imply zero games.
const ScoutStatsVersion = 3

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
type ScoutCoverage struct {
	FetchedAt  time.Time `json:"fetchedAt"`
	Complete   bool      `json:"complete"`
	Complete12 bool      `json:"complete12"`
	Complete30 bool      `json:"complete30"`
}

type ScoutQueue struct {
	Analysis  *PlayerAnalysis `json:"analysis,omitempty"`
	Coverage  *ScoutCoverage  `json:"coverage,omitempty"`
	QueueID   int             `json:"queueId"`
	Season    Stats           `json:"season"`
	Twelve    Stats           `json:"twelve"`
	Month     Stats           `json:"month"`
	Positions []ScoutPosition `json:"positions"`
}
type ScoutReport struct {
	ResponseID   uint64       `json:"responseId,omitempty"`
	ModelVersion int          `json:"modelVersion,omitempty"`
	State        string       `json:"state"`
	Retained     bool         `json:"retained,omitempty"`
	FetchedAt    time.Time    `json:"fetchedAt"`
	Season       *ScoutSeason `json:"season,omitempty"`
	Complete     bool         `json:"complete"`
	Complete12   bool         `json:"complete12"`
	Complete30   bool         `json:"complete30"`
	Read         int          `json:"read"`
	Message      string       `json:"message,omitempty"`
	// Coverage fields let the interface describe what was actually read instead
	// of collapsing every shortfall into one generic notice. Reason is a fixed
	// code, never an error string.
	Reason         string          `json:"reason,omitempty"`
	RetryAt        *time.Time      `json:"retryAt,omitempty"`
	PagesRead      int             `json:"pagesRead,omitempty"`
	CoverageEndsAt *time.Time      `json:"coverageEndsAt,omitempty"`
	Champions      []ScoutChampion `json:"champions"`
	Queues         []ScoutQueue    `json:"queues"`
}

func BuildScoutReport(matches []Match, masteries []ChampionMastery, season *ScoutSeason, cutoff time.Time, complete, complete12, complete30 bool) ScoutReport {
	r := ScoutReport{ModelVersion: ScoutStatsVersion, State: "ready", FetchedAt: cutoff, Season: season, Complete: complete && season != nil, Complete12: complete12, Complete30: complete30, Read: len(matches), Champions: []ScoutChampion{}, Queues: []ScoutQueue{}}
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
		if mode, ok := ScoutModeFor(m.QueueID); ok && !mode.Matches(valueOrZero(m.MapID), "") {
			r.Complete, r.Complete12, r.Complete30 = false, false, false
			continue
		}
		valid = append(valid, m)
		if !m.PlayedAt.Before(cutoff.Add(-30*24*time.Hour)) && m.ChampionID > 0 {
			ids[m.ChampionID] = true
		}
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
	seasonByHero, recentByHero := indexScoutChampions(seasonal), indexScoutChampions(valid)
	for _, id := range ordered {
		r.Champions = append(r.Champions, buildScoutChampion(id, seasonByHero[id], recentByHero[id], masteryByID, r.Complete, cutoff))
	}
	for _, mode := range scoutModes {
		q := buildScoutQueue(mode, seasonal, valid, cutoff, r.Complete30)
		q.Coverage = &ScoutCoverage{FetchedAt: cutoff, Complete: r.Complete, Complete12: r.Complete12, Complete30: r.Complete30}
		r.Queues = append(r.Queues, q)
	}
	return r
}

func buildScoutChampion(id int, seasonal, valid []Match, masteryByID map[int]ChampionMastery, complete bool, cutoff time.Time) ScoutChampion {
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
	if len(all) > 0 || complete {
		n := len(all)
		h.Games = &n
	}
	for _, mode := range scoutModes {
		q := mode.QueueID
		selected := []Match{}
		var k, d, a float64
		source := all
		if !mode.Ranked {
			source = valid
		}
		for _, m := range source {
			if m.ChampionID == id && m.QueueID == q && (mode.Ranked || !m.PlayedAt.Before(cutoff.Add(-30*24*time.Hour))) {
				selected = append(selected, m)
				if !IsRemake(m) && m.Kills != nil && m.Deaths != nil && m.Assists != nil {
					k += float64(*m.Kills)
					d += float64(*m.Deaths)
					a += float64(*m.Assists)
				}
			}
		}
		if !mode.Ranked && len(selected) == 0 {
			continue
		}
		stats := CalculateStats(selected)
		average := []float64{}
		if stats.KDAGames > 0 {
			n := float64(stats.KDAGames)
			average = []float64{k / n, d / n, a / n}
		}
		h.Queues = append(h.Queues, ScoutHeroQueue{QueueID: q, Stats: stats, Average: average})
	}
	return h
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func buildScoutQueue(mode ScoutMode, seasonal, valid []Match, cutoff time.Time, complete bool) ScoutQueue {
	q := mode.QueueID
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
			if !mode.Roles || IsRemake(m) {
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
	return ScoutQueue{QueueID: q, Analysis: BuildPlayerAnalysis(month, q, cutoff, complete), Season: CalculateStats(s), Twelve: CalculateStats(tw), Month: CalculateStats(month), Positions: p}
}

// Index once per page publication, avoiding a full-history scan per hero/mode.
func indexScoutChampions(matches []Match) map[int][]Match {
	byHero := make(map[int][]Match)
	for _, match := range matches {
		byHero[match.ChampionID] = append(byHero[match.ChampionID], match)
	}
	return byHero
}
