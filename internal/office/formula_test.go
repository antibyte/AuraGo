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

func TestEvaluateFormulaIgnoresTextInAggregateRanges(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "Header"}},
			{{Value: "2"}},
			{{Value: "3"}},
		},
	}
	got, err := EvaluateFormulaForSheet(sheet, "SUM(A1:A3)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "5" {
		t.Fatalf("result = %q, want %q", got, "5")
	}
}

func TestEvaluateFormulaAverageAliases(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "Header"}},
			{{Value: "2"}},
			{{Value: ""}},
			{{Value: "text"}},
			{{Value: "4"}},
		},
	}
	for _, formula := range []string{"AVG(A2:A5)", "AVERAGE(A2:A5)"} {
		got, err := EvaluateFormulaForSheet(sheet, formula)
		if err != nil {
			t.Fatalf("EvaluateFormulaForSheet(%q): %v", formula, err)
		}
		if got != "3" {
			t.Fatalf("EvaluateFormulaForSheet(%q) = %q, want %q", formula, got, "3")
		}
	}
}

func TestEvaluateFormulaCountRangeCountsNumericCells(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "Header"}},
			{{Value: "2"}},
			{{Value: ""}},
			{{Value: "text"}},
			{{Value: "4"}},
		},
	}
	got, err := EvaluateFormulaForSheet(sheet, "COUNT(A1:A5)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "2" {
		t.Fatalf("result = %q, want %q", got, "2")
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

func TestEvaluateFormulaRejectsOutOfBoundsRefsWithoutPanic(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "1"}},
		},
	}
	for _, formula := range []string{
		"XFE1",
		"A1048577",
		"ZZZZZZZZZZZZZZ1",
		"SUM(XFD1:XFE1)",
		"SUM(A1048576:A1048577)",
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("EvaluateFormulaForSheet(%q) panicked: %v", formula, recovered)
				}
			}()
			if _, err := EvaluateFormulaForSheet(sheet, formula); err == nil {
				t.Fatalf("EvaluateFormulaForSheet(%q) expected error", formula)
			}
		}()
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
