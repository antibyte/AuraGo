// Task G1: the coverage file built from the font cmaps.
// Run: node --test assets/storymaker/test/glyphs.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { GLYPH_FACES, buildGlyphs, decodeRanges, encodeRanges, glyphsText, rangesCover } from '../lib/glyphs.mjs';
import { buildOutputs, versionOf } from '../lib/catalog.mjs';
import { validateOutputs } from '../lib/validate.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const rangesOf = (id) => decodeRanges(buildGlyphs().faces.find((f) => f.id === id).d);

test('the delta format: first value is the start, then the gap from the previous end and the length', () => {
  // 880-883 is one range: start 880, length 883-880=3. 885 is the next: gap 885-883=2, length 0.
  assert.deepEqual(encodeRanges([880, 881, 882, 883, 885]), [880, 3, 2, 0]);
  assert.deepEqual(decodeRanges([880, 3, 2, 0]), [[880, 883], [885, 885]]);
});

test('faces keep the fallback order, and only the CJK face has its own byte cap', () => {
  const { schema, faces } = buildGlyphs();
  assert.equal(schema, 'storymaker-glyphs/1');
  assert.deepEqual(faces.map((f) => f.id), ['sm-greek', 'sm-greek-ext', 'geist', 'sm-deva', 'sm-arab', 'sm-cjk']);
  // 17 MiB = 17 * 1048576 bytes
  assert.deepEqual(faces.filter((f) => f.max_bytes).map((f) => [f.id, f.max_bytes]), [['sm-cjk', 17825792]]);
});

test('sm-greek covers U+03C9 and stays inside its unicode_range', () => {
  const ranges = rangesOf('sm-greek');
  assert.ok(rangesCover(ranges, 0x03c9));
  for (const [a, b] of ranges) assert.ok(a >= 0x0370 && b <= 0x03ff, `${a}-${b}`);
});

test('geist keeps Latin and excludes U+0370-03FF', () => {
  const ranges = rangesOf('geist');
  assert.ok(rangesCover(ranges, 0x0041));
  for (const [a, b] of ranges) assert.ok(b < 0x0370 || a > 0x03ff, `${a}-${b}`);
});

test('U+2026 is covered, and the validator refuses a file without it', () => {
  assert.ok(buildGlyphs().faces.some((f) => rangesCover(decodeRanges(f.d), 0x2026)));
  const built = buildOutputs();
  const files = new Map(built.files);
  // d [32,94] is U+0020-007E: start 32, end 32+94=126.
  files.set('glyphs.json', glyphsText({ schema: 'storymaker-glyphs/1', faces: [{ id: 'geist', family: 'Geist', file: '/fonts/Geist-Regular.woff2', weight: '400', unicode_range: '', d: [32, 94] }] }));
  assert.ok(validateOutputs({ ...built, files }).some((p) => p.includes('U+2026')));
});

test('the manifest names glyphs.json and the file joins the version hash', () => {
  const { files, manifest } = buildOutputs();
  assert.deepEqual(manifest.glyphs, { file: 'glyphs.json' });
  assert.match(files.get('catalog.json'), /"rig":\{[^}]*\},"glyphs":\{"file":"glyphs\.json"\},/);
  const changed = new Map(files);
  changed.set('glyphs.json', files.get('glyphs.json') + ' ');
  assert.notEqual(versionOf(changed), manifest.version);
});

test('--check fails when glyphs.json differs', () => {
  // A private copy of the tree, so a failure can never leave a changed committed file behind.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'sm-glyphs-'));
  try {
    const copy = (rel) => {
      fs.mkdirSync(path.dirname(path.join(tmp, rel)), { recursive: true });
      fs.cpSync(path.join(root, rel), path.join(tmp, rel), { recursive: true });
    };
    copy('scripts/build-storymaker-catalog.mjs');
    copy('assets/storymaker/lib');
    copy('assets/storymaker/sfx');
    copy('ui/img/storymaker');
    copy('ui/lang/storymaker_catalog');
    for (const f of GLYPH_FACES) copy('ui' + f.file);
    const check = () => spawnSync(process.execPath, [path.join(tmp, 'scripts', 'build-storymaker-catalog.mjs'), '--check'], { cwd: tmp, encoding: 'utf8' });
    const clean = check();
    assert.equal(clean.status, 0, clean.stdout + clean.stderr);
    const file = path.join(tmp, 'ui', 'img', 'storymaker', 'glyphs.json');
    fs.writeFileSync(file, fs.readFileSync(file, 'utf8') + ' ', 'utf8');
    const changed = check();
    assert.notEqual(changed.status, 0);
    assert.match(changed.stderr, /differs: .*glyphs\.json/);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
