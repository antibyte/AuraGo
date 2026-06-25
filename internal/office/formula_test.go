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

	if _, err := EvaluateFormulaForSheet(Sheet{}, "NOSUCHFUNC(A1:A2)"); err == nil {
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
				{{Formula: "NOSUCHFUNC(A1:A2)"}},
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

func TestEvaluateFormulaStringLiterals(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, `CONCAT("hello"," ","world")`)
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("result = %q, want %q", got, "hello world")
	}
}

func TestEvaluateFormulaIf(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "IF(1>0,10,20)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "10" {
		t.Fatalf("result = %q, want %q", got, "10")
	}

	got, err = EvaluateFormulaForSheet(Sheet{}, "IF(0>1,10,20)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "20" {
		t.Fatalf("result = %q, want %q", got, "20")
	}
}

func TestEvaluateFormulaComparisons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		formula string
		want    string
	}{
		{"IF(5>3,1,0)", "1"},
		{"IF(3>5,1,0)", "0"},
		{"IF(5=5,1,0)", "1"},
		{"IF(5<>5,1,0)", "0"},
		{"IF(3<=5,1,0)", "1"},
		{"IF(5>=5,1,0)", "1"},
	}
	for _, tt := range tests {
		got, err := EvaluateFormulaForSheet(Sheet{}, tt.formula)
		if err != nil {
			t.Fatalf("EvaluateFormulaForSheet(%q): %v", tt.formula, err)
		}
		if got != tt.want {
			t.Fatalf("EvaluateFormulaForSheet(%q) = %q, want %q", tt.formula, got, tt.want)
		}
	}
}

func TestEvaluateFormulaStringFunctions(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "Hello World"}},
		},
	}
	tests := []struct {
		formula string
		want    string
	}{
		{`UPPER("hello")`, "HELLO"},
		{`LOWER("HELLO")`, "hello"},
		{`LEN("hello")`, "5"},
		{`LEFT("hello",3)`, "hel"},
		{`RIGHT("hello",3)`, "llo"},
		{`MID("hello",2,3)`, "ell"},
		{`TRIM("  hi  ")`, "hi"},
	}
	for _, tt := range tests {
		got, err := EvaluateFormulaForSheet(sheet, tt.formula)
		if err != nil {
			t.Fatalf("EvaluateFormulaForSheet(%q): %v", tt.formula, err)
		}
		if got != tt.want {
			t.Fatalf("EvaluateFormulaForSheet(%q) = %q, want %q", tt.formula, got, tt.want)
		}
	}
}

func TestEvaluateFormulaLogical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		formula string
		want    string
	}{
		{"AND(1,1,1)", "1"},
		{"AND(1,0,1)", "0"},
		{"OR(0,0,1)", "1"},
		{"OR(0,0,0)", "0"},
		{"NOT(0)", "1"},
		{"NOT(1)", "0"},
	}
	for _, tt := range tests {
		got, err := EvaluateFormulaForSheet(Sheet{}, tt.formula)
		if err != nil {
			t.Fatalf("EvaluateFormulaForSheet(%q): %v", tt.formula, err)
		}
		if got != tt.want {
			t.Fatalf("EvaluateFormulaForSheet(%q) = %q, want %q", tt.formula, got, tt.want)
		}
	}
}

func TestEvaluateFormulaMedian(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "MEDIAN(1,3,5)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "3" {
		t.Fatalf("result = %q, want %q", got, "3")
	}

	got, err = EvaluateFormulaForSheet(Sheet{}, "MEDIAN(1,2,3,4)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "2.5" {
		t.Fatalf("result = %q, want %q", got, "2.5")
	}
}

func TestEvaluateFormulaTextJoin(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, `TEXTJOIN(",","a","b","c")`)
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "a,b,c" {
		t.Fatalf("result = %q, want %q", got, "a,b,c")
	}
}

func TestEvaluateFormulaRound(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "ROUND(3.14159,2)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "3.14" {
		t.Fatalf("result = %q, want %q", got, "3.14")
	}
}

func TestEvaluateFormulaCeilFloor(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "CEILING(3.2)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "4" {
		t.Fatalf("result = %q, want %q", got, "4")
	}

	got, err = EvaluateFormulaForSheet(Sheet{}, "FLOOR(3.7)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "3" {
		t.Fatalf("result = %q, want %q", got, "3")
	}
}

func TestEvaluateFormulaConcat(t *testing.T) {
	t.Parallel()

	sheet := Sheet{
		Name: "Sheet1",
		Rows: [][]Cell{
			{{Value: "Hello"}},
			{{Value: "World"}},
		},
	}
	got, err := EvaluateFormulaForSheet(sheet, `CONCAT(A1," ",A2)`)
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got != "Hello World" {
		t.Fatalf("result = %q, want %q", got, "Hello World")
	}
}

func TestEvaluateFormulaStdev(t *testing.T) {
	t.Parallel()

	got, err := EvaluateFormulaForSheet(Sheet{}, "STDEV(2,4,4,4,5,5,7,9)")
	if err != nil {
		t.Fatalf("EvaluateFormulaForSheet: %v", err)
	}
	if got == "" || got == "#ERR" {
		t.Fatalf("result = %q, want a valid number", got)
	}
}
