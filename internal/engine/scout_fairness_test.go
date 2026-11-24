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
	delay  time.Duration
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
	if f.delay > 0 {
		timer := time.NewTimer(f.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return core.HistoryPage{}, ctx.Err()
		}
	}
	if offset == 0 {
		f.first <- id
	}
	at := f.now.Add(-time.Duration(offset+1) * time.Hour)
	win := true
	k, d, a := 6, 2, 8
	return core.HistoryPage{Offset: offset, NextOffset: offset + 1, HasMore: offset == 0, Meta: core.HistoryMeta{Returned: 1, Parsed: 1}, Matches: []core.Match{{GameID: int64(offset + 1), PlayedAt: &at, QueueID: 2400, ChampionID: 3, Win: &win, Kills: &k, Deaths: &d, Assists: &a}}}, nil
}

func TestScoutFiveFirstResultsWithOneSecondSyntheticPages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fairScoutClient{fakeClient: &fakeClient{}, season: make(chan struct{}), first: make(chan string, 5), now: time.Now().UTC(), delay: time.Second}
	e := New(nil, nil)
	e.client, e.clientVersion = f, 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby"}
	for i := 0; i < 5; i++ {
		id := fmt.Sprint("timing-synthetic-", i)
		e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{ConnectionID: 1, Key: id, PUUID: id, IdentityState: core.IdentityKnown, ChampionID: 3})
	}
	started := time.Now()
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("five first-page results did not publish")
		case <-tick.C:
			ready := 0
			for _, p := range e.Snapshot().Participants {
				if p.Scout != nil && p.Scout.Read > 0 {
					ready++
				}
			}
			if ready == 5 {
				t.Logf("synthetic five-player first results: %s (1s/page, two workers; excludes connection/auth and real-client network)", time.Since(started).Round(time.Millisecond))
				return
			}
		}
	}
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
