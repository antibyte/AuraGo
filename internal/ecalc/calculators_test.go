package ecalc

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const solveTol = 1e-9

// calcVectors is the format of testdata/vectors/calculators/<id>.json.
//
// A set is a hand-checked consistent solution: a value for every variable and derived quantity (a JSON array for a
// list variable), optionally for one option of each choice. Every rule alternative and every derived alternative
// must reproduce its target from the set's other values, and every solvable input set must reproduce the whole set
// through Solve. A set is therefore one vector for each rule direction; every option of every choice needs two.
//
// A solve vector is a single Solve call: inputs (SI), the expected values (a subset), the exact error map and the
// exact list of warning (check) indexes.
//
// combos (only for a calculator with finder: combine, and required there) are Combine cases in the shape of the combine
// section of core.json: the entries Combine returns for the series, target and limit, in this order, with a ref.
type calcVectors struct {
	Calc   string          `json:"calc"`
	Combos []combineVector `json:"combos"`
	Sets   []struct {
		Choices map[string]string `json:"choices"`
		Values  map[string]any    `json:"values"`
		Ref     string            `json:"ref"`
	} `json:"sets"`
	Solve []struct {
		Inputs   map[string]any    `json:"inputs"`
		Choices  map[string]string `json:"choices"`
		Expect   map[string]any    `json:"expect"`
		Errors   map[string]string `json:"errors"`
		Warnings []int             `json:"warnings"`
		Ref      string            `json:"ref"`
	} `json:"solve"`
}

// split separates a vector's values into scalars and lists.
func split(t *testing.T, in map[string]any) (map[string]float64, map[string][]float64) {
	t.Helper()
	vals, lists := map[string]float64{}, map[string][]float64{}
	for k, v := range in {
		if arr, ok := v.([]any); ok {
			for _, x := range arr {
				lists[k] = append(lists[k], vecNum(t, x))
			}
			continue
		}
		vals[k] = vecNum(t, v)
	}
	return vals, lists
}

func loadVectors(t *testing.T, id string) calcVectors {
	t.Helper()
	var v calcVectors
	readJSON(t, "testdata/vectors/calculators/"+id+".json", &v)
	if v.Calc != id {
		t.Fatalf("%s.json declares calc %q", id, v.Calc)
	}
	return v
}

func TestCalculatorVectors(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.Calculators {
		t.Run(c.ID, func(t *testing.T) {
			v := loadVectors(t, c.ID)
			if len(v.Solve) == 0 {
				t.Error("no solve vectors")
			}

			// Every option of every choice has at least two sets (two in total when there is no choice).
			if len(c.Choices) == 0 && len(v.Sets) < 2 {
				t.Errorf("%d sets, every rule direction needs at least 2 vectors", len(v.Sets))
			}
			for i := range c.Choices {
				ch := &c.Choices[i]
				for _, o := range ch.Options {
					n := 0
					for _, s := range v.Sets {
						if ch.option(s.Choices[ch.ID]).ID == o.ID {
							n++
						}
					}
					if n < 2 {
						t.Errorf("choice %s option %s has %d sets, every rule direction needs at least 2 vectors", ch.ID, o.ID, n)
					}
				}
			}

			for i, s := range v.Sets {
				if s.Ref == "" {
					t.Errorf("set %d has no ref", i)
				}
				vals, lists := split(t, s.Values)
				for _, vr := range c.Vars {
					_, isScalar := vals[vr.ID]
					_, isList := lists[vr.ID]
					if (vr.List && !isList) || (!vr.List && !isScalar) {
						t.Errorf("set %d lacks the %s variable %s", i, map[bool]string{true: "list", false: "scalar"}[vr.List], vr.ID)
					}
				}
				for _, d := range c.Derived {
					if _, ok := vals[d.ID]; !ok {
						t.Errorf("set %d lacks the derived quantity %s", i, d.ID)
					}
				}

				// (a) every alternative of every rule and derived quantity reproduces its target.
				env := c.EvalEnv(s.Choices) // the constants and table aliases of the options, and the tables
				env.Lists = lists
				for k, x := range vals {
					env.Vals[k] = x
				}
				for _, tg := range c.targets() {
					for j, alt := range tg.alts {
						got, err := Eval(alt.Node, env)
						if err != nil || !near(got, vals[tg.name], solveTol) {
							t.Errorf("set %d (%s): %s alternative %d (%s) = %v (%v), want %v", i, s.Ref, tg.name, j, alt.Src, got, err, vals[tg.name])
						}
					}
				}

				// (b) every solvable input set reproduces the whole solution.
				for _, set := range c.Sets {
					in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: s.Choices}
					for _, name := range set {
						if c.Var(name).List {
							in.Lists[name] = lists[name]
						} else {
							in.Values[name] = vals[name]
						}
					}
					res := c.Solve(in)
					if len(res.Errors) != 0 {
						t.Errorf("set %d from %v: errors %v", i, set, res.Errors)
					}
					for name, want := range vals {
						if got := res.Values[name]; !near(got, want, solveTol) {
							t.Errorf("set %d from %v: %s = %v, want %v", i, set, name, got, want)
						}
					}
				}
			}

			for i, sv := range v.Solve {
				if sv.Ref == "" {
					t.Errorf("solve vector %d has no ref", i)
				}
				vals, lists := split(t, sv.Inputs)
				res := c.Solve(Inputs{Values: vals, Lists: lists, Choices: sv.Choices})
				for name, want := range sv.Expect {
					if got := res.Values[name]; !near(got, vecNum(t, want), solveTol) {
						t.Errorf("solve %d (%s): %s = %v, want %v", i, sv.Ref, name, got, want)
					}
				}
				wantErrors := sv.Errors
				if wantErrors == nil {
					wantErrors = map[string]string{}
				}
				if !reflect.DeepEqual(res.Errors, wantErrors) {
					t.Errorf("solve %d (%s): errors %v, want %v", i, sv.Ref, res.Errors, wantErrors)
				}
				wantWarnings := sv.Warnings
				if wantWarnings == nil {
					wantWarnings = []int{}
				}
				if !reflect.DeepEqual(res.Warnings, wantWarnings) {
					t.Errorf("solve %d (%s): warnings %v, want %v", i, sv.Ref, res.Warnings, wantWarnings)
				}
			}

			// Combos belong to the combiner finder and are required there.
			if (c.Finder == "combine") != (len(v.Combos) > 0) {
				t.Errorf("finder %q with %d combos: a calculator with finder combine needs combos, any other has none", c.Finder, len(v.Combos))
			}
			for _, combo := range v.Combos {
				checkCombine(t, combo)
			}
		})
	}
}

// TestCalculatorVectorFilesNameACalculator makes the vector directory strict: every file in it is called <calc>.json and
// names a calculator of the catalog. TestCalculatorVectors only reads the files of calculators that exist, so without
// this check a vector file for a missing or misspelled calculator would be ignored and writing the vectors first could
// never go red. (The other direction, a calculator without a file, already fails TestCalculatorVectors.)
func TestCalculatorVectorFilesNameACalculator(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.FromSlash("testdata/vectors/calculators/*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no calculator vector files found (%v)", err)
	}
	for _, file := range files {
		name := filepath.Base(file)
		if !strings.HasSuffix(name, ".json") {
			t.Errorf("%s: only <calc>.json files belong in testdata/vectors/calculators", name)
			continue
		}
		var v struct {
			Calc string `json:"calc"`
		}
		readJSON(t, "testdata/vectors/calculators/"+name, &v)
		switch {
		case v.Calc == "":
			t.Errorf("%s declares no calc", name)
		case name != v.Calc+".json":
			t.Errorf("%s declares calc %q, so the file must be called %s.json", name, v.Calc, v.Calc)
		case cat.Calculator(v.Calc) == nil:
			t.Errorf("%s names the calculator %q, which the catalog does not have", name, v.Calc)
		}
	}
}

// defaultInputs returns the initial inputs of a calculator: the variables that carry a default.
func defaultInputs(c *Calculator, choices map[string]string) Inputs {
	in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: choices}
	for _, v := range c.Vars {
		switch {
		case v.List && v.DefaultList != nil:
			in.Lists[v.ID] = v.DefaultList
		case v.Default != nil:
			in.Values[v.ID] = *v.Default
		}
	}
	return in
}

// TestRoundTripAllCalculators solves the defaults, then solves again from every solvable input set taken out of
// that solution: everything must come back. Every option of every choice is tried (the others at their default).
func TestRoundTripAllCalculators(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Calculators) == 0 {
		t.Fatal("the catalog has no calculators")
	}
	for _, c := range cat.Calculators {
		selections := []map[string]string{{}}
		for i := range c.Choices {
			for _, o := range c.Choices[i].Options {
				selections = append(selections, map[string]string{c.Choices[i].ID: o.ID})
			}
		}
		for _, choices := range selections {
			name := fmt.Sprintf("%s %v", c.ID, choices)
			defaults := defaultInputs(c, choices)
			base := c.Solve(defaults)
			if len(base.Errors) != 0 {
				t.Errorf("%s: the defaults give errors %v", name, base.Errors)
				continue
			}
			for _, set := range c.Sets {
				in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: choices}
				for _, id := range set {
					if c.Var(id).List {
						in.Lists[id] = defaults.Lists[id]
					} else {
						in.Values[id] = base.Values[id]
					}
				}
				res := c.Solve(in)
				if len(res.Errors) != 0 {
					t.Errorf("%s from %v: errors %v", name, set, res.Errors)
				}
				for k, want := range base.Values {
					if got := res.Values[k]; !near(got, want, solveTol) {
						t.Errorf("%s from %v: %s = %v, want %v", name, set, k, got, want)
					}
				}
			}
		}
	}
}

// TestWireResistanceSourceStatesTheSilverConstant keeps the written source of the wire resistance honest about the
// resistivity that the silver option really uses: 1.59e-8 Ohm*m is 15.9 nOhm*m (the text once said 1.59).
func TestWireResistanceSourceStatesTheSilverConstant(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	c := cat.Calculator("wire_resistance")
	if c == nil {
		t.Fatal("the catalog has no wire_resistance")
	}
	var rho float64
	for _, ch := range c.Choices {
		for _, o := range ch.Options {
			if o.ID == "silver" {
				rho = o.Consts["rho"]
			}
		}
	}
	if rho == 0 {
		t.Fatal("wire_resistance has no silver option with a rho constant")
	}
	want := "silver " + strconv.FormatFloat(rho*1e9, 'g', 3, 64) + " nΩ·m"
	if !strings.Contains(c.Source, want) {
		t.Errorf("the source of wire_resistance does not say %q (rho = %v Ω·m): %s", want, rho, c.Source)
	}
}
