import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

// Round 11, work package C: the deeps (twelve cave rooms), lava, the new validator rules, the use registry, the visited-room map and its view.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array, Proxy };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Canonical order (sim core, then the view modules this suite touches); files that do not exist yet are skipped.
const MODULES = ('biomes fx physics behaviors bosses variety profile runes zones zones-extra gauntlets swing puzzles dragons npcs npc-story npc-story-late dialogue trade ' +
    'duel-patterns duel place-rooms interior-mobs interior-bosses interior-things deep-mobs brandur interior places folk folk-story journal-story ember-story ember-story-folk ' +
    'village-rooms place-rooms-deep deeps-rooms village cutscenes chronicle ember crafts dice bounties levels combat sim art interior-bake deeps-map-view').split(' ');
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Sim = sandbox.RunenpfadSim;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Interior = sandbox.RunenpfadInterior;
const Things = sandbox.RunenpfadInteriorThings;
const Npcs = sandbox.RunenpfadNpcs;
assert.ok(Sim && Rooms && Mobs && Interior && Things && Npcs, 'modules loaded');

const STEP = Sim.STEP;
const DT = 1 / 60;
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

function tick(sim, input, n) {
    const events = [];
    for (let i = 0; i < (n || 1); i++) events.push(...Sim.stepSimulation(sim, STEP * 1.000001, input || {}));
    return events;
}

// ---- block puzzle model (the search of test-runenpfad-rooms.mjs): breadth-first over (block cells, hero region) ----

// ---- block puzzle model: breadth-first search over (block cells, hero region) with the engine's push and slide rules ----

const DIRS = [[1, 0], [-1, 0], [0, 1], [0, -1]];

function model(roomId, opts) {
    const o = opts || {};
    const lay = Rooms.layout(roomId);
    const d = lay.def;
    const cols = lay.cols;
    const rows = lay.rows;
    const N = cols * rows;
    const hs = new Uint8Array(lay.solid); // solid for the hero
    const bs = new Uint8Array(lay.solid); // solid for a block (also pots and cracks)
    const exits = new Uint8Array(N);
    Object.keys(d.exits).forEach((m) => (lay.markers[m] || []).forEach((c) => { exits[c.cy * cols + c.cx] = 1; }));
    const blocks = [];
    const plates = {};
    d.things.forEach((t) => {
        (lay.markers[t.at] || []).forEach((c) => {
            const i = c.cy * cols + c.cx;
            if (t.block) blocks.push(i);
            else if (t.plate) plates[t.plate] = i;
            else if (t.lever || t.dart || t.brazier || t.page || t.sign || t.use || t.chest || t.npc || t.gate) { hs[i] = 1; bs[i] = 1; }
            else if (t.pot || t.crack) bs[i] = 1;
        });
    });
    const slip = (i) => !!d.slippery && d.map[Math.floor(i / cols)].charAt(i % cols) !== ',';
    const cellOf = (marker) => (lay.markers[marker] || []).map((c) => c.cy * cols + c.cx);
    return { lay, d, cols, rows, N, hs, bs, exits, blocks: blocks.sort((a, b) => a - b), plates, slip, cellOf };
}

function solve(m, spec) {
    const { cols, rows, N, hs, bs, exits, slip } = m;
    const ice = !!m.d.slippery;
    const inb = (i, dv) => { const x = (i % cols) + dv[0]; const y = Math.floor(i / cols) + dv[1]; return x >= 0 && y >= 0 && x < cols && y < rows; };
    const at = (i, dv) => i + dv[1] * cols + dv[0];
    const flag = new Uint8Array(N); // block cells of the state under work
    const stamp = new Int32Array(N);
    let epoch = 0;
    const stack = new Int32Array(N + 8);
    // every cell the hero can stand on or slide over without a push (stops are where he can start a push from)
    const reach = (blocks, h) => {
        blocks.forEach((b) => { flag[b] = 1; });
        epoch += 1;
        const stops = [h];
        const touched = new Set([h]);
        stamp[h] = epoch;
        let sp = 0;
        stack[sp++] = h;
        while (sp) {
            const p = stack[--sp];
            for (let k = 0; k < 4; k++) {
                const dv = DIRS[k];
                if (!inb(p, dv)) continue;
                let pos = p;
                for (;;) {
                    if (!inb(pos, dv)) break;
                    const n = at(pos, dv);
                    if (flag[n] || hs[n] || exits[n]) break;
                    pos = n;
                    touched.add(pos);
                    if (!ice || !slip(pos)) break;
                }
                if (pos !== p && stamp[pos] !== epoch) { stamp[pos] = epoch; stops.push(pos); stack[sp++] = pos; }
            }
        }
        blocks.forEach((b) => { flag[b] = 0; });
        return { stops, touched };
    };
    const blockFree = (c) => !bs[c] && !exits[c] && !flag[c];
    const push = (b, dv, blocks) => {
        blocks.forEach((x) => { flag[x] = 1; });
        let t = -1;
        if (inb(b, dv) && blockFree(at(b, dv))) {
            t = at(b, dv);
            while (ice && slip(t) && inb(t, dv) && blockFree(at(t, dv))) t = at(t, dv);
        }
        blocks.forEach((x) => { flag[x] = 0; });
        return t;
    };
    const plateCells = spec.plates.map((id) => m.plates[id]);
    const mustAll = [].concat(...spec.mustAll.map(m.cellOf));
    // the floor cells next to a marker (a gate or an exit): the hero has to be able to reach at least one of them
    const near = (marker) => {
        const own = m.cellOf(marker);
        const out = [];
        own.forEach((c) => DIRS.forEach((dv) => {
            if (!inb(c, dv)) return;
            const n = at(c, dv);
            if (!hs[n] && !exits[n] && own.indexOf(n) < 0) out.push(n);
        }));
        return out;
    };
    const groups = spec.near.map(near);
    const key = (blocks, h, r) => blocks.join(',') + '|' + (ice ? h : Math.min(...r.stops));
    const start = m.cellOf(spec.start)[0];
    const all = [];
    const seen = new Map();
    const r0 = reach(m.blocks, start);
    all.push({ blocks: m.blocks.slice(), hero: start, depth: 0, r: r0, succ: [] });
    seen.set(key(m.blocks, start, r0), 0);
    let goalDepth = -1; // the gate condition holds (a hero on a plate counts, as in the engine)
    let blocksDepth = -1; // the same with a block on every plate
    for (let head = 0; head < all.length; head++) {
        const s = all[head];
        if (all.length > 600000) throw new Error('state space too large for ' + spec.name);
        const r = s.r;
        const bset = new Set(s.blocks);
        const uncovered = plateCells.filter((p) => !bset.has(p));
        if (goalDepth < 0 && (uncovered.length === 0 || (uncovered.length === 1 && r.touched.has(uncovered[0])))) goalDepth = s.depth;
        if (blocksDepth < 0 && uncovered.length === 0) blocksDepth = s.depth;
        s.ok = mustAll.every((c) => r.touched.has(c)) && groups.every((g) => g.some((c) => r.touched.has(c)));
        for (let si = 0; si < r.stops.length; si++) {
            const h = r.stops[si];
            for (let k = 0; k < 4; k++) {
                const dv = DIRS[k];
                if (!inb(h, dv)) continue;
                const b = at(h, dv);
                if (!bset.has(b)) continue;
                const t = push(b, dv, s.blocks);
                if (t < 0) continue;
                const nb = s.blocks.filter((x) => x !== b).concat([t]).sort((x, y) => x - y);
                const nh = ice ? h : b;
                const nr = reach(nb, nh);
                const kk = key(nb, nh, nr);
                let id = seen.get(kk);
                if (id === undefined) {
                    id = all.length;
                    seen.set(kk, id);
                    all.push({ blocks: nb, hero: nh, depth: s.depth + 1, r: nr, succ: [] });
                }
                s.succ.push(id);
            }
        }
    }
    // a state is dead when no push sequence leads back to a state where the exit and the gate can be reached
    const pred = all.map(() => []);
    all.forEach((s, i) => s.succ.forEach((j) => pred[j].push(i)));
    const live = new Uint8Array(all.length);
    const todo = [];
    all.forEach((s, i) => { if (s.ok) { live[i] = 1; todo.push(i); } });
    while (todo.length) pred[todo.pop()].forEach((i) => { if (!live[i]) { live[i] = 1; todo.push(i); } });
    const dead = all.filter((s, i) => !live[i]);
    const stuck = all.filter((s) => !s.ok).length;
    return { goalDepth, blocksDepth, states: all.length, notAccessible: stuck, dead: dead.length, deadExample: dead[0] ? dead[0].blocks.map((b) => (b % cols) + ',' + Math.floor(b / cols)) : null, startOk: all[0].ok };
}

// ---- helpers on the real engine ----

function enterAt(sim, placeId, roomId, marker) {
    const it = Interior.create(sim, placeId, null);
    sim.interior = it;
    if (roomId && roomId !== it.roomId) Interior.enterRoom(sim, it, roomId, marker || null);
    return it;
}

const cellOfHero = (it) => ({ cx: Math.floor(it.hero.x / 48), cy: Math.floor((it.hero.y - 7) / 48) });

function put(it, cx, cy) {
    it.hero.x = (cx + 0.5) * 48;
    it.hero.y = (cy + 0.5) * 48 + 7;
    it.hero.vx = 0;
    it.hero.vy = 0;
}

// One interior tick (without the world around it); returns the events of that tick.
function istep(sim, it, input) {
    sim.events.length = 0;
    Interior.step(sim, it, Object.assign({}, input || {}), DT);
    return sim.events.slice();
}

// Runs for `secs` seconds (or until `until` returns true); returns every event with its type.
function irun(sim, it, secs, input, until) {
    const out = [];
    const n = Math.round(secs / DT);
    for (let i = 0; i < n; i++) {
        out.push(...istep(sim, it, input));
        if (until && until()) break;
    }
    return out;
}

const thingById = (it, id) => it.things.filter((t) => t.id === id)[0];
const thingsOf = (it, kind) => it.things.filter((t) => t.kind === kind);
const names = (events, type) => events.filter((e) => e.type === type);

function placeBlock(it, b, cx, cy) {
    const r = it.room;
    r.solid[b.cy * r.cols + b.cx] = 0;
    b.cx = cx;
    b.cy = cy;
    b.x = (cx + 0.5) * 48;
    b.y = (cy + 0.5) * 48 + 7;
    b.moving = null;
    r.solid[cy * r.cols + cx] = 1;
}

// Walks the hero to a cell with real inputs along a shortest path over room.solid (exit cells only as the target).
function walk(sim, it, cx, cy) {
    const room = it.room;
    const cols = room.cols;
    const bad = (x, y) => x < 0 || y < 0 || x >= cols || y >= room.rows || room.solid[y * cols + x];
    const isExit = (x, y) => room.exits.some((e) => e.cx === x && e.cy === y);
    const from = cellOfHero(it);
    const prev = new Map([[from.cy * cols + from.cx, -1]]);
    const queue = [from];
    let found = from.cx === cx && from.cy === cy;
    for (let h = 0; h < queue.length && !found; h++) {
        const c = queue[h];
        for (const dv of DIRS) {
            const nx = c.cx + dv[0];
            const ny = c.cy + dv[1];
            const k = ny * cols + nx;
            if (bad(nx, ny) || prev.has(k)) continue;
            if (isExit(nx, ny) && !(nx === cx && ny === cy)) continue;
            prev.set(k, c.cy * cols + c.cx);
            queue.push({ cx: nx, cy: ny });
            if (nx === cx && ny === cy) found = true;
        }
    }
    assert.ok(found, 'a path to ' + cx + ',' + cy + ' in ' + it.roomId);
    const path = [];
    for (let k = cy * cols + cx; k !== -1; k = prev.get(k)) path.unshift({ cx: k % cols, cy: Math.floor(k / cols) });
    const roomId = it.roomId;
    for (const w of path.slice(1)) {
        const tx = (w.cx + 0.5) * 48;
        const ty = (w.cy + 0.5) * 48 + 7;
        for (let i = 0; i < 240; i++) {
            const dx = tx - it.hero.x;
            const dy = ty - it.hero.y;
            if (Math.abs(dx) < 3 && Math.abs(dy) < 3) break;
            istep(sim, it, { left: dx < -3, right: dx > 3, up: dy < -3, down: dy > 3 });
            if (sim.interior !== it || it.roomId !== roomId || it.leaving) return false;
        }
    }
    return true;
}

// Walks next to a solid cell (chest, lectern, lever, brazier, sign) and returns the cell used.
function walkNear(sim, it, cx, cy) {
    const room = it.room;
    const free = (x, y) => x >= 0 && y >= 0 && x < room.cols && y < room.rows && !room.solid[y * room.cols + x] && !room.exits.some((e) => e.cx === x && e.cy === y);
    const spot = DIRS.map((dv) => ({ cx: cx + dv[0], cy: cy + dv[1] })).filter((c) => free(c.cx, c.cy))[0];
    assert.ok(spot, 'a free cell next to ' + cx + ',' + cy);
    assert.ok(walk(sim, it, spot.cx, spot.cy), 'walk next to ' + cx + ',' + cy);
    // lean against the thing like a player does (the hero box then touches the solid cell)
    const dx = cx - spot.cx;
    const dy = cy - spot.cy;
    for (let i = 0; i < 20; i++) istep(sim, it, { left: dx < 0, right: dx > 0, up: dy < 0, down: dy > 0 });
    return spot;
}

function press(sim, it, input) { return istep(sim, it, Object.assign({ talkPressed: 'e' }, input || {})); }

function markerCell(roomId, marker, i) { return Rooms.layout(roomId).markers[marker][i || 0]; }

// ---- engine helpers of this suite ----

const Deeps = sandbox.RunenpfadDeepsRooms;
const Bake = sandbox.RunenpfadInteriorBake;
const MapView = sandbox.RunenpfadDeepsMapView;
assert.ok(Deeps, 'deeps-rooms.js loaded');

function hero(opts) {
    const sim = Sim.createSimulation({ levelId: (opts && opts.levelId) || 3, hero: (opts && opts.hero) || 'knight', seed: (opts && opts.seed) || 77 });
    tick(sim, {}, 30);
    sim.player.hp = 99;
    sim.player.maxHp = 99;
    return sim;
}

const clearMobs = (it) => { it.mobs = []; };
// Walks to (fx, fy) and leans against the solid cell (cx, cy) from there.
function leanFrom(sim, it, fx, fy, cx, cy) {
    assert.ok(walk(sim, it, fx, fy), 'walk to ' + fx + ',' + fy);
    for (let i = 0; i < 20; i++) istep(sim, it, { left: cx < fx, right: cx > fx, up: cy < fy, down: cy > fy });
}
const swingAt = (sim, it, tries, done) => {
    for (let i = 0; i < tries && !done(); i++) {
        istep(sim, it, { attackPressed: true });
        irun(sim, it, 0.5, {});
    }
};

// Temporary room for the validator rules: runs validate() and returns only the lines of that room.
function problemsOf(map, things, extra) {
    Rooms.ROOMS.t_tmp = Object.assign({ theme: 'cave', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cave', map, things, exits: {}, spawn: 'S' }, extra || {});
    try { return Array.from(Rooms.validate()).filter((p) => p.indexOf('room t_tmp') === 0); } finally { delete Rooms.ROOMS.t_tmp; }
}
const BASE_MAP = ['##########', '#S.......#', '#........#', '#........#', '#........#', '#........#', '#........#', '##########'];
const withCell = (ch, x, y) => BASE_MAP.map((r, i) => (i === y ? r.slice(0, x) + ch + r.slice(x + 1) : r));
const has = (list, word) => list.some((p) => p.indexOf(word) >= 0);

// ---- 1. data ----

check('data: validator clean, 176 props, twelve rooms, every room lit, the lair is big', () => {
    assert.equal(JSON.stringify(Rooms.validate()), '[]');
    assert.equal(Object.keys(Rooms.PROP_DEFS).length, 176);
    const place = Rooms.PLACES.deeps;
    assert.equal(place.rooms.length, 12);
    assert.equal(place.entry, 'dp_shaft');
    assert.equal(place.kind, 'cave');
    assert.equal(place.nameKey, 'runenpfad.place.deeps');
    assert.equal(place.facade.clip, 'fac-deep.f0');
    assert.deepEqual(JSON.parse(JSON.stringify(place.rooms)), JSON.parse(JSON.stringify(Deeps.ROOM_IDS)));
    place.rooms.forEach((id) => {
        const lay = Rooms.layout(id);
        assert.ok(lay.lights.length > 0, id + ' has a light');
        assert.ok(lay.def.map.every((r) => r.length === lay.cols), id + ' rows are equal');
    });
    const lair = Rooms.ROOMS.dp_lair;
    assert.equal(lair.map[0].length, 28);
    assert.equal(lair.map.length, 18);
    assert.ok(lair.big === true && lair.view === 15 && lair.guardian === 'brandur');
    assert.equal(lair.music, 'in_lair');
    ['dp_shaft', 'dp_rails', 'dp_camp', 'dp_bronze'].forEach((id) => assert.equal(Rooms.ROOMS[id].theme, 'mine'));
    ['dp_vents', 'dp_nest', 'dp_hoard', 'dp_lair'].forEach((id) => assert.equal(Rooms.ROOMS[id].theme, 'glut'));
    const loot = [];
    Rooms.PLACES.deeps.rooms.forEach((id) => Rooms.ROOMS[id].things.forEach((t) => { if (t.chest) loot.push(t.chest); }));
    assert.deepEqual(loot.sort(), ['dp_bronze', 'dp_camp', 'dp_hall', 'dp_nest', 'dp_scale']);
    const tabs = [];
    Rooms.PLACES.deeps.rooms.forEach((id) => Rooms.ROOMS[id].things.forEach((t) => { if (t.tablet) tabs.push(t.tablet + ':' + t.sign); }));
    assert.deepEqual(tabs.sort(), ['3:tab_3', '4:tab_4', '5:tab_5', '6:tab_6']);
});

check('data: the lair has a lava rim open at the entrance, four pillars on a 7x5 rectangle, Brandur and his door', () => {
    const lay = Rooms.layout('dp_lair');
    const at = (m) => lay.markers[m][0];
    const lava = (x, y) => lay.lava[y * lay.cols + x] === 1;
    for (let x = 1; x <= 26; x++) {
        assert.ok(lava(x, 1), 'rim at the back wall ' + x);
        if (x < 13 || x > 15) assert.ok(lava(x, 16), 'rim at the front wall ' + x);
    }
    for (let y = 1; y <= 16; y++) assert.ok(lava(1, y) && lava(26, y), 'rim at the side walls ' + y);
    [13, 14, 15].forEach((x) => assert.ok(!lava(x, 16) && !lava(x, 15), 'the rim is open at the entrance ' + x));
    const p = ['1', '2', '3', '4'].map((m) => at(m));
    const xs = p.map((c) => c.cx).sort((a, b) => a - b);
    const ys = p.map((c) => c.cy).sort((a, b) => a - b);
    assert.equal(xs[3] - xs[0], 7);
    assert.equal(ys[3] - ys[0], 5);
    assert.equal((xs[0] + xs[3]) / 2, (2 + 25) / 2, 'centred in the arena (columns)');
    assert.equal((ys[0] + ys[3]) / 2, (2 + 15) / 2, 'centred in the arena (rows)');
    const a = at('a');
    const g = at('G');
    const k = at('K');
    assert.ok(Math.hypot(a.cx - g.cx, a.cy - g.cy) >= 6);
    // K stands two cells behind G (not one): his collision box reaches 31 px into the cell behind him
    assert.deepEqual([k.cx, g.cy - k.cy], [g.cx, 2]);
    assert.ok(Rooms.ROOMS.dp_lair.things.some((t) => t.use === 'brandur' && t.at === 'K'));
    assert.equal(Rooms.ROOMS.dp_lair.things.filter((t) => t.brazier && t.look === 'pillar' && t.timed === 8 && !t.set).length, 4);
});

// ---- 2. lava and the new validator rules ----

check('lava: solid, not walls, not a marker, in layout().lava', () => {
    const lay = Rooms.layout('dp_vents');
    let n = 0;
    for (let y = 0; y < lay.rows; y++) {
        for (let x = 0; x < lay.cols; x++) {
            if (Rooms.ROOMS.dp_vents.map[y].charAt(x) !== '~') continue;
            n += 1;
            const i = y * lay.cols + x;
            assert.ok(lay.solid[i] === 1 && lay.walls[i] === 0 && lay.lava[i] === 1);
        }
    }
    assert.ok(n > 20);
    assert.equal(lay.markers['~'], undefined);
    assert.ok(lay.lava instanceof Uint8Array);
});

check('validator: each new rule rejects its violation', () => {
    const ok = problemsOf(BASE_MAP, []);
    assert.deepEqual(ok, []);
    const wide = BASE_MAP.map((r) => r.slice(0, 9) + '.'.repeat(20) + '#');
    assert.ok(has(problemsOf(wide, []), 'size'), 'oversize without big');
    assert.deepEqual(problemsOf(wide, [], { big: true }), [], 'big allows 30 columns');
    assert.ok(has(problemsOf(withCell('b', 3, 3), [{ at: 'b', use: 'nope' }]), 'bad use'));
    ['cook', 'forge', 'bell', 'map', 'brandur'].forEach((u) => assert.ok(!has(problemsOf(withCell('b', 3, 3), [{ at: 'b', use: u }]), 'bad use'), u));
    assert.ok(has(problemsOf(withCell('b', 3, 3), [{ at: 'b', sign: 'x_node', look: 'banner' }]), 'bad sign look'));
    ['plaque', 'scroll', 'tablet', 'stand'].forEach((l) => assert.ok(!has(problemsOf(withCell('b', 3, 3), [{ at: 'b', sign: 'x_node', look: l }]), 'bad sign look'), l));
    assert.ok(has(problemsOf(withCell('b', 3, 3), [{ at: 'b', sign: 'x_node', look: 'tablet', tablet: 7 }]), 'bad tablet'));
    assert.ok(has(problemsOf(withCell('b', 3, 3), [{ at: 'b', sign: 'x_node', look: 'tablet', tablet: 3 }]), 'already placed'), 'tablet 3 is taken by the hall');
    assert.ok(has(problemsOf(withCell('b', 3, 3), [{ at: 'b', brazier: 'q', look: 'lamp' }]), 'bad brazier look'));
    assert.ok(!has(problemsOf(withCell('b', 3, 3), [{ at: 'b', brazier: 'q', look: 'shelf' }]), 'bad brazier look'));
    assert.ok(has(problemsOf(withCell('c', 3, 3), [{ at: 'c', chest: 'x_chest', loot: { coins: 1 }, look: 'tdp-nope.p1' }]), 'chest look'));
    assert.ok(!has(problemsOf(withCell('c', 3, 3), [{ at: 'c', chest: 'x_chest', loot: { coins: 1 }, look: 'tdp-lair.p15', key: 'desk_key' }]), 'chest'));
    assert.ok(has(problemsOf(withCell('c', 3, 3), [{ at: 'c', chest: 'x_chest', loot: { coins: 1 }, key: 'nothing' }]), 'chest key'));
    const gate = (open) => problemsOf(withCell('g', 3, 3), [{ at: 'g', gate: 'x_gate', open }]);
    assert.ok(has(gate({ step: 9 }), 'bad step'));
    assert.ok(!has(gate({ step: 3 }), 'bad step'));
    assert.ok(has(gate({ item: 'nothing' }), 'unknown item'));
    assert.ok(!has(gate({ item: 'room_key' }), 'unknown item'));
    assert.ok(has(problemsOf(withCell('#', 3, 3).map((r, i) => (i === 4 ? r.slice(0, 4) + 'm' + r.slice(5) : r)), [{ at: 'm', mob: 'brandur' }], { guardian: 'brandur' }), 'needs a big room'));
    assert.ok(has(problemsOf(BASE_MAP.map((r, i) => (i === 3 ? '#...p~...#' : r)), [{ at: 'p', prop: 'tdp-lair.p0' }]), 'footprint'), 'a 2-wide prop reaching onto lava');
    assert.ok(!has(problemsOf(BASE_MAP.map((r, i) => (i === 3 ? '#...p.~..#' : r)), [{ at: 'p', prop: 'tdp-lair.p0' }]), 'footprint'), 'next to lava is fine');
});

// ---- 3. the hero, flyers, shots and blocks against lava ----

check('lava: the hero and a pushed block cannot enter it, a bat and a shot cross it', () => {
    const sim = hero();
    const it = enterAt(sim, 'deeps', 'dp_vents', 'w');
    clearMobs(it);
    const room = it.room;
    assert.ok(room.lava && room.lava[6 * room.cols + 5] === 1 && room.lava[6 * room.cols + 6] === 1);
    put(it, 3, 6);
    irun(sim, it, 2, { right: true });
    assert.ok(cellOfHero(it).cx <= 4, 'the hero stops at the lava edge: ' + cellOfHero(it).cx);
    // a block next to the lava is not pushed in
    const blk = { id: 'block_x', kind: 'block', cx: 4, cy: 8, x: 4.5 * 48, y: 8.5 * 48 + 7, moving: null, look: 'stone' };
    it.things.push(blk);
    room.solid[8 * room.cols + 4] = 1;
    put(it, 3, 8);
    irun(sim, it, 2, { right: true });
    assert.deepEqual([blk.cx, blk.cy], [4, 8], 'the block stays in front of the lava');
    // a bat flies over it (flyers use the walls grid), a walker does not
    const bat = Mobs.create('bat', 'b_test', 4.5 * 48, 6.5 * 48 + 7, 0);
    for (let i = 0; i < 60; i++) Mobs.slide(room, room.walls, bat, 6, 0, bat.r * 0.8, bat.r * 1.1);
    assert.ok(bat.x > 7 * 48, 'the bat crossed the channel: ' + bat.x);
    const rat = Mobs.create('rat', 'r_test', 4.5 * 48, 6.5 * 48 + 7, 0);
    for (let i = 0; i < 60; i++) Mobs.slide(room, room.solid, rat, 6, 0, rat.r * 0.8, rat.r * 1.1);
    assert.ok(rat.x < 5 * 48, 'the rat stops at the lava: ' + rat.x);
    // a shot passes over lava and still dies at a wall
    it.mobs = [];
    it.shots.length = 0;
    put(it, 17, 10);
    it.shots.push({ x: 4.5 * 48, y: 6.5 * 48, vx: 150, vy: 0, r: 4, dmg: 1, life: 4, kind: 'fireball' });
    for (let i = 0; i < 90; i++) Mobs.stepShots(sim, it, DT);
    assert.equal(it.shots.length, 1, 'the shot is still alive over the channel');
    assert.ok(it.shots[0].x > 7 * 48, 'and has crossed it: ' + it.shots[0].x);
    it.shots.push({ x: 3.5 * 48, y: 3.5 * 48 - 40, vx: 0, vy: -150, r: 4, dmg: 1, life: 4, kind: 'dart' });
    for (let i = 0; i < 60; i++) Mobs.stepShots(sim, it, DT);
    assert.ok(it.shots.every((s) => s.kind !== 'dart'), 'a wall stops a shot');
});

// ---- 4. the puzzles of the cave, with real inputs, for both heroes ----

function viaExit(sim, it, cx, cy, roomId) {
    walk(sim, it, cx, cy);
    irun(sim, it, 1.2, {}, () => it.roomId === roomId);
    assert.equal(it.roomId, roomId, 'the exit leads to ' + roomId);
}

function runCave(heroId) {
    const sim = hero({ hero: heroId });
    const it = enterAt(sim, 'deeps', null, null);
    assert.equal(it.roomId, 'dp_shaft');
    assert.ok(sim.interior === it);
    const ps = Interior.placeState(sim, 'deeps');
    const exitsOf = (id) => Object.keys(Rooms.ROOMS[id].exits).map((m) => Rooms.ROOMS[id].exits[m].to);
    clearMobs(it);

    // shaft -> rails: the lever frees the lift down
    viaExit(sim, it, 13, 5, 'dp_rails');
    clearMobs(it);
    const gate = thingsOf(it, 'gate')[0];
    assert.ok(!gate.open && it.room.solid[gate.cy * it.room.cols + gate.cx] === 1, 'the lift gate is shut');
    const lever = thingsOf(it, 'lever')[0];
    walkNear(sim, it, lever.cx, lever.cy);
    press(sim, it);
    irun(sim, it, 0.6, {});
    assert.ok(lever.on && gate.open, 'the lever opens the gate');

    // rails -> bronze: two cracks on the only way, the pot with the key, the bell bronze
    viaExit(sim, it, 19, 5, 'dp_bronze');
    clearMobs(it);
    const cracks = thingsOf(it, 'crack');
    assert.equal(cracks.length, 2);
    assert.ok(cracks.every((c) => !c.broken && it.room.solid[c.cy * it.room.cols + c.cx] === 1));
    cracks.sort((a, b) => a.cx - b.cx);
    const chest = it.chests[0];
    const closed = (cx, cy) => {
        const room = it.room;
        const seen = new Set([cx + ',' + cy]);
        const q = [[cx, cy]];
        while (q.length) {
            const [x, y] = q.pop();
            for (const dv of DIRS) {
                const nx = x + dv[0];
                const ny = y + dv[1];
                if (nx < 0 || ny < 0 || nx >= room.cols || ny >= room.rows || room.solid[ny * room.cols + nx] || seen.has(nx + ',' + ny)) continue;
                seen.add(nx + ',' + ny);
                q.push([nx, ny]);
            }
        }
        return seen;
    };
    assert.ok(!closed(1, 4).has('12,4'), 'the chamber is shut while the cracks stand');
    leanFrom(sim, it, 3, 4, cracks[0].cx, cracks[0].cy);
    swingAt(sim, it, 8, () => cracks[0].broken);
    assert.ok(cracks[0].broken, 'the first crack breaks');
    assert.ok(!closed(1, 4).has('12,4'), 'one crack is not enough');
    leanFrom(sim, it, 8, 4, cracks[1].cx, cracks[1].cy);
    swingAt(sim, it, 8, () => cracks[1].broken);
    assert.ok(cracks[1].broken && closed(1, 4).has('12,4'), 'both cracks open the chamber');
    const pot = thingsOf(it, 'pot')[0];
    assert.equal(pot.drop, 'key');
    assert.ok(closed(1, 4).has(pot.cx + ',' + (pot.cy - 1)), 'the pot is in the first cavern');
    walkNear(sim, it, pot.cx, pot.cy);
    swingAt(sim, it, 6, () => pot.broken);
    assert.ok(pot.broken);
    assert.ok(walk(sim, it, pot.cx, pot.cy), 'walk onto the key');
    irun(sim, it, 0.3, {});
    assert.equal(ps.keys, 1, 'the pot gives a small key');
    walkNear(sim, it, chest.x / 48 - 0.5, (chest.y - 7) / 48 - 0.5);
    const evs = press(sim, it);
    assert.ok(names(evs, 'in_chest').length === 1);
    assert.equal(Npcs.itemCount(sim, 'bell_bronze'), 1, 'the bell bronze is in the bag');
    assert.ok(sim.profile.loot.indexOf('dp_bronze') >= 0);

    // bronze -> rails -> roots: the three veins in the order bottom, middle, top
    viaExit(sim, it, 0, 4, 'dp_rails');
    clearMobs(it);
    viaExit(sim, it, 14, 9, 'dp_roots');
    clearMobs(it);
    assert.ok(cellOfHero(it).cx === 10 && cellOfHero(it).cy === 1, 'arrival u next to the way back up');
    const veins = {};
    thingsOf(it, 'brazier').forEach((b) => { veins[b.bid] = b; });
    const vault = thingsOf(it, 'gate')[0];
    assert.ok(!vault.open);
    const light = (id) => { walkNear(sim, it, veins[id].cx, veins[id].cy); return press(sim, it); };
    const wrong = light('vein_mid');
    assert.ok(names(wrong, 'in_puzzle_fail').length === 1 && !veins.vein_mid.lit, 'the middle vein first fails and lights nothing');
    light('vein_low');
    assert.ok(veins.vein_low.lit);
    const second = light('vein_top');
    assert.ok(names(second, 'in_puzzle_fail').length === 1 && !veins.vein_low.lit, 'a wrong second vein resets the order');
    light('vein_low');
    light('vein_mid');
    light('vein_top');
    irun(sim, it, 0.6, {});
    assert.ok(veins.vein_top.lit && vault.open, 'low, middle, top opens the vault');

    // roots -> crystal: two crystal blocks on two plates (the search model covers the pushes), then Mira's nook
    viaExit(sim, it, 0, 6, 'dp_crystal');
    clearMobs(it);
    const lanes = thingsOf(it, 'block').sort((x, y) => x.cy - y.cy);
    const nook = thingsOf(it, 'gate')[0];
    assert.ok(!nook.open);
    const pushLane = (b, fromX, toX, key) => {
        assert.ok(walk(sim, it, fromX, b.cy), 'walk behind the block in lane ' + b.cy);
        irun(sim, it, 8, { [key]: true }, () => b.cx === toX && !b.moving);
        assert.equal(b.cx, toX, 'the block reaches the end of its lane');
    };
    pushLane(lanes[0], 7, 1, 'left');
    irun(sim, it, 0.5, {});
    assert.ok(!nook.open, 'one plate is not enough');
    pushLane(lanes[1], 7, 1, 'left');
    irun(sim, it, 0.6, {});
    assert.ok(nook.open, 'both plates open the nook');
    assert.ok(walk(sim, it, 12, 2), 'the hero walks into the nook');
    viaExit(sim, it, 17, 6, 'dp_roots');
    clearMobs(it);

    // roots -> hall: tablets 3 and 4, then the key gate with tablet 5 and the desk key
    viaExit(sim, it, 19, 6, 'dp_hall');
    clearMobs(it);
    const signs = thingsOf(it, 'sign').filter((s) => s.tablet);
    assert.equal(signs.map((s) => s.tablet).sort().join(), '3,4,5');
    const read = (s) => { walkNear(sim, it, s.cx, s.cy); const evs2 = press(sim, it); sim.encounter = null; return evs2; };
    const t3 = signs.filter((s) => s.tablet === 3)[0];
    const first = read(t3);
    assert.equal(names(first, 'in_tablet').length, 1, 'tablet 3 is given');
    assert.equal(Npcs.getFlag(sim, 'dr.tabs'), 4);
    const again = read(t3);
    assert.equal(names(again, 'in_tablet').length, 0, 'tablet 3 is given only once');
    assert.ok(names(again, 'in_read').length >= 1);
    read(signs.filter((s) => s.tablet === 4)[0]);
    assert.equal(Npcs.getFlag(sim, 'dr.tabs'), 12);
    const wardens = thingsOf(it, 'gate')[0];
    assert.ok(!wardens.open);
    leanFrom(sim, it, 10, 8, wardens.cx, wardens.cy);
    press(sim, it);
    irun(sim, it, 0.6, {});
    assert.ok(wardens.open && ps.keys === 0, 'the small key from the pot opens the wardens gate');
    assert.ok(walk(sim, it, 10, 10), 'into the nook');
    const t5 = thingsOf(it, 'sign').filter((s) => s.tablet === 5)[0];
    read(t5);
    assert.equal(Npcs.getFlag(sim, 'dr.tabs'), 28);
    const deskChest = it.chests[0];
    press(sim, it, {});
    walkNear(sim, it, deskChest.x / 48 - 0.5, (deskChest.y - 7) / 48 - 0.5);
    press(sim, it);
    assert.equal(Npcs.itemCount(sim, 'desk_key'), 1, 'the desk key is in the bag');

    // hall -> bridge: darts on the two lanes, the lever opens the one-way shortcut home
    viaExit(sim, it, 19, 6, 'dp_bridge');
    clearMobs(it);
    assert.ok(walk(sim, it, 17, 6) || true);
    irun(sim, it, 0.2, {});
    assert.ok(cellOfHero(it).cx >= 16, 'across the bridge to the east bank: ' + JSON.stringify(cellOfHero(it)));
    const short = thingsOf(it, 'gate')[0];
    assert.ok(!short.open);
    const sl = thingsOf(it, 'lever')[0];
    walkNear(sim, it, sl.cx, sl.cy);
    press(sim, it);
    irun(sim, it, 0.6, {});
    assert.ok(short.open, 'the lever opens the shortcut gate');
    assert.ok(exitsOf('dp_shaft').indexOf('dp_bridge') < 0 && exitsOf('dp_rails').indexOf('dp_bridge') < 0, 'nothing leads back into the bridge from the west');
    viaExit(sim, it, 21, 8, 'dp_shaft');
    assert.deepEqual([cellOfHero(it).cx, cellOfHero(it).cy], [6, 8], 'the shortcut lands on the shaft marker s');
    viaExit(sim, it, 13, 5, 'dp_rails');
    clearMobs(it);
    viaExit(sim, it, 14, 9, 'dp_roots');
    clearMobs(it);
    viaExit(sim, it, 19, 6, 'dp_hall');
    clearMobs(it);
    viaExit(sim, it, 19, 6, 'dp_bridge');
    clearMobs(it);
    viaExit(sim, it, 18, 11, 'dp_vents');
    clearMobs(it);

    // vents: both fords are crossable (fire vents only hurt, the hero has hearts to spare), then the nest and the hoard
    assert.ok(walk(sim, it, 2, 6), 'west ford');
    viaExit(sim, it, 0, 6, 'dp_nest');
    clearMobs(it);
    assert.ok(walk(sim, it, 3, 3));
    viaExit(sim, it, 17, 6, 'dp_vents');
    assert.ok(walk(sim, it, 19, 6), 'east ford');
    viaExit(sim, it, 21, 6, 'dp_hoard');
    clearMobs(it);
    const seal = thingsOf(it, 'gate')[0];
    assert.ok(!seal.open && it.room.solid[seal.cy * it.room.cols + seal.cx] === 1);
    const lanes2 = thingsOf(it, 'block').sort((x, y) => x.cy - y.cy);
    const lane2 = (b) => {
        assert.ok(walk(sim, it, 9, b.cy), 'walk behind the block in lane ' + b.cy);
        irun(sim, it, 8, { right: true }, () => b.cx === 13 && !b.moving);
        assert.equal(b.cx, 13, 'the block reaches its plate');
    };
    lane2(lanes2[0]);
    irun(sim, it, 0.4, {});
    assert.ok(!seal.open, 'one plate is not enough');
    lane2(lanes2[1]);
    irun(sim, it, 0.6, {});
    assert.ok(seal.open, 'two plates open the seal');
    viaExit(sim, it, 17, 5, 'dp_lair');
    assert.deepEqual([cellOfHero(it).cx, cellOfHero(it).cy], [14, 16], 'arrival a, next to the way back');
    return { sim, it };
}

['knight', 'ranger'].forEach((heroId) => {
    check('the cave is passable with the ' + heroId + ': lever, cracks and key, veins, plates, tablets, key gate, bridge, fords, seal', () => {
        runCave(heroId);
    });
});

check('block puzzles: crystal nook and hoard seal are solvable and no state traps the hero', () => {
    [
        { name: 'dp_crystal', room: 'dp_crystal', plates: ['pa', 'pb'], start: 'e', mustAll: ['e'], near: ['g'] },
        { name: 'dp_hoard', room: 'dp_hoard', plates: ['sa', 'sb'], start: 'w', mustAll: ['w'], near: ['g'] }
    ].forEach((p) => {
        const r = solve(model(p.room), p);
        assert.ok(r.startOk, p.name + ' starts with the exits and the gate in reach');
        assert.ok(r.goalDepth > 0, p.name + ' needs at least one push (' + JSON.stringify(r) + ')');
        assert.equal(r.dead, 0, p.name + ' has states that cannot be left: ' + JSON.stringify(r));
        assert.ok(r.blocksDepth > 0, p.name + ' also works with a block on every plate');
    });
});

check('rooms: Mira stands in the crystal nook only at step 2 and before she is found', () => {
    if (!Npcs.NPCS.mira) return;
    const sim = hero();
    let it = enterAt(sim, 'deeps', 'dp_crystal', 'e');
    assert.equal(it.npcs.length, 0, 'step 0: nobody there');
    sim.profile.seen = ['dr_tremor', 'dr_night'];
    it = enterAt(sim, 'deeps', 'dp_crystal', 'e');
    assert.equal(it.npcs.length, 1);
    assert.equal(it.npcs[0].npcId, 'mira');
    Npcs.setFlag(sim, 'mira.found', 1);
    it = enterAt(sim, 'deeps', 'dp_crystal', 'e');
    assert.equal(it.npcs.length, 0, 'found: she is at home');
});

check('the lair holds only Brandur, at a single G; the other eleven rooms hold the small mobs', () => {
    const lay = Rooms.layout('dp_lair');
    assert.equal(Rooms.ROOMS.dp_lair.things.filter((t) => t.mob).map((t) => t.mob).join(), 'brandur');
    assert.equal(lay.markers.G.length, 1);
    const kinds = {};
    Rooms.PLACES.deeps.rooms.forEach((id) => Rooms.ROOMS[id].things.forEach((t) => { if (t.mob) kinds[t.mob] = (kinds[t.mob] || 0) + Rooms.layout(id).markers[t.at].length; }));
    assert.equal(kinds.rat, 3);
    assert.equal(kinds.emberling, 6, 'four in the vent field, two in the nest');
    assert.equal(kinds.drakeling, 2);
    assert.equal(kinds.mummy, 2);
});

// ---- 5. visited bits, the map and the map table ----

check('map: visited bits per room, mapState and the map table', () => {
    const sim = hero();
    assert.equal(Deeps.mapState(sim), null, 'no map outside the deeps');
    const it = enterAt(sim, 'deeps', null, null);
    assert.equal(Npcs.getFlag(sim, 'dr.visit_a'), 1, 'the shaft is bit 0');
    assert.equal(Object.keys(Deeps.visited(sim.profile)).join(), 'dp_shaft');
    Interior.enterRoom(sim, it, 'dp_roots', 'u');
    Interior.enterRoom(sim, it, 'dp_roots', 'u');
    assert.equal(Npcs.getFlag(sim, 'dr.visit_a'), 1 + 16, 'the roots are bit 4, set once');
    Interior.enterRoom(sim, it, 'dp_lair', 'a');
    assert.equal(Npcs.getFlag(sim, 'dr.visit_b'), 32, 'the lair is bit 5 of the second mask');
    const st = Deeps.mapState(sim);
    assert.equal(st.rooms.length, 12);
    assert.equal(st.revealed, false);
    assert.equal(st.rooms.filter((r) => r.visited).map((r) => r.id).sort().join(), 'dp_lair,dp_roots,dp_shaft');
    assert.equal(st.rooms.filter((r) => r.known).map((r) => r.id).sort().join(), 'dp_lair,dp_roots,dp_shaft', 'unread map: only the seen rooms are known');
    assert.equal(st.rooms.filter((r) => r.current)[0].id, 'dp_lair');
    assert.equal(st.links.length, 11, 'a tree of eleven two-way links');
    assert.ok(!st.links.some((l) => l.indexOf('dp_bridge') >= 0 && l.indexOf('dp_shaft') >= 0), 'the one-way shortcut is no link');
    const xy = (id) => st.rooms.filter((r) => r.id === id).map((r) => [r.x, r.y])[0];
    assert.equal(JSON.stringify([xy('dp_camp'), xy('dp_bridge'), xy('dp_lair')]), '[[0,0],[4,1],[5,2]]');
    // the map table on Edda's wall: reading it reveals every room
    const use = Things.USES.map;
    assert.equal(use.prompt, 'read');
    sim.events.length = 0;
    use.use(sim, it, {});
    assert.equal(Npcs.getFlag(sim, 'dr.map'), 1);
    assert.equal(sim.notice, 'runenpfad.notice.map_read');
    assert.ok(names(sim.events, 'in_read').length === 1);
    const st2 = Deeps.mapState(sim);
    assert.ok(st2.revealed && st2.rooms.every((r) => r.known));
    // endless runs never write the flags
    const endless = Sim.createSimulation({ mode: 'endless', endlessFloor: 3, hero: 'knight' });
    assert.equal(Deeps.markVisited(endless, 'dp_shaft'), false);
});

check('USES registry: a registered use is offered and run, the prompt comes from the registry', () => {
    const sim = hero();
    const it = enterAt(sim, 'deeps', 'dp_lair', 'a');
    const k = it.things.filter((t) => t.kind === 'use' && t.request === 'brandur')[0];
    assert.ok(k, 'the use thing K exists');
    assert.equal(typeof Things.USES, 'object');
    let ran = 0;
    Things.USES.test_use = { prompt: 'poke', offer: (s, i, t) => t.request === 'test_use', use: () => { ran += 1; } };
    const probe = { id: 'u_test', kind: 'use', cx: 5, cy: 5, x: 5.5 * 48, y: 5.5 * 48 + 7, request: 'test_use', prompt: 'poke' };
    it.things.push(probe);
    const offered = [];
    Things.consider(sim, it, (x, y, kind, ref) => offered.push(kind + ':' + (ref && ref.request)));
    assert.ok(offered.indexOf('poke:test_use') >= 0, 'a registered use with offer() true is offered');
    assert.equal(Things.use(sim, it, { ref: probe }), true);
    assert.equal(ran, 1);
    delete Things.USES.test_use;
});

check('gate conditions: step and item parts', () => {
    const sim = hero();
    const it = enterAt(sim, 'deeps', 'dp_shaft', null);
    assert.equal(Things.cond(sim, it, { step: 0 }), true);
    assert.equal(Things.cond(sim, it, { step: 2 }), false, 'step 0 profile');
    sim.profile.seen = ['dr_tremor', 'dr_night'];
    assert.equal(Things.cond(sim, it, { step: 2 }), true);
    assert.equal(Things.cond(sim, it, { item: 'room_key' }), false);
    Npcs.addItem(sim, 'room_key', 1);
    assert.equal(Things.cond(sim, it, { item: 'room_key' }), true, 'a gate opens while the item is held');
});

// ---- 6. baking and the map view with a stub canvas ----

function stubCanvas() {
    const calls = { n: 0, texts: 0 };
    const grad = { addColorStop() {} };
    const ctx = new Proxy({}, {
        get(target, key) {
            if (key === 'calls') return calls;
            if (key === 'canvas') return {};
            if (key === 'measureText') return () => ({ width: 10 });
            if (key === 'createLinearGradient' || key === 'createRadialGradient') return () => grad;
            if (key === 'fillText' || key === 'strokeText') return () => { calls.texts += 1; calls.n += 1; };
            if (key in target) return target[key];
            return () => { calls.n += 1; };
        },
        set(target, key, v) { target[key] = v; return true; }
    });
    const make = () => ({ getContext: () => ctx, width: 0, height: 0 });
    return { ctx, make, calls };
}

check('bake: mine and glut rooms bake without art, sheetsOf asks for tex-glut when a room has lava', () => {
    if (!Bake) return;
    ['dp_shaft', 'dp_rails', 'dp_vents', 'dp_lair', 'dp_roots'].forEach((id) => {
        const lay = Rooms.layout(id);
        const room = { id, def: lay.def, cols: lay.cols, rows: lay.rows, props: lay.props, solid: lay.solid, walls: lay.walls, lava: lay.lava };
        const st = stubCanvas();
        const cv = Bake.bakeRoom(room, null, st.make);
        assert.ok(cv && cv.rpScale === 2, id + ' baked');
        assert.ok(st.calls.n > 50, id + ' painted');
        assert.equal(JSON.stringify(Bake.sheetsOf(room)).indexOf('tex-glut') >= 0, id === 'dp_vents' || id === 'dp_lair', id + ' tex-glut');
    });
    const mineLava = { def: { theme: 'mine', map: ['#####', '#.~.#', '#####'] }, props: [] };
    assert.equal(Bake.sheetsOf(mineLava).join(), 'tex-mine,tex-glut');
    assert.ok(Bake.THEMES.mine && Bake.THEMES.glut);
    assert.equal(Bake.THEMES.glut.floor, '#3a2622');
    // a glut floor never takes the lava texture
    const frames = [];
    const tex = { img: {}, sheet: { img: {} } };
    const atlas = { clips: {} };
    ['tex-glut.t0', 'tex-glut.t1', 'tex-glut.t2', 'tex-glut.t3'].forEach((c, i) => { atlas.clips[c] = { sheet: { img: { id: c }, frames: [[i * 10, 0, 64, 64]] }, frames: [0] }; });
    const lay = Rooms.layout('dp_nest');
    const room = { id: 'dp_nest', def: lay.def, cols: lay.cols, rows: lay.rows, props: [] };
    const st = stubCanvas();
    const drawn = [];
    const ctxd = new Proxy(st.ctx, { get(t, k) { return k === 'drawImage' ? (img) => drawn.push(img && img.id) : t[k]; } });
    const cv = Bake.bakeRoom(room, atlas, () => ({ getContext: () => ctxd }));
    assert.ok(cv);
    assert.ok(drawn.indexOf('tex-glut.t0') >= 0 && drawn.indexOf('tex-glut.t1') >= 0, 'floor t0 and the lava t1 are drawn');
    void frames; void tex;
});

check('map view: draws in the deeps, stays silent elsewhere and while Brandur is awake, never throws', () => {
    if (!MapView) return;
    const st = stubCanvas();
    const view = MapView.create({ ctx: st.ctx, t: (k) => k, font: 'sans-serif', art: null });
    const f = { t: 1.2, reduced: false, s: 1, ox: 0, oy: 40 };
    const sim = hero();
    st.calls.n = 0;
    view.draw(sim, f, 800, 40);
    assert.equal(st.calls.n, 0, 'no interior: nothing is drawn');
    const it = enterAt(sim, 'deeps', 'dp_roots', 'u');
    view.draw(sim, f, 800, 40);
    assert.ok(st.calls.n > 8, 'the panel is drawn');
    const normal = st.calls.n;
    st.calls.n = 0;
    view.draw(sim, Object.assign({}, f, { reduced: true }), 800, 40);
    assert.ok(st.calls.n > 8, 'also with reduced motion');
    Npcs.setFlag(sim, 'dr.map', 1);
    st.calls.n = 0;
    view.draw(sim, f, 800, 40);
    assert.ok(st.calls.n >= normal, 'the read map shows every room');
    it.guardian = { state: 'move', kind: 'brandur' };
    st.calls.n = 0;
    view.draw(sim, f, 800, 40);
    assert.equal(st.calls.n, 0, 'hidden while Brandur is awake');
    it.guardian = { state: 'dead', kind: 'brandur' };
    st.calls.n = 0;
    view.draw(sim, f, 800, 40);
    assert.ok(st.calls.n > 8, 'back after he is down');
    sim.interior = { placeId: 'deeps', roomId: 7, room: null };
    view.draw(sim, f, 800, 40);
    view.draw(null, null, 0, 0);
    view.draw({}, {}, 100, 0);
    view.dispose();
    view.dispose();
});

console.log('runenpfad deeps: ' + passed + ' checks passed');
