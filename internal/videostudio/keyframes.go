package videostudio

import (
	"fmt"
	"maps"
	"slices"
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

// countKeyframes counts every keyframe of a clip toward MaxKeyframesPerClip. 2c adds the keyframes of effect entries.
func countKeyframes(clip Clip) int {
	total := 0
	for _, list := range clip.Keyframes {
		total += len(list)
	}
	return total
}

// validateKeyframes checks a clip's keyframes (index §4.18):
//   - Only clips on visual tracks have keyframes, and only on keyframable number paths of the registry.
//   - Each path has 1..MaxKeyframesPerParam keys, and the clip at most MaxKeyframesPerClip in all.
//   - Frames are strictly increasing integers within ±MaxClipSpeed·maxFrames; source time may lie outside the media.
//   - Each value lies in its registry range (width and height above 0), with a known easing.
//
// Parameters are not checked against each other: x + width may exceed 1, because the export's clip box clamps it.
// 2c adds crop.* and effect paths, 2d mask.*.
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
			if !finite(key.V) || key.V < spec.Min || key.V > spec.Max || (path == "width" || path == "height") && key.V <= 0 {
				return fmt.Errorf("clip %q has an invalid keyframe value %v on %q", clip.ID, key.V, path)
			}
			if !validEasing(key.E) {
				return fmt.Errorf("clip %q has an unsupported easing %q on %q", clip.ID, key.E, path)
			}
		}
	}
	return nil
}
