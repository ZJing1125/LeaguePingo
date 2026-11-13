package core

import (
	"fmt"
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

// No names, PUUIDs, source IDs or arbitrary templates enter the message.
// Positions refer to this snapshot's current ally order, including the self row.
func AnalysisBriefLines(snapshot Snapshot, includeSelf bool) ([]string, bool, int) {
	mode, ok := ScoutModeFor(snapshot.QueueID)
	if !ok || !mode.Matches(snapshot.MapID, snapshot.GameMode) {
		return nil, true, 0
	}
	lines := []string{"LeaguePingo｜" + mode.Label + "｜近30天已读表现分析"}
	ready, rated, allies := true, 0, 0
	for _, p := range snapshot.Participants {
		if p.Team != "ally" {
			continue
		}
		allies++
		if allies > 5 {
			return nil, true, 0
		}
		if !includeSelf && snapshot.SelfKey != "" && (p.Key == snapshot.SelfKey || p.PUUID == snapshot.SelfKey) {
			continue
		}
		var queue *ScoutQueue
		if p.IdentityState == IdentityKnown && p.Scout != nil {
			for i := range p.Scout.Queues {
				if p.Scout.Queues[i].QueueID == snapshot.QueueID {
					queue = &p.Scout.Queues[i]
					break
				}
			}
		}
		prefix := fmt.Sprintf("%dL｜", allies)
		if queue == nil || queue.Analysis == nil || queue.Analysis.Score == nil {
			lines = append(lines, prefix+"资料不足，无法评级")
			if p.IdentityState == IdentityKnown && (p.Scout == nil || p.Scout.State == "loading") {
				ready = false
			}
			continue
		}
		a := queue.Analysis
		mark := ""
		if a.Provisional {
			mark = "暂定"
		}
		line := fmt.Sprintf("%s%s%s级 %.1f分｜有效%d场", prefix, mark, a.Grade, *a.Score, a.ScoredGames)
		line += "｜" + AnalysisReason(a)
		if !mode.Arena {
			if queue.Month.WinRate != nil {
				line += fmt.Sprintf("｜胜率%.0f%%", *queue.Month.WinRate)
			}
			if queue.Month.KDA != nil {
				line += fmt.Sprintf("｜KDA %.2f", *queue.Month.KDA)
			}
		} else if arena := queue.Month.Arena; arena != nil {
			if arena.AveragePlace != nil {
				line += fmt.Sprintf("｜均%.2f名", *arena.AveragePlace)
			}
			if arena.FirstPlaceRate != nil {
				line += fmt.Sprintf("｜夺冠%.0f%%", *arena.FirstPlaceRate)
			}
		}
		lines = append(lines, line)
		rated++
	}
	if allies < 5 {
		return nil, false, 0 // Partial rosters cannot establish all five seats.
	}
	if len(lines) == 1 {
		return nil, ready, 0
	}
	return lines, ready, rated
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
