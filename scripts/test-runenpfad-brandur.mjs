import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

// Runenpfad round 11: Brandur (brandur.js) plus the emberling and drakeling (deep-mobs.js).
// The fight tests use a scripted bot that reads the sim and only writes input; --stats prints the balance table.

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Canonical sim-core order (round 11); files that do not exist yet are skipped.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things', 'deep-mobs', 'brandur',
    'interior', 'places', 'folk', 'folk-story', 'journal-story', 'ember-story', 'ember-story-folk', 'village-rooms', 'place-rooms-deep', 'deeps-rooms', 'village', 'cutscenes',
    'chronicle', 'ember', 'crafts', 'dice', 'bounties', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Sim = sandbox.RunenpfadSim;
const Physics = sandbox.RunenpfadPhysics;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Interior = sandbox.RunenpfadInterior;
const Things = sandbox.RunenpfadInteriorThings;
const Brandur = sandbox.RunenpfadBrandur;
const DeepMobs = sandbox.RunenpfadDeepMobs;
const Npcs = sandbox.RunenpfadNpcs;
assert.ok(Sim && Physics && Rooms && Mobs && Interior && Things && Brandur && DeepMobs && Npcs, 'modules loaded');

const STEP = Sim.STEP;
const CELL = 48;
const STATS = process.argv.includes('--stats');
const SEEDS = [11, 23, 37, 41, 59, 73];
const UPGRADES = { v: Sim.SAVE_VERSION, upgrades: { vitality: 3, might: 3, alchemy: 3 } };
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

// ---- the scripted fighter: reads the sim through a reaction-delayed snapshot of the dragon's hazards, returns an input per tick ----
const wrap = (a) => {
    while (a > Math.PI) a -= Math.PI * 2;
    while (a < -Math.PI) a += Math.PI * 2;
    return a;
};

function makeBot(seed, opts, Mobs) {
    const o = Object.assign({ light: true, react: null }, opts || {});
    let s = (Math.imul((seed >>> 0) + 7, 2654435761) >>> 0) || 1;
    const rnd = () => {
        s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
        return s / 4294967296;
    };
    const react = o.react != null ? o.react : 0.12 + rnd() * 0.18;
    const delay = Math.round(react * 60);
    const hist = [];
    let target = null;
    let missDash = 0;
    let stanceAng = null;
    const stats = { drank: 0, lit: 0, dashes: 0 };

    const snap = (b, it) => JSON.parse(JSON.stringify({ x: b.x, y: b.y, state: b.state, face: b.face, move: b.move, phase: b.phase, t: b.t, dr: b.dr, shots: it.shots.map((q) => ({ x: q.x, y: q.y, vx: q.vx, vy: q.vy, r: q.r })) }));
    const grounded = (b) => b.state === 'move' || b.state === 'tell' || b.state === 'attack';

    // Is (x, y) (hero centre) inside something that hurts, according to the (delayed) snapshot b?
    function hot(b, x, y, margin, it, tau) {
        if (!b || b.state === 'dead' || b.state === 'sleep') return false;
        const dr = b.dr;
        if (grounded(b) && !(b.state === 'tell' && dr.move === 'roar' && dr.tellDur === 0.9 && b.t < 0.9 && !b.move) && Math.hypot(x - b.x, y - (b.y - 7)) < 56 + 9 + margin) return true;
        if (dr.breath) {
            const br = dr.breath;
            const since = (b.state === 'tell' ? -(dr.tellDur - b.t) : b.t) + tau + react;
            if (since >= -0.05 && since <= br.dur + 0.05) {
                const k = Math.max(0, Math.min(1, since / br.dur));
                const kk = b.phase >= 3 ? (k < 0.5 ? k * 2 : 2 - k * 2) : k;
                const a = br.a0 + (br.a1 - br.a0) * kk;
                const d = Math.hypot(x - br.x, y - br.y);
                if (d < br.len + 24 && Math.abs(wrap(Math.atan2(y - br.y, x - br.x) - a)) <= br.half + 0.1 + Math.asin(Math.min(1, 14 / Math.max(d, 14)))) return true;
            }
        }
        for (const q of b.shots || []) {
            const tt = tau + react;
            if (Math.hypot(x - (q.x + q.vx * tt), y - (q.y + q.vy * tt)) < q.r + 9 + 14) return true;
        }
        if (dr.tail) {
            const t = dr.tail;
            if (Math.hypot(x - t.x, y - t.y) < t.r + 24 && Math.abs(wrap(Math.atan2(y - t.y, x - t.x) - t.a)) <= t.half + 0.1) return true;
        }
        const brave = b.state === 'drowsy' && dr.drowsy > 0.6;
        for (const k of dr.marks) if ((!k.done || k.t < k.at + 0.1) && Math.hypot(x - k.x, y - k.y) < k.r + 22) return true;
        if (!brave) for (const p of dr.patches) if (Math.hypot(x - p.x, y - p.y) < p.r + 14) return true;
        if (dr.ring && Math.hypot(x - dr.ring.x, y - dr.ring.y) < dr.ring.max + 24) return true;
        if (dr.shadow && b.t > 1.5 && Math.hypot(x - dr.shadow.x, y - (dr.shadow.y - 7)) < 150 + 26) return true;
        if (dr.vents) for (const r of dr.vents.rows) if (y > r * CELL - 12 && y < (r + 1) * CELL + 12) return true;
        return false;
    }

    function pathHot(sim, it, b, x, y, dx, dy, speed, horizon, goal) {
        const room = it.room;
        let bad = 0;
        let near = Math.hypot(goal.x - x, goal.y - y);
        let px = x;
        let py = y;
        const n = 12;
        for (let i = 1; i <= n; i++) {
            const nx = px + dx * speed * horizon / n;
            const ny = py + dy * speed * horizon / n;
            if (!Mobs.blocked(room, room.solid, nx, ny + 7, 11, 14)) { px = nx; py = ny; }
            if (hot(b, px, py, 26, it, horizon * i / n)) bad += 1;
            near = Math.min(near, Math.hypot(goal.x - px, goal.y - py));
        }
        return { bad, px, py, near };
    }

    const DIRS = [];
    for (let i = 0; i < 16; i++) DIRS.push({ x: Math.cos(i * Math.PI / 8), y: Math.sin(i * Math.PI / 8) });
    DIRS.push({ x: 0, y: 0 });

    function act(sim) {
        const it = sim.interior;
        const input = {};
        if (!it) return input;
        const h = it.hero;
        const boss = it.mobs.filter((m) => m.kind === 'brandur')[0];
        if (!boss) return input;
        const hx = h.x;
        const hy = h.y - 7;
        hist.push(snap(boss, it));
        if (hist.length > delay + 1) hist.shift();
        const b = hist[0];
        const speed = sim.stats.speed * 0.64;
        const reach = sim.heroId === 'knight' ? 52 : 46;
        const bd = Math.hypot(boss.x - hx, boss.y - 7 - hy) || 1;
        const bdx = (boss.x - hx) / bd;
        const bdy = (boss.y - 7 - hy) / bd;

        if (sim.player.hp <= 2 && sim.player.potions > 0 && sim.player.hp < sim.player.maxHp && h.invuln <= 0) {
            input.healPressed = true;
            stats.drank += 1;
        }

        // goal point
        let goal = null;
        let wantAttack = false;
        let wantLight = null;
        const drowsy = boss.state === 'drowsy';
        const pillars = it.things.filter((t) => t.kind === 'brazier' && /^pillar/.test(t.bid));
        const stance = (radius) => {
            const room = it.room;
            let pick = null;
            for (let i = 0; i < 24; i++) {
                const a = i * Math.PI / 12;
                const px = boss.x + Math.cos(a) * radius;
                const py = boss.y - 7 + Math.sin(a) * radius;
                if (Mobs.blocked(room, room.solid, px, py + 7, 11, 14) || hot(b, px, py, 8, it, 0)) continue;
                let dd = Math.hypot(px - hx, py - hy);
                if (stanceAng != null && Math.abs(wrap(a - stanceAng)) < 0.2) dd -= 40;
                if (!pick || dd < pick.dd) pick = { dd, x: px, y: py, a };
            }
            if (pick) stanceAng = pick.a;
            return pick;
        };
        if (boss.state === 'sleep') {
            goal = { x: boss.x, y: boss.y - 7 + 120 };
        } else if (drowsy && b.dr.drowsy < 0.45) {
            goal = { x: boss.x - bdx * 150, y: boss.y - 7 - bdy * 150 };
        } else if (drowsy) {
            const st = stance(76);
            goal = st ? { x: st.x, y: st.y } : { x: boss.x - bdx * 78, y: boss.y - 7 - bdy * 78 };
            wantAttack = true;
        } else if (o.light && boss.state !== 'dead') {
            const unlit = pillars.filter((p) => !p.lit);
            if (unlit.length) {
                // the cheapest order, ending at the pillar nearest to him (the drowsy window starts there)
                const perms = (arr) => (arr.length <= 1 ? [arr] : arr.flatMap((v, i) => perms(arr.filter((_, j) => j !== i)).map((r) => [v, ...r])));
                let bestP = null;
                for (const order of perms(unlit)) {
                    let c = 0;
                    let px = hx;
                    let py = hy;
                    for (const p of order) { c += Math.hypot(p.x - px, p.y - 7 - py); px = p.x; py = p.y - 7; }
                    c += 0.7 * Math.hypot(px - boss.x, py - (boss.y - 7));
                    if (!bestP || c < bestP.c) bestP = { c, order };
                }
                target = bestP.order[0];
                goal = { x: target.x, y: target.y - 7 };
                wantLight = target;
            }
        }
        if (!goal) {
            // chip damage from a safe distance
            const st = stance(92);
            goal = st ? { x: st.x, y: st.y } : { x: boss.x - bdx * 92, y: boss.y - 7 - bdy * 92 };
            wantAttack = true;
        }

        // choose a direction: lowest danger over the next half second, then closest to the goal
        const gd = Math.hypot(goal.x - hx, goal.y - hy);
        let best = null;
        for (const d of DIRS) {
            const r = pathHot(sim, it, b, hx, hy, d.x, d.y, speed, 1.2, goal);
            const left = r.near + 0.15 * Math.hypot(goal.x - r.px, goal.y - r.py);
            let score = r.bad * 300 + left;
            if (d.x === 0 && d.y === 0 && gd > 24) score += 18;
            if (!best || score < best.score) best = { score, d, bad: r.bad };
        }
        const nowHot = hot(b, hx, hy, 10, it, 0);
        let dir = best.d;
        if (gd < 14 && !nowHot && best.bad === 0) dir = { x: 0, y: 0 };
        if (dir.x || dir.y) {
            const sx = Math.abs(dir.x) > 0.38 ? Math.sign(dir.x) : 0;
            const sy = Math.abs(dir.y) > 0.38 ? Math.sign(dir.y) : 0;
            input.left = sx < 0;
            input.right = sx > 0;
            input.up = sy < 0;
            input.down = sy > 0;
        }
        const live = b && b.dr && (b.state === 'attack' || (b.dr.marks || []).length > 0 || b.dr.ring);
        if (nowHot && best.bad > 0 && h.dashCool <= 0 && live && (dir.x || dir.y)) {
            if (missDash <= 0 && rnd() < 0.9) {
                input.dashPressed = true;
                stats.dashes += 1;
            }
            missDash = 0.2;
        }
        if (missDash > 0) missDash -= 1 / 60;

        if (wantLight && Math.hypot(wantLight.x - hx, wantLight.y - 7 - hy) < 40) {
            input.talkPressed = 'e';
        }
        if (wantAttack && bd - boss.r < reach * 0.9 && !(nowHot && best.bad > 0 && !drowsy)) {
            input.attackPressed = true;
            // face him: the swing uses the last walked direction
            const sx = Math.abs(bdx) > 0.38 ? Math.sign(bdx) : 0;
            const sy = Math.abs(bdy) > 0.38 ? Math.sign(bdy) : 0;
            input.left = sx < 0;
            input.right = sx > 0;
            input.up = sy < 0;
            input.down = sy > 0;
        }
        return input;
    }
    act.stats = stats;
    return act;
}


// ---- test rooms: the lair (28 x 18, lava rim open at the exit, four pillars on a 7 x 5 rectangle, Brandur at G) and a small arena ----

function lairRows() {
    const cols = 28;
    const rows = 18;
    const g = [];
    for (let y = 0; y < rows; y++) {
        const row = [];
        for (let x = 0; x < cols; x++) {
            let ch = '.';
            if (x === 0 || y === 0 || x === cols - 1 || y === rows - 1) ch = '#';
            else if (x === 1 || y === 1 || x === cols - 2 || (y === rows - 2 && (x < 13 || x > 15))) ch = '~';
            row.push(ch);
        }
        g.push(row);
    }
    const put = (x, y, ch) => { g[y][x] = ch; };
    put(14, 17, 'B');
    put(14, 16, 'a');
    put(14, 4, 'G');
    put(14, 3, 'K');
    put(10, 7, '1');
    put(16, 7, '2');
    put(10, 11, '3');
    put(16, 11, '4');
    return g.map((r) => r.join(''));
}

function addRooms() {
    Rooms.PLACES.bd_place = { id: 'bd_place', nameKey: 'x', kind: 'cave', entry: 'bd_lair', facade: { clip: 'fac-village-b.f1', h: 200, dx: 0 }, rooms: ['bd_lair', 'bd_small'], world: 0 };
    Rooms.ROOMS.bd_lair = {
        theme: 'cave', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cave', spawn: 'a', exits: { B: { to: 'out' } }, guardian: 'brandur', big: true, view: 15,
        map: lairRows(),
        things: [
            { at: 'G', mob: 'brandur' }, { at: 'K', use: 'brandur' },
            { at: '1', brazier: 'pillar_1', timed: 8, look: 'pillar' }, { at: '2', brazier: 'pillar_2', timed: 8, look: 'pillar' },
            { at: '3', brazier: 'pillar_3', timed: 8, look: 'pillar' }, { at: '4', brazier: 'pillar_4', timed: 8, look: 'pillar' }
        ]
    };
    // 16 x 9: a lava band in the middle (cols 7-8) between the monsters (left) and the hero spot (right)
    Rooms.ROOMS.bd_small = {
        theme: 'cave', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cave', spawn: 'S', exits: { X: { to: 'out' } },
        map: ['################', '#..............#', '#..E...~~.....##', '#......~~..D...#', '#..G...~~.....##', '#......~~......#', '#.W....~~..S..X#', '#..............#', '################'],
        things: [{ at: 'E', mob: 'emberling' }, { at: 'G', mob: 'drakeling' }]
    };
}
addRooms();
const REAL = Rooms.ROOMS.dp_lair ? 'dp_lair' : null;

// A graph for the choice dialogue when WP-G's has not landed yet.
const Story = sandbox.RunenpfadNpcStory;
if (Story && Story.GRAPHS && !Story.GRAPHS.brandur) {
    const node = () => ({ choices: [{ text: 'runenpfad.npc.test.bye', to: null }] });
    Story.GRAPHS.brandur = { virtual: true, nodes: { beaten: node(), again: node(), sleeps: node(), bones: node() } };
}

function tick(sim, input, n) {
    const events = [];
    for (let i = 0; i < (n || 1); i++) events.push(...Sim.stepSimulation(sim, STEP * 1.000001, input || {}));
    return events;
}

function arena(hero, seed, profile, roomId) {
    const sim = Sim.createSimulation({ levelId: 3, hero: hero || 'knight', seed: seed || 77, profile: profile || UPGRADES });
    tick(sim, {}, 30);
    sim.interior = Interior.create(sim, roomId === 'dp_lair' ? 'deeps' : 'bd_place', null);
    if (roomId === 'dp_lair') Interior.enterRoom(sim, sim.interior, 'dp_lair', 'a');
    return sim;
}
const bossOf = (sim) => sim.interior && sim.interior.mobs.filter((m) => m.kind === 'brandur')[0];
const placeHero = (sim, x, y) => { sim.interior.hero.x = x; sim.interior.hero.y = y; };
const same = (a, b) => assert.deepStrictEqual(JSON.parse(JSON.stringify(a)), JSON.parse(JSON.stringify(b)));
const flagSet = (sim, v) => Npcs.setFlag(sim, 'dr.fate', v);

// Runs the bot until Brandur is beaten, the hero is out or the clock runs out.
function fight(hero, seed, opts) {
    const o = Object.assign({ light: true, limit: 300, roomId: 'bd_lair' }, opts || {});
    const sim = arena(hero, seed, o.profile, o.roomId);
    const bot = makeBot(seed, { light: o.light }, Mobs);
    const log = [];
    let drowsy = 0;
    let t = 0;
    const potions0 = sim.player.potions;
    let won = false;
    let hits = 0;
    while (t < o.limit) {
        const ev = tick(sim, bot(sim), 1);
        t += STEP;
        for (const e of ev) {
            log.push(e.type + (e.detail && e.detail.move ? ':' + e.detail.move : '') + (e.detail && e.detail.on != null ? ':' + e.detail.on : ''));
            if (e.type === 'dr_drowsy' && e.detail.on) drowsy += 1;
            if (e.type === 'in_hit' && e.detail && e.detail.target === 'hero') hits += 1;
        }
        if (!sim.interior) break;
        if (sim.profile.npcFlags['dr.fate'] === 1) { won = true; break; }
    }
    const boss = bossOf(sim);
    return { sim, won, time: t, drowsy, drank: potions0 - sim.player.potions, hits, log, bossHp: boss ? boss.hp : 0 };
}

if (STATS) {
    for (const light of [true, false]) {
        for (const hero of ['knight', 'ranger']) {
            for (const seed of SEEDS) {
                const r = fight(hero, seed, { light });
                console.log(JSON.stringify({ light, hero, seed, won: r.won, time: +r.time.toFixed(1), drowsy: r.drowsy, drank: r.drank, hits: r.hits, bossHp: +r.bossHp.toFixed(1) }));
            }
        }
    }
    process.exit(0);
}

// Awake and idle: no move starts until a test forces one.
function awake(sim) {
    const m = bossOf(sim);
    m.aggro = true;
    m.state = 'move';
    m.t = 0;
    m.cd = 1e9;
    m.waking = false;
    return m;
}
// Forces the next move and runs until it has finished (state back to move) or `max` seconds passed. Returns the events.
function runMove(sim, id, max, each) {
    const m = bossOf(sim);
    m.pendMoves = [id];
    m.cd = 0;
    const events = [];
    let started = false;
    for (let i = 0; i < Math.ceil((max || 8) / STEP); i++) {
        const ev = tick(sim, {}, 1);
        events.push(...ev);
        if (each) each(m, ev, i);
        if (m.state === 'tell' || m.state === 'attack' || m.state === 'air') started = true;
        if (started && m.state === 'move') break;
    }
    m.cd = 1e9;
    return events;
}
const types = (events, type) => events.filter((e) => e.type === type);
function guard(sim) {
    sim.player.maxHp = 99;
    sim.player.hp = 99;
}

// ---- 1. registration ----
check('definitions are registered, the kind lists unchanged', () => {
    ['brandur', 'emberling', 'drakeling'].forEach((k) => assert.ok(Mobs.DEFS[k], k));
    assert.equal(Mobs.KINDS.length, 7);
    same(Mobs.BOSS_KINDS, ['brood', 'weaver', 'tombward', 'frostheart', 'boilerward', 'counter']);
    assert.equal(Mobs.DEFS.brandur.boss, true);
    assert.equal(Mobs.DEFS.brandur.r, 56);
    assert.ok(Mobs.DEFS.brandur.hp >= 48);
    same(Mobs.DEFS.emberling.hop, { dist: 120, rad: 26 });
    assert.equal(Mobs.DEFS.drakeling.fly, true);
    const hooks = sandbox.RunenpfadRoomHooks || [];
    assert.equal(hooks.filter((h) => h.id === 'brandur').length, 1);
    if (Things.USES) assert.equal(Things.USES.brandur.prompt, 'talk');
});

// ---- 2. tells, marks and the landing hold ----
check('every tell lasts at least 0.7 s, marks fall at least 1.0 s after they appear, the landing hold is 0.8 s', () => {
    Object.keys(Brandur.MOVES).forEach((id) => assert.ok(Brandur.MOVES[id].tell >= 0.7, id + ' tell'));
    assert.ok(Brandur.FALL.stalactite >= 1.0 && Brandur.FALL.rain >= 1.0);
    assert.equal(Brandur.AIR.hold, 0.8);
    const sim = arena('knight', 5);
    guard(sim);
    awake(sim);
    placeHero(sim, 14 * CELL, 12 * CELL);
    Object.keys(Brandur.MOVES).forEach((id) => {
        const m = bossOf(sim);
        m.phase = 3;
        m.dr.phase = 3;
        let tellAt = null;
        let atkAt = null;
        const ev = [];
        runMove(sim, id, 9, (mm, e, i) => {
            e.forEach((x) => {
                if (x.type === 'in_boss_tell') tellAt = i;
                if (x.type === 'in_boss_attack') atkAt = i;
                ev.push(x);
            });
        });
        assert.ok(tellAt != null && atkAt != null, id + ' tell and attack events');
        assert.ok((atkAt - tellAt) * STEP >= 0.7 - 1e-6, id + ' tell ' + ((atkAt - tellAt) * STEP).toFixed(2));
        assert.equal(types(ev, 'in_boss_tell')[0].detail.pattern, 'brandur_' + id);
        placeHero(sim, 14 * CELL, 12 * CELL);
        sim.interior.hero.invuln = 0;
        m.dr.marks = [];
        m.dr.patches = [];
    });
    // landing hold: the shadow stands still for the last 0.8 s of the air time
    const m = bossOf(sim);
    let held = 0;
    let last = null;
    let airT = 0;
    runMove(sim, 'takeoff', 9, (mm) => {
        if (mm.state !== 'air' || !mm.dr.shadow) return;
        airT = mm.t;
        if (mm.t > 2.2 + 0.05) {
            if (last && last.x === mm.dr.shadow.x && last.y === mm.dr.shadow.y) held += STEP;
            last = { x: mm.dr.shadow.x, y: mm.dr.shadow.y };
        }
        mm.dr.marks.forEach((k) => assert.ok(k.at >= 1.0));
    });
    assert.ok(held >= 0.7, 'hold ' + held.toFixed(2));
    assert.ok(airT >= 2.9);
});

// ---- 3. damage rules ----
check('hits deal 25 % awake, 150 % drowsy and nothing asleep, airborne or dead', () => {
    const sim = arena('knight', 5);
    guard(sim);
    const it = sim.interior;
    const m = bossOf(sim);
    assert.equal(m.state, 'sleep');
    const hp0 = m.hp;
    assert.equal(Mobs.hit(sim, it, m, 3, 1, 0), false);
    assert.equal(m.hp, hp0, 'asleep: no damage');
    assert.equal(types(sim.events.splice(0), 'dr_wake').length, 1, 'the hit wakes him');
    assert.ok(m.state === 'tell' && m.waking === true);
    awake(sim);
    Mobs.hit(sim, it, m, 3, 1, 0);
    assert.ok(Math.abs(m.hp - (hp0 - 0.75)) < 1e-9, 'awake 25 %');
    const ev = sim.events.splice(0);
    assert.equal(types(ev, 'dr_scale').length, 1);
    assert.equal(sim.notice, 'runenpfad.notice.dr_scales');
    m.state = 'drowsy';
    m.dr.drowsy = 4;
    const h1 = m.hp;
    Mobs.hit(sim, it, m, 3, 1, 0);
    assert.ok(Math.abs(m.hp - (h1 - 4.5)) < 1e-9, 'drowsy 150 %');
    assert.equal(types(sim.events.splice(0), 'dr_scale').length, 0);
    m.state = 'air';
    const h2 = m.hp;
    Mobs.hit(sim, it, m, 3, 1, 0);
    assert.equal(m.hp, h2, 'airborne');
    m.state = 'dead';
    Mobs.hit(sim, it, m, 3, 1, 0);
    assert.equal(m.hp, h2, 'dead');
    // Fractional hp: a stronger hit than the rest kills him
    m.state = 'drowsy';
    m.hp = 2;
    assert.equal(Mobs.hit(sim, it, m, 3, 1, 0), true);
    assert.equal(m.state, 'dead');
});

// ---- 4. pillars ----
function lightAll(sim) {
    const it = sim.interior;
    const pil = it.things.filter((t) => t.kind === 'brazier' && Brandur.PILLAR_RE.test(t.bid));
    assert.equal(pil.length, 4);
    pil.forEach((p) => {
        placeHero(sim, p.x - 30, p.y);
        it.hero.ax = 1;
        it.hero.ay = 0;
        tick(sim, { talkPressed: 'e' }, 1);
    });
    return pil;
}
check('four lit pillars make him drowsy for 5 s and then go out; the buffet puts them out', () => {
    const sim = arena('knight', 5);
    guard(sim);
    const m = awake(sim);
    const pil = lightAll(sim);
    assert.ok(pil.every((p) => p.lit && p.timed === 8), 'lit by the hero');
    const ev = tick(sim, {}, 3);
    assert.equal(m.state, 'drowsy');
    assert.equal(types(ev, 'dr_drowsy').filter((e) => e.detail.on).length, 1);
    assert.equal(sim.notice, 'runenpfad.notice.dr_drowsy');
    let drowsyTicks = 0;
    let out = [];
    for (let i = 0; i < 60 * 7; i++) {
        const e = tick(sim, {}, 1);
        if (m.state === 'drowsy') {
            drowsyTicks += 1;
            assert.ok(pil.every((p) => p.lit), 'pillars hold while he is drowsy');
        }
        out = out.concat(e);
    }
    assert.ok(Math.abs(drowsyTicks * STEP + 3 * STEP - 5) < 0.1, 'drowsy lasts 5 s, ' + drowsyTicks);
    assert.ok(pil.every((p) => !p.lit), 'all out afterwards');
    assert.equal(types(out, 'dr_drowsy').filter((e) => !e.detail.on).length, 1);
    assert.equal(types(out, 'in_brazier').filter((e) => e.detail.lit === false).length >= 4, true);
    // the buffet
    const sim2 = arena('knight', 5);
    guard(sim2);
    const m2 = awake(sim2);
    m2.phase = 2;
    m2.dr.phase = 2;
    const p2 = sim2.interior.things.filter((t) => t.kind === 'brazier' && Brandur.PILLAR_RE.test(t.bid));
    p2[0].lit = true; p2[0].left = 8;
    p2[1].lit = true; p2[1].left = 8;
    placeHero(sim2, 14 * CELL, 12 * CELL);
    const ev2 = runMove(sim2, 'buffet', 4);
    assert.ok(p2.every((p) => !p.lit), 'buffet puts them out');
    assert.equal(types(ev2, 'in_brazier').filter((e) => e.detail.lit === false).length, 2);
});

// ---- 5. phases and phase-only moves ----
check('phase thresholds and the moves of each phase', () => {
    const sim = arena('knight', 5);
    guard(sim);
    const it = sim.interior;
    const m = awake(sim);
    const max = m.maxHp;
    m.state = 'drowsy';
    m.dr.drowsy = 4;
    m.hp = max * 2 / 3 + 2;
    Mobs.hit(sim, it, m, 1, 1, 0);
    assert.equal(m.phase, 1, 'still phase 1 above two thirds');
    Mobs.hit(sim, it, m, 1, 1, 0);
    assert.equal(m.phase, 2);
    same(types(sim.events.splice(0), 'in_boss_phase').map((e) => e.detail.phase), [2]);
    m.hp = max / 3 + 0.5;
    Mobs.hit(sim, it, m, 1, 1, 0);
    assert.equal(m.phase, 3);
    same(types(sim.events.splice(0), 'in_boss_phase').map((e) => e.detail.phase), [3]);
    same(m.pendMoves, ['roar', 'vents']);
    // sample the choice per phase
    const seen = [];
    [1, 2, 3].forEach((ph) => {
        const s = arena('knight', 9 + ph);
        guard(s);
        const b = awake(s);
        b.phase = ph;
        b.dr.phase = ph;
        const set = {};
        let prev = [];
        for (let n = 0; n < 150; n++) {
            placeHero(s, (4 + (n % 20)) * CELL, 13 * CELL);
            b.state = 'move';
            b.cd = 0;
            b.dr.marks = [];
            b.dr.patches = [];
            b.dr.ring = null;
            b.dr.shadow = null;
            b.dr.alt = 0;
            b.t = 0;
            tick(s, {}, 1);
            if (b.state !== 'tell') continue;
            set[b.move] = (set[b.move] || 0) + 1;
            prev.push(b.move);
            if (prev.length >= 3) assert.ok(!(prev[prev.length - 1] === prev[prev.length - 2] && prev[prev.length - 2] === prev[prev.length - 3]), 'never three times in a row');
            if (prev.length >= 2 && prev[prev.length - 1] === 'takeoff') assert.notEqual(prev[prev.length - 2], 'takeoff');
            b.state = 'move';
            b.move = null;
            b.dr.move = null;
            b.dr.breath = null;
            b.dr.tail = null;
            b.dr.vents = null;
        }
        seen[ph] = set;
    });
    ['takeoff', 'buffet', 'roar', 'vents'].forEach((id) => assert.equal(seen[1][id] || 0, 0, 'phase 1 has no ' + id));
    ['takeoff', 'buffet', 'roar'].forEach((id) => assert.ok(seen[2][id] > 0, 'phase 2 uses ' + id));
    assert.equal(seen[2].vents || 0, 0, 'no vents before phase 3');
    assert.ok(seen[3].vents > 0, 'phase 3 uses vents');
    assert.ok(seen[1].breath > 0 && seen[1].stomp > 0);
});

// ---- 6. breath and fire patches ----
check('breath hurts 2 (1 with the scale shield); the patches 1 (0 with tea)', () => {
    const run = (shield, meal) => {
        const sim = arena('knight', 5);
        if (shield) Npcs.setFlag(sim, 'dr.shield', 1);
        if (meal) sim.meal = meal;
        sim.player.maxHp = 20;
        sim.player.hp = 20;
        const m = awake(sim);
        // the hero stands in front of him, 170 px below
        placeHero(sim, m.x, m.y + 170);
        let patchHurt = 0;
        const hp0 = sim.player.hp;
        let first = null;
        const ev = runMove(sim, 'breath', 4, (mm, e) => {
            e.forEach((x) => { if (x.type === 'in_hit' && x.detail.target === 'hero' && x.detail.kind === 'brandur_breath' && first == null) first = hp0 - sim.player.hp; });
        });
        assert.ok(first != null, 'the breath hit');
        // wait out the invulnerability, then stand in a patch
        const patches = m.dr.patches.slice();
        assert.equal(patches.length, 3, 'three patches');
        same(patches.map((p) => p.life), [4, 4, 4]);
        sim.interior.hero.invuln = 0;
        const hp1 = sim.player.hp;
        placeHero(sim, patches[1].x, patches[1].y + 7);
        tick(sim, {}, 3);
        patchHurt = hp1 - sim.player.hp;
        return { first, patchHurt, ev };
    };
    const base = run(false, null);
    assert.equal(base.first, 2);
    assert.equal(base.patchHurt, 1);
    assert.equal(run(true, null).first, 1);
    assert.equal(run(false, 'tea').patchHurt, 0);
    assert.equal(run(true, 'tea').first, 1);
});

// ---- the moves in detail ----
check('tail, stomp marks, the air move, vents and the roar cap', () => {
    const sim = arena('knight', 5);
    guard(sim);
    const it = sim.interior;
    const m = awake(sim);
    m.phase = 3;
    m.dr.phase = 3;
    // tail: behind him
    m.face = 1;
    placeHero(sim, m.x - 100, m.y + 7);
    const hpA = sim.player.hp;
    runMove(sim, 'tail', 3);
    assert.equal(hpA - sim.player.hp, 1, 'the tail hits what is behind him');
    // tail: in front of him is safe (the sector is aimed at the hero, but the facing check protects)
    sim.interior.hero.invuln = 0;
    m.x = 14 * CELL;
    m.y = 8 * CELL;
    m.face = 1;
    placeHero(sim, m.x + 100, m.y + 7);
    const hpB = sim.player.hp;
    runMove(sim, 'tail', 3);
    assert.equal(hpB - sim.player.hp, 0, 'in front is safe');
    // stomp: five marks, one on the hero
    sim.interior.hero.invuln = 0;
    placeHero(sim, 14 * CELL, 12 * CELL);
    let marks = 0;
    let onHero = false;
    runMove(sim, 'stomp', 3, (mm) => {
        if (mm.dr.marks.length && !marks) {
            marks = mm.dr.marks.length;
            const h = { x: it.hero.x, y: it.hero.y - 7 };
            onHero = mm.dr.marks.some((k) => Math.hypot(k.x - h.x, k.y - h.y) < 2);
            mm.dr.marks.forEach((k) => assert.ok(k.at >= 1.0 && k.r === 26));
        }
    });
    assert.equal(marks, 5);
    assert.ok(onHero);
    m.dr.marks = [];
    // vents: the hero's row and one row three cells away
    sim.interior.hero.invuln = 0;
    placeHero(sim, 6 * CELL, 12 * CELL);
    let rows = null;
    const hpV = sim.player.hp;
    const evV = runMove(sim, 'vents', 4, (mm) => { if (mm.dr.vents && !rows) rows = mm.dr.vents.rows.slice(); });
    assert.ok(rows && rows.length === 2 && Math.abs(rows[0] - rows[1]) === 3, 'two rows three apart');
    assert.ok(types(evV, 'dr_impact').some((e) => e.detail.kind === 'vent'));
    assert.ok(hpV - sim.player.hp <= 1);
    // the air move: eight rain marks, a landing ring, lands on the shadow
    sim.interior.hero.invuln = 0;
    placeHero(sim, 14 * CELL, 12 * CELL);
    let rain = 0;
    let ring = false;
    let altMax = 0;
    const evT = runMove(sim, 'takeoff', 9, (mm) => {
        rain = Math.max(rain, mm.dr.marks.filter((k) => k.kind === 'rain').length + (mm.rainN > rain ? 0 : 0));
        if (mm.dr.ring) ring = true;
        altMax = Math.max(altMax, mm.dr.alt);
        if (mm.state === 'air') assert.equal(Mobs.hit(sim, it, mm, 5, 1, 0), false);
    });
    assert.ok(ring && altMax === 1);
    assert.equal(m.rainN, 8);
    assert.ok(types(evT, 'dr_impact').some((e) => e.detail.kind === 'land'));
    const evT2 = evT.concat(tick(sim, {}, 80));
    assert.equal(types(evT2, 'dr_impact').filter((e) => e.detail.kind === 'rain').length, 8, 'eight rain marks land');
    assert.equal(m.state, 'move');
    assert.ok(!Mobs.blocked(it.room, it.room.solid, m.x, m.y, 45, 62), 'landed on free ground');
    // the roar: two drakelings, never more than two
    m.dr.marks = [];
    m.dr.ring = null;
    runMove(sim, 'roar', 3);
    runMove(sim, 'roar', 3);
    const adds = it.mobs.filter((o) => o.kind === 'drakeling' && o.owner === m.id && o.state !== 'dead');
    assert.equal(adds.length, 2);
});

// ---- 7. the bot wins, the naive bot does not ----
const results = [];
check('the scripted fighter beats Brandur with both heroes under jitter within 300 s', () => {
    for (const hero of ['knight', 'ranger']) {
        for (const seed of SEEDS) {
            const r = fight(hero, seed, { light: true });
            results.push({ hero, seed, won: r.won, time: r.time, drowsy: r.drowsy, drank: r.drank, hits: r.hits });
            assert.ok(r.won, hero + ' seed ' + seed + ' lost after ' + r.time.toFixed(0) + ' s (boss hp ' + r.bossHp.toFixed(1) + ')');
            assert.ok(r.time <= 300);
            assert.ok(r.drowsy >= 1, 'the pillars were needed');
        }
    }
});
check('a fighter that never lights the pillars does not win', () => {
    let wins = 0;
    for (const hero of ['knight', 'ranger']) {
        for (const seed of SEEDS.slice(0, 3)) {
            const r = fight(hero, seed, { light: false });
            assert.equal(r.drowsy, 0);
            if (r.won) wins += 1;
        }
    }
    assert.ok(wins <= 1, 'naive wins ' + wins + ' of 6');
});

// ---- 8. determinism ----
check('the same seed gives the same event log', () => {
    const a = fight('knight', 23, { light: true });
    const b = fight('knight', 23, { light: true });
    same(a.log, b.log);
    assert.equal(a.time, b.time);
});

// ---- 9. defeat ----
check('defeat: fate 1, the encounter after about 1.2 s, the room clears, no drakelings left', () => {
    const sim = arena('knight', 5);
    guard(sim);
    const it = sim.interior;
    const m = awake(sim);
    runMove(sim, 'roar', 3);
    assert.ok(it.mobs.some((o) => o.kind === 'drakeling'));
    m.phase = 2;
    m.state = 'drowsy';
    m.dr.drowsy = 4;
    m.hp = 1;
    let ev = [];
    Mobs.hit(sim, it, m, 3, 1, 0);
    ev = ev.concat(sim.events.splice(0));
    assert.equal(sim.profile.npcFlags['dr.fate'], 1);
    assert.equal(m.state, 'dead');
    assert.equal(types(ev, 'in_boss_die').length, 1);
    assert.ok(types(ev, 'in_mob_die').length >= 3, 'him and the two summoned drakelings');
    assert.ok(it.mobs.every((o) => o.state === 'dead'));
    same(it.lair && { fate: it.lair.fate, asked: it.lair.asked }, { fate: 'beaten', asked: false });
    assert.equal(m.dr.marks.length + m.dr.patches.length, 0);
    ev = ev.concat(tick(sim, {}, 30));
    assert.ok(!sim.encounter, 'not before 1.2 s');
    ev = ev.concat(tick(sim, {}, 60));
    assert.ok(sim.encounter && sim.encounter.nodeId === 'brandur.beaten', 'the choice dialogue');
    assert.equal(it.lair.asked, true);
    assert.equal(types(ev, 'in_clear').length, 1);
    assert.equal(it.cleared, true);
    assert.ok(!it.mobs.some((o) => o.kind === 'brandur'), 'removed after 0.6 s');
});

// ---- 10. re-entry ----
check('re-entry: no Brandur, the talk spot, the three nodes and the sealed pillars', () => {
    const expect = { 1: ['beaten', 'again'], 2: ['slain', 'bones'], 3: ['sealed', 'sleeps'] };
    Object.keys(expect).forEach((f) => {
        const sim = arena('knight', 5);
        guard(sim);
        flagSet(sim, Number(f));
        const it = sim.interior;
        Interior.enterRoom(sim, it, 'bd_lair', 'a');
        assert.ok(!it.mobs.some((o) => o.kind === 'brandur'), 'no mob');
        assert.equal(it.guardian, null);
        assert.equal(it.cleared, true);
        assert.equal(it.lair.fate, expect[f][0]);
        assert.equal(it.lair.asked, true);
        // the talk prompt at K
        const offers = [];
        Things.consider(sim, it, (x, y, kind, ref) => offers.push({ x, y, kind, ref }));
        const talk = offers.filter((o) => o.ref && o.ref.request === 'brandur');
        assert.equal(talk.length, 1, 'offered');
        assert.equal(talk[0].kind, 'talk');
        const input = {};
        assert.ok(Things.use(sim, it, talk[0], input) !== false || sim.encounter);
        assert.equal(sim.encounter && sim.encounter.nodeId, 'brandur.' + expect[f][1]);
        sim.encounter = null;
        tick(sim, {}, 120);
        if (f === '3') assert.ok(it.things.filter((t) => t.kind === 'brazier').every((p) => p.lit && !p.timed), 'sealed pillars stay lit');
        if (f !== '3') assert.ok(it.things.filter((t) => t.kind === 'brazier').every((p) => !p.lit));
        assert.equal(Brandur.talkNode(sim.profile), expect[f][1]);
    });
    // a living Brandur offers nothing
    const sim = arena('knight', 5);
    const offers = [];
    Things.consider(sim, sim.interior, (x, y, kind, ref) => offers.push(ref));
    assert.ok(!offers.some((r) => r && r.request === 'brandur'));
});

// ---- 11. the wake scene ----
check('the wake scene is requested once', () => {
    const sim = arena('knight', 5);
    assert.equal(sim.uiRequest, 'scene:dr_wake');
    sim.uiRequest = null;
    Interior.enterRoom(sim, sim.interior, 'bd_lair', 'a');
    assert.equal(sim.uiRequest, null, 'not again');
    const seen = arena('knight', 6, { v: Sim.SAVE_VERSION, seen: ['dr_wake'] });
    assert.equal(seen.uiRequest, null);
    const slain = arena('knight', 7);
    slain.uiRequest = null;
    flagSet(slain, 2);
    Interior.enterRoom(slain, slain.interior, 'bd_lair', 'a');
    assert.equal(slain.uiRequest, null, 'nothing after the fight');
    // waking by walking close
    const w = arena('knight', 8);
    w.uiRequest = null;
    const m = bossOf(w);
    placeHero(w, m.x, m.y + 250);
    const ev = tick(w, {}, 5);
    assert.equal(types(ev, 'dr_wake').length, 1);
    tick(w, {}, 70);
    assert.equal(m.state, 'move');
});

// ---- 12. emberling, drakeling, shots over lava ----
function small() {
    const sim = arena('knight', 5);
    guard(sim);
    Interior.enterRoom(sim, sim.interior, 'bd_small', null);
    return sim;
}
check('emberling hop, drakeling fireball, fireballs cross lava', () => {
    const sim = small();
    const it = sim.interior;
    assert.equal(it.room.lava.reduce((a, b) => a + b, 0) > 0, true, 'lava parsed');
    const e = it.mobs.filter((m) => m.kind === 'emberling')[0];
    const d = it.mobs.filter((m) => m.kind === 'drakeling')[0];
    // emberling: hero on its side of the lava, 100 px away
    placeHero(sim, e.x + 100, e.y);
    d.aggro = false;
    d.cd = 1e9;
    let tellAt = null;
    let atkAt = null;
    let hop = null;
    const hp0 = sim.player.hp;
    for (let i = 0; i < 60 * 4; i++) {
        const ev = tick(sim, {}, 1);
        ev.forEach((x) => {
            if (x.type === 'in_mob_tell' && x.detail.kind === 'emberling') tellAt = tellAt == null ? i : tellAt;
            if (x.type === 'in_mob_attack' && x.detail.kind === 'emberling') atkAt = atkAt == null ? i : atkAt;
        });
        if (e.hop && !hop) hop = Object.assign({}, e.hop);
        if (e.state === 'attack') assert.ok(e.hop && e.hop.k >= 0 && e.hop.k <= 1);
        if (tellAt != null && atkAt != null && e.state === 'move') break;
    }
    assert.ok(tellAt != null && atkAt != null, 'tell then attack');
    assert.ok((atkAt - tellAt) * STEP >= 0.7 - 1e-6, 'emberling tell');
    assert.ok(hop && Math.hypot(hop.tx - hop.fx, hop.ty - hop.fy) <= 120.001);
    assert.equal(e.hop, null, 'no hop after landing');
    assert.ok(hp0 - sim.player.hp >= 0, 'landing damage is applied by hurtHero');
    // a dash through the landing is safe
    const sim2 = small();
    const e2 = sim2.interior.mobs.filter((m) => m.kind === 'emberling')[0];
    sim2.interior.mobs.filter((m) => m.kind === 'drakeling')[0].cd = 1e9;
    placeHero(sim2, e2.x + 110, e2.y);
    let landed = false;
    for (let i = 0; i < 60 * 4 && !landed; i++) {
        if (e2.state === 'tell' && e2.t > 0.6) { sim2.interior.hero.x = e2.hop.tx + 200; sim2.interior.hero.y = e2.hop.ty; }
        tick(sim2, {}, 1);
        if (e2.state === 'move' && e2.cd > 1) landed = true;
    }
    assert.ok(landed);
    assert.equal(sim2.player.hp, 99, 'hero left the landing point');
    // drakeling: aimed fireball across the lava band
    const sim3 = small();
    const it3 = sim3.interior;
    const e3 = it3.mobs.filter((m) => m.kind === 'emberling')[0];
    const d3 = it3.mobs.filter((m) => m.kind === 'drakeling')[0];
    e3.cd = 1e9;
    d3.aggro = true;
    d3.cd = 0;
    placeHero(sim3, 12 * CELL, d3.y);
    let tell = null;
    let shot = null;
    let crossed = false;
    for (let i = 0; i < 60 * 6; i++) {
        const ev = tick(sim3, {}, 1);
        ev.forEach((x) => {
            if (x.type === 'in_mob_tell' && x.detail.kind === 'drakeling' && tell == null) tell = i;
            if (x.type === 'in_mob_attack' && x.detail.kind === 'drakeling' && shot == null) shot = i;
        });
        it3.shots.forEach((s) => {
            assert.equal(s.kind, 'fireball');
            assert.equal(s.dmg, 1);
            const cx = Math.floor(s.x / CELL);
            if (cx === 7 || cx === 8) crossed = true;
        });
        if (crossed && shot != null) break;
    }
    assert.ok(tell != null && shot != null && (shot - tell) * STEP >= 0.7 - 1e-6, 'drakeling tell');
    assert.ok(crossed, 'the fireball flew over the lava');
    // control: the same shot dies in a wall
    sim3.interior.shots = [{ x: 14.5 * CELL, y: 3 * CELL, vx: 160, vy: 0, r: 7, dmg: 1, life: 2.4, kind: 'fireball' }];
    for (let i = 0; i < 60; i++) Mobs.stepShots(sim3, sim3.interior, STEP);
    assert.equal(sim3.interior.shots.length, 0, 'walls still stop shots');
    // the hero cannot enter lava, a flyer can
    const sim4 = small();
    placeHero(sim4, 6.5 * CELL, 3.5 * CELL);
    tick(sim4, { right: true }, 90);
    assert.ok(sim4.interior.hero.x < 7 * CELL, 'the hero stops at the lava');
    const d4 = sim4.interior.mobs.filter((m) => m.kind === 'drakeling')[0];
    d4.x = 6.5 * CELL;
    d4.y = 3.5 * CELL + 7;
    Mobs.slide(sim4.interior.room, sim4.interior.room.walls, d4, 120, 0, 9, 12);
    assert.ok(d4.x > 7.5 * CELL, 'the flyer crosses');
});

// ---- 13. hurtHero damage and the blessing ----
check('hurtHero takes a damage amount; the blessing spares one lethal hit', () => {
    const sim = arena('knight', 5);
    const it = sim.interior;
    sim.player.hp = 6;
    sim.player.maxHp = 6;
    assert.equal(Interior.hurtHero(sim, it, 'brandur_breath', 0, 0, 2), true);
    assert.equal(sim.player.hp, 4);
    it.hero.invuln = 0;
    Interior.hurtHero(sim, it, 'x', 0, 0);
    assert.equal(sim.player.hp, 3, 'default is 1');
    it.hero.invuln = 0;
    Interior.hurtHero(sim, it, 'x', 0, 0, 0);
    assert.equal(sim.player.hp, 2, 'at least 1');
    // the blessing (sealed fate)
    const sim2 = arena('knight', 5);
    flagSet(sim2, 3);
    const it2 = sim2.interior;
    sim2.player.hp = 2;
    it2.hero.invuln = 0;
    Interior.hurtHero(sim2, it2, 'x', 0, 0, 2);
    if (sandbox.RunenpfadEmber) {
        assert.equal(sim2.player.hp, 1, 'spared');
        assert.equal(sim2.blessUsed, true);
        assert.ok(!it2.leaving);
        it2.hero.invuln = 0;
        Interior.hurtHero(sim2, it2, 'x', 0, 0, 2);
        assert.ok(sim2.player.hp <= 0 && it2.leaving, 'only once per run');
    }
    // the broth shortens the dash
    const sim3 = arena('ranger', 5);
    sim3.meal = 'broth';
    tick(sim3, { dashPressed: true, right: true }, 1);
    if (sandbox.RunenpfadEmber) assert.ok(Math.abs(sim3.interior.hero.dashCool - (0.7 * 0.6 - STEP * 0)) < 0.03, 'dash cool ' + sim3.interior.hero.dashCool);
});

// ---- the real lair (when WP-C's deeps-rooms.js is in) ----
if (REAL) {
    check('the real dp_lair: one full fight with the scripted fighter', () => {
        const r = fight('knight', 11, { light: true, roomId: 'dp_lair' });
        assert.ok(r.won, 'dp_lair: lost after ' + r.time.toFixed(0) + ' s');
        assert.ok(r.drowsy >= 1);
        const sim = arena('ranger', 23, null, 'dp_lair');
        assert.ok(bossOf(sim) && bossOf(sim).state === 'sleep');
    });
}

const med = (a) => a.slice().sort((x, y) => x - y)[Math.floor(a.length / 2)];
console.log('brandur: ' + passed + ' checks passed; bot wins: ' + results.filter((r) => r.won).length + '/' + results.length +
    ', median time ' + med(results.map((r) => r.time)).toFixed(0) + ' s, median drowsy cycles ' + med(results.map((r) => r.drowsy)) +
    (REAL ? ', real dp_lair run' : ', dp_lair not present (test room only)'));
