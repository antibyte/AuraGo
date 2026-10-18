package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDashboardPersonalityDisabledStateIsNotVisibleByDefault(t *testing.T) {
	t.Parallel()

	html := readDashboardMarkup(t)
	marker := `id="personality-disabled" class="empty-state is-hidden"`
	if !strings.Contains(html, marker) {
		t.Fatalf("personality disabled placeholder should be hidden until API confirms disabled state; missing %q", marker)
	}
}

func TestDashboardPersonalityIncludesCharacterNotes(t *testing.T) {
	t.Parallel()
	html := readDashboardMarkup(t)
	if !strings.Contains(html, `id="character-notes"`) {
		t.Fatal("dashboard personality card must include character notes")
	}
	if !strings.Contains(html, `id="character-narrative-toggle"`) {
		t.Fatal("dashboard personality card must include narrative toggle")
	}
}

func TestDashboardPersonalityIncludesAffectPanel(t *testing.T) {
	t.Parallel()
	html := readDashboardMarkup(t)
	for _, marker := range []string{
		`id="affect-panel"`,
		`id="affect-event-list"`,
		`data-i18n="dashboard.personality_affect_title"`,
		`data-i18n="dashboard.personality_affect_valence"`,
		`data-i18n="dashboard.personality_affect_arousal"`,
		`data-i18n="dashboard.personality_affect_empty"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard personality card must include affect timeline marker %q", marker)
		}
	}
	js := readDashboardJS(t)
	if !strings.Contains(js, "function renderAffectPanel") {
		t.Fatal("dashboard widgets must render the affect panel")
	}
	if !strings.Contains(js, "data.affect_events") {
		t.Fatal("dashboard widgets must read affect_events from personality state")
	}
	if !strings.Contains(js, "function affectCauseLabel") {
		t.Fatal("dashboard widgets must localize affect cause codes")
	}
}

func TestConfigPersonalitySurfacesHelperBannerAndLivedNotes(t *testing.T) {
	t.Parallel()
	mainJS := readDesktopAssetText(t, "js/config/main.js")
	if !strings.Contains(mainJS, "personalitySectionNeedsHelperLLM") {
		t.Fatal("config personality section must detect a missing helper LLM")
	}
	if !strings.Contains(mainJS, "config.personality.helper_required") {
		t.Fatal("config personality section must show the helper-required banner")
	}
	if strings.Contains(mainJS, "pers.engine || pers.engine_v2") {
		t.Fatal("helper banner must not fire for V1-only personality engine")
	}
	promptsJS := readDesktopAssetText(t, "cfg/prompts.js")
	if !strings.Contains(promptsJS, `id="pers-lived-notes"`) {
		t.Fatal("prompts section must include the lived-notes panel")
	}
	if !strings.Contains(promptsJS, "hydratePromptsLivedNotes") {
		t.Fatal("prompts section must load live character notes")
	}
	if !strings.Contains(promptsJS, "config.personality.helper_required") {
		t.Fatal("prompts section must show the helper-required banner")
	}
}

// TestDashboardSectionsUseTwelveColumnGrid: sections lay their cards out on a fixed 12-column grid (reading
// order equals DOM order), container queries collapse spans on narrow sections, the drawer applies at 900px,
// and the masonry columns are gone from dashboard.css.
func TestDashboardSectionsUseTwelveColumnGrid(t *testing.T) {
	t.Parallel()

	const prefix = `.pw-page[data-workspace-page="dashboard"]`
	// dashboardCSSRuleBody takes everything since the previous brace as the selector header, so a comment
	// directly above a rule would hide it; look the rules up in the comment-free source.
	shell := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(normalizeAssetText(mustReadUIFile(t, "css/dashboard-shell.css")), "")

	grid := dashboardCSSRuleBody(t, shell, ".dash-grid")
	for _, marker := range []string{"display: grid;", "grid-template-columns: repeat(12, minmax(0, 1fr));", "gap: var(--pw-space-5);"} {
		if !strings.Contains(grid, marker) {
			t.Errorf("dashboard grid missing %q in block:\n%s", marker, grid)
		}
	}
	for _, span := range []string{"12", "8", "6", "4", "3"} {
		if body := dashboardCSSRuleBody(t, shell, ".dash-grid > .span-"+span); !strings.Contains(body, "grid-column: span "+span+";") {
			t.Errorf("span-%s must cover %s columns; block:\n%s", span, span, body)
		}
	}
	if body := dashboardCSSRuleBody(t, shell, ".dash-section"); !strings.Contains(body, "container-type: inline-size;") {
		t.Errorf("sections must be inline-size query containers; block:\n%s", body)
	}
	for selector, want := range map[string]string{
		".dash-section[hidden]":             "display: none;",
		".dash-grid > .dash-card.is-hidden": "display: none;",
		".dash-menu-btn":                    "display: none;",
	} {
		if body := dashboardCSSRuleBody(t, shell, selector); !strings.Contains(body, want) {
			t.Errorf("%s must declare %q; block:\n%s", selector, want, body)
		}
	}

	medium := dashboardCSSAtBlock(t, shell, "@container (max-width: 1099px)")
	for _, marker := range []string{
		prefix + " .dash-grid > .span-3,",
		prefix + " .dash-grid > .span-4 {",
		"grid-column: span 6;",
		prefix + " .dash-grid > .span-8 {",
		"grid-column: span 12;",
	} {
		if !strings.Contains(medium, marker) {
			t.Errorf("container query below 1100px missing %q", marker)
		}
	}
	narrow := dashboardCSSAtBlock(t, shell, "@container (max-width: 699px)")
	for _, span := range []string{"3", "4", "6", "8"} {
		if !strings.Contains(narrow, prefix+" .dash-grid > .span-"+span) {
			t.Errorf("container query below 700px must widen span-%s", span)
		}
	}
	if !strings.Contains(narrow, "grid-column: span 12;") {
		t.Error("container query below 700px must make every span full width")
	}

	drawer := dashboardCSSAtBlock(t, shell, "@media (max-width: 900px)")
	for _, marker := range []string{
		prefix + " .dash-sidebar {",
		"position: fixed;",
		"transform: translateX(-102%);",
		prefix + " .dash-sidebar.open {",
		"transform: translateX(0);",
		prefix + " .dash-sidebar-backdrop:not([hidden]) {",
		prefix + " .dash-menu-btn {",
	} {
		if !strings.Contains(drawer, marker) {
			t.Errorf("drawer media query missing %q", marker)
		}
	}
	assertOperationalStylesheetFullyScoped(t, shell, prefix)

	legacy := normalizeAssetText(mustReadUIFile(t, "css/dashboard.css"))
	if m := regexp.MustCompile(`(?m)(^|[\s;{])(columns|column-span|break-inside)\s*:`).FindString(legacy); m != "" {
		t.Errorf("dashboard.css still declares masonry layout %q", strings.TrimSpace(m))
	}
	for _, gone := range []string{"#tab-agent", ".dash-tab", ".dash-full-width", ".dash-banner-always", ".dashboard-grid", ".card-grid"} {
		if strings.Contains(legacy, gone) {
			t.Errorf("dashboard.css still styles the removed %s", gone)
		}
	}
}

func TestDashboardCSSIsCacheBustedForAgentGridLayout(t *testing.T) {
	t.Parallel()

	html := readDashboardMarkup(t)
	if !strings.Contains(html, `/css/dashboard.css?v={{.BuildVersion}}`) {
		t.Fatal("dashboard.css link must be cache-busted after agent dashboard grid layout fixes")
	}
}

func dashboardCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	rules := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	target := strings.TrimSpace(selector)
	prefixed := `.pw-page[data-workspace-page="dashboard"] ` + target
	var matches []string
	for _, match := range rules.FindAllStringSubmatch(source, -1) {
		header := strings.TrimSpace(match[1])
		if strings.HasPrefix(header, "@") {
			continue
		}
		for _, part := range strings.Split(header, ",") {
			part = strings.TrimSpace(part)
			if part == prefixed || part == target {
				matches = append(matches, match[2])
				break
			}
		}
	}
	if len(matches) == 0 {
		t.Fatalf("dashboard CSS missing selector %q", selector)
	}
	if len(matches) == 1 {
		return matches[0]
	}
	for _, body := range matches {
		if strings.Contains(body, "columns:") || strings.Contains(body, "break-inside:") || strings.Contains(body, "column-gap:") {
			return body
		}
	}
	return matches[0]
}

// dashboardCSSAtBlock returns the body of the first top-level block whose header is exactly header.
func dashboardCSSAtBlock(t *testing.T, css, header string) string {
	t.Helper()
	at := strings.Index(css, header+" {")
	if at < 0 {
		t.Fatalf("CSS missing block %q", header)
	}
	open := at + len(header) + 1
	depth := 0
	for i := open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[open+1 : i]
			}
		}
	}
	t.Fatalf("CSS block %q is not closed", header)
	return ""
}
