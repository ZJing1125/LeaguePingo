package diagnostics

import (
	"encoding/json"
	"strings"
	"sync"
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

func TestLatencyP95HasBoundedRecentWindowAndPreservesZero(t *testing.T) {
	c := &Collector{}
	for i := 0; i < 256; i++ {
		c.Record("/sgp-history", 10*time.Second, 500, true)
	}
	for i := 1; i <= 256; i++ {
		c.Record("/sgp-history", time.Duration(i)*time.Millisecond, 200, false)
	}
	m := c.Snapshot()["sgp-history"]
	if m.Calls != 512 || m.Failed != 256 || m.RecentSamples != 256 || m.P95MS == nil || *m.P95MS != 244 {
		t.Fatal(m)
	}
	c.Record("/sgp-group", 0, 200, false)
	z := c.Snapshot()["sgp-group"]
	if z.P95MS == nil || *z.P95MS != 0 || z.RecentSamples != 1 {
		t.Fatal("known zero latency lost", z)
	}
	*c.Snapshot()["sgp-history"].P95MS = 999999
	if *c.Snapshot()["sgp-history"].P95MS != 244 {
		t.Fatal("snapshot shares mutable percentile")
	}
	if len(c.recent) != 2 {
		t.Fatal("unbounded category window")
	}
}

func TestRequestLatencyConcurrentRecordingAndExport(t *testing.T) {
	c := &Collector{}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Record("/sgp-detail", time.Millisecond, 200, false)
			}
		}()
	}
	for i := 0; i < 100; i++ {
		c.Snapshot()
	}
	wg.Wait()
	m := c.Snapshot()["sgp-detail"]
	if m.Calls != 4000 || m.RecentSamples != 256 || m.P95MS == nil || *m.P95MS != 1 {
		t.Fatal(m)
	}
}
