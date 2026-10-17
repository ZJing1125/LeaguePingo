package source

import (
	"context"
	"errors"
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
		case request.URL.Path == "/lol-summoner/v1/current-summoner":
			body = `{"puuid":"p1"}`
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
	if len(paths) != 7 {
		t.Fatalf("unexpected request count: %v", paths)
	}
}

func TestLookupUsesCompleteEscapedRiotIDAndOnlyNameParameter(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/lol-summoner/v1/summoners" || len(query) != 1 || query.Get("name") != "测试 + 玩家#CN1" || r.URL.Fragment != "" {
			t.Fatal("lookup does not match the client's single-name-parameter contract")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"puuid":"test-account","gameName":"测试 + 玩家","tagLine":"CN1"}`))}, nil
	})
	player, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), " 测试 + 玩家 # CN1 ")
	if err != nil || player.PUUID != "test-account" {
		t.Fatalf("complete ID lookup failed: %v", err)
	}
}

func TestLookupRejectsWrongIdentityAndExplainsFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{"wrong tag", 200, `{"puuid":"p1","gameName":"Player","tagLine":"OTHER"}`, "无法确认"},
		{"wrong name", 200, `{"puuid":"p1","gameName":"Other","tagLine":"CN1"}`, "无法确认"},
		{"missing identity", 200, `{"gameName":"Player","tagLine":"CN1"}`, "无法确认"},
		{"bad JSON", 200, `invalid`, "返回格式异常"},
		{"bad request", 400, `sensitive response must not be logged`, "玩家身份查询失败"},
		{"not found", 404, `not found`, "玩家身份查询失败"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
			client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			_, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), "Player#CN1")
			if err == nil || !strings.Contains(err.Error(), test.message) || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("missing or unsafe identity error: %v", err)
			}
		})
	}
}

func TestInvalidIDMakesNoRequest(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{})
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid ID sent to client")
		return nil, errors.New("unexpected request")
	})
	for _, id := range []string{"", "Player", "#TAG", "Player#", "Player#  "} {
		if _, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), id); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
}

func TestActiveRosterOrientsTeamsWithoutChampSelectOrHistoryQueries(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
	selfCalls := 0
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/lol-gameflow/v1/session":
			body = `{"gameData":{"gameId":42,"teamOne":[{"puuid":"e1"},{"puuid":"e2"},{"puuid":"e3"},{"puuid":"e4"},{"puuid":"e5"}],"teamTwo":[{"puuid":"self"},{"puuid":"a2"},{"puuid":"a3"},{"puuid":"a4"},{"puuid":"a5"}]}}`
		case "/lol-summoner/v1/current-summoner":
			selfCalls++
			body = `{"puuid":"self"}`
		default:
			t.Fatalf("unexpected game roster request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	s := &TencentLCU{client: client}
	for i := 0; i < 2; i++ {
		gameID, roster, err := s.Roster(context.Background())
		if err != nil || gameID != 42 || len(roster) != 10 || knownCount(roster) != 10 || roster[0].PUUID != "self" || roster[5].Team != "enemy" {
			t.Fatalf("active roster incorrect: %v", err)
		}
	}
	if selfCalls != 1 {
		t.Fatal("current identity was not cached for this connection")
	}
}
