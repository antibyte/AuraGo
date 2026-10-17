package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

// writeEffectsMarkerPNG writes a 64x36 opaque black PNG with a white 8x6 block at x 40..47, y 6..11 (the top-right
// quarter), so a crop or a flip moves the block to a known place.
func writeEffectsMarkerPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			pixel := color.NRGBA{A: 255}
			if x >= 40 && x < 48 && y >= 6 && y < 12 {
				pixel = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetNRGBA(x, y, pixel)
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

// TestDesktopVideoStudioEffectsGLBrowser drives the GL compositor directly on the engine page (SwiftShader) with
// hand-built scenes (plan 2c, Task 10; index §4.6–§4.8). It checks that:
//   - every 2c program compiles;
//   - a flat PNG through the colour pass and through the LUT pass matches VideoStudioPicture.applyColor and sampleLUT
//     within 3/255, and pending() counts the LUT loads;
//   - the LUT texture is rebuilt after a context loss;
//   - a crop moves a marker to where cropRect puts it, a flip mirrors it, and a keyframed crop draws what the static crop
//     draws;
//   - a fragment shader that does not compile passes its input through and is reported once as ('effect', id).
func TestDesktopVideoStudioEffectsGLBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := t.TempDir()
	source := color.RGBA{R: 180, G: 96, B: 48, A: 255}
	writeEngineTestPNG(t, filepath.Join(dir, "flat.png"), 64, 36, source, 0)
	writeEffectsMarkerPNG(t, filepath.Join(dir, "marker.png"))
	cube, ok := videostudio.BuiltinLookCube("teal_orange")
	if !ok {
		t.Fatal("videostudio.BuiltinLookCube(teal_orange) is missing")
	}
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	timed := page.Timeout(2 * time.Minute)
	defer timed.CancelTimeout()
	result := timed.MustEval(`async (cubeText) => {
		const C = window.VideoStudioCompositor, E = window.VideoStudioEffects, P = window.VideoStudioPicture, S = window.VideoStudioShaders;
		const out = {}, stage = document.getElementById('stage');
		const load = src => new Promise((resolve, reject) => { const img = new Image(); img.onload = () => resolve(img); img.onerror = () => reject(new Error('cannot load ' + src)); img.src = src; });
		const flat = await load('/media/flat.png'), marker = await load('/media/marker.png');
		const project = { version: 3, name: 'Effects', width: 64, height: 36, fps: 25, assets: [], tracks: [] };
		const asset = { id: 'a', kind: 'image', width: 64, height: 36 };
		let serial = 0;
		const clipOf = extra => Object.assign({ id: 'c' + (++serial), asset_id: 'a', start: 0, offset: 0, duration: 40, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain' }, extra || {});
		const effect = (id, params) => ({ id, on: true, params: Object.assign(E.defaults(id), params || {}) });
		const scene = (clip, el) => ({ project, frame: 0, width: 64, height: 36, layers: [{ trackId: 'v1', items: [{ clip, asset, el, ready: true, state: E.clipState(clip, 0, project) }], transition: null }] });
		// builtin:invert is a 2^3 table that inverts each channel (red fastest), so a skipped pass would be obvious.
		const invertText = 'LUT_3D_SIZE 2\n' + [0, 1].flatMap(b => [0, 1].flatMap(g => [0, 1].map(r => (1 - r) + ' ' + (1 - g) + ' ' + (1 - b)))).join('\n') + '\n';
		const tables = { 'builtin:teal_orange': P.parseCube(cubeText), 'builtin:invert': P.parseCube(invertText) };
		const failures = [];
		let wake = null;
		const resource = () => new Promise((resolve, reject) => {
			const timer = setTimeout(() => reject(new Error('no onResource within 10 s')), 10000);
			wake = () => { clearTimeout(timer); wake = null; resolve(); };
		});
		const hooks = {
			onProgramFailed: (kind, id) => failures.push([kind, id]),
			loadLUT: ref => new Promise((resolve, reject) => setTimeout(() => (tables[ref] ? resolve(tables[ref]) : reject(new Error('unknown LUT ' + ref))), 30)),
			onResource: () => { if (wake) wake(); }
		};
		const canvas = document.createElement('canvas'); canvas.width = 64; canvas.height = 36; stage.replaceChildren(canvas);
		const painter = C.create(canvas, hooks);
		if (!painter) return { created: false, error: C.lastError };
		out.created = true;
		const px = (x, y) => Array.from(painter.read({ x, y, width: 1, height: 1 }).data);
		const rgb = [180 / 255, 96 / 255, 48 / 255];
		const bytes = colour => colour.map(value => Math.round(Math.max(0, Math.min(1, value)) * 255)).concat([255]);

		// Every 2c effect in one stack. The LUT arrives through loadLUT, and pending() counts it until onResource fires.
		const all = clipOf({ effects: [effect('color', { exposure: 0.2 }), effect('lut', { lut: 'builtin:teal_orange' }), effect('blur', { radius: 0.02 }), effect('sharpen', { strength: 0.5 }), effect('vignette', { amount: 0.5 })] });
		let waiting = resource();
		painter.frame(scene(all, flat));
		out.pendingWhileLoading = painter.pending();
		await waiting;
		out.pendingAfterLoad = painter.pending();
		painter.frame(scene(all, flat));
		out.allFailures = failures.slice();

		// Colour: the centre follows applyColor of the clip's values at the frame.
		const colour = clipOf({ effects: [effect('color', { exposure: 0.35, contrast: 1.2, saturation: 1.3, temperature: 5200, tint: 0.15, lift: 0.03, lift_x: 0.4, lift_y: -0.2, gamma: 1.15, gamma_x: -0.3, gain: 0.95, gain_y: 0.35 })] });
		painter.frame(scene(colour, flat));
		out.colourGot = px(32, 18);
		out.colourWant = bytes(P.applyColor(E.clipState(colour, 0, project).effects[0].values, rgb));

		// LUT at intensity 0.6. teal_orange is cached by ref; invert loads once.
		painter.frame(scene(clipOf({ effects: [effect('lut', { lut: 'builtin:teal_orange', intensity: 0.6 })] }), flat));
		out.lutPending = painter.pending();
		out.lutGot = px(32, 18);
		out.lutWant = bytes(P.sampleLUT(tables['builtin:teal_orange'], rgb, 0.6));
		const invert = clipOf({ effects: [effect('lut', { lut: 'builtin:invert', intensity: 0.6 })] });
		waiting = resource();
		painter.frame(scene(invert, flat));
		out.invertPending = painter.pending();
		await waiting;
		painter.frame(scene(invert, flat));
		out.invertGot = px(32, 18);
		out.invertWant = bytes(P.sampleLUT(tables['builtin:invert'], rgb, 0.6));

		// A table with a narrow domain (0.25..0.75) is drawn through VideoStudioPicture.unitDomain, the copy the export
		// stages (R3), so the GL capture follows the resampled table.
		tables['builtin:narrow'] = P.parseCube('DOMAIN_MIN 0.25 0.25 0.25\nDOMAIN_MAX 0.75 0.75 0.75\nLUT_3D_SIZE 2\n' + [0, 1].flatMap(b => [0, 1].flatMap(g => [0, 1].map(r => [r, g, b].map(v => 0.25 + 0.5 * v).join(' ')))).join('\n') + '\n');
		const narrow = clipOf({ effects: [effect('lut', { lut: 'builtin:narrow' })] });
		waiting = resource();
		painter.frame(scene(narrow, flat));
		await waiting;
		painter.frame(scene(narrow, flat));
		out.narrowGot = px(32, 18);
		out.narrowWant = bytes(P.sampleLUT(P.unitDomain(tables['builtin:narrow']), rgb, 1));

		// A context loss drops the 3D texture; after the restore it is rebuilt from the kept table without a new load.
		// restoreContext() needs a pause after the lost event (index §4.12, browser test rules).
		const lose = canvas.getContext('webgl2').getExtension('WEBGL_lose_context');
		const next = name => new Promise((resolve, reject) => {
			const timer = setTimeout(() => reject(new Error('timed out waiting for ' + name)), 10000);
			canvas.addEventListener(name, event => { clearTimeout(timer); resolve(event); }, { once: true });
		});
		const lost = next('webglcontextlost'); lose.loseContext(); await lost;
		await new Promise(resolve => setTimeout(resolve, 50));
		const restored = next('webglcontextrestored'); lose.restoreContext(); await restored;
		painter.frame(scene(invert, flat));
		out.restoredGot = px(32, 18);
		out.restoredPending = painter.pending();

		// Crop l 0.5, b 0.5 keeps the top-right quarter. On the 64x36 canvas the marker's centre (44, 9) lands at
		// ((44 - 32) x 2, 9 x 2) = (24, 18). flip_h mirrors it to x 40. The same crop as keyframes draws the same frame:
		// PICTURE flips the whole picture, and the mirrored crop rect becomes uv.
		const crop = { l: 0.5, t: 0, r: 0, b: 0.5 };
		out.cropRect = P.cropRect(crop, 64, 36);
		painter.frame(scene(clipOf(), marker)); out.plain = [px(24, 18), px(44, 9)];
		painter.frame(scene(clipOf({ crop }), marker)); out.cropped = [px(24, 18), px(8, 8)];
		painter.frame(scene(clipOf({ crop, flip_h: true }), marker)); out.flipped = [px(40, 18), px(24, 18)];
		const staticFlip = Array.from(painter.read().data);
		const keyed = clipOf({ crop, flip_h: true, keyframes: { 'crop.l': [{ f: 0, v: 0.5, e: 'linear' }, { f: 40, v: 0.5, e: 'linear' }] } });
		out.animated = E.cropAnimated(keyed);
		painter.frame(scene(keyed, marker));
		const animatedFlip = Array.from(painter.read().data);
		out.animatedDiff = staticFlip.reduce((max, value, index) => Math.max(max, Math.abs(value - animatedFlip[index])), 0);
		out.cropFailures = failures.slice();
		painter.dispose();

		// A colour program that does not compile passes the picture through and is reported once.
		const original = S.effect;
		S.effect = id => (id === 'color' ? { passes: ['#version 300 es\nnot glsl'] } : original(id));
		try {
			const broken = [];
			const other = document.createElement('canvas'); other.width = 64; other.height = 36; stage.replaceChildren(other);
			const fallback = C.create(other, { onProgramFailed: (kind, id) => broken.push([kind, id]) });
			const clip = clipOf({ effects: [effect('color', { exposure: 1 })] });
			fallback.frame(scene(clip, flat)); fallback.frame(scene(clip, flat));
			out.brokenGot = Array.from(fallback.read({ x: 32, y: 18, width: 1, height: 1 }).data);
			out.brokenFailures = broken;
			fallback.dispose();
		} finally {
			S.effect = original;
		}
		stage.replaceChildren();
		return out;
	}`, string(cube))
	checks := result.Map()
	compact := func(name string) string { return strings.Join(strings.Fields(checks[name].JSON("", "")), "") }
	if !checks["created"].Bool() {
		t.Fatalf("the compositor did not start: %s", result.JSON("", ""))
	}
	for name, want := range map[string]int{"pendingWhileLoading": 1, "pendingAfterLoad": 0, "lutPending": 0, "invertPending": 1, "restoredPending": 0} {
		if got := checks[name].Int(); got != want {
			t.Errorf("%s = %d, want %d (pending() counts LUT loads in flight)", name, got, want)
		}
	}
	for _, name := range []string{"allFailures", "cropFailures"} {
		if got := compact(name); got != "[]" {
			t.Errorf("%s = %s: PICTURE and the five 2c effect programs must compile", name, got)
		}
	}
	for got, want := range map[string]string{"colourGot": "colourWant", "lutGot": "lutWant", "invertGot": "invertWant", "restoredGot": "invertWant", "narrowGot": "narrowWant"} {
		if g, w := jsonRGBA(checks[got]), jsonRGBA(checks[want]); !rgbaNear(g, w, 3) {
			t.Errorf("%s = %v, want %v ±3", got, g, w)
		}
	}
	// A skipped pass would leave the source colour. Both expectations are far from it.
	for _, name := range []string{"colourWant", "invertWant"} {
		if rgbaNear(jsonRGBA(checks[name]), source, 12) {
			t.Errorf("%s %v is within 12 of the source %v, so it cannot tell a pass from a pass-through", name, jsonRGBA(checks[name]), source)
		}
	}
	if rect := checks["cropRect"]; rect.Get("x").Int() != 32 || rect.Get("y").Int() != 0 || rect.Get("w").Int() != 32 || rect.Get("h").Int() != 18 {
		t.Errorf("cropRect(l 0.5, b 0.5, 64x36) = %s, want the 32x18 rect at (32, 0)", rect.JSON("", ""))
	}
	white, black := color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{A: 255}
	for name, want := range map[string][2]color.RGBA{"plain": {black, white}, "cropped": {white, black}, "flipped": {white, black}} {
		pixels := checks[name].Arr()
		if len(pixels) != 2 {
			t.Errorf("%s = %s", name, checks[name].JSON("", ""))
			continue
		}
		for i := range pixels {
			if got := jsonRGBA(pixels[i]); !rgbaNear(got, want[i], 8) {
				t.Errorf("%s pixel %d = %v, want %v (the marker did not land where cropRect puts it)", name, i, got, want[i])
			}
		}
	}
	if !checks["animated"].Bool() {
		t.Error("VideoStudioEffects.cropAnimated must report the crop.l keys")
	}
	if diff := checks["animatedDiff"].Int(); diff > 3 {
		t.Errorf("the keyframed crop with flip_h differs from the static one by %d/255, want at most 3", diff)
	}
	if got := jsonRGBA(checks["brokenGot"]); !rgbaNear(got, source, 3) {
		t.Errorf("a colour program that does not compile drew %v, want the input %v passed through", got, source)
	}
	if got := compact("brokenFailures"); got != `[["effect","color"]]` {
		t.Errorf("onProgramFailed calls = %s, want one ('effect', 'color')", got)
	}
}
