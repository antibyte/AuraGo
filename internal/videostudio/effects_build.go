package videostudio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FFmpeg builders of the effect stack (index §4.10, §4.11, §4.20). clipChain.effects asks the builder of every entry that
// is on (EffectSpec.build) for its stage and joins the stages in stack order. A builder:
//   - reads its values from absolute frames;
//   - declares the pixel format of its input;
//   - returns nothing for identity values.

// effectInput is what an effect builder knows about one stack entry of one clip in one FFmpeg process. Window frame k
// is absolute timeline frame at + k.
type effectInput struct {
	id            string      // "<L>_<C>": layer and clip index, for instance names, labels and file names
	index         int         // the entry's stack index I
	spec          EffectSpec  // the entry's registry row (parameter defaults)
	whole         Clip        // the clip as on the timeline
	at            int         // absolute timeline frame of window frame 0
	frames        int         // window frames
	pieces        []rampPiece // the clip's ramp pieces, nil without a ramp
	rate          Rate
	image         bool      // a still (images, titles): alpha must survive every stage
	width, height int       // the processed picture in decoded pixels (clipChain.processedSize)
	lut           *LUT3D    // lut effect: the entry's table, nil without one
	lutNames      *lutNames // names the LUT files of the whole FFmpeg process (never nil from entryInput)
}

// effectStage is one entry's share of the chain: the pixel format its first filter needs, the format its last filter
// leaves, and its parts (filters, two-input flag, command tracks, LUT files). A stage without parts adds nothing.
type effectStage struct {
	input, output string
	parts         []chainStage
}

// effectBuilder builds one entry's stage.
type effectBuilder func(in effectInput) effectStage

// The builders are attached here rather than in the row literals of effects.go. A builder reaches the registry again
// (effectValueAt reads parameter defaults through EffectByID), and Go would report that as an initialization cycle.
func init() {
	builders := map[string]effectBuilder{"color": buildColorEffect, "lut": buildLUTEffect, "blur": buildBlurEffect, "sharpen": buildSharpenEffect, "vignette": buildVignetteEffect, "chroma_key": buildChromaKeyEffect}
	for i := range registryEffects {
		registryEffects[i].build = builders[registryEffects[i].ID]
	}
}

// keyed reports keyframes on a param of the entry.
func (in effectInput) keyed(param string) bool {
	return len(in.whole.Effects[in.index].Keyframes[param]) > 0
}

// value is a number param at window frame k: its keyframes, else the stored value, else the default (effectValueAt).
func (in effectInput) value(param string, k int) float64 {
	return effectValueAt(in.whole, in.index, param, in.at+k)
}

// values is a number param at every window frame.
func (in effectInput) values(param string) []float64 {
	values := make([]float64, in.frames)
	for k := range values {
		values[k] = in.value(param, k)
	}
	return values
}

// atDefault reports a param without keyframes whose value is its registry default.
func (in effectInput) atDefault(param string) bool {
	if in.keyed(param) {
		return false
	}
	for _, spec := range in.spec.Params {
		if spec.ID == param {
			def, ok := spec.Default.(float64)
			return ok && in.value(param, 0) == def
		}
	}
	return false
}

// target names a commanded filter instance of the entry: "<filter>@vs_<role>_<L>_<C>_<I>" (index §4.11).
func (in effectInput) target(filter, role string) string {
	return fmt.Sprintf("%s@vs_%s_%s_%d", filter, role, in.id, in.index)
}

// label names a graph label of the entry's two-input stages: "vs_fx_<L>_<C>_<I>_<name>".
func (in effectInput) label(name string) string {
	return fmt.Sprintf("vs_fx_%s_%d_%s", in.id, in.index, name)
}

// source stores the window frame's absolute frame and source position (registers 0 and 1) for an expression in the
// filter's time variable: t for vignette, T for blend (windowSourceExprVar, index §4.11).
func (in effectInput) source(variable string) ffExpr {
	return windowSourceExprVar(in.whole, in.at, in.pieces, in.rate, variable)
}

// curve is a number param as an expression of the source position in register 1: its keyframe curve, or its constant.
func (in effectInput) curve(param string) ffExpr {
	if keys := in.whole.Effects[in.index].Keyframes[param]; len(keys) > 0 {
		return curveExpr(keys, exprLd(1))
	}
	return exprNum(in.value(param, 0))
}

// text is a string param of the entry (the chroma_key colour): its params value, else the registry default.
func (in effectInput) text(param string) string {
	if value, ok := in.whole.Effects[in.index].Params[param].(string); ok {
		return value
	}
	for _, spec := range in.spec.Params {
		if spec.ID == param {
			def, _ := spec.Default.(string)
			return def
		}
	}
	return ""
}

// alphaBelowFull reports an input whose alpha can be below 255 at the entry: a still (images, titles), or a clip with
// an earlier chroma_key entry that is on.
func (in effectInput) alphaBelowFull() bool {
	if in.image {
		return true
	}
	for _, entry := range in.whole.Effects[:in.index] {
		if entry.On && entry.ID == "chroma_key" {
			return true
		}
	}
	return false
}

// effectNum writes an option number the way the sendcmd writer writes its values: the shortest decimal that reads back
// exactly, without an exponent, and negative zero as 0.
func effectNum(value float64) string {
	if value == 0 {
		value = 0
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// colorCurveParams are the color params that shape the per-channel curve (index §4.20, steps 1–5), in registry order.
// Saturation (step 6) is the matrix.
var colorCurveParams = []string{"exposure", "contrast", "temperature", "tint", "lift_x", "lift_y", "lift", "gamma_x", "gamma_y", "gamma", "gain_x", "gain_y", "gain"}

// colorMatrixOptions are colorchannelmixer's options for saturationMatrix's nine entries, row by row.
var colorMatrixOptions = []string{"rr", "rg", "rb", "gr", "gg", "gb", "br", "bg", "bb"}

// lutrgbOptions writes the three channel curves as lutrgb options in channelCurveExpr's fixed template.
func lutrgbOptions(curves [3]ChannelCurve) string {
	return fmt.Sprintf("r='%s':g='%s':b='%s'", channelCurveExpr(curves[0]).String(), channelCurveExpr(curves[1]).String(), channelCurveExpr(curves[2]).String())
}

// matrixOptions writes a saturation matrix as colorchannelmixer options.
func matrixOptions(m [9]float64) string {
	options := make([]string, len(colorMatrixOptions))
	for i, name := range colorMatrixOptions {
		options[i] = name + "=" + effectNum(m[i])
	}
	return strings.Join(options, ":")
}

// buildColorEffect is the color effect on rgba:
//   - steps 1–5 as lutrgb, one channelCurveExpr template per channel, left out at the defaults;
//   - step 6 as colorchannelmixer with saturationMatrix, left out at saturation 1.
//
// A keyframed part is a named instance whose commands set every window frame: lutrgb@vs_color_L_C_I gets curve tracks
// r, g and b, and colorchannelmixer@vs_sat_L_C_I gets the nine matrix entries.
func buildColorEffect(in effectInput) effectStage {
	curveKeyed, curveSet := false, false
	for _, param := range colorCurveParams {
		curveKeyed = curveKeyed || in.keyed(param)
		curveSet = curveSet || !in.atDefault(param)
	}
	curvesAt := func(k int) [3]ChannelCurve { return colorCurves(colorParamsAt(in.whole, in.index, in.at+k)) }
	var parts []chainStage
	switch {
	case curveKeyed:
		target := in.target("lutrgb", "color")
		tracks := []SendCmdTrack{{Target: target, Option: "r"}, {Target: target, Option: "g"}, {Target: target, Option: "b"}}
		for ch := range tracks {
			tracks[ch].Curves = make([]ChannelCurve, in.frames)
		}
		for k := range in.frames {
			curves := curvesAt(k)
			for ch := range tracks {
				tracks[ch].Curves[k] = curves[ch]
			}
		}
		parts = append(parts, chainStage{filter: target + "=" + lutrgbOptions(curvesAt(0)), tracks: tracks})
	case curveSet:
		parts = append(parts, chainStage{filter: "lutrgb=" + lutrgbOptions(curvesAt(0))})
	}
	switch {
	case in.keyed("saturation"):
		target := in.target("colorchannelmixer", "sat")
		saturation := in.values("saturation")
		tracks := make([]SendCmdTrack, len(colorMatrixOptions))
		for i, option := range colorMatrixOptions {
			tracks[i] = SendCmdTrack{Target: target, Option: option, Values: make([]float64, in.frames)}
		}
		for k, s := range saturation {
			matrix := saturationMatrix(s)
			for i := range tracks {
				tracks[i].Values[k] = matrix[i]
			}
		}
		parts = append(parts, chainStage{filter: target + "=" + matrixOptions(saturationMatrix(saturation[0])), tracks: tracks})
	case !in.atDefault("saturation"):
		parts = append(parts, chainStage{filter: "colorchannelmixer=" + matrixOptions(saturationMatrix(in.value("saturation", 0)))})
	}
	if len(parts) == 0 {
		return effectStage{}
	}
	return effectStage{input: "rgba", output: "rgba", parts: parts}
}

// buildLUTEffect is the lut effect on gbrap (index §4.11, decision A4). Every lut3d reads the Go-written
// vs_lut_<n>.cube of its table with trilinear interpolation: n numbers the process's distinct tables (lutNames), so the
// lut effects of one table share one file. The intensity k picks one of four forms:
//   - no table ("" ref) or a static 0: nothing;
//   - a static 1: lut3d alone;
//   - a static k in between: [lut][orig]blend=all_mode=normal:all_opacity=min(k, 0.999). The opacity weights the first
//     input, and an opacity of exactly 1 is not applied;
//   - keyframed: blend=all_expr='B+(A-B)*k(T)', where T is the frame time. blend does not follow commands reliably
//     (spike 2026-10-17), and this form costs about 19–20x lut3d alone.
//
// The blend forms are two-input stages, so the commands of later parts go to the next command group.
func buildLUTEffect(in effectInput) effectStage {
	if in.lut == nil {
		return effectStage{}
	}
	keyed := in.keyed("intensity")
	intensity := in.value("intensity", 0)
	if !keyed && intensity <= 0 {
		return effectStage{}
	}
	file := lutFile{name: in.lutNames.name(in.lut), lut: in.lut}
	lut3d := "lut3d=file=" + file.name + ":interp=trilinear"
	if !keyed && intensity >= 1 {
		return effectStage{input: "gbrap", output: "gbrap", parts: []chainStage{{filter: lut3d, luts: []lutFile{file}}}}
	}
	mix := "blend=all_mode=normal:all_opacity=" + effectNum(min(intensity, 0.999))
	if keyed {
		mix = "blend=all_expr='" + lutMixExpr(in).String() + "'"
	}
	toLUT, original, mapped := in.label("in"), in.label("orig"), in.label("lut")
	filter := fmt.Sprintf("split=2[%s][%s];[%s]%s[%s];[%s][%s]%s", toLUT, original, toLUT, lut3d, mapped, mapped, original, mix)
	return effectStage{input: "gbrap", output: "gbrap", parts: []chainStage{{filter: filter, twoInput: true, luts: []lutFile{file}}}}
}

// lutMixExpr is the keyframed intensity mix B + (A − B)·k. A is the LUT output, B the original, and k the intensity
// curve at the window frame's source position (windowSourceExprVar in T, the time variable blend provides).
func lutMixExpr(in effectInput) ffExpr {
	return exprOp(";", in.source("T"), exprOp("+", exprVar("B"), exprOp("*", exprOp("-", exprVar("A"), exprVar("B")), in.curve("intensity"))))
}

// buildBlurEffect is the blur effect on gbrap: gblur with sigma = radius·H of the processed picture (blurSigma) and four
// steps. The radius is not keyframable, because gblur's sigma does not follow commands reproducibly (spike 2026-10-17,
// index risk 16). On a still the blur runs premultiplied (decision A2), so transparent pixels do not bleed their colour
// into the edge. Radius 0 adds nothing.
func buildBlurEffect(in effectInput) effectStage {
	radius := in.value("radius", 0)
	if radius <= 0 {
		return effectStage{}
	}
	blur := "gblur=sigma=" + effectNum(blurSigma(radius, in.height)) + ":steps=4"
	if in.image {
		blur = "premultiply=inplace=1," + blur + ",unpremultiply=inplace=1"
	}
	return effectStage{input: "gbrap", output: "gbrap", parts: []chainStage{{filter: blur}}}
}

// buildSharpenEffect is the sharpen effect on gbrap: FFmpeg's contrast adaptive sharpening (cas) on the three colour
// planes. It is never left out, because CAS at strength 0 still sharpens. A keyframed strength names the instance
// (cas@vs_sharpen_L_C_I) and sets every window frame by command, which cas follows exactly (spike 2026-10-17).
func buildSharpenEffect(in effectInput) effectStage {
	part := chainStage{filter: "cas=strength=" + effectNum(in.value("strength", 0))}
	if in.keyed("strength") {
		target := in.target("cas", "sharpen")
		values := in.values("strength")
		part = chainStage{filter: target + "=strength=" + effectNum(values[0]), tracks: []SendCmdTrack{{Target: target, Option: "strength", Values: values}}}
	}
	return effectStage{input: "gbrap", output: "gbrap", parts: []chainStage{part}}
}

// buildVignetteEffect is the vignette effect (index §4.20): FFmpeg's vignette with dither off, at the angle that
// vignetteAngle folds amount and size into.
//   - Static values are constants evaluated once (eval=init). Keyframed ones are a t expression evaluated for every
//     frame (eval=frame). That costs about 11–13x, so it is used only when keyframed.
//   - vignette has no alpha formats. A video runs it on rgb24. A still splits its alpha off and merges it back
//     (alphaextract/alphamerge); this two-input stage moves the commands of later parts to the next command group.
//   - Amount 0 adds nothing.
func buildVignetteEffect(in effectInput) effectStage {
	keyed := in.keyed("amount") || in.keyed("size")
	if !keyed && in.value("amount", 0) <= 0 {
		return effectStage{}
	}
	vignette := fmt.Sprintf("vignette=angle=%s:eval=init:dither=0", effectNum(vignetteAngle(in.value("amount", 0), in.value("size", 0), in.width, in.height)))
	if keyed {
		vignette = fmt.Sprintf("vignette=angle='%s':eval=frame:dither=0", vignetteAngleExpr(in).String())
	}
	if !in.image {
		return effectStage{input: "rgb24", output: "rgb24", parts: []chainStage{{filter: vignette}}}
	}
	colour, alpha, mask, shaded := in.label("rgb"), in.label("alpha"), in.label("mask"), in.label("shaded")
	filter := fmt.Sprintf("split=2[%s][%s];[%s]alphaextract[%s];[%s]format=rgb24,%s,format=gbrap[%s];[%s][%s]alphamerge",
		colour, alpha, alpha, mask, colour, vignette, shaded, shaded, mask)
	return effectStage{input: "gbrap", output: "gbrap", parts: []chainStage{{filter: filter, twoInput: true}}}
}

// vignetteAngleExpr is vignetteAngle over the window as an expression in t (eval=frame):
// min(π/2, amount·(π/2)·R/(size·H)), with R = hypot(W/2, H/2) of the processed picture.
func vignetteAngleExpr(in effectInput) ffExpr {
	radius := math.Hypot(float64(in.width)/2, float64(in.height)/2)
	angle := exprOp("/", exprOp("*", in.curve("amount"), exprNum(math.Pi/2*radius)), exprOp("*", in.curve("size"), exprNum(float64(in.height))))
	return exprOp(";", in.source("t"), exprOp("min", exprNum(math.Pi/2), angle))
}

// colorkeyReplacesAlpha records what FFmpeg's colorkey does with an input alpha below 255:
//   - true: it writes the key over it;
//   - false: it multiplies the key into it.
//
// TestRenderColorkeyAlphaSemantics measures it on every FFmpeg the tests run on, and fails when this constant disagrees
// (plan 2d Task 4, index risk 20).
const colorkeyReplacesAlpha = true

// buildChromaKeyEffect is the chroma_key effect on rgba (index §4.22). It is never left out: colorkey keys at every
// similarity.
//   - colorkey at the key colour, with similarity and blend = smoothness. Go writes the colour as 0xRRGGBB from the
//     parsed hex (keyColor). Keyed similarity or smoothness name the instance colorkey@vs_key_L_C_I: it starts at the
//     window's first values, and sendcmd sets both options on every window frame (exact, spike 2026-10-17).
//   - despill for spill > 0 (chromaKeyDespill); spill 0 leaves it out.
//   - colorkey writes the key over the alpha (colorkeyReplacesAlpha). Where the input alpha can be below 255
//     (alphaBelowFull), the key therefore runs in a wrapper that multiplies the input alpha back (index §4.22):
//   - split; alphaextract keeps the input alpha a, and colorkey and despill key the other branch;
//   - the keyed picture splits again: alphaextract takes its key k, blend all_mode=multiply gives a·k//255 on
//     gray, and alphamerge sets that as the keyed picture's alpha. Everything stays rgba or gray, so no
//     rgba→gbrap step touches the alpha (exact on 5.1.9 and 8.1.1).
//
// The wrapper is a two-input stage, so the commands of later stages read the next command group. Its keyed
// colorkey sits in the split branch in front of the merge and reads the group in front of the stage.
func buildChromaKeyEffect(in effectInput) effectStage {
	rgb := keyColor(in.text("color"))
	keyer := "colorkey"
	var tracks []SendCmdTrack
	if in.keyed("similarity") || in.keyed("smoothness") {
		keyer = in.target("colorkey", "key")
		tracks = []SendCmdTrack{
			{Target: keyer, Option: "similarity", Values: in.values("similarity")},
			{Target: keyer, Option: "blend", Values: in.values("smoothness")},
		}
	}
	filter := fmt.Sprintf("%s=color=0x%02X%02X%02X:similarity=%s:blend=%s", keyer, rgb[0], rgb[1], rgb[2],
		effectNum(in.value("similarity", 0)), effectNum(in.value("smoothness", 0)))
	if despill, spillTracks := chromaKeyDespill(in, rgb); despill != "" {
		filter += "," + despill
		tracks = append(tracks, spillTracks...)
	}
	if !colorkeyReplacesAlpha || !in.alphaBelowFull() {
		return effectStage{input: "rgba", output: "rgba", parts: []chainStage{{filter: filter, tracks: tracks}}}
	}
	toKey, orig, inputAlpha, keyed := in.label("in"), in.label("orig"), in.label("a"), in.label("keyed")
	keyedRGBA, keyedCopy, keyPlane, product := in.label("k0"), in.label("k1"), in.label("ka"), in.label("m")
	wrapped := fmt.Sprintf("split=2[%s][%s];[%s]alphaextract[%s];[%s]%s[%s];[%s]split=2[%s][%s];[%s]alphaextract[%s];[%s][%s]blend=all_mode=multiply[%s];[%s][%s]alphamerge",
		toKey, orig, orig, inputAlpha, toKey, filter, keyed, keyed, keyedRGBA, keyedCopy, keyedCopy, keyPlane, keyPlane, inputAlpha, product, keyedRGBA, product)
	return effectStage{input: "rgba", output: "rgba", parts: []chainStage{{filter: wrapped, tracks: tracks, twoInput: true}}}
}

// despillScale is the despill option that suppresses a key colour's spill: green when its green is at least its blue,
// else blue (Task 3's spillBlue). despill knows only these two (resolved question 12).
func despillScale(rgb [3]uint8) string {
	if spillBlue(rgb) {
		return "blue"
	}
	return "green"
}

// despillFilter writes despill (instance name) for a key colour and a spill strength (index §4.22):
//   - the type, and the screen channel's scale −spill, from despillScale;
//   - mix 0.5 and expand 0, so f = 0.5;
//   - every other scale, brightness and alpha 0.
func despillFilter(name string, rgb [3]uint8, spill float64) string {
	kind := despillScale(rgb)
	green, blue := 0-spill, 0.0
	if kind == "blue" {
		green, blue = 0, 0-spill
	}
	return fmt.Sprintf("%s=type=%s:mix=0.5:expand=0:red=0:green=%s:blue=%s:brightness=0:alpha=0", name, kind, effectNum(green), effectNum(blue))
}

// chromaKeyDespill is the despill part of a chroma_key entry and its command tracks (index §4.11):
//   - static spill: despill at that spill, or "" at spill 0;
//   - keyed spill: despill@vs_spill_L_C_I at the window's first spill, with the screen channel's scale (−spill) set on
//     every window frame by sendcmd (exact on 5.1.9 and 8.1.1, TestRenderDespillCommands). A keyed spill that
//     reaches 0 keeps the filter, at scale 0.
func chromaKeyDespill(in effectInput, rgb [3]uint8) (string, []SendCmdTrack) {
	if !in.keyed("spill") {
		spill := in.value("spill", 0)
		if spill <= 0 {
			return "", nil
		}
		return despillFilter("despill", rgb, spill), nil
	}
	target := in.target("despill", "spill")
	values := in.values("spill")
	scales := make([]float64, len(values))
	for k, spill := range values {
		scales[k] = 0 - spill
	}
	return despillFilter(target, rgb, values[0]), []SendCmdTrack{{Target: target, Option: despillScale(rgb), Values: scales}}
}
