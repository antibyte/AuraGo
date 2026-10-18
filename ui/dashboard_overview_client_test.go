package ui

import (
	"strings"
	"testing"
)

// TestDashboardOverviewLoadsFromTheSummaryFeed pins the P3 client wiring:
//   - the overview loader renders from DashSummary instead of /api/dashboard/overview;
//   - both modules start before the router;
//   - five SSE handlers schedule a summary refresh;
//   - system_metrics updates ov-system in place, and the live dot follows AuraSSE;
//   - the banner and Quick Status renderers are gone.
func TestDashboardOverviewLoadsFromTheSummaryFeed(t *testing.T) {
	t.Parallel()

	overviewJS := readDesktopAssetText(t, "js/dashboard/sections/overview.js")
	start := strings.Index(overviewJS, "async function loadSectionOverview(")
	end := strings.Index(overviewJS, "DashSections.register({ id: 'overview', load: loadSectionOverview });")
	if start < 0 || end < start {
		t.Fatal("sections/overview.js misses loadSectionOverview before its DashSections.register call")
	}
	loader := overviewJS[start:end]
	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"const result = await DashSummary.refresh({ force });",
		"const summary = DashSummary.latest();",
		"if (!result.ok || !summary) {",
		"OVERVIEW_TILE_IDS.forEach(id => CardState.setError(id, retry, { status: result.status }));",
		"return false;",
		"DashOverview.render(summary);",
	} {
		if !strings.Contains(loader, marker) {
			t.Errorf("loadSectionOverview misses %q", marker)
		}
	}
	for _, gone := range []string{"/api/dashboard/overview", "renderAgentBanner", "renderQuickStatus", "card-quickstatus"} {
		if strings.Contains(loader, gone) {
			t.Errorf("loadSectionOverview still contains %q", gone)
		}
	}
	summaryInit := strings.Index(mainJS, "DashSummary.init();")
	overviewInit := strings.Index(mainJS, "DashOverview.init();")
	routerInit := strings.Index(mainJS, "DashRouter.init({ onShow: id => loadSectionContent(id) });")
	if summaryInit < 0 || overviewInit < 0 || routerInit < 0 || !(summaryInit < overviewInit && overviewInit < routerInit) {
		t.Errorf("initDashboard order = summary:%d overview:%d router:%d, want DashSummary.init, DashOverview.init, DashRouter.init", summaryInit, overviewInit, routerInit)
	}

	dashJS := readDashboardJS(t)
	for _, gone := range []string{"function renderAgentBanner(", "function renderQuickStatus(", "function formatDashboardTimestamp(", "qs-grid", "ab-model"} {
		if strings.Contains(dashJS, gone) {
			t.Errorf("the dashboard scripts still contain %q", gone)
		}
	}

	events := readDesktopAssetText(t, "js/dashboard/dashboard-events.js")
	if n := strings.Count(events, "DashSummary.scheduleRefresh();"); n != 5 {
		t.Errorf("dashboard-events.js schedules %d summary refreshes, want 5 (memory, personality, daemon, audit, budget_*)", n)
	}
	for _, marker := range []string{"DashOverview.updateHealth(sys);", "DashSummary.setLive('on');", "DashSummary.setLive('off');"} {
		if !strings.Contains(events, marker) {
			t.Errorf("dashboard-events.js misses %q", marker)
		}
	}
}
