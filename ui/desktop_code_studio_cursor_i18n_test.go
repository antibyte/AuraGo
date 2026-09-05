package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCodeStudioCursorI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio/core.js")
	for _, want := range []string{
		"tr('codeStudio.cursorPosition', undefined, { line, column: col })",
		"tr('desktop.bytes', undefined, { count: n })",
		"tr('desktop.kib', undefined, { count:",
		"tr('desktop.mib', undefined, { count:",
		"tr('desktop.gib', undefined, { count:",
		"tr('desktop.tib', undefined, { count:",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("code studio cursor i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"'Ln ' + line + ', Col '",
		"+ ' KiB'",
		"+ ' MiB'",
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
		got := values["codeStudio.cursorPosition"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty codeStudio.cursorPosition", path)
		}
		if !strings.Contains(got, "{{line}}") || !strings.Contains(got, "{{column}}") {
			t.Fatalf("%s codeStudio.cursorPosition must keep {{line}} and {{column}}", path)
		}
		if lang == "de" && got == "Ln {{line}}, Col {{column}}" {
			t.Fatalf("%s must not copy the English cursor position string", path)
		}
	}
}
