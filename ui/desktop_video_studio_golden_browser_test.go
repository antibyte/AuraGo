package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

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
