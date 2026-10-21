// WP5 filters (pixel-lib-fx.js + the catalog of pixel-filters.js): node test on tiny images. Loads every pixel lib into a vm.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { loadPixelLibs, appDir } from './pixel-test-util.mjs';

const { self: sandbox, AuraPixelLib: Lib } = loadPixelLibs();
if (!Lib || typeof Lib.handleMessage !== 'function' || !Lib.fx) throw new Error('AuraPixelLib.fx did not load');
const fx = Lib.fx;

let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed++;
        console.log('ok   ' + name);
    } catch (err) {
        console.log('FAIL ' + name + ': ' + (err && err.stack || err));
        process.exitCode = 1;
    }
}

// Deterministic test picture: colour gradients with some structure and a transparent corner.
function picture(w, h, opaque) {
    const img = Lib.createImage(w, h), d = img.data;
    for (let y = 0; y < h; y++) {
        for (let x = 0; x < w; x++) {
            const i = (y * w + x) * 4;
            d[i] = (x / Math.max(1, w - 1)) * 255;
            d[i + 1] = (y / Math.max(1, h - 1)) * 255;
            d[i + 2] = ((x * 7 + y * 13) % 32) * 8;
            d[i + 3] = !opaque && x < 3 && y < 3 ? 0 : 255;
        }
    }
    return img;
}
const same = (a, b) => a.width === b.width && a.height === b.height && a.data.every((v, i) => v === b.data[i]);
const mean = (img, c) => { let s = 0, n = 0; for (let i = c; i < img.data.length; i += 4) { s += img.data[i]; n++; } return s / n; };

// --- every filter runs ---

check('library exposes the catalog ids and the adjust filter', () => {
    const ids = fx.ids();
    assert.ok(ids.length >= 80, 'filters: ' + ids.length);
    assert.ok(ids.includes('adjust'));
    ['levels', 'curves', 'hue-saturation', 'color-balance', 'vibrance', 'channel-mixer', 'black-white', 'photo-filter', 'gradient-map', 'threshold', 'posterize', 'invert',
        'equalize', 'auto-levels', 'auto-contrast', 'auto-white-balance', 'blur', 'box-blur', 'motion-blur', 'radial-blur', 'zoom-blur', 'lens-blur', 'surface-blur', 'median',
        'unsharp-mask', 'high-pass', 'twirl', 'pinch', 'wave', 'ripple', 'polar', 'spherize', 'kaleidoscope', 'displace', 'glass', 'lens-distortion', 'oil-paint', 'halftone',
        'dither', 'pixel-art', 'toon', 'crystallize', 'pointillize', 'glowing-edges', 'comic', 'neon', 'add-noise', 'dust-scratches', 'light-leak', 'lens-flare', 'vhs', 'crt',
        'glitch', 'cross-process'].forEach(id => assert.ok(ids.includes(id), 'missing ' + id));
});

check('every filter id runs on a 16x16 image (opaque and with transparency) and keeps the size', () => {
    [picture(16, 16, true), picture(16, 16, false)].forEach(img => {
        fx.ids().forEach(id => {
            [1, 0.3].forEach(scale => {
                const out = fx.apply(img, id, {}, { scale });
                assert.equal(out.width, 16, id);
                assert.equal(out.height, 16, id);
                assert.equal(out.data.length, 16 * 16 * 4, id);
            });
        });
    });
});

check('filters do not change the input image', () => {
    const img = picture(16, 12, false), copy = Uint8ClampedArray.from(img.data);
    fx.ids().forEach(id => { fx.apply(img, id, {}, { scale: 1 }); });
    assert.ok(img.data.every((v, i) => v === copy[i]));
});

check('odd sizes and 1 pixel images do not throw', () => {
    [[1, 1], [3, 2], [17, 5], [1, 9]].forEach(([w, h]) => {
        const img = picture(w, h, true);
        fx.ids().forEach(id => { const out = fx.apply(img, id, {}, { scale: 1 }); assert.equal(out.data.length, w * h * 4, id + ' ' + w + 'x' + h); });
    });
});

check('seeded filters are deterministic and the seed matters', () => {
    const img = picture(24, 24, true);
    ['add-noise', 'grain', 'displace', 'glass', 'crystallize', 'pointillize', 'mosaic', 'vhs', 'glitch', 'vintage'].forEach(id => {
        const a = fx.apply(img, id, { seed: 7 }, { scale: 1 }), b = fx.apply(img, id, { seed: 7 }, { scale: 1 });
        assert.ok(same(a, b), id + ' is not deterministic');
    });
    assert.ok(!same(fx.apply(img, 'add-noise', { seed: 1 }, {}), fx.apply(img, 'add-noise', { seed: 2 }, {})));
    assert.ok(!same(fx.apply(img, 'glitch', { seed: 1, amount: 100 }, {}), fx.apply(img, 'glitch', { seed: 2, amount: 100 }, {})));
});

// --- numbers ---

check('invert twice is the identity and keeps alpha', () => {
    const img = picture(16, 16, false);
    const twice = fx.apply(fx.apply(img, 'invert', {}, {}), 'invert', {}, {});
    assert.ok(same(img, twice));
});

check('gaussian blur keeps the mean and smooths', () => {
    const img = picture(32, 32, true);
    const out = fx.apply(img, 'blur', { radius: 3 }, { scale: 1 });
    [0, 1, 2].forEach(c => assert.ok(Math.abs(mean(img, c) - mean(out, c)) < 2.5, 'channel ' + c));
    let before = 0, after = 0;
    for (let i = 2; i < img.data.length - 4; i += 4) { before += Math.abs(img.data[i + 4] - img.data[i]); after += Math.abs(out.data[i + 4] - out.data[i]); }
    assert.ok(after < before, 'blur reduces the local differences');
    const flat = Lib.createImage(8, 8);
    flat.data.fill(100);
    for (let i = 3; i < flat.data.length; i += 4) flat.data[i] = 255;
    assert.ok(same(flat, fx.apply(flat, 'blur', { radius: 5 }, {})), 'a flat image stays flat');
    assert.ok(same(flat, fx.apply(flat, 'median', { radius: 2 }, {})));
    assert.ok(same(flat, fx.apply(flat, 'surface-blur', { radius: 3, threshold: 30 }, {})));
});

check('blur radius is scaled for previews: the same radius at a smaller scale blurs less', () => {
    const img = picture(32, 32, true);
    const full = fx.apply(img, 'blur', { radius: 6 }, { scale: 1 }), small = fx.apply(img, 'blur', { radius: 6 }, { scale: 0.1 });
    const diff = o => { let s = 0; for (let i = 0; i < o.data.length; i += 4) s += Math.abs(o.data[i] - img.data[i]); return s; };
    assert.ok(diff(small) < diff(full));
});

check('levels maps the black and white points', () => {
    const img = Lib.createImage(3, 1);
    [50, 125, 200].forEach((v, k) => { img.data.set([v, v, v, 255], k * 4); });
    const out = fx.apply(img, 'levels', { in_black: 50, in_white: 200 }, {});
    assert.equal(out.data[0], 0);
    assert.equal(out.data[8], 255);
    assert.ok(Math.abs(out.data[4] - 128) <= 2);
    const lifted = fx.apply(img, 'levels', { out_black: 40, out_white: 220 }, {});
    assert.equal(lifted.data[0], 40 + Math.round((50 / 255) * 180));
    const red = fx.apply(img, 'levels', { channel: 'r', in_black: 50, in_white: 200 }, {});
    assert.equal(red.data[0], 0);
    assert.equal(red.data[1], 50, 'other channels untouched');
});

check('curves: identity keeps the image, points are hit, the spline stays monotone', () => {
    const img = picture(16, 16, true);
    assert.ok(same(img, fx.apply(img, 'curves', {}, {})));
    const lut = fx.splineLut([[0, 0], [64, 100], [128, 140], [255, 255]]);
    assert.equal(lut[0], 0);
    assert.equal(lut[64], 100);
    assert.equal(lut[255], 255);
    for (let i = 1; i < 256; i++) assert.ok(lut[i] >= lut[i - 1], 'monotone at ' + i);
    const inverted = fx.apply(img, 'curves', { curves: { rgb: [[0, 255], [255, 0]], r: [[0, 0], [255, 255]], g: [[0, 0], [255, 255]], b: [[0, 0], [255, 255]] } }, {});
    assert.ok(same(inverted, fx.apply(img, 'invert', {}, {})));
});

check('threshold, posterize and grayscale produce what they say', () => {
    const img = picture(16, 16, true);
    const t = fx.apply(img, 'threshold', { level: 100 }, {});
    for (let i = 0; i < t.data.length; i += 4) assert.ok(t.data[i] === 0 || t.data[i] === 255);
    const p = fx.apply(img, 'posterize', { levels: 3 }, {}), seen = new Set();
    for (let i = 0; i < p.data.length; i += 4) seen.add(p.data[i]);
    assert.ok(seen.size <= 3, 'levels: ' + [...seen]);
    const g = fx.apply(img, 'grayscale', {}, {});
    for (let i = 0; i < g.data.length; i += 4) assert.ok(g.data[i] === g.data[i + 1] && g.data[i] === g.data[i + 2]);
});

check('median removes salt noise', () => {
    const img = Lib.createImage(9, 9);
    img.data.fill(80);
    for (let i = 3; i < img.data.length; i += 4) img.data[i] = 255;
    const c = (4 * 9 + 4) * 4;
    img.data[c] = img.data[c + 1] = img.data[c + 2] = 255;
    const out = fx.apply(img, 'median', { radius: 1 }, { scale: 1 });
    assert.equal(out.data[c], 80);
});

check('dither: 1-bit gives only black and white, custom palettes honour the colour count', () => {
    const img = picture(24, 24, true);
    ['floyd-steinberg', 'atkinson', 'bayer2', 'bayer4', 'bayer8'].forEach(method => {
        const out = fx.apply(img, 'dither', { method, palette: '1bit' }, {});
        for (let i = 0; i < out.data.length; i += 4) assert.ok((out.data[i] === 0 || out.data[i] === 255) && out.data[i] === out.data[i + 1] && out.data[i] === out.data[i + 2], method);
    });
    const custom = fx.apply(img, 'dither', { method: 'floyd-steinberg', palette: 'custom', colors: 5 }, {}), colors = new Set();
    for (let i = 0; i < custom.data.length; i += 4) colors.add(custom.data[i] + ',' + custom.data[i + 1] + ',' + custom.data[i + 2]);
    assert.ok(colors.size <= 5 && colors.size >= 2, 'colours: ' + colors.size);
    const gb = fx.apply(img, 'pixel-art', { block: 4, palette: 'gameboy' }, {}), gbSet = new Set();
    for (let i = 0; i < gb.data.length; i += 4) gbSet.add(gb.data[i] + ',' + gb.data[i + 1] + ',' + gb.data[i + 2]);
    assert.ok(gbSet.size <= 4);
    assert.equal(fx.medianCut(img, 8).length <= 8, true);
});

check('pixelate and mosaic produce flat blocks', () => {
    const img = picture(16, 16, true), out = fx.apply(img, 'pixelate', { size: 4 }, { scale: 1 });
    for (let by = 0; by < 16; by += 4) for (let bx = 0; bx < 16; bx += 4) {
        const first = (by * 16 + bx) * 4;
        for (let y = by; y < by + 4; y++) for (let x = bx; x < bx + 4; x++) assert.equal(out.data[(y * 16 + x) * 4], out.data[first]);
    }
});

check('adjust: no sliders is the identity, exposure and contrast move the picture the right way', () => {
    const img = picture(16, 16, false);
    assert.ok(same(img, fx.apply(img, 'adjust', {}, {})));
    assert.equal(fx.adjustHasEffect({}), false);
    assert.equal(fx.adjustHasEffect({ contrast: 5 }), true);
    const up = fx.apply(img, 'adjust', { exposure: 40 }, {}), down = fx.apply(img, 'adjust', { exposure: -40 }, {});
    assert.ok(mean(up, 0) > mean(img, 0) && mean(down, 0) < mean(img, 0));
    const sat = fx.apply(img, 'adjust', { saturation: -100 }, {});
    for (let i = 0; i < sat.data.length; i += 4) assert.ok(Math.abs(sat.data[i] - sat.data[i + 1]) <= 1 && Math.abs(sat.data[i + 1] - sat.data[i + 2]) <= 1);
    const every = {};
    fx.adjustKeys.forEach(k => { every[k] = 30; });
    const out = fx.apply(picture(20, 14, true), 'adjust', every, {});
    assert.equal(out.width, 20);
});

check('auto tone stretches a flat dark picture and suggests sane values', () => {
    const img = Lib.createImage(16, 16);
    for (let i = 0; i < img.data.length; i += 4) { const v = 30 + ((i / 4) % 16) * 4; img.data.set([v, v * 0.9, v * 0.8, 255], i); }
    const tone = fx.autoTone(img);
    assert.ok(tone.exposure > 0 || tone.whites > 0, JSON.stringify(tone));
    Object.keys(tone).forEach(k => { assert.ok(fx.adjustKeys.includes(k), k); assert.ok(Math.abs(tone[k]) <= 100); });
    const out = fx.apply(img, 'adjust', tone, {});
    const range = o => { let lo = 255, hi = 0; for (let i = 0; i < o.data.length; i += 4) { const y = Lib.luma(o.data[i], o.data[i + 1], o.data[i + 2]); lo = Math.min(lo, y); hi = Math.max(hi, y); } return hi - lo; };
    assert.ok(range(out) > range(img), 'dynamic range grows');
});

check('histogram sums to the pixel count (transparent pixels are skipped)', () => {
    const img = picture(16, 16, true), h = fx.histogram(img);
    [h.r, h.g, h.b, h.l].forEach(bins => assert.equal(bins.reduce((a, b) => a + b, 0), 256));
    assert.equal(h.count, 256);
    const t = fx.histogram(picture(16, 16, false));
    assert.equal(t.l.reduce((a, b) => a + b, 0), 256 - 9);
});

check('compose: strength 0 keeps the base, 100 gives the filtered image, masks and blend modes behave', () => {
    const base = picture(8, 8, true), top = fx.apply(base, 'invert', {}, {});
    assert.ok(same(fx.compose(base, top, { mode: 'normal', strength: 0 }, null), base));
    assert.equal(fx.compose(base, top, null, null), top);
    assert.ok(same(fx.compose(base, top, { mode: 'normal', strength: 100 }, null), top));
    const half = fx.compose(base, top, { mode: 'normal', strength: 50 }, null);
    assert.ok(Math.abs(half.data[0] - (base.data[0] + top.data[0]) / 2) <= 1);
    const none = fx.compose(base, top, null, new Uint8ClampedArray(64));
    assert.ok(same(none, base));
    const all = fx.compose(base, top, null, new Uint8ClampedArray(64).fill(255));
    assert.ok(same(all, top));
    fx.fadeModes.forEach(mode => { const out = fx.compose(base, top, { mode, strength: 70 }, null); assert.equal(out.data.length, base.data.length, mode); });
    const mul = fx.compose(base, top, { mode: 'multiply', strength: 100 }, null);
    assert.ok(mul.data[0] <= base.data[0] + 1 && mul.data[0] <= top.data[0] + 1);
});

// --- the worker protocol ---

check('fx ops answer through the message protocol and transfer their buffers', () => {
    const img = picture(8, 8, true);
    const res = Lib.handleMessage({ id: 5, op: 'fx.render', args: { image: img, id: 'blur', params: { radius: 2 }, scale: 1, fade: { mode: 'normal', strength: 50 }, mask: null, keepRaw: true } });
    assert.equal(res.id, 5);
    assert.ok(res.result.image && res.result.raw);
    assert.ok(res.transfer.length >= 1);
    const bad = Lib.handleMessage({ id: 6, op: 'fx.apply', args: { image: img, id: 'no-such-filter' } });
    assert.match(bad.error, /unknown filter/);
    const hist = Lib.handleMessage({ id: 7, op: 'fx.histogram', args: { image: img, id: 'adjust', params: { contrast: 20 } } });
    assert.equal(hist.result.l.length, 256);
    assert.ok(Lib.handleMessage({ id: 8, op: 'fx.autoTone', args: { image: img } }).result.params);
    const composed = Lib.handleMessage({ id: 9, op: 'fx.compose', args: { base: img, raw: img, fade: { mode: 'screen', strength: 50 }, mask: null } });
    assert.equal(composed.result.image.width, 8);
});

// --- the catalog of pixel-filters.js against the library ---

check('catalog: every entry matches the library (ids, parameters, defaults, options)', () => {
    sandbox.window = sandbox;
    const registered = { panels: [], menus: [] };
    sandbox.AuraPixelApp = {
        bindRuntime: (r, fn) => fn,
        registerPanel: def => registered.panels.push(def),
        registerMenuItems: (id, items, opts) => registered.menus.push({ id, items, opts }),
        registerCommand() {}
    };
    vm.runInContext(fs.readFileSync(path.join(appDir, 'pixel-filters.js'), 'utf8'), sandbox, { filename: 'pixel-filters.js' });
    const Pixel = sandbox.AuraPixelApp, filters = Pixel.FILTERS;
    assert.ok(filters.length >= 80);
    const ids = new Set();
    filters.forEach(def => {
        assert.ok(!ids.has(def.id), 'duplicate ' + def.id);
        ids.add(def.id);
        assert.ok(fx.filters[def.id], 'the library has no filter ' + def.id);
        assert.ok(Pixel.FILTER_CATEGORIES.includes(def.cat), def.id + ' category');
        assert.ok(['fast', 'slow'].includes(def.preview), def.id + ' preview');
        assert.equal(def.op, 'fx.apply');
        assert.ok(def.labelKey.startsWith('pixel.filter_') && def.fallback, def.id + ' label');
        const defaults = fx.defaults(def.id), keys = new Set();
        def.params.forEach(p => {
            assert.ok(!keys.has(p.key), def.id + ' duplicate param ' + p.key);
            keys.add(p.key);
            assert.ok(p.key in defaults, def.id + ': library has no default for ' + p.key);
            assert.ok(p.labelKey && p.fallback, def.id + '.' + p.key + ' label');
            assert.deepEqual(p.default, defaults[p.key], def.id + '.' + p.key + ' default');
            if (p.type === 'range') {
                assert.ok(p.default >= p.min && p.default <= p.max, def.id + '.' + p.key + ' default ' + p.default + ' outside ' + p.min + '..' + p.max);
                assert.ok(p.max > p.min && p.step > 0);
            }
            if (p.type === 'select' || p.type === 'segmented') {
                assert.ok(p.options.length >= 2);
                assert.ok(p.options.some(o => o.value === p.default), def.id + '.' + p.key + ' default is not an option: ' + p.default);
                p.options.forEach(o => assert.ok(o.labelKey && o.fallback));
            }
            if (p.type === 'color') assert.match(p.default, /^#[0-9a-f]{6}$/i);
        });
        Object.keys(defaults).forEach(key => assert.ok(keys.has(key), def.id + ': library default ' + key + ' has no control'));
        // the schema defaults produce a result
        const values = {};
        def.params.forEach(p => { values[p.key] = JSON.parse(JSON.stringify(p.default)); });
        const out = fx.apply(picture(12, 12, true), def.id, Object.assign(values, def.thumb || {}), { scale: 0.3 });
        assert.equal(out.width, 12);
    });
    ['grayscale', 'monochrome', 'sepia', 'invert', 'warm', 'cool', 'tint', 'posterize', 'duotone', 'cyanotype', 'solarize', 'vintage', 'fade', 'high-contrast', 'vignette', 'bloom', 'glow',
        'blur', 'sharpen', 'edge', 'emboss', 'denoise', 'clarity', 'pixelate', 'mosaic', 'grain', 'sketch', 'aberration', 'tilt-shift'].forEach(id => assert.ok(ids.has(id), 'old filter ' + id + ' lost'));
    assert.ok(registered.panels.some(p => p.id === 'adjust') && registered.panels.some(p => p.id === 'filters'));
    assert.ok(registered.menus.some(m => m.id === 'filter'));
    const counts = {};
    filters.forEach(def => { counts[def.cat] = (counts[def.cat] || 0) + 1; });
    Pixel.FILTER_CATEGORIES.forEach(cat => assert.ok(counts[cat] >= 4, cat));
    console.log('     catalog: ' + filters.length + ' filters ' + JSON.stringify(counts));
});

if (!process.exitCode) console.log('test-pixel-fx: ' + passed + ' checks passed');
