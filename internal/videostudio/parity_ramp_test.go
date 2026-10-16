package videostudio

import (
	"encoding/json"
	"math"
	"testing"
)

// TestParityRamp runs the shared ramp fixture (plan index §4.13): the pieces, S, its inverse and the speed. The JS side
// runs the same file in TestVideoStudioEffectsParity.
func TestParityRamp(t *testing.T) {
	for _, c := range readParityCases(t, "ramp") {
		var input struct {
			Points []RampPoint `json:"points"`
			From   float64     `json:"from"`
			To     float64     `json:"to"`
			T      []float64   `json:"t"`
			S      []float64   `json:"s"`
		}
		var want struct {
			Pieces  [][5]float64 `json:"pieces"`
			Source  []float64    `json:"source"`
			Inverse []float64    `json:"inverse"`
			Speed   []float64    `json:"speed"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("%s input: %v", c.Name, err)
		}
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Fatalf("%s want: %v", c.Name, err)
		}
		pieces := rampPieces(input.Points, input.From, input.To)
		got := make([][5]float64, len(pieces))
		for i, p := range pieces {
			got[i] = [5]float64{p.t0, p.t1, p.s0, p.a, p.b}
		}
		if len(got) != len(want.Pieces) {
			t.Errorf("%s: %d pieces %v, want %d %v", c.Name, len(got), got, len(want.Pieces), want.Pieces)
			continue
		}
		for i := range got {
			for j := range got[i] {
				if math.Abs(got[i][j]-want.Pieces[i][j]) > 1e-6 {
					t.Errorf("%s: piece %d = %v, want %v", c.Name, i, got[i], want.Pieces[i])
					break
				}
			}
		}
		check := func(label string, xs, wants []float64, f func(float64) float64) {
			if len(xs) != len(wants) {
				t.Errorf("%s %s: %d inputs but %d wanted values", c.Name, label, len(xs), len(wants))
				return
			}
			for i, x := range xs {
				if value := f(x); math.Abs(value-wants[i]) > 1e-6 {
					t.Errorf("%s %s(%v) = %v, want %v", c.Name, label, x, value, wants[i])
				}
			}
		}
		check("source", input.T, want.Source, func(x float64) float64 { return rampSource(pieces, x) })
		check("inverse", input.S, want.Inverse, func(s float64) float64 { return rampInverse(pieces, s) })
		check("speed", input.T, want.Speed, func(x float64) float64 { return rampSpeed(pieces, x) })
	}
}
