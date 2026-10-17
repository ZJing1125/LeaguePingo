package lcu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"leaguepingo/internal/diagnostics"
)

const phaseTopic = "OnJsonApiEvent_lol-gameflow_v1_gameflow-phase"
const champSelectTopic = "OnJsonApiEvent_lol-champ-select_v1_session"

// WatchPhase subscribes only to the local phase endpoint, never the entire LCU
// event stream. Credentials and event payloads must not reach logs or the UI.
func (c *Client) WatchPhase(ctx context.Context, notify func(string)) error {
	previous := ""
	return c.watchTopic(ctx, phaseTopic, "/phase-events", 16<<10, func(payload []byte) {
		if phase := parsePhaseEvent(payload); phase != "" && phase != previous {
			previous = phase
			notify(phase)
		}
	})
}

func (c *Client) watchTopic(ctx context.Context, topic, category string, limit int64, notify func([]byte)) error {
	client := *c.HTTP
	client.Timeout = 0 // Only the handshake is bounded; an idle stream is normal.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("riot:"+c.Cred.Password)))
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	started := time.Now()
	conn, response, err := websocket.Dial(dialCtx, fmt.Sprintf("wss://127.0.0.1:%d/", c.Cred.Port), &websocket.DialOptions{
		HTTPClient: &client, HTTPHeader: headers,
	})
	cancel()
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	diagnostics.Default.Record(category, time.Since(started), status, err != nil)
	if err != nil {
		return errors.New("本地阶段通知暂不可用，继续有限轮询")
	}
	defer conn.CloseNow()
	conn.SetReadLimit(limit)
	writeCtx, stopWrite := context.WithTimeout(ctx, 3*time.Second)
	err = conn.Write(writeCtx, websocket.MessageText, []byte(`[5,"`+topic+`"]`))
	stopWrite()
	if err != nil {
		return errors.New("本地阶段通知订阅失败，继续有限轮询")
	}
	for {
		kind, payload, err := conn.Read(ctx)
		if err != nil {
			return errors.New("本地阶段通知已断开，继续有限轮询")
		}
		if kind != websocket.MessageText {
			continue
		}
		notify(payload)
	}
}

func parsePhaseEvent(raw []byte) string {
	var frame []json.RawMessage
	if json.Unmarshal(raw, &frame) != nil || len(frame) != 3 {
		return ""
	}
	var code int
	var topic string
	var event struct {
		URI       string `json:"uri"`
		EventType string `json:"eventType"`
		Data      string `json:"data"`
	}
	if json.Unmarshal(frame[0], &code) != nil || code != 8 || json.Unmarshal(frame[1], &topic) != nil || topic != phaseTopic || json.Unmarshal(frame[2], &event) != nil {
		return ""
	}
	if event.URI != "/lol-gameflow/v1/gameflow-phase" || (event.EventType != "Update" && event.EventType != "Create") {
		return ""
	}
	switch event.Data {
	case "None", "Lobby", "Matchmaking", "ReadyCheck", "ChampSelect", "GameStart", "InProgress", "Reconnect", "WaitingForStats", "PreEndOfGame", "EndOfGame", "WatchInProgress", "TerminatedInError":
		return event.Data
	}
	return ""
}
