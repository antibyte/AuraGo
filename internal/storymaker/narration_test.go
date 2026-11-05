package storymaker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// crossRow is one row of testdata/narration/cross_language.json, which scripts/storymaker-tests/16-narration.mjs reads too.
type crossRow struct {
	Input      string         `json:"input"`
	Plain      string         `json:"plain"`
	Cues       map[string]int `json:"cues"`
	CharsTotal int            `json:"chars_total"`
	Width      int            `json:"width"`
	Words      int            `json:"words"`
	Pieces     []Piece        `json:"pieces"`
}

func crossRows(t *testing.T) []crossRow {
	t.Helper()
	data, err := os.ReadFile("testdata/narration/cross_language.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []crossRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("the fixture has no rows")
	}
	return rows
}

func TestParseMarkersMatchesCrossLanguageFixture(t *testing.T) {
	for _, row := range crossRows(t) {
		plain, cues := parseMarkers(row.Input)
		if plain != row.Plain {
			t.Errorf("%q: plain %q, want %q", row.Input, plain, row.Plain)
		}
		if got := StripMarkers(row.Input); got != row.Plain {
			t.Errorf("%q: StripMarkers %q, want %q", row.Input, got, row.Plain)
		}
		if len(cues) != len(row.Cues) {
			t.Errorf("%q: cues %v, want %v", row.Input, cues, row.Cues)
		}
		for n, at := range cues {
			if want, ok := row.Cues[strconv.Itoa(n)]; !ok || want != at {
				t.Errorf("%q: cue %d at %d, want %d", row.Input, n, at, want)
			}
		}
		if got := UTF16Len(plain); got != row.CharsTotal {
			t.Errorf("%q: UTF16Len %d, want %d", row.Plain, got, row.CharsTotal)
		}
		if got := TextWidth(plain); got != row.Width {
			t.Errorf("%q: TextWidth %d, want %d", row.Plain, got, row.Width)
		}
		if got := WordCount(plain); got != row.Words {
			t.Errorf("%q: WordCount %d, want %d", row.Plain, got, row.Words)
		}
	}
}

func TestSplitPiecesMatchesCrossLanguageFixture(t *testing.T) {
	for _, row := range crossRows(t) {
		got := SplitPieces(row.Plain, ChunkLimit(""))
		if len(got) != len(row.Pieces) {
			t.Errorf("%q: %d pieces, want %d: %+v", row.Plain, len(got), len(row.Pieces), got)
			continue
		}
		sum := 0
		for i, p := range got {
			if p != row.Pieces[i] {
				t.Errorf("%q piece %d: got %+v, want %+v", row.Plain, i, p, row.Pieces[i])
			}
			sum += p.Chars
		}
		if sum != UTF16Len(row.Plain) {
			t.Errorf("%q: chars sum to %d, want %d", row.Plain, sum, UTF16Len(row.Plain))
		}
	}
}

func TestSplitPiecesSentencesAndMerges(t *testing.T) {
	// long has 37 runes and 7 words, other has 39 runes and 7 words, so neither is a fragment.
	const long = "Das ist ein etwas längerer Satz hier."
	const other = "Und noch ein weiterer langer Satz hier."
	cases := []struct {
		name  string
		text  string
		limit int
		want  []Piece
	}{
		// "Genau." has width 6. 6 + space + 37 = 44.
		{"a first fragment merges into the next", "Genau. " + long, 400, []Piece{{"Genau. " + long, 44, ""}}},
		{"a last fragment merges into the previous", long + " Genau.", 400, []Piece{{long + " Genau.", 44, ""}}},
		// 37 + space = 38. "Ja." + space + 39 = 43.
		{"a middle fragment merges into the next", long + " Ja. " + other, 400, []Piece{{long, 38, JoinSentence}, {"Ja. " + other, 43, ""}}},
		// The merged span of 44 runes is over the limit of 40, so "Genau." stays. 6 + space = 7.
		{"no merge over the limit", "Genau. " + long, 40, []Piece{{"Genau.", 7, JoinSentence}, {long, 37, ""}}},
		// The first sentence has 36 runes, with its closing quote. 36 + space = 37. The second has 32 runes.
		{"closers stay on the sentence", `Er sagt es ganz laut: "Das ist gut!" Dann geht er langsam nach Hause.`, 400, []Piece{
			{`Er sagt es ganz laut: "Das ist gut!"`, 37, JoinSentence}, {"Dann geht er langsam nach Hause.", 32, ""}}},
		// The dot of 3.14 is followed by a digit. The first sentence has 34 runes, 34 + space = 35. The second has 24.
		{"a decimal point is no sentence end", "Version 3.14 ist da und läuft gut. Das freut uns alle sehr.", 400, []Piece{
			{"Version 3.14 ist da und läuft gut.", 35, JoinSentence}, {"Das freut uns alle sehr.", 24, ""}}},
		// The next five stay one piece. Cut at the inner dot, both sides would be long enough not to merge back.
		// 34 + space + 45 = 80.
		{"an ordinal is no sentence end", "Die Firewall wurde am Ende des 19. Jahrhunderts in einem kleinen Labor erfunden.", 400, []Piece{
			{"Die Firewall wurde am Ende des 19. Jahrhunderts in einem kleinen Labor erfunden.", 80, ""}}},
		// 28 + space + 35 = 64. "z" and "B" are single letters.
		{"a single letter is no sentence end", "Viele Geräte im Netzwerk, z. B. Router und Switches, nutzen das.", 400, []Piece{
			{"Viele Geräte im Netzwerk, z. B. Router und Switches, nutzen das.", 64, ""}}},
		// 24 + space + 35 = 60.
		{"a title is no sentence end", "Im kleinen Labor von Dr. Müller entstand die erste Firewall.", 400, []Piece{
			{"Im kleinen Labor von Dr. Müller entstand die erste Firewall.", 60, ""}}},
		// 40 + space + 31 = 72.
		{"an abbreviation is no sentence end", "Das kleine Gerät liegt im Handel bei ca. 20 Euro und damit sehr günstig.", 400, []Piece{
			{"Das kleine Gerät liegt im Handel bei ca. 20 Euro und damit sehr günstig.", 72, ""}}},
		// 39 + space + 29 = 69. The first dot of "e.g." is followed by a letter, the second by a lower-case word.
		{"a dot before a lower-case word is no sentence end", "Many small devices on the network, e.g. routers and switches, use it.", 400, []Piece{
			{"Many small devices on the network, e.g. routers and switches, use it.", 69, ""}}},
		// A year has four digits and ends the sentence. 25 + space = 26. The second has 31 runes.
		{"a year ends the sentence", "Das geschah im Jahr 1990. Danach wurde alles ganz anders.", 400, []Piece{
			{"Das geschah im Jahr 1990.", 26, JoinSentence}, {"Danach wurde alles ganz anders.", 31, ""}}},
		// A closer follows the dot, so the dot still ends the sentence. 38 + space = 39. The second has 31 runes.
		{"a dot before a closer ends the sentence", `Das Kapitel heißt einfach "Nummer 19." Danach wurde alles ganz anders.`, 400, []Piece{
			{`Das Kapitel heißt einfach "Nummer 19."`, 39, JoinSentence}, {"Danach wurde alles ganz anders.", 31, ""}}},
		// U+201C closes a German quote, so the dot before it ends the sentence and the quote stays on it. 47 + space = 48.
		// The second has 32 runes.
		{"a German closing quote ends the sentence", "Er sagte ganz laut und deutlich: „Das ist gut.“ Dann ging er langsam nach Hause.", 400, []Piece{
			{"Er sagte ganz laut und deutlich: „Das ist gut.“", 48, JoinSentence}, {"Dann ging er langsam nach Hause.", 32, ""}}},
		// U+2018 closes the single form. Again 47 + space = 48.
		{"a German single closing quote ends the sentence", "Er sagte ganz laut und deutlich: ‚Das ist gut!‘ Dann ging er langsam nach Hause.", 400, []Piece{
			{"Er sagte ganz laut und deutlich: ‚Das ist gut!‘", 48, JoinSentence}, {"Dann ging er langsam nach Hause.", 32, ""}}},
		// U+00AB closes a German chevron quote. It stays on the sentence the ellipsis ended, so the next piece has no stray
		// quote. 53 + space = 54.
		{"a hard terminator keeps a German closer", "Sie rief ihm ganz laut und deutlich zu: »Warte doch…« Dann ging er langsam nach Hause.", 400, []Piece{
			{"Sie rief ihm ganz laut und deutlich zu: »Warte doch…«", 54, JoinSentence}, {"Dann ging er langsam nach Hause.", 32, ""}}},
		// U+201C opens a Chinese quote and no space follows the stop, so the quote starts the next sentence. 13 runes each.
		{"a quote that opens after a hard terminator goes to the next sentence", "他说今天的天气真的非常好。“明天的天气也会这样好。”", 400, []Piece{
			{"他说今天的天气真的非常好。", 13, JoinSentence}, {"“明天的天气也会这样好。”", 13, ""}}},
		// The vowel sign belongs to the word, so its last consonant is no single letter. 22 + space = 23. The second has 24.
		{"a consonant after a vowel sign is no single letter", "आज हमने बहुत सारा काम. कल फिर से बहुत सारा काम.", 400, []Piece{
			{"आज हमने बहुत सारा काम.", 23, JoinSentence}, {"कल फिर से बहुत सारा काम.", 24, ""}}},
	}
	for _, c := range cases {
		got := SplitPieces(c.text, c.limit)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s piece %d: got %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestSplitPiecesKeepsHanSentencesApart(t *testing.T) {
	// 11 characters: width 22, words ceil(11/2) = 6, so it is no fragment and nothing merges up to the limit.
	got := SplitPieces(strings.Repeat("今日は天気がいいです。", 5), 400)
	if len(got) != 5 {
		t.Fatalf("%d pieces, want 5: %+v", len(got), got)
	}
}

func TestSplitPiecesLongSentences(t *testing.T) {
	// words(n) has 5n-1 runes.
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("wort ", n)) }

	// Runes: 49, ", ", 44, ", ", 54, "." = 152, so the middle is 76. The commas stand at 49 (distance 27) and 95 (distance 19).
	// The left part keeps its comma: 96 runes, 97 units with the space. The right part has 55.
	text := words(10) + ", " + words(9) + ", " + words(11) + "."
	assertPieces(t, "comma", SplitPieces(text, 100), []Piece{
		{words(10) + ", " + words(9) + ",", 97, JoinSplit},
		{words(11) + ".", 55, ""},
	})

	// 120 words are 599 runes with no comma. "Google" has limit 200. The spaces stand at 5k-1, so the cuts fall at 199 and 399.
	// Three parts of 199 runes (40 words each). The first two also own a space.
	block := NarrationBlock{Voice: Voice{Provider: "Google"}, Text: words(120)}
	assertPieces(t, "whitespace", PiecesOf(block), []Piece{
		{words(40), 200, JoinSplit},
		{words(40), 200, JoinSplit},
		{words(40), 199, ""},
	})

	// 250 astral characters with no comma and no space: cuts at code points, 2 UTF-16 units each.
	kichi := string(rune(0x20BB7))
	assertPieces(t, "code point", SplitPieces(strings.Repeat(kichi, 250), 100), []Piece{
		{strings.Repeat(kichi, 100), 200, JoinSplit},
		{strings.Repeat(kichi, 100), 200, JoinSplit},
		{strings.Repeat(kichi, 50), 100, ""},
	})
}

func assertPieces(t *testing.T, name string, got, want []Piece) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d pieces, want %d: %+v", name, len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s piece %d: got %+v, want %+v", name, i, got[i], want[i])
		}
		if runeLen(got[i].Text) > 200 {
			t.Errorf("%s piece %d has %d runes", name, i, runeLen(got[i].Text))
		}
	}
}

func TestSplitPiecesEmptyAndChunkLimit(t *testing.T) {
	if got := SplitPieces("", 400); got != nil {
		t.Errorf("empty text gave %+v", got)
	}
	if got := SplitPieces("   ", 400); got != nil {
		t.Errorf("whitespace gave %+v", got)
	}
	for provider, want := range map[string]int{"Phonon ": 2000, "SANOTTS": 2000, "Google": 200, "elevenlabs": 400, "": 400} {
		if got := ChunkLimit(provider); got != want {
			t.Errorf("ChunkLimit(%q) = %d, want %d", provider, got, want)
		}
	}
}

func TestPieceKey(t *testing.T) {
	sum := sha256.Sum256([]byte("google|Kore|de-de|Hallo Welt."))
	want := hex.EncodeToString(sum[:])
	if got := PieceKey(Voice{Provider: "google", Voice: "Kore", Language: "de-de"}, "Hallo Welt."); got != want {
		t.Errorf("key %s, want %s", got, want)
	}
	if got := PieceKey(Voice{Provider: " Google ", Voice: "Kore", Language: "DE-de "}, "Hallo Welt."); got != want {
		t.Errorf("provider and language case changed the key: %s", got)
	}
	if PieceKey(Voice{Provider: "google", Voice: "kore", Language: "de-de"}, "Hallo Welt.") == want {
		t.Error("the voice must stay as written")
	}
	if PieceKey(Voice{Provider: "google", Voice: "Kore", Language: "de-de"}, "hallo welt.") == want {
		t.Error("the piece text must stay as written")
	}
}

func TestBlocks(t *testing.T) {
	annaVoice := Voice{Provider: "Phonon", Voice: "nele", Language: "de"}
	story := &Story{
		Voice: &Voice{Provider: " ElevenLabs ", Voice: "v1", Language: "de"},
		Cast: []CastMember{
			{ID: "anna", Character: "char.a", Voice: &annaVoice},
			{ID: "ben", Character: "char.b"},
		},
		Scenes: []Scene{
			{ID: "s1", Narration: "Hallo {1}Welt.", Beats: []Beat{
				{Do: "say", Who: "anna", Text: "Hi {2}du."},
				{Do: "wait", Seconds: 1},
				{Do: "say", Who: "ben", Text: "   "},
				{Do: "say", Who: "ben", Text: "{3}"},
				{Do: "say", Who: "anna"},
				// ben is not in the scene's actors, so he is off stage. His audio still plays.
				{Do: "say", Who: "ben", Text: "Ich bin nicht zu sehen."},
			}},
			{ID: "s2", Silent: true, Narration: "Wird nicht gesprochen.", Beats: []Beat{{Do: "say", Who: "anna", Text: "Trotzdem."}}},
			{ID: "s3", Kind: "code", Speaker: "anna", Narration: "Der Code spricht.", Beats: []Beat{{Do: "say", Who: "anna", Text: "Nie."}}},
			{ID: "s4", Narration: "  {1}  "},
		},
	}
	storyVoice := Voice{Provider: "elevenlabs", Voice: "v1", Language: "de"}
	cast := Voice{Provider: "phonon", Voice: "nele", Language: "de"}
	want := []NarrationBlock{
		{ID: "s1:n", Kind: BlockKindNarration, SceneID: "s1", SceneIndex: 0, Beat: -1, Speaker: "narrator", Voice: storyVoice, Text: "Hallo Welt."},
		{ID: "s1:b0", Kind: BlockKindDialogue, SceneID: "s1", SceneIndex: 0, Beat: 0, Speaker: "anna", Voice: cast, Text: "Hi du."},
		{ID: "s1:b5", Kind: BlockKindDialogue, SceneID: "s1", SceneIndex: 0, Beat: 5, Speaker: "ben", Voice: storyVoice, Text: "Ich bin nicht zu sehen."},
		{ID: "s2:b0", Kind: BlockKindDialogue, SceneID: "s2", SceneIndex: 1, Beat: 0, Speaker: "anna", Voice: cast, Text: "Trotzdem."},
		{ID: "s3:n", Kind: BlockKindNarration, SceneID: "s3", SceneIndex: 2, Beat: -1, Speaker: "anna", Voice: cast, Text: "Der Code spricht."},
	}
	got := Blocks(story)
	if len(got) != len(want) {
		t.Fatalf("%d blocks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

func TestBlocksWithoutAVoice(t *testing.T) {
	got := Blocks(&Story{Scenes: []Scene{{ID: "s1", Narration: "Hallo."}}})
	if len(got) != 1 || got[0].Voice.Provider != "" || got[0].Speaker != "narrator" {
		t.Fatalf("got %+v", got)
	}
}
