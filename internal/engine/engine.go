package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

type Client = source.Source

type Discoverer func() (Client, error)

const inGameRosterAttemptLimit = 3

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
	cache               map[string]core.Participant
	aliases             map[string]string
	profileRetry        map[string]time.Time
	clientVersion       uint64
	accountIdentity     string
	phaseEpoch          uint64
	phaseHint           string
	phaseWake           chan struct{}
	phaseWatchCancel    context.CancelFunc
	phaseWatchVersion   uint64
	phaseWatchSeq       uint64
	phaseWatchRetry     time.Time
	champWake           chan struct{}
	champWatchCancel    context.CancelFunc
	champWatchVersion   uint64
	champWatchSeq       uint64
	champEpoch          uint64
	champWatchRetry     time.Time
	scheduler           *requestScheduler
	prepareToken        uint64
	prepareRunning      bool
	prepareDeadline     time.Time
	prepareRetryAt      time.Time
	prepareAttempts     map[string]int
	preferred           string
	details             map[string]core.MatchDetail
}

func New(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	e := &Engine{discover: discover, onChange: onChange, lookupGate: make(chan struct{}, 1), phaseWake: make(chan struct{}, 1), champWake: make(chan struct{}, 1), scheduler: newScheduler(), cache: map[string]core.Participant{}, aliases: map[string]string{}, snapshot: core.Snapshot{Stage: "disconnected", UpdatedAt: time.Now(), Participants: []core.Participant{}}}
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
	e.syncPhaseEvents(ctx)
	e.syncChampSelectEvents(ctx)
	timer := time.NewTimer(e.pollInterval())
	defer timer.Stop()
	var champTimer *time.Timer
	var champTick <-chan time.Time
	defer func() {
		if champTimer != nil {
			champTimer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			e.mu.Lock()
			if e.cancel != nil {
				e.cancel()
			}
			e.stopPhaseEventsLocked()
			e.stopChampSelectEventsLocked()
			e.mu.Unlock()
			return
		case <-timer.C:
			e.poll(ctx)
			e.syncPhaseEvents(ctx)
			e.syncChampSelectEvents(ctx)
			timer.Reset(e.pollInterval())
		case <-e.phaseWake:
			e.poll(ctx)
			e.syncPhaseEvents(ctx)
			e.syncChampSelectEvents(ctx)
			timer.Reset(e.pollInterval())
		case <-e.champWake:
			if champTick == nil {
				champTimer = time.NewTimer(250 * time.Millisecond)
				champTick = champTimer.C
			}
		case <-champTick:
			champTick = nil
			e.refreshChampSelect(ctx)
			e.syncChampSelectEvents(ctx)
		}
	}
}

func (e *Engine) pollInterval() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	switch e.snapshot.Stage {
	case "preparing":
		return 3 * time.Second
	case "party":
		return 6 * time.Second
	case "in-game":
		if e.client != nil && activeMatch(e.snapshot.Phase) && !e.matchRosterComplete && e.matchRosterAttempts < inGameRosterAttemptLimit {
			return 2 * time.Second
		}
		if e.client != nil && e.snapshot.Phase == "GameStart" && !e.matchRosterComplete {
			return 5 * time.Second // Phase only after the loading-screen roster budget ends.
		}
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
			e.invalidateConnectionLocked()
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
	version := e.clientVersion
	epoch := e.phaseEpoch
	champEpoch := e.champEpoch
	e.mu.Unlock()
	if client == nil {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	phase, err := client.Phase(requestCtx)
	if err != nil {
		e.mu.Lock()
		if e.snapshot.Generation == generation && e.clientVersion == version && e.phaseEpoch == epoch {
			e.disconnectLocked("客户端本地接口连接失败：" + err.Error())
		}
		e.mu.Unlock()
		return
	}
	stage := stageForPhase(phase)
	account := ""
	if preparationStage(stage) {
		if accounts, ok := client.(source.AccountSource); ok {
			// Piggyback on low-frequency pregame polls; no account or history
			// polling is added to active gameplay.
			account, _ = accounts.AccountIdentity(requestCtx)
		}
	}
	var gameID int64
	var roster []core.Participant
	e.mu.Lock()
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		e.mu.Unlock()
		return
	}
	e.phaseHint = "" // This phase response postdates the last observed notification.
	if phase == "None" && e.snapshot.Phase != "None" {
		e.invalidateConnectionLocked()
		version = e.clientVersion
		if resetter, ok := client.(interface{ ResetIdentity() }); ok {
			resetter.ResetIdentity()
		}
	}
	if account != "" {
		if e.accountIdentity != "" && account != e.accountIdentity {
			e.invalidateConnectionLocked()
			version = e.clientVersion
		}
		e.accountIdentity = account
	}
	refresh := false
	if activeMatch(phase) {
		entering := !activeMatch(e.snapshot.Phase) || (phase == "GameStart" && e.snapshot.Phase != "GameStart")
		if entering {
			e.matchRosterAttempts = 0
			e.matchRosterComplete = false
		}
		// GameStart and InProgress are distinct client readiness windows. If
		// loading ended before gameflow exposed the full roster, grant one new
		// bounded local-identity refresh budget at the transition.
		if phase == "InProgress" && e.snapshot.Phase == "GameStart" && !e.matchRosterComplete {
			e.matchRosterAttempts = 0
			e.matchRosterComplete = false
		}
		if e.cancel != nil {
			e.cancel()
		}
		refresh = e.matchRosterAttempts < inGameRosterAttemptLimit && !e.matchRosterComplete
		if refresh {
			e.matchRosterAttempts++
		}
	} else if stage != "in-game" {
		e.matchRosterAttempts = 0
		e.matchRosterComplete = false
	}
	e.mu.Unlock()
	var rosterErr error
	if preparationStage(stage) || refresh {
		if stage == "party" {
			if lobby, ok := client.(source.LobbySource); ok {
				roster, rosterErr = lobby.Lobby(requestCtx)
			}
		} else {
			gameID, roster, rosterErr = readRoster(requestCtx, client, stage)
		}
		for i := range roster {
			roster[i].ConnectionID = version
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		return
	}
	if stage == "preparing" && e.champEpoch != champEpoch {
		return
	}
	if stage == "preparing" && e.snapshot.Stage == stage && e.snapshot.ConnectionID == version && gameID == 0 {
		gameID = e.snapshot.GameID
	}
	if stage == "in-game" {
		// Refresh only local identities at match entry (three attempts per
		// readiness phase; GameStart -> InProgress can renew once).
		// Historical data is fetched only on an explicit user action in-game.
		if refresh && rosterErr == nil && gameID != 0 && completeRoster(roster) {
			e.matchRosterComplete = true
		}
		if !refresh || rosterErr != nil || len(roster) == 0 {
			gameID = e.snapshot.GameID
			roster = clone(e.snapshot).Participants
		} else if e.snapshot.ConnectionID == version && gameID == e.snapshot.GameID && !rosterImproves(e.snapshot.Participants, roster) {
			roster = clone(e.snapshot).Participants
		}
		if gameID == 0 {
			gameID = e.snapshot.GameID
		}
	}
	if preparationStage(stage) && len(roster) == 0 && e.snapshot.Stage == stage && e.snapshot.ConnectionID == version {
		// A transient empty session must not discard prepared data.
		roster = clone(e.snapshot).Participants
		if gameID == 0 {
			gameID = e.snapshot.GameID
		}
	}
	if e.snapshot.ConnectionID != version || e.snapshot.Phase != phase || e.snapshot.GameID != gameID || !e.snapshot.Connected {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if (preparationStage(stage) || stage == "in-game") && rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if e.snapshot.Stage == "disconnected" {
		e.replaceLocked(stage, phase, gameID, roster, "")
	}
	if preparationStage(stage) || refresh {
		message := ""
		if rosterErr != nil {
			message = "本地名单暂时读取失败，保留已有资料；可稍后手动刷新"
		}
		if e.snapshot.RosterError != message {
			e.snapshot.RosterError = message
			e.publishLocked()
		}
	}
	e.startPreparationLocked()
}

// Retained data can still be displayed, but can never become a new connection's cache.
func (e *Engine) invalidateConnectionLocked() {
	e.stopPhaseEventsLocked()
	e.stopChampSelectEventsLocked()
	e.champWatchRetry = time.Time{}
	e.phaseHint = ""
	e.clientVersion++
	e.accountIdentity = ""
	e.matchRosterAttempts = 0
	e.matchRosterComplete = false
	e.cache = map[string]core.Participant{}
	e.aliases = map[string]string{}
	e.profileRetry = nil
	e.details = nil
	if e.cancel != nil {
		e.cancel()
	}
	e.cancel = nil
	e.prepareToken++
	e.prepareRunning = false
	e.prepareDeadline = time.Time{}
	e.prepareRetryAt = time.Time{}
	e.prepareAttempts = nil
	for i := range e.snapshot.Participants {
		e.snapshot.Participants[i].Stale = true
	}
}

// Keep the frozen match available while exposing why reconnection failed.
func (e *Engine) disconnectLocked(message string) {
	e.stopPhaseEventsLocked()
	e.stopChampSelectEventsLocked()
	e.client = nil
	if e.cancel != nil {
		e.cancel()
	}
	for i := range e.snapshot.Participants {
		e.snapshot.Participants[i].Stale = true
	}
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
	if stage != "preparing" {
		e.stopChampSelectEventsLocked()
	}
	sameConnection := e.snapshot.ConnectionID == e.clientVersion
	samePreparation := sameConnection && preparationStage(stage) && e.snapshot.Stage == stage && (gameID == 0 || e.snapshot.GameID == 0 || gameID == e.snapshot.GameID)
	if !samePreparation {
		if e.cancel != nil {
			e.cancel()
		}
		e.prepareToken++
		e.prepareRunning = false
		e.prepareDeadline = time.Time{}
		e.prepareRetryAt = time.Time{}
		e.prepareAttempts = map[string]int{}
	}
	if samePreparation && stage == "party" {
		for _, p := range roster {
			found := false
			for _, old := range e.snapshot.Participants {
				if old.PUUID == p.PUUID {
					found = true
					break
				}
			}
			if !found && core.IsKnownPUUID(p.PUUID) {
				e.prepareDeadline = time.Now().Add(45 * time.Second)
				break
			}
		}
	}
	e.snapshot.Generation++
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
	if sameConnection && (preparationStage(stage) || stage == "in-game") && (gameID == 0 || e.snapshot.GameID == 0 || gameID == e.snapshot.GameID) {
		for i := range roster {
			for _, old := range e.snapshot.Participants {
				if old.ConnectionID == e.clientVersion && old.PUUID != "" && old.PUUID == roster[i].PUUID {
					roster[i].History = append([]core.Match(nil), old.History...)
					roster[i].HistoryMeta = old.HistoryMeta
					roster[i].Stats = old.Stats
					roster[i].Profile = old.Profile
					roster[i].PrivacyState = old.PrivacyState
					roster[i].Cached = old.Cached
					roster[i].Stale = old.Stale
					roster[i].HistoryState = old.HistoryState
					if core.IsPlaceholderName(roster[i].DisplayName) && !core.IsPlaceholderName(old.DisplayName) {
						roster[i].DisplayName = old.DisplayName
					}
					roster[i].Error = old.Error
				}
			}
		}
	}
	// Names belong to a verified connection/player, independently of the match
	// or history timestamp. Never let a nameless roster overwrite the cache.
	for i := range roster {
		if cached, ok := e.cache[roster[i].PUUID]; ok && cached.ConnectionID == e.clientVersion && core.IsPlaceholderName(roster[i].DisplayName) && !core.IsPlaceholderName(cached.DisplayName) {
			roster[i].DisplayName = cached.DisplayName
		}
	}
	if stage == "in-game" || !samePreparation {
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
	e.snapshot = core.Snapshot{ConnectionID: e.clientVersion, Generation: e.snapshot.Generation, Phase: phase, Stage: stage,
		Connected: stage != "disconnected", Region: region, GameID: gameID, Frozen: stage == "in-game",
		UpdatedAt: time.Now(), Participants: roster, Message: message}
	e.publishLocked()
	if !preparationStage(stage) {
		e.cancel = nil
	}
	e.startPreparationLocked()
}

func (e *Engine) startPreparationLocked() {
	if !preparationStage(e.snapshot.Stage) || e.client == nil || e.prepareRunning || time.Now().Before(e.prepareRetryAt) || (e.phaseHint != "" && stageForPhase(e.phaseHint) != e.snapshot.Stage) {
		return
	}
	if e.prepareDeadline.IsZero() {
		e.prepareDeadline = time.Now().Add(45 * time.Second)
		e.prepareAttempts = map[string]int{}
	}
	if !time.Now().Before(e.prepareDeadline) {
		return
	}
	for _, p := range e.snapshot.Participants {
		if p.ConnectionID == e.clientVersion && core.IsKnownPUUID(p.PUUID) && !(historyCached(p) && cacheFresh(p) && !p.Stale) && e.prepareAttempts[p.PUUID] < 2 {
			ctx, cancel := context.WithDeadline(context.Background(), e.prepareDeadline)
			e.cancel, e.prepareRunning = cancel, true
			go func(token, version uint64, client Client) {
				defer cancel()
				e.prepare(ctx, token, client, version)
			}(e.prepareToken, e.clientVersion, e.client)
			return
		}
	}
}

func (e *Engine) prepare(ctx context.Context, token uint64, client Client, version uint64) {
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.prepareToken == token {
			e.prepareRunning = false
			changed := false
			for i := range e.snapshot.Participants {
				p := &e.snapshot.Participants[i]
				if p.HistoryState == core.HistoryLoading {
					p.HistoryState = core.HistoryPending
					p.Error = "准备已暂停，可主动查询"
					changed = true
				}
			}
			if changed {
				e.snapshot.UpdatedAt = time.Now()
				e.publishLocked()
			}
		}
	}()
	for ctx.Err() == nil {
		e.mu.RLock()
		if e.prepareToken != token || e.clientVersion != version || !preparationStage(e.snapshot.Stage) {
			e.mu.RUnlock()
			return
		}
		players := clone(e.snapshot).Participants
		preparingStage := e.snapshot.Stage
		preferred := e.preferred
		sort.SliceStable(players, func(i, j int) bool {
			a, b := e.prepareAttempts[players[i].PUUID], e.prepareAttempts[players[j].PUUID]
			if a != b {
				return a < b
			}
			return players[i].Key == preferred && players[j].Key != preferred
		})
		var player core.Participant
		for _, p := range players {
			if p.ConnectionID == version && core.IsKnownPUUID(p.PUUID) && !(historyCached(p) && cacheFresh(p) && !p.Stale) && e.prepareAttempts[p.PUUID] < 2 {
				player = p
				break
			}
		}
		e.mu.RUnlock()
		if player.PUUID == "" {
			return
		}
		e.mu.Lock()
		if e.prepareToken != token {
			e.mu.Unlock()
			return
		}
		e.prepareAttempts[player.PUUID]++
		e.mu.Unlock()
		e.mu.RLock()
		cached, ok := e.cache[player.PUUID]
		e.mu.RUnlock()
		if ok && cacheFresh(cached) {
			e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
				if core.IsPlaceholderName(p.DisplayName) && !core.IsPlaceholderName(cached.DisplayName) {
					p.DisplayName = cached.DisplayName
				}
				core.ApplyHistory(p, core.HistoryResult{Matches: cached.History, Meta: cached.HistoryMeta})
			})
			continue
		}
		e.updatePrepared(token, player.PUUID, func(p *core.Participant) { p.HistoryState = core.HistoryLoading; p.Error = "" })
		if !strings.Contains(player.DisplayName, "#") {
			nameCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			name, err := e.loadName(nameCtx, client, player.PUUID, false)
			cancel()
			if err == nil && name != "" {
				e.updatePrepared(token, player.PUUID, func(p *core.Participant) { p.DisplayName = name })
			}
		}
		phaseCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		phase, err := client.Phase(phaseCtx)
		cancel()
		if err != nil || stageForPhase(phase) != preparingStage || ctx.Err() != nil {
			e.mu.Lock()
			if e.prepareToken == token {
				e.prepareRetryAt = time.Now().Add(2 * time.Second)
			}
			e.mu.Unlock()
			return
		}
		requestCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		history, err := e.scheduler.history(requestCtx, fmt.Sprintf("%d:%s", version, player.PUUID), false, func(c context.Context) (core.HistoryResult, error) { return client.History(c, player.PUUID) })
		cancel()
		if err != nil {
			e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
				p.HistoryState = core.HistoryFailed
				p.Error = "战绩准备失败，可手动重试"
			})
			continue
		}
		e.updatePrepared(token, player.PUUID, func(p *core.Participant) { core.ApplyHistory(p, history) })
	}
}
func (e *Engine) updatePrepared(token uint64, puuid string, edit func(*core.Participant)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.prepareToken != token || !preparationStage(e.snapshot.Stage) {
		return
	}
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.PUUID == puuid {
			edit(p)
			if historyCached(*p) {
				e.cachePlayerLocked(*p)
			}
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
			return
		}
	}
}
func (e *Engine) loadName(ctx context.Context, client Client, puuid string, manual bool) (string, error) {
	release, err := e.scheduler.acquire(ctx, manual)
	if err != nil {
		return "", err
	}
	defer release()
	return client.Name(ctx, puuid)
}
func (e *Engine) PrioritizePlayer(key string) { e.mu.Lock(); e.preferred = key; e.mu.Unlock() }

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
	return e.lookup(ctx, riotID, false)
}
func (e *Engine) RefreshLookup(ctx context.Context, riotID string) (core.Participant, error) {
	return e.lookup(ctx, riotID, true)
}
func (e *Engine) lookup(ctx context.Context, riotID string, force bool) (core.Participant, error) {
	riotID, err := core.NormalizeRiotID(riotID)
	if err != nil {
		return core.Participant{}, err
	}
	ctx, release, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	e.mu.RLock()
	client := e.client
	generation := e.snapshot.Generation
	version := e.clientVersion
	cached := clone(e.snapshot)
	cacheID := e.aliases[normalizeID(riotID)]
	entry, hasEntry := e.cache[cacheID]
	e.mu.RUnlock()
	if !force && hasEntry && (client == nil || cacheFresh(entry)) {
		return e.enrichCachedProfile(ctx, cachedPlayer(entry, client == nil)), nil
	}
	for _, p := range cached.Participants {
		if !force && normalizeID(p.DisplayName) == normalizeID(riotID) && historyCached(p) && (client == nil || (p.ConnectionID == version && !p.Stale && cacheFresh(p))) {
			return e.enrichCachedProfile(ctx, cachedPlayer(p, client == nil)), nil
		}
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	releaseSlot, err := e.scheduler.acquire(ctx, true)
	if err != nil {
		return core.Participant{}, err
	}
	player, err := client.FindByRiotID(ctx, riotID)
	releaseSlot()
	if err != nil {
		return core.Participant{}, err
	}
	return e.loadManualHistory(ctx, client, generation, version, player, force, true)
}

func (e *Engine) LookupPlayer(ctx context.Context, key string) (core.Participant, error) {
	return e.lookupPlayer(ctx, key, false)
}
func (e *Engine) RefreshPlayer(ctx context.Context, key string) (core.Participant, error) {
	return e.lookupPlayer(ctx, key, true)
}
func (e *Engine) lookupPlayer(ctx context.Context, key string, force bool) (core.Participant, error) {
	ctx, release, err := e.beginLookup(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	defer release()
	e.mu.RLock()
	snapshot := clone(e.snapshot)
	version := e.clientVersion
	entry, hasEntry := e.cache[key]
	client := e.client
	e.mu.RUnlock()
	var player core.Participant
	for _, p := range snapshot.Participants {
		if p.Key == key && (client == nil || p.ConnectionID == version) {
			player = p
			break
		}
	}
	if player.PUUID == "" && hasEntry {
		player = entry
	}
	if !core.IsKnownPUUID(player.PUUID) {
		return core.Participant{}, errors.New("客户端尚未提供该玩家身份，可刷新名单或输入完整 ID 查询")
	}
	if !force && historyCached(player) && (client == nil || cacheFresh(player)) {
		return e.enrichCachedProfile(ctx, cachedPlayer(player, client == nil)), nil
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	if !strings.Contains(player.DisplayName, "#") {
		if name, err := e.loadName(ctx, client, player.PUUID, true); err == nil && name != "" {
			player.DisplayName = name
		}
	}
	return e.loadManualHistory(ctx, client, snapshot.Generation, version, player, force, true)
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

func (e *Engine) loadManualHistory(ctx context.Context, client Client, generation uint64, version uint64, player core.Participant, force, enrich bool) (core.Participant, error) {
	e.mu.RLock()
	cached, ok := e.cache[player.PUUID]
	sameClient := e.clientVersion == version
	e.mu.RUnlock()
	if !force && sameClient && ok && cacheFresh(cached) {
		if enrich {
			return e.enrichCachedProfile(ctx, cachedPlayer(cached, false)), nil
		}
		return cachedPlayer(cached, false), nil
	}
	hadHistory := historyCached(player)
	player.HistoryState = core.HistoryLoading
	history, err := e.scheduler.history(ctx, fmt.Sprintf("%d:%s", version, player.PUUID), true, func(c context.Context) (core.HistoryResult, error) { return client.History(c, player.PUUID) })
	if err != nil {
		// A refresh failure must keep verified history. A first failure still
		// returns a usable identity so optional profile data can be inspected.
		if (sameClient && ok && historyCached(cached)) || hadHistory {
			return core.Participant{}, fmt.Errorf("已找到玩家，但战绩请求失败: %w", err)
		}
		player.HistoryState = core.HistoryFailed
		player.Error = "已确认玩家身份，但战绩暂不可用；可重试战绩或独立更新资料"
	} else {
		core.ApplyHistory(&player, history)
	}
	player.ConnectionID = version
	// Enrich only explicit lookups, sharing the request lane and a short budget.
	// Unsupported optional APIs cannot discard successfully fetched history.
	if profiles, ok := client.(source.ProfileSource); ok && enrich {
		profileCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if release, err := e.scheduler.acquire(profileCtx, true); err == nil {
			e.markProfileAttempt(version, player.Key)
			profile := profiles.Profile(profileCtx, player.PUUID)
			previous := player.Profile
			if previous == nil && sameClient && ok {
				previous = cached.Profile
			}
			profile = mergeProfile(profile, previous)
			player.Profile = &profile
			player.PrivacyState = profile.PrivacyState
			release()
		}
		cancel()
	}
	e.mu.Lock()
	if e.client == nil || e.clientVersion != version {
		player.Stale = true
	}
	if e.client != nil && e.clientVersion == version {
		e.cachePlayerLocked(player)
	}
	if e.snapshot.Generation == generation && e.clientVersion == version {
		for i, p := range e.snapshot.Participants {
			if p.PUUID == player.PUUID {
				e.snapshot.Participants[i].ConnectionID = version
				e.snapshot.Participants[i].Cached = player.Cached
				e.snapshot.Participants[i].Stale = player.Stale
				e.snapshot.Participants[i].DisplayName = player.DisplayName
				e.snapshot.Participants[i].History = append([]core.Match(nil), player.History...)
				e.snapshot.Participants[i].HistoryMeta = player.HistoryMeta
				e.snapshot.Participants[i].Stats = player.Stats
				e.snapshot.Participants[i].Profile = player.Profile
				e.snapshot.Participants[i].PrivacyState = player.PrivacyState
				e.snapshot.Participants[i].HistoryState = player.HistoryState
				e.snapshot.Participants[i].Error = player.Error
			}
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
	e.mu.Unlock()
	return player, nil
}

func normalizeID(value string) string {
	normalized, err := core.NormalizeRiotID(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(normalized)
}
func cacheFresh(p core.Participant) bool {
	return p.HistoryMeta.FetchedAt != nil && time.Since(*p.HistoryMeta.FetchedAt) < 5*time.Minute
}
func cachedPlayer(p core.Participant, offline bool) core.Participant {
	p.Cached, p.Stale = true, offline || !cacheFresh(p)
	return p
}
func (e *Engine) cachePlayerLocked(p core.Participant) {
	if old, ok := e.cache[p.PUUID]; ok && old.ConnectionID == p.ConnectionID && core.IsPlaceholderName(p.DisplayName) && !core.IsPlaceholderName(old.DisplayName) {
		p.DisplayName = old.DisplayName
	}
	if len(e.cache) >= 64 {
		e.cache = map[string]core.Participant{}
		e.aliases = map[string]string{}
	}
	e.cache[p.PUUID] = p
	if alias := normalizeID(p.DisplayName); alias != "" {
		e.aliases[alias] = p.PUUID
	}
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
	version := e.clientVersion
	epoch, champEpoch := e.phaseEpoch, e.champEpoch
	e.mu.RUnlock()
	if client == nil {
		return e.Snapshot(), source.ErrNotConnected
	}
	phase, err := client.Phase(ctx)
	if err != nil {
		return e.Snapshot(), err
	}
	stage := stageForPhase(phase)
	if !preparationStage(stage) && !activeMatch(phase) {
		return e.Snapshot(), errors.New("当前没有可刷新的房间或对局名单，请稍后重试")
	}
	var gameID int64
	var roster []core.Participant
	if stage == "party" {
		lobby, ok := client.(source.LobbySource)
		if !ok {
			return e.Snapshot(), errors.New("当前数据源不支持房间成员")
		}
		roster, err = lobby.Lobby(ctx)
	} else {
		gameID, roster, err = readRoster(ctx, client, stage)
	}
	if err != nil {
		return e.Snapshot(), fmt.Errorf("本地名单刷新失败：%w", err)
	}
	if len(roster) == 0 {
		return e.Snapshot(), errors.New("客户端暂未提供名单，请稍后再试")
	}
	e.mu.Lock()
	if generation != e.snapshot.Generation || version != e.clientVersion || epoch != e.phaseEpoch || (stage == "preparing" && champEpoch != e.champEpoch) {
		e.mu.Unlock()
		return e.Snapshot(), errors.New("对局状态已变化，请重试刷新名单")
	}
	for i := range roster {
		roster[i].ConnectionID = version
	}
	if stage == "preparing" && e.snapshot.Stage == stage && gameID == 0 {
		gameID = e.snapshot.GameID
	}
	if activeMatch(phase) && e.snapshot.ConnectionID == version && gameID == e.snapshot.GameID && !rosterImproves(e.snapshot.Participants, roster) {
		roster = clone(e.snapshot).Participants
	}
	if activeMatch(phase) && (!activeMatch(e.snapshot.Phase) || (phase == "GameStart" && e.snapshot.Phase != "GameStart") || (gameID != 0 && gameID != e.snapshot.GameID)) {
		e.matchRosterAttempts = 0
		e.matchRosterComplete = false
	}
	if phase == "InProgress" && e.snapshot.Phase == "GameStart" && !e.matchRosterComplete {
		e.matchRosterAttempts = 0
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
	return known(next) > known(old) || (len(next) >= len(old) && known(next) >= known(old))
}
func activeMatch(phase string) bool {
	return phase == "GameStart" || phase == "InProgress" || phase == "Reconnect"
}

func stageForPhase(phase string) string {
	switch phase {
	case "Lobby", "Matchmaking", "ReadyCheck":
		return "party"
	case "ChampSelect":
		return "preparing"
	case "GameStart", "InProgress", "Reconnect", "WatchInProgress", "WaitingForStats", "PreEndOfGame", "EndOfGame":
		return "in-game"
	default:
		return "idle"
	}
}

func preparationStage(stage string) bool { return stage == "preparing" || stage == "party" }

func rosterChanged(old, current []core.Participant) bool {
	if len(old) != len(current) {
		return true
	}
	for i := range old {
		if old[i].ConnectionID != current[i].ConnectionID || old[i].Key != current[i].Key || old[i].ChampionID != current[i].ChampionID || old[i].ChampionState != current[i].ChampionState || old[i].Team != current[i].Team {
			return true
		}
	}
	return false
}

func clone(in core.Snapshot) core.Snapshot {
	out := in
	out.KnownCount, out.ReadyCount = 0, 0
	for _, p := range in.Participants {
		if core.IsKnownPUUID(p.PUUID) {
			out.KnownCount++
		}
		if historyCached(p) {
			out.ReadyCount++
		}
	}
	out.Participants = make([]core.Participant, len(in.Participants))
	copy(out.Participants, in.Participants)
	for i := range out.Participants {
		out.Participants[i].History = append([]core.Match(nil), in.Participants[i].History...)
		if historyCached(out.Participants[i]) && (!in.Connected || !cacheFresh(out.Participants[i]) || out.Participants[i].ConnectionID != in.ConnectionID) {
			out.Participants[i].Stale = true
		}
	}
	return out
}
