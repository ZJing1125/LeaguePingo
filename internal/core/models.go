package core

import "time"

type IdentityState string

const (
	IdentityKnown       IdentityState = "known"
	IdentityUnavailable IdentityState = "unavailable"
)

type HistoryState string

const (
	HistoryPending HistoryState = "pending"
	HistoryLoading HistoryState = "loading"
	HistoryReady   HistoryState = "ready"
	HistoryEmpty   HistoryState = "empty"
	HistoryFailed  HistoryState = "failed"
)

type Participant struct {
	Key           string        `json:"key"`
	PUUID         string        `json:"puuid,omitempty"`
	DisplayName   string        `json:"displayName"`
	Team          string        `json:"team"`
	ChampionID    int           `json:"championId,omitempty"`
	IdentityState IdentityState `json:"identityState"`
	PrivacyState  string        `json:"privacyState"`
	HistoryState  HistoryState  `json:"historyState"`
	History       []Match       `json:"history,omitempty"`
	Stats         Stats         `json:"stats"`
	Error         string        `json:"error,omitempty"`
}

type Match struct {
	GameID     int64     `json:"gameId"`
	PlayedAt   time.Time `json:"playedAt"`
	QueueID    int       `json:"queueId"`
	ChampionID int       `json:"championId"`
	Win        bool      `json:"win"`
	Kills      int       `json:"kills"`
	Deaths     int       `json:"deaths"`
	Assists    int       `json:"assists"`
}

type Stats struct {
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"winRate"`
	KDA     float64 `json:"kda"`
}

type Snapshot struct {
	Generation   uint64        `json:"generation"`
	Phase        string        `json:"phase"`
	Stage        string        `json:"stage"`
	Connected    bool          `json:"connected"`
	Region       string        `json:"region"`
	GameID       int64         `json:"gameId"`
	Frozen       bool          `json:"frozen"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	Participants []Participant `json:"participants"`
	Message      string        `json:"message,omitempty"`
}

func CalculateStats(matches []Match) Stats {
	var out Stats
	var kills, deaths, assists int
	for _, match := range matches {
		out.Games++
		if match.Win {
			out.Wins++
		}
		kills += match.Kills
		deaths += match.Deaths
		assists += match.Assists
	}
	if out.Games > 0 {
		out.WinRate = float64(out.Wins) * 100 / float64(out.Games)
	}
	if deaths == 0 {
		out.KDA = float64(kills + assists)
	} else {
		out.KDA = float64(kills+assists) / float64(deaths)
	}
	return out
}
