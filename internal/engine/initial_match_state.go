package engine

import (
	"context"
	"sync"
)

// initialMatchPreparation owns the one-shot entry batch. Engine validates the
// live connection/phase/roster first, always taking its lock before this one.
// No source I/O, engine publication or callback runs under mu.
type initialMatchPreparation struct {
	mu                sync.RWMutex
	parent            context.Context
	eligible, started bool
	version           uint64
	token             uint64
	gameID            int64
	cancel            context.CancelFunc
}

type initialMatchScope struct {
	version uint64
	gameID  int64
	token   uint64
}

func (s *initialMatchPreparation) setParent(parent context.Context) {
	s.mu.Lock()
	s.parent = parent
	s.mu.Unlock()
}

// Run's context also parents grouping, scouting and history recovery. It
// outlives the entry batch and must never be replaced by its 45-second child.
func (s *initialMatchPreparation) parentContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.parent
}

func (s *initialMatchPreparation) game() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gameID
}

// stop consumes eligibility without resetting the game or the started flag.
// A reconnect to this same game must not grant another entry batch.
func (s *initialMatchPreparation) stop() (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stopped := s.cancel != nil
	if stopped {
		s.cancel()
	}
	s.eligible = false
	return s.version, stopped
}

// Engine stops the previous batch and cleans its loading rows before entering
// a new game. Retain the cancelled handle until begin/finish, as before.
func (s *initialMatchPreparation) enterGame(gameID int64, version uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gameID == gameID {
		return
	}
	s.token++
	s.gameID, s.version = gameID, version
	s.started, s.eligible = false, true
}

// claim is called only after gameplay and bounded roster discovery are ready.
// Claiming an empty target set still consumes this game's one-shot batch.
func (s *initialMatchPreparation) claim(version uint64, gameID int64) (initialMatchScope, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.eligible || s.started || s.version != version || s.gameID != gameID {
		return initialMatchScope{}, false
	}
	s.started = true
	return initialMatchScope{version: s.version, gameID: s.gameID, token: s.token}, true
}

func (s *initialMatchPreparation) matchesLocked(scope initialMatchScope) bool {
	return s.version == scope.version && s.gameID == scope.gameID && s.token == scope.token
}

func (s *initialMatchPreparation) begin(scope initialMatchScope) (context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.matchesLocked(scope) || !s.eligible || !s.started || s.parent == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(s.parent, initialMatchBudget)
	s.cancel = cancel
	return ctx, true
}

func (s *initialMatchPreparation) empty(scope initialMatchScope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.matchesLocked(scope) {
		s.eligible = false
	}
}

func (s *initialMatchPreparation) valid(version uint64, gameID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eligible && s.cancel != nil && s.version == version && s.gameID == gameID
}

func (s *initialMatchPreparation) running(gameID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil && s.eligible && s.gameID == gameID
}

func (s *initialMatchPreparation) hasTask() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil
}

// A late worker cannot cancel or consume the next game's scope.
func (s *initialMatchPreparation) finish(scope initialMatchScope) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.matchesLocked(scope) {
		return false
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.eligible = false
	return true
}
