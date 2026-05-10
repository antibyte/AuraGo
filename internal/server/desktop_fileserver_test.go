package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/space-invaders/index.html?desktop_token=test", nil)
	rec := httptest.NewRecorder()

	if !serveDesktopExactIndexFile(rec, req, desktopDir) {
		t.Fatal("expected exact desktop index file to be served before FileServer redirect")
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
	} else if gameIndex := strings.Index(rec.Body.String(), `src="game.js"`); gameIndex < 0 || bridgeIndex > gameIndex {
		t.Fatalf("app key bridge must be injected before game scripts: bridge=%d game=%d body=%q", bridgeIndex, gameIndex, rec.Body.String())
	}
}
