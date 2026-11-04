package engine

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// reviewState owns the live candidate, one frozen review and settlement task.
// Engine -> review is the only nested lock order. Published reviews are immutable:
// updates replace the outer value, so view callers may read it after unlocking.
// No source I/O, engine callback or publication runs under this mutex.
type reviewState struct {
	mu        sync.RWMutex
	candidate *core.MatchReview
	review    *core.MatchReview
	owner     string
	cancel    context.CancelFunc
	token     uint64
}

func (r *reviewState) view() (*core.MatchReview, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.review, r.token
}

func (r *reviewState) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *reviewState) stopLocked() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.review != nil && r.review.State == "waiting" {
		r.finishLocked()
	}
	r.token++
}

func (r *reviewState) observe(snapshot core.Snapshot) *core.MatchReview {
	r.mu.Lock()
	defer r.mu.Unlock()
	if snapshot.SelfKey != "" {
		if r.owner != "" && r.owner != snapshot.SelfKey {
			r.stopLocked()
			r.review, r.candidate = nil, nil
		}
		r.owner = snapshot.SelfKey
	}
	phase := snapshotPhase(snapshot.Phase)
	if snapshot.Connected && phase == PhaseInProgress && snapshot.GameID > 0 && len(snapshot.Participants) > 0 {
		if r.candidate == nil || r.candidate.GameID != snapshot.GameID {
			r.stopLocked()
		}
		r.candidate = &core.MatchReview{ConnectionID: snapshot.ConnectionID, GameID: snapshot.GameID, Region: snapshot.Region, SelfKey: snapshot.SelfKey, QueueID: snapshot.QueueID, ScoutQueue: snapshot.ScoutQueue, CapturedAt: snapshot.UpdatedAt, Participants: clone(snapshot).Participants}
		return nil
	}
	if !reviewCanFreeze(snapshot, r.candidate) {
		return nil
	}
	frozen := freezeReview(r.candidate)
	if frozen == nil {
		return nil
	}
	r.stopLocked()
	r.review, r.candidate = frozen, nil
	return frozen
}

func reviewCanFreeze(snapshot core.Snapshot, candidate *core.MatchReview) bool {
	phase := snapshotPhase(snapshot.Phase)
	// Disconnection/reconnection is not evidence of a finished game.
	if !snapshot.Connected || activeMatch(phase) || phase == PhaseWatchInProgress || candidate == nil {
		return false
	}
	switch phase {
	case PhaseWaitingForStats, PhasePreEndOfGame, PhaseEndOfGame, PhaseLobby, PhaseMatchmaking, PhaseReadyCheck, PhaseChampSelect:
		return true
	case PhaseNone:
		return snapshot.SelfKey != "" && snapshot.SelfKey == candidate.SelfKey
	default:
		return false
	}
}

func freezeReview(candidate *core.MatchReview) *core.MatchReview {
	// Deep freeze once, including maps/pointers in reports/profiles. Failure
	// retains the candidate rather than publishing an incomplete clone.
	raw, err := json.Marshal(candidate)
	if err != nil {
		return nil
	}
	var frozen core.MatchReview
	if json.Unmarshal(raw, &frozen) != nil {
		return nil
	}
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
	return &frozen
}

func (r *reviewState) unavailable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.review != nil {
		copy := *r.review
		copy.State = "unavailable"
		r.review = &copy
	}
}

func (r *reviewState) begin() (context.Context, context.CancelFunc, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	r.cancel = cancel
	return ctx, cancel, r.token
}

func (r *reviewState) finish(gameID int64, token uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.review == nil || r.review.GameID != gameID || r.token != token {
		return false
	}
	r.finishLocked()
	return true
}

func (r *reviewState) finishLocked() {
	copy := *r.review
	copy.State = "unavailable"
	if copy.Result != nil {
		copy.State = "partial"
	}
	r.review = &copy
}

func (r *reviewState) matches(gameID int64, token uint64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.review != nil && r.review.GameID == gameID && r.token == token
}

func (r *reviewState) merge(gameID int64, token, version uint64, detail core.MatchDetail, manual bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.review == nil || r.review.GameID != gameID || r.token != token {
		return false
	}
	copy := *r.review
	detail.ConnectionID = version
	copy.Result, copy.State = &detail, "ready"
	if detail.Partial {
		copy.State = "partial"
		if !manual {
			copy.State = "waiting"
		}
	}
	// Practice is a legitimate one-player game; this does not add 5v5 shares.
	if copy.QueueID == 3140 && len(copy.Participants) == 1 && len(detail.Players) == 1 && detail.Players[0].Target {
		copy.State = "ready"
	}
	r.review = &copy
	return true
}
