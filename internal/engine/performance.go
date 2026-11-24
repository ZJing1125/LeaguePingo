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
	if !e.snapshot.Connected || snapshotPhase(e.snapshot.Phase) != PhaseChampSelect || e.phaseHint != PhaseUnknown && e.phaseHint != PhaseChampSelect {
		return
	}
	now := time.Now()
	e.observeRatingReadinessLocked(now)
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

func (e *Engine) TeamReadinessMetrics() diagnostics.TeamReadinessMetric {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.readiness.Ratings()
}
func (e *Engine) observeRatingReadinessLocked(now time.Time) {
	visible, known, rated, insufficient, reading := 0, 0, 0, 0, 0
	for _, p := range e.snapshot.Participants {
		if p.Team != "ally" {
			continue
		}
		visible++
		if p.IdentityState != core.IdentityKnown {
			continue
		}
		known++
		if p.TeamRating != nil && p.TeamRating.Place > 0 {
			rated++
			continue
		}
		if p.Scout == nil || p.Scout.State == "loading" {
			reading++
		} else {
			insufficient++
		}
	}
	e.readiness.ObserveRatings(now, visible, known, rated, insufficient, reading)
}
