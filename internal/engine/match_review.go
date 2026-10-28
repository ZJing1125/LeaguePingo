package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func (e *Engine) stopReviewLocked() {
	if e.reviewCancel != nil {
		e.reviewCancel()
		e.reviewCancel = nil
	}
	if e.review != nil && e.review.State == "waiting" {
		copy := *e.review
		copy.State = "unavailable"
		if copy.Result != nil {
			copy.State = "partial"
		}
		e.review = &copy
	}
	e.reviewToken++
}

func (e *Engine) syncReviewLocked() {
	s := e.snapshot
	if s.SelfKey != "" {
		if e.reviewOwner != "" && e.reviewOwner != s.SelfKey {
			e.stopReviewLocked()
			e.review, e.reviewCandidate = nil, nil
		}
		e.reviewOwner = s.SelfKey
	}
	if s.Connected && s.Phase == "InProgress" && s.GameID > 0 && len(s.Participants) > 0 {
		if e.reviewCandidate == nil || e.reviewCandidate.GameID != s.GameID {
			e.stopReviewLocked()
		}
		e.reviewCandidate = &core.MatchReview{ConnectionID: s.ConnectionID, GameID: s.GameID, Region: s.Region, SelfKey: s.SelfKey, QueueID: s.QueueID, ScoutQueue: s.ScoutQueue, CapturedAt: s.UpdatedAt, Participants: clone(s).Participants}
		return
	}
	// Disconnection/reconnection is not evidence of a finished game.
	if !s.Connected || activeMatch(s.Phase) || s.Phase == "WatchInProgress" || e.reviewCandidate == nil {
		return
	}
	switch s.Phase {
	case "WaitingForStats", "PreEndOfGame", "EndOfGame", "Lobby", "Matchmaking", "ReadyCheck", "ChampSelect":
	case "None":
		if s.SelfKey == "" || s.SelfKey != e.reviewCandidate.SelfKey {
			return
		}
	default:
		return
	}
	r := e.reviewCandidate
	// One deep freeze at completion, including maps/pointers in reports/profiles.
	raw, _ := json.Marshal(r)
	var frozen core.MatchReview
	if json.Unmarshal(raw, &frozen) != nil {
		return
	}
	// A finished review has no running preparation. Preserve real values and
	// their timestamps, but do not archive an endless loading/pending indicator.
	for i := range frozen.Participants {
		p := &frozen.Participants[i]
		p.Cached = true
		if p.IdentityState == core.IdentityKnown && (p.HistoryState == core.HistoryLoading || p.HistoryState == core.HistoryPending) {
			if len(p.History) > 0 {
				p.HistoryState = core.HistoryReady
			} else {
				p.HistoryState = core.HistoryFailed
				if p.Error == "" {
					p.Error = "本局结束时战绩未完整保留"
				}
			}
		}
		if p.Scout != nil && p.Scout.State == "loading" {
			p.Scout.State = "partial"
		}
	}
	frozen.State, frozen.CapturedAt = "waiting", time.Now()
	e.stopReviewLocked()
	e.review, e.reviewCandidate = &frozen, nil
	if e.client == nil {
		e.review.State = "unavailable"
		return
	}
	if _, ok := e.client.(source.MatchDetailSource); !ok {
		e.review.State = "unavailable"
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
		e.review.State = "unavailable"
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	e.reviewCancel = cancel
	token, version, client := e.reviewToken, e.clientVersion, e.client
	go func() {
		defer cancel()
		started := time.Now()
	attempts:
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				// Spend the existing 15-second budget across readiness windows,
				// rather than exhausting three fast 404s in the first three seconds.
				select {
				case <-ctx.Done():
					break attempts
				case <-time.After(max(0, time.Until(started.Add(time.Duration(attempt)*5*time.Second)))):
				}
			}
			if err := e.fetchReview(ctx, frozen.GameID, version, token, client, false); err == nil {
				e.mu.RLock()
				ready := e.review != nil && e.review.GameID == frozen.GameID && e.review.State == "ready"
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
		if e.reviewToken == token && e.review != nil && e.review.GameID == frozen.GameID {
			copy := *e.review
			if copy.Result != nil {
				copy.State = "partial"
			} else {
				copy.State = "unavailable"
			}
			e.review = &copy
			e.snapshot.UpdatedAt = time.Now()
			e.publishLocked()
		}
	}()
}

func (e *Engine) RefreshReview(ctx context.Context, gameID int64) (core.Snapshot, error) {
	ctx, done, err := e.beginLookup(ctx)
	if err != nil {
		return core.Snapshot{}, err
	}
	defer done()
	e.mu.Lock()
	if e.review == nil || e.review.GameID != gameID || e.client == nil || activeMatch(e.snapshot.Phase) ||
		e.accountIdentity == "" || e.review.SelfKey != e.accountIdentity || e.review.Region != e.client.Region() {
		e.mu.Unlock()
		return core.Snapshot{}, errors.New("当前无法同步这一局的结算，可继续查看已保留资料")
	}
	e.stopReviewLocked()
	token, version, client := e.reviewToken, e.clientVersion, e.client
	e.mu.Unlock()
	err = e.fetchReview(ctx, gameID, version, token, client, true)
	return e.Snapshot(), err
}

func (e *Engine) fetchReview(ctx context.Context, gameID int64, version, token uint64, client Client, manual bool) error {
	e.mu.RLock()
	r := e.review
	valid := r != nil && r.GameID == gameID && e.reviewToken == token && e.clientVersion == version && e.client != nil && e.snapshot.Connected && !activeMatch(e.snapshot.Phase)
	target := ""
	if valid {
		for _, p := range r.Participants {
			if p.Key == r.SelfKey || p.PUUID == r.SelfKey {
				target = p.PUUID
				break
			}
		}
	}
	e.mu.RUnlock()
	details, ok := client.(source.MatchDetailSource)
	if !valid || !ok || !core.IsKnownPUUID(target) {
		return errors.New("本局结算暂未返回")
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
	// Recheck after waiting for a manual operation/new match/account transition.
	e.mu.RLock()
	valid = e.reviewToken == token && e.clientVersion == version && e.client != nil && e.snapshot.Connected && !activeMatch(e.snapshot.Phase)
	e.mu.RUnlock()
	if !valid {
		return errors.New("对局已切换")
	}
	detail, err := details.MatchDetail(requestCtx, gameID, target)
	if requestCtx.Err() != nil {
		return requestCtx.Err()
	}
	if err != nil {
		return errors.New("本局结算暂未返回，可稍后同步")
	}
	if detail.GameID != gameID || len(detail.Players) == 0 {
		return errors.New("结算数据与本局不匹配")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.reviewToken != token || e.clientVersion != version || e.client == nil || !e.snapshot.Connected || e.review == nil || e.review.GameID != gameID || activeMatch(e.snapshot.Phase) {
		return errors.New("对局已切换")
	}
	copy := *e.review
	detail.ConnectionID = version
	copy.Result = &detail
	copy.State = "ready"
	if detail.Partial {
		copy.State = "partial"
		if !manual {
			copy.State = "waiting"
		}
	}
	// Practice is a legitimate one-player game. Team shares still stay absent
	// because ParseMatchDetail requires complete 5v5 for those comparisons.
	if copy.QueueID == 3140 && len(copy.Participants) == 1 && len(detail.Players) == 1 && detail.Players[0].Target {
		copy.State = "ready"
	}
	e.review = &copy
	e.snapshot.UpdatedAt = time.Now()
	e.publishLocked()
	return nil
}
