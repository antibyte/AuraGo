#!/usr/bin/env node
// Runs the pure CYD Creator designer modules (geometry, model, render) in Node and checks them:
// snapping and guides, resize/align/distribute maths, the layout store (history, coalescing,
// origins), every layout op incl. reparenting and tabs (structural mirror of Go ApplyOps), name
// rules incl. reserved names, and a render smoke test of all 26 widget types in every theme through
// the string serialiser. Capabilities come from ui/testdata/cyd_creator_designer_caps.json (kept
// equal to internal/cydcreator by TestCYDCreatorDesignerCapsFixtureMatchesCatalog). P.util comes
// from the real cyd-creator-core.js (plan 4a), which runs in Node and loads first.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';
import { webcrypto } from 'node:crypto';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '..');
const apps = path.join(root, 'ui', 'js', 'desktop', 'apps');
const caps = JSON.parse(fs.readFileSync(path.join(root, 'ui', 'testdata', 'cyd_creator_designer_caps.json'), 'utf8'));

const sandbox = { window: {}, console, crypto: webcrypto, structuredClone, Math, JSON, Object, Array, Set, Map, Number, String, RegExp, Error, Date, Uint8Array };
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
const core = path.join(apps, 'cyd-creator-core.js');
vm.runInContext(fs.readFileSync(core, 'utf8'), sandbox, { filename: core });
for (const file of ['cyd-creator-geometry.js', 'cyd-creator-model.js', 'cyd-creator-render.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const P = sandbox.window.CYDCreatorParts;
const G = P.geometry;
const M = P.model;
const R = P.render;

let failures = 0;
let checks = 0;
function check(name, cond, detail) {
    checks++;
    if (cond) return;
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) {
    const a = JSON.stringify(got);
    const b = JSON.stringify(want);
    check(name, a === b, 'got ' + a + ' want ' + b);
}
// fails expects a ModelError CYD_INVALID_LAYOUT whose reason contains text.
function fails(name, fn, text) {
    try { fn(); check(name, false, 'did not throw'); } catch (err) {
        check(name, err && err.code === 'CYD_INVALID_LAYOUT' && String(err.details && err.details.reason).includes(text), String(err && err.message));
    }
}

// ── geometry ──
eq('clampZoom rounds and clamps', [G.clampZoom(0), G.clampZoom(2.4), G.clampZoom(9), G.clampZoom('x')], [1, 2, 4, 1]);
eq('fitZoom picks the largest integer zoom', G.fitZoom({ w: 320, h: 240 }, { w: 1000, h: 800 }, 24), 2);
eq('fitZoom never below 1', G.fitZoom({ w: 800, h: 480 }, { w: 300, h: 200 }, 24), 1);
eq('fitZoom caps at 4', G.fitZoom({ w: 100, h: 100 }, { w: 2000, h: 2000 }, 0), 4);
const board = caps.boards[0];
eq('screenSize landscape', G.screenSize(board, 'landscape'), { w: 320, h: 240 });
eq('screenSize portrait swaps', G.screenSize(board, 'portrait'), { w: 240, h: 320 });
eq('screenSize flipped keeps landscape', G.screenSize(board, 'landscape_flipped'), { w: 320, h: 240 });
eq('screenSize native portrait panel', G.screenSize({ display: { width: 240, height: 320, native_orientation: 'portrait' } }, 'landscape'), { w: 320, h: 240 });
eq('snapStep accepts 0/4/8 only', [G.snapStep(4), G.snapStep(8), G.snapStep(5), G.snapStep('8')], [4, 8, 0, 8]);
eq('snapValue grid', [G.snapValue(13, 8), G.snapValue(13, 4), G.snapValue(13.4, 0)], [16, 12, 13]);
const targets = G.guideTargets([{ x: 100, y: 50, w: 40, h: 20 }], { x: 0, y: 0, w: 320, h: 240 });
eq('guide targets x', targets.x, [0, 100, 120, 140, 160, 320]);
eq('guide targets y', targets.y, [0, 50, 60, 70, 120, 240]);
const sm = G.snapMove({ x: 98, y: 33, w: 30, h: 10 }, targets, { step: 8, radius: 4 });
eq('snapMove left edge to sibling edge', [sm.x, sm.guides.x], [100, [100]]);
eq('snapMove y falls back to grid', [sm.y, sm.guides.y], [32, []]);
const smc = G.snapMove({ x: 145, y: 113, w: 30, h: 14 }, targets, { step: 0, radius: 4 });
eq('snapMove centre to screen centre', [smc.x + 15, smc.guides.x], [160, [160]]);
eq('snapMove centre y to screen centre', [smc.y + 7, smc.guides.y], [120, [120]]);
const smo = G.snapMove({ x: 13, y: 13, w: 10, h: 10 }, { x: [], y: [] }, { step: 8, origin: { x: 2, y: 2 } });
eq('snapMove grid relative to the parent origin', [smo.x, smo.y], [10, 10]);
const smOff = G.snapMove({ x: 98, y: 33, w: 30, h: 10 }, targets, { step: 8, guides: false });
eq('snapMove without guides (Alt)', [smOff.x, smOff.y], [96, 32]);
eq('resize se', G.resizeRect({ x: 10, y: 10, w: 40, h: 20 }, 'se', 5, 7), { x: 10, y: 10, w: 45, h: 27 });
eq('resize nw keeps the opposite corner', G.resizeRect({ x: 10, y: 10, w: 40, h: 20 }, 'nw', 5, 4), { x: 15, y: 14, w: 35, h: 16 });
eq('resize min size', G.resizeRect({ x: 10, y: 10, w: 40, h: 20 }, 'w', 100, 0), { x: 46, y: 10, w: 4, h: 20 });
eq('resize keep ratio', G.resizeRect({ x: 0, y: 0, w: 40, h: 20 }, 'se', 20, 0, { keepRatio: true }), { x: 0, y: 0, w: 60, h: 30 });
eq('resize n edge only', G.resizeRect({ x: 0, y: 10, w: 40, h: 20 }, 'n', 30, -4), { x: 0, y: 6, w: 40, h: 24 });
const sr = G.snapResize({ x: 10, y: 10, w: 88, h: 21 }, 'se', targets, { step: 8, radius: 4 });
eq('snapResize right edge to guide, bottom to grid', [sr.w, sr.h, sr.guides.x], [90, 22, [100]]);
eq('align left', G.align([{ x: 10, y: 0, w: 10, h: 10 }, { x: 30, y: 5, w: 20, h: 10 }], 'left'), [{ x: 10, y: 0, w: 10, h: 10 }, { x: 10, y: 5, w: 20, h: 10 }]);
eq('align right', G.align([{ x: 10, y: 0, w: 10, h: 10 }, { x: 30, y: 5, w: 20, h: 10 }], 'right').map(r => r.x), [40, 30]);
eq('align hcenter', G.align([{ x: 0, y: 0, w: 10, h: 10 }, { x: 40, y: 0, w: 20, h: 10 }], 'hcenter').map(r => r.x), [25, 20]);
eq('align bottom', G.align([{ x: 0, y: 0, w: 10, h: 10 }, { x: 0, y: 40, w: 10, h: 20 }], 'bottom').map(r => r.y), [50, 40]);
eq('align single to frame vcenter', G.align([{ x: 5, y: 0, w: 10, h: 20 }], 'vcenter', { x: 0, y: 0, w: 320, h: 240 }), [{ x: 5, y: 110, w: 10, h: 20 }]);
eq('distribute h keeps ends', G.distribute([{ x: 0, y: 0, w: 10, h: 10 }, { x: 100, y: 0, w: 10, h: 10 }, { x: 20, y: 0, w: 20, h: 10 }], 'h').map(r => r.x), [0, 100, 45]);
eq('distribute v', G.distribute([{ x: 0, y: 0, w: 10, h: 10 }, { x: 0, y: 15, w: 10, h: 10 }, { x: 0, y: 90, w: 10, h: 10 }], 'v').map(r => r.y), [0, 45, 90]);
eq('distribute needs three', G.distribute([{ x: 0, y: 0, w: 1, h: 1 }, { x: 9, y: 0, w: 1, h: 1 }], 'h').map(r => r.x), [0, 9]);
const items = [{ id: 'a', rect: { x: 0, y: 0, w: 50, h: 50 } }, { id: 'b', rect: { x: 20, y: 20, w: 50, h: 50 } }];
eq('hitTest top-most', [G.hitTest(items, { x: 25, y: 25 }), G.hitTest(items, { x: 5, y: 5 }), G.hitTest(items, { x: 90, y: 90 })], ['b', 'a', null]);
eq('inBox', G.inBox(items, { x: 60, y: 60, w: 5, h: 5 }), ['b']);
eq('nudge', [G.nudgeDelta('ArrowLeft', false), G.nudgeDelta('ArrowDown', true), G.nudgeDelta('a', false)], [{ x: -1, y: 0 }, { x: 0, y: 8 }, null]);
eq('toWorld', G.toWorld({ x: 125, y: 63 }, { left: 20, top: 10 }, 2), { x: 52, y: 26 });
eq('clampInto', G.clampInto({ x: 300, y: -5, w: 40, h: 20 }, { x: 0, y: 0, w: 320, h: 240 }), { x: 280, y: 0, w: 40, h: 20 });
// edge cases: small widgets, server limits, rounding, ties
const goLimits = fs.readFileSync(path.join(root, 'internal', 'cydcreator', 'limits.go'), 'utf8');
eq('MAX_SIDE mirrors the server maxWidgetSide', G.MAX_SIDE, Number((goLimits.match(/maxWidgetSide\s*=\s*(\d+)/) || [])[1]));
eq('fitZoom with a 0x0 viewport', G.fitZoom({ w: 320, h: 240 }, { w: 0, h: 0 }, 24), 1);
const one = { x: [100], y: [] };
eq('snapMove radius is inclusive', [G.snapMove({ x: 96, y: 0, w: 30, h: 10 }, one, { step: 0, radius: 4 }).x, G.snapMove({ x: 95, y: 0, w: 30, h: 10 }, one, { step: 0, radius: 4 }).x], [100, 95]);
eq('snapMove fractional radius', [G.snapMove({ x: 98, y: 0, w: 30, h: 10 }, one, { step: 0, radius: 1.5 }).x, G.snapMove({ x: 99, y: 0, w: 30, h: 10 }, one, { step: 0, radius: 1.5 }).x, G.snapMove({ x: 98, y: 0, w: 30, h: 10 }, one, { step: 0, radius: 2 }).x], [98, 100, 100]);
const tie = G.snapMove({ x: 98, y: 0, w: 30, h: 10 }, { x: [96, 100], y: [] }, { step: 0, radius: 4 });
eq('snapMove tie keeps the first line', [tie.x, tie.guides.x], [96, [96]]);
const neg = G.snapMove({ x: -13, y: -12, w: 10, h: 10 }, { x: [], y: [] }, { step: 8 });
eq('snapMove negative grid positions', [neg.x, neg.y], [-16, -8]);
eq('resize keeps a widget below MIN_SIZE (zero delta)', G.resizeRect({ x: 10, y: 10, w: 120, h: 2 }, 'se', 0, 0), { x: 10, y: 10, w: 120, h: 2 });
eq('resize keeps a small side while the other grows', G.resizeRect({ x: 10, y: 10, w: 120, h: 2 }, 'se', 5, 0), { x: 10, y: 10, w: 125, h: 2 });
eq('resize never shrinks a small side further', G.resizeRect({ x: 10, y: 10, w: 120, h: 2 }, 's', 0, -5), { x: 10, y: 10, w: 120, h: 2 });
eq('resize keep ratio ne', G.resizeRect({ x: 0, y: 10, w: 40, h: 20 }, 'ne', 20, 0, { keepRatio: true }), { x: 0, y: 0, w: 60, h: 30 });
eq('resize keep ratio nw', G.resizeRect({ x: 20, y: 20, w: 40, h: 20 }, 'nw', -20, 0, { keepRatio: true }), { x: 0, y: 10, w: 60, h: 30 });
eq('resize keep ratio with the min clamp', G.resizeRect({ x: 0, y: 0, w: 40, h: 10 }, 'se', -38, 0, { keepRatio: true }), { x: 0, y: 0, w: 4, h: 4 });
eq('resize keep ratio from below MIN_SIZE', G.resizeRect({ x: 0, y: 0, w: 120, h: 2 }, 'se', -60, 0, { keepRatio: true }), { x: 0, y: 0, w: 60, h: 2 });
eq('resize caps sides at MAX_SIDE', [G.resizeRect({ x: 0, y: 0, w: 2000, h: 20 }, 'e', 100, 0), G.resizeRect({ x: 100, y: 0, w: 2000, h: 20 }, 'w', -100, 0), G.resizeRect({ x: 0, y: 0, w: 20, h: 2000 }, 's', 0, 500).h],
    [{ x: 0, y: 0, w: 2048, h: 20 }, { x: 52, y: 0, w: 2048, h: 20 }, 2048]);
eq('resize keep ratio caps at MAX_SIDE and keeps the ratio', G.resizeRect({ x: 0, y: 0, w: 100, h: 200 }, 'se', 1000, 0, { keepRatio: true }), { x: 0, y: 0, w: 1024, h: 2048 });
eq('resize fractional delta keeps the opposite edges', G.resizeRect({ x: 10, y: 10, w: 40, h: 20 }, 'nw', 0.5, 0.5), { x: 11, y: 11, w: 39, h: 19 });
const srw = G.snapResize({ x: 98, y: 10, w: 42, h: 20 }, 'w', targets, { step: 8, radius: 4 });
eq('snapResize w edge to guide', [srw.x, srw.w, srw.guides.x], [100, 40, [100]]);
const srn = G.snapResize({ x: 10, y: 47, w: 20, h: 30 }, 'n', targets, { step: 8, radius: 4 });
eq('snapResize n edge to guide', [srn.y, srn.h, srn.guides.y], [50, 27, [50]]);
const sro = G.snapResize({ x: 10, y: 10, w: 20, h: 13 }, 'se', { x: [], y: [] }, { step: 8, origin: { x: 2, y: 2 } });
eq('snapResize grid relative to the parent origin', [sro.x, sro.y, sro.w, sro.h], [10, 10, 24, 16]);
const srOff = G.snapResize({ x: 10, y: 10, w: 88, h: 21 }, 'se', targets, { step: 8, guides: false });
eq('snapResize without guides (Alt)', [srOff.w, srOff.h, srOff.guides], [86, 22, { x: [], y: [] }]);
const srMin = G.snapResize({ x: 98, y: 0, w: 6, h: 10 }, 'e', one, { step: 0, radius: 4 });
eq('snapResize lists no guide the clamped edge misses', [srMin.w, srMin.guides.x], [4, []]);
eq('snapResize caps sides at MAX_SIDE', G.snapResize({ x: 6, y: 0, w: 2048, h: 10 }, 'e', { x: [], y: [] }, { step: 8 }).w, 2048);
eq('align top several', G.align([{ x: 0, y: 10, w: 10, h: 10 }, { x: 20, y: 30, w: 10, h: 20 }, { x: 40, y: 5, w: 10, h: 5 }], 'top').map(r => r.y), [5, 5, 5]);
eq('align vcenter several', G.align([{ x: 0, y: 10, w: 10, h: 10 }, { x: 20, y: 30, w: 10, h: 20 }, { x: 40, y: 5, w: 10, h: 5 }], 'vcenter').map(r => r.y), [23, 18, 25]);
eq('align hcenter rounds odd gaps up', G.align([{ x: 0, y: 0, w: 10, h: 10 }, { x: 41, y: 0, w: 20, h: 10 }], 'hcenter').map(r => r.x), [26, 21]);
eq('align widget larger than its frame', G.align([{ x: 5, y: 5, w: 151, h: 10 }], 'hcenter', { x: 0, y: 0, w: 100, h: 50 })[0].x, -25);
eq('align empty list', [G.align([], 'left', { x: 0, y: 0, w: 320, h: 240 }), G.align([], 'top')], [[], []]);
eq('distribute fractional gap', G.distribute([{ x: 0, y: 0, w: 10, h: 10 }, { x: 20, y: 0, w: 10, h: 10 }, { x: 80, y: 0, w: 10, h: 10 }, { x: 101, y: 0, w: 10, h: 10 }], 'h').map(r => r.x), [0, 34, 67, 101]);
const crowded = [{ x: 0, y: 0, w: 10, h: 10 }, { x: 5, y: 0, w: 200, h: 10 }, { x: 50, y: 0, w: 10, h: 10 }];
eq('distribute leaves rects wider than their span unchanged', G.distribute(crowded, 'h'), crowded);
eq('distribute spaces overlapping rects that fit their span', G.distribute([{ x: 0, y: 0, w: 50, h: 10 }, { x: 10, y: 0, w: 50, h: 10 }, { x: 60, y: 0, w: 50, h: 10 }], 'h').map(r => r.x), [0, 30, 60]);
eq('resize grows a stored 0 size to at least 1', [G.resizeRect({ x: 5, y: 5, w: 0, h: 0 }, 'se', 0, 0), G.snapResize({ x: 5, y: 5, w: 0, h: 0 }, 'se', { x: [], y: [] }, { step: 0 })].map(r => [r.w, r.h]), [[1, 1], [1, 1]]);
eq('clampInto rect larger than the frame', G.clampInto({ x: 50, y: 50, w: 400, h: 300 }, { x: 0, y: 0, w: 320, h: 240 }), { x: 0, y: 0, w: 400, h: 300 });
eq('clampInto frame with an offset origin', [G.clampInto({ x: 0, y: 60, w: 20, h: 20 }, { x: 10, y: 20, w: 100, h: 50 }), G.clampInto({ x: 30, y: 30, w: 200, h: 10 }, { x: 10, y: 20, w: 100, h: 50 })],
    [{ x: 10, y: 50, w: 20, h: 20 }, { x: 10, y: 30, w: 200, h: 10 }]);

// ── store ──
function project() {
    return {
        format: 1, template_version: '1.0.0', name: 'Test', board: 'esp32-2432s028r', orientation: 'landscape', theme: 'night',
        language: 'de', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3',
        settings: { wifi: 'optional', aurago_link: false, libraries: [], backlight: 100, statusbar_clock: false },
        start_screen: 's_home',
        screens: [
            { id: 's_home', name: 'home', title: 'Start', transition: 'none', style: {}, widgets: [
                { id: 'w_btn', type: 'button', name: 'btn_1', x: 10, y: 10, w: 120, h: 40 },
                { id: 'w_pnl', type: 'panel', name: 'pnl_1', x: 150, y: 20, w: 160, h: 100, children: [
                    { id: 'w_lbl', type: 'label', name: 'lbl_1', x: 4, y: 4, w: 100, h: 24 }] },
                { id: 'w_tabs', type: 'tabview', name: 'tabs_1', x: 10, y: 130, w: 300, h: 100, children: [] }] },
            { id: 's_two', name: 'details', title: '', transition: 'fade', style: {}, widgets: [] }],
        assets: []
    };
}
let clock = 1000;
const store = M.createStore(project(), { limit: 300, coalesceMs: 1200, now: () => clock });
const seen = [];
const unsub = store.subscribe((p, info) => seen.push(info.origin + ':' + info.label));
check('store snapshot is frozen', Object.isFrozen(store.get()) && Object.isFrozen(store.get().screens[0].widgets[0]));
check('store starts without history', !store.canUndo() && !store.canRedo());
store.change('rename', d => { d.screens[0].title = 'A'; });
eq('change emits local', seen.pop(), 'local:rename');
check('change pushes history', store.canUndo());
clock += 500; store.change('type', d => { d.screens[0].title = 'AB'; }, { coalesce: 'title' });
clock += 500; store.change('type', d => { d.screens[0].title = 'ABC'; }, { coalesce: 'title' });
clock += 500; store.change('type', d => { d.screens[0].title = 'ABCD'; }, { coalesce: 'title' });
store.undo();
eq('coalesced typing is one undo step', store.get().screens[0].title, 'A');
eq('undo emits undo origin', seen.pop(), 'undo:type');
store.redo();
eq('redo restores', store.get().screens[0].title, 'ABCD');
eq('redo emits redo origin', seen.pop(), 'redo:type');
clock += 100; store.change('type', d => { d.screens[0].title = 'X'; }, { coalesce: 'title' });
store.undo();
eq('no coalescing across undo/redo', store.get().screens[0].title, 'ABCD');
store.redo();
clock += 5000; store.change('type', d => { d.screens[0].title = 'Y'; }, { coalesce: 'title' });
store.undo();
eq('coalescing window expires after 1200 ms', store.get().screens[0].title, 'X');
store.redo();
const before = store.get();
try { store.change('boom', d => { d.screens[0].title = 'broken'; throw new Error('nope'); }); } catch (err) { /* expected */ }
check('throwing change leaves the snapshot', store.get() === before && store.get().screens[0].title === 'Y');
const histBefore = seen.length;
store.change('noop', d => { d.screens[0].title = 'Y'; });
check('no-op change is not recorded', seen.length === histBefore);
store.replace(project(), { resetHistory: true });
eq('replace emits remote', seen.pop(), 'remote:replace');
check('replace with resetHistory clears history', !store.canUndo() && !store.canRedo());
const limited = M.createStore(project(), { limit: 3, now: () => 0 });
for (let i = 0; i < 5; i++) limited.change('n' + i, d => { d.name = 'v' + i; });
let undos = 0;
while (limited.undo()) undos++;
eq('history limit', [undos, limited.get().name], [3, 'v1']);
unsub();
store.change('after', d => { d.name = 'unsubscribed'; });
check('unsubscribe stops events', seen[seen.length - 1] !== 'local:after');
const ret = store.change('ret', d => { d.name = 'r'; return 42; });
eq('change returns the fn result', ret, 42);

// ── ops (structural mirror of Go ApplyOps, no validation gate) ──
const specs = M.specMap(caps.widgets);
const ctx = { specs, screenSize: { w: 320, h: 240 }, themes: caps.themes.map(t => t.id) };
const base = project();
function ops(list) { return M.applyOps(base, list, ctx); }
let r = ops([{ op: 'add_widget', screen: 'home', widget: { type: 'label', x: 1, y: 2, w: 50, h: 20 } }]);
eq('add_widget appends to the root with a new id', [r.project.screens[0].widgets.length, /^w_[0-9a-f]{12}$/.test(r.results[0].id)], [4, true]);
eq('add_widget result op', r.results[0].op, 'add_widget');
check('applyOps leaves the input unchanged', base.screens[0].widgets.length === 3);
r = ops([{ op: 'add_widget', screen: 's_home', parent: 'pnl_1', widget: { id: 'w_new', type: 'switch', x: 0, y: 0, w: 56, h: 28 } }]);
const added = r.project.screens[0].widgets[1].children[1];
check('add_widget into a panel by name; supplied ids are replaced by fresh ones', !!added && added.type === 'switch' && added.id !== 'w_new' && added.id === r.results[0].id);
r = ops([{ op: 'add_widget', screen: 's_home', widget: { type: 'panel', x: 0, y: 0, children: [{ id: 'w_lbl', type: 'label' }] } }]);
const pnl = r.project.screens[0].widgets[3];
eq('add_widget fills zero sizes from the catalog and renews child ids', [pnl.w, pnl.h, pnl.children[0].w, pnl.children[0].h, pnl.children[0].id !== 'w_lbl'], [160, 100, 120, 24, true]);
r = ops([{ op: 'add_widget', screen: 's_home', widget: { type: 'statusbar' } }]);
eq('statusbar default width is the parent width', [r.project.screens[0].widgets[3].w, r.project.screens[0].widgets[3].h], [320, 24]);
r = ops([{ op: 'add_widget', screen: 's_home', x: 7, y: 9, widget: { type: 'led', x: 1, y: 1 } }]);
eq('add_widget honours x/y overrides', [r.project.screens[0].widgets[3].x, r.project.screens[0].widgets[3].y], [7, 9]);
r = ops([{ op: 'add_widget', screen: 's_home', widget: { type: 'button', events: [{ on: 'clicked', action: { type: 'goto_screen', screen: 'details' } }] } }]);
eq('goto_screen targets given by name are stored as ids', r.project.screens[0].widgets[3].events[0].action.screen, 's_two');
r = ops([{ op: 'add_widget', screen: 's_home', parent: 'w_tabs', tab: 1, widget: { type: 'label', x: 0, y: 0, w: 50, h: 20 } }]);
eq('add_widget into a tabview page', r.project.screens[0].widgets[2].children[0].tab, 1);
r = ops([{ op: 'add_widget', screen: 's_home', parent: 'w_tabs', tab: 5, widget: { type: 'label' } }]);
eq('an out-of-range tab is stored (validation reports TAB_INVALID)', r.project.screens[0].widgets[2].children[0].tab, 5);
r = ops([{ op: 'add_widget', screen: 's_home', parent: 'w_pnl', tab: 0, widget: { type: 'label' } }]);
check('tab 0 under a panel is fine', !('tab' in r.project.screens[0].widgets[1].children[1]));
fails('a non-zero tab outside a tabview fails', () => ops([{ op: 'add_widget', screen: 's_home', parent: 'w_pnl', tab: 1, widget: { type: 'label' } }]), 'only allowed when the parent is a tabview');
fails('unknown screen', () => ops([{ op: 'add_widget', screen: 'nope', widget: { type: 'label' } }]), 'not found');
fails('unknown type', () => ops([{ op: 'add_widget', screen: 'home', widget: { type: 'gizmo' } }]), 'unknown widget type');
fails('parent not a container', () => ops([{ op: 'add_widget', screen: 'home', parent: 'btn_1', widget: { type: 'label' } }]), 'cannot contain widgets');
fails('parent missing', () => ops([{ op: 'add_widget', screen: 'home', parent: 'zzz', widget: { type: 'label' } }]), 'parent:');
const deep = { type: 'panel', x: 0, y: 0, w: 10, h: 10, children: [{ type: 'panel', x: 0, y: 0, w: 5, h: 5, children: [{ type: 'panel', x: 0, y: 0, w: 5, h: 5, children: [{ type: 'label', x: 0, y: 0, w: 5, h: 5 }] }] }] };
check('limits are validation issues, not op failures (nesting 5)', ops([{ op: 'add_widget', screen: 'home', parent: 'w_pnl', widget: deep }]).results.length === 1);
fails('atomic: a failing op fails the call', () => ops([{ op: 'add_widget', screen: 'home', widget: { type: 'label' } }, { op: 'delete_widget', id: 'missing' }]), 'not found');
try { ops([{ op: 'add_widget', screen: 'home', widget: { type: 'label' } }, { op: 'nope' }]); } catch (err) { eq('error details carry index and op', [err.details.index, err.details.op], [1, 'nope']); }
r = ops([{ op: 'update_widget', id: 'btn_1', set: { x: 20, props: { text: 'Go', symbol: 'ok' }, style: { radius: 4 }, locked: true } }]);
const ub = r.project.screens[0].widgets[0];
eq('update_widget merges props/style', [ub.x, ub.props, ub.style, ub.locked], [20, { text: 'Go', symbol: 'ok' }, { radius: 4 }, true]);
r = M.applyOps(r.project, [{ op: 'update_widget', id: 'w_btn', set: { props: { symbol: null }, style: { radius: null }, locked: false, name: '' } }], ctx);
const ub2 = r.project.screens[0].widgets[0];
eq('update_widget null removes keys, empty maps vanish', [ub2.props, 'style' in ub2, 'locked' in ub2, 'name' in ub2], [{ text: 'Go' }, false, false, false]);
r = M.applyOps(r.project, [{ op: 'update_widget', id: 'w_btn', set: { props: null, tab: 0 } }], ctx);
check('props null resets every prop', !('props' in r.project.screens[0].widgets[0]));
fails('update_widget cannot set type', () => ops([{ op: 'update_widget', id: 'w_btn', set: { type: 'label' } }]), 'cannot be changed');
fails('update_widget rejects unknown keys', () => ops([{ op: 'update_widget', id: 'w_btn', set: { colour: 1 } }]), 'unknown widget field');
fails('update_widget rejects wrong types', () => ops([{ op: 'update_widget', id: 'w_btn', set: { x: 'abc' } }]), 'must be an integer');
fails('update_widget rejects unknown style keys', () => ops([{ op: 'update_widget', id: 'w_btn', set: { style: { shadow: 1 } } }]), 'unknown style field');
fails('update_widget needs set', () => ops([{ op: 'update_widget', id: 'w_btn', set: {} }]), 'set is empty');
r = ops([{ op: 'move_widget', id: 'w_btn', x: 5, y: 6 }]);
eq('move_widget position keeps the parent and z-order', [r.project.screens[0].widgets[0].id, r.project.screens[0].widgets[0].x, r.project.screens[0].widgets[0].y], ['w_btn', 5, 6]);
r = ops([{ op: 'move_widget', id: 'w_btn', parent: 'w_pnl', x: 1, y: 2 }]);
eq('move_widget reparents into a panel (top of z-order)', [r.project.screens[0].widgets.map(w => w.id), r.project.screens[0].widgets[0].children.map(w => w.id)], [['w_pnl', 'w_tabs'], ['w_lbl', 'w_btn']]);
r = ops([{ op: 'move_widget', id: 'w_lbl', screen: 's_home', x: 3, y: 3 }]);
eq('move_widget with screen and no parent goes to the screen root', r.project.screens[0].widgets.map(w => w.id), ['w_btn', 'w_pnl', 'w_tabs', 'w_lbl']);
r = ops([{ op: 'move_widget', id: 'w_btn', parent: 'w_tabs', tab: 1 }]);
eq('move_widget into a tab', r.project.screens[0].widgets[1].children[0].tab, 1);
r = M.applyOps(r.project, [{ op: 'move_widget', id: 'w_btn', screen: 's_two', x: 0, y: 0 }], ctx);
eq('move_widget to another screen drops tab', [r.project.screens[1].widgets[0].id, 'tab' in r.project.screens[1].widgets[0]], ['w_btn', false]);
fails('move into itself', () => ops([{ op: 'move_widget', id: 'w_pnl', parent: 'w_pnl' }]), 'into itself');
fails('move needs x/y, screen or parent', () => ops([{ op: 'move_widget', id: 'w_btn', tab: 1 }]), 'needs x/y');
r = ops([{ op: 'delete_widget', id: 'w_pnl' }]);
check('delete_widget removes the subtree', !M.findWidget(r.project, 'w_lbl') && r.project.screens[0].widgets.length === 2);
r = ops([{ op: 'add_screen', screen_data: { id: 's_mine', name: 'settings' } }]);
const ns = r.project.screens[2];
eq('add_screen defaults and a fresh id', [ns.name, ns.transition, ns.title, ns.widgets, ns.id !== 's_mine' && /^s_/.test(ns.id)], ['settings', 'none', '', [], true]);
r = ops([{ op: 'update_screen', id: 'details', set: { name: 'info', transition: 'slide_left', style: { bg_color: '#112233' } } }]);
eq('update_screen', [r.project.screens[1].name, r.project.screens[1].transition, r.project.screens[1].style], ['info', 'slide_left', { bg_color: '#112233' }]);
r = ops([{ op: 'delete_screen', id: 's_home' }]);
eq('delete_screen moves the start screen', [r.project.screens.length, r.project.start_screen], [1, 's_two']);
fails('the designer keeps the last screen', () => M.applyOps(r.project, [{ op: 'delete_screen', id: 's_two' }], ctx), 'last screen');
r = ops([{ op: 'set_theme', theme: 'terminal' }, { op: 'set_settings', set: { wifi: 'off', libraries: ['dht'], backlight: 60 } }]);
eq('set_theme + set_settings', [r.project.theme, r.project.settings.wifi, r.project.settings.libraries, r.project.settings.backlight], ['terminal', 'off', ['dht'], 60]);
fails('set_theme needs a known theme', () => ops([{ op: 'set_theme', theme: 'neon' }]), 'unknown theme');
fails('set_settings accepts only settings keys (like Go)', () => ops([{ op: 'set_settings', set: { orientation: 'portrait' } }]), 'unknown settings field');
r = ops([{ op: 'set_project', set: { orientation: 'portrait', language: 'en', timezone: 'UTC0', start_screen: 's_two' } }]);
eq('set_project (designer-only)', [r.project.orientation, r.project.language, r.project.timezone, r.project.start_screen], ['portrait', 'en', 'UTC0', 's_two']);
r = ops([{ op: 'reorder_widget', id: 'w_tabs', index: 0 }, { op: 'reorder_screen', id: 's_two', index: 0 }]);
eq('reorder widget (z-order) and screen', [r.project.screens[1].widgets.map(w => w.id), r.project.screens.map(s => s.id)], [['w_tabs', 'w_btn', 'w_pnl'], ['s_two', 's_home']]);

// ── lookups, geometry in the tree ──
eq('contentBox panel is the whole box (like validate.go)', M.contentBox(base.screens[0].widgets[1], specs.get('panel')), { x: 0, y: 0, w: 160, h: 100 });
eq('contentBox tabview top bar', M.contentBox(base.screens[0].widgets[2], specs.get('tabview')), { x: 0, y: 40, w: 300, h: 60 });
eq('contentBox tabview left bar', M.contentBox({ type: 'tabview', x: 0, y: 0, w: 200, h: 100, props: { bar_position: 'left', bar_size: 30 } }, specs.get('tabview')), { x: 30, y: 0, w: 170, h: 100 });
eq('absRect of a panel child', M.absRect(base, 'w_lbl', { specs }), { x: 154, y: 24, w: 100, h: 24 });
eq('findWidget by name', M.findWidget(base, 'lbl_1').widget.id, 'w_lbl');
eq('findWidget ancestors', M.findWidget(base, 'w_lbl').ancestors.map(a => a.id), ['w_pnl']);
eq('topLevel drops selected descendants', M.topLevel(base, ['w_lbl', 'w_pnl', 'w_btn']), ['w_pnl', 'w_btn']);
eq('prune', M.prune(base, ['w_lbl', 'gone']), ['w_lbl']);
eq('propValue default and override', [M.propValue({ props: {} }, specs.get('slider'), 'value'), M.propValue({ props: { value: 7 } }, specs.get('slider'), 'value')], [50, 7]);

// ── names ──
eq('name rule', ['btn_1', 'a', 'A1', '1a', 'a-b', 'a'.repeat(32), 'a'.repeat(33), ''].map(M.validName), [true, true, false, false, false, true, false, false]);
eq('reserved names (validate.go)', ['init', 'goto', 'back', 'theme_colors', 'event_index', 'async', 'screen_t', 'transition_t', 'scr_home', 'show_x', 'hide_x', 'font_big', 'temp_series', 'btn_1', 'screen'].map(M.reservedName),
    [true, true, true, true, true, true, true, true, true, true, true, true, true, false, false]);
eq('img_<asset id> collides with an image symbol', [M.nameIssue(Object.assign({}, base, { assets: [{ id: 'a_sun', file: 'assets/sun.png', w: 8, h: 8, alpha: false }] }), 'img_a_sun', 'w_btn'), M.nameIssue(base, 'img_a_sun', 'w_btn')], ['NAME_INVALID', '']);
eq('nameIssue', [M.nameIssue(base, 'btn_1', 'w_lbl'), M.nameIssue(base, 'btn_1', 'w_btn'), M.nameIssue(base, 'Bad', 'w_btn'), M.nameIssue(base, '', 'w_btn'), M.nameIssue(base, 'show_more', 'w_btn')], ['NAME_DUPLICATE', '', 'NAME_INVALID', '', 'NAME_INVALID']);
eq('screenNameIssue', [M.screenNameIssue(base, 'home', 's_two'), M.screenNameIssue(base, 'x y', 's_two'), M.screenNameIssue(base, 'info', 's_two')], ['SCREEN_NAME_DUPLICATE', 'SCREEN_NAME_INVALID', '']);
eq('proposeName', [M.proposeName(base, 'button'), M.proposeName(base, 'label'), M.proposeName(base, 'slider'), M.proposeName(base, 'button', ['btn_2'])], ['btn_2', 'lbl_2', 'sld_1', 'btn_3']);
check('proposals never use reserved names', Object.keys(M.PREFIX).every(type => !M.reservedName(M.proposeName(base, type))));
eq('proposeScreenName', [M.proposeScreenName(base, 'home'), M.proposeScreenName(base, 'Bad Name'), M.proposeScreenName(base)], ['home_1', 'screen_1', 'screen_1']);
const copy = M.cloneSubtree(base, base.screens[0].widgets[1]);
check('cloneSubtree renews ids and names', copy.id !== 'w_pnl' && copy.name === 'pnl_2' && copy.children[0].id !== 'w_lbl' && copy.children[0].name === 'lbl_2');
const nw = M.newWidget(base, specs.get('statusbar'), { x: 40, y: 40 }, { w: 320, h: 240 });
eq('newWidget statusbar spans the screen', [nw.x, nw.y, nw.w, nw.h, nw.name], [0, 0, 320, 24, 'status_1']);

// ── render ──
check('renderer for every catalog type', caps.widgets.every(s => R.TYPES.includes(s.type)), R.TYPES.join(','));
eq('font sizes mirror the catalog (statusbar small font)', R.FONT_SIZES, caps.fonts.sizes);
for (const theme of caps.themes) {
    for (const spec of caps.widgets) {
        const w = { id: 'w_' + spec.type, type: spec.type, x: 0, y: 0, w: spec.default_w || 320, h: spec.default_h, props: spec.type === 'image' ? { asset: 'a_sun' } : {} };
        let out = '';
        try { out = R.html(R.tree(spec, w, theme, { assetUrl: id => '/assets/' + id, activeTab: 1 })); } catch (err) { out = 'ERROR ' + err.message; }
        check('render ' + theme.id + '/' + spec.type, out.startsWith('<div') && out.includes('cc-wv-' + spec.type) && !out.includes('undefined') && !out.includes('NaN'), out.slice(0, 200));
    }
}
const byStyle = style => caps.themes.find(t => t.button_style === style);
const filled = byStyle('filled');
const btn = R.html(R.tree(specs.get('button'), { type: 'button', w: 120, h: 40, props: { text: 'Go', symbol: 'ok' } }, filled, {}));
check('filled button uses accent + on_accent', btn.includes('background:' + filled.tokens.accent) && btn.includes('color:' + filled.tokens.on_accent) && btn.includes('✓') && btn.includes('Go'), btn);
const bracketTheme = byStyle('bracket');
const bracket = R.html(R.tree(specs.get('button'), { type: 'button', w: 120, h: 40, props: { text: 'RUN' } }, bracketTheme, {}));
check('bracket button', bracket.includes('[') && bracket.includes(']') && bracket.includes(R.FAMILIES[bracketTheme.font].split(',')[0]), bracket);
const outlineTheme = byStyle('outline');
const outline = R.html(R.tree(specs.get('button'), { type: 'button', w: 120, h: 40 }, outlineTheme, {}));
check('outline button border in accent', outline.includes('solid ' + outlineTheme.tokens.accent) && outline.includes('Button'), outline);
const night = caps.themes.find(t => t.id === 'night');
const styled = R.html(R.tree(specs.get('label'), { type: 'label', w: 100, h: 20, props: { text: 'T', size: 28, font: 'inter', align: 'center' }, style: { text_color: '#FF0000', bg_color: '#000000', bg_opacity: 128 } }, night, {}));
check('label overrides and font', styled.includes('color:#FF0000') && styled.includes('rgba(0,0,0,0.502)') && styled.includes('font-size:28px') && styled.includes("'Inter'") && styled.includes('text-align:center'), styled);
const img = R.html(R.tree(specs.get('image'), { type: 'image', w: 48, h: 48, props: { asset: 'a_sun' } }, night, { assetUrl: id => '/x/' + id }));
check('image uses assetUrl and pixelated rendering', img.includes('src="/x/a_sun"') && img.includes('image-rendering:pixelated'), img);
const tabs = R.html(R.tree(specs.get('tabview'), { type: 'tabview', w: 300, h: 200, props: { tabs: ['A', 'B'] } }, night, { activeTab: 1 }));
check('tabview bar with tab hooks', tabs.includes('data-cc-tab="0"') && tabs.includes('data-cc-tab="1"') && tabs.includes('>B<'), tabs);
const spin = R.html(R.tree(specs.get('spinbox'), { type: 'spinbox', w: 140, h: 40, props: { value: 1234, digits: 5, decimals: 2 } }, night, {}));
check('spinbox formats digits/decimals', spin.includes('012.34'), spin);
const sb = R.html(R.tree(specs.get('statusbar'), { type: 'statusbar', w: 320, h: 24, props: { title: 'Wetter' } }, night, {}));
check('statusbar title + clock', sb.includes('Wetter') && sb.includes('12:34'), sb);
const pw = R.html(R.tree(specs.get('textarea'), { type: 'textarea', w: 200, h: 40, props: { text: 'abc', password: true } }, night, {}));
check('textarea password bullets like the device (*)', pw.includes('***') && !pw.includes('abc'), pw);
const sys = R.html(R.wifiSetup(night, { w: 320, h: 240 }, k => k));
check('wifi setup system preview', sys.includes('cyd_creator.designer.system_wifi_title') && sys.includes('cc-system-screen'), sys.slice(0, 200));
const xss = R.html(R.tree(specs.get('label'), { type: 'label', w: 10, h: 10, props: { text: '<img src=x onerror=alert(1)>' } }, night, {}));
check('render escapes text', !xss.includes('<img') && xss.includes('&lt;img'), xss);
check('render source holds no hex colour literals', !/#[0-9a-fA-F]{6}\b/.test(fs.readFileSync(path.join(apps, 'cyd-creator-render.js'), 'utf8')));

console.log((failures ? 'FAILED ' + failures : 'ok') + ' — ' + checks + ' checks');
process.exit(failures ? 1 : 0);
