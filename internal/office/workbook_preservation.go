package office

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Preserve cell and row extensions that Excelize cannot model. Structural edits
// are separately rejected when unknown worksheet references are present.
func preserveSheetCells(original, updated []byte, before, after EditorWorkbook, id string) ([]byte, error) {
	oldChildren, _, _, err := officeXMLChildren(original)
	if err != nil {
		return nil, err
	}
	newChildren, _, _, err := officeXMLChildren(updated)
	if err != nil {
		return nil, err
	}
	var oldData, newData officeXMLChild
	for _, c := range oldChildren {
		if c.Name == "sheetData" {
			oldData = c
		}
	}
	for _, c := range newChildren {
		if c.Name == "sheetData" {
			newData = c
		}
	}
	if oldData.End == 0 || newData.End == 0 {
		return updated, nil
	}
	old := original[oldData.Start:oldData.End]
	fresh := updated[newData.Start:newData.End]
	oldRows, _, _, err := officeXMLChildren(old)
	if err != nil {
		return nil, err
	}
	rows, head, tail, err := officeXMLChildren(fresh)
	if err != nil {
		return nil, err
	}
	originals := map[string][]byte{}
	for _, r := range oldRows {
		raw := old[r.Start:r.End]
		originals[xmlAttribute(raw, "r")] = raw
	}
	var out bytes.Buffer
	out.Write(fresh[:head])
	for _, r := range rows {
		raw := fresh[r.Start:r.End]
		previous := originals[xmlAttribute(raw, "r")]
		if len(previous) > 0 {
			oldCells, _, _, e := officeXMLChildren(previous)
			if e != nil {
				return nil, e
			}
			newCells, ch, ct, e := officeXMLChildren(raw)
			if e != nil {
				return nil, e
			}
			cells := map[string][]byte{}
			for _, c := range oldCells {
				if c.Name == "c" {
					v := previous[c.Start:c.End]
					cells[xmlAttribute(v, "r")] = v
				}
			}
			var row bytes.Buffer
			row.Write(raw[:ch])
			for _, c := range newCells {
				v := raw[c.Start:c.End]
				axis := xmlAttribute(v, "r")
				if c.Name == "c" && len(cells[axis]) > 0 {
					x, y, e := cellCoordinates(axis)
					if e != nil {
						return nil, e
					}
					if editorCellsEqual(before, after, before.Sheets[id].CellData[y][x], after.Sheets[id].CellData[y][x]) {
						v = cells[axis]
					} else {
						a, b := before.Sheets[id].CellData[y][x], after.Sheets[id].CellData[y][x]
						keepValue := a != nil && b != nil && a.Formula == b.Formula && reflect.DeepEqual(a.Value, b.Value)
						v, e = mergeCellXML(cells[axis], v, keepValue)
						if e != nil {
							return nil, e
						}
					}
				}
				row.Write(v)
			}
			for _, c := range oldCells {
				if c.Name != "c" {
					row.Write(previous[c.Start:c.End])
				}
			}
			row.Write(raw[ct:])
			raw = row.Bytes()
			// Retain unknown row attributes. The standard attributes are controlled by the editor.
			raw = mergeXMLAttributes(previous, raw, map[string]bool{"r": true, "s": true, "customFormat": true, "ht": true, "hidden": true, "customHeight": true, "spans": true})
		}
		out.Write(raw)
	}
	out.Write(fresh[tail:])
	return append(append(append([]byte{}, updated[:newData.Start]...), out.Bytes()...), updated[newData.End:]...), nil
}
func expandEmptyOfficeRoot(raw []byte) []byte {
	trim := bytes.TrimSpace(raw)
	end := bytes.IndexByte(trim, '>')
	if end == len(trim)-1 && bytes.HasSuffix(trim, []byte("/>")) {
		name := string(trim[1 : len(trim)-2])
		if i := strings.IndexAny(name, " \t\r\n"); i >= 0 {
			name = name[:i]
		}
		return []byte(string(trim[:len(trim)-2]) + ">" + "</" + name + ">")
	}
	return raw
}
func cellCoordinates(axis string) (int, int, error) {
	col, row := 0, 0
	i := 0
	for i < len(axis) && axis[i] >= 'A' && axis[i] <= 'Z' {
		col = col*26 + int(axis[i]-'A'+1)
		i++
	}
	n, err := strconv.Atoi(axis[i:])
	row = n
	if err != nil || col < 1 || row < 1 {
		return 0, 0, fmt.Errorf("Invalid cell address")
	}
	return col - 1, row - 1, nil
}
func xmlAttribute(raw []byte, name string) string {
	d := xml.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil {
		return ""
	}
	if start, ok := token.(xml.StartElement); ok {
		for _, a := range start.Attr {
			if a.Name.Local == name {
				return a.Value
			}
		}
	}
	return ""
}
func mergeCellXML(original, updated []byte, keepValue bool) ([]byte, error) {
	value, err := mergeOfficeXML(original, updated, map[string]bool{"f": !keepValue, "v": !keepValue, "is": !keepValue})
	if err != nil {
		return nil, err
	}
	// The new value's type and style are authoritative; other cell attributes survive.
	oldClose := bytes.IndexByte(value, '>')
	newClose := bytes.IndexByte(updated, '>')
	if oldClose < 0 || newClose < 0 {
		return nil, fmt.Errorf("Invalid cell XML")
	}
	body := value[oldClose+1:]
	header := []byte(`<c r="` + xmlText(xmlAttribute(updated, "r")) + `"`)
	if style := xmlAttribute(updated, "s"); style != "" {
		header = append(header, []byte(` s="`+xmlText(style)+`"`)...)
	}
	typeSource := updated
	if keepValue {
		typeSource = original
	}
	if kind := xmlAttribute(typeSource, "t"); kind != "" {
		header = append(header, []byte(` t="`+xmlText(kind)+`"`)...)
	}
	header = append(header, '>')
	if bytes.HasSuffix(header, []byte("/>")) {
		header = append(append([]byte{}, header[:len(header)-2]...), '>')
	}
	value = append(append([]byte{}, header...), body...)
	return mergeXMLAttributes(original, value, map[string]bool{"r": true, "s": true, "t": true}), nil
}
func mergeXMLAttributes(original, updated []byte, owned map[string]bool) []byte {
	end := bytes.IndexByte(updated, '>')
	if end < 0 {
		return updated
	}
	d := xml.NewDecoder(bytes.NewReader(original))
	token, err := d.RawToken()
	if err != nil {
		return updated
	}
	start, ok := token.(xml.StartElement)
	if !ok {
		return updated
	}
	for _, a := range start.Attr {
		name := a.Name.Local
		if a.Name.Space != "" {
			name = a.Name.Space + ":" + name
		}
		if owned[name] || bytes.Contains(updated[:end], []byte(" "+name+"=")) {
			continue
		}
		at := end
		if updated[end-1] == '/' {
			at--
		}
		attr := []byte(" " + name + "=\"" + xmlText(a.Value) + "\"")
		updated = append(append(append([]byte{}, updated[:at]...), attr...), updated[at:]...)
		end += len(attr)
	}
	return updated
}
func orderedOfficeChildren(data []byte) ([]byte, error) {
	children, head, tail, err := officeXMLChildren(data)
	if err != nil {
		return nil, err
	}
	// ECMA-376 worksheet order. Unknown extensions stay at the end, in original order.
	if !bytes.Contains(data[:head], []byte("worksheet")) {
		return data, nil
	}
	order := strings.Fields("sheetPr dimension sheetViews sheetFormatPr cols sheetData sheetCalcPr sheetProtection protectedRanges scenarios autoFilter sortState dataConsolidate customSheetViews mergeCells phoneticPr conditionalFormatting dataValidations hyperlinks printOptions pageMargins pageSetup headerFooter rowBreaks colBreaks customProperties cellWatches ignoredErrors smartTags drawing legacyDrawing legacyDrawingHF picture oleObjects controls webPublishItems tableParts extLst")
	rank := map[string]int{}
	for i, name := range order {
		rank[name] = i + 1
	}
	sort.SliceStable(children, func(i, j int) bool {
		a, b := rank[children[i].Name], rank[children[j].Name]
		if a == 0 {
			a = len(order) + 1
		}
		if b == 0 {
			b = len(order) + 1
		}
		return a < b
	})
	var out bytes.Buffer
	out.Write(data[:head])
	for _, c := range children {
		out.Write(data[c.Start:c.End])
	}
	out.Write(data[tail:])
	return out.Bytes(), nil
}
func editorColor(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "rgb(") && strings.HasSuffix(value, ")") {
		parts := strings.Split(value[4:len(value)-1], ",")
		if len(parts) == 3 {
			rgb := [3]int{}
			for i, p := range parts {
				n, e := strconv.Atoi(strings.TrimSpace(p))
				if e != nil || n < 0 || n > 255 {
					return value
				}
				rgb[i] = n
			}
			return fmt.Sprintf("%02X%02X%02X", rgb[0], rgb[1], rgb[2])
		}
	}
	value = strings.TrimPrefix(value, "#")
	if len(value) == 3 {
		value = string([]byte{value[0], value[0], value[1], value[1], value[2], value[2]})
	}
	return value
}

func mergeEditorStyle(base *excelize.Style, previous, next json.RawMessage) (*excelize.Style, error) {
	candidate, err := encodeEditorStyle(next)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return candidate, nil
	}
	var a, b map[string]interface{}
	_ = json.Unmarshal(previous, &a)
	_ = json.Unmarshal(next, &b)
	if base.Font == nil {
		base.Font = &excelize.Font{}
	}
	if base.Alignment == nil {
		base.Alignment = &excelize.Alignment{}
	}
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	for key := range keys {
		if reflect.DeepEqual(a[key], b[key]) {
			continue
		}
		switch key {
		case "ff":
			base.Font.Family = candidate.Font.Family
		case "fs":
			base.Font.Size = candidate.Font.Size
		case "bl":
			base.Font.Bold = candidate.Font.Bold
		case "it":
			base.Font.Italic = candidate.Font.Italic
		case "ul":
			base.Font.Underline = candidate.Font.Underline
		case "st":
			base.Font.Strike = candidate.Font.Strike
		case "cl":
			base.Font.Color = candidate.Font.Color
			base.Font.ColorTheme = nil
			base.Font.ColorIndexed = 0
			base.Font.ColorTint = 0
		case "bg":
			base.Fill = candidate.Fill
		case "bd":
			base.Border = candidate.Border
		case "ht":
			base.Alignment.Horizontal = candidate.Alignment.Horizontal
		case "vt":
			base.Alignment.Vertical = candidate.Alignment.Vertical
		case "tb":
			base.Alignment.WrapText = candidate.Alignment.WrapText
		case "n":
			base.NumFmt = 0
			base.CustomNumFmt = candidate.CustomNumFmt
		}
	}
	return base, nil
}

// Write the engine's calculated cache without replacing formulas or cell extensions.
func cacheEditorFormulaValues(parts map[string][]byte, book EditorWorkbook) error {
	for _, sheet := range book.Sheets {
		name := sheetPartByName(parts, sheet.Name)
		raw := parts[name]
		children, _, _, err := officeXMLChildren(raw)
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.Name != "sheetData" {
				continue
			}
			data := raw[child.Start:child.End]
			rows, head, tail, err := officeXMLChildren(data)
			if err != nil {
				return err
			}
			var out bytes.Buffer
			out.Write(data[:head])
			for _, row := range rows {
				r := expandEmptyOfficeRoot(data[row.Start:row.End])
				cells, ch, ct, e := officeXMLChildren(r)
				if e != nil {
					return e
				}
				var line bytes.Buffer
				line.Write(r[:ch])
				for _, cell := range cells {
					v := r[cell.Start:cell.End]
					col, rr, e := cellCoordinates(xmlAttribute(v, "r"))
					if e == nil {
						c := sheet.CellData[rr][col]
						if c != nil && c.Formula != "" {
							kind := ""
							value := ""
							switch x := c.Value.(type) {
							case string:
								kind = "str"
								value = x
								if strings.HasPrefix(x, "#") && strings.HasSuffix(x, "!") || x == "#N/A" || x == "#NAME?" {
									kind = "e"
								}
							case bool:
								kind = "b"
								if x {
									value = "1"
								} else {
									value = "0"
								}
							case float64:
								value = strconv.FormatFloat(x, 'g', -1, 64)
							case int:
								value = strconv.Itoa(x)
							}
							opening := []byte("<c")
							for _, key := range []string{"r", "s"} {
								if attr := xmlAttribute(v, key); attr != "" {
									opening = append(opening, []byte(" "+key+"=\""+xmlText(attr)+"\"")...)
								}
							}
							if kind != "" {
								opening = append(opening, []byte(" t=\""+kind+"\"")...)
							}
							opening = append(opening, '>')
							opening = mergeXMLAttributes(v, opening, map[string]bool{"r": true, "s": true, "t": true})
							expanded := expandEmptyOfficeRoot(v)
							items, _, _, e := officeXMLChildren(expanded)
							if e != nil {
								return e
							}
							var content bytes.Buffer
							content.Write(opening)
							for _, item := range items {
								if item.Name != "v" && item.Name != "is" {
									content.Write(expanded[item.Start:item.End])
								}
							}
							if c.Value != nil {
								content.WriteString("<v>" + xmlText(value) + "</v>")
							}
							content.WriteString("</c>")
							v = content.Bytes()
						}
					}
					line.Write(v)
				}
				line.Write(r[ct:])
				out.Write(line.Bytes())
			}
			out.Write(data[tail:])
			parts[name] = append(append(append([]byte{}, raw[:child.Start]...), out.Bytes()...), raw[child.End:]...)
			break
		}
	}
	return nil
}
func applyEditorFilteredRows(f *excelize.File, before, after EditorWorkbook) error {
	if !resourceChanged(before, after, resourceFilter) {
		return nil
	}
	for id, s := range after.Sheets {
		old, e := resourceSheet[editorFilter](before, resourceFilter, id)
		if e != nil {
			return e
		}
		next, e := resourceSheet[editorFilter](after, resourceFilter, id)
		if e != nil {
			return e
		}
		rows := map[int]bool{}
		for _, r := range old.Hidden {
			rows[r] = false
		}
		for _, r := range next.Hidden {
			rows[r] = true
		}
		for r, hidden := range rows {
			if r < 0 || r >= s.RowCount {
				return fmt.Errorf("Invalid filtered row")
			}
			if e = f.SetRowVisible(s.Name, r+1, !hidden && s.RowData[r].Hidden == 0); e != nil {
				return e
			}
		}
	}
	return nil
}

func EncodeEditorCSV(data []byte, name string) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if name == "" {
		name = f.GetSheetList()[0]
	}
	rows, err := f.GetRows(name)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	for _, row := range rows {
		for i, value := range row {
			row[i] = neutralizeCSVFormulaCell(value)
		}
		if err = writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return out.Bytes(), writer.Error()
}
