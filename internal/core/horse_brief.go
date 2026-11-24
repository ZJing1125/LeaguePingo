package core

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func HorseBriefLines(snapshot Snapshot, includeSelf bool) ([]string, bool, int) {
	mode, ok := ScoutModeFor(snapshot.QueueID)
	if !ok || mode.Arena || !mode.Matches(snapshot.MapID, snapshot.GameMode) {
		return nil, true, 0
	}
	allies := []Participant{}
	for _, p := range snapshot.Participants {
		if p.Team == "ally" {
			allies = append(allies, p)
		}
	}
	if len(allies) > 5 {
		return nil, false, 0
	}
	// cellId is zero-based, within each team. Reordering the array cannot move a seat.
	sort.SliceStable(allies, func(i, j int) bool {
		if allies[i].CellID == nil {
			return false
		}
		return allies[j].CellID == nil || *allies[i].CellID < *allies[j].CellID
	})
	lines, compact := []string{}, []string{}
	ready, rated := len(allies) == 5, 0
	seen := map[int]bool{}
	for index, p := range allies {
		seat := index + 1
		if p.CellID != nil {
			if *p.CellID < 0 || *p.CellID > 9 {
				return nil, false, 0
			}
			seat = *p.CellID%5 + 1
		}
		if seat < 1 || seat > 5 || seen[seat] {
			return nil, false, 0
		}
		seen[seat] = true
		if !includeSelf && snapshot.SelfKey != "" && (p.Key == snapshot.SelfKey || p.PUUID == snapshot.SelfKey) {
			continue
		}
		prefix := fmt.Sprintf("%d楼", seat)
		if mode.Roles {
			prefix += "(" + briefPosition(p.Position) + ")"
		}
		q := participantScoutQueue(p, snapshot.QueueID)
		text := "资料待取"
		tag := ""
		if q != nil && q.Analysis != nil && q.Analysis.Version >= 2 && q.Analysis.ScoredGames > 0 {
			a := q.Analysis
			if a.State == "ready" && a.ScoredGames >= 5 && a.Score != nil && !math.IsNaN(*a.Score) && !math.IsInf(*a.Score, 0) && *a.Score >= 0 && *a.Score <= 100 {
				text = fmt.Sprintf("%s%d场", HorseTier(*a.Score), a.ScoredGames)
				rated++
			} else {
				text = fmt.Sprintf("样本少%d场", a.ScoredGames)
			}
			if p.TeamRating != nil && len(p.TeamRating.Tags) > 0 {
				tag = p.TeamRating.Tags[0]
			}
		} else if p.Scout != nil && p.Scout.Complete30 && p.Scout.State != "loading" {
			text = "无公开战绩"
			if q != nil && q.Month.Games > 0 {
				text = "字段不足"
			}
		}
		if p.IdentityState == IdentityKnown && (p.Scout == nil || p.Scout.State == "loading" || !p.Scout.Complete30 && p.Scout.State == "partial" && (q == nil || q.Analysis == nil || q.Analysis.ScoredGames < 15)) {
			ready = false
		}
		compact = append(compact, prefix+" "+text)
		if tag != "" && text != "资料待取" {
			text += "·" + tag
		}
		lines = append(lines, prefix+" "+text)
	}
	// Keep every seat. Remove optional tags as a group when UTF-8 exceeds budget.
	if len(strings.Join(lines, " | ")) > AnalysisBriefMaxBytes {
		lines = compact
	}
	return lines, ready, rated
}

func briefPosition(position string) string {
	value := map[string]string{"TOP": "上", "JUNGLE": "野", "MIDDLE": "中", "MID": "中", "BOTTOM": "下", "ADC": "下", "UTILITY": "辅", "SUPPORT": "辅"}[strings.ToUpper(position)]
	if value == "" {
		return "待定"
	}
	return value
}
