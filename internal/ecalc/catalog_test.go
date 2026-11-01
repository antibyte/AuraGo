package ecalc

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// flow16 writes {cs: "v", da: "v", ...} for all 16 locales.
func flow16(v string) string {
	parts := make([]string, len(Locales))
	for i, l := range Locales {
		parts[i] = l + `: "` + v + `"`
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// baseCalc is a small valid calculator: current = voltage / resistance with two defaults.
var baseCalc = strings.ReplaceAll(`- id: t_div
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    a: {dim: voltage, label: A, name: {en: "A", de: "A"}, default: "10 V"}
    b: {dim: resistance, label: B, name: {en: "B", de: "B"}, default: "100"}
    c: {dim: current, label: C, name: {en: "C", de: "C"}}
  dof: 2
  rules:
    c: ["a/b"]
    a: ["c*b"]
    b: ["a/c"]
  derived:
    p: {dim: power, label: P, name: {en: "P", de: "P"}, expr: "a*c"}
  checks:
    - {when: "c > 1", warn: {en: "Too much.", de: "Zu viel."}}
  formula: ["c = a / b"]
`, "NAME16", flow16("T"))

func fixtureFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	units, err := os.ReadFile("catalog/units.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cats, err := os.ReadFile("catalog/categories.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"units.yaml": {Data: units}, "categories.yaml": {Data: cats}}
	for name, data := range files {
		fsys["calculators/"+name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestDefaultCatalogLoads(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	var ids []string
	for _, c := range cat.Categories {
		ids = append(ids, c.ID)
		if c.Name["en"] == "" || c.Name["de"] == "" || len(c.Name) != 16 {
			t.Errorf("category %s: names %v", c.ID, c.Name)
		}
	}
	want := "basics passive filters semiconductors power opamps timers rf digital wiring tables codes pinouts"
	if strings.Join(ids, " ") != want {
		t.Errorf("categories = %v, want %s", ids, want)
	}
	for _, dim := range []string{"voltage", "current", "resistance", "power", "charge", "energy", "number", "gain"} {
		if cat.Units.Dim(dim) == nil {
			t.Errorf("units.yaml lacks the dimension %s", dim)
		}
	}
	if again, _ := Default(); again != cat {
		t.Error("Default must return the same catalog every time")
	}
}

func TestLoadFixtureBuildsTheCalculator(t *testing.T) {
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": baseCalc}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cat.Calculator("t_div")
	if c == nil || c.DOF != 2 || len(c.Vars) != 3 || len(c.Rules) != 3 || len(c.Derived) != 1 || len(c.Checks) != 1 {
		t.Fatalf("unexpected calculator %+v", c)
	}
	if c.Vars[0].ID != "a" || c.Vars[2].ID != "c" || c.Rules[0].Target != "c" {
		t.Errorf("declaration order was not kept: %v / %v", c.Vars, c.Rules)
	}
	if a := c.Var("a"); a.Default == nil || *a.Default != 10 || !a.Input {
		t.Errorf("var a = %+v, want default 10 and input", a)
	}
	if b := c.Var("b"); b.Default == nil || *b.Default != 100 {
		t.Errorf("var b = %+v, want default 100", b)
	}
	if c.Var("c").Default != nil {
		t.Error("var c has no default")
	}
	if c.Checks[0].When.Src != "c > 1" || c.FormulaAST[0].Sexp() != "(= c (/ a b))" {
		t.Errorf("check/formula not parsed: %+v %s", c.Checks[0], c.FormulaAST[0].Sexp())
	}
}

func TestLoadRejectsBrokenCalculators(t *testing.T) {
	cases := []struct{ name, old, new, want string }{
		{"missing locale", `, zh: "T"}`, `}`, `missing "zh" text`},
		{"unknown language key", `de: "Doku."`, `de: "Doku.", xx: "Y"`, `unknown language key "xx"`},
		{"missing german doc", `doc: {en: "Doc.", de: "Doku."}`, `doc: {en: "Doc."}`, `doc: missing "de" text`},
		{"missing source", `source: "Textbook"`, `source: ""`, "source is required"},
		{"unknown category", `category: basics`, `category: nowhere`, `unknown category "nowhere"`},
		{"unknown kind", `kind: calculator`, `kind: table`, "kind must be calculator"},
		{"unknown field", `label: A,`, `label: A, colour: red,`, "colour"},
		{"unknown dimension", `dim: current`, `dim: speed`, `unknown dimension "speed"`},
		{"unknown display unit", `default: "10 V"}`, `default: "10 V", unit: dBm}`, `unit "dBm" is not an explicit unit`},
		{"unparsable default", `default: "10 V"`, `default: "ten"`, "default"},
		{"zero default of a positive dimension", `default: "100"`, `default: "0"`, "greater than zero"},
		{"wrong number of defaults", `, default: "100"}`, `}`, "exactly 2 input variables need a default, found 1"},
		{"rule that does not parse", `c: ["a/b"]`, `c: ["a/"]`, "rule c"},
		{"rule with an unknown name", `c: ["a/b"]`, `c: ["a/zz"]`, `unknown name "zz"`},
		{"rule using its own target", `a: ["c*b"]`, `a: ["c*a"]`, "uses its own target"},
		{"rule target that is not a variable", `b: ["a/c"]`, `q: ["a/c"]`, `rule target "q" is not a variable`},
		{"variable named like a function", `    c: {dim: current,`, `    min: {dim: current,`, `var "min" is reserved`},
		{"list variable outside sum", `default: "100"}`, `default: ["100", "200"], list: true}`, "used outside sum() or par()"},
		{"check with an unknown name", `when: "c > 1"`, `when: "zz > 1"`, `unknown name "zz"`},
		{"formula that does not parse", `formula: ["c = a / b"]`, `formula: ["c = a / "]`, "formula"},
		{"choice const colliding with a variable", `  dof: 2`,
			"  choices:\n    q: {name: " + flow16("Q") + ", default: o, options: {o: {name: " + flow16("O") + ", consts: {a: 1}}, p: {name: " + flow16("P") + ", consts: {a: 2}}}}\n  dof: 2",
			`choice const "a" collides with the var`},
	}
	for _, c := range cases {
		if !strings.Contains(baseCalc, c.old) {
			t.Fatalf("%s: fixture does not contain %q", c.name, c.old)
		}
		broken := strings.Replace(baseCalc, c.old, c.new, 1)
		_, err := Load(fixtureFS(t, map[string]string{"t.yaml": broken}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

func TestLoadRejectsDuplicateIDsAndEmptyFiles(t *testing.T) {
	_, err := Load(fixtureFS(t, map[string]string{"a.yaml": baseCalc, "b.yaml": baseCalc}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in calculators/a.yaml") {
		t.Errorf("duplicate id: error = %v", err)
	}
	_, err = Load(fixtureFS(t, map[string]string{"a.yaml": ""}))
	if err == nil || !strings.Contains(err.Error(), "the file is empty") {
		t.Errorf("empty file: error = %v", err)
	}
}
