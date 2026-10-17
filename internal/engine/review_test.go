package engine

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func waitPreparationStopped(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		running := e.prepareRunning
		e.mu.RUnlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("preparation did not stop within budget")
}

func TestPreparationRecoversAfterTransientPhaseFailureWithoutRosterChange(t *testing.T) {
	var phases atomic.Int32
	fake := &fakeClient{phase: "ChampSelect", gameID: 42, roster: fullTestRoster()[:2]}
	fake.phaseFn = func() (string, error) {
		if phases.Add(1) == 2 {
			return "", errors.New("temporary phase timeout")
		}
		return "ChampSelect", nil
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	for _, p := range e.Snapshot().Participants {
		if p.HistoryState == core.HistoryLoading {
			t.Fatal("abandoned task still loading")
		}
	}
	e.mu.Lock()
	deadline := e.prepareDeadline
	e.prepareRetryAt = time.Time{} // Advance retry eligibility without a wall-clock sleep.
	e.mu.Unlock()
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	for _, p := range e.Snapshot().Participants {
		if p.HistoryState != core.HistoryEmpty {
			t.Fatal("unchanged roster did not resume preparation")
		}
	}
	if e.prepareDeadline != deadline {
		t.Fatal("retry renewed the session time budget")
	}
}

func TestPreparationAttemptBudgetSurvivesHeroChanges(t *testing.T) {
	fake := &fakeClient{phase: "ChampSelect", roster: fullTestRoster()[:1], gameID: 42}
	var histories atomic.Int32
	fake.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		histories.Add(1)
		return core.HistoryResult{}, errors.New("temporary failure")
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	for i := 1; i <= 3; i++ {
		fake.mu.Lock()
		fake.roster[0].ChampionID = i
		fake.mu.Unlock()
		e.poll(context.Background())
		waitPreparationStopped(t, e)
	}
	if histories.Load() != 2 {
		t.Fatalf("session attempt budget restarted: %d", histories.Load())
	}
}

func TestRetainedSnapshotCannotBecomeNewConnectionCache(t *testing.T) {
	for _, byKey := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete-ID", true: "roster-key"}[byKey], func(t *testing.T) {
			fake := &fakeClient{phase: "InProgress", rosterErr: errors.New("temporary roster failure")}
			e := New(func() (Client, error) { return fake, nil }, nil)
			old := fullTestRoster()[0]
			core.ApplyHistory(&old, core.HistoryResult{Matches: []core.Match{{GameID: 99}}})
			e.snapshot = core.Snapshot{Stage: "in-game", Phase: "InProgress", Frozen: true, GameID: 42, Participants: []core.Participant{old}}
			e.poll(context.Background())
			if !e.Snapshot().Participants[0].Stale {
				t.Fatal("old connection data not marked stale")
			}
			if byKey {
				if _, err := e.LookupPlayer(context.Background(), old.Key); err == nil {
					t.Fatal("old connection identity silently accepted")
				}
			} else {
				p, err := e.Lookup(context.Background(), old.DisplayName)
				if err != nil || p.ConnectionID != e.clientVersion || p.Cached || len(p.History) != 0 {
					t.Fatalf("new connection reused retained history: %v", err)
				}
				if !strings.Contains(strings.Join(fake.calls, ","), "find,history") {
					t.Fatal("no new-source lookup")
				}
			}
		})
	}
}
