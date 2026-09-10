package office

import (
	"bytes"
	"encoding/json"
	"github.com/xuri/excelize/v2"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestWorkbookEditorResourcesPortable(t *testing.T) {
	original := editorFixture(t)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	id := doc.Workbook.SheetOrder[0]
	resources := map[string]interface{}{
		resourceValidation: map[string]interface{}{id: []interface{}{map[string]interface{}{"uid": "rule", "ranges": []EditorRange{{4, 8, 0, 0}}, "type": "list", "formula1": "[\"Yes\",\"No\"]", "showDropDown": true, "errorStyle": 1}}},
		resourceCondition:  map[string]interface{}{id: []interface{}{map[string]interface{}{"cfId": "rule", "ranges": []EditorRange{{1, 2, 1, 1}}, "rule": map[string]interface{}{"type": "highlightCell", "subType": "number", "operator": "greaterThan", "value": 500, "style": map[string]interface{}{"bg": map[string]string{"rgb": "rgb(220,239,230)"}}}}}},
		resourceNotes:      map[string]interface{}{id: map[int]interface{}{1: map[int]interface{}{0: editorNote{ID: "note", Note: "Check rent", Row: 1, Col: 0, Width: 220, Height: 140}}}},
		resourceNames:      map[string]interface{}{"name": editorDefinedName{ID: "name", Name: "Amounts", Ref: "'Budget'!$B$2:$B$3", Scope: "AllDefaultWorkbook"}},
		resourceFilter:     map[string]interface{}{id: map[string]interface{}{"ref": EditorRange{0, 2, 0, 1}, "filterColumns": []interface{}{map[string]interface{}{"colId": 0, "filters": map[string]interface{}{"filters": []string{"Rent", "Food"}}}}}},
	}
	doc.Workbook.Resources = nil
	for name, value := range resources {
		addWorkbookResource(&doc.Workbook, name, value)
	}
	result, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dv, err := f.GetDataValidations("Budget")
	if err != nil || len(dv) != 1 || dv[0].Formula1 != "\"Yes,No\"" {
		t.Fatalf("Validation: %+v %v", dv, err)
	}
	cf, err := f.GetConditionalFormats("Budget")
	if err != nil || len(cf["B2:B3"]) != 1 {
		t.Fatalf("Conditional formatting: %+v %v", cf, err)
	}
	notes, err := f.GetComments("Budget")
	if err != nil || len(notes) != 1 || notes[0].Text != "Check rent" {
		t.Fatalf("Notes: %+v %v", notes, err)
	}
	names := f.GetDefinedName()
	if len(names) != 1 || names[0].Name != "Amounts" {
		t.Fatalf("Names: %+v", names)
	}
	parts, err := readOfficeParts(result, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(parts["[Content_Types].xml"], []byte("/aurago/editor.json")) {
		t.Fatal("Missing metadata content type")
	}
	filter, ok := readEditorFilter(parts[sheetPartByName(parts, "Budget")])
	if !ok || len(filter.Columns) != 1 || len(filter.Columns[0].Filters.Values) != 2 {
		t.Fatalf("Filter: %+v", filter)
	}
	delete(parts, workbookMetadataPart)
	external, _ := writeOfficeParts(parts)
	reopened, err := DecodeEditorWorkbook(external)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{resourceValidation, resourceCondition, resourceNotes, resourceNames, resourceFilter} {
		var value map[string]interface{}
		if err = json.Unmarshal(workbookResource(reopened.Workbook, name), &value); err != nil || len(value) == 0 {
			t.Fatalf("Missing portable %s: %v", name, err)
		}
	}
}
func TestWorkbookEditorNestedPreservation(t *testing.T) {
	original := editorFixture(t)
	parts, _ := readOfficeParts(original, "xl/workbook.xml")
	key := sheetPartByName(parts, "Budget")
	parts[key] = bytes.Replace(parts[key], []byte("<worksheet "), []byte("<worksheet xmlns:test=\"urn:aurago:test\" "), 1)
	parts[key] = bytes.Replace(parts[key], []byte("<c r=\"A2\""), []byte("<c test:opaque=\"stay\" r=\"A2\""), 1)
	parts[key] = bytes.Replace(parts[key], []byte("<row r=\"2\""), []byte("<row test:unknown=\"stay\" r=\"2\""), 1)
	original, _ = writeOfficeParts(parts)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	id := doc.Workbook.SheetOrder[0]
	doc.Workbook.Sheets[id].CellData[1][1].Value = 1400.0
	saved, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := readOfficeParts(saved, "xl/workbook.xml")
	for _, v := range []string{"test:opaque=\"stay\"", "test:unknown=\"stay\""} {
		if !bytes.Contains(out[key], []byte(v)) {
			t.Fatalf("Lost %s", v)
		}
	}
}
func TestWorkbookEditorFiveCharts(t *testing.T) {
	for _, kind := range []string{"column", "bar", "line", "pie", "scatter"} {
		t.Run(kind, func(t *testing.T) {
			original := editorFixture(t)
			doc, err := DecodeEditorWorkbook(original)
			if err != nil {
				t.Fatal(err)
			}
			id := doc.Workbook.SheetOrder[0]
			chart := EditorChart{ID: "chart", Sheet: id, Type: kind, Title: "Editable", Range: "A1:B3", Anchor: "F2", Width: 480, Height: 290, Legend: true, XTitle: "Categories", YTitle: "Amounts", Colors: []string{"#123456"}}
			saved, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: []EditorChart{chart}})
			if err != nil {
				t.Fatal(err)
			}
			parts, _ := readOfficeParts(saved, "xl/workbook.xml")
			delete(parts, workbookMetadataPart)
			external, _ := writeOfficeParts(parts)
			doc, err = DecodeEditorWorkbook(external)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Charts) == 1 {
				if doc.Charts[0].Colors[0] != "#123456" {
					t.Fatalf("Chart color lost: %+v", doc.Charts[0])
				}
				if kind != "pie" && (doc.Charts[0].XTitle == "" || doc.Charts[0].YTitle == "") {
					t.Fatalf("Chart axis titles lost: %+v", doc.Charts[0])
				}
			}
			if len(doc.Charts) != 1 || doc.Charts[0].Type != kind {
				t.Fatalf("Chart round-trip: %+v", doc.Charts)
			}
		})
	}
}

func TestWorkbookEditorStylesGeometryAndFormulaCache(t *testing.T) {
	f := excelize.NewFile()
	style, e := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 12, VertAlign: "superscript"}, Alignment: &excelize.Alignment{TextRotation: 35, Indent: 2}, Protection: &excelize.Protection{Locked: true}})
	if e != nil {
		t.Fatal(e)
	}
	f.SetCellValue("Sheet1", "A1", "original")
	f.SetCellStyle("Sheet1", "A1", "A1", style)
	f.SetRowHeight("Sheet1", 5, 60)
	f.SetColWidth("Sheet1", "F", "F", 30)
	buf, e := f.WriteToBuffer()
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	original := buf.Bytes()
	doc, e := DecodeEditorWorkbook(original)
	if e != nil {
		t.Fatal(e)
	}
	s := doc.Workbook.Sheets[doc.Workbook.SheetOrder[0]]
	s.CellData[0][0].Value = "edited"
	s.CellData[0][0].Style = jsonBytes(map[string]interface{}{"ff": "Arial", "fs": 12, "bl": 1, "ht": 0, "vt": 0, "n": map[string]string{"pattern": "General"}})
	delete(s.RowData, 4)
	delete(s.ColumnData, 5)
	s.CellData[1] = map[int]*EditorCell{0: {Formula: "=SUM(12,13)", Value: float64(25), Type: 2}, 1: {Formula: `="hello"`, Value: "hello", Type: 1}}
	result, e := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if e != nil {
		t.Fatal(e)
	}
	out, e := excelize.OpenReader(bytes.NewReader(result))
	if e != nil {
		t.Fatal(e)
	}
	defer out.Close()
	id, _ := out.GetCellStyle("Sheet1", "A1")
	got, e := out.GetStyle(id)
	if e != nil {
		t.Fatal(e)
	}
	if !got.Font.Bold || got.Font.VertAlign != "superscript" || got.Alignment.TextRotation != 35 || got.Alignment.Indent != 2 || !got.Protection.Locked {
		t.Fatalf("Style dropped: %+v %+v %+v", got.Font, got.Alignment, got.Protection)
	}
	h, _ := out.GetRowHeight("Sheet1", 5)
	if h == 60 {
		t.Fatal("Row resize undo was not saved")
	}
	w, _ := out.GetColWidth("Sheet1", "F")
	if w == 30 {
		t.Fatal("Column resize undo was not saved")
	}
	for cell, want := range map[string]string{"A2": "25", "B2": "hello"} {
		got, e := out.GetCellValue("Sheet1", cell)
		if e != nil || got != want {
			t.Fatalf("Cached %s=%q: %v", cell, got, e)
		}
	}
	csv, e := EncodeEditorCSV(result, "Sheet1")
	if e != nil || !bytes.Contains(csv, []byte("25,hello")) {
		t.Fatalf("CSV values %s: %v", csv, e)
	}
	for _, xml := range []string{`<c r="A1"/>`, `<c r="A1" t="inlineStr"><is><t>hello</t></is></c>`} {
		merged, e := mergeCellXML([]byte(xml), []byte(`<c r="A1" t="s" s="2"><v>7</v></c>`), true)
		if e != nil {
			t.Fatal(e)
		}
		if _, _, _, e = officeXMLChildren(merged); e != nil {
			t.Fatal(e)
		}
		if bytes.Contains([]byte(xml), []byte("inlineStr")) && !bytes.Contains(merged, []byte(`t="inlineStr"`)) {
			t.Fatalf("Lost inline type: %s", merged)
		}
	}
}

func TestWorkbookEditorPicturesMacrosAndUnknownParts(t *testing.T) {
	f, err := excelize.OpenReader(bytes.NewReader(editorFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	var pixels bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	im.Set(1, 1, color.RGBA{255, 0, 0, 255})
	png.Encode(&pixels, im)
	if err = f.AddPictureFromBytes("Budget", "H1", &excelize.Picture{Extension: ".png", File: pixels.Bytes()}); err != nil {
		t.Fatal(err)
	}
	buffer, err := f.WriteToBuffer()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readOfficeParts(buffer.Bytes(), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	parts["xl/vbaProject.bin"] = []byte("opaque VBA fixture: never execute")
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"), []byte("application/vnd.ms-excel.sheet.macroEnabled.main+xml"), 1)
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte("</Types>"), []byte(`<Override PartName="/xl/vbaProject.bin" ContentType="application/vnd.ms-office.vbaProject"/><Override PartName="/customXml/data.xml" ContentType="application/xml"/></Types>`), 1)
	parts["customXml/data.xml"] = []byte(`<vendor xmlns="urn:vendor"><opaque>retain exactly</opaque></vendor>`)
	original, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.StructuralLocked || !WorkbookExtensionMatches(original, ".xlsm") || WorkbookExtensionMatches(original, ".xlsx") {
		t.Fatal("Macro guard missing")
	}
	s := doc.Workbook.Sheets[doc.Workbook.SheetOrder[0]]
	s.CellData[1][1].Value = 1000.0
	doc.Charts = []EditorChart{{ID: "test-chart", Sheet: s.ID, Type: "line", Title: "Amounts", Range: "A1:B3", Anchor: "H15", Width: 400, Height: 250, Legend: true}}
	output, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := readOfficeParts(output, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range parts {
		if strings.HasPrefix(name, "xl/media/") || name == "xl/vbaProject.bin" || name == "customXml/data.xml" {
			if !bytes.Equal(saved[name], data) {
				t.Fatalf("Lost %s", name)
			}
		}
	}
	if !bytes.Contains(saved["[Content_Types].xml"], []byte("/customXml/data.xml")) {
		t.Fatal("Unknown content type lost")
	}
	out, err := excelize.OpenReader(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	pictures, err := out.GetPictures("Budget", "H1")
	if err != nil || len(pictures) != 1 || !bytes.Equal(pictures[0].File, pixels.Bytes()) {
		t.Fatalf("Picture lost %d %v", len(pictures), err)
	}
	if CheckLegacyWorkbookRewrite("file.xlsm", output) == nil {
		t.Fatal("Legacy writer accepts macro workbook")
	}
}

func TestWorkbookEditorViewsFilterAndPrint(t *testing.T) {
	original := editorFixture(t)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	id := doc.Workbook.SheetOrder[0]
	doc.Workbook.Sheets[id].ZoomRatio = 1.4
	doc.Workbook.Sheets[id].ShowGridlines = 0
	doc.Workbook.Sheets[id].TabColor = "#336699"
	for i := range doc.Workbook.Resources {
		if doc.Workbook.Resources[i].Name == resourceFilter {
			doc.Workbook.Resources[i].Data = string(jsonBytes(map[string]interface{}{id: editorFilter{Ref: EditorRange{0, 2, 0, 1}, Hidden: []int{2}}}))
		}
	}
	doc.Workbook.Custom = jsonBytes(map[string]interface{}{"auragoPrint": editorPrint{Sheet: id, Area: "A1:B3", Orientation: "landscape", Paper: "letter", Scaling: "fit", Repeat: 1}})
	saved, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(saved))
	if err != nil {
		t.Fatal(err)
	}
	visible, _ := f.GetRowVisible("Budget", 3)
	f.Close()
	if visible {
		t.Fatal("Filtered row became visible")
	}
	parts, _ := readOfficeParts(saved, "xl/workbook.xml")
	delete(parts, workbookMetadataPart)
	external, _ := writeOfficeParts(parts)
	loaded, err := DecodeEditorWorkbook(external)
	if err != nil {
		t.Fatal(err)
	}
	sheet := loaded.Workbook.Sheets[loaded.Workbook.SheetOrder[0]]
	if sheet.ZoomRatio != 1.4 || sheet.ShowGridlines != 0 || sheet.TabColor != "#336699" {
		t.Fatalf("Portable view: %+v", sheet)
	}
	var custom struct {
		Print editorPrint `json:"auragoPrint"`
	}
	if err = json.Unmarshal(loaded.Workbook.Custom, &custom); err != nil || custom.Print.Orientation != "landscape" || custom.Print.Repeat != 1 || custom.Print.Area != "A1:B3" || custom.Print.Paper != "letter" {
		t.Fatalf("Print: %+v %v", custom, err)
	}
	doc, err = DecodeEditorWorkbook(saved)
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Workbook.Resources {
		if doc.Workbook.Resources[i].Name == resourceFilter {
			doc.Workbook.Resources[i].Data = "{}"
		}
	}
	saved, err = ApplyWorkbookEditorPatch(saved, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts})
	if err != nil {
		t.Fatal(err)
	}
	f, err = excelize.OpenReader(bytes.NewReader(saved))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	visible, _ = f.GetRowVisible("Budget", 3)
	if !visible {
		t.Fatal("Clearing filter did not restore row")
	}
}
