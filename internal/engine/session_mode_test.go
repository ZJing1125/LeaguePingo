package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

type modeLobbyClient struct {
	*fakeClient
	last source.SessionMetadata
	mode int
}

func (c *modeLobbyClient) ReadLobby(context.Context) ([]core.Participant, source.SessionMetadata, error) {
	return c.roster, source.SessionMetadata{QueueID: c.mode, RoomKey: "same-synthetic-lobby"}, nil
}
func (c *modeLobbyClient) ReadSelection(_ context.Context, previous source.SessionMetadata) (int64, []core.Participant, source.SessionMetadata, error) {
	c.last = previous
	return 42, c.roster, source.SessionMetadata{QueueID: c.mode, SelectionKey: "synthetic-selection"}, nil
}
func (c *modeLobbyClient) ReadGame(context.Context) (int64, []core.Participant, source.SessionMetadata, error) {
	return 42, c.roster, source.SessionMetadata{QueueID: c.mode, MapID: 12, GameMode: "ARAM"}, nil
}

func TestSameLobbySwitchUpdatesModeFromPollAndIgnoresLateResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &modeLobbyClient{fakeClient: &fakeClient{phase: "Lobby", roster: []core.Participant{
		{Key: "only-player", PUUID: "only-player", IdentityState: core.IdentityKnown, Team: "ally"},
	}}, mode: 420}
	e := New(nil, nil)
	e.client, e.clientVersion = client, 1
	e.poll(ctx)
	if s := e.Snapshot(); s.QueueID != 420 || s.ScoutQueue != 420 || len(s.Participants) != 1 {
		t.Fatalf("initial mode/read failed: queue %d, view %d, players %d", s.QueueID, s.ScoutQueue, len(s.Participants))
	}
	if _, err := e.SetScoutQueue(440); err != nil {
		t.Fatal(err)
	}

	old := e.Snapshot()
	results := pollRosterResult{generation: old.Generation, version: 1, epoch: e.phaseEpoch,
		stage: StageParty, phase: PhaseLobby, roster: old.Participants,
		mode: source.SessionMetadata{QueueID: 440, RoomKey: "same-synthetic-lobby"}}
	_, _, sequence, _ := e.phaseEvents.begin(ctx, 1, time.Now())
	e.observeLobby(1, sequence)
	select {
	case <-e.lobbyWake:
	default:
		t.Fatal("mode change event did not schedule a read")
	}
	e.finishPollLocked(results)
	if e.Snapshot().QueueID != 420 {
		t.Fatal("pre-event data replaced the current mode")
	}
	client.mode = 2400
	e.poll(ctx)
	next := e.Snapshot()
	if next.QueueID != 2400 || next.ScoutQueue != 2400 || next.Generation <= old.Generation {
		t.Fatalf("mode-only update should restore auto with the same roster: %+v", next)
	}
	client.mode = 450
	e.poll(ctx)
	if s := e.Snapshot(); s.QueueID != 450 || s.ScoutQueue != 450 {
		t.Fatalf("normal and Mayhem ARAM were not distinguished: %+v", s)
	}
	client.phase = "ChampSelect"
	e.poll(ctx)
	if client.last.QueueID != 450 || e.Snapshot().QueueID != 450 {
		t.Fatal("verified room mode did not reach the selection reader")
	}
}
