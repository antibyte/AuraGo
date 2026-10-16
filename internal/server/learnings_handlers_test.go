package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/codegraph"
	"aurago/internal/memory"
)

func learningHTTPFixture(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.Learnings.Enabled, s.Cfg.CodeGraph.Enabled, s.Cfg.Tools.Memory.Enabled = true, true, true
	s.Cfg.VirtualDesktop.Enabled, s.Cfg.VirtualDesktop.CodeStudio.Enabled = true, true
	s.Cfg.Directories.WorkspaceDir = t.TempDir()
	s.Cfg.VirtualDesktop.WorkspaceDir = t.TempDir()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	var err error
	s.ShortTermMem, err = memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.closeProjectKnowledge(); s.ShortTermMem.Close() })
	token, _, err := s.TokenManager.Create("knowledge test", []string{"admin", "desktop:admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerLearningRoutes(mux, s)
	return s, authMiddleware(s, mux), token
}

func learningHTTP(t *testing.T, h http.Handler, token, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestLearningHTTPPermissionsScopeAndRevision(t *testing.T) {
	s, h, token := learningHTTPFixture(t)
	_, scope, _, err := s.resolveKnowledgeProject(t.Context(), "workspace", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	quote := "Keep the portable Go backend"
	hash := sha256.Sum256([]byte(quote))
	l, err := s.ShortTermMem.ObserveLearning(t.Context(), memory.LearningDraft{Statement: quote, Rationale: "Portability", Kind: "decision", Confidence: .99}, scope, memory.LearningSource{Reference: "user:current", Session: "human", Kind: "user", Quote: quote, Hash: hex.EncodeToString(hash[:])}, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if w := learningHTTP(t, h, "", "GET", "/api/learnings", ""); w.Code == 200 {
		t.Fatal("unauthenticated memory leak")
	}
	read, _, _ := s.TokenManager.Create("reader", []string{"desktop:read"}, nil)
	if w := learningHTTP(t, h, read, "GET", "/api/desktop/learnings", ""); w.Code == 200 {
		t.Fatal("nonadmin memory leak")
	}
	if w := learningHTTP(t, h, token, "GET", "/api/desktop/learnings?project=code-studio&id="+l.ID, ""); w.Code != 404 {
		t.Fatal("foreign project", w.Code, w.Body.String())
	}
	if w := learningHTTP(t, h, token, "GET", "/api/learnings?project=workspace&id="+l.ID, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "history") {
		t.Fatal(w.Code, w.Body.String())
	}
	body := `{"id":"` + l.ID + `","revision":1,"action":"archive","reason":"No longer applicable"}`
	s.Cfg.Tools.Memory.ReadOnly = true
	if w := learningHTTP(t, h, token, "POST", "/api/learnings", body); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	s.Cfg.Tools.Memory.ReadOnly = false
	if w := learningHTTP(t, h, token, "POST", "/api/learnings", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := learningHTTP(t, h, token, "POST", "/api/learnings", body); w.Code != 409 {
		t.Fatal("stale revision", w.Code)
	}
	s.Cfg.Auth.Enabled = false
	if w := learningHTTP(t, h, "", "POST", "/api/learnings", body); w.Code != 403 {
		t.Fatal("missing origin accepted", w.Code)
	}
}

func TestLearningProjectResolutionAndGraphRevocation(t *testing.T) {
	s, h, token := learningHTTPFixture(t)
	for _, path := range []string{"../../secret.go", "C:/secret.go", "/etc/secret.go"} {
		if _, _, _, err := s.resolveKnowledgeProject(t.Context(), "code-studio", path); err == nil {
			t.Fatal("path accepted", path)
		}
	}
	if _, _, _, err := s.resolveKnowledgeProject(t.Context(), "C:/", ""); err == nil {
		t.Fatal("arbitrary root accepted")
	}
	if err := os.WriteFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "main.go"), []byte("package demo\nfunc Entry() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, scope, _, err := s.resolveKnowledgeProject(t.Context(), "code-studio", "/workspace/main.go")
	if err != nil || scope.Resource != "main.go" {
		t.Fatal(scope, err)
	}
	w := learningHTTP(t, h, token, "GET", "/api/desktop/code-graph?project=code-studio&operation=outline&file=main.go", "")
	var result codegraph.Result
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Symbols) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	run := agent.RunConfig{Config: s.Cfg}
	s.configureRunKnowledge(&run)
	target, ok := run.LearningTargetScope("virtual_desktop_files", "main.go")
	if !ok || target.Project != p.ID {
		t.Fatal(target, ok)
	}
	target, ok = run.LearningTargetScope("filesystem", "main.go")
	if !ok || target.Project == p.ID {
		t.Fatal("workspace roots conflated", target)
	}
	s.Cfg.VirtualDesktop.WorkspaceDir = t.TempDir()
	if s.authorizeGraphProject(p) {
		t.Fatal("replaced root retained authorization")
	}
	if _, err := s.codeGraphService().Query(t.Context(), p, codegraph.Request{Operation: "status"}); err == nil {
		t.Fatal("revoked graph leaked")
	}
	s.Cfg.CodeGraph.Enabled = false
	if w := learningHTTP(t, h, token, "GET", "/api/code-graph?project=workspace", ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
}
