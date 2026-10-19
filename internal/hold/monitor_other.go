//go:build !windows

package hold

import "errors"

type Monitor struct{ State *State }

func Start(*State) (*Monitor, error) { return nil, errors.New("局内按住快览仅支持 Windows") }
func (m *Monitor) Stop()             {}
func GameForeground() bool           { return false }
