package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

type Client = source.Source

type Discoverer func() (Client, error)

type Engine struct {
	mu         sync.RWMutex
	discover   Discoverer
	client     Client
	snapshot   core.Snapshot
	cancel     context.CancelFunc
	onChange   func(core.Snapshot)
	updates    chan core.Snapshot
	lastLookup time.Time
}

func New(discover Discoverer, onChange func(core.Snapshot)) *Engine {
	e := &Engine{discover: discover, onChange: onChange, snapshot: core.Snapshot{Stage: "disconnected", UpdatedAt: time.Now(), Participants: []core.Participant{}}}
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
		if err == nil {
			e.client = found
			client = found
		}
	}
	e.mu.Unlock()
	if client == nil {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	phase, err := client.Phase(requestCtx)
	if err != nil {
		e.mu.Lock()
		e.client = nil
		if e.snapshot.Frozen {
			e.snapshot.Connected = false
			e.snapshot.Message = "客户端连接中断；仍可查看已缓存的对局资料"
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
		} else {
			e.replaceLocked("disconnected", "", 0, nil, "客户端连接中断")
		}
		e.mu.Unlock()
		return
	}
	stage := stageForPhase(phase)
	var gameID int64
	var roster []core.Participant
	if stage == "preparing" {
		gameID, roster, _ = client.Roster(requestCtx)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if stage == "in-game" {
		// Never ask for identities or history once the real match begins.
		gameID = e.snapshot.GameID
		roster = clone(e.snapshot).Participants
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
	} else if stage == "preparing" && rosterChanged(e.snapshot.Participants, roster) {
		e.replaceLocked(stage, phase, gameID, roster, "")
	} else if e.snapshot.Stage == "disconnected" {
		e.replaceLocked(stage, phase, gameID, roster, "")
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
	if stage == "preparing" || stage == "in-game" {
		for i := range roster {
			for _, old := range e.snapshot.Participants {
				if old.PUUID != "" && old.PUUID == roster[i].PUUID {
					roster[i].History = append([]core.Match(nil), old.History...)
					roster[i].Stats = old.Stats
					roster[i].HistoryState = old.HistoryState
					roster[i].DisplayName = old.DisplayName
					roster[i].Error = old.Error
				}
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
		go e.prepare(queryCtx, generation, e.client, roster)
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
	e.mu.RLock()
	client := e.client
	frozen := e.snapshot.Frozen
	e.mu.RUnlock()
	if frozen {
		return core.Participant{}, errors.New("正式对局中暂停新的联网查询")
	}
	if client == nil {
		return core.Participant{}, source.ErrNotConnected
	}
	phase, err := client.Phase(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	if stageForPhase(phase) == "in-game" {
		return core.Participant{}, errors.New("正式对局中暂停新的联网查询")
	}
	player, err := client.FindByRiotID(ctx, riotID)
	if err != nil {
		return core.Participant{}, err
	}
	phase, err = client.Phase(ctx)
	if err != nil {
		return core.Participant{}, err
	}
	if stageForPhase(phase) == "in-game" {
		return core.Participant{}, errors.New("正式对局中暂停新的联网查询")
	}
	player.HistoryState = core.HistoryLoading
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
	return player, nil
}

func stageForPhase(phase string) string {
	switch phase {
	case "ChampSelect":
		return "preparing"
	case "GameStart", "InProgress", "WaitingForStats", "PreEndOfGame", "EndOfGame":
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
