package goldenframe

import (
	"image"
	"math"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

const deltaWidth, deltaHeight = 64, 32

// deltaImage is an opaque 64x32 test image whose three channels all hold level(x, y). rgba picks *image.RGBA (a painter
// read-back) instead of *image.NRGBA (a decoded PNG), so both pixel layouts of channels() are exercised.
func deltaImage(rgba bool, level func(x, y int) int) image.Image {
	pix := make([]uint8, deltaWidth*deltaHeight*4)
	for y := 0; y < deltaHeight; y++ {
		for x := 0; x < deltaWidth; x++ {
			i := (y*deltaWidth + x) * 4
			v := uint8(level(x, y))
			pix[i], pix[i+1], pix[i+2], pix[i+3] = v, v, v, 255
		}
	}
	rect := image.Rect(0, 0, deltaWidth, deltaHeight)
	if rgba {
		return &image.RGBA{Pix: pix, Stride: deltaWidth * 4, Rect: rect}
	}
	return &image.NRGBA{Pix: pix, Stride: deltaWidth * 4, Rect: rect}
}

// deltaFlat is mid grey. deltaChange adds step·(x − 32) and a ±noise checkerboard to it. With step 2 the change runs
// from −64 to +62 levels, and only the columns x = 31..33 change by less than 4 levels, so Changed is 61/64. The noise
// sums to zero in every column, so it never moves the slope.
func deltaFlat(rgba bool) image.Image { return deltaImage(rgba, func(int, int) int { return 128 }) }

func deltaChange(rgba bool, step, noise int) image.Image {
	return deltaImage(rgba, func(x, y int) int {
		n := noise
		if (x+y)%2 == 1 {
			n = -noise
		}
		return 128 + step*(x-32) + n
	})
}

// TestDeltaFidelity checks the measure on synthetic frames (index §4.12): an identical change gives slope 1 and r 1, a
// halved one slope 0.5, an ignored one slope 0 and r 0, and no change Changed 0.
func TestDeltaFidelity(t *testing.T) {
	want, wantBase, gotBase := deltaChange(false, 2, 0), deltaFlat(false), deltaFlat(true)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for _, c := range []struct {
		name              string
		want, got         image.Image
		changed, slope, r float64
	}{
		{"identical change", want, deltaChange(true, 2, 0), 61.0 / 64, 1, 1},
		{"halved change", want, deltaChange(true, 1, 0), 61.0 / 64, 0.5, 1},
		{"ignored feature", want, deltaFlat(true), 61.0 / 64, 0, 0},
		{"no change", deltaFlat(false), deltaFlat(true), 0, 0, 0},
	} {
		score, err := DeltaFidelity(c.want, c.got, wantBase, gotBase, 4)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !near(score.Changed, c.changed) || !near(score.Slope, c.slope) || !near(score.R, c.r) {
			t.Errorf("%s: %+v, want changed %v, slope %v, r %v", c.name, score, c.changed, c.slope, c.r)
		}
	}
	// Zero-mean noise of ±2 levels keeps the slope at exactly 1 and lowers r a little: sqrt(var/(var + 4)) = 0.9986.
	score, err := DeltaFidelity(want, deltaChange(true, 2, 2), wantBase, gotBase, 4)
	if err != nil || !near(score.Slope, 1) || score.R <= 0.998 || score.R >= 0.9995 {
		t.Errorf("noisy change: %+v (%v), want slope 1 and r about 0.9986", score, err)
	}
	if _, err := DeltaFidelity(want, image.NewRGBA(image.Rect(0, 0, 8, 8)), wantBase, gotBase, 4); err == nil || !strings.Contains(err.Error(), "sizes differ") {
		t.Errorf("a size mismatch must fail, got %v", err)
	}
	if _, err := DeltaFidelity(want, want, wantBase, gotBase, 0); err == nil {
		t.Error("minLevels 0 must fail: every sample would count as changed")
	}
}

// TestDeltaCheckAndDefaults pins the default bounds (index §4.12) and the messages Check gives per failed bound.
func TestDeltaCheckAndDefaults(t *testing.T) {
	for name, delta := range map[string]Delta{"DeltaBlend": DeltaBlend, "DeltaMask": DeltaMask, "DeltaKey": DeltaKey} {
		r := 0.95
		if name == "DeltaBlend" {
			r = 0.98
		}
		if delta.MinLevels != 4 || delta.MinChanged != 0.02 || delta.MinSlope != 0.9 || delta.MaxSlope != 1.1 || delta.MinR != r || delta.ReportOnly ||
			delta.Baseline.Name != "" || len(delta.Baseline.Tracks) != 0 {
			t.Errorf("%s = %+v, want MinLevels 4, MinChanged 0.02, slope 0.9..1.1, MinR %v, no baseline, not report-only", name, delta, r)
		}
	}
	if err := DeltaBlend.Check(DeltaScore{Changed: 0.95, Slope: 1, R: 1}); err != nil {
		t.Errorf("a faithful change failed: %v", err)
	}
	for _, c := range []struct {
		name     string
		delta    Delta
		score    DeltaScore
		has, not []string
	}{
		{"halved", DeltaBlend, DeltaScore{Changed: 0.95, Slope: 0.5, R: 1}, []string{"slope 0.5000", "more weakly"}, []string{"cannot catch", "other samples"}},
		{"doubled", DeltaBlend, DeltaScore{Changed: 0.95, Slope: 2, R: 1}, []string{"slope 2.0000", "more strongly"}, []string{"cannot catch", "other samples"}},
		{"unchanged", DeltaKey, DeltaScore{}, []string{"changed 0.0000", "cannot catch a missing feature"}, nil},
		{"uncorrelated", DeltaBlend, DeltaScore{Changed: 0.95, Slope: 1, R: 0.96}, []string{"r 0.9600 < 0.98", "other samples"}, []string{"cannot catch", "more "}},
	} {
		err := c.delta.Check(c.score)
		if err == nil {
			t.Errorf("%s: Check passed %+v", c.name, c.score)
			continue
		}
		for _, s := range c.has {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: %q lacks %q", c.name, err, s)
			}
		}
		for _, s := range c.not {
			if strings.Contains(err.Error(), s) {
				t.Errorf("%s: %q names a bound that holds (%q)", c.name, err, s)
			}
		}
	}
	if err := DeltaMask.Check(DeltaScore{Changed: 0.5, Slope: 1, R: 0.96}); err != nil {
		t.Errorf("masks and keys accept r 0.96 (their change sits at edges): %v", err)
	}
	// A heavily noisy preview fails on r alone: r = sqrt(1431.9/(1431.9 + 900)) = 0.7836, slope exactly 1.
	score, err := DeltaFidelity(deltaChange(false, 2, 0), deltaChange(true, 2, 30), deltaFlat(false), deltaFlat(true), 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeltaBlend.Check(score); err == nil || !strings.Contains(err.Error(), "r 0.78") || strings.Contains(err.Error(), "slope") {
		t.Errorf("noise ±30: %+v gave %v, want an r failure only (r about 0.78)", score, err)
	}
	// withDelta and reportOnly work on copies, so the defaults never change.
	c := withDelta(Case{Name: "blend_x"}, reportOnly(DeltaBlend), videostudio.Project{Name: "base"})
	if c.Delta == nil || c.Delta.Baseline.Name != "base" || !c.Delta.ReportOnly || DeltaBlend.ReportOnly || DeltaBlend.Baseline.Name != "" {
		t.Errorf("withDelta/reportOnly changed the defaults or lost the baseline: %+v, DeltaBlend %+v", c.Delta, DeltaBlend)
	}
}
