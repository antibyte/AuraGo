package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestHomepageAnnotationURLKeyMatchesTheSameDeployedPage(t *testing.T) {
	const registered = "https://site.example/blog"
	same := []string{
		"https://site.example/blog",
		"https://site.example/blog/",
		"https://site.example:443/blog",
		"HTTPS://Site.Example/blog",
		"https://site.example/blog?utm=1#top",
		" https://site.example/blog ",
	}
	for _, raw := range same {
		if homepageAnnotationURLKey(raw) != homepageAnnotationURLKey(registered) {
			t.Errorf("%q should match %q (%q vs %q)", raw, registered, homepageAnnotationURLKey(raw), homepageAnnotationURLKey(registered))
		}
	}
	if homepageAnnotationURLKey("http://site.example:80/") != homepageAnnotationURLKey("http://site.example") {
		t.Error("the default http port must be dropped")
	}
	if homepageAnnotationURLKey("http://site.example:8080/") == homepageAnnotationURLKey("http://site.example/") {
		t.Error("a non-default port is a different origin")
	}
	different := []string{
		"https://site.example/",
		"https://site.example/blog/post",
		"http://site.example/blog",
		"https://other.example/blog",
		"https://site.example:8443/blog",
		"https://sub.site.example/blog",
	}
	for _, raw := range different {
		if homepageAnnotationURLKey(raw) == homepageAnnotationURLKey(registered) {
			t.Errorf("%q must not match %q", raw, registered)
		}
	}
	unusable := []string{"", "   ", "ftp://site.example/", "file:///etc/passwd", "javascript:alert(1)", "//site.example/blog", "/blog", "site.example/blog", "https://", "https://user:pw@site.example/blog", "%zz"}
	for _, raw := range unusable {
		if key := homepageAnnotationURLKey(raw); key != "" {
			t.Errorf("%q produced the key %q", raw, key)
		}
	}
}

// homepageAnnotationRegistryServer registers two sites with deployed URLs.
func homepageAnnotationRegistryServer(t *testing.T) (*Server, int64) {
	t.Helper()
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init registry: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	siteA, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "SiteA", ProjectDir: "site-a", Framework: "astro"})
	if err != nil {
		t.Fatal(err)
	}
	siteB, _, err := tools.RegisterProject(db, tools.HomepageProject{Name: "SiteB", ProjectDir: "site-b", Framework: "astro"})
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range []tools.HomepageDeploymentRecord{
		{ProjectID: siteA, Provider: "vercel", URL: "https://site-a.vercel.app/?v=1"},
		{ProjectID: siteB, Provider: "netlify", URL: "https://Site-B.netlify.app/shop/"},
		{ProjectID: siteB, Provider: "ssh", URL: "https://b.example.org:8443/"},
	} {
		if err := tools.RecordHomepageDeployment(db, rec); err != nil {
			t.Fatalf("record deployment %s: %v", rec.URL, err)
		}
	}
	s := homepageAnnotationTestServer(t, 0)
	s.HomepageRegistryDB = db
	return s, siteA
}

func TestHomepageAnnotationSnapshotCapturesTheRegisteredPreviewURL(t *testing.T) {
	s, siteA := homepageAnnotationRegistryServer(t)
	var got tools.HomepageAnnotationCaptureOptions
	stubHomepageAnnotationCapture(t, func(_ context.Context, opts tools.HomepageAnnotationCaptureOptions) (*tools.HomepageAnnotationSnapshot, error) {
		got = opts
		return &tools.HomepageAnnotationSnapshot{URL: opts.URL.String(), Image: "data:image/jpeg;base64,AA=="}, nil
	})

	cases := []struct {
		name, target, preview, path, want string
	}{
		{"vercel exact", "vercel", "https://site-a.vercel.app/", "/pricing?x=1", "https://site-a.vercel.app/pricing?x=1"},
		{"vercel client query and case", "vercel", "HTTPS://SITE-A.vercel.app/?utm=7#hero", "/", "https://site-a.vercel.app/"},
		// The preview belongs to another site than the request names.
		{"another site with a path", "netlify", "https://site-b.netlify.app/shop", "/cart", "https://Site-B.netlify.app/shop/cart"},
		{"remote with a custom port", "remote", "https://b.example.org:8443", "/about", "https://b.example.org:8443/about"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got = tools.HomepageAnnotationCaptureOptions{}
			body := fmt.Sprintf(`{"site_id":%d,"target":%q,"preview_url":%q,"path":%q,"viewport":{"width":1280,"height":800}}`, siteA, tc.target, tc.preview, tc.path)
			rec := postHomepageAnnotation(s, http.MethodPost, body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			if got.URL == nil || got.URL.String() != tc.want {
				t.Fatalf("capture url = %v, want %q", got.URL, tc.want)
			}
			if got.AllowedOrigin != nil {
				t.Fatalf("a deployed capture must stay behind the SSRF guard, got origin %v", got.AllowedOrigin)
			}
		})
	}
}

func TestHomepageAnnotationSnapshotRefusesUnregisteredPreviewURLs(t *testing.T) {
	s, siteA := homepageAnnotationRegistryServer(t)
	called := false
	stubHomepageAnnotationCapture(t, func(context.Context, tools.HomepageAnnotationCaptureOptions) (*tools.HomepageAnnotationSnapshot, error) {
		called = true
		return &tools.HomepageAnnotationSnapshot{}, nil
	})
	for name, preview := range map[string]string{
		"missing":              "",
		"unknown host":         "https://evil.example/",
		"registered host path": "https://site-a.vercel.app/admin",
		"other scheme":         "http://site-a.vercel.app/",
		"loopback":             "http://127.0.0.1:8080/",
		"metadata service":     "http://169.254.169.254/latest/meta-data/",
		"not http":             "ftp://site-a.vercel.app/",
		"relative":             "/pricing",
		"credentials":          "https://user:pw@site-a.vercel.app/",
	} {
		t.Run(name, func(t *testing.T) {
			body := fmt.Sprintf(`{"site_id":%d,"target":"vercel","preview_url":%q,"path":"/","viewport":{"width":1280,"height":800}}`, siteA, preview)
			rec := postHomepageAnnotation(s, http.MethodPost, body)
			var out map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			if rec.Code != http.StatusNotFound || out["code"] != "preview_unavailable" {
				t.Fatalf("status %d code %q, want 404 preview_unavailable: %s", rec.Code, out["code"], rec.Body.String())
			}
		})
	}
	if called {
		t.Fatal("an unregistered preview URL must never reach the capture engine")
	}
}

func TestHomepageAnnotationSnapshotLocalIgnoresThePreviewURL(t *testing.T) {
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
	rec := postHomepageAnnotation(homepageAnnotationTestServer(t, port), http.MethodPost, `{"target":"local","preview_url":"http://169.254.169.254/","path":"/","viewport":{"width":1280,"height":800}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d/", port); got.URL.String() != want {
		t.Fatalf("capture url = %v, want %q", got.URL, want)
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
		{"deploy without preview url", http.MethodPost, `{"target":"vercel","path":"/","viewport":{"width":1280,"height":800}}`, open, http.StatusNotFound, "preview_unavailable"},
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
