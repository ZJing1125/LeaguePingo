package core

import "testing"

func TestLobbyRosterOnlyIncludesMembersAndKeepsUnavailableIdentity(t *testing.T) {
	players, err := ParseLobbyRoster([]byte(`{"members":[{"puuid":"self","summonerName":"Fixture"},{"puuid":"self"},{"puuid":"","summonerName":"masked"},{"puuid":"bot","isBot":true},{"puuid":"spectator","isSpectator":true}]}`))
	if err != nil || len(players) != 2 || players[0].Team != "ally" || players[1].IdentityState != IdentityUnavailable {
		t.Fatal("incorrect lobby members")
	}
	if _, err := ParseLobbyRoster([]byte(`{}`)); err == nil {
		t.Fatal("missing members accepted")
	}
}
