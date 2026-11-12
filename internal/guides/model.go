// Package guides serves public OP.GG hero guidance independently of player data.
package guides

import (
	"embed"
	"encoding/json"
	"fmt"

	"strconv"
	"strings"
	"time"

	"leaguepingo/internal/core"
)

// Derived from the bundled Riot Data Dragon portrait manifest; no player data.
//
//go:embed champions.json
var championFiles embed.FS
var champions = func() map[int]string {
	b, _ := championFiles.ReadFile("champions.json")
	m := map[int]string{}
	_ = json.Unmarshal(b, &m)
	return m
}()

type Query struct {
	Mode     string `json:"mode"`
	Champion int    `json:"champion"`
	Position string `json:"position"`
	Opponent int    `json:"opponent"`
	Patch    string `json:"patch"`
}
type Build struct {
	IDs     []int    `json:"ids"`
	Names   []string `json:"names"`
	Games   int      `json:"games"`
	WinRate *float64 `json:"winRate"`
}
type BuildSection struct {
	Label   string  `json:"label"`
	Options []Build `json:"options"`
}
type Tip struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}
type Augment struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tier        int      `json:"tier"`
	Rarity      int      `json:"rarity"`
	Performance *float64 `json:"performance"`
	Popularity  *float64 `json:"popularity"`
	WinRate     *float64 `json:"winRate,omitempty"`
	Games       int      `json:"games,omitempty"`
}
type Guide struct {
	Query           Query          `json:"query"`
	Version         string         `json:"version"`
	FetchedAt       time.Time      `json:"fetchedAt"`
	SourceURL       string         `json:"sourceUrl"`
	Builds          []BuildSection `json:"builds"`
	Runes           []int          `json:"runes"`
	RuneNames       []string       `json:"runeNames"`
	Skills          []string       `json:"skills"`
	Tips            []Tip          `json:"tips"`
	Augments        []Augment      `json:"augments,omitempty"`
	AugmentsOmitted int            `json:"augmentsOmitted,omitempty"`
	Notice          string         `json:"notice,omitempty"`
	ReferenceNote   string         `json:"referenceNote,omitempty"`
}
type State struct {
	Revision         uint64 `json:"revision"`
	Scope            string `json:"scope"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	Query            Query  `json:"query"`
	ModeLabel        string `json:"modeLabel"`
	Reference        bool   `json:"reference"`
	PositionExplicit bool   `json:"positionExplicit"`
	Base             *Guide `json:"base"`
	Matchup          *Guide `json:"matchup"`
	Stale            bool   `json:"stale"`
	VersionMismatch  bool   `json:"versionMismatch"`
	SourceURL        string `json:"sourceUrl,omitempty"`
	Candidates       []int  `json:"candidates,omitempty"`
}

func mode(s core.Snapshot) (string, string, bool) {
	if spec, ok := core.ScoutModeFor(s.QueueID); ok {
		if !spec.Matches(s.MapID, s.GameMode) {
			return "", "当前模式暂无已核验攻略", false
		}
		reference := spec.Guide == "ranked" && !spec.Ranked
		if reference {
			return spec.Guide, "峡谷参考", true
		}
		return spec.Guide, spec.Label, false
	}
	if s.QueueID == 1700 || s.QueueID == 1710 {
		if s.MapID != 0 && s.MapID != 30 || s.GameMode != "" && !strings.EqualFold(s.GameMode, "CHERRY") {
			return "", "当前模式暂无已核验攻略", false
		}
		return "arena", "斗魂竞技场", false
	}
	return "", "当前模式暂无已核验攻略", false
}

func scope(s core.Snapshot) string {
	return fmt.Sprintf("%d:%s:%d", s.ConnectionID, s.SelfKey, s.GameID)
}
func patch(v string) string {
	p := strings.Split(v, ".")
	if len(p) < 2 {
		return ""
	}
	major, e1 := strconv.ParseUint(p[0], 10, 16)
	minor, e2 := strconv.ParseUint(p[1], 10, 16)
	if e1 != nil || e2 != nil {
		return ""
	}
	return fmt.Sprintf("%d.%d", major, minor)
}
func validPosition(p string) bool {
	return p == "top" || p == "jungle" || p == "mid" || p == "adc" || p == "support"
}
func validQuery(q Query) bool {
	return champions[q.Champion] != "" && (q.Opponent == 0 || champions[q.Opponent] != "") && validPosition(q.Position) && (q.Mode == "ranked" || q.Mode == "flex" || q.Mode == "aram" || q.Mode == "aram-mayhem" || q.Mode == "arena" || q.Mode == "urf" || q.Mode == "nexus-blitz") && (q.Opponent == 0 || q.Mode == "ranked")
}

func noLane(mode string) bool {
	return mode == "aram" || mode == "aram-mayhem" || mode == "arena" || mode == "urf" || mode == "nexus-blitz"
}
func pageMode(mode string) bool {
	return mode == "aram-mayhem" || mode == "arena" || mode == "urf" || mode == "nexus-blitz"
}
