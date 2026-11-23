import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Same order as module-loader.js; the rooms of this suite live in place-rooms-deep.js, behind village-rooms.js.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things', 'interior', 'places',
    'folk', 'folk-story', 'journal-story', 'village-rooms', 'place-rooms-deep', 'village', 'cutscenes', 'chronicle', 'dice', 'bounties', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Interior = sandbox.RunenpfadInterior;
const Places = sandbox.RunenpfadPlaces;
const Things = sandbox.RunenpfadInteriorThings;
const Npcs = sandbox.RunenpfadNpcs;
assert.ok(Levels && Sim && Rooms && Mobs && Interior && Places && Things && Npcs, 'modules loaded');
assert.ok(sandbox.RunenpfadInteriorBosses && sandbox.RunenpfadChronicle && sandbox.RunenpfadNpcStory, 'guardians, chronicle and story modules loaded');

const STEP = Sim.STEP;
const DT = 1 / 60;
const CAVE_ROOMS = { beast_den_b: 'brood', crystal_grotto_b: 'weaver', sun_tomb_c: 'tombward', ice_cave_b: 'frostheart', boiler_room_c: 'boilerward' };
const OBS_ROOMS = ['obs_entry', 'obs_gears', 'obs_lens', 'obs_core'];
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

function newSim(opts) {
    const sim = Sim.createSimulation(opts || { levelId: 3 });
    tick(sim, {}, 30);
    return sim;
}

const TEXT = JSON.parse(fs.readFileSync(path.join(rootDir, 'docs/superpowers/plans/r10-text.json'), 'utf8'));
const loreText = (node) => TEXT['runenpfad.npc.lore.' + node][0];

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

const PUZZLES = [
    { name: 'crystal_grotto_b', room: 'crystal_grotto_b', plates: ['pa', 'pb'], start: 'a', mustAll: ['a'], near: ['w'] },
    { name: 'ice_cave_b', room: 'ice_cave_b', plates: ['floe'], start: 'a', mustAll: ['a'], near: [] },
    { name: 'obs_gears', room: 'obs_gears', plates: ['gear_a', 'gear_b', 'gear_c'], start: 'a', mustAll: [], near: ['B', 'g'] }
];

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

// ---- 1. data ----

check('the validator is clean and every place lists its rooms', () => {
    assert.equal(JSON.stringify(Rooms.validate()), '[]');
    Object.keys(CAVE_ROOMS).forEach((id) => {
        const place = Object.keys(Rooms.PLACES).filter((p) => Rooms.PLACES[p].rooms.indexOf(id) >= 0);
        assert.equal(place.length, 1, id + ' belongs to one place');
        assert.equal(Rooms.ROOMS[id].guardian, CAVE_ROOMS[id], id + ' guardian');
        const d = Rooms.ROOMS[id];
        assert.equal(d.exits.B.at, 'x', id + ' back exit lands on the floor marker x');
        const lay = Rooms.layout(id);
        const a = lay.markers.a[0];
        const b = lay.markers.B[0];
        assert.equal(Math.abs(a.cx - b.cx) + Math.abs(a.cy - b.cy), 1, id + ': the arrival a stands next to the exit B');
        const chest = d.things.filter((t) => t.chest)[0];
        assert.equal(chest.chest, id);
        assert.deepEqual(JSON.parse(JSON.stringify(chest.loot)), { coins: 80, give: 'potion' });
        assert.equal(d.things.filter((t) => t.gate && t.open.clear && t.stay).length, 1, id + ' clear gate');
        assert.equal(d.things.filter((t) => t.mob).length >= 1, true);
    });
    const pages = Object.keys(CAVE_ROOMS).map((id) => Rooms.ROOMS[id].things.filter((t) => t.page)[0].page);
    assert.deepEqual(pages, [2, 4, 6, 8, 10]);
    const o = Rooms.PLACES.observatory;
    assert.equal(o.kind, 'house');
    assert.equal(o.requires.flag, 'obs.open');
    assert.equal(o.facade.clip, 'fac-observatory.f0');
    assert.equal(o.nameKey, 'runenpfad.place.observatory');
    assert.deepEqual(JSON.parse(JSON.stringify(o.rooms)), OBS_ROOMS);
    assert.equal(Rooms.PLACES.beast_den.rooms.join(), 'beast_den,beast_den_b');
    assert.equal(Rooms.PLACES.sun_tomb.rooms.join(), 'sun_tomb_a,sun_tomb_b,sun_tomb_c');
    // every room fits the size rule and its way back exists
    Object.keys(CAVE_ROOMS).concat(OBS_ROOMS).forEach((id) => {
        const d = Rooms.ROOMS[id];
        assert.ok(d.map[0].length >= 10 && d.map[0].length <= 22 && d.map.length >= 8 && d.map.length <= 14, id + ' size');
        if (OBS_ROOMS.indexOf(id) >= 0) assert.equal(d.music, 'in_obs');
    });
});

// ---- 2. every block puzzle is solvable and no block can trap the hero ----

check('block puzzles: solvable by pushes, no state without a way back to the exit and the gate', () => {
    PUZZLES.forEach((p) => {
        const r = solve(model(p.room), p);
        const d = Rooms.ROOMS[p.room];
        assert.ok(r.startOk, p.name + ' starts with the exit and the gate in reach');
        assert.ok(r.goalDepth > 0, p.name + ' needs at least one push (' + JSON.stringify(r) + ')');
        assert.equal(r.dead, 0, p.name + ' has states that cannot be left: ' + JSON.stringify(r));
        if (d.things.filter((t) => t.block).length && p.name !== 'ice_cave_b') assert.ok(r.blocksDepth > 0, p.name + ' also works with a block on every plate');
        if (process.env.RP_VERBOSE) console.log(p.name, JSON.stringify(r));
    });
});


// ---- 3. the braziers: orders, signs and the decoration beside each flame ----

function propAnchor(p) { return { cx: Math.round(p.x / 48 - p.wCells / 2), cy: Math.round(p.y / 48 - 1) }; }

function checkBraziers(roomId, setId, signRoom, signNode, words, deco) {
    const d = Rooms.ROOMS[roomId];
    const lay = Rooms.layout(roomId);
    const text = loreText(signNode).toLowerCase();
    const said = words.map((w) => [w, text.indexOf(w.toLowerCase())]).sort((a, b) => a[1] - b[1]);
    assert.ok(said.every((x) => x[1] >= 0), 'the sign names every word');
    assert.deepEqual(JSON.parse(JSON.stringify(d.orders[setId])), said.map((x) => x[0]), roomId + ': the order equals the sign text');
    assert.ok(Rooms.ROOMS[signRoom].things.some((t) => t.sign === signNode), signRoom + ' has the sign ' + signNode);
    const braziers = d.things.filter((t) => t.brazier);
    assert.equal(braziers.length, 5);
    const xs = braziers.map((t) => ({ id: t.brazier, x: lay.markers[t.at][0].cx })).sort((a, b) => a.x - b.x).map((b) => b.id);
    assert.notEqual(xs.join(), d.orders[setId].join(), 'the flames do not stand in the solution order');
    assert.notEqual(xs.join(), d.orders[setId].slice().reverse().join(), 'nor in the reverse order');
    braziers.forEach((t) => {
        assert.equal(t.set, setId);
        const c = lay.markers[t.at][0];
        const near = lay.props.filter((p) => p.clip === deco[t.brazier]).map(propAnchor).filter((a) => Math.abs(a.cx - c.cx) + Math.abs(a.cy - c.cy) === 1);
        assert.equal(near.length, 1, roomId + ': ' + t.brazier + ' has its decoration ' + deco[t.brazier] + ' directly beside it');
    });
}

check('brazier orders equal the sign texts and every flame has its decoration directly beside it', () => {
    checkBraziers('sun_tomb_c', 'realms', 'sun_tomb_c', 'tomb_mural', ['wood', 'root', 'sand', 'ice', 'brass'],
        { wood: 'tdp-cave.p10', root: 'tdp-cave.p5', sand: 'tdp-ruin.p1', ice: 'tdp-ruin.p8', brass: 'tdp-ruin.p12' });
    checkBraziers('obs_lens', 'stars', 'obs_entry', 'obs_chart', ['boar', 'crown', 'eye', 'wyrm', 'gear'],
        { boar: 'tdp-stars.p0', crown: 'tdp-stars.p1', eye: 'tdp-stars.p2', wyrm: 'tdp-stars.p3', gear: 'tdp-stars.p4' });
});

// ---- 4. the puzzles with real inputs ----

function hero(opts) {
    const sim = Sim.createSimulation({ levelId: (opts && opts.levelId) || 3, hero: (opts && opts.hero) || 'knight', seed: (opts && opts.seed) || 77 });
    tick(sim, {}, 30);
    return sim;
}

check('crystal grotto: two crystal blocks pushed onto two plates open the way to the weaver', () => {
    const sim = hero();
    sim.player.hp = 99;
    const it = enterAt(sim, 'crystal_grotto', 'crystal_grotto_b', 'a');
    const [b1, b2] = thingsOf(it, 'block');
    const gates = thingsOf(it, 'gate').filter((g) => g.gid === 'seal');
    assert.equal(gates.length, 3);
    assert.ok(gates.every((g) => !g.open && it.room.solid[g.cy * it.room.cols + g.cx] === 1));
    const push = (b, dir, cx, cy) => irun(sim, it, 4, dir, () => b.cx === cx && b.cy === cy && !b.moving);
    walk(sim, it, 2, 6);
    push(b1, { up: true }, 2, 2);
    assert.equal(b1.cy, 2);
    walk(sim, it, 3, 2);
    push(b1, { left: true }, 1, 2);
    assert.deepEqual([b1.cx, b1.cy], [1, 2]);
    walk(sim, it, 5, 6);
    push(b2, { up: true }, 5, 2);
    walk(sim, it, 4, 2);
    const evs = push(b2, { right: true }, 6, 2);
    assert.deepEqual([b2.cx, b2.cy], [6, 2]);
    evs.push(...irun(sim, it, 1, {}));
    assert.ok(gates.every((g) => g.open), 'the plates open the gate');
    assert.ok(names(evs, 'in_gate').some((e) => e.detail.open === true));
    assert.ok(walk(sim, it, 8, 8), 'the hero walks through into the chamber');
});

check('ice cave: the slide route with a pushed ice block reaches the plate and the chamber', () => {
    const sim = hero();
    sim.player.hp = 99;
    const it = enterAt(sim, 'ice_cave', 'ice_cave_b', 'a');
    const gate = thingsOf(it, 'gate').filter((g) => g.gid === 'floe')[0];
    const block = thingsOf(it, 'block')[0];
    assert.ok(!gate.open);
    walk(sim, it, 5, 3);
    const evs = irun(sim, it, 1.5, { right: true });
    assert.deepEqual([cellOfHero(it).cx, cellOfHero(it).cy], [6, 3], 'the pillar stops the hero at the lake edge');
    evs.push(...irun(sim, it, 1.5, { up: true }));
    assert.deepEqual([cellOfHero(it).cx, cellOfHero(it).cy], [6, 1]);
    evs.push(...irun(sim, it, 4, { right: true }, () => cellOfHero(it).cx === 10 && !block.moving));
    assert.deepEqual([block.cx, block.cy], [11, 1], 'the block slides until the pillar and stops');
    assert.equal(cellOfHero(it).cx, 10, 'the hero slides up to the block');
    evs.push(...irun(sim, it, 2, { down: true }, () => gate.open));
    assert.ok(gate.open, 'crossing the plate opens the gate');
    if (process.env.RP_DEBUG) console.log('ICE events', names(evs, 'in_slide').length, 'slide', names(evs, 'in_push').length, 'push');
    assert.ok(names(evs, 'in_slide').length > 0 && names(evs, 'in_push').length > 0);
    evs.push(...irun(sim, it, 1.5, { down: true }));
    assert.ok(cellOfHero(it).cy >= 7, 'the hero reaches the rough floor of the chamber');
    // the way back: up through the gate, along the lake and out of the entry
    const room = it.roomId;
    irun(sim, it, 1.5, { up: true });
    irun(sim, it, 1.5, { left: true });
    irun(sim, it, 1.5, { down: true });
    assert.deepEqual([cellOfHero(it).cx, cellOfHero(it).cy], [6, 3], 'back at the lake edge');
    irun(sim, it, 1.2, { left: true });
    assert.ok(cellOfHero(it).cx <= 5, 'out of the lake and back on the rough floor of the entry');
    assert.equal(it.roomId, room);
});

check('gear room: three stone blocks pushed along their lanes open the gate', () => {
    const sim = hero();
    sim.player.hp = 99;
    const it = enterAt(sim, 'observatory', 'obs_gears', 'a');
    it.mobs = []; // a living clockbug would stand in the way of a block
    const blocks = thingsOf(it, 'block');
    const gates = thingsOf(it, 'gate');
    assert.equal(blocks.length, 3);
    assert.equal(gates.length, 2);
    const lane = (b, fromX, y, toX) => {
        assert.ok(walk(sim, it, fromX, y), 'walk to the start of the lane ' + y);
        irun(sim, it, 14, { right: true }, () => b.cx === toX && !b.moving);
        assert.deepEqual([b.cx, b.cy], [toX, y]);
    };
    lane(blocks.filter((b) => b.cy === 3)[0], 5, 3, 14);
    lane(blocks.filter((b) => b.cy === 5)[0], 4, 5, 14);
    assert.ok(gates.every((g) => !g.open), 'two plates are not enough');
    lane(blocks.filter((b) => b.cy === 7)[0], 5, 7, 14);
    irun(sim, it, 1, {});
    assert.ok(gates.every((g) => g.open), 'three plates open the gate');
});

check('the lever of the star hall opens the way to the gear room', () => {
    const sim = hero({ levelId: 12 });
    sim.player.hp = 99;
    const it = enterAt(sim, 'observatory', null, null);
    assert.equal(it.roomId, 'obs_entry');
    const gate = thingsOf(it, 'gate')[0];
    const lever = thingsOf(it, 'lever')[0];
    assert.ok(!gate.open);
    // the plate clue and the urns: smashing the urns drops nothing that blocks the way
    assert.ok(thingsOf(it, 'pot').length >= 4, 'urns to smash');
    const spot = walkNear(sim, it, lever.cx, lever.cy);
    assert.ok(spot);
    const evs = press(sim, it);
    assert.ok(lever.on && names(evs, 'in_lever').length === 1, 'the lever is pulled');
    irun(sim, it, 0.5, {});
    assert.ok(gate.open, 'the lever opens the gate');
    assert.ok(walk(sim, it, 14, 5), 'walk to the gate');
    irun(sim, it, 1.5, { right: true });
    assert.equal(it.roomId, 'obs_gears', 'the exit leads on to the gear room');
    assert.ok(cellOfHero(it).cx <= 6 && cellOfHero(it).cy === 5, 'the hero arrives at the west end of the gear room');
});

check('boiler room: the first valve silences the first vents, the second the vents behind it', () => {
    const sim = hero();
    sim.player.hp = 99;
    const it = enterAt(sim, 'boiler_room', 'boiler_room_c', 'a');
    const vents = (id) => thingsOf(it, 'spikes').filter((s) => s.when.off[0] === id);
    assert.equal(vents('valve_a').length, 4);
    assert.equal(vents('valve_b').length, 4);
    const evs = irun(sim, it, 4, {});
    assert.ok(names(evs, 'in_trap_warn').some((e) => e.detail.kind === 'vent'), 'the vents warn before they blow');
    assert.ok(vents('valve_a').concat(vents('valve_b')).every((v) => v.look === 'vent'));
    const sign = thingsOf(it, 'sign')[0];
    const lever = thingsOf(it, 'lever').filter((l) => l.lid === 'valve_a')[0];
    assert.ok(Math.abs(sign.cx - lever.cx) + Math.abs(sign.cy - lever.cy) <= 1, 'the note hangs beside the first valve');
    walkNear(sim, it, lever.cx, lever.cy);
    press(sim, it);
    assert.ok(lever.on);
    const seen = new Set();
    irun(sim, it, 6, {}, () => { vents('valve_a').forEach((v) => seen.add(v.phase)); return false; });
    assert.deepEqual([...seen], ['off'], 'the first vents stay quiet');
    const seenB = new Set();
    irun(sim, it, 4, {}, () => { vents('valve_b').forEach((v) => seenB.add(v.phase)); return false; });
    assert.ok(seenB.has('up'), 'the second vents still blow');
    const lever2 = thingsOf(it, 'lever').filter((l) => l.lid === 'valve_b')[0];
    walkNear(sim, it, lever2.cx, lever2.cy);
    press(sim, it);
    assert.ok(lever2.on);
    const seenC = new Set();
    irun(sim, it, 6, {}, () => { vents('valve_b').forEach((v) => seenC.add(v.phase)); return false; });
    assert.deepEqual([...seenC], ['off']);
});

// ---- 5. every trap can be passed without damage by waiting for its safe window ----

function trapWalk(hero2, roomId, placeId, route, idle, naive) {
    const sim = hero({ hero: hero2 });
    const it = enterAt(sim, placeId, roomId, 'a');
    it.mobs = [];
    const hp0 = sim.player.hp;
    irun(sim, it, idle || 0, {}); // start the walk at another phase of the traps
    const hp1 = sim.player.hp;
    const DARTS = thingsOf(it, 'dart');
    const unsafe = (cx, cy) => {
        const sp = thingsOf(it, 'spikes').filter((s) => s.cx === cx && s.cy === cy)[0];
        if (sp && sp.phase !== 'off' && (sp.phase !== 'down' || (sp.sp.period - 0.5 - sp.sp.up) - sp.t < 1.0)) return true;
        return DARTS.some((d) => {
            const v = { left: [-1, 0], right: [1, 0], up: [0, -1], down: [0, 1] }[d.dir];
            let x = d.cx + v[0];
            let y = d.cy + v[1];
            while (x >= 0 && y >= 0 && x < it.room.cols && y < it.room.rows && !it.room.solid[y * it.room.cols + x]) {
                if (x === cx && y === cy) {
                    return d.phase === 'warn' || it.shots.some((s) => s.kind === 'dart' && Math.hypot(s.x - (cx + 0.5) * 48, s.y - (cy + 0.5) * 48) < 150);
                }
                x += v[0];
                y += v[1];
            }
            return false;
        });
    };
    let wait = 0;
    for (const w of route) {
        for (let i = 0; i < 60 * 40; i++) {
            const tx = (w.cx + 0.5) * 48;
            const ty = (w.cy + 0.5) * 48 + 7;
            const dx = tx - it.hero.x;
            const dy = ty - it.hero.y;
            if (Math.abs(dx) < 3 && Math.abs(dy) < 3) break;
            const here = cellOfHero(it);
            const sx = Math.sign(Math.abs(dx) > 3 ? dx : 0);
            const sy = Math.sign(Math.abs(dy) > 3 ? dy : 0);
            const ahead = (sx || sy) && (unsafe(here.cx + sx, here.cy + sy) || unsafe(here.cx + 2 * sx, here.cy + 2 * sy));
            const cross = ahead && !unsafe(here.cx, here.cy);
            if (cross && !naive) { wait += 1; istep(sim, it, {}); } else istep(sim, it, { left: dx < -3, right: dx > 3, up: dy < -3, down: dy > 3 });
        }
    }
    return { hurt: Math.min(hp0, hp1) - sim.player.hp, wait, cell: cellOfHero(it) };
}

check('spikes, vents and darts: both heroes cross every trap corridor without damage, and the traps are live', () => {
    const route = (y, x0, x1) => { const r = []; for (let x = x0; x <= x1; x++) r.push({ cx: x, cy: y }); return r; };
    const tombRoute = [{ cx: 3, cy: 2 }].concat(route(2, 4, 9));
    const boilerRoute = [{ cx: 3, cy: 3 }, { cx: 4, cy: 3 }].concat(route(3, 5, 16)).concat([{ cx: 15, cy: 4 }, { cx: 15, cy: 6 }]);
    ['knight', 'ranger'].forEach((h) => {
        [0, 1.1, 2.2].forEach((idle) => {
            const tomb = trapWalk(h, 'sun_tomb_c', 'sun_tomb', tombRoute, idle);
            assert.equal(tomb.hurt, 0, h + ' crosses the spike corridor of the tomb unhurt (phase ' + idle + ')');
            assert.equal(tomb.cell.cx, 9);
            const boiler = trapWalk(h, 'boiler_room_c', 'boiler_room', boilerRoute, idle);
            assert.equal(boiler.hurt, 0, h + ' crosses the vents and darts of the boiler room unhurt (phase ' + idle + ')');
            assert.equal(boiler.cell.cy, 6);
        });
        // control: walking blindly at full speed gets hurt at some phase, so the waiting above is what keeps the hero safe
        const blind = [0, 1.1, 2.2].map((idle) => trapWalk(h, 'boiler_room_c', 'boiler_room', boilerRoute, idle, true).hurt);
        assert.ok(blind.some((n) => n > 0), 'the traps of the boiler room hurt a hero who does not wait: ' + blind.join());
        const blindTomb = [0, 1.1, 2.2].map((idle) => trapWalk(h, 'sun_tomb_c', 'sun_tomb', tombRoute, idle, true).hurt);
        assert.ok(blindTomb.some((n) => n > 0), 'the spikes of the tomb hurt a hero who does not wait: ' + blindTomb.join());
    });
});

// ---- 6. every guardian room: both heroes win with the simple bot, then the nook opens ----

// Scripted bot (as in test-runenpfad-bosses.mjs): retreat diagonally during tells, strike otherwise. `area` is the chamber (cells).
function bot(sim, boss, area) {
    const it = sim.interior;
    const h = it.hero;
    const input = {};
    const reach = sim.heroId === 'knight' ? 52 : 46;
    const cxr0 = ((area[0] + area[2] + 1) / 2) * 48;
    const cyr0 = ((area[1] + area[3] + 1) / 2) * 48;
    if (sim.player.hp <= 2 && sim.player.potions > 0) input.healPressed = true;
    const bx = boss.x - h.x;
    const by = boss.y - h.y;
    const dist = Math.hypot(bx, by) || 1;
    let dir;
    if (boss.move !== 'cross') bot.crossSign = 0; // one vertical choice per steam cross, not one per tick
    if (boss.aggro && (boss.state === 'tell' || boss.state === 'attack')) {
        const ax = -bx / dist;
        const ay = -by / dist;
        if (boss.move === 'cross') {
            // leave both steam bands: away along the row, and out of the row towards the middle of the chamber
            const dy0 = h.y - boss.y;
            if (!bot.crossSign) bot.crossSign = Math.abs(dy0) > 45 ? Math.sign(dy0) : (cyr0 > h.y ? 1 : -1);
            dir = { x: h.x >= boss.x ? 1 : -1, y: bot.crossSign };
        } else if (boss.move === 'ring' || boss.move === 'clock') {
            const off = (h.x - boss.x) * -boss.ay + (h.y - boss.y) * boss.ax;
            dir = { x: boss.ax + boss.ay * off * 0.1, y: boss.ay - boss.ax * off * 0.1 };
        } else {
            const rot = (sg) => ({ x: ax * Math.cos(0.8 * sg) - ay * Math.sin(0.8 * sg), y: ax * Math.sin(0.8 * sg) + ay * Math.cos(0.8 * sg) });
            const cxr = cxr0 - h.x;
            const cyr = cyr0 - h.y;
            let s = (rot(1).x * cxr + rot(1).y * cyr) > (rot(-1).x * cxr + rot(-1).y * cyr) ? 1 : -1;
            if (boss.beam) s = boss.beam.a1 > boss.beam.a0 ? -1 : 1;
            dir = rot(s);
        }
        (boss.zones || []).forEach((z) => {
            const zd = Math.hypot(h.x - z.x, h.y - 7 - z.y);
            if (zd < z.r + 30) dir = { x: (h.x - z.x) / (zd || 1), y: (h.y - 7 - z.y) / (zd || 1) };
        });
        if (h.dashCool <= 0 && boss.state === 'attack' && dist < 70) input.dashPressed = true;
    } else {
        let tg = boss;
        it.mobs.forEach((o) => {
            if (o.boss || o.state === 'dead') return;
            if (Math.hypot(o.x - h.x, o.y - h.y) < Math.hypot(tg.x - h.x, tg.y - h.y) || tg === boss) tg = o;
        });
        if (tg !== boss && (Math.hypot(tg.x - h.x, tg.y - h.y) > 150 || dist - boss.r < reach * 0.85)) tg = boss;
        const tx = tg.x - h.x;
        const ty = tg.y - h.y;
        const td = Math.hypot(tx, ty) || 1;
        dir = td > tg.r + 18 ? { x: tx / td, y: ty / td } : { x: 0, y: 0 };
        if (td - tg.r < reach * 0.85) input.attackPressed = true;
        const teller = it.mobs.filter((o) => !o.boss && (o.state === 'tell' || o.state === 'attack') && Math.hypot(o.x - h.x, o.y - h.y) < 130)[0];
        if (teller) {
            const ox = h.x - teller.x;
            const oy = h.y - teller.y;
            const od = Math.hypot(ox, oy) || 1;
            dir = { x: ox / od - oy / od * 0.8, y: oy / od + ox / od * 0.8 };
            delete input.attackPressed;
            if (h.dashCool <= 0 && od < 80) input.dashPressed = true;
        }
    }
    let near = null;
    it.shots.forEach((sh) => {
        const sx = sh.x - h.x;
        const sy = sh.y - (h.y - 7);
        const sd = Math.hypot(sx, sy);
        if (sd < 110 && sx * sh.vx + sy * sh.vy < 0 && (!near || sd < near.sd)) near = { sd, sh, sx, sy };
    });
    if (near) {
        const sp = Math.hypot(near.sh.vx, near.sh.vy) || 1;
        const side = (near.sx * near.sh.vy - near.sy * near.sh.vx) > 0 ? 1 : -1;
        dir = { x: -near.sh.vy / sp * side, y: near.sh.vx / sp * side };
        delete input.attackPressed;
    }
    it.room.exits.forEach((e) => {
        const ex = (e.cx + 0.5) * 48 - h.x;
        const ey = (e.cy + 0.5) * 48 - (h.y - 7);
        const ed = Math.hypot(ex, ey);
        if (ed < 90) { dir = { x: -ex / ed * 2, y: -ey / ed * 2 }; delete input.attackPressed; }
    });
    const n = Math.hypot(dir.x, dir.y) || 1;
    if (dir.x / n > 0.35) input.right = true;
    if (dir.x / n < -0.35) input.left = true;
    if (dir.y / n > 0.35) input.down = true;
    if (dir.y / n < -0.35) input.up = true;
    return input;
}

const lightAll = (order) => (sim, it) => {
    order.forEach((id) => Things.use(sim, it, { ref: thingsOf(it, 'brazier').filter((b) => b.bid === id)[0] }, {}));
    tick(sim, {}, 2);
};

const FIGHTS = [
    { room: 'beast_den_b', place: 'beast_den', area: [8, 1, 20, 8], enter: [8, 7], open: null },
    {
        room: 'crystal_grotto_b', place: 'crystal_grotto', area: [8, 4, 20, 12], enter: [8, 8],
        open: (sim, it) => { const [b1, b2] = thingsOf(it, 'block'); placeBlock(it, b1, 1, 2); placeBlock(it, b2, 6, 2); tick(sim, {}, 2); }
    },
    { room: 'sun_tomb_c', place: 'sun_tomb', area: [1, 6, 15, 12], enter: [14, 6], open: lightAll(['wood', 'root', 'sand', 'ice', 'brass']) },
    {
        room: 'ice_cave_b', place: 'ice_cave', area: [1, 7, 20, 12], enter: [10, 7],
        open: (sim, it) => { placeBlock(it, thingsOf(it, 'block')[0], 10, 4); tick(sim, {}, 2); }
    },
    { room: 'boiler_room_c', place: 'boiler_room', area: [1, 6, 16, 12], enter: [15, 6], open: null },
    { room: 'obs_core', place: 'observatory', area: [1, 5, 20, 12], enter: [1, 9], open: null }
];

function fight(f, heroId) {
    const sim = hero({ hero: heroId, levelId: f.place === 'observatory' ? 12 : 3 });
    const it = enterAt(sim, f.place, f.room, 'a');
    const hp0 = sim.player.hp;
    if (f.open) f.open(sim, it);
    put(it, f.enter[0], f.enter[1]);
    const log = { hp0, tells: 0, minTell: 99, events: [] };
    let tellAt = null;
    for (let i = 0; i < 60 * 150 && sim.interior === it; i++) {
        const boss = it.mobs.filter((m) => m.boss)[0];
        if (!boss) break;
        tick(sim, bot(sim, boss, f.area)).forEach((e) => {
            log.events.push(e.type);
            if (process.env.RP_DEBUG && e.type === 'in_hit' && e.detail.target === 'hero') console.log('HURT', f.room, heroId, boss.move || boss.state, 'hero', it.hero.x | 0, it.hero.y | 0, 'boss', boss.x | 0, boss.y | 0, 't', it.t.toFixed(1));
            if (e.type === 'in_boss_tell') tellAt = it.t;
            if (e.type === 'in_boss_attack') log.minTell = Math.min(log.minTell, it.t - tellAt);
        });
        if (boss.state === 'dead') break;
    }
    return { sim, it, log };
}

FIGHTS.forEach((f) => {
    check('guardian room ' + f.room + ': both heroes win, the nook opens, the lectern and the chest can be reached', () => {
        ['knight', 'ranger'].forEach((heroId) => {
            const { sim, it, log } = fight(f, heroId);
            const where = f.room + '/' + heroId;
            assert.ok(sim.interior === it && sim.player.hp > 0, where + ' hero alive (hp ' + sim.player.hp + '/' + log.hp0 + ')');
            assert.equal(log.events.filter((e) => e === 'in_boss_die').length, 1, where + ' guardian died');
            assert.ok(log.minTell >= 0.6 - 1e-6, where + ' tell ' + log.minTell);
            // the adds count for the clear; finish them like the bot does, then give the room a moment
            let guard = 0;
            while (!it.cleared && guard++ < 60 * 40) {
                const left = it.mobs.filter((m) => m.state !== 'dead')[0];
                if (!left) { tick(sim, {}); continue; }
                tick(sim, bot(sim, left, f.area));
            }
            tick(sim, {}, 30);
            assert.ok(it.cleared, where + ' room clear');
            const gate = thingsOf(it, 'gate').filter((g) => g.gid === 'nook' || g.gid === 'core')[0];
            assert.ok(gate.open, where + ' the clear gate is open');
            sim.encounter = null;
            const chest = it.chests[0];
            assert.ok(!chest.locked, where + ' chest unlocked');
            const coins0 = sim.profile.coins || 0;
            walkNear(sim, it, Math.floor(chest.x / 48), Math.floor((chest.y - 7) / 48));
            const evs = press(sim, it);
            assert.ok(names(evs, 'in_chest').some((e) => !e.detail.empty), where + ' the chest opens');
            assert.ok(sim.profile.loot.indexOf(chest.lootId) >= 0, where + ' the chest id is remembered');
            const want = f.room === 'obs_core' ? 150 : 80;
            assert.ok((sim.profile.coins || 0) - coins0 >= want || sim.stageCoins >= want, where + ' the chest pays ' + want + ' coins');
            const lectern = thingsOf(it, 'page')[0];
            if (lectern) {
                walkNear(sim, it, lectern.cx, lectern.cy);
                press(sim, it);
                const key = 'page_' + (lectern.n < 10 ? '0' : '') + lectern.n;
                assert.ok(sim.profile.loot.indexOf(key) >= 0, where + ' the page ' + lectern.n + ' is found');
                assert.ok(sim.encounter && sim.encounter.npcId === 'journal', where + ' the journal opens');
                sim.encounter = null;
            } else {
                const core = thingsOf(it, 'use')[0];
                walkNear(sim, it, core.cx, core.cy);
                press(sim, it);
                assert.ok(sim.encounter && sim.encounter.npcId === 'core', where + ' the core speaks');
                sim.encounter = null;
            }
        });
    });
});


// ---- 7. signs, endless, the real doorway of the beast den ----

const PLACE_OF = {};
Object.keys(Rooms.PLACES).forEach((p) => Rooms.PLACES[p].rooms.forEach((r) => { PLACE_OF[r] = p; }));

check('every sign of the new rooms opens its inscription', () => {
    Object.keys(CAVE_ROOMS).concat(OBS_ROOMS).forEach((id) => {
        Rooms.ROOMS[id].things.filter((t) => t.sign).forEach((t) => {
            assert.ok(TEXT['runenpfad.npc.lore.' + t.sign], 'text for ' + t.sign);
            const sim = hero({ levelId: 12 });
            const it = enterAt(sim, PLACE_OF[id], id, 'a');
            const sign = thingsOf(it, 'sign').filter((s) => s.node === t.sign)[0];
            walkNear(sim, it, sign.cx, sign.cy);
            const evs = press(sim, it);
            assert.ok(names(evs, 'in_read').length === 1 && sim.encounter && sim.encounter.nodeId === 'lore.' + t.sign, id + ': ' + t.sign + ' opens');
            sim.encounter = null;
        });
    });
});

check('endless: the new rooms work, lecterns are skipped and chests pay 15 coins', () => {
    Object.keys(CAVE_ROOMS).forEach((id) => {
        const sim = newSim({ mode: 'endless', endlessFloor: 3 });
        const it = enterAt(sim, PLACE_OF[id], id, 'a');
        assert.equal(thingsOf(it, 'page').length, 0, id + ' no lectern in endless');
        assert.equal(it.chests.length, 1);
        assert.equal(it.chests[0].repeat, true);
        assert.deepEqual(JSON.parse(JSON.stringify(it.chests[0].loot)), { coins: 15 });
        assert.ok(it.mobs.some((m) => m.boss), id + ' still holds its guardian');
    });
});

check('end to end: the beast den exit X leads to the brood chamber and the way back arrives on x', () => {
    const den = Rooms.ROOMS.beast_den;
    if (!den.exits.X || !Rooms.layout('beast_den').markers.x) {
        console.log('SKIPPED end to end: place-rooms.js has no exit X in beast_den yet');
        return;
    }
    assert.equal(den.exits.X.to, 'beast_den_b');
    assert.equal(den.exits.X.at, 'a');
    assert.equal(Object.keys(den.exits)[0], 'D', 'the out exit stays the first key');
    const sim = hero();
    sim.player.hp = 99;
    Interior.placeState(sim, 'beast_den').cleared.beast_den = true; // an empty first room: nothing disturbs the walk
    const it = enterAt(sim, 'beast_den', null);
    assert.equal(it.roomId, 'beast_den');
    const x = markerCell('beast_den', 'x');
    assert.ok(walk(sim, it, x.cx, x.cy), 'walk to the arrival marker x');
    irun(sim, it, 1.5, { left: true }, () => it.roomId === 'beast_den_b' && !it.leaving);
    assert.equal(it.roomId, 'beast_den_b', 'the exit X leads into the brood chamber');
    const a = markerCell('beast_den_b', 'a');
    assert.deepEqual(cellOfHero(it), { cx: a.cx, cy: a.cy }, 'the hero arrives on a');
    assert.ok(it.mobs.some((m) => m.kind === 'brood'), 'the guardian sleeps in its chamber');
    assert.ok(!it.mobs.filter((m) => m.boss)[0].aggro);
    irun(sim, it, 1.5, { down: true }, () => it.roomId === 'beast_den' && !it.leaving);
    assert.equal(it.roomId, 'beast_den', 'the exit B leads back');
    assert.deepEqual(cellOfHero(it), { cx: x.cx, cy: x.cy }, 'the hero arrives on x');
    // pots and the cracked wall of the brood chamber, with real swings
    Interior.enterRoom(sim, it, 'beast_den_b', 'a');
    const crack = thingsOf(it, 'crack')[0];
    const pot = thingsOf(it, 'pot')[0];
    walkNear(sim, it, crack.cx, crack.cy);
    it.hero.ax = 0;
    it.hero.ay = -1;
    irun(sim, it, 1.2, {}, () => false);
    const swings = [];
    for (let i = 0; i < 4 && !crack.broken; i++) {
        swings.push(...istep(sim, it, { attackPressed: true }));
        irun(sim, it, 0.6, {});
    }
    assert.ok(crack.broken, 'two swings break the cracked wall');
    assert.ok(it.room.solid[crack.cy * it.room.cols + crack.cx] === 0);
    assert.ok(walk(sim, it, 3, 2), 'the heart behind it can be fetched');
    assert.ok(pot && !pot.broken);
});

// ---- 8. the observatory ----

function enterDoor(sim, placeId) {
    const door = sim.stage.doors.filter((d) => d.placeId === placeId)[0];
    assert.ok(door, 'door for ' + placeId);
    sim.player.x = door.x;
    sim.player.y = door.y;
    sim.player.vx = 0;
    sim.player.vy = 0;
    sim.player.onGround = true;
    tick(sim, {}, 2);
    return { door, events: tick(sim, { talkPressed: 'e' }) };
}

check('observatory: sealed without obs.open, open with it', () => {
    const sim = newSim({ levelId: 12 });
    assert.equal(Places.sealed(sim, 'observatory'), true);
    const { door, events } = enterDoor(sim, 'observatory');
    assert.equal(sim.interior, null, 'the sealed door refuses');
    assert.ok(events.some((e) => e.type === 'in_locked'));
    assert.equal(sim.notice, 'runenpfad.notice.sealed');
    assert.ok(!events.some((e) => e.type === 'place_enter'));
    Npcs.setFlag(sim, 'obs.open', 1);
    assert.equal(Places.sealed(sim, 'observatory'), false);
    tick(sim, {}, 45);
    assert.ok(door.cool <= 0);
    const again = tick(sim, { talkPressed: 'e' });
    assert.ok(sim.interior && sim.interior.placeId === 'observatory' && sim.interior.roomId === 'obs_entry', 'the opened door lets the hero in');
    assert.ok(again.some((e) => e.type === 'place_enter' && e.detail.placeId === 'observatory'));
    // the whole chain of rooms is connected with arrival markers
    const seen = {};
    let room = 'obs_entry';
    for (let n = 0; n < 4 && !seen[room]; n++) {
        seen[room] = true;
        const exits = Rooms.ROOMS[room].exits;
        const fwd = Object.keys(exits).filter((m) => exits[m].to !== 'out' && OBS_ROOMS.indexOf(exits[m].to) > OBS_ROOMS.indexOf(room))[0];
        if (!fwd) break;
        room = exits[fwd].to;
    }
    assert.deepEqual(Object.keys(seen), OBS_ROOMS);
});

check('observatory: the inner gate needs the star order and all ten pages', () => {
    const sim = newSim({ levelId: 12 });
    sim.player.hp = 99;
    Npcs.setFlag(sim, 'obs.open', 1);
    const it = enterAt(sim, 'observatory', 'obs_lens', 'a');
    const gates = thingsOf(it, 'gate');
    assert.equal(gates.length, 2);
    const fl = (id) => thingsOf(it, 'brazier').filter((b) => b.bid === id)[0];
    const light = (id) => Things.use(sim, it, { ref: fl(id) }, {});
    const pages = (n) => { sim.profile.loot = Array.from({ length: n }, (v, i) => 'page_' + (i < 9 ? '0' : '') + (i + 1)); };
    pages(10);
    // a wrong order puts the whole set out
    sim.events.length = 0;
    light('crown');
    assert.ok(names(sim.events, 'in_puzzle_fail').length === 1 && thingsOf(it, 'brazier').every((b) => !b.lit), 'a wrong flame puts them all out');
    pages(9);
    ['boar', 'crown', 'eye', 'wyrm', 'gear'].forEach(light);
    irun(sim, it, 1, {});
    assert.ok(thingsOf(it, 'brazier').every((b) => b.lit), 'the five flames burn');
    assert.ok(gates.every((g) => !g.open), 'nine pages are not enough');
    sim.notice = '';
    Things.use(sim, it, { ref: gates[0] }, {});
    assert.equal(sim.notice, 'runenpfad.notice.need_pages');
    pages(10);
    irun(sim, it, 1, {});
    assert.ok(gates.every((g) => g.open), 'ten pages open the inner gate');
    // the correct order alone is not enough with the right pages missing: a fresh room, order wrong
    const sim2 = newSim({ levelId: 12 });
    const it2 = enterAt(sim2, 'observatory', 'obs_lens', 'a');
    sim2.profile.loot = Array.from({ length: 10 }, (v, i) => 'page_' + (i < 9 ? '0' : '') + (i + 1));
    ['boar', 'eye', 'crown', 'wyrm', 'gear'].forEach((id) => Things.use(sim2, it2, { ref: thingsOf(it2, 'brazier').filter((b) => b.bid === id)[0] }, {}));
    irun(sim2, it2, 1, {});
    assert.ok(thingsOf(it2, 'gate').every((g) => !g.open), 'ten pages but a wrong order keep the gate shut');
});

// ---- 9. the campaign ----

const WIDTHS = {
    1: 3360, 2: 4752, 3: 7200, 4: 6576, 5: 3456, 6: 6576, 7: 6048, 8: 6432, 9: 7392, 10: 3504, 11: 6288, 12: 6240, 13: 6384, 14: 6960, 15: 3456, 16: 5904, 17: 7536,
    18: 6768, 19: 7392, 20: 4080, 21: 6576, 22: 7536, 23: 6432, 24: 7728, 25: 4272
};

check('level 12 builds the observatory door right after crumble_chasm; every other level keeps its width', () => {
    const zones = Levels.CAMPAIGN_LEVELS.filter((l) => l.id === 12)[0].zones;
    const at = zones.indexOf('crumble_chasm');
    assert.ok(at >= 0 && zones[at + 1] === 'place:observatory', 'the zone list: ' + zones.join(','));
    for (let id = 1; id <= 25; id++) {
        const stage = Levels.buildStage(id);
        const door = stage.doors.filter((d) => d.placeId === 'observatory');
        if (id === 12) {
            assert.equal(stage.width, WIDTHS[12] + Places.ZONE_TILES * Levels.TILE, 'level 12 grows by exactly one door zone');
            assert.equal(door.length, 1);
            assert.equal(stage.doors.length, 1);
            assert.equal(door[0].facade.clip, 'fac-observatory.f0');
            assert.equal(door[0].kind, 'house');
            assert.equal(door[0].w, 56);
        } else {
            assert.equal(stage.width, WIDTHS[id], 'level ' + id + ' keeps its width');
            assert.equal(door.length, 0, 'level ' + id + ' has no observatory door');
        }
    }
});

console.log('runenpfad rooms: ' + passed + ' checks passed');
