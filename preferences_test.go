package main

import (
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/settings"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportDiagnosticsContainsNoPlayerData(t *testing.T) {
	dir := t.TempDir()
	store, _ := settings.Open(filepath.Join(dir, "settings.json"))
	s := &AppService{settings: store, configDir: dir}
	diagnostics.Default.Record("/lol-summoner/v1/summoners?name=secret-player", 0, 404, true)
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
