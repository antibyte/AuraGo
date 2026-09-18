package office

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Excelize 2.11.0 includes 93f0b3caed37, despite GO-2026-6452 currently
// listing every version as vulnerable. Keep the upstream exploit covered at
// both the dependency and editor boundaries while the database is corrected.
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
		// GetRows stops at a Columns error; 2.11.0 returns the preceding rows.
		// For this first-cell payload it must return no cells and never panic.
		rows, err := f.GetRows("Budget")
		if len(rows) != 0 || (err != nil && !strings.Contains(err.Error(), "invalid shared string index -1")) {
			t.Fatalf("invalid shared-string row was not rejected: rows=%v err=%v", rows, err)
		}
	})
}
