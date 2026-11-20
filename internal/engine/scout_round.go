package engine

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func (e *Engine) readScoutSeason(ctx context.Context, src source.ScoutSource) *core.ScoutSeason {
	for ctx.Err() == nil {
		automatic, release, err := e.scheduler.acquireAutomatic(ctx)
		if err != nil {
			return nil
		}
		request, cancel := context.WithTimeout(automatic, 3*time.Second)
		season, _ := src.ScoutSeason(request)
		cancel()
		interrupted := automatic.Err() != nil && ctx.Err() == nil
		release()
		if !interrupted {
			return season
		}
	}
	return nil
}

// A round is a barrier: every eligible player gets a page attempt before any
// player receives a second one. Retries return to the next round as well.
func (e *Engine) scoutPageRound(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, cursors []scoutCursor, season *core.ScoutSeason, cutoff, boundary time.Time, pages *atomic.Int64) {
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < scoutWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				e.readScoutPage(ctx, version, scope, token, src, &cursors[i], season, cutoff, boundary, pages)
			}
		}()
	}
	order := make([]int, len(cursors))
	for i := range order {
		order[i] = i
	}
	queue := e.Snapshot().QueueID
	sort.SliceStable(order, func(i, j int) bool {
		return !scoutHasSample(cursors[order[i]], queue) && scoutHasSample(cursors[order[j]], queue)
	})
	for _, i := range order {
		if cursors[i].done {
			continue
		}
		select {
		case work <- i:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
}

func (e *Engine) scoutMasteries(ctx context.Context, version uint64, scope string, token uint64, src source.ScoutSource, cursors []scoutCursor, season *core.ScoutSeason, cutoff time.Time) {
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < scoutWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				c := &cursors[i]
				request, cancel := context.WithTimeout(ctx, 2*time.Second)
				c.masteries, _ = src.ChampionMasteries(request, c.player.PUUID)
				cancel()
				e.saveScout(version, scope, token, c.player.PUUID, c.report(season, cutoff))
			}
		}()
	}
	for i := range cursors {
		select {
		case work <- i:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
}

// Current-mode coverage takes precedence over optional mastery and deep history.
func scoutHasSample(c scoutCursor, queue int) bool {
	for _, m := range c.matches {
		if m.QueueID == queue && !core.IsRemake(m) && m.PlayedAt != nil {
			if m.Win != nil || m.Performance != nil || m.Kills != nil && m.Deaths != nil && m.Assists != nil {
				return true
			}
		}
	}
	return false
}
func scoutSamplesReady(cursors []scoutCursor, queue int) bool {
	for _, c := range cursors {
		if !c.done && !scoutHasSample(c, queue) {
			return false
		}
	}
	return true
}
