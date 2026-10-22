// Runs the real Pixel modules (the loader's script order, including pixel.js render()) against a fake DOM with a
// recording canvas: layer model, history sharing, selection masks, commitLayerPixels and the WP0b registries
// (modules, panels, tools, menus, commands, overlays, jobs, parameter controls).
import assert from 'node:assert/strict';
import { installFakeDom, makeEl, loaderScripts, loaderStyles, loadPixelApp, englishT } from './pixel-test-dom.mjs';

installFakeDom();
// A module whose installer throws is logged and skipped by pixel.js; the integration check below fails on that.
const consoleErrors = [];
const realConsoleError = console.error;
console.error = (...args) => { consoleErrors.push(args.map(a => (a && a.message) || String(a)).join(' ')); };
const Pixel = loadPixelApp();

// Registered before the first window renders: captures every runtime that is created.
const runtimes = [];
const installOrder = [];
Pixel.registerModule('test-capture', r => { runtimes.push(r); installOrder.push('test-capture'); });

const notes = [];
let menusSeen = null;
function renderWindow(id) {
    const host = makeEl('div');
    window.PixelApp.render(host, id, {
        esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
        t: englishT(), api: async () => ({}), iconMarkup: () => '', notify: n => notes.push(n),
        setWindowMenus: (windowId, menus) => { menusSeen = menus; },
        readonly: false
    });
    return { host, rt: runtimes[runtimes.length - 1] };
}

const { host: host1, rt } = renderWindow('w1');
console.error = realConsoleError;
assert.ok(rt, 'runtime captured');
const events = [];
['layers-changed', 'selection-changed', 'document-changed', 'tool-changed', 'history-changed', 'active-layer-changed', 'panel-changed'].forEach(e => rt.on(e, () => events.push(e)));

function uniqueSnaps() {
    const s = new Set();
    rt.history.forEach(e => e.layers.forEach(l => { if (l.snap) s.add(l.snap); if (l.maskSnap) s.add(l.maskSnap); }));
    return s.size;
}
const results = [];
const pending = [];
function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') {
            const done = out.then(() => results.push('ok   ' + name), fail);
            pending.push(done);
            return done;
        }
        results.push('ok   ' + name);
    } catch (err) { fail(err); }
    return undefined;
}
const keyEvent = (props) => {
    const e = Object.assign({ key: '', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null } }, props);
    e.preventDefault = () => { e.defaultPrevented = true; e.prevented = true; };
    return e;
};
const pointerEvent = (props) => Object.assign({ clientX: 0, clientY: 0, button: 0, buttons: 1, pointerId: 1, pointerType: 'mouse', pressure: 0.5, shiftKey: false, altKey: false, ctrlKey: false, metaKey: false }, props);

// ---------------------------------------------------------------------------------------------------------------
// WP0a: layers, history, selection, commitLayerPixels
// ---------------------------------------------------------------------------------------------------------------

check('new blank canvas creates one document-sized layer', () => {
    rt.newBlankCanvas(100, 80);
    assert.equal(rt.layers.length, 1);
    assert.equal(rt.layers[0].canvas.width, 100);
    assert.equal(rt.layers[0].canvas.height, 80);
    assert.equal(rt.history.length, 1);
    assert.ok(events.includes('document-changed'));
});

check('add layer, history shares the unchanged background snapshot', () => {
    rt.addLayer();
    assert.equal(rt.layers.length, 2);
    assert.equal(rt.history.length, 2);
    assert.equal(rt.history[0].layers[0].snap, rt.history[1].layers[0].snap, 'background snap shared');
    assert.notEqual(rt.history[1].layers[1].snap, null);
    assert.equal(uniqueSnaps(), 2);
});

check('painting on the active layer bumps only its rev and re-snapshots only it', () => {
    const bg = rt.layers[0], top = rt.layers[1];
    const bgRev = bg.rev, topRev = top.rev;
    const ctx = rt.getActiveCtx();
    assert.equal(ctx.canvas, top.canvas);
    assert.equal(bg.rev, bgRev);
    assert.ok(top.rev > topRev);
    rt.pushHistory('draw:brush');
    assert.equal(rt.history.length, 3);
    assert.equal(rt.history[2].layers[0].snap, rt.history[0].layers[0].snap);
    assert.notEqual(rt.history[2].layers[1].snap, rt.history[1].layers[1].snap);
    assert.equal(uniqueSnaps(), 3);
});

check('undo restores layer count and reuses canvases whose rev matches', () => {
    const before = rt.layers[0].canvas;
    rt.undo();
    assert.equal(rt.layers.length, 2);
    assert.equal(rt.layers[0].canvas, before, 'background canvas reused');
    rt.undo();
    assert.equal(rt.layers.length, 1);
    rt.undo();
    assert.equal(rt.layers.length, 1);
    rt.redo(); rt.redo();
    assert.equal(rt.layers.length, 2);
    assert.equal(rt.historyIdx, 2);
});

check('redo tail is dropped by a new push', () => {
    rt.undo();
    rt.addLayer();
    assert.equal(rt.history.length, 3);
    assert.equal(rt.layers.length, 3);
    assert.equal(rt.historyIdx, 2);
});

check('layer ids stay unique and the active layer id survives restore', () => {
    const ids = rt.layers.map(l => l.id);
    assert.equal(new Set(ids).size, ids.length);
    const activeId = rt.activeLayer().id;
    rt.undo(); rt.redo();
    assert.equal(rt.activeLayer().id, activeId);
});

check('duplicate / merge down / flatten keep one canvas per layer', () => {
    rt.duplicateLayer();
    assert.equal(rt.layers.length, 4);
    rt.mergeDown();
    assert.equal(rt.layers.length, 3);
    rt.flattenLayers();
    assert.equal(rt.layers.length, 1);
    assert.equal(rt.layers[0].canvas.width, 100);
    rt.undo();
    assert.equal(rt.layers.length, 3);
});

check('delete layer', () => {
    rt.deleteLayer();
    assert.equal(rt.layers.length, 2);
    rt.undo();
    assert.equal(rt.layers.length, 3);
});

check('selection: rect mask, invert, combine modes', () => {
    rt.selection = { type: 'rect', x: 10, y: 10, w: 20, h: 10 };
    const m = rt.getSelectionMask();
    assert.equal(m.alpha.length, 100 * 80);
    assert.equal(m.alpha[10 * 100 + 10], 255);
    assert.equal(m.alpha[9 * 100 + 10], 0);
    assert.equal(m.w, 20); assert.equal(m.h, 10);
    assert.equal(rt.getSelectionMask(), m, 'cached by selectionRev');
    assert.equal(rt.invertSelection(), true);
    assert.equal(rt.selection.type, 'mask');
    assert.equal(rt.selection.alpha[10 * 100 + 10], 0);
    assert.equal(rt.selection.alpha[0], 255);
    rt.invertSelection();
    assert.equal(rt.selection.alpha[10 * 100 + 10], 255);
    const extra = new Uint8ClampedArray(100 * 80); extra[50 * 100 + 50] = 255;
    rt.setSelectionMask(extra, { mode: 'add' });
    assert.equal(rt.selection.alpha[50 * 100 + 50], 255);
    assert.equal(rt.selection.alpha[10 * 100 + 10], 255);
    rt.setSelectionMask(extra, { mode: 'subtract' });
    assert.equal(rt.selection.alpha[50 * 100 + 50], 0);
    rt.setSelectionMask(new Uint8ClampedArray(100 * 80), { mode: 'replace' });
    assert.equal(rt.selection, null, 'empty result deselects');
});

check('selection ellipse + magic wand + lasso produce masks', () => {
    rt.selection = { type: 'ellipse', x: 10, y: 10, w: 40, h: 30 };
    const e = rt.getSelectionMask();
    assert.ok(e && e.alpha.length === 8000);
    rt.deselect();
    rt.fillTolerance = 10;
    rt.magicWandSelect(5, 5);
    assert.equal(rt.selection.type, 'mask');
    rt.deselect();
    rt.lassoPath = [{ x: 5, y: 5 }, { x: 40, y: 5 }, { x: 40, y: 30 }];
    rt.commitLassoPath();
    // zero-filled fake canvas has no coverage, so the lasso yields no selection; must not throw
    rt.deselect();
});

check('commitLayerPixels honours selection, locks and writes history', () => {
    const layer = rt.activeLayer();
    const src = rt.makeCanvas(100, 80);
    assert.equal(rt.commitLayerPixels(layer, src, { label: 'test' }), true);
    assert.equal(rt.history[rt.historyIdx].label, 'test');
    rt.selection = { type: 'rect', x: 5, y: 5, w: 10, h: 10 };
    assert.equal(rt.commitLayerPixels(layer, src, { label: 'sel' }), true);
    rt.selection = { type: 'ellipse', x: 5, y: 5, w: 10, h: 10 };
    assert.equal(rt.commitLayerPixels(layer, src, { label: 'ell', history: false }), true);
    layer.lockAlpha = true;
    assert.equal(rt.commitLayerPixels(layer, src, { label: 'lockalpha', history: false }), true);
    layer.lockAlpha = false;
    layer.lockPixels = true;
    assert.equal(rt.commitLayerPixels(layer, src, { label: 'locked' }), false);
    layer.lockPixels = false;
    rt.deselect();
});

check('masks: toggle creates mask and history keeps it', () => {
    rt.toggleLayerMask(rt.activeLayerIdx);
    const layer = rt.activeLayer();
    assert.ok(layer.maskCanvas);
    assert.equal(rt.editMaskMode, true);
    rt.getActiveCtx();
    assert.ok(rt.history[rt.historyIdx].layers.find(l => l.meta.id === layer.id).maskSnap);
    rt.toggleLayerMask(rt.activeLayerIdx);
    assert.equal(rt.editMaskMode, false);
    rt.undo(); rt.redo();
});

check('brush stroke spec in mask mode paints alpha', () => {
    rt.toggleLayerMask(rt.activeLayerIdx);
    rt.activeTool = 'eraser';
    let spec = rt.brushStrokeSpec();
    assert.equal(spec.op, 'destination-out');
    rt.activeTool = 'brush';
    spec = rt.brushStrokeSpec();
    assert.equal(spec.color, '#ffffff');
    assert.equal(spec.op, 'source-over');
    rt.toggleLayerMask(rt.activeLayerIdx);
    rt.activeTool = null;
});

check('adjust preview is non-destructive and apply commits through history', () => {
    rt.captureAdjustmentSource();
    const layer = rt.activeLayer();
    const rev = layer.rev;
    rt.adjustments.brightness = 20;
    rt.applyAdjustmentsPreview();
    assert.ok(layer.preview, 'preview set');
    assert.equal(layer.rev, rev, 'layer untouched by preview');
    const hist = rt.history.length;
    rt.applyAdjustments();
    assert.equal(layer.preview, null);
    assert.ok(layer.rev > rev);
    assert.equal(rt.history.length, hist + 1);
    assert.equal(rt.adjustments.brightness, 0);
});

check('compare mode enter/exit leaves no preview behind', () => {
    rt.adjustments.contrast = 15;
    rt.toggleCompare();
    assert.equal(rt.compareMode, true);
    rt.renderCompare();
    rt.toggleCompare();
    assert.equal(rt.compareMode, false);
    rt.resetAdjustments();
    assert.equal(rt.activeLayer().preview, null);
});

check('painting with a pending adjust preview bakes it in first', () => {
    rt.captureAdjustmentSource();
    rt.adjustments.saturation = 30;
    rt.applyAdjustmentsPreview();
    const hist = rt.history.length;
    rt.getActiveCtx();
    assert.equal(rt.activeLayer().preview, null);
    assert.equal(rt.history.length, hist + 1);
});

check('filter preview is non-destructive; apply commits; reset discards', () => {
    const layer = rt.activeLayer();
    const rev = layer.rev;
    rt.selectFilter('grayscale');
    assert.ok(layer.preview, 'filter preview on layer');
    assert.equal(layer.rev, rev);
    rt.filterStrength = 60;
    rt.previewFilterBlend();
    rt.resetFilterPreview();
    assert.equal(layer.preview, null);
    assert.equal(rt.filterPreview, null);
    rt.selectFilter('sharpen');
    rt.filterCompareMode = true;
    rt.previewFilterBlend();
    const hist = rt.history.length;
    rt.applyFilter();
    assert.equal(layer.preview, null);
    assert.equal(rt.history.length, hist + 1);
    rt.filterCompareMode = false;
    rt.filterStrength = 100;
});

check('rotate / flip / resize bump every rev and keep canvases consistent', () => {
    const revs = rt.layers.map(l => l.rev);
    rt.rotateCanvas(90);
    assert.equal(rt.imgWidth, 80); assert.equal(rt.imgHeight, 100);
    rt.layers.forEach((l, i) => { assert.ok(l.rev > revs[i]); assert.equal(l.canvas.width, 80); if (l.maskCanvas) assert.equal(l.maskCanvas.width, 80); });
    rt.undo();
    assert.equal(rt.imgWidth, 100);
    rt.layers.forEach(l => assert.equal(l.canvas.width, 100));
    rt.redo(); rt.undo();
    rt.flipCanvas(true);
    rt.undo();
});

check('flattenDocument returns a document-sized canvas', () => {
    const c = rt.flattenDocument();
    assert.equal(c.width, rt.imgWidth); assert.equal(c.height, rt.imgHeight);
    const pooled = rt.flattenToDataURL('image/png');
    assert.ok(pooled.startsWith('data:'));
});

check('composite caches: many layers, blend, clip and mask do not throw', () => {
    rt.newBlankCanvas(64, 64);
    for (let i = 0; i < 5; i++) rt.addLayer();
    rt.layers[2].blend = 'multiply';
    rt.layers[3].clip = true;
    rt.layers[4].maskCanvas = rt.makeCanvas(64, 64);
    rt.layers[4].opacity = 0.5;
    rt.activeLayerIdx = 2;
    rt.cctx.log.length = 0;
    rt.compositeLayers();
    const draws = rt.cctx.log.filter(e => e[0] === 'drawImage').length;
    assert.ok(draws >= 3 && draws <= 6, 'draws=' + draws);
    rt.activeLayerIdx = 1;
    rt.compositeLayers();
    rt.activeLayerIdx = 5;
    rt.compositeLayers();
    rt.renderComposition(rt.makeCanvas(64, 64).getContext('2d'));
    rt.renderComposition(rt.makeCanvas(64, 64).getContext('2d'), rt.layers, { previews: false });
    const c = rt.sampleComposite(1, 1);
    assert.equal(c.length, 4);
    rt.layers[5].preview = rt.makeCanvas(64, 64);
    assert.equal(rt.sampleComposite(1, 1).length, 4);
    rt.layers[5].preview = null;
});

check('tool-changed and selection events fire', () => {
    rt.setActiveTool('brush');
    assert.ok(events.includes('tool-changed'));
    assert.ok(events.includes('selection-changed'));
    assert.ok(events.includes('history-changed'));
    rt.setActiveTool('brush');
});

check('move selection (mask) shifts alpha', () => {
    rt.newBlankCanvas(40, 40);
    const alpha = new Uint8ClampedArray(40 * 40);
    for (let y = 5; y < 10; y++) for (let x = 5; x < 10; x++) alpha[y * 40 + x] = 255;
    rt.setSelectionMask(alpha, { mode: 'replace' });
    rt.activeTool = 'move';
    assert.equal(rt.beginMoveSelection(), true);
    rt.selection = Object.assign({}, rt.selection, { x: 15, y: 12 });
    rt.commitMoveSelection();
    assert.equal(rt.selection.type, 'mask');
    assert.equal(rt.selection.x, 15); assert.equal(rt.selection.y, 12);
    assert.equal(rt.selection.alpha[12 * 40 + 15], 255);
    assert.equal(rt.selection.alpha[5 * 40 + 5], 0);
    rt.deselect();
    rt.activeTool = null;
});

check('cut / delete use the selection alpha', () => {
    rt.selection = { type: 'ellipse', x: 5, y: 5, w: 20, h: 20 };
    rt.deleteSelection();
    rt.selection = { type: 'rect', x: 5, y: 5, w: 20, h: 20 };
    rt.deleteSelection();
    rt.deselect();
});

check('floodFill and gradient go through commitLayerPixels', () => {
    const hist = rt.history.length;
    rt.fillTolerance = 5;
    rt.floodFill(3, 3, '#ff0000');
    assert.equal(rt.history.length, hist + 1);
    rt.gradientMode = 'radial';
    rt.brushOpacity = 50;
    rt.commitGradient(2, 2, 30, 30);
    assert.equal(rt.history.length, hist + 2);
    rt.brushOpacity = 100;
});

check('history budget keeps at least two entries and MAX_HISTORY bounds the stack', () => {
    for (let i = 0; i < 70; i++) { rt.getActiveCtx(); rt.pushHistory('x' + i); }
    assert.ok(rt.history.length <= 60, 'len ' + rt.history.length);
    assert.equal(rt.historyIdx, rt.history.length - 1);
});

await new Promise(r => setTimeout(r, 30));
check('layers-changed arrives after the frame', () => {
    assert.ok(events.includes('layers-changed'));
    assert.ok(events.includes('active-layer-changed'));
});

// sticker flow (needs the library stub)
window.AuraStickerLibrary = { image: async () => ({ width: 10, height: 10 }), mount: () => ({ setActive() {}, setDisabled() {}, dispose() {} }) };
rt.newBlankCanvas(60, 60);
await rt.addSticker('star');
check('sticker layer added with its own canvas, snap is null in history', () => {
    assert.equal(rt.layers.length, 2);
    const sticker = rt.layers[1];
    assert.ok(sticker.sticker && sticker.canvas.width === 60);
    const entry = rt.history[rt.historyIdx];
    assert.equal(entry.layers[1].snap, null);
    assert.ok(entry.layers[1].meta.sticker);
    rt.undo();
    assert.equal(rt.layers.length, 1);
    rt.redo();
    assert.ok(rt.layers[1].sticker, 'sticker restored from meta');
    assert.equal(rt.layers[1].canvas.width, 60);
});
check('sticker mask is allowed and rasterize bumps rev + pushes', () => {
    rt.toggleLayerMask(1);
    assert.ok(rt.layers[1].maskCanvas);
    assert.equal(rt.requireRasterLayer(), true, 'mask edit on sticker allowed');
    rt.toggleLayerMask(1);
    const rev = rt.layers[1].rev;
    rt.rasterizeSticker();
    assert.ok(!rt.layers[1].sticker);
    assert.ok(rt.layers[1].rev > rev);
    assert.ok(rt.history[rt.historyIdx].layers[1].snap, 'rasterized layer snapshotted');
});

// ---------------------------------------------------------------------------------------------------------------
// WP0b: loader, modules, docks
// ---------------------------------------------------------------------------------------------------------------

check('loader order: libs before core, every stub module and style listed, worker not listed', () => {
    const scripts = loaderScripts();
    const expected = ['pixel-state.js', 'pixel-lib-core.js', 'pixel-lib-fx.js', 'pixel-lib-gen.js', 'pixel-lib-cutout.js', 'pixel-lib-inpaint.js',
        'pixel-core.js', 'pixel-view.js', 'pixel-canvas.js', 'pixel-tools.js', 'pixel-actions.js', 'pixel-filters.js', 'pixel-events.js',
        'pixel-stickers.js', 'pixel-layers.js', 'pixel-selection.js', 'pixel-brush.js', 'pixel-tools-pro.js', 'pixel-transform.js',
        'pixel-generators.js', 'pixel-cutout.js', 'pixel-smart.js', 'pixel-commands.js', 'pixel-ora.js', 'pixel.js'];
    assert.deepEqual(scripts, expected);
    assert.ok(!scripts.includes('pixel-worker.js'));
    assert.deepEqual(loaderStyles(), ['pixel.css', 'desktop-stickers.css', 'pixel-stickers.css', 'pixel-layers.css', 'pixel-filters.css',
        'pixel-generators.css', 'pixel-cutout.css', 'pixel-selection.css', 'pixel-tools-pro.css', 'pixel-smart.css']);
});

check('modules install in load order and the stubs registered themselves', () => {
    const ids = Pixel.listModules().map(m => m.id);
    assert.deepEqual(ids.slice(0, 10), ['layers', 'selection', 'brush', 'tools-pro', 'transform', 'generators', 'cutout', 'smart', 'commands', 'ora']);
    assert.equal(ids[ids.length - 1], 'test-capture');
    assert.equal(rt.layers.length > 0, true);
});

check('markup: right column has the inspector tabs, the splitter and the bottom dock', () => {
    const html = host1.innerHTML;
    const at = s => { const i = html.indexOf(s); assert.ok(i >= 0, 'markup has ' + s); return i; };
    assert.ok(at('data-panel-container') < at('data-dock="inspector"'));
    assert.ok(at('data-dock="inspector"') < at('data-dock-splitter'));
    assert.ok(at('data-dock-splitter') < at('data-dock="bottom"'));
    const order = ['adjust', 'filters', 'colors', 'transform', 'ai', 'stickers'].map(id => at('data-panel="' + id + '"'));
    assert.deepEqual(order, order.slice().sort((a, b) => a - b), 'inspector tabs in order');
    assert.ok(at('data-panel="adjust"') < at('data-dock-splitter'));
    assert.ok(at('data-panel="layers"') > at('data-dock-splitter'));
    assert.ok(at('data-panel="history"') > at('data-panel="layers"'));
    assert.ok(!html.includes('class="pixel-tabs"') || html.indexOf('class="pixel-toolbar"') < html.indexOf('data-dock="inspector"'));
    // data hooks the camera handoff browser test and the shell rely on
    ['data-action="rotate-cw"', 'data-action="save"', 'data-canvas', 'data-section="transform"', 'data-tool-rail', 'data-options-bar'].forEach(at);
    assert.ok(/data-section="layers"(?! hidden)/.test(html), 'layers is the visible bottom section');
    assert.ok(/data-section="filters" hidden/.test(html), 'filters starts hidden');
    assert.ok(/data-section="adjust"(?! hidden)/.test(html), 'adjust is the visible inspector section');
    assert.deepEqual({ ...rt.activePanels }, { inspector: 'adjust', bottom: 'layers' });
});

check('tool rail is built from the registry in group order', () => {
    const html = host1.innerHTML;
    const idx = id => html.indexOf('data-draw-tool="' + id + '"');
    const order = ['select-rect', 'brush', 'fill', 'line', 'text'].map(idx);
    assert.ok(order.every(i => i >= 0));
    assert.deepEqual(order, order.slice().sort((a, b) => a - b));
    assert.equal((html.match(/data-draw-tool="/g) || []).length, Pixel.listTools().length);
    const groups = Pixel.toolGroups().map(g => g.group);
    ['select', 'paint', 'fill', 'shape', 'type'].forEach(g => assert.ok(groups.includes(g), 'group ' + g));
    const rank = g => Pixel.TOOL_GROUP_ORDER.indexOf(g);
    assert.deepEqual(groups, groups.slice().sort((a, b) => rank(a) - rank(b)), 'groups follow TOOL_GROUP_ORDER');
});

check('showPanel only touches the dock of that panel and runs onHide/onShow', () => {
    const log = [];
    Pixel.registerPanel({ id: 'test-a', dock: 'inspector', order: 500, labelKey: 'x', build: () => 'a', onShow: (r, prev) => log.push('show-a:' + prev), onHide: () => log.push('hide-a') });
    rt.showPanel('layers');
    assert.equal(rt.activePanels.inspector, 'adjust', 'bottom change leaves the inspector alone');
    assert.equal(rt.activePanels.bottom, 'layers');
    rt.showPanel('history');
    assert.equal(rt.activePanels.bottom, 'history');
    assert.equal(rt.activePanels.inspector, 'adjust');
    rt.adjustments.contrast = 10;
    rt.toggleCompare();
    assert.equal(rt.compareMode, true);
    rt.showPanel('transform');
    assert.equal(rt.compareMode, false, 'adjust onHide leaves compare mode');
    assert.equal(rt.activePanels.inspector, 'transform');
    rt.cropState = { active: true };
    rt.showPanel('test-a');
    assert.equal(rt.cropState, null, 'transform onHide cancels the crop');
    assert.deepEqual(log, ['show-a:transform']);
    rt.showPanel('adjust');
    assert.deepEqual(log, ['show-a:transform', 'hide-a']);
    rt.resetAdjustments();
    rt.showPanel('layers');
    assert.ok(events.includes('panel-changed'));
});

// ---------------------------------------------------------------------------------------------------------------
// shortcuts and commands
// ---------------------------------------------------------------------------------------------------------------

check('shortcut normalisation (Ctrl = Meta, letters lower case, Shift symbols folded)', () => {
    assert.equal(Pixel.normalizeShortcut('Ctrl+Shift+Z'), 'ctrl+shift+z');
    assert.equal(Pixel.normalizeShortcut('Shift+L'), 'shift+l');
    assert.equal(Pixel.normalizeShortcut('L'), 'l');
    assert.equal(Pixel.normalizeShortcut('Ctrl++'), 'ctrl+=');
    assert.equal(Pixel.normalizeShortcut('Ctrl+-'), 'ctrl+-');
    assert.equal(Pixel.normalizeShortcut('Cmd+S'), 'ctrl+s');
    assert.equal(Pixel.normalizeShortcut('Delete'), 'delete');
    assert.equal(Pixel.normalizeShortcut('Del'), 'delete');
    assert.equal(Pixel.normalizeShortcut('?'), 'shift+/');
    assert.equal(Pixel.normalizeShortcut('Shift+['), 'shift+[');
    assert.equal(Pixel.normalizeShortcut('Ctrl+Alt+G'), 'ctrl+alt+g');
    assert.equal(Pixel.normalizeShortcut(''), '');
    assert.equal(Pixel.shortcutFromEvent({ key: 'L', shiftKey: true }), 'shift+l');
    assert.equal(Pixel.shortcutFromEvent({ key: 'l' }), 'l');
    assert.equal(Pixel.shortcutFromEvent({ key: 'Z', ctrlKey: true, shiftKey: true }), 'ctrl+shift+z');
    assert.equal(Pixel.shortcutFromEvent({ key: 'z', metaKey: true }), 'ctrl+z');
    assert.equal(Pixel.shortcutFromEvent({ key: '=', ctrlKey: true }), 'ctrl+=');
    assert.equal(Pixel.shortcutFromEvent({ key: '+', ctrlKey: true, shiftKey: true }), 'ctrl+=');
    assert.equal(Pixel.shortcutFromEvent({ key: '?', shiftKey: true }), 'shift+/');
    assert.equal(Pixel.shortcutFromEvent({ key: '{', shiftKey: true }), 'shift+[');
    assert.equal(Pixel.shortcutFromEvent({ key: 'Shift', shiftKey: true }), '');
    assert.equal(Pixel.shortcutFromEvent({ key: 'F1' }), 'f1');
});

check('integration: every shortcut maps to one command, no module failed to install', () => {
    const byKey = new Map();
    rt.listCommands().forEach(cmd => cmd.shortcuts.forEach(key => byKey.set(key, (byKey.get(key) || []).concat(cmd.id))));
    const clashes = Array.from(byKey).filter(([, ids]) => ids.length > 1).map(([key, ids]) => key + ': ' + ids.join(', '));
    assert.deepEqual(clashes, [], 'shortcut clashes');
    // The window rendered with the built-in installers plus every feature module; none threw.
    ['layers', 'selection', 'brush', 'tools-pro', 'transform', 'generators', 'cutout', 'smart', 'commands', 'ora'].forEach(id => assert.ok(Pixel.listModules().some(m => m.id === id), id));
    assert.equal(consoleErrors.length, 0, 'console.error during install: ' + consoleErrors.join(' | ').slice(0, 300));
});

check('keyboard: tool shortcuts including Shift ones, only for the active window, never in inputs', () => {
    rt.newBlankCanvas(30, 30);
    rt.activeTool = null;
    Pixel.activeInstance = rt.state;
    const press = props => { const e = keyEvent(props); rt.handleKeydown(e); return e; };
    press({ key: 'b' });
    assert.equal(rt.activeTool, 'brush');
    press({ key: 'L', shiftKey: true });
    assert.equal(rt.activeTool, 'line', 'Shift+L is the line tool');
    press({ key: 'l' });
    assert.equal(rt.activeTool, 'lasso');
    press({ key: 'V', shiftKey: true });
    assert.equal(rt.activeTool, 'select-ellipse');
    press({ key: 'v' });
    assert.equal(rt.activeTool, 'select-rect');
    press({ key: 'A', shiftKey: true });
    assert.equal(rt.activeTool, 'arrow');
    press({ key: 'b', altKey: true });
    assert.equal(rt.activeTool, 'arrow', 'Alt+B is not the brush shortcut');
    // inputs and textareas are ignored
    const typed = keyEvent({ key: 'e', target: { closest: sel => (/input/.test(sel) ? {} : null) } });
    rt.handleKeydown(typed);
    assert.equal(rt.activeTool, 'arrow');
    // another Pixel window is the active one
    Pixel.activeInstance = { other: true };
    press({ key: 'e' });
    assert.equal(rt.activeTool, 'arrow', 'inactive window ignores keys');
    Pixel.activeInstance = rt.state;
    // the desktop already handled the key
    const handled = keyEvent({ key: 'e', defaultPrevented: true });
    rt.handleKeydown(handled);
    assert.equal(rt.activeTool, 'arrow');
    press({ key: 'e' });
    assert.equal(rt.activeTool, 'eraser');
    rt.setActiveTool('eraser');
    assert.equal(rt.activeTool, null);
});

check('keyboard: menu shortcuts run through commands (undo/redo, delete, escape, space pan)', () => {
    Pixel.activeInstance = rt.state;
    rt.addLayer();
    const count = rt.layers.length;
    const undo = keyEvent({ key: 'z', ctrlKey: true });
    rt.handleKeydown(undo);
    assert.ok(undo.prevented);
    assert.equal(rt.layers.length, count - 1, 'Ctrl+Z undid the layer');
    rt.handleKeydown(keyEvent({ key: 'Z', ctrlKey: true, shiftKey: true }));
    assert.equal(rt.layers.length, count, 'Ctrl+Shift+Z redid it');
    rt.handleKeydown(keyEvent({ key: 'z', metaKey: true }));
    assert.equal(rt.layers.length, count - 1, 'Meta equals Ctrl');
    rt.redo();
    rt.selection = { type: 'rect', x: 2, y: 2, w: 5, h: 5 };
    rt.handleKeydown(keyEvent({ key: 'Escape' }));
    assert.equal(rt.selection, null, 'Escape deselects');
    const idle = keyEvent({ key: 'Escape' });
    rt.handleKeydown(idle);
    assert.ok(!idle.prevented, 'Escape with nothing to cancel is left alone');
    rt.selection = { type: 'rect', x: 2, y: 2, w: 5, h: 5 };
    rt.handleKeydown(keyEvent({ key: 'Delete' }));
    assert.equal(rt.selection, null, 'Delete erases and deselects');
    rt.handleKeydown(keyEvent({ key: ' ' }));
    assert.equal(rt.spaceHeld, true);
    rt.handleKeyup({ key: ' ' });
    assert.equal(rt.spaceHeld, false);
    const zoom = rt.zoom;
    rt.handleKeydown(keyEvent({ key: '=', ctrlKey: true }));
    assert.ok(rt.zoom > zoom, 'Ctrl+= zooms in');
    rt.handleKeydown(keyEvent({ key: '-', ctrlKey: true }));
    assert.ok(Math.abs(rt.zoom - zoom) < 1e-9, 'Ctrl+- zooms back');
});

check('registerCommand: shortcut dispatch, enabled() fallthrough and later registration wins', () => {
    Pixel.activeInstance = rt.state;
    const log = [];
    Pixel.registerCommand({ id: 'test.cmd', labelKey: 'x', fallback: 'Test command', keywords: ['probe'], group: 'test', shortcut: 'Ctrl+Alt+K', run: r => log.push('first:' + (r === rt)) });
    rt.handleKeydown(keyEvent({ key: 'k', ctrlKey: true, altKey: true }));
    assert.deepEqual(log, ['first:true']);
    Pixel.registerCommand({ id: 'test.cmd2', fallback: 'Second', shortcut: ['Ctrl+Alt+K', 'Ctrl+Alt+J'], run: () => log.push('second'), enabled: () => false });
    rt.handleKeydown(keyEvent({ key: 'k', ctrlKey: true, altKey: true }));
    assert.deepEqual(log, ['first:true', 'first:true'], 'a disabled newer command falls through to the older one');
    Pixel.registerCommand({ id: 'test.cmd2', fallback: 'Second', shortcut: ['Ctrl+Alt+K'], run: () => log.push('second') });
    rt.handleKeydown(keyEvent({ key: 'k', ctrlKey: true, altKey: true }));
    assert.equal(log[log.length - 1], 'second', 'the newer enabled command wins');
    const all = rt.listCommands();
    const cmd = all.find(c => c.id === 'test.cmd');
    assert.equal(cmd.label, 'Test command');
    assert.deepEqual(cmd.keywords, ['probe']);
    assert.deepEqual(cmd.shortcuts, ['ctrl+alt+k']);
    assert.ok(rt.runCommand('test.cmd'));
    assert.equal(rt.runCommand('does.not.exist'), false);
});

check('every tool, panel and menu item is a command', () => {
    const ids = rt.listCommands().map(c => c.id);
    ['tool:brush', 'tool:select-rect', 'tool:text', 'panel:layers', 'panel:adjust', 'panel:stickers', 'menu:file:open', 'menu:edit:undo',
        'menu:image:rotate-cw', 'menu:view:zoom-fit', 'menu:ai:generate', 'edit.cancel', 'edit.delete-selection'].forEach(id => assert.ok(ids.includes(id), id));
    const brush = rt.listCommands().find(c => c.id === 'tool:brush');
    assert.ok(brush.label.startsWith('Tool: '));
    assert.deepEqual(brush.shortcuts, ['b']);
    assert.ok(rt.listCommands().find(c => c.id === 'panel:filters').label.startsWith('Show '));
    const shortcutsCmd = rt.listCommands().find(c => c.id === 'menu:view:shortcuts');
    assert.deepEqual(shortcutsCmd.shortcuts.sort(), ['f1', 'shift+/']);
});

// ---------------------------------------------------------------------------------------------------------------
// menus
// ---------------------------------------------------------------------------------------------------------------

check('window menus come from the registry in the fixed order and keep the literal items', () => {
    assert.ok(Array.isArray(menusSeen));
    const menuIds = menusSeen.map(m => m.id);
    const at = ['file', 'edit', 'image', 'view', 'ai'].map(id => menuIds.indexOf(id));
    assert.ok(at.every(i => i >= 0), 'the built-in menus are present');
    assert.deepEqual(at, at.slice().sort((a, b) => a - b), 'in the fixed order');
    menusSeen.forEach(m => m.items.forEach(function check(item) {
        if (item.type === 'separator') return;
        assert.ok(item.label, 'menu item ' + m.id + '/' + item.id + ' has a label');
        (item.items || []).forEach(check);
    }));
    const edit = menusSeen.find(m => m.id === 'edit');
    assert.equal(edit.items[0].id, 'undo');
    assert.equal(edit.items[0].shortcut, 'Ctrl+Z');
    assert.equal(edit.items[2].type, 'separator');
    const before = rt.layers.length;
    rt.addLayer();
    edit.items[0].action();
    assert.equal(rt.layers.length, before, 'item actions run on the window runtime');
    const image = menusSeen.find(m => m.id === 'image');
    [['rotate-cw', 'redo'], ['rotate-ccw', 'undo'], ['flip-h', 'sort'], ['flip-v', 'sort'], ['crop', 'scissors'], ['resize', 'maximize']].forEach(([id, icon]) => {
        const item = image.items.find(i => i.id === id);
        assert.ok(item, 'image menu keeps ' + id);
        assert.equal(item.icon, icon);
    });
});

check('registerMenuItems: new menus slot into the fixed order, chunks get separators, same id replaces', () => {
    Pixel.registerMenuItems('layer', [{ id: 'new', labelKey: 'x', fallback: 'New', action: () => {} }]);
    Pixel.registerMenuItems('select', [{ id: 'all', fallback: 'All', action: () => {} }]);
    Pixel.registerMenuItems('filter', () => [{ id: 'blur', fallback: 'Blur', action: () => {} }], { order: 5 });
    Pixel.registerMenuItems('generate', [{ id: 'noise', fallback: 'Noise', action: () => {}, enabled: r => !!r.imgWidth }]);
    let replaced = 0;
    Pixel.registerMenuItems('edit', [{ id: 'undo', labelKey: 'pixel.undo', icon: 'undo', shortcut: 'Ctrl+Z', action: () => { replaced++; } }, { id: 'fill', fallback: 'Fill', action: () => {} }]);
    const menus = Pixel.composeMenus(rt);
    const ids = menus.map(m => m.id);
    const order = ['file', 'edit', 'image', 'layer', 'select', 'filter', 'generate', 'view', 'ai'].map(id => ids.indexOf(id));
    assert.ok(order.every(i => i >= 0), 'all nine menus are present');
    assert.deepEqual(order, order.slice().sort((a, b) => a - b), 'in the fixed order');
    const edit = menus.find(m => m.id === 'edit');
    assert.equal(edit.items.filter(i => i.id === 'undo').length, 1);
    edit.items.find(i => i.id === 'undo').action();
    assert.equal(replaced, 1, 'the later registration replaced the built-in undo');
    assert.equal(edit.items[0].id, 'undo', 'and kept its place');
    const sepIndex = edit.items.findIndex((i, n) => n > 0 && i.type === 'separator' && edit.items[n + 1] && edit.items[n + 1].id === 'fill');
    assert.ok(sepIndex > 0, 'a separator precedes the new chunk');
    const gen = menus.find(m => m.id === 'generate').items.find(i => i.id === 'noise');
    assert.equal(typeof gen.disabled, 'function');
    assert.equal(gen.disabled(), false);
    assert.equal(gen.label, 'Noise', 'a missing label is filled from the English fallback');
    const filterItems = menus.find(m => m.id === 'filter').items;
    assert.equal(filterItems.find(i => i.id === 'blur').label, 'Blur');
    assert.ok(filterItems.indexOf(filterItems.find(i => i.id === 'blur')) <= filterItems.findIndex(i => i.id && i.id !== 'blur'), 'order 5 chunk comes first');
    assert.ok(rt.listCommands().some(c => c.id === 'menu:layer:new'));
});

check('translation safety net: English fallback, placeholder map, both together, key when nothing is known', () => {
    // Same contract as the desktop t(key, vars): the key for a missing entry, the 2nd argument is a placeholder map.
    const dict = { 'pixel.known': 'Known {n}', 'pixel.known_double': 'Double {{n}}' };
    const desktopT = (key, vars) => {
        let text = dict[key] || key;
        if (vars) Object.entries(vars).forEach(([name, value]) => { text = text.replaceAll('{{' + name + '}}', () => value).replaceAll('{' + name + '}', () => value); });
        return text;
    };
    const t = Pixel.makeTranslator(desktopT);
    assert.equal(t('pixel.missing', 'English text'), 'English text');
    assert.equal(t('pixel.missing', 'Max {n} layers', { n: 64 }), 'Max 64 layers');
    assert.equal(t('pixel.missing', 'Max {{n}} layers', { n: 64 }), 'Max 64 layers');
    assert.equal(t('pixel.missing', { n: 3 }), 'pixel.missing', 'no English text and no entry: the key');
    assert.equal(t('pixel.known', 'Fallback {n}', { n: 7 }), 'Known 7');
    assert.equal(t('pixel.known', { n: 8 }), 'Known 8');
    assert.equal(t('pixel.known_double', { n: 9 }), 'Double 9');
    assert.equal(t('pixel.known'), 'Known {n}');
    assert.equal(t('pixel.missing', '$& stays literal {x}', { x: '$1' }), '$& stays literal $1');
    assert.equal(Pixel.makeTranslator(undefined)('pixel.x', 'Plain'), 'Plain');
    assert.equal(Pixel.makeTranslator(() => { throw new Error('boom'); })('pixel.x', 'Survives'), 'Survives');
    assert.equal(rt.t('pixel.never_defined', 'Window t {n}', { n: 2 }), 'Window t 2', 'the window runtime uses the wrapped t');
});

// ---------------------------------------------------------------------------------------------------------------
// tools: registry, override, pointer dispatch
// ---------------------------------------------------------------------------------------------------------------

check('registerTool: override inherits metadata, owns the gesture, gates raster tools, builds its own options', () => {
    const calls = [];
    // The brush is a built-in with its own handlers by now: keep its definition and restore it at the end.
    const before = Pixel.getTool('brush');
    const savedBrush = Object.assign({}, before);
    assert.equal(before.group, 'paint');
    assert.equal(before.raster, true);
    Pixel.registerTool({
        id: 'brush',
        onDown: (r, e, p) => calls.push(['down', r === rt, p.x, p.y, p.pressure, p.shift, p.alt, p.ctrl]),
        onMove: (r, e, p) => calls.push(['move', p.x]),
        onUp: (r, e, p) => calls.push(['up', p.y]),
        onHover: (r, e, p) => calls.push(['hover', p.x]),
        options: () => '<i data-test-options>opts</i>',
        wireOptions: (r, bar) => calls.push(['wire', bar === rt.optionsBar]),
        onActivate: () => calls.push(['activate']),
        onDeactivate: () => calls.push(['deactivate'])
    });
    const def = Pixel.getTool('brush');
    assert.equal(def.group, 'paint', 'group inherited');
    assert.equal(def.shortcut, 'B', 'shortcut inherited');
    assert.equal(def.cursor, 'none', 'cursor inherited');
    assert.equal(def.raster, true);
    assert.ok(def.svg.startsWith('<svg'));
    rt.newBlankCanvas(50, 50);
    rt.activeTool = null;
    rt.setActiveTool('brush');
    assert.deepEqual(calls.splice(0), [['activate'], ['wire', true]]);
    assert.ok(rt.optionsBar.innerHTML.includes('data-test-options'));
    assert.equal(rt.getToolCursor(), 'none');
    rt.onCanvasMouseMove(pointerEvent({ clientX: 7.5, clientY: 9.25, buttons: 0 }));
    assert.deepEqual(calls.splice(0), [['hover', 7.5]]);
    const historyBefore = rt.history.length;
    rt.onCanvasMouseDown(pointerEvent({ clientX: 10.5, clientY: 20.25, shiftKey: true }));
    assert.deepEqual(calls.splice(0), [['down', true, 10.5, 20.25, 0.5, true, false, false]]);
    assert.equal(rt.isDrawing, false, 'the legacy brush did not run');
    assert.ok(rt.gesture && rt.gesture.def.id === 'brush');
    rt.onCanvasMouseMove(pointerEvent({ clientX: 11, clientY: 21 }));
    assert.deepEqual(calls.splice(0), [['move', 11]]);
    rt.onCanvasMouseUp(pointerEvent({ clientX: 12, clientY: 22 }));
    assert.deepEqual(calls.splice(0), [['up', 22]]);
    assert.equal(rt.gesture, null);
    assert.equal(rt.history.length, historyBefore, 'the tool commits its own history');
    // a locked layer refuses a raster tool before its handler runs
    rt.activeLayer().lockPixels = true;
    rt.onCanvasMouseDown(pointerEvent({ clientX: 1, clientY: 1 }));
    assert.deepEqual(calls.splice(0), []);
    rt.activeLayer().lockPixels = false;
    rt.setActiveTool('brush');
    assert.deepEqual(calls.splice(0), [['deactivate']]);
    assert.equal(rt.activeTool, null);
    // keys: the tool can consume a key before the command shortcuts
    Pixel.activeInstance = rt.state;
    Pixel.registerTool({ id: 'brush', onKey: (r, e) => { calls.push(['key', e.key]); return e.key === 'q'; } });
    rt.setActiveTool('brush');
    calls.length = 0;
    rt.handleKeydown(keyEvent({ key: 'q' }));
    rt.handleKeydown(keyEvent({ key: 'e' }));
    assert.deepEqual(calls, [['key', 'q'], ['key', 'e']]);
    assert.equal(rt.activeTool, 'eraser', 'a key the tool did not consume still runs its command');
    rt.setActiveTool('eraser');
    // restore the real brush for the following checks
    Pixel.registerTool(savedBrush);
    const restored = Pixel.getTool('brush');
    ['onDown', 'onMove', 'onUp', 'onHover', 'options', 'wireOptions', 'onKey', 'onActivate', 'onDeactivate'].forEach(key => assert.equal(restored[key], savedBrush[key], key + ' restored'));
    assert.equal(restored.group, 'paint');
});

await check('registerTool: new tool in a group, tool-preview overlay hook, pointer info is in document coordinates', () => {
    let overlayCalls = 0;
    Pixel.registerTool({ id: 'test-tool', group: 'retouch', order: 5, svg: '<svg></svg>', shortcut: 'Ctrl+Alt+T', labelKey: 'x', fallback: 'Test tool', raster: false, cursor: () => 'cell', overlay: () => { overlayCalls++; } });
    const groups = Pixel.toolGroups().map(g => g.group);
    ['select', 'paint', 'retouch', 'fill', 'shape', 'type'].forEach(g => assert.ok(groups.includes(g), 'group ' + g));
    const groupRank = g => Pixel.TOOL_GROUP_ORDER.indexOf(g);
    assert.deepEqual(groups, groups.slice().sort((a, b) => groupRank(a) - groupRank(b)), 'groups follow TOOL_GROUP_ORDER');
    const retouch = Pixel.listTools('retouch');
    assert.ok(retouch.some(d => d.id === 'test-tool') && retouch[0].order <= retouch[retouch.length - 1].order, 'the new tool is listed in its group');
    rt.setActiveTool('test-tool');
    assert.equal(rt.getToolCursor(), 'cell');
    rt.onCanvasMouseMove(pointerEvent({ clientX: 3, clientY: 4, buttons: 0 }));
    return new Promise(resolve => setTimeout(resolve, 20)).then(() => {
        assert.ok(overlayCalls > 0, 'the tool overlay is drawn through the compositor');
        const info = rt.pointerInfo(pointerEvent({ clientX: 10.5, clientY: 3.75, pressure: 0.25, buttons: 2, shiftKey: true, ctrlKey: true }));
        assert.deepEqual({ ...info }, { x: 10.5, y: 3.75, pressure: 0.25, buttons: 2, shift: true, alt: false, ctrl: true, pointerType: 'mouse', pointerId: 1 });
        rt.setActiveTool('test-tool');
    });
});

// ---------------------------------------------------------------------------------------------------------------
// overlay compositor
// ---------------------------------------------------------------------------------------------------------------

await check('overlays: built-in order, custom z order, coalesced repaint, clearOverlay keeps the others', async () => {
    const baseline = rt.listOverlays();
    const builtIn = ['selection', 'tool-preview', 'compare', 'cursor'].map(id => baseline.indexOf(id));
    assert.ok(builtIn.every(i => i >= 0), 'the built-in overlays are registered');
    assert.deepEqual(builtIn, builtIn.slice().sort((a, b) => a - b), 'in z order');
    assert.ok(baseline.indexOf('grids') < baseline.indexOf('selection'), 'the grid sits below the selection');
    const order = [];
    const offHigh = rt.registerOverlay('t-high', () => order.push('high'), 95);
    rt.registerOverlay('t-low', () => order.push('low'), 5);
    rt.registerOverlay('t-mid', () => order.push('mid'), 65);
    const withCustom = baseline.slice();
    withCustom.unshift('t-low');
    withCustom.splice(withCustom.indexOf('compare'), 0, 't-mid');
    withCustom.push('t-high');
    assert.deepEqual(rt.listOverlays(), withCustom);
    await new Promise(resolve => setTimeout(resolve, 20));
    order.length = 0;
    rt.requestOverlay(); rt.requestOverlay(); rt.requestOverlay();
    assert.deepEqual(order, [], 'repaint is deferred to the next frame');
    await new Promise(resolve => setTimeout(resolve, 20));
    assert.deepEqual(order, ['low', 'mid', 'high'], 'one repaint, ascending z');
    rt.toolPreview = () => order.push('preview');
    rt.clearOverlay();
    assert.equal(rt.toolPreview, null);
    assert.equal(rt.listOverlays().length, baseline.length + 3, 'clearOverlay leaves the other overlays registered');
    rt.registerOverlay('t-mid', () => order.push('mid2'), 65);
    order.length = 0;
    offHigh();
    await new Promise(resolve => setTimeout(resolve, 20));
    assert.deepEqual(order, ['low', 'mid2']);
    rt.unregisterOverlay('t-low'); rt.unregisterOverlay('t-mid');
    assert.deepEqual(rt.listOverlays(), baseline);
    // a throwing overlay must not take the others down
    let ok = 0;
    rt.registerOverlay('t-bad', () => { throw new Error('bad overlay'); }, 10);
    rt.registerOverlay('t-ok', () => { ok++; }, 11);
    const consoleError = console.error;
    console.error = () => {};
    rt.paintOverlays();
    console.error = consoleError;
    assert.equal(ok, 1);
    rt.unregisterOverlay('t-bad'); rt.unregisterOverlay('t-ok');
});

await check('marching ants: selection overlay keeps requesting frames while a selection exists, stops after deselect', async () => {
    rt.newBlankCanvas(30, 30);
    let draws = 0;
    const real = rt.drawMarchingAnts;
    rt.drawMarchingAnts = ctx => { draws++; return real(ctx); };
    rt.selection = { type: 'rect', x: 2, y: 2, w: 10, h: 10 };
    rt.startMarchingAnts();
    await new Promise(resolve => setTimeout(resolve, 500));
    assert.ok(draws >= 4, 'animated, draws=' + draws);
    rt.deselect();
    await new Promise(resolve => setTimeout(resolve, 100));
    const after = draws;
    await new Promise(resolve => setTimeout(resolve, 150));
    assert.ok(draws - after <= 1, 'stopped after deselect');
    rt.drawMarchingAnts = real;
});

// ---------------------------------------------------------------------------------------------------------------
// runJob and the worker host
// ---------------------------------------------------------------------------------------------------------------

await check('runJob without Worker: sync fallback, progress, errors, key replacement, abort, helpers', async () => {
    assert.equal(typeof Worker, 'undefined');
    const Lib = window.AuraPixelLib;
    Lib.registerOp('test.work', (args, ctx) => { ctx.progress(0.5); return { v: args.n * 2, flag: ctx.cancelled() }; });
    const seen = [];
    const out = await rt.runJob('test.work', { n: 21 }, { onProgress: f => seen.push(f) });
    assert.equal(out.v, 42);
    assert.deepEqual(seen, [0.5]);
    assert.equal((await rt.runJob('core.ping')).pong, true);
    await assert.rejects(rt.runJob('no.such.op', {}), /unknown op/);
    Lib.registerOp('test.throw', () => { throw new Error('op failed'); });
    await assert.rejects(rt.runJob('test.throw', {}), /op failed/);
    // key: a newer job replaces a queued older one
    const first = rt.runJob('test.work', { n: 1 }, { key: 'preview' });
    const second = rt.runJob('test.work', { n: 2 }, { key: 'preview' });
    await assert.rejects(first, err => err.name === 'AbortError');
    assert.equal((await second).v, 4);
    // abort signal
    const controller = new AbortController();
    const aborted = rt.runJob('test.work', { n: 3 }, { signal: controller.signal });
    controller.abort();
    await assert.rejects(aborted, err => err.name === 'AbortError');
    await assert.rejects(rt.runJob('test.work', {}, { signal: controller.signal }), err => err.name === 'AbortError');
    // many queued jobs run one after the other
    const many = await Promise.all([1, 2, 3, 4, 5].map(n => rt.runJob('test.work', { n })));
    assert.deepEqual(many.map(r => r.v), [2, 4, 6, 8, 10]);
    // image helpers
    const canvas = rt.makeCanvas(4, 3);
    const img = rt.imageFromCanvas(canvas);
    assert.equal(img.width, 4); assert.equal(img.height, 3); assert.equal(img.data.length, 48);
    const back = rt.canvasFromImage({ width: 2, height: 2, data: new Uint8ClampedArray(16) });
    assert.equal(back.width, 2);
    const resized = await rt.runJob('core.resize', { image: img, width: 2, height: 2 });
    assert.equal(resized.image.width, 2);
});

await check('runJob with workers: pool of two, abort terminates one and respawns, unproven worker falls back', async () => {
    const Lib = window.AuraPixelLib;
    const made = [];
    class FakeWorker {
        constructor(url) {
            this.url = url; this.terminated = false; made.push(this);
            setTimeout(() => { if (!this.terminated && this.onmessage) this.onmessage({ data: { ready: true } }); }, 1);
        }
        postMessage(msg, transfer) {
            this.lastTransfer = transfer;
            setTimeout(() => {
                if (this.terminated || !this.onmessage) return;
                const res = Lib.handleMessage(msg, { progress: f => this.onmessage({ data: { id: msg.id, progress: f } }) });
                if (res.error !== undefined) this.onmessage({ data: { id: res.id, error: res.error } });
                else this.onmessage({ data: { id: res.id, result: res.result } });
            }, 5);
        }
        terminate() { this.terminated = true; }
    }
    globalThis.Worker = FakeWorker;
    try {
        const { rt: pool } = renderWindow('w-pool');
        Lib.registerOp('test.slow', args => ({ n: args.n }));
        const seen = [];
        const results = await Promise.all([1, 2, 3, 4].map(n => pool.runJob('test.slow', { n }, { onProgress: f => seen.push(f) })));
        assert.deepEqual(results.map(r => r.n), [1, 2, 3, 4]);
        assert.equal(made.length, 2, 'at most two workers');
        assert.ok(made[0].url.includes('/js/desktop/apps/pixel-worker.js?v='), 'versioned worker url: ' + made[0].url);
        assert.equal(pool.jobInfo().usingWorkers, true);
        assert.equal(pool.jobInfo().proven, true);
        // transfer list is handed to a proven worker
        const buffer = new Uint8ClampedArray(8);
        await pool.runJob('test.slow', { n: 9 }, { transfer: [buffer.buffer] });
        assert.ok(made.some(w => w.lastTransfer && w.lastTransfer.length === 1));
        // key replacement of a job that is already running terminates its worker, a new one is spawned later
        const running = pool.runJob('test.slow', { n: 10 }, { key: 'k' });
        await new Promise(resolve => setTimeout(resolve, 1));
        const replacement = pool.runJob('test.slow', { n: 11 }, { key: 'k' });
        await assert.rejects(running, err => err.name === 'AbortError');
        assert.equal((await replacement).n, 11);
        assert.ok(made.some(w => w.terminated), 'the aborted job terminated its worker');
        // dispose rejects what is still pending and terminates everything
        const stuck = pool.runJob('test.slow', { n: 12 });
        window.PixelApp.dispose('w-pool');
        await assert.rejects(stuck, err => err.name === 'AbortError');
        assert.ok(made.every(w => w.terminated));
        await assert.rejects(pool.runJob('test.slow', {}), err => err.name === 'AbortError');
        // a worker that fails before it is ready: everything runs on the main thread
        class BrokenWorker {
            constructor() { setTimeout(() => { if (this.onerror) this.onerror({ message: 'blocked', preventDefault() {} }); }, 1); }
            postMessage() {}
            terminate() { this.terminated = true; }
        }
        globalThis.Worker = BrokenWorker;
        const { rt: broken } = renderWindow('w-broken');
        const buf = new Uint8ClampedArray(4);
        const fallback = await broken.runJob('test.slow', { n: 5, buf }, { transfer: [buf.buffer] });
        assert.equal(fallback.n, 5);
        assert.equal(buf.buffer.byteLength, 4, 'buffers are not transferred to a worker that never proved itself');
        assert.equal(broken.jobInfo().usingWorkers, false);
        assert.equal((await broken.runJob('test.slow', { n: 6 })).n, 6);
        window.PixelApp.dispose('w-broken');
    } finally {
        delete globalThis.Worker;
    }
});

// ---------------------------------------------------------------------------------------------------------------
// parameter controls
// ---------------------------------------------------------------------------------------------------------------

check('param controls: defaults, markup for every type, escaping and showIf', () => {
    const schema = [
        { key: 'amount', type: 'range', labelKey: 'x.amount', fallback: 'Amount <b>', default: 40, min: 0, max: 100, step: 5, unit: '%' },
        { key: 'count', type: 'int', fallback: 'Count', min: 1, max: 9 },
        { key: 'mode', type: 'select', fallback: 'Mode', default: 'b', options: [{ value: 'a', fallback: 'A' }, { value: 'b', fallback: 'B & C' }] },
        { key: 'shape', type: 'segmented', fallback: 'Shape', options: [{ value: 1, fallback: 'One' }, { value: 2, fallback: 'Two' }] },
        { key: 'soft', type: 'bool', fallback: 'Soft', default: true },
        { key: 'tint', type: 'color', fallback: 'Tint', default: '#336699' },
        { key: 'dir', type: 'angle', fallback: 'Direction', default: 45 },
        { key: 'seed', type: 'seed', fallback: 'Seed' },
        { key: 'centre', type: 'point', fallback: 'Centre', default: { x: 0.25, y: 0.75 } },
        { key: 'ramp', type: 'gradient', fallback: 'Gradient', default: 'sunset' },
        { key: 'curve', type: 'custom', fallback: 'Curve', render: v => '<canvas data-curve></canvas>', wire: () => {} },
        { key: 'extra', type: 'range', fallback: 'Extra', min: 0, max: 1, step: 0.1, showIf: v => v.mode === 'a' }
    ];
    const values = rt.defaultParams(schema);
    assert.equal(values.amount, 40);
    assert.equal(values.count, 1);
    assert.equal(values.mode, 'b');
    assert.equal(values.shape, 1);
    assert.equal(values.soft, true);
    assert.equal(values.tint, '#336699');
    assert.equal(values.dir, 45);
    assert.equal(values.seed, 1);
    assert.deepEqual({ ...values.centre }, { x: 0.25, y: 0.75 });
    assert.equal(values.curve, undefined);
    assert.ok(Array.isArray(values.ramp) && values.ramp.length >= 2 && values.ramp[0].color === '#1a1040', 'preset id resolves to a stops copy');
    const schemaCopy = JSON.stringify(schema.map(s => s.default));
    values.centre.x = 0.9;
    values.ramp[0].color = '#000000';
    assert.equal(JSON.stringify(schema.map(s => s.default)), schemaCopy, 'mutating values never touches the schema');
    assert.equal(window.AuraPixelLib.gradientPreset('sunset').stops[0].color, '#1a1040');
    const html = rt.buildParamControls(schema, values, { idPrefix: 'tst' });
    ['range', 'int', 'select', 'segmented', 'bool', 'color', 'angle', 'seed', 'point', 'gradient', 'custom'].forEach(type => assert.ok(html.includes('data-param-type="' + type + '"'), type));
    ['data-param="amount"', 'id="tst-amount"', 'type="range"', 'data-param-number', 'data-param-seg="1"', 'role="switch"', 'data-param-swatch', 'data-param-native', 'data-param-use="primary"',
        'data-param-dial', 'data-param-dice', 'data-param-pad', 'data-param-preset', 'data-param-add-stop', 'data-stop-color', 'data-curve'].forEach(s => assert.ok(html.includes(s), s));
    assert.ok(html.includes('Amount &lt;b&gt;'), 'labels are escaped');
    assert.ok(html.includes('B &amp; C'), 'option labels are escaped');
    assert.ok(/data-param="extra"[^>]*hidden/.test(html), 'showIf false renders hidden');
    assert.ok(/<option value="1" selected>B/.test(html), 'select marks the current value');
    values.mode = 'a';
    assert.ok(!/data-param="extra"[^>]*hidden/.test(rt.buildParamControls(schema, values)), 'showIf true renders visible');
    assert.equal(Pixel.paramVisible(schema[11], { mode: 'a' }), true);
    assert.equal(Pixel.paramVisible(schema[0], {}), true);
    // wiring without matching DOM must not throw and returns a handle
    const root = makeEl('div');
    const handle = rt.wireParamControls(root, schema, values, () => {});
    assert.equal(typeof handle.set, 'function');
    assert.equal(typeof handle.dispose, 'function');
    handle.dispose();
    assert.equal(rt.wireParamControls(null, schema, values), null);
});

// ---------------------------------------------------------------------------------------------------------------
// override by id in a fresh window, dispose
// ---------------------------------------------------------------------------------------------------------------

check('registerPanel override: later registration replaces the built-in and keeps dock and order', () => {
    const calls = [];
    Pixel.registerPanel({ id: 'layers', build: () => '<b data-custom-layers>mine</b>', mount: (r, el) => calls.push(['mount', !!el]), onShow: () => calls.push(['show']) });
    const def = Pixel.getPanel('layers');
    assert.equal(def.dock, 'bottom');
    assert.equal(def.order, 10);
    assert.equal(def.labelKey, 'pixel.layers');
    const { host, rt: win } = renderWindow('w-override');
    assert.ok(host.innerHTML.includes('data-custom-layers'));
    assert.ok(!host.innerHTML.includes('data-layer-list'), 'the built-in markup is gone');
    assert.deepEqual(calls, [['mount', true], ['show']]);
    assert.equal(win.activePanels.bottom, 'layers');
    assert.ok(host1.innerHTML.includes('data-layer-list'), 'windows rendered earlier keep their own markup');
    Pixel.registerPanel({ id: 'test-bottom', dock: 'bottom', order: 5, build: () => 'x', labelKey: 'y', fallback: 'T' });
    assert.deepEqual(Pixel.listPanels('bottom').map(p => p.id), ['test-bottom', 'layers', 'history']);
    window.PixelApp.dispose('w-override');
});

check('two windows: only the active one reacts and dispose is clean', () => {
    const { rt: other } = renderWindow('w-second');
    assert.equal(Pixel.activeInstance, other.state, 'a new window becomes the active one');
    rt.activeTool = null;
    rt.handleKeydown(keyEvent({ key: 'b' }));
    assert.equal(rt.activeTool, null, 'the first window ignores the key');
    other.handleKeydown(keyEvent({ key: 'b' }));
    assert.equal(other.activeTool, 'brush');
    rt.markActiveWindow();
    other.handleKeydown(keyEvent({ key: 'e' }));
    assert.equal(other.activeTool, 'brush', 'the second one no longer reacts');
    rt.handleKeydown(keyEvent({ key: 'e' }));
    assert.equal(rt.activeTool, 'eraser');
    rt.setActiveTool('eraser');
    window.PixelApp.dispose('w-second');
    assert.equal(other.state.disposed, true);
});

await Promise.all(pending);
await new Promise(r => setTimeout(r, 30));

// dispose the main window last
window.PixelApp.dispose('w1');
await new Promise(r => setTimeout(r, 10));

console.log(results.join('\n'));
if (results.some(r => r.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-core: ' + results.length + ' checks passed');
