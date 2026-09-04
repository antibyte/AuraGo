package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCalculatorInvalidExpressionI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/calculator.js")
	for _, want := range []string{
		"function calculatorErrorMessage(err)",
		"t('desktop.calc_invalid_expression')",
		"resultEl.textContent = calculatorErrorMessage(err)",
		"throw new Error('Invalid expression')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("calculator invalid-expression i18n missing marker %q", want)
		}
	}
	if strings.Count(source, "resultEl.textContent = err.message") != 0 {
		t.Fatal("calculator display must not show raw Invalid expression")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.calc_invalid_expression"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.calc_invalid_expression", path)
		}
		if lang == "de" && got == "Invalid expression" {
			t.Fatalf("%s must not copy the English invalid expression string", path)
		}
	}
}
