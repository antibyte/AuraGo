package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDesktopAppsPatchUsesWriteScopeAndUpdatesVisibility(t *testing.T) {
	t.Parallel()

	srv, _, writeToken := testDesktopPermissionServer(t)
	configureDesktopAppsHandlerTest(t, srv)

	req := httptest.NewRequest(http.MethodPatch, "/api/desktop/apps?id=files", strings.NewReader(`{"dock_visible":false}`))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	handleDesktopApps(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	bootstrap, err := srv.DesktopService.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, app := range bootstrap.BuiltinApps {
		if app.ID == "files" {
			if app.DockVisible || !app.StartVisible {
				t.Fatalf("files visibility = dock:%v start:%v, want dock:false start:true", app.DockVisible, app.StartVisible)
			}
			return
		}
	}
	t.Fatal("files app not found in bootstrap")
}

func TestDesktopAppsPatchRequiresVisibilityField(t *testing.T) {
	t.Parallel()

	srv := &Server{Cfg: &config.Config{}}
	configureDesktopAppsHandlerTest(t, srv)

	req := httptest.NewRequest(http.MethodPatch, "/api/desktop/apps?id=files", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handleDesktopApps(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDesktopAppsDeleteBuiltinReturnsForbidden(t *testing.T) {
	t.Parallel()

	srv := &Server{Cfg: &config.Config{}}
	configureDesktopAppsHandlerTest(t, srv)

	req := httptest.NewRequest(http.MethodDelete, "/api/desktop/apps?id=files", nil)
	rec := httptest.NewRecorder()
	handleDesktopApps(srv).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func configureDesktopAppsHandlerTest(t *testing.T, srv *Server) {
	t.Helper()
	tmp := t.TempDir()
	srv.Cfg.VirtualDesktop.Enabled = true
	srv.Cfg.VirtualDesktop.AllowGeneratedApps = true
	srv.Cfg.VirtualDesktop.WorkspaceDir = filepath.Join(tmp, "desktop")
	srv.Cfg.SQLite.VirtualDesktopPath = filepath.Join(tmp, "desktop.db")
	srv.Cfg.Directories.DataDir = filepath.Join(tmp, "data")
	srv.Cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	t.Cleanup(func() {
		if srv.DesktopService != nil {
			_ = srv.DesktopService.Close()
		}
	})
}
