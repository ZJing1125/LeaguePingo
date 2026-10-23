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

type reviewAccountClient struct {
	*pagingClient
	account string
}

func (c *reviewAccountClient) AccountIdentity(context.Context) (string, error) { return c.account, nil }
func TestNormalPostgameNonePreservesConnectionAndSettlement(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	ps := fullTestRoster()
	f := &reviewAccountClient{pagingClient: &pagingClient{fakeClient: &fakeClient{phase: "None"}, detailFn: func(ctx context.Context, id int64, target string) (core.MatchDetail, error) {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			close(done)
			return core.MatchDetail{}, ctx.Err()
		}
		close(done)
		return core.MatchDetail{GameID: id, Players: []core.MatchPlayer{{Target: true, ParticipantID: 1, Summary: core.Match{GameID: id}}}}, nil
	}}, account: ps[0].Key}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 7
	e.accountIdentity = f.account
	e.snapshot = core.Snapshot{ConnectionID: 7, Connected: true, Phase: "InProgress", Stage: "in-game", Region: "CN", GameID: 42, SelfKey: f.account, Participants: ps}
	e.publishLocked()
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("settlement never started")
	}
	e.poll(context.Background())
	if e.Snapshot().ConnectionID != 7 {
		t.Fatal("ordinary idle phase invalidated connection")
	}
	close(release)
	<-done
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if e.Snapshot().Review.Result != nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("returning home cancelled automatic settlement")
}

func TestPartialSettlementIsRetriedWithinOriginalBudget(t *testing.T) {
	first, second := make(chan struct{}), make(chan struct{})
	attempts := 0
	ps := fullTestRoster()
	f := &pagingClient{fakeClient: &fakeClient{}, detailFn: func(ctx context.Context, id int64, target string) (core.MatchDetail, error) {
		attempts++
		d := core.MatchDetail{GameID: id, Partial: attempts == 1, Players: []core.MatchPlayer{{ParticipantID: 1, Target: true, Summary: core.Match{GameID: id}}}}
		if attempts == 1 {
			close(first)
		} else {
			close(second)
		}
		return d, nil
	}}
	e := New(nil, nil)
	e.mu.Lock()
	e.client = f
	e.snapshot = core.Snapshot{Connected: true, Phase: "InProgress", Stage: "in-game", GameID: 42, SelfKey: ps[0].Key, Participants: ps}
	e.publishLocked()
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopReviewLocked(); e.mu.Unlock() }()
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("first settlement did not run")
	}
	select {
	case <-second:
	case <-time.After(6 * time.Second):
		t.Fatal("partial settlement incorrectly stopped readiness retries")
	}
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		if e.Snapshot().Review.State == "ready" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("complete settlement was not published")
}

func TestSinglePlayerPracticeSettlementIsReadyWithoutInventedTeamShares(t *testing.T) {
	ps := fullTestRoster()[:1]
	f := &pagingClient{fakeClient: &fakeClient{}, detailFn: func(ctx context.Context, id int64, target string) (core.MatchDetail, error) {
		return core.MatchDetail{GameID: id, Partial: true, Players: []core.MatchPlayer{{Target: true, ParticipantID: 1, Summary: core.Match{GameID: id}}}}, nil
	}}
	e := New(nil, nil)
	e.mu.Lock()
	e.client = f
	e.snapshot = core.Snapshot{Connected: true, Phase: "InProgress", Stage: "in-game", GameID: 42, QueueID: 3140, SelfKey: ps[0].Key, Participants: ps}
	e.publishLocked()
	e.snapshot.Phase = "EndOfGame"
	e.publishLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopReviewLocked(); e.mu.Unlock() }()
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		r := e.Snapshot().Review
		if r != nil && r.State == "ready" {
			if r.Result.Players[0].DamageShare != nil || r.Result.Players[0].Participation != nil {
				t.Fatal("practice fabricated 5v5 shares")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("legitimate solo practice treated as missing nine players")
}
