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
