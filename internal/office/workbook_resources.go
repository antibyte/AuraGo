package office

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const resourceValidation = "SHEET_DATA_VALIDATION_PLUGIN"
const resourceCondition = "SHEET_CONDITIONAL_FORMATTING_PLUGIN"
const resourceNotes = "SHEET_NOTE_PLUGIN"
const resourceNames = "SHEET_DEFINED_NAME_PLUGIN"
const resourceFilter = "SHEET_FILTER_PLUGIN"

func workbookResource(book EditorWorkbook, name string) json.RawMessage {
	for _, r := range book.Resources {
		if r.Name == name && r.Data != "" {
			return json.RawMessage(r.Data)
		}
	}
	return json.RawMessage("{}")
}
func resourceChanged(before, after EditorWorkbook, name string) bool {
	var a, b interface{}
	_ = json.Unmarshal(workbookResource(before, name), &a)
	_ = json.Unmarshal(workbookResource(after, name), &b)
	return !reflect.DeepEqual(a, b)
}
func addWorkbookResource(book *EditorWorkbook, name string, value interface{}) {
	book.Resources = append(book.Resources, EditorResource{Name: name, Data: string(jsonBytes(value))})
}
func resourceSheet[T any](book EditorWorkbook, name, id string) (T, error) {
	var zero T
	var all map[string]json.RawMessage
	if err := json.Unmarshal(workbookResource(book, name), &all); err != nil {
		return zero, fmt.Errorf("Invalid %s resource: %w", name, err)
	}
	raw := all[id]
	if len(raw) == 0 {
		return zero, nil
	}
	err := json.Unmarshal(raw, &zero)
	return zero, err
}
func editorRangeRef(r EditorRange) (string, error) {
	if r.StartRow < 0 || r.StartColumn < 0 || r.EndRow < r.StartRow || r.EndColumn < r.StartColumn || r.EndRow >= 1048576 || r.EndColumn >= 16384 {
		return "", fmt.Errorf("Invalid resource range")
	}
	a, _ := excelize.CoordinatesToCellName(r.StartColumn+1, r.StartRow+1)
	b, _ := excelize.CoordinatesToCellName(r.EndColumn+1, r.EndRow+1)
	return a + ":" + b, nil
}
func parseEditorRanges(ref string) []EditorRange {
	result := []EditorRange{}
	for _, part := range strings.Fields(strings.ReplaceAll(ref, "$", "")) {
		axes := strings.Split(part, ":")
		if len(axes) == 1 {
			axes = append(axes, axes[0])
		}
		if len(axes) != 2 {
			continue
		}
		x, y, e1 := excelize.CellNameToCoordinates(axes[0])
		xx, yy, e2 := excelize.CellNameToCoordinates(axes[1])
		if e1 == nil && e2 == nil {
			result = append(result, EditorRange{y - 1, yy - 1, x - 1, xx - 1})
		}
	}
	return result
}

type editorValidation struct {
	UID          string        `json:"uid"`
	Ranges       []EditorRange `json:"ranges"`
	Type         string        `json:"type"`
	Operator     string        `json:"operator"`
	Formula1     string        `json:"formula1"`
	Formula2     string        `json:"formula2"`
	AllowBlank   bool          `json:"allowBlank"`
	ShowDropDown bool          `json:"showDropDown"`
	ErrorStyle   int           `json:"errorStyle"`
	Error        string        `json:"error"`
	ErrorTitle   string        `json:"errorTitle"`
	Prompt       string        `json:"prompt"`
	PromptTitle  string        `json:"promptTitle"`
}
type editorCondition struct {
	ID     string        `json:"cfId"`
	Ranges []EditorRange `json:"ranges"`
	Stop   bool          `json:"stopIfTrue"`
	Rule   struct {
		Type     string          `json:"type"`
		SubType  string          `json:"subType"`
		Operator string          `json:"operator"`
		Value    interface{}     `json:"value"`
		Style    json.RawMessage `json:"style"`
	} `json:"rule"`
}
type editorNote struct {
	Note   string `json:"note"`
	ID     string `json:"id"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Width  uint   `json:"width"`
	Height uint   `json:"height"`
	Show   bool   `json:"show"`
	Author string `json:"author,omitempty"`
}
type editorDefinedName struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Ref     string `json:"formulaOrRefString"`
	Scope   string `json:"localSheetId"`
	Comment string `json:"comment,omitempty"`
}
type editorFilterColumn struct {
	ID      int `json:"colId" xml:"colId,attr"`
	Filters *struct {
		Values []string `json:"filters" xml:"filter>val"`
		Blank  bool     `json:"blank,omitempty" xml:"blank,attr,omitempty"`
	} `json:"filters,omitempty" xml:"-"`
	Custom *struct {
		And     bool `json:"and,omitempty"`
		Filters []struct {
			Operator string      `json:"operator"`
			Value    interface{} `json:"val"`
		} `json:"customFilters"`
	} `json:"customFilters,omitempty" xml:"-"`
}
type editorFilter struct {
	Ref     EditorRange          `json:"ref"`
	Columns []editorFilterColumn `json:"filterColumns"`
	Hidden  []int                `json:"cachedFilteredOut"`
}

func readEditorResources(f *excelize.File, parts map[string][]byte, doc *WorkbookEditorDocument) error {
	validations := map[string][]editorValidation{}
	conditions := map[string][]editorCondition{}
	notes := map[string]map[int]map[int]editorNote{}
	filters := map[string]editorFilter{}
	names := map[string]editorDefinedName{}
	for id, s := range doc.Workbook.Sheets {
		rules, err := f.GetDataValidations(s.Name)
		if err != nil {
			return err
		}
		for i, r := range rules {
			v := editorValidation{UID: fmt.Sprintf("dv-%s-%d", id, i), Ranges: parseEditorRanges(r.Sqref), Type: r.Type, Operator: r.Operator, Formula1: r.Formula1, Formula2: r.Formula2, AllowBlank: r.AllowBlank, ShowDropDown: !r.ShowDropDown, ErrorStyle: 1}
			if r.Type == "decimal" {
				v.Type = "decimal"
			}
			if r.Type == "list" && strings.HasPrefix(r.Formula1, "\"") {
				v.Formula1 = string(jsonBytes(strings.Split(strings.Trim(r.Formula1, "\""), ",")))
			}
			if r.ErrorStyle != nil && *r.ErrorStyle != "stop" {
				v.ErrorStyle = 2
			}
			if r.Error != nil {
				v.Error = *r.Error
			}
			if r.ErrorTitle != nil {
				v.ErrorTitle = *r.ErrorTitle
			}
			if r.Prompt != nil {
				v.Prompt = *r.Prompt
			}
			if r.PromptTitle != nil {
				v.PromptTitle = *r.PromptTitle
			}
			validations[id] = append(validations[id], v)
		}
		cf, err := f.GetConditionalFormats(s.Name)
		if err != nil {
			return err
		}
		for ref, rules := range cf {
			for i, r := range rules {
				c := editorCondition{ID: fmt.Sprintf("cf-%s-%s-%d", id, ref, i), Ranges: parseEditorRanges(ref), Stop: r.StopIfTrue}
				c.Rule.Type = "highlightCell"
				c.Rule.Operator = cfOperatorToEditor(r.Criteria)
				c.Rule.Value = r.Value
				switch r.Type {
				case "cell":
					c.Rule.SubType = "number"
					if n, e := strconv.ParseFloat(r.Value, 64); e == nil {
						c.Rule.Value = n
					}
				case "text":
					c.Rule.SubType = "text"
					c.Rule.Operator = "containsText"
				case "formula":
					c.Rule.Type = "formula"
					c.Rule.Value = r.Value
				default:
					doc.Limitations = append(doc.Limitations, "conditions_preserved")
					continue
				}
				if r.Format != nil {
					style, e := f.GetConditionalStyle(*r.Format)
					if e != nil {
						return e
					}
					c.Rule.Style = decodeEditorStyle(style)
				}
				conditions[id] = append(conditions[id], c)
			}
		}
		comments, err := f.GetComments(s.Name)
		if err != nil {
			return err
		}
		for _, c := range comments {
			x, y, e := excelize.CellNameToCoordinates(c.Cell)
			if e != nil {
				return e
			}
			if notes[id] == nil {
				notes[id] = map[int]map[int]editorNote{}
			}
			if notes[id][y-1] == nil {
				notes[id][y-1] = map[int]editorNote{}
			}
			text := c.Text
			if text == "" {
				for _, p := range c.Paragraph {
					text += p.Text
				}
			}
			notes[id][y-1][x-1] = editorNote{Note: text, ID: "note-" + c.Cell, Row: y - 1, Col: x - 1, Width: max(220, c.Width), Height: max(140, c.Height), Author: c.Author}
		}
		if filter, ok := readEditorFilter(parts[sheetPartByName(parts, s.Name)]); ok {
			filters[id] = filter
		}
	}
	for i, n := range f.GetDefinedName() {
		if strings.HasPrefix(n.Name, "_xlnm.") {
			continue
		}
		scope := "AllDefaultWorkbook"
		for id, s := range doc.Workbook.Sheets {
			if s.Name == n.Scope {
				scope = id
			}
		}
		id := fmt.Sprintf("name-%d", i)
		names[id] = editorDefinedName{ID: id, Name: n.Name, Ref: n.RefersTo, Scope: scope, Comment: n.Comment}
	}
	addWorkbookResource(&doc.Workbook, resourceValidation, validations)
	addWorkbookResource(&doc.Workbook, resourceCondition, conditions)
	addWorkbookResource(&doc.Workbook, resourceNotes, notes)
	addWorkbookResource(&doc.Workbook, resourceNames, names)
	addWorkbookResource(&doc.Workbook, resourceFilter, filters)
	return nil
}
func cfOperatorToEditor(value string) string {
	if v := map[string]string{">": "greaterThan", "<": "lessThan", "==": "equal", "!=": "notEqual", ">=": "greaterThanOrEqual", "<=": "lessThanOrEqual", "containing": "containsText"}[value]; v != "" {
		return v
	}
	return value
}
func cfOperatorToExcel(value string) string {
	if v := map[string]string{"greaterThan": ">", "lessThan": "<", "equal": "==", "notEqual": "!=", "greaterThanOrEqual": ">=", "lessThanOrEqual": "<=", "containsText": "containing"}[value]; v != "" {
		return v
	}
	return value
}

// Only changed resources are rewritten; unsupported original rules remain opaque.
func applyEditorResources(f *excelize.File, before, after EditorWorkbook) error {
	for id, s := range after.Sheets {
		if resourceChanged(before, after, resourceValidation) {
			prev, err := resourceSheet[[]editorValidation](before, resourceValidation, id)
			if err != nil {
				return err
			}
			next, err := resourceSheet[[]editorValidation](after, resourceValidation, id)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(prev, next) {
				if len(next) > 10000 {
					return fmt.Errorf("Too many validation rules")
				}
				for _, v := range prev {
					for _, r := range v.Ranges {
						ref, err := editorRangeRef(r)
						if err != nil {
							return err
						}
						if err = f.DeleteDataValidation(s.Name, ref); err != nil {
							return err
						}
					}
				}
				for _, v := range next {
					if v.Type != "list" && v.Type != "decimal" && v.Type != "whole" && v.Type != "date" && v.Type != "time" && v.Type != "textLength" && v.Type != "custom" {
						return fmt.Errorf("Unsupported validation type")
					}
					refs := []string{}
					for _, r := range v.Ranges {
						ref, err := editorRangeRef(r)
						if err != nil {
							return err
						}
						refs = append(refs, ref)
					}
					if len(refs) == 0 || len(refs) > 1000 {
						return fmt.Errorf("Invalid validation ranges")
					}
					r := excelize.NewDataValidation(v.AllowBlank)
					r.Type = v.Type
					r.Operator = v.Operator
					r.Sqref = strings.Join(refs, " ")
					r.Formula1 = v.Formula1
					r.Formula2 = v.Formula2
					r.ShowDropDown = !v.ShowDropDown
					if v.Type == "list" && strings.HasPrefix(v.Formula1, "[") {
						var list []string
						if err = json.Unmarshal([]byte(v.Formula1), &list); err != nil {
							return err
						}
						if err = r.SetDropList(list); err != nil {
							return err
						}
					}
					style := excelize.DataValidationErrorStyleStop
					if v.ErrorStyle != 1 {
						style = excelize.DataValidationErrorStyleWarning
					}
					r.SetError(style, v.ErrorTitle, v.Error)
					r.SetInput(v.PromptTitle, v.Prompt)
					if err = f.AddDataValidation(s.Name, r); err != nil {
						return err
					}
				}
			}
		}
		if resourceChanged(before, after, resourceCondition) {
			prev, err := resourceSheet[[]editorCondition](before, resourceCondition, id)
			if err != nil {
				return err
			}
			next, err := resourceSheet[[]editorCondition](after, resourceCondition, id)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(prev, next) {
				if len(next) > 10000 {
					return fmt.Errorf("Too many conditional rules")
				}
				for _, v := range prev {
					for _, r := range v.Ranges {
						ref, e := editorRangeRef(r)
						if e != nil {
							return e
						}
						if e = f.UnsetConditionalFormat(s.Name, ref); e != nil {
							return e
						}
					}
				}
				grouped := map[string][]excelize.ConditionalFormatOptions{}
				for _, v := range next {
					opt := excelize.ConditionalFormatOptions{Criteria: cfOperatorToExcel(v.Rule.Operator), Value: fmt.Sprint(v.Rule.Value), StopIfTrue: v.Stop}
					switch {
					case v.Rule.Type == "formula":
						opt.Type = "formula"
						opt.Criteria = "="
					case v.Rule.Type == "highlightCell" && v.Rule.SubType == "number":
						opt.Type = "cell"
					case v.Rule.Type == "highlightCell" && v.Rule.SubType == "text":
						opt.Type = "text"
					default:
						return fmt.Errorf("Unsupported conditional rule; preserve it unchanged")
					}
					style, e := encodeEditorStyle(v.Rule.Style)
					if e != nil {
						return e
					}
					styleID, e := f.NewConditionalStyle(style)
					if e != nil {
						return e
					}
					opt.Format = &styleID
					for _, r := range v.Ranges {
						ref, e := editorRangeRef(r)
						if e != nil {
							return e
						}
						grouped[ref] = append(grouped[ref], opt)
					}
				}
				for ref, opts := range grouped {
					if err = f.SetConditionalFormat(s.Name, ref, opts); err != nil {
						return err
					}
				}
			}
		}
		if resourceChanged(before, after, resourceNotes) {
			prev, err := resourceSheet[map[int]map[int]editorNote](before, resourceNotes, id)
			if err != nil {
				return err
			}
			next, err := resourceSheet[map[int]map[int]editorNote](after, resourceNotes, id)
			if err != nil {
				return err
			}
			for row, cols := range prev {
				for col, n := range cols {
					if reflect.DeepEqual(n, next[row][col]) {
						continue
					}
					axis, e := excelize.CoordinatesToCellName(col+1, row+1)
					if e != nil {
						return e
					}
					if e = f.DeleteComment(s.Name, axis); e != nil {
						return e
					}
				}
			}
			count := 0
			for row, cols := range next {
				for col, n := range cols {
					count++
					if count > 10000 || len(n.Note) > 32767 || row < 0 || col < 0 {
						return fmt.Errorf("Invalid cell note")
					}
					if reflect.DeepEqual(n, prev[row][col]) {
						continue
					}
					axis, e := excelize.CoordinatesToCellName(col+1, row+1)
					if e != nil {
						return e
					}
					author := n.Author
					if author == "" {
						author = "AuraGo"
					}
					if e = f.AddComment(s.Name, excelize.Comment{Cell: axis, Author: author, Text: n.Note, Width: min(1000, max(120, n.Width)), Height: min(1000, max(80, n.Height))}); e != nil {
						return e
					}
				}
			}
		}
	}
	if resourceChanged(before, after, resourceNames) {
		var prev, next map[string]editorDefinedName
		if err := json.Unmarshal(workbookResource(before, resourceNames), &prev); err != nil {
			return err
		}
		if err := json.Unmarshal(workbookResource(after, resourceNames), &next); err != nil {
			return err
		}
		scope := func(name editorDefinedName, book EditorWorkbook) string {
			if s := book.Sheets[name.Scope]; s != nil {
				return s.Name
			}
			return "Workbook"
		}
		for _, n := range prev {
			if err := f.DeleteDefinedName(&excelize.DefinedName{Name: n.Name, Scope: scope(n, before)}); err != nil {
				return err
			}
		}
		if len(next) > 10000 {
			return fmt.Errorf("Too many named ranges")
		}
		for _, n := range next {
			if len(n.Ref) > 8192 || len(n.Name) > 255 {
				return fmt.Errorf("Invalid named range")
			}
			if err := f.SetDefinedName(&excelize.DefinedName{Name: n.Name, RefersTo: n.Ref, Scope: scope(n, after), Comment: n.Comment}); err != nil {
				return err
			}
		}
	}
	return nil
}
func readEditorFilter(data []byte) (editorFilter, bool) {
	var raw struct {
		Filter *struct {
			Ref     string `xml:"ref,attr"`
			Columns []struct {
				ID      int `xml:"colId,attr"`
				Filters *struct {
					Blank  bool `xml:"blank,attr"`
					Values []struct {
						Value string `xml:"val,attr"`
					} `xml:"filter"`
				} `xml:"filters"`
				Custom *struct {
					And     bool `xml:"and,attr"`
					Filters []struct {
						Op    string `xml:"operator,attr"`
						Value string `xml:"val,attr"`
					} `xml:"customFilter"`
				} `xml:"customFilters"`
			} `xml:"filterColumn"`
		} `xml:"autoFilter"`
	}
	if xml.Unmarshal(data, &raw) != nil || raw.Filter == nil {
		return editorFilter{}, false
	}
	ranges := parseEditorRanges(raw.Filter.Ref)
	if len(ranges) != 1 {
		return editorFilter{}, false
	}
	result := editorFilter{Ref: ranges[0], Columns: []editorFilterColumn{}, Hidden: []int{}}
	for _, v := range raw.Filter.Columns {
		c := editorFilterColumn{ID: v.ID}
		m := map[string]interface{}{"colId": v.ID}
		if v.Filters != nil {
			values := []string{}
			for _, x := range v.Filters.Values {
				values = append(values, x.Value)
			}
			m["filters"] = map[string]interface{}{"filters": values, "blank": v.Filters.Blank}
		}
		if v.Custom != nil {
			values := []interface{}{}
			for _, x := range v.Custom.Filters {
				values = append(values, map[string]interface{}{"operator": x.Op, "val": x.Value})
			}
			m["customFilters"] = map[string]interface{}{"and": v.Custom.And, "customFilters": values}
		}
		_ = json.Unmarshal(jsonBytes(m), &c)
		result.Columns = append(result.Columns, c)
	}
	return result, true
}
func xmlText(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}
func editorFilterXML(filter editorFilter) ([]byte, error) {
	ref, err := editorRangeRef(filter.Ref)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("<autoFilter ref=\"" + ref + "\">")
	if len(filter.Columns) > 16384 {
		return nil, fmt.Errorf("Too many filter columns")
	}
	for _, c := range filter.Columns {
		if c.ID < 0 || c.ID > filter.Ref.EndColumn-filter.Ref.StartColumn {
			return nil, fmt.Errorf("Invalid filter column")
		}
		fmt.Fprintf(&b, "<filterColumn colId=\"%d\">", c.ID)
		if c.Filters != nil {
			fmt.Fprintf(&b, "<filters blank=\"%t\">", c.Filters.Blank)
			if len(c.Filters.Values) > 10000 {
				return nil, fmt.Errorf("Too many filter values")
			}
			for _, v := range c.Filters.Values {
				b.WriteString("<filter val=\"" + xmlText(v) + "\"/>")
			}
			b.WriteString("</filters>")
		}
		if c.Custom != nil {
			if len(c.Custom.Filters) > 2 {
				return nil, fmt.Errorf("Too many filter conditions")
			}
			fmt.Fprintf(&b, "<customFilters and=\"%t\">", c.Custom.And)
			for _, v := range c.Custom.Filters {
				op := v.Operator
				if !strings.Contains("|equal|notEqual|lessThan|lessThanOrEqual|greaterThan|greaterThanOrEqual|", "|"+op+"|") {
					return nil, fmt.Errorf("Invalid filter operator")
				}
				b.WriteString("<customFilter operator=\"" + op + "\" val=\"" + xmlText(fmt.Sprint(v.Value)) + "\"/>")
			}
			b.WriteString("</customFilters>")
		}
		b.WriteString("</filterColumn>")
	}
	b.WriteString("</autoFilter>")
	return []byte(b.String()), nil
}
func applyEditorFilterParts(parts map[string][]byte, before, after EditorWorkbook) error {
	if !resourceChanged(before, after, resourceFilter) {
		return nil
	}
	for id, s := range after.Sheets {
		old, err := resourceSheet[*editorFilter](before, resourceFilter, id)
		if err != nil {
			return err
		}
		next, err := resourceSheet[*editorFilter](after, resourceFilter, id)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(old, next) {
			continue
		}
		var value []byte
		if next != nil {
			value, err = editorFilterXML(*next)
			if err != nil {
				return err
			}
		}
		key := sheetPartByName(parts, s.Name)
		updated := []byte("<worksheet>" + string(value) + "</worksheet>")
		parts[key], err = mergeOfficeXML(parts[key], updated, map[string]bool{"autoFilter": true})
		if err != nil {
			return err
		}
	}
	return nil
}

// WorkbookExtensionMatches prevents dropping the macro-enabled package type on save-as.
func WorkbookExtensionMatches(data []byte, extension string) bool {
	if len(data) == 0 {
		return strings.EqualFold(extension, ".xlsx")
	}
	parts, err := readOfficeParts(data, "xl/workbook.xml")
	if err != nil {
		return false
	}
	macro := bytes.Contains(parts["[Content_Types].xml"], []byte("macroEnabled"))
	return macro && strings.EqualFold(extension, ".xlsm") || !macro && strings.EqualFold(extension, ".xlsx")
}

type editorPrint struct {
	Sheet       string `json:"sheet"`
	Area        string `json:"area"`
	Orientation string `json:"orientation"`
	Paper       string `json:"paper"`
	Scaling     string `json:"scaling"`
	Repeat      int    `json:"repeatRows"`
}

func applyEditorPrint(f *excelize.File, before, after EditorWorkbook) error {
	if bytes.Equal(before.Custom, after.Custom) {
		return nil
	}
	var custom struct {
		Print editorPrint `json:"auragoPrint"`
	}
	if len(after.Custom) == 0 {
		return nil
	}
	if err := json.Unmarshal(after.Custom, &custom); err != nil {
		return err
	}
	p := custom.Print
	if p.Area == "" {
		return nil
	}
	sheet := after.Sheets[p.Sheet]
	if sheet == nil {
		return fmt.Errorf("Invalid print sheet")
	}
	ranges := parseEditorRanges(p.Area)
	if len(ranges) != 1 {
		return fmt.Errorf("Invalid print area")
	}
	if _, err := editorRangeRef(ranges[0]); err != nil {
		return err
	}
	if p.Orientation != "landscape" && p.Orientation != "portrait" || p.Paper != "A4" && p.Paper != "letter" || p.Repeat < 0 || p.Repeat > 100 {
		return fmt.Errorf("Invalid print settings")
	}
	size := 9
	if p.Paper == "letter" {
		size = 1
	}
	width, height := 0, 0
	if p.Scaling == "fit" {
		width = 1
	}
	adjust := uint(100)
	if err := f.SetPageLayout(sheet.Name, &excelize.PageLayoutOptions{Size: &size, Orientation: &p.Orientation, FitToWidth: &width, FitToHeight: &height, AdjustTo: &adjust}); err != nil {
		return err
	}
	for _, name := range []string{"_xlnm.Print_Area", "_xlnm.Print_Titles"} {
		for _, existing := range f.GetDefinedName() {
			if existing.Name == name && existing.Scope == sheet.Name {
				if err := f.DeleteDefinedName(&existing); err != nil {
					return err
				}
			}
		}
	}
	prefix := "'" + strings.ReplaceAll(sheet.Name, "'", "''") + "'!"
	if err := f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", Scope: sheet.Name, RefersTo: prefix + p.Area}); err != nil {
		return err
	}
	if p.Repeat > 0 {
		return f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Titles", Scope: sheet.Name, RefersTo: prefix + fmt.Sprintf("$1:$%d", p.Repeat)})
	}
	return nil
}

// Read portable Excel print settings even without AuraGo metadata.
func readEditorPrint(f *excelize.File, book *EditorWorkbook) {
	for _, id := range book.SheetOrder {
		sheet := book.Sheets[id]
		p := editorPrint{Sheet: id, Orientation: "portrait", Paper: "A4", Scaling: "actual"}
		for _, name := range f.GetDefinedName() {
			if name.Scope != sheet.Name {
				continue
			}
			ref := name.RefersTo
			if at := strings.LastIndex(ref, "!"); at >= 0 {
				ref = ref[at+1:]
			}
			ref = strings.ReplaceAll(ref, "$", "")
			if name.Name == "_xlnm.Print_Area" && len(parseEditorRanges(ref)) == 1 {
				p.Area = ref
			}
			if name.Name == "_xlnm.Print_Titles" {
				fmt.Sscanf(ref, "1:%d", &p.Repeat)
			}
		}
		if p.Area == "" {
			continue
		}
		if layout, err := f.GetPageLayout(sheet.Name); err == nil {
			if layout.Orientation != nil {
				p.Orientation = *layout.Orientation
			}
			if layout.Size != nil && *layout.Size == 1 {
				p.Paper = "letter"
			}
			if layout.FitToWidth != nil && *layout.FitToWidth == 1 {
				p.Scaling = "fit"
			}
		}
		book.Custom = jsonBytes(map[string]interface{}{"auragoPrint": p})
		return
	}
}
