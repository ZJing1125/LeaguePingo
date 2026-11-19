package lcu

import (
	"fmt"
	"testing"
)

func TestLobbyEventDetectsModeWithoutRosterChange(t *testing.T) {
	event := func(queue int, other string) []byte {
		return []byte(fmt.Sprintf(`[8,"%s",{"uri":"/lol-lobby/v2/lobby","eventType":"Update","data":{"partyId":"same","gameConfig":{"queueId":%d},"warnings":[%q]}}]`, lobbyTopic, queue, other))
	}
	old, changed := lobbyEventSignature(event(420, "")), lobbyEventSignature(event(2400, ""))
	if old == "" || changed == "" || old == changed ||
		changed != lobbyEventSignature(event(2400, "unrelated")) {
		t.Fatal("lobby notices must track queue changes and ignore unrelated fields")
	}
	if lobbyEventSignature([]byte(`[8,"OnJsonApiEvent",{"uri":"/lol-lobby/v2/lobby","eventType":"Delete"}]`)) != "" {
		t.Fatal("broad events cannot wake lobby reads")
	}
}
