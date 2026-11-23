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

// Same order as module-loader.js; interior-bosses sits right after interior-mobs.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things', 'interior', 'places', 'folk', 'folk-story',
    'journal-story', 'village-rooms', 'place-rooms-deep', 'village', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Sim = sandbox.RunenpfadSim;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Bosses = sandbox.RunenpfadInteriorBosses;
const Interior = sandbox.RunenpfadInterior;
assert.ok(Sim && Rooms && Mobs && Bosses && Interior, 'modules loaded');

const STEP = Sim.STEP;
const GUARDIANS = ['brood', 'weaver', 'tombward', 'frostheart', 'boilerward', 'counter'];
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

// Test rooms: a wide hall with one guardian at B, and a hall with an exit cell X between guardian and hero.
function addRooms() {
    Rooms.PLACES.bt_place = { id: 'bt_place', nameKey: 'x', kind: 'cave', entry: 'bt_room', facade: { clip: 'fac-village-b.f1', h: 200, dx: 0 }, rooms: ['bt_room', 'bt_exit'], world: 0 };
    Rooms.ROOMS.bt_room = {
        theme: 'cave', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cave', spawn: 'S', exits: { D: { to: 'out' } },
        map: ['####################', '#..................#', '#..................#', '#..................#', '#........B.........#', '#..................#', '#..................#', '#..................#',
            '#..S...............#', '#..................#', '#########D##########'],
        things: [{ at: 'B', mob: 'brood' }]
    };
    Rooms.ROOMS.bt_exit = {
        theme: 'cave', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cave', spawn: 'S', exits: { X: { to: 'out' } },
        map: ['###############', '#.............#', '#.............#', '#.............#', '#.....B.X.S...#', '#.............#', '#.............#', '###############'],
        things: [{ at: 'B', mob: 'brood' }]
    };
}
function removeRooms() {
    delete Rooms.PLACES.bt_place;
    delete Rooms.ROOMS.bt_room;
    delete Rooms.ROOMS.bt_exit;
}

function arena(kind, hero, roomId, seed) {
    Rooms.ROOMS[roomId || 'bt_room'].things = [{ at: 'B', mob: kind }];
    const sim = Sim.createSimulation({ levelId: 3, hero: hero || 'knight', seed: seed || 77 });
    tick(sim, {}, 30);
    sim.interior = Interior.create(sim, 'bt_place', null);
    if (roomId && roomId !== 'bt_room') Interior.enterRoom(sim, sim.interior, roomId, null);
    return sim;
}
const bossOf = (sim) => sim.interior.mobs.filter((m) => m.boss)[0];

function inside(it, m) {
    const cx = Math.floor(m.x / 48);
    const cy = Math.floor((m.y - 7) / 48);
    return cx >= 1 && cy >= 1 && cx < it.room.cols - 1 && cy < it.room.rows - 1 && !it.room.exits.some((e) => e.cx === cx && e.cy === cy);
}

// Scripted bot: retreat diagonally during tells (straight along the aim for the gap rings), strike otherwise.
function bot(sim, boss) {
    const it = sim.interior;
    const h = it.hero;
    const input = {};
    const reach = sim.heroId === 'knight' ? 52 : 46;
    if (sim.player.hp <= 2 && sim.player.potions > 0) input.healPressed = true;
    const bx = boss.x - h.x;
    const by = boss.y - h.y;
    const dist = Math.hypot(bx, by) || 1;
    let dir;
    if (boss.aggro && (boss.state === 'tell' || boss.state === 'attack')) {
        const ax = -bx / dist;
        const ay = -by / dist;
        if (boss.move === 'ring' || boss.move === 'clock') {
            const off = (h.x - boss.x) * -boss.ay + (h.y - boss.y) * boss.ax;
            dir = { x: boss.ax + boss.ay * off * 0.1, y: boss.ay - boss.ax * off * 0.1 };
        } else {
            const rot = (sg) => ({ x: ax * Math.cos(0.8 * sg) - ay * Math.sin(0.8 * sg), y: ax * Math.sin(0.8 * sg) + ay * Math.cos(0.8 * sg) });
            const cxr = it.room.w / 2 - h.x;
            const cyr = it.room.h / 2 - h.y;
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
        // strike the nearest add first, otherwise the guardian; keep a little closer than contact so the swing faces it
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
    // dodge the nearest incoming shot sideways
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

function fight(kind, hero, seed) {
    const sim = arena(kind, hero, null, seed);
    const it = sim.interior;
    const log = { events: [], tells: [], minTell: 99, hurts: [], moves: new Set(), outside: false, hp0: sim.player.hp };
    let tellAt = null;
    for (let i = 0; i < 60 * 150 && sim.interior; i++) {
        const boss = bossOf(sim);
        if (!boss) break;
        tick(sim, bot(sim, boss)).forEach((e) => {
            if (e.type === 'in_hit' && e.detail.target === 'hero') log.hurts.push(e.detail.kind + '@' + (boss.move || boss.state) + ':' + it.t.toFixed(1));
            log.events.push(e.type + (e.detail && e.detail.pattern ? ':' + e.detail.pattern : ''));
            if (e.type === 'in_boss_tell') tellAt = { t: it.t, pattern: e.detail.pattern };
            if (process.env.RP_DEBUG && e.type === 'in_hit' && e.detail.target === 'hero') console.log('HURT', boss.move, boss.state, 'hero', it.hero.x | 0, it.hero.y | 0, 'boss', boss.x | 0, boss.y | 0, JSON.stringify(boss.cross));
            if (e.type === 'in_boss_attack') {
                assert.ok(tellAt && tellAt.pattern === e.detail.pattern, kind + ' attack without its tell');
                log.minTell = Math.min(log.minTell, it.t - tellAt.t);
                log.moves.add(e.detail.move);
                assert.equal(e.detail.pattern, kind + '_' + e.detail.move);
                assert.equal(e.detail.kind, kind);
                assert.ok(e.detail.x === undefined && e.detail.y === undefined);
            }
        });
        if (!inside(it, boss)) log.outside = true;
        if (boss.state === 'dead') break;
    }
    log.sim = sim;
    log.boss = sim.interior && sim.interior.mobs.filter((m) => m.boss)[0];
    return log;
}

check('data: KINDS stays normal, guardians are listed apart', () => {
    assert.equal(Mobs.KINDS.length, 7);
    assert.deepEqual(JSON.parse(JSON.stringify(Mobs.BOSS_KINDS)), GUARDIANS);
    GUARDIANS.forEach((k) => {
        assert.equal(Mobs.DEFS[k].boss, true, k);
        assert.ok(Bosses.SPECS[k].moves.every((mv) => mv.tell >= 0.7), k + ' tells');
    });
    assert.ok(Mobs.KINDS.every((k) => !Mobs.DEFS[k].boss));
    assert.deepEqual([14, 14, 18, 14, 20, 26], GUARDIANS.map((k) => Mobs.DEFS[k].hp));
    assert.equal(Mobs.create('rat', 'r', 10, 10, 0).boss, undefined);
});

addRooms();
try {
    check('every move of every guardian is announced at least 0.6 s before it lands', () => {
        GUARDIANS.forEach((kind) => {
            const spec = Bosses.SPECS[kind];
            const all = spec.moves.concat(spec.phase ? [spec.phase] : []);
            all.forEach((mv) => {
                const saved = spec.moves;
                spec.moves = [mv];
                try {
                    const sim = arena(kind);
                    const it = sim.interior;
                    sim.player.hp = 99;
                    sim.player.maxHp = 99;
                    const boss = bossOf(sim);
                    it.hero.x = boss.x - 150;
                    it.hero.y = boss.y;
                    boss.aggro = true;
                    boss.pend = spec.phase === mv; // the phase move comes through the pending flag
                    let tellAt = null;
                    let landed = 0;
                    for (let i = 0; i < 60 * 12 && !landed; i++) {
                        tick(sim, {}).forEach((e) => {
                            if (e.type === 'in_boss_tell') tellAt = it.t;
                            if (process.env.RP_DEBUG && e.type === 'in_hit' && e.detail.target === 'hero') console.log('HURT', boss.move, boss.state, 'hero', it.hero.x | 0, it.hero.y | 0, 'boss', boss.x | 0, boss.y | 0, JSON.stringify(boss.cross));
            if (e.type === 'in_boss_attack') {
                                assert.equal(e.detail.pattern, kind + '_' + mv.id);
                                assert.ok(tellAt !== null && it.t - tellAt >= 0.6 - 1e-6, kind + '_' + mv.id + ' tell ' + (it.t - tellAt));
                                landed += 1;
                            }
                        });
                    }
                    assert.ok(landed, kind + '_' + mv.id + ' happens');
                } finally {
                    spec.moves = saved;
                }
            });
        });
    });

    check('both heroes beat every guardian from base hp with the simple bot', () => {
        GUARDIANS.filter((k) => !process.env.RP_ONLY || k === process.env.RP_ONLY).forEach((kind) => {
            ['knight', 'ranger'].forEach((hero) => {
                const log = fight(kind, hero);
                const where = kind + '/' + hero;
                const die = log.events.filter((e) => e === 'in_boss_die');
                assert.equal(die.length, 1, where + ' died once (hp left ' + (log.boss && log.boss.hp) + ', hero hp ' + log.sim.player.hp + '/' + log.hp0 + ') hurts ' + log.hurts.join(' '));
                assert.ok(log.sim.player.hp > 0, where + ' hero alive');
                assert.ok(log.events.includes('in_mob_die'), where + ' normal death event too');
                assert.equal(log.events.filter((e) => e === 'in_boss_phase').length, 1, where + ' phase once');
                assert.ok(log.minTell >= 0.6 - 1e-6, where + ' tell ' + log.minTell);
                assert.ok(!log.outside, where + ' stayed inside the room, off the exits');
                if (process.env.RP_VERBOSE) console.log(where, 'hp', log.sim.player.hp + '/' + log.hp0, 'moves', [...log.moves].join(','), 't', log.sim.interior && log.sim.interior.t.toFixed(1), 'hurts', log.hurts.join(' '));
            });
        });
    });

    check('determinism', () => {
        const a = fight('counter', 'knight', 5);
        const b = fight('counter', 'knight', 5);
        assert.equal(JSON.stringify(a.events), JSON.stringify(b.events));
        assert.equal(a.sim.player.hp, b.sim.player.hp);
        assert.equal(a.sim.rngState, b.sim.rngState);
    });

    check('a hit only flashes the guardian and never cancels a tell', () => {
        const spec = Bosses.SPECS.tombward;
        const saved = spec.moves;
        spec.moves = [saved[0]];
        try {
            const sim = arena('tombward');
            const it = sim.interior;
            sim.player.hp = 99;
            const boss = bossOf(sim);
            it.hero.x = boss.x - 70;
            it.hero.y = boss.y;
            boss.aggro = true;
            let guard = 0;
            while (boss.state !== 'tell' && guard++ < 600) tick(sim, {});
            assert.equal(boss.state, 'tell');
            const x = boss.x;
            const y = boss.y;
            const events = [];
            Mobs.hit(sim, it, boss, 1, 1, 0);
            assert.equal(boss.state, 'tell');
            assert.ok(boss.hurtT > 0);
            assert.equal(boss.x, x);
            assert.equal(boss.y, y);
            assert.equal(boss.hp, 17);
            for (let i = 0; i < 90; i++) events.push(...tick(sim, {}));
            assert.ok(events.some((e) => e.type === 'in_boss_attack'), 'attack still lands');
            assert.ok(sim.events.length >= 0);
            // half hp: one phase event, and the raise is queued for the next move
            const ev = [];
            Mobs.hit(sim, it, boss, 8, 1, 0);
            Mobs.hit(sim, it, boss, 1, 1, 0);
            ev.push(...sim.events);
            assert.equal(ev.filter((e) => e.type === 'in_boss_phase').length, 1);
            assert.equal(boss.phase, 2);
        } finally {
            spec.moves = saved;
        }
    });

    check('the web patch slows the hero', () => {
        const spec = Bosses.SPECS.weaver;
        const saved = spec.moves;
        spec.moves = [saved.filter((mv) => mv.id === 'web')[0]];
        try {
            const sim = arena('weaver');
            const it = sim.interior;
            sim.player.hp = 99;
            const boss = bossOf(sim);
            it.hero.x = boss.x - 160;
            it.hero.y = boss.y;
            boss.aggro = true;
            let slow = 0;
            for (let i = 0; i < 60 * 6; i++) {
                tick(sim, {});
                if (it.hero.slowT > slow) slow = it.hero.slowT;
            }
            assert.equal(slow, 2);
            assert.ok(boss.patches.length >= 1 && boss.patches.length <= 2);
        } finally {
            spec.moves = saved;
        }
    });

    check('adds count for the room clear, spawn on free cells and respect the cap', () => {
        const spec = Bosses.SPECS.brood;
        const saved = spec.moves;
        spec.moves = [saved.filter((mv) => mv.id === 'spawn')[0]];
        try {
            const sim = arena('brood');
            const it = sim.interior;
            sim.player.hp = 99;
            const boss = bossOf(sim);
            it.hero.x = boss.x - 150;
            it.hero.y = boss.y;
            boss.aggro = true;
            let most = 0;
            for (let i = 0; i < 60 * 14; i++) {
                tick(sim, {});
                const adds = it.mobs.filter((m) => m.owner === boss.id);
                most = Math.max(most, adds.filter((m) => m.state !== 'dead').length);
            }
            const adds = it.mobs.filter((m) => m.owner === boss.id);
            assert.equal(most, 3, 'cap of three living adds');
            adds.forEach((m) => {
                assert.match(m.id, /^m_s\d+$/);
                assert.ok(m.aggro && inside(it, m), 'add awake, off the exits');
                assert.ok(!Mobs.blocked(it.room, it.room.solid, m.x, m.y, m.r * 0.8, m.r * 1.1), 'add on a free cell');
            });
            assert.equal(new Set(adds.map((m) => m.id)).size, adds.length, 'unique ids');
            const events = [];
            Mobs.hit(sim, it, boss, 99, 1, 0);
            events.push(...tick(sim, {}, 90));
            assert.ok(!it.cleared, 'living adds keep the room open');
            it.mobs.forEach((m) => { if (m.state !== 'dead') Mobs.hit(sim, it, m, 99, 1, 0); });
            events.push(...tick(sim, {}, 3));
            assert.ok(it.cleared && events.some((e) => e.type === 'in_clear'), 'room clears with the adds');
        } finally {
            spec.moves = saved;
        }
    });

    check('the wake rule: 220 px and no closed gate in the way', () => {
        const sim = arena('weaver');
        const it = sim.interior;
        sim.player.hp = 99;
        const boss = bossOf(sim);
        const bcx = Math.floor(boss.x / 48);
        const bcy = Math.floor((boss.y - 7) / 48);
        it.hero.x = boss.x - 250;
        it.hero.y = boss.y;
        tick(sim, {}, 60);
        assert.equal(boss.aggro, false, 'too far');
        it.hero.x = boss.x - 190;
        it.things = [{ kind: 'gate', cx: bcx - 2, cy: bcy, open: false, cond: { flag: 'never.set' } }];
        tick(sim, {}, 60);
        assert.equal(boss.aggro, false, 'closed gate between');
        it.things[0].cond = {}; it.things[0].open = true;
        tick(sim, {}, 2);
        assert.equal(boss.aggro, true, 'open gate lets it wake');
    });

    check('a guardian does not walk onto an exit cell', () => {
        const sim = arena('brood', 'knight', 'bt_exit');
        const it = sim.interior;
        assert.equal(it.roomId, 'bt_exit');
        sim.player.hp = 99;
        sim.player.maxHp = 99;
        const boss = bossOf(sim);
        const ex = it.room.exits[0];
        boss.aggro = true;
        for (let i = 0; i < 60 * 8; i++) {
            it.hero.x = (ex.cx + 2.5) * 48;
            it.hero.y = (ex.cy + 0.5) * 48 + 7;
            tick(sim, {});
            const hw = boss.r * 0.8;
            const overlap = boss.x + hw > ex.cx * 48 && boss.x - hw < (ex.cx + 1) * 48 && boss.y > ex.cy * 48 && boss.y - boss.r * 1.1 < (ex.cy + 1) * 48;
            assert.ok(!overlap, 'boss box overlaps the exit at tick ' + i);
        }
    });
} finally {
    removeRooms();
}

check('rooms still validate', () => {
    assert.equal(JSON.stringify(Rooms.validate()), '[]');
});

console.log('runenpfad bosses: ' + passed + ' checks passed');
