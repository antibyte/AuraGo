package ui

import "testing"

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
