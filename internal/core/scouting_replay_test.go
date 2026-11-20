package core

import (
	"testing"
	"time"
)

// Rebuild each successive page, including scoring fields, as the real reader does.
func BenchmarkScoutProgressivePages(b *testing.B) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	modes := []int{420, 440, 450, 2400}
	records := make([]Match, 1000)
	for i := range records {
		at := now.Add(-time.Duration(i) * time.Hour)
		win, k, d, a, duration := i%2 == 0, 5+i%10, 1+i%8, 4+i%18, 1200+i%1200
		records[i] = Match{GameID: int64(i + 1), QueueID: modes[i%len(modes)], ChampionID: 1 + i%150, PlayedAt: &at, Win: &win, Kills: &k, Deaths: &d, Assists: &a, Duration: &duration}
	}
	b.ReportAllocs()
	for b.Loop() {
		for player := 0; player < 10; player++ {
			for count := 20; count <= len(records); count += 20 {
				BuildScoutReport(records[:count], nil, nil, now, false, false, false)
			}
		}
	}
}
