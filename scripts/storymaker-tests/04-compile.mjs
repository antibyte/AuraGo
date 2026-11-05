// Task C5: compile: scene-local timing, cues, auto durations, transition overlap, beats, the support matrix.
import assert from 'node:assert/strict';
import { plain, readJSON } from '../storymaker-test-support.mjs';
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
    assert.deepEqual(r.audio_events, []); assert.deepEqual(r.checks, []);
    // Estimated narration still gets captions. s1 has 65 columns on 2 lines, s2 has 32 and s3 has 27: one caption per block.
    assert.deepEqual(r.captions.map(c => c.block_id), ['s1:n', 's2:n', 's3:n']);
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
    const measured = SM.compile(minimal, { narration: { s1: { blocks: [{ id: 's1:n', kind: 'narration', speaker: 'narrator', duration_ms: 2500, spans: [], levels: [], speech_ok: true, pieces: [{ hash: 'abc', ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 2500, chars: 11 }] }] } } }, catalog);
    assert.equal(measured.report.scenes[0].end_frame, Math.round(2.9 * 30));
    assert.deepEqual(plain(measured.report.audio_events), [{ kind: 'narration', ref: 'abc', start_ms: 0, scene_id: 's1', block_id: 's1:n', piece: 0 }]);
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
    const measured = SM.compile({ scenes: [{ id: 'a', narration: 'x y' }, { id: 'b', narration: 'x y' }] }, { narration: { a: { blocks: [{ id: 'a:n', kind: 'narration', speaker: 'narrator', duration_ms: 3000, spans: [], levels: [], speech_ok: true, pieces: [{ hash: 'h1', ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 3000, chars: 3 }] }] }, b: { blocks: [{ id: 'b:n', kind: 'narration', speaker: 'narrator', duration_ms: 1850, spans: [], levels: [], speech_ok: true, pieces: [{ hash: 'h2', ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 1000, chars: 2 }, { hash: 'h3', ext: 'wav', at_ms: 1350, in_ms: 0, out_ms: 500, chars: 1 }] }] } } }, catalog);
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
    for (const j of [2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
    assert.equal(codes('/scenes/0/beats/1/text', 'narration_missing').length, 1, 'a say without audio is estimated');
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
// Task S3: cue fields, cue_precision, the film end, and the report fixture that Go parses.
{
    const HASH_A = 'a'.repeat(64), HASH_B = 'b'.repeat(64), HASH_C = 'c'.repeat(64);
    const audioPiece = (hash, at, used, chars) => ({ hash, ext: 'wav', at_ms: at, in_ms: 0, out_ms: used, chars });
    const narrationBlock = (id, durationMs, pieces) => ({ id, kind: 'narration', speaker: 'narrator', duration_ms: durationMs, spans: [], levels: [], speech_ok: true, pieces });

    check('compile report: scenes[].cues and cue_precision', () => {
        assert.equal(SM.compile(minimal, {}, catalog).report.cue_precision, null, 'no cue in the film');
        const r = plain(compiled.report);
        // firewall s1 has one marker on an estimated block, the other scenes have none.
        const cueSeconds = plain(SM.planTiming(firewall, {}, catalog)).scenes.s1.cues['1'];
        assert.deepEqual(r.scenes[0].cues, [{ marker: 1, at_ms: Math.round(cueSeconds * 1000), placed: 'proportion' }]);
        assert.deepEqual(r.scenes[1].cues, []);
        assert.equal(r.cue_precision, 0);
        // One span cue and one estimated cue: the share is 1/2. 'eins zwei' has 9 units.
        const mixed = SM.compile({ scenes: [{ id: 'a', narration: 'eins {1}zwei' }, { id: 'b', narration: 'drei {1}vier' }] },
            { narration: { a: { blocks: [narrationBlock('a:n', 1000, [audioPiece(HASH_A, 0, 1000, 9)])] } } }, catalog).report;
        assert.equal(mixed.cue_precision, 0.5);
    });

    // The story behind internal/storymaker/render/testdata/report_measured.json, mini catalog, 30 fps.
    // s1: first scene, T = 0. Pieces of 25 + 1 and 29 UTF-16 units, used 1900 ms and 2350 ms, with the 350 ms sentence pause, so
    // piece 1 starts at 2250 ms and the block lasts 4600 ms. Scene length 4.6 + 0.4 tail = 5 s = 150 frames.
    // {1} stands at offset 26, the start of piece 1: at_ms 2250, placed span. One span cue of one cue: cue_precision 1.
    // s2: fade 0.5 s = 15 frames, so it starts at frame 150 - 15 = 135 and T = 0.5 s. Length 0.5 + 2 + 0.4 tail = 2.9 s = 87 frames,
    // range 135..222. The film has 222 frames = 7400 ms. The s2 block starts at 135/30 + 0.5 = 5 s = 5000 ms.
    const measuredStory = { scenes: [
        { id: 's1', narration: 'Das Paket kommt heute an. {1}Die Firewall prueft es genau.' },
        { id: 's2', transition: 'fade', narration: 'Das ist das Ende vom Film.' }
    ] };
    const measuredAudio = { narration: {
        s1: { blocks: [narrationBlock('s1:n', 4600, [audioPiece(HASH_A, 0, 1900, 26), audioPiece(HASH_B, 2250, 2350, 29)])] },
        s2: { blocks: [narrationBlock('s2:n', 2000, [audioPiece(HASH_C, 0, 2000, 26)])] }
    } };

    check('compile report: the measured report equals the fixture that Go parses', () => {
        const out = SM.compile(measuredStory, measuredAudio, catalog), report = plain(out.report);
        const fixture = readJSON('internal/storymaker/render/testdata/report_measured.json');
        const wire = r => { const { captions, issues, ...rest } = r; return rest; };
        assert.deepEqual(wire(report), wire(fixture));
        assert.ok(report.issues.every(i => i.severity !== 'error'), 'no error issue');
        assert.deepEqual(plain(SM.overlappingNarration(out.timeline)), [], 'no two narration events overlap');
    });

    check('compile: the film end drops late audio events with one warning on the scene', () => {
        // 10 s of speech: used 2650 ms + 350 ms pause, used 3550 ms + 350 ms pause, used 3100 ms. 'Eins. ' and 'Zwei. ' have 6 units, 'Drei.' has 5.
        const late = id => ({ narration: { [id]: { blocks: [narrationBlock(id + ':n', 10000, [audioPiece(HASH_A, 0, 2650, 6), audioPiece(HASH_B, 3000, 3550, 6), audioPiece(HASH_C, 6900, 3100, 5)])] } } });
        const warned = r => r.issues.filter(i => i.code === 'invalid_value' && i.path === '/scenes/0/duration');
        // Duration 3 s = 90 frames = 3000 ms: piece 1 starts exactly at the film end, piece 2 past it.
        const explicit = plain(SM.compile({ scenes: [{ id: 'a', duration: 3, narration: 'Eins. Zwei. Drei.' }] }, late('a'), catalog).report);
        assert.equal(explicit.duration_ms, 3000);
        assert.deepEqual(explicit.audio_events.map(e => e.ref), [HASH_A]);
        assert.equal(warned(explicit).length, 1);
        // A code scene with api.duration(1) = 30 frames = 1000 ms: only piece 0 starts inside the film.
        const code = plain(SM.compile({ scenes: [{ id: 'c', kind: 'code', narration: 'Eins. Zwei. Drei.' }] }, late('c'), catalog, { codeScenes: { c: { duration: 1, explicit: true } } }).report);
        assert.equal(code.duration_ms, 1000);
        assert.deepEqual(code.audio_events.map(e => e.ref), [HASH_A]);
        assert.equal(warned(code).length, 1);
    });

    // A code scene that reports 3.5 s of motion, 8 s of speech, then a narrated scene with a 0.5 s fade. 'Eins. Zwei. Drei.' has 17 units.
    const codeFirst = { scenes: [
        { id: 'c', kind: 'code', narration: 'Eins. Zwei. Drei.' },
        { id: 'n', transition: 'fade', narration: 'Das ist das Ende vom Film.' }
    ] };
    const codeFirstAudio = { narration: {
        c: { blocks: [narrationBlock('c:n', 8000, [audioPiece(HASH_A, 0, 8000, 17)])] },
        n: { blocks: [narrationBlock('n:n', 2000, [audioPiece(HASH_C, 0, 2000, 26)])] }
    } };
    const durationWarnings = r => r.issues.filter(i => i.code === 'invalid_value' && i.path === '/scenes/0/duration');

    check('compile: a code scene without api.duration covers its narration, so the next narration starts after it', () => {
        const out = SM.compile(codeFirst, codeFirstAudio, catalog, { codeScenes: { c: { duration: 3.5 } } }), report = plain(out.report);
        // T = 0, 8 s of speech + 0.4 s tail = 8.4 s = 252 frames. The fade of 15 frames starts the next scene at 237.
        assert.equal(report.scenes[0].end_frame, 252);
        assert.equal(report.scenes[1].start_frame, 237);
        assert.deepEqual(plain(SM.overlappingNarration(out.timeline)), []);
        assert.equal(durationWarnings(report).length, 0);
        // A motion longer than the speech keeps its own length.
        const longer = plain(SM.compile(codeFirst, codeFirstAudio, catalog, { codeScenes: { c: { duration: 9 } } }).report);
        assert.equal(longer.scenes[0].end_frame, 270);
        // A silent code scene after a fade keeps the length of its motion: 1 s = 30 frames, not T + 1 s = 45.
        const silent = plain(SM.compile({ scenes: [{ id: 'n', narration: 'Das ist das Ende vom Film.' }, { id: 'c', kind: 'code', transition: 'fade' }] }, {}, catalog, { codeScenes: { c: { duration: 1 } } }).report);
        assert.equal(silent.scenes[1].end_frame - silent.scenes[1].start_frame, 30);
    });

    check('compile: a code scene with api.duration keeps that length and warns once when the narration is longer', () => {
        const short = plain(SM.compile(codeFirst, codeFirstAudio, catalog, { codeScenes: { c: { duration: 3.5, explicit: true } } }).report);
        assert.equal(short.scenes[0].end_frame, 105);
        assert.equal(durationWarnings(short).length, 1);
        assert.equal(durationWarnings(short)[0].severity, 'warning');
        const fits = plain(SM.compile(codeFirst, codeFirstAudio, catalog, { codeScenes: { c: { duration: 9, explicit: true } } }).report);
        assert.equal(fits.scenes[0].end_frame, 270);
        assert.equal(durationWarnings(fits).length, 0);
    });
}
