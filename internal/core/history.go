package core

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
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
	ParticipantIdentities []historyIdentity `json:"participantIdentities"`
	Teams                 []struct {
		TeamID     int  `json:"teamId"`
		Towers     *int `json:"towerKills"`
		Dragons    *int `json:"dragonKills"`
		Barons     *int `json:"baronKills"`
		Inhibitors *int `json:"inhibitorKills"`
		Heralds    *int `json:"riftHeraldKills"`
		Hordes     *int `json:"hordeKills"`
		Objectives map[string]struct {
			Kills *int `json:"kills"`
		} `json:"objectives"`
	} `json:"teams"`
}
type historyIdentity struct {
	ParticipantID int `json:"participantId"`
	Player        struct {
		PUUID        string `json:"puuid"`
		GameName     string `json:"gameName"`
		TagLine      string `json:"tagLine"`
		SummonerName string `json:"summonerName"`
	} `json:"player"`
}
type historyParticipant struct {
	TeamID             int    `json:"teamId"`
	PUUID              string `json:"puuid"`
	ParticipantID      int    `json:"participantId"`
	ChampionID         int    `json:"championId"`
	Spell1             *int   `json:"spell1Id"`
	Spell2             *int   `json:"spell2Id"`
	TeamPosition       string `json:"teamPosition"`
	IndividualPosition string `json:"individualPosition"`
	Lane               string `json:"lane"`
	Role               string `json:"role"`
	GameName           string `json:"riotIdGameName"`
	TagLine            string `json:"riotIdTagline"`
	SummonerName       string `json:"summonerName"`
	Timeline           struct {
		Lane string `json:"lane"`
		Role string `json:"role"`
	} `json:"timeline"`
	Stats struct {
		Healing       *int  `json:"totalHeal"`
		Shielding     *int  `json:"totalDamageShieldedOnTeammates"`
		SubteamID     *int  `json:"playerSubteamId"`
		Placement     *int  `json:"subteamPlacement"`
		ParticipantID int   `json:"participantId"`
		TeamID        int   `json:"teamId"`
		EarlyEnd      *bool `json:"gameEndedInEarlySurrender"`
		Perk0         *int  `json:"perk0"`
		Perk1         *int  `json:"perk1"`
		Perk2         *int  `json:"perk2"`
		Perk3         *int  `json:"perk3"`
		Perk4         *int  `json:"perk4"`
		Perk5         *int  `json:"perk5"`
		Primary       *int  `json:"perkPrimaryStyle"`
		Secondary     *int  `json:"perkSubStyle"`
		Win           *bool `json:"win"`
		ChampionID    int   `json:"championId"`
		Kills         *int  `json:"kills"`
		Deaths        *int  `json:"deaths"`
		Assists       *int  `json:"assists"`
		Gold          *int  `json:"goldEarned"`
		Damage        *int  `json:"totalDamageDealtToChampions"`
		Minions       *int  `json:"totalMinionsKilled"`
		Neutral       *int  `json:"neutralMinionsKilled"`
		Item0         *int  `json:"item0"`
		Item1         *int  `json:"item1"`
		Item2         *int  `json:"item2"`
		Item3         *int  `json:"item3"`
		Item4         *int  `json:"item4"`
		Item5         *int  `json:"item5"`
		Item6         *int  `json:"item6"`
		ChampionLevel *int  `json:"champLevel"`
		DamageTaken   *int  `json:"totalDamageTaken"`
		VisionScore   *int  `json:"visionScore"`
		WardsPlaced   *int  `json:"wardsPlaced"`
		WardsKilled   *int  `json:"wardsKilled"`
		ControlWards  *int  `json:"visionWardsBoughtInGame"`
		TripleKills   *int  `json:"tripleKills"`
		QuadraKills   *int  `json:"quadraKills"`
		PentaKills    *int  `json:"pentaKills"`
		Shard0        *int  `json:"statPerk0"`
		Shard1        *int  `json:"statPerk1"`
		Shard2        *int  `json:"statPerk2"`
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
			normalizeHistoryParticipant(&p)
			// A conflicting explicit identity must never be overridden by an index.
			if p.PUUID != puuid && (IsKnownPUUID(p.PUUID) || p.ParticipantID <= 0 || identity[p.ParticipantID] != puuid) {
				continue
			}
			match := matchSummary(game, p, now)
			match.Roster, match.Participation = historyPeers(game, p)
			result.Matches = append(result.Matches, match)
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
		Performance: matchPerformance(game, p),
		Arena:       arenaResult(game, p),
		GameID:      game.GameID, QueueID: game.QueueID, ChampionID: champion, PlayedAt: playedAt,
		Win: p.Stats.Win, Kills: nonNegative(p.Stats.Kills), Deaths: nonNegative(p.Stats.Deaths), Assists: nonNegative(p.Stats.Assists),
		Duration: nonNegative(game.Duration), Gold: nonNegative(p.Stats.Gold), Damage: nonNegative(p.Stats.Damage), CS: cs,
		Spell1: nonNegative(p.Spell1), Spell2: nonNegative(p.Spell2),
		Items: []*int{nonNegative(p.Stats.Item0), nonNegative(p.Stats.Item1), nonNegative(p.Stats.Item2), nonNegative(p.Stats.Item3), nonNegative(p.Stats.Item4), nonNegative(p.Stats.Item5), nonNegative(p.Stats.Item6)},
		MapID: nonNegative(game.MapID), Position: historyPosition(game, p), Lane: historyLane(p), EarlyEnd: p.Stats.EarlyEnd,
		RuneIDs: runes, RunePrimary: nonNegative(p.Stats.Primary), RuneSecondary: nonNegative(p.Stats.Secondary),
		ChampionLevel: nonNegative(p.Stats.ChampionLevel), DamageTaken: nonNegative(p.Stats.DamageTaken), VisionScore: nonNegative(p.Stats.VisionScore),
		WardsPlaced: nonNegative(p.Stats.WardsPlaced), WardsKilled: nonNegative(p.Stats.WardsKilled), ControlWards: nonNegative(p.Stats.ControlWards),
		TripleKills: nonNegative(p.Stats.TripleKills), QuadraKills: nonNegative(p.Stats.QuadraKills), PentaKills: nonNegative(p.Stats.PentaKills),
		RuneShards: positiveIDs(p.Stats.Shard0, p.Stats.Shard1, p.Stats.Shard2),
	}
}

func historyLane(p historyParticipant) string {
	lane := p.Lane
	if lane == "" {
		lane = p.Timeline.Lane
	}
	return strings.ToUpper(strings.TrimSpace(lane))
}

func historyRole(p historyParticipant) string {
	role := p.Role
	if role == "" {
		role = p.Timeline.Role
	}
	return strings.ToUpper(strings.TrimSpace(role))
}

func normalizeHistoryParticipant(p *historyParticipant) {
	if p.ParticipantID <= 0 {
		p.ParticipantID = p.Stats.ParticipantID
	}
	if p.TeamID <= 0 {
		p.TeamID = p.Stats.TeamID
	}
}

func standardPosition(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "TOP":
		return "TOP"
	case "JUNGLE":
		return "JUNGLE"
	case "MID", "MIDDLE":
		return "MIDDLE"
	case "BOT", "BOTTOM":
		return "BOTTOM"
	case "SUPPORT", "UTILITY":
		return "UTILITY"
	default:
		return ""
	}
}

func historyPosition(game historyGame, p historyParticipant) string {
	if position := standardPosition(p.TeamPosition); position != "" {
		return position
	}
	if position := standardPosition(p.IndividualPosition); position != "" {
		return position
	}
	// Tencent's LCU history contract predates teamPosition. On standard Rift
	// queues it instead returns Riot's explicit post-game lane/role pair. Only
	// unambiguous combinations are normalized; a lane alone, champion choice,
	// or a JUNGLE label without Smite is not enough evidence.
	if game.MapID != nil && *game.MapID != 11 {
		return ""
	}
	switch game.QueueID {
	case 400, 420, 430, 440, 490:
	default:
		return ""
	}
	lane, role := historyLane(p), historyRole(p)
	switch {
	case lane == "TOP" && role == "SOLO":
		return "TOP"
	case (lane == "MID" || lane == "MIDDLE") && role == "SOLO":
		return "MIDDLE"
	case lane == "JUNGLE" && (p.Spell1 != nil && *p.Spell1 == 11 || p.Spell2 != nil && *p.Spell2 == 11):
		return "JUNGLE"
	case (lane == "BOT" || lane == "BOTTOM") && (role == "CARRY" || role == "DUO_CARRY"):
		return "BOTTOM"
	case (lane == "BOT" || lane == "BOTTOM") && (role == "SUPPORT" || role == "DUO_SUPPORT"):
		return "UTILITY"
	default:
		return ""
	}
}

func positiveIDs(values ...*int) []int {
	result := []int{}
	for _, value := range values {
		if value != nil && *value > 0 {
			result = append(result, *value)
		}
	}
	return result
}

func historyName(game historyGame, p historyParticipant) string {
	if p.GameName != "" && p.TagLine != "" {
		return p.GameName + "#" + p.TagLine
	}
	for _, identity := range game.ParticipantIdentities {
		if identity.ParticipantID != p.ParticipantID || (IsKnownPUUID(p.PUUID) && p.PUUID != identity.Player.PUUID) {
			continue
		}
		if identity.Player.GameName != "" && identity.Player.TagLine != "" {
			return identity.Player.GameName + "#" + identity.Player.TagLine
		}
		if identity.Player.SummonerName != "" {
			return identity.Player.SummonerName
		}
	}
	return p.SummonerName
}

func historyPeers(game historyGame, target historyParticipant) ([]MatchPeer, *float64) {
	if IsArenaQueue(game.QueueID) {
		return nil, nil // Arena subteams are not the two legacy teamId values.
	}
	peers := []MatchPeer{}
	seen := map[int]bool{}
	counts := map[int]int{}
	total, complete := 0, true
	for _, raw := range game.Participants {
		var p historyParticipant
		if json.Unmarshal(raw, &p) != nil {
			return nil, nil
		}
		normalizeHistoryParticipant(&p)
		if p.ParticipantID <= 0 || seen[p.ParticipantID] {
			return nil, nil
		}
		seen[p.ParticipantID] = true
		counts[p.TeamID]++
		champion := p.ChampionID
		if champion == 0 {
			champion = p.Stats.ChampionID
		}
		peers = append(peers, MatchPeer{ParticipantID: p.ParticipantID, TeamID: p.TeamID, ChampionID: champion, DisplayName: historyName(game, p), Target: p.ParticipantID == target.ParticipantID})
		if p.TeamID == target.TeamID {
			if p.Stats.Kills == nil || *p.Stats.Kills < 0 {
				complete = false
			} else {
				total += *p.Stats.Kills
			}
		}
	}
	if len(peers) != 10 || counts[100] != 5 || counts[200] != 5 || !complete || target.Stats.Kills == nil || target.Stats.Assists == nil || *target.Stats.Kills < 0 || *target.Stats.Assists < 0 || total <= 0 {
		return peers, nil
	}
	ka := *target.Stats.Kills + *target.Stats.Assists
	if ka > total {
		return peers, nil
	}
	rate := float64(ka) * 100 / float64(total)
	return peers, &rate
}
func nonNegative(value *int) *int {
	if value != nil && *value < 0 {
		return nil
	}
	return value
}
