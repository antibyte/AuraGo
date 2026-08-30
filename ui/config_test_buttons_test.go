package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigGenericIntegrationTestRegistryStaysInSync(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	if !strings.Contains(html, "/js/config/integration_actions.js") {
		t.Fatal("config.html must load the generic integration action registry")
	}

	registry := normalizeAssetText(mustReadUIFile(t, "js/config/integration_actions.js"))
	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	checks := []struct {
		section string
		button  string
		path    string
	}{
		{"telegram", "integration-telegram-test-btn", "/api/telegram/test-connection"},
		{"discord", "integration-discord-test-btn", "/api/discord/test"},
		{"rocketchat", "integration-rocketchat-test-btn", "/api/rocketchat/test"},
		{"home_assistant", "integration-home-assistant-test-btn", "/api/home-assistant/test"},
		{"proxmox", "integration-proxmox-test-btn", "/api/proxmox/test"},
		{"s3", "integration-s3-test-btn", "/api/s3/test"},
		{"frigate", "integration-frigate-test-btn", "/api/frigate/test"},
		{"ansible", "integration-ansible-test-btn", "/api/ansible/test"},
	}
	for _, check := range checks {
		for _, marker := range []string{check.section, check.button, check.path} {
			if !strings.Contains(registry, marker) {
				t.Fatalf("integration_actions.js missing %q", marker)
			}
		}
		if !strings.Contains(catalog, "'"+check.button+"'") {
			t.Fatalf("catalog.js missing action rule for %s", check.button)
		}
	}
	for _, stale := range []string{"'proxmox-test-btn'", "'s3-test-btn'"} {
		if strings.Contains(catalog, stale) {
			t.Fatalf("catalog.js still contains stale action rule %s", stale)
		}
	}
	for _, marker := range []string{"integrationActions.render(key)", "integrationActions.bind(key)"} {
		if !strings.Contains(normalizeAssetText(mustReadUIFile(t, "js/config/main.js")), marker) {
			t.Fatalf("main.js missing generic action hook %q", marker)
		}
	}
}

func TestConfigGenericIntegrationTestTranslationsCoverAllLocales(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.integration_test.title",
		"config.integration_test.description",
		"config.integration_test.button",
		"config.integration_test.testing",
		"config.integration_test.success",
		"config.integration_test.failure",
	}
	for _, locale := range locales {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/common/"+locale+".json"), &values); err != nil {
			t.Fatalf("parse %s translations: %v", locale, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing translation %q", locale, key)
			}
		}
	}
}
