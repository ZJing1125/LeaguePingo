package actionlane

import (
	"context"
	"testing"
	"time"
)

func TestStopAndReplacementRejectStaleSessions(t *testing.T) {
	var lane Executor
	ctx, first, ok := lane.Start(context.Background(), 0)
	if !ok || !lane.Current(first) {
		t.Fatal("first session did not start")
	}
	_, second, ok := lane.Start(context.Background(), 0)
	if !ok || lane.Current(first) || !lane.Current(second) {
		t.Fatal("replacement did not reject the stale session")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("replaced session was not cancelled")
	}
	lane.Stop()
	if lane.Current(second) {
		t.Fatal("stopped session remained current")
	}
}

func TestBlockPersistsUntilReset(t *testing.T) {
	var lane Executor
	ctx, session, ok := lane.Start(context.Background(), 0)
	if !ok {
		t.Fatal("session did not start")
	}
	lane.Block()
	if !lane.Blocked() || lane.Current(session) {
		t.Fatal("block did not invalidate the session")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("blocked session was not cancelled")
	}
	if _, _, ok := lane.Start(context.Background(), 0); ok {
		t.Fatal("blocked lane started a session")
	}
	lane.Reset()
	if _, next, ok := lane.Start(context.Background(), 0); !ok || !lane.Current(next) {
		t.Fatal("reset did not re-arm the lane")
	}
}

func TestPermitBoundsWindowAndAttempts(t *testing.T) {
	var lane Executor
	_, session, ok := lane.Start(context.Background(), 0)
	if !ok {
		t.Fatal("session did not start")
	}
	opened := time.Now()
	for want := 1; want <= 3; want++ {
		if got, ok := lane.Permit(session, opened, time.Second, 3); !ok || got != want {
			t.Fatalf("permit %d = %d, %v", want, got, ok)
		}
	}
	if _, ok := lane.Permit(session, opened, time.Second, 3); ok {
		t.Fatal("attempt limit was bypassed")
	}
	lane.Reset()
	_, next, ok := lane.Start(context.Background(), 0)
	if !ok {
		t.Fatal("replacement session did not start")
	}
	if _, ok := lane.Permit(next, time.Now().Add(-2*time.Second), time.Second, 3); ok {
		t.Fatal("expired action window was bypassed")
	}
}

func TestTimeoutCancelsAndFinishIsScoped(t *testing.T) {
	var lane Executor
	ctx, first, ok := lane.Start(context.Background(), 10*time.Millisecond)
	if !ok {
		t.Fatal("session did not start")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("session timeout did not fire")
	}
	_, second, ok := lane.Start(context.Background(), 0)
	if !ok {
		t.Fatal("second session did not start")
	}
	lane.Finish(first)
	if !lane.Current(second) {
		t.Fatal("stale finish cancelled the current session")
	}
}
