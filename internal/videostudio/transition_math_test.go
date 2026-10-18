package videostudio

import (
	"fmt"
	"math"
	"testing"
)

// The shared per-frame transition parameters (plan 2e D1, index §4.24) against the spike of 2026-10-18 (p2espike
// analysis-8.txt):
//   - the D = 25 schedule at 1920×1080;
//   - the snap-audit counterexamples, where the float32 edge is 1 px below the rational one;
//   - P at the first and last overlap frame;
//   - the measured fade values of a flat white → black pair (FFmpeg 8.1.1 and 5.1.9: trunc(255·P) with the
//     float32 P);
//   - the 2a wipe bands, column 0 of wipeleft's frame 0 included (D3).
func TestTransitionFrameMatchesSpikeSchedule(t *testing.T) {
	// "parameter schedules", D = 25 at 1920×1080. Columns:
	//   - the xfade wipeleft edge (the user's wiperight) and the xfade wiperight edge (the user's wipeleft);
	//   - the pixelize block, printed to 2 decimals;
	//   - zoomin zf and w, printed to 4 decimals;
	//   - the circlecrop radius, printed to 1 decimal;
	//   - the rectcrop half sizes.
	schedule := []struct {
		xfadeWipeLeft, xfadeWipeRight int
		block, zf, w, r               float64
		zw, zh                        int
	}{
		{1920, 0, 0.00, 1.0000, 1.0000, 1101.5, 960, 540},
		{1843, 76, 6.48, 0.9818, 1.0000, 857.7, 883, 496},
		{1766, 153, 8.64, 0.9314, 1.0000, 652.8, 806, 453},
		{1689, 230, 12.96, 0.8548, 1.0000, 483.5, 729, 410},
		{1612, 307, 17.28, 0.7583, 1.0000, 346.3, 652, 367},
		{1536, 383, 21.60, 0.6480, 1.0000, 237.9, 576, 324},
		{1459, 460, 25.92, 0.5300, 1.0000, 154.9, 499, 280},
		{1382, 537, 30.24, 0.4104, 1.0000, 93.8, 422, 237},
		{1305, 614, 34.56, 0.2955, 1.0000, 51.4, 345, 194},
		{1228, 691, 38.88, 0.1913, 1.0000, 24.2, 268, 151},
		{1152, 767, 43.20, 0.1040, 1.0000, 8.8, 192, 108},
		{1075, 844, 47.52, 0.0397, 1.0000, 1.9, 115, 64},
		{998, 921, 51.84, 0.0047, 1.0000, 0.1, 38, 21},
		{921, 998, 51.84, 0.0000, 0.9953, 0.1, 38, 21},
		{844, 1075, 47.52, 0.0000, 0.9603, 1.9, 115, 64},
		{767, 1152, 43.20, 0.0000, 0.8960, 8.8, 192, 108},
		{691, 1228, 38.88, 0.0000, 0.8087, 24.2, 268, 151},
		{614, 1305, 34.56, 0.0000, 0.7045, 51.4, 345, 194},
		{537, 1382, 30.24, 0.0000, 0.5896, 93.8, 422, 237},
		{460, 1459, 25.92, 0.0000, 0.4700, 154.9, 499, 280},
		{383, 1536, 21.60, 0.0000, 0.3520, 237.9, 576, 324},
		{307, 1612, 19.44, 0.0000, 0.2417, 346.3, 652, 367}, // pixelize: float32 d·50 ceils to 9, not 8
		{230, 1689, 12.96, 0.0000, 0.1452, 483.5, 729, 410},
		{153, 1766, 8.64, 0.0000, 0.0686, 652.8, 806, 453},
		{76, 1843, 6.48, 0.0000, 0.0182, 857.7, 883, 496},
	}
	near := func(got float32, want, half float64) bool { return math.Abs(float64(got)-want) <= half+1e-9 }
	for i, want := range schedule {
		label := fmt.Sprintf("D=25 i=%d", i)
		if got := transitionFrameAt(TransitionWipeRight, i, 25, 1920, 1080).Edge; got != want.xfadeWipeLeft {
			t.Errorf("%s: wiperight (xfade wipeleft) edge = %d, want %d", label, got, want.xfadeWipeLeft)
		}
		if got := transitionFrameAt(TransitionWipeLeft, i, 25, 1920, 1080).Edge; got != want.xfadeWipeRight {
			t.Errorf("%s: wipeleft (xfade wiperight) edge = %d, want %d", label, got, want.xfadeWipeRight)
		}
		if got := transitionFrameAt("pixelize", i, 25, 1920, 1080).Sq; !near(got, want.block, 0.005) {
			t.Errorf("%s: pixelize block = %v, want %.2f", label, got, want.block)
		}
		zoom := transitionFrameAt("zoomin", i, 25, 1920, 1080)
		if !near(zoom.Zf, want.zf, 0.00005) || !near(zoom.W, want.w, 0.00005) {
			t.Errorf("%s: zoomin zf, w = %v, %v; want %.4f, %.4f", label, zoom.Zf, zoom.W, want.zf, want.w)
		}
		if got := transitionFrameAt("circlecrop", i, 25, 1920, 1080).Radius; !near(got, want.r, 0.05) {
			t.Errorf("%s: circlecrop r = %v, want %.1f", label, got, want.r)
		}
		if rect := transitionFrameAt("rectcrop", i, 25, 1920, 1080); rect.ZW != want.zw || rect.ZH != want.zh || rect.CX != 960 || rect.CY != 540 {
			t.Errorf("%s: rectcrop = %d×%d about (%d, %d), want %d×%d about (960, 540)", label, rect.ZW, rect.ZH, rect.CX, rect.CY, want.zw, want.zh)
		}
	}
	// The snap audit: float32 edges where the rational floor(S·(D − i)/D) or floor(S·i/D) is 1 px larger.
	edge := func(f transitionFrame) int { return f.Edge }
	shift := func(f transitionFrame) int { return f.Shift }
	zh := func(f transitionFrame) int { return f.ZH }
	for _, c := range []struct {
		kind          TransitionType
		i, D          int
		width, height int
		field         func(transitionFrame) int
		want          int
	}{
		{"wipeup", 3, 5, 320, 180, edge, 71},            // rational 72
		{"wipeup", 4, 5, 320, 180, edge, 35},            // rational 36
		{"wipedown", 1, 5, 320, 180, edge, 35},          // rational 36
		{"wipedown", 2, 5, 320, 180, edge, 71},          // rational 72
		{TransitionWipeLeft, 1, 5, 180, 320, edge, 35},  // xfade wiperight on a 180 px wide frame; rational 36
		{TransitionWipeRight, 3, 5, 180, 320, edge, 71}, // xfade wipeleft; rational 72
		{"slideup", 3, 5, 320, 180, shift, 71},          // rational 72
		{"rectcrop", 1, 6, 320, 180, zh, 59},            // rational 60
	} {
		if got := c.field(transitionFrameAt(c.kind, c.i, c.D, c.width, c.height)); got != c.want {
			t.Errorf("%s i=%d D=%d %dx%d: %d, want the float32 value %d", c.kind, c.i, c.D, c.width, c.height, got, c.want)
		}
	}
	if sq := transitionFrameAt("pixelize", 1, 10, 320, 180).Sq; math.Abs(float64(sq)-2.16) > 1e-4 {
		t.Errorf("pixelize i=1 D=10 at 320x180: block %v, want 2.16 (float32 d·50 ceils to 6; the rational 5 gives 1.8)", sq)
	}
	// P is 1 at frame 0 and f(1 − f((D−1)/D)) ≈ 1/D at the last frame, never 0; Q = f(1 − P).
	for _, D := range []int{1, 3, 7, 20, 25, 90} {
		first, last := transitionFrameAt(TransitionDissolve, 0, D, 64, 36), transitionFrameAt(TransitionDissolve, D-1, D, 64, 36)
		if first.P != 1 || first.Q != 0 {
			t.Errorf("D=%d frame 0: P, Q = %v, %v; want 1, 0", D, first.P, first.Q)
		}
		wantLast := float32(1 - float32(float32(D-1)/float32(D)))
		if last.P != wantLast || !(last.P > 0) || math.Abs(float64(last.P)-1/float64(D)) > 1e-6 || last.Q != float32(1-last.P) {
			t.Errorf("D=%d last frame: P, Q = %v, %v; want %v ≈ 1/D and f(1 − P)", D, last.P, last.Q, wantLast)
		}
	}
	// Out-of-range input is clamped like JS transitionParams: D ≥ 1, i in 0..D−1. A kind outside the registry gets P and Q.
	if transitionFrameAt(TransitionDissolve, 30, 25, 64, 36) != transitionFrameAt(TransitionDissolve, 24, 25, 64, 36) ||
		transitionFrameAt(TransitionDissolve, -3, 0, 64, 36) != (transitionFrame{P: 1, Q: 0}) {
		t.Error("transitionFrameAt must clamp i into 0..D−1 and D to at least 1")
	}
	if got := transitionFrameAt("not-a-transition", 5, 20, 64, 36); got != (transitionFrame{P: 0.75, Q: 0.25}) {
		t.Errorf("an unknown kind = %+v, want P and Q only", got)
	}
	// The measured fade of a flat white → black pair (analysis-8.txt "progress convention"): trunc(mix(255, 0, P)).
	measured := []int{255, 244, 234, 224, 214, 204, 193, 183, 173, 163, 153, 142, 132, 122, 112, 101, 91, 81, 71, 61, 50, 40, 30, 20, 10}
	for i, want := range measured {
		if got := int(mix32(255, 0, transitionFrameAt(TransitionDissolve, i, 25, 320, 180).P)); got != want {
			t.Errorf("fade white → black, i=%d: %d, want the measured %d", i, got, want)
		}
	}
	// The 2a wipes (D3): the user's wipeleft (xfade wiperight, x > z shows A) shows B on [0, z + 1), so frame 0 already
	// shows column 0. The user's wiperight (xfade wipeleft, x > z shows B) shows B on [z + 1, W).
	for _, c := range []struct {
		kind         TransitionType
		i, D, width  int
		left, reveal int
	}{
		{TransitionWipeLeft, 0, 20, 1280, 0, 1},
		{TransitionWipeLeft, 5, 20, 1280, 0, 321},
		{TransitionWipeRight, 0, 20, 1280, 1280, 0},
		{TransitionWipeRight, 5, 20, 1280, 961, 319},
		{TransitionWipeLeft, 1, 5, 180, 0, 36},
		{TransitionWipeRight, 3, 5, 180, 72, 108},
		{TransitionWipeLeft, 19, 20, 640, 0, 609},
	} {
		left, reveal, ok := wipeEdge(c.kind, c.i, c.D, c.width)
		if !ok || left != c.left || reveal != c.reveal {
			t.Errorf("wipeEdge(%s, %d, %d, %d) = %d, %d, %t; want %d, %d", c.kind, c.i, c.D, c.width, left, reveal, ok, c.left, c.reveal)
		}
	}
	for _, kind := range []TransitionType{TransitionDissolve, TransitionBlack, "wipeup"} {
		if _, _, ok := wipeEdge(kind, 3, 10, 1280); ok {
			t.Errorf("wipeEdge(%s) reported an edge; only the 2a wipes have one", kind)
		}
	}
}
