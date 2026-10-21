package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

func TestScoutSeasonRequiresExplicitCurrentServiceBounds(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	for _, factor := range []int64{1, 1000} {
		raw := fmt.Sprintf(`{"seasonStart":%d,"seasonEnd":%d,"metadata":{"publicName":"Current","year":2026}}`, now.Add(-time.Hour).Unix()*factor, now.Add(time.Hour).Unix()*factor)
		for _, body := range []string{raw, "[" + raw + "]"} {
			s, err := parseScoutSeason([]byte(body), now)
			if err != nil || s.Name != "Current" || !s.Start.Equal(now.Add(-time.Hour)) {
				t.Fatal("current bounds not parsed")
			}
		}
	}
	for _, body := range []string{`{}`, `[]`, `{"seasonStart":1,"seasonEnd":2}`, `{"metadata":{"year":2026}}`} {
		if _, err := parseScoutSeason([]byte(body), now); err == nil {
			t.Fatal("missing/expired season inferred from year or system date")
		}
	}
}
func TestChampionMasteryReadOnlyIdentityAndNoFabricatedCounts(t *testing.T) {
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
	for _, mismatch := range []bool{false, true} {
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" || r.URL.Path != "/lol-champion-mastery/v1/target/champion-mastery" {
				t.Fatal("unexpected request")
			}
			id := "target"
			if mismatch {
				id = "another"
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`[{"puuid":%q,"championId":23,"championLevel":30,"championPoints":900000}]`, id)))}, nil
		})
		m, err := s.ChampionMasteries(context.Background(), "target")
		if mismatch {
			if err == nil || len(m) > 0 {
				t.Fatal("identity mismatch accepted")
			}
		} else if err != nil || len(m) != 1 || m[0].Points != 900000 {
			t.Fatal("mastery not retained")
		}
	}
	if _, err := s.ScoutPage(context.Background(), "", 0); err == nil {
		t.Fatal("unknown identity allowed")
	}
	s.rememberQueue([]byte(`{"gameConfig":{"queueId":440}}`), true)
	if s.CurrentQueue() != 440 {
		t.Fatal("lobby queue not cached")
	}
	s.rememberQueue([]byte(`{"gameData":{"queue":{"id":420}}}`), false)
	if s.CurrentQueue() != 420 {
		t.Fatal("game queue not cached")
	}
	s.rememberQueue([]byte(`{"gameConfig":{"queueId":0}}`), true)
	if s.CurrentQueue() != 0 {
		t.Fatal("custom game retained the previous ranked queue")
	}
}
