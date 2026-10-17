package goldenframe

import (
	"context"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// pcSubject returns the clip a 2c case tests: the first clip with a crop, a flip or effects. A background is plain.
func pcSubject(c Case) (videostudio.Clip, bool) {
	for _, track := range c.Project.Tracks {
		for _, clip := range track.Clips {
			if clip.Crop != nil || clip.FlipH || clip.FlipV || len(clip.Effects) > 0 {
				return clip, true
			}
		}
	}
	return videostudio.Clip{}, false
}

// pcEffectPath splits "effects.<i>.<param>" into the entry index and the param name.
func pcEffectPath(path string) (int, string, bool) {
	parts := strings.Split(path, ".")
	if len(parts) != 3 || parts[0] != "effects" {
		return 0, "", false
	}
	index, err := strconv.Atoi(parts[1])
	return index, parts[2], err == nil && index >= 0
}

// pcPathKeys returns the keyframes of a crop path or of an effect param.
func pcPathKeys(clip videostudio.Clip, path string) []videostudio.Keyframe {
	if index, param, ok := pcEffectPath(path); ok {
		if index >= len(clip.Effects) {
			return nil
		}
		return clip.Effects[index].Keyframes[param]
	}
	return clip.Keyframes[path]
}

// pcStatic is a path's static value: what a painter that ignored the keyframes would use.
func pcStatic(clip videostudio.Clip, path string) float64 {
	if index, param, ok := pcEffectPath(path); ok {
		if index < len(clip.Effects) {
			if value, isNumber := clip.Effects[index].Params[param].(float64); isNumber {
				return value
			}
		}
		return math.NaN()
	}
	if clip.Crop == nil {
		return 0
	}
	switch path {
	case "crop.l":
		return clip.Crop.L
	case "crop.t":
		return clip.Crop.T
	case "crop.r":
		return clip.Crop.R
	case "crop.b":
		return clip.Crop.B
	}
	return math.NaN()
}

func TestPictureCasesCoverGroups(t *testing.T) {
	gl, both := []string{PainterGL}, []string{PainterGL, Painter2D}
	type expect struct {
		threshold    float64
		painters     []string
		effects      []string           // effect ids in stack order
		values       map[string]float64 // at the case frame, through videostudio.ValueAt (cases_2c.go comments)
		keyed        []string           // keyframed paths: the static value must differ from the keyed one
		lut          string             // the lut entry's ref
		flipH, flipV bool
	}
	color := []string{"color"}
	want := map[string]expect{
		"color_exposure_contrast": {threshold: ThresholdColour, painters: gl, effects: color, values: map[string]float64{
			"effects.0.exposure": 0.4, "effects.0.contrast": 1.25, "effects.0.saturation": 1, "effects.0.temperature": 6500}},
		"color_temperature_tint": {threshold: ThresholdColour, painters: gl, effects: color, values: map[string]float64{
			"effects.0.temperature": 8500, "effects.0.tint": -0.2, "effects.0.exposure": 0}},
		"color_saturation": {threshold: ThresholdColour, painters: gl, effects: color, values: map[string]float64{
			"effects.0.saturation": 1.5, "effects.0.contrast": 1}},
		"color_wheels": {threshold: ThresholdColour, painters: gl, effects: color, values: map[string]float64{
			"effects.0.lift_x": 0.5, "effects.0.lift_y": -0.3, "effects.0.lift": 0.03, "effects.0.gamma_x": -0.4, "effects.0.gamma_y": 0.4,
			"effects.0.gamma": 1.1, "effects.0.gain_x": 0.3, "effects.0.gain_y": 0.5, "effects.0.gain": 0.95}},
		"color_video_gradient": {threshold: ThresholdColour, painters: gl, effects: color, values: map[string]float64{
			"effects.0.exposure": -0.3, "effects.0.contrast": 1.15, "effects.0.saturation": 1.2, "effects.0.temperature": 7200}},
		"color_keyframed_exposure": {threshold: ThresholdColour, painters: gl, effects: color,
			values: map[string]float64{"effects.0.exposure": 0.875}, keyed: []string{"effects.0.exposure"}},
		"color_stack_order": {threshold: ThresholdDetail, painters: gl, effects: []string{"color", "lut", "vignette"}, lut: MediaCube, values: map[string]float64{
			"effects.0.exposure": 0.3, "effects.0.saturation": 1.3, "effects.1.intensity": 1, "effects.2.amount": 0.6, "effects.2.size": 1}},
		"lut_builtin_teal_orange": {threshold: ThresholdColour, painters: gl, effects: []string{"lut"}, lut: "builtin:teal_orange",
			values: map[string]float64{"effects.0.intensity": 1}},
		"lut_upload_intensity": {threshold: ThresholdColour, painters: gl, effects: []string{"lut"}, lut: MediaCube,
			values: map[string]float64{"effects.0.intensity": 0.6}},
		"lut_keyframed_intensity": {threshold: ThresholdColour, painters: gl, effects: []string{"lut"}, lut: MediaCube,
			values: map[string]float64{"effects.0.intensity": 0.6}, keyed: []string{"effects.0.intensity"}},
		"crop_static_contain": {threshold: ThresholdGeometry, painters: both, values: map[string]float64{
			"crop.l": 0.1, "crop.t": 0.05, "crop.r": 0.2, "crop.b": 0.15}},
		"crop_static_cover": {threshold: ThresholdGeometry, painters: both, values: map[string]float64{
			"crop.l": 0, "crop.t": 0.2, "crop.r": 0, "crop.b": 0.3}},
		"crop_keyframed": {threshold: ThresholdGeometry, painters: both, keyed: []string{"crop.l", "crop.b"}, values: map[string]float64{
			"crop.l": 0.18, "crop.t": 0.1, "crop.r": 0.1, "crop.b": 0.12}},
		"flip_h": {threshold: ThresholdGeometry, painters: both, flipH: true},
		"flip_v_crop": {threshold: ThresholdGeometry, painters: both, flipV: true, values: map[string]float64{
			"crop.l": 0.05, "crop.t": 0.3, "crop.r": 0.25, "crop.b": 0}},
		"blur_static":     {threshold: ThresholdDetail, painters: gl, effects: []string{"blur"}, values: map[string]float64{"effects.0.radius": 0.01}},
		"blur_logo_alpha": {threshold: ThresholdDetail, painters: gl, effects: []string{"blur"}, values: map[string]float64{"effects.0.radius": 0.02}},
		"sharpen_static":  {threshold: ThresholdDetail, painters: gl, effects: []string{"sharpen"}, values: map[string]float64{"effects.0.strength": 0.8}},
		"sharpen_keyframed": {threshold: ThresholdDetail, painters: gl, effects: []string{"sharpen"},
			values: map[string]float64{"effects.0.strength": 0.75}, keyed: []string{"effects.0.strength"}},
		"vignette_static": {threshold: ThresholdDetail, painters: gl, effects: []string{"vignette"},
			values: map[string]float64{"effects.0.amount": 0.7, "effects.0.size": 0.8}},
		"vignette_logo_alpha": {threshold: ThresholdDetail, painters: gl, effects: []string{"vignette"},
			values: map[string]float64{"effects.0.amount": 0.8, "effects.0.size": 0.6}},
		"vignette_keyframed": {threshold: ThresholdDetail, painters: gl, effects: []string{"vignette"},
			values: map[string]float64{"effects.0.amount": 0.7, "effects.0.size": 1}, keyed: []string{"effects.0.amount"}},
	}
	m := fakeMedia()
	seen, groups := map[string]bool{}, map[string]int{}
	for _, c := range pictureCases(m) {
		spec, known := want[c.Name]
		if !known {
			t.Errorf("unexpected 2c case %q", c.Name)
			continue
		}
		if seen[c.Name] {
			t.Errorf("duplicate 2c case %q", c.Name)
		}
		seen[c.Name] = true
		group, _, _ := strings.Cut(c.Name, "_")
		groups[group]++
		if c.Threshold != spec.threshold {
			t.Errorf("%s: threshold %.2f, want %.2f", c.Name, c.Threshold, spec.threshold)
		}
		if !slices.Equal(c.Painters, spec.painters) {
			t.Errorf("%s: painters %v, want %v", c.Name, c.Painters, spec.painters)
		}
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		clip, ok := pcSubject(c)
		if !ok {
			t.Errorf("%s: no clip with a crop, a flip or effects", c.Name)
			continue
		}
		if c.Frame < clip.Start || c.Frame >= clip.Start+clip.Duration || c.Frame >= videostudio.EnabledDuration(c.Project) {
			t.Errorf("%s: frame %d lies outside the tested clip [%d, %d)", c.Name, c.Frame, clip.Start, clip.Start+clip.Duration)
		}
		// Effect cases are GL-only; crop and flip cases carry no effects and run on both painters.
		ids := make([]string, 0, len(clip.Effects))
		for _, entry := range clip.Effects {
			ids = append(ids, entry.ID)
			registry, found := videostudio.EffectByID(entry.ID)
			if !found || !entry.On {
				t.Errorf("%s: effect %q is unknown or off", c.Name, entry.ID)
				continue
			}
			for _, param := range registry.Params {
				if _, present := entry.Params[param.ID]; !present {
					t.Errorf("%s: effect %s lacks param %s (case entries carry complete params)", c.Name, entry.ID, param.ID)
				}
			}
		}
		if !slices.Equal(ids, spec.effects) {
			t.Errorf("%s: effects %v, want %v in this order", c.Name, ids, spec.effects)
		}
		if (group == "crop" || group == "flip") != (len(ids) == 0) {
			t.Errorf("%s: crop and flip cases carry no effects, every other case does", c.Name)
		}
		if len(ids) > 0 && !slices.Equal(c.Painters, gl) {
			t.Errorf("%s: an effect case must be GL-only (the 2D painter never draws effects), painters %v", c.Name, c.Painters)
		}
		if clip.FlipH != spec.flipH || clip.FlipV != spec.flipV {
			t.Errorf("%s: flips h %v v %v, want h %v v %v", c.Name, clip.FlipH, clip.FlipV, spec.flipH, spec.flipV)
		}
		for path, value := range spec.values {
			got, ok := videostudio.ValueAt(clip, path, c.Frame)
			if !ok || math.Abs(got-value) > 1e-9 {
				t.Errorf("%s: %s at frame %d is %v (ok %v), want %v", c.Name, path, c.Frame, got, ok, value)
			}
		}
		for _, path := range spec.keyed {
			if len(pcPathKeys(clip, path)) < 2 {
				t.Errorf("%s: %s has no keyframes", c.Name, path)
			}
			if math.Abs(pcStatic(clip, path)-spec.values[path]) < 0.05 {
				t.Errorf("%s: the static %s %v is too close to the keyed %v, so ignored keyframes would pass", c.Name, path, pcStatic(clip, path), spec.values[path])
			}
		}
		if spec.lut != "" {
			ref := ""
			for _, entry := range clip.Effects {
				if entry.ID == "lut" {
					ref, _ = entry.Params["lut"].(string)
				}
			}
			builtin, assetID, ok := videostudio.ParseLUTRef(ref)
			switch {
			case ref != spec.lut || !ok:
				t.Errorf("%s: lut ref %q (parsed %v), want %q", c.Name, ref, ok, spec.lut)
			case builtin != "":
				if _, found := videostudio.LookByID(builtin); !found {
					t.Errorf("%s: %q is not a registry look", c.Name, builtin)
				}
			default:
				found := false
				for _, asset := range c.Project.Assets {
					found = found || (asset.ID == assetID && asset.Kind == videostudio.AssetLUT)
				}
				if !found {
					t.Errorf("%s: the project has no lut asset %q", c.Name, assetID)
				}
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("2c case %q is missing", name)
		}
	}
	for group, count := range map[string]int{"color": 7, "lut": 3, "crop": 3, "flip": 2, "blur": 2, "sharpen": 2, "vignette": 3} {
		if groups[group] != count {
			t.Errorf("group %s has %d cases, want %d", group, groups[group], count)
		}
	}
	registered := map[string]bool{}
	for _, c := range Cases(m) {
		registered[c.Name] = true
	}
	for name := range want {
		if !registered[name] {
			t.Errorf("Cases does not list %q: pictureCases is not registered", name)
		}
	}
	for _, name := range []string{MediaChart, MediaDetail, MediaCube} {
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
}

// pcDecodePNG reads a generated PNG as NRGBA.
func pcDecodePNG(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", filepath.Base(path), err)
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

// TestPictureMediaGenerators writes the 2c media without FFmpeg and checks what the cases rely on:
// - the chart is opaque, and its first band is an exact grey ramp;
// - the detail chart is luma-only with enough contrast;
// - cube.cube parses as a 17³ table of cubeGrade in red-fastest order with the default domain;
// - InspectLUTFile records cube.cube as a lut asset.
func TestPictureMediaGenerators(t *testing.T) {
	dir, ctx := t.TempDir(), context.Background()
	for _, generate := range []mediaGenerator{chartMedia, detailMedia, cubeMedia} {
		if _, err := generate(ctx, "", dir); err != nil {
			t.Fatal(err)
		}
	}
	chart, detail := pcDecodePNG(t, filepath.Join(dir, "chart.png")), pcDecodePNG(t, filepath.Join(dir, "detail.png"))
	if chart.Bounds() != image.Rect(0, 0, 640, 360) || detail.Bounds() != image.Rect(0, 0, 640, 360) {
		t.Fatalf("chart %v, detail %v, want 640x360", chart.Bounds(), detail.Bounds())
	}
	low, high := 255, 0
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			if pixel := chart.NRGBAAt(x, y); pixel.A != 255 {
				t.Fatalf("chart (%d,%d) alpha %d, want opaque", x, y, pixel.A)
			}
			pixel := detail.NRGBAAt(x, y)
			if pixel.A != 255 || pixel.R != pixel.G || pixel.G != pixel.B {
				t.Fatalf("detail (%d,%d) = %v, want opaque grey (luma only)", x, y, pixel)
			}
			low, high = min(low, int(pixel.G)), max(high, int(pixel.G))
		}
	}
	if low > 64 || high < 191 {
		t.Errorf("detail spans %d..%d, want about 51..204", low, high)
	}
	// Row 20 is the middle of the grey band: equal channels that rise from left to right.
	previous := -1
	for x := 0; x < 640; x += 32 {
		pixel := chart.NRGBAAt(x, 20)
		if pixel.R != pixel.G || pixel.G != pixel.B || int(pixel.R) <= previous {
			t.Fatalf("chart grey band at x %d = %v after %d, want a rising grey ramp", x, pixel, previous)
		}
		previous = int(pixel.R)
	}
	path := filepath.Join(dir, "cube.cube")
	lut, err := videostudio.LoadLUTFile(path)
	if err != nil {
		t.Fatalf("LoadLUTFile(cube.cube): %v", err)
	}
	if lut.Size != 17 || lut.DomainMin != [3]float64{0, 0, 0} || lut.DomainMax != [3]float64{1, 1, 1} {
		t.Fatalf("cube.cube parsed as size %d, domain %v..%v; want 17 and [0,1]", lut.Size, lut.DomainMin, lut.DomainMax)
	}
	// Grid points (k/16) sample the written values exactly; (1, 0, 0) tells red-fastest from blue-fastest order.
	for _, point := range [][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 0.5, 1}, {0.25, 0.75, 0.5}} {
		got, grade := lut.Sample(point), cubeGrade(point[0], point[1], point[2])
		for channel := range got {
			if math.Abs(got[channel]-grade[channel]) > 1e-6 {
				t.Errorf("cube.cube at %v = %v, want cubeGrade %v", point, got, grade)
				break
			}
		}
	}
	asset, err := videostudio.InspectLUTFile(path)
	info, statErr := os.Stat(path)
	if err != nil || statErr != nil || asset.Kind != videostudio.AssetLUT || asset.LUTSize != 17 || asset.SizeBytes != info.Size() {
		t.Errorf("InspectLUTFile(cube.cube) = %+v, %v (stat %v), want kind lut, lut_size 17 and the file size", asset, err, statErr)
	}
}
