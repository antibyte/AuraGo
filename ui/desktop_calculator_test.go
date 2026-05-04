package ui

import (
	"strings"
	"testing"
)

func TestDesktopCalculatorDoesNotUseFunctionEval(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop main script missing from embedded UI: %v", err)
	}
	script := string(data)
	if strings.Contains(script, "Function(") {
		t.Fatal("desktop calculator must use a safe parser instead of Function eval")
	}
	for _, marker := range []string{
		"function tokenizeCalculatorExpression",
		"function parseCalculatorExpression",
		"function evaluateCalculatorExpression",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop calculator script missing safe parser marker %q", marker)
		}
	}
}
