package engine

import (
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
	"leaguepingo/internal/teambrief"
)

func (e *Engine) observeAnalysisBriefLocked() {
	if !e.readiness.SelectionStarted().IsZero() && snapshotPhase(e.snapshot.Phase) != PhaseChampSelect && (e.phaseHint == PhaseUnknown || e.phaseHint == PhaseChampSelect) {
		e.beginAnalysisBriefLocked(PhaseChampSelect)
		value := e.brief.Snapshot()
		e.snapshot.AnalysisBrief = &value
		return
	}
	if snapshotPhase(e.snapshot.Phase) == PhaseChampSelect && e.brief.SelectionChanged(e.session.SelectionKey) {
		now := time.Now()
		e.readiness.Phase("", now)
		e.readiness.Phase("ChampSelect", now)
	}
	state := e.brief.Snapshot()

	lines, ready, rated := core.HorseBriefLines(e.snapshot, state.IncludeSelf)
	active := e.snapshot.Connected && e.snapshot.SelfKey != "" &&
		snapshotPhase(e.snapshot.Phase) == PhaseChampSelect &&
		(e.phaseHint == PhaseUnknown || e.phaseHint == PhaseChampSelect)
	frame := teambrief.Frame{
		Scope: e.analysisBriefScopeLocked(), StartedAt: e.readiness.SelectionStarted(),
		SelectionKey: e.session.SelectionKey, GameID: e.snapshot.GameID, QueueID: e.snapshot.QueueID,
		Active: active, Lines: lines, Ready: ready, Rated: rated,
	}
	src, _ := e.client.(source.AnalysisBriefSource)
	if mode, ok := core.ScoutModeFor(e.snapshot.QueueID); !ok || mode.Arena || e.scoutModeLocked() == 0 {
		src = nil
	}
	for _, p := range e.snapshot.Participants {
		if !state.IncludeSelf && (p.Key == e.snapshot.SelfKey || p.PUUID == e.snapshot.SelfKey) {
			continue
		}
		q := findCurrentAnalysis(p, e.snapshot.QueueID)
		if p.Team == "ally" && q != nil && q.Version >= 2 && q.ScoredGames >= 5 && q.State == "ready" && p.Scout != nil && p.Scout.FetchedAt.Before(frame.StartedAt) {
			frame.Cached++
		}
	}
	value := e.brief.Observe(e.initialMatch.parentContext(), src, frame)
	e.snapshot.AnalysisBrief = &value
}

func (e *Engine) ConfigureAnalysisBrief(enabled bool, audience string, includeSelf bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.brief.Configure(enabled, audience, includeSelf); err != nil {
		return err
	}
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return nil
}

func (e *Engine) PreviewAnalysisBrief() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cfg := e.brief.Snapshot()
	lines, _, _ := core.HorseBriefLines(e.snapshot, cfg.IncludeSelf)
	return core.AnalysisBriefBody(lines)
}

// Preview uses the editable include-self preference without configuring actions.
func (e *Engine) PreviewAnalysisBriefWithOptions(includeSelf bool) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	lines, _, _ := core.HorseBriefLines(e.snapshot, includeSelf)
	return core.AnalysisBriefBody(lines)
}

func (e *Engine) MuteAnalysisBrief() core.AnalysisBriefState {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.brief.MuteCurrent()
	e.publishLocked()
	return e.brief.Snapshot()
}

func (e *Engine) analysisBriefScopeLocked() string {
	return fmt.Sprintf("%d:%d", e.clientVersion, e.readiness.SelectionStarted().UnixNano())
}

// Start the deadline as soon as phase is known, before roster/source I/O.
func (e *Engine) beginAnalysisBriefLocked(phase Phase) {
	src, _ := e.client.(source.AnalysisBriefSource)
	e.brief.Observe(e.initialMatch.parentContext(), src, teambrief.Frame{
		Scope: e.analysisBriefScopeLocked(), StartedAt: e.readiness.SelectionStarted(),
		Active: phase == PhaseChampSelect,
	})
}

func (e *Engine) AnalysisBriefStatus() core.AnalysisBriefState { return e.brief.Snapshot() }

func findCurrentAnalysis(p core.Participant, queue int) *core.PlayerAnalysis {
	if p.Scout == nil {
		return nil
	}
	for _, q := range p.Scout.Queues {
		if q.QueueID == queue {
			return q.Analysis
		}
	}
	return nil
}

func (e *Engine) observeBriefPhase(generation, version, epoch uint64, phase Phase, observed time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		return
	}
	e.readiness.Phase(string(phase), observed)
	if phase != snapshotPhase(e.snapshot.Phase) {
		e.beginAnalysisBriefLocked(phase)
	}
}
