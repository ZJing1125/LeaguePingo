package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

type Client = source.Source

type Discoverer func() (Client, error)

type Engine struct {
	mu                  sync.RWMutex
	discover            Discoverer
	client              Client
	snapshot            core.Snapshot
	cancel              context.CancelFunc
	onChange            func(core.Snapshot)
	updates             chan core.Snapshot
	lastLookup          time.Time
	lookupGate          chan struct{}
	matchRosterAttempts int
	matchRosterComplete bool
}

func New(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	e := &Engine{discover: discover, onChange: onChange, lookupGate: make(chan struct{}, 1), snapshot: core.Snapshot{Stage: "disconnected", UpdatedAt: time.Now(), Participants: []core.Participant{}}}
	if onChange != nil {
		e.updates = make(chan core.Snapshot, 32)
		go func() {
			for snapshot := range e.updates {
				onChange(snapshot)
			}
		}()
	}
	return e
}

func (e *Engine) Snapshot() core.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return clone(e.snapshot)
}

func (e *Engine) Run(ctx context.Context) {
	e.poll(ctx)
	timer := time.NewTimer(e.pollInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			e.mu.Lock()
			if e.cancel != nil {
				e.cancel()
			}
			e.mu.Unlock()
			return
		case <-timer.C:
			e.poll(ctx)
			timer.Reset(e.pollInterval())
		}
	}
}

func (e *Engine) pollInterval() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	switch e.snapshot.Stage {
	case "preparing":
		return 3 * time.Second
	case "in-game":
		return 20 * time.Second
	case "idle":
		return 6 * time.Second
	default:
		return 10 * time.Second
	}
}

func (e *Engine) poll(ctx context.Context) {
	e.mu.Lock()
	client := e.client
	if client == nil && time.Since(e.lastLookup) >= 10*time.Second {
		e.lastLookup = time.Now()
		e.mu.Unlock()
		found, err := e.discover()
		e.mu.Lock()
		if err == nil && found != nil {
			e.client = found
			client = found
		} else {
			message := "无法获取客户端连接信息"
			if err != nil {
				message = err.Error()
			}
			e.disconnectLocked(message)
		}
	}
	generation := e.snapshot.Generation
	e.mu.Unlock()
	if client == nil {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	phase, err := client.Phase(requestCtx)
	if err != nil {
		e.mu.Lock()
		if e.snapshot.Generation == generation {
			e.disconnectLocked("客户端本地接口连接失败：" + err.Error())
		}
		e.mu.Unlock()
		return
	}
	stage := stageForPhase(phase)
	var gameID int64
	var roster []core.Participant
	e.mu.Lock()
	if e.snapshot.Generation != generation {
		e.mu.Unlock()
		return
	}
	refresh := false
	if activeMatch(phase) {
		entering := !activeMatch(e.snapshot.Phase) || (phase == "GameStart" && e.snapshot.Phase != "GameStart")
		if entering {
			e.matchRosterAttempts = 0
			e.matchRosterComplete = false
		}
		if e.cancel != nil {
			e.cancel()
		}
		refresh = e.matchRosterAttempts < 3 && !e.matchRosterComplete
		if refresh {
			e.matchRosterAttempts++
		}
	} else if stage != "in-game" {
		e.matchRosterAttempts = 0
		e.matchRosterComplete = false
	}
	e.mu.Unlock()
	var rosterErr error
	if stage == "preparing" || refresh {
		gameID, roster, rosterErr = client.Roster(requestCtx)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot.Generation != generation {
		return
	}
	if stage == "in-game" {
		// Refresh only local identities at match entry (at most three attempts).
		// Historical data is fetched only on an explicit user action in-game.
		if refresh && rosterErr == nil && gameID != 0 && completeRoster(roster) {
			e.matchRosterComplete = true
		}
		if !refresh || rosterErr != nil || len(roster) == 0 {
			gameID = e.snapshot.GameID
			roster = clone(e.snapshot).Participants
		} else if gameID == e.snapshot.GameID && !rosterImproves(e.snapshot.Participants, roster) {
			roster = clone(e.snapshot).Participants
		}
		if gameID == 0 {
			gameID = e.snapshot.GameID
		}
	}
	if stage == "preparing" && len(roster) == 0 && e.snapshot.Stage == "preparing" {
		// A transient empty session must not discard prepared data.
		roster = clone(e.snapshot).Participants
		if gameID == 0 {
			gameID = e.snapshot.GameID
		}
	}
	if e.snapshot.Phase != phase || e.snapshot.GameID != gameID || !e.snapshot.Connected {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if (stage == "preparing" || stage == "in-game") && rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if e.snapshot.Stage == "disconnected" {
		e.replaceLocked(stage, phase, gameID, roster, "")
	}
}

// Keep the frozen match available while exposing why reconnection failed.
func (e *Engine) disconnectLocked(message string) {
	e.client = nil
	if e.snapshot.Frozen {
		message += "；仍可查看已缓存的对局资料"
	}
	if !e.snapshot.Connected && e.snapshot.Message == message {
		return
	}
	if e.snapshot.Frozen {
		e.snapshot.Connected = false
		e.snapshot.Message = message
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	} else {
		e.replaceLocked("disconnected", "", 0, nil, message)
	}
}

func (e *Engine) replaceLocked(stage, phase string, gameID int64, roster []core.Participant, message string) {
	if e.cancel != nil {
		e.cancel()
	}
	e.snapshot.Generation++
	generation := e.snapshot.Generation
	if roster == nil {
		roster = []core.Participant{}
	}
	if stage == "in-game" && message == "" {
		if len(roster) == 0 {
			message = "客户端已连接，当前尚未取得对局名单；可刷新名单或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else if !completeRoster(roster) {
			message = "当前名单仍有身份未提供；可刷新本地名单，或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else {
			message = "已取得本地对局名单，保留赛前战绩；缺少资料的玩家可点击手动查询。局内暂停自动战绩预取。"
		}
	}
	if (stage == "preparing" || stage == "in-game") && (gameID == 0 || e.snapshot.GameID == 0 || gameID == e.snapshot.GameID) {
		for i := range roster {
			for _, old := range e.snapshot.Participants {
				if old.PUUID != "" && old.PUUID == roster[i].PUUID {
					roster[i].History = append([]core.Match(nil), old.History...)
					roster[i].Stats = old.Stats
					roster[i].HistoryState = old.HistoryState
					if old.DisplayName != "" && old.DisplayName != "玩家资料加载中" {
						roster[i].DisplayName = old.DisplayName
					}
					roster[i].Error = old.Error
				}
			}
		}
	}
	if stage == "in-game" {
		for i := range roster {
			if roster[i].HistoryState == core.HistoryLoading {
				roster[i].HistoryState = core.HistoryPending
				roster[i].Error = "赛前查询未完成，可手动查询"
			}
		}
	}
	region := ""
	if e.client != nil {
		region = e.client.Region()
	}
	e.snapshot = core.Snapshot{Generation: generation, Phase: phase, Stage: stage,
		Connected: stage != "disconnected", Region: region, GameID: gameID, Frozen: stage == "in-game",
		UpdatedAt: time.Now(), Participants: roster, Message: message}
	e.publishLocked()
	if stage == "preparing" && e.client != nil {
		var queryCtx context.Context
		queryCtx, e.cancel = context.WithCancel(context.Background())
		go e.prepare(queryCtx, generation, e.client, clone(e.snapshot).Participants)
	} else {
		e.cancel = nil
	}
}

func (e *Engine) prepare(ctx context.Context, generation uint64, client Client, players []core.Participant) {
	// One request stream limits pressure on the game network and prevents bursts.
	for _, player := range players {
		if !core.IsKnownPUUID(player.PUUID) || ctx.Err() != nil {
			continue
		}
		if player.HistoryState == core.HistoryReady || player.HistoryState == core.HistoryEmpty {
			continue
		}
		e.updatePlayer(generation, player.Key, func(p *core.Participant) { p.HistoryState = core.HistoryLoading })
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		name, profileErr := client.Name(requestCtx, player.PUUID)
		cancel()
		if profileErr == nil && name != "" {
			e.updatePlayer(generation, player.Key, func(p *core.Participant) { p.DisplayName = name })
		}
		// Polling cancels this worker on transition; this extra check closes most
		// of the interval between two phase polls before a history request.
		phaseCtx, phaseCancel := context.WithTimeout(ctx, 2*time.Second)
		phase, phaseErr := client.Phase(phaseCtx)
		phaseCancel()
		if phaseErr != nil {
			e.updatePlayer(generation, player.Key, func(p *core.Participant) {
				p.HistoryState = core.HistoryFailed
				p.Error = "无法确认当前对局阶段"
			})
			return
		}
		if stageForPhase(phase) == "in-game" || ctx.Err() != nil {
			e.updatePlayer(generation, player.Key, func(p *core.Participant) {
				p.HistoryState = core.HistoryPending
				p.Error = "对局已开始，未继续查询"
			})
			return
		}
		requestCtx, cancel = context.WithTimeout(ctx, 6*time.Second)
		matches, err := client.History(requestCtx, player.PUUID)
		cancel()
		if err != nil {
			e.updatePlayer(generation, player.Key, func(p *core.Participant) {
				p.HistoryState = core.HistoryFailed
				p.Error = err.Error()
			})
			continue
		}
		e.updatePlayer(generation, player.Key, func(p *core.Participant) {
			if err != nil {
				p.HistoryState, p.Error = core.HistoryFailed, err.Error()
				return
			}
			p.History = matches
			p.Stats = core.CalculateStats(matches)
			if len(matches) == 0 {
				p.HistoryState = core.HistoryEmpty
			} else {
				p.HistoryState = core.HistoryReady
			}
		})
	}
}

func (e *Engine) updatePlayer(generation uint64, key string, edit func(*core.Participant)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot.Generation != generation || e.snapshot.Frozen {
		return
	}
	for i := range e.snapshot.Participants {
		if e.snapshot.Participants[i].Key == key {
			edit(&e.snapshot.Participants[i])
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
			return
		}
	}
}

func (e *Engine) publishLocked() {
	if e.onChange == nil {
		return
	}
	snapshot := clone(e.snapshot)
	select {
	case e.updates <- snapshot:
	default:
		// A slow UI consumer receives the latest state without stalling LCU polling.
		select {
		case <-e.updates:
		default:
		}
		e.updates <- snapshot
	}
}

func (e *Engine) Lookup(ctx context.Context, riotID string) (core.Participant, error) {
	ctx, release, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	e.mu.RLock()
	client := e.client
	generation := e.snapshot.Generation
	cached := clone(e.snapshot)
	e.mu.RUnlock()
	for _, p := range cached.Participants {
		if strings.EqualFold(p.DisplayName, strings.TrimSpace(riotID)) && historyCached(p) {
			return p, nil
		}
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	player, err := client.FindByRiotID(ctx, riotID)
	if err != nil {
		return core.Participant{}, err
	}
	return e.loadManualHistory(ctx, client, generation, player)
}

func (e *Engine) LookupPlayer(ctx context.Context, key string) (core.Participant, error) {
	ctx, release, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	snapshot := e.Snapshot()
	var player core.Participant
	for _, p := range snapshot.Participants {
		if p.Key == key {
			player = p
			break
		}
	}
	if !core.IsKnownPUUID(player.PUUID) {
		return core.Participant{}, errors.New("客户端尚未提供该玩家身份，可刷新名单或输入完整 ID 查询")
	}
	if historyCached(player) {
		return player, nil
	}
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	if name, err := client.Name(ctx, player.PUUID); err == nil && name != "" {
		player.DisplayName = name
	}
	return e.loadManualHistory(ctx, client, snapshot.Generation, player)
}

func (e *Engine) beginLookup(ctx context.Context) (context.Context, func(), error) {
	select {
	case e.lookupGate <- struct{}{}:
		bounded, cancel := context.WithTimeout(ctx, 12*time.Second)
		return bounded, func() { cancel(); <-e.lookupGate }, nil
	default:
		return nil, nil, errors.New("已有查询正在进行，请等待完成后再试")
	}
}

func (e *Engine) loadManualHistory(ctx context.Context, client Client, generation uint64, player core.Participant) (core.Participant, error) {
	player.HistoryState = core.HistoryLoading
	var err error
	player.History, err = client.History(ctx, player.PUUID)
	if err != nil {
		return core.Participant{}, fmt.Errorf("已找到玩家，但战绩请求失败: %w", err)
	}
	player.Stats = core.CalculateStats(player.History)
	if len(player.History) == 0 {
		player.HistoryState = core.HistoryEmpty
	} else {
		player.HistoryState = core.HistoryReady
	}
	e.mu.Lock()
	if e.snapshot.Generation == generation {
		for i, p := range e.snapshot.Participants {
			if p.PUUID == player.PUUID {
				e.snapshot.Participants[i].DisplayName = player.DisplayName
				e.snapshot.Participants[i].History = append([]core.Match(nil), player.History...)
				e.snapshot.Participants[i].Stats = player.Stats
				e.snapshot.Participants[i].HistoryState = player.HistoryState
				e.snapshot.Participants[i].Error = ""
			}
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
	e.mu.Unlock()
	return player, nil
}

func (e *Engine) RefreshRoster(ctx context.Context) (core.Snapshot, error) {
	ctx, release, err := e.beginLookup(ctx)
	if err != nil {
		return e.Snapshot(), err
	}
	defer release()
	e.mu.RLock()
	client := e.client
	generation := e.snapshot.Generation
	e.mu.RUnlock()
	if client == nil {
		return e.Snapshot(), source.ErrNotConnected
	}
	phase, err := client.Phase(ctx)
	if err != nil {
		return e.Snapshot(), err
	}
	stage := stageForPhase(phase)
	if stage != "preparing" && !activeMatch(phase) {
		return e.Snapshot(), errors.New("当前不在选人或进行中的对局，请等待进入对局后重试")
	}
	gameID, roster, err := client.Roster(ctx)
	if err != nil {
		return e.Snapshot(), fmt.Errorf("本地名单刷新失败：%w", err)
	}
	if len(roster) == 0 {
		return e.Snapshot(), errors.New("客户端暂未提供名单，请稍后再试")
	}
	e.mu.Lock()
	if generation != e.snapshot.Generation {
		e.mu.Unlock()
		return e.Snapshot(), errors.New("对局状态已变化，请重试刷新名单")
	}
	if gameID == e.snapshot.GameID && !rosterImproves(e.snapshot.Participants, roster) {
		roster = clone(e.snapshot).Participants
	}
	if activeMatch(phase) && (!activeMatch(e.snapshot.Phase) || (phase == "GameStart" && e.snapshot.Phase != "GameStart") || (gameID != 0 && gameID != e.snapshot.GameID)) {
		e.matchRosterAttempts = 0
		e.matchRosterComplete = false
	}
	e.replaceLocked(stage, phase, gameID, roster, "")
	if activeMatch(phase) && gameID != 0 && completeRoster(roster) {
		e.matchRosterComplete = true
	}
	e.mu.Unlock()
	return e.Snapshot(), nil
}

func historyCached(p core.Participant) bool {
	return p.HistoryState == core.HistoryReady || p.HistoryState == core.HistoryEmpty
}
func completeRoster(players []core.Participant) bool {
	if len(players) < 10 {
		return false
	}
	for _, p := range players {
		if !core.IsKnownPUUID(p.PUUID) {
			return false
		}
	}
	return true
}
func rosterImproves(old, next []core.Participant) bool {
	known := func(players []core.Participant) int {
		n := 0
		for _, p := range players {
			if core.IsKnownPUUID(p.PUUID) {
				n++
			}
		}
		return n
	}
	return len(next) >= len(old) && known(next) >= known(old)
}
func activeMatch(phase string) bool {
	return phase == "GameStart" || phase == "InProgress" || phase == "Reconnect"
}

func stageForPhase(phase string) string {
	switch phase {
	case "ChampSelect":
		return "preparing"
	case "GameStart", "InProgress", "Reconnect", "WatchInProgress", "WaitingForStats", "PreEndOfGame", "EndOfGame":
		return "in-game"
	default:
		return "idle"
	}
}

func rosterChanged(old, current []core.Participant) bool {
	if len(old) != len(current) {
		return true
	}
	for i := range old {
		if old[i].Key != current[i].Key || old[i].ChampionID != current[i].ChampionID || old[i].Team != current[i].Team {
			return true
		}
	}
	return false
}

func clone(in core.Snapshot) core.Snapshot {
	out := in
	out.Participants = make([]core.Participant, len(in.Participants))
	copy(out.Participants, in.Participants)
	for i := range out.Participants {
		out.Participants[i].History = append([]core.Match(nil), in.Participants[i].History...)
	}
	return out
}
