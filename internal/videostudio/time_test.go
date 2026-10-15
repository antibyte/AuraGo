package videostudio

import "testing"

func TestProjectRateAndConversions(t *testing.T) {
	cases := []struct {
		fps  int
		ntsc bool
		want Rate
		text string
	}{
		{30, false, Rate{30, 1}, "30"},
		{0, false, Rate{30, 1}, "30"},
		{25, false, Rate{25, 1}, "25"},
		{24, true, Rate{24000, 1001}, "24000/1001"},
		{30, true, Rate{30000, 1001}, "30000/1001"},
		{60, true, Rate{60000, 1001}, "60000/1001"},
	}
	for _, c := range cases {
		got := ProjectRate(Project{FPS: c.fps, NTSC: c.ntsc})
		if got != c.want || got.String() != c.text {
			t.Fatalf("ProjectRate(%d, ntsc=%v) = %+v %q, want %+v %q", c.fps, c.ntsc, got, got.String(), c.want, c.text)
		}
	}
	thirty, ntsc := Rate{30, 1}, Rate{30000, 1001}
	if got := thirty.SecondsArg(45); got != "1.500000" {
		t.Fatalf("SecondsArg(45) = %q", got)
	}
	if got := ntsc.SecondsArg(30); got != "1.001000" {
		t.Fatalf("NTSC SecondsArg(30) = %q", got)
	}
	for _, c := range []struct {
		rate Rate
		us   int64
		want int
	}{
		{thirty, 0, 0}, {thirty, 1, 1}, {thirty, 1_000_000, 30}, {thirty, 1_033_333, 31},
		{Rate{60, 1}, 2_000_000, 120}, {ntsc, 1_001_000, 30}, {Rate{25, 1}, 10_000_000, 250},
	} {
		if got := c.rate.FramesAt(c.us); got != c.want {
			t.Fatalf("%v.FramesAt(%d) = %d, want %d", c.rate, c.us, got, c.want)
		}
	}
	if thirty.SampleAt(30) != 48000 || ntsc.SampleAt(30) != 48048 || (Rate{25, 1}).SampleAt(1) != 1920 {
		t.Fatalf("SampleAt mismatch: %d %d %d", thirty.SampleAt(30), ntsc.SampleAt(30), (Rate{25, 1}).SampleAt(1))
	}
}
