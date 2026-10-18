package videostudio

import "math"

// In/out animations (plan 2e, spec §5.6, index §4.24).
//   - Clip.AnimIn and Clip.AnimOut hold a registry preset and a duration in timeline frames.
//   - The duration lies in AnimFrameRange of the project rate.
//   - The render clamps each span to half the clip (Task 4).
//
// JS VideoStudioAnim.SECONDS and frameRange mirror these numbers (Task 4, TestVideoStudioAnimConstantsMatchGo). Task 4
// adds the preset constants and the animation math to this file.

const (
	// AnimMinSeconds and AnimMaxSeconds bound an animation's duration. AnimDefaultSeconds is the length of a new one.
	AnimMinSeconds     = 0.2
	AnimMaxSeconds     = 3
	AnimDefaultSeconds = 0.5
)

// AnimFrameRange is the duration range of an in or out animation in timeline frames at rate, and the default length:
//   - the range is [max(1, round(0.2·fps)), round(3·fps)];
//   - the default is round(0.5·fps);
//   - fps = Num/Den, rounded half away from zero (math.Round).
//
// For example, 25 fps gives 5..75 (default 13), and 30 and 29.97 fps give 6..90 (default 15). An unset rate is the
// 30 fps default, as in ProjectRate.
func AnimFrameRange(rate Rate) (minFrames, maxFrames, defaultFrames int) {
	if rate.Num <= 0 || rate.Den <= 0 {
		rate = Rate{Num: FramesPerSecond, Den: 1}
	}
	frames := func(seconds float64) int { return int(math.Round(seconds * float64(rate.Num) / float64(rate.Den))) }
	return max(1, frames(AnimMinSeconds)), frames(AnimMaxSeconds), frames(AnimDefaultSeconds)
}
