package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

type fakeCodeStudioPreviewSource struct {
	files map[string]string
	sizes map[string]int64
	calls []string
}

func (f *fakeCodeStudioPreviewSource) Probe(_ context.Context, vpath string) (codeStudioFileProbe, error) {
	f.calls = append(f.calls, "probe "+vpath)
	content, ok := f.files[vpath]
	if !ok {
		return codeStudioFileProbe{}, codeStudioProbeRejection{message: "not a regular file"}
	}
	size := int64(len(content))
	if override, ok := f.sizes[vpath]; ok {
		size = override
	}
	return codeStudioFileProbe{Path: vpath, Size: size, Modified: time.Unix(1710000000, 0)}, nil
}

func (f *fakeCodeStudioPreviewSource) Read(_ context.Context, vpath string, max int64) ([]byte, error) {
	f.calls = append(f.calls, "read "+vpath)
	content, ok := f.files[vpath]
	if !ok {
		return nil, errors.New("missing")
	}
	if int64(len(content)) > max {
		return nil, errors.New("file exceeds size limit")
	}
	return []byte(content), nil
}

func newTestCodeStudioPreview(t *testing.T, files map[string]string) (*codeStudioPreviewHandler, *fakeCodeStudioPreviewSource, string) {
	t.Helper()
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.SessionSecret = "preview-test-secret"
	if files == nil {
		files = map[string]string{}
	}
	source := &fakeCodeStudioPreviewSource{files: files, sizes: map[string]int64{}}
	p := &codeStudioPreviewHandler{
		studio:   codeStudioHandlers{server: s},
		versions: codeStudioPreviewVersionsFor(s),
		source: func(_ context.Context, workspace string) (codeStudioPreviewSource, error) {
			if workspace != "local" {
				return nil, errCodeStudioPreviewWorkspaceNotFound
			}
			return source, nil
		},
		now:         time.Now,
		pollTimeout: 2 * time.Second,
	}
	return p, source, testCodeStudioPreviewToken(t, s, "local")
}

func testCodeStudioPreviewToken(t *testing.T, s *Server, workspace string) string {
	t.Helper()
	key, err := s.codeStudioPreviewSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	token, _, err := issueCodeStudioPreviewToken(key, workspace, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func serveCodeStudioPreview(p *codeStudioPreviewHandler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	p.handleServe(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestCodeStudioPreviewServesHTMLWithSandboxHeaders(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/index.html": "<!doctype html><html><head><title>x</title></head><body><h1>Hi</h1></BODY></html>",
	})
	p.studio.notifyWorkspaceChange("local", "/workspace/other.css")
	handler := securityHeadersMiddleware(http.HandlerFunc(p.handleServe), false, false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, codeStudioPreviewPrefix+token+"/index.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	for name, want := range map[string]string{
		"Content-Security-Policy":     codeStudioPreviewCSP,
		"X-Content-Type-Options":      "nosniff",
		"Referrer-Policy":             "no-referrer",
		"Cache-Control":               "no-store",
		"Access-Control-Allow-Origin": "*",
		"Content-Type":                "text/html; charset=utf-8",
		"X-Frame-Options":             "",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if strings.Contains(rec.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("the preview sandbox must not allow the same origin")
	}
	tag := `<script src="` + codeStudioPreviewPrefix + token + `/__aurago/live.js" data-aurago-version="1"></script>`
	if !strings.Contains(rec.Body.String(), "<head>"+tag+"<title>") {
		t.Fatalf("live script not injected right after <head>: %s", rec.Body.String())
	}
}

func TestCodeStudioPreviewInjectsLiveScript(t *testing.T) {
	const base = "/code-studio-preview/t/"
	htmlTag := `<script src="/code-studio-preview/t/__aurago/live.js" data-aurago-version="3"></script>`
	svgTag := `<script xmlns="http://www.w3.org/2000/svg" href="/code-studio-preview/t/__aurago/live.js" data-aurago-version="3"></script>`
	for _, tc := range []struct{ name, contentType, in, want string }{
		{"head with attributes", "text/html; charset=utf-8", `<HTML><HEAD lang="en"><title>x</title></HEAD></HTML>`, `<HTML><HEAD lang="en">` + htmlTag + `<title>x</title></HEAD></HTML>`},
		{"header is not head", "text/html; charset=utf-8", `<body><header>x</header></Body>`, `<body><header>x</header>` + htmlTag + `</Body>`},
		{"fragment", "text/html; charset=utf-8", `<p>plain</p>`, `<p>plain</p>` + htmlTag},
		{"svg", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`, `<svg xmlns="http://www.w3.org/2000/svg"><rect/>` + svgTag + `</svg>`},
		{"css untouched", "text/css; charset=utf-8", `body{}`, `body{}`},
	} {
		if got := string(injectCodeStudioPreviewLiveScript([]byte(tc.in), tc.contentType, base, 3)); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestCodeStudioPreviewDirectoryIndexAndRedirects(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/index.html":      "<body>root</body>",
		"/workspace/docs/index.html": "<body>docs</body>",
	})
	base := codeStudioPreviewPrefix + token
	for _, tc := range []struct {
		target   string
		status   int
		location string
		body     string
	}{
		{base, http.StatusTemporaryRedirect, base + "/", ""},
		{base + "/", http.StatusOK, "", "root"},
		{base + "/docs?x=1", http.StatusTemporaryRedirect, base + "/docs/?x=1", ""},
		{base + "/docs/", http.StatusOK, "", "docs"},
	} {
		rec := serveCodeStudioPreview(p, http.MethodGet, tc.target)
		if rec.Code != tc.status || rec.Header().Get("Location") != tc.location || !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s: status %d location %q body %q", tc.target, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
	}
}

func TestCodeStudioPreviewConfinesPaths(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{"/workspace/index.html": "<body></body>"})
	base := codeStudioPreviewPrefix + token
	for _, target := range []string{base + "/../secret.txt", base + "/a/%2e%2e/%2e%2e/etc/passwd", base + "/a%5Cb.html", base + "/a%00.html"} {
		if rec := serveCodeStudioPreview(p, http.MethodGet, target); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
	if len(source.calls) != 0 {
		t.Fatalf("source touched for rejected paths: %v", source.calls)
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/other"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown internal path status = %d, want 404", rec.Code)
	}
	serveCodeStudioPreview(p, http.MethodGet, base+"/index.html")
	for _, call := range source.calls {
		if !strings.Contains(call, " /workspace/") {
			t.Fatalf("source call outside the workspace: %q", call)
		}
	}
}

func TestCodeStudioPreviewRejectsBadTokensAndMethods(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{"/workspace/index.html": "<body></body>"})
	last := "A"
	if strings.HasSuffix(token, "A") {
		last = "B"
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+token[:len(token)-1]+last+"/index.html"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered token status = %d, want 401", rec.Code)
	}
	p.now = func() time.Time { return time.Now().Add(13 * time.Hour) }
	rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+token+"/index.html")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("expired token = %d %s, want 401 expired page", rec.Code, rec.Body.String())
	}
	p.now = time.Now
	rec = serveCodeStudioPreview(p, http.MethodPost, codeStudioPreviewPrefix+token+"/index.html")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d Allow %q, want 405 GET, HEAD", rec.Code, rec.Header().Get("Allow"))
	}
	gone := testCodeStudioPreviewToken(t, p.studio.server, "ssh-gone")
	if rec := serveCodeStudioPreview(p, http.MethodGet, codeStudioPreviewPrefix+gone+"/index.html"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Workspace not found") {
		t.Fatalf("unknown workspace = %d %s, want 404", rec.Code, rec.Body.String())
	}
	if len(source.calls) != 0 {
		t.Fatalf("source touched without a valid local token: %v", source.calls)
	}
}

func TestCodeStudioPreviewMissingTooLargeAndContentTypes(t *testing.T) {
	p, source, token := newTestCodeStudioPreview(t, map[string]string{
		"/workspace/big.html":  "<body></body>",
		"/workspace/style.css": "body{color:red}",
		"/workspace/app.mjs":   "export const x = 1;",
		"/workspace/logo.svg":  `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"/workspace/data.bin":  "\x89PNG\r\n\x1a\n0000",
	})
	source.sizes["/workspace/big.html"] = codeStudioPreviewMaxBytes + 1
	base := codeStudioPreviewPrefix + token
	rec := serveCodeStudioPreview(p, http.MethodGet, base+"/%3Cimg%20src=x%3E.html")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "&lt;img src=x&gt;.html") || strings.Contains(rec.Body.String(), "<img src=x>") {
		t.Fatalf("missing file page = %d %s, want an escaped 404", rec.Code, rec.Body.String())
	}
	if rec := serveCodeStudioPreview(p, http.MethodHead, base+"/missing.html"); rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("HEAD missing = %d with %d body bytes, want 404 without body", rec.Code, rec.Body.Len())
	}
	if rec := serveCodeStudioPreview(p, http.MethodGet, base+"/big.html"); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("big file status = %d, want 413", rec.Code)
	}
	for _, call := range source.calls {
		if call == "read /workspace/big.html" {
			t.Fatal("an oversized file was read")
		}
	}
	handler := securityHeadersMiddleware(http.HandlerFunc(p.handleServe), false, false)
	for path, want := range map[string]string{
		"/style.css": "text/css; charset=utf-8",
		"/app.mjs":   "text/javascript; charset=utf-8",
		"/logo.svg":  "image/svg+xml",
		"/data.bin":  "image/png",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, base+path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != want || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %q cache %q, want 200 %q no-store", path, rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), want)
		}
		if path == "/logo.svg" && !strings.Contains(rec.Body.String(), `href="`+base+`/__aurago/live.js"`) {
			t.Errorf("svg without live script: %s", rec.Body.String())
		}
	}
}

func TestCodeStudioPreviewVersionAndLiveScriptEndpoints(t *testing.T) {
	p, _, token := newTestCodeStudioPreview(t, nil)
	base := codeStudioPreviewPrefix + token
	rec := serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/live.js")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/javascript; charset=utf-8" || !strings.Contains(rec.Body.String(), "auragoPreview") {
		t.Fatalf("live.js = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.studio.notifyWorkspaceChange("local", "/workspace/styles.css")
	}()
	start := time.Now()
	rec = serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/version?since=0")
	var body struct {
		Version uint64   `json:"version"`
		Paths   []string `json:"paths"`
		Full    bool     `json:"full"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Version != 1 || len(body.Paths) != 1 || body.Paths[0] != "styles.css" || body.Full {
		t.Fatalf("version = %s (%v), want 1 [styles.css] false", rec.Body.String(), err)
	}
	if time.Since(start) > time.Second || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("long poll took %v, ACAO %q", time.Since(start), rec.Header().Get("Access-Control-Allow-Origin"))
	}
	p.pollTimeout = 50 * time.Millisecond
	rec = serveCodeStudioPreview(p, http.MethodGet, base+"/__aurago/version?since=1")
	body.Paths = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Version != 1 || body.Paths == nil || len(body.Paths) != 0 {
		t.Fatalf("idle poll = %s, want version 1 with an empty path list", rec.Body.String())
	}
}
