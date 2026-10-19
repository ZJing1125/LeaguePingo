package settings

import (
	"os"
	"path/filepath"
	"testing"
)

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
