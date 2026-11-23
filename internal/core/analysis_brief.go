package core

import (
	"fmt"
	"strings"
	"time"
)

// The helper uses a smaller UTF-8 budget than the old multiline brief.
const AnalysisBriefMaxBytes = 256

type AnalysisBriefState struct {
	Enabled     bool      `json:"enabled"`
	Audience    string    `json:"audience"`
	IncludeSelf bool      `json:"includeSelf"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Chat follows the selection roster order, with the same sample as its rating.
func AnalysisBriefLines(snapshot Snapshot, includeSelf bool) ([]string, bool, int) {
	mode, ok := ScoutModeFor(snapshot.QueueID)
	if !ok || mode.Arena || !mode.Matches(snapshot.MapID, snapshot.GameMode) {
		return nil, true, 0
	}
	lines, compact := []string{mode.Label + "·近30天"}, []string{mode.Label + "·近30天"}
	ready, rated, seat := true, 0, 0
	for _, p := range snapshot.Participants {
		if p.Team != "ally" {
			continue
		}
		seat++
		if seat > len(teamRatingLabels) {
			return nil, false, 0
		}
		if !includeSelf && snapshot.SelfKey != "" && (p.Key == snapshot.SelfKey || p.PUUID == snapshot.SelfKey) {
			continue
		}
		prefix := fmt.Sprintf("%d楼 ", seat)
		q := participantScoutQueue(p, snapshot.QueueID)
		text, short := "资料不足", "资料不足"
		if p.TeamRating != nil && q != nil && q.Analysis != nil && q.Analysis.ScoredGames > 0 {
			text, short = analysisBriefRating(p.TeamRating, q)
			rated++
		} else if p.IdentityState == IdentityKnown && (p.Scout == nil || p.Scout.State == "loading") {
			ready = false
		}
		lines, compact = append(lines, prefix+text), append(compact, prefix+short)
	}
	if len(lines) == 1 {
		return nil, false, 0
	}
	if len(AnalysisBriefBody(lines)) > AnalysisBriefMaxBytes {
		lines = compact
	}
	return lines, ready, rated
}

func analysisBriefRating(rating *TeamRating, q *ScoutQueue) (string, string) {
	a := q.Analysis
	label := rating.Label
	if rating.Rated < 2 {
		label = "个人参考"
	}
	if a.ScoredGames < 5 {
		label = "暂定"
	} else if a.Provisional {
		label += "(暂)"
	}
	games := fmt.Sprint(a.ScoredGames)
	if a.ScoredGames > 999 {
		games = "999+"
	}
	short := label + "(" + games + "场)"
	reason := AnalysisReason(a)
	if a.Basis == "basic" || label == "暂定" {
		return short, short
	}
	return label + "·" + reason + "(" + games + "场)", short
}

func AnalysisBriefBody(lines []string) string { return strings.Join(lines, " | ") }

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
