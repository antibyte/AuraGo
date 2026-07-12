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
		`id="knowledge-health-consistency"`,
		`id="knowledge-quality-id-duplicates"`,
		`id="knowledge-quality-generic"`,
		`dashboard.knowledge_health_title`,
		`dashboard.knowledge_health_consistency_title`,
		`dashboard.knowledge_quality_id_duplicates_title`,
		`dashboard.knowledge_quality_generic_title`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard HTML missing health marker %q", marker)
		}
	}
	for _, marker := range []string{
		"/api/knowledge-graph/health",
		"renderKnowledgeGraphHealth",
		"markKnowledgeCard('card-knowledge-graph-health', healthR.ok, healthR.status);",
		"markKnowledgeCard('card-knowledge-graph-quality', qualityR.ok, qualityR.status);",
		"markKnowledgeCard('card-knowledge-graph-summary', importantR.ok && statsR.ok, firstKnowledgeStatus(importantR, statsR));",
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
		"function renderKnowledgeGraphConsistency",
		"knowledge_health_consistency_title",
		"nodes_missing_from_index",
		"renderKnowledgeGraphDuplicateCandidates",
		"knowledge_quality_id_duplicates",
		"knowledge_quality_empty_generic",
		"recommended_target_id",
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

func TestDashboardKnowledgeGraphHealthTranslations(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"dashboard.knowledge_health_consistency_title",
		"dashboard.knowledge_health_consistency_ok",
		"dashboard.knowledge_health_consistency_nodes_missing",
		"dashboard.knowledge_health_consistency_edges_missing",
		"dashboard.knowledge_health_consistency_stale_nodes",
		"dashboard.knowledge_health_consistency_index_orphans",
		"dashboard.knowledge_quality_generic_title",
		"dashboard.knowledge_quality_empty_generic",
	}
	for _, lang := range langs {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			content := readDesktopAssetText(t, "lang/dashboard/"+lang+".json")
			for _, key := range keys {
				if !strings.Contains(content, `"`+key+`"`) {
					t.Fatalf("dashboard %s translations missing %s", lang, key)
				}
			}
		})
	}
}

func TestDashboardKnowledgeGraphVisualUsesBoundedCanvasSize(t *testing.T) {
	t.Parallel()

	widgetsJS := strings.ReplaceAll(readDesktopAssetText(t, "js/dashboard/widgets-knowledge.js"), "\r\n", "\n")
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/dashboard.css"), "\r\n", "\n")

	for _, marker := range []string{
		"const KG_VISUAL_MAX_HEIGHT = 460;",
		"function knowledgeGraphVisualSize(wrap)",
		"parseFloat(style.height)",
		"height: Math.min(KG_VISUAL_MAX_HEIGHT, Math.max(KG_VISUAL_MIN_HEIGHT, height))",
		"window.requestAnimationFrame(() => {",
		"if (wrap._forceGraphSize && wrap._forceGraphSize.width === size.width && wrap._forceGraphSize.height === size.height) return;",
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
		"contain: layout paint;",
		"overflow: hidden;",
		".knowledge-visual-wrap > div",
		".knowledge-visual-wrap canvas",
		"height: 100% !important;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("dashboard knowledge graph visual sizing missing CSS marker %q", marker)
		}
	}
}
