package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rosterMember struct {
	PUUID        string `json:"puuid"`
	ChampionID   int    `json:"championId"`
	SummonerName string `json:"summonerName"`
}

type gameflowSession struct {
	GameData struct {
		GameID  int64          `json:"gameId"`
		TeamOne []rosterMember `json:"teamOne"`
		TeamTwo []rosterMember `json:"teamTwo"`
	} `json:"gameData"`
}

type champSelectSession struct {
	MyTeam    []rosterMember `json:"myTeam"`
	TheirTeam []rosterMember `json:"theirTeam"`
}

func IsKnownPUUID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "0" && value != "00000000-0000-0000-0000-000000000000"
}

func ParseGameflowRoster(data []byte) (int64, []Participant, error) {
	return ParseGameflowRosterForPlayer(data, "")
}

// teamOne/teamTwo are map sides, not necessarily the local player's team.
func ParseGameflowRosterForPlayer(data []byte, selfPUUID string) (int64, []Participant, error) {
	var session gameflowSession
	if err := json.Unmarshal(data, &session); err != nil {
		return 0, nil, err
	}
	allies, enemies := session.GameData.TeamOne, session.GameData.TeamTwo
	if selfPUUID != "" {
		found := false
		for _, p := range allies {
			if p.PUUID == selfPUUID {
				found = true
			}
		}
		if !found {
			for _, p := range enemies {
				if p.PUUID == selfPUUID {
					found = true
					allies, enemies = enemies, allies
					break
				}
			}
		}
		if !found {
			return session.GameData.GameID, nil, fmt.Errorf("当前账号不在本局名单中，无法确认双方阵营")
		}
	}
	return session.GameData.GameID, mergeRoster(allies, enemies), nil
}

func ParseChampSelectRoster(data []byte) ([]Participant, error) {
	var session champSelectSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return mergeRoster(session.MyTeam, session.TheirTeam), nil
}

func mergeRoster(allies, enemies []rosterMember) []Participant {
	seen := make(map[string]struct{}, len(allies)+len(enemies))
	out := make([]Participant, 0, len(allies)+len(enemies))
	appendTeam := func(team string, members []rosterMember) {
		for index, member := range members {
			known := IsKnownPUUID(member.PUUID)
			key := member.PUUID
			if !known {
				key = fmt.Sprintf("%s-slot-%d", team, index)
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			identity := IdentityUnavailable
			name := "身份暂不可用"
			if known {
				identity = IdentityKnown
				name = "玩家资料加载中"
				if member.SummonerName != "" {
					name = member.SummonerName
				}
			}
			out = append(out, Participant{
				Key: key, PUUID: member.PUUID, DisplayName: name,
				Team: team, ChampionID: member.ChampionID,
				IdentityState: identity, PrivacyState: "unknown", HistoryState: HistoryPending,
			})
		}
	}
	appendTeam("ally", allies)
	appendTeam("enemy", enemies)
	return out
}
