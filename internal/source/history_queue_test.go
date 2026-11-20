package source

import (
	"context"
	"fmt"
	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProductionQueueVerificationAndRuntimeMismatch(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/lol-summoner/v1/current-summoner":
			return jsonResponse(`{"puuid":"target"}`), nil
		case "/lol-gameflow/v1/gameflow-phase":
			return jsonResponse(`"Lobby"`), nil
		case "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case "/entitlements/v1/token":
			return jsonResponse(`{"accessToken":"synthetic"}`), nil
		default:
			t.Fatal("unexpected local request")
			return nil, nil
		}
	})
	broken := false
	calls := 0
	remote := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		offset, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
		filtered := r.URL.Query().Get("tag") == "q_2400"
		rows := []string{}
		for i := offset; i < offset+20; i++ {
			id, q := i+1, 420
			if i%2 == 0 {
				q = 2400
			}
			if filtered {
				id, q = 2*i+1, 2400
			}
			if broken && filtered {
				q = 420
			}
			rows = append(rows, fmt.Sprintf(`{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_%d"},"json":{"gameId":%d,"platformId":"NJ100","queueId":%d,"participants":[{"puuid":"target","kills":2,"deaths":1,"assists":3}]}}`, id, id, q))
		}
		return jsonResponse(`{"games":[` + strings.Join(rows, ",") + `]}`), nil
	})}
	s := &TencentLCU{client: c, sgp: &sgpHistory{http: remote}}
	if s.SupportsHistoryQueue(2400) {
		t.Fatal("enabled before verification")
	}
	result, err := s.VerifyHistoryQueue(context.Background(), 2400)
	if err != nil || result.State != "verified" || result.Compared != 30 || !s.SupportsHistoryQueue(2400) || calls != 6 {
		t.Fatalf("production audit failed: %+v %v calls=%d", result, err, calls)
	}
	broken = true
	if _, err = s.HistoryPageForQueue(context.Background(), "target", 2400, 0); apperrors.CodeOf(err) != apperrors.Unsupported || s.SupportsHistoryQueue(2400) {
		t.Fatal("runtime wrong-queue contract did not disable candidate")
	}
	s.ResetIdentity()
	if s.SupportsHistoryQueue(2400) {
		t.Fatal("capability survived connection reset")
	}
}
func TestQueueCacheHeadIsolationAndDuplicates(t *testing.T) {
	r := &sgpHistory{}
	root := "/match-history-query/v1/products/lol/player/target/SUMMARY?count=20&startIndex=20"
	mixed := core.HistoryPage{Offset: 20, Matches: []core.Match{{GameID: 2, QueueID: 420}}}
	filtered := core.HistoryPage{Offset: 20, Matches: []core.Match{{GameID: 3, QueueID: 2400}}}
	r.rememberHead("target", 1)
	r.pages.put(root, mixed, time.Now())
	r.rememberHead("target/q_2400", 3)
	if _, ok := r.pages.get(root, time.Now()); !ok {
		t.Fatal("filtered initialization cleared mixed cache")
	}
	r.pages.put(root+"&tag=q_2400", filtered, time.Now())
	repeated := core.HistoryPage{Offset: 40, NextOffset: 41, Meta: core.HistoryMeta{Returned: 1}, Matches: []core.Match{{GameID: 3, QueueID: 2400}}}
	if validQueuePage(r, strings.Replace(root, "startIndex=20", "startIndex=40", 1)+"&tag=q_2400", repeated, 2400) {
		t.Fatal("repeated adjacent queue page accepted")
	}
	r.rememberHead("target/q_2400", 4)
	if _, ok := r.pages.get(root, time.Now()); !ok {
		t.Fatal("filtered head invalidated mixed page")
	}
	r.rememberHead("target", 5)
	if len(r.pages.items) != 0 {
		t.Fatal("new mixed history kept stale filtered pages")
	}
}
func TestQueueComparisonRejectsInsufficientChangedAndWrongOrder(t *testing.T) {
	mixed := make([]core.Match, 30)
	for i := range mixed {
		mixed[i] = core.Match{GameID: int64(i + 1), QueueID: 2400}
	}
	if state, _ := compareQueueHistory(mixed[:20], mixed[:20], mixed[:1], 2400); state != "insufficient" {
		t.Fatal(state)
	}
	tagged := append([]core.Match{}, mixed...)
	tagged[2], tagged[3] = tagged[3], tagged[2]
	if state, _ := compareQueueHistory(mixed, tagged, mixed[:1], 2400); state != "mismatch" {
		t.Fatal(state)
	}
	if state, _ := compareQueueHistory(mixed, mixed, mixed[1:2], 2400); state != "changed" {
		t.Fatal(state)
	}
}

func TestQueueAuditGuardRefusesActiveGameBeforeIdentityRead(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/lol-gameflow/v1/gameflow-phase" {
			t.Fatal("read beyond phase guard")
		}
		return jsonResponse(`"InProgress"`), nil
	})
	s := &TencentLCU{client: c}
	if err := s.queueAuditGuard(context.Background(), "target", 0); apperrors.CodeOf(err) != apperrors.QueryBusy || calls != 1 {
		t.Fatal("audit was allowed during game")
	}
}
