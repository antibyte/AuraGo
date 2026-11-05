package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// storymakerParityParent is the parent page of the parity test: it embeds the app player host exactly as the
// desktop app does (sandbox="allow-scripts" only), loads the story with the catalog, the four fonts and the code
// bundle, waits for the first frame and then asks for full-size frames (thumb with width = the export width), whose
// decoded pixels it hashes.
const storymakerParityParent = `<!doctype html><meta charset="utf-8"><title>parity</title>
<iframe id="f" sandbox="allow-scripts" style="width:480px;height:270px;border:0"></iframe>
<canvas id="c" width="1920" height="1080"></canvas>
<script>
window.__par = { hashes: {}, errors: [], log: [], done: false, failure: null };
const par = window.__par, iframe = document.getElementById('f');
const API = '/api/desktop/storymaker';
const PROJECT = '%s';
const FRACTIONS = %s;
let TARGETS = [];
async function getJSON(u) { const r = await fetch(u); if (!r.ok) throw new Error(u + ' ' + r.status); return r.json(); }
function post(msg) { iframe.contentWindow.postMessage(Object.assign({ sm: 1 }, msg), '*'); }
async function sha(bitmap) {
  const c = document.getElementById('c'), ctx = c.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, c.width, c.height);
  ctx.drawImage(bitmap, 0, 0);
  const data = ctx.getImageData(0, 0, bitmap.width, bitmap.height).data;
  const digest = await crypto.subtle.digest('SHA-256', data);
  return { size: bitmap.width + 'x' + bitmap.height, hash: Array.from(new Uint8Array(digest)).map(b => b.toString(16).padStart(2, '0')).join('') };
}
let next = 0;
function ask() {
  if (next >= TARGETS.length) { par.done = true; return; }
  const t = TARGETS[next];
  post({ type: 'thumb', req: 't' + next, scene_id: t.scene, frame: t.frame, width: 1920 });
}
window.addEventListener('message', async (ev) => {
  if (ev.source !== iframe.contentWindow) return;
  const m = ev.data;
  if (!m || m.sm !== 1) return;
  par.log.push(m.type);
  try {
    if (m.type === 'ready') {
      const project = await getJSON(API + '/projects/' + PROJECT);
      const catalog = await getJSON('/img/storymaker/catalog.json');
      const files = {};
      for (const p of [catalog.rig.file].concat(catalog.entries.filter(e => e.file && e.kind !== 'sfx').map(e => e.file))) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
      const fonts = [];
      for (const f of catalog.fonts) fonts.push({ name: f.name, family: f.family, weight: f.weight, data: await (await fetch(f.file)).arrayBuffer() });
      const bundles = {};
      for (const sc of project.story.scenes) if (sc.kind === 'code') bundles[sc.id] = await (await fetch(API + '/projects/' + PROJECT + '/scenes/' + sc.id + '/bundle')).text();
      post({ type: 'load', story: project.story, catalog, audio_info: {}, assets: files, fonts, code_bundles: bundles, format: { aspect: '16:9', fps: 30, width: 1920 } });
    } else if (m.type === 'compiled') {
      par.report = { frame_count: m.report.frame_count, scenes: m.report.scenes };
      TARGETS = FRACTIONS.map((f) => {
        const frame = Math.round(f * (m.report.frame_count - 1));
        const scenes = m.report.scenes.filter((s) => s.start_frame <= frame);
        return { frame, scene: scenes[scenes.length - 1].id };
      });
    } else if (m.type === 'time' && !par.asked) {
      par.asked = true;
      ask();
    } else if (m.type === 'thumb') {
      const t = TARGETS[next];
      par.hashes[t.frame] = await sha(m.bitmap);
      next++;
      ask();
    } else if (m.type === 'error') {
      par.errors.push({ code: m.code, message: m.message, scene_id: m.scene_id });
    }
  } catch (e) { par.failure = String(e && e.stack || e); par.done = true; }
});
iframe.src = API + '/player';
</script>`

// pixelHash hashes the straight RGBA pixels of a PNG frame, like the parent page hashes getImageData.
func pixelHash(t *testing.T, data []byte) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("a frame did not decode: %v", err)
		return ""
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	sum := sha256.Sum256(nrgba.Pix)
	return hex.EncodeToString(sum[:])
}

// TestStorymakerParityAppPlayerAndRenderPage renders firewall.json through the whole export pipeline and
// compares the decoded RGBA of several frames with the same frames drawn by the app player host in the sandboxed
// iframe: same Chromium, export canvas size, device pixel ratio 1, after the readiness barrier (the app host loads
// fonts and assets before it draws frame 0). Gated by AURAGO_RUN_BROWSER_SMOKE=1.
func TestStorymakerParityAppPlayerAndRenderPage(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker parity test")
	}
	chrome := os.Getenv("AURAGO_TEST_CHROME")
	if chrome == "" {
		found, ok := launcher.LookPath()
		if !ok {
			t.Skip("no Chrome or Chromium is installed")
		}
		chrome = found
	}
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		render.SetNoSandboxForTests(true)
		t.Cleanup(func() { render.SetNoSandboxForTests(false) })
	}

	s, _, token := storymakerAPIFixture(t)
	s.Cfg.Auth.Enabled = false // the iframe document request carries no bearer token
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.Cfg.Storymaker.Render.BrowserPath = chrome
	s.storymakerBrowsers = nil // the real resolver
	var mu sync.Mutex
	rendered := map[int]string{}
	s.storymakerFrameObserver = func(n int, transport string, data []byte) {
		h := pixelHash(t, data)
		mu.Lock()
		rendered[n] = h
		mu.Unlock()
	}

	// The project: create, save firewall.json and its code scene, all through the HTTP API.
	p := createStorymakerProject(t, s, token)
	story, err := os.ReadFile("../storymaker/testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	scene, err := os.ReadFile("../storymaker/testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", bytes.NewReader(lf(story)), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/scenes/s3/code", bytes.NewReader(lf(scene)), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusOK {
		t.Fatalf("save code scene: %d %s", w.Code, w.Body.String())
	}
	w = storymakerRequest(s, token, http.MethodGet, "/projects/"+p.Project.ID, nil, nil)
	var current struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &current)

	// The render gate needs stored narration.
	seedStoryNarrationForRender(t, s, p.Project.ID)

	// The export.
	w = storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": current.ETag})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	if w.Code != http.StatusAccepted {
		t.Fatalf("render: %d %s", w.Code, w.Body.String())
	}
	var job render.Job
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		var got struct {
			Job render.Job `json:"job"`
		}
		decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil), &got)
		job = got.Job
		if job.Terminal() {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if job.Status != render.StatusSucceeded {
		t.Fatalf("export: %+v error %+v", job, job.Error)
	}
	t.Logf("export: %d frames, %s", job.FramesTotal, job.Output.Path)

	// Frames to compare: the first, the middle of every scene, and the transitions between them.
	fractions := []float64{0, 0.17, 0.36, 0.5, 0.58, 0.62, 0.77, 0.9, 1}
	fractionsJSON, _ := json.Marshal(fractions)

	// The app side: the real player handler and API behind one origin, the static assets from the ui tree.
	uiDir, _ := filepath.Abs("../../ui")
	mux := http.NewServeMux()
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	mux.Handle("/js/storymaker/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/css/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/img/storymaker/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/fonts/", http.FileServer(http.Dir(uiDir)))
	mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, storymakerParityParent, p.Project.ID, fractionsJSON)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(chrome).Headless(true).Set("disable-gpu").Set("force-device-scale-factor", "1")
	l.Delete("no-sandbox")
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		l.Set("no-sandbox")
	}
	u, err := l.Launch()
	if err != nil {
		t.Skipf("the viewer browser did not start: %v", err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(u)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = browser.Close() }()
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1920, Height: 1080, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(srv.URL + "/parent"); err != nil {
		t.Fatal(err)
	}
	if err := page.Timeout(3 * time.Minute).Wait(rod.Eval(`() => window.__par && window.__par.done`)); err != nil {
		t.Fatalf("the player never finished: %v\n%s", err, page.MustEval(`() => JSON.stringify(window.__par)`).Str())
	}
	var res struct {
		Hashes  map[int]struct{ Size, Hash string } `json:"hashes"`
		Errors  []struct{ Code, Message string }    `json:"errors"`
		Failure string                              `json:"failure"`
		Report  struct {
			FrameCount int `json:"frame_count"`
		} `json:"report"`
	}
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__par)`).Str()), &res); err != nil {
		t.Fatal(err)
	}
	if res.Failure != "" || len(res.Errors) != 0 {
		t.Fatalf("player failure %q errors %+v", res.Failure, res.Errors)
	}
	if res.Report.FrameCount != job.FramesTotal {
		t.Fatalf("the app compiled %d frames, the export has %d", res.Report.FrameCount, job.FramesTotal)
	}
	if len(res.Hashes) < len(fractions)-2 {
		t.Fatalf("the app produced only %d frames", len(res.Hashes))
	}
	var mismatches []string
	for frame, app := range res.Hashes {
		mu.Lock()
		exp := rendered[frame]
		mu.Unlock()
		switch {
		case app.Hash == "":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the app produced no frame", frame))
		case app.Size != "1920x1080":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the app frame is %s", frame, app.Size))
		case exp == "":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the export has no such frame", frame))
		case app.Hash != exp:
			mismatches = append(mismatches, fmt.Sprintf("frame %d: app %s != export %s", frame, app.Hash[:12], exp[:12]))
		}
	}
	if len(mismatches) != 0 {
		sort.Strings(mismatches)
		t.Fatalf("app player and render page differ:\n%s", strings.Join(mismatches, "\n"))
	}
	t.Logf("%d frames identical between the app player host and the render page", len(res.Hashes))
	_ = storymaker.Schema
}
