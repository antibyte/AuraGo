package videostudio

import (
	"math"
	"strconv"
)

// Compositing math of plan 2d (plan index §4.22, normative): blend modes and the composite, the mask law L and the
// chroma key. Each part exists in two forms:
//   - The exact 8-bit forms of FFmpeg's blend, maskedmerge, colorkey and despill filters: blend8, mix8, keyAlpha8 and
//     despill8. The spike of 2026-10-17 measured them with 0 error on FFmpeg 5.1.9 and 8.1.1; the gated render tests
//     compare exports against them at 0 levels. The mask merge (blend c3_mode=multiply) is
//     blend8("multiply", alpha, m8) = alpha·m8//255.
//   - The real-valued forms that the GL preview evaluates: blendChannel, compositeChannel, maskAt, keyAlpha and
//     despillRGB. JS VideoStudioPicture (video-studio-picture.js) computes the same numbers (parity topics blend, mask
//     and key). They differ from the 8-bit forms by the truncations only: softlight up to 4 levels, the other modes up
//     to 2, the key up to 1.
//
// Colours are 0..1 in the real forms and 0..255 in the 8-bit forms. below is the opaque composite under a layer, layer
// the clip's straight colour, alpha its alpha (picture alpha · key · mask · opacity).

// blendChannel is one channel of a blend mode in the real form, below and layer in 0..1 (index §4.22 "GL form"). normal
// and unknown modes give the layer.
func blendChannel(mode string, below, layer float64) float64 {
	a, b := below, layer
	switch mode {
	case "multiply":
		return a * b
	case "screen":
		return 1 - (1-a)*(1-b)
	case "overlay":
		if 255*a < 127.5 {
			return 2 * a * b
		}
		return 1 - 2*(1-a)*(1-b)
	case "hardlight":
		if 255*b < 127.5 {
			return 2 * a * b
		}
		return 1 - 2*(1-a)*(1-b)
	case "softlight":
		return a*a + 2*b*a*(1-a)
	case "darken":
		return math.Min(a, b)
	case "lighten":
		return math.Max(a, b)
	case "difference":
		return math.Abs(a - b)
	case "exclusion":
		return a + b - 2*a*b
	case "addition":
		return math.Min(1, a+b)
	case "subtract":
		return math.Max(0, a-b)
	case "dodge":
		if b >= 1 {
			return 1
		}
		return math.Min(1, 256.0/255*a/(1-b))
	case "burn":
		if b <= 0 {
			return 0
		}
		return math.Max(0, 1-256.0/255*(1-a)/b)
	}
	return b
}

// compositeChannel is one channel of a layer composited onto the opaque composite below with a mode:
// below + (blendChannel − below)·alpha. With normal it is the straight-alpha mix of the export's overlay.
func compositeChannel(mode string, below, layer, alpha float64) float64 {
	return below + (blendChannel(mode, below, layer)-below)*alpha
}

// blend8 is FFmpeg's 8-bit blend formula of a mode (c0..c2_mode on gbrp, index §4.22). A is the first input, B the
// second: [below][layer], or [layer][below] for the modes in blendSwapped. // is integer floor:
//   - multiply A·B//255; screen 255 − (255−A)(255−B)//255;
//   - overlay A < 128 ? 2·(A·B//255) : 255 − 2·((255−A)(255−B)//255); hardlight the same with the branch on B;
//   - softlight A·A//255 + 2·(B·(A(255−A)//255)//255) (Pegtop with truncated products), clipped to 255;
//   - darken min, lighten max, difference |A−B|, exclusion A + B − 2AB//255, addition min(255, A+B), subtract
//     max(0, A−B);
//   - dodge A = 255 ? 255 : min(255, B·256//(255−A)); burn A = 0 ? 0 : max(0, 255 − (255−B)·256//A).
//
// normal and unknown modes give the layer.
func blend8(mode string, below, layer uint8) uint8 {
	a, b := int(below), int(layer)
	if blendSwapped[mode] {
		a, b = b, a
	}
	var v int
	switch mode {
	case "multiply":
		v = a * b / 255
	case "screen":
		v = 255 - (255-a)*(255-b)/255
	case "overlay", "hardlight":
		dark := a < 128
		if mode == "hardlight" {
			dark = b < 128
		}
		if dark {
			v = 2 * (a * b / 255)
		} else {
			v = 255 - 2*((255-a)*(255-b)/255)
		}
	case "softlight":
		v = min(255, a*a/255+2*(b*(a*(255-a)/255)/255))
	case "darken":
		v = min(a, b)
	case "lighten":
		v = max(a, b)
	case "difference":
		v = max(a-b, b-a)
	case "exclusion":
		v = a + b - 2*a*b/255
	case "addition":
		v = min(255, a+b)
	case "subtract":
		v = max(0, a-b)
	case "dodge":
		v = 255
		if a < 255 {
			v = min(255, b*256/(255-a))
		}
	case "burn":
		v = 0
		if a > 0 {
			v = max(0, 255-(255-b)*256/a)
		}
	default:
		v = b
	}
	return uint8(v)
}

// mix8 is the export's composite of a non-normal mode: maskedmerge with base below, overlay the blend output and mask
// the layer alpha, (below·(255 − α) + blended·α + 127)//255. It is exactly below at α = 0 and exactly blended at 255.
func mix8(below, blended, alpha uint8) uint8 {
	return uint8((int(below)*(255-int(alpha)) + int(blended)*int(alpha) + 127) / 255)
}

// overlapBlend is the effective mode of the overlap frames of a transition from a clip in mode outgoing to one in mode
// incoming (spec §6.2, index §4.22): the shared mode, else normal. An empty name is normal.
func overlapBlend(outgoing, incoming string) string {
	if outgoing == "" {
		outgoing = BlendNormal
	}
	if incoming == "" {
		incoming = BlendNormal
	}
	if outgoing == incoming {
		return outgoing
	}
	return BlendNormal
}

// maskFrame is a mask resolved for one frame in box pixels (law L, index §4.22): the box is W×H px, x right, y down.
// It does not know the pad canvas of keyframed geometry; the export offsets CX and CY by ((PW − bw)/2, (PH − bh)/2)
// there.
type maskFrame struct {
	CX, CY   float64   // the mask centre
	RX, RY   float64   // half width and half height of the unrotated mask
	Cos, Sin float64   // of θ = rotation·π/180, positive = clockwise on screen
	Ramp     float64   // F = max(feather·H, 1): the full width of the linear edge, centred on the outline
	Shape    MaskShape // anything but ellipse draws a rectangle
	Invert   bool
}

// maskFrameOf places a mask in a box of boxWidth×boxHeight px:
//
//	c = ((x + w/2)·W, (y + h/2)·H), rx = w·W/2, ry = h·H/2, F = max(feather·H, 1)
func maskFrameOf(mask Mask, boxWidth, boxHeight float64) maskFrame {
	theta := mask.Rotation * math.Pi / 180
	return maskFrame{
		CX: (mask.X + mask.W/2) * boxWidth, CY: (mask.Y + mask.H/2) * boxHeight,
		RX: mask.W * boxWidth / 2, RY: mask.H * boxHeight / 2,
		Cos: math.Cos(theta), Sin: math.Sin(theta),
		Ramp:  math.Max(mask.Feather*boxHeight, 1),
		Shape: mask.Shape, Invert: mask.Invert,
	}
}

// maskFrameAt resolves a clip's mask at absolute timeline frame frame in a box of boxWidth×boxHeight px: every number
// through ValueAt (mask keys at the source position, else the field), shape and invert from the mask. ok is false
// without a mask.
func maskFrameAt(clip Clip, frame, boxWidth, boxHeight int) (maskFrame, bool) {
	if clip.Mask == nil {
		return maskFrame{}, false
	}
	mask := *clip.Mask
	value := func(path string) float64 {
		v, _ := ValueAt(clip, path, frame)
		return v
	}
	mask.X, mask.Y, mask.W, mask.H = value("mask.x"), value("mask.y"), value("mask.w"), value("mask.h")
	mask.Rotation, mask.Feather = value("mask.rotation"), value("mask.feather")
	return maskFrameOf(mask, float64(boxWidth), float64(boxHeight)), true
}

// maskAt is law L at the box point (x, y), usually a pixel centre (i + 0.5, j + 0.5):
//
//	(dx, dy) = p − c; lx = dx·cosθ + dy·sinθ; ly = −dx·sinθ + dy·cosθ
//	rect:    qx = |lx| − rx, qy = |ly| − ry; s = hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0)   (exact)
//	ellipse: u = lx/rx, v = ly/ry, d = max(hypot(u, v), 0.000001); s = (d − 1)·d / max(sqrt(u²/rx² + v²/ry²), 1e-12)   (first order)
//	m = clip(0.5 − s/F, 0, 1), and 1 − m when inverted
//
// s is the signed distance in px, positive outside. The ellipse's gradient length is floored at 1e-12 as in the GL
// shader, so the exact centre gives a large negative s and m = 1, never NaN or Inf. The operations follow
// the spike's geq expression, so the export's expression (Task 6) evaluates to the same numbers.
func maskAt(m maskFrame, x, y float64) float64 {
	dx, dy := x-m.CX, y-m.CY
	lx := dx*m.Cos + dy*m.Sin
	ly := -dx*m.Sin + dy*m.Cos
	var s float64
	if m.Shape == MaskEllipse {
		u, v := lx/m.RX, ly/m.RY
		d := math.Max(math.Hypot(u, v), 0.000001)
		s = (d - 1) * d / math.Max(math.Sqrt(u*u/(m.RX*m.RX)+v*v/(m.RY*m.RY)), 1e-12)
	} else {
		qx, qy := math.Abs(lx)-m.RX, math.Abs(ly)-m.RY
		s = math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0)
	}
	value := clampUnit(0.5 - s/m.Ramp)
	if m.Invert {
		return 1 - value
	}
	return value
}

// maskAnimated reports mask.* keyframes on a clip.
func maskAnimated(clip Clip) bool {
	for _, path := range maskPaths {
		if len(clip.Keyframes[path]) > 0 {
			return true
		}
	}
	return false
}

// maskPerFrame reports whether a clip's mask changes over time: mask keys, or animated box geometry (clipAnimation, which
// 2e extends to in/out animation). The export then uses the per-frame generator on the pad canvas. False without a
// mask.
func maskPerFrame(clip Clip) bool {
	if clip.Mask == nil {
		return false
	}
	geometry, _ := clipAnimation(clip)
	return geometry || maskAnimated(clip)
}

// keyParams are the values of a chroma_key entry at one frame.
type keyParams struct {
	Color      [3]uint8 // the key colour
	Similarity float64  // colorkey similarity, 0.01..1
	Smoothness float64  // colorkey blend, 0..1; 0 (below 0.0001) is the hard step
	Spill      float64  // despill strength, 0..1; 0 leaves despill out
}

// keyParamsAt evaluates the chroma_key entry clip.Effects[index] at absolute timeline frame frame: similarity,
// smoothness and spill through effectValueAt (keys, params, registry default), the colour from params when it is a
// #rrggbb string, else the registry default.
func keyParamsAt(clip Clip, index, frame int) keyParams {
	hex := ""
	if spec, ok := effectParamSpec("chroma_key", "color"); ok {
		hex, _ = spec.Default.(string)
	}
	if index >= 0 && index < len(clip.Effects) {
		if value, ok := clip.Effects[index].Params["color"].(string); ok && validHexColor(value) {
			hex = value
		}
	}
	return keyParams{
		Color:      keyColor(hex),
		Similarity: effectValueAt(clip, index, "similarity", frame),
		Smoothness: effectValueAt(clip, index, "smoothness", frame),
		Spill:      effectValueAt(clip, index, "spill", frame),
	}
}

// keyColor parses a #rrggbb colour (either case). Anything else is black; callers pass validated colours.
func keyColor(hex string) [3]uint8 {
	if !validHexColor(hex) {
		return [3]uint8{}
	}
	value, _ := strconv.ParseUint(hex[1:], 16, 32)
	return [3]uint8{uint8(value >> 16), uint8(value >> 8), uint8(value)}
}

// spillBlue reports despill type blue for a key colour: blue when its blue exceeds its green, else green (despill
// knows only these two).
func spillBlue(color [3]uint8) bool {
	return color[2] > color[1]
}

// keyAlpha8 is FFmpeg colorkey's alpha for one pixel (index §4.22), with the filter's float32 similarity and
// iblend = 1/blend:
//
//	d = sqrt((dr² + dg² + db²)/(255²·3)); iblend < 10000 ? trunc(255·clip((d − similarity)·iblend, 0, 1)) : (d > similarity ? 255 : 0)
//
// colorkey leaves the colour untouched.
func keyAlpha8(p keyParams, rgb [3]uint8) uint8 {
	dr := float64(rgb[0]) - float64(p.Color[0])
	dg := float64(rgb[1]) - float64(p.Color[1])
	db := float64(rgb[2]) - float64(p.Color[2])
	diff := math.Sqrt((dr*dr + dg*dg + db*db) / (255.0 * 255.0 * 3.0))
	similarity := float64(float32(p.Similarity))
	iblend := float32(1) / float32(p.Smoothness)
	if iblend < 10000 {
		return uint8(clampUnit((diff-similarity)*float64(iblend)) * 255)
	}
	if diff > similarity {
		return 255
	}
	return 0
}

// despill8 is FFmpeg despill as the export configures it (index §4.22): type from the key colour (spillBlue), mix 0.5,
// expand 0, the screen channel's scale −spill, the other scales 0, brightness 0, alpha off. In float32 like the filter:
//
//	green: S = max(g − (r·0.5 + b·0.5), 0); g' = max(g − spill·S, 0)   (blue: the same with b and g swapped)
//
// and every channel leaves as trunc(255·c). Explicit float32 conversions keep Go from fusing a product into an add.
// Spill 0 leaves the pixel unchanged (the export omits despill).
func despill8(p keyParams, rgb [3]uint8) [3]uint8 {
	if !(p.Spill > 0) {
		return rgb
	}
	const mix = float32(0.5)
	factor := float32((1 - mix) * (1 - 0)) // (1 − mix)·(1 − expand) with expand 0
	scale := float32(-p.Spill)
	r, g, b := float32(rgb[0])/255, float32(rgb[1])/255, float32(rgb[2])/255
	level := func(c float32) uint8 { return uint8(min(max(int(float32(c*255)), 0), 255)) }
	if spillBlue(p.Color) {
		spill := max(b-float32(float32(r*mix)+float32(g*factor)), 0)
		b = max(b+float32(spill*scale), 0)
	} else {
		spill := max(g-float32(float32(r*mix)+float32(b*factor)), 0)
		g = max(g+float32(spill*scale), 0)
	}
	return [3]uint8{level(r), level(g), level(b)}
}

// keyAlpha is the real form of keyAlpha8, colours in 0..1: d = sqrt(Σ(c − key)²/3);
// 1/smoothness < 10000 ? clip((d − similarity)/smoothness, 0, 1) : (d > similarity ? 1 : 0).
func keyAlpha(p keyParams, rgb [3]float64) float64 {
	sum := 0.0
	for c := range rgb {
		d := rgb[c] - float64(p.Color[c])/255
		sum += d * d
	}
	d := math.Sqrt(sum / 3)
	if 1/p.Smoothness < 10000 {
		return clampUnit((d - p.Similarity) / p.Smoothness)
	}
	if d > p.Similarity {
		return 1
	}
	return 0
}

// despillRGB is the real form of despill8, colours in 0..1.
func despillRGB(p keyParams, rgb [3]float64) [3]float64 {
	if !(p.Spill > 0) {
		return rgb
	}
	r, g, b := rgb[0], rgb[1], rgb[2]
	if spillBlue(p.Color) {
		s := math.Max(b-(0.5*r+0.5*g), 0)
		rgb[2] = math.Max(b-p.Spill*s, 0)
	} else {
		s := math.Max(g-(0.5*r+0.5*b), 0)
		rgb[1] = math.Max(g-p.Spill*s, 0)
	}
	return rgb
}
