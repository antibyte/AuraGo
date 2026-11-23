import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const R = 'ui/js/desktop/apps/runenpfad/';

// Same order as module-loader.js (journal-story after folk-story, chronicle after cutscenes).
const BASE = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior', 'places', 'folk', 'folk-story'];
const REST = ['village-rooms', 'village', 'cutscenes', 'chronicle', 'levels', 'combat', 'sim'];

function makeSandbox(withJournal) {
    const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    const list = BASE.concat(withJournal ? ['journal-story'] : [], REST);
    for (const m of list) {
        const abs = path.join(rootDir, R + m + '.js');
        if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
    }
    return sandbox;
}

const plain = makeSandbox(false);
const sandbox = makeSandbox(true);
const Sim = sandbox.RunenpfadSim;
const Profile = sandbox.RunenpfadProfile;
const Npcs = sandbox.RunenpfadNpcs;
const Story = sandbox.RunenpfadNpcStory;
const Dialogue = sandbox.RunenpfadDialogue;
const Chronicle = sandbox.RunenpfadChronicle;
const Cut = sandbox.RunenpfadCutscenes;
assert.ok(Sim && Profile && Npcs && Dialogue && Chronicle && Cut, 'modules loaded');
assert.ok(Story.GRAPHS.journal && !plain.RunenpfadNpcStory.GRAPHS.journal, 'journal-story only in the full sandbox');

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

function tick(sim, input, n) {
    for (let i = 0; i < (n || 1); i++) Sim.stepSimulation(sim, STEP * 1.000001, input || {});
}
function newSim(profile, extra) {
    const sim = Sim.createSimulation(Object.assign({ mode: 'campaign', levelId: 0, hero: 'knight', profile: Profile.importProfile(Object.assign({ v: 1 }, profile || {})) }, extra || {}));
    tick(sim, {}, 30);
    return sim;
}
// A resident that stands in the stage list (the dialogue finds it there), opened through its meeting start.
function talk(sim, npcId, meetingId) {
    const npc = { ref: 'test_' + npcId, npcId: npcId, x: sim.player.x, y: sim.player.y, meetingId: meetingId, facing: 1, state: 'idle', mood: 'neutral', near: false, marker: null, provoked: false, sold: {}, hurtT: 0 };
    sim.stage.npcs = (sim.stage.npcs || []).filter((n) => n.ref !== npc.ref).concat(npc);
    assert.ok(Dialogue.open(sim, npc), npcId + ' opens');
    return npc;
}
const textOf = (sim) => Dialogue.view(sim).textKey.replace('runenpfad.npc.', '');
const choiceKeys = (sim) => Array.from(Dialogue.view(sim).choices, (c) => c.textKey.replace('runenpfad.npc.', ''));
function pick(sim, key) {
    const i = choiceKeys(sim).indexOf(key);
    assert.ok(i >= 0, key + ' is offered: ' + choiceKeys(sim).join(','));
    return Dialogue.choose(sim, i);
}
function startOf(sim, npcId, meetingId) {
    const st = Dialogue.startNode(sim, { npcId: npcId, meetingId: meetingId });
    return st ? st.local : null;
}

check('graphs validate with and without journal-story; ui values are checked', () => {
    assert.equal(JSON.stringify(plain.RunenpfadDialogue.validateGraphs()), '[]');
    assert.equal(JSON.stringify(Dialogue.validateGraphs()), '[]');
    const probe = (ui) => {
        Story.GRAPHS.__ui = { nodes: { a: { choices: [{ effects: [{ ui: ui }], to: null }] } } };
        Story.MEETINGS.push({ id: '__ui', npcId: '__ui', levels: [], starts: [{ to: 'a' }] });
        const bad = Dialogue.validateGraphs().some((p) => /bad ui/.test(p));
        delete Story.GRAPHS.__ui;
        Story.MEETINGS.pop();
        return bad;
    };
    assert.equal(probe('bogus'), true);
    assert.equal(probe('scene:Bad'), true);
    for (const ok of ['workshop', 'chronicle', 'board', 'dice:barkeep', 'song:in_cave', 'scene:true_end']) assert.equal(probe(ok), false, ok);
    assert.equal(JSON.stringify(Dialogue.validateGraphs()), '[]');
});

check('give adds a page once, sorted, with event, notice and counter; cap and endless are respected', () => {
    const sim = newSim({ loot: ['tavern_cellar'] });
    assert.equal(Chronicle.give(sim, 3), true);
    assert.equal(Chronicle.give(sim, 3), false, 'only once');
    assert.equal(Chronicle.give(sim, 1), true);
    assert.deepEqual(Array.from(sim.profile.loot), ['page_01', 'page_03', 'tavern_cellar']);
    assert.equal(Chronicle.count(sim.profile), 2);
    assert.equal(Chronicle.has(sim.profile, 1), true);
    assert.equal(Chronicle.has(sim.profile, 2), false);
    assert.equal(sim.storyDirty, true);
    assert.equal(sim.notice, 'runenpfad.notice.page_found');
    const f = sim.floatingTexts[sim.floatingTexts.length - 1];
    assert.equal(f.key, 'runenpfad.notice.page_found');
    assert.equal(f.suffix, ' 2/10');
    assert.ok(sim.events.some((e) => e.type === 'in_page' && e.detail.page === 1 && e.detail.first === true && !('x' in e.detail)), 'in_page event');
    assert.equal(Chronicle.give(sim, 0), false);
    assert.equal(Chronicle.give(sim, 11), false);
    const full = newSim({});
    full.profile.loot = [];
    for (let i = 0; i < 48; i++) full.profile.loot.push('zz_' + String(i).padStart(2, '0'));
    assert.equal(Chronicle.give(full, 2), false, 'the loot list stays at 48');
    assert.equal(full.profile.loot.length, 48);
    const endless = newSim({}, { mode: 'endless', levelId: 1 });
    assert.equal(Chronicle.give(endless, 4), false, 'no pages in endless mode');
    assert.deepEqual(Array.from(endless.profile.loot), []);
});

check('PAGES, list and chapter have the contract shape', () => {
    assert.equal(Chronicle.PAGES.length, 10);
    Chronicle.PAGES.forEach((p, i) => {
        assert.deepEqual(Object.keys(p), ['n', 'loot', 'placeId', 'level', 'titleKey']);
        assert.equal(p.n, i + 1);
        assert.equal(p.loot, 'page_' + String(i + 1).padStart(2, '0'));
        assert.match(p.loot, Profile.LOOT_RE);
        assert.equal(p.titleKey, 'runenpfad.page.' + (i + 1) + '.title');
    });
    assert.deepEqual(Array.from(Chronicle.PAGES, (p) => p.placeId), ['hermit_hut', 'beast_den', 'kobold_den', 'crystal_grotto', 'nomad_tent', 'sun_tomb', 'summit_hut', 'ice_cave', 'brass_lab', 'boiler_room']);
    assert.deepEqual(Array.from(Chronicle.PAGES, (p) => p.level), [1, 3, 6, 9, 11, 14, 16, 19, 22, 24]);
    const rows = Chronicle.list({ loot: ['page_02', 'page_10'] }, (k) => '<' + k + '>');
    assert.equal(rows.length, 10);
    assert.deepEqual(Array.from(rows.filter((r) => r.have), (r) => r.n), [2, 10]);
    assert.equal(rows[1].title, '<runenpfad.page.2.title>');
    assert.equal(rows[1].place, '<runenpfad.place.beast_den>');
    assert.equal(Chronicle.list(null)[0].title, 'runenpfad.page.1.title');
    assert.equal(Chronicle.chapter({}), 'closed');
    assert.equal(Chronicle.chapter({ npcFlags: { 'obs.open': 1 } }), 'open');
    assert.equal(Chronicle.chapter({ npcFlags: { 'obs.open': 1, 'obs.done': 1 } }), 'done');
});

check('the pages condition and the page effect', () => {
    const sim = newSim({ loot: ['page_01', 'page_02'] });
    assert.equal(Dialogue.evalCond({ pages: 2 }, sim), true);
    assert.equal(Dialogue.evalCond({ pages: 3 }, sim), false);
    Dialogue.applyEffects(sim, null, [{ page: 3 }]);
    assert.equal(Dialogue.evalCond({ pages: 3 }, sim), true);
    assert.equal(Chronicle.count(sim.profile), 3);
});

check('openVirtual opens the journal, holds the world tick and closes again', () => {
    const sim = newSim({});
    assert.equal(Dialogue.openVirtual(sim, 'journal', 'p1a'), true);
    assert.ok(sim.encounter && sim.encounter.kind === 'dialogue' && sim.encounter.virtual);
    const v = Dialogue.view(sim);
    assert.equal(v.npcId, 'journal');
    assert.equal(v.textKey, 'runenpfad.npc.journal.p1a');
    assert.deepEqual(Array.from(v.choices, (c) => c.textKey), ['runenpfad.npc.c.read_on']);
    assert.equal(Dialogue.openVirtual(sim, 'journal', 'p2a'), false, 'no second window');
    const t0 = sim.elapsed;
    tick(sim, {}, 20);
    assert.equal(sim.elapsed, t0, 'the world holds while the journal is open');
    Dialogue.choose(sim, 0);
    assert.equal(Dialogue.view(sim).textKey, 'runenpfad.npc.journal.p1b');
    const res = Dialogue.choose(sim, 0);
    assert.equal(res.closed, true);
    assert.equal(sim.encounter, null);
    tick(sim, {}, 3);
    assert.ok(sim.elapsed > t0, 'the world runs again');
    assert.equal(Dialogue.openVirtual(sim, 'journal', 'nope'), false);
    assert.equal(Dialogue.openVirtual(sim, 'nobody', 'p1a'), false);
    for (const id of Object.keys(Story.GRAPHS.lore.nodes)) {
        assert.equal(Dialogue.openVirtual(sim, 'lore', id), true, id);
        assert.equal(Dialogue.choose(sim, 0).closed, true);
    }
});

check('the Cog chapter opens the observatory once', () => {
    const sim = newSim({});
    assert.equal(startOf(sim, 'automaton', 'automaton_hall'), 'call');
    talk(sim, 'automaton', 'automaton_hall');
    assert.deepEqual(choiceKeys(sim), ['automaton.call.c1', 'c.farewell']);
    Dialogue.choose(sim, 0);
    assert.equal(textOf(sim), 'automaton.where');
    assert.equal(Npcs.hasFlag(sim, 'obs.open'), false);
    const res = Dialogue.choose(sim, 0);
    assert.equal(res.closed, true);
    assert.equal(Npcs.hasFlag(sim, 'obs.open'), true);
    assert.equal(sim.notice, 'runenpfad.notice.chapter_open');
    assert.equal(sim.uiRequest, 'scene:obs_intro');
    assert.equal(startOf(sim, 'automaton', 'automaton_hall'), 'wait');
    Npcs.setFlag(sim, 'obs.done', 1);
    assert.equal(startOf(sim, 'automaton', 'automaton_hall'), 'done');
});

check('the core dialogue sets obs.done, pays once and requests the true ending', () => {
    const sim = newSim({ coins: 10 });
    assert.equal(Dialogue.openVirtual(sim, 'core', 'query'), true);
    assert.equal(Dialogue.view(sim).choices.length, 3);
    Dialogue.choose(sim, 1);
    assert.equal(textOf(sim), 'core.a_walk');
    assert.equal(Npcs.getFlag(sim, 'obs.answer'), 2);
    assert.deepEqual(choiceKeys(sim), ['core.let_go', 'c.farewell']);
    Dialogue.choose(sim, 0);
    assert.equal(textOf(sim), 'core.release');
    assert.equal(Npcs.hasFlag(sim, 'obs.done'), false);
    const res = Dialogue.choose(sim, 0);
    assert.equal(res.closed, true);
    assert.equal(Npcs.hasFlag(sim, 'obs.done'), true);
    assert.equal(sim.profile.coins, 110);
    assert.equal(sim.uiRequest, 'scene:true_end');
    assert.equal(sim.encounter, null);
    // a second visit cannot pay again
    Dialogue.openVirtual(sim, 'core', 'query');
    Dialogue.choose(sim, 0);
    assert.deepEqual(choiceKeys(sim), ['c.farewell']);
    Dialogue.choose(sim, 0);
    assert.equal(sim.profile.coins, 110);
    assert.equal(Chronicle.chapter(sim.profile), 'done');
});

check('elder and bard starts follow pages and flags', () => {
    const at = (profile, npcId, meeting) => startOf(newSim(profile), npcId, meeting);
    assert.equal(at({ loot: ['page_01'] }, 'elder', 'elder_home'), 'pages');
    assert.equal(at({ loot: ['page_01'], npcFlags: { 'elder.pages': 1 } }, 'elder', 'elder_home'), 'hello');
    assert.equal(at({ loot: ['page_01'], npcFlags: { 'elder.pages': 1, 'elder.met': 1 } }, 'elder', 'elder_home'), 'again');
    assert.equal(at({ npcFlags: { 'obs.done': 1 } }, 'elder', 'elder_home'), 'home');
    assert.equal(at({ loot: ['page_01'], npcFlags: { 'obs.done': 1 } }, 'elder', 'elder_home'), 'home', 'home before pages');
    assert.equal(at({ npcFlags: { 'obs.done': 1, 'elder.home': 1 } }, 'elder', 'elder_home'), 'hello');
    assert.equal(at({ npcFlags: { 'obs.done': 1 } }, 'bard', 'bard_home'), 'verse');
    assert.equal(at({ npcFlags: { 'obs.done': 1, 'bard.verse': 1 } }, 'bard', 'bard_home'), 'r1');
    const sim = newSim({ loot: ['page_02'] });
    talk(sim, 'elder', 'elder_home');
    assert.equal(textOf(sim), 'elder.pages');
    assert.equal(Npcs.hasFlag(sim, 'elder.pages'), true);
    assert.equal(Dialogue.choose(sim, 0).closed, true);
    // the bard sells three songs for five coins
    const poor = newSim({ coins: 4 });
    talk(poor, 'bard', 'bard_home');
    pick(poor, 'bard.ask_song');
    assert.equal(textOf(poor), 'bard.songs');
    assert.deepEqual(choiceKeys(poor), ['c.farewell'], 'too poor for a verse');
    const rich = newSim({ coins: 50 });
    talk(rich, 'bard', 'bard_home');
    pick(rich, 'bard.ask_song');
    assert.equal(Dialogue.view(rich).choices.length, 4);
    assert.equal(Dialogue.choose(rich, 2).closed, true);
    assert.equal(rich.uiRequest, 'song:story_triumph');
    assert.equal(rich.profile.coins, 45);
});

check('hermit, barkeep, scribe, climber and automaton follow their pages', () => {
    const start = (profile, npcId, meeting) => startOf(newSim(profile), npcId, meeting);
    // hermit: the hint leads to the box, the done node comes once the page is held
    const sim = newSim({});
    talk(sim, 'hermit', 'hermit_home');
    assert.deepEqual(choiceKeys(sim), ['hermit.hello.c1', 'hermit.ask_box', 'c.farewell']);
    Dialogue.choose(sim, 1);
    assert.equal(textOf(sim), 'hermit.box');
    assert.equal(Dialogue.choose(sim, 0).closed, true);
    assert.equal(start({ loot: ['page_01'] }, 'hermit', 'hermit_home'), 'box_done');
    assert.equal(start({ loot: ['page_01'], npcFlags: { 'hermit.box_done': 1 } }, 'hermit', 'hermit_home'), 'hello');
    // barkeep: dice win -> page, or 25 coins -> page
    assert.equal(start({ npcFlags: { 'barkeep.dice_won': 1 } }, 'barkeep', 'barkeep_home'), 'mat_won');
    assert.equal(start({ npcFlags: { 'barkeep.dice_won': 1 }, loot: ['page_03'] }, 'barkeep', 'barkeep_home'), 'mat_done');
    assert.equal(start({ loot: ['page_03'] }, 'barkeep', 'barkeep_home'), 'mat_done');
    const won = newSim({ npcFlags: { 'barkeep.dice_won': 1 } });
    talk(won, 'barkeep', 'barkeep_home');
    assert.deepEqual(choiceKeys(won), ['c.take_page']);
    Dialogue.choose(won, 0);
    assert.equal(Chronicle.has(won.profile, 3), true);
    assert.equal(textOf(won), 'barkeep.mat_done');
    const pay = newSim({ coins: 30 });
    talk(pay, 'barkeep', 'barkeep_home');
    assert.deepEqual(choiceKeys(pay), ['c.trade', 'barkeep.hello.c2', 'c.farewell']);
    Dialogue.choose(pay, 1); // gossip
    assert.deepEqual(choiceKeys(pay), ['c.trade', 'barkeep.ask_mat', 'c.thanks']);
    Dialogue.choose(pay, 1);
    assert.deepEqual(choiceKeys(pay), ['c.play', 'barkeep.mat.c2', 'c.farewell']);
    Dialogue.choose(pay, 1);
    assert.equal(pay.profile.coins, 5);
    assert.equal(Chronicle.has(pay.profile, 3), true);
    const broke = newSim({ coins: 24 });
    talk(broke, 'barkeep', 'barkeep_home');
    Dialogue.choose(broke, 1);
    Dialogue.choose(broke, 1);
    assert.deepEqual(choiceKeys(broke), ['c.play', 'c.farewell'], 'the pay choice is hidden without 25 coins');
    // scribe: wrong answers give nothing, the right one gives page 5
    const sc = newSim({});
    talk(sc, 'scribe', 'scribe_home');
    Dialogue.choose(sc, 0); // orb
    assert.ok(choiceKeys(sc).includes('scribe.ask_stars'));
    Dialogue.choose(sc, 1);
    assert.equal(textOf(sc), 'scribe.stars');
    Dialogue.choose(sc, 0);
    assert.equal(textOf(sc), 'scribe.ask');
    Dialogue.choose(sc, 0);
    assert.equal(textOf(sc), 'scribe.wrong');
    assert.equal(Chronicle.has(sc.profile, 5), false);
    Dialogue.close(sc);
    talk(sc, 'scribe', 'scribe_home');
    pick(sc, 'scribe.ask_stars');
    Dialogue.choose(sc, 0);
    Dialogue.choose(sc, 2);
    assert.equal(textOf(sc), 'scribe.right');
    Dialogue.choose(sc, 0);
    assert.equal(Chronicle.has(sc.profile, 5), true);
    assert.equal(start({ loot: ['page_05'] }, 'scribe', 'scribe_home'), 'stars_done');
    // climber and automaton
    assert.equal(start({ loot: ['page_07'] }, 'climber', 'climber_home'), 'lamps_done');
    assert.equal(start({ loot: ['page_09'] }, 'automaton', 'automaton_home'), 'drawer_done');
    for (const [npcId, meeting, ask, node] of [['climber', 'climber_home', 'climber.ask_lamps', 'climber.lamps'], ['automaton', 'automaton_home', 'automaton.ask_drawer', 'automaton.drawer']]) {
        const s = newSim({});
        talk(s, npcId, meeting);
        assert.equal(Dialogue.view(s).choices.length, 3, npcId + ' first view');
        assert.ok(choiceKeys(s).includes(ask), ask);
        Dialogue.choose(s, 1);
        assert.equal(textOf(s), node);
        assert.equal(Dialogue.choose(s, 0).closed, true);
        const done = newSim({ loot: [npcId === 'climber' ? 'page_07' : 'page_09'] });
        talk(done, npcId, meeting);
        assert.ok(!choiceKeys(done).includes(ask), 'no hint once the page is held');
    }
});

check('Edda and Gundar talk', () => {
    const sim = newSim({});
    assert.equal(startOf(sim, 'edda', 'edda_home'), 'hello');
    talk(sim, 'edda', 'edda_home');
    assert.equal(Npcs.hasFlag(sim, 'edda.met'), true);
    assert.deepEqual(choiceKeys(sim), ['edda.hello.c1', 'edda.hello.c2', 'c.farewell']);
    Dialogue.choose(sim, 0);
    assert.equal(textOf(sim), 'edda.skadi');
    Dialogue.close(sim);
    assert.equal(startOf(sim, 'edda', 'edda_home'), 'again');
    talk(sim, 'gambler', 'gambler_home');
    assert.deepEqual(choiceKeys(sim), ['c.play', 'c.farewell']);
    assert.equal(Dialogue.choose(sim, 0).closed, true);
    assert.equal(sim.uiRequest, 'dice:gambler');
    assert.equal(Npcs.NPCS.journal.portrait, 'edda');
    for (const id of ['journal', 'lore', 'core', 'edda', 'gambler']) assert.equal(Npcs.NPCS[id].duelCoins, 0, id);
    assert.deepEqual(Array.from(sandbox.RunenpfadFolk.SPRITES.edda), ['td-folk-epilogue', 0]);
    assert.deepEqual(Array.from(sandbox.RunenpfadFolk.SPRITES.recruit_f), ['td-folk-epilogue', 1]);
    assert.deepEqual(Array.from(sandbox.RunenpfadFolk.SPRITES.recruit_m), ['td-folk-epilogue', 2]);
    assert.deepEqual(Array.from(sandbox.RunenpfadFolk.SPRITES.gambler), ['td-folk-epilogue', 3]);
});

check('the two chapter scenes exist, never trigger and show in the chronicle once seen', () => {
    assert.equal(Cut.SCENES.obs_intro.panels.length, 1);
    assert.equal(Cut.SCENES.true_end.panels.length, 3);
    assert.equal(Cut.SCENES.obs_intro.music, 'story_dark');
    assert.equal(Cut.SCENES.true_end.music, 'story_triumph');
    assert.deepEqual(Array.from(Cut.SCENES.true_end.panels, (p) => p.art), ['cut-trueend-1', 'cut-trueend-2', 'cut-trueend-3']);
    assert.equal(Cut.SCENES.obs_intro.panels[0].art, 'cut-obs-1');
    for (let level = 0; level <= 26; level++) {
        for (const type of ['stage_start', 'won']) {
            const out = Array.from(Cut.pending({ seen: [] }, { type: type, mode: 'campaign', levelId: level, epilogueKey: 'friend' }));
            assert.ok(!out.includes('obs_intro') && !out.includes('true_end'), type + ' ' + level);
        }
    }
    assert.equal(Cut.chronicle({ seen: [] }).length, 10);
    assert.equal(Cut.chronicle({ seen: ['obs_intro'] }).length, 11);
    const list = Cut.chronicle({ seen: ['end_friend', 'true_end', 'obs_intro'] });
    assert.deepEqual(Array.from(list.slice(-3), (e) => e.id), ['end_friend', 'obs_intro', 'true_end']);
    assert.ok(list.slice(-3).every((e) => e.seen));
});

check('the new modules are DOM-free, short and free of web addresses', () => {
    for (const f of ['chronicle.js', 'journal-story.js', 'dialogue.js', 'cutscenes.js', 'folk.js', 'folk-story.js']) {
        const raw = fs.readFileSync(path.join(rootDir, R + f), 'utf8');
        assert.ok(raw.split('\n').length < 1100, f + ' lines');
        assert.doesNotMatch(raw, /https?:\/\//, f);
        const src = raw.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');
        assert.doesNotMatch(src, /\bdocument\b|\bcanvas\b|Math\.random|Date\.now|performance\./, f);
        assert.doesNotMatch(src.replace(/typeof window !== 'undefined' \? window : globalThis/, ''), /\bwindow\b/, f);
    }
});

console.log('\n' + passed + ' tests passed');
