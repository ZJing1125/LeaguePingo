package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type fairScoutClient struct {
	*fakeClient
	mu     sync.Mutex
	order  []int
	season chan struct{}
	first  chan string
	now    time.Time
}

func (f *fairScoutClient) CurrentQueue() int { return 2400 }
func (f *fairScoutClient) ScoutSeason(ctx context.Context) (*core.ScoutSeason, error) {
	select {
	case <-f.season:
	case <-ctx.Done():
	}
	return nil, nil
}
func (f *fairScoutClient) ChampionMasteries(context.Context, string) ([]core.ChampionMastery, error) {
	return nil, nil
}
func (f *fairScoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	f.mu.Lock()
	f.order = append(f.order, offset)
	f.mu.Unlock()
	if offset == 0 {
		f.first <- id
	}
	at := f.now.Add(-time.Duration(offset+1) * time.Hour)
	win := true
	return core.HistoryPage{Offset: offset, NextOffset: offset + 1, HasMore: offset == 0, Meta: core.HistoryMeta{Returned: 1, Parsed: 1}, Matches: []core.Match{{GameID: int64(offset + 1), PlayedAt: &at, QueueID: 2400, ChampionID: 3, Win: &win}}}, nil
}

func TestScoutFairFirstPagesDoNotWaitForSeasonAndSwitchDoesNotFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fairScoutClient{fakeClient: &fakeClient{}, season: make(chan struct{}), first: make(chan string, 5), now: time.Now().UTC()}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby"}
	for i := 0; i < 5; i++ {
		id := fmt.Sprint("player-", i)
		e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{ConnectionID: 1, Key: id, PUUID: id, IdentityState: core.IdentityKnown, ChampionID: 3})
	}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	for i := 0; i < 5; i++ {
		select {
		case <-f.first:
		case <-time.After(time.Second):
			t.Fatal("first pages blocked behind season or another player's deep history")
		}
	}
	publishDeadline := time.Now().Add(time.Second)
	for {
		ready := 0
		for _, p := range e.Snapshot().Participants {
			if p.Scout != nil && p.Scout.Read > 0 {
				ready++
			}
		}
		if ready == 5 {
			break
		}
		if time.Now().After(publishDeadline) {
			t.Fatal("first data waited for blocked season metadata")
		}
		time.Sleep(time.Millisecond)
	}
	close(f.season)
	waitScout(t, e)
	deadline := time.Now().Add(time.Second)
	for e.scouting.hasTask() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.scouting.hasTask() {
		t.Fatal("round did not finish")
	}
	f.mu.Lock()
	order := append([]int(nil), f.order...)
	f.mu.Unlock()
	for _, offset := range order[:5] {
		if offset != 0 {
			t.Fatalf("first round was not fair: %v", order)
		}
	}
	before := len(order)
	for _, queue := range []int{450, 420, 2400, 0} {
		if _, err := e.SetScoutQueue(queue); err != nil {
			t.Fatal(err)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.order) != before || e.Snapshot().ScoutQueue != 2400 {
		t.Fatal("mode switch fetched or failed to follow current mode")
	}
}
