package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLooperCostI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/looper.js")
	for _, want := range []string{
		"function formatCost(usd, t)",
		"t('desktop.looper_cost_under')",
		"t('desktop.looper_cost', { amount:",
		"t('desktop.looper_tokens', { count:",
		"formatCost(data.estimated_cost_usd, t)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("looper cost i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"return '<$0.01'",
		"return '$' + usd.toFixed",
		"} tok`",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("looper still hardcodes %q", forbidden)
		}
	}

	sip := readDesktopAssetText(t, "js/desktop/apps/sip-phone.js")
	if strings.Contains(sip, "desktop.looper_cost") || strings.Contains(sip, "desktop.looper_tokens") {
		t.Fatal("SIP must not use looper cost keys")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(values["desktop.looper_cost_under"]) == "" {
			t.Fatalf("%s missing non-empty desktop.looper_cost_under", path)
		}
		if !strings.Contains(values["desktop.looper_cost"], "{{amount}}") {
			t.Fatalf("%s desktop.looper_cost must keep {{amount}}", path)
		}
		if !strings.Contains(values["desktop.looper_tokens"], "{{count}}") {
			t.Fatalf("%s desktop.looper_tokens must keep {{count}}", path)
		}
		if lang == "de" {
			if values["desktop.looper_cost_under"] == "<$0.01" {
				t.Fatalf("%s must not copy the English under-cost string", path)
			}
			if values["desktop.looper_tokens"] == "{{count}} tok" {
				t.Fatalf("%s must not copy the English token unit", path)
			}
		}
	}
}
