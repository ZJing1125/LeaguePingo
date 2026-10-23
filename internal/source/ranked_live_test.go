package source

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

// Explicit one-player source comparison. No history scans, WeGame cookies,
// raw responses, names, IDs, authorization values or full URLs are logged.
func TestLiveRankedSourceComparison(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_RANK_TEST") != "1" {
		t.Skip("explicit one-player ranked-source comparison only")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal("local client unavailable (redacted)")
	}
	defer c.HTTP.CloseIdleConnections()
	s := &TencentLCU{client: c}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var puuid string
	if target := os.Getenv("LEAGUEPINGO_LIVE_RANK_TARGET"); target != "" {
		player, lookupErr := s.FindByRiotID(ctx, target)
		if lookupErr != nil || !core.IsKnownPUUID(player.PUUID) {
			t.Fatal("target identity lookup failed (redacted)")
		}
		puuid = player.PUUID
	} else {
		puuid, err = s.currentPUUID(ctx)
		if err != nil {
			t.Fatal("current identity unavailable (redacted)")
		}
	}
	localCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	raw, localErr := c.Get(localCtx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(puuid))
	stop()
	if localErr != nil {
		t.Log("source=lcu request=failed; details redacted")
	} else {
		// Separately decode each container, so a conflict is visible rather
		// than silently hidden by normalization. Only allow-listed fields log.
		var containers struct {
			PUUID    string          `json:"puuid"`
			Queues   json.RawMessage `json:"queues"`
			QueueMap json.RawMessage `json:"queueMap"`
		}
		if json.Unmarshal(raw, &containers) != nil || (containers.PUUID != "" && containers.PUUID != puuid) {
			t.Fatal("local ranked response invalid or identity mismatched (redacted)")
		}
		for _, part := range []struct {
			name string
			raw  json.RawMessage
		}{{"queues", containers.Queues}, {"queueMap", containers.QueueMap}} {
			payload, _ := json.Marshal(map[string]json.RawMessage{part.name: part.raw})
			ranks, parseErr := decodeRankedStats(payload, puuid, "lcu")
			if parseErr != nil {
				t.Logf("source=lcu container=%s parse=failed", part.name)
				continue
			}
			logRankComparison(t, "lcu/"+part.name, ranks)
		}
	}
	remoteCtx, done := context.WithTimeout(ctx, rankedAttemptBudget)
	started := time.Now()
	ranks, remoteErr := s.sgpRanks(remoteCtx, puuid)
	t.Logf("source=sgp elapsedMs=%d budgetMs=%d", time.Since(started).Milliseconds(), rankedAttemptBudget.Milliseconds())
	done()
	defer func() {
		if s.sgp != nil && s.sgp.http != nil {
			s.sgp.http.CloseIdleConnections()
		}
		s.ResetIdentity()
	}()
	if remoteErr != nil {
		// sgpRanks only returns fixed sanitized errors, never raw HTTP errors.
		t.Fatalf("source=sgp failure=%s; comparison incomplete", remoteErr)
	}
	logRankComparison(t, "sgp", ranks)
	if localErr != nil || len(ranks) == 0 {
		t.Fatal("ranked-source comparison incomplete")
	}
	t.Log("comparison read only; matching queue and season/split scope must be checked before comparing WeGame totals")
}

func logRankComparison(t *testing.T, source string, ranks []core.Rank) {
	t.Helper()
	value := func(p *int) string {
		if p == nil {
			return "missing"
		}
		return strconv.Itoa(*p)
	}
	if len(ranks) == 0 {
		t.Logf("source=%s supported-queues=0", source)
	}
	for _, rank := range ranks {
		// Queue/tier are decoder allow-lists. Division is deliberately omitted.
		t.Logf("source=%s queue=%s tier=%s lp=%s wins=%s losses=%s season=%s", source, rank.Queue, rank.Tier, value(rank.LP), value(rank.Wins), value(rank.Losses), value(rank.SeasonID))
	}
}
