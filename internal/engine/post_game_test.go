package engine

import (
	"context"
	"leaguepingo/internal/core"
	"testing"
	"time"
)

type postGameClient struct {
	*fakeClient
	returned chan bool
	engine   *Engine
}

func (f *postGameClient) AccountIdentity(context.Context) (string, error) { return "player-0", nil }
func (f *postGameClient) LobbyControl(context.Context) (core.LobbyControl, error) {
	return core.LobbyControl{}, nil
}
func (f *postGameClient) StartSearch(context.Context) error { return nil }
func (f *postGameClient) PlayAgain(context.Context) error {
	s := f.engine.Snapshot()
	f.returned <- s.Review != nil && s.Review.GameID == 42 && len(s.Review.Participants) == 10
	f.mu.Lock()
	f.phase = "Lobby"
	f.mu.Unlock()
	return nil
}
func TestPostGameReturnCannotPrecedeFrozenReview(t *testing.T) {
	f := &postGameClient{fakeClient: &fakeClient{phase: "EndOfGame"}, returned: make(chan bool, 1)}
	e := New(nil, nil)
	f.engine = e
	e.client = f
	e.clientVersion = 1
	_ = e.ConfigurePostGame(true, false, 10)
	e.snapshot = core.Snapshot{Connected: true, ConnectionID: 1, SelfKey: "player-0", Phase: "InProgress", GameID: 42, QueueID: 420, Participants: fullTestRoster()}
	e.publishLocked()
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.postGame.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	select {
	case frozen := <-f.returned:
		if !frozen {
			t.Fatal("returned before review freeze")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("return not attempted")
	}
	e.mu.Lock()
	e.snapshot.Phase = "Lobby"
	e.publishLocked()
	e.mu.Unlock()
	if e.Snapshot().Review == nil || e.Snapshot().Review.GameID != 42 {
		t.Fatal("returning to lobby discarded review")
	}
}
