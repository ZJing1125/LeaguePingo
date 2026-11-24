package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

// parallelScoutClient holds every page until the test releases it and records
// the highest number of simultaneous page reads.
type parallelScoutClient struct {
	*scoutClient
	inFlight atomic.Int32
	max      atomic.Int32
	reached  chan struct{}
	once     sync.Once
	release  chan struct{}
}

func (p *parallelScoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	n := p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	for {
		observed := p.max.Load()
		if n <= observed || p.max.CompareAndSwap(observed, n) {
			break
		}
	}
	if n >= 2 {
		p.once.Do(func() { close(p.reached) })
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return core.HistoryPage{}, ctx.Err()
	}
	return p.scoutClient.ScoutPage(ctx, id, offset)
}

func TestScoutingReadsTwoPlayersInParallel(t *testing.T) {
	f := &parallelScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}, reached: make(chan struct{}), release: make(chan struct{})}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{
		{ConnectionID: 1, Key: "p1", PUUID: "p1", IdentityState: core.IdentityKnown, ChampionID: 23},
		{ConnectionID: 1, Key: "p2", PUUID: "p2", IdentityState: core.IdentityKnown, ChampionID: 24},
	}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	select {
	case <-f.reached:
	case <-time.After(2 * time.Second):
		t.Fatalf("only %d page reads ran at once", f.max.Load())
	}
	close(f.release)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := e.Snapshot()
		if len(s.Participants) == 2 && s.Participants[0].Scout != nil && s.Participants[1].Scout != nil &&
			s.Participants[0].Scout.State != "loading" && s.Participants[1].Scout.State != "loading" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("parallel scouting did not finish both players")
}

func TestChampSelectStartsAllFiveHistoryReadsTogether(t *testing.T) {
	f := &parallelScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}, reached: make(chan struct{}), release: make(chan struct{})}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "preparing", Phase: "ChampSelect", QueueID: 2400}
	for i := 0; i < 5; i++ {
		id := fmt.Sprint("concurrent-", i)
		e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{ConnectionID: 1, Key: id, PUUID: id, IdentityState: core.IdentityKnown, Team: "ally"})
	}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer close(f.release)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for f.max.Load() < 5 {
		select {
		case <-deadline.C:
			t.Fatalf("only %d concurrent readers", f.max.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

type independentScoutClient struct {
	*fairScoutClient
	slow         chan struct{}
	advanced     chan struct{}
	advancedOnce sync.Once
}

func (f *independentScoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	if id == "slow" {
		select {
		case <-f.slow:
		case <-ctx.Done():
			return core.HistoryPage{}, ctx.Err()
		}
	}
	if id != "slow" && offset > 0 {
		f.advancedOnce.Do(func() { close(f.advanced) })
	}
	return f.fairScoutClient.ScoutPage(ctx, id, offset)
}
func TestFastSelectionPlayerReadsNextPageWhilePeerStillPending(t *testing.T) {
	season := make(chan struct{})
	close(season)
	f := &independentScoutClient{fairScoutClient: &fairScoutClient{fakeClient: &fakeClient{}, season: season, first: make(chan string, 5), now: time.Now().UTC()}, slow: make(chan struct{}), advanced: make(chan struct{})}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Phase: "ChampSelect", Stage: "preparing", QueueID: 2400}
	for _, id := range []string{"slow", "fast"} {
		e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{ConnectionID: 1, Key: id, PUUID: id, Team: "ally", IdentityState: core.IdentityKnown})
	}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { close(f.slow); e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	select {
	case <-f.advanced:
	case <-time.After(time.Second):
		t.Fatal("fast player's second page waited for slow peer")
	}
}

type failingSeasonScoutClient struct {
	*scoutClient
	failures atomic.Int32
}

func (f *failingSeasonScoutClient) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	if offset > 0 {
		f.failures.Add(1)
		return core.HistoryPage{}, errors.New("synthetic history failure")
	}
	at := f.now.Add(-31 * 24 * time.Hour)
	return core.HistoryPage{Offset: 0, NextOffset: 1, HasMore: true, Meta: core.HistoryMeta{Returned: 1, Parsed: 1}, Matches: []core.Match{{GameID: 1, QueueID: 420, PlayedAt: &at}}}, nil
}
func TestSeasonExtensionDoesNotRestartFailedSelectionPage(t *testing.T) {
	f := &failingSeasonScoutClient{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}}
	e := New(nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Phase: "ChampSelect", Stage: "preparing", QueueID: 420, Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown, Team: "ally"}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	for e.scouting.hasTask() && ctx.Err() == nil {
		time.Sleep(time.Millisecond)
	}
	if ctx.Err() != nil || f.failures.Load() != 2 {
		t.Fatal("failure page reopened after retry", f.failures.Load())
	}
}
