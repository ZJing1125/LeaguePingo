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
	if c.Shortcut != "Ctrl+Shift+Q" || c.HoldEnabled || c.HoldKey != "F9" || !c.SidebarEnabled || !c.PreviewEnabled || c.SidebarWidth != 320 || !c.GuidesEnabled {
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
	s.SetGuides(false)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil || s.Snapshot().GuidesEnabled {
		t.Fatal("guide preference not persisted")
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

func TestAnalysisBriefDefaultsAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil || s.Snapshot().AutoAnalysisBrief || s.Snapshot().AnalysisAudience != "team" || !s.Snapshot().AnalysisIncludeSelf {
		t.Fatal("unsafe or incomplete brief defaults")
	}
	s.SetAnalysisBrief(true, "self", false)
	if s.Save() != nil {
		t.Fatal("save brief config")
	}
	loaded, err := Open(path)
	if err != nil || !loaded.Snapshot().AutoAnalysisBrief || loaded.Snapshot().AnalysisAudience != "self" || loaded.Snapshot().AnalysisIncludeSelf {
		t.Fatal("brief options lost")
	}
}

func TestPostGameDefaultsAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if s.Snapshot().AutoReturnEnabled || s.Snapshot().AutoQueueEnabled || s.Snapshot().AutoQueueDelay != 10 {
		t.Fatal("post-game default must be off/10 seconds")
	}
	s.SetPostGame(true, true, 15)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil || !s.Snapshot().AutoReturnEnabled || !s.Snapshot().AutoQueueEnabled || s.Snapshot().AutoQueueDelay != 15 {
		t.Fatal("post-game config not persisted")
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"autoReturnEnabled":true,"autoQueueEnabled":true,"autoQueueDelay":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, _ = Open(path)
	if s.Snapshot().AutoReturnEnabled || s.Snapshot().AutoQueueEnabled || s.Snapshot().AutoQueueDelay != 10 {
		t.Fatal("invalid delay did not disarm post-game automation")
	}
}

func TestSkipHonorIndependentPreferenceAndOldConfigs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if s.Snapshot().AutoSkipHonor {
		t.Fatal("skip honor enabled by default")
	}
	s.SetSkipHonor(true)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil || !s.Snapshot().AutoSkipHonor || s.Snapshot().AutoReturnEnabled || s.Snapshot().AutoQueueEnabled {
		t.Fatal("skip honor preference coupled to return/queue", err)
	}
	s.SetPostGame(true, true, 15)
	if !s.Snapshot().AutoSkipHonor {
		t.Fatal("return config overwrote skip preference")
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"autoReturnEnabled":true,"autoQueueEnabled":true,"autoQueueDelay":10}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil || s.Snapshot().AutoSkipHonor || !s.Snapshot().AutoReturnEnabled {
		t.Fatal("old configuration unexpectedly enabled skip", err)
	}
}
