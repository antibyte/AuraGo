// Batch B: Lucide node-to-path conversion and the committed icon data.
// Run: node --test assets/storymaker/test/icons.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { nodesToPaths, fmt } from '../lib/icons.mjs';

test('every node kind converts to path data on the 24 grid', () => {
  const one = (node) => nodesToPaths([node]).paths[0];
  assert.equal(fmt(1.23456), '1.235');
  assert.equal(one(['circle', { cx: '12', cy: '12', r: '10' }]), 'M2 12A10 10 0 1 0 22 12A10 10 0 1 0 2 12Z');
  assert.equal(one(['ellipse', { cx: '12', cy: '8', rx: '5', ry: '3' }]), 'M7 8A5 3 0 1 0 17 8A5 3 0 1 0 7 8Z');
  assert.equal(one(['rect', { x: '2', y: '4', width: '20', height: '16', rx: '2' }]), 'M4 4H20A2 2 0 0 1 22 6V18A2 2 0 0 1 20 20H4A2 2 0 0 1 2 18V6A2 2 0 0 1 4 4Z');
  assert.equal(one(['rect', { x: '3', y: '3', width: '18', height: '18' }]), 'M3 3H21V21H3Z');
  assert.equal(one(['line', { x1: '3', y1: '4', x2: '5', y2: '6.5' }]), 'M3 4L5 6.5');
  assert.equal(one(['polyline', { points: '3 4 5 6 7 8' }]), 'M3 4L5 6L7 8');
  assert.equal(one(['polygon', { points: '3 4 5 6 7 8' }]), 'M3 4L5 6L7 8Z');
  assert.equal(one(['path', { d: ' M1 2h3 ', key: 'k1' }]), 'M1 2h3');
});

test('a fill attribute other than none goes to fills, a transform throws', () => {
  const r = nodesToPaths([['path', { d: 'M1 1h2' }], ['circle', { cx: '5', cy: '5', r: '1', fill: 'currentColor' }], ['path', { d: 'M2 2h2', fill: 'none' }]]);
  assert.equal(r.paths.length, 3);
  assert.deepEqual(r.fills, ['M4 5A1 1 0 1 0 6 5A1 1 0 1 0 4 5Z']);
  assert.throws(() => nodesToPaths([['path', { d: 'M1 1', transform: 'rotate(5)' }]]), /transform/);
  assert.throws(() => nodesToPaths([['text', {}]]), /text/);
});
