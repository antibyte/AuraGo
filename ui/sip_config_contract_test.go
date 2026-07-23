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
		"/api/sip/config", "/api/sip/status", "/api/sip/test", "password_set",
		"sipSavedState", "sipComparable(current) !== sipSavedState", "button.disabled = !!reason",
		"auto_answer_delay_ms ?? 1000", `class="sip-settings-grid"`,
		`class="field-group sip-toggle-field"`, `class="toggle"`, `class="slider"`,
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
