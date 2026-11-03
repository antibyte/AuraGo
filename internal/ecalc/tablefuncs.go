package ecalc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// The table functions of the expression language read the tables of the catalog:
//
//	lookup(T, C, key)       the value of column C in the row whose key column equals key
//	lookup_ge(T, C, x, OUT) the value of column OUT in the first row whose column C is at least x
//	interp(T, X, Y, x)      the linear interpolation of column Y over column X
//
// T, C, X, Y and OUT are names, not values: a table id, a column id of that table, or an alias that a choice option
// defines in its refs (every option of the choice defines the same aliases, so the choice selects the table or the
// column). They are no variables: Deps leaves them out, and the loader resolves every call for every combination of
// the options of the choices whose aliases it uses.

// tableAlias is how the loader names a choice ref in the table of names an expression may use: an alias is no value.
const tableAlias = "choice alias"

// tableFunc describes a table function: the positions of the arguments that name a table or a column (the table first)
// and the position of the one argument that is an ordinary expression.
type tableFunc struct {
	refs  []int
	value int
}

var tableFuncs = map[string]tableFunc{
	"lookup":    {refs: []int{0, 1}, value: 2},
	"lookup_ge": {refs: []int{0, 1, 3}, value: 2},
	"interp":    {refs: []int{0, 1, 2}, value: 3},
}

func isTableFunc(name string) bool {
	_, ok := tableFuncs[name]
	return ok
}

// valueArgs returns the arguments of a call that are expressions: all of them, except for a table function, whose
// names of tables and columns are left out.
func valueArgs(n *Node) []*Node {
	if n.T == "call" {
		if tf, ok := tableFuncs[n.Name]; ok {
			return []*Node{n.Args[tf.value]}
		}
	}
	return n.Args
}

// walkCalls calls f for every call node of the tree.
func walkCalls(n *Node, f func(*Node)) {
	if n == nil {
		return
	}
	if n.T == "call" {
		f(n)
	}
	walkCalls(n.A, f)
	walkCalls(n.L, f)
	walkCalls(n.R, f)
	for _, a := range n.Args {
		walkCalls(a, f)
	}
	for _, a := range n.Parts {
		walkCalls(a, f)
	}
}

// hasTableCall reports whether the tree calls a table function.
func hasTableCall(n *Node) bool {
	found := false
	walkCalls(n, func(call *Node) { found = found || isTableFunc(call.Name) })
	return found
}

// resolveTableCall finds the table and the columns (in the order of the ref positions after the table) that a call names
// when the aliases in refs apply. The tables must exist, the columns must exist and be numeric, and lookup needs a
// numeric key column.
func resolveTableCall(call *Node, refs map[string]string, tables map[string]*Table) (*Table, []int, error) {
	if tables == nil {
		return nil, nil, errNoTable
	}
	pos := tableFuncs[call.Name].refs
	name := func(i int) string {
		n := call.Args[i].Name
		if r, ok := refs[n]; ok {
			return r
		}
		return n
	}
	t := tables[name(pos[0])]
	if t == nil {
		return nil, nil, fmt.Errorf("unknown table %q", name(pos[0]))
	}
	cols := make([]int, 0, len(pos)-1)
	for _, p := range pos[1:] {
		id := name(p)
		switch ci := t.ColumnIndex(id); {
		case ci < 0:
			return nil, nil, fmt.Errorf("table %q has no column %q", t.ID, id)
		case !t.Columns[ci].Numeric():
			return nil, nil, fmt.Errorf("column %q of table %q is text, %s needs numeric columns", id, t.ID, call.Name)
		default:
			cols = append(cols, ci)
		}
	}
	if call.Name == "lookup" && !t.Columns[t.keyIdx].Numeric() {
		return nil, nil, fmt.Errorf("table %q has the text key column %q, lookup needs a numeric key", t.ID, t.Columns[t.keyIdx].ID)
	}
	return t, cols, nil
}

var errNoTable = errors.New("no tables are available to the table functions")

// evalTableCall evaluates lookup, lookup_ge and interp.
func evalTableCall(n *Node, env Env) (float64, error) {
	t, cols, err := resolveTableCall(n, env.Refs, env.Tables)
	if err != nil {
		return 0, err
	}
	x, err := Eval(n.Args[tableFuncs[n.Name].value], env)
	if err != nil {
		return 0, err
	}
	switch n.Name {
	case "lookup":
		return t.lookup(cols[0], x), nil
	case "lookup_ge":
		return t.lookupGE(cols[0], x, cols[1]), nil
	}
	return t.interp(cols[0], cols[1], x), nil
}

// ---- use of tables by a calculator -------------------------------------------------------------------------------

// choice returns the choice with the given id, or nil.
func (c *Calculator) choice(id string) *Choice {
	for i := range c.Choices {
		if c.Choices[i].ID == id {
			return &c.Choices[i]
		}
	}
	return nil
}

// selectedRefs returns the aliases of the selected option of every choice (a missing or unknown selection is the
// default of the choice).
func (c *Calculator) selectedRefs(choices map[string]string) map[string]string {
	refs := map[string]string{}
	for i := range c.Choices {
		ch := &c.Choices[i]
		for k, v := range ch.option(choices[ch.ID]).Refs {
			refs[k] = v
		}
	}
	return refs
}

// EvalEnv returns the environment in which the expressions of the calculator are evaluated for the given selection of
// choices: the constants of the selected options as values, their table aliases and the tables of the catalog. The
// caller adds the variables.
func (c *Calculator) EvalEnv(choices map[string]string) Env {
	env := Env{Vals: map[string]float64{}, Lists: map[string][]float64{}, Refs: c.selectedRefs(choices), Tables: c.tables}
	for i := range c.Choices {
		ch := &c.Choices[i]
		for k, v := range ch.option(choices[ch.ID]).Consts {
			env.Vals[k] = v
		}
	}
	return env
}

// tableCall is one call of a table function in an expression of a calculator: where it sits, the expression text and
// the call node.
type tableCall struct {
	what string
	src  string
	node *Node
}

// tableCalls lists the table-function calls of every expression of the calculator.
func (c *Calculator) tableCalls() []tableCall {
	var out []tableCall
	add := func(what, src string, n *Node) {
		walkCalls(n, func(call *Node) {
			if isTableFunc(call.Name) {
				out = append(out, tableCall{what: what, src: src, node: call})
			}
		})
	}
	for _, r := range c.Rules {
		for _, a := range r.Alts {
			add("rule "+r.Target, a.Src, a.Node)
		}
	}
	for _, d := range c.Derived {
		for _, a := range d.Alts {
			add("derived "+d.ID, a.Src, a.Node)
		}
	}
	for i, ch := range c.Checks {
		add(fmt.Sprintf("check %d", i+1), ch.When.Src, ch.When.Node)
	}
	if c.Chart != nil {
		add("chart to", c.Chart.To.Src, c.Chart.To.Node)
	}
	for i, n := range c.FormulaAST {
		add("formula", c.Formula[i], n)
	}
	return out
}

// involved lists the indexes of the choices whose aliases a call names, ascending.
func (c *Calculator) involved(call *Node) []int {
	seen := map[int]bool{}
	for _, p := range tableFuncs[call.Name].refs {
		if ci, ok := c.aliasChoice[call.Args[p].Name]; ok {
			seen[ci] = true
		}
	}
	out := make([]int, 0, len(seen))
	for ci := range seen {
		out = append(out, ci)
	}
	sort.Ints(out)
	return out
}

// eachSelection calls f with every combination of the options of the given choices (indexes), as choice id -> option id.
// With no choice f is called once with an empty selection.
func (c *Calculator) eachSelection(choices []int, f func(sel map[string]string)) {
	idx := make([]int, len(choices))
	for {
		sel := map[string]string{}
		for k, ci := range choices {
			sel[c.Choices[ci].ID] = c.Choices[ci].Options[idx[k]].ID
		}
		f(sel)
		k := len(idx) - 1
		for ; k >= 0; k-- {
			if idx[k]++; idx[k] < len(c.Choices[choices[k]].Options) {
				break
			}
			idx[k] = 0
		}
		if k < 0 {
			return
		}
	}
}

// referencesTable reports whether some table-function call names the table when the choices in fixed (choice id ->
// option id) are selected, for at least one selection of the other choices the call depends on.
func (c *Calculator) referencesTable(tableID string, fixed map[string]string) bool {
	for _, call := range c.tableCalls() {
		found := false
		c.eachSelection(c.involved(call.node), func(sel map[string]string) {
			merged := map[string]string{}
			for k, v := range sel {
				merged[k] = v
			}
			for k, v := range fixed {
				if o, ok := merged[k]; ok && o != v {
					return
				}
				merged[k] = v
			}
			refs := c.selectedRefs(merged)
			name := call.node.Args[tableFuncs[call.node.Name].refs[0]].Name
			if r, ok := refs[name]; ok {
				name = r
			}
			found = found || name == tableID
		})
		if found {
			return true
		}
	}
	return false
}

// checkTableUse gives every calculator the tables of the catalog and resolves every table-function call for every
// combination of the options of the choices whose aliases it uses: the table and the columns must exist, be numeric,
// and interp needs a strictly monotonic x column. An alias that no call uses is an error.
func (l *loader) checkTableUse(cat *Catalog) {
	for _, c := range cat.Calculators {
		c.tables = cat.tableByID
		file := l.ids[c.ID]
		seen := map[string]bool{}
		fail := func(format string, args ...any) {
			msg := fmt.Sprintf("%s: %s: %s", file, c.ID, fmt.Sprintf(format, args...))
			if !seen[msg] {
				seen[msg] = true
				l.errs = append(l.errs, errors.New(msg))
			}
		}
		usedAlias := map[string]bool{}
		for _, call := range c.tableCalls() {
			for _, p := range tableFuncs[call.node.Name].refs {
				usedAlias[call.node.Args[p].Name] = true
			}
			c.eachSelection(c.involved(call.node), func(sel map[string]string) {
				t, cols, err := resolveTableCall(call.node, c.selectedRefs(sel), cat.tableByID)
				if err == nil && call.node.Name == "interp" {
					err = t.checkInterp(cols[0], cols[1])
				}
				if err != nil {
					fail("%s: %v in %q%s", call.what, err, call.src, c.selectionText(sel))
				}
			})
		}
		aliases := make([]string, 0, len(c.aliasChoice))
		for a := range c.aliasChoice {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		for _, a := range aliases {
			if !usedAlias[a] {
				fail("table alias %q is never used by a table function", a)
			}
		}
	}
}

// checkFinders validates the finders of the tables once all calculators are known: the calculator and its variable
// exist and have the dimension of the column, the fixed choices are real options, and the calculator's table functions
// name this table when those options are selected.
func (l *loader) checkFinders(cat *Catalog) {
	for _, t := range cat.Tables {
		f := t.Finder
		if f == nil {
			continue
		}
		fail := func(format string, args ...any) {
			l.errs = append(l.errs, fmt.Errorf("%s: %s: finder: %s", l.ids[t.ID], t.ID, fmt.Sprintf(format, args...)))
		}
		c := cat.byID[f.Calc]
		if c == nil {
			fail("%q is not a calculator", f.Calc)
			continue
		}
		dim := ""
		if v := c.Var(f.Var); v != nil {
			dim = v.Dim
		}
		for _, d := range c.Derived {
			if d.ID == f.Var {
				dim = d.Dim
			}
		}
		switch col := &t.Columns[t.ColumnIndex(f.Column)]; {
		case dim == "":
			fail("%q is neither a variable nor a derived quantity of %s", f.Var, c.ID)
		case dim != col.Dim:
			fail("%s has the dimension %s, column %s has %s", f.Var, dim, col.ID, col.Dim)
		}
		keys := make([]string, 0, len(f.Choices))
		for k := range f.Choices {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ch := c.choice(k)
			switch {
			case ch == nil:
				fail("%q is not a choice of %s", k, c.ID)
			case !hasOption(*ch, f.Choices[k]):
				fail("%q is not an option of choice %s", f.Choices[k], k)
			}
		}
		if !c.referencesTable(t.ID, f.Choices) {
			fail("the table functions of %s do not name table %s when the choices are %s", c.ID, t.ID, choiceList(keys, f.Choices))
		}
	}
}

func choiceList(keys []string, choices map[string]string) string {
	if len(keys) == 0 {
		return "unset"
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + choices[k]
	}
	return strings.Join(parts, ", ")
}

// selectionText describes a selection of options for an error message.
func (c *Calculator) selectionText(sel map[string]string) string {
	var parts []string
	for _, ch := range c.Choices {
		if o, ok := sel[ch.ID]; ok {
			parts = append(parts, ch.ID+"="+o)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (with " + strings.Join(parts, ", ") + ")"
}
