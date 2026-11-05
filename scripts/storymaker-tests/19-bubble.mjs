// Tasks S6-S8: speech and thought bubbles: bubble.js, the bubble entity, say with text and say without text.
import assert from 'node:assert/strict';
import { plain, renderAssets, FakeCanvas } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const SAFE = { x: 5, y: 5, w: 90, h: 90 };
const resolver = () => SM.parts.makeResolver([{ text: '#1f2a44', bg2: '#ffe0a3' }]);
const layoutOf = (cv, lines, name) => SM.bubble.layout(cv.ctx, lines, t.byId('text.' + name), { canvasW: 1920, canvasH: 1080, safe: SAFE, family: 'Storymaker Default' });

check('bubble: speech draws a rounded box with a tail that ends on the anchor', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'speech');
    cv.ctx.log.length = 0;
    SM.bubble.draw(cv.ctx, lay, { x: 960, y: 500 }, 'speech', resolver(), 1);
    // the box sits above the anchor, so nothing is clamped and the tail tip is the anchor itself
    assert.ok(cv.ctx.log.includes('lineTo(960,500)'), 'the tail tip');
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arcTo(')).length, 4, 'four rounded corners');
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arc(')).length, 0, 'no circles');
    assert.ok(cv.ctx.log.some(l => l.startsWith('fillText(Hallo Welt,960,')), 'the text is centred in the box');
});

check('bubble: thought draws a rounded box and three circles, and no tail', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'thought');
    cv.ctx.log.length = 0;
    SM.bubble.draw(cv.ctx, lay, { x: 960, y: 500 }, 'thought', resolver(), 1);
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arc(')).length, 3, 'three circles');
    assert.ok(!cv.ctx.log.includes('lineTo(960,500)'), 'no tail');
});

check('bubble: the box stays in the safe area for an actor at the edge', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'speech');
    // the safe area of 1920x1080: x from 5 % to 95 % is 96 to 1824, y from 5 % to 95 % is 54 to 1026
    for (const anchor of [{ x: 1900, y: 40 }, { x: 10, y: 40 }, { x: 1900, y: 1070 }, { x: 10, y: 1070 }]) {
        const b = SM.bubble.place(lay, anchor);
        assert.ok(b.x >= 96 && b.x + b.w <= 1824 && b.y >= 54 && b.y + b.h <= 1026, JSON.stringify(plain(b)));
        assert.ok(b.tip.y <= 1026, 'the tail stays inside too');
        assert.deepEqual(plain(SM.bubble.draw(cv.ctx, lay, anchor, 'speech', resolver(), 1)), plain(b), 'draw uses the clamped box');
    }
    const tl = SM.compile({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 'a', duration: 2, actors: [{ cast: 'anna', at: { x: 100, y: 88 } }] }] }, {}, catalog).timeline;
    tl.scenes[0].entities.push(SM.scene.createBubble({ id: 'b', owner: 'anna', kind: 'speech', lines: ['Hallo Welt'], t0: 0, t1: 2 }));
    const out = new FakeCanvas(1920, 1080);
    SM.renderFrame(tl, 30, out.ctx, renderAssets());
    // 10 characters at 0.5 em of round(0.042 x 1080) = 45 px are 225 px, pad round(0.014 x 1080) = 15, so the box is 255 px.
    // The actor stands at x 1824, so the box touches the right safe edge and its centre is 1824 - 255 / 2 = 1696.5.
    assert.ok(out.ctx.log.some(l => l.startsWith('fillText(Hallo Welt,1696.5,')), 'renderFrame draws the clamped bubble');
});

check('bubble: fades in and out over 0.12 s, linear', () => {
    const b = SM.scene.createBubble({ id: 'b', owner: 'anna', kind: 'speech', lines: ['x'], t0: 0, t1: 2 });
    assert.equal(SM.bubble.FADE_SECONDS, 0.12);
    assert.equal(SM.scene.evalEntity(b, 0).alpha, 0);
    near(SM.scene.evalEntity(b, 0.06).alpha, 0.5, 1e-9, 'half of a 0.12 s fade');
    assert.equal(SM.scene.evalEntity(b, 1).alpha, 1);
    near(SM.scene.evalEntity(b, 1.94).alpha, 0.5, 1e-9, 'half of the fade out');
    assert.equal(SM.scene.evalEntity(b, 2).alpha, 0);
    const s = SM.scene.evalEntity(b, 1);
    assert.deepEqual([s.type, s.owner, s.kind, s.style], ['bubble', 'anna', 'speech', 'speech']);
    assert.deepEqual(plain(s.lines), ['x']);
});

// ---- S7: say with text

const CAST = [{ id: 'anna', character: 'char.office-woman-1' }, { id: 'ben', character: 'char.office-woman-1' }];
const HASH_A = 'a'.repeat(64), HASH_B = 'b'.repeat(64);
// 'Hallo Welt' is 10 UTF-16 units: 'Hallo ' is 6 and 'Welt' is 4. The block plays 900 ms, a 350 ms pause and 750 ms: 2000 ms.
const measuredBlock = id => ({
    id, kind: 'dialogue', beat: 0, speaker: 'anna', duration_ms: 2000, spans: [[0, 900], [1250, 2000]], levels: [0, 120, 180], speech_ok: true,
    pieces: [{ hash: HASH_A, ext: 'wav', at_ms: 0, in_ms: 10, out_ms: 910, chars: 6 }, { hash: HASH_B, ext: 'wav', at_ms: 1250, in_ms: 0, out_ms: 750, chars: 4 }]
});
const say = extra => ({ do: 'say', who: 'anna', text: 'Hallo Welt', style: 'speech', ...extra });
const oneScene = (scene, more) => ({ cast: CAST, scenes: [{ id: 'a', transition: 'cut', actors: [{ cast: 'anna', at: 'left' }], ...scene }], ...more });
const windows = out => plain(out.timeline.narrationEvents).filter(e => e.kind === 'dialogue');
const bubbles = out => out.timeline.scenes[0].entities.filter(e => e.type === 'bubble');
const annaOf = out => out.timeline.scenes[0].entities.find(e => e.id === 'anna');

check('say: starts at the later of its cue and the end of the block before it; blocks never overlap', () => {
    const N = 4 / 2.3;   // 4 words of estimated narration at 2.3 words per second, from scene time 0
    const early = SM.compile(oneScene({ narration: '{1}eins zwei drei vier', beats: [say({ cue: 1 })] }), {}, catalog);
    near(windows(early)[0].start, N, 2e-3, 'a cue at 0 waits for the narration end');
    const late = SM.compile(oneScene({ narration: 'eins zwei drei vier{1}', beats: [say({ cue: 1, delay: 1 })] }), {}, catalog);
    near(windows(late)[0].start, N + 1, 2e-3, 'the cue at the text end plus a delay of 1 s is later than the narration end');
    // two estimated says: 2 words give the 1 s minimum, and with:"prev" cannot pull the second one onto the first
    const two = SM.compile(oneScene({ beats: [say(), say({ with: 'prev' })] }), {}, catalog), w = windows(two);
    near(w[0].start, 0, 1e-9);
    near(w[1].start, w[0].end, 1e-9, 'the second block starts when the first one ends');
    assert.deepEqual(plain(SM.overlappingNarration(two.timeline)), []);
    // the cursor follows a block that had to wait: after the 1 s say from N and a 0.5 s wait, the prop starts at N + 1.5
    const after = SM.compile(oneScene({ narration: 'eins zwei drei vier', beats: [say(), { do: 'wait', seconds: 0.5 }, { do: 'show', item: 'prop.lock', at: 'right', entry: 'fade' }] }), {}, catalog);
    near(after.timeline.scenes[0].entities.find(e => e.item === 'prop.lock').alpha.segs[0].t0, N + 1.5, 1e-6, 'the end of the say moved with its start');
});

check('say: an automatic scene covers the last block plus the 0.4 s tail', () => {
    // estimated: 1 s block + 0.4 s = 1.4 s = 42 frames at 30 fps
    assert.equal(SM.compile(oneScene({ beats: [say()] }), {}, catalog).report.scenes[0].end_frame, 42);
    // measured: 2 s block + 0.4 s = 2.4 s = 72 frames at 30 fps
    assert.equal(SM.compile(oneScene({ beats: [say()] }), { narration: { a: { blocks: [measuredBlock('a:b0')] } } }, catalog).report.scenes[0].end_frame, 72);
});

check('say: one dialogue event per piece with a film-global time, a bubble, a speech interval and a caption', () => {
    const story = { cast: CAST, scenes: [{ id: 'a', silent: true, duration: 2 }, { id: 'b', transition: 'cut', actors: [{ cast: 'anna', at: 'left' }], beats: [say()] }] };
    const out = SM.compile(story, { narration: { b: { blocks: [measuredBlock('b:b0')] } } }, catalog);
    // scene b starts at frame 60 = 2 s and its block at scene time 0, so each piece is at 2000 ms plus its at_ms
    assert.deepEqual(plain(out.report.audio_events), [
        { kind: 'dialogue', ref: HASH_A, start_ms: 2000, scene_id: 'b', block_id: 'b:b0', piece: 0 },
        { kind: 'dialogue', ref: HASH_B, start_ms: 3250, scene_id: 'b', block_id: 'b:b0', piece: 1 }
    ]);
    assert.ok(!out.report.issues.some(i => i.code === 'narration_missing'));
    const sc = out.timeline.scenes[1], anna = sc.entities.find(e => e.id === 'anna'), bubble = sc.entities.find(e => e.type === 'bubble');
    assert.deepEqual([bubble.owner, bubble.kind], ['anna', 'speech']);
    assert.deepEqual(plain(bubble.lines), ['Hallo Welt']);
    assert.equal(anna.speech.list.length, 1);
    near(anna.speech.list[0].t0, 0, 1e-9);
    near(anna.speech.list[0].t1, 2, 1e-9);
    assert.equal(anna.speech.list[0].data.block.id, 'b:b0');
    // the caption spans the block: cueMap(0) is 0 and cueMap(10) is the end of the last piece, 1250 + 750 = 2000
    const cap = plain(out.report.captions).find(c => c.block_id === 'b:b0');
    assert.deepEqual([cap.start_ms, cap.end_ms, cap.scene_id, cap.lines], [2000, 4000, 'b', ['Hallo Welt']]);
    const cv = new FakeCanvas(1920, 1080);
    SM.renderFrame(out.timeline, 75, cv.ctx, renderAssets());
    assert.ok(cv.ctx.log.some(l => l.startsWith('fillText(Hallo Welt,')), 'frame 75 is 0.5 s into scene b');
});

check('say: an estimated say warns narration_missing at its text and has no events and no speech interval', () => {
    const out = SM.compile(oneScene({ beats: [{ do: 'wait', seconds: 0.5 }, say()] }), {}, catalog);
    assert.deepEqual(plain(out.report.issues).filter(i => i.code === 'narration_missing').map(i => [i.severity, i.path]), [['warning', '/scenes/0/beats/1/text']]);
    assert.deepEqual(plain(out.report.audio_events), []);
    near(windows(out)[0].start, 0.5, 1e-9, 'it follows the wait');
    assert.equal(bubbles(out).length, 1);
    assert.equal(annaOf(out).speech.list.length, 0);
});

check('say: an off-stage actor keeps the events and gets no bubble, no speech interval and no bubble font', () => {
    const out = SM.compile(oneScene({ beats: [say({ who: 'ben' })] }, { style: { captions: false } }), { narration: { a: { blocks: [measuredBlock('a:b0')] } } }, catalog);
    assert.equal(plain(out.report.audio_events).length, 2);
    assert.equal(bubbles(out).length, 0);
    assert.equal(annaOf(out).speech.list.length, 0);
    assert.ok(out.report.issues.some(i => i.code === 'unknown_cast' && i.path === '/scenes/0/beats/0/who'));
    assert.ok(!plain(out.needs).includes('font:default/400'));
});

check('say: a block past an explicit scene duration warns once at the duration', () => {
    // measuredBlock plays 2 s, with piece 1 at 1250 ms
    const audio = { narration: { a: { blocks: [measuredBlock('a:b0')] } } };
    const warned = out => plain(out.report.issues).filter(i => i.code === 'invalid_value' && i.path === '/scenes/0/duration');
    // the only scene: both pieces start inside the 1.5 s film, and the film end cuts piece 1
    const cut = SM.compile(oneScene({ duration: 1.5, beats: [say()] }), audio, catalog);
    assert.equal(warned(cut).length, 1, 'a block cut at the film end');
    assert.equal(plain(cut.report.audio_events).length, 2, 'both pieces start inside the film');
    // a next scene starts its narration at 1.5 s, while the block plays until 2 s
    const story = oneScene({ duration: 1.5, beats: [say()] });
    story.scenes.push({ id: 'b', transition: 'cut', narration: 'eins zwei drei vier' });
    const overlap = SM.compile(story, audio, catalog);
    assert.equal(warned(overlap).length, 1, 'a block that reaches into the next scene');
    assert.equal(plain(SM.overlappingNarration(overlap.timeline)).length, 1, 'the two voices overlap');
    // piece 1 starts at 1250 ms, past the 1 s film: it is dropped, and the drop adds no second warning
    const dropped = SM.compile(oneScene({ duration: 1, beats: [say()] }), audio, catalog);
    assert.deepEqual(plain(dropped.report.audio_events).map(e => e.ref), [HASH_A]);
    assert.equal(warned(dropped).length, 1, 'still one warning');
    // narration and a say that both run long share one warning
    assert.equal(warned(SM.compile(oneScene({ duration: 1, narration: 'eins zwei drei vier', beats: [say()] }), audio, catalog)).length, 1, 'one warning per scene');
    // a block that fits
    assert.equal(warned(SM.compile(oneScene({ duration: 3, beats: [say()] }), audio, catalog)).length, 0, 'a block inside the scene');
});

check('say: a bubble adds the font key of its style', () => {
    const out = SM.compile(oneScene({ beats: [say({ style: 'thought' })] }, { style: { captions: false } }), {}, catalog);
    assert.equal(bubbles(out)[0].kind, 'thought');
    // the mini catalog has one face of the default font, so weight 500 resolves to it
    assert.ok(plain(out.needs).includes('font:default/400'));
});
