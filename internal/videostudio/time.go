package videostudio

import (
	"fmt"
	"math"
)

// AudioSampleRate is the sample rate of every rendered audio stream.
const AudioSampleRate = 48000

// Rate is an exact frame rate in frames per second (Num/Den). NTSC rates use Den 1001.
type Rate struct {
	Num int64
	Den int64
}

// ProjectRate returns the project's exact frame rate. An unset rate means the v1 default of 30 fps.
func ProjectRate(project Project) Rate {
	fps := int64(project.FPS)
	if fps <= 0 {
		fps = FramesPerSecond
	}
	if project.NTSC {
		return Rate{Num: fps * 1000, Den: 1001}
	}
	return Rate{Num: fps, Den: 1}
}

// Seconds converts frames to seconds.
func (r Rate) Seconds(frames int) float64 {
	return float64(frames) * float64(r.Den) / float64(r.Num)
}

// SecondsFloat converts a fractional frame count to seconds (source positions of clips with a speed).
func (r Rate) SecondsFloat(frames float64) float64 {
	return frames * float64(r.Den) / float64(r.Num)
}

// SecondsArg formats frames as an FFmpeg time argument with microsecond precision.
func (r Rate) SecondsArg(frames int) string {
	return fmt.Sprintf("%.6f", r.Seconds(frames))
}

// String formats the rate for FFmpeg options such as fps= and -r.
func (r Rate) String() string {
	if r.Den == 1 {
		return fmt.Sprintf("%d", r.Num)
	}
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

// FramesAt converts a media duration to whole frames. A frame counts once its start lies inside the
// duration; the 1/1000-frame tolerance absorbs microsecond rounding. The JS editor uses the same formula
// (Math.ceil(us * num / (den * 1e6) - 1e-3)), so both sides agree on every asset length.
func (r Rate) FramesAt(durationUs int64) int {
	if durationUs <= 0 {
		return 0
	}
	frames := math.Ceil(float64(durationUs)*float64(r.Num)/(float64(r.Den)*1e6) - 1e-3)
	if frames < 1 {
		return 1
	}
	if frames > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(frames)
}

// SampleAt returns the first 48 kHz audio sample of a timeline frame.
func (r Rate) SampleAt(frame int) int64 {
	return int64(frame) * AudioSampleRate * r.Den / r.Num
}
