package readycheck

import (
	"context"
	"errors"
	"leaguepingo/internal/core"
	"sync"
	"testing"
	"time"
)

type fake struct {
	mu                   sync.Mutex
	phase                string
	response             string
	posts, reads, phases int
	mode                 string
}

func (f *fake) Phase(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phases++
	return f.phase, nil
}
func (f *fake) ReadyCheck(context.Context) (core.ReadyCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.mode == "unknown" && f.posts > 0 {
		return core.ReadyCheck{}, errors.New("unavailable")
	}
	return core.ReadyCheck{State: "InProgress", PlayerResponse: f.response}, nil
}
func (f *fake) AcceptReadyCheck(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.posts++
	if f.mode != "pending" && f.mode != "unknown" {
		f.response = "Accepted"
	}
	if f.mode == "timeout" {
		return context.DeadlineExceeded
	}
	return nil
}
func (f *fake) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.posts }
func wait(t *testing.T, c *Controller, status string) {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		if c.Snapshot().Status == status {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("wanted %s, got %+v", status, c.Snapshot())
}
func start(t *testing.T, f *fake, delay int) *Controller {
	t.Helper()
	c := New()
	_ = c.Configure(true, delay)
	c.Observe(f, 1, f.phase)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("shutdown blocked")
		}
	})
	return c
}

func TestPollFallbackAndConsecutiveReadyChecks(t *testing.T) {
	f := &fake{phase: "Matchmaking", response: "None"}
	c := start(t, f, 0)
	// No phase or ready-check stream: the independent fallback discovers the pop.
	f.mu.Lock()
	f.phase = "ReadyCheck"
	f.mu.Unlock()
	wait(t, c, "accepted")
	for range 20 {
		c.Observe(f, 1, "ReadyCheck")
	}
	time.Sleep(30 * time.Millisecond)
	if f.count() != 1 {
		t.Fatal("duplicate notifications resubmitted acceptance")
	}
	f.mu.Lock()
	f.phase = "Matchmaking"
	f.mu.Unlock()
	c.Observe(f, 1, "Matchmaking")
	f.mu.Lock()
	f.phase = "ReadyCheck"
	f.response = "None"
	f.mu.Unlock()
	c.Observe(f, 1, "ReadyCheck")
	wait(t, c, "accepted")
	if f.count() != 2 {
		t.Fatal("next invitation was suppressed")
	}
}
func TestTimeoutIsVerifiedBeforeRetry(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None", mode: "timeout"}
	c := start(t, f, 0)
	wait(t, c, "accepted")
	if f.count() != 1 {
		t.Fatal("timed-out but accepted POST was retried")
	}
}
func TestUnknownResultIsNotRetried(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None", mode: "unknown"}
	c := start(t, f, 0)
	wait(t, c, "failed")
	time.Sleep(550 * time.Millisecond)
	if f.count() != 1 {
		t.Fatal("ambiguous POST was retried")
	}
}
func TestPendingRetryIsBounded(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None", mode: "pending"}
	c := start(t, f, 0)
	wait(t, c, "failed")
	if f.count() != 3 {
		t.Fatal("retry budget changed")
	}
}
func TestSkipDisarmsOnlyThisInvitation(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None"}
	c := start(t, f, 1)
	wait(t, c, "countdown")
	if _, err := c.Skip(); err != nil {
		t.Fatal(err)
	}
	c.ObserveAt(f, 1, "Matchmaking", time.Now().Add(-time.Second))
	for range 5 {
		c.Observe(f, 1, "ReadyCheck")
	}
	time.Sleep(1100 * time.Millisecond)
	if f.count() != 0 || c.Snapshot().Status != "skipped" {
		t.Fatal("skip was undone by an update")
	}
	f.mu.Lock()
	f.phase = "Matchmaking"
	f.mu.Unlock()
	c.Observe(f, 1, "Matchmaking")
	_ = c.Configure(true, 0)
	f.mu.Lock()
	f.phase = "ReadyCheck"
	f.mu.Unlock()
	c.Observe(f, 1, "ReadyCheck")
	wait(t, c, "accepted")
}
func TestManualResponseAndPhaseExitCancelCountdown(t *testing.T) {
	for _, response := range []string{"Accepted", "Declined", "exit", "disconnect", "disable"} {
		t.Run(response, func(t *testing.T) {
			f := &fake{phase: "ReadyCheck", response: "None"}
			c := start(t, f, 1)
			wait(t, c, "countdown")
			switch response {
			case "exit":
				f.mu.Lock()
				f.phase = "Lobby"
				f.mu.Unlock()
				c.Observe(f, 1, "Lobby")
			case "disconnect":
				c.Observe(nil, 0, "")
			case "disable":
				_ = c.Configure(false, 1)
			default:
				f.mu.Lock()
				f.response = response
				f.mu.Unlock()
			}
			time.Sleep(1100 * time.Millisecond)
			if f.count() != 0 {
				t.Fatal("cancelled or manually handled invitation was accepted")
			}
		})
	}
}
func TestDisabledControllerMakesNoRequests(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None"}
	c := New()
	c.Observe(f, 1, "ReadyCheck")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reads+f.posts+f.phases != 0 {
		t.Fatal("disabled controller queried LCU")
	}
}

type lateClient struct {
	*fake
	started chan struct{}
	release chan struct{}
}

func (f *lateClient) ReadyCheck(context.Context) (core.ReadyCheck, error) {
	select {
	case f.started <- struct{}{}:
	default:
	}
	<-f.release
	return core.ReadyCheck{State: "InProgress", PlayerResponse: "None"}, nil
}
func TestLateResponseCannotActAfterDisconnect(t *testing.T) {
	f := &lateClient{fake: &fake{phase: "ReadyCheck", response: "None"}, started: make(chan struct{}, 1), release: make(chan struct{})}
	c := New()
	_ = c.Configure(true, 0)
	c.Observe(f, 1, "ReadyCheck")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	select {
	case <-f.started:
	case <-time.After(time.Second):
		cancel()
		close(f.release)
		t.Fatal("request not started")
	}
	c.Observe(nil, 0, "")
	close(f.release)
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
	if f.count() != 0 {
		t.Fatal("late old-connection response submitted acceptance")
	}
}

type stalePhaseClient struct {
	*fake
	first   sync.Once
	started chan struct{}
	release chan struct{}
}

func (f *stalePhaseClient) Phase(context.Context) (string, error) {
	stale := false
	f.first.Do(func() { stale = true; close(f.started); <-f.release })
	if stale {
		return "Matchmaking", nil
	}
	return "ReadyCheck", nil
}
func TestOlderPhaseProbeCannotDisarmOrStallCurrentInvitation(t *testing.T) {
	f := &stalePhaseClient{fake: &fake{phase: "ReadyCheck", response: "None"}, started: make(chan struct{}), release: make(chan struct{})}
	c := New()
	_ = c.Configure(true, 0)
	c.Observe(f, 1, "ReadyCheck")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	select {
	case <-f.started:
	case <-time.After(time.Second):
		cancel()
		close(f.release)
		t.Fatal("probe not started")
	}
	// A repeated current-phase event arrives after the old HTTP probe began.
	c.Observe(f, 1, "ReadyCheck")
	close(f.release)
	wait(t, c, "accepted")
	cancel()
	<-done
	if f.count() != 1 {
		t.Fatal("old probe renewed acceptance")
	}
}

func TestNextPendingResponseRecoversMissedIntermediatePhases(t *testing.T) {
	f := &fake{phase: "ReadyCheck", response: "None"}
	c := start(t, f, 0)
	wait(t, c, "accepted")
	// Accepted -> None can only represent a fresh invitation. Simulate both
	// intermediate Matchmaking and ReadyCheck notifications being missed.
	f.mu.Lock()
	f.response = "None"
	f.mu.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for f.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if f.count() != 2 {
		t.Fatal("missed intermediate phases suppressed the second invitation")
	}
}
