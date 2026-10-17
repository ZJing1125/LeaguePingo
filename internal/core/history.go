package core

import (
	"encoding/json"
	"errors"
	"sort"
	"time"
)

type historyEnvelope struct {
	Games struct {
		Games []json.RawMessage `json:"games"`
	} `json:"games"`
}
type historyGame struct {
	GameID                int64             `json:"gameId"`
	GameCreation          int64             `json:"gameCreation"`
	GameDate              int64             `json:"gameDate"`
	QueueID               int               `json:"queueId"`
	MapID                 *int              `json:"mapId"`
	Duration              *int              `json:"gameDuration"`
	Participants          []json.RawMessage `json:"participants"`
	ParticipantIdentities []struct {
		ParticipantID int `json:"participantId"`
		Player        struct {
			PUUID        string `json:"puuid"`
			GameName     string `json:"gameName"`
			TagLine      string `json:"tagLine"`
			SummonerName string `json:"summonerName"`
		} `json:"player"`
	} `json:"participantIdentities"`
}
type historyParticipant struct {
	TeamID        int    `json:"teamId"`
	PUUID         string `json:"puuid"`
	ParticipantID int    `json:"participantId"`
	ChampionID    int    `json:"championId"`
	Spell1        *int   `json:"spell1Id"`
	Spell2        *int   `json:"spell2Id"`
	TeamPosition  string `json:"teamPosition"`
	Timeline      struct {
		Lane string `json:"lane"`
		Role string `json:"role"`
	} `json:"timeline"`
	Stats struct {
		EarlyEnd   *bool `json:"gameEndedInEarlySurrender"`
		Perk0      *int  `json:"perk0"`
		Perk1      *int  `json:"perk1"`
		Perk2      *int  `json:"perk2"`
		Perk3      *int  `json:"perk3"`
		Perk4      *int  `json:"perk4"`
		Perk5      *int  `json:"perk5"`
		Primary    *int  `json:"perkPrimaryStyle"`
		Secondary  *int  `json:"perkSubStyle"`
		Win        *bool `json:"win"`
		ChampionID int   `json:"championId"`
		Kills      *int  `json:"kills"`
		Deaths     *int  `json:"deaths"`
		Assists    *int  `json:"assists"`
		Gold       *int  `json:"goldEarned"`
		Damage     *int  `json:"totalDamageDealtToChampions"`
		Minions    *int  `json:"totalMinionsKilled"`
		Neutral    *int  `json:"neutralMinionsKilled"`
		Item0      *int  `json:"item0"`
		Item1      *int  `json:"item1"`
		Item2      *int  `json:"item2"`
		Item3      *int  `json:"item3"`
		Item4      *int  `json:"item4"`
		Item5      *int  `json:"item5"`
		Item6      *int  `json:"item6"`
	} `json:"stats"`
}

// Only verified participants contribute to a sample. Missing values stay nil.
func ParseHistory(data []byte, puuid string) (HistoryResult, error) {
	var response historyEnvelope
	if err := json.Unmarshal(data, &response); err != nil {
		return HistoryResult{}, errors.New("战绩返回格式异常")
	}
	if response.Games.Games == nil {
		return HistoryResult{}, errors.New("战绩接口未返回 games 数组")
	}
	if !IsKnownPUUID(puuid) {
		return HistoryResult{}, errors.New("无法核验目标玩家身份")
	}
	now := time.Now()
	result := HistoryResult{Matches: []Match{}, Meta: HistoryMeta{Returned: len(response.Games.Games), Reasons: map[string]int{}, FetchedAt: &now, Scope: "最近获取的至多 20 场，所有模式；筛选仅作用于已获取样本"}}
	seen := map[int64]bool{}
	exclude := func(reason string) { result.Meta.Excluded++; result.Meta.Reasons[reason]++ }
	for _, rawGame := range response.Games.Games {
		var game historyGame
		if json.Unmarshal(rawGame, &game) != nil || game.GameID <= 0 {
			exclude("invalid_record")
			continue
		}
		if seen[game.GameID] {
			exclude("duplicate")
			continue
		}
		identity := map[int]string{}
		for _, item := range game.ParticipantIdentities {
			if item.ParticipantID > 0 {
				identity[item.ParticipantID] = item.Player.PUUID
			}
		}
		found := false
		for _, raw := range game.Participants {
			var p historyParticipant
			if json.Unmarshal(raw, &p) != nil {
				continue
			}
			// A conflicting explicit identity must never be overridden by an index.
			if p.PUUID != puuid && (IsKnownPUUID(p.PUUID) || p.ParticipantID <= 0 || identity[p.ParticipantID] != puuid) {
				continue
			}
			result.Matches = append(result.Matches, matchSummary(game, p, now))
			seen[game.GameID], found = true, true
			break
		}
		if !found {
			exclude("identity_unverified")
		}
	}
	result.Meta.Parsed = len(result.Matches)
	if result.Meta.Returned > 0 && result.Meta.Parsed == 0 {
		return result, errors.New("战绩中没有可核验的目标玩家记录")
	}
	sort.SliceStable(result.Matches, func(i, j int) bool {
		a, b := result.Matches[i].PlayedAt, result.Matches[j].PlayedAt
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.After(*b)
	})
	return result, nil
}

func matchSummary(game historyGame, p historyParticipant, now time.Time) Match {
	champion := p.ChampionID
	if champion == 0 {
		champion = p.Stats.ChampionID
	}
	stamp := game.GameCreation
	if stamp <= 0 {
		stamp = game.GameDate
	}
	if stamp > 1e12 {
		stamp /= 1000
	}
	var playedAt *time.Time
	if stamp >= 946684800 && stamp <= now.Add(24*time.Hour).Unix() {
		value := time.Unix(stamp, 0)
		playedAt = &value
	}
	var cs *int
	if nonNegative(p.Stats.Minions) != nil && nonNegative(p.Stats.Neutral) != nil {
		value := *p.Stats.Minions + *p.Stats.Neutral
		cs = &value
	}
	runes := []int{}
	for _, id := range []*int{p.Stats.Perk0, p.Stats.Perk1, p.Stats.Perk2, p.Stats.Perk3, p.Stats.Perk4, p.Stats.Perk5} {
		if id != nil && *id > 0 {
			runes = append(runes, *id)
		}
	}
	return Match{
		GameID: game.GameID, QueueID: game.QueueID, ChampionID: champion, PlayedAt: playedAt,
		Win: p.Stats.Win, Kills: nonNegative(p.Stats.Kills), Deaths: nonNegative(p.Stats.Deaths), Assists: nonNegative(p.Stats.Assists),
		Duration: nonNegative(game.Duration), Gold: nonNegative(p.Stats.Gold), Damage: nonNegative(p.Stats.Damage), CS: cs,
		Spell1: nonNegative(p.Spell1), Spell2: nonNegative(p.Spell2),
		Items: []*int{nonNegative(p.Stats.Item0), nonNegative(p.Stats.Item1), nonNegative(p.Stats.Item2), nonNegative(p.Stats.Item3), nonNegative(p.Stats.Item4), nonNegative(p.Stats.Item5), nonNegative(p.Stats.Item6)},
		MapID: nonNegative(game.MapID), Position: p.TeamPosition, Lane: p.Timeline.Lane, EarlyEnd: p.Stats.EarlyEnd,
		RuneIDs: runes, RunePrimary: nonNegative(p.Stats.Primary), RuneSecondary: nonNegative(p.Stats.Secondary),
	}
}
func nonNegative(value *int) *int {
	if value != nil && *value < 0 {
		return nil
	}
	return value
}
