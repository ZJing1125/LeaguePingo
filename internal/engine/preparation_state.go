package engine

import (
	"context"
	"sort"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// preparationState owns one party/selection preparation window. Engine keeps
// connection/phase/roster validation, and always locks Engine before this state.
// No source access, engine callback or publication occurs under mu.
type preparationState struct {
	mu        sync.RWMutex
	cancel    context.CancelFunc
	token     uint64
	running   bool
	deadline  time.Time
	retryAt   time.Time
	attempts  map[string]int
	profiles  map[string]bool
	names     map[string]bool
	preferred string
}

// stop cancels work without renewing the same session's budget or attempts.
func (p *preparationState) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *preparationState) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetLocked()
	p.attempts, p.profiles, p.names = map[string]int{}, map[string]bool{}, map[string]bool{}
}

func (p *preparationState) invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetLocked()
	p.cancel, p.attempts, p.profiles, p.names = nil, nil, nil, nil
}

func (p *preparationState) resetLocked() {
	if p.cancel != nil {
		p.cancel()
	}
	p.token++
	p.running = false
	p.deadline, p.retryAt = time.Time{}, time.Time{}
}

func (p *preparationState) forgetCancel() {
	p.mu.Lock()
	p.cancel = nil
	p.mu.Unlock()
}

// A newly joined party member extends the available window for later resumes;
// an already-running context retains its original deadline, as before.
func (p *preparationState) extend(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deadline = now.Add(45 * time.Second)
	if p.attempts == nil {
		p.attempts = map[string]int{}
	}
	if p.profiles == nil {
		p.profiles = map[string]bool{}
	}
}

func (p *preparationState) prioritize(key string) {
	p.mu.Lock()
	p.preferred = key
	p.mu.Unlock()
}

func (p *preparationState) current(token uint64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.token == token
}

func (p *preparationState) begin(players []core.Participant, version uint64, profileEligible []bool, now time.Time) (context.Context, context.CancelFunc, uint64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running || now.Before(p.retryAt) {
		return nil, nil, 0, false
	}
	if p.deadline.IsZero() {
		p.deadline = now.Add(45 * time.Second)
		p.attempts, p.profiles, p.names = map[string]int{}, map[string]bool{}, map[string]bool{}
	}
	if !now.Before(p.deadline) {
		return nil, nil, 0, false
	}
	for i, player := range players {
		profile := i < len(profileEligible) && profileEligible[i] && p.needsProfileLocked(player.PUUID)
		if player.ConnectionID == version && core.IsKnownPUUID(player.PUUID) && (p.needsHistoryLocked(player) || profile) {
			ctx, cancel := context.WithDeadline(context.Background(), p.deadline)
			p.cancel, p.running = cancel, true
			return ctx, cancel, p.token, true
		}
	}
	return nil, nil, 0, false
}

func (p *preparationState) needsHistoryLocked(player core.Participant) bool {
	return !(historyCached(player) && cacheFresh(player) && !player.Stale) && p.attempts[player.PUUID] < 2
}

func (p *preparationState) needsProfileLocked(key string) bool {
	return len(p.profiles) < 10 && !p.profiles[key]
}

// players is the caller's private roster copy, so sorting never changes the
// published roster or cached records. Attempts sort before the preferred key.
func (p *preparationState) nextHistory(players []core.Participant, version uint64) core.Participant {
	p.mu.RLock()
	defer p.mu.RUnlock()
	sort.SliceStable(players, func(i, j int) bool {
		a, b := p.attempts[players[i].PUUID], p.attempts[players[j].PUUID]
		if a != b {
			return a < b
		}
		return players[i].Key == p.preferred && players[j].Key != p.preferred
	})
	for _, player := range players {
		if player.ConnectionID == version && core.IsKnownPUUID(player.PUUID) && p.needsHistoryLocked(player) {
			return player
		}
	}
	return core.Participant{}
}

func (p *preparationState) claimHistory(token uint64, key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != token {
		return false
	}
	p.attempts[key]++
	return true
}

func (p *preparationState) claimProfile(token uint64, players []core.Participant, eligible []bool) core.Participant {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != token {
		return core.Participant{}
	}
	for i, player := range players {
		if i < len(eligible) && eligible[i] && p.needsProfileLocked(player.PUUID) {
			p.profiles[player.PUUID] = true
			return player
		}
	}
	return core.Participant{}
}

func (p *preparationState) retryAfterPhaseFailure(token uint64, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token {
		p.retryAt = now.Add(2 * time.Second)
	}
}

func (p *preparationState) refundHistory(token uint64, key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token && p.attempts[key] > 0 {
		p.attempts[key]--
	}
}

func (p *preparationState) finish(token uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != token {
		return false
	}
	p.running = false
	return true
}

// Name readiness is independent of history freshness, within the same window.
func (p *preparationState) claimName(token uint64, key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != token || p.names[key] {
		return false
	}
	if p.names == nil {
		p.names = map[string]bool{}
	}
	p.names[key] = true
	return true
}
