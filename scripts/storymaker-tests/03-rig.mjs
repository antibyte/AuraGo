// Task C4: the 2D rig: bones, world matrices, slot order, pose, gesture and cycle keys.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, files } = t;
const SM = t.core();

check('rig: bone angles, world matrices, mirroring base and slot order', () => {
    const rig = t.files.rig, R = SM.rig, typing = t.byId('pose.typing');
    assert.equal(rig.bones.length, 13);
    assert.deepEqual(plain(rig.bones.map(b => b.name)), ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r']);
    const angles = R.boneAngles(rig, 'front', { scale: 1 }, typing.bones);
    assert.equal(angles.upper_leg_l, -80); assert.equal(angles.lower_leg_l, 80); assert.equal(angles.torso, 0);
    const world = R.boneMatrices(rig, 'front', { scale: 1 }, {});
    const head = world.head;
    near(head[4], 0, 1e-9); near(head[5], -190 - 8 - 108 - 10, 1e-9, 'standing head joint');
    const bent = R.boneMatrices(rig, 'front', { scale: 1 }, { torso: 90 });
    near(bent.neck[4], 108, 1e-9, 'a clockwise torso turn swings the neck to the right'); near(bent.neck[5], -198, 1e-9);
    near(R.boneMatrices(rig, 'front', { scale: 1 }, { upper_leg_l: -80 }).lower_leg_l[4], -16 + Math.sin(80 * Math.PI / 180) * 94, 1e-6, 'a thigh turned forward moves the knee forward');
    // body overrides move a joint
    const wide = R.boneMatrices(rig, 'front', { scale: 1, bones: { upper_arm_l: { rest: { front: { x: -50 } } } } }, {});
    near(wide.upper_arm_l[4], -50, 1e-9);
    // draw order differs per view: the far arm is in front of the torso only from the front
    const order = v => R.index(rig).zOrder[v].map(s => s.name);
    assert.ok(order('front').indexOf('sleeve_upper_r') > order('front').indexOf('torso'));
    assert.ok(order('three_quarter').indexOf('sleeve_upper_r') < order('three_quarter').indexOf('torso'));
    assert.equal(R.scaleFor(rig, 1080, 1, 1, { scale: 1 }), 0.36 * 1080 / 400);
    near(R.scaleFor(rig, 1080, 0.973, 1.5, { scale: 0.72 }), 0.36 * 1080 * 0.973 * 1.5 * 0.72 / 400, 1e-9);
    assert.deepEqual(plain(R.mul(R.translate(5, 7), R.rotate(0))), [1, 0, 0, 1, 5, 7]);
});

check('rig: pose, gesture and cycle evaluation return to the base pose', () => {
    const point = t.byId('gesture.point'), walk = t.byId('cycle.walk'), R = SM.rig;
    assert.deepEqual(plain(R.sampleKeys(point.keys, 0).bones), {});
    near(R.sampleKeys(point.keys, 0.15).bones.upper_arm_r, -42.5, 1e-9, 'smoothed halfway between the first two keys');
    assert.equal(R.sampleKeys(point.keys, 0.5).bones.upper_arm_r, -85);
    assert.deepEqual(plain(R.sampleKeys(point.keys, 1).bones), {}, 'a gesture returns to the base pose at t = 1');
    assert.deepEqual(plain(R.handsAt(point.keys, 0.5)), { r: 'point' });
    assert.equal(R.handsAt(point.keys, 1), null);
    assert.equal(R.handsAt(point.keys, 0.1), null);
    const half = R.sampleKeys(walk.keys, 0.25);
    near(half.bones.upper_leg_l, 0, 1e-9); near(half.offset_y, -3, 1e-9);
    near(R.cyclePhase(walk, 2.25), 0.25, 1e-9);
    near(R.cyclePhase(walk, -0.25), 0.75, 1e-9);
});
