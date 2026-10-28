package engine

import (
	"context"
	"leaguepingo/internal/core"
	"sync/atomic"
	"testing"
	"time"
)

type acceptingClient struct {
	*fakeClient
	accepted atomic.Bool
	posts    atomic.Int32
}

func (f *acceptingClient) Lobby(context.Context) ([]core.Participant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.Participant(nil), f.roster...), nil
}
func (f *acceptingClient) ReadyCheck(context.Context) (core.ReadyCheck, error) {
	response := "None"
	if f.accepted.Load() {
		response = "Accepted"
	}
	return core.ReadyCheck{State: "InProgress", PlayerResponse: response}, nil
}
func (f *acceptingClient) AcceptReadyCheck(context.Context) error {
	f.posts.Add(1)
	f.accepted.Store(true)
	return nil
}
func TestAutoAcceptDoesNotWaitForPlayerHistory(t *testing.T) {
	started := make(chan struct{}, 1)
	f := &acceptingClient{fakeClient: &fakeClient{phase: "Lobby", roster: fullTestRoster()}}
	f.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return core.HistoryResult{}, ctx.Err()
	}
	e := New(func() (Client, error) { return f, nil }, nil)
	_ = e.ConfigureAutoAccept(true, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("shutdown blocked")
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("history did not occupy preparation lane")
	}
	f.mu.Lock()
	f.phase = "ReadyCheck"
	f.mu.Unlock()
	// Same notification path as the existing phase stream, with history still blocked.
	e.mu.Lock()
	e.observeAutoAcceptLocked("ReadyCheck")
	e.mu.Unlock()
	deadline := time.Now().Add(time.Second)
	for !f.accepted.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !f.accepted.Load() || f.posts.Load() != 1 {
		t.Fatal("player history blocked the independent acceptance lane")
	}
}
