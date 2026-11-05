// Task C6: renderFrame: environment layers, props, actors, texts, transitions, determinism of the call log.
import assert from 'node:assert/strict';
import { plain, renderAssets, FakeCanvas, makeContext, digest, loadPlayer, coreOf } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall } = t;
const compiled = t.compiled();
const SM = t.core();

function renderLog(tl, n, assets, width) {
    const cv = new FakeCanvas(width || tl.width, width ? Math.round(width * tl.height / tl.width) : tl.height);
    SM.renderFrame(tl, n, cv.ctx, assets || renderAssets(), width ? { width } : undefined);
    return cv.ctx.log.map(l => l.replace(/canvas\d+/g, 'canvas'));
}

check('render: environment layers, props, actors by depth, texts; no unresolved token', () => {
    SM.parts.resetUnresolved();
    const log = renderLog(compiled.timeline, 40);
    const text = log.join('\n');
    assert.match(text, /createLinearGradient/);
    assert.ok(log.filter(l => l.startsWith('setTransform')).length > 20, 'many parts are drawn');
    assert.ok(text.includes('M-36,-52 C-38,-86'), 'the hair slot of the actor is drawn');
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
    // the first scene at 1 s: the lock prop is shown from the beat; the actor sits at the desk
    const lockDraws = renderLog(compiled.timeline, 100).filter(l => l.includes('M28,52'));
    assert.ok(lockDraws.length > 0, 'the lock is drawn');
    assert.equal(renderLog(compiled.timeline, 5).filter(l => l.includes('M28,52')).length, 0, 'the lock has not appeared yet at frame 5');
    // a text show beat
    const tl = SM.compile({ scenes: [{ id: 'a', beats: [{ do: 'show', text: 'Hallo Welt', at: 'center', entry: 'none' }] }] }, {}, catalog).timeline;
    const t = renderLog(tl, 10).filter(l => l.startsWith('fillText'));
    assert.deepEqual(t.map(l => l.replace(/,[\d.]+,[\d.]+\)$/, ')')), ['fillText(Hallo Welt)']);
    // an off-aspect canvas and a thumbnail width draw too
    assert.ok(renderLog(SM.compile({ ...firewall, format: { aspect: '9:16', fps: 30 } }, {}, catalog, { codeScenes: { s3: { duration: 5 } } }).timeline, 40).length > 100);
    assert.ok(renderLog(compiled.timeline, 40, undefined, 320).length > 100);
});

check('render: transitions composite two scene canvases (fade, slide-left, wipe), cut shows the new scene', () => {
    const mk = kind => SM.compile({ scenes: [{ id: 'a', silent: true }, { id: 'b', silent: true, transition: kind }] }, {}, catalog).timeline;
    const at = (tl, n) => { const cv = new FakeCanvas(tl.width, tl.height); SM.renderFrame(tl, n, cv.ctx, renderAssets()); return cv.ctx.log; };
    const fade = mk('fade'), half = fade.schedule.scenes[1].start_frame + 7;
    const fl = at(fade, half);
    assert.equal(fl.filter(l => l.startsWith('drawImage')).length, 2);
    assert.ok(fl.includes('set globalAlpha=0.5'), 'fade at p = 8/16');
    const slide = at(mk('slide-left'), mk('slide-left').schedule.scenes[1].start_frame + 7).filter(l => l.startsWith('drawImage'));
    assert.deepEqual(slide.map(l => l.replace(/canvas\d+/, 'canvas')), ['drawImage(canvas,-960,0)', 'drawImage(canvas,960,0)']);
    const wipe = at(mk('wipe'), mk('wipe').schedule.scenes[1].start_frame + 7);
    assert.ok(wipe.includes('rect(0,0,960,1080)') && wipe.includes('clip()'));
    const cutTl = SM.compile({ scenes: [{ id: 'a', silent: true }, { id: 'b', silent: true, transition: 'cut' }] }, {}, catalog).timeline;
    assert.equal(cutTl.schedule.scenes[1].tf, 0);
    assert.equal(at(cutTl, 30).filter(l => l.startsWith('drawImage')).length, 0);
});

check('determinism: the same frame renders the same call log, in any order, from a fresh compile', () => {
    const a = renderLog(compiled.timeline, 140), b = renderLog(compiled.timeline, 140);
    assert.equal(a.length, b.length);
    assert.deepEqual(a, b);
    const other = SM.compile(firewall, {}, catalog, { codeScenes: { s3: { duration: 5 } } });
    assert.deepEqual(renderLog(other.timeline, 140), a, 'a second compile of the same inputs');
    renderLog(compiled.timeline, 3); renderLog(compiled.timeline, 300);
    assert.deepEqual(renderLog(compiled.timeline, 140), a, 'rendering other frames in between changes nothing');
    assert.notDeepEqual(renderLog(compiled.timeline, 141), a);
    for (const n of [0, 50, 125, 130, 200, 230, 353]) assert.equal(digest(renderLog(compiled.timeline, n)), digest(renderLog(other.timeline, n)), 'frame ' + n);
    const placeholder = renderLog(compiled.timeline, 300);
    assert.ok(placeholder.some(l => l.includes('Code scene')), 'a code scene without a frame shows a placeholder');
});

check('highlight: pulse peaks at 1.08 on a prop and a text, outline strokes 6 design pixels in $accent, arrow draws above the entity', () => {
    const r3 = v => String(Math.round(v * 1000) / 1000);
    const onProp = style => SM.compile({ scenes: [{ id: 'a', transition: 'cut', props: [{ id: 'lock', item: 'prop.lock', at: 'center' }], beats: [{ do: 'highlight', target: 'lock', style }] }] }, {}, catalog).timeline;
    const onText = style => SM.compile({ scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'show', id: 't1', text: 'Hallo', entry: 'none' }, { do: 'highlight', target: 't1', style }] }] }, {}, catalog).timeline;
    // The beat starts at scene time 0 and lasts the default 0.8 s, so u = t / 0.8. sin(2 PI 2 u) peaks at u = 1/8, which is
    // frame 3 at 30 fps, and bottoms at u = 3/8, which is frame 9.
    let peak = 0;
    for (let i = 0; i <= 800; i++) peak = Math.max(peak, SM.render.pulseOf({ highlight: { style: 'pulse', u: i / 800 } }));
    near(peak, 1.08, 1e-9);
    // The lock: height 0.14 of 1080 at depth 0.8 + 0.25 x 18/26 (spot y 88 % on the plain stage floor 70..96), viewbox 100 x 120,
    // bottom centre on (960, 950.4).
    const depth = 0.8 + 0.25 * (18 / 26), boxH = 0.14 * 1080 * depth, boxW = boxH * 100 / 120, top = 0.88 * 1080 - boxH;
    const scaled = 'setTransform(' + r3(boxH / 120 * 1.08) + ',0,0,' + r3(boxH / 120 * 1.08) + ',';
    assert.ok(renderLog(onProp('pulse'), 3).some(l => l.startsWith(scaled)), 'the prop is drawn at 1.08 times its size at the peak');
    assert.ok(!renderLog(onProp('pulse'), 0).some(l => l.startsWith(scaled)), 'and at its own size at u = 0');
    // A text: the transform scales around the anchor (960, 302.4), which is 28 % of 1080 on the centre line.
    const textLog = renderLog(onText('pulse'), 3);
    const tr = textLog.indexOf('setTransform(1.08,0,0,1.08,' + r3(960 * -0.08) + ',' + r3(302.4 * -0.08) + ')');
    assert.ok(tr >= 0, 'drawText applies the pulse as a canvas transform');
    assert.ok(textLog.findIndex(l => l.startsWith('fillText(Hallo')) > tr, 'the text is drawn under it');
    // Outline: the strokes run in design pixels under the stage matrix, in the mini palette's accent.
    const outline = renderLog(onProp('outline'), 3);
    const at = outline.indexOf('strokeRect(' + [960 - boxW / 2, top, boxW, boxH].map(r3).join(',') + ')');
    assert.ok(at > 0, 'the stroke follows the entity box');
    assert.deepEqual(outline.slice(at - 2, at), ['set strokeStyle=#ff7a45', 'set lineWidth=6']);
    // A text box lives in canvas pixels, so at half size 6 design pixels are 3 canvas pixels.
    const half = renderLog(onText('outline'), 3, undefined, 960);
    assert.equal(half[half.findIndex(l => l.startsWith('strokeRect(')) - 1], 'set lineWidth=3');
    // Arrow: the tip sits 40 px above the box, plus 8 px at the sine's peak and minus 8 px at its low.
    const high = renderLog(onProp('arrow'), 3), tip = high.indexOf('moveTo(960,' + r3(top - 48) + ')');
    assert.ok(tip > high.findLastIndex(l => l.includes('M28,52')), 'the arrow is above the lock and drawn after it');
    assert.equal(high[tip - 2], 'set fillStyle=#ff7a45');
    assert.ok(renderLog(onProp('arrow'), 9).includes('moveTo(960,' + r3(top - 32) + ')'), 'the arrow bobs 8 px');
});

check('layout memo: a sliding text renders the same at its frames in both orders', () => {
    // The slide moves the anchor and the anchor sets maxWidth, so the wrap changes during the entry. Two fresh realms start with
    // empty memos, so a key without maxWidth would freeze the first wrap each realm sees, and the two orders would differ.
    const slide = { scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'show', text: 'Ein kurzer Titel hier', at: 'left', entry: 'slide' }] }] };
    const run = frames => {
        const S = coreOf(loadPlayer()), tl = S.compile(slide, {}, catalog).timeline, out = {};
        for (const n of frames) {
            const cv = new FakeCanvas(tl.width, tl.height);
            S.renderFrame(tl, n, cv.ctx, renderAssets());
            out[n] = cv.ctx.log.map(l => l.replace(/canvas\d+/g, 'canvas'));
        }
        return out;
    };
    const frames = Array.from({ length: 16 }, (_, n) => n);
    const forward = run(frames), backward = run([...frames].reverse());
    for (const n of frames) assert.deepEqual(backward[n], forward[n], 'frame ' + n);
    // The title is 76 px, so with the fake 0.5 em font the text is 21 x 38 = 798 px wide. The anchor slides from 436 to 576. At frame 1
    // (u = 1/15) it is below 470, so maxWidth = 2 x (x - 96) is below 750: two lines. At frame 15 it is 576, so maxWidth is 960: one line.
    const lines = n => forward[n].filter(l => l.startsWith('fillText(')).length;
    assert.equal(lines(1), 2);
    assert.equal(lines(15), 1);
});

check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
    const point = t.byId('gesture.point');
    // rigState: a gesture in progress, the typing pose seated, the walk cycle in profile
    const rc = SM.render.makeContext(compiled.timeline, renderAssets(), 1920, 1080);
    const st = { type: 'actor', pose: { a: 'stand', b: 'stand', u: 1 }, expression: 'happy', look: null, walkTime: null, phase: 0, gesture: { name: 'point', frac: 0.5 } };
    const rs = SM.render.rigState(st, 1.0, rc, files.rig);
    assert.equal(rs.offsets.upper_arm_r, -85); assert.deepEqual(plain(rs.hands), { r: 'point' }); assert.equal(rs.view, 'three_quarter');
    assert.equal(rs.expr.mouth, 'smile');
    const seated = SM.render.rigState({ ...st, gesture: null, pose: { a: 'stand', b: 'typing', u: 1 } }, 0, rc, files.rig);
    assert.equal(seated.offsetY, 92); assert.equal(seated.offsets.upper_leg_l, -80);
    const halfway = SM.render.rigState({ ...st, gesture: null, pose: { a: 'stand', b: 'typing', u: 0.5 } }, 0, rc, files.rig);
    assert.equal(halfway.offsets.upper_leg_l, -40); assert.equal(halfway.offsetY, 46);
    const walking = SM.render.rigState({ ...st, gesture: null, walkTime: 0.25, look: 'camera' }, 5, rc, files.rig);
    assert.equal(walking.view, 'profile', 'walking wins over looking at the camera');
    near(walking.offsetY, -3, 1e-9);
    const front = SM.render.rigState({ ...st, gesture: null, look: 'camera' }, 5, rc, files.rig);
    assert.equal(front.view, 'front');
});
