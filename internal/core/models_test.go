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
	got := CalculateStats([]Match{{Win: ptr(true), Kills: ptr(5), Deaths: ptr(1), Assists: ptr(5)}, {Win: ptr(false), Kills: ptr(2), Deaths: ptr(3), Assists: ptr(4)}})
	if got.Games != 2 || got.Wins != 1 || got.WinRate == nil || *got.WinRate != 50 || got.KDA == nil || *got.KDA != 4 {
		t.Fatalf("unexpected stats: %+v", got)
	}
}

func ptr[T any](v T) *T { return &v }

func TestStatsDistinguishEmptyUnknownAndZero(t *testing.T) {
	if got := CalculateStats(nil); got.WinRate != nil || got.KDA != nil {
		t.Fatal("empty sample has numeric statistics")
	}
	got := CalculateStats([]Match{{Win: ptr(false), Kills: ptr(0), Deaths: ptr(0), Assists: ptr(0)}, {}})
	if got.Games != 2 || got.OutcomeGames != 1 || got.KDAGames != 1 || got.WinRate == nil || *got.WinRate != 0 || got.KDA == nil || *got.KDA != 0 {
		t.Fatalf("valid zero lost or unknown included: %+v", got)
	}
	got = CalculateStats([]Match{{Kills: ptr(10), Deaths: ptr(1), Assists: ptr(0)}, {Kills: ptr(0), Deaths: ptr(9), Assists: ptr(0)}})
	if *got.KDA != 1 {
		t.Fatal("KDA must use aggregate totals")
	}
}

func TestShortExplicitEarlyEndIsRemakeAndNotAStatisticalLoss(t *testing.T) {
	remake := Match{EarlyEnd: ptr(true), Duration: ptr(75), Win: ptr(false), Kills: ptr(0), Deaths: ptr(0), Assists: ptr(0)}
	if !IsRemake(remake) {
		t.Fatal("short explicit early-end record was not classified as a remake")
	}
	got := CalculateStats([]Match{remake, {Win: ptr(true), Kills: ptr(4), Deaths: ptr(2), Assists: ptr(6)}})
	if got.Games != 2 || got.OutcomeGames != 1 || got.Wins != 1 || got.Losses != 0 || got.KDAGames != 1 || got.WinRate == nil || *got.WinRate != 100 || got.KDA == nil || *got.KDA != 5 {
		t.Fatalf("remake polluted aggregate statistics: %+v", got)
	}
	longEarlyLoss := Match{EarlyEnd: ptr(true), Duration: ptr(16 * 60), Win: ptr(false)}
	if IsRemake(longEarlyLoss) {
		t.Fatal("a normal early surrender was relabeled as a remake")
	}
}

func TestGameflowUsesCurrentPlayersTeamAndProvidedNames(t *testing.T) {
	data := []byte(`{"gameData":{"gameId":42,"teamOne":[{"puuid":"opponent","summonerName":"Opponent#TEST"}],"teamTwo":[{"puuid":"self","summonerName":"Me#TEST"}]}}`)
	_, players, err := ParseGameflowRosterForPlayer(data, "self")
	if err != nil || len(players) != 2 || players[0].PUUID != "self" || players[0].Team != "ally" || players[1].Team != "enemy" || players[1].DisplayName != "Opponent#TEST" {
		t.Fatalf("wrong local team: %+v %v", players, err)
	}
	if _, _, err := ParseGameflowRosterForPlayer(data, "not-in-match"); err == nil {
		t.Fatal("unknown local team was guessed")
	}
}
