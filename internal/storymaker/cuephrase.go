package storymaker

import (
	"unicode"
	"unicode/utf8"
)

// cueToken is one unit of a narration for phrase matching: a folded word, or one CJK character. start and end are byte
// offsets in the original text, which may hold {n} markers.
type cueToken struct {
	text       string
	start, end int
}

// cueTokens splits text into the units that a cue phrase is matched on. {n} markers are skipped, case is folded,
// punctuation and whitespace only separate. Letters, digits and marks make a word; a character of a script written
// without spaces (ucdCJKWord) is a unit of its own, so a phrase may start or end inside such a run.
func cueTokens(text string) []cueToken {
	skip := map[int]int{} // start of a marker -> its end
	for _, m := range narrationMarker.FindAllStringIndex(text, -1) {
		skip[m[0]] = m[1]
	}
	var out []cueToken
	var word []rune
	wordStart, wordEnd := 0, 0
	flush := func() {
		if len(word) > 0 {
			out = append(out, cueToken{text: string(word), start: wordStart, end: wordEnd})
			word = word[:0]
		}
	}
	for i := 0; i < len(text); {
		if end, ok := skip[i]; ok {
			i = end
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r):
			flush()
		case inUCDTable(ucdCJKWord, r):
			flush()
			out = append(out, cueToken{text: string(foldRune(r)), start: i, end: i + size})
		default:
			if len(word) == 0 {
				wordStart = i
			}
			word = append(word, foldRune(r))
			wordEnd = i + size
		}
		i += size
	}
	flush()
	return out
}

// foldRune is the smallest member of the case-folding orbit of r.
func foldRune(r rune) rune {
	least := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < least {
			least = f
		}
	}
	return least
}

// cuePhraseMatches returns the token index of every occurrence of phrase in tokens.
func cuePhraseMatches(tokens, phrase []cueToken) []int {
	var at []int
	for i := 0; len(phrase) > 0 && i+len(phrase) <= len(tokens); i++ {
		hit := true
		for j := range phrase {
			if tokens[i+j].text != phrase[j].text {
				hit = false
				break
			}
		}
		if hit {
			at = append(at, i)
		}
	}
	return at
}
