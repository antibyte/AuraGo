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

// animMergeGated records Task 8's gate (TestRenderAnimBlurMatchesCascade): maskedmerge's timeline enable switches on
// exactly the frames of the blur_in spans on FFmpeg 5.1.9 and 8.1.1, so the merges run only inside them. With false the
// merges run on every frame. Outside the spans every input of every merge is the sharp picture and the masks are 255, so
// the pixels are the same and only the cost differs (index §4.24).
const animMergeGated = true

// animLabel names a graph label of the clip's blur_in cascade: vs_an_<L>_<C>_<part> (index §4.10).
func animLabel(c clipChain, part string) string {
	return fmt.Sprintf("vs_an_%d_%d_%s", c.track, c.index, part)
}

// animBlurAlpha reports a picture whose alpha can be below 255 when it reaches the blur cascade: a still (images, titles)
// or a clip with a chroma_key entry that is on. Such a picture takes the premultiplied two-merge recipe (index §4.24).
func animBlurAlpha(c clipChain) bool {
	if c.item.image {
		return true
	}
	for _, entry := range c.item.whole.Effects {
		if entry.On && entry.ID == "chroma_key" {
			return true
		}
	}
	return false
}

// animMaskLevel is the constant mask m of blur cascade node n (1 or 2) at absolute frame frame (index §4.24). It is m1 or
// m2 of animAt inside a blur_in span, and 255 elsewhere, where both nodes pass the sharp picture.
func animMaskLevel(clip Clip, frame, n int) int {
	a := animAt(clip, frame)
	if !a.Blur {
		return 255
	}
	if n == 1 {
		return a.M1
	}
	return a.M2
}

// animMaskBranch is node n's constant mask for clip chain c (index §4.24): the source chain, ending in its output labels,
// and its command file vs_cmd_<L>_<C>_m<n>.txt.
//   - A 4x4 white gbrp source runs for the window. The file's sendcmd sets rr = gg = bb = m/255 of
//     colorchannelmixer@vs_animk_<L>_<C>_<n> on every window frame (animMaskLevel at at + k), which makes every plane
//     m. A neighbour upscale gives the processed picture size.
//   - Opaque: one gbrp output, m<n>.
//   - Alpha: the colour mask c<n> (gbrp; maskedmerge's format negotiation gives it alpha 255), and the alpha mask a<n>
//     (gbrap, alpha = m, through extractplanes and alphamerge, spike S8).
//
// The upscale's input never changes size, so the 5.1 t-term rule does not apply. Nothing but colorchannelmixer is
// commanded.
func animMaskBranch(c clipChain, n int, alpha bool) (filter string, file sendCmdFile) {
	width, height := c.processedSize()
	frames := c.item.clip.Duration
	target := fmt.Sprintf("colorchannelmixer@vs_animk_%d_%d_%d", c.track, c.index, n)
	levels := make([]float64, frames)
	for k := range levels {
		levels[k] = float64(animMaskLevel(c.item.whole, c.item.at+k, n)) / 255
	}
	file = sendCmdFile{name: fmt.Sprintf("vs_cmd_%d_%d_m%d.txt", c.track, c.index, n), tracks: []SendCmdTrack{
		{Target: target, Option: "rr", Values: levels},
		{Target: target, Option: "gg", Values: levels},
		{Target: target, Option: "bb", Values: levels},
	}}
	first := effectNum(levels[0])
	label := func(part string) string { return "[" + animLabel(c, fmt.Sprintf("%s%d", part, n)) + "]" }
	source := fmt.Sprintf("color=c=white:s=4x4:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=gbrp,sendcmd=f=%s,%s=rr=%s:gg=%s:bb=%s",
		c.rate, c.rate.SecondsArg(frames), frames, file.name, target, first, first, first)
	scale := fmt.Sprintf("scale=w=%d:h=%d:flags=neighbor", width, height)
	if !alpha {
		return source + "," + scale + ",format=gbrp" + label("m"), file
	}
	return source + ",split=3" + label("kc") + label("ka") + label("kg") + ";" +
		label("kc") + scale + ",format=gbrp" + label("c") + ";" +
		label("kg") + "extractplanes=g" + label("kp") + ";" +
		label("ka") + label("kp") + "alphamerge," + scale + ",format=gbrap" + label("a"), file
}

// animBlurGraph is the blur_in cascade of clip chain c (index §4.24). It returns the cascade without its closing filter,
// that closing filter ("" for the opaque recipe), and the two mask files. animBlurStage joins the cascade and the closing
// filter; the tests read the cascade on its own, where an alpha layer is still premultiplied.
//   - S = blurSigma(animBlurRadius, processed height), with g(S) and g(S/4) as gblur with four steps.
//   - Node 1 merges g(S) towards g(S/4) by m1, and node 2 merges that result towards the sharp picture by m2:
//     out = MM(MM(g(S), g(S/4), m1), sharp, m2). Outside the spans m1 = m2 = 255, so out is the sharp picture.
//   - Opaque input (animBlurAlpha false) runs on gbrp with one merge per node.
//   - An alpha layer runs premultiplied on gbrap with two merges per node: planes=7 with the colour mask, then planes=8
//     with m in the alpha mask's alpha plane. A single 4-plane merge on premultiplied input is wrong on 8.1.1 (spike S8).
//   - These filters carry the blur_in spans as their timeline enable (animSpanExpr): the gblurs, the merges
//     (animMergeGated) and an alpha layer's premultiply and unpremultiply. So the cascade costs only inside the spans,
//     and outside them an alpha layer skips the premultiply round trip. Every enable is exact per frame; outside the
//     spans every merge input is the sharp picture.
func animBlurGraph(c clipChain) (cascade, closing string, files []sendCmdFile) {
	_, height := c.processedSize()
	sigma := blurSigma(animBlurRadius, height)
	enable := "enable='" + animSpanExpr(c.item.whole, c.item.at, c.rate, "blur_in").String() + "'"
	label := func(part string) string { return "[" + animLabel(c, part) + "]" }
	blur := func(s float64) string { return "gblur=sigma=" + effectNum(s) + ":steps=4:" + enable }
	merge := func(planes string) string {
		switch {
		case animMergeGated && planes != "":
			return "maskedmerge=" + planes + ":" + enable
		case animMergeGated:
			return "maskedmerge=" + enable
		case planes != "":
			return "maskedmerge=" + planes
		}
		return "maskedmerge"
	}
	alpha := animBlurAlpha(c)
	mask1, file1 := animMaskBranch(c, 1, alpha)
	mask2, file2 := animMaskBranch(c, 2, alpha)
	files = []sendCmdFile{file1, file2}
	if !alpha {
		cascade = "format=gbrp,split=3" + label("s") + label("b1") + label("b2") + ";" +
			label("b1") + blur(sigma) + label("g1") + ";" +
			label("b2") + blur(sigma/4) + label("g2") + ";" +
			mask1 + ";" + mask2 + ";" +
			label("g1") + label("g2") + label("m1") + merge("") + label("x") + ";" +
			label("x") + label("s") + label("m2") + merge("")
		return cascade, "", files
	}
	cascade = "format=gbrap,premultiply=inplace=1:" + enable + ",split=4" + label("s1") + label("s2") + label("b1") + label("b2") + ";" +
		label("b1") + blur(sigma) + label("g1") + ";" +
		label("b2") + blur(sigma/4) + ",split=2" + label("g2a") + label("g2b") + ";" +
		mask1 + ";" + mask2 + ";" +
		label("g1") + label("g2a") + label("c1") + merge("planes=7") + label("x1") + ";" +
		label("x1") + label("g2b") + label("a1") + merge("planes=8") + label("y1") + ";" +
		label("y1") + label("s1") + label("c2") + merge("planes=7") + label("x2") + ";" +
		label("x2") + label("s2") + label("a2") + merge("planes=8")
	return cascade, "unpremultiply=inplace=1:" + enable, files
}

// animBlurStage is the blur_in cascade after the effect stack (index §4.10, §4.24), {} unless the clip has an effective
// blur_in span (animKindsOf).
//   - It is a two-input stage, so the commands of later stages read the next command group.
//   - Its two mask branches read their own command files (chainStage.files): no geq for the constant masks, and nothing
//     commands a two-input filter.
//
// Cost per 1080p frame (spike S5, 4 threads): 28.1 ms on 8.1.1 and 37.8 ms on 5.1.9 inside the spans. Outside them the
// gbrp(a) conversion, the split and the mask branches remain (Task 9 records both).
func animBlurStage(c clipChain) chainStage {
	if !animKindsOf(c.item.whole).blur {
		return chainStage{}
	}
	cascade, closing, files := animBlurGraph(c)
	if closing != "" {
		cascade += "," + closing
	}
	return chainStage{filter: cascade, files: files, twoInput: true}
}
