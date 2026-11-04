package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type partyClient struct{ *fakeClient }

type loadingLobbyClient struct{ *fakeClient }

func (*loadingLobbyClient) Lobby(context.Context) ([]core.Participant, error) {
	return []core.Participant{{Key: "player-0", PUUID: "player-0", DisplayName: "玩家名称暂不可用", Team: "ally", IdentityState: core.IdentityKnown, HistoryState: core.HistoryPending}}, nil
}

func TestDodgeReturnsToLobbyWithoutLoadingOrOldOpponents(t *testing.T) {
	fake := &loadingLobbyClient{&fakeClient{phase: "Lobby"}}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.client, e.clientVersion = fake, 1
	cached := core.Participant{Key: "player-0", PUUID: "player-0", ConnectionID: 1, DisplayName: "Verified#TEST", Team: "ally", IdentityState: core.IdentityKnown}
	core.ApplyHistory(&cached, core.HistoryResult{Matches: []core.Match{{GameID: 99}}})
	e.cachePlayerLocked(cached)
	old := fullTestRoster()
	for i := range old {
		old[i].ConnectionID = 1
	}
	old[0].DisplayName = "玩家资料加载中"
	e.snapshot = core.Snapshot{ConnectionID: 1, Stage: "preparing", Phase: "ChampSelect", GameID: 42, Connected: true, Participants: old}
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	s := e.Snapshot()
	if s.Stage != "party" || s.Phase != "Lobby" || s.GameID != 0 || len(s.Participants) != 1 || s.Participants[0].Team != "ally" {
		t.Fatalf("old selection roster survived dodge: stage=%s phase=%s game=%d players=%d", s.Stage, s.Phase, s.GameID, len(s.Participants))
	}
	if s.Participants[0].DisplayName != "Verified#TEST" || s.Participants[0].HistoryState != core.HistoryReady {
		t.Fatalf("cached verified player reverted to a loading placeholder: name=%q state=%s", s.Participants[0].DisplayName, s.Participants[0].HistoryState)
	}
}

func TestDodgeCancelsWorkerAndClearsAbandonedLoading(t *testing.T) {
	e := New(nil, nil)
	e.client = &loadingLobbyClient{&fakeClient{phase: "Lobby"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.cancel, e.prepareRunning = cancel, true
	p := fullTestRoster()[0]
	p.HistoryState = core.HistoryLoading
	e.snapshot = core.Snapshot{Stage: "preparing", Phase: "ChampSelect", GameID: 42, Participants: []core.Participant{p}}
	e.observePhase(0, 0, "Lobby")
	if ctx.Err() == nil {
		t.Fatal("dodge notification did not immediately cancel selection worker")
	}
	e.mu.Lock()
	// Observe the replacement before a new worker starts: this models a
	// temporarily unavailable source after the phase transition.
	e.client = nil
	e.replaceLocked("party", "Lobby", 0, []core.Participant{p}, "")
	e.mu.Unlock()
	if e.Snapshot().Participants[0].HistoryState == core.HistoryLoading {
		t.Fatal("cancelled selection worker left the lobby permanently loading")
	}
}

func TestCachedNameSurvivesPlaceholderProfileWrite(t *testing.T) {
	e := New(nil, nil)
	p := fullTestRoster()[0]
	e.cachePlayerLocked(p)
	placeholder := p
	placeholder.DisplayName = "玩家名称暂不可用"
	e.cachePlayerLocked(placeholder)
	if cachedPlayerForTest(e, p.PUUID).DisplayName != p.DisplayName {
		t.Fatal("profile-only cache write erased known name")
	}
	placeholder.ConnectionID++
	e.cachePlayerLocked(placeholder)
	if cachedPlayerForTest(e, p.PUUID).DisplayName == p.DisplayName {
		t.Fatal("name was inherited across connections")
	}
}

func (p *partyClient) Lobby(context.Context) ([]core.Participant, error) {
	p.record("lobby")
	return fullTestRoster()[:2], nil
}

func TestPartyPreparationDoesNotInventOpponentsAndResetsBudgetForChampSelect(t *testing.T) {
	fake := &partyClient{&fakeClient{phase: "Lobby"}}
	e := New(func() (Client, error) { return fake, nil }, nil)
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	s := e.Snapshot()
	if s.Stage != "party" || len(s.Participants) != 2 || s.ReadyCount != 2 {
		t.Fatal("party data not prepared")
	}
	for _, p := range s.Participants {
		if p.Team != "ally" {
			t.Fatal("invented opponents")
		}
	}
	token := e.prepareToken
	fake.mu.Lock()
	fake.phase = "ReadyCheck"
	fake.mu.Unlock()
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	if e.prepareToken != token {
		t.Fatal("ready check unnecessarily restarted party budget")
	}
	fake.mu.Lock()
	fake.phase = "ChampSelect"
	fake.roster = fullTestRoster()
	fake.gameID = 42
	fake.mu.Unlock()
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	if e.prepareToken == token || e.Snapshot().ReadyCount != 10 {
		t.Fatal("champ select not given own budget")
	}
	if e.pollInterval() != 3*time.Second {
		t.Fatal("unexpected champ select polling")
	}
}
