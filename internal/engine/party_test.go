package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type partyClient struct{ *fakeClient }

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
