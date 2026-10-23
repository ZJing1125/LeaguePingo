package core

import "testing"

func TestActivePickHoverOverridesStaleIntentWithoutUsingBans(t *testing.T) {
	raw := []byte(`{"myTeam":[{"puuid":"synthetic","cellId":0,"championId":0,"championPickIntent":23}],"theirTeam":[],"actions":[[{"actorCellId":0,"championId":910,"type":"pick","isInProgress":true,"completed":false},{"actorCellId":0,"championId":99,"type":"ban","isInProgress":true}]]}`)
	_, players, err := ParseChampSelectSession(raw)
	if err != nil || len(players) != 1 || players[0].ChampionID != 910 || players[0].ChampionState != "intent" {
		t.Fatal("unlocked hover was not published", err)
	}
	raw = []byte(`{"myTeam":[{"puuid":"synthetic","cellId":0,"championId":23,"championPickIntent":99}],"theirTeam":[],"actions":[[{"actorCellId":0,"championId":910,"type":"pick","isInProgress":true}]]}`)
	_, players, err = ParseChampSelectSession(raw)
	if err != nil || players[0].ChampionID != 23 || players[0].ChampionState != "selected" {
		t.Fatal("confirmed champion was overwritten")
	}
}
