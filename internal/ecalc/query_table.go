package ecalc

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// The rows of a table for the agent tool: QueryTable lists the rows that match a filter, one page of them, and the result
// of one lookup. Output texts are English; the structs carry JSON tags because the tool sends them as they are.

const (
	// DefaultTableLimit and MaxTableLimit bound the number of rows of one QueryTable answer.
	DefaultTableLimit = 20
	MaxTableLimit     = 50
	// MaxFilterLength is the longest table filter in characters (a filter is a word or a number; every row is compared
	// with it).
	MaxFilterLength = 128
)

// TableLookup is the lookup of a QueryTable request: Mode "exact" (the default) finds the row whose Column equals Value,
// "ge" the first row (table order) whose Column is at least Value, "interp" interpolates the column Output linearly
// over Column at Value. Value is written the way a person types it and read with the dimension and unit of Column (a
// bare number is in the column's unit; a text column compares text). Output names the column to return: without it
// exact and ge answer with the whole row; interp needs it. An empty Column means the key column (exact only).
type TableLookup struct {
	Column string
	Value  string
	Mode   string
	Output string
}

// TableRequest is the input of QueryTable. Filter keeps the rows with a cell that contains the text (case-insensitive,
// numbers as written in the answer); Offset and Limit page through the kept rows (Limit 0 = DefaultTableLimit, at most
// MaxTableLimit). A request with a Lookup and no filter, offset or limit lists no rows.
type TableRequest struct {
	Filter string
	Offset int
	Limit  int
	Lookup *TableLookup
}

// LookupResult is the answer to a TableLookup. Value is the lookup value as it was understood (with its unit); Row is
// the matching row (exact and ge), Result and SI the output cell.
type LookupResult struct {
	Mode    string    `json:"mode"`
	Column  string    `json:"column"`
	Value   string    `json:"value"`
	Found   bool      `json:"found"`
	Output  string    `json:"output,omitempty"`
	Result  string    `json:"result,omitempty"`
	SI      *float64  `json:"si,omitempty"`
	Row     []*string `json:"row,omitempty"`
	Message string    `json:"message,omitempty"`
}

// TableResult is what QueryTable returns. Rows hold the cells as written in the answer (a number with its unit, text,
// null for an empty cell) and SI the same cells as SI numbers (null for text and empty cells). Total counts the rows
// that match the filter; Truncated means that more of them exist than are listed, NextOffset is where to continue.
type TableResult struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Columns    []TableColumnInfo `json:"columns"`
	Total      int               `json:"total"`
	Offset     int               `json:"offset"`
	Returned   int               `json:"returned"`
	Truncated  bool              `json:"truncated,omitempty"`
	NextOffset int               `json:"next_offset,omitempty"`
	Rows       [][]*string       `json:"rows,omitempty"`
	SI         [][]*float64      `json:"si,omitempty"`
	Lookup     *LookupResult     `json:"lookup,omitempty"`
	Guide      bool              `json:"guide,omitempty"`
	Mains      bool              `json:"mains,omitempty"`
	Caution    string            `json:"caution,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	Source     string            `json:"source"`
	Note       string            `json:"note,omitempty"`
}

// Keep shortens the listed rows to the first n (the tool does that to fit its output budget) and keeps Returned,
// Truncated and NextOffset consistent.
func (r *TableResult) Keep(n int) {
	if n < 0 || n >= len(r.Rows) {
		return
	}
	r.Rows = r.Rows[:n]
	if len(r.SI) > n {
		r.SI = r.SI[:n]
	}
	r.Returned = n
	r.Truncated = true
	r.NextOffset = r.Offset + n
}

// cellOut writes one cell for an answer: its text (a number with the symbol of its column's unit, nil when empty) and,
// for a number, the SI value.
func (c *Catalog) cellOut(t *Table, r, col int) (*string, *float64) {
	cell := t.Rows[r][col]
	if cell.Null {
		return nil, nil
	}
	spec := &t.Columns[col]
	if !spec.Numeric() {
		s := cell.Text.in("en")
		return &s, nil
	}
	s := c.formatValue(cell.Num, spec.Dim, spec.Unit)
	return &s, siOut(cell.Num)
}

// siOut is an SI number of an answer, rounded to 12 significant digits so that the arithmetic of the unit conversion
// (2.5 mm2 is 2.4999999999999998e-6 m2) does not show; nil when x is not finite.
func siOut(x float64) *float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	x = roundSig(x, 12)
	return &x
}

// rowMatches reports whether a cell of row r contains needle (lower case); text cells are compared in English and German,
// numbers as they are written in the answer.
func (c *Catalog) rowMatches(t *Table, shown []*string, r int, needle string) bool {
	for col, s := range shown {
		if s == nil {
			continue
		}
		if strings.Contains(strings.ToLower(*s), needle) {
			return true
		}
		if !t.Columns[col].Numeric() {
			if de := t.Rows[r][col].Text.in("de"); strings.Contains(strings.ToLower(de), needle) {
				return true
			}
		}
	}
	return false
}

// QueryTable reads the rows of a table: the rows that match the filter, one page of them, and the result of the lookup.
func (c *Catalog) QueryTable(id string, req TableRequest) (TableResult, error) {
	t, err := c.lookupTable(id)
	if err != nil {
		return TableResult{}, err
	}
	filter := strings.TrimSpace(req.Filter)
	if utf8.RuneCountInString(filter) > MaxFilterLength {
		return TableResult{}, queryErr(CodeInvalidFilter, "filter must be at most %d characters; give a word or a number that occurs in a cell", MaxFilterLength)
	}
	res := TableResult{
		ID: t.ID, Name: t.Name.in("en"), Columns: c.columnInfo(t, false), Guide: t.Guide, Mains: t.Mains, Caution: t.caution(),
		Notes: t.Notes.in("en"), Source: t.Source, Offset: max(req.Offset, 0),
	}
	if req.Lookup != nil {
		lr, err := c.lookupInTable(t, *req.Lookup)
		if err != nil {
			return TableResult{}, err
		}
		res.Lookup = lr
	}

	shown := make([][]*string, len(t.Rows))
	si := make([][]*float64, len(t.Rows))
	for r := range t.Rows {
		shown[r], si[r] = make([]*string, len(t.Columns)), make([]*float64, len(t.Columns))
		for col := range t.Columns {
			shown[r][col], si[r][col] = c.cellOut(t, r, col)
		}
	}
	var matches []int
	needle := strings.ToLower(filter)
	for r := range t.Rows {
		if needle == "" || c.rowMatches(t, shown[r], r, needle) {
			matches = append(matches, r)
		}
	}
	res.Total = len(matches)
	if req.Lookup != nil && filter == "" && req.Offset == 0 && req.Limit == 0 {
		res.Note = "Only the lookup was answered; give filter, offset or limit to list rows."
		return res, nil
	}
	limit := req.Limit
	if limit < 1 {
		limit = DefaultTableLimit
	}
	limit = min(limit, MaxTableLimit)
	page := matches[min(res.Offset, len(matches)):]
	if len(page) > limit {
		page = page[:limit]
	}
	for _, r := range page {
		res.Rows = append(res.Rows, shown[r])
		res.SI = append(res.SI, si[r])
	}
	res.Returned = len(page)
	if end := min(res.Offset, len(matches)) + len(page); end < len(matches) {
		res.Truncated, res.NextOffset = true, end
	}
	if res.Total == 0 {
		res.Note = "No row matches the filter."
		if filter == "" {
			res.Note = "The table has no rows."
		}
	}
	return res, nil
}

// ---- lookup ---------------------------------------------------------------------------------------------------

func columnList(t *Table) string {
	return idsOf(t.Columns, func(c Column) string { return c.ID })
}

// findColumn resolves a column id (case-insensitive) of the table.
func findColumn(t *Table, name, role string) (int, *QueryError) {
	if i := t.ColumnIndex(strings.ToLower(strings.TrimSpace(name))); i >= 0 {
		return i, nil
	}
	return -1, queryErr(CodeUnknownColumn, "table %s has no column %q (%s); columns: %s", t.ID, name, role, columnList(t))
}

func (c *Catalog) lookupInTable(t *Table, lk TableLookup) (*LookupResult, error) {
	mode := strings.ToLower(strings.TrimSpace(lk.Mode))
	if mode == "" {
		mode = "exact"
	}
	if mode != "exact" && mode != "ge" && mode != "interp" {
		return nil, queryErr(CodeInvalidLookup, "lookup mode must be exact, ge or interp, got %q", lk.Mode)
	}
	value := strings.TrimSpace(lk.Value)
	if value == "" {
		return nil, queryErr(CodeInvalidLookup, "lookup needs a value")
	}
	colName := strings.TrimSpace(lk.Column)
	var col int
	if colName == "" && mode == "exact" {
		col = t.keyIdx
	} else if colName == "" {
		return nil, queryErr(CodeInvalidLookup, "lookup mode %s needs a column; columns: %s", mode, columnList(t))
	} else {
		var qe *QueryError
		if col, qe = findColumn(t, colName, "lookup column"); qe != nil {
			return nil, qe
		}
	}
	spec := &t.Columns[col]
	out := -1
	if name := strings.TrimSpace(lk.Output); name != "" {
		var qe *QueryError
		if out, qe = findColumn(t, name, "lookup output"); qe != nil {
			return nil, qe
		}
	}
	res := &LookupResult{Mode: mode, Column: spec.ID}
	if out >= 0 {
		res.Output = t.Columns[out].ID
	}

	// A text column compares text (exact only).
	if !spec.Numeric() {
		if mode != "exact" {
			return nil, queryErr(CodeInvalidLookup, "column %s holds text, so only the mode exact can search it; numeric columns: %s", spec.ID, numericColumns(t))
		}
		res.Value = value
		row := -1
		for r := range t.Rows {
			cell := t.Rows[r][col]
			if !cell.Null && (strings.EqualFold(strings.TrimSpace(cell.Text.in("en")), value) || strings.EqualFold(strings.TrimSpace(cell.Text.in("de")), value)) {
				row = r
				break
			}
		}
		c.fillLookup(res, t, row, out)
		if row < 0 {
			res.Message = fmt.Sprintf("no row has %s = %q", spec.ID, value)
		}
		return res, nil
	}

	x, err := c.Units.Parse(value, spec.Dim, spec.Unit)
	if err != nil {
		return nil, queryErr(CodeParse, "cannot read %q for column %s as a %s: %v", value, spec.ID, spec.Dim, err)
	}
	res.Value = c.formatValue(x, spec.Dim, spec.Unit)
	switch mode {
	case "exact":
		row := t.FindRow(col, x)
		c.fillLookup(res, t, row, out)
		if row < 0 {
			res.Message = fmt.Sprintf("no row has %s = %s", spec.ID, res.Value)
		}
	case "ge":
		row := -1
		largest := math.NaN()
		for r := range t.Rows {
			cell := t.Rows[r][col]
			if cell.Null {
				continue
			}
			if row < 0 && cell.Num >= x {
				row = r
			}
			if math.IsNaN(largest) || cell.Num > largest {
				largest = cell.Num
			}
		}
		c.fillLookup(res, t, row, out)
		if row < 0 {
			res.Message = fmt.Sprintf("no row has %s of at least %s", spec.ID, res.Value)
			if !math.IsNaN(largest) {
				res.Message += "; the largest is " + c.formatValue(largest, spec.Dim, spec.Unit)
			}
		}
	case "interp":
		if out < 0 {
			return nil, queryErr(CodeInvalidLookup, "lookup mode interp needs an output column; columns: %s", numericColumns(t))
		}
		if !t.Columns[out].Numeric() {
			return nil, queryErr(CodeInvalidLookup, "output column %s holds text and cannot be interpolated; numeric columns: %s", t.Columns[out].ID, numericColumns(t))
		}
		if err := t.checkInterp(col, out); err != nil {
			return nil, queryErr(CodeInvalidLookup, "%v", err)
		}
		y := t.Interp(spec.ID, t.Columns[out].ID, x)
		if math.IsNaN(y) {
			xs, _ := t.points(col, out)
			lo, hi := xs[0], xs[len(xs)-1]
			if lo > hi {
				lo, hi = hi, lo
			}
			res.Message = fmt.Sprintf("%s is outside the range of %s, %s to %s", res.Value, spec.ID, c.formatValue(lo, spec.Dim, spec.Unit), c.formatValue(hi, spec.Dim, spec.Unit))
			break
		}
		res.Found = true
		oc := &t.Columns[out]
		res.Result, res.SI = c.formatValue(y, oc.Dim, oc.Unit), siOut(y)
	}
	return res, nil
}

func numericColumns(t *Table) string {
	var ids []string
	for _, col := range t.Columns {
		if col.Numeric() {
			ids = append(ids, col.ID)
		}
	}
	return strings.Join(ids, ", ")
}

// fillLookup sets the result of an exact or ge lookup for row (-1: none): the row and, when an output column is named,
// its cell.
func (c *Catalog) fillLookup(res *LookupResult, t *Table, row, out int) {
	if row < 0 {
		return
	}
	res.Found = true
	res.Row = make([]*string, len(t.Columns))
	for col := range t.Columns {
		res.Row[col], _ = c.cellOut(t, row, col)
	}
	if out < 0 {
		return
	}
	text, si := c.cellOut(t, row, out)
	if text == nil {
		res.Message = fmt.Sprintf("the cell of %s is empty in that row", t.Columns[out].ID)
		return
	}
	res.Result, res.SI = *text, si
}
