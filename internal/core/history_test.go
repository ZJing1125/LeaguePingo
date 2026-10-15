package core

import "testing"

func TestParseHistoryKeepsOnlyRequestedParticipant(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":8,"gameCreation":1730000000000,"queueId":420,"participants":[{"puuid":"other","stats":{"kills":99}},{"puuid":"target","stats":{"win":true,"championId":23,"kills":4,"deaths":2,"assists":6}}]}]}}`)
	got, err := ParseHistory(data, "target")
	if err != nil || len(got) != 1 || got[0].Kills != 4 || got[0].ChampionID != 23 || !got[0].Win {
		t.Fatalf("matches=%+v err=%v", got, err)
	}
}

func TestParseHistoryUsesParticipantIdentities(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":9,"gameDate":1730000000000,"participants":[{"participantId":1,"championId":11,"stats":{"win":true,"kills":6,"deaths":3,"assists":8}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target"}}]}]}}`)
	got, err := ParseHistory(data, "target")
	if err != nil || len(got) != 1 || got[0].ChampionID != 11 || got[0].Kills != 6 {
		t.Fatalf("matches=%+v err=%v", got, err)
	}
	if _, err := ParseHistory(data, "someone-else"); err == nil {
		t.Fatal("unverified match history was treated as empty")
	}
}
