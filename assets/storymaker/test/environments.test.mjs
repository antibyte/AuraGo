// Task D6: the four starter environments.
// Run: node --test assets/storymaker/test/environments.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { ENVIRONMENTS, ENV_META } from '../lib/environments.mjs';
import { buildRig } from '../lib/rig.mjs';
import { POSES } from '../lib/motion.mjs';
import { boneWorlds } from '../lib/svg.mjs';

const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];
const BASE = ['far-left', 'left', 'center', 'right', 'far-right'];
const EXTRA = { 'plain-stage': ['board'], office: ['desk', 'door', 'board', 'window'], 'home-office': ['desk', 'door', 'window'], 'abstract-tech': ['board'] };

test('four environments with the five base spots, their extra spots and a floor line', () => {
  assert.deepEqual(Object.keys(ENVIRONMENTS), ['plain-stage', 'office', 'home-office', 'abstract-tech']);
  for (const [name, meta] of Object.entries(ENV_META)) {
    for (const s of [...BASE, ...EXTRA[name]]) assert.ok(meta.spots[s], `${name} ${s}`);
    assert.ok(meta.floor.y_far < meta.floor.y_near && meta.floor.scale_far < meta.floor.scale_near, name);
  }
  assert.equal(ENV_META.office.spots.desk.face, 'right');
  assert.equal(ENV_META.office.spots.door.face, 'left');
});

test('layers are valid path data and only use palette tokens or tokens defined for day and night', () => {
  for (const [name, build] of Object.entries(ENVIRONMENTS)) {
    const env = build();
    assert.ok(env.layers.length >= 3, name);
    const day = Object.keys(env.tokens.day).sort();
    const night = Object.keys(env.tokens.night).filter((k) => !PALETTE_TOKENS.includes(k)).sort();
    assert.deepEqual(night, day.filter((k) => !PALETTE_TOKENS.includes(k)), `${name}: night defines the same local tokens`);
    const known = new Set([...PALETTE_TOKENS, ...day]);
    for (const layer of env.layers) {
      assert.ok(layer.parallax >= 0 && layer.parallax <= 1, `${name} ${layer.name}`);
      for (const [i, s] of layer.shapes.entries()) {
        assert.ok(parsePath(s.d).ok, `${name} ${layer.name}[${i}]`);
        for (const c of [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter(Boolean)) {
          if (c[0] === '$') assert.ok(known.has(c.slice(1)), `${name} ${layer.name}[${i}] ${c}`);
        }
      }
    }
  }
});

test('the 9:16 and 1:1 crops keep every spot in view', () => {
  // the output shows a slice of the design frame as wide as it is at the design height
  const half = { '9:16': (100 * ((1080 * 9) / 16)) / 1920 / 2, '1:1': (100 * 1080) / 1920 / 2 };
  for (const [name, meta] of Object.entries(ENV_META)) {
    for (const [asp, h] of Object.entries(half)) {
      const a = meta.aspects[asp];
      assert.ok(typeof a.focus_x === 'number', `${name} ${asp}`);
      for (const [spot, base] of Object.entries(meta.spots)) {
        const x = (a.spots?.[spot] || base).x;
        assert.ok(x >= a.focus_x - h + 3 && x <= a.focus_x + h - 3, `${name} ${asp} ${spot} x=${x}`);
      }
      for (const spot of Object.keys(a.spots || {})) assert.ok(meta.spots[spot], `${name} ${asp} overrides unknown spot ${spot}`);
    }
  }
});

test('a seated actor at the desk spot sits on the chair and types on the desk (both offices)', () => {
  const rig = buildRig();
  const m = ENV_META.office;
  const depth = m.floor.scale_far + ((m.spots.desk.y - m.floor.y_far) / (m.floor.y_near - m.floor.y_far)) * (m.floor.scale_near - m.floor.scale_far);
  const unit = (rig.height * 1080 * depth) / 1000; // screen px per rig unit
  const feetX = (m.spots.desk.x / 100) * 1920;
  const feetY = (m.spots.desk.y / 100) * 1080;
  const worlds = boneWorlds(rig, 'average', 'three_quarter', POSES.typing.bones, {});
  const at = (bone, x, y) => {
    const w = worlds.get(bone);
    const px = w[0] * x + w[2] * y + w[4];
    const py = w[1] * x + w[3] * y + w[5] + POSES.typing.offset_y;
    return [feetX + px * unit, feetY + py * unit];
  };
  const [, hipsY] = at('hips', 0, 0);
  const [handX, handY] = at('lower_arm_r', 0, 135 + 40);
  const desk = ENVIRONMENTS.office().layers.find((l) => l.name === 'furniture').shapes.find((s) => s.fill === '$wood' && s.d.startsWith('M788 698'));
  const home = ENVIRONMENTS['home-office']().layers.find((l) => l.name === 'furniture').shapes.find((s) => s.fill === '$deskwood' && s.d.startsWith('M788 698'));
  assert.ok(desk && home, 'both desks have their top plate at y = 698 px');
  assert.ok(Math.abs(handY - 698) < 45, `hands at ${Math.round(handY)} px, desk top at 698 px`);
  assert.ok(handX > 780 && handX < 1300, `hands at x = ${Math.round(handX)} px, above the desk`);
  assert.ok(Math.abs(hipsY - 774) < 30, `hips at ${Math.round(hipsY)} px, chair seat at 774 px`);
});
