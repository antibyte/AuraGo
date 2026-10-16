package videostudio

import (
	"math"
	"testing"
)

// Sixteen eased points make the most pieces a clip can have: 15 spans of rampEaseSteps sub-pieces plus a constant
// piece before the first and after the last point (index §4.18).
func TestRampPiecesStayWithinLimit(t *testing.T) {
	if rampEaseSteps != 4 {
		t.Fatalf("rampEaseSteps = %d, want 4 (JS LIMITS.rampEaseSteps)", rampEaseSteps)
	}
	want := (MaxRampPoints-1)*rampEaseSteps + 2
	if want > MaxRampPieces {
		t.Fatalf("%d pieces exceed MaxRampPieces %d", want, MaxRampPieces)
	}
	points := make([]RampPoint, MaxRampPoints)
	for i := range points {
		points[i] = RampPoint{F: 5 + 10*i, Speed: 0.25 + float64(i%4), E: EaseInOut}
	}
	pieces := rampPieces(points, 0, 200)
	if len(pieces) != want {
		t.Fatalf("rampPieces = %d pieces, want %d", len(pieces), want)
	}
	if pieces[0].t0 != 0 || pieces[len(pieces)-1].t1 != 200 {
		t.Fatalf("pieces span %v..%v, want 0..200", pieces[0].t0, pieces[len(pieces)-1].t1)
	}
	for i := 1; i < len(pieces); i++ {
		prev, d := pieces[i-1], pieces[i-1].t1-pieces[i-1].t0
		if prev.t1 != pieces[i].t0 || math.Abs(prev.s0+d*(prev.a+prev.b*d/2)-pieces[i].s0) > 1e-9 {
			t.Fatalf("pieces %d and %d do not join: %+v %+v", i-1, i, prev, pieces[i])
		}
	}
}

// A linear span is one piece, a hold span one constant piece, an eased span rampEaseSteps sub-pieces whose node speeds
// follow Ease; the constant piece after the last point carries the target speed.
func TestRampPiecesFollowEasing(t *testing.T) {
	for _, c := range []struct {
		e      Easing
		pieces int
	}{
		{EaseLinear, 2}, {EaseHold, 2}, {EaseIn, rampEaseSteps + 1}, {EaseOut, rampEaseSteps + 1}, {EaseInOut, rampEaseSteps + 1},
	} {
		pieces := rampPieces([]RampPoint{{F: 0, Speed: 1, E: c.e}, {F: 40, Speed: 3, E: EaseLinear}}, 0, 50)
		if len(pieces) != c.pieces {
			t.Errorf("%s: %d pieces, want %d", c.e, len(pieces), c.pieces)
			continue
		}
		for k := 0; k <= rampEaseSteps; k++ {
			at := 40 * float64(k) / rampEaseSteps
			want := 1 + 2*Ease(c.e, float64(k)/rampEaseSteps)
			if got := rampSpeed(pieces, at); math.Abs(got-want) > 1e-12 {
				t.Errorf("%s: speed at %v = %v, want %v", c.e, at, got, want)
			}
		}
	}
}

// S is strictly increasing, S(0) = 0, and rampInverse undoes rampSource, inside and outside the pieces.
func TestRampSourceInverseRoundTrip(t *testing.T) {
	ramps := [][]RampPoint{
		{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 3, E: EaseLinear}},
		{{F: -10, Speed: 2, E: EaseHold}, {F: 10, Speed: 0.5, E: EaseInOut}, {F: 30, Speed: 16, E: EaseOut}, {F: 45, Speed: 0.25, E: EaseIn}},
	}
	for i, points := range ramps {
		pieces := rampPieces(points, -20, 80)
		if s := rampSource(pieces, 0); math.Abs(s) > 1e-12 {
			t.Errorf("ramp %d: S(0) = %v, want 0", i, s)
		}
		prev := math.Inf(-1)
		for x := -30.0; x <= 90; x += 0.25 {
			s := rampSource(pieces, x)
			if s <= prev {
				t.Fatalf("ramp %d: S does not increase at %v", i, x)
			}
			prev = s
			if back := rampInverse(pieces, s); math.Abs(back-x) > 1e-9 {
				t.Errorf("ramp %d: rampInverse(S(%v)) = %v", i, x, back)
			}
		}
	}
	if rampSource(nil, 7) != 7 || rampInverse(nil, 7) != 7 || rampSpeed(nil, 7) != 1 {
		t.Fatal("an empty piece list must play at speed 1")
	}
}

// Nodes depend only on their span, so a head trim or split that shifts the points by k keeps the curve exactly:
// S'(t) = S(t + k) - S(k), also when k cuts inside an eased span (index §4.18).
func TestRampTrimShiftsCurve(t *testing.T) {
	points := []RampPoint{{F: 0, Speed: 1, E: EaseInOut}, {F: 40, Speed: 4, E: EaseHold}, {F: 60, Speed: 0.5, E: EaseOut}, {F: 90, Speed: 2, E: EaseLinear}}
	whole := rampPieces(points, 0, 120)
	for _, k := range []int{7, 40, 53} {
		shifted := make([]RampPoint, len(points))
		for i, p := range points {
			shifted[i] = RampPoint{F: p.F - k, Speed: p.Speed, E: p.E}
		}
		part := rampPieces(shifted, 0, float64(120-k))
		for x := 0.0; x <= float64(120-k); x += 0.5 {
			want := rampSource(whole, x+float64(k)) - rampSource(whole, float64(k))
			if got := rampSource(part, x); math.Abs(got-want) > 1e-9 {
				t.Fatalf("head trim %d: S'(%v) = %v, want %v", k, x, got, want)
			}
		}
	}
}

// ClipSourceAt is offset + S(frame - start) with a ramp and offset + (frame - start)·speed without; keyframes follow it.
func TestClipSourceAtFollowsRamp(t *testing.T) {
	ramped := Clip{Start: 100, Offset: 10, Duration: 30, Ramp: []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 3, E: EaseLinear}}}
	fast := Clip{Start: 100, Offset: 30, Duration: 30, Speed: 2}
	for _, c := range []struct {
		name  string
		clip  Clip
		frame float64
		want  float64
	}{
		{"ramp halfway up", ramped, 110, 25},
		{"ramp inside the accelerating span", ramped, 105, 16.25},
		{"ramp after the last point", ramped, 125, 65},
		{"ramp before the clip at the first point's speed", ramped, 96, 6},
		{"constant speed", fast, 105, 40},
		{"constant speed at a fractional frame", fast, 100.5, 31},
	} {
		if got := ClipSourceAt(c.clip, c.frame); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: ClipSourceAt(%v) = %v, want %v", c.name, c.frame, got, c.want)
		}
	}
	if got := clipSourceExtent(ramped); math.Abs(got-67) > 1e-9 {
		t.Errorf("ramped extent = %v, want S(29) = 40 + 9*3 = 67", got)
	}
	if got := clipSourceExtent(fast); got != 58 {
		t.Errorf("speed 2 extent = %v, want 29*2 = 58", got)
	}
	ramped.Keyframes = map[string][]Keyframe{"x": {{F: 10, V: 0, E: EaseLinear}, {F: 50, V: 1, E: EaseLinear}}}
	if got, ok := ValueAt(ramped, "x", 110); !ok || math.Abs(got-0.375) > 1e-12 {
		t.Errorf("ValueAt with a ramp = %v, %v; want 0.375 (source 25)", got, ok)
	}
}
