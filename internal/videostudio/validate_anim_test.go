package videostudio

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// In and out animations (plan 2e, index §4.1, §4.24). validateAnim refuses with a *FieldError at the path of the
// offending field; path "" means Validate must accept the project. The duration range follows the project rate
// (AnimFrameRange), and a duration longer than the clip is valid: the render clamps each span to half the clip.
func TestValidateAnim(t *testing.T) {
	anim := func(preset string, duration int) *ClipAnim { return &ClipAnim{Preset: preset, Duration: duration} }
	in := func(id string, a *ClipAnim) func(*Project) { return onClip(id, func(c *Clip) { c.AnimIn = a }) }
	out := func(id string, a *ClipAnim) func(*Project) { return onClip(id, func(c *Clip) { c.AnimOut = a }) }
	all := func(edits ...func(*Project)) func(*Project) {
		return func(project *Project) {
			for _, edit := range edits {
				edit(project)
			}
		}
	}
	rate := func(fps int, ntsc bool) func(*Project) {
		return func(project *Project) { project.FPS, project.NTSC = fps, ntsc }
	}
	cases := []struct {
		name string
		edit func(*Project)
		path string
	}{
		{"pop in and fade out on the video clip", all(in("v", anim("pop", 13)), out("v", anim("fade", 10))), ""},
		{"a slide on the overlay image", in("o", anim("slide_up", 15)), ""},
		{"30 fps: 90 frames on a 60-frame clip (the render clamps)", out("o", anim("blur_in", 90)), ""},
		{"an in animation on the audio clip", in("a", anim("fade", 15)), "clips/a/anim_in"},
		{"an out animation on the audio clip", out("a", anim("fade", 15)), "clips/a/anim_out"},
		{"an empty preset", in("v", anim("", 15)), "clips/v/anim_in/preset"},
		{"an unknown preset", out("v", anim("bounce", 15)), "clips/v/anim_out/preset"},
		{"none is no preset (the editor deletes the field)", in("o", anim("none", 15)), "clips/o/anim_in/preset"},
		{"a zero duration", in("o", anim("pop", 0)), "clips/o/anim_in/duration"},
		{"a negative duration", out("o", anim("pop", -6)), "clips/o/anim_out/duration"},
		{"25 fps: 4 frames, one below the minimum", all(rate(25, false), in("v", anim("fade", 4))), "clips/v/anim_in/duration"},
		{"25 fps: 5 frames, the minimum", all(rate(25, false), in("v", anim("fade", 5))), ""},
		{"25 fps: 75 frames, the maximum", all(rate(25, false), out("v", anim("fade", 75))), ""},
		{"25 fps: 76 frames, one above the maximum", all(rate(25, false), out("v", anim("fade", 76))), "clips/v/anim_out/duration"},
		{"29.97 fps: 5 frames, one below the minimum", all(rate(30, true), in("o", anim("wipe", 5))), "clips/o/anim_in/duration"},
		{"29.97 fps: 6 frames, the minimum", all(rate(30, true), in("o", anim("wipe", 6))), ""},
		{"29.97 fps: 90 frames, the maximum", all(rate(30, true), out("o", anim("wipe", 90))), ""},
		{"29.97 fps: 91 frames, one above the maximum", all(rate(30, true), out("o", anim("wipe", 91))), "clips/o/anim_out/duration"},
		{"the out side is checked after a valid in side", all(in("v", anim("fade", 15)), out("v", anim("glow", 15))), "clips/v/anim_out/preset"},
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
	// Every registry preset validates in and out on a visual clip.
	presets := EffectRegistry().AnimPresets
	if len(presets) != 8 {
		t.Fatalf("animation presets = %d, want 8", len(presets))
	}
	for _, preset := range presets {
		project := effectTestProject()
		onClip("o", func(c *Clip) { c.AnimIn, c.AnimOut = anim(preset.ID, 15), anim(preset.ID, 20) })(&project)
		if err := Validate(project); err != nil {
			t.Errorf("preset %q: Validate = %v, want valid", preset.ID, err)
		}
	}
	// JSON (index §4.1): anim_in and anim_out follow effects, each with preset and duration; absent without animation.
	data, err := json.Marshal(Clip{ID: "c", AnimIn: anim("pop", 13), AnimOut: anim("fade", 10)})
	if err != nil || !strings.HasSuffix(string(data), `"anim_in":{"preset":"pop","duration":13},"anim_out":{"preset":"fade","duration":10}}`) {
		t.Fatalf("clip JSON = %s, %v", data, err)
	}
	if data, _ := json.Marshal(Clip{ID: "c"}); strings.Contains(string(data), "anim_") {
		t.Fatalf("a clip without animation carries %s", data)
	}
}

// AnimFrameRange (index §4.24): [max(1, round(0.2·fps)), round(3·fps)] with the default round(0.5·fps), rounded half
// away from zero, for every frame rate a project may use. An unset rate is the 30 fps default, as in ProjectRate.
func TestAnimFrameRange(t *testing.T) {
	if AnimMinSeconds != 0.2 || AnimMaxSeconds != 3 || AnimDefaultSeconds != 0.5 {
		t.Fatalf("seconds = %v..%v (default %v), want 0.2..3 (0.5)", AnimMinSeconds, AnimMaxSeconds, AnimDefaultSeconds)
	}
	want := map[FrameRateOption][3]int{
		{FPS: 24}: {5, 72, 12}, {FPS: 24, NTSC: true}: {5, 72, 12}, {FPS: 25}: {5, 75, 13},
		{FPS: 30}: {6, 90, 15}, {FPS: 30, NTSC: true}: {6, 90, 15}, {FPS: 50}: {10, 150, 25},
		{FPS: 60}: {12, 180, 30}, {FPS: 60, NTSC: true}: {12, 180, 30},
	}
	options := FrameRateOptions()
	if len(options) != len(want) {
		t.Fatalf("FrameRateOptions() = %v; the table covers %d rates", options, len(want))
	}
	for _, option := range options {
		minFrames, maxFrames, defaultFrames := AnimFrameRange(ProjectRate(Project{FPS: option.FPS, NTSC: option.NTSC}))
		if got := [3]int{minFrames, maxFrames, defaultFrames}; got != want[option] {
			t.Errorf("AnimFrameRange(%+v) = %v, want %v", option, got, want[option])
		}
	}
	if minFrames, maxFrames, defaultFrames := AnimFrameRange(Rate{}); minFrames != 6 || maxFrames != 90 || defaultFrames != 15 {
		t.Errorf("an unset rate = %d..%d (%d), want the 30 fps range 6..90 (15)", minFrames, maxFrames, defaultFrames)
	}
}
