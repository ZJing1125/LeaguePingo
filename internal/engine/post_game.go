package engine

import (
	"leaguepingo/internal/postgame"
	"leaguepingo/internal/source"
)

func (e *Engine) observePostGameLocked() {
	s := e.snapshot
	client, _ := e.client.(source.PostGameSource)
	if !s.Connected || s.ConnectionID != e.clientVersion {
		e.postGame.Observe(nil, postgame.Observation{})
		return
	}
	o := postgame.Observation{Version: s.ConnectionID, Account: s.SelfKey, Phase: s.Phase, GameID: s.GameID, QueueID: s.QueueID}
	if e.review != nil && e.review.ConnectionID == s.ConnectionID && e.review.SelfKey == s.SelfKey {
		o.ReviewID = e.review.GameID
		o.ReviewState = e.review.State
	}
	e.postGame.Observe(client, o)
}
func (e *Engine) ConfigurePostGame(ret, queue bool, delay int, skipHonor ...bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.postGame.Configure(ret, queue, delay, skipHonor...); err != nil {
		return err
	}
	e.observePostGameLocked()
	return nil
}
func (e *Engine) ConfigureSkipHonor(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.postGame.ConfigureSkipHonor(enabled)
	e.observePostGameLocked()
}
func (e *Engine) PostGameStatus() postgame.State          { return e.postGame.Snapshot() }
func (e *Engine) PostGameChanges() <-chan postgame.State  { return e.postGame.Changes() }
func (e *Engine) CancelPostGame() (postgame.State, error) { return e.postGame.Cancel() }
