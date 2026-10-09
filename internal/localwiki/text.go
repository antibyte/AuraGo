package localwiki

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/zim/xapian"
)

const (
	maxQueryRunes = 200
	maxQueryWords = 16
	// maxQueryBytes bounds the input before it is split: 200 runes of up to
	// four bytes plus the separators of 16 words, with room for padding.
	maxQueryBytes = 4 * (maxQueryRunes + maxQueryWords)
)

// foldText lowercases s and removes all combining marks (Mn, Mc and Me)
// exactly like libzim does for titles, article text and queries, so that
// "Münster" and "munster" compare equal and the result agrees with the
// terms of the ZIM's search indexes (Greek final sigma included).
func foldText(s string) string {
	return xapian.Normalize(s)
}

// normalizeQuery collapses whitespace and enforces the query limits
// (at most 200 characters and 16 words).
func normalizeQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if len(query) > maxQueryBytes {
		return "", ErrQueryTooLong
	}
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
	if max < 1 {
		return ""
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	all := []rune(s)
	r := all[:max-1]
	cut := len(r)
	spaced := false
	for i := len(r) - 1; i >= len(r)/2; i-- {
		if unicode.IsSpace(r[i]) {
			cut, spaced = i, true
			break
		}
	}
	if !spaced {
		// A cut between two runes of one user-perceived character would leave
		// a dangling mark, joiner or half a flag.
		for cut > 0 && splitsCluster(all, cut) {
			cut--
		}
	}
	return strings.TrimRightFunc(string(all[:cut]), func(c rune) bool {
		return unicode.IsSpace(c) || c == ',' || c == ';' || c == ':'
	}) + "…"
}

// splitsCluster reports whether cutting before rs[i] (0 < i < len(rs))
// would separate runes of one grapheme cluster: a combining mark, variation
// selector or emoji modifier after its base, either side of a zero width
// joiner, or the two halves of a regional-indicator flag.
func splitsCluster(rs []rune, i int) bool {
	cur, prev := rs[i], rs[i-1]
	if cur == zeroWidthJoiner || cur == variationSelector16 || unicode.Is(unicode.M, cur) || isEmojiModifier(cur) || prev == zeroWidthJoiner {
		return true
	}
	if isRegionalIndicator(cur) && isRegionalIndicator(prev) {
		n := 0
		for j := i - 1; j >= 0 && isRegionalIndicator(rs[j]); j-- {
			n++
		}
		return n%2 == 1
	}
	return false
}

const (
	zeroWidthJoiner     rune = 0x200D
	variationSelector16 rune = 0xFE0F
)

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isEmojiModifier(r rune) bool { return r >= 0x1F3FB && r <= 0x1F3FF }

// collapseSpace replaces runs of white space with one space and trims.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
