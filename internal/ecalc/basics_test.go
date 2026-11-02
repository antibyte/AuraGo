package ecalc

import "testing"

// TestPhaseOneCalculators pins the eleven phase 1 calculators: order, degrees of freedom and schematic ids. The UI
// draws a schematic for every calculator that names one (the ui/ package cross-checks the ids).
func TestPhaseOneCalculators(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id, schematic string
		dof           int
	}{
		{"ohms_law", "ohms_law", 2},
		{"voltage_divider", "voltage_divider", 3},
		{"loaded_divider", "loaded_divider", 4},
		{"current_divider", "current_divider", 3},
		{"series_parallel", "series_parallel", 1},
		{"decibel_ratio", "", 2},
		{"db_levels", "", 2},
		{"wheatstone", "wheatstone", 5},
		{"star_delta", "star_delta", 3},
		{"rms_waveform", "", 1},
		{"charge_energy", "", 2},
	}
	if len(cat.Calculators) != len(want) {
		t.Fatalf("catalog has %d calculators, want %d", len(cat.Calculators), len(want))
	}
	for i, w := range want {
		c := cat.Calculators[i]
		if c.ID != w.id || c.Category != "basics" || c.Schematic != w.schematic || c.DOF != w.dof {
			t.Errorf("calculator %d = {%s %s schematic %q dof %d}, want {%s basics schematic %q dof %d}",
				i, c.ID, c.Category, c.Schematic, c.DOF, w.id, w.schematic, w.dof)
		}
	}
}
