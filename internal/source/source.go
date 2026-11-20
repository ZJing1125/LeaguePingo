package source

import (
	"context"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

var ErrNotConnected = apperrors.New(apperrors.ClientNotRunning)

// SessionSource owns local client state; remote player data can be composed separately.
type SessionSource interface {
	Region() string
	Phase(context.Context) (string, error)
	Roster(context.Context) (int64, []core.Participant, error)
}
type PlayerSource interface {
	Name(context.Context, string) (string, error)
	History(context.Context, string) (core.HistoryResult, error)
	FindByRiotID(context.Context, string) (core.Participant, error)
}
type Source interface {
	SessionSource
	PlayerSource
}

// Optional enrichment must not make a valid history lookup fail.
type ProfileSource interface {
	Profile(context.Context, string) core.Profile
}

// Explicit opening of player details needs the avatar/level, not ranked data.
type BasicProfileSource interface {
	BasicProfile(context.Context, string) core.Profile
}

// An explicit rank retry must not spend its budget reloading names or avatars.
type RankSource interface {
	Ranks(context.Context, string) core.Profile
}

type HistoryPager interface {
	HistoryPage(context.Context, string, int) (core.HistoryPage, error)
}

// Independent season/time-window preparation; never changes recent-history pages.
type ScoutSource interface {
	ScoutSeason(context.Context) (*core.ScoutSeason, error)
	ChampionMasteries(context.Context, string) ([]core.ChampionMastery, error)
	ScoutPage(context.Context, string, int) (core.HistoryPage, error)
}

// Queue identity already obtained while reading the local session/lobby.
type QueueSource interface{ CurrentQueue() int }

type MatchDetailSource interface {
	MatchDetail(context.Context, int64, string) (core.MatchDetail, error)
}

type MatchTimelineSource interface {
	MatchTimeline(context.Context, core.MatchDetail, string) (core.MatchTimeline, error)
}

type LobbySource interface {
	Lobby(context.Context) ([]core.Participant, error)
}

// Optional local account check, used only outside active gameplay.
type AccountSource interface {
	AccountIdentity(context.Context) (string, error)
}

// CachedAccountSource exposes identity already verified by local roster reading.
// It must not make a network request, including during active gameplay.
type CachedAccountSource interface {
	CachedAccountIdentity() string
}

// Optional phase and lobby notifications are hints for checked local reads.
type SessionEventSource interface {
	WatchSession(context.Context, func(string), func()) error
}

// Optional phase-only notifications for sources without a lobby stream.
type PhaseEventSource interface {
	WatchPhase(context.Context, func(string)) error
}

// ReadyCheckSource is a narrowly scoped local action, separate from player data.
type ReadyCheckSource interface {
	Phase(context.Context) (string, error)
	ReadyCheck(context.Context) (core.ReadyCheck, error)
	AcceptReadyCheck(context.Context) error
}

type ReadyCheckEventSource interface {
	WatchReadyCheck(context.Context, func()) error
}

// Selection snapshots must come from champ-select, not the gameflow roster.
type ChampSelectSource interface {
	ChampSelectRoster(context.Context) (int64, []core.Participant, error)
}

type ChampSelectEventSource interface {
	WatchChampSelect(context.Context, func()) error
}

// PostGameSource exposes fixed local actions, never a generic mutation API.
type PostGameSource interface {
	Phase(context.Context) (string, error)
	AccountIdentity(context.Context) (string, error)
	LobbyControl(context.Context) (core.LobbyControl, error)
	PlayAgain(context.Context) error
	StartSearch(context.Context) error
}

// Optional, fixed local ceremony actions, with no player list or vote API.
type HonorSource interface {
	HonorBallot(context.Context) (gameID int64, hasSelections bool, err error)
	CompleteHonorBallot(context.Context) error
}

type HonorCompletionSource interface {
	HonorVoteCompletion(context.Context) (int64, error)
}

// Only the known post-game progress presentation can be continued; no generic
// sequence name or reward-selection API is exposed to the controller/UI.
type PostGameSequenceSource interface {
	PostGameSequence(context.Context) (string, error)
	ContinuePostGameProgress(context.Context) error
}

// Current-session metadata is obtained with existing roster/lobby reads.
// SessionRosterSource returns mode and roster from the same stage read.
type SessionRosterSource interface {
	ReadLobby(context.Context) ([]core.Participant, SessionMetadata, error)
	ReadSelection(context.Context, SessionMetadata) (int64, []core.Participant, SessionMetadata, error)
	ReadGame(context.Context) (int64, []core.Participant, SessionMetadata, error)
}

type SessionMetadata struct {
	RoomKey      string // transient lobby identity, never serialized
	SelectionKey string // transient selection identity, never serialized
	QueueID      int
	GameMode     string
	MapID        int
	Version      string
}
type GroupingSource interface {
	Grouping(context.Context, int64) ([]core.Participant, error)
}

// Queue reads use a separately verified scope and never populate mixed disk pages.
type QueueHistorySource interface {
	SupportsHistoryQueue(int) bool
	HistoryPageForQueue(context.Context, string, int, int) (core.HistoryPage, error)
	VerifyHistoryQueue(context.Context, int) (core.HistoryQueueVerification, error)
}
