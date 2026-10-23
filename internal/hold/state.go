// Package hold owns the physical-key state independently of any WebView.
package hold

import "sync"

type State struct {
	mu                        sync.Mutex
	enabled, allowed, visible bool
	pressed                   map[uint32]bool
	key                       uint32
	notify                    func()
}

func New(notify func()) *State {
	return &State{key: 0xC0, notify: notify, pressed: make(map[uint32]bool)}
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

func (s *State) Configure(enabled bool, key uint32) {
	s.mu.Lock()
	s.enabled, s.key, s.visible = enabled, key, false
	s.mu.Unlock()
	s.notify()
}
func (s *State) Allow(allowed bool) {
	s.mu.Lock()
	changed := s.allowed != allowed
	s.allowed = allowed
	if !allowed {
		s.visible = false
	}
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}
func (s *State) Input(key uint32, down, gameForeground bool) {
	s.mu.Lock()
	if key != 0xC0 && (key < 0x77 || key > 0x79) {
		s.mu.Unlock()
		return
	}
	wasDown := s.pressed[key]
	s.pressed[key] = down
	if key != s.key || (down && wasDown) {
		s.mu.Unlock()
		return
	}
	before := s.visible
	if !down {
		s.visible = false
	} else {
		s.visible = s.enabled && s.allowed && gameForeground
	}
	changed := before != s.visible
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}
func (s *State) Release() {
	s.mu.Lock()
	changed := s.visible
	s.visible = false
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}
func (s *State) Desired() bool           { s.mu.Lock(); defer s.mu.Unlock(); return s.visible }
func (s *State) CurrentKey() uint32      { s.mu.Lock(); defer s.mu.Unlock(); return s.key }
func (s *State) Pressed(key uint32) bool { s.mu.Lock(); defer s.mu.Unlock(); return s.pressed[key] }
