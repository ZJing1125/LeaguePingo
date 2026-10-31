package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/lcu"
)

type cancelingSGPBody struct{ cancel context.CancelFunc }

func (b cancelingSGPBody) Read([]byte) (int, error) {
	b.cancel()
	return 0, errors.New("private-response-body")
}
func (cancelingSGPBody) Close() error { return nil }

func TestSGPBodyReadFailureRetainsCancellationPrecedence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	r := &sgpHistory{origin: "https://synthetic.invalid", http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: cancelingSGPBody{cancel}}, nil
	})}}
	_, status, err := r.getAuthorized(ctx, "https://synthetic.invalid", "/synthetic", "synthetic-token", "/sgp-history", &serviceGate{})
	if status != 200 || calls != 1 || apperrors.CodeOf(err) != apperrors.RequestCanceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation or changed attempts: %v", err)
	}
}

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

func TestInitialHistoryUsesSameRichSGPSummaryAsLaterPages(t *testing.T) {
	remoteCalls := 0
	remote := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		remoteCalls++
		if r.URL.Query().Get("startIndex") != "0" || r.URL.Query().Get("count") != "20" {
			t.Fatal("initial history did not request the first summary page")
		}
		participants := []string{}
		for i := 1; i <= 10; i++ {
			team := 100
			puuid := fmt.Sprintf("player-%d", i)
			if i > 5 {
				team = 200
			}
			if i == 1 {
				puuid = "target"
			}
			participants = append(participants, fmt.Sprintf(`{"puuid":%q,"participantId":%d,"teamId":%d,"championId":%d,"kills":%d,"deaths":2,"assists":4}`, puuid, i, team, 20+i, i))
		}
		body := `{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_77"},"json":{"gameId":77,"platformId":"NJ100","queueId":420,"participants":[` + strings.Join(participants, ",") + `]}}]}`
		return jsonResponse(body), nil
	})}
	local := lcu.NewClient(lcu.Credentials{})
	local.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("rich SGP first page unexpectedly fell back to the incomplete LCU summary")
		return nil, nil
	})
	s := &TencentLCU{client: local, sgp: &sgpHistory{http: remote, origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "synthetic", expires: time.Now().Add(time.Minute)}}
	result, err := s.History(context.Background(), "target")
	if err != nil || remoteCalls != 1 || len(result.Matches) != 1 || len(result.Matches[0].Roster) != 10 || result.Matches[0].Participation == nil {
		t.Fatalf("initial rich history mismatch: %+v %v", result, err)
	}
}

func TestSGPScoutingAndBrowseSharePagesButChangedHeadInvalidatesCursorCache(t *testing.T) {
	calls, head := 0, 1
	r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "synthetic", expires: time.Now().Add(time.Minute)}
	r.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		offset, _ := strconv.Atoi(req.URL.Query().Get("startIndex"))
		id := offset + 1
		if offset == 0 {
			id = head
		}
		return jsonResponse(fmt.Sprintf(`{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_%d"},"json":{"gameId":%d,"platformId":"NJ100","queueId":420,"participants":[{"puuid":"target"}]}}]}`, id, id)), nil
	})}
	s := &TencentLCU{sgp: r}
	read := func(offset int) {
		t.Helper()
		if _, err := s.ScoutPage(context.Background(), "target", offset); err != nil {
			t.Fatal(err)
		}
	}
	read(0)
	read(20)
	read(20)
	if calls != 2 {
		t.Fatal("same immutable page was fetched twice")
	}
	head = 2
	read(0)
	read(20)
	if calls != 4 {
		t.Fatal("new game did not invalidate shifted cached pages")
	}
	if _, err := s.HistoryPage(context.Background(), "target", 20); err != nil || calls != 4 {
		t.Fatal("browse did not reuse matching scouting page")
	}
	s.ResetIdentity()
	if len(r.pages) > 0 || len(r.heads) > 0 {
		t.Fatal("account reset retained player cursors")
	}
}

func TestScoutingCursorCanPassTheUserListThousandRecordBoundary(t *testing.T) {
	r := &sgpHistory{origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100", token: "synthetic", expires: time.Now().Add(time.Minute)}
	r.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("startIndex") != "1000" {
			t.Fatal("scouting cursor was truncated")
		}
		return jsonResponse(`{"games":[{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_1001"},"json":{"gameId":1001,"platformId":"NJ100","queueId":420,"participants":[{"puuid":"target"}]}}]}`), nil
	})}
	s := &TencentLCU{sgp: r}
	p, err := s.ScoutPage(context.Background(), "target", 1000)
	if err != nil || p.NextOffset != 1001 {
		t.Fatal("independent season cursor inherited list limit")
	}
	if _, err := s.HistoryPage(context.Background(), "target", 1000); err == nil {
		t.Fatal("user list limit changed")
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
	gate := &serviceGate{}
	_, status, err := r.getAuthorized(context.Background(), r.origin, "/test?puuid=sensitive", r.token, "/sgp-history", gate)
	if err == nil || status != 302 || requests != 0 || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("redirect followed or sensitive error exposed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = r.getAuthorized(ctx, r.origin, "/test", r.token, "/sgp-history", gate); err == nil {
		t.Fatal("cancel ignored")
	}
}
