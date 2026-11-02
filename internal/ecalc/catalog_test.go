package ecalc

import (
	"encoding/json"
	"os"
	"reflect"
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

// mutate applies old/new pairs to a fixture (each old text once) and fails the test when a pair does not apply.
func mutate(t *testing.T, base string, pairs ...string) string {
	t.Helper()
	for i := 0; i+1 < len(pairs); i += 2 {
		if !strings.Contains(base, pairs[i]) {
			t.Fatalf("fixture does not contain %q", pairs[i])
		}
		base = strings.Replace(base, pairs[i], pairs[i+1], 1)
	}
	return base
}

// choiceYAML is a choice q whose option o ignores the variables of ignoreO (a YAML flow list) and whose option p ignores
// none; both define the constant k. It ends with the dof line of baseCalc, so it replaces "  dof: 2".
func choiceYAML(ignoreO, rest string) string {
	return "  choices:\n    q: {name: " + flow16("Q") + ", default: o, options: {o: {name: " + flow16("O") + ", consts: {k: 1}" + ignoreO +
		"}, p: {name: " + flow16("P") + ", consts: {k: 2}}}}\n" + rest + "  dof: 2"
}

// combineCalc is the smallest standard-value combiner: one scalar resistance, dof 1, no rules, no formula.
var combineCalc = strings.ReplaceAll(`- id: t_combine
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    target: {dim: resistance, label: R, name: {en: "R", de: "R"}, default: "5k"}
  dof: 1
  finder: combine
`, "NAME16", flow16("T"))

// The schema extensions of phase 2: the display-unit whitelist of a variable or derived quantity (units), the variables
// an option does not use (ignore), the chart and the finder.
func TestLoadAcceptsDisplayUnitsIgnoreChartAndFinder(t *testing.T) {
	src := mutate(t, baseCalc,
		`default: "10 V"}`, `default: "10 V", unit: dBV, units: ["", dBV, dBu]}`,
		`expr: "a*c"}`, `expr: "a*c", unit: dBm, units: [dBm, ""]}`,
		"  dof: 2", choiceYAML(", ignore: [b]", `  chart: {x: a, y: c, to: "10", samples: 21}`+"\n"))
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": src}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cat.Calculator("t_div")
	if got := c.Var("a").Units; !reflect.DeepEqual(got, []string{"", "dBV", "dBu"}) {
		t.Errorf("var a units = %q", got)
	}
	if got := c.Derived[0].Units; !reflect.DeepEqual(got, []string{"dBm", ""}) {
		t.Errorf("derived p units = %q", got)
	}
	if o := c.Choices[0].Options; !reflect.DeepEqual(o[0].Ignore, []string{"b"}) || len(o[1].Ignore) != 0 {
		t.Errorf("ignore lists = %q / %q, want [b] and none", o[0].Ignore, o[1].Ignore)
	}
	if ch := c.Chart; ch == nil || ch.X != "a" || ch.Y != "c" || ch.To.Src != "10" || ch.Samples != 21 {
		t.Errorf("chart = %+v, want x a, y c, to 10, 21 samples", ch)
	}

	// The catalog JSON carries all of it; a missing list is [] and no chart is null (never a missing key).
	body, _, err := cat.LocalizedJSON("en")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []struct {
			Finder string `json:"finder"`
			Chart  *struct {
				X       string `json:"x"`
				Y       string `json:"y"`
				To      string `json:"to"`
				Samples int    `json:"samples"`
			} `json:"chart"`
			Vars    []struct{ Units []string } `json:"vars"`
			Derived []struct{ Units []string } `json:"derived"`
			Choices []struct {
				Options []struct{ Ignore []string } `json:"options"`
			} `json:"choices"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	it := doc.Items[0]
	if it.Chart == nil || it.Chart.X != "a" || it.Chart.Y != "c" || it.Chart.To != "10" || it.Chart.Samples != 21 || it.Finder != "" {
		t.Errorf("item JSON chart/finder = %+v / %q", it.Chart, it.Finder)
	}
	if !reflect.DeepEqual(it.Vars[0].Units, []string{"", "dBV", "dBu"}) || it.Vars[1].Units == nil || len(it.Vars[1].Units) != 0 ||
		!reflect.DeepEqual(it.Derived[0].Units, []string{"dBm", ""}) {
		t.Errorf("JSON units = %q / %q / %q", it.Vars[0].Units, it.Vars[1].Units, it.Derived[0].Units)
	}
	if o := it.Choices[0].Options; !reflect.DeepEqual(o[0].Ignore, []string{"b"}) || o[1].Ignore == nil || len(o[1].Ignore) != 0 {
		t.Errorf("JSON ignore = %q / %q", o[0].Ignore, o[1].Ignore)
	}
	if !strings.Contains(string(body), `"chart":{"x":"a","y":"c","to":"10","samples":21}`) {
		t.Errorf("the chart is not written as {x, y, to, samples}: %s", body)
	}

	// Samples default to 101.
	dflt := mutate(t, src, `, samples: 21`, ``)
	if c := loadOne(t, dflt); c.Chart == nil || c.Chart.Samples != 101 {
		t.Errorf("default samples = %+v, want 101", c.Chart)
	}

	// The combiner finder: no formula, no rules.
	combiner, err := Load(fixtureFS(t, map[string]string{"t.yaml": combineCalc}))
	if err != nil {
		t.Fatalf("Load combiner: %v", err)
	}
	cc := combiner.Calculator("t_combine")
	if cc.Finder != "combine" || len(cc.Formula) != 0 || len(cc.Rules) != 0 || cc.Chart != nil {
		t.Errorf("combine calculator = finder %q formula %q rules %d chart %v", cc.Finder, cc.Formula, len(cc.Rules), cc.Chart)
	}
	cb, _, _ := combiner.LocalizedJSON("en")
	if !strings.Contains(string(cb), `"finder":"combine"`) || !strings.Contains(string(cb), `"chart":null`) || !strings.Contains(string(cb), `"formula":[]`) {
		t.Errorf("combine JSON lacks finder, chart null or an empty formula: %s", cb)
	}
}

func TestLoadRejectsBrokenDisplayUnitsIgnoreChartAndFinder(t *testing.T) {
	withChart := func(chart string) string {
		return mutate(t, baseCalc, "  dof: 2", "  chart: "+chart+"\n  dof: 2")
	}
	extraDerived := mutate(t, baseCalc, "    p: {dim: power,", "    q: {dim: resistance, label: Q, name: {en: \"Q\", de: \"Q\"}, expr: \"b*2\"}\n    p: {dim: power,")
	cases := []struct{ name, src, want string }{
		{"units entry that is no explicit unit", mutate(t, baseCalc, `default: "10 V"}`, `default: "10 V", units: [dBm]}`), `units entry "dBm" is not an explicit unit of voltage`},
		{"units entry listed twice", mutate(t, baseCalc, `default: "10 V"}`, `default: "10 V", units: [dBV, dBV]}`), `units entry "dBV" is listed twice`},
		{"unit that is not in the units list", mutate(t, baseCalc, `default: "10 V"}`, `default: "10 V", unit: dBV, units: [dBu]}`), `unit "dBV" is not in its units list`},
		{"derived units entry that is no explicit unit", mutate(t, baseCalc, `expr: "a*c"}`, `expr: "a*c", units: [Ah]}`), `units entry "Ah" is not an explicit unit of power`},
		{"ignore of an unknown variable", mutate(t, baseCalc, "  dof: 2", choiceYAML(", ignore: [zz]", "")), `ignore "zz" is not a variable`},
		{"ignore listed twice", mutate(t, baseCalc, "  dof: 2", choiceYAML(", ignore: [b, b]", "")), `ignore "b" is listed twice`},
		{"unknown finder", mutate(t, baseCalc, "  dof: 2", "  finder: bogus\n  dof: 2"), `finder "bogus" must be empty or combine`},
		{"combine finder with dof 2", mutate(t, baseCalc, "  dof: 2", "  finder: combine\n  dof: 2"), "finder combine needs dof 1"},
		{"combine finder without a resistance variable", mutate(t, baseCalc, "  dof: 2", "  finder: combine\n  dof: 2", "b: {dim: resistance", "b: {dim: current"), "exactly one scalar resistance variable, found 0"},
		{"combine finder with a list resistance", mutate(t, combineCalc, `target: {dim: resistance, label: R, name: {en: "R", de: "R"}, default: "5k"}`, `target: {dim: resistance, label: R, name: {en: "R", de: "R"}, list: true, default: ["5k"]}`), "exactly one scalar resistance variable, found 0"},
		{"no formula without the combine finder", mutate(t, combineCalc, "  finder: combine\n", ""), "formula is required"},
		{"unknown chart field", withChart(`{x: a, y: c, to: "10", color: red}`), "color"},
		{"chart x that is not a variable", withChart(`{x: zz, y: c, to: "10"}`), `chart: x "zz" is not a variable`},
		{"chart y without a rule", withChart(`{x: a, y: zz, to: "10"}`), `chart: y "zz" needs a rule or a derived expression`},
		{"chart y that does not use x", mutate(t, extraDerived, "  dof: 2", "  chart: {x: a, y: q, to: \"10\"}\n  dof: 2"), `chart: y "q" does not depend on x "a"`},
		{"chart to that does not parse", withChart(`{x: a, y: c, to: "1+"}`), "chart: to:"},
		{"chart to with an unknown name", withChart(`{x: a, y: c, to: "zz"}`), `unknown name "zz"`},
		{"chart to that uses x", withChart(`{x: a, y: c, to: "a*2"}`), `must not depend on x "a"`},
		{"chart with too few samples", withChart(`{x: a, y: c, to: "10", samples: 10}`), "samples must be between 11 and 401"},
		{"chart with too many samples", withChart(`{x: a, y: c, to: "10", samples: 402}`), "samples must be between 11 and 401"},
		{"chart over a list variable", mutate(t, mixCalc, "  dof: 2", "  chart: {x: rs, y: y, to: \"10\"}\n  dof: 2"), `chart: x "rs" is a list variable`},
	}
	for _, c := range cases {
		_, err := Load(fixtureFS(t, map[string]string{"t.yaml": c.src}))
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
