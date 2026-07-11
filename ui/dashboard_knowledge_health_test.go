package ui

import (
	"strings"
	"testing"
)

func TestDashboardKnowledgeGraphHealthContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	widgetsJS := readDesktopAssetText(t, "js/dashboard/widgets-knowledge.js")
	coreJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	css := readDesktopAssetText(t, "css/dashboard.css")

	for _, marker := range []string{
		`id="card-knowledge-graph-health"`,
		`id="knowledge-health-metrics"`,
		`id="knowledge-health-status"`,
		`id="knowledge-quality-id-duplicates"`,
		`dashboard.knowledge_health_title`,
		`dashboard.knowledge_quality_id_duplicates_title`,
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
		"knowledge_health_isolated_nodes",
		"knowledge_health_label_duplicate_groups",
		"knowledge_health_id_duplicate_groups",
		"renderKnowledgeGraphDuplicateCandidates",
		"knowledge_quality_id_duplicates",
		"knowledge_health_needs_reindex",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard knowledge widgets JS missing health marker %q", marker)
		}
	}
	for _, marker := range []string{
		"memory_graph_dirty_hint",
	} {
		if !strings.Contains(coreJS, marker) {
			t.Fatalf("dashboard core widgets JS missing health marker %q", marker)
		}
	}
	if !strings.Contains(css, ".knowledge-health-status") {
		t.Fatal("dashboard CSS missing .knowledge-health-status")
	}
}

func TestDashboardKnowledgeGraphVisualUsesBoundedCanvasSize(t *testing.T) {
	t.Parallel()

	widgetsJS := readDesktopAssetText(t, "js/dashboard/widgets-knowledge.js")
	css := readDesktopAssetText(t, "css/dashboard.css")

	for _, marker := range []string{
		"function knowledgeGraphVisualSize(wrap)",
		"parseFloat(style.height)",
		"wrap._forceGraph.width(size.width).height(size.height)",
		"wrap._forceGraph\n                .width(graphSize.width)\n                .height(graphSize.height)",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard knowledge graph visual sizing missing JS marker %q", marker)
		}
	}
	if strings.Contains(widgetsJS, "wrap.clientHeight || 360") {
		t.Fatal("dashboard knowledge graph visual must not size itself from content-driven clientHeight")
	}
	for _, marker := range []string{
		"height: clamp(360px, 42vh, 460px);",
		"min-height: 360px;",
		"max-height: 460px;",
		"overflow: hidden;",
		".knowledge-visual-wrap canvas",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("dashboard knowledge graph visual sizing missing CSS marker %q", marker)
		}
	}
}
