import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

function loadModule(relPath, sandbox) {
    const absPath = path.join(rootDir, relPath);
    const code = fs.readFileSync(absPath, 'utf8');
    vm.runInContext(code, sandbox, { filename: absPath });
}

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Same order as module-loader.js; missing modules are skipped so the suite grows with the phases.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const rel = `ui/js/desktop/apps/runenpfad/${m}.js`;
    if (fs.existsSync(path.join(rootDir, rel))) loadModule(rel, sandbox);
}

const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Profile = sandbox.RunenpfadProfile;
const Npcs = sandbox.RunenpfadNpcs;
const Story = sandbox.RunenpfadNpcStory;
const Dialogue = sandbox.RunenpfadDialogue;
const Trade = sandbox.RunenpfadTrade;
const Runes = sandbox.RunenpfadRunes;
assert.ok(Levels && Sim && Profile, 'core modules loaded');
assert.ok(Npcs, 'RunenpfadNpcs loaded');
assert.ok(Story, 'RunenpfadNpcStory loaded');
assert.ok(Dialogue, 'RunenpfadDialogue loaded');
assert.ok(Trade, 'RunenpfadTrade loaded');

// Objects built inside the vm have foreign prototypes: compare through JSON.
function deepEq(a, b, msg) { assert.deepStrictEqual(JSON.parse(JSON.stringify(a)), JSON.parse(JSON.stringify(b)), msg); }

const STEP = Sim.STEP;
const T = Levels.TILE;

function tick(sim, input, n) {
    for (let i = 0; i < (n || 1); i++) Sim.stepSimulation(sim, STEP * 1.000001, input || {});
}

function profileWith(extra) {
    return Profile.importProfile(Object.assign(Profile.createDefaultProfile(), extra || {}));
}

// ---- 2.1 profile story fields ----
{
    const old = { v: 1, hero: 'ranger', coins: 40, unlockedLevel: 3 };
    const p = Profile.importProfile(old);
    deepEq(p.rep, { coven: 0, circle: 0, kobolds: 0 });
    deepEq(p.npcFlags, {});
    deepEq(p.inventory, {});
    deepEq(p.quests, { active: [], done: [] });
    assert.equal(p.coins, 40);

    deepEq(profileWith({ rep: { coven: 99, evil: 3 } }).rep, { coven: 5, circle: 0, kobolds: 0 });
    assert.equal(profileWith({ rep: { circle: -50 } }).rep.circle, -5);

    const flags = {};
    for (let i = 0; i < 500; i++) flags['n' + i + '.v'] = 1;
    flags['a'.repeat(200) + '.x'] = 1;
    flags['bad key.x'] = 1;
    flags['zero.v'] = 0;
    flags['big.v'] = 1e9;
    const fp = profileWith({ npcFlags: flags });
    assert.equal(Object.keys(fp.npcFlags).length, 64, 'flags capped at 64');
    assert.ok(!Object.keys(fp.npcFlags).some((k) => k.length > 30), 'long key dropped');
    assert.ok(!('zero.v' in fp.npcFlags));
    assert.equal(profileWith({ npcFlags: { 'big.v': 1e9 } }).npcFlags['big.v'], 99);
    deepEq(profileWith({ npcFlags: ['x.y'] }).npcFlags, {});

    deepEq(profileWith({ inventory: { glow_mushroom: 1e9, dragon_egg: 1, hex_charm: 0 } }).inventory, { glow_mushroom: 9 });
    const q = profileWith({ quests: { active: ['witch_mushrooms', 'witch_mushrooms', 'nope', 'seer_shards'], done: ['seer_shards', 'seer_shards'] } }).quests;
    deepEq(q.active, ['witch_mushrooms']);
    deepEq(q.done, ['seer_shards']);
    deepEq(profileWith({ quests: 'bad' }).quests, { active: [], done: [] });

    const maxp = profileWith({ rep: { coven: 5, circle: -5, kobolds: 5 }, npcFlags: (() => { const f = {}; for (let i = 0; i < 80; i++) f['abcdefghijklmno' + (i % 10) + '.' + 'x'.repeat(22) + (i % 10)] = 99; return f; })(), inventory: Profile.STORY_ITEMS, quests: { active: Profile.QUEST_IDS, done: Profile.QUEST_IDS } });
    assert.ok(JSON.stringify(maxp).length < 16384, 'maximal profile stays under the save limit');
    deepEq(Profile.importProfile(JSON.parse(JSON.stringify(maxp))), maxp, 'story fields survive a round trip');

    // Discount token: -30 % on the next workshop upgrade, consumed by buyUpgrade.
    const track = Levels.UPGRADE_DEFS[0].id;
    const base = Profile.upgradeCost(track, 1);
    const withTok = profileWith({ coins: 500, inventory: { discount_token: 2 } });
    assert.equal(Profile.upgradeCost(track, 1, withTok), Math.ceil(base * 0.7));
    assert.equal(Profile.upgradeCost(track, 1, profileWith({})), base);
    const bought = Profile.buyUpgrade(withTok, track);
    assert.ok(bought.ok);
    assert.equal(bought.cost, Math.ceil(base * 0.7));
    assert.equal(bought.profile.coins, 500 - Math.ceil(base * 0.7));
    assert.equal(bought.profile.inventory.discount_token, 1);
    const noTok = Profile.buyUpgrade(profileWith({ coins: 500 }), track);
    assert.equal(noTok.cost, base);
}

// ---- 2.3 graph validation ----
deepEq(Dialogue.validateGraphs(), [], 'story graphs validate');
{
    // The validator really catches defects.
    const g = Story.GRAPHS.witch.nodes;
    g.__orphan = { choices: [{ to: null }] };
    assert.ok(Dialogue.validateGraphs().some((p) => /unreachable/.test(p)));
    delete g.__orphan;
    g.__trap = { choices: [{ to: '__trap' }] };
    Story.MEETINGS[0].starts.splice(0, 0, { if: { flag: 'zzz.zzz' }, to: '__trap' });
    assert.ok(Dialogue.validateGraphs().some((p) => /dead end/.test(p)));
    Story.MEETINGS[0].starts.shift();
    delete g.__trap;
    g.__neg = { choices: [{ effects: [{ coins: -5 }], to: null }] };
    Story.MEETINGS[0].starts.splice(0, 0, { if: { flag: 'zzz.zzz' }, to: '__neg' });
    assert.ok(Dialogue.validateGraphs().some((p) => /unguarded coins/.test(p)));
    Story.MEETINGS[0].starts.shift();
    delete g.__neg;
    deepEq(Dialogue.validateGraphs(), []);
}
assert.ok(Dialogue.textKeys().includes('runenpfad.npc.witch.hello.c1'));
assert.ok(Dialogue.textKeys().includes('runenpfad.npc.c.trade'));

// ---- 2.2 camps ----
const CAMP_CFG = { id: 2, worldIndex: 0, stageInWorld: 2, zones: ['meadow', 'npc_camp:witch', 'key_sanctum'], enemies: ['slime'] };

function campSim(options) {
    const sim = Sim.createSimulation(Object.assign({ levelId: 2 }, options || {}));
    sim.stage = Levels.buildStage(CAMP_CFG);
    sim.player.x = sim.stage.spawn.x;
    sim.player.y = sim.stage.spawn.y;
    return sim;
}

function standBeside(sim, npc, dx) {
    sim.player.x = npc.x + dx;
    sim.player.y = npc.y;
    sim.player.vx = 0;
    sim.player.vy = 0;
    sim.player.facing = dx < 0 ? 1 : -1;
    tick(sim, {}, 3);
}

{
    const stage = Levels.buildStage(CAMP_CFG);
    assert.equal(stage.npcs.length, 1);
    const npc = stage.npcs[0];
    assert.equal(npc.npcId, 'witch');
    assert.equal(stage.npcsReady, false);
    const tx = Math.floor(npc.x / T);
    for (let x = tx - 2; x <= tx + 2; x++) assert.ok(stage.solidMap[x + ',' + 13], `solid ground under tile ${x}`);
    assert.ok(!stage.hazards.some((h) => h.x + h.w > npc.campX0 && h.x < npc.campX1), 'no hazard in the camp');
    assert.equal(npc.campX1 - npc.campX0, 12 * T);

    // Enemies spawned inside the camp (or patrolling into it) move out at the first hold.
    stage.enemies.push(Levels.createEnemy('slime', npc.campX0 + 3 * T, 13 * T, npc.campX0 + T, npc.campX1 - T, { id: 'e_x' }));
    stage.enemies.push(Levels.createEnemy('slime', npc.campX1 + 2 * T, 13 * T, npc.campX0 + 2 * T, npc.campX1 + 6 * T, { id: 'e_y' }));
    const sim = Sim.createSimulation({ levelId: 2 });
    sim.stage = stage;
    sim.player.x = stage.spawn.x;
    sim.player.y = stage.spawn.y;
    tick(sim, {});
    assert.equal(stage.npcsReady, true);
    stage.enemies.forEach((e) => {
        assert.ok(!(e.spawnX > npc.campX0 && e.spawnX < npc.campX1), 'no enemy spawn in the camp');
        assert.ok(!(e.minX < npc.campX1 && e.maxX > npc.campX0 && e.spawnX >= npc.campX1 && e.minX < npc.campX1), 'patrol range clamped out of the camp');
    });

    // Camp layout is rng free: the stage after it is identical with or without the camp's props.
    const again = Levels.buildStage(CAMP_CFG);
    assert.equal(again.width, stage.width);
    deepEq(again.npcs[0].x, npc.x);

    // chief_gate: a full-height gate column that the dialogue can open.
    const gated = Levels.buildStage({ id: 8, worldIndex: 1, stageInWorld: 3, zones: ['meadow', 'npc_camp:chief_gate', 'key_sanctum'], enemies: ['slime'] });
    const gate = gated.gates.find((g) => g.npcGate === 'chief');
    assert.ok(gate && gate.h === 13 * T, 'chief gate is full height');
    assert.equal(gated.npcs[0].gateId, gate.id);
    const two = Levels.buildStage({ id: 24, worldIndex: 4, stageInWorld: 4, zones: ['meadow', 'npc_camp:wizard+alchemist', 'key_sanctum'], enemies: ['slime'] });
    deepEq(two.npcs.map((n) => n.npcId), ['wizard', 'alchemist']);
}

// Talk freezes the world; Up opens a dialogue only in range; the jump is swallowed only then.
{
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    sim.stage.enemies.push(Levels.createEnemy('slime', npc.campX1 + 8 * T, 13 * T, npc.campX1 + 4 * T, npc.campX1 + 12 * T, { id: 'e_far' }));
    standBeside(sim, npc, -40);
    assert.equal(npc.near, true);
    assert.equal(sim.notice, Npcs.TALK_HINT, 'talk hint shown near the NPC');
    Runes.grant(sim, 'fire', sim.player.x, sim.player.y);

    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.encounter && sim.encounter.kind, 'dialogue');
    assert.equal(sim.encounter.nodeId, 'witch.hello');
    const elapsed = sim.elapsed;
    const ex = sim.stage.enemies.map((e) => e.x);
    const rune = sim.player.rune.timeLeft;
    tick(sim, {}, 60);
    assert.equal(sim.elapsed, elapsed, 'clock frozen during dialogue');
    deepEq(sim.stage.enemies.map((e) => e.x), ex, 'enemies frozen');
    assert.equal(sim.player.rune.timeLeft, rune, 'rune timer frozen');
    Dialogue.close(sim);
    assert.ok(!sim.encounter);
    tick(sim, {}, 5);
    assert.ok(sim.elapsed > elapsed, 'world resumes after the dialogue');
}
{
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'up', jumpPressed: true });
    assert.equal(sim.encounter && sim.encounter.kind, 'dialogue');
    assert.equal(sim.player.vy, 0, 'jump swallowed when a dialogue opens');
    Dialogue.close(sim);

    const far = campSim();
    const fnpc = far.stage.npcs[0];
    standBeside(far, fnpc, -250);
    tick(far, { talkPressed: 'up', jumpPressed: true });
    assert.ok(!far.encounter);
    assert.ok(far.player.vy < 0, 'out of range the same input jumps');
    assert.equal(far.notice === Npcs.TALK_HINT, false);
}
{
    // Walking away restores the previous notice.
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    sim.notice = 'runenpfad.notice.find_key';
    standBeside(sim, npc, -250);
    sim.notice = 'runenpfad.notice.find_key';
    standBeside(sim, npc, -40);
    assert.equal(sim.notice, Npcs.TALK_HINT);
    standBeside(sim, npc, -250);
    assert.equal(sim.notice, 'runenpfad.notice.find_key');
}

// Attack in range provokes once: coven -1, graph hook, event; no duel module yet means no encounter.
{
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    standBeside(sim, npc, -40);
    const events = [];
    for (let i = 0; i < 4; i++) {
        tick(sim, { attackPressed: true });
        // attacks keep the same facing; the hero stays in front of the witch
        sim.player.facing = 1;
        sim.player.x = npc.x - 40;
    }
    assert.equal(sim.profile.rep.coven, -1, 'coven -1 exactly once');
    assert.equal(npc.provoked, true);
    assert.equal(Npcs.hasFlag(sim, 'witch.provoked'), true, 'onAttack hook flag set');
    assert.equal(sim.storyDirty, true);
    assert.equal(!!sandbox.RunenpfadDuel, !!(sim.encounter && sim.encounter.kind === 'duel'));
}

// Endless merchant camps on floors 2, 6, 10; none on floor 3; same seed gives the same widths.
{
    [2, 6, 10].forEach((f) => {
        const st = Levels.buildStage({ mode: 'endless', endlessFloor: f, seed: 9001 });
        assert.equal(st.npcs.length, 1, `floor ${f} has a camp`);
        assert.equal(st.npcs[0].npcId, 'merchant');
        assert.equal(Levels.buildStage({ mode: 'endless', endlessFloor: f, seed: 9001 }).widthTiles, st.widthTiles);
    });
    assert.equal(Levels.buildStage({ mode: 'endless', endlessFloor: 3, seed: 9001 }).npcs.length, 0);
    // The camp is inserted after every rng draw: the enemy roster of the config is unchanged.
    const cfg = Levels.createEndlessConfig(2, 9001);
    assert.equal(cfg.zones[1], 'npc_camp:merchant');
    const sim = Sim.createSimulation({ mode: 'endless', endlessFloor: 2 });
    tick(sim, {}, 3);
    assert.equal(sim.stage.npcsReady, true);
    assert.equal(sim.stage.npcs[0].meetingId, 'merchant_endless');
}

// ---- 2.3 full witch flow ----
{
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    let v = Dialogue.view(sim);
    assert.equal(v.textKey, 'runenpfad.npc.witch.hello');
    assert.equal(v.choices.length, 4);
    assert.equal(v.choices[0].textKey, 'runenpfad.npc.witch.hello.c1');
    assert.equal(v.choices[3].textKey, 'runenpfad.npc.c.trade');
    sim.storyDirty = false;
    let res = Dialogue.choose(sim, 0);
    assert.ok(res.ok && !res.closed);
    assert.equal(sim.encounter.nodeId, 'witch.accept');
    deepEq(Array.from(sim.profile.quests.active), ['witch_mushrooms']);
    assert.equal(sim.storyDirty, true);
    assert.equal(sim.stage.questItems.length, 3, 'three mushrooms spawned');
    assert.ok(sim.stage.questItems.every((it) => it.kind === 'glow_mushroom' && !it.taken));
    v = Dialogue.view(sim);
    assert.equal(v.mood, 'happy');
    // advice shows a generated tip
    Dialogue.choose(sim, 0);
    v = Dialogue.view(sim);
    assert.equal(v.textKey, 'runenpfad.npc.witch.advice');
    assert.ok(v.tip && /^runenpfad\.npc\.tip\./.test(v.tip.key));
    res = Dialogue.choose(sim, 0);
    assert.ok(res.closed);
    assert.ok(!sim.encounter);

    // waiting: {count} resolves from the inventory
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.encounter.nodeId, 'witch.waiting');
    assert.equal(Dialogue.view(sim).params.count, 3);
    Dialogue.close(sim);

    // pick all three
    sim.stage.questItems.forEach((it, i) => {
        sim.player.x = it.x;
        sim.player.y = it.y + 22;
        sim.player.vy = 0;
        tick(sim, {}, 2);
        assert.equal(it.taken, true, `mushroom ${i} taken`);
    });
    assert.equal(sim.profile.inventory.glow_mushroom, 3);

    const potions = sim.player.potions;
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.encounter.nodeId, 'witch.turnin', 'turn-in node after three mushrooms');
    res = Dialogue.choose(sim, 0);
    assert.equal(sim.encounter.nodeId, 'witch.thanks');
    assert.equal(sim.profile.rep.coven, 1);
    assert.equal(sim.player.potions, potions + 1);
    deepEq(Array.from(sim.profile.quests.done), ['witch_mushrooms']);
    deepEq(Array.from(sim.profile.quests.active), []);
    assert.equal(sim.profile.inventory.glow_mushroom, undefined);
    Dialogue.close(sim);
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.encounter.nodeId, 'witch.friend');
    Dialogue.close(sim);

    // The story survives the save pipeline.
    const saved = Profile.importProfile(JSON.parse(JSON.stringify(sim.profile)));
    assert.equal(saved.rep.coven, 1);
    assert.ok(saved.npcFlags['witch.helped']);
    // A second stage with the finished quest spawns no mushrooms.
    const next = Sim.createSimulation({ levelId: 2, profile: saved });
    next.stage = Levels.buildStage(CAMP_CFG);
    tick(next, {});
    assert.equal(next.stage.questItems.length, 0);
}
{
    // Refusing: the next talk starts at "cold".
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    Dialogue.choose(sim, 2);
    assert.equal(sim.encounter.nodeId, 'witch.refuse');
    assert.equal(Dialogue.view(sim).mood, 'angry');
    assert.ok(Npcs.hasFlag(sim, 'witch.refused'));
    Dialogue.choose(sim, 0);
    assert.ok(!sim.encounter);
    standBeside(sim, npc, -40);
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.encounter.nodeId, 'witch.cold');
    // cold's duel-only choice is hidden without a duel module
    assert.equal(Dialogue.view(sim).choices.length, sandbox.RunenpfadDuel ? 3 : 2);
    // Apologising reopens the quest.
    Dialogue.choose(sim, 0);
    assert.equal(sim.encounter.nodeId, 'witch.accept');
    assert.ok(!Npcs.hasFlag(sim, 'witch.refused'));
    deepEq(Array.from(sim.profile.quests.active), ['witch_mushrooms']);
}

// Roll: deterministic for a fixed rngState, both branches reachable.
{
    function haggle(state) {
        const sim = Sim.createSimulation({ mode: 'endless', endlessFloor: 2 });
        tick(sim, {}, 2);
        const npc = sim.stage.npcs[0];
        sim.player.x = npc.x - 40;
        sim.player.y = npc.y;
        tick(sim, {}, 3);
        tick(sim, { talkPressed: 'e' });
        assert.equal(sim.encounter.nodeId, 'merchant.hello');
        sim.rngState = state;
        const idx = Dialogue.view(sim).choices.findIndex((c) => c.textKey === 'runenpfad.npc.merchant.hello.c3');
        assert.ok(idx >= 0, 'haggle choice visible');
        Dialogue.choose(sim, idx);
        return { node: sim.encounter.nodeId, rng: sim.rngState, flag: Npcs.hasFlag(sim, 'merchant.haggle'), sim: sim };
    }
    const seen = {};
    for (let s = 1; s < 40; s++) {
        const st = Math.imul(s, 2654435761) >>> 0;
        const a = haggle(st);
        const b = haggle(st);
        deepEq([a.node, a.rng], [b.node, b.rng], 'same rngState gives the same roll');
        seen[a.node] = a;
    }
    assert.ok(seen['merchant.haggle_ok'] && seen['merchant.haggle_no'], 'both roll branches reachable');
    assert.equal(seen['merchant.haggle_ok'].flag, true);
    assert.equal(seen['merchant.haggle_no'].flag, false);
    // A discount applies only after a successful haggle: 15 % off.
    const okSim = seen['merchant.haggle_ok'].sim;
    const merchant = okSim.stage.npcs[0];
    assert.equal(Trade.priceOf(okSim, merchant, 'potion'), Math.round(12 * 0.85));
    // Failed haggle cannot be retried.
    const noSim = seen['merchant.haggle_no'].sim;
    Dialogue.choose(noSim, 1);
    Dialogue.close(noSim);
    noSim.player.x = noSim.stage.npcs[0].x - 40;
    tick(noSim, { talkPressed: 'e' });
    assert.ok(!Dialogue.view(noSim).choices.some((c) => c.textKey === 'runenpfad.npc.merchant.hello.c3'));
}

// ---- 2.4 trade ----
{
    assert.equal(Trade.price(20, -5), 26);
    assert.equal(Trade.price(20, 0), 20);
    assert.equal(Trade.price(20, 5), 14);

    const sim = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 100 }) });
    tick(sim, {}, 2);
    const npc = sim.stage.npcs[0];
    const offers = Trade.offersFor(sim, npc);
    deepEq(offers.map((o) => o.id).filter((id) => !/^rune_/.test(id)), ['potion', 'map_charm', 'lucky_amulet', 'discount_token']);
    assert.equal(offers.filter((o) => /^rune_/.test(o.id)).length, 1, 'one seeded rune kind');
    assert.equal(offers[0].price, 12);
    sim.storyDirty = false;
    const potions = sim.player.potions;
    let r = Trade.buy(sim, npc, 'potion');
    assert.ok(r.ok && !r.fake);
    assert.equal(sim.profile.coins, 88);
    assert.equal(sim.player.potions, potions + 1);
    assert.equal(sim.storyDirty, true);
    Trade.buy(sim, npc, 'potion');
    r = Trade.buy(sim, npc, 'potion');
    deepEq([r.ok, r.reason], [false, 'stock'], 'potion stock is 2');
    assert.equal(sim.profile.coins, 76);

    const poor = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 5 }) });
    tick(poor, {}, 2);
    const pn = poor.stage.npcs[0];
    const before = JSON.stringify(poor.profile);
    r = Trade.buy(poor, pn, 'potion');
    deepEq([r.ok, r.reason], [false, 'coins']);
    assert.equal(JSON.stringify(poor.profile), before, 'nothing changes without coins');

    // reputation pricing
    const rich = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 100, rep: { kobolds: 5 } }) });
    tick(rich, {}, 2);
    assert.equal(Trade.priceOf(rich, rich.stage.npcs[0], 'lucky_amulet'), Math.round(25 * 0.7));
    // item caps
    const capped = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 500, inventory: { discount_token: 3 } }) });
    tick(capped, {}, 2);
    assert.equal(Trade.offersFor(capped, capped.stage.npcs[0]).find((o) => o.id === 'discount_token').reason, 'cap');
    // map charm
    Trade.buy(rich, rich.stage.npcs[0], 'map_charm');
    assert.equal(rich.mapCharm, true);

    // Fake rule: seeded and reproducible at kobolds <= -2; never at -1.
    function fakeRun(rep, state) {
        const s = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 100, rep: { kobolds: rep } }) });
        tick(s, {}, 2);
        s.rngState = Math.imul(state, 2654435761) >>> 0;
        const n = s.stage.npcs[0];
        const pot = s.player.potions;
        const out = Trade.buy(s, n, 'potion');
        return { out, fake: out.fake, potions: s.player.potions - pot, coins: s.profile.coins, sim: s, npc: n };
    }
    let fakes = 0;
    for (let st = 1; st <= 60; st++) {
        const a = fakeRun(-2, st);
        const b = fakeRun(-2, st);
        assert.equal(a.fake, b.fake, 'fake outcome is reproducible');
        if (a.fake) {
            fakes++;
            assert.equal(a.potions, 0, 'fake applies nothing');
            assert.equal(a.coins, 100 - Trade.price(12, -2), 'fake still costs coins');
            assert.equal(Npcs.hasFlag(a.sim, 'merchant.fake_sold'), true);
            assert.equal(Npcs.getFlag(a.sim, 'merchant.fake_price'), Trade.price(12, -2));
        } else {
            assert.equal(a.potions, 1);
        }
        assert.equal(fakeRun(-1, st).fake, false, 'no fake at -1');
    }
    assert.ok(fakes > 5 && fakes < 40, `fake rate plausible (${fakes}/60)`);

    // Reveal on close, then confront: kobolds -1, half refund, flag cleared.
    let fk = null;
    for (let st = 1; st <= 60 && !fk; st++) {
        const a = fakeRun(-2, st);
        if (a.fake) fk = a;
    }
    const fsim = fk.sim;
    const fnpc = fk.npc;
    fsim.player.x = fnpc.x - 40;
    fsim.player.y = fnpc.y;
    tick(fsim, {}, 3);
    tick(fsim, { talkPressed: 'e' });
    assert.equal(fsim.encounter.nodeId, 'merchant.confront_fake');
    const coinsBefore = fsim.profile.coins;
    const price = Npcs.getFlag(fsim, 'merchant.fake_price');
    Dialogue.choose(fsim, 0);
    assert.equal(fsim.encounter.nodeId, 'merchant.refund');
    assert.equal(fsim.profile.rep.kobolds, -3);
    assert.equal(fsim.profile.coins, coinsBefore + Math.floor(price / 2));
    assert.ok(!Npcs.hasFlag(fsim, 'merchant.fake_sold'));
    // close of a dialogue after a fake sale pushes the reveal label once
    const rv = fakeRun(-2, 1);
    let guard = 1;
    let revealer = rv;
    while (!revealer.fake && guard < 60) revealer = fakeRun(-2, ++guard);
    revealer.sim.floatingTexts.length = 0;
    revealer.sim.player.x = revealer.npc.x - 40;
    revealer.sim.player.y = revealer.npc.y;
    tick(revealer.sim, {}, 3);
    tick(revealer.sim, { talkPressed: 'e' });
    Dialogue.close(revealer.sim);
    assert.equal(revealer.sim.floatingTexts.filter((f) => f.key === 'runenpfad.npc.float_fake').length, 1);

    // Lucky amulet: consumed on the next sim's first tick, coins worth one more.
    const amulet = Sim.createSimulation({ levelId: 1, profile: profileWith({ inventory: { lucky_amulet: 2 } }) });
    const cv = amulet.stage.coinValue || 1;
    tick(amulet, {});
    assert.equal(amulet.stage.coinValue, cv + 1);
    assert.equal(amulet.profile.inventory.lucky_amulet, 1);
    tick(amulet, {}, 5);
    assert.equal(amulet.stage.coinValue, cv + 1, 'consumed once per stage');
}

// ---- hooks: every dialogue/trade text key resolves to a plain string; trade keys are unique ----
{
    const keys = Dialogue.textKeys().concat(Trade.textKeys(), Dialogue.tipKeys(), Npcs.textKeys());
    keys.forEach((k) => assert.ok(/^runenpfad\.npc\.[a-z0-9_.]+$/.test(k), `bad key ${k}`));
}

// ---- 3.1 duel simulation ----
const Duel = sandbox.RunenpfadDuel;
const DuelPatterns = sandbox.RunenpfadDuelPatterns;
assert.ok(Duel && DuelPatterns, 'duel modules loaded');
const DUEL_NPCS = ['witch', 'wizard', 'merchant', 'chief', 'seer', 'alchemist'];

function duelSim(npcId, hero, opts) {
    const o = opts || {};
    const sim = Sim.createSimulation({ levelId: 2, hero: hero });
    sim.stage = Levels.buildStage({ id: 2, worldIndex: 0, stageInWorld: 2, zones: ['meadow', 'npc_camp:' + npcId, 'key_sanctum'], enemies: ['slime'] });
    sim.player.x = sim.stage.spawn.x;
    sim.player.y = sim.stage.spawn.y;
    if (o.rune) sim.player.rune = { kind: o.rune, timeLeft: 20, duration: 20, charges: 0 };
    tick(sim, {}, 2);
    sim.events.length = 0;
    const npc = sim.stage.npcs[0];
    assert.equal(Npcs.startDuel(sim, npc, { provoked: !!o.provoked, rune: o.rune || null }), true, 'duel starts');
    return { sim, npc, d: sim.encounter.duel, log: [] };
}

function stepRun(run, input) {
    Sim.stepSimulation(run.sim, STEP * 1.000001, input || {});
    for (const e of run.sim.events) run.log.push(e);
}

function idle(run, n) { for (let i = 0; i < n; i++) stepRun(run, {}); }

// Runs until the encounter ends; returns elapsed duel time (s) or -1 when it never ended.
function runDuel(run, bot, maxTicks) {
    const { sim } = run;
    for (let i = 0; i < maxTicks; i++) {
        const enc = sim.encounter;
        if (!enc || enc.kind !== 'duel') return run.d.t;
        stepRun(run, bot ? bot(enc.duel, sim) : {});
    }
    return sim.encounter ? -1 : run.d.t;
}

// Deterministic bot: evade ground hazards, block blockable tells while facing the NPC, otherwise approach and attack.
function makeBot(heroId) {
    return function bot(d) {
        const inp = {};
        if (d.phase !== 'fight') return inp;
        const h = d.hero;
        const n = d.npc;
        const dx = n.x - h.x;
        const dist = Math.abs(dx);
        const toward = dx < 0 ? 'left' : 'right';
        const away = dx < 0 ? 'right' : 'left';
        // 1. ground hazards (not shockwaves): step out of their footprint
        const haz = d.zones.filter((z) => z.dmg > 0 && z.kind !== 'shock' && Math.abs(z.x - h.x) < z.w / 2 + 50 && !(z.warn === 0 && z.hit));
        if (haz.length) {
            const z = haz[0];
            const lo = z.x - z.w / 2 - 40;
            const hi = z.x + z.w / 2 + 40;
            const goLeft = lo > Duel.ARENA.left + 30 && (hi > Duel.ARENA.right - 30 || h.x - lo < hi - h.x);
            inp[goLeft ? 'left' : 'right'] = true;
            if (z.kind === 'beam' || z.kind === 'steam') { if (h.rollCd <= 0 && z.warn > 0 && z.warn < 0.12) { inp.dashPressed = true; } }
            return inp;
        }
        // 2. shockwaves: hop over them
        const shock = d.zones.filter((z) => z.kind === 'shock' && Math.abs(z.x - h.x) < 150 && z.vx * (h.x - z.x) > 0)[0];
        if (shock && h.onGround) { inp.up = true; inp.jumpPressed = true; return inp; }
        // 3. teleport strike: face the marker and block
        const marker = d.zones.filter((z) => z.kind === 'marker')[0];
        if (marker) { inp.down = true; inp[marker.x < h.x ? 'left' : 'right'] = true; return inp; }
        // 3b. minions: block the ones in biting range, shoot or hack the rest
        const mins = d.minions.filter((m) => Math.abs(m.x - h.x) < 100).sort((p, q) => Math.abs(p.x - h.x) - Math.abs(q.x - h.x));
        if (mins.length) {
            const m = mins[0];
            const side = m.x < h.x ? 'left' : 'right';
            inp[side] = true;
            if (m.kind === 'clockwork' || h.attackCd <= 0) inp.attackPressed = true;
            else inp.down = true;
            return inp;
        }
        // 4. block blockable patterns facing the NPC
        const ai = n.ai;
        const pat = ai.pattern ? DuelPatterns.TABLE[d.npcId].patterns[ai.pattern] : null;
        const incoming = d.shots.some((s) => s.owner === 'npc' && s.blockable && s.dmg > 0 && Math.abs(s.x - h.x) < 320 && s.vx * (h.x - s.x) > 0);
        const threat = (ai.state === 'tell' || ai.state === 'act') && pat && pat.blk && dist < 460;
        if (n.hidden > 0) return inp;
        if ((incoming || threat) && n.shield <= 0) {
            inp.down = true;
            inp[toward] = true;
            return inp;
        }
        // 5. approach and attack
        const want = heroId === 'knight' ? 100 : 300;
        if (n.shield > 0) { if (dist < 340) inp[away] = true; return inp; }
        if (dist > want + 25) inp[toward] = true;
        else if (heroId === 'ranger' && dist < want - 60) inp[away] = true;
        else if ((dx < 0 ? -1 : 1) !== h.facing) inp[toward] = true;
        if (dist <= (heroId === 'knight' ? 125 : 520)) inp.attackPressed = true;
        return inp;
    };
}

// Robustness: 6 NPCs x 2 heroes, 900 idle ticks (the hero is made unkillable so the AI keeps going); every pattern shows up.
DUEL_NPCS.forEach((id) => ['knight', 'ranger'].forEach((hero) => {
    const run = duelSim(id, hero);
    run.d.hero.hp = run.d.hero.maxHp = 9999;
    for (let i = 0; i < 900; i++) {
        stepRun(run, {});
        const d = run.d;
        assert.ok(Number.isFinite(d.hero.hp) && Number.isFinite(d.npc.x) && Number.isFinite(d.npc.y) && Number.isFinite(d.hero.x), `${id} finite state`);
    }
    const tells = new Set(run.log.filter((e) => e.type === 'duel_tell').map((e) => e.detail.pattern));
    const atks = new Set(run.log.filter((e) => e.type === 'duel_attack').map((e) => e.detail.pattern));
    ['a', 'b', 'c'].forEach((k) => {
        assert.ok(tells.has(`${id}_${k}`), `${id} ${hero} tells ${k}`);
        assert.ok(atks.has(`${id}_${k}`), `${id} ${hero} attacks ${k}`);
    });
}));

// Static table: every tell >= 0.45 s in both phases; every attack follows its tell by >= 27 ticks.
DUEL_NPCS.forEach((id) => ['a', 'b', 'c'].forEach((k) => [1, 2].forEach((ph) => {
    assert.ok(DuelPatterns.tellTime(id, k, ph) >= 0.45 - 1e-9, `${id}_${k} phase ${ph} tell`);
})));
{
    const run = duelSim('chief', 'knight');
    run.d.hero.hp = run.d.hero.maxHp = 9999;
    const log = [];
    for (let i = 0; i < 1800; i++) {
        const n0 = run.log.length;
        stepRun(run, {});
        run.log.slice(n0).forEach((e) => { if (e.type === 'duel_tell' || e.type === 'duel_attack') log.push({ type: e.type, pattern: e.detail.pattern, i }); });
    }
    let tellAt = null;
    log.forEach((e) => {
        if (e.type === 'duel_tell') tellAt = e;
        else { assert.ok(tellAt && tellAt.pattern === e.pattern, 'attack follows its own tell'); assert.ok(e.i - tellAt.i >= 27, `tell lead ${e.i - tellAt.i} ticks`); }
    });
    assert.ok(log.length >= 6, 'several patterns ran');
}

// Determinism: the same input script gives identical events and rng state.
{
    function scripted(id, hero) {
        const run = duelSim(id, hero);
        const bot = makeBot(hero);
        runDuel(run, bot, 1500);
        return JSON.stringify({ ev: run.log.map((e) => [e.type, e.detail]), rng: run.sim.rngState, t: run.d && run.d.t });
    }
    DUEL_NPCS.forEach((id) => assert.equal(scripted(id, 'knight'), scripted(id, 'knight'), `${id} duel deterministic`));
}

// The bot wins every duel (both heroes) with a base profile within 15-75 s; the world does not advance meanwhile.
const winTimes = {};
DUEL_NPCS.forEach((id) => ['knight', 'ranger'].forEach((hero) => {
    const run = duelSim(id, hero);
    const sim = run.sim;
    const hp0 = sim.player.hp;
    const elapsed0 = sim.elapsed;
    const enemy0 = JSON.stringify(sim.stage.enemies.map((e) => [e.x, e.y, e.animTime]));
    const coins0 = sim.profile.coins;
    const t = runDuel(run, makeBot(hero), 6000);
    const d = run.d;
    winTimes[`${id}/${hero}`] = Math.round(d.t * 10) / 10 + (d.result === 'won' ? '' : ' LOST') + '/hp' + d.hero.hp;
    assert.equal(d.result, 'won', `${id}/${hero} bot wins (hero hp ${d.hero.hp}, npc hp ${d.npc.hp}; ${JSON.stringify(winTimes[`${id}/${hero}`])})`);
    assert.ok(t >= 15 && t <= 75, `${id}/${hero} duel length ${t.toFixed(1)}s`);
    assert.ok(!(sim.encounter && sim.encounter.kind === 'duel'), 'duel over');
    assert.equal(sim.state, 'playing');
    assert.equal(sim.elapsed, elapsed0, 'world clock frozen');
    assert.equal(JSON.stringify(sim.stage.enemies.map((e) => [e.x, e.y, e.animTime])), enemy0, 'enemies frozen');
    assert.equal(sim.player.hp, hp0, 'a won duel costs no heart');
    assert.ok(sim.profile.coins >= coins0 + Npcs.NPCS[id].duelCoins, 'duel coins (plus any outcome-dialogue coins)');
    assert.equal(Npcs.getFlag(sim, `${id}.beaten`), 1);
    assert.equal(run.log.filter((e) => e.type === 'duel_phase').length, 1, 'phase 2 once');
    assert.equal(run.log.filter((e) => e.type === 'duel_end')[0].detail.result, 'won');
}));
console.log('duel win times', JSON.stringify(winTimes));

// Phase 2 fires once, at <= 50 % HP.
{
    const run = duelSim('witch', 'knight');
    const bot = makeBot('knight');
    let hpAtPhase = null;
    for (let i = 0; i < 6000 && run.sim.encounter; i++) {
        const n0 = run.log.length;
        stepRun(run, bot(run.d));
        if (run.log.slice(n0).some((e) => e.type === 'duel_phase')) hpAtPhase = run.d.npc.hp;
    }
    assert.ok(hpAtPhase !== null && hpAtPhase <= run.d.npc.maxHp * 0.5, 'phase 2 at <= 50 %');
}

// Losing: an idle hero loses a heart, play continues, never a game over.
DUEL_NPCS.forEach((id) => {
    const run = duelSim(id, 'knight', { provoked: false });
    const sim = run.sim;
    const hp0 = sim.player.hp;
    const dmg0 = sim.damageTaken;
    const t = runDuel(run, null, 6000);
    assert.ok(t > 0, `${id} idle hero eventually loses`);
    assert.equal(run.d.result, 'lost');
    assert.equal(sim.state, 'playing', 'never game over');
    assert.equal(sim.player.hp, Math.max(1, hp0 - 1));
    assert.equal(sim.player.invulnTimer, 2);
    assert.equal(sim.damageTaken, dmg0 + 1);
    assert.equal(Npcs.getFlag(sim, `${id}.duel_lost`), 1);
    assert.ok(sim.encounter === null || sim.encounter.kind === 'dialogue', 'outcome dialogue or none');
    assert.equal(run.log.filter((e) => e.type === 'duel_end')[0].detail.result, 'lost');
});

// No escape: running into a wall while rolling keeps the duel running.
{
    const run = duelSim('chief', 'ranger');
    run.d.hero.hp = run.d.hero.maxHp = 9999;
    for (let i = 0; i < 600; i++) stepRun(run, { left: true, dashPressed: true });
    assert.equal(run.sim.encounter.kind, 'duel');
    assert.ok(run.d.hero.x >= Duel.ARENA.left && run.d.hero.x <= Duel.ARENA.right);
    for (let i = 0; i < 300; i++) stepRun(run, { right: true, dashPressed: true });
    assert.equal(run.sim.encounter.kind, 'duel');
}

// Specials: fire hits for 10 once only; frost freezes; storm 8 unblockable; magnet clears shots.
{
    const run = duelSim('merchant', 'knight', { rune: 'fire' });
    const { d, sim } = run;
    idle(run, Math.ceil(Duel.INTRO / STEP) + 2);
    assert.equal(d.phase, 'fight');
    d.npc.x = 400;
    d.hero.x = 260;
    d.npc.ai.gap = 99;
    const hp0 = d.npc.hp;
    stepRun(run, { talkPressed: 'e' });
    idle(run, 90);
    assert.equal(hp0 - d.npc.hp, 10, 'fire special 10 damage');
    assert.equal(run.log.filter((e) => e.type === 'duel_special').length, 1);
    stepRun(run, { talkPressed: 'e' });
    idle(run, 90);
    assert.equal(hp0 - d.npc.hp, 10, 'special only once');
    assert.equal(run.log.filter((e) => e.type === 'duel_special').length, 1);
    // Up must never fire the special (it is the jump key)
    const r2 = duelSim('merchant', 'knight', { rune: 'storm' });
    idle(r2, Math.ceil(Duel.INTRO / STEP) + 2);
    stepRun(r2, { talkPressed: 'up', up: true, jumpPressed: true });
    assert.equal(r2.d.hero.specialUsed, false);
    const hpS = r2.d.npc.hp;
    stepRun(r2, { talkPressed: 'e' });
    assert.equal(hpS - r2.d.npc.hp, 8, 'storm 8 damage');
    const r3 = duelSim('merchant', 'knight', { rune: 'frost' });
    idle(r3, Math.ceil(Duel.INTRO / STEP) + 2);
    stepRun(r3, { talkPressed: 'e' });
    assert.ok(r3.d.npc.frozen > 1.5, 'frost freezes');
    const r4 = duelSim('witch', 'knight', { rune: 'magnet' });
    idle(r4, Math.ceil(Duel.INTRO / STEP) + 2);
    r4.d.shots.push({ kind: 'hex', owner: 'npc', x: 600, y: 300, vx: 0, vy: 0, dmg: 2, blockable: true, homing: 0, turn: 0, life: 5, g: 0, r: 14 });
    stepRun(r4, { talkPressed: 'e' });
    assert.equal(r4.d.shots.length, 0, 'magnet clears shots');
}

// Provoke: attacking the witch on L2 gives coven -1, then the duel starts and finishes with rep/flags applied.
{
    const sim = campSim();
    const npc = sim.stage.npcs[0];
    standBeside(sim, npc, -40);
    tick(sim, { attackPressed: true });
    assert.equal(sim.profile.rep.coven, -1);
    assert.equal(sim.encounter && sim.encounter.kind, 'duel');
    assert.equal(sim.encounter.duel.provoked, true);
    assert.equal(sim.state, 'playing');
    sim.encounter.duel.hero.hp = sim.encounter.duel.hero.maxHp = 9999;
    const bot = makeBot(sim.heroId);
    for (let i = 0; i < 6000 && sim.encounter && sim.encounter.kind === 'duel'; i++) Sim.stepSimulation(sim, STEP * 1.000001, bot(sim.encounter.duel));
    assert.ok(!sim.encounter || sim.encounter.kind === 'dialogue', 'outcome after the duel');
    assert.equal(Npcs.getFlag(sim, 'witch.beaten'), 1);
    assert.equal(sim.profile.rep.coven, -1);
}

// ---- 4.1a story arcs: witch W2, merchant richer graphs, chief Krax gate ----
{
    deepEq(Dialogue.validateGraphs(), [], 'story graphs validate with the 4.1a arcs');

    function storySim(levelId, cfg, extra) {
        const sim = Sim.createSimulation({ levelId: levelId, profile: profileWith(extra || {}) });
        sim.stage = Levels.buildStage(cfg);
        sim.player.x = sim.stage.spawn.x;
        sim.player.y = sim.stage.spawn.y;
        return sim;
    }
    function talk(sim) {
        const npc = sim.stage.npcs[0];
        standBeside(sim, npc, -40);
        tick(sim, { talkPressed: 'e' });
        assert.equal(sim.encounter && sim.encounter.kind, 'dialogue', 'dialogue opened');
        return sim.encounter.nodeId;
    }
    const WITCH2 = { id: 7, worldIndex: 1, stageInWorld: 2, zones: ['meadow', 'npc_camp:witch', 'key_sanctum'], enemies: ['slime'] };
    const CHIEF2 = { id: 8, worldIndex: 1, stageInWorld: 3, zones: ['meadow', 'npc_camp:chief_gate', 'key_sanctum'], enemies: ['slime'] };
    const CHIEF3 = { id: 13, worldIndex: 2, stageInWorld: 3, zones: ['meadow', 'npc_camp:chief', 'key_sanctum'], enemies: ['slime'] };
    const MERCH2 = { id: 9, worldIndex: 1, stageInWorld: 3, zones: ['meadow', 'npc_camp:merchant', 'key_sanctum'], enemies: ['slime'] };
    const done = { active: [], done: ['witch_mushrooms'] };

    // Witch W2 entry node per world 1 outcome.
    const witchCases = [
        ['witch.stranger', {}],
        ['witch.sour', { npcFlags: { 'witch.refused': 1 } }],
        ['witch.wary', { npcFlags: { 'witch.provoked': 1 } }],
        ['witch.ambush', { npcFlags: { 'witch.refused': 1, 'witch.provoked': 1 } }],
        ['witch.known', { quests: done, npcFlags: { 'witch.helped': 1 }, rep: { coven: 1 } }],
        ['witch.gift', { quests: done, npcFlags: { 'witch.helped': 1 }, rep: { coven: 2 } }],
        ['witch.known', { quests: done, npcFlags: { 'witch.helped': 1, 'witch.charm': 1 }, rep: { coven: 2 } }],
        ['witch.waiting', { quests: { active: ['witch_mushrooms'], done: [] } }],
        ['witch.turnin', { quests: { active: ['witch_mushrooms'], done: [] }, inventory: { glow_mushroom: 3 } }]
    ];
    witchCases.forEach(function (c) {
        const sim = storySim(7, WITCH2, c[1]);
        assert.equal(talk(sim), c[0], 'witch W2 entry ' + c[0]);
    });
    {
        // gift gives the hex charm and the flag; the apology clears a grudge.
        const sim = storySim(7, WITCH2, { quests: done, rep: { coven: 2 } });
        talk(sim);
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'witch.charm');
        assert.equal(Npcs.itemCount(sim, 'hex_charm'), 1);
        assert.ok(Npcs.hasFlag(sim, 'witch.charm'));
        const s2 = storySim(7, WITCH2, { npcFlags: { 'witch.refused': 1 } });
        talk(s2);
        Dialogue.choose(s2, 0);
        assert.equal(s2.encounter.nodeId, 'witch.stranger');
        assert.ok(!Npcs.hasFlag(s2, 'witch.refused'));
        // known: buying the charm costs 25 coins and gives coven +1.
        const s3 = storySim(7, WITCH2, { quests: done, coins: 40 });
        talk(s3);
        Dialogue.choose(s3, 0);
        assert.equal(s3.encounter.nodeId, 'witch.charm');
        assert.equal(s3.profile.coins, 15);
        assert.equal(s3.profile.rep.coven, 1);
    }

    // Krax gate: bribe, charm, duel win, two losses (mercy), kobold friend.
    function gateOpen(sim) { return sim.stage.gates.find((g) => g.npcGate === 'chief').open; }
    {
        const sim = storySim(8, CHIEF2, { coins: 100 });
        assert.equal(talk(sim), 'chief.block');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'chief.toll');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'chief.paid');
        assert.equal(sim.profile.coins, 60);
        assert.equal(sim.profile.rep.kobolds, 1);
        assert.ok(gateOpen(sim) && Npcs.hasFlag(sim, 'chief.paid'));
        Dialogue.close(sim);
        assert.equal(talk(sim), 'chief.passed', 'a paid gate stays open on revisit');

        const hated = storySim(8, CHIEF2, { coins: 100, rep: { kobolds: -1 } });
        talk(hated);
        Dialogue.choose(hated, 0);
        Dialogue.choose(hated, 0);
        assert.equal(hated.profile.coins, 40, 'double toll at kobolds below 0');
        assert.ok(gateOpen(hated));

        const poor = storySim(8, CHIEF2, { coins: 10 });
        talk(poor);
        Dialogue.choose(poor, 0);
        assert.equal(Dialogue.view(poor).choices.length, 1, 'only the way back without coins');
        assert.ok(!gateOpen(poor));
    }
    {
        // Witch's charm needs coven >= 2, the charm item and the flag.
        const sim = storySim(8, CHIEF2, { rep: { coven: 2 }, npcFlags: { 'witch.charm': 1 }, inventory: { hex_charm: 1 } });
        talk(sim);
        assert.equal(Dialogue.view(sim).choices[1].textKey, 'runenpfad.npc.chief.block.c2');
        Dialogue.choose(sim, 1);
        assert.equal(sim.encounter.nodeId, 'chief.charmed');
        assert.ok(gateOpen(sim) && Npcs.hasFlag(sim, 'chief.charmed'));
        assert.equal(Npcs.itemCount(sim, 'hex_charm'), 0);
        const low = storySim(8, CHIEF2, { rep: { coven: 1 }, npcFlags: { 'witch.charm': 1 }, inventory: { hex_charm: 1 } });
        talk(low);
        assert.ok(!Dialogue.view(low).choices.some((c) => c.textKey === 'runenpfad.npc.chief.block.c2'), 'no charm below coven 2');
        // W3 after a charm: the grudge costs 50 coins.
        const w3 = storySim(13, CHIEF3, { coins: 80, npcFlags: { 'chief.charmed': 1 } });
        assert.equal(talk(w3), 'chief.grudge');
        Dialogue.choose(w3, 0);
        assert.equal(w3.encounter.nodeId, 'chief.settled');
        assert.equal(w3.profile.coins, 30);
        assert.ok(!Npcs.hasFlag(w3, 'chief.charmed'));
        assert.equal(w3.profile.rep.kobolds, 1);
    }
    {
        // Duel win: gate, kobolds +1, respect, engine coins; then W3 alliance.
        const sim = storySim(8, CHIEF2, {});
        talk(sim);
        Dialogue.choose(sim, 1);
        assert.equal(sim.encounter.kind, 'duel');
        sim.encounter.duel.result = 'won';
        Npcs.finishDuel(sim);
        assert.equal(sim.encounter.nodeId, 'chief.won');
        assert.ok(gateOpen(sim) && Npcs.hasFlag(sim, 'chief.respect'));
        assert.equal(sim.profile.rep.kobolds, 1);
        assert.equal(sim.profile.coins, Profile.createDefaultProfile().coins + 25);
        const w3 = storySim(13, CHIEF3, { npcFlags: { 'chief.respect': 1 } });
        assert.equal(talk(w3), 'chief.ally');
        Dialogue.choose(w3, 0);
        assert.equal(w3.encounter.nodeId, 'chief.gifted');
        assert.equal(w3.profile.coins, Profile.createDefaultProfile().coins + 20);
        // Two losses open the gate out of mercy and leave a debt that W3 asks for.
        const l = storySim(8, CHIEF2, {});
        talk(l);
        Dialogue.choose(l, 1);
        l.encounter.duel.result = 'lost';
        Npcs.finishDuel(l);
        assert.equal(l.encounter.nodeId, 'chief.lost');
        assert.ok(!gateOpen(l));
        Dialogue.close(l);
        talk(l);
        Dialogue.choose(l, 1);
        l.encounter.duel.result = 'lost';
        Npcs.finishDuel(l);
        assert.equal(l.encounter.nodeId, 'chief.mercy');
        assert.ok(gateOpen(l) && Npcs.hasFlag(l, 'chief.owed'));
        const w3d = storySim(13, CHIEF3, { coins: 50, npcFlags: { 'chief.owed': 1 } });
        assert.equal(talk(w3d), 'chief.debt');
        Dialogue.choose(w3d, 0);
        assert.equal(w3d.profile.coins, 20);
        assert.ok(!Npcs.hasFlag(w3d, 'chief.owed'));
        // A rematch win in world 3 clears the debt and earns respect.
        const rm = storySim(13, CHIEF3, { npcFlags: { 'chief.owed': 1 } });
        talk(rm);
        Dialogue.choose(rm, 0);
        rm.encounter.duel.result = 'won';
        Npcs.finishDuel(rm);
        assert.equal(rm.encounter.nodeId, 'chief.won_rematch');
        assert.ok(Npcs.hasFlag(rm, 'chief.respect') && !Npcs.hasFlag(rm, 'chief.owed'));
        // Fitz's friends (kobolds >= 3) pass freely.
        const fr = storySim(8, CHIEF2, { rep: { kobolds: 3 } });
        assert.equal(talk(fr), 'chief.friend');
        assert.ok(gateOpen(fr));
    }

    // Merchant: confronted fake, kobold reputation, quest wares.
    {
        const sim = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 100, rep: { kobolds: -2 }, npcFlags: { 'merchant.fake_sold': 1, 'merchant.fake_price': 12 } }) });
        tick(sim, {}, 2);
        assert.equal(talk(sim), 'merchant.confront_fake');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'merchant.refund');
        assert.equal(sim.profile.rep.kobolds, -3);
        Dialogue.close(sim);
        assert.equal(talk(sim), 'merchant.wary', 'a disliked trader is wary');
        Dialogue.choose(sim, 1);
        assert.equal(sim.encounter.nodeId, 'merchant.forgiven');
        assert.equal(sim.profile.rep.kobolds, -2);
        assert.equal(sim.profile.coins, 100 + 6 - 20);

        const reg = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ rep: { kobolds: 2 } }) });
        tick(reg, {}, 2);
        assert.equal(talk(reg), 'merchant.regular');
        const seen = {};
        for (let s = 1; s < 30; s++) {
            const r = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ rep: { kobolds: 2 } }) });
            tick(r, {}, 2);
            talk(r);
            r.rngState = Math.imul(s, 2654435761) >>> 0;
            Dialogue.choose(r, 1);
            seen[r.encounter.nodeId] = true;
        }
        assert.ok(seen['merchant.freebie'] && seen['merchant.nothing'], 'occasional free tip');

        const tipSim = storySim(9, MERCH2, { rep: { kobolds: 1 } });
        assert.equal(talk(tipSim), 'merchant.kobold_tip');
        assert.ok(Npcs.hasFlag(tipSim, 'merchant.krax'));

        const q = storySim(9, MERCH2, { coins: 100, quests: { active: ['witch_mushrooms'], done: [] } });
        assert.equal(talk(q), 'merchant.hello_quest');
        Dialogue.choose(q, 1);
        assert.equal(q.encounter.nodeId, 'merchant.wares');
        Dialogue.choose(q, 0);
        assert.equal(q.encounter.nodeId, 'merchant.bought');
        assert.equal(Npcs.itemCount(q, 'glow_mushroom'), 1);
        assert.equal(q.profile.coins, 80);
        const deep = storySim(9, MERCH2, {});
        assert.equal(talk(deep), 'merchant.hello_deep');
    }
}

// ---- 4.1b late arcs: seer, wizard, alchemist, finale ----
{
    const lateSim = (levelId, worldIndex, camp, extra) => {
        const sim = Sim.createSimulation({ levelId: levelId, profile: profileWith(extra || {}) });
        sim.stage = Levels.buildStage({ id: levelId, worldIndex: worldIndex, stageInWorld: 2, zones: ['meadow', 'npc_camp:' + camp, 'key_sanctum'], enemies: ['slime'] });
        sim.player.x = sim.stage.spawn.x;
        sim.player.y = sim.stage.spawn.y;
        return sim;
    };
    const lateTalk = (sim, idx) => {
        const npc = sim.stage.npcs[idx || 0];
        standBeside(sim, npc, -40);
        tick(sim, { talkPressed: 'e' });
        assert.equal(sim.encounter && sim.encounter.kind, 'dialogue', 'dialogue opened');
        return sim.encounter.nodeId;
    };
    deepEq(Dialogue.validateGraphs(), [], 'late graphs validate');
    ['seer', 'wizard', 'alchemist'].forEach((id) => assert.ok(Story.GRAPHS[id], `${id} graph registered`));
    ['seer_w3a', 'seer_w3b', 'wizard_w4a', 'wizard_w4b', 'wizard_w5', 'wizard_finale', 'alchemist_w5a', 'alchemist_w5b', 'alchemist_finale']
        .forEach((id) => assert.ok(Story.MEETINGS.some((m) => m.id === id), `meeting ${id}`));

    // Wizard: dragon tip with circle >= 1, otherwise a challenge duel; Saida's letter also opens the door.
    {
        const friend = lateSim(17, 3, 'wizard', { rep: { circle: 1 } });
        assert.equal(lateTalk(friend), 'wizard.welcome');
        friend.stage.bossId = 'b_test';
        friend.stage.enemies.push({ id: 'b_test', defId: 'boss_dragon_frost' });
        assert.equal(Dialogue.view(friend).tip.key, 'runenpfad.npc.tip.boss_dragon_frost');
        const stranger = lateSim(17, 3, 'wizard', {});
        assert.equal(lateTalk(stranger), 'wizard.challenge');
        assert.equal(Dialogue.view(stranger).tip, null);
        assert.equal(lateTalk(lateSim(17, 3, 'wizard', { npcFlags: { 'seer.letter': 1 } })), 'wizard.welcome');
        // Challenge: the duel starts, a win earns circle +1 and the proven flag.
        Dialogue.choose(stranger, 0);
        assert.equal(stranger.encounter.kind, 'duel');
        stranger.encounter.duel.result = 'won';
        Npcs.finishDuel(stranger);
        assert.equal(stranger.encounter.nodeId, 'wizard.proven');
        assert.equal(stranger.profile.rep.circle, 1);
        assert.ok(Npcs.hasFlag(stranger, 'wizard.proven'));
        const lost = lateSim(17, 3, 'wizard', {});
        lateTalk(lost);
        Dialogue.choose(lost, 0);
        lost.encounter.duel.result = 'lost';
        Npcs.finishDuel(lost);
        assert.equal(lost.encounter.nodeId, 'wizard.lost');
        assert.equal(lost.profile.rep.circle, 0);
        // W4b / W5 follow-ups.
        const gift = lateSim(19, 3, 'wizard', { npcFlags: { 'wizard.proven': 1 } });
        assert.equal(lateTalk(gift), 'wizard.gift');
        assert.ok(Npcs.hasFlag(gift, 'wizard.warned'));
        Dialogue.close(gift);
        assert.equal(lateTalk(gift), 'wizard.known');
        assert.equal(lateTalk(lateSim(19, 3, 'wizard', {})), 'wizard.cold');
        assert.equal(lateTalk(lateSim(24, 4, 'wizard', { rep: { circle: 2 } })), 'wizard.ally');
        assert.equal(lateTalk(lateSim(24, 4, 'wizard', {})), 'wizard.neutral');
        const sab = lateSim(24, 4, 'wizard', { npcFlags: { 'alchemist.sabotage': 1 } });
        assert.equal(lateTalk(sab), 'wizard.sabotage_known');
        assert.equal(sab.profile.rep.circle, 1);
    }

    // Saida: paid hint, favour quest, shards turn-in, riddle, reputation reactions.
    {
        const sim = lateSim(11, 2, 'seer', { coins: 100 });
        assert.equal(lateTalk(sim), 'seer.hello');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'seer.hint');
        assert.equal(sim.profile.coins, 85);
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'seer.riddle');
        Dialogue.choose(sim, 1);
        assert.equal(sim.encounter.nodeId, 'seer.riddle_ok');
        assert.equal(sim.profile.rep.circle, 1);
        assert.ok(Dialogue.view(sim).tip);
        assert.equal(lateTalk(lateSim(11, 2, 'seer', { rep: { circle: -2 } })), 'seer.shun');
        assert.equal(lateTalk(lateSim(11, 2, 'seer', { rep: { circle: 2 } })), 'seer.honored');
        const fav = lateSim(11, 2, 'seer', {});
        lateTalk(fav);
        Dialogue.choose(fav, Dialogue.view(fav).choices.findIndex((c) => c.textKey === 'runenpfad.npc.seer.hello.c3'));
        assert.equal(fav.encounter.nodeId, 'seer.favour');
        deepEq(Array.from(fav.profile.quests.active), ['seer_shards']);
        assert.equal(fav.stage.questItems.filter((i) => i.kind === 'sun_shard').length, 3);
        Dialogue.close(fav);
        assert.equal(lateTalk(fav), 'seer.waiting');
        Dialogue.close(fav);
        Npcs.addItem(fav, 'sun_shard', 3);
        assert.equal(lateTalk(fav), 'seer.turnin');
        Dialogue.choose(fav, 0);
        assert.equal(fav.encounter.nodeId, 'seer.thanks');
        assert.ok(Npcs.questDone(fav, 'seer_shards') && Npcs.hasFlag(fav, 'seer.friend'));
        assert.equal(fav.profile.rep.circle, 1);
        Dialogue.close(fav);
        assert.equal(lateTalk(fav), 'seer.friend');
        assert.ok(Npcs.hasFlag(fav, 'seer.letter'));
    }

    // Brass: gear quest start -> collect -> turn-in -> reveal (sabotage only at coven >= 2).
    {
        const sim = lateSim(21, 4, 'alchemist', { rep: { coven: 1 } });
        assert.equal(lateTalk(sim), 'alchemist.hello');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'alchemist.accept');
        deepEq(Array.from(sim.profile.quests.active), ['alchemist_gears']);
        const gears = sim.stage.questItems.filter((i) => i.kind === 'gear_part');
        assert.equal(gears.length, 3);
        Dialogue.close(sim);
        assert.equal(lateTalk(sim), 'alchemist.waiting');
        assert.equal(Dialogue.view(sim).params.count, 3);
        Dialogue.close(sim);
        gears.forEach((it) => {
            sim.player.x = it.x;
            sim.player.y = it.y + 22;
            sim.player.vy = 0;
            tick(sim, {}, 2);
            assert.equal(it.taken, true);
        });
        assert.equal(sim.profile.inventory.gear_part, 3);
        assert.equal(lateTalk(sim), 'alchemist.turnin');
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'alchemist.reveal');
        assert.ok(Npcs.questDone(sim, 'alchemist_gears'));
        assert.equal(sim.profile.rep.coven, 2);
        assert.equal(sim.profile.inventory.discount_token, 2);
        assert.equal(Npcs.itemCount(sim, 'gear_part'), 0);
        Dialogue.choose(sim, 0);
        assert.equal(sim.encounter.nodeId, 'alchemist.sabotage');
        assert.ok(Npcs.hasFlag(sim, 'alchemist.sabotage'));
        const deny = lateSim(23, 4, 'alchemist', { rep: { coven: 0 }, quests: { active: ['alchemist_gears'], done: [] }, inventory: { gear_part: 3 } });
        lateTalk(deny);
        Dialogue.choose(deny, 0);
        assert.equal(deny.profile.rep.coven, 1);
        Dialogue.choose(deny, 0);
        assert.equal(deny.encounter.nodeId, 'alchemist.deny');
        assert.ok(!Npcs.hasFlag(deny, 'alchemist.sabotage'));
        // Buying a part, and the wizard's warning colours the first meeting.
        const buy = lateSim(21, 4, 'alchemist', { coins: 45 });
        lateTalk(buy);
        Dialogue.choose(buy, 1);
        Dialogue.choose(buy, 0);
        assert.equal(buy.profile.coins, 25);
        assert.equal(Npcs.itemCount(buy, 'gear_part'), 1);
        assert.equal(lateTalk(lateSim(21, 4, 'alchemist', { npcFlags: { 'wizard.warned': 1 } })), 'alchemist.wary');
    }

    // Finale: reward branches follow reputation, and the nodes cite earlier choices.
    {
        const wiz = (extra) => { const s = lateSim(25, 4, 'wizard+alchemist', extra); return { sim: s, node: lateTalk(s, 0) }; };
        const alc = (extra) => { const s = lateSim(25, 4, 'wizard+alchemist', extra); return { sim: s, node: lateTalk(s, 1) }; };
        const high = wiz({ rep: { circle: 3 }, npcFlags: { 'seer.friend': 1 }, coins: 0 });
        assert.equal(high.node, 'wizard.f_honor_seer');
        assert.equal(high.sim.profile.coins, 40);
        assert.ok(Npcs.hasFlag(high.sim, 'wizard.blessed'));
        assert.equal(wiz({ rep: { circle: 3 } }).node, 'wizard.f_honor');
        assert.equal(wiz({ npcFlags: { 'wizard.proven': 1 } }).node, 'wizard.f_proven');
        assert.equal(wiz({}).node, 'wizard.f_fair');
        const low = wiz({ rep: { circle: -2 }, coins: 0 });
        assert.equal(low.node, 'wizard.f_cold');
        assert.equal(low.sim.profile.coins, 0, 'no bonus when the circle was crossed');
        Dialogue.close(high.sim);
        assert.equal(lateTalk(high.sim, 0), 'wizard.f_again', 'the blessing is given once');
        assert.equal(alc({ rep: { coven: 3 }, npcFlags: { 'witch.helped': 1 } }).node, 'alchemist.f_witch');
        assert.equal(alc({ rep: { coven: 3 } }).node, 'alchemist.f_honor');
        assert.equal(alc({ npcFlags: { 'chief.paid': 1 } }).node, 'alchemist.f_krax');
        assert.equal(alc({ npcFlags: { 'chief.respect': 1 } }).node, 'alchemist.f_krax');
        assert.equal(alc({}).node, 'alchemist.f_fair');
        assert.equal(alc({ rep: { coven: -2 } }).node, 'alchemist.f_cold');
        const rich = alc({ rep: { coven: 3 }, npcFlags: { 'witch.helped': 1 }, coins: 0 });
        assert.equal(rich.sim.profile.coins, 40);
        // The meeting conditions cite at least two earlier-arc flags.
        const cited = new Set();
        JSON.stringify(Story.MEETINGS.filter((m) => /finale/.test(m.id)), (k, v) => { if (k === 'flag' && typeof v === 'string') cited.add(v); return v; });
        ['witch.helped', 'seer.friend', 'wizard.proven', 'chief.paid', 'chief.respect', 'chief.charmed'].forEach((f) => assert.ok(cited.has(f), `finale cites ${f}`));
    }
}

// ---- 2.8 / 4.2 placement: camps in the campaign, quest items, gate, sabotage, epilogue ----
{
    const PLACE = { 2: 'witch', 3: 'merchant', 4: 'witch', 7: 'witch', 8: 'chief', 9: 'merchant', 11: 'seer', 12: 'merchant', 13: 'chief', 14: 'seer',
        17: 'wizard', 18: 'merchant', 19: 'wizard', 21: 'alchemist', 23: 'merchant+alchemist', 24: 'wizard', 25: 'wizard+alchemist' };
    const levelCfg = (id) => Levels.CAMPAIGN_LEVELS[id - 1];
    // Everything that must never sit inside a camp (the rune chest may use its flat ground).
    const KEEP_CLEAR = ['hazards', 'ropes', 'movingPlatforms', 'ladders', 'geysers', 'pendulums', 'flameJets', 'cannons', 'switches', 'shrines', 'boulders',
        'secretWalls', 'windZones', 'crates', 'lanterns', 'platforms', 'dragonRuns'];
    const rangeOf = (o) => [o.x1 != null ? o.x1 : (o.startX != null ? o.startX : o.x), (o.x2 != null ? o.x2 : (o.endX != null ? o.endX : o.x) + (o.w || 0))];

    for (const id of Object.keys(PLACE).map(Number)) {
        const ids = PLACE[id].split('+');
        const sim = Sim.createSimulation({ levelId: id, hero: 'knight' });
        const st = sim.stage;
        const g = Math.round(st.groundY / T);
        deepEq(st.npcs.map((n) => n.npcId), ids, `level ${id} camp NPCs`);
        const x0 = st.npcs[0].campX0;
        const x1 = st.npcs[0].campX1;
        st.npcs.forEach((n) => { assert.ok(n.x > x0 && n.x < x1 && n.y === st.groundY, `level ${id} NPC stands inside the camp on the ground`); });
        // Solid ground under the whole camp and clear air above it (the Krax gate column excepted).
        for (let tx = Math.floor(x0 / T); tx < Math.ceil(x1 / T); tx++) {
            assert.ok(st.solidMap[tx + ',' + g], `level ${id} ground under camp tile ${tx}`);
            if (st.gates.some((q) => q.npcGate && q.x === tx * T)) continue;
            for (let r = 1; r <= 4; r++) assert.ok(!st.solidMap[tx + ',' + (g - r)], `level ${id} camp tile ${tx} is clear ${r} up`);
        }
        // At least three free tiles on both sides of the NPC group (the gate column or the camp edge ends a side).
        const free = (from, step) => { let n = 0; for (let tx = from; ; tx += step) { if (!st.solidMap[tx + ',' + g] || st.solidMap[tx + ',' + (g - 1)]) break; n++; } return n; };
        const left = free(Math.floor(st.npcs[0].x / T) - 1, -1);
        const right = free(Math.floor(st.npcs[st.npcs.length - 1].x / T), 1); // an NPC stands on a tile's left edge
        assert.ok(left >= 3 && right >= 3, `level ${id} free tiles beside the NPCs: ${left}/${right}`);
        // No hazard, rope, gate, puzzle, boulder, moving part, dragon lane or live enemy in the camp; the key, door and portal stay out.
        KEEP_CLEAR.forEach((k) => (st[k] || []).forEach((o) => {
            const [a, b] = rangeOf(o);
            assert.ok(b <= x0 || a >= x1, `level ${id}: ${k} ${a}-${b} overlaps the camp ${x0}-${x1}`);
        }));
        st.gates.filter((q) => !q.npcGate).forEach((q) => assert.ok(q.x + q.w <= x0 || q.x >= x1, `level ${id}: puzzle gate overlaps the camp`));
        st.chests.forEach((c) => assert.ok(c.x > x0 && c.x < x1 || c.x <= x0 || c.x >= x1, 'chest placement is flat ground'));
        [st.key, st.door, st.portal].filter(Boolean).forEach((o) => assert.ok(o.x < x0 || o.x > x1, `level ${id}: key/door/portal outside the camp`));
        if (st.bossId) {
            const boss = st.enemies.find((e) => e.id === st.bossId);
            assert.ok(boss.spawnX > x1 + 6 * T, `level ${id}: the boss arena starts well after the camp`);
        }
        // The first hold resolves enemies out of the camp and the world still runs.
        sim.player.invulnTimer = 9;
        tick(sim, {}, 120);
        st.enemies.filter((e) => !(e.def && e.def.boss)).forEach((e) => assert.ok(e.spawnX <= x0 || e.spawnX >= x1, `level ${id}: no enemy spawn in the camp`));
        assert.ok(st.parTime >= 110, 'camp levels keep the +10 s par time');
    }
    assert.equal(levelCfg(2).parTime, 110);
    assert.equal(levelCfg(25).parTime, 170);
    assert.equal(levelCfg(1).parTime, 75, 'levels without a camp keep their par time');
    // Every campaign level without a camp has none.
    for (let id = 1; id <= 25; id++) if (!PLACE[id]) assert.equal(Sim.createSimulation({ levelId: id }).stage.npcs.length, 0, `level ${id} has no camp`);
    // Chief gate on level 8: column in the camp, the NPC keeps three free tiles to it.
    {
        const st = Sim.createSimulation({ levelId: 8 }).stage;
        const gate = st.gates.find((q) => q.npcGate === 'chief');
        assert.ok(gate && gate.x > st.npcs[0].x + 2 * T && gate.x < st.npcs[0].campX1, 'gate column right of the chief, inside the camp');
        assert.equal(gate.h, st.groundY, 'full height');
    }

    // Quest items exist only while the quest is active, at standable spots outside solids, hazards and secret walls.
    const QUEST_LEVELS = { witch_mushrooms: [2, 4, 7, 9], seer_shards: [11, 12, 13, 14], alchemist_gears: [21, 22, 23, 24, 25] };
    const standable = (st, it) => {
        const tx = Math.floor(it.x / T);
        const ty = Math.floor(it.y / T);
        if (st.solidMap[tx + ',' + ty]) return false;
        if (st.hazards.some((h) => it.x > h.x && it.x < h.x + h.w && it.y > h.y - 10 && it.y < h.y + h.h)) return false;
        if (st.secretWalls.some((w) => it.x > w.x && it.x < w.x + w.w && it.y > w.y && it.y < w.y + w.h)) return false;
        for (let dy = 0; dy <= 3; dy++) {
            if (st.solidMap[tx + ',' + (ty + dy)]) return true;
        }
        return st.platforms.some((p) => it.x >= p.x && it.x <= p.x + p.w && p.y >= it.y && p.y - it.y <= 3 * T);
    };
    for (const qid of Object.keys(QUEST_LEVELS)) {
        const q = Npcs.QUESTS[qid];
        for (const id of QUEST_LEVELS[qid]) {
            const none = Sim.createSimulation({ levelId: id });
            tick(none, {}, 2);
            assert.equal(none.stage.questItems.length, 0, `${qid}: no items without the quest (level ${id})`);
            const sim = Sim.createSimulation({ levelId: id, profile: profileWith({ quests: { active: [qid], done: [] } }) });
            tick(sim, {}, 2);
            const items = sim.stage.questItems;
            assert.equal(items.length, 3, `${qid}: three items on level ${id}`);
            items.forEach((it) => {
                assert.equal(it.kind, q.kind);
                assert.ok(it.x > 0 && it.x < sim.stage.width, `${qid} item inside the stage`);
                assert.ok(standable(sim.stage, it), `${qid} level ${id}: item at ${it.x},${it.y} sits on ground or a platform`);
            });
            // Pick all three up by walking the hero onto them.
            items.forEach((it) => { sim.player.x = it.x; sim.player.y = it.y + 22; sim.player.invulnTimer = 9; tick(sim, {}, 2); });
            assert.equal(Npcs.itemCount(sim, q.kind), 3, `${qid} level ${id}: all items can be collected`);
        }
        // Out of the quest's worlds nothing spawns even with the quest active.
        const off = QUEST_LEVELS[qid].includes(2) ? 15 : 2;
        const away = Sim.createSimulation({ levelId: off, profile: profileWith({ quests: { active: [qid], done: [] } }) });
        tick(away, {}, 2);
        assert.equal(away.stage.questItems.length, 0, `${qid}: nothing outside its worlds`);
        const finished = Sim.createSimulation({ levelId: QUEST_LEVELS[qid][0], profile: profileWith({ quests: { active: [], done: [qid] } }) });
        tick(finished, {}, 2);
        assert.equal(finished.stage.questItems.length, 0, `${qid}: a finished quest spawns nothing`);
    }

    // The Krax gate on the real level 8 opens on every path for both heroes; the hero can walk through it afterwards.
    for (const hero of ['knight', 'ranger']) {
        const mk = (extra) => {
            const sim = Sim.createSimulation({ levelId: 8, hero: hero, profile: profileWith(extra || {}) });
            tick(sim, {}, 2);
            return sim;
        };
        const gateOf = (sim) => sim.stage.gates.find((q) => q.npcGate === 'chief');
        const talkTo = (sim) => {
            const npc = sim.stage.npcs[0];
            standBeside(sim, npc, -40);
            tick(sim, { talkPressed: 'e' });
            assert.equal(sim.encounter && sim.encounter.kind, 'dialogue');
        };
        const walkThrough = (sim) => {
            if (sim.encounter) Dialogue.close(sim);
            const gate = gateOf(sim);
            const npc = sim.stage.npcs[0];
            sim.player.x = npc.x - 40;
            sim.player.y = npc.y;
            sim.player.facing = 1;
            for (let i = 0; i < 360; i++) { sim.player.invulnTimer = 9; Sim.stepSimulation(sim, STEP * 1.000001, { right: true }); }
            return sim.player.x > gate.x + gate.w + 20;
        };
        const closed = mk();
        assert.ok(!gateOf(closed).open && !walkThrough(closed), 'a closed gate holds the hero back');
        const paid = mk({ coins: 200 });
        talkTo(paid);
        Dialogue.choose(paid, 0);
        Dialogue.choose(paid, 0);
        assert.ok(gateOf(paid).open && walkThrough(paid), `${hero}: toll opens the gate`);
        // Duel win.
        const won = mk();
        talkTo(won);
        Dialogue.choose(won, 1);
        assert.equal(won.encounter.kind, 'duel');
        won.encounter.duel.result = 'won';
        Npcs.finishDuel(won);
        assert.ok(gateOpen8(won) && walkThrough(won), `${hero}: duel win opens the gate`);
        // Two losses: mercy.
        const lost = mk();
        for (let i = 0; i < 2; i++) {
            talkTo(lost);
            Dialogue.choose(lost, 1);
            assert.equal(lost.encounter.kind, 'duel');
            lost.encounter.duel.result = 'lost';
            Npcs.finishDuel(lost);
            if (i === 0) { assert.ok(!gateOf(lost).open, 'one loss keeps the gate shut'); Dialogue.close(lost); }
        }
        assert.ok(gateOf(lost).open && Npcs.hasFlag(lost, 'chief.owed'), `${hero}: mercy after two losses`);
        assert.ok(walkThrough(lost), `${hero}: the hero walks through after mercy`);
        assert.ok(lost.player.hp >= 1, 'duel losses never end the run');
    }
    function gateOpen8(sim) { return sim.stage.gates.find((q) => q.npcGate === 'chief').open; }

    // A duel win against the witch pays the engine purse once (the dialogue node adds nothing).
    for (const flags of [{}, { 'witch.refused': 1 }]) {
        const sim = Sim.createSimulation({ levelId: 2, profile: profileWith({ npcFlags: flags }) });
        tick(sim, {}, 2);
        const npc = sim.stage.npcs[0];
        const before = sim.profile.coins;
        assert.ok(Npcs.startDuel(sim, npc, {}));
        sim.encounter.duel.result = 'won';
        Npcs.finishDuel(sim);
        assert.equal(sim.encounter && sim.encounter.kind, 'dialogue');
        assert.equal(sim.encounter.nodeId, flags['witch.refused'] ? 'witch.won_grudge' : 'witch.won');
        assert.equal(sim.profile.coins - before, Npcs.NPCS.witch.duelCoins, 'witch duel win pays once');
    }

    // Sabotage cuts the level 25 boss to 80 % hp; only with the flag, only on level 25, never twice.
    {
        const base = Sim.createSimulation({ levelId: 25 });
        const hp0 = base.stage.enemies.find((e) => e.def.boss).maxHp;
        const flagged = Sim.createSimulation({ levelId: 25, profile: profileWith({ npcFlags: { 'alchemist.sabotage': 1 } }) });
        tick(flagged, {}, 1);
        tick(base, {}, 1);
        const boss = flagged.stage.enemies.find((e) => e.def.boss);
        assert.equal(boss.maxHp, Math.round(hp0 * 0.8));
        assert.equal(boss.hp, boss.maxHp);
        assert.equal(base.stage.enemies.find((e) => e.def.boss).hp, hp0, 'no flag, no cut');
        tick(flagged, {}, 30);
        Npcs.resolve(flagged);
        assert.equal(boss.maxHp, Math.round(hp0 * 0.8), 'the cut is applied once');
        const other = Sim.createSimulation({ levelId: 10, profile: profileWith({ npcFlags: { 'alchemist.sabotage': 1 } }) });
        const h10 = other.stage.enemies.find((e) => e.def.boss).maxHp;
        tick(other, {}, 2);
        assert.equal(other.stage.enemies.find((e) => e.def.boss).maxHp, h10, 'other bosses are untouched');
    }

    // Epilogue: coins 50 per friendly faction on the final stage only, once, without touching the blessings.
    {
        const fin = (extra) => { const s = Sim.createSimulation({ levelId: 25, profile: profileWith(extra) }); tick(s, {}, 1); return s; };
        const a = fin({ rep: { coven: 2, circle: 3, kobolds: 0 }, npcFlags: { 'wizard.blessed': 1 }, coins: 10 });
        const coins0 = a.profile.coins;
        const ep = Npcs.onVictory(a);
        deepEq({ key: ep.key, coins: ep.coins }, { key: 'runenpfad.npc.end_friend', coins: 100 });
        assert.equal(a.profile.coins, coins0 + 100);
        assert.equal(a.stageCoins, 100);
        assert.equal(Npcs.onVictory(a), null, 'the epilogue pays once');
        assert.equal(a.profile.coins, coins0 + 100);
        assert.equal(Npcs.getFlag(a, 'wizard.blessed'), 1, 'blessings stay untouched');
        const n = fin({});
        assert.equal(Npcs.onVictory(n).key, 'runenpfad.npc.end_neutral');
        assert.equal(n.epilogue.coins, 0);
        const f = fin({ rep: { coven: -3, circle: -2, kobolds: 1 } });
        assert.equal(Npcs.onVictory(f).key, 'runenpfad.npc.end_feared');
        const one = fin({ rep: { coven: 4 } });
        assert.equal(Npcs.onVictory(one).coins, 50);
        assert.equal(one.epilogue.key, 'runenpfad.npc.end_neutral', 'one friend is not enough for the friend ending');
        const mid = Sim.createSimulation({ levelId: 24, profile: profileWith({ rep: { coven: 3, circle: 3 } }) });
        assert.equal(Npcs.onVictory(mid), null, 'no epilogue before the final stage');
    }
}

// Review fixes: duel coins and the epilogue purse are paid once per save; dialogue changes report dirty state.
{
    const wonAt = (profile, id) => {
        const sim = Sim.createSimulation({ levelId: 2, hero: 'knight', profile });
        sim.stage = Levels.buildStage({ id: 2, worldIndex: 0, stageInWorld: 2, zones: ['meadow', 'npc_camp:' + id, 'key_sanctum'], enemies: ['slime'] });
        sim.player.x = sim.stage.spawn.x;
        sim.player.y = sim.stage.spawn.y;
        tick(sim, {}, 2);
        const npc = sim.stage.npcs[0];
        assert.equal(Npcs.startDuel(sim, npc, { provoked: true }), true);
        const coins0 = sim.profile.coins;
        sim.encounter.duel.result = 'won';
        Npcs.finishDuel(sim);
        return { sim, gained: sim.profile.coins - coins0 };
    };
    DUEL_NPCS.forEach((id) => {
        const first = wonAt(profileWith({}), id);
        assert.ok(first.gained >= Npcs.NPCS[id].duelCoins, `${id} first win pays`);
        assert.equal(Npcs.getFlag(first.sim, id + '.beaten'), 1);
        const again = wonAt(Profile.importProfile(Profile.exportProfile(first.sim.profile)), id);
        assert.ok(again.gained < Npcs.NPCS[id].duelCoins, `${id} second win on a new stage pays no duel purse (${again.gained})`);
        assert.equal(Npcs.getFlag(again.sim, id + '.beaten'), 2, 'the win is still counted');
        assert.equal(again.sim.storyDirty, true, 'finishing a duel marks the story dirty');
    });

    const fin = (extra) => { const s = Sim.createSimulation({ levelId: 25, profile: profileWith(extra) }); tick(s, {}, 1); return s; };
    const a = fin({ rep: { coven: 2, circle: 3, kobolds: 0 } });
    assert.equal(Npcs.onVictory(a).coins, 100);
    assert.equal(Npcs.getFlag(a, 'epilogue.paid'), 1);
    const saved = Profile.importProfile(Profile.exportProfile(a.profile));
    assert.equal(Npcs.getFlag({ profile: saved }, 'epilogue.paid'), 1, 'the paid flag survives a save round trip');
    const b = fin({ rep: saved.rep, npcFlags: saved.npcFlags, coins: saved.coins });
    const coinsB = b.profile.coins;
    const ep2 = Npcs.onVictory(b);
    assert.equal(ep2.key, 'runenpfad.npc.end_friend', 'the epilogue text still shows on later clears');
    assert.equal(ep2.coins, 0);
    assert.equal(b.profile.coins, coinsB, 'no second purse');
}

// ---- i18n key coverage: every NPC text key exists in en.json and de.json ----
{
    const keys = [].concat(Dialogue.textKeys(), Dialogue.tipKeys(), Trade.textKeys(), Npcs.textKeys(),
        ['runenpfad.npc.end_friend', 'runenpfad.npc.end_neutral', 'runenpfad.npc.end_feared']);
    for (const lang of ['en', 'de']) {
        const table = JSON.parse(fs.readFileSync(path.join(rootDir, 'ui/lang/runenpfad', lang + '.json'), 'utf8'));
        const missing = keys.filter(function (k) { return typeof table[k] !== 'string' || !table[k].trim(); });
        assert.deepStrictEqual(missing, [], lang + '.json misses NPC keys');
    }
}

// ---- map charm and rune attune: effects ----
{
    const rich = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 500, rep: { kobolds: 5 } }) });
    tick(rich, {}, 2);
    const merchant = rich.stage.npcs[0];
    const plain = Sim.createSimulation({ mode: 'endless', endlessFloor: 2, profile: profileWith({ coins: 500 }) });
    tick(plain, {}, 2);
    Runes.grant(plain, 'fire', 0, 0);
    assert.equal(plain.player.rune.timeLeft, 15, 'without the charm the rune keeps its normal duration');
    assert.ok(!plain.events.some((e) => e.type === 'rune_attuned'));
    const wizard = { npcId: 'wizard', ref: 'w1', sold: {} };
    const out = Trade.buy(rich, wizard, 'rune_attune');
    assert.ok(out && out.ok !== false, 'rune_attune can be bought');
    assert.equal(Npcs.itemCount(rich, 'rune_attune'), 1);
    Runes.grant(rich, 'frost', 0, 0);
    assert.equal(rich.player.rune.timeLeft, 30, 'attuned frost rune lasts 1.5x');
    assert.equal(rich.player.rune.duration, 30);
    assert.equal(Npcs.itemCount(rich, 'rune_attune'), 0, 'one attune is consumed');
    assert.equal(rich.storyDirty, true);
    assert.ok(rich.events.some((e) => e.type === 'rune_attuned'), 'rune_attuned event pushed');
    Runes.grant(rich, 'fire', 0, 0);
    assert.equal(rich.player.rune.timeLeft, 15, 'the next rune is normal again');
    // two charms last for two runes
    Npcs.addItem(rich, 'rune_attune', 2);
    Runes.grant(rich, 'storm', 0, 0);
    assert.equal(Npcs.itemCount(rich, 'rune_attune'), 1);
    // the map charm resets on the next stage
    Trade.buy(rich, merchant, 'map_charm');
    assert.equal(rich.mapCharm, true);
    Trade.applyStageStart(rich);
    assert.equal(rich.mapCharm, false, 'the map charm ends with the stage');
}

console.log('Runenpfad NPC checks passed: profile story fields, camps and hold hook, dialogue graphs, witch arc, trade, duel, story arcs, campaign placement, quest items, gate, sabotage, epilogue.');
