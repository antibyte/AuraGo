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
