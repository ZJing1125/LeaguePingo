package core

import (
	"math"
	"sort"
	"strings"
)

var horseTeamLabels = [...]string{"千里马", "上等马", "中等马", "下等马", "纯牛马"}

func rateHorseTeam(players []Participant, queue int, team string, version int) {
	rows := []int{}
	teamSize, visible := 0, 0
	complete := true
	for i, p := range players {
		if p.Team != team {
			continue
		}
		teamSize++
		if p.IdentityState != IdentityKnown {
			continue
		}
		visible++
		q := participantScoutQueue(p, queue)
		if q == nil || q.Analysis == nil || q.Analysis.Version < version {
			complete = false
			continue
		}
		if ScoutFetchStatus(p.Scout, q) == StatusPending && q.Analysis.ScoredGames < 15 {
			complete = false
		}
		if q.Analysis.Score == nil {
			continue
		}
		a := q.Analysis
		if math.IsNaN(*a.Score) || math.IsInf(*a.Score, 0) || *a.Score < 0 || *a.Score > 100 {
			continue
		}
		r := &TeamRating{QueueID: queue, Label: "样本少", Tags: horsePlayerTags(p, queue, players)}
		players[i].TeamRating = r
		if a.State == "ready" && a.ScoredGames >= 5 {
			r.Label = "评分待齐"
			rows = append(rows, i)
		}
	}
	// Hidden identities take no rating slot. Every visible player's analysis
	// must finish before ranking the available valid scores.
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := players[rows[i]], players[rows[j]]
		a, b := participantScoutQueue(left, queue).Analysis, participantScoutQueue(right, queue).Analysis
		if *a.Score != *b.Score {
			return *a.Score > *b.Score
		}
		if a.ScoredGames != b.ScoredGames {
			return a.ScoredGames > b.ScoredGames
		}
		if leftCell, rightCell := horseSortCell(left), horseSortCell(right); leftCell != rightCell {
			return leftCell < rightCell
		}
		return left.Key < right.Key
	})
	for place, index := range rows {
		r := players[index].TeamRating
		r.Rated = len(rows)
		if teamSize == 5 && visible > 0 && complete && len(rows) == visible {
			r.Place = place + 1
			r.Label = horseTeamLabels[place]
			if version >= 4 {
				a := participantScoutQueue(players[index], queue).Analysis
				r.Label = HorseTier(*a.Score)
				if place > 0 && *a.Score == *participantScoutQueue(players[rows[place-1]], queue).Analysis.Score {
					r.Place = players[rows[place-1]].TeamRating.Place
				}
			}
		}
		r.Tags = mergeTacticalTags(horseTacticalTags(players[index], queue, players, r), r.Tags)
		if len(r.Tags) > 0 {
			r.Reason = r.Tags[0]
		}
	}
}

func mergeTacticalTags(tactical, extra []string) []string {
	text := strings.Join(tactical, "|")
	for _, tag := range extra {
		if len(tactical) > 0 && (tag == "表现较均衡" || tag == "基础摘要评级" || strings.Contains(text, tag)) {
			continue
		}
		tactical = append(tactical, tag)
	}
	return uniqueTags(tactical)
}

func horseSortCell(player Participant) int {
	if player.CellID != nil && *player.CellID >= 0 && *player.CellID < 10 {
		return *player.CellID % 5
	}
	return 5
}
