# Part D: Cue by phrase

> Generated from the drafted and tested commits 2a529bde3..f7436c8fd. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task D1: match a cue phrase in a narration

**Files:**
- Create: `internal/storymaker/cuephrase.go`
- Create: `internal/storymaker/cuephrase_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/cuephrase_test.go`:

```go
package storymaker

import (
	"slices"
	"testing"
)

// cueMatchStarts returns the byte offsets in narration where phrase starts.
func cueMatchStarts(narration, phrase string) []int {
	tokens := cueTokens(narration)
	var starts []int
	for _, i := range cuePhraseMatches(tokens, cueTokens(phrase)) {
		starts = append(starts, tokens[i].start)
	}
	return starts
}

func TestCuePhraseMatching(t *testing.T) {
	cases := []struct {
		name, narration, phrase string
		want                    []int
	}{
		{"plain", "The firewall checks every packet.", "every packet", []int{20}},
		{"case folding", "Die ÄRZTIN kommt.", "ärztin", []int{4}},
		{"umlaut stays", "Die Ärztin kommt.", "arztin", nil},
		{"punctuation and spaces", "Wait,   then -- go!", "then go", []int{8}},
		{"word boundary at the start", "The unfirewalled net.", "firewalled", nil},
		{"word boundary at the end", "A firewall blocks.", "fire", nil},
		{"markers are skipped", "The {1}firewall {2}checks the net.", "firewall checks", []int{7}},
		{"marker inside the phrase span", "Look {3}here, now.", "look here", []int{0}},
		{"first and later occurrences", "Go on. Go on.", "go on", []int{0, 7}},
		{"cyrillic", "Это МОЙ дом.", "мой дом", []int{7}},
		{"cjk substring", "防火墙会检查每个数据包。", "检查每个", []int{12}},
		{"cjk across punctuation", "你好，世界。", "你好世界", []int{0}},
		{"cjk inside a longer run", "防火墙", "火", []int{3}},
		{"empty phrase", "Hello world", "  ,. ", nil},
	}
	for _, c := range cases {
		if got := cueMatchStarts(c.narration, c.phrase); !slices.Equal(got, c.want) {
			t.Errorf("%s: starts = %v, want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestCuePhraseMatching' -count=1`
Expected: internal\storymaker\cuephrase_test.go:10:12: undefined: cueTokens

- [ ] **Step 3: Implement**

Create `internal/storymaker/cuephrase.go`:

```go
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
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestCuePhraseMatching' -count=1`
Expected: --- PASS: TestCuePhraseMatching (0.00s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/cuephrase.go internal/storymaker/cuephrase_test.go
git commit -m "storymaker: match a cue phrase in a narration"
```

### Task D2: set a marker for each cue phrase and rewrite the beat

**Files:**
- Modify: `internal/storymaker/cuephrase.go`
- Modify: `internal/storymaker/cuephrase_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/cuephrase_test.go`, replace:

```go
		}
	}
}
```

with:

```go
		}
	}
}

// cueFields builds the fields of a scene: its narration and beats as the tool JSON gives them.
func cueFields(narration string, beats ...map[string]any) map[string]any {
	list := make([]any, len(beats))
	for i, b := range beats {
		list[i] = b
	}
	return map[string]any{"narration": narration, "beats": list}
}

func TestResolveCuePhrasesInsertsMarkers(t *testing.T) {
	fields := cueFields("The firewall checks every packet.",
		map[string]any{"do": "emote", "cue_phrase": "Every Packet"},
		map[string]any{"do": "emote", "cue_phrase": "firewall"},
		map[string]any{"do": "emote", "cue": float64(7)})
	notes, refusal := resolveCuePhrases(fields)
	if refusal != nil || len(notes) != 0 {
		t.Fatalf("notes %v, refusal %+v", notes, refusal)
	}
	if got, want := fields["narration"], "The {2}firewall checks {1}every packet."; got != want {
		t.Fatalf("narration = %q, want %q", got, want)
	}
	beats := fields["beats"].([]any)
	if b := beats[0].(map[string]any); b["cue"] != 1 || b["cue_phrase"] != nil || b["do"] != "emote" {
		t.Fatalf("beat 0 = %v", b)
	}
	if b := beats[1].(map[string]any); b["cue"] != 2 || b["cue_phrase"] != nil {
		t.Fatalf("beat 1 = %v", b)
	}
	if b := beats[2].(map[string]any); b["cue"] != float64(7) {
		t.Fatalf("beat 2 = %v, want it untouched", b)
	}
	if StripMarkers(fields["narration"].(string)) != "The firewall checks every packet." {
		t.Fatal("the markers change the spoken text")
	}
}

func TestResolveCuePhrasesReusesAndSkipsTaken(t *testing.T) {
	// The marker {1} already stands before "every"; {2} is taken by a beat cue that points at {2}.
	fields := cueFields("Check {1}every packet and the net.",
		map[string]any{"do": "emote", "cue_phrase": "every"},
		map[string]any{"do": "emote", "cue": float64(2)},
		map[string]any{"do": "emote", "cue_phrase": "net"})
	if _, refusal := resolveCuePhrases(fields); refusal != nil {
		t.Fatalf("refusal %+v", refusal)
	}
	if got, want := fields["narration"], "Check {1}every packet and the {3}net."; got != want {
		t.Fatalf("narration = %q, want %q", got, want)
	}
	if b := fields["beats"].([]any)[0].(map[string]any); b["cue"] != 1 {
		t.Fatalf("beat 0 = %v, want the reused marker 1", b)
	}
}

func TestResolveCuePhrasesFirstOccurrenceNote(t *testing.T) {
	fields := cueFields("Go on. Go on.", map[string]any{"do": "emote", "cue_phrase": "go on"})
	notes, refusal := resolveCuePhrases(fields)
	if refusal != nil || len(notes) != 1 {
		t.Fatalf("notes %v, refusal %+v", notes, refusal)
	}
	if got, want := fields["narration"], "{1}Go on. Go on."; got != want {
		t.Fatalf("narration = %q, want %q", got, want)
	}
}

func TestResolveCuePhrasesLeavesOtherFieldsAlone(t *testing.T) {
	fields := cueFields("Hello world.", map[string]any{"do": "emote", "cue": float64(1)})
	if notes, refusal := resolveCuePhrases(fields); notes != nil || refusal != nil || fields["narration"] != "Hello world." {
		t.Fatalf("notes %v, refusal %+v, fields %v", notes, refusal, fields)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestResolveCuePhrases' -count=1`
Expected: internal\storymaker\cuephrase_test.go:60:20: undefined: resolveCuePhrases

- [ ] **Step 3: Implement**

In `internal/storymaker/cuephrase.go`, replace:

```go
package storymaker

import (
	"unicode"
	"unicode/utf8"
)
```

with:

```go
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
```

In `internal/storymaker/cuephrase.go`, replace:

```go
	}
	return at
}
```

with:

```go
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
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestResolveCuePhrases' -count=1`
Expected: --- PASS: TestResolveCuePhrasesInsertsMarkers (0.00s) (4 PASS lines)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/cuephrase.go internal/storymaker/cuephrase_test.go
git commit -m "storymaker: set a marker for each cue phrase and rewrite the beat"
```

### Task D3: refuse unusable cue phrases and name the closest wording

**Files:**
- Modify: `internal/storymaker/cuephrase.go`
- Modify: `internal/storymaker/cuephrase_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/cuephrase_test.go`, replace:

```go

import (
	"slices"
	"testing"
)
```

with:

```go

import (
	"slices"
	"strings"
	"testing"
)
```

In `internal/storymaker/cuephrase_test.go`, replace:

```go
		t.Fatalf("notes %v, refusal %+v, fields %v", notes, refusal, fields)
	}
}
```

with:

```go
		t.Fatalf("notes %v, refusal %+v, fields %v", notes, refusal, fields)
	}
}

func TestResolveCuePhrasesRefusals(t *testing.T) {
	long := strings.Repeat("word ", 13)
	cases := []struct {
		name     string
		fields   map[string]any
		wantCode string
		message  string // a text the message must hold
		next     string // a text the next step must hold
	}{
		{"both cue and cue_phrase", cueFields("Hello world.", map[string]any{"do": "emote", "cue": float64(1), "cue_phrase": "hello"}),
			"cue_phrase_conflict", "beats[0]", "only cue_phrase"},
		{"empty", cueFields("Hello world.", map[string]any{"do": "emote", "cue_phrase": "  "}), "invalid_value", "1 to 60", "from the narration"},
		{"too long", cueFields("Hello world.", map[string]any{"do": "emote", "cue_phrase": long}), "invalid_value", "has 64 characters", "Shorten"},
		{"not text", cueFields("Hello world.", map[string]any{"do": "emote", "cue_phrase": float64(3)}), "invalid_value", "1 to 60", "narration"},
		{"punctuation only", cueFields("Hello world.", map[string]any{"do": "emote", "cue_phrase": "?!"}), "invalid_value", "no words", "not only punctuation"},
		{"not found names the closest words", cueFields("The firewall checks every packet.", map[string]any{"do": "emote", "cue_phrase": "evry pakket"}),
			"cue_phrase_not_found", `closest wording there is "every packet"`, `"every packet"`},
		{"closest has the same word count", cueFields("A firewall protects the net.", map[string]any{"do": "emote", "cue_phrase": "the network"}),
			"cue_phrase_not_found", `"the net"`, "copied from the narration"},
		{"no narration", cueFields("", map[string]any{"do": "emote", "cue_phrase": "hello"}),
			"cue_phrase_not_found", "no narration", "Write the narration first"},
		{"a later beat refuses the whole call", cueFields("Hello world.", map[string]any{"do": "emote", "cue_phrase": "hello"}, map[string]any{"do": "emote", "cue_phrase": "nowhere"}),
			"cue_phrase_not_found", `"nowhere"`, "Send cue_phrase"},
	}
	for _, c := range cases {
		narration := c.fields["narration"]
		notes, refusal := resolveCuePhrases(c.fields)
		if refusal == nil || notes != nil {
			t.Errorf("%s: notes %v, refusal %+v", c.name, notes, refusal)
			continue
		}
		if refusal.Code != c.wantCode || !strings.Contains(refusal.Message, c.message) || !strings.Contains(refusal.Next, c.next) {
			t.Errorf("%s: refusal = %+v, want code %s, message with %q, next with %q", c.name, refusal, c.wantCode, c.message, c.next)
		}
		if c.fields["narration"] != narration || c.fields["beats"].([]any)[0].(map[string]any)["cue_phrase"] == nil {
			t.Errorf("%s: a refusal changed the fields: %v", c.name, c.fields)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestResolveCuePhrasesRefusals' -count=1`
Expected: cuephrase_test.go:145: both cue and cue_phrase: notes [], refusal <nil>

- [ ] **Step 3: Implement**

In `internal/storymaker/cuephrase.go`, replace:

```go
	if !slices.ContainsFunc(beats, func(b any) bool { m, ok := b.(map[string]any); return ok && m["cue_phrase"] != nil }) {
		return nil, nil
	}
	narration, _ := fields["narration"].(string)
	taken := map[int]bool{}
	for _, m := range narrationMarker.FindAllStringSubmatch(narration, -1) {
```

with:

```go
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
```

In `internal/storymaker/cuephrase.go`, replace:

```go
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
```

with:

```go
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
```

In `internal/storymaker/cuephrase.go`, replace:

```go
	}
	return 0
}
```

with:

```go
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
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestResolveCuePhrasesRefusals' -count=1`
Expected: --- PASS: TestResolveCuePhrasesRefusals (0.00s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/cuephrase.go internal/storymaker/cuephrase_test.go
git commit -m "storymaker: refuse unusable cue phrases and name the closest wording"
```

### Task D4: resolve cue phrases in put_scene, patch_scene and dry_run

**Files:**
- Create: `internal/storymaker/studio_cuephrase_test.go`
- Modify: `internal/storymaker/studio_story_scenes.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/studio_cuephrase_test.go`:

```go
package storymaker

import (
	"context"
	"strings"
	"testing"
)

const cuePhraseScene = `"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present"}]`

func TestStudioSceneWriteResolvesCuePhrase(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	answer := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"narration":"Eine Firewall schuetzt das Netz. Das Netz bleibt sicher.",
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"das netz"}]}}`)
	if answer["status"] != "ok" {
		t.Fatalf("answer = %v, want ok", answer)
	}
	if next, _ := answer["next_action"].(string); !strings.Contains(next, "occurs 2 times") || !strings.Contains(next, "This scene is saved.") {
		t.Fatalf("next_action = %q, want the saved text and the note", next)
	}
	raw, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := DecodeStory(raw, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	s1 := story.Scenes[0]
	if want := "Eine Firewall schuetzt {1}das Netz. Das Netz bleibt sicher."; s1.Narration != want || s1.Beats[0].Cue != 1 {
		t.Fatalf("saved narration %q cue %d, want %q and 1", s1.Narration, s1.Beats[0].Cue, want)
	}
	if strings.Contains(string(raw), "cue_phrase") {
		t.Fatal("cue_phrase reached story.json")
	}

	// A patch that only sends a beat finds the phrase in the saved narration, whose marker it keeps.
	answer = studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Firewall"}]}}`)
	if answer["status"] != "ok" {
		t.Fatalf("patch answer = %v, want ok", answer)
	}
	raw, _, _ = ps.LoadStory(ctx, id)
	story, _, _ = DecodeStory(raw, DefaultLimits())
	if got := story.Scenes[0].Narration; got != "Eine {2}Firewall schuetzt {1}das Netz. Das Netz bleibt sicher." {
		t.Fatalf("narration after the patch = %q", got)
	}
}

func TestStudioSceneWriteRefusesCuePhrase(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	_, before, _ := ps.LoadStory(ctx, id)
	answer := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Firewal"}]}}`)
	if answer["status"] != "refused" || answer["code"] != "cue_phrase_not_found" || !strings.Contains(answer["message"].(string), `"Firewall"`) {
		t.Fatalf("answer = %v, want cue_phrase_not_found naming Firewall", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != before {
		t.Fatal("a refused write changed the story")
	}
	if studio.writes["s1"] != 1 {
		t.Fatalf("writes = %d, want the refused write counted", studio.writes["s1"])
	}
}

func TestStudioDryRunResolvesCuePhrase(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	answer := studioWriteCall(t, studio, ToolStory, "dry_run", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"schuetzt"}]}}`)
	if answer["status"] != "ok" || len(answer["issues"].([]any)) != 0 {
		t.Fatalf("answer = %v, want ok without issues", answer)
	}
	bad := studioWriteCall(t, studio, ToolStory, "dry_run", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"nirgends"}]}}`)
	if bad["status"] != "refused" || bad["code"] != "cue_phrase_not_found" {
		t.Fatalf("answer = %v, want cue_phrase_not_found", bad)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestStudio(SceneWrite.*CuePhrase|DryRunResolvesCuePhrase)' -count=1`
Expected: A beat has no field "cue_phrase". (unknown_field, answer status draft, want ok)

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_story_scenes.go`, replace:

```go
		return *refused
	}
	s.repairSceneFields(fields)
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
		if ev.scene.Kind == "code" {
```

with:

```go
		return *refused
	}
	s.repairSceneFields(fields)
	cueNotes, cueRefusal := resolveCuePhrases(fields)
	if cueRefusal != nil {
		s.writes[sceneID]++
		return sceneWriteRefused(op, sceneID, cueRefusal.Code, cueRefusal.Message, cueRefusal.Next, nil)
	}
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
		if ev.scene.Kind == "code" {
```

In `internal/storymaker/studio_story_scenes.go`, replace:

```go
				next = "The scene is saved but nothing is on stage. Add actors and beats with patch_scene."
			}
		}
		return sceneWriteOK(op, sceneID, newETag, ev.issues, next)
	}
	free := s.writes[sceneID] == 0 || ev.errors < s.lastIssueCount[sceneID]
	if !free && s.charged[sceneID] >= StudioSceneChargedLimit {
```

with:

```go
				next = "The scene is saved but nothing is on stage. Add actors and beats with patch_scene."
			}
		}
		return sceneWriteOK(op, sceneID, newETag, ev.issues, withCueNotes(next, cueNotes))
	}
	free := s.writes[sceneID] == 0 || ev.errors < s.lastIssueCount[sceneID]
	if !free && s.charged[sceneID] >= StudioSceneChargedLimit {
```

In `internal/storymaker/studio_story_scenes.go`, replace:

```go
		next = "Fix the listed issues with patch_scene and the same scene_id. Only a patch that removes an issue is still accepted."
	}
	return sceneWriteAnswer(op, "draft", sceneID, map[string]any{
		"draft": true, "scene_id": sceneID, "issues": sceneWriteIssues(ev.issues, false), "corrections_left": left, "next_action": next,
	})
}
```

with:

```go
		next = "Fix the listed issues with patch_scene and the same scene_id. Only a patch that removes an issue is still accepted."
	}
	return sceneWriteAnswer(op, "draft", sceneID, map[string]any{
		"draft": true, "scene_id": sceneID, "issues": sceneWriteIssues(ev.issues, false), "corrections_left": left, "next_action": withCueNotes(next, cueNotes),
	})
}
```

In `internal/storymaker/studio_story_scenes.go`, replace:

```go
	}
	s.dryRunsJob++
	s.dryRunsScene[target]++
	ev := s.evaluateSceneWrite(story, target, fields)
	body := map[string]any{
		"issues":           sceneWriteIssues(ev.issues, false),
		"defaults_applied": append([]DefaultApplied{}, ev.defaults...),
	}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
```

with:

```go
	}
	s.dryRunsJob++
	s.dryRunsScene[target]++
	cueNotes, cueRefusal := resolveCuePhrases(fields)
	if cueRefusal != nil {
		return sceneWriteRefused(op, sceneID, cueRefusal.Code, cueRefusal.Message, cueRefusal.Next, nil)
	}
	ev := s.evaluateSceneWrite(story, target, fields)
	body := map[string]any{
		"issues":           sceneWriteIssues(ev.issues, false),
		"defaults_applied": append([]DefaultApplied{}, ev.defaults...),
	}
	if next := withCueNotes("", cueNotes); next != "" {
		body["next_action"] = next
	}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
```

In `internal/storymaker/studio_story_scenes.go`, replace:

```go
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

func sceneWriteArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
```

with:

```go
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

// withCueNotes appends the notes of resolveCuePhrases to the next_action text of an answer.
func withCueNotes(next string, notes []string) string {
	return strings.TrimSpace(next + " " + strings.Join(notes, " "))
}

func sceneWriteArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestStudio(SceneWrite.*CuePhrase|DryRunResolvesCuePhrase)' -count=1`
Expected: --- PASS: TestStudioSceneWriteResolvesCuePhrase (0.01s) (3 PASS lines)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_cuephrase_test.go internal/storymaker/studio_story_scenes.go
git commit -m "storymaker: resolve cue phrases in put_scene, patch_scene and dry_run"
```

### Task D5: offer cue_phrase in the beat schema

**Files:**
- Modify: `internal/agent/native_tools_storymaker.go`
- Modify: `internal/agent/native_tools_storymaker_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/agent/native_tools_storymaker_test.go`, replace:

```go
		t.Fatalf("%s missing from the revise schemas", storymaker.ToolStory)
	}
}
```

with:

```go
		t.Fatalf("%s missing from the revise schemas", storymaker.ToolStory)
	}
}

func TestStorymakerBeatSchemaOffersCuePhrase(t *testing.T) {
	names := storymakerCatalogNames(t)
	for _, compact := range []bool{false, true} {
		story := storymakerToolParams(StorymakerPhaseToolSchemas(storymaker.PhaseFill, compact, names), storymaker.ToolStory)
		phrase := storymakerSchemaAt(story, "data", "beats", "[]", "cue_phrase")
		if phrase == nil || phrase["type"] != "string" || phrase["maxLength"] != 60 || phrase["description"] == "" {
			t.Errorf("compact=%v cue_phrase = %v, want a described string of at most 60 characters", compact, phrase)
		}
		if storymakerSchemaAt(story, "data", "beats", "[]", "cue") == nil {
			t.Errorf("compact=%v beat lost cue", compact)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: native_tools_storymaker_test.go:261: compact=false cue_phrase = map[], want a described string of at most 60 characters

- [ ] **Step 3: Implement**

In `internal/agent/native_tools_storymaker.go`, replace:

```go
	}
	return map[string]interface{}{
		"kind":        storymakerEnum("storyboard", "code"),
		"source":      map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only; first read get_code and the code reference"},
		"environment": environment,
		"variant":     storymakerEnum("day", "night"),
		"narration":   storymakerTyped("string"),
```

with:

```go
	}
	return map[string]interface{}{
		"kind":        storymakerEnum("storyboard", "code"),
		"source":      map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only"},
		"environment": environment,
		"variant":     storymakerEnum("day", "night"),
		"narration":   storymakerTyped("string"),
```

In `internal/agent/native_tools_storymaker.go`, replace:

```go
		"seconds":    storymakerTyped("number"),
		"with":       storymakerEnum("prev"),
		"cue":        storymakerTyped("integer"),
	}
	properties["count"] = storymakerTyped("integer")
	properties["id"] = storymakerTyped("string")
```

with:

```go
		"seconds":    storymakerTyped("number"),
		"with":       storymakerEnum("prev"),
		"cue":        storymakerTyped("integer"),
		"cue_phrase": map[string]interface{}{"type": "string", "maxLength": 60, "description": "narration words to cue on"},
	}
	properties["count"] = storymakerTyped("integer")
	properties["id"] = storymakerTyped("string")
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: ok  	aurago/internal/agent (all storymaker tests, incl. the 900-token cap; the source description shrank to "put_code only" to pay for the field)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/native_tools_storymaker.go internal/agent/native_tools_storymaker_test.go
git commit -m "storymaker: offer cue_phrase in the beat schema"
```

### Task D6: teach cue_phrase in the profile, the reference and the manual

**Files:**
- Modify: `internal/server/storymaker_agent_profile.go`
- Modify: `internal/server/storymaker_agent_profile_test.go`
- Modify: `internal/storymaker/reference/animation.md`
- Modify: `prompts/tools_manuals/storymaker.md`

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_agent_profile_test.go`, replace:

```go
							t.Errorf("%s lacks animation instruction %q", wantName, instruction)
						}
					}
				}
				if !strings.Contains(system, "Plan a meaningful visual for each idea") || strings.Contains(system, "use the nearest alternative, or a show beat") {
					t.Errorf("%s lacks catalog-first visual planning or retains the text-only fallback", wantName)
```

with:

```go
							t.Errorf("%s lacks animation instruction %q", wantName, instruction)
						}
					}
					if !strings.Contains(system, "cue_phrase") || strings.Contains(system, `"cue":1`) || strings.Contains(system, "write {1}") {
						t.Errorf("%s must teach cue_phrase and never the numbered marker", wantName)
					}
				}
				if !strings.Contains(system, "Plan a meaningful visual for each idea") || strings.Contains(system, "use the nearest alternative, or a show beat") {
					t.Errorf("%s lacks catalog-first visual planning or retains the text-only fallback", wantName)
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run 'TestStorymakerProfiles' -count=1`
Expected: storymaker_agent_profile_test.go:66: storymaker/v5/revise/normal must teach cue_phrase and never the numbered marker

- [ ] **Step 3: Implement**

In `internal/server/storymaker_agent_profile.go`, replace:

```go
const storymakerHouseRules = `## House rules
1. Open on a person: the first scene has an actor on stage in its first frame.
2. One idea per scene, within the word budget. A scene is about 8 seconds of speech.
3. Reveal each thing when it is named: write {1} in the narration before that word and set "cue":1 on the beat that shows it. Pair reveals with a simultaneous reaction ("with":"prev") such as gesture, expression, look, highlight or sfx.
4. When an on-stage cast member speaks the scene narration, set "speaker" to their cast id so their mouth lip-syncs automatically.
5. Never place a prop or text card on the same spot ("at") as an on-stage actor; use a free spot (for example actor at "left" or "desk", prop/text at "right", "center" or "board").
6. On-screen text is at most five words and never repeats the narration. No bullet lists, no three equal cards in a row.
```

with:

```go
const storymakerHouseRules = `## House rules
1. Open on a person: the first scene has an actor on stage in its first frame.
2. One idea per scene, within the word budget. A scene is about 8 seconds of speech.
3. Reveal each thing when it is named: set "cue_phrase" on the beat that shows it to the words of the narration that name it. The Studio sets the marker. Pair reveals with a simultaneous reaction ("with":"prev") such as gesture, expression, look, highlight or sfx.
4. When an on-stage cast member speaks the scene narration, set "speaker" to their cast id so their mouth lip-syncs automatically.
5. Never place a prop or text card on the same spot ("at") as an on-stage actor; use a free spot (for example actor at "left" or "desk", prop/text at "right", "center" or "board").
6. On-screen text is at most five words and never repeats the narration. No bullet lists, no three equal cards in a row.
```

In `internal/server/storymaker_agent_profile.go`, replace:

```go
1. Choose one template: product_explainer, how_to, concept_explainer, dialogue_story, data_story or onboarding.
2. Build the arc hook, problem, solution, how, benefit, close, using suggested_scenes. The context lists the environments with a line about each in environment_guide. The dialogue_story template uses one scene per speaker turn.
- Write the narration of the last scene first, so that the first scenes set up the ending.
- Narration adds what the picture cannot show. It never describes what is on screen, and you may place {1} markers before key reveal words.
- Read it once more and cut every word the film does not need.
3. Call storymaker_story with operation outline once: data has template, target_seconds and scenes, each with a short id such as s1 and its narration.
4. If the answer says the text is too long, shorten it and call outline again. An accepted outline ends this phase: stop.`
```

with:

```go
1. Choose one template: product_explainer, how_to, concept_explainer, dialogue_story, data_story or onboarding.
2. Build the arc hook, problem, solution, how, benefit, close, using suggested_scenes. The context lists the environments with a line about each in environment_guide. The dialogue_story template uses one scene per speaker turn.
- Write the narration of the last scene first, so that the first scenes set up the ending.
- Narration adds what the picture cannot show. It never describes what is on screen. Write it as plain text; a later beat names its words with cue_phrase.
- Read it once more and cut every word the film does not need.
3. Call storymaker_story with operation outline once: data has template, target_seconds and scenes, each with a short id such as s1 and its narration.
4. If the answer says the text is too long, shorten it and call outline again. An accepted outline ends this phase: stop.`
```

In `internal/server/storymaker_agent_profile.go`, replace:

```go

const storymakerFillWorkflow = `## Fill one scene
The context names one scene, plus previous_scene, next_scene, suggested_props and used_entries when available. Call storymaker_story with operation put_scene, its scene_id and data.
- data needs environment, actors and beats. Send narration with {1}, {2} markers before the key words that trigger beats, or leave narration out if it already has markers.
- Set speaker to an on-stage actor's cast id when that character speaks the narration (enabling automatic lip-sync).
- The context has a direction for this scene: its role, a suggested environment, framing, pose, gesture, entry, actor and the spots of the actor and the prop. Follow it unless the narration needs something else. The example in the context is built from it.
- Keep the same environment only while the scene's role stays the same, and do not repeat the gesture of previous_scene. The server checks the whole film: at least three environments in a long film, a framing other than wide in at least one scene, varied gestures, and every cast member on stage in some scene.
```

with:

```go

const storymakerFillWorkflow = `## Fill one scene
The context names one scene, plus previous_scene, next_scene, suggested_props and used_entries when available. Call storymaker_story with operation put_scene, its scene_id and data.
- data needs environment, actors and beats. Send narration as plain text, or leave it out to keep the saved one. A beat that starts on a word of the narration gets "cue_phrase" with those words, 1 to 60 characters copied from the narration; the Studio sets the marker. Markers like {1} that you see in a narration are the Studio's: keep them when you resend it.
- Set speaker to an on-stage actor's cast id when that character speaks the narration (enabling automatic lip-sync).
- The context has a direction for this scene: its role, a suggested environment, framing, pose, gesture, entry, actor and the spots of the actor and the prop. Follow it unless the narration needs something else. The example in the context is built from it.
- Keep the same environment only while the scene's role stays the same, and do not repeat the gesture of previous_scene. The server checks the whole film: at least three environments in a long film, a framing other than wide in at least one scene, varied gestures, and every cast member on stage in some scene.
```

In `internal/server/storymaker_agent_profile.go`, replace:

```go

const storymakerAnimationGuide = `## Animation and custom effects
- Read storymaker_catalog operation reference, topic animation before filling or revising visuals. It describes actual beat fields, timing, cues, easing, camera, highlights, sound and aspect-safe staging. Search/describe the live catalog for exact assets and usage examples.
- Explain through motion: walk for people, move for props; pair a gesture with a fitting expression and look. Use cue markers to align reveals with words; with prev overlaps actions. Control delay, duration, ease and count deliberately. A camera move should direct attention, not decorate every scene.
- Characters, environments and props are illustrated fixed identities. Rig articulation, breathing, blinking, planted walking and storyboard lip-sync are automatic. Environment layers supply parallax during camera movement. No unsupported view, clothing, shader or filter parameters.
- In cast/revise, set_meta supports style.rhythm calm/standard/lively and style.auto_sfx. Keep text short and check portrait/square composition. The check measures the layout of every storyboard scene: fix the layout findings it reports.
- Prefer storyboard for dialogue. For a missing visual motif, custom diagram or repeated drawing effect, read reference topic code: it includes an executable SVG asset example combining catalog props with g.path SVG path data (not raw SVG markup or an imported .svg file). Use coherent colours, readable silhouettes, layered detail and cue-bound motion. Keep labels as real text. Do not replace a speaking-actor scene merely to add decoration; code scenes support off-screen narration but have no dialogue/lip-sync binding.
```

with:

```go

const storymakerAnimationGuide = `## Animation and custom effects
- Read storymaker_catalog operation reference, topic animation before filling or revising visuals. It describes actual beat fields, timing, cues, easing, camera, highlights, sound and aspect-safe staging. Search/describe the live catalog for exact assets and usage examples.
- Explain through motion: walk for people, move for props; pair a gesture with a fitting expression and look. Use cue_phrase to align reveals with words; with prev overlaps actions. Control delay, duration, ease and count deliberately. A camera move should direct attention, not decorate every scene.
- Characters, environments and props are illustrated fixed identities. Rig articulation, breathing, blinking, planted walking and storyboard lip-sync are automatic. Environment layers supply parallax during camera movement. No unsupported view, clothing, shader or filter parameters.
- In cast/revise, set_meta supports style.rhythm calm/standard/lively and style.auto_sfx. Keep text short and check portrait/square composition. The check measures the layout of every storyboard scene: fix the layout findings it reports.
- Prefer storyboard for dialogue. For a missing visual motif, custom diagram or repeated drawing effect, read reference topic code: it includes an executable SVG asset example combining catalog props with g.path SVG path data (not raw SVG markup or an imported .svg file). Use coherent colours, readable silhouettes, layered detail and cue-bound motion. Keep labels as real text. Do not replace a speaking-actor scene merely to add decoration; code scenes support off-screen narration but have no dialogue/lip-sync binding.
```

In `internal/server/storymaker_agent_profile.go`, replace:

```go
	},
	storymaker.PhaseFill: {
		`{"action":"storymaker_catalog","operation":"search","kind":"prop","query":"lock"}`,
		`{"action":"storymaker_story","operation":"put_scene","scene_id":"s1","data":{"narration":"Eine {1}Firewall prüft jedes Paket.","speaker":"anna","environment":"env.server-room","actors":[{"cast":"anna","at":"left","pose":"stand","expression":"neutral"}],"beats":[{"do":"show","item":"prop.lock","at":"center","entry":"fade","cue":1},{"do":"gesture","who":"anna","gesture":"wave","with":"prev"}]}}`,
	},
	storymaker.PhaseCheck: {
		`{"action":"storymaker_check","operation":"validate"}`,
```

with:

```go
	},
	storymaker.PhaseFill: {
		`{"action":"storymaker_catalog","operation":"search","kind":"prop","query":"lock"}`,
		`{"action":"storymaker_story","operation":"put_scene","scene_id":"s1","data":{"narration":"Eine Firewall prüft jedes Paket.","speaker":"anna","environment":"env.server-room","actors":[{"cast":"anna","at":"left","pose":"stand","expression":"neutral"}],"beats":[{"do":"show","item":"prop.lock","at":"center","entry":"fade","cue_phrase":"Firewall"},{"do":"gesture","who":"anna","gesture":"wave","with":"prev"}]}}`,
	},
	storymaker.PhaseCheck: {
		`{"action":"storymaker_check","operation":"validate"}`,
```

In `internal/storymaker/reference/animation.md`, replace:

```markdown

## Timing

- Scene narration normally determines duration. Mark a spoken word with {1}, then use cue:1 on its beat. A cue must exist in that scene's narration. Keep narration when putting a scene, or resend it with the markers inserted.
- Without cue, actions follow the previous beat. with:"prev" starts together with the previous beat. delay offsets that start; duration controls the motion. Use count for repeated gestures.
- Eases: linear, ease-in, ease-out, ease-in-out, back-out, bounce-out. Use linear for constant travel, ease-in-out for camera/repositioning, back-out or bounce-out sparingly for emphasis. Text stops moving when it lands.
- In cast/revise, set_meta accepts style.rhythm calm/standard/lively and style.auto_sfx true/false. Rhythm changes default timings; explicit beat durations remain deliberate. Automatic sound effects require auto_sfx; music stays the user's choice in the app.
```

with:

```markdown

## Timing

- Scene narration normally determines duration. To start a beat on a spoken word, give the beat cue_phrase: 1 to 60 characters copied from that scene's narration (case and punctuation do not matter, umlauts do). The Studio finds the first match, sets the marker and cue itself, and refuses a phrase it cannot find with the closest wording. Write narration as plain text. Markers like {1} that you see in a saved narration are the Studio's: keep them when you resend the narration.
- Without cue, actions follow the previous beat. with:"prev" starts together with the previous beat. delay offsets that start; duration controls the motion. Use count for repeated gestures.
- Eases: linear, ease-in, ease-out, ease-in-out, back-out, bounce-out. Use linear for constant travel, ease-in-out for camera/repositioning, back-out or bounce-out sparingly for emphasis. Text stops moving when it lands.
- In cast/revise, set_meta accepts style.rhythm calm/standard/lively and style.auto_sfx true/false. Rhythm changes default timings; explicit beat durations remain deliberate. Automatic sound effects require auto_sfx; music stays the user's choice in the app.
```

In `internal/storymaker/reference/animation.md`, replace:

````markdown
Example scene payload (cast member anna must exist):

```json
{"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present","expression":"neutral"}],"narration":"Eine {1}Verschlüsselung schützt Deine Daten.","beats":[{"do":"show","id":"lock","item":"prop.lock","at":"right","entry":"pop","cue":1},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"emote","who":"anna","expression":"happy","with":"prev"},{"do":"highlight","target":"lock","style":"pulse","duration":0.8},{"do":"camera","op":"focus","target":"anna","duration":0.8,"ease":"ease-in-out"}]}
```

Use custom code when storyboard controls or catalog assets cannot express the explanation, for example a missing visual motif, repeated data-flow particles or a bespoke diagram. Read topic code before writing it. Charts/icons/diagrams that report not_supported_yet are not rendered: use actual props/text/shapes or a code scene instead. Validate with storymaker_check and fix errors; do not claim unsupported effects or verified layout.
````

with:

````markdown
Example scene payload (cast member anna must exist):

```json
{"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present","expression":"neutral"}],"narration":"Eine Verschlüsselung schützt Deine Daten.","beats":[{"do":"show","id":"lock","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Verschlüsselung"},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"emote","who":"anna","expression":"happy","with":"prev"},{"do":"highlight","target":"lock","style":"pulse","duration":0.8},{"do":"camera","op":"focus","target":"anna","duration":0.8,"ease":"ease-in-out"}]}
```

Use custom code when storyboard controls or catalog assets cannot express the explanation, for example a missing visual motif, repeated data-flow particles or a bespoke diagram. Read topic code before writing it. Charts/icons/diagrams that report not_supported_yet are not rendered: use actual props/text/shapes or a code scene instead. Validate with storymaker_check and fix errors; do not claim unsupported effects or verified layout.
````

In `prompts/tools_manuals/storymaker.md`, replace:

```markdown

Use `storymaker` when the user wants an explainer video from a topic. An isolated agent writes the story in the background in the Storymaker desktop app. This tool starts and follows that job; you never write scenes yourself.

Pass visual requirements in topic/instruction: the isolated author plans relevant catalog assets and supporting objects, searches concrete motifs and, when none fits, creates SVG path illustrations with the restricted drawing API. Its bundled references include executable asset-composition and effects examples, speech cues, camera, timing/easing, highlights and sound. Custom SVG paths are saved in revision-bound scene source, not imported as arbitrary SVG files. Speaking actors retain storyboard lip-sync; custom illustration scenes use off-screen narration. Raw PixiJS/GSAP, unsupported filters and code-scene dialogue/lip-sync are unavailable. Do not promise those effects or verified pixel layout.

Speech always uses bundled local Phonon, independently of chat TTS. Supported spoken languages are German, English, French, Spanish and Portuguese. The server automatically matches cast voices to language and catalog gender, including scene narration speakers and dialogue. The optional voice selects only a compatible Phonon narrator voice; do not suggest another provider. Unsupported speech languages or missing resources are reported explicitly.
```

with:

```markdown

Use `storymaker` when the user wants an explainer video from a topic. An isolated agent writes the story in the background in the Storymaker desktop app. This tool starts and follows that job; you never write scenes yourself.

Pass visual requirements in topic/instruction: the isolated author plans relevant catalog assets and supporting objects, searches concrete motifs and, when none fits, creates SVG path illustrations with the restricted drawing API. Its bundled references include executable asset-composition and effects examples, speech cues (a beat names the narration words it starts on with `cue_phrase`; the Studio sets the markers), camera, timing/easing, highlights and sound. Custom SVG paths are saved in revision-bound scene source, not imported as arbitrary SVG files. Speaking actors retain storyboard lip-sync; custom illustration scenes use off-screen narration. Raw PixiJS/GSAP, unsupported filters and code-scene dialogue/lip-sync are unavailable. Do not promise those effects or verified pixel layout.

Speech always uses bundled local Phonon, independently of chat TTS. Supported spoken languages are German, English, French, Spanish and Portuguese. The server automatically matches cast voices to language and catalog gender, including scene narration speakers and dialogue. The optional voice selects only a compatible Phonon narrator voice; do not suggest another provider. Unsupported speech languages or missing resources are reported explicitly.
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run 'TestStorymakerProfiles' -count=1`
Expected: --- PASS: TestStorymakerProfilesNamesToolsAndTextGuide (0.01s)

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_agent_profile.go internal/server/storymaker_agent_profile_test.go internal/storymaker/reference/animation.md prompts/tools_manuals/storymaker.md
git commit -m "storymaker: teach cue_phrase in the profile, the reference and the manual"
```

### Task D7: the fill packet's example beat uses cue_phrase

**Files:**
- Modify: `internal/storymaker/studio_direction_packet_test.go`
- Modify: `internal/storymaker/studio_packet.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_direction_packet_test.go`, replace:

```go
import (
	"context"
	"encoding/json"
	"testing"
)
```

with:

```go
import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)
```

In `internal/storymaker/studio_direction_packet_test.go`, replace:

```go
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i, d := range DirectionPlan(&s, "", cat) {
		raw, err := json.Marshal(packetFillExample(s.Cast, d))
		if err != nil {
			t.Fatal(err)
		}
```

with:

```go
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i, d := range DirectionPlan(&s, "", cat) {
		example := packetFillExample(s.Cast, d)
		// The example teaches cue_phrase, never a numbered cue, and its phrase must resolve like a model's would.
		beat := example["beats"].([]any)[0].(map[string]any)
		if beat["cue_phrase"] == nil || beat["cue"] != nil || strings.Contains(example["narration"].(string), "{") {
			t.Errorf("example %d (%s) must use cue_phrase and a plain narration: %v", i+1, d.Role, example)
		}
		if _, refusal := resolveCuePhrases(example); refusal != nil {
			t.Fatalf("example %d (%s): %+v", i+1, d.Role, refusal)
		}
		raw, err := json.Marshal(example)
		if err != nil {
			t.Fatal(err)
		}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestFillExamplesValidate' -count=1`
Expected: studio_direction_packet_test.go:19: example 1 (hook) must use cue_phrase and a plain narration

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_packet.go`, replace:

```go
		entry = "pop"
	}
	example := map[string]any{
		"narration":   "Eine {1}Firewall prüft jedes Paket am Eingang.",
		"environment": environment,
		"transition":  "wipe",
	}
	if dir.Framing != "" && dir.Framing != "wide" {
		example["camera"] = map[string]any{"framing": dir.Framing}
	}
	beats := []any{map[string]any{"do": "show", "item": prop, "at": propAt, "entry": entry, "cue": 1}}
	if len(cast) > 0 {
		who := cast[0].ID
		if dir.Actor != "" {
```

with:

```go
		entry = "pop"
	}
	example := map[string]any{
		"narration":   "Eine Firewall prüft jedes Paket am Eingang.",
		"environment": environment,
		"transition":  "wipe",
	}
	if dir.Framing != "" && dir.Framing != "wide" {
		example["camera"] = map[string]any{"framing": dir.Framing}
	}
	beats := []any{map[string]any{"do": "show", "item": prop, "at": propAt, "entry": entry, "cue_phrase": "Firewall"}}
	if len(cast) > 0 {
		who := cast[0].ID
		if dir.Actor != "" {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestFillExamplesValidate' -count=1`
Expected: --- PASS: TestFillExamplesValidate (0.03s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_direction_packet_test.go internal/storymaker/studio_packet.go
git commit -m "storymaker: the fill packet's example beat uses cue_phrase"
```

