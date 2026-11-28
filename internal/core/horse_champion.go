package core

import (
	"fmt"
	"strconv"
)

func horseChampionTags(p Participant, a *PlayerAnalysis) ([]string, []string) {
	tags := []string{}
	extra := []string{}
	if p.ChampionID > 0 {
		games, wins, known := 0, 0, 0
		for _, m := range a.Recent {
			if m.ChampionID == p.ChampionID {
				games++
				if m.Win != nil {
					known++
					if *m.Win {
						wins++
					}
				}
			}
		}
		name := championNames[strconv.Itoa(p.ChampionID)]
		if name == "" {
			name = "本英雄"
		}
		if games >= 8 && known == games {
			winRate := float64(wins) / float64(games)
			tag := fmt.Sprintf("%s%d场%d胜", name, games, wins)
			if a.Version >= 3 && winRate >= .62 {
				tag = fmt.Sprintf("🏆绝活%s(%d场%.0f%%胜)", name, games, winRate*100)
			} else if a.Version >= 3 && games >= 10 && winRate <= .42 {
				tag = fmt.Sprintf("⚠️%s(%d场%.0f%%胜)", name, games, winRate*100)
			}
			tags = append(tags, tag)
		}
		var masteryPoints *int
		if p.Scout != nil {
			for _, champ := range p.Scout.Champions {
				if champ.ChampionID == p.ChampionID && champ.Mastery != nil {
					points := champ.Mastery.Points
					masteryPoints = &points
					break
				}
			}
		}
		if games == 0 && len(a.Recent) >= 5 && masteryPoints != nil && *masteryPoints < 5000 {
			tags = append(tags, fmt.Sprintf("⚠️%s0场·成就%d", name, *masteryPoints))
		} else {
			if games == 0 && len(a.Recent) == 15 {
				tags = append(tags, "近15场未见本英雄")
			}
			if masteryPoints != nil {
				extra = append(extra, fmt.Sprintf("英雄成就%d点", *masteryPoints))
			}
		}
	}
	return tags, extra
}
