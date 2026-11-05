package engine

import (
	"context"
	"sync"
	"testing"
	"time"
)

type initialMatchTestView struct {
	started, eligible bool
	gameID            int64
	cancel            context.CancelFunc
}

func initialMatchForTest(s *initialMatchPreparation) initialMatchTestView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return initialMatchTestView{started: s.started, eligible: s.eligible, gameID: s.gameID, cancel: s.cancel}
}

func startEntryForTest(t *testing.T, s *initialMatchPreparation, version uint64, gameID int64) (initialMatchScope, context.Context) {
	t.Helper()
	s.enterGame(gameID, version)
	scope, claimed := s.claim(version, gameID)
	if !claimed {
		t.Fatal("entry scope not claimed")
	}
	ctx, started := s.begin(scope)
	if !started {
		t.Fatal("entry scope not started")
	}
	t.Cleanup(func() { s.finish(scope) })
	return scope, ctx
}

func TestEntryScopeOneShotAndLateCompletion(t *testing.T) {
	var s initialMatchPreparation
	s.setParent(context.Background())
	first, oldCtx := startEntryForTest(t, &s, 1, 42)
	if _, claimed := s.claim(1, 42); claimed {
		t.Fatal("polling claimed the entry scope twice")
	}
	if version, stopped := s.stop(); version != 1 || !stopped || oldCtx.Err() != context.Canceled || s.valid(1, 42) {
		t.Fatal("stop did not consume and cancel the scope")
	}
	s.enterGame(42, 2)
	if _, claimed := s.claim(2, 42); claimed {
		t.Fatal("reconnecting renewed the same game's eligibility")
	}
	second, currentCtx := startEntryForTest(t, &s, 2, 43)
	for _, stale := range []initialMatchScope{first, {version: 1, gameID: 43, token: second.token}, {version: 2, gameID: 43, token: first.token}} {
		if s.finish(stale) || currentCtx.Err() != nil || !s.valid(2, 43) {
			t.Fatal("a stale version, game or token completed the new scope")
		}
	}
	if s.valid(1, 43) || s.valid(2, 42) || s.running(42) || !s.running(43) {
		t.Fatal("scope observations crossed games or versions")
	}
	if !s.finish(second) || currentCtx.Err() != context.Canceled || s.hasTask() || s.valid(2, 43) {
		t.Fatal("current completion did not release the entry task")
	}
	if _, claimed := s.claim(2, 43); claimed {
		t.Fatal("completion granted another entry batch")
	}
}

func TestEntryBudgetAndRunContextLifetime(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	var s initialMatchPreparation
	s.setParent(parent)
	before := time.Now()
	scope, ctx := startEntryForTest(t, &s, 1, 42)
	after := time.Now()
	deadline, ok := ctx.Deadline()
	if !ok || deadline.Before(before.Add(initialMatchBudget)) || deadline.After(after.Add(initialMatchBudget)) {
		t.Fatal("entry task changed its 45-second budget")
	}
	s.finish(scope)
	if s.parentContext() != parent || parent.Err() != nil {
		t.Fatal("entry completion cancelled or replaced Run's shared parent")
	}
	_, next := startEntryForTest(t, &s, 1, 43)
	cancel()
	if next.Err() != context.Canceled {
		t.Fatal("Run cancellation did not reach the next entry task")
	}
}

func TestEntryEmptyTargetsConsumeEligibility(t *testing.T) {
	var s initialMatchPreparation
	s.setParent(context.Background())
	s.enterGame(42, 1)
	if _, claimed := s.claim(2, 42); claimed {
		t.Fatal("wrong connection claimed a pending entry")
	}
	scope, claimed := s.claim(1, 42)
	if !claimed {
		t.Fatal("entry not claimed")
	}
	s.empty(scope)
	if _, started := s.begin(scope); started || s.hasTask() {
		t.Fatal("empty target set started requests")
	}
	if _, claimed := s.claim(1, 42); claimed {
		t.Fatal("empty target set allowed another entry pass")
	}
	view := initialMatchForTest(&s)
	if !view.started || view.eligible {
		t.Fatal("empty target set lost the consumed state")
	}
}

func TestEntryStateConcurrentObservations(t *testing.T) {
	parent := context.Background()
	var s initialMatchPreparation
	s.setParent(parent)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				s.parentContext()
				s.game()
				s.hasTask()
				s.running(42)
				s.valid(1, 42)
				initialMatchForTest(&s)
			}
		}()
	}
	for game := int64(1); game <= 200; game++ {
		s.setParent(parent)
		s.enterGame(game, 1)
		scope, claimed := s.claim(1, game)
		if !claimed {
			t.Fatal("new game did not claim its scope")
		}
		if ctx, started := s.begin(scope); !started || ctx.Err() != nil {
			t.Fatal("new scope did not start")
		}
		s.stop()
		s.finish(scope)
	}
	wg.Wait()
}
