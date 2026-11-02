package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The table chrome owns its labels: every locale needs the data, more, read-only and zoom labels plus a
// selection size that names both dimensions. A raw key must never reach the user interface.
func TestDesktopSheetsChromeI18n(t *testing.T) {
	t.Parallel()

	required := map[string][]string{
		"desktop.sheets_data":           nil,
		"desktop.sheets_more":           nil,
		"desktop.sheets_readonly":       nil,
		"desktop.sheets_zoom":           nil,
		"desktop.sheets_selection_size": {"{{rows}}", "{{columns}}"},
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for key, placeholders := range required {
			got, ok := values[key]
			if !ok || strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			if strings.TrimSpace(got) == key {
				t.Fatalf("%s %s must not fall back to the raw key", path, key)
			}
			for _, placeholder := range placeholders {
				if !strings.Contains(got, placeholder) {
					t.Fatalf("%s %s must keep %s", path, key, placeholder)
				}
			}
		}
	}
}
