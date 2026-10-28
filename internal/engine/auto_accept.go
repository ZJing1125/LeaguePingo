package engine

import (
	"time"

	"leaguepingo/internal/readycheck"
	"leaguepingo/internal/source"
)

func (e *Engine) observeAutoAcceptLocked(phase string, at ...time.Time) {
	client, _ := e.client.(source.ReadyCheckSource)
	if len(at) > 0 {
		e.autoAccept.ObserveAt(client, e.clientVersion, phase, at[0])
	} else {
		e.autoAccept.Observe(client, e.clientVersion, phase)
	}
}
func (e *Engine) ConfigureAutoAccept(enabled bool, delay int) error {
	return e.autoAccept.Configure(enabled, delay)
}
func (e *Engine) AutoAcceptStatus() readycheck.State         { return e.autoAccept.Snapshot() }
func (e *Engine) AutoAcceptChanges() <-chan readycheck.State { return e.autoAccept.Changes() }
func (e *Engine) SkipAutoAccept() (readycheck.State, error)  { return e.autoAccept.Skip() }
