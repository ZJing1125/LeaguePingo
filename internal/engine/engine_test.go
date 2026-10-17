package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type fakeClient struct {
	mu        sync.Mutex
	phase     string
	phaseErr  error
	phaseFn   func() (string, error)
	champion  int
	calls     []string
	roster    []core.Participant
	rosterErr error
	gameID    int64
	historyFn func(context.Context, string) (core.HistoryResult, error)
	rosterFn  func()
}

type optionalProfileClient struct {
	*fakeClient
	cancelled bool
}

func (f *optionalProfileClient) Profile(ctx context.Context, _ string) core.Profile {
	<-ctx.Done()
	f.cancelled = true
	return core.Profile{RankState: "unavailable"}
}
func TestOptionalProfileTimeoutKeepsSuccessfulHistory(t *testing.T) {
	fake := &optionalProfileClient{fakeClient: &fakeClient{phase: "Lobby"}}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	p, err := e.Lookup(ctx, "Player#TEST")
	if err != nil || !historyCached(p) || !fake.cancelled {
		t.Fatalf("optional profile lost history: %v", err)
	}
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
	if f.phaseFn != nil {
		return f.phaseFn()
	}
	return f.phase, f.phaseErr
}

func TestDiscoveryFailureIsPublishedAndRecovers(t *testing.T) {
	discoverErr := errors.New("客户端 lockfile 为空")
	fake := &fakeClient{phase: "Lobby"}
	e := New(func() (Client, error) { return fake, discoverErr }, nil)
	// Observe publication without starting the UI callback worker.
	e.onChange = func(core.Snapshot) {}
	e.updates = make(chan core.Snapshot, 4)
	e.poll(context.Background())
	snapshot := e.Snapshot()
	if snapshot.Connected || snapshot.Stage != "disconnected" || !strings.Contains(snapshot.Message, "lockfile 为空") {
		t.Fatalf("discovery error not exposed: %+v", snapshot)
	}
	select {
	case published := <-e.updates:
		if published.Message != snapshot.Message {
			t.Fatal("published diagnostic differs from snapshot")
		}
	default:
		t.Fatal("discovery failure not published to UI")
	}
	// Repeated identical failures should not flood the event bridge.
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	if len(e.updates) != 0 {
		t.Fatal("unchanged discovery failure republished")
	}
	discoverErr = nil
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	snapshot = e.Snapshot()
	if !snapshot.Connected || snapshot.Phase != "Lobby" || snapshot.Message != "" {
		t.Fatalf("discovery did not recover: %+v", snapshot)
	}
}

func TestStartingMidGameRefreshesLocalRosterWithoutHistory(t *testing.T) {
	for _, phase := range []string{"GameStart", "InProgress", "Reconnect", "WatchInProgress", "WaitingForStats", "PreEndOfGame", "EndOfGame"} {
		t.Run(phase, func(t *testing.T) {
			fake := &fakeClient{phase: phase}
			e := New(func() (Client, error) { return fake, nil }, nil)
			e.poll(context.Background())
			snapshot := e.Snapshot()
			if !snapshot.Connected || !snapshot.Frozen || !strings.Contains(snapshot.Message, "自动战绩预取") {
				t.Fatalf("mid-game startup not explained: %+v", snapshot)
			}
			expected := 1
			if activeMatch(phase) {
				expected = 2
			}
			if len(fake.calls) != expected || fake.calls[0] != "phase" || (expected == 2 && fake.calls[1] != "roster") {
				t.Fatalf("mid-game startup made unexpected calls: %v", fake.calls)
			}
		})
	}
}

func TestFrozenCacheSurvivesPhaseAndDiscoveryFailure(t *testing.T) {
	fake := &fakeClient{phase: "InProgress", phaseErr: errors.New("LCU HTTP 401"), rosterErr: errors.New("temporarily unavailable")}
	var discoverErr error
	e := New(func() (Client, error) { return fake, discoverErr }, nil)
	e.client = fake
	e.snapshot = core.Snapshot{
		Stage: "in-game", Phase: "InProgress", Connected: true, Frozen: true, GameID: 42,
		Participants: []core.Participant{{Key: "cached", DisplayName: "Cached player"}},
	}
	e.poll(context.Background())
	if snapshot := e.Snapshot(); snapshot.Connected || !snapshot.Frozen || len(snapshot.Participants) != 1 || !strings.Contains(snapshot.Message, "401") {
		t.Fatalf("phase failure lost cache or diagnostic: %+v", snapshot)
	}
	discoverErr = errors.New("没有权限查询客户端启动参数")
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	if snapshot := e.Snapshot(); snapshot.Connected || !snapshot.Frozen || len(snapshot.Participants) != 1 || !strings.Contains(snapshot.Message, discoverErr.Error()) {
		t.Fatalf("rediscovery failure lost cache or diagnostic: %+v", snapshot)
	}
	discoverErr = nil
	fake.phaseErr = nil
	e.lastLookup = time.Time{}
	e.poll(context.Background())
	if snapshot := e.Snapshot(); !snapshot.Connected || !snapshot.Frozen || snapshot.GameID != 42 || len(snapshot.Participants) != 1 {
		t.Fatalf("cached game did not reconnect: %+v", snapshot)
	}
	for _, call := range fake.calls {
		if call != "phase" && call != "roster" {
			t.Fatalf("reconnect requested data during game: %s", call)
		}
	}
}
func (f *fakeClient) Roster(context.Context) (int64, []core.Participant, error) {
	f.record("roster")
	if f.rosterFn != nil {
		f.rosterFn()
	}
	f.mu.Lock()
	champion := f.champion
	roster, gameID, rosterErr := f.roster, f.gameID, f.rosterErr
	f.mu.Unlock()
	if rosterErr != nil {
		return 0, nil, rosterErr
	}
	if roster != nil {
		return gameID, append([]core.Participant(nil), roster...), nil
	}
	return core.ParseGameflowRoster([]byte(fmt.Sprintf(`{"gameData":{"gameId":19,"teamOne":[{"puuid":"alpha","championId":%d}],"teamTwo":[]}}`, champion)))
}
func (f *fakeClient) Name(context.Context, string) (string, error) {
	f.record("name")
	return "Alice#TEST", nil
}
func (f *fakeClient) History(ctx context.Context, puuid string) (core.HistoryResult, error) {
	f.record("history")
	if f.historyFn != nil {
		return f.historyFn(ctx, puuid)
	}
	return core.HistoryResult{Matches: []core.Match{}}, nil
}
func (f *fakeClient) FindByRiotID(context.Context, string) (core.Participant, error) {
	f.record("find")
	return core.Participant{Key: "manual", PUUID: "manual", DisplayName: "Manual#TEST", IdentityState: core.IdentityKnown}, nil
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

func fullTestRoster() []core.Participant {
	players := make([]core.Participant, 10)
	for i := range players {
		team := "ally"
		if i >= 5 {
			team = "enemy"
		}
		id := fmt.Sprintf("player-%d", i)
		players[i] = core.Participant{Key: id, PUUID: id, DisplayName: id + "#TEST", Team: team, IdentityState: core.IdentityKnown, HistoryState: core.HistoryPending}
	}
	return players
}

func TestGameEntryReplacesUnknownEnemiesAndKeepsPreparedHistory(t *testing.T) {
	full := fullTestRoster()
	fake := &fakeClient{phase: "InProgress", roster: full, gameID: 42}
	e := New(func() (Client, error) { return fake, nil }, nil)
	old := fullTestRoster()
	old[0].HistoryState, old[0].Stats = core.HistoryReady, core.Stats{Games: 1}
	old[0].History = []core.Match{{GameID: 5}}
	for i := 5; i < 10; i++ {
		old[i].PUUID = ""
		old[i].IdentityState = core.IdentityUnavailable
		old[i].Key = fmt.Sprintf("enemy-slot-%d", i)
	}
	e.client = fake
	e.snapshot = core.Snapshot{Stage: "preparing", Phase: "ChampSelect", GameID: 42, Connected: true, Participants: old}
	e.poll(context.Background())
	snapshot := e.Snapshot()
	if !snapshot.Frozen || !completeRoster(snapshot.Participants) || snapshot.Participants[5].PUUID != full[5].PUUID {
		t.Fatalf("enemies not refreshed: %+v", snapshot)
	}
	if snapshot.Participants[0].HistoryState != core.HistoryReady || len(snapshot.Participants[0].History) != 1 {
		t.Fatal("prepared history lost")
	}
	for i := 0; i < 3; i++ {
		e.poll(context.Background())
	}
	rosterCalls := 0
	for _, call := range fake.calls {
		if call == "roster" {
			rosterCalls++
		}
		if call == "name" || call == "history" {
			t.Fatal("background player requests in-game")
		}
	}
	if rosterCalls != 1 {
		t.Fatalf("complete roster fetched repeatedly: %d", rosterCalls)
	}
}

func TestInGameRosterRetriesAreBoundedAndManualRefreshWorks(t *testing.T) {
	fake := &fakeClient{phase: "InProgress", rosterErr: errors.New("not ready")}
	e := New(func() (Client, error) { return fake, nil }, nil)
	for i := 0; i < 6; i++ {
		e.poll(context.Background())
	}
	rosterCalls := 0
	for _, call := range fake.calls {
		if call == "roster" {
			rosterCalls++
		}
	}
	if rosterCalls != 3 {
		t.Fatalf("expected three bounded local attempts, got %d", rosterCalls)
	}
	fake.rosterErr = nil
	fake.roster = fullTestRoster()
	fake.gameID = 42
	snapshot, err := e.RefreshRoster(context.Background())
	if err != nil || !completeRoster(snapshot.Participants) || !snapshot.Frozen {
		t.Fatalf("manual refresh failed: %v", err)
	}
	for _, call := range fake.calls {
		if call == "name" || call == "history" {
			t.Fatal("refresh requested history")
		}
	}
}

func TestManualLookupAllowedInGameAndDoesNotStartAutomaticPrefetch(t *testing.T) {
	fake := &fakeClient{phase: "InProgress"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client = fake
	e.snapshot = core.Snapshot{Stage: "in-game", Frozen: true, Connected: true}
	player, err := e.Lookup(context.Background(), "Manual#TEST")
	if err != nil || player.HistoryState != core.HistoryEmpty {
		t.Fatalf("in-game manual lookup blocked: %v", err)
	}
	if strings.Join(fake.calls, ",") != "find,history" {
		t.Fatalf("unexpected manual calls: %v", fake.calls)
	}
}

func TestSelectedPlayerLookupUpdatesInGameCacheAndReusesIt(t *testing.T) {
	fake := &fakeClient{phase: "InProgress"}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client = fake
	e.snapshot = core.Snapshot{Stage: "in-game", Frozen: true, Connected: true, Generation: 5, Participants: fullTestRoster()}
	player, err := e.LookupPlayer(context.Background(), "player-5")
	if err != nil || player.HistoryState != core.HistoryEmpty {
		t.Fatalf("selected lookup failed: %v", err)
	}
	if e.Snapshot().Participants[5].HistoryState != core.HistoryEmpty {
		t.Fatal("manual result not saved in frozen cache")
	}
	before := len(fake.calls)
	if _, err := e.LookupPlayer(context.Background(), "player-5"); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != before {
		t.Fatal("cached player triggered another request")
	}
	if _, err := e.Lookup(context.Background(), "player-5#TEST"); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != before {
		t.Fatal("full-ID lookup missed cache")
	}
	ctx, release, err := e.beginLookup(context.Background())
	if err != nil || ctx == nil {
		t.Fatal("gate setup failed")
	}
	defer release()
	if _, err := e.Lookup(context.Background(), "Other#TEST"); err == nil {
		t.Fatal("parallel manual requests admitted")
	}
}

func TestNextGameRefreshDoesNotInheritOldMatchHistory(t *testing.T) {
	players := fullTestRoster()
	for i := range players {
		players[i].HistoryState = core.HistoryReady
		players[i].History = []core.Match{{GameID: 1}}
	}
	fake := &fakeClient{phase: "GameStart", gameID: 43, roster: fullTestRoster()}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client = fake
	e.matchRosterAttempts = 3
	e.snapshot = core.Snapshot{Stage: "in-game", Phase: "EndOfGame", GameID: 42, Frozen: true, Connected: true, Participants: players}
	e.poll(context.Background())
	snapshot := e.Snapshot()
	if snapshot.GameID != 43 || len(snapshot.Participants[0].History) != 0 || snapshot.Participants[0].HistoryState != core.HistoryPending {
		t.Fatal("new game retained old match data or failed to refresh")
	}
}

func TestNextGameRetriesAfterFailureDespiteCompleteOldRoster(t *testing.T) {
	fake := &fakeClient{phase: "GameStart", rosterErr: errors.New("not ready")}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client = fake
	e.matchRosterAttempts, e.matchRosterComplete = 3, true
	e.snapshot = core.Snapshot{Stage: "in-game", Phase: "EndOfGame", GameID: 42, Frozen: true, Connected: true, Participants: fullTestRoster()}
	e.poll(context.Background())
	if e.matchRosterComplete || e.matchRosterAttempts != 1 {
		t.Fatal("old complete roster suppressed the new match's retry budget")
	}
	fake.phase, fake.rosterErr, fake.gameID, fake.roster = "InProgress", nil, 43, fullTestRoster()
	e.poll(context.Background())
	if e.Snapshot().GameID != 43 || !e.matchRosterComplete || e.matchRosterAttempts != 2 {
		t.Fatal("new match roster did not recover after the first attempt failed")
	}
	before := len(fake.calls)
	e.poll(context.Background())
	if strings.Join(fake.calls[before:], ",") != "phase" {
		t.Fatal("complete new roster caused additional background requests")
	}
}

func TestManualLookupDoesNotPublishIntoAnotherGeneration(t *testing.T) {
	fake := &fakeClient{}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client = fake
	e.snapshot = core.Snapshot{Generation: 5, Stage: "in-game", Frozen: true, Participants: fullTestRoster()}
	fake.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 12*time.Second {
			t.Fatal("manual request has no bounded deadline")
		}
		e.mu.Lock()
		e.snapshot.Generation++
		e.mu.Unlock()
		return core.HistoryResult{Matches: []core.Match{{GameID: 99}}}, nil
	}
	player, err := e.LookupPlayer(context.Background(), "player-5")
	if err != nil || len(player.History) != 1 {
		t.Fatalf("explicit lookup did not return its result: %v", err)
	}
	if e.Snapshot().Participants[5].HistoryState != core.HistoryPending {
		t.Fatal("stale result mutated a different match generation")
	}
}

func TestManualLookupRejectsUnknownIdentityAndUsesOfflineCache(t *testing.T) {
	e := New(nil, nil)
	e.snapshot = core.Snapshot{Stage: "in-game", Frozen: true, Participants: []core.Participant{
		{Key: "cached", PUUID: "cached", DisplayName: "Cached#TEST", HistoryState: core.HistoryReady},
		{Key: "enemy-slot-0", IdentityState: core.IdentityUnavailable},
	}}
	if _, err := e.LookupPlayer(context.Background(), "enemy-slot-0"); err == nil || !strings.Contains(err.Error(), "尚未提供") {
		t.Fatal("unavailable identity was accepted or incorrectly explained")
	}
	if _, err := e.Lookup(context.Background(), "Cached#TEST"); err != nil {
		t.Fatalf("complete-ID cached data unavailable after disconnect: %v", err)
	}
	if _, err := e.LookupPlayer(context.Background(), "cached"); err != nil {
		t.Fatalf("selected-player cache unavailable after disconnect: %v", err)
	}
}

func TestManualRefreshOfNextGameResetsAutomaticBudget(t *testing.T) {
	fake := &fakeClient{phase: "GameStart", gameID: 43, roster: fullTestRoster()[:5]}
	e := New(nil, nil)
	e.client = fake
	e.snapshot = core.Snapshot{Phase: "EndOfGame", Stage: "in-game", Frozen: true, GameID: 42, Participants: fullTestRoster()}
	e.matchRosterAttempts, e.matchRosterComplete = 3, true
	if _, err := e.RefreshRoster(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.matchRosterAttempts != 0 || e.matchRosterComplete {
		t.Fatal("manual next-game refresh kept old completion state")
	}
	fake.roster = fullTestRoster()
	e.poll(context.Background())
	if !e.matchRosterComplete || !completeRoster(e.Snapshot().Participants) {
		t.Fatal("automatic refresh did not complete the manually detected new game")
	}
}

func TestPollDiscardsRosterFromSupersededGeneration(t *testing.T) {
	fake := &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}
	e := New(nil, nil)
	e.client = fake
	fake.rosterFn = func() {
		e.mu.Lock()
		e.snapshot = core.Snapshot{Generation: 10, Phase: "Lobby", Stage: "idle", Connected: true, Participants: []core.Participant{}}
		e.mu.Unlock()
	}
	e.poll(context.Background())
	if snapshot := e.Snapshot(); snapshot.Generation != 10 || snapshot.Phase != "Lobby" || len(snapshot.Participants) != 0 {
		t.Fatal("an obsolete roster overwrote a newer snapshot")
	}
}

func TestManualCacheExpiryRefreshAndFailurePreservesData(t *testing.T) {
	fake := &fakeClient{}
	e := New(nil, nil)
	e.client = fake
	if _, err := e.Lookup(context.Background(), "Manual#TEST"); err != nil {
		t.Fatal(err)
	}
	before := len(fake.calls)
	p, err := e.Lookup(context.Background(), " Manual # TEST ")
	if err != nil || !p.Cached || len(fake.calls) != before {
		t.Fatal("normalized manual result was not cached")
	}
	if p, err = e.Lookup(context.Background(), "\u2066Manual\u2069#\u2066TEST\u2069"); err != nil || !p.Cached || len(fake.calls) != before {
		t.Fatal("clipboard direction markers prevented cache reuse")
	}
	old := time.Now().Add(-6 * time.Minute)
	e.cache["manual"] = func() core.Participant { p := e.cache["manual"]; p.HistoryMeta.FetchedAt = &old; return p }()
	if _, err = e.Lookup(context.Background(), "Manual#TEST"); err != nil || len(fake.calls) == before {
		t.Fatal("expired cache prevented refresh")
	}
	fake.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{}, errors.New("temporary failure")
	}
	if _, err = e.RefreshLookup(context.Background(), "Manual#TEST"); err == nil {
		t.Fatal("forced refresh failure not reported")
	}
	e.client = nil
	p, err = e.Lookup(context.Background(), "Manual#TEST")
	if err != nil || !p.Cached || !p.Stale {
		t.Fatal("failed refresh destroyed offline cache")
	}
}

func TestInvalidCopiedIDDoesNotUseManualRequestLane(t *testing.T) {
	fake := &fakeClient{}
	e := New(nil, nil)
	e.client = fake
	for _, input := range []string{"\u2066\u2069#TEST", "Player#\u2066\u2069", "Player#TEST#extra"} {
		if _, err := e.Lookup(context.Background(), input); err == nil {
			t.Fatal("invalid copied ID accepted")
		}
	}
	if len(fake.calls) != 0 {
		t.Fatal("invalid input issued a source request")
	}
}

func TestManualResultFromOldConnectionCannotEnterNewCache(t *testing.T) {
	fake := &fakeClient{}
	e := New(nil, nil)
	e.client = fake
	fake.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		e.mu.Lock()
		e.clientVersion++
		e.mu.Unlock()
		return core.HistoryResult{}, nil
	}
	if _, err := e.Lookup(context.Background(), "Manual#TEST"); err != nil {
		t.Fatal(err)
	}
	if len(e.cache) != 0 {
		t.Fatal("old connection populated new connection cache")
	}
}
