package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLogViewerBytesI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/log-viewer.js")
	for _, want := range []string{
		"function formatSize(bytes, ctx)",
		"label('desktop.bytes', 0)",
		"'desktop.kib'",
		"'desktop.mib'",
		"'desktop.gib'",
		"'desktop.tib'",
		"formatSize(file.size, state.ctx)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("log-viewer bytes i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"+ ' B'",
		"+ ' KiB'",
		"+ ' MiB'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("log-viewer still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.bytes", "desktop.kib", "desktop.mib", "desktop.gib", "desktop.tib"} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		if lang == "fr" && values["desktop.tib"] == "{{count}} TiB" {
			t.Fatalf("%s must not copy the English tebibyte unit", path)
		}
	}
}
