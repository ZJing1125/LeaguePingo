package hold

import "testing"

func TestHoldReleaseFocusAndRemap(t *testing.T) {
	count := 0
	s := New(func() { count++ })
	s.Configure(true, Key("Backquote"))
	s.Input(0xC0, true, true)
	if s.Desired() {
		t.Fatal("outside game")
	}
	s.Input(0xC0, false, true)
	s.Allow(true)
	s.Input(0xC0, true, false)
	if s.Desired() {
		t.Fatal("desktop")
	}
	s.Input(0xC0, false, false)
	s.Input(0xC0, true, true)
	if !s.Desired() {
		t.Fatal("hold")
	}
	n := count
	s.Input(0xC0, true, true)
	if count != n {
		t.Fatal("autorepeat")
	}
	s.Release()
	if s.Desired() {
		t.Fatal("focus loss")
	}
	s.Input(0xC0, false, true)
	s.Input(0xC0, true, true)
	s.Allow(false)
	if s.Desired() {
		t.Fatal("game end")
	}
	s.Allow(true)
	s.Configure(true, Key("F8"))
	s.Input(0xC0, true, true)
	if s.Desired() {
		t.Fatal("old mapping")
	}
	s.Input(Key("F8"), true, true)
	if !s.Desired() {
		t.Fatal("new mapping")
	}
	s.Configure(false, Key("F8"))
	if s.Desired() || Key("Alt+F4") != 0 {
		t.Fatal("disable or invalid key")
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

func TestAutorepeatCannotReopenAfterFocusPhaseOrSettingChange(t *testing.T) {
	s := New(func() {})
	s.Configure(true, Key("Backquote"))
	s.Allow(true)
	s.Input(Key("Backquote"), true, true)
	s.Release()
	s.Input(Key("Backquote"), true, true)
	if s.Desired() {
		t.Fatal("repeat reopened after focus loss")
	}
	s.Input(Key("Backquote"), false, true)
	s.Input(Key("Backquote"), true, true)
	if !s.Desired() {
		t.Fatal("fresh press not accepted")
	}
	s.Configure(false, Key("Backquote"))
	s.Configure(true, Key("Backquote"))
	s.Input(Key("Backquote"), true, true)
	if s.Desired() {
		t.Fatal("repeat reopened after disabling")
	}
	s.Input(Key("F9"), true, true)
	s.Configure(true, Key("F9"))
	s.Input(Key("F9"), true, true)
	if s.Desired() {
		t.Fatal("held remapped key opened without a new press")
	}
	s.Input(Key("F9"), false, true)
	s.Input(Key("F9"), true, true)
	s.Allow(false)
	s.Allow(true)
	s.Input(Key("F9"), true, true)
	if s.Desired() {
		t.Fatal("repeat reopened after phase change")
	}
}
