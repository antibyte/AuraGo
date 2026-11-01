package ecalc

import (
	"math"
	"strings"
)

// Error codes of Result.Errors.
const (
	ErrUndetermined = "undetermined" // not enough values to compute the variable
	ErrNonFinite    = "nonfinite"    // NaN or infinite
	ErrRange        = "range"        // zero or negative for a dimension that must be positive
)

// Inputs are the known values for Solve, all in SI base units.
type Inputs struct {
	Values  map[string]float64   // scalar variables
	Lists   map[string][]float64 // list variables
	Choices map[string]string    // choice id -> option id (missing or unknown = the choice's default)
}

// Result is what Solve found.
type Result struct {
	Values   map[string]float64 // every variable and derived quantity; NaN when unknown
	Computed []string           // names computed from rules, in the order they were computed
	Errors   map[string]string  // variable -> error code
	Warnings []int              // indexes of the checks that fired
}

type target struct {
	name string
	alts []Expr
}

// targets lists what propagation can compute: the rule targets in declaration order, then the derived quantities.
func (c *Calculator) targets() []target {
	ts := make([]target, 0, len(c.Rules)+len(c.Derived))
	for _, r := range c.Rules {
		ts = append(ts, target{r.Target, r.Alts})
	}
	for _, d := range c.Derived {
		ts = append(ts, target{d.ID, d.Alts})
	}
	return ts
}

// propagate applies rules until nothing changes: every unknown target whose first applicable alternative has all
// its names known becomes known, and apply is called for it.
func (c *Calculator) propagate(known map[string]bool, apply func(t target, alt Expr)) {
	ts := c.targets()
	for changed := true; changed; {
		changed = false
		for _, t := range ts {
			if known[t.name] {
				continue
			}
			for _, alt := range t.alts {
				if allKnown(alt.Deps, known) {
					apply(t, alt)
					known[t.name] = true
					changed = true
					break
				}
			}
		}
	}
}

func allKnown(deps []string, known map[string]bool) bool {
	for _, d := range deps {
		if !known[d] {
			return false
		}
	}
	return true
}

func (ch *Choice) option(id string) *ChoiceOption {
	for _, want := range []string{id, ch.Default} {
		for i := range ch.Options {
			if ch.Options[i].ID == want {
				return &ch.Options[i]
			}
		}
	}
	return &ch.Options[0]
}

func valueError(d *Dim, x float64) string {
	switch {
	case math.IsNaN(x) || math.IsInf(x, 0):
		return ErrNonFinite
	case d.Positive && x <= 0:
		return ErrRange
	}
	return ""
}

// listError is the error code of a list variable. Finiteness is checked for every element before the range, so a list
// with a negative and an infinite element is "nonfinite" whatever their order (the ECMAScript twin does the same).
func listError(d *Dim, list []float64) string {
	for _, x := range list {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ErrNonFinite
		}
	}
	for _, x := range list {
		if code := valueError(d, x); code != "" {
			return code
		}
	}
	return ""
}

// Solve computes every variable that the inputs determine: the constants of the selected choice options and the
// inputs are known; rules and derived quantities are applied as a fixpoint; then errors and checks are evaluated.
func (c *Calculator) Solve(in Inputs) Result {
	env := Env{Vals: map[string]float64{}, Lists: map[string][]float64{}}
	known := map[string]bool{}
	for i := range c.Choices {
		ch := &c.Choices[i]
		for k, v := range ch.option(in.Choices[ch.ID]).Consts {
			env.Vals[k] = v
			known[k] = true
		}
	}
	for _, v := range c.Vars {
		if v.List {
			if list, ok := in.Lists[v.ID]; ok {
				env.Lists[v.ID] = list
				known[v.ID] = true
			}
		} else if x, ok := in.Values[v.ID]; ok {
			env.Vals[v.ID] = x
			known[v.ID] = true
		}
	}

	res := Result{Values: map[string]float64{}, Computed: []string{}, Errors: map[string]string{}, Warnings: []int{}}
	c.propagate(known, func(t target, alt Expr) {
		x, err := Eval(alt.Node, env)
		if err != nil {
			x = math.NaN()
		}
		env.Vals[t.name] = x
		res.Computed = append(res.Computed, t.name)
	})

	for i := range c.Vars {
		v := &c.Vars[i]
		switch {
		case !known[v.ID]:
			res.Errors[v.ID] = ErrUndetermined
			if !v.List {
				res.Values[v.ID] = math.NaN()
			}
		case v.List:
			if code := listError(v.dim, env.Lists[v.ID]); code != "" {
				res.Errors[v.ID] = code
			}
		default:
			x := env.Vals[v.ID]
			res.Values[v.ID] = x
			if code := valueError(v.dim, x); code != "" {
				res.Errors[v.ID] = code
			}
		}
	}
	for _, d := range c.Derived {
		x := math.NaN()
		if known[d.ID] && !math.IsInf(env.Vals[d.ID], 0) {
			x = env.Vals[d.ID]
		}
		res.Values[d.ID] = x
	}
	for i, ch := range c.Checks {
		if !finiteDeps(ch.When.Deps, known, env) {
			continue
		}
		if x, err := Eval(ch.When.Node, env); err == nil && !math.IsNaN(x) && x != 0 {
			res.Warnings = append(res.Warnings, i)
		}
	}
	return res
}

func finiteDeps(deps []string, known map[string]bool, env Env) bool {
	for _, d := range deps {
		if !known[d] {
			return false
		}
		if list, ok := env.Lists[d]; ok {
			for _, x := range list {
				if math.IsNaN(x) || math.IsInf(x, 0) {
					return false
				}
			}
		} else if x := env.Vals[d]; math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

// SolvableSets returns every combination of dof input variables (in declaration order) from which propagation can
// compute all variables. It looks at the structure of the rules only: any alternative whose names are known
// applies, and the constants of every choice count as known.
func (c *Calculator) SolvableSets() [][]string {
	var inputs []string
	for _, v := range c.Vars {
		if v.Input {
			inputs = append(inputs, v.ID)
		}
	}
	consts := map[string]bool{}
	for _, ch := range c.Choices {
		for _, o := range ch.Options {
			for k := range o.Consts {
				consts[k] = true
			}
		}
	}
	var out [][]string
	if c.DOF < 1 || c.DOF > len(inputs) {
		return out
	}
	idx := make([]int, c.DOF)
	var pick func(pos, start int)
	pick = func(pos, start int) {
		if pos == c.DOF {
			set := make([]string, c.DOF)
			known := map[string]bool{}
			for k := range consts {
				known[k] = true
			}
			for i, ix := range idx {
				set[i] = inputs[ix]
				known[set[i]] = true
			}
			c.propagate(known, func(target, Expr) {})
			for _, v := range c.Vars {
				if !known[v.ID] {
					return
				}
			}
			out = append(out, set)
			return
		}
		for i := start; i < len(inputs); i++ {
			idx[pos] = i
			pick(pos+1, i+1)
		}
	}
	pick(0, 0)
	return out
}

// solvableSets stores the solvable input sets on the calculator and checks that the initial inputs are one of them
// and that every input variable belongs to at least one.
func (l *loader) solvableSets(fail func(string, ...any), c *Calculator) {
	c.Sets = c.SolvableSets()
	var defaults []string
	for _, v := range c.Vars {
		if v.Input && (v.Default != nil || v.DefaultList != nil) {
			defaults = append(defaults, v.ID)
		}
	}
	isDefault := false
	inSet := map[string]bool{}
	for _, set := range c.Sets {
		if strings.Join(set, ",") == strings.Join(defaults, ",") {
			isDefault = true
		}
		for _, id := range set {
			inSet[id] = true
		}
	}
	if !isDefault {
		fail("the default inputs (%s) do not determine every variable", strings.Join(defaults, ", "))
	}
	for _, v := range c.Vars {
		if v.Input && !inSet[v.ID] {
			fail("var %s is an input but belongs to no solvable input set", v.ID)
		}
	}
}
