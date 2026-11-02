package engine

import "time"

// Stage and Phase are internal typed values. Snapshot keeps their string form
// so the Wails contract and frontend values remain unchanged.
type Stage string
type Phase string

const (
	StageDisconnected Stage = "disconnected"
	StageParty        Stage = "party"
	StagePreparing    Stage = "preparing"
	StageInGame       Stage = "in-game"
	StageIdle         Stage = "idle"
)

const (
	PhaseUnknown         Phase = ""
	PhaseNone            Phase = "None"
	PhaseLobby           Phase = "Lobby"
	PhaseMatchmaking     Phase = "Matchmaking"
	PhaseReadyCheck      Phase = "ReadyCheck"
	PhaseChampSelect     Phase = "ChampSelect"
	PhaseGameStart       Phase = "GameStart"
	PhaseInProgress      Phase = "InProgress"
	PhaseReconnect       Phase = "Reconnect"
	PhaseWatchInProgress Phase = "WatchInProgress"
	PhaseWaitingForStats Phase = "WaitingForStats"
	PhasePreEndOfGame    Phase = "PreEndOfGame"
	PhaseEndOfGame       Phase = "EndOfGame"
)

type stageBehavior struct {
	poll        time.Duration
	preparation bool
	connected   bool
	frozen      bool
}

var stageBehaviors = map[Stage]stageBehavior{
	StageDisconnected: {poll: 10 * time.Second},
	StageParty:        {poll: 6 * time.Second, preparation: true, connected: true},
	StagePreparing:    {poll: 3 * time.Second, preparation: true, connected: true},
	StageInGame:       {poll: 20 * time.Second, connected: true, frozen: true},
	StageIdle:         {poll: 6 * time.Second, connected: true},
}

var phaseStages = map[Phase]Stage{
	PhaseLobby:           StageParty,
	PhaseMatchmaking:     StageParty,
	PhaseReadyCheck:      StageParty,
	PhaseChampSelect:     StagePreparing,
	PhaseGameStart:       StageInGame,
	PhaseInProgress:      StageInGame,
	PhaseReconnect:       StageInGame,
	PhaseWatchInProgress: StageInGame,
	PhaseWaitingForStats: StageInGame,
	PhasePreEndOfGame:    StageInGame,
	PhaseEndOfGame:       StageInGame,
}

func behaviorForStage(stage Stage) stageBehavior {
	if behavior, ok := stageBehaviors[stage]; ok {
		return behavior
	}
	return stageBehaviors[StageDisconnected]
}

func activeMatch(phase Phase) bool {
	return phase == PhaseGameStart || phase == PhaseInProgress || phase == PhaseReconnect
}

func stageForPhase(phase Phase) Stage {
	if stage, ok := phaseStages[phase]; ok {
		return stage
	}
	return StageIdle
}

func preparationStage(stage Stage) bool { return behaviorForStage(stage).preparation }

func snapshotStage(value string) Stage { return Stage(value) }
func snapshotPhase(value string) Phase { return Phase(value) }
