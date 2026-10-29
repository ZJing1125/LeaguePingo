package source

import (
	"net/http"
	"strconv"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

// Accessed only under sgpMu. Waiting is done by callers outside the scheduler;
// this gate never sleeps, creates workers or retries a request by itself.
type serviceCooldown struct {
	failures int
	err      error
	until    time.Time
}

func transientServiceError(err error) bool {
	switch apperrors.CodeOf(err) {
	case apperrors.NotLoggedIn, apperrors.RateLimited, apperrors.ServiceBusy, apperrors.NetworkUnavailable, apperrors.RequestTimeout:
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
