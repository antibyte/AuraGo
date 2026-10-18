package videostudio

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// effectTestProject is animationProject (validate_animation_test.go) plus the lut asset "grade": "v" is a 300-frame
// video on the video track, "o" an image on the overlay track and "a" a song on the audio track, 60 frames each.
func effectTestProject() Project {
	project := animationProject()
	project.Assets = append(project.Assets, Asset{ID: "grade", Name: "grade.cube", Kind: AssetLUT, LUTSize: 17, SizeBytes: 1024})
	return project
}

// onClip edits the clip with the given id.
func onClip(id string, edit func(*Clip)) func(*Project) {
	return func(project *Project) {
		for ti := range project.Tracks {
			for ci := range project.Tracks[ti].Clips {
				if project.Tracks[ti].Clips[ci].ID == id {
					edit(&project.Tracks[ti].Clips[ci])
				}
			}
		}
	}
}

// onAsset edits the asset with the given id.
func onAsset(id string, edit func(*Asset)) func(*Project) {
	return func(project *Project) {
		for i := range project.Assets {
			if project.Assets[i].ID == id {
				edit(&project.Assets[i])
			}
		}
	}
}

// effectDefaults is a complete params object of a registry effect at its defaults, the way the editor creates one.
func effectDefaults(id string) map[string]any {
	spec, _ := EffectByID(id)
	params := make(map[string]any, len(spec.Params))
	for _, param := range spec.Params {
		params[param.ID] = param.Default
	}
	return params
}

// Crop, flips and the effect stack (plan 2c, index §4.1 and §4.2). Every refusal is a *FieldError with the path of
// the offending field; path "" means Validate must accept the project.
func TestValidateEffectStack(t *testing.T) {
	k := func(f int, v float64) Keyframe { return Keyframe{F: f, V: v, E: EaseLinear} }
	run := func(n int, v float64) []Keyframe {
		keys := make([]Keyframe, n)
		for i := range keys {
			keys[i] = k(i, v)
		}
		return keys
	}
	with := func(params map[string]any, key string, value any) map[string]any {
		out := maps.Clone(params)
		out[key] = value
		return out
	}
	entry := func(id string, params map[string]any) EffectInstance {
		return EffectInstance{ID: id, On: true, Params: params}
	}
	stack := func(entries ...EffectInstance) func(*Project) {
		return onClip("v", func(c *Clip) { c.Effects = entries })
	}
	cropV := func(crop Crop) func(*Project) { return onClip("v", func(c *Clip) { c.Crop = &crop }) }
	sharpens := func(n int) []EffectInstance {
		list := make([]EffectInstance, n)
		for i := range list {
			list[i] = EffectInstance{ID: "sharpen", On: i%2 == 0, Params: map[string]any{}}
		}
		return list
	}
	strengthKeys := func(keys ...Keyframe) func(*Project) {
		return stack(EffectInstance{ID: "sharpen", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"strength": keys}})
	}
	luts := func(n int) func(*Project) {
		return func(project *Project) {
			for i := 0; i < n; i++ {
				project.Assets = append(project.Assets, Asset{ID: fmt.Sprintf("lut-%d", i), Name: "look.cube", Kind: AssetLUT, LUTSize: 33})
			}
		}
	}
	// 9 parameters × 57 keys = 513 keys on effect entries alone, each list within 64.
	tooManyKeys := EffectInstance{ID: "color", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{}}
	for _, name := range []string{"exposure", "contrast", "saturation", "temperature", "tint", "lift_x", "lift_y", "lift", "gamma_x"} {
		spec, _ := effectParamSpec("color", name)
		tooManyKeys.Keyframes[name] = run(57, spec.Default.(float64))
	}
	bound := MaxClipSpeed * MaxDurationFrames(animationProject())
	color := effectDefaults("color")
	cases := []struct {
		name string
		edit func(*Project)
		path string
	}{
		{"crop and a flip on the video clip", onClip("v", func(c *Clip) { c.Crop, c.FlipH = &Crop{L: 0.1, R: 0.1, B: 0.05}, true }), ""},
		{"l + r at 0.95 within float rounding", cropV(Crop{L: 0.9, R: 0.05}), ""},
		{"both flips on the overlay image", onClip("o", func(c *Clip) { c.FlipH, c.FlipV = true, true }), ""},
		{"crop keyframes through the registry", onClip("v", func(c *Clip) { c.Keyframes = map[string][]Keyframe{"crop.l": {k(0, 0), k(20, 0.95)}} }), ""},
		{"a complete colour entry with keyframes", stack(EffectInstance{ID: "color", On: true, Params: color, Keyframes: map[string][]Keyframe{"exposure": {k(-5, -3), k(10, 3)}}}), ""},
		{"a built-in look with keyframed intensity on the overlay", onClip("o", func(c *Clip) {
			c.Effects = []EffectInstance{{ID: "lut", On: true, Params: map[string]any{"lut": "builtin:teal_orange", "intensity": 0.6}, Keyframes: map[string][]Keyframe{"intensity": {k(0, 0), k(30, 1)}}}}
		}), ""},
		{"the project LUT and a pass-through LUT that is off", stack(entry("lut", map[string]any{"lut": "grade"}), EffectInstance{ID: "lut", Params: map[string]any{"lut": ""}}), ""},
		{"eight effects with empty params", onClip("v", func(c *Clip) { c.Effects = sharpens(8) }), ""},
		{"crop on the audio clip", onClip("a", func(c *Clip) { c.Crop = &Crop{L: 0.1} }), "clips/a/crop"},
		{"a flip on the audio clip", onClip("a", func(c *Clip) { c.FlipV = true }), "clips/a/flip_v"},
		{"effects on the audio clip", onClip("a", func(c *Clip) { c.Effects = sharpens(1) }), "clips/a/effects"},
		{"an all-zero crop", cropV(Crop{}), "clips/v/crop"},
		{"a side above 0.95", cropV(Crop{T: 0.96}), "clips/v/crop"},
		{"a negative side", cropV(Crop{L: -0.1, R: 0.2}), "clips/v/crop"},
		{"t + b above 0.95", cropV(Crop{T: 0.5, B: 0.46}), "clips/v/crop"},
		{"a side that is not a number", cropV(Crop{L: math.NaN()}), "clips/v/crop"},
		{"nine effects", onClip("v", func(c *Clip) { c.Effects = sharpens(9) }), "clips/v/effects"},
		{"513 keyframes on effect entries", stack(tooManyKeys), "clips/v/effects"},
		{"an unknown effect", stack(entry("glow", map[string]any{})), "clips/v/effects/0"},
		{"params missing", stack(EffectInstance{ID: "blur", On: true}), "clips/v/effects/0/params"},
		{"an unknown parameter", stack(entry("blur", map[string]any{"sigma": 2.0})), "clips/v/effects/0/params/sigma"},
		{"a number given as text", stack(entry("color", with(color, "exposure", "0.5"))), "clips/v/effects/0/params/exposure"},
		{"exposure above 3", stack(entry("color", with(color, "exposure", 3.5))), "clips/v/effects/0/params/exposure"},
		{"a temperature below 2000 K", stack(entry("color", with(color, "temperature", 1500.0))), "clips/v/effects/0/params/temperature"},
		{"an unknown built-in look", stack(entry("lut", map[string]any{"lut": "builtin:sepia"})), "clips/v/effects/0/params/lut"},
		{"a LUT ref to the video asset", stack(entry("lut", map[string]any{"lut": "video"})), "clips/v/effects/0/params/lut"},
		{"a LUT ref that is not text", stack(entry("lut", map[string]any{"lut": 3.0})), "clips/v/effects/0/params/lut"},
		{"the second entry carries the error", stack(entry("color", color), entry("sharpen", map[string]any{"strength": 2.0})), "clips/v/effects/1/params/strength"},
		{"keyframes on the blur radius", stack(EffectInstance{ID: "blur", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"radius": {k(0, 0.01)}}}), "clips/v/effects/0/keyframes/radius"},
		{"keyframes on the LUT ref", stack(EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": ""}, Keyframes: map[string][]Keyframe{"lut": {k(0, 0)}}}), "clips/v/effects/0/keyframes/lut"},
		{"keyframes on an unknown parameter", stack(EffectInstance{ID: "sharpen", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"radius": {k(0, 1)}}}), "clips/v/effects/0/keyframes/radius"},
		{"an empty effect keyframe list", strengthKeys(), "clips/v/effects/0/keyframes/strength"},
		{"unsorted effect keyframes", strengthKeys(k(5, 0.5), k(3, 0.5)), "clips/v/effects/0/keyframes/strength"},
		{"an effect keyframe above its range", strengthKeys(k(0, 1.5)), "clips/v/effects/0/keyframes/strength"},
		{"an effect keyframe frame beyond the bound", strengthKeys(k(bound+1, 0.5)), "clips/v/effects/0/keyframes/strength"},
		{"an unknown effect easing", strengthKeys(Keyframe{F: 0, V: 0.5, E: "bounce"}), "clips/v/effects/0/keyframes/strength"},
		{"65 keys on one effect parameter", strengthKeys(run(65, 0.5)...), "clips/v/effects/0/keyframes/strength"},
		{"32 LUT assets", luts(31), ""},
		{"33 LUT assets", luts(32), "assets"},
		{"a LUT grid of 66", onAsset("grade", func(a *Asset) { a.LUTSize = 66 }), "assets/grade/lut_size"},
		{"a LUT grid of 1", onAsset("grade", func(a *Asset) { a.LUTSize = 1 }), "assets/grade/lut_size"},
		{"a LUT asset with a picture size", onAsset("grade", func(a *Asset) { a.Width, a.Height = 64, 64 }), "assets/grade"},
		{"lut_size on a video asset", onAsset("video", func(a *Asset) { a.LUTSize = 17 }), "assets/video/lut_size"},
	}
	for _, c := range cases {
		project := effectTestProject()
		c.edit(&project)
		err := Validate(project)
		var field *FieldError
		switch {
		case c.path == "" && err != nil:
			t.Errorf("%s: Validate = %v, want valid", c.name, err)
		case c.path != "" && (!errors.As(err, &field) || field.Path != c.path):
			t.Errorf("%s: Validate = %v, want a FieldError at %q", c.name, err, c.path)
		}
	}
}

// ValueAt resolves crop.l|t|r|b and effects.<i>.<param> (plan index §4.16) like the base paths: keyframes at the
// source position, else the static value, else the registry default.
func TestValueAtCropAndEffectPaths(t *testing.T) {
	clip := Clip{Start: 100, Offset: 20, Duration: 60, Crop: &Crop{L: 0.1, T: 0.2, R: 0.3, B: 0.05},
		Keyframes: map[string][]Keyframe{"crop.r": {{F: 20, V: 0, E: EaseLinear}, {F: 40, V: 0.4, E: EaseLinear}}},
		Effects: []EffectInstance{
			{ID: "color", On: true, Params: map[string]any{"exposure": 0.5, "lift": "high"}, Keyframes: map[string][]Keyframe{"contrast": {{F: 20, V: 1, E: EaseLinear}, {F: 30, V: 2, E: EaseLinear}}}},
			{ID: "lut", On: false, Params: map[string]any{"lut": "builtin:mono"}},
			{ID: "retired", On: true, Params: map[string]any{"amount": 1.0}},
		}}
	for _, c := range []struct {
		path  string
		frame int
		want  float64
	}{
		{"crop.l", 110, 0.1},
		{"crop.t", 110, 0.2},
		{"crop.b", 110, 0.05},
		{"crop.r", 110, 0.2},                 // source 30, halfway between the keys
		{"effects.0.exposure", 110, 0.5},     // the params value
		{"effects.0.contrast", 105, 1.5},     // entry keyframes at source 25
		{"effects.0.temperature", 110, 6500}, // registry default
		{"effects.0.lift", 110, 0},           // a value of the wrong type falls back to the default
		{"effects.1.intensity", 110, 1},      // an entry that is off still resolves
	} {
		if got, ok := ValueAt(clip, c.path, c.frame); !ok || math.Abs(got-c.want) > 1e-12 {
			t.Errorf("ValueAt(%s, %d) = %v, %v; want %v", c.path, c.frame, got, ok, c.want)
		}
	}
	for _, path := range []string{"effects.1.lut", "effects.2.amount", "effects.3.exposure", "effects.0.nope", "effects.00.exposure", "effects.-1.exposure", "effects.0", "crop.x", "flip_h"} {
		if _, ok := ValueAt(clip, path, 110); ok {
			t.Errorf("ValueAt accepted path %q", path)
		}
	}
	if got, ok := ValueAt(Clip{}, "crop.l", 0); !ok || got != 0 {
		t.Errorf("a clip without crop: ValueAt(crop.l) = %v, %v; want 0, true", got, ok)
	}
}

// The registry rows of plans 2c, 2d and 2e (index §4.2), pinned: the crop, flip, blend and mask base rows after fit,
// the six effects in menu order with their filters and parameters, the eight looks, the fourteen blend modes, the 24
// transitions and the eight animation presets with their filters. TestVideoStudioEffectsRegistryMatchesGo (ui) keeps
// the JS mirror equal.
func TestEffectRegistryRows(t *testing.T) {
	number := func(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }
	line := func(p ParamSpec) string {
		def := fmt.Sprint(p.Default)
		switch value := p.Default.(type) {
		case float64:
			def = number(value)
		case string:
			def = strconv.Quote(value)
		}
		text := p.ID + " " + string(p.Type) + " " + number(p.Min) + ".." + number(p.Max) + " =" + def
		if p.Step != 0 {
			text += " step " + number(p.Step)
		}
		if p.Keyframable {
			text += " kf"
		}
		if p.Unit != "" {
			text += " " + p.Unit
		}
		return text + " " + p.LabelKey
	}
	registry := EffectRegistry()
	if len(registry.Base) < 7 || registry.Base[6].ID != "fit" {
		t.Fatalf("the 2a base rows end with fit: %+v", registry.Base)
	}
	var got []string
	for _, p := range registry.Base[7:] {
		got = append(got, line(p))
	}
	for _, effect := range registry.Effects {
		got = append(got, "effect "+effect.ID+" "+effect.Category+" "+strings.Join(effect.Filters, ",")+" "+effect.LabelKey)
		for _, p := range effect.Params {
			got = append(got, line(p))
		}
	}
	want := []string{
		"crop.l number 0..0.95 =0 step 0.001 kf fraction crop_l",
		"crop.t number 0..0.95 =0 step 0.001 kf fraction crop_t",
		"crop.r number 0..0.95 =0 step 0.001 kf fraction crop_r",
		"crop.b number 0..0.95 =0 step 0.001 kf fraction crop_b",
		"flip_h bool 0..0 =false flip_h",
		"flip_v bool 0..0 =false flip_v",
		`blend choice 0..0 ="normal" blend`,
		`mask.shape choice 0..0 ="rect" mask_shape`,
		"mask.x number -1..1 =0.25 step 0.001 kf fraction mask_x",
		"mask.y number -1..1 =0.25 step 0.001 kf fraction mask_y",
		"mask.w number 0.01..2 =0.5 step 0.001 kf fraction mask_w",
		"mask.h number 0.01..2 =0.5 step 0.001 kf fraction mask_h",
		"mask.rotation number -360..360 =0 step 0.1 kf deg mask_rotation",
		"mask.feather number 0..0.5 =0 step 0.001 kf height mask_feather",
		"mask.invert bool 0..0 =false mask_invert",
		"effect color color lutrgb,colorchannelmixer effect.color.name",
		"exposure number -3..3 =0 step 0.01 kf ev effect.color.exposure",
		"contrast number 0..3 =1 step 0.01 kf effect.color.contrast",
		"saturation number 0..3 =1 step 0.01 kf effect.color.saturation",
		"temperature number 2000..12000 =6500 step 10 kf K effect.color.temperature",
		"tint number -1..1 =0 step 0.01 kf effect.color.tint",
		"lift_x number -1..1 =0 step 0.001 kf wheel effect.color.lift_x",
		"lift_y number -1..1 =0 step 0.001 kf wheel effect.color.lift_y",
		"lift number -0.5..0.5 =0 step 0.001 kf effect.color.lift",
		"gamma_x number -1..1 =0 step 0.001 kf wheel effect.color.gamma_x",
		"gamma_y number -1..1 =0 step 0.001 kf wheel effect.color.gamma_y",
		"gamma number 0.2..5 =1 step 0.01 kf effect.color.gamma",
		"gain_x number -1..1 =0 step 0.001 kf wheel effect.color.gain_x",
		"gain_y number -1..1 =0 step 0.001 kf wheel effect.color.gain_y",
		"gain number 0..4 =1 step 0.01 kf effect.color.gain",
		"effect lut color lut3d,blend effect.lut.name",
		`lut lut 0..0 ="" effect.lut.lut`,
		"intensity number 0..1 =1 step 0.01 kf effect.lut.intensity",
		"effect blur detail gblur,premultiply,unpremultiply effect.blur.name",
		"radius number 0..0.05 =0.01 step 0.0005 height effect.blur.radius",
		"effect sharpen detail cas effect.sharpen.name",
		"strength number 0..1 =0.5 step 0.01 kf effect.sharpen.strength",
		"effect vignette stylize vignette,alphaextract,alphamerge effect.vignette.name",
		"amount number 0..1 =0.5 step 0.01 kf effect.vignette.amount",
		"size number 0.25..2 =1 step 0.01 kf height effect.vignette.size",
		"effect chroma_key key colorkey,despill,alphaextract,alphamerge,blend effect.chroma_key.name",
		`color color 0..0 ="#00b140" effect.chroma_key.color`,
		"similarity number 0.01..1 =0.25 step 0.005 kf effect.chroma_key.similarity",
		"smoothness number 0..1 =0.1 step 0.005 kf effect.chroma_key.smoothness",
		"spill number 0..1 =0.5 step 0.01 kf effect.chroma_key.spill",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("registry rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var looks []string
	for _, look := range registry.Looks {
		if look.LabelKey != "look_"+look.ID {
			t.Errorf("look %q has label key %q", look.ID, look.LabelKey)
		}
		if found, ok := LookByID(look.ID); !ok || found != look {
			t.Errorf("LookByID(%q) = %+v, %v", look.ID, found, ok)
		}
		looks = append(looks, look.ID)
	}
	if want := []string{"warm", "cool", "teal_orange", "mono", "faded_film", "high_contrast", "vintage", "bleach_bypass"}; !slices.Equal(looks, want) {
		t.Fatalf("looks = %v, want %v", looks, want)
	}
	if _, ok := LookByID("sepia"); ok {
		t.Error("LookByID found an unknown look")
	}
	if spec, ok := effectParamSpec("color", "temperature"); !ok || spec.Default != 6500.0 {
		t.Errorf("effectParamSpec(color, temperature) = %+v, %v", spec, ok)
	}
	if _, ok := effectParamSpec("color", "radius"); ok {
		t.Error("effectParamSpec found a parameter of another effect")
	}
	if MaxEffectsPerClip != 8 {
		t.Errorf("MaxEffectsPerClip = %d, want 8", MaxEffectsPerClip)
	}
	// Blend modes (2d): normal has no FFmpeg name; every other mode carries its FFmpeg blend mode name, equal to its id.
	var modes []string
	for _, mode := range registry.BlendModes {
		modes = append(modes, mode.ID+":"+mode.Blend+":"+mode.LabelKey)
	}
	wantModes := []string{"normal::blend_normal"}
	for _, id := range []string{"multiply", "screen", "overlay", "softlight", "hardlight", "darken", "lighten", "difference", "exclusion", "addition", "subtract", "dodge", "burn"} {
		wantModes = append(wantModes, id+":"+id+":blend_"+id)
	}
	if !slices.Equal(modes, wantModes) {
		t.Fatalf("blend modes = %v, want %v", modes, wantModes)
	}
	if mode, ok := BlendModeByID("overlay"); !ok || mode.Blend != "overlay" {
		t.Errorf("BlendModeByID(overlay) = %+v, %v", mode, ok)
	}
	if _, ok := BlendModeByID("glow"); ok {
		t.Error("BlendModeByID found an unknown mode")
	}
	// Transitions (2e): the 2a four keep their ids, mapping and keys. The 20 new rows use their xfade names as ids and
	// transition_<id> as label keys, grouped basic (5), wipe (9), motion (6) and shape (4) in first-appearance order.
	var transitions []string
	for _, spec := range registry.Transitions {
		transitions = append(transitions, string(spec.ID)+":"+spec.XFade+":"+spec.Category+":"+spec.LabelKey)
	}
	wantTransitions := []string{"dissolve:fade:basic:dissolve", "black:fadeblack:basic:black", "wipeleft:wiperight:wipe:wipeleft", "wiperight:wipeleft:wipe:wiperight"}
	for _, row := range [][2]string{
		{"fadewhite", "basic"}, {"fadegrays", "basic"}, {"pixelize", "basic"},
		{"wipeup", "wipe"}, {"wipedown", "wipe"}, {"smoothleft", "wipe"}, {"smoothright", "wipe"}, {"diagtl", "wipe"}, {"diagbr", "wipe"}, {"radial", "wipe"},
		{"slideleft", "motion"}, {"slideright", "motion"}, {"slideup", "motion"}, {"slidedown", "motion"}, {"squeezeh", "motion"}, {"zoomin", "motion"},
		{"circleopen", "shape"}, {"circleclose", "shape"}, {"circlecrop", "shape"}, {"rectcrop", "shape"},
	} {
		wantTransitions = append(wantTransitions, row[0]+":"+row[0]+":"+row[1]+":transition_"+row[0])
	}
	if !slices.Equal(transitions, wantTransitions) {
		t.Fatalf("transitions:\n%s\nwant:\n%s", strings.Join(transitions, "\n"), strings.Join(wantTransitions, "\n"))
	}
	// Animation presets (2e) in menu order, with the filters their export needs (the capability check of Task 9).
	var presets []string
	for _, preset := range registry.AnimPresets {
		presets = append(presets, preset.ID+" "+preset.LabelKey+" "+strings.Join(preset.Filters, ","))
	}
	opacity := "sendcmd,colorchannelmixer"
	wantPresets := []string{
		"fade anim_fade " + opacity,
		"slide_up anim_slide_up " + opacity,
		"slide_down anim_slide_down " + opacity,
		"slide_left anim_slide_left " + opacity,
		"slide_right anim_slide_right " + opacity,
		"pop anim_pop " + opacity,
		"wipe anim_wipe geq,blend",
		"blur_in anim_blur_in gblur,maskedmerge,sendcmd,colorchannelmixer,premultiply,unpremultiply,extractplanes,alphamerge",
	}
	if !slices.Equal(presets, wantPresets) {
		t.Fatalf("animation presets:\n%s\nwant:\n%s", strings.Join(presets, "\n"), strings.Join(wantPresets, "\n"))
	}
}

// An effect is unavailable when FFmpeg lacks one of its filters (index §4.3). The export refuses it; the preview still
// draws it.
func TestRegistryUnavailableEffects(t *testing.T) {
	registry := EffectRegistry()
	for _, c := range []struct{ missing, want []string }{
		{nil, []string{}},
		{[]string{"filter:lut3d"}, []string{"lut"}},
		{[]string{"filter:blend"}, []string{"lut", "chroma_key"}},
		{[]string{"filter:premultiply"}, []string{"blur"}},
		{[]string{"filter:alphaextract"}, []string{"vignette", "chroma_key"}},
		{[]string{"filter:cas", "filter:lutrgb"}, []string{"color", "sharpen"}},
		{[]string{"filter:colorkey"}, []string{"chroma_key"}},
	} {
		if got := registry.Unavailable(c.missing).Effects; !slices.Equal(got, c.want) {
			t.Errorf("Unavailable(%v).Effects = %v, want %v", c.missing, got, c.want)
		}
	}
	// The probe covers every effect filter, including the ones phase2Filters does not name.
	for _, name := range []string{"premultiply", "unpremultiply", "alphaextract"} {
		if !slices.Contains(capabilityFilters(), name) {
			t.Errorf("capabilityFilters() lacks %q", name)
		}
	}
}
