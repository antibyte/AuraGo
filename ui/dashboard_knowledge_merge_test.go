package ui

import (
	"strings"
	"testing"
)

func TestDashboardKnowledgeGraphMergeContract(t *testing.T) {
	t.Parallel()

	widgetsJS := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"function mergeKnowledgeGraphNodes",
		"/api/knowledge-graph/merge",
		"data-kg-merge-source",
		"showConfirm(",
		"knowledge_quality_merge_btn",
	} {
		if !strings.Contains(widgetsJS, marker) {
			t.Fatalf("dashboard widgets JS missing merge marker %q", marker)
		}
	}
	if !strings.Contains(mainJS, "data-kg-merge-source") || !strings.Contains(mainJS, "mergeKnowledgeGraphNodes") {
		t.Fatal("dashboard main JS missing knowledge graph merge handler")
	}
	if strings.Contains(widgetsJS+mainJS, "alert(") {
		t.Fatal("dashboard knowledge merge UI must use modals/toasts instead of alert()")
	}
}