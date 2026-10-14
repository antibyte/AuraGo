#!/usr/bin/env node
// Runs the pure CYD Creator designer modules (geometry, model, render) in Node and checks them:
// snapping and guides, resize/align/distribute maths, the layout store (history, coalescing,
// origins), every layout op incl. reparenting and tabs (structural mirror of Go ApplyOps), name
// rules incl. reserved names, a render smoke test of all 26 widget types in every theme through
// the string serialiser, the interaction module (gestures, palette drag, keyboard) through the
// real designer and store on a minimal fake DOM, and the designer's shell wiring with the real
// canvas on that fake DOM (fit zoom, one render per change, flashes, live region), and the palette
// with the real designer and interaction module there (names, search, press/drag/keyboard adds,
// keys forwarded to a palette drag, disabled by the designer's lock signal), and the inspector with
// the real designer there (field edits checked like validate.go, refused entries kept across issue
// lists and store changes, focus and caret across re-renders, events and hooks, uploads through a
// stubbed inst.uploadAsset that replaces the store, disabled while locked), and the screens panel
// with the real designer and interaction module there (screen list ops, inline rename, the Wi-Fi
// system entry, screen settings, the layer list with its keys, focus kept across store changes,
// disabled by the lock signal). Capabilities come from
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

// ── server mirrors: constants the designer copies from internal/cydcreator, read from the Go sources ──
// goNum, goList and goPattern read a constant, a []string literal or a regexp.MustCompile pattern by
// name (whatever gofmt's alignment); jsConst reads a module-private constant of a designer source. A
// parser that finds nothing yields NaN/null/undefined, so a renamed Go symbol fails here as well.
const goValidate = fs.readFileSync(path.join(root, 'internal', 'cydcreator', 'validate.go'), 'utf8');
const goIds = fs.readFileSync(path.join(root, 'internal', 'cydcreator', 'ids.go'), 'utf8');
const goNum = (src, name) => { const m = src.match(new RegExp('\\b' + name + '\\s*=\\s*([\\d_]+)(?:\\s*<<\\s*(\\d+))?')); return m ? Number(m[1].replace(/_/g, '')) * 2 ** Number(m[2] || 0) : NaN; };
const goList = (src, name) => { const m = src.match(new RegExp('\\b' + name + '\\s*=\\s*\\[\\]string\\{([^}]*)\\}')); return m ? Array.from(m[1].matchAll(/"([^"]*)"/g), x => x[1]) : null; };
const goPattern = (src, name) => (src.match(new RegExp('\\b' + name + '\\s*=\\s*regexp\\.MustCompile\\(`([^`]*)`\\)')) || [])[1];
const jsConst = (file, name) => (fs.readFileSync(path.join(apps, file), 'utf8').match(new RegExp('\\bconst ' + name + ' = ([^;]+);')) || [])[1];
eq('model LIMITS mirror limits.go (MaxScreens, MaxWidgetsPerScreen, MaxNesting, MaxTextBytes, MaxHintRunes)',
    [M.LIMITS.screens, M.LIMITS.widgetsPerScreen, M.LIMITS.nesting, M.LIMITS.textBytes, M.LIMITS.hintRunes],
    ['MaxScreens', 'MaxWidgetsPerScreen', 'MaxNesting', 'MaxTextBytes', 'MaxHintRunes'].map(n => goNum(goLimits, n)));
eq('name rule and reserved names mirror validate.go (namePattern, reservedNames, reservedNamePrefixes, reservedNameSuffixes)',
    [M.NAME_RE.source, M.RESERVED_NAMES, M.RESERVED_PREFIXES, M.RESERVED_SUFFIXES],
    [goPattern(goValidate, 'namePattern'), goList(goValidate, 'reservedNames'), goList(goValidate, 'reservedNamePrefixes'), goList(goValidate, 'reservedNameSuffixes')]);
eq('model enumerations mirror validate.go (transitions, actionTypes, orientations, wifiModes)',
    [M.TRANSITIONS, M.ACTIONS, M.ORIENTATIONS, M.WIFI_MODES],
    ['transitions', 'actionTypes', 'orientations', 'wifiModes'].map(n => goList(goValidate, n)));
eq('render constants mirror the server (HEX colorPattern, ELEMENT_ID elementIDPattern, MAX_SIDE maxWidgetSide, MAX_TEXT MaxTextBytes)',
    ['HEX', 'ELEMENT_ID', 'MAX_SIDE', 'MAX_TEXT'].map(n => jsConst('cyd-creator-render.js', n)),
    ['/' + goPattern(goValidate, 'colorPattern') + '/', '/' + goPattern(goIds, 'elementIDPattern') + '/', String(goNum(goLimits, 'maxWidgetSide')), String(goNum(goLimits, 'MaxTextBytes'))]);
eq('canvas MAX_COORD mirrors maxCoordinate', jsConst('cyd-creator-canvas.js', 'MAX_COORD'), String(goNum(goLimits, 'maxCoordinate')));

// ── render ──
check('renderer for every catalog type', caps.widgets.every(s => R.TYPES.includes(s.type)), R.TYPES.join(','));
eq('font sizes mirror the catalog (statusbar small font)', R.FONT_SIZES, caps.fonts.sizes);
for (const theme of caps.themes) {
    for (const spec of caps.widgets) {
        const w = { id: 'w_' + spec.type, type: spec.type, x: 0, y: 0, w: spec.default_w || 320, h: spec.default_h, props: spec.type === 'image' ? { asset: 'a_sun' } : {} };
        let out = '';
        try { out = R.html(R.tree(spec, w, theme, { assetUrl: id => '/assets/' + id, activeTab: 1 })); } catch (err) { out = 'ERROR ' + err.message; }
        check('render ' + theme.id + '/' + spec.type, out.startsWith('<div') && out.includes('cydc-wv-' + spec.type) && !out.includes('undefined') && !out.includes('NaN'), out.slice(0, 200));
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
check('wifi setup system preview', sys.includes('cyd_creator.designer.system_wifi_title') && sys.includes('cydc-system-screen'), sys.slice(0, 200));
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
    getAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null; }
    setSelectionRange(start, end) { this.selectionStart = start; this.selectionEnd = end; }
    select() { this.setSelectionRange(0, String(this.value == null ? '' : this.value).length); }
    querySelectorAll(sel) {
        const out = [];
        const visit = n => n.kids.forEach(k => { if (k.tagName !== '#TEXT' && k.matches(sel)) out.push(k); visit(k); });
        visit(this);
        return out;
    }
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
        { id: 'w_pnl', type: 'panel', name: 'pnl_1', x: 100, y: 100, w: 120, h: 80, children: [
            { id: 'w_kid', type: 'label', name: 'lbl_kid', x: 10, y: 10, w: 40, h: 16 }] },
        { id: 'w_btn', type: 'button', name: 'btn_9', x: 200, y: 20, w: 100, h: 40 },
        { id: 'w_lock', type: 'label', name: 'lbl_lock', x: 250, y: 70, w: 40, h: 20, locked: true },
        // Content box (0, 200, 320, 40) below a 16 px tab bar.
        { id: 'w_tabs', type: 'tabview', name: 'tabs_1', x: 0, y: 184, w: 320, h: 56, props: { bar_size: 16 }, children: [] }] }];
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
    const vpRect = { left: 0, top: 0, right: 600, bottom: 400 };
    viewport.getBoundingClientRect = () => Object.assign({}, vpRect);
    const screenEl = viewport.appendChild(new El('div'));
    const input = root.appendChild(new El('input'));
    const button = root.appendChild(new El('button'));
    const outside = new El('textarea');
    const els = {};
    ['w_lbl', 'w_pnl', 'w_btn', 'w_lock', 'w_tabs'].forEach(id => { els[id] = screenEl.appendChild(new El('div', { ccWidgetId: id })); });
    els.w_kid = els.w_pnl.appendChild(new El('div', { ccWidgetId: 'w_kid' }));
    const tabHeader = els.w_tabs.appendChild(new El('span', { ccTab: '1' }));
    const handle = viewport.appendChild(new El('div', { ccHandle: 'se' }));
    const seen = { render: 0, moves: [], resizes: [], drops: [] };
    // The stand-in canvas: the screen host sits at client (host.x, host.y) (a scrolled viewport
    // moves it); world = (client - host) / zoom.
    const host = { x: 0, y: 0 };
    P.designerCanvas = { mount: () => ({
        root, viewport,
        render() { seen.render++; }, fit() { seen.fit = (seen.fit || 0) + 1; }, dispose() { seen.disposed = true; },
        previewMove(ids, delta) { seen.moves.push(delta); }, previewResize(id, r) { seen.resizes.push(r); },
        showGuides() {}, showMarquee() {}, showDrop(r) { seen.drops.push(r); }, announce() {},
        hostRect: () => ({ left: host.x, top: host.y, right: host.x + 320 * (d.zoom || 1), bottom: host.y + 240 * (d.zoom || 1) }),
        clientToWorld: (x, y) => G.toWorld({ x, y }, { left: host.x, top: host.y }, d.zoom || 1)
    }) };
    designer.mountCanvas(new El('div'));
    d.setZoom(1);
    d.setSnap(0);
    const widget = id => M.findWidget(iStore.get(), id);
    const at = id => { const f = widget(id); return f && [f.parent ? f.parent.id : null, f.widget.x, f.widget.y]; };
    const key = (k, extra, target) => fire(target || root, 'keydown', Object.assign({ key: k }, extra));
    const used = ev => ev.defaultPrevented && ev.stopped;
    const rootIds = () => iStore.get().screens[0].widgets.map(w => w.id);
    // drag presses on el at from (client px), moves to to and ends with type (pointerup by default).
    function drag(el, from, to, opts) {
        const o = Object.assign({ altKey: true, end: 'pointerup' }, opts);
        fire(el, 'pointerdown', { clientX: from[0], clientY: from[1] });
        fire(viewport, 'pointermove', { clientX: to[0], clientY: to[1], altKey: o.altKey, shiftKey: !!o.shiftKey });
        if (o.end) fire(viewport, o.end, { clientX: to[0], clientY: to[1] });
    }

    // ── move ──
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
    let n = origins.length;
    let previews = seen.moves.length;
    fire(els.w_lbl, 'pointerdown', { clientX: 20, clientY: 15 });
    fire(viewport, 'pointermove', { clientX: 22, clientY: 15 });
    fire(viewport, 'pointerup', { clientX: 22, clientY: 15 });
    check('a click (or a move below 3 px) selects without a preview or a store change', origins.length === n && seen.moves.length === previews && d.selection.join() === 'w_lbl' && !viewport.captured.size);

    // ── cancelling ──
    const before = JSON.stringify(iStore.get());
    const aborts = [
        ['readonly:changed', () => shellBus.emit('readonly:changed', true)],
        ['lock:changed', () => shellBus.emit('lock:changed', true, 'ccj_1')],
        ['mode:changed code', () => shellBus.emit('mode:changed', 'code')],
        ['window blur', () => (winListeners.blur || []).forEach(fn => fn({}))],
        ['a remote replace', () => iStore.replace(JSON.parse(before), { resetHistory: false })],
        ['lostpointercapture', () => fire(viewport, 'lostpointercapture', {})],
        ['a mouse move without a button', () => fire(viewport, 'pointermove', { clientX: 70, clientY: 15, buttons: 0 })]
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

    drag(els.w_lbl, [20, 15], [60, 15], { end: null });
    fire(viewport, 'pointermove', { pointerId: 3, clientX: 200, clientY: 15, altKey: true });
    fire(els.w_btn, 'pointerdown', { pointerId: 3, clientX: 210, clientY: 30 });
    fire(viewport, 'pointerup', { pointerId: 3, clientX: 200, clientY: 15 });
    check('a second pointer neither moves, starts nor ends the gesture', viewport.captured.has(1) && seen.moves[seen.moves.length - 1].x === 40 && d.selection.join() === 'w_lbl');
    d.commands.update('w_btn', { x: 205 });
    check('a local change (an inspector edit) does not cancel the gesture', viewport.captured.has(1));
    fire(viewport, 'pointerup', { clientX: 60, clientY: 15 });
    eq('the gesture commits after the local change', [widget('w_lbl').widget.x, widget('w_btn').widget.x], [50, 205]);
    iStore.undo();
    iStore.undo();
    drag(els.w_lbl, [20, 15], [60, 15], { end: null });
    inst.readonly = true;
    const rendersBefore = seen.render;
    fire(viewport, 'pointerup', { clientX: 60, clientY: 15 });
    inst.readonly = false;
    check('a refused commit re-renders the stored layout (no lingering preview)', seen.render === rendersBefore + 1 && JSON.stringify(iStore.get()) === before);

    // ── selection presses ──
    fire(els.w_btn, 'pointerdown', { ctrlKey: true });
    const toggledIn = d.selection.join();
    fire(els.w_btn, 'pointerdown', { ctrlKey: true });
    eq('Ctrl+click toggles without a gesture', [toggledIn, d.selection.join(), viewport.captured.size], ['w_lbl,w_btn', 'w_lbl', 0]);
    fire(tabHeader, 'pointerdown', {});
    eq('a tab header switches the page and selects the tabview', [d.activeTab('w_tabs'), d.selection.join(), viewport.captured.size], [1, 'w_tabs', 0]);
    fire(els.w_lock, 'pointerdown', { clientX: 260, clientY: 75 });
    fire(handle, 'pointerdown', { clientX: 290, clientY: 90 });
    check('a locked widget is selected but neither moves nor resizes', d.selection.join() === 'w_lock' && !viewport.captured.size);
    fire(viewport, 'pointerup', {});
    inst.readonly = true;
    fire(els.w_btn, 'pointerdown', { clientX: 210, clientY: 30 });
    const roSelection = d.selection.join();
    fire(handle, 'pointerdown', { clientX: 300, clientY: 60 });
    inst.readonly = false;
    check('read-only: a press only selects', roSelection === 'w_btn' && !viewport.captured.size && JSON.stringify(iStore.get()) === before);

    // ── reparenting ──
    drag(els.w_btn, [210, 30], [130, 120]);
    eq('dropping on a panel reparents with coordinates in the panel', at('w_btn'), ['w_pnl', 20, 10]);
    iStore.undo();
    eq('reparent is one undo step', at('w_btn'), [null, 200, 20]);
    drag(els.w_btn, [210, 30], [60, 215]);
    eq('dropping on the visible tabview page sets its tab', [at('w_btn'), widget('w_btn').widget.tab], [['w_tabs', 50, 5], 1]);
    iStore.undo();
    d.select(['w_kid']);
    drag(els.w_kid, [115, 115], [115, 75]);
    eq('dropping a panel child on the screen moves it to the screen root', at('w_kid'), [null, 110, 70]);
    iStore.undo();
    d.select(['w_kid', 'w_lbl']);
    drag(els.w_lbl, [20, 15], [25, 15]);
    eq('a selection across parents moves in place where the pressed widget lives', [at('w_kid'), at('w_lbl')], [['w_pnl', 15, 10], [null, 15, 10]]);
    iStore.undo();
    d.select(['w_lbl']);
    drag(els.w_lbl, [20, 15], [30, 15], { end: null });
    host.x = -20;
    fire(viewport, 'pointermove', { clientX: 30, clientY: 15, altKey: true });
    fire(viewport, 'pointerup', { clientX: 30, clientY: 15 });
    host.x = 0;
    eq('a viewport scrolled during a drag keeps the widget under the pointer', [seen.moves.pop(), widget('w_lbl').widget.x], [{ x: 30, y: 0 }, 40]);
    iStore.undo();
    check('reparent tests leave the layout as it was', JSON.stringify(iStore.get()) === before);

    // ── resize ──
    d.select(['w_lbl']);
    drag(handle, [90, 30], [103, 37]);
    eq('resize commits integers through update_widget', ['x', 'y', 'w', 'h'].map(k => widget('w_lbl').widget[k]), [10, 10, 93, 27]);
    d.setZoom(2);
    drag(handle, [206, 74], [211, 74]);
    eq('resize rounds the pointer delta at zoom 2', widget('w_lbl').widget.w, 96);
    d.setZoom(1);
    d.setSnap(8);
    n = origins.length;
    const resizes = seen.resizes.length;
    fire(handle, 'pointerdown', { clientX: 106, clientY: 37 });
    fire(viewport, 'pointermove', { clientX: 106, clientY: 37 });
    fire(viewport, 'pointermove', { clientX: 107, clientY: 38 });
    fire(viewport, 'pointerup', { clientX: 107, clientY: 38 });
    d.setSnap(0);
    check('a click on a handle (or a move below 3 px) does not resize, even with the grid on', origins.length === n && seen.resizes.length === resizes && widget('w_lbl').widget.w === 96 && widget('w_lbl').widget.h === 27 && !root.classList.contains('is-resizing'));
    const handlePress = fire(handle, 'pointerdown', { clientX: 106, clientY: 37 });
    fire(viewport, 'pointerup', { clientX: 106, clientY: 37 });
    check('a handle press is not stopped (the window manager raises the window)', handlePress.defaultPrevented && !handlePress.stopped);

    // ── box select ──
    d.select([]);
    drag(viewport, [5, 5], [95, 35], { altKey: false });
    eq('box select picks the root widgets it touches', d.selection, ['w_lbl']);
    fire(viewport, 'pointerdown', { clientX: 5, clientY: 5, offsetX: 600, offsetY: 5 });
    check('a press on the viewport scrollbar neither starts a gesture nor clears', !viewport.captured.size && d.selection.join() === 'w_lbl');

    // ── keyboard ──
    const x0 = widget('w_lbl').widget.x;
    const steps = [];
    for (let i = 0; i < 5; i++) steps.push(key('ArrowRight'));
    check('arrow keys are used and stop at the canvas', steps.every(used));
    eq('five nudges move 5 px', widget('w_lbl').widget.x, x0 + 5);
    key('ArrowDown', { shiftKey: true });
    eq('Shift+arrow nudges 8 px', widget('w_lbl').widget.y, 18);
    key('z', { ctrlKey: true });
    eq('a nudge burst is one undo step', [widget('w_lbl').widget.x, widget('w_lbl').widget.y], [x0, 10]);
    key('Z', { ctrlKey: true, shiftKey: true });
    eq('Ctrl+Shift+Z redoes', [widget('w_lbl').widget.x, widget('w_lbl').widget.y], [x0 + 5, 18]);
    key('z', { ctrlKey: true });
    const typed = key('Delete', {}, input);
    check('keys typed into a field stay native', !typed.defaultPrevented && !!widget('w_lbl'));
    check('Space and Enter on a button stay native', !key(' ', {}, button).defaultPrevented && !key('Enter', {}, button).defaultPrevented);
    check('keys of other windows and the code editor never reach the designer', !key('Delete', {}, outside).defaultPrevented && !!widget('w_lbl'));
    check('Ctrl/Meta+S bubbles on to the shell', !key('s', { ctrlKey: true }).defaultPrevented && !key('s', { metaKey: true }).defaultPrevented);
    const altZ = key('z', { ctrlKey: true, altKey: true });
    check('Ctrl+Alt, Ctrl+Shift (except Z) and Alt combinations are left to the desktop', !altZ.defaultPrevented && widget('w_lbl').widget.x === x0 && !key('C', { ctrlKey: true, shiftKey: true }).defaultPrevented && !key('Y', { ctrlKey: true, shiftKey: true }).defaultPrevented && !key('Delete', { altKey: true }).defaultPrevented && !!widget('w_lbl'));
    check('Enter and F2 on the canvas stop there (the desktop would open or rename its selected icon)', used(key('Enter')) && used(key('F2')));
    viewport.clientWidth = 0;
    check('a hidden canvas (Code mode) uses no keys', !key('Delete').defaultPrevented && !!widget('w_lbl'));
    viewport.clientWidth = 600;

    // clipboard and history keys
    d.select(['w_btn']);
    const count0 = rootIds().length;
    check('Ctrl+C is used', used(key('c', { ctrlKey: true })));
    key('v', { ctrlKey: true });
    const pasted = d.selection.join();
    check('Ctrl+V pastes a copy and selects it', rootIds().length === count0 + 1 && !!pasted && pasted !== 'w_btn' && widget(pasted).widget.type === 'button');
    key('d', { metaKey: true });
    check('Meta+D duplicates the selection', rootIds().length === count0 + 2 && d.selection.join() !== pasted);
    key('z', { ctrlKey: true });
    check('Ctrl+Z undoes the duplicate', rootIds().length === count0 + 1);
    key('y', { ctrlKey: true });
    check('Ctrl+Y redoes it', rootIds().length === count0 + 2);
    key('z', { ctrlKey: true });
    key('z', { ctrlKey: true });
    d.select(['w_btn']);
    key('x', { ctrlKey: true });
    check('Ctrl+X cuts the selection', !widget('w_btn') && rootIds().length === count0 - 1);
    key('v', { ctrlKey: true });
    check('the cut widget pastes back as a copy', rootIds().length === count0);
    key('z', { ctrlKey: true });
    key('z', { ctrlKey: true });
    check('undo brings the cut widget back', !!widget('w_btn') && rootIds().length === count0);
    key('a', { ctrlKey: true });
    eq('Ctrl+A selects every visible root widget', d.selection, rootIds());

    // keys during a gesture
    d.select(['w_lbl']);
    drag(els.w_lbl, [20, 15], [60, 15], { end: null });
    const xDrag = widget('w_lbl').widget.x;
    const del = key('Delete');
    const arrow = key('ArrowRight');
    check('during a gesture the designer keys are used without running', used(del) && used(arrow) && used(key('Backspace')) && used(key('Enter')) && used(key('z', { ctrlKey: true })) && !!widget('w_lbl') && widget('w_lbl').widget.x === xDrag && viewport.captured.has(1));
    check('during a gesture Ctrl+S, Alt combinations and Tab stay native', !key('s', { ctrlKey: true }).defaultPrevented && !key('x', { altKey: true }).defaultPrevented && !key('Tab').defaultPrevented);
    check('Escape cancels a gesture', used(key('Escape')) && !viewport.captured.size && widget('w_lbl').widget.x === xDrag);
    check('Escape clears the selection', used(key('Escape')) && d.selection.length === 0);
    check('Escape without a selection bubbles on', !key('Escape').defaultPrevented);
    d.select(['w_lbl']);
    key('Delete');
    check('Delete removes the selection', !widget('w_lbl'));
    key('+');
    eq('+ zooms in', d.zoom, 2);
    key('0');
    check('0 fits the screen', seen.fit === 1);

    // ── palette: a press adds at the centre, a drag drops into the container under the pointer ──
    const palette = new El('div');
    const item = palette.appendChild(new El('button', { ccPalette: 'button' }));
    const press = { clientX: 5, clientY: 5, pointerId: 2, button: 0, buttons: 1, pointerType: 'mouse', preventDefault() {} };
    const sample = M.newWidget(iStore.get(), specs.get('button'), { x: 0, y: 0 }, d.screenSize());
    d.interact.startPaletteDrag('button', Object.assign({ target: item }, press), item);
    check('a palette drag captures the pointer on the canvas viewport', viewport.captured.has(2) && !item.captured.size);
    fire(viewport, 'pointerup', { pointerId: 2 });
    const added = widget(d.selection[0]);
    eq('a palette press adds the catalog default with a proposed name', added && [added.widget.name, added.widget.w, added.widget.h, added.parent], [sample.name, specs.get('button').default_w, specs.get('button').default_h, null]);
    d.interact.startPaletteDrag('statusbar', Object.assign({ target: item }, press), item);
    fire(viewport, 'pointermove', { pointerId: 2, clientX: 700, clientY: 300 });
    const ghost = fakeDoc.body.kids[0];
    eq('the palette ghost has the new widget\'s default size (statusbar: screen width, zoom 2)', [ghost && ghost.className, ghost && ghost.style.width, ghost && ghost.classList.contains('is-outside')], ['cydc-drag-ghost', '640px', true]);
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 700, clientY: 300 });
    check('a drop outside the screen adds nothing and removes the ghost', !fakeDoc.body.kids.length && !M.findWidget(iStore.get(), 'status_1'));
    d.setZoom(1);
    n = origins.length;
    vpRect.right = 200;
    d.interact.startPaletteDrag('label', Object.assign({ target: item }, press), item);
    fire(viewport, 'pointermove', { pointerId: 2, clientX: 250, clientY: 50 });
    const clipped = !!fakeDoc.body.kids[0] && fakeDoc.body.kids[0].classList.contains('is-outside');
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 250, clientY: 50 });
    vpRect.right = 600;
    check('a drop on the part of the screen the viewport clips adds nothing', clipped && origins.length === n && !fakeDoc.body.kids.length);
    d.interact.startPaletteDrag('label', Object.assign({ target: item }, press), item);
    fire(viewport, 'pointermove', { pointerId: 2, clientX: 140, clientY: 130 });
    const swallowed = d.interact.gestureKey({ key: 'Delete', target: item, defaultPrevented: false });
    const cancelled = d.interact.gestureKey({ key: 'Escape', target: item, defaultPrevented: false });
    check('a palette drag cancelled with Escape (forwarded by the palette) removes the ghost and adds nothing', swallowed && cancelled && !fakeDoc.body.kids.length && origins.length === n && !viewport.captured.size && !root.classList.contains('is-dropping'));
    check('gestureKey takes nothing without a gesture', !d.interact.gestureKey({ key: 'Escape', target: item, defaultPrevented: false }));
    d.interact.startPaletteDrag('label', Object.assign({ target: item }, press), item);
    item.remove();
    fire(viewport, 'pointermove', { pointerId: 2, clientX: 140, clientY: 130 });
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 140, clientY: 130 });
    const dropped = widget(d.selection[0]);
    check('a palette drop adds into the panel under the pointer, even after the palette re-rendered', dropped && dropped.widget.type === 'label' && dropped.parent && dropped.parent.id === 'w_pnl', JSON.stringify(dropped && dropped.widget));
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
    const tag = all(canvas.root, el => el.attrs.class === 'cydc-tag')[0];
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
    check('a remote replace on the same screen renders once and flashes the changed widget', renders === before + 1 && canvas.widgetEl('w_btn').classList.contains('cydc-flash'));
    shellBus.emit('layout:issues', issues);
    check('a render during the flash keeps it', renders === before + 2 && canvas.widgetEl('w_btn').classList.contains('cydc-flash'));
    runTimers();
    shellBus.emit('layout:issues', []);
    check('the flash ends after its timer', !canvas.widgetEl('w_btn').classList.contains('cydc-flash'));
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

    // duplicate ids (an ID_DUPLICATE layout): the canvas maps the id to the first widget, like M.findWidget.
    const dup = JSON.parse(JSON.stringify(layout));
    dup.screens[0].widgets.push({ id: 'w_dup', type: 'panel', name: 'pnl_dup', x: 10, y: 150, w: 60, h: 40 }, { id: 'w_dup', type: 'label', name: 'lbl_dup', x: 200, y: 150, w: 60, h: 20 });
    dStore.replace(dup, { resetHistory: false });
    const dupEls = all(scr, el => el.dataset.ccWidgetId === 'w_dup');
    check('two elements render for a duplicate id; the first is the one M.findWidget returns', dupEls.length === 2 && M.findWidget(dStore.get(), 'w_dup').widget.name === 'pnl_dup' && canvas.widgetEl('w_dup') === dupEls[0]);
    canvas.previewMove(['w_dup'], { x: 5, y: 0 });
    eq('a move preview of a duplicate id moves the first widget only', [dupEls[0].style.left, dupEls[1].style.left], ['15px', '200px']);
    const dupView = dupEls[0].kids[0];
    canvas.previewResize('w_dup', { x: 10, y: 150, w: 80, h: 40 });
    check('a resize preview of a duplicate id redraws the first widget only', dupEls[0].kids[0] !== dupView && dupEls[0].style.width === '80px' && dupEls[1].style.width === '60px');
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

// ── palette (cyd-creator-palette.js) through the real designer and interact on the fake DOM ──
// Catalog groups and names (t.or with the catalog ids as fallback, unknown groups last), glyphs,
// accessibility attributes, search, a press and a drag through interact (the canvas takes the focus
// at the press), the keyboard add (a click with detail 0), the designer's keys from an entry, keys
// forwarded to a palette drag (Escape cancels it), the keys the desktop acts on, and the designer's
// lock signal: disabled while read-only, locked by a job or on the Wi-Fi system screen, without
// dropping the keyboard focus to the page.
{
    const paletteFile = path.join(apps, 'cyd-creator-palette.js');
    vm.runInContext(fs.readFileSync(paletteFile, 'utf8'), sandbox, { filename: paletteFile });
    const realCanvas = P.designerCanvas;
    const GLYPHS = P.designerPalette.GLYPHS;
    const all = (el, pred, out = []) => { if (pred(el)) out.push(el); el.kids.forEach(k => all(k, pred, out)); return out; };
    const textOf = el => (el.text !== undefined ? el.text : el.kids.map(textOf).join(''));
    // The stand-in canvas: a 320×240 screen at client (0, 0), zoom 1; it records the announcements.
    const croot = new El('div');
    const viewport = croot.appendChild(new El('div'));
    viewport.getBoundingClientRect = () => ({ left: 0, top: 0, right: 600, bottom: 400 });
    const said = [];
    P.designerCanvas = { mount: () => ({
        root: croot, viewport, render() {}, fit() {}, dispose() {}, previewMove() {}, previewResize() {},
        showGuides() {}, showMarquee() {}, showDrop() {}, announce(text) { said.push(text); },
        hostRect: () => ({ left: 0, top: 0, right: 320, bottom: 240 }),
        clientToWorld: (x, y) => ({ x, y })
    }) };
    const pStore = M.createStore(project(), {});
    const origins = [];
    pStore.subscribe((p, info) => origins.push(info.origin));
    const shellBus = P.createBus();
    let jobLock = false;
    const inst = { t: P.i18n(null), caps, readonly: false, isLocked: () => jobLock, layoutStore: pStore, bus: shellBus, notify() {}, assetUrl: id => '/a/' + id };
    sandbox.window.I18N = {
        'cyd_creator.widget.button': 'Schaltfläche', 'cyd_creator.group.basic': 'Grundlagen',
        'cyd_creator.designer.palette_item_hint': '{name} einfügen', 'cyd_creator.designer.added': '{name} hinzugefügt'
    };
    const designer = P.designer.create(inst);
    const d = designer.state;
    designer.mountCanvas(new El('div'));
    d.setZoom(1);
    d.setSnap(0);
    const host = new El('div');
    designer.mountPalette(host);
    const pal = host.kids[0];
    const search = all(pal, el => el.tagName === 'INPUT')[0];
    const items = () => all(pal, el => el.dataset.ccPalette !== undefined);
    const item = type => items().find(el => el.dataset.ccPalette === type);
    const types = () => items().map(el => el.dataset.ccPalette);
    const nameOf = el => textOf(all(el, k => k.attrs.class === 'cydc-palette-name')[0]);
    const glyphOf = el => all(el, k => k.attrs.class === 'cydc-palette-glyph')[0];
    const roots = () => pStore.get().screens[0].widgets;
    const ghosts = () => fakeDoc.body.kids.length;
    const used = ev => ev.defaultPrevented && ev.stopped;
    const setSearch = q => { search.value = q; fire(search, 'input', {}); };
    const press = type => {
        fire(item(type), 'pointerdown', { pointerId: 2, clientX: 5, clientY: 5 });
        fire(viewport, 'pointermove', { pointerId: 2, clientX: 200, clientY: 60 });
    };

    // catalog, names, glyphs, accessibility, search
    const order = ['basic', 'input', 'display', 'structure', 'composite'];
    eq('every catalog widget has one entry, grouped in catalog order', types(), order.flatMap(g => caps.widgets.filter(s => s.group === g).map(s => s.type)));
    eq('GLYPHS covers every catalog type', caps.widgets.map(s => s.type).filter(type => !GLYPHS[type]), []);
    eq('every entry shows its type\'s glyph', items().filter(el => textOf(glyphOf(el)) !== GLYPHS[el.dataset.ccPalette]).map(el => el.dataset.ccPalette), []);
    eq('group headings are translated or fall back to the catalog group id', all(pal, el => el.tagName === 'H4').map(textOf), ['Grundlagen', 'input', 'display', 'structure', 'composite']);
    eq('every group is a role=group labelled like its heading', all(pal, el => el.attrs.class === 'cydc-palette-group').map(g => [g.attrs.role, g.attrs['aria-label']]), [['group', 'Grundlagen'], ['group', 'input'], ['group', 'display'], ['group', 'structure'], ['group', 'composite']]);
    eq('glyphs are hidden from assistive technology', items().filter(el => glyphOf(el).attrs['aria-hidden'] !== 'true').length, 0);
    eq('the palette is a labelled section', [pal.tagName, pal.attrs['aria-label']], ['SECTION', 'cyd_creator.designer.palette_title']);
    eq('the search field is a labelled search input', [search.attrs.type, search.attrs['aria-label']], ['search', 'cyd_creator.designer.palette_search']);
    eq('entries are type=button', items().filter(el => el.attrs.type !== 'button').length, 0);
    eq('a translated entry name', nameOf(item('button')), 'Schaltfläche');
    eq('an untranslated entry name falls back to the catalog type', nameOf(item('led')), 'led');
    eq('the entry hint names the widget', item('button').attrs.title, 'Schaltfläche einfügen');
    eq('d.label translates with t.or and falls back to the catalog type', [d.label('button'), d.label('led')], ['Schaltfläche', 'led']);
    setSearch('SCHALT');
    eq('search matches the translated name', types(), ['button']);
    setSearch(' Butt ');
    eq('search matches the type id of a translated entry (trimmed, any case)', types(), ['button']);
    setSearch('zzz');
    eq('a search without a match lists no entry', items().length, 0);
    eq('… and shows the empty hint', all(pal, el => el.attrs.class === 'cydc-panel-empty').length, 1);
    setSearch('');
    eq('clearing the search lists every entry again', items().length, caps.widgets.length);

    // pointer: a press adds in the middle, a drag drops into the container under the pointer
    const n0 = roots().length;
    fakeDoc.activeElement = null;
    fire(glyphOf(item('button')), 'pointerdown', { pointerId: 2, clientX: 5, clientY: 5 });
    check('a press on an entry (also on its glyph) starts a palette drag captured on the canvas viewport', viewport.captured.has(2));
    check('the canvas takes the focus at the press (where pressed buttons take none, Escape still reaches the gesture)', fakeDoc.activeElement === croot);
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 5, clientY: 5 });
    fire(item('button'), 'click', { detail: 1 });
    const pressed = roots().filter(w => w.type === 'button' && w.id !== 'w_btn');
    eq('a press without movement adds one widget (the mouse click after it adds nothing)', [roots().length, pressed.length], [n0 + 1, 1]);
    eq('… in the middle with the catalog size', pressed[0] && [pressed[0].x, pressed[0].y, pressed[0].w, pressed[0].h], [100, 100, 120, 40]);
    check('… selects it', !!pressed[0] && d.selection.join() === pressed[0].id);
    check('… keeps the focus on the canvas', fakeDoc.activeElement === croot);
    eq('… and announces it with the translated name', said[said.length - 1], 'Schaltfläche hinzugefügt');
    press('led');
    const ghostShown = ghosts();
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 200, clientY: 60 });
    const led = M.findWidget(pStore.get(), d.selection[0]);
    eq('a drag shows the ghost', ghostShown, 1);
    eq('the drop removes the ghost and releases the pointer', [ghosts(), viewport.captured.size], [0, 0]);
    eq('the widget lands in the panel under the pointer', led && [led.widget.type, led.parent && led.parent.id], ['led', 'w_pnl']);
    eq('the add is announced with d.label (the catalog type as fallback)', said[said.length - 1], 'led hinzugefügt');

    // keyboard: Enter/Space make a click with detail 0, which adds in the middle; the designer's keys
    // work from the entry that keeps the focus; the keys the desktop acts on stop at the palette.
    const n1 = roots().length;
    const sw = item('switch');
    sw.focus();
    const enter = fire(sw, 'keydown', { key: 'Enter' });
    eq('Enter on an entry stops at the palette (the desktop would open its selected icon) and keeps its native click', [enter.stopped, enter.defaultPrevented], [true, false]);
    for (const mod of ['ctrlKey', 'metaKey', 'altKey', 'shiftKey']) {
        const ev = fire(sw, 'keydown', { key: 'Enter', [mod]: true });
        eq('Enter with ' + mod + ' stops at the palette too', [ev.stopped, ev.defaultPrevented], [true, false]);
    }
    fire(sw, 'click', { detail: 0 });
    const keyed = roots()[roots().length - 1];
    eq('a keyboard click adds one widget in the middle', [roots().length, keyed.type, keyed.x, keyed.y], [n1 + 1, 'switch', 132, 106]);
    check('… selects it', d.selection.join() === keyed.id);
    check('… and leaves the focus on the entry', fakeDoc.activeElement === sw);
    check('Delete on the entry is used by the designer, not passed to the desktop', used(fire(sw, 'keydown', { key: 'Delete' })));
    check('… and deletes the widget just added', !M.findWidget(pStore.get(), keyed.id));
    check('Ctrl+Z on the entry is used', used(fire(sw, 'keydown', { key: 'z', ctrlKey: true })));
    check('… and brings the widget back', !!M.findWidget(pStore.get(), keyed.id));
    const f2 = fire(sw, 'keydown', { key: 'F2' });
    eq('F2 on an entry stops at the palette (the desktop would rename its selected icon), nothing to prevent', [f2.stopped, f2.defaultPrevented], [true, false]);
    const space = fire(sw, 'keydown', { key: ' ' });
    eq('Space on an entry keeps its native click and bubbles', [space.stopped, space.defaultPrevented], [false, false]);
    const save = fire(sw, 'keydown', { key: 's', ctrlKey: true });
    eq('Ctrl+S on an entry bubbles on to the shell', [save.stopped, save.defaultPrevented], [false, false]);
    const tab = fire(sw, 'keydown', { key: 'Tab' });
    eq('Tab on an entry stays native', [tab.stopped, tab.defaultPrevented], [false, false]);
    d.select([keyed.id]);
    viewport.clientWidth = 0;
    const hidden = ['Delete', 'Backspace'].map(k => { const ev = fire(sw, 'keydown', { key: k }); return [k, ev.stopped, ev.defaultPrevented]; });
    viewport.clientWidth = 600;
    eq('with the canvas hidden (Code mode) Delete and Backspace still stop at the palette, unused', hidden, [['Delete', true, false], ['Backspace', true, false]]);
    check('… and delete nothing', !!M.findWidget(pStore.get(), keyed.id));
    search.focus();
    const typed = ['Delete', 'Backspace', 'Enter', 'F2', 'ArrowLeft', 'Escape'].map(k => { const ev = fire(search, 'keydown', { key: k }); return [k, ev.stopped, ev.defaultPrevented]; });
    eq('the search field keeps its keys', typed, ['Delete', 'Backspace', 'Enter', 'F2', 'ArrowLeft', 'Escape'].map(k => [k, false, false]));
    eq('… and they edit nothing', [!!M.findWidget(pStore.get(), keyed.id), M.findWidget(pStore.get(), keyed.id).widget.x, d.selection.join() === keyed.id], [true, 132, true]);

    // keys during a palette drag go to interact.gestureKey, wherever the focus is
    const kept = JSON.stringify(pStore.get());
    press('label');
    check('during a palette drag Delete from an entry is used without running', used(fire(item('label'), 'keydown', { key: 'Delete' })));
    check('… the selected widget stays', JSON.stringify(pStore.get()) === kept);
    check('Escape from an entry cancels the palette drag', used(fire(item('label'), 'keydown', { key: 'Escape' })));
    eq('… removes the ghost, releases the pointer and ends the dropping state', [ghosts(), viewport.captured.size, croot.classList.contains('is-dropping')], [0, 0, false]);
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 200, clientY: 60 });
    check('the release after Escape adds nothing', JSON.stringify(pStore.get()) === kept);
    press('label');
    check('Escape in the search field cancels a palette drag', used(fire(search, 'keydown', { key: 'Escape' })));
    eq('… removes the ghost and releases the pointer', [ghosts(), viewport.captured.size], [0, 0]);
    press('label');
    check('Escape at the canvas (focused at the press) cancels a palette drag', used(fire(croot, 'keydown', { key: 'Escape' })));
    eq('… removes the ghost and releases the pointer', [ghosts(), viewport.captured.size], [0, 0]);
    check('cancelled palette drags add nothing', JSON.stringify(pStore.get()) === kept);

    // the designer's lock signal: read-only, a job lock and the Wi-Fi system screen disable every entry
    const disabledAll = () => items().length === caps.widgets.length && items().every(el => el.disabled === true);
    const enabledAll = () => items().length === caps.widgets.length && items().every(el => !el.disabled);
    check('entries start enabled', enabledAll());
    check('… and the palette is not marked read-only', !pal.classList.contains('is-readonly'));
    item('led').focus();
    inst.readonly = true;
    shellBus.emit('readonly:changed', true);
    check('read-only disables every entry', disabledAll());
    check('… and marks the palette', pal.classList.contains('is-readonly'));
    check('a focused entry that gets disabled hands the focus to the search field, not the page', fakeDoc.activeElement === search);
    fire(item('label'), 'pointerdown', { pointerId: 2, clientX: 5, clientY: 5 });
    check('read-only: a press starts no drag', !viewport.captured.size);
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 5, clientY: 5 });
    fire(item('label'), 'click', { detail: 0 });
    check('read-only: neither a press nor a keyboard click adds anything', JSON.stringify(pStore.get()) === kept);
    inst.readonly = false;
    shellBus.emit('readonly:changed', false);
    check('read-only off enables the entries again', enabledAll());
    check('… and unmarks the palette', !pal.classList.contains('is-readonly'));
    press('label');
    eq('a palette drag runs before the job lock', [ghosts(), viewport.captured.size], [1, 1]);
    jobLock = true;
    shellBus.emit('lock:changed', true, 'ccj_1');
    fire(viewport, 'pointerup', { pointerId: 2, clientX: 200, clientY: 60 });
    eq('a job lock during a palette drag removes the ghost and releases the pointer', [ghosts(), viewport.captured.size], [0, 0]);
    check('… adds nothing', JSON.stringify(pStore.get()) === kept);
    check('… and disables the entries', disabledAll());
    jobLock = false;
    shellBus.emit('lock:changed', false, '');
    check('the released lock enables the entries again', enabledAll());
    const first = items()[0];
    d.setScreen('s_two');
    check('a switch between ordinary screens re-renders nothing', items()[0] === first);
    d.setScreen(d.SYSTEM_WIFI);
    check('the Wi-Fi system screen disables the entries (it takes no widgets)', d.isSystem() && disabledAll());
    eq('… and marks the palette as system, not read-only', [pal.classList.contains('is-system'), pal.classList.contains('is-readonly')], [true, false]);
    d.setScreen('s_home');
    check('another screen enables them again', enabledAll());
    check('… and unmarks the palette', !pal.classList.contains('is-system'));
    eq('every palette edit reached the store (the undo from Ctrl+Z as undo)', origins, ['local', 'local', 'local', 'local', 'undo']);

    designer.dispose();
    shellBus.emit('readonly:changed', true);
    eq('dispose removes the palette', host.kids.length, 0);
    eq('… and its search and key listeners', [(search.listeners.input || []).length, (pal.listeners.keydown || []).length], [0, 0]);
    check('a disposed palette ignores the lock signal', !items().some(el => el.disabled));

    // an unknown catalog group comes after the known ones, with the fallback glyph and names
    const extraCaps = Object.assign({}, caps, { widgets: caps.widgets.concat([{ type: 'gizmo', group: 'extra', container: false, default_w: 40, default_h: 40, props: [], events: [] }]) });
    const other = P.designer.create(Object.assign({}, inst, { caps: extraCaps, readonly: false, isLocked: () => false, layoutStore: M.createStore(project(), {}), bus: P.createBus() }));
    const host2 = new El('div');
    other.mountPalette(host2);
    const pal2 = host2.kids[0];
    eq('an unknown catalog group is listed after the known ones under its id', all(pal2, el => el.tagName === 'H4').map(textOf), ['Grundlagen'].concat(order.slice(1), ['extra']));
    const gizmo = all(pal2, el => el.dataset.ccPalette === 'gizmo')[0];
    eq('its entry has the fallback glyph and its type as the name', gizmo && [textOf(glyphOf(gizmo)), nameOf(gizmo), gizmo.parent.parent.attrs['aria-label']], ['◆', 'gizmo', 'extra']);
    other.dispose();
    delete sandbox.window.I18N;
    P.designerCanvas = realCanvas;
}

// ── inspector (cyd-creator-inspector-fields.js, cyd-creator-inspector.js) through the real designer ──
// Fields edit the store (origin local) with values validate.go accepts; refused entries stay in their
// field across issue lists and store changes; focus and caret survive re-renders; events and hooks use
// the shapes the server accepts; uploads go through inst.uploadAsset, stubbed like app.js (it replaces
// the store remotely before it answers), with the de-duplicating push as fallback; the designer's lock
// signal disables the panel.
{
    for (const file of ['cyd-creator-inspector-fields.js', 'cyd-creator-inspector.js']) {
        const full = path.join(apps, file);
        vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
    }
    const FD = P.designerFields;
    const LIM = P.designerInspector.LIMITS;
    const goNumber = (src, name) => Number(((src.match(new RegExp(name + '\\s*=\\s*([\\d_]+)')) || [])[1] || '').replace(/_/g, ''));
    const validateGo = fs.readFileSync(path.join(root, 'internal', 'cydcreator', 'validate.go'), 'utf8');
    eq('inspector limits mirror the server (maxCoordinate, intPropLimit, maxTimezoneBytes)', [LIM.coord, LIM.intProp, LIM.timezoneBytes], [goNumber(goLimits, 'maxCoordinate'), goNumber(goLimits, 'intPropLimit'), goNumber(validateGo, 'maxTimezoneBytes')]);
    check('the colour check is validate.go colorPattern', goPattern(validateGo, 'colorPattern') === FD.HEX.source, FD.HEX.source);
    eq('clampInt rounds, clamps and refuses empty or non-numeric input', [FD.clampInt('12.6', 0, 10), FD.clampInt('', 0, 10), FD.clampInt('x'), FD.clampInt(-5, 1, 2048), FD.clampInt('1e9', -4096, 4096), FD.clampInt(-0.4), FD.clampInt(true)], [10, null, null, 1, 4096, 0, null]);
    eq('utf8Bytes counts like Go len() (a lone surrogate arrives as U+FFFD)', ['abc', 'ä', '€', '😀', String.fromCharCode(0xd800)].map(FD.utf8Bytes), [3, 2, 3, 4, 3]);
    eq('textIssue mirrors textProblem', [FD.textIssue('a'.repeat(1024), 1024), FD.textIssue('ä'.repeat(513), 1024), FD.textIssue('a' + String.fromCharCode(0), 1024)], ['', 'TEXT_TOO_LONG', 'PROP_INVALID']);
    eq('listItems keeps empty lines in place (lists are index-addressed) and drops only trailing ones', [FD.listItems('a\n\n b \n\n', false), FD.listItems('\n', false), FD.listItems('', false)], [['a', '', 'b'], [], []]);
    eq('listItems for keys: empty lines are row breaks, merged, never leading or trailing', [FD.listItems('1\n\n2', true), FD.listItems('\n\n1\n2\n\n\n3\n\n', true), FD.listItems('\n\n', true)], [['1', '\n', '2'], ['1', '2', '\n', '3'], []]);
    const validateRange = name => (validateGo.match(new RegExp('\\{"' + name + '", st\\.\\w+, (\\d+), (\\d+)\\}')) || []).slice(1).map(Number);
    const backlightGo = (validateGo.match(/st\.Backlight < (\d+) \|\| st\.Backlight > (\d+)/) || []).slice(1).map(Number);
    eq('backlight and style ranges mirror validate.go', [LIM.backlight, ['bg_opacity', 'border_width', 'radius'].map(k => [0, LIM.style[k]])], [backlightGo, ['bg_opacity', 'border_width', 'radius'].map(validateRange)]);
    const assetsGo = fs.readFileSync(path.join(root, 'internal', 'cydcreator', 'assets.go'), 'utf8');
    const uploadGo = (assetsGo.match(/MaxAssetUploadBytes = (\d+) << (\d+)/) || []).slice(1).map(Number);
    eq('the upload size limit mirrors MaxAssetUploadBytes', FD.MAX_UPLOAD_BYTES, uploadGo.length ? uploadGo[0] << uploadGo[1] : -1);
    // fakeFile is a File stand-in: name, type, size and slice().arrayBuffer() over its first bytes.
    const PNG = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
    const fakeFile = (name, typeName, head, size) => ({ name, type: typeName, size: size == null ? 1000 : size, slice: (a, b) => ({ arrayBuffer: async () => Uint8Array.from(head.slice(a, b)).buffer }) });
    eq('imageKind mirrors assetExtension (PNG/JPEG magic bytes) plus SVG', await Promise.all([
        fakeFile('a.png', 'image/png', PNG), fakeFile('photo', '', [0xff, 0xd8, 0xff, 0xe0]), fakeFile('x.svg', '', []), fakeFile('a.png', 'image/png', [0x47, 0x49, 0x46, 0x38]), fakeFile('a.png', 'image/png', [])
    ].map(FD.imageKind)), ['png', 'jpeg', 'svg', '', '']);

    const layout = project();
    const home = layout.screens[0];
    home.widgets.push(
        { id: 'w_img', type: 'image', name: 'img_1', x: 10, y: 60, w: 32, h: 32 },
        { id: 'w_msg', type: 'msgbox', name: 'msg_1', x: 0, y: 0, w: 200, h: 100 },
        { id: 'w_led', type: 'led', name: 'led_1', x: 40, y: 60, w: 20, h: 20 },
        { id: 'w_plain', type: 'label', x: 60, y: 60, w: 60, h: 20 },
        { id: 'w_tbl', type: 'table', name: 'tbl_1', x: 0, y: 100, w: 200, h: 80, props: { columns: 2, rows: 2, cells: ['Name', 'Value', '', '42'] } },
        { id: 'w_kp', type: 'keypad', name: 'kp_1', x: 0, y: 0, w: 200, h: 160, props: { layout: 'custom', keys: ['1', '2'] } },
        { id: 'w_img2', type: 'image', name: 'img_2', x: 100, y: 60, w: 400, h: 300 },
        // Events on a type without events (an imported layout): listed so they can be removed.
        { id: 'w_stray', type: 'led', name: 'led_2', x: 80, y: 60, w: 20, h: 20, events: [{ on: 'clicked', action: { type: 'back' } }] });
    home.widgets.find(w => w.id === 'w_tabs').children.push({ id: 'w_tabkid', type: 'label', name: 'lbl_tab', x: 0, y: 0, w: 50, h: 20 });
    const iStore = M.createStore(layout, { coalesceMs: 1200 });
    const origins = [];
    iStore.subscribe((p, info) => origins.push(info.origin));
    const shellBus = P.createBus();
    let jobLock = false;
    const notes = [];
    const hooks = [];
    const inst = {
        t: P.i18n(null), caps, readonly: false, isLocked: () => jobLock, layoutStore: iStore, bus: shellBus, assetUrl: id => '/a/' + id,
        notify: (kind, text) => notes.push(kind + ': ' + text),
        code: { ensureHook: async spec => { hooks.push(spec); return true; } }
    };
    const designer = P.designer.create(inst);
    const d = designer.state;
    // The designer's live region (canvas.announce) is not mounted here: record what is spoken.
    const announced = [];
    d.announce = text => announced.push(text);
    const host = new El('div');
    designer.mountInspector(host);
    const insp = host.kids[0];
    const all = (el, pred, out = []) => { if (pred(el)) out.push(el); el.kids.forEach(k => all(k, pred, out)); return out; };
    const textOf = el => (el.text !== undefined ? el.text : el.kids.map(textOf).join(''));
    const field = key => all(insp, el => el.attrs['data-cc-focus'] === key)[0];
    const panel = () => insp.kids[0] && insp.kids[0].dataset.ccInspect;
    const noteOf = el => all(insp, x => !!x.attrs.id && x.attrs.id === String(el.attrs['aria-describedby'] || '').split(' ')[0])[0];
    const type = (key, value) => { const el = field(key); el.value = value; fire(el, 'input', {}); return el; };
    const pick = (key, value) => { const el = field(key); el.value = value; fire(el, 'change', {}); return el; };
    const tick = (key, on) => { const el = field(key); el.checked = on; fire(el, 'change', {}); return el; };
    const click = key => fire(field(key), 'click', {});
    const widget = id => M.findWidget(iStore.get(), id).widget;
    const settings = () => iStore.get().settings;
    const fieldset = () => all(insp, el => el.tagName === 'FIELDSET')[0];
    const refused = el => el.attrs['aria-invalid'] === 'true' && !!noteOf(el) && noteOf(el).attrs.class === 'cydc-field-error';
    const settle = () => new Promise(resolve => setTimeout(resolve, 0));
    const insideLabel = el => { for (let x = el; x; x = x.parent) if (x.tagName === 'LABEL') return true; return false; };
    const labelOf = el => all(insp, x => x.tagName === 'LABEL' && !!el.attrs.id && x.attrs.for === el.attrs.id)[0];

    // project settings
    check('with nothing selected the inspector shows the project with English fallbacks', panel() === 'project' && textOf(insp).includes('Device language') && textOf(insp).includes('Language and time'));
    click('theme:clear');
    eq('a theme swatch sets the theme', iStore.get().theme, 'clear');
    const grid = all(insp, el => el.attrs.role === 'radiogroup')[0];
    fire(field('theme:synthwave'), 'pointerenter', {});
    const previewed = d.previewTheme;
    fire(grid, 'pointerleave', {});
    check('hovering a swatch previews its theme; leaving the grid ends the preview', previewed === 'synthwave' && d.previewTheme === '');
    field('theme:clear').focus();
    fire(field('theme:clear'), 'keydown', { key: 'ArrowRight' });
    check('an arrow key picks the next theme and focuses its swatch', iStore.get().theme === 'night' && fakeDoc.activeElement === field('theme:night'));
    fire(field('theme:night'), 'blur', {});
    const range = type('backlight', '55');
    const output = all(insp, el => el.tagName === 'OUTPUT')[0];
    eq('the backlight slider stores integers within 10..100 and updates its output in place', [settings().backlight, textOf(output), field('backlight') === range], [55, '55 %', true]);
    type('backlight', '7');
    eq('a backlight below 10 is clamped', settings().backlight, 10);
    pick('orientation', 'portrait');
    pick('language', 'fr');
    eq('orientation and language go through set_project', [iStore.get().orientation, iStore.get().language], ['portrait', 'fr']);
    const tz = type('timezone', 'Zürich');
    check('a time zone with non-ASCII characters is refused and stays in the field', refused(tz) && iStore.get().timezone === 'CET-1CEST,M3.5.0,M10.5.0/3' && tz.value === 'Zürich');
    type('timezone', '   ');
    check('an empty time zone is refused', iStore.get().timezone === 'CET-1CEST,M3.5.0,M10.5.0/3' && refused(field('timezone')));
    type('timezone', ' UTC0 ');
    check('a valid time zone is stored trimmed and clears the message', iStore.get().timezone === 'UTC0' && field('timezone').attrs['aria-invalid'] === 'false');
    tick('lib:onewire', true);
    tick('lib:arduinojson', true);
    eq('libraries are stored in catalog order', settings().libraries, ['arduinojson', 'onewire']);
    tick('aurago_link', true);
    pick('wifi', 'off');
    check('Wi-Fi off also switches the AuraGo link off and disables it', settings().wifi === 'off' && settings().aurago_link === false && field('aurago_link').disabled === true);
    const linkHint = noteOf(field('aurago_link'));
    check('a11y: a checkbox hint is linked with aria-describedby, outside its label', !!linkHint && textOf(linkHint) === 'Needs Wi-Fi.' && !insideLabel(linkHint));
    shellBus.emit('layout:issues', [{ code: 'LIBRARY_UNKNOWN', severity: 'error', path: 'settings/libraries/0', message: 'unknown library' }]);
    check('a new issue list replaces only the issue list (the fields stay)', all(insp, el => el.attrs.class === 'cydc-issue is-error').length === 1 && field('timezone') === all(insp, el => el.attrs['data-cc-focus'] === 'timezone')[0]);
    const nvBoard = Object.assign({}, caps.boards[0], { display: Object.assign({}, caps.boards[0].display, { driver: 'NV3041A', native_orientation: 'landscape' }) });
    inst.caps = Object.assign({}, caps, { boards: [nvBoard] });
    d.bus.emit('lock');
    const orientationOptions = all(field('orientation'), el => el.tagName === 'OPTION');
    eq('an NV3041A board offers only its native orientation family (the stored portrait stays selectable)', orientationOptions.filter(o => o.disabled).map(o => o.value), ['portrait_flipped']);
    inst.caps = caps;
    pick('orientation', 'landscape');

    // widget: name, refused entries, focus and caret
    d.select(['w_btn']);
    check('one selected widget shows its panel', panel() === 'w_btn');
    const nameEl = type('name', 'Bad Name');
    check('an invalid name is refused: the layout keeps the last valid name, the field keeps the entry', widget('w_btn').name === 'btn_1' && refused(nameEl) && nameEl.value === 'Bad Name' && textOf(noteOf(nameEl)).startsWith('Names use lowercase'));
    shellBus.emit('layout:issues', [{ code: 'PROP_INVALID', severity: 'error', widget: 'w_btn', message: 'x' }]);
    check('an issue list after a save keeps the field and the half-typed name', field('name') === nameEl && nameEl.value === 'Bad Name' && all(insp, el => el.attrs.class === 'cydc-issue is-error').length === 1);
    check('a changed issue list of the shown panel is announced', announced[announced.length - 1] === 'Issues: 1', announced.join(' | '));
    const spokenBefore = announced.length;
    shellBus.emit('layout:issues', [{ code: 'PROP_INVALID', severity: 'error', widget: 'w_btn', message: 'x' }, { code: 'NAME_INVALID', severity: 'error', widget: 'w_img', message: 'y' }]);
    check('issues of other widgets change nothing on this panel and are not announced', announced.length === spokenBefore && field('name') === nameEl);
    const moved = JSON.parse(JSON.stringify(iStore.get()));
    moved.screens[0].widgets.find(w => w.id === 'w_btn').y = 12;
    iStore.replace(moved, { resetHistory: false });
    check('a store change re-renders but keeps the refused name with its message', field('name') !== nameEl && field('name').value === 'Bad Name' && refused(field('name')) && field('geo:y').value === '12');
    d.select(['w_img']);
    d.select(['w_btn']);
    check('another panel drops refused entries', field('name').value === 'btn_1' && field('name').attrs['aria-invalid'] !== 'true');
    shellBus.emit('layout:issues', []);
    check('an emptied issue list is announced too', announced[announced.length - 1] === 'No issues left.' && !all(insp, el => el.attrs.class === 'cydc-issue is-error').length);
    type('name', 'lbl_1');
    check('a duplicate name is refused', widget('w_btn').name === 'btn_1' && textOf(noteOf(field('name'))).startsWith('Another widget'));
    type('name', 'init');
    type('name', 'scr_x');
    check('reserved names are refused', widget('w_btn').name === 'btn_1' && refused(field('name')));
    const renamed = type('name', 'go_btn');
    check('a valid name is stored without re-rendering the panel', widget('w_btn').name === 'go_btn' && field('name') === renamed && renamed.attrs['aria-invalid'] === 'false');
    check('a11y: a <label for> names each control; hint, error and counter lines sit outside the label', !!labelOf(renamed) && !insideLabel(noteOf(renamed)) && !!labelOf(field('prop:text')) && !!labelOf(field('geo:x')) && !insideLabel(noteOf(field('geo:x'))) && !!labelOf(field('prop:symbol')));
    const composing = field('name');
    fire(composing, 'compositionstart', {});
    const imeMoved = JSON.parse(JSON.stringify(iStore.get()));
    imeMoved.screens[0].widgets.find(w => w.id === 'w_btn').y = 20;
    iStore.replace(imeMoved, { resetHistory: false });
    const keptDuringIme = field('name') === composing;
    fire(composing, 'compositionend', {});
    check('a re-render waits while an input method composes and runs at compositionend', keptDuringIme && field('name') !== composing && field('geo:y').value === '20');
    const txt = field('prop:text');
    txt.focus();
    txt.selectionStart = 2;
    txt.selectionEnd = 4;
    d.commands.nudge(1, 0);
    check('a store change from elsewhere re-renders and keeps the focus and the caret', field('prop:text') !== txt && fakeDoc.activeElement === field('prop:text') && field('prop:text').selectionStart === 2 && field('prop:text').selectionEnd === 4 && field('geo:x').value === '11');
    const typed = type('prop:text', 'Go');
    check('own edits do not re-render the panel (the caret stays)', widget('w_btn').props.text === 'Go' && field('prop:text') === typed && fakeDoc.activeElement === typed);
    type('prop:text', 'ä'.repeat(513));
    check('a text over 1024 UTF-8 bytes is refused', widget('w_btn').props.text === 'Go' && refused(field('prop:text')) && textOf(noteOf(field('prop:text'))).includes('1024'));
    type('prop:text', 'Button');
    check('the catalog default removes the prop', !widget('w_btn').props);
    type('geo:w', '5000');
    fire(field('geo:w'), 'change', {});
    type('geo:x', '-9999');
    type('geo:y', '12.7');
    type('geo:h', '0');
    type('geo:h', 'abc');
    eq('geometry is stored as integers within the server limits', [widget('w_btn').w, field('geo:w').value, widget('w_btn').x, widget('w_btn').y, widget('w_btn').h], [2048, '2048', -4096, 13, 1]);
    check('a clamped number is announced once per value', announced.filter(s => s === 'Allowed: 1 to 2048. Stored: 2048.').length === 1 && announced.includes('Allowed: -4096 to 4096. Stored: -4096.') && announced.includes('Allowed: 1 to 2048. Stored: 1.'), announced.join(' | '));
    tick('locked', true);
    eq('lock is a widget flag', widget('w_btn').locked, true);
    pick('prop:symbol', 'ok');
    eq('a symbol comes from the catalog list', widget('w_btn').props.symbol, 'ok');

    // props of other kinds and style overrides
    d.select(['w_tabs']);
    type('prop:bar_size', '200');
    pick('prop:bar_position', 'left');
    eq('int props clamp to the catalog range; enums take catalog options', [widget('w_tabs').props.bar_size, widget('w_tabs').props.bar_position], [80, 'left']);
    type('prop:tabs', '');
    check('a tabview keeps at least one tab', !widget('w_tabs').props.tabs && refused(field('prop:tabs')));
    type('prop:tabs', Array.from({ length: 9 }, (_, i) => 'T' + i).join('\n'));
    check('more items than max_len are refused', !widget('w_tabs').props.tabs && textOf(noteOf(field('prop:tabs'))).includes('8'));
    type('prop:tabs', 'A\n\nB\n');
    eq('a list keeps an empty line as an empty item in place (tabs are addressed by index)', widget('w_tabs').props.tabs, ['A', '', 'B']);
    d.select(['w_msg']);
    type('prop:buttons', '\n');
    check('a message box keeps at least one button (ApplyOps refuses an empty list)', !(widget('w_msg').props && widget('w_msg').props.buttons) && refused(field('prop:buttons')));
    type('prop:buttons', 'OK\nCancel');
    eq('message box buttons', widget('w_msg').props.buttons, ['OK', 'Cancel']);
    d.select(['w_tbl']);
    check('the cells editor shows an empty cell as an empty line', field('prop:cells').value === 'Name\nValue\n\n42', JSON.stringify(field('prop:cells').value));
    type('prop:cells', 'Name\nValue\n\n42x');
    eq('typing keeps table cells in place (an empty cell stays an empty item)', widget('w_tbl').props.cells, ['Name', 'Value', '', '42x']);
    type('prop:cells', 'Name\nValue\n\n42x\n\n');
    eq('trailing empty lines add no cells', widget('w_tbl').props.cells, ['Name', 'Value', '', '42x']);
    d.select(['w_kp']);
    type('prop:keys', '\n1\n2\n\n\n3\n');
    eq('keypad keys: one row break between rows, none leading or trailing', widget('w_kp').props.keys, ['1', '2', '\n', '3']);
    type('prop:keys', '\n\n');
    check('a custom keypad keeps at least one key', widget('w_kp').props.keys.join() === '1,2,\n,3' && refused(field('prop:keys')));
    d.select(['w_led']);
    const led = field('prop:color');
    led.value = '#12ab34';
    fire(led, 'input', {});
    led.value = 'red';
    fire(led, 'input', {});
    check('a picked colour is stored as #RRGGBB; anything else is ignored', widget('w_led').props.color === '#12AB34' && textOf(all(insp, el => el.attrs.class === 'cydc-color-value')[0]) === '#12AB34');
    click('prop:color:reset');
    check('"Use theme" removes the colour', !widget('w_led').props);
    const bg = field('style:bg_color');
    bg.value = '#ff0000';
    fire(bg, 'input', {});
    type('style:radius', '99');
    type('style:bg_opacity', '-3');
    const sorted = o => Object.fromEntries(Object.keys(o || {}).sort().map(k => [k, o[k]]));
    eq('style overrides: colours #RRGGBB, numbers clamped to validate.go ranges', sorted(widget('w_led').style), { bg_color: '#FF0000', bg_opacity: 0, radius: 64 });
    type('style:radius', '');
    click('style:bg_color:reset');
    eq('clearing an optional number or "Use theme" removes the override', widget('w_led').style, { bg_opacity: 0 });
    const opacity = field('style:bg_opacity');
    opacity.validity = { badInput: true };
    type('style:bg_opacity', '');
    check('an unreadable entry ("-", "5e") keeps the override and says so', widget('w_led').style.bg_opacity === 0 && refused(opacity) && textOf(noteOf(opacity)) === 'Enter a whole number.');
    opacity.validity = { badInput: false };
    fire(opacity, 'change', {});
    check('leaving the field shows the stored number again', opacity.value === '0' && opacity.attrs['aria-invalid'] === 'false');
    d.select(['w_lbl']);
    pick('prop:size', '20');
    pick('prop:font', 'inter');
    eq('font size is stored as a number, font as a catalog family', [widget('w_lbl').props.size, widget('w_lbl').props.font], [20, 'inter']);
    d.select(['w_tabkid']);
    pick('tab', '1');
    check('the tab page select moves the widget and shows that page', widget('w_tabkid').tab === 1 && d.activeTab('w_tabs') === 1);

    // events and hooks
    d.select(['w_plain']);
    click('event:add');
    const plain = widget('w_plain');
    check('adding an event to an unnamed widget proposes a name (NAME_REQUIRED)', /^lbl_\d+$/.test(plain.name || '') && JSON.stringify(plain.events) === '[{"on":"clicked","action":{"type":"logic","hint":""}}]', JSON.stringify(plain));
    type('event:0:hint', 'x'.repeat(501));
    check('a hint over 500 characters is refused', widget('w_plain').events[0].action.hint === '' && refused(field('event:0:hint')));
    type('event:0:hint', 'Reload the weather');
    eq('the logic hint is stored', widget('w_plain').events[0].action.hint, 'Reload the weather');
    let release = null;
    inst.code.ensureHook = spec => { hooks.push(spec); return new Promise(resolve => { release = resolve; }); };
    click('event:0:hook');
    click('event:0:hook');
    const hookPending = field('event:0:hook').disabled === true;
    release(true);
    await settle();
    eq('"Create hook" calls inst.code.ensureHook({name, event}) once; the button is disabled while it runs', [hooks, hookPending, field('event:0:hook').disabled], [[{ name: widget('w_plain').name, event: 'clicked' }], true, false]);
    inst.code.ensureHook = async () => { throw new Error('invalid hook name'); };
    notes.length = 0;
    click('event:0:hook');
    await settle();
    check('a hook request that throws is reported and the button works again', notes.join() === 'error: cyd_creator.error.generic' && field('event:0:hook').disabled === false, notes.join());
    inst.code.ensureHook = async spec => { hooks.push(spec); return true; };
    type('name', 'temp_lbl');
    check('a new name updates the hook row in place', textOf(all(insp, el => el.attrs.class === 'cydc-hook-name')[0]) === 'on_temp_lbl_clicked()');
    type('name', '');
    check('a widget with events cannot lose its name', widget('w_plain').name === 'temp_lbl' && textOf(noteOf(field('name'))).startsWith('Widgets with events'));
    const wifiOption = all(field('event:0:action'), el => el.tagName === 'OPTION' && el.value === 'open_wifi_setup')[0];
    check('open_wifi_setup is disabled while Wi-Fi is off', !!wifiOption && wifiOption.disabled === true);
    pick('event:0:action', 'goto_screen');
    eq('goto_screen targets another screen by id', widget('w_plain').events[0].action, { type: 'goto_screen', screen: 's_two' });
    pick('event:0:transition', 'fade');
    pick('event:0:screen', 's_home');
    eq('target and transition', widget('w_plain').events[0].action, { type: 'goto_screen', screen: 's_home', transition: 'fade' });
    pick('event:0:transition', '');
    eq('the screen default transition drops the key', widget('w_plain').events[0].action, { type: 'goto_screen', screen: 's_home' });
    pick('event:0:action', 'back');
    eq('back has no other keys', widget('w_plain').events[0].action, { type: 'back' });
    click('event:add');
    check('a second event takes the next free one; then "Add event" is disabled', widget('w_plain').events[1].on === 'long_pressed' && field('event:add').disabled === true);
    pick('event:1:on', 'clicked');
    check('an event bound already cannot be picked twice (its option is disabled, a pick is refused)', all(field('event:1:on'), el => el.tagName === 'OPTION' && el.value === 'clicked')[0].disabled === true && widget('w_plain').events.map(e => e.on).join() === 'clicked,long_pressed');
    click('event:0:remove');
    eq('removing an event', widget('w_plain').events.map(e => e.on), ['long_pressed']);
    check('every action uses only the keys validate.go knows', (widget('w_plain').events || []).every(e => Object.keys(e).join() === 'on,action' && Object.keys(e.action).every(k => ['type', 'screen', 'transition', 'hint'].includes(k))));
    check('Enter and Delete stop at the inspector; Ctrl+S reaches the window', fire(field('event:add'), 'keydown', { key: 'Enter' }).stopped && fire(field('name'), 'keydown', { key: 'Delete' }).stopped && !fire(field('name'), 'keydown', { key: 's', ctrlKey: true }).stopped);
    d.commands.run('edit', [{ op: 'update_widget', id: 'w_plain', set: { events: [{ on: 'long_pressed', action: { type: 'goto_screen', screen: 's_gone' } }] } }]);
    const goneOption = all(field('event:0:screen'), el => el.tagName === 'OPTION' && el.value === 's_gone')[0];
    check('a goto to a deleted screen shows "Missing screen", selected', !!goneOption && textOf(goneOption) === 'Missing screen' && goneOption.selected === true);
    pick('event:0:screen', 's_gone');
    pick('event:0:screen', 's_two');
    eq('only existing screens can be picked as targets', widget('w_plain').events[0].action, { type: 'goto_screen', screen: 's_two' });
    d.select(['w_stray']);
    check('events on a type without events are listed with a remove button and no "Add event"', !!field('event:0:remove') && !field('event:add') && textOf(insp).includes('This widget has no events.'));
    click('event:0:remove');
    check('… and can be removed', !widget('w_stray').events && !field('event:0:remove'));

    // uploads through inst.uploadAsset
    d.select(['w_img']);
    const sun = { id: 'a_sun', file: 'assets/sun.png', w: 48, h: 40, alpha: true };
    let busyShown = false;
    const before = origins.length;
    inst.uploadAsset = async file => {
        busyShown = textOf(insp).includes('Uploading…') && field('prop:asset:upload').disabled === true && file.name === 'sun.png';
        const cur = iStore.get();
        iStore.replace(Object.assign({}, cur, { assets: (cur.assets || []).concat([sun]) }), { resetHistory: false });
        return sun;
    };
    const upload = file => { const input = field('prop:asset:upload'); input.files = [file]; fire(input, 'change', {}); return input; };
    const png = (name, size) => fakeFile(name, 'image/png', PNG, size);
    const uploadInput = upload(png('sun.png'));
    await settle();
    const img = widget('w_img');
    check('an upload shows its progress and clears the file input', busyShown && uploadInput.value === '' && !textOf(insp).includes('Uploading…'));
    eq('the uploaded asset is set on the image, which takes its size; the asset is listed once', [img.props.asset, img.w, img.h, iStore.get().assets.filter(a => a.id === 'a_sun').length], ['a_sun', 48, 40, 1]);
    eq('the shell replaced the store (remote), the prop is one local edit', origins.slice(before), ['remote', 'local']);
    check('the asset select shows the new asset', all(field('prop:asset'), el => el.tagName === 'OPTION' && el.selected === true)[0].value === 'a_sun');
    inst.uploadAsset = async () => ({ id: 'a_moon', file: 'assets/moon.png', w: 10, h: 12, alpha: false, size: 999 });
    upload(png('moon.png'));
    await settle();
    upload(png('moon.png'));
    await settle();
    eq('fallback: an asset missing from the store is added once with only the Asset fields', iStore.get().assets.filter(a => a.id === 'a_moon'), [{ id: 'a_moon', file: 'assets/moon.png', w: 10, h: 12, alpha: false }]);
    inst.uploadAsset = async () => { throw Object.assign(new Error('too large'), { name: 'ApiError', status: 413, code: 'CYD_ASSET_TOO_LARGE' }); };
    notes.length = 0;
    upload(png('big.png'));
    await settle();
    check('a failed upload is reported and the field is usable again', notes.length === 1 && notes[0].startsWith('error: ') && field('prop:asset:upload').disabled !== true && widget('w_img').props.asset === 'a_moon', notes.join());
    pick('prop:asset', '');
    check('"No image" removes the asset prop', !widget('w_img').props);

    // checks before the upload (the server's limits), the SVG raster size, parallel uploads, a lock
    // that arrives during the upload, an answer without an asset
    let calls = 0;
    inst.uploadAsset = async () => { calls++; return sun; };
    notes.length = 0;
    upload(fakeFile('anim.gif', 'image/gif', [0x47, 0x49, 0x46, 0x38, 0x39, 0x61]));
    await settle();
    upload(png('huge.png', FD.MAX_UPLOAD_BYTES + 1));
    await settle();
    check('a file the server would refuse (not PNG/JPEG/SVG, more than 2 MiB) is refused before the upload, with the reason', calls === 0 && notes.join(' | ') === 'error: Only PNG, JPEG and SVG images can be uploaded. | error: The image is larger than 2 MiB.' && !textOf(insp).includes('Uploading…'), notes.join(' | '));
    const realRaster = FD.rasterizeSVG;
    const rasters = [];
    FD.rasterizeSVG = async (file, w, h) => { rasters.push([file.name, w, h]); return png('logo.png', 5000); };
    d.select(['w_img2']);
    upload(fakeFile('logo.svg', 'image/svg+xml', []));
    await settle();
    FD.rasterizeSVG = realRaster;
    check('an SVG (400×300 widget) is rasterised capped at the screen size (320×240) and uploaded', JSON.stringify(rasters) === '[["logo.svg",320,240]]' && calls === 1, JSON.stringify(rasters));
    const pending = [];
    const moonAsset = { id: 'a_moon', file: 'assets/moon.png', w: 10, h: 12, alpha: false };
    inst.uploadAsset = file => new Promise(resolve => pending.push(() => resolve(file.name === 'one.png' ? sun : moonAsset)));
    d.select(['w_img']);
    upload(png('one.png'));
    await settle();
    d.select(['w_img2']);
    upload(png('two.png'));
    await settle();
    pending[0]();
    await settle();
    const stillBusy = textOf(insp).includes('Uploading…') && field('prop:asset:upload').disabled === true;
    upload(png('three.png'));
    await settle();
    pending[1]();
    await settle();
    check('parallel uploads keep their own busy state; a second upload to a busy widget is ignored', stillBusy && pending.length === 2 && !textOf(insp).includes('Uploading…') && widget('w_img').props.asset === 'a_sun' && widget('w_img2').props.asset === 'a_moon');
    inst.uploadAsset = async () => { jobLock = true; shellBus.emit('lock:changed', true, 'ccj_2'); return moonAsset; };
    notes.length = 0;
    d.select(['w_img']);
    upload(png('late.png'));
    await settle();
    check('a lock that arrives during the upload is reported; the widget keeps its image', notes.join() === 'error: The image was uploaded but not set: the project became read-only or locked.' && widget('w_img').props.asset === 'a_sun', notes.join());
    jobLock = false;
    shellBus.emit('lock:changed', false, '');
    inst.uploadAsset = async () => null;
    notes.length = 0;
    upload(png('none.png'));
    await settle();
    check('an answer without an asset is reported', notes.join() === 'error: cyd_creator.error.generic' && widget('w_img').props.asset === 'a_sun', notes.join());

    // read-only, job lock, multi selection, the Wi-Fi system screen
    field('name').focus();
    inst.readonly = true;
    shellBus.emit('readonly:changed', true);
    check('read-only disables the fields and hands the focus to the panel', fieldset().disabled === true && insp.classList.contains('is-readonly') && fakeDoc.activeElement === insp);
    const frozen = JSON.stringify(iStore.get());
    type('name', 'ro_name');
    type('geo:x', '3');
    let roUploads = 0;
    inst.uploadAsset = async () => { roUploads++; return sun; };
    upload(png('sun.png'));
    await settle();
    check('read-only: edits and uploads change nothing', JSON.stringify(iStore.get()) === frozen && roUploads === 0);
    inst.readonly = false;
    shellBus.emit('readonly:changed', false);
    check('read-only off enables the fields again', fieldset().disabled !== true && !insp.classList.contains('is-readonly'));
    jobLock = true;
    shellBus.emit('lock:changed', true, 'ccj_1');
    const lockedSet = fieldset().disabled === true;
    type('geo:x', '3');
    check('a job lock disables the fields and refuses edits', lockedSet && JSON.stringify(iStore.get()) === frozen);
    jobLock = false;
    shellBus.emit('lock:changed', false, '');
    check('the released lock enables them again', fieldset().disabled !== true);
    d.select(['w_btn', 'w_img']);
    check('a multi selection lists the widgets', panel() === 'multi' && all(insp, el => el.tagName === 'LI').length === 2);
    d.commands.run('settings', [{ op: 'set_settings', set: { wifi: 'optional' } }]);
    d.setScreen(d.SYSTEM_WIFI);
    check('the Wi-Fi system screen shows its hint', panel() === 'system' && !fieldset());
    d.setScreen('s_home');
    check('every inspector edit reached the store with origin local (remote: two test replaces and the upload)', origins.filter(o => o !== 'local').join() === 'remote,remote,remote', origins.join());

    designer.dispose();
    d.bus.emit('lock');
    check('dispose removes the inspector', !host.kids.length && d.previewTheme === '');
}

// ── screens panel and layers (cyd-creator-screens.js) through the real designer and interact ──
// Screen list (open, add, inline rename with refused names, duplicate, delete through inst.confirm,
// reorder, start marker, issue count), the Wi-Fi system entry, settings of the active screen, the
// layer list (order, depth, flags, roving tab stop, list keys before interact's keys), the keys the
// desktop acts on, one render per change, the focus kept across store changes and the lock signal.
{
    const screensFile = path.join(apps, 'cyd-creator-screens.js');
    vm.runInContext(fs.readFileSync(screensFile, 'utf8'), sandbox, { filename: screensFile });
    const timers = new Map();
    let timerSeq = 0;
    sandbox.setTimeout = fn => { timers.set(++timerSeq, fn); return timerSeq; };
    sandbox.clearTimeout = id => { timers.delete(id); };
    const runTimers = () => Array.from(timers.entries()).forEach(([id, fn]) => { timers.delete(id); fn(); });
    const settle = () => new Promise(resolve => setTimeout(resolve, 0));
    const realCanvas = P.designerCanvas;
    const croot = new El('div');
    const viewport = croot.appendChild(new El('div'));
    viewport.getBoundingClientRect = () => ({ left: 0, top: 0, right: 600, bottom: 400 });
    const said = [];
    P.designerCanvas = { mount: () => ({
        root: croot, viewport, render() {}, fit() {}, dispose() {}, previewMove() {}, previewResize() {},
        showGuides() {}, showMarquee() {}, showDrop() {}, announce(text) { said.push(text); },
        hostRect: () => ({ left: 0, top: 0, right: 320, bottom: 240 }), clientToWorld: (x, y) => ({ x, y })
    }) };
    const layout = project();
    const home = layout.screens[0];
    home.widgets.find(w => w.id === 'w_tabs').children.push({ id: 'w_tk', type: 'label', name: 'lbl_tk', x: 0, y: 0, w: 40, h: 20, tab: 1 });
    home.widgets.push({ id: 'w_hid', type: 'label', name: 'lbl_hid', x: 0, y: 0, w: 10, h: 10, hidden: true, locked: true });
    const sStore = M.createStore(layout, { coalesceMs: 1200 });
    const origins = [];
    sStore.subscribe((p, info) => origins.push(info.origin));
    const shellBus = P.createBus();
    let jobLock = false;
    let answer = false;
    const asked = [];
    const notes = [];
    const inst = {
        t: P.i18n(null), caps, readonly: false, isLocked: () => jobLock, layoutStore: sStore, bus: shellBus, assetUrl: id => '/a/' + id,
        notify: (kind, text) => notes.push(kind + ': ' + text),
        confirm: async spec => { asked.push(spec); return typeof answer === 'function' ? answer() : answer; }
    };
    sandbox.window.I18N = { 'cyd_creator.designer.screens_title': 'Bildschirme' };
    const designer = P.designer.create(inst);
    const d = designer.state;
    designer.mountCanvas(new El('div'));
    const host = new El('div');
    fakeDoc.activeElement = null;
    designer.mountScreens(host);
    const scr = host.kids[0];
    let renders = 0;
    const replaceRoot = scr.replaceChildren;
    scr.replaceChildren = function (...nodes) { renders++; return replaceRoot.apply(this, nodes); };
    const all = (el, pred, out = []) => { if (pred(el)) out.push(el); el.kids.forEach(k => all(k, pred, out)); return out; };
    const textOf = el => (el.text !== undefined ? el.text : el.kids.map(textOf).join(''));
    const byKey = key => all(scr, el => el.attrs['data-cc-focus'] === key)[0];
    const rows = () => all(scr, el => el.dataset.ccScreenId !== undefined);
    const rowOf = id => rows().find(el => el.dataset.ccScreenId === id);
    const renameInput = () => all(scr, el => el.dataset.ccRename !== undefined)[0];
    const layerSel = () => all(scr, el => el.dataset.ccLayerId !== undefined);
    const layer = id => layerSel().find(el => el.dataset.ccLayerId === id);
    const tabStops = () => layerSel().filter(el => el.attrs.tabindex === '0').map(el => el.dataset.ccLayerId);
    const screenIds = () => sStore.get().screens.map(s => s.id);
    const screenOf = id => sStore.get().screens.find(s => s.id === id);
    const type = (el, value) => { el.value = value; fire(el, 'input', {}); return el; };
    const key = (el, k, mods) => fire(el, 'keydown', Object.assign({ key: k }, mods));
    const used = ev => ev.defaultPrevented && ev.stopped;
    const tools = () => all(scr, el => !!el.attrs['data-cc-cmd'] && el.attrs['data-cc-cmd'] !== 'screen-open');

    // structure: screens in order with the system entry, start marker, active entry, names, issue counts
    eq('screen rows in layout order, then the Wi-Fi system entry (Wi-Fi is not off)', rows().map(el => el.dataset.ccScreenId), ['s_home', 's_two', d.SYSTEM_WIFI]);
    check('only the start screen carries the start marker', all(rowOf('s_home'), el => el.attrs.class === 'cydc-start').length === 1 && all(rowOf('s_two'), el => el.attrs.class === 'cydc-start').length === 0);
    eq('the active screen\'s entry is aria-current', [byKey('open:s_home').attrs['aria-current'], byKey('open:s_two').attrs['aria-current']], ['true', undefined]);
    check('translated headings and English fallbacks', textOf(scr).includes('Bildschirme') && byKey('screen-add').attrs['aria-label'] === 'Add screen' && textOf(scr).includes('Layers'));
    shellBus.emit('layout:issues', [{ code: 'SCREEN_NAME_INVALID', severity: 'error', screen: 's_two', message: 'x' }, { code: 'NAME_INVALID', severity: 'error', widget: 'w_btn', message: 'bad name' }]);
    const count = all(rowOf('s_two'), el => el.attrs.class === 'cydc-count is-error')[0];
    eq('a screen\'s issue count shows the number and gives screen readers words', count && count.kids.map(k => [k.attrs['aria-hidden'] || k.attrs.class, textOf(k)]), [['true', '1'], ['cydc-sr-only', 'Issues: 1']]);
    check('a widget issue marks its layer with the issue text', all(layer('w_btn').parent, el => el.attrs.class === 'cydc-issue-mark is-error' && el.attrs.title === 'bad name').length === 1);
    check('… and its accessible name counts the issues', layer('w_btn').attrs['aria-label'] === 'btn_1, button, Issues: 1', layer('w_btn').attrs['aria-label']);

    // layers: front-most first, nested by depth, states in the accessible name, one tab stop
    eq('layers list front-most first, children after their container', layerSel().map(el => el.dataset.ccLayerId), ['w_hid', 'w_tabs', 'w_tk', 'w_pnl', 'w_lbl', 'w_btn']);
    eq('… indented by depth', layerSel().map(el => el.parent.style['--cc-depth']), ['0', '0', '1', '0', '1', '0']);
    eq('… with the depth as aria-level', layerSel().map(el => el.parent.attrs['aria-level']), ['1', '1', '2', '1', '2', '1']);
    eq('the accessible name holds name, type and state', [layer('w_hid').attrs['aria-label'], layer('w_tk').attrs['aria-label']], ['lbl_hid, label, locked, hidden', 'lbl_tk, label, tab 2']);
    eq('one tab stop: the first layer while nothing is selected', tabStops(), ['w_hid']);
    fire(layer('w_tk'), 'click', {});
    check('a layer on another tab page opens that page', d.selection.join() === 'w_tk' && d.activeTab('w_tabs') === 1);
    d.setTab('w_tabs', 0);
    fire(layer('w_tk'), 'click', { shiftKey: true });
    check('… but not when the click unselects it', !d.selection.length && d.activeTab('w_tabs') === 0);
    let before = renders;
    fire(all(layer('w_lbl'), el => el.attrs.class === 'cydc-layer-name')[0], 'click', {});
    check('a click on a layer (also on its text) selects the widget and renders once', d.selection.join() === 'w_lbl' && renders === before + 1);
    check('… marks it pressed and makes it the tab stop', layer('w_lbl').attrs['aria-pressed'] === 'true' && layer('w_lbl').parent.attrs.class.includes('is-selected') && tabStops().join() === 'w_lbl');
    fire(layer('w_btn'), 'click', { shiftKey: true });
    eq('Shift+click adds to the selection', d.selection, ['w_lbl', 'w_btn']);

    // layer flags keep the focus on the flag
    byKey('layer-lock:w_btn').focus();
    fire(byKey('layer-lock:w_btn'), 'click', {});
    check('the lock flag locks the widget and keeps the focus on the (new) flag', M.findWidget(sStore.get(), 'w_btn').widget.locked === true && fakeDoc.activeElement === byKey('layer-lock:w_btn') && byKey('layer-lock:w_btn').attrs['aria-pressed'] === 'true');
    fire(byKey('layer-lock:w_btn'), 'click', {});
    fire(byKey('layer-hide:w_btn'), 'click', {});
    check('the hide flag hides it; unlock worked', M.findWidget(sStore.get(), 'w_btn').widget.hidden === true && !M.findWidget(sStore.get(), 'w_btn').widget.locked && layer('w_btn').parent.attrs.class.includes('is-hidden'));
    fire(byKey('layer-hide:w_btn'), 'click', {});

    // layer keys: the list keys move the focus (they win over nudging), the others go to interact
    d.select(['w_btn']);
    layer('w_hid').focus();
    const down = key(layer('w_hid'), 'ArrowDown');
    check('ArrowDown moves the focus to the next layer and makes it the tab stop', down.defaultPrevented && fakeDoc.activeElement === layer('w_tabs') && tabStops().join() === 'w_tabs');
    key(layer('w_tabs'), 'End');
    check('End goes to the last layer', fakeDoc.activeElement === layer('w_btn'));
    key(layer('w_btn'), 'Home');
    key(layer('w_hid'), 'ArrowUp', { shiftKey: true });
    check('Home goes to the first; (Shift+)ArrowUp at the top stays there', fakeDoc.activeElement === layer('w_hid') && tabStops().join() === 'w_hid');
    eq('list keys nudge nothing', M.findWidget(sStore.get(), 'w_btn').widget.x, 10);
    layer('w_btn').focus();
    check('ArrowRight on a layer goes to interact and nudges the selection', used(key(layer('w_btn'), 'ArrowRight')) && M.findWidget(sStore.get(), 'w_btn').widget.x === 11);
    const enter = key(layer('w_btn'), 'Enter');
    const f2 = key(layer('w_btn'), 'F2');
    eq('Enter (native click) and F2 on a layer stop at the panel without being prevented', [enter.stopped, enter.defaultPrevented, f2.stopped, f2.defaultPrevented], [true, false, true, false]);
    check('Ctrl+S on a layer bubbles on to the shell', !key(layer('w_btn'), 's', { ctrlKey: true }).stopped);
    check('Delete on a layer is used by interact', used(key(layer('w_btn'), 'Delete')) && !M.findWidget(sStore.get(), 'w_btn'));
    check('… and the focus moves to the layer tab stop, not the page', fakeDoc.activeElement === layer('w_hid') && tabStops().join() === 'w_hid');
    check('Ctrl+Z on a layer is used and brings the widget back', used(key(layer('w_hid'), 'z', { ctrlKey: true })) && !!M.findWidget(sStore.get(), 'w_btn'));
    d.select([]);

    // inline rename: Enter commits, refused names stay with the reason, Escape cancels, blur commits
    fire(byKey('screen-rename:s_home'), 'click', {});
    const ri = renameInput();
    check('the rename tool opens the field with the name selected and focused', !!ri && ri.value === 'home' && fakeDoc.activeElement === ri && ri.selectionStart === 0 && ri.selectionEnd === 4);
    // The reason describes the field (aria-describedby) and is spoken once per change, not per render.
    const errOf = () => all(scr, el => !!el.attrs.id && el.attrs.id === renameInput().attrs['aria-describedby'])[0];
    const spoken = said.length;
    type(renameInput(), 'Main Screen');
    const bad = key(renameInput(), 'Enter');
    check('an invalid name is refused with the reason; the field keeps entry and focus', screenOf('s_home').name === 'home' && renameInput().value === 'Main Screen' && renameInput().attrs['aria-invalid'] === 'true' && fakeDoc.activeElement === renameInput() && !!errOf() && textOf(errOf()).startsWith('Screen names use'));
    eq('… Enter stops at the panel (the desktop would open its selected icon)', [bad.stopped, bad.defaultPrevented], [true, true]);
    check('… the reason is spoken once, from a description without role=alert', said.length === spoken + 1 && said[spoken].startsWith('Screen names use') && !all(scr, el => el.attrs.role === 'alert').length);
    type(renameInput(), 'Bad Name');
    key(renameInput(), 'Enter');
    shellBus.emit('layout:issues', [{ code: 'X', severity: 'warning', message: 'x' }]);
    check('the same reason again and re-renders speak nothing new', said.length === spoken + 1 && textOf(errOf()).startsWith('Screen names use'));
    type(renameInput(), 'details');
    key(renameInput(), 'Enter');
    check('a duplicate screen name is refused and its reason spoken', screenOf('s_home').name === 'home' && textOf(errOf()) === 'Another screen already uses this name.' && said.length === spoken + 2);
    type(renameInput(), ' main ');
    key(renameInput(), 'Enter');
    check('a valid name is stored trimmed; the focus returns to the screen\'s entry', screenOf('s_home').name === 'main' && !renameInput() && fakeDoc.activeElement === byKey('open:s_home'));
    const f2open = key(byKey('open:s_home'), 'F2');
    check('F2 on a screen entry renames it and stops at the panel', used(f2open) && !!renameInput() && fakeDoc.activeElement === renameInput());
    type(renameInput(), 'zzz');
    const ime = [key(renameInput(), 'Enter', { isComposing: true }), key(renameInput(), 'Escape', { keyCode: 229 })];
    check('Enter and Escape of an IME composition leave the rename alone', !!renameInput() && renameInput().value === 'zzz' && ime.every(ev => !ev.defaultPrevented) && screenOf('s_home').name === 'main');
    const esc = key(renameInput(), 'Escape');
    check('Escape cancels the rename, stops there and refocuses the entry', used(esc) && screenOf('s_home').name === 'main' && !renameInput() && fakeDoc.activeElement === byKey('open:s_home'));
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'kept');
    fire(renameInput(), 'focusout', {});
    shellBus.emit('layout:issues', []);
    runTimers();
    check('a re-render keeps the rename (entry, focus) and its blur commits nothing', !!renameInput() && renameInput().value === 'kept' && fakeDoc.activeElement === renameInput() && screenOf('s_home').name === 'main');
    const remote = JSON.parse(JSON.stringify(sStore.get()));
    remote.screens[1].title = 'Remote';
    sStore.replace(remote, { resetHistory: false });
    check('a remote store change keeps the rename entry and its focus', !!renameInput() && renameInput().value === 'kept' && fakeDoc.activeElement === renameInput());
    croot.focus();
    fire(renameInput(), 'focusout', {});
    runTimers();
    check('a blur commits the name and leaves the focus where the user put it', screenOf('s_home').name === 'kept' && !renameInput() && fakeDoc.activeElement === croot);
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), '9x');
    croot.focus();
    fire(renameInput(), 'focusout', {});
    runTimers();
    check('a blur with an invalid name keeps the field with the reason without taking the focus back', !!renameInput() && renameInput().attrs['aria-invalid'] === 'true' && fakeDoc.activeElement === croot && screenOf('s_home').name === 'kept');
    key(renameInput(), 'Escape');
    // A press in the panel moves the focus (blur) before its click: the commit waits for the release,
    // so the pressed element is still there when the click arrives.
    const winFire = (type, target) => (winListeners[type] || []).slice().forEach(fn => fn({ type, target }));
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'pressed');
    const pressedEntry = byKey('open:s_two');
    winFire('pointerdown', pressedEntry);
    pressedEntry.focus();
    fire(renameInput(), 'focusout', {});
    runTimers();
    check('during a press in the panel the blur commit waits; the pressed entry stays', !!renameInput() && scr.contains(pressedEntry) && screenOf('s_home').name === 'kept');
    winFire('pointerup', pressedEntry);
    fire(pressedEntry, 'click', {});
    check('… so its click arrives (the screen opens)', d.screenId === 's_two' && screenOf('s_home').name === 'kept');
    runTimers();
    check('… and the release commits the name, leaving the focus on the clicked entry', screenOf('s_home').name === 'pressed' && !renameInput() && fakeDoc.activeElement === byKey('open:s_two'));
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'outside');
    winFire('pointerdown', croot);
    croot.focus();
    fire(renameInput(), 'focusout', {});
    runTimers();
    check('a press outside the panel commits at once', screenOf('s_home').name === 'outside' && !renameInput());
    winFire('pointerup', croot);
    // A click on another screen's rename tool while renaming: the press defers the first commit,
    // the click starts the second rename, which commits the first one before (or keeps it, refused).
    const renameOther = () => {
        const tool = byKey('screen-rename:s_two');
        winFire('pointerdown', tool);
        tool.focus();
        fire(renameInput(), 'focusout', {});
        runTimers();
        winFire('pointerup', tool);
        fire(tool, 'click', {});
        runTimers();
    };
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'first');
    renameOther();
    check('a click on another screen\'s rename tool commits the first rename and starts the second', screenOf('s_home').name === 'first' && renameInput() && renameInput().dataset.ccRename === 's_two' && fakeDoc.activeElement === renameInput());
    key(renameInput(), 'Escape');
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'Bad Name');
    renameOther();
    check('… a refused first name keeps its field with the reason; the second does not start', screenOf('s_home').name === 'first' && all(scr, el => el.dataset.ccRename !== undefined).map(el => el.dataset.ccRename).join() === 's_home' && renameInput().value === 'Bad Name' && renameInput().attrs['aria-invalid'] === 'true' && screenOf('s_two').name === 'details');
    // A press whose release never arrives (released outside the browser) ends when the window loses
    // the focus: a later keyboard blur commits at once.
    renameInput().focus();
    type(renameInput(), 'held');
    winFire('pointerdown', byKey('screen-add'));
    winFire('blur', sandbox.window);
    croot.focus();
    fire(renameInput(), 'focusout', {});
    runTimers();
    check('leaving the window ends a press: the next blur commits without waiting', screenOf('s_home').name === 'held' && !renameInput());
    fire(byKey('screen-rename:s_home'), 'click', {});
    type(renameInput(), 'pressed');
    key(renameInput(), 'Enter');

    // add, duplicate, reorder, start screen
    fire(byKey('screen-add'), 'click', {});
    const added = sStore.get().screens[2];
    check('add appends a proposed name, opens it and announces it', !!added && added.name === 'screen_1' && d.screenId === added.id && said[said.length - 1] === 'Screen added');
    before = renders;
    d.setScreen('s_home');
    check('a screen switch renders once (screen, then an unchanged selection)', renders === before + 1 && byKey('open:s_home').attrs['aria-current'] === 'true');
    const n0 = screenIds().length;
    fire(byKey('screen-duplicate:s_home'), 'click', {});
    const copy = sStore.get().screens[1];
    const origIds = []; const origNames = []; M.walk({ screens: [screenOf('s_home')] }, w => { origIds.push(w.id); origNames.push(w.name); });
    const copyIds = []; const copyNames = []; M.walk({ screens: [copy] }, w => { copyIds.push(w.id); copyNames.push(w.name); });
    check('duplicate inserts the copy right after the original and opens it', screenIds().length === n0 + 1 && copy.name === 'pressed_1' && d.screenId === copy.id && copy.title === 'Start');
    check('… with fresh widget ids and names', copyIds.length === origIds.length && copyIds.every(id => !origIds.includes(id)) && copyNames.every(n => !origNames.includes(n)));
    sStore.undo();
    check('… as one undo step', screenIds().length === n0 && d.screenId === 's_home');
    byKey('open:s_home').focus();
    const altDown = key(byKey('open:s_home'), 'ArrowDown', { altKey: true });
    check('Alt+ArrowDown moves the screen down and keeps the focus on its entry', used(altDown) && screenIds().indexOf('s_home') === 1 && fakeDoc.activeElement === byKey('open:s_home'));
    const workspace = key(byKey('open:s_home'), 'ArrowDown', { altKey: true, ctrlKey: true });
    check('Ctrl+Alt+ArrowDown stays with the desktop', !workspace.defaultPrevented && !workspace.stopped && screenIds().indexOf('s_home') === 1);
    check('the move tools are disabled at the edges', byKey('screen-up:' + screenIds()[0]).disabled === true && byKey('screen-down:' + screenIds()[screenIds().length - 1]).disabled === true);
    byKey('screen-up:s_home').focus();
    fire(byKey('screen-up:s_home'), 'click', {});
    check('a tool that disables itself hands the focus to its screen\'s entry', screenIds()[0] === 's_home' && byKey('screen-up:s_home').disabled === true && fakeDoc.activeElement === byKey('open:s_home'));
    fire(byKey('screen-start:s_two'), 'click', {});
    check('the star makes a screen the start screen (by id) and is disabled there', sStore.get().start_screen === 's_two' && byKey('screen-start:s_two').disabled === true && all(rowOf('s_two'), el => el.attrs.class === 'cydc-start').length === 1);

    // delete through inst.confirm
    fire(byKey('open:s_two'), 'click', {});
    byKey('screen-delete:s_two').focus();
    fire(byKey('screen-delete:s_two'), 'click', {});
    await settle();
    eq('delete asks through inst.confirm', asked[0], { titleKey: 'cyd_creator.designer.screen_delete_title', bodyKey: 'cyd_creator.designer.screen_delete_body', vars: { name: 'details' }, okKey: 'cyd_creator.designer.delete', danger: true });
    check('… a cancelled dialog deletes nothing', !!screenOf('s_two'));
    answer = true;
    before = renders;
    fire(byKey('screen-delete:s_two'), 'click', {});
    await settle();
    check('a confirmed delete removes the screen and moves the start screen', !screenOf('s_two') && sStore.get().start_screen === screenIds()[0] && d.screenId === screenIds()[0]);
    check('… renders once (screen first, project marked screenChanged) and focuses the active entry', renders === before + 1 && fakeDoc.activeElement === byKey('open:' + d.screenId), 'renders ' + (renders - before));
    answer = () => { const next = JSON.parse(JSON.stringify(sStore.get())); next.screens = next.screens.filter(s => s.id !== added.id); sStore.replace(next, { resetHistory: false }); return true; };
    fire(byKey('screen-delete:' + added.id), 'click', {});
    await settle();
    check('a screen removed elsewhere during the dialog is skipped without an error', notes.length === 0 && screenIds().join() === 's_home');
    check('the last screen cannot be deleted', byKey('screen-delete:s_home').disabled === true);
    answer = false;

    // settings of the active screen
    const titleEl = byKey('screen-title');
    const entryTitle = () => all(rowOf('s_home'), el => el.attrs['data-cc-title'] === 's_home')[0];
    titleEl.focus();
    before = renders;
    type(titleEl, 'Hello');
    type(titleEl, 'Hello!');
    check('the title is stored while typing without re-rendering the panel', screenOf('s_home').title === 'Hello!' && byKey('screen-title') === titleEl && fakeDoc.activeElement === titleEl && renders === before);
    check('… and the entry shows it in place', textOf(entryTitle()) === 'Hello!');
    // Chrome fires change inside the blur (the focus is on the page then): a re-render there would
    // drop the focus to the page and lose the click that moved it.
    fakeDoc.activeElement = fakeDoc.body;
    fire(titleEl, 'change', {});
    check('the title\'s change (at blur) re-renders nothing', renders === before && byKey('screen-title') === titleEl);
    type(titleEl, '');
    check('an empty title empties the entry\'s title in place', textOf(entryTitle()) === '' && renders === before);
    type(titleEl, 'Hello!');
    const tr = byKey('screen-transition');
    tr.value = 'slide_left'; fire(tr, 'change', {});
    byKey('screen-transition').value = 'zoom'; fire(byKey('screen-transition'), 'change', {});
    eq('a transition comes from the list; others are ignored', screenOf('s_home').transition, 'slide_left');
    const bgEl = byKey('screen-bg');
    before = renders;
    type(bgEl, '#12ab34');
    type(bgEl, 'red');
    eq('the background is stored as #RRGGBB; anything else is ignored', screenOf('s_home').style, { bg_color: '#12AB34' });
    fire(bgEl, 'change', {});
    check('… the "Use theme" chip shows it in place; nothing re-renders', byKey('screen-bg-reset').attrs['aria-pressed'] === 'false' && byKey('screen-bg') === bgEl && renders === before);
    fire(byKey('screen-bg-reset'), 'click', {});
    check('"Use theme" removes the background, shows the chip pressed and the theme colour', !screenOf('s_home').style.bg_color && byKey('screen-bg-reset').attrs['aria-pressed'] === 'true' && byKey('screen-bg').value === caps.themes.find(x => x.id === sStore.get().theme).tokens.bg.toLowerCase());

    // the Wi-Fi system entry: a locked, view-only screen
    before = renders;
    fire(byKey('open:system'), 'click', {});
    check('the system entry opens the Wi-Fi setup screen with one render', d.isSystem() && renders === before + 1);
    check('… without screen settings and with the system hint instead of layers', !byKey('screen-title') && !layerSel().length && textOf(scr).includes('cannot be edited'));
    check('… and without tools on its entry', all(rowOf(d.SYSTEM_WIFI), el => !!el.attrs['data-cc-cmd'] && el.attrs['data-cc-cmd'] !== 'screen-open').length === 0);
    check('F2 on it renames nothing but stops at the panel', key(byKey('open:system'), 'F2').stopped && !renameInput());
    const del = key(byKey('open:s_home'), 'Delete');
    check('Delete on a screen entry stops at the panel and deletes nothing', del.stopped && !del.defaultPrevented && screenIds().join() === 's_home');
    before = renders;
    d.commands.run('settings', [{ op: 'set_settings', set: { wifi: 'off' } }]);
    check('Wi-Fi off removes the system entry and returns to the start screen with one render', !byKey('open:system') && d.screenId === 's_home' && renders === before + 1);
    d.commands.run('settings', [{ op: 'set_settings', set: { wifi: 'optional' } }]);

    // the lock signal: read-only and a job lock disable tools, fields and flags; opening and selecting stay
    fire(byKey('screen-rename:s_home'), 'click', {});
    const frozen = () => JSON.stringify(sStore.get());
    inst.readonly = true;
    shellBus.emit('readonly:changed', true);
    check('read-only ends a rename and hands the focus to the screen\'s entry', !renameInput() && fakeDoc.activeElement === byKey('open:s_home'));
    check('… disables every tool, flag and field and marks the panel', tools().every(el => el.disabled === true) && all(scr, el => el.tagName === 'FIELDSET')[0].disabled === true && scr.classList.contains('is-readonly'));
    check('… while screens still open and layers still select', !byKey('open:s_home').disabled && layerSel().every(el => !el.disabled));
    const kept = frozen();
    key(byKey('open:s_home'), 'F2');
    fire(byKey('screen-add'), 'click', {});
    fire(byKey('screen-duplicate:s_home'), 'click', {});
    d.select(['w_btn']);
    key(layer('w_btn'), 'Delete');
    type(byKey('screen-title'), 'ro');
    check('read-only: F2, tools, Delete and fields change nothing', frozen() === kept && !renameInput() && d.selection.join() === 'w_btn');
    inst.readonly = false;
    shellBus.emit('readonly:changed', false);
    check('read-only off enables them again', tools().filter(el => el.disabled).every(el => /^screen-(up|down|delete|start):/.test(el.attrs['data-cc-focus'])) && !scr.classList.contains('is-readonly'));
    byKey('screen-add').focus();
    jobLock = true;
    shellBus.emit('lock:changed', true, 'ccj_1');
    check('a job lock disables the tools and moves the focus off a disabled one', byKey('screen-add').disabled === true && fakeDoc.activeElement === byKey('open:s_home'));
    jobLock = false;
    shellBus.emit('lock:changed', false, '');
    check('the released lock enables them again', !byKey('screen-add').disabled);
    check('every panel edit reached the store as local (remote: the test replaces; undo: Ctrl+Z and the duplicate check)', origins.filter(o => o !== 'local').join() === 'undo,remote,undo,remote', origins.join());

    // the screen limit (M.LIMITS.screens = 16, limits.go MaxScreens)
    const full = JSON.parse(JSON.stringify(sStore.get()));
    for (let i = full.screens.length; i < M.LIMITS.screens; i++) full.screens.push({ id: 's_x' + i, name: 'x' + i, title: '', transition: 'none', style: {}, widgets: [] });
    sStore.replace(full, { resetHistory: false });
    check('at 16 screens add and every duplicate are disabled', M.LIMITS.screens === 16 && screenIds().length === 16 && byKey('screen-add').disabled === true && full.screens.every(s => byKey('screen-duplicate:' + s.id).disabled === true));
    full.screens.pop();
    sStore.replace(full, { resetHistory: false });
    check('at 15 screens they are enabled again', screenIds().length === 15 && !byKey('screen-add').disabled && full.screens.every(s => !byKey('screen-duplicate:' + s.id).disabled));

    designer.dispose();
    shellBus.emit('layout:issues', [{ code: 'X', severity: 'error', screen: 's_home' }]);
    check('dispose removes the panel and its listeners', !host.kids.length && !(scr.listeners.keydown || []).length && !timers.size);
    delete sandbox.window.I18N;
    P.designerCanvas = realCanvas;
}

console.log((failures ? 'FAILED ' + failures : 'ok') + ' — ' + checks + ' checks');
process.exit(failures ? 1 : 0);
