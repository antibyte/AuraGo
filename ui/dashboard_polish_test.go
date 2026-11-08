package ui

import (
	"regexp"
	"strings"
	"testing"
)

// TestDashboardPolishKeepsOrientationAndRefreshFeedback pins the finishing layer of the dashboard
// shell: a section header that stays under the sticky app header (and steps aside on phones), a
// refresh spinner driven by the shared aria-busy attribute, pointer-aware hover surfaces, reduced
// motion for the new press feedback, and an immediate scroll reset when the router switches sections.
func TestDashboardPolishKeepsOrientationAndRefreshFeedback(t *testing.T) {
	t.Parallel()

	shell := dashboardCSSWithoutComments(t, "css/dashboard-shell.css")

	header := dashboardCSSRuleBody(t, shell, ".dash-section-header")
	for _, want := range []string{
		"position: sticky;",
		"top: var(--pw-header-height);",
		"z-index: 60;",
		"align-self: start;",
		"border-bottom: 1px solid var(--pw-mat-divider);",
		"backdrop-filter: blur(10px);",
	} {
		if !strings.Contains(header, want) {
			t.Errorf(".dash-section-header misses %q; block:\n%s", want, header)
		}
	}

	content := dashboardCSSRuleBody(t, shell, ".dash-content")
	if !strings.Contains(content, "scroll-margin-top: calc(var(--pw-header-height) + var(--pw-space-3));") {
		t.Error("the skip-link target needs a scroll margin below the sticky app header")
	}

	// main.js only sets aria-busy on the section refresh button; section modules add .is-busy for
	// some cards. Both selectors must spin so no section is left without visible feedback.
	busy := dashboardCSSRuleBody(t, shell, `.dash-section-refresh[aria-busy="true"]`)
	for _, want := range []string{"cursor: progress;", "color: var(--pw-accent);"} {
		if !strings.Contains(busy, want) {
			t.Errorf("the busy refresh button misses %q; block:\n%s", want, busy)
		}
	}
	for _, selector := range []string{
		`.dash-section-refresh.is-busy .dash-ic`,
		`.dash-section-refresh[aria-busy="true"] .dash-ic`,
	} {
		spinner := dashboardCSSRuleBody(t, shell, selector)
		if !strings.Contains(spinner, "animation: dash-spin 0.9s linear infinite;") {
			t.Errorf("%s must spin the refresh icon", selector)
		}
	}

	for _, selector := range []string{".dash-section-refresh:hover", ".dash-section-action:hover"} {
		if body := dashboardCSSRuleBody(t, shell, selector); !strings.Contains(body, "background: var(--pw-surface-elevated);") {
			t.Errorf("%s needs a visible hover surface", selector)
		}
	}

	phone := dashboardCSSAtBlock(t, shell, "@media (max-width: 640px)")
	if !regexp.MustCompile(`(?s)\.dash-section-header\s*\{[^}]*position: static;`).MatchString(phone) {
		t.Error("phones must not pin a two-row section header over the viewport")
	}
	touch := dashboardCSSAtBlock(t, shell, "@media (hover: none)")
	if !regexp.MustCompile(`(?s)\.ov-tile-link:hover[^}]*background: transparent;`).MatchString(touch) {
		t.Error("touch pointers must not keep stuck hover surfaces on dashboard tiles")
	}
	reduced := dashboardCSSAtBlock(t, shell, "@media (prefers-reduced-motion: reduce)")
	if !strings.Contains(reduced, ".dash-section-refresh:active") {
		t.Error("the new press feedback must be disabled for reduced motion")
	}
	if !strings.Contains(shell, `[data-density="compact"] .dash-section-header`) {
		t.Error("compact density needs its own sticky-header padding")
	}

	router := normalizeAssetText(mustReadUIFile(t, "js/dashboard/dash-router.js"))
	if !strings.Contains(router, "window.scrollTo({ top: 0, left: 0, behavior: 'instant' });") {
		t.Error("a section switch must reset the scroll instantly, not animate the global smooth scroll")
	}
	if strings.Contains(router, "window.scrollTo(0, 0)") {
		t.Error("the router still uses the smooth-scrolling two-argument scrollTo")
	}
}
