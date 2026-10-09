package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualDesktopConfigExposesRetroNetToggle(t *testing.T) {
	t.Parallel()

	source := readDesktopOfficeTestFile(t, filepath.Join("cfg", "virtual_desktop.js"))
	row := `vdCfgToggleRow('config.virtual_desktop.retronet_label', 'help.virtual_desktop.retronet_enabled', data.retronet_enabled === true, 'virtual_desktop.retronet_enabled')`
	rowAt := strings.Index(source, row)
	serialAt := strings.Index(source, `'virtual_desktop.serial_host_enabled'`)
	if rowAt < 0 || serialAt < 0 || rowAt < serialAt {
		t.Fatalf("Retro-Net toggle row missing or not after the serial host row (row=%d serial=%d)", rowAt, serialAt)
	}

	const label, help = "config.virtual_desktop.retronet_label", "help.virtual_desktop.retronet_enabled"
	english := mustReadJSONMap(t, "lang/config/virtual_computers/en.json")
	for _, language := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := mustReadJSONMap(t, "lang/config/virtual_computers/"+language+".json")
		if !strings.Contains(values[label], "Retro-Net") {
			t.Errorf("%s %s = %q, want the proper name Retro-Net", language, label, values[label])
		}
		for _, term := range []string{"Telnet", "SSH", "IP"} {
			if !strings.Contains(values[help], term) {
				t.Errorf("%s %s lacks %q: %q", language, help, term, values[help])
			}
		}
		if language != "en" && (values[label] == english[label] || values[help] == english[help]) {
			t.Errorf("%s copies the English Retro-Net strings", language)
		}
	}
	german := mustReadJSONMap(t, "lang/config/virtual_computers/de.json")
	if !strings.Contains(german[help], "Du ") || !strings.Contains(german[help], "öffentliche") {
		t.Errorf("German help must address the user with Du and keep umlauts: %q", german[help])
	}
}
