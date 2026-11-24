package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/lcu"
)

func TestColdHistoryAuthorizationDoesNotBlockRankChannel(t *testing.T) {
	started := make(chan struct{})
	local := lcu.NewClient(lcu.Credentials{})
	local.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case "/entitlements/v1/token":
			close(started)
			<-r.Context().Done()
			return nil, r.Context().Err()
		case "/lol-league-session/v1/league-session-token":
			return jsonResponse(`"synthetic-ranked"`), nil
		}
		return nil, errors.New("unexpected local request")
	})
	s := &TencentLCU{client: local, sgp: &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer synthetic-ranked" {
			return nil, errors.New("incorrect token type")
		}
		return jsonResponse(`{"puuid":"target","queueMap":{}}`), nil
	})}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	historyDone := make(chan struct{})
	go func() { defer close(historyDone); _, _ = s.ScoutPage(ctx, "target", 0) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("history authorization did not start")
	}
	rankCtx, stopRank := context.WithTimeout(ctx, 500*time.Millisecond)
	defer stopRank()
	if _, err := s.sgpRanks(rankCtx, "target"); err != nil {
		t.Fatalf("rank blocked on unrelated authorization: %v", err)
	}
	cancel()
	<-historyDone
}

func TestSGPConcurrentReadsRefreshEachTokenOnlyOnce(t *testing.T) {
	var tokenReads atomic.Int32
	local := lcu.NewClient(lcu.Credentials{})
	local.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case "/entitlements/v1/token":
			tokenReads.Add(1)
			return jsonResponse(`{"accessToken":"synthetic-history"}`), nil
		}
		return nil, errors.New("unexpected local request")
	})
	s := &TencentLCU{client: local, sgp: &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"games":[]}`), nil
	})}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := s.ScoutPage(ctx, "target", 20); err != nil {
				t.Errorf("concurrent page failed: %v", err)
			}
		}()
	}
	workers.Wait()
	if tokenReads.Load() != 1 {
		t.Fatalf("concurrent reads refreshed authorization %d times", tokenReads.Load())
	}
}

func TestIdentityResetRejectsLateSGPAuthorizationAndPages(t *testing.T) {
	for _, duringAuth := range []bool{true, false} {
		t.Run(map[bool]string{true: "authorization", false: "page"}[duringAuth], func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			local := lcu.NewClient(lcu.Credentials{})
			local.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return jsonResponse(`{"accessToken":"synthetic-late"}`), nil
			})
			r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", leagueToken: "synthetic-rank", leagueExpires: time.Now().Add(time.Hour)}
			if !duringAuth {
				r.token, r.expires = "synthetic-history", time.Now().Add(time.Hour)
			}
			r.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return jsonResponse(`{"games":[]}`), nil
			})}
			s := &TencentLCU{client: local, sgp: r}
			done := make(chan error, 1)
			go func() { _, err := s.ScoutPage(ctx, "target", 20); done <- err }()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("synthetic request did not start")
			}
			s.ResetIdentity()
			close(release)
			if err := <-done; apperrors.CodeOf(err) != apperrors.StaleData {
				t.Fatalf("late result was accepted after reset: %v", err)
			}
			if r.token != "" || r.origin != "" || r.pages.len() != 0 {
				t.Fatal("late result repopulated a cleared session")
			}
		})
	}
}

func TestSGPRemoteConcurrencyIsBoundedPerChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan struct{}, 8)
	r := &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/rank" {
			return jsonResponse(`{}`), nil
		}
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}}
	var history, rank serviceGate
	var workers sync.WaitGroup
	for i := 0; i < sgpHistoryConcurrency+1; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, _, _ = r.getAuthorized(ctx, "https://synthetic.invalid", "/history", "synthetic", "/sgp-history", &history)
		}()
	}
	for i := 0; i < sgpHistoryConcurrency; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("history lane failed to reach its allowed concurrency")
		}
	}
	select {
	case <-started:
		t.Fatal("history lane exceeded its concurrency limit")
	case <-time.After(30 * time.Millisecond):
	}
	if _, _, err := r.getAuthorized(ctx, "https://synthetic.invalid", "/rank", "synthetic", "/sgp-rank", &rank); err != nil {
		t.Fatalf("full history lane blocked another channel: %v", err)
	}
	cancel()
	workers.Wait()
}

func TestChangedSGPHeadRejectsLateOffsetPage(t *testing.T) {
	var head atomic.Int64
	head.Store(1)
	started, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "synthetic", expires: time.Now().Add(time.Minute)}
	r.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		id := head.Load()
		if req.URL.Query().Get("startIndex") == "20" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return jsonResponse(fmt.Sprintf(`{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_%d"},"json":{"gameId":%d,"platformId":"NJ100","queueId":420,"participants":[{"puuid":"target"}]}}]}`, id, id)), nil
	})}
	s := &TencentLCU{sgp: r}
	if _, err := s.ScoutPage(ctx, "target", 0); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.ScoutPage(ctx, "target", 20); done <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("offset page did not start")
	}
	head.Store(2)
	if _, err := s.ScoutPage(ctx, "target", 0); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; apperrors.CodeOf(err) != apperrors.StaleData {
		t.Fatalf("outdated offset page was accepted: %v", err)
	}
	if r.pages.len() != 1 {
		t.Fatal("outdated offset page repopulated the cache")
	}
}
