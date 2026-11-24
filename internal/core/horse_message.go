package core

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const HorseBriefHeader = "[LeaguePingo] 本局五马分析"
const HorseBriefMaxBytes = 1024
const HorseBriefMaxRunes = 256

func formatHorseBrief(rows []string, tags [][]string) []string {
	// This is the assistant's bounded payload budget. The actual Chinese client
	// acceptance limit still needs live verification.
	for count := 3; count >= 0; count-- {
		lines := []string{HorseBriefHeader}
		for i, row := range rows {
			selected := []string{}
			for _, tag := range tags[i] {
				if len(selected) == count {
					break
				}
				if utf8.RuneCountInString(tag) <= 24 && !strings.ContainsAny(tag, "\r\n") {
					selected = append(selected, tag)
				}
			}
			if len(selected) > 0 {
				row += " | " + strings.Join(selected, " | ")
			}
			lines = append(lines, row)
		}
		if ValidAnalysisBriefBody(AnalysisBriefBody(lines)) {
			return lines
		}
	}
	return nil
}

func ValidAnalysisBriefBody(body string) bool {
	if strings.TrimSpace(body) == "" || !utf8.ValidString(body) {
		return false
	}
	multiline := strings.HasPrefix(body, HorseBriefHeader+"\n")
	if multiline {
		if len(body) > HorseBriefMaxBytes || utf8.RuneCountInString(body) > HorseBriefMaxRunes || strings.Count(body, "\n") > 5 {
			return false
		}
	} else if len(body) > AnalysisBriefMaxBytes || strings.ContainsRune(body, '\n') {
		return false
	}
	for _, r := range body {
		if unicode.IsControl(r) && !(multiline && r == '\n') || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}

func horseBriefLabel(label string) string {
	icons := map[string]string{"千里马": "🦄", "上等马": "🐎", "中等马": "🐎", "下等马": "🐂", "纯牛马": "💩"}
	return icons[label] + label
}
