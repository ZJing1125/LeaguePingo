package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type offsetRecorder struct {
	*scoutClient
	mu      sync.Mutex
	offsets []int
}

func (o *offsetRecorder) ScoutPage(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
	o.mu.Lock()
	o.offsets = append(o.offsets, offset)
	o.mu.Unlock()
	return o.scoutClient.ScoutPage(ctx, id, offset)
}

func (o *offsetRecorder) firstOffset() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.offsets) == 0 {
		return -1
	}
	return o.offsets[0]
}

func progressFixture(now time.Time, pages int) (*scoutProgress, []core.Match) {
	matches := make([]core.Match, 0, pages*20)
	seen := map[int64]bool{}
	for i := 0; i < pages*20; i++ {
		at := now.Add(-time.Duration(i)*24*time.Hour - time.Hour)
		matches = append(matches, core.Match{GameID: int64(i + 1), ChampionID: 23, QueueID: 420, PlayedAt: &at})
		seen[int64(i+1)] = true
	}
	return &scoutProgress{matches: matches, seen: seen, offset: pages * 20, oldest: *matches[len(matches)-1].PlayedAt, at: time.Now()}, matches
}

// A stage change used to restart every player at the first page. The saved
// cursor must continue the same range inside the five-minute window.
func TestScoutingResumesProgressAcrossStageChange(t *testing.T) {
	f := &offsetRecorder{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	progress, _ := progressFixture(f.now, 1)
	e.scouting.progress = map[string]*scoutProgress{"p": progress}
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown, ChampionID: 23}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	r := waitScout(t, e)
	if f.firstOffset() != 20 {
		t.Fatalf("stage change restarted the range at offset %d", f.firstOffset())
	}
	if r.Read != 120 || !r.Complete || !r.Complete30 {
		t.Fatalf("resumed range did not complete: read=%d complete=%v complete30=%v", r.Read, r.Complete, r.Complete30)
	}
}

func TestStaleScoutProgressIsIgnored(t *testing.T) {
	f := &offsetRecorder{scoutClient: &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	progress, _ := progressFixture(f.now, 1)
	progress.at = time.Now().Add(-6 * time.Minute)
	e.scouting.progress = map[string]*scoutProgress{"p": progress}
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "party", Phase: "Lobby", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown, ChampionID: 23}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	t.Cleanup(func() { e.mu.Lock(); e.stopScoutingLocked(); e.mu.Unlock() })
	waitScout(t, e)
	if f.firstOffset() != 0 {
		t.Fatalf("stale cursor was resumed at offset %d", f.firstOffset())
	}
}

// A finished match invalidates the range, so the next preparation starts over.
func TestFinishedMatchDropsScoutProgress(t *testing.T) {
	f := &scoutClient{fakeClient: &fakeClient{}, now: time.Now().UTC()}
	e := New(nil, nil)
	e.client = f
	e.clientVersion = 1
	e.initialMatch.setParent(context.Background())
	progress, _ := progressFixture(f.now, 1)
	e.scouting.progress = map[string]*scoutProgress{"p": progress}
	e.snapshot = core.Snapshot{ConnectionID: 1, Connected: true, Stage: "in-game", Phase: "EndOfGame", Participants: []core.Participant{{ConnectionID: 1, Key: "p", PUUID: "p", IdentityState: core.IdentityKnown}}}
	e.mu.Lock()
	e.syncScoutingLocked()
	e.mu.Unlock()
	if e.scouting.progress != nil {
		t.Fatal("post-game phase kept a cursor from before the result")
	}
}
