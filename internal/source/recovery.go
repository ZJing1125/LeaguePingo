package source

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// serviceGate gives one SGP feature its own cooldown and serialises only its own
// requests. History, rank and match detail must not share a gate: a slow page
// used to hold the single global mutex and the shared cooldown, so an unrelated
// rank or detail read failed with it.
type serviceGate struct {
	mu       sync.Mutex
	cooldown serviceCooldown
}

func (g *serviceGate) blocked(now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cooldown.blocked(now)
}

func (g *serviceGate) failed(err error, retryAfter string, now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cooldown.failed(err, retryAfter, now)
}

func (g *serviceGate) clear() {
	g.mu.Lock()
	g.cooldown = serviceCooldown{}
	g.mu.Unlock()
}

// Access is guarded by the caller's gate.
type serviceCooldown struct {
	failures int
	err      error
	until    time.Time
}

// Only a service that answered with a rejection opens the shared gate. A slow
// or interrupted request is not evidence that the regional service is refusing
// this client: gating the whole batch on one timeout made every participant
// collapse to zero games. The round budget and per-page retry remain the brake.
func transientServiceError(err error) bool {
	switch apperrors.CodeOf(err) {
	case apperrors.NotLoggedIn, apperrors.RateLimited, apperrors.ServiceBusy:
		return true
	}
	return false
}
func (g *serviceCooldown) blocked(now time.Time) error {
	if g.err != nil && now.Before(g.until) {
		return apperrors.WithRetryAt(g.err, g.until)
	}
	return nil
}
func (g *serviceCooldown) failed(err error, retryAfter string, now time.Time) error {
	if !transientServiceError(err) {
		return err
	}
	g.failures = min(g.failures+1, 4)
	delay := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second, time.Minute}[g.failures-1]
	g.until = now.Add(delay)
	if at, ok := retryAfterTime(retryAfter, now); ok && at.After(g.until) {
		g.until = at
	}
	g.err = apperrors.Normalize(err, apperrors.DataUnavailable)
	return apperrors.WithRetryAt(g.err, g.until)
}
func retryAfterTime(value string, now time.Time) (time.Time, bool) {
	if n, err := strconv.ParseInt(value, 10, 64); err == nil && n >= 0 {
		// Saturate the duration instead of overflowing a hostile/excessive header.
		if n > int64((1<<63-1)/time.Second) {
			return now.Add(time.Duration(1<<63 - 1)), true
		}
		return now.Add(time.Duration(n) * time.Second), true
	}
	at, err := http.ParseTime(value)
	return at, err == nil && at.After(now)
}
func historyServiceState(err error) *core.HistoryServiceState {
	if err == nil {
		return nil
	}
	return &core.HistoryServiceState{Code: string(apperrors.CodeOf(apperrors.Normalize(err, apperrors.DataUnavailable))), RetryAt: apperrors.RetryAt(err)}
}
