package videostudio

import (
	"errors"
	"math"
	"testing"
)

// refusedWithoutPath marks a case that Validate must refuse without a field path. validateKeyframes checks mask key
// values against the base rows and returns plain errors, as for every base path.
const refusedWithoutPath = "(refused)"

// Blend modes, masks and the chroma_key entry (plan 2d, index §4.1, §4.2). validateBlend and validateMask refuse with
// a *FieldError at the path of the offending field; path "" means Validate must accept the project.
// TestVideoStudioPictureValidationMatchesGo (ui) runs the same rows through the JS mirror, except the non-finite
// numbers, which JSON cannot carry.
func TestValidateCompositing(t *testing.T) {
	k := func(f int, v float64) Keyframe { return Keyframe{F: f, V: v, E: EaseLinear} }
	blendOn := func(id, mode string) func(*Project) { return onClip(id, func(c *Clip) { c.Blend = mode }) }
	validMask := func() *Mask {
		return &Mask{Shape: MaskEllipse, X: 0.25, Y: 0.2, W: 0.5, H: 0.6, Rotation: 15, Feather: 0.04}
	}
	same := func(*Mask) {}
	maskV := func(edit func(*Mask), keys map[string][]Keyframe) func(*Project) {
		return onClip("v", func(c *Clip) {
			mask := validMask()
			edit(mask)
			c.Mask, c.Keyframes = mask, keys
		})
	}
	keyEntry := func(params map[string]any, keys map[string][]Keyframe) func(*Project) {
		return onClip("v", func(c *Clip) {
			c.Effects = []EffectInstance{{ID: "chroma_key", On: true, Params: params, Keyframes: keys}}
		})
	}
	keyWith := func(name string, value any) map[string]any {
		params := effectDefaults("chroma_key")
		params[name] = value
		return params
	}
	cases := []struct {
		name string
		edit func(*Project)
		path string
	}{
		{"multiply on the video clip", blendOn("v", "multiply"), ""},
		{"an empty blend is normal", blendOn("v", ""), ""},
		{"an ellipse mask with keys on every keyframable path", maskV(same, map[string][]Keyframe{
			"mask.x": {k(0, -1), k(20, 1)}, "mask.y": {k(0, 0.1)}, "mask.w": {k(0, 0.01), k(30, 2)},
			"mask.h": {k(5, 0.5)}, "mask.rotation": {k(0, -360), k(10, 360)}, "mask.feather": {k(0, 0), k(10, 0.5)},
		}), ""},
		{"a rect mask at its bounds on the overlay image", onClip("o", func(c *Clip) {
			c.Mask = &Mask{Shape: MaskRect, X: -1, Y: 1, W: 2, H: 0.01, Rotation: -360, Feather: 0.5, Invert: true}
		}), ""},
		{"chroma_key at its defaults with keyed similarity and smoothness", keyEntry(effectDefaults("chroma_key"),
			map[string][]Keyframe{"similarity": {k(0, 0.01), k(30, 1)}, "smoothness": {k(0, 0)}}), ""},
		{"an upper-case key colour", keyEntry(keyWith("color", "#00B140"), nil), ""},
		{"blend normal", blendOn("v", "normal"), "clips/v/blend"},
		{"an unknown blend mode", blendOn("v", "glow"), "clips/v/blend"},
		{"a blend mode on the audio clip", blendOn("a", "multiply"), "clips/a/blend"},
		{"a mask on the audio clip", onClip("a", func(c *Clip) { c.Mask = validMask() }), "clips/a/mask"},
		{"mask keys without a mask", onClip("v", func(c *Clip) { c.Keyframes = map[string][]Keyframe{"mask.x": {k(0, 0.1)}} }), "clips/v/mask"},
		{"a mask shape star", maskV(func(m *Mask) { m.Shape = "star" }, nil), "clips/v/mask/shape"},
		{"a mask without a shape", maskV(func(m *Mask) { m.Shape = "" }, nil), "clips/v/mask/shape"},
		{"mask x above 1", maskV(func(m *Mask) { m.X = 1.5 }, nil), "clips/v/mask/x"},
		{"mask y below -1", maskV(func(m *Mask) { m.Y = -1.5 }, nil), "clips/v/mask/y"},
		{"mask w below 0.01", maskV(func(m *Mask) { m.W = 0.005 }, nil), "clips/v/mask/w"},
		{"mask h above 2", maskV(func(m *Mask) { m.H = 2.5 }, nil), "clips/v/mask/h"},
		{"mask rotation above 360", maskV(func(m *Mask) { m.Rotation = 400 }, nil), "clips/v/mask/rotation"},
		{"mask feather above 0.5", maskV(func(m *Mask) { m.Feather = 0.6 }, nil), "clips/v/mask/feather"},
		{"a mask feather that is not a number", maskV(func(m *Mask) { m.Feather = math.NaN() }, nil), "clips/v/mask/feather"},
		{"an infinite mask x", maskV(func(m *Mask) { m.X = math.Inf(1) }, nil), "clips/v/mask/x"},
		{"a mask key out of range", maskV(same, map[string][]Keyframe{"mask.w": {k(0, 3)}}), refusedWithoutPath},
		{"a mask key below the 1 % size", maskV(same, map[string][]Keyframe{"mask.h": {k(0, 0.005)}}), refusedWithoutPath},
		{"a key colour that is not #rrggbb", keyEntry(keyWith("color", "green"), nil), "clips/v/effects/0/params/color"},
		{"similarity below 0.01", keyEntry(keyWith("similarity", 0.0), nil), "clips/v/effects/0/params/similarity"},
		{"keyframes on the key colour", keyEntry(effectDefaults("chroma_key"), map[string][]Keyframe{"color": {k(0, 0)}}), "clips/v/effects/0/keyframes/color"},
	}
	for _, c := range cases {
		project := effectTestProject()
		c.edit(&project)
		err := Validate(project)
		var field *FieldError
		switch {
		case c.path == "" && err != nil:
			t.Errorf("%s: Validate = %v, want valid", c.name, err)
		case c.path == refusedWithoutPath && err == nil:
			t.Errorf("%s: Validate accepted the project", c.name)
		case c.path != "" && c.path != refusedWithoutPath && (!errors.As(err, &field) || field.Path != c.path):
			t.Errorf("%s: Validate = %v, want a FieldError at %q", c.name, err, c.path)
		}
	}
	// Every registry mode is accepted on a visual clip; normal is the absent field and refused by name.
	for _, mode := range EffectRegistry().BlendModes {
		project := effectTestProject()
		onClip("o", func(c *Clip) { c.Blend = mode.ID })(&project)
		err := Validate(project)
		var field *FieldError
		if mode.ID == BlendNormal {
			if !errors.As(err, &field) || field.Path != "clips/o/blend" {
				t.Errorf("blend %q: Validate = %v, want a FieldError at clips/o/blend", mode.ID, err)
			}
		} else if err != nil {
			t.Errorf("blend %q: Validate = %v, want valid", mode.ID, err)
		}
	}
}

// ValueAt resolves mask.x|y|w|h|rotation|feather like the other base paths (plan index §4.16): keyframes at the source
// position, else the clip's mask, else the registry default (the value of a new mask). Shape and invert are not numbers.
func TestValueAtMaskPaths(t *testing.T) {
	clip := Clip{Start: 100, Offset: 20, Duration: 60,
		Mask:      &Mask{Shape: MaskEllipse, X: 0.1, Y: 0.2, W: 0.5, H: 0.4, Rotation: 30, Feather: 0.05, Invert: true},
		Keyframes: map[string][]Keyframe{"mask.w": {{F: 20, V: 0.2, E: EaseLinear}, {F: 40, V: 0.6, E: EaseLinear}}}}
	for _, c := range []struct {
		path  string
		frame int
		want  float64
	}{
		{"mask.x", 110, 0.1},
		{"mask.y", 110, 0.2},
		{"mask.h", 110, 0.4},
		{"mask.rotation", 110, 30},
		{"mask.feather", 110, 0.05},
		{"mask.w", 110, 0.4}, // source 30, halfway between the keys
		{"mask.w", 90, 0.2},  // source 10, before the first key
	} {
		if got, ok := ValueAt(clip, c.path, c.frame); !ok || math.Abs(got-c.want) > 1e-12 {
			t.Errorf("ValueAt(%s, %d) = %v, %v; want %v", c.path, c.frame, got, ok, c.want)
		}
	}
	for _, path := range []string{"mask.shape", "mask.invert", "mask.z", "mask", "blend"} {
		if _, ok := ValueAt(clip, path, 110); ok {
			t.Errorf("ValueAt accepted path %q", path)
		}
	}
	for path, want := range map[string]float64{"mask.x": 0.25, "mask.y": 0.25, "mask.w": 0.5, "mask.h": 0.5, "mask.rotation": 0, "mask.feather": 0} {
		if got, ok := ValueAt(Clip{}, path, 0); !ok || got != want {
			t.Errorf("a clip without a mask: ValueAt(%s) = %v, %v; want the registry default %v", path, got, ok, want)
		}
	}
}
