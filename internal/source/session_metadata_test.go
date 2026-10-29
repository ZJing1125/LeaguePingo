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
