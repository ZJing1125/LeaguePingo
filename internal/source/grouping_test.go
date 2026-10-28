package source

import (
	"context"
	"net/http"
	"testing"
)

func TestCurrentGroupingRequiresSameGameAndSelf(t *testing.T) {
	raw := []byte(`{"game":{"id":42,"teamOne":[{"puuid":"enemy-a","teamParticipantId":8},{"puuid":"enemy-b","teamParticipantId":8}],"teamTwo":[{"puuid":"self","teamParticipantId":9,"selectedPosition":"MIDDLE"},{"puuid":"ally","teamParticipantId":9}]},"playerCredentials":{"encryptionKey":"do-not-export","serverIp":"do-not-export"}}`)
	ps, err := parseCurrentGrouping(raw, "self", 42)
	if err != nil || len(ps) != 4 || ps[0].PUUID != "self" || ps[0].Position != "mid" || ps[0].PremadeSize != 2 || ps[2].Team != "enemy" {
		t.Fatalf("bad GSM mapping %+v %v", ps, err)
	}
	if _, err = parseCurrentGrouping(raw, "self", 43); err == nil {
		t.Fatal("accepted another match")
	}
	if _, err = parseCurrentGrouping(raw, "other", 42); err == nil {
		t.Fatal("accepted missing local account")
	}
}

func TestGroupingUsesVerifiedCurrentRegionAndLeagueToken(t *testing.T) {
	s, _, _ := rankedTestSource(t, syntheticRanked)
	local := &sgpHistory{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "nj100-sgp.lol.qq.com:21019" || r.URL.Path != "/gsm/v1/ledge/region/NJ100/puuid/self" || r.Method != "GET" {
			t.Fatal("unverified grouping destination")
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-ranked-token" {
			t.Fatal("wrong grouping token")
		}
		return jsonResponse(`{"game":{"id":42,"teamOne":[{"puuid":"self","teamParticipantId":2}],"teamTwo":[]}}`), nil
	})}}
	ps, err := s.requestGrouping(context.Background(), "self", 42, local)
	if err != nil || len(ps) != 1 {
		t.Fatal(err)
	}
}
