package ecalc

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rows and lookups of QueryTable: the fixture table t_amp of query_items_test.go, the fixture catalog of the table
// functions (loadFixture) and the generated tables of the default catalog.

func TestQueryTableFormatsTheCellsInTheColumnUnits(t *testing.T) {
	cat := queryFixture(t)
	res, err := cat.QueryTable("t_amp", TableRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "t_amp" || res.Name != "T" || res.Total != 5 || res.Offset != 0 || res.Returned != 5 || res.Truncated || res.NextOffset != 0 || res.Lookup != nil ||
		res.Source != "Textbook" || res.Notes != "Installation method B2." || !res.Guide || !res.Mains || res.Note != "" || len(res.Columns) != 4 || res.Columns[0].Units != nil {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.Caution, "authoritative") || !strings.Contains(res.Caution, "qualified persons") {
		t.Errorf("caution = %q", res.Caution)
	}
	if got := derefAll(res.Rows[0]); strings.Join(got, "|") != "1.5 mm²|15.5 A|70 °C|ok" {
		t.Errorf("row 0 = %v", got)
	}
	if got := derefAll(res.Rows[1]); strings.Join(got, "|") != "2.5 mm²|19.5 A|70 °C|see note" {
		t.Errorf("row 1 = %v (a localized cell reads in English)", got)
	}
	// An empty cell is null, an empty string is not.
	if got := derefAll(res.Rows[3]); strings.Join(got, "|") != "6 mm²|<null>|90 °C|ok" || res.SI[3][1] != nil {
		t.Errorf("row 3 = %v, si %v", got, res.SI[3])
	}
	if res.Rows[2][3] == nil || *res.Rows[2][3] != "" || res.SI[2][3] != nil {
		t.Errorf("an empty string is a text cell: %v", res.Rows[2][3])
	}
	// SI is the number of the cell in SI base units; text has none.
	if s := res.SI[0]; s[0] == nil || *s[0] != 1.5e-6 || *s[1] != 15.5 || !near(*s[2], 343.15, 1e-12) || s[3] != nil {
		t.Errorf("si row 0 = %v %v %v %v", s[0], s[1], s[2], s[3])
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"rows":[["1.5 mm²","15.5 A","70 °C","ok"],`, `["6 mm²",null,"90 °C","ok"]`, `"si":[[0.0000015,15.5,343.15,null],[0.0000025,19.5,343.15,null],`, `"returned":5`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON lacks %s: %s", want, data)
		}
	}
	// Columns carry the unit symbol of their cells; text, plain numbers and columns whose cells carry their own prefix none.
	wantCols := []TableColumnInfo{
		{ID: "section", Name: "Cross-section", Dim: "area", Unit: "mm²", Key: true}, {ID: "amps", Name: "Current", Dim: "current"},
		{ID: "temp", Name: "Temperature", Dim: "temperature", Unit: "°C"}, {ID: "note", Name: "Note", Dim: "text"},
	}
	for i, w := range wantCols {
		if g := res.Columns[i]; g.ID != w.ID || g.Name != w.Name || g.Dim != w.Dim || g.Unit != w.Unit || g.Key != w.Key {
			t.Errorf("column %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestQueryTableRefusesOtherKindsAndUnknownIDs(t *testing.T) {
	real := queryCatalog(t)
	_, err := real.QueryTable("voltage_divider", TableRequest{})
	wantQueryError(t, err, CodeNotATable, "voltage_divider is a calculator, not a table")
	_, err = real.QueryTable("nope", TableRequest{})
	wantQueryError(t, err, CodeUnknownID, `unknown table "nope"`)
	_, err = real.QueryTable("", TableRequest{})
	wantQueryError(t, err, CodeUnknownID, "id is required")
}

func TestQueryTablePagesTheRows(t *testing.T) {
	cat := queryFixture(t)
	type page struct {
		offset, limit, returned, next int
		truncated                     bool
		first                         string
	}
	for _, p := range []page{
		{0, 2, 2, 2, true, "1.5 mm²"}, {2, 2, 2, 4, true, "4 mm²"}, {4, 2, 1, 0, false, "10 mm²"},
		{-3, 1, 1, 1, true, "1.5 mm²"}, {99, 2, 0, 0, false, ""}, {0, 5, 5, 0, false, "1.5 mm²"},
	} {
		res, err := cat.QueryTable("t_amp", TableRequest{Offset: p.offset, Limit: p.limit})
		if err != nil {
			t.Fatal(err)
		}
		first := ""
		if len(res.Rows) > 0 {
			first = *res.Rows[0][0]
		}
		if res.Total != 5 || res.Returned != p.returned || len(res.Rows) != p.returned || len(res.SI) != p.returned || res.Truncated != p.truncated || res.NextOffset != p.next || first != p.first || res.Offset != max(p.offset, 0) {
			t.Errorf("offset %d limit %d: %+v", p.offset, p.limit, res)
		}
	}
	// The limit is clamped to 1..50 and defaults to 20; E192 has 192 rows.
	real := queryCatalog(t)
	for limit, want := range map[int]int{0: DefaultTableLimit, -4: DefaultTableLimit, 7: 7, 500: MaxTableLimit} {
		res, err := real.QueryTable("eseries_e192", TableRequest{Limit: limit})
		if err != nil || res.Returned != want || res.Total != 192 || !res.Truncated || res.NextOffset != want {
			t.Errorf("limit %d: returned %d of %d next %d (%v)", limit, res.Returned, res.Total, res.NextOffset, err)
		}
	}
}

func TestQueryTableFilterMatchesAnyCellText(t *testing.T) {
	cat := queryFixture(t)
	for _, c := range []struct {
		filter string
		total  int
		first  string
	}{
		{"see note", 1, "2.5 mm²"}, {"SEE NOTE", 1, "2.5 mm²"}, {"siehe anmerkung", 1, "2.5 mm²"}, {"  ok ", 2, "1.5 mm²"},
		{"90", 3, "4 mm²"}, {"90 °c", 3, "4 mm²"}, {"mm²", 5, "1.5 mm²"}, {"19.5", 1, "2.5 mm²"}, {"—", 1, "10 mm²"}, {"nothing like it", 0, ""}, {"", 5, "1.5 mm²"},
	} {
		res, err := cat.QueryTable("t_amp", TableRequest{Filter: c.filter})
		if err != nil {
			t.Fatal(err)
		}
		first := ""
		if len(res.Rows) > 0 {
			first = *res.Rows[0][0]
		}
		if res.Total != c.total || res.Returned != c.total || first != c.first {
			t.Errorf("filter %q: total %d returned %d first %q, want %d and %q", c.filter, res.Total, res.Returned, first, c.total, c.first)
		}
	}
	// Total counts the matches, paging runs over them.
	res, _ := cat.QueryTable("t_amp", TableRequest{Filter: "90", Limit: 2})
	if res.Total != 3 || res.Returned != 2 || !res.Truncated || res.NextOffset != 2 {
		t.Errorf("paged filter = %+v", res)
	}
	if res, _ = cat.QueryTable("t_amp", TableRequest{Filter: "zzz"}); res.Note != "No row matches the filter." || res.Rows != nil {
		t.Errorf("no match = %+v", res)
	}
	// 128 characters are fine, 129 are not (characters, not bytes).
	if _, err := cat.QueryTable("t_amp", TableRequest{Filter: strings.Repeat("ä", MaxFilterLength)}); err != nil {
		t.Errorf("a filter of %d characters: %v", MaxFilterLength, err)
	}
	_, err := cat.QueryTable("t_amp", TableRequest{Filter: strings.Repeat("ä", MaxFilterLength+1)})
	wantQueryError(t, err, CodeInvalidFilter, "128 characters")
}

func TestTableResultKeepIsConsistent(t *testing.T) {
	cat := queryFixture(t)
	res, _ := cat.QueryTable("t_amp", TableRequest{Offset: 1, Limit: 4})
	res.Keep(2)
	if res.Returned != 2 || len(res.Rows) != 2 || len(res.SI) != 2 || !res.Truncated || res.NextOffset != 3 || res.Total != 5 {
		t.Errorf("after Keep(2) = %+v", res)
	}
	res.Keep(9)
	res.Keep(-1)
	if res.Returned != 2 {
		t.Errorf("Keep beyond the rows changes nothing: %+v", res)
	}
}

// ---- lookup ---------------------------------------------------------------------------------------------------

func lookupOf(t *testing.T, cat *Catalog, id string, lk TableLookup) *LookupResult {
	t.Helper()
	res, err := cat.QueryTable(id, TableRequest{Lookup: &lk})
	if err != nil {
		t.Fatalf("%+v: %v", lk, err)
	}
	if res.Lookup == nil {
		t.Fatalf("%+v: no lookup result", lk)
	}
	if res.Rows != nil || res.Returned != 0 || !strings.Contains(res.Note, "Only the lookup was answered") || res.Total != len(cat.Table(id).Rows) {
		t.Errorf("%+v: a lookup alone lists no rows: %+v", lk, res)
	}
	return res.Lookup
}

func TestLookupExact(t *testing.T) {
	cat := queryFixture(t)
	// The key column is the default column; a bare number is in the unit of the column, any unit of the dimension works.
	for _, value := range []string{"2.5", "2.5 mm2", "2.5mm2", "0.0000025 m²"} {
		l := lookupOf(t, cat, "t_amp", TableLookup{Value: value, Output: "amps"})
		if !l.Found || l.Mode != "exact" || l.Column != "section" || l.Value != "2.5 mm²" || l.Output != "amps" || l.Result != "19.5 A" || l.SI == nil || *l.SI != 19.5 ||
			strings.Join(derefAll(l.Row), "|") != "2.5 mm²|19.5 A|70 °C|see note" || l.Message != "" {
			t.Errorf("value %q: %+v", value, l)
		}
	}
	// Without an output column the whole row is the answer.
	if l := lookupOf(t, cat, "t_amp", TableLookup{Column: "SECTION", Value: "4", Mode: "EXACT"}); !l.Found || l.Result != "" || l.SI != nil || strings.Join(derefAll(l.Row), "|") != "4 mm²|26 A|90 °C|" {
		t.Errorf("whole row = %+v", l)
	}
	// Another numeric column: 70 is read in degC, the first row with 70 °C wins.
	if l := lookupOf(t, cat, "t_amp", TableLookup{Column: "temp", Value: "70", Output: "section"}); !l.Found || l.Value != "70 °C" || l.Result != "1.5 mm²" {
		t.Errorf("temperature = %+v", l)
	}
	// A text column compares text, case-insensitively, in English and German.
	if l := lookupOf(t, cat, "t_amp", TableLookup{Column: "note", Value: "SIEHE anmerkung", Output: "amps"}); !l.Found || l.Value != "SIEHE anmerkung" || l.Result != "19.5 A" {
		t.Errorf("text = %+v", l)
	}
	if l := lookupOf(t, cat, "t_amp", TableLookup{Column: "note", Value: "ok", Output: "section"}); !l.Found || l.Result != "1.5 mm²" {
		t.Errorf("first of two matches = %+v", l)
	}
	// Nothing found is an answer, not an error.
	l := lookupOf(t, cat, "t_amp", TableLookup{Value: "3", Output: "amps"})
	if l.Found || l.Result != "" || l.Row != nil || l.Message != "no row has section = 3 mm²" {
		t.Errorf("not found = %+v", l)
	}
	if l = lookupOf(t, cat, "t_amp", TableLookup{Column: "note", Value: "nope"}); l.Found || l.Message != `no row has note = "nope"` {
		t.Errorf("text not found = %+v", l)
	}
	// The row is there but the cell is empty.
	if l = lookupOf(t, cat, "t_amp", TableLookup{Value: "6", Output: "amps"}); !l.Found || l.Result != "" || l.SI != nil || l.Message != "the cell of amps is empty in that row" {
		t.Errorf("empty cell = %+v", l)
	}
}

func TestLookupGreaterOrEqual(t *testing.T) {
	cat := queryFixture(t)
	// The first row in table order whose column is at least the value (not the nearest, no tolerance).
	for value, want := range map[string]string{"20": "4 mm²", "19.5": "2.5 mm²", "15.5": "1.5 mm²", "0": "1.5 mm²", "46": "10 mm²"} {
		l := lookupOf(t, cat, "t_amp", TableLookup{Mode: "ge", Column: "amps", Value: value, Output: "section"})
		if !l.Found || l.Mode != "ge" || l.Result != want || l.Row == nil {
			t.Errorf("ge %s: %+v, want %s", value, l, want)
		}
	}
	l := lookupOf(t, cat, "t_amp", TableLookup{Mode: "ge", Column: "amps", Value: "47"})
	if l.Found || l.Row != nil || l.Message != "no row has amps of at least 47 A; the largest is 46 A" {
		t.Errorf("ge beyond the table = %+v", l)
	}
	// It agrees with the table function lookup_ge.
	tbl := cat.Table("t_amp")
	l = lookupOf(t, cat, "t_amp", TableLookup{Mode: "ge", Column: "amps", Value: "20", Output: "section"})
	if want := tbl.LookupGE("amps", 20, "section"); l.SI == nil || !near(*l.SI, want, 1e-11) {
		t.Errorf("ge = %v, LookupGE = %v", l.SI, want)
	}
}

func TestLookupInterpolates(t *testing.T) {
	cat := queryFixture(t)
	tbl := cat.Table("t_amp")
	// Linear between the rows where both columns are filled (the row of 6 mm2 has no current and is skipped).
	for value, want := range map[string]string{"2 mm2": "17.5 A", "7": "36 A", "1.5": "15.5 A", "10": "46 A"} {
		l := lookupOf(t, cat, "t_amp", TableLookup{Mode: "interp", Column: "section", Value: value, Output: "amps"})
		if !l.Found || l.Result != want || l.SI == nil || l.Row != nil {
			t.Errorf("interp %s: %+v, want %s", value, l, want)
		}
	}
	l := lookupOf(t, cat, "t_amp", TableLookup{Mode: "interp", Column: "section", Value: "7", Output: "amps"})
	if want := tbl.Interp("section", "amps", 7e-6); l.SI == nil || !near(*l.SI, want, 1e-11) {
		t.Errorf("interp = %v, Interp = %v", l.SI, want)
	}
	l = lookupOf(t, cat, "t_amp", TableLookup{Mode: "interp", Column: "section", Value: "12 mm2", Output: "amps"})
	if l.Found || l.Message != "12 mm² is outside the range of section, 1.5 mm² to 10 mm²" {
		t.Errorf("interp outside = %+v", l)
	}
	// A decreasing x column works too (the range is given low to high).
	fix := loadFixture(t)
	l = lookupOf(t, fix, "t_dec", TableLookup{Mode: "interp", Column: "x", Value: "20", Output: "y"})
	if l.Found || !strings.Contains(l.Message, "outside the range of x, 2.5 to 10") {
		t.Errorf("decreasing outside = %+v", l)
	}
	if l = lookupOf(t, fix, "t_dec", TableLookup{Mode: "interp", Column: "x", Value: "7.5", Output: "y"}); !l.Found || l.Result != "1.5" {
		t.Errorf("decreasing = %+v", l)
	}
}

func TestLookupRefusesWhatCannotWork(t *testing.T) {
	cat := queryFixture(t)
	for _, c := range []struct {
		name string
		lk   TableLookup
		code string
		msg  string
	}{
		{"unknown column", TableLookup{Column: "nope", Value: "1"}, CodeUnknownColumn, `table t_amp has no column "nope" (lookup column); columns: section, amps, temp, note`},
		{"unknown output", TableLookup{Value: "1.5", Output: "nope"}, CodeUnknownColumn, `(lookup output)`},
		{"unknown mode", TableLookup{Mode: "nearest", Column: "amps", Value: "1"}, CodeInvalidLookup, `mode must be exact, ge or interp, got "nearest"`},
		{"no value", TableLookup{Column: "amps", Value: "  "}, CodeInvalidLookup, "needs a value"},
		{"ge without column", TableLookup{Mode: "ge", Value: "1"}, CodeInvalidLookup, "mode ge needs a column"},
		{"ge on text", TableLookup{Mode: "ge", Column: "note", Value: "a"}, CodeInvalidLookup, "holds text, so only the mode exact can search it; numeric columns: section, amps, temp"},
		{"interp without output", TableLookup{Mode: "interp", Column: "section", Value: "2"}, CodeInvalidLookup, "needs an output column"},
		{"interp of text", TableLookup{Mode: "interp", Column: "section", Value: "2", Output: "note"}, CodeInvalidLookup, "holds text and cannot be interpolated"},
		{"interp over a column that is not monotonic", TableLookup{Mode: "interp", Column: "temp", Value: "80", Output: "amps"}, CodeInvalidLookup, "not strictly monotonic"},
		{"value of the wrong shape", TableLookup{Column: "amps", Value: "abc"}, CodeParse, `cannot read "abc" for column amps as a current`},
		{"value in the wrong dimension", TableLookup{Column: "amps", Value: "3 V"}, CodeParse, `column amps`},
		{"value that is too long", TableLookup{Column: "amps", Value: strings.Repeat("1", MaxValueLength+1)}, CodeParse, `column amps`},
	} {
		_, err := cat.QueryTable("t_amp", TableRequest{Lookup: &c.lk})
		if qe := AsQueryError(err); qe == nil || qe.Code != c.code || !strings.Contains(qe.Message, c.msg) {
			t.Errorf("%s: %v, want code %s containing %q", c.name, err, c.code, c.msg)
		}
	}
}

func TestLookupAndRowsTogether(t *testing.T) {
	cat := queryFixture(t)
	lk := TableLookup{Column: "amps", Mode: "ge", Value: "20", Output: "section"}
	// A filter, an offset or a limit asks for rows too.
	for name, req := range map[string]TableRequest{"filter": {Filter: "ok"}, "limit": {Limit: 2}, "offset": {Offset: 1}} {
		req.Lookup = &lk
		res, err := cat.QueryTable("t_amp", req)
		if err != nil || res.Lookup == nil || !res.Lookup.Found || len(res.Rows) == 0 || res.Note != "" {
			t.Errorf("%s: %+v (%v)", name, res, err)
		}
	}
}

func TestLookupInTheGeneratedTables(t *testing.T) {
	cat := queryCatalog(t)
	// AWG 12 and AWG 10 by their number; ASTM B258 gives 2.053 mm for AWG 12 and 2.588 mm / 5.261 mm2 for AWG 10, and the
	// copper resistance of AWG 10 is 3.277 ohm per km.
	for _, c := range []struct {
		lk   TableLookup
		want string
	}{
		{TableLookup{Column: "awg", Value: "12", Output: "diameter"}, "2.053 mm"},
		{TableLookup{Column: "awg", Value: "10", Output: "diameter"}, "2.588 mm"},
		{TableLookup{Column: "awg", Value: "10", Output: "area"}, "5.261 mm²"},
		{TableLookup{Column: "awg", Value: "10", Output: "r_km"}, "3.277 Ω"},
		{TableLookup{Column: "size", Value: "4/0", Output: "diameter"}, "11.68 mm"},
		{TableLookup{Value: "1/0", Output: "awg"}, "0"},
		{TableLookup{Column: "awg", Value: "-3", Output: "size"}, "4/0"},
	} {
		if l := lookupOf(t, cat, "awg", c.lk); !l.Found || l.Result != c.want {
			t.Errorf("awg %+v = %+v, want %s", c.lk, l, c.want)
		}
	}
	// ge answers with the first row in table order, like lookup_ge: the AWG table falls from 4/0 to 40, so the first
	// row is the answer whenever its area is large enough.
	if l := lookupOf(t, cat, "awg", TableLookup{Mode: "ge", Column: "area", Value: "3", Output: "size"}); !l.Found || l.Result != "4/0" {
		t.Errorf("ge on a falling column = %+v", l)
	}
	// Between two rows of the IPC-2221 grid.
	ipc := cat.Table("ipc2221")
	lo, hi := ipc.Rows[3][1].Num, ipc.Rows[4][1].Num // 1 A and 1.5 A, external 1 oz at 10 degC
	l := lookupOf(t, cat, "ipc2221", TableLookup{Mode: "interp", Column: "current", Value: "1.25", Output: "ext_1oz_10"})
	if !l.Found || l.SI == nil || !near(*l.SI, (lo+hi)/2, 1e-11) {
		t.Errorf("ipc2221 interp = %+v, want %v", l, (lo+hi)/2)
	}
	// 4.7 is a value of E3, E6, E12 and E24.
	l = lookupOf(t, cat, "eseries_e24", TableLookup{Value: "4.7"})
	if !l.Found || strings.Join(derefAll(l.Row), "|") != "4.7|●|●|●|●" {
		t.Errorf("e24 4.7 = %+v", l)
	}
	if l = lookupOf(t, cat, "eseries_e24", TableLookup{Value: "4.8"}); l.Found {
		t.Errorf("4.8 is no E24 value: %+v", l)
	}
}
