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
	if n := strings.Count(dashboard, "@media (max-width: 700px)"); n != 1 {
		t.Errorf("dashboard.css has %d @media (max-width: 700px) blocks, want 1", n)
	}
	narrow := dashboardCSSAtBlock(t, dashboard, "@media (max-width: 700px)")
	for _, selector := range []string{"#card-prompt .prompt-details-grid {", ".fs-dash-grid {"} {
		if !strings.Contains(narrow, selector) {
			t.Errorf("the 700px block misses %s", selector)
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
