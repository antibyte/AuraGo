package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestMediaTabsDoNotDuplicateIconsInLabels(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "media.html")
	for _, marker := range []string{
		`class="media-tab-icon"`,
		`data-i18n="media.tab_images"`,
		`data-i18n="media.tab_audio"`,
		`data-i18n="media.tab_videos"`,
		`data-i18n="media.tab_documents"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("media tabs missing marker %q", marker)
		}
	}

	keys := []string{
		"media.tab_images",
		"media.tab_audio",
		"media.tab_videos",
		"media.tab_documents",
	}
	locales, err := filepath.Glob(filepath.Join("lang", "media", "*.json"))
	if err != nil {
		t.Fatalf("glob media translations: %v", err)
	}
	if len(locales) < 16 {
		t.Fatalf("expected 16 media locales, got %d", len(locales))
	}
	for _, path := range locales {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			label := strings.TrimSpace(values[key])
			if label == "" {
				t.Fatalf("%s missing %s", path, key)
			}
			first := utf8FirstRune(label)
			if unicode.Is(unicode.So, first) || unicode.Is(unicode.Sk, first) {
				t.Fatalf("%s %s still starts with a symbol/emoji %q; icon lives in HTML", path, key, label)
			}
		}
	}
}

func utf8FirstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}
