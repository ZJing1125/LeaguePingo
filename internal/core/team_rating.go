package core

import (
	"math"
	"sort"
)

// TeamRating describes a player's standing among the rated members of this
// roster. The underlying analysis can be reused for a different roster.
type TeamRating struct {
	QueueID int    `json:"queueId"`
	Place   int    `json:"place"`
	Rated   int    `json:"rated"`
	Label   string `json:"label"`
	Reason  string `json:"reason"`
}

var teamRatingLabels = [...]string{"大腿", "主力", "中坚", "待起势", "需支援"}

func participantScoutQueue(p Participant, queue int) *ScoutQueue {
	if p.IdentityState != IdentityKnown || p.Scout == nil {
		return nil
	}
	for i := range p.Scout.Queues {
		if p.Scout.Queues[i].QueueID == queue {
			return &p.Scout.Queues[i]
		}
	}
	return nil
}

func ApplyTeamRatings(players []Participant, queue int) {
	for i := range players {
		players[i].TeamRating = nil
	}
	mode, ok := ScoutModeFor(queue)
	if !ok || mode.Arena {
		return
	}
	rateTeam(players, queue, "ally")
	rateTeam(players, queue, "enemy")
}

func rateTeam(players []Participant, queue int, team string) {
	type row struct {
		index    int
		score    float64
		analysis *PlayerAnalysis
	}
	var rows []row
	count := 0
	for i, player := range players {
		if player.Team != team {
			continue
		}
		count++
		if count > len(teamRatingLabels) {
			return
		}
		q := participantScoutQueue(player, queue)
		if q == nil || q.Analysis == nil || q.Analysis.Score == nil {
			continue
		}
		score := *q.Analysis.Score
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 10 {
			continue
		}
		rows = append(rows, row{i, score, q.Analysis})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	place := 0
	for i, row := range rows {
		if i == 0 || row.score < rows[i-1].score {
			place = i + 1
		}
		players[row.index].TeamRating = &TeamRating{QueueID: queue, Place: place, Rated: len(rows), Label: teamRatingLabels[place-1], Reason: AnalysisReason(row.analysis)}
	}
}
