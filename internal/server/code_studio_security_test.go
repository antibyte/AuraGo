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
		want    int
	}{
		{"write", h.handleFile, httptest.NewRequest(http.MethodPut, "/api/code-studio/file", strings.NewReader(`{"path":"/workspace/a.txt","content":"x"}`)), http.StatusForbidden},
		{"move", h.handleFile, httptest.NewRequest(http.MethodPatch, "/api/code-studio/file", strings.NewReader(`{"old_path":"/workspace/a.txt","new_path":"/workspace/b.txt"}`)), http.StatusForbidden},
		{"delete", h.handleFile, httptest.NewRequest(http.MethodDelete, "/api/code-studio/file?path=/workspace/a.txt", nil), http.StatusForbidden},
		{"directory", h.handleDirectory, httptest.NewRequest(http.MethodPost, "/api/code-studio/directory", strings.NewReader(`{"path":"/workspace/new"}`)), http.StatusForbidden},
		{"exec", h.handleExec, httptest.NewRequest(http.MethodPost, "/api/code-studio/exec", strings.NewReader(`{"command":"go","args":["test"]}`)), http.StatusForbidden},
		{"git branch create", h.handleGitBranch, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/branch?name=feature&action=create", nil), http.StatusMethodNotAllowed},
		{"git branch switch", h.handleGitBranch, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/branch?name=main&action=switch", nil), http.StatusMethodNotAllowed},
		{"git branch post", h.handleGitBranch, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"create","name":"feature"}`)), http.StatusForbidden},
		{"git identity", h.handleGitIdentity, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/identity", strings.NewReader(`{"name":"Ada","email":"ada@example.com"}`)), http.StatusForbidden},
		{"git commit", h.handleGitCommit, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"save","paths":["main.go"]}`)), http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, tt.req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.want, rec.Body.String())
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

func (noopCodeStudioDocker) PutArchive(context.Context, string, []byte) error { return nil }

func (noopCodeStudioDocker) Exec(context.Context, string, []string, time.Duration) (codeStudioExecResult, error) {
	return codeStudioExecResult{}, nil
}
