import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

// WP-G of Runenpfad round 11: the story graphs and narrative text of the ember dragon chapter
// (ember-story.js, ember-story-folk.js, the recruit and Cog nodes of journal-story.js, the new folk of folk.js).

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const R = 'ui/js/desktop/apps/runenpfad/';

// Sim core in the canonical order of the round-11 contract (section 2.2); files that do not exist yet are skipped.
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior-bosses', 'interior-things',
    'deep-mobs', 'brandur', 'interior', 'places', 'folk', 'folk-story', 'journal-story', 'ember-story', 'ember-story-folk', 'village-rooms', 'place-rooms-deep',
    'deeps-rooms', 'village', 'cutscenes', 'chronicle', 'ember', 'crafts', 'dice', 'bounties', 'levels', 'combat', 'sim'];

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
const missing = [];
for (const m of MODULES) {
    const abs = path.join(rootDir, R + m + '.js');
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
    else missing.push(m);
}
const Sim = sandbox.RunenpfadSim;
const Profile = sandbox.RunenpfadProfile;
const Npcs = sandbox.RunenpfadNpcs;
const Story = sandbox.RunenpfadNpcStory;
const Dialogue = sandbox.RunenpfadDialogue;
const Ember = sandbox.RunenpfadEmber;
const Cut = sandbox.RunenpfadCutscenes;
const Folk = sandbox.RunenpfadFolk;
assert.ok(Sim && Profile && Npcs && Story && Dialogue && Folk, 'sim core loaded');
assert.ok(Ember, 'ember.js (step, fate) is needed for the scenario checks');
assert.ok(Story.GRAPHS.brandur && Story.GRAPHS.mira && Story.GRAPHS.lore.nodes.tab_1, 'ember-story.js loaded');
if (missing.length) console.log('note: modules not present yet: ' + missing.join(', '));

// Values made inside the vm context have another Array/Object prototype: compare them as JSON.
const same = (a, b, msg) => assert.equal(JSON.stringify(a), JSON.stringify(b), msg);

const STEP = Sim.STEP;
let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed += 1;
        console.log('ok  ' + name);
    } catch (err) {
        console.error('FAILED: ' + name);
        throw err;
    }
}

function tick(sim, n) {
    for (let i = 0; i < (n || 1); i++) Sim.stepSimulation(sim, STEP * 1.000001, {});
}
function newSim(profile) {
    const sim = Sim.createSimulation({ mode: 'campaign', levelId: 0, hero: 'knight', profile: Profile.importProfile(Object.assign({ v: 1 }, profile || {})) });
    tick(sim, 30);
    return sim;
}
// The chapter scenarios: what the save looks like at each step (flags as in contract section 1.3).
const SCEN = {
    step0: {},
    step1: { seen: ['dr_tremor'] },
    step2: { seen: ['dr_tremor', 'dr_night'], npcFlags: {} },
    beaten: { seen: ['dr_tremor', 'dr_night'], npcFlags: { 'dr.fate': 1 } },
    slain: { seen: ['dr_tremor', 'dr_night'], npcFlags: { 'dr.fate': 2 } },
    sealed: { seen: ['dr_tremor', 'dr_night'], npcFlags: { 'dr.fate': 3, 'dr.bell': 1 } }
};
const sim = (name, extra) => {
    const sc = SCEN[name];
    const flags = Object.assign({}, sc.npcFlags || {}, (extra && extra.npcFlags) || {});
    return newSim(Object.assign({ unlockedLevel: 12 }, sc, extra || {}, { npcFlags: flags }));
};
const startOf = (s, npcId, meetingId) => {
    const st = Dialogue.startNode(s, { npcId: npcId, meetingId: meetingId });
    return st ? st.local : null;
};
// A resident in the stage list (the dialogue finds it there), opened through its meeting start.
function talk(s, npcId, meetingId) {
    const npc = { ref: 'test_' + npcId, npcId: npcId, x: s.player.x, y: s.player.y, meetingId: meetingId, facing: 1, state: 'idle', mood: 'neutral', near: false, marker: null, provoked: false, sold: {}, hurtT: 0 };
    s.stage.npcs = (s.stage.npcs || []).filter((n) => n.ref !== npc.ref).concat(npc);
    assert.ok(Dialogue.open(s, npc), npcId + ' opens');
    return npc;
}
const keysOf = (s) => Array.from(Dialogue.view(s).choices, (c) => c.textKey);
const nodeOfView = (s) => Dialogue.view(s).textKey.replace('runenpfad.npc.', '');
function pick(s, textKey) {
    const i = keysOf(s).indexOf(textKey);
    assert.ok(i >= 0, textKey + ' is offered: ' + keysOf(s).join(', '));
    return Dialogue.choose(s, i);
}

const MEETING_NPC = {
    elder_home: 'elder', innkeeper_home: 'innkeeper', shopkeeper_home: 'shopkeeper', smith_home: 'smith', bard_home: 'bard', gambler_home: 'gambler',
    edda_home: 'edda', automaton_hall: 'automaton', recruit_f_home: 'recruit_f', recruit_m_home: 'recruit_m',
    mira_deeps: 'mira', mira_home: 'mira', nokk_home: 'nokk', pip_home: 'pip', bram_home: 'bram'
};

check('every graph validates (validateGraphs is clean)', () => {
    same(Dialogue.validateGraphs(), []);
});

check('the new speakers, sprites and folk definitions exist', () => {
    ['mira', 'nokk', 'pip', 'bram', 'brandur', 'recruit_f', 'recruit_m'].forEach((id) => {
        const d = Npcs.NPCS[id];
        assert.ok(d, id + ' is an NPC');
        assert.equal(d.nameKey, 'runenpfad.npc.' + id + '.name');
        assert.equal(d.titleKey, 'runenpfad.npc.' + id + '.title');
        same(d.offers, []);
        assert.equal(d.duelCoins, 0);
    });
    assert.equal(Npcs.NPCS.brandur.portrait, 'brandur');
    same([Folk.SPRITES.mira, Folk.SPRITES.nokk, Folk.SPRITES.pip, Folk.SPRITES.bram], [['td-folk-deep', 0], ['td-folk-deep', 1], ['td-folk-deep', 2], ['td-folk-deep', 3]]);
    assert.equal(Npcs.NPCS.nokk.faction, 'kobolds');
    assert.equal(Npcs.NPCS.pip.faction, 'kobolds');
    assert.equal(Npcs.NPCS.mira.faction, null);
    same(Npcs.NPCS.shopkeeper.offers, ['potion', 'map_charm', 'lucky_amulet', 'rune_attune', 'turnip', 'ham', 'frostberry', 'honey']);
    assert.equal(Folk.RECRUITS.length, 2);
});

check('every meeting start resolves in every scenario, and the last start has no condition', () => {
    Object.keys(MEETING_NPC).forEach((mid) => {
        const m = Story.MEETINGS.filter((x) => x.id === mid)[0];
        assert.ok(m, 'meeting ' + mid);
        assert.equal(m.npcId, MEETING_NPC[mid]);
        assert.ok(!m.starts[m.starts.length - 1].if, mid + ' last start is plain');
        Object.keys(SCEN).forEach((name) => {
            const s = sim(name, mid === 'automaton_hall' ? { npcFlags: { 'obs.done': 1 } } : null);
            const local = startOf(s, m.npcId, mid);
            assert.ok(local, mid + ' resolves at ' + name);
            assert.ok(Story.GRAPHS[m.npcId].nodes[local], mid + ' ' + name + ' -> ' + local + ' exists');
        });
    });
});

check('villagers react at step 1, step 2 and step 5, and step 5 differs by fate', () => {
    ['elder_home', 'innkeeper_home', 'shopkeeper_home', 'smith_home', 'bard_home', 'gambler_home'].forEach((mid) => {
        const npcId = MEETING_NPC[mid];
        const at = (name) => startOf(sim(name), npcId, mid);
        assert.equal(at('step1'), 'dr_s1', mid + ' step 1');
        assert.equal(at('step2'), 'dr_s2', mid + ' step 2');
        assert.equal(at('beaten'), 'dr_s2', mid + ' beaten is still the night');
        assert.equal(at('slain'), 'dr_slain', mid + ' slain');
        assert.equal(at('sealed'), 'dr_sealed', mid + ' sealed');
        assert.notEqual(at('step0'), 'dr_s1', mid + ' step 0 stays the old talk');
    });
    // Edda stands in the hall after the observatory; Cog asks his question on his own nodes
    const eddaAt = (name) => startOf(sim(name, { npcFlags: { 'edda.met': 1 } }), 'edda', 'edda_home');
    same(['step1', 'step2', 'slain', 'sealed'].map(eddaAt), ['dr_s1', 'dr_s2', 'dr_slain', 'dr_sealed']);
    assert.equal(eddaAt('step0'), 'again');
    // the one-time starts stay in front: Runa's homecoming line and Edda's first hello
    const home = sim('sealed', { npcFlags: { 'obs.done': 1 } });
    assert.equal(startOf(home, 'elder', 'elder_home'), 'home');
    assert.equal(startOf(sim('slain'), 'edda', 'edda_home'), 'hello');
});

check('Runa: the legend scene is offered after three tablets and once', () => {
    const s = sim('step2', { npcFlags: { 'dr.tabs': 7 } });
    assert.equal(startOf(s, 'elder', 'elder_home'), 'legend');
    assert.equal(startOf(sim('step2', { npcFlags: { 'dr.tabs': 3 } }), 'elder', 'elder_home'), 'dr_s2', 'two tablets are not enough');
    talk(s, 'elder', 'elder_home');
    assert.equal(nodeOfView(s), 'elder.legend');
    Dialogue.choose(s, 0);
    assert.equal(s.uiRequest, 'scene:dr_legend');
    assert.equal(s.encounter, null, 'the dialogue closes for the scene');
    // the overlay marks the scene seen; the next visit is the plain night talk
    s.profile.seen = s.profile.seen.concat('dr_legend');
    assert.equal(startOf(s, 'elder', 'elder_home'), 'dr_s2');
    const slain = sim('slain', { npcFlags: { 'dr.tabs': 7 } });
    assert.equal(startOf(slain, 'elder', 'elder_home'), 'legend', 'the legend is still told later when it was never heard');
});

check('Hakon works the bell bronze and the dragon scale, Tilda knows when Mira is found', () => {
    const smith = (extra) => startOf(sim('step2', extra), 'smith', 'smith_home');
    assert.equal(smith({}), 'dr_s2');
    const withBronze = sim('step2');
    Npcs.addItem(withBronze, 'bell_bronze', 1);
    assert.equal(startOf(withBronze, 'smith', 'smith_home'), 'dr_bronze');
    Npcs.setFlag(withBronze, 'dr.bell', 1);
    assert.equal(startOf(withBronze, 'smith', 'smith_home'), 'dr_s2', 'a mended bell needs no more bronze');
    const withScale = sim('step2');
    Npcs.addItem(withScale, 'dragon_scale', 1);
    assert.equal(startOf(withScale, 'smith', 'smith_home'), 'dr_scale');
    assert.equal(startOf(sim('step2', { npcFlags: { 'mira.found': 1 } }), 'shopkeeper', 'shopkeeper_home'), 'dr_found');
    assert.equal(startOf(sim('sealed', { npcFlags: { 'mira.found': 1 } }), 'shopkeeper', 'shopkeeper_home'), 'dr_sealed');
    assert.equal(startOf(sim('step2'), 'shopkeeper', 'shopkeeper_home'), 'dr_s2');
});

check('Brandur: slay ends the fire, the cracked bell blocks the seal, the mended bell allows it', () => {
    // slay
    let s = sim('beaten');
    assert.ok(Dialogue.openVirtual(s, 'brandur', 'beaten'));
    same(keysOf(s), ['runenpfad.npc.brandur.beaten.c1', 'runenpfad.npc.brandur.beaten.c2', 'runenpfad.npc.brandur.beaten.c4']);
    pick(s, 'runenpfad.npc.brandur.beaten.c1');
    assert.equal(nodeOfView(s), 'brandur.slay');
    pick(s, 'runenpfad.npc.brandur.slay.c2');
    assert.equal(nodeOfView(s), 'brandur.again', 'waiting leads back to the choice');
    pick(s, 'runenpfad.npc.brandur.beaten.c1');
    const coins = s.profile.coins;
    pick(s, 'runenpfad.npc.brandur.slay.c1');
    assert.equal(Ember.fate(s.profile), 'slain');
    assert.equal(s.profile.npcFlags['dr.fate'], 2);
    assert.equal(s.uiRequest, 'scene:dr_slain');
    assert.equal(s.profile.coins, coins + 300, 'the hoard pays 300');
    // seal without the bell
    s = sim('beaten');
    Dialogue.openVirtual(s, 'brandur', 'beaten');
    pick(s, 'runenpfad.npc.brandur.beaten.c2');
    assert.equal(nodeOfView(s), 'brandur.seal_no');
    assert.equal(s.profile.npcFlags['dr.fate'], 1, 'the fate stays beaten');
    assert.equal(s.uiRequest, null);
    pick(s, 'runenpfad.npc.brandur.seal_no.c1');
    assert.equal(s.encounter, null);
    // seal with the bell
    s = sim('beaten', { npcFlags: { 'dr.bell': 1 } });
    Dialogue.openVirtual(s, 'brandur', 'beaten');
    pick(s, 'runenpfad.npc.brandur.beaten.c2');
    assert.equal(nodeOfView(s), 'brandur.seal');
    pick(s, 'runenpfad.npc.brandur.seal.c1');
    assert.equal(Ember.fate(s.profile), 'sealed');
    assert.equal(s.uiRequest, 'scene:dr_sealed');
    // the later nodes open too
    ['again', 'sleeps', 'bones'].forEach((id) => {
        const t = sim('sealed');
        assert.ok(Dialogue.openVirtual(t, 'brandur', id), id);
    });
});

check('Mira, Pip, Nokk, Bram and the recruits talk', () => {
    // Mira in the cave: found, then she leaves
    const cave = sim('step2');
    const mira = talk(cave, 'mira', 'mira_deeps');
    assert.equal(nodeOfView(cave), 'mira.hello');
    pick(cave, 'runenpfad.npc.mira.hello.c2');
    assert.equal(nodeOfView(cave), 'mira.light');
    pick(cave, 'runenpfad.npc.mira.light.c1');
    assert.equal(nodeOfView(cave), 'mira.found');
    assert.equal(Npcs.getFlag(cave, 'mira.found'), 1);
    pick(cave, 'runenpfad.npc.mira.found.c1');
    assert.equal(mira.state, 'gone', 'Mira leaves the cave');
    assert.equal(cave.encounter, null);
    // Pip: the charm goes back, once
    const p = sim('step2');
    assert.equal(startOf(p, 'pip', 'pip_home'), 'hello');
    Npcs.addItem(p, 'clan_charm', 1);
    assert.equal(startOf(p, 'pip', 'pip_home'), 'charm');
    const rep0 = Npcs.getRep(p, 'kobolds');
    talk(p, 'pip', 'pip_home');
    pick(p, 'runenpfad.npc.pip.charm.c1');
    assert.equal(nodeOfView(p), 'pip.charm_ok');
    assert.equal(Npcs.itemCount(p, 'clan_charm'), 0);
    assert.equal(Npcs.getRep(p, 'kobolds'), rep0 + 1);
    assert.equal(Npcs.getFlag(p, 'pip.charm'), 1);
    Dialogue.close(p);
    assert.equal(startOf(p, 'pip', 'pip_home'), 'happy');
    // Nokk, Bram and the fates
    ['nokk', 'pip', 'bram', 'mira'].forEach((id) => {
        const mid = id === 'mira' ? 'mira_home' : id + '_home';
        assert.equal(startOf(sim('slain'), id, mid), 'slain', id);
        assert.equal(startOf(sim('sealed'), id, mid), 'sealed', id);
    });
    const b = sim('step2');
    talk(b, 'bram', 'bram_home');
    pick(b, 'runenpfad.npc.bram.hello.c1');
    assert.equal(nodeOfView(b), 'bram.rise');
    pick(b, 'runenpfad.npc.bram.hello.c2');
    assert.equal(nodeOfView(b), 'bram.broth');
    const n = sim('step2');
    talk(n, 'nokk', 'nokk_home');
    pick(n, 'runenpfad.npc.nokk.hello.c1');
    pick(n, 'runenpfad.npc.nokk.hello.c2');
    assert.equal(nodeOfView(n), 'nokk.warm');
    // the recruits (after the observatory)
    ['recruit_f', 'recruit_m'].forEach((id) => {
        const mid = id + '_home';
        assert.equal(startOf(sim('step0'), id, mid), 'hello');
        assert.equal(startOf(sim('step2'), id, mid), 'hello');
        assert.equal(startOf(sim('slain'), id, mid), 'dr_slain');
        assert.equal(startOf(sim('sealed'), id, mid), 'dr_sealed');
        const s = sim('step0');
        talk(s, id, mid);
        assert.ok(Dialogue.view(s).choices.length >= 2, id + ' has a topic');
        Dialogue.choose(s, 0);
        assert.equal(Dialogue.view(s).choices.length, 1);
    });
});

check('Cog: the core\'s last line follows the answer, and he asks about the roots', () => {
    const at = (flags) => startOf(sim('step0', { npcFlags: flags }), 'automaton', 'automaton_hall');
    assert.equal(at({ 'obs.done': 1, 'obs.answer': 1 }), 'done_1');
    assert.equal(at({ 'obs.done': 1, 'obs.answer': 2 }), 'done_2');
    assert.equal(at({ 'obs.done': 1, 'obs.answer': 3 }), 'done_3');
    assert.equal(at({ 'obs.done': 1 }), 'done', 'saves without an answer keep the plain line');
    assert.equal(at({ 'obs.open': 1 }), 'wait');
    assert.equal(at({}), 'call');
    // the question appears only from step 1 on, and splits by step
    const asks = (name, node) => {
        const s = sim(name, { npcFlags: { 'obs.done': 1, 'obs.answer': 2 } });
        talk(s, 'automaton', 'automaton_hall');
        assert.equal(nodeOfView(s), 'automaton.' + node);
        return keysOf(s).includes('runenpfad.npc.automaton.dr_ask');
    };
    assert.equal(asks('step0', 'done_2'), false);
    assert.equal(asks('step2', 'done_2'), true);
    assert.equal(asks('slain', 'done_2'), true);
    const s = sim('step2', { npcFlags: { 'obs.done': 1, 'obs.answer': 1 } });
    talk(s, 'automaton', 'automaton_hall');
    pick(s, 'runenpfad.npc.automaton.dr_ask');
    assert.equal(nodeOfView(s), 'automaton.dr_s2');
    const t = sim('sealed', { npcFlags: { 'obs.done': 1, 'obs.answer': 3 } });
    talk(t, 'automaton', 'automaton_hall');
    pick(t, 'runenpfad.npc.automaton.dr_ask');
    assert.equal(nodeOfView(t), 'automaton.dr_s5');
    pick(t, 'runenpfad.npc.automaton.dr_s5.c1');
    assert.equal(nodeOfView(t), 'automaton.dr_sealed');
    const u = sim('slain', { npcFlags: { 'obs.done': 1, 'obs.answer': 3 } });
    talk(u, 'automaton', 'automaton_hall');
    pick(u, 'runenpfad.npc.automaton.dr_ask');
    pick(u, 'runenpfad.npc.automaton.dr_s5.c1');
    assert.equal(nodeOfView(u), 'automaton.dr_slain');
    // the quest nodes still lead where they did
    const q = sim('step2', { npcFlags: { 'epilogue.paid': 1 } });
    talk(q, 'automaton', 'automaton_hall');
    assert.equal(nodeOfView(q), 'automaton.call');
    pick(q, 'runenpfad.npc.automaton.call.c1');
    assert.equal(nodeOfView(q), 'automaton.where');
});

check('lore: 17 new inscriptions, six tablets, every one a single way back', () => {
    const ids = ['tab_1', 'tab_2', 'tab_3', 'tab_4', 'tab_5', 'tab_6', 'bram_chalk', 'kobold_scratch', 'vein_hint', 'finn_song', 'gundar_note', 'archive_hint',
        'tilda_gear', 'edda_letter_1', 'edda_letter_2', 'edda_notes', 'edda_map'];
    ids.forEach((id) => {
        const node = Story.GRAPHS.lore.nodes[id];
        assert.ok(node, id);
        assert.equal(node.choices.length, 1);
        assert.equal(node.choices[0].text, 'runenpfad.npc.c.step_back');
        assert.equal(node.choices[0].to, null);
        const s = sim('step0');
        assert.ok(Dialogue.openVirtual(s, 'lore', id), id + ' opens');
    });
    // the old inscriptions are still there
    ['tomb_mural', 'scroll_3', 'obs_door'].forEach((id) => assert.ok(Story.GRAPHS.lore.nodes[id], id));
});

check('at most six new NPC flag keys', () => {
    const flagRe = /'([a-z][a-z0-9_]{0,15}\.[a-z0-9_]{1,23})'/g;
    const mine = new Set();
    ['ember-story.js', 'ember-story-folk.js'].forEach((f) => {
        const src = fs.readFileSync(path.join(rootDir, R, f), 'utf8').split('\n').filter((l) => !/^\s*\/\//.test(l)).join('\n');
        let m;
        while ((m = flagRe.exec(src))) mine.add(m[1]);
    });
    // npc.* ids are not flags; dr.* belong to the chapter core (ember.js)
    const fresh = Array.from(mine).filter((k) => !/^dr\./.test(k) && !/^runenpfad\./.test(k) && !/^obs\./.test(k) && !/^(?:bard|elder|epilogue)\./.test(k));
    same(fresh.sort(), ['mira.found', 'pip.charm'], 'flags: ' + fresh.join(', '));
    assert.ok(fresh.length <= 6);
});

// ---- texts (the locale files hold them only after the coordinator's merge) ----
const langDir = process.env.RUNENPFAD_LANG_DIR || path.join(rootDir, 'ui/lang/runenpfad');
const lang = {};
['en', 'de'].forEach((l) => {
    const f = path.join(langDir, l + '.json');
    lang[l] = fs.existsSync(f) ? JSON.parse(fs.readFileSync(f, 'utf8')) : {};
});

function storyKeys() {
    const keys = [].concat(Dialogue.textKeys(), Npcs.textKeys());
    for (let n = 1; n <= 6; n++) keys.push('runenpfad.tablet.' + n + '.title');
    for (let n = 7; n <= 18; n++) keys.push('runenpfad.village.bark.' + n);
    if (Cut) {
        ['dr_tremor', 'dr_night', 'dr_legend', 'dr_wake', 'dr_slain', 'dr_sealed'].forEach((id) => {
            const sc = Cut.SCENES[id];
            if (!sc) return;
            keys.push(sc.titleKey);
            sc.panels.forEach((p) => {
                p.lines.forEach((k) => keys.push(k));
                (p.variants || []).forEach((v) => keys.push(v.key));
            });
        });
        Object.keys(Cut.SCENES).forEach((id) => Cut.SCENES[id].panels.forEach((p) => (p.variants || []).forEach((v) => keys.push(v.key))));
    }
    return keys.filter((k, i) => keys.indexOf(k) === i);
}

if (!lang.en['runenpfad.npc.brandur.beaten'] || !lang.de['runenpfad.npc.brandur.beaten']) {
    console.log('skip  text coverage: the round-11 keys are not in ui/lang yet (they arrive with the coordinator merge)');
} else {
    check('every story key exists in en and de, and cut lines and titles keep their length', () => {
        const keys = storyKeys();
        for (const l of ['en', 'de']) {
            const gone = keys.filter((k) => typeof lang[l][k] !== 'string' || !lang[l][k].trim());
            same(gone, [], l + ' misses keys');
            const max = l === 'en' ? 120 : 140;
            keys.filter((k) => /^runenpfad\.cut\.(dr_|end_|true_end)/.test(k) && !/\.title\./.test(k)).forEach((k) => assert.ok(lang[l][k].length <= max, l + ' ' + k + ' is ' + lang[l][k].length));
            keys.filter((k) => /^runenpfad\.(cut\.title\.dr_|tablet\.\d\.title)/.test(k)).forEach((k) => assert.ok(lang[l][k].length <= 40, l + ' ' + k));
        }
        assert.ok(keys.length > 150, 'many keys checked: ' + keys.length);
    });
    check('the changed keys carry the thin-spot fixes', () => {
        assert.match(lang.en['runenpfad.npc.elder.hello'], /Edda's apprentice/);
        assert.match(lang.en['runenpfad.npc.elder.keepers'], /sky-machine/);
        assert.match(lang.en['runenpfad.npc.automaton.call'], /counting in secret/);
        assert.match(lang.en['runenpfad.npc.alchemist.reveal'], /Edda asked/);
        assert.match(lang.en['runenpfad.cut.prologue.4a'], /apprentice/);
        assert.match(lang.de['runenpfad.npc.elder.hello'], /Lehrling/);
        assert.match(lang.de['runenpfad.npc.elder.keepers'], /Flugmaschine/);
    });
    check('the clues: veins low to top, books east to north, tablets in order', () => {
        const en = lang.en;
        const order = (text, words) => words.map((w) => text.toLowerCase().indexOf(w));
        const rising = (a) => a.every((x, i) => x >= 0 && (i === 0 || x > a[i - 1]));
        assert.ok(rising(order(en['runenpfad.npc.lore.bram_chalk'], ['low', 'middle', 'top'])), 'bram_chalk');
        assert.match(en['runenpfad.npc.lore.vein_hint'], /low.*middle.*high.*bottom up/i);
        assert.ok(rising(order(en['runenpfad.npc.lore.archive_hint'], ['east', 'south', 'west', 'north'])), 'archive_hint');
        const topics = [/first stone/i, /Skadi.*Brandur/i, /greed|wanted more/i, /sleep|sang/i, /bell/i, /together, or not at all/i];
        topics.forEach((re, i) => assert.match(en['runenpfad.npc.lore.tab_' + (i + 1)], re, 'tablet ' + (i + 1)));
    });
}

console.log('Runenpfad ember story checks passed (' + passed + ' checks): graphs, meetings by step and fate, Brandur choice, new folk, Cog, lore, flag budget.');
