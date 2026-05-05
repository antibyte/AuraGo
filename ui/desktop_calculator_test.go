package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDesktopCalculatorDoesNotUseFunctionEval(t *testing.T) {
	t.Parallel()

	script := readDesktopCalculatorScript(t)
	for _, forbidden := range []struct {
		name    string
		pattern string
	}{
		{name: "Function call", pattern: `\bFunction\s*\(`},
		{name: "new Function", pattern: `\bnew\s+Function\b`},
		{name: "eval", pattern: `\beval\s*\(`},
		{name: "globalThis.Function", pattern: `\bglobalThis\s*\.\s*Function\b`},
		{name: "window.Function", pattern: `\bwindow\s*\.\s*Function\b`},
		{name: "window bracket Function", pattern: `\bwindow\s*\[\s*['"]Function['"]\s*\]`},
		{name: "constructor constructor", pattern: `\bconstructor\s*\.\s*constructor\b`},
	} {
		if regexp.MustCompile(forbidden.pattern).MatchString(script) {
			t.Fatalf("desktop calculator must not use dynamic evaluation pattern %s", forbidden.name)
		}
	}
	for _, marker := range []string{
		"function tokenizeCalculatorExpression",
		"function parseCalculatorExpression",
		"function evaluateCalculatorExpression",
		"case 'sin':",
		"case 'cos':",
		"case 'tan':",
		"case 'sqrt':",
		"case 'log':",
		"case 'abs':",
		"calculatorFactorial(value)",
		"if (name === 'pi') return Math.PI;",
		"if (name === 'e') return Math.E;",
		"peek().value === '*' || peek().value === '/' || peek().value === '%'",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop calculator script missing safe parser marker %q", marker)
		}
	}
}

func TestDesktopCalculatorRejectsNonFiniteResults(t *testing.T) {
	t.Parallel()

	script := readDesktopCalculatorScript(t)
	for _, marker := range []string{
		"function ensureFiniteCalculatorResult(value)",
		"return ensureFiniteCalculatorResult(value);",
		"if (!Number.isFinite(value)) throw new Error('Invalid expression');",
		"if (value > 170) throw new Error('Invalid expression');",
		"value = operator === '!' ? calculatorFactorial(value) : Math.pow(value, 2);",
		"result = Number(value.toFixed(10));",
		"result = value;",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop calculator script missing finite-result guard marker %q", marker)
		}
	}
}

func readDesktopCalculatorScript(t *testing.T) string {
	t.Helper()
	data, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop main script missing from embedded UI: %v", err)
	}
	return string(data)
}
