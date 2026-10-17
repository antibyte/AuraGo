package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// dashboardShellI18nKeys lists the keys the P2 shell adds (contract §6 plus the error-patterns card title).
func dashboardShellI18nKeys() []string {
	keys := []string{
		"dashboard.nav.label",
		"dashboard.nav.menu",
		"dashboard.nav.group.agent",
		"dashboard.nav.group.knowledge",
		"dashboard.nav.group.operations",
		"dashboard.nav.group.analytics",
		"dashboard.section.refresh",
		"dashboard.section.updated",
		"dashboard.error_patterns_title",
	}
	for _, id := range dashboardSectionIDs {
		keys = append(keys, "dashboard.section."+id+".title", "dashboard.section."+id+".desc")
	}
	return keys
}

func TestDashboardShellKeysExistInAllLocales(t *testing.T) {
	t.Parallel()

	assertLocaleKeys(t, filepath.Join("lang", "dashboard"), dashboardShellI18nKeys())
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/"+lang+".json"), &values); err != nil {
			t.Fatalf("lang/dashboard/%s.json: %v", lang, err)
		}
		if !strings.Contains(values["dashboard.section.updated"], "{{time}}") {
			t.Errorf("lang/dashboard/%s.json: dashboard.section.updated must keep the {{time}} placeholder", lang)
		}
	}
}
