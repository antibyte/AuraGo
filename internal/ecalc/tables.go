package ecalc

import (
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Table items: a table is a catalog item of kind "table" (tables/*.yaml): named columns with a dimension and a display
// unit, rows of cells that the loader converts to SI, an optional key column for lookup(), and optionally a finder (a
// calculator whose solved value selects the highlighted row). The table functions of the expression language
// (tablefuncs.go) read the same rows.

const (
	// textDim is the dimension of a column that holds text.
	textDim = "text"
	// tablesCategory and codesCategory are the categories of the table and code items.
	tablesCategory = "tables"
	codesCategory  = "codes"
	// tableRel is the relative tolerance with which a key, a finder value or a lookup key matches a numeric cell.
	tableRel = 1e-9
)

// tableGroups are the list sections of the table rail, in display order.
var tableGroups = []string{"conductors", "components", "materials", "general"}

// Column is one column of a table. Dim is a dimension of units.yaml or "text". Unit is the unit of the cells in the YAML
// and the default display unit ("" = the base symbol with automatic prefixes); Units lists the display units the page
// offers (the semantics of the units list of a variable).
type Column struct {
	ID    string
	Name  Text
	Dim   string
	Unit  string
	Units []string
	Label string

	dim  *Dim
	unit *Unit
}

// Numeric reports whether the column holds numbers (every dimension except text).
func (c *Column) Numeric() bool { return c.Dim != textDim }

// ToSI converts a number written in the unit of the column (Unit, "" = SI already) to SI.
func (c *Column) ToSI(x float64) float64 {
	if c.unit != nil {
		return c.unit.ToBase(x)
	}
	return x
}

// FromSI converts an SI value to the unit of the column (Unit); the page shows it in that unit unless the user picks
// another of Units.
func (c *Column) FromSI(v float64) float64 {
	if c.unit != nil {
		return c.unit.FromBase(v)
	}
	return v
}

// Cell is one cell of a table. A numeric cell holds the SI value in Num; a text cell holds the text (a plain string is
// stored under "en" and so is shown in every language). An empty cell is Null; Num is NaN for every cell that holds no
// number.
type Cell struct {
	Num  float64
	Text Text
	Null bool
}

// TableFinder ties a table to a calculator: the value Var of the solved calculator selects the row whose Column value
// equals it, with the options in Choices fixed (the finder header hides those choices).
type TableFinder struct {
	Calc    string
	Column  string
	Var     string
	Choices map[string]string
}

// Table is a catalog item of kind "table".
type Table struct {
	ID       string
	Kind     string
	Category string
	Group    string
	Name     Text
	Doc      Text
	Notes    Text
	Tags     []string
	Source   string
	Guide    bool // the "guide values, the standard is authoritative" banner
	Mains    bool // the mains safety note
	Key      string
	Generate string // the generator of a formula-derived table, "" for a transcribed one
	Columns  []Column
	Rows     [][]Cell
	Finder   *TableFinder

	colIdx map[string]int
	keyIdx int
}

// ColumnIndex returns the index of the column with the given id, or -1.
func (t *Table) ColumnIndex(id string) int {
	if i, ok := t.colIdx[id]; ok {
		return i
	}
	return -1
}

// KeyColumn returns the index of the key column.
func (t *Table) KeyColumn() int { return t.keyIdx }

// CellText returns the text of a cell in the given language (English where a language has no text); a numeric or empty
// cell gives "".
func (t *Table) CellText(row, col int, lang string) string {
	if row < 0 || row >= len(t.Rows) || col < 0 || col >= len(t.Columns) {
		return ""
	}
	if c := t.Rows[row][col]; !c.Null && !t.Columns[col].Numeric() {
		return c.Text.in(lang)
	}
	return ""
}

// closeEnough reports whether a and b are equal within tableRel (relative to the larger magnitude); exact equality
// counts, NaN and infinities never match unless identical.
func closeEnough(a, b float64) bool {
	if a == b {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	return math.Abs(a-b) <= tableRel*math.Max(math.Abs(a), math.Abs(b))
}

// FindRow returns the first row (table order) whose numeric column col equals x within 1e-9 relative, or -1.
func (t *Table) FindRow(col int, x float64) int {
	if col < 0 || col >= len(t.Columns) || !t.Columns[col].Numeric() {
		return -1
	}
	for i, row := range t.Rows {
		if !row[col].Null && closeEnough(row[col].Num, x) {
			return i
		}
	}
	return -1
}

// Lookup is the table function lookup(T, column, key): the SI value of the column in the row whose key column equals key
// (within 1e-9 relative). It is NaN when no row matches, the cell is empty, or the column is unknown or text.
func (t *Table) Lookup(column string, key float64) float64 {
	c := t.ColumnIndex(column)
	if c < 0 || !t.Columns[c].Numeric() {
		return math.NaN()
	}
	return t.lookup(c, key)
}

// LookupGE is the table function lookup_ge(T, column, x, out): the SI value of out in the first row (table order) whose
// column is not empty and at least x; NaN when there is none or a column is unknown or text.
func (t *Table) LookupGE(column string, x float64, out string) float64 {
	c, o := t.ColumnIndex(column), t.ColumnIndex(out)
	if c < 0 || o < 0 || !t.Columns[c].Numeric() || !t.Columns[o].Numeric() {
		return math.NaN()
	}
	return t.lookupGE(c, x, o)
}

// Interp is the table function interp(T, xcol, ycol, x): the linear interpolation of ycol over xcol through the rows
// where both are filled, exact at the nodes, NaN outside the range of xcol (and for unknown or text columns).
func (t *Table) Interp(xcol, ycol string, x float64) float64 {
	xc, yc := t.ColumnIndex(xcol), t.ColumnIndex(ycol)
	if xc < 0 || yc < 0 || !t.Columns[xc].Numeric() || !t.Columns[yc].Numeric() {
		return math.NaN()
	}
	return t.interp(xc, yc, x)
}

func (t *Table) lookup(c int, key float64) float64 {
	if i := t.FindRow(t.keyIdx, key); i >= 0 {
		return t.Rows[i][c].Num
	}
	return math.NaN()
}

func (t *Table) lookupGE(c int, x float64, out int) float64 {
	if math.IsNaN(x) {
		return math.NaN()
	}
	for _, row := range t.Rows {
		if !row[c].Null && row[c].Num >= x {
			return row[out].Num
		}
	}
	return math.NaN()
}

// points returns the rows where both columns are filled, in table order.
func (t *Table) points(xc, yc int) (xs, ys []float64) {
	for _, row := range t.Rows {
		if !row[xc].Null && !row[yc].Null {
			xs = append(xs, row[xc].Num)
			ys = append(ys, row[yc].Num)
		}
	}
	return xs, ys
}

func (t *Table) interp(xc, yc int, x float64) float64 {
	xs, ys := t.points(xc, yc)
	for i := range xs {
		if xs[i] == x {
			return ys[i]
		}
	}
	for i := 0; i+1 < len(xs); i++ {
		x0, x1 := xs[i], xs[i+1]
		if (x > x0 && x < x1) || (x < x0 && x > x1) {
			step := (ys[i+1] - ys[i]) * ((x - x0) / (x1 - x0))
			return ys[i] + float64(step) // the conversion keeps a fused multiply-add out, like the JavaScript twin
		}
	}
	return math.NaN()
}

// checkInterp reports whether interp over the two columns is defined: at least two rows where both are filled and the x
// values strictly monotonic over them.
func (t *Table) checkInterp(xc, yc int) error {
	xs, _ := t.points(xc, yc)
	if len(xs) < 2 {
		return fmt.Errorf("interp needs at least two rows where %q and %q are both filled in table %q", t.Columns[xc].ID, t.Columns[yc].ID, t.ID)
	}
	up, down := true, true
	for i := 1; i < len(xs); i++ {
		up = up && xs[i] > xs[i-1]
		down = down && xs[i] < xs[i-1]
	}
	if !up && !down {
		return fmt.Errorf("column %q of table %q is not strictly monotonic, interp cannot use it as x", t.Columns[xc].ID, t.ID)
	}
	return nil
}

// ---- loading ------------------------------------------------------------------------------------------------------

type rawColumn struct {
	ID    string   `yaml:"id"`
	Name  Text     `yaml:"name"`
	Dim   string   `yaml:"dim"`
	Unit  string   `yaml:"unit"`
	Units []string `yaml:"units"`
	Label string   `yaml:"label"`
}

type rawTableFinder struct {
	Calc    string            `yaml:"calc"`
	Column  string            `yaml:"column"`
	Var     string            `yaml:"var"`
	Choices map[string]string `yaml:"choices"`
}

type rawTable struct {
	ID       string          `yaml:"id"`
	Kind     string          `yaml:"kind"`
	Category string          `yaml:"category"`
	Group    string          `yaml:"group"`
	Name     Text            `yaml:"name"`
	Doc      Text            `yaml:"doc"`
	Notes    Text            `yaml:"notes"`
	Tags     []string        `yaml:"tags"`
	Source   string          `yaml:"source"`
	Guide    bool            `yaml:"guide"`
	Mains    bool            `yaml:"mains"`
	Key      string          `yaml:"key"`
	Generate string          `yaml:"generate"`
	Columns  []rawColumn     `yaml:"columns"`
	Rows     yaml.Node       `yaml:"rows"`
	Finder   *rawTableFinder `yaml:"finder"`
}

// tables reads tables/*.yaml (sorted) into the catalog.
func (l *loader) tables(fsys fs.FS, cat *Catalog) error {
	files, err := fs.Glob(fsys, "tables/*.yaml")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		raws, ok := readItems[rawTable](l, fsys, file)
		if !ok {
			continue
		}
		for i := range raws {
			if t := l.table(file, &raws[i]); t != nil {
				cat.Tables = append(cat.Tables, t)
				cat.tableByID[t.ID] = t
				cat.kinds[t.ID] = "table"
			}
		}
	}
	return nil
}

// table builds and validates one table item; it returns nil when something is wrong.
func (l *loader) table(file string, r *rawTable) *Table {
	before := len(l.errs)
	label := r.ID
	if label == "" {
		label = "(item without id)"
	}
	fail := func(format string, args ...any) {
		l.errs = append(l.errs, fmt.Errorf("%s: %s: %s", file, label, fmt.Sprintf(format, args...)))
	}
	text := func(what string, t Text, required []string) {
		l.checkText(fmt.Sprintf("%s: %s: %s", file, label, what), t, required)
	}
	t := &Table{
		ID: r.ID, Kind: r.Kind, Category: r.Category, Group: r.Group, Name: r.Name, Doc: r.Doc, Notes: r.Notes, Tags: r.Tags,
		Source: r.Source, Guide: r.Guide, Mains: r.Mains, Generate: r.Generate, colIdx: map[string]int{},
	}
	l.claimID(r.ID, file, fail)
	if r.Kind != "table" {
		fail("kind must be table in tables/, got %q", r.Kind)
	}
	if r.Category != tablesCategory {
		fail("category must be %s, got %q", tablesCategory, r.Category)
	}
	if !contains(tableGroups, r.Group) {
		fail("group must be one of %s, got %q", strings.Join(tableGroups, ", "), r.Group)
	}
	l.commonFields(fail, text, r.Name, r.Doc, r.Tags, r.Source)
	if len(r.Notes) > 0 {
		text("notes", r.Notes, docLocales)
	}

	// A generated table takes its columns and rows from the engine, a transcribed one lists them.
	cols, key := r.Columns, r.Key
	var rows [][]Cell
	if r.Generate != "" {
		gen := tableGenerators[r.Generate]
		switch {
		case gen == nil:
			fail("generate %q is not one of %s", r.Generate, strings.Join(generatorIDs(), ", "))
		case len(r.Columns) > 0 || r.Rows.Kind != 0:
			fail("a generated table must not list columns or rows")
		default:
			g := gen()
			cols, rows = g.cols, g.rows
			if key == "" {
				key = g.key
			}
		}
	}
	t.Columns = l.columns(fail, text, cols)
	for i, c := range t.Columns {
		t.colIdx[c.ID] = i
	}
	if r.Generate == "" && len(t.Columns) > 0 {
		rows = l.rows(fail, text, t, &r.Rows)
	}
	t.Rows = rows
	if len(t.Columns) > 0 && len(t.Rows) > 0 {
		t.validateKey(fail, key)
	}

	if f := r.Finder; f != nil {
		tf := &TableFinder{Calc: f.Calc, Column: f.Column, Var: f.Var, Choices: map[string]string{}}
		for k, v := range f.Choices {
			tf.Choices[k] = v
		}
		ci := t.ColumnIndex(f.Column)
		switch {
		case f.Calc == "" || f.Var == "":
			fail("finder needs calc and var")
		case ci < 0:
			fail("finder: column %q is not a column of the table", f.Column)
		case !t.Columns[ci].Numeric():
			fail("finder: column %q is a text column", f.Column)
		}
		t.Finder = tf
	}

	if len(l.errs) > before {
		return nil
	}
	return t
}

// columns validates the columns of a table.
func (l *loader) columns(fail func(string, ...any), text func(string, Text, []string), raws []rawColumn) []Column {
	if len(raws) == 0 {
		fail("columns are required")
		return nil
	}
	seen := map[string]bool{}
	out := make([]Column, 0, len(raws))
	for _, rc := range raws {
		col := Column{ID: rc.ID, Name: rc.Name, Dim: rc.Dim, Unit: rc.Unit, Units: rc.Units, Label: rc.Label}
		what := "column " + col.ID
		switch {
		case !idRe.MatchString(col.ID):
			fail("column id %q must match %s", col.ID, idRe)
		case seen[col.ID]:
			fail("column %q is listed twice", col.ID)
		}
		seen[col.ID] = true
		text(what+": name", col.Name, docLocales)
		if col.Dim == textDim {
			if col.Unit != "" || len(col.Units) > 0 {
				fail("%s: a text column has no unit", what)
			}
		} else if col.dim = l.units.Dim(col.Dim); col.dim == nil {
			fail("%s: unknown dimension %q (a dimension of units.yaml, or %s)", what, col.Dim, textDim)
		} else {
			if col.Unit != "" {
				switch un := col.dim.Unit(col.Unit); {
				case un == nil:
					fail("%s: unit %q is not an explicit unit of %s", what, col.Unit, col.Dim)
				case un.Radix != 0:
					fail("%s: the number system %q cannot be the unit of table cells", what, col.Unit)
				default:
					col.unit = un
				}
			}
			checkDisplayUnits(fail, what, col.dim, col.Units, col.Unit)
		}
		out = append(out, col)
	}
	return out
}

// rows decodes the cells of a transcribed table: a numeric cell is a finite number in the unit of its column (converted
// to SI here) or null, a text cell a string or a mapping of languages.
func (l *loader) rows(fail func(string, ...any), text func(string, Text, []string), t *Table, n *yaml.Node) [][]Cell {
	switch {
	case n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null"):
		fail("rows are required")
		return nil
	case n.Kind != yaml.SequenceNode:
		fail("rows must be a list (line %d)", n.Line)
		return nil
	case len(n.Content) == 0:
		fail("rows must not be empty")
		return nil
	}
	out := make([][]Cell, 0, len(n.Content))
	for ri, rn := range n.Content {
		switch {
		case rn.Kind != yaml.SequenceNode:
			fail("row %d must be a list of cells (line %d)", ri+1, rn.Line)
			continue
		case len(rn.Content) != len(t.Columns):
			fail("row %d has %d cells, the table has %d columns (line %d)", ri+1, len(rn.Content), len(t.Columns), rn.Line)
			continue
		}
		row := make([]Cell, len(t.Columns))
		for ci, cn := range rn.Content {
			col := &t.Columns[ci]
			row[ci] = readCell(fail, text, fmt.Sprintf("row %d, column %s", ri+1, col.ID), col, cn)
		}
		out = append(out, row)
	}
	return out
}

func nullCell() Cell { return Cell{Num: math.NaN(), Null: true} }

func readCell(fail func(string, ...any), text func(string, Text, []string), where string, col *Column, n *yaml.Node) Cell {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nullCell()
	}
	if !col.Numeric() {
		switch {
		case n.Kind == yaml.ScalarNode && n.Tag == "!!str":
			return Cell{Num: math.NaN(), Text: Text{"en": n.Value}}
		case n.Kind == yaml.MappingNode:
			var t Text
			if err := decodeNode(n, &t); err != nil {
				fail("%s: %v", where, err)
				return nullCell()
			}
			text(where, t, []string{"en"})
			return Cell{Num: math.NaN(), Text: t}
		}
		fail("%s: a text column needs a string (quote numbers) or a mapping of languages, or null (line %d)", where, n.Line)
		return nullCell()
	}
	if n.Kind != yaml.ScalarNode || (n.Tag != "!!int" && n.Tag != "!!float") {
		fail("%s: a number or null is required (line %d)", where, n.Line)
		return nullCell()
	}
	var x float64
	if err := n.Decode(&x); err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
		fail("%s: %q is not a finite number (line %d)", where, n.Value, n.Line)
		return nullCell()
	}
	if col.unit != nil {
		x = col.ToSI(x)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			fail("%s: %q cannot be converted from %s to SI (line %d)", where, n.Value, col.Unit, n.Line)
			return nullCell()
		}
	}
	return Cell{Num: x}
}

// validateKey sets the key column (the first one unless key names another) and checks it: numeric with unique filled
// values, or text with unique values.
func (t *Table) validateKey(fail func(string, ...any), key string) {
	t.keyIdx = 0
	if key != "" {
		if i := t.ColumnIndex(key); i >= 0 {
			t.keyIdx = i
		} else {
			fail("key %q is not a column of the table", key)
		}
	}
	col := &t.Columns[t.keyIdx]
	t.Key = col.ID
	for r := range t.Rows {
		cell := t.Rows[r][t.keyIdx]
		if cell.Null {
			fail("row %d: the key column %s must not be empty", r+1, col.ID)
			continue
		}
		for q := 0; q < r; q++ {
			prev := t.Rows[q][t.keyIdx]
			if prev.Null {
				continue
			}
			same := prev.Text.in("en") == cell.Text.in("en")
			if col.Numeric() {
				same = closeEnough(prev.Num, cell.Num)
			}
			if same {
				fail("rows %d and %d have the same key in column %s", q+1, r+1, col.ID)
			}
		}
	}
}

// Table returns the table item with the given id, or nil.
func (c *Catalog) Table(id string) *Table { return c.tableByID[id] }

// ---- catalog JSON -------------------------------------------------------------------------------------------------

type columnJSON struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Dim   string   `json:"dim"`
	Unit  string   `json:"unit"`
	Units []string `json:"units"`
	Label string   `json:"label"`
}

type tableFinderJSON struct {
	Calc    string            `json:"calc"`
	Column  string            `json:"column"`
	Var     string            `json:"var"`
	Choices map[string]string `json:"choices"`
}

// tableJSON is a table item of the catalog JSON. rows holds, per cell, an SI number, null or a localized string.
type tableJSON struct {
	Kind     string           `json:"kind"`
	ID       string           `json:"id"`
	Category string           `json:"category"`
	Group    string           `json:"group"`
	Name     string           `json:"name"`
	NameEN   string           `json:"name_en"`
	Doc      string           `json:"doc"`
	Notes    string           `json:"notes"`
	Tags     []string         `json:"tags"`
	Source   string           `json:"source"`
	Guide    bool             `json:"guide"`
	Mains    bool             `json:"mains"`
	Key      string           `json:"key"`
	Columns  []columnJSON     `json:"columns"`
	Rows     [][]any          `json:"rows"`
	Finder   *tableFinderJSON `json:"finder"`
	// Generate is the generator of a formula-derived table, "" for a transcribed one: the page shows the cells of a
	// transcribed table with the digits they were written with.
	Generate string `json:"generate"`
}

func (t *Table) json(lang string) tableJSON {
	out := tableJSON{
		Kind: t.Kind, ID: t.ID, Category: t.Category, Group: t.Group, Name: t.Name.in(lang), NameEN: t.Name.in("en"),
		Doc: t.Doc.in(lang), Notes: t.Notes.in(lang), Tags: orEmpty(t.Tags), Source: t.Source, Guide: t.Guide, Mains: t.Mains,
		Key: t.Key, Columns: make([]columnJSON, 0, len(t.Columns)), Rows: make([][]any, 0, len(t.Rows)), Generate: t.Generate,
	}
	for _, c := range t.Columns {
		out.Columns = append(out.Columns, columnJSON{ID: c.ID, Name: c.Name.in(lang), Dim: c.Dim, Unit: c.Unit, Units: orEmpty(c.Units), Label: c.Label})
	}
	for _, row := range t.Rows {
		cells := make([]any, len(row))
		for i, cell := range row {
			switch {
			case cell.Null:
				cells[i] = nil
			case t.Columns[i].Numeric():
				cells[i] = cell.Num
			default:
				cells[i] = cell.Text.in(lang)
			}
		}
		out.Rows = append(out.Rows, cells)
	}
	if f := t.Finder; f != nil {
		choices := map[string]string{}
		for k, v := range f.Choices {
			choices[k] = v
		}
		out.Finder = &tableFinderJSON{Calc: f.Calc, Column: f.Column, Var: f.Var, Choices: choices}
	}
	return out
}
