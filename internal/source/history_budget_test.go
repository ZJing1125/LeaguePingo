package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

// The regional summary attempt must leave enough of the caller budget for the
// verified local fallback. A blocking summary is the reproducible shape of the
// reported failure: the remote service is slow, the client itself is healthy.
func TestHistoryKeepsLocalFallbackWhenSummaryIsSlow(t *testing.T) {
	local := lcu.NewClient(lcu.Credentials{})
	local.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case r.URL.Path == "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case r.URL.Path == "/entitlements/v1/token":
			return jsonResponse(`{"accessToken":"synthetic-secret"}`), nil
		case strings.HasPrefix(r.URL.Path, "/lol-match-history/v1/products/lol/"):
			return jsonResponse(`{"games":{"gameIndexBegin":0,"gameIndexEnd":0,"gameCount":1,"games":[{"gameId":7,"gameCreation":1700000000000,"queueId":420,"mapId":11,"gameDuration":1800,"participants":[{"puuid":"target","participantId":1,"teamId":100,"championId":1,"teamPosition":"MIDDLE","stats":{"win":true,"kills":1,"deaths":2,"assists":3}}],"participantIdentities":[{"participantId":1,"player":{"puuid":"target"}}]}]}}`), nil
		}
		t.Fatalf("unexpected local request %s", r.URL.Path)
		return nil, nil
	})
	remote := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	s := &TencentLCU{client: local, sgp: &sgpHistory{http: remote}}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	started := time.Now()
	result, err := s.History(ctx, "target")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("slow summary discarded the local fallback: %v", err)
	}
	if len(result.Matches) != 1 || result.Matches[0].GameID != 7 {
		t.Fatalf("local fallback page not applied: %+v", result.Matches)
	}
	if result.Meta.Service == nil {
		t.Fatal("fallback did not expose the unavailable service state")
	}
	// The remote attempt is capped below the whole caller budget, so the local
	// page still runs and the call returns before the six-second preparation
	// timeout that used to fail the entire history.
	if elapsed >= 6*time.Second {
		t.Fatalf("summary attempt consumed the caller budget: %s", elapsed)
	}
}

func TestRequestTimeoutDoesNotOpenTheSharedServiceGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r := &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}}
	gate := &serviceGate{}
	if _, _, err := r.getAuthorized(ctx, "https://synthetic.invalid", "/synthetic", "synthetic-token", "/sgp-history", gate); err == nil {
		t.Fatal("expected a timeout error")
	}
	if blocked := gate.blocked(time.Now()); blocked != nil {
		t.Fatal("a per-request timeout froze the whole batch")
	}
}

func TestServiceRejectionStillOpensTheSharedServiceGate(t *testing.T) {
	r := &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{"Retry-After": []string{"9"}}}, nil
	})}}
	gate := &serviceGate{}
	_, status, err := r.getAuthorized(context.Background(), "https://synthetic.invalid", "/synthetic", "synthetic-token", "/sgp-history", gate)
	if status != http.StatusTooManyRequests || err == nil {
		t.Fatalf("rate limit not surfaced: %d %v", status, err)
	}
	if blocked := gate.blocked(time.Now()); blocked == nil {
		t.Fatal("a rate-limited service did not open the shared gate")
	}
}

// The channel split is the point of the per-feature gates: while one SUMMARY
// page hangs, a rank read must still complete on its own gate and transport.
func TestRankReadIsNotBlockedBySlowHistory(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case "/entitlements/v1/token":
			return jsonResponse(`{"accessToken":"synthetic-secret"}`), nil
		case "/lol-league-session/v1/league-session-token":
			return jsonResponse(`"synthetic-league"`), nil
		}
		t.Fatalf("unexpected local request %s", r.URL.Path)
		return nil, nil
	})
	remote := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/SUMMARY") {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return jsonResponse(`{"puuid":"target","queueMap":{}}`), nil
	})}
	s := &TencentLCU{client: c, sgp: &sgpHistory{http: remote}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	historyDone := make(chan struct{})
	go func() { defer close(historyDone); _, _ = s.ScoutPage(ctx, "target", 0) }()
	time.Sleep(150 * time.Millisecond)
	rankDone := make(chan struct{})
	go func() { defer close(rankDone); _, _ = s.sgpRanks(ctx, "target") }()
	select {
	case <-rankDone:
	case <-time.After(2 * time.Second):
		t.Fatal("rank read blocked behind a slow history page")
	}
	cancel()
	<-historyDone
}
