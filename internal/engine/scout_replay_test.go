package engine

import (
	"leaguepingo/internal/core"
	"reflect"
	"testing"
	"time"
)

// Feed the real cursor/page and publication path, comparing each growing
// prefix against the unchanged batch calculator. No remote source is involved.
func TestScoutReplayComplexPagesKeepReferenceResultsAndPublishOnce(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	season := &core.ScoutSeason{Name: "synthetic", Start: now.Add(-60 * 24 * time.Hour), End: now.Add(time.Hour)}
	records := make([]core.Match, 1000)
	for i := range records {
		at := now.Add(-time.Duration(i) * 2 * time.Hour)
		win, k, d, a, duration := i%2 == 0, 6, 2, 8, 1800
		q := 420
		if i%31 == 0 {
			q = 2400
		}
		records[i] = core.Match{GameID: int64(i + 1), QueueID: q, ChampionID: i%10 + 1, PlayedAt: &at, Win: &win, Kills: &k, Deaths: &d, Assists: &a, Duration: &duration, Performance: &core.MatchPerformance{Version: core.PerformanceVersion, Basis: "class", Score: 6, Metrics: []core.PerformanceMetric{{Key: "damage", Score: 6, Games: 1}, {Key: "gold", Score: 6, Games: 1}, {Key: "survival", Score: 6, Games: 1}}}}
		if i%17 == 0 {
			early, short := true, 90
			records[i].EarlyEnd = &early
			records[i].Duration = &short
		}
		if i%11 == 0 {
			records[i].Kills = nil
			records[i].Performance = nil
		}
		if i > 0 && i%23 == 0 {
			records[i] = records[i-1]
		}
	}
	delivered := make(chan core.Snapshot, 1)
	e := New(nil, func(s core.Snapshot) { delivered <- s })
	defer close(e.publisher.updates)
	for player := 0; player < 10; player++ {
		cursor := &scoutCursor{seen: map[int64]bool{}}
		for offset := 0; offset < 1000; offset += 20 {
			cursor.absorbPage(core.HistoryPage{Matches: records[offset : offset+20], Meta: core.HistoryMeta{Returned: 20}, Offset: offset, NextOffset: offset + 20, HasMore: offset < 980}, season, now, now.Add(-90*24*time.Hour))
			got := cursor.report(season, now)
			want := core.BuildScoutReport(records[:offset+20], nil, season, now, got.Complete, got.Complete12, got.Complete30)
			if !reflect.DeepEqual(got.Queues, want.Queues) || !reflect.DeepEqual(got.Champions, want.Champions) {
				t.Fatalf("reference differs at player=%d offset=%d", player, offset)
			}
			e.publisher.publish(core.Snapshot{Participants: []core.Participant{{Scout: &got}}})
			select {
			case <-delivered:
			case <-time.After(3 * time.Second):
				t.Fatal("publication stalled")
			}
		}
	}
	metrics := e.PublicationMetrics()
	if metrics.Sent != 500 || metrics.Delivered != 500 || metrics.Dropped != 0 {
		t.Fatalf("unexpected publication volume: %+v", metrics)
	}
}
