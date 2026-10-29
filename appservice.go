package main

import (
	"context"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/guides"
	"leaguepingo/internal/settings"
)

// AppService is the small UI boundary; all game data remains inside the Go engine.
type AppService struct {
	guides            *guides.Service
	engine            *engine.Engine
	overlay           *overlayController
	settings          *settings.Store
	configDir         string
	settingsWarning   string
	configureShortcut func(string) error
	configureHold     func(bool, string) error
}

func (s *AppService) Snapshot() core.Snapshot { return s.engine.Snapshot() }

func (s *AppService) RefreshReview(ctx context.Context, gameID int64) (core.Snapshot, error) {
	return s.engine.RefreshReview(ctx, gameID)
}

func (s *AppService) SetScoutQueue(queue int) (core.Snapshot, error) {
	return s.engine.SetScoutQueue(queue)
}

func (s *AppService) CurrentPlayer(ctx context.Context, connectionID uint64) (core.Participant, error) {
	return s.engine.CurrentPlayer(ctx, connectionID)
}

func (s *AppService) Lookup(ctx context.Context, riotID string) (core.Participant, error) {
	return s.engine.Lookup(ctx, riotID)
}

func (s *AppService) ToggleOverlay() bool {
	return s.overlay.toggle().Visible
}

func (s *AppService) OverlayStatus() OverlayState { return s.overlay.snapshot() }
func (s *AppService) AuxiliaryReady()             { s.overlay.frontendReady() }
func (s *AppService) SetSidebarCollapsed(collapsed bool) OverlayState {
	return s.overlay.collapse(collapsed)
}
func (s *AppService) HideOverlay() OverlayState { return s.overlay.hide() }
func (s *AppService) SetOverlayPinned(pinned bool) (OverlayState, error) {
	state := s.overlay.setPinned(pinned)
	if s.settings != nil {
		s.settings.SetPinned(pinned)
		if err := s.settings.Save(); err != nil {
			return state, err
		}
	}
	return state, nil
}
func (s *AppService) LookupPlayer(ctx context.Context, key string) (core.Participant, error) {
	return s.engine.LookupPlayer(ctx, key)
}
func (s *AppService) RefreshRoster(ctx context.Context) (core.Snapshot, error) {
	return s.engine.RefreshRoster(ctx)
}
func (s *AppService) RefreshLookup(ctx context.Context, riotID string) (core.Participant, error) {
	return s.engine.RefreshLookup(ctx, riotID)
}
func (s *AppService) RefreshPlayer(ctx context.Context, key string) (core.Participant, error) {
	return s.engine.RefreshPlayer(ctx, key)
}
func (s *AppService) RefreshHistory(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	return s.engine.RefreshHistory(ctx, key, connectionID)
}

func (s *AppService) RefreshData(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	return s.engine.RefreshData(ctx, key, connectionID)
}
func (s *AppService) PrioritizePlayer(key string) { s.engine.PrioritizePlayer(key) }

func (s *AppService) HistoryPage(ctx context.Context, key string, connectionID uint64, offset int) (core.HistoryPage, error) {
	return s.engine.HistoryPage(ctx, key, connectionID, offset)
}

func (s *AppService) MatchDetail(ctx context.Context, key string, connectionID uint64, gameID int64) (core.MatchDetail, error) {
	return s.engine.MatchDetail(ctx, key, connectionID, gameID)
}

func (s *AppService) MatchTimeline(ctx context.Context, key string, connectionID uint64, gameID int64) (core.MatchTimeline, error) {
	return s.engine.MatchTimeline(ctx, key, connectionID, gameID)
}

func (s *AppService) LoadProfile(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	return s.engine.LoadProfile(ctx, key, connectionID)
}

func (s *AppService) EnsureProfile(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	return s.engine.EnsureProfile(ctx, key, connectionID)
}

func (s *AppService) LoadRanks(ctx context.Context, key string, connectionID uint64) (core.Participant, error) {
	return s.engine.LoadRanks(ctx, key, connectionID)
}

func (s *AppService) GuideStatus() guides.State {
	if s.guides == nil {
		return guides.State{Status: "idle"}
	}
	return s.guides.Snapshot()
}

func (s *AppService) GuideCandidates(scope string, champion int, ids []int) (guides.State, error) {
	if s.guides == nil {
		return guides.State{Status: "idle"}, nil
	}
	return s.guides.Candidates(scope, champion, ids)
}
func (s *AppService) SelectGuide(position string, opponent int, retry bool) (guides.State, error) {
	if s.guides == nil {
		return guides.State{Status: "idle"}, nil
	}
	return s.guides.Select(position, opponent, retry)
}
