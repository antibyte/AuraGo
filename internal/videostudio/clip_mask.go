package videostudio

import (
	"fmt"
	"math"
)

// The 2d mask stage (index §4.10, §4.22). A visual clip's mask is law L, evaluated in box space by geq into a gbrap
// stream (r = g = b = 0, alpha = 255·m). blend with c3_mode=multiply merges it into the clip's alpha, between fit and
// orient. There are two forms:
//   - static (no mask.* key, static box geometry): one full-resolution geq frame of Go numbers, repeated by loop;
//   - keyframed (a mask key, or keyframed box geometry): geq per frame in T at a quarter of the canvas per side, then a
//     bilinear upscale. The window start is baked into T (windowSourceExprVar), so every segmentation evaluates the
//     same absolute frames.
//
// The branch never uses sendcmd or gblur. Its expressions hold only Go-written numbers, keyframe curves and geq's X, Y
// and T.
//
// Register allocation of the keyframed expression. FFmpeg has ten registers, 0–9: curveExpr writes 3 and 4, and the
// window source writes 0 and 1 (2 and 3 with a ramp).
//
//	stage A, the frame's quantities (recomputed per pixel, because geq keeps nothing between pixels):
//	  5 = box width bw, 6 = box height bh      (box expressions in T, which read their own window source)
//	  0, 1 (2, 3 with a ramp) = window source   (only when a mask path has keys)
//	  7 = θ, 8 = F, 9 = w then rx, 2 = cx, 0 = h then ry, 5 = cy, 1 = cos θ, 7 = sin θ
//	stage B, the pixel (maskPixelExpr, shared with the static form, whose stage A is Go numbers):
//	  3 = dx, 4 = dy then ly, 6 = lx then s, 2 = qx or u, 5 = qy or v, 3 = d (ellipse)

// maskPlan is the layout of a clip's mask stream (index §4.22):
//   - perFrame: the keyframed form (geq per frame in T at quarter resolution), else the static form;
//   - the canvas the fit leaves, which the mask stream matches: the box BW×BH for static geometry, or the pad canvas
//     PW×PH with the box centred for keyframed geometry;
//   - the geq grid: the canvas for the static form, ceil(canvas/4) per side for the keyframed one;
//   - the box size in canvas px as the keyframed expression reads it: Go numbers for static geometry, or for keyframed
//     geometry the window's box expressions (the ones scale and pad evaluate in t), in T.
type maskPlan struct {
	perFrame                  bool
	canvasWidth, canvasHeight int
	gridWidth, gridHeight     int
	boxWidth, boxHeight       ffExpr
}

// maskPlanOf lays out the mask stream of clip chain c, which has a mask.
func maskPlanOf(c clipChain) maskPlan {
	plan := maskPlan{perFrame: maskPerFrame(c.item.whole)}
	if geometry, _ := clipAnimation(c.item.whole); geometry {
		g := c.geometry()
		plan.perFrame = true
		plan.canvasWidth, plan.canvasHeight = g.padWidth, g.padHeight
		plan.boxWidth, plan.boxHeight = renameExprVar(g.box.w, "t", "T"), renameExprVar(g.box.h, "t", "T")
	} else {
		_, _, plan.canvasWidth, plan.canvasHeight = clipBox(c.item.clip, c.canvasWidth, c.canvasHeight)
		plan.boxWidth, plan.boxHeight = exprNum(float64(plan.canvasWidth)), exprNum(float64(plan.canvasHeight))
	}
	plan.gridWidth, plan.gridHeight = plan.canvasWidth, plan.canvasHeight
	if plan.perFrame {
		plan.gridWidth, plan.gridHeight = (plan.canvasWidth+3)/4, (plan.canvasHeight+3)/4
	}
	return plan
}

// maskTerms are law L's frame quantities in canvas px as stage B reads them: Go numbers (static form) or registers
// (keyframed form).
type maskTerms struct {
	cx, cy, rx, ry, cos, sin, ramp ffExpr
}

// maskPixelExpr is stage B of law L at canvas px (px, py): 255·m, with m = clip(0.5 − s/F, 0, 1), 1 − m when inverted.
//   - The rect distance is exact.
//   - The ellipse distance is first order: (d − 1)·d / max(sqrt(u²/rx² + v²/ry²), 1e-12), with
//     d = max(hypot(u, v), 0.000001). The floor keeps the exact ellipse centre finite (m = 1), as in maskAt and GL.
//
// clip's first argument has no st(), because libavutil evaluates it twice.
func maskPixelExpr(shape MaskShape, invert bool, px, py ffExpr, t maskTerms) ffExpr {
	parts := []ffExpr{
		exprSt(3, exprOp("-", px, t.cx)),
		exprSt(4, exprOp("-", py, t.cy)),
		exprSt(6, exprOp("+", exprOp("*", exprLd(3), t.cos), exprOp("*", exprLd(4), t.sin))),
		exprSt(4, exprOp("-", exprOp("*", exprLd(4), t.cos), exprOp("*", exprLd(3), t.sin))),
	}
	if shape == MaskEllipse {
		parts = append(parts,
			exprSt(2, exprOp("/", exprLd(6), t.rx)),
			exprSt(5, exprOp("/", exprLd(4), t.ry)),
			exprSt(3, exprOp("max", exprOp("hypot", exprLd(2), exprLd(5)), exprNum(0.000001))),
			exprSt(6, exprOp("/", exprOp("*", exprOp("-", exprLd(3), exprNum(1)), exprLd(3)),
				exprOp("max", exprOp("sqrt", exprOp("+",
					exprOp("/", exprOp("*", exprLd(2), exprLd(2)), exprOp("*", t.rx, t.rx)),
					exprOp("/", exprOp("*", exprLd(5), exprLd(5)), exprOp("*", t.ry, t.ry)))), exprNum(1e-12)))))
	} else {
		parts = append(parts,
			exprSt(2, exprOp("-", exprOp("abs", exprLd(6)), t.rx)),
			exprSt(5, exprOp("-", exprOp("abs", exprLd(4)), t.ry)),
			exprSt(6, exprOp("+",
				exprOp("hypot", exprOp("max", exprLd(2), exprNum(0)), exprOp("max", exprLd(5), exprNum(0))),
				exprOp("min", exprOp("max", exprLd(2), exprLd(5)), exprNum(0)))))
	}
	m := exprOp("clip", exprOp("-", exprNum(0.5), exprOp("/", exprLd(6), t.ramp)), exprNum(0), exprNum(1))
	if invert {
		m = exprOp("-", exprNum(1), m)
	}
	return exprSeq(append(parts, exprOp("*", exprNum(255), m))...)
}

// maskExpr is law L (index §4.22) as the geq expression of the mask's alpha plane: 255·m at the geq pixel (X, Y).
//   - Static form: the pixel centre is (X + 0.5, Y + 0.5) in box px, and every frame quantity is a Go number:
//     c = (W/2 + (x + w/2 − 0.5)·W, H/2 + (y + h/2 − 0.5)·H), rx = w·W/2, ry = h·H/2, F = max(feather·H, 1).
//   - Keyframed form: the pixel centre is ((X + 0.5)·CW/GW, (Y + 0.5)·CH/GH) on the canvas. The box of bw(T)×bh(T)
//     sits centred in it: c = (CW/2 + (x + w/2 − 0.5)·bw, CH/2 + (y + h/2 − 0.5)·bh), rx = w·bw/2, ry = h·bh/2,
//     F = max(feather·bh, 1). Keyed mask paths are curveExpr at the window's source position in T.
func maskExpr(c clipChain, plan maskPlan) ffExpr {
	whole := c.item.whole
	mask := *whole.Mask
	px := exprOp("+", exprVar("X"), exprNum(0.5))
	py := exprOp("+", exprVar("Y"), exprNum(0.5))
	if !plan.perFrame {
		width, height := float64(plan.canvasWidth), float64(plan.canvasHeight)
		theta := mask.Rotation * math.Pi / 180
		return maskPixelExpr(mask.Shape, mask.Invert, px, py, maskTerms{
			cx:   exprNum(width/2 + (mask.X+mask.W/2-0.5)*width),
			cy:   exprNum(height/2 + (mask.Y+mask.H/2-0.5)*height),
			rx:   exprNum(mask.W * width / 2),
			ry:   exprNum(mask.H * height / 2),
			cos:  exprNum(math.Cos(theta)),
			sin:  exprNum(math.Sin(theta)),
			ramp: exprNum(math.Max(mask.Feather*height, 1)),
		})
	}
	if plan.gridWidth != plan.canvasWidth {
		px = exprOp("*", px, exprNum(float64(plan.canvasWidth)/float64(plan.gridWidth)))
	}
	if plan.gridHeight != plan.canvasHeight {
		py = exprOp("*", py, exprNum(float64(plan.canvasHeight)/float64(plan.gridHeight)))
	}
	value := func(path string, static float64) ffExpr {
		if keys := whole.Keyframes[path]; len(keys) > 0 {
			return curveExpr(keys, exprLd(1))
		}
		return exprNum(static)
	}
	parts := []ffExpr{exprSt(5, plan.boxWidth), exprSt(6, plan.boxHeight)}
	if maskAnimated(whole) {
		parts = append(parts, windowSourceExprVar(whole, c.item.at, c.item.pieces, c.rate, "T"))
	}
	parts = append(parts,
		exprSt(7, exprOp("*", value("mask.rotation", mask.Rotation), exprNum(math.Pi/180))),
		exprSt(8, exprOp("max", exprOp("*", value("mask.feather", mask.Feather), exprLd(6)), exprNum(1))),
		exprSt(9, value("mask.w", mask.W)),
		exprSt(2, exprOp("+", exprNum(float64(plan.canvasWidth)/2),
			exprOp("*", exprOp("-", exprOp("+", value("mask.x", mask.X), exprOp("/", exprLd(9), exprNum(2))), exprNum(0.5)), exprLd(5)))),
		exprSt(9, exprOp("/", exprOp("*", exprLd(9), exprLd(5)), exprNum(2))),
		exprSt(0, value("mask.h", mask.H)),
		exprSt(5, exprOp("+", exprNum(float64(plan.canvasHeight)/2),
			exprOp("*", exprOp("-", exprOp("+", value("mask.y", mask.Y), exprOp("/", exprLd(0), exprNum(2))), exprNum(0.5)), exprLd(6)))),
		exprSt(0, exprOp("/", exprOp("*", exprLd(0), exprLd(6)), exprNum(2))),
		exprSt(1, exprOp("cos", exprLd(7))),
		exprSt(7, exprOp("sin", exprLd(7))),
		maskPixelExpr(mask.Shape, mask.Invert, px, py, maskTerms{
			cx: exprLd(2), cy: exprLd(5), rx: exprLd(9), ry: exprLd(0), cos: exprLd(1), sin: exprLd(7), ramp: exprLd(8),
		}))
	return exprSeq(parts...)
}

// staticMaskGenerator renders the static form once at full resolution and repeats that frame for the window (index
// §4.22). geq costs 0.36 s (8.1.1) / 0.50 s (5.1.9) at 1080p, once per process.
func staticMaskGenerator(c clipChain, plan maskPlan) string {
	return fmt.Sprintf("nullsrc=s=%dx%d:r=%s:d=%s,format=gbrap,geq=r='0':g='0':b='0':a='%s',loop=loop=-1:size=1:start=0,trim=end_frame=%d,setpts=PTS-STARTPTS",
		plan.canvasWidth, plan.canvasHeight, c.rate, c.rate.SecondsArg(1), maskExpr(c, plan).String(), c.item.clip.Duration)
}

// framedMaskGenerator renders the keyframed form for every window frame at quarter resolution and upscales it
// bilinearly to the canvas (index §4.22, risk 19: 16 ms (8.1.1) / 52 ms (5.1.9) per 1080p frame with linear
// parameters). The scale's input never changes size, so the 5.1 t-term rule does not apply.
func framedMaskGenerator(c clipChain, plan maskPlan) string {
	frames := c.item.clip.Duration
	return fmt.Sprintf("nullsrc=s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=gbrap,geq=r='0':g='0':b='0':a='%s',scale=w=%d:h=%d:flags=bilinear",
		plan.gridWidth, plan.gridHeight, c.rate, c.rate.SecondsArg(frames), frames, maskExpr(c, plan).String(), plan.canvasWidth, plan.canvasHeight)
}

// maskStage is the clip's mask stage between fit and orient, {} without a mask (index §4.10). The clip enters gbrap
// natively: a packed rgba input converts here, the 2c-tolerated 5.1 alpha step. Then blend multiplies the mask's alpha in:
// RGB untouched, alpha = a·m8//255 (exactly 0 at m8 = 0 and exactly a at 255). It is a two-input stage, so the opacity
// commands read the next command group.
func maskStage(c clipChain) chainStage {
	if c.item.whole.Mask == nil {
		return chainStage{}
	}
	plan := maskPlanOf(c)
	generator := staticMaskGenerator(c, plan)
	if plan.perFrame {
		generator = framedMaskGenerator(c, plan)
	}
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	in, mask := "vs_mk_"+id+"_in", "vs_mk_"+id+"_m"
	return chainStage{filter: fmt.Sprintf("format=gbrap[%s];%s[%s];[%s][%s]blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply",
		in, generator, mask, in, mask), twoInput: true}
}
