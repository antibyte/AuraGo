package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestOpenSCADJobFileServesInlineByDefaultAndAttachmentOnDownload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.VirtualDesktop.OpenSCAD.Enabled = true
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "virtual_desktop.db")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	s := &Server{Cfg: cfg}
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
		if s.DesktopHub != nil {
			s.DesktopHub.Close()
		}
	})
	if _, _, err := s.getDesktopService(context.Background()); err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	jobDir := filepath.Join(cfg.Directories.DataDir, "openscad", "jobs", "oscad-inline")
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		t.Fatalf("create job dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "model.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0o600); err != nil {
		t.Fatalf("write svg: %v", err)
	}

	handlers := openSCADHandlers{server: s}
	inlineRec := httptest.NewRecorder()
	inlineReq := httptest.NewRequest(http.MethodGet, "/api/openscad/jobs/oscad-inline/files/model.svg", nil)
	handlers.handleJobPath(inlineRec, inlineReq)
	if inlineRec.Code != http.StatusOK {
		t.Fatalf("inline status = %d, want 200; body=%s", inlineRec.Code, inlineRec.Body.String())
	}
	if got := inlineRec.Header().Get("Content-Disposition"); strings.Contains(strings.ToLower(got), "attachment") {
		t.Fatalf("inline Content-Disposition = %q, want no attachment", got)
	}

	downloadRec := httptest.NewRecorder()
	downloadReq := httptest.NewRequest(http.MethodGet, "/api/openscad/jobs/oscad-inline/files/model.svg?download=1", nil)
	handlers.handleJobPath(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200; body=%s", downloadRec.Code, downloadRec.Body.String())
	}
	if got := downloadRec.Header().Get("Content-Disposition"); !strings.Contains(strings.ToLower(got), "attachment") {
		t.Fatalf("download Content-Disposition = %q, want attachment", got)
	}
}

func TestOpenSCADRenderReturnsPartialResultOnFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.VirtualDesktop.OpenSCAD.Enabled = true
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "virtual_desktop.db")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	s := &Server{Cfg: cfg}
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
		if s.DesktopHub != nil {
			s.DesktopHub.Close()
		}
	})
	if _, _, err := s.getDesktopService(context.Background()); err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	handlers := openSCADHandlers{server: s}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/openscad/render", strings.NewReader(`{"source_scad":"cube(1);","exports":["png"]}`))
	req.Header.Set("Content-Type", "application/json")
	handlers.handleRender(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.Code == http.StatusOK && body["status"] == "error" {
		if body["result"] == nil {
			t.Fatalf("expected partial result in error response, got %#v", body)
		}
	}
}
