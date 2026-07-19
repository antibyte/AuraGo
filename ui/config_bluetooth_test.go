package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readBluetoothUIFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestConfigBluetoothSectionIsWiredAndSafe(t *testing.T) {
	mainJS := readBluetoothUIFile(t, "js/config/main.js")
	for _, wanted := range []string{
		"{ key: 'bluetooth'",
		"bluetooth: { m: 'bluetooth', fn: 'renderBluetoothSection' }",
		"bluetooth: 104",
	} {
		if !strings.Contains(mainJS, wanted) {
			t.Fatalf("config main.js missing %q", wanted)
		}
	}

	module := readBluetoothUIFile(t, "cfg/bluetooth.js")
	for _, wanted := range []string{
		"function renderBluetoothSection(",
		"hasUnsavedConfigChanges()",
		"/api/bluetooth/status",
		"/api/bluetooth/reprobe",
		"/api/bluetooth/discover",
		"/api/bluetooth/devices/action",
		"/api/bluetooth/audio/test",
		"/api/bluetooth/audio/stop",
		`autocomplete="one-time-code"`,
	} {
		if !strings.Contains(module, wanted) {
			t.Fatalf("Bluetooth config module missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("Bluetooth config module uses forbidden native dialog %q", forbidden)
		}
	}

	dashboard := readBluetoothUIFile(t, "js/dashboard/dashboard-widgets.js")
	if !strings.Contains(dashboard, "dashboard.integration_bluetooth") {
		t.Fatal("dashboard integration mapping does not include Bluetooth")
	}
}

func TestConfigBluetoothTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var english map[string]string
	for _, locale := range locales {
		path := "lang/config/bluetooth/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readBluetoothUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if locale == "en" {
			english = values
			continue
		}
		if len(values) == 0 {
			t.Fatalf("%s has no Bluetooth translations", locale)
		}
	}
	if len(english) == 0 {
		t.Fatal("English Bluetooth translations are empty")
	}
	for _, locale := range locales {
		path := "lang/config/bluetooth/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readBluetoothUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for key := range english {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty Bluetooth translation %q", locale, key)
			}
		}
		if len(values) != len(english) {
			t.Fatalf("%s has %d Bluetooth keys, want %d", locale, len(values), len(english))
		}
	}
}
