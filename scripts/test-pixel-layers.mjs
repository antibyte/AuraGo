// WP1 layers dock: pure helpers, layer operations (runtime.layerOps), the Layer menu and its shortcuts, and the dock markup.
// Runs the real Pixel modules against the fake DOM of pixel-test-dom.mjs; only the module "layers" is judged here.
import assert from 'node:assert/strict';
import { installFakeDom, makeEl, loadPixelApp } from './pixel-test-dom.mjs';

installFakeDom();
const Pixel = loadPixelApp();
const H = Pixel.layerHelpers;

const runtimes = [];
Pixel.registerModule('test-capture-layers', r => { runtimes.push(r); });
const notes = [];
function renderWindow(id) {
    const host = makeEl('div');
    window.PixelApp.render(host, id, {
        esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
        t: (k, f) => f || k, api: async () => ({}), iconMarkup: () => '', notify: n => notes.push(n),
        setWindowMenus: () => {}, readonly: false
    });
    return { host, rt: runtimes[runtimes.length - 1] };
}

const results = [];
function check(name, fn) {
    try {
        fn();
        results.push('ok   ' + name);
    } catch (err) {
        results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    }
}
const keyEvent = props => {
    const e = Object.assign({ key: '', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: { closest: () => null } }, props);
    e.preventDefault = () => { e.defaultPrevented = true; };
    return e;
};

// A layer canvas that reports a filled rectangle as its alpha channel and records what is drawn into it.
function rectCanvas(w, h, rect) {
    const data = new Uint8ClampedArray(w * h * 4);
    if (rect) {
        for (let y = rect.y; y < rect.y + rect.h; y++) {
            for (let x = rect.x; x < rect.x + rect.w; x++) data[(y * w + x) * 4 + 3] = 255;
        }
    }
    const calls = [];
    const ctx = new Proxy({ calls }, {
        get(t, p) {
            if (p in t) return t[p];
            if (p === 'getImageData') return () => ({ data, width: w, height: h });
            return (...args) => { calls.push([String(p), ...args.map(a => (typeof a === 'number' ? a : null))]); };
        },
        set(t, p, v) { t[p] = v; return true; }
    });
    return { width: w, height: h, calls, getContext: () => ctx };
}

const { rt, host } = renderWindow('layers-w1');
rt.newBlankCanvas(100, 80);
const ops = rt.layerOps;
const names = () => rt.layers.map(l => l.name);
const lastLabel = () => rt.history[rt.historyIdx].label;
const lastNote = () => notes[notes.length - 1] || {};
// The history is capped (MAX_HISTORY), so entries are counted at the source.
const pushed = [];
const realPush = rt.pushHistory;
rt.pushHistory = function pushHistorySpy(label) { pushed.push(label); return realPush.call(this, label); };
// A new document: one background layer, empty history.
function fresh() {
    rt.newBlankCanvas(100, 80);
    rt.deselect();
    pushed.length = 0;
    return rt.layers[0];
}

// ---------------------------------------------------------------------------------------------------------------
// pure helpers
// ---------------------------------------------------------------------------------------------------------------

check('fitRect keeps the aspect ratio inside the thumbnail box and centres it', () => {
    assert.deepEqual(H.fitRect(1000, 500, 44, 32), { x: 0, y: 5, w: 44, h: 22 });
    assert.deepEqual(H.fitRect(500, 1000, 44, 32), { x: 14, y: 0, w: 16, h: 32 });
    assert.deepEqual(H.fitRect(0, 0, 44, 32), { x: 0, y: 0, w: 44, h: 32 });
    const tiny = H.fitRect(8192, 1, 44, 32);
    assert.ok(tiny.w === 44 && tiny.h >= 1, 'never collapses to zero');
});

check('dropSlot counts the rows above the pointer and maps to a stack position', () => {
    assert.equal(H.dropSlot([10, 30, 50], 5), 0);
    assert.equal(H.dropSlot([10, 30, 50], 31), 2);
    assert.equal(H.dropSlot([10, 30, 50], 99), 3);
    assert.equal(H.dropSlot([], 4), 0);
});

check('contentBounds and alignDelta', () => {
    const w = 10, h = 8, data = new Uint8ClampedArray(w * h * 4);
    assert.equal(H.contentBounds(data, w, h), null);
    [[2, 3], [5, 3], [4, 6]].forEach(([x, y]) => { data[(y * w + x) * 4 + 3] = 255; });
    const b = H.contentBounds(data, w, h);
    assert.deepEqual(b, { x: 2, y: 3, w: 4, h: 4 });
    const t = { x: 0, y: 0, w: 10, h: 8 };
    assert.deepEqual(H.alignDelta('left', b, t), { dx: -2, dy: 0 });
    assert.deepEqual(H.alignDelta('right', b, t), { dx: 4, dy: 0 });
    assert.deepEqual(H.alignDelta('hcenter', b, t), { dx: 1, dy: 0 });
    assert.deepEqual(H.alignDelta('top', b, t), { dx: 0, dy: -3 });
    assert.deepEqual(H.alignDelta('bottom', b, t), { dx: 0, dy: 1 });
    assert.deepEqual(H.alignDelta('vcenter', b, t), { dx: 0, dy: -1 });
    assert.deepEqual(H.alignDelta('nope', b, t), { dx: 0, dy: 0 });
});

check('uniqueName, fileSlug and fmt', () => {
    assert.equal(H.uniqueName('Layer', ['Background'], 2), 'Layer 2');
    assert.equal(H.uniqueName('Layer', ['Background', 'Layer 2', 'Layer 3'], 2), 'Layer 4');
    assert.equal(H.fileSlug('a/b:c*d?'), 'a_b_c_d_');
    assert.equal(H.fileSlug('   '), 'layer');
    assert.equal(H.fmt('{name} copy', { name: 'X' }), 'X copy');
    assert.equal(H.fmt('keep {n}', {}), 'keep {n}');
});

// ---------------------------------------------------------------------------------------------------------------
// registration
// ---------------------------------------------------------------------------------------------------------------

check('the module and panel are registered: bottom dock, order 10, markup hooks', () => {
    assert.ok(Pixel.listModules().some(m => m.id === 'layers'));
    const def = Pixel.getPanel('layers');
    assert.equal(def.dock, 'bottom');
    assert.equal(def.order, 10);
    assert.equal(def.labelKey, 'pixel.layers');
    const html = host.innerHTML;
    ['data-layer-list', 'data-layer-rows', 'data-layer-drop', 'data-layer-blend', 'data-layer-opacity-range', 'data-layer-opacity-number',
        'data-layer-lock="pixels"', 'data-layer-lock="alpha"', 'data-layer-lock="position"', 'data-layer-lock="clip"'].forEach(s => assert.ok(html.includes(s), s));
    ['new', 'copy', 'duplicate', 'mask', 'delete', 'merge', 'more'].forEach(a => assert.ok(html.includes('data-layer-act="' + a + '"'), a));
    const blend = html.slice(html.indexOf('data-layer-blend'), html.indexOf('</select>', html.indexOf('data-layer-blend')));
    assert.equal((blend.match(/<option value=/g) || []).length, Pixel.BLEND_MODES.length);
    assert.ok(blend.includes('value="color-dodge"') && blend.includes('>Color dodge<'));
    assert.ok(rt.layersPanel, 'the panel was mounted');
    const realSync = rt.layersPanel.sync;
    let synced = 0;
    rt.layersPanel.sync = () => { synced++; };
    rt.refreshLayerPanel();
    rt.layersPanel.sync = realSync;
    assert.equal(synced, 1, 'refreshLayerPanel drives the new dock');
});

function walk(items, out = []) {
    items.forEach(item => {
        if (item.type === 'separator' || item.hidden) return;
        if (item.items) walk(item.items, out);
        else out.push(item);
    });
    return out;
}
const layerMenu = () => Pixel.composeMenus(rt).find(m => m.id === 'layer');

check('the Layer menu has every operation of the brief', () => {
    const menu = layerMenu();
    assert.ok(menu, 'layer menu exists');
    const ids = walk(menu.items).map(i => i.id);
    ['new-layer', 'new-fill-layer', 'via-copy', 'duplicate', 'delete', 'rename', 'properties', 'merge-down', 'merge-visible', 'flatten', 'rasterize',
        'mask-reveal', 'mask-hide', 'mask-from-selection', 'mask-invert', 'mask-apply', 'mask-delete', 'mask-disable', 'mask-to-selection',
        'select-from-layer', 'clip', 'fill-primary', 'fill-secondary', 'clear', 'bring-forward', 'send-backward', 'select-above', 'select-below',
        'align-left', 'align-hcenter', 'align-right', 'align-top', 'align-vcenter', 'align-bottom', 'place-image', 'export-layer'].forEach(id => assert.ok(ids.includes(id), id));
    // mask-enable is hidden while the mask is enabled (and the other way round)
    assert.ok(!ids.includes('mask-enable'));
    ids.forEach(id => assert.ok(!/^\s*$/.test(id)));
    walk(menu.items).forEach(i => { assert.ok(i.label && i.labelKey, 'label and labelKey on ' + i.id); });
});

check('shortcuts are registered as commands', () => {
    const cmds = rt.listCommands();
    const has = (id, key) => { const c = cmds.find(x => x.id === id); assert.ok(c, id); assert.ok(c.shortcuts.includes(key), id + ' ' + key + ' in ' + c.shortcuts); };
    has('menu:layer:via-copy', 'ctrl+j');
    has('menu:layer:merge-down', 'ctrl+e');
    has('menu:layer:merge-visible', 'ctrl+shift+e');
    has('menu:layer:clip', 'ctrl+alt+g');
    has('menu:layer:arrange/bring-forward', 'ctrl+]');
    has('menu:layer:arrange/send-backward', 'ctrl+[');
    has('menu:layer:arrange/select-above', 'alt+]');
    has('menu:layer:arrange/select-below', 'alt+[');
    has('menu:layer:arrange/bring-forward', 'ctrl+shift+arrowup');
    has('menu:layer:arrange/select-below', 'alt+arrowdown');
    has('menu:layer:rename', 'f2');
});

// ---------------------------------------------------------------------------------------------------------------
// operations
// ---------------------------------------------------------------------------------------------------------------

check('newLayer inserts above the active layer, names it, selects it and records history', () => {
    const bg = fresh();
    const layer = ops.newLayer();
    assert.deepEqual(names(), [bg.name, 'Layer 2']);
    assert.equal(rt.activeLayer(), layer);
    assert.equal(layer.canvas.width, 100);
    assert.equal(lastLabel(), 'Add Layer');
    rt.activeLayerIdx = 0;
    const between = ops.newLayer();
    assert.equal(rt.layers.indexOf(between), 1, 'goes directly above the active layer');
    assert.deepEqual(names(), [bg.name, 'Layer 3', 'Layer 2']);
    const fill = ops.newFillLayer();
    assert.ok(fill.name.startsWith('Fill #'));
    assert.equal(lastLabel(), 'New fill layer');
});

check('shortcut Ctrl+J runs layer via copy; Ctrl+E merges down', () => {
    fresh();
    ops.newLayer();
    Pixel.activeInstance = rt.state;
    const before = rt.layers.length;
    rt.handleKeydown(keyEvent({ key: 'j', ctrlKey: true }));
    assert.equal(rt.layers.length, before + 1);
    assert.equal(lastLabel(), 'Layer via copy');
    assert.ok(rt.activeLayer().name.endsWith(' copy'));
    rt.handleKeydown(keyEvent({ key: 'e', ctrlKey: true }));
    assert.equal(rt.layers.length, before);
    assert.equal(lastLabel(), 'Merge Down');
});

check('MAX_LAYERS is respected and reported', () => {
    fresh();
    while (rt.layers.length < rt.MAX_LAYERS) assert.ok(ops.newLayer());
    const count = rt.layers.length;
    assert.equal(ops.newLayer(), null);
    assert.equal(ops.duplicate(), null);
    assert.equal(rt.layers.length, count);
    assert.equal(lastNote().type, 'error');
    assert.ok(lastNote().message.includes(String(rt.MAX_LAYERS)));
    while (rt.layers.length > 3) assert.ok(ops.remove(rt.layers[rt.layers.length - 1].id));
});

check('the last layer cannot be deleted; deleting selects the layer below', () => {
    rt.activeLayerIdx = 2;
    const below = rt.layers[1];
    assert.ok(ops.remove());
    assert.equal(rt.layers.length, 2);
    assert.equal(rt.activeLayer(), below);
    assert.equal(lastLabel(), 'Delete Layer');
    assert.ok(ops.remove());
    assert.equal(rt.layers.length, 1);
    assert.equal(ops.remove(), false);
    assert.equal(rt.layers.length, 1);
    assert.equal(lastNote().type, 'info');
});

check('duplicate copies properties, mask and clipping above the source', () => {
    const base = fresh();
    const src = ops.newLayer({ name: 'Src' });
    Object.assign(src, { opacity: 0.4, blend: 'multiply', clip: true, lockAlpha: true });
    src.maskCanvas = rt.makeCanvas(100, 80);
    const copy = ops.duplicate(src.id);
    assert.equal(rt.layers.indexOf(copy), rt.layers.indexOf(src) + 1);
    assert.equal(copy.name, 'Src copy');
    assert.deepEqual([copy.opacity, copy.blend, copy.clip, copy.lockAlpha], [0.4, 'multiply', true, true]);
    assert.ok(copy.maskCanvas && copy.maskCanvas !== src.maskCanvas, 'the mask is a copy');
    assert.notEqual(copy.canvas, src.canvas);
    assert.equal(lastLabel(), 'Duplicate');
    assert.equal(rt.layers[0], base);
});

check('moveTo / moveBy reorder and keep the active layer', () => {
    const ids = () => rt.layers.map(l => l.id);
    fresh();
    while (rt.layers.length < 4) ops.newLayer();
    const [a, b, c, d] = ids();
    rt.activeLayerIdx = 1;
    const active = rt.activeLayer();
    assert.ok(ops.moveTo(b, 3));
    assert.deepEqual(ids(), [a, c, d, b]);
    assert.equal(rt.activeLayer(), active);
    assert.equal(ops.moveTo(b, 3), false, 'same position is a no-op');
    assert.ok(ops.moveBy(-1, b));
    assert.deepEqual(ids(), [a, c, b, d]);
    assert.ok(ops.toFront(a));
    assert.equal(rt.layers[rt.layers.length - 1].id, a);
    assert.ok(ops.toBack(a));
    assert.equal(rt.layers[0].id, a);
    assert.equal(lastLabel(), 'Send to back');
    assert.ok(ops.moveBy(99, c));
    assert.equal(rt.layers[rt.layers.length - 1].id, c, 'clamped to the top');
    assert.equal(rt.layers.length, 4);
    assert.ok(d);
});

check('selecting: select, selectRelative and mask edit mode', () => {
    fresh();
    ops.newLayer();
    ops.select(rt.layers[0].id);
    assert.equal(rt.activeLayerIdx, 0);
    assert.ok(ops.selectRelative(1));
    assert.equal(rt.activeLayerIdx, 1);
    assert.equal(ops.selectRelative(-5), false);
    assert.equal(ops.select(123456), false);
    ops.select(rt.layers[0].id);
    ops.addMask({ mode: 'reveal' });
    assert.equal(rt.editMaskMode, true);
    ops.select(rt.layers[1].id);
    assert.equal(rt.editMaskMode, false, 'switching layers leaves mask edit');
});

check('properties: opacity, blend, name, locks and clipping go through history and undo', () => {
    fresh();
    const layer = ops.newLayer();
    pushed.length = 0;
    assert.ok(ops.setProps(layer.id, { opacity: 0.25, blend: 'screen', name: '  Sky  ' }, 'Layer properties'));
    assert.equal(pushed.length, 1);
    assert.deepEqual([layer.opacity, layer.blend, layer.name], [0.25, 'screen', 'Sky']);
    assert.equal(ops.setProps(layer.id, { blend: 'bogus', name: '   ', opacity: NaN }), false, 'invalid values are ignored');
    assert.equal(ops.setProps(layer.id, { opacity: 5 }), true);
    assert.equal(layer.opacity, 1, 'opacity is clamped');
    assert.equal(pushed.length, 2);
    assert.equal(ops.setOpacity(layer.id, 0.5), true);
    assert.equal(pushed.length, 2, 'live opacity does not add history');
    assert.ok(ops.toggleLock(layer.id, 'pixels'));
    assert.equal(layer.lockPixels, true);
    assert.equal(rt.commitLayerPixels(layer, rt.makeCanvas(100, 80), { label: 'x', history: false }), false, 'locked pixels refuse edits');
    assert.ok(ops.toggleLock(layer.id, 'pixels'));
    assert.ok(ops.toggleLock(layer.id, 'position') && ops.toggleLock(layer.id, 'alpha'));
    assert.deepEqual([layer.lockPosition, layer.lockAlpha], [true, true]);
    assert.ok(ops.toggleClip(layer.id));
    assert.equal(layer.clip, true);
    assert.equal(ops.toggleClip(rt.layers[0].id), false, 'the bottom layer cannot be clipped');
    rt.undo();
    assert.equal(rt.layerById(layer.id).clip, false);
    while (rt.historyIdx > 1) rt.undo();
    assert.equal(rt.layerById(layer.id).name, 'Layer 2', 'undo walks back to the original name');
    assert.equal(rt.layerById(layer.id).opacity, 1);
    while (rt.historyIdx < rt.history.length - 1) rt.redo();
    assert.equal(rt.layerById(layer.id).clip, true);
});

check('visibility: toggle and solo (Alt+click on the eye)', () => {
    fresh();
    ops.newLayer();
    ops.newLayer();
    const [a, b, c] = rt.layers;
    ops.toggleVisible(b.id);
    assert.equal(b.visible, false);
    assert.equal(lastLabel(), 'Hide layer');
    ops.toggleVisible(b.id);
    assert.equal(b.visible, true);
    ops.solo(c.id);
    assert.deepEqual(rt.layers.map(l => l.visible), rt.layers.map(l => l === c));
    assert.equal(lastLabel(), 'Show only this layer');
    ops.solo(c.id);
    assert.ok(rt.layers.every(l => l.visible));
    assert.equal(lastLabel(), 'Show all layers');
    assert.ok(a && b);
});

check('merge down bakes the pair into the lower layer; hidden or locked layers refuse', () => {
    fresh();
    while (rt.layers.length < 3) ops.newLayer();
    rt.activeLayerIdx = 2;
    const lower = rt.layers[1], count = rt.layers.length;
    lower.visible = false;
    assert.equal(ops.mergeDown(), false);
    assert.equal(lastNote().type, 'info');
    lower.visible = true;
    lower.lockPixels = true;
    assert.equal(ops.mergeDown(), false);
    lower.lockPixels = false;
    const rev = lower.rev;
    assert.ok(ops.mergeDown());
    assert.equal(rt.layers.length, count - 1);
    assert.equal(rt.activeLayer(), lower);
    assert.ok(lower.rev > rev && lower.opacity === 1 && !lower.maskCanvas);
    rt.activeLayerIdx = 0;
    assert.equal(ops.mergeDown(), false);
});

check('merge visible keeps hidden layers and puts the result at the lowest visible position', () => {
    fresh();
    while (rt.layers.length < 5) ops.newLayer();
    rt.layers.forEach(l => { l.visible = true; });
    rt.layers[2].visible = false;
    rt.layers[0].visible = false;
    const hidden = [rt.layers[0], rt.layers[2]];
    const visibleCount = rt.layers.filter(l => l.visible).length;
    const total = rt.layers.length;
    assert.ok(ops.mergeVisible());
    assert.equal(rt.layers.length, total - visibleCount + 1);
    assert.equal(rt.layers[0], hidden[0]);
    assert.equal(rt.layers[1].name, 'Merged', 'lowest visible layer was at index 1');
    assert.ok(rt.layers.includes(hidden[1]));
    assert.equal(rt.activeLayer(), rt.layers[1]);
    rt.layers.forEach(l => { l.visible = false; });
    rt.layers[0].visible = true;
    assert.equal(ops.mergeVisible(), false, 'one visible layer is nothing to merge');
});

check('flatten leaves one Background layer', () => {
    fresh();
    ops.newLayer();
    ops.newLayer();
    assert.ok(ops.flatten());
    assert.equal(rt.layers.length, 1);
    assert.equal(rt.layers[0].name, 'Background');
    assert.equal(rt.activeLayerIdx, 0);
    assert.equal(ops.flatten(), false, 'a single plain layer is already flat');
    assert.equal(lastLabel(), 'Flatten');
});

check('layer masks: add, invert, disable, apply, delete', () => {
    fresh();
    const layer = rt.activeLayer();
    assert.ok(ops.addMask({ mode: 'reveal' }));
    assert.ok(layer.maskCanvas && layer.maskEnabled && rt.editMaskMode);
    assert.equal(ops.addMask({ mode: 'reveal' }), false, 'only one mask per layer');
    assert.ok(rt.history[rt.historyIdx].layers[0].maskSnap, 'the mask is part of the history entry');
    const maskRev = layer.maskRev;
    assert.ok(ops.invertMask());
    assert.ok(layer.maskRev > maskRev);
    assert.ok(ops.toggleMask(layer.id));
    assert.equal(layer.maskEnabled, false);
    assert.equal(lastLabel(), 'Disable mask');
    assert.ok(ops.toggleMask(layer.id));
    assert.equal(ops.addMask({ mode: 'selection' }), false);
    assert.ok(ops.editMask(layer.id));
    assert.equal(rt.editMaskMode, false, 'toggles the edit mode off');
    layer.lockPixels = true;
    assert.equal(ops.applyMask(), false, 'a locked layer keeps its pixels');
    layer.lockPixels = false;
    const rev = layer.rev;
    assert.ok(ops.applyMask());
    assert.equal(layer.maskCanvas, null);
    assert.ok(layer.rev > rev, 'applying the mask changed the pixels');
    assert.equal(ops.invertMask(), false);
    assert.equal(lastNote().message, 'This layer has no mask.');
    ops.addMask({ mode: 'hide' });
    assert.ok(ops.deleteMask());
    assert.equal(layer.maskCanvas, null);
    assert.equal(lastLabel(), 'Delete mask');
});

check('masks from a selection need a selection', () => {
    fresh();
    assert.equal(ops.addMask({ mode: 'selection-hide' }), false);
    assert.equal(lastNote().message, 'Make a selection first.');
    rt.selection = { type: 'rect', x: 10, y: 10, w: 30, h: 20 };
    assert.ok(ops.addMask({ mode: 'selection' }));
    assert.ok(rt.activeLayer().maskCanvas);
    ops.deleteMask();
    assert.ok(ops.addMask({ mode: 'selection-hide' }));
    ops.deleteMask();
    rt.deselect();
});

check('layer via copy: selection only, cut and refusals', () => {
    fresh();
    const src = rt.activeLayer();
    src.canvas = rectCanvas(100, 80, { x: 0, y: 0, w: 100, h: 80 });
    rt.deselect();
    assert.equal(ops.viaCopy({ cut: true }), null, 'cut needs a selection');
    rt.selection = { type: 'rect', x: 10, y: 10, w: 20, h: 20 };
    const copy = ops.viaCopy();
    assert.ok(copy);
    assert.equal(rt.layers.indexOf(copy), rt.layers.indexOf(src) + 1);
    assert.equal(lastLabel(), 'Layer via copy');
    assert.ok(rt.selection, 'the selection stays');
    const cut = ops.viaCopy({ cut: true });
    assert.ok(cut);
    assert.equal(lastLabel(), 'Layer via cut');
    rt.deselect();
    src.canvas = rt.makeCanvas(100, 80);
});

check('selection from layer alpha, with add / subtract modes', () => {
    fresh();
    const layer = ops.newLayer({ name: 'Shape' });
    layer.canvas = rectCanvas(100, 80, { x: 10, y: 10, w: 20, h: 20 });
    assert.ok(ops.selectionFromLayer(layer.id));
    assert.equal(rt.selection.type, 'mask');
    assert.deepEqual([rt.selection.x, rt.selection.y, rt.selection.w, rt.selection.h], [10, 10, 20, 20]);
    layer.canvas = rectCanvas(100, 80, { x: 20, y: 20, w: 30, h: 30 });
    assert.ok(ops.selectionFromLayer(layer.id, 'add'));
    assert.deepEqual([rt.selection.x, rt.selection.y, rt.selection.w, rt.selection.h], [10, 10, 40, 40]);
    layer.canvas = rectCanvas(100, 80, { x: 10, y: 10, w: 40, h: 5 });
    assert.ok(ops.selectionFromLayer(layer.id, 'subtract'));
    assert.equal(rt.selection.y, 15);
    layer.canvas = rectCanvas(100, 80, null);
    rt.deselect();
    assert.equal(ops.selectionFromLayer(layer.id), false);
    assert.equal(lastNote().message, 'The layer is empty.');
    layer.maskCanvas = rectCanvas(100, 80, { x: 0, y: 0, w: 5, h: 5 });
    assert.ok(ops.selectionFromMask(layer.id));
    assert.deepEqual([rt.selection.w, rt.selection.h], [5, 5]);
    layer.maskCanvas = null;
    rt.deselect();
});

check('fill and clear go through commitLayerPixels (history labels)', () => {
    fresh();
    assert.ok(ops.fill('primary'));
    assert.equal(lastLabel(), 'Fill with primary color');
    assert.ok(ops.fill('secondary'));
    assert.equal(lastLabel(), 'Fill with secondary color');
    assert.ok(ops.fill('clear'));
    assert.equal(lastLabel(), 'Clear layer');
    const layer = rt.activeLayer();
    layer.lockPixels = true;
    const entries = pushed.length;
    assert.equal(ops.fill('primary'), false);
    assert.equal(pushed.length, entries);
    layer.lockPixels = false;
});

check('align moves the content bounds to the canvas or selection edges', () => {
    fresh();
    const layer = rt.activeLayer();
    layer.canvas = rectCanvas(100, 80, { x: 10, y: 10, w: 20, h: 10 });
    const ctx = layer.canvas.getContext('2d');
    assert.ok(ops.align('left'));
    const move = ctx.calls.filter(c => c[0] === 'drawImage').pop();
    assert.deepEqual(move.slice(2), [-10, 0], 'shifted left by the margin');
    assert.equal(lastLabel(), 'Align layer');
    ctx.calls.length = 0;
    rt.selection = { type: 'rect', x: 40, y: 20, w: 40, h: 40 };
    assert.ok(ops.align('hcenter'));
    assert.deepEqual(ctx.calls.filter(c => c[0] === 'drawImage').pop().slice(2), [40, 0], 'centred inside the selection');
    rt.deselect();
    layer.lockPosition = true;
    assert.equal(ops.align('top'), false);
    layer.lockPosition = false;
    layer.canvas = rt.makeCanvas(100, 80);
    assert.equal(ops.align('top'), false, 'an empty layer cannot be aligned');
    assert.equal(lastNote().message, 'The layer is empty.');
});

check('placeImageElement centres and scales large images down only', () => {
    const count = rt.layers.length;
    const layer = ops.placeImageElement({ naturalWidth: 400, naturalHeight: 200 }, 'photo');
    assert.ok(layer);
    assert.equal(layer.name, 'photo');
    assert.equal(rt.layers.length, count + 1);
    const draw = layer.canvas.getContext('2d').log.filter(e => e[0] === 'drawImage');
    assert.equal(draw.length, 1);
    assert.equal(ops.placeImageElement({ naturalWidth: 0, naturalHeight: 0 }), null);
    assert.equal(ops.exportLayer(), true);
});

check('rasterize needs a sticker layer', () => {
    assert.equal(ops.rasterize(), false);
});

// ---------------------------------------------------------------------------------------------------------------
// the dock
// ---------------------------------------------------------------------------------------------------------------

check('rows list the top layer first with badges, mask thumbnails and escaped names', () => {
    const lower = fresh();
    const mid = ops.newLayer({ name: 'A <b>"quoted"</b>' });
    const top = ops.newLayer({ name: 'Top' });
    Object.assign(top, { clip: true, blend: 'multiply', lockPixels: true, visible: false });
    mid.maskCanvas = rt.makeCanvas(100, 80);
    mid.maskEnabled = false;
    const html = rt.layersPanel.rowsHTML();
    const at = id => html.lastIndexOf('<div class="pixel-layer-item', html.indexOf('data-layer-id="' + id + '"'));
    assert.ok(at(top.id) >= 0 && at(top.id) < at(mid.id) && at(mid.id) < at(lower.id), 'top layer first');
    assert.ok(html.includes('A &lt;b&gt;&quot;quoted&quot;&lt;/b&gt;'), 'names are escaped');
    assert.ok(!html.includes('<b>'), 'no raw markup from names');
    const topRow = html.slice(at(top.id), at(mid.id));
    assert.ok(topRow.includes('pixel-layer-clipped'), 'clip rows are indented');
    assert.ok(topRow.includes('is-hidden'));
    assert.ok(topRow.includes('pixel-layer-badge-text') && topRow.includes('Multiply'));
    assert.ok(topRow.includes('pixel-layer-badge'), 'lock badge');
    assert.ok(!topRow.includes('data-layer-mask-thumb'));
    const midRow = html.slice(at(mid.id), at(lower.id));
    assert.ok(midRow.includes('data-layer-mask-thumb') && midRow.includes('pixel-layer-maskthumb disabled'));
    assert.ok(midRow.includes('data-layer-eye') && midRow.includes('data-layer-thumb') && midRow.includes('data-layer-name'));
});

check('the structure key follows names, flags and masks but not opacity or pixels', () => {
    fresh();
    const layer = ops.newLayer();
    const key = rt.layersPanel.structureKey();
    layer.opacity = 0.3;
    ops.setOpacity(layer.id, 0.35);
    rt.markLayerDirty(layer);
    assert.equal(rt.layersPanel.structureKey(), key);
    layer.name = 'Renamed';
    assert.notEqual(rt.layersPanel.structureKey(), key);
    const key2 = rt.layersPanel.structureKey();
    layer.clip = !layer.clip;
    assert.notEqual(rt.layersPanel.structureKey(), key2);
});

check('thumbnails are cached per rev', () => {
    fresh();
    const layer = ops.newLayer();
    const first = rt.layersPanel.thumbnailEntry(layer, false);
    assert.equal(rt.layersPanel.thumbnailEntry(layer, false), first);
    const key = first.key;
    rt.markLayerDirty(layer);
    assert.notEqual(rt.layersPanel.thumbnailEntry(layer, false).key, key);
    assert.ok(first.canvas.width >= 44 && first.canvas.height >= 32);
});

check('refreshLayerPanel and a hidden dock do not throw', () => {
    rt.refreshLayerPanel();
    rt.showPanel('history');
    rt.refreshLayerPanel();
    rt.showPanel('layers');
    ops.properties();
    rt.layersPanel.openMenu(10, 10, null);
    rt.layersPanel.closeMenu();
});

check('menu enabled states follow the stack', () => {
    fresh();
    const items = walk(layerMenu().items);
    const del = items.find(i => i.id === 'delete');
    assert.equal(del.disabled(), true, 'the last layer cannot be deleted');
    assert.equal(items.find(i => i.id === 'merge-down').disabled(), true);
    assert.equal(items.find(i => i.id === 'via-cut').disabled(), true, 'cut needs a selection');
    assert.equal(items.find(i => i.id === 'new-layer').disabled(), false);
    ops.newLayer();
    const after = walk(layerMenu().items);
    assert.equal(after.find(i => i.id === 'delete').disabled(), false);
    assert.equal(after.find(i => i.id === 'merge-down').disabled(), false);
});

// ---------------------------------------------------------------------------------------------------------------
// dock events: the dock is mounted on small hand-made elements so the listeners it registers can be driven directly
// ---------------------------------------------------------------------------------------------------------------

const matchesSel = (node, sel) => sel === node.tag || (sel.startsWith('[') && node.attrs.has(sel.slice(1, -1).split('=')[0]));
function mockEl(tag = 'div', attrs = []) {
    const el = {
        tag, attrs: new Set(attrs), dataset: {}, style: {}, listeners: {}, hidden: false, value: '', disabled: false, parent: null,
        classes: new Set(), top: 0, height: 40, width: 0, offsetTop: 0, offsetHeight: 40, removed: false,
        classList: {
            add: c => el.classes.add(c), remove: c => el.classes.delete(c), contains: c => el.classes.has(c),
            toggle: (c, on) => { if (on === undefined ? !el.classes.has(c) : on) el.classes.add(c); else el.classes.delete(c); }
        },
        addEventListener(type, fn) { (el.listeners[type] = el.listeners[type] || []).push(fn); },
        removeEventListener() {}, setAttribute() {}, focus() {}, select() {}, after() {}, scrollIntoView() {},
        setPointerCapture() {}, releasePointerCapture() {}, remove() { el.removed = true; },
        closest(sel) {
            const parts = sel.split(',').map(s => s.trim());
            for (let node = el; node; node = node.parent) if (parts.some(p => matchesSel(node, p))) return node;
            return null;
        },
        getBoundingClientRect: () => ({ left: 0, top: el.top, width: 200, height: el.height, bottom: el.top + el.height, right: 200 }),
        querySelector: () => null, querySelectorAll: () => [],
        getContext: () => new Proxy({}, { get: (t, p) => (p in t ? t[p] : () => {}), set: (t, p, v) => { t[p] = v; return true; } })
    };
    return el;
}
function fire(el, type, props) {
    const event = Object.assign({ button: 0, pointerId: 1, clientX: 0, clientY: 0, ctrlKey: false, shiftKey: false, altKey: false, metaKey: false, target: el, preventDefault() {}, stopPropagation() {} }, props);
    (el.listeners[type] || []).forEach(fn => fn(event));
}

const dock = (() => {
    const list = mockEl('div', ['data-layer-list']);
    const rows = mockEl('div', ['data-layer-rows']);
    const parts = {
        '[data-layer-list]': list, '[data-layer-rows]': rows, '[data-layer-drop]': mockEl('div'), '[data-layer-blend]': mockEl('select'),
        '[data-layer-opacity-range]': mockEl('input'), '[data-layer-opacity-number]': mockEl('input')
    };
    const lockBtns = ['pixels', 'alpha', 'position', 'clip'].map(kind => { const b = mockEl('button'); b.dataset.layerLock = kind; return b; });
    const footerBtns = ['new', 'copy', 'duplicate', 'mask', 'delete', 'merge', 'more'].map(act => { const b = mockEl('button'); b.dataset.layerAct = act; return b; });
    const section = mockEl('div');
    section.querySelector = sel => parts[sel] || null;
    section.querySelectorAll = sel => (sel === '[data-layer-lock]' ? lockBtns : (sel === '[data-layer-act]' ? footerBtns : []));
    section.closest = () => null;
    const cache = new Map();
    const rowMock = layer => {
        if (!cache.has(layer.id)) {
            const row = mockEl('div', ['data-layer-id']);
            row.dataset.layerId = String(layer.id);
            const name = mockEl('span', ['data-layer-name']);
            name.parent = row;
            row.querySelector = sel => {
                if (sel === '[data-layer-name]') return name;
                if (sel === '[data-layer-thumb]') return mockEl('canvas');
                if (sel === '[data-layer-mask-canvas]') return rt.layerById(layer.id) && rt.layerById(layer.id).maskCanvas ? mockEl('canvas') : null;
                return null;
            };
            row.name = name;
            cache.set(layer.id, row);
        }
        return cache.get(layer.id);
    };
    // Rows are listed top layer first, 40px each.
    const rowList = () => rt.layers.slice().reverse().map((layer, i) => { const row = rowMock(layer); row.top = row.offsetTop = i * 40; return row; });
    rows.querySelectorAll = () => rowList();
    rows.querySelector = sel => rowList().find(row => sel === '[data-layer-id="' + row.dataset.layerId + '"]') || null;
    const targetOf = (kind, layer) => {
        const row = rowMock(layer);
        const attr = { eye: 'data-layer-eye', mask: 'data-layer-mask-thumb', thumb: 'data-layer-thumb-wrap', name: 'data-layer-name' }[kind];
        if (!attr) return row;
        const el = mockEl('span', [attr]);
        el.parent = row;
        return el;
    };
    return { list, rows, parts, lockBtns, footerBtns, section, targetOf, rowMock };
})();

check('dock events: the panel mounts on plain elements', () => {
    fresh();
    rt.layersPanel.dispose();
    Pixel.getPanel('layers').mount(rt, dock.section);
    assert.ok(rt.layersPanel);
    ['click', 'dblclick', 'contextmenu', 'keydown', 'pointerdown', 'pointermove', 'pointerup'].forEach(type => assert.ok(dock.list.listeners[type], type));
    assert.ok(dock.parts['[data-layer-blend]'].listeners.change && dock.parts['[data-layer-opacity-range]'].listeners.input);
    assert.ok(dock.rows.innerHTML.includes('data-layer-id'), 'rows are rendered on mount');
});

check('dock events: clicks on the eye, thumbnails and rows', () => {
    fresh();
    const a = ops.newLayer(), b = ops.newLayer();
    fire(dock.list, 'click', { target: dock.targetOf('row', a) });
    assert.equal(rt.activeLayer(), a, 'a click selects the row');
    fire(dock.list, 'click', { target: dock.targetOf('eye', b) });
    assert.equal(b.visible, false);
    assert.equal(lastLabel(), 'Hide layer');
    assert.equal(rt.activeLayer(), a, 'the eye does not change the active layer');
    fire(dock.list, 'click', { target: dock.targetOf('eye', b), altKey: true });
    assert.ok(b.visible && !a.visible && !rt.layers[0].visible, 'Alt+click shows only this layer');
    fire(dock.list, 'click', { target: dock.targetOf('eye', b), altKey: true });
    assert.ok(rt.layers.every(l => l.visible), 'Alt+click again shows all');
    // Ctrl+click on the thumbnail: selection from the layer alpha (Shift adds, Alt subtracts)
    a.canvas = rectCanvas(100, 80, { x: 5, y: 5, w: 10, h: 10 });
    fire(dock.list, 'click', { target: dock.targetOf('thumb', a), ctrlKey: true });
    assert.deepEqual([rt.selection.x, rt.selection.w], [5, 10]);
    assert.equal(rt.activeLayer(), a);
    b.canvas = rectCanvas(100, 80, { x: 20, y: 5, w: 10, h: 10 });
    fire(dock.list, 'click', { target: dock.targetOf('thumb', b), ctrlKey: true, shiftKey: true });
    assert.deepEqual([rt.selection.x, rt.selection.w], [5, 25], 'Ctrl+Shift adds');
    assert.equal(rt.activeLayer(), a, 'selecting from a layer keeps the active layer');
    fire(dock.list, 'click', { target: dock.targetOf('thumb', b), ctrlKey: true, altKey: true });
    assert.deepEqual([rt.selection.x, rt.selection.w], [5, 10], 'Ctrl+Alt subtracts');
    rt.deselect();
});

check('dock events: the mask thumbnail edits, disables and selects', () => {
    fresh();
    const a = ops.newLayer(), b = ops.newLayer();
    rt.activeLayerIdx = rt.layers.indexOf(a);
    b.maskCanvas = rectCanvas(100, 80, { x: 0, y: 0, w: 8, h: 8 });
    fire(dock.list, 'click', { target: dock.targetOf('mask', b) });
    assert.equal(rt.activeLayer(), b);
    assert.equal(rt.editMaskMode, true, 'click = edit the mask');
    fire(dock.list, 'click', { target: dock.targetOf('mask', b) });
    assert.equal(rt.editMaskMode, false, 'click again = back to the pixels');
    fire(dock.list, 'click', { target: dock.targetOf('mask', b), shiftKey: true });
    assert.equal(b.maskEnabled, false, 'Shift+click disables the mask');
    fire(dock.list, 'click', { target: dock.targetOf('mask', b), shiftKey: true });
    assert.equal(b.maskEnabled, true);
    fire(dock.list, 'click', { target: dock.targetOf('mask', b), ctrlKey: true });
    assert.deepEqual([rt.selection.w, rt.selection.h], [8, 8], 'Ctrl+click = selection from the mask');
    rt.deselect();
    b.maskCanvas = null;
});

check('dock events: double-click renames inline (Enter commits, Escape cancels)', () => {
    fresh();
    const layer = ops.newLayer();
    const realCreate = document.createElement;
    const inputs = [];
    document.createElement = tag => { if (tag === 'input') { const el = mockEl('input'); inputs.push(el); return el; } return realCreate(tag); };
    try {
        fire(dock.list, 'dblclick', { target: dock.targetOf('name', layer) });
        assert.equal(inputs.length, 1);
        assert.equal(inputs[0].value, layer.name, 'the input starts with the current name');
        inputs[0].value = '  Sky  ';
        fire(inputs[0], 'keydown', { key: 'Enter' });
        assert.equal(layer.name, 'Sky');
        assert.equal(lastLabel(), 'Rename layer');
        fire(dock.list, 'dblclick', { target: dock.targetOf('name', layer) });
        inputs[1].value = 'Never';
        fire(inputs[1], 'keydown', { key: 'Escape' });
        assert.equal(layer.name, 'Sky', 'Escape keeps the old name');
        fire(dock.list, 'dblclick', { target: dock.targetOf('name', layer) });
        inputs[2].value = 'Blur';
        fire(inputs[2], 'blur');
        assert.equal(layer.name, 'Blur', 'leaving the field commits');
    } finally {
        document.createElement = realCreate;
    }
});

check('dock events: dragging a row moves the layer (drop slot from the pointer position)', () => {
    fresh();
    const a = ops.newLayer(), b = ops.newLayer(), c = ops.newLayer();
    const bottom = rt.layers[0];
    // display order (top first): c b a bottom -> rows at y 0, 40, 80, 120
    const before = pushed.length;
    fire(dock.list, 'pointerdown', { target: dock.targetOf('name', bottom), clientX: 10, clientY: 140 });
    fire(dock.list, 'pointermove', { clientX: 10, clientY: 138 });
    assert.deepEqual(rt.layers.map(l => l.id), [bottom, a, b, c].map(l => l.id), 'below the threshold nothing moves');
    fire(dock.list, 'pointermove', { clientX: 10, clientY: 10 });
    assert.ok(dock.list.classes.has('is-dragging'));
    fire(dock.list, 'pointerup', { clientY: 10 });
    assert.deepEqual(rt.layers.map(l => l.id), [a, b, c, bottom].map(l => l.id), 'dropped above the top row = on top of the stack');
    assert.equal(lastLabel(), 'Move layer');
    assert.equal(pushed.length, before + 1);
    assert.ok(!dock.list.classes.has('is-dragging'));
    // the click that follows a drag must not select
    rt.activeLayerIdx = 0;
    fire(dock.list, 'click', { target: dock.targetOf('row', c) });
    assert.notEqual(rt.activeLayer(), c, 'the click right after a drop is swallowed');
    // a drop between two rows
    const bottomNow = rt.layers[0];
    fire(dock.list, 'pointerdown', { target: dock.targetOf('name', bottomNow), clientX: 10, clientY: 140 });
    fire(dock.list, 'pointermove', { clientX: 10, clientY: 68 });
    fire(dock.list, 'pointerup', { clientY: 68 });
    assert.equal(rt.layers.indexOf(rt.layerById(bottomNow.id)), 1, 'dropped in the gap between the 2nd and 3rd row');
    // a press without movement is a plain click
    const entries = pushed.length;
    fire(dock.list, 'pointerdown', { target: dock.targetOf('name', a), clientX: 10, clientY: 10 });
    fire(dock.list, 'pointerup', { clientY: 10 });
    assert.equal(pushed.length, entries);
});

check('dock events: arrow keys, Home/End and F2 on the focused list', () => {
    fresh();
    ops.newLayer(); ops.newLayer();
    const press = (key, extra) => fire(dock.list, 'keydown', Object.assign({ key, target: mockEl('div') }, extra));
    rt.activeLayerIdx = 1;
    press('ArrowUp');
    assert.equal(rt.activeLayerIdx, 2);
    press('ArrowDown'); press('ArrowDown');
    assert.equal(rt.activeLayerIdx, 0);
    press('Home');
    assert.equal(rt.activeLayerIdx, 2);
    press('End');
    assert.equal(rt.activeLayerIdx, 0);
    press('ArrowUp', { ctrlKey: true });
    assert.equal(rt.activeLayerIdx, 0, 'modified arrows are left to the shortcuts');
    rt.activeLayerIdx = 1;
    const count = rt.layers.length;
    press('Delete');
    assert.equal(rt.layers.length, count - 1);
    press(' ');
    assert.equal(rt.activeLayer().visible, false);
});

check('dock events: header controls edit the active layer', () => {
    fresh();
    const layer = ops.newLayer();
    const blend = dock.parts['[data-layer-blend]'], range = dock.parts['[data-layer-opacity-range]'], num = dock.parts['[data-layer-opacity-number]'];
    assert.equal(blend.value, 'normal');
    blend.value = 'multiply';
    fire(blend, 'change');
    assert.equal(layer.blend, 'multiply');
    assert.equal(lastLabel(), 'Blend mode: Multiply');
    const entries = pushed.length;
    range.value = '40'; fire(range, 'input');
    range.value = '35'; fire(range, 'input');
    assert.equal(layer.opacity, 0.35);
    assert.equal(num.value, '35', 'the number follows the slider');
    assert.equal(pushed.length, entries, 'no history while the slider moves');
    fire(range, 'change');
    assert.equal(pushed.length, entries + 1);
    assert.equal(lastLabel(), 'Layer opacity');
    fire(range, 'change');
    assert.equal(pushed.length, entries + 1, 'releasing without a change adds nothing');
    num.value = '80'; fire(num, 'input'); fire(num, 'change');
    assert.equal(layer.opacity, 0.8);
    num.value = ''; fire(num, 'input');
    assert.equal(layer.opacity, 0.8, 'an empty number field is ignored');
    fire(dock.lockBtns[0], 'click');
    fire(dock.lockBtns[1], 'click');
    fire(dock.lockBtns[2], 'click');
    assert.deepEqual([layer.lockPixels, layer.lockAlpha, layer.lockPosition], [true, true, true]);
    assert.ok(dock.lockBtns[0].classes.has('active'), 'lock toggles show their state');
    fire(dock.lockBtns[3], 'click');
    assert.equal(layer.clip, true);
    assert.equal(dock.lockBtns[3].classes.has('active'), true);
    rt.activeLayerIdx = 0;
    ops.select(rt.layers[0].id);
    assert.equal(dock.lockBtns[3].disabled, true, 'the bottom layer cannot be clipped');
    assert.equal(blend.value, 'normal', 'the header follows the active layer');
});

check('dock events: footer buttons', () => {
    fresh();
    const act = name => fire(dock.footerBtns.find(b => b.dataset.layerAct === name), 'click');
    act('new');
    assert.equal(rt.layers.length, 2);
    act('copy');
    assert.equal(rt.layers.length, 3);
    act('duplicate');
    assert.equal(rt.layers.length, 4);
    act('merge');
    assert.equal(rt.layers.length, 3);
    act('mask');
    assert.ok(rt.activeLayer().maskCanvas, 'no selection: reveal all');
    rt.activeLayer().maskCanvas = null;
    rt.selection = { type: 'rect', x: 1, y: 1, w: 20, h: 20 };
    act('mask');
    assert.ok(rt.activeLayer().maskCanvas, 'with a selection: mask from the selection');
    rt.deselect();
    act('delete');
    assert.equal(rt.layers.length, 2);
    assert.equal(dock.footerBtns.find(b => b.dataset.layerAct === 'mask').disabled, false);
    act('delete');
    assert.equal(dock.footerBtns.find(b => b.dataset.layerAct === 'delete').disabled, true, 'delete is off for the last layer');
    assert.equal(dock.footerBtns.find(b => b.dataset.layerAct === 'merge').disabled, true, 'merge down is off for the bottom layer');
});

check('dispose releases the panel', () => {
    const panel = rt.layersPanel;
    window.PixelApp.dispose('layers-w1');
    assert.equal(rt.layersPanel, null);
    assert.doesNotThrow(() => panel.sync());
});

await new Promise(resolve => setTimeout(resolve, 150));
console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-layers: ' + results.length + ' checks passed');
