package ui

import (
	"regexp"
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

func TestDashboardAgentCardsUseMasonryColumnLayout(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/dashboard.css"), "\r\n", "\n")
	agentGrid := dashboardCSSRuleBody(t, css, "#tab-agent .dash-grid")
	for _, marker := range []string{
		"columns: 2;",
		"column-gap: clamp(1.25rem, 2vw, 1.6rem);",
	} {
		if !strings.Contains(agentGrid, marker) {
			t.Fatalf("agent dashboard masonry grid missing marker %q in block:\n%s", marker, agentGrid)
		}
	}
	if strings.Contains(agentGrid, "grid-template-columns:") {
		t.Fatalf("agent dashboard grid must not use CSS grid columns; block:\n%s", agentGrid)
	}

	cardRule := dashboardCSSRuleBody(t, css, "#tab-agent .dash-grid > .dash-card")
	for _, marker := range []string{
		"min-width: 0;",
		"max-width: 100%;",
		"box-sizing: border-box;",
	} {
		if !strings.Contains(cardRule, marker) {
			t.Fatalf("agent dashboard cards missing masonry marker %q in block:\n%s", marker, cardRule)
		}
	}

	baseGrid := regexp.MustCompile(`(?s)\.pw-page\[data-workspace-page="dashboard"\] \.dash-grid\s*\{[^}]*columns:\s*3;`)
	if !baseGrid.MatchString(css) {
		t.Fatal("dashboard base masonry grid must define columns: 3")
	}
	if !strings.Contains(css, "column-gap: var(--pw-space-5);") {
		t.Fatal("dashboard base masonry grid must keep the desktop column gap")
	}

	baseCardRule := dashboardCSSRuleBody(t, css, ".dash-grid > .dash-card")
	for _, marker := range []string{
		"break-inside: avoid;",
		"margin-bottom: var(--pw-space-5);",
	} {
		if !strings.Contains(baseCardRule, marker) {
			t.Fatalf("dashboard cards missing masonry marker %q in block:\n%s", marker, baseCardRule)
		}
	}

	fullWidthRule := dashboardCSSRuleBody(t, css, ".dash-grid > .dash-card.dash-full-width")
	if !strings.Contains(fullWidthRule, "column-span: all;") {
		t.Fatalf("full-width dashboard cards must span all masonry columns; block:\n%s", fullWidthRule)
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
