package ecalc

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

func testUnits(t *testing.T) *Units {
	t.Helper()
	data, err := os.ReadFile("catalog/units.yaml")
	if err != nil {
		t.Fatal(err)
	}
	u, err := loadUnits(data)
	if err != nil {
		t.Fatalf("units.yaml: %v", err)
	}
	return u
}

func TestUnitsVectors(t *testing.T) {
	var f struct {
		Parse []struct {
			In   string  `json:"in"`
			Dim  string  `json:"dim"`
			Unit string  `json:"unit"`
			Want float64 `json:"want"`
		} `json:"units_parse"`
		ParseError []struct {
			In   string `json:"in"`
			Dim  string `json:"dim"`
			Unit string `json:"unit"`
		} `json:"units_parse_error"`
		List []struct {
			In   string    `json:"in"`
			Dim  string    `json:"dim"`
			Want []float64 `json:"want"`
		} `json:"units_list"`
		ListError []struct {
			In  string `json:"in"`
			Dim string `json:"dim"`
		} `json:"units_list_error"`
		Format []struct {
			V    any    `json:"v"`
			Dim  string `json:"dim"`
			Unit string `json:"unit"`
			Sig  int    `json:"sig"`
			Want string `json:"want"`
		} `json:"units_format"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Parse) == 0 || len(f.Format) == 0 || len(f.List) == 0 {
		t.Fatal("core.json lacks the units sections")
	}
	u := testUnits(t)

	for _, v := range f.Parse {
		got, err := u.Parse(v.In, v.Dim, v.Unit)
		if err != nil {
			t.Errorf("Parse(%q, %s, %q): %v", v.In, v.Dim, v.Unit, err)
		} else if !near(got, v.Want, 1e-12) {
			t.Errorf("Parse(%q, %s, %q) = %v, want %v", v.In, v.Dim, v.Unit, got, v.Want)
		}
	}
	for _, v := range f.ParseError {
		if got, err := u.Parse(v.In, v.Dim, v.Unit); err == nil {
			t.Errorf("Parse(%q, %s, %q) = %v, want an error", v.In, v.Dim, v.Unit, got)
		}
	}
	for _, v := range f.List {
		got, err := u.ParseList(v.In, v.Dim, "")
		if err != nil || len(got) != len(v.Want) {
			t.Errorf("ParseList(%q) = %v, %v, want %v", v.In, got, err, v.Want)
			continue
		}
		for i := range got {
			if !near(got[i], v.Want[i], 1e-12) {
				t.Errorf("ParseList(%q)[%d] = %v, want %v", v.In, i, got[i], v.Want[i])
			}
		}
	}
	for _, v := range f.ListError {
		if got, err := u.ParseList(v.In, v.Dim, ""); err == nil {
			t.Errorf("ParseList(%q) = %v, want an error", v.In, got)
		}
	}
	for _, v := range f.Format {
		if got := u.Format(vecNum(t, v.V), v.Dim, v.Unit, v.Sig); got != v.Want {
			t.Errorf("Format(%v, %s, %q, %d) = %q, want %q", v.V, v.Dim, v.Unit, v.Sig, got, v.Want)
		}
	}
}

// The Greek mu (U+03BC) and the ohm sign (U+2126) look like the micro sign and the Greek omega; input folds them.
func TestUnitsFoldLookAlikes(t *testing.T) {
	u := testUnits(t)
	greekMu, ohmSign := string(rune(0x3BC)), string(rune(0x2126))
	if got, err := u.Parse("2"+greekMu+"2", "current", ""); err != nil || !near(got, 2.2e-6, 1e-12) {
		t.Errorf("greek mu: %v, %v", got, err)
	}
	if got, err := u.Parse("4.7k"+ohmSign, "resistance", ""); err != nil || !near(got, 4700, 1e-12) {
		t.Errorf("ohm sign: %v, %v", got, err)
	}
}

func TestLoadUnitsRejectsBrokenDefinitions(t *testing.T) {
	cases := []struct{ name, yaml, want string }{
		{"unknown field", "prefixes: []\ndims:\n  a: {symbol: A, colour: red}\n", "colour"},
		{"unknown prefix", "prefixes: [{id: k, factor: 1e3}]\ndims:\n  a: {symbol: A, prefixes: [k, M]}\n", `unknown prefix "M"`},
		{"prefixes descending", "prefixes: [{id: k, factor: 1e3}, {id: m, factor: 1e-3}]\ndims: {}\n", "ascending"},
		{"unit without conversion", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u}]}\n", "factor"},
		{"conversion with a foreign variable", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, to: \"y*2\", from: \"x/2\"}]}\n", "only x"},
		{"conversion that does not parse", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, to: \"x*\", from: \"x/2\"}]}\n", "unit \"u\""},
		{"duplicate dim", "prefixes: []\ndims:\n  a: {symbol: A}\n  a: {symbol: B}\n", "duplicate key"},
		{"empty alias", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, factor: 1, aliases: [\"\"]}]}\n", "must not be empty"},
		{"alias used by two units", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, factor: 1, aliases: [x]}, {id: v, symbol: v, factor: 2, aliases: [x]}]}\n", `"x" is used by`},
		{"alias equal to the base symbol", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, factor: 1, aliases: [A]}]}\n", `"A" is used by`},
		{"alias equal to another unit symbol", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, factor: 1}, {id: v, symbol: v, factor: 2, aliases: [u]}]}\n", `"u" is used by`},
		{"alias equal to a dimension alias", "prefixes: []\ndims:\n  a: {symbol: A, aliases: [z], units: [{id: u, symbol: u, factor: 1, aliases: [z]}]}\n", `"z" is used by`},
	}
	for _, c := range cases {
		_, err := loadUnits([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

// The dimensions of units.yaml in their order, with the base symbol, the prefixes, the positive flag and the ids of the
// explicit units. The unit dropdowns of the app follow this order, and the vectors rely on these definitions.
func TestUnitsCatalogDimensions(t *testing.T) {
	u := testUnits(t)
	want := []string{
		"voltage|V|n µ m '' k M|false|dBV dBu dBuV",
		"current|A|p n µ m '' k|false|",
		"resistance|Ω|m '' k M G|true|",
		"power|W|n µ m '' k M|false|dBm",
		"charge|C|n µ m ''|false|mAh Ah",
		"energy|J|n µ m '' k M|false|mWh Wh kWh",
		"number||(none)|true|",
		"gain|dB|(none)|false|",
		"capacitance|F|p n µ m ''|true|",
		"inductance|H|n µ m ''|true|",
		"frequency|Hz|'' k M G|true|",
		"time|s|p n µ m ''|true|min h",
		"length|m|µ m '' k|true|mil in ft oz AWG",
		"area|m²|''|true|mm2 mil2 kcmil AWG",
		"temperature|K|''|true|degC degF",
		"tempdiff|K|''|false|degC degF",
		"thermal_resistance|K/W|''|true|degCW",
		"resistivity|Ω·m|n µ m ''|true|ohmmm2m",
		"tempco|1/K|''|false|ppmK pctK",
		"ratio||''|false|pct",
		"angle|rad|m ''|false|deg",
		"baudrate|Bd|'' k M|true|",
	}
	var got []string
	for _, d := range u.Dims {
		prefixes := make([]string, len(d.Prefixes))
		for i, p := range d.Prefixes {
			prefixes[i] = p
			if p == "" {
				prefixes[i] = "''"
			}
		}
		if len(prefixes) == 0 {
			prefixes = []string{"(none)"}
		}
		ids := make([]string, len(d.Units))
		for i, un := range d.Units {
			ids[i] = un.ID
		}
		got = append(got, fmt.Sprintf("%s|%s|%s|%t|%s", d.ID, d.Symbol, strings.Join(prefixes, " "), d.Positive, strings.Join(ids, " ")))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("dimensions of units.yaml\n got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The units whose conversion is a formula or a published constant: AWG (ASTM B258; the published table is rounded to
// four digits, hence the loose tolerance on those rows), the degree in radians and the Fahrenheit degree of a
// temperature difference.
func TestUnitsReferenceConstants(t *testing.T) {
	u := testUnits(t)
	awgLen, awgArea := u.Dim("length").Unit("AWG"), u.Dim("area").Unit("AWG")
	for _, c := range []struct {
		name      string
		got, want float64
		tol       float64
	}{
		{"AWG 0 diameter, 8.251 mm", awgLen.ToBase(0), 8.251e-3, 5e-4},
		{"AWG 10 diameter, 2.588 mm", awgLen.ToBase(10), 2.588e-3, 5e-4},
		{"AWG 36 diameter, 0.127 mm", awgLen.ToBase(36), 0.127e-3, 1e-12},
		{"AWG 10 area, 5.261 mm2", awgArea.ToBase(10), 5.261e-6, 5e-4},
		{"AWG number of 0.127 mm", awgLen.FromBase(0.127e-3), 36, 1e-12},
		{"AWG number of 5.261154954510375 mm2", awgArea.FromBase(5.261154954510375e-6), 10, 1e-12},
		{"9 degF of temperature difference in K", u.Dim("tempdiff").Unit("degF").ToBase(9), 5, 1e-12},
		{"180 degrees in radians", u.Dim("angle").Unit("deg").ToBase(180), math.Pi, 1e-12},
	} {
		if !near(c.got, c.want, c.tol) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// The R infix ("4R7") reads on every dimension that has the alias R, not on one dimension by name; the JavaScript
// mirror makes the same decision (scripts/test-ecalc-core.mjs runs the same inputs).
func TestUnitsRInfixFollowsTheAlias(t *testing.T) {
	u, err := loadUnits([]byte("prefixes: [{id: k, factor: 1e3}]\ndims:\n  withr: {symbol: X, aliases: [R], prefixes: [k]}\n  without: {symbol: X, aliases: [Q], prefixes: [k]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]float64{"4R7": 4.7, "47R": 47, "4k7": 4700} {
		if got, err := u.Parse(in, "withr", ""); err != nil || !near(got, want, 1e-12) {
			t.Errorf("Parse(%q, withr) = %v, %v, want %v", in, got, err, want)
		}
	}
	if got, err := u.Parse("4R7", "without", ""); err == nil {
		t.Errorf("Parse(4R7, without) = %v, want an error (no alias R)", got)
	}
	if got, err := u.ParseList("4R7 1e400", "withr", ""); err == nil {
		t.Errorf("ParseList with an infinite item = %v, want an error", got)
	}
}
