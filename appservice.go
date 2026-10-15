package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
)

// AppService is the small UI boundary; all game data remains inside the Go engine.
type AppService struct {
	engine  *engine.Engine
	overlay application.Window
}

func (s *AppService) Snapshot() core.Snapshot { return s.engine.Snapshot() }

func (s *AppService) Lookup(ctx context.Context, riotID string) (core.Participant, error) {
	return s.engine.Lookup(ctx, riotID)
}

func (s *AppService) ToggleOverlay() bool {
	if s.overlay.IsVisible() {
		s.overlay.Hide()
		return false
	}
	s.overlay.Show()
	return true
}
