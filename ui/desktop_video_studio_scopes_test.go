package ui

import (
	"strings"
	"testing"
)

// TestVideoStudioScopeMath checks VideoStudioScopeMath (2f Task 2; index §4.26.2–§4.26.4) against analytic values:
//   - the constants and colours;
//   - BT.709 luma, where greys give Y8 = v;
//   - Cb/Cr at ±0.5 for the primaries and secondaries, and exactly 0 for greys;
//   - the skin angle;
//   - the histogram bars, the luma line and the peak without the end bins;
//   - the waveform and vectorscope bins and their log intensity;
//   - the graticule primitives, the throttle and the stored preference.
func TestVideoStudioScopeMath(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-scope-math.js');
const M = window.VideoStudioScopeMath;
const near = (got, want, eps, what) => assert(Math.abs(got - want) <= eps, what + ': ' + got + ', want ' + want + ' +- ' + eps);
const frame = (w, h, pixel) => {
  const data = new Uint8ClampedArray(w * h * 4);
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) { const [r, g, b] = pixel(x, y), o = (y * w + x) * 4; data[o] = r; data[o + 1] = g; data[o + 2] = b; data[o + 3] = 255; }
  return {width: w, height: h, data};
};
const px = (image, width, x, y) => Array.from(image.slice((y * width + x) * 4, (y * width + x) * 4 + 4));
const BG = [12, 15, 21, 255], LUMA = [235, 235, 235, 255], TRACE = [140, 255, 166, 255];

// Constants.
assert.deepEqual([M.SIZE, M.HISTOGRAM, M.WAVEFORM, M.VECTORSCOPE].map(size => [size.width, size.height]), [[256, 144], [256, 100], [256, 128], [128, 128]]);
assert.equal(M.READ_INTERVAL_MS, 100);
assert.deepEqual(Array.from(M.KINDS), ['histogram', 'waveform', 'vectorscope']);
assert(Object.isFrozen(M.KINDS) && Object.isFrozen(M.COLORS) && Object.isFrozen(M.COLORS.BG), 'the constants are frozen');
assert.deepEqual([M.COLORS.BG, M.COLORS.TRACE, M.COLORS.HR, M.COLORS.HG, M.COLORS.HB, M.COLORS.LUMA].map(c => Array.from(c)), [[12, 15, 21], [140, 255, 166], [255, 80, 80], [80, 230, 110], [90, 140, 255], [235, 235, 235]]);
assert.deepEqual([M.COLORS.GRID, M.COLORS.SKIN, M.COLORS.TARGET], ['rgba(255, 255, 255, 0.18)', 'rgba(255, 190, 140, 0.75)', 'rgba(255, 255, 255, 0.5)']);

// Luma: BT.709, full range, no linearisation.
near(M.luma(255, 255, 255), 255, 1e-9, 'white');
near(M.luma(255, 0, 0), 54.213, 1e-9, 'red');
near(M.luma(0, 255, 0), 182.376, 1e-9, 'green');
near(M.luma(0, 0, 255), 18.411, 1e-9, 'blue');
const ramp = M.analyse(frame(256, 4, x => [x, x, x]));
for (let v = 0; v < 256; v++) assert.equal(ramp.histogram.y[v], 4, 'grey ' + v + ' gives Y8 = ' + v);

// Colour difference.
near(M.chroma(0, 0, 255).cb, 0.5, 1e-9, 'blue Cb');
near(M.chroma(255, 255, 0).cb, -0.5, 1e-9, 'yellow Cb');
near(M.chroma(255, 0, 0).cr, 0.5, 1e-9, 'red Cr');
near(M.chroma(0, 255, 255).cr, -0.5, 1e-9, 'cyan Cr');
for (const v of [0, 1, 77, 128, 254, 255]) assert.deepEqual(M.chroma(v, v, v), {cb: 0, cr: 0}, 'grey ' + v + ' has no colour');

// Skin-tone line: the NTSC I axis (123 degrees in U/V) mapped into Cb/Cr.
const I = 123 * Math.PI / 180;
near(M.skinAngle(), Math.atan2(Math.sin(I) / (0.877283 * 1.5748), Math.cos(I) / (0.492111 * 1.8556)), 1e-12, 'skinAngle is the mapped I axis');
near(M.skinAngle(), 2.34737, 1e-6, 'skinAngle');
for (const [r, g, b] of [[141, 85, 36], [198, 134, 66], [224, 172, 105]]) {
  const c = M.chroma(r, g, b);
  near(Math.atan2(c.cr, c.cb), M.skinAngle(), 10 * Math.PI / 180, 'skin tone ' + [r, g, b].join(','));
}

// analyse: empty or short frames.
assert.equal(M.analyse(null), null);
assert.equal(M.analyse({width: 0, height: 4, data: new Uint8ClampedArray(0)}), null);
assert.equal(M.analyse({width: 2, height: 2, data: new Uint8ClampedArray(8)}), null, 'a short buffer');

// Histogram: a black and a white pixel. The peak over bins 1..254 is 0, so the peak over all bins (1) scales the bars.
const bw = M.analyse(frame(2, 1, x => (x ? [255, 255, 255] : [0, 0, 0])));
assert.equal(bw.samples, 2);
for (const channel of ['r', 'g', 'b', 'y']) assert.deepEqual([bw.histogram[channel][0], bw.histogram[channel][255], bw.histogram[channel].reduce((a, b) => a + b, 0)], [1, 1, 2], channel);
const bwImage = M.histogramImage(bw);
assert.equal(bwImage.length, 256 * 100 * 4);
assert.deepEqual(px(bwImage, 256, 0, 99), [225, 240, 244, 255], 'bin 0: the R, G and B bars at height 100, each adding half its colour to BG');
assert.deepEqual(px(bwImage, 256, 0, 0), LUMA, 'the luma line tops bin 0');
assert.deepEqual(px(bwImage, 256, 1, 50), LUMA, 'and falls to the floor between bin 0 and bin 1');
assert.deepEqual(px(bwImage, 256, 128, 50), BG, 'an empty bin is background');
assert.deepEqual(px(bwImage, 256, 128, 99), LUMA, 'the luma line runs along the floor where no pixel is');
// The peak ignores the end bins. With 100 black pixels and one (60, 120, 200), the peak is 1: the single pixel's bars
// reach the top while the black bins clip at 100.
const box = M.histogramImage(M.analyse(frame(101, 1, x => (x === 100 ? [60, 120, 200] : [0, 0, 0]))));
assert.deepEqual(px(box, 256, 60, 0), [140, 55, 61, 255], 'red bar at bin 60 is full height (BG + half of HR)');
assert.deepEqual(px(box, 256, 120, 0), [52, 130, 76, 255], 'green bar at bin 120');
assert.deepEqual(px(box, 256, 200, 0), [57, 85, 149, 255], 'blue bar at bin 200');
assert.deepEqual(px(box, 256, 0, 50), [225, 240, 244, 255], 'black clips at 100');
assert(M.histogramImage(null).every((value, index) => value === BG[index % 4]), 'no analysis: all background');

// Waveform: column floor(x·256/w), row 127 − (Y8 >> 1).
for (let x = 0; x < 256; x++) assert.equal(ramp.waveform[(127 - (x >> 1)) * 256 + x], 4, 'ramp column ' + x);
assert.equal(ramp.waveform.reduce((a, b) => a + b, 0), 1024);
const vertical = M.analyse(frame(256, 256, (x, y) => [y, y, y]));
let wrong = 0;
for (let i = 0; i < 256 * 128; i++) if (vertical.waveform[i] !== 2) wrong++;
assert.equal(wrong, 0, 'a vertical ramp puts 2 counts in every row of every column');
const wide = M.analyse(frame(512, 1, () => [255, 255, 255]));
for (let x = 0; x < 256; x++) assert.equal(wide.waveform[x], 2, 'a 512-wide frame folds two columns into one: ' + x);

// Vectorscope: Cb to the right, Cr up.
const colours = M.analyse(frame(3, 1, x => [[255, 0, 0], [0, 0, 255], [128, 128, 128]][x])).vectorscope;
assert.deepEqual([colours[0 * 128 + 49], colours[69 * 128 + 127], colours[64 * 128 + 64], colours.reduce((a, b) => a + b, 0)], [1, 1, 1, 3], 'red in (49, 0), blue in (127, 69), grey in (64, 64)');

// Intensity: v(n) = min(1, ln(1 + n)/ln(1 + K)), K = samples/1024 (waveform, 36 here) or samples/2304 (vectorscope, 16).
const dot = M.analyse(frame(256, 144, (x, y) => (x === 10 && y === 0 ? [255, 255, 255] : [0, 0, 0])));
const wave = M.waveformImage(dot);
assert.equal(wave.length, 256 * 128 * 4);
assert.deepEqual(px(wave, 256, 10, 0), [37, 61, 49, 255], 'one sample: v = ln 2 / ln 37');
assert.deepEqual(px(wave, 256, 10, 127), TRACE, '143 samples saturate');
assert.deepEqual(px(wave, 256, 11, 0), BG, 'an empty bin is background');
const scope = M.vectorscopeImage(dot);
assert.equal(scope.length, 128 * 128 * 4);
assert.deepEqual(px(scope, 128, 64, 64), TRACE, 'black and white are grey: one saturated bin');
assert.deepEqual(px(scope, 128, 65, 64), BG);
assert(M.waveformImage(null).every((value, index) => value === BG[index % 4]) && M.vectorscopeImage(null).length === 128 * 128 * 4);

// Graticule primitives in continuous plot pixels.
assert.deepEqual(M.graticule('histogram'), [64, 128, 192].map(x => ({kind: 'line', x0: x, y0: 0, x1: x, y1: 100, color: M.COLORS.GRID})));
assert.deepEqual(M.graticule('waveform'), [127.5, 95.5, 63.5, 32.5, 0.5].map(y => ({kind: 'line', x0: 0, y0: y, x1: 256, y1: y, color: M.COLORS.GRID})), '0, 25, 50, 75 and 100 %');
const vector = M.graticule('vectorscope');
assert.deepEqual(vector.slice(0, 4), [
  {kind: 'line', x0: 64, y0: 0, x1: 64, y1: 128, color: M.COLORS.GRID}, {kind: 'line', x0: 0, y0: 64, x1: 128, y1: 64, color: M.COLORS.GRID},
  {kind: 'circle', cx: 64, cy: 64, r: 32, color: M.COLORS.GRID}, {kind: 'circle', cx: 64, cy: 64, r: 64, color: M.COLORS.GRID}]);
const skin = vector[4];
assert.deepEqual([skin.kind, skin.x0, skin.y0, skin.color], ['line', 64, 64, M.COLORS.SKIN]);
near(skin.x1, 19.146, 0.002, 'skin line end x');
near(skin.y1, 18.348, 0.002, 'skin line end y');
const targets = vector.slice(5);
assert.equal(targets.length, 6);
assert(targets.every(item => item.kind === 'rect' && item.w === 5 && item.h === 5 && item.color === M.COLORS.TARGET), 'six 5 x 5 targets');
near(targets[0].x + 2.5, 53.02, 0.01, 'red target x');
near(targets[0].y + 2.5, 16.06, 0.01, 'red target y');
assert.deepEqual(M.graticule('parade'), []);
assert.notStrictEqual(M.graticule('histogram'), M.graticule('histogram'), 'a fresh list per call');

// Throttle.
assert.equal(M.due(0, 99, true), false, 'playing: not before 100 ms');
assert.equal(M.due(0, 100, true), true, 'playing: at 100 ms');
assert.equal(M.due(500, 500, false), true, 'paused: always');

// Stored preference.
const all = ['histogram', 'waveform', 'vectorscope'];
assert.deepEqual(M.prefs('{oops'), {open: false, show: all}, 'invalid JSON');
assert.deepEqual(M.prefs(null), {open: false, show: all}, 'nothing stored');
assert.deepEqual(M.prefs('[1,2]'), {open: false, show: all}, 'not an object');
assert.deepEqual(M.prefs('{"open":true,"show":["vectorscope","bogus","histogram"]}'), {open: true, show: ['histogram', 'vectorscope']}, 'unknown kinds dropped, KINDS order');
assert.deepEqual(M.prefs('{"open":true,"show":[]}'), {open: true, show: all}, 'an empty show means all three');
assert.deepEqual(M.prefs('{"open":"yes","show":"waveform"}'), {open: false, show: all}, 'open must be true, show a list');
process.stdout.write('ok');
`)
}

// TestVideoStudioScopesPanel checks the scopes panel (2f Task 3; index §4.26.2, §4.26.5) with stub elements, a preview
// whose readScaled promises the test resolves by hand, and a hand-set performance.now:
//   - wireScopes inserts, once, the toggle before "Render range exactly" and the dock after the timeline stage, with the
//     ARIA of §4.26.5, and reads the stored preference;
//   - a closed panel reads nothing; opening reads at once and leaves focus where it is;
//   - playing: a read at 0 ms, none at 50 ms, one at 100 ms; paused: every paint reads;
//   - a due paint during a pending read sets dirty, and the end of that read starts exactly one trailing read, without
//     a paint; a paint that is not due during a pending read is dropped;
//   - a null frame draws nothing; drawScopes puts one image per shown kind and then strokes its graticule;
//   - toggleScope keeps the last shown kind; Escape inside the panel closes it and focuses the toggle; the tile row
//     keeps its scroll keys;
//   - source mode and a closed panel read nothing; without a project the toggle is disabled and the panel stays shut;
//   - every change of open or show is written to aurago.videoStudio.scopes.
func TestVideoStudioScopesPanel(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-scope-math.js');
let clock = 1000;
global.performance = {now: () => clock};
const store = new Map(), focused = [];
window.VideoStudioParts = {
  tr: (s, key) => '[' + key + ']',
  esc: (s, value) => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';'),
  icon: (name, size) => '<i data-icon="' + name + '" data-size="' + size + '"></i>',
  readPref: key => (store.has(key) ? store.get(key) : null),
  writePref: (key, value) => { store.set(key, String(value)); }
};
require('./js/desktop/apps/video-studio-scopes.js');
const P = window.VideoStudioParts, M = window.VideoStudioScopeMath, PREF = 'aurago.videoStudio.scopes';
function node(name) {
  return {name, attrs: {}, hidden: false, disabled: false, dataset: {}, inserted: [],
    setAttribute(key, value) { this.attrs[key] = String(value); },
    getAttribute(key) { return Object.prototype.hasOwnProperty.call(this.attrs, key) ? this.attrs[key] : null; },
    insertAdjacentHTML(where, html) { this.inserted.push([where, html]); },
    focus() { focused.push(name); },
    closest() { return null; }};
}
function drawing() {
  const calls = [];
  return {calls, strokeStyle: '', lineWidth: 0,
    createImageData: (width, height) => ({width, height, data: new Uint8ClampedArray(width * height * 4)}),
    putImageData(image, x, y) { calls.push(['put', image.width, image.height, x, y]); },
    save() {}, restore() {}, beginPath() {}, moveTo() {}, lineTo() {}, arc() {}, rect() {},
    stroke() { calls.push(['stroke', this.strokeStyle, this.lineWidth]); }};
}
function makeState(id) {
  const els = {range: node('range'), stage: node('stage'), toggle: node('toggle'), section: node('section'), tiles: node('tiles')};
  els.toggle.closest = selector => (selector === '[data-action="scopes"]' ? els.toggle : null);
  els.tiles.closest = selector => (selector === '.vs-scopes-tiles' ? els.tiles : selector === '[data-scopes]' ? els.section : null);
  const selectors = {'.vs-transport-right [data-action="range-render"]': els.range, '.vs-preview-stage[data-stage="timeline"]': els.stage, '[data-action="scopes"]': els.toggle, '[data-scopes]': els.section};
  M.KINDS.forEach(kind => {
    const button = node('button ' + kind), tile = node('tile ' + kind), canvas = node('canvas ' + kind), ctx = drawing();
    button.dataset.scope = kind;
    button.closest = selector => (selector === '[data-scope]' ? button : selector === '[data-scopes]' ? els.section : null);
    canvas.getContext = type => (type === '2d' ? ctx : null);
    canvas.ctx = ctx;
    els['button ' + kind] = button; els['tile ' + kind] = tile; els['canvas ' + kind] = canvas;
    selectors['[data-scope="' + kind + '"]'] = button;
    selectors['[data-scope-tile="' + kind + '"]'] = tile;
    selectors['[data-scope-canvas="' + kind + '"]'] = canvas;
  });
  const reads = [];
  const preview = {playing: false, isPlaying() { return this.playing; }, seek() { throw new Error('the panel never paints'); },
    readScaled(width, height) { let resolve; const promise = new Promise(done => { resolve = done; }); reads.push({width, height, resolve}); return promise; }};
  const s = {id, project: {width: 1280, height: 720}, stageMode: 'timeline', preview, q: selector => selectors[selector] || null};
  return {s, els, reads};
}
const flush = () => new Promise(resolve => setImmediate(resolve));
const grey = (() => { const data = new Uint8ClampedArray(256 * 144 * 4).fill(128); for (let i = 3; i < data.length; i += 4) data[i] = 255; return {width: 256, height: 144, data}; })();
const puts = els => M.KINDS.map(kind => els['canvas ' + kind].ctx.calls.filter(call => call[0] === 'put').length);
const stored = () => JSON.parse(store.get(PREF));
(async () => {
  const {s, els, reads} = makeState('kf');
  assert.equal(P.wireScopes(s), true);
  // Markup.
  assert.deepEqual(els.range.inserted.map(([where]) => where), ['beforebegin']);
  assert.equal(els.range.inserted[0][1], '<button type="button" data-action="scopes" aria-expanded="false" aria-controls="vs-scopes-kf" aria-label="[scopes]" title="[scopesHint]"><i data-icon="scopes" data-size="17"></i></button>');
  assert.deepEqual(els.stage.inserted.map(([where]) => where), ['afterend']);
  const dock = els.stage.inserted[0][1];
  assert(dock.startsWith('<div class="vs-scopes-dock" data-stage="timeline"><section class="vs-scopes" id="vs-scopes-kf" data-scopes role="region" aria-label="[scopes]" hidden><div class="vs-segmented vs-scopes-bar" role="group" aria-label="[scopesShow]">'), dock);
  assert.deepEqual(Array.from(dock.matchAll(/<button type="button" data-scope="([a-z]+)" aria-pressed="true">\[(\w+)\]<\/button>/g), m => m.slice(1)), [['histogram', 'scopeHistogram'], ['waveform', 'scopeWaveform'], ['vectorscope', 'scopeVectorscope']]);
  assert(dock.includes('<div class="vs-scopes-tiles" role="group" aria-label="[scopes]" tabindex="0">'));
  assert.deepEqual(Array.from(dock.matchAll(/<figure class="vs-scope" data-scope-tile="([a-z]+)"><canvas data-scope-canvas="([a-z]+)" width="(\d+)" height="(\d+)" role="img" aria-label="\[(\w+)\]"><\/canvas><\/figure>/g), m => m.slice(1)),
    [['histogram', 'histogram', '256', '100', 'scopeHistogramLabel'], ['waveform', 'waveform', '256', '128', 'scopeWaveformLabel'], ['vectorscope', 'vectorscope', '128', '128', 'scopeVectorscopeLabel']]);
  assert(dock.endsWith('</section></div>'));
  // State: closed by default with all three kinds.
  assert.deepEqual({open: s.scopes.open, show: Array.from(s.scopes.show), last: s.scopes.last, pending: s.scopes.pending, dirty: s.scopes.dirty}, {open: false, show: ['histogram', 'waveform', 'vectorscope'], last: 0, pending: false, dirty: false});
  assert.deepEqual([els.toggle.attrs['aria-expanded'], els.toggle.disabled, els.section.hidden], ['false', false, true]);
  assert.equal(P.wireScopes(s), false, 'wireScopes runs once');
  assert.equal(els.range.inserted.length, 1);
  P.scopesAfterPaint(s);
  assert.equal(reads.length, 0, 'a closed panel reads nothing');

  // Opening reads the last frame at once; focus stays where it is (on the toggle).
  clock = 5000;
  assert.equal(P.handleScopeClick(s, {target: els.toggle}), true);
  assert.deepEqual([s.scopes.open, els.toggle.attrs['aria-expanded'], els.section.hidden], [true, 'true', false]);
  assert.deepEqual(reads.map(read => [read.width, read.height]), [[256, 144]]);
  assert.deepEqual(stored(), {open: true, show: ['histogram', 'waveform', 'vectorscope']});
  assert.deepEqual(focused, []);
  reads[0].resolve(grey); await flush();
  assert.deepEqual(puts(els), [1, 1, 1], 'each shown scope gets its image');
  for (const kind of M.KINDS) {
    const calls = els['canvas ' + kind].ctx.calls, size = {histogram: M.HISTOGRAM, waveform: M.WAVEFORM, vectorscope: M.VECTORSCOPE}[kind];
    assert.deepEqual(calls[0], ['put', size.width, size.height, 0, 0], kind + ': the image at the origin');
    assert.deepEqual(calls.slice(1).map(call => call[1]), M.graticule(kind).map(item => item.color), kind + ': then one stroke per graticule primitive');
    assert(calls.slice(1).every(call => call[0] === 'stroke' && call[2] === 1), kind + ': lineWidth 1');
  }

  // Playing: at most every 100 ms.
  s.preview.playing = true;
  clock = 6000; P.scopesAfterPaint(s);
  assert.equal(reads.length, 2, 'playing: a read at 0 ms');
  reads[1].resolve(grey); await flush();
  clock = 6050; P.scopesAfterPaint(s);
  assert.equal(reads.length, 2, 'none at 50 ms');
  clock = 6100; P.scopesAfterPaint(s);
  assert.equal(reads.length, 3, 'one at 100 ms');
  clock = 6150; P.scopesAfterPaint(s);
  assert.equal(s.scopes.dirty, false, 'a paint that is not due during a pending read is dropped');
  reads[2].resolve(grey); await flush();
  assert.equal(reads.length, 3, 'so no trailing read follows');
  clock = 6200; P.scopesAfterPaint(s);
  assert.equal(reads.length, 4);
  clock = 6300; P.scopesAfterPaint(s);
  assert.equal(s.scopes.dirty, true, 'a due paint during a pending read sets dirty');
  reads[3].resolve(grey); await flush();
  assert.equal(reads.length, 5, 'and the read ends with one trailing read');
  reads[4].resolve(grey); await flush();
  assert.equal(reads.length, 5);

  // Paused: every paint reads; paints during a pending read end in exactly one trailing read, without a paint.
  s.preview.playing = false;
  clock = 6301;
  for (let i = 0; i < 3; i++) { P.scopesAfterPaint(s); assert.equal(reads.length, 6 + i, 'paused: every paint reads'); reads.at(-1).resolve(grey); await flush(); }
  P.scopesAfterPaint(s);
  const pendingIndex = reads.length - 1;
  P.scopesAfterPaint(s); P.scopesAfterPaint(s);
  assert.equal(reads.length, pendingIndex + 1, 'one read in flight');
  assert.deepEqual([s.scopes.pending, s.scopes.dirty], [true, true]);
  reads[pendingIndex].resolve(grey); await flush();
  assert.equal(reads.length, pendingIndex + 2, 'exactly one trailing read of the last frame');
  assert.equal(s.scopes.dirty, false);
  reads.at(-1).resolve(grey); await flush();
  assert.equal(reads.length, pendingIndex + 2, 'and nothing after it');

  // A null frame draws nothing.
  const before = puts(els);
  P.scopesAfterPaint(s); reads.at(-1).resolve(null); await flush();
  assert.deepEqual(puts(els), before, 'a null frame draws nothing');
  assert.equal(P.drawScopes(s, null), false);

  // Kinds: a change reads again; only shown kinds are drawn; the last shown kind stays.
  const readsBefore = reads.length;
  assert.equal(P.toggleScope(s, 'waveform'), true);
  assert.deepEqual([els['button waveform'].attrs['aria-pressed'], els['tile waveform'].hidden], ['false', true], 'waveform off hides its tile');
  assert.deepEqual(stored().show, ['histogram', 'vectorscope']);
  assert.equal(reads.length, readsBefore + 1, 'a kind change reads the last frame again');
  const drawn = puts(els);
  reads.at(-1).resolve(grey); await flush();
  assert.deepEqual(puts(els).map((n, i) => n - drawn[i]), [1, 0, 1], 'only the shown kinds are drawn');
  assert.equal(P.toggleScope(s, 'histogram'), true);
  reads.at(-1).resolve(grey); await flush();
  assert.equal(P.toggleScope(s, 'vectorscope'), false, 'the last shown kind stays');
  assert.deepEqual([els['button vectorscope'].attrs['aria-pressed'], els['tile vectorscope'].hidden, Array.from(s.scopes.show)], ['true', false, ['vectorscope']]);
  assert.deepEqual(stored(), {open: true, show: ['vectorscope']});
  assert.equal(P.toggleScope(s, 'parade'), false, 'unknown kinds change nothing');
  assert.equal(P.handleScopeClick(s, {target: els['button waveform']}), true, 'the kind buttons go through handleScopeClick');
  assert.deepEqual(stored().show, ['waveform', 'vectorscope']);
  reads.at(-1).resolve(grey); await flush();

  // Keys: the tile row keeps its scroll keys; Escape closes and focuses the toggle.
  const prevented = [];
  const key = (name, target) => ({key: name, target, ctrlKey: false, metaKey: false, altKey: false, preventDefault() { prevented.push(name); }});
  assert.equal(P.handleScopeKeys(s, key('ArrowRight', els.tiles)), true, 'the tile row keeps its scroll keys');
  assert.equal(P.handleScopeKeys(s, key('ArrowRight', els['button waveform'])), false, 'the buttons keep their own keys');
  assert.equal(P.handleScopeKeys(s, key('Escape', els['button waveform'])), true);
  assert.deepEqual([prevented, s.scopes.open, els.section.hidden, els.toggle.attrs['aria-expanded'], focused], [['Escape'], false, true, 'false', ['toggle']], 'Escape closes the panel and focuses the toggle');
  assert.equal(stored().open, false);
  assert.equal(P.handleScopeKeys(s, key('Escape', els['button waveform'])), false, 'a closed panel takes no keys');
  const closedCount = reads.length;
  clock += 1000; P.scopesAfterPaint(s);
  assert.equal(reads.length, closedCount, 'closed: no read');

  // Source mode reads nothing.
  assert.equal(P.setScopesOpen(s, true), true);
  reads.at(-1).resolve(grey); await flush();
  s.stageMode = 'source';
  const sourceCount = reads.length;
  clock += 1000; P.scopesAfterPaint(s);
  assert.equal(reads.length, sourceCount, 'source mode reads nothing');
  s.stageMode = 'timeline';

  // Without a project the toggle is disabled and the panel does not open.
  s.project = null; P.syncScopes(s);
  assert.deepEqual([els.toggle.disabled, els.toggle.attrs['aria-expanded'], els.section.hidden], [true, 'false', true]);
  assert.equal(P.setScopesOpen(s, true), false);
  assert.equal(s.scopes.open, true, 'the remembered state stays');
  s.project = {width: 1280, height: 720}; P.syncScopes(s);
  assert.deepEqual([els.toggle.disabled, els.section.hidden], [false, false]);

  // The stored preference opens a new window's panel; the next paint reads.
  store.set(PREF, '{"open":true,"show":["waveform","bogus"]}');
  const second = makeState('w2');
  assert.equal(P.wireScopes(second.s), true);
  assert.deepEqual([second.s.scopes.open, Array.from(second.s.scopes.show)], [true, ['waveform']]);
  assert.deepEqual(M.KINDS.map(kind => [second.els['button ' + kind].attrs['aria-pressed'], second.els['tile ' + kind].hidden]), [['false', true], ['true', false], ['false', true]]);
  assert.deepEqual([second.els.toggle.attrs['aria-expanded'], second.els.section.hidden], ['true', false]);
  assert.equal(second.reads.length, 0, 'wiring reads nothing');
  P.scopesAfterPaint(second.s);
  assert.equal(second.reads.length, 1);
  process.stdout.write('ok');
})().catch(error => { console.error(error); process.exit(1); });
`)
}

// TestVideoStudioScopesLocales checks the nine scope strings (2f Task 3; index §4.26.5) in all 16 desktop locales:
//   - every key is present, and en and de are the binding texts;
//   - the hint, "Scopes to show" and the three canvas labels are translated;
//   - the four short names may equal English (nl, da, no, sv, pl and cs say "Histogram" too).
func TestVideoStudioScopesLocales(t *testing.T) {
	binding := map[string][2]string{
		"scopes":                {"Scopes", "Videoskope"},
		"scopesHint":            {"Show the histogram, waveform and vectorscope of the preview", "Histogramm, Wellenform und Vektorskop der Vorschau zeigen"},
		"scopesShow":            {"Scopes to show", "Angezeigte Videoskope"},
		"scopeHistogram":        {"Histogram", "Histogramm"},
		"scopeWaveform":         {"Waveform", "Wellenform"},
		"scopeVectorscope":      {"Vectorscope", "Vektorskop"},
		"scopeHistogramLabel":   {"Histogram of the preview: red, green, blue and luma (BT.709), dark on the left, bright on the right", "Histogramm der Vorschau: Rot, Grün, Blau und Luma (BT.709), dunkel links, hell rechts"},
		"scopeWaveformLabel":    {"Waveform of the preview: luma (BT.709) of each column from left to right, 0 % at the bottom, 100 % at the top", "Wellenform der Vorschau: Luma (BT.709) jeder Spalte von links nach rechts, 0 % unten, 100 % oben"},
		"scopeVectorscopeLabel": {"Vectorscope of the preview: colour as BT.709 Cb to the right and Cr upwards, with the skin-tone line", "Vektorskop der Vorschau: Farbe als BT.709-Cb nach rechts und -Cr nach oben, mit der Hautton-Linie"},
	}
	short := map[string]bool{"scopes": true, "scopeHistogram": true, "scopeWaveform": true, "scopeVectorscope": true}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for key := range binding {
			text := strings.TrimSpace(values["videoStudio."+key])
			switch {
			case text == "":
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			case lang != "en" && !short[key] && text == english["videoStudio."+key]:
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
	for key, texts := range binding {
		for i, lang := range []string{"en", "de"} {
			if got := readDesktopLocale(t, lang)["videoStudio."+key]; got != texts[i] {
				t.Errorf("%s: videoStudio.%s = %q, want %q", lang, key, got, texts[i])
			}
		}
	}
}
