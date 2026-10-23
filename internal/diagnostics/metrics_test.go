package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMetricsNeverRetainIdentityOrError(t *testing.T) {
	c := &Collector{}
	c.Record("/lol-summoner/v1/summoners?name=SECRET-NAME", time.Second, 500, true)
	c.Record("/lol-match-history/v1/products/lol/SECRET-PUUID/matches", time.Millisecond, 200, false)
	b, _ := json.Marshal(c.Snapshot())
	if strings.Contains(string(b), "SECRET") || len(c.Snapshot()) != 2 || c.Snapshot()["profile"].Failed != 1 {
		t.Fatal("unsafe metrics")
	}
}

func TestRankMetricsSeparateRemoteStatsFromSessionAuthorization(t *testing.T) {
	c := &Collector{}
	c.Record("/sgp-rank", time.Millisecond, 200, false)
	c.Record("/lol-league-session/v1/league-session-token", time.Millisecond, 401, true)
	if len(c.Snapshot()) != 2 || c.Snapshot()["sgp-rank"].Calls != 1 || c.Snapshot()["rank-session"].Failed != 1 {
		t.Fatal("rank source and session failure cannot be distinguished")
	}
}
