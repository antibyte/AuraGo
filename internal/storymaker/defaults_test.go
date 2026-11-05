package storymaker

import (
	"reflect"
	"strings"
	"testing"
)

func appliedMap(d []DefaultApplied) map[string]any {
	m := map[string]any{}
	for _, a := range d {
		m[a.Path] = a.Value
	}
	return m
}

func TestApplyDefaultsTable(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},
	  "cast":[{"id":"anna","character":"char.office-woman-1"},{"id":"ben","name":"Ben","character":"char.office-man-1"}],
	  "scenes":[{"id":"s1","narration":"Hallo."},
	            {"id":"s2","environment":"env.office","actors":[{"cast":"anna"}],"props":[{"item":"prop.lock"}]},
	            {"id":"s3","actors":[{"cast":"ben"},{"cast":"anna"}]}]}`)
	got := ApplyDefaults(&s)
	m := appliedMap(got)
	want := map[string]any{
		// scene 1: first scene
		"/scenes/0/environment": "env.plain-stage", "/scenes/0/variant": "day", "/scenes/0/speaker": "narrator",
		"/scenes/0/transition": "cut", "/scenes/0/duration": "auto", "/scenes/0/camera": map[string]any{"framing": "wide"},
		// scene 2: one actor at center, prop layer front
		"/scenes/1/variant": "day", "/scenes/1/speaker": "narrator", "/scenes/1/transition": "wipe", "/scenes/1/duration": "auto",
		"/scenes/1/camera": map[string]any{"framing": "wide"}, "/scenes/1/actors/0/at": "center", "/scenes/1/actors/0/pose": "stand",
		"/scenes/1/actors/0/facing": "auto", "/scenes/1/actors/0/expression": "neutral", "/scenes/1/actors/0/scale": 1,
		"/scenes/1/props/0/layer": "front",
		// scene 3: inherits env.office; two actors take left/right in cast order (anna first, whatever the scene order)
		"/scenes/2/environment": "env.office", "/scenes/2/variant": "day", "/scenes/2/speaker": "narrator", "/scenes/2/transition": "fade",
		"/scenes/2/duration": "auto", "/scenes/2/camera": map[string]any{"framing": "wide"},
		"/scenes/2/actors/0/at": "right", "/scenes/2/actors/0/pose": "stand", "/scenes/2/actors/0/facing": "auto", "/scenes/2/actors/0/expression": "neutral", "/scenes/2/actors/0/scale": 1,
		"/scenes/2/actors/1/at": "left", "/scenes/2/actors/1/pose": "stand", "/scenes/2/actors/1/facing": "auto", "/scenes/2/actors/1/expression": "neutral", "/scenes/2/actors/1/scale": 1,
		// cast name = id (only where omitted)
		"/cast/0/name": "anna",
	}
	if !reflect.DeepEqual(m, want) {
		for k, v := range want {
			if !reflect.DeepEqual(m[k], v) {
				t.Errorf("%s = %#v, want %#v", k, m[k], v)
			}
		}
		for k, v := range m {
			if _, ok := want[k]; !ok {
				t.Errorf("unexpected default %s = %#v", k, v)
			}
		}
	}
	if s.Scenes[1].Environment != "env.office" || s.Scenes[2].Actors[0].At.Spot != "right" || s.Cast[0].Name != "anna" || s.Cast[1].Name != "Ben" {
		t.Errorf("story not updated: %+v", s)
	}
	if !s.Scenes[0].Duration.Auto || s.Scenes[0].Camera.Framing != "wide" {
		t.Errorf("duration/camera: %+v %+v", s.Scenes[0].Duration, s.Scenes[0].Camera)
	}
}

func TestApplyDefaultsKeepsExplicitValues(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"9:16","fps":24},"style":{"palette":"calm","font":"default","captions":false},
	  "cast":[{"id":"anna","name":"Ann","character":"char.office-woman-1"}],
	  "scenes":[{"id":"s1","environment":"env.office","variant":"night","speaker":"anna","transition":"wipe","duration":5,"camera":{"framing":"close"},
	    "actors":[{"cast":"anna","at":{"x":10,"y":90},"pose":"sit","facing":"left","expression":"happy","scale":1.5}],
	    "props":[{"item":"prop.lock","layer":"back"}]}]}`)
	before, _ := CanonicalJSON(s)
	got := ApplyDefaults(&s)
	after, _ := CanonicalJSON(s)
	if len(got) != 0 || string(before) != string(after) {
		t.Fatalf("explicit values changed: %v\n%s\n%s", got, before, after)
	}
}

func TestApplyDefaultsTransitionFollowsTheEnvironment(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[
	  {"id":"s1","environment":"env.office"},
	  {"id":"s2","environment":"env.office"},
	  {"id":"s3","environment":"env.plain-stage"},
	  {"id":"s4"},
	  {"id":"s5","environment":"env.office","transition":"fade"}]}`)
	m := appliedMap(ApplyDefaults(&s))
	for path, want := range map[string]string{
		"/scenes/0/transition": "cut",  // first scene, whatever its environment
		"/scenes/1/transition": "fade", // env.office again
		"/scenes/2/transition": "wipe", // env.office to env.plain-stage
		"/scenes/3/transition": "fade", // no environment: inherits env.plain-stage
	} {
		if m[path] != want {
			t.Errorf("%s = %#v, want %q", path, m[path], want)
		}
	}
	// s5 changes the environment, but its stored fade stays and is not listed as a default.
	if _, listed := m["/scenes/4/transition"]; listed || s.Scenes[4].Transition != "fade" {
		t.Errorf("stored fade changed: listed=%v value=%q", listed, s.Scenes[4].Transition)
	}
}

func TestApplyDefaultsIsIdempotentAndNeverTouchesTiming(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[{"id":"s1","actors":[{"cast":"a"}],"beats":[{"do":"wait","seconds":2},{"do":"show","text":"x","at":"left","entry":"fade"}]}]}`)
	first := ApplyDefaults(&s)
	if len(first) == 0 {
		t.Fatal("nothing applied")
	}
	if again := ApplyDefaults(&s); len(again) != 0 {
		t.Fatalf("second call applied %v", again)
	}
	b := s.Scenes[0].Beats
	if b[0].Delay != 0 || b[0].Duration != 0 || b[0].Ease != "" || b[1].Delay != 0 || b[1].Duration != 0 || b[1].Ease != "" || b[0].Seconds != 2 {
		t.Fatalf("timing was touched: %+v", b)
	}
	for _, a := range first {
		if !strings.Contains(a.Path, "/beats/") {
			continue
		}
		if !strings.HasSuffix(a.Path, "/id") {
			t.Fatalf("a beat default other than the show id: %v", a)
		}
	}
}

func TestApplyDefaultsFillsDocumentLevelFields(t *testing.T) {
	var s Story
	got := appliedMap(ApplyDefaults(&s))
	want := map[string]any{
		"/schema": Schema, "/format/aspect": "16:9", "/format/fps": 30,
		"/style": map[string]any{"palette": "sunny", "font": "default", "captions": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	// A style with only a palette keeps the author's captions choice (false here, since the
	// story was built in code) and fills font.
	s2 := Story{Style: Style{Palette: "calm"}}
	m := appliedMap(ApplyDefaults(&s2))
	if m["/style/font"] != "default" || s2.Style.Palette != "calm" {
		t.Fatalf("%#v", m)
	}
}

func TestApplyDefaultsActorSlots(t *testing.T) {
	cast := `"cast":[{"id":"a","character":"char.x"},{"id":"b","character":"char.x"},{"id":"c","character":"char.x"},{"id":"d","character":"char.x"},{"id":"e","character":"char.x"}]`
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T",`+cast+`,"scenes":[
	  {"id":"s1","actors":[{"cast":"a"},{"cast":"b"},{"cast":"c"},{"cast":"d"}]},
	  {"id":"s2","actors":[{"cast":"d","at":"desk"},{"cast":"b"}]},
	  {"id":"s3","actors":[{"cast":"unknown"},{"cast":"a"}]}]}`)
	ApplyDefaults(&s)
	slots := func(i int) []string {
		var out []string
		for _, a := range s.Scenes[i].Actors {
			out = append(out, a.At.Spot)
		}
		return out
	}
	if got := slots(0); !reflect.DeepEqual(got, []string{"left", "right", "far-left", "far-right"}) {
		t.Errorf("four actors: %v", got)
	}
	// Explicit positions stay; the other actor keeps the slot of its rank (b is the first by cast order).
	if got := slots(1); !reflect.DeepEqual(got, []string{"desk", "left"}) {
		t.Errorf("explicit position kept, rank slot for the rest: %v", got)
	}
	// An actor of an unknown cast member ranks last.
	if got := slots(2); !reflect.DeepEqual(got, []string{"right", "left"}) {
		t.Errorf("unknown cast last: %v", got)
	}
}

func TestApplyDefaultsShowBeatInstanceIDs(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[{"id":"s1",
	  "props":[{"id":"lock-1","item":"prop.lock"}],
	  "beats":[
	    {"do":"show","item":"prop.lock","at":"left","entry":"pop"},
	    {"do":"show","item":"prop.lock","at":"right","entry":"pop"},
	    {"do":"show","item":"prop.shield","at":"center","entry":"fade"},
	    {"do":"show","item":"my.prop.red-box","at":"center","entry":"fade"},
	    {"do":"show","text":"Hallo","at":"center","entry":"fade"},
	    {"do":"show","icon":"Warning Sign","at":"center","entry":"fade"},
	    {"do":"show","item":"prop.shield","id":"keep","at":"center","entry":"fade"},
	    {"do":"wait","seconds":1}]}]}`)
	ApplyDefaults(&s)
	var ids []string
	for _, b := range s.Scenes[0].Beats {
		ids = append(ids, b.ID)
	}
	// lock-1 is taken by the prop, so the first shown lock is lock-2; ids count per item.
	want := []string{"lock-2", "lock-3", "shield-1", "red-box-1", "text-1", "warning-sign-1", "keep", ""}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for _, id := range ids {
		if id != "" && !validSlug(id) {
			t.Errorf("%q is not a slug", id)
		}
	}
}
