#!/usr/bin/env node
// Runs the pure CYD Creator designer modules (geometry, model, render) in Node and checks them:
// snapping and guides, resize/align/distribute maths, the layout store (history, coalescing,
// origins), every layout op incl. reparenting and tabs (structural mirror of Go ApplyOps), name
// rules incl. reserved names, a render smoke test of all 26 widget types in every theme through
// the string serialiser, the interaction module (gestures, palette drag, keyboard) through the
// real designer and store on a minimal fake DOM, and the designer's shell wiring with the real
// canvas on that fake DOM (fit zoom, one render per change, flashes, live region). Capabilities come from
// ui/testdata/cyd_creator_designer_caps.json (kept equal to internal/cydcreator by
// TestCYDCreatorDesignerCapsFixtureMatchesCatalog). P.util comes from the real cyd-creator-core.js
// (plan 4a), which runs in Node and loads first.
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
{
    // A remote replace ends coalescing: the next edit with the same key is a step of its own.
    let t = 0;
    const s = M.createStore(project(), { now: () => t });
    s.change('type', d => { d.name = 'A'; }, { coalesce: 'name' });
    s.replace(Object.assign({}, s.get(), { name: 'R' }), { resetHistory: false });
    t += 100;
    s.change('type', d => { d.name = 'B'; }, { coalesce: 'name' });
    s.undo();
    eq('a replace ends coalescing', s.get().name, 'R');
}
{
    // change() caches the JSON of the current snapshot for its no-op check; undo and redo must
    // refresh it, so an edit back to the state before the undo (or redo) is recorded.
    const s = M.createStore(project(), { now: () => 0 });
    s.change('a', d => { d.name = 'A'; });
    s.undo();
    s.change('a', d => { d.name = 'A'; });
    const afterUndo = [s.get().name, s.canUndo(), s.canRedo()];
    s.undo();
    s.change('noop', d => { d.name = 'Test'; });
    s.redo();
    s.change('t', d => { d.name = 'Test'; });
    eq('an edit back to the state before an undo or redo is recorded', [afterUndo, s.get().name, s.canUndo()], [['A', true, false], 'Test', true]);
}
{
    // An upload replaces the store with resetHistory false: undo goes back before it (the asset is
    // dropped again), redo re-adds it.
    const s = M.createStore(project(), { now: () => 0 });
    s.change('rename', d => { d.name = 'Edited'; });
    s.replace(Object.assign({}, s.get(), { assets: [{ id: 'a_sun', file: 'assets/sun.png', w: 8, h: 8, alpha: false }] }), { resetHistory: false });
    s.undo();
    const undone = [s.get().name, s.get().assets.length, s.canRedo()];
    s.redo();
    eq('undo across an upload replace drops the asset, redo re-adds it', [undone, s.get().name, s.get().assets.map(a => a.id)], [['Test', 0, true], 'Edited', ['a_sun']]);
}
{
    // emit works on a copy of the subscriber set: a subscriber removed during an emit still gets
    // that event, one added during an emit gets only later events.
    const s = M.createStore(project(), { now: () => 0 });
    const calls = [];
    let offB = null;
    s.subscribe((p, info) => {
        calls.push('A:' + info.label);
        if (info.label === 'one') { offB(); s.subscribe((q, i2) => calls.push('C:' + i2.label)); }
    });
    offB = s.subscribe((p, info) => calls.push('B:' + info.label));
    s.change('one', d => { d.name = '1'; });
    s.change('two', d => { d.name = '2'; });
    eq('subscribe/unsubscribe during an emit take effect from the next event', calls, ['A:one', 'B:one', 'A:two', 'C:two']);
}
{
    // The history is bounded by bytes as well as by count: a large layout (about 0.5 MB of JSON,
    // near the 1 MiB server maximum) keeps HISTORY_MAX_BYTES / size steps, and undo/redo work
    // across all of them.
    const big = project();
    big.name = 'v----';
    big.screens = [];
    for (let s = 0; s < 16; s++) {
        const widgets = [];
        for (let i = 0; i < 200; i++) widgets.push({ id: 'w_' + String(s * 1000 + i).padStart(12, '0'), type: 'button', name: 'btn_' + (s * 1000 + i), x: i % 300, y: i % 200, w: 120, h: 40, props: { text: 'Button ' + i, symbol: 'ok' }, style: { radius: 4 } });
        big.screens.push({ id: 's_' + String(s).padStart(12, '0'), name: 'scr' + s, title: 'Screen ' + s, transition: 'none', style: {}, widgets });
    }
    const size = JSON.stringify(big).length;
    const fit = Math.floor(M.HISTORY_MAX_BYTES / size);
    const s = M.createStore(big, { now: () => 0 });
    const n = fit + 10;
    for (let i = 0; i < n; i++) s.change('n', d => { d.name = 'v' + String(i).padStart(4, '0'); });
    let steps = 0;
    while (s.undo()) steps++;
    const oldest = s.get().name;
    let redone = 0;
    while (s.redo()) redone++;
    eq('a large layout keeps the history within HISTORY_MAX_BYTES and undo/redo work across it',
        [fit > 10 && fit < 300, steps, redone, oldest, s.get().name], [true, fit, fit, 'v' + String(n - 1 - fit).padStart(4, '0'), 'v' + String(n - 1).padStart(4, '0')]);
    s.change('n', d => { d.name = 'vnext'; });
    let again = 0;
    while (s.undo()) again++;
    eq('undo/redo keep the byte count: the next change trims back to the same bound', again, fit);
    let count = 0;
    const tiny = M.createStore(project(), { maxBytes: 1, now: () => 0 });
    for (let i = 0; i < 3; i++) tiny.change('n', d => { d.name = 'n' + i; });
    while (tiny.undo()) count++;
    eq('the newest step stays even when one snapshot exceeds maxBytes', [count, tiny.get().name], [1, 'n1']);
}

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
fails('add_widget rejects a fractional x override', () => ops([{ op: 'add_widget', screen: 'home', x: 1.5, widget: { type: 'label' } }]), 'x must be an integer');
fails('add_widget rejects fractional widget sizes', () => ops([{ op: 'add_widget', screen: 'home', widget: { type: 'label', x: 0, y: 0, w: 10.5, h: 20 } }]), 'w must be an integer');
fails('add_widget rejects non-integer child geometry', () => ops([{ op: 'add_widget', screen: 'home', widget: { type: 'panel', children: [{ type: 'label', h: '24' }] } }]), 'h must be an integer');
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

// ── render: untrusted layout values ──
// Colours reach CSS only as #RRGGBB (validate.go colorPattern). Hostile strings go into every
// colour/enum/font/symbol/asset prop of every catalog widget and into every style override; no
// output may contain "url(" or the raw string.
const hostile = ['url(https://attacker.example/beacon)', 'red;background:url(x)', 'expression(alert(1))', 'javascript:alert(1)', '#12345', '#1234567', 'rgb(1,2,3)', '#FFFFFF;background:url(x)', ' #FFFFFF'];
const guardedKinds = new Set(['color', 'enum', 'font', 'font_size', 'symbol', 'asset']);
const leaks = [];
for (const spec of caps.widgets) {
    for (const bad of hostile) {
        const props = Object.fromEntries(spec.props.filter(p => guardedKinds.has(p.kind)).map(p => [p.name, bad]));
        const style = { bg_color: bad, text_color: bad, accent_color: bad, border_color: bad, bg_opacity: bad, border_width: bad, radius: bad };
        const out = R.html(R.tree(spec, { type: spec.type, w: spec.default_w || 320, h: spec.default_h, props, style }, night, { assetUrl: id => '/a/' + encodeURIComponent(id) }));
        if (out.includes('url(') || out.includes(bad)) leaks.push(spec.type + ': ' + bad);
    }
}
check('hostile colours, enums and style values never reach the output', leaks.length === 0, leaks.slice(0, 6).join(' | '));
check('rgba/colors accept only #RRGGBB', R.rgba('url(x)', 1) === 'transparent' && R.rgba('#FF0000', 0.5) === 'rgba(255,0,0,0.5)' && R.colors(night, { text_color: 'red;x:url(y)', accent_color: '#ABCDEF' }).text === night.tokens.text && R.colors(night, { accent_color: '#ABCDEF' }).accent === '#ABCDEF');

// Numbers clamp to the catalog min/max, lists to max_len items; wrong types use the default.
const extremes = [1e9, -1e9, NaN, Infinity, -Infinity, 'x', '1e9', null, {}, [], true];
const huge = Array.from({ length: 100000 }, (_, i) => 'item ' + i);
const problems = [];
const started = Date.now();
for (const spec of caps.widgets) {
    for (const v of extremes) {
        const props = Object.fromEntries(spec.props.map(p => [p.name, p.kind === 'list' && typeof v === 'number' ? huge : v]));
        for (const geo of [[spec.default_w || 320, spec.default_h], [v, v]]) {
            let out = '';
            try {
                out = R.html(R.tree(spec, { type: spec.type, w: geo[0], h: geo[1], props, style: { radius: v, border_width: v, bg_opacity: v } }, night, { assetUrl: id => '/a/' + encodeURIComponent(id), activeTab: v, screenTitle: v, settings: v }));
            } catch (err) { problems.push(spec.type + ' ' + String(v) + ' threw ' + err.message); continue; }
            if (/NaN|undefined|Infinity/.test(out) || out.length > 300000) problems.push(spec.type + ' ' + String(v) + ' → ' + out.slice(0, 120));
        }
    }
}
const elapsed = Date.now() - started;
check('extreme numbers, wrong types and huge lists render without errors', problems.length === 0, problems.slice(0, 5).join(' | '));
check('extreme values render within the time budget (< 2 s)', elapsed < 2000, elapsed + ' ms');
const ints = type => Object.fromEntries(specs.get(type).props.filter(p => p.kind === 'int').map(p => [p.name, 1e9]));
const count = (out, tag) => out.split('<' + tag + ' ').length - 1;
eq('table clamps rows × columns to 32 × 8 cells', R.tree(specs.get('table'), { type: 'table', w: 200, h: 120, props: ints('table') }, night, {}).children.length, 256);
eq('roller clamps visible_rows to 7', R.tree(specs.get('roller'), { type: 'roller', w: 120, h: 100, props: ints('roller') }, night, {}).children.length, 7);
eq('gauge clamps ticks to 41 (+ needle)', count(R.html(R.tree(specs.get('gauge'), { type: 'gauge', w: 140, h: 140, props: ints('gauge') }, night, {})), 'line'), 42);
eq('chart clamps points to 100', count(R.html(R.tree(specs.get('chart'), { type: 'chart', w: 200, h: 120, props: ints('chart') }, night, {})), 'circle'), 100);
check('spinbox clamps digits to 8', R.html(R.tree(specs.get('spinbox'), { type: 'spinbox', w: 140, h: 40, props: { digits: 1e9, value: 5 } }, night, {})).includes('>00000005<'));
eq('lists keep at most max_len items', R.tree(specs.get('list'), { type: 'list', w: 200, h: 160, props: { items: huge } }, night, {}).children.length, 64);

// Deviation behaviours of the device previews.
// keypadMap expectations copied from internal/cydcreator/generate_widgets.go (keypadMap): the
// numeric/numeric_ok maps; any other layout uses the keys: "\n" breaks rows, without breaks more
// than four keys make rows of four, an empty key becomes " ".
const keyRows = props => R.tree(specs.get('keypad'), { type: 'keypad', w: 200, h: 160, props }, night, {}).children.map(r => r.children.map(k => k.children.join('')));
eq('keypad numeric map', keyRows({ layout: 'numeric' }), [['1', '2', '3'], ['4', '5', '6'], ['7', '8', '9'], ['.', '0', '⌫']]);
eq('keypad numeric_ok map', keyRows({ layout: 'numeric_ok' }), [['1', '2', '3'], ['4', '5', '6'], ['7', '8', '9'], ['⌫', '0', 'OK']]);
eq('keypad custom: rows of four', keyRows({ layout: 'custom', keys: ['A', 'B', 'C', 'D', 'E', 'F'] }), [['A', 'B', 'C', 'D'], ['E', 'F']]);
eq('keypad custom: four keys stay one row', keyRows({ layout: 'custom', keys: ['A', 'B', 'C', 'D'] }), [['A', 'B', 'C', 'D']]);
eq('keypad custom: line breaks win', keyRows({ layout: 'custom', keys: ['1', '\n', '2', '3', '4', '5', '6'] }), [['1'], ['2', '3', '4', '5', '6']]);
eq('keypad custom: empty key is a space', keyRows({ layout: 'custom', keys: ['', 'x'] }), [[' ', 'x']]);
eq('keypad unknown layout uses the keys', keyRows({ layout: 'weird', keys: ['a', 'b'] }), [['a', 'b']]);
const spinOut = props => R.html(R.tree(specs.get('spinbox'), { type: 'spinbox', w: 140, h: 40, props }, night, {}));
check('spinbox sign only with a negative minimum', spinOut({ min: -10, value: 5 }).includes('>+005<') && spinOut({ min: -10, value: -5 }).includes('>-005<') && spinOut({ value: 5 }).includes('>005<'));
const sbOut = (props, opts) => R.html(R.tree(specs.get('statusbar'), { type: 'statusbar', w: 320, h: 24, props }, night, opts));
check('statusbar falls back to the screen title', sbOut({}, { screenTitle: 'Home' }).includes('>Home<') && sbOut({ title: 'Own' }, { screenTitle: 'Home' }).includes('>Own<'));
const sbOff = sbOut({}, { settings: { wifi: 'off', statusbar_clock: false } });
const sbOn = sbOut({}, { settings: { wifi: 'optional', statusbar_clock: true } });
check('statusbar follows the Wi-Fi and clock settings', !sbOff.includes('12:34') && !sbOff.includes(R.SYMBOLS.wifi) && sbOn.includes('12:34') && sbOn.includes(R.SYMBOLS.wifi), sbOff + ' / ' + sbOn);
const clearTheme = caps.themes.find(t => t.id === 'clear');
const lblSquare = R.html(R.tree(specs.get('label'), { type: 'label', w: 80, h: 20, style: { bg_color: '#000000' } }, clearTheme, {}));
const lblOwn = R.html(R.tree(specs.get('label'), { type: 'label', w: 80, h: 20, style: { bg_color: '#000000', radius: 5 } }, clearTheme, {}));
const imgSquare = R.html(R.tree(specs.get('image'), { type: 'image', w: 48, h: 48, props: { asset: 'a_sun' } }, clearTheme, { assetUrl: id => '/x/' + id }));
check('label and image stay square unless the radius is overridden', clearTheme.tokens.radius > 0 && lblSquare.includes('border-radius:0px') && lblOwn.includes('border-radius:5px') && imgSquare.includes('border-radius:0px'), lblSquare);
const vBar = R.html(R.tree(specs.get('bar'), { type: 'bar', w: 20, h: 160, props: { value: 30 } }, night, {}));
const hBar = R.html(R.tree(specs.get('bar'), { type: 'bar', w: 160, h: 20, props: { value: 30 } }, night, {}));
const vSlider = R.html(R.tree(specs.get('slider'), { type: 'slider', w: 24, h: 160, props: { value: 25 } }, night, {}));
check('bar fills from the bottom when taller than wide', vBar.includes('bottom:0;height:30%') && hBar.includes('width:30%;height:100%'), vBar);
check('slider runs vertically when taller than wide', vSlider.includes('height:25%') && vSlider.includes('top:calc(75% - 17px)'), vSlider);

// ── interact (cyd-creator-interact.js) through the real designer on a minimal fake DOM ──
// The designer mounts a stand-in canvas (records previews and re-renders); interact gets real
// pointer and key events dispatched with bubbling, and every edit must reach the real store.
class El {
    constructor(tag, data) {
        this.tagName = tag.toUpperCase();
        this.dataset = Object.assign({}, data);
        this.attrs = {};
        this.parent = null;
        this.kids = [];
        this.listeners = {};
        this.captured = new Set();
        this.clientWidth = 600;
        this.clientHeight = 400;
        const cls = new Set();
        this.classList = { add: n => cls.add(n), remove: n => cls.delete(n), contains: n => cls.has(n), toggle: (n, on) => { const want = on === undefined ? !cls.has(n) : !!on; if (want) cls.add(n); else cls.delete(n); return want; } };
        this.style = { setProperty(k, v) { this[k] = String(v); } };
    }
    appendChild(c) { if (c.parent) c.remove(); c.parent = this; this.kids.push(c); return c; }
    replaceChildren(...nodes) { this.kids.slice().forEach(k => k.remove()); nodes.forEach(n => this.appendChild(n)); }
    replaceChild(next, old) {
        if (next.parent) next.remove();
        const i = this.kids.indexOf(old);
        if (i < 0) throw new Error('fake DOM: replaceChild of a non-child');
        this.kids[i] = next; next.parent = this; old.parent = null;
        return old;
    }
    remove() { if (this.parent) this.parent.kids.splice(this.parent.kids.indexOf(this), 1); this.parent = null; }
    // data-* attributes show in dataset, as in a browser.
    setAttribute(k, v) { this.attrs[k] = String(v); if (k.startsWith('data-')) this.dataset[k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase())] = String(v); }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter(f => f !== fn); }
    setPointerCapture(id) { this.captured.add(id); }
    hasPointerCapture(id) { return this.captured.has(id); }
    releasePointerCapture(id) { this.captured.delete(id); }
    focus() { fakeDoc.activeElement = this; }
    contains(n) { for (let x = n; x; x = x.parent) if (x === this) return true; return false; }
    // Selectors: tag, [attr], [attr="v"], [attr]:not([attr="v"]), comma lists.
    matches(sel) {
        return sel.split(',').some(part => {
            const m = /^([a-z]+)?(?:\[([a-z-]+)(?:="([^"]*)")?\])?(?::not\(\[([a-z-]+)="([^"]*)"\]\))?$/.exec(part.trim());
            if (!m) throw new Error('fake DOM: unsupported selector ' + part);
            const [, tag, attr, want, notAttr, notValue] = m;
            const value = a => (a.startsWith('data-') ? this.dataset[a.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase())] : this.attrs[a]);
            if (tag && this.tagName !== tag.toUpperCase()) return false;
            if (attr && (value(attr) === undefined || (want !== undefined && value(attr) !== want))) return false;
            return !(notAttr && value(notAttr) === notValue);
        });
    }
    closest(sel) { for (let x = this; x; x = x.parent) if (x.matches(sel)) return x; return null; }
}
const fakeDoc = { activeElement: null, body: new El('body'), createElement: tag => new El(tag), createElementNS: (ns, tag) => new El(tag), createTextNode: text => Object.assign(new El('#text'), { text }) };
const winListeners = {};
sandbox.document = fakeDoc;
sandbox.window.addEventListener = (type, fn) => { (winListeners[type] = winListeners[type] || []).push(fn); };
sandbox.window.removeEventListener = (type, fn) => { winListeners[type] = (winListeners[type] || []).filter(f => f !== fn); };
// fire dispatches a bubbling event from el; it returns the event (defaultPrevented, stopped).
function fire(el, type, props) {
    const ev = Object.assign({ type, target: el, button: 0, buttons: 1, pointerId: 1, pointerType: 'mouse', clientX: 0, clientY: 0, defaultPrevented: false, stopped: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, props);
    for (let x = el; x && !ev.stopped; x = x.parent) (x.listeners[type] || []).slice().forEach(fn => fn(ev));
    return ev;
}
for (const file of ['cyd-creator-designer.js', 'cyd-creator-interact.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
{
    const layout = project();
    layout.screens = [{ id: 's_one', name: 'one', title: 'One', transition: 'none', style: {}, widgets: [
        { id: 'w_lbl', type: 'label', name: 'lbl_1', x: 10, y: 10, w: 80, h: 20 },
        { id: 'w_pnl', type: 'panel', name: 'pnl_1', x: 100, y: 100, w: 120, h: 80 },
        { id: 'w_btn', type: 'button', name: 'btn_9', x: 200, y: 20, w: 100, h: 40 }] }];
    layout.start_screen = 's_one';
    const shellBus = P.createBus();
    const iStore = M.createStore(layout, { coalesceMs: 1200 });
    const origins = [];
    iStore.subscribe((p, info) => origins.push(info.origin));
    const inst = { t: P.i18n(null), caps, readonly: false, isLocked: () => false, layoutStore: iStore, bus: shellBus, notify() {}, assetUrl: id => '/a/' + id };
    const designer = P.designer.create(inst);
    const d = designer.state;
    const root = new El('div');
    const viewport = root.appendChild(new El('div'));
    const screenEl = viewport.appendChild(new El('div'));
    const input = root.appendChild(new El('input'));
    const button = root.appendChild(new El('button'));
    const outside = new El('textarea');
    const els = {};
    ['w_lbl', 'w_pnl', 'w_btn'].forEach(id => { els[id] = screenEl.appendChild(new El('div', { ccWidgetId: id })); });
    const handle = viewport.appendChild(new El('div', { ccHandle: 'se' }));
    const seen = { render: 0, moves: [], resizes: [], drops: [] };
    // The stand-in canvas: host at client (0, 0); world = client / zoom.
    P.designerCanvas = { mount: () => ({
        root, viewport,
        render() { seen.render++; }, fit() { seen.fit = (seen.fit || 0) + 1; }, dispose() { seen.disposed = true; },
        previewMove(ids, delta) { seen.moves.push(delta); }, previewResize(id, r) { seen.resizes.push(r); },
        showGuides() {}, showMarquee() {}, showDrop(r) { seen.drops.push(r); }, announce() {},
        hostRect: () => ({ left: 0, top: 0, right: 320 * (d.zoom || 1), bottom: 240 * (d.zoom || 1) }),
        clientToWorld: (x, y) => G.toWorld({ x, y }, { left: 0, top: 0 }, d.zoom || 1)
    }) };
    designer.mountCanvas(new El('div'));
    d.setZoom(1);
    d.setSnap(0);
    const widget = id => M.findWidget(iStore.get(), id);
    const key = (k, extra, target) => fire(target || root, 'keydown', Object.assign({ key: k }, extra));
    // drag presses on el at from (client px), moves to to and ends with type (pointerup by default).
    function drag(el, from, to, opts) {
        const o = Object.assign({ altKey: true, end: 'pointerup' }, opts);
        fire(el, 'pointerdown', { clientX: from[0], clientY: from[1] });
        fire(viewport, 'pointermove', { clientX: to[0], clientY: to[1], altKey: o.altKey, shiftKey: !!o.shiftKey });
        if (o.end) fire(viewport, o.end, { clientX: to[0], clientY: to[1] });
    }

    drag(els.w_lbl, [20, 15], [43, 15]);
    eq('move commits the previewed delta', [widget('w_lbl').widget.x, widget('w_lbl').widget.y, seen.moves.pop()], [33, 10, { x: 23, y: 0 }]);
    check('move releases the pointer capture and the dragging state', !viewport.captured.size && !root.classList.contains('is-dragging'));
    iStore.undo();
    eq('a move is one undo step', widget('w_lbl').widget.x, 10);
    d.setZoom(2);
    drag(els.w_lbl, [40, 30], [45, 30]);
    eq('pointer deltas are rounded to device pixels at zoom 2', widget('w_lbl').widget.x, 13);
    d.setZoom(1);
    d.setSnap(8);
    drag(els.w_lbl, [20, 15], [50, 15], { altKey: false });
    eq('a move without Alt snaps to the 8 px grid (x) and the button top guide (y)', [widget('w_lbl').widget.x, widget('w_lbl').widget.y], [40, 10]);
    d.setSnap(0);
    iStore.undo();
    iStore.undo();
    eq('undo restores the start position', widget('w_lbl').widget.x, 10);

    const before = JSON.stringify(iStore.get());
    const aborts = [
        ['readonly:changed', () => shellBus.emit('readonly:changed', true)],
        ['lock:changed', () => shellBus.emit('lock:changed', true, 'ccj_1')],
        ['mode:changed code', () => shellBus.emit('mode:changed', 'code')],
        ['window blur', () => (winListeners.blur || []).forEach(fn => fn({}))],
        ['a remote replace', () => iStore.replace(JSON.parse(before), { resetHistory: false })]
    ];
    for (const [name, emit] of aborts) {
        const renders = seen.render;
        drag(els.w_lbl, [20, 15], [60, 15], { end: null });
        check(name + ': a gesture runs before', viewport.captured.has(1) && root.classList.contains('is-dragging'));
        emit();
        fire(viewport, 'pointerup', { clientX: 60, clientY: 15 });
        check(name + ' cancels the gesture: capture released, stored layout re-rendered, nothing saved', !viewport.captured.size && !root.classList.contains('is-dragging') && seen.render > renders && JSON.stringify(iStore.get()) === before);
    }
    drag(els.w_lbl, [20, 15], [60, 15], { end: 'pointercancel' });
    check('pointercancel changes nothing', JSON.stringify(iStore.get()) === before && !viewport.captured.size);

    d.select(['w_btn']);
    drag(els.w_btn, [210, 30], [130, 120]);
    const into = widget('w_btn');
    eq('dropping on a panel reparents with coordinates in the panel', [into.parent && into.parent.id, into.widget.x, into.widget.y], ['w_pnl', 20, 10]);
    iStore.undo();
    eq('reparent is one undo step', [widget('w_btn').parent, widget('w_btn').widget.x], [null, 200]);

    d.select(['w_lbl']);
    drag(handle, [90, 30], [103, 37]);
    eq('resize commits integers through update_widget', ['x', 'y', 'w', 'h'].map(k => widget('w_lbl').widget[k]), [10, 10, 93, 27]);
    d.setZoom(2);
    drag(handle, [206, 74], [211, 74]);
    eq('resize rounds the pointer delta at zoom 2', widget('w_lbl').widget.w, 96);
    d.setZoom(1);

    d.select([]);
    drag(viewport, [5, 5], [95, 35], { altKey: false });
    eq('box select picks the root widgets it touches', d.selection, ['w_lbl']);
    fire(viewport, 'pointerdown', { clientX: 5, clientY: 5, offsetX: 600, offsetY: 5 });
    check('a press on the viewport scrollbar neither starts a gesture nor clears', !viewport.captured.size && d.selection.join() === 'w_lbl');

    // keyboard
    const x0 = widget('w_lbl').widget.x;
    const steps = [];
    for (let i = 0; i < 5; i++) steps.push(key('ArrowRight'));
    check('arrow keys are used and stop at the canvas', steps.every(ev => ev.defaultPrevented && ev.stopped));
    eq('five nudges move 5 px', widget('w_lbl').widget.x, x0 + 5);
    key('ArrowDown', { shiftKey: true });
    eq('Shift+arrow nudges 8 px', widget('w_lbl').widget.y, 18);
    key('z', { ctrlKey: true });
    eq('a nudge burst is one undo step', [widget('w_lbl').widget.x, widget('w_lbl').widget.y], [x0, 10]);
    key('z', { ctrlKey: true, shiftKey: true });
    eq('Ctrl+Shift+Z redoes', [widget('w_lbl').widget.x, widget('w_lbl').widget.y], [x0 + 5, 18]);
    const typed = key('Delete', {}, input);
    check('keys typed into a field stay native', !typed.defaultPrevented && !!widget('w_lbl'));
    check('Space and Enter on a button stay native', !key(' ', {}, button).defaultPrevented && !key('Enter', {}, button).defaultPrevented);
    check('keys of other windows and the code editor never reach the designer', !key('Delete', {}, outside).defaultPrevented && !!widget('w_lbl'));
    check('Ctrl+S bubbles on to the shell', !key('s', { ctrlKey: true }).defaultPrevented);
    viewport.clientWidth = 0;
    check('a hidden canvas (Code mode) uses no keys', !key('Delete').defaultPrevented && !!widget('w_lbl'));
    viewport.clientWidth = 600;
    drag(els.w_lbl, [20, 15], [60, 15], { end: null });
    check('during a gesture other keys wait', !key('Delete').defaultPrevented && !!widget('w_lbl'));
    check('Escape cancels a gesture', key('Escape').defaultPrevented && !viewport.captured.size);
    check('Escape clears the selection', key('Escape').defaultPrevented && d.selection.length === 0);
    check('Escape without a selection bubbles on', !key('Escape').defaultPrevented);
    d.select(['w_lbl']);
    key('Delete');
    check('Delete removes the selection', !widget('w_lbl'));
    key('+');
    eq('+ zooms in', d.zoom, 2);
    key('0');
    check('0 fits the screen', seen.fit === 1);

    // palette: a press adds at the centre, a drag drops into the container under the pointer.
    const item = new El('button', { ccPalette: 'button' });
    const press = { clientX: 5, clientY: 5, pointerId: 2, button: 0, buttons: 1, pointerType: 'mouse', preventDefault() {} };
    const sample = M.newWidget(iStore.get(), specs.get('button'), { x: 0, y: 0 }, d.screenSize());
    d.interact.startPaletteDrag('button', Object.assign({ target: item }, press), item);
    fire(item, 'pointerup', { pointerId: 2 });
    const added = widget(d.selection[0]);
    eq('a palette press adds the catalog default with a proposed name', [added.widget.name, added.widget.w, added.widget.h, added.parent], [sample.name, specs.get('button').default_w, specs.get('button').default_h, null]);
    d.interact.startPaletteDrag('statusbar', Object.assign({ target: item }, press), item);
    fire(item, 'pointermove', { pointerId: 2, clientX: 700, clientY: 300 });
    const ghost = fakeDoc.body.kids[0];
    eq('the palette ghost has the new widget\'s default size (statusbar: screen width, zoom 2)', [ghost && ghost.className, ghost && ghost.style.width, ghost && ghost.classList.contains('is-outside')], ['cc-drag-ghost', '640px', true]);
    fire(item, 'pointerup', { pointerId: 2, clientX: 700, clientY: 300 });
    check('a drop outside the screen adds nothing and removes the ghost', !fakeDoc.body.kids.length && !M.findWidget(iStore.get(), 'status_1'));
    d.setZoom(1);
    d.interact.startPaletteDrag('label', Object.assign({ target: item }, press), item);
    fire(item, 'pointermove', { pointerId: 2, clientX: 140, clientY: 130 });
    fire(item, 'pointerup', { pointerId: 2, clientX: 140, clientY: 130 });
    const dropped = widget(d.selection[0]);
    check('a palette drop adds into the panel under the pointer', dropped && dropped.widget.type === 'label' && dropped.parent && dropped.parent.id === 'w_pnl', JSON.stringify(dropped && dropped.widget));
    // The one remote origin is the test's own replace above.
    check('every designer edit reached the store with origin local (undo/redo for history)', origins.includes('local') && origins.filter(o => !['local', 'undo', 'redo'].includes(o)).join() === 'remote', origins.join());

    drag(els.w_pnl, [110, 110], [150, 110], { end: null });
    designer.dispose();
    check('dispose cancels a running gesture and releases the capture', !viewport.captured.size && seen.disposed && !(root.listeners.keydown || []).length && !(viewport.listeners.pointerdown || []).length);
}

// ── designer wiring and the real canvas (cyd-creator-designer.js, cyd-creator-canvas.js) ──
// The canvas renders into the fake DOM with interact stubbed and manual timers: shell bus events,
// issue de-duplication, read-only, the selection after remote replaces, the fit zoom, one render
// per change, flashes across re-renders, the live region and a bounded number of timers.
{
    const timers = new Map();
    let timerSeq = 0;
    sandbox.setTimeout = fn => { timers.set(++timerSeq, fn); return timerSeq; };
    sandbox.clearTimeout = id => { timers.delete(id); };
    // runTimers fires the pending timers and the ones they start (the live region's second step).
    const runTimers = () => { for (let round = 0; round < 4 && timers.size; round++) Array.from(timers.entries()).forEach(([id, fn]) => { timers.delete(id); fn(); }); };
    const canvasFile = path.join(apps, 'cyd-creator-canvas.js');
    vm.runInContext(fs.readFileSync(canvasFile, 'utf8'), sandbox, { filename: canvasFile });
    const landscape = G.screenSize(caps.boards[0], 'landscape');
    eq('fitZoom counts stage padding, bezel and label unscaled (800×600 → 2×, 1400×900 → 3×)', [P.designerCanvas.fitZoom(landscape, 800, 600), P.designerCanvas.fitZoom(landscape, 1400, 900)], [2, 3]);

    let aborts = 0;
    const realInteract = P.designerInteract;
    P.designerInteract = { attach: () => ({ abort() { aborts++; }, dispose() {} }) };
    const layout = project();
    layout.settings.wifi = 'off';
    layout.settings.statusbar_clock = true;
    layout.screens[0].title = 'Home Title';
    layout.screens[0].style = { bg_color: 'url(https://example.invalid/x.png)', bg_opacity: 128 };
    layout.screens[0].widgets.unshift({ id: 'w_bar', type: 'statusbar', name: 'status_1', x: 0, y: 0, w: 320, h: 24 });
    layout.screens[0].widgets.push({ id: 'w_odd"\n]', type: 'label', name: 'lbl_odd', x: 'x', y: 1e9, w: 99999, h: -5 });
    layout.screens[1].style = { bg_color: '#123456', bg_opacity: 128 };
    const dStore = M.createStore(layout, {});
    const shellBus = P.createBus();
    const inst = { t: P.i18n(null), caps, readonly: false, isLocked: () => false, layoutStore: dStore, bus: shellBus, notify() {}, assetUrl: id => '/a/' + id };
    const designer = P.designer.create(inst);
    const d = designer.state;
    const events = [];
    ['issues', 'lock', 'screen', 'selection', 'project', 'view'].forEach(type => d.bus.on(type, info => events.push(type === 'project' ? 'project:' + !!info.screenChanged : type)));
    const count = type => events.filter(e => e === type).length;
    const host = new El('div');
    designer.mountCanvas(host);
    const canvas = d.canvas;
    runTimers();
    const scr = canvas.screenEl;
    let renders = 0;
    const replaceScreen = scr.replaceChildren;
    scr.replaceChildren = function (...nodes) { renders++; return replaceScreen.apply(this, nodes); };
    const all = (el, pred, out = []) => { if (pred(el)) out.push(el); el.kids.forEach(k => all(k, pred, out)); return out; };
    const textOf = el => (el.text !== undefined ? el.text : (el.textContent || '') + el.kids.map(textOf).join(''));
    const live = all(canvas.root, el => el.attrs['aria-live'] === 'polite')[0];
    const handles = () => all(canvas.overlay, el => !!el.dataset.ccHandle).length;
    const tool = cmd => all(canvas.root, el => el.attrs['data-cc-cmd'] === cmd)[0];

    // fit and zoom
    canvas.viewport.clientWidth = 800; canvas.viewport.clientHeight = 600;
    canvas.fit(true);
    const at800 = d.zoom;
    canvas.viewport.clientWidth = 1400; canvas.viewport.clientHeight = 900;
    canvas.fit(true);
    eq('the mounted canvas fits 2× into 800×600 and 3× into 1400×900', [at800, d.zoom], [2, 3]);
    let views = count('view');
    let before = renders;
    canvas.fit(true);
    canvas.fit(true);
    d.setZoom(3, true);
    check('an unchanged zoom (resize ticks) emits nothing and renders nothing', count('view') === views && renders === before);
    d.setZoom(3);
    check('the same zoom without auto-fit is a change', count('view') === views + 1 && d.zoomAuto === false);
    canvas.viewport.clientWidth = 0;
    d.setZoom(2, true);
    views = count('view');
    canvas.fit(true);
    check('a hidden viewport (Code mode) keeps the zoom', d.zoom === 2 && count('view') === views);
    canvas.viewport.clientWidth = 1400;

    // rendering of untrusted values
    const bar = canvas.widgetEl('w_bar');
    check('the statusbar shows the screen title and follows the clock/Wi-Fi settings', !!bar && textOf(bar).includes('Home Title') && textOf(bar).includes('12:34') && !textOf(bar).includes(R.SYMBOLS.wifi), bar && textOf(bar));
    const odd = canvas.widgetEl('w_odd"\n]');
    eq('a widget id with quotes and a newline is found; its box is clamped', odd && [odd.style.left, odd.style.top, odd.style.width, odd.style.height], ['0px', '4096px', '2048px', '1px']);
    eq('an invalid screen bg_color falls back to the theme bg with the bg_opacity', scr.style.background, 'rgba(15,21,34,0.502)');
    const tag = all(canvas.root, el => el.attrs.class === 'cc-tag')[0];
    check('the board status uses the 4a tag with tone and t.or', !!tag && tag.attrs['data-tone'] === 'ok' && textOf(tag) === 'verified', tag && JSON.stringify(tag.attrs));
    check('the orientation label reuses cyd_creator.orientation.*', textOf(canvas.root).includes('cyd_creator.orientation.landscape'));

    // shell bus: issues, read-only, lock, mode
    const issues = [{ code: 'NAME_INVALID', severity: 'error', widget: 'w_btn', message: 'x' }];
    before = renders;
    shellBus.emit('layout:issues', issues);
    check('a new issue list emits once and renders', count('issues') === 1 && d.issues.length === 1 && renders === before + 1);
    shellBus.emit('layout:issues', JSON.parse(JSON.stringify(issues)));
    check('an identical issue list (after every save) emits and renders nothing', count('issues') === 1 && renders === before + 1);
    shellBus.emit('layout:issues', []);
    shellBus.emit('layout:issues', null);
    check('a changed list emits; null equals the empty list', count('issues') === 2 && d.issues.length === 0);
    d.select(['w_btn']);
    check('a single selection shows eight handles', handles() === 8);
    inst.readonly = true;
    shellBus.emit('readonly:changed', true);
    check('readonly:changed aborts the gesture and re-renders disabled', count('lock') === 1 && aborts === 1 && handles() === 0 && canvas.root.classList.contains('is-readonly') && tool('delete').disabled === true);
    check('commands refuse while read-only', d.readonly() && d.commands.addAtCenter('label') === null);
    inst.readonly = false;
    shellBus.emit('readonly:changed', false);
    check('read-only off re-enables handles and tools', count('lock') === 2 && handles() === 8 && tool('delete').disabled === false);
    shellBus.emit('lock:changed', true, 'ccj_1');
    shellBus.emit('mode:changed', 'code');
    shellBus.emit('mode:changed', 'design');
    check('lock:changed emits lock and aborts; Code mode aborts a gesture, Design mode does not', count('lock') === 3 && aborts === 4);

    // live region
    runTimers();
    d.select(['w_btn', 'w_tabs']);
    d.select(['w_tabs']);
    d.select(['w_btn', 'w_tabs']);
    check('selection messages wait for the selection to settle', live.textContent !== '2 widgets selected');
    runTimers();
    eq('the settled selection is spoken once', live.textContent, '2 widgets selected');
    d.select([]);
    runTimers();
    eq('a cleared selection is spoken', live.textContent, 'Selection cleared');
    d.setScreen('s_two');
    d.setScreen('s_home');
    live.textContent = 'unchanged';
    runTimers();
    check('screen switches with an empty selection speak nothing', live.textContent === 'unchanged');
    const added = d.commands.addAtCenter('button');
    runTimers();
    check('a command message replaces the pending selection message', !!added && d.selection.join() === added && live.textContent === 'cyd_creator.designer.added', live.textContent);
    dStore.undo();

    // remote replaces: selection upkeep, one render, flashes
    d.select(['w_btn', 'w_tabs']);
    const next = JSON.parse(JSON.stringify(dStore.get()));
    next.screens[0].widgets = next.screens[0].widgets.filter(w => w.id !== 'w_tabs');
    next.screens[0].widgets.find(w => w.id === 'w_btn').x = 40;
    before = renders;
    dStore.replace(next, { resetHistory: false });
    eq('a remote replace drops missing ids and keeps the survivors', d.selection, ['w_btn']);
    check('a remote replace on the same screen renders once and flashes the changed widget', renders === before + 1 && canvas.widgetEl('w_btn').classList.contains('cc-flash'));
    shellBus.emit('layout:issues', issues);
    check('a render during the flash keeps it', renders === before + 2 && canvas.widgetEl('w_btn').classList.contains('cc-flash'));
    runTimers();
    shellBus.emit('layout:issues', []);
    check('the flash ends after its timer', !canvas.widgetEl('w_btn').classList.contains('cc-flash'));
    d.setScreen('s_two');
    eq('a valid screen bg_color is shown with its bg_opacity', scr.style.background, 'rgba(18,52,86,0.502)');
    const gone = JSON.parse(JSON.stringify(dStore.get()));
    gone.screens = gone.screens.filter(s => s.id !== 's_two');
    events.length = 0;
    before = renders;
    dStore.replace(gone, { resetHistory: false });
    check('a remote replace removing the active screen falls back to the start screen', d.screenId === 's_home' && scr.dataset.ccScreenId === 's_home');
    check('… and renders once (screen first, project marked screenChanged)', renders === before + 1 && events.join() === 'screen,project:true', events.join() + ' renders ' + (renders - before));
    const empty = JSON.parse(JSON.stringify(dStore.get()));
    empty.screens = [];
    dStore.replace(empty, { resetHistory: false });
    check('without a screen the stale screen id and background are reset', d.screenId === '' && scr.dataset.ccScreenId === undefined && scr.style.background === '#0F1522' && !scr.kids.length);
    dStore.replace(layout, { resetHistory: false });

    // toolbar focus: a tool that disables itself (or is disabled by a selection change) hands the
    // focus to the canvas, never to the page (the desktop's Delete key deletes a selected file icon).
    d.select(['w_btn']);
    const del = tool('delete');
    del.focus();
    fire(del, 'click');
    check('Delete from the toolbar removes the widget and moves the focus to the canvas', !M.findWidget(dStore.get(), 'w_btn') && del.disabled === true && fakeDoc.activeElement === canvas.root);
    dStore.undo();
    d.select(['w_btn']);
    tool('align-left').focus();
    d.select([]);
    check('a focused tool disabled by a selection change hands the focus to the canvas', tool('align-left').disabled === true && fakeDoc.activeElement === canvas.root);

    // bounded timers, reveal, dispose
    for (let i = 0; i < 50; i++) { canvas.announce('n' + i); canvas.flash(['w_btn']); d.select([i % 2 ? 'w_btn' : 'w_pnl']); }
    check('announce, flash and selection messages keep one timer per purpose', timers.size <= 3, String(timers.size));
    shellBus.emit('designer:reveal', { widget: 'lbl_1' });
    check('designer:reveal selects by name inside its panel', d.selection.join() === 'w_lbl' && d.screenId === 's_home');
    shellBus.emit('designer:reveal', { screen: 'details' });
    check('designer:reveal switches the screen and clears the selection', d.screenId === 's_two' && d.selection.length === 0);
    designer.dispose();
    events.length = 0;
    shellBus.emit('layout:issues', issues);
    shellBus.emit('readonly:changed', true);
    check('a disposed designer ignores the shell bus, clears its timers and removes the canvas', !events.length && !timers.size && !host.kids.length);
    P.designerInteract = realInteract;
}

console.log((failures ? 'FAILED ' + failures : 'ok') + ' — ' + checks + ' checks');
process.exit(failures ? 1 : 0);
