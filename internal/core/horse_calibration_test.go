package core

import (
	"math"
	"testing"
	"time"
)

func TestPeerBaselinePreservesNeutralAndSeparatesStablePerformance(t *testing.T) {
	for _, mode := range []struct{ queue, mapID int }{{420, 11}, {450, 12}, {2400, 12}} {
		for _, ratio := range []float64{.5, .8, 1, 1.2, 1.5} {
			game := performanceGame()
			game["queueId"], game["mapId"] = mode.queue, mode.mapID
			target := game["participants"].([]any)[4].(map[string]any)["stats"].(map[string]any)
			target["assists"], target["visionScore"], target["totalDamageDealtToChampions"] = int(20*ratio), int(40*ratio), int(3000*ratio)
			match := parsePerformanceGame(t, game)
			if match.Performance == nil || match.Performance.Horse == nil || math.Abs(match.Performance.Horse.Score-60*ratio) > .01 {
				t.Fatalf("queue=%d ratio=%v peer comparison not calibrated: %+v", mode.queue, ratio, match.Performance)
			}
			history := []Match{}
			for i := range 15 {
				copy := match
				copy.GameID = int64(i + 1)
				history = append(history, copy)
			}
			analysis := BuildPlayerAnalysis(history, mode.queue, time.Now(), true)
			// The per-game score stays 60*ratio; the B-factor rescale maps
			// the 15-game mean around the 60 baseline before tiering.
			want := math.Round(clampScore(60+(60*ratio-60)*horseRules.ScoreScale, 0, 100)*10) / 10
			if analysis.Version != 4 || analysis.Score == nil || math.Abs(*analysis.Score-want) > .01 || analysis.Grade != HorseTier(want) {
				t.Fatal("weighted history lost the calibrated score", analysis)
			}
		}
	}
	t.Log("synthetic stable performance at 0.5x / 1.0x / 1.5x of peers scores 0 / 60 / 100 after the B-factor rescale")
}

func TestRescaledScoresSpreadAcrossTiersWhenGamesVary(t *testing.T) {
	// Regression for the all-middle-tier pile-up: the old suite only built
	// 15 identical games, so the 15-game mean compression was invisible.
	// Vary each game's peer ratio around a different mean per player and
	// require the rescaled grades to actually spread across tiers.
	for _, mode := range []struct{ queue, mapID int }{{420, 11}, {450, 12}, {2400, 12}} {
		grades := map[string]bool{}
		for _, tc := range []struct {
			name   string
			ratios [2]float64
			grade  string
		}{
			{"strong", [2]float64{1.3, 1.1}, "千里马"},
			{"average", [2]float64{1.1, 0.9}, "中等马"},
			{"weak", [2]float64{0.9, 0.7}, "纯牛马"},
		} {
			history := []Match{}
			for i := range 15 {
				game := performanceGame()
				game["queueId"], game["mapId"] = mode.queue, mode.mapID
				ratio := tc.ratios[i%2]
				target := game["participants"].([]any)[4].(map[string]any)["stats"].(map[string]any)
				target["assists"], target["visionScore"], target["totalDamageDealtToChampions"] = int(20*ratio), int(40*ratio), int(3000*ratio)
				match := parsePerformanceGame(t, game)
				match.GameID = int64(i + 1)
				history = append(history, match)
			}
			analysis := BuildPlayerAnalysis(history, mode.queue, time.Now(), true)
			if analysis.Score == nil || analysis.Grade != tc.grade {
				t.Fatalf("queue=%d %s: got grade %s score %v, want %s", mode.queue, tc.name, analysis.Grade, analysis.Score, tc.grade)
			}
			grades[analysis.Grade] = true
		}
		if len(grades) != 3 {
			t.Fatalf("queue=%d: varying histories collapsed to %v", mode.queue, grades)
		}
	}
}

func TestCalibratedAbsoluteGradesAllowSameTier(t *testing.T) {
	players := rankedHorsePlayers()
	for i := range players {
		score := 60.0 + float64(i)
		players[i].Scout.Queues[0].Analysis.Score = &score
		players[i].Scout.Queues[0].Analysis.Grade = "中等马"
	}
	ApplyTeamRatings(players, 420)
	for _, p := range players {
		if p.TeamRating.Label != "中等马" {
			t.Fatal("same score band forced a different horse tier")
		}
	}
	for i := range players {
		players[i].Scout.Queues[0].Analysis.Version = 3
		players[i].Scout.Queues[0].Analysis.Grade = ""
	}
	ApplyTeamRatings(players, 420)
	if players[0].TeamRating.Label != "纯牛马" || players[4].TeamRating.Label != "千里马" {
		t.Fatal("archived v3 was reinterpreted")
	}
}

func TestPeerBaselineExplicitZerosStayFinite(t *testing.T) {
	game := performanceGame()
	players := game["participants"].([]any)
	own := players[4].(map[string]any)["stats"].(map[string]any)
	peer := players[9].(map[string]any)["stats"].(map[string]any)
	for _, value := range []int{0, 40} {
		own["visionScore"], peer["visionScore"] = value, 0
		match := parsePerformanceGame(t, game)
		if match.Performance == nil || match.Performance.Horse == nil {
			t.Fatal("zero comparator dropped known metrics")
		}
		want := 60.0
		if value > 0 {
			want = 100
		}
		found := false
		for _, metric := range match.Performance.Horse.Metrics {
			if metric.Key == "vision" {
				found = true
				if metric.Score != want || math.IsNaN(metric.Score) || math.IsInf(metric.Score, 0) {
					t.Fatal("zero comparator corrupted metric")
				}
			}
		}
		if !found {
			t.Fatal("known zero treated as missing")
		}
	}
}
