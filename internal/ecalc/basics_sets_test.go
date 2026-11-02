package ecalc

import (
	"reflect"
	"testing"
)

// TestSolvableSetsOfBasics pins the input sets that determine each calculator. Sets are listed in the order of the
// variable declarations. A missing set is a missing rule direction; an extra one would be a rule that cannot work.
func TestSolvableSetsOfBasics(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id   string
		sets [][]string
	}{
		{"ohms_law", [][]string{{"u", "i"}, {"u", "r"}, {"u", "p"}, {"i", "r"}, {"i", "p"}, {"r", "p"}}},
		{"voltage_divider", [][]string{{"vin", "r1", "r2"}, {"vin", "r1", "vout"}, {"vin", "r2", "vout"}, {"r1", "r2", "vout"}}},
		{"loaded_divider", [][]string{
			{"vin", "r1", "r2", "rl"}, {"vin", "r1", "r2", "vout"}, {"vin", "r1", "rl", "vout"},
			{"vin", "r2", "rl", "vout"}, {"r1", "r2", "rl", "vout"},
		}},
		// The three currents {i, i1, i2} cannot fix the resistors, only their ratio, so that set is absent.
		{"current_divider", [][]string{
			{"i", "r1", "r2"}, {"i", "r1", "i1"}, {"i", "r1", "i2"}, {"i", "r2", "i1"}, {"i", "r2", "i2"},
			{"r1", "r2", "i1"}, {"r1", "r2", "i2"}, {"r1", "i1", "i2"}, {"r2", "i1", "i2"},
		}},
		{"series_parallel", [][]string{{"rs"}}},
		{"decibel_ratio", [][]string{{"a1", "a2"}, {"a1", "g"}, {"a2", "g"}}},
		{"db_levels", [][]string{{"v", "r"}, {"v", "p"}, {"r", "p"}}},
		{"wheatstone", [][]string{
			{"vs", "r1", "r2", "r3", "r4"}, {"vs", "r1", "r2", "r3", "vd"}, {"vs", "r1", "r2", "r4", "vd"},
			{"vs", "r1", "r3", "r4", "vd"}, {"vs", "r2", "r3", "r4", "vd"}, {"r1", "r2", "r3", "r4", "vd"},
		}},
		// Exactly the delta set and the star set: a mix such as {rab, rbc, ra} has no explicit rule path.
		{"star_delta", [][]string{{"rab", "rbc", "rca"}, {"ra", "rb", "rc"}}},
		{"rms_waveform", [][]string{{"vrms"}, {"vpk"}, {"vpp"}, {"vavg"}}},
		{"charge_energy", [][]string{{"q", "v"}, {"q", "e"}, {"v", "e"}}},
		// The combiner has one input and no rules: the target alone determines everything.
		{"eseries_combiner", [][]string{{"target"}}},
	}
	for _, w := range want {
		c := cat.Calculator(w.id)
		if c == nil {
			t.Errorf("calculator %s is missing", w.id)
			continue
		}
		if !reflect.DeepEqual(c.Sets, w.sets) {
			t.Errorf("%s: solvable sets\n got %v\nwant %v", w.id, c.Sets, w.sets)
		}
	}
}
