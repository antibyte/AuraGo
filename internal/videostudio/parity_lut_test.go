package videostudio

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

// TestParityLUT runs the shared .cube fixture (plan index §4.13, topic "lut") through ParseCube and Sample with the
// intensity mix (1−k)·in + k·L(in) of §4.20. JS runs the same file through VideoStudioPicture.parseCube and sampleLUT.
// A case with "unit_domain": true samples the table resampled onto [0, 1] (unitDomainLUT; JS unitDomain): the table
// FFmpeg reads from the staged copy and the preview uploads.
// A refused table must fail on the same line on both sides. The expected numbers are exact by construction (linear
// tables and a separable 3³ table), so this topic needs no fixture writer.
func TestParityLUT(t *testing.T) {
	for _, c := range readParityCases(t, "lut") {
		var input struct {
			Cube       string       `json:"cube"`
			RGB        [][3]float64 `json:"rgb"`
			Intensity  *float64     `json:"intensity"`
			UnitDomain bool         `json:"unit_domain"`
		}
		var want struct {
			Size      int          `json:"size"`
			RGB       [][3]float64 `json:"rgb"`
			ErrorLine int          `json:"error_line"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("%s input: %v", c.Name, err)
		}
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Fatalf("%s want: %v", c.Name, err)
		}
		table, err := ParseCube(strings.NewReader(input.Cube))
		if want.ErrorLine > 0 {
			var lutErr *LUTError
			if !errors.As(err, &lutErr) || lutErr.Line != want.ErrorLine {
				t.Errorf("%s: %v, want a refusal on line %d", c.Name, err, want.ErrorLine)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if input.UnitDomain {
			table = unitDomainLUT(table)
		}
		if table.Size != want.Size || len(want.RGB) != len(input.RGB) {
			t.Errorf("%s: grid %d, want %d (%d points, %d expected)", c.Name, table.Size, want.Size, len(input.RGB), len(want.RGB))
			continue
		}
		k := 1.0
		if input.Intensity != nil {
			k = *input.Intensity
		}
		for i, rgb := range input.RGB {
			sampled := table.Sample(rgb)
			for ch := range 3 {
				if got := (1-k)*rgb[ch] + k*sampled[ch]; math.Abs(got-want.RGB[i][ch]) > 1e-6 {
					t.Errorf("%s: point %d channel %d = %v, want %v", c.Name, i, ch, got, want.RGB[i][ch])
				}
			}
		}
	}
}
