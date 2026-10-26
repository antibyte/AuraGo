// Pure-library checks for pixel-lib-core.js (loaded into a vm that only has `self`, like the Pixel worker).
import assert from 'node:assert/strict';
import { loadPixelLibs, pixelLibFiles } from './pixel-test-util.mjs';

const { AuraPixelLib: Lib } = loadPixelLibs();
const results = [];
const pending = [];
function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') pending.push(out.then(() => results.push('ok   ' + name), fail));
        else results.push('ok   ' + name);
    } catch (err) { fail(err); }
}
const arr = value => Array.from(value);
const near = (a, b, eps, message) => assert.ok(Math.abs(a - b) <= eps, (message || 'values differ') + ': ' + a + ' vs ' + b);

check('every pixel-lib file loads and the core API is present', () => {
    assert.ok(pixelLibFiles().length >= 5, 'core + 4 stub libs');
    ['registerOp', 'handleMessage', 'clamp', 'clamp255', 'mulberry32', 'hashSeed', 'luma', 'gaussianKernel', 'blurAlpha', 'boxBlurRGBA',
        'rgbToHsl', 'hslToRgb', 'rgbToHsv', 'hsvToRgb', 'createImage', 'resizeBilinear', 'sampleBilinear', 'morphAlpha', 'featherAlpha',
        'alphaBounds', 'alphaBorder', 'gradientLUT'].forEach(name => assert.equal(typeof Lib[name], 'function', name));
    assert.ok(Lib.ops && typeof Lib.ops === 'object');
});

check('PRNG is deterministic per seed and stays in [0, 1)', () => {
    const a = Lib.mulberry32(42), b = Lib.mulberry32(42), c = Lib.mulberry32(43);
    const seqA = [], seqB = [], seqC = [];
    for (let i = 0; i < 20; i++) { seqA.push(a()); seqB.push(b()); seqC.push(c()); }
    assert.deepEqual(seqA, seqB);
    assert.notDeepEqual(seqA, seqC);
    seqA.forEach(v => assert.ok(v >= 0 && v < 1));
    assert.equal(Lib.hashSeed('pixel'), Lib.hashSeed('pixel'));
    assert.notEqual(Lib.hashSeed('pixel'), Lib.hashSeed('pixel2'));
    assert.ok(Lib.hashSeed('x') >= 0 && Lib.hashSeed('x') <= 0xFFFFFFFF);
});

check('clamp helpers and luma', () => {
    assert.equal(Lib.clamp(5, 0, 3), 3);
    assert.equal(Lib.clamp(-1, 0, 3), 0);
    assert.equal(Lib.clamp255(300), 255);
    assert.equal(Lib.clamp255(-4), 0);
    assert.equal(Lib.clamp255(12.4), 12);
    near(Lib.luma(255, 255, 255), 255, 1e-9);
    assert.equal(Lib.luma(0, 0, 0), 0);
});

check('handleMessage: success with transfer list, progress and cancel hooks', () => {
    Lib.registerOp('test.sum', (args, ctx) => {
        ctx.progress(0.5);
        ctx.progress(7);
        return { sum: args.a + args.b, buffer: new Uint8ClampedArray(4), cancelled: ctx.cancelled() };
    });
    const seen = [];
    const res = Lib.handleMessage({ id: 9, op: 'test.sum', args: { a: 2, b: 3 } }, { progress: f => seen.push(f), cancelled: () => true });
    assert.equal(res.id, 9);
    assert.equal(res.result.sum, 5);
    assert.equal(res.result.cancelled, true);
    assert.deepEqual(seen, [0.5, 1]);
    assert.equal(res.transfer.length, 1);
    assert.equal(res.transfer[0].byteLength, 4);
    assert.equal(res.error, undefined);
});

check('handleMessage: unknown op, throwing op and bad messages return { id, error }', () => {
    const unknown = Lib.handleMessage({ id: 1, op: 'nope.nothing', args: {} });
    assert.equal(unknown.id, 1);
    assert.match(unknown.error, /unknown op/);
    Lib.registerOp('test.boom', () => { throw new Error('kaput'); });
    const boom = Lib.handleMessage({ id: 2, op: 'test.boom' });
    assert.deepEqual({ id: boom.id, error: boom.error }, { id: 2, error: 'kaput' });
    const bad = Lib.handleMessage(null);
    assert.ok(bad.error);
    assert.throws(() => Lib.registerOp('', () => {}));
});

check('handleMessage: async ops resolve to the same shape', async () => {
    Lib.registerOp('test.async', async args => ({ v: args.v * 2 }));
    const res = await Lib.handleMessage({ id: 3, op: 'test.async', args: { v: 4 } });
    assert.equal(res.result.v, 8);
    assert.equal(res.id, 3);
});

check('built-in core ops respond', () => {
    const ping = Lib.handleMessage({ id: 1, op: 'core.ping' });
    assert.equal(ping.result.pong, true);
    const img = Lib.createImage(4, 4);
    img.data.fill(200);
    const resized = Lib.handleMessage({ id: 2, op: 'core.resize', args: { image: img, width: 2, height: 2 } });
    assert.equal(resized.result.image.width, 2);
});

check('gaussianKernel is normalised and symmetric', () => {
    const k = Lib.gaussianKernel(1.5);
    near(arr(k).reduce((s, v) => s + v, 0), 1, 1e-5);
    assert.equal(k.length % 2, 1);
    for (let i = 0; i < k.length; i++) near(k[i], k[k.length - 1 - i], 1e-9);
    assert.equal(Lib.gaussianKernel(0).length, 1);
});

check('blurAlpha on a tiny mask spreads a point and keeps its mass', () => {
    const w = 7, h = 7;
    const mask = new Uint8ClampedArray(w * h);
    mask[3 * w + 3] = 255;
    const out = Lib.blurAlpha(mask, w, h, 2);
    assert.equal(out.length, w * h);
    assert.ok(out[3 * w + 3] < 255 && out[3 * w + 3] > 0);
    assert.ok(out[3 * w + 4] > 0 && out[2 * w + 3] > 0);
    assert.equal(out[3 * w + 4], out[3 * w + 2], 'symmetric');
    assert.equal(out[2 * w + 3], out[4 * w + 3], 'symmetric');
    near(arr(out).reduce((s, v) => s + v, 0), 255, 12, 'mass is kept');
    assert.deepEqual(arr(Lib.blurAlpha(mask, w, h, 0)), arr(mask), 'radius 0 is a copy');
    assert.notEqual(Lib.blurAlpha(mask, w, h, 0), mask);
});

check('featherAlpha with a large radius (box path) is monotone across an edge and bounded', () => {
    const w = 80, h = 40;
    const mask = new Uint8ClampedArray(w * h);
    for (let y = 0; y < h; y++) for (let x = 40; x < w; x++) mask[y * w + x] = 255;
    const out = Lib.featherAlpha(mask, w, h, 16);
    const row = 20 * w;
    for (let x = 1; x < w; x++) assert.ok(out[row + x] >= out[row + x - 1], 'monotone at ' + x);
    assert.ok(out[row + 2] < 20 && out[row + 77] > 235);
    near(out[row + 40], 128, 14, 'half way at the edge');
});

check('boxBlurRGBA keeps colour of opaque pixels next to transparent ones', () => {
    const w = 8, h = 4;
    const data = new Uint8ClampedArray(w * h * 4);
    for (let y = 0; y < h; y++) for (let x = 0; x < 4; x++) { const o = (y * w + x) * 4; data[o] = 255; data[o + 3] = 255; }
    const out = Lib.boxBlurRGBA(data, w, h, 2);
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
        const o = (y * w + x) * 4;
        if (out[o + 3] > 0) assert.ok(out[o] >= 254 && out[o + 1] <= 1 && out[o + 2] <= 1, 'colour stays red at ' + x);
    }
    assert.ok(out[(0 * w + 3) * 4 + 3] < 255 && out[(0 * w + 4) * 4 + 3] > 0, 'alpha is blurred');
    const flat = new Uint8ClampedArray(w * h * 4).fill(90);
    assert.deepEqual(arr(Lib.boxBlurRGBA(flat, w, h, 3)), arr(flat));
});

check('resizeBilinear returns the requested size and keeps flat colour', () => {
    const img = Lib.createImage(4, 4);
    for (let i = 0; i < 16; i++) { img.data[i * 4] = 10; img.data[i * 4 + 1] = 120; img.data[i * 4 + 2] = 240; img.data[i * 4 + 3] = 255; }
    [[2, 2], [8, 6], [1, 1], [4, 4], [13, 7]].forEach(([w, h]) => {
        const out = Lib.resizeBilinear(img, w, h);
        assert.equal(out.width, w); assert.equal(out.height, h); assert.equal(out.data.length, w * h * 4);
        for (let i = 0; i < w * h; i++) assert.deepEqual(arr(out.data.slice(i * 4, i * 4 + 4)), [10, 120, 240, 255]);
    });
    const big = Lib.createImage(64, 64);
    big.data.fill(255);
    const small = Lib.resizeBilinear(big, 8, 8);
    assert.equal(small.width, 8);
    assert.equal(small.data[0], 255);
    assert.equal(Lib.resizeBilinear(Lib.createImage(0, 0), 3, 2).data.length, 24);
});

check('resizeBilinear and sampleBilinear weight colours by alpha', () => {
    const img = Lib.createImage(2, 1);
    img.data.set([255, 0, 0, 255, 0, 0, 0, 0]);
    const mid = Lib.sampleBilinear(img, 0.5, 0, []);
    assert.equal(mid[0], 255, 'transparent neighbour does not darken the red');
    near(mid[3], 127.5, 1e-6);
    const grown = Lib.resizeBilinear(img, 4, 1);
    for (let x = 0; x < 4; x++) if (grown.data[x * 4 + 3] > 0) assert.ok(grown.data[x * 4] >= 254);
    assert.deepEqual(arr(Lib.sampleBilinear(img, -5, -5, [])), [255, 0, 0, 255]);
});

check('colour space conversions', () => {
    assert.deepEqual(arr(Lib.rgbToHsl(255, 0, 0)), [0, 1, 0.5]);
    assert.deepEqual(arr(Lib.rgbToHsv(0, 255, 0)), [120, 1, 1]);
    [[12, 200, 77], [255, 255, 255], [0, 0, 0], [128, 128, 128], [250, 3, 90], [30, 30, 220]].forEach(([r, g, b]) => {
        const hsl = Lib.rgbToHsl(r, g, b);
        const back = Lib.hslToRgb(hsl[0], hsl[1], hsl[2]);
        [0, 1, 2].forEach(i => near(back[i], [r, g, b][i], 1e-6, 'hsl ' + [r, g, b]));
        const hsv = Lib.rgbToHsv(r, g, b);
        const back2 = Lib.hsvToRgb(hsv[0], hsv[1], hsv[2]);
        [0, 1, 2].forEach(i => near(back2[i], [r, g, b][i], 1e-6, 'hsv ' + [r, g, b]));
    });
    const out = [];
    assert.equal(Lib.hslToRgb(240, 1, 0.5, out), out);
    assert.deepEqual(arr(out).map(Math.round), [0, 0, 255]);
});

check('morphAlpha dilates and erodes with a square element and keeps border shapes', () => {
    const w = 9, h = 9;
    const dot = new Uint8ClampedArray(w * h);
    dot[4 * w + 4] = 255;
    const grown = Lib.morphAlpha(dot, w, h, 1, 'dilate');
    assert.equal(arr(grown).filter(v => v === 255).length, 9);
    assert.equal(arr(Lib.morphAlpha(dot, w, h, 2, 'dilate')).filter(v => v === 255).length, 25);
    const back = Lib.morphAlpha(grown, w, h, 1, 'erode');
    assert.equal(arr(back).filter(v => v === 255).length, 1);
    assert.equal(back[4 * w + 4], 255);
    const full = new Uint8ClampedArray(w * h).fill(255);
    assert.equal(arr(Lib.morphAlpha(full, w, h, 3, 'erode')).filter(v => v === 255).length, w * h, 'image edge does not erode');
    const corner = new Uint8ClampedArray(w * h);
    corner[0] = 255;
    assert.equal(arr(Lib.morphAlpha(corner, w, h, 1, 'dilate')).filter(v => v === 255).length, 4);
    assert.equal(arr(Lib.morphAlpha(dot, w, h, 0, 'dilate')).filter(v => v === 255).length, 1);
    const long = new Uint8ClampedArray(41 * 3);
    long[1 * 41 + 20] = 255;
    const wide = Lib.morphAlpha(long, 41, 3, 7, 'dilate');
    assert.equal(arr(wide).filter(v => v === 255).length, 3 * 15, 'radius larger than the height is fine');
});

check('morphAlpha matches a brute force reference on a random mask', () => {
    const w = 23, h = 17;
    const rand = Lib.mulberry32(7);
    const mask = new Uint8ClampedArray(w * h);
    for (let i = 0; i < mask.length; i++) mask[i] = Math.floor(rand() * 256);
    [1, 2, 3, 5].forEach(r => ['dilate', 'erode'].forEach(mode => {
        const fast = Lib.morphAlpha(mask, w, h, r, mode);
        for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
            let best = mode === 'dilate' ? 0 : 255;
            for (let dy = -r; dy <= r; dy++) for (let dx = -r; dx <= r; dx++) {
                const xx = x + dx, yy = y + dy;
                if (xx < 0 || yy < 0 || xx >= w || yy >= h) continue;
                const v = mask[yy * w + xx];
                best = mode === 'dilate' ? Math.max(best, v) : Math.min(best, v);
            }
            assert.equal(fast[y * w + x], best, mode + ' r' + r + ' at ' + x + ',' + y);
        }
    }));
});

check('alphaBounds and alphaBorder', () => {
    const w = 6, h = 5;
    const mask = new Uint8ClampedArray(w * h);
    assert.equal(Lib.alphaBounds(mask, w, h), null);
    for (let y = 1; y < 4; y++) for (let x = 2; x < 5; x++) mask[y * w + x] = 255;
    assert.deepEqual({ ...Lib.alphaBounds(mask, w, h) }, { x: 2, y: 1, w: 3, h: 3 });
    assert.equal(Lib.alphaBounds(mask, w, h, 255), null, 'threshold is exclusive');
    const border = arr(Lib.alphaBorder(mask, w, h));
    assert.equal(border.length, 8, 'ring of a 3x3 block');
    assert.ok(!border.includes(2 * w + 3), 'centre is not on the border');
});

check('gradient presets and lookup table', () => {
    const presets = Lib.GRADIENT_PRESETS;
    assert.ok(presets.length >= 16);
    ['grayscale', 'sunset', 'ocean', 'fire', 'ice', 'forest', 'rainbow', 'viridis', 'magma', 'inferno', 'plasma', 'copper', 'neon', 'pastel', 'earth', 'terrain', 'gold', 'sepia']
        .forEach(id => assert.ok(presets.some(p => p.id === id), 'preset ' + id));
    presets.forEach(p => {
        assert.ok(p.stops.length >= 2, p.id);
        assert.equal(p.stops[0].pos, 0);
        assert.equal(p.stops[p.stops.length - 1].pos, 1);
        p.stops.forEach((s, i) => { assert.match(s.color, /^#[0-9a-f]{6}$/i); if (i) assert.ok(s.pos >= p.stops[i - 1].pos, p.id + ' sorted'); });
    });
    const lut = Lib.gradientLUT(Lib.gradientPreset('grayscale').stops);
    assert.equal(lut.length, 256 * 4);
    assert.deepEqual(arr(lut.slice(0, 4)), [0, 0, 0, 255]);
    assert.deepEqual(arr(lut.slice(-4)), [255, 255, 255, 255]);
    near(lut[128 * 4], 128, 1.5);
    const small = Lib.gradientLUT([{ pos: 1, color: '#ff0000' }, { pos: 0, color: '#0000ff', alpha: 0.5 }], 3);
    assert.equal(small.length, 12);
    assert.deepEqual(arr(small.slice(0, 4)), [0, 0, 255, 128]);
    assert.deepEqual(arr(small.slice(8, 12)), [255, 0, 0, 255]);
    const cloned = Lib.cloneStops(presets[1].stops);
    cloned[0].color = '#123456';
    assert.notEqual(presets[1].stops[0].color, '#123456');
    assert.equal(Lib.gradientLUT([], 4).length, 16, 'empty stops fall back to grayscale');
});

check('parseColor and toHex', () => {
    assert.deepEqual(arr(Lib.parseColor('#f80')), [255, 136, 0, 255]);
    assert.deepEqual(arr(Lib.parseColor('336699')), [51, 102, 153, 255]);
    assert.equal(Lib.parseColor('nope'), null);
    assert.equal(Lib.toHex(255, 0, 128), '#ff0080');
});

await Promise.all(pending);
console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-lib: ' + results.length + ' checks passed');
