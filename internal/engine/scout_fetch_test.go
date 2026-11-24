package engine

import (
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestScoutFetchExhaustionDoesNotClaimFullMonth(t *testing.T) {
	now := time.Now()
	played := now.Add(-time.Hour)
	for _, matches := range [][]core.Match{nil, {{GameID: 1, QueueID: 450, PlayedAt: &played}}} {
		c := scoutCursor{seen: map[int64]bool{}}
		page := core.HistoryPage{Matches: matches, NextOffset: len(matches)}
		page.Meta.Returned = len(matches)
		c.absorbPage(page, nil, now, now.Add(-30*24*time.Hour))
		r := c.report(nil, now)
		if r.Complete30 {
			t.Fatal("service exhaustion claimed full month")
		}
		for _, q := range r.Queues {
			if q.QueueID == 420 && q.FetchStatus != core.StatusNoMatchInQueue {
				t.Fatal("successful empty queue remained pending", q)
			}
			if q.QueueID == 450 && len(matches) > 0 && q.FetchStatus != core.StatusSuccess {
				t.Fatal("known public records classified as empty")
			}
		}
	}
	c := scoutCursor{done: true, reason: "RequestTimeout"}
	r := c.report(nil, now)
	for _, q := range r.Queues {
		if q.FetchStatus != core.StatusTimeoutOrError {
			t.Fatal("timeout lost its fetch status")
		}
	}
}
