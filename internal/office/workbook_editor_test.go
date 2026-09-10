package office

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func editorFixture(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Budget")
	_ = f.SetCellStr("Budget", "A1", "Category")
	_ = f.SetCellStr("Budget", "B1", "Planned")
	_ = f.SetCellStr("Budget", "A2", "Rent")
	_ = f.SetCellFloat("Budget", "B2", 1250.50, -1, 64)
	_ = f.SetCellStr("Budget", "A3", "Food")
	_ = f.SetCellInt("Budget", "B3", 400)
	_ = f.SetCellFormula("Budget", "B4", "SUM(B2:B3)")
	_ = f.SetCellStr("Budget", "D1", "00123")
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "18344F"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DAE8F7"}}})
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellStyle("Budget", "A1", "B1", style)
	_ = f.SetColWidth("Budget", "A", "A", 22)
	_ = f.SetRowHeight("Budget", 1, 24)
	_ = f.SetPanes("Budget", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_, _ = f.NewSheet("Summary")
	_ = f.SetCellFormula("Summary", "A1", "'Budget'!B4")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
func TestWorkbookEditorRoundTrip(t *testing.T) {
	original := editorFixture(t)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	id := doc.Workbook.SheetOrder[0]
	s := doc.Workbook.Sheets[id]
	if s.CellData[1][1].Value != 1250.5 || s.CellData[0][3].Value != "00123" || s.Freeze.Y != 1 {
		t.Fatalf("typed model: %#v", s)
	}
	s.CellData[1][1].Value = 1300.75
	s.CellData[4] = map[int]*EditorCell{0: {Value: "Total", Type: 1}, 1: {Formula: "=AVERAGE(B2:B3)"}}
	chart := EditorChart{ID: "budget-chart", Sheet: id, Type: "column", Title: "Monthly budget", Range: "A1:B3", Anchor: "F2", Width: 480, Height: 280, Legend: true, Colors: []string{"#3d82bf"}}
	patch := WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: []EditorChart{chart}}
	saved, err := ApplyWorkbookEditorPatch(original, patch)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(saved))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := strings.Join(f.GetSheetList(), ","); got != "Budget,Summary" {
		t.Errorf("sheet order: %s", got)
	}
	value, err := f.GetCellValue("Budget", "B2", excelize.Options{RawCellValue: true})
	if err != nil || value != "1300.75" {
		t.Errorf("numeric roundtrip: %s %v", value, err)
	}
	value, _ = f.GetCellValue("Budget", "D1")
	if value != "00123" {
		t.Errorf("leading zeros lost: %s", value)
	}
	formula, _ := f.GetCellFormula("Summary", "A1")
	if formula != "'Budget'!B4" {
		t.Errorf("cross-sheet formula: %s", formula)
	}
	loaded, err := DecodeEditorWorkbook(saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Charts) != 1 || loaded.Charts[0].Type != "column" {
		t.Fatalf("chart metadata: %+v", loaded.Charts)
	}
	parts, err := readOfficeParts(saved, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	real := false
	for name, data := range parts {
		if strings.HasPrefix(name, "xl/charts/") && bytes.Contains(data, []byte("barChart")) {
			real = true
		}
	}
	if !real {
		t.Fatal("Missing real OOXML chart")
	}
	// The public OOXML representation also works without AuraGo metadata.
	delete(parts, workbookMetadataPart)
	plain, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	external, err := DecodeEditorWorkbook(plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(external.Charts) != 1 || external.Charts[0].Type != "column" || len(external.Charts[0].Series) != 1 {
		t.Errorf("standard chart decoding: %+v", external.Charts)
	}
	again, err := ApplyWorkbookEditorPatch(saved, WorkbookEditorPatch{SchemaVersion: 2, Workbook: loaded.Workbook, Charts: loaded.Charts})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := readOfficeParts(saved, "xl/workbook.xml")
	b, _ := readOfficeParts(again, "xl/workbook.xml")
	for name, data := range a {
		if strings.HasPrefix(name, "xl/charts/") && !bytes.Equal(data, b[name]) {
			t.Errorf("untouched chart rewritten: %s", name)
		}
	}
}
func TestWorkbookEditorPreservationAndLimits(t *testing.T) {
	original := editorFixture(t)
	parts, _ := readOfficeParts(original, "xl/workbook.xml")
	parts["xl/custom/vendor.bin"] = []byte{0, 1, 2, 7, 255}
	sheetPath := sheetPartByName(parts, "Budget")
	extension := []byte(`<extLst><ext uri="urn:vendor"><vendor xmlns="urn:vendor">preserve</vendor></ext></extLst>`)
	parts[sheetPath] = bytes.Replace(parts[sheetPath], []byte("</worksheet>"), append(extension, []byte("</worksheet>")...), 1)
	original, _ = writeOfficeParts(parts)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.StructuralLocked {
		t.Fatal("unknown extension must protect structural references")
	}
	s := doc.Workbook.Sheets[doc.Workbook.SheetOrder[0]]
	s.CellData[1][1].Value = 900.0
	patch := WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts}
	saved, err := ApplyWorkbookEditorPatch(original, patch)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := readOfficeParts(saved, "xl/workbook.xml")
	if !bytes.Equal(result["xl/custom/vendor.bin"], parts["xl/custom/vendor.bin"]) || !bytes.Contains(result[sheetPath], extension) {
		t.Fatal("Unknown content lost")
	}
	patch.Operations = []WorkbookStructureOperation{{Type: "insertRows", Sheet: s.ID, Index: 1, Count: 1}}
	if _, err = ApplyWorkbookEditorPatch(original, patch); err == nil {
		t.Fatal("Unsafe structure edit allowed")
	}
	patch.Operations = nil
	patch.Workbook.Sheets[s.ID].CellData[-1] = map[int]*EditorCell{0: {Value: "bad"}}
	if _, err = ApplyWorkbookEditorPatch(original, patch); err == nil {
		t.Fatal("Negative row accepted")
	}
	if err = CheckLegacyWorkbookRewrite("book.xlsx", original); err == nil {
		t.Fatal("Legacy lossy rewrite allowed")
	}
}
func TestWorkbookEditorNewAndInvalid(t *testing.T) {
	doc, err := DecodeEditorWorkbook(editorFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := ApplyWorkbookEditorPatch(nil, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: []EditorChart{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeEditorWorkbook(data); err != nil {
		t.Fatal(err)
	}
	var patch WorkbookEditorPatch
	if err = json.Unmarshal([]byte(`{"schema_version":2,"workbook":{"sheetOrder":[]}}`), &patch); err != nil {
		t.Fatal(err)
	}
	if _, err = ApplyWorkbookEditorPatch(nil, patch); err == nil {
		t.Fatal("Empty workbook accepted")
	}
}
