package core

import "testing"

func TestSelectionDistinguishesUnselectedMissingAndAnonymousSlots(t *testing.T) {
	id, players, err := ParseChampSelectSession([]byte(`{"gameId":42,"myTeam":[{"puuid":"p1","championId":0},{"puuid":"p2","championId":23},{"puuid":"p3"}],"theirTeam":[{"cellId":8,"championId":24},{"cellId":6,"championId":null}]}`))
	if err != nil || id != 42 || len(players) != 5 {
		t.Fatal("invalid parsed selection")
	}
	for i, state := range []string{"unselected", "selected", "unavailable", "selected", "unavailable"} {
		if players[i].ChampionState != state {
			t.Fatalf("incorrect selection state at %d", i)
		}
	}
	if players[3].IdentityState != IdentityUnavailable || players[3].ChampionID != 24 || players[3].Key != "enemy-cell-8" {
		t.Fatal("anonymous champion lost or misidentified")
	}
	_, reordered, err := ParseChampSelectSession([]byte(`{"myTeam":[],"theirTeam":[{"cellId":6},{"cellId":8,"championId":25}]}`))
	if err != nil || reordered[1].Key != players[3].Key || reordered[1].ChampionID != 25 {
		t.Fatal("anonymous slot identity changed on reorder")
	}
	for _, raw := range []string{`null`, `{}`, `{"myTeam":null,"theirTeam":[]}`, `{"myTeam":[],"theirTeam":null}`} {
		if _, _, err := ParseChampSelectSession([]byte(raw)); err == nil {
			t.Fatal("missing session accepted")
		}
	}
}

func TestSelectionIntentDoesNotOverrideConfirmedHero(t *testing.T) {
	_, players, err := ParseChampSelectSession([]byte(`{"myTeam":[{"championId":0,"championPickIntent":23},{"championId":24,"championPickIntent":23},{"championPickIntent":0}],"theirTeam":[]}`))
	if err != nil || len(players) != 3 {
		t.Fatal("selection intent parse failed")
	}
	if players[0].ChampionID != 23 || players[0].ChampionState != "intent" || players[1].ChampionID != 24 || players[1].ChampionState != "selected" || players[2].ChampionState != "unavailable" {
		t.Fatal("intent mistaken for confirmed selection or missing data")
	}
}
