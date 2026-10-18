package videostudio

import (
	"math"
	"testing"
)

func TestEasingCurves(t *testing.T) {
	for _, c := range []struct {
		e    Easing
		t    float64
		want float64
	}{
		{EaseLinear, 0.25, 0.25},
		{EaseIn, 0.5, 0.125},
		{EaseOut, 0.5, 0.875},
		{EaseInOut, 0.25, 0.0625},
		{EaseInOut, 0.75, 0.9375},
		{EaseHold, 0.999, 0},
		{EaseHold, 1, 1},
		{EaseIn, 1.5, 1},
		{EaseOut, -0.5, 0},
		{EaseInOut, math.NaN(), 0},
		{Easing("bounce"), 0.3, 0.3},
	} {
		if got := Ease(c.e, c.t); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("Ease(%s, %v) = %v, want %v", c.e, c.t, got, c.want)
		}
	}
	// Every curve runs from 0 to 1 without leaving 0..1 and never falls, so eased values stay between their keys.
	for _, e := range []Easing{EaseLinear, EaseIn, EaseOut, EaseInOut, EaseHold} {
		if Ease(e, 0) != 0 || Ease(e, 1) != 1 {
			t.Errorf("%s does not run from 0 to 1", e)
		}
		prev := 0.0
		for i := 1; i <= 100; i++ {
			v := Ease(e, float64(i)/100)
			if v < prev || v > 1 {
				t.Fatalf("%s leaves 0..1 or falls at %d%%: %v after %v", e, i, v, prev)
			}
			prev = v
		}
	}
}

// Before the first key its value, between keys the left key's easing (linear at 15, ease_in at 25, hold at 35),
// from the last key on its value; NaN reads as before the first key.
func TestSampleKeyframes(t *testing.T) {
	keys := []Keyframe{{F: 10, V: 1, E: EaseLinear}, {F: 20, V: 3, E: EaseIn}, {F: 30, V: 2, E: EaseHold}, {F: 40, V: 5, E: EaseOut}}
	for _, c := range []struct{ f, want float64 }{
		{-100, 1}, {10, 1}, {15, 2}, {20, 3}, {25, 2.875}, {35, 2}, {39.999, 2}, {40, 5}, {1e9, 5}, {math.NaN(), 1},
	} {
		if got := sampleKeyframes(keys, c.f); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("sampleKeyframes at %v = %v, want %v", c.f, got, c.want)
		}
	}
	if got := sampleKeyframes(nil, 3); got != 0 {
		t.Errorf("an empty list gives %v, want 0", got)
	}
}

// Keyframes are anchored to the content (spec §5.1): the same source frame shows the same value at any speed.
func TestValueAtAnchorsKeyframesToContent(t *testing.T) {
	clip := Clip{Start: 100, Offset: 30, Duration: 60, X: 0.9, Opacity: 0.7, Keyframes: map[string][]Keyframe{
		"x": {{F: 30, V: 0, E: EaseLinear}, {F: 50, V: 0.5, E: EaseLinear}},
	}}
	fast, slow := clip, clip
	fast.Speed, slow.Speed = 2, 0.5
	for _, c := range []struct {
		name  string
		clip  Clip
		frame int
		want  float64
	}{
		{"speed 1 shows source 40 at frame 110", clip, 110, 0.25},
		{"speed 2 shows source 40 at frame 105", fast, 105, 0.25},
		{"speed 0.5 shows source 40 at frame 120", slow, 120, 0.25},
		{"before the clip the first value holds", clip, 90, 0},
		{"after the last key the last value holds", clip, 200, 0.5},
	} {
		got, ok := ValueAt(c.clip, "x", c.frame)
		if !ok || math.Abs(got-c.want) > 1e-12 {
			t.Errorf("%s: ValueAt = %v, %v; want %v", c.name, got, ok, c.want)
		}
	}
	if got, ok := ValueAt(clip, "opacity", 120); !ok || got != 0.7 {
		t.Errorf("a path without keyframes keeps its static value: %v, %v", got, ok)
	}
	for _, path := range []string{"fit", "volume", "flip_h", "effects.0.radius", ""} {
		if _, ok := ValueAt(clip, path, 120); ok {
			t.Errorf("ValueAt accepted path %q", path)
		}
	}
	still := Clip{Start: 50, Duration: 75, Keyframes: map[string][]Keyframe{"opacity": {{F: 0, V: 0, E: EaseLinear}, {F: 25, V: 1, E: EaseLinear}}}}
	if got, _ := ValueAt(still, "opacity", 60); math.Abs(got-0.4) > 1e-12 {
		t.Errorf("a still uses the same formula with offset 0: %v, want 0.4", got)
	}
}

func TestKeyframeClipFrameAndAnimation(t *testing.T) {
	clip := Clip{Start: 0, Duration: 50, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.4, Rotation: 10, Opacity: 0.8}
	if geometry, opacity := clipAnimation(clip); geometry || opacity {
		t.Fatalf("a clip without keyframes is animated: geometry %v, opacity %v", geometry, opacity)
	}
	if got, want := clipFrameAt(clip, 12), (clipFrame{X: 0.1, Y: 0.2, Width: 0.5, Height: 0.4, Rotation: 10, Opacity: 0.8}); got != want {
		t.Fatalf("static clipFrameAt = %+v, want %+v", got, want)
	}
	clip.Keyframes = map[string][]Keyframe{"opacity": {{F: 0, V: 0, E: EaseLinear}, {F: 20, V: 1, E: EaseLinear}}}
	if geometry, opacity := clipAnimation(clip); geometry || !opacity {
		t.Fatalf("opacity keyframes: geometry %v, opacity %v", geometry, opacity)
	}
	if got := clipFrameAt(clip, 5); math.Abs(got.Opacity-0.25) > 1e-12 || got.X != 0.1 || got.Rotation != 10 {
		t.Fatalf("clipFrameAt with opacity keyframes = %+v", got)
	}
	clip.Keyframes["rotation"] = []Keyframe{{F: 0, V: 45, E: EaseHold}}
	if geometry, opacity := clipAnimation(clip); !geometry || !opacity {
		t.Fatalf("rotation and opacity keyframes: geometry %v, opacity %v", geometry, opacity)
	}
	if got := clipFrameAt(clip, 30); got.Rotation != 45 || got.Opacity != 1 {
		t.Fatalf("a single key replaces the static value and the last key holds: %+v", got)
	}
}

// 2e: an in/out animation counts like keyframes (index §4.10). Slides and pop animate the geometry; every preset with an
// opacity part animates the opacity. Both hold for the whole clip, and only with an effective span. A masked clip with
// a pop takes the per-frame mask; a fade keeps the static one.
func TestClipAnimationIncludesInOutAnimation(t *testing.T) {
	for _, c := range []struct {
		preset            string
		geometry, opacity bool
	}{{"fade", false, true}, {"slide_up", true, true}, {"slide_down", true, true}, {"slide_left", true, true}, {"slide_right", true, true},
		{"pop", true, true}, {"wipe", false, false}, {"blur_in", false, true}} {
		for side, clip := range map[string]Clip{
			"in":  {Duration: 40, AnimIn: &ClipAnim{Preset: c.preset, Duration: 10}},
			"out": {Duration: 40, AnimOut: &ClipAnim{Preset: c.preset, Duration: 10}},
		} {
			if geometry, opacity := clipAnimation(clip); geometry != c.geometry || opacity != c.opacity {
				t.Errorf("%s %s: geometry %v, opacity %v; want %v, %v", c.preset, side, geometry, opacity, c.geometry, c.opacity)
			}
		}
	}
	if geometry, opacity := clipAnimation(Clip{Duration: 1, AnimIn: &ClipAnim{Preset: "pop", Duration: 10}}); geometry || opacity {
		t.Errorf("a pop without an effective span animates: geometry %v, opacity %v", geometry, opacity)
	}
	masked := Clip{Duration: 40, Mask: &Mask{Shape: MaskRect, W: 0.5, H: 0.5}, AnimIn: &ClipAnim{Preset: "pop", Duration: 10}}
	if !maskPerFrame(masked) {
		t.Error("a masked pop keeps the static mask; the mask must follow the animated box")
	}
	masked.AnimIn.Preset = "fade"
	if maskPerFrame(masked) {
		t.Error("a masked fade takes the per-frame mask; its box does not move")
	}
}
