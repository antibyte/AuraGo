package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	projectID, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "HistoryUIProject", ProjectDir: "history-ui-project", Framework: "astro"})
	if err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}
	if _, err := tools.AddHomepageHistoryEntry(db, projectID, "decision", "Use dark hero", "homepage_file", []string{"design"}); err != nil {
		t.Fatalf("AddHomepageHistoryEntry failed: %v", err)
	}

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
	projectDir := "workspace-fallback"
	projectID, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "WorkspaceFallback", ProjectDir: projectDir, Framework: "astro"})
	if err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}
	if _, err := tools.AddHomepageHistoryEntry(db, projectID, "note", "Workspace fallback entry", "homepage_file", nil); err != nil {
		t.Fatalf("AddHomepageHistoryEntry failed: %v", err)
	}

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

func TestHandleHomepageHistoryWorkspaceFallbackDoesNotGuessMultipleProjects(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectAID, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "WorkspaceFallbackA", ProjectDir: "workspace-fallback-a", Framework: "astro"})
	if err != nil {
		t.Fatalf("RegisterProject A failed: %v", err)
	}
	if _, err := tools.AddHomepageHistoryEntry(db, projectAID, "note", "Entry A", "homepage_file", nil); err != nil {
		t.Fatalf("AddHomepageHistoryEntry A failed: %v", err)
	}
	projectBID, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "WorkspaceFallbackB", ProjectDir: "workspace-fallback-b", Framework: "astro"})
	if err != nil {
		t.Fatalf("RegisterProject B failed: %v", err)
	}
	if _, err := tools.AddHomepageHistoryEntry(db, projectBID, "note", "Entry B", "homepage_file", nil); err != nil {
		t.Fatalf("AddHomepageHistoryEntry B failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Homepage.WorkspacePath = t.TempDir()
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
	if listResp.Total != 0 || len(listResp.Entries) != 0 {
		t.Fatalf("expected ambiguous fallback to return empty history, got %+v", listResp)
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

func TestHandleHomepageSitesListDetailAndReconcile(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := tools.HomepageWriteFile(tools.HomepageConfig{WorkspacePath: workspace, DockerHost: "tcp://127.0.0.1:1"}, projectDir+"/index.html", "<h1>Site</h1>", slog.Default()); !strings.Contains(err, `"status":"ok"`) {
		t.Fatalf("write homepage file failed: %s", err)
	}
	cfg := &config.Config{}
	cfg.Homepage.WorkspacePath = workspace
	homepageCfg := tools.HomepageConfig{WorkspacePath: workspace}
	proj, err := tools.EnsureHomepageProjectForDir(db, homepageCfg, projectPath, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if got := tools.SaveHomepageRevisionAndState(homepageCfg, db, projectDir, "initial", "test", "test", nil, slog.Default()); len(got.Warnings) > 0 {
		t.Fatalf("save revision warnings: %v", got.Warnings)
	} else {
		remoteBody := "remote-one"
		remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(remoteBody))
		}))
		defer remoteServer.Close()
		if err := tools.RecordHomepageDeployment(db, tools.HomepageDeploymentRecord{
			ProjectID:        proj.ID,
			RevisionID:       got.RevisionID,
			Provider:         "netlify",
			ProviderTargetID: "site-1",
			ProviderDeployID: "deploy-1",
			URL:              remoteServer.URL,
			BuildDir:         ".",
			Status:           "ok",
		}); err != nil {
			t.Fatalf("record deployment: %v", err)
		}
		if _, err := tools.ReconcileHomepageProject(homepageCfg, db, projectDir, slog.Default()); err != nil {
			t.Fatalf("reconcile before detail: %v", err)
		}
	}

	s := &Server{HomepageRegistryDB: db, Cfg: cfg, Logger: slog.Default()}
	listReq := httptest.NewRequest(http.MethodGet, "/api/homepage/sites", nil)
	listRec := httptest.NewRecorder()
	handleHomepageSites(s)(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Status string `json:"status"`
		Total  int    `json:"total"`
		Sites  []struct {
			ID          int64  `json:"id"`
			ProjectDir  string `json:"project_dir"`
			DriftStatus string `json:"drift_status"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Sites) != 1 || listResp.Sites[0].ProjectDir != "site-a" {
		t.Fatalf("unexpected site list: %+v", listResp)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/homepage/sites/"+strconv.FormatInt(proj.ID, 10), nil)
	detailRec := httptest.NewRecorder()
	handleHomepageSiteByID(s)(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d: %s", detailRec.Code, detailRec.Body.String())
	}
	if !strings.Contains(detailRec.Body.String(), `"deploy_targets"`) {
		t.Fatalf("detail response missing deploy_targets: %s", detailRec.Body.String())
	}
	if !strings.Contains(detailRec.Body.String(), `"remote_observations"`) {
		t.Fatalf("detail response missing remote_observations: %s", detailRec.Body.String())
	}

	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>Changed</h1>"), 0644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	reconcileReq := httptest.NewRequest(http.MethodPost, "/api/homepage/sites/"+strconv.FormatInt(proj.ID, 10)+"/reconcile", nil)
	reconcileRec := httptest.NewRecorder()
	handleHomepageSiteByID(s)(reconcileRec, reconcileReq)
	if reconcileRec.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d: %s", reconcileRec.Code, reconcileRec.Body.String())
	}
	if !strings.Contains(reconcileRec.Body.String(), `"drift_status":"local_changed"`) {
		t.Fatalf("expected local_changed reconcile response, got %s", reconcileRec.Body.String())
	}
}
