package storymaker

import (
	"slices"
	"strings"
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
