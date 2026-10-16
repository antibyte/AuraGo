package videostudio

import "fmt"

// Speed ramp limits (spec §5.5, §5.8, plan index §4.18).
const (
	MaxRampPoints = 16
	// MaxRampPieces bounds the linear-speed pieces of one ramp inside a clip: 15 eased spans of four sub-pieces plus the
	// constant pieces before the first and after the last point make 62.
	MaxRampPieces = 64
)

// RampPoint is one point of a clip's speed ramp. F is clip time (timeline frames from the clip start; points outside
// the clip are kept, so trims never change the curve inside it), Speed the playback speed there, and E the easing
// towards the next point.
type RampPoint struct {
	F     int     `json:"f"`
	Speed float64 `json:"speed"`
	E     Easing  `json:"e"`
}

// validateRamp checks a ramp's shape (index §4.18). A ramp is allowed only on a video-track clip with a video asset.
// It has 2..MaxRampPoints points with strictly increasing integer frames within ±maxFrames, speeds MinClipSpeed..
// MaxClipSpeed and known easings, and Clip.Speed absent or 1. validateClip calls it before the source extent rule
// reads the ramp.
func validateRamp(track Track, clip Clip, asset Asset, maxFrames int) error {
	if len(clip.Ramp) == 0 {
		return nil
	}
	if track.Kind != TrackVideo || asset.Kind != AssetVideo {
		return fmt.Errorf("clip %q can have a speed ramp only on a video track with a video asset", clip.ID)
	}
	if len(clip.Ramp) < 2 || len(clip.Ramp) > MaxRampPoints {
		return fmt.Errorf("clip %q speed ramp needs 2 to %d points", clip.ID, MaxRampPoints)
	}
	if clip.Speed != 0 && clip.Speed != 1 {
		return fmt.Errorf("clip %q cannot combine a speed ramp with speed %v", clip.ID, clip.Speed)
	}
	for i, point := range clip.Ramp {
		if i > 0 && point.F <= clip.Ramp[i-1].F {
			return fmt.Errorf("clip %q speed ramp points must have strictly increasing frames", clip.ID)
		}
		if point.F < -maxFrames || point.F > maxFrames {
			return fmt.Errorf("clip %q speed ramp point frame %d is out of range", clip.ID, point.F)
		}
		if !finite(point.Speed) || point.Speed < MinClipSpeed || point.Speed > MaxClipSpeed {
			return fmt.Errorf("clip %q ramp speed %v must be between 0.25 and 16", clip.ID, point.Speed)
		}
		if !validEasing(point.E) {
			return fmt.Errorf("clip %q has an unsupported ramp easing %q", clip.ID, point.E)
		}
	}
	return nil
}
