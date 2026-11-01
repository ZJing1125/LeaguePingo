package hold

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func readyState(t *testing.T) *State {
	t.Helper()
	s := New(func() {})
	s.Configure(true, Key("Backquote"))
	s.Allow(true)
	s.reconcile(s.input.Load(), true, func(uint32) bool { return false }, func() bool { t.Fatal("idle process query"); return false })
	return s
}
func settle(s *State, down, foreground bool) {
	s.reconcile(s.input.Load(), true, func(uint32) bool { return down }, func() bool { return foreground })
}
func TestHoldReleaseFocusAndRemap(t *testing.T) {
	s := readyState(t)
	s.capture(Key("Backquote"), true)
	settle(s, true, true)
	if !s.Desired() {
		t.Fatal("hold")
	}
	word := s.input.Load()
	s.capture(Key("Backquote"), true)
	if s.input.Load() != word || !s.Desired() {
		t.Fatal("autorepeat queued work or hid window")
	}
	s.interrupt()
	if s.Desired() {
		t.Fatal("foreground change must immediately invalidate visibility")
	}
	settle(s, true, true)
	if s.Desired() {
		t.Fatal("focus return reopened a held key")
	}
	s.capture(Key("Backquote"), false)
	s.capture(Key("Backquote"), true)
	settle(s, true, true)
	if !s.Desired() {
		t.Fatal("fresh press after focus")
	}
	s.Configure(true, Key("F8"))
	settle(s, false, true)
	if s.capture(Key("Backquote"), true) || s.Desired() {
		t.Fatal("old binding")
	}
	s.capture(Key("F8"), true)
	settle(s, true, true)
	if !s.Desired() {
		t.Fatal("new binding")
	}
	s.Configure(false, Key("F8"))
	if s.Desired() || s.capture(Key("F8"), true) || Key("Alt+F4") != 0 {
		t.Fatal("disabled or invalid binding")
	}
}
func TestPhysicalBackquoteAcrossLayouts(t *testing.T) {
	if PhysicalKey(0xde, 0x29, false) != Key("Backquote") {
		t.Fatal("layout-specific virtual key lost physical mapping")
	}
	if PhysicalKey(0xc0, 0x23, false) == Key("Backquote") || PhysicalKey(0xde, 0x29, true) == Key("Backquote") {
		t.Fatal("another physical key matched")
	}
	if PhysicalKey(0x78, 0x43, false) != Key("F9") {
		t.Fatal("function key mapping")
	}
}

// Windows delivers the low-level hook before updating asynchronous key state.
// The fresh-edge path must not poll that old state, even if an old timer fires.
func TestFreshPressDoesNotSamplePreEventAsyncKeyState(t *testing.T) {
	s := readyState(t)
	s.capture(Key("Backquote"), true)
	s.reconcile(s.input.Load(), false, func(uint32) bool {
		t.Fatal("fresh press sampled pre-event async state")
		return false
	}, func() bool { return true })
	if !s.Desired() {
		t.Fatal("fresh press discarded")
	}
	settle(s, false, true)
	if s.Desired() {
		t.Fatal("later missed-up sampling failed")
	}
}

func TestOnlyConfiguredInputAndBoundedCoalescing(t *testing.T) {
	s := readyState(t)
	before := s.input.Load()
	for _, key := range []uint32{0, 'A', Key("F8"), Key("F9"), Key("F10")} {
		if s.capture(key, true) || s.input.Load() != before {
			t.Fatal("unconfigured input was retained")
		}
	}
	for range 10000 {
		s.capture(Key("Backquote"), true)
		s.capture(Key("Backquote"), false)
	}
	settle(s, false, true)
	if len(s.wake) != 1 || s.Desired() {
		t.Fatal("backlog replayed a released press")
	}
	if got := testing.AllocsPerRun(100, func() {
		s.capture(Key("Backquote"), true)
		s.interrupt()
		s.capture(Key("Backquote"), false)
	}); got != 0 {
		t.Fatalf("callback path allocates: %v", got)
	}
}
func TestCaptureDoesNotWaitForApplicationLock(t *testing.T) {
	s := readyState(t)
	s.mu.Lock()
	done := make(chan struct{})
	go func() {
		s.capture(Key("Backquote"), true)
		s.interrupt()
		s.capture(Key("Backquote"), false)
		close(done)
	}()
	select {
	case <-done:
		s.mu.Unlock()
	case <-time.After(time.Second):
		s.mu.Unlock()
		t.Fatal("callback waited for State mutex")
	}
}
func TestHeldKeyCannotReopenAcrossConfigurationOrPhase(t *testing.T) {
	for _, change := range []struct {
		name string
		fn   func(*State)
		key  uint32
	}{
		{"disable-enable", func(s *State) { s.Configure(false, Key("Backquote")); s.Configure(true, Key("Backquote")) }, Key("Backquote")},
		{"phase-exit-enter", func(s *State) { s.Allow(false); s.Allow(true) }, Key("Backquote")},
		{"remap-already-held", func(s *State) { s.Configure(true, Key("F9")) }, Key("F9")},
	} {
		t.Run(change.name, func(t *testing.T) {
			s := readyState(t)
			s.capture(Key("Backquote"), true)
			settle(s, true, true)
			change.fn(s)
			s.capture(change.key, true)
			settle(s, true, true)
			if s.Desired() {
				t.Fatal("held key reopened")
			}
			s.capture(change.key, false)
			s.capture(change.key, true)
			settle(s, true, true)
			if !s.Desired() {
				t.Fatal("fresh press failed")
			}
		})
	}
}
func TestDelayedForegroundReadCannotShowObsoletePress(t *testing.T) {
	for _, change := range []struct {
		name string
		fn   func(*State)
	}{
		{"release", func(s *State) { s.capture(Key("Backquote"), false) }},
		{"rapid-release-repress", func(s *State) { s.capture(Key("Backquote"), false); s.capture(Key("Backquote"), true) }},
		{"focus-out-and-back", func(s *State) { s.interrupt(); s.interrupt() }},
		{"disable", func(s *State) { s.Configure(false, Key("Backquote")) }},
		{"remap", func(s *State) { s.Configure(true, Key("F8")) }},
		{"phase", func(s *State) { s.Allow(false); s.Allow(true) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			s := readyState(t)
			s.capture(Key("Backquote"), true)
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				s.reconcile(s.input.Load(), true, func(uint32) bool { return true }, func() bool { close(entered); <-release; return true })
				close(done)
			}()
			<-entered
			change.fn(s)
			close(release)
			<-done
			if s.Desired() {
				t.Fatal("late process result showed an obsolete key scope")
			}
		})
	}
}
func TestMissingUpOrForegroundEventRecovers(t *testing.T) {
	for _, foreground := range []bool{false, true} {
		s := readyState(t)
		s.capture(Key("Backquote"), true)
		settle(s, true, true)
		// Either the hook missed up, or the foreground notification was missed.
		settle(s, !foreground, foreground)
		if s.Desired() {
			t.Fatal("timer did not release")
		}
		s.capture(Key("Backquote"), false)
		s.capture(Key("Backquote"), true)
		settle(s, true, true)
		if !s.Desired() {
			t.Fatal("fresh press after missed event was disabled")
		}
	}
}
func TestNoProcessReadsOutsideActiveHold(t *testing.T) {
	s := New(func() {})
	query := func() bool { t.Fatal("unexpected process query"); return false }
	key := func(uint32) bool { t.Fatal("unexpected key query"); return false }
	s.reconcile(s.input.Load(), true, key, query)
	s.Configure(true, Key("F8"))
	s.reconcile(s.input.Load(), true, key, query)
	s.Allow(true)
	s.reconcile(s.input.Load(), true, func(uint32) bool { return false }, query)
	s.reconcile(s.input.Load(), true, func(uint32) bool { return true }, query) // Already held, not observed down.
	s.interrupt()
	s.reconcile(s.input.Load(), true, func(uint32) bool { return true }, query)
	s.Allow(false)
	s.reconcile(s.input.Load(), true, key, query)
}
func TestWorkerLifecycleAndMissedReleaseTimer(t *testing.T) {
	s := readyState(t)
	var physical atomic.Bool
	shown := make(chan bool, 10)
	s.notify = func() { shown <- s.Desired() }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.run(ctx, func(uint32) bool { return physical.Load() }, func() bool { return true })
		close(done)
	}()
	physical.Store(true)
	s.capture(Key("Backquote"), true)
	waitShown := func(want bool) {
		t.Helper()
		select {
		case got := <-shown:
			if got != want {
				t.Fatalf("visible=%v want %v", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not reconcile")
		}
	}
	waitShown(true)
	physical.Store(false) // No up event: worker timer must recover.
	waitShown(false)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker leaked")
	}
	s.capture(Key("Backquote"), true)
	if s.Desired() {
		t.Fatal("shutdown reopened hold")
	}
}
func TestConcurrentConfigurationCaptureAndReads(t *testing.T) {
	s := readyState(t)
	var wg sync.WaitGroup
	for task := range 3 {
		wg.Go(func() {
			for i := range 500 {
				switch task {
				case 0:
					s.capture(Key("Backquote"), i%2 == 0)
					s.interrupt()
				case 1:
					s.Configure(i%2 == 0, Key("Backquote"))
					s.Allow(i%3 == 0)
				case 2:
					settle(s, true, true)
					_ = s.Desired()
				}
			}
		})
	}
	wg.Wait()
	s.Allow(false)
	if s.Desired() {
		t.Fatal("concurrent phase exit lost")
	}
}
