package ui

import (
	"strings"
	"testing"
)

func TestDashboardCronjobsTabContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	for _, marker := range []string{
		`data-tab="cronjobs"`,
		`id="tab-cronjobs"`,
		`id="cronjobs-search"`,
		`id="cronjobs-source-filter"`,
		`id="cronjobs-status-filter"`,
		`dashboard.cronjobs_status_error`,
		`id="cronjobs-tbody"`,
		`id="cronjobs-refresh"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard cronjobs UI missing marker %q", marker)
		}
	}

	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"'cronjobs'",
		"loadTabCronjobs()",
		"setupCronjobsControls()",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("dashboard main JS missing cronjobs marker %q", marker)
		}
	}

	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"function renderCronjobs",
		"/api/dashboard/cronjobs",
		"showConfirm(",
		"cronjobs-status-filter",
		"dashboard.cronjobs_status_error",
		"last_error",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard widgets JS missing cronjobs marker %q", marker)
		}
	}
	if strings.Contains(mainJS+widgetsJS, "alert(") {
		t.Fatal("dashboard cronjobs UI must use modals/toasts instead of alert()")
	}
}
