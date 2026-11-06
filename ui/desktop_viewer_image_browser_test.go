package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// TestDesktopViewerImagePathContract guards the two undefined identifiers the
// Viewer image type shipped with: `filePath` (never declared in this module)
// and `viewerTitle` (never queried). Both threw a ReferenceError that the
// content catch turned into "Failed to load file: filePath is not defined",
// so no image ever loaded.
func TestDesktopViewerImagePathContract(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/apps/viewer.js")
	if match := regexp.MustCompile(`\bfilePath\b`).FindString(source); match != "" {
		t.Fatalf("viewer.js uses the undeclared identifier %q; the viewer path lives in currentPath", match)
	}
	for _, marker := range []string{
		"data-viewer-title",
		"host.querySelector('[data-viewer-title]')",
		"encodeURIComponent(currentPath) + '&inline=1'",
		"openApp(editApp, { path: currentPath, forceNew: true })",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("viewer.js missing marker %q", marker)
		}
	}
}

// newViewerImageSmokeOrigin serves the Viewer image fixtures: the host page, a
// 2x3 PNG for the inline download endpoint and an empty viewer content body.
func newViewerImageSmokeOrigin(t *testing.T) string {
	t.Helper()

	fixture := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			fixture.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 20), B: 90, A: 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, fixture); err != nil {
		t.Fatalf("encode fixture png: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/desktop/download"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData.Bytes())
		case strings.HasPrefix(r.URL.Path, "/api/desktop/viewer/content"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"content":""}`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"></head><body><div id="host"></div></body></html>`))
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// TestDesktopViewerImageBrowser renders an image in the Viewer window and
// checks that the picture loads, the title carries the pixel size and no
// ReferenceError reaches the content area.
func TestDesktopViewerImageBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newViewerImageSmokeOrigin(t))
	defer page.MustClose()
	page.MustSetViewport(1100, 760, 1, false)
	if err := page.AddScriptTag("", string(mustReadUIFile(t, "js/desktop/apps/viewer.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => {
		window.__viewerErrors = [];
		window.addEventListener('error', event => window.__viewerErrors.push(String(event.message)));
		ViewerApp.render(document.getElementById('host'), 'fixture', {
			path: '/fixtures/sample.png',
			t: key => key,
			esc: value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char])),
			api: async () => ({ content: '' }),
			notify: () => {},
			openApp: () => {},
			iconMarkup: (key, fallback) => '<span>' + fallback + '</span>'
		});
	}`)

	waitForJSBool(t, page, `() => {
		const img = document.querySelector('.vd-viewer-image');
		return !!img && img.complete && img.naturalWidth > 0;
	}`)

	if text := page.MustEval(`() => (document.querySelector('.vd-viewer-error') || {}).textContent || ''`).String(); text != "" {
		t.Fatalf("viewer image reported an error: %s", text)
	}
	src := page.MustEval(`() => document.querySelector('.vd-viewer-image').getAttribute('src')`).String()
	if !strings.Contains(src, "/api/desktop/download?path=") || !strings.Contains(src, "inline=1") {
		t.Fatalf("viewer image src is not the inline download URL: %s", src)
	}
	if !strings.Contains(src, "%2Ffixtures%2Fsample.png") {
		t.Fatalf("viewer image src does not carry the viewer path: %s", src)
	}
	title := page.MustEval(`() => (document.querySelector('[data-viewer-title]') || {}).textContent || ''`).String()
	if !strings.Contains(title, "2×3") {
		t.Fatalf("viewer image title does not carry the pixel size: %q", title)
	}
	if errors := page.MustEval(`() => window.__viewerErrors.join(' | ')`).String(); errors != "" {
		t.Fatalf("viewer image raised uncaught errors: %s", errors)
	}
}
