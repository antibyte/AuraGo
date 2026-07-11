package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDashboardTabButtonsHaveMatchingPanels(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	tabRe := regexp.MustCompile(`data-tab="([^"]+)"`)
	matches := tabRe.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, match := range matches {
		tab := match[1]
		if seen[tab] {
			continue
		}
		seen[tab] = true
		panelID := `id="tab-` + tab + `"`
		if !strings.Contains(html, panelID) {
			t.Fatalf("dashboard tab %q has no matching panel marker %q", tab, panelID)
		}
	}

	if len(seen) == 0 {
		t.Fatal("dashboard must define at least one tab button")
	}
	if strings.Contains(html, `class="dash-tab-panel" id="main-content"`) {
		t.Fatal("overview panel must use id=\"tab-overview\" so showTab can reveal it")
	}
	if !strings.Contains(html, `href="#tab-overview" class="skip-link"`) {
		t.Fatal("dashboard skip link should target the overview tab panel")
	}
}

func TestDashboardIconSpriteLoadsAfterBodyExists(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	dashIconsRe := regexp.MustCompile(`<script[^>]*dash-icons\.js[^>]*></script>`)
	tag := dashIconsRe.FindString(html)
	if tag == "" {
		t.Fatal("dashboard must load the dash-icons.js sprite helper")
	}
	if !strings.Contains(tag, " defer") {
		t.Fatal("dashboard icon sprite script must load with defer so document.body exists before sprite injection")
	}
}

func TestDashboardUserPanelSpacingAndJournalContrast(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/dashboard.css")
	for _, want := range []string{
		"grid-template-columns: minmax(7rem, 0.85fr) minmax(0, 1.45fr) auto auto;",
		"padding: 0.55rem 0.75rem;",
		"grid-template-columns: minmax(0, 1fr) auto auto;",
		"background: color-mix(in srgb, var(--pw-surface-elevated) 88%, var(--pw-accent) 12%);",
		"color: color-mix(in srgb, var(--pw-text) 86%, var(--pw-muted) 14%);",
		"font-weight: 650;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("dashboard user/protocol polish CSS missing marker %q", want)
		}
	}
	if strings.Contains(css, "padding-left: 0.5rem;") {
		t.Fatal("dashboard profile hover must not shift text toward the card edge")
	}
}

func TestDashboardMissionHistoryContrast(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/dashboard.css")
	for _, marker := range []string{
		".mh-table {\n    min-width: 640px;",
		".mh-table th {\n    color: color-mix(in srgb, var(--pw-text) 88%, var(--pw-muted) 12%);",
		".mh-table td {\n    color: color-mix(in srgb, var(--pw-text) 94%, white 6%);",
		".mh-table td:first-child {\n    color: var(--pw-text);",
		".mh-status {\n    display: inline-flex;",
		".mh-trigger {\n    display: inline-flex;",
		".mh-status-success {\n    color: color-mix(in srgb, var(--success) 76%, white 24%);",
		"white-space: nowrap;",
		"color: color-mix(in srgb, var(--pw-text) 90%, white 10%);",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("dashboard mission history contrast CSS missing marker %q", marker)
		}
	}
}

func TestDashboardMissionHistoryClearsLoadingState(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"window.CardState.setLoaded('card-mission-history')",
		"window.CardState.setError('card-mission-history', () => loadMissionHistory(false), { status: resp.status })",
		"window.CardState.setError('card-mission-history', () => loadMissionHistory(false), { status: 0 })",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("dashboard mission history loader missing marker %q", marker)
		}
	}
}
