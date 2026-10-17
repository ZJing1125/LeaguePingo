package core

import "testing"

func TestParseHistoryKeepsOnlyRequestedParticipant(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":8,"gameCreation":1730000000000,"queueId":420,"participants":[{"puuid":"other","stats":{"kills":99}},{"puuid":"target","stats":{"win":true,"championId":23,"kills":4,"deaths":2,"assists":6}}]}]}}`)
	got, err := ParseHistory(data, "target")
	if err != nil || len(got.Matches) != 1 || got.Matches[0].Kills == nil || *got.Matches[0].Kills != 4 || got.Matches[0].ChampionID != 23 || got.Matches[0].Win == nil || !*got.Matches[0].Win {
		t.Fatalf("matches=%+v err=%v", got, err)
	}
}

func TestParseHistoryUsesParticipantIdentities(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":9,"gameDate":1730000000000,"participants":[{"participantId":1,"championId":11,"stats":{"win":true,"kills":6,"deaths":3,"assists":8}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target"}}]}]}}`)
	got, err := ParseHistory(data, "target")
	if err != nil || len(got.Matches) != 1 || got.Matches[0].ChampionID != 11 || got.Matches[0].Kills == nil || *got.Matches[0].Kills != 6 {
		t.Fatalf("matches=%+v err=%v", got, err)
	}
	if _, err := ParseHistory(data, "someone-else"); err == nil {
		t.Fatal("unverified match history was treated as empty")
	}
}

func TestHistoryMissingFieldsAndPartialSample(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":1,"participants":[{"puuid":"target","stats":{}}]},{"gameId":2,"participants":[{"puuid":"other"}]},"invalid",{"gameId":1,"participants":[{"puuid":"target"}]}]}}`)
	got, err := ParseHistory(data, "target")
	if err != nil || got.Meta.Returned != 4 || got.Meta.Parsed != 1 || got.Meta.Excluded != 3 {
		t.Fatalf("missing partial sample metadata: %+v %v", got.Meta, err)
	}
	m := got.Matches[0]
	if m.Win != nil || m.Kills != nil || m.Deaths != nil || m.Assists != nil || m.PlayedAt != nil {
		t.Fatal("missing data was replaced by real values")
	}
	s := CalculateStats(got.Matches)
	if s.WinRate != nil || s.KDA != nil || s.Losses != 0 {
		t.Fatal("unknown fields polluted statistics")
	}
}

func TestHistoryRejectsConflictingIdentity(t *testing.T) {
	data := []byte(`{"games":{"games":[{"gameId":1,"participants":[{"participantId":1,"puuid":"other"}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target"}}]}]}}`)
	if _, err := ParseHistory(data, "target"); err == nil {
		t.Fatal("conflicting identity accepted")
	}
}

func TestHistoryRichFieldsPreserveUnknownAndZero(t *testing.T) {
	got, err := ParseHistory([]byte(`{"games":{"games":[{"gameId":50,"gameDuration":1800,"participants":[{"puuid":"target","spell1Id":4,"spell2Id":14,"stats":{"goldEarned":0,"totalDamageDealtToChampions":-1,"totalMinionsKilled":20,"neutralMinionsKilled":5,"item0":0,"item1":3006}}]}]}}`), "target")
	if err != nil || len(got.Matches) != 1 {
		t.Fatal(err)
	}
	m := got.Matches[0]
	if m.Duration == nil || *m.Duration != 1800 || m.Gold == nil || *m.Gold != 0 || m.Damage != nil || m.CS == nil || *m.CS != 25 || len(m.Items) != 7 || m.Items[0] == nil || *m.Items[0] != 0 || m.Items[2] != nil || m.Spell1 == nil || *m.Spell1 != 4 {
		t.Fatal("rich fields conflated unknown and zero")
	}
}
