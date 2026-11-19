package lcu

import "encoding/json"

const lobbyTopic = "OnJsonApiEvent_lol-lobby_v2_lobby"

func lobbyEventSignature(raw []byte) string {
	var frame []json.RawMessage
	if json.Unmarshal(raw, &frame) != nil || len(frame) != 3 {
		return ""
	}
	var code int
	var topic string
	var event struct {
		URI  string `json:"uri"`
		Type string `json:"eventType"`
		Data struct {
			PartyID string `json:"partyId"`
			Config  *struct {
				Queue  int    `json:"queueId"`
				Map    int    `json:"mapId"`
				Mode   string `json:"gameMode"`
				Custom bool   `json:"isCustom"`
			} `json:"gameConfig"`
		} `json:"data"`
	}
	if json.Unmarshal(frame[0], &code) != nil || code != 8 ||
		json.Unmarshal(frame[1], &topic) != nil || topic != lobbyTopic ||
		json.Unmarshal(frame[2], &event) != nil || event.URI != "/lol-lobby/v2/lobby" {
		return ""
	}
	switch event.Type {
	case "Delete":
		return "deleted"
	case "Update", "Create":
		if event.Data.Config != nil {
			key, _ := json.Marshal(event.Data)
			return string(key) // Transient deduplication key, never logged.
		}
	}
	return ""
}
