package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSysworldHudMoneyI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sysworld-hud.js")
	for _, want := range []string{
		"function fmtMoney(v)",
		"formatUptimePhrase('desktop.looper_cost', { amount: amount })",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("sysworld HUD money i18n missing marker %q", want)
		}
	}
	if strings.Contains(source, "desktop.looper_cost_under") {
		t.Fatal("sysworld HUD must not use desktop.looper_cost_under")
	}
	for _, forbidden := range []string{
		"return '$' +",
		"'$' + (typeof v",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("sysworld HUD still hardcodes %q", forbidden)
		}
	}

	looper := readDesktopAssetText(t, "js/desktop/apps/looper.js")
	if !strings.Contains(looper, "t('desktop.looper_cost', { amount:") {
		t.Fatal("looper must keep desktop.looper_cost")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.looper_cost"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.looper_cost", path)
		}
		if !strings.Contains(got, "{{amount}}") {
			t.Fatalf("%s desktop.looper_cost must keep {{amount}}", path)
		}
		if lang == "de" && got == "${{amount}}" {
			t.Fatalf("%s must not copy the English looper cost prefix", path)
		}
	}
}
