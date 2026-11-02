package storymaker

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mustDecode(t *testing.T, data string) Story {
	t.Helper()
	s, issues, err := DecodeStory([]byte(data), DefaultLimits())
	if err != nil {
		t.Fatalf("DecodeStory: %v (%+v)", err, issues)
	}
	if len(issues) != 0 {
		t.Fatalf("issues on success: %+v", issues)
	}
	return s
}

func decodeFail(t *testing.T, data string) []Issue {
	t.Helper()
	_, issues, err := DecodeStory([]byte(data), DefaultLimits())
	if err == nil {
		t.Fatalf("expected an error for %.80q", data)
	}
	var de *DecodeError
	if !errors.As(err, &de) || len(issues) == 0 || len(de.Issues) != len(issues) {
		t.Fatalf("error %v must be a *DecodeError with issues, got %+v", err, issues)
	}
	return issues
}

func TestDecodeFixtures(t *testing.T) {
	for _, name := range []string{"minimal", "firewall"} {
		data, err := os.ReadFile("testdata/stories/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		s, issues, err := DecodeStory(data, DefaultLimits())
		if err != nil || len(issues) != 0 {
			t.Fatalf("%s: %v %+v", name, err, issues)
		}
		if s.Schema != Schema || len(s.Scenes) == 0 {
			t.Fatalf("%s: %+v", name, s)
		}
	}
	data, _ := os.ReadFile("testdata/stories/firewall.json")
	s := mustDecode(t, string(data))
	if s.Title != "Was ist eine Firewall?" || len(s.Cast) != 1 || len(s.Scenes) != 3 {
		t.Fatalf("firewall: %+v", s)
	}
	a := s.Scenes[0].Actors[0]
	if a.At == nil || a.At.Spot != "desk" || a.Pose != "typing" {
		t.Fatalf("actor: %+v", a)
	}
	if b := s.Scenes[0].Beats[0]; b.Cue != 1 || b.Do != "emote" || b.Expression != "surprised" {
		t.Fatalf("beat: %+v", b)
	}
	if !strings.Contains(s.Scenes[0].Narration, "{1}Plötzlich") || s.Scenes[2].Kind != "code" {
		t.Fatalf("scenes: %+v", s.Scenes)
	}
}

func TestDecodeUnknownFieldsHaveHintsAndPaths(t *testing.T) {
	story := `{"schema":"storymaker/1","title":"T","scenes":[
	  {"id":"s1","text":"Hallo","actors":[{"character":"anna","position":"left"}],
	   "beats":[{"type":"walk","actor":"anna","destination":"door"},{"do":"wait","seconds":1,"pause":2}]}]}`
	issues := decodeFail(t, story)
	byPath := map[string]Issue{}
	for _, is := range issues {
		if is.Code != CodeUnknownField || is.Severity != SeverityError {
			t.Fatalf("issue %+v", is)
		}
		byPath[is.Path] = is
	}
	want := map[string]string{
		"/scenes/0/text":                `"narration"`,
		"/scenes/0/actors/0/character":  `"cast"`,
		"/scenes/0/actors/0/position":   `"at"`,
		"/scenes/0/beats/0/type":        `"do"`,
		"/scenes/0/beats/0/actor":       `"who"`,
		"/scenes/0/beats/0/destination": `"to"`,
		"/scenes/0/beats/1/pause":       "one of the fields of a beat",
	}
	if len(byPath) != len(want) {
		t.Fatalf("want %d issues, got %d: %+v", len(want), len(byPath), issues)
	}
	for path, hint := range want {
		is, ok := byPath[path]
		if !ok {
			t.Errorf("no issue at %s", path)
			continue
		}
		if !strings.Contains(is.Fix, hint) {
			t.Errorf("%s: fix %q lacks %q", path, is.Fix, hint)
		}
	}
	// Document order: scene 0 fields before beats, beat 0 before beat 1.
	if issues[len(issues)-1].Path != "/scenes/0/beats/1/pause" {
		t.Errorf("issues not in document order: %+v", issues)
	}
}

func TestDecodeKeyMatchingIsExact(t *testing.T) {
	// encoding/json would accept "Title" for "title"; the format does not.
	issues := decodeFail(t, `{"schema":"storymaker/1","Title":"x","scenes":[]}`)
	if issues[0].Code != CodeUnknownField || issues[0].Path != "/Title" {
		t.Fatalf("%+v", issues)
	}
}

func TestDecodeFailureCodes(t *testing.T) {
	cases := []struct {
		name, data, code, path, msg string
	}{
		{"empty", "  ", CodeInvalidJSON, "", "empty"},
		{"truncated", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"`, CodeInvalidJSON, "", ""},
		{"syntax", "{\n  \"schema\": \"storymaker/1\",\n  \"title\" \"x\"\n}", CodeInvalidJSON, "", "line 3"},
		{"two objects", `{"schema":"storymaker/1"} {"a":1}`, CodeInvalidJSON, "", "more data"},
		{"array", `[]`, CodeInvalidJSON, "", "JSON object"},
		{"no schema", `{"title":"x"}`, CodeSchemaMismatch, "/schema", "must be"},
		{"other schema", `{"schema":"storymaker/2","title":"x"}`, CodeSchemaMismatch, "/schema", "storymaker/2"},
		{"fps as string", `{"schema":"storymaker/1","title":"x","format":{"aspect":"16:9","fps":"30"},"scenes":[]}`, CodeInvalidJSON, "/format/fps", "expected a number"},
		{"scenes as object", `{"schema":"storymaker/1","title":"x","scenes":{}}`, CodeInvalidJSON, "/scenes", "an array"},
		{"scene id as number", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"},{"id":5}]}`, CodeInvalidJSON, "/scenes/1/id", "a string"},
		{"nested beat count", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1","beats":[{"do":"wait"},{"do":"gesture","count":"two"}]}]}`, CodeInvalidJSON, "/scenes/0/beats/1/count", "a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := decodeFail(t, tc.data)
			is := issues[0]
			if is.Code != tc.code || is.Path != tc.path || is.Severity != SeverityError || !strings.Contains(is.Message, tc.msg) {
				t.Fatalf("got %+v, want code %s path %q msg containing %q", is, tc.code, tc.path, tc.msg)
			}
		})
	}
}

func TestDecodeTooLarge(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxStoryBytes = 50
	_, issues, err := DecodeStory([]byte(`{"schema":"storymaker/1","title":"`+strings.Repeat("x", 100)+`","scenes":[]}`), lim)
	if err == nil || issues[0].Code != CodeTooLarge {
		t.Fatalf("%v %+v", err, issues)
	}
}

func TestDecodeStripsBOM(t *testing.T) {
	mustDecode(t, "\xef\xbb\xbf"+`{"schema":"storymaker/1","title":"x","scenes":[]}`)
}

func TestPolymorphicPositionAndDuration(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"x","scenes":[
	  {"id":"s1","duration":"auto","actors":[{"cast":"a","at":"desk"},{"cast":"b","at":{"x":12.5,"y":80}},{"cast":"c","at":{"x":1}},{"cast":"d","at":7}]},
	  {"id":"s2","duration":6.5},{"id":"s3","duration":"soon"},{"id":"s4"}]}`)
	ac := s.Scenes[0].Actors
	if ac[0].At.Spot != "desk" || ac[0].At.Point {
		t.Errorf("spot: %+v", ac[0].At)
	}
	if !ac[1].At.Point || ac[1].At.X != 12.5 || ac[1].At.Y != 80 {
		t.Errorf("point: %+v", ac[1].At)
	}
	if len(ac[2].At.Invalid) == 0 || len(ac[3].At.Invalid) == 0 {
		t.Errorf("invalid shapes must be kept: %+v %+v", ac[2].At, ac[3].At)
	}
	if !s.Scenes[0].Duration.Auto || s.Scenes[1].Duration.Seconds != 6.5 || len(s.Scenes[2].Duration.Invalid) == 0 || s.Scenes[3].Duration != nil {
		t.Errorf("durations: %+v %+v %+v %+v", s.Scenes[0].Duration, s.Scenes[1].Duration, s.Scenes[2].Duration, s.Scenes[3].Duration)
	}
	out, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"at": "desk"`, `"duration": "auto"`, `"duration": 6.5`, `"duration": "soon"`, `"at": 7`, `"x": 12.5`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("canonical JSON lacks %s:\n%s", want, out)
		}
	}
	// What was saved is read back unchanged.
	again := mustDecode(t, string(out))
	if out2, _ := CanonicalJSON(again); !bytes.Equal(out, out2) {
		t.Errorf("round trip changed the story:\n%s\n%s", out, out2)
	}
	if !reflect.DeepEqual(s.Scenes, again.Scenes) {
		t.Errorf("round trip changed the scenes:\n%+v\n%+v", s.Scenes, again.Scenes)
	}
}

func TestStyleCaptionsDefaultOnWhenOmitted(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"x","style":{"palette":"calm"},"scenes":[]}`)
	if !s.Style.Captions || s.Style.Palette != "calm" {
		t.Fatalf("%+v", s.Style)
	}
	s = mustDecode(t, `{"schema":"storymaker/1","title":"x","style":{"captions":false},"scenes":[]}`)
	if s.Style.Captions {
		t.Fatalf("explicit false lost: %+v", s.Style)
	}
	s = mustDecode(t, `{"schema":"storymaker/1","title":"x","scenes":[]}`)
	if s.Style != (Style{}) {
		t.Fatalf("absent style must stay zero so ApplyDefaults can fill it: %+v", s.Style)
	}
}

func TestCanonicalJSONIsStableAndRoundTrips(t *testing.T) {
	for _, name := range []string{"minimal", "firewall"} {
		data, _ := os.ReadFile("testdata/stories/" + name + ".json")
		s := mustDecode(t, string(data))
		a, err := CanonicalJSON(s)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := CanonicalJSON(s)
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: not stable", name)
		}
		if !bytes.HasSuffix(a, []byte("}\n")) || !strings.HasPrefix(string(a), "{\n  \"schema\": \"storymaker/1\",\n  \"title\":") {
			t.Fatalf("%s: unexpected layout:\n%.200s", name, a)
		}
		again := mustDecode(t, string(a))
		if !reflect.DeepEqual(s, again) {
			t.Fatalf("%s: round trip changed the story", name)
		}
		// Same JSON value as the fixture.
		var x, y any
		_ = json.Unmarshal(data, &x)
		_ = json.Unmarshal(a, &y)
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("%s: canonical JSON differs from the fixture:\n%s", name, a)
		}
	}
	// No HTML escaping, and empty collections are [] not null.
	out, _ := CanonicalJSON(Story{Schema: Schema, Title: "A & B <c>"})
	if !strings.Contains(string(out), `"A & B <c>"`) || !strings.Contains(string(out), `"cast": []`) || !strings.Contains(string(out), `"scenes": []`) {
		t.Fatalf("%s", out)
	}
}

func TestPointerAtOffsetEscapesAndIndexes(t *testing.T) {
	data := `{"a/b":{"c~d":[0,{"e":"x"}]}}`
	off := int64(strings.Index(data, `"x"`) + 3)
	if got := pointerAtOffset([]byte(data), off); got != "/a~1b/c~0d/1/e" {
		t.Fatalf("pointer = %q", got)
	}
}
