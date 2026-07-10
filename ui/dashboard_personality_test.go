package ui

import (
	"strings"
	"testing"
)

func TestDashboardPersonalityDisabledStateIsNotVisibleByDefault(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	marker := `id="personality-disabled" class="empty-state is-hidden"`
	if !strings.Contains(html, marker) {
		t.Fatalf("personality disabled placeholder should be hidden until API confirms disabled state; missing %q", marker)
	}
}

func TestDashboardAgentCardsUseNonOverlappingGridTracks(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/dashboard.css"), "\r\n", "\n")
	agentGrid := dashboardCSSRuleBody(t, css, "#tab-agent .dash-grid")
	for _, marker := range []string{
		"grid-template-columns: repeat(2, minmax(0, 1fr));",
		"row-gap: clamp(1.25rem, 2vw, 1.6rem);",
		"column-gap: clamp(1.25rem, 2vw, 1.6rem);",
		"align-items: start;",
	} {
		if !strings.Contains(agentGrid, marker) {
			t.Fatalf("agent dashboard grid missing non-overlap marker %q in block:\n%s", marker, agentGrid)
		}
	}
	if strings.Contains(agentGrid, "column-gap: unset;") {
		t.Fatalf("agent dashboard grid must not reset the grid column gap after setting it; block:\n%s", agentGrid)
	}

	cardRule := dashboardCSSRuleBody(t, css, "#tab-agent .dash-grid > .dash-card")
	for _, marker := range []string{
		"min-width: 0;",
		"max-width: 100%;",
		"box-sizing: border-box;",
	} {
		if !strings.Contains(cardRule, marker) {
			t.Fatalf("agent dashboard cards missing non-overlap marker %q in block:\n%s", marker, cardRule)
		}
	}

	bodyRule := dashboardCSSRuleBody(t, css, "#tab-agent .dash-card-body")
	if !strings.Contains(bodyRule, "min-width: 0;") {
		t.Fatalf("agent dashboard card bodies must allow content to shrink; block:\n%s", bodyRule)
	}
}

func TestDashboardCSSIsCacheBustedForAgentGridLayout(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	if !strings.Contains(html, `/css/dashboard.css?v={{.BuildVersion}}`) {
		t.Fatal("dashboard.css link must be cache-busted after agent dashboard grid layout fixes")
	}
}

func dashboardCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	needle := selector + " {"
	start := strings.Index(source, needle)
	if start < 0 {
		t.Fatalf("dashboard CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("dashboard CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("dashboard CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}
