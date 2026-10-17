package source

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

// Explicit, bounded, read-only test. Does not select champions or request any
// history/profile. Only counts and match status may be logged.
func TestLiveChampSelectNotifications(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_SELECTION_TEST") != "1" {
		t.Skip("explicit selection verification only")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Skip("client unavailable; selection not verified")
	}
	defer c.HTTP.CloseIdleConnections()
	s := &TencentLCU{client: c}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	phase, err := s.Phase(ctx)
	if err != nil {
		t.Fatal("phase unavailable (redacted)")
	}
	if phase != "ChampSelect" {
		t.Skip("not in champion selection; event delivery not verified")
	}
	_, roster, err := s.ChampSelectRoster(ctx)
	if err != nil || len(roster) == 0 {
		t.Fatal("selection roster unavailable (redacted)")
	}
	watchCtx, stop := context.WithTimeout(ctx, 8*time.Second)
	defer stop()
	var events atomic.Int32
	_ = s.WatchChampSelect(watchCtx, func() { events.Add(1) })
	if events.Load() == 0 {
		t.Skip("selection roster readable, but no selection event observed; live swaps still unverified")
	}
	phase, err = s.Phase(ctx)
	if err != nil || phase != "ChampSelect" {
		t.Skip("selection ended; final roster comparison not performed")
	}
	_, final, err := s.ChampSelectRoster(ctx)
	if err != nil {
		t.Fatal("final selection read failed (redacted)")
	}
	t.Logf("selection roster read: initial=%d final=%d relevant notifications=%d; no history requested; desktop convergence still requires manual acceptance", len(roster), len(final), events.Load())
}
