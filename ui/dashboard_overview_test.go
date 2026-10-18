package ui

import (
	"regexp"
	"strings"
	"testing"
)

// dashboardOverviewTiles is the P3 overview layout in DOM order (spec §6): tile id, span, hideable, and the
// section a tile links to as a whole ("" = no tile link).
var dashboardOverviewTiles = []struct {
	id, span, link string
	hideable       bool
}{
	{"ov-status", "12", "", false},
	{"ov-attention", "12", "", false},
	{"ov-system", "3", "system", true},
	{"ov-budget", "3", "llm", true},
	{"ov-automations", "3", "automations", true},
	{"ov-knowledge", "3", "knowledge", true},
	{"ov-activity", "8", "", true},
	{"ov-agent-state", "4", "personality", true},
}

// TestDashboardOverviewTilesMarkup checks the overview section markup:
//   - the eight tiles are direct .dash-grid children in spec order, one span each;
//   - only the six tiles after status and attention are hideable: they carry a visibility switch and none of
//     the tiles has a collapse toggle;
//   - KPI tiles link to their section with a plain href, which the router's hashchange handler picks up;
//   - "Show all" and "Customize" sit in the section header before the refresh button.
func TestDashboardOverviewTilesMarkup(t *testing.T) {
	t.Parallel()

	section := readDashboardSection(t, "overview")
	gridAt := strings.Index(section, `<div class="dash-grid">`)
	if gridAt < 0 {
		t.Fatal("overview.html has no .dash-grid")
	}
	gridEnd, ok := dashboardDivEnd(section, gridAt)
	if !ok {
		t.Fatal("overview .dash-grid is not closed")
	}
	grid := section[gridAt:gridEnd]
	var children []string
	for i, depth := 0, 0; i < len(grid); {
		switch {
		case strings.HasPrefix(grid[i:], "<div"):
			if depth == 1 {
				children = append(children, grid[i:i+strings.Index(grid[i:], ">")+1])
			}
			depth++
			i += len("<div")
		case strings.HasPrefix(grid[i:], "</div>"):
			depth--
			i += len("</div>")
		default:
			i++
		}
	}
	if len(children) != len(dashboardOverviewTiles) {
		t.Fatalf("overview grid has %d direct children, want %d: %v", len(children), len(dashboardOverviewTiles), children)
	}
	for i, tile := range dashboardOverviewTiles {
		want := `<div class="dash-card dash-tile span-` + tile.span + `" id="` + tile.id + `" data-tile="` + tile.id + `"`
		if tile.hideable {
			want += ` data-tile-hideable`
		}
		want += `>`
		if children[i] != want {
			t.Errorf("grid child %d = %s, want %s", i, children[i], want)
		}
		markup := dashboardCardMarkup(t, section, tile.id)
		for _, marker := range []string{
			`<div class="dash-card-header"><svg class="dash-ic card-icon"`,
			`<h3 class="dash-card-title" data-i18n="dashboard.overview.`,
			`<div class="dash-card-body">`,
		} {
			if !strings.Contains(markup, marker) {
				t.Errorf("%s misses %q", tile.id, marker)
			}
		}
		toggle := `<button type="button" class="ov-tile-toggle" data-tile-toggle="` + tile.id + `" aria-pressed="true" hidden>`
		if got := strings.Contains(markup, toggle); got != tile.hideable {
			t.Errorf("%s visibility switch present = %v, want %v", tile.id, got, tile.hideable)
		}
		if tile.link != "" && !strings.Contains(markup, `<a class="ov-tile-link" href="#`+tile.link+`" id="`+tile.id+`-body">`) {
			t.Errorf("%s must link to #%s", tile.id, tile.link)
		}
	}
	for id, marker := range map[string]string{
		"ov-status":    `<span class="ov-pill" id="ov-overall" data-overall=""></span>`,
		"ov-attention": `<ul class="ov-attention-list" id="ov-attention-list"></ul>`,
		"ov-activity":  `<ul class="ov-activity-list" id="ov-activity-list"></ul>`,
	} {
		if !strings.Contains(dashboardCardMarkup(t, section, id), marker) {
			t.Errorf("%s misses %q", id, marker)
		}
	}
	if !strings.Contains(dashboardCardMarkup(t, section, "ov-status"), `<ul class="ov-status-line" id="ov-status-line"></ul>`) {
		t.Error("ov-status misses the status line")
	}
	for _, gone := range []string{"collapse-toggle", `id="card-`, "data-section-link", "agent-banner", "qs-grid"} {
		if strings.Contains(section, gone) {
			t.Errorf("overview.html still contains %q", gone)
		}
	}

	meta := regexp.MustCompile(`(?s)<div class="dash-section-meta">\s*` +
		regexp.QuoteMeta(`<span class="dash-section-updated" data-section-updated="overview"></span>`) + `\s*` +
		regexp.QuoteMeta(`<button type="button" class="dash-section-action" data-action="overview-show-all" hidden>`) + `.*?</button>\s*` +
		regexp.QuoteMeta(`<button type="button" class="dash-section-action" data-action="overview-customize" aria-pressed="false">`) + `.*?</button>\s*` +
		regexp.QuoteMeta(`<button type="button" class="dash-section-refresh" data-section-refresh="overview"`))
	if !meta.MatchString(section) {
		t.Error("overview header must hold, in order: updated time, Show all (hidden), Customize (aria-pressed), refresh")
	}

	shellCSS := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(normalizeAssetText(mustReadUIFile(t, "css/dashboard-shell.css")), "")
	for _, selector := range []string{".dash-section-action[hidden]", ".ov-tile-toggle[hidden]"} {
		if body := dashboardCSSRuleBody(t, shellCSS, selector); !strings.Contains(body, "display: none;") {
			t.Errorf("%s must declare display: none; block:\n%s", selector, body)
		}
	}
}

// TestDashboardSidebarStatusAndBadges: the sidebar opens with the status block (a plain link to #overview, not a
// .dash-nav-link, so the 14-link contract stays); overview, problems and automations carry one hidden badge
// each; the two P3 scripts load after dash-api.js and before main.js.
func TestDashboardSidebarStatusAndBadges(t *testing.T) {
	t.Parallel()

	shell := readDashboardShell(t)
	nav := `<nav class="dash-sidebar" id="dash-sidebar" data-i18n-aria-label="dashboard.nav.label">`
	status := `<a class="dash-sidebar-status" href="#overview" data-sidebar-status data-i18n-aria-label="dashboard.section.overview.title">`
	if !regexp.MustCompile(regexp.QuoteMeta(nav) + `\s*` + regexp.QuoteMeta(status)).MatchString(shell) {
		t.Fatalf("the sidebar must open with %s", status)
	}
	at := strings.Index(shell, status)
	block := shell[at : at+strings.Index(shell[at:], "</a>")]
	for _, marker := range []string{
		`<span class="dash-sidebar-pill" data-sidebar-overall></span>`,
		`<span class="status-dot gray" data-sidebar-busy aria-hidden="true"></span><span data-sidebar-busy-label></span>`,
		`<span class="status-dot gray" data-sidebar-live aria-hidden="true"></span><span data-sidebar-live-label></span>`,
	} {
		if !strings.Contains(block, marker) {
			t.Errorf("sidebar status block misses %q", marker)
		}
	}
	if strings.Contains(block, "dash-nav-link") || strings.Contains(block, "data-section-link") {
		t.Error("the status block must not be a section link")
	}
	if n := strings.Count(shell, `class="dash-nav-link"`); n != 14 {
		t.Errorf("sidebar has %d .dash-nav-link elements, want 14", n)
	}
	if n := strings.Count(shell, `class="dash-nav-badge"`); n != 3 {
		t.Errorf("sidebar has %d badges, want 3", n)
	}
	for _, id := range []string{"overview", "problems", "automations"} {
		link := `<a class="dash-nav-link" href="#` + id + `" data-section-link="` + id + `">`
		linkAt := strings.Index(shell, link)
		if linkAt < 0 {
			t.Errorf("sidebar link %s missing", id)
			continue
		}
		linkEnd := linkAt + strings.Index(shell[linkAt:], "</a>") + len("</a>")
		badge := `<span class="dash-nav-badge" data-nav-badge="` + id + `" role="img" hidden></span></a>`
		if !strings.HasSuffix(shell[linkAt:linkEnd], badge) {
			t.Errorf("sidebar link %s must end with %s", id, badge)
		}
	}

	apiAt := strings.Index(shell, `<script src="/js/dashboard/dash-api.js?v={{.BuildVersion}}" defer></script>`)
	summaryAt := strings.Index(shell, `<script src="/js/dashboard/dash-summary.js?v={{.BuildVersion}}" defer></script>`)
	overviewAt := strings.Index(shell, `<script src="/js/dashboard/dash-overview.js?v={{.BuildVersion}}" defer></script>`)
	mainAt := strings.Index(shell, `<script src="/js/dashboard/main.js?v={{.BuildVersion}}" defer></script>`)
	if apiAt < 0 || summaryAt < 0 || overviewAt < 0 || mainAt < 0 || !(apiAt < summaryAt && summaryAt < overviewAt && overviewAt < mainAt) {
		t.Errorf("script order = api:%d summary:%d overview:%d main:%d, want dash-api < dash-summary < dash-overview < main (all defer)", apiAt, summaryAt, overviewAt, mainAt)
	}
}
