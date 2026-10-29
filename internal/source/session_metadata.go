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
	var v struct {
		Map struct {
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
			QueueID  int    `json:"queueId"`
			GameMode string `json:"gameMode"`
			MapID    int    `json:"mapId"`
		} `json:"gameConfig"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return
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
		m = SessionMetadata{QueueID: v.GameConfig.QueueID, GameMode: v.GameConfig.GameMode, MapID: v.GameConfig.MapID}
	}
	s.metaMu.Lock()
	s.meta = m
	s.metaMu.Unlock()
}
