// Task D5: the 23 starter props.
// Run: node --test assets/storymaker/test/props.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { PROPS } from '../lib/props.mjs';

const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];
const IDS = ['laptop', 'phone', 'monitor', 'server', 'router', 'cloud', 'database', 'lock', 'key', 'shield', 'mail', 'document', 'coffee-cup', 'desk', 'chair', 'plant', 'check', 'cross', 'warning', 'idea', 'question', 'arrow-right', 'gear'];

test('all 23 props exist with sizes, tags and a hand hold for the cup', () => {
  assert.deepEqual(Object.keys(PROPS).sort(), [...IDS].sort());
  for (const [name, p] of Object.entries(PROPS)) {
    assert.ok(p.h > 0 && p.h < 0.4, name);
    assert.ok(p.tags.length >= 3, name);
  }
  assert.equal(PROPS['coffee-cup'].hold, true);
  assert.equal(PROPS.lock.h, 0.14);
  assert.equal(PROPS.shield.h, 0.16);
});

test('each prop is valid path data in its viewbox and uses only palette tokens and hex colours', () => {
  for (const [name, p] of Object.entries(PROPS)) {
    const { viewbox, shapes } = p.build();
    assert.equal(viewbox.length, 4, name);
    assert.ok(viewbox[2] > 0 && viewbox[3] > 0, name);
    assert.ok(shapes.length >= 3, `${name} has detail`);
    for (const [i, s] of shapes.entries()) {
      assert.ok(parsePath(s.d).ok, `${name}[${i}]`);
      for (const c of [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter(Boolean)) {
        assert.ok(c[0] === '$' ? PALETTE_TOKENS.includes(c.slice(1)) : /^#[0-9a-f]{6}$/i.test(c), `${name}[${i}] ${c}`);
      }
    }
  }
});

test('props are deterministic', () => {
  for (const p of Object.values(PROPS)) assert.equal(JSON.stringify(p.build()), JSON.stringify(p.build()));
});
