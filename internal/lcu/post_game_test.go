package lcu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validLobby = `{"partyId":"synthetic-party","canStartActivity":true,"gameConfig":{"queueId":420,"isCustom":false},"localMember":{"puuid":"synthetic-self","isLeader":true},"members":[{"puuid":"synthetic-self","isLeader":true}],"invitations":[]}`

func TestLobbyControlRequiresExplicitEligibility(t *testing.T) {
	good, err := parseLobbyControl([]byte(validLobby))
	if err != nil || !good.Leader || !good.Ready || good.QueueID != 420 || strings.Contains(good.PartyKey, "synthetic") || strings.Contains(good.MembersKey, "synthetic") {
		t.Fatalf("invalid minimal lobby: %v", err)
	}
	for _, raw := range []string{
		strings.Replace(validLobby, `"isCustom":false`, `"isCustom":true`, 1),
		strings.Replace(validLobby, `"isLeader":true`, `"unknown":true`, 1),
		strings.Replace(validLobby, `"canStartActivity":true`, `"other":true`, 1),
		strings.Replace(validLobby, `"partyId":"synthetic-party"`, `"partyId":""`, 1),
		strings.Replace(validLobby, `"members":[{"puuid":"synthetic-self","isLeader":true}]`, `"members":[]`, 1),
	} {
		if _, err := parseLobbyControl([]byte(raw)); err == nil {
			t.Fatal("incomplete or custom lobby allowed")
		}
	}
	pending, err := parseLobbyControl([]byte(strings.Replace(validLobby, `"invitations":[]`, `"invitations":[{"state":"Pending"}]`, 1)))
	if err != nil || pending.Ready {
		t.Fatal("pending invitation did not block search")
	}
}
func TestPostGameWritesUseOnlyFixedEndpointsAndAccept204(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method+" "+r.URL.Path]++
		switch {
		case r.Method == "POST" && (r.URL.Path == playAgainPath || r.URL.Path == startSearchPath):
			w.WriteHeader(204)
		case r.Method == "GET" && r.URL.Path == "/lol-lobby/v2/lobby":
			fmt.Fprint(w, validLobby)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := testEventClient(t, server.URL)
	if err := client.PlayAgain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.StartSearch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.LobbyControl(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls["POST "+playAgainPath] != 1 || calls["POST "+startSearchPath] != 1 {
		t.Fatal("unexpected mutation endpoint")
	}
}
