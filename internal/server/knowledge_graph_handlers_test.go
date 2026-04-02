package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/memory"
)

func newTestKnowledgeGraphServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	return &Server{KG: kg}
}

func TestHandleKnowledgeGraphNodes(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("proxmox", "Proxmox", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/nodes?limit=10", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodes(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var nodes []memory.Node
	if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "proxmox" {
		t.Fatalf("unexpected nodes payload: %#v", nodes)
	}
}

func TestHandleKnowledgeGraphSearch(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := s.KG.AddEdge("backup_server", "nas", "replicates_to", map[string]string{"notes": "nightly backup target"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/search?q=backup", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphSearch(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["query"] != "backup" {
		t.Fatalf("query = %#v, want backup", payload["query"])
	}
	if _, ok := payload["nodes"]; !ok {
		t.Fatalf("payload missing nodes: %#v", payload)
	}
	if _, ok := payload["edges"]; !ok {
		t.Fatalf("payload missing edges: %#v", payload)
	}
}

func TestHandleKnowledgeGraphNodeDetail(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("proxmox", "Proxmox", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := s.KG.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := s.KG.AddEdge("proxmox", "backup_server", "replicates_to", map[string]string{"notes": "nightly"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/node?id=proxmox&limit=10", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodeDetail(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["node"]; !ok {
		t.Fatalf("payload missing node: %#v", payload)
	}
	if _, ok := payload["neighbors"]; !ok {
		t.Fatalf("payload missing neighbors: %#v", payload)
	}
	if _, ok := payload["edges"]; !ok {
		t.Fatalf("payload missing edges: %#v", payload)
	}
}

func TestHandleKnowledgeGraphQuality(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("router", "Router", map[string]string{"type": "device", "protected": "true"}); err != nil {
		t.Fatalf("AddNode router: %v", err)
	}
	if err := s.KG.AddNode("nas_a", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode nas_a: %v", err)
	}
	if err := s.KG.AddNode("nas_b", "NAS", nil); err != nil {
		t.Fatalf("AddNode nas_b: %v", err)
	}
	if err := s.KG.AddEdge("router", "nas_a", "backs_up", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/quality?limit=5", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphQuality(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload memory.KnowledgeGraphQualityReport
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ProtectedNodes != 1 {
		t.Fatalf("ProtectedNodes = %d, want 1", payload.ProtectedNodes)
	}
	if payload.DuplicateGroups != 1 {
		t.Fatalf("DuplicateGroups = %d, want 1", payload.DuplicateGroups)
	}
	if payload.UntypedNodes != 1 {
		t.Fatalf("UntypedNodes = %d, want 1", payload.UntypedNodes)
	}
}

func TestHandleKnowledgeGraphNodeUpdate(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	body := bytes.NewBufferString(`{"id":"backup_server","label":"Primary Backup Server","properties":{"type":"device","notes":"manually updated"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/knowledge-graph/node", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodeDetail(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}

	node, err := s.KG.GetNode("backup_server")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil || node.Label != "Primary Backup Server" {
		t.Fatalf("unexpected node after update: %#v", node)
	}
	if node.Properties["notes"] != "manually updated" {
		t.Fatalf("notes = %#v, want manually updated", node.Properties["notes"])
	}
}

func TestHandleKnowledgeGraphNodeProtect(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("proxmox", "Proxmox", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	body := bytes.NewBufferString(`{"id":"proxmox","protected":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/node/protect", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodeProtect(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}

	node, err := s.KG.GetNode("proxmox")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil || !node.Protected {
		t.Fatalf("expected protected node, got %#v", node)
	}
}

func TestHandleKnowledgeGraphNodeDeleteProtectedRejected(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("vault", "Vault", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-graph/node?id=vault", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodeDetail(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleKnowledgeGraphEdgeUpdate(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddEdge("proxmox", "backup_server", "replicates_to", map[string]string{"notes": "nightly"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	body := bytes.NewBufferString(`{"source":"proxmox","target":"backup_server","relation":"replicates_to","new_relation":"backs_up","properties":{"notes":"manual"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/knowledge-graph/edge", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleKnowledgeGraphEdgeMutate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleKnowledgeGraphEdgeDelete(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddEdge("proxmox", "backup_server", "replicates_to", map[string]string{"notes": "nightly"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-graph/edge?source=proxmox&target=backup_server&relation=replicates_to", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphEdgeMutate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
}
