package source

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// Single explicit request/fallback, using the existing current-region session.
func (s *TencentLCU) sgpGameData(ctx context.Context, gameID int64, kind, category string) ([]byte, string, error) {
	gate := &s.channels.detail
	if err := gate.blocked(time.Now()); err != nil {
		return nil, "", err
	}
	s.sgpMu.Lock()
	r := s.sgpClientLocked()
	if err := s.ensureHistorySession(ctx, r); err != nil {
		s.sgpMu.Unlock()
		return nil, "", err
	}
	origin, token, platform := r.origin, r.token, r.platform
	s.sgpMu.Unlock()
	raw, status, err := r.getAuthorized(ctx, origin, fmt.Sprintf("/match-history-query/v1/products/lol/%s_%d/%s", platform, gameID, kind), token, category, gate)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		s.sgpMu.Lock()
		if r.token == token {
			r.token = ""
		}
		s.sgpMu.Unlock()
	}
	return raw, platform, err
}

func (s *TencentLCU) MatchTimeline(ctx context.Context, detail core.MatchDetail, target string) (core.MatchTimeline, error) {
	if detail.GameID <= 0 || !core.IsKnownPUUID(target) {
		return core.MatchTimeline{}, apperrors.New(apperrors.InvalidMatch)
	}
	localCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	raw, err := s.client.Get(localCtx, fmt.Sprintf("/lol-match-history/v1/game-timelines/%d", detail.GameID))
	cancel()
	if err == nil {
		return core.ParseMatchTimeline(raw, detail)
	}
	if ctx.Err() != nil {
		return core.MatchTimeline{}, ctx.Err()
	}
	raw, platform, err := s.sgpGameData(ctx, detail.GameID, "DETAILS", "/sgp-timeline")
	if err != nil {
		return core.MatchTimeline{}, apperrors.Normalize(err, apperrors.DataUnavailable)
	}
	return core.ParseSGPTimeline(raw, detail, target, platform)
}
