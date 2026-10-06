package ui

import (
	"encoding/json"
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

// TestDashboardCronjobsFlowJobsAreReadOnly: the server refuses edit, toggle and delete of a flow's schedule
// (409, managed_by "easydrag"), so both renderers show a label instead of Edit and Delete. The behaviour
// itself is checked by scripts/test-dashboard-cronjobs.mjs.
func TestDashboardCronjobsFlowJobsAreReadOnly(t *testing.T) {
	t.Parallel()

	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"function cronjobManagedLabel()",
		"job.managed_by === 'easydrag' ? cronjobManagedLabel() :",
		"job.source === 'flow' ? cronjobManagedLabel() :",
		"t('dashboard.cronjobs_managed_easydrag_hint')",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Errorf("dashboard widgets JS misses %q", marker)
		}
	}
	html := readDesktopAssetText(t, "dashboard.html")
	if !strings.Contains(html, `<option value="flow" data-i18n="dashboard.cronjobs_source_flow"></option>`) {
		t.Error("the cron job source filter misses the flow option")
	}
	if !strings.Contains(readDesktopAssetText(t, "css/dashboard.css"), `.pw-page[data-workspace-page="dashboard"] .cronjobs-managed {`) {
		t.Error("dashboard.css misses the managed label style")
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/dashboard/"+lang+".json")), &values); err != nil {
			t.Fatalf("lang/dashboard/%s.json: %v", lang, err)
		}
		for _, key := range []string{"dashboard.cronjobs_source_flow", "dashboard.cronjobs_managed_easydrag", "dashboard.cronjobs_managed_easydrag_hint"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("lang/dashboard/%s.json misses %s", lang, key)
			}
		}
	}
}
