package storymaker

import (
	"fmt"
	"strings"
)

// freeAPICardText is the code API in short, for the context of a free fill. It must stay within 1800 bytes with the two lines
// FreeAPICard adds; the code reference holds the long form.
const freeAPICardText = `API card for this code scene. Read storymaker_catalog reference topic code only when this card is not enough.
export default function scene(api) { ... } runs once and builds the scene; each frame is a pure function of t.
timing = {start, dur, ease}; ease is linear, ease-in, ease-out, ease-in-out, back-out or bounce-out. api.cue(n) is the second of narration marker {n}.
api.duration(seconds)
api.safe() returns {x, y, w, h}: the drawing area of this format in 1920x1080 design pixels, above the captions.
api.text(text, 'title' or 'label', {at}) returns a handle.
api.shape('rect', 'circle' or 'line', {at, w, h, fill, stroke}) returns a handle; w and h are design pixels.
handle.moveTo(at, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale, timing), rotateTo(degrees, timing)
api.tween(handle, {opacity, x, y, scale, rotation, width, height}, timing)
api.camera.zoom(factor, timing), api.camera.pan(at, timing), api.camera.shake(amount, timing)
api.random('seed') returns a seeded random function: call it while building, never in onFrame.
api.onFrame(function (t, g) { ... }): g.save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path(svgPath, 'fill', 'stroke' or 'both'), text(text, x, y, {size, weight, align, color}). g draws in design pixels.
at is {x, y} in percent of the safe area, or a spot: left, center, right.
Colours: the palette tokens $bg, $bg2, $fg, $accent, $accent2, $text, $outline, or #rrggbb.
Never: api.add, Math.random, Date, timers, imports, the DOM, or state that changes from frame to frame.`

// FreeAPICard is the card of a free fill packet: freeAPICardText, a palette line and a safe_box line, joined by "\n". The
// palette line names the tokens of the film's palette in catalog order and is left out when there are none.
func FreeAPICard(tokens map[string]string, box SafeBox) string {
	lines := []string{freeAPICardText}
	var parts []string
	for _, name := range paletteTokenNames {
		if hex := tokens[name]; hex != "" {
			parts = append(parts, "$"+name+" "+hex)
		}
	}
	if len(parts) > 0 {
		lines = append(lines, "Palette of this film: "+strings.Join(parts, ", "))
	}
	lines = append(lines, fmt.Sprintf("safe_box: x %d, y %d, w %d, h %d", box.X, box.Y, box.W, box.H))
	return strings.Join(lines, "\n")
}
