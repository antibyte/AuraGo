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
//   - 2b: retime (speed ramps) and runtime-command targets in transform and opacity.
//   - 2c: picture (crop, flip) and effects.
//   - 2d: mask and the blend path of place.
type clipChain struct {
	track, index              int
	item                      layerClip
	canvasWidth, canvasHeight int
	rate                      Rate
}

// filter returns the clip's chain ending in the label output. In 2a it is, byte for byte, the graph the export built
// before the split (TestClipChainSnapshots).
func (c clipChain) filter(output string) string {
	parts := make([]string, 0, 7)
	for _, stage := range []string{c.retime(), c.window(), c.picture(), c.effects(), c.mask(), c.transform(), c.opacity()} {
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

// picture crops and flips the source (plan 2c). Nothing in 2a.
func (c clipChain) picture() string { return "" }

// effects runs the clip's effect stack (plans 2c and 2d). Nothing in 2a.
func (c clipChain) effects() string { return "" }

// mask applies the clip's mask as alpha (plan 2d). Nothing in 2a.
func (c clipChain) mask() string { return "" }

// transform fits the picture into the clip box (contain pads, cover crops), converts to RGBA and rotates it.
func (c clipChain) transform() string {
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

// opacity scales the picture's alpha; full opacity adds no filter.
func (c clipChain) opacity() string {
	if c.item.clip.Opacity < 0.999999 {
		return fmt.Sprintf("colorchannelmixer=aa=%.6f", c.item.clip.Opacity)
	}
	return ""
}

// place overlays the clip layer on a transparent canvas-size background at the clip box (a rotated clip keeps its box
// centre) and ends in the label output.
func (c clipChain) place(output string) string {
	clip := c.item.clip
	x, y, boxWidth, boxHeight := clipBox(clip, c.canvasWidth, c.canvasHeight)
	layer := fmt.Sprintf("vs_layer_%d_%d", c.track, c.index)
	background := fmt.Sprintf("vs_clip_bg_%d_%d", c.track, c.index)
	positionX, positionY := fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)
	if clipRotated(clip) {
		positionX = fmt.Sprintf("'%d+%d/2-overlay_w/2'", x, boxWidth)
		positionY = fmt.Sprintf("'%d+%d/2-overlay_h/2'", y, boxHeight)
	}
	return fmt.Sprintf("[%s];color=c=black@0:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s];[%s][%s]overlay=x=%s:y=%s:format=auto:shortest=1[%s]", layer, c.canvasWidth, c.canvasHeight, c.rate, c.rate.SecondsArg(clip.Duration), clip.Duration, background, background, layer, positionX, positionY, output)
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
