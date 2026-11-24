package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
)

// Names come from the project's packaged Riot catalog, never a player query.
//
//go:embed champion_names.json
var championNamesJSON []byte
var championNames = func() map[string]string {
	var names map[string]string
	if err := json.Unmarshal(championNamesJSON, &names); err != nil {
		panic(err)
	}
	return names
}()

func horsePlayerTags(p Participant, queue int, peers []Participant) []string {
	q := participantScoutQueue(p, queue)
	if q == nil || q.Analysis == nil {
		return nil
	}
	a := q.Analysis
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
			tags = append(tags, fmt.Sprintf("%s%d场%d胜", name, games, wins))
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
	tags = append(tags, a.Tags...)
	for _, metric := range a.Metrics {
		if metric.Key == "eff" && metric.Score >= 72 {
			tags = append(tags, "经济转化高")
		}
		if metric.Key == "eff" && metric.Score <= 48 {
			tags = append(tags, "经济转化低")
		}
	}
	if len(tags) == 0 {
		tags = append(tags, AnalysisReason(a))
	}
	tags = append(tags, extra...)
	for _, peer := range peers {
		if peer.Team != p.Team || peer.Key == p.Key {
			continue
		}
		if p.GroupToken != "" && p.GroupToken == peer.GroupToken {
			tag := "客户端同组"
			if peer.CellID != nil && *peer.CellID >= 0 && *peer.CellID < 10 {
				tag = fmt.Sprintf("客户端同组(%d楼)", *peer.CellID%5+1)
			}
			tags = append(tags, tag)
		}
		other := participantScoutQueue(peer, queue)
		if other == nil || other.Analysis == nil {
			continue
		}
		count := 0
		for _, left := range a.Recent {
			for _, right := range other.Analysis.Recent {
				if left.GameID == right.GameID && left.TeamID != 0 && left.TeamID == right.TeamID {
					count++
					break
				}
			}
		}
		if count >= 2 {
			tags = append(tags, fmt.Sprintf("近期同队%d局", count))
		}
	}
	return uniqueTags(tags)
}

func uniqueTags(tags []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		if tag != "" && !seen[tag] {
			result = append(result, tag)
			seen[tag] = true
		}
	}
	return result
}
