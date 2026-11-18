import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const R = 'ui/js/desktop/apps/runenpfad/';

function loadModule(rel, sandbox) {
    const abs = path.join(rootDir, rel);
    vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const sandbox = { console, Math, Number, String, Boolean, Array, Object, JSON };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
loadModule(R + 'cutscenes.js', sandbox);
loadModule(R + 'cutscene-view.js', sandbox);

const Cut = sandbox.RunenpfadCutscenes;
const View = sandbox.RunenpfadCutsceneView;
assert.ok(Cut && View, 'cutscene modules loaded');

let passed = 0;
function test(name, fn) {
    fn();
    passed += 1;
    console.log('ok  ' + name);
}

const locales = {};
['en', 'de'].forEach((loc) => {
    locales[loc] = JSON.parse(fs.readFileSync(path.join(rootDir, 'ui/lang/runenpfad', loc + '.json'), 'utf8'));
});

const EXPECTED_IDS = ['prologue', 'intro_2', 'intro_3', 'intro_4', 'intro_5', 'after_1', 'after_2', 'after_3', 'after_4', 'finale', 'end_friend', 'end_neutral', 'end_feared'];
const ART_RE = /^cut-(prologue-[1-5]|intro[2-5]-[12]|after[1-4]-[12]|finale-[1-3]|end-(friend|neutral|feared))$/;
const FG_FROM_CONTRACT = ['cutfg-woodland', 'cutfg-cavern', 'cutfg-desert', 'cutfg-glacier', 'cutfg-citadel'];
const FX = ['motes', 'embers', 'snow', 'mist', 'sand', 'sparks', 'rain'];
const MUSIC = ['story_dawn', 'story_dark', 'story_triumph'];

test('scene table is valid', () => {
    assert.deepEqual(Array.from(Cut.IDS), EXPECTED_IDS);
    assert.equal(Object.keys(Cut.SCENES).length, 13);
    let panels = 0;
    const arts = new Set();
    for (const id of Cut.IDS) {
        const s = Cut.SCENES[id];
        assert.equal(s.id, id);
        assert.equal(s.titleKey, 'runenpfad.cut.title.' + id);
        assert.ok(MUSIC.includes(s.music), id + ' music');
        assert.ok(s.panels.length >= 1);
        for (const p of s.panels) {
            panels += 1;
            assert.match(p.art, ART_RE, p.art);
            assert.ok(!arts.has(p.art), 'duplicate art ' + p.art);
            arts.add(p.art);
            assert.ok(p.fg === null || FG_FROM_CONTRACT.includes(p.fg), id + ' fg ' + p.fg);
            for (const pt of [p.pan.from, p.pan.to]) {
                assert.equal(pt.length, 3);
                assert.ok(pt[0] >= 0 && pt[0] <= 1 && pt[1] >= 0 && pt[1] <= 1 && pt[2] >= 1, id + ' pan');
            }
            p.fx.forEach((f) => assert.ok(FX.includes(f), 'fx ' + f));
            p.lights.forEach((l) => {
                assert.ok(l.x >= 0 && l.x <= 1 && l.y >= 0 && l.y <= 1 && l.r > 0 && l.r <= 0.5, id + ' light');
                assert.match(l.color, /^#[0-9a-f]{6}$/i);
            });
            assert.ok(p.sprites.length <= 3);
            p.sprites.forEach((sp) => {
                assert.match(sp.clip, /^[a-z0-9-]+\.[a-z0-9_]+$/);
                assert.ok(sp.h > 0 && sp.h <= 0.5 && sp.from.length === 2 && sp.to.length === 2);
            });
            assert.ok(p.lines.length >= 1 && p.lines.length <= 2, id + ' lines');
        }
    }
    assert.equal(panels, 27);
});

test('every sprite clip of the table exists in the atlas', () => {
    const atlasFile = path.join(rootDir, 'ui/img/runenpfad/atlas.json');
    const atlas = JSON.parse(fs.readFileSync(atlasFile, 'utf8'));
    for (const id of Cut.IDS) {
        for (const p of Cut.SCENES[id].panels) {
            for (const sp of p.sprites) {
                const [sheet, clip] = sp.clip.split('.');
                assert.ok(atlas.sheets[sheet] && atlas.sheets[sheet].clips[clip], 'atlas clip ' + sp.clip);
            }
        }
    }
});

test('titles and lines exist in en and de with sane lengths', () => {
    for (const [loc, max] of [['en', 120], ['de', 140]]) {
        const dict = locales[loc];
        for (const id of Cut.IDS) {
            const s = Cut.SCENES[id];
            assert.ok(typeof dict[s.titleKey] === 'string' && dict[s.titleKey].trim(), loc + ' ' + s.titleKey);
            assert.ok(dict[s.titleKey].length <= 40, loc + ' title length ' + s.titleKey);
            for (const p of s.panels) {
                for (const key of p.lines) {
                    const text = dict[key];
                    assert.ok(typeof text === 'string' && text.trim(), loc + ' missing ' + key);
                    assert.ok(text.length <= max, loc + ' ' + key + ' is ' + text.length + ' chars');
                }
            }
        }
        for (const key of ['next', 'skip', 'locked', 'chronicle', 'replay']) {
            assert.ok(dict['runenpfad.cut.' + key], loc + ' runenpfad.cut.' + key);
        }
    }
});

test('no orphan runenpfad.cut line keys', () => {
    const known = new Set(['next', 'skip', 'locked', 'chronicle', 'replay']);
    Cut.IDS.forEach((id) => {
        known.add('title.' + id);
        Cut.SCENES[id].panels.forEach((p) => p.lines.forEach((k) => known.add(k.slice('runenpfad.cut.'.length))));
    });
    for (const loc of ['en', 'de']) {
        Object.keys(locales[loc]).filter((k) => k.startsWith('runenpfad.cut.')).forEach((k) => {
            assert.ok(known.has(k.slice('runenpfad.cut.'.length)), loc + ' orphan ' + k);
        });
    }
});

test('the end scenes agree with the epilogue plaques in tone', () => {
    // The plaque names the covens/circle/kobolds (friend), a quiet path (neutral) and locked doors (feared).
    assert.match(locales.en['runenpfad.cut.end_friend.1b'], /covens.*circle.*kobold/i);
    assert.match(locales.en['runenpfad.cut.end_neutral.1b'], /quiet/i);
    assert.match(locales.en['runenpfad.cut.end_feared.1a'], /shutters|whisper/i);
    assert.ok(locales.en['runenpfad.npc.end_friend'] && locales.en['runenpfad.npc.end_feared']);
});

// ---- pending ----
const camp = (type, levelId, extra) => Object.assign({ type: type, mode: 'campaign', levelId: levelId }, extra || {});

test('pending is table-driven for every trigger', () => {
    const starts = { 1: 'prologue', 6: 'intro_2', 11: 'intro_3', 16: 'intro_4', 21: 'intro_5' };
    const wins = { 5: 'after_1', 10: 'after_2', 15: 'after_3', 20: 'after_4' };
    for (let level = 1; level <= 25; level++) {
        const s = Cut.pending({ seen: [] }, camp('stage_start', level));
        // the prologue plays on any first campaign start; world intros follow on their own level
        const want = level === 1 ? ['prologue'] : starts[level] ? ['prologue', starts[level]] : ['prologue'];
        assert.deepEqual(Array.from(s), want, 'start ' + level);
        const later = Cut.pending({ seen: ['prologue'] }, camp('stage_start', level));
        assert.deepEqual(Array.from(later), starts[level] && level !== 1 ? [starts[level]] : [], 'start (prologue seen) ' + level);
        const w = Cut.pending({ seen: [] }, camp('won', level, { epilogueKey: 'runenpfad.npc.end_friend' }));
        if (wins[level]) assert.deepEqual(Array.from(w), [wins[level]], 'won ' + level);
        else if (level === 25) assert.deepEqual(Array.from(w), ['finale', 'end_friend']);
        else assert.deepEqual(Array.from(w), [], 'won ' + level);
    }
});

test('pending: endless never triggers, seen scenes never return', () => {
    for (const type of ['stage_start', 'won']) {
        for (const level of [1, 5, 6, 25]) {
            assert.deepEqual(Array.from(Cut.pending({ seen: [] }, { type: type, mode: 'endless', levelId: level })), []);
        }
    }
    assert.deepEqual(Array.from(Cut.pending({ seen: ['prologue'] }, camp('stage_start', 1))), []);
    assert.deepEqual(Array.from(Cut.pending({ seen: ['after_2'] }, camp('won', 10))), []);
    assert.deepEqual(Array.from(Cut.pending({ seen: ['after_1'] }, camp('stage_start', 1))), ['prologue']);
    assert.deepEqual(Array.from(Cut.pending({ seen: [] }, camp('stage_start', 13))), ['prologue'], 'prologue on any level');
    assert.deepEqual(Array.from(Cut.pending({ seen: [] }, camp('stage_start', 0))), [], 'hub never triggers');
    assert.deepEqual(Array.from(Cut.pending({}, camp('stage_start', 1))), ['prologue'], 'profile without seen');
    assert.deepEqual(Array.from(Cut.pending(null, camp('stage_start', 6))), ['prologue', 'intro_2'], 'no profile');
    assert.deepEqual(Array.from(Cut.pending({ seen: [] }, null)), []);
    assert.deepEqual(Array.from(Cut.pending({ seen: [] }, { type: 'dead', mode: 'campaign', levelId: 1 })), []);
});

test('pending: finale brings the chosen end scene once', () => {
    const keys = { 'runenpfad.npc.end_friend': 'end_friend', 'runenpfad.npc.end_neutral': 'end_neutral', 'runenpfad.npc.end_feared': 'end_feared', friend: 'end_friend', end_feared: 'end_feared', junk: 'end_neutral' };
    for (const [key, end] of Object.entries(keys)) {
        assert.deepEqual(Array.from(Cut.pending({ seen: [] }, camp('won', 25, { epilogueKey: key }))), ['finale', end], key);
    }
    assert.deepEqual(Array.from(Cut.pending({ seen: [] }, camp('won', 25))), ['finale', 'end_neutral'], 'no epilogue key');
    assert.deepEqual(Array.from(Cut.pending({ seen: ['finale'] }, camp('won', 25, { epilogueKey: 'friend' }))), [], 'finale seen: nothing');
    assert.deepEqual(Array.from(Cut.pending({ seen: ['end_friend'] }, camp('won', 25, { epilogueKey: 'friend' }))), ['finale'], 'end already seen');
});

// ---- player state machine ----
function typeLine(st, len) {
    Cut.setLineLength(st, len);
    return st;
}

test('typewriter runs at 40 characters per second and stops at the line length', () => {
    const st = Cut.create(['after_3']);
    assert.equal(st.done, false);
    typeLine(st, 100);
    Cut.step(st, 0.25);
    Cut.step(st, 0.25);
    assert.equal(st.chars, 20);
    assert.equal(st.lineDone, false);
    Cut.step(st, 0.25);
    Cut.step(st, 0.25);
    assert.equal(st.chars, 40);
    Cut.step(st, 10);
    assert.equal(st.chars, 50, 'a long frame is clamped to 0.25 s');
    for (let i = 0; i < 20; i++) Cut.step(st, 0.2);
    assert.equal(st.chars, 100);
    assert.equal(st.lineDone, true);
    assert.equal(st.panelT > 5, true);
    const cur = Cut.current(st);
    assert.equal(cur.scene.id, 'after_3');
    assert.equal(cur.lineKey, 'runenpfad.cut.after_3.1a');
});

test('advance completes the line first, then walks lines, panels, scenes and ends', () => {
    const st = Cut.create(['intro_2', 'after_1']);
    const visited = [];
    let guard = 0;
    while (!st.done && guard++ < 200) {
        const cur = Cut.current(st);
        visited.push(cur.lineKey);
        typeLine(st, 30);
        Cut.step(st, 0.1); // mid-typing
        assert.equal(st.lineDone, false);
        assert.equal(Cut.advance(st), 'complete');
        assert.equal(st.chars, 30);
        assert.equal(st.lineDone, true);
        Cut.advance(st);
    }
    assert.equal(st.done, true);
    assert.deepEqual(visited, [
        'runenpfad.cut.intro_2.1a', 'runenpfad.cut.intro_2.1b', 'runenpfad.cut.intro_2.2a', 'runenpfad.cut.intro_2.2b',
        'runenpfad.cut.after_1.1a', 'runenpfad.cut.after_1.1b', 'runenpfad.cut.after_1.2a', 'runenpfad.cut.after_1.2b'
    ]);
    assert.equal(Cut.current(st), null);
    assert.equal(Cut.advance(st), 'none');
});

test('advance reports panel and scene changes and resets the line state', () => {
    const st = Cut.create(['intro_3', 'intro_4']);
    const seq = [];
    let guard = 0;
    while (!st.done && guard++ < 50) {
        typeLine(st, 5);
        Cut.advance(st);
        seq.push(Cut.advance(st));
        if (!st.done) {
            assert.equal(st.chars, 0);
            assert.equal(st.lineDone, false);
            assert.equal(st.t, 0);
        }
    }
    assert.deepEqual(seq, ['line', 'panel', 'line', 'scene', 'line', 'panel', 'line', 'done']);
});

test('a line of length 0 and a state without setLineLength never trap the player', () => {
    const st = Cut.create(['end_neutral']);
    Cut.setLineLength(st, 0);
    assert.equal(st.lineDone, true);
    const st2 = Cut.create(['end_neutral']);
    Cut.step(st2, 0.2);
    assert.equal(Cut.advance(st2), 'complete');
    assert.equal(Cut.advance(st2), 'line');
});

test('skip ends the whole queue; create filters unknown and duplicate ids', () => {
    const st = Cut.create(['prologue', 'finale']);
    Cut.skip(st);
    assert.equal(st.done, true);
    assert.equal(Cut.current(st), null);
    Cut.step(st, 1);
    const f = Cut.create(['nope', 'finale', 'finale', 7, null]);
    assert.deepEqual(Array.from(f.ids), ['finale']);
    assert.equal(Cut.create([]).done, true);
    assert.equal(Cut.create(undefined).done, true);
    assert.doesNotThrow(() => { Cut.step(null, 1); Cut.advance(null); Cut.skip(null); Cut.current(null); Cut.setLineLength(null, 3); });
});

test('state fields follow the contract', () => {
    const st = Cut.create(['prologue']);
    for (const k of ['ids', 'sceneIndex', 'panelIndex', 'lineIndex', 't', 'panelT', 'chars', 'lineDone', 'done']) assert.ok(k in st, k);
    assert.equal(st.sceneIndex, 0);
    assert.equal(st.panelIndex, 0);
    assert.equal(st.lineIndex, 0);
});

test('chronicle lists the 10 main scenes and only the seen end scene', () => {
    let list = Cut.chronicle({ seen: [] });
    assert.equal(list.length, 10);
    assert.ok(list.every((e) => e.seen === false && e.titleKey === 'runenpfad.cut.title.' + e.id));
    assert.deepEqual(Array.from(list, (e) => e.id), EXPECTED_IDS.slice(0, 10));
    list = Cut.chronicle({ seen: ['prologue', 'after_2', 'finale', 'end_feared'] });
    assert.equal(list.length, 11);
    assert.deepEqual(Array.from(list.filter((e) => e.seen), (e) => e.id), ['prologue', 'after_2', 'finale', 'end_feared']);
    assert.equal(list[10].id, 'end_feared');
    assert.equal(Cut.chronicle(null).length, 10);
    // old saves: progress beyond a trigger level counts as seen
    const old = Cut.chronicle({ seen: [], unlockedLevel: 12 });
    assert.deepEqual(Array.from(old.filter((e) => e.seen), (e) => e.id), ['prologue', 'intro_2', 'intro_3', 'after_1', 'after_2']);
    assert.equal(Cut.chronicle({ seen: [], unlockedLevel: 1 }).filter((e) => e.seen).length, 0);
    assert.equal(Cut.chronicle({}).length, 10);
});

// ---- view: paints without art, never throws ----
function fakeCanvas() {
    const calls = { fillRect: 0, drawImage: 0, fillText: 0 };
    const grad = { addColorStop() {} };
    const ctx = new Proxy({}, {
        get(_, prop) {
            if (prop === 'createLinearGradient' || prop === 'createRadialGradient') return () => grad;
            if (prop === 'fillRect' || prop === 'drawImage' || prop === 'fillText') return () => { calls[prop] += 1; };
            return typeof prop === 'string' && /^[a-z]/.test(prop) && !(prop in {}) ? () => {} : undefined;
        },
        set() { return true; }
    });
    return { width: 640, height: 360, getContext: () => ctx, calls };
}

test('view renders every panel without art, with reduced motion and with a broken canvas', () => {
    const cv = fakeCanvas();
    const view = View.create(cv, { t: (k) => k });
    assert.doesNotThrow(() => view.prime(Cut.IDS.slice()));
    const st = Cut.create(Cut.IDS.slice());
    let now = 1000;
    let guard = 0;
    while (!st.done && guard++ < 200) {
        for (let f = 0; f < 3; f++) { now += 700; Cut.step(st, 0.7); view.render(st, now, f === 2); }
        typeLine(st, 5);
        Cut.advance(st);
        Cut.advance(st);
    }
    assert.ok(cv.calls.fillRect > 0, 'painted something');
    assert.ok(cv.calls.fillText > 0, 'fallback title drawn');
    assert.doesNotThrow(() => view.render(st, now, false), 'finished state');
    assert.doesNotThrow(() => view.render(null, now, false), 'no state');
    view.dispose();
    assert.doesNotThrow(() => view.render(Cut.create(['prologue']), now, false), 'after dispose');
    const bad = View.create({ width: 10, height: 10, getContext: () => null });
    assert.doesNotThrow(() => { bad.prime(['prologue']); bad.render(Cut.create(['prologue']), 5, false); bad.dispose(); });
    const none = View.create(null);
    assert.doesNotThrow(() => none.render(Cut.create(['prologue']), 5, false));
});

test('DOM-free module stays free of document, window, canvas and random numbers', () => {
    const src = fs.readFileSync(path.join(rootDir, R + 'cutscenes.js'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    assert.doesNotMatch(src, /\bdocument\b|\bcanvas\b|Math\.random|Date\.now|performance\./);
    assert.doesNotMatch(src.replace(/typeof window !== 'undefined' \? window : globalThis/, ''), /\bwindow\b/);
});

test('files stay below the line budget and use no innerHTML', () => {
    for (const f of ['cutscenes.js', 'cutscene-view.js', 'cutscene-ui.js']) {
        const src = fs.readFileSync(path.join(rootDir, R + f), 'utf8');
        assert.ok(src.split('\n').length < 1100, f);
        assert.doesNotMatch(src, /innerHTML|outerHTML|insertAdjacentHTML/);
    }
});

console.log('\n' + passed + ' tests passed');
