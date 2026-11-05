// Part A: the sfx beat. Impact placement, take rotation, typing keys and auto-SFX against the mini catalog.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const takesOf = id => catalog.entries.find(e => e.id === id).takes;
const sfxEvents = report => plain(report.audio_events).filter(e => e.kind === 'sfx');

check('sfx: a cued whoosh starts at the marker minus the impact of its take', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', narration: 'eins zwei drei {1}vier sechs sieben', beats: [{ do: 'sfx', sound: 'sfx.whoosh', cue: 1 }] }] };
    const marker = plain(SM.planTiming(story, {}, catalog)).scenes.a.cues['1'];
    const out = SM.compile(story, {}, catalog);
    const [ev] = sfxEvents(out.report);
    assert.equal(ev.ref, 'sfx.whoosh');
    assert.equal(ev.scene_id, 'a');
    // Frozen 4.6 step 6a: start = marker + delay - impact. Scene a starts at frame 0, so start_ms is the local time.
    near(ev.start_ms, (marker - takesOf('sfx.whoosh')[ev.take].impact_ms / 1000) * 1000, 1);
    assert.ok(!plain(out.report.issues).some(i => i.code === 'not_supported_yet' || i.code === 'cue_late'));
});

check('sfx: with:"prev" puts the impact on the landing of the previous beat', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', beats: [
        { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' },
        { do: 'sfx', sound: 'sfx.pop', with: 'prev' }
    ] }] };
    const [ev] = sfxEvents(SM.compile(story, {}, catalog).report);
    // The show starts at 0 and lasts the default 0.5 s. A pop on a prop lands at the spring's lead fraction (frozen 4.5).
    const landing = SM.motion.leadFraction('spring') * 0.5;
    near(ev.start_ms, (landing - takesOf('sfx.pop')[ev.take].impact_ms / 1000) * 1000, 1);
});

check('sfx: a repeat never takes the same take, across scenes too', () => {
    const whoosh = { do: 'sfx', sound: 'sfx.whoosh' };
    const sceneA = { id: 'a', transition: 'cut', beats: [whoosh, whoosh, whoosh] };
    const inA = sfxEvents(SM.compile({ scenes: [sceneA] }, {}, catalog).report).map(e => e.take);
    assert.equal(inA.length, 3);
    for (let i = 1; i < inA.length; i++) assert.notEqual(inA[i], inA[i - 1], 'inside a scene');
    // Pick a second scene whose unrotated first take equals the last take of scene a. Only a film-wide lastTake can change it.
    const count = takesOf('sfx.whoosh').length;
    let id = null;
    for (let n = 0; n < 64 && id === null; n++) if (SM.seed.takeIndex('b' + n, 0, 'sfx.whoosh', 0, count) === inA[2]) id = 'b' + n;
    assert.ok(id, 'a scene id with that first take exists');
    const both = sfxEvents(SM.compile({ scenes: [sceneA, { id, transition: 'cut', beats: [whoosh] }] }, {}, catalog).report).map(e => e.take);
    assert.notEqual(both[3], both[2], 'across scenes');
});

check('sfx: a clamp at scene time 0 does not warn', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', narration: '{1}eins zwei drei', beats: [{ do: 'sfx', sound: 'sfx.whoosh', cue: 1 }] }] };
    const out = SM.compile(story, {}, catalog);
    assert.equal(sfxEvents(out.report)[0].start_ms, 0);
    assert.ok(!plain(out.report.issues).some(i => i.code === 'cue_late'));
});

check('sfx: a cued sfx in scene 2 has start_ms from the scene start plus its local time', () => {
    const story = { scenes: [
        { id: 'a', transition: 'cut', duration: 2 },
        { id: 'b', transition: 'cut', narration: 'eins zwei {1}drei vier', beats: [{ do: 'sfx', sound: 'sfx.pop', cue: 1 }] }
    ] };
    const marker = plain(SM.planTiming(story, {}, catalog)).scenes.b.cues['1'];
    const report = SM.compile(story, {}, catalog).report;
    const [ev] = sfxEvents(report);
    const base = report.scenes[1].start_frame / 30;
    assert.ok(base > 0, 'scene b starts after scene a');
    assert.equal(ev.scene_id, 'b');
    near(ev.start_ms, (base + marker - takesOf('sfx.pop')[ev.take].impact_ms / 1000) * 1000, 1);
});

const typingStory = id => ({ scenes: [{ id, transition: 'cut', beats: [{ do: 'sfx', sound: 'sfx.typing', duration: 2 }] }] });

check('typing: keys sit 90 to 140 ms apart inside the duration', () => {
    const x = sfxEvents(SM.compile(typingStory('a'), {}, catalog).report).map(e => e.start_ms + takesOf('sfx.typing')[e.take].impact_ms);
    // The loop stops only when x reaches 2000 ms. x starts at the first take's impact (at most 13 ms in the mini catalog)
    // and grows by at most 140 ms, so there are at least 15 keys (13+140k < 2000 holds up to k = 14). It grows by at least
    // 90 ms, so there are at most floor(2000/90)+1 = 23.
    assert.ok(x.length >= 15 && x.length <= 23, 'key count ' + x.length);
    // start_ms is a rounded whole ms, which moves one gap by at most 1 ms.
    for (let i = 1; i < x.length; i++) {
        const gap = x[i] - x[i - 1];
        assert.ok(gap >= 89 && gap <= 141, 'gap ' + gap);
    }
    assert.ok(x[x.length - 1] < 2001, 'the last key sits inside the duration');
});

check('typing: takes are seeded and two compiles match', () => {
    const once = sfxEvents(SM.compile(typingStory('a'), {}, catalog).report);
    assert.deepEqual(sfxEvents(SM.compile(typingStory('a'), {}, catalog).report), once);
    for (let i = 1; i < once.length; i++) assert.notEqual(once[i].take, once[i - 1].take, 'no take twice in a row');
    const other = sfxEvents(SM.compile(typingStory('b'), {}, catalog).report);
    assert.notDeepEqual(other.map(e => e.start_ms), once.map(e => e.start_ms), 'the scene id seeds the spacing');
});

check('typing: the default duration scales with rhythm', () => {
    const frames = rhythm => SM.compile({ style: { rhythm }, scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'sfx', sound: 'sfx.typing' }] }] }, {}, catalog).report.scenes[0].end_frame;
    // 1200 ms times the factor at 30 fps: calm 1.5 s = 45 frames, standard 1.2 s = 36 frames. Lively is 0.96 s, under the
    // 1.0 s minimum of a scene without speech, so it stays at 30 frames.
    assert.equal(frames('calm'), 45);
    assert.equal(frames('standard'), 36);
    assert.equal(frames('lively'), 30);
});
