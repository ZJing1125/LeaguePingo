package hold

import (
	"context"
	"time"
)

const releaseCheckInterval = 150 * time.Millisecond

// reconcile runs off the hook's message thread. Native process/key reads and
// State's notification are deliberately confined here, including the timer.
func (s *State) reconcile(word uint64, checkPhysical bool, keyDown func(uint32) bool, foreground func() bool) bool {
	if word&(inputEnabled|inputAllowed) != inputEnabled|inputAllowed {
		s.publish(word, false)
		return false
	}
	key := uint32(word & inputKeyMask)
	// WH_KEYBOARD_LL runs before Windows updates GetAsyncKeyState. Trust a
	// fresh captured edge; sampling it immediately can falsely discard a press.
	physical := word&inputDown != 0
	if checkPhysical || word&inputBlocked != 0 {
		physical = keyDown(key)
	}
	if !physical {
		// Recover a missed up, or arm a newly enabled binding only while up.
		if word&(inputDown|inputBlocked) != 0 {
			next := revise(word, word&inputFlags&^(inputDown|inputBlocked))
			if !s.input.CompareAndSwap(word, next) {
				return s.active()
			}
			word = next
		}
		s.publish(word, false)
		return s.active()
	}
	if word&inputDown == 0 {
		// A key already held at startup/remap is not an observed new press.
		next := revise(word, word&inputFlags|inputDown|inputBlocked)
		if !s.input.CompareAndSwap(word, next) {
			return s.active()
		}
		word = next
	}
	if word&inputBlocked != 0 {
		s.publish(word, false)
		return s.active()
	}
	inGame := foreground()
	if checkPhysical {
		physical = keyDown(key) // Catch a missed up during a periodic read too.
	}
	if !inGame || !physical {
		flags := word&inputFlags | inputBlocked
		if !physical {
			flags &^= inputDown | inputBlocked
		}
		next := revise(word, flags)
		if !s.input.CompareAndSwap(word, next) {
			return s.active()
		}
		word = next
	}
	s.publish(word, inGame && physical && word&inputBlocked == 0)
	return s.active()
}
func (s *State) active() bool {
	word := s.input.Load()
	return word&(inputEnabled|inputAllowed|inputDown) == inputEnabled|inputAllowed|inputDown
}
func (s *State) run(ctx context.Context, keyDown func(uint32) bool, foreground func() bool) {
	var timer *time.Timer
	var tick <-chan time.Time
	var lastWord uint64
	stop := func() {
		if timer != nil {
			timer.Stop()
			timer, tick = nil, nil
		}
	}
	defer stop()
	defer s.Allow(false)
	for {
		checkPhysical := false
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-tick:
			checkPhysical = true
		}
		if ctx.Err() != nil {
			return
		}
		word := s.input.Load()
		// A stale timer must not sample a new key edge before Windows commits it.
		active := s.reconcile(word, checkPhysical && word == lastWord, keyDown, foreground)
		lastWord = word
		if active {
			if timer == nil {
				timer = time.NewTimer(releaseCheckInterval)
			} else {
				timer.Reset(releaseCheckInterval)
			}
			tick = timer.C
		} else {
			stop()
		}
	}
}
