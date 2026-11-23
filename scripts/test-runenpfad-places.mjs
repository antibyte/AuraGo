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

// Same order as module-loader.js (round 10 modules included; files that do not exist yet are skipped); the interiors modules sit between the duel and levels modules.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things', 'interior', 'places', 'folk', 'folk-story', 'journal-story', 'village-rooms',
    'place-rooms-deep', 'village', 'chronicle', 'levels', 'combat', 'sim'];
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
    assert.equal(Object.keys(Rooms.PROP_DEFS).length, 112);
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
const TABLE = { 1: 'hermit_hut', 3: 'beast_den', 6: 'kobold_den', 9: 'crystal_grotto', 11: 'nomad_tent', 12: 'observatory', 14: 'sun_tomb', 16: 'summit_hut', 19: 'ice_cave', 22: 'brass_lab', 24: 'boiler_room' };
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
    const base = ['innkeeper', 'bard', 'patron', 'child', 'gambler']; // Gundar is always at his dice table
    const present = (profile) => {
        const sim = hubSim(profile);
        sim.interior = Interior.create(sim, 'tavern', null);
        return sim.interior.npcs.map((n) => n.npcId).sort();
    };
    deq(present(), base.slice().sort(), 'nobody but the base list at the start (no conditional guest)');
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

// ---- round 10: house tasks, cave exits, village guests ----
const Things = sandbox.RunenpfadInteriorThings;
assert.ok(Things && sandbox.RunenpfadChronicle, 'round 10 modules loaded');

function houseSim(placeId, profile) {
    const sim = hubSim(profile);
    sim.interior = Interior.create(sim, placeId, null);
    return sim;
}
const thingsOf = (it, kind) => it.things.filter((t) => t.kind === kind);
const isFree = (it, cx, cy) => cx >= 0 && cy >= 0 && cx < it.room.cols && cy < it.room.rows && !it.room.solid[cy * it.room.cols + cx];
function putAt(it, cx, cy, dy) {
    it.hero.x = (cx + 0.5) * 48;
    it.hero.y = (cy + 0.5) * 48 + 7 + (dy || 0);
    it.hero.vx = 0;
    it.hero.vy = 0;
}
function heroCell(it) { return { cx: Math.floor(it.hero.x / 48), cy: Math.floor((it.hero.y - 7) / 48) }; }

// Shortest cell path over the free cells of the room as it is now (4-neighbour); null when there is none.
function cellPath(it, from, to) {
    const key = (c) => c.cx + ',' + c.cy;
    const prev = { [key(from)]: null };
    const queue = [from];
    while (queue.length) {
        const c = queue.shift();
        if (c.cx === to.cx && c.cy === to.cy) {
            const out = [];
            for (let k = c; k; k = prev[key(k)]) out.unshift(k);
            return out;
        }
        for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
            const n = { cx: c.cx + dx, cy: c.cy + dy };
            if (!isFree(it, n.cx, n.cy) || (key(n) in prev)) continue;
            prev[key(n)] = c;
            queue.push(n);
        }
    }
    return null;
}

// Walks the hero with real cardinal input along the cell path; returns the ticks it took.
function walkTo(sim, it, cx, cy) {
    const path = cellPath(it, heroCell(it), { cx, cy });
    assert.ok(path, 'a way to ' + cx + ',' + cy);
    let ticks = 0;
    for (const c of path.slice(1)) {
        const tx = (c.cx + 0.5) * 48;
        const ty = (c.cy + 0.5) * 48 + 7;
        for (let i = 0; i < 150 && (Math.abs(it.hero.x - tx) > 3 || Math.abs(it.hero.y - ty) > 3); i++) {
            const input = {};
            if (Math.abs(it.hero.x - tx) >= Math.abs(it.hero.y - ty)) input[it.hero.x < tx ? 'right' : 'left'] = true;
            else input[it.hero.y < ty ? 'down' : 'up'] = true;
            tick(sim, input);
            ticks += 1;
            assert.ok(sim.interior === it, 'still in the room while walking');
        }
    }
    return ticks;
}

// Stands beside a thing (walking there) and presses against it for a moment, so the hero ends within talk range.
function touch(sim, it, t) {
    const side = [[0, 1, 'up'], [0, -1, 'down'], [-1, 0, 'right'], [1, 0, 'left']].filter(([dx, dy]) => isFree(it, t.cx + dx, t.cy + dy) && cellPath(it, heroCell(it), { cx: t.cx + dx, cy: t.cy + dy }))[0];
    assert.ok(side, 'a free side of ' + t.id);
    const ticks = walkTo(sim, it, t.cx + side[0], t.cy + side[1]);
    const input = {};
    input[side[2]] = true;
    tick(sim, input, 12);
    return ticks + 12;
}
// Presses the interact key at a thing; returns the events of that tick.
function use(sim, it, t) {
    touch(sim, it, t);
    tick(sim, {}, 2);
    return tick(sim, { talkPressed: 'e' });
}
function smash(sim, it, t) {
    touch(sim, it, t);
    const dir = Math.sign(heroCell(it).cx - t.cx) || 0;
    it.hero.ax = -dir;
    it.hero.ay = dir ? 0 : Math.sign(t.cy - heroCell(it).cy);
    for (let i = 0; i < 120 && it.hero.attackCool > 0; i++) tick(sim, {});
    return tick(sim, { attackPressed: true });
}
const closeTalk = (sim) => { assert.ok(sim.encounter && sim.encounter.kind === 'dialogue', 'a text opened'); Dialogue.close(sim); };

check('cave exits: the new way on, the old exits stay first', () => {
    const want = [['beast_den', 'beast_den_b', 'D'], ['crystal_grotto', 'crystal_grotto_b', 'D'], ['sun_tomb_b', 'sun_tomb_c', 'U'], ['ice_cave', 'ice_cave_b', 'D'], ['boiler_room_b', 'boiler_room_c', 'W']];
    for (const [roomId, target, first] of want) {
        const r = Rooms.ROOMS[roomId];
        deq(Object.keys(r.exits), [first, 'X'], roomId + ' exits');
        deq(r.exits.X, { to: target, at: 'a' });
        const lay = Rooms.layout(roomId);
        assert.equal(lay.markers.X.length, 1, roomId + ' one X');
        assert.equal(lay.markers.x.length, 1, roomId + ' one x');
        const [X] = lay.markers.X;
        const [x] = lay.markers.x;
        assert.equal(Math.abs(X.cx - x.cx) + Math.abs(X.cy - x.cy), 1, roomId + ': x is directly in front of X');
        assert.ok(!r.things.some((t) => t.at === 'x' || t.at === 'X'), roomId + ': no thing on the exit markers');
    }
    assert.equal(Object.keys(Rooms.ROOMS.beast_den.exits)[0], 'D');
    assert.equal(Rooms.ROOMS.beast_den.exits.D.to, 'out');
});

check('beast den: the scratch sign sits out of the way, no new thing in the open floor', () => {
    const r = Rooms.ROOMS.beast_den;
    const sign = r.things.filter((t) => t.sign)[0];
    assert.equal(sign.sign, 'den_scratch');
    assert.equal(r.things.filter((t) => ['pot', 'block', 'plate', 'lever', 'gate', 'spikes', 'dart', 'brazier', 'crack', 'page', 'use'].some((k) => t[k] != null)).length, 0);
    const lay = Rooms.layout('beast_den');
    const [c] = lay.markers[sign.at];
    const walls = [[1, 0], [-1, 0], [0, 1], [0, -1]].filter(([dx, dy]) => lay.walls[(c.cy + dy) * lay.cols + c.cx + dx]).length;
    assert.ok(walls >= 2, 'the sign stands in a corner pocket of the wall');
});

check('hermit hut: push the crate aside, read page 1, two pots', () => {
    const sim = houseSim('hermit_hut');
    const it = sim.interior;
    const crate = thingsOf(it, 'block')[0];
    const lect = thingsOf(it, 'page')[0];
    assert.equal(lect.n, 1);
    assert.deepEqual([crate.cx + 1, crate.cy], [lect.cx, lect.cy], 'the crate stands in the mouth of the nook');
    assert.equal(thingsOf(it, 'pot').length, 2);
    assert.ok(!isFree(it, crate.cx, crate.cy), 'the nook is shut');
    const from = { cx: crate.cx, cy: crate.cy };
    putAt(it, crate.cx, crate.cy + 1);
    for (let i = 0; i < 120 && crate.cy === from.cy; i++) tick(sim, { up: true });
    tick(sim, {}, 20);
    assert.equal(crate.cy, from.cy - 1, 'pushed one cell along the wall');
    assert.ok(isFree(it, from.cx, from.cy), 'the mouth of the nook is free');
    assert.equal(sim.profile.loot.includes('page_01'), false);
    const events = use(sim, it, lect);
    assert.ok(events.some((e) => e.type === 'in_page' && e.detail.page === 1 && e.detail.first));
    assert.ok(sim.profile.loot.includes('page_01'));
    closeTalk(sim);
    const evs = thingsOf(it, 'pot').flatMap((p) => smash(sim, it, p));
    assert.equal(evs.filter((e) => e.type === 'in_break').length, 2, 'two pots broke');
    assert.ok(thingsOf(it, 'pot').every((p) => p.broken));
});

check('kobold den: dice table by Snik, four barrels, no lectern', () => {
    const sim = houseSim('kobold_den');
    const it = sim.interior;
    assert.equal(thingsOf(it, 'page').length, 0);
    assert.equal(thingsOf(it, 'pot').filter((p) => p.look === 'barrel').length, 4);
    const tables = thingsOf(it, 'use');
    assert.equal(tables.length, 2);
    assert.ok(tables.every((t) => t.request === 'dice:barkeep' && t.cy === tables[0].cy) && Math.abs(tables[0].cx - tables[1].cx) === 1, 'a 2x1 table');
    use(sim, it, tables[0]);
    assert.equal(sim.uiRequest, 'dice:barkeep');
    sim.uiRequest = null;
    const snik = it.npcs.filter((n) => n.npcId === 'barkeep')[0];
    assert.ok(Math.hypot(snik.x - tables[0].x, snik.y - tables[0].y) < 4 * 48, 'near Snik');
    const evs = thingsOf(it, 'pot').flatMap((p) => smash(sim, it, p));
    assert.equal(evs.filter((e) => e.type === 'in_break').length, 4);
});

check('nomad tent: three star scrolls at different spots, no lectern', () => {
    const sim = houseSim('nomad_tent');
    const it = sim.interior;
    assert.equal(thingsOf(it, 'page').length, 0);
    const signs = thingsOf(it, 'sign');
    deq(signs.map((s) => s.node).sort(), ['scroll_1', 'scroll_2', 'scroll_3']);
    for (let i = 0; i < 3; i++) for (let j = i + 1; j < 3; j++) assert.ok(Math.abs(signs[i].cx - signs[j].cx) + Math.abs(signs[i].cy - signs[j].cy) >= 4, 'scrolls are apart');
    for (const s of signs) {
        const events = use(sim, it, s);
        assert.ok(events.some((e) => e.type === 'in_read'), s.node);
        closeTalk(sim);
    }
});

check('summit hut: three storm lamps lit in time open the gate to page 7', () => {
    const lit = (secs) => {
        const sim = houseSim('summit_hut');
        const it = sim.interior;
        const lamps = thingsOf(it, 'brazier');
        assert.equal(lamps.length, 3);
        assert.ok(lamps.every((l) => l.set === 'lamps' && l.timed === 9));
        for (let i = 0; i < 3; i++) for (let j = i + 1; j < 3; j++) assert.ok(Math.abs(lamps[i].cx - lamps[j].cx) + Math.abs(lamps[i].cy - lamps[j].cy) >= 5, 'lamps are not side by side');
        const gate = thingsOf(it, 'gate')[0];
        assert.ok(!gate.open && it.puzzle === 'open');
        return { sim, it, lamps, gate, secs };
    };
    // a quick run from the spawn: the lamp above it, then the two far ones
    const a = lit();
    const order = [a.lamps.filter((l) => l.bid === 'lamp_c')[0], a.lamps.filter((l) => l.bid === 'lamp_a')[0], a.lamps.filter((l) => l.bid === 'lamp_b')[0]];
    let t = 0;
    for (let i = 0; i < 3; i++) {
        const before = a.sim.interior.t;
        use(a.sim, a.it, order[i]);
        if (i === 0) t = before;
        assert.ok(order[i].lit, order[i].bid + ' is lit');
    }
    const took = a.it.t - t;
    if (process.env.RP_VERBOSE) console.log('summit run', took.toFixed(2), 's');
    assert.ok(took < 8.5, 'the run takes ' + took.toFixed(1) + ' s of the 9');
    tick(a.sim, {}, 30);
    assert.ok(a.gate.open && a.it.puzzle === 'solved', 'the gate is open for good');
    putAt(a.it, a.gate.cx, a.gate.cy);
    const lect = thingsOf(a.it, 'page')[0];
    assert.equal(lect.n, 7);
    use(a.sim, a.it, lect);
    assert.ok(a.sim.profile.loot.includes('page_07'));
    closeTalk(a.sim);
    // too slow: the first lamp is out again before the last one is lit
    const b = lit();
    const first = b.lamps[0];
    use(b.sim, b.it, first);
    assert.ok(first.lit);
    tick(b.sim, {}, 60 * 10);
    assert.ok(!first.lit, 'the lamp went out after nine seconds');
    assert.ok(!b.gate.open);
});

check('brass lab: three linked levers, solved by a short search, open the gate to page 9', () => {
    const defs = Rooms.ROOMS.brass_lab.things.filter((t) => t.lever != null);
    const ids = defs.map((d) => d.lever);
    assert.equal(ids.length, 3);
    const start = defs.map((d) => !!d.on);
    const pull = (state, i) => state.map((v, k) => (k === i || (defs[i].flips || []).includes(ids[k]) ? !v : v));
    const solved = (state) => state.every(Boolean);
    assert.ok(!solved(start), 'not solved at the start');
    for (let i = 0; i < 3; i++) assert.ok(!solved(pull(start, i)), 'one pull does not solve it');
    // breadth first search over the 8 states
    const seen = { [start.join()]: [] };
    const queue = [start];
    let best = null;
    while (queue.length && !best) {
        const s = queue.shift();
        for (let i = 0; i < 3 && !best; i++) {
            const n = pull(s, i);
            if (seen[n.join()]) continue;
            seen[n.join()] = seen[s.join()].concat(i);
            if (solved(n)) best = seen[n.join()];
            else queue.push(n);
        }
    }
    assert.ok(best && best.length >= 2 && best.length <= 4, 'shortest solution ' + (best && best.length) + ' pulls');
    // the same pulls with real input through the engine
    const sim = houseSim('brass_lab');
    const it = sim.interior;
    const levers = thingsOf(it, 'lever');
    const gate = thingsOf(it, 'gate')[0];
    assert.ok(!gate.open && it.puzzle === 'open');
    for (const i of best) {
        const lever = levers.filter((l) => l.lid === ids[i])[0];
        const evs = use(sim, it, lever);
        assert.ok(evs.some((e) => e.type === 'in_lever'));
    }
    tick(sim, {}, 30);
    assert.ok(levers.every((l) => l.on) && gate.open && it.puzzle === 'solved');
    // the gate stays open when the levers are pulled again
    use(sim, it, levers[0]);
    tick(sim, {}, 30);
    assert.ok(gate.open);
    walkTo(sim, it, gate.cx, gate.cy);
    const lect = thingsOf(it, 'page')[0];
    assert.equal(lect.n, 9);
    const events = use(sim, it, lect);
    assert.ok(events.some((e) => e.type === 'in_page' && e.detail.page === 9));
    assert.ok(sim.profile.loot.includes('page_09'));
    closeTalk(sim);
});

check('hall: task board, Cog after the finale, Edda and the recruits after the observatory', () => {
    const who = (flags) => {
        const sim = houseSim('hall', flags ? { npcFlags: flags } : undefined);
        return sim.interior.npcs.map((n) => n.npcId).sort();
    };
    deq(who(), ['elder'], 'nobody extra at the start');
    deq(who({ 'epilogue.paid': 1 }), ['automaton', 'elder'], 'Cog only with epilogue.paid');
    deq(who({ 'obs.done': 1 }), ['edda', 'elder', 'recruit_f', 'recruit_m'], 'Edda and the recruits only with obs.done');
    deq(who({ 'epilogue.paid': 1, 'obs.done': 1 }), ['automaton', 'edda', 'elder', 'recruit_f', 'recruit_m']);
    const sim = houseSim('hall', { npcFlags: { 'epilogue.paid': 1, 'obs.done': 1 } });
    const it = sim.interior;
    const board = thingsOf(it, 'use');
    assert.equal(board.length, 2);
    assert.ok(board.every((b) => b.request === 'board' && b.cy === board[0].cy) && Math.abs(board[0].cx - board[1].cx) === 1, 'a 2x1 board');
    use(sim, it, board[0]);
    assert.equal(sim.uiRequest, 'board');
    sim.uiRequest = null;
    for (const id of ['automaton', 'edda']) {
        talkTo(sim, id);
        assert.ok(Dialogue.view(sim).choices.length >= 1);
        Dialogue.close(sim);
    }
    const girl = it.npcs.filter((n) => n.npcId === 'recruit_f')[0];
    standBy(sim, girl);
    tick(sim, {}, 2);
    tick(sim, { talkPressed: 'e' });
    assert.ok(!sim.encounter, 'recruits only stand around');
});

check('tavern: Gundar sits at his dice table', () => {
    const sim = houseSim('tavern');
    const it = sim.interior;
    const tables = thingsOf(it, 'use');
    assert.equal(tables.length, 2);
    assert.ok(tables.every((t) => t.request === 'dice:gambler'));
    talkTo(sim, 'gambler');
    const res = Dialogue.choose(sim, 0);
    assert.ok(res.closed);
    assert.equal(sim.uiRequest, 'dice:gambler');
    sim.uiRequest = null;
    use(sim, it, tables[0]);
    assert.equal(sim.uiRequest, 'dice:gambler');
});

console.log(`Runenpfad places checks passed: ${passed} groups (rooms, campaign doors, enter and freeze, walls, determinism, mob kinds and telegraphs, bot clears, KO, loot once, leave, interior dialogue, profile, endless, dialogue engine, village hub, village places, folk, tavern guests, cellar, pedestals, cave exits, house tasks, hall and tavern guests).`);
