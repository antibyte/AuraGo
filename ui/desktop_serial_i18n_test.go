package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestQuickConnectSerialTranslations(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/apps/quickconnect-serial.js") + readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	keys := regexp.MustCompile(`desktop\.qc_serial_[a-z_]+`).FindAllString(source, -1)
	if len(keys) == 0 {
		t.Fatal("serial UI has no translation keys")
	}
	for _, language := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		for _, section := range []struct {
			directory string
			keys      []string
		}{
			{"desktop", keys},
			{"config/virtual_computers", []string{
				"config.virtual_desktop.serial_browser_label", "config.virtual_desktop.serial_host_label",
				"help.virtual_desktop.serial_browser_enabled", "help.virtual_desktop.serial_host_enabled",
			}},
		} {
			path := filepath.Join("lang", section.directory, language+".json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var translations map[string]string
			if err := json.Unmarshal(data, &translations); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, key := range section.keys {
				if strings.TrimSpace(translations[key]) == "" {
					t.Errorf("%s missing %s", path, key)
				}
			}
		}
	}
}
