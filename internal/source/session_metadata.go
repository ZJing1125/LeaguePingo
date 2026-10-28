package source

import "encoding/json"

func (s *TencentLCU) SessionMetadata() SessionMetadata {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	return s.meta
}
func (s *TencentLCU) rememberMetadata(raw []byte, lobby bool) {
	var v struct {
		GameData struct {
			Queue struct {
				ID int `json:"id"`
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
