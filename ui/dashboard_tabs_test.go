package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDashboardIconSpriteLoadsAfterBodyExists(t *testing.T) {
	t.Parallel()

	html := readDashboardMarkup(t)
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

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/dashboard.css"), "\r\n", "\n")
	for _, marker := range []string{
		`.pw-page[data-workspace-page="dashboard"] .mh-table {
    min-width: 640px;`,
		`color: color-mix(in srgb, var(--pw-text) 88%, var(--pw-muted) 12%);`,
		`color: color-mix(in srgb, var(--pw-text) 94%, white 6%);`,
		`.pw-page[data-workspace-page="dashboard"] .mh-table td:first-child {
    color: var(--pw-text);`,
		`.pw-page[data-workspace-page="dashboard"] .mh-status {
    display: inline-flex;`,
		`.pw-page[data-workspace-page="dashboard"] .mh-trigger {
    display: inline-flex;`,
		`color: color-mix(in srgb, var(--pw-success) 76%, white 24%);`,
		"white-space: nowrap;",
		`color: color-mix(in srgb, var(--pw-text) 90%, white 10%);`,
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
