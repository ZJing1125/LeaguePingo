package source

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

const sgpChannelConcurrency = 4

// serviceGate gives one SGP feature its own cooldown and bounded request lane.
// History, rank and match detail must not share a gate: a slow page used to hold
// the single global mutex and the shared cooldown, so an unrelated rank or detail
// read failed with it.
type serviceGate struct {
	mu         sync.Mutex
	cooldown   serviceCooldown
	generation uint64
	rejections uint64
	limit      int
	once       sync.Once
	slots      chan struct{}
}

func (g *serviceGate) acquire(ctx context.Context) error {
	if ctx.Err() != nil {
		return apperrors.Request(ctx, ctx.Err(), apperrors.DataService)
	}
	if err := g.blocked(time.Now()); err != nil {
		return err
	}
	g.once.Do(func() {
		limit := g.limit
		if limit <= 0 {
			limit = sgpChannelConcurrency
		}
		g.slots = make(chan struct{}, limit)
	})
	select {
	case g.slots <- struct{}{}:
	case <-ctx.Done():
		return apperrors.Request(ctx, ctx.Err(), apperrors.DataService)
	}
	if ctx.Err() != nil {
		g.release()
		return apperrors.Request(ctx, ctx.Err(), apperrors.DataService)
	}
	// A rejection can open the channel cooldown while this request waits for a
	// slot. Recheck before doing any network I/O.
	if err := g.blocked(time.Now()); err != nil {
		g.release()
		return err
	}
	return nil
}

func (g *serviceGate) release() { <-g.slots }

func (g *serviceGate) blocked(now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cooldown.blocked(now)
}

func (g *serviceGate) failed(err error, retryAfter string, now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if transientServiceError(err) {
		g.rejections++
	}
	return g.cooldown.failed(err, retryAfter, now)
}

func (g *serviceGate) reset() {
	g.mu.Lock()
	g.generation++
	g.rejections = 0
	g.cooldown = serviceCooldown{}
	g.mu.Unlock()
}

type serviceVersion struct{ generation, rejections uint64 }

func (g *serviceGate) version() serviceVersion {
	g.mu.Lock()
	defer g.mu.Unlock()
	return serviceVersion{g.generation, g.rejections}
}

func (g *serviceGate) failedFor(version serviceVersion, err error, retryAfter string, now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if version.generation != g.generation {
		return apperrors.New(apperrors.StaleData)
	}
	if transientServiceError(err) {
		g.rejections++
	}
	return g.cooldown.failed(err, retryAfter, now)
}

func (g *serviceGate) clearFor(version serviceVersion) {
	g.mu.Lock()
	// Success that started before a rejection must not reopen the channel for
	// queued callers. Only a subsequent admitted request may clear that failure.
	if version.generation == g.generation && version.rejections == g.rejections {
		g.cooldown = serviceCooldown{}
	}
	g.mu.Unlock()
}

// Access is guarded by the caller's gate.
type serviceCooldown struct {
	failures int
	err      error
	until    time.Time
	jitter   func(time.Duration) time.Duration
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
	spread := min(delay/5, time.Minute-delay)
	if spread > 0 {
		jitter := time.Duration(rand.Int64N(int64(spread) + 1))
		if g.jitter != nil {
			jitter = g.jitter(spread)
		}
		delay += max(0, min(spread, jitter))
	}
	previous := g.until
	g.until = now.Add(delay)
	if at, ok := retryAfterTime(retryAfter, now); ok && at.After(g.until) {
		g.until = at
	}
	if previous.After(g.until) {
		g.until = previous
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
