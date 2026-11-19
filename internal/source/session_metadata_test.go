package source

import "testing"

func TestSessionMetadataActualLayers(t *testing.T) {
	for _, raw := range []string{
		`{"gameData":{"queue":{"id":2400,"gameMode":"ARAM","mapId":12}}}`,
		`{"gameData":{"queue":{"id":2400}},"map":{"id":12,"gameMode":"ARAM"}}`,
	} {
		s := &TencentLCU{}
		s.rememberMetadata([]byte(raw), false)
		m := s.SessionMetadata()
		if m.QueueID != 2400 || m.MapID != 12 || m.GameMode != "ARAM" {
			t.Fatalf("metadata lost %+v", m)
		}
	}
	s := &TencentLCU{}
	s.rememberMetadata([]byte(`{"gameData":{"queue":{"id":2400,"gameMode":"ARAM","mapId":12}},"map":{"id":11,"gameMode":"CLASSIC"}}`), false)
	if s.SessionMetadata().GameMode != "CONFLICT" {
		t.Fatal("conflicting mode trusted")
	}
	s.rememberMetadata([]byte(`{"gameConfig":{"queueId":450,"mapId":12,"gameMode":"ARAM"}}`), true)
	if s.SessionMetadata().QueueID != 450 {
		t.Fatal("mode remained stale after new lobby")
	}
}

func TestLobbyRandomMapUsesExactQueue(t *testing.T) {
	for _, input := range []struct {
		body string
		want int
	}{
		{`{"gameConfig":{"queueId":2400,"mapId":-1,"gameMode":"ARAM"}}`, 2400},
		{`{"gameConfig":{"queueId":450,"mapId":-1,"gameMode":"ARAM"}}`, 450},
		{`{"gameConfig":{"queueId":2400,"mapId":-1,"gameMode":"ARAM","isCustom":true}}`, 0},
		{`{"gameConfig":{"mapId":12,"gameMode":"ARAM"}}`, 0},
	} {
		meta, ok := parseSessionMetadata([]byte(input.body), true)
		if !ok || meta.QueueID != input.want || meta.MapID != 0 {
			t.Fatalf("expected queue %d with unselected map, got %+v", input.want, meta)
		}
	}
}
