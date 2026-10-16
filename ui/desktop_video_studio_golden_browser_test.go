package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
	"aurago/internal/videostudio/goldenframe"

	"github.com/go-rod/rod"
	"github.com/ysmood/gson"
)

// The WebGL2 helper on SwiftShader: quiet compile failures that are remembered, cleared and drawn targets read
// top-down, uploads without a flip, the target pool, and loss counting with restore.
func TestDesktopVideoStudioGLHelperBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, ""))
	requireWebGL2(t, page)
	result := page.MustEval(`async () => {
		const S = window.VideoStudioShaders, out = {};
		const stage = document.getElementById('stage');
		const canvas = document.createElement('canvas'); canvas.width = 8; canvas.height = 4; stage.replaceChildren(canvas);
		let lostCalls = 0, restoredCalls = 0;
		const ctx = window.VideoStudioGL.create(canvas, { onLost: () => { lostCalls++; }, onRestored: () => { restoredCalls++; } });
		out.created = !!ctx && ctx.gl instanceof WebGL2RenderingContext;
		let threw = false, broken;
		try { broken = ctx.program('broken', S.QUAD_VERTEX, '#version 300 es\nthis is not glsl'); } catch (_) { threw = true; }
		out.brokenNull = !threw && broken === null && ctx.failed('broken') === true && ctx.program('broken', S.QUAD_VERTEX, S.COPY) === null;
		const copy = ctx.program('copy', S.QUAD_VERTEX, S.COPY);
		out.copyCached = !!copy && ctx.program('copy', S.QUAD_VERTEX, S.COPY) === copy && !ctx.failed('copy');
		const px = (data, x, y, width) => Array.from(data.slice((y * width + x) * 4, (y * width + x) * 4 + 4));
		const target = ctx.target(8, 4);
		ctx.clear(target, [1, 0, 0, 1]);
		const cleared = ctx.read(target);
		out.clearRed = cleared.length === 128 && Array.from(cleared).every((value, index) => value === [255, 0, 0, 255][index % 4]);
		const art = document.createElement('canvas'); art.width = 8; art.height = 4;
		const art2d = art.getContext('2d');
		art2d.fillStyle = '#ff0000'; art2d.fillRect(0, 0, 8, 2);
		art2d.fillStyle = '#0000ff'; art2d.fillRect(0, 2, 8, 2);
		const texture = ctx.texture();
		out.upload = texture.upload(art) === true && texture.width === 8 && texture.height === 4;
		ctx.draw(copy, target, { u_flipY: 0 }, { u_source: texture });
		const rows = ctx.read(target);
		out.targetTop = px(rows, 3, 0, 8); out.targetBottom = px(rows, 3, 3, 8);
		const part = ctx.read(target, { x: 2, y: 3, width: 2, height: 1 });
		out.rectRead = part.length === 8 && part[0] === 0 && part[2] === 255 && part[6] === 255;
		ctx.draw(copy, null, { u_flipY: 1 }, { u_source: target });
		const screen = ctx.read(null);
		out.canvasTop = px(screen, 3, 0, 8); out.canvasBottom = px(screen, 3, 3, 8);
		const probe = document.createElement('canvas'); probe.width = 8; probe.height = 4;
		const view = probe.getContext('2d'); view.drawImage(canvas, 0, 0);
		out.shownTop = Array.from(view.getImageData(3, 0, 1, 1).data);
		ctx.release(target);
		const again = ctx.target(8, 4), other = ctx.target(4, 4);
		out.pooled = again === target && !!other && other !== target;
		ctx.release(again); ctx.release(other);
		const lose = ctx.gl.getExtension('WEBGL_lose_context');
		// An event that never comes fails the run after 10 s instead of hanging it. The pause after a loss is needed:
		// Chrome only accepts restoreContext() once the lost event has been dispatched, and an await continuation runs
		// inside the dispatch (microtask after the listener), where the call is silently refused.
		const next = name => new Promise((resolve, reject) => {
			const timer = setTimeout(() => reject(new Error('timed out waiting for ' + name)), 10000);
			canvas.addEventListener(name, event => { clearTimeout(timer); resolve(event); }, { once: true });
		});
		const afterDispatch = () => new Promise(resolve => setTimeout(resolve, 50));
		let lost = next('webglcontextlost'); lose.loseContext(); await lost; await afterDispatch();
		out.lostState = ctx.lost() && ctx.lossesWithin(60000) === 1 && lostCalls === 1 && ctx.program('copy', S.QUAD_VERTEX, S.COPY) === null && !ctx.failed('copy') && ctx.target(8, 4) === null;
		const restored = next('webglcontextrestored'); lose.restoreContext(); await restored;
		const rebuilt = ctx.program('copy', S.QUAD_VERTEX, S.COPY), fresh = ctx.target(8, 4);
		if (fresh) ctx.clear(fresh, [0, 1, 0, 1]);
		out.restoredState = !ctx.lost() && restoredCalls === 1 && !!rebuilt && rebuilt !== copy && !!fresh && px(ctx.read(fresh), 0, 0, 8).join() === '0,255,0,255' && texture.upload(art) === true;
		lost = next('webglcontextlost'); lose.loseContext(); await lost;
		out.secondLoss = ctx.lossesWithin(60000) === 2 && lostCalls === 2;
		ctx.dispose();
		return out;
	}`)
	checks := result.Map()
	for _, name := range []string{"created", "brokenNull", "copyCached", "clearRed", "upload", "rectRead", "pooled", "lostState", "restoredState", "secondLoss"} {
		if !checks[name].Bool() {
			t.Errorf("GL helper check %s failed: %s", name, result.JSON("", ""))
		}
	}
	for name, want := range map[string][4]int{
		"targetTop": {255, 0, 0, 255}, "targetBottom": {0, 0, 255, 255},
		"canvasTop": {255, 0, 0, 255}, "canvasBottom": {0, 0, 255, 255}, "shownTop": {255, 0, 0, 255},
	} {
		got := checks[name].Arr()
		if len(got) != 4 || got[0].Int() != want[0] || got[1].Int() != want[1] || got[2].Int() != want[2] || got[3].Int() != want[3] {
			t.Errorf("%s = %s, want %v (rows must read top-down and the canvas must show upright)", name, checks[name].JSON("", ""), want)
		}
	}
}

// writeEngineTestPNG writes a straight-alpha PNG of one colour; alpha falls to zero over the outer feather pixels.
func writeEngineTestPNG(t *testing.T, path string, width, height int, fill color.RGBA, feather int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			alpha := 255
			if feather > 0 {
				if edge := min(min(x, width-1-x), min(y, height-1-y)); edge < feather {
					alpha = 255 * edge / feather
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{R: fill.R, G: fill.G, B: fill.B, A: uint8(alpha)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func engineTestAsset(id string, width, height int) map[string]any {
	return map[string]any{"id": id, "name": id + ".png", "path": "media/" + id + ".png", "kind": "image", "duration_frames": 250, "width": width, "height": height, "has_audio": false}
}

func engineTestClip(id, asset string, start, duration int, extra map[string]any) map[string]any {
	clip := map[string]any{
		"id": id, "asset_id": asset, "start": start, "offset": 0, "duration": duration,
		"x": 0, "y": 0, "width": 1, "height": 1, "rotation": 0, "opacity": 1, "volume": 1,
		"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil,
	}
	for key, value := range extra {
		clip[key] = value
	}
	return clip
}

func engineTestTrack(id, kind string, clips ...map[string]any) map[string]any {
	list := make([]any, 0, len(clips))
	for _, clip := range clips {
		list = append(list, clip)
	}
	return map[string]any{"id": id, "kind": kind, "name": id, "muted": false, "hidden": false, "locked": false, "clips": list}
}

// engineTestProject is a 640×360, 25 fps version 3 project; tracks paint in array order (first is the bottom).
func engineTestProject(assets []map[string]any, tracks ...map[string]any) map[string]any {
	assetList := make([]any, 0, len(assets))
	for _, asset := range assets {
		assetList = append(assetList, asset)
	}
	trackList := make([]any, 0, len(tracks))
	for _, track := range tracks {
		trackList = append(trackList, track)
	}
	return map[string]any{"version": 3, "name": "Engine", "width": 640, "height": 360, "fps": 25, "assets": assetList, "tracks": trackList}
}

func jsonRGBA(v gson.JSON) color.RGBA {
	values := v.Arr()
	if len(values) < 4 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(values[0].Int()), G: uint8(values[1].Int()), B: uint8(values[2].Int()), A: uint8(values[3].Int())}
}

func rgbaNear(got, want color.RGBA, tolerance int) bool {
	near := func(a, b uint8) bool { d := int(a) - int(b); return d >= -tolerance && d <= tolerance }
	return near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B) && near(got.A, want.A)
}

// The GL compositor reproduces today's preview on the engine page (SwiftShader): placement and fit, opacity over a
// lower track, a dissolve midpoint, and hold-last-frame on a hand-built scene.
func TestDesktopVideoStudioCompositorBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := t.TempDir()
	writeEngineTestPNG(t, filepath.Join(dir, "logo.png"), 320, 180, color.RGBA{200, 120, 40, 255}, 24)
	writeEngineTestPNG(t, filepath.Join(dir, "portrait.png"), 180, 320, color.RGBA{40, 160, 220, 255}, 0)
	writeEngineTestPNG(t, filepath.Join(dir, "red.png"), 320, 180, color.RGBA{255, 0, 0, 255}, 0)
	writeEngineTestPNG(t, filepath.Join(dir, "blue.png"), 320, 180, color.RGBA{0, 0, 255, 255}, 0)
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	page.MustEval(`() => { Object.assign(window.vsGolden.files, {logo: 'logo.png', portrait: 'portrait.png', red: 'red.png', blue: 'blue.png'}); }`)
	assets := []map[string]any{engineTestAsset("logo", 320, 180), engineTestAsset("portrait", 180, 320), engineTestAsset("red", 320, 180), engineTestAsset("blue", 320, 180)}
	render := func(name string, frame int, tracks ...map[string]any) *image.RGBA {
		t.Helper()
		got := renderEngineFrame(t, page, engineTestProject(assets, tracks...), frame, "gl")
		if got.Painter != "gl" || !got.Settled {
			t.Fatalf("%s: painter %q settled %v, want gl and settled", name, got.Painter, got.Settled)
		}
		return got.Image
	}
	check := func(name string, img *image.RGBA, x, y int, want color.RGBA) {
		t.Helper()
		if got := img.RGBAAt(x, y); !rgbaNear(got, want, 3) {
			t.Errorf("%s: pixel (%d,%d) = %v, want %v ±3", name, x, y, got, want)
		}
	}
	logo := render("logo", 0, engineTestTrack("o1", "overlay", engineTestClip("c1", "logo", 0, 50, nil)))
	check("logo centre", logo, 320, 180, color.RGBA{200, 120, 40, 255})
	contain := render("contain", 0, engineTestTrack("v1", "video", engineTestClip("c1", "portrait", 0, 50, nil)))
	check("contain centre", contain, 320, 180, color.RGBA{40, 160, 220, 255})
	check("contain leaves a black bar", contain, 20, 180, color.RGBA{0, 0, 0, 255})
	cover := render("cover", 0, engineTestTrack("v1", "video", engineTestClip("c1", "portrait", 0, 50, map[string]any{"fit": "cover"})))
	check("cover fills the box", cover, 20, 180, color.RGBA{40, 160, 220, 255})
	mixed := render("opacity", 0,
		engineTestTrack("v1", "video", engineTestClip("c1", "red", 0, 50, nil)),
		engineTestTrack("o1", "overlay", engineTestClip("c2", "blue", 0, 50, map[string]any{"opacity": 0.5})))
	check("half opacity over a lower track", mixed, 320, 180, color.RGBA{128, 0, 128, 255})
	dissolve := render("dissolve", 25, engineTestTrack("v1", "video",
		engineTestClip("c1", "red", 0, 30, map[string]any{"transition": map[string]any{"type": "dissolve", "duration": 10}}),
		engineTestClip("c2", "blue", 20, 30, nil)))
	check("dissolve at p = 0.5", dissolve, 320, 180, color.RGBA{128, 0, 128, 255})
	hold := page.MustEval(`() => {
		const C = window.VideoStudioCompositor, E = window.VideoStudioEffects;
		const canvas = document.createElement('canvas'); canvas.width = 64; canvas.height = 36;
		document.getElementById('stage').replaceChildren(canvas);
		const painter = C.create(canvas, {});
		if (!painter) return { created: false, error: C.lastError };
		const solid = colour => { const c = document.createElement('canvas'); c.width = 16; c.height = 9; const g = c.getContext('2d'); g.fillStyle = colour; g.fillRect(0, 0, 16, 9); return c; };
		const red = solid('#ff0000'), blue = solid('#0000ff');
		const project = { version: 3, name: 'hold', width: 64, height: 36, fps: 25, assets: [], tracks: [] };
		const clip = { id: 'c1', asset_id: 'a', start: 0, offset: 0, duration: 50, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain' };
		const item = (el, ready, owner) => { const c = owner || clip; return { clip: c, asset: { id: 'a', kind: 'video', width: 16, height: 9 }, el, ready, state: E.clipState(c, 0, project) }; };
		const scene = items => ({ project, frame: 0, width: 64, height: 36, layers: [{ trackId: 'v1', items, transition: null }] });
		const centre = () => Array.from(painter.read({ x: 32, y: 18, width: 1, height: 1 }).data);
		painter.frame(scene([item(red, true)])); const first = centre();
		painter.frame(scene([item(blue, false)])); const notReady = centre();
		Object.defineProperty(blue, 'seeking', { value: true, configurable: true });
		painter.frame(scene([item(blue, true)])); const seeking = centre();
		delete blue.seeking;
		painter.frame(scene([item(blue, true)])); const ready = centre();
		painter.frame(scene([item(red, false, Object.assign({}, clip, { id: 'c2' }))])); const never = centre();
		painter.release('c1');
		painter.frame(scene([item(blue, false)])); const released = centre();
		painter.dispose();
		return { created: true, supported: C.supported(), first, notReady, seeking, ready, never, released };
	}`).Map()
	if !hold["created"].Bool() || !hold["supported"].Bool() {
		t.Fatalf("compositor did not start: %v", hold)
	}
	red, blue, black := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}, color.RGBA{0, 0, 0, 255}
	for name, want := range map[string]color.RGBA{"first": red, "notReady": red, "seeking": red, "ready": blue, "never": black, "released": black} {
		if got := jsonRGBA(hold[name]); !rgbaNear(got, want, 3) {
			t.Errorf("hold last frame %s = %v, want %v", name, got, want)
		}
	}
}

// Fallback on the engine page (SwiftShader): one loss and restore stays GL and repaints; three losses within 60 s switch
// to 2D on a fresh element that keeps its attributes; fallback('forced') and painter '2d' work; a transition program
// that does not compile draws a hard cut at p >= 0.5 and is reported once.
func TestDesktopVideoStudioPreviewFallbackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := t.TempDir()
	writeEngineTestPNG(t, filepath.Join(dir, "red.png"), 320, 180, color.RGBA{255, 0, 0, 255}, 0)
	writeEngineTestPNG(t, filepath.Join(dir, "blue.png"), 320, 180, color.RGBA{0, 0, 255, 255}, 0)
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	assets := []map[string]any{engineTestAsset("red", 320, 180), engineTestAsset("blue", 320, 180)}
	still := engineTestProject(assets, engineTestTrack("v1", "video", engineTestClip("c1", "red", 0, 50, nil)))
	dissolve := engineTestProject(assets, engineTestTrack("v1", "video",
		engineTestClip("c1", "red", 0, 30, map[string]any{"transition": map[string]any{"type": "dissolve", "duration": 10}}),
		engineTestClip("c2", "blue", 20, 30, nil)))
	out := page.MustEval(`async (still, dissolve) => {
		const out = {}, stage = document.getElementById('stage');
		Object.assign(vsGolden.files, { red: 'red.png', blue: 'blue.png' });
		// An event that never comes fails the run after 10 s instead of hanging it. restoreContext() needs a pause after
		// the lost event: Chrome ignores it inside the dispatch, and an await continuation still runs inside it.
		const next = (target, name) => new Promise((resolve, reject) => {
			const timer = setTimeout(() => reject(new Error('timed out waiting for ' + name)), 10000);
			target.addEventListener(name, event => { clearTimeout(timer); resolve(event); }, { once: true });
		});
		const afterDispatch = () => new Promise(resolve => setTimeout(resolve, 50));
		const centre = preview => { const c = stage.querySelector('canvas'); return Array.from(preview.capture({ x: Math.floor(c.width / 2), y: Math.floor(c.height / 2), width: 1, height: 1 }).data); };
		const mount = (project, painter, frame, changes, failures) => {
			const canvas = document.createElement('canvas');
			canvas.setAttribute('data-preview', ''); canvas.setAttribute('aria-label', 'Preview');
			canvas.width = project.width; canvas.height = project.height;
			stage.replaceChildren(canvas);
			let shown = frame;
			const preview = VideoStudioPreview.mount(canvas, () => project, () => shown, value => { shown = value; }, () => {}, () => 1, {
				painter, mediaURL: asset => vsGolden.mediaURL(asset),
				onPainterChange: (kind, reason, element) => changes.push([kind, reason, element === stage.querySelector('canvas'), element.hasAttribute('data-preview'), element.getAttribute('aria-label')]),
				onProgramFailed: (kind, id) => failures.push([kind, id])
			});
			return { preview, canvas };
		};
		let changes = [], failures = [];
		let { preview, canvas } = mount(still, 'gl', 0, changes, failures);
		await preview.settle(15000);
		out.startKind = preview.painterKind(); out.startPixel = centre(preview);
		const lose = canvas.getContext('webgl2') && canvas.getContext('webgl2').getExtension('WEBGL_lose_context');
		out.hasLoseContext = !!lose;
		let lost = next(canvas, 'webglcontextlost'); lose.loseContext(); await lost; await afterDispatch();
		let restored = next(canvas, 'webglcontextrestored'); lose.restoreContext(); await restored;
		await preview.settle(15000);
		out.afterRestoreKind = preview.painterKind(); out.afterRestorePixel = centre(preview);
		out.sameElement = stage.querySelector('canvas') === canvas;
		lost = next(canvas, 'webglcontextlost'); lose.loseContext(); await lost; await afterDispatch();
		restored = next(canvas, 'webglcontextrestored'); lose.restoreContext(); await restored;
		lost = next(canvas, 'webglcontextlost'); lose.loseContext(); await lost;
		await preview.settle(15000);
		const swapped = stage.querySelector('canvas');
		out.fallbackKind = preview.painterKind();
		out.replaced = swapped !== canvas && !canvas.isConnected;
		out.keepsAttributes = swapped.hasAttribute('data-preview') && swapped.getAttribute('aria-label') === 'Preview';
		out.fallbackPixel = centre(preview); out.lossChanges = changes.slice();
		preview.dispose();
		changes = []; failures = [];
		({ preview, canvas } = mount(still, 'gl', 0, changes, failures));
		await preview.settle(15000);
		out.forcedStart = preview.painterKind();
		out.forcedFirst = preview.fallback('forced'); out.forcedSecond = preview.fallback('forced');
		out.forcedKind = preview.painterKind(); out.forcedReplaced = stage.querySelector('canvas') !== canvas;
		await preview.settle(15000);
		out.forcedPixel = centre(preview); out.forcedChanges = changes.slice();
		preview.dispose();
		changes = []; failures = [];
		({ preview, canvas } = mount(still, '2d', 0, changes, failures));
		await preview.settle(15000);
		out.plainKind = preview.painterKind(); out.plainSame = stage.querySelector('canvas') === canvas;
		out.plainPixel = centre(preview); out.plainChanges = changes.length;
		preview.dispose();
		const original = VideoStudioShaders.transition;
		VideoStudioShaders.transition = () => '#version 300 es\nnot glsl';
		try {
			changes = []; failures = [];
			({ preview, canvas } = mount(dissolve, 'gl', 22, changes, failures));
			preview.seek(22); await preview.settle(15000);
			out.cutEarly = centre(preview);
			preview.seek(28); await preview.settle(15000);
			out.cutLate = centre(preview);
			out.cutKind = preview.painterKind(); out.cutFailures = failures.slice(); out.cutList = preview.failedPrograms();
			preview.dispose();
		} finally {
			VideoStudioShaders.transition = original;
		}
		stage.replaceChildren();
		return out;
	}`, still, dissolve)
	checks := out.Map()
	compact := func(name string) string { return strings.Join(strings.Fields(checks[name].JSON("", "")), "") }
	for name, want := range map[string]string{"startKind": "gl", "afterRestoreKind": "gl", "fallbackKind": "2d", "forcedStart": "gl", "forcedKind": "2d", "plainKind": "2d", "cutKind": "gl"} {
		if got := checks[name].Str(); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"hasLoseContext", "sameElement", "replaced", "keepsAttributes", "forcedFirst", "forcedReplaced", "plainSame"} {
		if !checks[name].Bool() {
			t.Errorf("%s is false: %s", name, out.JSON("", ""))
		}
	}
	if checks["forcedSecond"].Bool() {
		t.Error("a second fallback must report false")
	}
	red, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	for name, want := range map[string]color.RGBA{"startPixel": red, "afterRestorePixel": red, "fallbackPixel": red, "forcedPixel": red, "plainPixel": red, "cutEarly": red, "cutLate": blue} {
		if got := jsonRGBA(checks[name]); !rgbaNear(got, want, 3) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	for name, want := range map[string]string{
		"lossChanges":   `[["2d","context_lost",true,true,"Preview"]]`,
		"forcedChanges": `[["2d","forced",true,true,"Preview"]]`,
		"cutFailures":   `[["transition","dissolve"]]`,
	} {
		if got := compact(name); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	if checks["plainChanges"].Int() != 0 {
		t.Errorf("an explicit 2D preview must not report a painter change: %s", out.JSON("", ""))
	}
	if list := checks["cutList"].Arr(); len(list) != 1 || list[0].Get("kind").Str() != "transition" || list[0].Get("id").Str() != "dissolve" {
		t.Errorf("failedPrograms() = %s", checks["cutList"].JSON("", ""))
	}
}

// goldenSeekFile is the paused-seek evidence clip: 1 s at 25 fps, frame N a flat grey of luma 24+8N, so a captured
// pixel names the frame on screen.
const goldenSeekFile = "seekframes.mp4"

// TestDesktopVideoStudioGoldenFrames compares every registered golden case (goldenframe.Cases) between the export and
// the preview. RenderFrame writes the export's lossless RGB frame. vsGolden.render draws the same frame on the engine
// page with the requested painter. SSIM must reach the case's threshold. The seek_frame_starts subtest is the
// paused-seek evidence of plan 2a (open question 12). Needs AURAGO_RUN_BROWSER_SMOKE=1, ffmpeg and Chrome or Edge. Each
// case gets its own SwiftShader browser, so no browser outlives the launcher's 180 s context.
func TestDesktopVideoStudioGoldenFrames(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("golden frames need ffmpeg on the PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	media, err := goldenframe.MakeMedia(ctx, ffmpeg, t.TempDir())
	if err != nil {
		t.Fatalf("MakeMedia: %v", err)
	}
	seekPath := filepath.Join(media.Dir, goldenSeekFile)
	writeGoldenSeekMedia(t, ctx, ffmpeg, seekPath)
	engine := newVideoStudioEngineServer(t, media.Dir)
	t.Run("seek_frame_starts", func(t *testing.T) { checkGoldenSeekFrames(t, ctx, ffmpeg, engine, media.Files, seekPath) })
	frames := t.TempDir()
	for _, c := range goldenframe.Cases(media) {
		t.Run(c.Name, func(t *testing.T) {
			want := goldenRenderWant(t, ctx, ffmpeg, media, c, filepath.Join(frames, c.Name+".png"))
			page := goldenEnginePage(t, engine, media.Files)
			for _, painter := range c.Painters {
				t.Run(painter, func(t *testing.T) {
					got := goldenRenderGot(t, page, c, painter)
					score, err := goldenframe.SSIM(want, got)
					if err != nil {
						t.Fatalf("SSIM: %v", err)
					}
					failed := score.Min < c.Threshold
					kept := goldenKeepArtifacts(t, c.Name+"_"+painter, want, got, failed)
					if failed {
						t.Fatalf("SSIM R %.4f G %.4f B %.4f, min %.4f < %.2f (want/got/diff PNGs: %s)", score.R, score.G, score.B, score.Min, c.Threshold, kept)
					}
					t.Logf("SSIM min %.4f (threshold %.2f)", score.Min, c.Threshold)
				})
			}
		})
	}
}

// goldenEnginePage opens the Task 10 engine page in a fresh SwiftShader browser (openVideoStudioEngine) and fills the
// asset-file map that vsGolden.render resolves media through. The map is filled in place, because render keeps its
// own reference to the object.
func goldenEnginePage(t *testing.T, engine *httptest.Server, files map[string]string) *rod.Page {
	t.Helper()
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), engine)
	page.MustEval(`files=>{Object.assign(window.vsGolden.files,files);}`, files)
	return page
}

func goldenDecodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", filepath.Base(path), err)
	}
	return img
}

func goldenRenderWant(t *testing.T, ctx context.Context, ffmpeg string, media goldenframe.Media, c goldenframe.Case, output string) image.Image {
	t.Helper()
	if err := videostudio.RenderFrame(ctx, videostudio.FrameOptions{FFmpegPath: ffmpeg, Project: c.Project, AssetPaths: media.Paths, Frame: c.Frame, Output: output}); err != nil {
		t.Fatalf("RenderFrame(%s, frame %d): %v", c.Name, c.Frame, err)
	}
	return goldenDecodePNG(t, output)
}

// goldenRenderGot draws the case on the engine page with Task 10's renderEngineFrame and insists on the requested
// painter and settled media.
func goldenRenderGot(t *testing.T, page *rod.Page, c goldenframe.Case, painter string) image.Image {
	t.Helper()
	got := renderEngineFrame(t, page, c.Project, c.Frame, painter)
	if got.Painter != painter {
		t.Fatalf("vsGolden.render was asked for the %s painter and drew with %q (GL must work under the SwiftShader launcher)", painter, got.Painter)
	}
	if !got.Settled {
		t.Fatalf("%s/%s: the preview media did not settle within 15 s", c.Name, painter)
	}
	return got.Image
}

// goldenKeepArtifacts writes want/got/diff PNGs. With AURAGO_GOLDEN_OUT set it writes them there for every comparison,
// for review. Without it, it writes them only for a failed comparison, into a new temporary folder outside the checkout.
// It returns the folder, or "" when nothing was kept.
func goldenKeepArtifacts(t *testing.T, name string, want, got image.Image, failed bool) string {
	t.Helper()
	dir := os.Getenv("AURAGO_GOLDEN_OUT")
	if dir == "" && !failed {
		return ""
	}
	if dir == "" {
		var err error
		if dir, err = os.MkdirTemp("", "aurago-golden-"); err != nil {
			t.Logf("keep artifacts for %s: %v", name, err)
			return ""
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("keep artifacts for %s: %v", name, err)
		return ""
	}
	if err := goldenframe.WriteArtifacts(dir, name, want, got); err != nil {
		t.Logf("write artifacts for %s: %v", name, err)
		return ""
	}
	return dir
}

func writeGoldenSeekMedia(t *testing.T, ctx context.Context, ffmpeg, path string) {
	t.Helper()
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "nullsrc=s=320x180:r=25:d=1,format=gray,geq=lum='24+8*N'",
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "4", "-g", "1", "-bf", "0", "-pix_fmt", "yuv420p",
		"-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709", "-f", "mp4", path}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create %s: %v: %s", goldenSeekFile, err, out)
	}
}

// goldenSeekLevels decodes the evidence clip to RGB with FFmpeg and returns the centre green value of every frame.
func goldenSeekLevels(t *testing.T, ctx context.Context, ffmpeg, path string) []int {
	t.Helper()
	const width, height = 320, 180
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-i", path,
		"-map", "0:v:0", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1").Output()
	frame := width * height * 3
	if err != nil || len(out) == 0 || len(out)%frame != 0 {
		t.Fatalf("decode %s: %v (%d bytes)", goldenSeekFile, err, len(out))
	}
	levels := make([]int, len(out)/frame)
	for i := range levels {
		levels[i] = int(out[i*frame+((height/2)*width+width/2)*3+1])
	}
	if len(levels) != 25 {
		t.Fatalf("%s has %d frames, want 25", goldenSeekFile, len(levels))
	}
	return levels
}

func goldenSeekNearest(levels []int, value float64) (int, float64) {
	best, distance := -1, math.MaxFloat64
	for i, level := range levels {
		if d := math.Abs(float64(level) - value); d < distance {
			best, distance = i, d
		}
	}
	return best, distance
}

// checkGoldenSeekFrames is the paused-seek evidence of plan 2a (open question 12). A paused preview seeked to
// timeline frame k of a 25 fps project must show source frame k, the frame the export renders there.
func checkGoldenSeekFrames(t *testing.T, ctx context.Context, ffmpeg string, engine *httptest.Server, files map[string]string, path string) {
	levels := goldenSeekLevels(t, ctx, ffmpeg, path)
	asset, err := videostudio.Probe(ctx, ffmpeg, path)
	if err != nil {
		t.Fatalf("probe %s: %v", goldenSeekFile, err)
	}
	asset.ID = "seek-frames"
	asset = videostudio.AssetAtRate(asset, videostudio.Rate{Num: 25, Den: 1})
	project := videostudio.Project{Version: videostudio.CurrentVersion, Name: "Seek frames", Width: 1280, Height: 720, FPS: 25,
		Assets: []videostudio.Asset{asset},
		Tracks: []videostudio.Track{{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{
			{ID: "seek", AssetID: asset.ID, Start: 0, Duration: asset.DurationFrames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain},
		}}}}
	frames := []int{1, 3, 6, 7, 12, 20, 24}
	// The export: timeline frame k shows source frame k. If not, the evidence clip or RenderFrame is off, not the preview.
	dir := t.TempDir()
	for _, frame := range frames {
		output := filepath.Join(dir, fmt.Sprintf("seek-%02d.png", frame))
		if err := videostudio.RenderFrame(ctx, videostudio.FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: map[string]string{asset.ID: path}, Frame: frame, Output: output}); err != nil {
			t.Fatalf("RenderFrame(seek, %d): %v", frame, err)
		}
		_, g, _, _ := goldenDecodePNG(t, output).At(project.Width/2, project.Height/2).RGBA()
		if got, distance := goldenSeekNearest(levels, float64(g>>8)); got != frame || distance > 6 {
			t.Fatalf("the export shows source frame %d (distance %.1f) at timeline frame %d", got, distance, frame)
		}
	}
	for _, painter := range []string{"gl", "2d"} {
		t.Run(painter, func(t *testing.T) {
			page := goldenEnginePage(t, engine, files)
			t.Logf("browser: %s", page.Browser().MustVersion().Product)
			timed := page.Timeout(2 * time.Minute)
			defer timed.CancelTimeout()
			results := timed.MustEval(`async(project,frames,painter,file)=>{
				const canvas=document.createElement('canvas');canvas.width=project.width;canvas.height=project.height;canvas.setAttribute('data-vs-seek','');document.body.append(canvas);
				let current=0;
				const preview=window.VideoStudioPreview.mount(canvas,()=>project,()=>current,frame=>{current=frame;},null,()=>1,{painter,mediaURL:()=>'/media/'+file});
				const out=[];
				try{
					for(const frame of frames){
						preview.seek(frame);
						const settled=await preview.settle(15000);
						const shot=preview.capture({x:project.width/2-10,y:project.height/2-10,width:20,height:20});
						let sum=0;for(let i=1;i<shot.data.length;i+=4)sum+=shot.data[i];
						out.push({frame,settled,grey:sum/(shot.data.length/4),painter:preview.painterKind()});
					}
				}finally{preview.dispose();document.querySelectorAll('canvas[data-vs-seek]').forEach(node=>node.remove());}
				return out;
			}`, project, frames, painter, goldenSeekFile).Arr()
			var early []string
			for _, result := range results {
				frame, grey := result.Get("frame").Int(), result.Get("grey").Num()
				if !result.Get("settled").Bool() {
					t.Fatalf("frame %d did not settle", frame)
				}
				if kind := result.Get("painter").Str(); kind != painter {
					t.Fatalf("asked for the %s painter, drew with %s", painter, kind)
				}
				got, distance := goldenSeekNearest(levels, grey)
				switch {
				case distance > 6:
					t.Fatalf("frame %d shows grey %.1f, which is no frame of the evidence clip", frame, grey)
				case got == frame-1:
					early = append(early, strconv.Itoa(frame))
				case got != frame:
					t.Fatalf("frame %d shows source frame %d", frame, got)
				}
			}
			if len(early) > 0 {
				t.Fatalf("paused seeks to exact frame starts show the previous frame at frames %s: apply plan 2a Task 13 Step 9 (paused seeks to frame centres)", strings.Join(early, ", "))
			}
		})
	}
}

// A PNG that is still arriving already reports its size, but it is not complete and a WebGL upload of it leaves a blank
// texture (INVALID_VALUE, no exception). The preview must treat such an image as not ready and not settled: slow.png
// is served in two halves 1.5 s apart (640x360 noise around a solid 100x100 block), and the centre of the settled frame
// must show the block with both painters. Without the fix the GL frame was black, which flaked the compositor goldens
// under load.
func TestDesktopVideoStudioSlowImageBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	solid := color.NRGBA{R: 40, G: 160, B: 220, A: 255}
	img := image.NewNRGBA(image.Rect(0, 0, 640, 360))
	noise := rand.New(rand.NewSource(7))
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			if x >= 270 && x < 370 && y >= 130 && y < 230 {
				img.SetNRGBA(x, y, solid)
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(noise.Intn(256)), G: uint8(noise.Intn(256)), B: uint8(noise.Intn(256)), A: 255})
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	engine := newVideoStudioEngineServer(t, "")
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/media/slow.png" {
			engine.Config.Handler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data[:len(data)/2])
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(1500 * time.Millisecond)
		_, _ = w.Write(data[len(data)/2:])
	}))
	t.Cleanup(slow.Close)
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), slow)
	requireWebGL2(t, page)
	page.MustEval(`() => { Object.assign(window.vsGolden.files, {slow: 'slow.png'}); }`)
	project := engineTestProject([]map[string]any{engineTestAsset("slow", 640, 360)},
		engineTestTrack("v1", "video", engineTestClip("c1", "slow", 0, 50, nil)))
	for _, painter := range []string{"gl", "2d"} {
		got := renderEngineFrame(t, page, project, 0, painter)
		if got.Painter != painter || !got.Settled {
			t.Fatalf("%s: painter %q settled %v, want %s and settled", painter, got.Painter, got.Settled, painter)
		}
		if px := got.Image.RGBAAt(320, 180); !rgbaNear(px, color.RGBA{R: solid.R, G: solid.G, B: solid.B, A: 255}, 3) {
			t.Errorf("%s: centre pixel %v of the settled frame, want the solid block %v (the image was drawn before it finished loading)", painter, px, solid)
		}
	}
}
