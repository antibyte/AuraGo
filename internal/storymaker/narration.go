package storymaker

import (
	"regexp"
	"strconv"
	"strings"
)

// Block kinds and piece joins of the narration model.
const (
	BlockKindNarration = "narration"
	BlockKindDialogue  = "dialogue"
	JoinSentence       = "sentence" // 350 ms pause after this piece
	JoinSplit          = "split"    // 150 ms pause after this piece
)

// NarrationBlock is one spoken unit of a story: the narration of a scene, or the text of one say beat.
type NarrationBlock struct {
	ID         string // "<scene>:n" or "<scene>:b<beatIndex>"
	Kind       string
	SceneID    string
	SceneIndex int
	Beat       int    // beat index; -1 for kind narration
	Speaker    string // "narrator" or a cast id
	Voice      Voice  // resolved; Provider == "" when nothing resolves; Provider is in NormalizeProvider form
	Text       string // StripMarkers of the source text
}

// narrationMarker is the marker regex of narration.js. cuePattern in validate.go is narrower, so it cannot be reused.
var narrationMarker = regexp.MustCompile(`\{(\d+)\}`)

// Blocks lists the spoken blocks of a story in scene order, inside a scene the narration first and the say blocks in
// beat order. silent silences only the scene narration. A say whose actor is off stage still makes a block.
func Blocks(s *Story) []NarrationBlock {
	if s == nil {
		return nil
	}
	var out []NarrationBlock
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		if !sc.Silent {
			if text := StripMarkers(sc.Narration); text != "" {
				speaker := sc.Speaker
				if speaker == "" {
					speaker = "narrator"
				}
				out = append(out, NarrationBlock{
					ID: sc.ID + ":n", Kind: BlockKindNarration, SceneID: sc.ID, SceneIndex: i, Beat: -1,
					Speaker: speaker, Voice: blockVoice(s, speaker), Text: text,
				})
			}
		}
		if sc.Kind == "code" {
			continue
		}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do != "say" {
				continue
			}
			text := StripMarkers(b.Text)
			if text == "" {
				continue
			}
			out = append(out, NarrationBlock{
				ID: sc.ID + ":b" + strconv.Itoa(j), Kind: BlockKindDialogue, SceneID: sc.ID, SceneIndex: i, Beat: j,
				Speaker: b.Who, Voice: blockVoice(s, b.Who), Text: text,
			})
		}
	}
	return out
}

// blockVoice is the cast member's voice when the speaker is a cast id with a voice, else the story voice.
func blockVoice(s *Story, speaker string) Voice {
	for i := range s.Cast {
		if s.Cast[i].ID == speaker && s.Cast[i].Voice != nil {
			return normalizedVoice(*s.Cast[i].Voice)
		}
	}
	if s.Voice != nil {
		return normalizedVoice(*s.Voice)
	}
	return Voice{}
}

func normalizedVoice(v Voice) Voice {
	v.Provider = NormalizeProvider(v.Provider)
	return v
}

// NormalizeProvider is the trimmed, lower-cased provider id. The validator accepts mixed case without rewriting it.
func NormalizeProvider(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// StripMarkers is byte for byte SM.narration.parse(text).plain of ui/js/storymaker/narration.js.
func StripMarkers(text string) string {
	plain, _ := parseMarkers(text)
	return plain
}

// parseMarkers follows narration.js parse: the plain text and the first UTF-16 offset of each marker, clamped to the
// trimmed length.
func parseMarkers(text string) (string, map[int]int) {
	var b strings.Builder
	cues := map[int]int{}
	appendChunk := func(chunk string) {
		chunk = collapseJSSpace(chunk)
		if b.Len() == 0 || strings.HasSuffix(b.String(), " ") {
			chunk = strings.TrimPrefix(chunk, " ")
		}
		b.WriteString(chunk)
	}
	last := 0
	for _, m := range narrationMarker.FindAllStringSubmatchIndex(text, -1) {
		appendChunk(text[last:m[0]])
		if n, err := strconv.Atoi(text[m[2]:m[3]]); err == nil {
			if _, seen := cues[n]; !seen {
				cues[n] = UTF16Len(b.String())
			}
		}
		last = m[1]
	}
	appendChunk(text[last:])
	plain := strings.TrimSuffix(b.String(), " ")
	length := UTF16Len(plain)
	for n, at := range cues {
		if at > length {
			cues[n] = length
		}
	}
	return plain, cues
}

// collapseJSSpace turns each run of JS whitespace into one U+0020, like replace(/\s+/g, ' ').
func collapseJSSpace(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if IsJSSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
			}
			inSpace = true
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// IsJSSpace is exactly the JS regex class \s. Go's \s is ASCII only.
func IsJSSpace(r rune) bool {
	switch {
	case r >= 0x0009 && r <= 0x000D, r == 0x0020, r == 0x00A0, r == 0x1680,
		r >= 0x2000 && r <= 0x200A, r == 0x2028, r == 0x2029, r == 0x202F,
		r == 0x205F, r == 0x3000, r == 0xFEFF:
		return true
	}
	return false
}

// UTF16Len counts the UTF-16 units of s, the unit JS string offsets use.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return n
}

// TextWidth counts East Asian wide characters as 2 and everything else as 1. It equals ranges.width in JS.
func TextWidth(s string) int {
	n := 0
	for _, r := range s {
		n++
		if inUCDTable(ucdEastAsianWide, r) {
			n++
		}
	}
	return n
}

// WordCount skips JS whitespace. A run of n CJK word characters counts ceil(n/2), a run of anything else counts 1.
// It equals ranges.wordCount in JS.
func WordCount(s string) int {
	words, cjk := 0, 0
	inOther := false
	for _, r := range s {
		switch {
		case IsJSSpace(r):
			words += (cjk + 1) / 2
			cjk, inOther = 0, false
		case inUCDTable(ucdCJKWord, r):
			inOther = false
			cjk++
		default:
			words += (cjk + 1) / 2
			cjk = 0
			if !inOther {
				words++
				inOther = true
			}
		}
	}
	return words + (cjk+1)/2
}
