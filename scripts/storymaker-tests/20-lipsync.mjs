// Task S10: lip-sync: the level table, the seeded variation, dips, rest without levels, the track on a cast speaker and on a say.
import assert from 'node:assert/strict';
import { plain, renderAssets } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog, files } = t;
const SM = t.core();

const flat = (level, id = 'a:n') => ({ id, levels: Array(400).fill(level) });
const shapes = level => [...new Set(plain(SM.lipsync.track(flat(level))))].sort();

// One second of speech at 30 fps: ceil(1000 * 30 / 1000) = 30 levels, all in the mid band. Frame k plays at t = k / 30.
const LEVELS = Array(30).fill(150);
const block = (id, kind, extra) => ({
    id, kind, speaker: 'anna', duration_ms: 1000, spans: [[0, 1000]], speech_ok: true, levels: LEVELS,
    pieces: [{ hash: 'a'.repeat(64), ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 1000, chars: 10 }], ...extra
});
const cast = [{ id: 'anna', character: 'char.office-woman-1' }];
const actors = [{ cast: 'anna', at: 'center' }];
const speakerStory = speaker => ({ cast, scenes: [{ id: 'a', speaker, narration: 'Hallo Welt', actors }] });
const sayStory = { cast, scenes: [{ id: 'b', actors, beats: [{ do: 'say', who: 'anna', text: 'Hallo Welt', style: 'speech' }] }] };
const narrationAudio = () => ({ narration: { a: { blocks: [block('a:n', 'narration')] } } });
const actorOf = out => out.timeline.scenes[0].entities.find(e => e.type === 'actor');

function assertTrack(out, id) {
    const actor = actorOf(out), expected = plain(SM.lipsync.track({ id, levels: LEVELS }));
    // both blocks start at t = 0: the first scene has no entry transition and nothing runs before them
    for (const k of [0, 7, 15, 29]) assert.equal(SM.scene.evalEntity(actor, k / 30).mouth, expected[k], id + ' frame ' + k);
    assert.equal(SM.scene.evalEntity(actor, 1.1).mouth, null, id + ': after the block the expression decides the mouth again');
}

check('lipsync: the level table maps to mouths, with the seeded variation at 0.25', () => {
    // flat levels have no dip, so each frame goes through the table alone
    assert.deepEqual(shapes(0), ['rest']);
    assert.deepEqual(shapes(84), ['rest']);
    assert.deepEqual(shapes(85), ['e', 'fv']);
    assert.deepEqual(shapes(127), ['e', 'fv']);
    assert.deepEqual(shapes(128), ['ai', 'e']);
    assert.deepEqual(shapes(170), ['ai', 'e']);
    assert.deepEqual(shapes(171), ['o', 'u']);
    assert.deepEqual(shapes(255), ['o', 'u']);
    // 400 draws at p = 0.25: mean 100, sigma 8.7, so this band is 4.6 sigma wide on a fixed seed
    const variation = plain(SM.lipsync.track(flat(100))).filter(m => m === 'fv').length;
    assert.ok(variation >= 60 && variation <= 140, 'fv on ' + variation + ' of 400 frames');
});

check('lipsync: two runs are identical and the block id is the seed', () => {
    const first = plain(SM.lipsync.track(flat(150)));
    assert.deepEqual(plain(SM.lipsync.track(flat(150))), first);
    assert.notDeepEqual(plain(SM.lipsync.track(flat(150, 'a:b1'))), first, 'another block id gives another variation');
});

check('lipsync: a dip of 40 below both neighbours is mbp', () => {
    const track = levels => plain(SM.lipsync.track({ id: 'dip', levels }));
    assert.equal(track([140, 100, 140])[1], 'mbp', 'exactly 40 below both');
    assert.equal(track([140, 50, 140])[1], 'mbp', 'the dip rule wins over rest');
    assert.ok(['e', 'fv'].includes(track([140, 101, 140])[1]), '39 below is the low band');
    assert.notEqual(track([140, 100, 120])[1], 'mbp', 'only 20 below the right neighbour');
    const edges = track([100, 140, 100]);
    assert.notEqual(edges[0], 'mbp', 'the first frame has one neighbour');
    assert.notEqual(edges[2], 'mbp', 'the last frame has one neighbour');
});

check('lipsync: no levels give rest', () => {
    assert.deepEqual(plain(SM.lipsync.track({ id: 'x' })), [], 'no levels: no track');
    assert.deepEqual(plain(SM.lipsync.track({ id: 'x', levels: [] })), []);
    assert.equal(SM.lipsync.mouthAt({ t0: 0, t1: 1, data: { block: { id: 'x', levels: [] } } }, 0.5), 'rest');
    // an estimated say has no levels, so it gets no speech interval and the rig uses the mouth of the expression
    const actor = actorOf(SM.compile(sayStory, {}, catalog));
    assert.equal(actor.speech.list.length, 0);
    assert.equal(SM.scene.evalEntity(actor, 0.5).mouth, null);
});

check('lipsync: a cast speaker and a say who both get the track; a narrator scene moves no mouth', () => {
    const spoken = SM.compile(speakerStory('anna'), narrationAudio(), catalog);
    assertTrack(spoken, 'a:n');
    const st = SM.scene.evalEntity(actorOf(spoken), 0.5);
    assert.ok(['ai', 'e'].includes(st.mouth), 'level 150 is in the mid band');
    const rc = SM.render.makeContext(spoken.timeline, renderAssets(), 1920, 1080);
    assert.equal(SM.render.rigState(st, 0.5, rc, files.rig).mouth, st.mouth, 'render passes the mouth to the rig');

    const said = SM.compile(sayStory, { narration: { b: { blocks: [block('b:b0', 'dialogue', { beat: 0 })] } } }, catalog);
    assertTrack(said, 'b:b0');

    const narrated = SM.compile(speakerStory('narrator'), narrationAudio(), catalog);
    assert.equal(actorOf(narrated).speech.list.length, 0);
});
