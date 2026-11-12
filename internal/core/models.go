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
	GroupToken    string        `json:"-"` // transient opaque source ID; never persisted
	PremadeLabel  string        `json:"premadeLabel,omitempty"`
	PremadeGroup  string        `json:"premadeGroup,omitempty"`
	PremadeSize   int           `json:"premadeSize,omitempty"`
	GroupSource   string        `json:"groupSource,omitempty"`
	Position      string        `json:"position,omitempty"`
	ConnectionID  uint64        `json:"connectionId"`
	Key           string        `json:"key"`
	PUUID         string        `json:"puuid,omitempty"`
	DisplayName   string        `json:"displayName"`
	Team          string        `json:"team"`
	ChampionID    int           `json:"championId,omitempty"`
	ChampionState string        `json:"championState,omitempty"` // selected, intent, unselected, unavailable
	IdentityState IdentityState `json:"identityState"`
	PrivacyState  string        `json:"privacyState"`
	HistoryState  HistoryState  `json:"historyState"`
	History       []Match       `json:"history,omitempty"`
	HistoryMeta   HistoryMeta   `json:"historyMeta"`
	Cached        bool          `json:"cached"`
	Stale         bool          `json:"stale"`
	Stats         Stats         `json:"stats"`
	Error         string        `json:"error,omitempty"`
	Profile       *Profile      `json:"profile,omitempty"`
	Scout         *ScoutReport  `json:"scout,omitempty"`
}

// Optional capabilities: absent fields are never presented as unranked or zero.
type Profile struct {
	UpdatedAt        *time.Time                `json:"updatedAt"`
	Fields           map[string]FieldFreshness `json:"fields"`
	PrivacyState     string                    `json:"privacyState"`
	ProfileIconID    *int                      `json:"profileIconId"`
	ProfileIconData  string                    `json:"profileIconData"`
	Level            *int                      `json:"level"`
	RankState        string                    `json:"rankState"`
	RankMessage      string                    `json:"rankMessage,omitempty"`
	Ranks            []Rank                    `json:"ranks"`
	Masteries        []ChampionMastery         `json:"masteries"`
	MasteryFetchedAt *time.Time                `json:"masteryFetchedAt,omitempty"`
}

// A retained value remains useful but is not verified by the latest request.
type FieldFreshness struct {
	State     string     `json:"state"`
	FetchedAt *time.Time `json:"fetchedAt"`
}
type Rank struct {
	Queue                   string `json:"queue"`
	Tier                    string `json:"tier"`
	Division                string `json:"division"`
	LP                      *int   `json:"lp"`
	Wins                    *int   `json:"wins"`
	Losses                  *int   `json:"losses"`
	Source                  string `json:"source,omitempty"`
	SeasonID                *int   `json:"seasonId,omitempty"`
	HighestTier             string `json:"highestTier,omitempty"`
	HighestDivision         string `json:"highestDivision,omitempty"`
	PreviousHighestTier     string `json:"previousHighestTier,omitempty"`
	PreviousHighestDivision string `json:"previousHighestDivision,omitempty"`
	PreviousTier            string `json:"previousTier,omitempty"`
	PreviousDivision        string `json:"previousDivision,omitempty"`
}

type Match struct {
	GameID        int64       `json:"gameId"`
	PlayedAt      *time.Time  `json:"playedAt"`
	QueueID       int         `json:"queueId"`
	ChampionID    int         `json:"championId"`
	Win           *bool       `json:"win"`
	Kills         *int        `json:"kills"`
	Deaths        *int        `json:"deaths"`
	Assists       *int        `json:"assists"`
	Duration      *int        `json:"duration"`
	Gold          *int        `json:"gold"`
	Damage        *int        `json:"damage"`
	CS            *int        `json:"cs"`
	Items         []*int      `json:"items"`
	Spell1        *int        `json:"spell1"`
	Spell2        *int        `json:"spell2"`
	MapID         *int        `json:"mapId"`
	Position      string      `json:"position"`
	Lane          string      `json:"lane"`
	EarlyEnd      *bool       `json:"earlyEnd"`
	RuneIDs       []int       `json:"runeIds"`
	RunePrimary   *int        `json:"runePrimary"`
	RuneSecondary *int        `json:"runeSecondary"`
	RuneShards    []int       `json:"runeShards,omitempty"`
	ChampionLevel *int        `json:"championLevel"`
	DamageTaken   *int        `json:"damageTaken"`
	VisionScore   *int        `json:"visionScore"`
	WardsPlaced   *int        `json:"wardsPlaced"`
	WardsKilled   *int        `json:"wardsKilled"`
	ControlWards  *int        `json:"controlWards"`
	TripleKills   *int        `json:"tripleKills"`
	QuadraKills   *int        `json:"quadraKills"`
	PentaKills    *int        `json:"pentaKills"`
	Participation *float64    `json:"participation"`
	Roster        []MatchPeer `json:"roster,omitempty"`
}

// A compact preview from the already returned game; never a per-row lookup.
type MatchPeer struct {
	ParticipantID int    `json:"participantId"`
	TeamID        int    `json:"teamId"`
	ChampionID    int    `json:"championId"`
	DisplayName   string `json:"displayName"`
	Target        bool   `json:"target"`
}

// IsRemake uses both the explicit early-end signal and a short duration. A
// normal early surrender remains a completed loss; a short aborted game stays
// visible in history but does not contribute to win-rate or KDA statistics.
func IsRemake(match Match) bool {
	return match.EarlyEnd != nil && *match.EarlyEnd && match.Duration != nil && *match.Duration >= 0 && *match.Duration <= 5*60
}

type HistoryMeta struct {
	Service   *HistoryServiceState `json:"service,omitempty"`
	CheckedAt *time.Time           `json:"checkedAt,omitempty"`
	Returned  int                  `json:"returned"`
	Parsed    int                  `json:"parsed"`
	Excluded  int                  `json:"excluded"`
	Reasons   map[string]int       `json:"reasons,omitempty"`
	FetchedAt *time.Time           `json:"fetchedAt"`
	Scope     string               `json:"scope"`
}

// Transient service state. Never persist it as history or infer player privacy.
type HistoryServiceState struct {
	Code    string     `json:"code"`
	RetryAt *time.Time `json:"retryAt,omitempty"`
}
type HistoryResult struct {
	Matches []Match     `json:"matches"`
	Meta    HistoryMeta `json:"meta"`
}

type HistoryPage struct {
	Matches     []Match     `json:"matches"`
	Meta        HistoryMeta `json:"meta"`
	Offset      int         `json:"offset"`
	NextOffset  int         `json:"nextOffset"`
	HasMore     bool        `json:"hasMore"`
	LimitReason string      `json:"limitReason,omitempty"`
}

type MatchPlayer struct {
	ParticipantID int      `json:"participantId"`
	DisplayName   string   `json:"displayName"`
	TeamID        int      `json:"teamId"`
	Target        bool     `json:"target"`
	Summary       Match    `json:"summary"`
	DamageShare   *float64 `json:"damageShare"`
	GoldShare     *float64 `json:"goldShare"`
	Participation *float64 `json:"participation"`
}

type MatchDetail struct {
	ConnectionID uint64        `json:"connectionId"`
	GameID       int64         `json:"gameId"`
	Players      []MatchPlayer `json:"players"`
	Partial      bool          `json:"partial"`
	FetchedAt    time.Time     `json:"fetchedAt"`
	MapID        *int          `json:"mapId"`
	Teams        []MatchTeam   `json:"teams"`
}

type MatchTeam struct {
	TeamID     int  `json:"teamId"`
	Towers     *int `json:"towers"`
	Dragons    *int `json:"dragons"`
	Barons     *int `json:"barons"`
	Inhibitors *int `json:"inhibitors"`
	Heralds    *int `json:"heralds"`
	Hordes     *int `json:"hordes"`
}

type Stats struct {
	Remakes      int      `json:"remakes,omitempty"`
	Games        int      `json:"games"`
	Wins         int      `json:"wins"`
	Losses       int      `json:"losses"`
	OutcomeGames int      `json:"outcomeGames"`
	KDAGames     int      `json:"kdaGames"`
	WinRate      *float64 `json:"winRate"`
	KDA          *float64 `json:"kda"`
}

type Snapshot struct {
	GroupCoverage int           `json:"groupCoverage"`
	GroupStatus   string        `json:"groupStatus"`
	GameMode      string        `json:"gameMode,omitempty"`
	MapID         int           `json:"mapId,omitempty"`
	GameVersion   string        `json:"gameVersion,omitempty"`
	ConnectionID  uint64        `json:"connectionId"`
	SelfKey       string        `json:"selfKey"`
	KnownCount    int           `json:"knownCount"`
	ReadyCount    int           `json:"readyCount"`
	RosterError   string        `json:"rosterError"`
	Generation    uint64        `json:"generation"`
	Phase         string        `json:"phase"`
	Stage         string        `json:"stage"`
	Connected     bool          `json:"connected"`
	Region        string        `json:"region"`
	GameID        int64         `json:"gameId"`
	QueueID       int           `json:"queueId"`
	ScoutQueue    int           `json:"scoutQueue"`
	Frozen        bool          `json:"frozen"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Participants  []Participant `json:"participants"`
	Message       string        `json:"message,omitempty"`
	Review        *MatchReview  `json:"review,omitempty"`
}

// MatchReview is the latest finished match in memory. Participants contain the
// captured pre-match reports; Result contains only actual post-game statistics.
type MatchReview struct {
	ConnectionID uint64        `json:"connectionId"`
	GameID       int64         `json:"gameId"`
	Region       string        `json:"region"`
	SelfKey      string        `json:"selfKey"`
	QueueID      int           `json:"queueId"`
	ScoutQueue   int           `json:"scoutQueue"`
	CapturedAt   time.Time     `json:"capturedAt"`
	State        string        `json:"state"` // waiting, ready, partial, unavailable
	Participants []Participant `json:"participants"`
	Result       *MatchDetail  `json:"result,omitempty"`
}

func CalculateStats(matches []Match) Stats {
	var out Stats
	var kills, deaths, assists int
	for _, match := range matches {
		out.Games++
		if IsRemake(match) {
			out.Remakes++
			continue
		}
		if match.Win != nil {
			out.OutcomeGames++
			if *match.Win {
				out.Wins++
			} else {
				out.Losses++
			}
		}
		if match.Kills != nil && match.Deaths != nil && match.Assists != nil {
			out.KDAGames++
			kills += *match.Kills
			deaths += *match.Deaths
			assists += *match.Assists
		}
	}
	if out.OutcomeGames > 0 {
		value := float64(out.Wins) * 100 / float64(out.OutcomeGames)
		out.WinRate = &value
	}
	if out.KDAGames > 0 {
		denominator := deaths
		if denominator == 0 {
			denominator = 1
		}
		value := float64(kills+assists) / float64(denominator)
		out.KDA = &value
	}
	return out
}

func ApplyHistory(player *Participant, result HistoryResult) {
	now := time.Now()
	result.Meta.CheckedAt = &now
	if result.Meta.Service != nil && player.HistoryMeta.FetchedAt != nil && len(player.History) > 0 {
		player.HistoryMeta.Service = result.Meta.Service
		player.HistoryMeta.CheckedAt = &now
		player.Cached = true
		player.HistoryState = HistoryReady
		player.Error = ""
		return // A limited local fallback must not replace a verified full snapshot.
	}
	if result.Meta.FetchedAt == nil {
		now := time.Now()
		result.Meta.FetchedAt = &now
	}
	player.History, player.HistoryMeta = result.Matches, result.Meta
	player.Cached, player.Stale = false, false
	player.Stats = CalculateStats(result.Matches)
	player.Error = ""
	player.HistoryState = HistoryReady
	if len(result.Matches) == 0 {
		player.HistoryState = HistoryEmpty
	}
}
