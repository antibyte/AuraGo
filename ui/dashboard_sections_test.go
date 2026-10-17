package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
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

// TestDashboardTemplateExecutesAllSectionPartials: a {{template}} without its {{define}} only fails at Execute,
// so the shell and every partial are parsed and executed here with the page's real data shape.
func TestDashboardTemplateExecutesAllSectionPartials(t *testing.T) {
	t.Parallel()

	partials := dashboardSectionPartialFiles(t)
	if len(partials) != len(dashboardSectionIDs) {
		t.Fatalf("section partials = %v, want one per section %v", partials, dashboardSectionIDs)
	}
	tmpl, err := template.ParseFS(Content, append([]string{"dashboard.html"}, partials...)...)
	if err != nil {
		t.Fatalf("parse dashboard template: %v", err)
	}
	data := struct {
		Lang             string
		BuildVersion     string
		TemplateDataJSON template.JS
	}{Lang: "en", BuildVersion: "p2-test", TemplateDataJSON: template.JS(`{"systemLang":"en","i18n":{}}`)}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute dashboard template: %v", err)
	}
	page := out.String()
	for _, id := range dashboardSectionIDs {
		if got := strings.Count(page, `id="sec-`+id+`"`); got != 1 {
			t.Errorf("rendered dashboard has %d id=\"sec-%s\", want 1", got, id)
		}
	}
	if strings.Contains(page, "{{") {
		t.Error("rendered dashboard still contains template actions")
	}
}
