package goldenframe

import (
	"context"
	"image"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// cdTop returns the clips of a project's top track (the last one: tracks paint in array order), or nil with fewer than
// two tracks. Every 2d case tests the clips of its top track.
func cdTop(project videostudio.Project) []videostudio.Clip {
	if len(project.Tracks) < 2 {
		return nil
	}
	return project.Tracks[len(project.Tracks)-1].Clips
}

// cdBlends lists the clips' Blend fields; "" is normal.
func cdBlends(clips []videostudio.Clip) []string {
	out := []string{}
	for _, clip := range clips {
		out = append(out, clip.Blend)
	}
	return out
}

// cdEffectIDs lists a clip's effect ids in stack order (nil without effects).
func cdEffectIDs(clip videostudio.Clip) []string {
	var ids []string
	for _, entry := range clip.Effects {
		ids = append(ids, entry.ID)
	}
	return ids
}

// cdStatic is a path's static value, the one a painter that ignored the keyframes would use: a mask field, a box
// field, else pcStatic (crop sides and effect params).
func cdStatic(clip videostudio.Clip, path string) float64 {
	if field, ok := strings.CutPrefix(path, "mask."); ok {
		if clip.Mask == nil {
			return math.NaN()
		}
		switch field {
		case "x":
			return clip.Mask.X
		case "y":
			return clip.Mask.Y
		case "w":
			return clip.Mask.W
		case "h":
			return clip.Mask.H
		case "rotation":
			return clip.Mask.Rotation
		case "feather":
			return clip.Mask.Feather
		}
		return math.NaN()
	}
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
	return pcStatic(clip, path)
}

// cdKeyDefaults returns chroma_key's registry defaults: the numbers by param id, and the key colour.
func cdKeyDefaults(t *testing.T) (map[string]float64, string) {
	t.Helper()
	spec, ok := videostudio.EffectByID("chroma_key")
	if !ok {
		t.Fatal("the registry has no chroma_key effect (plan 2d Task 1)")
	}
	numbers, colour := map[string]float64{}, ""
	for _, param := range spec.Params {
		switch value := param.Default.(type) {
		case float64:
			numbers[param.ID] = value
		case string:
			if param.ID == "color" {
				colour = value
			}
		}
	}
	return numbers, colour
}

func cdHex(t *testing.T, hex string) [3]uint8 {
	t.Helper()
	value, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil || len(hex) != 7 || !strings.HasPrefix(hex, "#") {
		t.Fatalf("key colour %q is not #rrggbb", hex)
	}
	return [3]uint8{uint8(value >> 16), uint8(value >> 8), uint8(value)}
}

// cdKeyDistance is colorkey's colour distance (index §4.22): |rgb − key| / (255·√3), channels in 0..255.
func cdKeyDistance(rgb, key [3]uint8) float64 {
	sum := 0.0
	for c := range rgb {
		d := float64(rgb[c]) - float64(key[c])
		sum += d * d
	}
	return math.Sqrt(sum / (255 * 255 * 3))
}

// cdKeyAlpha is colorkey's alpha in its real form (index §4.22): clip((d − similarity)/smoothness, 0, 1), and a hard
// step at smoothness 0. videostudio's keyAlpha is unexported, so the test states the law itself.
func cdKeyAlpha(rgb, key [3]uint8, similarity, smoothness float64) float64 {
	d := cdKeyDistance(rgb, key)
	if smoothness <= 0 {
		if d > similarity {
			return 1
		}
		return 0
	}
	return math.Max(0, math.Min(1, (d-similarity)/smoothness))
}

// TestCompositingCasesCoverGroups pins the 2d cases (index §4.12):
//   - every blend mode has a case, and every case has a Delta with its group's bounds and a valid baseline;
//   - the values at each case frame, through ValueAt;
//   - the four 2c opt-ins.
func TestCompositingCasesCoverGroups(t *testing.T) {
	numbers, _ := cdKeyDefaults(t)
	ellipse, rect := videostudio.MaskEllipse, videostudio.MaskRect
	normal := []string{""}
	keyAt := func(similarity, smoothness, spill float64) map[string]float64 {
		return map[string]float64{"effects.0.similarity": similarity, "effects.0.smoothness": smoothness, "effects.0.spill": spill}
	}
	maskAt := func(x, y, w, h, rotation, feather float64) map[string]float64 {
		return map[string]float64{"mask.x": x, "mask.y": y, "mask.w": w, "mask.h": h, "mask.rotation": rotation, "mask.feather": feather}
	}
	merge := func(maps ...map[string]float64) map[string]float64 {
		out := map[string]float64{}
		for _, values := range maps {
			for path, value := range values {
				out[path] = value
			}
		}
		return out
	}
	type expect struct {
		threshold  float64
		delta      Delta    // the bounds of the case's Delta (its Baseline aside)
		blends     []string // Blend of the top track's clips in order ("" is normal)
		shape      videostudio.MaskShape
		invert     bool
		key        bool               // the subject's stack is [chroma_key]
		values     map[string]float64 // at the case frame, through videostudio.ValueAt (cases_2d.go comments)
		keyed      []string           // keyframed paths: the static value must differ from the keyed one
		progress   float64            // > 0: the frame's transition progress
		baseBlends []string           // the baseline's top track blends (nil: the baseline has no top track)
		baseKey    map[string]float64 // non-nil: the baseline's subject keeps chroma_key at these values
	}
	keyCase := func(values map[string]float64, keyed ...string) expect {
		return expect{threshold: ThresholdDetail, delta: DeltaKey, blends: normal, key: true, values: values, keyed: keyed, baseBlends: normal}
	}
	maskCase := func(shape videostudio.MaskShape, invert bool, values map[string]float64, keyed ...string) expect {
		return expect{threshold: ThresholdDetail, delta: DeltaMask, blends: normal, shape: shape, invert: invert, values: values, keyed: keyed, baseBlends: normal}
	}
	keyedSpill, keyedPaths := 0.2, []string{"effects.0.similarity", "effects.0.smoothness"}
	if cdSpillKeyframable() {
		keyedSpill, keyedPaths = 0.75, append(keyedPaths, "effects.0.spill")
	}
	spillFull := keyCase(keyAt(0.25, 0.1, 1))
	spillFull.baseKey = keyAt(0.25, 0.1, 0)
	defaults := keyAt(numbers["similarity"], numbers["smoothness"], numbers["spill"])
	want := map[string]expect{
		"key_green_default":    keyCase(defaults),
		"key_soft_edge":        keyCase(keyAt(0.2, 0.3, numbers["spill"])),
		"key_spill_full":       spillFull,
		"key_still_alpha":      keyCase(merge(defaults, map[string]float64{"x": 0.1, "y": 0.1, "width": 0.8, "height": 0.8})),
		"key_keyframed":        keyCase(keyAt(0.325, 0.09, keyedSpill), keyedPaths...),
		"mask_rect_static":     maskCase(rect, false, maskAt(0.2, 0.15, 0.5, 0.6, 0, 0)),
		"mask_ellipse_static":  maskCase(ellipse, false, maskAt(0.15, 0.1, 0.7, 0.8, 0, 0.02)),
		"mask_ellipse_rotated": maskCase(ellipse, false, maskAt(0.2, 0.25, 0.6, 0.5, 30, 0.01)),
		"mask_rect_feather":    maskCase(rect, false, maskAt(0.1, 0.1, 0.8, 0.8, -12, 0.15)),
		"mask_ellipse_invert":  maskCase(ellipse, true, maskAt(0.3, 0.2, 0.4, 0.6, 0, 0.03)),
		"mask_keyframed": maskCase(ellipse, false, maskAt(0.35, 0.2, 0.45, 0.6, 30, 0.06),
			"mask.x", "mask.w", "mask.rotation", "mask.feather"),
		"mask_box_animated": maskCase(rect, false, merge(maskAt(0.1, 0.1, 0.8, 0.8, 20, 0.05),
			map[string]float64{"x": 0.35, "y": 0.15, "width": 0.55, "height": 0.5, "rotation": 15}), "x", "width", "rotation"),
		"mask_cover_crop_animated": maskCase(ellipse, false, merge(maskAt(0.1, 0.1, 0.8, 0.8, 0, 0.04), map[string]float64{"crop.l": 0.18}), "crop.l"),
		"blend_normal_alpha":       {threshold: ThresholdColour, delta: DeltaBlend, blends: normal, values: map[string]float64{"opacity": 0.85}},
		"blend_transition_shared": {threshold: ThresholdTransition, delta: DeltaBlend, blends: []string{"multiply", "multiply"},
			values: map[string]float64{"opacity": 0.85}, progress: 0.5, baseBlends: []string{"", ""}},
		"blend_transition_mixed": {threshold: ThresholdTransition, delta: DeltaBlend, blends: []string{"multiply", "screen"},
			values: map[string]float64{"opacity": 0.85}, progress: 0.5, baseBlends: []string{"multiply", "multiply"}},
		"blend_masked_keyed": {threshold: ThresholdDetail, delta: DeltaMask, blends: []string{"screen"}, shape: ellipse, key: true,
			values: merge(keyAt(0.25, 0.1, 0.5), maskAt(0.1, 0.05, 0.8, 0.9, 0, 0.03)), baseBlends: normal},
	}
	// Every blend mode has its case (registry order, index §4.2).
	modes := []string{"multiply", "screen", "overlay", "softlight", "hardlight", "darken", "lighten", "difference", "exclusion", "addition", "subtract", "dodge", "burn"}
	var registered []string
	for _, mode := range videostudio.EffectRegistry().BlendModes {
		if mode.ID != videostudio.BlendNormal {
			registered = append(registered, mode.ID)
		}
	}
	if !slices.Equal(registered, modes) {
		t.Errorf("registry blend modes %v, want %v", registered, modes)
	}
	for _, mode := range modes {
		want["blend_"+mode] = expect{threshold: ThresholdColour, delta: DeltaBlend, blends: []string{mode}, values: map[string]float64{"opacity": 0.85}, baseBlends: normal}
	}
	m := fakeMedia()
	seen, groups := map[string]bool{}, map[string]int{}
	for _, c := range compositingCases(m) {
		spec, known := want[c.Name]
		if !known {
			t.Errorf("unexpected 2d case %q", c.Name)
			continue
		}
		if seen[c.Name] {
			t.Errorf("duplicate 2d case %q", c.Name)
		}
		seen[c.Name] = true
		group, _, _ := strings.Cut(c.Name, "_")
		groups[group]++
		if c.Threshold != spec.threshold {
			t.Errorf("%s: threshold %.2f, want %.2f", c.Name, c.Threshold, spec.threshold)
		}
		if !slices.Equal(c.Painters, []string{PainterGL}) {
			t.Errorf("%s: painters %v, want [gl] (the 2D painter draws no key, blend or mask)", c.Name, c.Painters)
		}
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		top := cdTop(c.Project)
		if len(top) == 0 {
			t.Errorf("%s: no clips on a top track", c.Name)
			continue
		}
		for _, clip := range top {
			if c.Frame < clip.Start || c.Frame >= clip.Start+clip.Duration || c.Frame >= videostudio.EnabledDuration(c.Project) {
				t.Errorf("%s: frame %d lies outside clip %s [%d, %d)", c.Name, c.Frame, clip.ID, clip.Start, clip.Start+clip.Duration)
			}
		}
		if got := cdBlends(top); !slices.Equal(got, spec.blends) {
			t.Errorf("%s: blend modes %q, want %q", c.Name, got, spec.blends)
		}
		subject := top[0]
		switch {
		case spec.shape == "" && subject.Mask != nil:
			t.Errorf("%s: unexpected mask %+v", c.Name, *subject.Mask)
		case spec.shape != "" && (subject.Mask == nil || subject.Mask.Shape != spec.shape || subject.Mask.Invert != spec.invert):
			t.Errorf("%s: mask %+v, want shape %s, invert %v", c.Name, subject.Mask, spec.shape, spec.invert)
		}
		var wantIDs []string
		if spec.key {
			wantIDs = []string{"chroma_key"}
		}
		if ids := cdEffectIDs(subject); !slices.Equal(ids, wantIDs) {
			t.Errorf("%s: effects %v, want %v", c.Name, ids, wantIDs)
		}
		if spec.key {
			registry, _ := videostudio.EffectByID("chroma_key")
			for _, param := range registry.Params {
				if _, present := subject.Effects[0].Params[param.ID]; !present {
					t.Errorf("%s: chroma_key lacks param %s (case entries carry complete params)", c.Name, param.ID)
				}
			}
		}
		for path, value := range spec.values {
			if got, ok := videostudio.ValueAt(subject, path, c.Frame); !ok || math.Abs(got-value) > 1e-9 {
				t.Errorf("%s: %s at frame %d is %v (ok %v), want %v", c.Name, path, c.Frame, got, ok, value)
			}
		}
		for _, path := range spec.keyed {
			if len(pcPathKeys(subject, path)) < 2 {
				t.Errorf("%s: %s has no keyframes", c.Name, path)
			}
			if math.Abs(cdStatic(subject, path)-spec.values[path]) < 0.05 {
				t.Errorf("%s: the static %s %v is too close to the keyed %v, so ignored keyframes would pass", c.Name, path, cdStatic(subject, path), spec.values[path])
			}
		}
		if spec.progress > 0 {
			if got, found := foundationProgressAt(c); !found || got != spec.progress {
				t.Errorf("%s: frame %d is transition progress %v (found %v), want %v", c.Name, c.Frame, got, found, spec.progress)
			}
		}
		// The delta check: every 2d case carries one, with its group's bounds and the feature turned off in the baseline.
		if c.Delta == nil {
			t.Errorf("%s: no Delta; every 2d case carries one (index §4.12)", c.Name)
			continue
		}
		bounds := *c.Delta
		bounds.Baseline = videostudio.Project{}
		if !reflect.DeepEqual(bounds, spec.delta) {
			t.Errorf("%s: delta bounds %+v, want %+v", c.Name, bounds, spec.delta)
		}
		base := c.Delta.Baseline
		if err := videostudio.Validate(base); err != nil {
			t.Errorf("%s: invalid baseline: %v", c.Name, err)
		}
		if base.Width != c.Project.Width || base.Height != c.Project.Height || base.FPS != c.Project.FPS || len(base.Assets) != len(c.Project.Assets) {
			t.Errorf("%s: the baseline's canvas, rate or assets differ from the case", c.Name)
		}
		if c.Frame >= videostudio.EnabledDuration(base) {
			t.Errorf("%s: frame %d lies outside the baseline", c.Name, c.Frame)
		}
		if reflect.DeepEqual(base.Tracks, c.Project.Tracks) {
			t.Errorf("%s: the baseline equals the case, so the delta measures nothing", c.Name)
		}
		baseTop := cdTop(base)
		if got := cdBlends(baseTop); !slices.Equal(got, spec.baseBlends) {
			t.Errorf("%s: baseline blend modes %q, want %q", c.Name, got, spec.baseBlends)
		}
		for _, clip := range baseTop {
			if clip.Mask != nil {
				t.Errorf("%s: the baseline clip %s keeps a mask", c.Name, clip.ID)
			}
		}
		if len(baseTop) > 0 {
			ids := cdEffectIDs(baseTop[0])
			switch {
			case spec.baseKey == nil && len(ids) > 0:
				t.Errorf("%s: the baseline keeps effects %v", c.Name, ids)
			case spec.baseKey != nil && !slices.Equal(ids, []string{"chroma_key"}):
				t.Errorf("%s: baseline effects %v, want [chroma_key]", c.Name, ids)
			case spec.baseKey != nil:
				for path, value := range spec.baseKey {
					if got, ok := videostudio.ValueAt(baseTop[0], path, c.Frame); !ok || math.Abs(got-value) > 1e-9 {
						t.Errorf("%s: baseline %s is %v (ok %v), want %v", c.Name, path, got, ok, value)
					}
				}
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("2d case %q is missing", name)
		}
	}
	for group, count := range map[string]int{"key": 5, "mask": 8, "blend": 17} {
		if groups[group] != count {
			t.Errorf("group %s has %d cases, want %d", group, groups[group], count)
		}
	}
	listed := map[string]bool{}
	for _, c := range Cases(m) {
		listed[c.Name] = true
		// Every registered baseline validates and covers its frame.
		if c.Delta != nil {
			if err := videostudio.Validate(c.Delta.Baseline); err != nil || c.Frame >= videostudio.EnabledDuration(c.Delta.Baseline) {
				t.Errorf("%s: baseline invalid (%v) or shorter than frame %d", c.Name, err, c.Frame)
			}
		}
	}
	for name := range want {
		if !listed[name] {
			t.Errorf("Cases does not list %q: compositingCases is not registered", name)
		}
	}
	for _, name := range []string{MediaGreen, MediaLayer} {
		count := 0
		for _, item := range mediaSet {
			if item.name == name {
				count++
			}
		}
		if count != 1 {
			t.Errorf("media %s is registered %d times, want once", name, count)
		}
	}
	// The 2c delta cases (plan 2d Task 12, plan 2f Task 4): each baseline is the case without its effect, and no other 2c
	// case has a Delta. Since F2 the colour and LUT checks are normal and the native guards are never report-only;
	// sharpen_static and blur_logo_alpha stay report-only only under F2 outcome B (magnification, cases_2c.go).
	deltas := map[string]Delta{
		"color_temperature_tint":  DeltaBlend,
		"lut_keyframed_intensity": DeltaBlend,
		"sharpen_static":          reportOnly(DeltaMask),
		"blur_logo_alpha":         reportOnly(DeltaMask),
		"blur_logo_alpha_native":  DeltaMask,
		"sharpen_static_native":   DeltaMask,
	}
	for _, c := range pictureCases(m) {
		spec, has := deltas[c.Name]
		if !has {
			if c.Delta != nil {
				t.Errorf("%s: unexpected Delta (only the six 2c delta cases have one)", c.Name)
			}
			continue
		}
		if c.Delta == nil {
			t.Errorf("%s: the 2c delta case has no Delta", c.Name)
			continue
		}
		bounds := *c.Delta
		bounds.Baseline = videostudio.Project{}
		if !reflect.DeepEqual(bounds, spec) {
			t.Errorf("%s: delta bounds %+v, want %+v", c.Name, bounds, spec)
		}
		if err := videostudio.Validate(c.Delta.Baseline); err != nil {
			t.Errorf("%s: invalid baseline: %v", c.Name, err)
		}
		if _, has := pcSubject(Case{Name: c.Name, Project: c.Delta.Baseline}); has {
			t.Errorf("%s: the baseline keeps a crop, a flip or effects; it must be the case without its effect", c.Name)
		}
	}
}

// TestCompositingMediaGenerators writes the 2d media without FFmpeg and checks what the cases rely on. The greenscreen
// part evaluates colorkey's alpha (index §4.22, real form) with chroma_key's registry defaults at probe points: the
// backdrop is keyed, the subject stays, and the graded patches span the default ramp. Its log line is the record behind
// Task 12's decision on the provisional defaults (index §4.23).
func TestCompositingMediaGenerators(t *testing.T) {
	dir, ctx := t.TempDir(), context.Background()
	for _, generate := range []mediaGenerator{greenMedia, layerMedia} {
		if _, err := generate(ctx, "", dir); err != nil {
			t.Fatal(err)
		}
	}
	green, layer := pcDecodePNG(t, filepath.Join(dir, "greenscreen.png")), pcDecodePNG(t, filepath.Join(dir, "layer.png"))
	if green.Bounds() != image.Rect(0, 0, 640, 360) || layer.Bounds() != image.Rect(0, 0, 640, 360) {
		t.Fatalf("greenscreen %v, layer %v, want 640x360", green.Bounds(), layer.Bounds())
	}
	numbers, colour := cdKeyDefaults(t)
	if colour != "#00b140" {
		t.Errorf("the greenscreen is built around #00b140, but the default key colour is %s (Task 12 keeps the colour)", colour)
	}
	key := cdHex(t, colour)
	rgb := func(x, y int) [3]uint8 { p := green.NRGBAAt(x, y); return [3]uint8{p.R, p.G, p.B} }
	alpha := func(x, y int) float64 {
		return cdKeyAlpha(rgb(x, y), key, numbers["similarity"], numbers["smoothness"])
	}
	for _, p := range [][2]int{{20, 20}, {320, 20}, {100, 200}, {600, 340}} {
		if a := green.NRGBAAt(p[0], p[1]).A; a != 255 || alpha(p[0], p[1]) != 0 {
			t.Errorf("backdrop %v: alpha %d, key alpha %.3f at the defaults, want opaque and keyed (distance %.3f)", p, a, alpha(p[0], p[1]), cdKeyDistance(rgb(p[0], p[1]), key))
		}
	}
	for name, p := range map[string][2]int{"skin": {320, 100}, "red": {268, 132}, "blue": {372, 132}, "shirt": {320, 250}} {
		if a := alpha(p[0], p[1]); a != 1 || green.NRGBAAt(p[0], p[1]).A != 255 {
			t.Errorf("subject %s %v: key alpha %.3f at the defaults, want an opaque, unkeyed pixel (distance %.3f)", name, p, a, cdKeyDistance(rgb(p[0], p[1]), key))
		}
	}
	// Green spill: inside the left subject edge, and over the whole shirt as a cast that grows to the right.
	spill := func(x, y int) int { c := rgb(x, y); return int(c[1]) - (int(c[0])+int(c[2]))/2 }
	if s := spill(175, 176); s < 20 {
		t.Errorf("the subject edge at (175, 176) has spill %d, want at least 20 (despill must have something to remove)", s)
	}
	if left, right := spill(230, 250), spill(410, 250); left < 20 || right < left+15 {
		t.Errorf("shirt spill %d (left) and %d (right), want at least 20 growing by 15 or more", left, right)
	}
	// The orange corner keeps its colour under the key; its own alpha falls to about 0.15 at the corner.
	if a := green.NRGBAAt(630, 6).A; a > 80 || alpha(630, 6) != 1 {
		t.Errorf("corner (630, 6): alpha %d, key alpha %.3f, want alpha at most 80 and unkeyed", a, alpha(630, 6))
	}
	if a := green.NRGBAAt(600, 40).A; a < 140 || a > 200 {
		t.Errorf("corner (600, 40): alpha %d, want 140..200", a)
	}
	var alphas []float64
	for i, d := range greenPatchDistances {
		x := 58 + 96*i
		if got := cdKeyDistance(rgb(x, 330), key); math.Abs(got-d) > 0.004 {
			t.Errorf("patch %d at (%d, 330): distance %.4f, want %.2f", i, x, got, d)
		}
		alphas = append(alphas, alpha(x, 330))
	}
	if alphas[0] != 0 || alphas[len(alphas)-1] != 1 || !slices.ContainsFunc(alphas, func(a float64) bool { return a > 0 && a < 1 }) {
		t.Errorf("graded patches key %v at the defaults, want keyed, partial and opaque patches", alphas)
	}
	t.Logf("chroma_key defaults (%s, similarity %.3f, smoothness %.3f, spill %.2f) on greenscreen.png: backdrop keyed, subject opaque, graded patch alphas %.2f",
		colour, numbers["similarity"], numbers["smoothness"], numbers["spill"], alphas)
	// layer.png: above the pastel band every channel runs from 0 to 255 and crosses 128, at full alpha.
	lowest, highest, below, above := [3]int{255, 255, 255}, [3]int{}, [3]int{}, [3]int{}
	for y := 0; y < 228; y++ {
		for x := 0; x < 640; x++ {
			p := layer.NRGBAAt(x, y)
			if p.A != 255 {
				t.Fatalf("layer (%d, %d) alpha %d, want opaque above the lower third", x, y, p.A)
			}
			for i, v := range [3]uint8{p.R, p.G, p.B} {
				lowest[i], highest[i] = min(lowest[i], int(v)), max(highest[i], int(v))
				if v < 128 {
					below[i]++
				} else {
					above[i]++
				}
			}
		}
	}
	for i := range lowest {
		if lowest[i] != 0 || highest[i] != 255 || below[i] == 0 || above[i] == 0 {
			t.Errorf("layer channel %d spans %d..%d (%d below 128, %d above), want 0..255 on both sides of 128", i, lowest[i], highest[i], below[i], above[i])
		}
	}
	// The lower third: alpha falls from 1 to 0.2.
	if a := layer.NRGBAAt(320, 239).A; a != 255 {
		t.Errorf("layer row 239 alpha %d, want 255", a)
	}
	previous := 256
	for y := 240; y < 360; y += 8 {
		a := int(layer.NRGBAAt(320, y).A)
		if a >= previous {
			t.Errorf("layer alpha at row %d is %d after %d, want a falling ramp", y, a, previous)
		}
		previous = a
	}
	if a := layer.NRGBAAt(320, 359).A; a < 50 || a > 54 {
		t.Errorf("layer row 359 alpha %d, want about 52 (0.2)", a)
	}
}
