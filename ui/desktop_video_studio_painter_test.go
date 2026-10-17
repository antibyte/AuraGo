package ui

import (
	"os/exec"
	"testing"
)

// videoStudioPainterPrelude loads the preview in Node with stub media, canvases that remember their context type
// (a canvas that gave a WebGL context refuses a 2D one, as in browsers) and a fake compositor. mount(painter, changes,
// failures) mounts on a fresh canvas with data-preview and aria-label attributes and records onPainterChange and
// onProgramFailed calls.
const videoStudioPainterPrelude = `
const assert = require('node:assert/strict');
global.window = {};
global.location = {origin: 'http://localhost'};
global.requestAnimationFrame = () => 1;
global.cancelAnimationFrame = () => {};
global.performance = {now: () => 100};
class MediaElement {
  constructor() { this.src = ''; this.paused = true; this.currentTime = 0; this.playbackRate = 1; this.readyState = 0; this.videoWidth = 0; this.videoHeight = 0; }
  addEventListener() {}
  load() {}
  pause() { this.paused = true; }
  play() { this.paused = false; return Promise.resolve(); }
  removeAttribute(name) { if (name === 'src') this.src = ''; }
}
class VideoElement extends MediaElement {}
global.HTMLMediaElement = MediaElement;
global.HTMLVideoElement = VideoElement;
global.Image = class { constructor() { this.naturalWidth = 0; this.naturalHeight = 0; } addEventListener() {} removeAttribute() {} };
function context2d(canvas) {
  const state = {canvas, globalAlpha: 1, globalCompositeOperation: 'source-over', fillStyle: '#000'};
  return new Proxy(state, {
    get(target, key) {
      if (key === 'getImageData') return (x, y, w, h) => ({width: w, height: h, data: new Uint8ClampedArray(w * h * 4).fill(7)});
      if (key in target) return target[key];
      return () => {};
    },
    set(target, key, value) { target[key] = value; return true; }
  });
}
let serial = 0;
const swaps = [];
function makeCanvas(attributes) {
  return {
    serial: ++serial, width: 300, height: 150, attributes: Object.assign({}, attributes), parentNode: null, contexts: [], mode: '', drawing: null,
    getContext(type) {
      this.contexts.push(type);
      if (this.mode && this.mode !== type) return null;
      this.mode = type;
      if (type !== '2d') return null;
      if (!this.drawing) this.drawing = context2d(this);
      return this.drawing;
    },
    cloneNode(deep) {
      assert.equal(deep, false, 'the fresh element is a shallow clone');
      const copy = makeCanvas(this.attributes);
      copy.width = this.width; copy.height = this.height;
      return copy;
    },
    replaceWith(next) { swaps.push([this.serial, next.serial]); next.parentNode = this.parentNode; this.parentNode = null; },
    hasAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attributes, name); },
    getAttribute(name) { return this.hasAttribute(name) ? this.attributes[name] : null; }
  };
}
global.document = {createElement(tag) { return tag === 'canvas' ? makeCanvas({}) : new VideoElement(); }};
const glPainters = [];
const fake = {supported: true, failNext: false};
window.VideoStudioCompositor = {
  lastError: '',
  supported: () => fake.supported,
  create(canvas, hooks) {
    canvas.getContext('webgl2');
    if (fake.failNext) { fake.failNext = false; window.VideoStudioCompositor.lastError = 'init_failed'; return null; }
    window.VideoStudioCompositor.lastError = '';
    const painter = {
      kind: 'gl', canvas, hooks, scenes: [], disposed: false,
      frame(scene) { this.scenes.push(scene); },
      read(rect) { const r = rect || {width: canvas.width, height: canvas.height}; return {width: r.width, height: r.height, data: new Uint8ClampedArray(r.width * r.height * 4).fill(200)}; },
      release() {},
      dispose() { this.disposed = true; }
    };
    glPainters.push(painter);
    return painter;
  }
};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-painter2d.js');
require('./js/desktop/apps/video-studio-preview.js');
const Preview = window.VideoStudioPreview;
const video = {id: 'v', kind: 'video', media_url: '/media/v.mp4', duration_frames: 300, width: 640, height: 360, has_audio: false};
const clipOf = (id, start, duration) => ({id, asset_id: 'v', start, offset: 0, duration, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain', transition: null});
const project = {version: 3, name: 'Painters', width: 64, height: 36, fps: 30, assets: [video], tracks: [{id: 'v1', kind: 'video', hidden: false, muted: false, clips: [clipOf('c1', 0, 60)]}]};
function mount(painter, changes, failures) {
  const canvas = makeCanvas({'data-preview': '', 'aria-label': 'Preview'});
  canvas.parentNode = {};
  const options = {onPainterChange: (kind, reason, element) => changes.push([kind, reason, element]), onProgramFailed: (kind, id) => failures.push([kind, id])};
  if (painter) options.painter = painter;
  const preview = Preview.mount(canvas, () => project, () => 0, () => {}, () => {}, () => 1, options);
  return {canvas, preview};
}
`

func runVideoStudioPainterScript(t *testing.T, body string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio preview checks")
	}
	if output, err := exec.Command(node, "-e", videoStudioPainterPrelude+body).CombinedOutput(); err != nil {
		t.Fatalf("Video Studio painter checks: %v\n%s", err, output)
	}
}

// Painter selection (index §4.5): 2D by default, GL on request when the compositor starts, a fresh element after a
// failed GL start, no swap without WebGL2, and the GL scene lists clips whose media has no frame yet.
func TestVideoStudioPreviewPainterSelection(t *testing.T) {
	runVideoStudioPainterScript(t, `
{
  const changes = [], failures = [];
  const {canvas, preview} = mount('', changes, failures);
  assert.equal(preview.painterKind(), '2d', 'the default painter is 2D');
  assert.equal(glPainters.length, 0, 'no GL attempt without a painter request');
  assert(canvas.contexts.length > 0 && canvas.contexts.every(type => type === '2d'));
  assert.equal(swaps.length, 0);
  preview.dispose();
}
{
  const changes = [], failures = [];
  const {canvas, preview} = mount('auto', changes, failures);
  assert.equal(preview.painterKind(), 'gl', "'auto' uses a compositor that starts");
  const gl = glPainters.at(-1);
  assert.equal(gl.canvas, canvas, 'GL draws on the mounted element');
  assert.equal(swaps.length, 0);
  const scene = gl.scenes.at(-1);
  assert(scene && scene.layers.length === 1, 'the GL painter received the frame description');
  const item = scene.layers[0].items[0];
  assert.equal(item && item.clip.id, 'c1');
  assert.equal(item.ready, false, 'a clip whose media has no frame yet is listed with ready false (hold last frame)');
  const shot = preview.capture({x: 0, y: 0, width: 2, height: 2});
  assert.equal(shot.width, 2);
  assert.equal(shot.data[0], 200, 'capture reads the GL painter');
  preview.dispose();
  assert(gl.disposed, 'dispose releases the GL painter');
}
{
  fake.failNext = true;
  const changes = [], failures = [];
  const {canvas, preview} = mount('gl', changes, failures);
  assert.equal(preview.painterKind(), '2d', 'a GL start failure draws in 2D');
  assert.equal(swaps.length, 1, 'the claimed element is replaced');
  assert.equal(swaps[0][0], canvas.serial);
  assert.deepEqual(canvas.contexts, ['webgl2'], 'the claimed element never gets a 2D context');
  assert.equal(preview.capture().data[0], 7, 'capture reads the 2D painter on the fresh element');
  preview.dispose();
}
{
  fake.supported = false;
  const changes = [], failures = [];
  const {canvas, preview} = mount('auto', changes, failures);
  assert.equal(preview.painterKind(), '2d');
  assert.equal(swaps.length, 1, 'no swap without a GL attempt');
  assert(canvas.contexts.every(type => type === '2d'));
  preview.dispose();
  fake.supported = true;
}
`)
}

// Fallback (index §4.5): the reasons reach onPainterChange with the element in use, fallback('forced') and the
// compositor's onFallback swap in a fresh element once, and failed programs are reported once and listed.
func TestVideoStudioPreviewPainterFallback(t *testing.T) {
	runVideoStudioPainterScript(t, `
{
  fake.failNext = true;
  const changes = [], failures = [];
  const {canvas, preview} = mount('gl', changes, failures);
  assert.deepEqual(changes.map(([kind, reason]) => [kind, reason]), [['2d', 'init_failed']], 'a failed GL start is reported with the compositor reason');
  const fresh = changes[0][2];
  assert.notEqual(fresh, canvas);
  assert.equal(fresh.getAttribute('data-preview'), '');
  assert.equal(fresh.getAttribute('aria-label'), 'Preview');
  assert.equal(preview.fallback('forced'), false, 'a 2D preview has nothing to fall back from');
  preview.dispose();
}
{
  fake.supported = false;
  const changes = [], failures = [];
  const {canvas, preview} = mount('auto', changes, failures);
  assert.deepEqual(changes.map(([kind, reason, element]) => [kind, reason, element === canvas]), [['2d', 'unsupported', true]]);
  preview.dispose();
  fake.supported = true;
}
{
  const changes = [], failures = [];
  const {canvas, preview} = mount('auto', changes, failures);
  assert.equal(changes.length, 0, 'a working GL start reports nothing');
  const gl = glPainters.at(-1);
  gl.hooks.onProgramFailed('transition', 'wipeleft');
  gl.hooks.onProgramFailed('transition', 'wipeleft');
  assert.deepEqual(failures, [['transition', 'wipeleft']], 'a failed program is reported once');
  assert.deepEqual(preview.failedPrograms(), [{kind: 'transition', id: 'wipeleft'}]);
  assert.equal(preview.painterKind(), 'gl', 'a failed transition program keeps GL');
  const before = swaps.length;
  assert.equal(preview.fallback('forced'), true);
  assert.equal(preview.fallback('forced'), false);
  assert.equal(preview.painterKind(), '2d');
  assert(gl.disposed, 'the GL painter is disposed on fallback');
  assert.equal(swaps.length, before + 1);
  assert.deepEqual(changes.map(([kind, reason]) => [kind, reason]), [['2d', 'forced']]);
  const fresh = changes[0][2];
  assert.equal(swaps.at(-1)[1], fresh.serial, 'the reported element is the one swapped in');
  assert.deepEqual(canvas.contexts, ['webgl2'], 'the old element never got a 2D context');
  assert(fresh.contexts.includes('2d'));
  assert.deepEqual(preview.failedPrograms(), [], 'program failures belong to the GL painter');
  assert.equal(preview.capture({x: 0, y: 0, width: 1, height: 1}).data[0], 7, 'capture reads the 2D painter');
  preview.dispose();
}
{
  const changes = [], failures = [];
  const {preview} = mount('auto', changes, failures);
  const gl = glPainters.at(-1);
  gl.hooks.onFallback('context_lost');
  assert.equal(preview.painterKind(), '2d');
  assert(gl.disposed);
  assert.deepEqual(changes.map(([kind, reason]) => [kind, reason]), [['2d', 'context_lost']]);
  preview.dispose();
}
`)
}
