//go:build windows

package hold

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"leaguepingo/internal/diagnostics"
)

func TestForegroundCallbackUsesWindowsCallbackABI(t *testing.T) {
	s := readyState(t)
	s.capture(Key("Backquote"), true)
	settle(s, true, true)
	callback := foregroundCallback(s.interrupt)
	syscall.SyscallN(callback, 0, 3, 0, 0, 0, 0, 0)
	if s.Desired() {
		t.Fatal("native foreground callback did not invalidate hold")
	}
}
func TestKeyboardCallbackAlwaysForwardsOriginalInput(t *testing.T) {
	s := readyState(t)
	var gotCode int
	var gotW uintptr
	var gotEvent unsafe.Pointer
	callback := keyboardCallback(s, func(code int, w uintptr, event unsafe.Pointer) uintptr {
		gotCode, gotW, gotEvent = code, w, event
		return 73
	})
	for _, tc := range []struct {
		name     string
		code     int
		message  uintptr
		event    keyEvent
		accepted bool
	}{
		{"negative", -1, 0x100, keyEvent{VK: 0xc0, Scan: 0x29}, false},
		{"non-action", 1, 0x100, keyEvent{VK: 0xc0, Scan: 0x29}, false},
		{"unknown-message", 0, 0x102, keyEvent{VK: 0xc0, Scan: 0x29}, false},
		{"injected", 0, 0x100, keyEvent{VK: 0xc0, Scan: 0x29, Flags: 0x10}, false},
		{"low-integrity", 0, 0x100, keyEvent{VK: 0xc0, Scan: 0x29, Flags: 2}, false},
		{"unrelated", 0, 0x100, keyEvent{VK: 'A', Scan: 0x1e}, false},
		{"unconfigured-function-key", 0, 0x100, keyEvent{VK: 0x77, Scan: 0x42}, false},
		{"wrong-physical-key", 0, 0x100, keyEvent{VK: 0xc0, Scan: 0x23}, false},
		{"physical-layout-variant", 0, 0x104, keyEvent{VK: 0xde, Scan: 0x29}, true},
		{"repeat", 0, 0x100, keyEvent{VK: 0xc0, Scan: 0x29}, true},
		{"release", 0, 0x105, keyEvent{VK: 0xc0, Scan: 0x29}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := diagnostics.KeyboardCallback.Snapshot().Calls
			event := &tc.event
			// Holding the application mutex proves the callback never takes it.
			s.mu.Lock()
			r, _, _ := syscall.SyscallN(callback, uintptr(tc.code), tc.message, uintptr(unsafe.Pointer(event)))
			s.mu.Unlock()
			runtime.KeepAlive(event)
			if r != 73 || gotCode != tc.code || gotW != tc.message || gotEvent != unsafe.Pointer(event) {
				t.Fatal("event swallowed/modified or downstream result lost")
			}
			delta := diagnostics.KeyboardCallback.Snapshot().Calls - before
			if (delta == 1) != tc.accepted {
				t.Fatalf("configured physical input metric=%d", delta)
			}
		})
	}
	// The zero payload path also must reach the next hook.
	r, _, _ := syscall.SyscallN(callback, 0, 0x100, 0)
	if r != 73 || gotEvent != nil {
		t.Fatal("nil event not forwarded")
	}
}
