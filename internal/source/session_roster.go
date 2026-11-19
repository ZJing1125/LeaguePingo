package source

import (
	"context"
	"encoding/json"

	"leaguepingo/internal/core"
)

const gameflowSession = "/lol-gameflow/v1/session"
const champSelectSession = "/lol-champ-select/v1/session"

func (s *TencentLCU) ReadLobby(ctx context.Context) ([]core.Participant, SessionMetadata, error) {
	raw, err := s.client.Get(ctx, "/lol-lobby/v2/lobby")
	if err != nil {
		return nil, SessionMetadata{}, err
	}
	players, err := core.ParseLobbyRoster(raw)
	meta, _ := parseSessionMetadata(raw, true)
	return players, meta, err
}

func (s *TencentLCU) ReadSelection(ctx context.Context, previous SessionMetadata) (int64, []core.Participant, SessionMetadata, error) {
	raw, err := s.client.Get(ctx, champSelectSession)
	if err != nil {
		return 0, nil, SessionMetadata{}, err
	}
	id, players, err := core.ParseChampSelectSession(raw)
	if err != nil {
		return id, nil, SessionMetadata{}, err
	}
	fields := readSelectionMetadata(raw)
	meta := SessionMetadata{SelectionKey: fields.Key}
	switch {
	case fields.QueueID > 0:
		meta.QueueID = fields.QueueID
	case previous.QueueID > 0 && (previous.SelectionKey == "" || previous.SelectionKey == fields.Key):
		meta.QueueID, meta.MapID, meta.GameMode = previous.QueueID, previous.MapID, previous.GameMode
	default:
		if raw, readErr := s.client.Get(ctx, gameflowSession); readErr == nil {
			var identity struct {
				Phase    string `json:"phase"`
				GameData struct {
					GameID int64 `json:"gameId"`
				} `json:"gameData"`
			}
			if json.Unmarshal(raw, &identity) == nil && identity.GameData.GameID > 0 &&
				(id > 0 && identity.GameData.GameID == id || id == 0 && identity.Phase == "ChampSelect") {
				mode, _ := parseSessionMetadata(raw, false)
				meta.QueueID, meta.MapID, meta.GameMode = mode.QueueID, mode.MapID, mode.GameMode
			}
		}
	}
	return id, players, meta, nil
}

func (s *TencentLCU) ReadGame(ctx context.Context) (int64, []core.Participant, SessionMetadata, error) {
	var id int64
	var players []core.Participant
	var firstErr error
	var meta SessionMetadata
	if raw, err := s.client.Get(ctx, gameflowSession); err == nil {
		meta, _ = parseSessionMetadata(raw, false)
		self, selfErr := s.currentPUUID(ctx)
		if selfErr == nil {
			id, players, firstErr = core.ParseGameflowRosterForPlayer(raw, self)
		} else {
			firstErr = selfErr
		}
	} else {
		firstErr = err
	}
	if len(players) >= 10 && knownCount(players) == len(players) {
		return id, players, meta, nil
	}
	if raw, err := s.client.Get(ctx, champSelectSession); err == nil {
		if selection, parseErr := core.ParseChampSelectRoster(raw); parseErr == nil &&
			(knownCount(selection) > knownCount(players) || (knownCount(selection) == knownCount(players) && len(selection) > len(players))) {
			if firstErr != nil {
				meta = SessionMetadata{}
			}
			return id, selection, meta, nil
		}
	}
	if len(players) > 0 {
		return id, players, meta, nil
	}
	return id, players, meta, firstErr
}

type selectionMetadata struct {
	GameID  int64
	QueueID int
	Key     string
}

func readSelectionMetadata(raw []byte) selectionMetadata {
	var fields struct {
		GameID  int64           `json:"gameId"`
		QueueID int             `json:"queueId"`
		Chat    json.RawMessage `json:"chatDetails"`
	}
	var chat struct {
		ID string `json:"multiUserChatId"`
	}
	// Broken optional chat fields must leave the selection roster usable.
	_ = json.Unmarshal(raw, &fields)
	_ = json.Unmarshal(fields.Chat, &chat)
	return selectionMetadata{fields.GameID, fields.QueueID, chat.ID}
}
