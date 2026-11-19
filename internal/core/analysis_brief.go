package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type AnalysisBriefState struct {
	Enabled     bool      `json:"enabled"`
	Audience    string    `json:"audience"`
	IncludeSelf bool      `json:"includeSelf"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Chat identifies players by their selection-list order and never includes account identities.
func AnalysisBriefLines(snapshot Snapshot, includeSelf bool) ([]string, bool, int) {
	mode, ok := ScoutModeFor(snapshot.QueueID)
	if !ok || mode.Arena || !mode.Matches(snapshot.MapID, snapshot.GameMode) {
		return nil, true, 0
	}
	type row struct {
		seat   int
		player Participant
	}
	var rows []row
	allies := 0
	for _, p := range snapshot.Participants {
		if p.Team != "ally" {
			continue
		}
		allies++
		if !includeSelf && snapshot.SelfKey != "" && (p.Key == snapshot.SelfKey || p.PUUID == snapshot.SelfKey) {
			continue
		}
		rows = append(rows, row{allies, p})
	}
	if allies > len(teamRatingLabels) || len(rows) == 0 {
		return nil, false, 0
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].player.TeamRating, rows[j].player.TeamRating
		return a != nil && (b == nil || a.Place < b.Place)
	})
	lines := []string{mode.Label + "｜我方近30天表现"}
	ready, rated := true, 0
	for _, r := range rows {
		prefix := fmt.Sprintf("%d楼 ", r.seat)
		q := participantScoutQueue(r.player, snapshot.QueueID)
		if r.player.TeamRating == nil || q == nil || q.Analysis == nil {
			lines = append(lines, prefix+"资料不足")
			if r.player.IdentityState == IdentityKnown && (r.player.Scout == nil || r.player.Scout.State == "loading") {
				ready = false
			}
			continue
		}
		lines = append(lines, prefix+analysisBriefRating(r.player.TeamRating, q))
		rated++
	}
	return lines, ready, rated
}

func analysisBriefRating(rating *TeamRating, q *ScoutQueue) string {
	label := rating.Label
	if q.Analysis.Provisional {
		label += "·暂定"
	}
	line := fmt.Sprintf("【%s %d/%d】%d场", label, rating.Place, rating.Rated, q.Analysis.ScoredGames)
	if q.Month.WinRate != nil {
		line += fmt.Sprintf(" · 胜率%.0f%%", *q.Month.WinRate)
	}
	if q.Month.KDA != nil {
		line += fmt.Sprintf(" · KDA %.1f", *q.Month.KDA)
	}
	if q.Analysis.Basis != "basic" {
		line += " · " + rating.Reason
	}
	return line
}

func AnalysisBriefBody(lines []string) string { return strings.Join(lines, "\n") }

func AnalysisReason(a *PlayerAnalysis) string {
	if a == nil || a.Score == nil {
		return "资料不足"
	}
	if a.Basis == "basic" {
		return "基础摘要评级"
	}
	labels := map[string]string{"damage": "输出", "gold": "经济", "participation": "参团", "survival": "生存", "cs": "补刀", "vision": "视野", "taken": "承伤", "healing": "治疗", "shielding": "护盾", "placement": "名次"}
	var high, low *PerformanceMetric
	for i := range a.Metrics {
		m := &a.Metrics[i]
		if m.Games <= 0 || labels[m.Key] == "" {
			continue
		}
		if high == nil || m.Score > high.Score {
			high = m
		}
		if low == nil || m.Score < low.Score {
			low = m
		}
	}
	if high != nil && high.Score >= 6.5 {
		return labels[high.Key] + "较好"
	}
	if low != nil && low.Score <= 3.5 {
		return labels[low.Key] + "偏弱"
	}
	return "表现较均衡"
}
