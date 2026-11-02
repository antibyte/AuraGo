// Task D1: the path reader, the shape helpers and the section 3a renderer.
// Run: node --test assets/storymaker/test/geometry.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { n, poly, smooth, ellipse, roundRect, capsule, limb, ribbon, rng } from '../lib/util.mjs';
import { boneWorlds, evalKeys, shapeSvg, Ctx } from '../lib/svg.mjs';
import { BODIES, BODY_NAMES, hipHeight } from '../lib/dims.mjs';

test('the path reader accepts the commands the generators emit and rejects malformed data', () => {
  for (const d of ['M0 0L1 1Z', 'M1.5-2C1 2 3 4 5 6Q1 2 3 4A5 5 0 1 0 10 10Z', 'M0 0h5v5H0z', 'M0 0 10 10 20 0']) assert.ok(parsePath(d).ok, d);
  for (const d of ['', 'L1 1', 'M0', 'M0 0L1', 'M0 0A5 5 0 2 0 1 1', 'M0 0Zx', 'M0 0 L 1 e']) assert.equal(parsePath(d).ok, false, d);
});

test('shape helpers emit valid path data and round to one decimal', () => {
  assert.equal(n(1.26), 1.3);
  assert.equal(Object.is(n(-0.01), -0), false);
  const shapes = [
    poly([[0, 0], [10, 0], [5, 8]]), smooth([[0, 0], [10, 2], [20, 0], [10, -6]]), ellipse(5, 5, 4, 2), roundRect(0, 0, 40, 20, 6),
    capsule(0, 0, 30, 10, 8), limb(0, 100, 30, 20, 2), ribbon([[0, 0], [10, 4], [20, 0]], 4), ribbon([[0, 0], [30, 0]], (t) => 6 - 4 * t),
  ];
  for (const d of shapes) assert.ok(parsePath(d).ok, d);
  const r = rng(5);
  assert.notEqual(r(), r());
  assert.equal(rng(5)(), rng(5)());
});

test('bone transforms follow section 3a and negative angles swing towards +x', () => {
  const rig = {
    bones: [
      { name: 'root', parent: null, rest: { front: { x: 0, y: 0, angle: 0 } } },
      { name: 'arm', parent: 'root', rest: { front: { x: 10, y: -100, angle: 0 } } },
      { name: 'hand', parent: 'arm', rest: { front: { x: 0, y: 50, angle: 0 } } },
    ],
    bodies: { tall: { bones: { arm: { rest: { front: { y: -120 } } } } } },
  };
  const at = (w, x, y) => [w[0] * x + w[2] * y + w[4], w[1] * x + w[3] * y + w[5]];
  const stand = boneWorlds(rig, 'none', 'front', {}, {});
  assert.deepEqual(at(stand.get('hand'), 0, 0).map(Math.round), [10, -50]);
  const raised = boneWorlds(rig, 'none', 'front', { arm: -90 }, {});
  assert.deepEqual(at(raised.get('hand'), 0, 0).map(Math.round), [60, -100]); // hangs 50 below, -90 swings it to +x
  const gesture = boneWorlds(rig, 'none', 'front', { arm: -90 }, { arm: 90 });
  assert.deepEqual(at(gesture.get('hand'), 0, 0).map(Math.round), [10, -50]); // pose offsets and deltas add up
  assert.deepEqual(at(boneWorlds(rig, 'tall', 'front').get('arm'), 0, 0).map(Math.round), [10, -120]); // body override
});

test('key frames blend linearly, hold the latest hands and return to the first key', () => {
  const keys = [{ t: 0, bones: {} }, { t: 0.5, bones: { a: 10 }, hands: { r: 'point' }, offset_y: 4 }, { t: 1, bones: {} }];
  assert.equal(evalKeys(keys, 0.25).bones.a, 5);
  assert.equal(evalKeys(keys, 0.5).bones.a, 10);
  assert.equal(evalKeys(keys, 0.75).hands.r, 'point');
  assert.equal(evalKeys(keys, 1).bones.a, 0);
  assert.equal(evalKeys(keys, 0.25).offset_y, 2);
});

test('shapes resolve $tokens through the given maps and report unresolved ones', () => {
  const ctx = new Ctx();
  const svg = shapeSvg({ d: 'M0 0L1 1Z', fill: '$skin', opacity: 0.5 }, [{ skin: '#c4875f' }], ctx);
  assert.match(svg, /fill="#c4875f"/);
  assert.match(svg, /opacity="0.5"/);
  shapeSvg({ d: 'M0 0L1 1Z', fill: '$nope' }, [{}], ctx);
  assert.deepEqual([...ctx.unresolved], ['$nope']);
});

test('body proportions share the hand and foot attachment lengths', () => {
  for (const name of BODY_NAMES) {
    const b = BODIES[name];
    assert.equal(b.torsoLen, 255, name);
    assert.ok(hipHeight(b) >= 375 && hipHeight(b) <= 440, name);
  }
});
