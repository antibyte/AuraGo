package ecalc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// mixCalc has a choice (constant f), a list variable and derived quantities that reduce the list.
var mixCalc = strings.ReplaceAll(`- id: t_mix
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    x: {dim: number, label: X, name: {en: "X", de: "X"}, default: 3}
    rs: {dim: resistance, label: R, name: {en: "R", de: "R"}, list: true, default: ["100", "300"]}
    y: {dim: number, label: Y, name: {en: "Y", de: "Y"}}
  choices:
    m: {name: NAME16, default: dbl, options: {dbl: {name: NAME16, consts: {f: 2}}, tri: {name: NAME16, consts: {f: 3}}}}
  dof: 2
  rules:
    y: ["x*f"]
    x: ["y/f"]
  derived:
    rsum: {dim: resistance, label: R_S, name: {en: "Sum", de: "Summe"}, expr: "sum(rs)"}
    rpar: {dim: resistance, label: R_P, name: {en: "Par", de: "Par"}, expr: "par(rs)"}
  formula: ["y = x * f"]
`, "NAME16", flow16("T"))

func loadOne(t *testing.T, calcYAML string) *Calculator {
	t.Helper()
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": calcYAML}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cat.Calculators) != 1 {
		t.Fatalf("got %d calculators", len(cat.Calculators))
	}
	return cat.Calculators[0]
}

func TestSolvePropagatesAndFlagsProblems(t *testing.T) {
	c := loadOne(t, baseCalc)

	res := c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 100}})
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("errors %v warnings %v, want none", res.Errors, res.Warnings)
	}
	if !near(res.Values["c"], 0.1, 1e-12) || !near(res.Values["p"], 1, 1e-12) {
		t.Errorf("values = %v, want c=0.1 p=1", res.Values)
	}
	if !reflect.DeepEqual(res.Computed, []string{"c", "p"}) {
		t.Errorf("computed = %v, want [c p]", res.Computed)
	}

	// A check fires when its condition holds: 10 V / 5 ohm = 2 A.
	if res := c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 5}}); !reflect.DeepEqual(res.Warnings, []int{0}) {
		t.Errorf("warnings = %v, want [0]", res.Warnings)
	}

	// Under-determined: nothing can be computed from one value.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10}})
	want := map[string]string{"b": ErrUndetermined, "c": ErrUndetermined}
	if !reflect.DeepEqual(res.Errors, want) || !math.IsNaN(res.Values["b"]) || !math.IsNaN(res.Values["p"]) {
		t.Errorf("under-determined: errors %v values %v", res.Errors, res.Values)
	}

	// A zero resistance is out of range and makes the current infinite; an infinite derived value is NaN, no error.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 0}})
	want = map[string]string{"b": ErrRange, "c": ErrNonFinite}
	if !reflect.DeepEqual(res.Errors, want) || !math.IsNaN(res.Values["p"]) || len(res.Warnings) != 0 {
		t.Errorf("zero resistance: errors %v values %v warnings %v", res.Errors, res.Values, res.Warnings)
	}

	// Inputs that are not variables (a derived quantity, an unknown name) are ignored.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 100, "zz": 5, "p": 7}})
	if _, ok := res.Values["zz"]; ok || !near(res.Values["p"], 1, 1e-12) {
		t.Errorf("values = %v, want p=1 computed and no zz", res.Values)
	}
}

func TestSolveChoicesAndLists(t *testing.T) {
	c := loadOne(t, mixCalc)
	in := Inputs{
		Values: map[string]float64{"x": 3},
		Lists:  map[string][]float64{"rs": {100, 300}},
	}

	res := c.Solve(in) // default option dbl: f = 2
	if !near(res.Values["y"], 6, 1e-12) || !near(res.Values["rsum"], 400, 1e-12) || !near(res.Values["rpar"], 75, 1e-12) {
		t.Errorf("dbl: values = %v, want y=6 rsum=400 rpar=75", res.Values)
	}
	in.Choices = map[string]string{"m": "tri"}
	if res := c.Solve(in); !near(res.Values["y"], 9, 1e-12) {
		t.Errorf("tri: y = %v, want 9", res.Values["y"])
	}
	in.Choices = map[string]string{"m": "nonsense"}
	if res := c.Solve(in); !near(res.Values["y"], 6, 1e-12) {
		t.Errorf("unknown option: y = %v, want the default option (6)", res.Values["y"])
	}

	// A zero element of a positive list is out of range; the parallel value is then 0, not an error.
	in.Lists = map[string][]float64{"rs": {100, 0}}
	res = c.Solve(in)
	if !reflect.DeepEqual(res.Errors, map[string]string{"rs": ErrRange}) || !near(res.Values["rpar"], 0, 1e-12) {
		t.Errorf("zero element: errors %v values %v", res.Errors, res.Values)
	}

	// Without the list the variable is undetermined and the derived values stay NaN.
	res = c.Solve(Inputs{Values: map[string]float64{"x": 3}})
	if !reflect.DeepEqual(res.Errors, map[string]string{"rs": ErrUndetermined}) || !math.IsNaN(res.Values["rsum"]) {
		t.Errorf("no list: errors %v values %v", res.Errors, res.Values)
	}
}

func TestSolvableSets(t *testing.T) {
	if got := loadOne(t, baseCalc).Sets; !reflect.DeepEqual(got, [][]string{{"a", "b"}, {"a", "c"}, {"b", "c"}}) {
		t.Errorf("t_div sets = %v, want all three pairs", got)
	}
	// {x, rs} and {rs, y} work; {x, y} cannot know the list.
	if got := loadOne(t, mixCalc).Sets; !reflect.DeepEqual(got, [][]string{{"x", "rs"}, {"rs", "y"}}) {
		t.Errorf("t_mix sets = %v, want [x rs] and [rs y]", got)
	}
}

func TestLoadRejectsUnsolvableCalculators(t *testing.T) {
	// d is an input without any rule, and c can never be an input: neither default nor input sets work out.
	broken := strings.Replace(baseCalc, "    c: {dim: current, label: C, name: {en: \"C\", de: \"C\"}}\n",
		"    c: {dim: current, label: C, name: {en: \"C\", de: \"C\"}}\n    d: {dim: current, label: D, name: {en: \"D\", de: \"D\"}}\n", 1)
	_, err := Load(fixtureFS(t, map[string]string{"t.yaml": broken}))
	if err == nil {
		t.Fatal("a calculator with an input variable nobody can compute must be rejected")
	}
	for _, want := range []string{"do not determine every variable", "var c is an input but belongs to no solvable input set", "var d is an input but belongs to no solvable input set"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
