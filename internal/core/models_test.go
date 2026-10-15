package core

import "testing"

func TestRosterKeepsUnavailableSlotsWithoutDuplicatingKnownPlayers(t *testing.T) {
	data := []byte(`{"gameData":{"gameId":42,"teamOne":[{"puuid":"alpha","championId":1},{"puuid":""}],"teamTwo":[{"puuid":"bravo"},{"puuid":"alpha"}]}}`)
	gameID, players, err := ParseGameflowRoster(data)
	if err != nil || gameID != 42 || len(players) != 3 {
		t.Fatalf("gameID=%d players=%v err=%v", gameID, players, err)
	}
	if players[1].IdentityState != IdentityUnavailable || players[1].PUUID != "" {
		t.Fatalf("unavailable slot was treated as a queryable player: %+v", players[1])
	}
	if players[2].Team != "enemy" {
		t.Fatalf("enemy assignment lost: %+v", players[2])
	}
}

func TestStatsUseOnlyReturnedMatches(t *testing.T) {
	got := CalculateStats([]Match{{Win: true, Kills: 5, Deaths: 1, Assists: 5}, {Win: false, Kills: 2, Deaths: 3, Assists: 4}})
	if got.Games != 2 || got.Wins != 1 || got.WinRate != 50 || got.KDA != 4 {
		t.Fatalf("unexpected stats: %+v", got)
	}
}
