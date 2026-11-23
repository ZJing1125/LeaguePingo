package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/postgame"
	"leaguepingo/internal/readycheck"
	"leaguepingo/internal/source"
	"leaguepingo/internal/teambrief"
)

type Client = source.Source

type Discoverer func() (Client, error)

const inGameRosterAttemptLimit = 3

type Engine struct {
	brief           *teambrief.Controller
	historyRecovery historyRecovery
	readiness       diagnostics.Readiness
	grouping        groupingState
	autoAccept      *readycheck.Controller
	postGame        *postgame.Controller
	mu              sync.RWMutex
	discovery       *clientDiscovery
	client          Client
	snapshot        core.Snapshot
	session         source.SessionMetadata
	publisher       *snapshotPublisher
	queries         *playerQueries
	roster          rosterDiscovery
	players         playerCache
	clientVersion   uint64
	accountIdentity string
	phaseEpoch      uint64
	phaseHint       Phase
	phaseEvents     *eventSubscription
	lobbyWake       chan struct{}
	champEvents     *eventSubscription
	champEpoch      uint64
	scheduler       *requestScheduler
	preparation     preparationState
	initialMatch    initialMatchPreparation
	scouting        scoutPreparation
	reviews         reviewState
	historyStore    *historyStore
}

func New(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	return newEngine(discover, onChange)
}

func NewWithHistoryCache(discover Discoverer, onChange func(core.Snapshot), path string) (*Engine, error) {
	e := newEngine(discover, onChange)
	store, err := openHistoryStore(path)
	if store != nil {
		e.historyStore = store
	}
	return e, err
}

func newEngine(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	e := &Engine{discovery: newClientDiscovery(discover), publisher: newSnapshotPublisher(onChange), phaseEvents: newEventSubscription(), champEvents: newEventSubscription(), scheduler: newScheduler(), snapshot: core.Snapshot{Stage: string(StageDisconnected), UpdatedAt: time.Now(), Participants: []core.Participant{}}}
	e.queries = newPlayerQueries(e, e.scheduler)
	e.autoAccept = readycheck.New()
	e.historyRecovery.init()
	e.postGame = postgame.New()
	e.brief = teambrief.New()
	e.lobbyWake = make(chan struct{}, 1)
	return e
}

func (e *Engine) Snapshot() core.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return clone(e.snapshot)
}

func (e *Engine) Run(ctx context.Context) {
	defer e.brief.Stop()
	postDone := make(chan struct{})
	go func() { defer close(postDone); e.postGame.Run(ctx) }()
	defer func() { <-postDone }()
	acceptDone := make(chan struct{})
	go func() { defer close(acceptDone); e.autoAccept.Run(ctx) }()
	defer func() { <-acceptDone }()
	defer e.historyStore.close()
	e.mu.Lock()
	e.initialMatch.setParent(ctx)
	e.mu.Unlock()
	e.poll(ctx)
	e.syncPhaseEvents(ctx)
	e.syncChampSelectEvents(ctx)
	timer := time.NewTimer(e.pollInterval())
	defer timer.Stop()
	var champTimer *time.Timer
	var champTick <-chan time.Time
	var lobbyTimer *time.Timer
	var lobbyTick <-chan time.Time
	defer func() {
		if lobbyTimer != nil {
			lobbyTimer.Stop()
		}
		if champTimer != nil {
			champTimer.Stop()
		}
	}()
	for {
		select {
		case <-e.brief.Changes():
			e.mu.Lock()
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
			e.mu.Unlock()
		case <-ctx.Done():
			e.mu.Lock()
			e.historyRecovery.stop()
			e.readiness.Reset()
			e.stopReviewLocked()
			e.stopScoutingLocked()
			e.queries.stopHome()
			e.stopInitialMatchLocked()
			e.preparation.stop()
			e.stopPhaseEventsLocked()
			e.stopChampSelectEventsLocked()
			e.mu.Unlock()
			return
		case <-timer.C:
			e.poll(ctx)
			e.syncPhaseEvents(ctx)
			e.syncChampSelectEvents(ctx)
			timer.Reset(e.pollInterval())
		case <-e.phaseEvents.wake:
			e.poll(ctx)
			e.syncPhaseEvents(ctx)
			e.syncChampSelectEvents(ctx)
			timer.Reset(e.pollInterval())
		case <-e.lobbyWake:
			if lobbyTick == nil {
				lobbyTimer = time.NewTimer(250 * time.Millisecond)
				lobbyTick = lobbyTimer.C
			}
		case tick := <-lobbyTick:
			lobbyTick = nil
			e.poll(ctx)
			diagnostics.Default.Record("/room/event-to-snapshot", time.Since(tick.Add(-250*time.Millisecond)), 0, false)
			timer.Reset(e.pollInterval())
		case <-e.champEvents.wake:
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
	stage := snapshotStage(e.snapshot.Stage)
	if stage == StageInGame {
		if e.client != nil && activeMatch(snapshotPhase(e.snapshot.Phase)) && !e.roster.view().complete && e.roster.view().attempts < inGameRosterAttemptLimit {
			return 2 * time.Second
		}
		if e.client != nil && snapshotPhase(e.snapshot.Phase) == PhaseGameStart && !e.roster.view().complete {
			return 5 * time.Second // Phase only after the loading-screen roster budget ends.
		}
	}
	return behaviorForStage(stage).poll
}

func (e *Engine) poll(ctx context.Context) {
	e.mu.Lock()
	client := e.client
	if client == nil && e.discovery.claim(time.Now()) {
		e.mu.Unlock()
		found, err := e.discovery.find()
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
	phaseValue, err := client.Phase(requestCtx)
	if err != nil {
		e.mu.Lock()
		if e.snapshot.Generation == generation && e.clientVersion == version && e.phaseEpoch == epoch {
			e.disconnectLocked(apperrors.UserMessage(err, apperrors.ClientUnavailable))
		}
		e.mu.Unlock()
		return
	}
	phase := Phase(phaseValue)
	phaseObserved := time.Now()
	stage := stageForPhase(phase)
	account := ""
	if preparationStage(stage) || stage == StageIdle {
		if accounts, ok := client.(source.AccountSource); ok {
			// Piggyback on low-frequency pregame polls; no account or history
			// polling is added to active gameplay.
			account, _ = accounts.AccountIdentity(requestCtx)
		}
	}
	e.mu.Lock()
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		e.mu.Unlock()
		return
	}
	e.phaseHint = PhaseUnknown // This phase response postdates the last observed notification.
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
		e.acceptAccountNameLocked()
	}
	e.readiness.Phase(string(phase), phaseObserved)
	e.observeAutoAcceptLocked(phase, phaseStarted)
	refresh := e.roster.observe(phase, snapshotPhase(e.snapshot.Phase), stage)
	previous := e.selectionModeLocked(stage)
	if activeMatch(phase) {
		e.preparation.stop()
	}
	e.mu.Unlock()
	gameID, roster, mode, rosterErr := readPollRoster(requestCtx, client, stage, refresh, version, previous)
	if stage == StageInGame {
		if cached, ok := client.(source.CachedAccountSource); ok {
			account = cached.CachedAccountIdentity()
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.finishPollLocked(pollRosterResult{
		generation: generation, version: version, epoch: epoch, champEpoch: champEpoch,
		phase: phase, stage: stage, account: account, gameID: gameID,
		roster: roster, mode: mode, rosterErr: rosterErr, refresh: refresh,
	})
}

type pollRosterResult struct {
	generation, version, epoch, champEpoch uint64
	phase                                  Phase
	stage                                  Stage
	account                                string
	gameID                                 int64
	roster                                 []core.Participant
	mode                                   source.SessionMetadata
	rosterErr                              error
	refresh                                bool
}

func readPollRoster(ctx context.Context, client Client, stage Stage, refresh bool, version uint64, previous source.SessionMetadata) (int64, []core.Participant, source.SessionMetadata, error) {
	if !preparationStage(stage) && !refresh {
		return 0, nil, source.SessionMetadata{}, nil
	}
	gameID, roster, mode, err := readStageRoster(ctx, client, stage, previous)
	return gameID, rosterForConnection(roster, version), mode, err
}

func (e *Engine) finishPollLocked(result pollRosterResult) {
	generation, version, epoch, champEpoch := result.generation, result.version, result.epoch, result.champEpoch
	phase, stage, account := result.phase, result.stage, result.account
	gameID, roster, rosterErr, refresh := result.gameID, result.roster, result.rosterErr, result.refresh
	if e.snapshot.Generation != generation || e.clientVersion != version || e.phaseEpoch != epoch {
		return
	}
	if e.accountIdentity == "" && core.IsKnownPUUID(account) {
		e.accountIdentity = account
	}
	if stage == StagePreparing && e.champEpoch != champEpoch {
		return
	}
	if stage == StagePreparing && snapshotStage(e.snapshot.Stage) == stage && e.snapshot.ConnectionID == version && gameID == 0 && !e.newSelectionLocked(result.mode) {
		gameID = e.snapshot.GameID
	}
	modeChanged := e.updateSessionLocked(stage, gameID, result.mode, preparationStage(stage) || refresh, rosterErr)
	if stage == StageInGame {
		// Refresh only local identities at match entry (three attempts per
		// readiness phase; GameStart -> InProgress can renew once).
		// Run may fill missing names/first histories once after entry discovery.
		if refresh && rosterErr == nil && gameID != 0 && completeRoster(roster) && e.modeDiscoveredLocked() {
			e.roster.markComplete()
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
	if preparationStage(stage) && len(roster) == 0 && snapshotStage(e.snapshot.Stage) == stage && e.snapshot.ConnectionID == version {
		// A transient empty session must not discard prepared data.
		roster = clone(e.snapshot).Participants
		if gameID == 0 {
			gameID = e.snapshot.GameID
		}
	}
	if e.snapshot.ConnectionID != version || snapshotPhase(e.snapshot.Phase) != phase || e.snapshot.GameID != gameID || !e.snapshot.Connected || e.snapshot.SelfKey != e.accountIdentity {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if (preparationStage(stage) || stage == StageInGame) && rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if snapshotStage(e.snapshot.Stage) == StageDisconnected {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if modeChanged {
		e.snapshot.Generation++
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
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
	e.brief.Observe(e.initialMatch.parentContext(), nil, teambrief.Frame{})
	e.historyRecovery.reset()
	e.readiness.Reset()
	e.grouping.stop()
	e.autoAccept.Observe(nil, 0, "")
	e.postGame.Observe(nil, postgame.Observation{})
	e.stopReviewLocked()
	e.stopScoutingLocked()
	e.scouting.invalidate()
	e.queries.stopHome()
	e.stopInitialMatchLocked()
	e.stopPhaseEventsLocked()
	e.stopChampSelectEventsLocked()
	e.champEvents.resetRetry()
	e.phaseHint = ""
	e.clientVersion++
	e.session = source.SessionMetadata{}
	e.accountIdentity = ""
	e.roster.reset()
	e.players.reset()
	e.preparation.invalidate()
	for i := range e.snapshot.Participants {
		e.snapshot.Participants[i].Stale = true
	}
}

// Keep the frozen match available while exposing why reconnection failed.
func (e *Engine) disconnectLocked(message string) {
	e.readiness.Reset()
	e.grouping.stop()
	e.autoAccept.Observe(nil, 0, "")
	e.postGame.Observe(nil, postgame.Observation{})
	e.stopReviewLocked()
	e.stopScoutingLocked()
	e.queries.stopHome()
	e.stopInitialMatchLocked()
	e.stopPhaseEventsLocked()
	e.stopChampSelectEventsLocked()
	e.client = nil
	e.preparation.stop()
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
		e.replaceLocked(StageDisconnected, PhaseUnknown, 0, nil, message)
	}
}

func (e *Engine) replaceLocked(stage Stage, phase Phase, gameID int64, roster []core.Participant, message string) {
	if phase != snapshotPhase(e.snapshot.Phase) {
		e.historyRecovery.stop()
	}
	e.readiness.Phase(string(phase), time.Now())
	if stage != StageIdle {
		e.queries.stopHome()
	}
	if entryGame := e.initialMatch.game(); entryGame != 0 && (!activeMatch(phase) || (gameID != 0 && entryGame != gameID)) {
		e.stopInitialMatchLocked()
	}
	if stage != StagePreparing {
		e.stopChampSelectEventsLocked()
	}
	sameConnection := e.snapshot.ConnectionID == e.clientVersion
	samePreparation := sameConnection && preparationStage(stage) && snapshotStage(e.snapshot.Stage) == stage && (gameID == 0 || e.snapshot.GameID == 0 || gameID == e.snapshot.GameID)
	if !samePreparation {
		e.preparation.reset()
	}
	if samePreparation && stage == StageParty {
		for _, p := range roster {
			found := false
			for _, old := range e.snapshot.Participants {
				if old.PUUID == p.PUUID {
					found = true
					break
				}
			}
			if !found && core.IsKnownPUUID(p.PUUID) {
				e.preparation.extend(time.Now())
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
			e.players.restorePlayer(roster[i])
		}
	}
	if stage == StageInGame && message == "" {
		if len(roster) == 0 {
			message = "客户端已连接，当前尚未取得对局名单；可刷新名单或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else if !completeRoster(roster) {
			message = "当前名单仍有身份未提供；可刷新本地名单，或输入完整 ID 手动查询。局内暂停自动战绩预取。"
		} else {
			message = "已取得本地对局名单，保留赛前战绩；缺少资料的玩家可点击手动查询。局内暂停自动战绩预取。"
		}
		if e.initialMatch.running(gameID) {
			message = initialMatchMessage
		}
	}
	e.carryPreparedRosterLocked(roster, stage, gameID, sameConnection)
	// Names belong to a verified connection/player, independently of the match
	// or history timestamp. Never let a nameless roster overwrite the cache.
	for i := range roster {
		if cached, ok := e.players.player(roster[i].PUUID); ok && cached.ConnectionID == e.clientVersion {
			if core.IsPlaceholderName(roster[i].DisplayName) && !core.IsPlaceholderName(cached.DisplayName) {
				roster[i].DisplayName = cached.DisplayName
			}
			if roster[i].Profile == nil {
				roster[i].Profile = cached.Profile
				roster[i].PrivacyState = cached.PrivacyState
			}
		}
	}
	if (stage == StageInGame && !e.initialMatch.hasTask()) || (stage != StageInGame && !samePreparation) {
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
	behavior := behaviorForStage(stage)
	e.snapshot = core.Snapshot{ConnectionID: e.clientVersion, SelfKey: e.accountIdentity, Generation: e.snapshot.Generation, Phase: string(phase), Stage: string(stage),
		Connected: behavior.connected, Region: region, GameID: gameID, Frozen: behavior.frozen,
		UpdatedAt: time.Now(), Participants: roster, Message: message}
	e.publishLocked()
	if !preparationStage(stage) {
		e.preparation.forgetCancel()
	}
	e.startPreparationLocked()
	e.startInitialMatchLocked()
	e.syncScoutingLocked()
}

func (e *Engine) carryPreparedRosterLocked(roster []core.Participant, stage Stage, gameID int64, sameConnection bool) {
	if sameConnection && (preparationStage(stage) || stage == StageInGame) && (gameID == 0 || e.snapshot.GameID == 0 || gameID == e.snapshot.GameID) {
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
}

func (e *Engine) updatePrepared(token uint64, puuid string, edit func(*core.Participant)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.preparation.current(token) || !preparationStage(snapshotStage(e.snapshot.Stage)) {
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
func (e *Engine) PrioritizePlayer(key string) { e.preparation.prioritize(key) }

func (e *Engine) publishLocked() {
	e.applySessionLocked()
	core.ApplyTeamRatings(e.snapshot.Participants, e.scoutModeLocked())
	e.syncGroupingLocked()
	e.observeAnalysisBriefLocked()
	e.syncReviewLocked()
	e.snapshot.Review, _ = e.reviews.view()
	e.observePostGameLocked()
	e.observeReadinessLocked()
	if e.publisher.enabled() {
		e.publisher.publish(clone(e.snapshot))
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
	entry, hasEntry := e.players.byAlias(normalizeID(riotID))
	e.mu.RUnlock()
	if !force && hasEntry {
		if player, ok := e.reuseLookupHistory(ctx, client, generation, version, entry); ok {
			return player, nil
		}
	}
	for _, p := range cached.Participants {
		if !force && normalizeID(p.DisplayName) == normalizeID(riotID) {
			if player, ok := e.reuseLookupHistory(ctx, client, generation, version, p); ok {
				return player, nil
			}
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
	return e.loadManualHistory(ctx, client, generation, version, player, force, force)
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
	entry, hasEntry := e.players.player(key)
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
	if !force {
		if cached, ok := e.reuseLookupHistory(ctx, client, snapshot.Generation, version, player); ok {
			return cached, nil
		}
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	return e.loadManualHistory(ctx, client, snapshot.Generation, version, player, force, force)
}

func (e *Engine) beginLookup(ctx context.Context) (context.Context, func(), error) {
	return e.queries.begin(ctx)
}

func (e *Engine) loadManualHistory(ctx context.Context, client Client, generation uint64, version uint64, player core.Participant, force, enrich bool) (core.Participant, error) {
	return e.loadManualHistoryGuarded(ctx, client, generation, version, player, force, enrich, nil)
}

// validLocked optionally prevents a cancelled home task from replacing cache
// prepared by a newer phase. It is evaluated while e.mu is held.
func (e *Engine) loadManualHistoryGuarded(ctx context.Context, client Client, generation uint64, version uint64, player core.Participant, force, enrich bool, validLocked func() bool) (core.Participant, error) {
	return e.queries.history(ctx, client, generation, version, player, force, enrich, validLocked)
}

func (e *Engine) queryCachedPlayer(key string, version uint64) queryCacheRead {
	e.mu.RLock()
	defer e.mu.RUnlock()
	player, found := e.players.player(key)
	return queryCacheRead{player: player, found: found, current: e.clientVersion == version}
}

func (e *Engine) queryCurrent(version uint64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.client != nil && e.clientVersion == version
}

// Only Engine commits query data: the home guard, current connection and newer
// profile clock are checked atomically with cache/snapshot updates.
func (e *Engine) commitManualQuery(generation, version uint64, player core.Participant, validLocked func() bool) (core.Participant, error) {
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
	if latest, ok := e.players.player(player.PUUID); ok && latest.ConnectionID == version && latest.Profile != nil &&
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
		e.historyRecovery.forget(p.Key)
	}
	p = e.players.remember(p)
	if len(p.History) > 0 && !p.Cached {
		returned := p.HistoryMeta.Returned
		if returned <= 0 || returned > len(p.History) {
			returned = len(p.History)
		}
		e.historyStore.put(p.PUUID, core.HistoryPage{Matches: p.History, Meta: p.HistoryMeta, Offset: 0, NextOffset: returned, HasMore: returned >= 20})
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
	if e.client == nil || e.clientVersion != version || player.ConnectionID != version {
		e.mu.Unlock()
		return
	}
	parent := e.initialMatch.parentContext()
	if parent == nil {
		parent = context.Background()
	}
	ctx, task, started := e.historyRecovery.beginBackground(parent, player.PUUID)
	if !started {
		e.mu.Unlock()
		return
	}
	epoch := e.phaseEpoch
	e.mu.Unlock()
	go e.readHydratedHistory(ctx, task, client, generation, version, epoch, player)
}

func (e *Engine) readHydratedHistory(ctx context.Context, task *historyRecoveryTask, client Client, generation, version, epoch uint64, player core.Participant) {
	defer e.historyRecovery.finishBackground(player.PUUID, task)
	history, err := e.scheduler.history(ctx, fmt.Sprintf("%d:%s", version, player.PUUID), false, func(request context.Context) (core.HistoryResult, error) {
		e.mu.RLock()
		current := e.client != nil && e.clientVersion == version && e.phaseEpoch == epoch
		e.mu.RUnlock()
		if !current {
			return core.HistoryResult{}, apperrors.New(apperrors.StaleData)
		}
		return client.History(request, player.PUUID)
	})
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil || e.client == nil || e.clientVersion != version || e.phaseEpoch != epoch {
		return
	}
	if current, ok := e.players.player(player.PUUID); ok && current.ConnectionID == version {
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
				if err == nil {
					core.ApplyHistory(&e.snapshot.Participants[i], history)
				} else {
					e.snapshot.Participants[i].HistoryMeta.Service = player.HistoryMeta.Service
					e.snapshot.Participants[i].HistoryMeta.CheckedAt = player.HistoryMeta.CheckedAt
					e.snapshot.Participants[i].Error = player.Error
				}
			}
		}
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
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
	previous := e.selectionModeLocked(StagePreparing)
	e.mu.RUnlock()
	if client == nil {
		return e.Snapshot(), source.ErrNotConnected
	}
	phaseValue, err := client.Phase(ctx)
	if err != nil {
		return e.Snapshot(), err
	}
	phase := Phase(phaseValue)
	stage := stageForPhase(phase)
	if !preparationStage(stage) && !activeMatch(phase) {
		return e.Snapshot(), apperrors.New(apperrors.NoRoster)
	}
	gameID, roster, mode, err := readStageRoster(ctx, client, stage, previous)
	if err != nil {
		return e.Snapshot(), apperrors.Normalize(err, apperrors.NoRoster)
	}
	if len(roster) == 0 {
		return e.Snapshot(), apperrors.New(apperrors.NoRoster)
	}
	e.mu.Lock()
	if generation != e.snapshot.Generation || version != e.clientVersion || epoch != e.phaseEpoch || (stage == StagePreparing && champEpoch != e.champEpoch) {
		e.mu.Unlock()
		return e.Snapshot(), apperrors.New(apperrors.StaleData)
	}
	roster = rosterForConnection(roster, version)
	if stage == StagePreparing && snapshotStage(e.snapshot.Stage) == stage && gameID == 0 && !e.newSelectionLocked(mode) {
		gameID = e.snapshot.GameID
	}
	if activeMatch(phase) && e.snapshot.ConnectionID == version && gameID == e.snapshot.GameID && !rosterImproves(e.snapshot.Participants, roster) {
		roster = clone(e.snapshot).Participants
	}
	e.updateSessionLocked(stage, gameID, mode, true, nil)
	e.roster.refreshed(phase, snapshotPhase(e.snapshot.Phase), gameID, e.snapshot.GameID)
	// Explicit refresh also renews the independent scouting round. Ordinary
	// polls, card selection and overlay visibility do not reset it.
	e.stopScoutingLocked()
	e.scouting.refresh(roster)
	e.replaceLocked(stage, phase, gameID, roster, "")
	if activeMatch(phase) && gameID != 0 && completeRoster(roster) && e.modeDiscoveredLocked() {
		e.roster.markComplete()
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

func (e *Engine) WaitForQueryIdle(ctx context.Context) error {
	return e.queries.waitForIdle(ctx)
}
