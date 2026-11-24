package lcu

import (
	"context"
	"encoding/json"
)

// Only roster/selection changes wake a bounded HTTP verification. Timer,
// chat, actions and unrelated session fields cannot generate extra reads.
func (c *Client) WatchChampSelect(ctx context.Context, notify func()) error {
	previous := ""
	return c.watchTopic(ctx, champSelectTopic, "/champ-select-events", 256<<10, func(raw []byte) {
		if signature := champSelectSignature(raw); signature != "" && signature != previous {
			previous = signature
			notify()
		}
	})
}

func champSelectSignature(raw []byte) string {
	var frame []json.RawMessage
	if json.Unmarshal(raw, &frame) != nil || len(frame) != 3 {
		return ""
	}
	var code int
	var topic string
	var event struct {
		URI       string          `json:"uri"`
		EventType string          `json:"eventType"`
		Data      json.RawMessage `json:"data"`
	}
	if json.Unmarshal(frame[0], &code) != nil || code != 8 || json.Unmarshal(frame[1], &topic) != nil || topic != champSelectTopic || json.Unmarshal(frame[2], &event) != nil || event.URI != "/lol-champ-select/v1/session" {
		return ""
	}
	if event.EventType == "Delete" {
		return "deleted"
	}
	if event.EventType != "Create" && event.EventType != "Update" {
		return ""
	}
	type member struct {
		CellID             *int   `json:"cellId"`
		AssignedPosition   string `json:"assignedPosition"`
		SelectedPosition   string `json:"selectedPosition"`
		PUUID              string `json:"puuid"`
		ChampionID         *int   `json:"championId"`
		ChampionPickIntent *int   `json:"championPickIntent"`
	}
	var data struct {
		GameID  int64 `json:"gameId"`
		QueueID int   `json:"queueId"`
		Chat    struct {
			ID json.RawMessage `json:"multiUserChatId"`
		} `json:"chatDetails"`
		MyTeam    []member `json:"myTeam"`
		TheirTeam []member `json:"theirTeam"`
	}
	if json.Unmarshal(event.Data, &data) != nil || data.MyTeam == nil || data.TheirTeam == nil {
		return ""
	}
	signature, _ := json.Marshal(data)
	return string(signature) // Memory only; never log or publish the signature.
}
