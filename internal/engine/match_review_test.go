package engine

import (
	"context"
	"leaguepingo/internal/core"
	"testing"
	"time"
)

func TestFinishedReviewIsDeepFrozenAndSurvivesNewLobby(t *testing.T) {
	e := New(nil, nil)
	players := fullTestRoster()
	level := 50
	players[0].Profile = &core.Profile{Level: &level, Ranks: []core.Rank{{Tier: "GOLD"}}}
	players[0].HistoryState = core.HistoryLoading
	players[0].Scout = &core.ScoutReport{State: "loading"}
	e.snapshot = core.Snapshot{Connected: true, Phase: "InProgress", Stage: "in-game", Region: "CN", GameID: 42, SelfKey: players[0].Key, Participants: players}
	e.publishLocked()
	if e.review != nil {
		t.Fatal("live match exposed as finished review")
	}
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	if e.review == nil || e.review.GameID != 42 || len(e.review.Participants) != 10 {
		t.Fatal("finished roster lost")
	}
	if e.review.Participants[0].HistoryState == core.HistoryLoading || e.review.Participants[0].Scout.State == "loading" {
		t.Fatal("completed review retained running preparation")
	}
	level = 999
	players[0].Profile.Ranks[0].Tier = "IRON"
	players[0].DisplayName = "changed"
	if *e.review.Participants[0].Profile.Level != 50 || e.review.Participants[0].Profile.Ranks[0].Tier != "GOLD" {
		t.Fatal("nested profile was not frozen")
	}
	e.snapshot = core.Snapshot{Connected: true, Phase: "Lobby", Stage: "party", SelfKey: players[0].Key, Participants: players[:1]}
	e.publishLocked()
	if e.Snapshot().Review.GameID != 42 || len(e.Snapshot().Review.Participants) != 10 {
		t.Fatal("new lobby overwrote review")
	}
	e.snapshot.SelfKey = "different-verified-account"
	e.publishLocked()
	if e.Snapshot().Review != nil {
		t.Fatal("review leaked across accounts")
	}
}

func TestDodgeAndDisconnectDoNotCreateFinishedReview(t *testing.T) {
	for _, phase := range []string{"ChampSelect", "GameStart"} {
		e := New(nil, nil)
		e.snapshot = core.Snapshot{Connected: true, Phase: phase, GameID: 42, Participants: fullTestRoster()}
		e.publishLocked()
		e.snapshot.Phase = "Lobby"
		e.publishLocked()
		if e.review != nil {
			t.Fatal("unfinished selection/loading archived")
		}
	}
	e := New(nil, nil)
	e.snapshot = core.Snapshot{Connected: true, Phase: "InProgress", GameID: 42, Participants: fullTestRoster()}
	e.publishLocked()
	e.snapshot.Connected = false
	e.publishLocked()
	if e.review != nil {
		t.Fatal("disconnect treated as game end")
	}
}

func TestReviewResultCannotArriveIntoNewMatch(t *testing.T) {
	started, finish, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	fake := &pagingClient{fakeClient: &fakeClient{}, detailFn: func(ctx context.Context, id int64, target string) (core.MatchDetail, error) {
		close(started)
		<-finish
		close(returned)
		return core.MatchDetail{GameID: id, Players: []core.MatchPlayer{{Target: true, ParticipantID: 1, Summary: core.Match{GameID: id}}}}, nil
	}}
	e := New(nil, nil)
	e.client = fake
	ps := fullTestRoster()
	e.snapshot = core.Snapshot{Connected: true, Phase: "InProgress", Region: "CN", GameID: 42, SelfKey: ps[0].Key, Participants: ps}
	e.publishLocked()
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("bounded settlement did not start")
	}
	e.mu.Lock()
	e.snapshot.Phase = "InProgress"
	e.snapshot.GameID = 43
	e.publishLocked()
	e.mu.Unlock()
	close(finish)
	<-returned
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.review.Result != nil || e.reviewCandidate.GameID != 43 {
		t.Fatal("old settlement contaminated new game")
	}
	e.stopReviewLocked()
}
