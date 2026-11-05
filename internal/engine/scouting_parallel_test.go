package engine

import (
	"context"
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
