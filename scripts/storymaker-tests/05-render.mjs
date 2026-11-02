// Task C6: renderFrame: environment layers, props, actors, texts, transitions, determinism of the call log.
import assert from 'node:assert/strict';
import { plain, renderAssets, FakeCanvas, makeContext, digest } from '../storymaker-test-support.mjs';
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
