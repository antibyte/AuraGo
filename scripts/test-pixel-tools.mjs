// WP4 tools: pure geometry / pixel helpers (brush path, symmetry, snapping, free transform, crop, shapes, gradient, flood fill,
// diffusion fill, red-eye) and the registrations of the tools in the real module set (fake DOM, no pixels are rendered).
import assert from 'node:assert/strict';
import { installFakeDom, makeEl, loadPixelApp } from './pixel-test-dom.mjs';

installFakeDom();
const errors = [];
const originalError = console.error;
console.error = (...args) => { errors.push(args.map(String).join(' ')); };
const Pixel = loadPixelApp();
const { brush: Brush, toolsPro: Pro, transformMath: T } = Pixel;
const lib = window.AuraPixelLib;

let count = 0;
function check(name, fn) {
    try { fn(); count++; console.log('ok   ' + name); } catch (err) { console.log('FAIL ' + name + ': ' + (err && err.stack || err)); process.exitCode = 1; }
}
const near = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) <= eps, a + ' !~ ' + b);

// --- brush path ---

check('path: dabs sit on the line at the requested spacing', () => {
    const dabs = [];
    const path = Brush.createPath({ spacing: () => 10, emit: (x, y, pr, ang) => dabs.push({ x, y, pr, ang }) });
    path.begin(0, 0, 1);
    path.move(50, 0, 1);
    path.finish(100, 0, 1);
    assert.equal(dabs.length, 11, 'first dab plus one every 10 px up to 100');
    dabs.forEach((d, i) => { near(d.x, i * 10, 1e-6); near(d.y, 0, 1e-6); });
    near(dabs[5].ang, 0);
});
check('path: a curve keeps the spacing along its length and pressure is interpolated', () => {
    const dabs = [];
    const path = Brush.createPath({ spacing: () => 5, emit: (x, y, pr) => dabs.push({ x, y, pr }) });
    path.begin(0, 0, 0.2);
    for (let i = 1; i <= 20; i++) path.move(i * 5, Math.sin(i / 3) * 20, 0.2 + i * 0.04);
    path.finish(100, Math.sin(20 / 3) * 20, 1);
    assert.ok(dabs.length > 20);
    for (let i = 1; i < dabs.length; i++) { const d = Math.hypot(dabs[i].x - dabs[i - 1].x, dabs[i].y - dabs[i - 1].y); assert.ok(d <= 5.05 && d > 2, 'gap ' + d); }
    assert.ok(dabs[dabs.length - 1].pr > dabs[1].pr, 'pressure grows along the stroke');
});
check('path: the leash lags behind the pointer but finish() reaches it', () => {
    const dabs = [];
    const path = Brush.createPath({ leash: 30, spacing: () => 2, emit: (x, y) => dabs.push({ x, y }) });
    path.begin(0, 0, 1);
    path.move(20, 0, 1);
    assert.equal(dabs.length, 1, 'within the leash the brush does not move');
    path.move(100, 0, 1);
    assert.ok(dabs[dabs.length - 1].x <= 70 + 1e-6, 'brush stays 30 px behind');
    path.finish(100, 0, 1);
    near(dabs[dabs.length - 1].x, 100, 2.01);
});
check('path: silent begin continues a line without a double dab', () => {
    const dabs = [];
    const path = Brush.createPath({ spacing: () => 10, emit: (x) => dabs.push(x) });
    path.begin(0, 0, 1, true);
    path.finish(30, 0, 1);
    assert.deepEqual(dabs.map(Math.round), [10, 20, 30]);
});

check('symmetry: mirrors and radial copies', () => {
    assert.equal(Brush.symmetryPoints('off', 4, 100, 80, 10, 20, 0).length, 1);
    const v = Brush.symmetryPoints('vertical', 4, 100, 80, 10, 20, 0);
    assert.deepEqual(v.map(p => [p.x, p.y]), [[10, 20], [90, 20]]);
    const b = Brush.symmetryPoints('both', 4, 100, 80, 10, 20, 0);
    assert.deepEqual(b.map(p => [p.x, p.y]), [[10, 20], [90, 20], [10, 60], [90, 60]]);
    const rad = Brush.symmetryPoints('radial', 4, 100, 100, 70, 50, 0);
    assert.equal(rad.length, 4);
    near(rad[1].x, 50); near(rad[1].y, 70); near(rad[2].x, 30); near(rad[2].y, 50);
    assert.equal(Brush.symmetryPoints('radial', 6, 100, 100, 70, 50, 0).length, 6);
});

check('falloff, pressure and rng helpers', () => {
    assert.equal(Brush.falloff(0.99, 100), 1);
    assert.equal(Brush.falloff(1, 50), 0);
    assert.equal(Brush.falloff(0, 0), 1);
    assert.ok(Brush.falloff(0.25, 0) > Brush.falloff(0.5, 0) && Brush.falloff(0.5, 0) > Brush.falloff(0.75, 0));
    assert.equal(Brush.pressureOf({ pointerType: 'mouse', pressure: 0.5 }), 1);
    assert.equal(Brush.pressureOf({ pointerType: 'touch', pressure: 0 }), 1);
    assert.equal(Brush.pressureOf({ pointerType: 'pen', pressure: 0.4 }), 0.4);
    assert.deepEqual(Brush.pressureFactors('off', 0.3), { size: 1, alpha: 1 });
    assert.deepEqual(Brush.pressureFactors('both', 0.3), { size: 0.3, alpha: 0.3 });
    const a = Brush.makeRng(5), b = Brush.makeRng(5);
    assert.equal(a(), b());
});

// --- transform geometry ---

check('snapBox: snaps to canvas centre and reports the guide', () => {
    const lines = { xs: [0, 50, 100], ys: [0, 40, 80] };
    const sn = T.snapBox({ x0: 30, y0: 10, x1: 66, y1: 30 }, lines, 4);
    assert.equal(sn.dx, 2, 'centre 48 -> 50');
    assert.deepEqual(sn.xs, [50]);
    assert.equal(sn.dy, 0);
    assert.deepEqual(sn.ys, []);
    const none = T.snapBox({ x0: 20, y0: 10, x1: 40, y1: 30 }, lines, 4);
    assert.equal(none.dx, 0); assert.deepEqual(none.xs, []);
    const edge = T.snapBox({ x0: -3, y0: 5, x1: 20, y1: 30 }, lines, 6);
    assert.equal(edge.dx, 3); assert.deepEqual(edge.xs, [0]);
});

check('free transform: matrix, quad and hit test', () => {
    const g = { cx: 50, cy: 40, sx: 2, sy: 1, rot: 0 };
    const q = T.quad(g, 20, 10);
    assert.deepEqual(q.map(p => [Math.round(p.x), Math.round(p.y)]), [[30, 35], [70, 35], [70, 45], [30, 45]]);
    const r90 = T.quad({ cx: 0, cy: 0, sx: 1, sy: 1, rot: Math.PI / 2 }, 20, 10);
    near(r90[0].x, 5); near(r90[0].y, -10);
    assert.equal(T.hitTest(g, 20, 10, { x: 50, y: 40 }, 4).kind, 'move');
    assert.equal(T.hitTest(g, 20, 10, { x: 90, y: 40 }, 4).kind, 'rotate');
    const h = T.hitTest(g, 20, 10, { x: 70, y: 45 }, 4);
    assert.deepEqual([h.kind, h.hx, h.hy], ['scale', 1, 1]);
});
check('free transform: scale handles, shift ratio, alt from centre, flip', () => {
    const g0 = { cx: 50, cy: 50, sx: 1, sy: 1, rot: 0 };
    const edge = T.scaleDrag(g0, 1, 0, { x: 80, y: 77 }, 40, 20, false, false);
    near(edge.sx, 1.25); near(edge.sy, 1); near(edge.cx, 55); near(edge.cy, 50);
    const corner = T.scaleDrag(g0, 1, 1, { x: 90, y: 70 }, 40, 20, false, false);
    near(corner.sx, 1.5); near(corner.sy, 1.5); near(corner.cx, 60); near(corner.cy, 55);
    const uniform = T.scaleDrag(g0, 1, 1, { x: 90, y: 62 }, 40, 20, true, false);
    near(uniform.sx, uniform.sy);
    near(uniform.sx, 1.5);
    const centre = T.scaleDrag(g0, 1, 0, { x: 90, y: 50 }, 40, 20, false, true);
    near(centre.sx, 2); near(centre.cx, 50);
    const flip = T.scaleDrag(g0, 1, 0, { x: 20, y: 50 }, 40, 20, false, false);
    assert.ok(flip.sx < 0, 'dragging past the opposite edge flips');
    near(flip.cx, 25);
    const rotated = T.scaleDrag({ cx: 0, cy: 0, sx: 1, sy: 1, rot: Math.PI / 2 }, 1, 0, { x: 0, y: 30 }, 20, 10, false, false);
    near(rotated.sx, 2);
});

check('crop: handle drags, ratio, move, hit test', () => {
    const rect = { x0: 10, y0: 10, x1: 60, y1: 40 };
    const free = T.cropDrag(rect, 1, 1, { x: 80, y: 70 }, null, 100, 100);
    assert.deepEqual(free, { x0: 10, y0: 10, x1: 80, y1: 70 });
    const clamped = T.cropDrag(rect, 1, 1, { x: 150, y: 200 }, null, 100, 100);
    assert.deepEqual(clamped, { x0: 10, y0: 10, x1: 100, y1: 100 });
    const sq = T.cropDrag(rect, 1, 1, { x: 90, y: 50 }, 1, 100, 100);
    near(sq.x1 - sq.x0, sq.y1 - sq.y0);
    near(sq.x1 - sq.x0, 80);
    const wide = T.cropDrag({ x0: 0, y0: 0, x1: 0, y1: 0 }, 1, 1, { x: 100, y: 10 }, 16 / 9, 100, 100);
    near((wide.x1 - wide.x0) / (wide.y1 - wide.y0), 16 / 9, 1e-9);
    assert.ok(wide.x1 <= 100 && wide.y1 <= 100);
    const left = T.cropDrag(rect, -1, 0, { x: 30, y: 0 }, null, 100, 100);
    assert.deepEqual(left, { x0: 30, y0: 10, x1: 60, y1: 40 });
    const reversed = T.cropDrag({ x0: 40, y0: 40, x1: 40, y1: 40 }, 1, 1, { x: 10, y: 5 }, null, 100, 100);
    assert.deepEqual(reversed, { x0: 10, y0: 5, x1: 40, y1: 40 });
    assert.deepEqual(T.cropMove(rect, 100, -50, 100, 100), { x0: 50, y0: 0, x1: 100, y1: 30 });
    assert.equal(T.hitCrop(rect, { x: 10, y: 25 }, 3).hx, -1);
    assert.equal(T.hitCrop(rect, { x: 60, y: 40 }, 3).hy, 1);
    assert.equal(T.hitCrop(rect, { x: 30, y: 25 }, 3).kind, 'move');
    assert.equal(T.hitCrop(rect, { x: 90, y: 90 }, 3).kind, 'outside');
    const fit = T.cropRatio({ x0: 0, y0: 0, x1: 100, y1: 100 }, 2);
    near((fit.x1 - fit.x0) / (fit.y1 - fit.y0), 2);
});
check('shiftAlpha drops what leaves the canvas', () => {
    const a = new Uint8ClampedArray(16);
    a[5] = 255;
    const s = T.shiftAlpha(a, 4, 4, 1, 1);
    assert.equal(s[10], 255);
    assert.equal(s.reduce((x, y) => x + y, 0), 255);
    assert.equal(T.shiftAlpha(a, 4, 4, 3, 3).reduce((x, y) => x + y, 0), 0);
});

// --- shapes, gradient, fill ---

check('shapes: constrain and from-centre geometry, polygon and star vertices', () => {
    assert.deepEqual(Pro.shapeGeo('rectangle', { x0: 10, y0: 10, x1: 50, y1: 30 }), { x0: 10, y0: 10, x1: 50, y1: 30 });
    assert.deepEqual(Pro.shapeGeo('rectangle', { x0: 10, y0: 10, x1: 50, y1: 30, shift: true }), { x0: 10, y0: 10, x1: 50, y1: 50 });
    assert.deepEqual(Pro.shapeGeo('ellipse', { x0: 50, y0: 50, x1: 60, y1: 70, alt: true }), { x0: 40, y0: 30, x1: 60, y1: 70 });
    const line = Pro.shapeGeo('line', { x0: 0, y0: 0, x1: 40, y1: 5, shift: true });
    near(line.y1, 0, 1e-9); near(line.x1, Math.hypot(40, 5), 1e-9);
    const hex = Pro.polyPoints(50, 50, 10, 10, 6, null);
    assert.equal(hex.length, 6);
    near(hex[0].x, 50); near(hex[0].y, 40);
    const star = Pro.polyPoints(50, 50, 10, 10, 5, 0.5);
    assert.equal(star.length, 10);
    near(Math.hypot(star[1].x - 50, star[1].y - 50), 5);
    near(Math.hypot(star[2].x - 50, star[2].y - 50), 10);
});

check('gradient: linear, reflected, radial, conic and reverse', () => {
    const lut = lib.gradientLUT([{ pos: 0, color: '#000000' }, { pos: 1, color: '#ffffff' }], 256);
    const base = { x0: 0, y0: 0, x1: 100, y1: 0, lut, reverse: false, dither: false };
    const rect = { x: 0, y: 0, w: 100, h: 2 };
    const lin = Pro.gradientRender(Object.assign({ type: 'linear' }, base), rect, 1);
    assert.equal(lin.width, 100); assert.equal(lin.height, 2);
    assert.ok(lin.data[0] < 5 && lin.data[99 * 4] > 250 && lin.data[50 * 4] > 120 && lin.data[50 * 4] < 135);
    assert.equal(lin.data[3], 255);
    const rev = Pro.gradientRender(Object.assign({ type: 'linear' }, base, { reverse: true }), rect, 1);
    assert.ok(rev.data[0] > 250 && rev.data[99 * 4] < 5);
    const mid = { x0: 50, y0: 0, x1: 100, y1: 0, lut, reverse: false, dither: false };
    const refl = Pro.gradientRender(Object.assign({ type: 'reflected' }, mid), rect, 1);
    assert.ok(Math.abs(refl.data[29 * 4] - refl.data[70 * 4]) <= 1, 'symmetric around the start');
    const rad = Pro.gradientRender(Object.assign({ type: 'radial' }, mid, { y0: 50, y1: 50 }), { x: 0, y: 0, w: 100, h: 100 }, 0.5);
    assert.equal(rad.width, 50);
    assert.ok(rad.data[(25 * 50 + 25) * 4] < 20, 'centre is the first stop');
    const conic = Pro.gradientRender(Object.assign({ type: 'conic' }, { x0: 50, y0: 50, x1: 100, y1: 50, lut, reverse: false, dither: false }), { x: 0, y: 0, w: 100, h: 100 }, 1);
    const at = (x, y) => conic.data[(y * 100 + x) * 4];
    assert.ok(at(75, 51) < 15 && at(25, 51) > 120 && at(25, 51) < 135, 'angle 0 = first stop, half turn = middle');
    assert.equal(Pro.gradientRender(Object.assign({ type: 'linear' }, base, { x1: 0 }), rect, 1).data[3], 0, 'no drag, no gradient');
    const alphaLut = lib.gradientLUT([{ pos: 0, color: '#ff0000' }, { pos: 1, color: '#ff0000', alpha: 0 }], 256);
    const fade = Pro.gradientRender(Object.assign({ type: 'linear' }, base, { lut: alphaLut }), rect, 1);
    assert.ok(fade.data[3] > 250 && fade.data[99 * 4 + 3] < 5);
    const dith = Pro.gradientRender(Object.assign({ type: 'linear' }, base, { dither: true }), rect, 1);
    assert.ok(dith.data[50 * 4] > 118 && dith.data[50 * 4] < 138);
});

check('fill: contiguous flood, global flood, tolerance and soft edge', () => {
    const w = 8, h = 4, d = new Uint8ClampedArray(w * h * 4);
    for (let i = 0; i < w * h; i++) { d[i * 4] = d[i * 4 + 1] = d[i * 4 + 2] = 200; d[i * 4 + 3] = 255; }
    for (let y = 0; y < h; y++) { const i = (y * w + 4) * 4; d[i] = d[i + 1] = d[i + 2] = 0; }
    const left = Pro.floodRegion(d, w, h, 0, 0, 10, true);
    assert.equal(left.filter(v => v).length, 16, 'the black column stops the flood');
    const right = Pro.floodRegion(d, w, h, 7, 0, 10, true);
    assert.equal(right.filter(v => v).length, 12);
    const global = Pro.floodRegion(d, w, h, 0, 0, 10, false);
    assert.equal(global.filter(v => v).length, 28, 'global includes the far side');
    const loose = Pro.floodRegion(d, w, h, 0, 0, 255, true);
    assert.equal(loose.filter(v => v).length, 32);
    const soft = Pro.softenRegion(Pro.floodRegion(d, w, h, 0, 0, 10, true), w, h);
    assert.ok(soft[0] === 255 && soft[4] > 0 && soft[4] < 255, 'one soft pixel next to the region');
    const t = new Uint8ClampedArray(2 * 4);
    t[3] = 0; t[4] = 255; t[7] = 0;
    assert.equal(Pro.floodRegion(t, 2, 1, 0, 0, 0, true).filter(v => v).length, 2, 'transparent pixels are equal whatever their colour');
});

check('diffusion fill closes a hole in a ramp', () => {
    const w = 24, h = 24, img = { width: w, height: h, data: new Uint8ClampedArray(w * h * 4) }, mask = new Uint8ClampedArray(w * h);
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) { const i = (y * w + x) * 4; img.data[i] = x * 10; img.data[i + 1] = y * 10; img.data[i + 2] = 100; img.data[i + 3] = 255; }
    for (let y = 9; y < 15; y++) for (let x = 9; x < 15; x++) mask[y * w + x] = 255;
    const holed = { width: w, height: h, data: new Uint8ClampedArray(img.data) };
    for (let p = 0; p < w * h; p++) if (mask[p]) { holed.data[p * 4] = 0; holed.data[p * 4 + 1] = 0; holed.data[p * 4 + 2] = 0; }
    const out = Pro.diffusionFill(holed, mask);
    let worst = 0;
    for (let p = 0; p < w * h; p++) {
        if (!mask[p]) { assert.equal(out.data[p * 4], img.data[p * 4], 'outside the mask is untouched'); continue; }
        worst = Math.max(worst, Math.abs(out.data[p * 4] - img.data[p * 4]), Math.abs(out.data[p * 4 + 1] - img.data[p * 4 + 1]));
    }
    assert.ok(worst < 45, 'worst error ' + worst);
});

check('red-eye: the red disc is neutralised, the rest untouched', () => {
    const w = 20, h = 20, d = new Uint8ClampedArray(w * h * 4);
    for (let p = 0; p < w * h; p++) { d[p * 4] = 120; d[p * 4 + 1] = 110; d[p * 4 + 2] = 100; d[p * 4 + 3] = 255; }
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) if (Math.hypot(x - 10, y - 10) < 4) { const i = (y * w + x) * 4; d[i] = 230; d[i + 1] = 30; d[i + 2] = 30; }
    const n = Pro.fixRedEye({ width: w, height: h, data: d }, 9, 11, 12, 0.7);
    assert.ok(n > 30 && n < 80, 'pixels fixed ' + n);
    const c = (10 * w + 10) * 4;
    assert.ok(d[c] < 100 && d[c] <= d[c + 1] * 1.5, 'pupil is no longer red');
    assert.equal(d[0], 120);
    assert.equal(Pro.fixRedEye({ width: w, height: h, data: new Uint8ClampedArray(w * h * 4).fill(90) }, 5, 5, 8, 0.5), 0);
});

check('zoom steps', () => {
    assert.equal(Pro.nextZoom(1, 1), 1.5);
    assert.equal(Pro.nextZoom(1, -1), 0.667);
    assert.equal(Pro.nextZoom(20, 1), 20);
    assert.equal(Pro.nextZoom(0.05, -1), 0.05);
});

// --- registrations in the real module set ---

const runtimes = [];
Pixel.registerModule('tools-test-capture', r => { runtimes.push(r); });
const host = makeEl('div');
window.PixelApp.render(host, 'tools-1', { esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'), t: (k, f) => f || k, api: async () => ({}), iconMarkup: () => '', notify: () => {}, setWindowMenus: () => {} });
const rt = runtimes[0];
const pointer = props => Object.assign({ clientX: 0, clientY: 0, button: 0, buttons: 1, pointerId: 1, pointerType: 'mouse', pressure: 0.5, shiftKey: false, altKey: false, ctrlKey: false, metaKey: false, type: 'pointermove' }, props);
const press = (r, x, y, extra) => r.onCanvasMouseDown(pointer(Object.assign({ clientX: x, clientY: y, type: 'pointerdown' }, extra)));
const drag = (r, x, y, extra) => r.onCanvasMouseMove(pointer(Object.assign({ clientX: x, clientY: y }, extra)));
const release = (r, x, y, extra) => r.onCanvasMouseUp(pointer(Object.assign({ clientX: x, clientY: y, buttons: 0, type: 'pointerup' }, extra)));

check('every WP4 tool is registered with the right group, shortcut and handlers', () => {
    const expected = {
        brush: ['paint', 'B'], pencil: ['paint', 'P'], airbrush: ['paint', 'A'], eraser: ['paint', 'E'],
        'clone-stamp': ['retouch', 'S'], healing: ['retouch', 'Shift+J'], 'spot-heal': ['retouch', 'J'], smudge: ['retouch', 'Y'], 'blur-brush': ['retouch', 'N'], 'dodge-burn': ['retouch', 'D'], 'color-replace': ['retouch', 'Shift+B'], 'red-eye': ['retouch', 'Shift+E'],
        fill: ['fill', 'G'], gradient: ['fill', 'U'],
        line: ['shape', 'Shift+L'], arrow: ['shape', 'Shift+A'], rectangle: ['shape', 'R'], ellipse: ['shape', 'O'], 'rounded-rect': ['shape', 'Shift+R'], polygon: ['shape', 'Shift+O'], star: ['shape', 'Shift+S'],
        move: ['select', 'M'], crop: ['view', 'C'], hand: ['view', 'H'], zoom: ['view', 'Z']
    };
    Object.keys(expected).forEach(id => {
        const def = Pixel.getTool(id);
        assert.ok(def, id + ' registered');
        assert.equal(def.group, expected[id][0], id + ' group');
        assert.equal(def.shortcut || Pixel.TOOL_SHORTCUTS[id], expected[id][1], id + ' shortcut');
        assert.ok(Pixel.toolHasHandlers(def), id + ' owns its gesture');
        assert.ok(def.svg && def.svg.startsWith('<svg'), id + ' has an icon');
    });
    const shortcuts = Object.keys(expected).map(id => Pixel.normalizeShortcut(Pixel.getTool(id).shortcut));
    assert.equal(new Set(shortcuts).size, shortcuts.length, 'no two WP4 tools share a shortcut');
    ['brush', 'airbrush', 'eraser', 'pencil', 'smudge', 'blur-brush', 'dodge-burn', 'clone-stamp', 'healing', 'color-replace', 'spot-heal'].forEach(id => assert.equal(Pixel.getTool(id).cursor, 'none', id + ' hides the system cursor'));
    assert.equal(Pixel.getTool('move').raster, false);
    assert.equal(Pixel.getTool('brush').raster, true);
    // Free transform: Ctrl+Alt+T / Ctrl+T belong to the Edit menu item only, so no second command owns the key.
    const ft = Pixel.getTool('free-transform');
    assert.equal(ft.group, 'select');
    assert.equal(ft.shortcut, '');
    assert.ok(Pixel.toolHasHandlers(ft));
    const item = Pixel.listMenuItems('edit', rt).find(i => i.id === 'free-transform');
    assert.equal(item.shortcut, 'Ctrl+Alt+T');
    assert.deepEqual(item.shortcuts, ['Ctrl+T']);
});

check('every tool builds an option bar and wires it', () => {
    rt.newBlankCanvas(60, 40);
    Object.keys({ brush: 1, pencil: 1, airbrush: 1, eraser: 1, smudge: 1, 'blur-brush': 1, 'dodge-burn': 1, 'clone-stamp': 1, healing: 1, 'color-replace': 1, 'spot-heal': 1, 'red-eye': 1, fill: 1, gradient: 1, line: 1, arrow: 1, rectangle: 1, ellipse: 1, 'rounded-rect': 1, polygon: 1, star: 1, move: 1, crop: 1 }).forEach(id => {
        rt.activeTool = null;
        rt.setActiveTool(id);
        assert.equal(rt.activeTool, id);
        assert.ok(rt.optionsBar.innerHTML.includes('data-param-root') || id === 'x', id + ' options');
        rt.setActiveTool(id);
    });
    assert.deepEqual(errors, []);
});

check('brush stroke: dabs go to the stroke buffer, history gets one entry', () => {
    rt.newBlankCanvas(80, 60);
    rt.setActiveTool('brush');
    const history = rt.history.length;
    press(rt, 10, 10);
    assert.ok(rt.strokeState, 'stroke open');
    drag(rt, 30, 20);
    drag(rt, 50, 30);
    release(rt, 60, 30);
    assert.equal(rt.strokeState, null);
    assert.equal(rt.history.length, history + 1);
    assert.equal(rt.history[rt.historyIdx].label, 'draw:brush');
    const buffer = rt.brushBuffer.surface.getContext('2d');
    assert.ok(buffer.log.length > 5, 'many dabs were stamped');
    const layerLog = rt.layers[0].canvas.getContext('2d').log;
    assert.ok(layerLog.some(l => l[2] === 'source-over'), 'the buffer was composited with source-over');
    assert.deepEqual(errors, []);
});
check('eraser composites with destination-out, pencil snaps to pixels, Alt picks colour', () => {
    rt.setActiveTool('brush');
    rt.setActiveTool('eraser');
    press(rt, 5, 5); drag(rt, 25, 5); release(rt, 25, 5);
    assert.ok(rt.layers[0].canvas.getContext('2d').log.some(l => l[2] === 'destination-out'));
    rt.setActiveTool('eraser');
    rt.setActiveTool('pencil');
    const before = rt.history.length;
    press(rt, 5, 5); drag(rt, 40, 30); release(rt, 40, 30);
    assert.equal(rt.history.length, before + 1);
    const color = rt.primaryColor;
    press(rt, 3, 3, { altKey: true });
    assert.equal(rt.strokeState, null, 'Alt+click does not paint');
    release(rt, 3, 3);
    assert.equal(typeof rt.primaryColor, 'string');
    assert.ok(color);
    rt.setActiveTool('pencil');
    assert.deepEqual(errors, []);
});
check('selection clipping, alpha lock and mask editing choose the right target and operation', () => {
    rt.selection = { type: 'ellipse', x: 10, y: 10, w: 40, h: 30 };
    rt.setActiveTool('brush');
    press(rt, 20, 20); drag(rt, 30, 25); release(rt, 30, 25);
    assert.equal(rt.history[rt.historyIdx].label, 'draw:brush');
    rt.deselect();
    rt.activeLayer().lockAlpha = true;
    press(rt, 20, 20); drag(rt, 30, 25); release(rt, 30, 25);
    assert.ok(rt.layers[0].canvas.getContext('2d').log.some(l => l[2] === 'source-atop'), 'alpha lock paints with source-atop');
    rt.activeLayer().lockAlpha = false;
    rt.toggleLayerMask(rt.activeLayerIdx);
    assert.equal(rt.editMaskMode, true);
    const maskLog = rt.activeLayer().maskCanvas.getContext('2d').log;
    const n = maskLog.length;
    press(rt, 20, 20); drag(rt, 30, 25); release(rt, 30, 25);
    assert.ok(maskLog.length > n, 'the stroke went into the mask canvas');
    rt.toggleLayerMask(rt.activeLayerIdx);
    rt.setActiveTool('brush');
    assert.deepEqual(errors, []);
});
check('brush keys: [ ] change the size, digits the opacity, Escape cancels a stroke', () => {
    rt.setActiveTool('brush');
    Pixel.activeInstance = rt.state;
    const s = rt.brushSettingsFor('brush');
    const size = s.size;
    const key = (props) => rt.handleKeydown(Object.assign({ key: '', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null }, preventDefault() { this.defaultPrevented = true; } }, props));
    key({ key: ']', code: 'BracketRight' });
    assert.ok(s.size > size);
    key({ key: '[', code: 'BracketLeft' });
    assert.equal(s.size, size);
    key({ key: '{', code: 'BracketLeft', shiftKey: true });
    assert.equal(s.hardness, 90);
    key({ key: '3' });
    assert.equal(s.opacity, 30);
    assert.equal(rt.brushOpacity, 30, 'the runtime opacity follows');
    key({ key: '0' });
    assert.equal(s.opacity, 100);
    const history = rt.history.length;
    press(rt, 10, 10); drag(rt, 30, 30);
    key({ key: 'Escape' });
    assert.equal(rt.strokeState, null);
    assert.equal(rt.history.length, history, 'a cancelled stroke leaves no history');
    rt.setActiveTool('brush');
});
check('presets apply to the settings', () => {
    rt.setActiveTool('brush');
    const cfg = Brush.registry.brush;
    assert.ok(Brush.PRESETS.length >= 10);
    Brush.applyPreset(rt, cfg, 'calligraphy');
    const s = rt.brushSettingsFor('brush');
    assert.equal(s.roundness, 18);
    assert.equal(s.angle, 40);
    Brush.applyPreset(rt, cfg, 'star');
    assert.equal(s.shape, 'star');
    assert.equal(s.roundness, 100, 'a preset resets what it does not set');
    Brush.applyPreset(rt, cfg, 'round_hard');
    assert.equal(s.shape, 'round');
    rt.setActiveTool('brush');
});

check('shapes, gradient and fill commit through commitLayerPixels', () => {
    const commits = [];
    const original = rt.commitLayerPixels;
    rt.commitLayerPixels = function (layer, source, opts) { commits.push(opts && opts.label); return original.call(rt, layer, source, opts); };
    ['rectangle', 'rounded-rect', 'ellipse', 'polygon', 'star', 'line', 'arrow'].forEach(id => {
        rt.setActiveTool(id);
        press(rt, 10, 10); drag(rt, 40, 30); release(rt, 40, 30);
        rt.setActiveTool(id);
    });
    assert.deepEqual(commits, ['shape:rectangle', 'shape:rounded-rect', 'shape:ellipse', 'shape:polygon', 'shape:star', 'shape:line', 'shape:arrow']);
    commits.length = 0;
    rt.setActiveTool('gradient');
    press(rt, 0, 0); drag(rt, 40, 10); release(rt, 60, 40);
    rt.setActiveTool('gradient');
    rt.setActiveTool('fill');
    press(rt, 10, 10);
    rt.setActiveTool('fill');
    assert.deepEqual(commits, ['gradient', 'fill']);
    rt.commitLayerPixels = original;
    assert.deepEqual(errors, []);
});

check('retouch tools run a stroke without errors', () => {
    ['smudge', 'blur-brush', 'dodge-burn', 'color-replace'].forEach(id => {
        rt.setActiveTool(id);
        const history = rt.history.length;
        press(rt, 10, 10); drag(rt, 25, 20); drag(rt, 40, 30); release(rt, 45, 30);
        assert.equal(rt.strokeState, null, id);
        assert.ok(rt.history.length >= history, id);
        rt.setActiveTool(id);
    });
    // clone stamp needs a source first
    rt.setActiveTool('clone-stamp');
    press(rt, 10, 10);
    assert.equal(rt.strokeState, null, 'no source: no stroke');
    release(rt, 10, 10);
    press(rt, 8, 8, { altKey: true });
    release(rt, 8, 8, { altKey: true });
    assert.ok(rt.cloneSource);
    const history = rt.history.length;
    press(rt, 30, 30); drag(rt, 40, 35); release(rt, 45, 35);
    assert.equal(rt.history.length, history + 1);
    rt.setActiveTool('clone-stamp');
    rt.setActiveTool('healing');
    press(rt, 30, 30); drag(rt, 40, 35); release(rt, 45, 35);
    rt.setActiveTool('healing');
    rt.setActiveTool('red-eye');
    press(rt, 20, 20);
    rt.setActiveTool('red-eye');
    assert.deepEqual(errors, []);
});

check('spot heal marks the stroke and fills it afterwards (diffusion fallback)', async () => {
    rt.setActiveTool('spot-heal');
    press(rt, 20, 20); drag(rt, 30, 24); release(rt, 30, 24);
    assert.equal(rt.spotMark, null);
    rt.setActiveTool('spot-heal');
    assert.deepEqual(errors, []);
});

check('move: nudges merge into one history entry, Escape cancels a drag, locked layers refuse', () => {
    rt.newBlankCanvas(60, 40);
    rt.setActiveTool('move');
    Pixel.activeInstance = rt.state;
    const key = (props) => rt.handleKeydown(Object.assign({ key: '', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null }, preventDefault() { this.defaultPrevented = true; } }, props));
    const before = rt.history.length;
    key({ key: 'ArrowRight' });
    key({ key: 'ArrowRight' });
    key({ key: 'ArrowDown', shiftKey: true });
    assert.equal(rt.history.length, before + 1, 'merged');
    assert.equal(rt.history[rt.historyIdx].label, 'nudge');
    press(rt, 10, 10); drag(rt, 20, 20);
    assert.ok(rt.moveDrag);
    key({ key: 'Escape' });
    assert.equal(rt.moveDrag, null);
    release(rt, 20, 20);
    assert.equal(rt.history.length, before + 1);
    press(rt, 10, 10); drag(rt, 20, 20); release(rt, 20, 20);
    assert.equal(rt.history.length, before + 2);
    assert.equal(rt.history[rt.historyIdx].label, 'move');
    rt.activeLayer().lockPosition = true;
    press(rt, 10, 10);
    assert.equal(rt.moveDrag, null);
    release(rt, 10, 10);
    rt.activeLayer().lockPosition = false;
    rt.selection = { type: 'rect', x: 5, y: 5, w: 10, h: 10 };
    const snapOpt = rt.viewOpts.snap;
    rt.viewOpts.snap = false;
    press(rt, 8, 8); drag(rt, 18, 18);
    assert.equal(rt.selection.x, 15, 'a rect selection follows the pixels');
    release(rt, 18, 18);
    assert.equal(rt.history[rt.historyIdx].label, 'move');
    rt.viewOpts.snap = true;
    rt.selection = { type: 'rect', x: 5, y: 5, w: 10, h: 10 };
    rt.activeLayer().canvas.getContext('2d').getImageData = () => ({ width: 60, height: 40, data: new Uint8ClampedArray(60 * 40 * 4).fill(255) });
    press(rt, 8, 8); drag(rt, 18, 18);
    assert.equal(rt.selection.x, 20, 'the right edge (25) snaps to the canvas centre line (30)');
    assert.deepEqual(rt.moveDrag.guides.xs, [30]);
    release(rt, 18, 18);
    delete rt.activeLayer().canvas.getContext('2d').getImageData;
    rt.viewOpts.snap = snapOpt;
    rt.deselect();
    rt.setActiveTool('move');
    assert.deepEqual(errors, []);
});

check('free transform: session, numeric fields, apply and cancel', () => {
    rt.newBlankCanvas(60, 40);
    rt.startFreeTransform();
    assert.equal(rt.activeTool, 'free-transform');
    assert.ok(rt.ft, 'session open');
    const s = rt.toolSettings['free-transform'];
    assert.deepEqual([s.x, s.y, s.w, s.h], [30, 20, 60, 40]);
    const handle = rt.toolUiHandles['free-transform'];
    press(rt, 30, 20); drag(rt, 40, 30); release(rt, 40, 30);
    near(rt.ft.g.cx, 40);
    const history = rt.history.length;
    rt.handleKeydown({ key: 'Enter', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null }, preventDefault() {} });
    assert.equal(rt.ft, null, 'Enter applies');
    assert.equal(rt.history.length, history + 1);
    assert.equal(rt.history[rt.historyIdx].label, 'transform');
    assert.equal(rt.activeTool, null, 'back to the tool before');
    rt.startFreeTransform();
    rt.handleKeydown({ key: 'Escape', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null }, preventDefault() {} });
    assert.equal(rt.ft, null);
    assert.equal(rt.history.length, history + 1, 'Escape leaves no history');
    void handle;
    // unchanged sessions apply to nothing
    rt.startFreeTransform();
    rt.setActiveTool('free-transform');
    assert.equal(rt.history.length, history + 1);
    rt.selection = { type: 'rect', x: 5, y: 5, w: 20, h: 20 };
    rt.startFreeTransform('outline');
    assert.equal(rt.ft.mode, 'outline');
    rt.ft.g.sx = 2;
    const steps = rt.history.length;
    rt.setActiveTool('free-transform');
    assert.equal(rt.history.length, steps + 1);
    assert.equal(rt.history[rt.historyIdx].label, 'transform:selection');
    rt.deselect();
    assert.deepEqual(errors, []);
});

check('layer flips and rotation run as one-shot transforms', () => {
    const labels = [];
    const original = rt.commitLayerPixels;
    rt.commitLayerPixels = function (layer, source, opts) { labels.push(opts.label); return original.call(rt, layer, source, opts); };
    const edit = Pixel.listCommands ? rt.listCommands().filter(c => /^menu:edit:transform-layer\//.test(c.id)) : [];
    assert.equal(edit.length, 4);
    edit.forEach(c => c.run(rt));
    assert.deepEqual(labels, ['transform', 'transform', 'transform', 'transform']);
    rt.commitLayerPixels = original;
    assert.equal(rt.ft, null);
});

check('crop tool: session, ratio, apply through transformDocument', () => {
    rt.newBlankCanvas(100, 80);
    rt.startCrop();
    assert.equal(rt.activeTool, 'crop');
    assert.deepEqual(rt.crop, { x0: 0, y0: 0, x1: 100, y1: 80 });
    Pixel.toolUi.set(rt, Pixel.toolUi.configs.crop, 'ratio', '1:1');
    assert.deepEqual(rt.crop, { x0: 10, y0: 0, x1: 90, y1: 80 }, 'the full-image rectangle is fitted to the ratio');
    press(rt, 90, 40); drag(rt, 70, 40); release(rt, 70, 40);
    assert.equal(Math.round(rt.crop.x1 - rt.crop.x0), Math.round(rt.crop.y1 - rt.crop.y0), 'square crop');
    assert.equal(Math.round(rt.crop.x1 - rt.crop.x0), 60);
    const calls = [];
    const original = rt.transformDocument;
    rt.transformDocument = function (matrix, w, h) { calls.push([matrix.slice(), w, h]); return original.call(rt, matrix, w, h); };
    const history = rt.history.length;
    rt.handleKeydown({ key: 'Enter', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null }, preventDefault() {} });
    assert.equal(calls.length, 1);
    assert.deepEqual(calls[0][0].slice(0, 4), [1, 0, 0, 1]);
    assert.equal(calls[0][1], calls[0][2]);
    assert.equal(rt.history.length, history + 1);
    assert.equal(rt.history[rt.historyIdx].label, 'crop');
    assert.equal(rt.activeTool, null);
    rt.transformDocument = original;
    rt.startCrop();
    rt.cancelCrop();
    assert.equal(rt.activeTool, null);
    assert.equal(rt.crop, null);
    assert.deepEqual(errors, []);
});

check('hand, zoom and the View menu', () => {
    rt.newBlankCanvas(100, 80);
    rt.setActiveTool('zoom');
    const z = rt.zoom;
    press(rt, 50, 40); release(rt, 50, 40);
    assert.ok(rt.zoom > z, 'click zooms in');
    rt.setActiveTool('zoom');
    rt.setActiveTool('hand');
    press(rt, 50, 40); drag(rt, 60, 45); release(rt, 60, 45);
    assert.equal(rt.handDrag, null);
    rt.setActiveTool('hand');
    const view = rt.listCommands().filter(c => c.group === 'view').map(c => c.id);
    ['menu:view:zoom-fill', 'menu:view:pixel-grid', 'menu:view:grid', 'menu:view:snap', 'menu:view:grid-size/grid-size-16'].forEach(id => assert.ok(view.includes(id), id));
    assert.equal(rt.viewOpts.snap, true);
    rt.runCommand('menu:view:snap');
    assert.equal(rt.viewOpts.snap, false);
    rt.runCommand('menu:view:grid-size/grid-size-16');
    assert.equal(rt.viewOpts.grid, true);
    assert.equal(rt.viewOpts.gridSize, 16);
    assert.ok(rt.listOverlays().includes('grids'));
    rt.zoomFill();
    assert.deepEqual(errors, []);
});

check('Brush panel and module registrations', () => {
    const ids = Pixel.listPanels('inspector').map(p => p.id);
    assert.ok(ids.indexOf('brush') > ids.indexOf('colors') && ids.indexOf('brush') < ids.indexOf('transform'));
    assert.ok(Pixel.listModules().map(m => m.id).includes('brush'));
    assert.deepEqual(errors, [], 'no installer or handler logged an error');
});

console.error = originalError;
if (process.exitCode) console.log('test-pixel-tools: FAILED');
else console.log('test-pixel-tools: ' + count + ' checks passed');
