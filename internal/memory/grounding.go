package memory

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Automatic extraction and profile updates are written by a model that may
// have read a web page or a mail in the same turn. A quote that really occurs in
// the human message ties an item to human words, but not the item's own
// wording to them: DerivedFrom closes that gap by asking whether the item says
// what the human text says.

// groundingShare is the share of an item's distinctive words that its source
// must contain (three of five).
const (
	groundingShareNum = 3
	groundingShareDen = 5
)

// groundingStopWords are words too common to prove anything: the filler of a
// stored claim ("The user wants ...") and of a follow-up summary.
var groundingStopWords = map[string]bool{
	"that": true, "this": true, "with": true, "from": true, "have": true, "been": true, "were": true,
	"will": true, "would": true, "should": true, "could": true, "their": true, "there": true, "about": true,
	"which": true, "into": true, "also": true, "what": true, "when": true, "then": true, "than": true,
	"them": true, "they": true, "does": true, "doing": true, "your": true, "must": true, "need": true,
	"needs": true, "needed": true, "user": true, "users": true, "wants": true, "want": true, "please": true,
	"follow": true, "remind": true, "reminder": true, "task": true, "todo": true, "action": true, "item": true,
	"nutzer": true, "benutzer": true, "dass": true, "wird": true, "werden": true, "wurde": true, "haben": true,
	"sind": true, "einen": true, "einem": true, "einer": true, "eines": true, "nicht": true, "auch": true,
	"oder": true, "aber": true, "wenn": true, "dann": true, "sein": true, "seine": true, "seiner": true,
	"möchte": true, "soll": true, "sollen": true, "bitte": true, "erinnere": true,
}

// NormalizeGroundingText folds text for a comparison with human words: format
// characters dropped (the analyzers read the isolated view that strips them),
// case folded, whitespace runs collapsed to one space.
func NormalizeGroundingText(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// DerivedFrom reports whether text says what source says: at least three of
// every five of the distinctive words of text (four or more letters or a digit,
// no stop word) occur in source, a plural or an inflection counting as the
// same word. A text without a distinctive word is derived from anything; the
// caller's own checks (a verbatim quote, the directive screen) bound it.
func DerivedFrom(text, source string) bool {
	words := groundingWords(NormalizeGroundingText(text), true)
	if len(words) == 0 {
		return true
	}
	have := groundingWords(NormalizeGroundingText(source), false)
	covered := 0
	for _, word := range words {
		for _, candidate := range have {
			if sameGroundingWord(word, candidate) {
				covered++
				break
			}
		}
	}
	return covered*groundingShareDen >= len(words)*groundingShareNum
}

// groundingWords splits normalised text into words. Han, Hiragana and Katakana
// text has no spaces, so its runs become overlapping two-character words. With
// distinctive set, short words and stop words are dropped and duplicates
// merged.
func groundingWords(text string, distinctive bool) []string {
	var words []string
	seen := map[string]bool{}
	add := func(word string) {
		if distinctive {
			if groundingStopWords[word] || (utf8.RuneCountInString(word) < 4 && !strings.ContainsFunc(word, unicode.IsDigit) && !isGroundingCJK([]rune(word)[0])) {
				return
			}
			if seen[word] {
				return
			}
			seen[word] = true
		}
		words = append(words, word)
	}
	var word, cjk []rune
	flushWord := func() {
		if len(word) > 0 {
			add(string(word))
			word = word[:0]
		}
	}
	flushCJK := func() {
		switch {
		case len(cjk) == 1:
			add(string(cjk))
		case len(cjk) > 1:
			for i := 0; i+1 < len(cjk); i++ {
				add(string(cjk[i : i+2]))
			}
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case isGroundingCJK(r):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			word = append(word, r)
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return words
}

func isGroundingCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

// sameGroundingWord matches two words that are equal or differ only in an
// ending ("runs"/"run", "lives"/"live", "wohnt"/"wohne", "answered"/"answer"):
// the common prefix has at least three letters, the shorter word has at most one
// letter beyond it and the longer at most two ("the" is no stem of "theory").
// A word with a digit or CJK text must match exactly.
func sameGroundingWord(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	for _, word := range [][]rune{ra, rb} {
		for _, r := range word {
			if unicode.IsDigit(r) || isGroundingCJK(r) {
				return false
			}
		}
	}
	common := 0
	for common < len(ra) && common < len(rb) && ra[common] == rb[common] {
		common++
	}
	return common >= 3 && min(len(ra), len(rb))-common <= 1 && max(len(ra), len(rb))-common <= 2
}
