package core

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

func groupToken(id *int64) string {
	if id == nil || *id <= 0 {
		return ""
	}
	return fmt.Sprint(*id)
}
func rosterPosition(values ...string) string {
	for _, v := range values {
		switch strings.ToUpper(v) {
		case "TOP":
			return "top"
		case "JUNGLE":
			return "jungle"
		case "MIDDLE", "MID":
			return "mid"
		case "BOTTOM", "ADC":
			return "adc"
		case "UTILITY", "SUPPORT":
			return "support"
		}
	}
	return ""
}

// AssignPremades describes explicit source groups only. Missing/unique IDs never
// imply solo queue. Direct LCU slot fields do not require revealing an identity.
// Scope by map side; numeric IDs may be reused by the other side.
func AssignPremades(players []Participant, source string) {
	groups := map[string][]int{}
	for i := range players {
		p := &players[i]
		p.PremadeGroup = ""
		p.PremadeSize = 0
		p.GroupSource = ""
		if (IsKnownPUUID(p.PUUID) || p.Key != "") && (p.Team == "ally" || p.Team == "enemy") && p.GroupToken != "" {
			p.GroupSource = source
			groups[p.Team+":"+p.GroupToken] = append(groups[p.Team+":"+p.GroupToken], i)
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ids := groups[k]
		if len(ids) < 2 || len(ids) > 5 {
			continue
		}
		for _, i := range ids {
			players[i].PremadeGroup = fmt.Sprintf("%s-%x", players[i].Team, sha256.Sum256([]byte(players[i].GroupToken)))
			players[i].PremadeSize = len(ids)
			players[i].GroupSource = source
		}
	}
}
