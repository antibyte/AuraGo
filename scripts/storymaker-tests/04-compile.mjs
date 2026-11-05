// Task C5: compile: scene-local timing, cues, auto durations, transition overlap, beats, the support matrix.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, firewall, minimal } = t;
const compiled = t.compiled(), sceneById = t.sceneById;
const SM = t.core();

check('compile: scene ranges in frames, transitions overlap, the first scene has no entry transition', () => {
    const r = plain(compiled.report);
    assert.equal(r.fps, 30);
    assert.deepEqual(r.scenes.map(s => [s.id, s.kind]), [['s1', 'storyboard'], ['s2', 'storyboard'], ['s3', 'code']]);
    // s1: 9 words / 2.3 = 3.913 s + 0.4 s tail = 4.313 s = 129 frames (no entry transition although it says fade)
    assert.deepEqual([r.scenes[0].start_frame, r.scenes[0].end_frame], [0, 129]);
    // s2: fade 0.5 s = 15 frames overlap: starts at 129 - 15
    assert.equal(r.scenes[1].start_frame, 129 - 15);
    assert.ok(r.scenes[1].start_frame < r.scenes[0].end_frame, 'transitions overlap');
    assert.equal(r.scenes[2].start_frame, r.scenes[1].end_frame - 15);
    assert.equal(r.scenes[2].end_frame - r.scenes[2].start_frame, 150, 'the code scene lasts api.duration(5) = 150 frames');
    assert.equal(r.frame_count, Math.max(...r.scenes.map(s => s.end_frame)));
    assert.equal(r.duration_ms, Math.round(r.frame_count * 1000 / 30));
    assert.equal(r.width, 1920); assert.equal(r.height, 1080); assert.equal(r.catalog_version, 'mini00000000');
    assert.deepEqual(r.audio_events, []); assert.deepEqual(r.captions, []); assert.deepEqual(r.checks, []);
    assert.deepEqual(plain(compiled.schedule.scenes.map(s => s.tf)), [0, 15, 15]);
});

check('compile: cue placement is proportional to the character position over the estimated duration', () => {
    const plan = plain(SM.planTiming(firewall, {}, catalog)).scenes.s1;
    const spoken = 'Anna arbeitet im Homeoffice. Plötzlich klopft ein Unbekannter an.';
    const pos = 'Anna arbeitet im Homeoffice. '.length;
    const seconds = 9 / 2.3;
    near(plan.narration_seconds, seconds, 1e-9);
    near(plan.cues['1'], 0 + pos / spoken.length * seconds, 1e-9, 'cue 1');
    // the cue beat starts at that time: the surprised expression appears there
    const anna = sceneById('s1').entities.find(e => e.id === 'anna');
    const step = anna.expr.steps.find(s => s.v === 'surprised');
    near(step.t, plan.cues['1'], 1e-9);
    // a later scene: the cue origin is its own scene start, offset by its entry transition
    const fw2 = plain(SM.planTiming({ ...firewall, scenes: [firewall.scenes[0], { id: 'x', narration: 'eins zwei {1}drei vier', transition: 'fade' }] }, {}, catalog)).scenes.x;
    near(fw2.narration_start, 0.5, 1e-9);
    near(fw2.cues['1'], 0.5 + (10 / 19) * (4 / 2.3), 1e-9);
});

check('compile: auto durations, silent scenes, explicit durations and measured audio', () => {
    const min = SM.compile(minimal, {}, catalog);
    assert.equal(min.report.scenes[0].end_frame, 42, '"Hallo Welt." is 2 words: the 1 s minimum + 0.4 s tail');
    const silent = SM.compile({ scenes: [{ id: 'a', silent: true, narration: 'ignored words ignored' }, { id: 'b', transition: 'cut' }] }, {}, catalog).report;
    assert.deepEqual(plain(silent.scenes.map(s => [s.start_frame, s.end_frame])), [[0, 30], [30, 60]], 'silent scenes last 1 s');
    const explicit = SM.compile({ scenes: [{ id: 'a', duration: 2.5, narration: 'a b c d e f g h i j k l m n o p' }] }, {}, catalog).report;
    assert.equal(explicit.scenes[0].end_frame, 75);
    assert.ok(explicit.issues.some(i => i.code === 'invalid_value' && i.path === '/scenes/0/duration'), 'narration longer than the scene is a warning');
    // beats extend an auto scene
    const longBeat = SM.compile({ scenes: [{ id: 'a', beats: [{ do: 'wait', seconds: 4 }] }] }, {}, catalog).report;
    assert.equal(longBeat.scenes[0].end_frame, 120);
    // measured audio replaces the estimate and produces audio events
    const measured = SM.compile(minimal, { narration: { s1: { blocks: [{ hash: 'abc', duration_ms: 2500 }] } } }, catalog);
    assert.equal(measured.report.scenes[0].end_frame, Math.round(2.9 * 30));
    assert.deepEqual(plain(measured.report.audio_events), [{ kind: 'narration', ref: 'abc', start_ms: 0, gain: 1 }]);
    assert.ok(!measured.report.issues.some(i => i.code === 'narration_missing'));
    assert.ok(compiled.report.issues.filter(i => i.code === 'narration_missing').length === 3 && compiled.report.issues.every(i => i.code !== 'narration_missing' || i.severity === 'warning'));
});

check('compile: beat scheduling with "with: prev", cue, delay and duration', () => {
    const s2 = sceneById('s2'), anna = s2.entities.find(e => e.id === 'anna');
    const walk = anna.walks.list[0], gesture = anna.gestures.list[0];
    near(walk.t0, 0.5, 1e-9, 'beats start when the entry transition ends');
    near(gesture.t0, walk.t0, 1e-9, 'with: prev starts together with the previous beat');
    near(gesture.t1 - gesture.t0, 1.2, 1e-9, 'the gesture lasts the catalog duration');
    // walk duration from the distance at the catalog speed: left 30% -> centre 50% = 384 px at 12% of 1920 px/s x depth 0.973
    near(walk.t1 - walk.t0, 384 / (0.12 * 1920 * (0.82 + 0.23 * (16 / 24))), 0.02, 'walk duration');
    const shield = s2.entities.find(e => e.item === 'prop.shield');
    // the shield's target differs from the gesture's, so it starts at 0.4 of the anchor; the anchor stays on the walk, the longest beat of the with-prev group
    near(shield.alpha.segs[0].t0, walk.t0 + 0.4 * (walk.t1 - walk.t0), 1e-9, 'the next beat on another target overlaps the anchor');
    const custom = SM.compile({ scenes: [{ id: 'a', transition: 'cut', narration: 'eins zwei drei {1}vier fünf sechs', actors: [], beats: [
        { do: 'wait', seconds: 1 },
        { do: 'wait', seconds: 2, delay: 0.5 },
        { do: 'wait', seconds: 0.5, with: 'prev' },
        { do: 'wait', cue: 1, duration: 0.25 },
        { do: 'wait', cue: 7 },
        { do: 'wait', seconds: 1 }
    ] }] }, {}, catalog);
    const ends = custom.report.scenes[0].end_frame;
    const cueT = 15 / 30 * (6 / 2.3);
    assert.ok(custom.report.issues.some(i => i.code === 'unknown_cue' && i.path === '/scenes/0/beats/4/cue'), 'a missing cue marker is reported');
    // chain: 0-1, 1.5-3.5 (delay), with prev at 1.5-2, the cue beat re-anchors at the cue time (0.25 s), the unknown cue follows it (1 s), the last beat 1 s
    const expectedEnd = Math.max(3.5, cueT + 0.25 + 1 + 1);
    near(ends / 30, expectedEnd, 0.02, 'scene end');
});

check('compile: narration windows never overlap (vacuous in phase 1, kept as an assertion)', () => {
    assert.deepEqual(plain(SM.overlappingNarration(compiled.timeline)), []);
    assert.equal(compiled.timeline.narrationEvents.length, 3);
    const ev = compiled.timeline.narrationEvents;
    for (let i = 1; i < ev.length; i++) assert.ok(ev[i].start >= ev[i - 1].end + 0.38, 'the 0.4 s tail separates the narration blocks');
    const heavy = SM.compile({ scenes: Array.from({ length: 6 }, (_, i) => ({ id: 's' + i, narration: 'ein längerer Satz mit mehreren Worten für Szene ' + i })) }, {}, catalog);
    assert.deepEqual(plain(SM.overlappingNarration(heavy.timeline)), []);
    const measured = SM.compile({ scenes: [{ id: 'a', narration: 'x y' }, { id: 'b', narration: 'x y' }] }, { narration: { a: { blocks: [{ hash: 'h1', duration_ms: 3000 }] }, b: { blocks: [{ hash: 'h2', duration_ms: 1000 }, { hash: 'h3', duration_ms: 500 }] } } }, catalog);
    assert.deepEqual(plain(SM.overlappingNarration(measured.timeline)), []);
    assert.equal(measured.report.audio_events.length, 3);
});

check('compile: the phase-1 support matrix reports not_supported_yet and falls back', () => {
    const s = { schema: 'storymaker/1', cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [
        { id: 'a', actors: [{ cast: 'anna', pose: 'wave', expression: 'laugh' }], beats: [
            { do: 'highlight', target: 'anna', style: 'pulse' }, { do: 'say', who: 'anna', text: 'x', style: 'speech' }, { do: 'sfx', sound: 'sfx.pop' },
            { do: 'chart', chart: 'bar', data: { labels: [], values: [] }, at: 'right' }, { do: 'show', icon: 'icon.x', at: 'left' }, { do: 'show', chart: 'bar', at: 'left' },
            { do: 'gesture', who: 'anna', gesture: 'shrug' }, { do: 'pose', who: 'anna', pose: 'flying' }
        ] },
        { id: 'b', transition: 'zoom' }, { id: 'c', transition: 'slide-right' }, { id: 'd', transition: 'wipe' }, { id: 'e', transition: 'slide-left' }, { id: 'f', transition: 'blink' }] };
    const out = SM.compile(s, {}, catalog), issues = plain(out.report.issues);
    const codes = (path, code) => issues.filter(i => i.path === path && i.code === code);
    assert.equal(codes('/scenes/0/beats/0', 'not_supported_yet').length, 0, 'highlight is played');
    for (const j of [1, 2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
    assert.equal(codes('/scenes/0/beats/4', 'not_supported_yet').length, 1, 'show with an icon');
    assert.equal(codes('/scenes/0/beats/5', 'not_supported_yet').length, 1, 'show with a chart');
    assert.ok(issues.every(i => i.code !== 'not_supported_yet' || i.severity === 'minor'));
    assert.equal(codes('/scenes/0/actors/0/pose', 'not_supported_yet').length, 1, 'a known pose that the catalog lacks');
    assert.equal(codes('/scenes/0/actors/0/expression', 'not_supported_yet').length, 1);
    assert.equal(codes('/scenes/0/beats/6/gesture', 'not_supported_yet').length, 1);
    assert.equal(codes('/scenes/0/beats/7/pose', 'unknown_name').length, 1, 'an unknown pose name');
    const actor = out.timeline.scenes[0].entities[0];
    assert.equal(actor.pose.init, 'stand', 'the pose falls back to stand');
    assert.equal(actor.expr.init, 'neutral');
    assert.equal(codes('/scenes/1/transition', 'not_supported_yet').length, 1, 'zoom');
    assert.equal(codes('/scenes/2/transition', 'not_supported_yet').length, 1, 'slide-right');
    assert.equal(codes('/scenes/3/transition', 'not_supported_yet').length, 0, 'wipe is supported');
    assert.equal(codes('/scenes/4/transition', 'not_supported_yet').length, 0, 'slide-left is supported');
    assert.equal(codes('/scenes/5/transition', 'unknown_name').length, 1);
    assert.deepEqual(plain(out.schedule.scenes.map(s => s.transition)), ['cut', 'fade', 'fade', 'wipe', 'slide-left', 'fade']);
    assert.deepEqual(plain(out.report.unverified), []);
    const u = compiled.report.unverified;
    assert.equal(u.length, 1); assert.equal(u[0].code, 'layout_unverified'); assert.equal(u[0].scene_id, 's3'); assert.equal(u[0].severity, 'minor');
});

check('compile tolerates missing optional fields and reports a story without scenes', () => {
    const bare = SM.compile({ scenes: [{ id: 'a' }] }, undefined, catalog);
    assert.equal(bare.report.fps, 30);
    assert.equal(bare.report.frame_count, 30);
    const empty = SM.compile({}, {}, catalog);
    assert.equal(empty.report.frame_count, 0);
    assert.ok(empty.report.issues.some(i => i.severity === 'error' && i.path === '/scenes'));
    const noCast = SM.compile({ scenes: [{ id: 'a', actors: [{ cast: 'ghost' }] }] }, {}, catalog);
    assert.ok(noCast.report.issues.some(i => i.code === 'unknown_cast' && i.severity === 'warning'));
    const badEnv = SM.compile({ scenes: [{ id: 'a', environment: 'env.mars' }] }, {}, catalog);
    assert.ok(badEnv.report.issues.some(i => i.code === 'unknown_asset'));
    const codeNoBundle = SM.compile({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog, { codeScenes: {} });
    assert.ok(codeNoBundle.report.issues.some(i => i.code === 'code_error' && i.severity === 'error'));
    const codeFailed = SM.compile({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog, { codeScenes: { c: { error: { code: 'code_error', message: 'boom', line: 3, column: 5 } } } });
    assert.match(codeFailed.report.issues.find(i => i.code === 'code_error').message, /boom \(line 3, column 5\)/);
    const needs = plain(compiled.needs);
    assert.deepEqual(needs, ['catalog:environments/abstract-tech.json', 'catalog:environments/office.json', 'catalog:props/lock.json', 'catalog:props/shield.json', 'catalog:rig/human.json']);
});

check('frame plan: one layer, or two with the transition share p in (0, 1)', () => {
    const sched = compiled.schedule;
    assert.deepEqual(plain(SM.schedule.framePlan(sched, 0)), { cur: 0, layers: [{ i: 0, f: 0 }], transition: null });
    const mid = plain(SM.schedule.framePlan(sched, 114 + 7));
    assert.equal(mid.cur, 1);
    assert.deepEqual(mid.layers, [{ i: 0, f: 121 }, { i: 1, f: 7 }]);
    assert.equal(mid.transition.kind, 'fade');
    near(mid.transition.p, 8 / 16, 1e-9);
    const after = plain(SM.schedule.framePlan(sched, 114 + 15));
    assert.equal(after.layers.length, 1);
    assert.equal(plain(SM.schedule.framePlan(sched, compiled.report.frame_count - 1)).cur, 2);
});
