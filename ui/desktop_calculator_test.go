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
		"value = calculatorFactorial(value);",
		"result = Number(value.toFixed(10));",
		"result = value;",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop calculator script missing finite-result guard marker %q", marker)
		}
	}
}

func TestDesktopCalculatorUsesReadableKeysAndOrderedLayouts(t *testing.T) {
	t.Parallel()

	script := readDesktopCalculatorScript(t)
	if strings.ContainsRune(script, '\uFFFD') {
		t.Fatal("desktop calculator script contains replacement characters")
	}
	for _, forbidden := range []string{
		"{ key: '?' }",
		"key === '?'",
		"Backspace: '?'",
		"key: '\u00f7'",
		"key: '\u00d7'",
		"key: '\u232b'",
		"key: '\u00b1'",
		"key: '\u03c0'",
		"key: '\u221a'",
		"key: 'x\u00b2'",
		"key: 'x\u02b8'",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("desktop calculator script still contains unreadable key marker %q", forbidden)
		}
	}
	for _, marker := range []string{
		"{ key: 'backspace', label: 'Back' }",
		"else if (key === 'backspace') expression = expression.slice(0, -1);",
		"else if (key === 'negate') expression = expression ? `(-1*(${expression}))` : '-';",
		"else if (key === 'square') expression += '^2';",
		"else if (key === 'power') expression += '^';",
		"else if (['sin', 'cos', 'tan', 'log', 'ln', 'sqrt'].includes(key)) expression += `${key}(`;",
		"const map = { Enter: '=', Backspace: 'backspace', Escape: 'C', '*': '*', '/': '/' };",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop calculator script missing readable key marker %q", marker)
		}
	}

	assertOrderedCalculatorMarkers(t, script, []string{
		"standard: [",
		"{ key: 'C', kind: 'danger' }",
		"{ key: 'CE', kind: 'danger' }",
		"{ key: 'backspace', label: 'Back' }",
		"{ key: '/', kind: 'op' }",
		"{ key: '7' }",
		"{ key: '8' }",
		"{ key: '9' }",
		"{ key: '*', kind: 'op' }",
		"{ key: '4' }",
		"{ key: '5' }",
		"{ key: '6' }",
		"{ key: '-', kind: 'op' }",
		"{ key: '1' }",
		"{ key: '2' }",
		"{ key: '3' }",
		"{ key: '+', kind: 'op' }",
		"{ key: 'negate', label: '+/-' }",
		"{ key: '0' }",
		"{ key: '.' }",
		"{ key: '=', kind: 'eq' }",
	})
	assertOrderedCalculatorMarkers(t, script, []string{
		"scientific: [",
		"{ key: 'sin', kind: 'fn' }",
		"{ key: 'C', kind: 'danger' }",
		"{ key: 'CE', kind: 'danger' }",
		"{ key: 'backspace', label: 'Back' }",
		"{ key: '/', kind: 'op' }",
		"{ key: 'cos', kind: 'fn' }",
		"{ key: '7' }",
		"{ key: '8' }",
		"{ key: '9' }",
		"{ key: '*', kind: 'op' }",
		"{ key: 'tan', kind: 'fn' }",
		"{ key: '4' }",
		"{ key: '5' }",
		"{ key: '6' }",
		"{ key: '-', kind: 'op' }",
		"{ key: 'log', kind: 'fn' }",
		"{ key: '1' }",
		"{ key: '2' }",
		"{ key: '3' }",
		"{ key: '+', kind: 'op' }",
		"{ key: 'ln', kind: 'fn' }",
		"{ key: 'negate', label: '+/-' }",
		"{ key: '0' }",
		"{ key: '.' }",
		"{ key: '=', kind: 'eq' }",
	})
	assertOrderedCalculatorMarkers(t, script, []string{
		"programmer: [",
		"{ key: 'AND', kind: 'fn' }",
		"{ key: 'C', kind: 'danger' }",
		"{ key: 'CE', kind: 'danger' }",
		"{ key: 'backspace', label: 'Back' }",
		"{ key: '/', kind: 'op' }",
		"{ key: 'OR', kind: 'fn' }",
		"{ key: '7' }",
		"{ key: '8' }",
		"{ key: '9' }",
		"{ key: '*', kind: 'op' }",
		"{ key: 'XOR', kind: 'fn' }",
		"{ key: '4' }",
		"{ key: '5' }",
		"{ key: '6' }",
		"{ key: '-', kind: 'op' }",
		"{ key: 'NOT', kind: 'fn' }",
		"{ key: '1' }",
		"{ key: '2' }",
		"{ key: '3' }",
		"{ key: '+', kind: 'op' }",
	})
}

func TestDesktopCalculatorGridKeepsEqualsButtonInRow(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readAllDesktopAppCSS(t), "\r\n", "\n")
	eqRuleStart := strings.Index(css, ".vd-calc-keys button.eq {")
	if eqRuleStart < 0 {
		t.Fatal("desktop calculator CSS missing equals button rule")
	}
	eqRuleEnd := strings.Index(css[eqRuleStart:], "}")
	if eqRuleEnd < 0 {
		t.Fatal("desktop calculator CSS equals button rule is not closed")
	}
	eqRule := css[eqRuleStart : eqRuleStart+eqRuleEnd]
	if strings.Contains(eqRule, "grid-column") {
		t.Fatalf("desktop calculator equals button must not span columns by default: %s", eqRule)
	}
	for _, marker := range []string{
		".vd-calc.scientific-on .vd-calc-keys {\n    grid-template-columns: repeat(5, 1fr);",
		".vd-calc.programmer-on .vd-calc-keys {\n    grid-template-columns: repeat(5, 1fr);",
		".vd-calc.programmer-on .vd-calc-keys button.fn",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop calculator CSS missing grid marker %q", marker)
		}
	}
}

func readDesktopCalculatorScript(t *testing.T) string {
	t.Helper()
	return readDesktopAssetText(t, "js/desktop/main.js")
}

func assertOrderedCalculatorMarkers(t *testing.T, source string, markers []string) {
	t.Helper()

	offset := 0
	for _, marker := range markers {
		index := strings.Index(source[offset:], marker)
		if index < 0 {
			t.Fatalf("desktop calculator layout marker %q missing after offset %d", marker, offset)
		}
		offset += index + len(marker)
	}
}
