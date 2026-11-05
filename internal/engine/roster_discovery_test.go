package engine

import (
	"sync"
	"testing"
)

func seedRosterForTest(e *Engine, attempts int, complete bool) {
	e.roster.mu.Lock()
	e.roster.state = rosterReadiness{attempts: attempts, complete: complete}
	e.roster.mu.Unlock()
}

func TestRosterDiscoveryReadinessWindowsAndCompletion(t *testing.T) {
	var r rosterDiscovery
	previous := PhaseChampSelect
	for i := 0; i < inGameRosterAttemptLimit; i++ {
		if !r.observe(PhaseGameStart, previous, StageInGame) {
			t.Fatal("loading discovery lost an attempt")
		}
		previous = PhaseGameStart
	}
	if r.observe(PhaseGameStart, previous, StageInGame) {
		t.Fatal("loading discovery exceeded three attempts")
	}
	if !r.observe(PhaseInProgress, PhaseGameStart, StageInGame) || r.view().attempts != 1 {
		t.Fatal("incomplete loading roster did not get the gameplay window")
	}
	r.markComplete()
	if r.observe(PhaseInProgress, PhaseInProgress, StageInGame) || !r.view().complete {
		t.Fatal("complete roster triggered more reads")
	}
	if r.observe(PhaseInProgress, PhaseGameStart, StageInGame) {
		t.Fatal("complete loading roster was renewed")
	}
	for _, phase := range []Phase{PhaseWaitingForStats, PhasePreEndOfGame, PhaseEndOfGame} {
		if r.observe(phase, PhaseInProgress, StageInGame) || !r.view().complete {
			t.Fatal("settlement cleared the existing local readiness state")
		}
	}
	r.observe(PhaseLobby, PhaseEndOfGame, StageParty)
	if r.view() != (rosterReadiness{}) {
		t.Fatal("leaving game did not clear the old window")
	}
}

func TestRosterDiscoveryExplicitRefreshDoesNotRenewSameGame(t *testing.T) {
	var r rosterDiscovery
	for i := 0; i < inGameRosterAttemptLimit; i++ {
		r.observe(PhaseInProgress, PhaseInProgress, StageInGame)
	}
	r.markComplete()
	r.refreshed(PhaseInProgress, PhaseInProgress, 42, 42)
	if r.view() != (rosterReadiness{attempts: 3, complete: true}) {
		t.Fatal("same-game refresh renewed discovery")
	}
	r.refreshed(PhaseInProgress, PhaseInProgress, 43, 42)
	if r.view() != (rosterReadiness{}) {
		t.Fatal("different game inherited the completed old roster")
	}
	for i := 0; i < inGameRosterAttemptLimit; i++ {
		r.observe(PhaseGameStart, PhaseGameStart, StageInGame)
	}
	r.refreshed(PhaseInProgress, PhaseGameStart, 43, 43)
	if r.view().attempts != 0 || r.view().complete {
		t.Fatal("explicit gameplay transition kept the incomplete loading cap")
	}
}

func TestRosterDiscoveryConcurrentObservations(t *testing.T) {
	var r rosterDiscovery
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				r.view()
				r.markComplete()
				r.reset()
				r.observe(PhaseInProgress, PhaseInProgress, StageInGame)
				r.refreshed(PhaseGameStart, PhaseLobby, 42, 0)
			}
		}()
	}
	wg.Wait()
}
