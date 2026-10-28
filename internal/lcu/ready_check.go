package lcu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"leaguepingo/internal/core"
)

const readyCheckPath = "/lol-matchmaking/v1/ready-check"
const readyCheckTopic = "OnJsonApiEvent_lol-matchmaking_v1_ready-check"

func (c *Client) ReadyCheck(ctx context.Context) (core.ReadyCheck, error) {
	raw, err := c.Get(ctx, readyCheckPath)
	if err != nil {
		return core.ReadyCheck{}, err
	}
	var state core.ReadyCheck
	if json.Unmarshal(raw, &state) != nil || state.State == "" || state.PlayerResponse == "" {
		return core.ReadyCheck{}, errors.New("无法确认本人的对局响应状态")
	}
	return state, nil
}

// Deliberately no generic mutation method is exposed to the desktop bridge.
func (c *Client) AcceptReadyCheck(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, readyCheckPath+"/accept", nil)
	return err
}

func (c *Client) WatchReadyCheck(ctx context.Context, notify func()) error {
	return c.watchTopic(ctx, readyCheckTopic, "/ready-check-events", 16<<10, func(raw []byte) {
		var frame []json.RawMessage
		if json.Unmarshal(raw, &frame) != nil || len(frame) != 3 {
			return
		}
		var code int
		var topic string
		var event struct {
			URI       string `json:"uri"`
			EventType string `json:"eventType"`
		}
		if json.Unmarshal(frame[0], &code) == nil && code == 8 && json.Unmarshal(frame[1], &topic) == nil && topic == readyCheckTopic && json.Unmarshal(frame[2], &event) == nil && event.URI == readyCheckPath && (event.EventType == "Update" || event.EventType == "Create" || event.EventType == "Delete") {
			notify() // Wake only: always verify via HTTP before acting.
		}
	})
}
