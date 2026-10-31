package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// TestInfraPDFLocaleKeysInEveryDesktopLocale pins the strings of the Infra PDF export: the dialog keys are read by the
// browser, the desktop.infra_pdf_* keys by the server that draws the PDF.
func TestInfraPDFLocaleKeysInEveryDesktopLocale(t *testing.T) {
	keys := []string{
		"desktop.infra_export_pdf", "desktop.infra_export_pdf_paper", "desktop.infra_export_pdf_sections",
		"desktop.infra_export_pdf_section_map", "desktop.infra_export_pdf_section_hosts", "desktop.infra_export_pdf_section_inventory",
		"desktop.infra_export_pdf_create", "desktop.infra_export_pdf_busy", "desktop.infra_export_pdf_done", "desktop.infra_export_pdf_failed",
	}
	for _, name := range []string{
		"title", "assets", "overview", "part", "unplaced", "page", "page_ref", "generated",
		"stat_devices", "stat_vms", "stat_containers", "stat_services", "stat_networks", "stat_connections", "stat_running",
		"legend", "hosts_title", "hostless", "continued", "guests", "inventory_title", "connections_title", "no_place",
		"col_name", "col_kind", "col_address", "col_mac", "col_host", "col_system", "col_sources", "col_power",
		"col_from", "col_to", "col_port", "col_speed", "col_label", "empty",
	} {
		keys = append(keys, "desktop.infra_pdf_"+name)
	}
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	placeholder := regexp.MustCompile(`\{[a-z]+\}`)
	load := func(locale string) map[string]string {
		var dictionary map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/desktop/"+locale+".json"), &dictionary); err != nil {
			t.Fatalf("%s.json: %v", locale, err)
		}
		return dictionary
	}
	english := load("en")
	for _, locale := range locales {
		dictionary := load(locale)
		for _, key := range keys {
			value := strings.TrimSpace(dictionary[key])
			if value == "" {
				t.Errorf("%s.json is missing %s", locale, key)
				continue
			}
			for _, token := range placeholder.FindAllString(english[key], -1) {
				if !strings.Contains(value, token) {
					t.Errorf("%s.json %s lost the placeholder %s: %q", locale, key, token, value)
				}
			}
		}
	}
}
