package engine

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// scoutPreparation owns the bounded round, reports and resumable cursors.
// Engine validates live scope/identity first; lock order is Engine -> mu.
// Report/cursor values are immutable after publication. No I/O or callbacks
// run while mu is held.
type scoutPreparation struct {
	mu          sync.RWMutex
	cancel      context.CancelFunc
	token       uint64
	scope       string
	attempted   map[string]bool
	cache       map[string]*core.ScoutReport
	progress    map[string]*scoutProgress
	queue       int
	manual      bool
	deadline    time.Time
	pages       int
	lastPublish time.Time
}

func (s *scoutPreparation) selectQueue(queue int) {
	s.mu.Lock()
	s.queue, s.manual = queue, true
	s.mu.Unlock()
}

func (s *scoutPreparation) queueFor(mode int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue != 420 && s.queue != 440 {
		s.queue = 420
	}
	if !s.manual && (mode == 420 || mode == 440) {
		s.queue = mode
	}
	return s.queue
}

func (s *scoutPreparation) stop(players []core.Participant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked(players)
}

func (s *scoutPreparation) stopLocked(players []core.Participant) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.token++
	for i := range players {
		if r := players[i].Scout; r != nil && r.State == "loading" {
			copy := *r
			copy.State, copy.Message = "partial", "已保留读取结果"
			players[i].Scout = &copy
			if s.cache != nil {
				s.cache[players[i].PUUID] = &copy
			}
		}
	}
}

func (s *scoutPreparation) scopeFor(scope string, mode int, players []core.Participant) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scope != scope {
		s.stopLocked(players)
		s.scope = scope
		s.attempted = map[string]bool{}
		s.deadline = time.Time{}
		s.pages, s.manual = 0, false
		if mode == 420 || mode == 440 {
			s.queue = mode
		}
	}
	return s.queue
}

func (s *scoutPreparation) hasTask() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil
}

// targets updates only the caller's Engine-locked roster, never publishes it.
// Cache-only rows count as attempted, preserving the ten-identity round cap.
func (s *scoutPreparation) targets(players []core.Participant, now time.Time) ([]core.Participant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = map[string]*core.ScoutReport{}
	}
	if s.cancel != nil || !s.deadline.IsZero() && now.After(s.deadline) || s.pages >= scoutPageLimit {
		return nil, false
	}
	var targets []core.Participant
	changed := false
	for i := range players {
		p := &players[i]
		if p.IdentityState != core.IdentityKnown || !core.IsKnownPUUID(p.PUUID) || s.attempted[p.PUUID] || len(s.attempted) >= 10 {
			continue
		}
		s.attempted[p.PUUID] = true
		if cached := s.cache[p.PUUID]; cached != nil && cached.State != "loading" && cached.Complete30 && now.Sub(cached.FetchedAt) < 5*time.Minute && (cached.Season == nil || now.Before(cached.Season.End)) {
			changed = changed || p.Scout != cached
			p.Scout = cached
			continue
		}
		if len(targets) >= 10 {
			continue
		}
		r := core.ScoutReport{State: "loading", Champions: []core.ScoutChampion{}, Queues: []core.ScoutQueue{}}
		if p.Scout != nil {
			r = *p.Scout
			r.State = "loading"
		}
		p.Scout = &r
		changed = true
		targets = append(targets, *p)
	}
	return targets, changed
}

func (s *scoutPreparation) begin(parent context.Context, now time.Time) (context.Context, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadline.IsZero() {
		s.deadline = now.Add(scoutRoundBudget)
	}
	ctx, cancel := context.WithDeadline(parent, s.deadline)
	s.cancel = cancel
	s.token++
	return ctx, s.token
}

func (s *scoutPreparation) current(scope string, token uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scope == scope && s.token == token
}

func (s *scoutPreparation) finish(scope string, token uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scope != scope || s.token != token {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
}

func (s *scoutPreparation) rememberReport(id string, report *core.ScoutReport, first bool, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= 64 {
		s.cache = map[string]*core.ScoutReport{}
	}
	s.cache[id] = report
	if first || report.State != "loading" || now.Sub(s.lastPublish) >= time.Second {
		s.lastPublish = now
		return true
	}
	return false
}

func (s *scoutPreparation) pageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pages
}

func (s *scoutPreparation) recordPages(pages int) {
	s.mu.Lock()
	s.pages = pages
	s.mu.Unlock()
}

func (s *scoutPreparation) clearProgress() {
	s.mu.Lock()
	s.progress = nil
	s.mu.Unlock()
}

func (s *scoutPreparation) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scope, s.cache, s.progress, s.attempted = "", nil, nil, nil
}

func (s *scoutPreparation) refresh(players []core.Participant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempted, s.deadline, s.pages = map[string]bool{}, time.Time{}, 0
	for _, p := range players {
		delete(s.cache, p.PUUID)
	}
}

func (s *scoutPreparation) resume(player core.Participant, now time.Time) scoutCursor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := scoutCursor{player: player, seen: map[int64]bool{}}
	progress := s.progress[player.PUUID]
	if !progress.fresh(now) {
		return c
	}
	c.matches = append([]core.Match(nil), progress.matches...)
	for id := range progress.seen {
		c.seen[id] = true
	}
	c.offset, c.oldest = progress.offset, progress.oldest
	c.complete, c.twelve, c.month = progress.complete, progress.twelve, progress.month
	return c
}

func (s *scoutPreparation) rememberProgress(c *scoutCursor, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.progress == nil || len(s.progress) >= 16 {
		next := map[string]*scoutProgress{}
		for id, p := range s.progress {
			if p.fresh(now) {
				next[id] = p
			}
		}
		s.progress = next
	}
	seen := make(map[int64]bool, len(c.seen))
	for id := range c.seen {
		seen[id] = true
	}
	s.progress[c.player.PUUID] = &scoutProgress{
		matches: append([]core.Match(nil), c.matches...), seen: seen, offset: c.offset,
		oldest: c.oldest, complete: c.complete, twelve: c.twelve, month: c.month, at: now,
	}
}
