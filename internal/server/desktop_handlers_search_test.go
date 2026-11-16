package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopSearchPagedResponse(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	for path, content := range map[string][]byte{
		"search-fixtures/alpha.png": []byte("image"),
		"search-fixtures/beta.txt":  []byte("text"),
		"search-fixtures/gamma.png": []byte("another image"),
	} {
		if err := svc.WriteFileBytes(context.Background(), path, content, desktop.SourceUser); err != nil {
			t.Fatalf("WriteFileBytes(%q): %v", path, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/search?path=search-fixtures&query=a&type=image&limit=1", nil)
	resp := httptest.NewRecorder()
	handleDesktopSearch(s).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("search status = %d, body %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Files      []desktop.FileEntry `json:"files"`
		HasMore    bool                `json:"has_more"`
		Truncated  bool                `json:"truncated"`
		NextOffset int                 `json:"next_offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(body.Files) != 1 || body.Files[0].Path != "search-fixtures/alpha.png" || !body.HasMore || body.Truncated || body.NextOffset != 1 {
		t.Fatalf("paged search response = %+v", body)
	}
}

func TestDesktopSearchPagedRequestRejectsInvalidBounds(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	for _, query := range []string{
		"/api/desktop/search?query=x&limit=101",
		"/api/desktop/search?query=x&offset=1001",
		"/api/desktop/search?query=x&modified_after=not-a-date&limit=10",
		"/api/desktop/search?query=x&min_size=1.5&limit=10",
	} {
		req := httptest.NewRequest(http.MethodGet, query, nil)
		resp := httptest.NewRecorder()
		handleDesktopSearch(s).ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("search %q status = %d, want 400: %s", query, resp.Code, resp.Body.String())
		}
	}
}
