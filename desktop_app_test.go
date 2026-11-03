package main

import (
	"context"
	"path/filepath"
	"testing"

	"leaguepingo/internal/settings"
)

func TestDesktopConfigDirKeepsSmokeSettingsIsolated(t *testing.T) {
	normal := filepath.Join(t.TempDir(), "normal")
	isolated := filepath.Join(t.TempDir(), "isolated")
	if got, err := desktopConfigDir(normal, false, isolated); err != nil || got != filepath.Join(normal, "LeaguePingo") {
		t.Fatal("normal startup changed its settings location")
	}
	if got, err := desktopConfigDir(normal, true, isolated); err != nil || got != isolated {
		t.Fatal("offline smoke escaped the isolated location")
	}
	if _, err := desktopConfigDir(normal, true, ""); err == nil {
		t.Fatal("offline smoke silently used normal settings")
	}
}

func TestDesktopShutdownCancelsWorkersAndPersistsSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	store.SetHold(false, "F10")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &desktopApplication{ctx: ctx, cancel: cancel, backend: &AppService{settings: store}}
	r.shutdown()
	if ctx.Err() == nil {
		t.Fatal("shutdown did not cancel background services")
	}
	reopened, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg := reopened.Snapshot(); cfg.HoldEnabled || cfg.HoldKey != "F10" {
		t.Fatal("shutdown lost pending settings")
	}
}
