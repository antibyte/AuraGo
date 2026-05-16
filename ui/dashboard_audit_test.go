package ui

import (
	"strings"
	"testing"
)

func TestDashboardAuditTabContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	for _, marker := range []string{
		`data-tab="audit"`,
		`id="tab-audit"`,
		`id="audit-search"`,
		`id="audit-source-filter"`,
		`id="audit-status-filter"`,
		`id="audit-type-filter"`,
		`id="audit-from-filter"`,
		`id="audit-to-filter"`,
		`id="audit-tbody"`,
		`id="audit-clear-filtered"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard audit UI missing marker %q", marker)
		}
	}

	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"'audit'",
		"loadTabAudit()",
		"setupAuditControls()",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("dashboard main JS missing audit marker %q", marker)
		}
	}

	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"function renderAuditEvents",
		"DELETE_AUDIT_EVENTS",
		"/api/dashboard/audit",
		"showConfirm(",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard widgets JS missing audit marker %q", marker)
		}
	}
	if strings.Contains(mainJS+widgetsJS, "alert(") {
		t.Fatal("dashboard audit UI must use modals/toasts instead of alert()")
	}
}
