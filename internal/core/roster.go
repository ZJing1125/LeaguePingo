package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rosterMember struct {
	TeamParticipantID *int64 `json:"teamParticipantId"`
	SelectedPosition  string `json:"selectedPosition"`
	AssignedPosition  string `json:"assignedPosition"`
	NameFields
	PUUID              string `json:"puuid"`
	ChampionID         *int   `json:"championId"`
	ChampionPickIntent *int   `json:"championPickIntent"`
	CellID             *int   `json:"cellId"`
}

type gameflowSession struct {
	GameData struct {
		GameID  int64          `json:"gameId"`
		TeamOne []rosterMember `json:"teamOne"`
		TeamTwo []rosterMember `json:"teamTwo"`
	} `json:"gameData"`
}

type champSelectSession struct {
	GameID    int64          `json:"gameId"`
	MyTeam    []rosterMember `json:"myTeam"`
	TheirTeam []rosterMember `json:"theirTeam"`
	Actions   [][]struct {
		ActorCellID *int   `json:"actorCellId"`
		ChampionID  int    `json:"championId"`
		Type        string `json:"type"`
		Completed   bool   `json:"completed"`
		InProgress  bool   `json:"isInProgress"`
	} `json:"actions"`
}

func IsKnownPUUID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "0" && value != "00000000-0000-0000-0000-000000000000"
}

func IsPlaceholderName(name string) bool {
	return name == "" || name == "玩家资料加载中" || name == "玩家名称暂不可用"
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
	players := mergeRoster(allies, enemies)
	AssignPremades(players, "lcu")
	return session.GameData.GameID, players, nil
}

func ParseChampSelectRoster(data []byte) ([]Participant, error) {
	_, roster, err := ParseChampSelectSession(data)
	return roster, err
}

func ParseChampSelectSession(data []byte) (int64, []Participant, error) {
	var session champSelectSession
	if err := json.Unmarshal(data, &session); err != nil {
		return 0, nil, err
	}
	if session.MyTeam == nil || session.TheirTeam == nil {
		return 0, nil, fmt.Errorf("选人名单格式不可用")
	}
	// Active pick actions can contain the current hover before myTeam updates
	// championId/pickIntent. Bans and other players' actions never become a pick.
	for _, round := range session.Actions {
		for _, a := range round {
			if a.Type != "pick" || a.ActorCellID == nil || !a.InProgress || a.Completed || a.ChampionID <= 0 {
				continue
			}
			for i := range session.MyTeam {
				member := &session.MyTeam[i]
				if member.CellID != nil && *member.CellID == *a.ActorCellID && (member.ChampionID == nil || *member.ChampionID == 0) {
					id := a.ChampionID
					member.ChampionPickIntent = &id
				}
			}
		}
	}
	return session.GameID, mergeRoster(session.MyTeam, session.TheirTeam), nil
}

func ParseLobbyRoster(data []byte) ([]Participant, error) {
	var lobby struct {
		Members []struct {
			NameFields
			PUUID     string `json:"puuid"`
			Bot       bool   `json:"isBot"`
			Spectator bool   `json:"isSpectator"`
		} `json:"members"`
	}
	if json.Unmarshal(data, &lobby) != nil || lobby.Members == nil {
		return nil, fmt.Errorf("房间成员格式不可用")
	}
	members := []rosterMember{}
	for _, p := range lobby.Members {
		if !p.Bot && !p.Spectator {
			members = append(members, rosterMember{PUUID: p.PUUID, NameFields: p.NameFields})
		}
	}
	return mergeRoster(members, nil), nil
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
				if member.CellID != nil {
					key = fmt.Sprintf("%s-cell-%d", team, *member.CellID)
				}
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			identity := IdentityUnavailable
			name := "身份暂不可用"
			if known {
				identity = IdentityKnown
				name = "玩家名称暂不可用"
				if provided := member.Name(); provided != "" {
					name = provided
				}
			}
			championID, championState := 0, "unavailable"
			if member.ChampionID != nil && *member.ChampionID >= 0 {
				championID, championState = *member.ChampionID, "unselected"
				if championID > 0 {
					championState = "selected"
				}
			}
			if championID == 0 && member.ChampionPickIntent != nil && *member.ChampionPickIntent > 0 {
				championID, championState = *member.ChampionPickIntent, "intent"
			}
			out = append(out, Participant{
				Key: key, PUUID: member.PUUID, DisplayName: name,
				GroupToken: groupToken(member.TeamParticipantID), Position: rosterPosition(member.SelectedPosition, member.AssignedPosition),
				Team: team, ChampionID: championID, ChampionState: championState,
				IdentityState: identity, PrivacyState: "unknown", HistoryState: HistoryPending,
			})
		}
	}
	appendTeam("ally", allies)
	appendTeam("enemy", enemies)
	return out
}
