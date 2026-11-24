package core

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func rankedHorsePlayers() []Participant {
	out := []Participant{}
	for cell, score := range []float64{62, 85, 40, 76, 55} {
		a := BuildPlayerAnalysis(horseHistory(15, 420, score), 420, time.Now(), true)
		seat := cell
		out = append(out, Participant{Key: fmt.Sprint(cell), Team: "ally", CellID: &seat, IdentityState: IdentityKnown,
			Position: []string{"TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"}[cell],
			Scout:    &ScoutReport{State: "ready", Complete30: true, Queues: []ScoutQueue{{QueueID: 420, Analysis: a}}}})
	}
	return out
}

func TestFiveHorseLabelsFollowTeamScoresAndKeepPhysicalFloors(t *testing.T) {
	players := rankedHorsePlayers()
	ApplyTeamRatings(players, 420)
	for i, label := range []string{"中等马", "千里马", "纯牛马", "上等马", "下等马"} {
		if players[i].TeamRating.Label != label {
			t.Fatalf("floor %d got %s", i+1, players[i].TeamRating.Label)
		}
	}
	lines, _, rated := HorseBriefLines(Snapshot{QueueID: 420, Participants: players}, true)
	body := AnalysisBriefBody(lines)
	if rated != 5 || !strings.Contains(body, "1楼(上) 🐎中等马") || !strings.Contains(body, "2楼(野) 🦄千里马") || strings.Count(body, "\n") != 5 || !ValidAnalysisBriefBody(body) {
		t.Fatal("multiline format changed seats or lost grades", body)
	}
	players[0].Scout.Queues[0].FetchStatus = StatusPending
	players[0].Scout.Queues[0].Analysis.ScoredGames = 3
	ApplyTeamRatings(players, 420)
	for _, p := range players[1:] {
		if p.TeamRating.Label != "评分待齐" || p.TeamRating.Place != 0 {
			t.Fatal("incomplete team received final grades")
		}
	}
	players[0].IdentityState = IdentityUnavailable
	ApplyTeamRatings(players, 420)
	if players[0].TeamRating != nil {
		t.Fatal("hidden identity received grade")
	}
	for _, p := range players[1:] {
		if p.TeamRating == nil || p.TeamRating.Place == 0 {
			t.Fatal("hidden peer blocked visible team")
		}
	}

	players[0].IdentityState = IdentityUnavailable
	ApplyTeamRatings(players, 420)
	if players[0].TeamRating != nil {
		t.Fatal("hidden identity received a horse label")
	}
	for i, label := range map[int]string{1: "千里马", 2: "下等马", 3: "上等马", 4: "中等马"} {
		if players[i].TeamRating == nil || players[i].TeamRating.Label != label || players[i].TeamRating.Rated != 4 {
			t.Fatal("hidden identity occupied a ranking slot", i, players[i].TeamRating)
		}
	}

}

func TestHorseTacticalLabelsNeedPerformanceAndChampionEvidence(t *testing.T) {
	players := rankedHorsePlayers()
	for i := range players {
		a := players[i].Scout.Queues[0].Analysis
		for g := range a.Recent {
			dpm, eff, part := 500.0, 1.0, .7
			won := true
			if i == 1 {
				dpm, eff = 1000, 1.3
			}
			a.Recent[g].DamagePerMinute = &dpm
			a.Recent[g].EfficiencyRatio = &eff
			a.Recent[g].Participation = &part
			a.Recent[g].Win = &won
		}
	}
	players[1].ChampionID = 64
	ApplyTeamRatings(players, 420)
	tags := strings.Join(players[1].TeamRating.Tags, "|")
	for _, want := range []string{"院长级大腿", "靠谱火力点", "绝活李青", "连胜"} {
		if !strings.Contains(tags, want) {
			t.Fatal("missing observed dimension", want, tags)
		}
	}
	if strings.Count(tags, "连胜") != 1 {
		t.Fatal("repeated the same form evidence", tags)
	}
	lines, _, _ := HorseBriefLines(Snapshot{QueueID: 420, Participants: players}, true)
	body := AnalysisBriefBody(lines)
	if !strings.Contains(body, "院长级大腿") || !strings.Contains(body, "靠谱火力点") || !ValidAnalysisBriefBody(body) {
		t.Fatal("compressed brief lost tactical dimensions", body)
	}
	players[1].Scout.Queues[0].Analysis.Score = new(float64)
	ApplyTeamRatings(players, 420)
	if strings.Contains(strings.Join(players[1].TeamRating.Tags, "|"), "院长级大腿") {
		t.Fatal("relative place invented dominance")
	}
}
