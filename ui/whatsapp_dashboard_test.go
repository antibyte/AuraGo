package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardWhatsAppTileIsWired(t *testing.T) {
	js := readDashboardJS(t)
	for _, want := range []string{
		"whatsapp: dashIcon('chat')",
		"whatsapp: t('dashboard.integration_whatsapp')",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("the dashboard scripts miss %q", want)
		}
	}
	raw, err := os.ReadFile(filepath.Join("cfg", "whatsapp.js"))
	if err != nil {
		t.Fatalf("read cfg/whatsapp.js: %v", err)
	}
	const entry = "recipient_not_on_whatsapp: () => t('config.whatsapp.err_recipient_not_on_whatsapp')"
	if !strings.Contains(normalizeAssetText(raw), entry) {
		t.Fatalf("cfg/whatsapp.js ERRORS misses %q", entry)
	}

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, group := range []struct {
		dir string
		key string
	}{
		{filepath.Join("lang", "dashboard"), "dashboard.integration_whatsapp"},
		{filepath.Join("lang", "config", "whatsapp"), "config.whatsapp.err_recipient_not_on_whatsapp"},
	} {
		for _, locale := range locales {
			path := filepath.Join(group.dir, locale+".json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var bundle map[string]interface{}
			if err := json.Unmarshal(data, &bundle); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if text, ok := bundle[group.key].(string); !ok || strings.TrimSpace(text) == "" {
				t.Errorf("%s is missing %s", path, group.key)
			}
		}
	}
}
