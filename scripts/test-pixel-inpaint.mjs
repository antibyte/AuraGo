// WP7 intelligence: node tests for pixel-lib-inpaint.js (PatchMatch inpainting, palette extraction, harmonies, content
// bounds) and for the DOM-free command matcher of pixel-commands.js (fuzzy score, EN/DE synonyms, context suggestions).
import assert from 'node:assert/strict';
import { loadPixelLibs, loadAppScripts } from './pixel-test-util.mjs';

const { AuraPixelLib: Lib } = loadPixelLibs();
assert.equal(typeof Lib.inpaint, 'function', 'pixel-lib-inpaint.js should export inpaint');
assert.equal(typeof Lib.ops['inpaint.fill'], 'function', 'the inpaint.fill op is registered');

const results = [];
// Objects made inside the vm have another Object.prototype; strict deepEqual needs host objects.
const plain = value => JSON.parse(JSON.stringify(value));
function check(name, fn) {
    try {
        fn();
        results.push('ok   ' + name);
    } catch (err) {
        results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    }
}

// --- helpers ---

function image(w, h, pixel) {
    const img = Lib.createImage(w, h);
    for (let y = 0; y < h; y++) {
        for (let x = 0; x < w; x++) {
            const c = pixel(x, y), o = (y * w + x) * 4;
            img.data[o] = c[0]; img.data[o + 1] = c[1]; img.data[o + 2] = c[2]; img.data[o + 3] = c.length > 3 ? c[3] : 255;
        }
    }
    return img;
}

const STRIPE_COLORS = [[200, 30, 30], [30, 160, 60], [40, 70, 210]];
const stripes = (w, h, period) => image(w, h, x => STRIPE_COLORS[Math.floor(x / period) % 3]);
const checker = (w, h, cell) => image(w, h, (x, y) => ((Math.floor(x / cell) + Math.floor(y / cell)) % 2 ? [230, 184, 90] : [40, 32, 90]));

function rectMask(w, h, x0, y0, bw, bh) {
    const mask = new Uint8ClampedArray(w * h);
    for (let y = y0; y < y0 + bh; y++) for (let x = x0; x < x0 + bw; x++) mask[y * w + x] = 255;
    return mask;
}

// The picture with the masked pixels painted magenta, as the user sees them before the fill.
function punched(truth, mask) {
    const copy = { width: truth.width, height: truth.height, data: new Uint8ClampedArray(truth.data) };
    for (let i = 0; i < mask.length; i++) if (mask[i]) { copy.data[i * 4] = 255; copy.data[i * 4 + 1] = 0; copy.data[i * 4 + 2] = 255; copy.data[i * 4 + 3] = 255; }
    return copy;
}

function meanAbsError(a, b, mask) {
    let sum = 0, n = 0;
    for (let i = 0; i < mask.length; i++) {
        if (!mask[i]) continue;
        for (let c = 0; c < 3; c++) sum += Math.abs(a.data[i * 4 + c] - b.data[i * 4 + c]);
        n += 3;
    }
    return n ? sum / n : 0;
}

function outsideUntouched(before, after, mask) {
    for (let i = 0; i < mask.length; i++) {
        if (mask[i]) continue;
        for (let c = 0; c < 4; c++) if (before.data[i * 4 + c] !== after.data[i * 4 + c]) return false;
    }
    return true;
}

// Smooth random blobs: a texture without exact repeats.
function blobs(w, h, seed) {
    const rand = Lib.mulberry32(seed);
    const img = image(w, h, () => [110, 120, 100]);
    for (let k = 0; k < (w * h) / 350; k++) {
        const cx = rand() * w, cy = rand() * h, rad = 3 + rand() * 9;
        const col = [40 + rand() * 180, 60 + rand() * 160, 30 + rand() * 120];
        for (let y = Math.max(0, Math.floor(cy - rad)); y < Math.min(h, cy + rad); y++) {
            for (let x = Math.max(0, Math.floor(cx - rad)); x < Math.min(w, cx + rad); x++) {
                if ((x - cx) ** 2 + (y - cy) ** 2 < rad * rad) { const o = (y * w + x) * 4; img.data[o] = col[0]; img.data[o + 1] = col[1]; img.data[o + 2] = col[2]; }
            }
        }
    }
    return img;
}

// --- inpainting ---

check('inpaint refills a hole in a striped image exactly and leaves everything else alone', () => {
    const truth = stripes(96, 96, 6);
    const mask = rectMask(96, 96, 34, 34, 28, 28);
    const before = punched(truth, mask);
    const res = Lib.inpaint(before, mask, { seed: 3 });
    assert.equal(res.image.width, 96);
    assert.equal(res.image.data.length, truth.data.length);
    assert.ok(meanAbsError(res.image, truth, mask) < 3, 'stripes continue through the hole: ' + meanAbsError(res.image, truth, mask));
    assert.ok(outsideUntouched(before, res.image, mask), 'pixels outside the mask are unchanged');
    assert.notEqual(res.image, before, 'a copy is returned by default');
    assert.equal(before.data[(40 * 96 + 40) * 4], 255, 'the input keeps its hole');
});

check('inpaint continues a checkerboard and an off-grid stripe period', () => {
    const board = checker(128, 128, 8);
    const boardMask = rectMask(128, 128, 44, 44, 40, 40);
    const a = Lib.inpaint(punched(board, boardMask), boardMask, {});
    assert.ok(meanAbsError(a.image, board, boardMask) < 4, 'checkerboard: ' + meanAbsError(a.image, board, boardMask));
    const wide = stripes(128, 128, 11);
    const wideMask = rectMask(128, 128, 40, 40, 48, 48);
    const b = Lib.inpaint(punched(wide, wideMask), wideMask, {});
    assert.ok(meanAbsError(b.image, wide, wideMask) < 4, 'period 11: ' + meanAbsError(b.image, wide, wideMask));
});

check('inpaint fills a textured picture with matching, non-flat content', () => {
    const truth = blobs(200, 160, 11);
    const mask = rectMask(200, 160, 70, 50, 56, 50);
    const res = Lib.inpaint(punched(truth, mask), mask, { seed: 5 });
    const stat = (img, pick) => {
        let sum = 0, sq = 0, n = 0;
        for (let i = 0; i < mask.length; i++) {
            if (!pick(i)) continue;
            const v = img.data[i * 4 + 1];
            sum += v; sq += v * v; n++;
        }
        const mean = sum / n;
        return { mean, std: Math.sqrt(Math.max(0, sq / n - mean * mean)) };
    };
    const around = stat(truth, i => !mask[i]);
    const inside = stat(res.image, i => mask[i]);
    assert.ok(Math.abs(inside.mean - around.mean) < 25, 'similar mean colour ' + inside.mean + ' vs ' + around.mean);
    assert.ok(inside.std > around.std * 0.4, 'the fill keeps texture (std ' + inside.std + ' vs ' + around.std + ')');
    // no magenta left
    for (let i = 0; i < mask.length; i++) {
        if (!mask[i]) continue;
        assert.ok(!(res.image.data[i * 4] > 240 && res.image.data[i * 4 + 1] < 20), 'no leftover hole colour');
    }
});

check('speed: a 120 px hole in a 512 px image', () => {
    const truth = stripes(512, 512, 6);
    const mask = rectMask(512, 512, 200, 200, 120, 120);
    const t0 = Date.now();
    const res = Lib.inpaint(punched(truth, mask), mask, {});
    const ms = Date.now() - t0;
    results.push('     (512x512, 120x120 hole: ' + ms + ' ms, ' + res.stats.levels + ' levels, ' + res.stats.iterations + ' iterations)');
    assert.ok(meanAbsError(res.image, truth, mask) < 3);
    assert.ok(ms < 20000, 'finishes in a sensible time: ' + ms);
    assert.deepEqual(plain(res.region), { x: 80, y: 80, w: 360, h: 360 }, 'working region = hole + margin (auto margin = hole size, at least 64)');
});

check('inpaint.fill op: progress, in-place mode and the region', () => {
    const truth = stripes(100, 80, 6);
    const mask = rectMask(100, 80, 40, 30, 20, 20);
    const img = punched(truth, mask);
    const seen = [];
    const res = Lib.handleMessage({ id: 9, op: 'inpaint.fill', args: { image: img, mask, patch: 7, inPlace: true } }, { progress: f => seen.push(f) });
    assert.equal(res.id, 9);
    assert.equal(res.error, undefined);
    assert.equal(res.result.image, img, 'in place returns the same image');
    assert.ok(meanAbsError(res.result.image, truth, mask) < 3);
    assert.ok(seen.length > 2 && seen.every((v, i) => i === 0 || v >= seen[i - 1]) && seen[seen.length - 1] === 1, 'progress rises to 1');
    assert.ok(res.transfer.length >= 1, 'the filled buffer is transferred');
    const empty = Lib.handleMessage({ id: 1, op: 'inpaint.fill', args: { image: img, mask: new Uint8ClampedArray(100 * 80) } });
    assert.equal(empty.result.region, null, 'an empty mask changes nothing');
});

check('inpaint.fill: cancelling stops the work', () => {
    const truth = stripes(200, 200, 6);
    const mask = rectMask(200, 200, 60, 60, 80, 80);
    const res = Lib.handleMessage({ id: 2, op: 'inpaint.fill', args: { image: punched(truth, mask), mask } }, { cancelled: () => true });
    assert.match(res.error, /Aborted/);
});

check('inpaint keeps transparency and handles holes at the border', () => {
    const w = 120, h = 80;
    const half = image(w, h, (x, y) => (x < 60 ? STRIPE_COLORS[Math.floor(y / 5) % 3].concat([255]) : [0, 0, 0, 0]));
    const mask = rectMask(w, h, 20, 20, 24, 24);
    const res = Lib.inpaint(punched(half, mask), mask, {});
    for (let y = 20; y < 44; y++) for (let x = 20; x < 44; x++) assert.equal(res.image.data[(y * w + x) * 4 + 3], 255, 'opaque area stays opaque');
    assert.ok(meanAbsError(res.image, half, mask) < 6, 'horizontal bands continue: ' + meanAbsError(res.image, half, mask));
    const corner = rectMask(w, h, 0, 0, 14, 14);
    const cornerRes = Lib.inpaint(punched(half, corner), corner, {});
    for (let y = 0; y < 14; y++) for (let x = 0; x < 14; x++) {
        const o = (y * w + x) * 4;
        assert.ok(!(cornerRes.image.data[o] > 240 && cornerRes.image.data[o + 1] < 20), 'a hole in the corner is filled (no hole colour left)');
    }
    assert.ok(meanAbsError(cornerRes.image, half, corner) < 30, 'and plausibly: ' + meanAbsError(cornerRes.image, half, corner));
    const edge = rectMask(w, h, 50, 60, 30, 20);
    const edgeRes = Lib.inpaint(punched(half, edge), edge, {});
    // the right half is transparent: pixels there can stay transparent, the left part must be opaque again
    assert.equal(edgeRes.image.data[(70 * w + 52) * 4 + 3], 255);
});

check('inpaint refuses a picture without any pixel to copy from and a wrong mask size', () => {
    const full = rectMask(30, 30, 0, 0, 30, 30);
    assert.throws(() => Lib.inpaint(image(30, 30, () => [9, 9, 9]), full, {}), /no-sources/);
    assert.throws(() => Lib.inpaint(image(30, 30, () => [9, 9, 9]), new Uint8ClampedArray(10), {}), /one value per pixel/);
});

check('very large holes are solved at a smaller scale and scaled back', () => {
    const truth = stripes(200, 200, 8);
    const mask = rectMask(200, 200, 40, 40, 110, 110);
    const res = Lib.inpaint(punched(truth, mask), mask, { maxHole: 3000 });
    assert.ok(res.stats.scale && res.stats.scale < 0.6, 'scaled: ' + res.stats.scale);
    assert.ok(meanAbsError(res.image, truth, mask) < 40, 'still stripes: ' + meanAbsError(res.image, truth, mask));
});

check('inpaintRegion: hole bounds plus margin, clipped to the image', () => {
    assert.deepEqual(plain(Lib.inpaintRegion({ x: 100, y: 100, w: 10, h: 10 }, 1000, 1000, { margin: 20 })), { x: 80, y: 80, w: 50, h: 50 });
    assert.deepEqual(plain(Lib.inpaintRegion({ x: 5, y: 5, w: 10, h: 10 }, 100, 100, { margin: 30 })), { x: 0, y: 0, w: 45, h: 45 });
    assert.deepEqual(plain(Lib.inpaintRegion({ x: 40, y: 40, w: 10, h: 10 }, 100, 100, { margin: 'all' })), { x: 0, y: 0, w: 100, h: 100 });
    const auto = Lib.inpaintRegion({ x: 300, y: 300, w: 200, h: 100 }, 2000, 2000, {});
    assert.equal(auto.w, 200 + 2 * 200, 'auto margin follows the hole size');
});

// --- palette, colour ---

check('palette extraction finds the three colours of a three-colour image', () => {
    const red = [220, 30, 30], green = [30, 180, 60], blue = [30, 60, 220];
    const img = image(60, 40, (x, y) => (y < 20 ? (x < 30 ? red : green) : blue)); // 600 + 600 + 1200 pixels
    const palette = Lib.extractPalette(img, 3, { seed: 1 });
    assert.equal(palette.length, 3);
    assert.equal(palette[0].hex, Lib.toHex(...blue), 'sorted by population');
    assert.deepEqual(plain(palette.map(c => c.hex)).sort(), [Lib.toHex(...red), Lib.toHex(...green), Lib.toHex(...blue)].sort());
    assert.ok(Math.abs(palette[0].share - 0.5) < 0.01);
    assert.equal(Lib.extractPalette(img, 12).length, 3, 'asking for more than there is returns the distinct colours');
    assert.equal(Lib.extractPalette(image(4, 4, () => [1, 2, 3, 0]), 5).length, 0, 'transparent pixels are ignored');
});

check('palette of a gradient is spread over the range and sorted', () => {
    const img = image(256, 8, x => [x, 255 - x, 128]);
    const palette = Lib.extractPalette(img, 5);
    assert.ok(palette.length >= 4 && palette.length <= 5);
    for (let i = 1; i < palette.length; i++) assert.ok(palette[i - 1].count >= palette[i].count);
    const reds = palette.map(c => c.r);
    assert.ok(Math.max(...reds) - Math.min(...reds) > 100, 'covers the gradient');
});

check('Lab round trip and harmonies', () => {
    [[255, 0, 0], [12, 200, 90], [250, 250, 250], [0, 0, 0]].forEach(c => {
        const lab = Lib.rgbToLab(...c);
        const back = Lib.labToRgb(...lab);
        c.forEach((v, i) => assert.ok(Math.abs(back[i] - v) < 1.5, 'channel ' + i + ' of ' + c));
    });
    assert.ok(Math.abs(Lib.rgbToLab(255, 255, 255)[0] - 100) < 0.1);
    const harmonies = Lib.colorHarmonies('#ff0000');
    const byId = Object.fromEntries(plain(harmonies).map(h => [h.id, h.colors]));
    assert.deepEqual(plain(harmonies).map(h => h.id), ['complementary', 'analogous', 'triadic', 'split-complementary', 'tetradic', 'monochrome']);
    assert.deepEqual(byId.complementary, ['#ff0000', '#00ffff']);
    assert.deepEqual(byId.triadic, ['#ff0000', '#00ff00', '#0000ff']);
    assert.equal(byId.analogous.length, 3);
    assert.equal(byId.analogous[1], '#ff0000');
    assert.equal(byId.monochrome.length, 5);
    assert.equal(byId.monochrome[2], '#ff0000');
    assert.deepEqual(plain(Lib.colorHarmonies('nonsense')), []);
});

// --- content bounds ---

check('content bounds: alpha mode, colour mode and background detection', () => {
    const w = 40, h = 30;
    const sprite = image(w, h, (x, y) => (x >= 10 && x < 25 && y >= 5 && y < 20 ? [200, 50, 50, 255] : [0, 0, 0, 0]));
    assert.deepEqual(plain(Lib.contentBounds(sprite)), { x: 10, y: 5, w: 15, h: 15 });
    assert.equal(Lib.contentBounds(image(8, 8, () => [1, 2, 3, 0])), null);
    const onWhite = image(w, h, (x, y) => (x >= 12 && x < 30 && y >= 8 && y < 18 ? [30, 30, 30, 255] : [250, 250, 250, 255]));
    assert.deepEqual(plain(Lib.contentBounds(onWhite)), { x: 0, y: 0, w: w, h: h }, 'alpha mode sees the whole opaque image');
    assert.deepEqual(plain(Lib.contentBounds(onWhite, { mode: 'color', background: [250, 250, 250] })), { x: 12, y: 8, w: 18, h: 10 });
    assert.equal(Lib.contentBounds(onWhite, { mode: 'color', background: [250, 250, 250], tolerance: 255 }), null);
    assert.deepEqual(plain(Lib.estimateBackground([[250, 250, 250, 255], [251, 249, 250, 255], [250, 250, 250, 255], [10, 10, 10, 255]])), { transparent: false, color: [250, 250, 250] });
    assert.deepEqual(plain(Lib.estimateBackground([[0, 0, 0, 0], [0, 0, 0, 0], [5, 5, 5, 255], [0, 0, 0, 0]])), { transparent: true, color: null });
});

// --- command matcher (pixel-commands.js, DOM-free parts) ---

globalThis.window = globalThis;
globalThis.AuraPixelApp = { registerModule() {}, registerMenuItems() {}, registerCommand() {}, registerPanel() {}, registerTool() {} };
loadAppScripts(['pixel-commands.js']);
const Pixel = globalThis.AuraPixelApp;

const catalogue = [
    ['f.gblur', 'Gaussian blur', 'filter'], ['f.mblur', 'Motion blur', 'filter'], ['f.zblur', 'Zoom blur', 'filter'], ['l.bg', 'Background layer', 'layer'],
    ['c.bgremove', 'Remove background', 'ai'], ['f.noise', 'Add noise', 'filter'], ['l.new', 'New layer', 'layer'], ['l.dup', 'Duplicate layer', 'layer'],
    ['f.sharp', 'Sharpen', 'filter'], ['i.canvas', 'Canvas size…', 'image'], ['i.crop', 'Crop', 'image'], ['i.trim', 'Trim transparent pixels', 'image'],
    ['e.caf', 'Content-aware fill…', 'edit'], ['s.feather', 'Feather selection', 'select'], ['f.gmap', 'Gradient map', 'filter'], ['f.bc', 'Brightness / contrast', 'filter'],
    ['t.brush', 'Tool: Brush', 'tools'], ['i.rot', 'Rotate 90° clockwise', 'image']
].map(([id, label, group]) => ({ id, label, group, keywords: [] }));

check('fuzzy score: subsequence with word-start bonus', () => {
    assert.equal(Pixel.fuzzyScore('xyz', 'Gaussian blur'), -1, 'not a subsequence');
    assert.ok(Pixel.fuzzyScore('gblur', 'Gaussian blur') > 30);
    assert.ok(Pixel.fuzzyScore('gblur', 'Gaussian blur') > Pixel.fuzzyScore('gblur', 'Background layer blur'), 'word starts beat scattered letters');
    assert.ok(Pixel.fuzzyScore('blur', 'Blur') > Pixel.fuzzyScore('blur', 'Gaussian blur'), 'a prefix beats a later word');
    assert.ok(Pixel.fuzzyScore('layer', 'Layer') > Pixel.fuzzyScore('layer', 'Layer mask settings'), 'a tighter text scores higher');
    const match = Pixel.fuzzyMatch('gb', 'Gaussian blur');
    assert.deepEqual(match.positions, [0, 9], 'positions point at the matched letters');
    assert.equal(Pixel.fuzzyMatch('é', 'Edge').positions[0], 0, 'accents fold');
});

check('search ranks "gaussian blur" first for "gblur"', () => {
    const hits = Pixel.searchCommands(catalogue, 'gblur');
    assert.equal(hits[0].command.id, 'f.gblur');
    assert.equal(Pixel.searchCommands(catalogue, 'gauss blur')[0].command.id, 'f.gblur', 'several words all have to match');
    assert.equal(Pixel.searchCommands(catalogue, 'cafill')[0].command.id, 'e.caf');
    assert.equal(Pixel.searchCommands(catalogue, 'qqqq').length, 0);
});

check('German synonyms find the English commands and the other way round', () => {
    assert.equal(Pixel.searchCommands(catalogue, 'freistellen')[0].command.id, 'c.bgremove');
    assert.equal(Pixel.searchCommands(catalogue, 'rauschen')[0].command.id, 'f.noise');
    assert.ok(['f.gblur', 'f.mblur', 'f.zblur'].includes(Pixel.searchCommands(catalogue, 'weichzeichnen')[0].command.id));
    const layers = Pixel.searchCommands(catalogue, 'ebene').map(h => h.command.id);
    assert.ok(layers.includes('l.new') && layers.includes('l.dup'));
    assert.equal(Pixel.searchCommands(catalogue, 'inhaltsbasiert')[0].command.id, 'e.caf');
    assert.equal(Pixel.searchCommands(catalogue, 'schärfen')[0].command.id, 'f.sharp');
    assert.equal(Pixel.searchCommands(catalogue, 'zuschneiden')[0].command.id, 'i.crop');
    assert.equal(Pixel.searchCommands(catalogue, 'trim')[0].command.id, 'i.trim', 'a direct hit beats a synonym');
    // German labels found with English words
    const german = [{ id: 'g1', label: 'Gaußscher Weichzeichner', group: 'filter', keywords: [] }, { id: 'g2', label: 'Neue Ebene', group: 'layer', keywords: [] }];
    assert.equal(Pixel.searchCommands(german, 'blur')[0].command.id, 'g1');
    assert.equal(Pixel.searchCommands(german, 'layer')[0].command.id, 'g2');
    assert.ok(Pixel.synonymsFor('layer').includes('ebene'));
    assert.deepEqual(Pixel.synonymsFor('x'), []);
});

check('keywords and the recent boost take part in ranking', () => {
    const list = [{ id: 'a', label: 'Alpha tool', group: '', keywords: ['special sauce'] }, { id: 'b', label: 'Beta tool', group: '', keywords: [] }, { id: 'c', label: 'Gamma tool', group: '', keywords: [] }];
    assert.equal(Pixel.searchCommands(list, 'sauce')[0].command.id, 'a');
    const hits = Pixel.searchCommands(list, 'tool', { boost: { c: 50 } });
    assert.equal(hits[0].command.id, 'c');
    assert.equal(Pixel.searchCommands(list, '').length, 3, 'an empty query keeps everything in order');
});

check('context suggestions follow the document state', () => {
    const cmds = [
        { id: 'menu:edit:content-aware-fill', label: 'Content-aware fill…' },
        { id: 'menu:select:feather', label: 'Feather…' },
        { id: 'menu:layer:copy-to-layer', label: 'Copy to new layer' },
        { id: 'filter.auto-enhance', label: 'Auto enhance' },
        { id: 'ai.remove-bg', label: 'Remove background' },
        { id: 'menu:image:trim', label: 'Trim transparent pixels' },
        { id: 'menu:select:disabled-feather', label: 'Feather again', enabled: () => false }
    ];
    const none = { selection: null, imgWidth: 10, imgHeight: 10, layers: [{}, {}], history: [{}, {}] };
    assert.deepEqual(Pixel.contextSuggestions(none, cmds), []);
    const withSelection = Object.assign({}, none, { selection: { type: 'rect' } });
    assert.deepEqual(Pixel.contextSuggestions(withSelection, cmds).map(c => c.id), ['menu:edit:content-aware-fill', 'menu:select:feather', 'menu:layer:copy-to-layer']);
    const photo = { selection: null, imgWidth: 800, imgHeight: 600, layers: [{}], history: [{ label: 'open' }] };
    assert.deepEqual(Pixel.contextSuggestions(photo, cmds).map(c => c.id), ['filter.auto-enhance', 'ai.remove-bg']);
    const edited = Object.assign({}, photo, { history: [{ label: 'open' }, { label: 'draw:brush' }] });
    assert.deepEqual(Pixel.contextSuggestions(edited, cmds), []);
});

console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) {
    console.error('test-pixel-inpaint: failures');
    process.exit(1);
}
console.log('test-pixel-inpaint: ' + results.filter(line => line.startsWith('ok')).length + ' checks passed');
