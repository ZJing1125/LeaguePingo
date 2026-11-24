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
			tag := fmt.Sprintf("%s%d场%d胜", name, games, wins)
			if a.Version >= 3 && float64(wins)/float64(games) >= .62 {
				tag = fmt.Sprintf("🏆绝活%s(%d场%.0f%%胜)", name, games, float64(wins)*100/float64(games))
			}
			tags = append(tags, tag)
		}
		if games == 0 && len(a.Recent) == 15 {
			tags = append(tags, "近15场未见本英雄")
		}
		if p.Scout != nil {
			for _, champ := range p.Scout.Champions {
				if champ.ChampionID == p.ChampionID && champ.Mastery != nil {
					extra = append(extra, fmt.Sprintf("英雄成就%d点", champ.Mastery.Points))
				}
			}
		}
	}
	return tags, extra
}
