package office

import "testing"

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
