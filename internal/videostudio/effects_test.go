package videostudio

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestEffectRegistryInvariants(t *testing.T) {
	registry := EffectRegistry()
	if registry.Version != RegistryVersion || RegistryVersion != 1 {
		t.Fatalf("registry version = %d, RegistryVersion = %d", registry.Version, RegistryVersion)
	}
	checkParams := func(where string, params []ParamSpec) {
		t.Helper()
		seen := map[string]bool{}
		for _, param := range params {
			if param.ID == "" || seen[param.ID] {
				t.Fatalf("%s: empty or duplicate parameter id %q", where, param.ID)
			}
			seen[param.ID] = true
			if param.LabelKey == "" || strings.HasPrefix(param.LabelKey, "videoStudio.") {
				t.Fatalf("%s.%s: label key %q must be set, without the videoStudio. prefix", where, param.ID, param.LabelKey)
			}
			switch param.Type {
			case ParamNumber:
				value, ok := param.Default.(float64)
				if !ok || !(param.Min < param.Max) || param.Step <= 0 || value < param.Min || value > param.Max {
					t.Fatalf("%s.%s: a number needs min < max, step > 0 and a float64 default in range: %+v", where, param.ID, param)
				}
			case ParamChoice:
				value, ok := param.Default.(string)
				if !ok || !slices.Contains(param.Choices, value) || param.Keyframable {
					t.Fatalf("%s.%s: a choice needs its default among the choices and is not keyframable: %+v", where, param.ID, param)
				}
			case ParamBool:
				if _, ok := param.Default.(bool); !ok || param.Keyframable {
					t.Fatalf("%s.%s: a bool needs a bool default and is not keyframable: %+v", where, param.ID, param)
				}
			case ParamColor:
				value, ok := param.Default.(string)
				if !ok || len(value) != 7 || value[0] != '#' || strings.Trim(strings.ToLower(value[1:]), "0123456789abcdef") != "" {
					t.Fatalf("%s.%s: a colour needs a #rrggbb default: %+v", where, param.ID, param)
				}
			case ParamLUT:
				if _, ok := param.Default.(string); !ok || param.Keyframable {
					t.Fatalf("%s.%s: a LUT needs a string default and is not keyframable: %+v", where, param.ID, param)
				}
			default:
				t.Fatalf("%s.%s: unknown parameter type %q", where, param.ID, param.Type)
			}
		}
	}

	checkParams("base", registry.Base)
	for _, id := range []string{"x", "y", "width", "height", "rotation", "opacity"} {
		if param, ok := BaseParam(id); !ok || param.Type != ParamNumber || !param.Keyframable {
			t.Fatalf("base parameter %q = %+v, %t; want a keyframable number", id, param, ok)
		}
	}
	if fit, ok := BaseParam("fit"); !ok || fit.Type != ParamChoice || fit.Default != "contain" || !slices.Equal(fit.Choices, []string{"contain", "cover"}) {
		t.Fatalf("base parameter fit = %+v, %t", fit, ok)
	}
	if _, ok := BaseParam("no-such-param"); ok {
		t.Fatal("BaseParam found an unknown id")
	}

	effects := map[string]bool{}
	for _, effect := range registry.Effects {
		if effect.ID == "" || effects[effect.ID] || effect.Category == "" || effect.LabelKey == "" || effect.Params == nil || effect.Filters == nil {
			t.Fatalf("effect %+v needs a unique id, a category, a label key and non-nil params and filters", effect)
		}
		effects[effect.ID] = true
		checkParams("effect "+effect.ID, effect.Params)
		if found, ok := EffectByID(effect.ID); !ok || found.ID != effect.ID {
			t.Fatalf("EffectByID(%q) = %+v, %t", effect.ID, found, ok)
		}
	}
	if _, ok := EffectByID("no-such-effect"); ok {
		t.Fatal("EffectByID found an unknown id")
	}

	transitions := map[TransitionType]bool{}
	for _, spec := range registry.Transitions {
		if spec.ID == "" || transitions[spec.ID] || spec.XFade == "" || spec.Category == "" || spec.LabelKey == "" {
			t.Fatalf("transition %+v needs a unique id, an xfade name, a category and a label key", spec)
		}
		transitions[spec.ID] = true
		if found, ok := TransitionByID(spec.ID); !ok || found != spec {
			t.Fatalf("TransitionByID(%q) = %+v, %t", spec.ID, found, ok)
		}
	}
	if _, ok := TransitionByID("not-a-transition"); ok {
		t.Fatal("TransitionByID found an unknown id")
	}
	// The phase 1 transitions keep their ids, export mapping and label keys, first in menu order.
	phase1 := []TransitionSpec{
		{ID: TransitionDissolve, XFade: "fade", Category: "basic", LabelKey: "dissolve"},
		{ID: TransitionBlack, XFade: "fadeblack", Category: "basic", LabelKey: "black"},
		{ID: TransitionWipeLeft, XFade: "wiperight", Category: "wipe", LabelKey: "wipeleft"},
		{ID: TransitionWipeRight, XFade: "wipeleft", Category: "wipe", LabelKey: "wiperight"},
	}
	if len(registry.Transitions) < len(phase1) || !slices.Equal(registry.Transitions[:len(phase1)], phase1) {
		t.Fatalf("first transitions = %+v, want %+v", registry.Transitions, phase1)
	}

	ids := map[string]bool{}
	for _, mode := range registry.BlendModes {
		if mode.ID == "" || ids["blend:"+mode.ID] || mode.LabelKey == "" {
			t.Fatalf("blend mode %+v needs a unique id and a label key", mode)
		}
		ids["blend:"+mode.ID] = true
	}
	// The base blend row offers exactly the BlendModes ids in menu order (2d). Normal comes first and is the only mode
	// without an FFmpeg name; every other mode's FFmpeg name is its id.
	var modeIDs []string
	for i, mode := range registry.BlendModes {
		modeIDs = append(modeIDs, mode.ID)
		if (i == 0) != (mode.Blend == "") || i > 0 && mode.Blend != mode.ID {
			t.Fatalf("blend mode %d = %+v: only normal (first) has no FFmpeg name, the others use their id", i, mode)
		}
		if found, ok := BlendModeByID(mode.ID); !ok || found != mode {
			t.Fatalf("BlendModeByID(%q) = %+v, %t", mode.ID, found, ok)
		}
	}
	if blend, ok := BaseParam("blend"); !ok || len(modeIDs) == 0 || modeIDs[0] != BlendNormal || blend.Default != BlendNormal || !slices.Equal(blend.Choices, modeIDs) {
		t.Fatalf("base row blend = %+v, %t; want the choices %v with default normal", blend, ok, modeIDs)
	}
	if shape, ok := BaseParam("mask.shape"); !ok || shape.Default != string(MaskRect) || !slices.Equal(shape.Choices, []string{string(MaskRect), string(MaskEllipse)}) {
		t.Fatalf("base row mask.shape = %+v, %t", shape, ok)
	}
	for _, preset := range registry.AnimPresets {
		if preset.ID == "" || ids["anim:"+preset.ID] || preset.LabelKey == "" {
			t.Fatalf("animation preset %+v needs a unique id and a label key", preset)
		}
		ids["anim:"+preset.ID] = true
	}
	for _, look := range registry.Looks {
		if look.ID == "" || ids["look:"+look.ID] || look.LabelKey == "" {
			t.Fatalf("look %+v needs a unique id and a label key", look)
		}
		ids["look:"+look.ID] = true
	}

	// EffectRegistry returns a deep copy: changing it does not change the tables.
	registry.Base[0].ID = "changed"
	registry.Transitions[0].XFade = "changed"
	for i := range registry.Base {
		if len(registry.Base[i].Choices) > 0 {
			registry.Base[i].Choices[0] = "changed"
		}
	}
	if again := EffectRegistry(); again.Base[0].ID != "x" || again.Transitions[0].XFade != "fade" {
		t.Fatal("EffectRegistry shares its tables with the caller")
	}
	if fit, _ := BaseParam("fit"); fit.Choices[0] != "contain" {
		t.Fatal("EffectRegistry shares parameter choices with the caller")
	}

	// JSON: every list is an array (never null) and the parameter shape is fixed.
	data, err := json.Marshal(EffectRegistry())
	if err != nil {
		t.Fatal(err)
	}
	var lists map[string]json.RawMessage
	if err := json.Unmarshal(data, &lists); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"base", "effects", "transitions", "blend_modes", "anim_presets", "looks"} {
		if raw := lists[key]; len(raw) == 0 || raw[0] != '[' {
			t.Fatalf("registry JSON %q = %s, want an array", key, raw)
		}
	}
	for _, want := range []string{
		`{"id":"x","type":"number","min":0,"max":1,"default":0,"step":0.001,"keyframable":true,"unit":"fraction","label_key":"box_x"}`,
		`{"id":"opacity","type":"number","min":0,"max":1,"default":1,"step":0.01,"keyframable":true,"label_key":"opacity"}`,
		`{"id":"fit","type":"choice","min":0,"max":0,"default":"contain","choices":["contain","cover"],"label_key":"fit"}`,
		`{"id":"dissolve","xfade":"fade","category":"basic","label_key":"dissolve"}`,
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("registry JSON lacks %s:\n%s", want, data)
		}
	}
	var decoded Registry
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Version != RegistryVersion || len(decoded.Base) != len(EffectRegistry().Base) || !slices.Equal(decoded.Transitions, EffectRegistry().Transitions) {
		t.Fatalf("registry JSON round trip = %+v, %v", decoded, err)
	}
}

func TestValidateRejectsUnknownTransition(t *testing.T) {
	project := validProject()
	project.Tracks[0].Clips = []Clip{
		{ID: "out", AssetID: "video", Start: 0, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionDissolve, Duration: 10}},
		{ID: "in", AssetID: "video", Start: 50, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
	}
	for _, spec := range EffectRegistry().Transitions {
		project.Tracks[0].Clips[0].Transition.Type = spec.ID
		if err := Validate(project); err != nil {
			t.Fatalf("registry transition %q rejected: %v", spec.ID, err)
		}
	}
	for _, unknown := range []TransitionType{"", "not-a-transition", "Dissolve"} {
		project.Tracks[0].Clips[0].Transition.Type = unknown
		if err := Validate(project); err == nil || !strings.Contains(err.Error(), "unsupported transition") {
			t.Fatalf("transition %q: Validate() = %v, want an unsupported transition error", unknown, err)
		}
	}
}

func TestAppendTransitionTakesXFadeFromRegistry(t *testing.T) {
	for _, spec := range EffectRegistry().Transitions {
		sequence := 0
		join := func() string { sequence++; return fmt.Sprintf("j%d", sequence) }
		_, filters := appendTransition(nil, join, "out", "in", Clip{Start: 0, Duration: 20}, Transition{Type: spec.ID, Duration: 10}, "settb=1/30", Rate{30, 1})
		if graph := strings.Join(filters, ";"); !strings.Contains(graph, "xfade=transition="+spec.XFade+":duration=") {
			t.Fatalf("transition %q: graph lacks xfade %q:\n%s", spec.ID, spec.XFade, graph)
		}
	}
}
