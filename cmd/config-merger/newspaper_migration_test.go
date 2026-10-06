package main

import "testing"

func TestMergeUserConfigKeepsNewspaperChoicesOptIn(t *testing.T) {
	template, err := parseYAMLMap(`newspaper:
  enabled: false
  budget_mode: auto
  overview_sources: []
  max_minutes: 30
`)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := parseYAMLMap(`newspaper:
  enabled: true
  max_minutes: 30
`)
	if err != nil {
		t.Fatal(err)
	}
	result := mergeUserConfig(template, legacy)
	if !result.needsWrite() {
		t.Fatal("legacy config did not persist compatibility defaults")
	}
	newspaperMap, _ := asStringMap(result.merged["newspaper"])
	if newspaperMap["budget_mode"] != "fixed" {
		t.Fatalf("legacy budget mode = %#v, want fixed", newspaperMap["budget_mode"])
	}
	sources, ok := newspaperMap["overview_sources"].([]interface{})
	if !ok || len(sources) != 0 {
		t.Fatalf("legacy overview sources = %#v, want []", newspaperMap["overview_sources"])
	}

	optedIn, err := parseYAMLMap(`newspaper:
  enabled: true
  budget_mode: auto
  overview_sources: [google_news]
`)
	if err != nil {
		t.Fatal(err)
	}
	result = mergeUserConfig(template, optedIn)
	newspaperMap, _ = asStringMap(result.merged["newspaper"])
	if newspaperMap["budget_mode"] != "auto" {
		t.Fatalf("explicit budget mode = %#v, want auto", newspaperMap["budget_mode"])
	}
	sources, ok = newspaperMap["overview_sources"].([]interface{})
	if !ok || len(sources) != 1 || sources[0] != "google_news" {
		t.Fatalf("explicit overview sources = %#v", newspaperMap["overview_sources"])
	}
}

func TestRecoverCorruptedConfigKeepsNewspaperChoicesOptIn(t *testing.T) {
	template, err := parseYAMLMap(`newspaper:
  budget_mode: auto
  overview_sources: [google_news]
`)
	if err != nil {
		t.Fatal(err)
	}
	legacyNewspaper, err := parseYAMLMap("newspaper:\n  enabled: true\n")
	if err != nil {
		t.Fatal(err)
	}
	merged := recoverCorruptedConfig(template, legacyNewspaper)
	newspaperMap, _ := asStringMap(merged["newspaper"])
	if newspaperMap["budget_mode"] != "fixed" {
		t.Fatalf("recovered budget mode = %#v, want fixed", newspaperMap["budget_mode"])
	}
	if sources, ok := newspaperMap["overview_sources"].([]interface{}); !ok || len(sources) != 0 {
		t.Fatalf("recovered overview sources = %#v, want []", newspaperMap["overview_sources"])
	}

	// An entirely unsalvageable pre-existing config still receives safe legacy
	// Newspaper choices; a fresh install follows the separate template-copy path.
	merged = recoverCorruptedConfig(template, nil)
	newspaperMap, _ = asStringMap(merged["newspaper"])
	if newspaperMap["budget_mode"] != "fixed" {
		t.Fatalf("unsalvageable config budget mode = %#v, want fixed", newspaperMap["budget_mode"])
	}
	if sources, ok := newspaperMap["overview_sources"].([]interface{}); !ok || len(sources) != 0 {
		t.Fatalf("unsalvageable config overview sources = %#v, want []", newspaperMap["overview_sources"])
	}
}
