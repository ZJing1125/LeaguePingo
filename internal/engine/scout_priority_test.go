package engine

import (
	"context"
	"leaguepingo/internal/core"
	"testing"
	"time"
)

type delayedMasteryScout struct {
	*fakeClient
	second  chan struct{}
	mastery chan struct{}
	now     time.Time
}

func (*delayedMasteryScout) ScoutSeason(context.Context) (*core.ScoutSeason, error) { return nil, nil }
func (f *delayedMasteryScout) ChampionMasteries(ctx context.Context, _ string) ([]core.ChampionMastery, error) {
	close(f.mastery)
	<-ctx.Done()
	return nil, ctx.Err()
}
func (f *delayedMasteryScout) ScoutPage(_ context.Context, _ string, offset int) (core.HistoryPage, error) {
	queue := 420
	if offset > 0 {
		queue = 2400
		close(f.second)
	}
	at := f.now.Add(-time.Duration(offset+1) * time.Hour)
	win := true
	return core.HistoryPage{Offset: offset, NextOffset: offset + 1, HasMore: offset == 0, Meta: core.HistoryMeta{Returned: 1, Parsed: 1}, Matches: []core.Match{{GameID: int64(offset + 1), QueueID: queue, ChampionID: 3, PlayedAt: &at, Win: &win}}}, nil
}
func TestCurrentModeSampleArrivesBeforeOptionalMastery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &delayedMasteryScout{fakeClient: &fakeClient{}, second: make(chan struct{}), mastery: make(chan struct{}), now: time.Now().UTC()}
	e := New(nil, nil)
	e.client, e.clientVersion = f, 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, QueueID: 2400, MapID: 12, GameMode: "ARAM", Stage: "party", Phase: "Lobby", Participants: []core.Participant{{Key: "priority-synthetic", PUUID: "priority-synthetic", ConnectionID: 1, IdentityState: core.IdentityKnown, Team: "ally"}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	select {
	case <-f.second:
	case <-f.mastery:
		t.Fatal("optional mastery delayed current-mode page")
	case <-time.After(3 * time.Second):
		t.Fatal("second page was not scheduled")
	}
}
