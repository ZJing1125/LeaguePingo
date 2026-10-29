package engine

import (
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

func (e *Engine) ReadinessMetrics() diagnostics.ReadinessMetric {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.readiness.Snapshot()
}
func (e *Engine) observeReadinessLocked() {
	if !e.snapshot.Connected || e.snapshot.Phase != "ChampSelect" || e.phaseHint != "" && e.phaseHint != "ChampSelect" {
		return
	}
	now := time.Now()
	for _, p := range e.snapshot.Participants {
		if p.Team != "ally" {
			continue
		}
		if (p.HistoryState == core.HistoryReady || p.HistoryState == core.HistoryEmpty) && p.HistoryMeta.FetchedAt != nil {
			e.readiness.First(now, *p.HistoryMeta.FetchedAt)
			return
		}
		if p.Scout != nil && !p.Scout.FetchedAt.IsZero() && (p.Scout.Read > 0 || p.Scout.State == "ready") {
			e.readiness.First(now, p.Scout.FetchedAt)
			return
		}
	}
}
