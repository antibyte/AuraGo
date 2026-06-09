package ui

import (
	"strings"
	"testing"
)

func TestDashboardMemoryCurationContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	for _, marker := range []string{
		`id="memory-health-summary"`,
		`id="memory-curator-list"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard memory UI missing marker %q", marker)
		}
	}

	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"/api/dashboard/memory/curation/dry-run",
		"/api/dashboard/memory/curation/apply",
		"/api/dashboard/memory/hygiene/dry-run",
		"/api/dashboard/memory/hygiene/apply",
		"APPLY_MEMORY_CURATION",
		"APPLY_MEMORY_HYGIENE",
		"runMemoryCurationDryRun",
		"runMemoryHygieneDryRun",
		"applyMemoryCurationSafeActions",
		"applyMemoryHygieneSafeActions",
		"memory-curator-actionbar",
		"memory-hygiene-panel",
		"dashboard.memory_curator_archived",
		"dashboard.memory_hygiene_title",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard widgets JS missing memory curation marker %q", marker)
		}
	}
	if strings.Contains(widgetsJS, "alert(") {
		t.Fatal("dashboard memory curation UI must use modals/toasts instead of alert()")
	}
}
