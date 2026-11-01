package ecalc

import (
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
	}
	for _, c := range cases {
		_, err := loadUnits([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
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
