package lcu

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func briefClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewTLSServer(handler)
	t.Cleanup(s.Close)
	return &Client{HTTP: s.Client(), Cred: Credentials{Port: s.Listener.Addr().(*net.TCPAddr).Port, Password: "synthetic"}}
}

func TestAnalysisChatOnlyUsesCurrentChampSelectAndFixedAudience(t *testing.T) {
	for _, status := range []int{200, 201, 204} {
		for _, audience := range []string{"team", "self"} {
			posts := 0
			c := briefClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/lol-gameflow/v1/gameflow-phase":
					_, _ = w.Write([]byte(`"ChampSelect"`))
				case conversationsPath:
					_, _ = w.Write([]byte(`[{"id":"synthetic@champsel","type":"championSelect"},{"id":"friend-chat","type":"chat"}]`))
				case conversationsPath + "/synthetic@champsel/messages":
					if r.Method != "POST" {
						t.Error("unexpected chat verb")
					}
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					kind := "chat"
					if audience == "self" {
						kind = "celebration"
					}
					if body["type"] != kind || body["body"] != "synthetic analysis" {
						t.Error("wrong audience/body")
					}
					posts++
					w.WriteHeader(status)
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(404)
				}
			})
			if err := c.SendAnalysisBrief(context.Background(), "synthetic@champsel", "synthetic analysis", audience); err != nil || posts != 1 {
				t.Fatalf("chat status %d: %v / %d", status, err, posts)
			}
		}
	}
}

func TestAnalysisChatRejectsStaleRoomsPhasesAndUntrustedInput(t *testing.T) {
	requests, posts := 0, 0
	phase := "InProgress"
	c := briefClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == "POST" {
			posts++
			w.WriteHeader(500)
			return
		}
		if r.URL.Path == "/lol-gameflow/v1/gameflow-phase" {
			_ = json.NewEncoder(w).Encode(phase)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"new-room","type":"championSelect"}]`))
	})
	if c.SendAnalysisBrief(context.Background(), "old-room", "text", "team") == nil || posts != 0 {
		t.Fatal("sent in game")
	}
	phase = "ChampSelect"
	if c.SendAnalysisBrief(context.Background(), "old-room", "text", "team") == nil || posts != 0 {
		t.Fatal("sent to stale room")
	}
	before := requests
	for _, id := range []string{"../friend", "id/messages", "id?query", "%2e%2e", ""} {
		if c.SendAnalysisBrief(context.Background(), id, "text", "team") == nil {
			t.Fatal("invalid room accepted")
		}
	}
	if c.SendAnalysisBrief(context.Background(), "new-room", strings.Repeat("字", 451), "team") == nil {
		t.Fatal("long text accepted")
	}
	if c.SendAnalysisBrief(context.Background(), "new-room", "text", "all") == nil {
		t.Fatal("arbitrary audience accepted")
	}
	if requests != before {
		t.Fatal("untrusted input caused a request")
	}
}
