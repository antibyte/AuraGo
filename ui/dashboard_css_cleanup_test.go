package ui

import (
	"regexp"
	"strings"
	"testing"
)

// dashboardCSSWithoutComments returns a stylesheet with LF line endings and without comments, so rule lookups see
// bare selector headers.
func dashboardCSSWithoutComments(t *testing.T, path string) string {
	t.Helper()
	return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(normalizeAssetText(mustReadUIFile(t, path)), "")
}

// TestDashboardCSSDropsSupersededRules: the agent banner and Quick Status cards are gone since P3 and the tab bar
// since P2, so their rules are deleted; the duplicated 700px (dashboard.css) and 640px (dashboard-shell.css) media
// blocks are one block each, after the base rules they override.
func TestDashboardCSSDropsSupersededRules(t *testing.T) {
	t.Parallel()

	dashboard := dashboardCSSWithoutComments(t, "css/dashboard.css")
	for _, gone := range []string{"#card-agent-banner", ".agent-banner", ".qs-"} {
		if strings.Contains(dashboard, gone) {
			t.Errorf("dashboard.css still styles the removed %s", gone)
		}
	}
	if strings.Contains(normalizeAssetText(mustReadUIFile(t, "css/dashboard.css")), "D-14/D-15") {
		t.Error("dashboard.css keeps the orphaned D-14/D-15 breakpoint comments")
	}
	// The former 700px narrow block sits on the 640px breakpoint (wave 4 set) as the last block of the file.
	if strings.Contains(dashboard, "@media (max-width: 700px)") {
		t.Error("dashboard.css still uses the off-scale 700px breakpoint")
	}
	lastNarrow := strings.LastIndex(dashboard, "@media (max-width: 640px) {")
	if lastNarrow < 0 {
		t.Fatal("dashboard.css misses the final 640px narrow block")
	}
	narrow := dashboardCSSAtBlock(t, dashboard[lastNarrow:], "@media (max-width: 640px)")
	for _, selector := range []string{"#card-prompt .prompt-details-grid {", ".fs-dash-grid {"} {
		if !strings.Contains(narrow, selector) {
			t.Errorf("the final 640px block misses %s", selector)
		}
	}

	shell := dashboardCSSWithoutComments(t, "css/dashboard-shell.css")
	if n := strings.Count(shell, "@media (max-width: 640px)"); n != 1 {
		t.Errorf("dashboard-shell.css has %d @media (max-width: 640px) blocks, want 1", n)
	}
	phone := dashboardCSSAtBlock(t, shell, "@media (max-width: 640px)")
	for _, selector := range []string{".dash-content {", ".dash-section-meta {", ".dash-section-refresh {", `[data-density="compact"] .dash-nav-link {`, ".dash-section-action {"} {
		if !strings.Contains(phone, selector) {
			t.Errorf("the 640px block misses %s", selector)
		}
	}
	// The block must follow the base .dash-section-action rule, or its 44px would lose to min-height: 40px.
	if strings.Index(shell, "@media (max-width: 640px)") < strings.Index(shell, `.pw-page[data-workspace-page="dashboard"] .dash-section-action {`) {
		t.Error("the 640px block must follow the base .dash-section-action rule")
	}

	enhancements := dashboardCSSWithoutComments(t, "css/enhancements.css")
	for _, gone := range []string{".dash-tab", "panelFadeIn"} {
		if strings.Contains(enhancements, gone) {
			t.Errorf("enhancements.css still has the tab-bar rule %s", gone)
		}
	}
}

// TestDashboardControlsAndProfileSearchStyles: the logs, problems, cron job and audit control bars use the shared
// .btn look (wave 3) instead of the former .log-btn rules, and the profile search box keeps its height in the user
// card's column flex body (.pw-search sets flex: 1 1 220px, which grew it to about 220 px there).
func TestDashboardControlsAndProfileSearchStyles(t *testing.T) {
	t.Parallel()

	css := dashboardCSSWithoutComments(t, "css/dashboard.css")
	for _, section := range []string{"logs", "problems", "automations", "audit"} {
		html := normalizeAssetText(mustReadUIFile(t, "dashboard/sections/"+section+".html"))
		for _, m := range regexp.MustCompile(`<button[^>]*class="([^"]*(?:log-btn|mh-more-btn)[^"]*)"`).FindAllStringSubmatch(html, -1) {
			if !strings.Contains(m[1], "btn btn-") || !strings.Contains(m[1], "btn-sm") {
				t.Errorf("dashboard/sections/%s.html: button class %q must use the shared btn variant + btn-sm", section, m[1])
			}
		}
	}
	for _, gone := range []string{".log-controls .log-btn", ".cf-overlay", ".cf-modal", ".cf-add-btn", ".audit-danger-btn"} {
		if strings.Contains(css, gone) {
			t.Errorf("dashboard.css still styles the superseded %s", gone)
		}
	}
	if body := dashboardCSSRuleBody(t, css, ".dash-card-body > .profile-search"); !strings.Contains(body, "flex: none;") {
		t.Error("a profile search box directly in a card body must not grow (flex: none)")
	}
}
