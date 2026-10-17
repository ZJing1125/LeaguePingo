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

func TestBasicProfileSkipsRankAndFullProfileGetsAvatarBeforeSlowRank(t *testing.T) {
	icon, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=")
	for _, full := range []bool{false, true} {
		s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
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
		s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
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
		s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
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
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
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
		s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
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
