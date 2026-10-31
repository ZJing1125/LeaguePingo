package diagnostics

import (
	"testing"
	"time"
)

// The adaptive page budget asks for one category's percentile on every request;
// it must read only the bounded recent window and never report unknown data.
func TestPercentileReadsRecentWindowPerCategory(t *testing.T) {
	c := &Collector{}
	for i := 1; i <= 100; i++ {
		c.Record("/sgp-history", time.Duration(i)*time.Millisecond, 200, false)
	}
	if p95, ok := c.Percentile("sgp-history", 95); !ok || p95 != 95 {
		t.Fatalf("p95 = %d, %v; want 95", p95, ok)
	}
	if p50, ok := c.Percentile("sgp-history", 50); !ok || p50 != 50 {
		t.Fatalf("p50 = %d, %v; want 50", p50, ok)
	}
	if _, ok := c.Percentile("sgp-rank", 95); ok {
		t.Fatal("unknown category reported samples")
	}
	if _, ok := c.Percentile("sgp-history", 0); ok {
		t.Fatal("invalid percentile reported samples")
	}
	c.Record("/lol-gameflow/v1/gameflow-phase", time.Second, 200, false)
	if p95, ok := c.Percentile("sgp-history", 95); !ok || p95 != 95 {
		t.Fatalf("unrelated category changed the window: %d %v", p95, ok)
	}
}
