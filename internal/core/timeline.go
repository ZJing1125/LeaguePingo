package core

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"
)

type TimelinePosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type TimelinePlayer struct {
	ParticipantID int               `json:"participantId"`
	Gold          *int              `json:"gold"`
	CS            *int              `json:"cs"`
	Level         *int              `json:"level"`
	Position      *TimelinePosition `json:"position,omitempty"`
}
type TimelineFrame struct {
	Timestamp int64            `json:"timestamp"`
	Players   []TimelinePlayer `json:"players"`
}
type TimelineEvent struct {
	Timestamp     int64             `json:"timestamp"`
	Type          string            `json:"type"`
	ParticipantID int               `json:"participantId"`
	KillerID      int               `json:"killerId"`
	VictimID      int               `json:"victimId"`
	TeamID        int               `json:"teamId"`
	ItemID        int               `json:"itemId"`
	BeforeID      int               `json:"beforeId"`
	AfterID       int               `json:"afterId"`
	SkillSlot     int               `json:"skillSlot"`
	Monster       string            `json:"monster"`
	Building      string            `json:"building"`
	Position      *TimelinePosition `json:"position,omitempty"`
}
type MatchTimeline struct {
	ConnectionID uint64          `json:"connectionId"`
	GameID       int64           `json:"gameId"`
	Frames       []TimelineFrame `json:"frames"`
	Events       []TimelineEvent `json:"events"`
	Partial      bool            `json:"partial"`
	FetchedAt    time.Time       `json:"fetchedAt"`
	Detail       *MatchDetail    `json:"detail,omitempty"`
}

// Wire format stays separate from the verified timeline model.
type matchTimelineResponse struct {
	GameID int64 `json:"gameId"`
	Frames []struct {
		Timestamp    *int64 `json:"timestamp"`
		Participants map[string]struct {
			ID       int  `json:"participantId"`
			Gold     *int `json:"totalGold"`
			Minions  *int `json:"minionsKilled"`
			Jungle   *int `json:"jungleMinionsKilled"`
			Level    *int `json:"level"`
			Position *struct {
				X *int `json:"x"`
				Y *int `json:"y"`
			} `json:"position"`
		} `json:"participantFrames"`
		Events []struct {
			Timestamp     *int64 `json:"timestamp"`
			Type          string `json:"type"`
			ParticipantID int    `json:"participantId"`
			KillerID      int    `json:"killerId"`
			VictimID      int    `json:"victimId"`
			TeamID        int    `json:"teamId"`
			KillerTeamID  int    `json:"killerTeamId"`
			ItemID        int    `json:"itemId"`
			BeforeID      int    `json:"beforeId"`
			AfterID       int    `json:"afterId"`
			SkillSlot     int    `json:"skillSlot"`
			Monster       string `json:"monsterType"`
			Building      string `json:"buildingType"`
			Position      *struct {
				X *int `json:"x"`
				Y *int `json:"y"`
			} `json:"position"`
		} `json:"events"`
	} `json:"frames"`
}

// Processes actual timeline frames; never synthesizes a route or economy curve
// from the final scoreboard. Participant IDs must belong to the verified game.
func ParseMatchTimeline(raw []byte, detail MatchDetail) (MatchTimeline, error) {
	var data matchTimelineResponse
	if len(raw) > 8<<20 || detail.GameID <= 0 || len(detail.Players) == 0 || json.Unmarshal(raw, &data) != nil || data.Frames == nil || len(data.Frames) == 0 || len(data.Frames) > 720 || (data.GameID != 0 && data.GameID != detail.GameID) {
		return MatchTimeline{}, errors.New("时间线格式或对局编号无法核验")
	}
	ids := map[int]bool{}
	for _, player := range detail.Players {
		ids[player.ParticipantID] = true
	}
	known := func(id int) bool { return id == 0 || ids[id] }
	position := func(x, y *int) *TimelinePosition {
		if x == nil || y == nil || *x < -1000 || *y < -1000 || *x > 30000 || *y > 30000 {
			return nil
		}
		return &TimelinePosition{X: *x, Y: *y}
	}
	out := MatchTimeline{GameID: detail.GameID, Frames: []TimelineFrame{}, Events: []TimelineEvent{}, Partial: detail.Partial, FetchedAt: time.Now()}
	last := int64(-1)
	for _, f := range data.Frames {
		if f.Timestamp == nil || *f.Timestamp < 0 || *f.Timestamp > 12*60*60*1000 || *f.Timestamp <= last || len(f.Participants) > 64 {
			return MatchTimeline{}, errors.New("时间线帧顺序或范围异常")
		}
		last = *f.Timestamp
		frame := TimelineFrame{Timestamp: last, Players: []TimelinePlayer{}}
		seen := map[int]bool{}
		for key, p := range f.Participants {
			id, err := strconv.Atoi(key)
			if err != nil || id <= 0 || !ids[id] || (p.ID != 0 && p.ID != id) || seen[id] {
				return MatchTimeline{}, errors.New("时间线参与者不属于本场对局")
			}
			seen[id] = true
			item := TimelinePlayer{ParticipantID: id, Gold: nonNegative(p.Gold), Level: nonNegative(p.Level)}
			if p.Minions != nil && p.Jungle != nil && *p.Minions >= 0 && *p.Jungle >= 0 {
				v := *p.Minions + *p.Jungle
				item.CS = &v
			}
			if p.Position != nil {
				item.Position = position(p.Position.X, p.Position.Y)
			}
			if item.Gold == nil {
				out.Partial = true
			}
			frame.Players = append(frame.Players, item)
		}
		if len(frame.Players) != len(detail.Players) {
			out.Partial = true
		}
		sort.Slice(frame.Players, func(i, j int) bool { return frame.Players[i].ParticipantID < frame.Players[j].ParticipantID })
		out.Frames = append(out.Frames, frame)
		for _, e := range f.Events {
			if len(out.Events) >= 20000 {
				return MatchTimeline{}, errors.New("时间线事件超出范围")
			}
			switch e.Type {
			case "CHAMPION_KILL", "BUILDING_KILL", "ELITE_MONSTER_KILL", "ITEM_PURCHASED", "ITEM_SOLD", "ITEM_DESTROYED", "ITEM_UNDO", "SKILL_LEVEL_UP":
			default:
				continue
			}
			if e.Timestamp == nil || *e.Timestamp < 0 || *e.Timestamp > 12*60*60*1000 || !known(e.ParticipantID) || !known(e.KillerID) || !known(e.VictimID) || (e.TeamID != 0 && e.TeamID != 100 && e.TeamID != 200) {
				return MatchTimeline{}, errors.New("时间线事件身份或时间异常")
			}
			item := TimelineEvent{Timestamp: *e.Timestamp, Type: e.Type, ParticipantID: e.ParticipantID, KillerID: e.KillerID, VictimID: e.VictimID, TeamID: e.TeamID, ItemID: max(0, e.ItemID), BeforeID: max(0, e.BeforeID), AfterID: max(0, e.AfterID), SkillSlot: e.SkillSlot}
			if item.TeamID == 0 && (e.KillerTeamID == 100 || e.KillerTeamID == 200) {
				item.TeamID = e.KillerTeamID
			}
			switch e.Monster {
			case "DRAGON", "HORDE", "RIFTHERALD", "RIFT_HERALD", "BARON_NASHOR", "ATAKHAN":
				item.Monster = e.Monster
			}
			switch e.Building {
			case "TOWER_BUILDING", "INHIBITOR_BUILDING", "NEXUS_BUILDING":
				item.Building = e.Building
			}
			if e.Position != nil {
				item.Position = position(e.Position.X, e.Position.Y)
			}
			out.Events = append(out.Events, item)
		}
	}
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].Timestamp < out.Events[j].Timestamp })
	return out, nil
}
