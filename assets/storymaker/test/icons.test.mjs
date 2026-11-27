// Batch B: Lucide node-to-path conversion and the committed icon data.
// Run: node --test assets/storymaker/test/icons.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { nodesToPaths, buildIconData, fmt, PATH_CHARS, CATEGORIES, LUCIDE_INTEGRITY, LUCIDE_VERSION } from '../lib/icons.mjs';

const iconsDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'icons');

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

test('the committed icon data is complete', () => {
  const allow = JSON.parse(fs.readFileSync(path.join(iconsDir, 'allow-list.json'), 'utf8'));
  assert.equal(allow.schema, 'storymaker-icon-allow-list/1');
  assert.deepEqual(Object.keys(allow.categories), CATEGORIES);
  const raw = fs.readFileSync(path.join(iconsDir, 'lucide.json'), 'utf8');
  assert.ok(raw.length <= 256 * 1024, 'at most 256 KB');
  assert.ok(!raw.includes('\r'), 'LF only');
  const data = JSON.parse(raw);
  assert.equal(data.schema, 'storymaker-icons/1');
  assert.equal(data.source, 'lucide-static@1.82.0');
  assert.equal(data.grid, 24);
  assert.equal(data.stroke, 2);
  const ids = data.icons.map((i) => i.id);
  assert.deepEqual(ids, [...ids].sort(), 'sorted by id');
  assert.equal(new Set(ids).size, ids.length, 'no duplicate id');
  assert.ok(ids.length >= 490 && ids.length <= 530, `count ${ids.length}`);
  const byId = new Map(data.icons.map((i) => [i.id, i]));
  const wanted = new Set();
  for (const [cat, list] of Object.entries(allow.categories)) {
    for (const name of list) {
      wanted.add(`icon.${name}`);
      const icon = byId.get(`icon.${name}`);
      assert.ok(icon, `icon.${name} is missing`);
      assert.ok(icon.tags.includes(cat), `icon.${name} lacks the tag ${cat}`);
    }
  }
  assert.deepEqual(ids, [...wanted].sort());
  for (const icon of data.icons) {
    assert.ok(icon.paths.length >= 1 && icon.paths.length <= 40, icon.id);
    assert.ok(icon.tags.length <= 32, icon.id);
    for (const p of [...icon.paths, ...(icon.fills ?? [])]) assert.ok(p.length <= 2000 && PATH_CHARS.test(p), `${icon.id}: ${p}`);
  }
  const notice = fs.readFileSync(path.join(iconsDir, 'NOTICE.txt'), 'utf8');
  assert.ok(notice.includes('ISC') && notice.includes('MIT') && notice.includes(LUCIDE_INTEGRITY));
  assert.ok(notice.includes(`${ids.length} icons`));
  assert.ok(fs.readFileSync(path.join(iconsDir, 'LICENSE.txt'), 'utf8').includes('ISC License'));
});

test('a rect corner radius defaults to the raw other radius before clamping', () => {
  const d = nodesToPaths([['rect', { x: '0', y: '0', width: '20', height: '40', rx: '30' }]]).paths[0];
  assert.equal(d, 'M10 0H10A10 20 0 0 1 20 20V20A10 20 0 0 1 10 40H10A10 20 0 0 1 0 20V20A10 20 0 0 1 10 0Z');
  const e = nodesToPaths([['rect', { x: '0', y: '0', width: '20', height: '40', ry: '30' }]]).paths[0];
  assert.ok(e.startsWith('M10 0H10A10 20 0 0 1'), e);
});

test('the icon build refuses a package that is not lucide-static at the pinned version', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sm-icons-'));
  try {
    const allow = { schema: 'storymaker-icon-allow-list/1', categories: {} };
    for (const [name, version] of [['lucide-static', '1.81.0'], ['lucide-other', LUCIDE_VERSION]]) {
      fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify({ name, version }));
      const built = buildIconData(dir, allow);
      assert.ok(built.problems.some((p) => p.includes('package.json')), `${name}@${version}: ${built.problems}`);
    }
    fs.writeFileSync(path.join(dir, 'package.json'), '{}');
    assert.ok(buildIconData(dir, allow).problems.length > 0, 'a package without a name');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
