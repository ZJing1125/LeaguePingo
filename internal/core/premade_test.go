package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPremadeGroupsAreExplicitAndScopedBySide(t *testing.T) {
	raw := []byte(`{"gameData":{"gameId":42,"teamOne":[{"puuid":"a","teamParticipantId":12,"selectedPosition":"MIDDLE"},{"puuid":"b","teamParticipantId":12},{"puuid":"c","teamParticipantId":0},{"puuid":"d"}],"teamTwo":[{"puuid":"e","teamParticipantId":12},{"puuid":"f","teamParticipantId":12},{"puuid":"g","teamParticipantId":15}]}}`)
	_, ps, err := ParseGameflowRosterForPlayer(raw, "a")
	if err != nil {
		t.Fatal(err)
	}
	if ps[0].PremadeSize != 2 || ps[0].Position != "mid" || ps[0].PremadeGroup != ps[1].PremadeGroup || ps[0].PremadeGroup == ps[4].PremadeGroup || ps[2].PremadeGroup != "" || ps[3].PremadeGroup != "" || ps[6].PremadeGroup != "" {
		t.Fatalf("wrong grouping %+v", ps)
	}
	if ps[6].GroupSource != "lcu" {
		t.Fatal("unique ID must remain source-observed, not solo inference")
	}
	encoded, _ := json.Marshal(ps)
	if strings.Contains(string(encoded), `"groupToken"`) || strings.Contains(string(encoded), `ally-12`) {
		t.Fatal("raw group IDs exposed")
	}
}

func TestExplicitAnonymousSlotsCanShowGroupsWithoutUnmasking(t *testing.T) {
	_, ps, err := ParseGameflowRosterForPlayer([]byte(`{"gameData":{"gameId":42,"teamOne":[{"puuid":"self","teamParticipantId":12},{"teamParticipantId":12}],"teamTwo":[{"teamParticipantId":15},{"teamParticipantId":15}]}}`), "self")
	if err != nil || len(ps) != 4 || ps[1].PremadeSize != 2 || ps[2].PremadeSize != 2 {
		t.Fatal("explicit anonymous group omitted", err)
	}
	if IsKnownPUUID(ps[1].PUUID) || ps[1].IdentityState == IdentityKnown {
		t.Fatal("grouping revealed an identity")
	}
	if ps[0].PremadeGroup == ps[2].PremadeGroup {
		t.Fatal("sides merged")
	}
}
