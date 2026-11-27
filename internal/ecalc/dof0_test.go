package ecalc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// levelCalc has no numeric input: two choices carry the logic levels as constants, the noise margins are computed.
var levelCalc = strings.ReplaceAll(`- id: t_levels
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    nmh: {dim: voltage, label: NM_H, name: {en: "High margin", de: "Hoher Rand"}, input: false}
    nml: {dim: voltage, label: NM_L, name: {en: "Low margin", de: "Niedriger Rand"}, input: false}
  choices:
    drv:
      name: NAME16
      default: a
      options:
        a: {name: NAME16, consts: {voh: 4.9, vol: 0.1}}
        b: {name: NAME16, consts: {voh: 2.4, vol: 0.4}}
    rcv:
      name: NAME16
      default: a
      options:
        a: {name: NAME16, consts: {vih: 2.0, vil: 0.8}}
        b: {name: NAME16, consts: {vih: 3.5, vil: 1.5}}
  dof: 0
  rules:
    nmh: ["voh - vih"]
    nml: ["vil - vol"]
  derived:
    voh_shown: {dim: voltage, label: V_OH, name: {en: "VOH", de: "VOH"}, expr: "voh"}
    vil_shown: {dim: voltage, label: V_IL, name: {en: "VIL", de: "VIL"}, expr: "vil"}
  checks:
    - {when: "nmh < 0", warn: {en: "No high margin.", de: "Kein hoher Rand."}}
  formula: ["nmh = voh - vih", "nml = vil - vol"]
`, "NAME16", flow16("T"))

func TestLoadAcceptsCalculatorOfOnlyChoices(t *testing.T) {
	c := loadOne(t, levelCalc)
	if c.DOF != 0 || !reflect.DeepEqual(c.Sets, [][]string{{}}) {
		t.Errorf("dof %d sets %v, want 0 and one empty set", c.DOF, c.Sets)
	}
}

func TestLoadRefusesBadDofZero(t *testing.T) {
	withInput := strings.Replace(levelCalc, "  choices:\n",
		"    x: {dim: voltage, label: X, name: {en: \"X\", de: \"X\"}, default: \"1 V\"}\n  choices:\n", 1)
	if _, err := Load(fixtureFS(t, map[string]string{"t.yaml": withInput})); err == nil || !strings.Contains(err.Error(), "dof 0 is only for calculators without input variables") {
		t.Errorf("dof 0 with an input: error %v", err)
	}
	// A calculator without choices (and without inputs) has nothing to choose.
	noChoices := levelCalc[:strings.Index(levelCalc, "  choices:")] + levelCalc[strings.Index(levelCalc, "  dof: 0"):]
	if _, err := Load(fixtureFS(t, map[string]string{"t.yaml": noChoices})); err == nil || !strings.Contains(err.Error(), "dof 0 needs at least one choice") {
		t.Errorf("dof 0 without choices: error %v", err)
	}
	// Every other calculator keeps the old message.
	dof0 := strings.Replace(baseCalc, "dof: 2", "dof: 0", 1)
	if _, err := Load(fixtureFS(t, map[string]string{"t.yaml": dof0})); err == nil {
		t.Error("dof 0 on a normal calculator must be refused")
	}
}

func TestSolveFromChoicesOnly(t *testing.T) {
	c := loadOne(t, levelCalc)
	for _, tc := range []struct {
		drv, rcv string
		nmh, nml float64
		voh, vil float64
		warn     []int
	}{
		{"a", "a", 2.9, 0.7, 4.9, 0.8, []int{}},
		{"b", "b", -1.1, 1.1, 2.4, 1.5, []int{0}},
		{"a", "b", 1.4, 1.4, 4.9, 1.5, []int{}},
		{"b", "a", 0.4, 0.4, 2.4, 0.8, []int{}},
	} {
		res := c.Solve(Inputs{Choices: map[string]string{"drv": tc.drv, "rcv": tc.rcv}})
		if len(res.Errors) != 0 {
			t.Errorf("%s/%s: errors %v", tc.drv, tc.rcv, res.Errors)
		}
		for name, want := range map[string]float64{"nmh": tc.nmh, "nml": tc.nml, "voh_shown": tc.voh, "vil_shown": tc.vil} {
			if got := res.Values[name]; math.Abs(got-want) > 1e-9 {
				t.Errorf("%s/%s: %s = %v, want %v", tc.drv, tc.rcv, name, got, want)
			}
		}
		if !reflect.DeepEqual(res.Warnings, tc.warn) {
			t.Errorf("%s/%s: warnings %v, want %v", tc.drv, tc.rcv, res.Warnings, tc.warn)
		}
	}
}

func TestCalculateWithoutValues(t *testing.T) {
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": levelCalc}))
	if err != nil {
		t.Fatal(err)
	}
	res, err := cat.Calculate("t_levels", CalcRequest{Choices: []NamedValue{{Name: "drv", Value: "b"}, {Name: "rcv", Value: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Basis) != 0 || len(res.Warnings) != 1 || res.Warnings[0] != "No high margin." {
		t.Errorf("basis %v warnings %v", res.Basis, res.Warnings)
	}
}

// A dimension without prefixes writes whole numbers with all their digits.
func TestCalculateBitsRangeKeepsAllDigits(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("bits_range", CalcRequest{Values: []NamedValue{{Name: "n", Value: "16"}}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"vmax": "65535", "smin": "-32768", "smax": "32767", "count": "65536"}
	for _, q := range res.Quantities {
		if w, ok := want[q.ID]; ok && q.Formatted != w {
			t.Errorf("%s formatted %q, want %q", q.ID, q.Formatted, w)
		}
	}
}
