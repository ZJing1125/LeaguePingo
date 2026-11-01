// Package hold owns the configured physical-key state independently of WebViews.
package hold

import (
	"sync"
	"sync/atomic"
)

const (
	inputKeyMask uint64 = 0xff
	inputEnabled uint64 = 1 << 8
	inputAllowed uint64 = 1 << 9
	inputDown    uint64 = 1 << 10
	inputBlocked uint64 = 1 << 11
	inputVersion uint64 = 1 << 16
	inputFlags   uint64 = inputVersion - 1
)

type State struct {
	// Native callbacks only touch input and wake. One word contains the binding,
	// physical level and revision, so coalescing cannot replay an obsolete press.
	input  atomic.Uint64
	wake   chan struct{}
	mu     sync.Mutex
	shown  uint64
	notify func()
}

func New(notify func()) *State {
	s := &State{notify: notify, wake: make(chan struct{}, 1)}
	s.input.Store(uint64(Key("Backquote")) | inputBlocked)
	return s
}
func Key(name string) uint32 {
	switch name {
	case "Backquote":
		return 0xC0
	case "F8":
		return 0x77
	case "F9":
		return 0x78
	case "F10":
		return 0x79
	}
	return 0
}

// Backquote is the physical key below Esc (scan 0x29), even when the
// keyboard layout assigns a different virtual key. Function keys use VK codes.
func PhysicalKey(vk, scan uint32, extended bool) uint32 {
	if scan == 0x29 && !extended {
		return 0xC0
	}
	if vk == 0xC0 {
		return 0
	}
	return vk
}

func revise(old, flags uint64) uint64 { return (old &^ inputFlags) + inputVersion | flags }
func (s *State) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *State) Configure(enabled bool, key uint32) {
	s.mu.Lock()
	for old := s.input.Load(); ; old = s.input.Load() {
		flags := old & inputAllowed
		if key != 0 && (key == Key("Backquote") || key >= Key("F8") && key <= Key("F10")) {
			flags |= uint64(key)
			if enabled {
				flags |= inputEnabled
			}
		}
		// A binding change never turns an already held key into a fresh press.
		if old&inputKeyMask == flags&inputKeyMask {
			flags |= old & inputDown
		}
		if s.input.CompareAndSwap(old, revise(old, flags|inputBlocked)) {
			break
		}
	}
	s.shown = 0
	s.mu.Unlock()
	s.notify()
	s.signal()
}
func (s *State) Allow(allowed bool) {
	s.mu.Lock()
	for old := s.input.Load(); ; old = s.input.Load() {
		if (old&inputAllowed != 0) == allowed {
			s.mu.Unlock()
			return
		}
		flags := old & inputFlags &^ inputAllowed
		if allowed {
			flags |= inputAllowed
		}
		if s.input.CompareAndSwap(old, revise(old, flags|inputBlocked)) {
			break
		}
	}
	s.shown = 0
	s.mu.Unlock()
	s.notify()
	s.signal()
}

// capture is called on the hook thread. It retains only the enabled configured
// key's current level, never an input stream. Auto-repeat does not queue work.
func (s *State) capture(key uint32, down bool) bool {
	for old := s.input.Load(); ; old = s.input.Load() {
		if old&inputEnabled == 0 || uint32(old&inputKeyMask) != key {
			return false
		}
		if (old&inputDown != 0) == down {
			return true
		}
		flags := old & inputFlags &^ inputDown
		if down {
			flags |= inputDown
		} else {
			flags &^= inputBlocked
		}
		if s.input.CompareAndSwap(old, revise(old, flags)) {
			s.signal()
			return true
		}
	}
}

// interrupt is also callback-safe. Losing foreground invalidates an in-flight
// read even if focus returns before the worker finishes. Release is required.
func (s *State) interrupt() {
	for old := s.input.Load(); ; old = s.input.Load() {
		if old&inputDown == 0 {
			return
		}
		if s.input.CompareAndSwap(old, revise(old, old&inputFlags|inputBlocked)) {
			s.signal()
			return
		}
	}
}
func (s *State) Desired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shown != 0 && s.input.Load() == s.shown
}

// publish runs only on the worker, after native reads. Configuration, release,
// focus and shutdown all change the revision and reject a delayed show.
func (s *State) publish(expected uint64, visible bool) {
	s.mu.Lock()
	if s.input.Load() != expected {
		s.mu.Unlock()
		return
	}
	before := s.shown
	s.shown = 0
	if visible {
		s.shown = expected
	}
	changed := before != s.shown
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}
