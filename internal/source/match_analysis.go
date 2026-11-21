package source

import (
	"context"
	"fmt"
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
	session, err := s.sgpSession(ctx, false, gate)
	if err != nil {
		return nil, "", err
	}
	raw, err := s.getSGPAuthorized(ctx, session, fmt.Sprintf("/match-history-query/v1/products/lol/%s_%d/%s", session.platform, gameID, kind), category, gate)
	return raw, session.platform, err
}

func (s *TencentLCU) MatchTimeline(ctx context.Context, detail core.MatchDetail, target string) (core.MatchTimeline, error) {
	if detail.GameID <= 0 || !core.IsKnownPUUID(target) {
		return core.MatchTimeline{}, apperrors.New(apperrors.InvalidMatch)
	}
	localCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	raw, err := s.client.Get(localCtx, fmt.Sprintf("/lol-match-history/v1/game-timelines/%d", detail.GameID))
	cancel()
	var local core.MatchTimeline
	if err == nil {
		local, err = core.ParseMatchTimeline(raw, detail)
		if err != nil {
			return core.MatchTimeline{}, err
		}
		if !timelineNeedsEvents(local, detail) {
			return s.timelineWithRunes(ctx, local, detail, target), nil
		}
	}
	if ctx.Err() != nil {
		return core.MatchTimeline{}, ctx.Err()
	}
	raw, platform, remoteErr := s.sgpGameData(ctx, detail.GameID, "DETAILS", "/sgp-timeline")
	if remoteErr != nil {
		if err == nil {
			local.Partial = true
			return s.timelineWithRunes(ctx, local, detail, target), nil
		}
		return core.MatchTimeline{}, apperrors.Normalize(remoteErr, apperrors.DataUnavailable)
	}
	remote, parseErr := core.ParseSGPTimeline(raw, detail, target, platform)
	if parseErr != nil {
		return core.MatchTimeline{}, parseErr
	}
	if err == nil {
		remote = mergeTimelineEvents(local, remote, detail)
	}
	return s.timelineWithRunes(ctx, remote, detail, target), nil
}
