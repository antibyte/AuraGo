package office

import (
	"strings"
	"testing"
)

func TestEvaluateFormulaArithmetic(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "1+2*3")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "7" {
		t.Fatalf("result = %q, want %q", got, "7")
	}
}

func TestEvaluateFormulaSumRange(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "2"}},
			{{Value: "3"}},
		},
	}
	got, err := EvaluateFormulaForSheet(sheet, "SUM(A1:A2)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "5" {
		t.Fatalf("result = %q, want %q", got, "5")
	}
}

func TestEvaluateFormulaRejectsUnknownFunction(t *testing.T) {
	t.Parallel()

	if _, err := EvaluateFormulaForSheet(Sheet{}, "MEDIAN(A1:A2)"); err == nil {
		t.Fatal("expected unknown function error")
	}
}

func TestEvaluateFormulaRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	for _, formula := range []string{
		"SUM(A1:)",
		"SUM(A2:A1)",
		"SUM(A1:1A)",
	} {
		if _, err := EvaluateFormulaForSheet(Sheet{}, formula); err == nil {
			t.Fatalf("EvaluateFormulaForSheet(%q) expected error", formula)
		}
	}
}

func TestEvaluateFormulaRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, formula := range []string{
		`"text"`,
		"Sheet2!A1",
		"[book.xlsx]Sheet1!A1",
		"1&2",
		strings.Repeat("1", 4097),
	} {
		if _, err := EvaluateFormulaForSheet(Sheet{}, formula); err == nil {
			t.Fatalf("EvaluateFormulaForSheet(%q) expected error", formula)
		}
	}
}

func TestEvaluateFormulaRejectsNonFiniteResults(t *testing.T) {
	t.Parallel()

	if _, err := EvaluateFormulaForSheet(Sheet{}, "1/0"); err == nil {
		t.Fatal("expected non-finite result error")
	}
}

func TestEncodeWorkbookRejectsInvalidFormula(t *testing.T) {
	t.Parallel()

	workbook := Workbook{
		Sheets: []Sheet{{
			Name: "Budget",
			Rows: [][]Cell{
				{{Formula: "MEDIAN(A1:A2)"}},
			},
		}},
	}
	_, err := EncodeWorkbook(workbook)
	if err == nil {
		t.Fatal("expected invalid formula error")
	}
	if !strings.Contains(err.Error(), "invalid formula Budget!A1:") {
		t.Fatalf("error = %q, want contextual invalid formula error", err)
	}
}
