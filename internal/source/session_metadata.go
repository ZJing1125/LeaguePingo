package source

import (
	"encoding/json"
	"strings"
)

func (s *TencentLCU) SessionMetadata() SessionMetadata {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	return s.meta
}
func (s *TencentLCU) rememberMetadata(raw []byte, lobby bool) {
	m, ok := parseSessionMetadata(raw, lobby)
	if ok {
		s.rememberSession(m)
	}
}

func (s *TencentLCU) rememberSession(m SessionMetadata) {
	s.metaMu.Lock()
	s.meta = m
	s.metaMu.Unlock()
}

func parseSessionMetadata(raw []byte, lobby bool) (SessionMetadata, bool) {
	var v struct {
		PartyID string `json:"partyId"`
		Map     struct {
			ID       int    `json:"id"`
			GameMode string `json:"gameMode"`
		} `json:"map"`
		GameData struct {
			Queue struct {
				ID       int    `json:"id"`
				GameMode string `json:"gameMode"`
				MapID    int    `json:"mapId"`
			} `json:"queue"`
			GameMode string `json:"gameMode"`
			Map      struct {
				ID int `json:"id"`
			} `json:"map"`
			MapID       int    `json:"mapId"`
			GameVersion string `json:"gameVersion"`
		} `json:"gameData"`
		GameConfig struct {
			Custom   bool   `json:"isCustom"`
			QueueID  int    `json:"queueId"`
			GameMode string `json:"gameMode"`
			MapID    int    `json:"mapId"`
		} `json:"gameConfig"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return SessionMetadata{}, false
	}
	m := SessionMetadata{QueueID: v.GameData.Queue.ID, GameMode: v.GameData.GameMode, MapID: v.GameData.Map.ID, Version: v.GameData.GameVersion}
	if v.GameData.Queue.GameMode != "" {
		m.GameMode = v.GameData.Queue.GameMode
	}
	if v.GameData.Queue.MapID != 0 {
		m.MapID = v.GameData.Queue.MapID
	}
	// A disagreement is unusable metadata, never permission to alias a mode.
	if v.Map.ID != 0 {
		if m.MapID != 0 && m.MapID != v.Map.ID {
			m.GameMode = "CONFLICT"
		}
		m.MapID = v.Map.ID
	}
	if v.Map.GameMode != "" {
		if m.GameMode != "" && !strings.EqualFold(m.GameMode, v.Map.GameMode) {
			m.GameMode = "CONFLICT"
		} else {
			m.GameMode = v.Map.GameMode
		}
	}
	if m.MapID == 0 {
		m.MapID = v.GameData.MapID
	}
	if lobby {
		m = lobbyMetadata(v.PartyID, v.GameConfig.QueueID, v.GameConfig.MapID, v.GameConfig.GameMode, v.GameConfig.Custom)
	}
	return m, true
}

func lobbyMetadata(room string, queue, mapID int, mode string, custom bool) SessionMetadata {
	m := SessionMetadata{RoomKey: room}
	if !custom && queue > 0 {
		m.QueueID, m.GameMode = queue, mode
		m.MapID = max(0, mapID)
		// The ARAM lobby can select a random map. Its exact queue identifies
		// the ruleset before the client has chosen the actual map.
		if queue == 450 || queue == 2400 {
			m.MapID = 0
		}
	}
	return m
}
