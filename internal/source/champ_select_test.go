package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/lcu"
)

func TestSelectionUsesDedicatedSessionNotGameflowOrPlayerQueries(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{Port: 12345})
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/lol-champ-select/v1/session" {
			t.Fatal("selection made unrelated request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"gameId":42,"queueId":2400,"myTeam":[{"puuid":"self","championId":23}],"theirTeam":[{"cellId":5,"championId":24}]}`))}, nil
	})
	id, players, err := (&TencentLCU{client: c}).ChampSelectRoster(context.Background())
	if err != nil || id != 42 || len(players) != 2 || players[1].ChampionID != 24 || calls != 1 {
		t.Fatal("selection source failed")
	}
}

func TestOptionalChatMetadataCannotBreakSelectionModeOrRoster(t *testing.T) {
	for _, chat := range []string{`null`, `"unexpected"`, `{"multiUserChatId":12}`, `{"multiUserChatId":"synthetic-room"}`} {
		c := lcu.NewClient(lcu.Credentials{Port: 12345})
		c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(`{"gameId":42,"queueId":2400,"myTeam":[{"puuid":"self","championId":23}],"theirTeam":[],"chatDetails":` + chat + `}`), nil
		})
		s := &TencentLCU{client: c}
		_, roster, err := s.ChampSelectRoster(context.Background())
		if err != nil || len(roster) != 1 || s.CurrentQueue() != 2400 {
			t.Fatal("optional chat metadata changed core selection behavior", err)
		}
		if chat == `{"multiUserChatId":"synthetic-room"}` && s.SelectionIdentity() != "synthetic-room" {
			t.Fatal("selection identity missing")
		}
	}
}

func TestSelectionVerificationRejectsChangedSelection(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{Port: 12345})
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != champSelectSession {
			t.Fatal("selection verification read an unrelated endpoint")
		}
		return jsonResponse(`{"gameId":42,"queueId":2400,"chatDetails":{"multiUserChatId":"synthetic-room"}}`), nil
	})
	s := &TencentLCU{client: c}
	ctx := context.Background()
	if err := s.VerifySelection(ctx, "synthetic-room", 42, 2400); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		key   string
		game  int64
		queue int
	}{{"other-room", 42, 2400}, {"synthetic-room", 43, 2400}, {"synthetic-room", 42, 450}} {
		if s.VerifySelection(ctx, input.key, input.game, input.queue) == nil {
			t.Fatal("selection verification accepted a different session")
		}
	}
}
