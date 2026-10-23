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

const syntheticRanked = `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"PLATINUM","rank":"I","leaguePoints":39,"wins":36,"losses":38},{"queueType":"RANKED_FLEX_SR","tier":"SILVER","rank":"II","leaguePoints":27,"wins":20,"losses":30}],"seasons":{"RANKED_SOLO_5x5":{"currentSeasonId":37},"RANKED_FLEX_SR":{"currentSeasonId":38}}}`

func rankedTestSource(t *testing.T, response string) (*TencentLCU, *int, *int) {
	t.Helper()
	local, remote := new(int), new(int)
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{}), sgp: &sgpHistory{}}
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*local++
		switch r.URL.Path {
		case "/lol-rso-auth/v1/authorization":
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		case "/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint":
			return jsonResponse(`"https://nj100-sgp.lol.qq.com:21019"`), nil
		case "/lol-league-session/v1/league-session-token":
			return jsonResponse(`"synthetic-ranked-token"`), nil
		case "/entitlements/v1/token":
			return jsonResponse(`{"accessToken":"synthetic-history-token"}`), nil
		case "/lol-ranked/v1/ranked-stats/target":
			return jsonResponse(`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"PLATINUM","division":"I","leaguePoints":39,"wins":178,"losses":0},{"queueType":"RANKED_FLEX_SR","tier":"SILVER","division":"II","leaguePoints":27,"wins":2,"losses":3}]}`), nil
		case "/lol-summoner/v2/summoners/puuid/target":
			return jsonResponse(`{"puuid":"target","summonerLevel":100}`), nil
		default:
			t.Fatal("unexpected local request; endpoint redacted")
			return nil, nil
		}
	})
	s.sgp.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*remote++
		if r.URL.Scheme != "https" || r.URL.Host != "nj100-sgp.lol.qq.com:21019" || r.Method != "GET" {
			t.Fatal("rank request used an unverified destination or method")
		}
		if r.URL.Path == "/match-history-query/v1/products/lol/player/target/SUMMARY" {
			if r.Header.Get("Authorization") != "Bearer synthetic-history-token" {
				t.Fatal("history received the ranked token")
			}
			return jsonResponse(`{"games":[]}`), nil
		}
		if r.URL.Path != "/leagues-ledge/v2/rankedStats/puuid/target" || r.Header.Get("Authorization") != "Bearer synthetic-ranked-token" {
			t.Fatal("wrong ranked endpoint or token type")
		}
		return jsonResponse(response), nil
	})}
	return s, local, remote
}

func TestUnifiedProfileAndRankQueryUseRankedServiceCounters(t *testing.T) {
	s, local, remote := rankedTestSource(t, syntheticRanked)
	// Reproduce a local zero-loss response and a different ranked-service
	// record. These are synthetic counters, not the reported user's results.
	fallback, err := s.lcuRanks(context.Background(), "target")
	if err != nil || fallback[0].Losses == nil || *fallback[0].Losses != 0 {
		t.Fatal("local zero-loss fixture failed")
	}
	p := s.Profile(context.Background(), "target")
	if p.RankState != "ready" || p.Level == nil || len(p.Ranks) != 2 || *local != 5 || *remote != 1 {
		t.Fatal("unified profile failed or repeated local rank lookup")
	}
	for i, want := range []struct {
		losses, season int
		division       string
	}{{38, 37, "I"}, {30, 38, "II"}} {
		rank := p.Ranks[i]
		if rank.Source != "sgp" || rank.Losses == nil || *rank.Losses != want.losses || rank.SeasonID == nil || *rank.SeasonID != want.season || rank.Division != want.division {
			t.Fatal("ranked source, counter, season or division lost")
		}
	}
	if s.Ranks(context.Background(), "target").RankState != "ready" || *local != 5 || *remote != 2 {
		t.Fatal("rank-only query did not reuse its session")
	}
	if _, err := s.HistoryPage(context.Background(), "target", 20); err != nil || *local != 6 || *remote != 3 {
		t.Fatal("ranked and history credentials were confused or origin not reused")
	}
	s.ResetIdentity()
	if s.sgp.leagueToken != "" || s.sgp.token != "" || !s.sgp.leagueExpires.IsZero() || !s.sgp.expires.IsZero() {
		t.Fatal("account reset retained authorization")
	}
}

func TestRankedFallbackIsPerQueueAndNeverStitchesCounters(t *testing.T) {
	s, _, _ := rankedTestSource(t, `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD","rank":"IV","leaguePoints":5,"wins":36}]}`)
	p := s.Ranks(context.Background(), "target")
	if len(p.Ranks) != 2 || p.Ranks[0].Source != "sgp" || p.Ranks[0].Tier != "GOLD" || p.Ranks[0].Losses != nil || p.Ranks[1].Source != "lcu" || *p.Ranks[1].Losses != 3 {
		t.Fatal("fallback mixed different sources within a queue or dropped the other queue")
	}
}

func TestRankedUnauthorizedClearsOnlyRankedTokenAndDoesNotRetry(t *testing.T) {
	s, local, remote := rankedTestSource(t, syntheticRanked)
	if _, err := s.sgpRanks(context.Background(), "target"); err != nil {
		t.Fatal(err)
	}
	s.sgp.token, s.sgp.expires = "synthetic-history", time.Now().Add(time.Minute)
	s.sgp.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		*remote++
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("sensitive response must not be retained"))}, nil
	})
	p := s.Ranks(context.Background(), "target")
	if p.RankState != "ready" || p.Ranks[0].Source != "lcu" || *remote != 2 || *local != 4 || s.sgp.leagueToken != "" || s.sgp.token != "synthetic-history" {
		t.Fatal("authorization failure retried or damaged history authorization")
	}
}

func TestSlowRankedServiceLeavesTimeForLocalProfile(t *testing.T) {
	s, _, _ := rankedTestSource(t, syntheticRanked)
	s.sgp.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p := s.Profile(ctx, "target")
	if p.RankState != "ready" || p.Level == nil || p.Ranks[0].Source != "lcu" || ctx.Err() != nil {
		t.Fatal("ranked service consumed the entire existing profile budget")
	}
}

func TestRankedStatsDecodeDoesNotDependOnContainerOrder(t *testing.T) {
	for _, body := range []string{
		`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD"}],"queueMap":{"RANKED_SOLO_5x5":{"tier":"GOLD","wins":36,"losses":38}}}`,
		`{"queues":{"RANKED_SOLO_5x5":{"tier":"GOLD","wins":36,"losses":38}},"queueMap":{"RANKED_SOLO_5x5":{"tier":"GOLD"}}}`,
	} {
		ranks, err := decodeRankedStats([]byte(body), "target", "lcu")
		if err != nil || len(ranks) != 1 || ranks[0].Losses == nil || *ranks[0].Losses != 38 {
			t.Fatal("partial first container suppressed available ranked counters")
		}
	}
	ranks, err := decodeRankedStats([]byte(`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD","wins":36,"losses":0}],"queueMap":{"RANKED_SOLO_5x5":{"tier":"GOLD","wins":36,"losses":38},"RANKED_FLEX_SR":{"tier":"SILVER","wins":20,"losses":30}}}`), "target", "lcu")
	if err != nil || len(ranks) != 1 || ranks[0].Queue != "RANKED_FLEX_SR" {
		t.Fatal("conflicting duplicates were resolved by guessing or lost the independent queue")
	}
}

func TestRankedServicePreservesOptionalCountsAndVerifiedIdentity(t *testing.T) {
	for _, test := range []struct {
		body           string
		missing, empty bool
	}{
		{`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD","rank":"II","wins":3}]}`, true, false},
		{`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD","rank":"II","wins":3,"losses":0}]}`, false, false},
		{`{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"NONE","wins":0,"losses":0}]}`, false, false},
		{`{"queues":[{"queueType":"RANKED_TFT","tier":"GOLD","wins":3,"losses":4}]}`, false, true},
	} {
		ranks, err := decodeRankedStats([]byte(test.body), "target", "sgp")
		if err != nil || (len(ranks) == 0) != test.empty {
			t.Fatal("optional ranked response rejected or unrelated queue accepted")
		}
		if !test.empty && ((ranks[0].Losses == nil) != test.missing || (ranks[0].Losses != nil && *ranks[0].Losses != 0)) {
			t.Fatal("missing or explicit zero loss altered")
		}
	}
	if _, err := decodeRankedStats([]byte(`{"puuid":"other","queues":[]}`), "target", "sgp"); err == nil {
		t.Fatal("ranked identity mismatch accepted")
	}
}

func TestRankedAuthorizationRejectsWrongTokenShapeAndUntrustedOrigin(t *testing.T) {
	for _, body := range []string{`{"accessToken":"wrong-type"}`, `""`, `"bad\r\ntoken"`} {
		s, _, remote := rankedTestSource(t, syntheticRanked)
		original := s.client.HTTP.Transport
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/lol-league-session/v1/league-session-token" {
				return jsonResponse(body), nil
			}
			return original.RoundTrip(r)
		})
		if _, err := s.sgpRanks(context.Background(), "target"); err == nil || *remote != 0 {
			t.Fatal("wrong token contract reached the ranked service")
		}
	}
	s, _, remote := rankedTestSource(t, syntheticRanked)
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/lol-rso-auth/v1/authorization" {
			return jsonResponse(`{"currentPlatformId":"NJ100"}`), nil
		}
		return jsonResponse(`"https://evil.example"`), nil
	})
	if _, err := s.sgpRanks(context.Background(), "target"); err == nil || *remote != 0 {
		t.Fatal("unverified ranked destination was accepted")
	}
}
