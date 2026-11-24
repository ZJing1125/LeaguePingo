package engine

import "leaguepingo/internal/core"

func (c *scoutCursor) setQueueFetchStatus(report *core.ScoutReport) {
	// A successful final page may end before 30 days for a new or sparse account.
	terminal := !c.invalid && c.reason == "" && (c.month || c.done && !c.hasMore && c.pages > 0)
	for i := range report.Queues {
		q := &report.Queues[i]
		q.FetchStatus = core.StatusPending
		switch {
		case c.invalid || c.reason != "":
			q.FetchStatus = core.StatusTimeoutOrError
		case terminal && q.Month.Games == 0:
			q.FetchStatus = core.StatusNoMatchInQueue
			if q.Analysis != nil {
				q.Analysis.State = "empty"
			}
		case terminal || q.Analysis != nil && q.Analysis.ScoredGames >= 15:
			q.FetchStatus = core.StatusSuccess
		}
	}
}
