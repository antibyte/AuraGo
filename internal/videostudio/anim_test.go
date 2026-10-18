package videostudio

import (
	"fmt"
	"math"
	"testing"
)

// animProject is a 25 fps project for the 2e animation tests (Tasks 4–9) on a width × height canvas: one track of the
// given kind holding clips. assets nil gives the image "logo" (1920×1080) and the video "video" (300 frames,
// 1920×1080); overlay tracks take image assets only.
func animProject(width, height int, kind TrackKind, assets []Asset, clips ...Clip) Project {
	if assets == nil {
		assets = []Asset{
			{ID: "logo", Name: "logo.png", Kind: AssetImage, Width: 1920, Height: 1080},
			{ID: "video", Name: "video.mp4", Kind: AssetVideo, DurationFrames: 300, Width: 1920, Height: 1080},
		}
	}
	return Project{Version: CurrentVersion, Name: "Anim", Width: width, Height: height, FPS: 25, Assets: assets,
		Tracks: []Track{{ID: "anim", Name: "Anim", Kind: kind, Clips: clips}}}
}

// animClip is a clip of animProject's "logo" in the centre quarter box with the given animations.
func animClip(id string, start, duration int, in, out *ClipAnim) Clip {
	return Clip{ID: id, AssetID: "logo", Start: start, Duration: duration, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5,
		Opacity: 1, Volume: 1, Fit: FitContain, AnimIn: in, AnimOut: out}
}

// TestAnimStateAt pins animAt (index §4.24) for every preset, in and out, at p = 0, 0.5, (D − 1)/D and at rest. The
// clip has spans of 12 frames on 40 frames at frame 10, so the in span is j = 0…11 (p = j/12) and the out span
// j = 28…39 (p = (39 − j)/12).
//   - The first frame (in) and the last (out) are fully hidden.
//   - Slides move against their direction on the way in and along it on the way out.
//   - The pop peaks at 1.08 at p = 0.7 and settles by smoothstep.
//   - clipFrameAt and clipOpacityAt multiply the keyed opacity; the keyed geometry stays.
func TestAnimStateAt(t *testing.T) {
	near := func(got, want float64, label string) {
		t.Helper()
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s: %v, want %v", label, got, want)
		}
	}
	shift := func(p float64) float64 { return 0.15 * math.Pow(1-p, 3) } // (1 − e_out(p))·0.15
	pop := func(p float64) float64 {
		if p < 0.7 {
			return 0.6 + 0.48*(1-math.Pow(1-p/0.7, 3))
		}
		u := (p - 0.7) / 0.3
		return 1.08 - 0.08*u*u*(3-2*u)
	}
	expect := func(preset string, p float64, side int) animState {
		a := animState{Opacity: 1, Scale: 1, M1: 255, M2: 255, P: p}
		s := float64(side)
		switch preset {
		case "fade":
			a.Opacity = p
		case "slide_up": // up is −y: in starts below the rest position, out ends above it
			a.DY, a.Opacity = s*shift(p), p
		case "slide_down":
			a.DY, a.Opacity = -s*shift(p), p
		case "slide_left": // left is −x: in starts right of the rest position, out ends left of it
			a.DX, a.Opacity = s*shift(p), p
		case "slide_right":
			a.DX, a.Opacity = -s*shift(p), p
		case "pop":
			a.Scale, a.Opacity = pop(p), math.Min(1, p/0.3)
		case "wipe":
			a.Wipe = side
		case "blur_in":
			a.Blur, a.Opacity = true, p
			a.M1, a.M2 = int(math.Round(255*math.Min(p/0.75, 1))), int(math.Round(255*math.Max((p-0.75)/0.25, 0)))
		}
		return a
	}
	frames := []struct {
		frame int
		p     float64
		side  int // 1 in, −1 out, 0 at rest
	}{{9, 1, 0}, {10, 0, 1}, {16, 0.5, 1}, {21, 11.0 / 12, 1}, {30, 1, 0}, {38, 11.0 / 12, -1}, {43, 0.5, -1}, {49, 0, -1}, {50, 1, 0}}
	for _, preset := range []string{"fade", "slide_up", "slide_down", "slide_left", "slide_right", "pop", "wipe", "blur_in"} {
		clip := animClip("c", 10, 40, &ClipAnim{Preset: preset, Duration: 12}, &ClipAnim{Preset: preset, Duration: 12})
		if err := Validate(animProject(1280, 720, TrackOverlay, nil, clip)); err != nil {
			t.Fatalf("%s: the test clip is invalid: %v", preset, err)
		}
		for _, f := range frames {
			got := animAt(clip, f.frame)
			if f.side == 0 {
				if got != animRest {
					t.Errorf("%s frame %d: %+v, want the rest state", preset, f.frame, got)
				}
				continue
			}
			want, label := expect(preset, f.p, f.side), fmt.Sprintf("%s frame %d", preset, f.frame)
			near(got.Opacity, want.Opacity, label+" opacity")
			near(got.DX, want.DX, label+" dx")
			near(got.DY, want.DY, label+" dy")
			near(got.Scale, want.Scale, label+" scale")
			near(got.P, want.P, label+" p")
			if got.Blur != want.Blur || got.M1 != want.M1 || got.M2 != want.M2 || got.Wipe != want.Wipe {
				t.Errorf("%s: blur %v (%d, %d), wipe %d; want blur %v (%d, %d), wipe %d", label, got.Blur, got.M1, got.M2, got.Wipe, want.Blur, want.M1, want.M2, want.Wipe)
			}
		}
		// The first frame (in) and the last (out) are fully hidden: no opacity, or no visible column.
		for _, frame := range []int{10, 49} {
			a := animAt(clip, frame)
			if x0, x1 := animRevealPx(a, 640); a.Opacity != 0 && x0 != x1 {
				t.Errorf("%s frame %d is visible: opacity %v, columns [%d, %d)", preset, frame, a.Opacity, x0, x1)
			}
		}
	}
	// Slide signs at p = 0: slide_up arrives from 15 % below and leaves to 15 % above.
	up := animClip("u", 0, 40, &ClipAnim{Preset: "slide_up", Duration: 10}, &ClipAnim{Preset: "slide_up", Duration: 10})
	if in, out := animAt(up, 0), animAt(up, 39); in.DY != 0.15 || in.DX != 0 || out.DY != -0.15 || out.DX != 0 {
		t.Errorf("slide_up offsets: in (%v, %v), out (%v, %v); want (0, 0.15) and (0, -0.15)", in.DX, in.DY, out.DX, out.DY)
	}
	// The pop curve: 0.6 at p = 0, the 1.08 peak at p = 0.7, 1.04 halfway down the smoothstep tail, 1 at rest; rising
	// before the peak, falling after it, without a jump.
	if got := animPopScale(0); got != 0.6 {
		t.Errorf("pop at p = 0: %v, want 0.6", got)
	}
	if got := animPopScale(0.7); got != 1.08 {
		t.Errorf("pop peak: %v, want 1.08 at p = 0.7", got)
	}
	near(animPopScale(0.85), 1.04, "pop halfway down the tail")
	near(animPopScale(1), 1, "pop at p = 1")
	if math.Abs(animPopScale(0.7-1e-9)-1.08) > 1e-6 {
		t.Errorf("the pop curve jumps at its peak: %v", animPopScale(0.7-1e-9))
	}
	for p := 0.0; p < 0.69; p += 0.01 {
		if animPopScale(p+0.01) <= animPopScale(p) {
			t.Fatalf("the pop does not rise at p = %v", p)
		}
	}
	for p := 0.7; p < 0.99; p += 0.01 {
		if animPopScale(p+0.01) > animPopScale(p) {
			t.Fatalf("the pop does not settle at p = %v", p)
		}
	}
	// The written-out spans are Go's exact constant differences (JS uses the same literals).
	if rise, settle, tail := animPopPeak-animPopFrom, animPopPeak-1.0, 1-animPopPeakAt; rise != animPopRise || settle != animPopSettle || tail != animPopTail {
		t.Errorf("pop spans %v, %v, %v; want %v, %v, %v", animPopRise, animPopSettle, animPopTail, rise, settle, tail)
	}
	// clipFrameAt and clipOpacityAt: the keyed opacity times the animation's; the geometry stays keyed.
	keyed := animClip("k", 0, 40, &ClipAnim{Preset: "fade", Duration: 10}, nil)
	keyed.Opacity = 0.5
	if got := clipFrameAt(keyed, 5); got.Opacity != 0.25 || got.X != 0.25 || got.Width != 0.5 {
		t.Errorf("clipFrameAt in the fade = %+v, want opacity 0.25 and the keyed box", got)
	}
	if got := clipOpacityAt(keyed, 20); got != 0.5 {
		t.Errorf("clipOpacityAt at rest = %v, want the keyed 0.5", got)
	}
	keyed.Keyframes = map[string][]Keyframe{"opacity": {{F: 0, V: 0.2, E: EaseLinear}, {F: 10, V: 1, E: EaseLinear}}}
	near(clipOpacityAt(keyed, 5), 0.3, "keyed 0.6 times fade 0.5")
}

// TestAnimBlurMasks pins the blur_in cascade's masks: m1 = round(255·min(u/0.75, 1)) and
// m2 = round(255·max((u − 0.75)/0.25, 0)). The sequence is the spike's (S5, 25 fps, u = n/12.5): m1 rises to 245, then
// m2 takes over at 51, 133, 214 and 255. At rest both are 255 (sharp).
func TestAnimBlurMasks(t *testing.T) {
	want := [][2]int{{0, 0}, {27, 0}, {54, 0}, {82, 0}, {109, 0}, {136, 0}, {163, 0}, {190, 0}, {218, 0}, {245, 0}, {255, 51}, {255, 133}, {255, 214}, {255, 255}, {255, 255}}
	for n, w := range want {
		if m1, m2 := animBlurMasks(min(float64(n)/12.5, 1)); m1 != w[0] || m2 != w[1] {
			t.Errorf("frame %d: masks (%d, %d), want (%d, %d)", n, m1, m2, w[0], w[1])
		}
	}
	for _, c := range []struct {
		p      float64
		m1, m2 int
	}{{-1, 0, 0}, {0.75, 255, 0}, {1, 255, 255}, {2, 255, 255}} {
		if m1, m2 := animBlurMasks(c.p); m1 != c.m1 || m2 != c.m2 {
			t.Errorf("p = %v: masks (%d, %d), want (%d, %d)", c.p, m1, m2, c.m1, c.m2)
		}
	}
	clip := animClip("b", 0, 40, &ClipAnim{Preset: "blur_in", Duration: 12}, nil)
	if a := animAt(clip, 6); !a.Blur || a.M1 != 170 || a.M2 != 0 || a.Opacity != 0.5 {
		t.Errorf("blur_in at p = 0.5: %+v, want blur with masks (170, 0) at opacity 0.5", a)
	}
	if a := animAt(clip, 20); a.Blur || a.M1 != 255 || a.M2 != 255 {
		t.Errorf("blur_in at rest: %+v, want no blur and masks (255, 255)", a)
	}
}

// TestAnimSpansClamp pins the effective spans (index §4.24): the stored duration, at most floor(duration/2) on each
// side, so the spans never overlap. A side whose span is 0 counts for nothing in animKindsOf.
func TestAnimSpansClamp(t *testing.T) {
	anim := func(preset string, frames int) *ClipAnim { return &ClipAnim{Preset: preset, Duration: frames} }
	for _, c := range []struct {
		name    string
		clip    Clip
		in, out int
		kinds   animKinds
	}{
		{"in 30 on 40 frames", Clip{Duration: 40, AnimIn: anim("fade", 30)}, 20, 0, animKinds{opacity: true}},
		{"both 30 on 41 frames", Clip{Duration: 41, AnimIn: anim("pop", 30), AnimOut: anim("wipe", 30)}, 20, 20, animKinds{geometry: true, opacity: true, wipe: true}},
		{"a one-frame clip", Clip{Duration: 1, AnimIn: anim("fade", 5), AnimOut: anim("slide_up", 5)}, 0, 0, animKinds{}},
		{"no animation", Clip{Duration: 40}, 0, 0, animKinds{}},
		{"13 on 25 frames", Clip{Duration: 25, AnimIn: anim("blur_in", 13), AnimOut: anim("slide_up", 5)}, 12, 5, animKinds{geometry: true, opacity: true, blur: true}},
		{"a slide alone", Clip{Duration: 40, AnimOut: anim("slide_right", 10)}, 0, 10, animKinds{geometry: true, opacity: true}},
	} {
		if in, out := animSpans(c.clip); in != c.in || out != c.out {
			t.Errorf("%s: spans (%d, %d), want (%d, %d)", c.name, in, out, c.in, c.out)
		}
		if got := animKindsOf(c.clip); got != c.kinds {
			t.Errorf("%s: kinds %+v, want %+v", c.name, got, c.kinds)
		}
	}
	if a := animAt(Clip{Duration: 1, AnimIn: anim("fade", 5)}, 0); a != animRest {
		t.Errorf("a clip without an effective span is at rest: %+v", a)
	}
}

// TestAnimBox pins the exact box of index §4.24 and the wipe columns.
//   - W' = max(2, 2·round(BW·s/2)) and H' likewise, about the box centre, with no canvas cap.
//   - The centre moves by 2·round(d·canvas/2), halves away from zero. The box may leave the canvas.
//   - A wipe in shows [0, R), a wipe out [W' − R, W'), with R = round(p·W').
func TestAnimBox(t *testing.T) {
	for _, c := range []struct {
		name           string
		x, y, w, h     int
		a              animState
		cw, ch         int
		cx, cy, bw, bh int
	}{
		{"rest", 100, 50, 400, 226, animRest, 1280, 720, 300, 163, 400, 226},
		{"the pop peak", 320, 180, 640, 360, animState{Scale: 1.08}, 1280, 720, 640, 360, 692, 388},
		{"scale and offset", 100, 50, 400, 226, animState{DX: 0.15, DY: -0.0723, Scale: 1.08}, 1280, 720, 492, 111, 432, 244},
		{"half a pixel right", 100, 50, 400, 226, animState{DX: 1.0 / 256, Scale: 1}, 1280, 720, 306, 163, 400, 226},
		{"half a pixel left", 100, 50, 400, 226, animState{DX: -1.0 / 256, Scale: 1}, 1280, 720, 294, 163, 400, 226},
		{"the smallest box", 0, 0, 2, 2, animState{Scale: 0.6}, 1280, 720, 1, 1, 2, 2},
		{"a small box", 0, 0, 10, 10, animState{Scale: 0.6}, 1280, 720, 5, 5, 6, 6},
		{"off the canvas", 0, 216, 512, 288, animState{DX: -0.15, Scale: 1}, 1280, 720, 64, 360, 512, 288},
		{"past the canvas", 0, 0, 1920, 1080, animState{Scale: 1.08}, 1920, 1080, 960, 540, 2074, 1166},
	} {
		if cx, cy, bw, bh := animBox(c.x, c.y, c.w, c.h, c.a, c.cw, c.ch); cx != c.cx || cy != c.cy || bw != c.bw || bh != c.bh {
			t.Errorf("%s: centre (%d, %d) size %dx%d, want (%d, %d) %dx%d", c.name, cx, cy, bw, bh, c.cx, c.cy, c.bw, c.bh)
		}
	}
	for _, c := range []struct {
		name   string
		a      animState
		width  int
		x0, x1 int
	}{
		{"in, half", animState{Wipe: 1, P: 0.5}, 432, 0, 216},
		{"out, half", animState{Wipe: -1, P: 0.5}, 432, 216, 432},
		{"in, a third", animState{Wipe: 1, P: 1.0 / 3}, 10, 0, 3},
		{"out, a third", animState{Wipe: -1, P: 1.0 / 3}, 10, 7, 10},
		{"in, a half column rounds up", animState{Wipe: 1, P: 0.25}, 10, 0, 3},
		{"in, the first frame", animState{Wipe: 1, P: 0}, 640, 0, 0},
		{"at rest", animRest, 640, 0, 640},
	} {
		if x0, x1 := animRevealPx(c.a, c.width); x0 != c.x0 || x1 != c.x1 {
			t.Errorf("%s: columns [%d, %d), want [%d, %d)", c.name, x0, x1, c.x0, c.x1)
		}
	}
}
