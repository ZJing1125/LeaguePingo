package source

import (
	"context"
	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTimelineLocalIdentityFailureNeverFallsBackToAnotherGame(t *testing.T) {
	calls := 0
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/lol-match-history/v1/game-timelines/42" {
			t.Fatal("unexpected timeline request")
		}
		return jsonResponse(`{"gameId":43,"frames":[]}`), nil
	})
	detail := core.MatchDetail{GameID: 42, Partial: true, Players: []core.MatchPlayer{{ParticipantID: 1, Target: true}}}
	if _, err := s.MatchTimeline(context.Background(), detail, "target"); err == nil || calls != 1 {
		t.Fatal("invalid successful response fell back or was accepted")
	}
}
func TestUnifiedProfileOptionalMasterySharesParentBudget(t *testing.T) {
	s, _, _ := rankedTestSource(t, syntheticRanked)
	original := s.client.HTTP.Transport
	masteryCalls := 0
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "champion-mastery") {
			masteryCalls++
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > 310*time.Millisecond {
				t.Fatal("optional mastery lacks its subdeadline")
			}
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return original.RoundTrip(r)
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got := s.Profile(ctx, "target")
	if ctx.Err() != nil || got.RankState != "ready" || got.Masteries != nil || masteryCalls != 1 {
		t.Fatal("optional timeout discarded valid ranks or started another round")
	}
}
