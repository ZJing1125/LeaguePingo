// Package guides serves public OP.GG hero guidance independently of player data.
package guides

import (
	"embed"
	"encoding/json"
	"fmt"

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
	if s.QueueID == 2400 {
		if (s.MapID != 0 && s.MapID != 12) || (s.GameMode != "" && !strings.EqualFold(s.GameMode, "ARAM") && !strings.EqualFold(s.GameMode, "ARAM_MAYHEM")) {
			return "", "当前模式暂无已核验攻略", false
		}
		return "aram-mayhem", "海克斯大乱斗", false
	}
	if (s.QueueID == 420 || s.QueueID == 440 || s.QueueID == 400 || s.QueueID == 430 || s.QueueID == 490) && ((s.MapID != 0 && s.MapID != 11) || (s.GameMode != "" && !strings.EqualFold(s.GameMode, "CLASSIC"))) {
		return "", "当前模式暂无已核验攻略", false
	}
	if s.QueueID == 450 && ((s.MapID != 0 && s.MapID != 12) || (s.GameMode != "" && !strings.EqualFold(s.GameMode, "ARAM"))) {
		return "", "当前模式暂无已核验攻略", false
	}
	// Queue is authoritative. Do not treat every map 12/ARAM variant as classic ARAM.
	switch s.QueueID {
	case 420:
		return "ranked", "单双排", false
	case 440:
		return "flex", "灵活排位", false
	case 450:
		return "aram", "极地大乱斗", false
	case 400, 430, 490:
		return "ranked", "峡谷参考", true
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
	return p[0] + "." + p[1]
}
func validPosition(p string) bool {
	return p == "top" || p == "jungle" || p == "mid" || p == "adc" || p == "support"
}
func validQuery(q Query) bool {
	return champions[q.Champion] != "" && (q.Opponent == 0 || champions[q.Opponent] != "") && validPosition(q.Position) && (q.Mode == "ranked" || q.Mode == "flex" || q.Mode == "aram" || q.Mode == "aram-mayhem") && (q.Opponent == 0 || q.Mode == "ranked")
}
