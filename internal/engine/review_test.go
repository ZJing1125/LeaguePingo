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

func TestInGameReconnectRenewsRosterBudgetWithoutAutomaticHistory(t *testing.T) {
	fake := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	first := e.Snapshot().ConnectionID
	if !e.matchRosterComplete {
		t.Fatal("initial roster incomplete")
	}
	fake.phaseErr = errors.New("temporary disconnect")
	e.poll(context.Background())
	fake.phaseErr = nil
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	s := e.Snapshot()
	if s.ConnectionID <= first || !s.Connected || !e.matchRosterComplete {
		t.Fatal("reconnection did not recover roster")
	}
	for _, p := range s.Participants {
		if p.ConnectionID != s.ConnectionID {
			t.Fatal("old roster still used after reconnect")
		}
	}
	for _, call := range fake.calls {
		if call == "history" {
			t.Fatal("reconnection fetched automatic in-game history")
		}
	}
	if _, err := e.LookupPlayer(context.Background(), s.Participants[0].Key); err != nil {
		t.Fatalf("recovered roster cannot be queried: %v", err)
	}
}

func TestFirstHistoryFailureRetainsQueryableIdentityAndProfile(t *testing.T) {
	fake := &profileClient{fakeClient: &fakeClient{}, profile: core.Profile{PrivacyState: "private", RankState: "unavailable"}}
	fake.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{}, errors.New("history denied")
	}
	e := New(nil, nil)
	e.client = fake
	p, err := e.Lookup(context.Background(), "Manual#TEST")
	if err != nil || p.IdentityState != core.IdentityKnown || p.HistoryState != core.HistoryFailed || p.PrivacyState != "private" || p.Error == "" {
		t.Fatalf("identity/profile lost on first history failure: %v", err)
	}
	if _, err = e.LoadProfile(context.Background(), p.Key, p.ConnectionID); err != nil {
		t.Fatalf("profile retry blocked: %v", err)
	}
}

type accountClient struct {
	*fakeClient
	account      string
	accountCalls int
}

func (c *accountClient) AccountIdentity(context.Context) (string, error) {
	c.accountCalls++
	return c.account, nil
}

func TestAccountChangeWithoutNoneInvalidatesCacheOnlyBeforeGame(t *testing.T) {
	fake := &accountClient{fakeClient: &fakeClient{phase: "Lobby"}, account: "account-a"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	old := e.clientVersion
	p := fullTestRoster()[0]
	p.ConnectionID = old
	core.ApplyHistory(&p, core.HistoryResult{})
	e.cachePlayerLocked(p)
	e.snapshot.Participants = []core.Participant{p}
	fake.account = "account-b"
	e.poll(context.Background())
	if e.clientVersion <= old || len(e.cache) != 0 || e.Snapshot().ConnectionID != e.clientVersion || len(e.Snapshot().Participants) != 0 {
		t.Fatal("same-connection account change retained old identity/cache")
	}
	fake.phase = "InProgress"
	fake.roster = fullTestRoster()
	fake.gameID = 42
	before := fake.accountCalls
	e.poll(context.Background())
	e.poll(context.Background())
	if fake.accountCalls != before {
		t.Fatal("account checks added to gameplay")
	}
}
