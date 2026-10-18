package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ysmood/gson"
)

// writeAnimMarkerPNG writes the 320x180 blur subject of the blur_in cases: opaque black with a white 64x40 block at
// (200, 40) and a white 1 px line at x = 100 from y = 20 to 159, so both the wide and the narrow blur node show.
func writeAnimMarkerPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 320, 180))
	for y := range 180 {
		for x := range 320 {
			pixel := color.NRGBA{A: 255}
			if (x >= 200 && x < 264 && y >= 40 && y < 80) || (x == 100 && y >= 20 && y < 160) {
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

// animGLScript drives the GL compositor and the 2D painter on the engine page with hand-built scenes whose clip
// states come from VideoStudioEffects.clipState (state.anim included, Task 5).
const animGLScript = `
const C = window.VideoStudioCompositor, E = window.VideoStudioEffects, P = window.VideoStudioPicture, S = window.VideoStudioShaders, A = window.VideoStudioAnim, D2 = window.VideoStudioPainter2D;
const stage = document.getElementById('stage');
const load = name => new Promise((resolve, reject) => { const img = new Image(); img.onload = () => resolve(img); img.onerror = () => reject(new Error('cannot load ' + name)); img.src = '/media/' + name + '.png'; });
const [white, marker, red, blue] = await Promise.all(['anim_white', 'anim_marker', 'anim_red', 'anim_blue'].map(load));
const sized = (width, height) => ({ version: 3, name: 'Animation', width, height, fps: 25, assets: [], tracks: [] });
const big = sized(320, 180), wideProject = sized(640, 360), source = { width: 160, height: 90 };
let serial = 0;
const clipOf = extra => Object.assign({ id: 'c' + (++serial), asset_id: 'a', start: 0, offset: 0, duration: 40, x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 0, opacity: 1, fit: 'cover' }, extra || {});
const itemOf = (project, clip, el, frame) => ({ clip, asset: { id: clip.asset_id, kind: 'image', width: el.naturalWidth, height: el.naturalHeight }, el, ready: true, state: E.clipState(clip, frame, project) });
const sceneOf = (project, items, frame, transition) => ({ project, frame, width: project.width, height: project.height, layers: [{ trackId: 'v1', items, transition: transition || null }] });
const canvasFor = project => { const canvas = document.createElement('canvas'); canvas.width = project.width; canvas.height = project.height; stage.replaceChildren(canvas); return canvas; };
const shoot = (painter, project, clip, el, frame) => { painter.frame(sceneOf(project, [itemOf(project, clip, el, frame)], frame)); return painter.read(); };
const peakOf = state => 255 * state.opacity * (state.anim ? state.anim.opacity : 1);
// The pixels whose red reaches threshold: their bounding box [x0, y0, x1, y1] (inclusive) and their centroid.
const extent = (shot, threshold) => {
  let x0 = Infinity, y0 = Infinity, x1 = -1, y1 = -1, sx = 0, sy = 0, n = 0;
  for (let y = 0; y < shot.height; y++) for (let x = 0; x < shot.width; x++) {
    if (shot.data[(y * shot.width + x) * 4] < threshold) continue;
    x0 = Math.min(x0, x); y0 = Math.min(y0, y); x1 = Math.max(x1, x); y1 = Math.max(y1, y); sx += x + 0.5; sy += y + 0.5; n++;
  }
  return n ? { box: [x0, y0, x1, y1], centroid: [sx / n, sy / n] } : { box: [], centroid: [] };
};
// A rect {x, y, w, h} in project px, clipped to the canvas, as an extent.
const clipped = (project, r) => {
  const x0 = Math.max(0, r.x), y0 = Math.max(0, r.y), x1 = Math.min(project.width, r.x + r.w), y1 = Math.min(project.height, r.y + r.h);
  return { box: [x0, y0, x1 - 1, y1 - 1], centroid: [(x0 + x1) / 2, (y0 + y1) / 2] };
};
const out = { failures: [] };
let painter = C.create(canvasFor(big), { onProgramFailed: (kind, id) => out.failures.push([kind, id]) });
if (!painter) return { created: false, error: C.lastError };
out.created = true;
// Geometry (index §4.24 "Exact box"). Cover shows VideoStudioAnim.box of the keyed clipBox; contain shows the fitted
// picture of placement(…, anim). rest is the clip's keyed geometry, so every case can fail.
const boxCase = (clip, frame) => {
  const st = E.clipState(clip, frame, big), keyed = E.placement(st, big, big, source), animated = E.placement(st, big, big, source, st.anim);
  const want = clip.fit === 'contain' ? animated.draw : A.box(E.placement(st, big, big, null).box, st.anim, big);
  return { got: extent(shoot(painter, big, clip, white, frame), peakOf(st) / 2), want: clipped(big, want), rest: clipped(big, clip.fit === 'contain' ? keyed.draw : keyed.box) };
};
out.slide = boxCase(clipOf({ anim_in: { preset: 'slide_left', duration: 10 } }), 3);
out.pop = boxCase(clipOf({ anim_in: { preset: 'pop', duration: 10 } }), 7);
out.popContain = boxCase(clipOf({ fit: 'contain', anim_in: { preset: 'pop', duration: 10 } }), 7);
out.offCanvas = boxCase(clipOf({ x: 0.5, anim_out: { preset: 'slide_right', duration: 10 } }), 36);
// Opacity: keyed 0.8 × the slide's fade p = 0.3, read at the animated box centre.
{
  const clip = clipOf({ opacity: 0.8, anim_in: { preset: 'slide_left', duration: 10 } }), st = E.clipState(clip, 3, big), shot = shoot(painter, big, clip, white, 3);
  const r = A.box(E.placement(st, big, big, null).box, st.anim, big);
  out.slideAlpha = { got: shot.data[((r.y + r.h / 2) * big.width + r.x + r.w / 2) * 4], want: peakOf(st) };
}
// Wipe edges, exact: the visible columns on the box's middle row are revealPx's columns.
const wipeCase = (clip, frame) => {
  const st = E.clipState(clip, frame, big), place = E.placement(st, big, big, source, st.anim), columns = A.revealPx(st.anim, place.box.w);
  const shot = shoot(painter, big, clip, white, frame), row = place.box.y + place.box.h / 2;
  let first = -1, last = -1;
  for (let x = 0; x < big.width; x++) if (shot.data[(row * big.width + x) * 4] >= 128) { if (first < 0) first = x; last = x; }
  return { got: [first, last], want: [place.box.x + columns[0], place.box.x + columns[1] - 1], whole: [place.box.x, place.box.x + place.box.w - 1] };
};
out.wipeIn = wipeCase(clipOf({ anim_in: { preset: 'wipe', duration: 10 } }), 4);
out.wipeOut = wipeCase(clipOf({ anim_out: { preset: 'wipe', duration: 10 } }), 35);
// A masked pop: the mask lives in the animated box (index §4.24 "Masks").
{
  const mask = { shape: 'ellipse', x: 0.2, y: 0.15, w: 0.6, h: 0.7, rotation: 0, feather: 0.1, invert: false };
  const clip = clipOf({ mask, anim_in: { preset: 'pop', duration: 10 } }), st = E.clipState(clip, 7, big), shot = shoot(painter, big, clip, white, 7);
  const inBox = b => { const f = P.maskFrame(st.mask, { width: b.w, height: b.h }); return (x, y) => (x >= b.x && x < b.x + b.w && y >= b.y && y < b.y + b.h ? P.maskAt(f, x - b.x, y - b.y) : 0); };
  const want = inBox(E.placement(st, big, big, source, st.anim).box), rest = inBox(E.placement(st, big, big, source).box);
  let max = 0, at = null, restGap = 0;
  for (let y = 0; y < big.height; y++) for (let x = 0; x < big.width; x++) {
    const m = Math.round(255 * want(x + 0.5, y + 0.5)), got = shot.data[(y * big.width + x) * 4], d = Math.abs(got - m);
    if (d > max) { max = d; at = [x, y, got, m]; }
    restGap = Math.max(restGap, Math.abs(m - Math.round(255 * rest(x + 0.5, y + 0.5))));
  }
  out.maskedPop = { max, at, restGap, scale: st.anim.scale };
}
painter.dispose();
// blur_in against the same GL blur passes: MM(MM(g(S), g(S/4), m1), sharp, m2) × p (index §4.24). The 320x180 marker
// fills its 320x180 box at 1:1, so mixing the placed references equals mixing in clip space.
painter = C.create(canvasFor(wideProject), { onProgramFailed: (kind, id) => out.failures.push([kind, id]) });
const effect = (id, params) => ({ id, on: true, params: Object.assign(E.defaults(id), params || {}) });
const sharp = shoot(painter, wideProject, clipOf(), marker, 0).data.slice();
const gS = shoot(painter, wideProject, clipOf({ effects: [effect('blur', { radius: A.BLUR_RADIUS })] }), marker, 0).data.slice();
const gS4 = shoot(painter, wideProject, clipOf({ effects: [effect('blur', { radius: A.BLUR_RADIUS / 4 })] }), marker, 0).data.slice();
const blurCase = frame => {
  const clip = clipOf({ anim_in: { preset: 'blur_in', duration: 12 } }), st = E.clipState(clip, frame, wideProject);
  const got = shoot(painter, wideProject, clip, marker, frame).data, m1 = st.anim.blur.m1 / 255, m2 = st.anim.blur.m2 / 255, k = st.opacity * st.anim.opacity;
  let max = 0, at = null, sharpGap = 0;
  for (let i = 0; i < got.length; i += 4) for (let c = 0; c < 3; c++) {
    const first = gS[i + c] + (gS4[i + c] - gS[i + c]) * m1, want = Math.round((first + (sharp[i + c] - first) * m2) * k);
    const d = Math.abs(got[i + c] - want);
    if (d > max) { max = d; at = [(i / 4) % wideProject.width, Math.floor(i / 4 / wideProject.width), c, got[i + c], want]; }
    sharpGap = Math.max(sharpGap, Math.abs(want - Math.round(sharp[i + c] * k)));
  }
  return { max, at, sharpGap, blur: st.anim.blur };
};
out.blurEarly = blurCase(6);
out.blurLate = blurCase(10);
painter.dispose();
// An ANIM_MIX that does not compile: the clip draws sharp (still fading), reported once as ('effect', 'anim:blur_in').
const originalMix = S.ANIM_MIX;
S.ANIM_MIX = '#version 300 es\nnot glsl';
try {
  const broken = [], fallback = C.create(canvasFor(wideProject), { onProgramFailed: (kind, id) => broken.push([kind, id]) });
  const clip = clipOf({ anim_in: { preset: 'blur_in', duration: 12 } }), st = E.clipState(clip, 6, wideProject), k = st.opacity * st.anim.opacity;
  shoot(fallback, wideProject, clip, marker, 6);
  const got = shoot(fallback, wideProject, clip, marker, 6).data;
  let max = 0;
  for (let i = 0; i < got.length; i += 4) for (let c = 0; c < 3; c++) max = Math.max(max, Math.abs(got[i + c] - Math.round(sharp[i + c] * k)));
  out.broken = { max, failures: broken };
  fallback.dispose();
} finally {
  S.ANIM_MIX = originalMix;
}
// The 2D painter (index §4.8) uses no even rounding:
// - the box moves by (dx·W, dy·H) and scales about its centre;
// - alpha is multiplied by anim.opacity;
// - the wipe clips in box space.
// want is that rule's rectangle [x0, y0, x1, y1) in canvas px.
const twoD = (project, clip, el, frame) => {
  const p2 = D2.create(canvasFor(project)), st = E.clipState(clip, frame, project);
  p2.frame(sceneOf(project, [itemOf(project, clip, el, frame)], frame));
  const shot = p2.read();
  p2.dispose();
  const a = st.anim || { dx: 0, dy: 0, scale: 1, opacity: 1, wipe: null }, W = project.width, H = project.height;
  const w = st.width * W * a.scale, h = st.height * H * a.scale;
  const cx = (st.x + st.width / 2) * W + a.dx * W, cy = (st.y + st.height / 2) * H + a.dy * H;
  let x0 = cx - w / 2, x1 = cx + w / 2;
  if (a.wipe) { if (a.wipe.side === 'out') x0 = x1 - a.wipe.p * w; else x1 = x0 + a.wipe.p * w; }
  return { shot, peak: peakOf(st), want: [x0, cy - h / 2, x1, cy + h / 2], centre: [Math.floor((x0 + x1) / 2), Math.floor(cy)] };
};
out.twoD = {};
[['slide', clipOf({ anim_in: { preset: 'slide_left', duration: 10 } }), 3], ['pop', clipOf({ anim_in: { preset: 'pop', duration: 10 } }), 7], ['wipe', clipOf({ anim_in: { preset: 'wipe', duration: 10 } }), 4]].forEach(([name, clip, frame]) => {
  const r = twoD(big, clip, white, frame);
  out.twoD[name] = { got: extent(r.shot, r.peak / 2), want: r.want, peak: r.peak, centre: r.shot.data[(r.centre[1] * big.width + r.centre[0]) * 4] };
});
// blur_in in 2D fades without a blur. The block and the 1 px line both read 255·p, and the pixel 2 px beside the line
// stays black.
{
  const r = twoD(wideProject, clipOf({ anim_in: { preset: 'blur_in', duration: 12 } }), marker, 6), px = (x, y) => r.shot.data[(y * wideProject.width + x) * 4];
  out.twoDBlur = { block: px(160 + 232, 90 + 60), line: px(160 + 100, 90 + 90), beside: px(160 + 102, 90 + 90), peak: r.peak };
}
// A transition without a 2D implementation draws as a dissolve.
{
  const p2 = D2.create(canvasFor(big));
  const outClip = clipOf({ x: 0, y: 0, width: 1, height: 1, duration: 20, transition: { type: 'circleopen', duration: 10 } }), incClip = clipOf({ x: 0, y: 0, width: 1, height: 1, start: 10, duration: 20 });
  p2.frame(sceneOf(big, [], 15, { type: 'circleopen', progress: 0.5, index: 5, duration: 10, out: itemOf(big, outClip, red, 15), inc: itemOf(big, incClip, blue, 15) }));
  out.twoDTransition = Array.from(p2.read({ x: 160, y: 90, width: 1, height: 1 }).data);
  p2.dispose();
}
stage.replaceChildren();
return out;
`

func animInts(v gson.JSON) []int {
	values := v.Arr()
	out := make([]int, len(values))
	for i := range values {
		out[i] = values[i].Int()
	}
	return out
}

func animNums(v gson.JSON) []float64 {
	values := v.Arr()
	out := make([]float64, len(values))
	for i := range values {
		out[i] = values[i].Num()
	}
	return out
}

// TestDesktopVideoStudioAnimGLBrowser checks the in/out animation in both preview painters (plan 2e Task 12; index
// §4.7, §4.8, §4.24) on the engine page under SwiftShader.
// GL compositor:
//   - extent and centroid within ±1 px of VideoStudioAnim.box for a slide, a pop, a contain pop and a box that leaves
//     the canvas; opacity is keyed × anim;
//   - the wipe edge columns are exact, for in and out;
//   - a masked pop follows the animated box;
//   - blur_in matches the cascade of the same GL blur passes within 3/255;
//   - a broken ANIM_MIX draws sharp and reports ('effect', 'anim:blur_in');
//   - every program compiles.
//
// 2D painter:
//   - offset, scale, alpha and wipe within ±1 px of its rule;
//   - blur_in fades without a blur;
//   - a transition without a 2D implementation draws as a dissolve.
func TestDesktopVideoStudioAnimGLBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := t.TempDir()
	writeFlatNRGBAPNG(t, filepath.Join(dir, "anim_white.png"), 160, 90, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	writeAnimMarkerPNG(t, filepath.Join(dir, "anim_marker.png"))
	writeFlatNRGBAPNG(t, filepath.Join(dir, "anim_red.png"), 320, 180, color.NRGBA{R: 255, A: 255})
	writeFlatNRGBAPNG(t, filepath.Join(dir, "anim_blue.png"), 320, 180, color.NRGBA{B: 255, A: 255})
	page := openVideoStudioEngine(t, newWebGLSmokeBrowser(t), newVideoStudioEngineServer(t, dir))
	requireWebGL2(t, page)
	timed := page.Timeout(4 * time.Minute)
	defer timed.CancelTimeout()
	result, err := timed.Eval("async () => {" + animGLScript + "\n}")
	if err != nil {
		t.Fatalf("animation script: %v", err)
	}
	checks := result.Value.Map()
	if !checks["created"].Bool() {
		t.Fatalf("the compositor did not start: %s", result.Value.JSON("", ""))
	}
	if got := compositingJSON(checks["failures"]); got != "[]" {
		t.Errorf("program failures = %s, want none (PLACE, PLACE_MASKED, BLUR and ANIM_MIX must compile)", got)
	}
	within := func(got, want []int, tolerance int) bool {
		if len(got) != len(want) {
			return false
		}
		for k := range want {
			if d := got[k] - want[k]; d < -tolerance || d > tolerance {
				return false
			}
		}
		return true
	}
	for _, name := range []string{"slide", "pop", "popContain", "offCanvas"} {
		c := checks[name]
		got, want, rest := animInts(c.Get("got").Get("box")), animInts(c.Get("want").Get("box")), animInts(c.Get("rest").Get("box"))
		if !within(got, want, 1) {
			t.Errorf("%s: extent %v, want %v ±1 px (VideoStudioAnim.box / placement with anim)", name, got, want)
		}
		gc, wc := animNums(c.Get("got").Get("centroid")), animNums(c.Get("want").Get("centroid"))
		for k := range wc {
			if k >= len(gc) || math.Abs(gc[k]-wc[k]) > 1 {
				t.Errorf("%s: centroid %v, want %v ±1 px", name, gc, wc)
				break
			}
		}
		if slices.Equal(want, rest) {
			t.Errorf("%s: the animated geometry %v equals the keyed one, so the case cannot fail", name, want)
		}
	}
	if got, want := checks["slideAlpha"].Get("got").Num(), checks["slideAlpha"].Get("want").Num(); math.Abs(got-want) > 2 {
		t.Errorf("slide alpha = %v, want %v ±2 (keyed opacity × anim opacity)", got, want)
	}
	for _, name := range []string{"wipeIn", "wipeOut"} {
		c := checks[name]
		got, want, whole := animInts(c.Get("got")), animInts(c.Get("want")), animInts(c.Get("whole"))
		if !slices.Equal(got, want) {
			t.Errorf("%s: visible columns %v, want exactly %v (revealPx in box columns)", name, got, want)
		}
		if slices.Equal(want, whole) {
			t.Errorf("%s: the reveal %v covers the whole box, so the case cannot fail", name, want)
		}
	}
	masked := checks["maskedPop"]
	if diff := masked.Get("max").Int(); diff > 3 {
		t.Errorf("masked pop differs from maskAt in the animated box by %d/255 at %s, want at most 3", diff, compositingJSON(masked.Get("at")))
	}
	if gap := masked.Get("restGap").Int(); gap <= 12 {
		t.Errorf("the animated and keyed masks differ by only %d/255, so the case cannot tell them apart", gap)
	}
	if scale := masked.Get("scale").Num(); math.Abs(scale-1.08) > 1e-9 {
		t.Errorf("pop scale at p = 0.7 = %v, want the 1.08 peak", scale)
	}
	for _, name := range []string{"blurEarly", "blurLate"} {
		c := checks[name]
		t.Logf("%s: masks %s, largest difference %d/255", name, compositingJSON(c.Get("blur")), c.Get("max").Int())
		if diff := c.Get("max").Int(); diff > 3 {
			t.Errorf("%s: blur_in differs from MM(MM(g(S), g(S/4), m1), sharp, m2) by %d/255 at %s, want at most 3", name, diff, compositingJSON(c.Get("at")))
		}
		if gap := c.Get("sharpGap").Int(); gap <= 12 {
			t.Errorf("%s: the cascade is within %d/255 of the sharp picture, so the case cannot fail", name, gap)
		}
	}
	if diff := checks["broken"].Get("max").Int(); diff > 3 {
		t.Errorf("a broken ANIM_MIX drew %d/255 off the sharp, faded clip, want at most 3", diff)
	}
	if got := compositingJSON(checks["broken"].Get("failures")); got != `[["effect","anim:blur_in"]]` {
		t.Errorf("onProgramFailed calls = %s, want one ('effect', 'anim:blur_in')", got)
	}
	for _, name := range []string{"slide", "pop", "wipe"} {
		c := checks["twoD"].Get(name)
		r := animNums(c.Get("want"))
		want := []int{int(math.Round(r[0])), int(math.Round(r[1])), int(math.Round(r[2])) - 1, int(math.Round(r[3])) - 1}
		if got := animInts(c.Get("got").Get("box")); !within(got, want, 1) {
			t.Errorf("2D %s: extent %v, want %v ±1 px (offset (dx·W, dy·H), scale about the centre, wipe clip)", name, got, want)
		}
		if got, peak := c.Get("centre").Num(), c.Get("peak").Num(); math.Abs(got-peak) > 2 {
			t.Errorf("2D %s: centre %v, want %v ±2 (globalAlpha × anim.opacity)", name, got, peak)
		}
	}
	blur2d := checks["twoDBlur"]
	peak := blur2d.Get("peak").Num()
	for _, name := range []string{"block", "line"} {
		if got := blur2d.Get(name).Num(); math.Abs(got-peak) > 2 {
			t.Errorf("2D blur_in %s = %v, want %v ±2 (the fade without a blur)", name, got, peak)
		}
	}
	if got := blur2d.Get("beside").Num(); got > 2 {
		t.Errorf("2D blur_in: 2 px beside the line reads %v, want black (no blur in 2D)", got)
	}
	if got := jsonRGBA(checks["twoDTransition"]); !rgbaNear(got, color.RGBA{R: 128, B: 128, A: 255}, 3) {
		t.Errorf("2D circleopen at p = 0.5 drew %v, want the dissolve (128, 0, 128) ±3", got)
	}
}
