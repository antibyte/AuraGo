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

// Same order as module-loader.js; the interiors modules sit between the duel and levels modules.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior', 'places', 'folk', 'folk-story', 'village-rooms', 'village',
    'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Profile = sandbox.RunenpfadProfile;
const Npcs = sandbox.RunenpfadNpcs;
const Story = sandbox.RunenpfadNpcStory;
const Dialogue = sandbox.RunenpfadDialogue;
const Rooms = sandbox.RunenpfadPlaceRooms;
const Mobs = sandbox.RunenpfadInteriorMobs;
const Interior = sandbox.RunenpfadInterior;
const Places = sandbox.RunenpfadPlaces;
assert.ok(Levels && Sim && Profile && Npcs && Rooms && Mobs && Interior && Places, 'modules loaded');

const STEP = Sim.STEP;
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
    const sim = Sim.createSimulation(opts);
    tick(sim, {}, 30); // settle on the ground
    return sim;
}

function enter(sim, placeId) {
    const door = sim.stage.doors.filter((d) => d.placeId === placeId)[0];
    assert.ok(door, 'door for ' + placeId);
    sim.player.x = door.x;
    sim.player.y = door.y;
    sim.player.vx = 0;
    sim.player.vy = 0;
    sim.player.onGround = true;
    tick(sim, {}, 2);
    const events = tick(sim, { talkPressed: 'e' });
    return { door, events };
}

// ---- (a) data ----
check('rooms validate', () => {
    assert.equal(JSON.stringify(Rooms.validate()), "[]");
    assert.equal(Object.keys(Rooms.PROP_DEFS).length, 64);
    ['hermit_hut', 'beast_den', 'kobold_den', 'crystal_grotto', 'nomad_tent', 'sun_tomb', 'summit_hut', 'ice_cave', 'brass_lab', 'boiler_room'].forEach((id) => {
        assert.ok(Rooms.PLACES[id], id);
    });
    // the validator really catches defects
    const room = Rooms.ROOMS.hermit_hut;
    const saved = room.exits.D;
    delete room.exits.D;
    assert.ok(Rooms.validate().some((p) => /no out exit/.test(p)));
    room.exits.D = saved;
    room.things.push({ at: 'Z', mob: 'rat' });
    assert.ok(Rooms.validate().some((p) => /marker Z missing/.test(p)));
    room.things.pop();
    assert.equal(JSON.stringify(Rooms.validate()), "[]");
});

// ---- (b) campaign levels ----
const TABLE = { 1: 'hermit_hut', 3: 'beast_den', 6: 'kobold_den', 9: 'crystal_grotto', 11: 'nomad_tent', 14: 'sun_tomb', 16: 'summit_hut', 19: 'ice_cave', 22: 'brass_lab', 24: 'boiler_room' };
check('campaign doors', () => {
    for (let id = 1; id <= 25; id++) {
        const stage = Levels.buildStage(id);
        assert.ok(stage.width > 0 && stage.portal && stage.key, 'level ' + id + ' builds');
        const doors = stage.doors;
        if (TABLE[id]) {
            assert.equal(doors.length, 1, 'level ' + id);
            assert.equal(doors[0].placeId, TABLE[id]);
            assert.equal(doors[0].w, 56);
            assert.ok(doors[0].facade.clip && doors[0].facade.h > 0);
            assert.equal(doors[0].y, stage.groundY);
            assert.ok(stage.solidMap[Math.floor(doors[0].x / 48) + ',' + (Math.floor(stage.groundY / 48))], 'ground under the door');
        } else {
            assert.equal(doors.length, 0, 'level ' + id);
        }
    }
});

// ---- (c) enter, frozen clock ----
check('enter freezes the level', () => {
    const sim = newSim({ levelId: 3 });
    const { door, events } = enter(sim, 'beast_den');
    assert.ok(events.some((e) => e.type === 'place_enter' && e.detail.placeId === 'beast_den' && e.detail.kind === 'cave'));
    assert.ok(sim.interior, 'interior open');
    assert.equal(sim.interior.doorId, door.id);
    const before = sim.elapsed;
    const enemyX = sim.stage.enemies.map((e) => e.x);
    tick(sim, { right: true }, 120);
    assert.equal(sim.elapsed, before, 'clock frozen');
    assert.equal(JSON.stringify(sim.stage.enemies.map((e) => e.x)), JSON.stringify(enemyX));
    assert.ok(sim.interior.t > 1.9);
    assert.equal(Places.locked(sim), true);
    assert.equal(Places.placeOf(sim).id, 'beast_den');
});

// ---- (d) walls hold ----
check('walls hold the hero', () => {
    for (const dir of [{ left: true }, { right: true }, { up: true }, { down: true }, { up: true, left: true }, { down: true, right: true }]) {
        const sim = newSim({ levelId: 3 });
        enter(sim, 'beast_den');
        sim.player.hp = 99;
        sim.player.maxHp = 99;
        const it = sim.interior;
        for (let i = 0; i < 300; i++) {
            tick(sim, dir);
            if (!sim.interior) break;
            const h = it.hero;
            assert.ok(h.x > 0 && h.y > 0 && h.x < it.room.w && h.y < it.room.h, 'inside the room');
            assert.ok(!Mobs.blocked(it.room, it.room.solid, h.x, h.y, 11, 14), 'not inside a solid cell');
        }
        assert.ok(sim.interior, 'still inside ' + JSON.stringify(dir));
    }
});

// ---- (e) determinism ----
function scripted(i) {
    const k = Math.floor(i / 25) % 4;
    return { right: k === 0 || k === 3, down: k === 1, left: k === 2, up: k === 3 && i % 2 === 0, attackPressed: i % 17 === 0, dashPressed: i % 97 === 0 };
}
function snapshot(sim) {
    const it = sim.interior;
    return JSON.stringify({ hero: it.hero, mobs: it.mobs, shots: it.shots, pickups: it.pickups, rng: sim.rngState });
}
check('determinism', () => {
    const run = () => {
        const sim = newSim({ levelId: 3, seed: 4242 });
        enter(sim, 'beast_den');
        sim.player.hp = 99;
        sim.player.maxHp = 99;
        // wake the monsters so the script has something to react to
        sim.interior.mobs.forEach((m) => { m.aggro = true; });
        for (let i = 0; i < 600; i++) tick(sim, scripted(i));
        return snapshot(sim);
    };
    assert.equal(run(), run());
});

// ---- bot used by (f) and (g) ----
function botPlan(sim) {
    const it = sim.interior;
    const h = it.hero;
    const living = it.mobs.filter((m) => m.state !== 'dead');
    const input = {};
    if (sim.player.hp <= 2 && sim.player.potions > 0) input.healPressed = true;
    if (!living.length) return input;
    const dist = (m) => Math.hypot(m.x - h.x, (m.y - 7) - (h.y - 7));
    const reach = sim.heroId === 'knight' ? 52 : 46;
    living.sort((a, b) => dist(a) - dist(b));
    const t = living[0];
    const toward = (m, sign) => {
        const dx = (m.x - h.x) * sign;
        const dy = (m.y - h.y) * sign;
        if (dx > 10) input.right = true;
        if (dx < -10) input.left = true;
        if (dy > 10) input.down = true;
        if (dy < -10) input.up = true;
    };
    const teller = living.filter((m) => (m.state === 'tell' || m.state === 'attack') && dist(m) < 120 && m.state !== 'hurt')[0];
    if (teller && dist(teller) > reach * 0.7 && h.dashCool <= 0) {
        toward(teller, -1);
        input.dashPressed = true;
    } else if (teller && dist(teller) > reach * 0.7) {
        toward(teller, -1);
    } else {
        toward(t, 1);
        if (dist(t) - t.r < reach * 0.85) input.attackPressed = true;
    }
    return input;
}

// Wraps the plan with a stuck escape: no pathfinding, so after a blocked stretch it side-steps for a moment.
function botInput(sim) {
    const it = sim.interior;
    const h = it.hero;
    const st = sim.__bot || (sim.__bot = { x: 0, y: 0, n: 0, side: 0, until: 0 });
    if (Math.hypot(h.x - st.x, h.y - st.y) < 0.5) st.n += 1;
    else { st.n = 0; st.x = h.x; st.y = h.y; }
    const input = botPlan(sim);
    if (st.n > 8 && (input.left || input.right || input.up || input.down) && !input.attackPressed) {
        st.until = it.t + 0.6;
        st.side = (st.side + 1) % 4;
        st.n = 0;
    }
    if (it.t < st.until) {
        const out = {};
        out[['down', 'right', 'left', 'up'][st.side]] = true;
        if (input.healPressed) out.healPressed = true;
        return out;
    }
    return input;
}

// ---- (f) every kind, telegraphs ----
check('each kind dies, attacks are telegraphed', () => {
    Mobs.KINDS.forEach((kind) => {
        const sim = newSim({ levelId: 3, hero: 'knight' });
        enter(sim, 'beast_den');
        sim.player.hp = 99;
        sim.player.maxHp = 99;
        const it = sim.interior;
        it.mobs = [];
        const start = it.hero;
        const spot = { x: start.x, y: start.y - 5 * 48 };
        const m = Mobs.create(kind, 't_' + kind, spot.x, spot.y, 1);
        m.aggro = true;
        it.mobs.push(m);
        it.hadMobs = false; // keep the room state untouched
        const tells = {};
        let attacks = 0;
        let died = false;
        for (let i = 0; i < 60 * 40 && !died; i++) {
            const waiting = i < 60 * 3;
            const input = waiting ? {} : botInput(sim, {});
            tick(sim, input).forEach((e) => {
                if (e.type === 'in_mob_tell') tells[e.detail.id] = it.t;
                if (e.type === 'in_mob_attack') {
                    attacks += 1;
                    assert.ok(tells[e.detail.id] !== undefined, kind + ' attack without tell');
                    assert.ok(it.t - tells[e.detail.id] >= 0.45 - 1e-6, kind + ' tell shorter than 0.45 s: ' + (it.t - tells[e.detail.id]));
                }
                if (e.type === 'in_mob_die') died = true;
            });
        }
        assert.ok(died, kind + ' can be killed');
        assert.ok(attacks > 0, kind + ' attacked at least once');
    });
});

// ---- (g) bot clears the den ----
check('bot clears beast_den with both heroes', () => {
    ['knight', 'ranger'].forEach((hero) => {
        const sim = newSim({ levelId: 3, hero });
        enter(sim, 'beast_den');
        const it = sim.interior;
        let cleared = false;
        const events = [];
        for (let i = 0; i < 60 * 150 && sim.interior && !cleared; i++) {
            events.push(...tick(sim, botInput(sim, {})));
            cleared = !!(sim.interior && sim.interior.cleared);
        }
        assert.ok(sim.interior, hero + ' was not knocked out');
        assert.ok(cleared, hero + ' cleared the room'); if (process.env.RP_VERBOSE) console.log(hero, 'cleared in', sim.interior.t.toFixed(1), 's, hp', sim.player.hp + '/' + sim.player.maxHp, 'potions', sim.player.potions);
        assert.ok(events.some((e) => e.type === 'in_clear'));
        assert.equal(sim.stage.placeState.beast_den.cleared.beast_den, true);
        assert.equal(sim.interior.chests[0].locked, false, 'chest unlocked');
    });
});

// ---- (h) KO ----
check('KO sends the hero outside', () => {
    const sim = newSim({ levelId: 3 });
    const { door } = enter(sim, 'beast_den');
    sim.player.hp = 1;
    const dmgBefore = sim.damageTaken;
    const it = sim.interior;
    const m = Mobs.create('rat', 'ko', it.hero.x, it.hero.y, 0);
    m.aggro = true;
    it.mobs = [m];
    const events = [];
    for (let i = 0; i < 120 && sim.interior; i++) events.push(...tick(sim, {}));
    assert.equal(sim.interior, null, 'left the interior');
    assert.equal(sim.state, 'playing');
    assert.equal(sim.damageTaken, dmgBefore);
    assert.equal(sim.player.hp, 1);
    assert.ok(sim.player.invulnTimer > 1.9);
    assert.equal(sim.player.x, door.x);
    assert.ok(events.some((e) => e.type === 'place_leave' && e.detail.reason === 'ko'));
    assert.ok(events.some((e) => e.type === 'place_ko'));
});

// ---- (i) loot once ----
function clearAndOpen(sim) {
    const it = sim.interior;
    it.mobs.forEach((m) => Mobs.hit(sim, it, m, 99, 0, 1));
    tick(sim, {}, 3);
    const chest = it.chests[0];
    it.hero.x = chest.x - 36;
    it.hero.y = chest.y;
    return tick(sim, { talkPressed: 'e' });
}
check('chest pays once per save', () => {
    const sim = newSim({ levelId: 3 });
    enter(sim, 'beast_den');
    const coins = sim.profile.coins;
    const events = clearAndOpen(sim);
    assert.ok(events.some((e) => e.type === 'in_chest' && e.detail.lootId === 'beast_den' && !e.detail.empty));
    assert.equal(sim.profile.coins, coins + 40);
    assert.ok(sim.profile.loot.includes('beast_den'));
    assert.equal(sim.player.rune && sim.player.rune.kind, 'fire');
    assert.equal(sim.interior.chests[0].opened, true);
    // second visit in a fresh sim with the saved profile
    const saved = JSON.parse(JSON.stringify(sim.profile));
    const sim2 = newSim({ levelId: 3, profile: saved });
    enter(sim2, 'beast_den');
    assert.equal(sim2.interior.chests[0].opened, true, 'chest shows as opened');
    const coins2 = sim2.profile.coins;
    clearAndOpen(sim2);
    assert.equal(sim2.profile.coins, coins2, 'nothing paid twice');
});

// ---- (j) leaving ----
check('leave through the exit', () => {
    const sim = newSim({ levelId: 3 });
    const { door } = enter(sim, 'beast_den');
    const it = sim.interior;
    const ex = it.room.exits[0];
    it.hero.x = (ex.cx + 0.5) * 48;
    it.hero.y = (ex.cy - 1 + 0.5) * 48 + 7;
    const events = [];
    for (let i = 0; i < 120 && sim.interior; i++) events.push(...tick(sim, { down: true }));
    assert.equal(sim.interior, null);
    assert.ok(events.some((e) => e.type === 'place_leave' && e.detail.reason === 'exit'));
    assert.equal(sim.player.x, door.x);
    assert.ok(door.cool > 0.5, 'cooldown set');
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.interior, null, 'no instant re-entry');
    const before = sim.elapsed;
    tick(sim, {}, 60);
    assert.ok(sim.elapsed > before + 0.9, 'level clock runs again');
    tick(sim, { talkPressed: 'e' });
    assert.ok(sim.interior, 'can enter again after the cooldown');
});

// ---- (k) dialogue inside an interior ----
check('dialogue inside an interior', () => {
    Npcs.NPCS.fakey = { faction: null, nameKey: 'x.name', titleKey: 'x.title', color: '#fff', height: 74, fallback: {}, offers: [], duelCoins: 0 };
    Story.MEETINGS.push({ id: 'fakey_home', npcId: 'fakey', levels: [], starts: [{ to: 'hello' }] });
    Story.GRAPHS.fakey = { nodes: { hello: { choices: [{ to: null }] } } };
    Rooms.PLACES.test_place = { id: 'test_place', nameKey: 'x', kind: 'house', entry: 'test_room', facade: { clip: 'fac-village-b.f1', h: 200, dx: 0 }, rooms: ['test_room'], world: 0 };
    Rooms.ROOMS.test_room = {
        theme: 'wood', floor: 0, floorAlt: 1, wall: 2, dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: { to: 'out' } },
        map: ['##########', '#........#', '#...N....#', '#........#', '#........#', '#........#', '#........#', '#...S....#', '####D#####'],
        things: [{ at: 'N', npc: 'fakey', meeting: 'fakey_home' }, { at: 'N', npc: 'ghost_npc', meeting: 'ghost_home' }]
    };
    try {
        const sim = newSim({ levelId: 3 });
        sim.interior = Interior.create(sim, 'test_place', null);
        assert.equal(sim.interior.npcs.length, 1, 'unknown npc ids are skipped');
        const npc = sim.interior.npcs[0];
        sim.interior.hero.x = npc.x;
        sim.interior.hero.y = npc.y + 40;
        tick(sim, {}, 2);
        assert.equal(sim.interior.prompt && sim.interior.prompt.kind, 'talk');
        tick(sim, { talkPressed: 'e' });
        assert.ok(sim.encounter && sim.encounter.kind === 'dialogue', 'dialogue opened');
        const t0 = sim.interior.t;
        tick(sim, {}, 30);
        assert.equal(sim.interior.t, t0, 'interior held while talking');
        const res = Dialogue.choose(sim, 0);
        assert.ok(res.closed);
        assert.ok(!sim.encounter);
        tick(sim, {}, 10);
        assert.ok(sim.interior.t > t0, 'interior continues');
    } finally {
        delete Npcs.NPCS.fakey;
        Story.MEETINGS.pop();
        delete Story.GRAPHS.fakey;
        delete Rooms.PLACES.test_place;
        delete Rooms.ROOMS.test_room;
    }
    assert.equal(JSON.stringify(Rooms.validate()), "[]");
    assert.equal(JSON.stringify(Dialogue.validateGraphs()), "[]");
});

// ---- (l) profile ----
check('profile loot and seen', () => {
    const many = [];
    for (let i = 0; i < 70; i++) many.push('l_' + String(i).padStart(2, '0'));
    const p = Profile.importProfile({ v: 1, loot: ['b_x', 'a_y', 'BAD', 'a_y', 'x', 7, 'has space', 'z'.repeat(30)], seen: ['story_a', 'story_a', 'Nope', 'q'] });
    assert.deepEqual(JSON.parse(JSON.stringify(p.loot)), ['a_y', 'b_x']);
    assert.deepEqual(JSON.parse(JSON.stringify(p.seen)), ['story_a']);
    const big = Profile.importProfile({ v: 1, loot: many, seen: many.map((s) => 's' + s).slice(0, 40) });
    assert.equal(big.loot.length, 48);
    assert.equal(big.seen.length, 24);
    assert.deepEqual(JSON.parse(JSON.stringify(big.loot)), many.slice(0, 48));
    assert.deepEqual(JSON.parse(JSON.stringify(Profile.importProfile({ v: 1 }).loot)), []);
    assert.ok(Profile.LOOT_RE.test('tavern_cellar') && Profile.SEEN_RE.test('prologue'));
    const flags = {};
    for (let i = 0; i < 200; i++) flags['n' + i + '.v'] = 1;
    assert.equal(Object.keys(Profile.importProfile({ v: 1, npcFlags: flags }).npcFlags).length, 96);
    assert.ok(JSON.stringify(Profile.importProfile({ v: 1, loot: many, seen: many, npcFlags: flags })).length < 16384);
});

// ---- (m) endless ----
check('endless caves', () => {
    for (const floor of [1, 2, 4, 5, 6, 7]) assert.equal(Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed: 9001 }).doors.length, 0, 'floor ' + floor);
    const names = [];
    for (const floor of [3, 8, 13, 18, 23, 28]) {
        const stage = Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed: 9001 });
        assert.equal(stage.doors.length, 1, 'floor ' + floor);
        names.push(stage.doors[0].placeId);
        const i = stage.doors[0].x;
        assert.ok(i > 0 && i < stage.width);
    }
    assert.deepEqual(names, ['beast_den', 'crystal_grotto', 'sun_tomb', 'ice_cave', 'boiler_room', 'beast_den']);
    const sim = newSim({ mode: 'endless', endlessFloor: 3 });
    enter(sim, 'beast_den');
    assert.ok(sim.interior);
    const coins = sim.profile.coins;
    clearAndOpen(sim);
    assert.equal(sim.profile.coins, coins + 15);
    assert.deepEqual(JSON.parse(JSON.stringify(sim.profile.loot)), []);
    assert.equal(sim.player.rune, null, 'endless chests give coins only');
});

// ---- dialogue engine additions ----
check('dialogue conditions and effects', () => {
    const sim = newSim({ levelId: 3, profile: Profile.importProfile({ v: 1, unlockedLevel: 7, loot: ['tavern_cellar'] }) });
    assert.equal(Dialogue.evalCond({ progress: 6 }, sim), true);
    assert.equal(Dialogue.evalCond({ progress: 8 }, sim), false);
    assert.equal(Dialogue.evalCond({ loot: 'tavern_cellar' }, sim), true);
    assert.equal(Dialogue.evalCond({ loot: 'other_one' }, sim), false);
    sim.player.hp = 1;
    Dialogue.applyEffects(sim, null, [{ give: 'meal' }]);
    assert.equal(sim.player.hp, sim.player.maxHp);
    Story.GRAPHS.__ui = { nodes: { a: { choices: [{ effects: [{ ui: 'bogus' }], to: null }] } } };
    Story.MEETINGS.push({ id: '__ui', npcId: '__ui', levels: [], starts: [{ to: 'a' }] });
    assert.ok(Dialogue.validateGraphs().some((p) => /bad ui/.test(p)));
    delete Story.GRAPHS.__ui;
    Story.MEETINGS.pop();
    assert.equal(JSON.stringify(Dialogue.validateGraphs()), "[]");
});

// ---- village hub and folk (T6) ----
function deq(a, b, msg) { assert.deepStrictEqual(JSON.parse(JSON.stringify(a)), JSON.parse(JSON.stringify(b)), msg); }
const Trade = sandbox.RunenpfadTrade;
const Folk = sandbox.RunenpfadFolk;
const Village = sandbox.RunenpfadVillage;
const Duel = sandbox.RunenpfadDuel;
assert.ok(Trade && Folk && Village, 'folk and village modules loaded');

function hubSim(profile) { return newSim({ mode: 'campaign', levelId: 0, hero: 'knight', profile: profile ? Profile.importProfile(Object.assign({ v: 1 }, profile)) : undefined }); }
function standBy(sim, npc) {
    const it = sim.interior;
    for (const [dx, dy] of [[-38, 0], [38, 0], [0, 38], [0, -38]]) {
        const x = npc.x + dx;
        const y = npc.y + dy;
        if (!Mobs.blocked(it.room, it.room.solid, x, y, 11, 14)) { it.hero.x = x; it.hero.y = y; return; }
    }
    assert.fail('no free spot beside ' + npc.npcId);
}
function talkTo(sim, npcId) {
    const npc = sim.interior.npcs.filter((n) => n.npcId === npcId)[0];
    assert.ok(npc, npcId + ' stands in ' + sim.interior.placeId);
    standBy(sim, npc);
    tick(sim, {}, 2);
    assert.equal(sim.interior.prompt && sim.interior.prompt.kind, 'talk', npcId + ' shows a talk prompt');
    tick(sim, { talkPressed: 'e' });
    assert.ok(sim.encounter && sim.encounter.kind === 'dialogue' && sim.encounter.npcId === npcId, npcId + ' dialogue opened');
    return npc;
}
function walkOut(sim) {
    const it = sim.interior;
    const ex = it.room.exits.filter((e) => e.to === 'out')[0];
    it.hero.x = (ex.cx + 0.5) * 48;
    it.hero.y = (ex.cy - 1 + 0.5) * 48 + 7;
    const events = [];
    for (let i = 0; i < 120 && sim.interior; i++) events.push(...tick(sim, { down: true }));
    return events;
}

check('hub stage', () => {
    assert.equal(Levels.CAMPAIGN_LEVELS.length, 25);
    const stage = Levels.buildStage(0);
    assert.equal(stage.hub, true);
    assert.equal(stage.id, 0);
    assert.ok(stage.widthTiles >= 60 && stage.widthTiles <= 70, 'about 64 tiles: ' + stage.widthTiles);
    assert.equal(stage.enemies.length, 0);
    assert.equal(stage.hazards.length, 0);
    assert.equal(stage.collectibles.length, 0, 'no gems or coins');
    assert.equal(stage.checkpoints.length, 0);
    assert.equal(stage.chests.length, 0);
    assert.equal(stage.key, null);
    assert.equal(stage.portal, null);
    assert.equal(stage.door, null);
    assert.equal(stage.bossId, null);
    assert.equal(stage.mod, null, 'no rune trial');
    for (let tx = 1; tx < stage.widthTiles - 1; tx++) assert.ok(stage.solidMap[tx + ',13'], 'ground at tile ' + tx);
    // doors: four buildings and the rune arch, left to right, at least 9 tiles apart
    deq(stage.doors.map((d) => d.placeId), ['tavern', 'shop', 'hall', 'smithy', 'arch']);
    deq(stage.doors.map((d) => d.kind), ['house', 'house', 'house', 'house', 'arch']);
    for (let i = 1; i < stage.doors.length; i++) assert.ok(stage.doors[i].x - stage.doors[i - 1].x >= 9 * 48, 'door spacing ' + i);
    const fac = (id) => stage.doors.filter((d) => d.placeId === id)[0].facade;
    deq(JSON.parse(JSON.stringify(fac('tavern'))), { clip: 'fac-village-a.f0', h: 257, dx: 3 });
    deq(JSON.parse(JSON.stringify(fac('smithy'))), { clip: 'fac-village-a.f2', h: 250, dx: -65 });
    deq(JSON.parse(JSON.stringify(fac('arch'))), { clip: 'fac-village-b.f2', h: 250, dx: 0 });
    stage.doors.forEach((d) => { assert.equal(d.y, stage.groundY); assert.equal(d.w, 56); });
    for (const p of ['tavern', 'shop', 'hall', 'smithy']) assert.ok(Rooms.PLACES[p], p);
    // decor and ambient villagers
    const clips = stage.hubProps.map((p) => p.clip);
    for (let i = 0; i < 8; i++) assert.ok(clips.includes('vil-props.p' + i), 'prop p' + i);
    assert.ok(clips.includes('fac-village-b.f1') && clips.includes('fac-village-b.f3'), 'cottage and well');
    stage.hubProps.forEach((p) => { assert.ok(p.x > 0 && p.x < stage.width && p.y === stage.groundY && p.h > 0 && typeof p.flip === 'boolean'); });
    deq(stage.barks.map((b) => b.sheet), ['npc-villager', 'npc-guard']);
    stage.barks.forEach((b) => assert.ok(b.lines.length >= 3 && b.near === false && b.line === 0));
    // other levels carry no hub fields
    assert.ok(!Levels.buildStage(1).hub);
});

check('hub simulation: walk the whole street', () => {
    const sim = hubSim();
    assert.equal(sim.mode, 'campaign');
    assert.equal(sim.levelId, 0);
    assert.equal(sim.stage.hub, true);
    assert.ok(sim.player.x < 4 * 48, 'spawns at the left');
    const maxHp = sim.player.maxHp;
    const near = [];
    for (let i = 0; i < 3600; i++) {
        tick(sim, { right: true }).forEach((e) => { if (e.type === 'place_near') near.push(e.detail.placeId); assert.ok(e.type !== 'portal_win'); });
        assert.equal(sim.state, 'playing');
        assert.equal(sim.player.hp, maxHp);
    }
    assert.ok(sim.player.x > sim.stage.doors[4].x, 'walked past the arch');
    deq(near, ['tavern', 'shop', 'hall', 'smithy', 'arch']);
    assert.equal(sim.interior, null);
    assert.equal(sim.uiRequest, null, 'walking past never enters');
    assert.equal(sim.earnedStars, 0);
    assert.equal(sim.hasKey, false);
    assert.ok(sim.elapsed > 55, 'the clock runs');
});

check('hub villagers bark', () => {
    const sim = hubSim();
    const b = sim.stage.barks[0];
    sim.player.x = b.x;
    tick(sim, {}, 2);
    assert.equal(b.near, true);
    assert.equal(b.line, 0);
    tick(sim, {}, 215);
    assert.equal(b.line, 1, 'next line after 3.5 s');
    tick(sim, {}, 210);
    assert.equal(b.line, 2);
    tick(sim, {}, 210);
    assert.equal(b.line, 0, 'wraps around');
    sim.player.x = b.x + 130;
    tick(sim, {}, 2);
    assert.equal(b.near, false);
    assert.equal(b.t, 0);
});

check('enter each village place and leave again', () => {
    for (const placeId of ['tavern', 'shop', 'smithy', 'hall']) {
        const sim = hubSim();
        const { door, events } = enter(sim, placeId);
        assert.ok(events.some((e) => e.type === 'place_enter' && e.detail.placeId === placeId && e.detail.kind === 'house'), placeId);
        assert.ok(sim.interior && sim.interior.placeId === placeId);
        assert.equal(sim.interior.room.def.theme, 'wood');
        tick(sim, {}, 60);
        const out = walkOut(sim);
        assert.equal(sim.interior, null, placeId + ' left');
        assert.ok(out.some((e) => e.type === 'place_leave' && e.detail.reason === 'exit'));
        assert.equal(sim.player.x, door.x);
        assert.equal(sim.state, 'playing');
        assert.equal(sim.player.hp, sim.player.maxHp);
    }
});

check('rune arch opens the world map', () => {
    for (const key of ['e', 'up']) {
        const sim = hubSim();
        const door = sim.stage.doors[4];
        sim.player.x = door.x;
        sim.player.y = door.y;
        sim.player.vx = 0;
        tick(sim, {}, 2);
        assert.equal(sim.uiRequest, null);
        tick(sim, { talkPressed: key });
        assert.equal(sim.uiRequest, 'worldmap', key);
        assert.equal(sim.interior, null);
        assert.equal(sim.state, 'playing');
    }
});

check('folk definitions', () => {
    assert.equal(JSON.stringify(Rooms.validate()), '[]');
    const ids = ['elder', 'innkeeper', 'smith', 'shopkeeper', 'bard', 'hermit', 'barkeep', 'scribe', 'climber', 'automaton'];
    ids.concat(Folk.AMBIENT).forEach((id) => {
        const def = Npcs.NPCS[id];
        assert.ok(def, id + ' defined');
        assert.equal(def.duelCoins, 0);
        assert.ok(def.color && def.fallback && def.fallback.pack, id + ' colour and fallback');
        assert.ok(Folk.SPRITES[id], id + ' sprite');
        assert.equal(Duel.create(newSim({ levelId: 1 }), id, {}), null, id + ' cannot be duelled');
    });
    deq(Folk.AMBIENT, ['patron', 'child', 'villager_f', 'villager_m']);
    ['witch', 'wizard', 'merchant', 'chief', 'seer', 'alchemist'].forEach((id) => assert.ok(Folk.SPRITES[id], id));
    deq(JSON.parse(JSON.stringify(Folk.SPRITES.elder)), ['td-folk-village', 3]);
    assert.equal(Npcs.NPCS.barkeep.faction, 'kobolds');
    assert.equal(Npcs.NPCS.hermit.faction, 'coven');
    assert.equal(Npcs.NPCS.scribe.faction, 'circle');
    assert.equal(Npcs.NPCS.elder.faction, null);
    deq(Npcs.NPCS.innkeeper.offers, ['meal', 'potion']);
    assert.equal(JSON.stringify(Dialogue.validateGraphs()), '[]');
});

check('every resident can be talked to and the dialogue closes', () => {
    const homes = [['tavern', 'innkeeper'], ['tavern', 'bard'], ['shop', 'shopkeeper'], ['smithy', 'smith'], ['hall', 'elder'], ['hermit_hut', 'hermit'],
        ['kobold_den', 'barkeep'], ['nomad_tent', 'scribe'], ['summit_hut', 'climber'], ['brass_lab', 'automaton']];
    for (const [placeId, npcId] of homes) {
        const sim = hubSim();
        sim.interior = Interior.create(sim, placeId, null);
        talkTo(sim, npcId);
        const v = Dialogue.view(sim);
        assert.equal(v.npcId, npcId);
        assert.ok(v.choices.length >= 1 && v.choices.length <= 3, npcId + ' choices');
        for (let i = 0; i < 12 && sim.encounter; i++) {
            const view = Dialogue.view(sim);
            if (view.panel === 'trade') { Dialogue.closeTrade(sim); continue; }
            Dialogue.choose(sim, view.choices.length - 1);
        }
        assert.ok(!sim.encounter, npcId + ' dialogue closed');
        tick(sim, {}, 5);
        assert.ok(sim.interior, 'still inside after talking to ' + npcId);
    }
    // ambient villagers have no talk prompt
    const sim = hubSim();
    sim.interior = Interior.create(sim, 'tavern', null);
    const patron = sim.interior.npcs.filter((n) => n.npcId === 'patron')[0];
    standBy(sim, patron);
    tick(sim, {}, 2);
    assert.ok(!(sim.interior.prompt && sim.interior.prompt.kind === 'talk' && sim.interior.prompt.x === patron.x && sim.interior.prompt.y === patron.y));
    tick(sim, { talkPressed: 'e' });
    assert.ok(!sim.encounter);
});

check('story starts follow progress', () => {
    const textOf = (profile, placeId, npcId) => {
        const sim = hubSim(profile);
        sim.interior = Interior.create(sim, placeId, null);
        talkTo(sim, npcId);
        return Dialogue.view(sim).textKey;
    };
    assert.equal(textOf({}, 'hall', 'elder'), 'runenpfad.npc.elder.hello');
    assert.equal(textOf({ unlockedLevel: 6, npcFlags: { 'elder.met': 1 } }, 'hall', 'elder'), 'runenpfad.npc.elder.p6');
    assert.equal(textOf({ unlockedLevel: 21, npcFlags: { 'elder.met': 1 } }, 'hall', 'elder'), 'runenpfad.npc.elder.p21');
    assert.equal(textOf({ unlockedLevel: 3 }, 'tavern', 'bard'), 'runenpfad.npc.bard.r1');
    assert.equal(textOf({ unlockedLevel: 11 }, 'tavern', 'bard'), 'runenpfad.npc.bard.r3');
    assert.equal(textOf({ unlockedLevel: 21 }, 'tavern', 'bard'), 'runenpfad.npc.bard.r5');
    assert.equal(textOf({}, 'tavern', 'innkeeper'), 'runenpfad.npc.innkeeper.hello');
    assert.equal(textOf({ loot: ['tavern_cellar'] }, 'tavern', 'innkeeper'), 'runenpfad.npc.innkeeper.cellar_done');
});

check('the smith opens the workshop and the elder the chronicle', () => {
    const sim = hubSim();
    sim.interior = Interior.create(sim, 'smithy', null);
    talkTo(sim, 'smith');
    assert.equal(sim.uiRequest, null);
    const res = Dialogue.choose(sim, 0);
    assert.ok(res.closed);
    assert.ok(!sim.encounter);
    assert.equal(sim.uiRequest, 'workshop');
    const sim2 = hubSim({ unlockedLevel: 6, npcFlags: { 'elder.met': 1 } });
    sim2.interior = Interior.create(sim2, 'hall', null);
    talkTo(sim2, 'elder');
    Dialogue.choose(sim2, 0);
    assert.equal(sim2.uiRequest, 'chronicle');
});

check('npc_talk inside an interior carries no room coordinates', () => {
    const sim = hubSim();
    sim.interior = Interior.create(sim, 'smithy', null);
    const npc = sim.interior.npcs.filter((n) => n.npcId === 'smith')[0];
    standBy(sim, npc);
    tick(sim, {}, 2);
    const events = tick(sim, { talkPressed: 'e' });
    const talk = events.filter((e) => e.type === 'npc_talk')[0];
    assert.ok(talk, 'npc_talk pushed');
    assert.ok(!('x' in (talk.detail || {})) && !('y' in (talk.detail || {})), 'no x/y');
});

check('gifts are given once', () => {
    const sim = hubSim();
    sim.interior = Interior.create(sim, 'hermit_hut', null);
    talkTo(sim, 'hermit');
    const potions = sim.player.potions;
    Dialogue.choose(sim, 0); // what happened
    Dialogue.choose(sim, 0); // why
    assert.equal(Dialogue.view(sim).textKey, 'runenpfad.npc.hermit.why');
    Dialogue.choose(sim, 0); // the flask
    assert.equal(sim.player.potions, potions + 1);
    assert.equal(Npcs.hasFlag(sim, 'hermit.gift'), true);
    Dialogue.close(sim);
    talkTo(sim, 'hermit');
    Dialogue.choose(sim, 0);
    Dialogue.choose(sim, 0);
    assert.equal(Dialogue.view(sim).choices.length, 1, 'no second gift choice');
    const sim2 = hubSim();
    sim2.interior = Interior.create(sim2, 'brass_lab', null);
    talkTo(sim2, 'automaton');
    Dialogue.choose(sim2, 0); // notes
    Dialogue.choose(sim2, 1); // can I help
    Dialogue.choose(sim2, 0); // take the token
    assert.equal(Npcs.itemCount(sim2, 'discount_token'), 1);
    assert.equal(Npcs.hasFlag(sim2, 'automaton.gift'), true);
});

check('the meal heals to full and costs coins', () => {
    const sim = hubSim({ coins: 50 });
    sim.interior = Interior.create(sim, 'tavern', null);
    talkTo(sim, 'innkeeper');
    const res = Dialogue.choose(sim, 0);
    assert.equal(res.action, 'trade');
    const offers = Dialogue.view(sim).offers.map((o) => o.id);
    deq(offers, ['meal', 'potion']);
    const npc = sim.interior.npcs.filter((n) => n.npcId === 'innkeeper')[0];
    sim.player.hp = 1;
    const out = Trade.buy(sim, npc, 'meal');
    assert.ok(out.ok);
    assert.equal(sim.player.hp, sim.player.maxHp);
    assert.equal(sim.profile.coins, 50 - 8);
    assert.equal(Trade.priceOf(sim, npc, 'meal'), 8, 'villagers have no reputation discount');
    const poor = hubSim({ coins: 3 });
    poor.interior = Interior.create(poor, 'tavern', null);
    const n2 = poor.interior.npcs.filter((n) => n.npcId === 'innkeeper')[0];
    assert.equal(Trade.buy(poor, n2, 'meal').ok, false);
    assert.equal(poor.profile.coins, 3);
    // the stock lives in the stage's place state
    assert.equal(poor.stage.placeState.tavern.sold[n2.ref] && typeof poor.stage.placeState.tavern.sold[n2.ref], 'object');
});

check('tavern guests appear only when their condition holds', () => {
    const base = ['innkeeper', 'bard', 'patron', 'child'];
    const present = (profile) => {
        const sim = hubSim(profile);
        sim.interior = Interior.create(sim, 'tavern', null);
        return sim.interior.npcs.map((n) => n.npcId).sort();
    };
    deq(present(), base.slice().sort(), 'nobody extra at the start');
    const cases = [
        ['witch', { quests: { active: [], done: ['witch_mushrooms'] } }, ['witch']],
        ['chief', { npcFlags: { 'chief.paid': 1 } }, ['chief']],
        ['chief', { npcFlags: { 'chief.respect': 1 } }, ['chief']],
        ['chief', { npcFlags: { 'chief.charmed': 1 } }, ['chief']],
        ['merchant', { unlockedLevel: 11 }, ['merchant']],
        ['seer', { npcFlags: { 'seer.friend': 1 } }, ['seer']],
        ['wizard', { unlockedLevel: 21 }, ['merchant', 'wizard']],
        ['alchemist', { npcFlags: { 'alchemist.revealed': 1 } }, ['alchemist']]
    ];
    for (const [id, profile, extra] of cases) {
        deq(present(profile), base.concat(extra).sort(), id + ' ' + JSON.stringify(profile));
        const sim = hubSim(profile);
        sim.interior = Interior.create(sim, 'tavern', null);
        talkTo(sim, id);
        assert.equal(Dialogue.view(sim).textKey, 'runenpfad.npc.' + id + '.tavern');
        assert.equal(Dialogue.choose(sim, 0).closed, true);
    }
});

check('cellar chest pays once', () => {
    const sim = hubSim();
    sim.interior = Interior.create(sim, 'tavern', null);
    Interior.enterRoom(sim, sim.interior, 'tavern_cellar', 'e');
    const it = sim.interior;
    assert.equal(it.mobs.filter((m) => m.kind === 'rat').length, 5);
    assert.equal(it.chests[0].locked, true);
    assert.equal(it.room.def.dark, 0.6);
    const coins = sim.profile.coins;
    const potions = sim.player.potions;
    it.mobs.forEach((m) => Mobs.hit(sim, it, m, 99, 0, 1));
    tick(sim, {}, 3);
    assert.equal(it.chests[0].locked, false);
    const open = (s) => {
        const c = s.interior.chests[0];
        s.interior.hero.x = c.x;
        s.interior.hero.y = c.y + 38;
        tick(s, {}, 2);
        return tick(s, { talkPressed: 'e' });
    };
    const events = open(sim);
    assert.ok(events.some((e) => e.type === 'in_chest' && e.detail.lootId === 'tavern_cellar' && !e.detail.empty));
    assert.equal(sim.profile.coins, coins + 30);
    assert.equal(sim.player.potions, potions + 1);
    assert.ok(sim.profile.loot.includes('tavern_cellar'));
    open(sim);
    assert.equal(sim.profile.coins, coins + 30, 'no second payout in the same visit');
    // a later run: the chest shows as opened and the innkeeper is thankful
    const sim2 = hubSim(JSON.parse(JSON.stringify(sim.profile)));
    sim2.interior = Interior.create(sim2, 'tavern', null);
    Interior.enterRoom(sim2, sim2.interior, 'tavern_cellar', 'e');
    assert.equal(sim2.interior.chests[0].opened, true);
    const c2 = sim2.profile.coins;
    sim2.interior.mobs.forEach((m) => Mobs.hit(sim2, sim2.interior, m, 99, 0, 1));
    tick(sim2, {}, 3);
    open(sim2);
    assert.equal(sim2.profile.coins, c2);
    // the trapdoor leads down and back up
    const sim3 = hubSim();
    sim3.interior = Interior.create(sim3, 'tavern', null);
    const trap = sim3.interior.room.exits.filter((e) => e.to === 'tavern_cellar')[0];
    sim3.interior.hero.x = (trap.cx - 1 + 0.5) * 48;
    sim3.interior.hero.y = (trap.cy + 0.5) * 48 + 7;
    tick(sim3, {}, 2);
    for (let i = 0; i < 60 && sim3.interior.roomId === 'tavern_main'; i++) tick(sim3, { right: true });
    assert.equal(sim3.interior.roomId, 'tavern_cellar', 'down the trapdoor');
});

check('pedestals light up with the story', () => {
    const lit = (profile) => {
        const sim = hubSim(profile);
        sim.interior = Interior.create(sim, 'hall', null);
        return sim.interior.room.lights.filter((l) => l.color === '#9ad8ff').length;
    };
    assert.equal(lit(), 0);
    assert.equal(lit({ unlockedLevel: 6 }), 1);
    assert.equal(lit({ unlockedLevel: 16 }), 3);
    assert.equal(lit({ unlockedLevel: 21 }), 4);
    assert.equal(lit({ unlockedLevel: 21, npcFlags: { 'epilogue.paid': 1 } }), 5);
});

console.log(`Runenpfad places checks passed: ${passed} groups (rooms, campaign doors, enter and freeze, walls, determinism, mob kinds and telegraphs, bot clears, KO, loot once, leave, interior dialogue, profile, endless, dialogue engine, village hub, village places, folk, tavern guests, cellar, pedestals).`);
