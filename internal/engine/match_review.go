package engine

import (
	"context"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func (e *Engine) stopReviewLocked() { e.reviews.stop() }

func (e *Engine) syncReviewLocked() {
	frozen := e.reviews.observe(e.snapshot)
	if frozen == nil {
		return
	}
	if e.client == nil {
		e.reviews.unavailable()
		return
	}
	if _, ok := e.client.(source.MatchDetailSource); !ok {
		e.reviews.unavailable()
		return
	}
	verifiedTarget := false
	for _, p := range frozen.Participants {
		if (p.Key == frozen.SelfKey || p.PUUID == frozen.SelfKey) && core.IsKnownPUUID(p.PUUID) {
			verifiedTarget = true
			break
		}
	}
	if !verifiedTarget {
		e.reviews.unavailable()
		return
	}
	ctx, cancel, token := e.reviews.begin()
	go e.completeReview(ctx, cancel, frozen.GameID, e.clientVersion, token, e.client)
}

func (e *Engine) completeReview(ctx context.Context, cancel context.CancelFunc, gameID int64, version, token uint64, client Client) {
	defer cancel()
	started := time.Now()
attempts:
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Keep the existing readiness windows within the 15-second budget.
			select {
			case <-ctx.Done():
				break attempts
			case <-time.After(max(0, time.Until(started.Add(time.Duration(attempt)*5*time.Second)))):
			}
		}
		if err := e.fetchReview(ctx, gameID, version, token, client, false); err == nil {
			e.mu.RLock()
			review, _ := e.reviews.view()
			ready := review != nil && review.GameID == gameID && review.State == "ready"
			e.mu.RUnlock()
			if ready {
				return
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.reviews.finish(gameID, token) {
		e.snapshot.UpdatedAt = time.Now()
		e.publishLocked()
	}
}

func (e *Engine) RefreshReview(ctx context.Context, gameID int64) (core.Snapshot, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Snapshot{}, err
	}
	defer done()
	e.mu.Lock()
	review, _ := e.reviews.view()
	if review == nil || review.GameID != gameID || e.client == nil || activeMatch(snapshotPhase(e.snapshot.Phase)) ||
		e.accountIdentity == "" || review.SelfKey != e.accountIdentity || review.Region != e.client.Region() {
		e.mu.Unlock()
		return core.Snapshot{}, apperrors.New(apperrors.MatchNotReady)
	}
	e.stopReviewLocked()
	_, token := e.reviews.view()
	version, client := e.clientVersion, e.client
	e.mu.Unlock()
	err = e.fetchReview(ctx, gameID, version, token, client, true)
	return e.Snapshot(), err
}

// Engine lock must be held before inspecting component lifetime and live phase.
func (e *Engine) reviewReadValidLocked(gameID int64, version, token uint64) bool {
	return e.reviews.matches(gameID, token) && e.clientVersion == version && e.client != nil && e.snapshot.Connected && !activeMatch(snapshotPhase(e.snapshot.Phase))
}

func (e *Engine) fetchReview(ctx context.Context, gameID int64, version, token uint64, client Client, manual bool) error {
	e.mu.RLock()
	review, _ := e.reviews.view()
	valid := e.reviewReadValidLocked(gameID, version, token)
	target := ""
	if valid {
		for _, p := range review.Participants {
			if p.Key == review.SelfKey || p.PUUID == review.SelfKey {
				target = p.PUUID
				break
			}
		}
	}
	e.mu.RUnlock()
	details, ok := client.(source.MatchDetailSource)
	if !valid || !ok || !core.IsKnownPUUID(target) {
		return apperrors.New(apperrors.MatchNotReady)
	}
	req, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	requestCtx := req
	var release func()
	var err error
	if manual {
		release, err = e.scheduler.acquire(req, true)
	} else {
		requestCtx, release, err = e.scheduler.acquireAutomatic(req)
	}
	if err != nil {
		return err
	}
	defer release()
	// Check again after the scheduler and after source I/O. No component lock
	// is held while waiting for another operation or the source.
	e.mu.RLock()
	valid = e.reviewReadValidLocked(gameID, version, token)
	e.mu.RUnlock()
	if !valid {
		return apperrors.New(apperrors.StaleData)
	}
	detail, err := details.MatchDetail(requestCtx, gameID, target)
	if requestCtx.Err() != nil {
		return requestCtx.Err()
	}
	if err != nil {
		return apperrors.New(apperrors.MatchNotReady)
	}
	if detail.GameID != gameID || len(detail.Players) == 0 {
		return apperrors.New(apperrors.InvalidResponse)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.reviewReadValidLocked(gameID, version, token) || !e.reviews.merge(gameID, token, version, detail, manual) {
		return apperrors.New(apperrors.StaleData)
	}
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return nil
}
