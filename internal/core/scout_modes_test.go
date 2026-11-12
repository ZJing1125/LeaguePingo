package core

import (
	"testing"
	"time"
)

func TestScoutModesSeparateHeroesWindowsAndRemakesWithoutSeason(t *testing.T) {
	now := time.Now().UTC()
	recent, old := now.Add(-time.Hour), now.Add(-31*24*time.Hour)
	yes, no, k, d, a, duration := true, false, 6, 2, 10, 90
	matches := []Match{
		{GameID: 1, QueueID: 2400, ChampionID: 3, PlayedAt: &recent, Win: &yes, Kills: &k, Deaths: &d, Assists: &a},
		{GameID: 2, QueueID: 450, ChampionID: 3, PlayedAt: &recent, Win: &no},
		{GameID: 3, QueueID: 2400, ChampionID: 3, PlayedAt: &old, Win: &no},
		{GameID: 4, QueueID: 2400, ChampionID: 3, PlayedAt: &recent, Win: &no, EarlyEnd: &yes, Duration: &duration},
		{GameID: 5, QueueID: 2400, ChampionID: 3, PlayedAt: &recent},
	}
	r := BuildScoutReport(matches, nil, nil, now, false, false, false)
	for _, q := range r.Queues {
		if q.QueueID == 2400 && (q.Month.Games != 3 || q.Month.Remakes != 1 || q.Month.OutcomeGames != 1 || q.Month.Wins != 1 || len(q.Positions) != 0) {
			t.Fatalf("bad mayhem: %+v", q)
		}
		if q.QueueID == 450 && q.Month.Losses != 1 {
			t.Fatal("classic ARAM mixed")
		}
	}
	for _, h := range r.Champions {
		if h.Games != nil {
			t.Fatal("invented season")
		}
		for _, q := range h.Queues {
			if q.QueueID == 2400 && (q.Stats.OutcomeGames != 1 || len(q.Average) != 3 || q.Average[0] != 6) {
				t.Fatalf("bad hero: %+v", q)
			}
		}
	}
}

func TestScoutMergeRetainsEachModeWithoutDroppingNewMayhemSample(t *testing.T) {
	now := time.Now().UTC()
	at := now.Add(-time.Hour)
	win := true
	old := BuildScoutReport([]Match{{GameID: 1, QueueID: 420, PlayedAt: &at, ChampionID: 3, Win: &win}}, nil, nil, now.Add(-time.Minute), false, false, false)
	old.Read = 200
	old.Complete30 = true
	next := BuildScoutReport([]Match{{GameID: 2, QueueID: 2400, PlayedAt: &at, ChampionID: 3, Win: &win}}, nil, nil, now, false, false, false)
	next.State = "partial"
	merged := MergeScoutReports(&old, next)
	for _, q := range merged.Queues {
		if (q.QueueID == 420 || q.QueueID == 2400) && q.Month.Wins != 1 {
			t.Fatalf("dropped a mode: %+v", q)
		}
	}
	for _, h := range merged.Champions {
		for _, q := range h.Queues {
			if q.QueueID == 2400 && q.Stats.Wins != 1 {
				t.Fatal("dropped new hero sample")
			}
		}
	}
	for _, q := range old.Champions[0].Queues {
		if q.QueueID == 2400 {
			t.Fatal("mutated published report")
		}
	}
}

func TestScoutCatalogKeepsEveryQueueIndependent(t *testing.T) {
	now := time.Now().UTC()
	at := now.Add(-time.Hour)
	win := true
	season := &ScoutSeason{Start: now.Add(-90 * 24 * time.Hour), End: now.Add(24 * time.Hour)}
	for _, mode := range ScoutModes() {
		r := BuildScoutReport([]Match{{GameID: 1, QueueID: mode.QueueID, ChampionID: 3, PlayedAt: &at, Win: &win, Position: "TOP", MapID: &mode.MapID}}, nil, season, now, false, false, false)
		for _, q := range r.Queues {
			expected := 0
			if q.QueueID == mode.QueueID {
				expected = 1
			}
			if q.Month.Games != expected {
				t.Fatalf("queue %d mixed with %d", mode.QueueID, q.QueueID)
			}
			if !mode.Roles && len(q.Positions) != 0 {
				t.Fatal("invented lane")
			}
		}
		if !mode.Matches(mode.MapID, mode.GameModes[0]) || mode.Matches(999, "INVALID") {
			t.Fatal("mode conflict accepted")
		}
	}
	if _, ok := ScoutModeFor(999999); ok {
		t.Fatal("unknown queue became ranked")
	}
}
