package core

import (
	"fmt"
	"math"
	"strings"
)

func HorseBriefLines(snapshot Snapshot, includeSelf bool) ([]string, bool, int) {
	mode, ok := ScoutModeFor(snapshot.QueueID)
	if !ok || mode.Arena || !mode.Matches(snapshot.MapID, snapshot.GameMode) {
		return nil, false, 0
	}
	seats, valid := horseBriefSeats(snapshot.Participants)
	if !valid {
		return nil, false, 0
	}
	lines, tags := []string{}, [][]string{}
	ready, rated, known := true, 0, 0
	for index, player := range seats {
		p := Participant{}
		if player != nil {
			p = *player
		} else {
			ready = false
		}
		prefix := fmt.Sprintf("%d楼", index+1)
		if mode.Roles {
			prefix += "(" + briefPosition(p.Position) + ")"
		}
		q := participantScoutQueue(p, snapshot.QueueID)
		status := ScoutFetchStatus(p.Scout, q)
		if p.IdentityState == IdentityKnown {
			known++
			if status == StatusPending && (q == nil || q.Analysis == nil || q.Analysis.Version < 3 || q.Analysis.ScoredGames < 15) {
				ready = false
			}
		}
		if !includeSelf && snapshot.SelfKey != "" && (p.Key == snapshot.SelfKey || p.PUUID == snapshot.SelfKey) {
			continue
		}
		text, isRated := horseBriefText(p.Scout, q, status, p.TeamRating)
		if player != nil && p.IdentityState != IdentityKnown {
			text = "身份暂不可用"
		}
		if isRated {
			rated++
		}
		lines = append(lines, prefix+" "+text)
		rowTags := []string{}
		if q != nil && q.Analysis != nil && q.Analysis.ScoredGames > 0 && p.TeamRating != nil {
			rowTags = p.TeamRating.Tags
		}
		tags = append(tags, rowTags)
	}
	lines = formatHorseBrief(lines, tags)
	return lines, ready && known > 0, rated
}

func horseBriefSeats(players []Participant) ([5]*Participant, bool) {
	var seats [5]*Participant
	allies := []Participant{}
	for _, p := range players {
		if p.Team == "ally" {
			allies = append(allies, p)
		}
	}
	if len(allies) > len(seats) {
		return seats, false
	}
	for i := range allies {
		p := &allies[i]
		if p.CellID == nil {
			continue
		}
		if *p.CellID < 0 || *p.CellID > 9 || seats[*p.CellID%5] != nil {
			return seats, false
		}
		seats[*p.CellID%5] = p
	}
	// Legacy sources with no cellId retain roster order in the available seats.
	for i := range allies {
		if allies[i].CellID != nil {
			continue
		}
		for index := range seats {
			if seats[index] == nil {
				seats[index] = &allies[i]
				break
			}
		}
	}
	return seats, true
}

func horseBriefText(report *ScoutReport, q *ScoutQueue, status FetchStatus, rating *TeamRating) (string, bool) {
	if q != nil && q.Analysis != nil && q.Analysis.Version >= 3 && q.Analysis.ScoredGames > 0 {
		a := q.Analysis
		if a.State == "ready" && a.ScoredGames >= 5 && a.Score != nil && !math.IsNaN(*a.Score) && !math.IsInf(*a.Score, 0) && *a.Score >= 0 && *a.Score <= 100 {
			label := "评分待齐"
			if rating != nil {
				label = rating.Label
			}
			if label == "评分待齐" {
				return fmt.Sprintf("评分待齐（%.1f分/%d场）", *a.Score, a.ScoredGames), true
			}
			return fmt.Sprintf("%s（%d场）", horseBriefLabel(label), a.ScoredGames), true
		}
		return fmt.Sprintf("样本少（%d场）", a.ScoredGames), false
	}
	switch status {
	case StatusNoMatchInQueue:
		return "无本模式战绩", false
	case StatusTimeoutOrError:
		if report != nil && report.Reason == "RequestTimeout" {
			return "网络超时/资料待取", false
		}
		return "资料暂不可用", false
	case StatusSuccess:
		return "字段不足", false
	default:
		return "资料待取", false
	}
}

func briefPosition(position string) string {
	value := map[string]string{"TOP": "上", "JUNGLE": "野", "MIDDLE": "中", "MID": "中", "BOTTOM": "下", "ADC": "下", "UTILITY": "辅", "SUPPORT": "辅"}[strings.ToUpper(position)]
	if value == "" {
		return "待定"
	}
	return value
}
