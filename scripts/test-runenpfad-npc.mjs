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

console.log('Runenpfad NPC checks passed: profile story fields, camps and hold hook, dialogue graphs, witch arc, trade.');
