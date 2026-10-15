package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

type TencentLCU struct{ client *lcu.Client }

func (*TencentLCU) Region() string { return "CN" }

func DiscoverTencent() (Source, error) {
	client, err := lcu.Discover()
	if err != nil {
		return nil, err
	}
	return &TencentLCU{client: client}, nil
}

func (s *TencentLCU) Phase(ctx context.Context) (string, error) {
	raw, err := s.client.Get(ctx, "/lol-gameflow/v1/gameflow-phase")
	if err != nil {
		return "", err
	}
	var phase string
	err = json.Unmarshal(raw, &phase)
	return phase, err
}

func (s *TencentLCU) Roster(ctx context.Context) (int64, []core.Participant, error) {
	var gameID int64
	var roster []core.Participant
	var firstErr error
	if raw, err := s.client.Get(ctx, "/lol-gameflow/v1/session"); err == nil {
		gameID, roster, firstErr = core.ParseGameflowRoster(raw)
	} else {
		firstErr = err
	}
	if raw, err := s.client.Get(ctx, "/lol-champ-select/v1/session"); err == nil {
		if selectRoster, parseErr := core.ParseChampSelectRoster(raw); parseErr == nil && len(selectRoster) > len(roster) {
			return gameID, selectRoster, nil
		}
	}
	if len(roster) > 0 {
		return gameID, roster, nil
	}
	return gameID, roster, firstErr
}

func (s *TencentLCU) Name(ctx context.Context, puuid string) (string, error) {
	raw, err := s.client.Get(ctx, "/lol-summoner/v2/summoners/puuid/"+url.PathEscape(puuid))
	if err != nil {
		return "", err
	}
	var profile struct {
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", err
	}
	if profile.GameName == "" {
		return "", errors.New("玩家名称不可用")
	}
	if profile.TagLine == "" {
		return profile.GameName, nil
	}
	return profile.GameName + "#" + profile.TagLine, nil
}

func (s *TencentLCU) History(ctx context.Context, puuid string) ([]core.Match, error) {
	raw, err := s.client.Get(ctx, lcu.HistoryPath(puuid, 20))
	if err != nil {
		return nil, err
	}
	return core.ParseHistory(raw, puuid)
}

func (s *TencentLCU) FindByRiotID(ctx context.Context, riotID string) (core.Participant, error) {
	parts := strings.SplitN(strings.TrimSpace(riotID), "#", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return core.Participant{}, errors.New("请输入完整 ID，例如 玩家名#标签")
	}
	path := "/lol-summoner/v1/summoners?name=" + url.QueryEscape(parts[0]) + "&tagLine=" + url.QueryEscape(parts[1])
	raw, err := s.client.Get(ctx, path)
	if err != nil {
		return core.Participant{}, err
	}
	var profile struct {
		PUUID    string `json:"puuid"`
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return core.Participant{}, err
	}
	if !core.IsKnownPUUID(profile.PUUID) || !strings.EqualFold(profile.GameName, parts[0]) || !strings.EqualFold(profile.TagLine, parts[1]) {
		return core.Participant{}, errors.New("无法确认此 ID 对应的账号")
	}
	return core.Participant{Key: profile.PUUID, PUUID: profile.PUUID,
		DisplayName: profile.GameName + "#" + profile.TagLine, IdentityState: core.IdentityKnown,
		PrivacyState: "unknown", HistoryState: core.HistoryPending}, nil
}
