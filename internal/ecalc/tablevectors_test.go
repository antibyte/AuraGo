package ecalc

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// tableCellVector and tableLookupVector are the entries of testdata/vectors/tables/<table id>.json.
//
// A cell vector names a row by key (a number in the unit of the key column, or the English text of a text key), a column,
// and the value want that the cell must hold: a number in the unit of that column (matching at the relative error tol,
// 1e-9 when unset), the English text of a text cell, or null for an empty cell. A lookup vector evaluates expr (with env
// and refs) against the tables of the catalog and wants an SI number, or "nan", at the relative error tol. Every entry
// names its source in ref. A transcribed table has at least three cells, a generated one at least five, which the
// vectors check against published values.
type tableCellVector struct {
	Key    any     `json:"key"`
	Column string  `json:"column"`
	Want   any     `json:"want"`
	Tol    float64 `json:"tol"`
	Ref    string  `json:"ref"`
}

type tableLookupVector struct {
	Expr string             `json:"expr"`
	Env  map[string]float64 `json:"env"`
	Refs map[string]string  `json:"refs"`
	Want any                `json:"want"`
	Tol  float64            `json:"tol"`
	Ref  string             `json:"ref"`
}

type tableVectors struct {
	Table   string              `json:"table"`
	Cells   []tableCellVector   `json:"cells"`
	Lookups []tableLookupVector `json:"lookups"`
}

const tableVectorDir = "testdata/vectors/tables"

// checkTableCell checks one cell vector against the table; it returns a description of the problem or "".
func checkTableCell(t *Table, v tableCellVector) string {
	ci := t.ColumnIndex(v.Column)
	if ci < 0 {
		return fmt.Sprintf("table %s has no column %q", t.ID, v.Column)
	}
	key := &t.Columns[t.KeyColumn()]
	row := -1
	switch k := v.Key.(type) {
	case float64:
		if !key.Numeric() {
			return fmt.Sprintf("key %v is a number but the key column %s is text", k, key.ID)
		}
		row = t.FindRow(t.KeyColumn(), key.ToSI(k))
	case string:
		if key.Numeric() {
			return fmt.Sprintf("key %q is a string but the key column %s is numeric", k, key.ID)
		}
		for r := range t.Rows {
			if t.CellText(r, t.KeyColumn(), "en") == k {
				row = r
				break
			}
		}
	default:
		return fmt.Sprintf("key %v must be a number or a string", v.Key)
	}
	if row < 0 {
		return fmt.Sprintf("no row has the key %v in column %s", v.Key, key.ID)
	}
	cell, col := t.Rows[row][ci], &t.Columns[ci]
	switch want := v.Want.(type) {
	case nil:
		if !cell.Null {
			return fmt.Sprintf("row %v, column %s is not empty", v.Key, v.Column)
		}
	case string:
		if col.Numeric() {
			return fmt.Sprintf("row %v, column %s is numeric but want is the text %q", v.Key, v.Column, want)
		}
		if got := t.CellText(row, ci, "en"); cell.Null || got != want {
			return fmt.Sprintf("row %v, column %s = %q, want %q", v.Key, v.Column, got, want)
		}
	case float64:
		if !col.Numeric() {
			return fmt.Sprintf("row %v, column %s is text but want is the number %v", v.Key, v.Column, want)
		}
		if cell.Null {
			return fmt.Sprintf("row %v, column %s is empty, want %v", v.Key, v.Column, want)
		}
		got := col.FromSI(cell.Num)
		tol := v.Tol
		if tol == 0 {
			tol = 1e-9
		}
		if !near(got, want, tol) {
			return fmt.Sprintf("row %v, column %s = %v, want %v (tolerance %g)", v.Key, v.Column, got, want, tol)
		}
	default:
		return fmt.Sprintf("want %v must be a number, a string or null", v.Want)
	}
	return ""
}

func TestTableVectors(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Tables) == 0 {
		t.Fatal("the catalog has no tables")
	}
	for _, tbl := range cat.Tables {
		t.Run(tbl.ID, func(t *testing.T) {
			var v tableVectors
			readJSON(t, tableVectorDir+"/"+tbl.ID+".json", &v)
			if v.Table != tbl.ID {
				t.Fatalf("%s.json declares table %q", tbl.ID, v.Table)
			}
			need := 3
			if tbl.Generate != "" {
				need = 5
			}
			if len(v.Cells) < need {
				t.Errorf("%d cells, a table of this kind needs at least %d spot-checked cells", len(v.Cells), need)
			}
			for i, c := range v.Cells {
				if c.Ref == "" {
					t.Errorf("cell %d (%v, %s) has no ref", i, c.Key, c.Column)
				}
				if msg := checkTableCell(tbl, c); msg != "" {
					t.Errorf("cell %d (%s): %s", i, c.Ref, msg)
				}
			}
			for i, l := range v.Lookups {
				if l.Ref == "" {
					t.Errorf("lookup %d (%s) has no ref", i, l.Expr)
				}
				n, err := ParseExpr(l.Expr)
				if err != nil {
					t.Errorf("lookup %d %s: %v", i, l.Expr, err)
					continue
				}
				got, err := Eval(n, Env{Vals: l.Env, Refs: l.Refs, Tables: cat.tableByID})
				if err != nil {
					t.Errorf("lookup %d %s: %v", i, l.Expr, err)
					continue
				}
				tol := l.Tol
				if tol == 0 {
					tol = 1e-9
				}
				if want := vecNum(t, l.Want); !near(got, want, tol) {
					t.Errorf("lookup %d %s = %v, want %v (%s)", i, l.Expr, got, want, l.Ref)
				}
			}
		})
	}
}

// TestTableVectorFilesNameATable keeps the vector directory strict, like the calculator one: every file is <table>.json
// and names a table of the catalog, so vectors written before their table go red.
func TestTableVectorFilesNameATable(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.FromSlash(tableVectorDir + "/*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no table vector files found (%v)", err)
	}
	for _, file := range files {
		name := filepath.Base(file)
		if !strings.HasSuffix(name, ".json") {
			t.Errorf("%s: only <table>.json files belong in %s", name, tableVectorDir)
			continue
		}
		var v tableVectors
		readJSON(t, tableVectorDir+"/"+name, &v)
		switch {
		case v.Table == "":
			t.Errorf("%s declares no table", name)
		case name != v.Table+".json":
			t.Errorf("%s declares table %q, so the file must be called %s.json", name, v.Table, v.Table)
		case cat.Table(v.Table) == nil:
			t.Errorf("%s names the table %q, which the catalog does not have", name, v.Table)
		}
	}
}

// The checker itself: it must see a wrong value, a wrong text, a missing row, a wrong key type and an empty cell.
func TestTableCellVectorCheckerSeesMistakes(t *testing.T) {
	tbl := loadTable(t, baseTable)
	ok := []tableCellVector{
		{Key: 1.5, Column: "amps", Want: 15.5},
		{Key: 2.5, Column: "section", Want: 2.5},
		{Key: 1.5, Column: "temp", Want: 70.0},
		{Key: 2.5, Column: "note", Want: "see note"},
		{Key: 4.0, Column: "amps", Want: nil},
		{Key: 1.5, Column: "amps", Want: 15.51, Tol: 1e-3},
	}
	for _, c := range ok {
		if msg := checkTableCell(tbl, c); msg != "" {
			t.Errorf("%+v: %s", c, msg)
		}
	}
	bad := map[string]tableCellVector{
		"a wrong number":      {Key: 1.5, Column: "amps", Want: 15.6},
		"a wrong text":        {Key: 2.5, Column: "note", Want: "other"},
		"a missing row":       {Key: 3.0, Column: "amps", Want: 1.0},
		"an unknown column":   {Key: 1.5, Column: "nope", Want: 1.0},
		"a string key":        {Key: "1.5", Column: "amps", Want: 15.5},
		"a text for a number": {Key: 1.5, Column: "amps", Want: "15.5"},
		"a number for a text": {Key: 1.5, Column: "note", Want: 5.0},
		"an empty cell":       {Key: 4.0, Column: "amps", Want: 1.0},
		"a filled cell":       {Key: 1.5, Column: "amps", Want: nil},
	}
	for name, c := range bad {
		if msg := checkTableCell(tbl, c); msg == "" {
			t.Errorf("%s: the checker accepted %+v", name, c)
		}
	}
}
