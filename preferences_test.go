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
