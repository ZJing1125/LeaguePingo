package engine

import "sync"

// rosterDiscovery owns only the local-identity readiness budget. Engine owns
// source I/O and verifies the connection/phase before applying observations.
// Engine -> mu is the fixed lock order; no callbacks occur under mu.
type rosterDiscovery struct {
	mu    sync.RWMutex
	state rosterReadiness
}

type rosterReadiness struct {
	attempts int
	complete bool
}

func (r *rosterDiscovery) view() rosterReadiness {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

func (r *rosterDiscovery) reset() {
	r.mu.Lock()
	r.state = rosterReadiness{}
	r.mu.Unlock()
}

func (r *rosterDiscovery) markComplete() {
	r.mu.Lock()
	r.state.complete = true
	r.mu.Unlock()
}

func (r *rosterDiscovery) observe(phase, previous Phase, stage Stage) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !activeMatch(phase) {
		if stage != StageInGame {
			r.state = rosterReadiness{}
		}
		return false
	}
	entering := !activeMatch(previous) || phase == PhaseGameStart && previous != PhaseGameStart
	if entering || phase == PhaseInProgress && previous == PhaseGameStart && !r.state.complete {
		r.state = rosterReadiness{}
	}
	refresh := r.state.attempts < inGameRosterAttemptLimit && !r.state.complete
	if refresh {
		r.state.attempts++
	}
	return refresh
}

// An explicit roster refresh may renew a new game's discovery window. It does
// not itself consume an automatic attempt or renew an unchanged active game.
func (r *rosterDiscovery) refreshed(phase, previous Phase, gameID, previousGameID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if activeMatch(phase) && (!activeMatch(previous) || phase == PhaseGameStart && previous != PhaseGameStart || gameID != 0 && gameID != previousGameID) {
		r.state = rosterReadiness{}
	}
	if phase == PhaseInProgress && previous == PhaseGameStart && !r.state.complete {
		r.state.attempts = 0
	}
}
