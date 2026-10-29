package source

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/apperrors"
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
		case request.URL.Path == "/lol-rso-auth/v1/authorization":
			body = `{"currentPlatformId":"NJ100"}`
		case strings.Contains(request.URL.Path, "ServiceEndpoint"):
			// An unusable discovery result exercises the bounded LCU fallback.
			body = `"https://invalid.example"`
		case strings.Contains(request.URL.Path, "gameflow/v1/session"):
			body = `{"gameData":{"gameId":42,"teamOne":[{"puuid":"p1"}],"teamTwo":[]}}`
		case strings.Contains(request.URL.Path, "champ-select/v1/session"):
			body = `{"myTeam":[{"puuid":"p1"}],"theirTeam":[{"puuid":""}]}`
		case request.URL.Path == "/lol-summoner/v1/current-summoner":
			body = `{"puuid":"p1"}`
		case strings.Contains(request.URL.Path, "summoners/puuid"):
			body = `{"puuid":"p1","gameName":"Player","tagLine":"CN1"}`
		case strings.Contains(request.URL.Path, "match-history"):
			body = `{"games":{"games":[{"gameId":10,"participants":[{"participantId":1,"championId":12,"stats":{"win":true,"kills":8}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"p1"}}]}]}}`
		case strings.Contains(request.URL.Path, "summoners"):
			body = `[{"puuid":"p1","gameName":"Player","tagLine":"CN1"}]`
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
	if err != nil || len(matches.Matches) != 1 || matches.Matches[0].ChampionID != 12 {
		t.Fatalf("matches=%+v err=%v", matches, err)
	}
	player, err := s.FindByRiotID(ctx, "Player#CN1")
	if err != nil || player.PUUID != "p1" {
		t.Fatalf("player=%+v err=%v", player, err)
	}
	if _, err := s.FindByRiotID(ctx, "Player"); err == nil {
		t.Fatal("partial Riot ID accepted")
	}
	if len(paths) != 9 {
		t.Fatalf("unexpected request count: %v", paths)
	}
}

func TestLookupUsesStructuredAliasWithoutQueryParameters(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var aliases []map[string]string
		if r.Method != http.MethodPost || r.URL.Path != "/lol-summoner/v1/summoners/aliases" || r.URL.RawQuery != "" || r.URL.Fragment != "" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("lookup does not match the client's structured alias contract")
		}
		if json.NewDecoder(r.Body).Decode(&aliases) != nil || len(aliases) != 1 || len(aliases[0]) != 2 || aliases[0]["gameName"] != "测试 + 玩家" || aliases[0]["tagLine"] != "CN1" {
			t.Fatal("lookup body must contain exactly one complete alias")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"puuid":"test-account","gameName":"测试 + 玩家","tagLine":"CN1"}]`))}, nil
	})
	player, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), " 测试 + 玩家 # CN1 ")
	if err != nil || player.PUUID != "test-account" {
		t.Fatalf("complete ID lookup failed: %v", err)
	}
}

func TestLookupCleansCopiedDirectionMarkers(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var aliases []map[string]string
		if json.NewDecoder(r.Body).Decode(&aliases) != nil || len(aliases) != 1 || aliases[0]["gameName"] != "测试 + 玩家" || aliases[0]["tagLine"] != "TEST" {
			t.Fatal("copied formatting characters were sent to the identity endpoint")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"puuid":"test-account","gameName":"测试 + 玩家","tagLine":"TEST"}]`))}, nil
	})
	for _, input := range []string{"\u2066测试 + 玩家\u2069#\u2066TEST\u2069", "\u200e \u2067测试 + 玩家\u2069 # \u2068TEST\u2069\u200f", "\ufeff测试 + 玩家\u200b#\u200bTEST\ufeff"} {
		p, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), input)
		if err != nil || p.DisplayName != "测试 + 玩家#TEST" {
			t.Fatalf("copied ID lookup failed: %v", err)
		}
	}
}

func TestLookupTimeoutDoesNotBlamePlayerID(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345})
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	_, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), "PrivatePlayer#TEST")
	if err == nil || apperrors.CodeOf(err) != apperrors.RequestTimeout || !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "请确认完整") || strings.Contains(err.Error(), "PrivatePlayer") {
		t.Fatal("timeout should be explained separately and without the query URL")
	}
}

func TestLookupRejectsWrongIdentityAndExplainsFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		code   apperrors.Code
	}{
		{"wrong tag", 200, `[{"puuid":"p1","gameName":"Player","tagLine":"OTHER"}]`, apperrors.PlayerIdentityUnavailable},
		{"wrong name", 200, `[{"puuid":"p1","gameName":"Other","tagLine":"CN1"}]`, apperrors.PlayerIdentityUnavailable},
		{"missing identity", 200, `[{"gameName":"Player","tagLine":"CN1"}]`, apperrors.PlayerIdentityUnavailable},
		{"empty result", 200, `[]`, apperrors.PlayerNotFound},
		{"null result", 200, `null`, apperrors.InvalidResponse},
		{"object instead of array", 200, `{}`, apperrors.InvalidResponse},
		{"null profile", 200, `[null]`, apperrors.PlayerIdentityUnavailable},
		{"ambiguous result", 200, `[{"puuid":"p1","gameName":"Player","tagLine":"CN1"},{"puuid":"p2","gameName":"Player","tagLine":"CN1"}]`, apperrors.PlayerIdentityUnavailable},
		{"bad JSON", 200, `invalid`, apperrors.InvalidResponse},
		{"bad request", 400, `sensitive response must not be logged`, apperrors.DataUnavailable},
		{"not found", 404, `not found`, apperrors.DataUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
			calls := 0
			client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			_, err := (&TencentLCU{client: client}).FindByRiotID(context.Background(), "Player#CN1")
			if err == nil || apperrors.CodeOf(err) != test.code || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("missing or unsafe identity error: %v", err)
			}
			if calls != 1 {
				t.Fatal("failed lookup must not retry or fall back")
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
	for _, id := range []string{"", "Player", "#TAG", "Player#", "Player#  ", "\u2066\u2069#TAG", "Player#\u2066\u2069", "Player#TAG#extra", "Play\x00er#TAG"} {
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

func TestGameflowKnownIdentitiesSurviveLongerSelectionFallback(t *testing.T) {
	client := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/lol-gameflow/v1/session":
			body = `{"gameData":{"gameId":42,"teamOne":[{"puuid":"e1"},{"puuid":"e2"},{"puuid":"e3"},{"puuid":"e4"},{"puuid":"e5"}],"teamTwo":[{"puuid":"self"},{"puuid":"a2"},{"puuid":"a3"},{"puuid":"a4"}]}}`
		case "/lol-summoner/v1/current-summoner":
			body = `{"puuid":"self"}`
		case "/lol-champ-select/v1/session":
			body = `{"myTeam":[{"puuid":"self"},{"puuid":"a2"},{"puuid":"a3"},{"puuid":"a4"},{"puuid":"a5"}],"theirTeam":[{},{},{},{},{}]}`
		default:
			t.Fatalf("unexpected player-data request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	gameID, roster, err := (&TencentLCU{client: client}).Roster(context.Background())
	if err != nil || gameID != 42 || len(roster) != 9 || knownCount(roster) != 9 {
		t.Fatalf("longer anonymous selection replaced known gameflow identities: count=%d known=%d err=%v", len(roster), knownCount(roster), err)
	}
	if roster[0].PUUID != "self" || roster[4].Team != "enemy" {
		t.Fatal("gameflow team orientation lost")
	}
}
