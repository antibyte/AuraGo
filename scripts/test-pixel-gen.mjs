// WP6 generators: pure library checks (every generator and preset renders, determinism, tileable noise wraps, Mandelbrot
// centre, resolution independence, schema sanity) and the Generate module against the fake DOM (panel, menu, commands,
// new layer / live preview with history safety).
import assert from 'node:assert/strict';
import { loadPixelLibs } from './pixel-test-util.mjs';
import { installFakeDom, makeEl, loadPixelApp } from './pixel-test-dom.mjs';

const results = [];
// Async checks return their promise; the app checks below are awaited one after the other because they share a window.
function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') return out.then(() => results.push('ok   ' + name), fail);
        results.push('ok   ' + name);
    } catch (err) { fail(err); }
    return undefined;
}

// The vm context of loadPixelLibs is what the worker looks like (no DOM, only `self`); it is slow, so it only gets a smoke test.
const vmLib = loadPixelLibs().AuraPixelLib;
check('library loads in a bare vm context and renders through the op protocol', () => {
    ['perlin', 'checkerboard', 'mandelbrot'].forEach(id => {
        const res = vmLib.handleMessage({ id: 1, op: 'gen.render', args: { id, params: {}, width: 16, height: 12 } });
        assert.equal(res.error, undefined, id + ': ' + res.error);
        assert.equal(res.result.image.width, 16);
        assert.equal(res.result.image.data.length, 16 * 12 * 4);
        assert.equal(res.transfer.length, 1, 'one buffer is transferred (image and data share it)');
    });
});

// Everything else runs in the main context at full speed.
installFakeDom();
const Pixel = loadPixelApp();
const Lib = globalThis.AuraPixelLib;
const hooks = { progress() {}, cancelled: () => false };
const render = (id, params, w, h, extra) => Lib.ops['gen.render'](Object.assign({ id, params: params || {}, width: w || 32, height: h || w || 32 }, extra || {}), hooks);
const bytes = img => Buffer.from(img.data.buffer, img.data.byteOffset, img.data.length);
// Generators with simulations or many strokes get lighter settings so the whole run stays fast.
const LIGHT = { 'reaction-diffusion': { detail: 80, iterations: 200 }, 'flow-field': { lines: 150, steps: 30 }, moire: { kind: 'waves' }, maze: { cells: 6 } };
const light = (id, params) => Object.assign({}, params || {}, LIGHT[id] || {});

check('the catalog covers the planned generators in four categories', () => {
    const ids = Lib.GENERATORS.map(g => g.id);
    assert.equal(new Set(ids).size, ids.length, 'unique ids');
    assert.deepEqual(Lib.GENERATOR_CATEGORIES, ['noise', 'texture', 'pattern', 'math']);
    const byCat = cat => Lib.GENERATORS.filter(g => g.cat === cat).map(g => g.id);
    ['white-noise', 'value-noise', 'perlin', 'simplex', 'worley', 'fbm', 'ridged', 'turbulence', 'warped'].forEach(id => assert.ok(byCat('noise').includes(id), id));
    ['clouds', 'marble', 'wood', 'plasma', 'camouflage', 'lava', 'caustics', 'nebula', 'paper', 'brushed-metal'].forEach(id => assert.ok(byCat('texture').includes(id), id));
    ['checkerboard', 'stripes', 'grid', 'polka-dots', 'hexagons', 'truchet', 'stained-glass', 'halftone', 'bricks', 'waves'].forEach(id => assert.ok(byCat('pattern').includes(id), id));
    ['gradient', 'mandelbrot', 'julia', 'newton', 'burning-ship', 'spirograph', 'rose', 'lissajous', 'superformula', 'phyllotaxis', 'moire', 'flow-field', 'reaction-diffusion', 'maze']
        .forEach(id => assert.ok(byCat('math').includes(id), id));
    Lib.GENERATORS.forEach(g => assert.ok(g.presets.length >= 1 && g.presets.length <= 5, g.id + ' has 1-5 presets'));
    assert.equal(Pixel.GENERATORS, Lib.GENERATORS, 'Pixel.GENERATORS is the library catalog');
});

check('schemas are valid and every preset only sets known keys with legal values', () => {
    Lib.GENERATORS.forEach(g => {
        const keys = g.params.map(p => p.key);
        assert.equal(new Set(keys).size, keys.length, g.id + ': unique keys');
        g.params.forEach(p => {
            assert.ok(p.labelKey.startsWith('pixel.gp_') && p.fallback, g.id + '.' + p.key + ' label');
            if (p.type === 'range' || p.type === 'int') {
                assert.ok(p.default >= p.min && p.default <= p.max, g.id + '.' + p.key + ' default in range');
                assert.ok(p.max > p.min && p.step > 0, g.id + '.' + p.key + ' range');
            }
            if (p.type === 'select' || p.type === 'segmented') assert.ok(p.options.some(o => o.value === p.default), g.id + '.' + p.key + ' default option');
        });
        g.presets.forEach(preset => {
            Object.keys(preset.values).forEach(key => {
                const p = g.params.find(item => item.key === key);
                assert.ok(p, g.id + '/' + preset.id + ': unknown key ' + key);
                const v = preset.values[key];
                if (p.type === 'range' || p.type === 'int') assert.ok(v >= p.min && v <= p.max, g.id + '/' + preset.id + '.' + key + ' in range (' + v + ')');
                if (p.type === 'select' || p.type === 'segmented') assert.ok(p.options.some(o => o.value === v), g.id + '/' + preset.id + '.' + key + ' option');
                if (p.type === 'gradient') assert.ok(typeof v === 'string' ? !!Lib.gradientPreset(v) : Array.isArray(v) && v.length >= 2, g.id + '/' + preset.id + ' gradient');
            });
        });
    });
});

check('every generator and every preset renders a valid, non-trivial image', () => {
    Lib.GENERATORS.forEach(g => {
        const variants = [['default', {}]].concat(g.presets.map(p => [p.id, p.values]));
        variants.forEach(([name, values]) => {
            const res = render(g.id, light(g.id, values), 37, 29);
            assert.equal(res.image.width, 37, g.id + '/' + name);
            assert.equal(res.image.height, 29);
            assert.equal(res.image.data.length, 37 * 29 * 4);
            assert.equal(res.data, res.image.data, 'same typed array under both names');
            const d = res.image.data;
            let distinct = new Set();
            for (let i = 0; i < d.length; i += 4) distinct.add((d[i] << 16) | (d[i + 1] << 8) | d[i + 2]);
            assert.ok(distinct.size > 1 || g.id === 'gradient', g.id + '/' + name + ' is not a flat image');
            for (let i = 3; i < d.length; i += 4) assert.equal(d[i], 255, g.id + '/' + name + ' is opaque');
        });
    });
});

check('same seed gives the same bytes, another seed another image', () => {
    Lib.GENERATORS.filter(g => g.params.some(p => p.key === 'seed')).forEach(g => {
        const a = render(g.id, light(g.id, { seed: 7 }), 40, 32).image, b = render(g.id, light(g.id, { seed: 7 }), 40, 32).image;
        const c = render(g.id, light(g.id, { seed: 8 }), 40, 32).image;
        assert.ok(bytes(a).equals(bytes(b)), g.id + ' is deterministic');
        assert.ok(!bytes(a).equals(bytes(c)), g.id + ' reacts to the seed');
    });
});

check('tileable fields are exactly periodic (every base noise and fractal kind)', () => {
    const kit = Lib.genKit, fr = { w: 72, h: 60, ax: 1.2, ay: 1, du: 1.2 / 72, dv: 1 / 60 };
    ['value', 'perlin', 'simplex'].forEach(type => ['fbm', 'ridged', 'turbulence', 'billow'].forEach(kind => {
        const f = kit.makeFbm({ type, seed: 5, cells: 3.3, octaves: 5, lacunarity: 2, gain: 0.5, kind, tile: true, offsetX: 0.17, offsetY: -0.31 }, fr);
        for (let i = 0; i < 120; i++) {
            const u = i * 0.0071, v = i * 0.0043;
            assert.ok(Math.abs(f.t(u, v) - f.t(u + fr.ax, v)) < 1e-9, type + '/' + kind + ' repeats horizontally');
            assert.ok(Math.abs(f.t(u, v) - f.t(u, v + fr.ay)) < 1e-9, type + '/' + kind + ' repeats vertically');
        }
    }));
    const lay = kit.noiseLayout('perlin', 7.3, fr, true), w = kit.worleyNoise(3, 1, 0);
    for (let i = 0; i < 100; i++) {
        const x = i * 0.37, y = i * 0.21;
        const a = w(x, y, lay.nx, lay.ny), f1 = a.f1, id = a.id;
        const b = w(x + lay.nx, y + lay.ny, lay.nx, lay.ny);
        assert.ok(Math.abs(b.f1 - f1) < 1e-9 && b.id === id, 'Worley repeats with the lattice period');
    }
});

check('tileable generators wrap: the seam is no rougher than the picture itself (several seeds)', () => {
    const ids = Lib.GENERATORS.filter(g => g.params.some(p => p.key === 'tileable')).map(g => g.id);
    assert.ok(ids.length >= 12, 'tileable switch on the noise family and several textures (' + ids.length + ')');
    const stat = (id, tile) => {
        const W = 72, H = 60;
        let seamX = 0, innerX = 0, seamY = 0, innerY = 0;
        for (let seed = 1; seed <= 6; seed++) {
            const d = render(id, { tileable: tile, seed, scale: 24 }, W, H).image.data;
            const lum = (x, y) => { const o = (y * W + x) * 4; return d[o] * 0.299 + d[o + 1] * 0.587 + d[o + 2] * 0.114; };
            for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
                const dx = Math.abs(lum(x, y) - lum((x + 1) % W, y)), dy = Math.abs(lum(x, y) - lum(x, (y + 1) % H));
                if (x === W - 1) seamX += dx; else innerX += dx;
                if (y === H - 1) seamY += dy; else innerY += dy;
            }
        }
        return { x: (seamX / 6 / H) / (innerX / 6 / (H * (W - 1))), y: (seamY / 6 / W) / (innerY / 6 / (W * (H - 1))) };
    };
    ids.forEach(id => {
        const s = stat(id, true);
        assert.ok(s.x < 1.9 && s.y < 1.9, id + ': seam/inner ratio ' + s.x.toFixed(2) + ' / ' + s.y.toFixed(2));
    });
    const plain = stat('perlin', false);
    assert.ok(plain.x > 2.5 || plain.y > 2.5, 'a plain noise does have a seam, so the check is meaningful (' + plain.x.toFixed(2) + ')');
});

check('Mandelbrot: the centre of the classic view is inside the set, the corner is outside', () => {
    const img = render('mandelbrot', {}, 33, 33).image, d = img.data;
    const centre = (16 * 33 + 16) * 4, corner = 0;
    assert.deepEqual([d[centre], d[centre + 1], d[centre + 2]], [0, 0, 0], 'inside colour (black)');
    assert.ok(d[corner] + d[corner + 1] + d[corner + 2] > 30, 'outside is coloured');
    const white = render('mandelbrot', { inside: '#ff0000' }, 33, 33).image.data;
    assert.deepEqual([white[centre], white[centre + 1], white[centre + 2]], [255, 0, 0], 'the inside colour is a parameter');
    const julia = render('julia', { cRe: 0, cIm: 0 }, 33, 33).image.data;
    assert.deepEqual([julia[centre], julia[centre + 1], julia[centre + 2]], [0, 0, 0], 'Julia c=0: the unit disc is inside');
});

check('a small preview looks like the full render when both share the reference size', () => {
    ['perlin', 'clouds', 'marble', 'checkerboard', 'worley', 'caustics', 'hexagons'].forEach(id => {
        const big = render(id, { seed: 3 }, 192, 128, { refWidth: 192, refHeight: 128 }).image;
        const small = render(id, { seed: 3 }, 48, 32, { refWidth: 192, refHeight: 128 }).image;
        const down = Lib.resizeBilinear(big, 48, 32);
        let sa = 0, sb = 0, sab = 0, saa = 0, sbb = 0, n = 0;
        for (let i = 0; i < small.data.length; i += 4) {
            const a = Lib.luma(small.data[i], small.data[i + 1], small.data[i + 2]), b = Lib.luma(down.data[i], down.data[i + 1], down.data[i + 2]);
            sa += a; sb += b; sab += a * b; saa += a * a; sbb += b * b; n++;
        }
        const cov = sab / n - (sa / n) * (sb / n), corr = cov / Math.sqrt((saa / n - (sa / n) ** 2) * (sbb / n - (sb / n) ** 2) || 1);
        assert.ok(corr > 0.75, id + ' preview correlation ' + corr.toFixed(3));
    });
});

check('noise primitives keep the calibration the fBm normalisation relies on', () => {
    const kit = Lib.genKit;
    ['value', 'perlin', 'simplex'].forEach(type => {
        const fn = kit.makeBase(type, 99), rng = Lib.mulberry32(5);
        let sum = 0, sq = 0, max = 0;
        const n = 120000;
        for (let i = 0; i < n; i++) { const v = fn(rng() * 300, rng() * 300, 0, 0); sum += v; sq += v * v; max = Math.max(max, Math.abs(v)); }
        const std = Math.sqrt(sq / n - (sum / n) ** 2);
        assert.ok(Math.abs(std - kit.CAL[type].std) / kit.CAL[type].std < 0.06, type + ' std ' + std.toFixed(4) + ' vs ' + kit.CAL[type].std);
        assert.ok(max <= 1.02, type + ' stays within [-1, 1] (' + max.toFixed(3) + ')');
    });
});

check('Worley distances are ordered and the herringbone tiling covers every square exactly once', () => {
    const kit = Lib.genKit;
    ['euclidean', 'manhattan', 'chebyshev'].forEach((name, metric) => {
        const w = kit.worleyNoise(4, 1, metric), rng = Lib.mulberry32(9);
        for (let i = 0; i < 4000; i++) {
            const r = w(rng() * 20, rng() * 20, 0, 0);
            assert.ok(r.f1 >= 0 && r.f1 <= r.f2, name + ' f1 <= f2');
        }
    });
    let bad = 0;
    for (let j = -12; j <= 12; j++) for (let i = -12; i <= 12; i++) {
        const b = kit.herringboneBrick(i + 0.5, j + 0.5, {});
        const squares = b.vertical ? [[b.ai, b.aj], [b.ai, b.aj + 1]] : [[b.ai, b.aj], [b.ai + 1, b.aj]];
        const inside = i >= b.x0 && i < b.x1 && j >= b.y0 && j < b.y1;
        const same = squares.every(([si, sj]) => { const c = kit.herringboneBrick(si + 0.5, sj + 0.5, {}); return c.ai === b.ai && c.aj === b.aj && c.vertical === b.vertical; });
        if (!inside || !same) bad++;
    }
    assert.equal(bad, 0);
});

check('parameter validation falls back to defaults and clamps', () => {
    const p = Lib.normalizeGeneratorParams('perlin', { seed: -5, scale: 'x', octaves: 99, contrast: 1e9, gradient: 'nope', invert: 1, tileable: 'yes', stray: 1 });
    assert.equal(p.stray, undefined);
    assert.equal(p.octaves, 8, 'int clamped to its max');
    assert.equal(p.contrast, 4);
    assert.equal(p.scale, 30, 'garbage falls back to the default');
    assert.ok(Array.isArray(p.gradient) && p.gradient.length >= 2, 'unknown gradient preset -> default stops');
    assert.equal(p.invert, true);
    assert.ok(Number.isInteger(p.seed) && p.seed >= 0);
    const q = Lib.normalizeGeneratorParams('checkerboard', { color1: 'red', angle: 450, tiles: 0 });
    assert.equal(q.color1, '#f2f2f2');
    assert.equal(q.angle, 90);
    assert.equal(q.tiles, 1);
    assert.doesNotThrow(() => render('perlin', { scale: NaN, seed: 'abc', gradient: null }, 8, 8));
});

check('gen.render reports progress, honours cancellation and rejects unknown ids', () => {
    const seen = [];
    Lib.ops['gen.render']({ id: 'perlin', params: {}, width: 64, height: 64 }, { progress: f => seen.push(f), cancelled: () => false });
    assert.ok(seen.length > 2 && seen.every((f, i) => f >= 0 && f <= 1 && (i === 0 || f >= seen[i - 1])), 'monotone progress');
    assert.equal(seen[seen.length - 1], 1);
    let polls = 0;
    const cancelled = Lib.ops['gen.render']({ id: 'fbm', params: {}, width: 128, height: 128 }, { progress() {}, cancelled: () => ++polls > 2 });
    assert.equal(cancelled.cancelled, true);
    assert.ok(polls < 10, 'stops within a few rows');
    const res = Lib.handleMessage({ id: 2, op: 'gen.render', args: { id: 'nope', width: 8, height: 8 } });
    assert.match(res.error, /unknown generator/);
    // an op without hooks (called directly) still works
    assert.equal(Lib.ops['gen.render']({ id: 'checkerboard', width: 8, height: 8 }).image.width, 8);
});

check('draft renders are lighter but keep their size', () => {
    const full = render('reaction-diffusion', { detail: 120, iterations: 400 }, 40, 40).image;
    const draft = render('reaction-diffusion', { detail: 120, iterations: 400 }, 40, 40, { draft: true }).image;
    assert.equal(draft.width, full.width);
});

// ---------------------------------------------------------------------------------------------------------------
// the Generate module in the fake-DOM app
// ---------------------------------------------------------------------------------------------------------------

const runtimes = [];
Pixel.registerModule('test-capture-gen', r => { runtimes.push(r); });
const notes = [];
let menus = null;
const host = makeEl('div');
window.PixelApp.render(host, 'gen1', {
    esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
    t: (k, f) => f || k, api: async () => ({}), iconMarkup: () => '', notify: n => notes.push(n),
    setWindowMenus: (id, m) => { menus = m; }, readonly: false
});
const rt = runtimes[runtimes.length - 1];
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

check('the generators module installs and registers panel, menu and one command per generator', () => {
    assert.ok(rt, 'runtime captured');
    assert.equal(typeof rt.openGenerator, 'function', 'module installer ran');
    const panel = Pixel.getPanel('generate');
    assert.ok(panel && panel.dock === 'inspector');
    assert.ok(host.innerHTML.includes('data-section="generate"'), 'panel section is in the markup');
    assert.ok(host.innerHTML.includes('data-gen-root') && host.innerHTML.includes('data-gen-cats'));
    const generate = menus.find(m => m.id === 'generate');
    assert.ok(generate, 'generate menu exists');
    const subs = generate.items.filter(i => i.items);
    assert.deepEqual(subs.map(i => i.id), ['cat-noise', 'cat-texture', 'cat-pattern', 'cat-math']);
    assert.equal(subs.reduce((n, i) => n + i.items.length, 0), Lib.GENERATORS.length, 'every generator is in the menu');
    const ids = rt.listCommands().map(c => c.id);
    Lib.GENERATORS.forEach(g => assert.ok(ids.includes('generate:' + g.id), 'command for ' + g.id));
    assert.equal(rt.listCommands().find(c => c.id === 'generate:perlin').label, 'Generate: Perlin noise');
    assert.ok(ids.includes('panel:generate'));
});

await check('renderGenerator resolves with a canvas and the image of the requested size', async () => {
    const { canvas, image } = await rt.renderGenerator('checkerboard', { tiles: 4 }, { width: 32, height: 24 });
    assert.equal(canvas.width, 32);
    assert.equal(canvas.height, 24);
    assert.equal(image.data.length, 32 * 24 * 4);
    await assert.rejects(rt.renderGenerator('does-not-exist'), /unknown generator/);
});

await check('Generate adds a named layer above the active one with the chosen blend and opacity; history label generate:<id>', async () => {
    rt.newBlankCanvas(40, 30);
    const before = rt.layers.length, activeBefore = rt.activeLayerIdx;
    rt.runCommand('generate:checkerboard');
    const st = rt.generatorState();
    assert.equal(st.id, 'checkerboard');
    assert.ok(st.values && st.values.tiles === 8, 'defaults are loaded');
    st.opacity = 60; st.blend = 'multiply';
    await rt.generateNow();
    assert.equal(rt.layers.length, before + 1);
    const layer = rt.activeLayer();
    assert.equal(rt.activeLayerIdx, activeBefore + 1);
    assert.equal(layer.name, 'Checkerboard');
    assert.equal(layer.opacity, 0.6);
    assert.equal(layer.blend, 'multiply');
    assert.equal(rt.history[rt.historyIdx].label, 'generate:checkerboard');
});

await check('replace and into-selection outputs go through commitLayerPixels on the active layer', async () => {
    rt.newBlankCanvas(40, 30);
    const st = rt.generatorState();
    rt.openGenerator('stripes');
    const calls = [];
    const original = rt.commitLayerPixels;
    rt.commitLayerPixels = (layer, source, opts) => { calls.push([layer === rt.activeLayer(), source.width, opts.label, opts.respectSelection]); return true; };
    st.output = 'replace';
    await rt.generateNow();
    st.output = 'selection';
    await rt.generateNow();
    rt.commitLayerPixels = original;
    assert.deepEqual(calls, [[true, 40, 'generate:stripes', false], [true, 40, 'generate:stripes', true]]);
});

await check('without a document Generate creates one from the size fields', async () => {
    const host2 = makeEl('div');
    window.PixelApp.render(host2, 'gen2', { esc: v => String(v), t: (k, f) => f || k, api: async () => ({}), iconMarkup: () => '', notify() {}, setWindowMenus() {}, readonly: false });
    const r2 = runtimes[runtimes.length - 1];
    assert.equal(r2.imgWidth, 0);
    r2.openGenerator('perlin');
    const st = r2.generatorState();
    st.docW = 96; st.docH = 64;
    await r2.generateNow();
    assert.equal(r2.imgWidth, 96);
    assert.equal(r2.imgHeight, 64);
    assert.equal(r2.layers.length, 1);
    assert.equal(r2.history[r2.historyIdx].label, 'generate:perlin');
});

await check('live preview uses a transient layer that never reaches the history and disappears with the toggle', async () => {
    rt.newBlankCanvas(40, 30);
    rt.openGenerator('perlin');
    const st = rt.generatorState();
    st.output = 'layer';
    const base = rt.layers.length;
    rt.setGeneratorLive(true);
    await sleep(900);
    const live = rt.layers.find(l => l.transient);
    assert.ok(live, 'transient layer inserted');
    assert.equal(rt.layers.length, base + 1);
    assert.ok(live.preview, 'its pixels are a preview canvas');
    assert.equal(rt.layers.indexOf(live), rt.activeLayerIdx + 1, 'directly above the active layer');
    rt.pushHistory('some other edit');
    const entry = rt.history[rt.historyIdx];
    assert.ok(!entry.layers.some(item => item.meta.id === live.id), 'history does not remember the preview layer');
    rt.setGeneratorLive(false);
    assert.equal(rt.layers.some(l => l.transient), false);
    assert.equal(rt.layers.length, base);
    assert.equal(st.live, false);
    // undo that drops the transient layer switches the toggle off by itself
    rt.setGeneratorLive(true);
    await sleep(900);
    rt.undo();
    await sleep(60);
    assert.equal(rt.generatorState().live, false);
    assert.equal(rt.layers.some(l => l.transient), false);
});

await check('switching panels or tools drops the live preview', async () => {
    rt.newBlankCanvas(40, 30);
    rt.openGenerator('checkerboard');
    rt.generatorState().output = 'layer';
    rt.setGeneratorLive(true);
    await sleep(900);
    assert.ok(rt.layers.some(l => l.transient));
    rt.showPanel('adjust');
    assert.equal(rt.layers.some(l => l.transient), false);
    assert.equal(rt.generatorState().live, false);
});

await sleep(50);
console.log(results.join('\n'));
const failed = results.filter(line => line.startsWith('FAIL'));
if (failed.length) {
    console.error('test-pixel-gen: ' + failed.length + ' of ' + results.length + ' checks failed');
    process.exit(1);
}
console.log('test-pixel-gen: ' + results.length + ' checks passed');
process.exit(0);
