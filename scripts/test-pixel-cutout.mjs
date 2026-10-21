// WP8a cutout (pixel-lib-cutout.js): classic engine (colour key and GrabCut), colour key, refine, decontaminate and the
// background blur on small synthetic images. The libs are loaded into a vm that only has `self`, like the Pixel worker.
import assert from 'node:assert/strict';
import { loadPixelLibs } from './pixel-test-util.mjs';

// Only the core and this file: the test does not depend on the other packages' libs.
const { AuraPixelLib: Lib } = loadPixelLibs(['pixel-lib-core.js', 'pixel-lib-cutout.js']);
const C = Lib.cutout;
const results = [];
const pending = [];
// Checks may be synchronous or async; the printed order is the call order only for the awaited ones, so everything is
// collected before the summary.
function check(name, fn) {
    const run = (async () => {
        try {
            await fn();
            results.push('ok   ' + name);
        } catch (err) {
            results.push('FAIL ' + name + ': ' + (err && err.stack || err));
        }
    })();
    pending.push(run);
    return run;
}

// --- synthetic pictures ---

function noise(rand, amount) {
    return (rand() - 0.5) * 2 * amount;
}

// Subject disc on a background; truth is 1 inside the disc. bgAt / fgAt return [r, g, b] per pixel.
function scene(w, h, { bgAt, fgAt, radius, cx = 0.5, cy = 0.5, grain = 8, seed = 1 }) {
    const rand = Lib.mulberry32(seed);
    const data = new Uint8ClampedArray(w * h * 4);
    const truth = new Uint8Array(w * h);
    for (let y = 0; y < h; y++) {
        for (let x = 0; x < w; x++) {
            const inside = Math.hypot(x + 0.5 - w * cx, y + 0.5 - h * cy) < radius * Math.min(w, h);
            truth[y * w + x] = inside ? 1 : 0;
            const base = inside ? fgAt(x, y) : bgAt(x, y);
            const o = (y * w + x) * 4;
            data[o] = base[0] + noise(rand, grain);
            data[o + 1] = base[1] + noise(rand, grain);
            data[o + 2] = base[2] + noise(rand, grain);
            data[o + 3] = 255;
        }
    }
    return { image: { width: w, height: h, data }, truth, w, h };
}

function iou(alpha, truth, threshold = 127) {
    let both = 0, either = 0;
    for (let i = 0; i < truth.length; i++) {
        const a = alpha[i] > threshold ? 1 : 0;
        if (a && truth[i]) both++;
        if (a || truth[i]) either++;
    }
    return both / either;
}

const area = (alpha, threshold = 127) => alpha.reduce((sum, v) => sum + (v > threshold ? 1 : 0), 0);
const plainBlueRed = seed => scene(64, 64, { bgAt: () => [40, 90, 200], fgAt: () => [210, 50, 50], radius: 0.28, grain: 9, seed });
const textured = scene(64, 64, {
    bgAt: (x, y) => [60 + x * 1.2, 120 + 40 * Math.sin(y / 5), 90 + y * 0.8],
    fgAt: (x, y) => [190 + 25 * Math.sin(x / 3), 70 + 20 * Math.cos(y / 4), 60],
    radius: 0.3, grain: 10, seed: 2
});

check('every cutout op is registered and the library reports its helpers', () => {
    ['cutout.classic', 'cutout.chroma', 'cutout.refine', 'cutout.decontaminate', 'cutout.blur'].forEach(name => assert.equal(typeof Lib.ops[name], 'function', name));
    assert.ok(C && typeof C.minCutGrid === 'function');
});

check('classic engine: plain blue background with a red disc and noise (IoU > 0.9, colour key path)', () => {
    for (const seed of [1, 2, 3]) {
        const s = plainBlueRed(seed);
        const res = C.classic({ image: s.image });
        assert.equal(res.method, 'key', 'a plain background takes the soft colour key');
        assert.equal(res.alpha.length, 64 * 64);
        const score = iou(res.alpha, s.truth);
        assert.ok(score > 0.9, 'IoU ' + score.toFixed(3) + ' for seed ' + seed);
    }
});

check('classic engine: GrabCut separates a textured subject from a gradient background (IoU > 0.9)', () => {
    const res = C.classic({ image: textured.image });
    assert.equal(res.method, 'grabcut', 'a busy border cannot be keyed');
    const score = iou(res.alpha, textured.truth);
    assert.ok(score > 0.9, 'IoU ' + score.toFixed(3));
    // forcing GrabCut on the plain picture works as well
    const plain = plainBlueRed(4);
    const forced = C.classic({ image: plain.image, method: 'grabcut' });
    assert.equal(forced.method, 'grabcut');
    assert.ok(iou(forced.alpha, plain.truth) > 0.9, 'forced GrabCut IoU ' + iou(forced.alpha, plain.truth).toFixed(3));
});

check('classic engine: off-centre subject and a subject touching the border', () => {
    const off = scene(96, 64, { bgAt: (x, y) => [40 + x * 0.6, 100 + y * 0.4, 170], fgAt: () => [220, 80, 60], radius: 0.25, cx: 0.3, grain: 8, seed: 6 });
    assert.ok(iou(C.classic({ image: off.image }).alpha, off.truth) > 0.9, 'off-centre');
    const edge = scene(64, 64, { bgAt: (x, y) => [50 + x, 110 + 30 * Math.sin(y / 6), 150], fgAt: (x, y) => [210, 90 + 20 * Math.sin(x / 4), 70], radius: 0.3, cy: 0.78, grain: 8, seed: 7 });
    assert.ok(iou(C.classic({ image: edge.image }).alpha, edge.truth) > 0.85, 'touching the bottom edge');
});

check('classic engine is deterministic and reports progress', () => {
    const seen = [];
    const a = C.classic({ image: textured.image }, { progress: f => seen.push(f), cancelled: () => false });
    const b = C.classic({ image: textured.image });
    assert.deepEqual(Array.from(a.alpha), Array.from(b.alpha));
    assert.ok(seen.length >= 3 && seen[seen.length - 1] === 1, 'progress ends at 1: ' + seen.join(','));
    assert.ok(seen.every((f, i) => i === 0 || f >= seen[i - 1]), 'progress never goes back');
});

check('classic engine: scribbles are hard constraints, an init matte steers the result', () => {
    const s = plainBlueRed(5);
    const w = 64;
    const fg = new Uint8ClampedArray(w * w), bg = new Uint8ClampedArray(w * w);
    for (let x = 28; x < 36; x++) fg[32 * w + x] = 255;
    for (let x = 2; x < 10; x++) bg[2 * w + x] = 255;
    const res = C.classic({ image: s.image, scribbles: { fg, bg } });
    assert.equal(res.method, 'grabcut', 'hints always use the interactive model');
    assert.equal(res.alpha[32 * w + 30], 255);
    assert.equal(res.alpha[2 * w + 5], 0);
    assert.ok(iou(res.alpha, s.truth) > 0.9);
    // a bg scribble inside the disc cuts that spot out; a wrong init is repaired by the colours
    const bgInside = new Uint8ClampedArray(w * w);
    for (let y = 30; y < 34; y++) for (let x = 30; x < 34; x++) bgInside[y * w + x] = 255;
    const cut = C.classic({ image: s.image, scribbles: { fg: new Uint8ClampedArray(w * w), bg: bgInside } });
    assert.equal(cut.alpha[32 * w + 32], 0, 'removed spot stays removed');
    const init = new Uint8ClampedArray(w * w);
    for (let y = 0; y < w; y++) for (let x = 0; x < w / 2; x++) init[y * w + x] = 255;
    const fixed = C.classic({ image: s.image, init });
    assert.ok(iou(fixed.alpha, s.truth) > 0.8, 'init IoU ' + iou(fixed.alpha, s.truth).toFixed(3));
});

check('classic engine: transparent input pixels are background', () => {
    const s = plainBlueRed(8);
    for (let y = 0; y < 10; y++) for (let x = 0; x < 64; x++) s.image.data[(y * 64 + x) * 4 + 3] = 0;
    const res = C.classic({ image: s.image, method: 'grabcut' });
    for (let x = 0; x < 64; x++) assert.equal(res.alpha[5 * 64 + x], 0);
});

check('cleanMatte drops small islands and fills small holes unless hints say otherwise', () => {
    const w = 40, h = 40;
    const build = () => {
        const a = new Uint8ClampedArray(w * h);
        for (let y = 8; y < 32; y++) for (let x = 8; x < 32; x++) a[y * w + x] = 255;
        for (let y = 2; y < 4; y++) for (let x = 2; x < 4; x++) a[y * w + x] = 255;
        for (let y = 18; y < 20; y++) for (let x = 18; x < 20; x++) a[y * w + x] = 0;
        return a;
    };
    const plain = build();
    C.cleanMatte(plain, w, h, 127, null, null);
    assert.equal(plain[2 * w + 2], 0, 'island dropped');
    assert.equal(plain[18 * w + 18], 255, 'hole filled');
    assert.equal(plain[10 * w + 10], 255);
    const keep = new Uint8Array(w * h), drop = new Uint8Array(w * h);
    keep[2 * w + 2] = 1; drop[18 * w + 18] = 1;
    const hinted = build();
    C.cleanMatte(hinted, w, h, 127, keep, drop);
    assert.equal(hinted[2 * w + 2], 255, 'island with a keep hint stays');
    assert.equal(hinted[18 * w + 18], 0, 'hole with a remove hint stays open');
});

check('minCutGrid finds the cheapest cut', () => {
    // 6 x 3 grid: left two columns tied to the source, right two to the sink; the cheapest cut is the weak column between
    const w = 6, h = 3, n = w * h;
    const rcap = new Int32Array(n * 8), trcap = new Int32Array(n), valid = new Uint8Array(n);
    const set = (p, d, q, od, cap) => { rcap[p * 8 + d] = cap; rcap[q * 8 + od] = cap; };
    for (let y = 0; y < h; y++) {
        for (let x = 0; x < w; x++) {
            const p = y * w + x;
            if (x < w - 1) { valid[p] |= 1; valid[p + 1] |= 2; set(p, 0, p + 1, 1, x === 2 ? 1 : 50); }
            if (y < h - 1) { valid[p] |= 4; valid[p + w] |= 8; set(p, 2, p + w, 3, 50); }
            if (x < 2) trcap[p] = 100;
            if (x > 3) trcap[p] = -100;
        }
    }
    const label = C.minCutGrid(w, h, rcap, trcap, valid);
    for (let y = 0; y < h; y++) {
        for (let x = 0; x < w; x++) assert.equal(label[y * w + x], x <= 2 ? 1 : 0, 'pixel ' + x + ',' + y);
    }
});

check('chroma key removes a green background and despills the edge', () => {
    const w = 64;
    const rand = Lib.mulberry32(9);
    const data = new Uint8ClampedArray(w * w * 4);
    const truth = new Uint8Array(w * w);
    for (let y = 0; y < w; y++) {
        for (let x = 0; x < w; x++) {
            const inside = Math.hypot(x - 32, y - 32) < 18;
            truth[y * w + x] = inside ? 1 : 0;
            const shade = 0.75 + 0.25 * x / w;
            const base = inside ? [200, 60 + (Math.hypot(x - 32, y - 32) > 15 ? 90 : 0), 50] : [30 * shade, 200 * shade, 40 * shade];
            const o = (y * w + x) * 4;
            for (let c = 0; c < 3; c++) data[o + c] = base[c] + noise(rand, 8);
            data[o + 3] = 255;
        }
    }
    const image = { width: w, height: w, data };
    for (const color of ['auto', '#1ec828', [30, 200, 40]]) {
        const res = C.chroma({ image, color, tolerance: 40, softness: 20, despill: 80 });
        assert.equal(res.alpha.length, w * w);
        assert.equal(res.image.width, w);
        assert.ok(iou(res.alpha, truth) > 0.95, 'IoU ' + iou(res.alpha, truth).toFixed(3) + ' for ' + JSON.stringify(color));
        assert.equal(res.alpha[2 * w + 2], 0, 'background gone');
        assert.equal(res.alpha[32 * w + 32], 255, 'subject kept');
    }
    const strong = { width: 1, height: 1, data: Uint8ClampedArray.from([180, 200, 60, 255]) };
    const keyed = C.chroma({ image: strong, color: '#00ff00', despill: 100 });
    assert.ok(keyed.image.data[1] <= 120 + 1, 'green pulled to the mean of red and blue: ' + keyed.image.data[1]);
    const kept = C.chroma({ image: strong, color: '#00ff00', despill: 0 });
    assert.equal(kept.image.data[1], 200, 'no despill leaves the colour alone');
    // a neutral key colour has no spill channel
    const grey = C.chroma({ image: { width: 1, height: 1, data: Uint8ClampedArray.from([220, 220, 220, 255]) }, color: '#dcdcdc', despill: 100 });
    assert.deepEqual(Array.from(grey.image.data.slice(0, 3)), [220, 220, 220]);
});

check('refine: full-size matte from a small one, values in 0..255, edge follows the picture', () => {
    const full = scene(128, 128, { bgAt: () => [40, 90, 200], fgAt: () => [210, 60, 50], radius: 0.3, grain: 8, seed: 3 });
    const small = Lib.resizeBilinear(full.image, 32, 32);
    const proxy = C.classic({ image: small });
    const res = C.refine({ image: full.image, alpha: proxy.alpha, alphaWidth: 32, alphaHeight: 32 });
    assert.equal(res.alpha.length, 128 * 128);
    assert.ok(res.alpha instanceof Uint8ClampedArray || Object.prototype.toString.call(res.alpha) === '[object Uint8ClampedArray]');
    assert.ok(Math.min(...res.alpha) >= 0 && Math.max(...res.alpha) <= 255);
    const plain = C.resizeMask(proxy.alpha, 32, 32, 128, 128);
    const refined = iou(res.alpha, full.truth), upsampled = iou(plain, full.truth);
    assert.ok(refined > 0.95, 'refined IoU ' + refined.toFixed(4));
    assert.ok(refined >= upsampled - 0.002, 'the guided filter does not lose against a plain upsample: ' + refined.toFixed(4) + ' vs ' + upsampled.toFixed(4));
    // no image: plain resample to width x height
    const bare = C.refine({ alpha: proxy.alpha, alphaWidth: 32, alphaHeight: 32, width: 64, height: 48 });
    assert.equal(bare.alpha.length, 64 * 48);
    assert.throws(() => C.refine({ image: full.image, alpha: new Uint8ClampedArray(10) }), /size/);
});

check('refine: edge shift grows and shrinks by about its pixel count, smooth and feather change the edge', () => {
    const full = scene(128, 128, { bgAt: () => [40, 90, 200], fgAt: () => [210, 60, 50], radius: 0.3, grain: 6, seed: 4 });
    const matte = new Uint8ClampedArray(128 * 128);
    for (let i = 0; i < matte.length; i++) matte[i] = full.truth[i] * 255;
    const base = C.refine({ image: full.image, alpha: matte });
    const radiusOf = a => Math.sqrt(area(a) / Math.PI);
    const r0 = radiusOf(base.alpha);
    for (const shift of [-4, -2, 2, 4]) {
        const moved = C.refine({ image: full.image, alpha: matte, shift });
        const delta = radiusOf(moved.alpha) - r0;
        assert.ok(Math.abs(delta - shift) < 0.8, 'shift ' + shift + ' moved the radius by ' + delta.toFixed(2));
    }
    const ramp = a => a.reduce((sum, v) => sum + (v > 10 && v < 245 ? 1 : 0), 0);
    const feathered = C.refine({ image: full.image, alpha: matte, feather: 8 });
    assert.ok(ramp(feathered.alpha) > ramp(base.alpha) * 1.5, 'feather widens the ramp: ' + ramp(feathered.alpha) + ' vs ' + ramp(base.alpha));
    // a jagged edge gets calmer
    const ragged = new Uint8ClampedArray(matte);
    const rand = Lib.mulberry32(11);
    for (let i = 0; i < ragged.length; i++) if (ragged[i] === 255 && rand() < 0.03) ragged[i] = 0;
    const holes = a => { let n = 0; for (let i = 0; i < a.length; i++) if (full.truth[i] && a[i] < 128) n++; return n; };
    const smoothed = C.refine({ alpha: ragged, width: 128, height: 128, smooth: 6 });
    assert.ok(holes(smoothed.alpha) < holes(ragged) / 2, 'smoothing heals speckles: ' + holes(smoothed.alpha) + ' vs ' + holes(ragged));
    // the input is never changed in place
    assert.equal(matte[64 * 128 + 64], 255);
});

check('decontaminate replaces the background tint in the transition band with the subject colour', () => {
    const w = 120, bg = [40, 90, 200], fg = [210, 60, 50];
    const data = new Uint8ClampedArray(w * w * 4), alpha = new Uint8ClampedArray(w * w), cover = new Float32Array(w * w);
    for (let y = 0; y < w; y++) {
        for (let x = 0; x < w; x++) {
            let hit = 0;
            for (let sy = 0; sy < 4; sy++) for (let sx = 0; sx < 4; sx++) if (Math.hypot(x + (sx + 0.5) / 4 - w / 2, y + (sy + 0.5) / 4 - w / 2) < 35) hit++;
            const c = hit / 16;
            cover[y * w + x] = c;
            alpha[y * w + x] = Math.round(c * 255);
            for (let k = 0; k < 3; k++) data[(y * w + x) * 4 + k] = bg[k] * (1 - c) + fg[k] * c;
            data[(y * w + x) * 4 + 3] = 255;
        }
    }
    const res = C.decontaminate({ image: { width: w, height: w, data }, alpha });
    let before = 0, after = 0, count = 0;
    for (let i = 0; i < w * w; i++) {
        if (cover[i] > 0.15 && cover[i] < 0.9) {
            count++;
            for (let k = 0; k < 3; k++) { before += Math.abs(data[i * 4 + k] - fg[k]); after += Math.abs(res.image.data[i * 4 + k] - fg[k]); }
        }
    }
    assert.ok(count > 50, 'edge pixels exist');
    assert.ok(after / count < before / count / 5, 'colour error ' + (before / count / 3).toFixed(1) + ' -> ' + (after / count / 3).toFixed(1));
    const centre = (w / 2 * w + w / 2) * 4;
    assert.deepEqual(Array.from(res.image.data.slice(centre, centre + 4)), [210, 60, 50, 255], 'solid subject pixels are untouched');
    const edge = alpha.findIndex(v => v > 40 && v < 200);
    assert.equal(res.image.data[edge * 4 + 3], alpha[edge], 'the output carries the matte as alpha');
    assert.throws(() => C.decontaminate({ image: { width: 2, height: 2, data: new Uint8ClampedArray(16) }, alpha: new Uint8ClampedArray(3) }), /size/);
});

check('blur keeps the subject colours out of the blurred background', () => {
    const w = 96;
    const data = new Uint8ClampedArray(w * w * 4), alpha = new Uint8ClampedArray(w * w);
    for (let y = 0; y < w; y++) {
        for (let x = 0; x < w; x++) {
            const inside = Math.hypot(x - 48, y - 48) < 20;
            alpha[y * w + x] = inside ? 255 : 0;
            const c = inside ? [255, 0, 0] : (((x >> 3) + (y >> 3)) & 1 ? [20, 20, 200] : [240, 240, 240]);
            data.set([c[0], c[1], c[2], 255], (y * w + x) * 4);
        }
    }
    for (const radius of [4, 14, 30]) {
        const res = C.blur({ image: { width: w, height: w, data }, alpha, radius });
        assert.equal(res.image.width, w);
        let reddest = -999;
        for (let y = 0; y < w; y++) {
            for (let x = 0; x < w; x++) {
                const d = Math.hypot(x - 48, y - 48);
                if (d > 22 && d < 30) { const q = (y * w + x) * 4; reddest = Math.max(reddest, res.image.data[q] - Math.max(res.image.data[q + 1], res.image.data[q + 2])); }
            }
        }
        assert.ok(reddest < 40, 'no red halo next to the subject (radius ' + radius + '): ' + reddest);
        assert.equal(res.image.data[3], 255);
    }
    const soft = C.blur({ image: { width: w, height: w, data }, alpha, radius: 20 });
    const a = (6 * w + 7) * 4, b = (6 * w + 8) * 4;
    const sharpStep = Math.abs(data[a] - data[b]), blurredStep = Math.abs(soft.image.data[a] - soft.image.data[b]);
    assert.ok(blurredStep < sharpStep / 4, 'the background really is blurred: step ' + sharpStep + ' -> ' + blurredStep);
    const copy = C.blur({ image: { width: w, height: w, data }, alpha, radius: 0 });
    assert.deepEqual(Array.from(copy.image.data.slice(0, 16)), Array.from(data.slice(0, 16)));
});

check('ops run through handleMessage with progress and a transfer list', () => {
    const s = plainBlueRed(12);
    const seen = [];
    const res = Lib.handleMessage({ id: 5, op: 'cutout.classic', args: { image: s.image } }, { progress: f => seen.push(f), cancelled: () => false });
    assert.equal(res.error, undefined);
    assert.equal(res.id, 5);
    assert.ok(res.transfer.length >= 1 && res.transfer[0].byteLength === 64 * 64, 'the matte buffer is transferred');
    assert.ok(seen.length > 0);
    const refined = Lib.handleMessage({ id: 6, op: 'cutout.refine', args: { image: s.image, alpha: res.result.alpha } });
    assert.equal(refined.result.alpha.length, 64 * 64);
    const bad = Lib.handleMessage({ id: 7, op: 'cutout.refine', args: { alpha: new Uint8ClampedArray(3) } });
    assert.ok(bad.error, 'a bad request returns { id, error }');
    const cancelled = Lib.handleMessage({ id: 8, op: 'cutout.classic', args: { image: textured.image, method: 'grabcut' } }, { cancelled: () => true });
    assert.match(cancelled.error || '', /cancel/);
});


// ---------------------------------------------------------------------------------------------------------------
// The cutout module against the fake DOM of test-pixel-core (canvases there hold no pixels, so these checks follow the
// flows: busy state, history labels, layer order, engine choice, refine contexts; the maths is covered above).
// ---------------------------------------------------------------------------------------------------------------

async function moduleChecks() {
    const { installFakeDom, makeEl, loadPixelApp } = await import('./pixel-test-dom.mjs');
    installFakeDom();
    const Pixel = loadPixelApp();
    const runtimes = [];
    Pixel.registerModule('test-capture', r => { runtimes.push(r); });
    const notes = [];
    const calls = [];
    const scenario = { status: { available: false, runtime_ready: false, busy: false, models: [] }, cutout: null, failure: null };
    const api = async (url, options) => {
        calls.push({ url, method: options && options.method, body: options && options.body });
        if (url === '/api/pixel/cutout/status') return JSON.parse(JSON.stringify(scenario.status));
        if (url === '/api/pixel/cutout') {
            if (scenario.failure) throw scenario.failure;
            return scenario.cutout;
        }
        return {};
    };
    const host = makeEl('div');
    window.PixelApp.render(host, 'w-cutout', {
        esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
        t: (k, f) => f || k, api, iconMarkup: () => '', notify: n => notes.push(n), setWindowMenus: () => {}, readonly: false
    });
    const rt = runtimes[runtimes.length - 1];
    const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
    // undo moves the index, not the length: positions and labels are read from historyIdx
    const last = () => rt.history[rt.historyIdx].label;
    const cutoutCalls = () => calls.filter(c => c.url === '/api/pixel/cutout');

    check('module: panel, tool, menu items, commands and runtime methods are registered', () => {
        assert.ok(Pixel.listModules().some(m => m.id === 'cutout'));
        assert.equal(Pixel.getPanel('cutout').dock, 'inspector');
        assert.ok(Pixel.getPanel('cutout').order > Pixel.getPanel('transform').order && Pixel.getPanel('cutout').order < Pixel.getPanel('ai').order);
        const tool = Pixel.getTool('cutout-brush');
        assert.ok(tool && typeof tool.onDown === 'function' && typeof tool.onUp === 'function' && typeof tool.overlay === 'function');
        assert.equal(tool.raster, false, 'the brush paints the mask, so a pixel lock must not block it');
        ['computeSubjectMask', 'removeBackground', 'cutoutSelectSubject', 'cutoutToLayer', 'cutoutBlurBackground', 'cutoutApplyMask', 'cutoutSetRefine']
            .forEach(name => assert.equal(typeof rt[name], 'function', name));
        const ids = rt.listCommands().map(c => c.id);
        ['menu:ai:cutout-remove-bg', 'menu:ai:cutout-to-layer', 'menu:ai:cutout-blur-bg', 'panel:cutout', 'tool:cutout-brush'].forEach(id => assert.ok(ids.includes(id), id));
        assert.ok(host.innerHTML.includes('data-section="cutout"') && host.innerHTML.includes('data-cutout-action="remove"'));
        assert.ok(!host.innerHTML.includes('class="cc-'), 'no cookie-blocker prone class names');
    });

    rt.newBlankCanvas(40, 30);

    await check('module: no image, no job', async () => {
        const blank = renderBlank();
        await assert.rejects(blank.computeSubjectMask({}), /document/);
        assert.equal(await blank.removeBackground(), false);
    });

    await check('module: computeSubjectMask is document-sized and starts without any download', async () => {
        const res = await rt.computeSubjectMask({ engine: 'classic' });
        assert.deepEqual([res.width, res.height, res.alpha.length, res.engine], [40, 30, 1200, 'classic']);
        const auto = await rt.computeSubjectMask({});
        assert.equal(auto.engine, 'classic', 'no ready model: automatic means classic');
        assert.ok(calls.some(c => c.url === '/api/pixel/cutout/status'));
        assert.ok(!calls.some(c => c.url === '/api/pixel/cutout/prepare'), 'models are only downloaded on a click');
        assert.equal(cutoutCalls().length, 0);
        assert.equal(rt.cutoutState.busy, null, 'the busy state is released');
    });

    await check('module: remove background = one history step, a mask and a refine context', async () => {
        const before = rt.historyIdx;
        assert.equal(await rt.removeBackground(), true);
        const layer = rt.activeLayer();
        assert.ok(layer.maskCanvas && layer.maskEnabled);
        assert.equal(rt.historyIdx, before + 1);
        assert.equal(last(), 'cutout:remove-bg');
        assert.ok(rt.cutoutState.remembered.get(layer.id).get(layer.maskRev), 'context under the current mask revision');
        rt.undo();
        assert.equal(rt.activeLayer().maskCanvas, null, 'undo drops the mask');
        rt.redo();
        assert.ok(rt.activeLayer().maskCanvas);
        const again = rt.activeLayer();
        assert.ok(rt.cutoutState.remembered.get(again.id).get(again.maskRev), 'the context is found again after redo');
    });

    await check('module: a second job is refused while one runs, cancel aborts it', async () => {
        const running = rt.removeBackground();
        await wait(0);
        notes.length = 0;
        assert.equal(await rt.removeBackground(), false);
        assert.ok(notes.some(n => /already running/.test(n.message)));
        assert.ok(rt.cutoutState.busy);
        rt.cutoutState.busy.ac.abort();
        assert.equal(await running, false);
        assert.equal(rt.cutoutState.busy, null);
    });

    await check('module: select subject takes the mask of a cut-out layer, blur and cut-out add layers in the right order', async () => {
        assert.equal(await rt.cutoutSelectSubject(), true);
        const subject = rt.activeLayer();
        const count = rt.layers.length;
        assert.equal(await rt.cutoutBlurBackground(), true);
        assert.equal(rt.layers.length, count + 1);
        assert.equal(rt.activeLayer(), subject, 'the subject stays active');
        assert.ok(rt.layers.indexOf(subject) === 1 && rt.layers[0] !== subject, 'the blurred copy sits below');
        assert.equal(last(), 'cutout:blur-bg');
        assert.equal(await rt.cutoutToLayer(), true);
        assert.equal(rt.layers.length, count + 2);
        assert.equal(subject.visible, false, 'the source is hidden');
        assert.equal(rt.activeLayerIdx, rt.layers.indexOf(subject) + 1, 'the cut-out layer is active, above the source');
        assert.equal(last(), 'cutout:to-layer');
        rt.undo();
        rt.undo();
        assert.equal(rt.layers.length, count, 'undo takes both layers away');
        assert.equal(rt.activeLayer().id, subject.id);
        assert.equal(rt.activeLayer().visible, true);
    });

    await check('module: replace background adds a colour or generated layer below the subject', async () => {
        const subject = rt.activeLayer();
        const count = rt.layers.length;
        assert.equal(await rt.cutoutReplaceBackground({ mode: 'colour', color: '#336699' }), true);
        assert.equal(rt.layers.length, count + 1);
        assert.equal(rt.activeLayer(), subject, 'the subject stays active');
        assert.equal(last(), 'cutout:replace-bg');
        assert.equal(rt.layerIndexById(subject.id), 1);
        rt.undo();
        const generated = Pixel.GENERATORS.length ? await rt.cutoutReplaceBackground({ mode: 'generated', generator: 'clouds' }) : true;
        assert.equal(generated, true);
        if (Pixel.GENERATORS.length) {
            assert.equal(rt.layers.length, count + 1);
            assert.equal(last(), 'cutout:replace-bg');
            rt.undo();
        }
        assert.equal(rt.layers.length, count);
        const before = rt.historyIdx;
        assert.equal(await rt.cutoutReplaceBackground({ mode: 'transparent' }), true, 'transparent = the mask already does it');
        assert.equal(rt.historyIdx, before);
        rt.cutoutState.replaceValues.replaceMode = 'colour';
    });

    await check('module: refine values settle into one history step and follow undo', async () => {
        const layer = rt.activeLayer();
        const before = rt.historyIdx;
        assert.equal(rt.cutoutSetRefine('shift', 3), true);
        rt.cutoutSetRefine('feather', 4);
        assert.equal(rt.historyIdx, before, 'moving a slider alone does not fill the history');
        await wait(900);
        assert.equal(rt.historyIdx, before + 1);
        assert.equal(last(), 'cutout:refine');
        const cx = rt.cutoutState.remembered.get(layer.id).get(layer.maskRev);
        assert.equal(cx.params.shift, 3);
        assert.equal(cx.params.feather, 4);
        assert.equal(rt.cutoutSetRefine('nonsense', 1), false);
        rt.undo();
        await wait(60);
        const restored = rt.activeLayer();
        const back = rt.cutoutState.remembered.get(restored.id).get(restored.maskRev);
        assert.equal(back.params.shift, 0, 'undo finds the context of the earlier step');
        assert.equal(rt.cutoutState.refineValues.shift, 0, 'and the sliders follow');
    });

    await check('module: fringe settings change no mask, so they add no history step', async () => {
        const before = rt.historyIdx;
        rt.cutoutSetRefine('decontaminate', false);
        await wait(900);
        assert.equal(rt.historyIdx, before);
        rt.cutoutSetRefine('decontaminate', true);
        await wait(900);
    });

    await check('module: view modes register their overlay only while one is chosen', async () => {
        assert.ok(!rt.listOverlays().includes('cutout-view'));
        rt.cutoutSetRefine('view', 'black');
        assert.ok(rt.listOverlays().includes('cutout-view'));
        assert.ok(rt.listOverlays().indexOf('cutout-view') < rt.listOverlays().indexOf('selection'), 'below the selection outline');
        rt.cutoutSetRefine('view', 'normal');
        assert.ok(!rt.listOverlays().includes('cutout-view'));
    });

    await check('module: the brush records its strokes in the context and keeps them across a refine', async () => {
        const layer = rt.activeLayer();
        rt.setActiveTool('cutout-brush');
        const before = rt.historyIdx;
        const e = { button: 0, clientX: 10, clientY: 10, pointerId: 1, buttons: 1, altKey: false };
        rt.onCanvasMouseDown(e);
        rt.onCanvasMouseMove(Object.assign({}, e, { clientX: 20, clientY: 12 }));
        rt.onCanvasMouseUp(e);
        assert.equal(rt.historyIdx, before + 1);
        assert.equal(last(), 'cutout:brush');
        const cx = rt.cutoutState.remembered.get(layer.id).get(layer.maskRev);
        assert.equal(cx.strokes.length, 1);
        assert.equal(cx.strokes[0].mode, 'keep');
        assert.ok(cx.strokes[0].pts.length >= 2);
        // Alt flips the mode for that stroke
        rt.onCanvasMouseDown(Object.assign({}, e, { altKey: true }));
        rt.onCanvasMouseUp(e);
        const second = rt.cutoutState.remembered.get(layer.id).get(layer.maskRev);
        assert.equal(second.strokes.length, 2);
        assert.equal(second.strokes[1].mode, 'remove');
        rt.cutoutSetRefine('smooth', 2);
        await wait(900);
        const refined = rt.cutoutState.remembered.get(layer.id).get(layer.maskRev);
        assert.equal(refined.strokes.length, 2, 'strokes survive a recomputation');
        rt.setActiveTool('cutout-brush');
    });

    await check('module: the first brush stroke on a layer without a mask starts one', () => {
        rt.addLayer();
        const layer = rt.activeLayer();
        assert.equal(layer.maskCanvas, null);
        rt.setActiveTool('cutout-brush');
        const e = { button: 0, clientX: 5, clientY: 5, pointerId: 1, buttons: 1 };
        rt.onCanvasMouseDown(e);
        rt.onCanvasMouseUp(e);
        assert.ok(layer.maskCanvas);
        assert.equal(last(), 'cutout:brush');
        assert.equal(rt.cutoutState.remembered.get(layer.id), undefined, 'a hand-made mask has no refine context');
        rt.setActiveTool('cutout-brush');
        rt.undo();
        rt.undo();
    });

    await check('module: apply mask bakes it into the pixels through commitLayerPixels', async () => {
        const layer = rt.activeLayer();
        assert.ok(layer.maskCanvas);
        layer.lockPixels = true;
        assert.equal(await rt.cutoutApplyMask(), false, 'a pixel-locked layer refuses');
        assert.ok(layer.maskCanvas);
        layer.lockPixels = false;
        assert.equal(await rt.cutoutApplyMask(), true);
        assert.equal(layer.maskCanvas, null);
        assert.equal(last(), 'cutout:apply-mask');
        rt.undo();
        assert.ok(rt.activeLayer().maskCanvas, 'undo brings the mask back');
    });

    await check('module: a ready AI model is used by "auto", with the image as PNG and the model id', async () => {
        scenario.status = { available: true, runtime_ready: true, busy: false, models: [
            { id: 'standard', name: 'ISNet general-use', license: 'Apache-2.0', size_bytes: 178648008, ready: true, downloading: false, progress: 1, error: '' },
            { id: 'precise', name: 'BiRefNet general lite', license: 'MIT', size_bytes: 224005088, ready: false, downloading: false, progress: 0, error: '' }
        ] };
        scenario.cutout = { status: 'ok', mask: 'data:image/png;base64,AAAA', width: 40, height: 30, ms: 12 };
        rt.cutoutState.statusLoaded = false;
        calls.length = 0;
        const res = await rt.computeSubjectMask({});
        assert.equal(res.engine, 'standard');
        assert.equal(res.alpha.length, 1200);
        const sent = cutoutCalls();
        assert.equal(sent.length, 1);
        assert.equal(sent[0].method, 'POST');
        const body = JSON.parse(sent[0].body);
        assert.equal(body.model, 'standard');
        assert.match(body.source_data, /^data:image\/png;base64,/);
        assert.ok(!calls.some(c => c.url === '/api/pixel/cutout/prepare'));
    });

    await check('module: an AI failure falls back to the classic engine with a note', async () => {
        const failure = new Error('busy');
        failure.status = 429;
        failure.body = { status: 'busy' };
        scenario.failure = failure;
        notes.length = 0;
        const res = await rt.computeSubjectMask({});
        scenario.failure = null;
        assert.equal(res.engine, 'classic');
        assert.ok(notes.some(n => n.type === 'info' && /classic engine/.test(n.message) && /another cutout/.test(n.message)), JSON.stringify(notes));
    });

    await check('module: an explicit AI model that is not downloaded is not requested', async () => {
        notes.length = 0;
        calls.length = 0;
        rt.cutoutState.engineValues.engine = 'precise';
        const res = await rt.computeSubjectMask({ engine: 'precise' });
        rt.cutoutState.engineValues.engine = 'auto';
        assert.equal(res.engine, 'classic');
        assert.equal(cutoutCalls().length, 0);
        assert.ok(notes.some(n => /not downloaded/.test(n.message)));
        assert.ok(!calls.some(c => c.url === '/api/pixel/cutout/prepare'));
    });

    await check('module: the colour key engine runs cutout.chroma and keeps its own matte size', async () => {
        const res = await rt.computeSubjectMask({ engine: 'key' });
        assert.equal(res.engine, 'key');
        assert.equal(res.alpha.length, 1200);
    });

    await check('module: smart brush mode re-runs the classic engine with the strokes as hints', async () => {
        scenario.status = { available: false, models: [] };
        rt.cutoutState.statusLoaded = false;
        const layer = rt.activeLayer();
        rt.cutoutState.brushValues.smart = true;
        rt.setActiveTool('cutout-brush');
        const before = rt.historyIdx;
        const e = { button: 0, clientX: 8, clientY: 8, pointerId: 1, buttons: 1, altKey: false };
        rt.onCanvasMouseDown(e);
        rt.onCanvasMouseMove(Object.assign({}, e, { clientX: 14, clientY: 9 }));
        rt.onCanvasMouseUp(e);
        await wait(300);
        rt.cutoutState.brushValues.smart = false;
        rt.setActiveTool('cutout-brush');
        const labels = rt.history.slice(before + 1, rt.historyIdx + 1).map(entry => entry.label);
        assert.deepEqual(labels, ['cutout:brush', 'cutout:smart']);
        const cx = rt.cutoutState.remembered.get(layer.id).get(layer.maskRev);
        assert.equal(cx.engine, 'classic');
        assert.deepEqual(cx.strokes, [], 'the strokes went into the engine run');
    });

    check('module: disposing the window stops its timers', () => {
        window.PixelApp.dispose('w-cutout');
        assert.ok(rt.state.disposed);
    });

    function renderBlank() {
        const blankHost = makeEl('div');
        window.PixelApp.render(blankHost, 'w-blank', {
            esc: v => String(v), t: (k, f) => f || k, api, iconMarkup: () => '', notify: () => {}, setWindowMenus: () => {}, readonly: false
        });
        return runtimes[runtimes.length - 1];
    }
}

try {
    await moduleChecks();
} catch (err) {
    results.push('FAIL module checks could not run (does every Pixel script load?): ' + (err && err.stack || err));
}

await Promise.all(pending);
console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-cutout: ' + results.length + ' checks passed');
