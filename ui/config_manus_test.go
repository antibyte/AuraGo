package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigManusSectionIsRegistered(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/config/main.js")
	for _, marker := range []string{
		"{ key: 'manus'",
		"config.section.manus.label",
		"manus: { m: 'manus', fn: 'renderManusSection' }",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main module missing Manus marker %q", marker)
		}
	}

	catalogJS := readDesktopAssetText(t, "js/config/catalog.js")
	if !strings.Contains(catalogJS, "'manus-test-btn': { credentialPaths: ['manus.api_key'] }") {
		t.Fatal("Manus connection test must participate in vault-aware credential validation")
	}
}

func TestConfigManusUsesVaultPoliciesAndCatalogSelectors(t *testing.T) {
	t.Parallel()

	manusJS := readDesktopAssetText(t, "cfg/manus.js")
	for _, marker := range []string{
		"async function renderManusSection",
		`data-path="manus.api_key"`,
		"key: 'manus_api_key'",
		"/api/vault/secrets",
		"/api/manus/test",
		"/api/manus/projects",
		"/api/manus/connectors",
		"/api/manus/skills",
		`data-path="manus.default_agent_profile"`,
		"['manus-1.6', 'manus-1.6-lite', 'manus-1.6-max']",
		"data-type=\"array\"",
		"showConfirm(",
		"function manusJSArg",
		"escapeAttr(JSON.stringify(String(value || '')))",
	} {
		if !strings.Contains(manusJS, marker) {
			t.Fatalf("Manus config module missing marker %q", marker)
		}
	}
	if strings.Contains(manusJS, "alert(") || strings.Contains(manusJS, "confirm(") {
		t.Fatal("Manus config module must use AuraGo modals/toasts instead of browser alert/confirm")
	}
}

func TestConfigManusTranslationsAreComplete(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	required := []string{
		"config.manus.enabled",
		"config.manus.api_key",
		"config.manus.test_connection",
		"config.manus.read_only",
		"config.manus.allow_create_tasks",
		"config.manus.allow_send_messages",
		"config.manus.allow_stop_tasks",
		"config.manus.allow_file_uploads",
		"config.manus.allow_file_downloads",
		"config.manus.default_agent_profile",
		"config.manus.allowed_projects",
		"config.manus.allowed_connectors",
		"config.manus.allowed_skills",
		"config.manus.danger_confirm_title",
		"help.manus.read_only",
		"help.manus.allowed_skills",
	}

	for _, lang := range languages {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/config/manus/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse Manus %s translations: %v", lang, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("Manus %s translation missing %q", lang, key)
			}
		}
	}
}

func TestConfigManusGermanUsesPersonalFormAndNativeCharacters(t *testing.T) {
	t.Parallel()

	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/config/manus/de.json")), &values); err != nil {
		t.Fatalf("parse German Manus translations: %v", err)
	}
	joined := strings.Join([]string{
		values["config.manus.danger_confirm"],
		values["help.manus.allowed_skills"],
		values["config.manus.api_key_saved"],
	}, " ")
	if !strings.Contains(joined, "Du") || !strings.Contains(joined, "Schlüssel") || !strings.Contains(joined, "ausgewählt") {
		t.Fatalf("German Manus copy must use personal form and native umlauts: %q", joined)
	}
	if strings.Contains(joined, "Sie ") || strings.Contains(joined, "Schluessel") || strings.Contains(joined, "ausgewaehlt") {
		t.Fatalf("German Manus copy contains disallowed formal/ascii wording: %q", joined)
	}
}

func TestConfigManusHelpContractExists(t *testing.T) {
	t.Parallel()

	helpJSON := readDesktopAssetText(t, "config_help.json")
	var values map[string]any
	if err := json.Unmarshal([]byte(helpJSON), &values); err != nil {
		t.Fatalf("parse config help: %v", err)
	}
	if _, ok := values["manus"]; !ok {
		t.Fatal("config help must document Manus")
	}
}
