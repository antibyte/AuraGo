package videostudio

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// animationProject holds three clips that validate: "v" (a 300-frame video on a video track, offset 10), "o" (an
// image on an overlay track) and "a" (a song on an audio track), each 60 frames long at 30 fps.
func animationProject() Project {
	project := validProject()
	project.Assets = append(project.Assets,
		Asset{ID: "logo", Name: "logo.png", Kind: AssetImage, Width: 64, Height: 64},
		Asset{ID: "song", Name: "song.mp3", Kind: AssetAudio, DurationFrames: 600, HasAudio: true})
	project.Tracks = []Track{
		{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "v", AssetID: "video", Offset: 10, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
		{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{{ID: "o", AssetID: "logo", Duration: 60, X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2, Opacity: 1, Volume: 1, Fit: FitContain}}},
		{ID: "audio", Name: "Audio", Kind: TrackAudio, Clips: []Clip{{ID: "a", AssetID: "song", Duration: 60, Volume: 1}}},
	}
	return project
}

// editAnimationClip returns animationProject with edit applied to the clip with the given id.
func editAnimationClip(id string, edit func(*Clip)) Project {
	project := animationProject()
	for ti := range project.Tracks {
		for ci := range project.Tracks[ti].Clips {
			if project.Tracks[ti].Clips[ci].ID == id {
				edit(&project.Tracks[ti].Clips[ci])
			}
		}
	}
	return project
}

// animationCase edits one clip of animationProject; want is "" when Validate must accept the result, else part of the
// error it must return.
type animationCase struct {
	name, id string
	edit     func(*Clip)
	want     string
}

func runAnimationCases(t *testing.T, cases []animationCase) {
	t.Helper()
	for _, c := range cases {
		err := Validate(editAnimationClip(c.id, c.edit))
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: Validate = %v, want valid", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: Validate = %v, want an error containing %q", c.name, err, c.want)
		}
	}
}

// Keyframe structure (plan index §4.18): visual tracks only, keyframable number paths, 1..64 per path and 512 per
// clip, strictly increasing integer frames within ±MaxClipSpeed·maxFrames, values in the registry range (width and
// height at least MinBoxSize, 1 %), known easings. No cross-parameter check.
func TestValidateKeyframes(t *testing.T) {
	key := func(f int, v float64, e Easing) Keyframe { return Keyframe{F: f, V: v, E: e} }
	run := func(n int, v float64) []Keyframe {
		keys := make([]Keyframe, n)
		for i := range keys {
			keys[i] = key(i, v, EaseLinear)
		}
		return keys
	}
	set := func(keyframes map[string][]Keyframe) func(*Clip) {
		return func(clip *Clip) { clip.Keyframes = keyframes }
	}
	tooMany := map[string][]Keyframe{}
	for i := 0; i < 9; i++ {
		tooMany[fmt.Sprintf("p%d", i)] = run(64, 1)
	}
	bound := MaxClipSpeed * MaxDurationFrames(animationProject())
	runAnimationCases(t, []animationCase{
		{name: "keyframes on the video clip", id: "v", edit: set(map[string][]Keyframe{"x": {key(0, 0, EaseLinear), key(30, 0.5, EaseInOut)}, "opacity": {key(10, 1, EaseHold)}})},
		{name: "every easing on the overlay clip, frames before the source start", id: "o", edit: set(map[string][]Keyframe{"rotation": {key(-20, -360, EaseIn), key(0, 0, EaseOut), key(5, 45.5, EaseInOut), key(9, 360, EaseHold), key(12, 0, EaseLinear)}})},
		{name: "no cross-parameter check (x + width may pass 1)", id: "v", edit: set(map[string][]Keyframe{"x": {key(0, 0.9, EaseLinear)}, "width": {key(0, 1, EaseLinear)}})},
		{name: "64 keys on one path", id: "v", edit: set(map[string][]Keyframe{"opacity": run(64, 1)})},
		{name: "frames at the bound", id: "v", edit: set(map[string][]Keyframe{"x": {key(-bound, 0, EaseLinear), key(bound, 1, EaseLinear)}})},
		{name: "an empty map", id: "v", edit: set(map[string][]Keyframe{})},
		{name: "keyframes on an audio clip", id: "a", edit: set(map[string][]Keyframe{"opacity": {key(0, 1, EaseLinear)}}), want: "audio track"},
		{name: "an unknown path", id: "v", edit: set(map[string][]Keyframe{"volume": {key(0, 1, EaseLinear)}}), want: "unsupported path"},
		{name: "a choice parameter", id: "v", edit: set(map[string][]Keyframe{"fit": {key(0, 1, EaseLinear)}}), want: "unsupported path"},
		{name: "an empty list", id: "v", edit: set(map[string][]Keyframe{"x": {}}), want: "is empty"},
		{name: "65 keys on one path", id: "v", edit: set(map[string][]Keyframe{"opacity": run(65, 1)}), want: "more than 64"},
		{name: "more than 512 keys per clip", id: "v", edit: set(tooMany), want: "more than 512"},
		{name: "unsorted frames", id: "v", edit: set(map[string][]Keyframe{"x": {key(5, 0, EaseLinear), key(3, 1, EaseLinear)}}), want: "strictly increasing"},
		{name: "a repeated frame", id: "v", edit: set(map[string][]Keyframe{"x": {key(5, 0, EaseLinear), key(5, 1, EaseLinear)}}), want: "strictly increasing"},
		{name: "a frame beyond the bound", id: "v", edit: set(map[string][]Keyframe{"x": {key(bound+1, 0, EaseLinear)}}), want: "out of range"},
		{name: "a value outside the range", id: "v", edit: set(map[string][]Keyframe{"x": {key(0, 1.5, EaseLinear)}}), want: "invalid keyframe value"},
		{name: "width 0", id: "v", edit: set(map[string][]Keyframe{"width": {key(0, 0, EaseLinear)}}), want: "invalid keyframe value"},
		// Boxes the editor makes are at least 1 % wide and tall (MinBoxSize); a sliver would make a cover bound of many
		// canvas widths.
		{name: "width at the 1 % minimum", id: "v", edit: set(map[string][]Keyframe{"width": {key(0, MinBoxSize, EaseLinear), key(10, 1, EaseLinear)}})},
		{name: "height at the 1 % minimum", id: "v", edit: set(map[string][]Keyframe{"height": {key(0, 1, EaseLinear), key(10, MinBoxSize, EaseLinear)}})},
		{name: "width below 1 %", id: "v", edit: set(map[string][]Keyframe{"width": {key(0, 0.009, EaseLinear)}}), want: "invalid keyframe value"},
		{name: "height below 1 %", id: "v", edit: set(map[string][]Keyframe{"height": {key(0, 1, EaseLinear), key(10, 0.0001, EaseLinear)}}), want: "invalid keyframe value"},
		{name: "a small opacity is no box", id: "v", edit: set(map[string][]Keyframe{"opacity": {key(0, 0.001, EaseLinear)}})},
		{name: "rotation beyond 360", id: "o", edit: set(map[string][]Keyframe{"rotation": {key(0, 400, EaseLinear)}}), want: "invalid keyframe value"},
		{name: "not a number", id: "v", edit: set(map[string][]Keyframe{"opacity": {key(0, math.NaN(), EaseLinear)}}), want: "invalid keyframe value"},
		{name: "an unknown easing", id: "v", edit: set(map[string][]Keyframe{"x": {key(0, 0, "bounce")}}), want: "unsupported easing"},
		{name: "a missing easing", id: "v", edit: set(map[string][]Keyframe{"x": {key(0, 0, "")}}), want: "unsupported easing"},
	})
}

// Ramp shape (plan index §4.18): video track with a video asset only, 2..16 points, strictly increasing integer
// frames within ±maxFrames (points outside the clip are kept), speeds 0.25..16, known easings, and Speed absent or 1.
func TestValidateRampShape(t *testing.T) {
	point := func(f int, speed float64, e Easing) RampPoint { return RampPoint{F: f, Speed: speed, E: e} }
	ramp := func(points ...RampPoint) func(*Clip) {
		return func(clip *Clip) { clip.Ramp = points }
	}
	easings := []Easing{EaseLinear, EaseIn, EaseOut, EaseInOut, EaseHold}
	sixteen, seventeen := make([]RampPoint, 16), make([]RampPoint, 17)
	for i := range sixteen {
		sixteen[i] = point(-20+10*i, []float64{0.25, 0.5, 1, 2}[i%4], easings[i%5])
	}
	for i := range seventeen {
		seventeen[i] = point(i, 1, EaseLinear)
	}
	maxFrames := MaxDurationFrames(animationProject())
	two := []RampPoint{point(0, 1, EaseInOut), point(30, 2, EaseLinear)}
	runAnimationCases(t, []animationCase{
		{name: "a two-point ramp", id: "v", edit: ramp(two...)},
		{name: "sixteen points with every easing, outside the clip too", id: "v", edit: ramp(sixteen...)},
		{name: "speed 1 beside a ramp", id: "v", edit: func(clip *Clip) { clip.Speed, clip.Ramp = 1, two }},
		{name: "bounds", id: "v", edit: ramp(point(-maxFrames, 0.25, EaseLinear), point(0, 0.25, EaseHold), point(maxFrames, 16, EaseLinear))},
		{name: "a ramp on the overlay image", id: "o", edit: ramp(two...), want: "speed ramp only"},
		{name: "a ramp on a still on the video track", id: "v", edit: func(clip *Clip) { clip.AssetID, clip.Offset, clip.Ramp = "logo", 0, two }, want: "speed ramp only"},
		{name: "a ramp on the audio clip", id: "a", edit: ramp(two...), want: "speed ramp only"},
		{name: "one point", id: "v", edit: ramp(point(0, 2, EaseLinear)), want: "2 to 16"},
		{name: "seventeen points", id: "v", edit: ramp(seventeen...), want: "2 to 16"},
		{name: "a constant speed beside a ramp", id: "v", edit: func(clip *Clip) { clip.Speed, clip.Ramp = 2, two }, want: "cannot combine"},
		{name: "unsorted points", id: "v", edit: ramp(point(10, 1, EaseLinear), point(5, 2, EaseLinear)), want: "strictly increasing"},
		{name: "a point beyond the timeline limit", id: "v", edit: ramp(point(0, 1, EaseLinear), point(maxFrames+1, 2, EaseLinear)), want: "out of range"},
		{name: "a speed below 0.25", id: "v", edit: ramp(point(0, 0.2, EaseLinear), point(30, 2, EaseLinear)), want: "ramp speed"},
		{name: "a speed above 16", id: "v", edit: ramp(point(0, 1, EaseLinear), point(30, 16.5, EaseLinear)), want: "ramp speed"},
		{name: "an unknown ramp easing", id: "v", edit: ramp(point(0, 1, ""), point(30, 2, EaseLinear)), want: "unsupported ramp easing"},
	})
}

// The source extent rule follows the ramp (index §4.18): offset + S(duration - 1) <= duration_frames - 1 (+1e-9).
func TestValidateRampExtent(t *testing.T) {
	up := []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 3, E: EaseLinear}}
	slow := []RampPoint{{F: 0, Speed: 0.5, E: EaseLinear}, {F: 10, Speed: 0.5, E: EaseLinear}}
	runAnimationCases(t, []animationCase{
		{name: "a ramp that ends on the last source frame", id: "v", edit: func(clip *Clip) { clip.Offset, clip.Ramp = 142, up }},
		{name: "a ramp one frame past the media", id: "v", edit: func(clip *Clip) { clip.Offset, clip.Ramp = 143, up }, want: "exceeds asset"},
		{name: "a slow ramp holds a clip longer than the media at 1x", id: "v", edit: func(clip *Clip) { clip.Offset, clip.Duration, clip.Ramp = 0, 400, slow }},
		{name: "the same clip without the ramp", id: "v", edit: func(clip *Clip) { clip.Offset, clip.Duration = 0, 400 }, want: "exceeds asset"},
	})
	if got := clipSourceExtent(Clip{Duration: 60, Ramp: up}); math.Abs(got-157) > 1e-9 {
		t.Fatalf("clipSourceExtent = %v, want S(59) = 40 + 39*3 = 157", got)
	}
}
