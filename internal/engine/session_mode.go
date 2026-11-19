package engine

import (
	"context"
	"slices"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func readStageRoster(ctx context.Context, client Client, stage Stage, previous source.SessionMetadata) (int64, []core.Participant, source.SessionMetadata, error) {
	if local, ok := client.(source.SessionRosterSource); ok {
		switch stage {
		case StageParty:
			players, mode, err := local.ReadLobby(ctx)
			return 0, players, mode, err
		case StagePreparing:
			return local.ReadSelection(ctx, previous)
		default:
			return local.ReadGame(ctx)
		}
	}
	if stage == StageParty {
		if lobby, ok := client.(source.LobbySource); ok {
			players, err := lobby.Lobby(ctx)
			return 0, players, source.SessionMetadata{}, err
		}
		return 0, nil, source.SessionMetadata{}, apperrors.New(apperrors.Unsupported)
	}
	if stage == StagePreparing {
		if selection, ok := client.(source.ChampSelectSource); ok {
			id, players, err := selection.ChampSelectRoster(ctx)
			return id, players, source.SessionMetadata{}, err
		}
	}
	id, players, err := client.Roster(ctx)
	return id, players, source.SessionMetadata{}, err
}

// The engine updates its roster while preparing player data. A source may
// return the same slice on successive reads, so take ownership before editing.
func rosterForConnection(players []core.Participant, version uint64) []core.Participant {
	roster := slices.Clone(players)
	for i := range roster {
		roster[i].ConnectionID = version
	}
	return roster
}

func (e *Engine) selectionModeLocked(next Stage) source.SessionMetadata {
	if next == StagePreparing && e.snapshot.ConnectionID == e.clientVersion {
		switch snapshotStage(e.snapshot.Stage) {
		case StageParty, StagePreparing:
			return e.session
		}
	}
	return source.SessionMetadata{}
}

// The caller has already verified the connection, phase, and response epoch.
func (e *Engine) updateSessionLocked(stage Stage, gameID int64, next source.SessionMetadata, read bool, err error) bool {
	old := e.session
	newSelection := stage == StagePreparing && e.newSelectionLocked(next)
	sameConnection := e.snapshot.ConnectionID == e.clientVersion
	sameGame := gameID == 0 || e.snapshot.GameID == 0 || e.snapshot.GameID == gameID
	sameStage := sameConnection && sameGame && snapshotStage(e.snapshot.Stage) == stage
	switch {
	case stage == StageIdle || stage == StageDisconnected:
		e.session = source.SessionMetadata{}
	case read && err == nil:
		// Keep the verified selection mode during loading if the sparse gameflow
		// response doesn't have a queue yet. Don't carry metadata between games.
		if stage == StageInGame && next.QueueID == 0 && sameConnection && sameGame &&
			(sameStage || snapshotStage(e.snapshot.Stage) == StagePreparing) {
			break
		}
		e.session = next
	case !sameStage:
		e.session = source.SessionMetadata{}
	}
	if read && err == nil && (newSelection || stage == StageParty && old.RoomKey != e.session.RoomKey) {
		e.scouting.selectQueue(0)
	}
	return old != e.session
}

func (e *Engine) newSelectionLocked(next source.SessionMetadata) bool {
	return e.session.SelectionKey != "" && next.SelectionKey != "" && e.session.SelectionKey != next.SelectionKey
}

func (e *Engine) modeDiscoveredLocked() bool {
	_, scoped := e.client.(source.SessionRosterSource)
	return !scoped || e.session.QueueID > 0
}

func (e *Engine) applySessionLocked() {
	mode := e.session
	// The optional legacy queue reader also supports sources that supply no
	// stage metadata. Tencent always uses the checked result above.
	if _, scoped := e.client.(source.SessionRosterSource); scoped {
		e.snapshot.QueueID, e.snapshot.MapID = mode.QueueID, mode.MapID
		e.snapshot.GameMode, e.snapshot.GameVersion = mode.GameMode, mode.Version
	} else if q, ok := e.client.(source.QueueSource); ok {
		e.snapshot.QueueID = q.CurrentQueue()
	}
	e.snapshot.ScoutQueue = e.scouting.queueFor(e.scoutModeLocked())
}
