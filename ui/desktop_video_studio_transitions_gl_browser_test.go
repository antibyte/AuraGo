package ui

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// transitionsGLScript draws every row of the xfade probe fixture (plan 2e Task 10) with the shipped transition sources.
//   - The two layers are textures of the fixture's own patterns, uploaded byte for byte. These are the premultiplied
//     values xfade saw.
//   - The uniforms come from VideoStudioPicture.transitionUniforms, as drawTransition passes them.
//   - The result is read without blending and compared with every sample.
//
// It also reads the D3 quirk pixels with both layers' values, and draws a broken circleopen source through the
// compositor.
const transitionsGLScript = `
const S = window.VideoStudioShaders, P = window.VideoStudioPicture, E = window.VideoStudioEffects, G = window.VideoStudioGL, C = window.VideoStudioCompositor;
const fixture = JSON.parse(fixtureText), stage = document.getElementById('stage');
const out = { created: false, rows: [], missing: [], failures: [], quirks: {} };
const canvas = document.createElement('canvas'); canvas.width = 2; canvas.height = 2; stage.replaceChildren(canvas);
const ctx = G.create(canvas, {});
if (!ctx) return out;
out.created = true;
// The fixture's patterns (Go xfadePatterns) as RGBA bytes.
// - Plane: (c0·x + c1·y + c2) mod 256.
// - The alpha variant: α = 64 + ((a0·x + a1·y + a2) mod 192), colour floor(plane·α/255).
const patternBytes = (side, variant, W, H) => {
  const spec = fixture.patterns[side], data = new Uint8ClampedArray(W * H * 4);
  for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
    const alpha = variant === 'alpha' ? 64 + (spec.alpha[0] * x + spec.alpha[1] * y + spec.alpha[2]) % 192 : 255;
    const value = c => { const v = (c[0] * x + c[1] * y + c[2]) % 256; return variant === 'alpha' ? Math.floor(v * alpha / 255) : v; };
    const i = (y * W + x) * 4;
    data[i] = value(spec.planes[2]); data[i + 1] = value(spec.planes[0]); data[i + 2] = value(spec.planes[1]); data[i + 3] = alpha;
  }
  return data;
};
const layers = new Map();
const layer = (side, variant, W, H) => {
  const key = [side, variant, W, H].join(':');
  if (!layers.has(key)) {
    const bytes = patternBytes(side, variant, W, H), texture = ctx.texture();
    if (!texture.upload(new ImageData(bytes, W, H))) throw new Error('cannot upload ' + key);
    layers.set(key, { texture, bytes });
  }
  return layers.get(key);
};
const programs = new Map();
const programFor = type => {
  if (!programs.has(type)) {
    const source = S.transition(type), program = source ? ctx.program('transition:' + type, S.QUAD_VERTEX, source) : null;
    if (!source) out.missing.push(type); else if (!program) out.failures.push(type);
    programs.set(type, program);
  }
  return programs.get(type);
};
const pixelOf = (data, W, x, y) => Array.from(data.subarray((y * W + x) * 4, (y * W + x) * 4 + 4));
for (const row of fixture.rows) {
  const [W, H] = row.size, program = programFor(row.type);
  if (!program) continue;
  const a = layer('a', row.variant, W, H), b = layer('b', row.variant, W, H), target = ctx.target(W, H);
  const uniforms = Object.assign({ u_flipY: 0, u_size: [W, H] }, P.transitionUniforms(row.type, row.i, row.D, { width: W, height: H }));
  ctx.draw(program, target, uniforms, { u_from: a.texture, u_to: b.texture });
  const data = ctx.read(target);
  ctx.release(target);
  const edges = new Set(row.edges || []);
  let max = 0, edgeMax = 0, at = null;
  row.samples.forEach((sample, k) => {
    const got = pixelOf(data, W, sample[0], sample[1]);
    got.forEach((value, c) => {
      const d = Math.abs(value - sample[2 + c]);
      if (d > max) { max = d; at = { x: sample[0], y: sample[1], got, want: sample.slice(2), edge: edges.has(k) }; }
      if (edges.has(k)) edgeMax = Math.max(edgeMax, d);
    });
  });
  out.rows.push({ type: row.type, variant: row.variant, size: row.size, D: row.D, i: row.i, samples: row.samples.length, edges: edges.size, max, edgeMax, at });
  // The D3 quirk pixels (index §4.24) on the 96x54 opaque rows, with both layers' values at the same pixel.
  if (row.variant === 'opaque' && W === 96) {
    const cx = W >> 1, cy = H >> 1, q = (x, y) => ({ got: pixelOf(data, W, x, y), a: pixelOf(a.bytes, W, x, y), b: pixelOf(b.bytes, W, x, y) });
    if (row.type === 'wipeleft' && row.i === 0) Object.assign(out.quirks, { wipeleftColumn0: q(0, cy), wipeleftColumn1: q(1, cy) });
    if (row.type === 'wipedown' && row.i === 0) Object.assign(out.quirks, { wipedownRow0: q(cx, 0), wipedownRow1: q(cx, 1) });
    if (row.type === 'rectcrop' && row.i === 0) Object.assign(out.quirks, { rectcropLeft: q(0, cy), rectcropTop: q(cx, 0), rectcropInside: q(1, cy) });
    if (row.type === 'radial' && row.i === row.D - 1) out.quirks.radialWedge = q(cx - 1, 0);
  }
}
layers.forEach(entry => entry.texture.dispose());
ctx.dispose();
// A 2e source that does not compile: a hard cut at p >= 0.5, reported once as ('transition', id) (index §4.5).
const load = name => new Promise((resolve, reject) => { const img = new Image(); img.onload = () => resolve(img); img.onerror = () => reject(new Error('cannot load ' + name)); img.src = '/media/' + name + '.png'; });
const [red, blue] = await Promise.all(['tx_red', 'tx_blue'].map(load));
const project = { version: 3, name: 'Transitions', width: 64, height: 36, fps: 25, assets: [], tracks: [] };
const clipOf = (id, start, transition) => ({ id, asset_id: id, start, offset: 0, duration: 20, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', transition: transition || null });
const outClip = clipOf('red', 0, { type: 'circleopen', duration: 10 }), incClip = clipOf('blue', 10);
const itemOf = (clip, el, frame) => ({ clip, asset: { id: clip.asset_id, kind: 'image', width: 64, height: 36 }, el, ready: true, state: E.clipState(clip, frame, project) });
const sceneAt = index => ({ project, frame: 10 + index, width: 64, height: 36, layers: [{ trackId: 'v1', items: [], transition: { type: 'circleopen', progress: index / 10, index, duration: 10, out: itemOf(outClip, red, 10 + index), inc: itemOf(incClip, blue, 10 + index) } }] });
const original = S.transition;
S.transition = id => (id === 'circleopen' ? '#version 300 es\nnot glsl' : original(id));
try {
  const broken = [], other = document.createElement('canvas'); other.width = 64; other.height = 36; stage.replaceChildren(other);
  const painter = C.create(other, { onProgramFailed: (kind, id) => broken.push([kind, id]) });
  painter.frame(sceneAt(3)); out.cutEarly = Array.from(painter.read({ x: 32, y: 18, width: 1, height: 1 }).data);
  painter.frame(sceneAt(7)); out.cutLate = Array.from(painter.read({ x: 32, y: 18, width: 1, height: 1 }).data);
  out.brokenFailures = broken;
  painter.dispose();
} finally {
  S.transition = original;
}
stage.replaceChildren();
return out;
`

// TestDesktopVideoStudioTransitionsGLBrowser checks the GL transition sources against FFmpeg 8.1.1's xfade (the probe
// fixture, plan 2e Task 10; index §4.7, §4.24) on SwiftShader:
//   - every registry transition compiles;
//   - every sample of every row is within ±2/255 on premultiplied values. The opaque layers differ by 128 in red, so ±2
//     on both pixels of each D1 edge also pins the A/B choice there;
//   - the D3 quirks hold: frame 0 of wipeleft and wipedown shows 1 px of B, rectcrop shows a black border at frame 0,
//     and radial keeps a wedge of A at the last frame;
//   - a source that does not compile draws a hard cut and is reported once. The stage turns that report into
//     previewTransitionFallback (TestVideoStudioPreviewEffectNotice).
func TestDesktopVideoStudioTransitionsGLBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture, err := os.ReadFile(filepath.Join("..", "internal", "videostudio", "testdata", "xfade", "probes.json"))
	if err != nil {
		t.Fatalf("xfade probe fixture (plan 2e Task 10): %v", err)
	}
	var parsed struct {
		Rows []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(fixture, &parsed); err != nil || len(parsed.Rows) == 0 {
		t.Fatalf("xfade probe fixture rows: %v (%d rows)", err, len(parsed.Rows))
	}
	dir := t.TempDir()
	writeFlatNRGBAPNG(t, filepath.Join(dir, "tx_red.png"), 64, 36, color.NRGBA{R: 255, A: 255})
	writeFlatNRGBAPNG(t, filepath.Join(dir, "tx_blue.png"), 64, 36, color.NRGBA{B: 255, A: 255})
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	timed := page.Timeout(4 * time.Minute)
	defer timed.CancelTimeout()
	result, err := timed.Eval("async (fixtureText) => {"+transitionsGLScript+"\n}", string(fixture))
	if err != nil {
		t.Fatalf("transitions script: %v", err)
	}
	checks := result.Value.Map()
	if !checks["created"].Bool() {
		t.Fatalf("no WebGL2 helper: %s", result.Value.JSON("", ""))
	}
	for _, name := range []string{"missing", "failures"} {
		if got := compositingJSON(checks[name]); got != "[]" {
			t.Errorf("%s = %s, want none (every registry transition has a source that compiles)", name, got)
		}
	}
	rows := checks["rows"].Arr()
	if len(rows) != len(parsed.Rows) {
		t.Errorf("drew %d of the %d fixture rows", len(rows), len(parsed.Rows))
	}
	// These types copy a texel of one layer unchanged, so their edge samples must match FFmpeg exactly (pixelize
	// blends by P and keeps the ±2 bound).
	copies := []string{"wipeleft", "wiperight", "wipeup", "wipedown", "slideleft", "slideright", "slideup", "slidedown", "squeezeh", "circlecrop", "rectcrop"}
	worst, edged := map[string]int{}, map[string]bool{}
	for _, row := range rows {
		typ, diff := row.Get("type").Str(), row.Get("max").Int()
		worst[typ] = max(worst[typ], diff)
		if row.Get("edges").Int() > 0 {
			edged[typ] = true
		}
		if diff > 2 {
			t.Errorf("%s %s %s D=%d i=%d: %d/255 off FFmpeg at %s, want ±2 on premultiplied values (on an edge sample: the A/B choice moved)",
				typ, row.Get("variant").Str(), compositingJSON(row.Get("size")), row.Get("D").Int(), row.Get("i").Int(), diff, compositingJSON(row.Get("at")))
		}
		if edge := row.Get("edgeMax").Int(); edge != 0 && slices.Contains(copies, typ) {
			t.Errorf("%s %s %s D=%d i=%d: an edge sample is %d/255 off FFmpeg, want exact (a copied texel on the right side of the edge)",
				typ, row.Get("variant").Str(), compositingJSON(row.Get("size")), row.Get("D").Int(), row.Get("i").Int(), edge)
		}
	}
	for _, typ := range []string{"wipeleft", "wiperight", "wipeup", "wipedown", "slideleft", "slideright", "slideup", "slidedown", "circlecrop", "rectcrop", "pixelize", "squeezeh"} {
		if !edged[typ] {
			t.Errorf("no %s row carries edge samples, so its A/B choice is never checked", typ)
		}
	}
	names := make([]string, 0, len(worst))
	for name := range worst {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		t.Logf("%s: largest difference from FFmpeg 8.1.1 %d/255", name, worst[name])
	}
	quirk := func(name string) (got, a, b color.RGBA) {
		q := checks["quirks"].Get(name)
		return jsonRGBA(q.Get("got")), jsonRGBA(q.Get("a")), jsonRGBA(q.Get("b"))
	}
	for _, name := range []string{"wipeleftColumn0", "wipedownRow0"} {
		if got, a, b := quirk(name); !rgbaNear(got, b, 2) || rgbaNear(got, a, 2) {
			t.Errorf("%s = %v, want the incoming %v at frame 0 (1 px of B, §4.24 D3), not the outgoing %v", name, got, b, a)
		}
	}
	for _, name := range []string{"wipeleftColumn1", "wipedownRow1", "rectcropInside"} {
		if got, a, _ := quirk(name); !rgbaNear(got, a, 2) {
			t.Errorf("%s = %v, want the outgoing %v at frame 0", name, got, a)
		}
	}
	for _, name := range []string{"rectcropLeft", "rectcropTop"} {
		if got, _, _ := quirk(name); !rgbaNear(got, color.RGBA{A: 255}, 2) {
			t.Errorf("%s = %v, want opaque black (rectcrop's 1 px border at frame 0)", name, got)
		}
	}
	if got, _, b := quirk("radialWedge"); rgbaNear(got, b, 8) {
		t.Errorf("radialWedge = %v is within 8 of the incoming %v: radial's last frame keeps a wedge of A left of 12 o'clock", got, b)
	}
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	if got := jsonRGBA(checks["cutEarly"]); !rgbaNear(got, red, 3) {
		t.Errorf("broken circleopen at p = 0.3 drew %v, want the outgoing red (hard cut)", got)
	}
	if got := jsonRGBA(checks["cutLate"]); !rgbaNear(got, blue, 3) {
		t.Errorf("broken circleopen at p = 0.7 drew %v, want the incoming blue (hard cut)", got)
	}
	if got := compositingJSON(checks["brokenFailures"]); got != `[["transition","circleopen"]]` {
		t.Errorf("onProgramFailed calls = %s, want one ('transition', 'circleopen')", got)
	}
}
