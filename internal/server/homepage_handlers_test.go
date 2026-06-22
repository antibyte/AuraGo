package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/tsnetnode"
)

func TestHomepageBrowserURLUsesRequestHost(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "192.168.1.50:8443"

	got := homepageBrowserURLForRequest(req, 5173)
	if got != "http://192.168.1.50:5173" {
		t.Fatalf("homepageBrowserURLForRequest() = %q, want request host URL", got)
	}
}

func TestHomepageBrowserURLDoesNotInventTailscalePort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "aurago.taild1480.ts.net"

	got := homepageBrowserURLForRequest(req, 5173)
	if got != "" {
		t.Fatalf("homepageBrowserURLForRequest() = %q, want no local URL over Tailscale", got)
	}
}

func TestHomepageStatusBrowserURLUsesTailscaleHomepageExposure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Homepage.WebServerPort = 8080
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ExposeHomepage = true
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	s.TsNetManager = tsnetnode.NewManager(cfg, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "aurago.taild1480.ts.net"

	got := homepageStatusBrowserURL(s, cfg, req)
	if got != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("homepageStatusBrowserURL() = %q, want Tailscale Homepage URL", got)
	}
}

func TestEnrichHomepageStatusUsesTailscaleBrowserURL(t *testing.T) {
	t.Parallel()

	payload := map[string]interface{}{
		"web_container": map[string]interface{}{"running": true, "exists": true},
	}
	enrichHomepageStatusForRequest(payload, "https://aurago.taild1480.ts.net:8443")

	if payload["preview_url"] != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("preview_url = %#v, want Tailscale URL", payload["preview_url"])
	}
	web, ok := payload["web_container"].(map[string]interface{})
	if !ok || web["browser_url"] != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("web_container browser_url = %#v", payload["web_container"])
	}
}

func TestHandleHomepageHistoryListAndDelete(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := tools.RegisterProject(db, tools.HomepageProject{Name: "HistoryUIProject", Framework: "astro"})
	_, _ = tools.AddHomepageHistoryEntry(db, projectID, "decision", "Use dark hero", "homepage_file", []string{"design"})

	s := &Server{HomepageRegistryDB: db, Logger: slog.Default()}
	handler := handleHomepageHistory(s)

	// List history
	req := httptest.NewRequest(http.MethodGet, "/api/homepage/history?project_id="+strconv.FormatInt(projectID, 10), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Status  string `json:"status"`
		Total   int    `json:"total"`
		Entries []tools.HomepageHistoryEntry
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if listResp.Total != 1 {
		t.Fatalf("total = %d, want 1", listResp.Total)
	}
	if len(listResp.Entries) != 1 || listResp.Entries[0].Content != "Use dark hero" {
		t.Fatalf("unexpected entries: %+v", listResp.Entries)
	}

	// Delete history entry
	entryID := listResp.Entries[0].ID
	delReq := httptest.NewRequest(http.MethodDelete, "/api/homepage/history?id="+strconv.FormatInt(entryID, 10), nil)
	delRec := httptest.NewRecorder()
	handler(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for delete, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Verify deletion
	req2 := httptest.NewRequest(http.MethodGet, "/api/homepage/history?project_id="+strconv.FormatInt(projectID, 10), nil)
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	var listResp2 struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &listResp2)
	if listResp2.Total != 0 {
		t.Fatalf("total after delete = %d, want 0", listResp2.Total)
	}
}

func TestHandleHomepageHistoryWorkspaceFallback(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	workspace := t.TempDir()
	projectID, _, _ := tools.RegisterProject(db, tools.HomepageProject{Name: "WorkspaceFallback", ProjectDir: workspace, Framework: "astro"})
	_, _ = tools.AddHomepageHistoryEntry(db, projectID, "note", "Workspace fallback entry", "homepage_file", nil)

	cfg := &config.Config{}
	cfg.Homepage.WorkspacePath = workspace
	s := &Server{HomepageRegistryDB: db, Cfg: cfg, Logger: slog.Default()}
	handler := handleHomepageHistory(s)

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/history", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Total   int `json:"total"`
		Entries []tools.HomepageHistoryEntry
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if listResp.Total != 1 {
		t.Fatalf("total = %d, want 1", listResp.Total)
	}
}

func TestHandleHomepageHistoryUnknownProjectReturnsEmpty(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Homepage.WorkspacePath = "/nonexistent/workspace"
	s := &Server{HomepageRegistryDB: db, Cfg: cfg, Logger: slog.Default()}
	handler := handleHomepageHistory(s)

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/history", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Total   int `json:"total"`
		Entries []tools.HomepageHistoryEntry
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if listResp.Total != 0 {
		t.Fatalf("total = %d, want 0 for unknown project", listResp.Total)
	}
}
