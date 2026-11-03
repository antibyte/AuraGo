package ecalc

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// tableFixture is the "table_fixture" section of testdata/vectors/core.json: a small catalog tree with four tables and
// the calculator ec_fix, which reads them through the aliases of its choice options. Files are YAML (a list of lines per
// file); Items is what the catalog JSON carries for them, which the JavaScript test registers.
type tableFixture struct {
	Files map[string][]string `json:"files"`
	Items []any               `json:"items"`
}

type tableFunctionVector struct {
	Expr string             `json:"expr"`
	Env  map[string]float64 `json:"env"`
	Refs map[string]string  `json:"refs"`
	Want any                `json:"want"`
	Ref  string             `json:"ref"`
}

type tableSolveVector struct {
	Inputs  map[string]float64 `json:"inputs"`
	Choices map[string]string  `json:"choices"`
	Expect  map[string]any     `json:"expect"`
	Errors  map[string]string  `json:"errors"`
	Ref     string             `json:"ref"`
}

type tableVectorFile struct {
	Fixture   tableFixture          `json:"table_fixture"`
	Functions []tableFunctionVector `json:"table_functions"`
	Errors    []tableFunctionVector `json:"table_function_errors"`
	Solve     []tableSolveVector    `json:"table_solve"`
}

func readTableVectors(t *testing.T) tableVectorFile {
	t.Helper()
	var f tableVectorFile
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Fixture.Files) == 0 || len(f.Functions) == 0 || len(f.Errors) == 0 || len(f.Solve) == 0 {
		t.Fatal("core.json lacks the table_fixture, table_functions, table_function_errors or table_solve sections")
	}
	return f
}

// fixtureFiles returns the files of the fixture with NAME16 replaced by a name in all 16 locales.
func (f tableFixture) files() map[string]string {
	out := map[string]string{}
	for name, lines := range f.Files {
		out[name] = strings.ReplaceAll(strings.Join(lines, "\n")+"\n", "NAME16", flow16("T"))
	}
	return out
}

func loadFixture(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(treeFS(t, readTableVectors(t).Fixture.files()))
	if err != nil {
		t.Fatalf("the fixture does not load: %v", err)
	}
	return cat
}

// The items in core.json are exactly what the Go side makes of the fixture files: the JavaScript test cannot drift away
// from the YAML that Go loads.
func TestTableFixtureItemsAreWhatGoMakesOfTheFiles(t *testing.T) {
	f := readTableVectors(t)
	body, _, err := loadFixture(t).LocalizedJSON("en")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc.Items, f.Fixture.Items) {
		got, _ := json.MarshalIndent(doc.Items, "", "  ")
		t.Errorf("core.json table_fixture.items differ from the catalog JSON of the fixture files; they are:\n%s", got)
	}
}

func TestTableFunctionVectors(t *testing.T) {
	f := readTableVectors(t)
	cat := loadFixture(t)
	for _, v := range f.Functions {
		if v.Ref == "" {
			t.Errorf("%s has no ref", v.Expr)
		}
		n, err := ParseExpr(v.Expr)
		if err != nil {
			t.Errorf("%s: parse: %v", v.Expr, err)
			continue
		}
		got, err := Eval(n, Env{Vals: v.Env, Refs: v.Refs, Tables: cat.tableByID})
		if err != nil {
			t.Errorf("%s: eval: %v", v.Expr, err)
			continue
		}
		if want := vecNum(t, v.Want); !near(got, want, 1e-12) {
			t.Errorf("%s = %v, want %v (%s)", v.Expr, got, want, v.Ref)
		}
	}
	for _, v := range f.Errors {
		n, err := ParseExpr(v.Expr)
		if err != nil {
			t.Errorf("%s: parse: %v", v.Expr, err)
			continue
		}
		if got, err := Eval(n, Env{Vals: v.Env, Refs: v.Refs, Tables: cat.tableByID}); err == nil {
			t.Errorf("%s evaluated to %v, want an error", v.Expr, got)
		}
	}

	// Without any tables in the environment a table function is an error, not a panic.
	n, _ := ParseExpr("lookup(t_fix, y, 2)")
	if _, err := Eval(n, Env{}); err == nil {
		t.Error("lookup without tables in the environment must fail")
	}
}

func TestTableSolveVectors(t *testing.T) {
	f := readTableVectors(t)
	c := loadFixture(t).Calculator("ec_fix")
	if c == nil {
		t.Fatal("the fixture has no ec_fix")
	}
	for _, v := range f.Solve {
		res := c.Solve(Inputs{Values: v.Inputs, Choices: v.Choices})
		for name, want := range v.Expect {
			if got := res.Values[name]; !near(got, vecNum(t, want), 1e-12) {
				t.Errorf("%s: %s = %v, want %v", v.Ref, name, got, want)
			}
		}
		wantErrors := v.Errors
		if wantErrors == nil {
			wantErrors = map[string]string{}
		}
		if !reflect.DeepEqual(res.Errors, wantErrors) {
			t.Errorf("%s: errors %v, want %v", v.Ref, res.Errors, wantErrors)
		}
	}
}

// EvalEnv gives the constants and the table aliases of the selected options, and the tables.
func TestEvalEnvCarriesAliasesAndTables(t *testing.T) {
	c := loadFixture(t).Calculator("ec_fix")
	env := c.EvalEnv(nil)
	if !reflect.DeepEqual(env.Refs, map[string]string{"tbl": "t_fix", "col": "y"}) || len(env.Tables) != 4 || env.Tables["t_alt"] == nil {
		t.Errorf("default env: refs %v, %d tables", env.Refs, len(env.Tables))
	}
	env = c.EvalEnv(map[string]string{"src": "b", "col": "z", "nope": "x"})
	if !reflect.DeepEqual(env.Refs, map[string]string{"tbl": "t_alt", "col": "z"}) {
		t.Errorf("env with src b and col z: refs %v", env.Refs)
	}
	if env = c.EvalEnv(map[string]string{"src": "unknown"}); env.Refs["tbl"] != "t_fix" {
		t.Errorf("an unknown option falls back to the default: %v", env.Refs)
	}
}

func TestTableFixtureJSONCarriesRefsAndFinder(t *testing.T) {
	body, _, err := loadFixture(t).LocalizedJSON("en")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 5 || doc.Items[0]["id"] != "ec_fix" || doc.Items[1]["id"] != "t_fix" {
		t.Fatalf("items: %d, first %v", len(doc.Items), doc.Items[0]["id"])
	}
	opts := doc.Items[0]["choices"].([]any)[0].(map[string]any)["options"].([]any)
	if !reflect.DeepEqual(opts[1].(map[string]any)["refs"], map[string]any{"tbl": "t_alt"}) {
		t.Errorf("option refs = %v", opts[1])
	}
	if !reflect.DeepEqual(doc.Items[1]["finder"], map[string]any{"calc": "ec_fix", "column": "x", "var": "q", "choices": map[string]any{"src": "a"}}) {
		t.Errorf("finder = %v", doc.Items[1]["finder"])
	}
	for _, it := range doc.Items[1:] {
		checkOtherItemJSON(t, it)
	}
}

// Every problem with the use of a table is found when the catalog loads.
func TestLoadRejectsBrokenTableUse(t *testing.T) {
	f := readTableVectors(t).Fixture
	files := f.files()
	const calc, tables = "calculators/fixture.yaml", "tables/fixture.yaml"
	cases := []struct {
		name, file, old, new, want string
	}{
		{"unknown table", calc, "refs: {tbl: t_fix}", "refs: {tbl: t_nope}", `unknown table "t_nope" in "lookup(tbl, col, q)" (with src=a, col=y)`},
		{"unknown column", calc, "refs: {col: y}", "refs: {col: nope}", `table "t_fix" has no column "nope"`},
		{"text column", calc, "refs: {col: y}", "refs: {col: n}", `column "n" of table "t_fix" is text, lookup needs numeric columns`},
		{"a text key for lookup", calc, "r: ['lookup(tbl, col, q)']", "r: ['lookup(t_txt, v, q)']", `table "t_txt" has the text key column "n", lookup needs a numeric key`},
		{"x column that is not monotonic", tables, "[20, 2.5, 15, 'q']", "[5, 2.5, 15, 'q']", `column "x" of table "t_alt" is not strictly monotonic, interp cannot use it as x`},
		{"interp with fewer than two rows", tables, "[10, 1.5, 5, 'p']\n    - [20, 2.5, 15, 'q']", "[10, 1.5, null, 'p']\n    - [20, 2.5, null, 'q']", `interp needs at least two rows where "x" and "z" are both filled in table "t_alt"`},
		{"options that define different aliases", calc, "refs: {col: z}", "refs: {colz: z}", "choice col: option z defines different refs than y"},
		{"alias that is also a variable", calc, "refs: {tbl: t_fix}", "refs: {q: t_fix}", `choice alias "q" collides with the var of the same name`},
		{"alias that is no table or column id", calc, "refs: {tbl: t_fix}", "refs: {tbl: T-Fix}", "must be a table or column id"},
		{"alias used as a value", calc, "expr: 'lookup_ge(tbl, z, q, x)'", "expr: 'tbl + q'", `"tbl" is a table alias, it can only name a table or a column`},
		{"table function in the chart", calc, "  dof: 1\n", "  chart: {x: q, y: r, to: 'lookup(t_fix, y, 2)'}\n  dof: 1\n", "chart: to"},
		{"arity", calc, "expr: 'interp(tbl, x, z, q)'", "expr: 'interp(tbl, x, z)'", `wrong number of arguments (3) for function "interp"`},
		{"a value where a name belongs", calc, "expr: 'interp(tbl, x, z, q)'", "expr: 'interp(tbl, 1, z, q)'", `argument 2 of "interp" must be a table or column name`},
		{"finder with an unknown calculator", tables, "calc: ec_fix, column: x, var: q, choices: {src: a}", "calc: nope, column: x, var: q, choices: {src: a}", `finder: "nope" is not a calculator`},
		{"finder with an unknown variable", tables, "var: q, choices: {src: a}", "var: nope, choices: {src: a}", `finder: "nope" is neither a variable nor a derived quantity of ec_fix`},
		{"finder variable of another dimension", tables, "{id: x, name: {en: X, de: X}, dim: ratio}", "{id: x, name: {en: X, de: X}, dim: current}", "finder: q has the dimension ratio, column x has current"},
		{"finder with an unknown choice", tables, "choices: {src: a}", "choices: {nope: a}", `finder: "nope" is not a choice of ec_fix`},
		{"finder with an unknown option", tables, "choices: {src: a}", "choices: {src: zz}", `finder: "zz" is not an option of choice src`},
		{"finder the calculator does not use", tables, "choices: {src: a}", "choices: {src: b}", "finder: the table functions of ec_fix do not name table t_fix when the choices are src=b"},
	}
	for _, c := range cases {
		broken := map[string]string{}
		for k, v := range files {
			broken[k] = v
		}
		if !strings.Contains(broken[c.file], c.old) {
			t.Fatalf("%s: the fixture does not contain %q", c.name, c.old)
		}
		broken[c.file] = strings.Replace(broken[c.file], c.old, c.new, 1)
		_, err := Load(treeFS(t, broken))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}

	// An alias that no table function uses.
	unused := map[string]string{}
	for k, v := range files {
		unused[k] = v
	}
	unused[calc] = strings.NewReplacer("lookup(tbl, col, q)", "lookup(tbl, y, q)").Replace(unused[calc])
	if _, err := Load(treeFS(t, unused)); err == nil || !strings.Contains(err.Error(), `table alias "col" is never used by a table function`) {
		t.Errorf("unused alias: error = %v", err)
	}

	// A finder without fixed choices needs only some selection that names the table.
	open := map[string]string{}
	for k, v := range files {
		open[k] = v
	}
	open[tables] = strings.Replace(open[tables], "choices: {src: a}", "choices: {}", 1)
	cat, err := Load(treeFS(t, open))
	if err != nil {
		t.Fatalf("a finder without choices: %v", err)
	}
	if f := cat.Table("t_fix").Finder; f == nil || f.Calc != "ec_fix" || f.Column != "x" || f.Var != "q" || f.Choices == nil || len(f.Choices) != 0 {
		t.Errorf("finder = %+v, want empty (non-nil) choices", f)
	}
}

// A table function is a function name: it cannot be a variable or a choice constant, and Deps does not count the names
// of tables and columns.
func TestTableFunctionNamesAreReserved(t *testing.T) {
	for _, name := range []string{"lookup", "lookup_ge", "interp"} {
		src := strings.Replace(baseCalc, "    c: {dim: current,", "    "+name+": {dim: current,", 1)
		if _, err := Load(fixtureFS(t, map[string]string{"t.yaml": src})); err == nil || !strings.Contains(err.Error(), `var "`+name+`" is reserved`) {
			t.Errorf("a variable called %s: error = %v", name, err)
		}
	}
	n, err := ParseExpr("lookup(sections, ampacity, a + b) + interp(t, x, y, c) * lookup_ge(t, x, d, out)")
	if err != nil {
		t.Fatal(err)
	}
	if got := Deps(n); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("Deps = %v, want [a b c d]", got)
	}
}

// Table functions have no meaning in a unit conversion, which sees no tables.
func TestUnitConversionsRefuseTableFunctions(t *testing.T) {
	units := `prefixes: []
dims:
  length: {symbol: m, prefixes: [""], units: [{id: tab, symbol: tab, to: "lookup(t, c, x)", from: "x"}]}
`
	if _, err := loadUnits([]byte(units)); err == nil || !strings.Contains(err.Error(), "table functions are not allowed in a unit conversion") {
		t.Errorf("error = %v", err)
	}
}

// Tables are looked up by the loader's own checks and the NaN rules of the functions never produce an error.
func TestTableFunctionResultsAreNaNNotErrors(t *testing.T) {
	cat := loadFixture(t)
	env := Env{Vals: map[string]float64{}, Tables: cat.tableByID}
	for _, src := range []string{"lookup(t_fix, y, 3)", "lookup_ge(t_fix, y, 99, x)", "interp(t_fix, x, z, 100)"} {
		n, _ := ParseExpr(src)
		got, err := Eval(n, env)
		if err != nil || !math.IsNaN(got) {
			t.Errorf("%s = %v, %v, want NaN and no error", src, got, err)
		}
	}
}
