package storyplan

import "testing"

func TestWordBudget(t *testing.T) {
	for _, tc := range []struct {
		language string
		seconds  float64
		want     int
	}{
		{"de", 5, 9},       // floor(2.2 × 5) − 2
		{"DE-at", 5, 9},    // first two letters, any case
		{"en", 4, 8},       // floor(2.5 × 4) − 2
		{"en-US", 9.5, 21}, // floor(23.75) − 2
		{"fr", 10, 21},     // other languages: floor(2.3 × 10) − 2
		{"", 10, 21},
		{"en", 0.5, 0}, // never below 0
		{"de", 0, 0},
		{"de", -3, 0},
	} {
		if got := WordBudget(tc.language, tc.seconds); got != tc.want {
			t.Errorf("WordBudget(%q, %v) = %d, want %d", tc.language, tc.seconds, got, tc.want)
		}
	}
}

func TestCountWords(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"", 0},
		{"Der Fischer fährt hinaus.", 4},
		{"  Hallo — Welt  ", 2}, // a lone dash is not a word
		{"渔夫出海了", 3},            // five Han characters: 5 ÷ 2 rounded up
		{"漁師が海へ出る", 4},          // seven Han and Hiragana characters
		{"AuraGo 很好", 2},
	} {
		if got := CountWords(tc.text); got != tc.want {
			t.Errorf("CountWords(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}
