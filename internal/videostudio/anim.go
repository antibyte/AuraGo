package videostudio

import "math"

// In/out animations (plan 2e, spec §5.6, index §4.24).
//   - Clip.AnimIn and Clip.AnimOut hold a registry preset and a duration in timeline frames.
//   - The duration lies in AnimFrameRange of the project rate.
//   - The render clamps each span to half the clip (animSpans).
//
// JS VideoStudioAnim.SECONDS and frameRange mirror these numbers (TestVideoStudioAnimConstantsMatchGo). The preset
// constants and the animation math follow AnimFrameRange.

const (
	// AnimMinSeconds and AnimMaxSeconds bound an animation's duration. AnimDefaultSeconds is the length of a new one.
	AnimMinSeconds     = 0.2
	AnimMaxSeconds     = 3
	AnimDefaultSeconds = 0.5
)

// AnimFrameRange is the duration range of an in or out animation in timeline frames at rate, and the default length:
//   - the range is [max(1, round(0.2·fps)), round(3·fps)];
//   - the default is round(0.5·fps);
//   - fps = Num/Den, rounded half away from zero (math.Round).
//
// For example, 25 fps gives 5..75 (default 13), and 30 and 29.97 fps give 6..90 (default 15). An unset rate is the
// 30 fps default, as in ProjectRate.
func AnimFrameRange(rate Rate) (minFrames, maxFrames, defaultFrames int) {
	if rate.Num <= 0 || rate.Den <= 0 {
		rate = Rate{Num: FramesPerSecond, Den: 1}
	}
	frames := func(seconds float64) int { return int(math.Round(seconds * float64(rate.Num) / float64(rate.Den))) }
	return max(1, frames(AnimMinSeconds)), frames(AnimMaxSeconds), frames(AnimDefaultSeconds)
}

// ---- Plan 2e Task 4: the animation model (spec §5.6, index §4.24) ----
//
// An in or out animation plays over its effective span at the clip's start or end, in timeline frames:
//   - animSpans: the stored duration, at most half the clip, so the spans never overlap;
//   - speed and ramps change neither the spans nor the progress.
//
// p is the rest progress: 1 at rest, 0 on the clip's first frame (in) and last frame (out), which are fully hidden.
//
// The animation applies on top of the keyed values and never changes them:
//   - the opacity multiplies;
//   - the offset moves the box centre after clipBox's even clamp;
//   - the scale sizes the box about that centre (animBox).
//
// JS VideoStudioAnim (video-studio-anim.js) computes the same numbers (parity topic anim).
//
// Every product that feeds a sum below is converted explicitly (float64(…)). No fused multiply-add can then change a
// value against the export's expressions (animExprs, Task 6) or JS.

// The presets' numbers. TestVideoStudioAnimConstantsMatchGo (ui) reads each `name = <decimal>` line of this block.
// animPopRise, animPopSettle and animPopTail are the pop curve's spans written out: Go folds constant differences
// exactly, while a JS subtraction such as 1.08 − 0.6 gives a neighbouring double.
const (
	animSlideDistance = 0.15 // a slide's offset at p = 0, as a share of the canvas side it runs along
	animPopFrom       = 0.6  // pop scale at p = 0
	animPopPeak       = 1.08 // pop scale at p = animPopPeakAt
	animPopPeakAt     = 0.7
	animPopFadeUntil  = 0.3  // pop opacity = min(1, p/0.3)
	animPopRise       = 0.48 // animPopPeak − animPopFrom
	animPopSettle     = 0.08 // animPopPeak − 1
	animPopTail       = 0.3  // 1 − animPopPeakAt
	animBlurRadius    = 0.02 // blur_in: S = 0.02 · the processed picture height (index §4.20's resolution rule)
	animBlurNode      = 0.75 // blur_in: the progress at which the cascade shows g(S/4) alone
)

// animSlideAxes are the slide presets' unit directions u_d in canvas axes (x right, y down).
var animSlideAxes = map[string][2]int{"slide_up": {0, -1}, "slide_down": {0, 1}, "slide_left": {-1, 0}, "slide_right": {1, 0}}

// animState is a clip's in/out animation at one timeline frame (animAt):
//   - Opacity multiplies the keyed opacity; DX and DY move the box centre by these shares of the canvas width and
//     height; Scale sizes the box about its centre.
//   - Blur is set inside a blur_in span; M1 and M2 are the cascade's mask values 0..255 (255 at rest: sharp).
//   - Wipe is 1 inside a wipe's in span, −1 inside its out span, else 0.
//   - P is the rest progress of the active side: 1 at rest, 0 on the clip's first (in) and last (out) frame.
//
// animRest is the state outside both spans; the zero value is not.
type animState struct {
	Opacity, DX, DY, Scale float64
	Blur                   bool
	M1, M2                 int
	Wipe                   int
	P                      float64
}

var animRest = animState{Opacity: 1, Scale: 1, M1: 255, M2: 255, P: 1}

// animKinds says what a clip's effective in/out animations change (animKindsOf).
type animKinds struct{ geometry, opacity, wipe, blur bool }

// animSpans returns the effective span lengths in timeline frames (index §4.24): the stored duration, at most half the
// clip rounded down, so the in and out spans never overlap. 0 means no animation on that side. Validate keeps every
// stored duration inside AnimFrameRange; the clip length never makes a project invalid.
func animSpans(clip Clip) (in, out int) {
	half := max(clip.Duration, 0) / 2
	if a := clip.AnimIn; a != nil && a.Duration > 0 {
		in = min(a.Duration, half)
	}
	if a := clip.AnimOut; a != nil && a.Duration > 0 {
		out = min(a.Duration, half)
	}
	return in, out
}

// animAt is a clip's in/out animation at absolute timeline frame frame (index §4.24). With j = frame − Start:
//   - j < D_in: the in preset at p = j/D_in;
//   - j ≥ Duration − D_out: the out preset at p = (Duration − 1 − j)/D_out;
//   - otherwise, and outside the clip, animRest.
//
// JS: VideoStudioAnim.at (null at rest).
func animAt(clip Clip, frame int) animState {
	in, out := animSpans(clip)
	j := frame - clip.Start
	switch {
	case j < 0 || j >= clip.Duration:
		return animRest
	case j < in:
		return animPresetAt(clip.AnimIn.Preset, float64(j)/float64(in), 1)
	case j >= clip.Duration-out:
		return animPresetAt(clip.AnimOut.Preset, float64(clip.Duration-1-j)/float64(out), -1)
	}
	return animRest
}

// animPresetAt is one preset at rest progress p on side 1 (in) or −1 (out):
//   - fade: opacity p;
//   - slide_<d>: offset σ·(1 − e_out(p))·0.15·u_d with σ = −side, so in arrives moving along d and out leaves along d;
//     opacity p;
//   - pop: the S3 scale curve (animPopScale), opacity min(1, p/0.3);
//   - wipe: the side; the columns come from animRevealPx;
//   - blur_in: the cascade masks (animBlurMasks), opacity p.
//
// An unknown preset keeps the rest values.
func animPresetAt(preset string, p float64, side int) animState {
	a := animRest
	a.P = p
	if axis, ok := animSlideAxes[preset]; ok {
		shift := animSlideShift(p)
		a.DX, a.DY, a.Opacity = float64(-side*axis[0])*shift, float64(-side*axis[1])*shift, p
		return a
	}
	switch preset {
	case "fade":
		a.Opacity = p
	case "pop":
		a.Scale, a.Opacity = animPopScale(p), min(1, p/animPopFadeUntil)
	case "wipe":
		a.Wipe = side
	case "blur_in":
		a.Opacity, a.Blur = p, true
		a.M1, a.M2 = animBlurMasks(p)
	}
	return a
}

// animEaseOut is e_out(t) = 1 − (1 − t)³ in the order of Ease(EaseOut, t) and easeExpr: u = 1 − t, then 1 − (u·u)·u.
func animEaseOut(t float64) float64 {
	u := 1 - t
	return 1 - float64(float64(u*u)*u)
}

// animSlideShift is a slide's distance at progress p as a share of the canvas side: (1 − e_out(p))·0.15.
func animSlideShift(p float64) float64 { return (1 - animEaseOut(p)) * animSlideDistance }

// animPopScale is the pop curve of spike S3: 0.6 + 0.48·e_out(p/0.7) below p = 0.7, then 1.08 − 0.08·sm((p − 0.7)/0.3)
// with sm(u) = u²(3 − 2u).
func animPopScale(p float64) float64 {
	if p < animPopPeakAt {
		return animPopFrom + float64(animPopRise*animEaseOut(p/animPopPeakAt))
	}
	u := (p - animPopPeakAt) / animPopTail
	return animPopPeak - float64(animPopSettle*float64(float64(u*u)*(3-float64(2*u))))
}

// animBlurMasks are the blur_in cascade's mask values at progress p (clamped to 0..1), integers 0..255 (index §4.24):
// m1 = round(255·min(p/0.75, 1)) mixes g(S) into g(S/4), and m2 = round(255·max((p − 0.75)/0.25, 0)) then mixes the
// sharp picture in. At p = 1 both are 255. JS: VideoStudioAnim.blurMasks.
func animBlurMasks(p float64) (m1, m2 int) {
	p = min(max(p, 0), 1)
	return int(math.Round(255 * min(p/animBlurNode, 1))), int(math.Round(255 * max((p-animBlurNode)/(1-animBlurNode), 0)))
}

// animKindsOf reports what a clip's effective in/out animations change (index §4.10):
//   - geometry for slides and pop;
//   - opacity for every preset with an opacity part (fade, slides, pop, blur_in);
//   - the wipe and the blur cascade.
//
// A side whose span is 0 (animSpans) counts for nothing.
func animKindsOf(clip Clip) animKinds {
	in, out := animSpans(clip)
	var kinds animKinds
	for _, side := range []struct {
		anim *ClipAnim
		span int
	}{{clip.AnimIn, in}, {clip.AnimOut, out}} {
		if side.span == 0 {
			continue
		}
		_, slide := animSlideAxes[side.anim.Preset]
		switch {
		case slide || side.anim.Preset == "pop":
			kinds.geometry, kinds.opacity = true, true
		case side.anim.Preset == "fade":
			kinds.opacity = true
		case side.anim.Preset == "wipe":
			kinds.wipe = true
		case side.anim.Preset == "blur_in":
			kinds.blur, kinds.opacity = true, true
		}
	}
	return kinds
}

// animBox is the exact box of index §4.24 in project px. It returns the centre (cx, cy) and the size (bw, bh). x, y, w
// and h are the keyed clipBox:
//   - bw = max(2, 2·round(w·Scale/2)) and bh likewise, about the box centre, with no canvas cap;
//   - cx = x + w/2 + 2·round(DX·canvasWidth/2), cy likewise.
//
// The box may leave or exceed the canvas. JS: VideoStudioAnim.box, which returns {x: cx − bw/2, y: cy − bh/2, w, h}.
func animBox(x, y, w, h int, a animState, canvasWidth, canvasHeight int) (cx, cy, bw, bh int) {
	bw = max(2, int(2*math.Round(float64(w)*a.Scale/2)))
	bh = max(2, int(2*math.Round(float64(h)*a.Scale/2)))
	cx = x + w/2 + int(2*math.Round(a.DX*float64(canvasWidth)/2))
	cy = y + h/2 + int(2*math.Round(a.DY*float64(canvasHeight)/2))
	return cx, cy, bw, bh
}

// animRevealPx is the range [x0, x1) of box columns a wipe shows on a box boxWidth px wide, with R = round(P·boxWidth).
// In shows [0, R) and out shows [boxWidth − R, boxWidth), so both read left to right. Without a wipe it is the whole
// box. JS: VideoStudioAnim.revealPx.
func animRevealPx(a animState, boxWidth int) (x0, x1 int) {
	reveal := int(math.Round(a.P * float64(boxWidth)))
	switch a.Wipe {
	case 1:
		return 0, reveal
	case -1:
		return boxWidth - reveal, boxWidth
	}
	return 0, boxWidth
}

// ---- Plan 2e Task 6: the animation's geometry as export expressions (index §4.10, §4.24) ----

// animExpr is the geometry of a clip's in/out animation as expressions of the absolute frame (animExprs): the box scale
// and the centre offsets dx and dy as shares of the canvas width and height.
type animExpr struct{ scale, dx, dy ffExpr }

// animExprs is animAt's geometry as expressions of frame, an expression of the absolute timeline frame. boxExprs passes
// register 0, which windowSourceExpr fills with T = at + round(max(t,0)·Num/Den). Each part is a tree over the spans:
//
//	if(lt(frame, Start + D_in), (st(7, p_in); in term), if(lt(frame, Start + Duration − D_out), rest, (st(7, p_out); out term)))
//
// with p_in = (frame − Start)/D_in and p_out = (Start + Duration − 1 − frame)/D_out.
//   - The terms repeat animPresetAt's operations in the same order: pop keeps smoothstep's input in register 8, and
//     ease_out keeps its inner term in register 4 (easeExpr). Go's eval and FFmpeg therefore give animBox's numbers
//     exactly.
//   - A part that no effective preset moves is its rest value as a literal (scale 1, offsets 0).
//   - A side whose span is 0 adds no branch.
//
// Every register is stored before it is read, so a filter that keeps its registers from frame to frame (scale, overlay)
// or from row to row (the mask's geq) reads nothing stale.
func animExprs(clip Clip, frame ffExpr) animExpr {
	in, out := animSpans(clip)
	p := exprLd(7)
	slide := func(axis int) func(preset string, sigma float64) (ffExpr, bool) {
		return func(preset string, sigma float64) (ffExpr, bool) {
			u, ok := animSlideAxes[preset]
			if !ok || u[axis] == 0 {
				return ffExpr{}, false
			}
			shift := exprOp("*", exprOp("-", exprNum(1), easeExpr(EaseOut, p)), exprNum(animSlideDistance)) // animSlideShift
			return exprOp("*", exprNum(sigma*float64(u[axis])), shift), true
		}
	}
	pop := func(preset string, _ float64) (ffExpr, bool) { // animPopScale
		if preset != "pop" {
			return ffExpr{}, false
		}
		u := exprLd(8)
		rise := exprOp("+", exprNum(animPopFrom), exprOp("*", exprNum(animPopRise), easeExpr(EaseOut, exprOp("/", p, exprNum(animPopPeakAt)))))
		settle := exprOp(";", exprSt(8, exprOp("/", exprOp("-", p, exprNum(animPopPeakAt)), exprNum(animPopTail))),
			exprOp("-", exprNum(animPopPeak), exprOp("*", exprNum(animPopSettle), exprOp("*", exprOp("*", u, u), exprOp("-", exprNum(3), exprOp("*", exprNum(2), u))))))
		return exprOp("if", exprOp("lt", p, exprNum(animPopPeakAt)), rise, settle), true
	}
	part := func(rest float64, term func(preset string, sigma float64) (ffExpr, bool)) ffExpr {
		tree := exprNum(rest)
		if out > 0 {
			if body, ok := term(clip.AnimOut.Preset, 1); ok {
				progress := exprSt(7, exprOp("/", exprOp("-", exprNum(float64(clip.Start+clip.Duration-1)), frame), exprNum(float64(out))))
				tree = exprOp("if", exprOp("lt", frame, exprNum(float64(clip.Start+clip.Duration-out))), tree, exprOp(";", progress, body))
			}
		}
		if in > 0 {
			if body, ok := term(clip.AnimIn.Preset, -1); ok {
				progress := exprSt(7, exprOp("/", exprOp("-", frame, exprNum(float64(clip.Start))), exprNum(float64(in))))
				tree = exprOp("if", exprOp("lt", frame, exprNum(float64(clip.Start+in))), exprOp(";", progress, body), tree)
			}
		}
		return tree
	}
	return animExpr{scale: part(1, pop), dx: part(0, slide(0)), dy: part(0, slide(1))}
}

// ---- Plan 2e Task 7: the span expression of the timeline enables (index §4.11, §4.24) ----

// animWindowFrame is the absolute timeline frame of a window frame from a filter's time variable in seconds:
// at + round(max(variable, 0)·Num/Den), the frame windowSourceExprVar keeps in register 0 (index §4.11). variable is t
// for timeline enable expressions and T for geq. max keeps a NaN time on the window's first frame.
func animWindowFrame(at int, rate Rate, variable string) ffExpr {
	return exprOp("+", exprNum(float64(at)), exprOp("round", exprOp("/",
		exprOp("*", exprOp("max", exprVar(variable), exprNum(0)), exprNum(float64(rate.Num))), exprNum(float64(rate.Den)))))
}

// animSpanExpr is the timeline expression (enable=) that is 1 on the absolute frames of the clip's effective spans
// (animSpans) whose preset is one of kinds, and 0 elsewhere (index §4.11, §4.24):
//   - register 0 holds the frame F (animWindowFrame in t; the filter's t counts from the window start);
//   - the in span is gte(F, start)·lt(F, start + D_in), the out span gte(F, end − D_out)·lt(F, end);
//   - max joins the two.
//
// Without such a span the expression is the constant 0. Only Go-written frame numbers and the rate enter the text, so
// every segmentation enables the same absolute frames.
func animSpanExpr(clip Clip, at int, rate Rate, kinds ...string) ffExpr {
	wanted := func(anim *ClipAnim) bool {
		if anim == nil {
			return false
		}
		for _, kind := range kinds {
			if anim.Preset == kind {
				return true
			}
		}
		return false
	}
	spanIn, spanOut := animSpans(clip)
	frame := exprLd(0)
	span := func(from, to int) ffExpr {
		return exprOp("*", exprOp("gte", frame, exprNum(float64(from))), exprOp("lt", frame, exprNum(float64(to))))
	}
	var terms []ffExpr
	if spanIn > 0 && wanted(clip.AnimIn) {
		terms = append(terms, span(clip.Start, clip.Start+spanIn))
	}
	if end := clip.Start + clip.Duration; spanOut > 0 && wanted(clip.AnimOut) {
		terms = append(terms, span(end-spanOut, end))
	}
	if len(terms) == 0 {
		return exprNum(0)
	}
	on := terms[0]
	if len(terms) == 2 {
		on = exprOp("max", terms[0], terms[1])
	}
	return exprOp(";", exprSt(0, animWindowFrame(at, rate, "t")), on)
}
