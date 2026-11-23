package source

import (
	"context"
	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
	"net/http"
	"testing"
)

func TestAccountNameReusesVerifiedCurrentSummonerInLobby(t *testing.T) {
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
	calls := 0
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.Path {
		case "/lol-summoner/v1/current-summoner":
			return jsonResponse(`{"puuid":"target","gameName":"Current","tagLine":"TEST"}`), nil
		case "/lol-lobby/v2/lobby":
			return jsonResponse(`{"members":[{"puuid":"target"}],"gameConfig":{"queueId":2400,"gameMode":"UNRECOGNIZED"}}`), nil
		default:
			t.Fatal("name enrichment made an additional profile request")
			return nil, nil
		}
	})
	id, err := s.AccountIdentity(context.Background())
	if err != nil || id != "target" {
		t.Fatal("current identity failed")
	}
	name, err := s.Name(context.Background(), id)
	if err != nil || name != "Current#TEST" || calls != 1 {
		t.Fatal("current name was discarded")
	}
	roster, mode, err := s.ReadLobby(context.Background())
	if err != nil || len(roster) != 1 || roster[0].DisplayName != name || mode.QueueID != 2400 || calls != 2 {
		t.Fatal("room lost current-account name or queue")
	}
	s.ResetIdentity()
	if s.selfName != "" || s.selfPUUID != "" {
		t.Fatal("name survived identity reset")
	}
}

func TestCurrentAccountKeepsNameAcrossEmptyStartupResponses(t *testing.T) {
	s := &TencentLCU{client: lcu.NewClient(lcu.Credentials{})}
	body := `{"puuid":"first","gameName":"Ready","tagLine":"TEST"}`
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { return jsonResponse(body), nil })
	for _, next := range []string{body, `{"puuid":"first"}`} {
		body = next
		if _, err := s.AccountIdentity(context.Background()); err != nil {
			t.Fatal(err)
		}
		if s.CachedAccountName("first") != "Ready#TEST" {
			t.Fatal("valid name was lost")
		}
	}
	body = `{"puuid":"second"}`
	if _, err := s.AccountIdentity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.CachedAccountName("first") != "" || s.CachedAccountName("second") != "" {
		t.Fatal("name crossed accounts")
	}
}

func TestMayhemTimelineDoesNotTreatAbsentRunesAsPartial(t *testing.T) {
	s := &TencentLCU{}
	detail := core.MatchDetail{Players: []core.MatchPlayer{{Target: true, Summary: core.Match{QueueID: 2400, AugmentIDs: []int{1234}}}}}
	timeline := s.timelineWithRunes(context.Background(), core.MatchTimeline{}, detail, "target")
	if timeline.Partial {
		t.Fatal("non-rune mode was marked incomplete")
	}
}
