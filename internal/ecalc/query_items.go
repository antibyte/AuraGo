package ecalc

import (
	"fmt"
	"sort"
	"strings"
)

// The query API of the items that are not calculators, next to the calculator queries of query.go: Search, SearchAll and
// Overview list items of all three kinds (searchItems), ResolveItem finds an item of any kind, and DescribeTable
// describes a table. The rows of a table are in query_table.go, the component codes in query_codes.go. Output texts are
// English; the structs carry JSON tags because the tool sends them as they are.

// The cautions the tables carry: guide marks values transcribed from a standard, mains marks installation data.
const (
	guideCaution = "Guide values from the cited source; the standard itself is authoritative."
	mainsCaution = "Mains installation data: the applicable national rules decide, and installation work is for qualified persons."
)

// ---- items of every kind ----------------------------------------------------------------------------------------

// searchItem is what the search needs to know about an item of any kind. fallback is the summary of an item without a
// doc text.
type searchItem struct {
	id, kind, category string
	name, doc          Text
	tags               []string
	fallback           string
}

func (it searchItem) summary() string {
	if s := firstSentence(it.doc.in("en")); s != "" {
		return s
	}
	return firstSentence(it.fallback)
}

// searchItems lists every item in catalog order: the calculators, then the tables, then the codes.
func (c *Catalog) searchItems() []searchItem {
	items := make([]searchItem, 0, c.ItemCount())
	for _, calc := range c.Calculators {
		items = append(items, searchItem{id: calc.ID, kind: calc.Kind, category: calc.Category, name: calc.Name, doc: calc.Doc, tags: calc.Tags})
	}
	for _, t := range c.Tables {
		names := make([]string, len(t.Columns))
		for i, col := range t.Columns {
			names[i] = col.Name.in("en")
		}
		items = append(items, searchItem{
			id: t.ID, kind: "table", category: t.Category, name: t.Name, doc: t.Doc, tags: t.Tags,
			fallback: fmt.Sprintf("Table with %d rows; columns: %s.", len(t.Rows), strings.Join(names, ", ")),
		})
	}
	for _, k := range c.Codes {
		items = append(items, searchItem{
			id: k.ID, kind: "code", category: k.Category, name: k.Name, doc: k.Doc, tags: k.Tags,
			fallback: fmt.Sprintf("Component code of kind %s.", k.CodeKind),
		})
	}
	return items
}

// ItemCount is the number of items of all kinds.
func (c *Catalog) ItemCount() int { return len(c.Calculators) + len(c.Tables) + len(c.Codes) }

// ---- describe -------------------------------------------------------------------------------------------------

// TableColumnInfo describes one column of a table. Unit is the symbol of the unit the cells are shown in; it is empty
// for text, for plain numbers and for a column whose cells carry their own prefix (4.7 kΩ, 250 µm). Units, given by
// DescribeTable, are the units a column may be shown in.
type TableColumnInfo struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Dim   string   `json:"dim"`
	Unit  string   `json:"unit,omitempty"`
	Units []string `json:"units,omitempty"`
	Key   bool     `json:"key,omitempty"`
}

// TableFinderInfo is the calculator that selects a row of the table: its variable Var, solved with the fixed Choices,
// is looked up in Column.
type TableFinderInfo struct {
	Calculator string       `json:"calculator"`
	Column     string       `json:"column"`
	Var        string       `json:"var"`
	Choices    []NamedValue `json:"choices,omitempty"`
}

// TableDescription is what DescribeTable returns.
type TableDescription struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Category string            `json:"category"`
	Group    string            `json:"group"`
	Name     string            `json:"name"`
	Doc      string            `json:"doc,omitempty"`
	Notes    string            `json:"notes,omitempty"`
	Source   string            `json:"source"`
	Guide    bool              `json:"guide,omitempty"`
	Mains    bool              `json:"mains,omitempty"`
	Caution  string            `json:"caution,omitempty"`
	Key      string            `json:"key"`
	RowCount int               `json:"row_count"`
	Columns  []TableColumnInfo `json:"columns"`
	Finder   *TableFinderInfo  `json:"finder,omitempty"`
}

func (t *Table) caution() string {
	var parts []string
	if t.Guide {
		parts = append(parts, guideCaution)
	}
	if t.Mains {
		parts = append(parts, mainsCaution)
	}
	return strings.Join(parts, " ")
}

// columnInfo lists the columns of a table; withUnits adds the display units each column offers.
func (c *Catalog) columnInfo(t *Table, withUnits bool) []TableColumnInfo {
	out := make([]TableColumnInfo, len(t.Columns))
	for i := range t.Columns {
		col := &t.Columns[i]
		info := TableColumnInfo{ID: col.ID, Name: col.Name.in("en"), Dim: col.Dim, Key: i == t.keyIdx}
		if col.Unit != "" {
			info.Unit = c.unitSymbol(col.Dim, col.Unit)
		}
		if withUnits {
			info.Units = c.unitSymbols(&Var{Dim: col.Dim, Units: col.Units})
		}
		out[i] = info
	}
	return out
}

// lookupTable finds a table by id (case-insensitive). The error of an unknown id suggests the best search hits of the
// tables; the id of a calculator or a code says what it is.
func (c *Catalog) lookupTable(id string) (*Table, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return nil, queryErr(CodeUnknownID, "id is required; use search to find a table id")
	}
	if t := c.Table(key); t != nil {
		return t, nil
	}
	return nil, c.notFound(id, key, "table")
}

// DescribeTable returns the structure of a table: columns with units, the key column, the number of rows, the notes,
// the cautions of guide and mains tables and the finder calculator.
func (c *Catalog) DescribeTable(id string) (TableDescription, error) {
	t, err := c.lookupTable(id)
	if err != nil {
		return TableDescription{}, err
	}
	d := TableDescription{
		ID: t.ID, Kind: "table", Category: t.Category, Group: t.Group, Name: t.Name.in("en"), Doc: t.Doc.in("en"),
		Notes: t.Notes.in("en"), Source: t.Source, Guide: t.Guide, Mains: t.Mains, Caution: t.caution(), Key: t.Key,
		RowCount: len(t.Rows), Columns: c.columnInfo(t, true),
	}
	if f := t.Finder; f != nil {
		info := &TableFinderInfo{Calculator: f.Calc, Column: f.Column, Var: f.Var}
		names := make([]string, 0, len(f.Choices))
		for name := range f.Choices {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			info.Choices = append(info.Choices, NamedValue{Name: name, Value: f.Choices[name]})
		}
		d.Finder = info
	}
	return d, nil
}
