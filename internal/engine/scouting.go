package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

const scoutRoundBudget = 180 * time.Second
const scoutPageLimit = 500 // at most 10,000 source records across the whole round

type scoutPreparation struct {
	cancel      context.CancelFunc
	token       uint64
	scope       string
	attempted   map[string]bool
	cache       map[string]*core.ScoutReport
	queue       int
	manual      bool
	deadline    time.Time
	pages       int
	lastPublish time.Time
}

func (e *Engine) SetScoutQueue(queue int) (core.Snapshot, error) {
	if queue != 420 && queue != 440 {
		return e.Snapshot(), errors.New("请选择单双排或灵活组排")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scouting.queue, e.scouting.manual = queue, true
	e.snapshot.ScoutQueue = queue
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return clone(e.snapshot), nil
}

func (e *Engine) stopScoutingLocked() {
	if e.scouting.cancel != nil {
		e.scouting.cancel()
		e.scouting.cancel = nil
	}
	e.scouting.token++
	for i := range e.snapshot.Participants {
		if r := e.snapshot.Participants[i].Scout; r != nil && r.State == "loading" {
			copy := *r
			copy.State = "partial"
			copy.Message = "已保留读取结果"
			e.snapshot.Participants[i].Scout = &copy
			if e.scouting.cache != nil {
				e.scouting.cache[e.snapshot.Participants[i].PUUID] = &copy
			}
		}
	}
}

func (e *Engine) syncScoutingLocked() {
	s := &e.scouting
	if e.client == nil || !e.snapshot.Connected {
		if s.cancel != nil {
			e.stopScoutingLocked()
		}
		return
	}
	oldQueue, oldMode := e.snapshot.ScoutQueue, e.snapshot.QueueID
	reportChanged := false
	defer func() {
		if reportChanged || oldQueue != e.snapshot.ScoutQueue || oldMode != e.snapshot.QueueID {
			e.publishLocked()
		}
	}()
	if s.queue != 420 && s.queue != 440 {
		s.queue = 420
	}
	if q, ok := e.client.(source.QueueSource); ok {
		e.snapshot.QueueID = q.CurrentQueue()
	}
	if !s.manual && (e.snapshot.QueueID == 420 || e.snapshot.QueueID == 440) {
		s.queue = e.snapshot.QueueID
	}
	e.snapshot.ScoutQueue = s.queue
	if e.client == nil || !e.snapshot.Connected || e.initialMatch.parent == nil || e.initialMatch.parent.Err() != nil || (!preparationStage(e.snapshot.Stage) && !activeMatch(e.snapshot.Phase)) || e.snapshot.Phase == "GameStart" || e.phaseHint != "" && stageForPhase(e.phaseHint) != e.snapshot.Stage {
		if s.cancel != nil {
			e.stopScoutingLocked()
		}
		return
	}
	src, ok := e.client.(source.ScoutSource)
	if !ok {
		return
	}
	if e.snapshot.Stage == "in-game" && !completeRoster(e.snapshot.Participants) && e.matchRosterAttempts < inGameRosterAttemptLimit {
		return
	}
	scope := fmt.Sprintf("%d:%s:%d", e.clientVersion, e.snapshot.Stage, e.snapshot.GameID)
	if s.scope != scope {
		e.stopScoutingLocked()
		s.scope = scope
		s.attempted = map[string]bool{}
		s.deadline = time.Time{}
		s.pages = 0
		s.manual = false
		if e.snapshot.QueueID == 420 || e.snapshot.QueueID == 440 {
			s.queue = e.snapshot.QueueID
		}
		e.snapshot.ScoutQueue = s.queue
	}
	if s.cache == nil {
		s.cache = map[string]*core.ScoutReport{}
	}
	if s.cancel != nil {
		return
	}
	if !s.deadline.IsZero() && time.Now().After(s.deadline) {
		return
	}
	if s.pages >= scoutPageLimit {
		return
	}
	var targets []core.Participant
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.IdentityState != core.IdentityKnown || !core.IsKnownPUUID(p.PUUID) || s.attempted[p.PUUID] {
			continue
		}
		if len(s.attempted) >= 10 {
			continue
		}
		s.attempted[p.PUUID] = true
		if cached := s.cache[p.PUUID]; cached != nil && cached.State != "loading" && cached.Complete30 && time.Since(cached.FetchedAt) < 5*time.Minute && (cached.Season == nil || time.Now().Before(cached.Season.End)) {
			if p.Scout != cached {
				reportChanged = true
			}
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
		reportChanged = true
		targets = append(targets, *p)
	}
	if len(targets) == 0 {
		return
	}
	if s.deadline.IsZero() {
		s.deadline = time.Now().Add(scoutRoundBudget)
	}
	ctx, cancel := context.WithDeadline(e.initialMatch.parent, s.deadline)
	s.cancel = cancel
	s.token++
	token, version := s.token, e.clientVersion
	go e.prepareScouting(ctx, version, scope, token, src, targets)
}

func (e *Engine) scoutValidLocked(version uint64, scope string, token uint64) bool {
	return e.client != nil && e.snapshot.Connected && e.clientVersion == version && e.scouting.scope == scope && e.scouting.token == token && (e.phaseHint == "" || stageForPhase(e.phaseHint) == e.snapshot.Stage)
}
func (e *Engine) saveScout(version uint64, scope string, token uint64, id string, r core.ScoutReport) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.scoutValidLocked(version, scope, token) {
		return false
	}
	found := false
	firstData := false
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.PUUID == id && p.ConnectionID == version {
			// Refresh progress/failure must not replace a usable earlier report
			// with an empty or shorter range. Keep its original cutoff, explicitly
			// mark retention, and never carry a verified old season into a new one.
			if old := p.Scout; old != nil && r.State != "ready" && old.Read > r.Read && (old.Complete30 || r.Read == 0) {
				sameSeason := old.Season == nil && r.Season == nil || old.Season != nil && r.Season != nil && old.Season.Start.Equal(r.Season.Start) || old.Season != nil && r.Season == nil && !r.FetchedAt.Before(old.Season.Start) && r.FetchedAt.Before(old.Season.End)
				if sameSeason {
					state := r.State
					r = *old
					r.State = state
					r.Retained = true
					r.Message = "保留上次统计"
				}
			}
			if p.Scout == nil || p.Scout.FetchedAt.IsZero() || p.Scout.Read == 0 && r.Read > 0 {
				firstData = true
			}
			p.Scout = &r
			found = true
		}
	}
	if found {
		if len(e.scouting.cache) >= 64 {
			e.scouting.cache = map[string]*core.ScoutReport{}
		}
		e.scouting.cache[id] = &r
		e.snapshot.UpdatedAt = time.Now()
		if firstData || r.State != "loading" || time.Since(e.scouting.lastPublish) >= time.Second {
			e.scouting.lastPublish = time.Now()
			e.publishLocked()
		}
	}
	return found
}

type scoutCursor struct {
	player                                 core.Participant
	masteries                              []core.ChampionMastery
	matches                                []core.Match
	seen                                   map[int64]bool
	offset                                 int
	done, complete, twelve, month, invalid bool
	oldest                                 time.Time
	message                                string
}

func (e *Engine) prepareScouting(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, targets []core.Participant) {
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.scoutValidLocked(version, scope, token) {
			if e.scouting.cancel != nil {
				e.scouting.cancel()
			}
			e.scouting.cancel = nil
			e.publishLocked()
		}
	}()
	cutoff := time.Now().UTC()
	var season *core.ScoutSeason
	for ctx.Err() == nil {
		automaticCtx, release, err := e.scheduler.acquireAutomatic(ctx)
		if err != nil {
			break
		}
		request, cancel := context.WithTimeout(automaticCtx, 3*time.Second)
		season, _ = src.ScoutSeason(request)
		cancel()
		interrupted := automaticCtx.Err() != nil && ctx.Err() == nil
		release()
		if !interrupted {
			break
		}
	}
	e.mu.Lock()
	if e.scoutValidLocked(version, scope, token) {
		e.syncScoutingLocked()
	}
	e.mu.Unlock()
	cursors := make([]scoutCursor, len(targets))
	for i, p := range targets {
		cursors[i] = scoutCursor{player: p, seen: map[int64]bool{}}
	}
	boundary := cutoff.Add(-30 * 24 * time.Hour)
	if season != nil && season.Start.Before(boundary) {
		boundary = season.Start
	}
	e.mu.RLock()
	count := e.scouting.pages
	e.mu.RUnlock()
	for count < scoutPageLimit && ctx.Err() == nil {
		remaining := false
		for i := range cursors {
			c := &cursors[i]
			if c.done {
				continue
			}
			remaining = true
			e.mu.RLock()
			valid := e.scoutValidLocked(version, scope, token)
			present := false
			for _, p := range e.snapshot.Participants {
				if p.PUUID == c.player.PUUID {
					present = true
				}
			}
			e.mu.RUnlock()
			if !valid {
				return
			}
			if !present {
				c.done = true
				continue
			}
			// Deliver each player's first page before spending up to twenty
			// seconds on everybody else's mastery. No champion-lock dependency
			// and no extra requests: the same bounded round fills all heroes.
			if c.offset == 0 {
				if automaticCtx, release, err := e.scheduler.acquireAutomatic(ctx); err == nil {
					request, cancel := context.WithTimeout(automaticCtx, 2*time.Second)
					c.masteries, _ = src.ChampionMasteries(request, c.player.PUUID)
					cancel()
					interrupted := automaticCtx.Err() != nil && ctx.Err() == nil
					release()
					if interrupted {
						continue
					}
				}
				initial := core.BuildScoutReport(nil, c.masteries, season, cutoff, false, false, false)
				initial.State = "loading"
				if !e.saveScout(version, scope, token, c.player.PUUID, initial) {
					c.done = true
					continue
				}
			}
			automaticCtx, release, err := e.scheduler.acquireAutomatic(ctx)
			if err != nil {
				break
			}
			request, cancel := context.WithTimeout(automaticCtx, 4*time.Second)
			page, err := src.ScoutPage(request, c.player.PUUID, c.offset)
			cancel()
			interrupted := automaticCtx.Err() != nil && ctx.Err() == nil
			release()
			if interrupted {
				continue
			}
			count++
			e.mu.Lock()
			if e.scoutValidLocked(version, scope, token) {
				e.scouting.pages = count
			}
			e.mu.Unlock()
			if err != nil {
				c.done = true
				c.message = "已保留读取结果"
			} else {
				if page.Offset != c.offset || page.Meta.Returned < 0 || page.Meta.Returned > 20 || page.NextOffset != c.offset+page.Meta.Returned || page.LimitReason != "" {
					c.done = true
					c.invalid = true
					c.message = "当前记录范围受限"
				}
				added := 0
				for _, m := range page.Matches {
					if c.seen[m.GameID] || m.GameID <= 0 {
						continue
					}
					c.seen[m.GameID] = true
					added++
					if m.PlayedAt == nil || m.PlayedAt.After(cutoff) {
						c.invalid = true
						continue
					}
					if !c.oldest.IsZero() && m.PlayedAt.After(c.oldest) {
						c.invalid = true
					}
					c.oldest = *m.PlayedAt
					c.matches = append(c.matches, m)
				}
				if page.Meta.Excluded > 0 {
					c.invalid = true
				}
				if page.Meta.Returned > 0 && added == 0 {
					c.done = true
					c.invalid = true
					c.message = "当前记录范围受限"
				}
				if !c.invalid {
					// Service exhaustion can be a retention boundary. A full
					// time range requires an actual record reaching its start.
					c.twelve = !c.oldest.IsZero() && !c.oldest.After(cutoff.Add(-12*time.Hour))
					c.month = !c.oldest.IsZero() && !c.oldest.After(cutoff.Add(-30*24*time.Hour))
					c.complete = season != nil && !c.oldest.IsZero() && !c.oldest.After(season.Start)
				}
				if !page.HasMore || !c.oldest.IsZero() && !c.oldest.After(boundary) || page.NextOffset >= 20000 {
					c.done = true
				}
				c.offset = page.NextOffset
			}
			if c.invalid {
				c.complete, c.twelve, c.month = false, false, false
				c.done = true
			}
			r := core.BuildScoutReport(c.matches, c.masteries, season, cutoff, c.complete, c.twelve, c.month)
			if !c.done {
				r.State = "loading"
			} else if !r.Complete || !r.Complete30 {
				r.State = "partial"
			}
			r.Message = c.message
			if !e.saveScout(version, scope, token, c.player.PUUID, r) {
				c.done = true
			}
			timer := time.NewTimer(150 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
			if count >= scoutPageLimit || ctx.Err() != nil {
				break
			}
		}
		if !remaining {
			break
		}
	}
	for i := range cursors {
		c := &cursors[i]
		if !c.done {
			r := core.BuildScoutReport(c.matches, c.masteries, season, cutoff, c.complete, c.twelve, c.month)
			r.State = "partial"
			r.Message = "已保留读取结果"
			e.saveScout(version, scope, token, c.player.PUUID, r)
		}
	}
}
