package videostudio

import (
	"math"
	"regexp"
	"testing"
)

// A mask expression holds only Go-written numbers, the four operators, parentheses, ';' and ',', and these words: the
// functions of law L and of the keyframe curves, and geq's X, Y and T (index §3, review focus on graph strings).
var (
	maskExprText    = regexp.MustCompile(`^[0-9A-Za-z.()*/+;,-]+$`)
	maskExprWords   = regexp.MustCompile(`[A-Za-z]+`)
	maskExprAllowed = map[string]bool{"st": true, "ld": true, "abs": true, "hypot": true, "max": true, "min": true, "clip": true, "sqrt": true,
		"cos": true, "sin": true, "if": true, "lt": true, "round": true, "X": true, "Y": true, "T": true}
)

// checkMaskRegisters fails when a st or ld of e names anything but a literal register 0..9. FFmpeg clamps others, which
// would read the wrong register.
func checkMaskRegisters(t *testing.T, name string, e ffExpr) {
	t.Helper()
	if e.op == "st" || e.op == "ld" {
		if len(e.args) == 0 || e.args[0].op != "num" || e.args[0].num != math.Trunc(e.args[0].num) || e.args[0].num < 0 || e.args[0].num > 9 {
			t.Fatalf("%s: register outside 0–9 in %s", name, e.String())
		}
	}
	for _, arg := range e.args {
		checkMaskRegisters(t, name, arg)
	}
}

// TestMaskExprMatchesMaskAt evaluates the geq mask expression with FFmpeg's arithmetic (ffExpr.eval) on a grid of
// sample pixels, at the window's first, sixth and last frame, against Task 3's maskAt. It requires agreement within
// 1e-9, with T = k·Den/Num as geq computes it and the pixel at its box position.
//   - Cases: static rect, ellipse (rotated and inverted), a mask reaching past the box, all six mask paths keyed, keyed
//     box geometry (pad canvas, box centred), a ramped clip, cover with crop keys.
//   - Every expression stays inside registers 0–9 and the vocabulary above.
func TestMaskExprMatchesMaskAt(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(clip Clip, at int) clipChain {
		local := clip
		local.Start, local.Duration = 0, clip.Start+clip.Duration-at
		item := layerClip{clip: local, whole: clip, at: at, pictureWidth: 1280, pictureHeight: 720, sourceAspect: 16.0 / 9}
		if len(clip.Ramp) > 0 {
			item.pieces = rampPieces(clip.Ramp, 0, float64(clip.Duration))
		}
		return clipChain{item: item, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	base := func(edit func(*Clip)) Clip {
		c := Clip{ID: "m", AssetID: "a", Start: 4, Duration: 30, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.6, Opacity: 1, Volume: 1, Fit: FitContain,
			Mask: &Mask{Shape: MaskEllipse, X: 0.2, Y: 0.15, W: 0.5, H: 0.6, Feather: 0.03}}
		edit(&c)
		return c
	}
	keys := func(from, to float64, e Easing) []Keyframe {
		return []Keyframe{{F: 0, V: from, E: e}, {F: 12, V: (from + to) / 2, E: EaseLinear}, {F: 26, V: to, E: EaseLinear}}
	}
	for _, tc := range []struct {
		name string
		clip Clip
		at   int
	}{
		{"static rect", base(func(c *Clip) { c.Mask.Shape = MaskRect }), 4},
		{"static ellipse rotated and inverted", base(func(c *Clip) { c.Mask.Rotation, c.Mask.Invert, c.Mask.Feather = 33, true, 0.08 }), 4},
		{"static rect at 1 px feather, reaching past the box", base(func(c *Clip) { c.Mask.Shape, c.Mask.X, c.Mask.W, c.Mask.Feather = MaskRect, -0.3, 0.9, 0 }), 4},
		{"every mask path keyed", base(func(c *Clip) {
			c.Keyframes = map[string][]Keyframe{"mask.x": keys(0.1, 0.3, EaseInOut), "mask.y": keys(0.2, 0.05, EaseOut), "mask.w": keys(0.3, 0.6, EaseIn),
				"mask.h": keys(0.5, 0.2, EaseLinear), "mask.rotation": keys(-20, 45, EaseInOut), "mask.feather": keys(0, 0.1, EaseHold)}
		}), 9},
		{"keyed box geometry", base(func(c *Clip) {
			c.Mask.Shape, c.Mask.Rotation = MaskRect, 12
			c.Keyframes = map[string][]Keyframe{"x": keys(0.05, 0.3, EaseLinear), "width": keys(0.3, 0.6, EaseOut), "height": keys(0.4, 0.5, EaseIn), "rotation": keys(0, 30, EaseLinear)}
		}), 4},
		{"ramped clip with a keyed mask and box", base(func(c *Clip) {
			c.Mask.Invert = true
			c.Ramp = []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 2, E: EaseLinear}}
			c.Keyframes = map[string][]Keyframe{"mask.x": keys(0.1, 0.4, EaseLinear), "mask.feather": keys(0.01, 0.2, EaseOut), "y": keys(0.1, 0.3, EaseLinear)}
		}), 10},
		{"cover with crop keys and a keyed mask", base(func(c *Clip) {
			c.Fit = FitCover
			c.Keyframes = map[string][]Keyframe{"crop.l": keys(0, 0.3, EaseLinear), "mask.w": keys(0.2, 0.8, EaseLinear)}
		}), 4},
	} {
		c := chain(tc.clip, tc.at)
		plan := maskPlanOf(c)
		expr := maskExpr(c, plan)
		text := expr.String()
		if !maskExprText.MatchString(text) {
			t.Fatalf("%s: characters outside the mask vocabulary: %s", tc.name, text)
		}
		for _, word := range maskExprWords.FindAllString(text, -1) {
			if !maskExprAllowed[word] {
				t.Fatalf("%s: word %q in the mask expression", tc.name, word)
			}
		}
		checkMaskRegisters(t, tc.name, expr)
		geometry, _ := clipAnimation(tc.clip)
		if plan.perFrame != (geometry || maskAnimated(tc.clip)) {
			t.Fatalf("%s: perFrame %v", tc.name, plan.perFrame)
		}
		var g geometryPlan
		if geometry {
			g = c.geometry()
			if plan.canvasWidth != g.padWidth || plan.canvasHeight != g.padHeight {
				t.Fatalf("%s: canvas %dx%d, want the pad canvas %dx%d", tc.name, plan.canvasWidth, plan.canvasHeight, g.padWidth, g.padHeight)
			}
		}
		scaleX, scaleY := float64(plan.canvasWidth)/float64(plan.gridWidth), float64(plan.canvasHeight)/float64(plan.gridHeight)
		stepX, stepY := max(1, plan.gridWidth/41), max(1, plan.gridHeight/23)
		for _, k := range []int{0, 5, c.item.clip.Duration - 1} {
			frame := tc.at + k
			bw, bh := plan.canvasWidth, plan.canvasHeight
			if geometry {
				bw, bh = g.widths[k], g.heights[k]
			}
			mf, ok := maskFrameAt(tc.clip, frame, bw, bh)
			if !ok {
				t.Fatalf("%s: maskFrameAt found no mask", tc.name)
			}
			ox, oy := float64(plan.canvasWidth-bw)/2, float64(plan.canvasHeight-bh)/2
			vars := map[string]float64{"T": float64(k) * (float64(pal.Den) / float64(pal.Num))}
			for Y := 0; Y < plan.gridHeight; Y += stepY {
				for X := 0; X < plan.gridWidth; X += stepX {
					vars["X"], vars["Y"] = float64(X), float64(Y)
					got := expr.eval(vars) / 255
					px, py := (float64(X)+0.5)*scaleX, (float64(Y)+0.5)*scaleY
					if want := maskAt(mf, px-ox, py-oy); math.Abs(got-want) > 1e-9 {
						t.Fatalf("%s frame %d at (%d, %d): expression %v, maskAt %v", tc.name, frame, X, Y, got, want)
					}
				}
			}
		}
	}
}
