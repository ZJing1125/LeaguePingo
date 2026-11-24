package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

// Each visible ally owns its cursor. A slow peer cannot delay another ally's
// second page. Publication and the existing shared request budget stay bounded.
func (e *Engine) prepareSelectionScouting(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, cursors []scoutCursor, result <-chan *core.ScoutSeason, cutoff time.Time, pages *atomic.Int64) {
	ready := make(chan struct{})
	var season *core.ScoutSeason
	go func() {
		select {
		case season = <-result:
		case <-ctx.Done():
		}
		close(ready)
	}()
	seasonNow := func() *core.ScoutSeason {
		select {
		case <-ready:
			return season
		default:
			return nil
		}
	}
	var workers sync.WaitGroup
	for i := range cursors {
		workers.Add(1)
		go func() {
			defer workers.Done()
			e.readSelectionCursor(ctx, version, scope, token, src, &cursors[i], seasonNow, cutoff, pages)
			// Optional season data never delays a player's first history publication.
			select {
			case <-ready:
			case <-ctx.Done():
				return
			}
			e.readSelectionCursor(ctx, version, scope, token, src, &cursors[i], seasonNow, cutoff, pages)
			c := &cursors[i]
			if ctx.Err() != nil {
				return
			}
			if len(c.masteries) == 0 {
				request, cancel := context.WithTimeout(ctx, 2*time.Second)
				c.masteries, _ = src.ChampionMasteries(request, c.player.PUUID)
				cancel()
			}
			e.saveScout(version, scope, token, c.player.PUUID, c.report(seasonNow(), cutoff))
		}()
	}
	workers.Wait()
}

func (e *Engine) readSelectionCursor(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, c *scoutCursor, seasonNow func() *core.ScoutSeason, cutoff time.Time, pages *atomic.Int64) {
	for ctx.Err() == nil && pages.Load() < scoutPageLimit {
		season := seasonNow()
		boundary := cutoff.Add(-30 * 24 * time.Hour)
		if mode, ok := core.ScoutModeFor(e.Snapshot().QueueID); ok && mode.Ranked && season != nil && season.Start.Before(boundary) {
			boundary = season.Start
			if c.done && c.month && c.reason == "" && c.pageFails == 0 && !c.invalid && c.hasMore && !c.oldest.IsZero() && c.oldest.After(boundary) {
				c.done = false
			}
		}
		if c.done {
			return
		}
		e.readScoutPage(ctx, version, scope, token, src, c, season, cutoff, boundary, pages)
	}
}
