package core

import (
	_ "embed"
	"encoding/json"
	"strings"
)

type ScoutMode struct {
	QueueID   int      `json:"queueId"`
	Label     string   `json:"label"`
	Ranked    bool     `json:"ranked"`
	Roles     bool     `json:"roles"`
	Arena     bool     `json:"arena,omitempty"`
	MapID     int      `json:"mapId"`
	GameModes []string `json:"gameModes"`
	Guide     string   `json:"guide"`
}

// Shared with the frontend; exact queues stay independent even if guides share a category.
//
//go:embed scout_modes.json
var scoutModesJSON []byte

var scoutModes = func() []ScoutMode {
	var modes []ScoutMode
	if err := json.Unmarshal(scoutModesJSON, &modes); err != nil {
		panic(err)
	}
	return modes
}()

func ScoutModes() []ScoutMode { return append([]ScoutMode(nil), scoutModes...) }
func ScoutModeFor(queue int) (ScoutMode, bool) {
	for _, mode := range scoutModes {
		if mode.QueueID == queue {
			return mode, true
		}
	}
	return ScoutMode{}, false
}
func (m ScoutMode) Matches(mapID int, game string) bool {
	if mapID != 0 && mapID != m.MapID {
		return false
	}
	if game == "" {
		return true
	}
	for _, value := range m.GameModes {
		if strings.EqualFold(game, value) {
			return true
		}
	}
	return false
}
