package storymaker

import (
	"strings"
	"testing"
)

// composeStory has a show beat (index 0) and a gesture beat (index 1) in one scene.
const composeStory = `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},
"style":{"palette":"sunny","font":"default","captions":true},
"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
"scenes":[{"id":"s1","environment":"env.office","narration":"Eine {1}Firewall schuetzt das Netz.","speaker":"anna",
"actors":[{"cast":"anna","at":"left","pose":"present"}],
"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1},{"do":"gesture","who":"anna","gesture":"point","with":"prev"}]}]}`

func TestBeatScaleRoundTripsAndIsRangeChecked(t *testing.T) {
	cat := loadReal(t)
	s := mustDecode(t, composeStory)
	s.Scenes[0].Beats[0].Scale = 1.5
	raw, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"scale": 1.5`) {
		t.Fatalf("canonical JSON lacks the beat scale:\n%s", raw)
	}
	back := mustDecode(t, string(raw))
	if back.Scenes[0].Beats[0].Scale != 1.5 {
		t.Fatalf("scale after a round trip = %v", back.Scenes[0].Beats[0].Scale)
	}
	if issues := Validate(&back, cat, DefaultLimits()); HasErrors(issues) {
		t.Fatalf("a scale of 1.5 is valid: %s", dump(issues))
	}

	for _, bad := range []float64{0.2, 2.5, -1} {
		s.Scenes[0].Beats[0].Scale = bad
		found := false
		for _, is := range Validate(&s, cat, DefaultLimits()) {
			if is.Path == "/scenes/0/beats/0/scale" && is.Severity == SeverityError {
				found = true
			}
		}
		if !found {
			t.Errorf("scale %v must be refused at /scenes/0/beats/0/scale", bad)
		}
	}

	// A scale on a gesture beat is a warning, not an error.
	s.Scenes[0].Beats[0].Scale = 0
	s.Scenes[0].Beats[1].Scale = 1.2
	warned := false
	for _, is := range Validate(&s, cat, DefaultLimits()) {
		if is.Path == "/scenes/0/beats/1/scale" {
			warned = is.Severity == SeverityWarning
		}
	}
	if !warned {
		t.Error("a scale on a gesture beat must be a warning")
	}
}

func TestStyleComposeIsWrittenOnlyWhenSetAndAcceptsOnlyV2(t *testing.T) {
	cat := loadReal(t)
	s := mustDecode(t, composeStory)
	raw, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "compose") {
		t.Fatalf("compose must not be written when it is not set:\n%s", raw)
	}

	s.Style.Compose = "v2"
	raw, err = CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"compose": "v2"`) {
		t.Fatalf("canonical JSON lacks compose:\n%s", raw)
	}
	back := mustDecode(t, string(raw))
	if back.Style.Compose != "v2" {
		t.Fatalf("compose after a round trip = %q", back.Style.Compose)
	}
	if issues := Validate(&back, cat, DefaultLimits()); HasErrors(issues) {
		t.Fatalf("v2 is valid: %s", dump(issues))
	}

	s.Style.Compose = "v3"
	found := false
	for _, is := range Validate(&s, cat, DefaultLimits()) {
		if is.Path == "/style/compose" && is.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Error("compose v3 must be refused at /style/compose")
	}

	// ApplyDefaults never sets it.
	plain := Story{Schema: Schema, Title: "x", Format: Format{Aspect: "16:9", FPS: 30}, Scenes: []Scene{{ID: "s1"}}}
	ApplyDefaults(&plain)
	if plain.Style.Compose != "" {
		t.Errorf("ApplyDefaults set compose to %q", plain.Style.Compose)
	}
}
