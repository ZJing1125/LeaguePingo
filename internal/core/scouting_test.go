package core

import (
	"testing"
	"time"
)

func TestScoutSeasonAndRankedWindowsAreIndependentOfRecentTwenty(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	season := &ScoutSeason{Name: "2026", Start: now.Add(-200 * 24 * time.Hour), End: now.Add(20 * 24 * time.Hour)}
	matches := make([]Match, 1100)
	for i := range matches {
		played := now.Add(-time.Duration(i) * time.Hour)
		q := 420
		if i%2 == 1 {
			q = 440
		}
		win, k, d, a := i%3 != 0, 10, 2, 8
		matches[i] = Match{GameID: int64(i + 1), PlayedAt: &played, QueueID: q, ChampionID: 23, Win: &win, Kills: &k, Deaths: &d, Assists: &a, Position: "TOP"}
	}
	r := BuildScoutReport(matches, []ChampionMastery{{ChampionID: 23, Level: 40, Points: 910000}}, season, now, true, true, true)
	if r.Champions[0].Games == nil || *r.Champions[0].Games != 1100 {
		t.Fatal("season champion experience was capped or mastery converted to games")
	}
	if r.Champions[0].Queues[0].Stats.Games != 550 || r.Champions[0].Queues[1].Stats.Games != 550 {
		t.Fatal("ranked hero queues mixed")
	}
	if r.Queues[0].Twelve.Games != 7 || r.Queues[1].Twelve.Games != 6 || r.Queues[0].Month.Games != 361 || r.Queues[1].Month.Games != 360 {
		t.Fatal("ranked time windows must include all records and inclusive boundaries")
	}
	if r.Queues[0].Positions[0].Games != 361 || r.Champions[0].Queues[0].Average[0] != 10 {
		t.Fatal("role or hero averages wrong")
	}
	if r.Champions[0].Mastery.Points != 910000 {
		t.Fatal("mastery lost")
	}
}

func TestScoutPartialRangesUnknownOutcomesAndSeasonBoundary(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-25 * 24 * time.Hour)
	recent := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	season := &ScoutSeason{Name: "current", Start: now.Add(-20 * 24 * time.Hour), End: now.Add(20 * 24 * time.Hour)}
	loss := false
	m := []Match{{GameID: 1, ChampionID: 23, QueueID: 420, PlayedAt: &old, Win: &loss, Position: "TOP"}, {GameID: 2, ChampionID: 23, QueueID: 440, PlayedAt: &recent, Position: "JUNGLE"}, {GameID: 3, ChampionID: 23, QueueID: 450, PlayedAt: &recent, Win: &loss, Position: "TOP"}, {GameID: 4, ChampionID: 23, QueueID: 420, PlayedAt: &future, Win: &loss}, {GameID: 5, ChampionID: 23, QueueID: 420, Win: &loss}}
	m = append(m, m[1])
	r := BuildScoutReport(m, nil, season, now, false, false, false)
	if *r.Champions[0].Games != 2 || r.Champions[0].Queues[0].Stats.Games != 0 || r.Queues[0].Month.Games != 1 || r.Queues[1].Month.Games != 1 {
		t.Fatal("season, time windows, duplicate, future or mode filters wrong")
	}
	if r.Queues[0].Month.WinRate == nil || *r.Queues[0].Month.WinRate != 0 || r.Queues[1].Month.WinRate != nil {
		t.Fatal("unknown outcome became a real zero")
	}
	withoutSeason := BuildScoutReport(m, nil, nil, now, true, true, true)
	if withoutSeason.Complete || len(withoutSeason.Champions) != 0 {
		t.Fatal("unknown season must not invent champion counts")
	}
}
