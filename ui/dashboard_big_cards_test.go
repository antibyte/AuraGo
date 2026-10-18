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

// dashboardDisclosure is one detail block of a big card: its data-disclosure key suffix, the data-i18n key of its
// summary label, markup fragments its body must hold, and whether a renderer fills its count badge.
type dashboardDisclosure struct {
	key, label string
	holds      []string
	counted    bool
}

// dashboardBigCard is one P5 card: its KPI row id, an optional fragment the KPI row must sit inside (it opens before
// the row), fragments that stay visible outside every disclosure, and the disclosures in DOM order.
type dashboardBigCard struct {
	section, card, kpi string
	kpiInside          string
	outside            []string
	disclosures        []dashboardDisclosure
}

var (
	dashboardDetailsTag    = regexp.MustCompile(`<details[^>]*>`)
	dashboardSummaryMarkup = regexp.MustCompile(`^<summary class="dash-disclosure-summary"><span(?: id="[a-z-]+")? data-i18n="(dashboard\.[a-z0-9_.]+)"></span><span class="dash-disclosure-count" data-disclosure-count hidden></span></summary>\s*<div class="dash-disclosure-body">`)
	dashboardInlineHandler = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*["']`)
)

// assertDashboardBigCard checks the P5 anatomy of one card:
//   - exactly one KPI row, before the first disclosure (and inside kpiInside), filled by a renderDashKpiRow call;
//   - every disclosure uses the shared, closed-by-default markup with the expected key, summary label and contents;
//   - counted disclosures get their count from a renderer;
//   - the outside fragments stay visible;
//   - the card has no inline handler.
func assertDashboardBigCard(t *testing.T, spec dashboardBigCard) {
	t.Helper()
	card := dashboardCardMarkup(t, readDashboardSection(t, spec.section), spec.card)
	dashJS := readDashboardJS(t)

	row := `<div class="dash-kpi-row" id="` + spec.kpi + `" role="list"></div>`
	if strings.Count(card, row) != 1 {
		t.Fatalf("%s needs exactly one %s", spec.card, row)
	}
	rowAt := strings.Index(card, row)
	if first := strings.Index(card, "<details"); first >= 0 && first < rowAt {
		t.Errorf("%s: the KPI row must come before the first disclosure", spec.card)
	}
	if spec.kpiInside != "" {
		if at := strings.Index(card, spec.kpiInside); at < 0 || at > rowAt {
			t.Errorf("%s: the KPI row must sit inside %s", spec.card, spec.kpiInside)
		}
	}
	if !strings.Contains(dashJS, "renderDashKpiRow(document.getElementById('"+spec.kpi+"'), [") {
		t.Errorf("no renderer fills #%s with renderDashKpiRow", spec.kpi)
	}

	tags := dashboardDetailsTag.FindAllStringIndex(card, -1)
	if len(tags) != len(spec.disclosures) {
		t.Fatalf("%s has %d disclosures, want %d", spec.card, len(tags), len(spec.disclosures))
	}
	var inside strings.Builder
	for i, at := range tags {
		want := spec.disclosures[i]
		id := spec.card + ":" + want.key
		if tag, wantTag := card[at[0]:at[1]], `<details class="dash-disclosure" data-disclosure="`+id+`">`; tag != wantTag {
			t.Errorf("%s disclosure %d is %s, want %s (closed by default: no open attribute)", spec.card, i+1, tag, wantTag)
		}
		end := strings.Index(card[at[1]:], "</details>")
		if end < 0 {
			t.Fatalf("%s is not closed", id)
		}
		block := card[at[1] : at[1]+end]
		inside.WriteString(block)
		if m := dashboardSummaryMarkup.FindStringSubmatch(strings.TrimSpace(block)); m == nil {
			t.Errorf("%s must start with the shared summary (label span, hidden count badge) and a .dash-disclosure-body", id)
		} else if m[1] != want.label {
			t.Errorf("%s summary label = %s, want %s", id, m[1], want.label)
		}
		for _, fragment := range want.holds {
			if !strings.Contains(block, fragment) {
				t.Errorf("%s must hold %s", id, fragment)
			}
		}
		if want.counted && !strings.Contains(dashJS, "setCount('"+id+"'") {
			t.Errorf("no renderer fills the count badge of %s (DashDisclosures.setCount)", id)
		}
	}
	for _, fragment := range spec.outside {
		if !strings.Contains(card, fragment) || strings.Contains(inside.String(), fragment) {
			t.Errorf("%s must keep %s visible, outside the disclosures", spec.card, fragment)
		}
	}
	if dashboardInlineHandler.MatchString(card) {
		t.Errorf("%s uses an inline event handler; use data-action and the delegated listener", spec.card)
	}
}

// TestDashboardPromptCardPutsKPIsFirst (decision 3): four headline KPIs inside #prompt-content, then the module,
// base, fit, cache and section-distribution blocks as closed disclosures; #prompt-no-data stays outside.
func TestDashboardPromptCardPutsKPIsFirst(t *testing.T) {
	t.Parallel()
	assertDashboardBigCard(t, dashboardBigCard{
		section: "prompts", card: "card-prompt", kpi: "prompt-headline-kpis", kpiInside: `id="prompt-content"`,
		outside: []string{`id="prompt-no-data"`, `id="prompt-content"`},
		disclosures: []dashboardDisclosure{
			{key: "modules", label: "dashboard.prompt_modules_title", holds: []string{`id="prompt-module-kpis"`}},
			{key: "base", label: "dashboard.prompt_base_title", holds: []string{`id="prompt-kpis"`, `id="prompt-size-chart"`, `id="prompt-tier-chart"`}},
			{key: "fit", label: "dashboard.prompt_fit_title", holds: []string{`id="prompt-fit-kpis"`, `id="shed-list"`, `id="prompt-savings-chart"`}},
			{key: "cache", label: "dashboard.prompt_cache_title", holds: []string{`id="prompt-cache-kpis"`}},
			{key: "sections", label: "dashboard.prompt_section_dist", holds: []string{`id="prompt-section-chart-title"`, `id="prompt-section-dist-chart"`, `aria-labelledby="prompt-section-chart-title"`, `id="prompt-section-legend"`}},
		},
	})
}

// TestDashboardToolingCardPutsKPIsFirst (decision 4): format errors, recoveries, policy signals and failing tools on
// top; the nine-item summary strip and the eight sections in closed disclosures (the failures one counted). The card
// still starts hidden until telemetry exists.
func TestDashboardToolingCardPutsKPIsFirst(t *testing.T) {
	t.Parallel()
	d := func(key, label, id string, counted bool) dashboardDisclosure {
		return dashboardDisclosure{key: key, label: label, holds: []string{`id="` + id + `"`}, counted: counted}
	}
	assertDashboardBigCard(t, dashboardBigCard{
		section: "tools", card: "card-tooling-telemetry", kpi: "tooling-telemetry-kpis",
		disclosures: []dashboardDisclosure{
			d("summary", "dashboard.tooling_telemetry_all_counters", "tooling-telemetry-summary", false),
			d("parse", "dashboard.tooling_telemetry_parse_sources", "tooling-telemetry-parse", false),
			d("recovery", "dashboard.tooling_telemetry_recovery_events", "tooling-telemetry-recovery", false),
			d("policy", "dashboard.tooling_telemetry_policy_events", "tooling-telemetry-policy", false),
			d("retrieval", "dashboard.tooling_telemetry_retrieval_events", "tooling-telemetry-retrieval", false),
			d("failures", "dashboard.tooling_telemetry_failure_tools", "tooling-telemetry-failures", true),
			d("families", "dashboard.tooling_telemetry_tool_families", "tooling-telemetry-families", false),
			d("scopes", "dashboard.tooling_telemetry_scopes", "tooling-telemetry-scopes", false),
			d("comparison", "dashboard.tooling_telemetry_model_comparison", "tooling-telemetry-comparison", false),
		},
	})
	if !strings.Contains(readDashboardSection(t, "tools"), `<div class="dash-card span-12 is-hidden" id="card-tooling-telemetry">`) {
		t.Error("card-tooling-telemetry must still start hidden until it has telemetry")
	}
}

// TestDashboardKnowledgeCardsPutKPIsFirst (decision 5): health shows consistency issues, pending nodes and open
// conflicts, keeps its status pills visible and puts its metrics and the consistency list (counted) into disclosures;
// quality shows protected, isolated, untyped and duplicate candidates and puts its metrics and its five lists
// (counted) into disclosures.
func TestDashboardKnowledgeCardsPutKPIsFirst(t *testing.T) {
	t.Parallel()
	assertDashboardBigCard(t, dashboardBigCard{
		section: "knowledge", card: "card-knowledge-graph-health", kpi: "knowledge-health-kpis",
		outside: []string{`id="knowledge-health-status"`},
		disclosures: []dashboardDisclosure{
			{key: "metrics", label: "dashboard.knowledge_all_metrics", holds: []string{`id="knowledge-health-metrics"`}},
			{key: "consistency", label: "dashboard.knowledge_health_consistency_title", holds: []string{`id="knowledge-health-consistency"`}, counted: true},
		},
	})
	list := func(key, label, id string) dashboardDisclosure {
		return dashboardDisclosure{key: key, label: label, holds: []string{`id="` + id + `"`}, counted: true}
	}
	assertDashboardBigCard(t, dashboardBigCard{
		section: "knowledge", card: "card-knowledge-graph-quality", kpi: "knowledge-quality-kpis",
		disclosures: []dashboardDisclosure{
			{key: "metrics", label: "dashboard.knowledge_all_metrics", holds: []string{`id="knowledge-quality-metrics"`}},
			list("isolated", "dashboard.knowledge_quality_isolated_title", "knowledge-quality-isolated"),
			list("untyped", "dashboard.knowledge_quality_untyped_title", "knowledge-quality-untyped"),
			list("generic", "dashboard.knowledge_quality_generic_title", "knowledge-quality-generic"),
			list("label-duplicates", "dashboard.knowledge_quality_label_duplicates_title", "knowledge-quality-duplicates"),
			list("id-duplicates", "dashboard.knowledge_quality_id_duplicates_title", "knowledge-quality-id-duplicates"),
		},
	})
}
