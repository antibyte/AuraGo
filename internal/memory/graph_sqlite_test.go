package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory/kgquery"
	"aurago/internal/memory/kgsemantic"

	chromem "github.com/philippgille/chromem-go"
)

func newTestKG(t *testing.T) *KnowledgeGraph {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { kg.Close() })
	return kg
}

func TestKGAddNodeAndStats(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("alice", "Alice Smith", map[string]string{"type": "person"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("bob", "Bob Jones", map[string]string{"type": "person"}); err != nil {
		t.Fatal(err)
	}

	nodes, edges, _ := kg.Stats()
	if nodes != 2 {
		t.Errorf("expected 2 nodes, got %d", nodes)
	}
	if edges != 0 {
		t.Errorf("expected 0 edges, got %d", edges)
	}
}

func TestKGAddNodeRejectsBlankIDAndTrimsStoredID(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("   ", "Blank", nil); err == nil {
		t.Fatal("expected blank node id to be rejected")
	}
	if err := kg.AddNode("  trimmed_node  ", "Trimmed", nil); err != nil {
		t.Fatalf("AddNode with padded id: %v", err)
	}
	if node, err := kg.GetNode("trimmed_node"); err != nil || node == nil {
		t.Fatalf("expected trimmed node id to be stored, node=%v err=%v", node, err)
	}
	if node, err := kg.GetNode("  trimmed_node  "); err != nil || node != nil {
		t.Fatalf("expected padded node id not to be stored, node=%v err=%v", node, err)
	}
}

func TestKGAddEdge(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("alice", "bob", "knows", nil); err != nil {
		t.Fatal(err)
	}

	nodes, edges, _ := kg.Stats()
	if nodes != 2 { // auto-created
		t.Errorf("expected 2 auto-created nodes, got %d", nodes)
	}
	if edges != 1 {
		t.Errorf("expected 1 edge, got %d", edges)
	}
}

func TestKGAddEdgeDefaultsQualityMetadata(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("andi", "agodesk", "uses", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	edge := mustFindTestEdge(t, kg, "andi", "agodesk", "uses")
	if edge.Properties["source"] != "manual" {
		t.Fatalf("source = %q, want manual in %#v", edge.Properties["source"], edge.Properties)
	}
	if edge.Properties["confidence"] != "1.00" {
		t.Fatalf("confidence = %q, want 1.00 in %#v", edge.Properties["confidence"], edge.Properties)
	}
	if edge.Properties["extracted_at"] != time.Now().Format("2006-01-02") {
		t.Fatalf("extracted_at = %q, want today in %#v", edge.Properties["extracted_at"], edge.Properties)
	}
}

func TestKGAddEdgeRejectsBlankEndpointOrRelationAndTrimsIdentity(t *testing.T) {
	kg := newTestKG(t)

	for _, tc := range []struct {
		name     string
		source   string
		target   string
		relation string
	}{
		{name: "blank source", source: " ", target: "target", relation: "rel"},
		{name: "blank target", source: "source", target: " ", relation: "rel"},
		{name: "blank relation", source: "source", target: "target", relation: " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := kg.AddEdge(tc.source, tc.target, tc.relation, nil); err == nil {
				t.Fatal("expected invalid edge identity to be rejected")
			}
		})
	}

	if err := kg.AddEdge(" source ", " target ", " relates_to ", nil); err != nil {
		t.Fatalf("AddEdge with padded identity: %v", err)
	}
	nodes, edges := kg.GetNeighbors("source", 10)
	if len(nodes) != 1 || len(edges) != 1 || edges[0].Source != "source" || edges[0].Target != "target" || edges[0].Relation != "relates_to" {
		t.Fatalf("expected trimmed edge identity, nodes=%v edges=%v", nodes, edges)
	}
}

func TestKGAddEdgeUpsert(t *testing.T) {
	kg := newTestKG(t)

	kg.AddEdge("a", "b", "rel", map[string]string{"weight": "1"})
	kg.AddEdge("a", "b", "rel", map[string]string{"weight": "2"})

	_, edges, _ := kg.Stats()
	if edges != 1 {
		t.Errorf("expected 1 edge after upsert, got %d", edges)
	}
}

func TestKGBulkMergeExtractedEntitiesDefaultsAndPreservesEdgeQualityMetadata(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.BulkMergeExtractedEntities(nil, []Edge{
		{Source: "andi", Target: "agodesk", Relation: "uses"},
		{
			Source:   "andi",
			Target:   "caddy",
			Relation: "manages",
			Properties: map[string]string{
				"source":       "auto_extraction",
				"confidence":   "0.90",
				"extracted_at": "2026-01-01",
				"detail":       "existing",
			},
		},
	}); err != nil {
		t.Fatalf("initial BulkMergeExtractedEntities: %v", err)
	}

	defaulted := mustFindTestEdge(t, kg, "andi", "agodesk", "uses")
	if defaulted.Properties["source"] != "auto_extraction" {
		t.Fatalf("defaulted source = %q, want auto_extraction in %#v", defaulted.Properties["source"], defaulted.Properties)
	}
	if defaulted.Properties["confidence"] != "0.50" {
		t.Fatalf("defaulted confidence = %q, want 0.50 in %#v", defaulted.Properties["confidence"], defaulted.Properties)
	}
	if defaulted.Properties["extracted_at"] != time.Now().Format("2006-01-02") {
		t.Fatalf("defaulted extracted_at = %q, want today in %#v", defaulted.Properties["extracted_at"], defaulted.Properties)
	}

	if err := kg.BulkMergeExtractedEntities(nil, []Edge{{
		Source:   "andi",
		Target:   "caddy",
		Relation: "manages",
		Properties: map[string]string{
			"confidence": "0.20",
			"detail":     "incoming",
		},
	}}); err != nil {
		t.Fatalf("second BulkMergeExtractedEntities: %v", err)
	}
	preserved := mustFindTestEdge(t, kg, "andi", "caddy", "manages")
	if preserved.Properties["source"] != "auto_extraction" ||
		preserved.Properties["confidence"] != "0.90" ||
		preserved.Properties["extracted_at"] != "2026-01-01" ||
		preserved.Properties["detail"] != "existing" {
		t.Fatalf("expected higher-confidence existing edge metadata to be preserved, got %#v", preserved.Properties)
	}
}

func mustFindTestEdge(t *testing.T, kg *KnowledgeGraph, source, target, relation string) Edge {
	t.Helper()
	edges, err := kg.GetAllEdges(100)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	for _, edge := range edges {
		if edge.Source == source && edge.Target == target && edge.Relation == relation {
			return edge
		}
	}
	t.Fatalf("missing edge %s -> %s / %s in %#v", source, target, relation, edges)
	return Edge{}
}

func TestKGSearch(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("home_server", "Home Server", map[string]string{"type": "device", "os": "linux"})
	kg.AddNode("work_laptop", "Work Laptop", map[string]string{"type": "device", "os": "windows"})
	kg.AddEdge("home_server", "work_laptop", "connected_to", nil)

	result := kg.Search("server")
	if result == "[]" {
		t.Error("expected search results for 'server', got empty")
	}

	result = kg.Search("nonexistent_xyz_12345")
	if result != "[]" {
		t.Errorf("expected empty results for nonexistent query, got: %s", result)
	}
}

func TestKGSearchHidesLowConfidenceCoMentionsByDefault(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("andi", "Andi", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode andi: %v", err)
	}
	if err := kg.AddNode("png", "png", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode png: %v", err)
	}
	if err := kg.AddEdge("andi", "png", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge pending: %v", err)
	}

	var defaultPayload struct {
		Edges []Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(kg.Search("andi")), &defaultPayload); err != nil {
		t.Fatalf("unmarshal default search: %v", err)
	}
	if len(defaultPayload.Edges) != 0 {
		t.Fatalf("default search should hide low-confidence co-mentions, got %#v", defaultPayload.Edges)
	}

	var overridePayload struct {
		Edges []Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(kg.SearchWithOptions("andi", KnowledgeGraphQueryOptions{IncludeLowConfidence: true})), &overridePayload); err != nil {
		t.Fatalf("unmarshal override search: %v", err)
	}
	if len(overridePayload.Edges) != 1 || overridePayload.Edges[0].Relation != "co_mentioned_with" {
		t.Fatalf("override search should include low-confidence co-mention, got %#v", overridePayload.Edges)
	}
}

func TestEscapeFTS5PreservesQuotedPhrase(t *testing.T) {
	got := kgquery.EscapeFTS5(`"Raspberry Pi"`)
	want := `"Raspberry Pi"`
	if got != want {
		t.Fatalf("escapeFTS5 quoted phrase = %q, want %q", got, want)
	}
}

func TestEscapeFTS5EscapesInternalQuotes(t *testing.T) {
	got := kgquery.EscapeFTS5(`"say ""hello"""`)
	want := `"say """"hello"""""`
	if got != want {
		t.Fatalf("escapeFTS5 internal quotes = %q, want %q", got, want)
	}
}

func TestEscapeFTS5UsesANDForMultipleWords(t *testing.T) {
	got := kgquery.EscapeFTS5("docker host")
	want := `"docker" AND "host"`
	if got != want {
		t.Fatalf("escapeFTS5 multi-word = %q, want %q", got, want)
	}
}

func TestKGPruneOutgoingRelationEdgesRemovesStaleTargets(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddEdge("contact_1", "org_old", "belongs_to", nil); err != nil {
		t.Fatalf("AddEdge old: %v", err)
	}
	if err := kg.AddEdge("contact_1", "org_new", "belongs_to", nil); err != nil {
		t.Fatalf("AddEdge new: %v", err)
	}

	removed, err := kg.PruneOutgoingRelationEdges("contact_1", "belongs_to", map[string]struct{}{"org_new": {}})
	if err != nil {
		t.Fatalf("PruneOutgoingRelationEdges: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	edges, err := kg.GetImportantEdges(10, []string{"contact_1"})
	if err != nil {
		t.Fatalf("GetImportantEdges: %v", err)
	}
	for _, edge := range edges {
		if edge.Relation == "belongs_to" && edge.Target == "org_old" {
			t.Fatal("expected stale org_old belongs_to edge to be removed")
		}
	}
}

func TestKGFlushAccessHitsPersistsQueuedNodeAccess(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("accessed", "Accessed", nil); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: "accessed"})
	if err := kg.FlushAccessHits(); err != nil {
		t.Fatalf("FlushAccessHits: %v", err)
	}
	var count int
	if err := kg.db.QueryRow("SELECT access_count FROM kg_nodes WHERE id = ?", "accessed").Scan(&count); err != nil {
		t.Fatalf("query access_count: %v", err)
	}
	if count < 1 {
		t.Fatalf("access_count = %d, want >= 1 after flush", count)
	}
}

func TestKGAccessCountReliableNilGraphIsFalse(t *testing.T) {
	var kg *KnowledgeGraph
	if kg.accessCountReliable() {
		t.Fatal("nil knowledge graph should not report reliable access counts")
	}
}

func TestKGSearchFTS5(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("raspberry_pi", "Raspberry Pi 4", map[string]string{"type": "device"})
	kg.AddNode("docker_host", "Docker Host Server", map[string]string{"type": "service"})

	result := kg.Search("raspberry")
	if result == "[]" {
		t.Error("FTS5 search for 'raspberry' should return results")
	}
}

func TestKGGetNeighborsHidesLowConfidenceCoMentionsBeforeLimit(t *testing.T) {
	kg := newTestKG(t)

	for _, node := range []Node{
		{ID: "andi", Label: "Andi", Properties: map[string]string{"type": "person"}},
		{ID: "agodesk", Label: "AgoDesk", Properties: map[string]string{"type": "service"}},
		{ID: "png", Label: "png", Properties: map[string]string{"type": "concept"}},
	} {
		if err := kg.AddNode(node.ID, node.Label, node.Properties); err != nil {
			t.Fatalf("AddNode %s: %v", node.ID, err)
		}
	}
	if err := kg.AddEdge("andi", "agodesk", "uses", map[string]string{"source": "manual"}); err != nil {
		t.Fatalf("AddEdge uses: %v", err)
	}
	if err := kg.AddEdge("andi", "png", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge pending: %v", err)
	}

	nodes, edges := kg.GetNeighbors("andi", 1)
	if len(nodes) != 1 || nodes[0].ID != "agodesk" {
		t.Fatalf("default neighbors should skip pending edge before limit, nodes=%#v edges=%#v", nodes, edges)
	}
	for _, edge := range edges {
		if edge.Relation == "co_mentioned_with" {
			t.Fatalf("default neighbors should hide low-confidence co-mentions, got %#v", edges)
		}
	}

	nodes, edges = kg.GetNeighborsWithOptions("andi", 20, KnowledgeGraphQueryOptions{IncludeLowConfidence: true})
	if len(nodes) < 2 {
		t.Fatalf("override neighbors should include both neighbors, nodes=%#v edges=%#v", nodes, edges)
	}
	var foundPending bool
	for _, edge := range edges {
		if edge.Relation == "co_mentioned_with" {
			foundPending = true
		}
	}
	if !foundPending {
		t.Fatalf("override neighbors should include pending co-mention, got %#v", edges)
	}
}

func TestKGExploreFallsBackToFTSWhenSemanticReturnsNoNodes(t *testing.T) {
	kg := newTestKG(t)

	db := chromem.NewDB()
	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		if strings.EqualFold(strings.TrimSpace(text), "backup") {
			return []float32{1, 0}, nil
		}
		return []float32{0, 1}, nil
	}
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	var payload struct {
		Nodes []Node `json:"nodes"`
	}
	result := kg.Explore("backup")
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal explore result %q: %v", result, err)
	}
	for _, node := range payload.Nodes {
		if node.ID == "backup_server" {
			return
		}
	}
	t.Fatalf("expected FTS fallback to include backup_server, got %q", result)
}

func TestKGExploreDeduplicatesSharedEdges(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("alpha_lab", "Alpha Datalab", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode alpha_lab: %v", err)
	}
	if err := kg.AddNode("beta_lab", "Beta Datalab", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode beta_lab: %v", err)
	}
	if err := kg.AddEdge("alpha_lab", "beta_lab", "related_to", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	var payload struct {
		Edges []Edge `json:"edges"`
	}
	result := kg.Explore("datalab")
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal explore result %q: %v", result, err)
	}
	if len(payload.Edges) != 1 {
		t.Fatalf("expected 1 deduplicated edge, got %d in %q", len(payload.Edges), result)
	}
}

func TestKGSearchReturnsMatchingNodesDespiteCorruptEdgeJSON(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("good_node", "Rack-B Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.AddNode("peer", "Peer", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode peer: %v", err)
	}
	if _, err := kg.db.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties)
		VALUES (?, ?, ?, ?)
	`, "good_node", "peer", "connects_to", "{not-json"); err != nil {
		t.Fatalf("insert corrupt edge: %v", err)
	}

	result := kg.Search("rack-b")
	if result == "[]" {
		t.Fatal("expected node match despite corrupt edge JSON")
	}
	var payload struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal search result: %v", err)
	}
	if len(payload.Nodes) == 0 {
		t.Fatalf("expected matching node in partial search result, got %q", result)
	}
}

func TestKGEnableSemanticSearchDisabledConfigIsNoOp(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.EnableSemanticSearch(nil); err == nil {
		t.Fatal("expected nil config error")
	}
	if err := kg.EnableSemanticSearch(&config.Config{}); err != nil {
		t.Fatalf("disabled embeddings should remain no-op, got %v", err)
	}
}

func TestKGEdgeFTS5Indexing(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("docker_host", "compose_stack", "runs_stack", map[string]string{
		"notes": "container orchestration for homelab services",
	}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	var count int
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_edges_fts WHERE kg_edges_fts MATCH ?`, "orchestration").Scan(&count); err != nil {
		t.Fatalf("query kg_edges_fts: %v", err)
	}
	if count == 0 {
		t.Fatal("expected kg_edges_fts to index edge properties")
	}
}

func TestKnowledgeGraphRebuildsLegacyFTSSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kg.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	legacyStatements := []string{
		`CREATE TABLE IF NOT EXISTS kg_nodes (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT NOT NULL UNIQUE,
			label TEXT NOT NULL DEFAULT '',
			properties TEXT NOT NULL DEFAULT '{}',
			access_count INTEGER NOT NULL DEFAULT 0,
			protected INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS kg_edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			target TEXT NOT NULL,
			relation TEXT NOT NULL,
			properties TEXT NOT NULL DEFAULT '{}',
			access_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(source, target, relation)
		);`,
		`CREATE VIRTUAL TABLE kg_nodes_fts USING fts5(
			id, label, properties_text, content=kg_nodes, content_rowid=rowid
		);`,
		`CREATE VIRTUAL TABLE kg_edges_fts USING fts5(
			source, target, relation, properties_text, content=kg_edges, content_rowid=id
		);`,
	}
	for _, stmt := range legacyStatements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup legacy stmt failed: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO kg_nodes (id, label, properties) VALUES ('legacy_node', 'Legacy Node', '{"type":"service"}')`); err != nil {
		t.Fatalf("insert legacy node: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO kg_nodes (id, label, properties) VALUES ('target', 'Target Node', '{"type":"service"}')`); err != nil {
		t.Fatalf("insert legacy target node: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO kg_edges (source, target, relation, properties) VALUES ('legacy_node', 'target', 'relates_to', '{"notes":"legacy edge"}')`); err != nil {
		t.Fatalf("insert legacy edge: %v", err)
	}
	_ = db.Close()

	kg, err := NewKnowledgeGraph(dbPath, "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph with legacy FTS schema: %v", err)
	}
	defer kg.Close()

	if result := kg.Search("legacy"); result == "[]" {
		t.Fatal("expected search results after legacy FTS rebuild")
	}

	var hasEdgeUpdatedAt bool
	if err := kg.db.QueryRow("SELECT count(*)>0 FROM pragma_table_info('kg_edges') WHERE name='updated_at'").Scan(&hasEdgeUpdatedAt); err != nil {
		t.Fatalf("query kg_edges updated_at column: %v", err)
	}
	if !hasEdgeUpdatedAt {
		t.Fatal("legacy kg_edges table was not migrated with updated_at")
	}

	var edgeUpdatedAt string
	if err := kg.db.QueryRow(`SELECT COALESCE(updated_at, '') FROM kg_edges WHERE source='legacy_node' AND target='target' AND relation='relates_to'`).Scan(&edgeUpdatedAt); err != nil {
		t.Fatalf("query migrated edge updated_at: %v", err)
	}
	if strings.TrimSpace(edgeUpdatedAt) == "" {
		t.Fatal("legacy kg_edges.updated_at was not backfilled")
	}
}

func TestKGDeleteNode(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("alice", "Alice", nil)
	kg.AddEdge("alice", "bob", "knows", nil)

	if err := kg.DeleteNode("alice"); err != nil {
		t.Fatal(err)
	}

	nodes, edges, _ := kg.Stats()
	if nodes != 1 { // bob remains
		t.Errorf("expected 1 node after delete, got %d", nodes)
	}
	if edges != 0 { // edge removed
		t.Errorf("expected 0 edges after node delete, got %d", edges)
	}
}

func TestKGEdgesCascadeWhenNodeDeletedDirectly(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("alice", "Alice", nil); err != nil {
		t.Fatalf("AddNode alice: %v", err)
	}
	if err := kg.AddEdge("alice", "bob", "knows", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	if _, err := kg.db.Exec("DELETE FROM kg_nodes WHERE id = ?", "alice"); err != nil {
		t.Fatalf("delete kg_node directly: %v", err)
	}

	var edgeCount int
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE source = ? OR target = ?", "alice", "alice").Scan(&edgeCount); err != nil {
		t.Fatalf("count cascaded edges: %v", err)
	}
	if edgeCount != 0 {
		t.Fatalf("expected cascaded edges to be deleted, got %d", edgeCount)
	}
}

func TestKGDeleteProtectedNodeRejected(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("important", "Important", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	err := kg.DeleteNode("important")
	if err == nil {
		t.Fatal("expected protected node delete to fail")
	}
	if !errors.Is(err, ErrKnowledgeGraphProtectedNode) {
		t.Fatalf("expected ErrKnowledgeGraphProtectedNode, got %v", err)
	}
}

func TestKGMergeNodesRejectsProtectedSource(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("target", "Target", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode target: %v", err)
	}
	if err := kg.AddNode("protected_source", "Protected", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode protected_source: %v", err)
	}

	err := kg.MergeNodes("target", "protected_source")
	if err == nil {
		t.Fatal("expected protected source merge to fail")
	}
	if !errors.Is(err, ErrKnowledgeGraphProtectedNode) {
		t.Fatalf("expected ErrKnowledgeGraphProtectedNode, got %v", err)
	}
}

func TestKGDeleteEdge(t *testing.T) {
	kg := newTestKG(t)

	kg.AddEdge("a", "b", "rel1", nil)
	kg.AddEdge("a", "b", "rel2", nil)

	if err := kg.DeleteEdge("a", "b", "rel1"); err != nil {
		t.Fatal(err)
	}

	_, edges, _ := kg.Stats()
	if edges != 1 {
		t.Errorf("expected 1 edge after delete, got %d", edges)
	}
}

func TestKGMergeNodesMovesEdgesAndMergesProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("target", "Target Node", map[string]string{"type": "service", "source": "manual"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("source", "Source Node", map[string]string{"ip": "192.168.1.10", "role": "backup"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("source", "peer", "connects_to", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("client", "source", "uses", nil); err != nil {
		t.Fatal(err)
	}

	if err := kg.MergeNodes("target", "source"); err != nil {
		t.Fatalf("MergeNodes: %v", err)
	}

	sourceNode, err := kg.GetNode("source")
	if err != nil {
		t.Fatalf("GetNode(source): %v", err)
	}
	if sourceNode != nil {
		t.Fatal("expected source node to be removed after merge")
	}

	targetNode, err := kg.GetNode("target")
	if err != nil {
		t.Fatalf("GetNode(target): %v", err)
	}
	if targetNode == nil {
		t.Fatal("expected target node to remain after merge")
	}
	if targetNode.Properties["ip"] != "192.168.1.10" {
		t.Fatalf("expected merged target to keep source properties, got %#v", targetNode.Properties)
	}
	if targetNode.Properties["role"] != "backup" {
		t.Fatalf("expected merged target to keep source role, got %#v", targetNode.Properties)
	}

	edges, err := kg.GetAllEdges(20)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	foundOutgoing := false
	foundIncoming := false
	for _, edge := range edges {
		if edge.Source == "target" && edge.Target == "peer" && edge.Relation == "connects_to" {
			foundOutgoing = true
		}
		if edge.Source == "client" && edge.Target == "target" && edge.Relation == "uses" {
			foundIncoming = true
		}
		if edge.Source == "source" || edge.Target == "source" {
			t.Fatalf("found stale edge still referencing source node: %#v", edge)
		}
	}
	if !foundOutgoing || !foundIncoming {
		t.Fatalf("expected merged edges to be rewired to target, got %#v", edges)
	}
}

func TestKGUpdateEdge(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("a", "b", "rel1", map[string]string{"notes": "before"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	edge, err := kg.UpdateEdge("a", "b", "rel1", "rel2", map[string]string{"notes": "after"})
	if err != nil {
		t.Fatalf("UpdateEdge: %v", err)
	}
	if edge == nil {
		t.Fatal("expected updated edge")
	}
	if edge.Relation != "rel2" {
		t.Fatalf("relation = %q, want rel2", edge.Relation)
	}
	if edge.Properties["notes"] != "after" {
		t.Fatalf("notes = %q, want after", edge.Properties["notes"])
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Relation != "rel2" {
		t.Fatalf("unexpected edges after update: %#v", edges)
	}
}

func TestKGQualityReport(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("router", "Router", map[string]string{"type": "device", "protected": "true"}); err != nil {
		t.Fatalf("AddNode router: %v", err)
	}
	if err := kg.AddNode("nas_primary", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode nas_primary: %v", err)
	}
	// Force a duplicate by bypassing AddNode which would otherwise merge it.
	if _, err := kg.db.Exec("INSERT INTO kg_nodes (id, label, properties) VALUES (?, ?, ?)", "nas_secondary", "NAS", "{}"); err != nil {
		t.Fatalf("Insert orphaned NAS: %v", err)
	}

	if err := kg.AddNode("orphan", "Orphan", nil); err != nil {
		t.Fatalf("AddNode orphan: %v", err)
	}
	if err := kg.AddEdge("router", "nas_primary", "backs_up", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	report, err := kg.QualityReport(10)
	if err != nil {
		t.Fatalf("QualityReport: %v", err)
	}
	if report.Nodes != 4 {
		t.Fatalf("Nodes = %d, want 4", report.Nodes)
	}
	if report.Edges != 1 {
		t.Fatalf("Edges = %d, want 1", report.Edges)
	}
	if report.ProtectedNodes != 1 {
		t.Fatalf("ProtectedNodes = %d, want 1", report.ProtectedNodes)
	}
	if report.IsolatedNodes != 2 {
		t.Fatalf("IsolatedNodes = %d, want 2", report.IsolatedNodes)
	}
	if report.UntypedNodes != 2 {
		t.Fatalf("UntypedNodes = %d, want 2", report.UntypedNodes)
	}
	if report.DuplicateGroups != 1 {
		t.Fatalf("DuplicateGroups = %d, want 1", report.DuplicateGroups)
	}
	if report.DuplicateNodes != 2 {
		t.Fatalf("DuplicateNodes = %d, want 2", report.DuplicateNodes)
	}
	if len(report.DuplicateCandidates) != 1 {
		t.Fatalf("DuplicateCandidates len = %d, want 1", len(report.DuplicateCandidates))
	}
	gotIDs := strings.Join(report.DuplicateCandidates[0].IDs, ",")
	if gotIDs != "nas_primary,nas_secondary" {
		t.Fatalf("duplicate IDs = %q, want nas_primary,nas_secondary", gotIDs)
	}
}

func TestKGQualityReportDetectsIDDuplicateVariants(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("truenas", "TrueNAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode truenas: %v", err)
	}
	if _, err := kg.db.Exec("INSERT INTO kg_nodes (id, label, properties) VALUES (?, ?, ?)", "true_nas", "TrueNAS Alt", "{}"); err != nil {
		t.Fatalf("Insert true_nas: %v", err)
	}

	report, err := kg.QualityReport(10)
	if err != nil {
		t.Fatalf("QualityReport: %v", err)
	}
	if report.IDDuplicateGroups != 1 {
		t.Fatalf("IDDuplicateGroups = %d, want 1", report.IDDuplicateGroups)
	}
	if report.IDDuplicateNodes != 2 {
		t.Fatalf("IDDuplicateNodes = %d, want 2", report.IDDuplicateNodes)
	}
	if len(report.IDDuplicateCandidates) != 1 {
		t.Fatalf("IDDuplicateCandidates len = %d, want 1", len(report.IDDuplicateCandidates))
	}
	gotIDs := strings.Join(report.IDDuplicateCandidates[0].IDs, ",")
	if gotIDs != "truenas,true_nas" {
		t.Fatalf("id duplicate IDs = %q, want truenas,true_nas", gotIDs)
	}
}

func TestKGQualityReportSuggestsNormalizedIDDuplicatesWithoutAutoMerge(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("caddy_server", "Caddy Server", map[string]string{
		"type":       "service",
		"source":     "manual",
		"confidence": "1.00",
	}); err != nil {
		t.Fatalf("AddNode caddy_server: %v", err)
	}
	if err := kg.AddNode("caddyserver", "Caddy Server", map[string]string{
		"type":       "service",
		"source":     "auto_extraction",
		"confidence": "0.70",
	}); err != nil {
		t.Fatalf("AddNode caddyserver: %v", err)
	}

	report, err := kg.QualityReport(10)
	if err != nil {
		t.Fatalf("QualityReport: %v", err)
	}

	found := false
	for _, candidate := range report.IDDuplicateCandidates {
		haveA, haveB := false, false
		for _, id := range candidate.IDs {
			if id == "caddy_server" {
				haveA = true
			}
			if id == "caddyserver" {
				haveB = true
			}
		}
		if haveA && haveB {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected caddy_server and caddyserver duplicate suggestion, got %#v", report.IDDuplicateCandidates)
	}

	for _, id := range []string{"caddy_server", "caddyserver"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("expected node %s to remain unmerged", id)
		}
	}
}

func TestKGHealthReportIncludesQualityKPIs(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("truenas", "TrueNAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode truenas: %v", err)
	}
	if _, err := kg.db.Exec("INSERT INTO kg_nodes (id, label, properties) VALUES (?, ?, ?)", "true_nas", "TrueNAS Alt", "{}"); err != nil {
		t.Fatalf("Insert true_nas: %v", err)
	}

	report, err := kg.HealthReport()
	if err != nil {
		t.Fatalf("HealthReport: %v", err)
	}
	if report.IsolatedNodes != 2 {
		t.Fatalf("IsolatedNodes = %d, want 2", report.IsolatedNodes)
	}
	if report.DuplicateGroups != 1 {
		t.Fatalf("DuplicateGroups = %d, want 1", report.DuplicateGroups)
	}
	if report.LabelDuplicateGroups != 0 {
		t.Fatalf("LabelDuplicateGroups = %d, want 0", report.LabelDuplicateGroups)
	}
	if report.IDDuplicateGroups != 1 {
		t.Fatalf("IDDuplicateGroups = %d, want 1", report.IDDuplicateGroups)
	}
}

func TestKGQualityReportIgnoresUnrelatedIDVariants(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("contact_12", "Contact 12", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode contact_12: %v", err)
	}
	if _, err := kg.db.Exec("INSERT INTO kg_nodes (id, label, properties) VALUES (?, ?, ?)", "contact12", "Billing Service", "{}"); err != nil {
		t.Fatalf("Insert contact12: %v", err)
	}

	report, err := kg.QualityReport(10)
	if err != nil {
		t.Fatalf("QualityReport: %v", err)
	}
	if report.IDDuplicateGroups != 0 {
		t.Fatalf("IDDuplicateGroups = %d, want 0 for unrelated labels", report.IDDuplicateGroups)
	}
}

func TestKGQualityReportReturnsQueryErrors(t *testing.T) {
	kg := newTestKG(t)

	if _, err := kg.db.Exec("DROP TABLE kg_nodes"); err != nil {
		t.Fatalf("drop kg_nodes: %v", err)
	}

	if _, err := kg.QualityReport(10); err == nil {
		t.Fatal("QualityReport() error = nil, want schema query error")
	}
}

func TestKGOptimizeGraph(t *testing.T) {
	kg := newTestKG(t)

	// Add a low-priority node (no edges, no access)
	kg.AddNode("temp", "Temporary", nil)
	// Add a protected node
	kg.AddNode("important", "Important", map[string]string{"protected": "true"})
	// Add a connected node (has degree)
	kg.AddEdge("hub", "spoke1", "connects", nil)
	kg.AddEdge("hub", "spoke2", "connects", nil)

	removed, err := kg.OptimizeGraph(3)
	if err != nil {
		t.Fatal(err)
	}

	if removed < 1 {
		t.Errorf("expected at least 1 removed node, got %d", removed)
	}

	// Protected node must survive
	nodes, _, _ := kg.Stats()
	if nodes == 0 {
		t.Error("all nodes removed — protected node should have survived")
	}
}

func TestKGOptimizeGraphProtectsConfiguredSources(t *testing.T) {
	kg := newTestKG(t)
	kg.SetProtectOptimizeSources([]string{"planner"})
	kg.SetProtectIDPrefixes([]string{"core_fact_"})

	if err := kg.AddNode("todo_1", "Probe Todo", map[string]string{"type": "task", "source": "planner"}); err != nil {
		t.Fatalf("AddNode planner: %v", err)
	}
	if err := kg.AddNode("core_fact_42", "Core Fact", map[string]string{"type": "concept", "source": "auto_extraction"}); err != nil {
		t.Fatalf("AddNode core_fact: %v", err)
	}
	if err := kg.AddNode("temp_low", "Temporary", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode temp: %v", err)
	}

	removed, err := kg.OptimizeGraph(1)
	if err != nil {
		t.Fatalf("OptimizeGraph: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (only temp_low)", removed)
	}
	for _, id := range []string{"todo_1", "core_fact_42"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("expected protected node %s to survive optimize", id)
		}
	}
}

func TestKGOptimizeGraphPreservesConnectedLowAccessNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("low", "Low Priority", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode low: %v", err)
	}
	for i := 0; i < 5; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		if err := kg.AddNode(peerID, peerID, nil); err != nil {
			t.Fatalf("AddNode %s: %v", peerID, err)
		}
		if err := kg.AddEdge("low", peerID, "links", nil); err != nil {
			t.Fatalf("AddEdge low->%s: %v", peerID, err)
		}
	}

	_, err := kg.OptimizeGraph(10)
	if err != nil {
		t.Fatalf("OptimizeGraph: %v", err)
	}
	if node, err := kg.GetNode("low"); err != nil || node == nil {
		t.Fatal("expected connected low-priority node to survive transactional optimize")
	}
}

func TestKGMergeNodesScopedDedupPreservesUnrelatedEdges(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("alpha", "beta", "links", nil); err != nil {
		t.Fatalf("AddEdge alpha->beta: %v", err)
	}
	if err := kg.AddEdge("target", "shared", "uses", nil); err != nil {
		t.Fatalf("AddEdge target->shared: %v", err)
	}
	if err := kg.AddEdge("source", "shared", "uses", nil); err != nil {
		t.Fatalf("AddEdge source->shared: %v", err)
	}

	if err := kg.MergeNodes("target", "source"); err != nil {
		t.Fatalf("MergeNodes: %v", err)
	}

	var unrelated int
	if err := kg.db.QueryRow(
		`SELECT COUNT(*) FROM kg_edges WHERE source = ? AND target = ? AND relation = ?`,
		"alpha", "beta", "links",
	).Scan(&unrelated); err != nil {
		t.Fatalf("count unrelated edges: %v", err)
	}
	if unrelated != 1 {
		t.Fatalf("scoped dedup touched unrelated edges: got %d rows, want 1", unrelated)
	}

	var merged int
	if err := kg.db.QueryRow(
		`SELECT COUNT(*) FROM kg_edges WHERE source = ? AND target = ? AND relation = ?`,
		"target", "shared", "uses",
	).Scan(&merged); err != nil {
		t.Fatalf("count merged edges: %v", err)
	}
	if merged != 1 {
		t.Fatalf("expected merged duplicate edges to collapse to 1 row, got %d", merged)
	}
}

func TestKGSetMinSemanticSimilarityConcurrent(t *testing.T) {
	kg := newTestKG(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			kg.SetMinSemanticSimilarity(v)
			_ = kg.getMinSemanticSimilarity()
		}(float64(i) / 100)
	}
	wg.Wait()
	if sim := kg.getMinSemanticSimilarity(); sim < 0 || sim > 1 {
		t.Fatalf("invalid min semantic similarity after concurrent updates: %f", sim)
	}
}

func TestKGOptimizeGraphBatchDeletesMultipleNodes(t *testing.T) {
	kg := newTestKG(t)

	for _, id := range []string{"temp_a", "temp_b", "temp_c"} {
		if err := kg.AddNode(id, id, map[string]string{"type": "concept"}); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
	}
	if err := kg.AddNode("keeper", "Keeper", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode keeper: %v", err)
	}

	removed, err := kg.OptimizeGraph(1)
	if err != nil {
		t.Fatalf("OptimizeGraph: %v", err)
	}
	if removed != 3 {
		t.Fatalf("removed = %d, want 3 low-priority nodes in one batch", removed)
	}
	keeper, err := kg.GetNode("keeper")
	if err != nil {
		t.Fatalf("GetNode keeper: %v", err)
	}
	if keeper == nil {
		t.Fatal("expected protected keeper node to survive batch optimize")
	}
}

func TestKGSuggestRelationsIgnoresUnqualifiedNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("noise_a", "Noise A", map[string]string{"type": "device", "source": "auto_extraction"}); err != nil {
		t.Fatalf("AddNode noise_a: %v", err)
	}
	if err := kg.AddNode("noise_b", "Noise B", map[string]string{"type": "device", "source": "auto_extraction"}); err != nil {
		t.Fatalf("AddNode noise_b: %v", err)
	}
	if result := kg.SuggestRelations(10); result != "[]" {
		t.Fatalf("unqualified suggestions = %q, want []", result)
	}

	if err := kg.AddNode("manual_a", "Manual A", map[string]string{"type": "device", "source": "manual"}); err != nil {
		t.Fatalf("AddNode manual_a: %v", err)
	}
	if err := kg.AddNode("manual_b", "Manual B", map[string]string{"type": "device", "source": "manual"}); err != nil {
		t.Fatalf("AddNode manual_b: %v", err)
	}
	result := kg.SuggestRelations(10)
	var suggestions []map[string]string
	if err := json.Unmarshal([]byte(result), &suggestions); err != nil {
		t.Fatalf("unmarshal suggestions: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("qualified suggestions = %d, want 1 same_type pair", len(suggestions))
	}
	if suggestions[0]["reason"] != "same_type" {
		t.Fatalf("reason = %q, want same_type", suggestions[0]["reason"])
	}
}

func TestKGHealthReportWithoutSemanticIndex(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("nas", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	report, err := kg.HealthReport()
	if err != nil {
		t.Fatalf("HealthReport: %v", err)
	}
	if report.SemanticEnabled {
		t.Fatal("expected semantic_enabled=false without semantic index")
	}
	if report.DirtyNodes != 1 {
		t.Fatalf("DirtyNodes = %d, want 1", report.DirtyNodes)
	}
	if report.TotalNodes != 1 {
		t.Fatalf("TotalNodes = %d, want 1", report.TotalNodes)
	}
	if report.Consistency != nil {
		t.Fatal("expected no consistency sub-report when semantic search is disabled")
	}
}

func TestKGGetNeighbors(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("center", "Center Node", nil)
	kg.AddEdge("center", "n1", "rel1", nil)
	kg.AddEdge("center", "n2", "rel2", nil)
	kg.AddEdge("n3", "center", "rel3", nil)

	nodes, edges := kg.GetNeighbors("center", 10)
	if len(edges) != 3 {
		t.Errorf("expected 3 edges for center, got %d", len(edges))
	}
	if len(nodes) != 3 { // n1, n2, n3
		t.Errorf("expected 3 neighbor nodes, got %d", len(nodes))
	}
}

func TestKGGetNeighborsLimitsNeighborNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("center", "Center", nil); err != nil {
		t.Fatalf("AddNode center: %v", err)
	}
	for _, id := range []string{"n1", "n2", "n3", "n4"} {
		if err := kg.AddNode(id, id, nil); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
	}
	if err := kg.AddEdge("center", "n1", "rel_a", nil); err != nil {
		t.Fatalf("AddEdge n1: %v", err)
	}
	if err := kg.AddEdge("center", "n2", "rel_b", nil); err != nil {
		t.Fatalf("AddEdge n2: %v", err)
	}
	if err := kg.AddEdge("center", "n3", "rel_c", nil); err != nil {
		t.Fatalf("AddEdge n3: %v", err)
	}
	if err := kg.AddEdge("center", "n4", "rel_d", nil); err != nil {
		t.Fatalf("AddEdge n4: %v", err)
	}
	if err := kg.AddEdge("center", "n4", "rel_e", nil); err != nil {
		t.Fatalf("AddEdge n4 second relation: %v", err)
	}
	for _, spec := range []struct {
		relation string
		offset   string
	}{
		{"rel_a", "-40 minutes"},
		{"rel_b", "-30 minutes"},
		{"rel_c", "-20 minutes"},
		{"rel_d", "-10 minutes"},
		{"rel_e", "-1 minutes"},
	} {
		if _, err := kg.db.Exec(`UPDATE kg_edges SET updated_at = datetime('now', ?) WHERE source = 'center' AND relation = ?`, spec.offset, spec.relation); err != nil {
			t.Fatalf("update edge timestamp %s: %v", spec.relation, err)
		}
	}

	nodes, edges := kg.GetNeighbors("center", 2)
	if len(nodes) != 2 {
		t.Fatalf("neighbor nodes = %d, want 2", len(nodes))
	}
	if len(edges) != 3 {
		t.Fatalf("edges = %d, want 3 (two relations to n4 plus one other neighbor)", len(edges))
	}
	gotIDs := []string{nodes[0].ID, nodes[1].ID}
	if gotIDs[0] != "n4" || gotIDs[1] != "n3" {
		t.Fatalf("neighbor order = %v, want [n4 n3] by most recent edge", gotIDs)
	}
}

func TestKGSearchDeduplicatesMatchedNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("server_prod", "Production Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	result := kg.Search("server")
	var payload struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal search result: %v", err)
	}
	count := 0
	for _, node := range payload.Nodes {
		if node.ID == "server_prod" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("server_prod occurrences = %d, want 1", count)
	}
}

func TestKGExploreFTSEscapesLikeMetacharacters(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("cpu_100_percent", "CPU at 100%", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode cpu: %v", err)
	}
	if err := kg.AddNode("other_host", "Other Host", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode other: %v", err)
	}

	tx, err := kg.beginReadTx("TestKGExploreFTSEscapesLikeMetacharacters")
	if err != nil {
		t.Fatalf("beginReadTx: %v", err)
	}
	defer tx.Rollback()

	nodes, err := kg.exploreFTS(tx, "100%", 10)
	if err != nil {
		t.Fatalf("exploreFTS: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("matched nodes = %d, want 1 for literal percent query", len(nodes))
	}
	if nodes[0].ID != "cpu_100_percent" {
		t.Fatalf("matched node = %q, want cpu_100_percent", nodes[0].ID)
	}
}

func TestKGGetImportantEdgesOrdersByAccessCount(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("hot_a", "Hot A", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode hot_a: %v", err)
	}
	if err := kg.AddNode("hot_b", "Hot B", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode hot_b: %v", err)
	}
	if err := kg.AddNode("cold_a", "Cold A", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode cold_a: %v", err)
	}
	if err := kg.AddNode("cold_b", "Cold B", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode cold_b: %v", err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET access_count = 50 WHERE id IN ('hot_a', 'hot_b')`); err != nil {
		t.Fatalf("update hot access counts: %v", err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET access_count = 1 WHERE id IN ('cold_a', 'cold_b')`); err != nil {
		t.Fatalf("update cold access counts: %v", err)
	}
	if err := kg.AddEdge("hot_a", "hot_b", "connects_to", nil); err != nil {
		t.Fatalf("AddEdge hot: %v", err)
	}
	if err := kg.AddEdge("cold_a", "cold_b", "connects_to", nil); err != nil {
		t.Fatalf("AddEdge cold: %v", err)
	}

	edges, err := kg.GetImportantEdges(1, nil)
	if err != nil {
		t.Fatalf("GetImportantEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	got := edges[0]
	if got.Source != "hot_a" || got.Target != "hot_b" {
		t.Fatalf("edge = %#v, want hot_a->hot_b ordered by highest endpoint access", got)
	}
}

func TestKGSearchForContext(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("proxmox", "Proxmox Server", map[string]string{"type": "service", "ip": "192.168.1.100"})
	kg.AddEdge("proxmox", "vm1", "hosts", nil)
	kg.AddEdge("proxmox", "vm2", "hosts", nil)

	ctx := kg.SearchForContext("proxmox", 5, 800)
	if ctx == "" {
		t.Error("expected non-empty context for 'proxmox'")
	}
	if len(ctx) > 800 {
		t.Errorf("context exceeds maxChars: %d", len(ctx))
	}
}

func TestKGSearchForContextBatchesEdgesAcrossMultipleNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("proxmox", "Proxmox", map[string]string{"type": "service"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("nas", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("proxmox", "backup_server", "replicates_to", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("backup_server", "nas", "stores_on", nil); err != nil {
		t.Fatal(err)
	}

	ctx := kg.SearchForContext("backup", 3, 1000)
	for _, needle := range []string{"backup_server", "replicates_to", "stores_on"} {
		if !strings.Contains(ctx, needle) {
			t.Fatalf("expected context to contain %q, got %q", needle, ctx)
		}
	}
}

func TestKGBulkAddEntities(t *testing.T) {
	kg := newTestKG(t)

	nodes := []Node{
		{ID: "user1", Label: "User One", Properties: map[string]string{"type": "person"}},
		{ID: "user2", Label: "User Two", Properties: map[string]string{"type": "person"}},
	}
	edges := []Edge{
		{Source: "user1", Target: "user2", Relation: "knows"},
	}

	if err := kg.BulkAddEntities(nodes, edges); err != nil {
		t.Fatal(err)
	}

	n, e, _ := kg.Stats()
	if n != 2 {
		t.Errorf("expected 2 nodes, got %d", n)
	}
	if e != 1 {
		t.Errorf("expected 1 edge, got %d", e)
	}
}

func TestKGBulkAddEntitiesMergesExistingNodeProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("nas", "NAS", map[string]string{
		"type":   "device",
		"notes":  "manual notes",
		"source": "manual",
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	err := kg.BulkAddEntities(
		[]Node{{
			ID:    "nas",
			Label: "Network Storage",
			Properties: map[string]string{
				"notes":  "synced notes",
				"vendor": "synology",
				"source": "inventory",
			},
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("BulkAddEntities: %v", err)
	}

	node, err := kg.GetNode("nas")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("expected merged node")
	}
	if node.Label != "NAS" {
		t.Fatalf("label = %q, want existing curated label NAS", node.Label)
	}
	if node.Properties["notes"] != "synced notes" {
		t.Fatalf("notes = %q, want overwrite merge from bulk add", node.Properties["notes"])
	}
	if node.Properties["vendor"] != "synology" {
		t.Fatalf("vendor = %q, want synology", node.Properties["vendor"])
	}
	if node.Properties["type"] != "device" {
		t.Fatalf("type = %q, want preserved device type", node.Properties["type"])
	}
}

func TestKGAddEdgeCreatesPlaceholderNodeProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("server", "Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.AddEdge("server", "ghost_peer", "connects_to", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	ghost, err := kg.GetNode("ghost_peer")
	if err != nil {
		t.Fatalf("GetNode ghost_peer: %v", err)
	}
	if ghost == nil {
		t.Fatal("expected auto-created placeholder node")
	}
	if ghost.Label != knowledgeGraphPlaceholderLabel {
		t.Fatalf("label = %q, want %q", ghost.Label, knowledgeGraphPlaceholderLabel)
	}
	if ghost.Properties["source"] != knowledgeGraphPlaceholderSource {
		t.Fatalf("source = %q, want %q", ghost.Properties["source"], knowledgeGraphPlaceholderSource)
	}
	if ghost.Properties["type"] != "unknown" {
		t.Fatalf("type = %q, want unknown", ghost.Properties["type"])
	}
}

func TestKGCleanupStaleGraphRemovesStalePlaceholders(t *testing.T) {
	kg := newTestKG(t)

	propsJSON, err := json.Marshal(knowledgeGraphPlaceholderNodeProperties())
	if err != nil {
		t.Fatalf("marshal placeholder props: %v", err)
	}
	for _, spec := range []struct {
		id      string
		updated string
	}{
		{"stale_placeholder", "-8 days"},
		{"fresh_placeholder", "-1 days"},
	} {
		if _, err := kg.db.Exec(`
			INSERT INTO kg_nodes (id, label, properties, updated_at)
			VALUES (?, ?, ?, datetime('now', ?))
		`, spec.id, knowledgeGraphPlaceholderLabel, string(propsJSON), spec.updated); err != nil {
			t.Fatalf("insert placeholder %s: %v", spec.id, err)
		}
	}
	if _, err := kg.db.Exec(`
		INSERT INTO kg_nodes (id, label, properties, updated_at)
		VALUES ('linked_placeholder', ?, ?, datetime('now', '-8 days'))
	`, knowledgeGraphPlaceholderLabel, string(propsJSON)); err != nil {
		t.Fatalf("insert linked placeholder: %v", err)
	}
	if err := kg.AddEdge("linked_placeholder", "anchor", "mentions", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	_, nodesRemoved, err := kg.CleanupStaleGraph(30)
	if err != nil {
		t.Fatalf("CleanupStaleGraph: %v", err)
	}
	if nodesRemoved != 1 {
		t.Fatalf("nodesRemoved = %d, want 1 (only stale isolated placeholder)", nodesRemoved)
	}

	stale, err := kg.GetNode("stale_placeholder")
	if err != nil {
		t.Fatalf("GetNode stale_placeholder: %v", err)
	}
	if stale != nil {
		t.Fatal("expected stale isolated placeholder to be removed")
	}
	for _, id := range []string{"fresh_placeholder", "linked_placeholder", "anchor"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("expected node %s to survive cleanup", id)
		}
	}
}

func TestKGBulkMergeExtractedEntitiesPreservesExistingProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("proxmox", "Proxmox", map[string]string{
		"type":   "service",
		"notes":  "manually curated",
		"source": "manual",
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.AddEdge("proxmox", "backup_server", "replicates_to", map[string]string{
		"notes":  "hand-written edge note",
		"source": "manual",
	}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	err := kg.BulkMergeExtractedEntities(
		[]Node{
			{
				ID:    "proxmox",
				Label: "Proxmox VE Host",
				Properties: map[string]string{
					"type":         "platform",
					"ip":           "192.168.1.50",
					"notes":        "llm-generated note",
					"source":       "auto_extraction",
					"extracted_at": "2026-04-02",
				},
			},
			{
				ID:    "proxmox",
				Label: "Proxmox VE",
				Properties: map[string]string{
					"vendor": "proxmox",
				},
			},
		},
		[]Edge{
			{
				Source:   "proxmox",
				Target:   "backup_server",
				Relation: "replicates_to",
				Properties: map[string]string{
					"notes":        "llm edge note",
					"schedule":     "nightly",
					"source":       "auto_extraction",
					"extracted_at": "2026-04-02",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("BulkMergeExtractedEntities: %v", err)
	}

	nodes, err := kg.GetAllNodes(10)
	if err != nil {
		t.Fatalf("GetAllNodes: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected merged node")
	}
	var proxmox Node
	for _, node := range nodes {
		if node.ID == "proxmox" {
			proxmox = node
			break
		}
	}
	if proxmox.Label != "Proxmox" {
		t.Fatalf("expected existing curated label to be preserved, got %q", proxmox.Label)
	}
	if proxmox.Properties["notes"] != "manually curated" {
		t.Fatalf("expected curated node note to be preserved, got %q", proxmox.Properties["notes"])
	}
	if proxmox.Properties["ip"] != "192.168.1.50" || proxmox.Properties["vendor"] != "proxmox" {
		t.Fatalf("expected new extracted properties to be merged, got %#v", proxmox.Properties)
	}
	if proxmox.Properties["source"] != "manual" {
		t.Fatalf("expected existing node source to win, got %q", proxmox.Properties["source"])
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("expected merged edge")
	}
	edge := edges[0]
	if edge.Properties["notes"] != "hand-written edge note" {
		t.Fatalf("expected curated edge note to be preserved, got %q", edge.Properties["notes"])
	}
	if edge.Properties["schedule"] != "nightly" {
		t.Fatalf("expected new edge property to be merged, got %#v", edge.Properties)
	}
	if edge.Properties["source"] != "manual" {
		t.Fatalf("expected existing edge source to win, got %q", edge.Properties["source"])
	}
}

func TestKGBulkMergeExtractedEntitiesPrefersHigherConfidenceAutoProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.BulkMergeExtractedEntities([]Node{{
		ID:    "nas",
		Label: "NAS",
		Properties: map[string]string{
			"type":       "device",
			"notes":      "long but weak extracted description",
			"source":     "file_sync",
			"confidence": "0.35",
		},
	}}, nil); err != nil {
		t.Fatalf("seed BulkMergeExtractedEntities: %v", err)
	}

	if err := kg.BulkMergeExtractedEntities([]Node{{
		ID:    "nas",
		Label: "NAS Storage",
		Properties: map[string]string{
			"type":       "device",
			"notes":      "verified storage host",
			"source":     "file_sync",
			"confidence": "0.90",
		},
	}}, nil); err != nil {
		t.Fatalf("update BulkMergeExtractedEntities: %v", err)
	}

	node, err := kg.GetNode("nas")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Properties["notes"] != "verified storage host" {
		t.Fatalf("notes = %q, want higher confidence incoming value", node.Properties["notes"])
	}
}

func TestKGUpdateNodeMergesPartialProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("server1", "Server", map[string]string{
		"type":  "device",
		"ip":    "10.0.0.1",
		"notes": "initial",
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	node, err := kg.UpdateNode("server1", "", map[string]string{"notes": "updated"})
	if err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	if node == nil {
		t.Fatal("expected updated node")
	}
	if node.Properties["ip"] != "10.0.0.1" {
		t.Fatalf("ip = %q, want preserved 10.0.0.1", node.Properties["ip"])
	}
	if node.Properties["notes"] != "updated" {
		t.Fatalf("notes = %q, want updated", node.Properties["notes"])
	}
}

func TestKGUpdateNodePreservesProtectionAndProperties(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("backup_server", "Backup Server", map[string]string{
		"type":      "device",
		"protected": "true",
		"notes":     "initial",
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	node, err := kg.UpdateNode("backup_server", "Primary Backup Server", map[string]string{
		"type":  "device",
		"notes": "updated",
	})
	if err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	if node == nil {
		t.Fatal("expected updated node")
	}
	if !node.Protected {
		t.Fatal("expected protected flag to survive node update")
	}
	if node.Label != "Primary Backup Server" {
		t.Fatalf("label = %q, want updated label", node.Label)
	}
	if node.Properties["notes"] != "updated" {
		t.Fatalf("notes = %q, want updated", node.Properties["notes"])
	}
	if node.Properties["protected"] != "true" {
		t.Fatalf("protected property = %q, want true", node.Properties["protected"])
	}
}

func TestKGNodeUpsert(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("alice", "Alice v1", map[string]string{"role": "tester"})
	kg.AddNode("alice", "Alice v2", map[string]string{"role": "developer"})

	nodes, _, _ := kg.Stats()
	if nodes != 1 {
		t.Errorf("expected 1 node after upsert, got %d", nodes)
	}
}

func TestKGGetAllNodesEdges(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("a", "Node A", nil)
	kg.AddNode("b", "Node B", nil)
	kg.AddEdge("a", "b", "connects", nil)

	nodes, err := kg.GetAllNodes(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	edges, err := kg.GetAllEdges(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}

func TestKGMigrateFromJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a temp JSON file simulating old graph.json
	tmpJSON, err := os.CreateTemp("", "graph_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	jsonPath := tmpJSON.Name()
	defer os.Remove(jsonPath)
	defer os.Remove(jsonPath + ".migrated")

	jsonContent := `{
		"nodes": {
			"test_node": {"id": "test_node", "label": "Test Node", "properties": {"role": "tester"}},
			"other_node": {"id": "other_node", "label": "Other", "properties": {}}
		},
		"edges": [
			{"source": "test_node", "target": "other_node", "relation": "connects_to", "properties": {}}
		]
	}`
	tmpJSON.WriteString(jsonContent)
	tmpJSON.Close()

	kg, err := NewKnowledgeGraph(":memory:", jsonPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()

	nodes, edges, _ := kg.Stats()
	if nodes != 2 {
		t.Errorf("expected 2 migrated nodes, got %d", nodes)
	}
	if edges != 1 {
		t.Errorf("expected 1 migrated edge, got %d", edges)
	}

	// Original file should be renamed
	if _, err := os.Stat(jsonPath + ".migrated"); os.IsNotExist(err) {
		t.Error("expected .migrated file to exist after migration")
	}
}

// TestKGSearchUnionFindsLIKEOnlyNode verifies that the UNION query finds nodes that
// would only be matched by LIKE (not FTS5), e.g. numeric IDs.
func TestKGSearchUnionFindsLIKEOnlyNode(t *testing.T) {
	kg := newTestKG(t)

	// Add a node whose label will only be found via LIKE (pure number, not indexed by FTS5).
	if err := kg.AddNode("node-42", "42", map[string]string{"kind": "number"}); err != nil {
		t.Fatal(err)
	}
	// Add a normal textual node
	if err := kg.AddNode("alice", "Alice Smith", map[string]string{}); err != nil {
		t.Fatal(err)
	}

	result := kg.Search("Alice")
	if result == "[]" {
		t.Error("Search('Alice') returned empty result — expected node found via FTS5 or LIKE")
	}
}

// TestKGSearchAccessCountWorker verifies access counts are updated via the worker pool
// (no goroutine explosion: only one background worker is running).
func TestKGSearchAccessCountWorker(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("cat", "Cat", map[string]string{}); err != nil {
		t.Fatal(err)
	}

	// Perform multiple searches to push IDs into the access queue
	for i := 0; i < 5; i++ {
		kg.Search("Cat")
	}

	// Wait for the worker goroutine to drain the queue (poll up to 500ms).
	var accessCount int
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		_ = kg.db.QueryRow("SELECT access_count FROM kg_nodes WHERE id = 'cat'").Scan(&accessCount)
		if accessCount > 0 {
			break
		}
	}
	if accessCount == 0 {
		t.Error("access_count was not updated by worker pool after multiple searches")
	}
}

func TestKGPropertyValuesPreservedBeyondLegacyLimit(t *testing.T) {
	kg := newTestKG(t)

	longValue := strings.Repeat("x", 120)
	if err := kg.AddNode("device_1", "Device 1", map[string]string{"notes": longValue}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.AddEdge("device_1", "device_2", "connected_to", map[string]string{"notes": longValue}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	nodes, err := kg.GetAllNodes(10)
	if err != nil {
		t.Fatalf("GetAllNodes: %v", err)
	}
	var device1 *Node
	for i := range nodes {
		if nodes[i].ID == "device_1" {
			device1 = &nodes[i]
			break
		}
	}
	if device1 == nil {
		t.Fatalf("device_1 node not found in %d nodes", len(nodes))
	}
	if got := device1.Properties["notes"]; got != longValue {
		t.Fatalf("node property was unexpectedly truncated: got len %d want len %d", len(got), len(longValue))
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if got := edges[0].Properties["notes"]; got != longValue {
		t.Fatalf("edge property was unexpectedly truncated: got len %d want len %d", len(got), len(longValue))
	}
}

func TestKGSearchMatchesEdgePropertiesAndUpdatesEdgeAccessCount(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("proxmox", "backup_server", "replicates_to", map[string]string{
		"notes": "nightly replication target in rack-b",
	}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	result := kg.Search("rack-b")
	if result == "[]" {
		t.Fatal("expected edge property search result, got empty")
	}

	var payload struct {
		Edges []Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal search result: %v", err)
	}
	if len(payload.Edges) == 0 {
		t.Fatal("expected at least one edge in search result")
	}

	var accessCount int
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		_ = kg.db.QueryRow(
			"SELECT access_count FROM kg_edges WHERE source = ? AND target = ? AND relation = ?",
			"proxmox", "backup_server", "replicates_to",
		).Scan(&accessCount)
		if accessCount > 0 {
			break
		}
	}
	if accessCount == 0 {
		t.Fatal("edge access_count was not updated after search")
	}
}

func TestKGSearchForContextUsesSemanticIndex(t *testing.T) {
	kg := newTestKG(t)

	db := chromem.NewDB()
	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "virtualization"), strings.Contains(lower, "hypervisor"):
			return []float32{1, 0}, nil
		default:
			return []float32{0, 1}, nil
		}
	}
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("proxmox", "Virtualization Host", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	ctx := kg.SearchForContext("hypervisor", 3, 800)
	if !strings.Contains(ctx, "proxmox") {
		t.Fatalf("expected semantic KG search to include proxmox, got %q", ctx)
	}
}

func TestKGSearchForContextPrefersExactTextMatchOverSemanticNoise(t *testing.T) {
	kg := newTestKG(t)

	db := chromem.NewDB()
	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		lower := strings.ToLower(strings.TrimSpace(text))
		switch {
		case lower == "rosemarie":
			return []float32{1, 0}, nil
		case strings.Contains(lower, "assistant"):
			return []float32{1, 0}, nil
		default:
			return []float32{0, 1}, nil
		}
	}
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("assistant", "assistant", map[string]string{"type": "activity_entity"}); err != nil {
		t.Fatalf("AddNode assistant: %v", err)
	}
	if err := kg.AddNode("rosemarie_west", "Rosemarie West", map[string]string{
		"type":       "person",
		"collection": "file_index",
		"source":     "file_sync",
	}); err != nil {
		t.Fatalf("AddNode Rosemarie: %v", err)
	}

	ctx := kg.SearchForContext("Rosemarie", 1, 800)
	if !strings.Contains(ctx, "rosemarie_west") {
		t.Fatalf("expected exact KG text match to win over semantic noise, got %q", ctx)
	}
	if strings.Contains(ctx, "assistant") {
		t.Fatalf("expected semantic noise to be excluded when limit is tight, got %q", ctx)
	}
}

func TestKGSemanticQuerySkipsShortInputs(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{query: "", want: true},
		{query: "*", want: true},
		{query: "hi", want: true},
		{query: "S3", want: false},
		{query: "NAS", want: false},
		{query: "status?", want: true},
		{query: "tailscale", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := kgsemantic.ShouldSkipQuery(tt.query)
			if got != tt.want {
				t.Fatalf("ShouldSkipQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// TestKGCoOccurrenceThreshold verifies that co_mentioned_with edges are promoted
// from "pending" to "activity_turn" only once the coOccurrenceThreshold is reached.
func TestKGCoOccurrenceThreshold(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("alice", "Alice", nil)
	kg.AddNode("bob", "Bob", nil)

	// Below threshold: edge should exist but remain pending.
	for i := 0; i < coOccurrenceThreshold-1; i++ {
		if err := kg.IncrementCoOccurrence("alice", "bob", "2026-01-01"); err != nil {
			t.Fatalf("IncrementCoOccurrence (step %d): %v", i+1, err)
		}
	}
	var propsJSON string
	err := kg.db.QueryRow(
		"SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
		"alice", "bob",
	).Scan(&propsJSON)
	if err != nil {
		t.Fatalf("edge should exist after %d co-mentions: %v", coOccurrenceThreshold-1, err)
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		t.Fatalf("unmarshal props: %v", err)
	}
	if props["source"] != "pending" {
		t.Errorf("expected source='pending' below threshold, got %q", props["source"])
	}

	// At threshold: edge should be promoted to activity_turn.
	if err := kg.IncrementCoOccurrence("alice", "bob", "2026-01-02"); err != nil {
		t.Fatalf("IncrementCoOccurrence (threshold step): %v", err)
	}
	err = kg.db.QueryRow(
		"SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
		"alice", "bob",
	).Scan(&propsJSON)
	if err != nil {
		t.Fatalf("edge should exist at threshold: %v", err)
	}
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		t.Fatalf("unmarshal props: %v", err)
	}
	if props["source"] != "activity_turn" {
		t.Errorf("expected source='activity_turn' at threshold, got %q", props["source"])
	}
	if props["weight"] != strconv.Itoa(coOccurrenceThreshold) {
		t.Errorf("expected weight='%d' at threshold, got %q", coOccurrenceThreshold, props["weight"])
	}
}

func TestKGIncrementCoOccurrenceUpsertIncrementsExisting(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("alice", "Alice", nil); err != nil {
		t.Fatalf("AddNode alice: %v", err)
	}
	if err := kg.AddNode("bob", "Bob", nil); err != nil {
		t.Fatalf("AddNode bob: %v", err)
	}

	if err := kg.IncrementCoOccurrence("alice", "bob", "2026-06-20"); err != nil {
		t.Fatalf("first IncrementCoOccurrence: %v", err)
	}
	if err := kg.IncrementCoOccurrence("alice", "bob", "2026-06-21"); err != nil {
		t.Fatalf("second IncrementCoOccurrence: %v", err)
	}

	var propsJSON string
	if err := kg.db.QueryRow(
		"SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
		"alice", "bob",
	).Scan(&propsJSON); err != nil {
		t.Fatalf("query edge: %v", err)
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		t.Fatalf("unmarshal props: %v", err)
	}
	if props["weight"] != "2" {
		t.Fatalf("weight = %q, want 2 after upsert increment", props["weight"])
	}
	if props["date"] != "2026-06-21" {
		t.Fatalf("date = %q, want latest upsert date", props["date"])
	}
}

func TestKGAddNodeAppliesSchemaDefaults(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("router", "Router", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	node, err := kg.GetNode("router")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	for _, key := range []string{"ip", "mac", "os"} {
		if _, ok := node.Properties[key]; !ok {
			t.Fatalf("expected schema default key %q on device node", key)
		}
	}
}

func TestKGBulkAddEntitiesSetsUpdatedAt(t *testing.T) {
	kg := newTestKG(t)

	before := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	if err := kg.BulkAddEntities([]Node{{
		ID:    "bulk_node",
		Label: "Bulk Node",
		Properties: map[string]string{
			"type": "concept",
		},
	}}, nil); err != nil {
		t.Fatalf("BulkAddEntities: %v", err)
	}

	var updatedAt string
	if err := kg.db.QueryRow("SELECT updated_at FROM kg_nodes WHERE id = ?", "bulk_node").Scan(&updatedAt); err != nil {
		t.Fatalf("query updated_at: %v", err)
	}
	if strings.TrimSpace(updatedAt) == "" {
		t.Fatal("expected non-empty updated_at after bulk add")
	}
	if updatedAt < before {
		t.Fatalf("updated_at %q should be newer than %q", updatedAt, before)
	}
}

// TestKGCoOccurrencePropertiesJSON verifies that the date value is properly JSON-encoded
// and not injected via string concatenation.
func TestKGCoOccurrencePropertiesJSON(t *testing.T) {
	kg := newTestKG(t)
	kg.AddNode("a", "A", nil)
	kg.AddNode("b", "B", nil)

	if err := kg.IncrementCoOccurrence("a", "b", "2026-01-01"); err != nil {
		t.Fatalf("IncrementCoOccurrence: %v", err)
	}
	var propsJSON string
	if err := kg.db.QueryRow(
		"SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
		"a", "b",
	).Scan(&propsJSON); err != nil {
		t.Fatalf("query edge: %v", err)
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		t.Errorf("properties should be valid JSON, got %q: %v", propsJSON, err)
	}
	if props["date"] != "2026-01-01" {
		t.Errorf("expected date='2026-01-01', got %q", props["date"])
	}
}

// TestKGGetSubgraphBFS verifies that GetSubgraph correctly performs BFS up to maxDepth
// and returns the expected set of nodes and edges.
func TestKGGetSubgraphBFS(t *testing.T) {
	kg := newTestKG(t)

	// Build chain: A → B → C → D
	for _, id := range []string{"a", "b", "c", "d"} {
		if err := kg.AddNode(id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	kg.AddEdge("a", "b", "rel", nil)
	kg.AddEdge("b", "c", "rel", nil)
	kg.AddEdge("c", "d", "rel", nil)

	// maxDepth=2 from A: should reach B (depth 1) and C (depth 2), not D (depth 3).
	nodes, edges := kg.GetSubgraph("a", 2)
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	if !nodeIDs["a"] {
		t.Error("expected center node 'a' in subgraph")
	}
	if !nodeIDs["b"] {
		t.Error("expected depth-1 node 'b' in subgraph")
	}
	if !nodeIDs["c"] {
		t.Error("expected depth-2 node 'c' in subgraph")
	}
	if nodeIDs["d"] {
		t.Error("node 'd' at depth 3 should not be in subgraph with maxDepth=2")
	}
	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges, got %d", len(edges))
	}
}

func TestKGGetSubgraphBranchingBFSDoesNotLeakPastMaxDepth(t *testing.T) {
	kg := newTestKG(t)

	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := kg.AddNode(id, id, nil); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
	}
	if err := kg.AddEdge("a", "b", "rel", nil); err != nil {
		t.Fatalf("AddEdge a-b: %v", err)
	}
	if err := kg.AddEdge("a", "c", "rel", nil); err != nil {
		t.Fatalf("AddEdge a-c: %v", err)
	}
	if err := kg.AddEdge("b", "d", "rel", nil); err != nil {
		t.Fatalf("AddEdge b-d: %v", err)
	}
	if err := kg.AddEdge("c", "d", "rel", nil); err != nil {
		t.Fatalf("AddEdge c-d: %v", err)
	}
	if err := kg.AddEdge("d", "e", "rel", nil); err != nil {
		t.Fatalf("AddEdge d-e: %v", err)
	}

	nodes, edges := kg.GetSubgraph("a", 2)
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if !nodeIDs[id] {
			t.Fatalf("expected node %q in depth-2 branching subgraph; nodes=%v", id, nodeIDs)
		}
	}
	if nodeIDs["e"] {
		t.Fatalf("node e at depth 3 should not be included; nodes=%v", nodeIDs)
	}
	if len(edges) != 4 {
		t.Fatalf("edges len = %d, want the four edges within depth 2", len(edges))
	}
}

// TestKGGetSubgraphCycle verifies that GetSubgraph terminates and does not loop
// when the graph contains cycles.
func TestKGGetSubgraphCycle(t *testing.T) {
	kg := newTestKG(t)

	for _, id := range []string{"x", "y", "z"} {
		kg.AddNode(id, id, nil)
	}
	kg.AddEdge("x", "y", "rel", nil)
	kg.AddEdge("y", "z", "rel", nil)
	kg.AddEdge("z", "x", "rel", nil) // cycle

	// Must return without hanging.
	done := make(chan struct{})
	go func() {
		kg.GetSubgraph("x", 3)
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("GetSubgraph did not terminate within 5s on cyclic graph")
	}
}

// TestValidIdentifierValid verifies that valid SQLite identifiers are accepted.
func TestValidIdentifierValid(t *testing.T) {
	valid := []string{"kg_nodes", "kg_edges", "col1", "_private", "CamelCase", "with_underscores_123"}
	for _, id := range valid {
		if !validIdentifier(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
}

// TestValidIdentifierInvalid verifies that SQL injection attempts are rejected.
func TestValidIdentifierInvalid(t *testing.T) {
	invalid := []string{
		"'; DROP TABLE kg_nodes;--", // SQL injection attempt
		"kg_nodes; DROP TABLE kg",   // SQL injection via semicolon
		"kg_nodes' OR '1'='1",       // SQL injection via quotes
		"kg_nodes--comment",         // SQL comment injection
		"",                          // empty string
		"kg nodes",                  // space
		"kg.nodes",                  // dot
		"kg[nodes",                  // bracket
		"kg(nodes",                  // paren
		"kg\"nodes",                 // double quote
	}
	for _, id := range invalid {
		if validIdentifier(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}

// TestQuoteIdentifier verifies safe quoting of identifiers with embedded double-quotes.
func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"kg_nodes", `"kg_nodes"`},
		{`with"quote`, `"with""quote"`},
		{"simple", `"simple"`},
	}
	for _, tc := range tests {
		got := quoteIdentifier(tc.input)
		if got != tc.expected {
			t.Errorf("quoteIdentifier(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestKGDeleteNodesBySourceFile(t *testing.T) {
	kg := newTestKG(t)

	// Add nodes with source_file property
	if err := kg.AddNode("n1", "Node 1", map[string]string{"source": "file_sync", "source_file": "/docs/a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("n2", "Node 2", map[string]string{"source": "file_sync", "source_file": "/docs/b.txt"}); err != nil {
		t.Fatal(err)
	}
	// Protected node should not be deleted
	if err := kg.AddNode("n3", "Node 3", map[string]string{"source": "file_sync", "source_file": "/docs/a.txt", "protected": "true"}); err != nil {
		t.Fatal(err)
	}

	deleted, err := kg.DeleteNodesBySourceFile("/docs/a.txt")
	if err != nil {
		t.Fatalf("DeleteNodesBySourceFile: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted node, got %d", deleted)
	}

	n, _, _ := kg.Stats()
	if n != 2 {
		t.Errorf("expected 2 remaining nodes, got %d", n)
	}
}

func TestKGDeleteEdgesBySourceFile(t *testing.T) {
	kg := newTestKG(t)

	// Ensure nodes exist
	_ = kg.AddNode("a", "A", nil)
	_ = kg.AddNode("b", "B", nil)

	if err := kg.AddEdge("a", "b", "rel1", map[string]string{"source": "file_sync", "source_file": "/docs/a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("a", "b", "rel2", map[string]string{"source": "file_sync", "source_file": "/docs/b.txt"}); err != nil {
		t.Fatal(err)
	}

	deleted, err := kg.DeleteEdgesBySourceFile("/docs/a.txt")
	if err != nil {
		t.Fatalf("DeleteEdgesBySourceFile: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted edge, got %d", deleted)
	}

	_, edges, _ := kg.Stats()
	if edges != 1 {
		t.Errorf("expected 1 remaining edge, got %d", edges)
	}
}

func TestKGFindOrphanedFileSyncEntities(t *testing.T) {
	kg := newTestKG(t)

	_ = kg.AddNode("n1", "Node 1", map[string]string{"source": "file_sync", "source_file": "/docs/active.txt"})
	_ = kg.AddNode("n2", "Node 2", map[string]string{"source": "file_sync", "source_file": "/docs/orphan.txt"})
	_ = kg.AddNode("n3", "Node 3", map[string]string{"source": "manual"})

	_ = kg.AddNode("a", "A", nil)
	_ = kg.AddNode("b", "B", nil)
	_ = kg.AddEdge("a", "b", "rel1", map[string]string{"source": "file_sync", "source_file": "/docs/orphan.txt"})

	orphanNodes, orphanEdges, err := kg.FindOrphanedFileSyncEntities([]string{"/docs/active.txt"})
	if err != nil {
		t.Fatalf("FindOrphanedFileSyncEntities: %v", err)
	}
	if len(orphanNodes) != 1 {
		t.Errorf("expected 1 orphan node, got %d", len(orphanNodes))
	}
	if len(orphanEdges) != 1 {
		t.Errorf("expected 1 orphan edge, got %d", len(orphanEdges))
	}
	if len(orphanNodes) > 0 && orphanNodes[0].ID != "n2" {
		t.Errorf("expected orphan node n2, got %s", orphanNodes[0].ID)
	}
}

func TestKGGetNodesBySourceFile(t *testing.T) {
	kg := newTestKG(t)

	_ = kg.AddNode("n1", "Node 1", map[string]string{"source": "file_sync", "source_file": "/docs/report.md"})
	_ = kg.AddNode("n2", "Node 2", map[string]string{"source": "file_sync", "source_file": "/docs/report.md"})
	_ = kg.AddNode("n3", "Node 3", map[string]string{"source": "file_sync", "source_file": "/docs/other.md"})

	nodes, err := kg.GetNodesBySourceFile("/docs/report.md", 10)
	if err != nil {
		t.Fatalf("GetNodesBySourceFile: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.Properties["source_file"] != "/docs/report.md" {
			t.Errorf("expected source_file /docs/report.md, got %s", n.Properties["source_file"])
		}
	}
}

func TestKGGetEdgesBySourceFile(t *testing.T) {
	kg := newTestKG(t)

	_ = kg.AddNode("a", "A", nil)
	_ = kg.AddNode("b", "B", nil)
	_ = kg.AddNode("c", "C", nil)

	_ = kg.AddEdge("a", "b", "rel1", map[string]string{"source": "file_sync", "source_file": "/docs/report.md"})
	_ = kg.AddEdge("b", "c", "rel2", map[string]string{"source": "file_sync", "source_file": "/docs/report.md"})
	_ = kg.AddEdge("a", "c", "rel3", map[string]string{"source": "file_sync", "source_file": "/docs/other.md"})

	edges, err := kg.GetEdgesBySourceFile("/docs/report.md", 10)
	if err != nil {
		t.Fatalf("GetEdgesBySourceFile: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
	for _, e := range edges {
		if e.Properties["source_file"] != "/docs/report.md" {
			t.Errorf("expected source_file /docs/report.md, got %s", e.Properties["source_file"])
		}
	}
}

func TestKGGetSourceFilesByNodeID(t *testing.T) {
	kg := newTestKG(t)

	// Node with its own source_file
	_ = kg.AddNode("n1", "Node 1", map[string]string{"source": "file_sync", "source_file": "/docs/owner.md"})

	// Connected edges with different source_files
	_ = kg.AddNode("n2", "Node 2", nil)
	_ = kg.AddNode("n3", "Node 3", nil)
	_ = kg.AddEdge("n1", "n2", "rel1", map[string]string{"source": "file_sync", "source_file": "/docs/edge1.md"})
	_ = kg.AddEdge("n3", "n1", "rel2", map[string]string{"source": "file_sync", "source_file": "/docs/edge2.md"})
	// Duplicate source_file should be deduplicated
	_ = kg.AddEdge("n1", "n3", "rel3", map[string]string{"source": "file_sync", "source_file": "/docs/edge2.md"})

	files, err := kg.GetSourceFilesByNodeID("n1", 10)
	if err != nil {
		t.Fatalf("GetSourceFilesByNodeID: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 unique source files, got %d: %v", len(files), files)
	}

	expected := map[string]bool{
		"/docs/owner.md": false,
		"/docs/edge1.md": false,
		"/docs/edge2.md": false,
	}
	for _, f := range files {
		expected[f] = true
	}
	for f, found := range expected {
		if !found {
			t.Errorf("missing expected source file %s", f)
		}
	}
}

func TestKGGetSourceFilesByNodeID_Limit(t *testing.T) {
	kg := newTestKG(t)

	_ = kg.AddNode("n1", "Node 1", map[string]string{"source_file": "/docs/a.md"})
	_ = kg.AddNode("n2", "Node 2", nil)
	_ = kg.AddEdge("n1", "n2", "rel1", map[string]string{"source_file": "/docs/b.md"})

	files, err := kg.GetSourceFilesByNodeID("n1", 1)
	if err != nil {
		t.Fatalf("GetSourceFilesByNodeID: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file due to limit, got %d", len(files))
	}
}

func TestKGFileSyncTimestampsHandleMissingExtractedAt(t *testing.T) {
	kg := newTestKG(t)

	_ = kg.AddNode("file_entity", "File Entity", map[string]string{
		"source":      "file_sync",
		"collection":  "file_index",
		"source_file": "/docs/manual.pdf",
	})

	globalLastSync, err := kg.GetLastFileSyncTime("")
	if err != nil {
		t.Fatalf("GetLastFileSyncTime global returned error for missing extracted_at: %v", err)
	}
	if globalLastSync != nil {
		t.Fatalf("global last sync = %v, want nil", globalLastSync)
	}

	collectionLastSync, err := kg.GetLastFileSyncTime("file_index")
	if err != nil {
		t.Fatalf("GetLastFileSyncTime collection returned error for missing extracted_at: %v", err)
	}
	if collectionLastSync != nil {
		t.Fatalf("collection last sync = %v, want nil", collectionLastSync)
	}

	stats, err := kg.GetCollectionFileSyncStats("file_index")
	if err != nil {
		t.Fatalf("GetCollectionFileSyncStats returned error for missing extracted_at: %v", err)
	}
	if stats.LastSyncAt != nil {
		t.Fatalf("stats.LastSyncAt = %v, want nil", stats.LastSyncAt)
	}
}
