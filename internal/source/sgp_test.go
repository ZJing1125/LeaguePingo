package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestSGPExplicitPagingUsesTrustedHostSessionAndRawOffset(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	localCalls, remoteCalls := 0, 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		localCalls++
		body := ""
		switch r.URL.Path {
		case "/lol-rso-auth/v1/authorization":
			body = `{"currentPlatformId":"NJ100"}`
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			body = `"https://nj100-sgp.lol.qq.com:21019"`
		case "/entitlements/v1/token":
			body = `{"accessToken":"synthetic-secret"}`
		default:
			t.Fatal("unexpected credential/profile request")
		}
		return jsonResponse(body), nil
	})
	remote := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		remoteCalls++
		if r.URL.Scheme != "https" || r.URL.Host != "nj100-sgp.lol.qq.com:21019" || r.Header.Get("Authorization") != "Bearer synthetic-secret" || r.Method != "GET" || r.URL.Query().Get("count") != "20" {
			t.Fatal("unsafe or incorrect remote request")
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
		return jsonResponse(fmt.Sprintf(`{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_%d"},"json":{"gameId":%d,"platformId":"NJ100","queueId":440,"participants":[{"puuid":"target","kills":4,"deaths":2,"assists":8}]}}]}`, offset+1, offset+1)), nil
	})}
	s := &TencentLCU{client: c, sgp: &sgpHistory{http: remote}}
	for _, offset := range []int{20, 40} {
		p, err := s.HistoryPage(context.Background(), "target", offset)
		if err != nil || p.Matches[0].GameID != int64(offset+1) || p.NextOffset != offset+1 {
			t.Fatalf("remote page mismatch: %v", err)
		}
	}
	if localCalls != 3 || remoteCalls != 2 {
		t.Fatal("credentials fetched repeatedly")
	}
	s.ResetIdentity()
	if s.sgp.token != "" || s.sgp.origin != "" {
		t.Fatal("reset retained session credentials")
	}
}

func TestSGPRejectsUntrustedOriginsAndFallbackDoesNotPretendExhaustion(t *testing.T) {
	for _, origin := range []string{"http://nj100-sgp.lol.qq.com:21019", "https://evil.example", "https://nj100-sgp.lol.qq.com.evil.example:21019", "https://hn1-sgp.lol.qq.com:21019", "https://nj100-sgp.lol.qq.com:21019/path", "https://user@nj100-sgp.lol.qq.com:21019"} {
		if _, err := trustedSGPOrigin("NJ100", origin); err == nil {
			t.Fatal("untrusted origin accepted")
		}
	}
	c := lcu.NewClient(lcu.Credentials{})
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path == "/lol-rso-auth/v1/authorization" {
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		}
		if strings.Contains(r.URL.Path, "ServiceEndpoint") {
			return jsonResponse(`"https://evil.example"`), nil
		}
		if !strings.Contains(r.URL.Path, "match-history") || r.URL.Query().Get("begIndex") != "0" || r.URL.Query().Get("endIndex") != "49" {
			t.Fatal("unsafe fallback request")
		}
		var games []string
		for i := 1; i <= 50; i++ {
			games = append(games, fmt.Sprintf(`{"gameId":%d,"participants":[{"puuid":"target"}]}`, i))
		}
		return jsonResponse(`{"games":{"gameIndexBegin":0,"games":[` + strings.Join(games, ",") + `]}}`), nil
	})
	s := &TencentLCU{client: c}
	p, err := s.HistoryPage(context.Background(), "target", 20)
	if err != nil || len(p.Matches) != 30 || p.Matches[0].GameID != 21 || p.NextOffset != 50 || p.HasMore || p.LimitReason == "" || calls != 3 {
		t.Fatalf("limited prefix fallback incorrect: %v", err)
	}
	if _, err = s.HistoryPage(context.Background(), "target", 60); err == nil {
		t.Fatal("out-of-range prefix accepted")
	}
}

func TestSGPRejectsRedirectsAndRedactsErrors(t *testing.T) {
	requests := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(200) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	r := &sgpHistory{http: &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, origin: server.URL, token: "synthetic-secret"}
	_, status, err := r.get(context.Background(), "/test?puuid=sensitive")
	if err == nil || status != 302 || requests != 0 || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("redirect followed or sensitive error exposed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = r.get(ctx, "/test"); err == nil {
		t.Fatal("cancel ignored")
	}
}
