package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHomepageProbeLocalPreview(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
		available               bool
	}{
		{"published document", "<html><body>Published</body></html>", "text/html", 200, true},
		{"empty 404", "", "", 404, false},
		{"empty 200", "", "text/html", 200, false},
		{"truncated HTML", "<html><body>Incomplete", "text/html", 200, false},
		{"not HTML", `{"status":"ok"}`, "application/json", 200, false},
		{"oversized document", strings.Repeat("x", maxHomepageWriteFileSize+1), "text/html", 200, false},
		{"redirect", "", "text/html", 302, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" || r.Method != http.MethodGet {
					t.Errorf("unexpected preview request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			workspace := t.TempDir()
			if err := saveWebserverState(workspace, "selected-site", "."); err != nil {
				t.Fatal(err)
			}
			result := homepageProbeLocalPreview(HomepageConfig{WorkspacePath: workspace, WebServerPort: server.Listener.Addr().(*net.TCPAddr).Port})
			if result.Available != tc.available || result.HTTPStatus != tc.status || result.ProjectDir != "selected-site" {
				t.Fatalf("unexpected preview status: %+v", result)
			}
		})
	}
}

func TestHomepageProbeLocalPreviewHonorsCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := homepageProbeLocalPreview(HomepageConfig{Context: ctx, WebServerPort: server.Listener.Addr().(*net.TCPAddr).Port})
	if result.Available || result.HTTPStatus != 0 || calls.Load() != 0 {
		t.Fatalf("cancelled probe performed I/O: %+v (%d calls)", result, calls.Load())
	}
}
