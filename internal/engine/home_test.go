package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestHomeUsesVerifiedAccountAndReusesHistory(t *testing.T) {
	fake := &accountClient{fakeClient: &fakeClient{phase: "None"}, account: "verified-self"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	s := e.Snapshot()
	if s.SelfKey != "verified-self" || s.Stage != "idle" {
		t.Fatal("idle account identity missing")
	}
	p, err := e.CurrentPlayer(context.Background(), s.ConnectionID)
	if err != nil || p.PUUID != "verified-self" || !historyCached(p) {
		t.Fatalf("home account load failed: %v", err)
	}
	before := len(fake.calls)
	if _, err := e.CurrentPlayer(context.Background(), s.ConnectionID); err != nil || len(fake.calls) != before {
		t.Fatal("fresh home load repeated source requests")
	}
	fake.account = "different-self"
	e.poll(context.Background())
	if e.Snapshot().SelfKey != "different-self" || e.Snapshot().ConnectionID == s.ConnectionID {
		t.Fatal("idle account switch did not invalidate home")
	}
	if _, err := e.CurrentPlayer(context.Background(), s.ConnectionID); err == nil {
		t.Fatal("old connection allowed to load account")
	}
}

func TestHomeCannotStartExtraPreparationDuringGame(t *testing.T) {
	for _, stage := range []string{"party", "preparing", "in-game"} {
		t.Run(stage, func(t *testing.T) {
			fake := &fakeClient{}
			e := New(nil, nil)
			e.client, e.clientVersion, e.accountIdentity = fake, 1, "verified-self"
			e.snapshot = core.Snapshot{Connected: true, ConnectionID: 1, Stage: stage, SelfKey: "verified-self"}
			if _, err := e.CurrentPlayer(context.Background(), 1); err == nil || len(fake.calls) != 0 {
				t.Fatal("home started a new preparation outside idle")
			}
			p := core.Participant{Key: "verified-self", PUUID: "verified-self", ConnectionID: 1, IdentityState: core.IdentityKnown}
			core.ApplyHistory(&p, core.HistoryResult{})
			e.cachePlayerLocked(p)
			if result, err := e.CurrentPlayer(context.Background(), 1); err != nil || result.Key != p.Key || len(fake.calls) != 0 {
				t.Fatal("home failed to reuse account cache")
			}
		})
	}
}

func TestHomePreparationStopsWhenLeavingIdle(t *testing.T) {
	started := make(chan struct{})
	fake := &accountClient{fakeClient: &fakeClient{phase: "None"}, account: "verified-self"}
	fake.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		close(started)
		<-ctx.Done()
		return core.HistoryResult{}, ctx.Err()
	}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	finished := make(chan error, 1)
	go func() { _, err := e.CurrentPlayer(context.Background(), e.Snapshot().ConnectionID); finished <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("home did not start")
	}
	e.mu.Lock()
	e.replaceLocked("in-game", "InProgress", 42, nil, "")
	e.mu.Unlock()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled home load returned success")
		}
		if cacheCountsForTest(e).players != 0 {
			t.Fatal("cancelled home load overwrote the next phase's cache")
		}
	case <-time.After(time.Second):
		t.Fatal("home was not cancelled on game entry")
	}
}

type cachedHomeAccountClient struct{ *fakeClient }

func (*cachedHomeAccountClient) CachedAccountIdentity() string { return "cached-self" }

func TestInGameHomeIdentityComesFromAlreadyReadLocalRoster(t *testing.T) {
	fake := &cachedHomeAccountClient{&fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	if e.Snapshot().SelfKey != "cached-self" {
		t.Fatal("already verified local identity missing from in-game home")
	}
	if len(fake.calls) != 2 || fake.calls[0] != "phase" || fake.calls[1] != "roster" {
		t.Fatalf("home identity added in-game requests: %v", fake.calls)
	}
}

func TestPhaseEventCancelsHomeBeforeConfirmingPoll(t *testing.T) {
	e := New(nil, nil)
	e.client, e.clientVersion, e.phaseEvents.sequence = &fakeClient{}, 1, 1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.homeCancel = cancel
	e.observePhase(1, 1, "Lobby")
	if ctx.Err() == nil {
		t.Fatal("leaving idle did not cancel home on the phase notification")
	}
}

type homeOptionalClient struct{ *optionalProfileClient }

func (*homeOptionalClient) AccountIdentity(context.Context) (string, error) {
	return "verified-self", nil
}

func TestHomeOptionalDeadlineRetainsCompletedHistory(t *testing.T) {
	fake := &homeOptionalClient{&optionalProfileClient{fakeClient: &fakeClient{phase: "None"}}}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	p, err := e.CurrentPlayer(ctx, e.Snapshot().ConnectionID)
	if err != nil || !historyCached(p) || !fake.cancelled {
		t.Fatalf("optional deadline discarded completed home history: %v", err)
	}
}
