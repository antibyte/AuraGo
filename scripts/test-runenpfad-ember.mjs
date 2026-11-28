import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const dir = path.join(rootDir, 'ui/js/desktop/apps/runenpfad');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Canonical sim-core order of the round 11 contract (files that do not exist yet are skipped).
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things',
    'deep-mobs', 'brandur', 'interior', 'places', 'folk', 'folk-story', 'journal-story', 'ember-story', 'ember-story-folk', 'village-rooms', 'place-rooms-deep', 'deeps-rooms',
    'village', 'cutscenes', 'chronicle', 'ember', 'crafts', 'dice', 'bounties', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(dir, `${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const { RunenpfadSim: Sim, RunenpfadProfile: Profile, RunenpfadNpcs: Npcs, RunenpfadDialogue: Dialogue, RunenpfadPlaceRooms: Rooms,
    RunenpfadEmber: Ember, RunenpfadChronicle: Chron, RunenpfadCutscenes: Cut, RunenpfadCombat: Combat } = sandbox;
const Story = sandbox.RunenpfadNpcStory;
const Interior = sandbox.RunenpfadInterior;
const Places = sandbox.RunenpfadPlaces;
const Levels = sandbox.RunenpfadLevels;
assert.ok(Sim && Profile && Npcs && Dialogue && Rooms && Ember && Chron && Cut && Combat && Places && Levels, 'modules loaded');

// The hub test needs the two new doors; stand-ins copy an existing house until WP-C and WP-F land.
const stubbed = [];
['deeps', 'edda_house'].forEach((id) => {
    if (!Rooms.PLACES[id]) {
        Rooms.PLACES[id] = Object.assign({}, Rooms.PLACES.tavern, { id, kind: id === 'deeps' ? 'cave' : 'house' });
        stubbed.push(id);
    }
});

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
function prof(extra) { return Profile.importProfile(Object.assign({ v: 1 }, extra || {})); }
function newSim(opts) {
    const sim = Sim.createSimulation(opts);
    tick(sim, {}, 30);
    return sim;
}
function hubSim(profile, hero) { return newSim({ mode: 'campaign', levelId: 0, hero: hero || 'knight', profile: prof(profile) }); }
function deq(a, b, msg) { assert.deepEqual(JSON.parse(JSON.stringify(a)), JSON.parse(JSON.stringify(b)), msg); }
function evTypes(events) { return events.map((e) => e.type); }

// ---- 1. step, fate, hubScene ----
check('step, fate and hubScene tables', () => {
    const p = (seen, flags, lv) => ({ seen: seen || [], npcFlags: flags || {}, unlockedLevel: lv || 1 });
    assert.equal(Ember.step(p()), 0);
    assert.equal(Ember.step(p(['dr_tremor'])), 1);
    assert.equal(Ember.step(p(['dr_tremor', 'dr_night'])), 2);
    assert.equal(Ember.step(p(['dr_night'], { 'dr.fate': 1 })), 3);
    assert.equal(Ember.step(p([], { 'dr.fate': 2 })), 5);
    assert.equal(Ember.step(p([], { 'dr.fate': 3 })), 5);
    assert.equal(Ember.step(null), 0);
    assert.equal(Ember.fate(p()), null);
    assert.equal(Ember.fate(p([], { 'dr.fate': 1 })), 'beaten');
    assert.equal(Ember.fate(p([], { 'dr.fate': 2 })), 'slain');
    assert.equal(Ember.fate(p([], { 'dr.fate': 3 })), 'sealed');
    assert.equal(Ember.hubScene(p([], {}, 5)), null);
    assert.equal(Ember.hubScene(p([], {}, 6)), 'dr_tremor');
    assert.equal(Ember.hubScene(p(['dr_tremor'], {}, 10)), null);
    assert.equal(Ember.hubScene(p(['dr_tremor'], {}, 11)), 'dr_night');
    assert.equal(Ember.hubScene(p([], {}, 11)), 'dr_tremor');
    assert.equal(Ember.hubScene(p(['dr_tremor', 'dr_night'], {}, 25)), null);
});

// ---- 2. hub scene requests ----
check('hub asks for dr_tremor, then dr_night', () => {
    const sim = hubSim({ unlockedLevel: 11 });
    assert.equal(sim.uiRequest, 'scene:dr_tremor');
    sim.uiRequest = null;
    tick(sim, {}, 10);
    assert.equal(sim.uiRequest, null, 'asked only once');
    sim.profile.seen = ['dr_tremor'];
    tick(sim, {}, 2);
    assert.equal(sim.uiRequest, 'scene:dr_night');
    sim.uiRequest = null;
    sim.profile.seen = ['dr_night', 'dr_tremor'];
    tick(sim, {}, 5);
    assert.equal(sim.uiRequest, null);
    assert.equal(hubSim({ unlockedLevel: 5 }).uiRequest, null, 'below level 6 nothing happens');
    const lvl = newSim({ mode: 'campaign', levelId: 1, profile: prof({ unlockedLevel: 11 }) });
    assert.equal(lvl.uiRequest, null, 'only the hub asks');
});

// ---- 3. doors ----
check('hub doors and the deeps door gate', () => {
    const sim = hubSim();
    deq(sim.stage.doors.map((d) => d.placeId), ['deeps', 'edda_house', 'tavern', 'shop', 'hall', 'smithy', 'arch']);
    const deeps = sim.stage.doors[0];
    assert.equal(deeps.off, true);
    deq(JSON.parse(JSON.stringify(deeps.facade)), { clip: 'fac-deep.f0', h: 170, dx: 0 });
    const edda = sim.stage.doors[1];
    assert.equal(edda.off, false);
    assert.equal(edda.facade.clip, 'fac-deep.f3');
    for (let i = 1; i < sim.stage.doors.length; i++) assert.ok(sim.stage.doors[i].x - sim.stage.doors[i - 1].x >= 9 * 48 - 1, 'doors at least 9 tiles apart');
    function stand(door) {
        sim.player.x = door.x; sim.player.y = door.y; sim.player.vx = 0; sim.player.vy = 0; sim.player.onGround = true;
        tick(sim, {}, 2);
    }
    stand(deeps);
    assert.equal(deeps.near, false, 'off door is not near');
    assert.equal(sim.interior, null);
    tick(sim, { talkPressed: 'e' });
    assert.equal(sim.interior, null, 'off door does not open');
    sim.profile.seen = ['dr_tremor', 'dr_night'];
    stand(deeps);
    assert.equal(deeps.off, false);
    assert.equal(deeps.near, true);
    tick(sim, { talkPressed: 'e' });
    assert.ok(sim.interior, 'enterable at step 2');
    assert.equal(sim.interior.placeId, 'deeps');
});

// ---- 4. applyHub per step ----
check('applyHub per step', () => {
    const sim = hubSim({ unlockedLevel: 3 });
    const st = sim.stage;
    const well = st.hubProps.filter((p) => p.id === 'well')[0];
    const barn = st.hubProps.filter((p) => p.id === 'barn')[0];
    const deeps = st.doors.filter((d) => d.placeId === 'deeps')[0];
    assert.ok(well && barn, 'well and barn props');
    assert.equal(well.clip, 'fac-village-b.f3');
    assert.equal(barn.clip, 'fac-deep.f2');
    assert.ok(!st.hubProps.some((p) => p.clip === 'fac-village-b.f1'), 'the old cottage decor is gone');
    const baseBarks = st.barks.map((b) => b.lines.slice());
    const set = (seen, flags) => { sim.profile.seen = seen; Object.keys(flags || {}).forEach((k) => Npcs.setFlag(sim, k, flags[k])); tick(sim, {}, 2); };
    assert.equal(well.hidden, false); assert.equal(well.steam, false); assert.equal(deeps.off, true);
    set(['dr_tremor']);
    assert.equal(well.hidden, false); assert.equal(well.steam, true); assert.equal(deeps.off, true); assert.equal(barn.clip, 'fac-deep.f2');
    deq(st.barks[0].lines, ['7', '8', '9'].map((n) => 'runenpfad.village.bark.' + n));
    deq(st.barks[1].lines, ['10', '11', '12'].map((n) => 'runenpfad.village.bark.' + n));
    set(['dr_tremor', 'dr_night']);
    assert.equal(well.hidden, true); assert.equal(deeps.off, false); assert.equal(deeps.steam, true); assert.equal(barn.clip, 'fac-deep.f1');
    set(['dr_tremor', 'dr_night'], { 'dr.fate': 1 });
    assert.equal(barn.clip, 'fac-deep.f1'); assert.equal(deeps.steam, true);
    Npcs.setFlag(sim, 'dr.fate', 3); tick(sim, {}, 2);
    assert.equal(well.hidden, true); assert.equal(deeps.off, false); assert.equal(deeps.steam, false); assert.equal(barn.clip, 'fac-deep.f2');
    deq(st.barks[0].lines, ['13', '14', '15'].map((n) => 'runenpfad.village.bark.' + n));
    deq(st.barks[1].lines, ['16', '17', '18'].map((n) => 'runenpfad.village.bark.' + n));
    assert.ok(st.barks.every((b) => b.line < b.lines.length));
    Ember.applyHub(st, 0);
    deq(st.barks.map((b) => b.lines), baseBarks, 'step 0 restores the original barks');
    const bare = Levels.buildStage(0);
    assert.ok(bare.hub && bare.doors.length === 7, 'built hub carries the doors');
});

// ---- 5. tablets ----
check('tablets: once, bitmask, event, endless refusal', () => {
    const sim = hubSim();
    assert.equal(Chron.TABLET_TOTAL, 6);
    assert.equal(Chron.tabletCount(sim.profile), 0);
    sim.events.length = 0;
    assert.equal(Chron.giveTablet(sim, 3), true);
    assert.equal(Chron.giveTablet(sim, 3), false);
    assert.equal(Chron.giveTablet(sim, 7), false);
    assert.equal(Chron.giveTablet(sim, 1), true);
    assert.equal(sim.profile.npcFlags['dr.tabs'], 0b000101);
    assert.ok(Chron.hasTablet(sim.profile, 1) && Chron.hasTablet(sim.profile, 3) && !Chron.hasTablet(sim.profile, 2));
    assert.equal(Chron.tabletCount(sim.profile), 2);
    const ev = sim.events.filter((e) => e.type === 'in_tablet');
    assert.equal(ev.length, 2);
    deq(JSON.parse(JSON.stringify(ev[0].detail)), { tablet: 3, first: true });
    assert.equal(sim.notice, 'runenpfad.notice.tablet_found');
    const rows = Chron.tabletList(sim.profile, (k) => k);
    assert.equal(rows.length, 6);
    deq(rows.map((r) => r.have), [true, false, true, false, false, false]);
    deq(rows.map((r) => r.placeId), ['hall', 'hall', 'deeps', 'deeps', 'deeps', 'deeps']);
    assert.equal(rows[0].titleKey, 'runenpfad.tablet.1.title');
    const end = newSim({ mode: 'endless', endlessFloor: 1, profile: prof() });
    assert.equal(Chron.giveTablet(end, 4), false, 'endless refuses');
});

// ---- 6. ember effects ----
check('ember effects: slay and seal', () => {
    const sim = hubSim({ coins: 10 });
    const hp0 = sim.player.maxHp;
    Dialogue.applyEffects(sim, null, [{ ember: 'slay' }]);
    assert.equal(Ember.fate(sim.profile), null, 'slay needs a beaten Brandur');
    assert.equal(sim.profile.coins, 10);
    Ember.setFate(sim, 'beaten');
    assert.equal(Ember.setFate(sim, 'beaten'), false, 'no repeat');
    sim.player.hp = hp0 - 1;
    const stage0 = sim.stageCoins;
    Dialogue.applyEffects(sim, null, [{ ember: 'slay' }]);
    assert.equal(Ember.fate(sim.profile), 'slain');
    assert.equal(sim.profile.coins, 310);
    assert.equal(sim.stageCoins, stage0 + 300);
    assert.equal(sim.player.maxHp, hp0 + 1);
    assert.equal(sim.player.hp, hp0);
    assert.equal(Profile.heroStats('knight', sim.profile).maxHp, Profile.heroStats('knight', prof()).maxHp + 1);
    assert.equal(sim.notice, 'runenpfad.notice.ember_heart');
    assert.ok(sim.events.some((e) => e.type === 'dr_outcome' && e.detail.fate === 'slain'));
    assert.equal(Ember.setFate(sim, 'sealed'), false, 'a slain fate never changes');
    Dialogue.applyEffects(sim, null, [{ ember: 'slay' }]);
    assert.equal(sim.profile.coins, 310, 'slay pays once');

    const s2 = hubSim();
    Ember.setFate(s2, 'beaten');
    Dialogue.applyEffects(s2, null, [{ ember: 'seal' }]);
    assert.equal(Ember.fate(s2.profile), 'beaten', 'seal without the bell does nothing');
    Npcs.setFlag(s2, 'dr.bell', 1);
    Dialogue.applyEffects(s2, null, [{ ember: 'seal' }]);
    assert.equal(Ember.fate(s2.profile), 'sealed');
    assert.equal(s2.notice, 'runenpfad.notice.blessing_gained');
    assert.ok(s2.events.some((e) => e.type === 'dr_outcome' && e.detail.fate === 'sealed'));
    assert.equal(Ember.maxHpBonus(s2.profile), 0);
    assert.equal(Ember.maxHpBonus(sim.profile), 1);
});

// ---- 7. spareLethal ----
check('spareLethal: once per stage, hurtPlayer, interior hurtHero, not a pit fall', () => {
    const sim = hubSim();
    assert.equal(Ember.spareLethal(sim), false, 'no blessing without the seal');
    Npcs.setFlag(sim, 'dr.fate', 3);
    sim.player.hp = 1; sim.player.invulnTimer = 0;
    Combat.hurtPlayer(sim, 2, sim.player.x);
    assert.equal(sim.player.hp, 1, 'the blessing spares the first lethal hit');
    assert.equal(sim.blessUsed, true);
    assert.equal(sim.notice, 'runenpfad.notice.blessing');
    const bless = sim.events.filter((e) => e.type === 'dr_blessing');
    assert.equal(bless.length, 1);
    assert.ok(typeof bless[0].detail.x === 'number', 'world blessing carries a position');
    sim.player.invulnTimer = 0;
    sim.checkpointUsed = true;
    Combat.hurtPlayer(sim, 2, sim.player.x);
    assert.equal(sim.player.hp, 0, 'second lethal hit lands');

    const pit = hubSim();
    Npcs.setFlag(pit, 'dr.fate', 3);
    pit.player.hp = 3; pit.player.invulnTimer = 0; pit.checkpointUsed = true;
    Combat.hurtPlayer(pit, pit.player.maxHp, pit.player.x);
    assert.equal(pit.player.hp, 0, 'a pit fall is not spared');
    assert.equal(!!pit.blessUsed, false);

    const next = hubSim();
    next.profile = sim.profile;
    assert.equal(!!next.blessUsed, false, 'a fresh sim has a fresh blessing');

    if (Interior && /spareLethal/.test(fs.readFileSync(path.join(dir, 'interior.js'), 'utf8'))) {
        const s3 = hubSim();
        Npcs.setFlag(s3, 'dr.fate', 3);
        const door = s3.stage.doors.filter((d) => d.placeId === 'tavern')[0];
        s3.player.x = door.x; s3.player.y = door.y; s3.player.onGround = true;
        tick(s3, {}, 2);
        tick(s3, { talkPressed: 'e' });
        assert.ok(s3.interior, 'inside the tavern');
        s3.player.hp = 1;
        s3.interior.hero.hurtT = 0;
        s3.interior.hero.invuln = 0;
        Interior.hurtHero(s3, s3.interior, 'rat', 0, 0, 3);
        assert.equal(s3.player.hp, 1, 'interior hurtHero spares once');
        const inside = s3.events.filter((e) => e.type === 'dr_blessing');
        assert.equal(inside.length, 1);
        assert.ok(!('x' in (inside[0].detail || {})), 'interior event carries no x');
    }
});

// ---- 8. meals ----
check('meals: hub keeps, level consumes, stew heart, broth dash', () => {
    const hub = hubSim({ npcFlags: { 'dr.meal': 1 } });
    const base = Profile.heroStats('knight', prof()).maxHp;
    assert.equal(Ember.mealOf(hub), 'stew');
    assert.equal(hub.player.maxHp, base + 1);
    assert.equal(hub.player.hp, base + 1);
    assert.equal(Npcs.getFlag(hub, 'dr.meal'), 1, 'the hub keeps the meal');
    assert.equal(Ember.setMeal(hub, 'tea'), false, 'only one meal at a time');

    const lvl = newSim({ mode: 'campaign', levelId: 1, profile: hub.profile });
    assert.equal(lvl.player.maxHp, base + 1);
    assert.equal(Ember.mealOf(lvl), 'stew');
    assert.equal(Npcs.getFlag(lvl, 'dr.meal'), 0, 'a level run consumes it');
    const after = newSim({ mode: 'campaign', levelId: 1, profile: lvl.profile });
    assert.equal(after.player.maxHp, base);
    assert.equal(Ember.mealOf(after), null);

    const cook = hubSim();
    assert.equal(Ember.setMeal(cook, 'nonsense'), false);
    assert.equal(Ember.setMeal(cook, 'stew'), true);
    assert.equal(cook.player.maxHp, cook.player.maxHp);
    assert.equal(cook.player.maxHp, base + 1);
    assert.equal(Npcs.getFlag(cook, 'dr.meal'), 1);
    assert.equal(Ember.setMeal(cook, 'broth'), false);

    // a workshop purchase rebuilds maxHp from heroStats (app.js); the stew heart stays
    cook.stats = Sim.heroStats(cook.heroId, cook.profile);
    cook.player.maxHp = Ember.maxHpFor(cook);
    cook.player.hp = Math.min(cook.player.maxHp, cook.player.hp + 1);
    assert.equal(cook.player.maxHp, base + 1, 'stew heart survives the rebuild');

    const tea = hubSim({ npcFlags: { 'dr.meal': 2 } });
    assert.equal(Ember.mealOf(tea), 'tea');
    assert.equal(tea.player.maxHp, base);

    const broth = newSim({ mode: 'campaign', levelId: 1, hero: 'ranger', profile: prof({ npcFlags: { 'dr.meal': 3 } }) });
    assert.equal(Ember.mealOf(broth), 'broth');
    tick(broth, { dashPressed: true });
    assert.ok(broth.player.dashCooldown > 0.4 && broth.player.dashCooldown <= 0.45, 'broth dash cooldown ' + broth.player.dashCooldown);
    const plain = newSim({ mode: 'campaign', levelId: 1, hero: 'ranger', profile: prof() });
    tick(plain, { dashPressed: true });
    assert.ok(plain.player.dashCooldown > 0.6, 'normal dash cooldown ' + plain.player.dashCooldown);
});

// ---- 9. dialogue conditions ----
check('the four COND keys evaluate and validate', () => {
    const sim = hubSim({ seen: ['dr_tremor'], npcFlags: { 'dr.fate': 1 } });
    const ev = (c) => Dialogue.evalCond(c, sim);
    assert.equal(ev({ step: 3 }), true);
    assert.equal(ev({ step: 4 }), false);
    assert.equal(ev({ fate: 'beaten' }), true);
    assert.equal(ev({ fate: 'slain' }), false);
    assert.equal(ev({ seen: 'dr_tremor' }), true);
    assert.equal(ev({ seen: 'dr_night' }), false);
    assert.equal(ev({ tablets: 1 }), false);
    Chron.giveTablet(sim, 2);
    assert.equal(ev({ tablets: 1 }), true);
    assert.equal(ev({ tablets: 2 }), false);
    assert.equal(ev({ not: { step: 5 }, step: 3 }), true);

    const GRAPHS = Story.GRAPHS;
    const probs = (cond, effects) => {
        GRAPHS.zz_test = { virtual: true, nodes: { a: { choices: [{ to: null, if: cond, effects: effects }, { to: null }] } } };
        const out = Dialogue.validateGraphs().filter((p) => p.indexOf('zz_test') === 0);
        delete GRAPHS.zz_test;
        return out;
    };
    deq(probs({ step: 5 }), []);
    deq(probs({ tablets: 6 }), []);
    deq(probs({ seen: 'dr_night' }), []);
    deq(probs({ fate: 'sealed' }), []);
    deq(probs(null, [{ ember: 'slay' }]), []);
    deq(probs(null, [{ ember: 'seal' }]), []);
    assert.equal(probs({ step: 6 }).length, 1);
    assert.equal(probs({ step: 1.5 }).length, 1);
    assert.equal(probs({ tablets: 0 }).length, 1);
    assert.equal(probs({ tablets: 7 }).length, 1);
    assert.equal(probs({ seen: 'Bad Id' }).length, 1);
    assert.equal(probs({ fate: 'dead' }).length, 1);
    assert.equal(probs(null, [{ ember: 'kill' }]).length, 1);
});

// ---- 10. scenes ----
check('21 scenes, chapter ids, variants and advance', () => {
    assert.equal(Cut.IDS.length, 21);
    const dr = ['dr_tremor', 'dr_night', 'dr_legend', 'dr_wake', 'dr_slain', 'dr_sealed'];
    dr.forEach((id) => { assert.ok(Cut.SCENES[id], id); assert.ok(Cut.CHAPTER_IDS.includes(id), id + ' is a chapter scene'); });
    deq(Cut.SCENES.dr_night.panels.map((p) => p.art), ['cut-dragon-3', 'cut-dragon-4']);
    assert.equal(Cut.SCENES.dr_legend.music, 'story_dawn');
    assert.equal(Cut.SCENES.dr_sealed.music, 'story_dawn');
    dr.forEach((id) => Cut.SCENES[id].panels.forEach((p) => {
        assert.equal(p.lines.length, 2); assert.equal(p.fg, null); assert.ok(p.sprites.length <= 3);
        p.lights.forEach((l) => assert.ok(l.r <= 0.5));
        p.fx.forEach((f) => assert.ok(Cut.FX_KINDS.includes(f)));
    }));
    Cut.IDS.forEach((id) => Cut.SCENES[id].panels.forEach((p) => assert.ok(p.lines.length <= 2, 'panel.lines never grows')));
    const withVariants = [];
    Cut.IDS.forEach((id) => Cut.SCENES[id].panels.forEach((p, i) => { if (p.variants) withVariants.push(id + ':' + (i + 1)); }));
    deq(withVariants.sort(), ['end_feared:1', 'end_friend:1', 'end_neutral:1', 'true_end:3']);
    deq(Cut.SCENES.end_friend.panels[0].variants.map((v) => v.fate), ['slain', 'sealed']);
    assert.equal(Cut.SCENES.true_end.panels[2].variants[1].key, 'runenpfad.cut.true_end.3sealed');

    const walk = (state) => {
        const keys = [];
        for (let guard = 0; guard < 60 && !state.done; guard++) {
            const cur = Cut.current(state);
            if (cur && keys[keys.length - 1] !== cur.lineKey) keys.push(cur.lineKey);
            Cut.setLineLength(state, 3);
            Cut.step(state, 1);
            Cut.advance(state);
        }
        return keys;
    };
    const plain = walk(Cut.create(['end_friend'], { npcFlags: {} }));
    deq(plain, ['runenpfad.cut.end_friend.1a', 'runenpfad.cut.end_friend.1b']);
    const slain = walk(Cut.create(['end_friend'], { npcFlags: { 'dr.fate': 2 } }));
    deq(slain, ['runenpfad.cut.end_friend.1a', 'runenpfad.cut.end_friend.1b', 'runenpfad.cut.end_friend.1slain']);
    const sealed = walk(Cut.create(['true_end'], { npcFlags: { 'dr.fate': 3 } }));
    assert.equal(sealed[sealed.length - 1], 'runenpfad.cut.true_end.3sealed');
    assert.equal(sealed.length, 7);
    const beaten = walk(Cut.create(['true_end'], { npcFlags: { 'dr.fate': 1 } }));
    assert.equal(beaten.length, 6, 'beaten shows no variant');
    deq(walk(Cut.create(['end_friend'])), plain, 'no profile, no variant');
    const cur = Cut.current(Cut.create(['dr_night']));
    assert.equal(cur.lines.length, 2);
    assert.ok(Cut.chronicle({ seen: ['dr_wake', 'dr_tremor'], unlockedLevel: 1 }).filter((e) => e.id.startsWith('dr_')).length === 2);
});

// ---- 11. quest-item notices ----
check('takeLoot sets a notice for quest items', () => {
    const sim = hubSim({ coins: 0 });
    ['bell_bronze', 'dragon_scale', 'desk_key', 'room_key', 'clan_charm'].forEach((id, i) => {
        sim.notice = '';
        Places.takeLoot(sim, { lootId: 'zz_' + i, loot: { item: { [id]: 1 } } });
        assert.equal(sim.notice, 'runenpfad.notice.got_' + id);
        assert.equal(Npcs.itemCount(sim, id), 1);
    });
    sim.notice = '';
    Places.takeLoot(sim, { lootId: 'zz_9', loot: { item: { turnip: 2 } } });
    assert.equal(sim.notice, '', 'food items give no quest notice');
    assert.equal(Npcs.itemCount(sim, 'turnip'), 2);
    const ids = Object.keys(Profile.STORY_ITEMS);
    ['bell_bronze', 'dragon_scale', 'desk_key', 'room_key', 'clan_charm', 'turnip', 'ham', 'frostberry', 'honey'].forEach((id) => assert.ok(ids.includes(id), id));
    assert.equal(Profile.STORY_ITEMS.turnip, 3);
    assert.equal(Profile.STORY_ITEMS.bell_bronze, 1);
});

// ---- 12. maximal profile ----
check('a maximal profile stays under the save limit', () => {
    const flags = {};
    for (let i = 0; i < 96; i++) flags['abcdefghijklmno' + (i % 10) + '.' + 'x'.repeat(22) + String(i).padStart(2, '0').slice(-1) + String.fromCharCode(97 + (i % 26))] = 99;
    const lootIds = Array.from({ length: 48 }, (_, i) => 'loot_' + 'x'.repeat(14) + String(i).padStart(4, '0'));
    const seen = Array.from({ length: 24 }, (_, i) => 'seen_' + 'x'.repeat(9) + String(i).padStart(2, '0'));
    const p = prof({ npcFlags: flags, loot: lootIds, seen, inventory: Profile.STORY_ITEMS, quests: { active: Profile.QUEST_IDS, done: Profile.QUEST_IDS }, rep: { coven: 5, circle: -5, kobolds: 5 } });
    assert.ok(Object.keys(p.npcFlags).length <= 96);
    assert.ok(JSON.stringify(p).length < 16384, 'size ' + JSON.stringify(p).length);
    deq(Profile.importProfile(JSON.parse(JSON.stringify(p))), p, 'round trip');
});

// ---- 13. flag budget ----
check('flag budget: distinct static keys plus 12 dynamic stay at 90 or below', () => {
    const files = fs.readdirSync(dir).filter((f) => /\.js$/.test(f) && !/(-view|-ui|^art|^audio|^music|^sfx|^app|^view|^hud|^decor|^backdrops|^gamepad|^postfx|^camera|^feel|^atmosphere|^parallax|^props|^set-pieces|^actors|^stage-art|^interior-bake|^cutscene|^games)/.test(f));
    const keys = new Set();
    const RE = /['"]([a-z][a-z0-9_]{0,15}\.[a-z0-9_]{1,23})['"]/g;
    files.forEach((f) => {
        const src = fs.readFileSync(path.join(dir, f), 'utf8');
        for (const m of src.matchAll(RE)) {
            const k = m[1];
            if (/\.(beaten|duel_lost)$/.test(k)) continue;
            if (/^(sim|root|window|math|json|object|array|number|ctx|profile|stage|state|it|e|p|m|c|d|o|s|t)\./.test(k)) continue;
            if (/\.(js|css|json|webp|png|mjs)$/.test(k)) continue;
            keys.add(k);
        }
    });
    // Only keys that really are flags: known prefixes of the story modules.
    const flagKeys = [...keys].filter((k) => Profile.FLAG_RE.test(k) && /^(dr|obs|mira|pip|nokk|bram|recruit_[fm]|epilogue|elder|witch|seer|alchemist|automaton|hermit|kobold|gate|ruins|lore|tavern|shop|smith|cog|tilda|berta|hall|desk|house)\./.test(k));
    const total = flagKeys.length + 12;
    assert.ok(total <= 90, 'flag keys ' + total + ': ' + flagKeys.join(', '));
    ['dr.fate', 'dr.tabs', 'dr.bell', 'dr.shield', 'dr.meal', 'dr.map', 'dr.visit_a', 'dr.visit_b'].forEach((k) => assert.ok(Profile.FLAG_RE.test(k), k));
    deq(Object.values(Ember.FLAGS).sort(), ['dr.bell', 'dr.fate', 'dr.map', 'dr.meal', 'dr.shield', 'dr.tabs', 'dr.visit_a', 'dr.visit_b']);
});

if (stubbed.length) console.log('note: stand-in places used for ' + stubbed.join(', '));
console.log('test-runenpfad-ember: ' + passed + ' checks passed');
