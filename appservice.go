package main

import (
	"context"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
)

// AppService is the small UI boundary; all game data remains inside the Go engine.
type AppService struct {
	engine  *engine.Engine
	overlay *overlayController
}

func (s *AppService) Snapshot() core.Snapshot { return s.engine.Snapshot() }

func (s *AppService) Lookup(ctx context.Context, riotID string) (core.Participant, error) {
	return s.engine.Lookup(ctx, riotID)
}

func (s *AppService) ToggleOverlay() bool {
	return s.overlay.toggle().Visible
}

func (s *AppService) OverlayStatus() OverlayState               { return s.overlay.snapshot() }
func (s *AppService) HideOverlay() OverlayState                 { return s.overlay.hide() }
func (s *AppService) SetOverlayPinned(pinned bool) OverlayState { return s.overlay.setPinned(pinned) }
func (s *AppService) LookupPlayer(ctx context.Context, key string) (core.Participant, error) {
	return s.engine.LookupPlayer(ctx, key)
}
func (s *AppService) RefreshRoster(ctx context.Context) (core.Snapshot, error) {
	return s.engine.RefreshRoster(ctx)
}
