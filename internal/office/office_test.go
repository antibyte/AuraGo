package office

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestDOCXTextRoundTrip(t *testing.T) {
	t.Parallel()

	want := Document{
		Title: "Project Notes",
		Text:  "Hello AuraGo\nThis is a DOCX round trip.",
	}
	data, err := EncodeDOCX(want)
	if err != nil {
		t.Fatalf("EncodeDOCX: %v", err)
	}
	got, err := DecodeDocument("notes.docx", data)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if got.Title != want.Title {
		t.Fatalf("title = %q, want %q", got.Title, want.Title)
	}
	if got.Text != want.Text {
		t.Fatalf("text = %q, want %q", got.Text, want.Text)
	}
	if len(got.Delta.Ops) == 0 {
		t.Fatal("expected Quill-compatible delta operations")
	}
}

func TestDOCXPreservesBasicHTMLFormatting(t *testing.T) {
	t.Parallel()

	want := Document{
		Title: "Formatted",
		Text:  "Heading\nBold and italic",
		HTML:  `<h1>Heading</h1><p><strong>Bold</strong> and <em>italic</em></p>`,
	}
	data, err := EncodeDOCX(want)
	if err != nil {
		t.Fatalf("EncodeDOCX: %v", err)
	}
	documentXML := readDocxPart(t, data, "word/document.xml")
	for _, marker := range []string{`<w:pStyle w:val="Heading1"/>`, "<w:b/>", "<w:i/>"} {
		if !strings.Contains(documentXML, marker) {
			t.Fatalf("document.xml missing rich text marker %q:\n%s", marker, documentXML)
		}
	}
	got, err := DecodeDocument("formatted.docx", data)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	for _, marker := range []string{"<h1>Heading</h1>", "<strong>Bold</strong>", "<em>italic</em>"} {
		if !strings.Contains(got.HTML, marker) {
			t.Fatalf("decoded HTML missing marker %q: %q", marker, got.HTML)
		}
	}
}

func TestWorkbookXLSXRoundTrip(t *testing.T) {
	t.Parallel()

	want := Workbook{
		Sheets: []Sheet{{
			Name: "Budget",
			Rows: [][]Cell{
				{{Value: "Item"}, {Value: "Amount"}},
				{{Value: "Coffee"}, {Value: "12.50"}},
				{{Value: "Total"}, {Formula: "SUM(B2:B2)"}},
			},
		}},
	}
	data, err := EncodeWorkbook(want)
	if err != nil {
		t.Fatalf("EncodeWorkbook: %v", err)
	}
	got, err := DecodeWorkbook("budget.xlsx", data)
	if err != nil {
		t.Fatalf("DecodeWorkbook: %v", err)
	}
	if len(got.Sheets) != 1 {
		t.Fatalf("sheets = %+v", got.Sheets)
	}
	rows := got.Sheets[0].Rows
	if got.Sheets[0].Name != "Budget" || rows[1][0].Value != "Coffee" || rows[2][1].Formula != "SUM(B2:B2)" {
		t.Fatalf("workbook = %+v", got)
	}
}

func TestEncodeWorkbookNormalizesAvgFormula(t *testing.T) {
	t.Parallel()

	workbook := Workbook{
		Sheets: []Sheet{{
			Name: "Budget",
			Rows: [][]Cell{
				{{Value: "2"}},
				{{Value: "4"}},
				{{Formula: "AVG(A1:A2)"}},
				{{Formula: "SUM(AVG(A1:A2))"}},
				{{Formula: "AVG (A1:A2)"}},
			},
		}},
	}
	data, err := EncodeWorkbook(workbook)
	if err != nil {
		t.Fatalf("EncodeWorkbook: %v", err)
	}
	got, err := DecodeWorkbook("budget.xlsx", data)
	if err != nil {
		t.Fatalf("DecodeWorkbook: %v", err)
	}
	if got.Sheets[0].Rows[2][0].Formula != "AVERAGE(A1:A2)" {
		t.Fatalf("formula = %q, want %q", got.Sheets[0].Rows[2][0].Formula, "AVERAGE(A1:A2)")
	}
	if got.Sheets[0].Rows[3][0].Formula != "SUM(AVERAGE(A1:A2))" {
		t.Fatalf("nested formula = %q, want %q", got.Sheets[0].Rows[3][0].Formula, "SUM(AVERAGE(A1:A2))")
	}
	if got.Sheets[0].Rows[4][0].Formula != "AVERAGE (A1:A2)" {
		t.Fatalf("spaced formula = %q, want %q", got.Sheets[0].Rows[4][0].Formula, "AVERAGE (A1:A2)")
	}
	if normalized := normalizeFormulaForXLSX("A1+AVGA(A1:A2)+AVG1+AVG(A1:A2)"); normalized != "A1+AVGA(A1:A2)+AVG1+AVERAGE(A1:A2)" {
		t.Fatalf("boundary normalization = %q", normalized)
	}
}

func TestWorkbookCSVRoundTrip(t *testing.T) {
	t.Parallel()

	got, err := DecodeWorkbook("people.csv", []byte("Name,Role\nAndi,Admin\n"))
	if err != nil {
		t.Fatalf("DecodeWorkbook csv: %v", err)
	}
	if len(got.Sheets) != 1 || got.Sheets[0].Rows[1][1].Value != "Admin" {
		t.Fatalf("csv workbook = %+v", got)
	}
	data, err := EncodeCSV(got, "Sheet1")
	if err != nil {
		t.Fatalf("EncodeCSV: %v", err)
	}
	if string(data) != "Name,Role\nAndi,Admin\n" {
		t.Fatalf("csv = %q", string(data))
	}
}

func TestEncodeCSVNeutralizesDangerousCells(t *testing.T) {
	t.Parallel()

	workbook := Workbook{Sheets: []Sheet{{
		Name: "Sheet1",
		Rows: [][]Cell{{
			{Value: "=cmd"},
			{Value: " +cmd"},
			{Value: "-cmd"},
			{Value: "@cmd"},
			{Formula: "SUM(A2:A2)"},
		}},
	}}}
	data, err := EncodeCSV(workbook, "Sheet1")
	if err != nil {
		t.Fatalf("EncodeCSV: %v", err)
	}
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	want := []string{"'=cmd", "' +cmd", "'-cmd", "'@cmd", "'=SUM(A2:A2)"}
	if strings.Join(records[0], "|") != strings.Join(want, "|") {
		t.Fatalf("csv row = %#v, want %#v", records[0], want)
	}
}

func TestEncodeCSVValidatesFormulas(t *testing.T) {
	t.Parallel()

	workbook := Workbook{Sheets: []Sheet{{
		Name: "Sheet1",
		Rows: [][]Cell{{{Formula: "MEDIAN(A1:A2)"}}},
	}}}
	_, err := EncodeCSV(workbook, "Sheet1")
	if err == nil {
		t.Fatal("expected invalid formula error")
	}
	if !strings.Contains(err.Error(), "invalid formula Sheet1!A1:") {
		t.Fatalf("error = %q, want contextual invalid formula error", err)
	}
}

func readDocxPart(t *testing.T, data []byte, name string) string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open docx: %v", err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		raw, err := readZipFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(raw)
	}
	t.Fatalf("docx part %s missing", name)
	return ""
}
