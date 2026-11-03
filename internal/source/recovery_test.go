package source

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/lcu"
)

func TestServiceCooldownBackoffAndRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	for _, status := range []int{401, 429, 500, 503} {
		gate := serviceCooldown{jitter: func(time.Duration) time.Duration { return 0 }}
		for i, delay := range []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second, time.Minute} {
			err := gate.failed(apperrors.HTTP(apperrors.DataService, status), "", now)
			if apperrors.RetryAt(err) == nil || !apperrors.RetryAt(err).Equal(now.Add(delay)) || gate.failures != i+1 {
				t.Fatalf("wrong backoff: %v", err)
			}
			if gate.blocked(now.Add(delay-time.Millisecond)) == nil || gate.blocked(now.Add(delay)) != nil {
				t.Fatal("gate ignored due time")
			}
			now = now.Add(delay)
		}
	}
	for _, header := range []string{"120", now.Add(2 * time.Minute).Format(http.TimeFormat)} {
		var gate serviceCooldown
		gate.failed(apperrors.HTTP(apperrors.DataService, 429), header, now)
		if gate.until.Before(now.Add(2 * time.Minute)) {
			t.Fatal("retried before server Retry-After")
		}
	}
	for _, err := range []error{context.Canceled, apperrors.HTTP(apperrors.DataService, 403), apperrors.New(apperrors.InvalidResponse)} {
		var gate serviceCooldown
		gate.failed(err, "120", now)
		if gate.blocked(now) != nil || gate.failures != 0 {
			t.Fatal("retried cancellation/permanent failure")
		}
	}
}

func TestServiceBackoffJitterNeverShortensWaitOrExceedsCap(t *testing.T) {
	now := time.Now()
	for _, maximum := range []bool{false, true} {
		gate := serviceCooldown{jitter: func(spread time.Duration) time.Duration {
			if maximum {
				return spread
			}
			return 0
		}}
		for _, base := range []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second, time.Minute} {
			err := gate.failed(apperrors.HTTP(apperrors.DataService, 429), "", now)
			wait := apperrors.RetryAt(err).Sub(now)
			if wait < base || wait > min(base+base/5, time.Minute) {
				t.Fatalf("jitter escaped its bounds: %v", wait)
			}
			now = gate.until
		}
	}
	gate := serviceCooldown{}
	err := gate.failed(apperrors.HTTP(apperrors.DataService, 429), "120", now)
	if apperrors.RetryAt(err).Before(now.Add(2 * time.Minute)) {
		t.Fatal("jitter bypassed Retry-After")
	}
}

func TestConcurrentRejectionsCannotShortenServerWait(t *testing.T) {
	now := time.Now()
	gate := serviceCooldown{jitter: func(time.Duration) time.Duration { return 0 }}
	first := gate.failed(apperrors.HTTP(apperrors.DataService, 429), "120", now)
	next := gate.failed(apperrors.HTTP(apperrors.DataService, 503), "", now.Add(time.Second))
	if apperrors.RetryAt(next).Before(*apperrors.RetryAt(first)) {
		t.Fatal("overlapping rejection bypassed the longer server wait")
	}
}

func TestRateLimitSurvivesConcurrentSuccessAndStopsRequestBurst(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var calls atomic.Int32
	r := &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.URL.Path == "/delayed-success" {
			close(started)
			select {
			case <-release:
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			return jsonResponse(`{}`), nil
		}
		return &http.Response{StatusCode: 429, Body: jsonResponse("").Body, Header: http.Header{"Retry-After": []string{"60"}}}, nil
	})}}
	gate := &serviceGate{limit: 2}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = r.getAuthorized(ctx, "https://synthetic.invalid", "/delayed-success", "synthetic", "/sgp-history", gate)
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("delayed success did not start")
	}
	_, status, err := r.getAuthorized(ctx, "https://synthetic.invalid", "/rejection", "synthetic", "/sgp-history", gate)
	if status != 429 || apperrors.CodeOf(err) != apperrors.RateLimited {
		t.Fatal("limit response was lost")
	}
	close(release)
	<-done
	var burst sync.WaitGroup
	for range 64 {
		burst.Add(1)
		go func() {
			defer burst.Done()
			_, _, err := r.getAuthorized(ctx, "https://synthetic.invalid", "/queued", "synthetic", "/sgp-history", gate)
			if apperrors.CodeOf(err) != apperrors.RateLimited {
				t.Errorf("cooldown bypassed: %v", err)
			}
		}()
	}
	burst.Wait()
	if calls.Load() != 2 {
		t.Fatalf("rate limit caused a request burst: %d", calls.Load())
	}
	if gate.blocked(time.Now()) == nil {
		t.Fatal("pre-rejection success reopened the gate")
	}
	// A successful request admitted after the wait can clear the old rejection.
	gate.mu.Lock()
	gate.cooldown.until = time.Now().Add(-time.Second)
	gate.mu.Unlock()
	version := gate.version()
	gate.clearFor(version)
	if gate.blocked(time.Now()) != nil {
		t.Fatal("new success failed to clear the expired rejection")
	}
}

func TestServiceGateLimitsOnlyItsOwnChannel(t *testing.T) {
	gate := serviceGate{limit: 1}
	if err := gate.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	blocked := make(chan error, 1)
	go func() { blocked <- gate.acquire(ctx) }()

	other := serviceGate{limit: 1}
	if err := other.acquire(context.Background()); err != nil {
		t.Fatalf("independent channel was blocked: %v", err)
	}
	other.release()
	if err := <-blocked; !errors.Is(err, context.DeadlineExceeded) || apperrors.CodeOf(err) != apperrors.RequestTimeout {
		t.Fatalf("same channel bypassed its limit: %v", err)
	}
	gate.release()
}

func TestServiceGateResetDiscardsLateFailuresAndSuccesses(t *testing.T) {
	var gate serviceGate
	previous := gate.version()
	gate.reset()
	if err := gate.failedFor(previous, apperrors.HTTP(apperrors.DataService, 429), "60", time.Now()); apperrors.CodeOf(err) != apperrors.StaleData || gate.blocked(time.Now()) != nil {
		t.Fatal("old rejection poisoned the new session")
	}
	gate.failed(apperrors.HTTP(apperrors.DataService, 429), "60", time.Now())
	gate.clearFor(previous)
	if gate.blocked(time.Now()) == nil {
		t.Fatal("old success bypassed the new session cooldown")
	}
}

func TestOutageRetainsLocalHistoryAndGateCoversOtherPlayers(t *testing.T) {
	remoteCalls, localHistory, tokenReads := 0, 0, 0
	c := lcu.NewClient(lcu.Credentials{Port: 12345})
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/entitlements/v1/token" {
			tokenReads++
			return jsonResponse(`{"accessToken":"refreshed"}`), nil
		}
		if !strings.HasPrefix(r.URL.Path, "/lol-match-history/") {
			t.Fatalf("unexpected local request: %s", r.URL.Path)
		}
		localHistory++
		return jsonResponse(`{"games":{"games":[{"gameId":7,"participants":[{"participantId":1,"championId":1,"stats":{"win":true}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target"}}]}]}}`), nil
	})
	r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "old", expires: time.Now().Add(time.Hour), leagueToken: "league", leagueExpires: time.Now().Add(time.Hour)}
	r.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		remoteCalls++
		if remoteCalls == 1 {
			return &http.Response{StatusCode: 401, Body: jsonResponse("").Body, Header: http.Header{}}, nil
		}
		if req.Header.Get("Authorization") != "Bearer refreshed" {
			t.Fatal("expired token reused")
		}
		return jsonResponse(`{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_8"},"json":{"gameId":8,"platformId":"NJ100","queueId":420,"participants":[{"puuid":"target","kills":4}]}}]}`), nil
	})}
	s := &TencentLCU{client: c, sgp: r}
	first, err := s.History(context.Background(), "target")
	if err != nil || len(first.Matches) != 1 || first.Meta.Service == nil || first.Meta.Service.Code != "NotLoggedIn" || first.Meta.Service.RetryAt == nil || r.token != "" {
		t.Fatal("local fallback or auth invalidation lost")
	}
	_, _ = s.History(context.Background(), "target")
	_, _, _ = r.getAuthorized(context.Background(), r.origin, "/different-player", "different-token", "/sgp-history", &s.channels.history)
	if remoteCalls != 1 || tokenReads != 0 || localHistory != 2 {
		t.Fatal("cooldown bypassed through another request in the same channel")
	}
	s.channels.history.mu.Lock()
	s.channels.history.cooldown.until = time.Now().Add(-time.Second) // Advance only the synthetic source gate.
	s.channels.history.mu.Unlock()
	restored, err := s.History(context.Background(), "target")
	if err != nil || len(restored.Matches) != 1 || restored.Matches[0].GameID != 8 || restored.Meta.Service != nil || tokenReads != 1 || remoteCalls != 2 || s.channels.history.cooldown.err != nil {
		t.Fatal("session refresh/remote recovery did not clear degradation")
	}
	s.ResetIdentity()
	if s.channels.history.cooldown.err != nil || r.token != "" {
		t.Fatal("new session inherited old failure")
	}
}

func TestLocalFallbackCancellationIsNotReplacedByServiceRetry(t *testing.T) {
	for _, paged := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		c := lcu.NewClient(lcu.Credentials{Port: 12345})
		c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { cancel(); return nil, context.Canceled })
		r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "token", expires: time.Now().Add(time.Hour), http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 503, Body: jsonResponse("").Body}, nil
		})}}
		s := &TencentLCU{client: c, sgp: r}
		var err error
		if paged {
			_, err = s.HistoryPage(ctx, "target", 20)
		} else {
			_, err = s.History(ctx, "target")
		}
		cancel()
		if !errors.Is(err, context.Canceled) || apperrors.CodeOf(err) != apperrors.RequestCanceled {
			t.Fatal("fallback cancellation became a retriable service failure")
		}
	}
}
