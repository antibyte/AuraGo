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
// match. A term written with a leading "=" matches the folded words exactly
// instead of their stems, for a word whose stem is shared with an unrelated
// word ("malen" and the English "male").
type toolIntentTerm struct {
	words []string
	stems []string
	exact bool
}

// toolIntentHead is a German noun that matches the word itself, its plural
// and compound words that start or end with it: "datei" is found in Datei,
// Dateien, Dateiname and Konfigurationsdatei. A head is written as
//
//	"datei"            at either end of a compound
//	"^kommando"        only at the start (Kommandozeile, not Einsatzkommando)
//	"$verbrauch:cpu"   only at the end, after one of the listed modifiers
//	"verzeichnis:unter,arbeits"  at the start, and at the end only after one
//	                   of the listed modifiers (Unterverzeichnis, not
//	                   Inhaltsverzeichnis)
type toolIntentHead struct {
	word, stem string
	start, end bool
	// tails, when set, lists the only prefixes a compound that ends with the
	// head may have.
	tails map[string]bool
}

// parseToolIntentHead reads the notation of toolIntentHead.
func parseToolIntentHead(spec string) toolIntentHead {
	head := toolIntentHead{start: true, end: true}
	if rest, ok := strings.CutPrefix(spec, "^"); ok {
		spec, head.end = rest, false
	} else if rest, ok := strings.CutPrefix(spec, "$"); ok {
		spec, head.start = rest, false
	}
	if word, tails, ok := strings.Cut(spec, ":"); ok {
		spec = word
		head.tails = make(map[string]bool)
		for _, tail := range strings.Split(tails, ",") {
			head.tails[foldToolSearchText(strings.TrimSpace(tail))] = true
		}
	}
	head.word = foldToolSearchText(spec)
	head.stem = stemToolSearchWord(head.word)
	return head
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
		term, ok := compileToolIntentTerm(phrase)
		if !ok {
			continue
		}
		table.first[term.stems[0]] = append(table.first[term.stems[0]], len(table.terms))
		table.terms = append(table.terms, term)
	}
	for _, head := range heads {
		table.heads = append(table.heads, parseToolIntentHead(head))
	}
	return table
}

// compileToolIntentTerm folds and stems one phrase.
func compileToolIntentTerm(phrase string) (toolIntentTerm, bool) {
	exact := strings.HasPrefix(phrase, "=")
	words := toolSearchWords(foldToolSearchText(strings.TrimPrefix(phrase, "=")), false)
	if len(words) == 0 {
		return toolIntentTerm{}, false
	}
	term := toolIntentTerm{words: words, stems: make([]string, len(words)), exact: exact}
	for i, word := range words {
		term.stems[i] = stemToolSearchWord(word)
	}
	return term, true
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

// matchesUnless is matches, except that a term does not count when the word in
// front of it is blocked: "docker image" is no image request.
func (t *toolIntentTerms) matchesUnless(text toolIntentText, blocked func(previous string) bool) bool {
	if t == nil {
		return false
	}
	for i := range text.words {
		for _, index := range t.first[text.stems[i]] {
			if t.terms[index].matchesAt(text, i) && (i == 0 || !blocked(text.words[i-1])) {
				return true
			}
		}
	}
	for _, head := range t.heads {
		if text.hasHead(head) {
			return true
		}
	}
	return false
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
		if term.exact && text.words[i+j] != term.words[j] {
			return false
		}
		if _, stop := toolSearchStopwords[text.words[i+j]]; stop && text.words[i+j] != term.words[j] {
			return false
		}
	}
	return true
}

// occursIn reports whether the term starts at any word of the message.
func (term toolIntentTerm) occursIn(text toolIntentText) bool {
	for i := range text.words {
		if term.matchesAt(text, i) {
			return true
		}
	}
	return false
}

// hasWord reports a whole word of the message that has the stem of the term's
// word, with the stopword rule of matchesAt.
func (text toolIntentText) hasWord(word, stem string) bool {
	for i := range text.words {
		if text.stems[i] != stem {
			continue
		}
		if _, stop := toolSearchStopwords[text.words[i]]; stop && text.words[i] != word {
			continue
		}
		return true
	}
	return false
}

// toolHeadInflections are the endings a plural or genitive adds to a head at
// the end of a compound: Unterverzeichnisse, Heimnetzwerke, Konfigurationsdateien.
// Not "-er": Verbraucher is a consumer, not a Verbrauch.
var toolHeadInflections = []string{"", "e", "en", "n", "s", "se", "sen"}

// endsWith reports a word that ends with the head, possibly inflected, after
// a prefix its notation allows.
func (head toolIntentHead) endsWith(word string) bool {
	for _, inflection := range toolHeadInflections {
		if rest, ok := strings.CutSuffix(word, head.word+inflection); ok && (head.tails == nil || rest == "" || head.tails[rest]) {
			return true
		}
	}
	if rest, ok := strings.CutSuffix(word, head.stem); ok && (head.tails == nil || rest == "" || head.tails[rest]) {
		return true
	}
	return false
}

// hasHead reports a word that starts or ends with the head as its notation
// allows; the stem covers a plural ("Konfigurationsdateien").
func (text toolIntentText) hasHead(head toolIntentHead) bool {
	if utf8.RuneCountInString(head.word) < toolIntentMinHeadRunes {
		return false
	}
	for i, word := range text.words {
		if head.start && strings.HasPrefix(word, head.word) {
			return true
		}
		if !head.end {
			continue
		}
		if head.endsWith(word) || head.endsWith(text.stems[i]) {
			return true
		}
	}
	return false
}
