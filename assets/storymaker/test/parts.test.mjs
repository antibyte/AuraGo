// Task D2: head surface model, face features, hair and head extras.
// Run: node --test assets/storymaker/test/parts.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { VIEWS } from '../lib/dims.mjs';
import { edges, surf, wrap, facing, HEAD_TOP } from '../lib/headgeo.mjs';
import { headShapes, eyeShapes, browShapes, mouthShapes, EYE_STYLES, EYE_STATES, BROW_STYLES, BROW_STATES, MOUTH_NAMES } from '../lib/face.mjs';
import { HAIR_STYLES, hairShapes } from '../lib/hair.mjs';
import { GLASSES, FACIAL_HAIR, ACCESSORIES, glassesShapes, facialHairShapes, accessoryShapes } from '../lib/extras.mjs';

const TOKENS = new Set(['skin', 'hair', 'outline', 'eye_white', 'iris', 'lip', 'accent', 'accent2']);

function check(where, shapes) {
  assert.ok(Array.isArray(shapes), where);
  for (const [i, s] of shapes.entries()) {
    const p = parsePath(s.d);
    assert.ok(p.ok, `${where}[${i}]: ${p.error} (${s.d.slice(0, 60)})`);
    assert.ok(s.fill || s.stroke, `${where}[${i}] draws nothing`);
    for (const c of [s.fill, s.stroke]) if (typeof c === 'string' && c[0] === '$') assert.ok(TOKENS.has(c.slice(1)), `${where}: ${c}`);
  }
}

test('the head surface keeps the three views consistent', () => {
  // the silhouette is symmetric in the front view and moves towards +x when the head turns right
  const [l0, r0] = edges('front', -120);
  assert.ok(Math.abs(l0 + r0) < 1e-6);
  const [l1, r1] = edges('profile', -120);
  assert.ok(r1 > r0 * 0.8 && l1 > -110 && l1 < -90);
  assert.equal(surf('front', 0, -120)[0], 0);
  assert.ok(surf('profile', 0, -120)[0] > 80, 'the face centre sits at the front edge in profile');
  // the far eye is hidden in profile, both eyes are visible in the front view
  assert.ok(facing('front', -41, -112) && facing('front', 41, -112));
  assert.ok(facing('profile', -41, -112) && !facing('profile', 41, -112));
  assert.ok(wrap('three_quarter', 41, -112)[0] > wrap('three_quarter', -41, -112)[0]);
  assert.ok(HEAD_TOP < -200);
});

test('every face, hair and extra variant is valid path data in every view', () => {
  for (const v of VIEWS) {
    for (const body of ['average', 'elder']) check(`head ${body} ${v}`, headShapes(body, v));
    for (const style of Object.keys(EYE_STYLES)) for (const st of EYE_STATES) check(`eyes ${style}/${st} ${v}`, eyeShapes(style, st, v));
    for (const style of Object.keys(BROW_STYLES)) for (const st of BROW_STATES) check(`brows ${style}/${st} ${v}`, browShapes(style, st, v));
    for (const m of MOUTH_NAMES) check(`mouth ${m} ${v}`, mouthShapes(m, v));
    for (const h of HAIR_STYLES) {
      const { back, front } = hairShapes(h, v);
      check(`hair back ${h} ${v}`, back);
      check(`hair front ${h} ${v}`, front);
      if (h !== 'bald') assert.ok(front.length > 0, `${h} ${v} draws hair`);
    }
    for (const g of GLASSES.filter((x) => x !== 'none')) check(`glasses ${g} ${v}`, glassesShapes(g, v));
    for (const f of FACIAL_HAIR.filter((x) => x !== 'none')) check(`facial hair ${f} ${v}`, facialHairShapes(f, v));
    for (const a of ACCESSORIES.filter((x) => x !== 'none')) check(`accessory ${a} ${v}`, accessoryShapes(a, v));
  }
});

test('the profile shows one eye and one brow, the other views two', () => {
  const count = (shapes) => shapes.length;
  assert.ok(count(eyeShapes('round', 'neutral', 'front')) > count(eyeShapes('round', 'neutral', 'profile')));
  assert.equal(browShapes('soft', 'neutral', 'front').length, 2);
  assert.equal(browShapes('soft', 'neutral', 'profile').length, 1);
});
