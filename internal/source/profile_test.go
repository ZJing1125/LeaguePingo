package source

import (
	"context"
	"encoding/base64"
	"io"
	"leaguepingo/internal/lcu"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Existing profile cases exercise the local fallback. The independent ranked
// endpoint fails explicitly and immediately, without real network traffic.
func profileTestSource() *TencentLCU {
	return &TencentLCU{client: lcu.NewClient(lcu.Credentials{}), sgp: &sgpHistory{
		origin: "https://nj100-sgp.lol.qq.com:21019", platform: "NJ100",
		leagueToken: "synthetic-league", leagueExpires: time.Now().Add(time.Minute),
		http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	}}
}

func TestBasicProfileSkipsRankAndFullProfileGetsAvatarBeforeSlowRank(t *testing.T) {
	icon, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=")
	for _, full := range []bool{false, true} {
		s := profileTestSource()
		var calls []string
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, r.URL.Path)
			body := `{"puuid":"target","profileIconId":123,"summonerLevel":100}`
			if strings.Contains(r.URL.Path, "profile-icons") {
				body = string(icon)
			}
			if strings.Contains(r.URL.Path, "ranked-stats") {
				if len(calls) != 3 || !strings.Contains(calls[1], "profile-icons") {
					t.Fatal("rank blocked avatar request")
				}
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		p := s.BasicProfile(ctx, "target")
		if full {
			calls = nil
			p = s.Profile(ctx, "target")
		}
		cancel()
		if p.ProfileIconData == "" || p.Level == nil || (full && len(calls) != 3) || (!full && len(calls) != 2) {
			t.Fatal("basic profile lost or unexpected rank call")
		}
	}
}

func TestProfileOptionalAndUnknownRanks(t *testing.T) {
	for _, body := range []string{`{}`, `{"queues":[]}`, `{"queues":[{"queueType":"RANKED_SOLO_5x5"}]}`, `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"UNKNOWN"}]}`} {
		s := profileTestSource()
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			data := body
			if strings.Contains(r.URL.Path, "summoners") {
				data = `{"puuid":"other","summonerLevel":99}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
		})
		p := s.Profile(context.Background(), "target")
		if p.RankState != "unavailable" || len(p.Ranks) != 0 || p.Level != nil {
			t.Fatal("unknown or mismatched profile accepted")
		}
	}
}

func TestProfileIconUsesVerifiedIDAndRejectsNonImage(t *testing.T) {
	icon, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=")
	for _, good := range []bool{true, false} {
		s := profileTestSource()
		iconCalls := 0
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := `{"queues":[]}`
			if strings.Contains(r.URL.Path, "summoners") {
				body = `{"puuid":"target","profileIconId":123}`
			}
			if strings.Contains(r.URL.Path, "profile-icons") {
				iconCalls++
				if r.URL.Path != "/lol-game-data/assets/v1/profile-icons/123.jpg" {
					t.Fatal("unexpected asset endpoint")
				}
				if good {
					body = string(icon)
				} else {
					body = "<html>error</html>"
				}
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		p := s.Profile(context.Background(), "target")
		if iconCalls != 1 || (p.ProfileIconData != "") != good {
			t.Fatal("verified icon handling failed")
		}
	}
}
func TestProfileParsesVerifiedLevelAndRanks(t *testing.T) {
	s := profileTestSource()
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data := `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"EMERALD","division":"II","leaguePoints":0,"wins":4,"losses":-1}]}`
		if strings.Contains(r.URL.Path, "summoners") {
			data = `{"puuid":"target","summonerLevel":50}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})
	p := s.Profile(context.Background(), "target")
	if p.Level == nil || *p.Level != 50 || len(p.Ranks) != 1 || p.Ranks[0].LP == nil || *p.Ranks[0].LP != 0 || p.Ranks[0].Losses != nil {
		t.Fatal("optional profile values parsed incorrectly")
	}
}

func TestProfilePrivacyRequiresMatchingIdentityAndExplicitValue(t *testing.T) {
	for _, test := range []struct{ body, want string }{
		{`{"puuid":"target","privacy":"PRIVATE"}`, "private"},
		{`{"puuid":"target","privacy":"PUBLIC"}`, "public"},
		{`{"puuid":"target","privacy":"OTHER"}`, "unknown"},
		{`{"puuid":"other","privacy":"PRIVATE"}`, "unknown"},
	} {
		s := profileTestSource()
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := "{}"
			if strings.Contains(r.URL.Path, "summoners") {
				body = test.body
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if got := s.Profile(context.Background(), "target"); got.PrivacyState != test.want {
			t.Fatalf("privacy=%s want=%s", got.PrivacyState, test.want)
		}
	}
}

func TestRankedQueueMapIsAccepted(t *testing.T) {
	s := profileTestSource()
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"puuid":"target","summonerLevel":50}`
		if strings.Contains(r.URL.Path, "ranked-stats") {
			body = `{"queueMap":{"RANKED_SOLO_5x5":{"tier":"EMERALD","division":"II","leaguePoints":0}}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	p := s.Profile(context.Background(), "target")
	if p.RankState != "ready" || len(p.Ranks) != 1 || p.Ranks[0].LP == nil || *p.Ranks[0].LP != 0 {
		t.Fatal("verified queueMap rank was lost")
	}
}

func TestLCURanksTreatNonSelfZeroLossAsRestrictedWithoutIdentityRequest(t *testing.T) {
	for _, self := range []string{"", "other", "target"} {
		for _, layout := range []string{"queues", "queueMap"} {
			t.Run(self+"/"+layout, func(t *testing.T) {
				s := profileTestSource()
				s.selfPUUID = self
				calls := 0
				s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if r.URL.Path != "/lol-ranked/v1/ranked-stats/target" {
						t.Fatal("local fallback added an identity request")
					}
					body := `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"PLATINUM","division":"I","leaguePoints":39,"wins":178,"losses":0},{"queueType":"RANKED_FLEX_SR","tier":"SILVER","wins":26,"losses":27}]}`
					if layout == "queueMap" {
						body = `{"queueMap":{"RANKED_SOLO_5x5":{"tier":"PLATINUM","division":"I","leaguePoints":39,"wins":178,"losses":0},"RANKED_FLEX_SR":{"tier":"SILVER","wins":26,"losses":27}}}`
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
				})
				p := s.Ranks(context.Background(), "target")
				if calls != 1 || len(p.Ranks) != 2 || p.RankState != "ready" {
					t.Fatal("rank response/request budget changed")
				}
				for _, rank := range p.Ranks {
					if rank.Queue == "RANKED_SOLO_5x5" {
						if rank.Wins == nil || *rank.Wins != 178 || rank.LP == nil || *rank.LP != 39 {
							t.Fatal("available rank fields were discarded")
						}
						if self == "target" && (rank.Losses == nil || *rank.Losses != 0) {
							t.Fatal("verified self zero-loss record was discarded")
						}
						if self != "target" && rank.Losses != nil {
							t.Fatal("restricted non-self zero loss was treated as an undefeated record")
						}
					} else if rank.Losses == nil || *rank.Losses != 27 {
						t.Fatal("independent flex losses changed")
					}
				}
			})
		}
	}
}

func TestRanksOnlyRequestAndVerifiedFormats(t *testing.T) {
	for _, test := range []struct{ name, body, state, tier string }{
		{"array", `{"queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD"}]}`, "ready", "GOLD"},
		{"map", `{"queues":{"RANKED_SOLO_5x5":{"tier":"EMERALD"}}}`, "ready", "EMERALD"},
		{"unranked", `{"queueMap":{"RANKED_SOLO_5x5":{"tier":"NONE"}}}`, "ready", "NONE"},
		{"missing", `{}`, "unavailable", ""},
		{"empty", `{"queues":[]}`, "unavailable", ""},
		{"unknown", `{"queueMap":{"RANKED_SOLO_5x5":{"tier":"UNKNOWN"}}}`, "unavailable", ""},
		{"conflict", `{"queueMap":{"RANKED_SOLO_5x5":{"queueType":"RANKED_FLEX_SR","tier":"GOLD"}}}`, "unavailable", ""},
		{"wrong-player", `{"puuid":"other","queues":[{"queueType":"RANKED_SOLO_5x5","tier":"GOLD"}]}`, "failed", ""},
		{"invalid", `{"queues":false}`, "failed", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := profileTestSource()
			calls := 0
			s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != "/lol-ranked/v1/ranked-stats/target" {
					t.Fatal("rank retry fetched unrelated profile data")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			p := s.Ranks(context.Background(), "target")
			if calls != 1 || p.RankState != test.state {
				t.Fatal("wrong rank state/request count", p.RankState, calls)
			}
			if test.tier != "" && (len(p.Ranks) != 1 || p.Ranks[0].Tier != test.tier) {
				t.Fatal("verified rank lost")
			}
			if test.tier == "" && (len(p.Ranks) != 0 || p.RankMessage == "") {
				t.Fatal("missing rank reported without reason")
			}
			if len(s.Ranks(context.Background(), "").Ranks) != 0 || calls != 1 {
				t.Fatal("unknown identity triggered request")
			}
		})
	}
}

func TestRankTimeoutIsRecoverableAndDoesNotExposeSourceDetails(t *testing.T) {
	s := profileTestSource()
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	p := s.Ranks(ctx, "target")
	if p.RankState != "failed" || p.RankMessage != "段位查询超时，可重试" || len(p.Ranks) != 0 {
		t.Fatal("rank timeout was hidden or exposed raw error")
	}
}

func TestSlowAvatarDoesNotPreventUnifiedRankResponse(t *testing.T) {
	s := profileTestSource()
	ranked := false
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"puuid":"target","summonerLevel":100,"profileIconId":123}`
		if strings.Contains(r.URL.Path, "profile-icons") {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		if strings.Contains(r.URL.Path, "ranked-stats") {
			ranked = true
			body = `{"queueMap":{"RANKED_SOLO_5x5":{"tier":"GOLD"}}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p := s.Profile(ctx, "target")
	if !ranked || p.RankState != "ready" || p.Level == nil || p.ProfileIconData != "" {
		t.Fatal("slow avatar starved unified rank response")
	}
}
