package core

import "fmt"

// Tactical labels require observable evidence beyond the relative horse place.
func horseTacticalTags(p Participant, queue int, peers []Participant, rating *TeamRating) []string {
	q := participantScoutQueue(p, queue)
	if q == nil || q.Analysis == nil {
		return nil
	}
	a := q.Analysis
	tags := []string{}
	deaths, dpm, eff := 0.0, 0.0, 0.0
	dpmGames, effGames := 0, 0
	for _, g := range a.Recent {
		deaths += float64(g.Deaths)
		if g.DamagePerMinute != nil {
			dpm += *g.DamagePerMinute
			dpmGames++
		}
		if g.EfficiencyRatio != nil {
			eff += *g.EfficiencyRatio
			effGames++
		}
	}
	if rating.Place == 1 && a.Score != nil && *a.Score >= 85 && dpmGames >= 5 && highestTeamDPM(dpm/float64(dpmGames), queue, peers, p.Team) {
		tags = append(tags, "👑院长级大腿")
	}
	if a.Score != nil && *a.Score >= 72 && effGames >= 5 && eff/float64(effGames) >= 1.2 {
		tags = append(tags, "⚔️靠谱火力点")
	}
	if a.Score != nil && *a.Score < 55 && len(a.Recent) >= 5 && deaths/float64(len(a.Recent)) >= 7 {
		tags = append(tags, fmt.Sprintf("⚠️场均死亡%.1f", deaths/float64(len(a.Recent))))
	}
	if len(a.Recent) < 5 {
		return tags
	}
	wins, losses, kda, kp, recentDeaths, kpGames := 0, 0, 0.0, 0.0, 0.0, 0
	for i, g := range a.Recent[:5] {
		kda += g.KDA / 5
		recentDeaths += float64(g.Deaths)
		if g.Participation != nil {
			kp += *g.Participation
			kpGames++
		}
		if i == wins && g.Win != nil && *g.Win {
			wins++
		}
		if i == losses && g.Win != nil && !*g.Win {
			losses++
		}
	}
	if wins >= 4 && kda > 4 {
		tags = append(tags, fmt.Sprintf("🔥%d连胜·KDA%.1f", wins, kda))
	}
	if losses >= 3 && recentDeaths/5 >= 7 {
		tags = append(tags, fmt.Sprintf("⚠️%d连败·场均死%.1f", losses, recentDeaths/5))
	}
	if losses >= 3 && kpGames == 5 && kp/5 > .65 {
		tags = append(tags, fmt.Sprintf("🛡️%d连败·参团%.0f%%", losses, kp/5*100))
	}
	return tags
}

func highestTeamDPM(value float64, queue int, peers []Participant, team string) bool {
	known := 0
	for _, peer := range peers {
		if peer.Team != team {
			continue
		}
		q := participantScoutQueue(peer, queue)
		if q == nil || q.Analysis == nil {
			continue
		}
		total, n := 0.0, 0
		for _, game := range q.Analysis.Recent {
			if game.DamagePerMinute != nil {
				total += *game.DamagePerMinute
				n++
			}
		}
		if n < 5 {
			continue
		}
		known++
		if total/float64(n) > value {
			return false
		}
	}
	return known == 5
}
