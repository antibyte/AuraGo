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
