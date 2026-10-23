package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func ParseMatchDetail(raw []byte, gameID int64, target string) (MatchDetail, error) {
	var game historyGame
	if json.Unmarshal(raw, &game) != nil || gameID <= 0 || game.GameID != gameID || len(game.Participants) == 0 || len(game.Participants) > 64 || !IsKnownPUUID(target) {
		return MatchDetail{}, errors.New("对局详情格式或比赛编号不匹配")
	}
	now := time.Now()
	out := MatchDetail{GameID: gameID, FetchedAt: now, Players: []MatchPlayer{}, MapID: nonNegative(game.MapID), Teams: []MatchTeam{}}
	seen := map[int]bool{}
	for _, item := range game.Participants {
		var p historyParticipant
		if json.Unmarshal(item, &p) != nil || p.ParticipantID <= 0 || seen[p.ParticipantID] {
			return MatchDetail{}, errors.New("对局参与者编号缺失或重复")
		}
		seen[p.ParticipantID] = true
		name := fmt.Sprintf("玩家 %d（身份未提供）", p.ParticipantID)
		if value := historyName(game, p); value != "" {
			name = value
		}
		verified := p.PUUID == target
		for _, identity := range game.ParticipantIdentities {
			if identity.ParticipantID != p.ParticipantID {
				continue
			}
			if IsKnownPUUID(p.PUUID) && p.PUUID != identity.Player.PUUID {
				continue
			}
			if identity.Player.PUUID == target {
				verified = true
			}
			if identity.Player.GameName != "" && identity.Player.TagLine != "" {
				name = identity.Player.GameName + "#" + identity.Player.TagLine
			} else if identity.Player.SummonerName != "" {
				name = identity.Player.SummonerName
			}
		}
		out.Players = append(out.Players, MatchPlayer{ParticipantID: p.ParticipantID, DisplayName: name, TeamID: p.TeamID, Target: verified, Summary: matchSummary(game, p, now)})
	}
	targets := 0
	counts := map[int]int{}
	for _, p := range out.Players {
		if p.Target {
			targets++
		}
		counts[p.TeamID]++
	}
	if targets != 1 {
		return MatchDetail{}, errors.New("对局详情不能唯一核验目标玩家")
	}
	out.Partial = len(out.Players) != 10 || counts[100] != 5 || counts[200] != 5
	teamSeen := map[int]bool{}
	for _, team := range game.Teams {
		if teamSeen[team.TeamID] || (team.TeamID != 100 && team.TeamID != 200) {
			continue
		}
		teamSeen[team.TeamID] = true
		value := func(local *int, key string) *int {
			if local != nil {
				return nonNegative(local)
			}
			return nonNegative(team.Objectives[key].Kills)
		}
		out.Teams = append(out.Teams, MatchTeam{TeamID: team.TeamID, Towers: value(team.Towers, "tower"), Dragons: value(team.Dragons, "dragon"), Barons: value(team.Barons, "baron"), Inhibitors: value(team.Inhibitors, "inhibitor"), Heralds: value(team.Heralds, "riftHerald"), Hordes: value(team.Hordes, "horde")})
	}
	if !out.Partial {
		for i := range out.Players {
			p := &out.Players[i]
			p.DamageShare = teamShare(out.Players, p.TeamID, p.Summary.Damage, func(m Match) *int { return m.Damage })
			p.GoldShare = teamShare(out.Players, p.TeamID, p.Summary.Gold, func(m Match) *int { return m.Gold })
			if p.Summary.Kills != nil && p.Summary.Assists != nil {
				ka := *p.Summary.Kills + *p.Summary.Assists
				p.Participation = teamShare(out.Players, p.TeamID, &ka, func(m Match) *int { return m.Kills })
				p.Summary.Participation = p.Participation
			}
		}
	}
	return out, nil
}

func teamShare(players []MatchPlayer, team int, value *int, metric func(Match) *int) *float64 {
	if value == nil {
		return nil
	}
	total := 0
	for _, p := range players {
		if p.TeamID != team {
			continue
		}
		v := metric(p.Summary)
		if v == nil {
			return nil
		}
		total += *v
	}
	if total <= 0 || *value > total {
		return nil
	}
	share := float64(*value) * 100 / float64(total)
	return &share
}
