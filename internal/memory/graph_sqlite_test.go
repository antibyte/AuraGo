package memory

import (
	"log/slog"
	"os"
	"testing"
	"time"
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

	nodes, edges := kg.Stats()
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

	nodes, edges := kg.Stats()
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

	_, edges := kg.Stats()
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

func TestKGDeleteNode(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("alice", "Alice", nil)
	kg.AddEdge("alice", "bob", "knows", nil)

	if err := kg.DeleteNode("alice"); err != nil {
		t.Fatal(err)
	}

	nodes, edges := kg.Stats()
	if nodes != 1 { // bob remains
		t.Errorf("expected 1 node after delete, got %d", nodes)
	}
	if edges != 0 { // edge removed
		t.Errorf("expected 0 edges after node delete, got %d", edges)
	}
}

func TestKGDeleteEdge(t *testing.T) {
	kg := newTestKG(t)

	kg.AddEdge("a", "b", "rel1", nil)
	kg.AddEdge("a", "b", "rel2", nil)

	if err := kg.DeleteEdge("a", "b", "rel1"); err != nil {
		t.Fatal(err)
	}

	_, edges := kg.Stats()
	if edges != 1 {
		t.Errorf("expected 1 edge after delete, got %d", edges)
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
	nodes, _ := kg.Stats()
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

	n, e := kg.Stats()
	if n != 2 {
		t.Errorf("expected 2 nodes, got %d", n)
	}
	if e != 1 {
		t.Errorf("expected 1 edge, got %d", e)
	}
}

func TestKGNodeUpsert(t *testing.T) {
	kg := newTestKG(t)

	kg.AddNode("alice", "Alice v1", map[string]string{"role": "tester"})
	kg.AddNode("alice", "Alice v2", map[string]string{"role": "developer"})

	nodes, _ := kg.Stats()
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

	nodes, edges := kg.Stats()
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
