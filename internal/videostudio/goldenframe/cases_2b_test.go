package goldenframe

import (
	"math"
	"slices"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// kfAnimatedClip returns the clip that a 2b case animates (id "k").
func kfAnimatedClip(c Case) (videostudio.Clip, bool) {
	for _, track := range c.Project.Tracks {
		for _, clip := range track.Clips {
			if clip.ID == "k" {
				return clip, true
			}
		}
	}
	return videostudio.Clip{}, false
}

// kfStatic is the clip's own (static) value of a base path: what a painter that ignored keyframes would draw.
func kfStatic(clip videostudio.Clip, path string) float64 {
	switch path {
	case "x":
		return clip.X
	case "y":
		return clip.Y
	case "width":
		return clip.Width
	case "height":
		return clip.Height
	case "rotation":
		return clip.Rotation
	case "opacity":
		return clip.Opacity
	}
	return math.NaN()
}

func TestKeyframeCasesCoverGroups(t *testing.T) {
	// Keyframed values at the case frame, hand-computed from the curves of index §4.18 (cases_2b.go explains each).
	values := map[string]map[string]float64{
		"keyframe_position_linear":   {"x": 0.25, "y": 0.26},
		"keyframe_scale_ease_in_out": {"width": 0.40546875, "height": 0.40546875},
		"keyframe_rotation_ease_out": {"rotation": 70.56},
		"keyframe_opacity_hold":      {"opacity": 0.4},
		"keyframe_before_first":      {"x": 0.1},
		"keyframe_combined_speed2":   {"x": 0.34, "opacity": 0.8},
		"ramp_keyframed":             {"x": 0.3453125},
	}
	// Source frames the ramped clip has consumed at the case frame, S(frame). Ramp clips start at frame 0 with offset 0,
	// so ClipSourceAt gives S(frame) whether it is read as a source position or as consumed frames.
	sources := map[string]float64{"ramp_up_mid": 32.8125, "ramp_slow_middle_mid": 27.5, "ramp_down_late": 56.5625, "ramp_keyframed": 47.25}
	names := []string{"keyframe_position_linear", "keyframe_scale_ease_in_out", "keyframe_rotation_ease_out", "keyframe_opacity_hold",
		"keyframe_before_first", "keyframe_combined_speed2", "ramp_up_mid", "ramp_slow_middle_mid", "ramp_down_late", "ramp_keyframed"}
	m := fakeMedia()
	seen, groups := map[string]bool{}, map[string]int{}
	for _, c := range keyframeCases(m) {
		if !slices.Contains(names, c.Name) {
			t.Errorf("unexpected 2b case %q", c.Name)
			continue
		}
		if seen[c.Name] {
			t.Errorf("duplicate 2b case %q", c.Name)
		}
		seen[c.Name] = true
		group, _, _ := strings.Cut(c.Name, "_")
		groups[group]++
		if c.Threshold != ThresholdGeometry {
			t.Errorf("%s: threshold %.2f, want %.2f", c.Name, c.Threshold, ThresholdGeometry)
		}
		if !slices.Equal(c.Painters, []string{PainterGL, Painter2D}) {
			t.Errorf("%s: painters %v, want [gl 2d]", c.Name, c.Painters)
		}
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		clip, ok := kfAnimatedClip(c)
		if !ok {
			t.Errorf("%s: no animated clip k", c.Name)
			continue
		}
		if c.Frame < clip.Start || c.Frame >= clip.Start+clip.Duration || c.Frame >= videostudio.EnabledDuration(c.Project) {
			t.Errorf("%s: frame %d lies outside the animated clip [%d, %d)", c.Name, c.Frame, clip.Start, clip.Start+clip.Duration)
		}
		if group == "keyframe" && len(clip.Keyframes) == 0 {
			t.Errorf("%s: the animated clip has no keyframes", c.Name)
		}
		for path, want := range values[c.Name] {
			got, ok := videostudio.ValueAt(clip, path, c.Frame)
			if !ok || math.Abs(got-want) > 1e-9 {
				t.Errorf("%s: %s at frame %d is %v (ok %v), want %v", c.Name, path, c.Frame, got, ok, want)
			}
			if math.Abs(kfStatic(clip, path)-want) < 0.05 {
				t.Errorf("%s: the static %s %v is too close to the keyframed %v, so ignored keyframes would pass", c.Name, path, kfStatic(clip, path), want)
			}
		}
		want, ramped := sources[c.Name]
		if !ramped {
			if group == "ramp" {
				t.Errorf("%s: no expected source position", c.Name)
			}
			continue
		}
		if len(clip.Ramp) < 2 || clip.Start != 0 || clip.Offset != 0 {
			t.Errorf("%s: a ramp case animates a ramped clip that starts at frame 0 with offset 0", c.Name)
			continue
		}
		s := videostudio.ClipSourceAt(clip, float64(c.Frame))
		if math.Abs(s-want) > 1e-9 {
			t.Errorf("%s: S(%d) = %v, want %v", c.Name, c.Frame, s, want)
		}
		// Away from whole frames, so the export's 1 ms snapping tolerance and float rounding in the preview's seek
		// cannot pick a neighbouring source frame.
		if frac := s - math.Floor(s); frac < 0.1 || frac > 0.9 {
			t.Errorf("%s: S(%d) = %v lies within 0.1 of a whole source frame", c.Name, c.Frame, s)
		}
		if math.Floor(s) == float64(c.Frame) {
			t.Errorf("%s: frame %d shows source frame %v with and without the ramp", c.Name, c.Frame, math.Floor(s))
		}
	}
	for _, name := range names {
		if !seen[name] {
			t.Errorf("2b case %q is missing", name)
		}
	}
	if groups["keyframe"] != 6 || groups["ramp"] != 4 {
		t.Errorf("groups %v, want 6 keyframe and 4 ramp cases", groups)
	}
	registered := map[string]bool{}
	for _, c := range Cases(m) {
		registered[c.Name] = true
	}
	for _, name := range names {
		if !registered[name] {
			t.Errorf("Cases does not list %q: keyframeCases is not registered", name)
		}
	}
	count := 0
	for _, item := range mediaSet {
		if item.name == MediaTicker {
			count++
		}
	}
	if count != 1 {
		t.Errorf("MediaTicker is registered %d times, want once", count)
	}
}
