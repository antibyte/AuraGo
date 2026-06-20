package ui

import (
	"strings"
	"testing"
)

func TestDashboardKnowledgeGraphHealthContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	css := readDesktopAssetText(t, "css/dashboard.css")

	for _, marker := range []string{
		`id="card-knowledge-graph-health"`,
		`id="knowledge-health-metrics"`,
		`id="knowledge-health-status"`,
		`dashboard.knowledge_health_title`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard HTML missing health marker %q", marker)
		}
	}
	for _, marker := range []string{
		"/api/knowledge-graph/health",
		"renderKnowledgeGraphHealth",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("dashboard main JS missing health marker %q", marker)
		}
	}
	for _, marker := range []string{
		"function renderKnowledgeGraphHealth",
		"knowledge_health_dirty_nodes",
		"knowledge_health_needs_reindex",
		"memory_graph_dirty_hint",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard widgets JS missing health marker %q", marker)
		}
	}
	if !strings.Contains(css, ".knowledge-health-status") {
		t.Fatal("dashboard CSS missing .knowledge-health-status")
	}
}