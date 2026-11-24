package lcu

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
				case chatSessionPath:
					_, _ = w.Write([]byte(`{"sessionState":"loaded"}`))
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
			if attempted, err := c.SendAnalysisBrief(context.Background(), "synthetic@champsel", "synthetic analysis", audience); !attempted || err != nil || posts != 1 {
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
		if r.URL.Path == chatSessionPath {
			_, _ = w.Write([]byte(`{"sessionState":"loaded"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"new-room","type":"championSelect"}]`))
	})
	if attempted, err := c.SendAnalysisBrief(context.Background(), "old-room", "text", "team"); attempted || err == nil || posts != 0 {
		t.Fatal("sent in game")
	}
	phase = "ChampSelect"
	if attempted, err := c.SendAnalysisBrief(context.Background(), "old-room", "text", "team"); attempted || err == nil || posts != 0 {
		t.Fatal("sent to stale room")
	}
	before := requests
	for _, id := range []string{"../friend", "id/messages", "id?query", "%2e%2e", ""} {
		if attempted, err := c.SendAnalysisBrief(context.Background(), id, "text", "team"); attempted || err == nil {
			t.Fatal("invalid room accepted")
		}
	}
	for _, body := range []string{strings.Repeat("字", 86), "玩家一\n玩家二", "玩家一\u2028玩家二", "\x00", " "} {
		if attempted, err := c.SendAnalysisBrief(context.Background(), "new-room", body, "team"); attempted || err == nil {
			t.Fatal("invalid byte length or multiline text accepted")
		}
	}
	if attempted, err := c.SendAnalysisBrief(context.Background(), "new-room", "text", "all"); attempted || err == nil {
		t.Fatal("arbitrary audience accepted")
	}
	if requests != before {
		t.Fatal("untrusted input caused a request")
	}
}

func TestAnalysisChatDoesNotWriteDuringReconnectOrTreatItAsAnAttempt(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
	}{
		{"initializing", `{"sessionState":"initializing"}`, 200},
		{"reconnecting", `{"sessionState":"connecting"}`, 200},
		{"disconnected", `{"sessionState":"disconnected"}`, 200},
		{"unknown state", `{"sessionState":"unknown"}`, 200},
		{"missing state", `{}`, 200},
		{"unsupported route", `{}`, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var posts atomic.Int32
			c := briefClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/lol-gameflow/v1/gameflow-phase":
					_, _ = w.Write([]byte(`"ChampSelect"`))
				case chatSessionPath:
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.response))
				case conversationsPath:
					_, _ = w.Write([]byte(`[{"id":"open-room","type":"championSelect"}]`))
				default:
					posts.Add(1)
					w.WriteHeader(http.StatusNoContent)
				}
			})
			attempted, err := c.SendAnalysisBrief(context.Background(), "open-room", "valid brief", "team")
			if attempted || err == nil || posts.Load() != 0 {
				t.Fatal("a listed chat room allowed writing without a ready chat session")
			}
		})
	}
}

func TestMultilineHorseBriefIsOneJSONMessage(t *testing.T) {
	body := "[LeaguePingo] 本局五马分析\n1楼(上) 🦄千里马（15场） | 👑院长级大腿\n2楼(野) 🐎上等马（15场） | 近期同队2局\n3楼(中) 🐎中等马（15场） | 4连胜\n4楼(下) 🐂下等马（15场） | 经济转化低\n5楼(辅) 💩纯牛马（15场） | 场均死亡8.0"
	posts := 0
	client := briefClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lol-gameflow/v1/gameflow-phase":
			_, _ = w.Write([]byte(`"ChampSelect"`))
		case chatSessionPath:
			_, _ = w.Write([]byte(`{"sessionState":"loaded"}`))
		case conversationsPath:
			_, _ = w.Write([]byte(`[{"id":"synthetic","type":"championSelect"}]`))
		case conversationsPath + "/synthetic/messages":
			var payload map[string]string
			if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["body"] != body {
				t.Error("multiline payload altered")
			}
			posts++
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	})
	dispatched, err := client.SendAnalysisBrief(context.Background(), "synthetic", body, "team")
	if err != nil || !dispatched || posts != 1 {
		t.Fatal("rich briefing was rejected or split", err, posts)
	}
}
