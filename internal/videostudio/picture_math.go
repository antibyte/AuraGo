package videostudio

import (
	"math"
	"strconv"
)

// Colour and picture math of plan 2c (plan index §4.20, normative). Its users:
//   - the export builders, the crop chain and the built-in looks;
//   - the tests.
//
// JS VideoStudioPicture (video-studio-picture.js) computes the same numbers (parity topics color, wheels and picture).

// ChannelCurve is steps 1–5 of the colour formula for one channel, v in 0..1:
//
//	x = clamp((v·Mul − 0.5)·Contrast + 0.5, 0, 1)
//	x = clamp((x·(1 − Lift) + Lift)·Gain, 0, 1)
//	out = x^InvGamma
//
// The fields:
//   - Mul is 2^exposure times the channel's white-balance gain.
//   - Lift and Gain are the wheel-shifted masters.
//   - InvGamma is 1/gamma_c.
//
// channelCurveExpr writes the curve as the lutrgb expression of the export. Step 6 (saturation) is the
// colorchannelmixer matrix (saturationMatrix).
type ChannelCurve struct {
	Mul, Contrast, Lift, Gain, InvGamma float64
}

// apply evaluates the curve at v (0..1) in float. The export's lutrgb table holds round(255·apply(val/255)).
func (c ChannelCurve) apply(v float64) float64 {
	x := clampUnit((v*c.Mul-0.5)*c.Contrast + 0.5)
	x = clampUnit((x*(1-c.Lift) + c.Lift) * c.Gain)
	return math.Pow(x, c.InvGamma)
}

// colorParams are the values of a "color" effect entry at one frame, in registry order. The JSON tags are the
// parameter ids (parity fixtures).
type colorParams struct {
	Exposure    float64 `json:"exposure"`
	Contrast    float64 `json:"contrast"`
	Saturation  float64 `json:"saturation"`
	Temperature float64 `json:"temperature"`
	Tint        float64 `json:"tint"`
	LiftX       float64 `json:"lift_x"`
	LiftY       float64 `json:"lift_y"`
	Lift        float64 `json:"lift"`
	GammaX      float64 `json:"gamma_x"`
	GammaY      float64 `json:"gamma_y"`
	Gamma       float64 `json:"gamma"`
	GainX       float64 `json:"gain_x"`
	GainY       float64 `json:"gain_y"`
	Gain        float64 `json:"gain"`
}

// defaultColorParams is the identity grade: every colour parameter at its registry default. TestColorFormula checks it
// against the registry through colorParamsAt.
func defaultColorParams() colorParams {
	return colorParams{Contrast: 1, Saturation: 1, Temperature: ColorNeutralKelvin, Gamma: 1, Gain: 1}
}

// colorParamsAt evaluates the colour parameters of clip.Effects[index] at absolute timeline frame frame. Each value
// comes from the entry's keyframes, else its params, else the registry default (effectValueAt). The entry must be a
// "color" entry; for any other entry every value is 0.
func colorParamsAt(clip Clip, index, frame int) colorParams {
	value := func(param string) float64 { return effectValueAt(clip, index, param, frame) }
	return colorParams{
		Exposure: value("exposure"), Contrast: value("contrast"), Saturation: value("saturation"),
		Temperature: value("temperature"), Tint: value("tint"),
		LiftX: value("lift_x"), LiftY: value("lift_y"), Lift: value("lift"),
		GammaX: value("gamma_x"), GammaY: value("gamma_y"), Gamma: value("gamma"),
		GainX: value("gain_x"), GainY: value("gain_y"), Gain: value("gain"),
	}
}

// whiteBalance returns the per-channel gains of a temperature in kelvin and a tint:
//
//	(e^{0.5·s}, e^{−0.3·tint}, e^{−0.5·s}), with s = ln(T/6500)
//
// divided by their BT.709 luma. So 6500 K and tint 0 give 1, 1, 1, and any change keeps the luma of grey. Higher K
// warms (red up, blue down; the camera convention); +tint is magenta (green down).
func whiteBalance(temperature, tint float64) [3]float64 {
	s := math.Log(temperature / ColorNeutralKelvin)
	gains := [3]float64{math.Exp(colorTemperatureGain * s), math.Exp(-colorTintGain * tint), math.Exp(-colorTemperatureGain * s)}
	luma := lumaBT709[0]*gains[0] + lumaBT709[1]*gains[1] + lumaBT709[2]*gains[2]
	for c := range gains {
		gains[c] /= luma
	}
	return gains
}

// wheelPhases holds (cos φ, sin φ) of the channel angles φ = 0°, 120° and 240° (R, G, B).
var wheelPhases = [3][2]float64{{1, 0}, {-0.5, math.Sqrt(3) / 2}, {-0.5, -math.Sqrt(3) / 2}}

// wheelOffset maps a wheel position (x, y) to the per-channel offsets x·cos φ_c + y·sin φ_c, without the factor
// ColorWheelK. Wheels are stored as x/y with x = r·cos θ and y = r·sin θ (spec §7.3, refined by plan 2c).
func wheelOffset(x, y float64) [3]float64 {
	var out [3]float64
	for c, phase := range wheelPhases {
		out[c] = x*phase[0] + y*phase[1]
	}
	return out
}

// colorCurves builds steps 1–5 per channel:
//   - Mul = 2^exposure·wb_c;
//   - lift_c = lift + k·w_c(lift_x, lift_y);
//   - gain_c = gain + k·w_c(gain_x, gain_y);
//   - gamma_c = gamma·e^{k·w_c(gamma_x, gamma_y)}, so InvGamma = 1/gamma_c.
func colorCurves(p colorParams) [3]ChannelCurve {
	wb := whiteBalance(p.Temperature, p.Tint)
	exposure := math.Exp2(p.Exposure)
	lift, gamma, gain := wheelOffset(p.LiftX, p.LiftY), wheelOffset(p.GammaX, p.GammaY), wheelOffset(p.GainX, p.GainY)
	var curves [3]ChannelCurve
	for c := range curves {
		curves[c] = ChannelCurve{
			Mul:      exposure * wb[c],
			Contrast: p.Contrast,
			Lift:     p.Lift + ColorWheelK*lift[c],
			Gain:     p.Gain + ColorWheelK*gain[c],
			InvGamma: 1 / (p.Gamma * math.Exp(ColorWheelK*gamma[c])),
		}
	}
	return curves
}

// saturationMatrix is step 6 as the colorchannelmixer coefficients rr, rg, rb, gr, gg, gb, br, bg, bb. Each output
// channel is y + s·(c − y), with y the BT.709 luma: (1 − s)·luma, plus s on the diagonal.
func saturationMatrix(s float64) [9]float64 {
	var m [9]float64
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			m[row*3+col] = (1 - s) * lumaBT709[col]
			if row == col {
				m[row*3+col] += s
			}
		}
	}
	return m
}

// applyColor is the whole colour formula (steps 1–6) in float, for one RGB triple in 0..1.
func applyColor(p colorParams, rgb [3]float64) [3]float64 {
	curves := colorCurves(p)
	var x [3]float64
	for c := range x {
		x[c] = curves[c].apply(rgb[c])
	}
	y := lumaBT709[0]*x[0] + lumaBT709[1]*x[1] + lumaBT709[2]*x[2]
	var out [3]float64
	for c := range out {
		out[c] = clampUnit(y + p.Saturation*(x[c]-y))
	}
	return out
}

// cropRect is a crop in pixels of a width×height picture (decoded and autorotated):
//
//	r' = min(r, 0.95 − l), b' = min(b, 0.95 − t)
//	x = min(2·round(l·W/2), max(0, W − 2)), w = min(W − x, max(2, W − x − 2·round(r'·W/2)))
//
// and y and h likewise, so the rectangle always lies inside the picture: a 1×1 picture gives x 0 and w 1. Go's
// math.Round rounds halves away from zero; JS VideoStudioPicture.cropRect does the same. The fractions refer to the
// unflipped source (mirroredCrop).
func cropRect(crop Crop, width, height int) (x, y, w, h int) {
	r := math.Min(crop.R, maxCropFraction-crop.L)
	b := math.Min(crop.B, maxCropFraction-crop.T)
	x = min(2*int(math.Round(crop.L*float64(width)/2)), max(0, width-2))
	y = min(2*int(math.Round(crop.T*float64(height)/2)), max(0, height-2))
	w = min(width-x, max(2, width-x-2*int(math.Round(r*float64(width)/2))))
	h = min(height-y, max(2, height-y-2*int(math.Round(b*float64(height)/2))))
	return x, y, w, h
}

// mirroredCrop is the crop of a flipped picture: flip_h swaps l and r, flip_v swaps t and b. An animated crop runs
// after the flips, so it needs the mirrored fractions.
func mirroredCrop(crop Crop, flipH, flipV bool) Crop {
	if flipH {
		crop.L, crop.R = crop.R, crop.L
	}
	if flipV {
		crop.T, crop.B = crop.B, crop.T
	}
	return crop
}

// vignetteAngle is the FFmpeg vignette angle of amount and size on a width×height picture:
//
//	α = min(π/2, amount·(π/2)·R/(size·H)), with R = hypot(W/2, H/2)
//
// TestVignetteMatchesFormula (Task 9) confirms the cos⁴ falloff.
func vignetteAngle(amount, size float64, width, height int) float64 {
	if height <= 0 || !(size > 0) {
		return 0
	}
	radius := math.Hypot(float64(width)/2, float64(height)/2)
	return math.Min(math.Pi/2, amount*(math.Pi/2)*radius/(size*float64(height)))
}

// blurSigma is the Gaussian sigma in pixels of a blur radius (a share of the picture height; resolution rule
// §4.20).
func blurSigma(radius float64, height int) float64 {
	return radius * float64(height)
}

// effectValueAt is a number parameter of clip.Effects[index] at absolute timeline frame frame, through ValueAt
// (effects.<i>.<param>). It is 0 for an unknown entry or parameter.
func effectValueAt(clip Clip, index int, param string, frame int) float64 {
	value, _ := ValueAt(clip, "effects."+strconv.Itoa(index)+"."+param, frame)
	return value
}

// cropAnimated reports whether a crop side has keyframes. The effects then run on the whole picture, and the crop
// follows them in front of the transform (index §4.20).
func cropAnimated(clip Clip) bool {
	for _, path := range []string{"crop.l", "crop.t", "crop.r", "crop.b"} {
		if len(clip.Keyframes[path]) > 0 {
			return true
		}
	}
	return false
}

// clampUnit clamps v to 0..1; NaN becomes 0.
func clampUnit(v float64) float64 {
	if !(v > 0) {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// channelCurveExpr is the lutrgb expression of one colour channel (index §4.11):
//
//	clip(round(255*pow(clip((clip((val/255*M-0.5)*C+0.5,0,1)*(1-L)+L)*G,0,1),IG)),0,255)
//
// It follows steps 1–5 of §4.20 on the input level val, the same operations as ChannelCurve.apply. It is built
// through ffExpr:
//   - every infix is parenthesized;
//   - every number is FormatFloat('f', -1), negatives in parentheses;
//   - the only letters are clip, round, pow and val.
//
// The sendcmd writer checks the text against that grammar before it quotes it.
func channelCurveExpr(c ChannelCurve) ffExpr {
	level := exprOp("/", exprVar("val"), exprNum(255))
	contrast := exprOp("clip", exprOp("+", exprOp("*", exprOp("-", exprOp("*", level, exprNum(c.Mul)), exprNum(0.5)), exprNum(c.Contrast)), exprNum(0.5)), exprNum(0), exprNum(1))
	lifted := exprOp("+", exprOp("*", contrast, exprOp("-", exprNum(1), exprNum(c.Lift))), exprNum(c.Lift))
	gained := exprOp("clip", exprOp("*", lifted, exprNum(c.Gain)), exprNum(0), exprNum(1))
	return exprOp("clip", exprOp("round", exprOp("*", exprNum(255), exprOp("pow", gained, exprNum(c.InvGamma)))), exprNum(0), exprNum(255))
}
