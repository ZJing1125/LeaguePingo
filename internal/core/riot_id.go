package core

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeRiotID removes clipboard presentation controls, not characters that
// can distinguish names (inner spaces, joiners, accents or variation selectors).
// Keep the equivalent input cleanup in frontend/src/presentation.ts in sync.
func NormalizeRiotID(value string) (string, error) {
	invalid := errors.New("请输入完整 ID，例如 玩家名#标签")
	if !utf8.ValidString(value) {
		return "", invalid
	}
	value = strings.Map(func(r rune) rune {
		if r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			return -1
		}
		return r
	}, value)
	if strings.Count(value, "#") != 1 {
		return "", invalid
	}
	name, tag, _ := strings.Cut(value, "#")
	trim := func(s string) string {
		return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff' })
	}
	name, tag = trim(name), trim(tag)
	if name == "" || tag == "" || strings.ContainsFunc(name+tag, unicode.IsControl) {
		return "", invalid
	}
	return name + "#" + tag, nil
}
