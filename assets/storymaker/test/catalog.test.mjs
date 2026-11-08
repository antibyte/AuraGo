// Task D7: catalog assembly, the build validator and the build script.
// The schema checks on the committed files live in ui/storymaker_catalog_test.go (task D8).
// Run: node --test assets/storymaker/test/catalog.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildOutputs, sameOutput, versionOf } from '../lib/catalog.mjs';
import { validateOutputs } from '../lib/validate.mjs';
import { NAMES, LANGS, localeTables, nameKey } from '../lib/names.mjs';
import { RASTER_NAMES } from '../lib/raster-names.mjs';
import { buildRasterOutputs } from '../lib/raster-catalog.mjs';
import { validateRasterOutputs } from '../lib/raster-validate.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

test('two builds produce identical bytes and the version hash follows the files', () => {
  const a = buildOutputs();
  const b = buildOutputs();
  assert.deepEqual([...a.files.keys()], [...b.files.keys()]);
  for (const [p, content] of a.files) assert.deepEqual(content, b.files.get(p), p);
  assert.equal(a.manifest.version, versionOf(a.files));
  assert.match(a.manifest.version, /^[0-9a-f]{12}$/);
  for (const text of a.files.values()) if (!Buffer.isBuffer(text)) assert.ok(!text.includes('\r'), 'LF only');
  // 105 phase 1 files, glyphs.json and 3 new thumbnails
  assert.equal(a.files.size, 134); // 109 after batch 2B plus 25 WAVs
});

test('versionOf hashes a Buffer raw and text LF-normalised, and sameOutput follows the same rule', () => {
  const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex').slice(0, 12);
  const crlf = Buffer.from([0x41, 0x0d, 0x0a, 0x42]); // the bytes of A CR LF B: in binary data that pair is not a line end
  const lfOnly = Buffer.from([0x41, 0x0a, 0x42]);
  assert.notEqual(hash(crlf), hash(lfOnly));
  assert.equal(versionOf(new Map([['sfx/a.wav', crlf]])), hash(crlf));
  assert.equal(versionOf(new Map([['a.json', 'A\r\nB']])), hash(lfOnly));
  assert.equal(sameOutput(crlf, Buffer.from(crlf)), true);
  assert.equal(sameOutput(lfOnly, crlf), false);
  assert.equal(sameOutput(crlf, 'A\nB'), true); // a text output survives a CRLF checkout
  assert.equal(sameOutput(Buffer.from('A\nC'), 'A\nB'), false);
});

test('the generated catalog passes the build validator', () => {
  assert.deepEqual(validateOutputs(buildOutputs()), []);
});

test('the validator catches missing ids, bad path data, unknown tokens and a stale version', () => {
  const clone = () => {
    const built = buildOutputs();
    return { ...built, files: new Map(built.files), entries: built.entries.slice() };
  };
  let c = clone();
  c.entries = c.entries.filter((e) => e.id !== 'prop.lock');
  assert.ok(validateOutputs(c).some((p) => p.includes('prop.lock')));

  c = clone();
  const gear = JSON.parse(c.files.get('props/gear.json'));
  gear.shapes[0].d = 'M0 0L10';
  c.files.set('props/gear.json', JSON.stringify(gear));
  assert.ok(validateOutputs(c).some((p) => p.includes('bad path data')));

  c = clone();
  const key = JSON.parse(c.files.get('props/key.json'));
  key.shapes[0].fill = '$nope';
  c.files.set('props/key.json', JSON.stringify(key));
  assert.ok(validateOutputs(c).some((p) => p.includes('unknown token $nope')));

  c = clone();
  c.files.set('props/key.json', c.files.get('props/key.json') + ' ');
  assert.ok(validateOutputs(c).some((p) => p.includes('version')));
});

test('locale rows have 16 real translations for every entry and palette', () => {
  const { manifest } = buildOutputs();
  const ids = [...manifest.entries.map((e) => e.id), ...manifest.palettes.map((p) => p.id)];
  assert.deepEqual(Object.keys(NAMES).filter(id => !Object.hasOwn(RASTER_NAMES, id)).sort(), ids.sort());
  for (const row of Object.values(NAMES)) assert.equal(row.length, LANGS.length);
});

test('the manifest follows the contract: stage, rig, fonts, palettes and entry shapes', () => {
  const { manifest, entries } = buildOutputs();
  assert.deepEqual(manifest.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
  assert.deepEqual(manifest.rig, { id: 'rig.human/1', file: 'rig/human.json' });
  assert.ok(manifest.fonts.every((f) => f.name === 'default' && f.family === 'Geist' && f.file.startsWith('/fonts/') && f.licence === 'OFL-1.1'));
  assert.deepEqual(manifest.palettes.map((p) => p.id), ['palette.sunny', 'palette.calm']);
  const byId = Object.fromEntries(entries.map((e) => [e.id, e]));
  assert.equal(byId['pose.typing'].name, 'typing');
  assert.equal(byId['pose.typing'].name_key, 'storymaker.catalog.pose_typing');
  assert.equal(byId['char.office-woman-1'].name, 'char.office-woman-1');
  assert.deepEqual(byId['prop.lock'].usage_example, { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' });
  assert.deepEqual(byId['env.office'].variants, ['day', 'night']);
  assert.equal(byId['cycle.walk'].speed, 16);
  assert.equal(byId['cycle.idle'].speed, undefined);
  assert.equal(entries.length, 86); // 79 after batch 2B plus 7 sfx
});

test('the bubble and subtitle styles have size 0.042 and a name in 16 locales', () => {
  const { entries } = buildOutputs();
  const tables = localeTables();
  assert.equal(LANGS.length, 16);
  for (const id of ['text.speech', 'text.thought', 'text.subtitle']) {
    const style = entries.find((e) => e.id === id);
    assert.equal(style?.kind, 'text_style', id);
    assert.equal(style.size, 0.042, id);
    for (const lang of LANGS) assert.ok(tables[lang][nameKey(id)]?.trim(), id + ' in ' + lang);
  }
});

test('the sfx entries follow the frozen shape', () => {
  const { entries, files } = buildOutputs();
  const sfx = entries.filter((e) => e.kind === 'sfx');
  // frozen catalog sfx entry: seven names, 4 + 4 + 4 + 3 + 2 + 2 + 6 = 25 takes
  assert.deepEqual(sfx.map((e) => [e.id, e.takes.length]), [['sfx.whoosh', 4], ['sfx.pop', 4], ['sfx.click', 4], ['sfx.ding', 3], ['sfx.success', 2], ['sfx.error', 2], ['sfx.typing', 6]]);
  const byId = Object.fromEntries(sfx.map((e) => [e.id, e]));
  assert.deepEqual(byId['sfx.typing'].pattern, { spacing_ms: [90, 140], default_duration_ms: 1200 });
  assert.equal(byId['sfx.typing'].hit, undefined);
  assert.equal(byId['sfx.error'].align, 'onset');
  assert.equal(byId['sfx.success'].align, 'onset');
  assert.equal(byId['sfx.pop'].align, 'peak');
  assert.deepEqual(byId['sfx.pop'].hit, { over_db: 4.5, dip_db: 3 });
  assert.deepEqual(byId['sfx.ding'].hit, { over_db: 4.5, dip_db: 3 });
  assert.deepEqual(byId['sfx.success'].hit, { over_db: 4.5, dip_db: 4 });
  assert.deepEqual(byId['sfx.error'].hit, { over_db: 4.5, dip_db: 4 });
  assert.equal(byId['sfx.whoosh'].hit, undefined);
  assert.equal(byId['sfx.click'].hit, undefined);
  assert.equal(byId['sfx.pop'].impact_ms, 3.75); // (3.5 + 4.5 + 3.5 + 3.5) / 4
  for (const e of sfx) {
    assert.equal(e.thumb, '', e.id);
    assert.equal(e.file, e.takes[0].file, e.id);
    for (const t of e.takes) assert.ok(Buffer.isBuffer(files.get(t.file)), t.file);
  }
});

test('the raster build validator refuses changed resources and invalid joint calibration', () => {
  const built = buildRasterOutputs();
  const changed = Buffer.from(built.files.get('sfx/pop_1.wav'));
  changed[changed.length - 1] ^= 1;
  built.files.set('sfx/pop_1.wav', changed);
  assert.ok(validateRasterOutputs(built).some(p => p.includes('Incorrect inventory: sfx/pop_1.wav')));
  const file='characters/office-woman-1.json',descriptor=JSON.parse(built.files.get(file));
  for(const shear of ['bad',null,1,-1]){
    descriptor.views.front[0].shear=shear;
    built.files.set(file,JSON.stringify(descriptor));
    assert.ok(validateRasterOutputs(built).some(p=>p.includes('Bad joint calibration: char.office-woman-1/')));
  }
});

test('node scripts/build-storymaker-catalog.mjs --check is green on the committed outputs', () => {
  const r = spawnSync(process.execPath, [path.join(root, 'scripts', 'build-storymaker-catalog.mjs'), '--check'], { cwd: root, encoding: 'utf8' });
  assert.equal(r.status, 0, r.stdout + r.stderr);
});
