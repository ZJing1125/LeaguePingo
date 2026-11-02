package guides

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const compactFixture = `class Result: champion,data
class Data: core_items,runes,skills,trends
class Item: ids,ids_names,play,win
class Rune: primary_rune_ids,primary_rune_names,secondary_rune_ids,secondary_rune_names,stat_mod_ids,stat_mod_names
class Skill: order
class Trends: win
class Trend: version

Result("正义巨像",Data(Item([3152,3157],["火箭腰带","中娅"],10,6),Rune([8229],["奥术彗星"],[8444],["复苏之风"],[5008],["适应之力"]),Skill([1,3,2,4]),Trends(Trend("16.21"))))`

func TestCompactAndJSONNormalization(t *testing.T) {
	value, err := decodeCompact(compactFixture)
	if err != nil {
		t.Fatal(err)
	}
	q := Query{Mode: "ranked", Champion: 3, Position: "mid"}
	g, err := normalize(q, value.(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	if g.Version != "16.21" || len(g.Builds) != 1 || *g.Builds[0].Options[0].WinRate != 60 || len(g.Runes) != 3 || strings.Join(g.Skills, "") != "QEWR" {
		t.Fatalf("bad guide %+v", g)
	}
	var root map[string]any
	_ = json.Unmarshal([]byte(`{"data":{"core_items":[{"ids":[1],"play":0}],"opponent_champion_tip":"<script>text only</script>"}}`), &root)
	g, err = normalize(q, root)
	if err != nil || g.Builds[0].Options[0].WinRate != nil || len(g.Tips) != 1 {
		t.Fatal("missing data must not become zero")
	}
	for _, bad := range []string{`__import__("os").system("id")`, compactFixture + `; run()`, `class R: x\nR(Unknown(1))`, strings.Repeat("[", 100) + "0" + strings.Repeat("]", 100)} {
		if _, err := decodeCompact(bad); err == nil {
			t.Fatal("accepted executable or invalid content")
		}
	}
}
func TestOPGGProtocolSSEAndPrivacy(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" {
			t.Error("wrong method")
		}
		var req struct {
			ID     int            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "synthetic")
			fmt.Fprint(w, `{"result":{"protocolVersion":"2025-06-18"}}`)
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			if r.Header.Get("Mcp-Session-Id") != "synthetic" || r.Header.Get("MCP-Protocol-Version") == "" {
				t.Error("missing handshake")
			}
			args := req.Params["arguments"].(map[string]any)
			if args["champion"] != "GALIO" || args["game_mode"] != "aram" || len(args) != 5 {
				t.Error("unexpected arguments or identity sent")
			}
			response, _ := json.Marshal(map[string]any{"result": map[string]any{"content": []map[string]string{{"type": "text", "text": compactFixture}}}})
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", response)
		default:
			t.Error("unexpected RPC")
		}
	}))
	defer srv.Close()
	o := &OPgg{http: srv.Client(), endpoint: srv.URL}
	g, err := o.Fetch(context.Background(), Query{Mode: "aram", Champion: 3, Position: "mid"})
	if err != nil || g == nil || calls != 3 {
		t.Fatalf("%v calls %d", err, calls)
	}
	if _, err = o.Fetch(context.Background(), Query{Mode: "aram", Champion: 3, Position: "mid"}); err != nil || calls != 4 {
		t.Fatalf("session not reused: %v calls %d", err, calls)
	}
	o.sessionExpiry = time.Now().Add(-time.Second)
	if _, err = o.Fetch(context.Background(), Query{Mode: "aram", Champion: 3, Position: "mid"}); err != nil || calls != 7 {
		t.Fatalf("expired session not renewed: %v calls %d", err, calls)
	}
	if _, err = o.Fetch(context.Background(), Query{Mode: "aram", Champion: 3, Position: "mid", Opponent: 103}); err == nil || calls != 7 {
		t.Fatal("ARAM queried lane matchup")
	}
}

func TestOPGGExpiredSessionReinitializesOnce(t *testing.T) {
	initializes, notifications, tools := 0, 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "initialize":
			initializes++
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("session-%d", initializes))
			fmt.Fprint(w, `{"result":{"protocolVersion":"2025-06-18"}}`)
		case "notifications/initialized":
			notifications++
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			tools++
			if tools == 2 && r.Header.Get("Mcp-Session-Id") == "session-1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			response, _ := json.Marshal(map[string]any{"result": map[string]any{"content": []map[string]string{{"type": "text", "text": compactFixture}}}})
			_, _ = w.Write(response)
		default:
			t.Errorf("unexpected method %q", req.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	o := &OPgg{http: srv.Client(), endpoint: srv.URL}
	query := Query{Mode: "ranked", Champion: 3, Position: "mid"}
	if _, err := o.Fetch(context.Background(), query); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Fetch(context.Background(), query); err != nil {
		t.Fatal(err)
	}
	if initializes != 2 || notifications != 2 || tools != 3 {
		t.Fatalf("unexpected session recovery: init=%d notifications=%d tools=%d", initializes, notifications, tools)
	}
}
func TestLiveOPGG(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_TEST_OPGG") != "1" {
		t.Skip("public-network opt-in")
	}
	for _, q := range []Query{{Mode: "ranked", Champion: 3, Position: "mid"}, {Mode: "aram", Champion: 3, Position: "mid"}, {Mode: "ranked", Champion: 3, Position: "mid", Opponent: 103}, {Mode: "flex", Champion: 3, Position: "mid"}} {
		ctx, c := context.WithTimeout(context.Background(), 8*time.Second)
		g, err := NewOPgg().Fetch(ctx, q)
		c()
		if err != nil {
			t.Fatal(q.Mode, err)
		}
		t.Logf("%s matchup=%t: %d build sections, %d runes, %d tips", q.Mode, q.Opponent > 0, len(g.Builds), len(g.Runes), len(g.Tips))
	}
}
