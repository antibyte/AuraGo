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

// Same order as module-loader.js; modules that do not exist yet are skipped.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-things', 'interior', 'places', 'folk', 'folk-story',
    'village-rooms', 'place-rooms-deep', 'village', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Sim = sandbox.RunenpfadSim;
const Physics = sandbox.RunenpfadPhysics;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Interior = sandbox.RunenpfadInterior;
const Places = sandbox.RunenpfadPlaces;
const Things = sandbox.RunenpfadInteriorThings;
assert.ok(Sim && Physics && Rooms && Mobs && Interior && Places && Things, 'modules loaded');

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
    } finally {
        delete Rooms.ROOMS.t_room;
        delete Rooms.ROOMS.t_room2;
        delete Rooms.PLACES.t_place;
    }
}

function newSim(opts) {
    const sim = Sim.createSimulation(opts || { levelId: 3 });
    for (let i = 0; i < 30; i++) Sim.stepSimulation(sim, STEP * 1.000001, {});
    return sim;
}

// A twelve by eight test room. `rows` replaces the interior rows 1..6 (the spawn row 6 keeps S unless replaced).
const WALL = '############';
const OPEN = '#..........#';
function roomRows(over) {
    const rows = [WALL, OPEN, OPEN, OPEN, OPEN, OPEN, '#.....S....#', '#####D######'];
    Object.keys(over || {}).forEach((k) => { rows[Number(k)] = over[k]; });
    return rows;
}

function start(rows, things, extra, simOpts) {
    Rooms.PLACES.t_place = { id: 't_place', nameKey: 'x', kind: 'house', entry: 't_room', facade: { clip: 'fac-village-b.f1', h: 200, dx: 0 }, rooms: ['t_room'], world: 0 };
    Rooms.ROOMS.t_room = Object.assign({
        theme: 'wood', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: { to: 'out' } }, map: rows, things: things || []
    }, extra || {});
    const sim = newSim(simOpts);
    const it = Interior.create(sim, 't_place', null);
    sim.interior = it;
    return { sim, it };
}

function put(it, cx, cy, dy) {
    it.hero.x = (cx + 0.5) * 48;
    it.hero.y = (cy + 0.5) * 48 + 7 + (dy || 0);
    it.hero.vx = 0;
    it.hero.vy = 0;
}

// Runs the interior for `secs` seconds; returns the events with the tick index.
function run(sim, it, secs, input, each) {
    const out = [];
    const n = Math.round(secs / DT);
    for (let i = 0; i < n; i++) {
        sim.events.length = 0;
        const inp = typeof input === 'function' ? input(i) : Object.assign({}, input || {});
        Interior.step(sim, it, inp, DT);
        sim.events.forEach((e) => out.push({ type: e.type, detail: e.detail, i }));
        if (each && each(i) === false) break;
    }
    return out;
}

function types(evs, t) { return evs.filter((e) => e.type === t); }
function thing(it, id) { return it.things.filter((t) => t.id === id)[0]; }
function solidAt(it, t) { return it.room.solid[t.cy * it.room.cols + t.cx]; }
function wallAt(it, t) { return it.room.walls[t.cy * it.room.cols + t.cx]; }

// Stands the hero below a thing and presses the interact key for one tick.
function interact(sim, it, t) {
    put(it, t.cx, t.cy, 36);
    return run(sim, it, DT, { talkPressed: 'e' });
}

// One swing at a thing from the cell below (aim up). Waits for the swing cooldown first.
function swingUp(sim, it, t) {
    put(it, t.cx, t.cy, 30);
    it.hero.ax = 0;
    it.hero.ay = -1;
    const evs = [];
    for (let i = 0; i < 120 && it.hero.attackCool > 0; i++) evs.push(...run(sim, it, DT, {}));
    evs.push(...run(sim, it, DT, { attackPressed: true }));
    return evs;
}

// ---- breakables ----
check('a pot breaks with one swing and drops by the seed', () => {
    const a = start(roomRows({ 3: '#....P.....#' }), [{ at: 'P', pot: 'pot' }]);
    const b = newSim();
    const roll = Physics.rand(b);
    const want = roll < 0.45 ? 'coin' : (roll < 0.55 ? 'heart' : null);
    const pot = thing(a.it, 'pot_0');
    assert.equal(solidAt(a.it, pot), 1, 'a pot is solid');
    const evs = swingUp(a.sim, a.it, pot);
    assert.ok(pot.broken && solidAt(a.it, pot) === 0, 'broken and walkable');
    assert.equal(types(evs, 'in_break').length, 1);
    assert.equal(types(evs, 'in_break')[0].detail.kind, 'pot');
    const dropped = a.it.pickups.filter((p) => p.x === pot.x && p.y === pot.y)[0];
    assert.equal(dropped ? dropped.kind : null, want, 'drop follows the first roll of the seed');
    // explicit drops need no roll
    const c = start(roomRows({ 3: '#....P.....#' }), [{ at: 'P', pot: 'crate', drop: 'key' }]);
    const state = c.sim.rngState;
    swingUp(c.sim, c.it, thing(c.it, 'pot_0'));
    assert.equal(c.sim.rngState, state, 'explicit drop rolls nothing');
    assert.ok(c.it.pickups.some((p) => p.kind === 'key' && p.key === 'pk_t_room_pot_0'));
    // the pot stays broken after leaving and entering again, the key lies there until taken
    Interior.enterRoom(c.sim, c.it, 't_room', 'S');
    assert.ok(thing(c.it, 'pot_0').broken);
    assert.equal(solidAt(c.it, thing(c.it, 'pot_0')), 0);
    assert.ok(c.it.pickups.some((p) => p.kind === 'key'), 'key still lies there');
});

// ---- blocks ----
check('a block is pushed one cell and stops at a wall', () => {
    const { sim, it } = start(roomRows({ 3: '#...B..#...#' }), [{ at: 'B', block: 'stone' }]);
    const b = thing(it, 'block_0');
    put(it, 2, 3);
    let evs = run(sim, it, 2, { right: true }, () => !b.moving);
    assert.ok(b.moving, 'a push starts after walking against it');
    assert.ok(types(evs, 'in_push').length === 1);
    assert.equal(solidAt(it, { cx: 4, cy: 3 }), 1, 'source stays solid while moving');
    assert.equal(solidAt(it, { cx: 5, cy: 3 }), 1, 'target is solid while moving');
    run(sim, it, 0.25, {});
    assert.equal(b.cx, 5);
    assert.equal(b.moving, null);
    assert.equal(solidAt(it, { cx: 4, cy: 3 }), 0);
    evs = run(sim, it, 4, { right: true });
    assert.equal(b.cx, 6, 'stops against the wall at 7');
    assert.equal(types(evs, 'in_push').length, 1);
});

check('in a slippery room a block and the hero slide until blocked', () => {
    const rows = roomRows({ 3: '#...B..#...#', 4: '#,,,,,,,,,,#', 5: OPEN });
    const { sim, it } = start(rows, [{ at: 'B', block: 'ice' }], { slippery: true });
    const b = thing(it, 'block_0');
    put(it, 2, 3);
    const evs = run(sim, it, 2, { right: true }, () => !b.moving);
    assert.ok(b.moving);
    run(sim, it, 1, {});
    assert.equal(b.cx, 6, 'the block slid to the wall');
    assert.ok(types(evs, 'in_push').length === 1, 'one push, then it slid');
    // hero: one tick of input on ice keeps the hero going until the wall
    put(it, 2, 5);
    const slide = run(sim, it, DT * 2, { right: true });
    run(sim, it, 4, {});
    assert.ok(it.hero.x > 11 * 48 - 14, 'slid to the east wall: ' + it.hero.x);
    assert.ok(types(slide, 'in_slide').length === 1);
    // on rough ground ',' the hero stops at once
    put(it, 2, 4);
    run(sim, it, DT * 2, { right: true });
    const x = it.hero.x;
    run(sim, it, 1, {});
    assert.ok(Math.abs(it.hero.x - x) < 1, 'no slide on rough ground');
});

check('holding against a wall on ice emits no further in_slide', () => {
    const { sim, it } = start(roomRows({ 3: '#.....#....#' }), [], { slippery: true });
    put(it, 2, 3);
    const first = run(sim, it, 1, { right: true });
    assert.equal(types(first, 'in_slide').length, 1, 'one event for the slide that started');
    const held = run(sim, it, 1, { right: true });
    assert.equal(types(held, 'in_slide').length, 0, 'pushing against the wall is silent');
});

check('a block does not move onto a mob or a pickup', () => {
    const rows = roomRows({ 3: '#...Bm.....#', 5: '#...B......#' });
    const { sim, it } = start(rows, [{ at: 'B', block: 'stone' }, { at: 'm', mob: 'rat' }]);
    const mob = it.mobs[0];
    mob.state = 'hurt';
    mob.t = -1000;
    const b = it.things.filter((t) => t.cy === 3)[0];
    put(it, 2, 3);
    run(sim, it, 1.5, { right: true });
    assert.equal(b.cx, 4, 'blocked by the mob');
    mob.state = 'dead';
    mob.deadT = 0;
    const c = it.things.filter((t) => t.cy === 5)[0];
    it.pickups.push({ x: 5.5 * 48, y: 5.5 * 48 + 7, kind: 'coin', taken: false });
    put(it, 2, 5);
    run(sim, it, 1.5, { right: true }, () => it.hero.x < 3.5 * 48);
    assert.equal(c.cx, 4, 'blocked by the pickup');
});

// ---- plates, gates, levers ----
check('a plate opens a gate while pressed and a latch keeps it', () => {
    const rows = roomRows({ 2: '#.P...G..L.#' });
    const { sim, it } = start(rows, [{ at: 'P', plate: 'a' }, { at: 'G', gate: 'g', open: { on: ['a'] } }, { at: 'L', plate: 'b', latch: true }]);
    const gate = thing(it, 'gate_0');
    const plate = thing(it, 'plate_0');
    assert.equal(gate.open, false);
    assert.equal(solidAt(it, gate), 1);
    assert.equal(wallAt(it, gate), 1, 'closed gates also block flying mobs');
    put(it, 2, 2);
    let evs = run(sim, it, 0.5, {});
    assert.ok(plate.on && gate.open);
    assert.equal(solidAt(it, gate), 0);
    assert.equal(wallAt(it, gate), 0);
    assert.ok(types(evs, 'in_plate')[0].detail.on === true && types(evs, 'in_gate')[0].detail.open === true);
    put(it, 5, 4);
    evs = run(sim, it, 0.5, {});
    assert.ok(!plate.on && !gate.open && solidAt(it, gate) === 1, 'closes when released');
    // latch plate stays on
    const latch = thing(it, 'plate_1');
    put(it, latch.cx, latch.cy);
    run(sim, it, 0.3, {});
    put(it, 5, 4);
    run(sim, it, 0.3, {});
    assert.ok(latch.on, 'latched');
});

check('a closed gate stops a flying mob', () => {
    const rows = roomRows({ 2: '#....G.....#' });
    const { it } = start(rows, [{ at: 'G', gate: 'g', open: { flag: 'never.set' } }]);
    const bat = Mobs.create('bat', 'b0', 4.5 * 48, 2.5 * 48 + 7, 0);
    const d = Mobs.DEFS.bat;
    for (let i = 0; i < 60; i++) Mobs.slide(it.room, it.room.walls, bat, 3, 0, d.r * 0.8, d.r * 1.1);
    assert.ok(bat.x < 5 * 48, 'the closed gate holds the bat: ' + bat.x);
});

check('levers flip each other and open a gate', () => {
    const rows = roomRows({ 2: '#.A....B.G.#' });
    const { sim, it } = start(rows, [{ at: 'A', lever: 'l1', flips: ['l2'] }, { at: 'B', lever: 'l2' }, { at: 'G', gate: 'g', open: { on: ['l1', 'l2'] } }]);
    const l1 = thing(it, 'lever_0');
    const l2 = thing(it, 'lever_1');
    const gate = thing(it, 'gate_0');
    assert.ok(!l1.on && !l2.on && !gate.open);
    assert.equal(it.puzzle, 'open');
    let evs = interact(sim, it, l1);
    assert.equal(it.prompt && it.prompt.kind, 'pull');
    assert.ok(l1.on && l2.on, 'flips toggles the other lever');
    assert.equal(types(evs, 'in_lever')[0].detail.on, true);
    run(sim, it, 0.1, {});
    assert.ok(gate.open);
    assert.equal(it.puzzle, 'solved');
    evs = interact(sim, it, l1);
    assert.ok(!l1.on && !l2.on);
    run(sim, it, 0.1, {});
    assert.ok(!gate.open, 'a plain gate closes again');
    // a solved room keeps its stay gates open on the next visit
    const second = start(roomRows({ 2: '#.A....B.G.#' }), [{ at: 'A', lever: 'l1', flips: ['l2'] }, { at: 'B', lever: 'l2' }, { at: 'G', gate: 'g', stay: true, open: { on: ['l1'] } }]);
    interact(second.sim, second.it, thing(second.it, 'lever_0'));
    run(second.sim, second.it, 0.1, {});
    assert.ok(thing(second.it, 'gate_0').open);
    Interior.enterRoom(second.sim, second.it, 't_room', 'S');
    assert.ok(thing(second.it, 'gate_0').open, 'stay gate open after re-entering a solved room');
    assert.ok(!thing(second.it, 'lever_0').on, 'levers are rebuilt from the definition');
});

// ---- traps ----
check('spikes hurt only while up and warn 0.5 s before', () => {
    const rows = roomRows({ 3: '#...X......#' });
    const { sim, it } = start(rows, [{ at: 'X', spikes: { period: 3, up: 1, offset: 0 } }]);
    const s = thing(it, 'spikes_0');
    put(it, s.cx, s.cy);
    const hp = sim.player.hp;
    const evs = run(sim, it, 1.4, {});
    assert.equal(s.phase, 'down');
    assert.equal(sim.player.hp, hp, 'safe while retracted');
    const more = run(sim, it, 1.2, {});
    const warn = types(evs.concat(more), 'in_trap_warn')[0];
    const up = types(evs.concat(more), 'in_trap')[0];
    assert.ok(warn && up && warn.detail.kind === 'spikes');
    assert.ok(Math.abs((up.i - warn.i) * DT - 0.5) < 0.05, 'the tell lasts 0.5 s: ' + (up.i - warn.i) * DT);
    assert.equal(sim.player.hp, hp - 1, 'hurt while up');
    assert.ok(types(more, 'in_hit').length === 1);
});

check('the hero crosses a spike cell untouched by timing', () => {
    const rows = roomRows({ 3: '#...X......#' });
    const { sim, it } = start(rows, [{ at: 'X', spikes: { period: 3, up: 1, offset: 0 } }]);
    put(it, 2, 3);
    const hp = sim.player.hp;
    run(sim, it, 1.2, { right: true });
    assert.ok(it.hero.x > 5 * 48, 'crossed the cell: ' + it.hero.x);
    assert.equal(sim.player.hp, hp);
});

check('a dart shot flies along its row and dies at a wall', () => {
    const rows = roomRows({ 2: '#A.........#' });
    const { sim, it } = start(rows, [{ at: 'A', dart: 'right', every: 2, offset: 0 }]);
    put(it, 5, 5);
    const hp = sim.player.hp;
    let sawShot = false;
    const evs = run(sim, it, 6, {}, () => { if (it.shots.length) sawShot = true; });
    assert.ok(sawShot, 'a shot was fired');
    assert.ok(types(evs, 'in_trap_warn').some((e) => e.detail.kind === 'dart'));
    assert.ok(types(evs, 'in_trap').some((e) => e.detail.kind === 'dart'));
    run(sim, it, 4, {});
    assert.equal(it.shots.filter((s) => s.kind === 'dart' && s.x > 11 * 48).length, 0, 'no shot passes the wall');
    assert.equal(sim.player.hp, hp);
    const shot = it.shots[0];
    assert.ok(!shot || shot.x < 11 * 48);
});

// ---- braziers ----
check('brazier order: success opens the gate, a wrong brazier puts the set out', () => {
    const rows = roomRows({ 2: '#.A..B...G.#' });
    const { sim, it } = start(rows, [{ at: 'A', brazier: 'b1', set: 's' }, { at: 'B', brazier: 'b2', set: 's' }, { at: 'G', gate: 'g', open: { order: 's' } }], { orders: { s: ['b1', 'b2'] } });
    const b1 = thing(it, 'brazier_0');
    const b2 = thing(it, 'brazier_1');
    const gate = thing(it, 'gate_0');
    let evs = interact(sim, it, b2);
    assert.equal(types(evs, 'in_puzzle_fail').length, 1);
    assert.ok(!b1.lit && !b2.lit);
    evs = interact(sim, it, b1);
    assert.equal(types(evs, 'in_brazier')[0].detail.step, 0);
    assert.ok(b1.lit && !gate.open);
    evs = interact(sim, it, b2);
    assert.equal(types(evs, 'in_brazier')[0].detail.step, 1);
    const more = run(sim, it, 0.1, {});
    assert.ok(gate.open);
    assert.equal(types(more, 'in_puzzle_done').length, 1);
    const lights = Things.lights(it);
    assert.equal(lights.filter((l) => l.flicker).length, 2, 'two lit braziers give light');
    // a lit brazier cannot be lit twice, a swing lights one too
    const sw = start(roomRows({ 2: '#.A.........#' }), [{ at: 'A', brazier: 'x' }]);
    swingUp(sw.sim, sw.it, thing(sw.it, 'brazier_0'));
    assert.ok(thing(sw.it, 'brazier_0').lit);
});

check('a timed brazier goes out unless its set is complete', () => {
    const rows = roomRows({ 2: '#.A..B...C.#' });
    const { sim, it } = start(rows, [{ at: 'A', brazier: 'u1', set: 'u', timed: 1 }, { at: 'B', brazier: 'u2', set: 'u', timed: 1 }, { at: 'C', brazier: 'solo', timed: 1 }]);
    const [u1, u2, solo] = [thing(it, 'brazier_0'), thing(it, 'brazier_1'), thing(it, 'brazier_2')];
    interact(sim, it, solo);
    assert.ok(solo.lit);
    const evs = run(sim, it, 1.3, {});
    assert.ok(!solo.lit);
    assert.ok(types(evs, 'in_brazier').some((e) => e.detail.lit === false));
    interact(sim, it, u1);
    interact(sim, it, u2);
    run(sim, it, 2.5, {});
    assert.ok(u1.lit && u2.lit, 'a complete set stays lit');
});

// ---- cracks, keys, pages ----
check('a crack needs two hits', () => {
    const { sim, it } = start(roomRows({ 2: '#.K.........#' }), [{ at: 'K', crack: true }]);
    const k = thing(it, 'crack_0');
    assert.equal(solidAt(it, k), 1);
    let evs = swingUp(sim, it, k);
    assert.ok(!k.broken && k.hp === 1 && solidAt(it, k) === 1);
    assert.equal(types(evs, 'in_crack')[0].detail.broken, false);
    evs = swingUp(sim, it, k);
    assert.ok(k.broken && solidAt(it, k) === 0);
    assert.equal(types(evs, 'in_crack')[0].detail.broken, true);
    Interior.enterRoom(sim, it, 't_room', 'S');
    assert.ok(thing(it, 'crack_0').broken, 'stays broken');
});

check('a key gate consumes the key and a key survives leaving the place', () => {
    const rows = roomRows({ 2: '#.G....Y...#' });
    const { sim, it } = start(rows, [{ at: 'G', gate: 'door', open: { key: true } }, { at: 'Y', pickup: 'key' }]);
    const gate = thing(it, 'gate_0');
    interact(sim, it, gate);
    assert.equal(sim.notice, 'runenpfad.notice.smallkey_need');
    assert.ok(!gate.open);
    put(it, 7, 2);
    const evs = run(sim, it, 0.1, {});
    assert.equal(types(evs, 'in_key').length, 1);
    assert.equal(Interior.placeState(sim, 't_place').keys, 1);
    // leave the place and come back: the key is still counted for this stage run
    sim.interior = null;
    const again = Interior.create(sim, 't_place', null);
    sim.interior = again;
    assert.equal(Interior.placeState(sim, 't_place').keys, 1);
    const g2 = thing(again, 'gate_0');
    interact(sim, again, g2);
    assert.equal(again.prompt && again.prompt.kind, 'unlock');
    run(sim, again, DT, {});
    assert.ok(g2.open, 'opened with the key');
    assert.equal(Interior.placeState(sim, 't_place').keys, 0);
    Interior.enterRoom(sim, again, 't_room', 'S');
    assert.ok(thing(again, 'gate_0').open, 'open for good');
});

check('lecterns and signs use the dialogue and the profile', () => {
    const rows = roomRows({ 2: '#.L..N......#' });
    const { sim, it } = start(rows, [{ at: 'L', page: 3 }, { at: 'N', sign: 'tomb_mural' }]);
    const page = thing(it, 'page_0');
    const calls = [];
    sandbox.RunenpfadDialogue.openVirtual = (s, npc, node) => { calls.push(npc + ':' + node); };
    try {
        assert.equal(page.have, false);
        assert.equal(Things.lights(it).length, 1, 'a lectern with a missing page glows');
        interact(sim, it, page);
        interact(sim, it, thing(it, 'sign_0'));
        assert.deepEqual(calls, ['journal:p3a', 'lore:tomb_mural']);
        sim.profile.loot.push('page_03');
        run(sim, it, 0.1, {});
        assert.equal(page.have, true);
        assert.equal(Things.lights(it).length, 0);
    } finally {
        delete sandbox.RunenpfadDialogue.openVirtual;
    }
    sim.mode = 'endless';
    Interior.enterRoom(sim, it, 't_room', 'S');
    assert.equal(it.things.filter((t) => t.kind === 'page').length, 0, 'no pages in endless mode');
});

// ---- determinism and validator ----
check('the same inputs give the same state', () => {
    const play = () => {
        const rows = roomRows({ 2: '#.A..B...K.#', 3: '#...P..X...#', 4: '#...B......#' });
        const { sim, it } = start(rows, [{ at: 'A', lever: 'l' }, { at: 'B', block: 'stone' }, { at: 'K', crack: true }, { at: 'P', pot: 'barrel' }, { at: 'X', spikes: { period: 3, up: 1, offset: 0.4 } }]);
        const evs = run(sim, it, 4, (i) => ({ right: i % 90 < 60, down: i % 50 < 10, attackPressed: i % 70 === 0, talkPressed: i % 120 === 5 ? 'e' : null }));
        return JSON.stringify({ evs, things: it.things, hero: it.hero, pickups: it.pickups, rng: sim.rngState });
    };
    assert.equal(play(), play());
});

check('the validator rejects a broken room for each new rule', () => {
    const base = [
        '##############',
        '#...........##',
        '#.A.......G..#',
        '#............#',
        '#............#',
        '#............#',
        '#.....S......#',
        '######D#######'
    ];
    const good = [{ at: 'A', lever: 'l1' }, { at: 'G', gate: 'g', open: { on: ['l1'] } }];
    const problems = (rows, things, extra) => {
        Rooms.PLACES.t_place = { id: 't_place', nameKey: 'x', kind: 'house', entry: 't_room', facade: { clip: 'fac-village-b.f1', h: 200, dx: 0 }, rooms: ['t_room'], world: 0 };
        Rooms.ROOMS.t_room = Object.assign({ theme: 'wood', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: { to: 'out' } }, map: rows, things: things }, extra || {});
        return Rooms.validate().join('\n');
    };
    const own = (text) => text.split(/\r?\n/).filter((l) => /t_room|t_place/.test(l)).join(' | ');
    const baseline = Rooms.validate().length;
    assert.equal(own(problems(base, good)), '', 'the good room validates');
    const row = (i, s) => base.map((r, k) => (k === i ? s : r));
    const cases = [
        [base, [{ at: 'A', lever: 'l1' }, { at: 'G', gate: 'g', open: { on: ['nope'] } }], null, /unknown id nope/],
        [base, [{ at: 'A', brazier: 'b1', set: 's' }], { orders: { s: ['b1', 'zz'] } }, /unknown brazier zz/],
        [base, good, { orders: { q: ['b1'] } }, /without braziers/],
        [row(1, '#B..........##'), [{ at: 'B', block: 'stone' }], null, /cannot be pushed/],
        [base, [{ at: 'D', pot: 'pot' }], null, /stands on an exit/],
        [base, [{ at: 'A', page: 11 }], null, /bad page number/],
        [base, [{ at: 'A', page: 2 }, { at: 'G', page: 2 }], null, /already placed/],
        [base, [{ at: 'A', spikes: { period: 2, up: 1.5, offset: 0 } }], null, /spikes need/],
        [base, [{ at: 'A', sign: 'Bad Node' }], null, /bad sign node/],
        [base, [{ at: 'A', mob: 'zzz' }], null, /unknown mob zzz/],
        [base, [{ at: 'A', lever: 'l1' }, { at: 'G', gate: 'g', open: { clear: true } }], null, /needs clear/],
        [base, [{ at: 'A', mob: 'rat' }], { guardian: 'weaver' }, /has no mob thing|unknown guardian/],
        [base.map((r, k) => ({ 1: '#....#......##', 2: '#.A.#P#......#', 3: '#....#.......#' }[k] || r)), [{ at: 'A', lever: 'l1' }, { at: 'P', plate: 'p' }], null, /plate at \d+,\d+ is unreachable/]
    ];
    cases.forEach(([rows, things, extra, re]) => {
        const text = own(problems(rows, things, extra));
        assert.ok(re.test(text), String(re) + ' missing in:\n' + text);
    });
    assert.equal(Rooms.PROP_DEFS['tdp-mech.p0'].solid, true);
    assert.equal(Object.keys(Rooms.PROP_DEFS).length, 112);
    delete Rooms.ROOMS.t_room;
    delete Rooms.PLACES.t_place;
    assert.equal(Rooms.validate().length, baseline, 'test rooms leave no trace');
});

check('a sealed place refuses the door until its flag is set', () => {
    const sim = newSim({ levelId: 3 });
    Rooms.PLACES.beast_den.requires = { flag: 'obs.open' };
    try {
        assert.equal(Places.sealed(sim, 'beast_den'), true);
        const door = sim.stage.doors.filter((d) => d.placeId === 'beast_den')[0];
        sim.player.x = door.x;
        sim.player.y = door.y;
        sim.player.onGround = true;
        const input = { talkPressed: 'e' };
        sim.events.length = 0;
        const res = Places.tryEnter(sim, input);
        assert.equal(res, true);
        assert.ok(!sim.interior, 'still outside');
        assert.equal(sim.notice, 'runenpfad.notice.sealed');
        assert.ok(sim.events.some((e) => e.type === 'in_locked'));
        sandbox.RunenpfadNpcs.setFlag(sim, 'obs.open', 1);
        assert.equal(Places.sealed(sim, 'beast_den'), false);
    } finally {
        delete Rooms.PLACES.beast_den.requires;
    }
});

console.log(`Runenpfad interior things checks passed: ${passed} groups (pots, blocks, ice, plates and gates, flying mob gate, levers, spikes, darts, braziers, timed braziers, cracks, keys, lecterns, determinism, validator, sealed door).`);
