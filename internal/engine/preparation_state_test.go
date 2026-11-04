package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type preparationView struct {
	token    uint64
	running  bool
	deadline time.Time
}

func preparationForTest(e *Engine) preparationView {
	e.preparation.mu.RLock()
	defer e.preparation.mu.RUnlock()
	return preparationView{e.preparation.token, e.preparation.running, e.preparation.deadline}
}

func TestPreparationStateKeepsWindowAttemptsAndPriority(t *testing.T) {
	var state preparationState
	players := fullTestRoster()[:2]
	state.prioritize(players[1].Key)
	start := time.Now()
	ctx, cancel, token, started := state.begin(players, players[0].ConnectionID, nil, start)
	if !started {
		t.Fatal("new preparation did not start")
	}
	defer cancel()
	deadline, _ := ctx.Deadline()
	if !deadline.Equal(start.Add(45 * time.Second)) {
		t.Fatal("new preparation window changed")
	}
	if _, _, _, duplicate := state.begin(players, players[0].ConnectionID, nil, start); duplicate {
		t.Fatal("one session started parallel workers")
	}
	next := state.nextHistory(append([]core.Participant{}, players...), players[0].ConnectionID)
	if next.Key != players[1].Key || !state.claimHistory(token, next.PUUID) {
		t.Fatal("preferred player was not first among equal attempts")
	}
	if next = state.nextHistory(append([]core.Participant{}, players...), players[0].ConnectionID); next.Key != players[0].Key {
		t.Fatal("priority bypassed the least-attempted player")
	}
	state.claimHistory(token, players[0].PUUID)
	state.claimHistory(token, players[1].PUUID)
	state.claimHistory(token, players[0].PUUID)
	if next := state.nextHistory(append([]core.Participant{}, players...), players[0].ConnectionID); next.PUUID != "" {
		t.Fatal("more than two history attempts selected")
	}
	state.refundHistory(token, players[1].PUUID)
	if next := state.nextHistory(append([]core.Participant{}, players...), players[0].ConnectionID); next.Key != players[1].Key {
		t.Fatal("manual preemption consumed a history attempt")
	}
	state.retryAfterPhaseFailure(token, start)
	state.stop()
	state.finish(token)
	if ctx.Err() != context.Canceled {
		t.Fatal("stop did not cancel the current worker")
	}
	if _, _, _, resumed := state.begin(players, players[0].ConnectionID, nil, start.Add(time.Second)); resumed {
		t.Fatal("phase retry cooling was bypassed")
	}
	resumedCtx, resumedCancel, resumedToken, resumed := state.begin(players, players[0].ConnectionID, nil, start.Add(2*time.Second))
	if !resumed {
		t.Fatal("eligible retry did not resume")
	}
	defer resumedCancel()
	resumedDeadline, _ := resumedCtx.Deadline()
	if resumedToken != token || !resumedDeadline.Equal(deadline) {
		t.Fatal("retry renewed the session token/window")
	}
	state.stop()
	state.finish(resumedToken)
	if _, extraCancel, beyondWindow, renewed := state.begin(players, players[0].ConnectionID, nil, deadline); renewed {
		extraCancel()
		t.Fatalf("expired window resumed at token %d", beyondWindow)
	}
}

func TestPreparationStateResetsRejectLateFinishAndKeepsProfileCap(t *testing.T) {
	var state preparationState
	players := fullTestRoster()
	players = append(players, core.Participant{Key: "extra", PUUID: "extra", ConnectionID: players[0].ConnectionID})
	for i := range players {
		players[i].HistoryState = core.HistoryReady
		players[i].HistoryMeta.FetchedAt = nil
	}
	eligible := make([]bool, len(players))
	for i := range players {
		eligible[i] = true
	}
	ctx, cancel, token, started := state.begin(players, players[0].ConnectionID, eligible, time.Now())
	if !started {
		t.Fatal("preparation did not start")
	}
	defer cancel()
	for range 10 {
		if p := state.claimProfile(token, players, eligible); p.PUUID == "" {
			t.Fatal("profile was skipped before its one-per-player cap")
		}
	}
	if p := state.claimProfile(token, players, eligible); p.PUUID != "" {
		t.Fatal("profile repeated within the same window")
	}
	state.reset()
	if ctx.Err() != context.Canceled || state.current(token) || state.finish(token) || state.claimHistory(token, players[0].PUUID) {
		t.Fatal("reset accepted old work or failed to cancel it")
	}
	_, nextCancel, nextToken, nextStarted := state.begin(players, players[0].ConnectionID, eligible, time.Now())
	if !nextStarted || nextToken == token {
		t.Fatal("new session did not receive its own token")
	}
	defer nextCancel()
	if state.finish(token) {
		t.Fatal("late finish stopped the new session")
	}
	state.mu.RLock()
	running := state.running
	state.mu.RUnlock()
	if !running {
		t.Fatal("new worker lost its running state")
	}
}

func TestPreparationProfileEligibilityBelongsToEachRosterRecord(t *testing.T) {
	for _, oldFirst := range []bool{false, true} {
		var state preparationState
		stamp := time.Now()
		known := core.Participant{Key: "current", PUUID: "same", ConnectionID: 7, HistoryState: core.HistoryEmpty, HistoryMeta: core.HistoryMeta{FetchedAt: &stamp}}
		old := known
		old.Key, old.ConnectionID = "old", 6
		players, eligible := []core.Participant{known, old}, []bool{true, false}
		if oldFirst {
			players, eligible = []core.Participant{old, known}, []bool{false, true}
		}
		_, cancel, token, started := state.begin(players, 7, eligible, stamp)
		if !started {
			t.Fatal("old duplicate suppressed the eligible record")
		}
		got := state.claimProfile(token, players, eligible)
		cancel()
		if got.Key != "current" || got.ConnectionID != 7 {
			t.Fatal("old duplicate inherited another record's eligibility")
		}
	}
}
