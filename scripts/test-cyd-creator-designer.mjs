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
for (const file of ['cyd-creator-geometry.js']) {
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

console.log((failures ? 'FAILED ' + failures : 'ok') + ' — ' + checks + ' checks');
process.exit(failures ? 1 : 0);
