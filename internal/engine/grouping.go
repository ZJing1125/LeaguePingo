package engine

import (
	"context"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func (e *Engine) syncGroupingLocked() {
	key, active := e.grouping.apply(&e.snapshot)
	if !active {
		return
	}
	complete := len(e.snapshot.Participants) == 10
	for _, p := range e.snapshot.Participants {
		complete = complete && p.GroupToken != ""
	}
	src, ok := e.client.(source.GroupingSource)
	if complete || !ok {
		return
	}
	ctx, cancel, started := e.grouping.begin(e.initialMatch.parent, key)
	if started {
		go e.readGrouping(ctx, cancel, src, key, e.snapshot)
	}
}

func (e *Engine) readGrouping(ctx context.Context, cancel context.CancelFunc, src source.GroupingSource, key string, snapshot core.Snapshot) {
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		req, stop := context.WithTimeout(ctx, 2*time.Second)
		members, err := src.Grouping(req, snapshot.GameID)
		stop()
		proceed, complete := e.acceptGrouping(ctx, key, snapshot, members, err)
		if !proceed || complete {
			return
		}
		if attempt == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func (e *Engine) acceptGrouping(ctx context.Context, key string, snapshot core.Snapshot, members []core.Participant, err error) (bool, bool) {
	if ctx.Err() != nil {
		return false, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	valid := ctx.Err() == nil && e.grouping.current(key) && e.clientVersion == snapshot.ConnectionID && e.snapshot.SelfKey == snapshot.SelfKey && e.snapshot.GameID == snapshot.GameID && activeMatch(snapshotPhase(e.snapshot.Phase))
	if !valid {
		return false, false
	}
	if err != nil {
		return true, false
	}
	// Use the same identity/side checks in product and explicit audit.
	if _, checkErr := core.MergeGroupFields(e.snapshot.Participants, members); checkErr != nil {
		return false, false
	}
	accepted, complete := e.grouping.merge(key, members)
	if !accepted {
		return false, false
	}
	// Component lock is released before this re-enters grouping presentation.
	e.publishLocked()
	return true, complete
}
