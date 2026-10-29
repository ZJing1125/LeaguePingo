package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/settings"
)

func TestExportDiagnosticsContainsNoPlayerData(t *testing.T) {
	dir := t.TempDir()
	store, _ := settings.Open(filepath.Join(dir, "settings.json"))
	s := &AppService{settings: store, configDir: dir}
	diagnostics.Default.Record("/lol-summoner/v1/summoners?name=secret-player", 0, 404, true)
	diagnostics.Default.Record("/sgp-history", 0, 200, false)
	diagnostics.KeyboardCallback.Record(250 * time.Microsecond)
	file, err := s.ExportDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(file, dir+string(os.PathSeparator)) || strings.Contains(string(b), "secret-player") || strings.Contains(string(b), "puuid") || !strings.Contains(string(b), "performanceScope") {
		t.Fatal("unsafe diagnostics export")
	}
	var report struct {
		Performance performanceDiagnostics `json:"performance"`
		Info        AppInfo                `json:"info"`
	}
	if json.Unmarshal(b, &report) != nil || report.Performance.Keyboard.Calls == 0 || report.Performance.Resources.LastCPUPercentMachine != nil || report.Info.Requests["sgp-history"].P95MS == nil {
		t.Fatal("performance metrics missing or unknown fabricated")
	}
	performanceJSON, _ := json.Marshal(report.Performance)
	for _, forbidden := range []string{`"pid"`, `"created"`, `"processes"`, `"groupToken"`, `"cpuSeconds"`} {
		if strings.Contains(string(performanceJSON), forbidden) {
			t.Fatal("unsafe performance export", forbidden)
		}
	}
}

func TestCompanionDiagnosticsExcludeIdentityAndGameID(t *testing.T) {
	dir := t.TempDir()
	store, _ := settings.Open(filepath.Join(dir, "settings.json"))
	o := &overlayController{dispatch: func(f func()) { f() }, ready: true, state: OverlayState{Mode: "hidden", HoldWarning: "测试原生状态"}}
	o.latest.Phase = "InProgress"
	o.latest.Stage = "in-game"
	o.latest.GameID = 987654321234
	o.latest.SelfKey = "private-player-id"
	s := &AppService{settings: store, configDir: dir, overlay: o}
	file, err := s.ExportDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "private-player-id") || strings.Contains(string(b), "987654321234") || !strings.Contains(string(b), `"frontendReady": true`) || !strings.Contains(string(b), `"phase": "InProgress"`) {
		t.Fatal("unsafe or incomplete companion diagnostics")
	}
}

func TestAutoAcceptSaveThenApplyAndRollback(t *testing.T) {
	dir := t.TempDir()
	store, _ := settings.Open(filepath.Join(dir, "settings.json"))
	e := engine.New(nil, nil)
	s := &AppService{settings: store, engine: e}
	if _, err := s.ConfigureAutoAccept(true, 3); err != nil {
		t.Fatal(err)
	}
	if !e.AutoAcceptStatus().Enabled || e.AutoAcceptStatus().DelaySeconds != 3 {
		t.Fatal("saved preference was not applied")
	}
	if _, err := s.ConfigureAutoAccept(true, 2); err == nil {
		t.Fatal("invalid delay accepted")
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	broken, _ := settings.Open(filepath.Join(blocker, "settings.json"))
	s.settings = broken
	_ = e.ConfigureAutoAccept(false, 1)
	if _, err := s.ConfigureAutoAccept(true, 5); err == nil {
		t.Fatal("save failure was hidden")
	}
	if broken.Snapshot().AutoAcceptEnabled || e.AutoAcceptStatus().Enabled {
		t.Fatal("save failure enabled the feature")
	}
}

func TestPostGameSaveAppliesOnlyAfterPersistence(t *testing.T) {
	dir := t.TempDir()
	store, _ := settings.Open(filepath.Join(dir, "settings.json"))
	e := engine.New(nil, nil)
	s := &AppService{settings: store, engine: e}
	if _, err := s.ConfigurePostGame(true, true, 10); err != nil {
		t.Fatal(err)
	}
	if !e.PostGameStatus().ReturnEnabled || !e.PostGameStatus().QueueEnabled {
		t.Fatal("post-game setting not applied")
	}
	if _, err := s.ConfigurePostGame(true, true, 0); err == nil {
		t.Fatal("unsupported delay accepted")
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	broken, _ := settings.Open(filepath.Join(blocker, "settings.json"))
	s.settings = broken
	_ = e.ConfigurePostGame(false, false, 10)
	if _, err := s.ConfigurePostGame(true, true, 15); err == nil {
		t.Fatal("save failure hidden")
	}
	if broken.Snapshot().AutoQueueEnabled || e.PostGameStatus().QueueEnabled {
		t.Fatal("failed save enabled automatic queue")
	}
}
