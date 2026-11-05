package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/source"
)

const scoutRoundBudget = 180 * time.Second
const scoutPageLimit = 500 // at most 10,000 source records across the whole round
const scoutWorkers = 2     // two players read in parallel; the page cap still bounds the round

// Fixed reason codes carried to the interface. They are not error text; the
// front end maps known codes to static Chinese and falls back for the rest.
const (
	scoutReasonRangeLimited = "range_limited"
	scoutReasonIncomplete   = "incomplete"
)

// scoutPageFloor/Ceiling bound the adaptive page timeout. The floor keeps the
// historic four seconds for a healthy service; the ceiling stops one slow round
// from consuming a whole worker for minutes.
const (
	scoutPageFloor   = 4 * time.Second
	scoutPageCeiling = 8 * time.Second
)

// scoutPageTimeout follows the recent SGP page latency: twice the 95th
// percentile, never below the floor and never above the ceiling. A slower link
// therefore gets a longer page budget instead of failing every page.
func scoutPageTimeout(collector *diagnostics.Collector) time.Duration {
	timeout := scoutPageFloor
	if collector != nil {
		if p95, ok := collector.Percentile("sgp-history", 95); ok {
			if doubled := time.Duration(p95) * 2 * time.Millisecond; doubled > timeout {
				timeout = doubled
			}
		}
	}
	if timeout > scoutPageCeiling {
		timeout = scoutPageCeiling
	}
	return timeout
}

func (e *Engine) SetScoutQueue(queue int) (core.Snapshot, error) {
	if queue != 420 && queue != 440 {
		return e.Snapshot(), apperrors.New(apperrors.InvalidQueue)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scouting.selectQueue(queue)
	e.snapshot.ScoutQueue = queue
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return clone(e.snapshot), nil
}

func (e *Engine) stopScoutingLocked() {
	e.scouting.stop(e.snapshot.Participants)
}

func (e *Engine) syncScoutingLocked() {
	// A finished result changes the range, so it cannot resume a pre-result cursor.
	switch snapshotPhase(e.snapshot.Phase) {
	case PhaseWaitingForStats, PhasePreEndOfGame, PhaseEndOfGame:
		e.scouting.clearProgress()
	}
	if e.client == nil || !e.snapshot.Connected {
		if e.scouting.hasTask() {
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
	if q, ok := e.client.(source.QueueSource); ok {
		e.snapshot.QueueID = q.CurrentQueue()
	}
	e.snapshot.ScoutQueue = e.scouting.queueFor(e.snapshot.QueueID)
	parent := e.initialMatch.parentContext()
	if parent == nil || parent.Err() != nil || (!preparationStage(snapshotStage(e.snapshot.Stage)) && !activeMatch(snapshotPhase(e.snapshot.Phase))) || snapshotPhase(e.snapshot.Phase) == PhaseGameStart || e.phaseHint != PhaseUnknown && stageForPhase(e.phaseHint) != snapshotStage(e.snapshot.Stage) {
		if e.scouting.hasTask() {
			e.stopScoutingLocked()
		}
		return
	}
	src, ok := e.client.(source.ScoutSource)
	if !ok {
		return
	}
	if snapshotStage(e.snapshot.Stage) == StageInGame && !completeRoster(e.snapshot.Participants) && e.roster.view().attempts < inGameRosterAttemptLimit {
		return
	}
	scope := fmt.Sprintf("%d:%s:%d", e.clientVersion, e.snapshot.Stage, e.snapshot.GameID)
	e.snapshot.ScoutQueue = e.scouting.scopeFor(scope, e.snapshot.QueueID, e.snapshot.Participants)
	targets, changed := e.scouting.targets(e.snapshot.Participants, time.Now())
	reportChanged = changed
	if len(targets) == 0 {
		return
	}
	ctx, token := e.scouting.begin(parent, time.Now())
	go e.prepareScouting(ctx, e.clientVersion, scope, token, src, targets)
}

func (e *Engine) scoutValidLocked(version uint64, scope string, token uint64) bool {
	return e.client != nil && e.snapshot.Connected && e.clientVersion == version && e.scouting.current(scope, token) && (e.phaseHint == PhaseUnknown || stageForPhase(e.phaseHint) == snapshotStage(e.snapshot.Stage))
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
		now := time.Now()
		e.snapshot.UpdatedAt = now
		if e.scouting.rememberReport(id, &r, firstData, now) {
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
	pages                                  int
	pageFails                              int
	reason                                 string
	retryAt                                *time.Time
	done, complete, twelve, month, invalid bool
	oldest                                 time.Time
	message                                string
}

// scoutProgress is a bounded, memory-only read cursor kept for one connection.
// It lets a stage change continue an unfinished range instead of starting at the
// first page again; it expires with the same five-minute window as a report and
// is never written to disk.
type scoutProgress struct {
	matches                 []core.Match
	seen                    map[int64]bool
	offset                  int
	oldest                  time.Time
	complete, twelve, month bool
	at                      time.Time
}

func (p *scoutProgress) fresh(now time.Time) bool {
	return p != nil && now.Sub(p.at) < 5*time.Minute && p.offset > 0 && len(p.matches) <= 1000
}

func (e *Engine) resumeScoutCursor(p core.Participant) scoutCursor {
	return e.scouting.resume(p, time.Now())
}

func (e *Engine) rememberScoutProgress(version uint64, scope string, token uint64, c *scoutCursor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Cancellation can happen between publishing a page and saving its cursor.
	// Do not resurrect progress after a phase/connection/result reset.
	if e.scoutValidLocked(version, scope, token) {
		e.scouting.rememberProgress(c, time.Now())
	}
}

func (e *Engine) prepareScouting(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, targets []core.Participant) {
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.scoutValidLocked(version, scope, token) {
			e.scouting.finish(scope, token)
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
		cursors[i] = e.resumeScoutCursor(p)
	}
	boundary := cutoff.Add(-30 * 24 * time.Hour)
	if season != nil && season.Start.Before(boundary) {
		boundary = season.Start
	}
	count := e.scouting.pageCount()
	var pages atomic.Int64
	if count < scoutPageLimit {
		pages.Store(int64(count))
	} else {
		pages.Store(int64(scoutPageLimit))
	}
	workers := scoutWorkers
	if len(cursors) < workers {
		workers = len(cursors)
	}
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				e.runScoutCursor(ctx, version, scope, token, src, &cursors[i], season, cutoff, boundary, &pages)
			}
		}()
	}
	for i := range cursors {
		work <- i
	}
	close(work)
	wg.Wait()
	for i := range cursors {
		c := &cursors[i]
		if !c.done {
			r := core.BuildScoutReport(c.matches, c.masteries, season, cutoff, c.complete, c.twelve, c.month)
			r.State = "partial"
			r.Message = "已保留读取结果"
			r.Reason = scoutReasonIncomplete
			r.PagesRead = c.pages
			if !c.oldest.IsZero() {
				covered := c.oldest
				r.CoverageEndsAt = &covered
			}
			e.saveScout(version, scope, token, c.player.PUUID, r)
		}
	}
}

// waitScoutPace keeps the historic 150 ms spacing between remote pages.
func waitScoutPace(ctx context.Context) bool {
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// runScoutCursor fills one player's range. Workers share the round deadline and
// page budget but never touch another player's cursor.
func (e *Engine) runScoutCursor(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, c *scoutCursor, season *core.ScoutSeason, cutoff, boundary time.Time, pages *atomic.Int64) {
	for ctx.Err() == nil {
		if pages.Load() >= int64(scoutPageLimit) {
			return
		}
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
			return
		}
		// Deliver each player's first page before spending up to twenty seconds
		// on everybody else's mastery. No champion-lock dependency and no extra
		// requests: the same bounded round fills all heroes.
		if c.offset == 0 || len(c.masteries) == 0 {
			// Mastery is a short local read; it must not sit behind another
			// player's page scan or another worker's mastery.
			request, cancel := context.WithTimeout(ctx, 2*time.Second)
			c.masteries, _ = src.ChampionMasteries(request, c.player.PUUID)
			cancel()
			if ctx.Err() != nil {
				return
			}
			if c.offset == 0 {
				initial := core.BuildScoutReport(nil, c.masteries, season, cutoff, false, false, false)
				initial.State = "loading"
				if !e.saveScout(version, scope, token, c.player.PUUID, initial) {
					c.done = true
					return
				}
			}
		}
		// Reserve the page before the request so two workers cannot overshoot
		// the shared cap.
		if pages.Add(1) > int64(scoutPageLimit) {
			pages.Add(-1)
			return
		}
		request, cancel := context.WithTimeout(ctx, scoutPageTimeout(diagnostics.Default))
		page, err := src.ScoutPage(request, c.player.PUUID, c.offset)
		cancel()
		n := pages.Load()
		e.mu.Lock()
		if e.scoutValidLocked(version, scope, token) {
			e.scouting.recordPages(int(n))
		}
		e.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			// One bounded retry per page. A single slow page must not freeze
			// this player for the whole stage; the retry is a new request, so
			// the page budget above still counts it.
			if c.pageFails < 1 {
				c.pageFails++
				if !waitScoutPace(ctx) {
					return
				}
				continue
			}
			c.done = true
			c.reason = string(apperrors.CodeOf(err))
			c.retryAt = apperrors.RetryAt(err)
			c.message = apperrors.UserMessage(err, apperrors.DataUnavailable)
		} else {
			c.absorbPage(page, season, cutoff, boundary)
		}
		r := c.report(season, cutoff)
		if !e.saveScout(version, scope, token, c.player.PUUID, r) {
			c.done = true
		}
		e.rememberScoutProgress(version, scope, token, c)
		if c.done {
			return
		}
		if !waitScoutPace(ctx) {
			return
		}
	}
}

func (c *scoutCursor) absorbPage(page core.HistoryPage, season *core.ScoutSeason, cutoff, boundary time.Time) {
	c.pageFails = 0
	c.pages++
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
		// Service exhaustion can be a retention boundary. A full time
		// range requires an actual record reaching its start.
		c.twelve = !c.oldest.IsZero() && !c.oldest.After(cutoff.Add(-12*time.Hour))
		c.month = !c.oldest.IsZero() && !c.oldest.After(cutoff.Add(-30*24*time.Hour))
		c.complete = season != nil && !c.oldest.IsZero() && !c.oldest.After(season.Start)
	}
	if !page.HasMore || !c.oldest.IsZero() && !c.oldest.After(boundary) || page.NextOffset >= 20000 {
		c.done = true
	}
	c.offset = page.NextOffset
}

func (c *scoutCursor) report(season *core.ScoutSeason, cutoff time.Time) core.ScoutReport {
	if c.invalid {
		c.complete, c.twelve, c.month = false, false, false
		c.done = true
		if c.reason == "" {
			c.reason = scoutReasonRangeLimited
		}
	}
	r := core.BuildScoutReport(c.matches, c.masteries, season, cutoff, c.complete, c.twelve, c.month)
	if !c.done {
		r.State = "loading"
	} else if !r.Complete || !r.Complete30 {
		r.State = "partial"
	}
	r.Message = c.message
	r.Reason = c.reason
	r.RetryAt = c.retryAt
	r.PagesRead = c.pages
	if !c.oldest.IsZero() {
		covered := c.oldest
		r.CoverageEndsAt = &covered
	}
	return r
}
