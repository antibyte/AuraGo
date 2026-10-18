package videostudio

import "fmt"

// The clip chain's in/out animation stages (index §4.10, §4.24). Both are two-input stages built in Go:
//   - animRevealStage: the wipe, a reveal row merged into the alpha in box space after the mask stage (Task 7);
//   - animBlurStage: the blur_in cascade after the effect stack (Task 8).
//
// Their expressions hold only Go-written numbers, registers and the T-baked absolute frame. Their timeline enable
// expressions hold only the clip's spans (animSpanExpr), so every segmentation renders the same frames. sendcmd never
// targets a two-input filter here (index §4.11).

// animRevealGated records Task 7's gate (TestRenderRevealBlendEnableGate): blend's timeline enable switches on exactly
// the frames of the wipe spans on FFmpeg 5.1.9 and 8.1.1, so the reveal merge runs only inside them. With false the merge
// runs on every frame. The reveal row is 255 everywhere outside the spans and a·255/255 = a, so the pixels are the same
// and only the cost differs (index §4.24).
const animRevealGated = true

// animRevealStage is the wipe's reveal (index §4.24), {} unless the clip has an effective wipe span (animKindsOf).
//   - The clip enters gbrap. Behind the mask stage this is a no-op; otherwise it is the one tolerated rgba → gbrap step
//     (±1 alpha below FFmpeg 6).
//   - A one-row geq of the fit canvas width writes alpha 255 on the columns the wipe shows (revealExpr).
//   - A neighbour upscale repeats the row over the canvas height.
//   - blend multiplies it into the alpha: RGB untouched, a·255//255 = a on shown columns and 0 on hidden ones.
//   - The merge is gated by the wipe spans (animRevealGated).
//
// The upscale's input never changes size, so the 5.1 t-term rule does not apply. Cost per frame: the geq of CW px and
// the upscale on every frame, the merge only inside the spans (Task 9 records it).
func animRevealStage(c clipChain) chainStage {
	if !animKindsOf(c.item.whole).wipe {
		return chainStage{}
	}
	plan := maskPlanOf(c)
	frames := c.item.clip.Duration
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	in, mask := "vs_rv_"+id+"_in", "vs_rv_"+id+"_m"
	generator := fmt.Sprintf("nullsrc=s=%dx1:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=gbrap,geq=r='0':g='0':b='0':a='%s',scale=w=%d:h=%d:flags=neighbor",
		plan.canvasWidth, c.rate, c.rate.SecondsArg(frames), frames, revealExpr(c, plan).String(), plan.canvasWidth, plan.canvasHeight)
	merge := "blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply"
	if animRevealGated {
		merge += ":enable='" + animSpanExpr(c.item.whole, c.item.at, c.rate, "wipe").String() + "'"
	}
	return chainStage{filter: fmt.Sprintf("format=gbrap[%s];%s[%s];[%s][%s]%s", in, generator, mask, in, mask, merge), twoInput: true}
}

// revealExpr is the alpha of the reveal row at geq pixel X of frame T (index §4.24). It is 255 on the canvas columns the
// wipe shows and 0 elsewhere. Outside the wipe spans it is 255 on the whole row, where the merge is the identity. It
// reads only the canvas and the box width of plan (maskPlanOf: the reveal shares the mask's box space), and needs no mask.
//
// The frame values depend on T alone, so the row's first pixel computes them (if(lt(X, 0.5), …)). The row is one slice
// job that runs from X = 0 (the 2d mask's row cache):
//   - 5 = the box width W' in canvas px: a Go number, or the box expression of the pad canvas in T, which uses registers
//     0–4, 7 and 8 (Task 6) before 5 is stored; 6–9 are stored after it, so nothing is kept across it;
//   - 6 = the absolute frame F = at + round(max(T, 0)·Num/Den);
//   - 7 = the box's first canvas column, (CW − W')/2;
//   - 8 and 9 = the first and the end column: [7, 7 + R) in the in span, [7 + W' − R, 7 + W') in the out span, [0, CW)
//     at rest. R = round(p·W') with p = (F − start)/D_in or (end − 1 − F)/D_out (animRevealPx).
//
// Every pixel then reads 255·gte(X, ld(8))·lt(X, ld(9)).
func revealExpr(c clipChain, plan maskPlan) ffExpr {
	whole := c.item.whole
	spanIn, spanOut := animSpans(whole)
	width, frame, left := exprLd(5), exprLd(6), exprLd(7)
	reveal := func(progress ffExpr) ffExpr { return exprOp("round", exprOp("*", progress, width)) }
	columns := exprSeq(exprSt(8, exprNum(0)), exprSt(9, exprNum(float64(plan.canvasWidth))))
	end := whole.Start + whole.Duration
	if spanOut > 0 && whole.AnimOut != nil && whole.AnimOut.Preset == "wipe" {
		progress := exprOp("/", exprOp("-", exprNum(float64(end-1)), frame), exprNum(float64(spanOut)))
		columns = exprOp("if", exprOp("gte", frame, exprNum(float64(end-spanOut))),
			exprSeq(exprSt(9, exprOp("+", left, width)), exprSt(8, exprOp("-", exprLd(9), reveal(progress)))), columns)
	}
	if spanIn > 0 && whole.AnimIn != nil && whole.AnimIn.Preset == "wipe" {
		progress := exprOp("/", exprOp("-", frame, exprNum(float64(whole.Start))), exprNum(float64(spanIn)))
		columns = exprOp("if", exprOp("lt", frame, exprNum(float64(whole.Start+spanIn))),
			exprSeq(exprSt(8, left), exprSt(9, exprOp("+", left, reveal(progress)))), columns)
	}
	once := exprSeq(exprSt(5, plan.boxWidth), exprSt(6, animWindowFrame(c.item.at, c.rate, "T")),
		exprSt(7, exprOp("/", exprOp("-", exprNum(float64(plan.canvasWidth)), width), exprNum(2))), columns)
	return exprSeq(exprOp("if", exprOp("lt", exprVar("X"), exprNum(0.5)), once, exprNum(0)),
		exprOp("*", exprNum(255), exprOp("*", exprOp("gte", exprVar("X"), exprLd(8)), exprOp("lt", exprVar("X"), exprLd(9)))))
}
