package storymaker

import (
	"encoding/json"
	"os"
	"strconv"
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
