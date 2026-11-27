import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

// Round 11 view suite: the Brandur renderer and the interior-view hooks run against a recording stub context.
// Fake interior states are built here, so the suite does not depend on the timing of brandur.js.

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const appDir = path.join(rootDir, 'ui/js/desktop/apps/runenpfad');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, WeakMap, JSON, Date, Uint8Array, Promise, isFinite, parseInt };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

const MODULES = ['art', 'interior-bake', 'interior-things-view', 'brandur-view', 'deeps-map-view', 'interior-view'];
for (const m of MODULES) {
    const abs = path.join(appDir, `${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}
assert.ok(sandbox.RunenpfadBrandurView && sandbox.RunenpfadInteriorView && sandbox.RunenpfadArt, 'view modules loaded');

let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed += 1;
    } catch (err) {
        console.error('FAILED: ' + name);
        throw err;
    }
}

// ---- recording stub context ----

function makeCtx() {
    const log = { images: [], fills: [], strokes: [], scales: [], calls: 0 };
    const store = {};
    let ctx = null;
    const canvasStub = () => ({ width: 1, height: 1, getContext: () => makeCtx().ctx, ownerDocument: null });
    store.canvas = { ownerDocument: { createElement: canvasStub }, width: 900, height: 700, closest: () => null, getBoundingClientRect: () => ({ top: 0, bottom: 0 }) };
    ctx = new Proxy(store, {
        get(t, k) {
            if (k in t) return t[k];
            if (k === 'createRadialGradient' || k === 'createLinearGradient') return () => ({ addColorStop() {} });
            if (k === 'measureText') return () => ({ width: 40 });
            if (k === 'scale') return (x, y) => { log.calls++; log.scales.push([x, y]); };
            if (k === 'drawImage') return (img) => { log.calls++; log.images.push(img && img.id ? img.id : '?'); };
            return () => { log.calls++; };
        },
        set(t, k, v) {
            if (k === 'fillStyle') log.fills.push(String(v));
            if (k === 'strokeStyle') log.strokes.push(String(v));
            t[k] = v;
            return true;
        }
    });
    return { ctx, log };
}

// Fake atlas from the real atlas.json: image objects carry the sheet id so draws can be attributed.
const atlasJson = JSON.parse(fs.readFileSync(path.join(rootDir, 'ui/img/runenpfad/atlas.json'), 'utf8')).sheets;
function fakeAtlas() {
    const atlas = { sheets: {}, clips: {}, lazy: {}, ver: '' };
    const want = ['td-boss-brandur', 'td-brandur-b', 'td-brandur-air', 'td-fx-breath', 'td-fx-hazards', 'tdp-lair', 'tdp-home', 'tdp-house', 'tex-glut', 'tex-mine'];
    for (const id of want) {
        const sh = atlasJson[id];
        if (!sh) continue;
        const sheet = { img: { id }, frames: sh.frames, refH: sh.refH };
        atlas.sheets[id] = sheet;
        for (const [name, c] of Object.entries(sh.clips)) {
            const clip = { sheet, name, frames: c.frames, ms: c.ms, loop: !!c.loop };
            atlas.clips[`${id}.${name}`] = clip;
            if (name === sh.default) atlas.clips[id] = clip;
        }
    }
    return atlas;
}

// ---- fake interior ----

const COLS = 20;
const ROWS = 14;
function buildRoom() {
    const map = [];
    for (let cy = 0; cy < ROWS; cy++) {
        let row = '';
        for (let cx = 0; cx < COLS; cx++) {
            if (cx === 0 || cy === 0 || cx === COLS - 1 || cy === ROWS - 1) row += '#';
            else if (cx === 1 || cx === COLS - 2 || cy === 1 || cy === ROWS - 2) row += '~';
            else row += '.';
        }
        map.push(row);
    }
    const lava = new Uint8Array(COLS * ROWS);
    map.forEach((r, cy) => { for (let cx = 0; cx < COLS; cx++) if (r[cx] === '~') lava[cy * COLS + cx] = 1; });
    const def = { theme: 'glut', map, view: 15, big: true, guardian: 'brandur', dark: 0.35, things: [] };
    return { id: 'dp_lair', def, cols: COLS, rows: ROWS, w: COLS * 48, h: ROWS * 48, solid: new Uint8Array(COLS * ROWS), walls: new Uint8Array(COLS * ROWS), lava, props: [], lights: [], exits: [], rests: [] };
}

function buildMob(over) {
    const dr = Object.assign({ move: null, t: 0, tellDur: 1, alt: 0, shadow: null, breath: null, tail: null, patches: [], marks: [], vents: null, ring: null, drowsy: 0, pillars: 0, phase: 1 }, (over && over.dr) || {});
    return Object.assign({ id: 1, kind: 'brandur', x: 480, y: 360, vx: 0, vy: 0, hp: 40, maxHp: 48, state: 'move', t: 0, face: 1, hurtT: 0, deadT: 0, r: 56, ax: 1, ay: 0, phase: 1, dr }, over || {}, { dr });
}

function buildSim(mob, lair, extra) {
    const room = buildRoom();
    const it = {
        placeId: 'deeps', roomId: 'dp_lair', room, mobs: mob ? [mob] : [], things: [], chests: [], pickups: [], npcs: [], shots: [], floats: [],
        hero: { x: 300, y: 420, dir: 'right', moving: false, hurtT: 0, attackT: 0, dashT: 0, invuln: 0, walkT: 0 },
        prompt: null, wipe: null, guardian: mob || null, lair: lair || null, t: 4, cleared: false
    };
    if (extra) Object.assign(it, extra);
    return { heroId: 'knight', mode: 'campaign', profile: { npcFlags: {}, loot: [], seen: [] }, interior: it };
}

function renderOnce(sim, opts) {
    const o = opts || {};
    const { ctx, log } = makeCtx();
    const view = sandbox.RunenpfadInteriorView.create({ ctx, art: o.atlas || null, t: (k) => k, font: '', sheets: {}, drawStaticAsset() {} });
    for (let i = 0; i < 2; i++) view.render(sim, { viewW: 900, viewH: 700, nowSec: 1.3 + i * 0.05, dt: 0.016, reducedMotion: !!o.reduced });
    view.dispose();
    return log;
}

const ATLAS = fakeAtlas();
assert.ok(ATLAS.clips['td-boss-brandur.breath_tell'] && ATLAS.clips['td-fx-breath.loop'] && ATLAS.clips['td-brandur-air.dive'], 'fake atlas has the dragon clips');

// ---- states (contract list) ----

const marks = [
    { x: 400, y: 300, r: 26, t: 0.4, at: 1.1, kind: 'stalactite', done: false },
    { x: 520, y: 420, r: 26, t: 0.9, at: 1.1, kind: 'stalactite', done: false },
    { x: 560, y: 300, r: 24, t: 0.6, at: 1.0, kind: 'rain', done: false },
    { x: 360, y: 440, r: 24, t: 0.8, at: 1.0, kind: 'rain', done: false },
    { x: 440, y: 440, r: 26, t: 1.2, at: 1.1, kind: 'stalactite', done: true }
];
const breathLive = { x: 540, y: 353, a: 0.2, a0: -0.17, a1: 0.57, len: 260, half: 0.436, t: 0.5, dur: 1.2, live: true };
const STATES = {
    sleep: () => buildMob({ state: 'sleep' }),
    move: () => buildMob({ state: 'move' }),
    hurt: () => buildMob({ state: 'move', hurtT: 0.4 }),
    breath_tell: () => buildMob({ state: 'tell', dr: { move: 'breath', t: 0.5, tellDur: 1, breath: { x: 540, y: 353, a: 0.2, a0: -0.17, a1: 0.57, len: 260, half: 0.436, t: 0, dur: 1.2, live: false } } }),
    breath_tell_noobj: () => buildMob({ state: 'tell', dr: { move: 'breath', t: 0.5, tellDur: 1 } }),
    breath_attack: () => buildMob({ state: 'attack', dr: { move: 'breath', t: 0.5, breath: breathLive, patches: [{ x: 640, y: 360, r: 30, left: 3, life: 4 }] } }),
    tail_tell: () => buildMob({ state: 'tell', dr: { move: 'tail', t: 0.4, tellDur: 0.8, tail: { x: 480, y: 353, a: Math.PI, r: 150, half: 1.22 } } }),
    tail_attack: () => buildMob({ state: 'attack', dr: { move: 'tail', t: 0.1, tail: { x: 480, y: 353, a: Math.PI, r: 150, half: 1.22 } } }),
    stomp_tell: () => buildMob({ state: 'tell', dr: { move: 'stomp', t: 0.5, tellDur: 0.9 } }),
    stomp_marks: () => buildMob({ state: 'attack', dr: { move: 'stomp', t: 0.1, marks: marks.map((m) => Object.assign({}, m)) } }),
    takeoff_tell: () => buildMob({ state: 'tell', dr: { move: 'takeoff', t: 0.4, tellDur: 0.8, alt: 0.3 } }),
    air_rain: () => buildMob({ state: 'air', dr: { move: 'takeoff', t: 1, alt: 1, shadow: { x: 400, y: 420 }, marks: marks.map((m) => Object.assign({}, m)) } }),
    air_dive: () => buildMob({ state: 'air', dr: { move: 'takeoff', t: 2.6, alt: 0.4, shadow: { x: 400, y: 420 } } }),
    land: () => buildMob({ state: 'move', dr: { ring: { x: 480, y: 360, r: 40, max: 150, t: 0.2 } } }),
    ring: () => buildMob({ state: 'move', dr: { ring: { x: 480, y: 360, r: 90, max: 150, t: 0.35 } } }),
    buffet_tell: () => buildMob({ state: 'tell', dr: { move: 'buffet', t: 0.4, tellDur: 0.8 } }),
    roar_tell: () => buildMob({ state: 'tell', dr: { move: 'roar', t: 0.4, tellDur: 0.9 } }),
    vents_tell: () => buildMob({ state: 'tell', dr: { move: 'vents', t: 0.5, tellDur: 1, vents: { rows: [5, 8], t: 0.5, at: 1, dur: 0.6 } } }),
    vents_live: () => buildMob({ state: 'attack', dr: { move: 'vents', t: 0.2, vents: { rows: [5, 8], t: 1.2, at: 1, dur: 0.6 } } }),
    drowsy: () => buildMob({ state: 'drowsy', dr: { drowsy: 3.2, pillars: 4 } }),
    dead: () => buildMob({ state: 'dead', hp: 0, deadT: 0.2 }),
    phase3: () => buildMob({ state: 'move', hp: 10, phase: 3, dr: { phase: 3, pillars: 2 } })
};
const LAIRS = {
    beaten: () => ({ fate: 'beaten', x: 480, y: 360, face: 1, t: 2, asked: true }),
    slain: () => ({ fate: 'slain', x: 480, y: 360, face: -1, t: 2, asked: true }),
    sealed: () => ({ fate: 'sealed', x: 480, y: 360, face: 1, t: 2, asked: true })
};

for (const [name, mk] of Object.entries(STATES)) {
    for (const reduced of [false, true]) {
        for (const art of [null, ATLAS]) {
            check(`render ${name} reduced=${reduced} art=${!!art}`, () => {
                const sim = buildSim(mk());
                const before = JSON.stringify(sim.interior);
                const log = renderOnce(sim, { atlas: art, reduced });
                assert.ok(log.calls > 0, 'something was drawn');
                assert.equal(JSON.stringify(sim.interior), before, 'render is read-only');
            });
        }
    }
}
for (const [name, mk] of Object.entries(LAIRS)) {
    for (const reduced of [false, true]) {
        for (const art of [null, ATLAS]) {
            check(`render sleeper ${name} reduced=${reduced} art=${!!art}`, () => {
                const sim = buildSim(null, mk());
                const before = JSON.stringify(sim.interior);
                const log = renderOnce(sim, { atlas: art, reduced });
                assert.ok(log.calls > 0);
                assert.equal(JSON.stringify(sim.interior), before);
            });
        }
    }
}
check('render lava room without brandur', () => {
    const sim = buildSim(null, null, { guardian: null });
    sim.interior.room.def.guardian = null;
    const before = JSON.stringify(sim.interior);
    renderOnce(sim, { atlas: ATLAS });
    renderOnce(sim, { atlas: null, reduced: true });
    assert.equal(JSON.stringify(sim.interior), before);
});
check('render with a bad sim does not throw', () => {
    renderOnce({ interior: null }, {});
    const sim = buildSim(buildMob({ state: 'tell', dr: { move: 'breath', t: NaN, breath: { x: NaN, y: undefined, a: 'x' } } }));
    renderOnce(sim, { atlas: ATLAS });
    const sim2 = buildSim(buildMob({ state: 'attack' }));
    delete sim2.interior.mobs[0].dr;
    renderOnce(sim2, { atlas: ATLAS });
});

// ---- clip choice (fake atlas records which sheet was drawn) ----

function sheetsDrawn(name, reduced) {
    return new Set(renderOnce(buildSim(STATES[name]()), { atlas: ATLAS, reduced }).images);
}
function sleeperSheets(name) {
    return new Set(renderOnce(buildSim(null, LAIRS[name]()), { atlas: ATLAS }).images);
}
check('clips follow the state', () => {
    assert.ok(sheetsDrawn('sleep').has('td-brandur-b'));
    assert.ok(sheetsDrawn('move').has('td-boss-brandur'));
    assert.ok(sheetsDrawn('breath_tell').has('td-boss-brandur'));
    assert.ok(sheetsDrawn('tail_tell').has('td-brandur-b'));
    assert.ok(sheetsDrawn('stomp_tell').has('td-brandur-b'));
    assert.ok(sheetsDrawn('buffet_tell').has('td-brandur-b'));
    assert.ok(sheetsDrawn('roar_tell').has('td-brandur-b'));
    assert.ok(sheetsDrawn('drowsy').has('td-brandur-b'));
    assert.ok(sheetsDrawn('dead').has('td-boss-brandur'));
    assert.ok(sheetsDrawn('takeoff_tell').has('td-brandur-air'));
    assert.ok(sheetsDrawn('air_rain').has('td-brandur-air'));
    assert.ok(sheetsDrawn('air_dive').has('td-brandur-air'));
    assert.ok(sheetsDrawn('land').has('td-brandur-air'), 'landing pose');
    assert.ok(!sheetsDrawn('air_rain').has('td-boss-brandur'), 'no ground body while airborne');
});
check('breath jet only while live, in both motion modes', () => {
    for (const reduced of [false, true]) {
        assert.ok(sheetsDrawn('breath_attack', reduced).has('td-fx-breath'));
        assert.ok(!sheetsDrawn('breath_tell', reduced).has('td-fx-breath'));
    }
});
check('hazard sprites: marks, patches, vents', () => {
    assert.ok(sheetsDrawn('stomp_marks').has('td-fx-hazards'));
    assert.ok(sheetsDrawn('breath_attack').has('td-fx-hazards'), 'fire patch');
    assert.ok(sheetsDrawn('vents_live').has('td-fx-hazards'));
});
check('sleepers: beaten and sealed sleep, slain lies defeated', () => {
    assert.ok(sleeperSheets('beaten').has('td-brandur-b'));
    assert.ok(sleeperSheets('sealed').has('td-brandur-b'));
    assert.ok(sleeperSheets('slain').has('td-boss-brandur'));
    assert.ok(!sleeperSheets('slain').has('td-brandur-b'));
});
check('a dying mob and the fresh sleeper are not drawn twice', () => {
    const sim = buildSim(buildMob({ state: 'dead', hp: 0, deadT: 0.1 }), LAIRS.beaten());
    const log = renderOnce(sim, { atlas: ATLAS });
    assert.ok(log.images.includes('td-boss-brandur'));
    assert.ok(!log.images.includes('td-brandur-b'), 'the sleeper waits until the body is gone');
});

// ---- warnings stay readable under reduced motion ----

check('telegraphs are filled and outlined, also with reduced motion', () => {
    for (const reduced of [false, true]) {
        for (const name of ['breath_tell', 'breath_tell_noobj', 'tail_tell', 'stomp_tell', 'stomp_marks', 'vents_tell', 'takeoff_tell']) {
            const log = renderOnce(buildSim(STATES[name]()), { atlas: null, reduced });
            assert.ok(log.fills.some((c) => /^rgba\(255, 84, 64, 0\.[2-9]/.test(c)), `${name} warning fill (reduced=${reduced})`);
            assert.ok(log.strokes.some((c) => /^rgba\(255, 170, 140, 0\.[7-9]/.test(c) || c === 'rgba(255, 170, 140, 1.000)'), `${name} warning outline (reduced=${reduced})`);
        }
    }
});

// ---- the view object itself ----

check('view API, sheets, lights and sorted entries', () => {
    const { ctx } = makeCtx();
    const dv = sandbox.RunenpfadBrandurView.create({ ctx, art: ATLAS, t: (k) => k, font: '' });
    for (const k of ['owns', 'sheets', 'drawFloor', 'sorted', 'drawObject', 'drawTop', 'lights', 'drawBossBar', 'dispose']) assert.equal(typeof dv[k], 'function', k);
    const sim = buildSim(STATES.breath_attack());
    const iv = sim.interior;
    const ids = dv.sheets(iv);
    for (const id of ['td-boss-brandur', 'td-brandur-b', 'td-brandur-air', 'td-fx-breath', 'td-fx-hazards', 'tdp-lair', 'tex-glut']) assert.ok(ids.includes(id), id);
    const plain = buildSim(null, null, { guardian: null });
    plain.interior.room.def.guardian = null;
    const plainIds = dv.sheets(plain.interior);
    assert.ok(plainIds.includes('tex-glut') && !plainIds.includes('td-boss-brandur'), 'a lava room alone only asks for the glut texture');
    const ls = dv.lights(iv);
    assert.ok(ls.some((l) => l.r === 110 && l.color === '#ff7a2a' && l.flicker), 'lava lights');
    assert.ok(ls.some((l) => l.r === 260), 'live breath light');
    assert.ok(ls.some((l) => l.r === 70), 'patch light');
    assert.ok(dv.sorted(iv).length === 1 && dv.sorted(plain.interior).length === 0);
    assert.ok(dv.sorted(buildSim(STATES.air_rain()).interior).length === 0, 'the airborne body is drawn on top, not sorted');
    assert.equal(dv.owns({ kind: 'brandur' }), true);
    assert.equal(dv.owns({ kind: 'bat' }), false);
    dv.dispose();
});

check('drawBossBar is true only while brandur lives', () => {
    const { ctx } = makeCtx();
    const dv = sandbox.RunenpfadBrandurView.create({ ctx, art: null, t: (k) => k, font: '' });
    const f = { s: 1, t: 1, reduced: false, oy: 62 };
    assert.equal(dv.drawBossBar(buildSim(STATES.move()).interior, f, 900, 62), true);
    assert.equal(dv.drawBossBar(buildSim(STATES.drowsy()).interior, f, 900, 62), true);
    assert.equal(dv.drawBossBar(buildSim(STATES.dead()).interior, f, 900, 62), false);
    assert.equal(dv.drawBossBar(buildSim(null, LAIRS.beaten()).interior, f, 900, 62), false);
    const bat = buildSim(buildMob({ kind: 'bat' }));
    assert.equal(dv.drawBossBar(bat.interior, f, 900, 62), false);
    assert.equal(dv.drawBossBar({}, f, 900, 62), false);
    // the interior view draws the bar through the hook while he lives, never after
    assert.ok(renderOnce(buildSim(STATES.move()), {}).calls > 0);
});

// ---- hooks of interior-view.js and interior-things-view.js ----

check('chest look draws the prop clip; the scale vanishes once taken', () => {
    const chest = (look, opened) => ({ x: 300, y: 300, look, opened, locked: false, key: null });
    const sim = buildSim(null, null, { guardian: null, chests: [chest('tdp-lair.p15', false)] });
    sim.interior.room.def.guardian = null;
    assert.ok(renderOnce(sim, { atlas: ATLAS }).images.includes('tdp-lair'));
    const taken = buildSim(null, null, { guardian: null, chests: [chest('tdp-lair.p15', true)] });
    taken.interior.room.def.guardian = null;
    assert.ok(!renderOnce(taken, { atlas: ATLAS }).images.includes('tdp-lair'), 'the opened scale chest draws no prop');
    const trunk = buildSim(null, null, { guardian: null, chests: [chest('tdp-house.p8', true)] });
    trunk.interior.room.def.guardian = null;
    assert.ok(renderOnce(trunk, { atlas: ATLAS }).images.includes('tdp-house'), 'an opened trunk stays, dimmed');
    renderOnce(buildSim(null, null, { guardian: null, chests: [chest('tdp-lair.p15', false)] }), { atlas: null });
});

check('things of the new kinds: pillar, shelf, tablet, stand, uses and prompts', () => {
    const things = [
        { kind: 'brazier', id: 'pillar_1', bid: 'pillar_1', look: 'pillar', lit: true, left: 6, timed: 8, cx: 4, cy: 4 },
        { kind: 'brazier', id: 'pillar_2', bid: 'pillar_2', look: 'pillar', lit: false, cx: 8, cy: 4 },
        { kind: 'brazier', id: 'shelf_e', bid: 'shelf_e', look: 'shelf', lit: true, cx: 12, cy: 4 },
        { kind: 'sign', id: 'tab_3', look: 'tablet', tablet: 3, cx: 4, cy: 8 },
        { kind: 'sign', id: 'tab_1', look: 'stand', tablet: 1, cx: 8, cy: 8 },
        { kind: 'use', id: 'u1', use: 'cook', cx: 4, cy: 10 }, { kind: 'use', id: 'u2', use: 'cook', cx: 5, cy: 10 },
        { kind: 'use', id: 'u3', use: 'forge', cx: 8, cy: 10 }, { kind: 'use', id: 'u4', use: 'forge', cx: 9, cy: 10 },
        { kind: 'use', id: 'u5', use: 'bell', cx: 12, cy: 10 }, { kind: 'use', id: 'u6', use: 'bell', cx: 13, cy: 10 },
        { kind: 'use', id: 'u7', use: 'bell', cx: 12, cy: 11 }, { kind: 'use', id: 'u8', use: 'bell', cx: 13, cy: 11 },
        { kind: 'use', id: 'u9', use: 'map', cx: 16, cy: 10 }, { kind: 'use', id: 'u10', use: 'map', cx: 17, cy: 10 },
        { kind: 'use', id: 'u11', use: 'brandur', cx: 16, cy: 4 }
    ];
    for (const prompt of ['cook', 'forge', 'ring', 'talk', 'read', 'weird']) {
        const sim = buildSim(null, null, { guardian: null, things, prompt: { kind: prompt, x: 300, y: 300 } });
        sim.interior.room.def.guardian = null;
        const before = JSON.stringify(sim.interior);
        const log = renderOnce(sim, { atlas: ATLAS });
        assert.equal(JSON.stringify(sim.interior), before);
        if (prompt === 'cook') {
            for (const id of ['tdp-lair', 'tdp-home', 'tdp-house']) assert.ok(log.images.includes(id), id);
        }
        renderOnce(sim, { atlas: null, reduced: true });
    }
});

check('the room view scale follows def.view', () => {
    // vh 700: 15 cells -> 700 / 720 clamps to 1; 11 cells -> 700 / 528 = 1.33
    const roomScale = (view) => {
        const sim = buildSim(buildMob());
        sim.interior.room.def.view = view;
        return renderOnce(sim, { atlas: ATLAS }).scales.map((s) => s[0]).find((v) => v > 0.99 && v < 2.6 && v !== 1 ? true : v === 1);
    };
    const wide = roomScale(15);
    const tight = roomScale(0);
    assert.equal(wide, 1);
    assert.ok(Math.abs(tight - 700 / (11 * 48)) < 1e-9, 'default view is 11 cells, got ' + tight);
});

console.log(`runenpfad deeps view suite: ${passed} checks passed`);
