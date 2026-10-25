package agent

import (
	"strings"
	"unicode/utf8"
)

// toolIntentText is a user message split into the folded words and stems that
// the intent matchers compare. Matching is by whole words, never by substring:
// "profile" is not "file", "report" is not "port" and "Programm" is not "ram".
// The folding and the stemmer are the ones of the discover_tools search
// (tool_search_text.go), so a plural, a verb form or an umlaut spelled "ae"
// meets its term.
type toolIntentText struct {
	words []string
	stems []string
}

// newToolIntentText folds and stems a message. Like the ranker it looks at the
// first and last 4 KB of a very long message only.
func newToolIntentText(text string) toolIntentText {
	words := toolSearchWords(foldToolSearchText(boundToolSearchQuery(text)), false)
	stems := make([]string, len(words))
	for i, word := range words {
		stems[i] = stemToolSearchWord(word)
	}
	return toolIntentText{words: words, stems: stems}
}

// toolIntentTerm is one compiled phrase: consecutive words that all have to
// match.
type toolIntentTerm struct {
	words []string
	stems []string
}

// toolIntentHead is a German noun that matches the word itself, its plural
// and any compound word that starts or ends with it: "datei" is found in
// Datei, Dateien, Dateiname and Konfigurationsdatei.
type toolIntentHead struct {
	word, stem string
}

// toolIntentTerms is a compiled term table: whole-word phrases plus German
// compound heads.
type toolIntentTerms struct {
	terms []toolIntentTerm
	// first indexes the terms by the stem of their first word.
	first map[string][]int
	heads []toolIntentHead
}

// toolIntentMinHeadRunes is the shortest compound head. A head is a German
// word that is no English word either, so "bild", "port" or "mail" never match
// inside unrelated words.
const toolIntentMinHeadRunes = 5

// newToolIntentTerms compiles phrases (any spelling the folding accepts) and
// compound heads. A phrase and a head must not name the same word, or a
// message counts it twice.
func newToolIntentTerms(phrases, heads []string) *toolIntentTerms {
	table := &toolIntentTerms{first: make(map[string][]int, len(phrases))}
	for _, phrase := range phrases {
		words := toolSearchWords(foldToolSearchText(phrase), false)
		if len(words) == 0 {
			continue
		}
		term := toolIntentTerm{words: words, stems: make([]string, len(words))}
		for i, word := range words {
			term.stems[i] = stemToolSearchWord(word)
		}
		table.first[term.stems[0]] = append(table.first[term.stems[0]], len(table.terms))
		table.terms = append(table.terms, term)
	}
	for _, head := range heads {
		word := foldToolSearchText(head)
		table.heads = append(table.heads, toolIntentHead{word: word, stem: stemToolSearchWord(word)})
	}
	return table
}

// count returns how many different terms and compound heads occur in the text.
func (t *toolIntentTerms) count(text toolIntentText) int {
	if t == nil || len(text.words) == 0 {
		return 0
	}
	matched := 0
	seen := make(map[int]bool, 8)
	for i := range text.words {
		for _, index := range t.first[text.stems[i]] {
			if !seen[index] && t.terms[index].matchesAt(text, i) {
				seen[index] = true
				matched++
			}
		}
	}
	for _, head := range t.heads {
		if text.hasHead(head) {
			matched++
		}
	}
	return matched
}

// matches reports whether any term or head occurs in the text.
func (t *toolIntentTerms) matches(text toolIntentText) bool {
	return t.count(text) > 0
}

// matchesAt reports whether the term starts at word i. A stem alone is not
// enough for a stopword of the message: "note" and "not" share the stem "not",
// but "not" never means a note.
func (term toolIntentTerm) matchesAt(text toolIntentText, i int) bool {
	if i+len(term.stems) > len(text.stems) {
		return false
	}
	for j, stem := range term.stems {
		if text.stems[i+j] != stem {
			return false
		}
		if _, stop := toolSearchStopwords[text.words[i+j]]; stop && text.words[i+j] != term.words[j] {
			return false
		}
	}
	return true
}

// hasHead reports a word that starts or ends with the head; the stem covers a
// plural ("Konfigurationsdateien").
func (text toolIntentText) hasHead(head toolIntentHead) bool {
	if utf8.RuneCountInString(head.word) < toolIntentMinHeadRunes {
		return false
	}
	for i, word := range text.words {
		if strings.HasPrefix(word, head.word) || strings.HasSuffix(word, head.word) || strings.HasSuffix(text.stems[i], head.stem) {
			return true
		}
	}
	return false
}
