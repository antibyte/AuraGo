package goldenframe

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/videostudio"
)

// Colour and picture goldens (plan 2c, Task 12; index §4.12, §4.20).
//
// Media:
//   - Colour and LUT cases use the PNG chart (chart.png); detail effects use the PNG detail chart (detail.png). No
//     yuv420p-to-RGB conversion enters these comparisons.
//   - Plan 2b measured that chroma-heavy video loses about 0.03 SSIM, because ffmpeg's scaler and Chrome's decoder place
//     the chroma samples about 0.4 px apart. detail.png is luma-only for the same reason.
//   - One colour case (color_video_gradient), the cover crop (portrait) and the keyframed vignette (gradient) use the
//     smooth generated videos. Their low spatial frequency hides that offset; the foundation cases pass at 0.98 on them.
//   - Every band and patch edge of the charts is soft, so the export's bicubic and the preview's bilinear upscale agree.
//
// Painters: effects run on the GL compositor only, because the 2D painter never draws effects (index §4.8). Crop and
// flip cases run on both painters.
//
// Thresholds (index §4.12):
// - colour and LUT: ThresholdColour;
// - crop and flip: ThresholdGeometry;
// - blur, sharpen and vignette: ThresholdDetail.
// color_stack_order uses ThresholdDetail because its stack ends in a vignette. A case with another threshold states the
// measured reason on the line above it.
//
// Values: each case comment gives the parameter values at the case frame, and cases_2c_test.go pins them through
// videostudio.ValueAt. Keyframes sit on clips that start at frame 0 with offset 0, so a source frame is the timeline
// frame. Effect params are complete: the registry defaults plus the case's values.
//
// Delta checks (plan 2d Task 12, plan 2f Task 4; index §4.12, §4.26.9, risk 21). SSIM barely separates these cases from
// a missing effect (plan 2c), so they also compare the change their effect makes against a baseline without it:
//   - color_temperature_tint and lut_keyframed_intensity take DeltaBlend's bounds (smooth per-pixel changes);
//   - sharpen_static and blur_logo_alpha take DeltaMask's, because their change sits at edges. They stay report-only:
//     the preview magnifies the effect's result bilinearly where the export scales bicubically (F2 outcome B; the
//     measured slopes are above each case);
//   - the native guards blur_logo_alpha_native and sharpen_static_native draw the picture 1:1 in four tiles, so they
//     measure the pass without magnification, with DeltaMask's bounds, never report-only.
//
// cases_2c.go sorts between cases_2b.go and cases_foundation.go, so these cases register second. The order has no
// meaning.
func init() {
	registerCases(pictureCases)
	registerMedia(MediaChart, chartMedia)
	registerMedia(MediaDetail, detailMedia)
	registerMedia(MediaCube, cubeMedia)
}

// Names (asset ids) of the 2c media.
const (
	MediaChart  = "chart"  // chart.png: 640x360 opaque colour chart (chartPixel)
	MediaDetail = "detail" // detail.png: 640x360 opaque luma-only gratings and soft edges (detailPixel)
	MediaCube   = "cube"   // cube.cube: 17³ LUT of cubeGrade, asset kind lut (cubeMedia)
)

var (
	chartMedia  = pngFixture("chart.png", 640, 360, chartPixel)
	detailMedia = pngFixture("detail.png", 640, 360, detailPixel)
)

// The chart has chartBands bands of 40 rows. Neighbouring bands blend over chartSoft of a band (6 rows) on either side
// of their border.
const (
	chartBands = 9
	chartSoft  = 0.15
)

func pcSmooth(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return x * x * (3 - 2*x)
}

func pcMix(a, b [3]float64, w float64) [3]float64 {
	return [3]float64{a[0] + (b[0]-a[0])*w, a[1] + (b[1]-a[1])*w, a[2] + (b[2]-a[2])*w}
}

func pcByte(v float64) uint8 { return uint8(math.Round(255 * math.Max(0, math.Min(1, v)))) }

// chartBand is the colour of chart band i at horizontal position u (0..1) and position f (0..1) inside the band.
// - Band 0 is a grey ramp.
// - Bands 1-6 are red, green, blue, cyan, magenta and yellow ramps: 0.05 + 0.9 u on, 0.05 off.
// - Band 7 holds four skin tones.
// - Band 8 is a soft colour field.
func chartBand(i int, u, f float64) [3]float64 {
	on := 0.05 + 0.9*u
	switch i {
	case 0:
		return [3]float64{u, u, u}
	case 1:
		return [3]float64{on, 0.05, 0.05}
	case 2:
		return [3]float64{0.05, on, 0.05}
	case 3:
		return [3]float64{0.05, 0.05, on}
	case 4:
		return [3]float64{0.05, on, on}
	case 5:
		return [3]float64{on, 0.05, on}
	case 6:
		return [3]float64{on, on, 0.05}
	case 7:
		// Four skin tones, light to dark, 160 px each, blended over about 10 px on either side of a border.
		tones := [4][3]float64{{0.93, 0.78, 0.67}, {0.82, 0.62, 0.49}, {0.62, 0.43, 0.32}, {0.40, 0.27, 0.20}}
		p := u * 4
		k := min(3, int(p))
		d := p - float64(k)
		switch {
		case d < 0.06 && k > 0:
			return pcMix(tones[k-1], tones[k], pcSmooth((d+0.06)/0.12))
		case d > 0.94 && k < 3:
			return pcMix(tones[k], tones[k+1], pcSmooth((d-0.94)/0.12))
		}
		return tones[k]
	default:
		return [3]float64{0.5 + 0.3*math.Sin(2*math.Pi*u+0.5), 0.5 + 0.3*math.Sin(2*math.Pi*(0.6*u+0.5*f)+2), 0.5 + 0.3*math.Cos(math.Pi*u+f)}
	}
}

// chartPixel draws the opaque colour chart: chartBand per 40-row band, with soft borders between bands.
func chartPixel(x, y int) color.NRGBA {
	u := (float64(x) + 0.5) / 640
	t := (float64(y) + 0.5) / 40
	i := int(t)
	f := t - float64(i)
	c := chartBand(i, u, f)
	switch {
	case f < chartSoft && i > 0:
		c = pcMix(chartBand(i-1, u, 1), c, pcSmooth((f+chartSoft)/(2*chartSoft)))
	case f > 1-chartSoft && i < chartBands-1:
		c = pcMix(c, chartBand(i+1, u, 0), pcSmooth((f-(1-chartSoft))/(2*chartSoft)))
	}
	return color.NRGBA{R: pcByte(c[0]), G: pcByte(c[1]), B: pcByte(c[2]), A: 255}
}

// detailPixel draws opaque luma-only detail (r = g = b), blended across x 370..398:
// - left: vertical sine gratings around 0.5 with amplitude 0.3, period 16 px at the top to 40 px at the bottom;
// - right: a checkerboard of 64 px cells (0.25 / 0.75) whose edges are softened to about 3 px.
func detailPixel(x, y int) color.NRGBA {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	grating := 0.5 + 0.3*math.Sin(2*math.Pi*fx/(16+24*fy/360))
	square := func(v float64) float64 { return math.Tanh(10 * math.Sin(math.Pi*v/64)) }
	blocks := 0.5 + 0.25*square(fx-384)*square(fy)
	v := pcByte(grating + (blocks-grating)*pcSmooth((fx-370)/28))
	return color.NRGBA{R: v, G: v, B: v, A: 255}
}

// cubeGrade is the grade that cube.cube samples: a smooth cross-process that lifts and cools the shadows and warms the
// highlights, with channel mixing, so trilinear interpolation and the channel order both show.
func cubeGrade(r, g, b float64) [3]float64 {
	y := 0.2126*r + 0.7152*g + 0.0722*b
	return [3]float64{
		math.Min(1, 0.06+0.9*r+0.08*y),
		math.Min(1, 0.03+0.94*math.Pow(g, 0.92)),
		math.Min(1, 0.12+0.7*b+0.1*(1-y)),
	}
}

// cubeMedia writes cube.cube: cubeGrade on a 17³ grid, red fastest, with a comment, a TITLE and explicit
// DOMAIN_MIN/DOMAIN_MAX lines of the default domain, numbers with %.6f. MakeMedia inspects it as a lut asset.
func cubeMedia(_ context.Context, _, dir string) (string, error) {
	const file, grid = "cube.cube", 17
	var text strings.Builder
	fmt.Fprintf(&text, "# Video Studio golden grade (plan 2c)\nTITLE \"golden grade\"\nLUT_3D_SIZE %d\nDOMAIN_MIN 0 0 0\nDOMAIN_MAX 1 1 1\n", grid)
	for bi := 0; bi < grid; bi++ {
		for gi := 0; gi < grid; gi++ {
			for ri := 0; ri < grid; ri++ {
				out := cubeGrade(float64(ri)/(grid-1), float64(gi)/(grid-1), float64(bi)/(grid-1))
				fmt.Fprintf(&text, "%.6f %.6f %.6f\n", out[0], out[1], out[2])
			}
		}
	}
	return file, os.WriteFile(filepath.Join(dir, file), []byte(text.String()), 0o644)
}

// pcEdit changes one property of a clip built by pcClip.
type pcEdit func(*videostudio.Clip)

// pcClip is a clip of assetID at timeline frames [start, start+duration) with NewClip's defaults and the given edits.
func pcClip(id, assetID string, start, duration int, edits ...pcEdit) videostudio.Clip {
	clip := NewClip(id, assetID, start, duration)
	for _, edit := range edits {
		edit(&clip)
	}
	return clip
}

func pcBox(x, y, width, height float64) pcEdit {
	return func(clip *videostudio.Clip) { clip.X, clip.Y, clip.Width, clip.Height = x, y, width, height }
}

func pcCover(clip *videostudio.Clip) { clip.Fit = videostudio.FitCover }

func pcCrop(l, t, r, b float64) pcEdit {
	return func(clip *videostudio.Clip) { clip.Crop = &videostudio.Crop{L: l, T: t, R: r, B: b} }
}

func pcFlipH(clip *videostudio.Clip) { clip.FlipH = true }

func pcFlipV(clip *videostudio.Clip) { clip.FlipV = true }

// pcKeys sets the keyframes of a base path (crop.l|t|r|b).
func pcKeys(path string, keys ...videostudio.Keyframe) pcEdit {
	return func(clip *videostudio.Clip) {
		if clip.Keyframes == nil {
			clip.Keyframes = map[string][]videostudio.Keyframe{}
		}
		clip.Keyframes[path] = keys
	}
}

// pcEffect appends an enabled effect entry with complete params: the registry defaults, overridden by params. Number
// values must be float64 literals (write 8500.0, not 8500).
func pcEffect(id string, params map[string]any) pcEdit {
	return func(clip *videostudio.Clip) {
		entry := videostudio.EffectInstance{ID: id, On: true, Params: map[string]any{}}
		if spec, ok := videostudio.EffectByID(id); ok {
			for _, param := range spec.Params {
				entry.Params[param.ID] = param.Default
			}
		}
		for key, value := range params {
			entry.Params[key] = value
		}
		clip.Effects = append(clip.Effects, entry)
	}
}

// pcEffectKeys sets the keyframes of one param of the clip's effect entry index (added by pcEffect before).
func pcEffectKeys(index int, param string, keys ...videostudio.Keyframe) pcEdit {
	return func(clip *videostudio.Clip) {
		entry := &clip.Effects[index]
		if entry.Keyframes == nil {
			entry.Keyframes = map[string][]videostudio.Keyframe{}
		}
		entry.Keyframes[param] = keys
	}
}

func pcKey(f int, v float64) videostudio.Keyframe {
	return videostudio.Keyframe{F: f, V: v, E: videostudio.EaseLinear}
}

func pcCase(m Media, name string, frame int, threshold float64, painters []string, tracks ...videostudio.Track) Case {
	return Case{Name: name, Project: m.Project(name, tracks...), Frame: frame, Threshold: threshold, Painters: append([]string(nil), painters...)}
}

// pcQuadrants are the four 640x360 boxes of the 1280x720 canvas (x, y): a 640x360 picture sits 1:1 in each.
var pcQuadrants = [4][2]float64{{0, 0}, {0.5, 0}, {0, 0.5}, {0.5, 0.5}}

// pcLogoTiles draws the logo 1:1 in each quadrant, each tile on its own overlay track over the full-frame chart; the
// edits apply to every tile (plan 2f Task 4). Drawn 1:1 the preview applies an effect pass without magnifying its
// result, and four tiles change four times the samples of one.
func pcLogoTiles(edits ...pcEdit) []videostudio.Track {
	tracks := []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaChart, 0, 40))}
	for i, q := range pcQuadrants {
		tile := append([]pcEdit{pcBox(q[0], q[1], 0.5, 0.5)}, edits...)
		tracks = append(tracks, OverlayTrack(fmt.Sprintf("o%d", i+1), pcClip(fmt.Sprintf("l%d", i+1), MediaLogo, 0, 40, tile...)))
	}
	return tracks
}

// pcDetailTiles draws detail.png 1:1 in each quadrant, each tile on its own video track; the edits apply to every tile.
func pcDetailTiles(edits ...pcEdit) []videostudio.Track {
	var tracks []videostudio.Track
	for i, q := range pcQuadrants {
		tile := append([]pcEdit{pcBox(q[0], q[1], 0.5, 0.5)}, edits...)
		tracks = append(tracks, VideoTrack(fmt.Sprintf("v%d", i+1), pcClip(fmt.Sprintf("d%d", i+1), MediaDetail, 0, 40, tile...)))
	}
	return tracks
}

func pictureCases(m Media) []Case {
	gl, both := []string{PainterGL}, []string{PainterGL, Painter2D}
	// The full-frame chart and detail clips (640x360 into the 1280x720 project, contained: exactly 2x).
	chart := func(edits ...pcEdit) videostudio.Track {
		return VideoTrack("v1", pcClip("c", MediaChart, 0, 40, edits...))
	}
	detail := func(edits ...pcEdit) videostudio.Track {
		return VideoTrack("v1", pcClip("d", MediaDetail, 0, 40, edits...))
	}
	// logoOver puts the soft-edged logo on an overlay track over the full-frame chart; the effect runs on the logo.
	logoOver := func(edits ...pcEdit) []videostudio.Track {
		return []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaChart, 0, 40)), OverlayTrack("o1", pcClip("l", MediaLogo, 0, 40, edits...))}
	}
	gradient := func(edits ...pcEdit) videostudio.Track {
		return VideoTrack("v1", pcClip("g", MediaGradient, 0, 60, edits...))
	}
	return []Case{
		// exposure +0.4 EV (x1.32), contrast 1.25 about 0.5: the ramps' bright ends clip on both sides.
		pcCase(m, "color_exposure_contrast", 5, ThresholdColour, gl, chart(pcEffect("color", map[string]any{"exposure": 0.4, "contrast": 1.25}))),
		// 8500 K warms (camera convention), tint -0.2 adds green.
		withDelta(pcCase(m, "color_temperature_tint", 5, ThresholdColour, gl, chart(pcEffect("color", map[string]any{"temperature": 8500.0, "tint": -0.2}))),
			DeltaBlend, m.Project("color_temperature_tint_baseline", chart())),
		// saturation 1.5 about BT.709 luma: the colour ramps clip, the grey ramp and the skin patches show the luma weights.
		pcCase(m, "color_saturation", 5, ThresholdColour, gl, chart(pcEffect("color", map[string]any{"saturation": 1.5}))),
		// lift (0.5, -0.3) + 0.03, gamma (-0.4, 0.4) x 1.1, gain (0.3, 0.5) x 0.95, as x/y wheels with k = 0.2.
		pcCase(m, "color_wheels", 5, ThresholdColour, gl, chart(pcEffect("color", map[string]any{"lift_x": 0.5, "lift_y": -0.3, "lift": 0.03,
			"gamma_x": -0.4, "gamma_y": 0.4, "gamma": 1.1, "gain_x": 0.3, "gain_y": 0.5, "gain": 0.95}))),
		// The only colour case through yuv420p: the smooth gradient video at frame 30 (exposure -0.3, contrast 1.15,
		// saturation 1.2, 7200 K).
		pcCase(m, "color_video_gradient", 30, ThresholdColour, gl, gradient(pcEffect("color", map[string]any{"exposure": -0.3, "contrast": 1.15,
			"saturation": 1.2, "temperature": 7200.0}))),
		// exposure keys -1 at source frame 0 to 1.5 at 20 (linear): frame 15 is 0.875 EV (the static exposure is 0).
		pcCase(m, "color_keyframed_exposure", 15, ThresholdColour, gl, chart(pcEffect("color", nil), pcEffectKeys(0, "exposure", pcKey(0, -1), pcKey(20, 1.5)))),
		// Threshold ThresholdDetail, not ThresholdColour: the stack (color -> lut -> vignette) ends in a vignette (index
		// §4.12). exposure 0.3, saturation 1.3; the uploaded cube at intensity 1; vignette amount 0.6, size 1.
		pcCase(m, "color_stack_order", 5, ThresholdDetail, gl, chart(pcEffect("color", map[string]any{"exposure": 0.3, "saturation": 1.3}),
			pcEffect("lut", map[string]any{"lut": MediaCube}), pcEffect("vignette", map[string]any{"amount": 0.6}))),
		// The built-in look at intensity 1: the GL preview fetches it from the engine's look route.
		pcCase(m, "lut_builtin_teal_orange", 5, ThresholdColour, gl, chart(pcEffect("lut", map[string]any{"lut": "builtin:teal_orange"}))),
		// The uploaded cube at a static 0.6 (export: blend all_opacity with the LUT output first).
		pcCase(m, "lut_upload_intensity", 5, ThresholdColour, gl, chart(pcEffect("lut", map[string]any{"lut": MediaCube, "intensity": 0.6}))),
		// intensity keys 0.2 at source frame 0 to 1 at 30: frame 15 is 0.6, the static value of lut_upload_intensity
		// (export: blend all_expr with T). The static intensity is 1.
		withDelta(pcCase(m, "lut_keyframed_intensity", 15, ThresholdColour, gl, chart(pcEffect("lut", map[string]any{"lut": MediaCube}),
			pcEffectKeys(0, "intensity", pcKey(0, 0.2), pcKey(30, 1)))), DeltaBlend, m.Project("lut_keyframed_intensity_baseline", chart())),
		// crop 0.1/0.05/0.2/0.15 of 640x360 is the 448x288 rect at (64, 18) (cropRect); contained it is 1120x720 at x 80.
		pcCase(m, "crop_static_contain", 5, ThresholdGeometry, both, chart(pcCrop(0.1, 0.05, 0.2, 0.15))),
		// The portrait video (720x1280) cropped t 0.2, b 0.3 to the 720x640 rect at (0, 256), covering the 1024x576 box at
		// (128, 72): the cover window is the middle 576 of 910 scaled rows.
		pcCase(m, "crop_static_cover", 10, ThresholdGeometry, both, VideoTrack("v1", pcClip("p", MediaPortrait, 0, 40,
			pcBox(0.1, 0.1, 0.8, 0.8), pcCover, pcCrop(0, 0.2, 0, 0.3)))),
		// crop.l keys 0 -> 0.3 and crop.b keys 0.3 -> 0 over source frames 0..20 (linear). Frame 12 crops l 0.18, t 0.1,
		// r 0.1, b 0.12: the 460x280 rect at (116, 36). The static crop is 0.1/0.1/0.1/0.25. With crop keyframes the crop
		// runs just before the transform.
		pcCase(m, "crop_keyframed", 12, ThresholdGeometry, both, chart(pcCrop(0.1, 0.1, 0.1, 0.25),
			pcKeys("crop.l", pcKey(0, 0), pcKey(20, 0.3)), pcKeys("crop.b", pcKey(0, 0.3), pcKey(20, 0)))),
		pcCase(m, "flip_h", 5, ThresholdGeometry, both, chart(pcFlipH)),
		// Crop fractions refer to the unflipped source. The static crop takes the 448x252 rect at (32, 108) first, then
		// vflip mirrors it; with the 16:9 rect it fills the frame.
		pcCase(m, "flip_v_crop", 5, ThresholdGeometry, both, chart(pcFlipV, pcCrop(0.05, 0.3, 0.25, 0))),
		// radius 0.01 of the 360 px picture: sigma 3.6 px on both sides (export gblur with premultiply on image clips).
		pcCase(m, "blur_static", 5, ThresholdDetail, gl, detail(pcEffect("blur", map[string]any{"radius": 0.01}))),
		// The logo's soft alpha edge blurred, radius 0.02 of its 360 px: sigma 7.2 px. The 768x432 box at (256, 144).
		// Report-only Delta (plan 2f Task 4, F2 outcome B, index §4.26.9): drawn 1:1 the preview blur follows the export
		// (blur_logo_alpha_native: slope 0.9314; the opaque detail chart at radius 0.01 and 0.02: 1.0255 and 1.0391), magnified
		// 1.2x it does not (slope 0.8660; 2e 0.8660). The gap is the magnification of the blurred picture, GL bilinear in
		// PLACE against the export's bicubic scale, not the blur pass. The logo at radius 0.01 (slope 0.7004) is no
		// evidence: its change reaches 0.0104 of the samples (below MinChanged) and 0.0000 at 8 levels, so
		// it sits in the export's 1-2 level rounding noise. Bicubic magnification in PLACE is phase 3 backlog.
		withDelta(pcCase(m, "blur_logo_alpha", 10, ThresholdDetail, gl, logoOver(pcBox(0.2, 0.2, 0.6, 0.6), pcEffect("blur", map[string]any{"radius": 0.02}))...),
			reportOnly(DeltaMask), m.Project("blur_logo_alpha_baseline", logoOver(pcBox(0.2, 0.2, 0.6, 0.6))...)),
		// The native blur guard (plan 2f Task 4, index §4.12): the logo 1:1 in the four quadrants over the chart, radius 0.02
		// on every tile, normal DeltaMask bounds (measured changed 0.0903, slope 0.9314, r 0.9930). Four tiles, because one
		// 1:1 tile changes barely enough samples for MinChanged (0.0247).
		cdCase(m, "blur_logo_alpha_native", 10, ThresholdDetail, DeltaMask, pcLogoTiles(pcEffect("blur", map[string]any{"radius": 0.02})), pcLogoTiles()),
		// CAS at strength 0.8 on the gratings and edges.
		// Report-only Delta (plan 2f Task 4, F2 outcome B): drawn 1:1 the preview sharpen follows the export
		// (sharpen_static_native: slope 0.9654), magnified 2x it does not (slope 0.8815; 2e 0.8815). The gap is the
		// magnification: bilinear interpolation weakens the fine detail CAS adds, the export's bicubic scale keeps more of
		// it. Strength 0.4 changes too few samples to measure (0.0005). The CAS constants still fit FFmpeg (refit 15.104
		// and 4.009). Bicubic magnification in PLACE is phase 3 backlog.
		withDelta(pcCase(m, "sharpen_static", 5, ThresholdDetail, gl, detail(pcEffect("sharpen", map[string]any{"strength": 0.8}))),
			reportOnly(DeltaMask), m.Project("sharpen_static_baseline", detail())),
		// The native sharpen guard: detail.png 1:1 in the four quadrants (one video track each), strength 0.8 on every tile,
		// normal DeltaMask bounds (measured changed 0.0275, slope 0.9654, r 0.9900; one tile: changed 0.0069).
		cdCase(m, "sharpen_static_native", 5, ThresholdDetail, DeltaMask, pcDetailTiles(pcEffect("sharpen", map[string]any{"strength": 0.8})), pcDetailTiles()),
		// strength keys 0 at source frame 0 to 1 at 20: frame 15 is 0.75 (the static strength is 0.2; export: cas commands).
		pcCase(m, "sharpen_keyframed", 15, ThresholdDetail, gl, detail(pcEffect("sharpen", map[string]any{"strength": 0.2}),
			pcEffectKeys(0, "strength", pcKey(0, 0), pcKey(20, 1)))),
		// amount 0.7, size 0.8 on 640x360: angle min(pi/2, 0.7 (pi/2) hypot(320, 180) / (0.8 x 360)) = 1.4018 rad.
		pcCase(m, "vignette_static", 5, ThresholdDetail, gl, chart(pcEffect("vignette", map[string]any{"amount": 0.7, "size": 0.8}))),
		// The logo keeps its alpha (export: alphaextract / alphamerge around the vignette). amount 0.8, size 0.6: the angle
		// clamps to pi/2. The 896x504 box at (192, 108).
		pcCase(m, "vignette_logo_alpha", 10, ThresholdDetail, gl, logoOver(pcBox(0.15, 0.15, 0.7, 0.7), pcEffect("vignette", map[string]any{"amount": 0.8, "size": 0.6}))...),
		// On the gradient video (no alpha wrapper, eval=frame): amount keys 0.1 at source frame 0 to 0.9 at 20; frame 15 is
		// 0.7 with size 1, angle 1.1214 rad (the static amount is 0.2).
		pcCase(m, "vignette_keyframed", 15, ThresholdDetail, gl, gradient(pcEffect("vignette", map[string]any{"amount": 0.2}),
			pcEffectKeys(0, "amount", pcKey(0, 0.1), pcKey(20, 0.9)))),
	}
}
