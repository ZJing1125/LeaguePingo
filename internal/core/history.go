package core

import (
	"encoding/json"
	"errors"
	"time"
)

type historyEnvelope struct {
	Games struct {
		Games []historyGame `json:"games"`
	} `json:"games"`
}

type historyGame struct {
	GameID                int64             `json:"gameId"`
	GameCreation          int64             `json:"gameCreation"`
	GameDate              int64             `json:"gameDate"`
	QueueID               int               `json:"queueId"`
	Participants          []json.RawMessage `json:"participants"`
	ParticipantIdentities []struct {
		ParticipantID int `json:"participantId"`
		Player        struct {
			PUUID string `json:"puuid"`
		} `json:"player"`
	} `json:"participantIdentities"`
}

type historyParticipant struct {
	PUUID         string `json:"puuid"`
	ParticipantID int    `json:"participantId"`
	ChampionID    int    `json:"championId"`
	Stats         struct {
		Win        bool `json:"win"`
		ChampionID int  `json:"championId"`
		Kills      int  `json:"kills"`
		Deaths     int  `json:"deaths"`
		Assists    int  `json:"assists"`
	} `json:"stats"`
}

// ParseHistory reads only records whose participant can be matched to the requested PUUID.
func ParseHistory(data []byte, puuid string) ([]Match, error) {
	var response historyEnvelope
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if response.Games.Games == nil {
		return nil, errors.New("战绩接口未返回 games 数组")
	}
	result := make([]Match, 0, len(response.Games.Games))
	for _, game := range response.Games.Games {
		identity := make(map[int]string, len(game.ParticipantIdentities))
		for _, item := range game.ParticipantIdentities {
			identity[item.ParticipantID] = item.Player.PUUID
		}
		for _, raw := range game.Participants {
			var player historyParticipant
			if err := json.Unmarshal(raw, &player); err != nil || (player.PUUID != puuid && identity[player.ParticipantID] != puuid) {
				continue
			}
			championID := player.ChampionID
			if championID == 0 {
				championID = player.Stats.ChampionID
			}
			stamp := game.GameCreation
			if stamp == 0 {
				stamp = game.GameDate
			}
			if stamp > 1e12 {
				stamp /= 1000
			}
			result = append(result, Match{
				GameID: game.GameID, PlayedAt: time.Unix(stamp, 0), QueueID: game.QueueID,
				ChampionID: championID, Win: player.Stats.Win,
				Kills: player.Stats.Kills, Deaths: player.Stats.Deaths, Assists: player.Stats.Assists,
			})
			break
		}
	}
	if len(response.Games.Games) > 0 && len(result) == 0 {
		return nil, errors.New("战绩中无法核验目标玩家身份")
	}
	return result, nil
}
