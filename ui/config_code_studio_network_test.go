package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStudioConfigNetworkToggle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "virtual_desktop.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, marker := range []string{
		"config.virtual_desktop.code_studio_heading",
		"config.virtual_desktop.code_studio_allow_network_label",
		"help.virtual_desktop.code_studio_allow_network",
		"virtual_desktop.code_studio.allow_network",
		"data.code_studio.allow_network === true",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.virtual_desktop.code_studio_heading",
		"config.virtual_desktop.code_studio_allow_network_label",
		"help.virtual_desktop.code_studio_allow_network",
	}
	var english map[string]string
	bundles := make(map[string]map[string]string, len(langs))
	for _, lang := range langs {
		raw, err := os.ReadFile(filepath.Join("lang", "config", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		bundles[lang] = values
		if lang == "en" {
			english = values
		}
	}
	for _, lang := range langs {
		values := bundles[lang]
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, '�') {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
		}
		if lang == "de" {
			for _, key := range keys {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+values[key]+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
		if lang != "en" {
			helpKey := "help.virtual_desktop.code_studio_allow_network"
			labelKey := "config.virtual_desktop.code_studio_allow_network_label"
			if values[helpKey] == english[helpKey] {
				t.Errorf("%s copies the English network help", lang)
			}
			if values[labelKey] == english[labelKey] {
				t.Errorf("%s copies the English network label", lang)
			}
		}
	}
}
