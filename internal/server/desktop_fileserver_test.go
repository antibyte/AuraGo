package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestServeDesktopExactIndexFileInjectsEmbedTokenIntoSiblingAssets(t *testing.T) {
	t.Parallel()

	desktopDir := t.TempDir()
	appDir := filepath.Join(desktopDir, "Apps", "nasscad")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte("<!doctype html><html><head><script src=\"three.js\"></script></head><body>OK</body></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	token, err := issueDesktopEmbedToken("0123456789abcdef0123456789abcdef", "Apps/nasscad/index.html", time.Now())
	if err != nil {
		t.Fatalf("issueDesktopEmbedToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/nasscad/index.html?desktop_token="+token, nil)
	req.Host = "aurago.example.test"
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	if !serveDesktopExactIndexFile(rec, req, desktopDir, nil) {
		t.Fatal("expected exact desktop index file to be served")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `src="three.js?desktop_v=`) {
		t.Fatalf("three.js URL was not cache-busted: %q", body)
	}
	if !strings.Contains(body, "desktop_token=") {
		t.Fatalf("three.js URL did not inherit desktop embed token: %q", body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src https://aurago.example.test") {
		t.Fatalf("app CSP did not include request origin for sandboxed sibling scripts: %q", csp)
	}
}

func TestServeDesktopExactIndexFileAvoidsFileServerRedirect(t *testing.T) {
	t.Parallel()

	desktopDir := t.TempDir()
	appDir := filepath.Join(desktopDir, "Apps", "space-invaders")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte("<!doctype html><html><head><title>Space</title><script src=\"game.js\"></script></head><body><main>Game</main></body></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/space-invaders/index.html", nil)
	req.Host = ""
	rec := httptest.NewRecorder()

	if !serveDesktopExactIndexFile(rec, req, desktopDir, nil) {
		t.Fatal("expected exact desktop index file to be served before FileServer redirect")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != desktopAppWorkspaceCSP {
		t.Fatalf("Content-Security-Policy = %q, want app CSP %q", got, desktopAppWorkspaceCSP)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("Location = %q, want no redirect", location)
	}
	if !strings.Contains(rec.Body.String(), "<title>Space</title>") {
		t.Fatalf("body did not contain app index HTML: %q", rec.Body.String())
	}
	if cacheControl := rec.Header().Get("Cache-Control"); !strings.Contains(cacheControl, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store for generated app HTML", cacheControl)
	}
	if !strings.Contains(rec.Body.String(), `src="game.js?desktop_v=`) {
		t.Fatalf("app resource URL was not cache-busted: %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "desktop_token=") {
		t.Fatalf("app resource URL should not inject desktop_token when request lacks one: %q", rec.Body.String())
	}
	for _, want := range []string{
		desktopAppKeyBridgeMarker,
		"aurago.desktop.key-event",
		"function installStorageShim(name)",
		"localStorage",
		"keyboardHandlers",
		"originalWindowAddEventListener",
		"handler.call(window,eventObject)",
		"code=String(msg.code||'')",
		"new KeyboardEvent",
		"window.dispatchEvent(new KeyboardEvent(eventType,init))",
		"document.dispatchEvent(new KeyboardEvent(eventType,init))",
		"querySelectorAll('canvas,[tabindex]')",
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body did not contain app key bridge marker %q: %q", want, rec.Body.String())
		}
	}
	if bridgeIndex := strings.Index(rec.Body.String(), desktopAppKeyBridgeMarker); bridgeIndex < 0 {
		t.Fatalf("body did not contain app key bridge marker: %q", rec.Body.String())
	} else if gameIndex := strings.Index(rec.Body.String(), `src="game.js?desktop_v=`); gameIndex < 0 || bridgeIndex > gameIndex {
		t.Fatalf("app key bridge must be injected before game scripts: bridge=%d game=%d body=%q", bridgeIndex, gameIndex, rec.Body.String())
	}
}

func TestServeDesktopExactIndexFileRewritesPrinterCameraURLToProxy(t *testing.T) {
	t.Parallel()

	desktopDir := t.TempDir()
	appDir := filepath.Join(desktopDir, "Apps", "printer-camera")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	rawURL := "http://192.168.6.181:3031/video"
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte(`<video src="`+rawURL+`"></video>`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "printer-1",
		URL: "ws://192.168.6.181/websocket",
	}}

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/printer-camera/index.html?desktop_token=test", nil)
	rec := httptest.NewRecorder()
	if !serveDesktopExactIndexFile(rec, req, desktopDir, cfg) {
		t.Fatal("expected exact desktop index file to be served")
	}
	if strings.Contains(rec.Body.String(), rawURL) {
		t.Fatalf("served app kept raw camera URL: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `/api/3d-printers/printer-1/camera/stream`) {
		t.Fatalf("served app did not contain proxied camera URL: %s", rec.Body.String())
	}
}

func TestSetDesktopFileResponseHeadersForcesAttachmentForUntrustedExtensions(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	setDesktopFileResponseHeaders(rec, httptest.NewRequest(http.MethodGet, "/files/desktop/Notes/readme.txt", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "readme.txt") {
		t.Fatalf("Content-Disposition = %q, want attachment filename", got)
	}
}

func TestSetDesktopFileResponseHeadersAllowsAppAssetsInline(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/files/desktop/Apps/game/game.js",
		"/files/desktop/Apps/game/style.css",
		"/files/desktop/Apps/game/sprite.png",
	} {
		rec := httptest.NewRecorder()
		setDesktopFileResponseHeaders(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Content-Disposition"); got != "" {
			t.Fatalf("Content-Disposition for %s = %q, want empty", path, got)
		}
	}
}
