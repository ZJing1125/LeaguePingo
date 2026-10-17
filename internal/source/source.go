package source

import (
	"context"
	"errors"

	"leaguepingo/internal/core"
)

var ErrNotConnected = errors.New("战绩数据源未连接")

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

type HistoryPager interface {
	HistoryPage(context.Context, string, int) (core.HistoryPage, error)
}

type MatchDetailSource interface {
	MatchDetail(context.Context, int64, string) (core.MatchDetail, error)
}

type LobbySource interface {
	Lobby(context.Context) ([]core.Participant, error)
}

// Optional local account check, used only outside active gameplay.
type AccountSource interface {
	AccountIdentity(context.Context) (string, error)
}

// Optional phase-only notifications. The callback is a wake-up hint; the engine
// still verifies state through Phase. Implementations block until cancelled or failed.
type PhaseEventSource interface {
	WatchPhase(context.Context, func(string)) error
}
