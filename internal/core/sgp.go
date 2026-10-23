package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Normalize the official summary envelope, then reuse identity validation and
// nullable stats parsing. No unknown values are replaced with invented zeros.
func ParseSGPHistory(raw []byte, target, platform string) (HistoryResult, error) {
	// Metadata uses snake_case; retain explicit tags below rather than accepting
	// mismatched platform/match summaries as valid pages.
	var envelope struct {
		Games []struct {
			Metadata struct {
				Product  string `json:"product"`
				InfoType string `json:"info_type"`
				MatchID  string `json:"match_id"`
			} `json:"metadata"`
			JSON json.RawMessage `json:"json"`
		} `json:"games"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Games == nil || len(envelope.Games) > 20 {
		return HistoryResult{}, errors.New("完整战绩页格式异常")
	}
	normalized := []json.RawMessage{}
	for _, item := range envelope.Games {
		var game struct {
			GameID       int64             `json:"gameId"`
			Platform     string            `json:"platformId"`
			Participants []json.RawMessage `json:"participants"`
		}
		if json.Unmarshal(item.JSON, &game) != nil || game.GameID <= 0 || game.Platform != platform || item.Metadata.Product != "lol" || item.Metadata.InfoType != "summary" || item.Metadata.MatchID != fmt.Sprintf("%s_%d", platform, game.GameID) {
			return HistoryResult{}, errors.New("战绩服务的比赛或大区无法核验")
		}
		out, err := normalizeSGPGame(item.JSON)
		if err != nil {
			return HistoryResult{}, err
		}
		normalized = append(normalized, out)
	}
	data, _ := json.Marshal(map[string]any{"games": map[string]any{"games": normalized}})
	result, err := ParseHistory(data, target)
	result.Meta.Scope = "腾讯当前大区完整战绩分页；仅统计已核验目标玩家记录"
	return result, err
}

func normalizeSGPGame(raw []byte) ([]byte, error) {
	var game struct {
		Participants []json.RawMessage `json:"participants"`
	}
	if json.Unmarshal(raw, &game) != nil || len(game.Participants) > 64 {
		return nil, errors.New("战绩参与者格式异常")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	participants := []json.RawMessage{}
	for _, participant := range game.Participants {
		var p map[string]json.RawMessage
		if json.Unmarshal(participant, &p) != nil || p == nil {
			return nil, errors.New("战绩参与者格式异常")
		}
		// SGP stats are flat; LCU stats are nested. Preserve both identity and nulls.
		stats := participant
		p["stats"] = stats
		if v, ok := p["summoner1Id"]; ok {
			p["spell1Id"] = v
		}
		if v, ok := p["summoner2Id"]; ok {
			p["spell2Id"] = v
		}
		var perks struct {
			StatPerks struct {
				Offense *int `json:"offense"`
				Flex    *int `json:"flex"`
				Defense *int `json:"defense"`
			} `json:"statPerks"`
			Styles []struct {
				Style      int `json:"style"`
				Selections []struct {
					Perk int `json:"perk"`
				} `json:"selections"`
			} `json:"styles"`
		}
		if json.Unmarshal(p["perks"], &perks) == nil {
			var statFields map[string]json.RawMessage
			_ = json.Unmarshal(stats, &statFields)
			index := 0
			for i, style := range perks.Styles {
				if i < 2 && style.Style > 0 {
					key := "perkPrimaryStyle"
					if i == 1 {
						key = "perkSubStyle"
					}
					statFields[key], _ = json.Marshal(style.Style)
				}
				for _, selection := range style.Selections {
					if index < 6 {
						statFields[fmt.Sprintf("perk%d", index)], _ = json.Marshal(selection.Perk)
						index++
					}
				}
			}
			for i, shard := range []*int{perks.StatPerks.Offense, perks.StatPerks.Flex, perks.StatPerks.Defense} {
				if shard != nil {
					statFields[fmt.Sprintf("statPerk%d", i)], _ = json.Marshal(shard)
				}
			}
			p["stats"], _ = json.Marshal(statFields)
		}
		out, _ := json.Marshal(p)
		participants = append(participants, out)
	}
	fields["participants"], _ = json.Marshal(participants)
	out, _ := json.Marshal(fields)
	return out, nil
}

// Explicit full scoreboard from the same verified summary contract.
func ParseSGPMatchDetail(raw []byte, gameID int64, target, platform string) (MatchDetail, error) {
	var item struct {
		Metadata struct {
			Product  string `json:"product"`
			InfoType string `json:"info_type"`
			MatchID  string `json:"match_id"`
		} `json:"metadata"`
		JSON json.RawMessage `json:"json"`
	}
	if json.Unmarshal(raw, &item) != nil || item.Metadata.Product != "lol" || item.Metadata.InfoType != "summary" || item.Metadata.MatchID != fmt.Sprintf("%s_%d", platform, gameID) {
		return MatchDetail{}, errors.New("对局详情服务的大区或比赛无法核验")
	}
	var game struct {
		GameID   int64  `json:"gameId"`
		Platform string `json:"platformId"`
	}
	if json.Unmarshal(item.JSON, &game) != nil || game.GameID != gameID || game.Platform != platform {
		return MatchDetail{}, errors.New("对局详情身份范围不匹配")
	}
	data, err := normalizeSGPGame(item.JSON)
	if err != nil {
		return MatchDetail{}, err
	}
	return ParseMatchDetail(data, gameID, target)
}

func ParseSGPTimeline(raw []byte, detail MatchDetail, target, platform string) (MatchTimeline, error) {
	if !IsKnownPUUID(target) {
		return MatchTimeline{}, errors.New("时间线目标身份缺失")
	}
	var item struct {
		Metadata struct {
			Product  string `json:"product"`
			InfoType string `json:"info_type"`
			MatchID  string `json:"match_id"`
		} `json:"metadata"`
		JSON json.RawMessage `json:"json"`
	}
	if json.Unmarshal(raw, &item) != nil || item.Metadata.Product != "lol" || item.Metadata.InfoType != "details" || item.Metadata.MatchID != fmt.Sprintf("%s_%d", platform, detail.GameID) {
		return MatchTimeline{}, errors.New("时间线服务的大区或比赛无法核验")
	}
	var game struct {
		GameID       int64 `json:"gameId"`
		Participants []struct {
			ID    int    `json:"participantId"`
			PUUID string `json:"puuid"`
		} `json:"participants"`
	}
	if json.Unmarshal(item.JSON, &game) != nil || game.GameID != detail.GameID || len(game.Participants) > 64 {
		return MatchTimeline{}, errors.New("时间线对局编号不匹配")
	}
	targets := 0
	seen := map[int]bool{}
	for _, p := range game.Participants {
		if p.ID <= 0 || seen[p.ID] {
			return MatchTimeline{}, errors.New("时间线参与者编号重复或缺失")
		}
		seen[p.ID] = true
		if p.PUUID == target {
			for _, known := range detail.Players {
				if known.Target && known.ParticipantID == p.ID {
					targets++
				}
			}
		}
	}
	if targets != 1 {
		return MatchTimeline{}, errors.New("时间线目标玩家无法核验")
	}
	return ParseMatchTimeline(item.JSON, detail)
}
