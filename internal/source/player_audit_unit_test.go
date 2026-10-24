package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func auditFixtureSource(t *testing.T, repeat bool) (*TencentLCU, *int) {
	t.Helper()
	s, _, _ := rankedTestSource(t, syntheticRanked)
	participants := []any{}
	frames := map[string]any{}
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("private-participant-%d", i)
		if i == 1 {
			id = "target"
		}
		team := 100
		if i > 5 {
			team = 200
		}
		p := map[string]any{"participantId": i, "puuid": id, "teamId": team, "championId": 1, "riotIdGameName": "private-display-name", "riotIdTagline": "PRIVATE", "kills": 1, "deaths": 0, "assists": 1, "win": i <= 5, "goldEarned": 200, "totalDamageDealtToChampions": 100, "totalDamageTaken": 50, "totalMinionsKilled": 10, "neutralMinionsKilled": 0, "champLevel": 2, "visionScore": 0, "wardsPlaced": 0, "wardsKilled": 0, "visionWardsBoughtInGame": 0, "spell1Id": 4, "spell2Id": 14, "teamPosition": "MIDDLE"}
		for slot := 0; slot < 7; slot++ {
			p[fmt.Sprintf("item%d", slot)] = 0
		}
		participants = append(participants, p)
		frames[strconv.Itoa(i)] = map[string]any{"participantId": i, "totalGold": 200, "minionsKilled": 10, "jungleMinionsKilled": 0, "level": 2, "position": map[string]int{"x": 1000, "y": 2000}}
	}
	stamp := time.Now().Add(-time.Hour).UnixMilli()
	game := map[string]any{"gameId": 42, "platformId": "NJ100", "queueId": 420, "mapId": 11, "gameCreation": stamp, "gameDuration": 600, "participants": participants}
	gameRaw, _ := json.Marshal(game)
	detail, _ := core.ParseSGPMatchDetail([]byte(`{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_42"},"json":`+string(gameRaw)+`}`), 42, "target", "NJ100")
	// Convert the same flat source to its LCU representation, independently of
	// normalizeSGPGame: the source comparison must exercise both parsers.
	localPlayers := []any{}
	for _, item := range participants {
		p := item.(map[string]any)
		out := map[string]any{}
		for k, v := range p {
			out[k] = v
		}
		out["stats"] = p
		localPlayers = append(localPlayers, out)
	}
	localGame := map[string]any{}
	for k, v := range game {
		localGame[k] = v
	}
	localGame["participants"] = localPlayers
	localRaw, _ := json.Marshal(localGame)
	timeline := map[string]any{"gameId": 42, "participants": []any{map[string]any{"participantId": 1, "puuid": "target"}}, "frames": []any{map[string]any{"timestamp": 0, "participantFrames": frames, "events": []any{}}, map[string]any{"timestamp": 60000, "participantFrames": frames, "events": []any{map[string]any{"timestamp": 30000, "type": "SKILL_LEVEL_UP", "participantId": 1, "skillSlot": 1}}}}}
	timelineRaw, _ := json.Marshal(timeline)
	original := s.client.HTTP.Transport
	s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/lol-gameflow/v1/session":
			return jsonResponse(`{"gameData":{"queue":{"id":420}}}`), nil
		case "/lol-seasons/v1/season/product/LOL":
			return jsonResponse(fmt.Sprintf(`{"seasonStart":%d,"seasonEnd":%d}`, stamp-60*24*60*60*1000, stamp+60*24*60*60*1000)), nil
		case "/lol-match-history/v1/games/42":
			return jsonResponse(string(localRaw)), nil
		case "/lol-match-history/v1/game-timelines/42":
			return jsonResponse(string(timelineRaw)), nil
		}
		return original.RoundTrip(r)
	})
	requests := new(int)
	s.sgp.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*requests++
		switch r.URL.Path {
		case "/leagues-ledge/v2/rankedStats/puuid/target":
			return jsonResponse(syntheticRanked), nil
		case "/match-history-query/v1/products/lol/NJ100_42/SUMMARY":
			return jsonResponse(`{"metadata":{"product":"lol","info_type":"summary","match_id":"NJ100_42"},"json":` + string(gameRaw) + `}`), nil
		case "/match-history-query/v1/products/lol/NJ100_42/DETAILS":
			return jsonResponse(`{"metadata":{"product":"lol","info_type":"details","match_id":"NJ100_42"},"json":` + string(timelineRaw) + `}`), nil
		case "/match-history-query/v1/products/lol/player/target/SUMMARY":
			offset, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
			if repeat {
				offset = 0
			}
			n := 20
			if offset == 40 {
				n = 2
			}
			games := []any{}
			for i := 0; i < n; i++ {
				id := 42 + offset + i
				g := map[string]any{}
				for k, v := range game {
					g[k] = v
				}
				g["gameId"] = id
				g["gameCreation"] = stamp - int64(offset+i)*3600000
				games = append(games, map[string]any{"metadata": map[string]any{"product": "lol", "info_type": "summary", "match_id": fmt.Sprintf("NJ100_%d", id)}, "json": g})
			}
			b, _ := json.Marshal(map[string]any{"games": games})
			return jsonResponse(string(b)), nil
		default:
			t.Fatal("unexpected audit destination (redacted)")
			return nil, nil
		}
	})}
	if detail.Partial {
		t.Fatal("invalid full-team fixture")
	}
	return s, requests
}

func TestPlayerAuditUsesBoundedRealPathsAndOnlyAllowListedReport(t *testing.T) {
	s, requests := auditFixtureSource(t, false)
	r := runPlayerAudit(context.Background(), s, "target", "all", func() error { return nil })
	if *requests != 6 {
		t.Fatal("unexpected remote count", *requests)
	}
	for _, step := range r.Steps {
		if step.Section == "rank_compare" {
			if step.State != "different" || len(step.Different) != 4 || step.Counts["same_known_season"] != 0 {
				t.Fatal("ranked source disagreement or unknown season was hidden", step)
			}
			continue
		}
		if step.State != "verified" {
			t.Fatalf("%s %s", step.Section, step.State)
		}
		if step.Section == "sample_analysis" && (step.Counts["unique_games"] != 42 || step.Counts["days30_boundary_reached"] != 0 || step.Counts["hours12_boundary_reached"] != 1 || step.Counts["queue420_month_wins"] != 42) {
			t.Fatal("coverage claimed a complete month or mixed queues", step.Counts)
		}
		if strings.HasPrefix(step.Section, "detail_") && step.Section != "detail_compare" && (step.Counts["players"] != 10 || step.Counts["targets"] != 1 || step.Counts["participation"] != 10 || step.Counts["seven_item_slots"] != 10 || step.Counts["vision"] != 10) {
			t.Fatal("detail coverage lost zero values", step.Counts)
		}
		if strings.HasPrefix(step.Section, "timeline_") && (step.Counts["full_team_gold_frames"] != 2 || step.Counts["SKILL_LEVEL_UP"] != 1) {
			t.Fatal("timeline coverage incorrect", step.Counts)
		}
	}
	raw, _ := json.Marshal(r)
	for _, forbidden := range []string{"private-", "PRIVATE", "NJ100", "puuid", "https:", "synthetic-ranked-token"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("report leaked a non-allow-listed field")
		}
	}
}

func TestPlayerAuditRepeatedPagesAndSourceDifferencesStayVisible(t *testing.T) {
	s, requests := auditFixtureSource(t, true)
	r := runPlayerAudit(context.Background(), s, "target", "history", func() error { return nil })
	if *requests != 2 {
		t.Fatal("repeated pages continued scanning", *requests)
	}
	for _, step := range r.Steps {
		if step.Section == "history_page_2" && (step.State != "partial" || step.Counts["duplicates"] != 20) {
			t.Fatal("duplicate evidence hidden")
		}
		if step.Section == "sample_analysis" && (step.State != "partial" || step.Counts["hours12_boundary_reached"] != 0) {
			t.Fatal("broken cursor claimed complete range")
		}
	}
	a := core.MatchDetail{Players: []core.MatchPlayer{{ParticipantID: 1, Summary: core.Match{Gold: intValue(0)}}}}
	b := core.MatchDetail{Players: []core.MatchPlayer{{ParticipantID: 1}}}
	if got := auditDetailDifferences(a, b); len(got) != 1 || got[0] != "gold" {
		t.Fatal("missing versus zero source difference hidden", got)
	}
}

func TestPlayerAuditStopsAfterAccountOrPhaseChangeAndHasNoAutoRetries(t *testing.T) {
	s, requests := auditFixtureSource(t, false)
	checks := 0
	r := runPlayerAudit(context.Background(), s, "target", "all", func() error {
		checks++
		if checks >= 2 {
			return fmt.Errorf("private-identity-error")
		}
		return nil
	})
	if *requests != 0 || r.Steps[0].State != "interrupted" {
		t.Fatal("audit continued after context changed")
	}
	for _, step := range r.Steps[1:] {
		if step.State != "not_run" {
			t.Fatal("later check executed")
		}
	}
	s, requests = auditFixtureSource(t, false)
	s.sgp.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		*requests++
		return nil, fmt.Errorf("private-network-response")
	})
	r = runPlayerAudit(context.Background(), s, "target", "rank", func() error { return nil })
	if *requests != 1 || r.Steps[1].State != "failed" {
		t.Fatal("failure retried or presented as verified")
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "private-") {
		t.Fatal("raw error in report")
	}
}

func intValue(n int) *int { return &n }
