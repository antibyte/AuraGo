package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"path/filepath"
	"regexp"
	"slices"
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

// dashboardCardPlacement is the P2 placement: section → cards in DOM order with their span (spec §5.1, §7.2).
var dashboardCardPlacement = map[string][]string{
	"overview":    nil, // P3: ov-* tiles instead of cards (TestDashboardOverviewTilesMarkup)
	"personality": {"card-personality:12"},
	"memory":      {"card-memory:8", "card-memory-reflection:4"},
	"user":        {"card-profile:6", "card-activity-overview:6", "card-journal:8", "card-newspaper:4"},
	"knowledge":   {"card-knowledge-graph-summary:12", "card-knowledge-graph-visual:8", "card-knowledge-graph-results:4", "card-knowledge-graph-health:6", "card-knowledge-graph-quality:6"},
	"filesync":    {"card-filesync:12"},
	"problems":    {"card-operational-issues:12", "card-error-patterns:12"},
	"automations": {"card-cronjobs:12", "card-mission-history:8", "card-daemons:4"},
	"system":      {"card-system:4", "card-operations:8", "card-guardian:4", "card-realtime-speech:4", "card-github:4"},
	"audit":       {"card-audit-log:12"},
	"logs":        {"card-logs:12"},
	"llm":         {"card-budget:8", "card-llm-router:4", "card-helper-llm:12"},
	"prompts":     {"card-prompt:12", "card-optimization:6", "card-compression:6"},
	"tools":       {"card-selftest:12", "card-activity:12", "card-adaptive-tools:12", "card-tooling-telemetry:12"},
}

// TestDashboardSidebarLinksMatchSectionPartials: one sidebar link per section partial and vice versa, in the
// contract order; the shell carries the drawer controls and the router/data scripts; the tab layout is gone.
// (The legacy hash map is covered by part B's node router test.)
func TestDashboardSidebarLinksMatchSectionPartials(t *testing.T) {
	t.Parallel()

	shell := readDashboardShell(t)
	var links []string
	for _, m := range regexp.MustCompile(`<a class="dash-nav-link" href="#([a-z]+)" data-section-link="([a-z]+)">`).FindAllStringSubmatch(shell, -1) {
		if m[1] != m[2] {
			t.Errorf("sidebar link href #%s carries data-section-link %q", m[1], m[2])
		}
		links = append(links, m[2])
	}
	if !slices.Equal(links, dashboardSectionIDs) {
		t.Fatalf("sidebar links = %v, want %v", links, dashboardSectionIDs)
	}
	want := make([]string, 0, len(dashboardSectionIDs))
	for _, id := range dashboardSectionIDs {
		want = append(want, "dashboard/sections/"+id+".html")
	}
	slices.Sort(want)
	if got := dashboardSectionPartialFiles(t); !slices.Equal(got, want) {
		t.Fatalf("section partials = %v, want exactly %v", got, want)
	}

	previous := -1
	for _, id := range dashboardSectionIDs {
		call := `{{template "dash-section-` + id + `" .}}`
		if n := strings.Count(shell, call); n != 1 {
			t.Errorf("dashboard.html calls %s %d times, want 1", call, n)
			continue
		}
		if at := strings.Index(shell, call); at < previous {
			t.Errorf("%s is out of sidebar order", call)
		} else {
			previous = at
		}
		section := readDashboardSection(t, id)
		for _, marker := range []string{
			`{{define "dash-section-` + id + `"}}`,
			`<section class="dash-section" id="sec-` + id + `" data-section="` + id + `" aria-labelledby="sec-` + id + `-title" hidden>`,
			`<h2 class="dash-section-title" id="sec-` + id + `-title" tabindex="-1" data-i18n="dashboard.section.` + id + `.title"></h2>`,
			`<p class="dash-section-desc" data-i18n="dashboard.section.` + id + `.desc"></p>`,
			`<span class="dash-section-updated" data-section-updated="` + id + `"></span>`,
			`<button type="button" class="dash-section-refresh" data-section-refresh="` + id + `" data-i18n-title="dashboard.section.refresh" data-i18n-aria-label="dashboard.section.refresh">`,
			`<div class="dash-grid">`,
		} {
			if !strings.Contains(section, marker) {
				t.Errorf("%s.html missing %q", id, marker)
			}
		}
	}

	markup := readDashboardMarkup(t)
	for _, marker := range []string{
		`href="#main-content" class="skip-link"`,
		`<main class="dash-content pw-page-frame" id="main-content">`,
		`<h1 class="visually-hidden" data-i18n="dashboard.page_heading"></h1>`,
		`<nav class="dash-sidebar" id="dash-sidebar" data-i18n-aria-label="dashboard.nav.label">`,
		`<div class="dash-sidebar-backdrop" id="dash-sidebar-backdrop" hidden></div>`,
		`<button type="button" class="dash-menu-btn" id="dash-menu-btn" aria-controls="dash-sidebar" aria-expanded="false" data-i18n-aria-label="dashboard.nav.menu" data-i18n-title="dashboard.nav.menu">`,
		`<div class="dash-nav-group-label" id="dash-nav-group-operations" data-i18n="dashboard.nav.group.operations"></div>`,
		`<ul class="dash-nav-list" aria-labelledby="dash-nav-group-operations">`,
	} {
		if !strings.Contains(markup, marker) {
			t.Errorf("dashboard shell missing %q", marker)
		}
	}
	if !regexp.MustCompile(`<div class="header-actions">\s*<button type="button" class="dash-menu-btn"`).MatchString(shell) {
		t.Error("the menu button must be the first child of .header-actions")
	}
	routerAt := strings.Index(shell, `/js/dashboard/dash-router.js?v={{.BuildVersion}}`)
	apiAt := strings.Index(shell, `/js/dashboard/dash-api.js?v={{.BuildVersion}}`)
	mainAt := strings.Index(shell, `/js/dashboard/main.js?v={{.BuildVersion}}`)
	if routerAt < 0 || apiAt < 0 || mainAt < 0 || routerAt > mainAt || apiAt > mainAt {
		t.Errorf("script order = router:%d api:%d main:%d, want both before main.js", routerAt, apiAt, mainAt)
	}
	for _, gone := range []string{`id="dashTabs"`, `dash-tab-panel`, `data-tab=`, `dash-banner-always`, `pw-page-heading`, `dash-full-width`, `class="container pw-page-frame"`} {
		if strings.Contains(markup, gone) {
			t.Errorf("dashboard markup still contains the legacy %q", gone)
		}
	}
}

// TestDashboardCardsFollowSectionPlacementAndAnatomy: every card sits in its section with one span class and
// has the full header anatomy (icon, h3 title, collapse toggle); error patterns are their own card.
func TestDashboardCardsFollowSectionPlacementAndAnatomy(t *testing.T) {
	t.Parallel()

	cardOpen := regexp.MustCompile(`<div class="dash-card((?: [a-z0-9-]+)*)" id="(card-[a-z0-9-]+)">`)
	if m := cardOpen.FindString(readDashboardShell(t)); m != "" {
		t.Fatalf("dashboard.html must not hold cards; found %s", m)
	}
	seen := map[string]bool{}
	for _, id := range dashboardSectionIDs {
		section := readDashboardSection(t, id)
		var got []string
		for _, m := range cardOpen.FindAllStringSubmatch(section, -1) {
			card := m[2]
			var spans []string
			for _, class := range strings.Fields(m[1]) {
				if strings.HasPrefix(class, "span-") {
					spans = append(spans, strings.TrimPrefix(class, "span-"))
				}
			}
			if len(spans) != 1 {
				t.Errorf("%s has span classes %v, want exactly one", card, spans)
			}
			got = append(got, card+":"+strings.Join(spans, ","))
			if seen[card] {
				t.Errorf("%s appears more than once", card)
			}
			seen[card] = true

			markup := dashboardCardMarkup(t, section, card)
			headerAt := strings.Index(markup, `<div class="dash-card-header">`)
			if headerAt < 0 {
				t.Errorf("%s has no .dash-card-header", card)
				continue
			}
			headerEnd, ok := dashboardDivEnd(markup, headerAt)
			if !ok {
				t.Fatalf("%s header is not closed", card)
			}
			header := markup[headerAt:headerEnd]
			for _, marker := range []string{
				`<svg class="dash-ic card-icon"`,
				`<h3 class="dash-card-title" data-i18n="`,
				`<button type="button" class="collapse-toggle" data-card="` + card + `" data-i18n-title="dashboard.activity_collapse" title="" aria-expanded="true">`,
			} {
				if !strings.Contains(header, marker) {
					t.Errorf("%s header misses %q", card, marker)
				}
			}
			if !strings.Contains(markup, `<div class="dash-card-body"`) {
				t.Errorf("%s has no .dash-card-body", card)
			}
		}
		if !slices.Equal(got, dashboardCardPlacement[id]) {
			t.Errorf("section %s cards = %v, want %v", id, got, dashboardCardPlacement[id])
		}
	}
	if len(seen) != 35 {
		t.Errorf("dashboard has %d cards, want 35", len(seen))
	}

	errorCard := dashboardCardMarkup(t, readDashboardSection(t, "problems"), "card-error-patterns")
	for _, marker := range []string{`data-i18n="dashboard.error_patterns_title"`, `id="error-patterns-wrap"`, `id="error-patterns-list"`} {
		if !strings.Contains(errorCard, marker) {
			t.Errorf("card-error-patterns misses %q", marker)
		}
	}
	if strings.Contains(dashboardCardMarkup(t, readDashboardSection(t, "memory"), "card-memory"), "error-patterns") {
		t.Error("the error-pattern block must leave card-memory")
	}
}
