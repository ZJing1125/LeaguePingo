package engine

import (
	"testing"
	"time"

	"leaguepingo/internal/diagnostics"
)

// The page budget follows the recent SGP latency, bounded on both ends so a
// healthy link keeps four seconds and a very slow one never stalls a worker.
func TestScoutPageTimeoutFollowsRecentLatency(t *testing.T) {
	collector := &diagnostics.Collector{}
	if got := scoutPageTimeout(collector); got != 4*time.Second {
		t.Fatalf("empty window timeout = %s", got)
	}
	for i := 0; i < 20; i++ {
		collector.Record("/sgp-history", 3*time.Second, 200, false)
	}
	if got := scoutPageTimeout(collector); got != 6*time.Second {
		t.Fatalf("slow window timeout = %s, want 6s", got)
	}
	for i := 0; i < 100; i++ {
		collector.Record("/sgp-history", 10*time.Second, 200, false)
	}
	if got := scoutPageTimeout(collector); got != 8*time.Second {
		t.Fatalf("very slow window timeout = %s, want the 8s ceiling", got)
	}
	if got := scoutPageTimeout(nil); got != 4*time.Second {
		t.Fatalf("nil collector timeout = %s", got)
	}
}
