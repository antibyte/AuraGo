package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardLocalWikipediaBadgeIsWired(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("js", "dashboard", "dashboard-widgets.js"))
	if err != nil {
		t.Fatalf("read dashboard-widgets.js: %v", err)
	}
	js := string(raw)
	for _, want := range []string{
		"local_wikipedia: dashIcon('book')",
		"local_wikipedia: t('dashboard.integration_local_wikipedia')",
		"overview.local_wikipedia",
		"t('dashboard.local_wikipedia_update_hint', { date: update.date })",
		"t('dashboard.local_wikipedia_update_title', { date: update.date })",
		`href="/config#local_wikipedia"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("dashboard-widgets.js is missing %q", want)
		}
	}
}

func TestDashboardLocalWikipediaTranslations(t *testing.T) {
	keys := []string{
		"dashboard.integration_local_wikipedia",
		"dashboard.local_wikipedia_update_hint",
		"dashboard.local_wikipedia_update_title",
	}
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	values := map[string]map[string]string{}
	for _, locale := range locales {
		path := filepath.Join("lang", "dashboard", locale+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var bundle map[string]interface{}
		if err := json.Unmarshal(raw, &bundle); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		values[locale] = map[string]string{}
		for _, key := range keys {
			text, ok := bundle[key].(string)
			if !ok || strings.TrimSpace(text) == "" {
				t.Fatalf("%s is missing %s", path, key)
			}
			values[locale][key] = text
		}
		for _, key := range keys[1:] {
			if !strings.Contains(values[locale][key], "{{date}}") {
				t.Fatalf("%s %s must keep the {{date}} placeholder", path, key)
			}
		}
	}
	for _, locale := range locales {
		if locale == "en" {
			continue
		}
		for _, key := range keys {
			if values[locale][key] == values["en"][key] {
				t.Fatalf("%s %s is still English", locale, key)
			}
		}
	}
	if strings.Contains(" "+values["de"]["dashboard.local_wikipedia_update_title"]+" ", " Sie ") {
		t.Fatal("German dashboard hint must use Du")
	}
}
