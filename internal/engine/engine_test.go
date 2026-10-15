package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type fakeClient struct {
	mu       sync.Mutex
	phase    string
	champion int
	calls    []string
}

func (*fakeClient) Region() string { return "CN" }

func (f *fakeClient) record(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, path)
}

func (f *fakeClient) Phase(context.Context) (string, error) {
	f.record("phase")
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.phase, nil
}
func (f *fakeClient) Roster(context.Context) (int64, []core.Participant, error) {
	f.record("roster")
	f.mu.Lock()
	champion := f.champion
	f.mu.Unlock()
	return core.ParseGameflowRoster([]byte(fmt.Sprintf(`{"gameData":{"gameId":19,"teamOne":[{"puuid":"alpha","championId":%d}],"teamTwo":[]}}`, champion)))
}
func (f *fakeClient) Name(context.Context, string) (string, error) {
	f.record("name")
	return "Alice#TEST", nil
}
func (f *fakeClient) History(context.Context, string) ([]core.Match, error) {
	f.record("history")
	return []core.Match{}, nil
}
func (f *fakeClient) FindByRiotID(context.Context, string) (core.Participant, error) {
	f.record("find")
	return core.Participant{}, nil
}

func TestEnteringGameFreezesSnapshotAndStopsHistoryRequests(t *testing.T) {
	fake := &fakeClient{phase: "ChampSelect"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	if len(e.Snapshot().Participants) != 1 {
		t.Fatalf("expected one player in preparation: %+v", e.Snapshot())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.Snapshot().Participants[0].HistoryState == core.HistoryEmpty {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.mu.Lock()
	fake.phase = "InProgress"
	fake.mu.Unlock()
	e.poll(context.Background())
	snapshot := e.Snapshot()
	if !snapshot.Frozen || len(snapshot.Participants) != 1 || snapshot.Participants[0].DisplayName != "Alice#TEST" {
		t.Fatalf("snapshot not frozen with prepared identity: %+v", snapshot)
	}
	fake.mu.Lock()
	before := len(fake.calls)
	fake.mu.Unlock()
	e.poll(context.Background())
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, path := range fake.calls[before:] {
		if strings.Contains(path, "history") {
			t.Fatalf("history request after game started: %s", path)
		}
	}
}

func TestPollIntervalsReduceInGameActivity(t *testing.T) {
	e := New(nil, nil)
	if e.pollInterval() != 10*time.Second {
		t.Fatal("unexpected disconnected interval")
	}
	e.snapshot.Stage = "preparing"
	if e.pollInterval() != 3*time.Second {
		t.Fatal("unexpected preparation interval")
	}
	e.snapshot.Stage = "in-game"
	if e.pollInterval() != 20*time.Second {
		t.Fatal("unexpected in-game interval")
	}
}

func TestRosterUpdateKeepsPreparedHistory(t *testing.T) {
	fake := &fakeClient{phase: "ChampSelect"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(e.Snapshot().Participants) > 0 && e.Snapshot().Participants[0].HistoryState == core.HistoryEmpty {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.mu.Lock()
	fake.champion = 22
	fake.mu.Unlock()
	e.poll(context.Background())
	player := e.Snapshot().Participants[0]
	if player.ChampionID != 22 || player.HistoryState != core.HistoryEmpty || player.DisplayName != "Alice#TEST" {
		t.Fatalf("prepared data lost after champion change: %+v", player)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	count := 0
	for _, call := range fake.calls {
		if call == "history" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("history queried %d times", count)
	}
}
