package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/lcu"
)

func TestHistoryPaginationUsesRawOffsetAndVerifiedIdentity(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("begIndex") != "20" || r.URL.Query().Get("endIndex") != "39" {
			t.Fatal("wrong page range")
		}
		body := `{"games":{"games":[{"gameId":21,"participants":[{"puuid":"target"}]},{"gameId":22,"participants":[{"puuid":"other"}]}]}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	s := &TencentLCU{client: c}
	p, err := s.HistoryPage(context.Background(), "target", 20)
	if err != nil || len(p.Matches) != 1 || p.Meta.Excluded != 1 || p.NextOffset != 22 || p.HasMore {
		t.Fatalf("bad parsed page: %v", err)
	}
	for _, offset := range []int{-1, 1000} {
		if _, err := s.HistoryPage(context.Background(), "target", offset); err == nil {
			t.Fatal("invalid range accepted")
		}
	}
	if calls != 1 {
		t.Fatal("invalid range sent a request")
	}
}

func TestDetailRequestsOneGameAndRejectsDifferentMatch(t *testing.T) {
	c := lcu.NewClient(lcu.Credentials{})
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/lol-match-history/v1/games/42" {
			t.Fatal("incorrect detail path")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"gameId":43,"participants":[{"participantId":1,"puuid":"target"}]}`))}, nil
	})
	if _, err := (&TencentLCU{client: c}).MatchDetail(context.Background(), 42, "target"); err == nil {
		t.Fatal("wrong game body accepted")
	}
}
