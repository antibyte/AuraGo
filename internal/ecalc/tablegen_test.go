package ecalc

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func defaultTable(t *testing.T, id string) (*Catalog, *Table) {
	t.Helper()
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	tbl := cat.Table(id)
	if tbl == nil {
		t.Fatalf("the catalog has no table %s", id)
	}
	return cat, tbl
}

func TestGeneratedTablesAreInTheDefaultCatalog(t *testing.T) {
	cat, _ := defaultTable(t, "awg")
	want := map[string]string{"awg": "conductors", "ipc2221": "conductors", "eseries_e24": "components", "eseries_e192": "components"}
	for id, group := range want {
		tbl := cat.Table(id)
		if tbl == nil || tbl.Generate != id || tbl.Group != group || tbl.Kind != "table" || tbl.Category != "tables" {
			t.Errorf("%s = %+v, want a generated table of group %s", id, tbl, group)
			continue
		}
		if tbl.Doc["en"] == "" || tbl.Doc["de"] == "" || tbl.Notes["en"] == "" || tbl.Notes["de"] == "" || tbl.Source == "" || len(tbl.Name) != 16 {
			t.Errorf("%s lacks a doc, notes, a source or a name in 16 locales", id)
		}
	}
	if !cat.Table("ipc2221").Guide {
		t.Error("ipc2221 is a design guide: guide values")
	}
	if got := generatorIDs(); !reflect.DeepEqual(got, []string{"awg", "eseries_e192", "eseries_e24", "ipc2221"}) {
		t.Errorf("generatorIDs = %v", got)
	}
}

func TestGeneratedAWGTable(t *testing.T) {
	cat, tbl := defaultTable(t, "awg")
	if tbl.Key != "size" || len(tbl.Rows) != 44 {
		t.Fatalf("key %s, %d rows, want key size and the 44 sizes 4/0 ... 40", tbl.Key, len(tbl.Rows))
	}
	col := func(id string) *Column { return &tbl.Columns[tbl.ColumnIndex(id)] }
	if c := col("size"); c.Numeric() {
		t.Error("size is text")
	}
	if c := col("awg"); c.Dim != "number" || c.Unit != "" {
		t.Errorf("awg column = %+v", c)
	}
	if c := col("diameter"); c.Dim != "length" || c.Unit != "" || !reflect.DeepEqual(c.Units, []string{"", "in", "mil"}) {
		t.Errorf("diameter column = %+v", c)
	}
	if c := col("area"); c.Dim != "area" || c.Unit != "mm2" || !reflect.DeepEqual(c.Units, []string{"mm2", "kcmil"}) {
		t.Errorf("area column = %+v", c)
	}
	if c := col("r_km"); c.Dim != "resistance" || c.Unit != "" {
		t.Errorf("r_km column = %+v", c)
	}
	sizes := []string{"4/0", "3/0", "2/0", "1/0", "1", "2", "3"}
	for i, want := range sizes {
		if got := tbl.CellText(i, 0, "en"); got != want {
			t.Errorf("row %d size = %q, want %q", i, got, want)
		}
	}
	if tbl.CellText(43, 0, "en") != "40" || tbl.Rows[0][1].Num != -3 || tbl.Rows[3][1].Num != 0 || tbl.Rows[43][1].Num != 40 {
		t.Error("the sizes run from 4/0 (-3) through 1/0 (0) to 40")
	}

	// The rows agree with the AWG units of units.yaml (the same ASTM B258 formula, written there as expressions), and the
	// resistance is that of 1 km of annealed copper, 1/58 ohm*mm2/m.
	length, area := cat.Units.Dim("length").Unit("AWG"), cat.Units.Dim("area").Unit("AWG")
	di, ai, ri, ni := tbl.ColumnIndex("diameter"), tbl.ColumnIndex("area"), tbl.ColumnIndex("r_km"), tbl.ColumnIndex("awg")
	for r, row := range tbl.Rows {
		n := row[ni].Num
		if !near(row[di].Num, length.ToBase(n), 1e-12) || !near(row[ai].Num, area.ToBase(n), 1e-12) {
			t.Errorf("AWG %v: diameter %v / area %v differ from the AWG units %v / %v", n, row[di].Num, row[ai].Num, length.ToBase(n), area.ToBase(n))
		}
		if want := 1000 * (1e-6 / 58) / row[ai].Num; !near(row[ri].Num, want, 1e-12) {
			t.Errorf("AWG %v: r_km = %v, want %v", n, row[ri].Num, want)
		}
		if r > 0 {
			prev := tbl.Rows[r-1]
			if !(row[di].Num < prev[di].Num && row[ai].Num < prev[ai].Num && row[ri].Num > prev[ri].Num) {
				t.Errorf("AWG %v: a higher number is a thinner wire with more resistance", n)
			}
			if !near(row[di].Num/prev[di].Num, math.Pow(92, -1.0/39), 1e-12) {
				t.Errorf("AWG %v: the diameter ratio of neighbouring gauges is 92^(-1/39), got %v", n, row[di].Num/prev[di].Num)
			}
		}
	}
	// Defining sizes: AWG 36 is 0.005 in and 4/0 is 0.460 in.
	if !near(tbl.Rows[0][di].Num, 0.46*0.0254, 1e-12) || !near(tbl.Rows[39][di].Num, 0.005*0.0254, 1e-12) {
		t.Errorf("4/0 = %v m, AWG 36 = %v m", tbl.Rows[0][di].Num, tbl.Rows[39][di].Num)
	}
}

func TestGeneratedIPC2221Table(t *testing.T) {
	_, tbl := defaultTable(t, "ipc2221")
	if tbl.Key != "current" || len(tbl.Rows) != 12 || len(tbl.Columns) != 7 {
		t.Fatalf("key %s, %d rows, %d columns, want key current, 12 currents and 7 columns", tbl.Key, len(tbl.Rows), len(tbl.Columns))
	}
	amps := []float64{0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10}
	for r, want := range amps {
		if tbl.Rows[r][0].Num != want {
			t.Errorf("row %d current = %v, want %v", r, tbl.Rows[r][0].Num, want)
		}
	}
	for _, id := range []string{"ext_1oz_10", "ext_1oz_20", "ext_2oz_10", "int_1oz_10", "int_1oz_20", "int_2oz_10"} {
		c := tbl.Columns[tbl.ColumnIndex(id)]
		if c.Dim != "length" || c.Unit != "mm" || !reflect.DeepEqual(c.Units, []string{"mm", "mil"}) {
			t.Errorf("column %s = %+v, want length in mm, shown in mm or mil", id, c)
		}
	}
	// Every width solves the IPC-2221 equation back to its current: I = k * dT^0.44 * A^0.725, A in square mils.
	const mil, ounce = 25.4e-6, 35e-6
	type variant struct {
		id        string
		k, dT, oz float64
	}
	for _, v := range []variant{{"ext_1oz_10", 0.048, 10, 1}, {"ext_1oz_20", 0.048, 20, 1}, {"ext_2oz_10", 0.048, 10, 2}, {"int_1oz_10", 0.024, 10, 1}, {"int_1oz_20", 0.024, 20, 1}, {"int_2oz_10", 0.024, 10, 2}} {
		ci := tbl.ColumnIndex(v.id)
		for r, row := range tbl.Rows {
			areaMil2 := row[ci].Num * v.oz * ounce / (mil * mil)
			if got := v.k * math.Pow(v.dT, 0.44) * math.Pow(areaMil2, 0.725); !near(got, row[0].Num, 1e-9) {
				t.Errorf("%s at %v A: the width carries %v A", v.id, row[0].Num, got)
			}
			if r > 0 && row[ci].Num <= tbl.Rows[r-1][ci].Num {
				t.Errorf("%s: a larger current needs a wider trace", v.id)
			}
		}
	}
	// 2 oz halves the width, an inner layer needs 2^(1/0.725) times the outer width, a larger rise allows a narrower trace.
	e1, e2, i1, e20 := tbl.ColumnIndex("ext_1oz_10"), tbl.ColumnIndex("ext_2oz_10"), tbl.ColumnIndex("int_1oz_10"), tbl.ColumnIndex("ext_1oz_20")
	for _, row := range tbl.Rows {
		if !near(row[e2].Num, row[e1].Num/2, 1e-12) || !near(row[i1].Num/row[e1].Num, math.Pow(2, 1/0.725), 1e-12) || !(row[e20].Num < row[e1].Num) {
			t.Errorf("%v A: widths %v %v %v %v break the IPC-2221 relations", row[0].Num, row[e1].Num, row[e2].Num, row[i1].Num, row[e20].Num)
		}
	}
}

func TestGeneratedESeriesTables(t *testing.T) {
	_, e24 := defaultTable(t, "eseries_e24")
	_, e192 := defaultTable(t, "eseries_e192")
	if e24.Key != "value" || e192.Key != "value" || len(e24.Rows) != 24 || len(e192.Rows) != 192 {
		t.Fatalf("keys %s %s, rows %d %d, want key value and 24 and 192 rows", e24.Key, e192.Key, len(e24.Rows), len(e192.Rows))
	}
	check := func(tbl *Table, series ...string) {
		finest := series[len(series)-1]
		for r, row := range tbl.Rows {
			if row[0].Num != eseriesTables[finest][r] {
				t.Errorf("%s row %d = %v, want the %s value %v", tbl.ID, r, row[0].Num, finest, eseriesTables[finest][r])
			}
		}
		for _, s := range series {
			ci := tbl.ColumnIndex(strings.ToLower(s))
			if ci < 0 || tbl.Columns[ci].Numeric() || tbl.Columns[ci].Name["en"] != s {
				t.Fatalf("%s lacks the text column %s", tbl.ID, s)
			}
			marked := 0
			for r, row := range tbl.Rows {
				mark := tbl.CellText(r, ci, "en")
				if mark != "" && mark != "●" {
					t.Errorf("%s %s row %d = %q, want a dot or nothing", tbl.ID, s, r, mark)
				}
				if (mark == "●") != slices.Contains(eseriesTables[s], row[0].Num) {
					t.Errorf("%s: %v is marked %q in %s", tbl.ID, row[0].Num, mark, s)
				}
				if mark == "●" {
					marked++
				}
			}
			if marked != len(eseriesTables[s]) {
				t.Errorf("%s marks %d values in %s, want %d", tbl.ID, marked, s, len(eseriesTables[s]))
			}
		}
	}
	check(e24, "E3", "E6", "E12", "E24")
	check(e192, "E48", "E96", "E192")
	// The engine's tables nest: E3 in E6 in E12 in E24, and E48 in E96 in E192.
	for _, pair := range [][2]string{{"E3", "E6"}, {"E6", "E12"}, {"E12", "E24"}, {"E48", "E96"}, {"E96", "E192"}} {
		for _, v := range eseriesTables[pair[0]] {
			if !slices.Contains(eseriesTables[pair[1]], v) {
				t.Errorf("%v of %s is not in %s", v, pair[0], pair[1])
			}
		}
	}
}

// A generated table takes everything but the descriptive fields from the engine.
func TestLoadRejectsBrokenGeneratedTables(t *testing.T) {
	const awg = `- id: g_awg
  kind: table
  category: tables
  group: conductors
  name: NAME16
  tags: [test]
  source: "Textbook"
  generate: awg
`
	src := strings.ReplaceAll(awg, "NAME16", flow16("T"))
	cat, err := Load(treeFS(t, map[string]string{"tables/g.yaml": src}))
	if err != nil {
		t.Fatalf("a generated table: %v", err)
	}
	if g := cat.Table("g_awg"); g == nil || len(g.Rows) != 44 || g.Key != "size" {
		t.Fatalf("generated table = %+v", g)
	}

	// The key can be another column of the generated table.
	cat, err = Load(treeFS(t, map[string]string{"tables/g.yaml": src + "  key: awg\n"}))
	if err != nil || cat.Table("g_awg").Key != "awg" || cat.Table("g_awg").KeyColumn() != 1 {
		t.Errorf("key awg: %v", err)
	}

	cases := map[string]struct{ extra, want string }{
		"columns": {"  columns:\n    - {id: x, name: {en: X, de: X}, dim: ratio}\n", "a generated table must not list columns or rows"},
		"rows":    {"  rows:\n    - [1]\n", "a generated table must not list columns or rows"},
		"key":     {"  key: nope\n", `key "nope" is not a column of the table`},
	}
	for name, c := range cases {
		if _, err := Load(treeFS(t, map[string]string{"tables/g.yaml": src + c.extra})); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", name, err, c.want)
		}
	}
	unknown := strings.Replace(src, "generate: awg", "generate: awg2", 1)
	if _, err := Load(treeFS(t, map[string]string{"tables/g.yaml": unknown})); err == nil || !strings.Contains(err.Error(), `generate "awg2" is not one of awg, eseries_e192, eseries_e24, ipc2221`) {
		t.Errorf("unknown generator: error = %v", err)
	}
}

// The generated tables are served like any other table: SI numbers, localized names, all columns.
func TestGeneratedTablesInTheCatalogJSON(t *testing.T) {
	en, _, _ := fetch(t, "en")
	byID := map[string]int{}
	for i, it := range en.Items {
		byID[it.ID] = i
	}
	for _, id := range []string{"awg", "ipc2221", "eseries_e24", "eseries_e192"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("the catalog JSON lacks the generated table %s", id)
		}
	}
	if en.Items[byID["awg"]].Name != "AWG wire gauges" {
		t.Errorf("name = %q", en.Items[byID["awg"]].Name)
	}
	de, _, _ := fetch(t, "de")
	if de.Items[byID["awg"]].Name != "AWG-Leitergrößen" || de.Items[byID["awg"]].NameEN != "AWG wire gauges" {
		t.Errorf("de name %q / %q", de.Items[byID["awg"]].Name, de.Items[byID["awg"]].NameEN)
	}
}
