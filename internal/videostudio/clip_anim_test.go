package videostudio

import "testing"

// The wipe's timeline expression (index §4.11, §4.24). It is 1 exactly on the absolute frames of the clip's effective spans
// whose preset is listed, for windows that start at the clip, inside the in span and inside the out span, at 25 and
// 30000/1001 fps. t is window-local, as FFmpeg computes it: frame index times av_q2d(time base).
func TestAnimSpanExprMatchesSpans(t *testing.T) {
	clip := Clip{ID: "s", AssetID: "a", Start: 30, Duration: 20, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		AnimIn: &ClipAnim{Preset: "wipe", Duration: 6}, AnimOut: &ClipAnim{Preset: "blur_in", Duration: 25}}
	// D_in = 6: frames [30, 36); D_out = min(25, 20/2) = 10: frames [40, 50).
	if in, out := animSpans(clip); in != 6 || out != 10 {
		t.Fatalf("animSpans = %d, %d; want 6, 10", in, out)
	}
	inSpan := func(f int) bool { return f >= 30 && f < 36 }
	outSpan := func(f int) bool { return f >= 40 && f < 50 }
	cases := []struct {
		kinds []string
		want  func(int) bool
	}{
		{[]string{"wipe"}, inSpan},
		{[]string{"blur_in"}, outSpan},
		{[]string{"blur_in", "wipe"}, func(f int) bool { return inSpan(f) || outSpan(f) }},
	}
	for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}} {
		step := float64(rate.Den) / float64(rate.Num)
		for _, at := range []int{30, 33, 41} {
			for _, c := range cases {
				expr := animSpanExpr(clip, at, rate, c.kinds...)
				for k := range clip.Start + clip.Duration - at {
					want := 0.0
					if c.want(at + k) {
						want = 1
					}
					if got := expr.eval(map[string]float64{"t": float64(k) * step}); got != want {
						t.Fatalf("rate %s, window at %d, kinds %v: frame %d gives %v, want %v (%s)", rate, at, c.kinds, at+k, got, want, expr.String())
					}
				}
			}
		}
	}
	pal := Rate{Num: 25, Den: 1}
	if got, want := animSpanExpr(clip, 30, pal, "wipe", "blur_in").String(), "(st(0,(30+round(((max(t,0)*25)/1))));max((gte(ld(0),30)*lt(ld(0),36)),(gte(ld(0),40)*lt(ld(0),50))))"; got != want {
		t.Fatalf("span expression:\n got %s\nwant %s", got, want)
	}
	short := clip
	short.Duration = 1 // floor(1/2) = 0: neither span plays
	for name, expr := range map[string]ffExpr{
		"a preset the clip does not use": animSpanExpr(clip, 30, pal, "fade"),
		"a one-frame clip":               animSpanExpr(short, 30, pal, "wipe", "blur_in"),
		"no animation":                   animSpanExpr(Clip{Start: 30, Duration: 20}, 30, pal, "wipe"),
	} {
		if expr.String() != "0" {
			t.Errorf("%s: %s, want the constant 0", name, expr.String())
		}
	}
}

// revealModel is the Go model of the reveal row on the fit canvas of plan at an absolute frame (index §4.24). It returns
// the canvas columns [from, to) the wipe shows, and whether a wipe span is active there (at rest every column shows).
//   - It reads the keyed box (clipFrameAt, clipBox), the animated box (animBox) and the box columns (animRevealPx).
//   - The box sits centred on the canvas.
func revealModel(c clipChain, plan maskPlan, frame int) (from, to int, active bool) {
	whole := c.item.whole
	a := animAt(whole, frame)
	if a.Wipe == 0 {
		return 0, plan.canvasWidth, false
	}
	f := clipFrameAt(whole, frame)
	x, y, w, h := clipBox(Clip{X: f.X, Y: f.Y, Width: f.Width, Height: f.Height}, c.canvasWidth, c.canvasHeight)
	_, _, boxWidth, _ := animBox(x, y, w, h, a, c.canvasWidth, c.canvasHeight)
	x0, x1 := animRevealPx(a, boxWidth)
	left := (plan.canvasWidth - boxWidth) / 2
	return left + x0, left + x1, true
}

// The reveal row's geq expression (index §4.24) equals revealModel on every column of every window frame:
//   - the static box at 25 and 30000/1001 fps;
//   - keyed geometry on the pad canvas;
//   - a wipe in with an out pop, which takes the pad canvas through Task 6's animation terms;
//   - a ramped clip with a keyed width, whose spans stay in timeline frames.
//
// Each case runs the whole clip and a window that starts inside the in span. It evaluates like geq: the pixels of a row
// from X = 0 with registers kept from pixel to pixel and frame to frame, so the row cache is exercised.
func TestRevealExprMatchesAnimReveal(t *testing.T) {
	pal, ntsc := Rate{Num: 25, Den: 1}, Rate{Num: 30000, Den: 1001}
	wipe := func(frames int) *ClipAnim { return &ClipAnim{Preset: "wipe", Duration: frames} }
	base := func(edit func(*Clip)) Clip {
		c := Clip{ID: "w", AssetID: "a", Start: 12, Duration: 24, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
			AnimIn: wipe(7), AnimOut: wipe(6)}
		edit(&c)
		return c
	}
	cases := []struct {
		name string
		clip Clip
		rate Rate
		pad  bool // the fit canvas is the pad canvas of animated geometry
	}{
		{"static box", base(func(*Clip) {}), pal, false},
		{"static box, NTSC", base(func(*Clip) {}), ntsc, false},
		{"keyed box (pad canvas)", base(func(c *Clip) {
			c.Keyframes = map[string][]Keyframe{"x": linearKeys(0.1, 0.3, 23), "width": linearKeys(0.3, 0.55, 23), "height": linearKeys(0.3, 0.5, 23)}
		}), ntsc, true},
		{"wipe in, pop out (pad canvas)", base(func(c *Clip) { c.AnimOut = &ClipAnim{Preset: "pop", Duration: 8} }), pal, true},
		{"ramped, keyed width", base(func(c *Clip) {
			c.Ramp = []RampPoint{{F: 0, Speed: 1, E: EaseInOut}, {F: 16, Speed: 2.5, E: EaseLinear}}
			c.Keyframes = map[string][]Keyframe{"width": linearKeys(0.3, 0.6, 40)}
		}), pal, true},
	}
	for _, tc := range cases {
		if geometry, _ := clipAnimation(tc.clip); geometry != tc.pad {
			t.Fatalf("%s: animated geometry %v, want %v", tc.name, geometry, tc.pad)
		}
		for _, at := range []int{tc.clip.Start, tc.clip.Start + 4} {
			local := tc.clip
			local.Start, local.Duration = 0, tc.clip.Start+tc.clip.Duration-at
			c := clipChain{track: 1, index: 3, item: layerClip{clip: local, whole: tc.clip, at: at, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: tc.rate}
			if len(tc.clip.Ramp) > 0 {
				c.item.pieces = rampPieces(tc.clip.Ramp, 0, float64(tc.clip.Duration))
			}
			plan := maskPlanOf(c)
			expr := revealExpr(c, plan)
			step := float64(tc.rate.Den) / float64(tc.rate.Num)
			var registers [10]float64
			hidden, shown := 0, 0
			for k := range local.Duration {
				from, to, active := revealModel(c, plan, at+k)
				for x := range plan.canvasWidth {
					want := 0.0
					if x >= from && x < to {
						want = 255
					}
					got := expr.evalWith(map[string]float64{"T": float64(k) * step, "X": float64(x)}, &registers)
					if got != want {
						t.Fatalf("%s, window at %d: frame %d column %d gives %v, want %v (shown [%d, %d) of %d)", tc.name, at, at+k, x, got, want, from, to, plan.canvasWidth)
					}
					if active && want == 0 {
						hidden++
					} else if active {
						shown++
					}
				}
			}
			if hidden == 0 || shown == 0 {
				t.Fatalf("%s, window at %d: the spans hid %d and showed %d columns; the case does not test the edges", tc.name, at, hidden, shown)
			}
		}
	}
}
