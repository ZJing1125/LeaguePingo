package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Synthetic records exercise both documented wire formats, never player data.
func arenaGame(teams int, flat bool) map[string]any {
	players := []any{}
	for i := 0; i < teams*2; i++ {
		stats := map[string]any{"playerSubteamId": 10 + i/2, "subteamPlacement": i/2 + 1, "win": i/2 < teams/2, "kills": 6, "deaths": 2, "assists": 4}
		p := map[string]any{"participantId": i + 1, "puuid": fmt.Sprintf("synthetic-other-%d", i), "teamId": 100, "championId": 23}
		if i == 0 {
			p["puuid"] = "synthetic-target"
		}
		if flat {
			for k, v := range stats {
				p[k] = v
			}
		} else {
			p["stats"] = stats
		}
		players = append(players, p)
	}
	identities := []any{}
	for _, raw := range players {
		p := raw.(map[string]any)
		identities = append(identities, map[string]any{"participantId": p["participantId"], "player": map[string]any{"puuid": p["puuid"]}})
	}
	return map[string]any{"gameId": 42, "queueId": 1700, "mapId": 30, "gameCreation": time.Now().Add(-time.Hour).UnixMilli(), "gameDuration": 120, "platformId": "NJ100", "participants": players, "participantIdentities": identities}
}

func parseArenaGame(t *testing.T, game map[string]any, sgp bool) Match {
	t.Helper()
	var body any = map[string]any{"games": map[string]any{"games": []any{game}}}
	if sgp {
		ids := []any{}
		for _, raw := range game["participantIdentities"].([]any) {
			ids = append(ids, raw.(map[string]any)["player"].(map[string]any)["puuid"])
		}
		body = map[string]any{"games": []any{map[string]any{"metadata": map[string]any{"product": "lol", "info_type": "summary", "match_id": "NJ100_42", "participants": ids}, "json": game}}}
	}
	raw, _ := json.Marshal(body)
	var result HistoryResult
	var err error
	if sgp {
		result, err = ParseSGPHistory(raw, "synthetic-target", "NJ100")
	} else {
		result, err = ParseHistory(raw, "synthetic-target")
	}
	if err != nil || len(result.Matches) != 1 {
		t.Fatalf("synthetic parse: %v", err)
	}
	return result.Matches[0]
}

func TestArenaLCUAndSGPContracts(t *testing.T) {
	for _, flat := range []bool{false, true} {
		for _, teams := range []int{4, 8} {
			game := arenaGame(teams, flat)
			m := parseArenaGame(t, game, flat)
			if m.Arena == nil || m.Arena.Placement != 1 || m.Arena.TeamCount == nil || *m.Arena.TeamCount != teams || m.Participation != nil || len(m.Roster) != 0 {
				t.Fatalf("wrong Arena result: %+v", m.Arena)
			}
			game["queueId"] = 1710
			if parseArenaGame(t, game, flat).Arena == nil {
				t.Fatal("1710 contract lost")
			}
			raw, _ := json.Marshal(m)
			if strings.Contains(string(raw), "playerSubteamId") {
				t.Fatal("source group identifier retained")
			}
		}
	}
}

func TestArenaMissingConflictingAndPartialFields(t *testing.T) {
	tests := []struct {
		name         string
		edit         func(map[string]any, []any, map[string]any)
		place, teams bool
	}{
		{"absent", func(_ map[string]any, _ []any, s map[string]any) { delete(s, "subteamPlacement") }, false, false},
		{"zero", func(_ map[string]any, _ []any, s map[string]any) { s["subteamPlacement"] = 0 }, false, false},
		{"negative", func(_ map[string]any, _ []any, s map[string]any) { s["subteamPlacement"] = -1 }, false, false},
		{"out of range", func(_ map[string]any, _ []any, s map[string]any) { s["subteamPlacement"] = 33 }, false, false},
		{"generic placement is not alias", func(_ map[string]any, _ []any, s map[string]any) { delete(s, "subteamPlacement"); s["placement"] = 1 }, false, false},
		{"wrong queue", func(g map[string]any, _ []any, _ map[string]any) { g["queueId"] = 2400 }, false, false},
		{"wrong map", func(g map[string]any, _ []any, _ map[string]any) { g["mapId"] = 11 }, false, false},
		{"teammate conflict", func(_ map[string]any, p []any, _ map[string]any) {
			p[1].(map[string]any)["stats"].(map[string]any)["subteamPlacement"] = 2
		}, false, false},
		{"partial roster", func(g map[string]any, p []any, _ map[string]any) { g["participants"] = p[:7] }, true, false},
		{"partial looks like smaller format", func(g map[string]any, p []any, _ map[string]any) { g["participants"] = p[:4] }, true, false},
		{"missing independent roster", func(g map[string]any, _ []any, _ map[string]any) { delete(g, "participantIdentities") }, true, false},
		{"no subteam ID", func(_ map[string]any, _ []any, s map[string]any) { delete(s, "playerSubteamId") }, true, false},
		{"duplicate participant", func(_ map[string]any, p []any, _ map[string]any) { p[7].(map[string]any)["participantId"] = 7 }, true, false},
		{"conflicting final places", func(_ map[string]any, p []any, _ map[string]any) {
			for _, i := range []int{6, 7} {
				p[i].(map[string]any)["stats"].(map[string]any)["subteamPlacement"] = 3
			}
		}, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := arenaGame(4, false)
			p := g["participants"].([]any)
			tc.edit(g, p, p[0].(map[string]any)["stats"].(map[string]any))
			m := parseArenaGame(t, g, false)
			if (m.Arena != nil) != tc.place || m.Arena != nil && (m.Arena.TeamCount != nil) != tc.teams {
				t.Fatalf("unexpected derived result: %+v", m.Arena)
			}
		})
	}
}

func TestArenaStatsUseIndependentValidDenominators(t *testing.T) {
	yes, no, four, eight, short := true, false, 4, 8, 60
	matches := []Match{
		{QueueID: 1700, Win: &no, Arena: &ArenaResult{Placement: 1, TeamCount: &four}, EarlyEnd: &yes, Duration: &short},
		{QueueID: 1700, Win: &yes, Arena: &ArenaResult{Placement: 3, TeamCount: &four}},
		{QueueID: 1700, Win: &no, Arena: &ArenaResult{Placement: 4, TeamCount: &eight}},
		{QueueID: 1700, Win: &yes, Arena: &ArenaResult{Placement: 2}},
		{QueueID: 1700, Win: &yes},
		{QueueID: 420, Win: &yes},
	}
	s := CalculateStats(matches)
	a := s.Arena
	if s.Remakes != 0 || s.OutcomeGames != 1 || s.Wins != 1 || a == nil || a.PlacementGames != 4 || a.FirstPlaces != 1 || *a.FirstPlaceRate != 25 || *a.AveragePlace != 2.5 || a.TopHalfGames != 3 || a.TopHalf != 2 {
		t.Fatalf("incorrect denominators: %+v / %+v", s, a)
	}
	missing := CalculateStats([]Match{{QueueID: 1700, Win: &yes}})
	if missing.WinRate != nil || missing.Arena.FirstPlaceRate != nil || missing.Arena.TopHalfRate != nil {
		t.Fatal("legacy win or missing fields invented a result")
	}
}

func TestArenaSGPRosterMetadataIsRequiredForTeamDenominator(t *testing.T) {
	game := arenaGame(8, true)
	// Only four complete pairs remain: structurally this could be an older
	// smaller Arena. The independent metadata still lists all sixteen players.
	game["participants"] = game["participants"].([]any)[:8]
	m := parseArenaGame(t, game, true)
	if m.Arena == nil || m.Arena.TeamCount != nil {
		t.Fatal("partial SGP summary invented a smaller format")
	}
	raw, _ := json.Marshal(map[string]any{"games": []any{map[string]any{"metadata": map[string]any{"product": "lol", "info_type": "summary", "match_id": "NJ100_42"}, "json": game}}})
	result, err := ParseSGPHistory(raw, "synthetic-target", "NJ100")
	if err != nil || result.Matches[0].Arena == nil || result.Matches[0].Arena.TeamCount != nil {
		t.Fatal("missing metadata became a team denominator or lost verified placement")
	}
	for _, roster := range []any{"unrecognized-format", []any{1, 2}, nil} {
		raw, _ := json.Marshal(map[string]any{"games": []any{map[string]any{"metadata": map[string]any{"product": "lol", "info_type": "summary", "match_id": "NJ100_42", "participants": roster}, "json": game}}})
		result, err := ParseSGPHistory(raw, "synthetic-target", "NJ100")
		if err != nil || result.Matches[0].Arena == nil || result.Matches[0].Arena.TeamCount != nil {
			t.Fatal("optional roster failure discarded personal result or invented team count")
		}
	}
}

func TestArenaDetailKeepsIndividualResultsWithoutLegacyTeamShares(t *testing.T) {
	raw, _ := json.Marshal(arenaGame(8, false))
	d, err := ParseMatchDetail(raw, 42, "synthetic-target")
	if err != nil || len(d.Players) != 16 || !d.Partial {
		t.Fatalf("Arena detail parse: %v", err)
	}
	for _, p := range d.Players {
		if p.Summary.Arena == nil || p.DamageShare != nil || p.GoldShare != nil || p.Participation != nil {
			t.Fatal("lost Arena placement or applied legacy 5v5 denominator")
		}
	}
}

func TestArenaScoutWindowsAndQueuesStayIndependent(t *testing.T) {
	now := time.Now().UTC()
	recent, month, old := now.Add(-time.Hour), now.Add(-24*time.Hour), now.Add(-31*24*time.Hour)
	matches := []Match{
		{GameID: 1, QueueID: 1700, ChampionID: 23, PlayedAt: &recent, Arena: &ArenaResult{Placement: 1}},
		{GameID: 2, QueueID: 1700, ChampionID: 23, PlayedAt: &month, Arena: &ArenaResult{Placement: 3}},
		{GameID: 3, QueueID: 1710, ChampionID: 23, PlayedAt: &recent, Arena: &ArenaResult{Placement: 4}},
		{GameID: 4, QueueID: 1700, ChampionID: 23, PlayedAt: &old, Arena: &ArenaResult{Placement: 1}},
		{GameID: 1, QueueID: 1700, ChampionID: 23, PlayedAt: &recent, Arena: &ArenaResult{Placement: 1}},
	}
	r := BuildScoutReport(matches, nil, nil, now, false, true, true)
	if r.ModelVersion != ScoutStatsVersion || r.Season != nil {
		t.Fatal("unverified season or missing model version")
	}
	for _, q := range r.Queues {
		if q.QueueID == 1700 && (q.Month.Games != 2 || q.Twelve.Games != 1 || *q.Month.Arena.FirstPlaceRate != 50 || *q.Month.Arena.AveragePlace != 2) {
			t.Fatalf("window, dedupe or queue mix: %+v", q)
		}
		if q.QueueID == 1710 && (q.Month.Games != 1 || *q.Month.Arena.FirstPlaceRate != 0) {
			t.Fatal("1710 borrowed 1700")
		}
	}
	for _, q := range r.Champions[0].Queues {
		if q.QueueID == 1700 && q.Stats.Arena.PlacementGames != 2 {
			t.Fatal("hero statistics used wrong window")
		}
	}
}

func BenchmarkScoutTenPlayersMixedModes(b *testing.B) {
	now := time.Now().UTC()
	matches := make([]Match, 1000)
	modes := ScoutModes()
	for i := range matches {
		at := now.Add(-time.Duration(i) * time.Hour)
		mode := modes[i%len(modes)]
		matches[i] = Match{GameID: int64(i + 1), QueueID: mode.QueueID, ChampionID: 1 + i%150, PlayedAt: &at, MapID: &mode.MapID}
		if mode.Arena {
			matches[i].Arena = &ArenaResult{Placement: 2}
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for player := 0; player < 10; player++ {
			BuildScoutReport(matches, nil, nil, now, false, false, false)
		}
	}
}

func TestArenaPartialRefreshImprovesFieldsWithoutLosingQueueCoverage(t *testing.T) {
	now := time.Now().UTC()
	at := now.Add(-time.Hour)
	game := Match{GameID: 1, QueueID: 1700, ChampionID: 23, PlayedAt: &at}
	old := BuildScoutReport([]Match{game}, nil, nil, now.Add(-time.Minute), false, true, true)
	old.Read = 200
	game.Arena = &ArenaResult{Placement: 2}
	next := BuildScoutReport([]Match{game}, nil, nil, now, false, false, false)
	next.State = "partial"
	merged := MergeScoutReports(&old, next)
	for _, q := range merged.Queues {
		if q.QueueID == 1700 && (q.Month.Arena == nil || q.Month.Arena.PlacementGames != 1 || q.Coverage.Complete30 || !q.Coverage.FetchedAt.Equal(now)) {
			t.Fatal("lost new Arena fields or borrowed old full coverage")
		}
	}
	for _, q := range old.Queues {
		if q.QueueID == 1700 && q.Month.Arena.PlacementGames != 0 {
			t.Fatal("mutated published report")
		}
	}
}
