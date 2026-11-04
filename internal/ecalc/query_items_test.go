package ecalc

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// queryTables is the table of the query tests: a key column in mm2, a current with an empty cell, a temperature in degC
// (not monotonic, so interp refuses it as x), and a text column with a plain, a localized and an empty string. It is
// a guide table and a mains table.
var queryTables = strings.ReplaceAll(`- id: t_amp
  kind: table
  category: tables
  group: conductors
  name: NAME16
  doc: {en: "Ampacity of copper cables. A second sentence.", de: "Strombelastbarkeit von Kupferkabeln."}
  notes: {en: "Installation method B2.", de: "Verlegeart B2."}
  tags: [ampacity, kabel]
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
    - [1.5, 15.5, 70, "ok"]
    - [2.5, 19.5, 70, {en: "see note", de: "siehe Anmerkung"}]
    - [4, 26, 90, ""]
    - [6, null, 90, "ok"]
    - [10, 46, 90, "—"]
`, "NAME16", flow16("T"))

// suggestedIDs reads the ids out of "...; did you mean a, b, c?" (none when the message has no suggestions).
func suggestedIDs(message string) []string {
	_, list, ok := strings.Cut(message, "did you mean ")
	if !ok {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.TrimSpace(list), "?"), ", ")
}

func queryFixture(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(treeFS(t, map[string]string{"tables/amp.yaml": queryTables, "codes/k.yaml": baseCode}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func derefAll(row []*string) []string {
	out := make([]string, len(row))
	for i, p := range row {
		out[i] = "<null>"
		if p != nil {
			out[i] = *p
		}
	}
	return out
}

// ---- search and describe ---------------------------------------------------------------------------------------

func TestSearchFindsTablesAndCodes(t *testing.T) {
	cat := queryFixture(t)
	for query, want := range map[string]SearchHit{
		"t_amp":       {ID: "t_amp", Kind: "table", Category: "tables", Name: "T", Summary: "Ampacity of copper cables."},
		"kabel":       {ID: "t_amp", Kind: "table", Category: "tables", Name: "T", Summary: "Ampacity of copper cables."},
		"ampacity":    {ID: "t_amp", Kind: "table", Category: "tables", Name: "T", Summary: "Ampacity of copper cables."},
		"t_code":      {ID: "t_code", Kind: "code", Category: "codes", Name: "T", Summary: "Doc."},
		"codes":       {ID: "t_code", Kind: "code", Category: "codes", Name: "T", Summary: "Doc."},
		"T_CODE  ":    {ID: "t_code", Kind: "code", Category: "codes", Name: "T", Summary: "Doc."},
		"tables":      {ID: "t_amp", Kind: "table", Category: "tables", Name: "T", Summary: "Ampacity of copper cables."},
		"Querschnitt": {},
	} {
		hits := cat.SearchAll(query)
		if want.ID == "" {
			if len(hits) != 0 {
				t.Errorf("SearchAll(%q) = %v, want nothing (column names are not searched)", query, hitIDs(hits))
			}
			continue
		}
		if len(hits) == 0 || hits[0] != want {
			t.Errorf("SearchAll(%q) = %+v, want %+v first", query, hits, want)
		}
	}
	// An empty query lists the tables, then the codes; a table without a doc is summarized by its shape.
	if got := hitIDs(cat.SearchAll("")); strings.Join(got, ",") != "t_amp,t_code" {
		t.Errorf("SearchAll(\"\") = %v", got)
	}
	fix := loadFixture(t)
	hit := fix.SearchAll("t_txt")[0]
	if hit.Kind != "table" || hit.Summary != "Table with 2 rows; columns: Name, V." {
		t.Errorf("a table without doc: %+v", hit)
	}
}

func TestSearchFindsTheGeneratedTablesOfTheDefaultCatalog(t *testing.T) {
	cat := queryCatalog(t)
	// The exact id beats the calculator awg_conversion; the category lists every table.
	if hits := cat.SearchAll("awg"); len(hits) < 2 || hits[0].ID != "awg" || hits[0].Kind != "table" || hits[0].Category != "tables" || hits[0].Summary == "" {
		t.Fatalf("SearchAll(awg) = %+v", hits)
	}
	listed := map[string]bool{}
	for _, h := range cat.SearchAll("tables") {
		listed[h.ID] = h.Kind == "table"
	}
	for _, tbl := range cat.Tables {
		if !listed[tbl.ID] {
			t.Errorf("SearchAll(tables) does not list the table %s", tbl.ID)
		}
	}
}

func TestOverviewCountsItemsOfEveryKind(t *testing.T) {
	cat := queryFixture(t)
	got := cat.Overview()
	if len(got) != 2 || got[0] != (CategorySummary{ID: "tables", Name: "Tables", Count: 1}) || got[1] != (CategorySummary{ID: "codes", Name: "Codes", Count: 1}) {
		t.Fatalf("overview = %+v, want the two categories that hold items", got)
	}
}

func TestDescribeTable(t *testing.T) {
	cat := queryFixture(t)
	d, err := cat.DescribeTable(" T_AMP ")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "t_amp" || d.Kind != "table" || d.Category != "tables" || d.Group != "conductors" || d.Name != "T" || d.Source != "Textbook" ||
		d.Doc != "Ampacity of copper cables. A second sentence." || d.Notes != "Installation method B2." || d.Key != "section" || d.RowCount != 5 || d.Finder != nil {
		t.Fatalf("description = %+v", d)
	}
	if !d.Guide || !d.Mains || !strings.Contains(d.Caution, "standard itself is authoritative") || !strings.Contains(d.Caution, "qualified persons") {
		t.Errorf("guide %v mains %v caution %q", d.Guide, d.Mains, d.Caution)
	}
	want := []TableColumnInfo{
		{ID: "section", Name: "Cross-section", Dim: "area", Unit: "mm²", Units: []string{"mm²", "kcmil"}, Key: true},
		{ID: "amps", Name: "Current", Dim: "current", Units: d.Columns[1].Units},
		{ID: "temp", Name: "Temperature", Dim: "temperature", Unit: "°C", Units: d.Columns[2].Units},
		{ID: "note", Name: "Note", Dim: "text"},
	}
	for i, w := range want {
		g := d.Columns[i]
		if g.ID != w.ID || g.Name != w.Name || g.Dim != w.Dim || g.Unit != w.Unit || g.Key != w.Key || strings.Join(g.Units, ",") != strings.Join(w.Units, ",") {
			t.Errorf("column %d = %+v, want %+v", i, g, w)
		}
	}
	if len(d.Columns[2].Units) < 3 || len(d.Columns[3].Units) != 0 {
		t.Errorf("a column without a unit list offers every unit of its dimension, text offers none: %v %v", d.Columns[2].Units, d.Columns[3].Units)
	}
	// A table with a finder names its calculator and the fixed choices.
	f, err := loadFixture(t).DescribeTable("t_fix")
	if err != nil {
		t.Fatal(err)
	}
	if f.Finder == nil || f.Finder.Calculator != "ec_fix" || f.Finder.Column != "x" || f.Finder.Var != "q" || len(f.Finder.Choices) != 1 || f.Finder.Choices[0] != (NamedValue{Name: "src", Value: "a"}) {
		t.Errorf("finder = %+v", f.Finder)
	}
	if f.Guide || f.Mains || f.Caution != "" {
		t.Errorf("a plain table has no cautions: %+v", f)
	}
	data, err := json.Marshal(d)
	if err != nil || !strings.Contains(string(data), `"row_count":5`) || strings.Contains(string(data), `"finder"`) {
		t.Errorf("JSON = %s (%v)", data, err)
	}
}

func TestResolveItemAndTheErrorsOfTheWrongKind(t *testing.T) {
	cat := queryFixture(t)
	for id, want := range map[string]string{" T_AMP ": "table", "t_code": "code"} {
		kind, canonical, err := cat.ResolveItem(id)
		if err != nil || kind != want || canonical != strings.ToLower(strings.TrimSpace(id)) {
			t.Errorf("ResolveItem(%q) = %q %q %v", id, kind, canonical, err)
		}
	}
	real := queryCatalog(t)
	if kind, id, err := real.ResolveItem("Voltage_Divider"); err != nil || kind != "calculator" || id != "voltage_divider" {
		t.Errorf("ResolveItem(Voltage_Divider) = %q %q %v", kind, id, err)
	}
	_, _, err := real.ResolveItem("")
	wantQueryError(t, err, CodeUnknownID, "id is required", "calculator, table, code or pinout")
	_, _, err = real.ResolveItem("awg_wire")
	wantQueryError(t, err, CodeUnknownID, `unknown id "awg_wire"`, "did you mean", "awg")

	// The id of an item of another kind says what it is and which operation reads it.
	_, err = cat.Describe("t_amp")
	wantQueryError(t, err, CodeNotACalculator, "t_amp is a table, not a calculator", "table reads its rows")
	_, err = cat.Calculate("t_code", CalcRequest{})
	wantQueryError(t, err, CodeNotACalculator, "t_code is a code, not a calculator", "decode and encode")
	_, err = cat.Check("t_amp", CalcRequest{})
	wantQueryError(t, err, CodeNotACalculator)
	_, err = real.DescribeTable("voltage_divider")
	wantQueryError(t, err, CodeNotATable, "voltage_divider is a calculator, not a table", "describe and calculate")
	// An unknown id suggests items of the wanted kind only. Which other tables are suggested depends on the content of
	// the catalog, so the tests look at the list, not at its wording or order.
	_, err = real.DescribeTable("awg_table")
	qe := wantQueryError(t, err, CodeUnknownID, `unknown table "awg_table"`, "did you mean")
	if got := suggestedIDs(qe.Message); !slices.Contains(got, "awg") || len(got) > 3 {
		t.Errorf("suggestions %v of %q, want at most three and awg among them", got, qe.Message)
	}
	_, err = real.DescribeTable("awg_conv")
	qe = wantQueryError(t, err, CodeUnknownID, `unknown table "awg_conv"`)
	for _, id := range suggestedIDs(qe.Message) {
		if kind, _ := real.Item(id); kind != "table" {
			t.Errorf("only tables are suggested for a table, got the %s %s: %s", kind, id, qe.Message)
		}
	}
	_, err = real.DescribeTable("")
	wantQueryError(t, err, CodeUnknownID, "id is required", "table id")
}
