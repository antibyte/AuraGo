package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRulesSectionContract(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/config/main.js")
	for _, marker := range []string{
		"config.section.rules.label",
		"rules: { m: 'rules', fn: 'renderRulesSection' }",
		"CONFIG_ASSET_VERSION = '18'",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main JS missing rules marker %q", marker)
		}
	}

	rulesJS := readDesktopAssetText(t, "cfg/rules.js")
	for _, marker := range []string{
		"async function renderRulesSection",
		"/api/config/rules",
		"rules-design-input",
		"rules-rule-input",
		"showConfirm(",
	} {
		if !strings.Contains(rulesJS, marker) {
			t.Fatalf("rules config module missing marker %q", marker)
		}
	}
	if strings.Contains(rulesJS, "alert(") {
		t.Fatal("rules config module must use modals/toasts instead of alert()")
	}
}

func TestConfigRulesTranslationsExistInAllLocales(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("lang", "config", "sections", "*.json"))
	if err != nil {
		t.Fatalf("glob section translations: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all config section language files, got %d", len(files))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for _, key := range []string{
			"config.section.rules.label",
			"config.section.rules.desc",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}
}
