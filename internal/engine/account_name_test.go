package engine

import (
	"leaguepingo/internal/core"
	"testing"
)

type namedAccountClient struct {
	*fakeClient
	name string
}

func (c *namedAccountClient) CachedAccountName(string) string { return c.name }

func TestLateAccountNamePublishesWithoutReplacingHistory(t *testing.T) {
	e := &Engine{clientVersion: 4, accountIdentity: "self"}
	e.client = &namedAccountClient{fakeClient: &fakeClient{}, name: "Ready#TEST"}
	p := core.Participant{Key: "self", PUUID: "self", ConnectionID: 4, History: []core.Match{{GameID: 17}}}
	e.players.remember(p)
	e.acceptAccountNameLocked()
	got, _ := e.players.player("self")
	if got.DisplayName != "Ready#TEST" || len(got.History) != 1 || got.History[0].GameID != 17 {
		t.Fatal("late identity replaced prepared data")
	}
	e.clientVersion = 5
	e.client = &namedAccountClient{fakeClient: &fakeClient{}, name: "Other#TEST"}
	e.acceptAccountNameLocked()
	got, _ = e.players.player("self")
	if got.DisplayName != "Ready#TEST" {
		t.Fatal("old connection was changed")
	}
}
