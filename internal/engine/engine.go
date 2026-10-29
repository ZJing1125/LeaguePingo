package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/postgame"
	"leaguepingo/internal/readycheck"
	"leaguepingo/internal/source"
)

type Client = source.Source

type Discoverer func() (Client, error)

const inGameRosterAttemptLimit = 3

type Engine struct {
	historyRecovery     historyRecovery
	readiness           diagnostics.Readiness
	groupLabels         map[string]string
	groupScope          string
	groupAttempted      bool
	groupCancel         context.CancelFunc
	groupMembers        map[string]string
	groupSides          map[string]string
	groupPositions      map[string]string
	autoAccept          *readycheck.Controller
	postGame            *postgame.Controller
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
	homeCancel          context.CancelFunc
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
	prepareProfiles     map[string]bool
	preferred           string
	details             map[string]core.MatchDetail
	timelines           map[string]core.MatchTimeline
	initialMatch        initialMatchPreparation
	scouting            scoutPreparation
	reviewCandidate     *core.MatchReview
	review              *core.MatchReview
	reviewOwner         string
	reviewCancel        context.CancelFunc
	reviewToken         uint64
	historyStore        *historyStore
}

func New(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	return newEngine(discover, onChange)
}

func NewWithHistoryCache(discover Discoverer, onChange func(core.Snapshot), path string) (*Engine, error) {
	e := newEngine(discover, onChange)
	store, err := openHistoryStore(path)
	if err == nil {
		e.historyStore = store
	}
	return e, err
}

func newEngine(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	e := &Engine{discover: discover, onChange: onChange, lookupGate: make(chan struct{}, 1), phaseWake: make(chan struct{}, 1), champWake: make(chan struct{}, 1), scheduler: newScheduler(), cache: map[string]core.Participant{}, aliases: map[string]string{}, snapshot: core.Snapshot{Stage: "disconnected", UpdatedAt: time.Now(), Participants: []core.Participant{}}}
	e.autoAccept = readycheck.New()
	e.historyRecovery.updates = make(chan core.Participant, 64)
	e.postGame = postgame.New()
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
	postDone := make(chan struct{})
	go func() { defer close(postDone); e.postGame.Run(ctx) }()
	defer func() { <-postDone }()
	acceptDone := make(chan struct{})
	go func() { defer close(acceptDone); e.autoAccept.Run(ctx) }()
	defer func() { <-acceptDone }()
	defer e.historyStore.close()
	e.mu.Lock()
	e.initialMatch.parent = ctx
	e.mu.Unlock()
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
			e.historyRecovery.stop()
			e.readiness.Reset()
			e.stopReviewLocked()
			e.stopScoutingLocked()
			if e.homeCancel != nil {
				e.homeCancel()
			}
			e.stopInitialMatchLocked()
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
			message := apperrors.Message(apperrors.ClientNotRunning)
			if err != nil {
				message = apperrors.UserMessage(err, apperrors.ClientUnavailable)
				apperrors.Failures.Record(apperrors.LocalClient, err)
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
	phaseStarted := time.Now()
	phase, err := client.Phase(requestCtx)
	if err != nil {
		e.mu.Lock()
		if e.snapshot.Generation == generation && e.clientVersion == version && e.phaseEpoch == epoch {
			e.disconnectLocked(apperrors.UserMessage(err, apperrors.ClientUnavailable))
		}
		e.mu.Unlock()
		return
	}
	phaseObserved := time.Now()
	stage := stageForPhase(phase)
	account := ""
	if preparationStage(stage) || stage == "idle" {
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
	// None is also the normal postgame home screen. It is not a new
	// connection: invalidating here cancels settlement before it is available.
	// The current-account response above still isolates actual account changes.
	if account != "" {
		if e.accountIdentity != "" && account != e.accountIdentity {
			e.invalidateConnectionLocked()
			version = e.clientVersion
			if resetter, ok := client.(interface{ ResetIdentity() }); ok {
				resetter.ResetIdentity()
			}
		}
		e.accountIdentity = account
	}
	e.readiness.Phase(phase, phaseObserved)
	e.observeAutoAcceptLocked(phase, phaseStarted)
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
	if stage == "in-game" {
		if cached, ok := client.(source.CachedAccountSource); ok {
			account = cached.CachedAccountIdentity()
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		return
	}
	if e.accountIdentity == "" && core.IsKnownPUUID(account) {
		e.accountIdentity = account
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
		// Run may fill missing names/first histories once after entry discovery.
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
	if e.snapshot.ConnectionID != version || e.snapshot.Phase != phase || e.snapshot.GameID != gameID || !e.snapshot.Connected || e.snapshot.SelfKey != e.accountIdentity {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if (preparationStage(stage) || stage == "in-game") && rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if e.snapshot.Stage == "disconnected" {
		e.replaceLocked(stage, phase, gameID, roster, "")
	}
	if preparationStage(stage) || refresh {
		message := ""
		if rosterErr != nil {
			message = "名单暂未更新；" + apperrors.UserMessage(rosterErr, apperrors.NoRoster)
		}
		if e.snapshot.RosterError != message {
			e.snapshot.RosterError = message
			e.publishLocked()
		}
	}
	e.startPreparationLocked()
	e.startInitialMatchLocked()
	e.syncScoutingLocked()
}

// Retained data can still be displayed, but can never become a new connection's cache.
func (e *Engine) invalidateConnectionLocked() {
	e.historyRecovery.stop()
	e.historyRecovery = historyRecovery{updates: e.historyRecovery.updates}
	e.readiness.Reset()
	if e.groupCancel != nil {
		e.groupCancel()
		e.groupCancel = nil
	}
	e.autoAccept.Observe(nil, 0, "")
	e.postGame.Observe(nil, postgame.Observation{})
	e.stopReviewLocked()
	e.stopScoutingLocked()
	e.scouting.scope = ""
	e.scouting.cache = nil
	e.scouting.attempted = nil
	if e.homeCancel != nil {
		e.homeCancel()
	}
	e.stopInitialMatchLocked()
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
	e.timelines = nil
	if e.cancel != nil {
		e.cancel()
	}
	e.cancel = nil
	e.prepareToken++
	e.prepareRunning = false
	e.prepareDeadline = time.Time{}
	e.prepareRetryAt = time.Time{}
	e.prepareAttempts = nil
	e.prepareProfiles = nil
	for i := range e.snapshot.Participants {
		e.snapshot.Participants[i].Stale = true
	}
}

// Keep the frozen match available while exposing why reconnection failed.
func (e *Engine) disconnectLocked(message string) {
	e.readiness.Reset()
	if e.groupCancel != nil {
		e.groupCancel()
		e.groupCancel = nil
	}
	e.autoAccept.Observe(nil, 0, "")
	e.postGame.Observe(nil, postgame.Observation{})
	e.stopReviewLocked()
	e.stopScoutingLocked()
	if e.homeCancel != nil {
		e.homeCancel()
	}
	e.stopInitialMatchLocked()
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
	if phase != e.snapshot.Phase {
		e.historyRecovery.stop()
	}
	e.readiness.Phase(phase, time.Now())
	if stage != "idle" && e.homeCancel != nil {
		e.homeCancel()
	}
	if e.initialMatch.gameID != 0 && (!activeMatch(phase) || (gameID != 0 && e.initialMatch.gameID != gameID)) {
		e.stopInitialMatchLocked()
	}
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
		e.prepareProfiles = map[string]bool{}
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
	for i := range roster {
		if e.hydrateHistory(&roster[i]) {
			roster[i].ConnectionID = e.clientVersion
			e.cache[roster[i].PUUID] = roster[i]
		}
	}
	if stage == "in-game" && message == "" {
		if len(roster) == 0 {
			message = "客户端已连接，当前尚未取得对局名单；可刷新名单或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else if !completeRoster(roster) {
			message = "当前名单仍有身份未提供；可刷新本地名单，或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else {
			message = "已取得本地对局名单，保留赛前战绩；缺少资料的玩家可点击手动查询。局内暂停自动战绩预取。"
		}
		if e.initialMatch.cancel != nil && e.initialMatch.eligible && e.initialMatch.gameID == gameID {
			message = initialMatchMessage
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
					roster[i].Scout = old.Scout
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
		if cached, ok := e.cache[roster[i].PUUID]; ok && cached.ConnectionID == e.clientVersion {
			if core.IsPlaceholderName(roster[i].DisplayName) && !core.IsPlaceholderName(cached.DisplayName) {
				roster[i].DisplayName = cached.DisplayName
			}
			if roster[i].Profile == nil {
				roster[i].Profile = cached.Profile
				roster[i].PrivacyState = cached.PrivacyState
			}
		}
	}
	if (stage == "in-game" && e.initialMatch.cancel == nil) || (stage != "in-game" && !samePreparation) {
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
	e.snapshot = core.Snapshot{ConnectionID: e.clientVersion, SelfKey: e.accountIdentity, Generation: e.snapshot.Generation, Phase: phase, Stage: stage,
		Connected: stage != "disconnected", Region: region, GameID: gameID, Frozen: stage == "in-game",
		UpdatedAt: time.Now(), Participants: roster, Message: message}
	e.publishLocked()
	if !preparationStage(stage) {
		e.cancel = nil
	}
	e.startPreparationLocked()
	e.startInitialMatchLocked()
	e.syncScoutingLocked()
}

func (e *Engine) startPreparationLocked() {
	if !preparationStage(e.snapshot.Stage) || e.client == nil || e.prepareRunning || time.Now().Before(e.prepareRetryAt) || (e.phaseHint != "" && stageForPhase(e.phaseHint) != e.snapshot.Stage) {
		return
	}
	if e.prepareDeadline.IsZero() {
		e.prepareDeadline = time.Now().Add(45 * time.Second)
		e.prepareAttempts = map[string]int{}
		e.prepareProfiles = map[string]bool{}
	}
	if !time.Now().Before(e.prepareDeadline) {
		return
	}
	for _, p := range e.snapshot.Participants {
		if p.ConnectionID == e.clientVersion && core.IsKnownPUUID(p.PUUID) && ((!(historyCached(p) && cacheFresh(p) && !p.Stale) && e.prepareAttempts[p.PUUID] < 2) || e.needsPreparedProfileLocked(p)) {
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
			e.mu.Lock()
			if e.prepareToken != token || e.clientVersion != version || e.snapshot.Stage != preparingStage || ctx.Err() != nil {
				e.mu.Unlock()
				return
			}
			for _, p := range e.snapshot.Participants {
				if e.needsPreparedProfileLocked(p) {
					player = p
					e.prepareProfiles[p.PUUID] = true
					break
				}
			}
			e.mu.Unlock()
			if player.PUUID == "" {
				return
			}
			e.prepareOptionalProfile(ctx, client, player, func() bool {
				return e.client != nil && e.clientVersion == version && e.prepareToken == token && e.snapshot.Stage == preparingStage && (e.phaseHint == "" || stageForPhase(e.phaseHint) == preparingStage)
			})
			continue
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
			name, err := e.loadName(nameCtx, client, player.PUUID, version, false)
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
			if errors.Is(err, context.Canceled) && ctx.Err() == nil {
				e.mu.Lock()
				if e.prepareToken == token && e.prepareAttempts[player.PUUID] > 0 {
					e.prepareAttempts[player.PUUID]--
				}
				e.mu.Unlock()
				e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
					p.HistoryState = core.HistoryPending
					p.Error = ""
				})
				continue
			}
			e.updatePrepared(token, player.PUUID, func(p *core.Participant) {
				p.HistoryState = core.HistoryFailed
				p.Error = apperrors.UserMessage(err, apperrors.DataUnavailable)
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
			if historyCached(*p) || p.Profile != nil {
				e.cachePlayerLocked(*p)
			}
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
			return
		}
	}
}
func (e *Engine) loadName(ctx context.Context, client Client, puuid string, version uint64, manual bool) (string, error) {
	requestCtx := ctx
	var release func()
	var err error
	if manual {
		release, err = e.scheduler.acquire(ctx, true)
	} else {
		requestCtx, release, err = e.scheduler.acquireAutomatic(ctx)
	}
	if err != nil {
		return "", err
	}
	defer release()
	e.mu.RLock()
	current := e.client != nil && e.clientVersion == version
	e.mu.RUnlock()
	if !current {
		return "", apperrors.New(apperrors.StaleData)
	}
	return client.Name(requestCtx, puuid)
}
func (e *Engine) PrioritizePlayer(key string) { e.mu.Lock(); e.preferred = key; e.mu.Unlock() }

func (e *Engine) publishLocked() {
	if m, ok := e.client.(source.SessionMetadataSource); ok {
		v := m.SessionMetadata()
		e.snapshot.QueueID = v.QueueID
		e.snapshot.GameMode = v.GameMode
		e.snapshot.MapID = v.MapID
		e.snapshot.GameVersion = v.Version
	}
	e.syncGroupingLocked()
	e.syncReviewLocked()
	e.snapshot.Review = e.review
	e.observePostGameLocked()
	e.observeReadinessLocked()
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
	player.ConnectionID = version
	if !force && e.hydrateHistory(&player) {
		e.mu.Lock()
		e.cachePlayerLocked(player)
		e.mu.Unlock()
		e.refreshHydratedHistory(client, generation, version, player)
		return player, nil
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
		return core.Participant{}, apperrors.New(apperrors.PlayerIdentityUnavailable)
	}
	if !force && historyCached(player) && (client == nil || cacheFresh(player)) {
		return e.enrichCachedProfile(ctx, cachedPlayer(player, client == nil)), nil
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	return e.loadManualHistory(ctx, client, snapshot.Generation, version, player, force, true)
}

func (e *Engine) beginLookup(ctx context.Context) (context.Context, func(), error) {
	select {
	case e.lookupGate <- struct{}{}:
		bounded, cancel := context.WithTimeout(ctx, 12*time.Second)
		return bounded, func() { cancel(); <-e.lookupGate }, nil
	default:
		return nil, nil, apperrors.New(apperrors.QueryBusy)
	}
}

func (e *Engine) loadManualHistory(ctx context.Context, client Client, generation uint64, version uint64, player core.Participant, force, enrich bool) (core.Participant, error) {
	return e.loadManualHistoryGuarded(ctx, client, generation, version, player, force, enrich, nil)
}

// validLocked optionally prevents a cancelled home task from replacing cache
// prepared by a newer phase. It is evaluated while e.mu is held.
func (e *Engine) loadManualHistoryGuarded(ctx context.Context, client Client, generation uint64, version uint64, player core.Participant, force, enrich bool, validLocked func() bool) (core.Participant, error) {
	// A complete-ID lookup already verified the name; preserve it even when
	// the matching history cache was created from a nameless game roster.
	if !core.IsPlaceholderName(player.DisplayName) {
		player = e.resolvePlayerName(ctx, client, version, player, true)
	}
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
	player = e.resolvePlayerName(ctx, client, version, player, true)
	if player.Stale {
		return core.Participant{}, apperrors.New(apperrors.StaleData)
	}
	hadHistory := historyCached(player)
	player.HistoryState = core.HistoryLoading
	history, err := e.scheduler.history(ctx, fmt.Sprintf("%d:%s", version, player.PUUID), true, func(c context.Context) (core.HistoryResult, error) {
		e.mu.RLock()
		current := e.client != nil && e.clientVersion == version
		e.mu.RUnlock()
		if !current {
			return core.HistoryResult{}, source.ErrNotConnected
		}
		return client.History(c, player.PUUID)
	})
	if err != nil {
		// A refresh failure must keep verified history. A first failure still
		// returns a usable identity so optional profile data can be inspected.
		if history.Meta.Service == nil && ((sameClient && ok && historyCached(cached)) || hadHistory) {
			return core.Participant{}, apperrors.Normalize(err, apperrors.DataUnavailable)
		}
		if sameClient && ok && historyCached(cached) {
			player = cached
		}
		player.HistoryState = core.HistoryFailed
		if len(player.History) > 0 {
			player.HistoryState = core.HistoryReady
			player.Cached = true
		}
		checked := time.Now()
		player.HistoryMeta.CheckedAt = &checked
		player.HistoryMeta.Service = history.Meta.Service
		player.Error = "已确认玩家身份；" + apperrors.UserMessage(err, apperrors.DataUnavailable)
	} else {
		core.ApplyHistory(&player, history)
	}
	player.ConnectionID = version
	// Enrich only explicit lookups, sharing the request lane and a short budget.
	// Unsupported optional APIs cannot discard successfully fetched history.
	if profiles, ok := client.(source.ProfileSource); ok && enrich {
		profileCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if release, err := e.scheduler.acquire(profileCtx, true); err == nil {
			e.mu.RLock()
			current := e.client != nil && e.clientVersion == version
			e.mu.RUnlock()
			if !current {
				release()
				cancel()
				player.Stale = true
				return player, nil
			}
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
	if validLocked != nil && !validLocked() {
		e.mu.Unlock()
		return core.Participant{}, context.Canceled
	}
	if e.client == nil || e.clientVersion != version {
		player.Stale = true
	}
	// Background profile completion and manual history use independent clocks.
	// Keep a newer profile that completed while history was waiting in the lane.
	if latest, ok := e.cache[player.PUUID]; ok && latest.ConnectionID == version && latest.Profile != nil &&
		(player.Profile == nil || (latest.Profile.UpdatedAt != nil && (player.Profile.UpdatedAt == nil || latest.Profile.UpdatedAt.After(*player.Profile.UpdatedAt)))) {
		player.Profile, player.PrivacyState = latest.Profile, latest.PrivacyState
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
	if p.HistoryMeta.Service == nil && historyCached(p) && p.Error == "" {
		delete(e.historyRecovery.entries, p.Key)
	}
	if old, ok := e.cache[p.PUUID]; ok && old.ConnectionID == p.ConnectionID && core.IsPlaceholderName(p.DisplayName) && !core.IsPlaceholderName(old.DisplayName) {
		p.DisplayName = old.DisplayName
	}
	if len(e.cache) >= 64 {
		e.cache = map[string]core.Participant{}
		e.aliases = map[string]string{}
	}
	e.cache[p.PUUID] = p
	if len(p.History) > 0 && !p.Cached {
		returned := p.HistoryMeta.Returned
		if returned <= 0 || returned > len(p.History) {
			returned = len(p.History)
		}
		e.historyStore.put(p.PUUID, core.HistoryPage{Matches: p.History, Meta: p.HistoryMeta, Offset: 0, NextOffset: returned, HasMore: returned >= 20})
	}
	if alias := normalizeID(p.DisplayName); alias != "" {
		e.aliases[alias] = p.PUUID
	}
}

func (e *Engine) hydrateHistory(player *core.Participant) bool {
	if player == nil || player.IdentityState != core.IdentityKnown || !core.IsKnownPUUID(player.PUUID) || historyCached(*player) {
		return false
	}
	page, ok := e.historyStore.get(player.PUUID, 0, 0)
	if !ok || len(page.Matches) == 0 {
		return false
	}
	core.ApplyHistory(player, core.HistoryResult{Matches: page.Matches, Meta: page.Meta})
	player.Cached = true
	player.Stale = !cacheFresh(*player)
	return true
}

func (e *Engine) refreshHydratedHistory(client Client, generation, version uint64, player core.Participant) {
	e.mu.Lock()
	if e.historyRecovery.background == nil {
		e.historyRecovery.background = map[string]*historyRecoveryEntry{}
	}
	if e.historyRecovery.background[player.PUUID] != nil || len(e.historyRecovery.background) >= 64 {
		e.mu.Unlock()
		return
	}
	parent := e.initialMatch.parent
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	task := &historyRecoveryEntry{cancel: cancel}
	e.historyRecovery.background[player.PUUID] = task
	epoch := e.phaseEpoch
	e.mu.Unlock()
	go func() {
		defer cancel()
		defer func() {
			e.mu.Lock()
			if e.historyRecovery.background[player.PUUID] == task {
				delete(e.historyRecovery.background, player.PUUID)
			}
			e.mu.Unlock()
		}()
		history, err := e.scheduler.history(ctx, fmt.Sprintf("%d:%s", version, player.PUUID), false, func(request context.Context) (core.HistoryResult, error) {
			return client.History(request, player.PUUID)
		})
		e.mu.Lock()
		defer e.mu.Unlock()
		if ctx.Err() != nil || e.client == nil || e.clientVersion != version || e.phaseEpoch != epoch {
			return
		}
		if current, ok := e.cache[player.PUUID]; ok && current.ConnectionID == version {
			player = current
		}
		if err == nil {
			core.ApplyHistory(&player, history)
		} else {
			checked := time.Now()
			player.HistoryMeta.CheckedAt = &checked
			player.HistoryMeta.Service = history.Meta.Service
			player.Error = apperrors.UserMessage(err, apperrors.DataUnavailable)
		}
		player.ConnectionID = version
		e.cachePlayerLocked(player)
		e.publishPlayerLocked(player)
		if e.snapshot.Generation == generation {
			for i := range e.snapshot.Participants {
				if e.snapshot.Participants[i].PUUID == player.PUUID {
					e.snapshot.Participants[i] = player
				}
			}
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
		}
	}()
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
		return e.Snapshot(), apperrors.New(apperrors.NoRoster)
	}
	var gameID int64
	var roster []core.Participant
	if stage == "party" {
		lobby, ok := client.(source.LobbySource)
		if !ok {
			return e.Snapshot(), apperrors.New(apperrors.Unsupported)
		}
		roster, err = lobby.Lobby(ctx)
	} else {
		gameID, roster, err = readRoster(ctx, client, stage)
	}
	if err != nil {
		return e.Snapshot(), apperrors.Normalize(err, apperrors.NoRoster)
	}
	if len(roster) == 0 {
		return e.Snapshot(), apperrors.New(apperrors.NoRoster)
	}
	e.mu.Lock()
	if generation != e.snapshot.Generation || version != e.clientVersion || epoch != e.phaseEpoch || (stage == "preparing" && champEpoch != e.champEpoch) {
		e.mu.Unlock()
		return e.Snapshot(), apperrors.New(apperrors.StaleData)
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
	// Explicit refresh also renews the independent scouting round. Ordinary
	// polls, card selection and overlay visibility do not reset it.
	e.stopScoutingLocked()
	e.scouting.attempted = map[string]bool{}
	e.scouting.deadline = time.Time{}
	e.scouting.pages = 0
	for _, p := range roster {
		delete(e.scouting.cache, p.PUUID)
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
		if old[i].ConnectionID != current[i].ConnectionID || old[i].Key != current[i].Key || old[i].ChampionID != current[i].ChampionID || old[i].ChampionState != current[i].ChampionState || old[i].Team != current[i].Team || old[i].GroupToken != current[i].GroupToken || old[i].Position != current[i].Position {
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
