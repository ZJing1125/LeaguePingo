package engine

import (
	"context"

	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type groupClient struct {
	*fakeClient
	count atomic.Int32
	block chan struct{}
}

func (f *groupClient) Grouping(context.Context, int64) ([]core.Participant, error) {
	f.count.Add(1)
	if f.block != nil {
		<-f.block
	}
	return []core.Participant{{PUUID: "self", Team: "ally", GroupToken: "8", Position: "mid"}, {PUUID: "ally", Team: "ally", GroupToken: "8"}, {PUUID: "enemy", Team: "enemy", GroupToken: "9"}, {PUUID: "enemy2", Team: "enemy", GroupToken: "9"}}, nil
}
func TestGroupingOnceAndLateConnectionIsolation(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "late"}[late], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &groupClient{fakeClient: &fakeClient{}, block: make(chan struct{})}
			e := New(nil, nil)
			e.client = f
			e.clientVersion = 1
			e.initialMatch.parent = ctx
			e.snapshot = core.Snapshot{ConnectionID: 1, SelfKey: "self", GameID: 42, QueueID: 420, Connected: true, Phase: "InProgress", Participants: []core.Participant{{PUUID: "self", Key: "self", Team: "ally"}, {PUUID: "ally", Key: "ally", Team: "ally"}, {PUUID: "enemy", Key: "enemy", Team: "enemy"}, {PUUID: "enemy2", Key: "enemy2", Team: "enemy"}}}
			e.mu.Lock()
			for range 10 {
				e.syncGroupingLocked()
			}
			e.mu.Unlock()
			deadline := time.Now().Add(time.Second)
			for f.count.Load() == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if late {
				e.mu.Lock()
				e.snapshot.Connected = false
				e.snapshot.ConnectionID = 2
				e.snapshot.GameID = 43
				e.clientVersion = 2
				e.syncGroupingLocked()
				e.mu.Unlock()
			}
			close(f.block)
			time.Sleep(80 * time.Millisecond)
			s := e.Snapshot()
			if !late && s.Participants[0].PremadeSize != 2 {
				t.Fatalf("group not merged %+v", s)
			}
			if late && s.Participants[0].PremadeGroup != "" {
				t.Fatal("late data merged")
			}
			if f.count.Load() > 2 {
				t.Fatal("unbounded group calls")
			}
		})
	}
}

func TestLateRosterIdentityUsesPreparedGroupMetadata(t *testing.T) {
	e := New(nil, nil)
	e.snapshot = core.Snapshot{ConnectionID: 1, SelfKey: "self", GameID: 42, QueueID: 420, Connected: true, Phase: "InProgress", Participants: []core.Participant{{PUUID: "self", Key: "self", Team: "ally"}, {PUUID: "late", Key: "late", Team: "ally"}}}
	e.clientVersion = 1
	e.groupScope = "1:self:42"
	e.groupAttempted = true
	e.groupMembers = map[string]string{"self": "8", "late": "8"}
	e.groupSides = map[string]string{"self": "ally", "late": "ally"}
	e.syncGroupingLocked()
	if e.snapshot.Participants[1].PremadeSize != 2 || e.snapshot.Participants[0].PremadeLabel != "A" {
		t.Fatal("prepared group not reused")
	}
	old := e.snapshot.Participants[0].PremadeGroup
	e.snapshot.GameID = 43
	e.snapshot.Participants[0].GroupToken = "8"
	e.syncGroupingLocked()
	if e.snapshot.Participants[0].PremadeGroup == old {
		t.Fatal("group code not scoped to game")
	}
}

type partialGroupClient struct {
	*fakeClient
	count atomic.Int32
}

func (f *partialGroupClient) Grouping(context.Context, int64) ([]core.Participant, error) {
	if f.count.Add(1) == 1 {
		return []core.Participant{{PUUID: "self", Team: "ally", GroupToken: "8"}, {PUUID: "late", Team: "ally", GroupToken: "8"}}, nil
	}
	return []core.Participant{{PUUID: "enemy", Team: "enemy", GroupToken: "9"}, {PUUID: "enemy2", Team: "enemy", GroupToken: "9"}}, nil
}
func TestSecondPartialGroupingDoesNotEraseLateIdentity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &partialGroupClient{fakeClient: &fakeClient{}}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.parent = ctx
	e.snapshot = core.Snapshot{ConnectionID: 1, SelfKey: "self", GameID: 42, QueueID: 420, Connected: true, Phase: "InProgress", Participants: []core.Participant{{Key: "self", PUUID: "self", Team: "ally"}}}
	e.mu.Lock()
	e.syncGroupingLocked()
	e.mu.Unlock()
	end := time.Now().Add(3 * time.Second)
	for f.count.Load() < 2 && time.Now().Before(end) {
		time.Sleep(10 * time.Millisecond)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if f.count.Load() != 2 {
		t.Fatal("second bounded lookup not attempted")
	}
	// Wait for publication under the same lock before inspecting prepared fields.
	if e.groupMembers["late"] != "8" {
		t.Fatal("second partial response erased prepared identity")
	}
	e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{Key: "late", PUUID: "late", Team: "ally"})
	e.syncGroupingLocked()
	if e.snapshot.Participants[1].PremadeSize != 2 || e.snapshot.Participants[1].GroupSource != "sgp" {
		t.Fatal("late group not restored with correct source")
	}
}
