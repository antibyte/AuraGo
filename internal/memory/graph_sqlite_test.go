package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestKGAddEdgeUpsert(t *testing.T) {
	kg := newTestKG(t)

	kg.AddEdge("a", "b", "rel", map[string]string{"weight": "1"})
	kg.AddEdge("a", "b", "rel", map[string]string{"weight": "2"})

	_, edges, _ := kg.Stats()
	if edges != 1 {
		t.Errorf("expected 1 edge after upsert, got %d", edges)
	}
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

func TestKGSearchFTS5(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("raspberry_pi", "Raspberry Pi 4", map[string]string{"type": "device"})
	kg.AddNode("docker_host", "Docker Host Server", map[string]string{"type": "service"})

	result := kg.Search("raspberry")
	if result == "[]" {
		t.Error("FTS5 search for 'raspberry' should return results")
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
	if err := kg.AddNode("nas_secondary", "NAS", nil); err != nil {
		t.Fatalf("AddNode nas_secondary: %v", err)
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
	if got := nodes[0].Properties["notes"]; got != longValue {
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
