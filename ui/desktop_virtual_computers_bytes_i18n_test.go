package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopVirtualComputersBytesI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/virtual-computers.js")
	for _, want := range []string{
		"function formatBytes(value, context)",
		"label('desktop.bytes', bytes)",
		"label('desktop.kib', Math.round(bytes / 1024))",
		"label('desktop.mib', (bytes / (1024 * 1024)).toFixed(1))",
		"label('desktop.gib', (bytes / (1024 * 1024 * 1024)).toFixed(1))",
		"formatBytes(volume.size_bytes, c)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("virtual computers bytes i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"} KB`",
		"} MB`",
		"} GB`",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("virtual computers still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.bytes", "desktop.kib", "desktop.mib", "desktop.gib"} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		if lang == "fr" {
			if values["desktop.gib"] == "{{count}} GiB" {
				t.Fatalf("%s must not copy the English gibibyte unit", path)
			}
		}
	}
}
