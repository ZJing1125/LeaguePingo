package core

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func horseHistory(n int, queue int, score float64) []Match {
	now := time.Now().UTC()
	k, d, a, seconds := 6, 2, 8, 1800
	result := []Match{}
	for i := 0; i < n; i++ {
		at := now.Add(-time.Duration(i+1) * time.Hour)
		won := i%3 != 0
		result = append(result, Match{GameID: int64(i + 1), QueueID: queue, ChampionID: 64, PlayedAt: &at, Kills: &k, Deaths: &d, Assists: &a, Duration: &seconds, Win: &won,
			Performance: &MatchPerformance{Horse: &HorsePerformance{Version: 3, Score: score, Metrics: []PerformanceMetric{{Key: "kda", Score: score, Games: 1}, {Key: "dpm", Score: score, Games: 1}, {Key: "participation", Score: score, Games: 1}}}},
			Roster:      []MatchPeer{{Target: true, TeamID: 100}},
		})
	}
	return result
}

func TestHorseScoreBoundsSampleWindowAndExactQueue(t *testing.T) {
	for _, score := range []float64{0, 39.9, 40, 54.9, 55, 60, 71.9, 72, 84.9, 85, 100} {
		a := BuildPlayerAnalysis(horseHistory(25, 2400, score), 2400, time.Now(), true)
		// The per-game score is constant, so the weighted mean equals it;
		// the B-factor rescale then maps it around the 60 baseline.
		want := math.Round(clampScore(60+(score-60)*horseRules.ScoreScale, 0, 100)*10) / 10
		if a.Version != 4 || a.ScoredGames != 15 || a.Grade != HorseTier(want) || math.Abs(*a.Score-want) > .01 {
			t.Fatalf("score %v: %+v", score, a)
		}
	}
	a := BuildPlayerAnalysis(horseHistory(4, 2400, 90), 2400, time.Now(), true)
	if a.Grade != "" || a.State != "sample" {
		t.Fatal("small sample got formal grade")
	}
	sample := append(horseHistory(5, 2400, 60), horseHistory(15, 450, 90)...)
	sample = append(sample, sample[0])
	a = BuildPlayerAnalysis(sample, 2400, time.Now(), true)
	if a.ScoredGames != 5 || a.Grade != "中等马" {
		t.Fatal("duplicate or foreign queue changed tier")
	}
}

func TestHorseRiftAndBrawlNormalizeSamePositionOrClassToSixty(t *testing.T) {
	game := performanceGame()
	for _, mode := range []struct{ q, m int }{{420, 11}, {2400, 12}, {450, 12}} {
		game["queueId"], game["mapId"] = mode.q, mode.m
		m := parsePerformanceGame(t, game)
		if m.Performance == nil || m.Performance.Horse == nil || math.Abs(m.Performance.Horse.Score-60) > .001 {
			t.Fatal("baseline drift", m.Performance)
		}
		if mode.q != 420 {
			for _, metric := range m.Performance.Horse.Metrics {
				if metric.Key != "dpm" && metric.Key != "kda" && metric.Key != "participation" {
					t.Fatal("brawl included lane metrics")
				}
			}
		}
	}
}

func TestHorseRecentDropRequiresBothRelativeAndAbsoluteDeathIncrease(t *testing.T) {
	games := make([]AnalysisGame, 15)
	for i := range games {
		rate := 1.0
		kda := 5.0
		if i < 5 {
			rate = 1.6
			kda = 2
		}
		games[i] = AnalysisGame{GameID: int64(i + 1), KDA: kda, DeathsPerTen: &rate}
	}
	tags := strings.Join(horseFormTags(games), "|")
	if !strings.Contains(tags, "近期死亡上升") {
		t.Fatal("objective decline missing", tags)
	}
	for i := 0; i < 5; i++ {
		rate := 1.4
		games[i].DeathsPerTen = &rate
	}
	if strings.Contains(strings.Join(horseFormTags(games), "|"), "死亡上升") {
		t.Fatal("tiny absolute change raised warning")
	}
	if strings.Contains(strings.Join(horseFormTags(games[:14]), "|"), "死亡上升") {
		t.Fatal("incomplete prior window invented trend")
	}
}

func TestHorseBriefMixedStatesAndSeatGrades(t *testing.T) {
	snapshot := Snapshot{QueueID: 420, MapID: 11, GameMode: "CLASSIC"}
	for i := 0; i < 5; i++ {
		cell := 4 - i
		a := BuildPlayerAnalysis(horseHistory(15, 420, 60), 420, time.Now(), true)
		p := Participant{Key: fmt.Sprint(i), CellID: &cell, Team: "ally", IdentityState: IdentityKnown, Position: "MIDDLE", Scout: &ScoutReport{State: "ready", Complete30: true, Queues: []ScoutQueue{{QueueID: 420, Analysis: a}}}}
		snapshot.Participants = append(snapshot.Participants, p)
	}
	ApplyTeamRatings(snapshot.Participants, 420)
	for _, p := range snapshot.Participants {
		if p.TeamRating.Label != "中等马" {
			t.Fatal("team grade did not follow stable seat tie-break")
		}
	}
	snapshot.Participants[0].Scout = nil
	snapshot.Participants[1].Scout = &ScoutReport{State: "ready", Complete30: true}
	snapshot.Participants[2].Scout.Queues[0].Analysis = BuildPlayerAnalysis(horseHistory(3, 420, 99), 420, time.Now(), true)
	ApplyTeamRatings(snapshot.Participants, 420)
	lines, _, rated := HorseBriefLines(snapshot, true)
	body := AnalysisBriefBody(lines)
	if rated != 2 || !strings.HasPrefix(body, HorseBriefHeader) || !strings.Contains(body, "3楼(中) 样本少（3场）") || !strings.Contains(body, "4楼(中) 无本模式战绩") || !strings.Contains(body, "5楼(中) 资料待取") || !ValidAnalysisBriefBody(body) {
		t.Fatal("mixed state or stable seat missing", body, rated)
	}
}

func TestHorseHistoricalCooccurrenceIsSameTeamEvidence(t *testing.T) {
	a := BuildPlayerAnalysis(horseHistory(15, 2400, 60), 2400, time.Now(), true)
	b := BuildPlayerAnalysis(horseHistory(15, 2400, 60), 2400, time.Now(), true)
	p := Participant{Key: "a", Team: "ally", IdentityState: IdentityKnown, Scout: &ScoutReport{Queues: []ScoutQueue{{QueueID: 2400, Analysis: a}}}}
	peer := p
	peer.Key = "b"
	peer.Scout = &ScoutReport{Queues: []ScoutQueue{{QueueID: 2400, Analysis: b}}}
	tags := strings.Join(horsePlayerTags(p, 2400, []Participant{p, peer}), "|")
	if !strings.Contains(tags, "近期同队15局") || strings.Contains(tags, "双排") {
		t.Fatal("cooccurrence overclaimed", tags)
	}
	for i := range b.Recent {
		b.Recent[i].TeamID = 200
	}
	if strings.Contains(strings.Join(horsePlayerTags(p, 2400, []Participant{p, peer}), "|"), "同队") {
		t.Fatal("opposing teams counted together")
	}
}

func TestHorseEconomicEfficiencyThresholdUsesSameLaneCohort(t *testing.T) {
	for _, sample := range []struct {
		damage int
		ratio  float64
	}{{30000, 1.2}, {20000, .8}} {
		game := performanceGame()
		players := game["participants"].([]any)
		players[4].(map[string]any)["puuid"] = "other"
		target := players[2].(map[string]any)
		target["puuid"] = "target"
		target["stats"].(map[string]any)["totalDamageDealtToChampions"] = sample.damage
		m := parsePerformanceGame(t, game)
		if m.Performance == nil || m.Performance.Horse == nil || m.Performance.Horse.EfficiencyRatio == nil || math.Abs(*m.Performance.Horse.EfficiencyRatio-sample.ratio) > .001 {
			t.Fatal("economic ratio did not use same lane", m.Performance)
		}
	}
}

func TestHorseDirectGroupTagsRequireSameSideAndNonemptyToken(t *testing.T) {
	a := BuildPlayerAnalysis(horseHistory(15, 2400, 60), 2400, time.Now(), true)
	cell := 1
	p := Participant{Key: "self", Team: "ally", GroupToken: "memory-only", IdentityState: IdentityKnown, Scout: &ScoutReport{Queues: []ScoutQueue{{QueueID: 2400, Analysis: a}}}}
	peer := p
	peer.Key = "peer"
	peer.CellID = &cell
	tags := strings.Join(horsePlayerTags(p, 2400, []Participant{p, peer}), "|")
	if !strings.Contains(tags, "客户端同组(2楼)") || strings.Contains(tags, "memory-only") {
		t.Fatal("direct group metadata leaked or missing", tags)
	}
	peer.Team = "enemy"
	if strings.Contains(strings.Join(horsePlayerTags(p, 2400, []Participant{p, peer}), "|"), "客户端同组") {
		t.Fatal("group crossed sides")
	}
}
