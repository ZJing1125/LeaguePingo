package source

import (
	"context"
	"os"
	"testing"

	"leaguepingo/internal/lcu"
)

// Explicit live entry exercises the same verifier shipped in Settings.
func TestLiveQueueHistoryAudit(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_QUEUE_AUDIT") != "1" {
		t.Skip("explicit Windows self-account queue audit")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal("local client unavailable")
	}
	s := &TencentLCU{client: c}
	defer c.HTTP.CloseIdleConnections()
	defer s.ResetIdentity()
	result, err := s.VerifyHistoryQueue(context.Background(), 2400)
	if err != nil {
		t.Fatal("queue audit failed; no capability enabled")
	}
	t.Logf("QUEUE_AUDIT queue=%d state=%s mixed=%d filtered=%d compared=%d current-session-only", result.QueueID, result.State, result.Mixed, result.Filtered, result.Compared)
}
