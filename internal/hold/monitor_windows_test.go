//go:build windows

package hold

import "testing"

func TestForegroundCallbackUsesWindowsCallbackABI(t *testing.T) {
	if foregroundCallback(func() {}) == 0 {
		t.Fatal("foreground callback unavailable")
	}
}
