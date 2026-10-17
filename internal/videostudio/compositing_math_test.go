package videostudio

import (
	"maps"
	"math"
	"testing"
)

// The exact 8-bit blend forms on hand-checked points (spike 2026-10-17, index §4.22): the ends of dodge and burn, the
// overlay and hardlight threshold at 127/128, the softlight truncations, the composite mix and the transition rule.
// The real forms stay within the truncation error of the 8-bit forms.
func TestBlend8MatchesSpike(t *testing.T) {
	if !maps.Equal(blendSwapped, map[string]bool{"dodge": true, "burn": true}) {
		t.Fatalf("blendSwapped = %v, want dodge and burn (TestVideoStudioPictureConstantsMatchGo pins the JS mirror)", blendSwapped)
	}
	for _, c := range []struct {
		mode         string
		below, layer uint8
		want         uint8
	}{
		{"multiply", 200, 100, 78},
		{"multiply", 255, 255, 255},
		{"multiply", 0, 255, 0},
		{"screen", 200, 100, 222},
		{"screen", 0, 0, 0},
		{"overlay", 126, 200, 196},
		{"overlay", 127, 200, 198}, // below 127: the multiply branch
		{"overlay", 128, 200, 201}, // below 128: the screen branch
		{"overlay", 129, 200, 201},
		{"hardlight", 200, 127, 198}, // the branch follows the layer
		{"hardlight", 200, 128, 201},
		{"softlight", 100, 200, 133}, // truncated products; the real Pegtop form gives 134.6
		{"softlight", 128, 0, 64},
		{"softlight", 255, 255, 255},
		{"darken", 200, 100, 100},
		{"lighten", 200, 100, 200},
		{"difference", 100, 200, 100},
		{"exclusion", 200, 100, 144},
		{"addition", 200, 100, 255},
		{"addition", 100, 50, 150},
		{"subtract", 200, 100, 100}, // below − layer
		{"subtract", 100, 200, 0},
		{"dodge", 100, 100, 165},
		{"dodge", 100, 200, 255},
		{"dodge", 0, 255, 255}, // layer 255 gives 255 even over black
		{"dodge", 128, 0, 128},
		{"burn", 200, 100, 115},
		{"burn", 100, 200, 57},
		{"burn", 255, 0, 0}, // layer 0 gives 0 even over white
		{"burn", 0, 255, 0},
		{"normal", 10, 200, 200},
		{"glow", 10, 200, 200}, // unknown modes: the layer
	} {
		if got := blend8(c.mode, c.below, c.layer); got != c.want {
			t.Errorf("blend8(%s, below %d, layer %d) = %d, want %d", c.mode, c.below, c.layer, got, c.want)
		}
	}
	// The mask merge is blend c3_mode=multiply: alpha·m8//255, exactly 0 at m8 = 0 and exactly the alpha at 255.
	if blend8("multiply", 200, 0) != 0 || blend8("multiply", 200, 255) != 200 || blend8("multiply", 200, 128) != 100 {
		t.Error("the mask merge must be alpha·m8//255")
	}
	// maskedmerge: (below·(255 − α) + E·α + 127)//255, which is the float mix rounded to the nearest level.
	if mix8(100, 200, 0) != 100 || mix8(100, 200, 255) != 200 || mix8(100, 200, 128) != 150 {
		t.Errorf("mix8 at α 0, 255, 128 = %d, %d, %d; want 100, 200, 150", mix8(100, 200, 0), mix8(100, 200, 255), mix8(100, 200, 128))
	}
	for below := 0; below < 256; below += 15 {
		for blended := 0; blended < 256; blended += 17 {
			for alpha := 0; alpha < 256; alpha += 5 {
				want := math.Round(float64(below) + float64(blended-below)*float64(alpha)/255)
				if got := mix8(uint8(below), uint8(blended), uint8(alpha)); float64(got) != want {
					t.Fatalf("mix8(%d, %d, %d) = %d, want the rounded float mix %v", below, blended, alpha, got, want)
				}
			}
		}
	}
	// The real forms differ from the 8-bit forms by the truncations only: softlight's three truncated products up to 4
	// levels (spike: 3.97), every other mode up to 2.
	for _, mode := range EffectRegistry().BlendModes {
		bound := 2.0
		if mode.ID == "softlight" {
			bound = 4
		}
		for a := 0; a < 256; a += 5 {
			for b := 0; b < 256; b += 5 {
				real := math.Round(255 * blendChannel(mode.ID, float64(a)/255, float64(b)/255))
				if diff := math.Abs(real - float64(blend8(mode.ID, uint8(a), uint8(b)))); diff > bound {
					t.Fatalf("%s at below %d, layer %d: real form %v vs blend8 %d", mode.ID, a, b, real, blend8(mode.ID, uint8(a), uint8(b)))
				}
			}
		}
	}
	near := func(label string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	near("dodge with layer 1 over black", blendChannel("dodge", 0, 1), 1)
	near("burn with layer 0 over white", blendChannel("burn", 1, 0), 0)
	near("overlay just below the threshold", blendChannel("overlay", 0.49, 0.3), 2*0.49*0.3)
	near("overlay at the threshold", blendChannel("overlay", 0.5, 0.3), 0.3)
	near("screen at alpha 0.5", compositeChannel("screen", 0.2, 0.6, 0.5), 0.2+(1-0.8*0.4-0.2)*0.5)
	near("alpha 0 keeps the composite below", compositeChannel("multiply", 0.4, 0.5, 0), 0.4)
	near("normal is the straight-alpha mix", compositeChannel(BlendNormal, 0.2, 0.6, 0.25), 0.3)
	// The overlap of a transition takes the shared mode, else normal; an empty name is normal.
	for _, c := range [][3]string{{"multiply", "multiply", "multiply"}, {"multiply", "screen", BlendNormal}, {"", BlendNormal, BlendNormal}, {"screen", "", BlendNormal}, {"", "", BlendNormal}} {
		if got := overlapBlend(c[0], c[1]); got != c[2] {
			t.Errorf("overlapBlend(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// The exact colorkey and despill forms (spike 2026-10-17, index §4.22) on hand-checked points: the similarity and the
// float32 1/smoothness ramp, the smoothness 0 step, and despill green and blue with mix 0.5, expand 0 and the screen
// channel's scale −spill. The real forms stay within one level of them.
func TestKey8MatchesSpike(t *testing.T) {
	if keyColor("#00b140") != [3]uint8{0, 177, 64} || keyColor("#00B140") != [3]uint8{0, 177, 64} || keyColor("#0047bb") != [3]uint8{0, 71, 187} || keyColor("green") != [3]uint8{} {
		t.Fatalf("keyColor = %v %v %v %v", keyColor("#00b140"), keyColor("#00B140"), keyColor("#0047bb"), keyColor("green"))
	}
	unit := func(rgb [3]uint8) [3]float64 {
		return [3]float64{float64(rgb[0]) / 255, float64(rgb[1]) / 255, float64(rgb[2]) / 255}
	}
	green := keyParams{Color: keyColor("#00b140"), Similarity: 0.25, Smoothness: 0.1, Spill: 0.5}
	hard := green
	hard.Smoothness = 0
	wide := green
	wide.Similarity, wide.Smoothness = 0.3, 0.2
	for _, c := range []struct {
		name string
		p    keyParams
		rgb  [3]uint8
		want uint8
	}{
		{"the key colour", green, [3]uint8{0, 177, 64}, 0},
		{"white", green, [3]uint8{255, 255, 255}, 255},
		{"inside the similarity (d 0.2264)", green, [3]uint8{100, 177, 64}, 0},
		{"on the ramp (d 0.2717)", green, [3]uint8{120, 177, 64}, 55},
		{"high on the ramp (d 0.3396)", green, [3]uint8{150, 177, 64}, 228},
		{"smoothness 0 is a hard step: above", hard, [3]uint8{120, 177, 64}, 255},
		{"smoothness 0 is a hard step: below", hard, [3]uint8{100, 177, 64}, 0},
		{"similarity 0.3, smoothness 0.2 (float32 similarity and 1/blend)", wide, [3]uint8{150, 177, 64}, 50},
	} {
		if got := keyAlpha8(c.p, c.rgb); got != c.want {
			t.Errorf("%s: keyAlpha8 = %d, want %d", c.name, got, c.want)
		}
		if real := 255 * keyAlpha(c.p, unit(c.rgb)); real < float64(c.want)-1e-9 || real >= float64(c.want)+1 {
			t.Errorf("%s: 255·keyAlpha = %v, want [%d, %d)", c.name, real, c.want, c.want+1)
		}
	}
	full, light, off := green, green, green
	full.Spill, light.Spill, off.Spill = 1, 0.3, 0
	blue := keyParams{Color: keyColor("#0047bb"), Similarity: 0.25, Smoothness: 0.1, Spill: 0.5}
	red := keyParams{Color: keyColor("#ff0000"), Similarity: 0.25, Smoothness: 0.1, Spill: 0.5}
	for _, c := range []struct {
		name      string
		p         keyParams
		rgb, want [3]uint8
	}{
		{"green on a green key: g − 0.5·1", green, [3]uint8{0, 255, 0}, [3]uint8{0, 127, 0}},
		{"yellow: S = 1 − 0.5", green, [3]uint8{255, 255, 0}, [3]uint8{255, 191, 0}},
		{"white has no spill", green, [3]uint8{255, 255, 255}, [3]uint8{255, 255, 255}},
		{"a mid colour through the float32 path", green, [3]uint8{100, 200, 50}, [3]uint8{100, 137, 50}},
		{"spill 1 removes the whole excess", full, [3]uint8{0, 255, 0}, [3]uint8{0, 0, 0}},
		{"spill 0.3", light, [3]uint8{0, 255, 0}, [3]uint8{0, 178, 0}},
		{"spill 0 leaves despill out", off, [3]uint8{0, 255, 0}, [3]uint8{0, 255, 0}},
		{"a blue key reduces blue", blue, [3]uint8{0, 0, 255}, [3]uint8{0, 0, 127}},
		{"magenta on a blue key", blue, [3]uint8{255, 0, 255}, [3]uint8{255, 0, 191}},
		{"green on a blue key has no blue spill", blue, [3]uint8{0, 255, 0}, [3]uint8{0, 255, 0}},
		{"a red key uses the green type (kg ≥ kb)", red, [3]uint8{0, 255, 0}, [3]uint8{0, 127, 0}},
	} {
		if got := despill8(c.p, c.rgb); got != c.want {
			t.Errorf("%s: despill8 = %v, want %v", c.name, got, c.want)
		}
		real := despillRGB(c.p, unit(c.rgb))
		for i := range real {
			if level := 255 * real[i]; level < float64(c.want[i])-1e-9 || level >= float64(c.want[i])+1 {
				t.Errorf("%s: 255·despillRGB[%d] = %v, want [%d, %d)", c.name, i, level, c.want[i], c.want[i]+1)
			}
		}
	}
	// keyParamsAt reads the entry at a frame: keyed similarity, stored params, the registry default for a missing or
	// invalid colour.
	clip := Clip{Effects: []EffectInstance{{ID: "chroma_key", On: true, Params: map[string]any{"color": "#0047BB", "spill": 0.7},
		Keyframes: map[string][]Keyframe{"similarity": {{F: 0, V: 0.1, E: EaseLinear}, {F: 10, V: 0.3, E: EaseLinear}}}}}}
	chromaKeyDefault := func(param string) float64 { // the registry default: a Task 12 tuning keeps this test green
		spec, _ := effectParamSpec("chroma_key", param)
		value, _ := spec.Default.(float64)
		return value
	}
	p := keyParamsAt(clip, 0, 5)
	if p.Color != [3]uint8{0, 71, 187} || math.Abs(p.Similarity-0.2) > 1e-12 || p.Smoothness != chromaKeyDefault("smoothness") || p.Spill != 0.7 {
		t.Errorf("keyParamsAt = %+v", p)
	}
	clip.Effects[0].Params = map[string]any{"color": "green"}
	if p := keyParamsAt(clip, 0, 0); p.Color != [3]uint8{0, 177, 64} || p.Spill != chromaKeyDefault("spill") {
		t.Errorf("an invalid colour and a missing spill read the registry defaults: %+v", p)
	}
}

// Law L in box pixels (index §4.22): inside, outside and on the edge; the exact rectangle distance and the ellipse
// distance on its axes; rotation clockwise on screen; invert; the 1 px minimum ramp; the frame of a keyed mask; and
// which clips need a mask per frame.
func TestMaskAtShapes(t *testing.T) {
	near := func(label string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	rect := Mask{Shape: MaskRect, X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
	frame := maskFrameOf(rect, 400, 200)
	if frame.CX != 200 || frame.CY != 100 || frame.RX != 100 || frame.RY != 50 || frame.Ramp != 1 {
		t.Fatalf("maskFrameOf(rect, 400x200) = %+v", frame)
	}
	near("rect centre", maskAt(frame, 200, 100), 1)
	near("rect outside", maskAt(frame, 350, 100), 0)
	near("on the edge", maskAt(frame, 300, 100), 0.5)
	near("a quarter pixel outside the edge (F = 1 px)", maskAt(frame, 300.25, 100), 0.25)
	near("a quarter pixel inside", maskAt(frame, 299.75, 100), 0.75)
	near("below the bottom edge", maskAt(frame, 200, 160), 0)
	soft := rect
	soft.Feather = 0.1 // F = 20 px
	near("feather 20 px, 5 px outside", maskAt(maskFrameOf(soft, 400, 200), 305, 100), 0.25)
	near("outside a corner", maskAt(maskFrameOf(soft, 400, 200), 310, 160), 0)
	wide := rect
	wide.Feather = 0.2 // F = 40 px
	near("the corner distance is the exact hypot", maskAt(maskFrameOf(wide, 400, 200), 310, 160), 0.5-math.Hypot(10, 10)/40)
	turned := rect
	turned.Rotation = 90
	near("a 90° turn swaps the extents", maskAt(maskFrameOf(turned, 400, 200), 200, 160), 1)
	tilted := rect
	tilted.Rotation = 30
	tf := maskFrameOf(tilted, 400, 200)
	cos, sin := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
	near("positive rotation turns clockwise on screen", maskAt(tf, 200+90*cos, 100+90*sin), 1)
	near("and not counter-clockwise", maskAt(tf, 200+90*cos, 100-90*sin), 0)
	ellipse := Mask{Shape: MaskEllipse, X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Feather: 0.2}
	ef := maskFrameOf(ellipse, 400, 200)
	near("ellipse on its x axis, 10 px inside", maskAt(ef, 290, 100), 0.75)
	near("ellipse on its y axis, 10 px inside", maskAt(ef, 200, 140), 0.75)
	near("ellipse on its x axis, 10 px outside", maskAt(ef, 310, 100), 0.25)
	near("ellipse centre", maskAt(ef, 200, 100), 1)
	inverted := ellipse
	inverted.Invert = true
	near("invert", maskAt(maskFrameOf(inverted, 400, 200), 290, 100), 0.25)
	near("invert at the centre", maskAt(maskFrameOf(inverted, 400, 200), 200, 100), 0)
	if ramp := maskFrameOf(Mask{Shape: MaskRect, X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Feather: 0.05}, 20, 10).Ramp; ramp != 1 {
		t.Errorf("feather 0.05 of a 10 px box: ramp %v, want the 1 px minimum", ramp)
	}
	keyed := Clip{Duration: 30, Mask: &Mask{Shape: MaskEllipse, X: 0, Y: 0.25, W: 0.5, H: 0.5, Rotation: 10, Feather: 0.1},
		Keyframes: map[string][]Keyframe{"mask.x": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 0.5, E: EaseLinear}}}}
	at, ok := maskFrameAt(keyed, 5, 400, 200)
	if !ok || math.Abs(at.CX-200) > 1e-9 || math.Abs(at.CY-100) > 1e-9 || math.Abs(at.Ramp-20) > 1e-9 || at.Shape != MaskEllipse || math.Abs(at.Cos-math.Cos(math.Pi/18)) > 1e-12 {
		t.Errorf("maskFrameAt(keyed, 5) = %+v, %v; want the centre at (200, 100) from mask.x 0.25", at, ok)
	}
	if _, ok := maskFrameAt(Clip{}, 0, 400, 200); ok {
		t.Error("maskFrameAt resolved a clip without a mask")
	}
	static := Clip{Mask: &rect}
	moving := Clip{Mask: &rect, Keyframes: map[string][]Keyframe{"x": {{F: 0, V: 0.1, E: EaseLinear}}}}
	unmasked := Clip{Keyframes: map[string][]Keyframe{"x": {{F: 0, V: 0.1, E: EaseLinear}}}}
	if !maskAnimated(keyed) || !maskPerFrame(keyed) || maskAnimated(static) || maskPerFrame(static) || maskAnimated(moving) || !maskPerFrame(moving) || maskPerFrame(unmasked) {
		t.Error("maskAnimated follows the mask.* keys; maskPerFrame adds animated box geometry and needs a mask")
	}
}
