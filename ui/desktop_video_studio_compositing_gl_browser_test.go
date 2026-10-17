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

	"github.com/go-rod/rod"
	"github.com/ysmood/gson"
)

// writeFlatNRGBAPNG writes a width x height PNG of one straight-alpha colour, alpha included.
func writeFlatNRGBAPNG(t *testing.T, path string, width, height int, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
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

// compositingJSON is a JSON value without white space, for comparing lists such as the program failures.
func compositingJSON(v gson.JSON) string { return strings.Join(strings.Fields(v.JSON("", "")), "") }

// compositingGLPrelude declares what the /key, /mask and /blend scripts share. They drive VideoStudioCompositor on the
// engine page with hand-built scenes and read the composite through painter.read:
//   - load(name): /media/<name>.png as a decoded Image;
//   - sized(w, h): a version 3 project of that size; clipOf(extra): an image clip covering the canvas;
//   - effect(id, params): an effect entry with the registry defaults;
//   - itemOf(project, clip, el, frame): a scene item whose state comes from VideoStudioEffects.clipState, like the
//     preview's describeItem;
//   - sceneOf(project, layers, frame): layers in paint order, each {items, transition};
//   - painterFor(project, hooks): a compositor on a fresh canvas in #stage; pixel(painter, x, y): one RGBA pixel;
//   - bytes(rgb): a 0..1 colour as an opaque 8-bit RGBA pixel.
const compositingGLPrelude = `
const C = window.VideoStudioCompositor, E = window.VideoStudioEffects, P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const stage = document.getElementById('stage');
const load = name => new Promise((resolve, reject) => { const img = new Image(); img.onload = () => resolve(img); img.onerror = () => reject(new Error('cannot load ' + name)); img.src = '/media/' + name + '.png'; });
const sized = (width, height) => ({ version: 3, name: 'Compositing', width, height, fps: 25, assets: [], tracks: [] });
let serial = 0;
const clipOf = extra => Object.assign({ id: 'c' + (++serial), asset_id: 'a', start: 0, offset: 0, duration: 40, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain' }, extra || {});
const effect = (id, params) => ({ id, on: true, params: Object.assign(E.defaults(id), params || {}) });
const itemOf = (project, clip, el, frame) => ({ clip, asset: { id: clip.asset_id, kind: 'image', width: el.naturalWidth, height: el.naturalHeight }, el, ready: true, state: E.clipState(clip, frame || 0, project) });
const sceneOf = (project, layers, frame) => ({ project, frame: frame || 0, width: project.width, height: project.height, layers: layers.map((layer, index) => Object.assign({ trackId: 'v' + (index + 1), items: [], transition: null }, layer)) });
const painterFor = (project, hooks) => { const canvas = document.createElement('canvas'); canvas.width = project.width; canvas.height = project.height; stage.replaceChildren(canvas); return C.create(canvas, hooks || {}); };
const pixel = (painter, x, y) => Array.from(painter.read({ x, y, width: 1, height: 1 }).data);
const bytes = rgb => rgb.map(value => Math.round(Math.max(0, Math.min(1, value)) * 255)).concat([255]);
`

// evalCompositingGL runs body inside an async function after compositingGLPrelude and returns the object it resolves
// to. A body returns {created: false, error} when the compositor does not start.
func evalCompositingGL(t *testing.T, page *rod.Page, body string) map[string]gson.JSON {
	t.Helper()
	timed := page.Timeout(3 * time.Minute)
	defer timed.CancelTimeout()
	result, err := timed.Eval("async () => {" + compositingGLPrelude + body + "\n}")
	if err != nil {
		t.Fatalf("compositing script: %v", err)
	}
	checks := result.Value.Map()
	if !checks["created"].Bool() {
		t.Fatalf("the compositor did not start: %s", result.Value.JSON("", ""))
	}
	return checks
}

// TestDesktopVideoStudioCompositingGLBrowser drives the GL compositor on the engine page (SwiftShader) with hand-built
// scenes for plan 2d (index §4.7, §4.8, §4.22). Each subtest checks three things: its programs compile; the captured
// pixels match the real forms of VideoStudioPicture within 3/255; and a test-injected broken shader passes its input
// through and is reported once:
//   - key (Task 9): the KEY pass against keyAlpha and despill;
//   - mask (Task 10): PLACE_MASKED against maskAt in box space.
func TestDesktopVideoStudioCompositingGLBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := t.TempDir()
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	t.Run("key", func(t *testing.T) { testCompositingGLKey(t, page, dir) })
	t.Run("mask", func(t *testing.T) { testCompositingGLMask(t, page, dir) })
}

// compositingGLKeyScript keys flat 64x36 patches with chroma_key and reads the centre over black and over white.
const compositingGLKeyScript = `
const project = sized(64, 36), out = {}, failures = [];
const [patch, still, green, blueScreen, white] = await Promise.all(['key_patch', 'key_still', 'key_green', 'key_blue', 'key_white'].map(load));
const painter = painterFor(project, { onProgramFailed: (kind, id) => failures.push([kind, id]) });
if (!painter) return { created: false, error: C.lastError };
out.created = true;
const under = clipOf();
// One keyed clip at frame 0, drawn over opaque black and over a white track:
// - over black the capture is despill(c) x k; over white it is despill(c) x k + (1 - k), so the pair pins the despilled
//   colour and the key alpha;
// - k includes the picture's own alpha (the key multiplies it);
// - plainWant leaves out despill; replaceWant lets the key replace the picture's alpha instead of multiplying it.
const keyCase = (el, rgb, alpha, params) => {
  const clip = clipOf({ effects: [effect('chroma_key', params)] });
  const values = E.clipState(clip, 0, project).effects[0].values;
  const key = P.keyAlpha(values, rgb), k = key * alpha, colour = P.despill(values, rgb);
  painter.frame(sceneOf(project, [{ items: [itemOf(project, clip, el)] }]));
  const overBlack = pixel(painter, 32, 18);
  painter.frame(sceneOf(project, [{ items: [itemOf(project, under, white)] }, { items: [itemOf(project, clip, el)] }]));
  return {
    k, overBlack, overWhite: pixel(painter, 32, 18),
    overBlackWant: bytes(colour.map(v => v * k)), overWhiteWant: bytes(colour.map(v => v * k + 1 - k)),
    plainWant: bytes(rgb.map(v => v * k)), replaceWant: bytes(colour.map(v => v * key + 1 - key))
  };
};
const patchRGB = [132 / 255, 177 / 255, 64 / 255];
const greenKey = { color: '#00b140', similarity: 0.25, smoothness: 0.1, spill: 0.5 };
// d = 132 / 255 / sqrt(3) = 0.299: on the smoothness ramp (k about 0.49), green spill 79 levels.
out.patch = keyCase(patch, patchRGB, 1, greenKey);
// Smoothness 0 is colorkey's step: d > similarity keeps the patch (k = 1), despill still applies.
out.hard = keyCase(patch, patchRGB, 1, Object.assign({}, greenKey, { smoothness: 0 }));
// A still with alpha 128: the key multiplies it (k about 0.5 x 0.5).
out.still = keyCase(still, patchRGB, 128 / 255, Object.assign({}, greenKey, { similarity: 0.15, smoothness: 0.3 }));
// The key colour itself is fully transparent.
out.screen = keyCase(green, [0, 177 / 255, 64 / 255], 1, greenKey);
// A blue screen: d = 0.146, k about 0.46, and despill works on blue.
out.blue = keyCase(blueScreen, [60 / 255, 90 / 255, 200 / 255], 1, { color: '#0047bb', similarity: 0.1, smoothness: 0.1, spill: 0.5 });
out.failures = failures.slice();
painter.dispose();
// A KEY program that does not compile passes the picture through and is reported once.
const original = S.effect;
S.effect = id => (id === 'chroma_key' ? { passes: ['#version 300 es\nnot glsl'] } : original(id));
try {
  const broken = [];
  const fallback = painterFor(project, { onProgramFailed: (kind, id) => broken.push([kind, id]) });
  const clip = clipOf({ effects: [effect('chroma_key', greenKey)] });
  fallback.frame(sceneOf(project, [{ items: [itemOf(project, clip, patch)] }]));
  fallback.frame(sceneOf(project, [{ items: [itemOf(project, clip, patch)] }]));
  out.brokenGot = pixel(fallback, 32, 18);
  out.brokenFailures = broken;
  fallback.dispose();
} finally {
  S.effect = original;
}
stage.replaceChildren();
return out;
`

// testCompositingGLKey is the /key subtest (plan 2d Task 9): KEY against VideoStudioPicture.keyAlpha and despill.
func testCompositingGLKey(t *testing.T, page *rod.Page, dir string) {
	for name, fill := range map[string]color.NRGBA{
		"key_patch": {R: 132, G: 177, B: 64, A: 255},
		"key_still": {R: 132, G: 177, B: 64, A: 128},
		"key_green": {R: 0, G: 177, B: 64, A: 255},
		"key_blue":  {R: 60, G: 90, B: 200, A: 255},
		"key_white": {R: 255, G: 255, B: 255, A: 255},
	} {
		writeFlatNRGBAPNG(t, filepath.Join(dir, name+".png"), 64, 36, fill)
	}
	checks := evalCompositingGL(t, page, compositingGLKeyScript)
	if got := compositingJSON(checks["failures"]); got != "[]" {
		t.Errorf("program failures = %s, want none (KEY must compile)", got)
	}
	for _, name := range []string{"patch", "hard", "still", "screen", "blue"} {
		for _, side := range []string{"overBlack", "overWhite"} {
			if got, want := jsonRGBA(checks[name].Get(side)), jsonRGBA(checks[name].Get(side+"Want")); !rgbaNear(got, want, 3) {
				t.Errorf("%s %s = %v, want %v ±3 (VideoStudioPicture keyAlpha and despill)", name, side, got, want)
			}
		}
	}
	// Every case must be able to fail: soft key alphas sit on the ramp, the step and the key colour hit the ends,
	// despill moves the colour, and multiplying the still's alpha differs from replacing it.
	for name, bounds := range map[string][2]float64{"patch": {0.2, 0.8}, "blue": {0.2, 0.8}, "hard": {1, 1}, "screen": {0, 0}} {
		if k := checks[name].Get("k").Num(); k < bounds[0] || k > bounds[1] {
			t.Errorf("%s key alpha = %.4f, want %v..%v", name, k, bounds[0], bounds[1])
		}
	}
	for _, name := range []string{"patch", "blue"} {
		if rgbaNear(jsonRGBA(checks[name].Get("overBlackWant")), jsonRGBA(checks[name].Get("plainWant")), 12) {
			t.Errorf("%s: despill moves the colour by at most 12 levels, so the case cannot catch a missing despill", name)
		}
	}
	if rgbaNear(jsonRGBA(checks["still"].Get("overWhiteWant")), jsonRGBA(checks["still"].Get("replaceWant")), 12) {
		t.Error("still: multiplying and replacing the alpha differ by at most 12 levels, so the case cannot tell them apart")
	}
	source := color.RGBA{R: 132, G: 177, B: 64, A: 255}
	if rgbaNear(jsonRGBA(checks["patch"].Get("overBlackWant")), source, 12) {
		t.Error("patch: the keyed colour is within 12 of the source, so the case cannot tell the pass from a pass-through")
	}
	if got := jsonRGBA(checks["brokenGot"]); !rgbaNear(got, source, 3) {
		t.Errorf("a KEY program that does not compile drew %v, want the input %v passed through", got, source)
	}
	if got := compositingJSON(checks["brokenFailures"]); got != `[["effect","chroma_key"]]` {
		t.Errorf("onProgramFailed calls = %s, want one ('effect', 'chroma_key')", got)
	}
}

// compositingGLMaskScript draws masked white clips on a 128x72 canvas and compares every pixel with maskAt.
const compositingGLMaskScript = `
const project = sized(128, 72), W = 128, H = 72, out = {}, failures = [];
const [white, square, red, blue] = await Promise.all(['mask_white', 'mask_square', 'mask_red', 'mask_blue'].map(load));
const painter = painterFor(project, { onProgramFailed: (kind, id) => failures.push([kind, id]) });
if (!painter) return { created: false, error: C.lastError };
out.created = true;
// The largest channel difference between the capture and want(x, y), with rgb in 0..1 at the pixel centre (x, y),
// over the whole canvas, plus the pixel and channel where it occurs.
const compare = want => {
  const data = painter.read().data;
  let max = 0, at = null;
  for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
    const rgb = want(x + 0.5, y + 0.5);
    for (let c = 0; c < 3; c++) {
      const d = Math.abs(data[(y * W + x) * 4 + c] - Math.round(Math.max(0, Math.min(1, rgb[c])) * 255));
      if (d > max) { max = d; at = [x, y, c]; }
    }
  }
  return { max, at };
};
const grey = m => [m, m, m];
// A static rotated ellipse on a full-canvas box, so box px are canvas px. band counts the pixels on the edge ramp.
const ellipse = { shape: 'ellipse', x: 0.2, y: 0.15, w: 0.5, h: 0.6, rotation: 20, feather: 0.1, invert: false };
const staticItem = itemOf(project, clipOf({ mask: ellipse }), white);
painter.frame(sceneOf(project, [{ items: [staticItem] }]));
const staticMask = P.maskFrame(staticItem.state.mask, { width: W, height: H });
let band = 0;
out.static = compare((x, y) => { const m = P.maskAt(staticMask, x, y); if (m > 0.02 && m < 0.98) band++; return grey(m); });
out.staticBand = band;
out.staticCentre = pixel(painter, Math.floor(staticMask.cx), Math.floor(staticMask.cy));
out.staticCorner = pixel(painter, 1, 1);
// Keyed mask.x and mask.feather at frame 20: the state holds the keys' values (x 0.3, feather 0.11), and PLACE_MASKED
// draws them like a static mask.
const keyedItem = itemOf(project, clipOf({ mask: Object.assign({}, ellipse, { x: 0.25, w: 0.4 }), keyframes: { 'mask.x': [{ f: 0, v: 0.1, e: 'linear' }, { f: 40, v: 0.5, e: 'linear' }], 'mask.feather': [{ f: 0, v: 0.02, e: 'linear' }, { f: 40, v: 0.2, e: 'linear' }] } }), white, 20);
out.keyedX = keyedItem.state.mask.x;
out.keyedAtFrame = Math.abs(keyedItem.state.mask.x - 0.3) < 1e-9 && Math.abs(keyedItem.state.mask.feather - 0.11) < 1e-9;
painter.frame(sceneOf(project, [{ items: [keyedItem] }], 20));
const keyedMask = P.maskFrame(keyedItem.state.mask, { width: W, height: H });
out.keyed = compare((x, y) => grey(P.maskAt(keyedMask, x, y)));
// A clip rotated by 30° with a rectangle rotated by 15°: the mask turns with the box. want maps each canvas pixel back
// into box px (the inverse of the quad's clockwise rotation about the box centre); outside the box nothing is drawn.
// The feather is wide (F = 3.6 px): SwiftShader interpolates v_box on a rotated quad with a position error of up to
// 0.03 px (0.014 px on average, inwards; none on an axis-aligned quad), which a 1.8 px ramp (feather 0.05) turns into
// 4/255. Rotation and placement errors would still show as dozens of levels.
const rect = { shape: 'rect', x: 0.2, y: 0.2, w: 0.6, h: 0.6, rotation: 15, feather: 0.1, invert: false };
const rotatedItem = itemOf(project, clipOf({ x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 30, mask: rect }), white);
painter.frame(sceneOf(project, [{ items: [rotatedItem] }]));
const turned = E.placement(rotatedItem.state, { width: W, height: H }, project, { width: W, height: H });
const tb = turned.box, cos = Math.cos(turned.angle), sin = Math.sin(turned.angle);
const rotatedMask = P.maskFrame(rotatedItem.state.mask, { width: tb.w, height: tb.h });
out.rotated = compare((x, y) => {
  const dx = x - (tb.x + tb.w / 2), dy = y - (tb.y + tb.h / 2);
  const bx = dx * cos + dy * sin + tb.w / 2, by = -dx * sin + dy * cos + tb.h / 2;
  return grey(bx >= 0 && bx <= tb.w && by >= 0 && by <= tb.h ? P.maskAt(rotatedMask, bx, by) : 0);
});
// Contain letterbox: a square picture in the full-canvas box over red, with an inverted ellipse. The mask lives in box
// px, not in the picture's, and the letterbox stays red because only the fitted picture is drawn.
const invert = { shape: 'ellipse', x: 0.3, y: 0.2, w: 0.4, h: 0.6, rotation: 0, feather: 0.05, invert: true };
const squareItem = itemOf(project, clipOf({ mask: invert }), square);
painter.frame(sceneOf(project, [{ items: [itemOf(project, clipOf(), red)] }, { items: [squareItem] }]));
const fitted = E.placement(squareItem.state, { width: W, height: H }, project, { width: 72, height: 72 });
const fd = fitted.draw, invertMask = P.maskFrame(squareItem.state.mask, { width: fitted.box.w, height: fitted.box.h });
out.drawRect = [fd.x, fd.y, fd.w, fd.h];
out.letterbox = compare((x, y) => {
  const m = x >= fd.x && x < fd.x + fd.w && y >= fd.y && y < fd.y + fd.h ? P.maskAt(invertMask, x - fitted.box.x, y - fitted.box.y) : 0;
  return [1, m, m];
});
out.letterboxPixel = pixel(painter, 4, 36);
// A masked clip dissolving into an unmasked blue one at p = 0.5. The pair mixes premultiplied, so the capture over
// black is (0.5 m, 0.5 m, 0.5 m + 0.5).
const outItem = itemOf(project, clipOf({ duration: 20, mask: ellipse }), white, 15), incItem = itemOf(project, clipOf({ start: 10, duration: 20 }), blue, 15);
painter.frame(sceneOf(project, [{ transition: { type: 'dissolve', progress: 0.5, out: outItem, inc: incItem } }], 15));
const pairMask = P.maskFrame(outItem.state.mask, { width: W, height: H });
out.transition = compare((x, y) => { const m = P.maskAt(pairMask, x, y); return [0.5 * m, 0.5 * m, 0.5 * m + 0.5]; });
out.failures = failures.slice();
painter.dispose();
// A PLACE_MASKED program that does not compile draws the clip unmasked and is reported once.
const original = S.PLACE_MASKED;
S.PLACE_MASKED = '#version 300 es\nnot glsl';
try {
  const broken = [];
  const fallback = painterFor(project, { onProgramFailed: (kind, id) => broken.push([kind, id]) });
  fallback.frame(sceneOf(project, [{ items: [staticItem] }]));
  fallback.frame(sceneOf(project, [{ items: [staticItem] }]));
  out.brokenCorner = pixel(fallback, 1, 1);
  out.brokenFailures = broken;
  fallback.dispose();
} finally {
  S.PLACE_MASKED = original;
}
stage.replaceChildren();
return out;
`

// testCompositingGLMask is the /mask subtest (plan 2d Task 10): PLACE_MASKED against VideoStudioPicture.maskAt, in
// box space, for a static, a keyed, a rotated, a letterboxed and a transition case.
func testCompositingGLMask(t *testing.T, page *rod.Page, dir string) {
	media := map[string]struct {
		width, height int
		fill          color.NRGBA
	}{
		"mask_white":  {128, 72, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		"mask_square": {72, 72, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		"mask_red":    {128, 72, color.NRGBA{R: 255, A: 255}},
		"mask_blue":   {128, 72, color.NRGBA{B: 255, A: 255}},
	}
	for name, m := range media {
		writeFlatNRGBAPNG(t, filepath.Join(dir, name+".png"), m.width, m.height, m.fill)
	}
	checks := evalCompositingGL(t, page, compositingGLMaskScript)
	if got := compositingJSON(checks["failures"]); got != "[]" {
		t.Errorf("program failures = %s, want none (PLACE_MASKED must compile)", got)
	}
	for _, name := range []string{"static", "keyed", "rotated", "letterbox", "transition"} {
		diff := checks[name].Get("max").Int()
		t.Logf("%s: largest difference from maskAt %d/255", name, diff)
		if diff > 3 {
			t.Errorf("%s: the capture differs from maskAt by %d/255 at %s (x, y, channel), want at most 3", name, diff, compositingJSON(checks[name].Get("at")))
		}
	}
	if band := checks["staticBand"].Int(); band < 200 {
		t.Errorf("only %d pixels lie on the static edge ramp, want at least 200 (the case must test the feather)", band)
	}
	if !checks["keyedAtFrame"].Bool() {
		t.Errorf("keyed mask.x at frame 20 = %v, want 0.3 and feather 0.11 (the keys, not the static field)", checks["keyedX"].Num())
	}
	white, black, red := color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{A: 255}, color.RGBA{R: 255, A: 255}
	for name, want := range map[string]color.RGBA{"staticCentre": white, "staticCorner": black, "letterboxPixel": red, "brokenCorner": white} {
		if got := jsonRGBA(checks[name]); !rgbaNear(got, want, 3) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	if got := compositingJSON(checks["drawRect"]); got != "[28,0,72,72]" {
		t.Errorf("contain draw rect = %s, want [28,0,72,72] (the letterbox case needs side bars)", got)
	}
	if got := compositingJSON(checks["brokenFailures"]); got != `[["effect","mask"]]` {
		t.Errorf("onProgramFailed calls = %s, want one ('effect', 'mask')", got)
	}
}
