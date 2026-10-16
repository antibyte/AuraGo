package videostudio

import (
	"fmt"
	"math"
	"strings"
)

// clipChain builds one visual clip's export filter chain (spec §7.1, plan index §4.10): the clip's input is retimed
// onto the project frame grid, cut to its window, transformed into its box and placed on a transparent canvas-size
// layer. Each stage returns its filters without the separating comma, or "" when it adds nothing; filter joins the
// stages in the preview's pipeline order. Later slices extend a stage, never the assembly in filter:
//   - 2b: retime (speed ramps), the commands stage (sendcmd) and runtime-command targets in transform and opacity.
//   - 2c: picture (crop, flip) and effects.
//   - 2d: mask and the blend path of place.
type clipChain struct {
	track, index              int
	item                      layerClip
	canvasWidth, canvasHeight int
	rate                      Rate
}

// filter returns the clip's chain ending in the label output. A clip without keyframes and ramp produces, byte for
// byte, the 2a graph (TestClipChainSnapshots).
func (c clipChain) filter(output string) string {
	parts := make([]string, 0, 8)
	for _, stage := range []string{c.retime(), c.window(), c.commands(), c.picture(), c.effects(), c.mask(), c.transform(), c.opacity()} {
		if stage != "" {
			parts = append(parts, stage)
		}
	}
	return fmt.Sprintf("[%d:V:0]%s", c.item.input, strings.Join(parts, ",")) + c.place(output)
}

// retime samples the source on the project frame grid. Images start at zero. Video keeps absolute timestamps, so fps
// samples it on the source's own cadence and output frame from becomes the clip's first frame. When the video stream
// starts after the container (for example behind leading audio) no source frame covers the first positions; the
// first decoded frame is then moved back to from, so fps holds it there instead of the clip running short. The video
// path is videoResample, which the still capture (derive.go) shares so a still shows the frame the renderer shows.
func (c clipChain) retime() string {
	if c.item.image {
		return fmt.Sprintf("setpts=PTS-STARTPTS,fps=%s", c.rate)
	}
	return videoResample(c.item.from, c.item.clip.Offset, ClipSpeed(c.item.clip), c.rate)
}

// window shifts the retimed frames into the clip and bounds clone padding to a small multiple of the clip's duration.
func (c clipChain) window() string {
	clip := c.item.clip
	return fmt.Sprintf("setpts='max(PTS-%d,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS", c.item.first, clip.Duration, clip.Duration*2, clip.Duration)
}

// commands reads the clip's sendcmd script (index §4.11) when it drives single-input filters: the keyframed opacity and
// the cover crop of keyframed geometry. It sits right after window, whose timestamps restart at the window's first
// frame, so the script's times are window-local; only single-input filters lie between it and its targets (a
// two-input filter in between would let commands overtake their frames).
func (c clipChain) commands() string {
	if file, ok := c.commandFile(); ok {
		return "sendcmd=f=" + file.name
	}
	return ""
}

// picture crops and flips the source (plan 2c). Nothing in 2a.
func (c clipChain) picture() string { return "" }

// effects runs the clip's effect stack (plans 2c and 2d). Nothing in 2a.
func (c clipChain) effects() string { return "" }

// mask applies the clip's mask as alpha (plan 2d). Nothing in 2a.
func (c clipChain) mask() string { return "" }

// transform fits the picture into the clip box (contain pads, cover crops), converts to RGBA and rotates it.
func (c clipChain) transform() string {
	if geometry, _ := clipAnimation(c.item.whole); geometry {
		return c.animatedTransform()
	}
	clip := c.item.clip
	_, _, boxWidth, boxHeight := clipBox(clip, c.canvasWidth, c.canvasHeight)
	scale := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0", boxWidth, boxHeight, boxWidth, boxHeight)
	if clip.Fit == FitCover {
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=%d:h=%d", boxWidth, boxHeight, boxWidth, boxHeight)
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
// with the window starting at the clip's start.
func (c clipChain) geometry() geometryPlan {
	whole := c.item.whole
	g := geometryPlan{box: boxExprs(whole, c.item.at, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate)}
	step := float64(c.rate.Den) / float64(c.rate.Num)
	sizes := func(box boxExpr, frames int) (widths, heights []int, rotated bool) {
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
	g.widths, g.heights, g.rotated = sizes(g.box, c.item.clip.Duration)
	clipWidths, clipHeights := g.widths, g.heights
	if c.item.at != whole.Start || c.item.clip.Duration != whole.Duration {
		clipWidths, clipHeights, g.rotated = sizes(boxExprs(whole, whole.Start, c.item.pieces, c.canvasWidth, c.canvasHeight, c.rate), whole.Duration)
	}
	for k := range clipWidths {
		g.padWidth, g.padHeight = max(g.padWidth, clipWidths[k]), max(g.padHeight, clipHeights[k])
	}
	if whole.Fit == FitCover {
		g.boundWidth, g.boundHeight, g.low, g.high = coverBound(clipWidths, clipHeights, c.item.sourceAspect)
	}
	g.layerWidth, g.layerHeight = g.padWidth, g.padHeight
	if g.rotated {
		g.padWidth, g.padHeight = g.padWidth+2, g.padHeight+2
		side := evenCeil(math.Hypot(float64(g.padWidth), float64(g.padHeight)))
		g.layerWidth, g.layerHeight = side, side
	}
	return g
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
			fmt.Sprintf("crop=w='min(iw,ih*%s)':h='min(ih,iw/%s)'", exprNum(g.high).String(), exprNum(g.low).String()),
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

// commandFile is the clip's sendcmd script for this FFmpeg process (index §4.11):
//   - the opacity of every window frame when opacity is keyframed;
//   - the cover crop's size of every window frame when the geometry is keyframed.
//
// Values come from absolute frames (layerClip.at + k), and the crop sizes are exactly the box sizes FFmpeg's scale
// computes. ok is false when the clip drives no runtime command.
func (c clipChain) commandFile() (sendCmdFile, bool) {
	geometry, opacity := clipAnimation(c.item.whole)
	id := fmt.Sprintf("%d_%d", c.track, c.index)
	var tracks []SendCmdTrack
	if opacity {
		values := make([]float64, c.item.clip.Duration)
		for k := range values {
			values[k], _ = ValueAt(c.item.whole, "opacity", c.item.at+k)
		}
		tracks = append(tracks, SendCmdTrack{Target: "colorchannelmixer@vs_opacity_" + id, Option: "aa", Values: values})
	}
	if geometry && c.item.whole.Fit == FitCover {
		g := c.geometry()
		widths, heights := make([]float64, len(g.widths)), make([]float64, len(g.heights))
		for k := range widths {
			widths[k], heights[k] = float64(g.widths[k]), float64(g.heights[k])
		}
		tracks = append(tracks,
			SendCmdTrack{Target: "crop@vs_crop_" + id, Option: "w", Values: widths},
			SendCmdTrack{Target: "crop@vs_crop_" + id, Option: "h", Values: heights})
	}
	if len(tracks) == 0 {
		return sendCmdFile{}, false
	}
	return sendCmdFile{name: "vs_cmd_" + id + ".txt", tracks: tracks}, true
}

// coverBound returns an even canvas that holds the cover-scaled picture of every window frame, and the range
// [low, high] of the window's box aspects that the static crop clamps the source aspect to.
//   - The source aspect (asset width / height) counts in both orientations, because FFmpeg's autorotation may swap it.
//   - Without a source aspect, the clamp range stands in.
//   - The margin (1/8 plus 4 px) absorbs force_divisible_by and the crop's chroma rounding for sources of at least
//     16 px.
func coverBound(widths, heights []int, sourceAspect float64) (boundWidth, boundHeight int, low, high float64) {
	low, high = math.Inf(1), 0
	for k := range widths {
		aspect := float64(widths[k]) / float64(heights[k])
		low, high = min(low, aspect), max(high, aspect)
	}
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
