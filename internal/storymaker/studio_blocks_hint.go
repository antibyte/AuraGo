package storymaker

import (
	"strings"
	"unicode"
)

// blockHintRule is one rule of SuggestedBlock: the first rule with a matching word or phrase decides.
type blockHintRule struct {
	block   string
	words   map[string][]string // by language base; a word matches exactly unless it is listed in blockHintStems
	phrases map[string][]string // by language base; matched on whole words
}

// blockHintStems are the rule words that also match as a prefix (inflections and compounds); every other word is exact.
var blockHintStems = map[string]bool{"prozent": true, "percent": true, "kreislauf": true, "schichten": true}

var blockHintRules = []blockHintRule{
	{block: "chart", words: map[string][]string{"de": {"prozent"}, "en": {"percent"}}},
	{block: "compare", words: map[string][]string{"de": {"statt", "anstatt"}, "en": {"instead", "versus"}}, phrases: map[string][]string{"de": {"im gegensatz"}}},
	{block: "cycle", words: map[string][]string{"de": {"kreislauf", "schleife"}, "en": {"loop", "cycle"}}},
	{block: "stack", words: map[string][]string{"de": {"schichten"}, "en": {"layers"}}, phrases: map[string][]string{"de": {"besteht aus"}, "en": {"consists of"}}},
	{block: "flow", words: map[string][]string{"de": {"zuerst", "dann", "danach"}, "en": {"first", "then"}}},
}

// SuggestedBlock names the explainer block that the narration of a scene points to: "flow", "chart", "cycle", "stack" or
// "compare", or "" when it points to none. It reads German and English narration only. The first matching rule wins:
// a percent sign or the word percent, then compare, cycle, stack, flow, and last a number of two or more digits.
func SuggestedBlock(narration, language string) string {
	lang := LanguageBase(language)
	if lang != "de" && lang != "en" {
		return ""
	}
	text := strings.ToLower(StripMarkers(narration))
	// A hyphen inside a word keeps it one word, so "first-hand" is not "first".
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' })
	for i, w := range words {
		words[i] = strings.Trim(w, "-")
	}
	joined := " " + strings.Join(words, " ") + " "
	hasWord := func(rule string) bool {
		if rule == "statt" {
			return hasCompareStatt(text)
		}
		for _, w := range words {
			if w == rule || (blockHintStems[rule] && strings.HasPrefix(w, rule)) {
				return true
			}
		}
		return false
	}
	for _, rule := range blockHintRules {
		if rule.block == "chart" && strings.Contains(text, "%") {
			return rule.block
		}
		for _, w := range rule.words[lang] {
			if hasWord(w) {
				return rule.block
			}
		}
		for _, p := range rule.phrases[lang] {
			if strings.Contains(joined, " "+p+" ") {
				return rule.block
			}
		}
	}
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) >= 2 && strings.Trim(w, "0123456789") == "" && !isYearLike(w) {
			return "chart"
		}
	}
	return ""
}

// isYearLike reports a four-digit number from 1900 to 2099, which names a year and not a quantity.
func isYearLike(w string) bool {
	return len(w) == 4 && (strings.HasPrefix(w, "19") || strings.HasPrefix(w, "20"))
}

// hasCompareStatt reports the German "statt" as a comparison: in a clause that does not end with it and has no form of
// "finden" ("Das Treffen findet am Freitag statt" is an event, "Züge statt Autos" a comparison).
func hasCompareStatt(text string) bool {
	clauses := strings.FieldsFunc(text, func(r rune) bool { return strings.ContainsRune(".!?;:,\n()", r) })
	for _, clause := range clauses {
		ws := strings.FieldsFunc(clause, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' })
		finden := false
		for _, w := range ws {
			if strings.HasPrefix(w, "find") || strings.HasPrefix(w, "fand") || strings.HasPrefix(w, "gefund") {
				finden = true
			}
		}
		for i, w := range ws {
			if w == "statt" && i < len(ws)-1 && !finden {
				return true
			}
		}
	}
	return false
}
