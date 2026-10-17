package ui

import (
	"encoding/base64"
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
	"aurago/internal/videostudio/goldenframe"

	"github.com/go-rod/rod"
)

// videoStudioEngineScripts returns the 'video-studio' scripts of the module loader up to and including
// video-studio-preview.js, in load order (index §5). The engine page follows every later insertion by itself.
func videoStudioEngineScripts(t *testing.T) []string {
	t.Helper()
	loader, err := os.ReadFile(filepath.Join("js", "desktop", "core", "module-loader.js"))
	if err != nil {
		t.Fatalf("module loader: %v", err)
	}
	entry := regexp.MustCompile(`'video-studio':\s*\{[^}]*?scripts:\s*\[([^\]]*)\]`).FindSubmatch(loader)
	if entry == nil {
		t.Fatal("module loader has no 'video-studio' scripts list")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'(/js/desktop/apps/video-studio-[a-z0-9-]+\.js)'`).FindAllSubmatch(entry[1], -1) {
		scripts = append(scripts, string(match[1]))
		if strings.HasSuffix(string(match[1]), "/video-studio-preview.js") {
			return scripts
		}
	}
	t.Fatal("module loader does not load video-studio-preview.js")
	return nil
}

const videoStudioEngineHead = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Video Studio engine</title>
<style>html,body{margin:0;background:#000}#stage canvas{display:block}</style></head><body><div id="stage"></div>
<script>window.vsEngineErrors=[];addEventListener('error',e=>vsEngineErrors.push(String(e.message)));addEventListener('unhandledrejection',e=>vsEngineErrors.push(String(e.reason)));</script>
`

// videoStudioGoldenJS defines the test-only window.vsGolden on the engine page; it is never shipped.
// render(project, frame, painter) mounts the shipped preview on a fresh canvas in #stage with {painter, mediaURL}, seeks,
// awaits settle(15000) and captures the frame as base64 RGBA. Media resolve through vsGolden.files (asset id -> file
// name served under /media/); an asset without an entry keeps its media_url.
const videoStudioGoldenJS = `(function () {
  'use strict';
  const files = {};
  function base64(bytes) {
    let binary = '';
    for (let i = 0; i < bytes.length; i += 0x8000) binary += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
    return btoa(binary);
  }
  function mediaURL(asset) {
    const name = asset && files[asset.id];
    return name ? '/media/' + encodeURIComponent(name) : String(asset && asset.media_url || '');
  }
  async function render(project, frame, painter) {
    const stage = document.getElementById('stage');
    const canvas = document.createElement('canvas');
    canvas.setAttribute('data-preview', '');
    canvas.width = Math.max(2, Number(project.width) || 2);
    canvas.height = Math.max(2, Number(project.height) || 2);
    stage.replaceChildren(canvas);
    let shown = Math.max(0, Math.round(Number(frame) || 0));
    const preview = window.VideoStudioPreview.mount(canvas, () => project, () => shown, value => { shown = value; }, () => {}, () => 1, { painter: painter || '2d', mediaURL });
    try {
      preview.seek(shown);
      const settled = await preview.settle(15000);
      const shot = preview.capture();
      return { painter: preview.painterKind(), width: shot.width, height: shot.height, rgba: base64(shot.data), settled: !!settled };
    } finally {
      preview.dispose();
      stage.replaceChildren();
    }
  }
  window.vsGolden = { files, mediaURL, render };
})();`

// videoStudioEngineLookPath is the server's built-in look endpoint (index §4.20), which the engine page mirrors.
const videoStudioEngineLookPath = "/api/desktop/video-studio/luts/builtin/"

// newVideoStudioEngineServer serves:
//   - the shipped UI files;
//   - mediaDir under /media/ (when set);
//   - the built-in looks under /api/desktop/video-studio/luts/builtin/<id>.cube from videostudio.BuiltinLookCube;
//   - /engine, a page that loads only the preview engine (module-loader order up to video-studio-preview.js) plus
//     window.vsGolden.
func newVideoStudioEngineServer(t *testing.T, mediaDir string) *httptest.Server {
	t.Helper()
	var page strings.Builder
	page.WriteString(videoStudioEngineHead)
	for _, script := range videoStudioEngineScripts(t) {
		fmt.Fprintf(&page, "<script src=\"%s\"></script>\n", script)
	}
	page.WriteString("<script>" + videoStudioGoldenJS + "</script></body></html>")
	html := page.String()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	if mediaDir != "" {
		mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(mediaDir))))
	}
	// GL previews fetch builtin:<id> refs here, with the bytes the server's look endpoint serves.
	mux.HandleFunc(videoStudioEngineLookPath, func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, videoStudioEngineLookPath), ".cube")
		data, found := videostudio.BuiltinLookCube(id)
		if !ok || !found {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/engine", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(html))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// openVideoStudioEngine opens /engine and waits until the engine and window.vsGolden are ready without script errors.
func openVideoStudioEngine(t *testing.T, browser *rod.Browser, server *httptest.Server) *rod.Page {
	t.Helper()
	page := browser.MustPage(server.URL + "/engine")
	t.Cleanup(func() { _ = page.Close() })
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!window.vsGolden && !!window.VideoStudioPreview`)
	if errs := page.MustEval(`() => JSON.stringify(window.vsEngineErrors)`).Str(); errs != "[]" {
		t.Fatalf("engine page errors: %s", errs)
	}
	return page
}

// engineFrame is one frame rendered by window.vsGolden.render.
type engineFrame struct {
	Painter string
	Settled bool
	Image   *image.RGBA
}

// renderEngineFrame runs window.vsGolden.render(project, frame, painter) and decodes the RGBA capture.
func renderEngineFrame(t *testing.T, page *rod.Page, project any, frame int, painter string) engineFrame {
	t.Helper()
	result, err := page.Timeout(60*time.Second).Eval(`(project, frame, painter) => window.vsGolden.render(project, frame, painter)`, project, frame, painter)
	if err != nil {
		t.Fatalf("vsGolden.render(frame %d, %s): %v", frame, painter, err)
	}
	value := result.Value
	raw, err := base64.StdEncoding.DecodeString(value.Get("rgba").Str())
	if err != nil {
		t.Fatalf("decode engine frame: %v", err)
	}
	img, err := goldenframe.DecodeRGBA(value.Get("width").Int(), value.Get("height").Int(), raw)
	if err != nil {
		t.Fatalf("engine frame pixels: %v", err)
	}
	return engineFrame{Painter: value.Get("painter").Str(), Settled: value.Get("settled").Bool(), Image: img}
}
