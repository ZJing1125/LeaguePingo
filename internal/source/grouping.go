package source

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"net/url"
	"strings"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// One current-game lookup via the verified local account, never arbitrary players.
// A separate ephemeral transport/session avoids holding the history mutex on I/O.
func (s *TencentLCU) Grouping(ctx context.Context, gameID int64) ([]core.Participant, error) {
	self := s.CachedAccountIdentity()
	if !core.IsKnownPUUID(self) || gameID <= 0 {
		return nil, errors.New("当前对局未确认")
	}
	local := (&TencentLCU{client: s.client}).sgpClientLocked()
	return s.requestGrouping(ctx, self, gameID, local)
}

func (s *TencentLCU) requestGrouping(ctx context.Context, self string, gameID int64, local *sgpHistory) ([]core.Participant, error) {
	gate := &s.channels.group
	if err := gate.blocked(time.Now()); err != nil {
		return nil, err
	}
	s.sgpMu.Lock()
	epoch := s.sgpEpoch
	s.sgpMu.Unlock()
	check := func() error {
		s.sgpMu.Lock()
		defer s.sgpMu.Unlock()
		if s.sgpEpoch != epoch {
			return apperrors.New(apperrors.StaleData)
		}
		return nil
	}
	origin, platform, err := s.fetchSGPOrigin(ctx)
	if err != nil {
		return nil, err
	}
	local.origin, local.platform = origin, platform
	tokenRaw, err := s.client.Get(ctx, "/lol-league-session/v1/league-session-token")
	if err != nil {
		return nil, err
	}
	var token string
	if json.Unmarshal(tokenRaw, &token) != nil || token == "" || len(token) > 32768 || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("当前对局授权不可用")
	}
	local.token = token
	raw, _, err := local.getAuthorizedChecked(ctx, local.origin, "/gsm/v1/ledge/region/"+url.PathEscape(local.platform)+"/puuid/"+url.PathEscape(self), token, "/sgp-group", gate, check)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	return parseCurrentGrouping(raw, self, gameID)
}

func parseCurrentGrouping(raw []byte, self string, gameID int64) ([]core.Participant, error) {
	var response struct {
		Game json.RawMessage `json:"game"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Game) == 0 {
		return nil, errors.New("当前对局分组不可用")
	}
	var game map[string]json.RawMessage
	if json.Unmarshal(response.Game, &game) != nil {
		return nil, errors.New("当前对局格式不可用")
	}
	game["gameId"] = game["id"]
	gameRaw, _ := json.Marshal(game)
	wrapped, _ := json.Marshal(map[string]json.RawMessage{"gameData": gameRaw})
	id, players, err := core.ParseGameflowRosterForPlayer(wrapped, self)
	if err != nil || id != gameID {
		return nil, errors.New("返回的对局与当前比赛不一致")
	}
	core.AssignPremades(players, "sgp")
	return players, nil
}
