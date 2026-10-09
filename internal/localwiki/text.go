package localwiki

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	maxQueryRunes = 200
	maxQueryWords = 16
)

// foldText lowercases s and removes all combining marks (NFD, drop Mn, Mc
// and Me, NFC) like libzim's text normalisation, so "Münster" and "munster"
// compare equal.
func foldText(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.M)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}

// normalizeQuery collapses whitespace and enforces the query limits
// (at most 200 characters and 16 words).
func normalizeQuery(query string) (string, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return "", ErrQueryEmpty
	}
	if len(words) > maxQueryWords {
		return "", ErrQueryTooLong
	}
	normalized := strings.Join(words, " ")
	if utf8.RuneCountInString(normalized) > maxQueryRunes {
		return "", ErrQueryTooLong
	}
	return normalized, nil
}

// isWordRune reports whether r belongs to a word for matching purposes.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isCJK reports whether r is written without spaces between words.
func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// foldedWords splits folded text into words.
func foldedWords(s string) []string {
	return strings.FieldsFunc(foldText(s), func(r rune) bool { return !isWordRune(r) })
}

// matchWords returns the folded query words used to find a snippet sentence.
// Single-letter words are dropped unless they are CJK characters.
func matchWords(query string) []string {
	var out []string
	for _, w := range foldedWords(query) {
		first, _ := utf8.DecodeRuneInString(w)
		if utf8.RuneCountInString(w) < 2 && !isCJK(first) {
			continue
		}
		out = append(out, w)
	}
	return out
}

// containsQueryWord reports whether text contains one of the folded query
// words: as a word prefix for spaced scripts, as a substring for CJK.
func containsQueryWord(text string, words []string) bool {
	if len(words) == 0 {
		return false
	}
	folded := foldText(text)
	tokens := strings.FieldsFunc(folded, func(r rune) bool { return !isWordRune(r) })
	for _, w := range words {
		first, _ := utf8.DecodeRuneInString(w)
		if isCJK(first) {
			if strings.Contains(folded, w) {
				return true
			}
			continue
		}
		for _, tok := range tokens {
			if strings.HasPrefix(tok, w) {
				return true
			}
		}
	}
	return false
}

// titleKey reduces a title to a comparison key: folded, underscores as
// spaces, a trailing parenthetical qualifier removed, punctuation collapsed.
// "Berlin (Stadt)" and "berlin" share the key "berlin".
func titleKey(title string) string {
	s := strings.TrimSpace(strings.ReplaceAll(title, "_", " "))
	if strings.HasSuffix(s, ")") {
		if open := strings.LastIndex(s, " ("); open > 0 {
			s = s[:open]
		}
	}
	return strings.Join(foldedWords(s), " ")
}

// pathCandidates returns the ZIM paths a typed title most likely has:
// spaces as underscores, as typed and with an upper-case first letter.
func pathCandidates(title string) []string {
	base := strings.ReplaceAll(strings.Join(strings.Fields(title), " "), " ", "_")
	if base == "" {
		return nil
	}
	out := []string{base}
	if upper := upperFirst(base); upper != base {
		out = append(out, upper)
	}
	return out
}

// upperFirst upper-cases the first rune of s.
func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || unicode.IsUpper(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// truncateRunes shortens s to at most max runes, preferring a word boundary,
// and marks a cut with "…".
func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)[:max-1]
	cut := len(r)
	for i := len(r) - 1; i >= len(r)/2; i-- {
		if unicode.IsSpace(r[i]) {
			cut = i
			break
		}
	}
	return strings.TrimRightFunc(string(r[:cut]), func(c rune) bool {
		return unicode.IsSpace(c) || c == ',' || c == ';' || c == ':'
	}) + "…"
}

// collapseSpace replaces runs of white space with one space and trims.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
