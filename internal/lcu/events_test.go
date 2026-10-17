package lcu

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWatchPhaseUsesSingleTopicDeduplicatesAndStops(t *testing.T) {
	serverDone := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		if r.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("riot:synthetic")) {
			t.Error("missing local authentication")
			w.WriteHeader(401)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error("test stream upgrade failed")
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, subscription, err := conn.Read(ctx)
		if err != nil || string(subscription) != `[5,"`+phaseTopic+`"]` {
			t.Error("not subscribed exclusively to phase topic")
			return
		}
		for _, frame := range []string{
			`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":"InProgress"}]`,
			`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":"InProgress"}]`,
			`[8,"OnJsonApiEvent",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":"Lobby"}]`,
			`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":"EndOfGame"}]`,
		} {
			if conn.Write(ctx, websocket.MessageText, []byte(frame)) != nil {
				return
			}
		}
		_, _, _ = conn.Read(ctx) // Cancellation must close the client side.
	}))
	defer server.Close()
	client := testEventClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan string, 4)
	done := make(chan error, 1)
	go func() { done <- client.WatchPhase(ctx, func(s string) { updates <- s }) }()
	for _, expected := range []string{"InProgress", "EndOfGame"} {
		select {
		case got := <-updates:
			if got != expected {
				t.Fatalf("phase got %s, expected %s", got, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no phase notification")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not stop on cancellation")
	}
	if len(updates) != 0 {
		t.Fatal("duplicate or wrong topic was emitted")
	}
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("local stream connection not closed")
	}
}

func testEventClient(t *testing.T, address string) *Client {
	t.Helper()
	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(Credentials{Port: port, Password: "synthetic"})
}

func TestWatchPhaseDoesNotFollowRedirectOrLeakDetails(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/synthetic-secret", http.StatusFound)
	}))
	defer server.Close()
	err := testEventClient(t, server.URL).WatchPhase(context.Background(), func(string) { t.Error("unexpected event") })
	if err == nil || hits.Load() != 0 || strings.Contains(err.Error(), "synthetic") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("redirect followed or event error leaked request details")
	}
}

func TestParsePhaseEventRejectsUnrelatedOrUnknownFrames(t *testing.T) {
	for _, raw := range []string{
		`null`, `{}`, `[]`, `invalid`, `[8]`,
		`[5,"` + phaseTopic + `",{}]`,
		`[8,"` + phaseTopic + `",{"uri":"/other","eventType":"Update","data":"Lobby"}]`,
		`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Delete","data":"Lobby"}]`,
		`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":"NewUnverifiedPhase"}]`,
		`[8,"` + phaseTopic + `",{"uri":"/lol-gameflow/v1/gameflow-phase","eventType":"Update","data":{}}]`,
	} {
		if parsePhaseEvent([]byte(raw)) != "" {
			t.Fatal("unverified frame accepted")
		}
	}
}
