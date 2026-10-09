package office

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Negative shared-string references must return an error at both the
// dependency and editor boundaries.
func TestWorkbookRejectsInvalidSharedStringIndex(t *testing.T) {
	parts, err := readOfficeParts(editorFixture(t), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	parts[sheetPartByName(parts, "Budget")] = []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>-1</v></c></row></sheetData></worksheet>`)
	data, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	checks := map[string]func() error{
		"GetCellValue":         func() error { _, err := f.GetCellValue("Budget", "A1"); return err },
		"DecodeEditorWorkbook": func() error { _, err := DecodeEditorWorkbook(data); return err },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil || !strings.Contains(err.Error(), "invalid shared string index -1") {
				t.Fatalf("negative shared-string reference must return a validation error: %v", err)
			}
		})
	}
	t.Run("GetRows", func(t *testing.T) {
		// A first-cell validation failure must return no cells and never panic.
		rows, err := f.GetRows("Budget")
		if len(rows) != 0 || (err != nil && !strings.Contains(err.Error(), "invalid shared string index -1")) {
			t.Fatalf("invalid shared-string row was not rejected: rows=%v err=%v", rows, err)
		}
	})
}

func TestWorkbookReadsOutOfOrderCells(t *testing.T) {
	parts, err := readOfficeParts(editorFixture(t), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	// GHSA-8mcq-6wmr-jrjv: the highest column precedes the last cell in a row.
	parts[sheetPartByName(parts, "Budget")] = []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="C1"><v>3</v></c><c r="A1"><v>1</v></c></row></sheetData></worksheet>`)
	data, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	// GetRows does not preserve these coordinates; keep flattening paths blocked.
	checks := map[string]func() error{
		"DecodeWorkbook":             func() error { _, err := DecodeWorkbook("budget.xlsx", data); return err },
		"CheckLegacyWorkbookRewrite": func() error { return CheckLegacyWorkbookRewrite("budget.xlsx", data) },
		"EncodeEditorCSV":            func() error { _, err := EncodeEditorCSV(data, "Budget"); return err },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil || !strings.Contains(err.Error(), "workbook column limit exceeded") {
				t.Fatalf("out-of-order cells must not be flattened: %v", err)
			}
		})
	}
	doc, err := DecodeEditorWorkbook(data)
	if err != nil {
		t.Fatalf("editor workbook decode: %v", err)
	}
	sheet := doc.Workbook.Sheets[doc.Workbook.SheetOrder[0]]
	if sheet.CellData[0][0].Value != float64(1) || sheet.CellData[0][2].Value != float64(3) {
		t.Fatalf("editor misplaced out-of-order cells: %+v", sheet.CellData)
	}
}

func TestWorkbookReadsNegativeStyleIndexes(t *testing.T) {
	for _, tc := range []struct {
		name string
		old  string
		bad  string
	}{
		{"fill", `fillId="2"`, `fillId="-1"`},
		{"font", `fontId="1"`, `fontId="-1"`},
		{"border", `borderId="0"`, `borderId="-1"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts, err := readOfficeParts(editorFixture(t), "xl/workbook.xml")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(parts["xl/styles.xml"], []byte(tc.old)) {
				t.Fatal("style fixture no longer contains the target index")
			}
			// GHSA-5h23-36rv-pm65: invalid style indices must never panic.
			parts["xl/styles.xml"] = bytes.ReplaceAll(parts["xl/styles.xml"], []byte(tc.old), []byte(tc.bad))
			data, err := writeOfficeParts(parts)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeEditorWorkbook(data); err != nil {
				t.Fatalf("editor must safely read a style with a negative %s index: %v", tc.name, err)
			}
		})
	}
}
