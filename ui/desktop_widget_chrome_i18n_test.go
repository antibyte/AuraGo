package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

var widgetChromeI18nKeys = []string{
	"desktop.quickchat_error",
	"desktop.system_info_host",
	"desktop.widget_update_failed",
}

func TestDesktopWidgetChromeI18n(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "t('desktop.quickchat_error')") {
		t.Fatal("quick chat widget must translate fallback errors")
	}
	if strings.Contains(shell, "err.message || 'Error'") {
		t.Fatal("quick chat widget still hardcodes English Error")
	}

	drawer := readDesktopAssetText(t, "js/desktop/core/widget-drawer-runtime.js")
	if !strings.Contains(drawer, "t('desktop.widget_update_failed')") {
		t.Fatal("widget drawer must translate update failures")
	}
	if strings.Contains(drawer, "Failed to update widget") {
		t.Fatal("widget drawer still hardcodes English update failure")
	}

	sysmon := readDesktopAssetText(t, "js/desktop/core/widget-sysmon-runtime.js")
	if !strings.Contains(sysmon, "t('desktop.system_info_host')") {
		t.Fatal("sysmon widget must translate the host uptime label")
	}
	if strings.Contains(sysmon, "`Host ${") {
		t.Fatal("sysmon widget still hardcodes English Host")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range widgetChromeI18nKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang != "en" && values["desktop.widget_update_failed"] == "Could not update the widget." {
			t.Fatalf("%s must not copy the English widget update failure", path)
		}
	}
}
