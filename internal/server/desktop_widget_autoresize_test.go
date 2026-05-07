package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !serveDesktopWidgetAutoResizeHTML(rec, req, root) {
		t.Fatal("expected widget HTML to be served with auto-resize injection")
	}

	body := rec.Body.String()
	for _, want := range []string{
		desktopWidgetAutoResizeMarker,
		"desktop:widget:resize",
		"ResizeObserver",
		"MutationObserver",
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
