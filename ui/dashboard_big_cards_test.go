package ui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDashboardDisclosuresBootAndRestore: initDashboard wires DashDisclosures before the router shows the first
// section, and loadSectionContent re-opens the stored disclosures of its section after a successful load (after the
// Updated stamp, inside the try block, so a failed loader skips both).
func TestDashboardDisclosuresBootAndRestore(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/dashboard/main.js"))
	boot := strings.Index(mainJS, "DashDisclosures.init(document);")
	router := strings.Index(mainJS, "DashRouter.init({ onShow: id => loadSectionContent(id) });")
	if boot < 0 || router < 0 || boot > router {
		t.Errorf("initDashboard must call DashDisclosures.init(document) before DashRouter.init (init at %d, router at %d)", boot, router)
	}
	stamp := strings.Index(mainJS, "t('dashboard.section.updated').replace('{{time}}', new Date().toLocaleTimeString())")
	restore := strings.Index(mainJS, "DashDisclosures.restore(document.getElementById('sec-' + id));")
	warn := strings.Index(mainJS, "console.warn('loadSectionContent failed', id, err);")
	if stamp < 0 || restore < stamp || warn < restore {
		t.Errorf("loadSectionContent must call DashDisclosures.restore after the Updated stamp, inside its try block (stamp %d, restore %d, catch %d)", stamp, restore, warn)
	}
}

// TestDashboardKPIRowAndDisclosureStyles pins the shared P5 styles in dashboard-shell.css: auto-fit KPI cells whose
// values sit on the first row (so they line up when a label wraps), a KPI button with a focus ring, disclosures without
// the native marker and with a chevron that turns when open, 44 px summaries with a focus ring, hidden empty count
// badges, and 44 px again for compact density on phones.
func TestDashboardKPIRowAndDisclosureStyles(t *testing.T) {
	t.Parallel()

	shell := dashboardCSSWithoutComments(t, "css/dashboard-shell.css")
	for _, rule := range []struct {
		selector string
		wants    []string
	}{
		{".dash-kpi-row", []string{"display: grid;", "grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));"}},
		{".dash-kpi", []string{"display: grid;", "grid-template-rows: auto 1fr;"}},
		{".dash-kpi-button", []string{"display: grid;", "grid-template-rows: auto 1fr;", "cursor: pointer;"}},
		{".dash-kpi-button:focus-visible", []string{"outline: 2px solid var(--pw-accent);"}},
		{".dash-disclosure-summary", []string{"list-style: none;", "min-height: 44px;", "cursor: pointer;"}},
		{".dash-disclosure-summary::-webkit-details-marker", []string{"display: none;"}},
		{".dash-disclosure[open] > .dash-disclosure-summary::before", []string{"transform: rotate(45deg);"}},
		{".dash-disclosure-summary:focus-visible", []string{"outline: 2px solid var(--pw-accent);"}},
		{".dash-disclosure-count[hidden]", []string{"display: none;"}},
	} {
		body := dashboardCSSRuleBody(t, shell, rule.selector)
		for _, want := range rule.wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s misses %q; block:\n%s", rule.selector, want, body)
			}
		}
	}
	phone := dashboardCSSAtBlock(t, shell, "@media (max-width: 640px)")
	if !regexp.MustCompile(`(?s)\[data-density="compact"\] \.dash-disclosure-summary\s*\{[^}]*min-height: 44px;`).MatchString(phone) {
		t.Error("compact density on phones must restore 44 px disclosure summaries")
	}
}

// dashboardBigCardI18nKeys lists the keys P5 adds for the KPI rows and disclosure summaries of the big cards.
func dashboardBigCardI18nKeys() []string {
	return []string{
		"dashboard.prompt_cache_title",
		"dashboard.tooling_telemetry_format_errors",
		"dashboard.tooling_telemetry_all_counters",
		"dashboard.knowledge_all_metrics",
		"dashboard.knowledge_health_consistency_issues",
		"dashboard.knowledge_health_consistency_unavailable",
		"dashboard.memory_all_counters",
		"dashboard.memory_health_title",
		"dashboard.memory_insights_title",
		"dashboard.memory_milestones_title",
	}
}

func TestDashboardBigCardKeysExistInAllLocales(t *testing.T) {
	t.Parallel()
	assertLocaleKeys(t, filepath.Join("lang", "dashboard"), dashboardBigCardI18nKeys())
}
