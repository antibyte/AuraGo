package memory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKGSearchReportsFTSFailureAndKeepsLikeResults(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("alice", "Alice Smith", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts`); err != nil {
		t.Fatalf("drop kg_nodes_fts: %v", err)
	}

	result, err := kg.SearchResultWithOptions("Alice", KnowledgeGraphQueryOptions{})
	if err == nil || !strings.Contains(err.Error(), "kg_nodes_fts") {
		t.Fatalf("err = %v, want the failed FTS query to be named", err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].ID != "alice" {
		t.Fatalf("nodes = %+v, want the LIKE fallback to keep alice", result.Nodes)
	}

	raw := kg.SearchWithOptions("Alice", KnowledgeGraphQueryOptions{})
	var payload struct {
		Nodes  []Node `json:"nodes"`
		Errors string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if len(payload.Nodes) != 1 || !strings.Contains(payload.Errors, "kg_nodes_fts") {
		t.Fatalf("partial search JSON = %s, want nodes plus a named error", raw)
	}
}

func TestKGSearchForContextLikeFallbackTreatsWildcardsLiterally(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("pct", "100% done", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode pct: %v", err)
	}
	if err := kg.AddNode("under", "100_done", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode under: %v", err)
	}
	// Without the FTS table the FTS query fails and SearchForContext falls back to LIKE.
	if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts`); err != nil {
		t.Fatalf("drop kg_nodes_fts: %v", err)
	}

	ids := func(query string) map[string]bool {
		t.Helper()
		found := map[string]bool{}
		for _, n := range kg.SearchForContextStructured(query, 5, 800).Nodes {
			found[n.ID] = true
		}
		return found
	}

	if got := ids("100%"); !got["pct"] || got["under"] {
		t.Fatalf("query 100%% found %v, want only pct (escaped %% is literal)", got)
	}
	if got := ids("100_"); !got["under"] || got["pct"] {
		t.Fatalf("query 100_ found %v, want only under (escaped _ is literal)", got)
	}
}

func TestKGSearchLikeTreatsBackslashLiterally(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("share", `share\docs`, map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode share: %v", err)
	}
	if err := kg.AddNode("plain", "sharedocs", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode plain: %v", err)
	}
	if err := kg.AddEdge("share", "nas", `backs\up`, nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	// Force every search path onto LIKE.
	for _, table := range []string{"kg_nodes_fts", "kg_edges_fts"} {
		if _, err := kg.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}

	result, _ := kg.SearchResultWithOptions(`share\docs`, KnowledgeGraphQueryOptions{})
	if len(result.Nodes) != 1 || result.Nodes[0].ID != "share" {
		t.Fatalf(`SearchResultWithOptions(share\docs) nodes = %+v, want only share`, result.Nodes)
	}
	edges, _ := kg.SearchResultWithOptions(`backs\up`, KnowledgeGraphQueryOptions{})
	if len(edges.Edges) != 1 || edges.Edges[0].Relation != `backs\up` {
		t.Fatalf(`SearchResultWithOptions(backs\up) edges = %+v, want the backs\up edge`, edges.Edges)
	}
	ctx := kg.SearchForContextStructured(`share\docs`, 5, 800)
	if len(ctx.Nodes) != 1 || ctx.Nodes[0].ID != "share" {
		t.Fatalf(`SearchForContextStructured(share\docs) nodes = %+v, want only share`, ctx.Nodes)
	}
}

func TestKGSearchReportsDatabaseFailureInsteadOfEmptyResult(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("alice", "Alice Smith", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := kg.SearchResultWithOptions("Alice", KnowledgeGraphQueryOptions{}); err == nil {
		t.Fatal("SearchResultWithOptions on a closed database returned no error")
	}
	raw := kg.SearchWithOptions("Alice", KnowledgeGraphQueryOptions{})
	if raw == "[]" || !strings.Contains(raw, `"error"`) {
		t.Fatalf("SearchWithOptions = %s, want a JSON error instead of an empty result", raw)
	}
}
