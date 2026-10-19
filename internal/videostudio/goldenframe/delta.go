package goldenframe

import (
	"errors"
	"fmt"
	"image"
	"math"
	"strings"

	"aurago/internal/videostudio"
)

// Delta fidelity (plan 2d, index §4.12, risk 21). SSIM alone can pass a frame whose feature is missing: plan 2c measured
// 0.9987 for a sharpen the preview ignored. A case with a Delta therefore also renders its Baseline, the same case with
// the feature off, on both sides, and compares the change the feature makes:
//
//	d_export = want − want(Baseline), d_gl = got − got(Baseline), per channel in 8-bit levels.
//
// Only samples whose export change is at least MinLevels count. The check fails when:
//   - Changed, the share of all channel samples that count, is below MinChanged: the case cannot catch a missing feature;
//   - Slope, the least-squares slope through 0 of d_gl on d_export, lies outside [MinSlope, MaxSlope]: the preview
//     applies the feature more weakly or more strongly than the export;
//   - R, the Pearson correlation of d_gl and d_export, is below MinR: the preview changes other samples than the export.
type Delta struct {
	Baseline   videostudio.Project
	MinLevels  int
	MinChanged float64
	MinSlope   float64
	MaxSlope   float64
	MinR       float64
	// ReportOnly logs a failed check instead of failing the case. Plan 2f (F2) keeps it only on 2c cases whose remaining
	// difference is the preview's magnification, not the effect pass; cases_2c.go names them with the measured slopes.
	ReportOnly bool
}

// DeltaScore is what DeltaFidelity measured.
type DeltaScore struct{ Changed, Slope, R float64 }

// The default bounds; withDelta adds the Baseline. Change them only once, with the measured reason added here.
//   - MinLevels 4: above the 1–2 level difference between the export's 8-bit forms and GL's real forms (index §4.22),
//     so rounding noise never counts as change.
//   - MinChanged 0.02: about 55 000 channel samples at 1280x720. An ignored feature leaves 0.
//   - Slope 0.9..1.1: plan 2c measured 0.98–1.04 on colour, LUT, blur and sharpen (sharpen 0.98 and 1.02; the LUT at
//     intensity 0.6 gives 1.036 through 8-bit rounding). A feature at half strength gives 0.5, a missing one 0.
//   - MinR 0.98 for blend modes, whose change is a smooth per-pixel function of smooth media (plan 2c: r ≥ 0.9995 on
//     colour).
//   - MinR 0.95 for masks and keys, whose change sits at edges. There the export's quarter-resolution keyframed mask
//     (50 % edge shift ≤ 1.25 px, index §4.22) and bicubic versus bilinear upscaling move large changes by a fraction
//     of a pixel.
var (
	DeltaBlend = Delta{MinLevels: 4, MinChanged: 0.02, MinSlope: 0.9, MaxSlope: 1.1, MinR: 0.98}
	DeltaMask  = Delta{MinLevels: 4, MinChanged: 0.02, MinSlope: 0.9, MaxSlope: 1.1, MinR: 0.95}
	DeltaKey   = Delta{MinLevels: 4, MinChanged: 0.02, MinSlope: 0.9, MaxSlope: 1.1, MinR: 0.95}
)

// DeltaFidelity measures how the preview's change (got − gotBase) follows the export's change (want − wantBase), over
// the R, G and B samples whose export change is at least minLevels (index §4.12). The four images must have one size;
// alpha is ignored (the goldens are opaque). Without a counting sample, Slope and R are 0. With a change that is the
// same everywhere, R is 0, because a constant change has no correlation.
func DeltaFidelity(want, got, wantBase, gotBase image.Image, minLevels int) (DeltaScore, error) {
	if minLevels < 1 {
		return DeltaScore{}, fmt.Errorf("minLevels is %d, want at least 1", minLevels)
	}
	size := want.Bounds().Size()
	if size.X <= 0 || size.Y <= 0 {
		return DeltaScore{}, errors.New("the images are empty")
	}
	for _, other := range []struct {
		name string
		img  image.Image
	}{{"got", got}, {"want baseline", wantBase}, {"got baseline", gotBase}} {
		if other.img.Bounds().Size() != size {
			return DeltaScore{}, fmt.Errorf("image sizes differ: want %dx%d, %s %dx%d", size.X, size.Y, other.name, other.img.Bounds().Dx(), other.img.Bounds().Dy())
		}
	}
	w, g, wb, gb := channels(want), channels(got), channels(wantBase), channels(gotBase)
	var n, total int
	var sx, sy, sxx, syy, sxy float64
	for c := range w {
		for i := range w[c] {
			total++
			de := w[c][i] - wb[c][i]
			if math.Abs(de) < float64(minLevels) {
				continue
			}
			dg := g[c][i] - gb[c][i]
			n++
			sx, sy = sx+de, sy+dg
			sxx, syy, sxy = sxx+de*de, syy+dg*dg, sxy+de*dg
		}
	}
	score := DeltaScore{Changed: float64(n) / float64(total)}
	if n == 0 {
		return score, nil
	}
	score.Slope = sxy / sxx
	count := float64(n)
	vx, vy, cov := sxx-sx*sx/count, syy-sy*sy/count, sxy-sx*sy/count
	if vx > 0 && vy > 0 {
		score.R = cov / math.Sqrt(vx*vy)
	}
	return score, nil
}

// Check returns nil when score meets every bound of d, else an error naming each failed bound.
func (d Delta) Check(score DeltaScore) error {
	var failed []string
	if !(score.Changed >= d.MinChanged) {
		failed = append(failed, fmt.Sprintf("changed %.4f < %.4f: the case cannot catch a missing feature", score.Changed, d.MinChanged))
	}
	if !(score.Slope >= d.MinSlope && score.Slope <= d.MaxSlope) {
		strength := "more strongly"
		if !(score.Slope >= d.MinSlope) {
			strength = "more weakly"
		}
		failed = append(failed, fmt.Sprintf("slope %.4f outside %.2f..%.2f: the preview applies the feature %s than the export", score.Slope, d.MinSlope, d.MaxSlope, strength))
	}
	if !(score.R >= d.MinR) {
		failed = append(failed, fmt.Sprintf("r %.4f < %.2f: the preview changes other samples than the export", score.R, d.MinR))
	}
	if len(failed) == 0 {
		return nil
	}
	return errors.New(strings.Join(failed, "; "))
}

// withDelta returns c with a copy of delta whose Baseline is baseline (the same case with its feature off).
func withDelta(c Case, delta Delta, baseline videostudio.Project) Case {
	delta.Baseline = baseline
	c.Delta = &delta
	return c
}

// reportOnly returns a copy of delta that logs a failed check instead of failing (the 2c opt-ins).
func reportOnly(delta Delta) Delta {
	delta.ReportOnly = true
	return delta
}
