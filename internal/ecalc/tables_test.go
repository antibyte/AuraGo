package ecalc

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// treeFS builds a small catalog in memory: the real units.yaml and categories.yaml plus the given files, whose names are
// relative to the catalog root ("calculators/c.yaml", "tables/t.yaml", "codes/k.yaml").
func treeFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	for _, name := range []string{"units.yaml", "categories.yaml"} {
		data, err := os.ReadFile("catalog/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

// baseTable is a small valid table: a key column in mm2 (shown in mm2 or kcmil), an empty cell, a temperature in degC, and a
// text column with a plain, a localized and an empty string.
var baseTable = strings.ReplaceAll(`- id: t_sec
  kind: table
  category: tables
  group: conductors
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  notes: {en: "Note.", de: "Hinweis."}
  tags: [test]
  source: "Textbook"
  guide: true
  mains: true
  key: section
  columns:
    - {id: section, name: {en: Cross-section, de: Querschnitt}, dim: area, unit: mm2, units: [mm2, kcmil], label: A}
    - {id: amps, name: {en: Current, de: Strom}, dim: current}
    - {id: temp, name: {en: Temperature, de: Temperatur}, dim: temperature, unit: degC}
    - {id: note, name: {en: Note, de: Hinweis}, dim: text}
  rows:
    - [1.5, 15.5, 70, "—"]
    - [2.5, 19.5, 90, {en: "see note", de: "siehe Anmerkung"}]
    - [4, null, 90, ""]
`, "NAME16", flow16("T"))

func loadTable(t *testing.T, src string) *Table {
	t.Helper()
	cat, err := Load(treeFS(t, map[string]string{"tables/t.yaml": src}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cat.Tables) != 1 {
		t.Fatalf("got %d tables", len(cat.Tables))
	}
	return cat.Tables[0]
}

func TestLoadTableConvertsCellsToSI(t *testing.T) {
	cat, err := Load(treeFS(t, map[string]string{"tables/t.yaml": baseTable}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tbl := cat.Table("t_sec")
	if tbl == nil || len(cat.Tables) != 1 || tbl.Group != "conductors" || !tbl.Guide || !tbl.Mains || tbl.Key != "section" || tbl.KeyColumn() != 0 {
		t.Fatalf("unexpected table %+v", tbl)
	}
	if kind, ok := cat.Item("t_sec"); !ok || kind != "table" {
		t.Errorf("Item(t_sec) = %q, %v, want table", kind, ok)
	}
	if _, ok := cat.Item("nope"); ok {
		t.Error("Item(nope) must report false")
	}
	if len(tbl.Columns) != 4 || len(tbl.Rows) != 3 {
		t.Fatalf("%d columns, %d rows", len(tbl.Columns), len(tbl.Rows))
	}
	if c := tbl.Columns[0]; c.ID != "section" || c.Dim != "area" || c.Unit != "mm2" || c.Label != "A" || !reflect.DeepEqual(c.Units, []string{"mm2", "kcmil"}) || !c.Numeric() {
		t.Errorf("column section = %+v", c)
	}
	if c := tbl.Columns[3]; c.Numeric() || c.Dim != "text" {
		t.Errorf("column note = %+v, want text", c)
	}
	if !near(tbl.Rows[0][0].Num, 1.5e-6, 1e-12) || !near(tbl.Rows[2][0].Num, 4e-6, 1e-12) {
		t.Errorf("the mm2 cells are %v and %v, want 1.5e-6 and 4e-6 (m2)", tbl.Rows[0][0].Num, tbl.Rows[2][0].Num)
	}
	if tbl.Rows[0][1].Num != 15.5 || !near(tbl.Rows[0][2].Num, 343.15, 1e-12) || !near(tbl.Rows[1][2].Num, 363.15, 1e-12) {
		t.Errorf("amps %v, degC cells %v and %v, want 15.5, 343.15 K and 363.15 K", tbl.Rows[0][1].Num, tbl.Rows[0][2].Num, tbl.Rows[1][2].Num)
	}
	if c := tbl.Rows[2][1]; !c.Null || !math.IsNaN(c.Num) {
		t.Errorf("the null cell = %+v, want Null with NaN", c)
	}
	if got := tbl.CellText(0, 3, "de"); got != "—" {
		t.Errorf("a plain string reads %q in every language", got)
	}
	if tbl.CellText(1, 3, "de") != "siehe Anmerkung" || tbl.CellText(1, 3, "fr") != "see note" || tbl.CellText(1, 3, "en") != "see note" {
		t.Errorf("a localized cell falls back to English: %q %q", tbl.CellText(1, 3, "de"), tbl.CellText(1, 3, "fr"))
	}
	if c := tbl.Rows[2][3]; c.Null || tbl.CellText(2, 3, "en") != "" {
		t.Errorf("an empty string is no null cell: %+v", c)
	}
	if tbl.CellText(9, 0, "en") != "" || tbl.CellText(0, 0, "en") != "" {
		t.Error("CellText of a missing row or of a numeric cell must be empty")
	}
	if tbl.ColumnIndex("amps") != 1 || tbl.ColumnIndex("nope") != -1 {
		t.Error("ColumnIndex")
	}
	// A column converts between its own unit and SI (a column without a unit is SI already).
	sec, amps, temp := &tbl.Columns[0], &tbl.Columns[1], &tbl.Columns[2]
	if !near(sec.ToSI(1.5), 1.5e-6, 1e-12) || !near(sec.FromSI(2.5e-6), 2.5, 1e-12) || amps.ToSI(7) != 7 || amps.FromSI(7) != 7 || !near(temp.FromSI(363.15), 90, 1e-12) {
		t.Errorf("ToSI/FromSI: %v %v %v %v %v", sec.ToSI(1.5), sec.FromSI(2.5e-6), amps.ToSI(7), amps.FromSI(7), temp.FromSI(363.15))
	}
}

// The key column is the first one unless key names another; a text key is fine.
func TestTableKeyColumn(t *testing.T) {
	first := loadTable(t, strings.Replace(baseTable, "  key: section\n", "", 1))
	if first.Key != "section" || first.KeyColumn() != 0 {
		t.Errorf("default key = %s (%d), want the first column", first.Key, first.KeyColumn())
	}
	other := loadTable(t, strings.Replace(baseTable, "key: section", "key: note", 1))
	if other.Key != "note" || other.KeyColumn() != 3 {
		t.Errorf("key = %s (%d), want note (3)", other.Key, other.KeyColumn())
	}
}

func TestTableJSONShape(t *testing.T) {
	cat, err := Load(treeFS(t, map[string]string{"tables/t.yaml": baseTable}))
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := cat.LocalizedJSON("de")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || len(doc.Items) != 1 {
		t.Fatalf("decode: %v (%d items)", err, len(doc.Items))
	}
	it := doc.Items[0]
	want := []string{"kind", "id", "category", "group", "name", "name_en", "doc", "notes", "tags", "source", "guide", "mains", "key", "columns", "rows", "finder", "generate"}
	if len(it) != len(want) {
		t.Errorf("the table item has %d keys, want %d: %v", len(it), len(want), it)
	}
	for _, key := range want {
		if _, ok := it[key]; !ok {
			t.Errorf("the table item lacks the key %s", key)
		}
	}
	if it["kind"] != "table" || it["id"] != "t_sec" || it["group"] != "conductors" || it["name"] != "T" || it["name_en"] != "T" || it["doc"] != "Doku." || it["notes"] != "Hinweis." ||
		it["guide"] != true || it["mains"] != true || it["key"] != "section" || it["source"] != "Textbook" {
		t.Errorf("table item = %v", it)
	}
	cols := it["columns"].([]any)
	if len(cols) != 4 {
		t.Fatalf("%d columns", len(cols))
	}
	first, third := cols[0].(map[string]any), cols[1].(map[string]any)
	if !reflect.DeepEqual(first, map[string]any{"id": "section", "name": "Querschnitt", "dim": "area", "unit": "mm2", "units": []any{"mm2", "kcmil"}, "label": "A"}) {
		t.Errorf("column section = %v", first)
	}
	if !reflect.DeepEqual(third, map[string]any{"id": "amps", "name": "Strom", "dim": "current", "unit": "", "units": []any{}, "label": ""}) {
		t.Errorf("column amps (units [] and label \"\" are always written) = %v", third)
	}
	rows := it["rows"].([]any)
	r1, r2, r3 := rows[0].([]any), rows[1].([]any), rows[2].([]any)
	if r1[0].(float64) != 1.5e-6 || r1[3] != "—" || r2[3] != "siehe Anmerkung" || r3[1] != nil || r3[3] != "" {
		t.Errorf("rows = %v", rows)
	}
	// generate names the generator of a formula-derived table; a transcribed table has "" (the page shows its cells with
	// the digits they were written with, a generated one with the significant-digit setting).
	if g, ok := it["generate"]; !ok || g != "" {
		t.Errorf("generate = %#v (present %v), want an empty string for a transcribed table", g, ok)
	}
	// No finder is null, not missing (a finder is covered with the table functions); French falls back to English.
	if f, ok := it["finder"]; !ok || f != nil {
		t.Errorf("finder = %v (present %v), want null", f, ok)
	}
	fr, _, _ := cat.LocalizedJSON("fr")
	var frDoc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(fr, &frDoc); err != nil {
		t.Fatal(err)
	}
	if frDoc.Items[0]["doc"] != "Doc." || frDoc.Items[0]["rows"].([]any)[1].([]any)[3] != "see note" {
		t.Errorf("French doc and cell must fall back to English: %v", frDoc.Items[0])
	}
}

// checkOtherItemJSON checks a table or code item of the catalog JSON: the keys are always all there and the collections
// are arrays (the checks of calculators are in TestLocalizedJSONNeverHasNullCollections).
func checkOtherItemJSON(t *testing.T, it map[string]any) {
	t.Helper()
	switch it["kind"] {
	case "table":
		for _, key := range []string{"tags", "columns", "rows"} {
			if _, ok := it[key].([]any); !ok {
				t.Errorf("table %v: %s = %#v, want an array", it["id"], key, it[key])
			}
		}
		for _, key := range []string{"id", "category", "group", "name", "name_en", "doc", "notes", "source", "key", "generate"} {
			if _, ok := it[key].(string); !ok {
				t.Errorf("table %v: %s = %#v, want a string", it["id"], key, it[key])
			}
		}
		for _, key := range []string{"guide", "mains"} {
			if _, ok := it[key].(bool); !ok {
				t.Errorf("table %v: %s = %#v, want a boolean", it["id"], key, it[key])
			}
		}
		finder, hasFinder := it["finder"]
		if _, isObject := finder.(map[string]any); !hasFinder || (finder != nil && !isObject) {
			t.Errorf("table %v: finder = %#v, want null or an object", it["id"], finder)
		}
		for _, c := range it["columns"].([]any) {
			col := c.(map[string]any)
			for _, key := range []string{"id", "name", "dim", "unit", "label"} {
				if _, ok := col[key].(string); !ok {
					t.Errorf("table %v column %v: %s = %#v, want a string", it["id"], col["id"], key, col[key])
				}
			}
			if _, ok := col["units"].([]any); !ok {
				t.Errorf("table %v column %v: units = %#v, want an array", it["id"], col["id"], col["units"])
			}
		}
		for i, r := range it["rows"].([]any) {
			row, ok := r.([]any)
			if !ok || len(row) != len(it["columns"].([]any)) {
				t.Errorf("table %v row %d = %#v, want one cell per column", it["id"], i, r)
			}
		}
	case "code":
		for _, key := range []string{"id", "category", "code", "name", "name_en", "doc", "source"} {
			if _, ok := it[key].(string); !ok {
				t.Errorf("code %v: %s = %#v, want a string", it["id"], key, it[key])
			}
		}
		if _, ok := it["tags"].([]any); !ok {
			t.Errorf("code %v: tags = %#v, want an array", it["id"], it["tags"])
		}
		if _, ok := it["data"]; !ok {
			t.Errorf("code %v lacks data", it["id"])
		}
	case "pinout":
		for _, key := range []string{"tags", "pins", "variants"} {
			if _, ok := it[key].([]any); !ok {
				t.Errorf("pinout %v: %s = %#v, want an array", it["id"], key, it[key])
			}
		}
		if _, ok := it["shape"].(map[string]any); !ok {
			t.Errorf("pinout %v: shape = %#v, want an object", it["id"], it["shape"])
		}
	default:
		t.Errorf("item %v has the unknown kind %v", it["id"], it["kind"])
	}
}

func TestLoadRejectsBrokenTables(t *testing.T) {
	long := strings.Repeat("a", 64)
	cases := []struct{ name, old, new, want string }{
		{"wrong kind", `kind: table`, `kind: calculator`, "kind must be table in tables/"},
		{"wrong category", `category: tables`, `category: basics`, "category must be tables"},
		{"unknown group", `group: conductors`, `group: misc`, `group must be one of conductors, components, materials, general, got "misc"`},
		{"missing locale", `, zh: "T"}`, `}`, `missing "zh" text`},
		{"doc without german", `doc: {en: "Doc.", de: "Doku."}`, `doc: {en: "Doc."}`, `doc: missing "de" text`},
		{"notes without german", `notes: {en: "Note.", de: "Hinweis."}`, `notes: {en: "Note."}`, `notes: missing "de" text`},
		{"missing source", `source: "Textbook"`, `source: ""`, "source is required"},
		{"no tags", `tags: [test]`, `tags: []`, "at least one tag is required"},
		{"unknown field", `guide: true`, "guide: true\n  colour: red", "colour"},
		{"bad id", `id: t_sec`, `id: T-sec`, "id must match"},
		{"id too long", `id: t_sec`, `id: ` + long, "id must be at most 63 characters"},
		{"unknown dimension", `dim: current}`, `dim: speed}`, `unknown dimension "speed"`},
		{"unit that is no explicit unit", `dim: current}`, `dim: current, unit: dBm}`, `unit "dBm" is not an explicit unit of current`},
		{"units entry of another dimension", `units: [mm2, kcmil]`, `units: [mm2, Wh]`, `units entry "Wh" is not an explicit unit of area`},
		{"unit missing from the units list", `unit: mm2, units: [mm2, kcmil]`, `unit: mm2, units: [kcmil]`, `unit "mm2" is not in its units list`},
		{"number system as cell unit", `dim: current}`, `dim: integer, unit: hex}`, `the number system "hex" cannot be the unit of table cells`},
		{"duplicate column", `{id: amps,`, `{id: section,`, `column "section" is listed twice`},
		{"bad column id", `{id: amps,`, `{id: Amps,`, `column id "Amps" must match`},
		{"text column with a unit", `dim: text}`, `dim: text, unit: mm2}`, "a text column has no unit"},
		{"column name without german", `name: {en: Current, de: Strom}`, `name: {en: Current}`, `column amps: name: missing "de" text`},
		{"short row", `    - [1.5, 15.5, 70, "—"]`, `    - [1.5, 15.5, 70]`, "row 1 has 3 cells, the table has 4 columns"},
		{"text in a numeric column", `[2.5, 19.5, 90,`, `[2.5, "19.5", 90,`, "row 2, column amps: a number or null is required"},
		{"number in a text column", `[1.5, 15.5, 70, "—"]`, `[1.5, 15.5, 70, 5]`, "row 1, column note: a text column needs a string"},
		{"infinite number", `[1.5, 15.5, 70,`, `[1.5, .inf, 70,`, "is not a finite number"},
		{"text cell with an unknown language", `{en: "see note", de: "siehe Anmerkung"}`, `{en: "see note", xx: "y"}`, `unknown language key "xx"`},
		{"text cell without english", `{en: "see note", de: "siehe Anmerkung"}`, `{de: "siehe Anmerkung"}`, `missing "en" text`},
		{"duplicate key", `    - [2.5, 19.5, 90,`, `    - [1.5, 19.5, 90,`, "rows 1 and 2 have the same key in column section"},
		{"keys within 1e-9 are the same", `    - [2.5, 19.5, 90,`, `    - [1.5000000001, 19.5, 90,`, "have the same key"},
		{"empty key cell", `    - [4, null, 90, ""]`, `    - [null, null, 90, ""]`, "the key column section must not be empty"},
		{"unknown key column", `key: section`, `key: nope`, `key "nope" is not a column of the table`},
		{"unknown generator", `key: section`, "generate: nope\n  key: section", `generate "nope" is not one of`},
		{"finder with a column that does not exist", `key: section`, "key: section\n  finder: {calc: c, column: nope, var: v}", `finder: column "nope" is not a column of the table`},
		{"finder on a text column", `key: section`, "key: section\n  finder: {calc: c, column: note, var: v}", `finder: column "note" is a text column`},
		{"finder without a calculator", `key: section`, "key: section\n  finder: {column: amps, var: v}", "finder needs calc and var"},
	}
	for _, c := range cases {
		broken := mutate(t, baseTable, c.old, c.new)
		_, err := Load(treeFS(t, map[string]string{"tables/t.yaml": broken}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}

	// A table without rows, or with an empty list.
	head := baseTable[:strings.Index(baseTable, "  rows:")]
	for name, src := range map[string]string{"missing rows": head, "empty rows": head + "  rows: []\n"} {
		want := map[string]string{"missing rows": "rows are required", "empty rows": "rows must not be empty"}[name]
		if _, err := Load(treeFS(t, map[string]string{"tables/t.yaml": src})); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to contain %q", name, err, want)
		}
	}
	if _, err := Load(treeFS(t, map[string]string{"tables/t.yaml": head + "  rows: 5\n"})); err == nil || !strings.Contains(err.Error(), "rows must be a list") {
		t.Errorf("rows that are no list: error = %v", err)
	}
	if _, err := Load(treeFS(t, map[string]string{"tables/t.yaml": head})); err == nil {
		t.Error("a table without rows must not load")
	}
}

// A zero-width gap in the checks above: a table whose columns are missing, and two files.
func TestLoadRejectsEmptyTableFilesAndMissingColumns(t *testing.T) {
	if _, err := Load(treeFS(t, map[string]string{"tables/a.yaml": ""})); err == nil || !strings.Contains(err.Error(), "the file is empty") {
		t.Errorf("empty file: error = %v", err)
	}
	noColumns := baseTable[:strings.Index(baseTable, "  columns:")]
	if _, err := Load(treeFS(t, map[string]string{"tables/t.yaml": noColumns})); err == nil || !strings.Contains(err.Error(), "columns are required") {
		t.Errorf("no columns: error = %v", err)
	}
}

// Ids are unique across calculators, tables and codes; every kind lives in its own directory.
func TestItemKindsShareOneIDSpaceAndOwnTheirDirectory(t *testing.T) {
	sameID := strings.Replace(baseTable, "id: t_sec", "id: t_div", 1)
	_, err := Load(treeFS(t, map[string]string{"calculators/c.yaml": baseCalc, "tables/t.yaml": sameID}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in calculators/c.yaml") {
		t.Errorf("table with the id of a calculator: error = %v", err)
	}
	_, err = Load(treeFS(t, map[string]string{"tables/a.yaml": baseTable, "tables/b.yaml": baseTable}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in tables/a.yaml") {
		t.Errorf("two tables with one id: error = %v", err)
	}

	// A calculator in tables/, a table in calculators/.
	_, err = Load(treeFS(t, map[string]string{"tables/c.yaml": baseCalc}))
	if err == nil || !strings.Contains(err.Error(), "field vars not found in type ecalc.rawTable") {
		t.Errorf("calculator in tables/: error = %v", err)
	}
	_, err = Load(treeFS(t, map[string]string{"calculators/t.yaml": baseTable}))
	if err == nil || !strings.Contains(err.Error(), "field group not found") {
		t.Errorf("table in calculators/: error = %v", err)
	}
	_, err = Load(treeFS(t, map[string]string{"calculators/t.yaml": strings.Replace(baseCalc, "kind: calculator", "kind: table", 1)}))
	if err == nil || !strings.Contains(err.Error(), "kind must be calculator in calculators/") {
		t.Errorf("kind table in calculators/: error = %v", err)
	}

	// A calculator belongs to a calculator category.
	for _, cat := range []string{"tables", "codes", "pinouts"} {
		_, err = Load(treeFS(t, map[string]string{"calculators/c.yaml": strings.Replace(baseCalc, "category: basics", "category: "+cat, 1)}))
		if err == nil || !strings.Contains(err.Error(), "reserved for other kinds of items") {
			t.Errorf("calculator in category %s: error = %v", cat, err)
		}
	}

	// All three kinds side by side: the JSON lists calculators, then tables, then codes.
	cat, err := Load(treeFS(t, map[string]string{"calculators/c.yaml": baseCalc, "tables/t.yaml": baseTable}))
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := cat.Item("t_div"); k != "calculator" || cat.Calculator("t_div") == nil || cat.Table("t_div") != nil {
		t.Errorf("t_div is a %q", k)
	}
	body, _, _ := cat.LocalizedJSON("en")
	var doc struct {
		Items []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 2 || doc.Items[0].Kind != "calculator" || doc.Items[1].Kind != "table" || doc.Items[1].ID != "t_sec" {
		t.Errorf("items = %+v, want the calculator then the table", doc.Items)
	}
}

// The primitives behind lookup, lookup_ge and interp, on a table with an empty cell and a decreasing column.
func TestTablePrimitives(t *testing.T) {
	tbl := loadTable(t, strings.ReplaceAll(`- id: t_prim
  kind: table
  category: tables
  group: general
  name: NAME16
  tags: [test]
  source: "Textbook"
  columns:
    - {id: x, name: {en: X, de: X}, dim: ratio}
    - {id: y, name: {en: Y, de: Y}, dim: ratio}
    - {id: d, name: {en: D, de: D}, dim: ratio}
    - {id: n, name: {en: N, de: N}, dim: text}
  rows:
    - [1, 10, 9, "a"]
    - [2, 20, 6, "b"]
    - [4, 15, 3, "c"]
    - [8, null, 0, "d"]
`, "NAME16", flow16("T")))
	nan := math.NaN()
	check := func(name string, got, want float64) {
		t.Helper()
		if !near(got, want, 1e-12) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("Lookup y at 2", tbl.Lookup("y", 2), 20)
	check("Lookup y at 8 (empty cell)", tbl.Lookup("y", 8), nan)
	check("Lookup y at 3 (no row)", tbl.Lookup("y", 3), nan)
	check("Lookup y within 1e-9", tbl.Lookup("y", 2.0000000001), 20)
	check("Lookup y just outside 1e-9", tbl.Lookup("y", 2.000001), nan)
	check("Lookup of a text column", tbl.Lookup("n", 2), nan)
	check("Lookup of an unknown column", tbl.Lookup("q", 2), nan)
	check("LookupGE y>=12 -> x", tbl.LookupGE("y", 12, "x"), 2)
	check("LookupGE y>=21", tbl.LookupGE("y", 21, "x"), nan)
	check("LookupGE y>=0 -> d", tbl.LookupGE("y", 0, "d"), 9)
	check("LookupGE x>=5 hits the empty y", tbl.LookupGE("x", 5, "y"), nan)
	check("LookupGE x>=8 (equal counts)", tbl.LookupGE("x", 8, "d"), 0)
	check("LookupGE NaN", tbl.LookupGE("x", nan, "d"), nan)
	check("LookupGE with a text output", tbl.LookupGE("x", 1, "n"), nan)
	check("Interp node", tbl.Interp("x", "d", 2), 6)
	check("Interp middle", tbl.Interp("x", "d", 3), 4.5)
	check("Interp end node", tbl.Interp("x", "d", 8), 0)
	check("Interp below", tbl.Interp("x", "d", 0.5), nan)
	check("Interp above", tbl.Interp("x", "d", 9), nan)
	check("Interp NaN", tbl.Interp("x", "d", nan), nan)
	check("Interp skips rows with an empty cell", tbl.Interp("x", "y", 3), 17.5)
	check("Interp ends where y ends", tbl.Interp("x", "y", 6), nan)
	if i := tbl.FindRow(0, 4); i != 2 {
		t.Errorf("FindRow(x, 4) = %d, want 2", i)
	}
	if i := tbl.FindRow(3, 4); i != -1 {
		t.Errorf("FindRow on a text column = %d, want -1", i)
	}
	if i := tbl.FindRow(1, 15.0000000001); i != 2 {
		t.Errorf("FindRow within 1e-9 = %d, want 2", i)
	}
}
