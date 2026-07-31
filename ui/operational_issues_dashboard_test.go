package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationalIssuesDashboardContract(t *testing.T) {
	t.Parallel()

	html := string(mustReadUIFile(t, "dashboard.html"))
	for _, marker := range []string{
		`id="card-operational-issues"`,
		`id="operational-issues-status"`,
		`id="operational-issues-kind"`,
		`id="operational-issues-severity"`,
		`id="operational-issues-confirm"`,
		`/js/dashboard/operational-issues.js`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard missing operational-issue marker %q", marker)
		}
	}

	js := string(mustReadUIFile(t, "js/dashboard/operational-issues.js"))
	for _, marker := range []string{
		"/api/operational-issues?",
		"/api/operational-issues/stale-preview",
		"/api/operational-issues/archive-stale",
		"textContent",
		"replaceChildren",
		"ARCHIVE_STALE_ISSUES",
		"dataset.label",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("operational-issue JavaScript missing %q", marker)
		}
	}
	for _, forbidden := range []string{"window.confirm(", ".innerHTML", "document.write("} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("operational-issue JavaScript contains unsafe UI primitive %q", forbidden)
		}
	}

	css := string(mustReadUIFile(t, "css/dashboard.css"))
	if !strings.Contains(css, `.pw-page[data-workspace-page="dashboard"] .operational-issues-controls`) {
		t.Fatal("operational-issue styles must remain scoped to the dashboard")
	}
}

func TestOperationalIssuesTranslationsExistInEveryLocale(t *testing.T) {
	t.Parallel()

	dashboardKeys := []string{
		"dashboard.operational_issues_title",
		"dashboard.operational_issues_desc",
		"dashboard.operational_issues_filter_active",
		"dashboard.operational_issues_filter_archived",
		"dashboard.operational_issues_kind_runtime",
		"dashboard.operational_issues_kind_tool",
		"dashboard.operational_issues_kind_review",
		"dashboard.operational_issues_confirm_stale",
		"dashboard.operational_issues_status_archived",
	}
	backendKeys := []string{
		"backend.operational_issue_notice_title",
		"backend.operational_issue_notice_last_seen",
		"backend.operational_issue_notice_severity",
		"backend.operational_issue_severity_error",
		"backend.operational_issue_severity_warning",
		"backend.operational_issue_severity_info",
	}
	assertLocaleKeys(t, filepath.Join("lang", "dashboard"), dashboardKeys)
	assertLocaleKeys(t, filepath.Join("lang", "backend"), backendKeys)
}

func assertLocaleKeys(t *testing.T, dir string, keys []string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}
	locales := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		locales++
		var values map[string]any
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s/%s: %v", dir, entry.Name(), err)
		}
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("decode %s/%s: %v", dir, entry.Name(), err)
		}
		for _, key := range keys {
			value, ok := values[key].(string)
			if !ok || strings.TrimSpace(value) == "" {
				t.Errorf("%s/%s missing non-empty %q", dir, entry.Name(), key)
			}
		}
	}
	if locales != 16 {
		t.Fatalf("%s locales = %d, want 16", dir, locales)
	}
}
