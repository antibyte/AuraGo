package videostudio

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Keyframes (spec §5.1, §5.2, §5.8, plan index §4.18). Clip.Keyframes maps a base parameter path to its keys. F is
// source time in project frames (the unit of Clip.Offset), so split, trims, slip, slide and speed changes keep every
// key on the same picture content. Between keys i and i+1 the easing of key i applies; before the first and after the
// last key the value is constant. JS VideoStudioEffects mirrors every rule in this file.

// Easing names the curve from one keyframe (or ramp point) to the next.
type Easing string

const (
	EaseLinear Easing = "linear"
	EaseIn     Easing = "ease_in"
	EaseOut    Easing = "ease_out"
	EaseInOut  Easing = "ease_in_out"
	EaseHold   Easing = "hold"
)

// Keyframe limits (spec §5.8): per parameter path, and per clip (base paths and, from 2c, effect parameters).
const (
	MaxKeyframesPerParam = 64
	MaxKeyframesPerClip  = 512
	// MinBoxSize is the smallest width or height value a keyframe may hold, as a share of the canvas: the editor's 1 %
	// box minimum (JS VideoStudioEffects.MIN_BOX_SIZE). A thinner sliver would give a cover clip a box aspect of hundreds
	// and the export a pad bound of that many canvas widths.
	MinBoxSize = 0.01
)

// Keyframe is one key of a parameter: value V at source frame F, eased towards the next key with E.
type Keyframe struct {
	F int     `json:"f"` // source time in project frames (the unit of Clip.Offset)
	V float64 `json:"v"`
	E Easing  `json:"e"` // always present
}

// validEasing reports whether e is one of the five easings.
func validEasing(e Easing) bool {
	switch e {
	case EaseLinear, EaseIn, EaseOut, EaseInOut, EaseHold:
		return true
	}
	return false
}

// countKeyframes counts every keyframe of a clip toward MaxKeyframesPerClip: the base paths and the keyframes of every
// effect entry (spec §5.8).
func countKeyframes(clip Clip) int {
	total := 0
	for _, list := range clip.Keyframes {
		total += len(list)
	}
	for _, entry := range clip.Effects {
		for _, list := range entry.Keyframes {
			total += len(list)
		}
	}
	return total
}

// validateKeyframes checks a clip's keyframes (index §4.18):
//   - Only clips on visual tracks have keyframes, and only on keyframable number paths of the registry.
//   - Each path has 1..MaxKeyframesPerParam keys, and the clip at most MaxKeyframesPerClip in all.
//   - Frames are strictly increasing integers within ±MaxClipSpeed·maxFrames; source time may lie outside the media.
//   - Each value lies in its registry range (width and height at least MinBoxSize), with a known easing.
//
// Parameters are not checked against each other: x + width may exceed 1, because the export's clip box clamps it.
// crop.l|t|r|b are registry base rows (2c), so they pass here; validateEffects checks the keyframes of effect entries.
// 2d adds mask.*.
func validateKeyframes(track Track, clip Clip, maxFrames int) error {
	if len(clip.Keyframes) == 0 {
		return nil
	}
	if track.Kind == TrackAudio {
		return fmt.Errorf("clip %q on an audio track cannot have keyframes", clip.ID)
	}
	if countKeyframes(clip) > MaxKeyframesPerClip {
		return fmt.Errorf("clip %q has more than %d keyframes", clip.ID, MaxKeyframesPerClip)
	}
	bound := MaxClipSpeed * maxFrames
	for _, path := range slices.Sorted(maps.Keys(clip.Keyframes)) {
		list := clip.Keyframes[path]
		spec, ok := BaseParam(path)
		if !ok || !spec.Keyframable || spec.Type != ParamNumber {
			return fmt.Errorf("clip %q has keyframes on an unsupported path %q", clip.ID, path)
		}
		if len(list) == 0 {
			return fmt.Errorf("clip %q keyframe list %q is empty", clip.ID, path)
		}
		if len(list) > MaxKeyframesPerParam {
			return fmt.Errorf("clip %q has more than %d keyframes on %q", clip.ID, MaxKeyframesPerParam, path)
		}
		for i, key := range list {
			if i > 0 && key.F <= list[i-1].F {
				return fmt.Errorf("clip %q keyframes on %q must have strictly increasing frames", clip.ID, path)
			}
			if key.F < -bound || key.F > bound {
				return fmt.Errorf("clip %q keyframe frame %d on %q is out of range", clip.ID, key.F, path)
			}
			if !finite(key.V) || key.V < spec.Min || key.V > spec.Max || (path == "width" || path == "height") && key.V < MinBoxSize {
				return fmt.Errorf("clip %q has an invalid keyframe value %v on %q", clip.ID, key.V, path)
			}
			if !validEasing(key.E) {
				return fmt.Errorf("clip %q has an unsupported easing %q on %q", clip.ID, key.E, path)
			}
		}
	}
	return nil
}

// Ease maps a span's progress t (clamped to 0..1) through an easing curve (spec §5.2, plan index §4.18):
//   - linear: t;
//   - ease_in: t³;
//   - ease_out: 1 − (1 − t)³;
//   - ease_in_out: 4t³ below one half, 1 − (2 − 2t)³/2 from there;
//   - hold: 0 until t reaches 1.
//
// Unknown names are linear. JS VideoStudioEffects.ease computes the same numbers (parity topic "easing").
func Ease(e Easing, t float64) float64 {
	if !(t > 0) {
		t = 0
	} else if t > 1 {
		t = 1
	}
	switch e {
	case EaseIn:
		return t * t * t
	case EaseOut:
		u := 1 - t
		return 1 - u*u*u
	case EaseInOut:
		if t < 0.5 {
			return 4 * t * t * t
		}
		u := -2*t + 2
		return 1 - u*u*u/2
	case EaseHold:
		if t < 1 {
			return 0
		}
		return 1
	}
	return t
}

// sampleKeyframes is the value of a keyframe list at source position f (fractional):
//   - up to the first key, the first value;
//   - from the last key on, the last value;
//   - between keys i and i+1, the value eased with key i's easing.
//
// The list must be sorted by F (Validate guarantees it); an empty list gives 0. JS: VideoStudioEffects.sample.
func sampleKeyframes(list []Keyframe, f float64) float64 {
	if len(list) == 0 {
		return 0
	}
	if !(f > float64(list[0].F)) {
		return list[0].V
	}
	last := list[len(list)-1]
	if f >= float64(last.F) {
		return last.V
	}
	i := sort.Search(len(list), func(i int) bool { return float64(list[i].F) > f }) - 1
	a, b := list[i], list[i+1]
	t := (f - float64(a.F)) / float64(b.F-a.F)
	return a.V + (b.V-a.V)*Ease(a.E, t)
}

// ValueAt returns a keyframable path of a clip at absolute timeline frame frame (plan index §4.4, §4.16). Paths:
//   - the base paths x, y, width, height, rotation, opacity and crop.l|t|r|b (0 without a crop);
//   - effects.<i>.<param>, a number parameter of effect entry i: its keyframes, else its params value, else the
//     registry default.
//
// With keyframes on the path, the list is sampled at ClipSourceAt(clip, frame), the source position the clip shows
// there: offset + (frame − start)·speed, or offset + S(frame − start) with a ramp. Keyframes thus stay on the picture
// content (spec §5.1). Without keyframes it returns the static value, and the frame may lie outside the clip. ok is
// false for any other path, for an unknown entry or effect, and for a parameter that is not a number. JS:
// VideoStudioEffects.valueAt.
func ValueAt(clip Clip, path string, frame int) (float64, bool) {
	var static float64
	switch path {
	case "x":
		static = clip.X
	case "y":
		static = clip.Y
	case "width":
		static = clip.Width
	case "height":
		static = clip.Height
	case "rotation":
		static = clip.Rotation
	case "opacity":
		static = clip.Opacity
	case "crop.l", "crop.t", "crop.r", "crop.b":
		static = cropValue(clip.Crop, path)
	default:
		return effectEntryValue(clip, path, frame)
	}
	if list := clip.Keyframes[path]; len(list) > 0 {
		return sampleKeyframes(list, ClipSourceAt(clip, float64(frame))), true
	}
	return static, true
}

// cropValue is one side of a clip's static crop (path crop.l, crop.t, crop.r or crop.b); it is 0 without a crop.
func cropValue(crop *Crop, path string) float64 {
	if crop == nil {
		return 0
	}
	switch path {
	case "crop.l":
		return crop.L
	case "crop.t":
		return crop.T
	case "crop.r":
		return crop.R
	}
	return crop.B
}

// effectEntryValue resolves effects.<i>.<param> for ValueAt. <i> is a plain decimal index into clip.Effects (the entry
// may be on or off), and <param> must be a number parameter of the entry's registry effect.
func effectEntryValue(clip Clip, path string, frame int) (float64, bool) {
	rest, ok := strings.CutPrefix(path, "effects.")
	if !ok {
		return 0, false
	}
	indexText, param, ok := strings.Cut(rest, ".")
	index, err := strconv.Atoi(indexText)
	if !ok || err != nil || index < 0 || index >= len(clip.Effects) || strconv.Itoa(index) != indexText {
		return 0, false
	}
	entry := clip.Effects[index]
	spec, ok := effectParamSpec(entry.ID, param)
	if !ok || spec.Type != ParamNumber {
		return 0, false
	}
	if list := entry.Keyframes[param]; len(list) > 0 {
		return sampleKeyframes(list, ClipSourceAt(clip, float64(frame))), true
	}
	if value, ok := entry.Params[param].(float64); ok && finite(value) {
		return value, true
	}
	value, _ := spec.Default.(float64)
	return value, true
}

// clipFrame is the resolved picture state of a visual clip at one timeline frame. It is the Go side of the JS
// clipState for the keyframable base properties. The export's animated chain and its tests read it, and 2e applies
// in/out animation here, on top of the keyframes.
type clipFrame struct {
	X, Y, Width, Height, Rotation, Opacity float64
}

// clipFrameAt resolves every keyframable base property of a clip at absolute timeline frame frame through ValueAt.
func clipFrameAt(clip Clip, frame int) clipFrame {
	value := func(path string) float64 {
		v, _ := ValueAt(clip, path, frame)
		return v
	}
	return clipFrame{X: value("x"), Y: value("y"), Width: value("width"), Height: value("height"), Rotation: value("rotation"), Opacity: value("opacity")}
}

// clipAnimation reports which parts of a clip's picture can change over time:
//   - geometry, when x, y, width, height or rotation has keyframes;
//   - opacity, when opacity has keyframes.
//
// A path with a single keyframe counts, because its value replaces the static field. 2e also marks clips with in/out
// animation.
func clipAnimation(clip Clip) (geometry, opacity bool) {
	for _, path := range []string{"x", "y", "width", "height", "rotation"} {
		if len(clip.Keyframes[path]) > 0 {
			geometry = true
		}
	}
	return geometry, len(clip.Keyframes["opacity"]) > 0
}

// SendCmdTrack is one runtime option of one Go-named single-input filter instance. It holds one entry per window
// frame of a clip (window frame k = absolute frame layerClip.at + k). Exactly one of Values and Curves is set.
type SendCmdTrack struct {
	Target string         // "<filter>@vs_<role>_<L>_<C>[_<I>]", checked against sendCmdTargetPattern (I = stack index)
	Option string         // allowed per "<filter>@vs_<role>" in sendCmdOptions
	Values []float64      // numeric tracks; finite; all tracks of one file have the same length
	Curves []ChannelCurve // expression tracks (sendCmdExprOptions only), written through channelCurveExpr
}

// sendCmdTargetPattern is the only shape of a command target: a Go-named filter instance "<filter>@vs_<role>_<L>_<C>".
// Effect filters append the entry's stack index. The first group is the sendCmdOptions key.
var sendCmdTargetPattern = regexp.MustCompile(`^([a-z][a-z0-9]*@vs_[a-z]+)_[0-9]+_[0-9]+(?:_[0-9]+)?$`)

// sendCmdOptions lists, per "<filter>@vs_<role>", the options a script may set. Only single-input filters belong here:
// sendcmd to a two-input filter (overlay, blend) lands 1–3 frames early on FFmpeg 8.1 (index §4.11, risk 10). Commands
// behind a two-input stage go into a later command group (assembleStages). 2d extends the list.
var sendCmdOptions = map[string][]string{
	"colorchannelmixer@vs_opacity": {"aa"},
	"crop@vs_crop":                 {"w", "h"},
	"crop@vs_picture":              {"w", "h", "x", "y"},
	"lutrgb@vs_color":              {"r", "g", "b"},
	"colorchannelmixer@vs_sat":     {"rr", "rg", "rb", "gr", "gg", "gb", "br", "bg", "bb"},
	"cas@vs_sharpen":               {"strength"},
}

// sendCmdExprOptions are the options that take colour-curve expressions instead of numbers (spike 2026-10-17: lutrgb
// r/g/b expression commands are exact to 1 LSB on 5.1.9 and 8.1.1). Numeric tracks are refused for them, and curve
// tracks for every other option.
var sendCmdExprOptions = map[string][]string{
	"lutrgb@vs_color": {"r", "g", "b"},
}

var (
	sendCmdCurvePattern = regexp.MustCompile(`^[0-9a-z.()*/+,-]+$`)
	sendCmdCurveWords   = regexp.MustCompile(`[a-z]+`)
)

// validSendCmdCurve reports whether text is inside the colour-curve grammar: digits, '.', parentheses, the four
// operators and commas, with letters only in clip, round, pow and val.
func validSendCmdCurve(text string) bool {
	if !sendCmdCurvePattern.MatchString(text) {
		return false
	}
	for _, word := range sendCmdCurveWords.FindAllString(text, -1) {
		switch word {
		case "clip", "round", "pow", "val":
		default:
			return false
		}
	}
	return true
}

// sendCmdCurveValue writes one colour curve as a single-quoted expression argument.
func sendCmdCurveValue(curve ChannelCurve) (string, error) {
	for _, v := range []float64{curve.Mul, curve.Contrast, curve.Lift, curve.Gain, curve.InvGamma} {
		if !finite(v) {
			return "", fmt.Errorf("a colour curve has a non-finite number")
		}
	}
	text := channelCurveExpr(curve).String()
	if !validSendCmdCurve(text) {
		return "", fmt.Errorf("colour curve expression %q is outside the sendcmd grammar", text)
	}
	return "'" + text + "'", nil
}

func sendCmdTrackLength(track SendCmdTrack) int { return max(len(track.Values), len(track.Curves)) }

// sendCmdUnchanged reports whether window frame k repeats frame k−1.
func sendCmdUnchanged(track SendCmdTrack, k int) bool {
	if len(track.Curves) > 0 {
		return track.Curves[k] == track.Curves[k-1]
	}
	return track.Values[k] == track.Values[k-1]
}

// WriteSendCmd writes one clip's sendcmd script (0o600).
//   - Frame k's commands start half a frame early ((k − 0.5)·Den/Num seconds, %.6f; frame 0 at 0), so NTSC
//     microsecond rounding never moves a command to a neighbour frame.
//   - The commands of one frame share one interval (comma-separated), and a value equal to the previous frame's is
//     skipped.
//   - Only Go-formatted numbers, grammar-checked curve expressions, checked targets and allowed options are written.
func WriteSendCmd(path string, rate Rate, tracks []SendCmdTrack) error {
	script, err := sendCmdScript(rate, tracks)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write sendcmd script: %w", err)
	}
	return nil
}

// sendCmdScript returns the script WriteSendCmd writes, or an error for an invalid rate or track.
func sendCmdScript(rate Rate, tracks []SendCmdTrack) (string, error) {
	if rate.Num <= 0 || rate.Den <= 0 {
		return "", fmt.Errorf("sendcmd script needs a valid frame rate")
	}
	if len(tracks) == 0 || sendCmdTrackLength(tracks[0]) == 0 {
		return "", fmt.Errorf("sendcmd script needs a track with values")
	}
	frames := sendCmdTrackLength(tracks[0])
	seen := make(map[string]bool, len(tracks))
	texts := make([][]string, len(tracks))
	for i, track := range tracks {
		match := sendCmdTargetPattern.FindStringSubmatch(track.Target)
		if match == nil {
			return "", fmt.Errorf("sendcmd target %q is not a Go-named filter instance", track.Target)
		}
		curves := len(track.Curves) > 0
		if curves == (len(track.Values) > 0) {
			return "", fmt.Errorf("sendcmd track %s %s must hold either numbers or colour curves", track.Target, track.Option)
		}
		expression := slices.Contains(sendCmdExprOptions[match[1]], track.Option)
		if !slices.Contains(sendCmdOptions[match[1]], track.Option) || curves != expression {
			return "", fmt.Errorf("sendcmd option %q is not allowed for %q", track.Option, track.Target)
		}
		key := track.Target + " " + track.Option
		if seen[key] {
			return "", fmt.Errorf("sendcmd track %s appears twice", key)
		}
		seen[key] = true
		if sendCmdTrackLength(track) != frames {
			return "", fmt.Errorf("sendcmd track %s has %d values, want %d", key, sendCmdTrackLength(track), frames)
		}
		texts[i] = make([]string, frames)
		for k := range frames {
			if curves {
				text, err := sendCmdCurveValue(track.Curves[k])
				if err != nil {
					return "", fmt.Errorf("sendcmd track %s: %w", key, err)
				}
				texts[i][k] = text
				continue
			}
			if !finite(track.Values[k]) {
				return "", fmt.Errorf("sendcmd track %s has a non-finite value", key)
			}
			texts[i][k] = strconv.FormatFloat(track.Values[k], 'f', -1, 64)
		}
	}
	var script strings.Builder
	commands := make([]string, 0, len(tracks))
	for k := range frames {
		commands = commands[:0]
		for i, track := range tracks {
			if k > 0 && sendCmdUnchanged(track, k) {
				continue
			}
			commands = append(commands, track.Target+" "+track.Option+" "+texts[i][k])
		}
		if len(commands) == 0 {
			continue
		}
		seconds := 0.0
		if k > 0 {
			seconds = (float64(k) - 0.5) * float64(rate.Den) / float64(rate.Num)
		}
		fmt.Fprintf(&script, "%.6f %s;\n", seconds, strings.Join(commands, ", "))
	}
	return script.String(), nil
}
