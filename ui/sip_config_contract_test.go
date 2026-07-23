package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSIPConfigUIUsesSavedStateAndMaskedSecret(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "sip.js"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, marker := range []string{
		"/api/sip/config", "/api/sip/providers", "/api/sip/setup", "/api/sip/status", "/api/sip/test", "password_set",
		"sipSavedState", "sipComparable(current) !== sipSavedState", "button.disabled = !!reason",
		"auto_answer_delay_ms ?? 1000", `class="sip-settings-grid"`,
		`class="field-group sip-toggle-field"`, `class="toggle"`, `class="slider"`,
		`class="sip-wizard-shell"`, `class="sip-provider-grid"`, `class="sip-advanced"`,
		"confirm_replace", "sipWizardPassword", "sipAdvancedDirty",
		"const canReusePassword = sipConfigState.password_set && sipConfigState.preset_id === provider.id",
		"async function renderSIPSection() {\n    sipWizardStep = 1;",
		"sipWizardPassword = '';\n    sipWizardQuery = '';\n    sipWizardMessage = '';",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP config UI missing contract marker %q", marker)
		}
	}
	if strings.Contains(source, "localStorage") || strings.Contains(source, "sessionStorage") {
		t.Fatal("SIP config UI must not persist credentials in browser storage")
	}
}

func TestSIPConfigHasResponsiveLayoutStyles(t *testing.T) {
	page, err := os.ReadFile("config.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `/css/config-sip.css?v={{.BuildVersion}}`) {
		t.Fatal("config page does not load the SIP layout stylesheet")
	}

	data, err := os.ReadFile(filepath.Join("css", "config-sip.css"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, marker := range []string{
		`[data-workspace-page="config"] .sip-settings-grid`, "display: grid",
		"repeat(2, minmax(0, 1fr))", "@media (max-width: 820px)",
		"grid-template-columns: minmax(0, 1fr)", ".sip-toggle-field",
		".sip-wizard-shell", ".sip-provider-grid", ".sip-advanced",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP layout stylesheet missing contract marker %q", marker)
		}
	}
}

func TestSIPConfigTranslationsComplete(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	required := []string{
		"config.section.sip.label", "config.section.sip.desc", "config.sip.title", "config.sip.description",
		"config.sip.password_stored", "config.sip.password_missing", "config.sip.save_first", "config.sip.status_value",
		"config.sip.wizard.advanced", "config.sip.wizard.advanced_hint", "config.sip.wizard.applied",
		"config.sip.wizard.apply", "config.sip.wizard.automatic", "config.sip.wizard.back",
		"config.sip.wizard.category_europe", "config.sip.wizard.category_germany", "config.sip.wizard.category_global",
		"config.sip.wizard.category_local", "config.sip.wizard.category_north_america", "config.sip.wizard.category_pbx",
		"config.sip.wizard.change", "config.sip.wizard.choose", "config.sip.wizard.configured",
		"config.sip.wizard.continue", "config.sip.wizard.documentation", "config.sip.wizard.eyebrow",
		"config.sip.wizard.intro", "config.sip.wizard.no_results", "config.sip.wizard.notice_account_server",
		"config.sip.wizard.notice_device_password", "config.sip.wizard.notice_fritzbox_phone",
		"config.sip.wizard.notice_pbx_credentials", "config.sip.wizard.notice_router_recommended",
		"config.sip.wizard.phone_number", "config.sip.wizard.prefer_srv", "config.sip.wizard.progress",
		"config.sip.wizard.replace_confirm", "config.sip.wizard.replace_required", "config.sip.wizard.required",
		"config.sip.wizard.review", "config.sip.wizard.safe_registration", "config.sip.wizard.safe_title",
		"config.sip.wizard.search", "config.sip.wizard.server", "config.sip.wizard.title",
	}
	for _, locale := range locales {
		data, err := os.ReadFile(filepath.Join("lang", "config", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatalf("%s config locale: %v", locale, err)
		}
		for _, key := range required {
			if strings.TrimSpace(messages[key]) == "" {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
	}
}
