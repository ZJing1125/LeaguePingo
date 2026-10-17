package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

type TencentLCU struct {
	client    *lcu.Client
	selfMu    sync.Mutex
	selfPUUID string
}

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
		self, selfErr := s.currentPUUID(ctx)
		if selfErr == nil {
			gameID, roster, firstErr = core.ParseGameflowRosterForPlayer(raw, self)
		} else {
			firstErr = selfErr
		}
	} else {
		firstErr = err
	}
	if len(roster) >= 10 && knownCount(roster) == len(roster) {
		return gameID, roster, nil
	}
	if raw, err := s.client.Get(ctx, "/lol-champ-select/v1/session"); err == nil {
		if selectRoster, parseErr := core.ParseChampSelectRoster(raw); parseErr == nil && (len(selectRoster) > len(roster) || knownCount(selectRoster) > knownCount(roster)) {
			return gameID, selectRoster, nil
		}
	}
	if len(roster) > 0 {
		return gameID, roster, nil
	}
	return gameID, roster, firstErr
}

func knownCount(players []core.Participant) int {
	n := 0
	for _, p := range players {
		if core.IsKnownPUUID(p.PUUID) {
			n++
		}
	}
	return n
}

func (s *TencentLCU) currentPUUID(ctx context.Context) (string, error) {
	s.selfMu.Lock()
	defer s.selfMu.Unlock()
	if s.selfPUUID != "" {
		return s.selfPUUID, nil
	}
	raw, err := s.client.Get(ctx, "/lol-summoner/v1/current-summoner")
	if err != nil {
		return "", err
	}
	var profile struct {
		PUUID string `json:"puuid"`
	}
	if json.Unmarshal(raw, &profile) != nil || !core.IsKnownPUUID(profile.PUUID) {
		return "", errors.New("无法确认当前账号的阵营")
	}
	s.selfPUUID = profile.PUUID
	return profile.PUUID, nil
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
	parts[0], parts[1] = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	// The running CN client's /help contract accepts only `name` here.
	// Pass the complete Riot ID in that one parameter; # must be query-escaped.
	path := "/lol-summoner/v1/summoners?" + url.Values{"name": {parts[0] + "#" + parts[1]}}.Encode()
	raw, err := s.client.Get(ctx, path)
	if err != nil {
		return core.Participant{}, fmt.Errorf("玩家身份查询失败（请确认完整 ID 和当前登录大区）：%w", err)
	}
	var profile struct {
		PUUID    string `json:"puuid"`
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return core.Participant{}, errors.New("玩家身份查询返回格式异常")
	}
	if !core.IsKnownPUUID(profile.PUUID) || !strings.EqualFold(profile.GameName, parts[0]) || !strings.EqualFold(profile.TagLine, parts[1]) {
		return core.Participant{}, errors.New("无法确认此 ID 对应的账号")
	}
	return core.Participant{Key: profile.PUUID, PUUID: profile.PUUID,
		DisplayName: profile.GameName + "#" + profile.TagLine, IdentityState: core.IdentityKnown,
		PrivacyState: "unknown", HistoryState: core.HistoryPending}, nil
}
