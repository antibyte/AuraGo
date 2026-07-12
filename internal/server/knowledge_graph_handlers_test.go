package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"aurago/internal/config"
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

func TestHandleKnowledgeGraphSearchHonorsIncludeLowConfidence(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("andi", "Andi", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode andi: %v", err)
	}
	if err := s.KG.AddNode("png", "png", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode png: %v", err)
	}
	if err := s.KG.AddEdge("andi", "png", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge pending: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/search?q=andi", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphSearch(s).ServeHTTP(rec, req)
	var hidden struct {
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &hidden); err != nil {
		t.Fatalf("decode default payload: %v", err)
	}
	if len(hidden.Edges) != 0 {
		t.Fatalf("default search should hide low-confidence edge, got %#v", hidden.Edges)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/search?q=andi&include_low_confidence=true", nil)
	rec = httptest.NewRecorder()
	handleKnowledgeGraphSearch(s).ServeHTTP(rec, req)
	var included struct {
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &included); err != nil {
		t.Fatalf("decode override payload: %v", err)
	}
	if len(included.Edges) != 1 || included.Edges[0].Relation != "co_mentioned_with" {
		t.Fatalf("override search should include low-confidence edge, got %#v", included.Edges)
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

func TestHandleKnowledgeGraphNodeDetailHonorsIncludeLowConfidence(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("andi", "Andi", map[string]string{"type": "person"}); err != nil {
		t.Fatalf("AddNode andi: %v", err)
	}
	if err := s.KG.AddNode("png", "png", map[string]string{"type": "concept"}); err != nil {
		t.Fatalf("AddNode png: %v", err)
	}
	if err := s.KG.AddEdge("andi", "png", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge pending: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/node?id=andi&limit=10", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphNodeDetail(s).ServeHTTP(rec, req)
	var hidden struct {
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &hidden); err != nil {
		t.Fatalf("decode default payload: %v", err)
	}
	if len(hidden.Edges) != 0 {
		t.Fatalf("default node detail should hide low-confidence edge, got %#v", hidden.Edges)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/node?id=andi&limit=10&include_low_confidence=true", nil)
	rec = httptest.NewRecorder()
	handleKnowledgeGraphNodeDetail(s).ServeHTTP(rec, req)
	var included struct {
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &included); err != nil {
		t.Fatalf("decode override payload: %v", err)
	}
	if len(included.Edges) != 1 || included.Edges[0].Relation != "co_mentioned_with" {
		t.Fatalf("override node detail should include low-confidence edge, got %#v", included.Edges)
	}
}

func TestHandleKnowledgeGraphHealth(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("nas", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/health", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphHealth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload memory.KnowledgeGraphHealthReport
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.TotalNodes != 1 {
		t.Fatalf("TotalNodes = %d, want 1", payload.TotalNodes)
	}
	if payload.DirtyNodes != 1 {
		t.Fatalf("DirtyNodes = %d, want 1", payload.DirtyNodes)
	}
}

func TestHandleKnowledgeGraphHealthUnavailable(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/health", nil)
	rec := httptest.NewRecorder()

	handleKnowledgeGraphHealth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["error"] == "" {
		t.Fatalf("payload error = %#v, want non-empty error message", payload["error"])
	}
}

func TestHandleKnowledgeGraphHealthIncludesLifecycleCounts(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if _, err := s.KG.AddEdgeWithProvenance("user", "german", "primary_language", nil, memory.KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatalf("AddEdgeWithProvenance german: %v", err)
	}
	if _, err := s.KG.AddEdgeWithProvenance("user", "english", "primary_language", nil, memory.KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatalf("AddEdgeWithProvenance english: %v", err)
	}
	if err := s.KG.RetractEdge("user", "english", "primary_language", "wrong fact"); err != nil {
		t.Fatalf("RetractEdge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/health", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphHealth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload memory.KnowledgeGraphHealthReport
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.AcceptedEdges != 1 || payload.RetractedEdges != 1 || payload.OpenConflicts != 0 {
		t.Fatalf("unexpected lifecycle counts: %+v", payload)
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
	if err := s.KG.AddEdge("router", "nas_b", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge pending: %v", err)
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
	if payload.PendingEdges != 1 {
		t.Fatalf("PendingEdges = %d, want 1", payload.PendingEdges)
	}
	if payload.LowConfidenceEdges != 1 {
		t.Fatalf("LowConfidenceEdges = %d, want 1", payload.LowConfidenceEdges)
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

func TestHandleKnowledgeGraphEdgeClaims(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	claim, err := s.KG.AddEdgeWithProvenance("server", "portainer", "uses", nil, memory.KGProvenanceInput{
		SourceKind:   "user",
		SessionID:    "session-test",
		RawText:      "Server uses Portainer",
		EvidenceType: "remember",
	})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/edge/claims?source=server&target=portainer&relation=uses&include_inactive=true", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphEdgeClaims(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Status string           `json:"status"`
		Count  int              `json:"count"`
		Claims []memory.KGClaim `json:"claims"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Status != "ok" || payload.Count != 1 || payload.Claims[0].ID != claim.ID {
		t.Fatalf("unexpected claims payload: %+v", payload)
	}
	if payload.Claims[0].Evidence == nil || payload.Claims[0].Evidence.RawText != "Server uses Portainer" {
		t.Fatalf("claim evidence missing from payload: %+v", payload.Claims[0])
	}
}

func TestHandleKnowledgeGraphConflictsAndResolve(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	germanClaim, err := s.KG.AddEdgeWithProvenance("user", "german", "primary_language", nil, memory.KGProvenanceInput{SourceKind: "user"})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance german: %v", err)
	}
	englishClaim, err := s.KG.AddEdgeWithProvenance("user", "english", "primary_language", nil, memory.KGProvenanceInput{SourceKind: "user"})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance english: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/conflicts?limit=10", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphConflicts(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var conflictsPayload struct {
		Status    string              `json:"status"`
		Count     int                 `json:"count"`
		Conflicts []memory.KGConflict `json:"conflicts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &conflictsPayload); err != nil {
		t.Fatalf("decode conflicts: %v", err)
	}
	if conflictsPayload.Status != "ok" || conflictsPayload.Count != 1 || len(conflictsPayload.Conflicts) != 1 {
		t.Fatalf("unexpected conflicts payload: %+v", conflictsPayload)
	}

	body := bytes.NewBufferString(`{"conflict_id":` + strconv.FormatInt(conflictsPayload.Conflicts[0].ID, 10) + `,"claim_id":"` + englishClaim.ID + `","reason":"new correction"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/conflicts/resolve", body)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handleKnowledgeGraphConflictResolve(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
	losingClaims, err := s.KG.GetClaimsForEdge("user", "german", "primary_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge losing: %v", err)
	}
	if germanClaim.ID == englishClaim.ID || len(losingClaims) != 1 || losingClaims[0].Status != memory.KGClaimSuperseded {
		t.Fatalf("expected losing claim superseded, got %+v", losingClaims)
	}
}

func TestRequireAdminBlocksKnowledgeGraphConflictResolveWhenAuthEnabled(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	s.Cfg = &config.Config{}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "session-secret"
	s.Cfg.Auth.PasswordHash = "configured"

	body := bytes.NewBufferString(`{"conflict_id":1,"claim_id":"claim-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/conflicts/resolve", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	requireAdmin(s, handleKnowledgeGraphConflictResolve(s)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleKnowledgeGraphMerge(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("nas_primary", "NAS", map[string]string{"type": "device", "ip": "10.0.0.1"}); err != nil {
		t.Fatalf("AddNode primary: %v", err)
	}
	if err := s.KG.AddNode("nas_secondary", "NAS", map[string]string{"type": "device", "role": "backup"}); err != nil {
		t.Fatalf("AddNode secondary: %v", err)
	}
	if err := s.KG.AddEdge("nas_secondary", "service_backup", "hosts", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	body := bytes.NewBufferString(`{"target_id":"nas_primary","source_id":"nas_secondary"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/merge", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleKnowledgeGraphMerge(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if node, err := s.KG.GetNode("nas_secondary"); err != nil || node != nil {
		t.Fatalf("expected source node removed, node=%v err=%v", node, err)
	}
	merged, err := s.KG.GetNode("nas_primary")
	if err != nil || merged == nil {
		t.Fatalf("expected merged target node, node=%v err=%v", merged, err)
	}
	if merged.Properties["ip"] != "10.0.0.1" || merged.Properties["role"] != "backup" {
		t.Fatalf("merged properties = %#v, want merged props", merged.Properties)
	}
}

func TestRequireAdminBlocksKnowledgeGraphMergeWhenAuthEnabled(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	s.Cfg = &config.Config{}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "session-secret"
	s.Cfg.Auth.PasswordHash = "configured"

	body := bytes.NewBufferString(`{"target_id":"target","source_id":"source"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/merge", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	requireAdmin(s, handleKnowledgeGraphMerge(s)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleKnowledgeGraphMergeProtectedSourceRejected(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("keeper", "Keeper", map[string]string{"protected": "true"}); err != nil {
		t.Fatalf("AddNode keeper: %v", err)
	}
	if err := s.KG.AddNode("target", "Target", nil); err != nil {
		t.Fatalf("AddNode target: %v", err)
	}

	body := bytes.NewBufferString(`{"target_id":"target","source_id":"keeper"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-graph/merge", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleKnowledgeGraphMerge(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d, body=%s", rec.Code, rec.Body.String())
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
