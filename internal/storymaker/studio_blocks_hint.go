package storymaker

import (
	"strings"
	"unicode"
)

// blockHintRule is one rule of SuggestedBlock: the first rule with a matching word or phrase decides.
type blockHintRule struct {
	block   string
	words   map[string][]string // by language base; a word of 5 letters or more also matches as a prefix
	phrases map[string][]string // by language base; matched on whole words
}

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
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	joined := " " + strings.Join(words, " ") + " "
	hasWord := func(rule string) bool {
		for _, w := range words {
			if w == rule || (len([]rune(rule)) >= 5 && strings.HasPrefix(w, rule)) {
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
	for _, w := range words {
		if len(w) >= 2 && strings.Trim(w, "0123456789") == "" {
			return "chart"
		}
	}
	return ""
}
