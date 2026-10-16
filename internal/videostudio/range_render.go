package videostudio

// "Render range exactly" (spec §7.8): a short timeline range rendered through the export pipeline at a fixed preview
// height, so the editor can compare it with the live preview. The server's range_render job uses these helpers.
const (
	// RangeRenderHeight is the output height of a range render; the width follows the project's aspect ratio.
	RangeRenderHeight = 540
	// MaxRangeRenderSeconds bounds one range render. A longer range is refused, never shortened.
	MaxRangeRenderSeconds = 60
)

// RangeRenderSize is the output size of a range render: RangeRenderHeight lines and the nearest even width for the
// project's aspect ratio (H.264 4:2:0 needs even sizes), computed in integers. A project without a size gives 960x540.
func RangeRenderSize(project Project) (width, height int) {
	if project.Width <= 0 || project.Height <= 0 {
		return 960, RangeRenderHeight
	}
	h := int64(project.Height)
	w := (int64(RangeRenderHeight)*int64(project.Width) + h) / (2 * h) * 2
	if w < 2 {
		w = 2
	}
	if w > MaxSourceDimension {
		w = MaxSourceDimension
	}
	return int(w), RangeRenderHeight
}

// MaxRangeRenderFrames is the longest range render at the rate: MaxRangeRenderSeconds rounded down to whole frames
// (1798 at 29.97 fps). The editor uses the same bound (video-studio-rangerender.js).
func MaxRangeRenderFrames(rate Rate) int {
	if rate.Num <= 0 || rate.Den <= 0 {
		return 0
	}
	return int(int64(MaxRangeRenderSeconds) * rate.Num / rate.Den)
}
