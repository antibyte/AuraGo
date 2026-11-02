// Task R: the player against the real starter catalog (ui/img/storymaker, part D). The rules where the two parts had to be
// reconciled (seated root offset, text box units, point poses when mirrored, gesture hands, environment layer order, spots in
// the narrow aspects, text colours on dark environments, the camera's actor reach), and a spread of frames of firewall.json and
// of the cast sheet (internal/storymaker/testdata/player/castsheet.json) that must draw without an unresolved token.
import assert from 'node:assert/strict';
import { plain, realCatalog, realFiles, realAssets, renderAssets, playerStory, story, FakeCanvas } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

// The async checks of the earlier modules (the host and render driver) run against real timers and deadlines; let them finish before
// this module's long synchronous work (the real rig, a hundred rendered frames) blocks the event loop.
await Promise.all(t.pending);

const { check, near } = t;
const SM = t.core();
const catalog = realCatalog(), files = realFiles();
const entry = id => catalog.entries.find(e => e.id === id);
const firewall = story('firewall');
const codeScenes = { s3: { duration: 5 } };
const compileReal = (s, opts) => SM.compile(s, {}, catalog, { codeScenes, ...opts });
const compiled = compileReal(firewall);
const idle = { type: 'actor', pose: { a: 'stand', b: 'stand', u: 1 }, expression: 'neutral', look: null, walkTime: null, phase: 0, gesture: null, facing: 1 };
const rigStateOf = (over, time) => SM.render.rigState({ ...idle, ...over }, time || 0, SM.render.makeContext(compiled.timeline, realAssets(), 1920, 1080), files.rig);

// ---- seated root offset -------------------------------------------------------------------------------------------------

check('real catalog: a seated pose drops the root by pose.offset_y, blended with the pose blend', () => {
    assert.equal(entry('pose.typing').offset_y, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 1 } }).offsetY, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 0.5 } }).offsetY, 107.5);
    assert.equal(rigStateOf({ pose: { a: 'sit', b: 'typing', u: 0.5 } }).offsetY, 215, 'two seated poses do not add up');
    assert.equal(rigStateOf({ pose: { a: 'typing', b: 'stand', u: 0.25 } }).offsetY, 161.25);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'stand', u: 1 } }).offsetY, 0);
});

check('rig: the drop of a pose is offset_y, else seat_drop, seated_offset, the lower leg, a fraction of the height (seated poses only)', () => {
    const drop = (rig, pose) => SM.rig.poseDrop(rig, pose);
    assert.equal(drop({ seat_drop: 215 }, { seated: true, offset_y: 180 }), 180, 'offset_y wins');
    assert.equal(drop({ seat_drop: 215 }, { offset_y: 40 }), 40, 'offset_y also works on a pose that is not seated');
    assert.equal(drop({ seat_drop: 215, seated_offset: 99 }, { seated: true }), 215);
    assert.equal(drop({ seated_offset: 99 }, { seated: true }), 99);
    assert.equal(drop({ units_height: 1000, bones: [{ name: 'lower_leg_l', length: 205 }] }, { seated: true }), 205);
    assert.equal(drop({ units_height: 1000, bones: [] }, { seated: true }), 240);
    assert.equal(drop({ seat_drop: 215 }, { bones: {} }), 0, 'a standing pose never drops');
    assert.equal(drop({ seat_drop: 215 }, null), 0);
    // the mini catalog's typing pose has seated without offset_y and the mini rig has seated_offset 92
    assert.equal(drop(t.files.rig, t.byId('pose.typing')), 92);
});

check('real catalog: the typing woman at the desk spot sits on the chair seat of the office (D pins the seat at 774 px)', () => {
    const tl = compileReal({ scenes: [{ id: 'a', environment: 'env.office', actors: [{ cast: 'anna', at: 'desk', pose: 'typing' }] }], cast: [{ id: 'anna', character: 'char.office-woman-1' }] }).timeline;
    const sc = tl.scenes[0], st = SM.scene.evalEntity(sc.entities[0], 0), stage = SM.render.stageFor(sc, SM.render.makeContext(tl, realAssets(), 1920, 1080));
    const s = SM.rig.scaleFor(files.rig, 1080, stage.depthScale(st.y, st.depth), 1, SM.rig.bodyOf(files.rig, 'average'));
    const hips = st.y + s * (-440 + 215);
    near(hips, 774, 15, 'hip joint height in design pixels');
});

// ---- text box units -----------------------------------------------------------------------------------------------------

check('text: box pad and radius are fractions of the canvas height, like the size (real caption and label styles)', () => {
    const caption = entry('text.caption'), label = entry('text.label'), title = entry('text.title');
    const lay = (style, h) => SM.text.layout(new FakeCanvas(10, 10).ctx, 'Hallo Welt', style, { canvasH: h, maxWidth: 1500, family: 'Geist' });
    assert.equal(lay(caption, 1080).pad, 13, 'round(0.012 x 1080)');
    assert.equal(lay(caption, 1080).radius, 13);
    assert.equal(lay(caption, 540).pad, 6);
    assert.equal(lay(label, 1080).pad, 11, 'round(0.010 x 1080)');
    assert.equal(lay(label, 1080).radius, 22, 'round(0.020 x 1080)');
    assert.equal(lay(title, 1080).pad, 0, 'no box, no padding');
    // a box without numbers falls back to em: 0.4 x the font size, radius 0.3 x the font size
    const bare = lay({ ...caption, box: { fill: '#000000' } }, 1000);
    assert.equal(bare.pad, Math.round(0.4 * bare.px));
    assert.equal(bare.radius, Math.round(0.3 * bare.px));
    // and the corners that are drawn use the radius
    const cv = new FakeCanvas(1920, 1080), l = lay(caption, 1080);
    SM.text.draw(cv.ctx, l, caption, 960, 900, SM.parts.makeResolver([{}]), 1);
    assert.equal(cv.ctx.log.filter(x => x.startsWith('arcTo(') && x.endsWith(',13)')).length, 4, 'four rounded corners of radius 13');
});

// ---- point poses and mirroring ------------------------------------------------------------------------------------------

// The screen side that the hand of a pose reaches: x of the hand tip relative to the root, after the mirror of a left-facing actor.
function handSide(poseName, facing) {
    const rs = rigStateOf({ pose: { a: poseName, b: poseName, u: 1 }, facing }), body = SM.rig.bodyOf(files.rig, 'average');
    const world = SM.rig.boneMatrices(files.rig, rs.view, body, rs.offsets);
    const hand = rs.hands.l === 'point' ? world.lower_arm_l : world.lower_arm_r, tip = hand[2] * 135 + hand[4];
    return Math.sign(tip) * facing;
}

check('rig state: point-left and point-right keep their screen direction when the actor faces left', () => {
    assert.equal(handSide('point-right', 1), 1);
    assert.equal(handSide('point-left', 1), -1);
    assert.equal(handSide('point-right', -1), 1, 'facing left: point-right still reaches the right edge of the screen');
    assert.equal(handSide('point-left', -1), -1);
    // the swap picks the other pose entry: its bones and its view hint
    const left = rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: -1 });
    assert.equal(left.offsets.upper_arm_l, 88, 'the bones of point-left');
    assert.equal(left.hands.l, 'point');
    assert.equal(left.view, 'front');
    assert.equal(rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: 1 }).view, 'three_quarter');
    // other poses are not mirrored by name
    assert.equal(rigStateOf({ pose: { a: 'wave', b: 'wave', u: 1 }, facing: -1 }).offsets.upper_arm_r, -128);
    // a catalog that has no point-left keeps the name (the mini catalog has neither)
    const mini = SM.compile({ cast: [{ id: 'a', character: 'char.office-woman-1' }], scenes: [{ id: 's', actors: [{ cast: 'a', at: 'left', pose: 'stand' }] }] }, {}, t.catalog);
    const rcMini = SM.render.makeContext(mini.timeline, renderAssets(), 1920, 1080);
    assert.equal(SM.render.rigState({ ...idle, pose: { a: 'stand', b: 'stand', u: 1 }, facing: -1 }, 0, rcMini, t.files.rig).view, 'three_quarter');
});

// ---- gesture hands ------------------------------------------------------------------------------------------------------

check('rig: a gesture hand shape holds until a later key sets another one (D: count points only at its first key)', () => {
    const count = entry('gesture.count'), celebrate = entry('gesture.celebrate');
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.1)), null);
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.5)), { r: 'point' }, 'the finger keeps pointing while the arm counts');
    assert.equal(SM.rig.handsAt(count.keys, 1), null, 'the end of the gesture returns to the pose hands');
    assert.deepEqual(plain(SM.rig.handsAt(celebrate.keys, 0.5)), { l: 'open', r: 'open' });
    assert.deepEqual(plain(SM.rig.handsAt([{ t: 0, bones: {}, hands: { l: 'fist' } }, { t: 0.5, bones: {}, hands: { r: 'point' } }], 0.7)), { l: 'fist', r: 'point' }, 'hands merge per side');
    assert.equal(rigStateOf({ gesture: { name: 'count', frac: 0.5 } }).hands.r, 'point');
    assert.equal(rigStateOf({ pose: { a: 'think', b: 'think', u: 1 } }).hands.r, 'fist', 'a pose hand shape without a gesture');
});

