package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestLoadingPreparesBothTeamsAnalysisAndInProgressReusesIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	season := make(chan struct{})
	close(season)
	f := &fairScoutClient{fakeClient: &fakeClient{}, season: season, first: make(chan string, 10), now: time.Now().UTC()}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, QueueID: 2400, GameID: 42, Phase: "GameStart", Stage: "in-game", Participants: fullTestRoster()}
	for i := range e.snapshot.Participants {
		e.snapshot.Participants[i].ConnectionID = 1
	}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() }()
	waitScout(t, e)
	deadline := time.Now().Add(5 * time.Second)
	for e.scouting.hasTask() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.scouting.hasTask() {
		t.Fatal("loading round did not finish")
	}
	for _, p := range e.Snapshot().Participants {
		found := false
		for _, q := range p.Scout.Queues {
			if q.QueueID == 2400 {
				found = q.Analysis != nil && q.Analysis.Score != nil
			}
		}
		if !found {
			t.Fatal("loading player lacked cached analysis")
		}
	}
	f.mu.Lock()
	before := len(f.order)
	f.mu.Unlock()
	e.mu.Lock()
	e.snapshot.Phase = "InProgress"
	e.syncScoutingLocked()
	e.mu.Unlock()
	f.mu.Lock()
	after := len(f.order)
	f.mu.Unlock()
	if after != before {
		t.Fatal("phase transition restarted player history")
	}
}
