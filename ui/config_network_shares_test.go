package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readNetworkSharesUIFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestConfigNetworkSharesSectionIsWiredAndSafe(t *testing.T) {
	mainJS := readNetworkSharesUIFile(t, "js/config/main.js")
	for _, wanted := range []string{
		"{ key: 'network_shares'",
		"network_shares: { m: 'network_shares', fn: 'renderNetworkSharesSection' }",
		"network_shares: 105",
	} {
		if !strings.Contains(mainJS, wanted) {
			t.Fatalf("config main.js missing %q", wanted)
		}
	}

	module := readNetworkSharesUIFile(t, "cfg/network_shares.js")
	for _, wanted := range []string{
		"function renderNetworkSharesSection(",
		"hasUnsavedConfigChanges()",
		"/api/network-shares/status",
		"/api/network-shares/reprobe",
		"/api/network-shares/validate",
		"/api/network-shares/' + encodeURIComponent(",
		`id="ns-share-modal"`,
		`id="ns-delete-modal"`,
		`id="ns-create-btn"`,
		"allowed_principals",
		"allowed_clients",
	} {
		if !strings.Contains(module, wanted) {
			t.Fatalf("network-shares config module missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("network-shares config module uses forbidden native dialog %q", forbidden)
		}
	}

	dashboard := readNetworkSharesUIFile(t, "js/dashboard/dashboard-widgets.js")
	for _, wanted := range []string{"dashboard.integration_network_shares", "network_shares: dashIcon('folder')"} {
		if !strings.Contains(dashboard, wanted) {
			t.Fatalf("dashboard integration mapping missing %q", wanted)
		}
	}
}

func TestConfigNetworkSharesTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var english map[string]string
	for _, locale := range locales {
		path := "lang/config/network_shares/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readNetworkSharesUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if locale == "en" {
			english = values
		}
	}
	if len(english) == 0 {
		t.Fatal("English network-shares translations are empty")
	}
	for _, locale := range locales {
		path := "lang/config/network_shares/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readNetworkSharesUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for key := range english {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty network-shares translation %q", locale, key)
			}
		}
		if len(values) != len(english) {
			t.Fatalf("%s has %d network-shares keys, want %d", locale, len(values), len(english))
		}
	}
}
