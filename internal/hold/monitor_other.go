//go:build !windows

package hold

import "leaguepingo/internal/apperrors"

type Monitor struct{ State *State }

func Start(*State) (*Monitor, error) { return nil, apperrors.New(apperrors.HoldUnavailable) }
func (m *Monitor) Stop()             {}
func GameForeground() bool           { return false }
