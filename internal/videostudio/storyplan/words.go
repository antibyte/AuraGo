package storyplan

import (
	"math"
	"strings"
	"unicode"
)

// Narration speaking rates in words per second of use_seconds (spec §8), and the words every scene keeps free.
const (
	wordsPerSecondGerman  = 2.2
	wordsPerSecondEnglish = 2.5
	wordsPerSecondOther   = 2.3
	wordBudgetReserve     = 2
)

// WordBudget is the number of narration words that fit into useSeconds: floor(rate × useSeconds) − 2, at least 0. The
// rate is 2.2 words per second for German, 2.5 for English and 2.3 for every other language. The language is matched
// by its first two letters, case-insensitive ("de-AT" counts as German).
func WordBudget(language string, useSeconds float64) int {
	if useSeconds <= 0 {
		return 0
	}
	rate := wordsPerSecondOther
	switch languagePrefix(language) {
	case "de":
		rate = wordsPerSecondGerman
	case "en":
		rate = wordsPerSecondEnglish
	}
	// The small epsilon keeps products like 2.2 × 5 = 11.000000000000002 and 2.3 × 10 on the whole number.
	return max(int(math.Floor(rate*useSeconds+1e-9))-wordBudgetReserve, 0)
}

// CountWords counts the words of a narration. The text is split on Unicode white space, and a piece counts as a word
// when it holds a letter or a digit (a lone dash does not). Chinese and Japanese are written without spaces, so their
// characters (Han, Hiragana, Katakana) are counted on their own: every two characters count as one word, rounded up.
// The count does not depend on the plan language; the script decides.
func CountWords(text string) int {
	words, cjk := 0, 0
	for _, field := range strings.Fields(text) {
		hasWord := false
		for _, r := range field {
			switch {
			case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana):
				cjk++
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				hasWord = true
			}
		}
		if hasWord {
			words++
		}
	}
	return words + (cjk+1)/2
}

func languagePrefix(language string) string {
	l := strings.ToLower(strings.TrimSpace(language))
	if len(l) > 2 {
		l = l[:2]
	}
	return l
}
