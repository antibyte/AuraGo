package videostudio

import (
	"math"
	"reflect"
	"testing"
)

// referenceGeometry is the clip layout computed from scratch, without the memo: the oracle the memoized geometry must
// equal byte for byte. It sizes the window and, when the window is not the whole clip, the whole clip frame by frame.
func referenceGeometry(c clipChain) geometryPlan {
	whole := c.item.whole
	g := geometryPlan{box: boxExprs(whole, c.item.at, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate)}
	step := float64(c.rate.Den) / float64(c.rate.Num)
	sizes := func(box boxExpr, frames int) (widths, heights []int, rotated bool) {
		widths, heights = make([]int, frames), make([]int, frames)
		for k := range frames {
			vars := map[string]float64{"t": float64(k) * step}
			widths[k], heights[k] = int(box.w.eval(vars)), int(box.h.eval(vars))
			if math.Abs(box.angle.eval(vars)) > 0.000001*math.Pi/180 {
				rotated = true
			}
		}
		return widths, heights, rotated
	}
	g.widths, g.heights, g.rotated = sizes(g.box, c.item.clip.Duration)
	clipWidths, clipHeights := g.widths, g.heights
	if c.item.at != whole.Start || c.item.clip.Duration != whole.Duration {
		clipWidths, clipHeights, g.rotated = sizes(boxExprs(whole, whole.Start, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate), whole.Duration)
	}
	for k := range clipWidths {
		g.padWidth, g.padHeight = max(g.padWidth, clipWidths[k]), max(g.padHeight, clipHeights[k])
	}
	if whole.Fit == FitCover {
		g.boundWidth, g.boundHeight, g.low, g.high = coverBound(clipWidths, clipHeights, c.item.sourceAspect)
	}
	g.layerWidth, g.layerHeight = g.padWidth, g.padHeight
	if g.rotated {
		g.padWidth, g.padHeight = g.padWidth+2, g.padHeight+2
		side := evenCeil(math.Hypot(float64(g.padWidth), float64(g.padHeight)))
		g.layerWidth, g.layerHeight = side, side
	}
	return g
}

// A clip with keyframed geometry is sized over all of its frames (the pad canvas, the rotate stage and the cover bound),
// whatever window a segment shows. A one-hour clip is 108000 frames, about half a second of expression evaluation, and
// every stage of every window asks again (filter, place and the command file each call geometry), so a render that
// repeated it spent minutes on graph building. The whole-clip sizes are now computed once per clip: wholeExtentHook
// counts the computations, and the result equals the from-scratch layout.
func TestGeometrySizesTheWholeClipOnce(t *testing.T) {
	const frames, windowFrames = 18000, 300 // ten minutes at 30 fps, windows of ten seconds
	rate := Rate{Num: 30, Den: 1}
	keyed := func(width float64) Clip {
		return Clip{ID: "long", AssetID: "a", Start: 90, Duration: frames, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitCover,
			Keyframes: map[string][]Keyframe{
				"width":    {{F: 0, V: width, E: EaseLinear}, {F: frames / 2, V: 0.9, E: EaseInOut}, {F: frames, V: 0.35, E: EaseLinear}},
				"rotation": {{F: 0, V: 0, E: EaseLinear}, {F: frames, V: 90, E: EaseOut}},
			}}
	}
	// chain returns the chain of window number n of the clip, as segmentLayers builds it.
	chain := func(whole Clip, n, canvasWidth, canvasHeight int) clipChain {
		local := whole
		local.Start, local.Duration = 0, windowFrames
		return clipChain{track: 1, index: 2, item: layerClip{clip: local, whole: whole, at: whole.Start + n*windowFrames, sourceAspect: 16.0 / 9}, canvasWidth: canvasWidth, canvasHeight: canvasHeight, rate: rate}
	}
	computed := 0
	wholeExtentHook = func() { computed++ }
	defer func() { wholeExtentHook = nil }()

	whole := keyed(0.2371) // a width no other test uses, so the memo holds no entry for this clip yet
	windows := frames / windowFrames
	for n := 0; n < windows; n++ {
		c := chain(whole, n, 1920, 1080)
		_ = c.filter("out") // transform and place
		if _, ok := c.commandFile(); !ok {
			t.Fatal("a cover clip with keyframed geometry has a command file")
		}
	}
	if computed != 1 {
		t.Fatalf("the whole clip was sized %d times for %d windows, want once", computed, windows)
	}
	// The memoized layout equals the from-scratch one: first, middle and last window.
	for _, n := range []int{0, windows / 2, windows - 1} {
		c := chain(whole, n, 1920, 1080)
		if got, want := c.geometry(), referenceGeometry(c); !reflect.DeepEqual(got, want) {
			t.Fatalf("window %d: geometry differs from the from-scratch layout:\n got %+v\nwant %+v", n, got, want)
		}
	}
	if computed != 1 {
		t.Fatalf("comparing with the reference sized the clip again: %d computations", computed)
	}
	// Whatever feeds the sizes is part of the key: another keyframe value, another canvas or another source aspect is
	// another clip layout.
	other := keyed(0.2372)
	if got, want := chain(other, 3, 1920, 1080).geometry(), referenceGeometry(chain(other, 3, 1920, 1080)); !reflect.DeepEqual(got, want) || computed != 2 {
		t.Fatalf("another keyframe value: %d computations, geometry equal %v", computed, reflect.DeepEqual(got, want))
	}
	if got, want := chain(whole, 3, 1280, 720).geometry(), referenceGeometry(chain(whole, 3, 1280, 720)); !reflect.DeepEqual(got, want) || computed != 3 {
		t.Fatalf("another canvas: %d computations, geometry equal %v", computed, reflect.DeepEqual(got, want))
	}
	narrow := chain(whole, 3, 1920, 1080)
	narrow.item.sourceAspect = 1
	if got, want := narrow.geometry(), referenceGeometry(narrow); !reflect.DeepEqual(got, want) || computed != 4 {
		t.Fatalf("another source aspect: %d computations, geometry equal %v", computed, reflect.DeepEqual(got, want))
	}
	// A clip that fits one window is sized from that window's own frames, as before.
	short := keyed(0.2373)
	short.Duration = windowFrames
	short.Keyframes = map[string][]Keyframe{"width": {{F: 0, V: 0.2, E: EaseLinear}, {F: windowFrames, V: 0.6, E: EaseLinear}}}
	one := clipChain{item: layerClip{clip: short, whole: short, at: short.Start, sourceAspect: 16.0 / 9}, canvasWidth: 1920, canvasHeight: 1080, rate: rate}
	if got, want := one.geometry(), referenceGeometry(one); !reflect.DeepEqual(got, want) {
		t.Fatalf("one-window clip: geometry differs from the from-scratch layout:\n got %+v\nwant %+v", got, want)
	}
}
