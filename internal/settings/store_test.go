package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySettingsKeepKeysAndGainCompanionDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"shortcut":"Ctrl+Shift+Q","holdEnabled":false,"holdKey":"F9"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Snapshot()
	if c.Shortcut != "Ctrl+Shift+Q" || c.HoldEnabled || c.HoldKey != "F9" || !c.SidebarEnabled || !c.PreviewEnabled || c.SidebarWidth != 320 {
		t.Fatal("legacy preferences/default migration failed")
	}
	s.SetCompanion(false, false, 360)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c = s.Snapshot()
	if c.SidebarEnabled || c.PreviewEnabled || c.SidebarWidth != 360 || c.HoldKey != "F9" {
		t.Fatal("companion save replaced old shortcut preferences")
	}
}

func TestStoreRoundtripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.SetBounds(false, Rect{X: -1400, Y: 20, Width: 500, Height: 600})
	s.SetPinned(false)
	s.SetShortcut("Ctrl+Shift+L")
	if err = s.Save(); err != nil {
		t.Fatal(err)
	}
	next, err := Open(path)
	if err != nil || next.Snapshot().Overlay.X != -1400 || next.Snapshot().Pinned || next.Snapshot().Shortcut != "Ctrl+Shift+L" {
		t.Fatal("settings lost")
	}
	if err = next.Save(); err != nil {
		t.Fatal("replace existing settings", err)
	}
	if err = os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	next, err = Open(path)
	if err == nil || next.Snapshot() != Default() {
		t.Fatal("corrupt settings not recovered")
	}
}
func TestClampDisconnectedAndNegativeMonitor(t *testing.T) {
	areas := []Rect{{Width: 1920, Height: 1040}, {X: -1600, Width: 1600, Height: 900}}
	r := Clamp(Rect{X: 99999, Y: 99999, Width: 540, Height: 660}, areas, 400, 360)
	if r.X+r.Width > 1920 || r.Y+r.Height > 1040 {
		t.Fatal("offscreen window")
	}
	r = Clamp(Rect{X: -1500, Y: 10, Width: 540, Height: 660}, areas, 400, 360)
	if r.X != -1500 {
		t.Fatal("valid negative monitor lost")
	}
	if _, err := NormalizeShortcut("Alt+F4"); err == nil {
		t.Fatal("unsafe shortcut accepted")
	}
}

func TestExistingSettingsReceiveHoldDefaultsAndRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"shortcut":"Ctrl+Shift+P","pinned":true,"overlay":{"width":540,"height":660}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Snapshot().HoldEnabled || s.Snapshot().HoldKey != "Backquote" || s.Snapshot().Overlay.Width != 540 {
		t.Fatal("migration lost window position or defaults")
	}
	s.SetHold(false, "F9")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil || s.Snapshot().HoldEnabled || s.Snapshot().HoldKey != "F9" {
		t.Fatal("hold preference lost")
	}
	if ValidHoldKey("Alt+F4") {
		t.Fatal("invalid hold key")
	}
}

func TestAutoAcceptDefaultsAndRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().AutoAcceptEnabled || s.Snapshot().AutoAcceptDelay != 1 {
		t.Fatal("auto accept must default off with one-second delay")
	}
	s.SetAutoAccept(true, 3)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil || !s.Snapshot().AutoAcceptEnabled || s.Snapshot().AutoAcceptDelay != 3 {
		t.Fatal("preference not persisted")
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"autoAcceptEnabled":true,"autoAcceptDelay":-1}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, _ = Open(path)
	if s.Snapshot().AutoAcceptEnabled || s.Snapshot().AutoAcceptDelay != 1 {
		t.Fatal("invalid preference should disarm the action")
	}
}
