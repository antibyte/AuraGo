package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestCodeStudioPreviewIssueReturnsWorkspaceScopedURL(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.Cfg.VirtualDesktop.ReadOnly = true // a preview only reads: it must work on a read-only desktop
	p := newCodeStudioPreviewHandler(codeStudioHandlers{server: s})
	issue := func(method, body, workspace string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/code-studio/preview", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if workspace != "" {
			req.Header.Set("X-Code-Studio-Workspace", workspace)
		}
		rec := httptest.NewRecorder()
		p.handleIssue(rec, req)
		return rec
	}
	rec := issue(http.MethodPost, `{"path":"/workspace/site/my page.html"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("issue = %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		URL       string `json:"url"`
		Base      string `json:"base"`
		Token     string `json:"token"`
		Workspace string `json:"workspace"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Base != codeStudioPreviewPrefix+got.Token+"/" || got.URL != got.Base+"site/my%20page.html" || got.Workspace != "local" || got.ExpiresAt == "" {
		t.Fatalf("issue body = %+v", got)
	}
	s.Cfg.VirtualDesktop.ReadOnly = false // the store lookup below must keep the live desktop service config
	key, _ := s.codeStudioPreviewSigningKey()
	claims, ok := parseCodeStudioPreviewToken(key, got.Token, time.Now())
	if !ok || claims.Workspace != "local" || time.Until(time.Unix(claims.Expires, 0)) < 11*time.Hour+59*time.Minute {
		t.Fatalf("claims = %+v %v, want local for 12 h", claims, ok)
	}
	if rec := issue(http.MethodPost, `{"path":"/workspace/index.html"}`, "ssh-missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown workspace = %d, want 404", rec.Code)
	}
	if rec := issue(http.MethodPost, `{"path":"/etc/passwd"}`, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("outside path = %d, want 400", rec.Code)
	}
	if rec := issue(http.MethodGet, "", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rec.Code)
	}
}

// Mode L: the real local source reads through the container with the 25 MiB preview cap, not the 1 MiB editor cap.
// Mode A: keep it when Plan A's local backend still runs through h.docker; otherwise drop it.
func TestCodeStudioPreviewLocalSourceReadsThroughContainer(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{
		{ExitCode: 0, Output: "13|1710000000\n/workspace/index.html\x00"},
		{ExitCode: 0, Output: base64.StdEncoding.EncodeToString([]byte("<body></body>"))},
	}}
	p := newCodeStudioPreviewHandler(codeStudioHandlers{server: s, docker: docker})
	rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+testCodeStudioPreviewToken(t, s, "local")+"/index.html")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "__aurago/live.js") {
		t.Fatalf("local preview = %d %s", rec.Code, rec.Body.String())
	}
	if len(docker.commands) != 2 || docker.commands[0][4] != "probe" || docker.commands[1][4] != "read" || docker.commands[1][6] != "26214400" {
		t.Fatalf("commands = %q, want probe then read with the 25 MiB cap", docker.commands)
	}
}

func TestCodeStudioPreviewRouteIsRegisteredAndExemptFromSessionAuth(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	mux := http.NewServeMux()
	registerCodeStudioRoutes(mux, s)
	for path, want := range map[string]string{
		"/code-studio-preview/tok/index.html": codeStudioPreviewPrefix,
		"/api/code-studio/preview":            "/api/code-studio/preview",
	} {
		if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, path, nil)); pattern != want {
			t.Fatalf("%s routed to %q, want %q", path, pattern, want)
		}
	}
	handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for path, exempt := range map[string]bool{
		"/code-studio-preview/tok/index.html":       true,
		"/code-studio-preview/tok/__aurago/version": true,
		"/code-studio-previewx/tok/index.html":      false,
		"/api/code-studio/preview":                  false,
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if exempt != (rec.Code == http.StatusNoContent) {
			t.Errorf("%s: status %d, exempt=%v", path, rec.Code, exempt)
		}
	}
}
