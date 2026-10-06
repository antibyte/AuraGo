package office

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestWorkbookPreflightRejectsSparseAllocationBeforeDecode(t *testing.T) {
	for _, body := range []string{
		`<worksheet><sheetData><row r="100001"><c r="A100001"/></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c r="XFE1"/></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c r="A2"/></row></sheetData></worksheet>`,
	} {
		if err := validateWorkbookAllocation(map[string][]byte{"xl/worksheets/sheet1.xml": []byte(body)}); err == nil {
			t.Fatal("oversized/malformed sheet accepted")
		}
	}
	var body strings.Builder
	body.WriteString("<worksheet><sheetData>")
	for i := 1; i <= 62; i++ {
		fmt.Fprintf(&body, `<row r="%d"><c r="XFD%d"/></row>`, i, i)
	}
	body.WriteString("</sheetData></worksheet>")
	if err := validateWorkbookAllocation(map[string][]byte{"xl/worksheets/sheet1.xml": []byte(body.String())}); err == nil {
		t.Fatal("expanded cell budget accepted")
	}
}

func TestWorkbookPreflightCapsAggregateExpandedRowSlots(t *testing.T) {
	parts := make(map[string][]byte, 11)
	for i := 1; i <= 10; i++ {
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", i)] = []byte(`<worksheet><sheetData><row r="100000"><c r="A100000"><v>1</v></c></row></sheetData></worksheet>`)
	}
	parts["xl/sharedStrings.xml"] = []byte(`<sst><si><t></t></si></sst>`)
	// GetRows omits a distant row whose only shared-string cell resolves to an
	// empty string, so it should not consume the aggregate expanded-row budget.
	parts["xl/worksheets/sheet11.xml"] = []byte(`<worksheet><sheetData><row r="100000"><c r="A100000" t="s"><v>0</v></c></row></sheetData></worksheet>`)
	if err := validateWorkbookAllocation(parts); err != nil {
		t.Fatalf("ten populated 100,000-row sheets plus an empty sheet: %v", err)
	}

	parts["xl/worksheets/sheet11.xml"] = []byte(`<worksheet><sheetData><row r="100000"><c r="A100000"><v>1</v></c></row></sheetData></worksheet>`)
	if err := validateWorkbookAllocation(parts); err == nil || !strings.Contains(err.Error(), "workbook expanded row limit exceeded") {
		t.Fatalf("eleven populated 100,000-row sheets error = %v", err)
	}

	// These entry points must reject using the XML preflight before Excelize
	// opens the package or either path materializes the expanded rows.
	parts["[Content_Types].xml"] = []byte(`<Types/>`)
	parts["xl/workbook.xml"] = []byte(`<workbook/>`)
	data, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckLegacyWorkbookRewrite("book.xlsx", data); err == nil || !strings.Contains(err.Error(), "workbook expanded row limit exceeded") {
		t.Fatalf("legacy rewrite aggregate row error = %v", err)
	}
	if _, err := EncodeEditorCSV(data, "Sheet1"); err == nil || !strings.Contains(err.Error(), "workbook expanded row limit exceeded") {
		t.Fatalf("CSV aggregate row error = %v", err)
	}
}

func TestLegacyWorkbookRewriteAndCSVPreflightSparseRows(t *testing.T) {
	parts, err := readOfficeParts(editorFixture(t), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	sheet := sheetPartByName(parts, "Budget")
	parts[sheet] = bytes.Replace(parts[sheet], []byte("</sheetData>"), []byte(`<row r="100001"><c r="A100001"><v>overflow</v></c></row></sheetData>`), 1)
	sparse, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckLegacyWorkbookRewrite("book.xlsx", sparse); err == nil || !strings.Contains(err.Error(), "workbook row limit exceeded") {
		t.Fatalf("legacy rewrite sparse row error = %v", err)
	}
	if _, err := EncodeEditorCSV(sparse, "Budget"); err == nil || !strings.Contains(err.Error(), "workbook row limit exceeded") {
		t.Fatalf("CSV sparse row error = %v", err)
	}
}
