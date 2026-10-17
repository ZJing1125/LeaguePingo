package core

import (
	"strings"
	"testing"
)

const sgpFixture = `{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_42"},"json":{"gameId":42,"platformId":"NJ100","queueId":440,"gameCreation":1792188000000,"gameDuration":1800,"participants":[{"puuid":"other","kills":99},{"participantId":2,"puuid":"target","championId":23,"kills":0,"deaths":2,"assists":8,"win":false,"goldEarned":0,"summoner1Id":4,"summoner2Id":14,"teamPosition":"TOP","perks":{"styles":[{"style":8000,"selections":[{"perk":8005}]},{"style":8100,"selections":[{"perk":8139}]}]}}]}}]}`

func TestSGPHistoryVerifiesTargetAndPreservesFields(t *testing.T) {
	r, err := ParseSGPHistory([]byte(sgpFixture), "target", "NJ100")
	if err != nil || len(r.Matches) != 1 {
		t.Fatalf("summary parse failed: %v", err)
	}
	m := r.Matches[0]
	if m.QueueID != 440 || m.Kills == nil || *m.Kills != 0 || m.Deaths == nil || *m.Deaths != 2 || m.Win == nil || *m.Win || m.Damage != nil || m.Gold == nil || *m.Gold != 0 || m.Spell1 == nil || *m.Spell1 != 4 || m.Spell2 == nil || *m.Spell2 != 14 || m.Position != "TOP" || len(m.RuneIDs) != 2 || *m.RunePrimary != 8000 {
		t.Fatal("summary fields corrupted or unknowns replaced")
	}
	for _, body := range []string{
		strings.Replace(sgpFixture, `"platformId":"NJ100"`, `"platformId":"HN1"`, 1),
		strings.Replace(sgpFixture, `"match_id":"NJ100_42"`, `"match_id":"NJ100_43"`, 1),
		strings.ReplaceAll(sgpFixture, `"puuid":"target"`, `"puuid":"unrelated"`),
		`{"games":null}`, `{}`, `{"games":["invalid"]}`,
		strings.Replace(sgpFixture, `{"puuid":"other","kills":99}`, `null`, 1),
	} {
		if _, err := ParseSGPHistory([]byte(body), "target", "NJ100"); err == nil {
			t.Fatal("unverified summary accepted")
		}
	}
}
