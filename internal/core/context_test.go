package core

import "testing"

func TestHistoryContextUsesExplicitFieldsWithoutInferringRemake(t *testing.T) {
	for _, test := range []struct {
		name, body string
		known      bool
	}{
		{"explicit", `{"games":{"games":[{"gameId":42,"mapId":11,"gameDuration":90,"participants":[{"puuid":"target","teamPosition":"UTILITY","timeline":{"lane":"BOTTOM"},"stats":{"gameEndedInEarlySurrender":true,"perk0":8005,"perk1":0,"perk2":-1,"perkPrimaryStyle":8000,"perkSubStyle":8100}}]}]}}`, true},
		{"unknown", `{"games":{"games":[{"gameId":42,"gameDuration":90,"participants":[{"puuid":"target","stats":{}}]}]}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, err := ParseHistory([]byte(test.body), "target")
			if err != nil {
				t.Fatal(err)
			}
			m := r.Matches[0]
			if test.known {
				if m.EarlyEnd == nil || !*m.EarlyEnd || m.MapID == nil || *m.MapID != 11 || m.Position != "UTILITY" || m.Lane != "BOTTOM" || len(m.RuneIDs) != 1 || m.RuneIDs[0] != 8005 || m.RunePrimary == nil || *m.RunePrimary != 8000 || m.RuneSecondary == nil || *m.RuneSecondary != 8100 {
					t.Fatal("explicit context not parsed")
				}
			} else if m.EarlyEnd != nil || m.MapID != nil || m.Position != "" || len(m.RuneIDs) != 0 || m.RunePrimary != nil || m.RuneSecondary != nil {
				t.Fatal("context invented from short duration")
			}
		})
	}
}

func TestExplicitFalseEarlyEndIsNotUnknown(t *testing.T) {
	r, err := ParseHistory([]byte(`{"games":{"games":[{"gameId":42,"participants":[{"puuid":"target","stats":{"gameEndedInEarlySurrender":false}}]}]}}`), "target")
	if err != nil || r.Matches[0].EarlyEnd == nil || *r.Matches[0].EarlyEnd {
		t.Fatal("explicit false flag lost", err)
	}
}

func TestHistoryPositionUsesVerifiedFieldsAndStrictLegacyLaneRoles(t *testing.T) {
	body := `{"games":{"games":[
		{"gameId":1,"queueId":420,"mapId":11,"participants":[{"puuid":"target","timeline":{"lane":"TOP","role":"SOLO"}}]},
		{"gameId":2,"queueId":420,"mapId":11,"participants":[{"puuid":"target","spell1Id":11,"timeline":{"lane":"JUNGLE","role":"NONE"}}]},
		{"gameId":3,"queueId":440,"mapId":11,"participants":[{"puuid":"target","timeline":{"lane":"MIDDLE","role":"SOLO"}}]},
		{"gameId":4,"queueId":440,"mapId":11,"participants":[{"puuid":"target","timeline":{"lane":"BOTTOM","role":"CARRY"}}]},
		{"gameId":5,"queueId":420,"mapId":11,"participants":[{"puuid":"target","timeline":{"lane":"BOTTOM","role":"SUPPORT"}}]},
		{"gameId":6,"queueId":420,"mapId":11,"participants":[{"puuid":"target","individualPosition":"UTILITY","timeline":{"lane":"TOP","role":"SOLO"}}]},
		{"gameId":7,"queueId":420,"mapId":11,"participants":[{"puuid":"target","timeline":{"lane":"JUNGLE","role":"NONE"}}]},
		{"gameId":8,"queueId":450,"mapId":12,"participants":[{"puuid":"target","spell1Id":11,"timeline":{"lane":"JUNGLE","role":"NONE"}}]}
	]}}`
	result, err := ParseHistory([]byte(body), "target")
	if err != nil || len(result.Matches) != 8 {
		t.Fatal("legacy position fixture failed", err, len(result.Matches))
	}
	byID := map[int64]string{}
	for _, match := range result.Matches {
		byID[match.GameID] = match.Position
	}
	for id, want := range map[int64]string{1: "TOP", 2: "JUNGLE", 3: "MIDDLE", 4: "BOTTOM", 5: "UTILITY", 6: "UTILITY", 7: "", 8: ""} {
		if byID[id] != want {
			t.Fatalf("game %d position=%q want=%q", id, byID[id], want)
		}
	}
}
