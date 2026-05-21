package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestDesktopStoreOperationContextCancelsOnShutdown(t *testing.T) {
	shutdownCh := make(chan struct{})
	ctx, cancel := desktopStoreOperationContext(shutdownCh, time.Minute)
	defer cancel()

	close(shutdownCh)
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("context error = %v, want canceled", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("operation context did not cancel on shutdown")
	}
}

func TestDesktopStoreInstallRejectsVirtualDesktopReadOnly(t *testing.T) {
	s := testDesktopStorePolicyServer(t, true, true, false)
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/store/install", bytes.NewBufferString(`{"app_id":"node-red","bind_mode":"local"}`))
	rec := httptest.NewRecorder()

	handleDesktopStoreInstall(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestDesktopStoreInstallRejectsDockerReadOnly(t *testing.T) {
	s := testDesktopStorePolicyServer(t, false, true, true)
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/store/install", bytes.NewBufferString(`{"app_id":"node-red","bind_mode":"local"}`))
	rec := httptest.NewRecorder()

	handleDesktopStoreInstall(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func testDesktopStorePolicyServer(t *testing.T, desktopReadOnly, dockerEnabled, dockerReadOnly bool) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.ReadOnly = desktopReadOnly
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "virtual_desktop.db")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Docker.Enabled = dockerEnabled
	cfg.Docker.ReadOnly = dockerReadOnly
	return &Server{Cfg: cfg}
}
