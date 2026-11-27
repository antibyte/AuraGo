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
	for _, m := range narrationAnyMarker.FindAllStringIndex(text, -1) {
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
	for i, b := range beats {
		if m, ok := b.(map[string]any); ok && m["cue_phrase"] != nil {
			if refusal := checkCuePhraseBeat(i, m); refusal != nil {
				return nil, refusal
			}
		}
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
		phrase := strings.TrimSpace(m["cue_phrase"].(string))
		tokens := cueTokens(narration)
		phraseTokens := cueTokens(phrase)
		hits := cuePhraseMatches(tokens, phraseTokens)
		if len(hits) == 0 {
			return nil, cuePhraseNotFound(phrase, narration, tokens, phraseTokens)
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

// checkCuePhraseBeat refuses a beat whose cue_phrase cannot be used: not text, empty, too long, without a word, or next
// to a cue. beat is the index of the beat in the scene.
func checkCuePhraseBeat(beat int, m map[string]any) *cuePhraseRefusal {
	where := fmt.Sprintf("beats[%d]", beat)
	phrase, isText := m["cue_phrase"].(string)
	phrase = strings.TrimSpace(phrase)
	switch {
	case !isText || phrase == "":
		return &cuePhraseRefusal{Code: "invalid_value", Message: where + ".cue_phrase must be text of 1 to 60 characters.",
			Next: "Send a few words from the narration as cue_phrase, for example the words the beat should start on."}
	case utf8.RuneCountInString(phrase) > cuePhraseMax:
		return &cuePhraseRefusal{Code: "invalid_value", Message: fmt.Sprintf("%s.cue_phrase has %d characters; the limit is %d.", where, utf8.RuneCountInString(phrase), cuePhraseMax),
			Next: "Shorten cue_phrase to the first few words of the passage."}
	case len(cueTokens(phrase)) == 0:
		return &cuePhraseRefusal{Code: "invalid_value", Message: where + ".cue_phrase has no words.",
			Next: "Send words from the narration as cue_phrase, not only punctuation."}
	case cueNumber(m["cue"]) != 0:
		return &cuePhraseRefusal{Code: "cue_phrase_conflict", Message: where + " has both cue and cue_phrase.",
			Next: "Keep only cue_phrase on this beat; the Studio sets the marker."}
	}
	return nil
}

// cuePhraseNotFound refuses a phrase that is not in the narration and names the closest wording that is.
func cuePhraseNotFound(phrase, narration string, tokens, phraseTokens []cueToken) *cuePhraseRefusal {
	if len(tokens) == 0 {
		return &cuePhraseRefusal{Code: "cue_phrase_not_found", Message: "This scene has no narration, so cue_phrase has nothing to point at.",
			Next: "Write the narration first with put_scene or patch_scene, then name a phrase of it."}
	}
	closest := closestCuePhrase(narration, tokens, len(phraseTokens), phraseTokens)
	return &cuePhraseRefusal{Code: "cue_phrase_not_found",
		Message: fmt.Sprintf("The phrase %q is not in the narration of this scene. The closest wording there is %q.", phrase, closest),
		Next:    fmt.Sprintf("Send cue_phrase %q or another wording copied from the narration.", closest)}
}

// closestCuePhrase returns the text of the window of count tokens whose normalised words are the fewest edits away from
// phrase. A window never outgrows the narration.
func closestCuePhrase(narration string, tokens []cueToken, count int, phrase []cueToken) string {
	count = min(count, len(tokens))
	want := joinCueTokens(phrase)
	best, bestAt := -1, 0
	for i := 0; i+count <= len(tokens); i++ {
		d := editDistance([]rune(joinCueTokens(tokens[i:i+count])), []rune(want))
		if best < 0 || d < best {
			best, bestAt = d, i
		}
	}
	return StripMarkers(narration[tokens[bestAt].start:tokens[bestAt+count-1].end])
}

func joinCueTokens(tokens []cueToken) string {
	words := make([]string, len(tokens))
	for i, t := range tokens {
		words[i] = t.text
	}
	return strings.Join(words, " ")
}

// editDistance is the Levenshtein distance of two rune slices.
func editDistance(a, b []rune) int {
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(a); i++ {
		diag := row[0]
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			diag, row[j] = row[j], min(row[j]+1, row[j-1]+1, diag+cost)
		}
	}
	return row[len(b)]
}
