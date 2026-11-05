package storymaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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

// Piece is one provider call of a block.
type Piece struct {
	Text  string // what is synthesised and hashed; no leading or trailing whitespace
	Chars int    // UTF-16 units of the source span including the whitespace after it
	Join  string // how it joins the next piece; "" on the last
}

const (
	fragmentWidth = 20 // a part below this width merges
	fragmentWords = 3  // a part with this many words or fewer merges
	minSplitRunes = 40 // each side of a comma split has at least this many runes
)

// pieceSpan is one part of a text, in runes. [start, end) is the source span with its whitespace, [lo, hi) the text.
type pieceSpan struct {
	start, lo, hi, end int
	join               string
}

// SplitPieces cuts text into one piece per sentence, splits a sentence over limit runes, and merges fragments.
// Chars sums to UTF16Len(text). Empty text gives nil.
func SplitPieces(text string, limit int) []Piece {
	if limit < 1 {
		limit = 1
	}
	rs := []rune(text)
	var parts []pieceSpan
	for _, sentence := range sentenceSpans(rs) {
		split := splitLongSentence(rs, sentence, limit)
		for i := range split {
			split[i].join = JoinSplit
		}
		split[len(split)-1].join = JoinSentence
		parts = append(parts, split...)
	}
	if len(parts) == 0 {
		return nil
	}
	parts = mergeFragments(rs, parts, limit)
	parts[len(parts)-1].join = ""
	out := make([]Piece, 0, len(parts))
	for _, p := range parts {
		out = append(out, Piece{Text: string(rs[p.lo:p.hi]), Chars: UTF16Len(string(rs[p.start:p.end])), Join: p.join})
	}
	return out
}

// PiecesOf splits a block with the chunk limit of its provider.
func PiecesOf(b NarrationBlock) []Piece {
	return SplitPieces(b.Text, ChunkLimit(b.Voice.Provider))
}

// ChunkLimit is the longest text of one provider call, in runes.
func ChunkLimit(provider string) int {
	switch NormalizeProvider(provider) {
	case "phonon", "sanotts":
		return 2000
	case "google":
		return 200
	}
	return 400
}

// PieceKey is the lower-case hex sha256 of provider|voice|lang|pieceText. Provider and language ignore case, because the
// validator accepts mixed case without rewriting it. Voice and text stay as written.
func PieceKey(v Voice, pieceText string) string {
	provider := NormalizeProvider(v.Provider)
	lang := strings.ToLower(strings.TrimSpace(v.Language))
	sum := sha256.Sum256([]byte(provider + "|" + v.Voice + "|" + lang + "|" + pieceText))
	return hex.EncodeToString(sum[:])
}

// spanOf trims JS whitespace from both ends of [start, end) to find the text range.
func spanOf(rs []rune, start, end int) pieceSpan {
	lo, hi := start, end
	for lo < hi && IsJSSpace(rs[lo]) {
		lo++
	}
	for hi > lo && IsJSSpace(rs[hi-1]) {
		hi--
	}
	return pieceSpan{start: start, lo: lo, hi: hi, end: end}
}

// sentenceSpans cuts the text into contiguous sentences. The whitespace after a sentence belongs to its span.
func sentenceSpans(rs []rune) []pieceSpan {
	var out []pieceSpan
	start := 0
	for i := 0; i < len(rs); {
		stop := sentenceEnd(rs, i)
		if stop < 0 {
			i++
			continue
		}
		for stop < len(rs) && IsJSSpace(rs[stop]) {
			stop++
		}
		out = append(out, spanOf(rs, start, stop))
		start = stop
		i = stop
	}
	if tail := spanOf(rs, start, len(rs)); tail.lo < tail.hi {
		out = append(out, tail)
	}
	return out
}

// sentenceEnd returns the rune index just after the sentence that ends at i, closers included, or -1.
// A dot directly followed by whitespace can stay inside the sentence, see dotContinues.
func sentenceEnd(rs []rune, i int) int {
	r := rs[i]
	switch {
	case isASCIITerminator(r):
		j := i + 1
		for j < len(rs) && isSentenceCloser(rs[j]) {
			j++
		}
		if j < len(rs) && !IsJSSpace(rs[j]) {
			return -1
		}
		if r == '.' && j == i+1 && j < len(rs) && dotContinues(rs, i) {
			return -1
		}
		return j
	case isHardTerminator(r):
		j := i + 1
		for j < len(rs) && (isHardTerminator(rs[j]) || isASCIITerminator(rs[j]) || isSentenceCloser(rs[j])) && !opensNextSentence(rs, j) {
			j++
		}
		return j
	}
	return -1
}

func isASCIITerminator(r rune) bool { return r == '.' || r == '!' || r == '?' }

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// dotContinues says that the dot at i, directly followed by whitespace, stays inside its sentence. It does when the next
// word starts in lower case or with a digit, or when the token before the dot is 1 to 3 digits (an ordinal like "19."), a
// single letter ("z. B.", "e.g.") or a common abbreviation. A year of 4 digits still ends the sentence. A missed sentence
// end costs one longer piece, which still splits at the limit. A false one puts a pause and a new provider call inside
// the sentence. So the rule leans towards not splitting. A combining mark belongs to the token, because the last
// consonant of an Indic word after a vowel sign is no single letter.
func dotContinues(rs []rune, i int) bool {
	next := i + 1
	for next < len(rs) && IsJSSpace(rs[next]) {
		next++
	}
	if next < len(rs) && (unicode.IsLower(rs[next]) || isASCIIDigit(rs[next])) {
		return true
	}
	start := i
	for start > 0 && (unicode.IsLetter(rs[start-1]) || unicode.IsDigit(rs[start-1]) || unicode.IsMark(rs[start-1])) {
		start--
	}
	token := rs[start:i]
	if len(token) == 1 && unicode.IsLetter(token[0]) {
		return true
	}
	digits := 0
	for digits < len(token) && isASCIIDigit(token[digits]) {
		digits++
	}
	if digits == len(token) && digits >= 1 && digits <= 3 {
		return true
	}
	switch strings.ToLower(string(token)) {
	case "dr", "prof", "nr", "bzw", "ca", "usw", "etc", "vs", "mr", "mrs", "st", "bspw", "ggf", "evtl", "inkl", "mio", "mrd", "vgl":
		return true
	}
	return false
}

// isHardTerminator ends a sentence anywhere: ellipsis, CJK stop, fullwidth ! and ?, Arabic stop and question mark,
// danda, double danda, Greek question mark.
func isHardTerminator(r rune) bool {
	switch r {
	case 0x2026, 0x3002, 0xFF01, 0xFF1F, 0x06D4, 0x061F, 0x0964, 0x0965, 0x037E:
		return true
	}
	return false
}

// opensNextSentence says that the quote at j, in the run after a hard terminator, belongs to the next sentence. U+201C,
// U+2018 and U+00AB close a German quote but open an English, Chinese or French one, and Chinese puts no space after its
// stop. So a letter or digit directly after the quote means it opens. The ASCII rule needs no check, because a closer run
// followed by a letter is no sentence end there.
func opensNextSentence(rs []rune, j int) bool {
	switch rs[j] {
	case 0x201C, 0x2018, 0x00AB:
		return j+1 < len(rs) && (unicode.IsLetter(rs[j+1]) || unicode.IsDigit(rs[j+1]))
	}
	return false
}

// isSentenceCloser lists the quotes and brackets that stay on the sentence. U+201C, U+2018 and U+00AB are German closers,
// see opensNextSentence for their other use.
func isSentenceCloser(r rune) bool {
	switch r {
	case 0x22, 0x27, 0x201D, 0x2019, 0x201C, 0x2018, 0x00BB, 0x00AB, 0x203A, 0x29, 0x5D, 0x7D, 0xFF09, 0x300D, 0x300F, 0x3011, 0x3009, 0x300B:
		return true
	}
	return false
}

// isSplitComma lists the comma, Arabic comma, ideographic comma, fullwidth comma, semicolon and fullwidth semicolon.
func isSplitComma(r rune) bool {
	switch r {
	case 0x2C, 0x060C, 0x3001, 0xFF0C, 0x3B, 0xFF1B:
		return true
	}
	return false
}

// splitLongSentence cuts a span over limit runes: at the comma nearest its middle, else at the last whitespace before
// the limit, else at the limit.
func splitLongSentence(rs []rune, sp pieceSpan, limit int) []pieceSpan {
	if sp.hi-sp.lo <= limit {
		return []pieceSpan{sp}
	}
	cut := commaCut(rs, sp)
	if cut < 0 {
		cut = whitespaceCut(rs, sp, limit)
	}
	if cut < 0 {
		cut = sp.lo + limit
	}
	left := splitLongSentence(rs, spanOf(rs, sp.start, cut), limit)
	return append(left, splitLongSentence(rs, spanOf(rs, cut, sp.end), limit)...)
}

// commaCut returns the start of the right part for the best comma, or -1. The comma and its whitespace stay on the left.
func commaCut(rs []rune, sp pieceSpan) int {
	mid := (sp.lo + sp.hi) / 2
	best, bestDist := -1, 0
	for i := sp.lo; i < sp.hi; i++ {
		if !isSplitComma(rs[i]) {
			continue
		}
		cut := i + 1
		for cut < sp.hi && IsJSSpace(rs[cut]) {
			cut++
		}
		if i+1-sp.lo < minSplitRunes || sp.hi-cut < minSplitRunes {
			continue
		}
		dist := i - mid
		if dist < 0 {
			dist = -dist
		}
		if best < 0 || dist < bestDist {
			best, bestDist = cut, dist
		}
	}
	return best
}

// whitespaceCut returns the start of the right part after the last whitespace at or before the limit, or -1.
func whitespaceCut(rs []rune, sp pieceSpan, limit int) int {
	for i := sp.lo + limit; i > sp.lo; i-- {
		if IsJSSpace(rs[i]) {
			cut := i + 1
			for cut < sp.hi && IsJSSpace(rs[cut]) {
				cut++
			}
			return cut
		}
	}
	return -1
}

// mergeFragments merges a fragment into the next part, or into the previous one if it is last, when the merged span
// stays within the limit. A merged part is tested again and keeps the join of its last member.
func mergeFragments(rs []rune, parts []pieceSpan, limit int) []pieceSpan {
	isFragment := func(p pieceSpan) bool {
		text := string(rs[p.lo:p.hi])
		return TextWidth(text) < fragmentWidth || WordCount(text) <= fragmentWords
	}
	merged := func(a, b pieceSpan) pieceSpan {
		return pieceSpan{start: a.start, lo: a.lo, hi: b.hi, end: b.end, join: b.join}
	}
	for i := 0; i < len(parts); {
		switch {
		case !isFragment(parts[i]):
			i++
		case i+1 < len(parts):
			if parts[i+1].hi-parts[i].lo <= limit {
				parts[i] = merged(parts[i], parts[i+1])
				parts = append(parts[:i+1], parts[i+2:]...)
			} else {
				i++
			}
		case i > 0 && parts[i].hi-parts[i-1].lo <= limit:
			parts[i-1] = merged(parts[i-1], parts[i])
			parts = parts[:i]
			i--
		default:
			i++
		}
	}
	return parts
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

// SceneAudio holds the complete blocks of one scene.
type SceneAudio struct {
	Blocks []AudioBlock `json:"blocks"`
}

// AudioInfo is the compile input. Music is never in it.
type AudioInfo struct {
	Narration map[string]SceneAudio `json:"narration"`
}

// MissingBlock names a block with at least one piece lacking audio or a valid envelope.
type MissingBlock struct {
	SceneID string `json:"scene_id"`
	BlockID string `json:"block_id"`
	Chars   int    `json:"chars"` // code points of the missing pieces' text
}

// CharCounts counts code points of piece text, which is what the cloud caps count.
type CharCounts struct {
	Total   int `json:"total"`
	Missing int `json:"missing"`
}

// MusicRef names the stored music of a story.
type MusicRef struct {
	Key        string `json:"key"`
	Ext        string `json:"ext"`
	DurationMS int    `json:"duration_ms"`
}

// AudioAnswer is the body of GET /projects/{id}/narration.
type AudioAnswer struct {
	Narration  map[string]SceneAudio `json:"narration"`
	Missing    []MissingBlock        `json:"missing"`
	Chars      CharCounts            `json:"chars"`
	Music      *MusicRef             `json:"music"`
	MusicIssue string                `json:"music_issue,omitempty"` // "source_missing"
}

// Info returns the part of the answer that compile reads.
func (a AudioAnswer) Info() AudioInfo { return AudioInfo{Narration: a.Narration} }

// ErrMusicSourceMissing reports a file or track source that no longer exists.
var ErrMusicSourceMissing = errors.New("storymaker: the music source is missing")

// MusicKeyResolver returns the key of the story's music. "" means nothing is to be played: mode
// none, no music, volume 0, or mode file with no source chosen yet. A nil resolver means "". A
// file or track source that was set and no longer exists returns ("", ErrMusicSourceMissing).
type MusicKeyResolver func(ctx context.Context, m *Music) (string, error)

// PieceFile is what PieceFiles found for one piece. HasEnv says the file exists; ParseEnvelope
// decides validity.
type PieceFile struct {
	Ext    string
	HasEnv bool
}

var (
	// storedPieceName is the whitelist of PieceFiles. It also skips leftover .aurago-write-* files.
	storedPieceName = regexp.MustCompile(`^([0-9a-f]{64})\.(wav|mp3|env\.json)$`)
	// storedAudioName is the whitelist of AudioPath and OpenAudio. The audio route has its own, narrower one.
	storedAudioName = regexp.MustCompile(`^[0-9a-f]{64}\.(wav|mp3|env\.json|meta\.json|pending\.json)$`)
)

// Size caps of the audio answer. They mirror CAPS.maxLevels and CAPS.maxTotalLevels in ui/js/storymaker/host-app.js
// (checkAudioInfo), which refuses the whole load when one is exceeded. Go enforces them first, by dropping levels.
const (
	MaxBlockLevels = 27000  // levels of one block: 900 s at 30 fps
	MaxTotalLevels = 200000 // levels of one answer
	// maxBlockSpans bounds Spans of one block. Nothing reads Spans today (cue placement uses pieces and speech_ok), and a
	// piece of 10 ms speech and 100 ms gaps makes about 5000 pairs per 600 s of audio.
	maxBlockSpans = 2000
)

// limitAudioBlock keeps the answer small without touching timing. A block whose levels exceed perBlock, or would push the
// running total over all, loses its levels (an empty value, still [] in JSON) and adds nothing to the total: it keeps
// pieces, duration and speech_ok, so only lip-sync and the waveform of that block are lost. A block with more than
// maxBlockSpans spans gets one span per used piece range instead.
func limitAudioBlock(b *AudioBlock, total *int, perBlock, all int) {
	if n := len(b.Levels); n > perBlock || *total+n > all {
		b.Levels = Levels{}
	} else {
		*total += n
	}
	if len(b.Spans) > maxBlockSpans {
		b.Spans = make([][2]int, 0, len(b.Pieces))
		for _, p := range b.Pieces {
			if p.OutMS > p.InMS {
				b.Spans = append(b.Spans, [2]int{p.AtMS, p.AtMS + p.OutMS - p.InMS})
			}
		}
	}
}

// AudioAnswer builds the answer for a decoded story. A block enters Narration only when every piece
// has audio and a valid envelope; otherwise it enters Missing and JS estimates it. A missing music
// source sets MusicIssue and keeps the narration; any other resolver error is returned.
func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error) {
	key := ""
	capture := resolve
	if resolve != nil {
		capture = func(ctx context.Context, m *Music) (string, error) {
			k, err := resolve(ctx, m)
			if err == nil {
				key = k
			}
			return k, err
		}
	}
	answer, err := p.narrationAnswer(ctx, id, s, capture)
	if err != nil || key == "" {
		return answer, err
	}
	// Music stays null unless the key's audio file and a valid meta file both exist. Only the meta
	// is read, so this answer never decodes a track.
	meta, audioExt, err := p.MusicInfo(ctx, id, key)
	if err != nil {
		return AudioAnswer{}, err
	}
	if meta.V != 0 && audioExt != "" {
		answer.Music = &MusicRef{Key: key, Ext: audioExt, DurationMS: meta.DurationMS}
	}
	return answer, nil
}

// narrationAnswer collects the narration part of the answer and the music issue.
func (p *ProjectStore) narrationAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error) {
	files, err := p.PieceFiles(ctx, id)
	if err != nil {
		return AudioAnswer{}, err
	}
	ans := AudioAnswer{Narration: map[string]SceneAudio{}, Missing: []MissingBlock{}}
	totalLevels := 0
	for _, b := range Blocks(s) {
		pieces := PiecesOf(b)
		inputs := make([]AudioPieceInput, 0, len(pieces))
		missing := 0
		for _, piece := range pieces {
			chars := runeLen(piece.Text)
			ans.Chars.Total += chars
			in, ok, err := p.storedPiece(ctx, id, b.Voice, piece, files)
			if err != nil {
				return AudioAnswer{}, err
			}
			if !ok {
				missing += chars
				continue
			}
			inputs = append(inputs, in)
		}
		if missing > 0 {
			ans.Chars.Missing += missing
			ans.Missing = append(ans.Missing, MissingBlock{SceneID: b.SceneID, BlockID: b.ID, Chars: missing})
			continue
		}
		scene := ans.Narration[b.SceneID]
		block := AssembleBlock(b, inputs, s.Format.FPS)
		limitAudioBlock(&block, &totalLevels, MaxBlockLevels, MaxTotalLevels)
		scene.Blocks = append(scene.Blocks, block)
		ans.Narration[b.SceneID] = scene
	}
	// Only the resolver's error counts here. AudioAnswer looks up the key's audio and meta files and
	// sets Music.
	if resolve != nil {
		if _, err := resolve(ctx, s.Music); err != nil {
			if !errors.Is(err, ErrMusicSourceMissing) {
				return AudioAnswer{}, fmt.Errorf("storymaker: music key: %w", err)
			}
			ans.MusicIssue = "source_missing"
		}
	}
	return ans, nil
}

// storedPiece returns the input of a piece whose audio and valid envelope exist. Envelopes are
// ordinary workspace files, so an unreadable or invalid one counts as missing.
func (p *ProjectStore) storedPiece(ctx context.Context, id string, v Voice, piece Piece, files map[string]PieceFile) (AudioPieceInput, bool, error) {
	hash := PieceKey(v, piece.Text)
	f := files[hash]
	if f.Ext == "" || !f.HasEnv {
		return AudioPieceInput{}, false, nil
	}
	data, _, err := p.st.ReadFile(ctx, p.file(id, "audio", "narration", hash+".env.json"))
	if err != nil {
		if ctx.Err() != nil {
			return AudioPieceInput{}, false, ctx.Err()
		}
		return AudioPieceInput{}, false, nil
	}
	env, err := ParseEnvelope(data)
	if err != nil {
		return AudioPieceInput{}, false, nil
	}
	return AudioPieceInput{Hash: hash, Ext: f.Ext, Chars: piece.Chars, Join: piece.Join, Env: env}, true, nil
}

// PieceFiles lists audio/narration and accepts only names matching storedPieceName. When a hash
// has both formats the first in sorted order wins.
func (p *ProjectStore) PieceFiles(ctx context.Context, id string) (map[string]PieceFile, error) {
	if err := p.checkID(id); err != nil {
		return nil, err
	}
	names, err := p.st.List(ctx, p.file(id, "audio", "narration"))
	if err != nil {
		return nil, fmt.Errorf("storymaker: list narration audio: %w", err)
	}
	out := make(map[string]PieceFile, len(names))
	for _, name := range names {
		m := storedPieceName.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		f := out[m[1]]
		if m[2] == "env.json" {
			f.HasEnv = true
		} else if f.Ext == "" {
			f.Ext = m[2]
		}
		out[m[1]] = f
	}
	return out, nil
}

// AudioPath is the desktop path of an audio file, for Publisher.WriteStream. folder is narration
// or music. The name must match storedAudioName, so it also reaches the envelope and music
// bookkeeping files. The audio route keeps its own whitelist for what it serves.
func (p *ProjectStore) AudioPath(id, folder, file string) (string, error) {
	if err := p.checkID(id); err != nil {
		return "", err
	}
	if folder != "narration" && folder != "music" {
		return "", fmt.Errorf("%w: audio folder %q", ErrInvalidArgument, clip(folder, 40))
	}
	if !storedAudioName.MatchString(file) {
		return "", fmt.Errorf("%w: audio file %q", ErrInvalidArgument, clip(file, 80))
	}
	return p.file(id, "audio", folder, file), nil
}

// OpenAudio opens a file named by AudioPath. A missing file wraps fs.ErrNotExist.
func (p *ProjectStore) OpenAudio(ctx context.Context, id, folder, file string) (io.ReadSeekCloser, FileInfo, error) {
	full, err := p.AudioPath(id, folder, file)
	if err != nil {
		return nil, FileInfo{}, err
	}
	return p.st.Open(ctx, full)
}

// WritePiece stores the audio create-only, so an existing file is not an error and stays, then
// writes the envelope.
func (p *ProjectStore) WritePiece(ctx context.Context, id, hash, ext string, data []byte, env Envelope) error {
	full, err := p.pieceAudioPath(id, hash, ext)
	if err != nil {
		return err
	}
	if _, err := p.st.WriteFile(ctx, full, data, MustNotExist); err != nil && !errors.Is(err, ErrConflict) {
		return fmt.Errorf("storymaker: write narration audio: %w", err)
	}
	return p.WriteEnvelope(ctx, id, hash, env)
}

// WriteEnvelope writes or replaces the envelope of a stored piece.
func (p *ProjectStore) WriteEnvelope(ctx context.Context, id, hash string, env Envelope) error {
	full, err := p.AudioPath(id, "narration", hash+".env.json")
	if err != nil {
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("storymaker: encode envelope: %w", err)
	}
	if _, err := p.st.WriteFile(ctx, full, data, ""); err != nil {
		return fmt.Errorf("storymaker: write envelope: %w", err)
	}
	return nil
}

// RemovePiece removes the audio and the envelope of a piece. A missing file is not an error.
func (p *ProjectStore) RemovePiece(ctx context.Context, id, hash, ext string) error {
	full, err := p.pieceAudioPath(id, hash, ext)
	if err != nil {
		return err
	}
	if err := p.st.Remove(ctx, full); err != nil {
		return fmt.Errorf("storymaker: remove narration audio: %w", err)
	}
	env, err := p.AudioPath(id, "narration", hash+".env.json")
	if err != nil {
		return err
	}
	if err := p.st.Remove(ctx, env); err != nil {
		return fmt.Errorf("storymaker: remove envelope: %w", err)
	}
	return nil
}

func (p *ProjectStore) pieceAudioPath(id, hash, ext string) (string, error) {
	if ext != "wav" && ext != "mp3" {
		return "", fmt.Errorf("%w: audio format %q", ErrInvalidArgument, clip(ext, 16))
	}
	return p.AudioPath(id, "narration", hash+"."+ext)
}
