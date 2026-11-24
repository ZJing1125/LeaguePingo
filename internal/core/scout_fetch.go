package core

// FetchStatus describes this queue's preparation independently of coverage and
// rating. Exhausting the service is a terminal read, not proof of 30-day coverage.
type FetchStatus string

const (
	StatusPending        FetchStatus = "pending"
	StatusSuccess        FetchStatus = "success"
	StatusNoMatchInQueue FetchStatus = "no-match-in-queue"
	StatusTimeoutOrError FetchStatus = "timeout-or-error"
)

func ScoutFetchStatus(report *ScoutReport, queue *ScoutQueue) FetchStatus {
	if queue != nil && queue.FetchStatus != "" {
		return queue.FetchStatus
	}
	// Older in-memory and archived reports have no explicit result field.
	if report == nil || report.State == "loading" {
		return StatusPending
	}
	if report.Reason != "" {
		return StatusTimeoutOrError
	}
	if report.Complete30 {
		if queue == nil || queue.Month.Games == 0 && (queue.Analysis == nil || queue.Analysis.ScoredGames == 0) {
			return StatusNoMatchInQueue
		}
		return StatusSuccess
	}
	return StatusPending
}
