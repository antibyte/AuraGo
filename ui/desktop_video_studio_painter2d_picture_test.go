package ui

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// runVideoStudioPictureNode runs a self-contained Node check from the ui directory.
func runVideoStudioPictureNode(t *testing.T, name, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio preview checks")
	}
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, output)
	}
}

// TestVideoStudioPainter2DCropFlip checks the 2D painter's picture (plan 2c, Task 11; index §4.8) with a recording
// context. The crop becomes drawImage's source rectangle: it is rounded in decoded pixels (asset size, swapped for a
// rotated element), mapped to the element (a proxy), static or per frame, and fit with contain or cover. Flips are
// negative scales about the box centre. Effects are never drawn. pending() is always 0.
func TestVideoStudioPainter2DCropFlip(t *testing.T) {
	runVideoStudioPictureNode(t, "2D painter crop/flip", `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-painter2d.js');
const E = window.VideoStudioEffects, P = window.VideoStudioPicture;
const calls = [], sets = [];
const context = new Proxy({}, {
  get(target, key) { return key in target ? target[key] : (...args) => { calls.push([String(key), ...args]); }; },
  set(target, key, value) { sets.push(String(key)); target[key] = value; return true; }
});
const canvas = {width: 0, height: 0, getContext: () => context};
const painter = window.VideoStudioPainter2D.create(canvas);
assert.equal(painter.pending(), 0, 'the 2D painter loads nothing');
const project = {version: 3, name: 'Crop', width: 1280, height: 720, fps: 25, assets: [], tracks: []};
const clip = extra => Object.assign({id: 'c', asset_id: 'v', start: 0, offset: 0, duration: 40, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null}, extra);
// A 1280x720 video played through a 640x360 proxy: crop pixels round on the original, then scale to the element.
const video = {id: 'v', kind: 'video', width: 1280, height: 720, duration_frames: 100, has_audio: false};
const proxy = {videoWidth: 640, videoHeight: 360};
function draw(c, frame, asset, el) {
  calls.length = 0; sets.length = 0;
  painter.frame({project, frame, width: 640, height: 360, layers: [{trackId: 'v1', items: [{clip: c, asset: asset || video, el: el || proxy, ready: true, state: E.clipState(c, frame, project)}], transition: null}]});
  const images = calls.filter(call => call[0] === 'drawImage');
  assert.equal(images.length, 1, 'one clip, one drawImage');
  return {source: images[0].slice(2), scales: calls.filter(call => call[0] === 'scale').map(call => call.slice(1)), rects: calls.filter(call => call[0] === 'rect').map(call => call.slice(1)), sets: sets.slice()};
}
// No crop, no flip: the whole element, contained, and no scale call.
const plain = draw(clip({}), 5);
assert.deepEqual(plain.source, [0, 0, 640, 360, -320, -180, 640, 360]);
assert.deepEqual(plain.scales, []);
// Static crop l 0.1, t 0.2, r 0.3: the 768x576 rect at (128, 144) of 1280x720 is (64, 72, 384, 288) on the proxy.
// Contained in 640x360 at scale 1.25: 480x360. flip_h mirrors with scale(-1, 1).
assert.deepEqual(P.cropRect({l: 0.1, t: 0.2, r: 0.3, b: 0}, 1280, 720), {x: 128, y: 144, w: 768, h: 576});
const cropped = draw(clip({crop: {l: 0.1, t: 0.2, r: 0.3, b: 0}, flip_h: true}), 5);
assert.deepEqual(cropped.source, [64, 72, 384, 288, -240, -180, 480, 360]);
assert.deepEqual(cropped.scales, [[-1, 1]]);
// Animated: crop.l keys 0.1 -> 0.3 over source frames 0..20. Frame 10 crops l 0.2, the 640x576 rect at (256, 144):
// (128, 72, 320, 288) on the proxy, contained at 1.25 to 400x360. flip_v is scale(1, -1).
const animated = draw(clip({crop: {l: 0.1, t: 0.2, r: 0.3, b: 0}, flip_v: true, keyframes: {'crop.l': [{f: 0, v: 0.1, e: 'linear'}, {f: 20, v: 0.3, e: 'linear'}]}}), 10);
assert.deepEqual(animated.source, [128, 72, 320, 288, -200, -180, 400, 360], 'animated crop uses the frame rect');
assert.deepEqual(animated.scales, [[1, -1]]);
// Cover: the left half (320x360 on the proxy) covers the 640x360 box at scale 2 and is clipped to the box.
const cover = draw(clip({fit: 'cover', crop: {l: 0, t: 0, r: 0.5, b: 0}}), 0);
assert.deepEqual(cover.source, [0, 0, 320, 360, -320, -360, 640, 720]);
assert.deepEqual(cover.rects, [[-320, -180, 640, 360]], 'cover clips to the box');
// A phone video stored 1920x1080 and shown upright (element 1080x1920): fractions refer to the decoded 1080x1920.
const rotated = draw(clip({crop: {l: 0, t: 0.25, r: 0, b: 0.25}}), 0, {id: 'v', kind: 'video', width: 1920, height: 1080, duration_frames: 100, has_audio: false}, {videoWidth: 1080, videoHeight: 1920});
assert.deepEqual(rotated.source, [0, 480, 1080, 960, -202.5, -180, 405, 360]);
// Effects are never drawn: the same picture, no canvas filter.
const graded = draw(clip({effects: [{id: 'color', on: true, params: Object.assign(E.defaults('color'), {exposure: 2})}]}), 5);
assert.deepEqual(graded.source, plain.source, 'effects do not change the 2D picture');
assert(!graded.sets.includes('filter'), 'no canvas filter stands in for an effect');
`)
}

// TestVideoStudioPreviewSettleWaitsForLUT checks the preview side of LUT loading with a fake GL painter (plan 2c,
// Task 11; index §4.5):
// - the compositor gets the loadLUT and onResource hooks;
// - builtin refs fetch the look endpoint, and project LUT assets go through the media URL (same origin only);
// - the text is parsed with VideoStudioPicture.parseCube, and a finished load repaints;
// - settle() waits while the painter reports pending() > 0.
func TestVideoStudioPreviewSettleWaitsForLUT(t *testing.T) {
	runVideoStudioPictureNode(t, "preview LUT loading", `
const assert = require('node:assert/strict');
global.window = {};
global.location = {origin: 'http://localhost'};
global.requestAnimationFrame = () => 1;
global.cancelAnimationFrame = () => {};
global.performance = {now: () => Date.now()};
class MediaElement { constructor() { this.src = ''; this.paused = true; this.currentTime = 0; this.playbackRate = 1; this.readyState = 2; } addEventListener() {} load() {} pause() { this.paused = true; } play() { this.paused = false; return Promise.resolve(); } removeAttribute() {} }
global.HTMLMediaElement = MediaElement;
global.HTMLVideoElement = class extends MediaElement {};
global.Image = class { addEventListener() {} removeAttribute() {} };
const drawing = new Proxy({}, {get: (target, key) => (key in target ? target[key] : () => {}), set: (target, key, value) => { target[key] = value; return true; }});
global.document = {createElement: () => ({width: 0, height: 0, getContext: () => drawing})};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-painter2d.js');
require('./js/desktop/apps/video-studio-preview.js');
const P = window.VideoStudioPicture;
const responses = new Map(), fetches = [];
global.fetch = (url, init) => {
  fetches.push([String(url), init && init.credentials]);
  const body = responses.get(String(url));
  return Promise.resolve(body == null ? {ok: false, status: 404, text: () => Promise.resolve('')} : {ok: true, status: 200, text: () => Promise.resolve(body)});
};
let gl = null;
window.VideoStudioCompositor = {
  lastError: '', supported: () => true,
  create(canvas, hooks) {
    gl = {kind: 'gl', canvas, hooks, waiting: 0, paints: 0, frame() { this.paints++; }, read: () => ({width: 0, height: 0, data: new Uint8ClampedArray(0)}), release() {}, dispose() {}, pending() { return this.waiting; }};
    return gl;
  }
};
const CUBE = 'TITLE "two"\nLUT_3D_SIZE 2\n0 0 0\n1 0 0\n0 1 0\n1 1 0\n0 0 1\n1 0 1\n0 1 1\n1 1 1\n';
const project = {version: 3, name: 'LUT', width: 64, height: 36, fps: 25, tracks: [], assets: [
  {id: 'look', kind: 'lut', name: 'look.cube', duration_frames: 0, lut_size: 2, media_url: '/api/desktop/video-studio/projects/p/media/look'},
  {id: 'far', kind: 'lut', name: 'far.cube', duration_frames: 0, lut_size: 2, media_url: 'https://elsewhere.example/far.cube'},
  {id: 'still', kind: 'image', name: 'still.png', duration_frames: 0, width: 64, height: 36, media_url: '/media/still.png'}]};
const canvas = {width: 64, height: 36, parentNode: null, getContext: () => drawing, cloneNode() { return Object.assign({}, this); }, replaceWith() {}};
const preview = window.VideoStudioPreview.mount(canvas, () => project, () => 0, () => {}, () => {}, () => 1,
  {painter: 'gl', mediaURL: asset => (asset.id === 'look' ? '/media/look.cube' : asset.media_url)});
(async () => {
  assert.equal(preview.painterKind(), 'gl');
  assert.equal(typeof gl.hooks.loadLUT, 'function', 'the compositor gets loadLUT');
  assert.equal(typeof gl.hooks.onResource, 'function', 'and onResource');
  responses.set('http://localhost/api/desktop/video-studio/luts/builtin/teal_orange.cube', CUBE);
  responses.set('http://localhost/media/look.cube', CUBE);
  const want = P.parseCube(CUBE);
  const builtin = await gl.hooks.loadLUT('builtin:teal_orange');
  assert.deepEqual([builtin.size, Array.from(builtin.data)], [want.size, Array.from(want.data)], 'a built-in look is parsed with parseCube');
  assert.deepEqual(fetches.at(-1), ['http://localhost/api/desktop/video-studio/luts/builtin/teal_orange.cube', 'same-origin']);
  assert.equal((await gl.hooks.loadLUT('look')).size, 2);
  assert.deepEqual(fetches.at(-1), ['http://localhost/media/look.cube', 'same-origin'], 'a project LUT goes through the media URL');
  const before = fetches.length;
  await assert.rejects(gl.hooks.loadLUT('far'), 'a cross-origin media URL is refused');
  await assert.rejects(gl.hooks.loadLUT('still'), 'only assets of kind lut');
  await assert.rejects(gl.hooks.loadLUT('missing'), 'unknown assets are refused');
  await assert.rejects(gl.hooks.loadLUT('builtin:'), 'a builtin ref needs an id');
  assert.equal(fetches.length, before, 'refused refs fetch nothing');
  responses.set('http://localhost/api/desktop/video-studio/luts/builtin/mono.cube', 'LUT_3D_SIZE 2\n0 0 0\n');
  await assert.rejects(gl.hooks.loadLUT('builtin:mono'), error => Number.isInteger(error.line), 'a broken table rejects with the parseCube line');
  await assert.rejects(gl.hooks.loadLUT('builtin:cool'), /lut_http_404/, 'an HTTP error rejects');
  const paints = gl.paints;
  gl.hooks.onResource();
  assert.equal(gl.paints, paints + 1, 'a finished load repaints');
  gl.waiting = 1;
  let settled = null;
  const waiting = preview.settle(3000).then(value => { settled = value; });
  await new Promise(resolve => setTimeout(resolve, 120));
  assert.equal(settled, null, 'settle waits while a LUT loads');
  gl.waiting = 0;
  await waiting;
  assert.equal(settled, true, 'and resolves once nothing is pending');
  gl.waiting = 1;
  assert.equal(await preview.settle(60), false, 'a load that never finishes times out');
  gl.waiting = 0;
  preview.dispose();
})().catch(error => { console.error(error); process.exit(1); });
`)
}

// TestVideoStudioPreviewEffectNotice checks the persistent preview notice (plan 2c, Task 11; index §4.5):
// - previewFallback while the 2D painter draws;
// - previewTransitionFallback when GL lists a failed transition, which takes precedence;
// - previewEffectFallback when GL lists only failed effects;
// - hidden otherwise.
func TestVideoStudioPreviewEffectNotice(t *testing.T) {
	runVideoStudioPictureNode(t, "preview notice", `
const assert = require('node:assert/strict');
global.window = {VideoStudioParts: {PREF_TIMELINE: 'vs.timeline', tr: (s, key) => key}};
require('./js/desktop/apps/video-studio-stage.js');
const P = window.VideoStudioParts;
const notice = {textContent: 'stale', hidden: false};
const state = (kind, failed) => ({project: {}, q: selector => (selector === '[data-preview-notice]' ? notice : null), preview: {painterKind: () => kind, failedPrograms: () => failed}});
const show = (kind, failed) => { P.syncPreviewNotice(state(kind, failed)); return [notice.textContent, notice.hidden]; };
assert.deepEqual(show('gl', [{kind: 'effect', id: 'blur'}]), ['previewEffectFallback', false], 'a failed effect program');
assert.deepEqual(show('gl', [{kind: 'effect', id: 'blur'}, {kind: 'transition', id: 'wipeleft'}]), ['previewTransitionFallback', false], 'a transition failure takes precedence');
assert.deepEqual(show('2d', []), ['previewFallback', false], 'the 2D painter');
assert.deepEqual(show('gl', []), ['', true], 'nothing failed');
`)
}

// TestVideoStudioEngineServesBuiltinLooks checks the engine page's look route (index §4.12). GL goldens resolve
// builtin:<id> through it with the bytes of videostudio.BuiltinLookCube.
func TestVideoStudioEngineServesBuiltinLooks(t *testing.T) {
	server := newVideoStudioEngineServer(t, "")
	want, ok := videostudio.BuiltinLookCube("teal_orange")
	if !ok {
		t.Fatal("videostudio.BuiltinLookCube(teal_orange) is missing")
	}
	get := func(path string) (int, string, []byte) {
		t.Helper()
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, response.Header.Get("Content-Type"), body
	}
	if code, kind, body := get("/api/desktop/video-studio/luts/builtin/teal_orange.cube"); code != http.StatusOK || !strings.HasPrefix(kind, "text/plain") || string(body) != string(want) {
		t.Fatalf("teal_orange.cube: status %d, type %q, %d bytes; want 200, text/plain and the %d bytes of BuiltinLookCube", code, kind, len(body), len(want))
	}
	for _, path := range []string{"/api/desktop/video-studio/luts/builtin/unknown.cube", "/api/desktop/video-studio/luts/builtin/teal_orange"} {
		if code, _, _ := get(path); code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, code)
		}
	}
}

// TestVideoStudioPainter2DReadScaled checks readScaled (2f Task 1; index §4.26.1) with recording canvases.
//   - The 2D painter resolves null before its first frame, after dispose and for a size outside 1..1024.
//   - After a frame it draws the whole canvas onto a small canvas made with {alpha: false, willReadFrequently: true}
//     and imageSmoothingEnabled false (point sampling), and resolves to that canvas's getImageData.
//   - The small canvas is reused for the same size and rebuilt for another. The painter's own canvas is never
//     willReadFrequently, and readScaled never draws on it.
//   - The preview hands the painter's promise through: null without a painter that reads, and after dispose.
//   - options.onPaint runs last in every paint, after the painter and setFrame. A throwing onPaint does not break the
//     paint, and readScaled never paints (capture still does).
func TestVideoStudioPainter2DReadScaled(t *testing.T) {
	runVideoStudioPictureNode(t, "2D painter readScaled", `
const assert = require('node:assert/strict');
global.window = {};
delete global.OffscreenCanvas;
const smalls = [];
function recorder(owner) {
  const calls = [];
  return {owner, calls, imageSmoothingEnabled: true, globalAlpha: 1, globalCompositeOperation: 'source-over', fillStyle: '#000',
    drawImage(...args) { calls.push(['drawImage', this.imageSmoothingEnabled, ...args]); },
    getImageData(x, y, w, h) { calls.push(['getImageData', x, y, w, h]); const data = new Uint8ClampedArray(w * h * 4).fill(9); data.owner = owner; return {width: w, height: h, data}; },
    fillRect() {}, clearRect() {}, save() {}, restore() {}, translate() {}, rotate() {}, scale() {}, beginPath() {}, rect() {}, clip() {}};
}
global.document = {createElement(tag) {
  assert.equal(tag, 'canvas', 'readScaled creates canvases only');
  const small = {width: 300, height: 150, contexts: [], context: null,
    getContext(type, options) { this.contexts.push([type, options]); if (!this.context) this.context = recorder(this); return this.context; }};
  smalls.push(small);
  return small;
}};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-painter2d.js');
const own = [], main = recorder(null);
const canvas = {width: 0, height: 0, getContext(type, options) { own.push([type, options]); return main; }};
const painter = window.VideoStudioPainter2D.create(canvas);
assert.deepEqual(own, [['2d', {alpha: false}]], 'the painter canvas is never willReadFrequently');
const scene = {project: {width: 1280, height: 720}, frame: 0, width: 1280, height: 720, layers: []};
(async () => {
  const early = painter.readScaled(256, 144);
  assert(early instanceof Promise, 'readScaled returns a promise');
  assert.equal(await early, null, 'null before the first frame');
  assert.equal(smalls.length, 0, 'no small canvas before the first frame');
  painter.frame(scene);
  const shot = await painter.readScaled(256, 144);
  assert.equal(smalls.length, 1, 'the first read makes the small canvas');
  const small = smalls[0];
  assert.deepEqual([small.width, small.height], [256, 144]);
  assert.deepEqual(small.contexts, [['2d', {alpha: false, willReadFrequently: true}]]);
  assert.deepEqual(small.context.calls, [['drawImage', false, canvas, 0, 0, 1280, 720, 0, 0, 256, 144], ['getImageData', 0, 0, 256, 144]], 'the whole frame, point-sampled, then read');
  assert.deepEqual([shot.width, shot.height, shot.data.length, shot.data[0], shot.data.owner === small], [256, 144, 256 * 144 * 4, 9, true], 'resolves to the small canvas getImageData');
  await painter.readScaled(256, 144);
  assert.equal(smalls.length, 1, 'the same size reuses the small canvas');
  assert.equal(small.context.calls.length, 4);
  const tiny = await painter.readScaled(7, 5);
  assert.equal(smalls.length, 2, 'another size makes a new small canvas');
  assert.deepEqual([smalls[1].width, smalls[1].height, tiny.width, tiny.height], [7, 5, 7, 5]);
  assert.deepEqual(smalls[1].context.calls[0], ['drawImage', false, canvas, 0, 0, 1280, 720, 0, 0, 7, 5]);
  for (const [w, h] of [[0, 144], [2000, 144], [256, 1025], [2.5, 3], ['256', 144], [NaN, 1]]) assert.equal(await painter.readScaled(w, h), null, 'invalid size ' + w + ' x ' + h);
  assert.equal(smalls.length, 2, 'an invalid size makes nothing');
  assert.equal(main.calls.filter(call => call[0] === 'drawImage' || call[0] === 'getImageData').length, 0, 'readScaled never draws on or reads the painter canvas');
  painter.dispose();
  assert.equal(await painter.readScaled(256, 144), null, 'null after dispose');
})().catch(error => { console.error(error); process.exit(1); });
`)
	runVideoStudioPainterScript(t, `
(async () => {
  {
    const order = [];
    const canvas = makeCanvas({'data-preview': ''}); canvas.parentNode = {};
    const preview = Preview.mount(canvas, () => project, () => 0, value => order.push('setFrame ' + value), () => {}, () => 1,
      {painter: 'gl', onPaint: () => order.push('onPaint after ' + glPainters.at(-1).scenes.length + ' scene(s)')});
    assert.equal(preview.painterKind(), 'gl');
    assert.deepEqual(order, ['setFrame 0', 'onPaint after 1 scene(s)'], 'onPaint runs last, after the painter drew and setFrame');
    order.length = 0;
    preview.seek(3);
    assert.deepEqual(order, ['setFrame 3', 'onPaint after 2 scene(s)']);
    order.length = 0;
    const none = preview.readScaled(256, 144);
    assert(none instanceof Promise, 'the preview returns a promise');
    assert.equal(await none, null, 'a painter without readScaled gives null');
    assert.deepEqual(order, [], 'readScaled never paints');
    preview.dispose();
  }
  {
    let paints = 0;
    const canvas = makeCanvas({'data-preview': ''}); canvas.parentNode = {};
    const preview = Preview.mount(canvas, () => project, () => 0, () => {}, () => {}, () => 1, {onPaint: () => { paints++; }});
    assert.equal(preview.painterKind(), '2d');
    const before = paints;
    const shot = await preview.readScaled(4, 2);
    assert.deepEqual([shot.width, shot.height, shot.data[0]], [4, 2, 7], 'the 2D painter reads its last frame');
    assert.equal(paints, before, 'without a paint');
    preview.capture();
    assert.equal(paints, before + 1, 'capture still paints');
    preview.dispose();
    assert.equal(await preview.readScaled(4, 2), null, 'null after dispose');
  }
  {
    let shown = -1;
    const canvas = makeCanvas({'data-preview': ''}); canvas.parentNode = {};
    const preview = Preview.mount(canvas, () => project, () => 0, value => { shown = value; }, () => {}, () => 1, {onPaint: () => { throw new Error('scope hook'); }});
    preview.seek(5);
    assert.equal(shown, 5, 'a throwing onPaint does not break the paint');
    assert.equal(preview.getFrame(), 5);
    preview.dispose();
  }
})().catch(error => { console.error(error); process.exit(1); });
`)
}
