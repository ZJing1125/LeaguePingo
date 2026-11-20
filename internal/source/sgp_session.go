package source

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
)

// Only refreshes of the same token wait for one another. Neither local nor
// remote I/O holds sgpMu, and a waiting caller keeps its own cancellation budget.
type sgpAuthLane struct {
	once sync.Once
	slot chan struct{}
}

func (l *sgpAuthLane) acquire(ctx context.Context) error {
	l.once.Do(func() { l.slot = make(chan struct{}, 1) })
	select {
	case l.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-l.slot
			return apperrors.Request(ctx, err, apperrors.LocalClient)
		}
		return nil
	case <-ctx.Done():
		return apperrors.Request(ctx, ctx.Err(), apperrors.LocalClient)
	}
}

type sgpSession struct {
	client                  *sgpHistory
	epoch                   uint64
	origin, platform, token string
	league                  bool
}

func (s *TencentLCU) sgpSession(ctx context.Context, league bool, gate *serviceGate) (sgpSession, error) {
	s.sgpMu.Lock()
	epoch := s.sgpEpoch
	s.sgpMu.Unlock()
	lane := &s.historyAuth
	if league {
		lane = &s.rankAuth
	}
	if err := lane.acquire(ctx); err != nil {
		return sgpSession{}, err
	}
	defer func() { <-lane.slot }()
	if err := gate.blocked(time.Now()); err != nil {
		return sgpSession{}, err
	}
	s.sgpMu.Lock()
	if s.sgpEpoch != epoch {
		s.sgpMu.Unlock()
		return sgpSession{}, apperrors.New(apperrors.StaleData)
	}
	r := s.sgpClientLocked()
	session := sgpSession{client: r, epoch: s.sgpEpoch, origin: r.origin, platform: r.platform, token: r.token, league: league}
	expires := r.expires
	if league {
		session.token, expires = r.leagueToken, r.leagueExpires
	}
	_, originErr := trustedSGPOrigin(r.platform, r.origin)
	canReuseOrigin := originErr == nil && ((r.token != "" && time.Now().Before(r.expires)) || (r.leagueToken != "" && time.Now().Before(r.leagueExpires)))
	s.sgpMu.Unlock()
	if originErr == nil && session.token != "" && time.Now().Before(expires) {
		return session, nil
	}
	if !canReuseOrigin {
		var err error
		session.origin, session.platform, err = s.fetchSGPOrigin(ctx)
		if err != nil {
			return sgpSession{}, err
		}
	}
	token, err := s.fetchSGPToken(ctx, league)
	if err != nil {
		return sgpSession{}, err
	}
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if ctx.Err() != nil {
		return sgpSession{}, apperrors.Request(ctx, ctx.Err(), apperrors.LocalClient)
	}
	if s.sgpEpoch != session.epoch || s.sgp != r {
		return sgpSession{}, apperrors.New(apperrors.StaleData)
	}
	s.applySGPOriginLocked(r, session.origin, session.platform)
	if league {
		r.leagueToken, r.leagueExpires = token, time.Now().Add(2*time.Minute)
	} else {
		r.token, r.expires = token, time.Now().Add(2*time.Minute)
	}
	session.token = token
	return session, nil
}

func (s *TencentLCU) fetchSGPToken(ctx context.Context, league bool) (string, error) {
	path := "/entitlements/v1/token"
	if league {
		path = "/lol-league-session/v1/league-session-token"
	}
	raw, err := s.client.Get(ctx, path)
	if err != nil {
		return "", err
	}
	var token string
	if league {
		err = json.Unmarshal(raw, &token)
	} else {
		var response struct {
			Access string `json:"accessToken"`
		}
		err = json.Unmarshal(raw, &response)
		token = response.Access
	}
	if err != nil || strings.TrimSpace(token) == "" || len(token) > 32768 || strings.ContainsAny(token, "\r\n") {
		return "", apperrors.New(apperrors.NotLoggedIn)
	}
	return token, nil
}

func (s *TencentLCU) applySGPOriginLocked(r *sgpHistory, origin, platform string) {
	if r.origin != origin || r.platform != platform {
		s.verifiedQueues = nil
		r.token, r.leagueToken = "", ""
		r.expires, r.leagueExpires = time.Time{}, time.Time{}
		r.pages.clear()
		r.heads = nil
	}
	r.origin, r.platform = origin, platform
}

func (s *TencentLCU) sessionValidLocked(session sgpSession) bool {
	return s.sgp == session.client && s.sgpEpoch == session.epoch && session.client.origin == session.origin && session.client.platform == session.platform
}

func (s *TencentLCU) getSGPAuthorized(ctx context.Context, session sgpSession, path, category string, gate *serviceGate) ([]byte, error) {
	check := func() error {
		s.sgpMu.Lock()
		defer s.sgpMu.Unlock()
		if !s.sessionValidLocked(session) {
			return apperrors.New(apperrors.StaleData)
		}
		return nil
	}
	raw, status, err := session.client.getAuthorizedChecked(ctx, session.origin, path, session.token, category, gate, check)
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if ctx.Err() != nil {
		return nil, apperrors.Request(ctx, ctx.Err(), apperrors.DataService)
	}
	if !s.sessionValidLocked(session) {
		return nil, apperrors.New(apperrors.StaleData)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		token := &session.client.token
		if session.league {
			token = &session.client.leagueToken
		}
		if *token == session.token {
			*token = ""
		}
	}
	return raw, err
}
