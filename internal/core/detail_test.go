package core

import (
	"encoding/json"
	"testing"
)

func detailFixture(t *testing.T, missing bool) []byte {
	t.Helper()
	players, identities := []any{}, []any{}
	for i := 1; i <= 10; i++ {
		team := 100
		if i > 5 {
			team = 200
		}
		stats := map[string]any{"kills": 1, "deaths": 0, "assists": 1, "goldEarned": 200, "totalDamageDealtToChampions": 100}
		if missing && i == 5 {
			delete(stats, "totalDamageDealtToChampions")
		}
		players = append(players, map[string]any{"participantId": i, "teamId": team, "championId": 23, "stats": stats})
		id := "other"
		if i == 1 {
			id = "target"
		}
		identities = append(identities, map[string]any{"participantId": i, "player": map[string]any{"puuid": id, "gameName": "Fixture", "tagLine": "TEST"}})
	}
	raw, err := json.Marshal(map[string]any{"gameId": 42, "participants": players, "participantIdentities": identities})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMatchDetailSharesUseCompleteTeamTotals(t *testing.T) {
	d, err := ParseMatchDetail(detailFixture(t, false), 42, "target")
	if err != nil || d.Partial || len(d.Players) != 10 {
		t.Fatalf("detail rejected: %v", err)
	}
	p := d.Players[0]
	if !p.Target || p.DamageShare == nil || *p.DamageShare != 20 || p.GoldShare == nil || *p.GoldShare != 20 || p.Participation == nil || *p.Participation != 40 {
		t.Fatal("incorrect team denominators")
	}
	d, err = ParseMatchDetail(detailFixture(t, true), 42, "target")
	if err != nil || d.Players[0].DamageShare != nil || d.Players[0].GoldShare == nil || d.Players[5].DamageShare == nil {
		t.Fatal("missing field affected wrong team or invented denominator")
	}
}

func TestMatchDetailRejectsWrongGameIdentityAndIncompleteTotals(t *testing.T) {
	if _, err := ParseMatchDetail(detailFixture(t, false), 43, "target"); err == nil {
		t.Fatal("wrong game accepted")
	}
	if _, err := ParseMatchDetail(detailFixture(t, false), 42, "missing"); err == nil {
		t.Fatal("unverified identity accepted")
	}
	for _, raw := range []string{
		`{"gameId":42,"participants":[{"participantId":1,"puuid":"target","teamId":100,"stats":{"kills":2,"assists":1}}]}`,
		`{"gameId":42,"participants":[{"participantId":1,"puuid":"target"},{"participantId":1,"puuid":"other"}]}`,
	} {
		d, err := ParseMatchDetail([]byte(raw), 42, "target")
		if err == nil && (!d.Partial || d.Players[0].Participation != nil) {
			t.Fatal("partial roster produced a share")
		}
	}
}

func TestMatchDetailKeepsKnownRiotIDAlongsideLegacyName(t *testing.T) {
	raw := []byte(`{"gameId":42,"participants":[{"participantId":1,"puuid":"target","teamId":100,"riotIdGameName":"Fixture","riotIdTagline":"TEST"}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target","summonerName":"Legacy"}}]}`)
	detail, err := ParseMatchDetail(raw, 42, "target")
	if err != nil {
		t.Fatal(err)
	}
	if got := detail.Players[0].DisplayName; got != "Fixture#TEST" {
		t.Fatalf("verified full ID was replaced by legacy name: %q", got)
	}
}
