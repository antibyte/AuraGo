package ecalc

import (
	"strings"
	"testing"
)

// TestPhaseOneCalculators pins the basics category (the eleven calculators of phase 1 and the standard-value combiner
// of phase 2): its ids in order, degrees of freedom and schematic ids. Calculators of the other categories do not
// matter here, so adding a category file never breaks this test. The UI draws a schematic for every calculator that
// names one (the ui/ package cross-checks the ids).
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
		{"eseries_combiner", "", 1},
	}
	var basics []*Calculator
	for _, c := range cat.Calculators {
		if c.Category == "basics" {
			basics = append(basics, c)
		}
	}
	if len(basics) != len(want) {
		t.Fatalf("the basics category has %d calculators, want %d", len(basics), len(want))
	}
	for i, w := range want {
		c := basics[i]
		if c.ID != w.id || c.Schematic != w.schematic || c.DOF != w.dof {
			t.Errorf("basics calculator %d = {%s schematic %q dof %d}, want {%s schematic %q dof %d}",
				i, c.ID, c.Schematic, c.DOF, w.id, w.schematic, w.dof)
		}
	}
}

// TestESeriesCombinerItem pins the shape of the standard-value combiner: one scalar resistance (the target, 5 kOhm),
// dof 1, the combine finder, no rules, no formula, no schematic and no chart; the page and the agent list the
// combinations of the selected E-series.
func TestESeriesCombinerItem(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	c := cat.Calculator("eseries_combiner")
	if c == nil {
		t.Fatal("the catalog has no eseries_combiner")
	}
	if c.Category != "basics" || c.Kind != "calculator" || c.Finder != "combine" || c.DOF != 1 || c.Schematic != "" || c.Chart != nil ||
		len(c.Rules) != 0 || len(c.Derived) != 0 || len(c.Formula) != 0 || len(c.Choices) != 0 || len(c.Vars) != 1 {
		t.Errorf("unexpected shape: %+v", c)
	}
	v := c.Vars[0]
	if v.ID != "target" || v.Dim != "resistance" || v.Label != "R" || v.List || !v.Input || v.ESeries || v.Default == nil || *v.Default != 5000 {
		t.Errorf("target var = %+v, want a scalar resistance input R with the default 5 kOhm", v)
	}
	if len(c.Name) != len(Locales) || c.Name["en"] != "Standard-value combiner" || c.Doc["en"] == "" || c.Doc["de"] == "" || len(c.Tags) < 5 {
		t.Errorf("name %v doc %v tags %v", c.Name, c.Doc, c.Tags)
	}
	if !strings.Contains(c.Source, "IEC 60063") {
		t.Errorf("source %q does not cite IEC 60063", c.Source)
	}
}
