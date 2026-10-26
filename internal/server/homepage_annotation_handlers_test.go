package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestNormalizeHomepageAnnotationPath(t *testing.T) {
	ok := map[string]string{"": "/", " /about ": "/about", "/blog?page=2#top": "/blog?page=2", "/a%20b": "/a%20b"}
	for in, want := range ok {
		got, err := normalizeHomepageAnnotationPath(in)
		if err != nil || got.String() != want {
			t.Errorf("normalize(%q) = %v, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"about", "//evil.test/x", `/a\b`, "/a\nb", "http://evil.test/", "/" + strings.Repeat("a", 600)} {
		if _, err := normalizeHomepageAnnotationPath(bad); err == nil {
			t.Errorf("normalize(%q) was accepted", bad)
		}
	}
}

func TestHomepageAnnotationDeployURLPrefersTheTargetProvider(t *testing.T) {
	site := tools.HomepageManagedSite{
		LastDeployURL: "https://last.example/",
		DeployTargets: []tools.HomepageManagedDeployTarget{
			{Provider: "local", URL: "http://127.0.0.1:8080"},
			{Provider: "netlify", URL: "https://site.netlify.app"},
			{Provider: "vercel", URL: "https://site.vercel.app"},
		},
	}
	cases := map[string]string{
		"vercel":   "https://site.vercel.app",
		"netlify":  "https://site.netlify.app",
		"here_now": "https://site.netlify.app",
		"remote":   "https://site.netlify.app",
	}
	for target, want := range cases {
		if got := homepageAnnotationDeployURL(target, site); got != want {
			t.Errorf("%s = %q, want %q", target, got, want)
		}
	}
	if got := homepageAnnotationDeployURL("remote", tools.HomepageManagedSite{LastDeployURL: "https://last.example/"}); got != "https://last.example/" {
		t.Errorf("fallback = %q", got)
	}
}

func homepageAnnotationTestServer(t *testing.T, port int) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WebServerPort = port
	return &Server{Cfg: cfg, Logger: slog.Default()}
}

func stubHomepageAnnotationCapture(t *testing.T, fn func(context.Context, tools.HomepageAnnotationCaptureOptions) (*tools.HomepageAnnotationSnapshot, error)) {
	t.Helper()
	orig := homepageAnnotationCapture
	homepageAnnotationCapture = fn
	t.Cleanup(func() { homepageAnnotationCapture = orig })
}

func postHomepageAnnotation(s *Server, method, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handleHomepageAnnotationSnapshot(s)(rec, httptest.NewRequest(method, "/api/homepage/annotation-snapshot", strings.NewReader(body)))
	return rec
}

func TestHomepageAnnotationSnapshotResolvesTheLocalPreview(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	var got tools.HomepageAnnotationCaptureOptions
	stubHomepageAnnotationCapture(t, func(_ context.Context, opts tools.HomepageAnnotationCaptureOptions) (*tools.HomepageAnnotationSnapshot, error) {
		got = opts
		return &tools.HomepageAnnotationSnapshot{URL: opts.URL.String(), Image: "data:image/jpeg;base64,AA=="}, nil
	})

	rec := postHomepageAnnotation(homepageAnnotationTestServer(t, port), http.MethodPost, `{"target":"local","path":"/about?x=1","viewport":{"width":768,"height":900}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	host := fmt.Sprintf("127.0.0.1:%d", port)
	if got.URL.String() != "http://"+host+"/about?x=1" || got.AllowedOrigin == nil || got.AllowedOrigin.Host != host || got.Width != 768 || got.Height != 900 {
		t.Fatalf("capture options = %+v (url %v)", got, got.URL)
	}
	var snap tools.HomepageAnnotationSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil || snap.Path != "/about?x=1" {
		t.Fatalf("response path = %q, err %v", snap.Path, err)
	}
}

func TestHomepageAnnotationSnapshotErrors(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	open := ln.Addr().(*net.TCPAddr).Port
	closedLn, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := closedLn.Addr().(*net.TCPAddr).Port
	_ = closedLn.Close()

	stubHomepageAnnotationCapture(t, func(context.Context, tools.HomepageAnnotationCaptureOptions) (*tools.HomepageAnnotationSnapshot, error) {
		return nil, fmt.Errorf("%w: second capture", tools.ErrHomepageAnnotationBusy)
	})
	cases := []struct {
		name, method, body string
		port, status       int
		code               string
	}{
		{"method", http.MethodGet, ``, open, http.StatusMethodNotAllowed, "method_not_allowed"},
		{"path", http.MethodPost, `{"target":"local","path":"about","viewport":{"width":1280,"height":800}}`, open, http.StatusBadRequest, "invalid_path"},
		{"viewport", http.MethodPost, `{"target":"local","path":"/","viewport":{"width":1000,"height":800}}`, open, http.StatusBadRequest, "invalid_viewport"},
		{"target", http.MethodPost, `{"target":"ftp","path":"/","viewport":{"width":1280,"height":800}}`, open, http.StatusBadRequest, "invalid_target"},
		{"closed preview", http.MethodPost, `{"target":"local","path":"/","viewport":{"width":1280,"height":800}}`, closed, http.StatusNotFound, "preview_unavailable"},
		{"deploy without site", http.MethodPost, `{"target":"vercel","path":"/","viewport":{"width":1280,"height":800}}`, open, http.StatusNotFound, "preview_unavailable"},
		{"busy", http.MethodPost, `{"target":"local","path":"/","viewport":{"width":1280,"height":800}}`, open, http.StatusConflict, "capture_busy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postHomepageAnnotation(homepageAnnotationTestServer(t, tc.port), tc.method, tc.body)
			var body map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if rec.Code != tc.status || body["code"] != tc.code {
				t.Fatalf("status %d code %q, want %d %q: %s", rec.Code, body["code"], tc.status, tc.code, rec.Body.String())
			}
		})
	}
}
