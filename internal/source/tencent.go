package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

type TencentLCU struct {
	client    *lcu.Client
	selfMu    sync.Mutex
	selfPUUID string
	sgpMu     sync.Mutex
	sgp       *sgpHistory
	queueID   atomic.Int32
	masteryMu sync.Mutex
	masteries map[string]cachedMastery
}

func (*TencentLCU) Region() string { return "CN" }
func (s *TencentLCU) ResetIdentity() {
	s.queueID.Store(0)
	s.selfMu.Lock()
	s.selfPUUID = ""
	s.selfMu.Unlock()
	s.sgpMu.Lock()
	if s.sgp != nil {
		s.sgp.token, s.sgp.origin, s.sgp.platform = "", "", ""
		s.sgp.leagueToken = ""
		s.sgp.expires, s.sgp.leagueExpires = time.Time{}, time.Time{}
		s.sgp.pages = nil
		s.sgp.heads = nil
	}
	s.sgpMu.Unlock()
	s.masteryMu.Lock()
	s.masteries = nil
	s.masteryMu.Unlock()
}

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

func (s *TencentLCU) WatchPhase(ctx context.Context, notify func(string)) error {
	return s.client.WatchPhase(ctx, notify)
}

func (s *TencentLCU) WatchChampSelect(ctx context.Context, notify func()) error {
	return s.client.WatchChampSelect(ctx, notify)
}

func (s *TencentLCU) ChampSelectRoster(ctx context.Context) (int64, []core.Participant, error) {
	raw, err := s.client.Get(ctx, "/lol-champ-select/v1/session")
	if err != nil {
		return 0, nil, err
	}
	return core.ParseChampSelectSession(raw)
}

func (s *TencentLCU) Roster(ctx context.Context) (int64, []core.Participant, error) {
	var gameID int64
	var roster []core.Participant
	var firstErr error
	if raw, err := s.client.Get(ctx, "/lol-gameflow/v1/session"); err == nil {
		s.rememberQueue(raw, false)
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
		if selectRoster, parseErr := core.ParseChampSelectRoster(raw); parseErr == nil && (knownCount(selectRoster) > knownCount(roster) || (knownCount(selectRoster) == knownCount(roster) && len(selectRoster) > len(roster))) {
			return gameID, selectRoster, nil
		}
	}
	if len(roster) > 0 {
		return gameID, roster, nil
	}
	return gameID, roster, firstErr
}

func (s *TencentLCU) Lobby(ctx context.Context) ([]core.Participant, error) {
	raw, err := s.client.Get(ctx, "/lol-lobby/v2/lobby")
	if err != nil {
		return nil, err
	}
	s.rememberQueue(raw, true)
	return core.ParseLobbyRoster(raw)
}

func (s *TencentLCU) CurrentQueue() int { return int(s.queueID.Load()) }
func (s *TencentLCU) rememberQueue(raw []byte, lobby bool) {
	var v struct {
		GameData struct {
			Queue struct {
				ID *int `json:"id"`
			} `json:"queue"`
		} `json:"gameData"`
		GameConfig struct {
			QueueID *int `json:"queueId"`
		} `json:"gameConfig"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	q := v.GameData.Queue.ID
	if lobby {
		q = v.GameConfig.QueueID
	}
	if q != nil && *q >= 0 {
		s.queueID.Store(int32(*q))
	}
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

func (s *TencentLCU) AccountIdentity(ctx context.Context) (string, error) {
	raw, err := s.client.Get(ctx, "/lol-summoner/v1/current-summoner")
	if err != nil {
		return "", err
	}
	var p struct {
		PUUID string `json:"puuid"`
	}
	if json.Unmarshal(raw, &p) != nil || !core.IsKnownPUUID(p.PUUID) {
		return "", errors.New("客户端当前账号身份暂不可核验")
	}
	s.selfMu.Lock()
	s.selfPUUID = p.PUUID
	s.selfMu.Unlock()
	return p.PUUID, nil
}

func (s *TencentLCU) CachedAccountIdentity() string {
	s.selfMu.Lock()
	defer s.selfMu.Unlock()
	return s.selfPUUID
}

func (s *TencentLCU) Name(ctx context.Context, puuid string) (string, error) {
	if !core.IsKnownPUUID(puuid) {
		return "", errors.New("玩家名称不可用")
	}
	raw, err := s.client.Get(ctx, "/lol-summoner/v2/summoners/puuid/"+url.PathEscape(puuid))
	if err != nil {
		return "", err
	}
	var profile struct {
		core.NameFields
		PUUID string `json:"puuid"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", err
	}
	if profile.PUUID != puuid || !core.IsKnownPUUID(puuid) || profile.Name() == "" {
		return "", errors.New("玩家名称不可用")
	}
	return profile.Name(), nil
}

func (s *TencentLCU) History(ctx context.Context, puuid string) (core.HistoryResult, error) {
	// The first recent-history batch stays on LCU. Independent season scouting
	// uses ScoutPage; user-driven browsing uses HistoryPage.
	page, err := s.lcuHistoryPage(ctx, puuid, 0, 20)
	if err == nil {
		s.sgpMu.Lock()
		if s.sgp != nil {
			var head int64
			if len(page.Matches) > 0 {
				head = page.Matches[0].GameID
			}
			s.sgp.rememberHead(puuid, head)
		}
		s.sgpMu.Unlock()
	}
	return core.HistoryResult{Matches: page.Matches, Meta: page.Meta}, err
}

func (s *TencentLCU) HistoryPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	if !core.IsKnownPUUID(puuid) || offset < 0 || offset > 980 {
		return core.HistoryPage{}, errors.New("战绩分页参数无效（最多浏览 1000 条）")
	}
	page, err := s.sgpHistoryPage(ctx, puuid, offset)
	if err == nil {
		return page, nil
	}
	if ctx.Err() != nil {
		return core.HistoryPage{}, errors.New("战绩补取超时或已取消，已有记录保留")
	}
	// Some CN clients ignore begIndex. A single verified prefix is a limited
	// fallback, never evidence that the account only has 50 lifetime games.
	page, fallbackErr := s.lcuHistoryPage(ctx, puuid, offset, 50)
	if fallbackErr != nil {
		return core.HistoryPage{}, errors.New("完整战绩服务暂不可用，客户端也未提供有效后续页；已有记录保留，请稍后重试")
	}
	page.LimitReason = "完整战绩服务暂不可用，当前仅展示客户端最近至多 50 场混合模式记录；不能据此判断该模式无战绩。"
	page.HasMore = false
	return page, nil
}

func (s *TencentLCU) lcuHistoryPage(ctx context.Context, puuid string, offset, count int) (core.HistoryPage, error) {
	if !core.IsKnownPUUID(puuid) || offset < 0 || (count != 20 && count != 50) || offset >= count {
		return core.HistoryPage{}, errors.New("客户端历史参数无效")
	}
	// Prefix retrieval is deliberate: do not trust begIndex on CN clients.
	raw, err := s.client.Get(ctx, fmt.Sprintf("/lol-match-history/v1/products/lol/%s/matches?begIndex=0&endIndex=%d", url.PathEscape(puuid), count-1))
	if err != nil {
		return core.HistoryPage{}, err
	}
	var envelope struct {
		Games struct {
			Begin *int              `json:"gameIndexBegin"`
			Games []json.RawMessage `json:"games"`
		} `json:"games"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Games.Games == nil || (envelope.Games.Begin != nil && *envelope.Games.Begin != 0) {
		return core.HistoryPage{}, errors.New("客户端战绩页范围异常")
	}
	if len(envelope.Games.Games) > count {
		return core.HistoryPage{}, errors.New("客户端有限历史范围不能提供此页")
	}
	if offset > len(envelope.Games.Games) {
		return core.HistoryPage{}, errors.New("客户端未提供所请求的后续记录")
	}
	envelope.Games.Games = envelope.Games.Games[offset:]
	raw, _ = json.Marshal(envelope)
	result, err := core.ParseHistory(raw, puuid)
	if err != nil {
		return core.HistoryPage{}, err
	}
	result.Meta.Scope = fmt.Sprintf("客户端已核验混合模式记录，第 %d 条起", offset+1)
	return core.HistoryPage{Matches: result.Matches, Meta: result.Meta, Offset: offset, NextOffset: offset + result.Meta.Returned, HasMore: result.Meta.Returned >= 20 && offset+result.Meta.Returned <= 980}, nil
}

func (s *TencentLCU) FindByRiotID(ctx context.Context, riotID string) (core.Participant, error) {
	riotID, err := core.NormalizeRiotID(riotID)
	if err != nil {
		return core.Participant{}, err
	}
	name, tag, _ := strings.Cut(riotID, "#")
	// The current CN client exposes a structured alias lookup. One explicit
	// request only: do not retry a timeout through the legacy name endpoint.
	raw, err := s.client.LookupSummonerAlias(ctx, name, tag)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return core.Participant{}, errors.New("玩家身份查询超时：客户端暂未响应，请稍后重试")
		}
		if errors.Is(err, context.Canceled) {
			return core.Participant{}, errors.New("玩家身份查询已取消")
		}
		return core.Participant{}, fmt.Errorf("玩家身份查询失败：%w", err)
	}
	var profiles []struct {
		PUUID    string `json:"puuid"`
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}
	if err := json.Unmarshal(raw, &profiles); err != nil || profiles == nil {
		return core.Participant{}, errors.New("玩家身份查询返回格式异常")
	}
	if len(profiles) == 0 {
		return core.Participant{}, errors.New("当前客户端未找到此 ID 对应的账号，请核对名称、标签和所在大区")
	}
	if len(profiles) != 1 {
		return core.Participant{}, errors.New("无法确认此 ID 对应的账号")
	}
	profile := profiles[0]
	if !core.IsKnownPUUID(profile.PUUID) || !strings.EqualFold(profile.GameName, name) || !strings.EqualFold(profile.TagLine, tag) {
		return core.Participant{}, errors.New("无法确认此 ID 对应的账号")
	}
	return core.Participant{Key: profile.PUUID, PUUID: profile.PUUID,
		DisplayName: profile.GameName + "#" + profile.TagLine, IdentityState: core.IdentityKnown,
		PrivacyState: "unknown", HistoryState: core.HistoryPending}, nil
}

func (s *TencentLCU) MatchDetail(ctx context.Context, gameID int64, target string) (core.MatchDetail, error) {
	if gameID <= 0 || !core.IsKnownPUUID(target) {
		return core.MatchDetail{}, errors.New("对局详情参数无效")
	}
	localCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	raw, err := s.client.Get(localCtx, fmt.Sprintf("/lol-match-history/v1/games/%d", gameID))
	cancel()
	if err == nil {
		return core.ParseMatchDetail(raw, gameID, target)
	}
	if ctx.Err() != nil {
		return core.MatchDetail{}, ctx.Err()
	}
	raw, platform, err := s.sgpGameData(ctx, gameID, "SUMMARY", "/sgp-detail")
	if err != nil {
		return core.MatchDetail{}, fmt.Errorf("本场双方详情暂未取得：%w", err)
	}
	return core.ParseSGPMatchDetail(raw, gameID, target, platform)
}
