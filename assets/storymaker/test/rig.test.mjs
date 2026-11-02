// Task D3: bodies, clothes and the assembled rig/human.json.
// Run: node --test assets/storymaker/test/rig.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { buildRig } from '../lib/rig.mjs';
import { VIEWS } from '../lib/dims.mjs';
import { Ctx, characterSvg } from '../lib/svg.mjs';
import { MOUTH_NAMES } from '../lib/face.mjs';

const rig = buildRig();
const PALETTE = { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' };
const STARTERS = [
  { body: 'average', skin: 'medium', hair: 'bun', hair_color: 'dark-brown', eyes: 'round', brows: 'soft', facial_hair: 'none', glasses: 'none', outfit: 'business', accessory: 'none' },
  { body: 'broad', skin: 'light', hair: 'side-part', hair_color: 'chestnut', eyes: 'almond', brows: 'bold', facial_hair: 'stubble', glasses: 'none', outfit: 'suit', accessory: 'none' },
  { body: 'slim', skin: 'brown', hair: 'messy', hair_color: 'black', eyes: 'big', brows: 'thin', facial_hair: 'none', glasses: 'square', outfit: 'it', accessory: 'headset' },
  { body: 'elder', skin: 'tan', hair: 'balding', hair_color: 'white', eyes: 'almond', brows: 'soft', facial_hair: 'mustache', glasses: 'round', outfit: 'cardigan', accessory: 'none' },
];

test('the rig has the 13 bones, per-view rest transforms and per-view slot order', () => {
  assert.equal(rig.id, 'rig.human/1');
  assert.deepEqual(rig.bones.map((b) => b.name), ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r']);
  for (const b of rig.bones) for (const v of VIEWS) assert.ok(['x', 'y', 'angle'].every((k) => Number.isFinite(b.rest[v][k])), `${b.name} ${v}`);
  const slot = (name) => rig.slots.find((s) => s.name === name);
  for (const end of ['hand_l', 'hand_r', 'foot_l', 'foot_r']) assert.ok(slot(end), end);
  // far limbs sit behind the torso in the angled views, near limbs in front; in the front view arms sit above it
  assert.ok(slot('arm_u_l').z.three_quarter < slot('torso').z.three_quarter);
  assert.ok(slot('arm_u_r').z.three_quarter > slot('torso').z.three_quarter);
  assert.ok(slot('arm_u_l').z.front > slot('torso').z.front);
  assert.equal(rig.seat_drop, 215);
});

test('the _r limbs are on the screen right in the front view and the near side in the angled views', () => {
  const x = (bone, view) => rig.bones.find((b) => b.name === bone).rest[view].x;
  assert.ok(x('upper_arm_r', 'front') > 0 && x('upper_arm_l', 'front') < 0);
  assert.ok(x('upper_arm_r', 'three_quarter') < 0 && x('upper_arm_l', 'three_quarter') > 0);
});

test('every starter character renders in every view with every expression, hand and mouth', () => {
  const exprs = [['neutral', 'neutral', 'rest'], ['smile', 'neutral', 'smile'], ['laugh', 'raised', 'laugh'], ['sad', 'sad', 'frown'], ['angry', 'angry', 'grit'], ['wide', 'raised', 'o'], ['neutral', 'confused', 'squiggle'], ['lookup', 'think', 'side'], ['worried', 'worried', 'wobble'], ['narrow', 'determined', 'firm']];
  for (const params of STARTERS) {
    for (const view of VIEWS) {
      for (const [eyes, brows, mouth] of exprs) {
        for (const hand of ['open', 'fist', 'point', 'hold']) {
          const ctx = new Ctx();
          characterSvg(rig, params, PALETTE, ctx, { view, pose: { bones: {}, hands: { l: hand, r: hand } }, expression: { eyes, brows, mouth } });
          assert.deepEqual([...ctx.missing], [], `${params.hair} ${view} ${eyes}`);
          assert.deepEqual([...ctx.unresolved], []);
        }
      }
      for (const m of MOUTH_NAMES) {
        const ctx = new Ctx();
        characterSvg(rig, params, PALETTE, ctx, { view, mouth: m });
        assert.equal(ctx.missing.size, 0);
      }
    }
  }
});

test('all variants are valid path data and every parameter value has shapes', () => {
  for (const [slot, keys] of Object.entries(rig.variants)) {
    for (const [key, views] of Object.entries(keys)) {
      for (const v of VIEWS) {
        assert.ok(Array.isArray(views[v]), `${slot}/${key}/${v}`);
        for (const s of views[v]) assert.ok(parsePath(s.d).ok, `${slot}/${key}/${v}`);
      }
    }
  }
  for (const body of rig.params.body) {
    assert.ok(rig.bodies[body], body);
    assert.ok(rig.variants.torso[body] && rig.variants.pelvis[body], body);
  }
  for (const style of rig.params.hair) assert.ok(rig.variants.hair[style] && rig.variants.hair_back[style], style);
  for (const o of Object.values(rig.params.outfit)) {
    assert.ok(rig.variants.top_det[o.top] && rig.variants.sleeve_u_r[o.top] && rig.variants.pelvis_det[o.bottom] && rig.variants.foot_r[o.shoes]);
  }
  assert.ok(rig.params.skin.medium && rig.params.hair_color['dark-brown']);
  assert.deepEqual(Object.keys(rig.params.fixed).sort(), ['eye_white', 'iris', 'lip', 'outline']);
});

test('body types differ from the average rig only by the rest offsets they override', () => {
  assert.equal(rig.bodies.average.bones, undefined);
  assert.ok(rig.bodies.broad.bones.upper_arm_l.rest.front.x < rig.bones.find((b) => b.name === 'upper_arm_l').rest.front.x);
  assert.equal(rig.bodies.child.scale < 1, true);
});
