package engine

import (
	"context"
	"leaguepingo/internal/core"
	"sync/atomic"
	"testing"
	"time"
)

type deadlineBriefClient struct {
	*fakeClient
	posts         atomic.Int32
	sent          chan string
	identityBlock func()
}

func (f *deadlineBriefClient) AccountIdentity(context.Context) (string, error) {
	if f.identityBlock != nil {
		f.identityBlock()
	}
	return "self", nil
}
func (*deadlineBriefClient) ChampSelectConversation(context.Context) (string, error) {
	return "selection", nil
}
func (f *deadlineBriefClient) SendAnalysisBrief(_ context.Context, _ string, body string, _ string) (bool, error) {
	if f.sent != nil {
		f.sent <- body
	}
	f.posts.Add(1)
	return true, nil
}

func TestBriefDeadlineStartsBeforeBlockingRosterIO(t *testing.T) {
	if testing.Short() {
		t.Skip("real five-second deadline")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	f := &deadlineBriefClient{fakeClient: &fakeClient{phase: "ChampSelect", rosterFn: func() { close(entered); <-release }}}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	_ = e.brief.Configure(true, "team", true)
	done := make(chan struct{})
	go func() { e.poll(ctx); close(done) }()
	<-entered
	defer func() { close(release); <-done; e.brief.Stop() }()
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	for e.AnalysisBriefStatus().Status != "no-data" {
		select {
		case <-deadline.C:
			t.Fatalf("deadline waited for roster: %+v", e.AnalysisBriefStatus())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	state := e.AnalysisBriefStatus()
	if state.FreezeMS > 5200 || state.ReadySeats != 0 || f.posts.Load() != 0 {
		t.Fatal("late or nonempty freeze", state)
	}
}

func TestBriefDeadlineAlsoRunsWhileAccountIdentityIsPending(t *testing.T) {
	if testing.Short() {
		t.Skip("real five-second deadline")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	f := &deadlineBriefClient{fakeClient: &fakeClient{phase: "ChampSelect"}, identityBlock: func() { close(entered); <-release }}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	_ = e.brief.Configure(true, "team", true)
	done := make(chan struct{})
	go func() { e.poll(ctx); close(done) }()
	<-entered
	defer func() { close(release); <-done; e.brief.Stop() }()
	e.mu.Lock()
	e.publishLocked()
	e.mu.Unlock()
	if e.AnalysisBriefStatus().Status != "waiting" {
		t.Fatal("publication cancelled pending selection deadline")
	}
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	for e.AnalysisBriefStatus().Status != "no-data" {
		select {
		case <-deadline.C:
			t.Fatal("deadline waited for account identity")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if e.AnalysisBriefStatus().FreezeMS > 5200 || f.posts.Load() != 0 {
		t.Fatal("invalid identity-pending dispatch")
	}
}

func TestVerifiedSelectionKeyResetsMuteWithoutIntermediatePhase(t *testing.T) {
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e.brief.Stop()
	e.client = &deadlineBriefClient{fakeClient: &fakeClient{}}
	e.clientVersion = 1
	e.accountIdentity = "self"
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{Connected: true, SelfKey: "self", Phase: "ChampSelect", Stage: "preparing", QueueID: 2400, MapID: 12, GameMode: "ARAM"}
	start := time.Now().Add(-time.Second)
	e.readiness.Phase("ChampSelect", start)
	e.session.SelectionKey = "first"
	_ = e.brief.Configure(true, "team", true)
	e.observeAnalysisBriefLocked()
	e.brief.MuteCurrent()
	e.session.SelectionKey = "second"
	e.observeAnalysisBriefLocked()
	if !e.readiness.SelectionStarted().After(start) || e.brief.Snapshot().Status != "waiting" {
		t.Fatal("new selection inherited mute", e.brief.Snapshot())
	}
}
