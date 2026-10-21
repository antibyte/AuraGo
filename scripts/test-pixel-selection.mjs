// WP3 selection (pixel-selection.js): the pure kit (masks, distance transform, region growing, contours) on tiny images,
// then the real module in the fake DOM of test-pixel-core.mjs: registered tools / menu items / shortcuts, selection modes,
// moving, feathering, modify commands and the layer conversions.
import assert from 'node:assert/strict';
import { installFakeDom, makeEl, loadPixelApp } from './pixel-test-dom.mjs';

installFakeDom();
const Pixel = loadPixelApp();
const kit = Pixel.selectionKit;
assert.ok(kit, 'Pixel.selectionKit exists');

const results = [];
// Checks run one after the other: the module checks share one window.
let chain = Promise.resolve();
function check(name, fn) {
    chain = chain.then(async () => {
        try {
            await fn();
            results.push('ok   ' + name);
        } catch (err) {
            results.push('FAIL ' + name + ': ' + (err && err.stack || err));
        }
    });
}
const sum = a => a.reduce((s, v) => s + v, 0);
const count = (a, test) => { let n = 0; for (let i = 0; i < a.length; i++) if (test(a[i])) n++; return n; };

// ---------------------------------------------------------------------------------------------------------------
// Pure kit
// ---------------------------------------------------------------------------------------------------------------

check('rectMask and shiftMask', () => {
    const m = kit.rectMask(10, 10, 2, 3, 4, 2);
    assert.equal(count(m, v => v === 255), 8);
    assert.equal(m[3 * 10 + 2], 255);
    assert.equal(m[2 * 10 + 2], 0);
    const s = kit.shiftMask(m, 10, 10, 3, -1);
    assert.equal(s[2 * 10 + 5], 255);
    assert.equal(count(s, v => v === 255), 8);
    assert.equal(count(kit.shiftMask(m, 10, 10, 9, 0), v => v === 255), 0, 'pushed out of the document');
});

check('ellipseMask area, symmetry and the aliased variant', () => {
    const m = kit.ellipseMask(100, 100, 10, 10, 40, 30, true);
    const area = sum(m) / 255;
    assert.ok(Math.abs(area - Math.PI * 20 * 15) < 0.03 * Math.PI * 20 * 15, 'area ' + area);
    for (let y = 10; y < 40; y++) assert.equal(m[y * 100 + 20], m[y * 100 + 39], 'mirror symmetric');
    assert.ok(count(m, v => v > 0 && v < 255) > 20, 'soft edge');
    const hard = kit.ellipseMask(100, 100, 10, 10, 40, 30, false);
    assert.equal(count(hard, v => v > 0 && v < 255), 0);
    assert.ok(Math.abs(count(hard, v => v === 255) - Math.PI * 300) < 0.03 * Math.PI * 300);
});

check('polygonMask: exact squares, half-pixel anti-alias, winding, concave shapes', () => {
    const sq = kit.polygonMask(10, 10, [{ x: 2, y: 2 }, { x: 6, y: 2 }, { x: 6, y: 6 }, { x: 2, y: 6 }], true);
    assert.equal(count(sq, v => v === 255), 16);
    assert.equal(count(sq, v => v > 0 && v < 255), 0);
    const half = kit.polygonMask(10, 10, [{ x: 2.5, y: 2.5 }, { x: 6.5, y: 2.5 }, { x: 6.5, y: 6.5 }, { x: 2.5, y: 6.5 }], true);
    assert.equal(half[2 * 10 + 2], 64, 'quarter covered corner');
    assert.equal(half[2 * 10 + 4], 128, 'half covered edge');
    assert.equal(half[4 * 10 + 4], 255);
    const hard = kit.polygonMask(10, 10, [{ x: 2.5, y: 2.5 }, { x: 6.5, y: 2.5 }, { x: 6.5, y: 6.5 }, { x: 2.5, y: 6.5 }], false);
    assert.equal(count(hard, v => v > 0 && v < 255), 0);
    const tri = kit.polygonMask(60, 60, [{ x: 5, y: 5 }, { x: 45, y: 10 }, { x: 20, y: 50 }], true);
    const area = Math.abs((45 - 5) * (50 - 5) - (20 - 5) * (10 - 5)) / 2;
    assert.ok(Math.abs(sum(tri) / 255 - area) < 0.01 * area, 'triangle area');
    const bow = kit.polygonMask(20, 20, [{ x: 2, y: 2 }, { x: 10, y: 10 }, { x: 10, y: 2 }, { x: 2, y: 10 }], false);
    assert.ok(bow[6 * 20 + 3] === 255 && bow[6 * 20 + 8] === 255 && bow[3 * 20 + 6] === 0, 'self-intersecting shape fills both lobes');
    assert.equal(kit.polygonMask(10, 10, [{ x: 1, y: 1 }, { x: 5, y: 5 }], true), null);
});

check('morphRound: round grow, shrink, border and stroke rings', () => {
    const base = new Uint8ClampedArray(40 * 40);
    for (let y = 17; y < 23; y++) base.fill(255, y * 40 + 17, y * 40 + 23);
    const grown = kit.morphRound(base, 40, 40, 3, 'grow');
    const b = kit.boundsOf(grown, 40, 40, 0);
    assert.deepEqual([b.x, b.y, b.w, b.h], [14, 14, 12, 12]);
    assert.equal(grown[14 * 40 + 14], 0, 'the corner of a round grow stays empty');
    assert.equal(grown[19 * 40 + 14], 255);
    assert.equal(grown[15 * 40 + 15], 255);
    const shrunk = kit.morphRound(base, 40, 40, 1, 'shrink');
    assert.deepEqual(kit.boundsOf(shrunk, 40, 40, 254), { x: 18, y: 18, w: 4, h: 4 });
    assert.equal(count(kit.morphRound(base, 40, 40, 2, 'shrink'), v => v === 255), 4);
    assert.equal(sum(kit.morphRound(base, 40, 40, 3, 'shrink')), 0, 'shrinks away');
    const border = kit.ringMask(base, 40, 40, 2, 'center');
    assert.equal(border[19 * 40 + 17], 255, 'inside edge');
    assert.equal(border[19 * 40 + 16], 255, 'outside edge');
    assert.equal(border[19 * 40 + 19], 0, 'inner area');
    assert.equal(border[19 * 40 + 14], 0);
    const inside = kit.ringMask(base, 40, 40, 1, 'inside');
    assert.equal(inside[19 * 40 + 17], 255);
    assert.equal(inside[19 * 40 + 16], 0);
    const outside = kit.ringMask(base, 40, 40, 1, 'outside');
    assert.equal(outside[19 * 40 + 16], 255);
    assert.equal(outside[19 * 40 + 17], 0);
    // a shape that touches the document edge does not erode from it
    const edge = new Uint8ClampedArray(20 * 20);
    for (let y = 0; y < 10; y++) edge.fill(255, y * 20, y * 20 + 10);
    const edgeShrunk = kit.morphRound(edge, 20, 20, 2, 'shrink');
    assert.equal(edgeShrunk[0], 255);
    assert.equal(edgeShrunk[5 * 20 + 9], 0);
});

check('floodSelect: tolerance, contiguous and global', () => {
    const w = 11, h = 4;
    const rgba = new Uint8ClampedArray(w * h * 4);
    for (let p = 0; p < w * h; p++) {
        const x = p % w;
        const v = x === 5 ? 200 : (x >= 8 && x <= 9 ? 12 : 10);
        rgba.set([v, v, v, 255], p * 4);
    }
    const contiguous = kit.floodSelect(rgba, w, h, 1, 1, 4, true);
    assert.equal(count(contiguous, v => v === 255), 5 * h);
    const everywhere = kit.floodSelect(rgba, w, h, 1, 1, 4, false);
    assert.equal(count(everywhere, v => v === 255), 10 * h);
    assert.equal(count(kit.floodSelect(rgba, w, h, 1, 1, 0, false), v => v === 255), 8 * h, 'tolerance 0 skips the 12s');
    assert.equal(count(kit.floodSelect(rgba, w, h, 99, 1, 4, true), v => v === 255), 0, 'seed outside');
});

check('colorRangeAlpha: fuzziness, exclusion, invert, transparency', () => {
    const px = [[255, 0, 0, 255], [240, 20, 20, 255], [0, 0, 255, 255], [255, 0, 0, 0]];
    const rgba = new Uint8ClampedArray(px.flat());
    const red = kit.colorRangeAlpha(rgba, 4, 1, [[255, 0, 0]], [], 60, false);
    assert.equal(red[0], 255);
    assert.ok(red[1] > 128, 'near red is mostly in');
    assert.equal(red[2], 0);
    assert.equal(red[3], 0, 'transparent pixels are never picked');
    assert.equal(kit.colorRangeAlpha(rgba, 4, 1, [[255, 0, 0]], [], 0, false)[1], 0, 'fuzziness 0 is exact');
    const loose = kit.colorRangeAlpha(rgba, 4, 1, [[255, 0, 0]], [], 200, false);
    assert.ok(loose[2] === 0 || loose[2] < 255);
    const without = kit.colorRangeAlpha(rgba, 4, 1, [[255, 0, 0], [0, 0, 255]], [[0, 0, 255]], 60, false);
    assert.equal(without[2], 0, 'excluded colour');
    assert.equal(kit.colorRangeAlpha(rgba, 4, 1, [[255, 0, 0]], [], 60, true)[2], 255, 'invert');
});

check('createGrower: stops at a different colour, honours the edge barrier', () => {
    const w = 20, h = 6;
    const rgba = new Uint8ClampedArray(w * h * 4);
    for (let p = 0; p < w * h; p++) rgba.set((p % w) < 10 ? [200, 50, 50, 255] : [50, 50, 200, 255], p * 4);
    const g = kit.createGrower(rgba, w, h, { tolerance: 25, barrier: 14 });
    g.seedDisc(2, 3, 1.5);
    assert.equal(g.grow(-1), true);
    const bin = g.binary();
    assert.equal(count(bin, v => v === 1), 10 * h, 'the red half');
    assert.equal(bin[3 * w + 12], 0);
    const box = g.drainDirty();
    assert.deepEqual([box.x, box.y, box.w, box.h], [0, 0, 10, h]);
    assert.equal(g.drainDirty(), null);

    const lineW = 21;
    const grey = new Uint8ClampedArray(lineW * h * 4);
    for (let p = 0; p < lineW * h; p++) grey.set((p % lineW) === 10 ? [60, 60, 60, 255] : [100, 100, 100, 255], p * 4);
    const blocked = kit.createGrower(grey, lineW, h, { tolerance: 25, barrier: 8 });
    blocked.seedDisc(2, 3, 1);
    blocked.grow(-1);
    assert.equal(count(blocked.binary(), v => v === 1), 10 * h, 'the line is an edge: growth stops before it');
    const open = kit.createGrower(grey, lineW, h, { tolerance: 25, barrier: 1000 });
    open.seedDisc(2, 3, 1);
    open.grow(-1);
    assert.equal(count(open.binary(), v => v === 1), lineW * h, 'without a barrier it crosses');
    // the brush teaches a second colour: the region follows it
    const two = new Uint8ClampedArray(w * h * 4);
    for (let p = 0; p < w * h; p++) two.set((p % w) < 10 ? [200, 50, 50, 255] : [50, 50, 200, 255], p * 4);
    const learn = kit.createGrower(two, w, h, { tolerance: 25, barrier: 1000 });
    learn.seedDisc(2, 3, 1);
    learn.grow(-1);
    learn.seedDisc(15, 3, 1);
    learn.grow(-1);
    assert.equal(count(learn.binary(), v => v === 1), w * h);
});

check('majority, upscaleBinary and smoothEdges', () => {
    const bin = new Uint8Array(100);
    for (let y = 2; y < 8; y++) for (let x = 2; x < 8; x++) bin[y * 10 + x] = 1;
    bin[5 * 10 + 5] = 0;
    bin[1 * 10 + 8] = 1;
    const cleaned = kit.majority(bin, 10, 10);
    assert.equal(cleaned[5 * 10 + 5], 1, 'hole filled');
    assert.equal(cleaned[1 * 10 + 8], 0, 'speck removed');
    const small = new Uint8Array(16);
    for (let y = 1; y < 3; y++) for (let x = 1; x < 3; x++) small[y * 4 + x] = 1;
    const big = kit.upscaleBinary(small, 4, 4, 8, 8);
    assert.ok(Math.abs(count(big, v => v >= 128) - 16) <= 4, 'area x4: ' + count(big, v => v >= 128));
    const same = kit.upscaleBinary(small, 4, 4, 4, 4);
    assert.deepEqual(Array.from(same), Array.from(small, v => v * 255));
    const hard = new Uint8ClampedArray(20 * 20);
    for (let y = 5; y < 10; y++) hard.fill(255, y * 20 + 5, y * 20 + 10);
    kit.smoothEdges(hard, 20, 20);
    assert.equal(hard[7 * 20 + 7], 255);
    assert.ok(hard[7 * 20 + 4] > 0 && hard[7 * 20 + 4] < 128, 'soft outside neighbour');
    assert.ok(hard[7 * 20 + 5] > 128 && hard[7 * 20 + 5] < 255, 'soft inside edge');
});

check('traceContours: loops, holes, collinear points, diagonal pixels, edge cap', () => {
    const w = 10, h = 10;
    const a = kit.rectMask(w, h, 2, 2, 4, 4);
    const loops = kit.traceContours(a, w, h, kit.boundsOf(a, w, h, 127));
    assert.equal(loops.length, 1);
    assert.equal(loops[0].length, 8, 'only the four corners');
    const ring = kit.rectMask(w, h, 1, 1, 6, 6);
    for (let y = 3; y < 5; y++) ring.fill(0, y * w + 3, y * w + 5);
    assert.equal(kit.traceContours(ring, w, h, kit.boundsOf(ring, w, h, 127)).length, 2, 'outline and hole');
    const diag = new Uint8ClampedArray(w * h);
    diag[1 * w + 2] = 255; diag[2 * w + 1] = 255;
    const loopsDiag = kit.traceContours(diag, w, h, kit.boundsOf(diag, w, h, 127));
    assert.ok(loopsDiag.length >= 1 && loopsDiag.length <= 2);
    assert.equal(kit.traceContours(a, w, h, kit.boundsOf(a, w, h, 127), 3), null, 'edge cap');
    const full = new Uint8ClampedArray(w * h).fill(255);
    assert.equal(kit.traceContours(full, w, h, { x: 0, y: 0, w, h })[0].length, 8, 'the document border is an outline too');
});

// ---------------------------------------------------------------------------------------------------------------
// Module in the fake DOM
// ---------------------------------------------------------------------------------------------------------------

// pixel.js runs the built-in installers; the filters package is rewritten in parallel and may not provide its installer
// at every moment. The selection checks do not depend on it.
if (typeof Pixel.installFilters !== 'function') Pixel.installFilters = () => {};
const runtimes = [];
Pixel.registerModule('test-capture', r => { runtimes.push(r); });
const notes = [];
let menus = null;
const host = makeEl('div');
window.PixelApp.render(host, 'sel1', {
    esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
    t: (k, f) => f || k, api: async () => ({}), iconMarkup: () => '', notify: n => notes.push(n),
    setWindowMenus: (id, m) => { menus = m; }, readonly: false
});
const rt = runtimes[runtimes.length - 1];
const P = (x, y, extra) => Object.assign({ x, y, pressure: 0.5, buttons: 1, shift: false, alt: false, ctrl: false, pointerType: 'mouse', pointerId: 1 }, extra);
const EV = { button: 0 };
const idle = async () => { await rt.selectionUI.chain; };
const selCount = () => { const m = rt.getSelectionMask(); return m ? count(m.alpha, v => v > 127) : 0; };

check('module installed and tools registered', () => {
    assert.ok(rt && rt.selectionUI, 'selectionUI exists (the installer did not throw)');
    ['select-rect', 'select-ellipse', 'lasso', 'magic-wand', 'polygon-lasso', 'quick-select'].forEach(id => {
        const def = Pixel.getTool(id);
        assert.ok(def, id);
        assert.equal(def.group, 'select', id + ' group');
        assert.equal(typeof def.onDown, 'function', id + ' owns its gesture');
        assert.equal(typeof def.options, 'function');
        assert.equal(def.raster, false);
    });
    assert.equal(Pixel.getTool('select-rect').shortcut, 'V', 'inherited from the built-in');
    assert.equal(Pixel.getTool('quick-select').shortcut, 'Shift+W');
    assert.ok(Pixel.getTool('polygon-lasso').svg.includes('<svg'));
    assert.ok(rt.listOverlays().includes('selection'));
});

check('select menu, commands and shortcuts', () => {
    const select = (menus || []).find(m => m.id === 'select');
    assert.ok(select, 'select menu built');
    const ids = select.items.map(i => i.id);
    ['select-all', 'deselect', 'reselect', 'inverse', 'color-range', 'modify', 'stroke', 'cut-to-layer'].forEach(id => assert.ok(ids.includes(id), id));
    assert.deepEqual(select.items.find(i => i.id === 'modify').items.map(i => i.id), ['grow', 'shrink', 'border', 'smooth', 'feather']);
    const cmds = rt.listCommands();
    const keys = new Set(cmds.flatMap(c => c.shortcuts));
    ['ctrl+a', 'ctrl+d', 'ctrl+shift+d', 'ctrl+shift+i', 'shift+f6', 'ctrl+shift+j', 'alt+backspace', 'ctrl+backspace', 'shift+k', 'shift+w'].forEach(k => assert.ok(keys.has(k), 'shortcut ' + k));
    assert.ok(cmds.some(c => c.id === 'menu:select:color-range'), 'colour range is a command');
    assert.ok(cmds.some(c => c.id === 'menu:select:modify/feather'));
    const edit = menus.find(m => m.id === 'edit');
    assert.ok(edit.items.some(i => i.id === 'select-all') && edit.items.some(i => i.id === 'deselect'), 'edit items replaced in place');
    // The items are decided by feature detection only. A package that registers the same id (cutout does for
    // select-subject) replaces ours in place; its item then has no hidden() and is not checked here.
    const saved = { subject: rt.computeSubjectMask, fill: rt.contentAwareFill };
    const item = id => Pixel.composeMenus(rt).find(m => m.id === 'select').items.find(i => i.id === id);
    const expectHidden = (id, value, message) => { const found = item(id); if (found && typeof found.hidden === 'boolean') assert.equal(found.hidden, value, message); };
    rt.computeSubjectMask = undefined;
    rt.contentAwareFill = undefined;
    expectHidden('select-subject', true, 'hidden without computeSubjectMask');
    expectHidden('fill-content-aware', true, 'hidden without contentAwareFill');
    rt.computeSubjectMask = async () => null;
    rt.contentAwareFill = () => {};
    expectHidden('select-subject', false, 'shown with computeSubjectMask');
    expectHidden('fill-content-aware', false, 'shown with contentAwareFill');
    rt.computeSubjectMask = saved.subject;
    rt.contentAwareFill = saved.fill;
});

check('rectangle tool: new, add, subtract, intersect, click to deselect, history', async () => {
    rt.newBlankCanvas(100, 80);
    rt.isDirty = false;
    const tool = Pixel.getTool('select-rect');
    tool.onDown(rt, EV, P(10, 10)); tool.onMove(rt, EV, P(30, 30)); tool.onUp(rt, EV, P(30, 30));
    await idle();
    assert.deepEqual([rt.selection.type, rt.selection.x, rt.selection.y, rt.selection.w, rt.selection.h], ['rect', 10, 10, 20, 20]);
    assert.equal(rt.history[rt.historyIdx].label, 'select:rect');
    assert.equal(rt.isDirty, false, 'selecting does not dirty the file');
    assert.ok(String(rt.statusText.textContent).includes('20 × 20'), 'status shows the size: ' + rt.statusText.textContent);
    // add (Shift held before the drag)
    tool.onDown(rt, EV, P(40, 40, { shift: true })); tool.onMove(rt, EV, P(50, 50, { shift: true })); tool.onUp(rt, EV, P(50, 50, { shift: true }));
    await idle();
    assert.equal(rt.selection.type, 'mask');
    assert.equal(selCount(), 400 + 100);
    // subtract (Alt)
    tool.onDown(rt, EV, P(10, 10, { alt: true })); tool.onMove(rt, EV, P(20, 30, { alt: true })); tool.onUp(rt, EV, P(20, 30, { alt: true }));
    await idle();
    assert.equal(selCount(), 200 + 100);
    // intersect (Shift+Alt)
    tool.onDown(rt, EV, P(15, 0, { shift: true, alt: true })); tool.onMove(rt, EV, P(80, 20, { shift: true, alt: true })); tool.onUp(rt, EV, P(80, 20, { shift: true, alt: true }));
    await idle();
    assert.equal(selCount(), 10 * 10, 'x 20..30 of the first rect, y 10..20');
    // a click deselects
    tool.onDown(rt, EV, P(90, 70)); tool.onUp(rt, EV, P(90, 70));
    await idle();
    assert.equal(rt.selection, null);
    rt.undo();
    assert.ok(rt.selection && selCount() === 100, 'undo brings the intersect result back');
    rt.undo();
    assert.equal(selCount(), 300);
    rt.deselect();
});

check('rectangle tool: fixed size, fixed ratio, Shift constrains after the drag began', async () => {
    const tool = Pixel.getTool('select-rect');
    const o = rt.selectionUI.opts;
    rt.deselect();
    o.style = 'fixed'; o.sizeW = 30; o.sizeH = 20;
    tool.onDown(rt, EV, P(5, 5)); tool.onUp(rt, EV, P(5, 5));
    await idle();
    assert.deepEqual([rt.selection.x, rt.selection.y, rt.selection.w, rt.selection.h], [5, 5, 30, 20]);
    rt.deselect();
    o.style = 'ratio'; o.ratioW = 2; o.ratioH = 1;
    tool.onDown(rt, EV, P(10, 10)); tool.onMove(rt, EV, P(50, 20)); tool.onUp(rt, EV, P(50, 20));
    await idle();
    assert.deepEqual([rt.selection.w, rt.selection.h], [40, 20]);
    rt.deselect();
    o.style = 'normal';
    tool.onDown(rt, EV, P(10, 10)); tool.onMove(rt, EV, P(50, 20)); tool.onMove(rt, EV, P(50, 20, { shift: true })); tool.onUp(rt, EV, P(50, 20, { shift: true }));
    await idle();
    assert.deepEqual([rt.selection.w, rt.selection.h], [40, 40], 'a square');
    rt.deselect();
    tool.onDown(rt, EV, P(60, 40)); tool.onMove(rt, EV, P(70, 45, { alt: true })); tool.onUp(rt, EV, P(70, 45, { alt: true }));
    await idle();
    assert.deepEqual([rt.selection.x, rt.selection.y, rt.selection.w, rt.selection.h], [50, 35, 20, 10], 'from the centre');
    rt.deselect();
});

check('moving the outline: rectangle, mask and arrow keys', async () => {
    const tool = Pixel.getTool('select-rect');
    tool.onDown(rt, EV, P(10, 10)); tool.onMove(rt, EV, P(30, 30)); tool.onUp(rt, EV, P(30, 30));
    await idle();
    tool.onDown(rt, EV, P(20, 20)); tool.onMove(rt, EV, P(25, 28)); tool.onUp(rt, EV, P(25, 28));
    assert.deepEqual([rt.selection.x, rt.selection.y], [15, 18]);
    assert.equal(rt.history[rt.historyIdx].label, 'select:move');
    rt.selection = null;
    // mask selection moves with its alpha
    rt.setSelectionMask(kit.rectMask(100, 80, 10, 10, 10, 10), { mode: 'replace' });
    const lasso = Pixel.getTool('lasso');
    lasso.onDown(rt, EV, P(15, 15)); lasso.onMove(rt, EV, P(20, 18)); lasso.onUp(rt, EV, P(20, 18));
    assert.equal(rt.selection.type, 'mask');
    assert.equal(rt.getSelectionMask().alpha[13 * 100 + 15], 255);
    assert.equal(rt.getSelectionMask().alpha[10 * 100 + 10], 0);
    assert.equal(selCount(), 100);
    // arrow keys
    assert.equal(lasso.onKey(rt, { key: 'ArrowRight', shiftKey: true }), true);
    assert.equal(rt.getSelectionMask().x, 25, 'moved by 10');
    assert.equal(lasso.onKey(rt, { key: 'a', shiftKey: false }), false);
    clearTimeout(rt.selectionUI.nudgeTimer);
    rt.deselect();
});

check('feathered ellipse, lasso and polygon commit through the worker fallback', async () => {
    const o = rt.selectionUI.opts;
    o.feather = 4;
    const ell = Pixel.getTool('select-ellipse');
    ell.onDown(rt, EV, P(20, 20)); ell.onMove(rt, EV, P(60, 50)); ell.onUp(rt, EV, P(60, 50));
    await idle();
    assert.equal(rt.selection.type, 'mask');
    const soft = count(rt.selection.alpha, v => v > 0 && v < 255);
    assert.ok(soft > 100, 'feathered edge: ' + soft);
    assert.equal(rt.history[rt.historyIdx].label, 'select:ellipse');
    o.feather = 0;
    ell.onDown(rt, EV, P(20, 20)); ell.onMove(rt, EV, P(60, 50)); ell.onUp(rt, EV, P(60, 50));
    await idle();
    assert.equal(rt.selection.type, 'ellipse', 'plain ellipses stay vector');
    const poly = Pixel.getTool('polygon-lasso');
    rt.deselect();
    [[10, 10], [40, 10], [40, 40], [10, 40]].forEach(([x, y]) => { poly.onDown(rt, EV, P(x, y)); });
    assert.equal(rt.selection, null, 'still open');
    assert.equal(poly.onKey(rt, { key: 'Backspace' }), true);
    assert.equal(rt.selectionUI.poly.pts.length, 3);
    poly.onDown(rt, EV, P(10, 40));
    assert.equal(poly.onKey(rt, { key: 'Enter' }), true);
    await idle();
    assert.equal(selCount(), 30 * 30);
    assert.equal(rt.history[rt.historyIdx].label, 'select:polygon');
    [[10, 10], [30, 10], [30, 30]].forEach(([x, y]) => { poly.onDown(rt, EV, P(x, y)); });
    assert.equal(poly.onKey(rt, { key: 'Escape' }), true);
    assert.equal(rt.selectionUI.poly, null, 'escape cancels');
    assert.equal(poly.onKey(rt, { key: 'Escape' }), false, 'escape passes on when nothing is open');
    rt.deselect();
});

check('magic wand and quick selection on the (blank) fake image', async () => {
    const wand = Pixel.getTool('magic-wand');
    wand.onDown(rt, EV, P(5, 5));
    await idle();
    assert.equal(selCount(), 100 * 80, 'a blank layer is one region');
    assert.equal(rt.history[rt.historyIdx].label, 'select:wand');
    rt.deselect();
    rt.selectionUI.opts.contiguous = false;
    wand.onDown(rt, EV, P(5, 5, { alt: true }));
    await idle();
    assert.equal(rt.selection, null, 'subtract without a selection does nothing');
    const qs = Pixel.getTool('quick-select');
    qs.onDown(rt, EV, P(50, 40)); qs.onMove(rt, EV, P(52, 42)); qs.onUp(rt, EV, P(52, 42));
    await idle();
    assert.ok(selCount() > 7000, 'blank image grows to the whole document: ' + selCount());
    assert.equal(rt.history[rt.historyIdx].label, 'select:quick');
    rt.deselect();
});

check('select commands: all, inverse, deselect, reselect, history', () => {
    rt.runCommand('menu:select:select-all');
    assert.equal(rt.selection.type, 'rect');
    assert.equal(rt.history[rt.historyIdx].label, 'select:all');
    rt.selection = { type: 'rect', x: 10, y: 10, w: 20, h: 10 };
    rt.runCommand('menu:select:inverse');
    assert.equal(rt.selection.type, 'mask');
    assert.equal(selCount(), 8000 - 200);
    rt.runCommand('menu:select:deselect');
    assert.equal(rt.selection, null);
    assert.equal(rt.history[rt.historyIdx].label, 'select:none');
    assert.equal(rt.runCommand('menu:select:reselect'), true);
    assert.equal(selCount(), 8000 - 200, 'the inverted selection returns');
    assert.equal(rt.history[rt.historyIdx].label, 'select:reselect');
    assert.equal(rt.runCommand('menu:edit:deselect'), true);
    assert.equal(rt.selection, null);
});

check('modify: grow, shrink, border, smooth, feather', async () => {
    const square = () => rt.setSelectionMask(kit.rectMask(100, 80, 40, 30, 10, 10), { mode: 'replace' });
    square();
    assert.equal(await rt.modifySelection('grow', 3), true);
    let box = rt.getSelectionMask();
    assert.deepEqual([box.x, box.y, box.w, box.h], [37, 27, 16, 16]);
    assert.equal(rt.history[rt.historyIdx].label, 'select:grow');
    assert.equal(await rt.modifySelection('shrink', 3), true);
    box = rt.getSelectionMask();
    assert.deepEqual([box.w, box.h], [10, 10]);
    assert.equal(await rt.modifySelection('border', 4), true);
    assert.equal(rt.getSelectionMask().alpha[35 * 100 + 45], 0, 'the middle is open');
    assert.equal(rt.getSelectionMask().alpha[35 * 100 + 40], 255);
    square();
    assert.equal(await rt.modifySelection('feather', 4), true);
    assert.ok(count(rt.selection.alpha, v => v > 0 && v < 255) > 40, 'soft');
    square();
    assert.equal(await rt.modifySelection('smooth', 3), true);
    assert.ok(rt.getSelectionMask().alpha[35 * 100 + 45] === 255);
    square();
    assert.equal(await rt.modifySelection('shrink', 8), false, 'shrinking away empties the selection');
    assert.equal(rt.selection, null);
    assert.ok(notes.some(n => /empty/i.test(n.message)));
});

check('selection to layers, mask, fill and stroke', async () => {
    rt.setSelectionMask(kit.rectMask(100, 80, 10, 10, 20, 20), { mode: 'replace' });
    const layers = rt.layers.length;
    assert.equal(rt.layerFromSelection(false), true);
    assert.equal(rt.layers.length, layers + 1);
    assert.equal(rt.history[rt.historyIdx].label, 'select:copy-to-layer');
    assert.equal(rt.layerFromSelection(true), true);
    assert.equal(rt.layers.length, layers + 2);
    assert.equal(rt.history[rt.historyIdx].label, 'select:cut-to-layer');
    assert.equal(rt.selectionToLayerMask(), true);
    assert.ok(rt.activeLayer().maskCanvas, 'mask created');
    assert.equal(rt.history[rt.historyIdx].label, 'select:to-mask');
    assert.equal(rt.runCommand('menu:select:fill-primary'), true);
    assert.equal(rt.history[rt.historyIdx].label, 'fill:selection');
    assert.equal(await rt.selectFromLayerAlpha(null, 'replace'), false, 'the fake layers are empty');
    assert.equal(rt.selection, null);
    // stroke: dialogs open in body; drive the function through the dialog form
    rt.setSelectionMask(kit.rectMask(100, 80, 30, 30, 20, 20), { mode: 'replace' });
    assert.equal(rt.runCommand('menu:select:stroke'), true);
    assert.ok(rt.selectionUI.dialog, 'dialog opened');
    rt.deselect();
});

check('colour range panel opens, previews and closes', async () => {
    rt.setSelectionMask(kit.rectMask(100, 80, 30, 30, 20, 20), { mode: 'replace' });
    assert.equal(rt.runCommand('menu:select:color-range'), true);
    const cr = rt.selectionUI.cr;
    assert.ok(cr && cr.el && cr.preview, 'panel state');
    assert.equal(cr.pw, 100);
    cr.include.push([0, 0, 0]);
    rt.selectionUI.colorRange.apply();
    await idle();
    assert.equal(rt.selectionUI.cr, null, 'closed on apply');
    assert.equal(rt.selection, null, 'the fake image is fully transparent, which a colour range never picks');
    rt.selectionUI.colorRange.open();
    rt.selectionUI.colorRange.close();
    assert.equal(rt.selectionUI.cr, null);
});

check('overlay draws every selection type without throwing', () => {
    const errors = [];
    const original = console.error;
    console.error = (...args) => errors.push(args.join(' '));
    try {
        [{ type: 'rect', x: 5, y: 5, w: 10, h: 10 }, { type: 'ellipse', x: 5, y: 5, w: 10, h: 10 }].forEach(sel => { rt.selection = sel; rt.paintOverlays(); });
        rt.setSelectionMask(kit.rectMask(100, 80, 30, 30, 20, 20), { mode: 'replace' });
        rt.paintOverlays();
        const noise = new Uint8ClampedArray(100 * 80);
        for (let i = 0; i < noise.length; i += 2) noise[i] = 255;
        rt.setSelectionMask(noise, { mode: 'replace' });
        rt.paintOverlays();
        rt.deselect();
        rt.paintOverlays();
    } finally {
        console.error = original;
    }
    assert.deepEqual(errors, []);
});

chain.then(() => {
    results.forEach(line => console.log(line));
    const failed = results.filter(line => line.startsWith('FAIL'));
    if (failed.length) {
        console.error('test-pixel-selection: ' + failed.length + ' failed');
        process.exit(1);
    }
    console.log('test-pixel-selection: ' + results.length + ' checks passed');
    process.exit(0);
});
