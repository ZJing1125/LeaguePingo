package engine

import (
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
	"leaguepingo/internal/teambrief"
)

func (e *Engine) observeAnalysisBriefLocked() {
	state := e.brief.Snapshot()
	if !state.Enabled {
		value := e.brief.Observe(e.initialMatch.parentContext(), nil, teambrief.Frame{})
		e.snapshot.AnalysisBrief = &value
		return
	}
	lines, ready, rated := core.AnalysisBriefLines(e.snapshot, state.IncludeSelf)
	selection := fmt.Sprintf("phase-%d", e.phaseEpoch)
	if e.session.SelectionKey != "" {
		selection = e.session.SelectionKey
	} else if e.snapshot.GameID > 0 {
		selection = fmt.Sprintf("game-%d", e.snapshot.GameID)
	}
	active := e.snapshot.Connected && e.snapshot.SelfKey != "" &&
		snapshotPhase(e.snapshot.Phase) == PhaseChampSelect &&
		(e.phaseHint == PhaseUnknown || e.phaseHint == PhaseChampSelect)
	frame := teambrief.Frame{
		Scope:        fmt.Sprintf("%d:%s:%s:%d", e.clientVersion, e.snapshot.SelfKey, selection, e.snapshot.QueueID),
		SelectionKey: e.session.SelectionKey, GameID: e.snapshot.GameID, QueueID: e.snapshot.QueueID,
		Active: active, Lines: lines, Ready: ready, Rated: rated,
	}
	src, _ := e.client.(source.AnalysisBriefSource)
	if mode, ok := core.ScoutModeFor(e.snapshot.QueueID); !ok || mode.Arena || e.scoutModeLocked() == 0 {
		src = nil
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
	lines, _, _ := core.AnalysisBriefLines(e.snapshot, cfg.IncludeSelf)
	return core.AnalysisBriefBody(lines)
}

// Preview uses the editable include-self preference without configuring actions.
func (e *Engine) PreviewAnalysisBriefWithOptions(includeSelf bool) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	lines, _, _ := core.AnalysisBriefLines(e.snapshot, includeSelf)
	return core.AnalysisBriefBody(lines)
}
