package dockerstacks

import "testing"

func TestLintNamesTheSourceLineOfAVariableInAMultiLineScalar(t *testing.T) {
	src := "services:\n" +
		"  web:\n" +
		"    image: nginx:1.27\n" +
		"    restart: always\n" +
		"    command: >\n" +
		"      echo hello\n" +
		"      ${MISSING_A}\n" +
		"    environment:\n" +
		"      LIT: |\n" +
		"        line one\n" +
		"        ${MISSING_B}\n" +
		"      DQ: \"first\n" +
		"        ${MISSING_C}\"\n" +
		"      PL: first\n" +
		"        ${MISSING_D}\n" +
		"      ONE: ${MISSING_E}\n" +
		"      ESC: $$LITERAL then $MISSING_F\n"
	res := Lint(LintInput{Compose: src}, nil)
	want := map[string][2]int{ // variable -> line, column
		"MISSING_A": {7, 7},
		"MISSING_B": {11, 0}, // a literal block keeps its line mapping and has no column
		"MISSING_C": {13, 9},
		"MISSING_D": {15, 9},
		"MISSING_E": {16, 12}, // a single-line scalar keeps the column where the scalar starts
		"MISSING_F": {17, 12},
	}
	got := map[string][2]int{}
	for _, d := range res.Diagnostics {
		if d.Code == "variable_missing" {
			got[d.Params["variable"]] = [2]int{d.Line, d.Column}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("variable_missing = %v, want %v", got, want)
	}
	for name, pos := range want {
		if got[name] != pos {
			t.Errorf("%s reported at line %d column %d, want line %d column %d", name, got[name][0], got[name][1], pos[0], pos[1])
		}
	}
}
