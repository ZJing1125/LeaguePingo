package engine

import (
	"testing"
	"time"
)

func TestPhaseStageMapping(t *testing.T) {
	tests := map[Phase]Stage{
		PhaseLobby: StageParty, PhaseMatchmaking: StageParty, PhaseReadyCheck: StageParty,
		PhaseChampSelect: StagePreparing,
		PhaseGameStart: StageInGame, PhaseInProgress: StageInGame, PhaseReconnect: StageInGame,
		PhaseWatchInProgress: StageInGame, PhaseWaitingForStats: StageInGame,
		PhasePreEndOfGame: StageInGame, PhaseEndOfGame: StageInGame,
		PhaseNone: StageIdle, PhaseUnknown: StageIdle, Phase("FuturePhase"): StageIdle,
	}
	for phase, want := range tests {
		if got := stageForPhase(phase); got != want {
			t.Errorf("stageForPhase(%q) = %q, want %q", phase, got, want)
		}
	}
}

func TestStageBehaviorTable(t *testing.T) {
	tests := map[Stage]stageBehavior{
		StageDisconnected: {poll: 10 * time.Second},
		StageParty:        {poll: 6 * time.Second, preparation: true, connected: true},
		StagePreparing:    {poll: 3 * time.Second, preparation: true, connected: true},
		StageInGame:       {poll: 20 * time.Second, connected: true, frozen: true},
		StageIdle:         {poll: 6 * time.Second, connected: true},
	}
	for stage, want := range tests {
		if got := behaviorForStage(stage); got != want {
			t.Errorf("behaviorForStage(%q) = %+v, want %+v", stage, got, want)
		}
	}
	if got := behaviorForStage(Stage("future")); got != tests[StageDisconnected] {
		t.Fatalf("unknown stage did not fail closed: %+v", got)
	}
}

func TestTypedValuesKeepExternalStrings(t *testing.T) {
	if string(StageDisconnected) != "disconnected" || string(StageParty) != "party" || string(StagePreparing) != "preparing" || string(StageInGame) != "in-game" || string(StageIdle) != "idle" {
		t.Fatal("stage JSON values changed")
	}
	if string(PhaseNone) != "None" || string(PhaseLobby) != "Lobby" || string(PhaseChampSelect) != "ChampSelect" || string(PhaseGameStart) != "GameStart" || string(PhaseInProgress) != "InProgress" || string(PhaseEndOfGame) != "EndOfGame" {
		t.Fatal("phase JSON values changed")
	}
}
