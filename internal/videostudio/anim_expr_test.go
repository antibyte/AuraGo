package videostudio

import (
	"reflect"
	"strings"
	"testing"
)

// TestBoxExprsMatchAnimBox evaluates the box expressions of animated clips (boxExprs with animExprs, index §4.10,
// §4.24) in Go, with FFmpeg's arithmetic, at three rates. It runs every frame of windows that start at, 7 and 23 frames
// into the clip. Size and centre must equal animBox of clipBox over ValueAt (keyframedBox) and animAt, exactly.
//
// Cases:
//   - pop and slide on both sides, and spans clamped to half a 9-frame clip;
//   - a ramped clip with keyed x and width, a rotated clip, cover;
//   - a box that leaves the canvas and a full-canvas pop past it.
//
// The animation terms store only registers 4, 7 and 8 and read the frame from register 0. A clip without a slide or
// pop, or without an effective span, keeps the 2b expressions byte for byte.
func TestBoxExprsMatchAnimBox(t *testing.T) {
	clip := func(edit func(*Clip)) Clip {
		c := Clip{ID: "a", AssetID: "v", Start: 10, Offset: 7, Duration: 40, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain}
		edit(&c)
		return c
	}
	anim := func(preset string, frames int) *ClipAnim { return &ClipAnim{Preset: preset, Duration: frames} }
	withoutAnim := func(c Clip) Clip {
		c.AnimIn, c.AnimOut = nil, nil
		return c
	}
	cases := map[string]Clip{
		"pop in, slide_left out": clip(func(c *Clip) { c.AnimIn, c.AnimOut = anim("pop", 12), anim("slide_left", 10) }),
		"slide_up in, pop out, clamped to 4 frames": clip(func(c *Clip) {
			c.Duration, c.AnimIn, c.AnimOut = 9, anim("slide_up", 30), anim("pop", 30)
		}),
		"ramped with keyed x and width": clip(func(c *Clip) {
			c.Ramp = []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 2, E: EaseOut}}
			c.Keyframes = map[string][]Keyframe{
				"x":     {{F: 7, V: 0.05, E: EaseInOut}, {F: 40, V: 0.4, E: EaseLinear}},
				"width": {{F: 10, V: 0.3, E: EaseIn}, {F: 50, V: 0.7, E: EaseLinear}},
			}
			c.AnimIn, c.AnimOut = anim("slide_right", 15), anim("pop", 15)
		}),
		"rotated, slide_down in":           clip(func(c *Clip) { c.Rotation, c.AnimIn = 20, anim("slide_down", 13) }),
		"cover, pop in":                    clip(func(c *Clip) { c.Fit, c.AnimIn = FitCover, anim("pop", 10) }),
		"off the canvas, slide_left out":   clip(func(c *Clip) { c.X, c.AnimOut = 0, anim("slide_left", 12) }),
		"full canvas, pop past the canvas": clip(func(c *Clip) { c.X, c.Y, c.Width, c.Height, c.AnimIn = 0, 0, 1, 1, anim("pop", 10) }),
	}
	for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}, {Num: 60, Den: 1}} {
		for name, c := range cases {
			var pieces []rampPiece
			if len(c.Ramp) > 0 {
				pieces = rampPieces(c.Ramp, 0, float64(c.Duration))
			}
			for _, at := range []int{c.Start, c.Start + 7, c.Start + 23} {
				if at >= c.Start+c.Duration {
					continue
				}
				box := boxExprs(c, at, pieces, 1280, 720, rate)
				if got, want := box.angle.String(), boxExprs(withoutAnim(c), at, pieces, 1280, 720, rate).angle.String(); got != want {
					t.Fatalf("%s: the animation changed the angle:\n got %s\nwant %s", name, got, want)
				}
				for k := 0; at+k < c.Start+c.Duration; k++ {
					frame := at + k
					vars := map[string]float64{"t": float64(k) * (float64(rate.Den) / float64(rate.Num))}
					rest := keyframedBox(t, c, frame, 1280, 720)
					cx, cy, bw, bh := animBox(rest.Min.X, rest.Min.Y, rest.Dx(), rest.Dy(), animAt(c, frame), 1280, 720)
					if gw, gh := box.w.eval(vars), box.h.eval(vars); gw != float64(bw) || gh != float64(bh) {
						t.Fatalf("%s at %s, window %d, frame %d: box %vx%v, want %dx%d", name, rate, at, frame, gw, gh, bw, bh)
					}
					if gx, gy := box.cx.eval(vars), box.cy.eval(vars); gx != float64(cx) || gy != float64(cy) {
						t.Fatalf("%s at %s, window %d, frame %d: centre (%v, %v), want (%d, %d)", name, rate, at, frame, gx, gy, cx, cy)
					}
				}
			}
		}
	}
	// The animation terms store 4, 7 and 8 only and read nothing but those and the frame in register 0.
	for name, c := range cases {
		a := animExprs(c, exprLd(0))
		var visit func(ffExpr)
		visit = func(x ffExpr) {
			if x.op == "st" || x.op == "ld" {
				r := int(x.args[0].num)
				if x.op == "st" && r != 4 && r != 7 && r != 8 || x.op == "ld" && r != 0 && r != 4 && r != 7 && r != 8 {
					t.Fatalf("%s: the animation term uses register %d: %s", name, r, x.String())
				}
			}
			for _, arg := range x.args {
				visit(arg)
			}
		}
		for _, part := range []ffExpr{a.scale, a.dx, a.dy} {
			visit(part)
		}
	}
	// Without a slide or pop, or without an effective span, the expressions are 2b's byte for byte.
	pal := Rate{Num: 25, Den: 1}
	plain := boxExprs(clip(func(*Clip) {}), 10, nil, 1280, 720, pal)
	for name, c := range map[string]Clip{
		"fade":                          clip(func(c *Clip) { c.AnimIn, c.AnimOut = anim("fade", 10), anim("fade", 8) }),
		"wipe":                          clip(func(c *Clip) { c.AnimIn = anim("wipe", 10) }),
		"blur_in":                       clip(func(c *Clip) { c.AnimIn = anim("blur_in", 10) }),
		"pop without an effective span": clip(func(c *Clip) { c.Duration, c.AnimIn = 1, anim("pop", 10) }),
	} {
		got := boxExprs(c, 10, nil, 1280, 720, pal)
		for i, pair := range [][2]ffExpr{{got.w, plain.w}, {got.h, plain.h}, {got.cx, plain.cx}, {got.cy, plain.cy}, {got.angle, plain.angle}} {
			if pair[0].String() != pair[1].String() {
				t.Fatalf("%s: expression %d changed:\n got %s\nwant %s", name, i, pair[0].String(), pair[1].String())
			}
		}
		if a := animExprs(c, exprLd(0)); !exprIsNum(a.scale, 1) || !exprIsNum(a.dx, 0) || !exprIsNum(a.dy, 0) {
			t.Fatalf("%s: animExprs = %s, %s, %s; want 1, 0, 0", name, a.scale.String(), a.dx.String(), a.dy.String())
		}
	}
}

// The animation's opacity is part of the layer alpha before the track composites (index §4.24, blend modes). A
// multiply clip with a fade keeps the blend runs and the composite of the same clip without the fade. Only its own
// chain gains the opacity filter and its commands.
func TestAnimLayersKeepBlendRuns(t *testing.T) {
	seg := Segment{Start: 0, End: 30}
	build := func(top Clip) ([]blendRun, string, graphFiles) {
		project, sources := blendGraphProject(top)
		rate := ProjectRate(project)
		layers, _, err := segmentLayers(project, seg, sources, rate)
		if err != nil {
			t.Fatal(err)
		}
		_, graph, files, err := buildVideoSegmentWith(project, seg, sources, rate, videoTail{width: 1280, height: 720})
		if err != nil {
			t.Fatal(err)
		}
		return blendRuns(layers[1], 30), graph, files
	}
	plain := blendGraphClip("top", 0, 30, "multiply", nil)
	faded := plain
	faded.AnimIn = &ClipAnim{Preset: "fade", Duration: 10}
	plainRuns, plainGraph, _ := build(plain)
	runs, graph, files := build(faded)
	if !reflect.DeepEqual(runs, plainRuns) {
		t.Fatalf("blend runs %+v, want those without the fade %+v", runs, plainRuns)
	}
	if n := strings.Count(graph, "c0_mode=multiply"); n == 0 || n != strings.Count(plainGraph, "c0_mode=multiply") {
		t.Fatalf("multiply composites: %d, without the fade %d", n, strings.Count(plainGraph, "c0_mode=multiply"))
	}
	if !strings.Contains(graph, "colorchannelmixer@vs_opacity_1_0=aa=0[vs_layer_1_0]") || strings.Contains(plainGraph, "vs_opacity_1_0") {
		t.Fatalf("the fade's opacity must end the clip chain, in front of its layer label:\n%s", graph)
	}
	found := false
	for _, file := range files.commands {
		found = found || file.name == "vs_cmd_1_0.txt" && len(file.tracks) == 1 && file.tracks[0].Values[5] == 0.5
	}
	if !found {
		t.Fatalf("no opacity commands with 0.5 at frame 5 in %+v", files.commands)
	}
}
