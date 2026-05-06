package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestCodeStudioReadonlyBlocksMutatingHandlers(t *testing.T) {
	t.Parallel()

	srv := testCodeStudioReadonlyServer(t)
	h := codeStudioHandlers{server: srv, docker: noopCodeStudioDocker{}}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		req     *http.Request
	}{
		{"write", h.handleFile, httptest.NewRequest(http.MethodPut, "/api/code-studio/file", strings.NewReader(`{"path":"/workspace/a.txt","content":"x"}`))},
		{"move", h.handleFile, httptest.NewRequest(http.MethodPatch, "/api/code-studio/file", strings.NewReader(`{"old_path":"/workspace/a.txt","new_path":"/workspace/b.txt"}`))},
		{"delete", h.handleFile, httptest.NewRequest(http.MethodDelete, "/api/code-studio/file?path=/workspace/a.txt", nil)},
		{"directory", h.handleDirectory, httptest.NewRequest(http.MethodPost, "/api/code-studio/directory", strings.NewReader(`{"path":"/workspace/new"}`))},
		{"exec", h.handleExec, httptest.NewRequest(http.MethodPost, "/api/code-studio/exec", strings.NewReader(`{"command":"go","args":["test"]}`))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, tt.req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
		})
	}
}

func TestCodeStudioReadonlyBlocksUpload(t *testing.T) {
	t.Parallel()

	srv := testCodeStudioReadonlyServer(t)
	h := codeStudioHandlers{server: srv, docker: noopCodeStudioDocker{}}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("path", "/workspace")
	part, err := writer.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("x"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/code-studio/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.handleUpload(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func testCodeStudioReadonlyServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.ReadOnly = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "desktop")
	cfg.VirtualDesktop.CodeStudio.Enabled = true
	cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.WorkspaceDir = t.TempDir()
	srv := newServerFromOptions(StartOptions{
		Cfg:        cfg,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		ShutdownCh: make(chan struct{}),
	})
	return srv
}

type noopCodeStudioDocker struct{}

func (noopCodeStudioDocker) Exec(context.Context, string, []string, time.Duration) (codeStudioExecResult, error) {
	return codeStudioExecResult{}, nil
}

func (noopCodeStudioDocker) CreateTerminalExec(context.Context, string, int, int) (string, error) {
	return "", nil
}
func (noopCodeStudioDocker) StartExec(context.Context, string) ([]byte, error)  { return nil, nil }
func (noopCodeStudioDocker) ResizeExec(context.Context, string, int, int) error { return nil }
