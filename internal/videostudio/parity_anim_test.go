package videostudio

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Shared in/out animation fixture (plan index §4.13, §4.24): topic anim.
//   - Go computes every want with anim.go (animAt, animBox, animRevealPx, AnimFrameRange, animSpans).
//   - TestVideoStudioEffectsParity (ui) runs the same file through window.VideoStudioAnim.
//   - AURAGO_UPDATE_PARITY=1 regenerates it from Go first (runParityTopic).

// animParityClip is the part of a clip the animation reads.
type animParityClip struct {
	Start    int       `json:"start"`
	Duration int       `json:"duration"`
	AnimIn   *ClipAnim `json:"anim_in,omitempty"`
	AnimOut  *ClipAnim `json:"anim_out,omitempty"`
}

type animParityBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type animParityCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type animParityWipe struct {
	Side string  `json:"side"` // in or out
	P    float64 `json:"p"`
}

type animParityBlur struct {
	M1 int `json:"m1"`
	M2 int `json:"m2"`
}

type animParityMotion struct {
	DX    float64 `json:"dx"`
	DY    float64 `json:"dy"`
	Scale float64 `json:"scale"`
}

// The fixture inputs, one shape per kind.
type animStateInput struct {
	Kind  string         `json:"kind"` // state
	Clip  animParityClip `json:"clip"`
	FPS   int            `json:"fps"`
	NTSC  bool           `json:"ntsc"`
	Frame int            `json:"frame"`
}

type animBoxInput struct {
	Kind   string           `json:"kind"` // box
	Box    animParityBox    `json:"box"`
	Anim   animParityMotion `json:"anim"`
	Canvas animParityCanvas `json:"canvas"`
}

type animRevealInput struct {
	Kind string `json:"kind"` // reveal
	Anim struct {
		Wipe *animParityWipe `json:"wipe"`
	} `json:"anim"`
	Width int `json:"width"`
}

type animRangeInput struct {
	Kind string `json:"kind"` // range
	FPS  int    `json:"fps"`
	NTSC bool   `json:"ntsc"`
}

type animSpansInput struct {
	Kind string         `json:"kind"` // spans
	Clip animParityClip `json:"clip"`
	FPS  int            `json:"fps"`
}

// animParityInput decodes every kind; the fields of the other kinds stay zero.
type animParityInput struct {
	Kind  string         `json:"kind"`
	Clip  animParityClip `json:"clip"`
	FPS   int            `json:"fps"`
	NTSC  bool           `json:"ntsc"`
	Frame int            `json:"frame"`
	Box   animParityBox  `json:"box"`
	Anim  struct {
		DX    float64         `json:"dx"`
		DY    float64         `json:"dy"`
		Scale float64         `json:"scale"`
		Wipe  *animParityWipe `json:"wipe"`
	} `json:"anim"`
	Canvas animParityCanvas `json:"canvas"`
	Width  int              `json:"width"`
}

// animStateWant is a state as both sides report it: at rest active is false with the rest values.
type animStateWant struct {
	Active  bool            `json:"active"`
	Opacity float64         `json:"opacity"`
	DX      float64         `json:"dx"`
	DY      float64         `json:"dy"`
	Scale   float64         `json:"scale"`
	Blur    *animParityBlur `json:"blur"`
	Wipe    *animParityWipe `json:"wipe"`
}

func animParitySamples() []paritySample {
	var samples []paritySample
	// Every preset on both sides: spans of 12 on a 40-frame clip at frame 10 (in p = 0, 0.5, 11/12 at frames 10, 16,
	// 21; out p = 11/12, 0.5, 0 at frames 38, 43, 49; rest at 9, 30 and 50).
	for _, preset := range []string{"fade", "slide_up", "slide_down", "slide_left", "slide_right", "pop", "wipe", "blur_in"} {
		clip := animParityClip{Start: 10, Duration: 40, AnimIn: &ClipAnim{Preset: preset, Duration: 12}, AnimOut: &ClipAnim{Preset: preset, Duration: 12}}
		for _, frame := range []int{9, 10, 16, 21, 30, 38, 43, 49, 50} {
			samples = append(samples, paritySample{fmt.Sprintf("state %s frame %d", preset, frame), animStateInput{Kind: "state", Clip: clip, FPS: 25, Frame: frame}})
		}
	}
	// A 9-frame clip clamps both spans to 4 frames; the rate changes nothing (timeline frames).
	short := animParityClip{Duration: 9, AnimIn: &ClipAnim{Preset: "pop", Duration: 13}, AnimOut: &ClipAnim{Preset: "slide_left", Duration: 30}}
	for _, frame := range []int{0, 2, 3, 4, 5, 8} {
		samples = append(samples, paritySample{fmt.Sprintf("state clamped frame %d", frame), animStateInput{Kind: "state", Clip: short, FPS: 30, NTSC: true, Frame: frame}})
	}
	// D = 10: p = 0.7 on frame 7, the 1.08 peak.
	samples = append(samples, paritySample{"state pop peak", animStateInput{Kind: "state", Clip: animParityClip{Duration: 30, AnimIn: &ClipAnim{Preset: "pop", Duration: 10}}, FPS: 25, Frame: 7}})
	// Exact boxes. Every rest box is an even clipBox inside its canvas (Task 5 rebuilds clip states from them).
	box := func(name string, b animParityBox, m animParityMotion, width, height int) paritySample {
		return paritySample{"box " + name, animBoxInput{Kind: "box", Box: b, Anim: m, Canvas: animParityCanvas{Width: width, Height: height}}}
	}
	samples = append(samples,
		box("pop peak", animParityBox{320, 180, 640, 360}, animParityMotion{Scale: 1.08}, 1280, 720),
		box("pop start", animParityBox{320, 180, 640, 360}, animParityMotion{Scale: 0.6}, 1280, 720),
		box("slide left 15 %", animParityBox{320, 180, 640, 360}, animParityMotion{DX: 0.15, Scale: 1}, 1280, 720),
		box("slide up off the top", animParityBox{100, 0, 400, 226}, animParityMotion{DY: -0.15, Scale: 1}, 1280, 720),
		box("half pixels round away from zero", animParityBox{100, 50, 400, 226}, animParityMotion{DX: 1.0 / 256, DY: -1.0 / 256, Scale: 1}, 1280, 720),
		box("scale and offset", animParityBox{100, 50, 400, 226}, animParityMotion{DX: 0.15, DY: -0.0723, Scale: 1.08}, 1280, 720),
		box("smallest box", animParityBox{0, 0, 2, 2}, animParityMotion{Scale: 0.6}, 1280, 720),
		box("off the canvas left", animParityBox{0, 216, 512, 288}, animParityMotion{DX: -0.15, Scale: 1}, 1280, 720),
		box("full canvas pop beyond the canvas", animParityBox{0, 0, 1920, 1080}, animParityMotion{Scale: 1.08}, 1920, 1080),
		box("portrait canvas", animParityBox{108, 384, 864, 1152}, animParityMotion{DY: 0.15, Scale: 0.8}, 1080, 1920),
	)
	reveal := func(name string, wipe *animParityWipe, width int) paritySample {
		input := animRevealInput{Kind: "reveal", Width: width}
		input.Anim.Wipe = wipe
		return paritySample{"reveal " + name, input}
	}
	samples = append(samples,
		reveal("in half", &animParityWipe{Side: "in", P: 0.5}, 432),
		reveal("out half", &animParityWipe{Side: "out", P: 0.5}, 432),
		reveal("in a third", &animParityWipe{Side: "in", P: 1.0 / 3}, 10),
		reveal("out a third", &animParityWipe{Side: "out", P: 1.0 / 3}, 10),
		reveal("in a half column rounds up", &animParityWipe{Side: "in", P: 0.25}, 10),
		reveal("in first frame", &animParityWipe{Side: "in", P: 0}, 640),
		reveal("out last frame", &animParityWipe{Side: "out", P: 0}, 640),
		reveal("at rest", nil, 640),
	)
	for _, rate := range []struct {
		fps  int
		ntsc bool
	}{{24, false}, {24, true}, {25, false}, {30, false}, {30, true}, {50, false}, {60, false}, {60, true}} {
		name := fmt.Sprintf("range %d fps", rate.fps)
		if rate.ntsc {
			name += " ntsc"
		}
		samples = append(samples, paritySample{name, animRangeInput{Kind: "range", FPS: rate.fps, NTSC: rate.ntsc}})
	}
	spans := func(name string, clip animParityClip) paritySample {
		return paritySample{"spans " + name, animSpansInput{Kind: "spans", Clip: clip, FPS: 25}}
	}
	return append(samples,
		spans("in 30 on 40 frames", animParityClip{Duration: 40, AnimIn: &ClipAnim{Preset: "fade", Duration: 30}}),
		spans("both 30 on 41 frames", animParityClip{Duration: 41, AnimIn: &ClipAnim{Preset: "pop", Duration: 30}, AnimOut: &ClipAnim{Preset: "wipe", Duration: 30}}),
		spans("one frame", animParityClip{Duration: 1, AnimIn: &ClipAnim{Preset: "fade", Duration: 5}, AnimOut: &ClipAnim{Preset: "fade", Duration: 5}}),
		spans("none", animParityClip{Duration: 40}),
		spans("13 on 25 frames", animParityClip{Duration: 25, AnimIn: &ClipAnim{Preset: "blur_in", Duration: 13}, AnimOut: &ClipAnim{Preset: "slide_up", Duration: 5}}),
	)
}

func animParityResult(raw json.RawMessage) (any, error) {
	var input animParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	clip := Clip{Start: input.Clip.Start, Duration: input.Clip.Duration, AnimIn: input.Clip.AnimIn, AnimOut: input.Clip.AnimOut}
	switch input.Kind {
	case "state":
		a := animAt(clip, input.Frame)
		want := animStateWant{Active: a.P < 1, Opacity: a.Opacity, DX: a.DX, DY: a.DY, Scale: a.Scale}
		if a.Blur {
			want.Blur = &animParityBlur{M1: a.M1, M2: a.M2}
		}
		switch a.Wipe {
		case 1:
			want.Wipe = &animParityWipe{Side: "in", P: a.P}
		case -1:
			want.Wipe = &animParityWipe{Side: "out", P: a.P}
		}
		return want, nil
	case "box":
		a := animState{DX: input.Anim.DX, DY: input.Anim.DY, Scale: input.Anim.Scale}
		cx, cy, w, h := animBox(input.Box.X, input.Box.Y, input.Box.W, input.Box.H, a, input.Canvas.Width, input.Canvas.Height)
		return map[string]int{"x": cx - w/2, "y": cy - h/2, "w": w, "h": h}, nil
	case "reveal":
		a := animRest
		if wipe := input.Anim.Wipe; wipe != nil {
			a.P, a.Wipe = wipe.P, 1
			if wipe.Side == "out" {
				a.Wipe = -1
			}
		}
		x0, x1 := animRevealPx(a, input.Width)
		return map[string]int{"x0": x0, "x1": x1}, nil
	case "range":
		lo, hi, def := AnimFrameRange(ProjectRate(Project{FPS: input.FPS, NTSC: input.NTSC}))
		return map[string]int{"min": lo, "max": hi, "default": def}, nil
	case "spans":
		in, out := animSpans(clip)
		return map[string]int{"in": in, "out": out}, nil
	}
	return nil, fmt.Errorf("unknown anim input kind %q", input.Kind)
}

// TestParityAnim runs the animation fixture. JS: VideoStudioAnim.at, box, revealPx, frameRange and spans.
func TestParityAnim(t *testing.T) {
	runParityTopic(t, "anim", animParitySamples(), animParityResult)
}
