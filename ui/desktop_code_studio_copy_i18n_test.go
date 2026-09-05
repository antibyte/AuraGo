package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCodeStudioCopyI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio/agent.js")
	for _, want := range []string{
		"esc(tr('desktop.copy'))",
		"tr('desktop.copied')",
		"tr('desktop.copy')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("code studio copy i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"btn.textContent = 'Copied!'",
		"btn.textContent = 'Copy'",
		"cs-md-code-copy\">Copy<",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("code studio still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.copy", "desktop.copied"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang == "de" && values["desktop.copy"] == "Copy" {
			t.Fatalf("%s must not copy the English copy label", path)
		}
	}
}
