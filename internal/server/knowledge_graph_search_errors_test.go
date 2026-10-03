package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleKnowledgeGraphSearchReportsDatabaseFailure(t *testing.T) {
	s := newTestKnowledgeGraphServer(t)
	if err := s.KG.AddNode("backup_server", "Backup Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := s.KG.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-graph/search?q=backup", nil)
	rec := httptest.NewRecorder()
	handleKnowledgeGraphSearch(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when the knowledge graph cannot be searched; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlePeopleLookupReportsDatabaseFailure(t *testing.T) {
	for _, mode := range []string{"fts", "explore"} {
		t.Run(mode, func(t *testing.T) {
			s := newTestKnowledgeGraphServer(t)
			if err := s.KG.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/people/lookup?q=andi&mode="+mode, nil)
			rec := httptest.NewRecorder()
			handlePeopleLookup(s).ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
