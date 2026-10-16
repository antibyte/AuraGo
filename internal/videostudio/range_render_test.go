package videostudio

import "testing"

func TestRangeRenderSizeKeepsTheAspectAtFiveFortyLines(t *testing.T) {
	for _, tc := range []struct{ width, height, want int }{
		{1280, 720, 960}, {1920, 1080, 960}, {3840, 2160, 960},
		{720, 1280, 304}, {1080, 1920, 304},
		{720, 720, 540}, {1080, 1350, 432},
	} {
		width, height := RangeRenderSize(Project{Width: tc.width, Height: tc.height})
		if width != tc.want || height != RangeRenderHeight {
			t.Errorf("%dx%d renders at %dx%d, want %dx%d", tc.width, tc.height, width, height, tc.want, RangeRenderHeight)
		}
	}
	for _, size := range CanvasSizes() {
		width, height := RangeRenderSize(Project{Width: size[0], Height: size[1]})
		if height != 540 || width < 2 || width%2 != 0 {
			t.Errorf("%dx%d renders at %dx%d; want 540 lines and an even width", size[0], size[1], width, height)
		}
	}
	if width, height := RangeRenderSize(Project{}); width != 960 || height != 540 {
		t.Errorf("a project without a size renders at %dx%d, want 960x540", width, height)
	}
}

func TestMaxRangeRenderFramesIsSixtySeconds(t *testing.T) {
	for _, tc := range []struct {
		rate Rate
		want int
	}{
		{Rate{24, 1}, 1440}, {Rate{25, 1}, 1500}, {Rate{30, 1}, 1800}, {Rate{50, 1}, 3000}, {Rate{60, 1}, 3600},
		{Rate{24000, 1001}, 1438}, {Rate{30000, 1001}, 1798}, {Rate{60000, 1001}, 3596},
	} {
		got := MaxRangeRenderFrames(tc.rate)
		if got != tc.want {
			t.Errorf("%s: %d frames, want %d", tc.rate, got, tc.want)
		}
		if tc.rate.Seconds(got) > MaxRangeRenderSeconds+1e-9 || tc.rate.Seconds(got+1) <= MaxRangeRenderSeconds {
			t.Errorf("%s: %d frames is not the longest whole-frame range within %d s", tc.rate, got, MaxRangeRenderSeconds)
		}
	}
	if got := MaxRangeRenderFrames(Rate{}); got != 0 {
		t.Errorf("an invalid rate allows %d frames, want 0", got)
	}
}
