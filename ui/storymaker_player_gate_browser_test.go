package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Storymaker phase 1 gate (spec "Fallback if workers are unavailable in the sandboxed iframe", contract section 6/8) and the
// render page browser test (contract section 7). Gated by AURAGO_RUN_BROWSER_SMOKE=1, launched like the other browser
// smokes (newSmokeBrowser: the installed Chrome or Edge).
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymaker(PlayerGate|RenderMode)Browser' -count=1 -timeout 10m
//
// The gate serves the app player markup under the exact player CSP of the spec and embeds it in an opaque-origin
// sandbox="allow-scripts" iframe of a parent page that speaks the contract section 8 protocol. It asserts that blob workers
// importScripts the absolute bundle URL, that OffscreenCanvas, Path2D and FontFace work in them, that an ImageBitmap travels
// to the parent, that a frame is not blank, the compile report, and that a code scene cannot fetch.

const storymakerPlayerCSP = "sandbox allow-scripts; default-src 'none'; script-src %[1]s/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src %[1]s/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"

const storymakerRenderCSP = "sandbox allow-scripts; default-src 'none'; script-src %[1]s/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src %[1]s/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"

// storymakerGateScene is a hand-written code-scene bundle in the form of the Go bundler (an IIFE that registers
// self.__smScene). It tries to leave the realm and paints the outcome: red = fetch was blocked, green = importScripts of a
// URL outside the player path was blocked, blue = the worker's FontFace is loaded. White means all three.
const storymakerGateScene = `var __smSceneModule = (function () {
  function scene(api) {
    api.duration(4);
    api.text('gate', 'title');
    var fetchBlocked = 0, importBlocked = 0, probe = '%[1]s/probe';
    try { fetch(probe).then(function () {}, function () { fetchBlocked = 255; }); } catch (e) { fetchBlocked = 255; }
    try { importScripts(probe + '.js'); } catch (e) { importBlocked = 255; }
    api.onFrame(function (t, g) {
      var fontOk = 0;
      self.fonts.forEach(function (f) { if (f.family === 'Storymaker Default' && f.status === 'loaded') fontOk = 255; });
      g.fill('rgb(' + fetchBlocked + ',' + importBlocked + ',' + fontOk + ')');
      g.rect(0, 0, 1920, 1080);
    });
  }
  return { default: scene };
})();
self.__smScene = __smSceneModule.default;
`

const storymakerGateParent = `<!doctype html><meta charset="utf-8"><title>Storymaker gate parent</title>
<iframe id="f" sandbox="allow-scripts" style="width:480px;height:270px;border:0"></iframe>
<canvas id="c" width="160" height="90"></canvas>
<script>
window.__gate = { log: [], errors: [], thumbs: {}, done: false, failure: null };
const iframe = document.getElementById('f');
const gate = window.__gate;
async function getJSON(u) { const r = await fetch(u); if (!r.ok) throw new Error(u + ' ' + r.status); return r.json(); }
let story, catalog, files = {}, fontBuf, bundle;
async function prepare() {
  story = await getJSON('/fixtures/firewall.json');
  catalog = await getJSON('/img/storymaker/catalog.json');
  const paths = [catalog.rig.file].concat(catalog.entries.filter(e => e.file).map(e => e.file));
  for (const p of paths) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
  fontBuf = await (await fetch('/fonts/placeholder.woff2')).arrayBuffer();
  bundle = await (await fetch('/fixtures/gate.bundle.js')).text();
}
function post(msg, transfer) { iframe.contentWindow.postMessage(Object.assign({ sm: 1 }, msg), '*', transfer || []); }
function sample(id, bitmap) {
  const c = document.getElementById('c'), ctx = c.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, c.width, c.height);
  ctx.drawImage(bitmap, 0, 0);
  const px = ctx.getImageData(Math.floor(bitmap.width / 2), Math.floor(bitmap.height / 2), 1, 1).data;
  const seen = new Set();
  for (let y = 0; y < bitmap.height; y += 6) for (let x = 0; x < bitmap.width; x += 6) { const d = ctx.getImageData(x, y, 1, 1).data; seen.add(d[0] + ',' + d[1] + ',' + d[2]); }
  gate.thumbs[id] = { width: bitmap.width, height: bitmap.height, center: [px[0], px[1], px[2], px[3]], distinct: seen.size, isBitmap: bitmap instanceof ImageBitmap };
}
let tries = 0;
window.addEventListener('message', async (ev) => {
  if (ev.source !== iframe.contentWindow) return;
  const m = ev.data;
  if (!m || m.sm !== 1) return;
  gate.log.push(m.type);
  try {
    if (m.type === 'ready') {
      gate.caps = m.caps;
      await prepare();
      post({ type: 'load', story, catalog, audio_info: {}, assets: files, fonts: [{ name: 'default', family: catalog.fonts[0].family, weight: 400, data: fontBuf }], code_bundles: { s3: bundle }, format: { aspect: '16:9', fps: 30, width: 640 } });
    } else if (m.type === 'compiled') {
      gate.report = m.report;
    } else if (m.type === 'time' && m.frame === 0 && !gate.asked) {
      gate.asked = true;
      post({ type: 'thumb', req: 't1', scene_id: 's1', width: 160 });
      post({ type: 'thumb', req: 't3', scene_id: 's3', width: 160 });
    } else if (m.type === 'thumb') {
      sample(m.scene_id, m.bitmap);
      const t3 = gate.thumbs.s3;
      if (m.scene_id === 's3' && t3.center[0] === 0 && t3.center[1] === 0 && t3.center[2] === 0 && tries++ < 30) {
        setTimeout(() => post({ type: 'thumb', req: 't3b' + tries, scene_id: 's3', width: 160 }), 100);
      } else if (gate.thumbs.s1 && gate.thumbs.s3) {
        gate.opaque = (function () { try { iframe.contentWindow.document; return false; } catch (e) { return true; } })();
        gate.done = true;
      }
    } else if (m.type === 'error') {
      gate.errors.push({ code: m.code, message: m.message, scene_id: m.scene_id });
    }
  } catch (e) { gate.failure = String(e && e.stack || e); gate.done = true; }
});
iframe.src = '/player';
</script>`

type storymakerGateServer struct {
	srv       *httptest.Server
	probeHits atomic.Int64
	headers   sync.Map
}

func storymakerFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read %s: %v (run npm run build:ui first)", rel, err)
	}
	return data
}

func newStorymakerGateServer(t *testing.T) *storymakerGateServer {
	t.Helper()
	g := &storymakerGateServer{}
	mini := filepath.Join("..", "internal", "storymaker", "testdata", "catalog-mini")
	mux := http.NewServeMux()
	var origin string
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) { g.probeHits.Add(1); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/probe.js", func(w http.ResponseWriter, r *http.Request) {
		g.probeHits.Add(1)
		w.Header().Set("Content-Type", "text/javascript")
	})
	mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, storymakerGateParent)
	})
	mux.HandleFunc("/player", func(w http.ResponseWriter, r *http.Request) {
		csp := fmt.Sprintf(storymakerPlayerCSP, origin)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="app" data-sm-base="%[1]s"><head><meta charset="utf-8"><link rel="stylesheet" href="%[1]s/css/storymaker-player.css"></head><body><canvas id="sm-stage"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, origin)
	})
	mux.HandleFunc("/js/storymaker/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if name != "host.bundle.js" && name != "player.bundle.js" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/"+name))
	})
	mux.HandleFunc("/css/storymaker-player.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "css/storymaker-player.css"))
	})
	mux.HandleFunc("/fonts/placeholder.woff2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/Geist-Regular.woff2"))
	})
	mux.HandleFunc("/fixtures/firewall.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, "../internal/storymaker/testdata/stories/firewall.json"))
	})
	mux.HandleFunc("/fixtures/gate.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = io.WriteString(w, fmt.Sprintf(storymakerGateScene, origin))
	})
	mux.HandleFunc("/img/storymaker/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/img/storymaker/")
		if strings.Contains(rel, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, filepath.ToSlash(filepath.Join(mini, filepath.FromSlash(rel)))))
	})
	g.srv = httptest.NewServer(mux)
	origin = g.srv.URL
	t.Cleanup(g.srv.Close)
	return g
}

type storymakerGateResult struct {
	Caps   struct{ Workers, Offscreen bool } `json:"caps"`
	Log    []string                          `json:"log"`
	Opaque bool                              `json:"opaque"`
	Report struct {
		FrameCount int `json:"frame_count"`
		FPS        int `json:"fps"`
		Width      int `json:"width"`
		Height     int `json:"height"`
		Scenes     []struct {
			ID         string `json:"id"`
			StartFrame int    `json:"start_frame"`
			EndFrame   int    `json:"end_frame"`
			Kind       string `json:"kind"`
		} `json:"scenes"`
		Issues []struct{ Severity, Code, Path string } `json:"issues"`
	} `json:"report"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		SceneID string `json:"scene_id"`
	} `json:"errors"`
	Thumbs  map[string]storymakerGateThumb `json:"thumbs"`
	Failure string                         `json:"failure"`
}

type storymakerGateThumb struct {
	Width    int   `json:"width"`
	Height   int   `json:"height"`
	Center   []int `json:"center"`
	Distinct int   `json:"distinct"`
	IsBitmap bool  `json:"isBitmap"`
}

func TestStorymakerPlayerGateBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker player gate")
	}
	g := newStorymakerGateServer(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage("").Timeout(120 * time.Second)
	defer page.Close()
	var logMu sync.Mutex
	var browserLog []string
	go page.EachEvent(func(e *proto.LogEntryAdded) {
		logMu.Lock()
		browserLog = append(browserLog, e.Entry.Text+" "+e.Entry.URL)
		logMu.Unlock()
	})()
	if err := (proto.LogEnable{}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustNavigate(g.srv.URL + "/parent")
	if err := page.Wait(rod.Eval(`() => window.__gate && window.__gate.done`)); err != nil {
		t.Fatalf("the gate did not finish: %v\n%s", err, page.MustEval(`() => JSON.stringify(window.__gate)`).Str())
	}
	var res storymakerGateResult
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__gate)`).Str()), &res); err != nil {
		t.Fatal(err)
	}
	t.Logf("gate: caps=%+v log=%v thumbs=%+v errors=%+v frame_count=%d", res.Caps, res.Log, res.Thumbs, res.Errors, res.Report.FrameCount)
	if res.Failure != "" {
		t.Fatalf("parent page failure: %s", res.Failure)
	}
	if !res.Caps.Workers || !res.Caps.Offscreen {
		t.Fatalf("the sandboxed iframe reports caps %+v, want workers and OffscreenCanvas", res.Caps)
	}
	if !res.Opaque {
		t.Error("the player iframe is not opaque-origin: the parent can reach its document")
	}
	if len(res.Log) < 3 || res.Log[0] != "ready" || res.Log[1] != "compiled" {
		t.Errorf("message order %v, want ready, compiled, time ...", res.Log)
	}
	for _, e := range res.Errors {
		t.Errorf("player error %s (scene %q): %s", e.Code, e.SceneID, e.Message)
	}
	// compiled report: scene ranges and frame count (s1 129 frames, s2 114..219, s3 204..324 with api.duration(4))
	if res.Report.FrameCount != 324 || res.Report.FPS != 30 || res.Report.Width != 640 || res.Report.Height != 360 {
		t.Errorf("report = %+v, want 324 frames at 30 fps, 640x360", res.Report)
	}
	want := [][3]int{{0, 129, 0}, {114, 219, 0}, {204, 324, 0}}
	if len(res.Report.Scenes) != 3 {
		t.Fatalf("report scenes = %+v", res.Report.Scenes)
	}
	for i, s := range res.Report.Scenes {
		if s.StartFrame != want[i][0] || s.EndFrame != want[i][1] {
			t.Errorf("scene %s range [%d,%d), want [%d,%d)", s.ID, s.StartFrame, s.EndFrame, want[i][0], want[i][1])
		}
	}
	for _, issue := range res.Report.Issues {
		if issue.Severity == "error" {
			t.Errorf("compile issue %+v", issue)
		}
	}
	// ImageBitmap transfer to the parent and a frame that is not blank
	s1, s3 := res.Thumbs["s1"], res.Thumbs["s3"]
	if !s1.IsBitmap || s1.Width != 160 || s1.Height != 90 {
		t.Errorf("s1 thumbnail = %+v, want a 160x90 ImageBitmap", s1)
	}
	if s1.Distinct < 6 {
		t.Errorf("s1 thumbnail has only %d distinct colours: the frame is blank", s1.Distinct)
	}
	// the code scene paints rgb(r, g, b): r=255 fetch blocked, g=255 importScripts outside /js/storymaker/ blocked, b=255 FontFace loaded in the worker
	if len(s3.Center) != 4 || s3.Center[0] != 255 || s3.Center[1] != 255 || s3.Center[2] != 255 {
		t.Errorf("code scene probe pixel = %v, want [255 255 255 255] (fetch blocked, import blocked, FontFace loaded)", s3.Center)
	}
	// the CSP must have refused the probes (fetch, importScripts) and nothing that the player legitimately loads
	logMu.Lock()
	for _, line := range browserLog {
		t.Logf("browser log: %s", line)
		for _, legit := range []string{"host.bundle.js", "player.bundle.js", "storymaker-player.css"} {
			if strings.Contains(line, legit) {
				t.Errorf("a legitimate player resource was refused or failed: %s", line)
			}
		}
	}
	logMu.Unlock()
	if hits := g.probeHits.Load(); hits != 0 {
		t.Errorf("the code scene reached the server %d times: the CSP did not block it", hits)
	}
}

// ---- render page ---------------------------------------------------------------------------------------------------------

type storymakerIngest struct {
	mu        sync.Mutex
	base      string
	transport string
	story     string
	width     int
	height    int
	report    map[string]any
	frames    []int
	types     []string
	sizes     []int
	badOrder  []string
	restartAt int
	restarted bool
	errors    []map[string]any
	options   int
	origins   map[string]bool
	pngDims   [][2]int
	hashes    []string
	frame5    []byte
	shot      []byte
	saveDir   string
	frameHdr  int // frame requests that carried an X-SM-Frame header (none should)
	framePre  int // preflights of POST /frame (none should)
}

func (in *storymakerIngest) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
	}
}

func TestStorymakerRenderModeBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	for _, transport := range []string{"png", "webp", "raw"} {
		t.Run(transport, func(t *testing.T) {
			in := runStorymakerRenderPage(t, browser, transport, "minimal", 5, 0)
			if len(in.errors) != 0 {
				t.Fatalf("page errors: %v", in.errors)
			}
			if in.report == nil || int(in.report["frame_count"].(float64)) != 42 {
				t.Fatalf("report = %v, want 42 frames", in.report)
			}
			// restart at frame 5: 0..5 once, then 0..41
			wantFrames := append([]int{0, 1, 2, 3, 4, 5}, func() []int {
				all := make([]int, 42)
				for i := range all {
					all[i] = i
				}
				return all
			}()...)
			if fmt.Sprint(in.frames) != fmt.Sprint(wantFrames) {
				t.Fatalf("frames posted = %v, want %v", in.frames, wantFrames)
			}
			// frames are CORS-simple requests (frame number in the query, text/plain): Chrome sends no preflight per frame
			for i, ct := range in.types {
				if ct != "text/plain" {
					t.Fatalf("frame %d content type %q, want text/plain", i, ct)
				}
			}
			if in.frameHdr != 0 || in.framePre != 0 {
				t.Errorf("%d frame requests carried an X-SM-Frame header and %d preflights hit /frame: frames must be CORS-simple", in.frameHdr, in.framePre)
			}
			if transport == "raw" {
				for i, n := range in.sizes {
					if n != in.width*in.height*4 {
						t.Fatalf("raw frame %d has %d bytes, want %d", i, n, in.width*in.height*4)
					}
				}
			}
			if transport == "png" {
				for i, d := range in.pngDims {
					if d != [2]int{in.width, in.height} {
						t.Fatalf("png frame %d is %v, want %dx%d", i, d, in.width, in.height)
					}
				}
			}
			if in.options == 0 {
				t.Error("the opaque-origin page never sent a CORS preflight: the test is not exercising the cross-origin path")
			}
			for origin := range in.origins {
				if origin != "null" {
					t.Errorf("a request carried Origin %q, want null (opaque-origin render page)", origin)
				}
			}
		})
	}
}

// runStorymakerRenderPage serves the render markup (contract section 6) under the render CSP, boot.json and a fake ingest
// (contract section 7) from httptest, opens the page and waits until the ingest has answered done.
func runStorymakerRenderPage(t *testing.T, browser *rod.Browser, transport, storyName string, restartAt, width int) *storymakerIngest {
	t.Helper()
	const prefix = "/0123456789abcdef0123456789abcdef"
	mini := filepath.Join("..", "internal", "storymaker", "testdata", "catalog-mini")
	if width == 0 {
		width = 320
	}
	in := &storymakerIngest{transport: transport, story: storyName, width: width, height: width * 9 / 16, restartAt: restartAt, origins: map[string]bool{}, saveDir: os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")}
	done := make(chan struct{})
	var closeOnce sync.Once
	mux := http.NewServeMux()
	var origin string
	codeBundles := func(base string) map[string]string {
		if storyName == "firewall" {
			return map[string]string{"s3": base + "/scenes/s3.js"}
		}
		return map[string]string{}
	}
	handle := func(path string, fn http.HandlerFunc) {
		mux.HandleFunc(prefix+path, func(w http.ResponseWriter, r *http.Request) {
			in.mu.Lock()
			if o := r.Header.Get("Origin"); o != "" {
				in.origins[o] = true
			}
			if r.Method == http.MethodOptions {
				in.options++
				if strings.HasSuffix(r.URL.Path, "/frame") {
					in.framePre++
				}
			}
			in.mu.Unlock()
			in.cors(w, r)
			if r.Method == http.MethodOptions {
				return
			}
			fn(w, r)
		})
	}
	handle("/", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerRenderCSP, base))
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="render" data-sm-base="%[1]s"><head><meta charset="utf-8"></head><body><canvas id="sm-stage" width="%[2]d" height="%[3]d"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, base, in.width, in.height)
	})
	handle("/boot.json", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": "render", "fps": 30, "width": in.width, "height": in.height, "transport": transport, "frame_limit": 400,
			"story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/",
			"code_bundles": codeBundles(origin + prefix),
			"fonts":        []map[string]any{{"name": "default", "family": "Storymaker Default", "weight": 400, "url": base + "/fonts/default.woff2"}},
			"audio_info":   map[string]any{},
		})
	})
	handle("/story.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, "../internal/storymaker/testdata/stories/"+storyName+".json"))
	})
	handle("/catalog/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix+"/catalog/")
		if strings.Contains(rel, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, filepath.ToSlash(filepath.Join(mini, filepath.FromSlash(rel)))))
	})
	handle("/fonts/default.woff2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/Geist-Regular.woff2"))
	})
	handle("/js/storymaker/host.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/host.bundle.js"))
	})
	handle("/js/storymaker/player.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/player.bundle.js"))
	})
	handle("/scenes/s3.js", func(w http.ResponseWriter, r *http.Request) {
		// bundled the way the Go side does it (contract section 4): esbuild IIFE with globalName __smSceneModule
		src := storymakerFile(t, "../internal/storymaker/testdata/scenes/s3.js")
		res := api.Transform(string(src), api.TransformOptions{Format: api.FormatIIFE, GlobalName: "__smSceneModule", Target: api.ES2020, LegalComments: api.LegalCommentsNone, Loader: api.LoaderJS, Sourcefile: "scenes/s3.js"})
		if len(res.Errors) != 0 {
			http.Error(w, res.Errors[0].Text, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(append(res.Code, []byte("\nself.__smScene=__smSceneModule.default;\n")...))
	})
	handle("/report", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		in.mu.Lock()
		_ = json.Unmarshal(body, &in.report)
		in.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"start":true,"from_frame":0}`)
	})
	handle("/error", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		in.mu.Lock()
		in.errors = append(in.errors, body)
		in.mu.Unlock()
		closeOnce.Do(func() { close(done) })
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		var n int
		fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
		in.mu.Lock()
		defer in.mu.Unlock()
		in.frames = append(in.frames, n)
		if n == 5 && transport == "png" && in.frame5 == nil {
			in.frame5 = append([]byte(nil), body...)
		}
		sum := sha256.Sum256(body)
		in.hashes = append(in.hashes, fmt.Sprintf("%x", sum[:8]))
		if in.saveDir != "" && transport == "png" && n%30 == 0 {
			_ = os.MkdirAll(in.saveDir, 0o755)
			_ = os.WriteFile(filepath.Join(in.saveDir, fmt.Sprintf("storymaker-%s-%03d.png", in.story, n)), body, 0o644)
		}
		in.types = append(in.types, r.Header.Get("Content-Type"))
		if r.Header.Get("X-SM-Frame") != "" {
			in.frameHdr++
		}
		in.sizes = append(in.sizes, len(body))
		if transport == "png" && len(body) > 24 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
			in.pngDims = append(in.pngDims, [2]int{int(binary.BigEndian.Uint32(body[16:20])), int(binary.BigEndian.Uint32(body[20:24]))})
		} else if transport == "png" {
			in.pngDims = append(in.pngDims, [2]int{0, 0})
		}
		if transport == "webp" && !(len(body) > 12 && string(body[:4]) == "RIFF" && string(body[8:12]) == "WEBP") {
			in.badOrder = append(in.badOrder, fmt.Sprintf("frame %d is not a WebP", n))
		}
		w.Header().Set("Content-Type", "application/json")
		total := int(in.report["frame_count"].(float64))
		switch {
		case n == in.restartAt && !in.restarted:
			in.restarted = true
			_, _ = io.WriteString(w, `{"restart":true}`)
		case n == total-1:
			_, _ = io.WriteString(w, `{"ok":true,"done":true}`)
			closeOnce.Do(func() { close(done) })
		default:
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	origin = srv.URL

	page := browser.MustPage(origin + prefix + "/").Timeout(120 * time.Second)
	defer page.Close()
	if transport == "screenshot" {
		// the page only draws on window.__smRenderFrame(n); the driver (here: the test, in production Go over CDP) captures
		page.MustSetViewport(in.width, in.height, 1, false)
		if err := page.Wait(rod.Eval(`() => window.__smStatus && (window.__smStatus.phase === 'ready' || window.__smStatus.phase === 'error')`)); err != nil {
			t.Fatalf("the screenshot page never got ready: %v", err)
		}
		if phase := page.MustEval(`() => window.__smStatus.phase`).Str(); phase != "ready" {
			t.Fatalf("screenshot page status %s", page.MustEval(`() => JSON.stringify(window.__smStatus)`).Str())
		}
		page.MustEval(`() => window.__smRenderFrame(5)`)
		in.shot = page.MustScreenshot()
		return in
	}
	select {
	case <-done:
	case <-time.After(110 * time.Second):
		in.mu.Lock()
		defer in.mu.Unlock()
		t.Fatalf("the render page did not finish: frames=%d errors=%v status=%s", len(in.frames), in.errors, page.MustEval(`() => JSON.stringify(window.__smStatus || null)`).Str())
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.badOrder) != 0 {
		t.Fatalf("bad frames: %v", in.badOrder)
	}
	return in
}

// TestStorymakerFirewallRenderBrowser renders the whole firewall story, code scene included (the real s3.js bundled with
// Go's esbuild), twice, and requires bit-identical frames (the determinism test of the spec). AURAGO_BROWSER_ARTIFACT_DIR
// keeps every 30th frame as a PNG for a look.
func TestStorymakerFirewallRenderBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	transport, width := "png", 640
	if v := os.Getenv("AURAGO_STORYMAKER_TRANSPORT"); v != "" {
		transport = v
	}
	if v := os.Getenv("AURAGO_STORYMAKER_WIDTH"); v != "" {
		fmt.Sscanf(v, "%d", &width)
	}
	started := time.Now()
	first := runStorymakerRenderPage(t, browser, transport, "firewall", -1, width)
	elapsed := time.Since(started)
	t.Logf("%s %dx%d: %d frames in %s = %.1f frames/s (local loopback ingest, no capture proxy)", transport, width, width*9/16, len(first.frames), elapsed.Round(time.Millisecond), float64(len(first.frames))/elapsed.Seconds())
	second := runStorymakerRenderPage(t, browser, transport, "firewall", -1, width)
	for _, in := range []*storymakerIngest{first, second} {
		if len(in.errors) != 0 {
			t.Fatalf("page errors: %v", in.errors)
		}
		if in.report == nil || int(in.report["frame_count"].(float64)) != 354 || len(in.frames) != 354 {
			t.Fatalf("report = %v, frames = %d, want 354", in.report, len(in.frames))
		}
	}
	if fmt.Sprint(first.hashes) != fmt.Sprint(second.hashes) {
		for i := range first.hashes {
			if first.hashes[i] != second.hashes[i] {
				t.Fatalf("frame %d differs between two renders of the same story (%s vs %s)", i, first.hashes[i], second.hashes[i])
			}
		}
	}
	distinct := map[string]bool{}
	for _, h := range first.hashes {
		distinct[h] = true
	}
	if len(distinct) < 200 {
		t.Errorf("only %d distinct frames out of 354: the story barely moves", len(distinct))
	}
}

// TestStorymakerRenderScreenshotBrowser covers the fourth transport: the page draws frame 5 on window.__smRenderFrame(5) and a
// CDP screenshot at device pixel ratio 1 must show the same pixels as the frame that the png transport posts.
func TestStorymakerRenderScreenshotBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	viaPNG := runStorymakerRenderPage(t, browser, "png", "firewall", -1, 640)
	viaShot := runStorymakerRenderPage(t, browser, "screenshot", "firewall", -1, 640)
	want, err := png.Decode(bytes.NewReader(viaPNG.frame5))
	if err != nil {
		t.Fatalf("decode the posted frame: %v", err)
	}
	got, err := png.Decode(bytes.NewReader(viaShot.shot))
	if err != nil {
		t.Fatalf("decode the screenshot: %v", err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("screenshot is %v, the posted frame %v", got.Bounds(), want.Bounds())
	}
	differing, maxDiff := 0, 0
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			wr, wg, wb, _ := want.At(x, y).RGBA()
			gr, gg, gb, _ := got.At(x, y).RGBA()
			if wr != gr || wg != gg || wb != gb {
				differing++
				for _, d := range []int{int(wr>>8) - int(gr>>8), int(wg>>8) - int(gg>>8), int(wb>>8) - int(gb>>8)} {
					if d < 0 {
						d = -d
					}
					maxDiff = max(maxDiff, d)
				}
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
	}
	t.Logf("screenshot vs posted png frame 5: %d of %d pixels differ", differing, b.Dx()*b.Dy())
	if differing > 0 {
		t.Logf("max channel difference %d, bounding box %d,%d-%d,%d", maxDiff, minX, minY, maxX, maxY)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "storymaker-shot.png"), viaShot.shot, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "storymaker-frame5.png"), viaPNG.frame5, 0o644)
	}
	if differing != 0 {
		t.Errorf("%d pixels differ between the CDP screenshot and the posted frame", differing)
	}
}
