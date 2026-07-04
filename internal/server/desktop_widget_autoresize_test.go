package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestDesktopWidgetAutoResizeInjectionServesWidgetHTML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Widgets"), 0755); err != nil {
		t.Fatalf("mkdir widget dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Widgets", "weather.html"), []byte("<!doctype html><html><body><main>Weather</main></body></html>"), 0644); err != nil {
		t.Fatalf("write widget: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/weather.html?widget_id=weather", nil)
	rec := httptest.NewRecorder()
	if !serveDesktopWidgetAutoResizeHTML(rec, req, root, nil) {
		t.Fatal("expected widget HTML to be served with auto-resize injection")
	}

	body := rec.Body.String()
	for _, want := range []string{
		desktopWidgetAutoResizeMarker,
		"desktop:widget:resize",
		"ResizeObserver",
		"MutationObserver",
		"doc.scrollHeight",
		"body.scrollHeight",
		"viewportWidth",
		"viewportHeight",
		"contentOverflowsViewport",
		"lastResizePayload",
		"lastResizePostAt",
		"shouldPostResize",
		"Math.abs(next.width-lastResizePayload.width)>2",
		"</script></body>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("injected widget HTML missing %q: %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
}

func TestDesktopWidgetAutoResizeServesAppBackedWidgetHTMLWithWidgetCSP(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	appDir := filepath.Join(root, "Apps", "weather")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app widget dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "widget.html"), []byte("<!doctype html><html><body><main>Weather</main></body></html>"), 0644); err != nil {
		t.Fatalf("write widget: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/weather/widget.html?widget_id=weather", nil)
	rec := httptest.NewRecorder()
	if !serveDesktopWidgetAutoResizeHTML(rec, req, root, nil) {
		t.Fatal("expected app-backed widget HTML to be served with auto-resize injection")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != desktopWidgetWorkspaceCSP {
		t.Fatalf("Content-Security-Policy = %q, want widget CSP %q", got, desktopWidgetWorkspaceCSP)
	}
}

func TestDesktopWidgetAutoResizeRewritesPrinterCameraURLToProxy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Widgets", "printer-camera"), 0755); err != nil {
		t.Fatalf("mkdir widget dir: %v", err)
	}
	rawURL := "http://192.168.6.181:3031/video"
	if err := os.WriteFile(filepath.Join(root, "Widgets", "printer-camera", "index.html"), []byte(`<img src="`+rawURL+`?t=1">`), 0644); err != nil {
		t.Fatalf("write widget: %v", err)
	}
	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "printer-1",
		URL: "ws://192.168.6.181/websocket",
	}}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/printer-camera/index.html?widget_id=printer-camera", nil)
	rec := httptest.NewRecorder()
	if !serveDesktopWidgetAutoResizeHTML(rec, req, root, cfg) {
		t.Fatal("expected widget HTML to be served")
	}
	if strings.Contains(rec.Body.String(), rawURL) {
		t.Fatalf("served widget kept raw camera URL: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `/api/3d-printers/printer-1/camera/stream?t=1`) {
		t.Fatalf("served widget did not contain proxied camera URL: %s", rec.Body.String())
	}
}

func TestDesktopWidgetAutoResizeAddsDesktopTokenToStaticPrinterCameraProxy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Widgets", "printer-camera"), 0755); err != nil {
		t.Fatalf("mkdir widget dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Widgets", "printer-camera", "index.html"), []byte(`<img src="/api/3d-printers/printer-1/camera/stream">`), 0644); err != nil {
		t.Fatalf("write widget: %v", err)
	}
	cfg := &config.Config{}
	cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	pageToken, err := issueDesktopEmbedToken(cfg.Auth.SessionSecret, "Widgets/printer-camera/index.html", time.Now())
	if err != nil {
		t.Fatalf("issueDesktopEmbedToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/printer-camera/index.html?widget_id=printer-camera&desktop_token="+pageToken, nil)
	rec := httptest.NewRecorder()
	if !serveDesktopWidgetAutoResizeHTML(rec, req, root, cfg) {
		t.Fatal("expected widget HTML to be served")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `/api/3d-printers/printer-1/camera/stream?desktop_token=`) {
		t.Fatalf("served widget did not append desktop token to camera proxy: %s", rec.Body.String())
	}
	if strings.Contains(body, pageToken) {
		t.Fatalf("served widget reused generic page token for camera proxy: %s", body)
	}
	streamToken := strings.TrimPrefix(strings.Split(strings.Split(body, `desktop_token=`)[1], `"`)[0], "")
	reqStream := httptest.NewRequest(http.MethodGet, "/api/3d-printers/printer-1/camera/stream?desktop_token="+streamToken, nil)
	if !validDesktopEmbedResourceToken(reqStream, cfg.Auth.SessionSecret, time.Now()) {
		t.Fatalf("camera proxy token is not valid for its exact stream path: %s", body)
	}
}

func TestDesktopWidgetAutoResizeInjectionIsConditionalAndIdempotent(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/weather.html", nil)
	if shouldInjectDesktopWidgetAutoResize(req) {
		t.Fatal("expected widget HTML without widget_id to skip auto-resize injection")
	}
	req = httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/weather.css?widget_id=weather", nil)
	if shouldInjectDesktopWidgetAutoResize(req) {
		t.Fatal("expected non-HTML widget asset to skip auto-resize injection")
	}
	req = httptest.NewRequest(http.MethodPost, "/files/desktop/Widgets/weather.html?widget_id=weather", nil)
	if shouldInjectDesktopWidgetAutoResize(req) {
		t.Fatal("expected non-GET/HEAD request to skip auto-resize injection")
	}

	html := []byte("<html><body><main>Weather</main></body></html>")
	once := injectDesktopWidgetAutoResizeHTML(html)
	twice := injectDesktopWidgetAutoResizeHTML(once)
	if strings.Count(string(twice), desktopWidgetAutoResizeMarker) != 1 {
		t.Fatalf("auto-resize injection should be idempotent, got %d markers", strings.Count(string(twice), desktopWidgetAutoResizeMarker))
	}
}
