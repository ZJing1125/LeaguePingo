package engine

import (
	"context"
	"testing"

	"leaguepingo/internal/core"
)

func reviewSnapshot(gameID int64) core.Snapshot {
	players := fullTestRoster()
	return core.Snapshot{ConnectionID: 7, Connected: true, Phase: "InProgress", Stage: "in-game", Region: "CN", GameID: gameID, SelfKey: players[0].Key, QueueID: 420, Participants: players}
}

func TestReviewStateKeepsPublishedViewsImmutable(t *testing.T) {
	var reviews reviewState
	snapshot := reviewSnapshot(42)
	reviews.observe(snapshot)
	snapshot.Phase = "EndOfGame"
	frozen := reviews.observe(snapshot)
	if frozen == nil || frozen.State != "waiting" {
		t.Fatal("finished candidate was not frozen")
	}
	ctx, cancel, token := reviews.begin()
	defer cancel()
	detail := core.MatchDetail{GameID: 42, Partial: true, Players: []core.MatchPlayer{{ParticipantID: 1, Target: true}}}
	if !reviews.merge(42, token, 7, detail, false) {
		t.Fatal("valid automatic partial result not accepted")
	}
	partial, _ := reviews.view()
	if partial.Result == nil || partial.State != "waiting" || frozen.Result != nil {
		t.Fatal("partial update modified a previously published review")
	}
	reviews.stop()
	stopped, nextToken := reviews.view()
	if ctx.Err() != context.Canceled || nextToken == token || stopped.State != "partial" || partial.State != "waiting" {
		t.Fatal("stop changed an old view or failed to cancel/invalidate the task")
	}
	detail.Partial = false
	if reviews.merge(42, token, 7, detail, false) || reviews.finish(42, token) {
		t.Fatal("cancelled task changed the component")
	}
	// A new game retains the last completed review until it actually finishes.
	next := reviewSnapshot(43)
	reviews.observe(next)
	retained, _ := reviews.view()
	if retained.GameID != 42 {
		t.Fatal("new live candidate overwrote the archived game")
	}
	next.Phase = "EndOfGame"
	reviews.observe(next)
	if reviews.finish(42, token) {
		t.Fatal("old completion applied to the new review")
	}
	latest, _ := reviews.view()
	if latest.GameID != 43 || latest.State != "waiting" || latest.Result != nil {
		t.Fatal("new review inherited an old result/state")
	}
}

func TestReviewStateRequiresCompletionAndVerifiedNoneOwner(t *testing.T) {
	var reviews reviewState
	snapshot := reviewSnapshot(42)
	self := snapshot.SelfKey
	reviews.observe(snapshot)
	for _, state := range []struct {
		connected bool
		phase     string
		self      string
	}{{false, "EndOfGame", self}, {true, "Reconnect", self}, {true, "WatchInProgress", self}, {true, "None", ""}} {
		snapshot.Connected, snapshot.Phase, snapshot.SelfKey = state.connected, state.phase, state.self
		if reviews.observe(snapshot) != nil {
			t.Fatal("non-completion archived a game")
		}
		if frozen, _ := reviews.view(); frozen != nil {
			t.Fatal("non-completion exposed a review")
		}
	}
	snapshot.Phase, snapshot.SelfKey = "None", self
	if reviews.observe(snapshot) == nil {
		t.Fatal("verified normal return home lost the finished game")
	}
	snapshot.Phase, snapshot.SelfKey = "Lobby", "different-verified-account"
	reviews.observe(snapshot)
	if review, _ := reviews.view(); review != nil {
		t.Fatal("review leaked across account ownership")
	}
}
