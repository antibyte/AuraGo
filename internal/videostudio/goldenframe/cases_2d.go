package goldenframe

import (
	"image/color"
	"math"

	"aurago/internal/videostudio"
)

// Compositing goldens (plan 2d, Task 12; index §4.12, §4.22): chroma key, masks, blend modes and blend transitions.
//
// Media (Go-written PNGs, so no yuv420p conversion enters these comparisons):
//   - greenscreen.png (greenPixel) holds:
//   - the key colour #00b140 under uneven light;
//   - a subject: skin with red and blue patches over a light grey shirt with a growing green cast;
//   - a green spill band inside the subject edge;
//   - six patches at graded colorkey distances;
//   - a semi-transparent orange corner (input alpha under the key).
//   - layer.png (layerPixel): hue across x and value across y at saturation 1, so every channel runs from 0 to 255 and
//     crosses 128, which reaches every branch of every mode. The lower third is a pastel band whose alpha falls from 1
//     to 0.2.
//
// Placement:
//   - key cases: the greenscreen on an overlay track over the chart;
//   - mask cases: the chart on an overlay track over the blue still, so every masked-out pixel turns blue;
//   - blend cases: layer.png at opacity 0.85 on an overlay track over the chart.
//
// Painters: GL only. The 2D painter draws no key, blend or mask (index §4.5).
//
// Thresholds (index §4.12): key and mask ThresholdDetail, blend ThresholdColour, blend transitions ThresholdTransition,
// and ThresholdDetail for the combined case (it holds a key and a mask). A case with another threshold states the
// measured reason on the line above it.
//
// Delta: every case carries one (DeltaKey, DeltaMask or DeltaBlend). Its Baseline is the same case with the feature off,
// with three exceptions, each explained above its case: key_spill_full, blend_transition_mixed and blend_normal_alpha.
//
// Values: each case comment gives the values at the case frame, and cases_2d_test.go pins them through ValueAt.
// Keyframes sit on still clips that start at frame 0 with offset 0, so a source frame is the timeline frame.
//
// cases_2d.go sorts between cases_2c.go and cases_foundation.go, so these cases register third. The order has no
// meaning.
func init() {
	registerCases(compositingCases)
	registerMedia(MediaGreen, greenMedia)
	registerMedia(MediaLayer, layerMedia)
}

// Names (asset ids) of the 2d media.
const (
	MediaGreen = "greenscreen" // greenscreen.png: 640x360 NRGBA green screen (greenPixel)
	MediaLayer = "layer"       // layer.png: 640x360 NRGBA blend layer (layerPixel)
)

var (
	greenMedia = pngFixture("greenscreen.png", 640, 360, greenPixel)
	layerMedia = pngFixture("layer.png", 640, 360, layerPixel)
)

// greenKey is the colour the greenscreen is built around: chroma_key's default #00b140 (index §4.2).
var greenKey = [3]float64{0, 177.0 / 255, 64.0 / 255}

// greenPatchDistances are the colorkey distances (|rgb − key| / (255·√3), index §4.22) of the six graded patches. Each
// patch is the key colour with red raised by d·√3, so at similarity 0.25 and smoothness 0.1 the row reads keyed,
// keyed, 30 %, 70 %, opaque, opaque.
var greenPatchDistances = [6]float64{0.15, 0.22, 0.28, 0.32, 0.36, 0.5}

// The greenscreen layout in source pixels.
const (
	greenSubjectX, greenSubjectY   = 320.0, 176.0 // centre of the subject ellipse (x 170..470, y 56..296)
	greenSubjectRX, greenSubjectRY = 150.0, 120.0
	greenEdge                      = 6.0         // the subject edge blends into the backdrop over about this many px
	greenSpill                     = 16.0        // the green spill reaches this far inside the subject edge
	greenPatchY, greenPatchSize    = 312.0, 36.0 // patches: 36 px squares at y 312..348, x 40 + 96 i
	greenCornerRadius              = 110.0       // the orange disc around the top right corner
)

// greenPixel draws the greenscreen (see the file comment). Edges are soft, so the export's bicubic and the preview's
// bilinear upscale agree.
func greenPixel(x, y int) color.NRGBA {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	// The backdrop: the key colour under uneven light, 0.8 at the top left to 1.08 at the bottom right (colorkey distance
	// at most 0.09, far inside every similarity the cases use).
	light := 0.8 + 0.28*(0.6*fx/640+0.4*fy/360)
	c := [3]float64{greenKey[0] * light, greenKey[1] * light, greenKey[2] * light}
	alpha := 1.0
	// The subject. inside is the approximate distance from the ellipse edge in px (> 0 inside).
	inside := (1 - math.Hypot((fx-greenSubjectX)/greenSubjectRX, (fy-greenSubjectY)/greenSubjectRY)) * greenSubjectRY
	if cover := pcSmooth(inside/greenEdge + 0.5); cover > 0 {
		skin := [3]float64{0.93, 0.78, 0.67}
		skin = pcMix(skin, [3]float64{0.86, 0.16, 0.16}, pcSmooth((30-math.Hypot(fx-268, fy-132))/4+0.5))
		skin = pcMix(skin, [3]float64{0.16, 0.24, 0.78}, pcSmooth((30-math.Hypot(fx-372, fy-132))/4+0.5))
		// The shirt: light grey with a green cast of 20 levels at its left end to 60 at its right end.
		cast := (20 + 40*math.Max(0, math.Min(1, (fx-170)/300))) / 255
		shirt := [3]float64{190.0 / 255, 190.0/255 + cast, 186.0 / 255}
		subject := pcMix(skin, shirt, pcSmooth((fy-176)/24))
		// The green spill from the backdrop just inside the edge.
		subject[1] = math.Min(1, subject[1]+0.2*pcSmooth(1-inside/greenSpill))
		c = pcMix(c, subject, cover)
	}
	for i, d := range greenPatchDistances {
		left := 40 + 96*float64(i)
		edge := math.Min(math.Min(fx-left, left+greenPatchSize-fx), math.Min(fy-greenPatchY, greenPatchY+greenPatchSize-fy))
		if w := pcSmooth(edge/3 + 0.5); w > 0 {
			c = pcMix(c, [3]float64{d * math.Sqrt(3), greenKey[1], greenKey[2]}, w)
		}
	}
	// The corner: an orange disc that the key keeps, whose own alpha falls to 0.15 at the corner.
	if rr := math.Hypot(640-fx, fy); rr < greenCornerRadius+4 {
		c = pcMix(c, [3]float64{0.9, 0.47, 0.16}, pcSmooth((greenCornerRadius-rr)/8+0.5))
		alpha = 0.15 + 0.85*pcSmooth(rr/(greenCornerRadius-10))
	}
	return color.NRGBA{R: pcByte(c[0]), G: pcByte(c[1]), B: pcByte(c[2]), A: pcByte(alpha)}
}

// layerPixel draws the blend layer:
//   - rows 0..227: hue across x, value from 1 (rows 0..7) to 0 (row 232), saturation 1;
//   - from row 228: blended over 24 rows into a pastel band (saturation 0.55, value 0.9);
//   - from row 240: alpha falls from 1 to 0.2 at the bottom.
func layerPixel(x, y int) color.NRGBA {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	hue := fx / 640
	value := 1 - math.Max(0, math.Min(1, (fy-8)/224))
	c := pcMix(layerHSV(hue, 1, value), layerHSV(hue, 0.55, 0.9), pcSmooth((fy-228)/24))
	alpha := 1 - 0.8*math.Max(0, math.Min(1, (fy-240)/120))
	return color.NRGBA{R: pcByte(c[0]), G: pcByte(c[1]), B: pcByte(c[2]), A: pcByte(alpha)}
}

// layerHSV converts hue (0..1 for 0..360 degrees), saturation and value to RGB (the hexcone formula).
func layerHSV(hue, saturation, value float64) [3]float64 {
	channel := func(n float64) float64 {
		k := math.Mod(n+hue*6, 6)
		return value - value*saturation*math.Max(0, math.Min(1, math.Min(k, 4-k)))
	}
	return [3]float64{channel(5), channel(3), channel(1)}
}

// cdBlend sets a clip's blend mode; normal leaves the field empty (the canonical form, index §4.1).
func cdBlend(mode string) pcEdit {
	return func(clip *videostudio.Clip) {
		if mode == videostudio.BlendNormal {
			clip.Blend = ""
			return
		}
		clip.Blend = mode
	}
}

// cdMask gives a clip a mask. Position and size are fractions of the clip box, feather of the box height.
func cdMask(shape videostudio.MaskShape, x, y, w, h, rotation, feather float64, invert bool) pcEdit {
	return func(clip *videostudio.Clip) {
		clip.Mask = &videostudio.Mask{Shape: shape, X: x, Y: y, W: w, H: h, Rotation: rotation, Feather: feather, Invert: invert}
	}
}

func cdOpacity(value float64) pcEdit { return func(clip *videostudio.Clip) { clip.Opacity = value } }

// cdDissolve gives a clip an outgoing dissolve of frames frames.
func cdDissolve(frames int) pcEdit {
	return func(clip *videostudio.Clip) {
		clip.Transition = &videostudio.Transition{Type: videostudio.TransitionDissolve, Duration: frames}
	}
}

// cdKey appends a chroma_key entry with complete params: the registry defaults, overridden by params (float64 values).
func cdKey(params map[string]any) pcEdit { return pcEffect("chroma_key", params) }

// cdWith returns base followed by more in a new slice, so shared edit lists are never appended to in place.
func cdWith(base []pcEdit, more ...pcEdit) []pcEdit {
	return append(append([]pcEdit{}, base...), more...)
}

// cdSpillKeyframable reports whether Task 4's despill gate made chroma_key's spill keyframable (index §4.11).
func cdSpillKeyframable() bool {
	spec, ok := videostudio.EffectByID("chroma_key")
	if !ok {
		return false
	}
	for _, param := range spec.Params {
		if param.ID == "spill" {
			return param.Keyframable
		}
	}
	return false
}

// cdCase is a GL-only case with a delta check whose baseline is the project of the baseline tracks.
func cdCase(m Media, name string, frame int, threshold float64, delta Delta, tracks, baseline []videostudio.Track) Case {
	c := pcCase(m, name, frame, threshold, []string{PainterGL}, tracks...)
	return withDelta(c, delta, m.Project(name+"_baseline", baseline...))
}

func compositingCases(m Media) []Case {
	ellipse, rect := videostudio.MaskEllipse, videostudio.MaskRect
	chart := func() videostudio.Track { return VideoTrack("v1", pcClip("bg", MediaChart, 0, 60)) }
	green := func(edits ...pcEdit) []videostudio.Track {
		return []videostudio.Track{chart(), OverlayTrack("o1", pcClip("g", MediaGreen, 0, 60, edits...))}
	}
	masked := func(edits ...pcEdit) []videostudio.Track {
		return []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaBlue, 0, 60)), OverlayTrack("o1", pcClip("m", MediaChart, 0, 60, edits...))}
	}
	layer := func(edits ...pcEdit) []videostudio.Track {
		return []videostudio.Track{chart(), OverlayTrack("o1", pcClip("l", MediaLayer, 0, 60, cdWith([]pcEdit{cdOpacity(0.85)}, edits...)...))}
	}
	// pair: layer.png (frames 0-39) dissolving over frames 20-39 into the mirrored layer (frames 20-59), both at opacity
	// 0.85; frame 30 is progress 0.5.
	pair := func(a, b string) []videostudio.Track {
		return []videostudio.Track{chart(), OverlayTrack("o1",
			pcClip("a", MediaLayer, 0, 40, cdOpacity(0.85), cdBlend(a), cdDissolve(20)),
			pcClip("b", MediaLayer, 20, 40, cdOpacity(0.85), cdBlend(b), pcFlipH))}
	}
	key := func(similarity, smoothness, spill float64) pcEdit {
		return cdKey(map[string]any{"similarity": similarity, "smoothness": smoothness, "spill": spill})
	}
	keyframed := []pcEdit{key(0.15, 0.25, 0.2),
		pcEffectKeys(0, "similarity", pcKey(0, 0.1), pcKey(20, 0.4)),
		pcEffectKeys(0, "smoothness", pcKey(0, 0.3), pcKey(20, 0.02))}
	if cdSpillKeyframable() {
		keyframed = append(keyframed, pcEffectKeys(0, "spill", pcKey(0, 0), pcKey(20, 1)))
	}
	keyedBox := []pcEdit{pcBox(0.1, 0.15, 0.4, 0.5),
		pcKeys("x", pcKey(0, 0.05), pcKey(20, 0.45)), pcKeys("width", pcKey(0, 0.4), pcKey(20, 0.6)), pcKeys("rotation", pcKey(0, 0), pcKey(20, 20))}
	coverCrop := []pcEdit{pcBox(0.25, 0.05, 0.5, 0.9), pcCover, pcCrop(0.05, 0, 0, 0), pcKeys("crop.l", pcKey(0, 0), pcKey(20, 0.3))}
	stillBox := pcBox(0.1, 0.1, 0.8, 0.8)
	cases := []Case{
		// The default key (cases_2d_test.go reads its values from the registry). The backdrop goes and the chart shows
		// through; the graded patches key 0, 0, 0.3, 0.7, 1, 1.
		cdCase(m, "key_green_default", 5, ThresholdDetail, DeltaKey, green(cdKey(nil)), green()),
		// A soft key: similarity 0.2, smoothness 0.3, spill at its default. Alpha ramps over distances 0.2..0.5, so the
		// subject's soft edge and four of the six patches are partly transparent.
		cdCase(m, "key_soft_edge", 5, ThresholdDetail, DeltaKey, green(cdKey(map[string]any{"similarity": 0.2, "smoothness": 0.3})), green()),
		// Full spill suppression: similarity 0.25, smoothness 0.1, spill 1. The baseline keys with spill 0 (despill
		// omitted), so the delta isolates the despill: the shirt's cast (22..62 levels of green) and the spill band. A
		// key-off baseline would hide a missing despill under the key's own, much larger change.
		cdCase(m, "key_spill_full", 5, ThresholdDetail, DeltaKey, green(key(0.25, 0.1, 1)), green(key(0.25, 0.1, 0))),
		// The default key on the greenscreen in the box (0.1, 0.1, 0.8, 0.8). The key keeps the orange corner, so its
		// alpha is picture alpha × key (the alpha combine, index §4.22) over the chart.
		cdCase(m, "key_still_alpha", 5, ThresholdDetail, DeltaKey, green(stillBox, cdKey(nil)), green(stillBox)),
		// Keys over frames 0..20: similarity 0.1 -> 0.4 and smoothness 0.3 -> 0.02. Frame 15 keys with 0.325 and 0.09;
		// the static values are 0.15 and 0.25. If Task 4's despill gate made spill keyframable, spill keys 0 -> 1 give
		// 0.75 (static 0.2); otherwise spill stays 0.2.
		cdCase(m, "key_keyframed", 15, ThresholdDetail, DeltaKey, green(keyframed...), green()),
		// Static masks (geq once at full resolution, then loop) on the full-frame chart over blue.
		// The rect (0.2, 0.15, 0.5, 0.6) without feather (F = 1 px) is the 640x432 rect at (256, 108).
		cdCase(m, "mask_rect_static", 5, ThresholdDetail, DeltaMask, masked(cdMask(rect, 0.2, 0.15, 0.5, 0.6, 0, 0, false)), masked()),
		// The ellipse (0.15, 0.1, 0.7, 0.8) with feather 0.02 of the 720 px box height (F = 14.4 px).
		cdCase(m, "mask_ellipse_static", 5, ThresholdDetail, DeltaMask, masked(cdMask(ellipse, 0.15, 0.1, 0.7, 0.8, 0, 0.02, false)), masked()),
		// The ellipse (0.2, 0.25, 0.6, 0.5) rotated 30 degrees clockwise about its centre, feather 0.01 (F = 7.2 px).
		cdCase(m, "mask_ellipse_rotated", 5, ThresholdDetail, DeltaMask, masked(cdMask(ellipse, 0.2, 0.25, 0.6, 0.5, 30, 0.01, false)), masked()),
		// The rect (0.1, 0.1, 0.8, 0.8) rotated -12 degrees, feather 0.15 (F = 108 px: the ramp spans several chart bands).
		cdCase(m, "mask_rect_feather", 5, ThresholdDetail, DeltaMask, masked(cdMask(rect, 0.1, 0.1, 0.8, 0.8, -12, 0.15, false)), masked()),
		// The inverted ellipse (0.3, 0.2, 0.4, 0.6), feather 0.03: the chart stays outside it, and blue shows inside.
		cdCase(m, "mask_ellipse_invert", 5, ThresholdDetail, DeltaMask, masked(cdMask(ellipse, 0.3, 0.2, 0.4, 0.6, 0, 0.03, true)), masked()),
		// A keyframed mask (quarter-resolution geq per frame). The ellipse's keys over frames 0..20 are x 0.05 -> 0.45,
		// w 0.3 -> 0.5, rotation 0 -> 40 and feather 0 -> 0.08. Frame 15 is x 0.35, w 0.45, rotation 30 and feather
		// 0.06 (F = 43.2 px), with the static y 0.2 and h 0.6. The static values are x 0.1, w 0.3, rotation 0 and
		// feather 0.
		cdCase(m, "mask_keyframed", 15, ThresholdDetail, DeltaMask, masked(cdMask(ellipse, 0.1, 0.2, 0.3, 0.6, 0, 0, false),
			pcKeys("mask.x", pcKey(0, 0.05), pcKey(20, 0.45)), pcKeys("mask.w", pcKey(0, 0.3), pcKey(20, 0.5)),
			pcKeys("mask.rotation", pcKey(0, 0), pcKey(20, 40)), pcKeys("mask.feather", pcKey(0, 0), pcKey(20, 0.08))), masked()),
		// A keyed box carries a static mask (the pad-canvas path).
		// - Box keys over frames 0..20: x 0.05 -> 0.45, width 0.4 -> 0.6, rotation 0 -> 20. Frame 15 is x 0.35,
		//   width 0.55 and rotation 15, with y 0.15 and height 0.5. The static box is (0.1, 0.15, 0.4, 0.5) at rotation 0.
		// - The mask: the rect (0.1, 0.1, 0.8, 0.8) rotated 20 degrees, feather 0.05 (F = 18 px). It moves, scales and
		//   turns with the box.
		cdCase(m, "mask_box_animated", 15, ThresholdDetail, DeltaMask, masked(cdWith(keyedBox, cdMask(rect, 0.1, 0.1, 0.8, 0.8, 20, 0.05, false))...), masked(keyedBox...)),
		// Box space against an animated crop. The chart covers the tall box (0.25, 0.05, 0.5, 0.9), 640x648 px, while
		// crop.l keys 0 -> 0.3 over frames 0..20 (0.18 at frame 12; static 0.05). The ellipse mask (0.1, 0.1, 0.8, 0.8),
		// feather 0.04, stays on the box: it comes from the static generator, and the crop only moves the picture under it.
		cdCase(m, "mask_cover_crop_animated", 12, ThresholdDetail, DeltaMask, masked(cdWith(coverCrop, cdMask(ellipse, 0.1, 0.1, 0.8, 0.8, 0, 0.04, false))...), masked(coverCrop...)),
	}
	// One case per blend mode, in registry order. layer.png (opacity 0.85; alpha falls to 0.2 in its lower third) over
	// the chart; the baseline is the same layer in normal.
	for _, mode := range videostudio.EffectRegistry().BlendModes {
		if mode.ID == videostudio.BlendNormal {
			continue
		}
		cases = append(cases, cdCase(m, "blend_"+mode.ID, 5, ThresholdColour, DeltaBlend, layer(cdBlend(mode.ID)), layer()))
	}
	return append(cases,
		// Normal with alpha takes the existing overlay path (an all-normal layer keeps the 2c graph). The baseline is the
		// chart alone, so the delta covers the whole straight-alpha mix.
		cdCase(m, "blend_normal_alpha", 5, ThresholdColour, DeltaBlend, layer(), []videostudio.Track{chart()}),
		// A dissolve between two multiply clips keeps multiply in the overlap (index §4.22). The baseline is the same pair
		// in normal.
		cdCase(m, "blend_transition_shared", 30, ThresholdTransition, DeltaBlend, pair("multiply", "multiply"), pair(videostudio.BlendNormal, videostudio.BlendNormal)),
		// multiply into screen: the overlap uses normal. The baseline is the shared multiply pair: a normal baseline
		// equals the overlap and would change nothing (Changed 0). This delta shows that both engines leave multiply
		// for normal in the overlap.
		cdCase(m, "blend_transition_mixed", 30, ThresholdTransition, DeltaBlend, pair("multiply", "screen"), pair("multiply", "multiply")),
		// Threshold ThresholdDetail, not ThresholdColour: the case contains a key and a mask (index §4.12). The clip is
		// screened, masked by the ellipse (0.1, 0.05, 0.8, 0.9; feather 0.03) and keyed with similarity 0.25,
		// smoothness 0.1 and spill 0.5. Key alpha × mask × opacity is the mix weight of the screen composite. The
		// baseline is the plain greenscreen.
		cdCase(m, "blend_masked_keyed", 5, ThresholdDetail, DeltaMask,
			green(key(0.25, 0.1, 0.5), cdBlend("screen"), cdMask(ellipse, 0.1, 0.05, 0.8, 0.9, 0, 0.03, false)), green()),
	)
}
