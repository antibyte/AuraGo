package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Config section edits the administrator-owned read allowlist as one "tool: op, op"
// line per tool, and every locale names the control (audit 4.16).
func TestDetectiveConfigAllowlistEditor(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("cfg", "detective.js"))
	if err != nil {
		t.Fatal(err)
	}
	app := string(b)
	for _, want := range []string{
		"function detectiveAllowlistText(map)",
		"function detectiveAllowlistMap(text)",
		"function syncDetectiveAllowlist(area)",
		"configData.detective.extra_read_operations = detectiveAllowlistMap(area.value)",
		"setDirty(true)",
		"data-role=\"detective-allowlist\"",
		"label('extra_read_help')",
		"label('extra_read_format')",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("cfg/detective.js lacks %q", want)
		}
	}
	labels := map[string]string{
		"extra_read_operations": "Approved",
		"extra_read_help":       "Adds",
		"extra_read_format":     "One line",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		raw, err := os.ReadFile(filepath.Join("lang", "config", "detective", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for key, hint := range labels {
			if !strings.Contains(text, `"config.detective.`+key+`"`) {
				t.Fatalf("%s.json lacks config.detective.%s", lang, key)
			}
			_ = hint
		}
	}
}
