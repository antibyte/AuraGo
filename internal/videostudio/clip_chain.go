package videostudio

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"sync"
)

// clipChain builds one visual clip's export filter chain (spec §7.1, plan index §4.10). The clip's input is retimed
// onto the project frame grid, cut to its window, transformed into its box and placed on a transparent canvas-size
// layer.
//
// Each string stage returns its filters without the separating comma, or "" when it adds nothing. build joins the
// stages in the preview's pipeline order (stages, assembleStages) and collects the command and LUT files they read.
// Later slices extend a stage, never the assembly in build:
//   - 2b: retime (speed ramps) and runtime-command targets in transform and opacity.
//   - 2c: picture (crop, flip), effects (one chainStage per entry), command groups after two-input stages and
//     the animated crop in front of the transform.
//   - 2d: mask (a two-input stage) and the blend path of place.
type clipChain struct {
	track, index              int
	item                      layerClip
	canvasWidth, canvasHeight int
	rate                      Rate
}

// clipBuild is one clip's part of a segment graph (index §4.10):
//   - the filter chain ending in the output label;
//   - the sendcmd scripts it reads (one per non-empty command group);
//   - the LUT tables it reads.
//
// Files are referred to by bare name.
type clipBuild struct {
	filter   string
	commands []sendCmdFile
	luts     []lutFile
}

// chainStage is one stage between window and place, as assembleStages joins it:
//   - filter: its filters without the separating comma ("" adds nothing);
//   - tracks: the runtime commands for the single-input filters in it;
//   - luts: the LUT tables it reads by bare name (vs_lut_<L>_<C>_<I>.cube);
//   - twoInput: the stage ends in a two-input filter (blend, alphamerge, 2d's mask merge).
//
// The tracks of every later stage go to the next command group, because commands must not cross a framesync. A
// two-input stage holds no command target in 2c. Targets inside one, before its merge, would be read by the group in
// front of it.
type chainStage struct {
	filter   string
	tracks   []SendCmdTrack
	luts     []lutFile
	twoInput bool
}

// build assembles the clip's chain once. A clip without keyframes, ramp, picture fields or effects produces, byte for
// byte, the 2a graph (TestClipChainSnapshots).
func (c clipChain) build(output string) clipBuild {
	body, commands, luts := assembleStages(fmt.Sprintf("%d_%d", c.track, c.index), c.stages())
	parts := make([]string, 0, len(body)+2)
	for _, stage := range []string{c.retime(), c.window()} {
		if stage != "" {
			parts = append(parts, stage)
		}
	}
	parts = append(parts, body...)
	return clipBuild{filter: fmt.Sprintf("[%d:V:0]%s", c.item.input, strings.Join(parts, ",")) + c.place(output), commands: commands, luts: luts}
}

// filter returns the clip's chain ending in the label output.
func (c clipChain) filter(output string) string { return c.build(output).filter }

// stages lists the stages after window in the preview's pipeline order: picture, the effect entries, mask, the
// animated crop, transform and opacity. The animated crop (plan 2c) is its own stage right in front of the transform,
// so its commands join the group of the last two-input stage before it.
func (c clipChain) stages() []chainStage {
	stages := []chainStage{{filter: c.picture()}}
	stages = append(stages, c.effects()...)
	return append(stages,
		chainStage{filter: c.mask()},
		chainStage{filter: c.animatedCrop(), tracks: c.pictureTracks()},
		chainStage{filter: c.transform(), tracks: c.transformTracks()},
		chainStage{filter: c.opacity(), tracks: c.opacityTracks()})
}

// assembleStages joins the stages of clip id ("<L>_<C>") and splits their command tracks into command groups (index
// §4.11):
//   - Group 0 is vs_cmd_<L>_<C>.txt. Its sendcmd comes first, directly after window, whose timestamps restart at the
//     window's first frame, so the script's times are window-local.
//   - Group g ≥ 1 is vs_cmd_<L>_<C>_<g>.txt. Its sendcmd follows the g-th two-input stage, whose output keeps those
//     timestamps.
//   - Between a sendcmd and its targets lie single-input filters only, so no command overtakes its frame.
//   - Empty groups get neither a file nor a sendcmd.
//
// The LUT files are listed in stage order.
func assembleStages(id string, stages []chainStage) (filters []string, commands []sendCmdFile, luts []lutFile) {
	groups := [][]SendCmdTrack{nil}
	for _, stage := range stages {
		last := len(groups) - 1
		groups[last] = append(groups[last], stage.tracks...)
		if stage.twoInput {
			groups = append(groups, nil)
		}
	}
	reader := func(group int) {
		if len(groups[group]) == 0 {
			return
		}
		name := "vs_cmd_" + id + ".txt"
		if group > 0 {
			name = fmt.Sprintf("vs_cmd_%s_%d.txt", id, group)
		}
		commands = append(commands, sendCmdFile{name: name, tracks: groups[group]})
		filters = append(filters, "sendcmd=f="+name)
	}
	reader(0)
	group := 0
	for _, stage := range stages {
		if stage.filter != "" {
			filters = append(filters, stage.filter)
		}
		luts = append(luts, stage.luts...)
		if stage.twoInput {
			group++
			reader(group)
		}
	}
	return filters, commands, luts
}

// retime samples the source on the project frame grid. Images start at zero. Video keeps absolute timestamps, so fps
// samples it on the source's own cadence and output frame from becomes the clip's first frame. When the video stream
// starts after the container (for example behind leading audio) no source frame covers the first positions; the
// first decoded frame is then moved back to from, so fps holds it there instead of the clip running short. The video
// path is videoResample, which the still capture (derive.go) shares so a still shows the frame the renderer shows.
// A ramped clip uses rampResample instead (index §4.18).
func (c clipChain) retime() string {
	if c.item.image {
		return fmt.Sprintf("setpts=PTS-STARTPTS,fps=%s", c.rate)
	}
	if len(c.item.pieces) > 0 {
		return rampResample(c.item.from, c.item.clip.Offset, c.item.pieces, c.rate)
	}
	return videoResample(c.item.from, c.item.clip.Offset, ClipSpeed(c.item.clip), c.rate)
}

// window shifts the retimed frames into the clip and bounds clone padding to a small multiple of the clip's duration.
func (c clipChain) window() string {
	clip := c.item.clip
	return fmt.Sprintf("setpts='max(PTS-%d,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS", c.item.first, clip.Duration, clip.Duration*2, clip.Duration)
}

// picture crops and flips the source (index §4.10, §4.20). Crop fractions refer to the unflipped source:
//   - A static crop cuts cropRect's rectangle with expressions in iw/ih, so it is right in whatever orientation
//     autorotation delivers. The flips follow it.
//   - With crop keyframes only the flips run here. The effects then see the whole flipped picture, and the crop follows
//     them in front of the transform (animatedCrop). The reason: filters after a size-changing crop keep the size they
//     were configured with, so variable-size frames cannot pass through the effect filters (plan 2c decision 4).
func (c clipChain) picture() string {
	whole := c.item.whole
	var parts []string
	if whole.Crop != nil && !cropAnimated(whole) {
		parts = append(parts, staticCropFilter(*whole.Crop))
	}
	if whole.FlipH {
		parts = append(parts, "hflip")
	}
	if whole.FlipV {
		parts = append(parts, "vflip")
	}
	return strings.Join(parts, ",")
}

// cropRectExprs is cropRect (index §4.20) as crop-filter expressions in the input size iw × ih:
//   - x = min(2·round(l·iw/2), max(0, iw − 2)) and w = min(iw − x, max(2, iw − x − 2·round(r'·iw/2))), with
//     r' = min(r, 0.95 − l);
//   - y and h likewise.
//
// Go computes r' and b' and writes them as numbers. ffExpr's round is FFmpeg's (half away from zero), like math.Round,
// and its min and max are FFmpeg's. TestCropRectExprMatchesCropRect evaluates the expressions against cropRect.
func cropRectExprs(crop Crop) (x, y, w, h ffExpr) {
	even := func(fraction float64, side string) ffExpr { // 2·round(fraction·side/2)
		return exprOp("*", exprOp("round", exprOp("/", exprOp("*", exprNum(fraction), exprVar(side)), exprNum(2))), exprNum(2))
	}
	position := func(fraction float64, side string) ffExpr { // min(2·round(fraction·side/2), max(0, side − 2))
		return exprOp("min", even(fraction, side), exprOp("max", exprNum(0), exprOp("-", exprVar(side), exprNum(2))))
	}
	size := func(start ffExpr, fraction float64, side string) ffExpr { // min(side − start, max(2, side − start − even))
		rest := exprOp("-", exprVar(side), start)
		return exprOp("min", rest, exprOp("max", exprNum(2), exprOp("-", rest, even(fraction, side))))
	}
	right, bottom := math.Min(crop.R, maxCropFraction-crop.L), math.Min(crop.B, maxCropFraction-crop.T)
	x, y = position(crop.L, "iw"), position(crop.T, "ih")
	return x, y, size(x, right, "iw"), size(y, bottom, "ih")
}

// staticCropFilter cuts a static crop. exact=1 keeps the odd sizes of a subsampled source (a 4:2:0 JPEG of odd width) at
// cropRect's numbers instead of rounding them down; the positions are even anyway.
func staticCropFilter(crop Crop) string {
	x, y, w, h := cropRectExprs(crop)
	return fmt.Sprintf("crop=w='%s':h='%s':x='%s':y='%s':exact=1", w.String(), h.String(), x.String(), y.String())
}

// pictureRect is one frame's crop of a clip with crop keyframes, in decoded source pixels.
type pictureRect struct{ x, y, w, h int }

// pictureRects is the crop of every window frame of a clip with crop keyframes (index §4.10):
//   - the crop values at absolute frame at + k (ValueAt over crop.l|t|r|b);
//   - mirrored for the flips that picture() already applied (mirroredCrop);
//   - as cropRect's rectangle of the decoded, autorotated picture (layerClip.pictureWidth × pictureHeight);
//   - for cover, the window of that frame's box aspect inside it (coverWindow).
func (c clipChain) pictureRects() []pictureRect {
	whole := c.item.whole
	frames := c.item.clip.Duration
	var boxWidths, boxHeights []int
	if whole.Fit == FitCover {
		if geometry, _ := clipAnimation(whole); geometry {
			g := c.geometry()
			boxWidths, boxHeights = g.widths, g.heights
		} else {
			_, _, w, h := clipBox(c.item.clip, c.canvasWidth, c.canvasHeight)
			boxWidths, boxHeights = make([]int, frames), make([]int, frames)
			for k := range frames {
				boxWidths[k], boxHeights[k] = w, h
			}
		}
	}
	value := func(path string, frame int) float64 {
		v, _ := ValueAt(whole, path, frame)
		return v
	}
	rects := make([]pictureRect, frames)
	for k := range rects {
		frame := c.item.at + k
		crop := mirroredCrop(Crop{L: value("crop.l", frame), T: value("crop.t", frame), R: value("crop.r", frame), B: value("crop.b", frame)}, whole.FlipH, whole.FlipV)
		x, y, w, h := cropRect(crop, c.item.pictureWidth, c.item.pictureHeight)
		if boxWidths != nil {
			x, y, w, h = coverWindow(x, y, w, h, boxWidths[k], boxHeights[k])
		}
		rects[k] = pictureRect{x: x, y: y, w: w, h: h}
	}
	return rects
}

// animatedCrop cuts the per-frame crop of a clip with crop keyframes right in front of the transform.
// crop@vs_picture_L_C starts at the window's first rectangle and takes w, h, x and y of every later frame from sendcmd
// (pictureTracks). It returns "" without crop keyframes.
func (c clipChain) animatedCrop() string {
	if !cropAnimated(c.item.whole) {
		return ""
	}
	first := c.pictureRects()[0]
	return fmt.Sprintf("crop@vs_picture_%d_%d=w=%d:h=%d:x=%d:y=%d:exact=1", c.track, c.index, first.w, first.h, first.x, first.y)
}

// pictureTracks are the commands of crop@vs_picture_L_C: w, h, x and y for every window frame, in that order, so a
// frame's size is set before its position. nil without crop keyframes.
func (c clipChain) pictureTracks() []SendCmdTrack {
	if !cropAnimated(c.item.whole) {
		return nil
	}
	rects := c.pictureRects()
	w, h, x, y := make([]float64, len(rects)), make([]float64, len(rects)), make([]float64, len(rects)), make([]float64, len(rects))
	for k, rect := range rects {
		w[k], h[k], x[k], y[k] = float64(rect.w), float64(rect.h), float64(rect.x), float64(rect.y)
	}
	target := fmt.Sprintf("crop@vs_picture_%d_%d", c.track, c.index)
	return []SendCmdTrack{
		{Target: target, Option: "w", Values: w}, {Target: target, Option: "h", Values: h},
		{Target: target, Option: "x", Values: x}, {Target: target, Option: "y", Values: y},
	}
}

// coverWindow is the part of a crop rectangle that a cover fit of the box shows: the largest centred rectangle with the
// box's aspect, in whole pixels and at least 2 px across. The exact scale that follows fills the box with it, so an
// animated crop needs no second crop on a constant canvas.
func coverWindow(x, y, w, h, boxWidth, boxHeight int) (int, int, int, int) {
	if int64(w)*int64(boxHeight) > int64(h)*int64(boxWidth) { // wider than the box: trim the sides
		width := min(w, max(2, int(math.Round(float64(h)*float64(boxWidth)/float64(boxHeight)))))
		return x + (w-width)/2, y, width, h
	}
	height := min(h, max(2, int(math.Round(float64(w)*float64(boxHeight)/float64(boxWidth)))))
	return x, y + (h-height)/2, w, height
}

// effects runs the clip's effect stack (index §4.10). Every entry that is on, in stack order, is built by its registry
// builder (EffectSpec.build). Each builder declares the pixel format of its input, and format= is inserted only where
// it differs from the format the previous stage left. Identity entries and effects without a builder leave no part. A
// two-input part (a LUT blend, the vignette alpha wrapper) carries twoInput, so assembleStages opens a command group
// after it.
func (c clipChain) effects() []chainStage {
	var parts []chainStage
	format := ""
	for index, entry := range c.item.whole.Effects {
		if !entry.On {
			continue
		}
		spec, ok := EffectByID(entry.ID)
		if !ok || spec.build == nil {
			continue // Validate admits registry effects only; a later slice's row has no builder until that slice lands.
		}
		stage := spec.build(c.entryInput(index, spec))
		if len(stage.parts) == 0 {
			continue
		}
		if stage.input != format {
			stage.parts[0].filter = "format=" + stage.input + "," + stage.parts[0].filter
		}
		format = stage.output
		parts = append(parts, stage.parts...)
	}
	return parts
}

// entryInput describes stack entry index of the clip for its builder.
func (c clipChain) entryInput(index int, spec EffectSpec) effectInput {
	width, height := c.processedSize()
	return effectInput{id: fmt.Sprintf("%d_%d", c.track, c.index), index: index, spec: spec, whole: c.item.whole, at: c.item.at,
		frames: c.item.clip.Duration, pieces: c.item.pieces, rate: c.rate, image: c.item.image, width: width, height: height, lut: c.item.luts[index]}
}

// processedSize is the picture the effects process, in decoded pixels (index §4.20, resolution rule): the static crop
// rectangle, or the whole decoded picture when the crop is keyframed or absent.
func (c clipChain) processedSize() (width, height int) {
	width, height = c.item.pictureWidth, c.item.pictureHeight
	if crop := c.item.whole.Crop; crop != nil && !cropAnimated(c.item.whole) && width > 0 && height > 0 {
		_, _, width, height = cropRect(*crop, width, height)
	}
	return width, height
}

// mask applies the clip's mask as alpha (plan 2d, a two-input stage). Nothing in 2c.
func (c clipChain) mask() string { return "" }

// transform fits the picture into the clip box (contain pads, cover crops), converts to RGBA and rotates it.
//   - A static cover clip whose source is more than 16x off the box aspect first crops the source to within that factor
//     (staticCoverClamp), so the cover-scaled picture stays a bounded multiple of the box.
//   - With crop keyframes, the picture arrives from crop@vs_picture in a new size whenever the crop changes. Contain
//     re-fits every frame (scale and pad with eval=frame). With eval=init, scale treats its previous output size as the
//     target when the input size changes. Cover scales the composed window exactly onto the box (index §4.10).
//   - Both scales read t in w and h ('<BW>+0*t') with eval=frame. FFmpeg 5.1's scale reconfigures only when a frame
//     differs from its input link, and the commanded crop has already updated that link: a constant-size scale there
//     keeps its old scaler (5.1 hangs), and the t-term forces a reconfiguration on every frame (index risk 14). It
//     costs about 2.3× on 5.1 at 1080p, so only clips with crop keyframes use it.
func (c clipChain) transform() string {
	if geometry, _ := clipAnimation(c.item.whole); geometry {
		return c.animatedTransform()
	}
	clip := c.item.clip
	_, _, boxWidth, boxHeight := clipBox(clip, c.canvasWidth, c.canvasHeight)
	cropped := cropAnimated(c.item.whole)
	var scale string
	switch {
	case cropped && clip.Fit == FitCover:
		scale = fmt.Sprintf("scale=w='%d+0*t':h='%d+0*t':eval=frame", boxWidth, boxHeight)
	case cropped:
		scale = fmt.Sprintf("scale=w='%d+0*t':h='%d+0*t':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame", boxWidth, boxHeight, boxWidth, boxHeight)
	case clip.Fit == FitCover:
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=%d:h=%d", boxWidth, boxHeight, boxWidth, boxHeight)
		if clamp := staticCoverClamp(boxWidth, boxHeight, c.item.sourceAspect); clamp != "" {
			scale = clamp + "," + scale
		}
	default:
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0", boxWidth, boxHeight, boxWidth, boxHeight)
	}
	rotation := ""
	if clipRotated(clip) {
		angle := fmt.Sprintf("(%.6f*PI/180)", clip.Rotation)
		rotation = fmt.Sprintf(",rotate=%s:ow=rotw(%s):oh=roth(%s):c=black@0", angle, angle, angle)
	}
	return scale + ",format=rgba" + rotation
}

// opacity scales the picture's alpha; full opacity adds no filter. Keyframed opacity names the instance and starts at
// the window's first value; the sendcmd script sets every later frame.
func (c clipChain) opacity() string {
	if _, keyframed := clipAnimation(c.item.whole); keyframed {
		first, _ := ValueAt(c.item.whole, "opacity", c.item.at)
		return fmt.Sprintf("colorchannelmixer@vs_opacity_%d_%d=aa=%s", c.track, c.index, exprNum(first).String())
	}
	if c.item.clip.Opacity < 0.999999 {
		return fmt.Sprintf("colorchannelmixer=aa=%.6f", c.item.clip.Opacity)
	}
	return ""
}

// place overlays the clip layer on a transparent canvas-size background and ends in the label output. A static clip
// sits at its box (a rotated one keeps its box centre). With keyframed geometry the constant layer canvas
// (geometryPlan.layerWidth/layerHeight) is centred on the per-frame box centre by overlay expressions in t (index §4.11).
func (c clipChain) place(output string) string {
	clip := c.item.clip
	layer := fmt.Sprintf("vs_layer_%d_%d", c.track, c.index)
	background := fmt.Sprintf("vs_clip_bg_%d_%d", c.track, c.index)
	var overlay string
	if geometry, _ := clipAnimation(c.item.whole); geometry {
		g := c.geometry()
		x := exprOp("-", g.box.cx, exprNum(float64(g.layerWidth/2)))
		y := exprOp("-", g.box.cy, exprNum(float64(g.layerHeight/2)))
		overlay = fmt.Sprintf("overlay@vs_place_%d_%d=x='%s':y='%s':eval=frame:format=auto:shortest=1", c.track, c.index, x.String(), y.String())
	} else {
		x, y, boxWidth, boxHeight := clipBox(clip, c.canvasWidth, c.canvasHeight)
		positionX, positionY := fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)
		if clipRotated(clip) {
			positionX = fmt.Sprintf("'%d+%d/2-overlay_w/2'", x, boxWidth)
			positionY = fmt.Sprintf("'%d+%d/2-overlay_h/2'", y, boxHeight)
		}
		overlay = fmt.Sprintf("overlay=x=%s:y=%s:format=auto:shortest=1", positionX, positionY)
	}
	return fmt.Sprintf("[%s];color=c=black@0:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s];[%s][%s]%s[%s]", layer, c.canvasWidth, c.canvasHeight, c.rate, c.rate.SecondsArg(clip.Duration), clip.Duration, background, background, layer, overlay, output)
}

// clipRotated reports a rotation the export applies (anything above a millionth of a degree).
func clipRotated(clip Clip) bool { return math.Abs(clip.Rotation) > 0.000001 }

// clipBox is the clip's box on a canvas in whole, even pixels: even size of at least 2 px, even position, inside the
// canvas. The JS preview mirrors it (VideoStudioEffects.placement; parity fixture testdata/parity/geometry.json).
func clipBox(clip Clip, canvasWidth, canvasHeight int) (x, y, w, h int) {
	w = evenPixels(clip.Width*float64(canvasWidth), canvasWidth)
	h = evenPixels(clip.Height*float64(canvasHeight), canvasHeight)
	x = evenPosition(clip.X*float64(canvasWidth), w, canvasWidth)
	y = evenPosition(clip.Y*float64(canvasHeight), h, canvasHeight)
	return x, y, w, h
}

func evenPixels(value float64, bound int) int {
	pixels := int(math.Round(value/2) * 2)
	if pixels < 2 {
		return 2
	}
	if pixels > bound {
		return bound
	}
	return pixels
}

func evenPosition(value float64, box, bound int) int {
	position := int(math.Round(value/2) * 2)
	maxPosition := bound - box
	if position < 0 {
		return 0
	}
	if position > maxPosition {
		position = maxPosition
	}
	return position
}

// geometryPlan is the layout of a clip with keyframed geometry (index §4.11):
//   - the box expressions of the window;
//   - the box size of every window frame, evaluated in Go exactly as FFmpeg evaluates the scale expressions;
//   - the constant canvas pad gives every frame;
//   - the canvas the place overlay positions;
//   - for cover, the aspect clamp and the bound the picture is padded to.
//
// The canvas sizes, the rotate stage and the cover clamp come from the WHOLE clip, not from the window: they decide how
// FFmpeg samples the picture (the clamp crops the source, rotate resamples the canvas), so a window-dependent value
// would render one clip differently in different segments.
type geometryPlan struct {
	box                     boxExpr
	widths, heights         []int
	padWidth, padHeight     int // largest box of the clip; 2 px more when rotated (transparent ring for rotate's border)
	layerWidth, layerHeight int // the pad canvas, or the square rotate canvas evenCeil(hypot(pad)) when rotated
	rotated                 bool
	boundWidth, boundHeight int     // cover: the canvas that holds the cover-scaled picture of every clip frame (coverBound)
	low, high               float64 // cover: the range of the clip's box aspects the source aspect is clamped to
}

// geometry lays out the clip's keyframed box. Window frame k has t = k·Den/Num, the value FFmpeg computes as
// pts·av_q2d(tb) after window; the clip's own frames j (absolute frame whole.Start + j) are evaluated the same way
// with the window starting at the clip's start. The window's own part (box expressions and sizes) is cheap; the
// whole-clip part (wholeExtent) is remembered, because every stage of every window asks for it.
func (c clipChain) geometry() geometryPlan {
	whole := c.item.whole
	g := geometryPlan{box: boxExprs(whole, c.item.at, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate)}
	var windowRotated bool
	g.widths, g.heights, windowRotated = c.frameSizes(g.box, c.item.clip.Duration)
	var extent wholeExtent
	if c.item.at == whole.Start && c.item.clip.Duration == whole.Duration {
		extent = c.extentOf(g.widths, g.heights, windowRotated)
	} else {
		extent = c.wholeExtent()
	}
	g.padWidth, g.padHeight, g.rotated = extent.padWidth, extent.padHeight, extent.rotated
	g.boundWidth, g.boundHeight, g.low, g.high = extent.boundWidth, extent.boundHeight, extent.low, extent.high
	g.layerWidth, g.layerHeight = g.padWidth, g.padHeight
	if g.rotated {
		g.padWidth, g.padHeight = g.padWidth+2, g.padHeight+2
		side := evenCeil(math.Hypot(float64(g.padWidth), float64(g.padHeight)))
		g.layerWidth, g.layerHeight = side, side
	}
	return g
}

// frameSizes evaluates a box frame by frame, the way FFmpeg evaluates the scale expressions: window frame k has
// t = k·Den/Num. rotated reports a frame whose angle exceeds a millionth of a degree.
func (c clipChain) frameSizes(box boxExpr, frames int) (widths, heights []int, rotated bool) {
	step := float64(c.rate.Den) / float64(c.rate.Num)
	widths, heights = make([]int, frames), make([]int, frames)
	for k := range frames {
		vars := map[string]float64{"t": float64(k) * step}
		widths[k], heights[k] = int(box.w.eval(vars)), int(box.h.eval(vars))
		if math.Abs(box.angle.eval(vars)) > 0.000001*math.Pi/180 {
			rotated = true
		}
	}
	return widths, heights, rotated
}

// wholeExtent is what a clip with keyframed geometry needs from all of its frames: the largest box (the pad canvas),
// whether any frame rotates and, for cover, the bound and the clamp range of coverBound.
type wholeExtent struct {
	padWidth, padHeight     int
	rotated                 bool
	boundWidth, boundHeight int
	low, high               float64
}

// extentOf reduces the box sizes of the clip's frames to the extent.
func (c clipChain) extentOf(widths, heights []int, rotated bool) wholeExtent {
	extent := wholeExtent{rotated: rotated}
	for k := range widths {
		extent.padWidth, extent.padHeight = max(extent.padWidth, widths[k]), max(extent.padHeight, heights[k])
	}
	if c.item.whole.Fit == FitCover {
		extent.boundWidth, extent.boundHeight, extent.low, extent.high = coverBound(widths, heights, c.item.sourceAspect)
	}
	return extent
}

// extentMemoLimit bounds the memo; it is cleared when one more clip layout appears.
const extentMemoLimit = 256

// wholeExtents remembers the extent of every clip layout this process has sized, under a digest of everything that
// feeds it: the three box expressions of the whole clip, its length, the frame rate, the canvas, the cover fit and the
// source aspect. A render builds the graph of every segment window and each of its stages asks for the extent; a
// one-hour clip is 108000 frames of expression evaluation, about half a second, so asking again for every window and
// stage took minutes. The extent is a pure function of the key, so a remembered value is the value that would be
// computed.
var wholeExtents = struct {
	sync.Mutex
	byKey map[[sha256.Size]byte]wholeExtent
}{byKey: map[[sha256.Size]byte]wholeExtent{}}

// wholeExtentHook, when set, is called for every extent that is computed instead of remembered (tests count them).
var wholeExtentHook func()

// wholeExtent sizes the clip over all of its frames, not just the segment window, so every segmentation of one clip
// renders it on one canvas (a window-only value broke segmentation parity).
func (c clipChain) wholeExtent() wholeExtent {
	whole := c.item.whole
	box := boxExprs(whole, whole.Start, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate)
	digest := sha256.New()
	for _, expr := range []ffExpr{box.w, box.h, box.angle} {
		text := expr.String()
		fmt.Fprintf(digest, "%d:%s|", len(text), text)
	}
	fmt.Fprintf(digest, "%d|%d/%d|%dx%d|%t|%x", whole.Duration, c.rate.Num, c.rate.Den, c.canvasWidth, c.canvasHeight, whole.Fit == FitCover, math.Float64bits(c.item.sourceAspect))
	var key [sha256.Size]byte
	copy(key[:], digest.Sum(nil))
	wholeExtents.Lock()
	extent, ok := wholeExtents.byKey[key]
	wholeExtents.Unlock()
	if ok {
		return extent
	}
	if wholeExtentHook != nil {
		wholeExtentHook()
	}
	widths, heights, rotated := c.frameSizes(box, whole.Duration)
	extent = c.extentOf(widths, heights, rotated)
	wholeExtents.Lock()
	if len(wholeExtents.byKey) >= extentMemoLimit {
		clear(wholeExtents.byKey)
	}
	wholeExtents.byKey[key] = extent
	wholeExtents.Unlock()
	return extent
}

// animatedTransform fits a clip with keyframed geometry into its per-frame box.
//   - scale evaluates the box in t (eval=frame).
//   - pad (eval=frame, which re-reads each frame's size) centres every frame on one constant canvas. The later filters
//     need that: format, crop and rotate keep the input size the graph was configured with, and rotate also repeats
//     its input's edge one pixel outward (so a rotated window gets a transparent 2 px ring).
//   - Cover first clamps the source aspect to the clip's box aspects, so the cover-scaled picture fits a bound
//     computed in Go. It then pads to that constant bound and crops the box there. The crop takes its per-frame size
//     from the sendcmd script and re-validates it against its input before the frame arrives, so that input must not
//     change size.
//   - rotate follows only when the clip rotates.
//   - With crop keyframes, cover scales crop@vs_picture's window exactly; contain keeps the 2b chain.
func (c clipChain) animatedTransform() string {
	g := c.geometry()
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	pad := func(name string, width, height int) string {
		return fmt.Sprintf("pad%s=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame", name, width, height)
	}
	var parts []string
	if c.item.whole.Fit == FitCover && cropAnimated(c.item.whole) {
		// crop@vs_picture already cut the window of this frame's box aspect: scale fills the box exactly and pad keeps the
		// constant canvas, without the aspect clamp, cover bound or crop@vs_crop of the 2b chain.
		parts = append(parts, fmt.Sprintf("scale@vs_scale_%s=w='%s':h='%s':eval=frame", id, g.box.w.String(), g.box.h.String()))
	} else if c.item.whole.Fit == FitCover {
		parts = append(parts,
			aspectClampCrop(g.low, g.high, 0),
			fmt.Sprintf("scale@vs_scale_%s=w='%s':h='%s':force_original_aspect_ratio=increase:force_divisible_by=2:eval=frame", id, g.box.w.String(), g.box.h.String()),
			pad("", g.boundWidth, g.boundHeight),
			fmt.Sprintf("crop@vs_crop_%s=w=%d:h=%d", id, g.widths[0], g.heights[0]))
	} else {
		parts = append(parts, fmt.Sprintf("scale@vs_scale_%s=w='%s':h='%s':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame", id, g.box.w.String(), g.box.h.String()))
	}
	parts = append(parts, pad("@vs_pad_"+id, g.padWidth, g.padHeight), "format=rgba")
	if g.rotated {
		parts = append(parts, fmt.Sprintf("rotate@vs_rotate_%s=a='%s':ow=%d:oh=%d:c=black@0", id, g.box.angle.String(), g.layerWidth, g.layerHeight))
	}
	return strings.Join(parts, ",")
}

// commandFiles are the clip's sendcmd scripts for this FFmpeg process (index §4.11), one per non-empty command group.
func (c clipChain) commandFiles() []sendCmdFile {
	_, commands, _ := assembleStages(fmt.Sprintf("%d_%d", c.track, c.index), c.stages())
	return commands
}

// transformTracks drive the transform stage: the cover crop of keyframed geometry, its size for every window frame
// (crop@vs_crop_L_C, index §4.11), exactly the box sizes FFmpeg's scale computes, from absolute frames. A clip with
// crop keyframes cuts its cover window in crop@vs_picture instead (pictureTracks), so it commands no crop@vs_crop.
func (c clipChain) transformTracks() []SendCmdTrack {
	if geometry, _ := clipAnimation(c.item.whole); !geometry || c.item.whole.Fit != FitCover || cropAnimated(c.item.whole) {
		return nil
	}
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	g := c.geometry()
	widths, heights := make([]float64, len(g.widths)), make([]float64, len(g.heights))
	for k := range widths {
		widths[k], heights[k] = float64(g.widths[k]), float64(g.heights[k])
	}
	return []SendCmdTrack{
		{Target: "crop@vs_crop_" + id, Option: "w", Values: widths},
		{Target: "crop@vs_crop_" + id, Option: "h", Values: heights},
	}
}

// opacityTracks drive the opacity stage: when opacity is keyframed, they hold its value for every window frame, from
// absolute frames (layerClip.at + k).
func (c clipChain) opacityTracks() []SendCmdTrack {
	if _, opacity := clipAnimation(c.item.whole); !opacity {
		return nil
	}
	values := make([]float64, c.item.clip.Duration)
	for k := range values {
		values[k], _ = ValueAt(c.item.whole, "opacity", c.item.at+k)
	}
	return []SendCmdTrack{{Target: fmt.Sprintf("colorchannelmixer@vs_opacity_%d_%d", c.track, c.index), Option: "aa", Values: values}}
}

// coverBound returns an even canvas that holds the cover-scaled picture of every window frame, and the range
// [low, high] of the window's box aspects that the static crop clamps the source aspect to.
//   - The source aspect (asset width / height) counts in both orientations, because FFmpeg's autorotation may swap it.
//   - Without a source aspect, the clamp range stands in.
//   - The range is held to 1/maxCoverAspect..maxCoverAspect (1/16 to 16, the import limit for videos). A box 2 px tall
//     next to a 1080 px one has the aspects 960 and 1.8, and a bound that covered a 1024:1 source at the taller box
//     would be 1.1 million pixels wide. With the clamp, a side of the bound is at most 16 times a box side plus the
//     margin, whatever the boxes and the source look like (TestCoverBoundStaysNearTheCanvas). A source or box within
//     1/16 to 16 is not changed: the crop above trims only what no frame of the clip shows.
//   - The margin (1/8 plus 4 px) absorbs force_divisible_by and the crop's chroma rounding for sources of at least
//     16 px.
func coverBound(widths, heights []int, sourceAspect float64) (boundWidth, boundHeight int, low, high float64) {
	low, high = math.Inf(1), 0
	for k := range widths {
		aspect := float64(widths[k]) / float64(heights[k])
		low, high = min(low, aspect), max(high, aspect)
	}
	low, high = min(max(low, 1.0/maxCoverAspect), maxCoverAspect), min(max(high, 1.0/maxCoverAspect), maxCoverAspect)
	pictureLow, pictureHigh := low, high
	if sourceAspect > 0 {
		upright, turned := min(max(sourceAspect, low), high), min(max(1/sourceAspect, low), high)
		pictureLow, pictureHigh = min(upright, turned), max(upright, turned)
	}
	var width, height float64
	for k := range widths {
		width = max(width, float64(widths[k]), float64(heights[k])*pictureHigh)
		height = max(height, float64(heights[k]), float64(widths[k])/pictureLow)
	}
	return evenCeil(width*1.125 + 4), evenCeil(height*1.125 + 4), low, high
}

// evenCeil rounds up to the next even whole number.
func evenCeil(value float64) int { return int(math.Ceil(value/2)) * 2 }
