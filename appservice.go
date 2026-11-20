package main

import (
	"context"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/guides"
	"leaguepingo/internal/settings"
)

// AppService is the small UI boundary; all game data remains inside the Go engine.
type AppService struct {
	resources         *diagnostics.Resources
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

func (s *AppService) RefreshReview(ctx context.Context, gameID int64) (serviceValue core.Snapshot, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshReview(ctx, gameID)
}

func (s *AppService) SetScoutQueue(queue int) (serviceValue core.Snapshot, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.SetScoutQueue(queue)
}

func (s *AppService) CurrentPlayer(ctx context.Context, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.CurrentPlayer(ctx, connectionID)
}

func (s *AppService) Lookup(ctx context.Context, riotID string) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
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
func (s *AppService) SetOverlayPinned(pinned bool) (serviceValue OverlayState, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	state := s.overlay.setPinned(pinned)
	if s.settings != nil {
		s.settings.SetPinned(pinned)
		if err := s.settings.Save(); err != nil {
			return state, err
		}
	}
	return state, nil
}
func (s *AppService) LookupPlayer(ctx context.Context, key string) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.LookupPlayer(ctx, key)
}
func (s *AppService) RefreshRoster(ctx context.Context) (serviceValue core.Snapshot, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshRoster(ctx)
}
func (s *AppService) RefreshLookup(ctx context.Context, riotID string) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshLookup(ctx, riotID)
}
func (s *AppService) RefreshPlayer(ctx context.Context, key string) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshPlayer(ctx, key)
}
func (s *AppService) RefreshHistory(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshHistory(ctx, key, connectionID)
}

func (s *AppService) RetryHistory(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RetryHistory(ctx, key, connectionID)
}

func (s *AppService) RefreshData(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.RefreshData(ctx, key, connectionID)
}
func (s *AppService) PrioritizePlayer(key string) { s.engine.PrioritizePlayer(key) }

func (s *AppService) HistoryPage(ctx context.Context, key string, connectionID uint64, offset int) (serviceValue core.HistoryPage, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.HistoryPage(ctx, key, connectionID, offset)
}

func (s *AppService) MatchDetail(ctx context.Context, key string, connectionID uint64, gameID int64) (serviceValue core.MatchDetail, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.MatchDetail(ctx, key, connectionID, gameID)
}

func (s *AppService) MatchTimeline(ctx context.Context, key string, connectionID uint64, gameID int64) (serviceValue core.MatchTimeline, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.MatchTimeline(ctx, key, connectionID, gameID)
}

func (s *AppService) LoadProfile(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.LoadProfile(ctx, key, connectionID)
}

func (s *AppService) EnsureProfile(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.EnsureProfile(ctx, key, connectionID)
}

func (s *AppService) LoadRanks(ctx context.Context, key string, connectionID uint64) (serviceValue core.Participant, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.LoadRanks(ctx, key, connectionID)
}

func (s *AppService) GuideStatus() guides.State {
	if s.guides == nil {
		return guides.State{Status: "idle"}
	}
	return s.guides.Snapshot()
}

func (s *AppService) GuideCandidates(scope string, champion int, ids []int) (serviceValue guides.State, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	if s.guides == nil {
		return guides.State{Status: "idle"}, nil
	}
	return s.guides.Candidates(scope, champion, ids)
}
func (s *AppService) SelectGuide(position string, opponent int, retry bool) (serviceValue guides.State, serviceErr error) {
	defer apperrors.Present(&serviceErr)
	if s.guides == nil {
		return guides.State{Status: "idle"}, nil
	}
	return s.guides.Select(position, opponent, retry)
}

func (s *AppService) WaitForQueryIdle(ctx context.Context) (serviceErr error) {
	defer apperrors.Present(&serviceErr)
	return s.engine.WaitForQueryIdle(ctx)
}
