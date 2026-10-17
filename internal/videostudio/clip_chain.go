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
//   - 2c: picture (crop, flip), effects (one chainStage per entry) and command groups after two-input stages.
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

// stages lists the stages after window in the preview's pipeline order: picture, the effect entries, mask,
// transform and opacity.
func (c clipChain) stages() []chainStage {
	stages := []chainStage{{filter: c.picture()}}
	stages = append(stages, c.effects()...)
	return append(stages,
		chainStage{filter: c.mask()},
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

// picture crops and flips the source (plan 2c Task 7). Nothing until then.
func (c clipChain) picture() string { return "" }

// effects runs the clip's effect stack, one chainStage per on entry in stack order (plan 2c Task 8, 2d). A two-input
// entry (a LUT blend, the vignette alpha wrapper) is its own stage with twoInput set. Nothing until then.
func (c clipChain) effects() []chainStage { return nil }

// mask applies the clip's mask as alpha (plan 2d, a two-input stage). Nothing in 2c.
func (c clipChain) mask() string { return "" }

// transform fits the picture into the clip box (contain pads, cover crops), converts to RGBA and rotates it. A static
// cover clip whose source is more than 16x off the box aspect first crops the source to within that factor
// (staticCoverClamp), so the cover-scaled picture stays a bounded multiple of the box.
func (c clipChain) transform() string {
	if geometry, _ := clipAnimation(c.item.whole); geometry {
		return c.animatedTransform()
	}
	clip := c.item.clip
	_, _, boxWidth, boxHeight := clipBox(clip, c.canvasWidth, c.canvasHeight)
	scale := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0", boxWidth, boxHeight, boxWidth, boxHeight)
	if clip.Fit == FitCover {
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=%d:h=%d", boxWidth, boxHeight, boxWidth, boxHeight)
		if clamp := staticCoverClamp(boxWidth, boxHeight, c.item.sourceAspect); clamp != "" {
			scale = clamp + "," + scale
		}
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
func (c clipChain) animatedTransform() string {
	g := c.geometry()
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	pad := func(name string, width, height int) string {
		return fmt.Sprintf("pad%s=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame", name, width, height)
	}
	var parts []string
	if c.item.whole.Fit == FitCover {
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

// transformTracks drive the transform stage. When the geometry is keyframed, they hold the cover crop's size for
// every window frame: exactly the box sizes FFmpeg's scale computes, from absolute frames. 2c Task 7 adds the
// animated picture crop, which opens transform.
func (c clipChain) transformTracks() []SendCmdTrack {
	if geometry, _ := clipAnimation(c.item.whole); !geometry || c.item.whole.Fit != FitCover {
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
