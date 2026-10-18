package core

import "strings"

// NameFields accepts names provided with a verified local roster/profile.
// It never establishes identity; callers must check the player's PUUID first.
type NameFields struct {
	GameName       string `json:"gameName"`
	TagLine        string `json:"tagLine"`
	RiotIDGameName string `json:"riotIdGameName"`
	RiotIDTagLine  string `json:"riotIdTagLine"`
	SummonerName   string `json:"summonerName"`
	DisplayName    string `json:"displayName"`
}

func (n NameFields) Name() string {
	name, tag := n.GameName, n.TagLine
	if name == "" && n.RiotIDGameName != "" {
		name, tag = n.RiotIDGameName, n.RiotIDTagLine
	}
	if name == "" {
		name = n.DisplayName
	}
	if name == "" {
		name = n.SummonerName
	}
	name, tag = strings.TrimSpace(name), strings.TrimSpace(tag)
	if name == "" {
		return ""
	}
	if tag != "" && !strings.Contains(name, "#") {
		name += "#" + tag
	}
	if IsPlaceholderName(name) {
		return ""
	}
	if strings.Contains(name, "#") {
		if normalized, err := NormalizeRiotID(name); err == nil {
			return normalized
		}
		return ""
	}
	return name
}
