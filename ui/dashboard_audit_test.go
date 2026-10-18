package ui

import (
	"strings"
	"testing"
)

func TestDashboardAuditTabContract(t *testing.T) {
	t.Parallel()

	html := readDashboardMarkup(t)
	for _, marker := range []string{
		`data-section-link="audit"`,
		`id="sec-audit"`,
		`id="audit-search"`,
		`id="audit-source-filter"`,
		`id="audit-status-filter"`,
		`id="audit-type-filter"`,
		`value="agent_action"`,
		`id="audit-from-filter"`,
		`id="audit-to-filter"`,
		`id="audit-tbody"`,
		`id="audit-clear-filtered"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard audit UI missing marker %q", marker)
		}
	}

	dashJS := readDashboardJS(t)
	for _, marker := range []string{
		"'audit'",
		"loadTabAudit()",
		"setupAuditControls()",
		"function renderAuditEvents",
		"DELETE_AUDIT_EVENTS",
		"/api/dashboard/audit",
		"showConfirm(",
		"audit-cell-summary",
		"audit-cell-actions",
		"scheduleAuditRefresh",
	} {
		if !strings.Contains(dashJS, marker) {
			t.Fatalf("dashboard JS missing audit marker %q", marker)
		}
	}
	eventsJS := readDesktopAssetText(t, "js/dashboard/dashboard-events.js")
	for _, marker := range []string{
		"scheduleAuditRefresh",
		"setTimeout",
	} {
		if !strings.Contains(eventsJS, marker) {
			t.Fatalf("dashboard events JS missing audit refresh debounce marker %q", marker)
		}
	}

	css := readDesktopAssetText(t, "css/dashboard.css")
	for _, marker := range []string{
		".audit-controls",
		"flex-wrap: wrap",
		"border-spacing: 0 0.4rem",
		".audit-cell-summary",
		".audit-summary-text",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("dashboard audit CSS missing readability marker %q", marker)
		}
	}
	if strings.Contains(dashJS, "alert(") {
		t.Fatal("dashboard audit UI must use modals/toasts instead of alert()")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		langFile := readDesktopAssetText(t, "lang/dashboard/"+lang+".json")
		if !strings.Contains(langFile, `"dashboard.audit_type_agent_action"`) {
			t.Fatalf("dashboard %s translation missing agent_action audit label", lang)
		}
	}
}
