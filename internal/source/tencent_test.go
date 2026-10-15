package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/lcu"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestTencentLCUAdapter(t *testing.T) {
	var paths []string
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "test"})
	client.HTTP.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.RequestURI())
		var body string
		switch {
		case strings.Contains(request.URL.Path, "gameflow-phase"):
			body = `"ChampSelect"`
		case strings.Contains(request.URL.Path, "gameflow/v1/session"):
			body = `{"gameData":{"gameId":42,"teamOne":[{"puuid":"p1"}],"teamTwo":[]}}`
		case strings.Contains(request.URL.Path, "champ-select/v1/session"):
			body = `{"myTeam":[{"puuid":"p1"}],"theirTeam":[{"puuid":""}]}`
		case strings.Contains(request.URL.Path, "summoners/puuid"):
			body = `{"gameName":"Player","tagLine":"CN1"}`
		case strings.Contains(request.URL.Path, "match-history"):
			body = `{"games":{"games":[{"gameId":10,"participants":[{"participantId":1,"championId":12,"stats":{"win":true,"kills":8}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"p1"}}]}]}}`
		case strings.Contains(request.URL.Path, "summoners"):
			body = `{"puuid":"p1","gameName":"Player","tagLine":"CN1"}`
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	s := &TencentLCU{client: client}
	ctx := context.Background()
	phase, err := s.Phase(ctx)
	if err != nil || phase != "ChampSelect" {
		t.Fatalf("phase=%q err=%v", phase, err)
	}
	gameID, roster, err := s.Roster(ctx)
	if err != nil || gameID != 42 || len(roster) != 2 || roster[1].IdentityState != "unavailable" {
		t.Fatalf("roster=%+v id=%d err=%v", roster, gameID, err)
	}
	name, err := s.Name(ctx, "p1")
	if err != nil || name != "Player#CN1" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	matches, err := s.History(ctx, "p1")
	if err != nil || len(matches) != 1 || matches[0].ChampionID != 12 {
		t.Fatalf("matches=%+v err=%v", matches, err)
	}
	player, err := s.FindByRiotID(ctx, "Player#CN1")
	if err != nil || player.PUUID != "p1" {
		t.Fatalf("player=%+v err=%v", player, err)
	}
	if _, err := s.FindByRiotID(ctx, "Player"); err == nil {
		t.Fatal("partial Riot ID accepted")
	}
	if len(paths) != 6 {
		t.Fatalf("unexpected request count: %v", paths)
	}
}
