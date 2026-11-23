package core

import "testing"

func TestShardsPreserveMissingMiddleAndRepeatedID(t *testing.T) {
	a, b := 5008, 5001
	got := shardSlots(&a, nil, &b)
	if len(got) != 3 || got[0] != 5008 || got[1] != 0 || got[2] != 5001 {
		t.Fatal("missing shard shifted the other slots")
	}
	got = shardSlots(&a, &a, &b)
	if got[0] != got[1] {
		t.Fatal("repeated shard was removed")
	}
}

func TestHistoryPreservesActualAugmentsAndVersion(t *testing.T) {
	raw := []byte(`{"games":{"games":[{"gameId":71,"queueId":2400,"mapId":12,"gameVersion":"16.23.1","participants":[{"participantId":1,"puuid":"target","teamId":100,"championId":23,"stats":{"playerAugment1":1234,"playerAugment3":5678,"statPerk0":5007,"statPerk1":5011,"statPerk2":5001}}]}]}}`)
	result, err := ParseHistory(raw, "target")
	if err != nil || len(result.Matches) != 1 {
		t.Fatalf("history failed: %v", err)
	}
	match := result.Matches[0]
	if match.GameVersion != "16.23.1" || len(match.AugmentIDs) != 6 || match.AugmentIDs[0] != 1234 || match.AugmentIDs[1] != 0 || match.AugmentIDs[2] != 5678 {
		t.Fatalf("actual selection slots changed: %+v", match)
	}
	if len(match.RuneShards) != 3 || match.RuneShards[1] != 5011 || match.RuneShards[2] != 5001 {
		t.Fatal("flat and scaling health collapsed")
	}
}
