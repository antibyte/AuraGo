// Task D4: poses, gestures and walk/idle cycles.
// Run: node --test assets/storymaker/test/motion.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { POSES, GESTURES, CYCLES, SEAT_DROP } from '../lib/motion.mjs';
import { evalKeys } from '../lib/svg.mjs';

const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const HANDS = ['open', 'fist', 'point', 'hold'];

test('the ten poses and nine gestures exist and only use known bones and hands', () => {
  assert.deepEqual(Object.keys(POSES), ['stand', 'sit', 'typing', 'point-left', 'point-right', 'wave', 'think', 'present', 'arms-crossed', 'hands-on-hips']);
  assert.deepEqual(Object.keys(GESTURES), ['point', 'wave', 'shrug', 'think', 'present', 'thumbs-up', 'celebrate', 'facepalm', 'count']);
  const all = [...Object.values(POSES), ...Object.values(GESTURES).flatMap((g) => g.keys), ...Object.values(CYCLES).flatMap((c) => c.keys)];
  for (const item of all) {
    for (const [b, deg] of Object.entries(item.bones)) assert.ok(BONES.includes(b) && Number.isFinite(deg), b);
    for (const h of Object.values(item.hands || {})) assert.ok(HANDS.includes(h), h);
  }
});

test('seated poses drop the root so that the feet stay on the floor line', () => {
  assert.equal(SEAT_DROP, 215);
  for (const name of ['sit', 'typing']) {
    assert.equal(POSES[name].seated, true);
    assert.equal(POSES[name].offset_y, SEAT_DROP);
    assert.equal(POSES[name].bones.upper_leg_r, -90); // thighs forward
    assert.equal(POSES[name].bones.lower_leg_r, 90); // shins vertical
  }
  assert.equal(POSES.stand.offset_y, undefined);
});

test('every gesture starts and ends on the base pose and has a positive duration', () => {
  for (const [name, g] of Object.entries(GESTURES)) {
    assert.equal(g.keys[0].t, 0, name);
    assert.equal(g.keys.at(-1).t, 1, name);
    assert.deepEqual(g.keys[0].bones, g.keys.at(-1).bones, name);
    assert.ok(g.duration > 0, name);
    g.keys.forEach((k, i) => i && assert.ok(k.t >= g.keys[i - 1].t, name));
  }
  assert.deepEqual(GESTURES.point.keys[1].bones, { upper_arm_r: -85, lower_arm_r: -10 });
});

test('the walk cycle is periodic, mirrored over half a period and drops the root while the legs are spread', () => {
  const walk = CYCLES.walk;
  assert.equal(walk.period, 1);
  const a = evalKeys(walk.keys, 0);
  const b = evalKeys(walk.keys, 0.5);
  const c = evalKeys(walk.keys, 1);
  assert.deepEqual(a.bones, c.bones);
  assert.equal(a.bones.upper_leg_r, -b.bones.upper_leg_r);
  assert.equal(a.bones.upper_leg_l, -b.bones.upper_leg_l);
  assert.equal(a.bones.upper_arm_r, -b.bones.upper_arm_r);
  assert.ok(a.offset_y > evalKeys(walk.keys, 0.25).offset_y, 'lowest at contact, highest while passing');
  // stride: 420 units of leg at 18 degrees; speed in percent of the 1920 px width at 0.55 * 1080 / 1000 px per unit
  const step = 2 * 420 * Math.sin((18 * Math.PI) / 180);
  assert.equal(Math.round(((2 * step * 0.55 * 1.08) / 1920) * 100), walk.speed);
  assert.equal(CYCLES.idle.speed, undefined);
  assert.equal(CYCLES.idle.period, 3);
});
