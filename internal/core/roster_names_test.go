package core

import "testing"

func TestRosterReadsProvidedRiotIDsWithoutInventingUnknownNames(t *testing.T) {
	for _, test := range []struct {
		name, body string
		lobby      bool
	}{
		{"gameflow", `{"gameData":{"gameId":42,"teamOne":[{"puuid":"self","gameName":"Current","tagLine":"TEST"}],"teamTwo":[{"puuid":"enemy","riotIdGameName":"Opponent","riotIdTagLine":"TEST"},{"gameName":"Hidden","tagLine":"TEST"}]}}`, false},
		{"lobby", `{"members":[{"puuid":"self","gameName":"Current","tagLine":"TEST"},{"gameName":"Hidden","tagLine":"TEST"}]}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var players []Participant
			var err error
			if test.lobby {
				players, err = ParseLobbyRoster([]byte(test.body))
			} else {
				_, players, err = ParseGameflowRosterForPlayer([]byte(test.body), "self")
			}
			if err != nil || players[0].DisplayName != "Current#TEST" {
				t.Fatal("provided Riot ID was discarded", err)
			}
			if !test.lobby && players[1].DisplayName != "Opponent#TEST" {
				t.Fatal("provided opponent Riot ID was discarded")
			}
			if players[len(players)-1].DisplayName != "身份暂不可用" {
				t.Fatal("name without verified identity became queryable")
			}
		})
	}
}
