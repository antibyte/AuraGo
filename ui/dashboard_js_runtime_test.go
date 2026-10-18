package ui

import (
	"os/exec"
	"testing"
)

// runDashboardNodeCheck runs a ui/testdata/*.cjs runtime check for a dashboard script
// (go test runs in the package directory, so the path is relative to ui/).
func runDashboardNodeCheck(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the dashboard runtime checks")
	}
	if out, err := exec.Command(node, script).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", script, err, out)
	}
}

func TestDashboardRouterResolvesSections(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-router.cjs")
}

func TestDashboardAPICachesAndSharesRequests(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-api.cjs")
}

func TestDashboardSummaryFeedSchedulesRefreshes(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-summary.cjs")
}

func TestDashboardOverviewHelpers(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-overview.cjs")
}

func TestDashboardSectionRegistry(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-sections.cjs")
}

func TestDashboardEscapeHtmlComesFromMainJS(t *testing.T) {
	t.Parallel()
	runDashboardNodeCheck(t, "testdata/dash-page-scripts.cjs")
}
