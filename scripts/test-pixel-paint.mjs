// Checks for pixel-lib-paint.js and pixel-lib-wet.js (pigment mixing, papers, brush engines, wet layer).
import assert from 'node:assert/strict';
import { loadPixelLibs, readAppFile } from './pixel-test-util.mjs';

const { AuraPixelLib: Lib } = loadPixelLibs();
const paint = Lib.paint;
const results = [];
function check(name, fn) {
    try { fn(); results.push('ok   ' + name); } catch (err) { results.push('FAIL ' + name + ': ' + (err && err.stack || err)); }
}
const near = (a, b, eps, message) => assert.ok(Math.abs(a - b) <= eps, (message || 'values differ') + ': ' + a + ' vs ' + b);

function makeBuf(w, h, fill, docW, docH) {
    const data = new Uint8ClampedArray(w * h * 4);
    for (let i = 0; i < w * h; i++) { data[i * 4] = fill[0]; data[i * 4 + 1] = fill[1]; data[i * 4 + 2] = fill[2]; data[i * 4 + 3] = fill[3]; }
    return { x: 0, y: 0, w, h, data, sel: null, docW: docW || w, docH: docH || h, lockAlpha: false };
}
// Drags a brush along a polyline with the brush's own spacing; returns the dab centres.
function stroke(brush, buf, pts, size, pressure, flow) {
    const dabs = [];
    let first = true;
    for (let s = 0; s + 1 < pts.length; s++) {
        const [x0, y0] = pts[s], [x1, y1] = pts[s + 1], len = Math.hypot(x1 - x0, y1 - y0), ang = Math.atan2(y1 - y0, x1 - x0);
        const step = brush.spacing(size);
        for (let d = first ? 0 : step; d <= len; d += step) {
            const x = x0 + (x1 - x0) * d / len, y = y0 + (y1 - y0) * d / len;
            brush.dab(buf, x, y, size, pressure, ang, flow == null ? 1 : flow);
            dabs.push([x, y]);
        }
        first = false;
    }
    return dabs;
}
function segDist(px, py, x0, y0, x1, y1) {
    const dx = x1 - x0, dy = y1 - y0, t = Math.max(0, Math.min(1, ((px - x0) * dx + (py - y0) * dy) / (dx * dx + dy * dy)));
    return Math.hypot(px - (x0 + t * dx), py - (y0 + t * dy));
}

check('pigment mixing matches the reference values and round-trips exactly', () => {
    const refs = [
        [[0, 0, 255], [255, 255, 0], [39, 160, 108]], [[220, 30, 30], [252, 211, 0], [230, 100, 0]],
        [[220, 30, 30], [20, 50, 200], [89, 41, 110]], [[255, 0, 0], [0, 255, 0], [118, 106, 19]],
        [[200, 20, 20], [255, 255, 255], [215, 91, 72]], [[0, 0, 0], [255, 255, 255], [84, 84, 84]]
    ];
    refs.forEach(([a, b, want]) => {
        const got = paint.mixRgb(a, b, 0.5);
        for (let c = 0; c < 3; c++) near(got[c], want[c], 3, 'mix ' + a + ' + ' + b + ' channel ' + c);
    });
    const rand = Lib.mulberry32(7), col = () => [Math.floor(rand() * 256), Math.floor(rand() * 256), Math.floor(rand() * 256)];
    for (let i = 0; i < 2000; i++) {
        const c = col(), other = col();
        assert.deepEqual(Array.from(paint.mixRgb(c, other, 0)), c);
        assert.deepEqual(Array.from(paint.mixRgb(other, c, 1)), c);
    }
    assert.equal(paint.srgbToLinear.length, 256);
    assert.equal(paint.linearToSrgb(2), 255);
    assert.equal(paint.linearToSrgb(-1), 0);
});

check('glaze over white gives the ink, yellow over blue goes green', () => {
    const white = paint.pigmentOf(255, 255, 255), out = new Float32Array(8);
    [[200, 30, 30], [20, 50, 200], [250, 220, 10], [10, 10, 10], [120, 160, 90]].forEach(c => {
        const rgb = paint.pigmentToRgb(paint.glaze(out, white, paint.pigmentOf(c[0], c[1], c[2]), 1));
        for (let k = 0; k < 3; k++) near(rgb[k], c[k], 2, 'glaze ' + c);
    });
    const blue = paint.pigmentOf(0, 0, 255), yellow = paint.pigmentOf(255, 255, 0);
    const g = paint.pigmentToRgb(paint.glaze(out, blue, yellow, 1));
    assert.ok(g[1] > g[0] && g[1] > g[2], 'green dominant: ' + g);
});

check('papers: size, range, mean, tileable, deterministic', () => {
    assert.equal(paint.PAPERS.length, 8);
    paint.PAPERS.forEach(def => {
        const p = paint.makePaper(def.id, 3), h = p.height;
        assert.equal(p.size, 256);
        assert.equal(h.length, 65536);
        let min = 1, max = 0, sum = 0;
        for (let i = 0; i < h.length; i++) { min = Math.min(min, h[i]); max = Math.max(max, h[i]); sum += h[i]; }
        assert.ok(min >= 0 && max <= 1, def.id + ' range');
        near(sum / h.length, 0.5, 0.05, def.id + ' mean');
        const stepsX = [], stepsY = [];
        for (let y = 0; y < 256; y++) for (let x = 0; x < 255; x++) stepsX.push(Math.abs(h[y * 256 + x + 1] - h[y * 256 + x]));
        for (let y = 0; y < 255; y++) for (let x = 0; x < 256; x++) stepsY.push(Math.abs(h[(y + 1) * 256 + x] - h[y * 256 + x]));
        const p99 = a => a.slice().sort((u, v) => u - v)[Math.floor(a.length * 0.99)];
        let seamX = 0, seamY = 0;
        for (let i = 0; i < 256; i++) { seamX = Math.max(seamX, Math.abs(h[i * 256] - h[i * 256 + 255])); seamY = Math.max(seamY, Math.abs(h[i] - h[255 * 256 + i])); }
        assert.ok(seamX <= p99(stepsX) * 1.5 + 1e-9, def.id + ' seam x ' + seamX + ' vs ' + p99(stepsX));
        assert.ok(seamY <= p99(stepsY) * 1.5 + 1e-9, def.id + ' seam y ' + seamY + ' vs ' + p99(stepsY));
        assert.equal(paint.makePaper(def.id, 3), p, 'cached');
    });
    const a = paint.makePaper('rough', 11), b = paint.makePaper('rough', 12), c = paint.makePaper('rough', 11);
    assert.deepEqual(Array.from(a.height.slice(0, 500)), Array.from(c.height.slice(0, 500)));
    assert.notDeepEqual(Array.from(a.height.slice(0, 500)), Array.from(b.height.slice(0, 500)));
    const img = paint.renderPaper(paint.makePaper('canvas', 1), 40, 30, {});
    assert.equal(img.length, 40 * 30 * 4);
    assert.equal(img[3], 255);
});

check('bristle: reach, selection, lockAlpha, colour mixing', () => {
    const paper = paint.makePaper('cold_press', 1);
    const mk = (color, seed) => paint.createBrush('oil', { size: 30, color, paper, seed: seed || 5, docW: 200, docH: 120 });
    // reach
    const buf = makeBuf(200, 120, [255, 255, 255, 255]);
    const brush = mk([200, 30, 30]);
    brush.begin(false);
    const pts = [[50, 60], [150, 60]];
    stroke(brush, buf, pts, 30, 0.8, 1);
    const reach = brush.reach(30);
    let changed = 0;
    for (let y = 0; y < 120; y++) for (let x = 0; x < 200; x++) {
        const i = (y * 200 + x) * 4;
        if (buf.data[i] !== 255 || buf.data[i + 1] !== 255 || buf.data[i + 2] !== 255) {
            changed++;
            assert.ok(segDist(x + 0.5, y + 0.5, 50, 60, 150, 60) <= reach, 'pixel ' + x + ',' + y + ' beyond reach ' + reach);
        }
    }
    assert.ok(changed > 500, 'the stroke paints something: ' + changed);
    // selection: left half deselected
    const sb = makeBuf(200, 120, [255, 255, 255, 255]);
    sb.sel = new Uint8Array(200 * 120);
    for (let y = 0; y < 120; y++) for (let x = 100; x < 200; x++) sb.sel[y * 200 + x] = 255;
    const b2 = mk([200, 30, 30]);
    b2.begin(false);
    stroke(b2, sb, [[30, 60], [170, 60]], 30, 0.8, 1);
    let left = 0, right = 0;
    for (let y = 0; y < 120; y++) for (let x = 0; x < 200; x++) {
        const i = (y * 200 + x) * 4;
        if (sb.data[i + 1] !== 255) { if (x < 100) left++; else right++; }
    }
    assert.equal(left, 0, 'deselected area untouched');
    assert.ok(right > 300, 'selected area painted');
    // lockAlpha on a transparent buffer paints nothing
    const lb = makeBuf(200, 120, [0, 0, 0, 0]);
    lb.lockAlpha = true;
    const b3 = mk([200, 30, 30]);
    b3.begin(false);
    stroke(b3, lb, [[30, 60], [170, 60]], 30, 0.8, 1);
    assert.ok(lb.data.every(v => v === 0), 'lockAlpha keeps transparent pixels');
    // yellow dragged across blue/white leaves green in the transition
    const gb = makeBuf(200, 120, [255, 255, 255, 255]);
    for (let y = 0; y < 120; y++) for (let x = 0; x < 100; x++) { const i = (y * 200 + x) * 4; gb.data[i] = 0; gb.data[i + 1] = 0; gb.data[i + 2] = 255; }
    gb.wetMap = new Uint8Array(200 * 120); // the blue area is still wet, so the brush may pick it up
    for (let y = 0; y < 120; y++) for (let x = 0; x < 100; x++) gb.wetMap[y * 200 + x] = 255;
    const yb = mk([255, 255, 0]);
    yb.begin(false);
    stroke(yb, gb, [[20, 60], [180, 60]], 30, 0.8, 1);
    let green = 0;
    for (let i = 0; i < 200 * 120; i++) { const r = gb.data[i * 4], g = gb.data[i * 4 + 1], b = gb.data[i * 4 + 2]; if (g > r + 15 && g > b + 15) green++; }
    assert.ok(green >= 30, 'green pixels: ' + green);
});

check('bristle: dry canvas is not picked up, low load runs dry on rough paper', () => {
    const rough = paint.makePaper('rough', 1);
    const dry = makeBuf(300, 80, [0, 0, 255, 255]);
    dry.wetMap = new Uint8Array(300 * 80);
    const yb = paint.createBrush('oil', { size: 30, color: [255, 255, 0], paper: rough, seed: 5, docW: 300, docH: 80 });
    yb.begin(false);
    stroke(yb, dry, [[20, 40], [280, 40]], 30, 0.8, 1);
    let sumB = 0, n = 0;
    for (let y = 35; y < 46; y++) for (let x = 200; x < 280; x++) { sumB += dry.data[(y * 300 + x) * 4 + 2]; n++; }
    assert.ok(sumB / n < 90, 'dry blue stays unpicked, mean b ' + sumB / n);
    const long = makeBuf(460, 60, [255, 255, 255, 255]);
    long.wetMap = new Uint8Array(460 * 60);
    const ob = paint.createBrush('oil', { size: 24, color: [20, 100, 50], load: 0.25, grain: 0.9, paper: rough, seed: 6, docW: 460, docH: 60 });
    ob.begin(false);
    stroke(ob, long, [[30, 30], [430, 30]], 24, 0.8, 1);
    const cover = (x0, x1) => { let c = 0; for (let y = 20; y < 41; y++) for (let x = x0; x < x1; x++) if (long.data[(y * 460 + x) * 4] < 200) c++; return c; };
    const first = cover(30, 163), last = cover(297, 430);
    assert.ok(first > 300 && last / first < 0.7, 'coverage first third ' + first + ', last third ' + last);
});

check('bristle: the reservoir refills the tip, yellow recovers after a wet blue band', () => {
    const W = 400, H = 60, buf = makeBuf(W, H, [255, 255, 255, 255]);
    buf.wetMap = new Uint8Array(W * H);
    for (let y = 0; y < H; y++) for (let x = 100; x < 140; x++) { const i = y * W + x; buf.data[i * 4] = 0; buf.data[i * 4 + 1] = 0; buf.data[i * 4 + 2] = 255; buf.wetMap[i] = 255; }
    const brush = paint.createBrush('oil', { size: 24, color: [255, 255, 0], paper: paint.makePaper('cold_press', 1), seed: 8, docW: W, docH: H });
    brush.begin(false);
    stroke(brush, buf, [[20, 30], [340, 30]], 24, 0.8, 1);
    let green = 0;
    for (let y = 18; y < 43; y++) for (let x = 100; x < 190; x++) { const i = (y * W + x) * 4, r = buf.data[i], g = buf.data[i + 1], b = buf.data[i + 2]; if (g > r + 10 && g > b + 10) green++; }
    assert.ok(green >= 20, 'green where it crossed the wet band: ' + green);
    let r = 0, g = 0, b = 0, n = 0;
    for (let y = 26; y < 35; y++) for (let x = 250; x < 272; x++) { const i = (y * W + x) * 4; r += buf.data[i]; g += buf.data[i + 1]; b += buf.data[i + 2]; n++; }
    r /= n; g /= n; b /= n;
    assert.ok(r > 180 && g > 150 && b < 110, 'yellow again 120 px later: ' + [r, g, b].map(Math.round));
});

check('configure keeps the carried paint and changes the options', () => {
    const paper = paint.makePaper('smooth', 1);
    const brush = paint.createBrush('oil', { size: 30, color: [200, 30, 30], paper, seed: 2, docW: 100, docH: 100 });
    brush.begin(false);
    brush.configure({ load: 0 });
    brush.begin(false);
    const empty = makeBuf(100, 100, [0, 0, 0, 0]);
    assert.equal(brush.dab(empty, 50, 50, 30, 0.8, 0, 1), null, 'load 0 deposits nothing');
    brush.configure({ load: 0.8 });
    brush.begin(false);
    assert.ok(brush.dab(empty, 50, 50, 30, 0.8, 0, 1), 'load 0.8 deposits');
    brush.configure({ color: [0, 0, 255] });
    const carried = brush.carried();
    assert.ok(carried[0] > carried[2], 'colour change waits for the next reload: ' + carried);
    brush.begin(false);
    const after = brush.carried();
    assert.ok(after[2] > after[0], 'new colour after begin: ' + after);
});

check('dry: charcoal catches on paper peaks, pressure fills the valleys', () => {
    const paper = paint.makePaper('rough', 1);
    const ratio = pressure => {
        const buf = makeBuf(200, 120, [255, 255, 255, 255]);
        const brush = paint.createBrush('charcoal', { size: 26, color: [0, 0, 0], paper, seed: 4, docW: 200, docH: 120 });
        brush.begin(false);
        stroke(brush, buf, [[20, 60], [180, 60]], 26, pressure, 1);
        let hi = 0, nHi = 0, lo = 0, nLo = 0;
        for (let y = 54; y < 67; y++) for (let x = 40; x < 160; x++) {
            const h = paint.paperHeight(paper, x, y, 1), dark = 255 - buf.data[(y * 200 + x) * 4];
            if (h > 0.6) { hi += dark; nHi++; } else if (h < 0.4) { lo += dark; nLo++; }
        }
        return (hi / nHi) / Math.max(lo / nLo, 1e-6);
    };
    const low = ratio(0.35), high = ratio(1.0);
    assert.ok(low > 1.6, 'ratio at 0.35: ' + low);
    assert.ok(high < low, 'ratio at 1.0 ' + high + ' < ' + low);
});

check('marker: no self-darkening, second stroke darkens', () => {
    const paper = paint.makePaper('smooth', 1);
    const buf = makeBuf(200, 200, [255, 255, 255, 255]);
    const brush = paint.createBrush('marker', { size: 20, color: [200, 0, 0], paper, seed: 1, docW: 200, docH: 200 });
    brush.begin(false);
    stroke(brush, buf, [[30, 100], [170, 100]], 20, 1, 1);
    stroke(brush, buf, [[100, 30], [100, 170]], 20, 1, 1);
    const g = (x, y) => buf.data[(y * 200 + x) * 4 + 1];
    assert.ok(g(50, 100) < 200, 'marker paints: ' + g(50, 100));
    near(g(100, 100), g(50, 100), 6, 'crossing vs plain');
    const before = g(50, 100);
    brush.begin(false);
    stroke(brush, buf, [[30, 100], [170, 100]], 20, 1, 1);
    assert.ok(g(50, 100) < before - 6, 'second stroke darker: ' + g(50, 100) + ' vs ' + before);
});

check('wet: conservation, drying time, dark edge, render', () => {
    const paper = paint.makePaper('cold_press', 1);
    const wet = paint.createWet({ docW: 200, docH: 120, paper, paperScale: 1 });
    assert.equal(wet.s, 1);
    const brush = paint.createBrush('watercolor', { size: 30, color: [30, 60, 160], paper, seed: 1, docW: 200, docH: 120 });
    brush.begin(false);
    stroke(brush, wet, [[40, 60], [160, 60]], 30, 0.8, 1);
    assert.ok(wet.isWet() && wet.hasPaint());
    const total = () => { let t = 0; for (let i = 0; i < wet.m.length; i++) t += wet.m[i]; return t; };
    const t0 = total();
    for (let i = 0; i < 50; i++) wet.step();
    near(total() / t0, 1, 0.005, 'pigment conserved');
    let steps = 50;
    while (wet.step() && steps < 2000) steps++;
    assert.ok(steps >= 60 && steps <= 400, 'dries in ' + steps + ' steps');
    assert.ok(!wet.isWet());
    // dark edge: along columns through the stroke, the 2-cell rim beats the middle
    let rim = 0, mid = 0, nr = 0, nm = 0;
    for (let x = 70; x <= 130; x++) {
        let lo = -1, hi = -1;
        for (let y = 0; y < wet.gh; y++) if (wet.m[y * wet.gw + x] > 0.02) { if (lo < 0) lo = y; hi = y; }
        assert.ok(lo >= 0 && hi - lo > 10, 'stroke footprint at column ' + x);
        const span = hi - lo;
        for (let y = lo; y <= hi; y++) {
            const v = wet.m[y * wet.gw + x];
            if (y - lo < 2 || hi - y < 2) { rim += v; nr++; } else if (Math.abs(y - (lo + hi) / 2) < span * 0.2) { mid += v; nm++; }
        }
    }
    assert.ok(rim / nr > mid / nm, 'rim ' + rim / nr + ' > centre ' + mid / nm);
    // render over white and over transparent
    const white = makeBuf(200, 120, [255, 255, 255, 255]), out = makeBuf(200, 120, [0, 0, 0, 0]);
    wet.render(out, white);
    const c = (60 * 200 + 100) * 4;
    assert.ok(out.data[c] < 255 && out.data[c] >= 30 && out.data[c + 2] > out.data[c], 'between white and ink: ' + Array.from(out.data.slice(c, c + 4)));
    assert.equal(out.data[c + 3], 255);
    const clear = makeBuf(200, 120, [0, 0, 0, 0]), out2 = makeBuf(200, 120, [0, 0, 0, 0]);
    wet.render(out2, clear);
    assert.ok(out2.data[c + 3] > 0 && out2.data[c + 2] > out2.data[c], 'alpha and un-composited colour: ' + Array.from(out2.data.slice(c, c + 4)));
    assert.equal(wet.dirtyRect(), null, 'render clears the dirty rect');
    wet.bake();
    assert.ok(!wet.hasPaint() && wet.paintRect() === null);
});

check('determinism and previews', () => {
    const run = () => {
        const buf = makeBuf(160, 100, [255, 255, 255, 255]);
        const brush = paint.createBrush('oil', { size: 24, color: [30, 90, 200], paper: paint.makePaper('canvas', 2), seed: 9, docW: 160, docH: 100 });
        brush.begin(false);
        stroke(brush, buf, [[20, 50], [140, 50]], 24, 0.7, 1);
        return buf.data;
    };
    assert.deepEqual(Array.from(run()), Array.from(run()));
    paint.MEDIA.forEach(m => {
        const img = paint.previewStroke(m.id, 96, 48, { seed: 1 });
        assert.equal(img.length, 96 * 48 * 4);
        const plain = paint.renderPaper(paint.makePaper('cold_press', 1), 96, 48, {});
        let diff = 0;
        for (let i = 0; i < img.length; i++) if (img[i] !== plain[i]) diff++;
        assert.ok(diff > 40, m.id + ' preview draws a stroke: ' + diff);
    });
});

check('library files are small and DOM-free', () => {
    ['pixel-lib-paint.js', 'pixel-lib-wet.js'].forEach(name => {
        const text = readAppFile(name);
        assert.ok(text.split('\n').length < 1000, name + ' under 1000 lines');
        assert.ok(!/\b(document|window)\b/.test(text), name + ' has no document/window');
    });
});

console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-paint: ' + results.length + ' checks passed');
