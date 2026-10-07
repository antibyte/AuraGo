package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readQuickConnectSerialSources returns every Quick Connect serial module in
// Desktop main-bundle order, the order the standalone browser fixtures need.
func readQuickConnectSerialSources(t *testing.T) string {
	t.Helper()
	build, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read bundle builder: %v", err)
	}
	var parts []string
	for _, match := range regexp.MustCompile(`'ui/(js/desktop/apps/quickconnect-serial(?:-[a-z]+)?\.js)'`).FindAllStringSubmatch(string(build), -1) {
		parts = append(parts, match[1])
	}
	onDisk, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "quickconnect-serial*.js"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) == 0 || len(parts) != len(onDisk) || parts[len(parts)-1] != "js/desktop/apps/quickconnect-serial.js" {
		t.Fatalf("bundle parts %v must list every serial module %v, ending with the controller", parts, onDisk)
	}
	var source strings.Builder
	for _, part := range parts {
		source.WriteString(readDesktopAssetText(t, part))
		source.WriteString("\n")
	}
	return source.String()
}

func TestQuickConnectSerialTranslations(t *testing.T) {
	source := readQuickConnectSerialSources(t) + readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
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
