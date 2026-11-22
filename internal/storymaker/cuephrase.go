package storymaker

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

// cuePhraseMax is the longest cue_phrase in characters.
const cuePhraseMax = 60

// cueMarkerMax is the highest marker number the validator accepts (cuePattern allows three digits).
const cueMarkerMax = 999

// cuePhraseRefusal is a refused cue_phrase; Code is cue_phrase_not_found, cue_phrase_conflict or invalid_value.
type cuePhraseRefusal struct{ Code, Message, Next string }

// cueTrailingMarker finds a marker that ends a text.
var cueTrailingMarker = regexp.MustCompile(`\{(\d{1,3})\}$`)

// resolveCuePhrases rewrites fields in place: every beat with cue_phrase gets a {n} marker in fields["narration"] and
// cue: n, and loses cue_phrase. notes say when a phrase occurred more than once.
func resolveCuePhrases(fields map[string]any) (notes []string, refusal *cuePhraseRefusal) {
	beats, _ := fields["beats"].([]any)
	if !slices.ContainsFunc(beats, func(b any) bool { m, ok := b.(map[string]any); return ok && m["cue_phrase"] != nil }) {
		return nil, nil
	}
	narration, _ := fields["narration"].(string)
	taken := map[int]bool{}
	for _, m := range narrationMarker.FindAllStringSubmatch(narration, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			taken[n] = true
		}
	}
	for _, b := range beats {
		if m, ok := b.(map[string]any); ok {
			taken[cueNumber(m["cue"])] = true
		}
	}
	out := make([]any, len(beats))
	for i, b := range beats {
		m, ok := b.(map[string]any)
		if !ok || m["cue_phrase"] == nil {
			out[i] = b
			continue
		}
		phrase, _ := m["cue_phrase"].(string)
		tokens := cueTokens(narration)
		hits := cuePhraseMatches(tokens, cueTokens(phrase))
		if len(hits) == 0 {
			return nil, &cuePhraseRefusal{Code: "cue_phrase_not_found",
				Message: fmt.Sprintf("The phrase %q is not in the narration of this scene.", phrase),
				Next:    "Send a cue_phrase that stands in the narration."}
		}
		if len(hits) > 1 {
			notes = append(notes, fmt.Sprintf("The phrase %q occurs %d times in the narration; the first one is cued.", phrase, len(hits)))
		}
		at := tokens[hits[0]].start
		n := 0
		if loc := cueTrailingMarker.FindStringSubmatchIndex(strings.TrimRightFunc(narration[:at], IsJSSpace)); loc != nil {
			n, _ = strconv.Atoi(strings.TrimRightFunc(narration[:at], IsJSSpace)[loc[2]:loc[3]])
		}
		if n == 0 {
			for n = 1; n <= cueMarkerMax && taken[n]; n++ {
			}
			if n > cueMarkerMax {
				return nil, &cuePhraseRefusal{Code: "invalid_value", Message: "The narration has no free marker number left.",
					Next: "Remove markers from the narration or use fewer cue phrases."}
			}
			taken[n] = true
			narration = narration[:at] + "{" + strconv.Itoa(n) + "}" + narration[at:]
		}
		clone := maps.Clone(m)
		delete(clone, "cue_phrase")
		clone["cue"] = n
		out[i] = clone
	}
	fields["narration"] = narration
	fields["beats"] = out
	return notes, nil
}

// cueNumber reads a decoded JSON number; anything else is 0.
func cueNumber(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
