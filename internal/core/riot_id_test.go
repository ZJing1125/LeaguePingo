package core

import "testing"

func TestNormalizeRiotID(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"\u2066测试 + 玩家\u2069#\u2066TEST\u2069", "测试 + 玩家#TEST"},
		{" \u061c\u200e\u202aPlayer\u202c\u200f # \u2067TEST\u2069 ", "Player#TEST"},
		{"\u2068Player\u2069#\u2066TEST\u2069", "Player#TEST"},
		{"\ufeff Player\u200b#\u200bTEST\ufeff", "Player#TEST"},
		{"A  B+e\u0301\u200c\u200d\ufe0f#Tag", "A  B+e\u0301\u200c\u200d\ufe0f#Tag"},
		{"A\u200bB#Tag", "A\u200bB#Tag"},
	} {
		got, err := NormalizeRiotID(tt.input)
		if err != nil || got != tt.want {
			t.Fatalf("normalization mismatch: got=%q err=%v", got, err)
		}
		if again, err := NormalizeRiotID(got); err != nil || again != got {
			t.Fatal("normalization not idempotent")
		}
	}
	for _, value := range []string{"", "Player", "Player#", "#TEST", "\u2066\u2069#TEST", "Player#\ufeff\u200b", "Player#TEST#extra", "Play\n er#TEST", "Player#T\x00EST", string([]byte{0xff}) + "#TEST"} {
		if _, err := NormalizeRiotID(value); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
}
