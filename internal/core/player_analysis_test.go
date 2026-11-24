package core

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func performanceGame() map[string]any {
	players := []any{}
	roles := []string{"TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"}
	for i := 0; i < 10; i++ {
		team, id := 100, fmt.Sprintf("synthetic-%d", i)
		if i >= 5 {
			team = 200
		}
		if i == 4 {
			id = "target"
		}
		stats := map[string]any{"kills": 10, "deaths": 2, "assists": 10, "goldEarned": 12000, "totalDamageDealtToChampions": 25000, "totalDamageTaken": 18000, "totalMinionsKilled": 150, "neutralMinionsKilled": 0, "visionScore": 8, "totalHeal": 1000, "totalDamageShieldedOnTeammates": 0, "win": i < 5}
		champion := 64
		if i%5 == 4 {
			champion = 16
			stats["kills"], stats["assists"], stats["goldEarned"], stats["totalDamageDealtToChampions"], stats["visionScore"], stats["totalHeal"], stats["totalDamageShieldedOnTeammates"] = 0, 20, 8000, 3000, 40, 8000, 1000
		}
		players = append(players, map[string]any{"participantId": i + 1, "puuid": id, "teamId": team, "championId": champion, "teamPosition": roles[i%5], "stats": stats})
	}
	return map[string]any{"gameId": 42, "queueId": 420, "mapId": 11, "gameCreation": time.Now().Add(-time.Hour).UnixMilli(), "gameDuration": 1800, "participants": players}
}

func parsePerformanceGame(t *testing.T, game map[string]any) Match {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"games": map[string]any{"games": []any{game}}})
	r, err := ParseHistory(raw, "target")
	if err != nil || len(r.Matches) != 1 {
		t.Fatalf("parse synthetic performance: %v", err)
	}
	return r.Matches[0]
}

func TestPerformanceComparesSupportsByLaneAndARAMByClass(t *testing.T) {
	game := performanceGame()
	m := parsePerformanceGame(t, game)
	if m.Performance == nil || m.Performance.Basis != "position" || math.Abs(m.Performance.Score-5) > 1e-9 {
		t.Fatalf("support not compared with support: %+v", m.Performance)
	}
	game["queueId"], game["mapId"] = 2400, 12
	m = parsePerformanceGame(t, game)
	if m.Performance == nil || m.Performance.Basis != "class" || math.Abs(m.Performance.Score-5) > 1e-9 {
		t.Fatalf("ARAM support not compared by class: %+v", m.Performance)
	}
	for _, metric := range m.Performance.Metrics {
		if metric.Key == "vision" || metric.Key == "cs" {
			t.Fatal("ARAM included Rift metrics")
		}
	}
}

func TestPerformanceMissingFieldsAndIncompleteRosterDoNotBecomeZero(t *testing.T) {
	game := performanceGame()
	players := game["participants"].([]any)
	delete(players[9].(map[string]any)["stats"].(map[string]any), "goldEarned")
	m := parsePerformanceGame(t, game)
	if m.Performance == nil || math.Abs(m.Performance.Score-5) > 1e-9 {
		t.Fatal("unknown comparison became zero or killed valid metrics")
	}
	for _, metric := range m.Performance.Metrics {
		if metric.Key == "gold" {
			t.Fatal("unknown peer gold became a measured comparison")
		}
	}
	game["participants"] = players[:9]
	m = parsePerformanceGame(t, game)
	if m.Performance != nil {
		t.Fatal("partial roster used as complete comparison")
	}
}

func TestAnalysisIndependentGradesBoundsSmallSamplesAndUnknown(t *testing.T) {
	now := time.Now().UTC()
	at := now.Add(-time.Hour)
	win, k, d, a := true, 6, 2, 8
	match := Match{GameID: 1, QueueID: 2400, PlayedAt: &at, Win: &win, Kills: &k, Deaths: &d, Assists: &a}
	one := BuildPlayerAnalysis([]Match{match}, 2400, now, false)
	if one.Score == nil || !one.Provisional || one.Grade == "S" || one.Basis != "basic" || one.ScoredGames != 1 {
		t.Fatalf("small sample mislabeled: %+v", one)
	}
	empty := BuildPlayerAnalysis([]Match{{GameID: 1, QueueID: 2400, PlayedAt: &at}}, 2400, now, true)
	if empty.Score != nil || empty.State != "insufficient" || empty.Grade != "" {
		t.Fatal("unknown became low grade")
	}
	if BuildPlayerAnalysis([]Match{match}, 99999, now, true) != nil {
		t.Fatal("unknown mode guessed")
	}
	full := []Match{}
	for i := 0; i < 30; i++ {
		m := match
		m.GameID = int64(i + 1)
		m.Performance = &MatchPerformance{Version: PerformanceVersion, Score: 9.8, Basis: "class", Horse:&HorsePerformance{Version:2,Score:98,Metrics:[]PerformanceMetric{{Key:"dpm",Score:98,Games:1},{Key:"kda",Score:98,Games:1},{Key:"participation",Score:98,Games:1}}}, Metrics: []PerformanceMetric{{Key: "damage", Score: 9.8, Games: 1}, {Key: "gold", Score: 9.8, Games: 1}, {Key: "survival", Score: 9.8, Games: 1}}}
		full = append(full, m)
	}
	first, second := BuildPlayerAnalysis(full, 2400, now, true), BuildPlayerAnalysis(full, 2400, now, true)
	if first.Grade != "千里马" || second.Grade != "千里马" || first.Provisional || *first.Score > 100 || first.Confidence != "high" {
		t.Fatalf("independent grade failed: %+v", first)
	}
	if analysisGrade(0) != "D" || analysisGrade(8.5) != "S" || analysisGrade(7) != "A" {
		t.Fatal("thresholds")
	}
}

func TestAnalysisExactQueueWindowDedupRemakeAndArena(t *testing.T) {
	now := time.Now().UTC()
	at, old := now.Add(-time.Hour), now.Add(-31*24*time.Hour)
	yes, duration, n := true, 90, 8
	valid := Match{GameID: 1, QueueID: 1700, PlayedAt: &at, Arena: &ArenaResult{Placement: 1, TeamCount: &n}, Win: &yes, EarlyEnd: &yes, Duration: &duration}
	other := valid
	other.GameID = 2
	other.QueueID = 1710
	stale := valid
	stale.GameID = 3
	stale.PlayedAt = &old
	unknown := valid
	unknown.GameID = 4
	unknown.Arena = &ArenaResult{Placement: 2}
	a := BuildPlayerAnalysis([]Match{valid, valid, other, stale, unknown}, 1700, now, true)
	if a.Games != 2 || a.ScoredGames != 1 || a.Basis != "placement" || *a.Score <= 5 {
		t.Fatalf("Arena/queue/window mix: %+v", a)
	}
	remake := valid
	remake.QueueID = 420
	remake.Arena = nil
	if BuildPlayerAnalysis([]Match{remake}, 420, now, true).ScoredGames != 0 {
		t.Fatal("remake rated")
	}
}

func TestBriefUsesActualModeAndSameGradeWithoutNames(t *testing.T) {
	s := Snapshot{QueueID: 2400, ScoutQueue: 420, MapID: 12, GameMode: "ARAM", SelfKey: "self"}
	for i := 0; i < 5; i++ {
		score := 5 + float64(i)/2
		p := Participant{Key: fmt.Sprint(i), PUUID: fmt.Sprint("private-", i), DisplayName: "PrivateName#DO-NOT-SEND", Team: "ally", IdentityState: IdentityKnown, Scout: &ScoutReport{State: "partial", Queues: []ScoutQueue{{QueueID: 2400, Analysis: &PlayerAnalysis{Grade: "A", Score: &score, ScoredGames: 3, Provisional: true}, Month: Stats{Games: 3}}, {QueueID: 420, Analysis: &PlayerAnalysis{Grade: "D", Score: &score}}}}}
		if i == 0 {
			p.Key = "self"
		}
		s.Participants = append(s.Participants, p)
	}
	ApplyTeamRatings(s.Participants, s.QueueID)
	lines, ready, rated := AnalysisBriefLines(s, true)
	body := AnalysisBriefBody(lines)
	if !ready || rated != 5 || len(lines) != 6 || !strings.HasPrefix(lines[1], "1楼 暂定(3场)") || strings.Contains(body, "D级") || strings.Contains(body, "PrivateName") || strings.Contains(body, "private-") {
		t.Fatal("brief differed from actual mode or leaked names")
	}
	lines, _, _ = AnalysisBriefLines(s, false)
	if len(lines) != 5 || strings.Contains(AnalysisBriefBody(lines), "1楼") {
		t.Fatal("self exclusion changed seat numbers")
	}
}

func TestBriefFitsSingleLineByteBudgetForAllModes(t *testing.T) {
	for _, mode := range ScoutModes() {
		if mode.Arena {
			continue
		}
		snapshot := Snapshot{QueueID: mode.QueueID}
		for i := 0; i < 5; i++ {
			winRate, kda := 99.99, 999.94
			queue := ScoutQueue{
				QueueID:  mode.QueueID,
				Analysis: &PlayerAnalysis{Provisional: true, ScoredGames: 10000},
				Month:    Stats{WinRate: &winRate, KDA: &kda},
			}
			snapshot.Participants = append(snapshot.Participants, Participant{
				Team: "ally", IdentityState: IdentityKnown,
				TeamRating: &TeamRating{Label: teamRatingLabels[i], Place: i + 1, Rated: 5},
				Scout:      &ScoutReport{Queues: []ScoutQueue{queue}},
			})
		}
		lines, ready, rated := AnalysisBriefLines(snapshot, true)
		body := AnalysisBriefBody(lines)
		if !ready || rated != 5 || strings.Count(body, "楼 ") != 5 ||
			strings.Count(body, "999+场") != 5 || strings.Count(body, "(暂)") != 5 || len(body) > AnalysisBriefMaxBytes || strings.ContainsAny(body, "\r\n") {
			t.Fatalf("queue %d produced %d bytes and %d ratings", mode.QueueID, len(body), rated)
		}
	}
}
