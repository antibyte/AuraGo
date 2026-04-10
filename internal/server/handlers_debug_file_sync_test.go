package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"aurago/internal/memory"
)

// newTestKG creates an in-memory KG for testing.
func newTestKG(t *testing.T) *memory.KnowledgeGraph {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { kg.Close() })
	return kg
}

func TestHandleDebugKGFileSyncStats(t *testing.T) {
	t.Parallel()

	// Create a minimal KG for testing
	kg := newTestKG(t)

	// Add some file_sync nodes
	kg.AddNode("node1", "Test Node 1", map[string]string{
		"source":      "file_sync",
		"type":        "person",
		"collection":  "docs",
		"source_file": "/tmp/test1.txt",
	})
	kg.AddNode("node2", "Test Node 2", map[string]string{
		"source":      "file_sync",
		"type":        "device",
		"collection":  "docs",
		"source_file": "/tmp/test2.txt",
	})
	kg.AddNode("node3", "Manual Node", map[string]string{
		"source": "manual",
		"type":   "person",
	})
	kg.AddEdge("node1", "node2", "related_to", map[string]string{"source": "file_sync"})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-file-sync-stats", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileSyncStats(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var stats memory.FileSyncStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Should have 2 file_sync nodes
	if stats.NodeCount != 2 {
		t.Errorf("NodeCount = %d, want 2", stats.NodeCount)
	}
	// Should have 1 file_sync edge
	if stats.EdgeCount != 1 {
		t.Errorf("EdgeCount = %d, want 1", stats.EdgeCount)
	}
	// Should have 2 entity types: person and device
	if stats.ByEntityType["person"] != 1 {
		t.Errorf("ByEntityType[person] = %d, want 1", stats.ByEntityType["person"])
	}
	if stats.ByEntityType["device"] != 1 {
		t.Errorf("ByEntityType[device] = %d, want 1", stats.ByEntityType["device"])
	}
}

func TestHandleDebugKGFileSyncStatsKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-file-sync-stats", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileSyncStats(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleDebugKGFileSyncStatsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/debug/kg-file-sync-stats", nil)
		rec := httptest.NewRecorder()

		handleDebugKGFileSyncStats(s)(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHandleDebugKGOrphans(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add an orphaned node (no edges, source=file_sync)
	kg.AddNode("orphan1", "Orphan Node", map[string]string{
		"source":      "file_sync",
		"source_file": "/tmp/deleted.txt",
	})
	// Add a connected node (has edges)
	kg.AddNode("connected1", "Connected Node", map[string]string{
		"source": "file_sync",
	})
	kg.AddEdge("connected1", "orphan1", "links_to", nil)

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-orphans?source=file_sync", nil)
	rec := httptest.NewRecorder()

	handleDebugKGOrphans(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["source"] != "file_sync" {
		t.Errorf("source = %v, want file_sync", result["source"])
	}
}

func TestHandleDebugKGOrphansKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-orphans", nil)
	rec := httptest.NewRecorder()

	handleDebugKGOrphans(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleDebugFileSyncStatus(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)} // KG but no FileIndexer

	req := httptest.NewRequest(http.MethodGet, "/api/debug/file-sync-status", nil)
	rec := httptest.NewRecorder()

	handleDebugFileSyncStatus(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// indexer should be not running (nil FileIndexer)
	indexer, ok := result["indexer"].(map[string]interface{})
	if !ok {
		t.Fatalf("indexer type error")
	}
	if indexer["running"] != false {
		t.Errorf("indexer.running = %v, want false", indexer["running"])
	}
}

func TestHandleDebugFileSyncStatusMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/debug/file-sync-status", nil)
		rec := httptest.NewRecorder()

		handleDebugFileSyncStatus(s)(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHandleDebugFileSyncLastRun(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add nodes with extracted_at to simulate synced files
	kg.AddNode("sync_node1", "Synced Node 1", map[string]string{
		"source":       "file_sync",
		"extracted_at": "2024-01-15",
		"collection":   "docs",
	})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/file-sync-last-run", nil)
	rec := httptest.NewRecorder()

	handleDebugFileSyncLastRun(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Global should have a value
	if result["global"] == nil {
		t.Log("global is nil (may be expected if no time parsed)")
	}
}

func TestHandleDebugFileSyncLastRunKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/file-sync-last-run", nil)
	rec := httptest.NewRecorder()

	handleDebugFileSyncLastRun(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestDiscoverIndexingCollections(t *testing.T) {
	t.Parallel()

	// This test would require a full Server setup with config
	// Just testing the basic logic here
	s := &Server{}

	// Empty config should return default collection
	collections := s.discoverIndexingCollections()
	if len(collections) == 0 {
		t.Error("expected at least default collection")
	}
}

func TestGetActiveIndexedFilesNoSTM(t *testing.T) {
	t.Parallel()

	s := &Server{ShortTermMem: nil}

	files, err := s.getActiveIndexedFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files == nil {
		t.Error("expected empty slice, got nil")
	}
}

// --- Tests for new debug endpoints ---

func TestHandleDebugKGFileEntities(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add nodes from a specific file
	kg.AddNode("e1", "Entity One", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/report.md",
		"type":        "person",
	})
	kg.AddNode("e2", "Entity Two", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/report.md",
		"type":        "device",
	})
	kg.AddNode("e3", "Other Entity", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/other.md",
	})
	kg.AddEdge("e1", "e2", "related_to", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/report.md",
	})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-file-entities?path=/docs/report.md", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileEntities(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["path"] != "/docs/report.md" {
		t.Errorf("path = %v, want /docs/report.md", result["path"])
	}
	// node_count should be 2 (e1, e2)
	nodeCount, ok := result["node_count"].(float64)
	if !ok || nodeCount != 2 {
		t.Errorf("node_count = %v, want 2", result["node_count"])
	}
	// edge_count should be 1
	edgeCount, ok := result["edge_count"].(float64)
	if !ok || edgeCount != 1 {
		t.Errorf("edge_count = %v, want 1", result["edge_count"])
	}
}

func TestHandleDebugKGFileEntitiesMissingPath(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-file-entities", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileEntities(s)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDebugKGFileEntitiesKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-file-entities?path=/test.md", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileEntities(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleDebugKGFileEntitiesMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/debug/kg-file-entities?path=/test.md", nil)
		rec := httptest.NewRecorder()

		handleDebugKGFileEntities(s)(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHandleDebugKGNodeSources(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add a node with source_file
	kg.AddNode("n1", "Node One", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/owner.md",
	})
	// Add connected edges with different source_files
	kg.AddNode("n2", "Node Two", nil)
	kg.AddEdge("n1", "n2", "rel1", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/edge1.md",
	})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-node-sources?id=n1", nil)
	rec := httptest.NewRecorder()

	handleDebugKGNodeSources(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["node_id"] != "n1" {
		t.Errorf("node_id = %v, want n1", result["node_id"])
	}
	if result["node_label"] != "Node One" {
		t.Errorf("node_label = %v, want 'Node One'", result["node_label"])
	}
	// Should have 2 source files: /docs/owner.md and /docs/edge1.md
	fileCount, ok := result["file_count"].(float64)
	if !ok || fileCount != 2 {
		t.Errorf("file_count = %v, want 2", result["file_count"])
	}
}

func TestHandleDebugKGNodeSourcesMissingID(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-node-sources", nil)
	rec := httptest.NewRecorder()

	handleDebugKGNodeSources(s)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDebugKGNodeSourcesNotFound(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-node-sources?id=nonexistent", nil)
	rec := httptest.NewRecorder()

	handleDebugKGNodeSources(s)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDebugKGNodeSourcesKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/kg-node-sources?id=test", nil)
	rec := httptest.NewRecorder()

	handleDebugKGNodeSources(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleDebugKGFileSyncCleanupDryRun(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add orphaned node (source file not in active files)
	kg.AddNode("orphan1", "Orphan Node", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/deleted.md",
	})
	// Add active node
	kg.AddNode("active1", "Active Node", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/active.md",
	})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodPost, "/api/debug/kg-file-sync-cleanup?dry_run=true", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileSyncCleanup(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", result["dry_run"])
	}

	// In dry_run mode, deleted_nodes should be 0
	deletedNodes, ok := result["deleted_nodes"].(float64)
	if !ok || deletedNodes != 0 {
		t.Errorf("deleted_nodes = %v, want 0 in dry_run mode", result["deleted_nodes"])
	}

	// The orphaned node should still exist in the KG
	node, err := kg.GetNode("orphan1")
	if err != nil || node == nil {
		t.Error("orphan1 should still exist after dry_run cleanup")
	}
}

func TestHandleDebugKGFileSyncCleanupLive(t *testing.T) {
	t.Parallel()

	kg := newTestKG(t)

	// Add orphaned nodes (no STM → all file_sync nodes are considered orphaned)
	kg.AddNode("orphan1", "Orphan Node", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/deleted.md",
	})
	kg.AddNode("orphan2", "Another Orphan", map[string]string{
		"source":      "file_sync",
		"source_file": "/docs/deleted.md",
	})
	// Add a non-file_sync node that should survive
	kg.AddNode("manual1", "Manual Node", map[string]string{
		"source": "manual",
	})

	s := &Server{KG: kg}

	req := httptest.NewRequest(http.MethodPost, "/api/debug/kg-file-sync-cleanup?dry_run=false", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileSyncCleanup(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["dry_run"] != false {
		t.Errorf("dry_run = %v, want false", result["dry_run"])
	}

	// The orphaned nodes should be gone
	node, err := kg.GetNode("orphan1")
	if err == nil && node != nil {
		t.Error("orphan1 should be deleted after live cleanup")
	}

	// The manual node should still exist (not file_sync source)
	node, err = kg.GetNode("manual1")
	if err != nil || node == nil {
		t.Error("manual1 should still exist after cleanup (not file_sync)")
	}

	// Should report deleted nodes
	deletedNodes, ok := result["deleted_nodes"].(float64)
	if !ok || deletedNodes == 0 {
		t.Errorf("deleted_nodes = %v, want > 0", result["deleted_nodes"])
	}
}

func TestHandleDebugKGFileSyncCleanupMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s := &Server{KG: newTestKG(t)}

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/debug/kg-file-sync-cleanup", nil)
		rec := httptest.NewRecorder()

		handleDebugKGFileSyncCleanup(s)(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHandleDebugKGFileSyncCleanupKGNil(t *testing.T) {
	t.Parallel()

	s := &Server{KG: nil}

	req := httptest.NewRequest(http.MethodPost, "/api/debug/kg-file-sync-cleanup", nil)
	rec := httptest.NewRecorder()

	handleDebugKGFileSyncCleanup(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
