package office

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const XLSXMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
const workbookMetadataPart = "aurago/editor.json"

var ErrWorkbookRequiresNativeEditor = errors.New("This workbook contains features the legacy editor cannot preserve; use Tabellen or export an explicit simplified copy")

// EditorWorkbook follows Univer's public snapshot contract. Cell values remain typed.
type EditorWorkbook struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	AppVersion string                     `json:"appVersion"`
	Locale     string                     `json:"locale"`
	SheetOrder []string                   `json:"sheetOrder"`
	Sheets     map[string]*EditorSheet    `json:"sheets"`
	Styles     map[string]json.RawMessage `json:"styles"`
	Resources  []EditorResource           `json:"resources,omitempty"`
	Custom     json.RawMessage            `json:"custom,omitempty"`
}
type EditorResource struct {
	Name string `json:"name"`
	Data string `json:"data"`
}
type EditorSheet struct {
	ID                 string                      `json:"id"`
	Name               string                      `json:"name"`
	RowCount           int                         `json:"rowCount"`
	ColumnCount        int                         `json:"columnCount"`
	DefaultRowHeight   float64                     `json:"defaultRowHeight"`
	DefaultColumnWidth float64                     `json:"defaultColumnWidth"`
	CellData           map[int]map[int]*EditorCell `json:"cellData"`
	RowData            map[int]EditorDimension     `json:"rowData"`
	ColumnData         map[int]EditorDimension     `json:"columnData"`
	MergeData          []EditorRange               `json:"mergeData"`
	Freeze             EditorFreeze                `json:"freeze"`
	ZoomRatio          float64                     `json:"zoomRatio"`
	Hidden             int                         `json:"hidden"`
	ShowGridlines      int                         `json:"showGridlines"`
	RowHeader          map[string]int              `json:"rowHeader"`
	ColumnHeader       map[string]int              `json:"columnHeader"`
	TabColor           string                      `json:"tabColor,omitempty"`
	Custom             json.RawMessage             `json:"custom,omitempty"`
}
type EditorCell struct {
	Value    interface{}     `json:"v,omitempty"`
	Type     int             `json:"t,omitempty"`
	Formula  string          `json:"f,omitempty"`
	Style    json.RawMessage `json:"s,omitempty"`
	RichText json.RawMessage `json:"p,omitempty"`
	Custom   json.RawMessage `json:"custom,omitempty"`
}
type EditorDimension struct {
	H      float64         `json:"h,omitempty"`
	W      float64         `json:"w,omitempty"`
	Hidden int             `json:"hd,omitempty"`
	Auto   int             `json:"ia,omitempty"`
	Style  json.RawMessage `json:"s,omitempty"`
}
type EditorRange struct {
	StartRow    int `json:"startRow"`
	EndRow      int `json:"endRow"`
	StartColumn int `json:"startColumn"`
	EndColumn   int `json:"endColumn"`
}
type EditorFreeze struct {
	X      int `json:"xSplit"`
	Y      int `json:"ySplit"`
	Row    int `json:"startRow"`
	Column int `json:"startColumn"`
}
type WorkbookEditorDocument struct {
	SchemaVersion    int            `json:"schema_version"`
	Workbook         EditorWorkbook `json:"workbook"`
	Charts           []EditorChart  `json:"charts"`
	Limitations      []string       `json:"limitations"`
	StructuralLocked bool           `json:"structural_locked"`
}
type EditorChart struct {
	ID       string              `json:"id"`
	Sheet    string              `json:"sheet"`
	Type     string              `json:"type"`
	Title    string              `json:"title"`
	Range    string              `json:"range"`
	Anchor   string              `json:"anchor"`
	X        int                 `json:"x"`
	Y        int                 `json:"y"`
	Width    int                 `json:"width"`
	Height   int                 `json:"height"`
	Legend   bool                `json:"legend"`
	XTitle   string              `json:"x_title,omitempty"`
	YTitle   string              `json:"y_title,omitempty"`
	Colors   []string            `json:"colors,omitempty"`
	Series   []EditorChartSeries `json:"series,omitempty"`
	Readonly bool                `json:"readonly,omitempty"`
}
type EditorChartSeries struct {
	Name       string `json:"name"`
	Categories string `json:"categories"`
	Values     string `json:"values"`
	Color      string `json:"color,omitempty"`
}
type WorkbookStructureOperation struct {
	Type  string `json:"type"`
	Sheet string `json:"sheet"`
	Index int    `json:"index"`
	Count int    `json:"count"`
}
type WorkbookEditorPatch struct {
	SchemaVersion int                          `json:"schema_version"`
	Workbook      EditorWorkbook               `json:"workbook"`
	Charts        []EditorChart                `json:"charts"`
	Operations    []WorkbookStructureOperation `json:"operations,omitempty"`
}
type workbookMetadata struct {
	Fingerprint string         `json:"fingerprint"`
	Workbook    EditorWorkbook `json:"workbook"`
	Charts      []EditorChart  `json:"charts"`
}

func workbookFingerprint(parts map[string][]byte) string {
	names := make([]string, 0, len(parts))
	for name := range parts {
		if name != workbookMetadataPart {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(parts[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}
func emptyEditorWorkbook() EditorWorkbook {
	return EditorWorkbook{ID: "aurago-workbook", Name: "Workbook", AppVersion: "0.25.1", Locale: "enUS", Styles: map[string]json.RawMessage{}, Sheets: map[string]*EditorSheet{}, SheetOrder: []string{}}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func jsonBytes(v interface{}) json.RawMessage { b, _ := json.Marshal(v); return b }
func decodeEditorStyle(style *excelize.Style) json.RawMessage {
	m := map[string]interface{}{}
	if s := style.Font; s != nil {
		m["ff"] = s.Family
		m["fs"] = s.Size
		m["bl"] = boolInt(s.Bold)
		m["it"] = boolInt(s.Italic)
		m["ul"] = map[string]int{"s": boolInt(s.Underline != "")}
		m["st"] = map[string]int{"s": boolInt(s.Strike)}
		if s.Color != "" {
			m["cl"] = map[string]string{"rgb": "#" + s.Color}
		}
	}
	if len(style.Fill.Color) > 0 && style.Fill.Pattern != 0 {
		m["bg"] = map[string]string{"rgb": "#" + style.Fill.Color[0]}
	}
	if s := style.Alignment; s != nil {
		m["ht"] = map[string]int{"left": 1, "center": 2, "right": 3, "justify": 4, "distributed": 6}[s.Horizontal]
		m["vt"] = map[string]int{"top": 1, "center": 2, "bottom": 3}[s.Vertical]
		if s.WrapText {
			m["tb"] = 3
		}
	}
	if style.CustomNumFmt != nil {
		m["n"] = map[string]string{"pattern": *style.CustomNumFmt}
	} else if p := map[int]string{0: "General", 1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00", 9: "0%", 10: "0.00%", 11: "0.00E+00", 14: "yyyy-mm-dd", 15: "d-mmm-yy", 20: "h:mm", 21: "h:mm:ss", 22: "m/d/yy h:mm", 49: "@"}[style.NumFmt]; p != "" {
		m["n"] = map[string]string{"pattern": p}
	}
	bd := map[string]interface{}{}
	for _, b := range style.Border {
		key := map[string]string{"left": "l", "right": "r", "top": "t", "bottom": "b", "diagonalDown": "bl_tr", "diagonalUp": "tl_br"}[b.Type]
		if key != "" && b.Style > 0 {
			bd[key] = map[string]interface{}{"s": b.Style, "cl": map[string]string{"rgb": "#" + b.Color}}
		}
	}
	if len(bd) > 0 {
		m["bd"] = bd
	}
	return jsonBytes(m)
}

func DecodeEditorWorkbook(data []byte) (WorkbookEditorDocument, error) {
	parts, err := readOfficeParts(data, "xl/workbook.xml")
	if err != nil {
		return WorkbookEditorDocument{}, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return WorkbookEditorDocument{}, err
	}
	defer f.Close()
	doc := WorkbookEditorDocument{SchemaVersion: 2, Workbook: emptyEditorWorkbook(), Charts: []EditorChart{}, Limitations: []string{}}
	// Metadata is only trusted when every standard/opaque package part still matches.
	var meta workbookMetadata
	if json.Unmarshal(parts[workbookMetadataPart], &meta) == nil && meta.Fingerprint == workbookFingerprint(parts) && validateEditorWorkbook(meta.Workbook) == nil {
		doc.Workbook = meta.Workbook
		doc.Charts = meta.Charts
	} else {
		for _, name := range f.GetSheetList() {
			index, _ := f.GetSheetIndex(name)
			id := "sheet-" + strconv.Itoa(index+1)
			sheet := &EditorSheet{ID: id, Name: name, RowCount: 1000, ColumnCount: 26, DefaultRowHeight: 24, DefaultColumnWidth: 100, CellData: map[int]map[int]*EditorCell{}, RowData: map[int]EditorDimension{}, ColumnData: map[int]EditorDimension{}, MergeData: []EditorRange{}, Freeze: EditorFreeze{Row: -1, Column: -1}, ZoomRatio: 1, ShowGridlines: 1, RowHeader: map[string]int{"width": 48}, ColumnHeader: map[string]int{"height": 26}}
			visible, _ := f.GetSheetVisible(name)
			sheet.Hidden = boolInt(!visible)
			if view, e := f.GetSheetView(name, 0); e == nil {
				if view.ZoomScale != nil && *view.ZoomScale > 0 {
					sheet.ZoomRatio = float64(*view.ZoomScale) / 100
				}
				if view.ShowGridLines != nil {
					sheet.ShowGridlines = boolInt(*view.ShowGridLines)
				}
			}
			if props, e := f.GetSheetProps(name); e == nil && props.TabColorRGB != nil {
				sheet.TabColor = "#" + strings.TrimPrefix(*props.TabColorRGB, "FF")
			}
			sheetPath := sheetPartByName(parts, name)
			var raw struct {
				Format struct {
					DefaultRowHeight float64 `xml:"defaultRowHeight,attr"`
					DefaultColWidth  float64 `xml:"defaultColWidth,attr"`
				} `xml:"sheetFormatPr"`
				Cols []struct {
					Min    int     `xml:"min,attr"`
					Max    int     `xml:"max,attr"`
					Width  float64 `xml:"width,attr"`
					Hidden bool    `xml:"hidden,attr"`
					Style  int     `xml:"style,attr"`
				} `xml:"cols>col"`
				Rows []struct {
					Number int     `xml:"r,attr"`
					Height float64 `xml:"ht,attr"`
					Hidden bool    `xml:"hidden,attr"`
					Cells  []struct {
						Address string `xml:"r,attr"`
						Style   int    `xml:"s,attr"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			if err = xml.Unmarshal(parts[sheetPath], &raw); err != nil {
				return doc, err
			}
			if raw.Format.DefaultRowHeight > 0 {
				sheet.DefaultRowHeight = raw.Format.DefaultRowHeight * 96 / 72
			}
			if raw.Format.DefaultColWidth > 0 {
				sheet.DefaultColumnWidth = math.Round(raw.Format.DefaultColWidth*7 + 5)
			}
			for _, col := range raw.Cols {
				if col.Min < 1 || col.Max > 16384 {
					return doc, fmt.Errorf("Invalid column extent")
				}
				for c := col.Min - 1; c < col.Max; c++ {
					sheet.ColumnData[c] = EditorDimension{W: math.Round(col.Width*7 + 5), Hidden: boolInt(col.Hidden)}
				}
			}
			total := 0
			for _, row := range raw.Rows {
				if row.Number < 1 || row.Number > 1048576 {
					return doc, fmt.Errorf("Invalid row extent")
				}
				r := row.Number - 1
				if row.Height > 0 || row.Hidden {
					sheet.RowData[r] = EditorDimension{H: row.Height * 96 / 72, Hidden: boolInt(row.Hidden)}
				}
				for _, cell := range row.Cells {
					total++
					if total > 2000000 {
						return doc, fmt.Errorf("Workbook exceeds editable cell limit")
					}
					c, rr, e := excelize.CellNameToCoordinates(cell.Address)
					if e != nil {
						return doc, e
					}
					rr--
					c--
					value, e := f.GetCellValue(name, cell.Address, excelize.Options{RawCellValue: true})
					if e != nil {
						return doc, e
					}
					formula, e := f.GetCellFormula(name, cell.Address)
					if e != nil {
						return doc, e
					}
					typ, _ := f.GetCellType(name, cell.Address)
					ec := &EditorCell{Value: value, Type: 1}
					if typ == excelize.CellTypeBool {
						ec.Type = 3
						ec.Value = boolInt(value == "1" || value == "TRUE")
					} else if typ == excelize.CellTypeNumber || typ == excelize.CellTypeUnset {
						if n, e := strconv.ParseFloat(value, 64); e == nil {
							ec.Type = 2
							ec.Value = n
						}
					}
					if formula != "" {
						ec.Formula = "=" + formula
					}
					if cell.Style > 0 {
						key := "excel-" + strconv.Itoa(cell.Style)
						if _, ok := doc.Workbook.Styles[key]; !ok {
							style, e := f.GetStyle(cell.Style)
							if e != nil {
								return doc, e
							}
							doc.Workbook.Styles[key] = decodeEditorStyle(style)
						}
						ec.Style = jsonBytes(key)
					}
					if sheet.CellData[rr] == nil {
						sheet.CellData[rr] = map[int]*EditorCell{}
					}
					sheet.CellData[rr][c] = ec
					sheet.RowCount = max(sheet.RowCount, rr+50)
					sheet.ColumnCount = max(sheet.ColumnCount, c+5)
				}
			}
			merged, e := f.GetMergeCells(name, true)
			if e != nil {
				return doc, e
			}
			for _, m := range merged {
				a, b, _ := excelize.CellNameToCoordinates(m.GetStartAxis())
				c, d, _ := excelize.CellNameToCoordinates(m.GetEndAxis())
				sheet.MergeData = append(sheet.MergeData, EditorRange{b - 1, d - 1, a - 1, c - 1})
			}
			panes, e := f.GetPanes(name)
			if e == nil && panes.Freeze {
				sheet.Freeze = EditorFreeze{panes.XSplit, panes.YSplit, panes.YSplit, panes.XSplit}
			}
			doc.Workbook.Sheets[id] = sheet
			doc.Workbook.SheetOrder = append(doc.Workbook.SheetOrder, id)
		}
		readEditorPrint(f, &doc.Workbook)
		doc.Charts = readEditorCharts(parts, doc.Workbook)
		if err = readEditorResources(f, parts, &doc); err != nil {
			return doc, err
		}
	}
	for name, content := range parts {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			var rules struct {
				Rules []struct {
					Type string `xml:"type,attr"`
				} `xml:"conditionalFormatting>cfRule"`
			}
			if err := xml.Unmarshal(content, &rules); err != nil {
				return doc, err
			}
			for _, rule := range rules.Rules {
				if rule.Type != "cellIs" && rule.Type != "expression" && rule.Type != "containsText" {
					doc.Limitations = append(doc.Limitations, "conditions_preserved")
					break
				}
			}
		}
		if strings.Contains(name, "vbaProject") {
			doc.Limitations = append(doc.Limitations, "macros_preserved")
			doc.StructuralLocked = true
		}
		if strings.HasPrefix(name, "xl/externalLinks/") || strings.HasPrefix(name, "xl/pivot") || strings.HasPrefix(name, "xl/slicer") {
			doc.StructuralLocked = true
		}
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") && (bytes.Contains(content, []byte("<extLst")) || bytes.Contains(content, []byte(":extLst")) || bytes.Contains(content, []byte("<sheetProtection"))) {
			doc.StructuralLocked = true
		}
	}
	if doc.StructuralLocked {
		doc.Limitations = append(doc.Limitations, "structure_locked")
	}
	return doc, nil
}
func validateEditorWorkbook(book EditorWorkbook) error {
	if len(book.SheetOrder) == 0 || len(book.SheetOrder) > 256 || len(book.Sheets) != len(book.SheetOrder) {
		return fmt.Errorf("Invalid sheet count")
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	total := 0
	for _, id := range book.SheetOrder {
		s := book.Sheets[id]
		if s == nil || id == "" || len(id) > 128 || ids[id] || s.ID != id {
			return fmt.Errorf("Invalid sheet identity")
		}
		ids[id] = true
		if s.Name == "" || len([]rune(s.Name)) > 31 || strings.ContainsAny(s.Name, "[]:*?/\\") || names[strings.ToLower(s.Name)] {
			return fmt.Errorf("Invalid or duplicate sheet name")
		}
		names[strings.ToLower(s.Name)] = true
		if s.RowCount < 1 || s.RowCount > 1048576 || s.ColumnCount < 1 || s.ColumnCount > 16384 {
			return fmt.Errorf("Invalid sheet dimensions")
		}
		for r, cols := range s.CellData {
			if r < 0 || r >= 1048576 {
				return fmt.Errorf("Invalid cell row")
			}
			for c, cell := range cols {
				total++
				if c < 0 || c >= 16384 || total > 2000000 {
					return fmt.Errorf("Workbook exceeds cell limits")
				}
				if cell == nil {
					continue
				}
				if len(cell.Formula) > 8192 {
					return fmt.Errorf("Formula exceeds limit")
				}
				switch v := cell.Value.(type) {
				case nil, bool:
				case float64:
					if math.IsNaN(v) || math.IsInf(v, 0) {
						return fmt.Errorf("Invalid numeric cell")
					}
				case string:
					if len([]rune(v)) > 32767 {
						return fmt.Errorf("Cell text exceeds limit")
					}
				default:
					return fmt.Errorf("Unsupported cell value")
				}
			}
		}
		for _, r := range s.MergeData {
			if r.StartRow < 0 || r.StartColumn < 0 || r.EndRow < r.StartRow || r.EndColumn < r.StartColumn || r.EndRow >= 1048576 || r.EndColumn >= 16384 {
				return fmt.Errorf("Invalid merged range")
			}
		}
		if s.Freeze.X < 0 || s.Freeze.Y < 0 || s.Freeze.X > 16384 || s.Freeze.Y > 1048576 {
			return fmt.Errorf("Invalid freeze range")
		}
	}
	if len(book.Resources) > 64 || len(book.Styles) > 100000 {
		return fmt.Errorf("Workbook resources exceed limit")
	}
	for _, r := range book.Resources {
		if len(r.Data) > 16<<20 || len(r.Name) > 128 {
			return fmt.Errorf("Resource exceeds limit")
		}
	}
	return nil
}

func resolvedStyle(book EditorWorkbook, cell *EditorCell) json.RawMessage {
	if cell == nil {
		return nil
	}
	var id string
	if json.Unmarshal(cell.Style, &id) == nil {
		return book.Styles[id]
	}
	return cell.Style
}
func encodeEditorStyle(raw json.RawMessage) (*excelize.Style, error) {
	var s struct {
		Font      string  `json:"ff"`
		Size      float64 `json:"fs"`
		Bold      int     `json:"bl"`
		Italic    int     `json:"it"`
		Underline struct {
			S int `json:"s"`
		} `json:"ul"`
		Strike struct {
			S int `json:"s"`
		} `json:"st"`
		Color struct {
			RGB string `json:"rgb"`
		} `json:"cl"`
		Background struct {
			RGB string `json:"rgb"`
		} `json:"bg"`
		Horizontal int `json:"ht"`
		Vertical   int `json:"vt"`
		Wrap       int `json:"tb"`
		Number     struct {
			Pattern string `json:"pattern"`
		} `json:"n"`
		Border map[string]struct {
			S     int `json:"s"`
			Color struct {
				RGB string `json:"rgb"`
			} `json:"cl"`
		} `json:"bd"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("Invalid cell style: %w", err)
		}
	}
	if s.Size < 0 || s.Size > 409 || len(s.Font) > 200 || len(s.Number.Pattern) > 512 {
		return nil, fmt.Errorf("Style exceeds limits")
	}
	style := &excelize.Style{Font: &excelize.Font{Family: s.Font, Size: s.Size, Bold: s.Bold == 1, Italic: s.Italic == 1, Strike: s.Strike.S == 1, Color: editorColor(s.Color.RGB)}, Alignment: &excelize.Alignment{Horizontal: map[int]string{1: "left", 2: "center", 3: "right", 4: "justify", 6: "distributed"}[s.Horizontal], Vertical: map[int]string{1: "top", 2: "center", 3: "bottom"}[s.Vertical], WrapText: s.Wrap == 3}}
	if s.Underline.S == 1 {
		style.Font.Underline = "single"
	}
	if s.Background.RGB != "" {
		style.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{editorColor(s.Background.RGB)}}
	}
	if s.Number.Pattern != "" {
		style.CustomNumFmt = &s.Number.Pattern
	}
	for key, b := range s.Border {
		kind := map[string]string{"t": "top", "r": "right", "b": "bottom", "l": "left"}[key]
		if kind != "" {
			if b.S < 0 || b.S > 13 {
				return nil, fmt.Errorf("Invalid border style")
			}
			style.Border = append(style.Border, excelize.Border{Type: kind, Style: b.S, Color: editorColor(b.Color.RGB)})
		}
	}
	return style, nil
}
func editorCellsEqual(aBook, bBook EditorWorkbook, a, b *EditorCell) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return reflect.DeepEqual(a.Value, b.Value) && a.Type == b.Type && a.Formula == b.Formula && bytes.Equal(resolvedStyle(aBook, a), resolvedStyle(bBook, b)) && bytes.Equal(a.RichText, b.RichText)
}

// ApplyWorkbookEditorPatch edits the original package. The caller must verify its
// version again inside the file-service lock before publishing these bytes.
func ApplyWorkbookEditorPatch(original []byte, patch WorkbookEditorPatch) ([]byte, error) {
	if patch.SchemaVersion != 2 {
		return nil, fmt.Errorf("Unsupported workbook schema")
	}
	if err := validateEditorWorkbook(patch.Workbook); err != nil {
		return nil, err
	}
	var f *excelize.File
	var err error
	before := WorkbookEditorDocument{Workbook: emptyEditorWorkbook()}
	originalParts := map[string][]byte{}
	if len(original) > 0 {
		before, err = DecodeEditorWorkbook(original)
		if err != nil {
			return nil, err
		}
		originalParts, err = readOfficeParts(original, "xl/workbook.xml")
		if err != nil {
			return nil, err
		}
		f, err = excelize.OpenReader(bytes.NewReader(original))
	} else {
		f = excelize.NewFile()
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	structural := len(patch.Operations) > 0 || !reflect.DeepEqual(before.Workbook.SheetOrder, patch.Workbook.SheetOrder)
	for id, s := range patch.Workbook.Sheets {
		if old := before.Workbook.Sheets[id]; old != nil && old.Name != s.Name {
			structural = true
		}
	}
	if structural && before.StructuralLocked {
		return nil, fmt.Errorf("Structural changes are locked because this workbook contains unsupported references")
	}
	if resourceChanged(before.Workbook, patch.Workbook, resourceCondition) {
		for _, limitation := range before.Limitations {
			if limitation == "conditions_preserved" {
				return nil, fmt.Errorf("Unsupported conditional rules must remain unchanged")
			}
		}
	}
	if len(patch.Operations) > 10000 {
		return nil, fmt.Errorf("Too many structural operations")
	}
	// Rename in two passes, so exchanging sheet names is valid.
	for id, s := range patch.Workbook.Sheets {
		if old := before.Workbook.Sheets[id]; old != nil && old.Name != s.Name {
			if err = f.SetSheetName(old.Name, "ag-"+id[:min(20, len(id))]); err != nil {
				return nil, err
			}
		}
	}
	for _, id := range patch.Workbook.SheetOrder {
		s := patch.Workbook.Sheets[id]
		old := before.Workbook.Sheets[id]
		if old == nil {
			if len(original) == 0 && len(f.GetSheetList()) == 1 && f.GetSheetList()[0] == "Sheet1" {
				err = f.SetSheetName("Sheet1", s.Name)
			} else {
				_, err = f.NewSheet(s.Name)
			}
		} else if old.Name != s.Name {
			err = f.SetSheetName("ag-"+id[:min(20, len(id))], s.Name)
		}
		if err != nil {
			return nil, err
		}
	}
	for id, s := range before.Workbook.Sheets {
		if patch.Workbook.Sheets[id] == nil {
			if err = f.DeleteSheet(s.Name); err != nil {
				return nil, err
			}
		}
	}
	for _, op := range patch.Operations {
		s := patch.Workbook.Sheets[op.Sheet]
		limit := 1048576
		if strings.HasSuffix(op.Type, "Columns") {
			limit = 16384
		}
		if s == nil || op.Index < 0 || op.Count < 1 || op.Index+op.Count > limit {
			return nil, fmt.Errorf("Invalid structural operation")
		}
		axis, _ := excelize.ColumnNumberToName(op.Index + 1)
		switch op.Type {
		case "insertRows":
			err = f.InsertRows(s.Name, op.Index+1, op.Count)
		case "deleteRows":
			for i := 0; i < op.Count && err == nil; i++ {
				err = f.RemoveRow(s.Name, op.Index+1)
			}
		case "insertColumns":
			err = f.InsertCols(s.Name, axis, op.Count)
		case "deleteColumns":
			for i := 0; i < op.Count && err == nil; i++ {
				err = f.RemoveCol(s.Name, axis)
			}
		default:
			return nil, fmt.Errorf("Unknown structural operation")
		}
		if err != nil {
			return nil, err
		}
	}
	styleIDs := map[string]int{}
	dirtySheets := map[string]bool{}
	for _, id := range patch.Workbook.SheetOrder {
		s := patch.Workbook.Sheets[id]
		old := before.Workbook.Sheets[id]
		oldCells := map[int]map[int]*EditorCell{}
		if old != nil {
			oldCells = old.CellData
		}
		coords := map[int]map[int]bool{}
		for _, cells := range []map[int]map[int]*EditorCell{oldCells, s.CellData} {
			for r, cols := range cells {
				if coords[r] == nil {
					coords[r] = map[int]bool{}
				}
				for c := range cols {
					coords[r][c] = true
				}
			}
		}
		for r, cols := range coords {
			for c := range cols {
				cell := s.CellData[r][c]
				prev := oldCells[r][c]
				if !structural && editorCellsEqual(before.Workbook, patch.Workbook, prev, cell) {
					continue
				}
				dirtySheets[s.Name] = true
				address, _ := excelize.CoordinatesToCellName(c+1, r+1)
				if cell == nil {
					err = f.SetCellValue(s.Name, address, nil)
					if err == nil {
						err = f.SetCellStyle(s.Name, address, address, 0)
					}
				} else {
					if len(cell.RichText) > 0 && string(cell.RichText) != "null" && !bytes.Equal(cell.RichText, func() json.RawMessage {
						if prev != nil {
							return prev.RichText
						}
						return nil
					}()) {
						return nil, fmt.Errorf("Rich-text cell edits require a supported text projection")
					}
					if cell.Formula != "" {
						formula := normalizeFormulaForXLSX(strings.TrimPrefix(cell.Formula, "="))
						err = f.SetCellFormula(s.Name, address, formula)
					} else {
						err = f.SetCellFormula(s.Name, address, "")
						if err == nil {
							if cell.Type == 3 {
								truth := cell.Value == true || cell.Value == float64(1)
								err = f.SetCellBool(s.Name, address, truth)
							} else {
								err = f.SetCellValue(s.Name, address, cell.Value)
							}
						}
					}
					if err == nil {
						raw := resolvedStyle(patch.Workbook, cell)
						baseID, e := f.GetCellStyle(s.Name, address)
						if e != nil {
							return nil, e
						}
						key := strconv.Itoa(baseID) + ":" + string(resolvedStyle(before.Workbook, prev)) + ":" + string(raw)
						styleID, ok := styleIDs[key]
						if !ok {
							base, e := f.GetStyle(baseID)
							if e != nil {
								return nil, e
							}
							style, e := mergeEditorStyle(base, resolvedStyle(before.Workbook, prev), raw)
							if e != nil {
								return nil, e
							}
							styleID, e = f.NewStyle(style)
							if e != nil {
								return nil, e
							}
							styleIDs[key] = styleID
						}
						err = f.SetCellStyle(s.Name, address, address, styleID)
					}
				}
				if err != nil {
					return nil, err
				}
			}
		}
		if old == nil || !reflect.DeepEqual(old.RowData, s.RowData) || !reflect.DeepEqual(old.ColumnData, s.ColumnData) || old.DefaultColumnWidth != s.DefaultColumnWidth || old.DefaultRowHeight != s.DefaultRowHeight {
			dirtySheets[s.Name] = true
			props := excelize.SheetPropsOptions{DefaultRowHeight: ptr(s.DefaultRowHeight * 72 / 96), DefaultColWidth: ptr(math.Max(0, (s.DefaultColumnWidth-5)/7))}
			if err = f.SetSheetProps(s.Name, &props); err != nil {
				return nil, err
			}
			rowData := map[int]EditorDimension{}
			if old != nil {
				for r := range old.RowData {
					rowData[r] = EditorDimension{}
				}
			}
			for r, d := range s.RowData {
				rowData[r] = d
			}
			for r, d := range rowData {
				if r < 0 || r >= 1048576 || d.H < 0 || d.H > 546 {
					return nil, fmt.Errorf("Invalid row size")
				}
				if d.H == 0 {
					d.H = s.DefaultRowHeight
				}
				if d.H > 0 {
					if err = f.SetRowHeight(s.Name, r+1, d.H*72/96); err != nil {
						return nil, err
					}
				}
				if err = f.SetRowVisible(s.Name, r+1, d.Hidden == 0); err != nil {
					return nil, err
				}
			}
			columnData := map[int]EditorDimension{}
			if old != nil {
				for c := range old.ColumnData {
					columnData[c] = EditorDimension{}
				}
			}
			for c, d := range s.ColumnData {
				columnData[c] = d
			}
			for c, d := range columnData {
				if c < 0 || c >= 16384 || d.W < 0 || d.W > 1790 {
					return nil, fmt.Errorf("Invalid column size")
				}
				col, _ := excelize.ColumnNumberToName(c + 1)
				if d.W == 0 {
					d.W = s.DefaultColumnWidth
				}
				if d.W > 0 {
					if err = f.SetColWidth(s.Name, col, col, math.Max(0, (d.W-5)/7)); err != nil {
						return nil, err
					}
				}
				if err = f.SetColVisible(s.Name, col, d.Hidden == 0); err != nil {
					return nil, err
				}
			}
		}
		if old == nil || !reflect.DeepEqual(old.MergeData, s.MergeData) {
			dirtySheets[s.Name] = true
			merges, _ := f.GetMergeCells(s.Name, true)
			for _, m := range merges {
				if err = f.UnmergeCell(s.Name, m.GetStartAxis(), m.GetEndAxis()); err != nil {
					return nil, err
				}
			}
			for _, m := range s.MergeData {
				a, _ := excelize.CoordinatesToCellName(m.StartColumn+1, m.StartRow+1)
				b, _ := excelize.CoordinatesToCellName(m.EndColumn+1, m.EndRow+1)
				if err = f.MergeCell(s.Name, a, b); err != nil {
					return nil, err
				}
			}
		}
		if old == nil || old.Freeze != s.Freeze {
			dirtySheets[s.Name] = true
			top, _ := excelize.CoordinatesToCellName(s.Freeze.X+1, s.Freeze.Y+1)
			if err = f.SetPanes(s.Name, &excelize.Panes{Freeze: s.Freeze.X > 0 || s.Freeze.Y > 0, XSplit: s.Freeze.X, YSplit: s.Freeze.Y, TopLeftCell: top, ActivePane: "bottomRight"}); err != nil {
				return nil, err
			}
		}
		if old == nil || old.ZoomRatio != s.ZoomRatio || old.ShowGridlines != s.ShowGridlines {
			dirtySheets[s.Name] = true
			zoom := s.ZoomRatio * 100
			if zoom == 0 {
				zoom = 100
			}
			if err = f.SetSheetView(s.Name, 0, &excelize.ViewOptions{ZoomScale: &zoom, ShowGridLines: ptr(s.ShowGridlines != 0)}); err != nil {
				return nil, err
			}
		}
		if old == nil || old.TabColor != s.TabColor {
			dirtySheets[s.Name] = true
			if err = f.SetSheetProps(s.Name, &excelize.SheetPropsOptions{TabColorRGB: ptr(strings.TrimPrefix(s.TabColor, "#"))}); err != nil {
				return nil, err
			}
		}
		if old == nil || old.Hidden != s.Hidden {
			if err = f.SetSheetVisible(s.Name, s.Hidden == 0); err != nil {
				return nil, err
			}
		}
	}
	for index := len(patch.Workbook.SheetOrder) - 2; index >= 0; index-- {
		if err = f.MoveSheet(patch.Workbook.Sheets[patch.Workbook.SheetOrder[index]].Name, patch.Workbook.Sheets[patch.Workbook.SheetOrder[index+1]].Name); err != nil {
			return nil, err
		}
	}
	if err = applyEditorFilteredRows(f, before.Workbook, patch.Workbook); err != nil {
		return nil, err
	}
	if err = applyEditorPrint(f, before.Workbook, patch.Workbook); err != nil {
		return nil, err
	}
	if err = applyEditorResources(f, before.Workbook, patch.Workbook); err != nil {
		return nil, err
	}
	if err = applyEditorCharts(f, before, patch); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	updated, err := readOfficeParts(buf.Bytes(), "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	updated, err = preserveWorkbookParts(originalParts, updated, dirtySheets, structural, before, patch)
	if err != nil {
		return nil, err
	}
	if err = applyEditorFilterParts(updated, before.Workbook, patch.Workbook); err != nil {
		return nil, err
	}
	if err = cacheEditorFormulaValues(updated, patch.Workbook); err != nil {
		return nil, err
	}
	if !bytes.Contains(updated["[Content_Types].xml"], []byte(`PartName="/aurago/editor.json"`)) {
		updated["[Content_Types].xml"] = bytes.Replace(updated["[Content_Types].xml"], []byte("</Types>"), []byte(`<Override PartName="/aurago/editor.json" ContentType="application/json"/></Types>`), 1)
	}
	metadata := workbookMetadata{Fingerprint: workbookFingerprint(updated), Workbook: patch.Workbook, Charts: patch.Charts}
	updated[workbookMetadataPart] = jsonBytes(metadata)
	return writeOfficeParts(updated)
}
func ptr[T any](v T) *T { return &v }
func writeOfficeParts(parts map[string][]byte) ([]byte, error) {
	var output bytes.Buffer
	z := zip.NewWriter(&output)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(parts[name]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
func sheetPartByName(parts map[string][]byte, name string) string {
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if xml.Unmarshal(parts["xl/workbook.xml"], &wb) != nil {
		return ""
	}
	rel := officeRelationships(parts["xl/_rels/workbook.xml.rels"])
	for _, s := range wb.Sheets {
		if s.Name == name {
			return resolveOfficePart("xl/workbook.xml", rel[s.ID])
		}
	}
	return ""
}
func officeRelationships(data []byte) map[string]string {
	var rel struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	_ = xml.Unmarshal(data, &rel)
	result := map[string]string{}
	for _, r := range rel.Items {
		if r.Mode != "External" {
			result[r.ID] = r.Target
		}
	}
	return result
}
func resolveOfficePart(base, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(path.Dir(base), target))
}

// Preserve untouched XML children byte-for-byte, including extension namespaces.
type officeXMLChild struct {
	Name  string
	Start int
	End   int
}

func officeXMLChildren(data []byte) ([]officeXMLChild, int, int, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	start := 0
	name := ""
	rootEnd := 0
	closeStart := 0
	var children []officeXMLChild
	for {
		offset := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				rootEnd = int(d.InputOffset())
			}
			if depth == 2 {
				start = offset
				name = t.Name.Local
			}
		case xml.EndElement:
			if depth == 2 {
				children = append(children, officeXMLChild{name, start, int(d.InputOffset())})
			}
			if depth == 1 {
				closeStart = offset
			}
			depth--
		}
	}
	return children, rootEnd, closeStart, nil
}
func mergeOfficeXML(original, updated []byte, owned map[string]bool) ([]byte, error) {
	original = expandEmptyOfficeRoot(original)
	updated = expandEmptyOfficeRoot(updated)
	old, head, tail, err := officeXMLChildren(original)
	if err != nil {
		return nil, err
	}
	fresh, _, _, err := officeXMLChildren(updated)
	if err != nil {
		return nil, err
	}
	replacements := map[string][]byte{}
	for _, c := range fresh {
		if owned[c.Name] {
			replacements[c.Name] = append(replacements[c.Name], updated[c.Start:c.End]...)
		}
	}
	var out bytes.Buffer
	out.Write(original[:head])
	written := map[string]bool{}
	for _, c := range old {
		if !owned[c.Name] {
			out.Write(original[c.Start:c.End])
		} else if !written[c.Name] {
			out.Write(replacements[c.Name])
			written[c.Name] = true
		}
	}
	for _, c := range fresh {
		if owned[c.Name] && !written[c.Name] {
			out.Write(replacements[c.Name])
			written[c.Name] = true
		}
	}
	out.Write(original[tail:])
	value := mergeXMLAttributes(updated, out.Bytes(), map[string]bool{})
	return orderedOfficeChildren(value)
}
func preserveWorkbookParts(original, updated map[string][]byte, dirty map[string]bool, structural bool, before WorkbookEditorDocument, patch WorkbookEditorPatch) (map[string][]byte, error) {
	if len(original) == 0 {
		return updated, nil
	}
	chartsChanged := !reflect.DeepEqual(before.Charts, patch.Charts)
	resourcesChanged := resourceChanged(before.Workbook, patch.Workbook, resourceValidation) || resourceChanged(before.Workbook, patch.Workbook, resourceCondition) || resourceChanged(before.Workbook, patch.Workbook, resourceNotes) || resourceChanged(before.Workbook, patch.Workbook, resourceFilter)
	ownedWorksheet := map[string]bool{"dimension": true, "sheetViews": true, "sheetFormatPr": true, "cols": true, "sheetData": true, "mergeCells": true, "drawing": chartsChanged, "dataValidations": resourceChanged(before.Workbook, patch.Workbook, resourceValidation), "conditionalFormatting": resourceChanged(before.Workbook, patch.Workbook, resourceCondition), "pageSetup": !bytes.Equal(before.Workbook.Custom, patch.Workbook.Custom), "sheetPr": true, "legacyDrawing": resourceChanged(before.Workbook, patch.Workbook, resourceNotes)}
	for _, sheet := range before.Workbook.Sheets {
		current := patch.Workbook.Sheets[sheet.ID]
		if current == nil {
			continue
		}
		oldPath := sheetPartByName(original, sheet.Name)
		newPath := sheetPartByName(updated, current.Name)
		if oldPath == "" || newPath == "" {
			return nil, fmt.Errorf("Cannot preserve worksheet package mapping")
		}
		if !dirty[current.Name] && !structural && !chartsChanged && !resourcesChanged && bytes.Equal(before.Workbook.Custom, patch.Workbook.Custom) {
			updated[newPath] = original[oldPath]
		} else {
			if !structural {
				var e error
				updated[newPath], e = preserveSheetCells(original[oldPath], updated[newPath], before.Workbook, patch.Workbook, sheet.ID)
				if e != nil {
					return nil, e
				}
			}
			merged, err := mergeOfficeXML(original[oldPath], updated[newPath], ownedWorksheet)
			if err != nil {
				return nil, err
			}
			updated[newPath] = merged
		}
	}
	// Package parts not handled by the editor survive even when Excelize omits them.
	for name, data := range original {
		if name == workbookMetadataPart {
			continue
		}
		if _, ok := updated[name]; !ok && !strings.HasPrefix(name, "xl/worksheets/") && !strings.HasPrefix(name, "xl/charts/") {
			updated[name] = data
		}
	}
	for _, name := range []string{"xl/workbook.xml", "xl/styles.xml"} {
		if len(original[name]) == 0 {
			continue
		}
		owned := map[string]bool{"sheets": true, "bookViews": true, "calcPr": true, "definedNames": true}
		if name == "xl/styles.xml" {
			owned = map[string]bool{"numFmts": true, "fonts": true, "fills": true, "borders": true, "cellStyleXfs": true, "cellXfs": true, "cellStyles": true, "dxfs": true}
		}
		merged, err := mergeOfficeXML(original[name], updated[name], owned)
		if err != nil {
			return nil, err
		}
		updated[name] = merged
	}
	return updated, nil
}

func CheckLegacyWorkbookRewrite(name string, original []byte) error {
	ext := strings.ToLower(path.Ext(name))
	if ext != ".xlsx" && ext != ".xlsm" {
		return nil
	}
	parts, err := readOfficeParts(original, "xl/workbook.xml")
	if err != nil {
		return err
	}
	f, err := excelize.OpenReader(bytes.NewReader(original))
	if err != nil {
		return err
	}
	defer f.Close()
	for _, name := range f.GetSheetList() {
		rows, e := f.GetRows(name)
		if e != nil {
			return e
		}
		for r, row := range rows {
			for c := range row {
				axis, _ := excelize.CoordinatesToCellName(c+1, r+1)
				id, e := f.GetCellStyle(name, axis)
				if e != nil {
					return e
				}
				style, e := f.GetStyle(id)
				if e != nil {
					return e
				}
				if font := style.Font; font != nil && (font.Family != "" && font.Family != "Calibri" || font.Size != 0 && font.Size != 11 || font.Strike || font.VertAlign != "") {
					return ErrWorkbookRequiresNativeEditor
				}
				if a := style.Alignment; a != nil && (a.WrapText || a.ShrinkToFit || a.TextRotation != 0 || a.Indent != 0 || a.RelativeIndent != 0) {
					return ErrWorkbookRequiresNativeEditor
				}
				if style.Protection != nil {
					return ErrWorkbookRequiresNativeEditor
				}
			}
		}
	}
	// The reduced legacy model cannot encode geometry, typed values, objects or resources.
	for n, data := range parts {
		if n == workbookMetadataPart || strings.HasPrefix(n, "xl/drawings/") || strings.HasPrefix(n, "xl/charts/") || strings.Contains(n, "vbaProject") || strings.HasPrefix(n, "xl/comments") || strings.HasPrefix(n, "xl/tables/") || strings.HasPrefix(n, "xl/externalLinks/") {
			return ErrWorkbookRequiresNativeEditor
		}
		if strings.HasPrefix(n, "customXml/") || strings.HasPrefix(n, "xl/custom/") || strings.HasPrefix(n, "xl/pivot") || strings.HasPrefix(n, "xl/slicer") || n == "xl/sharedStrings.xml" && bytes.Contains(data, []byte("<r>")) {
			return ErrWorkbookRequiresNativeEditor
		}
		if n == "xl/workbook.xml" && bytes.Contains(data, []byte("<definedName")) {
			return ErrWorkbookRequiresNativeEditor
		}
		if strings.HasPrefix(n, "xl/worksheets/") && strings.HasSuffix(n, ".xml") {
			for _, tag := range []string{"mergeCells", "cols", "pane", "autoFilter", "dataValidations", "conditionalFormatting", "extLst", "hyperlinks", "sheetProtection", "pageSetup", "headerFooter", "drawing"} {
				if bytes.Contains(data, []byte("<"+tag)) {
					return ErrWorkbookRequiresNativeEditor
				}
			}
			var raw struct {
				Rows []struct {
					Height string `xml:"ht,attr"`
					Hidden bool   `xml:"hidden,attr"`
					Cells  []struct {
						Type    string `xml:"t,attr"`
						Value   string `xml:"v"`
						Formula string `xml:"f"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			if bytes.Contains(data, []byte("<r>")) {
				return ErrWorkbookRequiresNativeEditor
			}
			if xml.Unmarshal(data, &raw) != nil {
				return ErrWorkbookRequiresNativeEditor
			}
			for _, r := range raw.Rows {
				if r.Height != "" || r.Hidden {
					return ErrWorkbookRequiresNativeEditor
				}
				for _, c := range r.Cells {
					if c.Formula == "" && c.Value != "" && c.Type != "s" && c.Type != "inlineStr" && c.Type != "str" {
						return ErrWorkbookRequiresNativeEditor
					}
				}
			}
		}
	}
	return nil
}
