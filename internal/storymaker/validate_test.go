package storymaker

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// storyOf wraps scene JSON into a valid story with one cast member (anna).
func storyOf(t *testing.T, scenes string) Story {
	t.Helper()
	return mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},`+
		`"style":{"palette":"sunny","font":"default","captions":true},`+
		`"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],"scenes":[`+scenes+`]}`)
}

func validateScenes(t *testing.T, scenes string) []Issue {
	t.Helper()
	s := storyOf(t, scenes)
	return Validate(&s, loadMini(t), DefaultLimits())
}

func findIssue(issues []Issue, code, severity, path string) *Issue {
	for i := range issues {
		if issues[i].Code == code && issues[i].Severity == severity && issues[i].Path == path {
			return &issues[i]
		}
	}
	return nil
}

func dump(issues []Issue) string {
	var b strings.Builder
	for _, is := range issues {
		fmt.Fprintf(&b, "\n  %s %s %s: %s | %s", is.Severity, is.Code, is.Path, is.Message, is.Fix)
	}
	return b.String()
}

func expectIssue(t *testing.T, issues []Issue, code, severity, path string) *Issue {
	t.Helper()
	is := findIssue(issues, code, severity, path)
	if is == nil {
		t.Fatalf("no %s %s at %s in:%s", severity, code, path, dump(issues))
	}
	return is
}

func TestValidateFixturesAreClean(t *testing.T) {
	cat := loadMini(t)
	for _, name := range []string{"minimal", "firewall"} {
		data, _ := os.ReadFile("testdata/stories/" + name + ".json")
		s := mustDecode(t, string(data))
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s: %s", name, dump(issues))
		}
		ApplyDefaults(&s)
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s after defaults: %s", name, dump(issues))
		}
	}
}

func TestValidateIssueShape(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","environment":"env.offce","narration":"x","actors":[{"cast":"anna","at":"balcony"}]}`)
	for _, is := range issues {
		if is.Severity == "" || is.Code == "" || is.Message == "" || !strings.HasPrefix(is.Path, "/") {
			t.Errorf("incomplete issue %+v", is)
		}
		if is.SceneID != "s1" {
			t.Errorf("scene id missing: %+v", is)
		}
	}
}

func TestValidateReferences(t *testing.T) {
	cases := []struct {
		name, scenes, code, severity, path string
		fix                                string // substring of the fix
	}{
		{"unknown environment", `{"id":"s1","environment":"env.offce"}`, CodeUnknownAsset, SeverityError, "/scenes/0/environment", "env.office"},
		{"environment without prefix", `{"id":"s1","environment":"office"}`, CodeUnknownAsset, SeverityError, "/scenes/0/environment", "env.office"},
		{"environment wrong kind", `{"id":"s1","environment":"prop.lock"}`, CodeInvalidValue, SeverityError, "/scenes/0/environment", "env."},
		{"unknown prop item", `{"id":"s1","props":[{"item":"prop.padlock"}]}`, CodeUnknownAsset, SeverityError, "/scenes/0/props/0/item", "prop.lock"},
		{"prop missing item", `{"id":"s1","props":[{"at":"left"}]}`, CodeMissingField, SeverityError, "/scenes/0/props/0/item", ""},
		{"unknown show item", `{"id":"s1","beats":[{"do":"show","item":"prop.rocket","at":"left","entry":"pop"}]}`, CodeUnknownAsset, SeverityError, "/scenes/0/beats/0/item", "prop."},
		{"unknown spot", `{"id":"s1","environment":"env.office","actors":[{"cast":"anna","at":"balcony"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "desk"},
		{"spot of another environment", `{"id":"s1","environment":"env.plain-stage","actors":[{"cast":"anna","at":"desk"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "center"},
		{"spot inherited from the previous scene", `{"id":"s1","environment":"env.office"},{"id":"s2","actors":[{"cast":"anna","at":"desk"}]}`, "", "", "", ""},
		{"project background keeps three spots", `{"id":"s1","environment":"my.bg.park","actors":[{"cast":"anna","at":"desk"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "left"},
		{"point out of range", `{"id":"s1","actors":[{"cast":"anna","at":{"x":120,"y":50}}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", ""},
		{"position wrong shape", `{"id":"s1","actors":[{"cast":"anna","at":{"x":50}}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", "center"},
		{"empty spot", `{"id":"s1","actors":[{"cast":"anna","at":""}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", ""},
		{"actor of unknown cast", `{"id":"s1","actors":[{"cast":"bob"}]}`, CodeUnknownCast, SeverityError, "/scenes/0/actors/0/cast", "anna"},
		{"actor without cast", `{"id":"s1","actors":[{"at":"left"}]}`, CodeMissingField, SeverityError, "/scenes/0/actors/0/cast", ""},
		{"same actor twice", `{"id":"s1","actors":[{"cast":"anna"},{"cast":"anna"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/actors/1/cast", ""},
		{"unknown speaker", `{"id":"s1","speaker":"bob"}`, CodeUnknownCast, SeverityError, "/scenes/0/speaker", "anna"},
		{"beat of unknown cast", `{"id":"s1","beats":[{"do":"pose","who":"bob","pose":"stand"}]}`, CodeUnknownCast, SeverityError, "/scenes/0/beats/0/who", "anna"},
		{"beat of an actor who is not on stage", `{"id":"s1","beats":[{"do":"pose","who":"anna","pose":"stand"}]}`, CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who", "enter"},
		{"enter puts the actor on stage", `{"id":"s1","beats":[{"do":"enter","who":"anna","from":"left","to":"center"},{"do":"pose","who":"anna","pose":"stand"}]}`, "", "", "", ""},
		{"exit takes the actor off stage", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"exit","who":"anna","to":"left"},{"do":"pose","who":"anna","pose":"stand"}]}`, CodeUnknownCast, SeverityWarning, "/scenes/0/beats/1/who", ""},
		{"cue without marker", `{"id":"s1","narration":"Eins. {1}Zwei.","beats":[{"do":"wait","seconds":1,"cue":2}]}`, CodeUnknownCue, SeverityError, "/scenes/0/beats/0/cue", "{1}"},
		{"cue without any narration", `{"id":"s1","beats":[{"do":"wait","seconds":1,"cue":1}]}`, CodeUnknownCue, SeverityError, "/scenes/0/beats/0/cue", "no markers"},
		{"cue with marker", `{"id":"s1","narration":"Eins. {2}Zwei.","beats":[{"do":"wait","seconds":1,"cue":2}]}`, "", "", "", ""},
		{"target outside the scene", `{"id":"s1","beats":[{"do":"hide","target":"prop.lock"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/target", "camera"},
		{"target is an item shown by a beat", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"prop.lock"}]}`, "", "", "", ""},
		{"target is the derived instance id", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"lock-1"}]}`, "", "", "", ""},
		{"ambiguous item target", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"left","entry":"pop"},{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"prop.lock"}]}`, CodeInvalidValue, SeverityError, "/scenes/0/beats/2/target", "lock-1"},
		{"look at a spot", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"look","who":"anna","target":"center"}]}`, "", "", "", ""},
		{"duplicate scene id", `{"id":"s1"},{"id":"s1"}`, CodeDuplicateID, SeverityError, "/scenes/1/id", ""},
		{"bad scene id", `{"id":"Scene 1"}`, CodeInvalidValue, SeverityError, "/scenes/0/id", ""},
		{"missing scene id", `{"narration":"x"}`, CodeMissingField, SeverityError, "/scenes/0/id", ""},
		{"duplicate prop id", `{"id":"s1","props":[{"id":"a","item":"prop.lock"},{"id":"a","item":"prop.shield"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/props/1/id", ""},
		{"show id collides with a prop id", `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"show","id":"a","item":"prop.shield","at":"left","entry":"pop"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/beats/0/id", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateScenes(t, tc.scenes)
			if tc.code == "" {
				for _, is := range issues {
					if is.Severity != SeverityMinor {
						t.Fatalf("unexpected issues:%s", dump(issues))
					}
				}
				return
			}
			is := expectIssue(t, issues, tc.code, tc.severity, tc.path)
			if !strings.Contains(is.Fix, tc.fix) {
				t.Fatalf("fix %q lacks %q", is.Fix, tc.fix)
			}
		})
	}
}

func TestValidateAlternativesAreLimitedAndInTheFieldForm(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna","pose":"typin","expression":"suprised"}],"beats":[{"do":"dance","who":"anna"}]}`)
	pose := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/actors/0/pose")
	if !strings.Contains(pose.Fix, `Use one of: typing, stand.`) || !strings.Contains(pose.Fix, `{"pose":"typing"}`) {
		t.Errorf("pose fix: %s", pose.Fix)
	}
	expr := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/actors/0/expression")
	if !strings.Contains(expr.Fix, "surprised") || strings.Contains(expr.Fix, "expr.") {
		t.Errorf("expression fix must offer bare names: %s", expr.Fix)
	}
	do := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/beats/0/do")
	list := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(do.Fix, ". Example:", 2)[0], "Use one of: "), ".")
	if n := len(strings.Split(list, ", ")); n != 5 {
		t.Errorf("a beat kind offers at most 5 alternatives, got %d: %s", n, do.Fix)
	}
}

func TestValidateNames(t *testing.T) {
	cases := []struct {
		name, scenes, code, severity, path string
	}{
		// Closed names the catalog lacks: minor, never an error.
		{"pose missing from the catalog", `{"id":"s1","actors":[{"cast":"anna","pose":"facepalm"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/actors/0/pose"},
		{"expression missing from the catalog", `{"id":"s1","actors":[{"cast":"anna","expression":"laugh"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/actors/0/expression"},
		{"gesture missing from the catalog", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"gesture","who":"anna","gesture":"wave"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0/gesture"},
		// Unknown names: error.
		{"unknown pose", `{"id":"s1","actors":[{"cast":"anna","pose":"flying"}]}`, CodeUnknownName, SeverityError, "/scenes/0/actors/0/pose"},
		{"unknown gesture", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"gesture","who":"anna","gesture":"dab"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/gesture"},
		{"unknown transition", `{"id":"s1","transition":"spin"}`, CodeUnknownName, SeverityError, "/scenes/0/transition"},
		{"unknown ease", `{"id":"s1","beats":[{"do":"wait","seconds":1,"ease":"bouncy"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/ease"},
		{"unknown text style", `{"id":"s1","beats":[{"do":"show","text":"Hi","style":"huge","at":"center","entry":"fade"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/style"},
		// ID form is canonicalised with a note.
		{"pose in id form", `{"id":"s1","actors":[{"cast":"anna","pose":"pose.typing"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/actors/0/pose"},
		{"expression in id form", `{"id":"s1","actors":[{"cast":"anna","expression":"expr.happy"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/actors/0/expression"},
		{"transition in id form", `{"id":"s1","transition":"trans.fade"}`, CodeCanonicalised, SeverityMinor, "/scenes/0/transition"},
		{"ease in id form", `{"id":"s1","beats":[{"do":"wait","seconds":1,"ease":"ease.linear"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/beats/0/ease"},
		// The phase 1 support matrix.
		{"transition not played yet", `{"id":"s1","transition":"slide-right"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"transition zoom", `{"id":"s1","transition":"zoom"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"sfx beat", `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"chart beat", `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"show with icon", `{"id":"s1","beats":[{"do":"show","icon":"warning","at":"center","entry":"pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateScenes(t, tc.scenes), tc.code, tc.severity, tc.path)
		})
	}
	// Supported names raise nothing.
	clean := `{"id":"s1","transition":"wipe","actors":[{"cast":"anna","pose":"typing","expression":"happy"}],"beats":[` +
		`{"do":"gesture","who":"anna","gesture":"point","count":2,"ease":"back-out"},{"do":"show","text":"Hi","style":"title","at":"center","entry":"fade"}]}`
	if issues := validateScenes(t, clean); len(issues) != 0 {
		t.Errorf("supported names rejected:%s", dump(issues))
	}
}

// The highlight beat is played from phase 2 on, so Validate no longer marks it.
func TestValidateHighlightIsSupported(t *testing.T) {
	for _, style := range []string{"pulse", "outline", "arrow"} {
		issues := validateScenes(t, `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"highlight","target":"a","style":"`+style+`"}]}`)
		if is := findIssue(issues, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"); is != nil {
			t.Errorf("%s still reports not_supported_yet:%s", style, dump(issues))
		}
	}
}

func TestValidateCanonicalisesInPlace(t *testing.T) {
	s := storyOf(t, `{"id":"s1","transition":"trans.fade","actors":[{"cast":"anna","pose":"pose.typing","expression":"expression.happy"}],`+
		`"beats":[{"do":"gesture","who":"anna","gesture":"gesture.point","ease":"ease.linear"}]}`)
	Validate(&s, loadMini(t), DefaultLimits())
	sc := s.Scenes[0]
	if sc.Transition != "fade" || sc.Actors[0].Pose != "typing" || sc.Actors[0].Expression != "happy" || sc.Beats[0].Gesture != "point" || sc.Beats[0].Ease != "linear" {
		t.Fatalf("not canonicalised: %+v", sc)
	}
	// A second pass finds nothing left to note.
	for _, is := range Validate(&s, loadMini(t), DefaultLimits()) {
		if is.Code == CodeCanonicalised {
			t.Fatalf("still canonicalising: %+v", is)
		}
	}
}

func TestValidateBeatRequiredFields(t *testing.T) {
	cases := []struct {
		beat    string
		missing []string // fields reported as missing_field
	}{
		{`{"do":"enter"}`, []string{"who", "from", "to"}},
		{`{"do":"exit","who":"anna"}`, []string{"to"}},
		{`{"do":"walk","who":"anna"}`, []string{"to"}},
		{`{"do":"walk","to":"left"}`, []string{"who"}},
		{`{"do":"move","target":"anna","to":"left"}`, nil},
		{`{"do":"pose","who":"anna"}`, []string{"pose"}},
		{`{"do":"gesture","who":"anna"}`, []string{"gesture"}},
		{`{"do":"emote","who":"anna"}`, []string{"expression"}},
		{`{"do":"look","who":"anna"}`, []string{"target"}},
		{`{"do":"say","who":"anna"}`, []string{"style"}},
		{`{"do":"show","at":"left","entry":"pop"}`, []string{"item"}},
		{`{"do":"show","item":"prop.lock"}`, []string{"at", "entry"}},
		{`{"do":"hide"}`, []string{"target"}},
		{`{"do":"highlight","target":"anna"}`, []string{"style"}},
		{`{"do":"camera"}`, []string{"op"}},
		{`{"do":"chart","chart":"bar"}`, []string{"data", "at"}},
		{`{"do":"sfx"}`, []string{"sound"}},
		{`{"do":"wait"}`, []string{"seconds"}},
		{`{}`, []string{"do"}},
	}
	for _, tc := range cases {
		t.Run(tc.beat, func(t *testing.T) {
			issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[`+tc.beat+`]}`)
			got := map[string]bool{}
			for _, is := range issues {
				if is.Code == CodeMissingField {
					got[strings.TrimPrefix(is.Path, "/scenes/0/beats/0/")] = true
					if is.Severity != SeverityError || is.Fix == "" {
						t.Errorf("missing field must be an error with a fix: %+v", is)
					}
				}
			}
			if len(got) != len(tc.missing) {
				t.Fatalf("missing fields %v, want %v:%s", got, tc.missing, dump(issues))
			}
			for _, m := range tc.missing {
				if !got[m] {
					t.Errorf("%s not reported:%s", m, dump(issues))
				}
			}
		})
	}
	// The fix of a missing field carries the example of the beat kind.
	issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"walk","who":"anna"}]}`)
	if is := expectIssue(t, issues, CodeMissingField, SeverityError, "/scenes/0/beats/0/to"); !strings.Contains(is.Fix, `{"do":"walk","who":"anna","to":"door"}`) {
		t.Errorf("fix: %s", is.Fix)
	}
}

func TestValidateEnumsAndRanges(t *testing.T) {
	story := func(mod string) string { return `{"id":"s1","actors":[{"cast":"anna"}],` + mod + `}` }
	cases := []struct{ name, scene, code, path string }{
		{"variant", story(`"variant":"dusk"`), CodeInvalidValue, "/scenes/0/variant"},
		{"kind", story(`"kind":"movie"`), CodeInvalidValue, "/scenes/0/kind"},
		{"camera framing", story(`"camera":{"framing":"huge"}`), CodeInvalidValue, "/scenes/0/camera/framing"},
		{"facing", `{"id":"s1","actors":[{"cast":"anna","facing":"up"}]}`, CodeInvalidValue, "/scenes/0/actors/0/facing"},
		{"actor scale", `{"id":"s1","actors":[{"cast":"anna","scale":3}]}`, CodeInvalidValue, "/scenes/0/actors/0/scale"},
		{"prop layer", `{"id":"s1","props":[{"item":"prop.lock","layer":"middle"}]}`, CodeInvalidValue, "/scenes/0/props/0/layer"},
		{"duration text", story(`"duration":"soon"`), CodeInvalidValue, "/scenes/0/duration"},
		{"duration zero", story(`"duration":0`), CodeInvalidValue, "/scenes/0/duration"},
		{"duration too long", story(`"duration":901`), CodeLimitExceeded, "/scenes/0/duration"},
		{"entry", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"left","entry":"explode"}]}`, CodeInvalidValue, "/scenes/0/beats/0/entry"},
		{"with", story(`"beats":[{"do":"wait","seconds":1,"with":"next"}]`), CodeInvalidValue, "/scenes/0/beats/0/with"},
		{"count", story(`"beats":[{"do":"gesture","who":"anna","gesture":"point","count":9}]`), CodeInvalidValue, "/scenes/0/beats/0/count"},
		{"delay", story(`"beats":[{"do":"wait","seconds":1,"delay":-1}]`), CodeInvalidValue, "/scenes/0/beats/0/delay"},
		{"camera op", story(`"beats":[{"do":"camera","op":"spin"}]`), CodeInvalidValue, "/scenes/0/beats/0/op"},
		{"say style", story(`"beats":[{"do":"say","who":"anna","style":"shout"}]`), CodeInvalidValue, "/scenes/0/beats/0/style"},
		{"chart data", `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a","b"],"values":[1]},"at":"left"}]}`, CodeInvalidValue, "/scenes/0/beats/0/data"},
		{"wait too long", story(`"beats":[{"do":"wait","seconds":9999}]`), CodeInvalidValue, "/scenes/0/beats/0/seconds"},
		{"show two things", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","text":"x","at":"left","entry":"pop"}]}`, CodeInvalidValue, "/scenes/0/beats/0/item"},
		{"first beat runs with prev", story(`"beats":[{"do":"wait","seconds":1,"with":"prev"}]`), CodeInvalidValue, "/scenes/0/beats/0/with"},
		{"code scene with beats", `{"id":"s1","kind":"code","beats":[{"do":"wait","seconds":1}]}`, CodeInvalidValue, "/scenes/0/kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateScenes(t, tc.scene)
			var found *Issue
			for i := range issues {
				if issues[i].Code == tc.code && issues[i].Path == tc.path {
					found = &issues[i]
				}
			}
			if found == nil {
				t.Fatalf("no %s at %s:%s", tc.code, tc.path, dump(issues))
			}
			if found.Fix == "" {
				t.Errorf("issue without a fix: %+v", found)
			}
		})
	}
}

func TestValidateStoryLevel(t *testing.T) {
	cat := loadMini(t)
	base := func() Story { return storyOf(t, `{"id":"s1"}`) }
	cases := []struct {
		name string
		mod  func(*Story)
		code string
		path string
	}{
		{"schema", func(s *Story) { s.Schema = "x" }, CodeSchemaMismatch, "/schema"},
		{"title", func(s *Story) { s.Title = "  " }, CodeMissingField, "/title"},
		{"title length", func(s *Story) { s.Title = strings.Repeat("ä", 121) }, CodeInvalidValue, "/title"},
		{"aspect", func(s *Story) { s.Format.Aspect = "4:3" }, CodeInvalidValue, "/format/aspect"},
		{"aspect missing", func(s *Story) { s.Format.Aspect = "" }, CodeMissingField, "/format/aspect"},
		{"fps", func(s *Story) { s.Format.FPS = 60 }, CodeInvalidValue, "/format/fps"},
		{"palette", func(s *Story) { s.Style.Palette = "neon" }, CodeUnknownName, "/style/palette"},
		{"font", func(s *Story) { s.Style.Font = "comic" }, CodeUnknownName, "/style/font"},
		{"no scenes", func(s *Story) { s.Scenes = nil }, CodeMissingField, "/scenes"},
		{"cast id", func(s *Story) { s.Cast[0].ID = "Anna K" }, CodeInvalidValue, "/cast/0/id"},
		{"cast character", func(s *Story) { s.Cast[0].Character = "char.nobody" }, CodeUnknownAsset, "/cast/0/character"},
		{"cast character kind", func(s *Story) { s.Cast[0].Character = "prop.lock" }, CodeInvalidValue, "/cast/0/character"},
		{"duplicate cast", func(s *Story) { s.Cast = append(s.Cast, s.Cast[0]) }, CodeDuplicateID, "/cast/1/id"},
		{"voice provider", func(s *Story) { s.Voice = &Voice{Language: "de"} }, CodeMissingField, "/voice/provider"},
		{"voice language", func(s *Story) { s.Voice = &Voice{Provider: "piper"} }, CodeMissingField, "/voice/language"},
		{"voice provider active", func(s *Story) { s.Voice = &Voice{Provider: "active", Language: "de"} }, CodeInvalidValue, "/voice/provider"},
		{"voice provider unknown", func(s *Story) { s.Voice = &Voice{Provider: "gemini", Language: "de"} }, CodeUnknownName, "/voice/provider"},
		{"voice id", func(s *Story) { s.Voice = &Voice{Provider: "piper", Voice: "../../etc/passwd", Language: "de"} }, CodeInvalidValue, "/voice/voice"},
		{"cast voice provider", func(s *Story) { s.Cast[0].Voice = &Voice{Language: "de"} }, CodeMissingField, "/cast/0/voice/provider"},
		{"cast voice provider active", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "Active", Language: "de"} }, CodeInvalidValue, "/cast/0/voice/provider"},
		{"cast voice provider unknown", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "gemini", Language: "de"} }, CodeUnknownName, "/cast/0/voice/provider"},
		{"cast voice id", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "piper", Voice: "a b", Language: "de"} }, CodeInvalidValue, "/cast/0/voice/voice"},
		{"music mode", func(s *Story) { s.Music = &Music{Mode: "radio"} }, CodeInvalidValue, "/music/mode"},
		{"music volume", func(s *Story) { v := 1.5; s.Music = &Music{Mode: "generated", Volume: &v} }, CodeInvalidValue, "/music/volume"},
		{"music file missing", func(s *Story) { s.Music = &Music{Mode: "file"} }, CodeMissingField, "/music/file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.mod(&s)
			issues := Validate(&s, cat, DefaultLimits())
			var found bool
			for _, is := range issues {
				if is.Code == tc.code && is.Path == tc.path && is.Severity == SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("no error %s at %s:%s", tc.code, tc.path, dump(issues))
			}
		})
	}
	// Valid voice ids and a music setting that is only a note.
	s := base()
	s.Voice = &Voice{Provider: "piper", Voice: "de_DE-thorsten-high", Language: "de"}
	s.Music = &Music{Mode: "generated", Mood: "upbeat-corporate"}
	issues := Validate(&s, cat, DefaultLimits())
	if HasErrors(issues) {
		t.Fatalf("errors:%s", dump(issues))
	}
	expectIssue(t, issues, CodeNotSupportedYet, SeverityMinor, "/music")
}

func TestValidateStyleRhythm(t *testing.T) {
	cat := loadMini(t)
	check := func(rhythm string) []Issue {
		s := storyOf(t, `{"id":"s1"}`)
		s.Style.Rhythm = rhythm
		return Validate(&s, cat, DefaultLimits())
	}
	expectIssue(t, check("fast"), CodeInvalidValue, SeverityError, "/style/rhythm")
	for _, rhythm := range []string{"", "calm", "standard", "lively"} {
		if is := findIssue(check(rhythm), CodeInvalidValue, SeverityError, "/style/rhythm"); is != nil {
			t.Errorf("rhythm %q refused: %+v", rhythm, is)
		}
	}
}

func TestValidateVoiceProvider(t *testing.T) {
	cat := loadMini(t)
	voiceIssues := func(path, provider string) []Issue {
		s := storyOf(t, `{"id":"s1"}`)
		v := &Voice{Provider: provider, Language: "de"}
		if path == "/voice" {
			s.Voice = v
		} else {
			s.Cast[0].Voice = v
		}
		return Validate(&s, cat, DefaultLimits())
	}
	for _, path := range []string{"/voice", "/cast/0/voice"} {
		// The TTS dispatcher ids (internal/tools/tts.go synthesizeProvider); case and space do not matter.
		for _, p := range []string{"phonon", "piper", "supertonic", "xai", "openai", "elevenlabs", "minimax", "mistral", "resemble",
			"stepfun", "sanotts", "google", "speech_lab", "Piper ", " SPEECH_LAB"} {
			if issues := voiceIssues(path, p); len(issues) != 0 {
				t.Errorf("%s provider %q:%s", path, p, dump(issues))
			}
		}
		// "active" is the default of a new project, never a saved provider.
		for _, p := range []string{"active", "Active", " ACTIVE "} {
			is := expectIssue(t, voiceIssues(path, p), CodeInvalidValue, SeverityError, path+"/provider")
			if !strings.Contains(is.Message, "concrete") || !strings.Contains(is.Fix, `{"provider":"piper"}`) {
				t.Errorf("%s provider %q: %+v", path, p, is)
			}
		}
		// An unknown provider (also an alias of the dispatcher) gets at most five alternatives.
		for _, p := range []string{"gemini", "s2s", "tts.piper", "piper?"} {
			is := expectIssue(t, voiceIssues(path, p), CodeUnknownName, SeverityError, path+"/provider")
			list := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(is.Fix, ". Example:", 2)[0], "Use one of: "), ".")
			alts := strings.Split(list, ", ")
			if len(alts) != 5 || !strings.Contains(is.Fix, `Example: {"provider":"`+alts[0]+`"}`) || !strings.Contains(is.Message, p) {
				t.Errorf("%s provider %q: %+v", path, p, is)
			}
		}
		// The closest provider comes first.
		for p, want := range map[string]string{"pipr": "piper", "elevenlab": "elevenlabs", "Open-AI": "openai"} {
			is := expectIssue(t, voiceIssues(path, p), CodeUnknownName, SeverityError, path+"/provider")
			if !strings.HasPrefix(is.Fix, "Use one of: "+want+",") {
				t.Errorf("%s provider %q: %s", path, p, is.Fix)
			}
		}
	}
}

func TestValidateMusicFilePaths(t *testing.T) {
	for path, ok := range map[string]bool{
		"Documents/Music/theme.mp3": true, "Music/a b.wav": true,
		"https://example.com/a.mp3": false, "file:///etc/passwd": false, "C:/Users/a.mp3": false, "/etc/passwd": false,
		"\\\\server\\share\\a.mp3": false, "../secret.mp3": false, "Documents/../../a.mp3": false, "Documents//a.mp3": false,
		"Documents\\a.mp3": false, "./a.mp3": false, "a\x00.mp3": false,
	} {
		s := storyOf(t, `{"id":"s1"}`)
		s.Music = &Music{Mode: "file", File: path}
		bad := findIssue(Validate(&s, loadMini(t), DefaultLimits()), CodeInvalidValue, SeverityError, "/music/file") != nil
		if bad == ok {
			t.Errorf("music file %q: accepted=%v, want %v", path, !bad, ok)
		}
	}
}

func TestValidateLimits(t *testing.T) {
	cat := loadMini(t)
	lim := Limits{MaxScenes: 2, MaxItemsPerScene: 2, MaxBeatsPerScene: 2, MaxNarrationChars: 10, MaxDurationSeconds: 30}
	s := storyOf(t, `{"id":"s1","narration":"`+strings.Repeat("ä", 11)+`","actors":[{"cast":"anna"}],"props":[{"item":"prop.lock"},{"item":"prop.shield"}],`+
		`"beats":[{"do":"wait","seconds":1},{"do":"wait","seconds":1},{"do":"wait","seconds":1}],"duration":20},{"id":"s2","duration":20},{"id":"s3"}`)
	issues := Validate(&s, cat, lim)
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes") // scene count
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/narration")
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/actors")
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/beats")
	n := 0
	for _, is := range issues {
		if is.Code == CodeLimitExceeded && is.Path == "/scenes" {
			n++
		}
	}
	if n != 2 { // too many scenes and explicit durations over the maximum (40 s > 30 s)
		t.Errorf("want two /scenes limit issues, got %d:%s", n, dump(issues))
	}
	// The default limits are the ones of the spec.
	if d := DefaultLimits(); d.MaxScenes != 200 || d.MaxItemsPerScene != 40 || d.MaxBeatsPerScene != 60 || d.MaxNarrationChars != 1500 ||
		d.MaxStoryBytes != 1<<20 || d.MaxCodeBytes != 64<<10 || d.MaxDurationSeconds != 900 {
		t.Errorf("default limits: %+v", d)
	}
}

func TestValidateWithoutCatalogChecksStructureOnly(t *testing.T) {
	s := storyOf(t, `{"id":"s1","environment":"env.nowhere","actors":[{"cast":"anna","at":"balcony","pose":"typing"}],"beats":[{"do":"hide","target":"x-1"},{"do":"walk","who":"anna"}]}`)
	issues := Validate(&s, nil, DefaultLimits())
	for _, is := range issues {
		switch is.Code {
		case CodeUnknownAsset, CodeUnknownSpot, CodeNotSupportedYet:
			t.Errorf("catalog-dependent issue without a catalog: %+v", is)
		}
	}
	expectIssue(t, issues, CodeMissingField, SeverityError, "/scenes/0/beats/1/to")
}

func TestValidateCueInSilentScene(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","silent":true,"narration":"Eins {1}","beats":[{"do":"wait","seconds":1,"cue":1}]}`)
	expectIssue(t, issues, CodeUnknownCue, SeverityWarning, "/scenes/0/beats/0/cue")
}

func TestCreateLikeStoryIsValid(t *testing.T) {
	s := Story{Schema: Schema, Title: "T", Format: Format{Aspect: "16:9", FPS: 30}, Cast: []CastMember{}, Scenes: []Scene{{ID: "s1"}}}
	ApplyDefaults(&s)
	if issues := Validate(&s, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("a fresh story must validate cleanly:%s", dump(issues))
	}
}

func TestValidateGlyphMissing(t *testing.T) {
	thai := string(rune(0x0E01))
	zwj := string(rune(0x200D))
	lineSep := string(rune(0x2028))
	show := func(text string) string {
		return `{"id":"s1","beats":[{"do":"show","text":"` + text + `","at":"center","entry":"fade"}]}`
	}
	bareSay := `"actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech"}]`
	cases := []struct {
		name     string
		captions bool
		scene    string
		path     string // "" means no glyph_missing at all
	}{
		{"show text", true, show(thai), "/scenes/0/beats/0/text"},
		{"say text", true, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech","text":"` + thai + `"}]}`, "/scenes/0/beats/0/text"},
		{"zero width joiner is exempt", true, show("a" + zwj + "b"), ""},
		{"line separator is exempt", true, show("a" + lineSep + "b"), ""},
		{"narration with captions on", true, `{"id":"s1","narration":"` + thai + `"}`, "/scenes/0/narration"},
		{"narration with captions off", false, `{"id":"s1","narration":"` + thai + `"}`, ""},
		{"narration with captions off and a say without text", false, `{"id":"s1","speaker":"anna","narration":"` + thai + `",` + bareSay + `}`, "/scenes/0/narration"},
		{"silent narration with captions on", true, `{"id":"s1","silent":true,"narration":"` + thai + `"}`, ""},
		{"silent narration with a say without text", true, `{"id":"s1","silent":true,"narration":"` + thai + `",` + bareSay + `}`, ""},
		{"code scene narration with captions on", true, `{"id":"s1","kind":"code","narration":"` + thai + `"}`, "/scenes/0/narration"},
		{"code scene narration with captions off", false, `{"id":"s1","kind":"code","narration":"` + thai + `"}`, ""},
		{"code scene strings stay unchecked", true, `{"id":"s1","kind":"code","beats":[{"do":"show","text":"` + thai + `","at":"center","entry":"fade"}]}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := storyOf(t, tc.scene)
			s.Style.Captions = tc.captions
			var found []Issue
			for _, is := range Validate(&s, loadReal(t), DefaultLimits()) {
				if is.Code == CodeGlyphMissing {
					found = append(found, is)
				}
			}
			if tc.path == "" {
				if len(found) != 0 {
					t.Fatalf("unexpected glyph_missing:%s", dump(found))
				}
				return
			}
			if len(found) != 1 || found[0].Path != tc.path || found[0].Severity != SeverityError {
				t.Fatalf("want one error at %s, got:%s", tc.path, dump(found))
			}
			if !strings.Contains(found[0].Message, "U+0E01") {
				t.Fatalf("message does not name the character: %s", found[0].Message)
			}
		})
	}
	t.Run("names at most three characters", func(t *testing.T) {
		// Four distinct Thai letters and one repeat, so the fourth stays unnamed.
		s := storyOf(t, show(string([]rune{0x0E01, 0x0E02, 0x0E03, 0x0E04, 0x0E01})))
		is := expectIssue(t, Validate(&s, loadReal(t), DefaultLimits()), CodeGlyphMissing, SeverityError, "/scenes/0/beats/0/text")
		if !strings.Contains(is.Message, fmt.Sprintf("U+0E01 %q", thai)) || !strings.Contains(is.Message, "U+0E03") || strings.Contains(is.Message, "U+0E04") {
			t.Fatalf("message: %s", is.Message)
		}
	})
	t.Run("the mini catalog has no coverage, so nothing is checked", func(t *testing.T) {
		for _, is := range validateScenes(t, show(thai)) {
			if is.Code == CodeGlyphMissing {
				t.Fatalf("unexpected: %+v", is)
			}
		}
	})
}

func TestValidateSayAndSpeaker(t *testing.T) {
	const bareSay = `{"do":"say","who":"anna","style":"speech"}`
	stage := func(extra, beats string) string {
		return `{"id":"s1",` + extra + `"actors":[{"cast":"anna"}],"beats":[` + beats + `]}`
	}

	// A say without text shows the narration of the scene speaker, so its who must be that speaker.
	expectIssue(t, validateScenes(t, stage("", bareSay)), CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who")
	markerOnly := `{"do":"say","who":"anna","style":"speech","text":"{1} "}`
	expectIssue(t, validateScenes(t, stage(`"speaker":"narrator",`, markerOnly)), CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who")
	if issues := validateScenes(t, stage(`"speaker":"anna",`, bareSay)); len(issues) != 0 {
		t.Errorf("a say by the speaker:%s", dump(issues))
	}

	// In a silent scene it has no narration to show: a warning, never an error.
	issues := validateScenes(t, stage(`"silent":true,`, bareSay))
	expectIssue(t, issues, CodeInvalidValue, SeverityWarning, "/scenes/0/beats/0/text")
	if is := findIssue(issues, CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who"); is != nil {
		t.Errorf("a silent scene must not block on who: %+v", is)
	}

	// The say text has the narration limit: DefaultLimits().MaxNarrationChars is 1500, so 1501 is one too many.
	sayOf := func(n int) string {
		return `{"do":"say","who":"anna","style":"speech","text":"` + strings.Repeat("a", n) + `"}`
	}
	expectIssue(t, validateScenes(t, stage("", sayOf(1501))), CodeLimitExceeded, SeverityError, "/scenes/0/beats/0/text")
	if issues := validateScenes(t, stage("", sayOf(1500))); len(issues) != 0 {
		t.Errorf("a say at the limit:%s", dump(issues))
	}

	// Off stage the speech still plays. Every other beat is still ignored.
	off := expectIssue(t, validateScenes(t, `{"id":"s1","beats":[{"do":"say","who":"anna","style":"speech","text":"Hi"}]}`), CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who")
	if !strings.Contains(off.Message, "still plays") {
		t.Errorf("off-stage say: %s", off.Message)
	}
	pose := expectIssue(t, validateScenes(t, `{"id":"s1","beats":[{"do":"pose","who":"anna","pose":"stand"}]}`), CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who")
	if !strings.Contains(pose.Message, "ignored") {
		t.Errorf("off-stage pose: %s", pose.Message)
	}

	// A cast speaker and a say are played now, so neither is not_supported_yet.
	for _, scene := range []string{`{"id":"s1","speaker":"anna"}`, stage("", sayOf(2))} {
		for _, is := range validateScenes(t, scene) {
			if is.Code == CodeNotSupportedYet {
				t.Errorf("%s: %+v", scene, is)
			}
		}
	}
}
