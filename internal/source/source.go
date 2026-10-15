package source

import (
	"context"
	"errors"

	"leaguepingo/internal/core"
)

var ErrNotConnected = errors.New("战绩数据源未连接")

// Source is the game-data boundary. A future Riot international implementation
// can supply its own identity and history API without changing the engine.
type Source interface {
	Region() string
	Phase(context.Context) (string, error)
	Roster(context.Context) (int64, []core.Participant, error)
	Name(context.Context, string) (string, error)
	History(context.Context, string) ([]core.Match, error)
	FindByRiotID(context.Context, string) (core.Participant, error)
}
