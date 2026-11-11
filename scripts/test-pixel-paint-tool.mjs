// Art brush tool (pixel-paint.js, pixel-paint-ui.js): registrations in the real module set (fake DOM), medium defaults,
// i18n coverage of every key the two files use, the region cache (one read, one write per flush) and the wet session end.
// Pixels of the engines themselves are tested in test-pixel-paint.mjs; the Atelier panel needs a real DOM and is not clicked here.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { installFakeDom, makeEl, loadPixelApp, englishT } from './pixel-test-dom.mjs';
import { appDir, repoRoot } from './pixel-test-util.mjs';

installFakeDom();
const errors = [];
const originalError = console.error;
console.error = (...args) => { errors.push(args.map(String).join(' ')); };
const Pixel = loadPixelApp();
const Art = Pixel.art;
const paint = window.AuraPixelLib.paint;

let count = 0;
function check(name, fn) {
    try { const out = fn(); if (out && typeof out.then === 'function') throw new Error('sync checks only'); count++; console.log('ok   ' + name); }
    catch (err) { console.log('FAIL ' + name + ': ' + (err && err.stack || err)); process.exitCode = 1; }
}

const runtimes = [];
Pixel.registerModule('paint-test-capture', r => { runtimes.push(r); });
window.PixelApp.render(makeEl('div'), 'paint-1', {
    esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
    t: englishT(), api: async () => ({}), iconMarkup: () => '', notify: () => {}, setWindowMenus: () => {}
});
const rt = runtimes[0];
console.error = originalError;

check('the paint modules load without errors and the runtime carries the art state', () => {
    assert.deepEqual(errors, []);
    assert.ok(Art && Art.cfg, 'Pixel.art exists');
    assert.deepEqual(rt.artPaper, { id: 'cold_press', scale: 1 });
    ['artBrush', 'artWet'].forEach(key => assert.equal(rt[key], null, key));
});

check('art-brush is a paint tool with the specified schema and bar', () => {
    const def = Pixel.getTool('art-brush');
    assert.ok(def);
    assert.equal(def.group, 'paint');
    assert.equal(def.labelKey, 'pixel.art_brush');
    assert.equal(def.onDown.name, 'onDown', 'onDown is wrapped to end wet sessions first');
    const keys = Art.cfg.schema.map(f => f.key);
    ['medium', 'size', 'load', 'mix', 'wetness', 'grain', 'flow', 'smoothing', 'pressure', 'keepDirty'].forEach(k => assert.ok(keys.includes(k), k));
    assert.deepEqual(Art.cfg.bar, ['medium', 'size', 'load', 'mix', 'wetness', 'grain']);
    const medium = Art.cfg.schema.find(f => f.key === 'medium');
    assert.deepEqual(medium.options.map(o => o.value), paint.MEDIA.map(m => m.id));
    assert.equal(medium.default, 'oil');
    const field = k => Art.cfg.schema.find(f => f.key === k);
    assert.equal(field('mix').showIf({ medium: 'oil' }), true);
    assert.equal(field('mix').showIf({ medium: 'watercolor' }), false);
    assert.equal(field('wetness').showIf({ medium: 'watercolor' }), true);
    assert.equal(field('keepDirty').showIf({ medium: 'ink' }), false);
    const s = rt.brushSettingsFor('art-brush');
    assert.equal(s.angleFollow, true);
    assert.equal(s.spacing, 4);
});

check('choosing each medium copies its defaults and other sliders keep the medium', () => {
    const s = rt.brushSettingsFor('art-brush');
    paint.MEDIA.forEach(m => {
        Art.setMedium(rt, m.id);
        assert.equal(s.medium, m.id);
        ['size', 'load', 'mix', 'wetness', 'grain'].forEach(k => assert.equal(s[k], m.defaults[k], m.id + ' ' + k));
    });
    Art.setMedium(rt, 'charcoal');
    Pixel.toolUi.set(rt, Art.cfg, 'load', 33);
    assert.equal(s.medium, 'charcoal');
    assert.equal(s.load, 33);
    assert.equal(s.size, paint.MEDIA.find(m => m.id === 'charcoal').defaults.size);
});

check('the Atelier panel, the commands and the Layer menu item are registered', () => {
    const panel = Pixel.getPanel('art');
    assert.ok(panel);
    assert.equal(panel.dock, 'inspector');
    assert.equal(panel.labelKey, 'pixel.panel_art');
    const html = panel.build(rt);
    assert.equal((html.match(/data-art-medium=/g) || []).length, 12, 'twelve medium buttons');
    assert.ok(html.includes('data-art-palette'));
    ['art.dry-now', 'art.clean-brush', 'art.paper-layer'].forEach(id => assert.ok(Pixel.getCommand(id), id));
    assert.ok(Pixel.listMenuItems('layer', rt).some(i => i.id === 'art-paper-layer'));
});

check('every pixel.art_* / pixel.panel_art key used is in all 16 language files with a non-empty text', () => {
    const keys = new Set();
    ['pixel-paint.js', 'pixel-paint-ui.js'].forEach(file => {
        const src = fs.readFileSync(path.join(appDir, file), 'utf8');
        for (const m of src.matchAll(/'(pixel\.(?:art_|cmd_art_|panel_art)[a-z_]*)'/g)) if (!m[1].endsWith('_')) keys.add(m[1]);
    });
    paint.MEDIA.forEach(m => keys.add(m.labelKey));
    paint.PAPERS.forEach(p => keys.add(p.labelKey));
    ['pixel.art_sec_medium', 'pixel.art_state_wet', 'pixel.art_palette_hint', 'pixel.art_paper_layer'].forEach(k => assert.ok(keys.has(k), k + ' is used'));
    const langs = ['en', 'cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const en = JSON.parse(fs.readFileSync(path.join(repoRoot, 'ui', 'lang', 'desktop', 'en.json'), 'utf8'));
    langs.forEach(lang => {
        const data = JSON.parse(fs.readFileSync(path.join(repoRoot, 'ui', 'lang', 'desktop', lang + '.json'), 'utf8'));
        keys.forEach(key => {
            assert.equal(typeof data[key], 'string', lang + ' lacks ' + key);
            assert.ok(data[key].trim().length > 0, lang + ' has an empty ' + key);
            if (lang !== 'en' && ['pixel.art_brush', 'pixel.art_palette_hint', 'pixel.art_keep_dirty'].includes(key)) assert.notEqual(data[key], en[key], lang + ' copies English for ' + key);
        });
    });
});

// --- region cache with a stub session and the real oil engine ---

function stubWindow(extra) {
    const stats = { reads: 0, writes: 0, flushes: 0, written: null };
    const S = {
        w: 800, h: 300, inMask: false, lockAlpha: false,
        read(rect) { stats.reads++; return new ImageData(new Uint8ClampedArray(rect.w * rect.h * 4).fill(255), rect.w, rect.h); },
        write(rect, old, next) { stats.writes++; stats.written = { rect, next: new Uint8ClampedArray(next.data) }; },
        mark() {}, flush() { stats.flushes++; return true; }
    };
    const r = Object.assign({
        primaryColor: '#d02020', artPaper: { id: 'smooth', scale: 1 }, artBrush: null, artWet: null, artWetMap: null, artWetMapUsed: false, artPanelRefresh: null,
        activeLayer: () => ({ id: 7 }), layerById: () => null
    }, extra);
    return { r, S, stats };
}

check('commitRegion: several dabs make one read and one write per flush', () => {
    const { r, S, stats } = stubWindow();
    const s = Object.assign({}, rt.brushSettingsFor('art-brush'), { medium: 'oil', size: 30, load: 80, mix: 60, grain: 0 });
    const st = { r, s, S, color: '#d02020' };
    assert.notEqual(Art.cfg.begin(r, st), false);
    assert.ok(s.spacing > 0 && s.spacing < 100, 'spacing is derived from the brush');
    assert.ok(r.artWetMap && r.artWetMap.map.length === 800 * 300, 'wet map allocated for a dry medium');
    for (let i = 0; i < 6; i++) Art.cfg.stamp(st, 80 + i * 4, 60, 30, 1, 0, 1);
    assert.equal(stats.reads, 1, 'one getImageData for the first region');
    assert.equal(stats.writes, 0, 'nothing is written per dab');
    st.S.flush();
    assert.equal(stats.writes, 1, 'one write for the flush');
    assert.equal(stats.flushes, 1);
    let red = 0;
    const d = stats.written.next;
    for (let i = 0; i < d.length; i += 4) if (d[i] > d[i + 1] + 40) red++;
    assert.ok(red > 50, 'the oil stroke painted red pixels (' + red + ')');
    st.S.flush();
    assert.equal(stats.writes, 1, 'a flush without new dabs writes nothing');
    Art.cfg.stamp(st, 700, 250, 30, 1, 0, 1); // far outside the cached region: commit + a second read
    st.S.flush();
    assert.equal(stats.reads, 2);
    assert.equal(stats.writes, 2);
});

check('a wet medium starts a wet session; finishWet drops it and records history once', () => {
    const calls = { history: [], dirty: 0, composite: 0 };
    const { r, S } = stubWindow({
        layerById: id => (id === 7 ? { id: 7, canvas: makeEl('canvas') } : null), copyCanvas: () => makeEl('canvas'),
        markLayerDirty() { calls.dirty++; }, compositeLayers() { calls.composite++; }, pushHistory(label) { calls.history.push(label); },
        strokeState: null
    });
    const s = Object.assign({}, rt.brushSettingsFor('art-brush'), { medium: 'watercolor', size: 40, load: 45, wetness: 70, grain: 55 });
    const st = { r, s, S, color: '#2060c0' };
    assert.notEqual(Art.cfg.begin(r, st), false);
    const ws = r.artWet;
    assert.ok(ws && ws.wet && ws.timer, 'wet session with a timer');
    for (let i = 0; i < 5; i++) Art.cfg.stamp(st, 60 + i * 5, 60, 40, 1, 0, 1);
    assert.ok(ws.wet.hasPaint(), 'the dabs went into the wet layer');
    ws.changed = true;
    assert.equal(Art.finishWet(r, { fast: true }), true);
    assert.equal(r.artWet, null);
    assert.equal(ws.wet.hasPaint(), false, 'baked');
    assert.deepEqual(calls.history, ['draw:art-brush']);
    assert.equal(calls.composite, 1);
});

const slowWet = (stepMs, counter) => ({
    step() { counter.n++; const t0 = Date.now(); while (Date.now() - t0 < stepMs) { /* busy */ } return true; },
    dirtyRect: () => null, paintRect: () => null, bake() {}, hasPaint: () => true
});
const alphaCanvas = alpha => ({ width: 10, height: 10, getContext: () => ({ getImageData: () => ({ data: [0, 0, 0, alpha] }), putImageData() {} }) });

check('fast finishWet stops at its time budget instead of running 600 steps', () => {
    const { r } = stubWindow({ markLayerDirty() {}, compositeLayers() {}, pushHistory() {} });
    const counter = { n: 0 };
    r.artWet = { layerId: 7, wet: slowWet(8, counter), changed: false, timer: 0 };
    const t0 = Date.now();
    Art.finishWet(r, { fast: true });
    const ms = Date.now() - t0;
    assert.equal(r.artWet, null);
    assert.ok(counter.n >= 2 && counter.n <= 8, 'steps ' + counter.n);
    assert.ok(ms < 150, 'took ' + ms + ' ms');
});

check('dryNow does nothing while a stroke is in progress', () => {
    const { r } = stubWindow({ strokeState: {}, markLayerDirty() {}, compositeLayers() {}, pushHistory() { throw new Error('no history mid-stroke'); } });
    const wet = slowWet(0, { n: 0 });
    r.artWet = { layerId: 7, wet, changed: true, timer: 0 };
    r.artWetMapUsed = true;
    Art.dryNow(r);
    assert.ok(r.artWet, 'session kept');
    assert.equal(r.artWetMapUsed, true);
    assert.equal(Pixel.getCommand('art.dry-now').enabled(r), false);
});

check('a foreign edit of the layer ends the wet session silently; a missing layer too', () => {
    const calls = [];
    const layer = { id: 7, rev: 5, canvas: makeEl('canvas') };
    const { r } = stubWindow({ layerById: () => layer, markLayerDirty() { calls.push('dirty'); }, pushHistory() { calls.push('history'); } });
    r.artWet = { layerId: 7, rev: 4, wet: slowWet(0, { n: 0 }), changed: true, timer: 0 };
    assert.equal(Art.renderWet(r), false);
    assert.equal(r.artWet, null);
    assert.deepEqual(calls, []);
    const gone = stubWindow({ layerById: () => null }).r;
    gone.artWet = { layerId: 7, rev: 1, wet: slowWet(0, { n: 0 }), changed: true, timer: 0 };
    assert.equal(Art.renderWet(gone), false);
    assert.equal(gone.artWet, null);
});

check('abort of a wet stroke keeps the session and renders the whole paint area again', () => {
    const rendered = [];
    const layer = { id: 7, rev: 1, canvas: makeEl('canvas') };
    const { r } = stubWindow({ layerById: () => layer, markLayerDirty() { layer.rev++; }, compositeLayers() {}, strokeState: null });
    const wet = { paintRect: () => ({ x: 0, y: 0, w: 4, h: 4 }), dirtyRect: () => null, render(out) { rendered.push(out.w); }, bake() {}, hasPaint: () => true };
    const timer = setInterval(() => {}, 1000);
    r.artWet = { layerId: 7, rev: 0, wet, base: makeEl('canvas'), docW: 10, docH: 10, changed: false, timer };
    Art.cfg.abort(r, { art: { wet: true } });
    assert.ok(r.artWet, 'session kept');
    assert.deepEqual(rendered, [4]);
    assert.equal(r.artWet.changed, true);
    assert.equal(r.artWet.rev, layer.rev);
    clearInterval(timer);
});

check('a wet session must end before a dry stroke or a stroke on another layer starts', () => {
    const { r } = stubWindow();
    const layer = { id: 7, canvas: { width: 800, height: 300 } };
    const paper = paint.makePaper('smooth');
    r.artWet = { layerId: 7, docW: 800, docH: 300, paperId: paper.id, scale: 1 };
    assert.equal(Art.wetMustEnd(r, { medium: 'watercolor' }, layer), false, 'same layer, same paper');
    assert.equal(Art.wetMustEnd(r, { medium: 'oil' }, layer), true, 'dry medium');
    assert.equal(Art.wetMustEnd(r, { medium: 'ink' }, { id: 8, canvas: layer.canvas }), true, 'other layer');
});

check('paper layer placement: above an opaque active layer with a painting layer on top, else below the active one', () => {
    assert.deepEqual(Art.paperPlacement(2, true), { paper: 3, paint: 4, active: 'paint' });
    assert.deepEqual(Art.paperPlacement(2, false), { paper: 2, paint: -1, active: 'keep' });
    assert.equal(Art.layerIsOpaque({ canvas: alphaCanvas(255) }), true);
    assert.equal(Art.layerIsOpaque({ canvas: alphaCanvas(0) }), false);
    const build = alpha => {
        const history = [], bg = { name: 'bg', canvas: alphaCanvas(alpha) };
        const r = {
            imgWidth: 10, imgHeight: 10, layers: [bg], MAX_LAYERS: 50, artPaper: { id: 'smooth', scale: 1 }, activeLayerIdx: 0, editMaskMode: true,
            t: k => k, notify() {}, activeLayer() { return this.layers[this.activeLayerIdx]; }, createLayer: o => ({ name: o.name, canvas: alphaCanvas(0) }),
            markLayerDirty() {}, layersStructureChanged() {}, pushHistory: l => history.push(l)
        };
        return { r, bg, history };
    };
    let t = build(255);
    assert.equal(Art.addPaperLayer(t.r), true);
    assert.deepEqual(t.r.layers.map(l => l.name), ['bg', 'pixel.art_paper_layer', 'pixel.art_paint_layer']);
    assert.equal(t.r.activeLayerIdx, 2);
    assert.equal(t.history.length, 1);
    t = build(0);
    Art.addPaperLayer(t.r);
    assert.deepEqual(t.r.layers.map(l => l.name), ['pixel.art_paper_layer', 'bg']);
    assert.equal(t.r.activeLayer(), t.bg);
    assert.equal(t.history.length, 1);
});

// The interval itself is observed after the fact: no tick may run once finishWet returned.
const pending = [];
(function asyncTimerCheck() {
    const calls = { history: 0 };
    const { r } = stubWindow({ layerById: () => null, markLayerDirty() {}, compositeLayers() {}, pushHistory() { calls.history++; } });
    let ticks = 0;
    const wet = { step: () => false, dirtyRect: () => null, bake() {}, hasPaint: () => true };
    r.artWet = { layerId: 7, wet, changed: false, timer: setInterval(() => { ticks++; }, 5) };
    Art.finishWet(r, { fast: true });
    pending.push(new Promise(resolve => setTimeout(resolve, 40)).then(() => {
        try { assert.equal(r.artWet, null); assert.equal(ticks, 0, 'timer cleared'); assert.equal(calls.history, 0, 'no history without changes'); count++; console.log('ok   finishWet clears the interval and adds no history when nothing changed'); }
        catch (err) { console.log('FAIL finishWet timer: ' + (err && err.stack || err)); process.exitCode = 1; }
    }));
})();

await Promise.all(pending);
console.log('test-pixel-paint-tool: ' + count + ' checks passed');
process.exit(process.exitCode || 0);
