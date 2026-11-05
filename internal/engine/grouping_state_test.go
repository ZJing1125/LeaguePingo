package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func groupingSnapshot(game int64) core.Snapshot {
	return core.Snapshot{ConnectionID: 1, SelfKey: "self", GameID: game, QueueID: 420, Connected: true, Phase: "InProgress",
		Participants: []core.Participant{{Key: "self", PUUID: "self", Team: "ally"}, {Key: "ally", PUUID: "ally", Team: "ally"}}}
}

func TestGroupingStateScopeCancellationAndLabelLifetime(t *testing.T) {
	var groups groupingState
	snapshot := groupingSnapshot(42)
	key, active := groups.apply(&snapshot)
	if !active {
		t.Fatal("active game not recognized")
	}
	ctx, cancel, started := groups.begin(context.Background(), key)
	if !started {
		t.Fatal("initial group task not reserved")
	}
	defer cancel()
	members := []core.Participant{{PUUID: "self", Team: "ally", GroupToken: "8", Position: "mid"}, {PUUID: "ally", Team: "ally", GroupToken: "8"}}
	if accepted, complete := groups.merge(key, members); !accepted || complete {
		t.Fatal("partial metadata not retained as partial")
	}
	groups.apply(&snapshot)
	label := snapshot.Participants[0].PremadeGroup
	if snapshot.Participants[0].PremadeLabel != "A" || snapshot.GroupStatus != "observed" || snapshot.Participants[0].Position != "mid" {
		t.Fatal("prepared metadata presentation changed")
	}
	groups.stop()
	if ctx.Err() != context.Canceled {
		t.Fatal("component did not cancel its task")
	}
	if _, _, restarted := groups.begin(context.Background(), key); restarted {
		t.Fatal("same-game stop restarted a one-shot task")
	}
	// Replacement rosters retain metadata/labels in the same game, then lose the
	// old identity's metadata on a new game. No engine lock is needed here.
	replacement := groupingSnapshot(42)
	groups.apply(&replacement)
	if replacement.Participants[0].PremadeGroup != label || replacement.Participants[0].GroupSource != "sgp" {
		t.Fatal("same-game roster lost its stable group")
	}
	next := groupingSnapshot(43)
	nextKey, _ := groups.apply(&next)
	if accepted, _ := groups.merge(key, members); accepted {
		t.Fatal("old game refilled the component")
	}
	if next.Participants[0].PremadeGroup != "" || next.Participants[0].Position != "" {
		t.Fatal("new game inherited old metadata")
	}
	_, nextCancel, nextStarted := groups.begin(context.Background(), nextKey)
	if !nextStarted {
		t.Fatal("new game did not receive its own task")
	}
	nextCancel()
}

func TestGroupingStateRejectsConflictingPartialMetadata(t *testing.T) {
	var groups groupingState
	snapshot := groupingSnapshot(42)
	key, _ := groups.apply(&snapshot)
	groups.merge(key, []core.Participant{{PUUID: "self", Team: "ally", GroupToken: "8", Position: "mid"}})
	for _, conflicting := range []core.Participant{{PUUID: "self", Team: "enemy", GroupToken: "8"}, {PUUID: "self", Team: "ally", GroupToken: "9"}} {
		if accepted, _ := groups.merge(key, []core.Participant{conflicting, {PUUID: "ally", Team: "ally", GroupToken: "9"}}); accepted {
			t.Fatal("conflict was partially committed")
		}
	}
	groups.apply(&snapshot)
	if snapshot.Participants[0].GroupToken != "8" || snapshot.Participants[0].Position != "mid" || snapshot.Participants[1].GroupToken != "" {
		t.Fatal("rejected partial metadata changed the prior result")
	}
}

func TestGroupingStateKeepsReconnectInactive(t *testing.T) {
	var groups groupingState
	snapshot := groupingSnapshot(42)
	key, _ := groups.apply(&snapshot)
	ctx, cancel, _ := groups.begin(context.Background(), key)
	defer cancel()
	snapshot.Phase = "Reconnect"
	if _, active := groups.apply(&snapshot); active || ctx.Err() != context.Canceled {
		t.Fatal("refactor expanded grouping into reconnect")
	}
}

func TestGroupingCancellationDoesNotWaitForSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &groupClient{fakeClient: &fakeClient{}, block: make(chan struct{})}
	defer close(f.block)
	e := New(nil, nil)
	e.client, e.clientVersion = f, 1
	e.initialMatch.setParent(ctx)
	e.snapshot = groupingSnapshot(42)
	e.mu.Lock()
	e.syncGroupingLocked()
	e.mu.Unlock()
	deadline := time.Now().Add(time.Second)
	for f.count.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if f.count.Load() == 0 {
		t.Fatal("source did not start")
	}
	stopped := make(chan struct{})
	go func() { e.grouping.stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("source I/O held the grouping lock")
	}
}
