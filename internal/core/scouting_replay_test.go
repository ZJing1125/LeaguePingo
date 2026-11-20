package core

import (
	"testing"
	"time"
)

// Model both a rich mixed history and a sparse Mayhem history across 30-day
// and season boundaries. Duplicates, remakes and unknown fields are intentional.
func scoutReplayHistory(sparse bool, now time.Time) []Match {
	records := make([]Match, 1000)
	modes := []int{420, 440, 450, 2400}
	for i := range records {
		at := now.Add(-time.Duration(i) * 2 * time.Hour)
		win, k, d, a, duration, gold, damage, cs := i%2 == 0, 5+i%10, 1+i%8, 4+i%18, 1200+i%1200, 9000+i, 10000+i*10, 150+i%120
		q := modes[i%4]
		if sparse {
			q = 420
			if i%31 == 0 {
				q = 2400
			}
		}
		records[i] = Match{GameID: int64(i + 1), QueueID: q, ChampionID: 1 + i%150, PlayedAt: &at, Win: &win, Kills: &k, Deaths: &d, Assists: &a, Duration: &duration, Gold: &gold, Damage: &damage, CS: &cs, Position: []string{"top", "jungle", "mid", "bottom", "support"}[i%5], Performance: &MatchPerformance{Version: PerformanceVersion, Basis: "position", Score: float64(i%100) / 10, Metrics: []PerformanceMetric{{Key: "damage", Score: 5, Games: 1}, {Key: "gold", Score: 6, Games: 1}, {Key: "survival", Score: 4, Games: 1}}}}
		if i%17 == 0 {
			early, short := true, 90
			records[i].EarlyEnd = &early
			records[i].Duration = &short
		}
		if i%11 == 0 {
			records[i].Kills = nil
			records[i].Performance = nil
			records[i].Position = ""
		}
		if i%37 == 0 {
			records[i].PlayedAt = nil
		}
		if i > 0 && i%23 == 0 {
			records[i] = records[i-1]
		}
	}
	return records
}
func BenchmarkScoutProgressivePages(b *testing.B) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	season := &ScoutSeason{Name: "synthetic", Start: now.Add(-60 * 24 * time.Hour), End: now.Add(24 * time.Hour)}
	for _, tc := range []struct {
		name   string
		sparse bool
	}{{"rich-mixed", false}, {"sparse-Mayhem", true}} {
		records := scoutReplayHistory(tc.sparse, now)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for player := 0; player < 10; player++ {
					for count := 20; count <= len(records); count += 20 {
						BuildScoutReport(records[:count], nil, season, now, false, false, false)
					}
				}
			}
		})
	}
}
func TestScoutReplayBoundariesUnknownsAndDedup(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	season := &ScoutSeason{Name: "synthetic", Start: now.Add(-60 * 24 * time.Hour), End: now.Add(time.Hour)}
	records := scoutReplayHistory(true, now)
	for n := 20; n <= 1000; n += 20 {
		report := BuildScoutReport(records[:n], nil, season, now, false, false, false)
		for _, q := range report.Queues {
			if q.Season.Games < q.Month.Games && q.QueueID == 420 {
				t.Fatal("season window lost recent matches")
			}
			if q.Analysis != nil && q.Analysis.ScoredGames > q.Analysis.Games {
				t.Fatal("unknown/remake scored as a game")
			}
		}
	}
	dup := append(append([]Match{}, records...), records...)
	original, repeated := BuildScoutReport(records, nil, season, now, false, false, false), BuildScoutReport(dup, nil, season, now, false, false, false)
	for i, q := range original.Queues {
		other := repeated.Queues[i]
		if q.Month.Games != other.Month.Games || q.Season.Games != other.Season.Games {
			t.Fatal("duplicate pages changed range statistics")
		}
	}
}
