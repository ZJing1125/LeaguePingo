package guides

import (
	"context"
	"errors"

	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type fakeProvider struct {
	mu    sync.Mutex
	calls int
	fail  bool
	block chan struct{}
}

func (f *fakeProvider) Fetch(ctx context.Context, q Query) (*Guide, error) {
	f.mu.Lock()
	f.calls++
	fail := f.fail
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
	if fail {
		return nil, errors.New("synthetic unavailable")
	}
	return &Guide{Query: q, Version: "16.21", FetchedAt: time.Now(), SourceURL: "https://op.gg/zh-cn/lol/champions/galio/build", Builds: []BuildSection{{Label: "核心装备", Options: []Build{{IDs: []int{3152}}}}}}, nil
}
func guideSnapshot() core.Snapshot {
	return core.Snapshot{ConnectionID: 1, SelfKey: "self", GameID: 42, QueueID: 420, Connected: true, Phase: "InProgress", GameVersion: "16.21.1", Participants: []core.Participant{{Key: "self", ChampionID: 3, ChampionState: "selected", Position: "mid", Team: "ally"}, {Key: "enemy", ChampionID: 103, Position: "mid", Team: "enemy"}}}
}
func waitGuide(t *testing.T, s *Service, status string) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if s.Snapshot().Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %s got %+v", status, s.Snapshot())
}
func TestGuidePreparationCachingModeAndFailure(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	defer c()
	f := &fakeProvider{}
	dir := t.TempDir()
	s := New(ctx, dir, f)
	snap := guideSnapshot()
	s.Observe(snap)
	for range 20 {
		s.Observe(snap)
	}
	waitGuide(t, s, "ready")
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	if calls != 2 {
		t.Fatal("duplicate snapshots caused requests", calls)
	}
	if s.Snapshot().Matchup == nil {
		t.Fatal("no explicit lane matchup")
	}
	other := New(ctx, dir, f)
	other.Observe(snap)
	waitGuide(t, other, "ready")
	f.mu.Lock()
	if f.calls != calls {
		t.Fatal("disk cache not used")
	}
	f.fail = true
	f.mu.Unlock()
	_, _ = s.Select("mid", 103, true)
	waitGuide(t, s, "partial")
	if s.Snapshot().Base == nil {
		t.Fatal("failed refresh erased cached guide")
	}
	snap.QueueID = 1700
	s.Observe(snap)
	if s.Snapshot().Status != "unsupported" || s.Snapshot().Base != nil {
		t.Fatal("unknown mode reused ARAM/ranked guide")
	}
	snap.QueueID = 450
	s.Observe(snap)
	waitGuide(t, s, "failed")
	if s.Snapshot().Query.Opponent != 0 {
		t.Fatal("ARAM sent lane matchup")
	}
	files, _ := os.ReadDir(dir)
	for _, file := range files {
		b, _ := os.ReadFile(filepath.Join(dir, file.Name()))
		if len(b) > 256<<10 {
			t.Fatal("oversized cache")
		}
	}
}
func TestCancelledLateGuideAndUnknownPosition(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	defer c()
	f := &fakeProvider{block: make(chan struct{})}
	s := New(ctx, t.TempDir(), f)
	snap := guideSnapshot()
	snap.Participants[0].Position = ""
	s.Observe(snap)
	if s.Snapshot().Status != "position" {
		t.Fatal("guessed lane")
	}
	if _, err := s.Select("mid", 999, false); err == nil {
		t.Fatal("arbitrary enemy accepted")
	}
	_, _ = s.Select("mid", 103, false)
	time.Sleep(850 * time.Millisecond)
	snap.ConnectionID = 2
	snap.Connected = false
	s.Observe(snap)
	close(f.block)
	time.Sleep(50 * time.Millisecond)
	if s.Snapshot().Status != "idle" || s.Snapshot().Base != nil {
		t.Fatal("late guide leaked across connection")
	}
}

func TestModeClassificationDoesNotAliasVariants(t *testing.T) {
	for _, tc := range []struct {
		q, mapID       int
		game, expected string
		reference      bool
	}{{420, 11, "CLASSIC", "ranked", false}, {440, 11, "CLASSIC", "flex", false}, {450, 12, "ARAM", "aram", false}, {2400, 12, "ARAM", "aram-mayhem", false}, {2400, 11, "CLASSIC", "", false}, {2420, 12, "ARAM", "", false}, {420, 12, "ARAM", "", false}, {450, 12, "ARAM_MAYHEM", "", false}, {430, 11, "CLASSIC", "ranked", true}} {
		got, _, ref := mode(core.Snapshot{QueueID: tc.q, MapID: tc.mapID, GameMode: tc.game})
		if got != tc.expected || ref != tc.reference {
			t.Fatalf("wrong mode %+v -> %s %t", tc, got, ref)
		}
	}
}

func TestMemoryGuideRetentionAndMayhemCandidates(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	defer c()
	s := New(ctx, "", &fakeProvider{})
	q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid"}
	s.cache[cacheKey(q)] = &Guide{Query: q, FetchedAt: time.Now().Add(-8 * 24 * time.Hour)}
	if s.cached(q) != nil {
		t.Fatal("expired in-memory guide returned")
	}
	snap := guideSnapshot()
	snap.QueueID = 2400
	snap.Participants[0].Position = ""
	s.Observe(snap)
	if s.Snapshot().Query.Position != "mid" || s.Snapshot().Query.Opponent != 0 || s.Snapshot().Status == "position" {
		t.Fatal("Mayhem requested a lane or enemy")
	}
	s.mu.Lock()
	s.state.Base = &Guide{Query: q, Augments: []Augment{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}}
	s.mu.Unlock()
	got, err := s.Candidates(s.Snapshot().Scope, 3, []int{1, 2, 3})
	if err != nil || len(got.Candidates) != 3 {
		t.Fatal(err)
	}
	for _, ids := range [][]int{{1, 2, 3, 4}, {1, 1}, {99}} {
		if _, err := s.Candidates(s.Snapshot().Scope, 3, ids); err == nil {
			t.Fatal("invalid candidates accepted")
		}
	}
	snap.Participants[0].ChampionID = 99
	s.Observe(snap)
	if len(s.Snapshot().Candidates) > 0 {
		t.Fatal("old champion candidates retained")
	}
}
