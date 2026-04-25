package server

import (
	"bytes"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/remote"
)

func TestOAuthCallbackDoesNotRenderProviderErrorHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?state=missing&error=%3Cscript%3Ealert(1)%3C%2Fscript%3E&error_description=%3Cimg%20src=x%20onerror=alert(2)%3E", nil)
	rec := httptest.NewRecorder()

	handleOAuthCallback(&Server{}).ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<script>") || strings.Contains(body, "<img") || strings.Contains(body, "onerror=") {
		t.Fatalf("OAuth callback rendered executable provider HTML: %s", body)
	}
}

func TestAuthBypassDoesNotAllowArbitraryStaticSuffix(t *testing.T) {
	if isAuthBypassed("/api/config/secret.png") {
		t.Fatal("unexpected auth bypass for arbitrary API path ending in .png")
	}
	if !isAuthBypassed("/img/robot.png") {
		t.Fatal("expected public UI image assets to remain available without auth")
	}
}

func TestHandleUploadRejectsActiveContentExtensions(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	s := &Server{
		Cfg:    cfg,
		Logger: slog.Default(),
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "evil.html")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("<script>alert(1)</script>")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleUpload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "attachments")); err == nil {
		t.Fatal("upload created attachments directory for rejected active content")
	}
}

func TestWebSocketOriginPolicyRejectsMismatchedBrowserOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/remote/ws", nil)
	req.Host = "aurago.local"
	req.Header.Set("Origin", "https://evil.example")

	if remoteUpgrader.CheckOrigin(req) {
		t.Fatal("remote websocket accepted mismatched browser origin")
	}
	if wsUpgrader.CheckOrigin(req) {
		t.Fatal("invasion websocket accepted mismatched browser origin")
	}
}

func TestWebSocketOriginPolicyAllowsNonBrowserAgents(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/remote/ws", nil)
	req.Host = "aurago.local"

	if !remoteUpgrader.CheckOrigin(req) {
		t.Fatal("remote websocket rejected non-browser request without Origin")
	}
	if !wsUpgrader.CheckOrigin(req) {
		t.Fatal("invasion websocket rejected non-browser request without Origin")
	}
}

func TestRemoteDownloadSuccessDoesNotIncrementLoginLockout(t *testing.T) {
	loginMu.Lock()
	loginRecords = make(map[string]*loginRecord)
	loginMu.Unlock()

	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll("deploy", 0755); err != nil {
		t.Fatalf("MkdirAll deploy: %v", err)
	}
	if err := os.WriteFile(filepath.Join("deploy", "aurago-remote_linux_amd64"), []byte("binary"), 0644); err != nil {
		t.Fatalf("WriteFile binary: %v", err)
	}

	db, err := remote.InitDB(filepath.Join(tmp, "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Server.Port = 8088
	s := &Server{
		Cfg:       cfg,
		Logger:    slog.Default(),
		RemoteHub: remote.NewRemoteHub(db, nil, slog.Default()),
	}
	handler := handleRemoteDownload(s)
	clientIP := "203.0.113.10"

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=test", nil)
		req.Host = "aurago.local"
		req.RemoteAddr = clientIP + ":1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("download %d status = %d, want 200; body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	if IsLockedOut(clientIP) {
		t.Fatal("successful remote downloads should not increment login lockout bucket")
	}
}
