# Part G: Glyph coverage and text flow

Batch 2B. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-G-1 -->

### Task G1: Coverage build from the font cmaps
**Batch:** 2B  **Depends on:** M3
**Files:**
- Create: `assets/storymaker/lib/glyphs.mjs`
- Modify: `assets/storymaker/lib/catalog.mjs`
- Modify: `assets/storymaker/lib/validate.mjs`
- Modify: `scripts/build-storymaker-catalog.mjs`
- Create (generated): `ui/img/storymaker/glyphs.json`
- Modify (generated): `ui/img/storymaker/catalog.json`
- Test: `assets/storymaker/test/glyphs.test.mjs` (new)
- Test: `assets/storymaker/test/catalog.test.mjs`
- Test: `ui/storymaker_catalog_test.go`

Run every command from the root of `.worktrees/storymaker-p2b`, except impact, which runs from `C:/Users/Andi/Documents/repo/AuraGo`.

- [ ] **Step 1: Impact**

Run this for each of `buildOutputs`, `manifestText`, `validateOutputs`, `listExisting`, `smCatalog` and `TestStorymakerCatalogFilesVersionAndBudget`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildOutputs" --direction upstream --repo .
```

Report callers, processes and risk. Per outline section 5, warn on HIGH or CRITICAL, and confirm UNKNOWN with a text search.

- [ ] **Step 2: Write the failing test**

Create `assets/storymaker/test/glyphs.test.mjs`:

```js
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
```

In `assets/storymaker/test/catalog.test.mjs`, replace this line. The new value is 105 plus `glyphs.json`.

Old:
```js
  assert.equal(a.files.size, 105);
```
New:
```js
  assert.equal(a.files.size, 106);
```

In `ui/storymaker_catalog_test.go`, make two replacements.

Old:
```go
	Rig      map[string]any   `json:"rig"`
```
New:
```go
	Rig      map[string]any   `json:"rig"`
	Glyphs   map[string]any   `json:"glyphs"`
```

Old:
```go
	referenced := map[string]bool{"catalog.json": true, smStr(c.Rig, "file"): true}
```
New:
```go
	referenced := map[string]bool{"catalog.json": true, smStr(c.Rig, "file"): true, smStr(c.Glyphs, "file"): true}
```

- [ ] **Step 3: Run it**

```powershell
node --test assets/storymaker/test/glyphs.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
```

Expected failures:
- `glyphs.test.mjs` fails with `ERR_MODULE_NOT_FOUND` for `../lib/glyphs.mjs`, because the module does not exist yet.
- `catalog.test.mjs` fails with `105 !== 106`, because `buildOutputs` has no `glyphs.json` yet.

- [ ] **Step 4: Implement**

Create `assets/storymaker/lib/glyphs.mjs`:

```js
// Fallback font coverage, schema storymaker-glyphs/1. The catalog build reads the cmap of each face and writes
// ui/img/storymaker/glyphs.json. Go and the player read only that file, so neither needs a font parser.
// Ported from docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/fonts/cmap.mjs and glyphs.mjs.

import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export const GLYPHS_SCHEMA = 'storymaker-glyphs/1';

const MIB = 1048576;
const DEFAULT_FONT_CAP = 8 * MIB;

// Faces in fallback order. Greek is first with its range, because Geist has only 6 Greek letters and Chrome would draw
// omega as a capital Omega from them. CJK is last, because it also covers Latin.
export const GLYPH_FACES = [
  { id: 'sm-greek', family: 'SMGreek', file: '/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2', weight: '300 700', unicode_range: 'U+0370-03FF' },
  { id: 'sm-greek-ext', family: 'SMGreek', file: '/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2', weight: '300 700', unicode_range: 'U+1F00-1FFF' },
  { id: 'geist', family: 'Geist', file: '/fonts/Geist-Regular.woff2', weight: '400', unicode_range: '', exclude: [[0x0370, 0x03ff]] },
  { id: 'sm-deva', family: 'SMDeva', file: '/fonts/qr/NotoSansDevanagari.ttf', weight: '100 900', unicode_range: '' },
  { id: 'sm-arab', family: 'SMArab', file: '/fonts/qr/NotoSansArabic.ttf', weight: '100 900', unicode_range: '' },
  { id: 'sm-cjk', family: 'SMCJK', file: '/fonts/qr/NotoSansCJKsc.otf', weight: '400', unicode_range: '', max_bytes: 17 * MIB },
];

// The known table tags of the WOFF2 format, in the order of its specification.
const WOFF2_TAGS = ['cmap', 'head', 'hhea', 'hmtx', 'maxp', 'name', 'OS/2', 'post', 'cvt ', 'fpgm', 'glyf', 'loca', 'prep', 'CFF ', 'VORG', 'EBDT', 'EBLC', 'gasp', 'hdmx', 'kern', 'LTSH', 'PCLT', 'VDMX', 'vhea', 'vmtx', 'BASE', 'GDEF', 'GPOS', 'GSUB', 'EBSC', 'JSTF', 'MATH', 'CBDT', 'CBLC', 'COLR', 'CPAL', 'SVG ', 'sbix', 'acnt', 'avar', 'bdat', 'bloc', 'bsln', 'cvar', 'fdsc', 'feat', 'fmtx', 'fvar', 'gvar', 'hsty', 'just', 'lcar', 'mort', 'morx', 'opbd', 'prop', 'trak', 'Zapf', 'Silf', 'Glat', 'Gloc', 'Feat', 'Sill'];

/** The tables of a WOFF2 file by tag. Transformed tables stay as stored; only cmap and name are read. */
export function woff2Tables(buf) {
  if (buf.toString('latin1', 0, 4) !== 'wOF2') throw new Error('not a WOFF2 file');
  const numTables = buf.readUInt16BE(12);
  const totalCompressed = buf.readUInt32BE(20);
  let p = 48;
  const base128 = () => {
    let value = 0;
    for (let i = 0; i < 5; i++) {
      const b = buf[p++];
      value = value * 128 + (b & 127);
      if (!(b & 128)) return value;
    }
    throw new Error('bad base128 value');
  };
  const entries = [];
  for (let i = 0; i < numTables; i++) {
    const flags = buf[p++];
    let tag;
    if ((flags & 63) === 63) {
      tag = buf.toString('latin1', p, p + 4);
      p += 4;
    } else {
      tag = WOFF2_TAGS[flags & 63];
    }
    const version = flags >> 6;
    const orig = base128();
    const transformed = tag === 'glyf' || tag === 'loca' ? version === 0 : version !== 0;
    entries.push({ tag, length: transformed ? base128() : orig });
  }
  const data = zlib.brotliDecompressSync(buf.subarray(p, p + totalCompressed));
  const tables = {};
  let offset = 0;
  for (const e of entries) {
    tables[e.tag] = data.subarray(offset, offset + e.length);
    offset += e.length;
  }
  return tables;
}

/** The tables of a TTF, OTF or font collection by tag. */
export function sfntTables(buf) {
  const base = buf.toString('latin1', 0, 4) === 'ttcf' ? buf.readUInt32BE(12) : 0;
  const count = buf.readUInt16BE(base + 4);
  const tables = {};
  for (let i = 0; i < count; i++) {
    const e = base + 12 + i * 16;
    const offset = buf.readUInt32BE(e + 8);
    tables[buf.toString('latin1', e, e + 4)] = buf.subarray(offset, offset + buf.readUInt32BE(e + 12));
  }
  return tables;
}

function readCmapFormat4(c, off, set) {
  const segX2 = c.readUInt16BE(off + 6);
  const segs = segX2 / 2;
  const endO = off + 14;
  const startO = endO + segX2 + 2;
  const deltaO = startO + segX2;
  const rangeO = deltaO + segX2;
  for (let s = 0; s < segs; s++) {
    const end = c.readUInt16BE(endO + s * 2);
    const start = c.readUInt16BE(startO + s * 2);
    const delta = c.readInt16BE(deltaO + s * 2);
    const ro = c.readUInt16BE(rangeO + s * 2);
    for (let cp = start; cp <= end && cp !== 0xffff; cp++) {
      let g;
      if (ro === 0) {
        g = (cp + delta) & 0xffff;
      } else {
        g = c.readUInt16BE(rangeO + s * 2 + ro + (cp - start) * 2);
        if (g) g = (g + delta) & 0xffff;
      }
      if (g) set.add(cp);
    }
  }
}

/** The Unicode code points of a cmap table, without U+0000. */
export function parseCmap(c) {
  const set = new Set();
  const n = c.readUInt16BE(2);
  for (let i = 0; i < n; i++) {
    const plat = c.readUInt16BE(4 + i * 8);
    const enc = c.readUInt16BE(6 + i * 8);
    const off = c.readUInt32BE(8 + i * 8);
    if (plat !== 0 && !(plat === 3 && (enc === 1 || enc === 10))) continue;
    const fmt = c.readUInt16BE(off);
    if (fmt === 4) {
      readCmapFormat4(c, off, set);
    } else if (fmt === 12 || fmt === 13) {
      const groups = c.readUInt32BE(off + 12);
      for (let g = 0; g < groups; g++) {
        const first = c.readUInt32BE(off + 16 + g * 12);
        const last = c.readUInt32BE(off + 20 + g * 12);
        for (let cp = first; cp <= last; cp++) set.add(cp);
      }
    } else if (fmt === 6) {
      const first = c.readUInt16BE(off + 6);
      const count = c.readUInt16BE(off + 8);
      for (let k = 0; k < count; k++) if (c.readUInt16BE(off + 10 + k * 2)) set.add(first + k);
    } else if (fmt === 0) {
      for (let k = 0; k < 256; k++) if (c[off + 6 + k]) set.add(k);
    }
  }
  set.delete(0);
  return set;
}

function parseUnicodeRange(text) {
  if (!text) return null;
  const m = /^U\+([0-9A-F]+)-([0-9A-F]+)$/.exec(text);
  if (!m) throw new Error(`bad unicode_range ${text}`);
  return [parseInt(m[1], 16), parseInt(m[2], 16)];
}

// A code point is covered if it is in the face's cmap, inside its unicode_range and outside its exclusions.
function effectiveCodepoints(face) {
  const buf = fs.readFileSync(path.join(ROOT, 'ui', ...face.file.split('/')));
  const cap = face.max_bytes || DEFAULT_FONT_CAP;
  if (buf.length > cap) throw new Error(`${face.file}: ${buf.length} bytes is over its cap of ${cap}`);
  const tables = face.file.endsWith('.woff2') ? woff2Tables(buf) : sfntTables(buf);
  if (!tables.cmap) throw new Error(`${face.file}: no cmap table`);
  const only = parseUnicodeRange(face.unicode_range);
  const exclude = face.exclude || [];
  return [...parseCmap(tables.cmap)]
    .filter((cp) => (!only || (cp >= only[0] && cp <= only[1])) && !exclude.some(([a, b]) => cp >= a && cp <= b))
    .sort((a, b) => a - b);
}

/** Sorted code points to the delta format: first value the start, then gap = start - previous end, len = end - start. */
export function encodeRanges(cps) {
  const ranges = [];
  for (const cp of cps) {
    const last = ranges[ranges.length - 1];
    if (last && cp === last[1] + 1) last[1] = cp;
    else ranges.push([cp, cp]);
  }
  const d = [];
  let prevEnd = 0;
  for (const [start, end] of ranges) {
    d.push(start - prevEnd, end - start);
    prevEnd = end;
  }
  return d;
}

/** The delta format to inclusive [start, end] pairs. */
export function decodeRanges(d) {
  const out = [];
  let end = 0;
  for (let i = 0; i < d.length; i += 2) {
    const start = end + d[i];
    end = start + d[i + 1];
    out.push([start, end]);
  }
  return out;
}

export function rangesCover(ranges, cp) {
  return ranges.some(([a, b]) => cp >= a && cp <= b);
}

let cached = null;

/** The storymaker-glyphs/1 document. Reading the CJK font takes a moment, so one process reads it once. */
export function buildGlyphs() {
  if (cached) return cached;
  const faces = GLYPH_FACES.map((face) => {
    const { exclude, ...row } = face;
    return { ...row, d: encodeRanges(effectiveCodepoints(face)) };
  });
  cached = { schema: GLYPHS_SCHEMA, faces };
  return cached;
}

/** The file text: one face per line, LF line endings. */
export function glyphsText(data) {
  const rows = data.faces.map((f) => '  ' + JSON.stringify(f));
  return `{"schema":${JSON.stringify(data.schema)},"faces":[\n${rows.join(',\n')}\n]}\n`;
}
```

In `assets/storymaker/lib/catalog.mjs`, make four replacements.

Old:
```js
import { roundRect, fill, line, poly, circle } from './util.mjs';
```
New:
```js
import { roundRect, fill, line, poly, circle } from './util.mjs';
import { buildGlyphs, glyphsText } from './glyphs.mjs';
```

Old:
```js
  lines.push('{' + [`"schema":${j(manifest.schema)}`, `"version":${j(manifest.version)}`, `"stage":${j(manifest.stage)}`, `"rig":${j(manifest.rig)}`].join(',') + ',');
```
New:
```js
  lines.push('{' + [`"schema":${j(manifest.schema)}`, `"version":${j(manifest.version)}`, `"stage":${j(manifest.stage)}`, `"rig":${j(manifest.rig)}`, `"glyphs":${j(manifest.glyphs)}`].join(',') + ',');
```

Old:
```js
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
```
New:
```js
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
  files.set('glyphs.json', glyphsText(buildGlyphs()));
```

Old:
```js
    rig: { id: 'rig.human/1', file: 'rig/human.json' },
```
New:
```js
    rig: { id: 'rig.human/1', file: 'rig/human.json' },
    glyphs: { file: 'glyphs.json' },
```

In `assets/storymaker/lib/validate.mjs`, make three replacements.

Old:
```js
import { MOUTH_NAMES } from './face.mjs';
```
New:
```js
import { MOUTH_NAMES } from './face.mjs';
import { GLYPHS_SCHEMA, decodeRanges, rangesCover } from './glyphs.mjs';
```

Old:
```js
export function validateOutputs(built, { maxProblems = 40 } = {}) {
```
New:
```js
function glyphProblems(data, add) {
  if (data.schema !== GLYPHS_SCHEMA) add(`glyphs.json: schema must be ${GLYPHS_SCHEMA}`);
  const ids = new Set();
  for (const f of data.faces) {
    if (ids.has(f.id)) add(`glyphs.json: duplicate face ${f.id}`);
    ids.add(f.id);
    if (!f.family || !f.weight || !f.file.startsWith('/fonts/')) add(`glyphs.json: face ${f.id} needs a family, a weight and a /fonts/ file`);
    if (!f.d.length || f.d.length % 2) add(`glyphs.json: face ${f.id} needs pairs of range values`);
    for (let i = 0; i < f.d.length; i += 2) {
      if (f.d[i] < (i ? 1 : 0) || f.d[i + 1] < 0) add(`glyphs.json: face ${f.id} is not monotonic at value ${i}`);
    }
  }
  if (!data.faces.some((f) => rangesCover(decodeRanges(f.d), 0x2026))) add('glyphs.json: U+2026 is not covered by any face');
}

export function validateOutputs(built, { maxProblems = 40 } = {}) {
```

Old:
```js
  return out;
```
New:
```js
  // fallback coverage
  if (manifest.glyphs?.file !== 'glyphs.json') add('the manifest must name glyphs.json');
  const glyphText = files.get('glyphs.json');
  if (typeof glyphText !== 'string') add('glyphs.json is missing');
  else glyphProblems(JSON.parse(glyphText), add);
  return out;
```

In `scripts/build-storymaker-catalog.mjs`, make one replacement.

Old:
```js
  if (fs.existsSync(path.join(outDir, 'catalog.json'))) found.push(path.join(outDir, 'catalog.json'));
```
New:
```js
  for (const f of ['catalog.json', 'glyphs.json']) if (fs.existsSync(path.join(outDir, f))) found.push(path.join(outDir, f));
```

Do not paste `ui/img/storymaker/glyphs.json` or `ui/img/storymaker/catalog.json`. Step 5 generates them.

- [ ] **Step 5: Run**

```powershell
node scripts/build-storymaker-catalog.mjs
git status --short ui/img/storymaker ui/lang/storymaker_catalog
node --test assets/storymaker/test/glyphs.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
npm run check:storymaker-catalog
npm run test:storymaker
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerCatalog' -count=1 -timeout 20m
go test ./internal/storymaker -run RealCatalog -count=1
```

Expected:
- `git status` lists only a changed `ui/img/storymaker/catalog.json` and a new `ui/img/storymaker/glyphs.json`.
- Every test and check passes.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add assets/storymaker/lib/glyphs.mjs assets/storymaker/lib/catalog.mjs assets/storymaker/lib/validate.mjs scripts/build-storymaker-catalog.mjs ui/img/storymaker/glyphs.json ui/img/storymaker/catalog.json assets/storymaker/test/glyphs.test.mjs assets/storymaker/test/catalog.test.mjs ui/storymaker_catalog_test.go
git commit -m "feat(storymaker): build glyph coverage data from the font cmaps" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Per outline section 5, a PARTIAL answer on new files is accepted with a grep confirmation. Add that as one `-m` paragraph before the trailer. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the same place.

### Task G2: Font notices and QR note
**Batch:** 2B  **Depends on:** G1
**Files:**
- Create: `ui/fonts/Inter-LICENSE.txt`
- Modify: `licences/THIRD_PARTY_LIBRARIES.md`
- Modify: `ui/fonts/qr/AGENTS.md`
- Modify: `internal/storymaker/AGENTS.md`
- Test: `ui/storymaker_catalog_test.go`

Run every command from the root of `.worktrees/storymaker-p2b`.

- [ ] **Step 1: Write the failing test**

In `ui/storymaker_catalog_test.go`, add the test by replacing this line.

Old:
```go
// ---------------------------------------------------------------- locale names
```
New:
```go
// Every fallback face has a notice row in the inventory that names its file, and Inter has its licence text next to the fonts.
func TestStorymakerGlyphFaceNotices(t *testing.T) {
	var glyphs struct {
		Faces []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"faces"`
	}
	smReadJSON(t, "glyphs.json", &glyphs)
	if len(glyphs.Faces) == 0 {
		t.Fatal("glyphs.json lists no faces")
	}
	inventory, err := os.ReadFile(filepath.Join("..", "licences", "THIRD_PARTY_LIBRARIES.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, line := range strings.Split(string(smLF(inventory)), "\n") {
		if strings.HasPrefix(line, "| ") && strings.Contains(line, "SIL Open Font License 1.1") {
			rows = append(rows, line)
		}
	}
	for _, face := range glyphs.Faces {
		base := face.File[strings.LastIndex(face.File, "/")+1:]
		found := false
		for _, row := range rows {
			found = found || strings.Contains(row, base)
		}
		if !found {
			t.Errorf("face %s: no OFL row in THIRD_PARTY_LIBRARIES.md names %s", face.ID, base)
		}
	}
	licence, err := os.ReadFile(filepath.Join("fonts", "Inter-LICENSE.txt"))
	if err != nil {
		t.Fatalf("Inter licence: %v", err)
	}
	for _, want := range []string{"Inter Project Authors", "SIL OPEN FONT LICENSE Version 1.1"} {
		if !bytes.Contains(licence, []byte(want)) {
			t.Errorf("fonts/Inter-LICENSE.txt does not contain %q", want)
		}
	}
}

// ---------------------------------------------------------------- locale names
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerGlyphFaceNotices$' -count=1
```

Expected: FAIL.
- All six faces report that no OFL row names their file, because today's font rows name no files.
- The test then stops on `Inter licence:`, because `ui/fonts/Inter-LICENSE.txt` does not exist.

- [ ] **Step 3: Implement**

Generate the licence with a script, so the OFL text stays verbatim. Write this file with the Write tool into your session scratchpad as `make-inter-license.mjs`:

```js
// usage: node make-inter-license.mjs <repo root>
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const root = path.resolve(process.argv[2]);
const { woff2Tables } = await import(pathToFileURL(path.join(root, 'assets/storymaker/lib/glyphs.mjs')).href);

// The line of the google/fonts copy of Inter's OFL file, used only when the font has no such name record.
const FALLBACK = 'Copyright 2020 The Inter Project Authors (https://github.com/rsms/inter)';

function fontCopyright() {
  const font = fs.readFileSync(path.join(root, 'ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2'));
  const name = woff2Tables(font).name;
  if (!name) return '';
  const count = name.readUInt16BE(2);
  const strings = name.readUInt16BE(4);
  for (let i = 0; i < count; i++) {
    const r = 6 + i * 12;
    // platform 3 stores UTF-16BE; name id 0 is the copyright notice
    if (name.readUInt16BE(r) === 3 && name.readUInt16BE(r + 6) === 0) {
      const length = name.readUInt16BE(r + 8);
      const offset = name.readUInt16BE(r + 10);
      return Buffer.from(name.subarray(strings + offset, strings + offset + length)).swap16().toString('utf16le').trim();
    }
  }
  return '';
}

const read = fontCopyright();
const copyright = read.includes('Inter Project Authors') ? read : FALLBACK;
console.log(copyright === read ? 'copyright line read from the font:' : 'copyright line from the fallback:', copyright);
const body = fs.readFileSync(path.join(root, 'ui/fonts/Montserrat-LICENSE.txt'), 'utf8').replace(/\r\n/g, '\n').split('\n').slice(1).join('\n');
fs.writeFileSync(path.join(root, 'ui/fonts/Inter-LICENSE.txt'), copyright + '\n' + body, 'utf8');
```

Run it from the worktree root, with the absolute path of your scratchpad:

```powershell
node <session scratchpad>/make-inter-license.mjs .
```

Record in your report whether it printed "read from the font" or "from the fallback".

In `licences/THIRD_PARTY_LIBRARIES.md`, make two replacements.

Old:
```text
| Geist | SIL Open Font License 1.1 | https://vercel.com/font · https://github.com/vercel/geist-font |
```
New:
```text
| Geist | SIL Open Font License 1.1 | https://vercel.com/font · https://github.com/vercel/geist-font · `ui/fonts/Geist-Regular.woff2` also carries the Latin coverage of the Storymaker fallback chain |
```

Old:
```text
| Inter | SIL Open Font License 1.1 | https://github.com/rsms/inter |
```
New:
```text
| Inter | SIL Open Font License 1.1 | https://github.com/rsms/inter · `ui/fonts/Inter-LICENSE.txt` · the Greek subset `ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2` and the Greek Extended subset `ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2` are also Storymaker fallback faces |
| Noto Sans Devanagari | SIL Open Font License 1.1 | https://github.com/notofonts/devanagari · `ui/fonts/qr/NotoSansDevanagari.ttf`, licence `ui/fonts/qr/OFL-Devanagari.txt`, revision and hash in `ui/fonts/qr/manifest.json`; QR captions and Storymaker fallback face |
| Noto Sans Arabic | SIL Open Font License 1.1 | https://github.com/notofonts/arabic · `ui/fonts/qr/NotoSansArabic.ttf`, licence `ui/fonts/qr/OFL-Arabic.txt`, revision and hash in `ui/fonts/qr/manifest.json`; QR captions and Storymaker fallback face |
| Noto Sans CJK SC | SIL Open Font License 1.1 | https://github.com/notofonts/noto-cjk · `ui/fonts/qr/NotoSansCJKsc.otf`, licence `ui/fonts/qr/OFL-CJK.txt`, revision and hash in `ui/fonts/qr/manifest.json`; QR captions and Storymaker fallback face |
```

In `ui/fonts/qr/AGENTS.md`, make one replacement.

Old:
```text
- CJK uses the Simplified Chinese regional glyph style and covers Japanese kana/Han; regional typographic variants are not promised.
```
New:
```text
- CJK uses the Simplified Chinese regional glyph style and covers Japanese kana/Han; regional typographic variants are not promised.
- Storymaker also uses `NotoSansDevanagari.ttf`, `NotoSansArabic.ttf` and `NotoSansCJKsc.otf` as fallback faces. `ui/img/storymaker/glyphs.json` holds their coverage, and `NotoSansCJKsc.otf` has a named `max_bytes` exception of 17 MiB there. After replacing or renaming one of these files, run `node scripts/build-storymaker-catalog.mjs`, because the coverage data and the catalog version follow the font bytes. Do not subset them: a subset breaks the per-character check.
```

In `internal/storymaker/AGENTS.md`, make one replacement. `internal/storymaker/glyphs.go` arrives with Task G3 in this batch.

Old:
```text
## Child DOX Index
```
New:
```text
## Fallback fonts (part G)

- `ui/img/storymaker/glyphs.json` (`storymaker-glyphs/1`) is generated by `scripts/build-storymaker-catalog.mjs` from the font cmaps (`assets/storymaker/lib/glyphs.mjs`). It joins the catalog version and `--check`. Never edit it by hand.
- Faces stay out of `catalog.fonts`, so `style.font` cannot name them. A face file that joins `glyphs.json` needs a licence text and a row in `licences/THIRD_PARTY_LIBRARIES.md` that names the file. `TestStorymakerGlyphFaceNotices` in `ui/storymaker_catalog_test.go` enforces this.
- `glyphs.go` reads the file through `LoadCatalog`. Go has no WOFF2 code.

## Child DOX Index
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(StorymakerGlyphFaceNotices|DesignerFontsVendored|C64)' -count=1
Get-Content ui/fonts/Inter-LICENSE.txt -TotalCount 3
```

Expected:
- The tests PASS. The other two names guard rows in the same inventory.
- The first line of the licence is the Inter copyright line, and the third line starts with `This Font Software`.

- [ ] **Step 5: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/fonts/Inter-LICENSE.txt licences/THIRD_PARTY_LIBRARIES.md ui/storymaker_catalog_test.go
git add -f ui/fonts/qr/AGENTS.md internal/storymaker/AGENTS.md
git commit -m "docs(licences): add notices for the Storymaker fallback faces" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Per outline section 5, a PARTIAL answer on new files is accepted with a grep confirmation. Add that as one `-m` paragraph before the trailer. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the same place.

### Task G3: Go coverage reader
**Batch:** 2B  **Depends on:** G1, M3
**Files:**
- Create: `internal/storymaker/glyphs.go`
- Create: `internal/storymaker/narration.go`
- Modify: `internal/storymaker/catalog.go`
- Test: `internal/storymaker/glyphs_test.go` (new)
- Test: `internal/storymaker/catalog_real_test.go`

Run every command from the root of `.worktrees/storymaker-p2b`, except impact, which runs from `C:/Users/Andi/Documents/repo/AuraGo`. `internal/storymaker/narration.go` holds only `IsJSSpace` here. Task N1 fills the rest of it.

- [ ] **Step 1: Impact**

Run this for each of `LoadCatalog`, `Catalog` and `catalogFile`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "LoadCatalog" --direction upstream --repo .
```

Report callers, processes and risk. Per outline section 5, warn on HIGH or CRITICAL, and confirm UNKNOWN with a text search.

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/glyphs_test.go`:

```go
package storymaker

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// Face "a": d [233,0] is U+00E9 only.
// Face "b": d [65,2,10,3,21,0,132,0] decodes to 65-67, then 67+10=77 to 77+3=80, then 80+21=101, then 101+132=233.
const glyphsFixture = `{"schema":"storymaker-glyphs/1","faces":[
 {"id":"a","family":"A","file":"/fonts/a.woff2","weight":"300 700","unicode_range":"U+00E9-00E9","d":[233,0]},
 {"id":"b","family":"B","file":"/fonts/qr/b.otf","weight":"400","unicode_range":"","max_bytes":17825792,"d":[65,2,10,3,21,0,132,0]}
]}`

func loadGlyphsFixture(t *testing.T) *Glyphs {
	t.Helper()
	g, err := LoadGlyphs(fstest.MapFS{"glyphs.json": {Data: []byte(glyphsFixture)}}, "glyphs.json")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestLoadGlyphsDecodesFacesInOrder(t *testing.T) {
	g := loadGlyphsFixture(t)
	want := []GlyphFace{
		{ID: "a", Family: "A", File: "/fonts/a.woff2", Weight: "300 700", UnicodeRange: "U+00E9-00E9"},
		{ID: "b", Family: "B", File: "/fonts/qr/b.otf", Weight: "400", MaxBytes: 17825792},
	}
	if !reflect.DeepEqual(g.Faces, want) {
		t.Fatalf("Faces = %+v", g.Faces)
	}
	// U+00E9 is in both faces, and the first one in order wins. 68 and 76 fall in the gap between the first two ranges of "b".
	for r, face := range map[rune]int{0xE9: 0, 65: 1, 67: 1, 68: -1, 76: -1, 77: 1, 80: 1, 81: -1, 101: 1} {
		if got := g.FaceOf(r); got != face {
			t.Errorf("FaceOf(%U) = %d, want %d", r, got, face)
		}
	}
}

func TestLoadGlyphsRefusesBadFiles(t *testing.T) {
	face := func(d string) string {
		return `{"schema":"storymaker-glyphs/1","faces":[{"id":"a","family":"A","file":"/fonts/a.woff2","weight":"400","unicode_range":"","d":` + d + `}]}`
	}
	cases := []struct {
		name string
		data string
		want string
	}{
		{"gap of zero", face("[65,2,0,1]"), "monotonic"},
		{"negative gap", face("[65,2,-3,1]"), "monotonic"},
		{"odd value count", face("[65,2,10]"), "pairs"},
		{"past the last code point", face("[1114111,1]"), "U+10FFFF"},
		{"wrong schema", strings.Replace(glyphsFixture, "storymaker-glyphs/1", "storymaker-glyphs/2", 1), "schema"},
		{"file outside the fonts folder", strings.Replace(glyphsFixture, "/fonts/a.woff2", "a.woff2", 1), "/fonts/"},
		{"over 1 MiB", `{"schema":"storymaker-glyphs/1","faces":[],"pad":"` + strings.Repeat("x", 1<<20) + `"}`, "larger than"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadGlyphs(fstest.MapFS{"glyphs.json": {Data: []byte(tc.data)}}, "glyphs.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestGlyphsCoveredExemptions(t *testing.T) {
	g := loadGlyphsFixture(t)
	// U+2028 and U+2009 are JS whitespace, U+200C and U+200D are default-ignorable, U+0001 is a C0 control.
	for _, r := range []rune{0xE9, 0x2028, 0x2009, 0x200C, 0x200D, 0x0001} {
		if !g.Covered(r) {
			t.Errorf("Covered(%U) = false", r)
		}
	}
	for _, r := range []rune{0x0E01, 0x0301} {
		if g.Covered(r) {
			t.Errorf("Covered(%U) = true", r)
		}
	}
	if !IsDefaultIgnorable(0x200C) || !IsDefaultIgnorable(0x200D) || IsDefaultIgnorable('a') {
		t.Error("IsDefaultIgnorable does not follow the generated table")
	}
}

func TestGlyphsMissingUsesTheNFCFormAndKeepsOrder(t *testing.T) {
	g := loadGlyphsFixture(t)
	// e + U+0301 composes to U+00E9, which the fixture covers. Read raw, U+0301 would be missing.
	if got := g.Missing("e" + string(rune(0x0301))); got != nil {
		t.Errorf("NFC form: Missing = %U", got)
	}
	text := string([]rune{0x0E02, 0x0E01, 0x0E02, 'e'})
	if got, want := g.Missing(text), []rune{0x0E02, 0x0E01}; !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %U, want %U", got, want)
	}
}

func TestIsJSSpaceIsExactlyTheFrozenSet(t *testing.T) {
	for _, r := range []rune{0x09, 0x0D, 0x20, 0xA0, 0x1680, 0x2000, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF} {
		if !IsJSSpace(r) {
			t.Errorf("IsJSSpace(%U) = false", r)
		}
	}
	for _, r := range []rune{0x08, 0x0E, 0x85, 0x180E, 0x200B, 0x2030, 0x205E, 0xFEFE} {
		if IsJSSpace(r) {
			t.Errorf("IsJSSpace(%U) = true", r)
		}
	}
}

func TestMiniCatalogHasNoGlyphsAndSkipsTheCheck(t *testing.T) {
	c := loadMini(t)
	if c.Glyphs() != nil || c.GlyphsFile() != "" {
		t.Fatalf("mini catalog: Glyphs() = %v, GlyphsFile() = %q", c.Glyphs(), c.GlyphsFile())
	}
	thai := rune(0x0E01)
	if c.Glyphs().Missing(string(thai)) != nil || !c.Glyphs().Covered(thai) {
		t.Error("a nil Glyphs must skip the check")
	}
	var none *Catalog
	if none.Glyphs() != nil || none.GlyphsFile() != "" {
		t.Error("a nil catalog must have no glyphs")
	}
}

func TestLoadCatalogReadsGlyphsFromItsFolder(t *testing.T) {
	named := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"glyphs.json"},"rig":{`, 1)
	})
	fsys := fstest.MapFS{"img/catalog.json": {Data: named}, "img/glyphs.json": {Data: []byte(glyphsFixture)}}
	c, err := LoadCatalog(fsys, "img/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.GlyphsFile() != "glyphs.json" || c.Glyphs().FaceOf(65) != 1 {
		t.Fatalf("GlyphsFile() = %q, FaceOf(65) = %d", c.GlyphsFile(), c.Glyphs().FaceOf(65))
	}
	delete(fsys, "img/glyphs.json")
	if _, err := LoadCatalog(fsys, "img/catalog.json"); err == nil || !strings.Contains(err.Error(), "glyphs") {
		t.Errorf("a missing glyphs file: error %v", err)
	}
	escaping := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"../glyphs.json"},"rig":{`, 1)
	})
	if _, err := LoadCatalog(fstest.MapFS{"img/catalog.json": {Data: escaping}}, "img/catalog.json"); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Errorf("an escaping glyphs path: error %v", err)
	}
}
```

In `internal/storymaker/catalog_real_test.go`, add the real-catalog test by replacing this line.

Old:
```go
func TestRealCatalogValidatesTheFixtureStories(t *testing.T) {
```
New:
```go
func TestRealCatalogGlyphs(t *testing.T) {
	cat := loadReal(t)
	g := cat.Glyphs()
	if g == nil || cat.GlyphsFile() != "glyphs.json" {
		t.Fatalf("Glyphs() = %v, GlyphsFile() = %q", g, cat.GlyphsFile())
	}
	// The Inter Greek file also holds U+0041, but its unicode_range keeps that out, so Latin falls to Geist.
	for r, want := range map[rune]string{0x03C9: "sm-greek", 0x0041: "geist"} {
		i := g.FaceOf(r)
		if i < 0 || g.Faces[i].ID != want {
			t.Errorf("FaceOf(%U) = %d, want face %s", r, i, want)
		}
	}
	for _, r := range []rune{0x2026, 0x2028, 0x200D, 0x0001} {
		if !g.Covered(r) {
			t.Errorf("Covered(%U) = false", r)
		}
	}
	// No shipped face has Thai: U+0E01 is in neither Geist nor the three Noto cmaps, and Inter Greek is limited to Greek.
	if g.Covered(0x0E01) {
		t.Error("Covered(U+0E01) = true")
	}
}

func TestRealCatalogValidatesTheFixtureStories(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '(?i)(glyph|jsspace)' -count=1
```

Expected: the build fails with `undefined: Glyphs`, `undefined: LoadGlyphs`, `undefined: IsJSSpace` and `c.Glyphs undefined`, because none of them exist yet.

- [ ] **Step 4: Implement**

Create `internal/storymaker/narration.go`:

```go
package storymaker

// IsJSSpace reports the white space set of ECMAScript. Narration collapses it to U+0020 and the player's
// glyphs.needsFor skips it, so Go and JS must agree on it exactly.
func IsJSSpace(r rune) bool {
	switch {
	case r >= 0x0009 && r <= 0x000D, r >= 0x2000 && r <= 0x200A:
		return true
	case r == 0x0020, r == 0x00A0, r == 0x1680, r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000, r == 0xFEFF:
		return true
	}
	return false
}
```

Create `internal/storymaker/glyphs.go`:

```go
package storymaker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// GlyphsSchema is the schema string of glyphs.json.
const GlyphsSchema = "storymaker-glyphs/1"

// maxGlyphsBytes caps the coverage file.
const maxGlyphsBytes = 1 << 20

// GlyphFace is one fallback face of glyphs.json.
type GlyphFace struct {
	ID, Family, File string
	Weight           string // FontFace descriptor: "400", "300 700", "100 900"
	UnicodeRange     string // "" means no restriction
	MaxBytes         int64  // 0 means the 8 MiB cap
}

// Glyphs is the decoded coverage file: the faces in fallback order and the code points of each.
// It is immutable after LoadGlyphs. A nil Glyphs covers everything, so the check is skipped.
type Glyphs struct {
	Faces []GlyphFace

	ranges [][][2]rune // per face: inclusive pairs in ascending order
}

// glyphsRef is the manifest key that names the coverage file.
type glyphsRef struct {
	File string `json:"file"`
}

type glyphsDoc struct {
	Schema string `json:"schema"`
	Faces  []struct {
		ID           string  `json:"id"`
		Family       string  `json:"family"`
		File         string  `json:"file"`
		Weight       string  `json:"weight"`
		UnicodeRange string  `json:"unicode_range"`
		MaxBytes     int64   `json:"max_bytes"`
		D            []int64 `json:"d"`
	} `json:"faces"`
}

// LoadGlyphs reads and checks the coverage file at path. The build generates it from the font cmaps,
// so Go needs no font parser.
func LoadGlyphs(fsys fs.FS, path string) (*Glyphs, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("storymaker: open glyphs: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxGlyphsBytes+1))
	if err != nil {
		return nil, fmt.Errorf("storymaker: read glyphs: %w", err)
	}
	if len(data) > maxGlyphsBytes {
		return nil, fmt.Errorf("storymaker: glyphs %s is larger than %d bytes", path, maxGlyphsBytes)
	}
	var doc glyphsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("storymaker: parse glyphs %s: %w", path, err)
	}
	if doc.Schema != GlyphsSchema {
		return nil, fmt.Errorf("storymaker: glyphs %s: schema is %q, want %q", path, doc.Schema, GlyphsSchema)
	}
	g := &Glyphs{}
	for i, face := range doc.Faces {
		if face.ID == "" || face.Family == "" || face.Weight == "" || face.MaxBytes < 0 {
			return nil, fmt.Errorf("storymaker: glyphs %s: faces[%d] needs id, family and weight", path, i)
		}
		if !strings.HasPrefix(face.File, "/fonts/") || strings.Contains(face.File, "..") {
			return nil, fmt.Errorf("storymaker: glyphs %s: face %s file must be a /fonts/ path", path, face.ID)
		}
		ranges, err := decodeGlyphRanges(face.D)
		if err != nil {
			return nil, fmt.Errorf("storymaker: glyphs %s: face %s: %w", path, face.ID, err)
		}
		g.Faces = append(g.Faces, GlyphFace{
			ID: face.ID, Family: face.Family, File: face.File, Weight: face.Weight,
			UnicodeRange: face.UnicodeRange, MaxBytes: face.MaxBytes,
		})
		g.ranges = append(g.ranges, ranges)
	}
	return g, nil
}

// decodeGlyphRanges reads the delta format: the first value is the first start, then
// gap = start - previous end and length = end - start.
func decodeGlyphRanges(d []int64) ([][2]rune, error) {
	if len(d)%2 != 0 {
		return nil, fmt.Errorf("d must hold pairs of values, got %d values", len(d))
	}
	const last = int64(unicode.MaxRune)
	ranges := make([][2]rune, 0, len(d)/2)
	var end int64
	for i := 0; i < len(d); i += 2 {
		gap, length := d[i], d[i+1]
		if gap < 0 || (i > 0 && gap == 0) || length < 0 {
			return nil, fmt.Errorf("ranges are not monotonic at value %d", i)
		}
		if gap > last || length > last || end+gap+length > last {
			return nil, fmt.Errorf("the range at value %d runs past U+10FFFF", i)
		}
		start := end + gap
		end = start + length
		ranges = append(ranges, [2]rune{rune(start), rune(end)})
	}
	return ranges, nil
}

func glyphRangesHave(ranges [][2]rune, r rune) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][1] >= r })
	return i < len(ranges) && ranges[i][0] <= r
}

// FaceOf returns the index of the first face in fallback order that covers r, or -1.
func (g *Glyphs) FaceOf(r rune) int {
	if g == nil {
		return -1
	}
	for i := range g.ranges {
		if glyphRangesHave(g.ranges[i], r) {
			return i
		}
	}
	return -1
}

// Covered reports a character that needs no missing-glyph error: a face has it, it draws nothing
// (default-ignorable, C0 control), or it is JS white space, which the player skips as well.
func (g *Glyphs) Covered(r rune) bool {
	if g == nil {
		return true
	}
	return g.FaceOf(r) >= 0 || IsDefaultIgnorable(r) || IsJSSpace(r) || r < 0x20
}

// Missing lists the characters of the NFC form of text that are not covered, distinct and in order.
func (g *Glyphs) Missing(text string) []rune {
	if g == nil {
		return nil
	}
	var out []rune
	seen := map[rune]bool{}
	for _, r := range norm.NFC.String(text) {
		if !seen[r] && !g.Covered(r) {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// IsDefaultIgnorable reports a Default_Ignorable_Code_Point of the generated Unicode table in ranges_gen.go.
func IsDefaultIgnorable(r rune) bool {
	return inUCDTable(ucdDefaultIgnorable, r)
}

// Glyphs returns the coverage data, or nil when the manifest names no glyphs file.
func (c *Catalog) Glyphs() *Glyphs {
	if c == nil {
		return nil
	}
	return c.glyphs
}

// GlyphsFile returns the coverage file name, relative to the folder of catalog.json, or "".
func (c *Catalog) GlyphsFile() string {
	if c == nil {
		return ""
	}
	return c.glyphsFile
}

// loadGlyphs reads the file the manifest names, from the folder of catalog.json.
func (c *Catalog) loadGlyphs(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
		bad("glyphs needs a relative file path")
		return
	}
	g, err := LoadGlyphs(fsys, path.Join(path.Dir(catalogPath), file))
	if err != nil {
		bad("%v", err)
		return
	}
	c.glyphs, c.glyphsFile = g, file
}
```

In `internal/storymaker/catalog.go`, make four replacements. Inside `LoadCatalog` the parameter `path` hides the `path` package, so the path work sits in `loadGlyphs`.

Old:
```go
	byID    map[string]int
```
New:
```go
	byID    map[string]int

	glyphs     *Glyphs // nil when the manifest names no glyphs file
	glyphsFile string
```

Old:
```go
	Rig      RigRef            `json:"rig"`
```
New:
```go
	Rig      RigRef            `json:"rig"`
	Glyphs   *glyphsRef        `json:"glyphs"`
```

Old:
```go
// entries reference; VerifyFiles does that.
```
New:
```go
// entries reference; VerifyFiles does that. It does read the glyphs file the manifest names, from the same folder.
```

Old:
```go
	if len(problems) > 0 {
```
New:
```go
	if m.Glyphs != nil {
		c.loadGlyphs(fsys, path, m.Glyphs.File, bad)
	}

	if len(problems) > 0 {
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '(?i)(glyph|jsspace)' -count=1
go test ./internal/storymaker/... -count=1
gofmt -l internal/storymaker
go vet ./internal/storymaker/...
```

Expected:
- All tests PASS.
- `gofmt -l` prints nothing, and `go vet` reports nothing.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/glyphs.go internal/storymaker/narration.go internal/storymaker/catalog.go internal/storymaker/glyphs_test.go internal/storymaker/catalog_real_test.go
git commit -m "feat(storymaker): read glyph coverage through the catalog" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Per outline section 5, a PARTIAL answer on new files is accepted with a grep confirmation. Add that as one `-m` paragraph before the trailer. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the same place.

<!-- G-G-2 -->

### Task G4: `glyph_missing` in `Validate`
**Batch:** 2B  **Depends on:** G3
**Files:**
Modify: `internal/storymaker/validate.go`
Modify: `internal/storymaker/model.go`
Test: `internal/storymaker/validate_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout for `Validate` and for `story` (the method on `validator`):

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Validate" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "story" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to `internal/storymaker/validate_test.go`. The file already imports `fmt`, `os`, `strings` and `testing`. `loadReal` is in `internal/storymaker/catalog_real_test.go`. The real catalog carries coverage since G3, and the mini catalog has none.

```go
func TestValidateGlyphMissing(t *testing.T) {
	thai := string(rune(0x0E01))
	zwj := string(rune(0x200D))
	lineSep := string(rune(0x2028))
	show := func(text string) string {
		return `{"id":"s1","beats":[{"do":"show","text":"` + text + `","at":"center","entry":"fade"}]}`
	}
	bareSay := `"actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech"}]`
	cases := []struct {
		name     string
		captions bool
		scene    string
		path     string // "" means no glyph_missing at all
	}{
		{"show text", true, show(thai), "/scenes/0/beats/0/text"},
		{"say text", true, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech","text":"` + thai + `"}]}`, "/scenes/0/beats/0/text"},
		{"zero width joiner is exempt", true, show("a" + zwj + "b"), ""},
		{"line separator is exempt", true, show("a" + lineSep + "b"), ""},
		{"narration with captions on", true, `{"id":"s1","narration":"` + thai + `"}`, "/scenes/0/narration"},
		{"narration with captions off", false, `{"id":"s1","narration":"` + thai + `"}`, ""},
		{"narration with captions off and a say without text", false, `{"id":"s1","speaker":"anna","narration":"` + thai + `",` + bareSay + `}`, "/scenes/0/narration"},
		{"silent narration with captions on", true, `{"id":"s1","silent":true,"narration":"` + thai + `"}`, ""},
		{"silent narration with a say without text", true, `{"id":"s1","silent":true,"narration":"` + thai + `",` + bareSay + `}`, ""},
		{"code scene narration with captions on", true, `{"id":"s1","kind":"code","narration":"` + thai + `"}`, "/scenes/0/narration"},
		{"code scene narration with captions off", false, `{"id":"s1","kind":"code","narration":"` + thai + `"}`, ""},
		{"code scene strings stay unchecked", true, `{"id":"s1","kind":"code","beats":[{"do":"show","text":"` + thai + `","at":"center","entry":"fade"}]}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := storyOf(t, tc.scene)
			s.Style.Captions = tc.captions
			var found []Issue
			for _, is := range Validate(&s, loadReal(t), DefaultLimits()) {
				if is.Code == CodeGlyphMissing {
					found = append(found, is)
				}
			}
			if tc.path == "" {
				if len(found) != 0 {
					t.Fatalf("unexpected glyph_missing:%s", dump(found))
				}
				return
			}
			if len(found) != 1 || found[0].Path != tc.path || found[0].Severity != SeverityError {
				t.Fatalf("want one error at %s, got:%s", tc.path, dump(found))
			}
			if !strings.Contains(found[0].Message, "U+0E01") {
				t.Fatalf("message does not name the character: %s", found[0].Message)
			}
		})
	}
	t.Run("names at most three characters", func(t *testing.T) {
		// Four distinct Thai letters and one repeat, so the fourth stays unnamed.
		s := storyOf(t, show(string([]rune{0x0E01, 0x0E02, 0x0E03, 0x0E04, 0x0E01})))
		is := expectIssue(t, Validate(&s, loadReal(t), DefaultLimits()), CodeGlyphMissing, SeverityError, "/scenes/0/beats/0/text")
		if !strings.Contains(is.Message, fmt.Sprintf("U+0E01 %q", thai)) || !strings.Contains(is.Message, "U+0E03") || strings.Contains(is.Message, "U+0E04") {
			t.Fatalf("message: %s", is.Message)
		}
	})
	t.Run("the mini catalog has no coverage, so nothing is checked", func(t *testing.T) {
		for _, is := range validateScenes(t, show(thai)) {
			if is.Code == CodeGlyphMissing {
				t.Fatalf("unexpected: %+v", is)
			}
		}
	})
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^TestValidateGlyphMissing$' -count=1
```

Expected: the build fails with `undefined: CodeGlyphMissing`, because the code does not exist yet.

- [ ] **Step 4: Implement**

**`internal/storymaker/model.go`.** The issue codes of 2A sit in their own const blocks: `CodeCueLate` from M9 and the lint codes from M14, whose block ends directly above this line. This code also gets its own declaration.

Old text:

```go
// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```

New text:

```go
// CodeGlyphMissing is an error of Validate: a drawn text has a character that no font of the catalog covers.
const CodeGlyphMissing = "glyph_missing"

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```

**`internal/storymaker/validate.go`, first edit.**

Old text:

```go
	v.scenes()
```

New text:

```go
	v.scenes()
	v.glyphs()
```

**`internal/storymaker/validate.go`, second edit.**

Old text:

```go
func (v *validator) format() {
```

New text:

```go
// glyphs reports text that no fallback face can draw, one glyph_missing per text path. A catalog without coverage data skips
// the check. Code-scene strings stay layout_unverified, so a code scene answers only for its narration. A silent scene's
// narration is never drawn, so it is never checked.
func (v *validator) glyphs() {
	if v.cat == nil || v.cat.Glyphs() == nil {
		return
	}
	g := v.cat.Glyphs()
	check := func(path, text string) {
		missing := g.Missing(text)
		if len(missing) == 0 {
			return
		}
		v.errorf(CodeGlyphMissing, path, "No font covers these characters: "+glyphNames(missing)+".",
			"Remove or replace them. Emoji, Thai and Hebrew have no font in this catalog.")
	}
	for i := range v.s.Scenes {
		sc := &v.s.Scenes[i]
		v.sceneID = sc.ID
		storyboard := sc.Kind != "code"
		bareSay := false
		if storyboard {
			for j := range sc.Beats {
				if sc.Beats[j].Do == "say" && !sayHasText(sc.Beats[j].Text) {
					bareSay = true
				}
			}
		}
		if !sc.Silent && (v.s.Style.Captions || bareSay) {
			check(jp("scenes", i)+"/narration", sc.Narration)
		}
		if !storyboard {
			continue
		}
		for j := range sc.Beats {
			if b := &sc.Beats[j]; b.Do == "show" || b.Do == "say" {
				check(jp("scenes", i, "beats", j)+"/text", b.Text)
			}
		}
	}
	v.sceneID = ""
}

// glyphNames names at most three runes as U+XXXX "x".
func glyphNames(runes []rune) string {
	names := make([]string, 0, 3)
	for _, r := range runes {
		if len(names) == 3 {
			break
		}
		names = append(names, fmt.Sprintf("U+%04X %q", r, string(r)))
	}
	if extra := len(runes) - len(names); extra > 0 {
		return strings.Join(names, ", ") + fmt.Sprintf(" and %d more", extra)
	}
	return strings.Join(names, ", ")
}

// sayMarkerPattern is the marker form of the player's narration parser: any count of ASCII digits, so wider than cuePattern.
var sayMarkerPattern = regexp.MustCompile(`\{[0-9]+\}`)

// sayHasText is true when a say's text keeps a character after its markers and JS whitespace go. It is the same test as Blocks
// (StripMarkers(text) is not empty) and the player (parsed.words > 0), so a say of only spaces counts as a say without text.
func sayHasText(text string) bool {
	return strings.TrimFunc(sayMarkerPattern.ReplaceAllString(text, ""), IsJSSpace) != ""
}

func (v *validator) format() {
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/... -count=1
gofmt -l internal/storymaker
```

Expected: every package passes and `gofmt -l` prints nothing. `TestRealCatalogValidatesTheFixtureStories` stays clean, because the fixtures use Latin text that the faces cover.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

```powershell
git add internal/storymaker/validate.go internal/storymaker/model.go internal/storymaker/validate_test.go
git commit -m "feat(storymaker): glyph_missing in Validate" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task G5: Say and speaker validation
**Batch:** 2B  **Depends on:** G4
**Files:**
Modify: `internal/storymaker/validate.go`
Modify: `internal/storymaker/model.go`
Test: `internal/storymaker/validate_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout for `speaker`, `beat` (both methods on `validator`) and `phase1Beats`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "speaker" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "beat" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "phase1Beats" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

**`internal/storymaker/validate_test.go`, first edit.** Delete the two rows that pin the old support matrix in `TestValidateNames`.

Old text:

```go
		{"say beat", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech","text":"Hi"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
```

New: nothing.

Old text:

```go
		{"cast speaker", `{"id":"s1","speaker":"anna"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/speaker"},
```

New: nothing.

**`internal/storymaker/validate_test.go`, second edit.** Append:

```go
func TestValidateSayAndSpeaker(t *testing.T) {
	const bareSay = `{"do":"say","who":"anna","style":"speech"}`
	stage := func(extra, beats string) string {
		return `{"id":"s1",` + extra + `"actors":[{"cast":"anna"}],"beats":[` + beats + `]}`
	}

	// A say without text shows the narration of the scene speaker, so its who must be that speaker.
	expectIssue(t, validateScenes(t, stage("", bareSay)), CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who")
	markerOnly := `{"do":"say","who":"anna","style":"speech","text":"{1} "}`
	expectIssue(t, validateScenes(t, stage(`"speaker":"narrator",`, markerOnly)), CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who")
	if issues := validateScenes(t, stage(`"speaker":"anna",`, bareSay)); len(issues) != 0 {
		t.Errorf("a say by the speaker:%s", dump(issues))
	}

	// In a silent scene it has no narration to show: a warning, never an error.
	issues := validateScenes(t, stage(`"silent":true,`, bareSay))
	expectIssue(t, issues, CodeInvalidValue, SeverityWarning, "/scenes/0/beats/0/text")
	if is := findIssue(issues, CodeInvalidValue, SeverityError, "/scenes/0/beats/0/who"); is != nil {
		t.Errorf("a silent scene must not block on who: %+v", is)
	}

	// The say text has the narration limit: DefaultLimits().MaxNarrationChars is 1500, so 1501 is one too many.
	sayOf := func(n int) string {
		return `{"do":"say","who":"anna","style":"speech","text":"` + strings.Repeat("a", n) + `"}`
	}
	expectIssue(t, validateScenes(t, stage("", sayOf(1501))), CodeLimitExceeded, SeverityError, "/scenes/0/beats/0/text")
	if issues := validateScenes(t, stage("", sayOf(1500))); len(issues) != 0 {
		t.Errorf("a say at the limit:%s", dump(issues))
	}

	// Off stage the speech still plays. Every other beat is still ignored.
	off := expectIssue(t, validateScenes(t, `{"id":"s1","beats":[{"do":"say","who":"anna","style":"speech","text":"Hi"}]}`), CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who")
	if !strings.Contains(off.Message, "still plays") {
		t.Errorf("off-stage say: %s", off.Message)
	}
	pose := expectIssue(t, validateScenes(t, `{"id":"s1","beats":[{"do":"pose","who":"anna","pose":"stand"}]}`), CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who")
	if !strings.Contains(pose.Message, "ignored") {
		t.Errorf("off-stage pose: %s", pose.Message)
	}

	// A cast speaker and a say are played now, so neither is not_supported_yet.
	for _, scene := range []string{`{"id":"s1","speaker":"anna"}`, stage("", sayOf(2))} {
		for _, is := range validateScenes(t, scene) {
			if is.Code == CodeNotSupportedYet {
				t.Errorf("%s: %+v", scene, is)
			}
		}
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^TestValidateSayAndSpeaker$' -count=1
```

Expected: FAIL with `no error invalid_value at /scenes/0/beats/0/who`, because `validator.beat` does not check a say's text yet.

- [ ] **Step 4: Implement**

**`internal/storymaker/model.go`.** M12 leaves the list as `nameSet("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "highlight", "camera", "wait")`. The edit touches only the start of the line.

Old text:

```go
	phase1Beats       = nameSet(
```

New text:

```go
	phase1Beats       = nameSet("say", 
```

The list follows on the same line, so `gofmt` sees `nameSet("say", "enter", ...`. Keep exactly one space after the comma of `"say",`.

**`internal/storymaker/validate.go`, `speaker`.**

Old text:

```go
		if _, ok := v.cast[sc.Speaker]; !ok {
			v.errorf(CodeUnknownCast, base+"/speaker", fmt.Sprintf("The speaker %q is not in the cast.", clip(sc.Speaker, 40)),
				v.castFix("speaker", sc.Speaker))
			return
		}
		v.notYet(base+"/speaker", "A cast speaker (lip-sync and speech bubble) is accepted but not shown yet; the scene plays like a narrator scene.")
```

New text:

```go
		if _, ok := v.cast[sc.Speaker]; !ok {
			v.errorf(CodeUnknownCast, base+"/speaker", fmt.Sprintf("The speaker %q is not in the cast.", clip(sc.Speaker, 40)),
				v.castFix("speaker", sc.Speaker))
		}
```

**`internal/storymaker/validate.go`, `needWho`.**

Old text:

```go
			v.warnf(CodeUnknownCast, bb+"/who", fmt.Sprintf("%q is not on stage here; the beat is ignored.", b.Who),
				`Add the actor to "actors" or use an enter beat before this one.`)
```

New text:

```go
			msg := fmt.Sprintf("%q is not on stage here; the beat is ignored.", b.Who)
			if b.Do == "say" {
				msg = fmt.Sprintf("%q is not on stage here; the speech still plays, with no bubble and no lip-sync.", b.Who)
			}
			v.warnf(CodeUnknownCast, bb+"/who", msg, `Add the actor to "actors" or use an enter beat before this one.`)
```

**`internal/storymaker/validate.go`, the `say` case.** `case "show":` follows the closing brace of the style switch of `say`, so the new call runs after that switch.

Old text:

```go
	case "show":
```

New text:

```go
		v.sayText(bb, b, sc)
	case "show":
```

**`internal/storymaker/validate.go`, the new method.** `sayHasText` is from G4.

Old text:

```go
func (v *validator) cueFix(refs *sceneRefs, got int) string {
```

New text:

```go
// sayText checks the text of a say beat. A say without text shows the caption lines of the scene narration, so its who must
// be the scene speaker. A silent scene has no narration to show, so no bubble is drawn. A say with text has the narration
// limit, which also bounds its blocks and pieces.
func (v *validator) sayText(bb string, b *Beat, sc *Scene) {
	if n := runeLen(b.Text); n > v.lim.MaxNarrationChars {
		v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The say text has %d characters; the limit is %d.", n, v.lim.MaxNarrationChars),
			"Shorten the text or split it into two say beats.")
		return
	}
	if sayHasText(b.Text) {
		return
	}
	if sc.Silent {
		v.warnf(CodeInvalidValue, bb+"/text", "A say without text shows the scene's narration, but this scene is silent, so no bubble is drawn.",
			`Add "text", or clear "silent".`)
		return
	}
	speaker := sc.Speaker
	if speaker == "" {
		speaker = "narrator"
	}
	if b.Who != "" && b.Who != speaker {
		v.errorf(CodeInvalidValue, bb+"/who", fmt.Sprintf("A say without text shows the narration of the speaker %q, so \"who\" must be that speaker, not %q.", speaker, clip(b.Who, 40)),
			fmt.Sprintf(`Add "text", or set the scene's "speaker" to %q.`, clip(b.Who, 40)))
	}
}

func (v *validator) cueFix(refs *sceneRefs, got int) string {
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/... -count=1
gofmt -l internal/storymaker
```

Expected: every package passes and `gofmt -l` prints nothing. `TestValidateBeatRequiredFields` and `TestValidateEnumsAndRanges` still pass, because they filter by code and path.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

```powershell
git add internal/storymaker/validate.go internal/storymaker/model.go internal/storymaker/validate_test.go
git commit -m "feat(storymaker): say and speaker validation" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task G6: `glyphs.js` and text flow (D26)
**Batch:** 2B  **Depends on:** M3, G1
**Files:**
Create: `ui/js/storymaker/glyphs.js`
Modify: `ui/js/storymaker/text.js`
Modify: `ui/js/storymaker/codescene.js`
Modify: `scripts/storymaker-bundle-parts.js`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/15-glyphs.mjs` (create)
Test: `scripts/storymaker-tests/02-stage-parts-text.mjs`
Test: `scripts/storymaker-tests/06-codescene.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout for `fontString`, `fitEllipsis`, `breakWord`, `wrap`, `balance`, `layout`, `draw`, `storymakerPlayerParts` and `facade` (in `ui/js/storymaker/codescene.js`), one command per symbol. `balance` is new since M13, so the stale index may not know it. Confirm it with a text search.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "fontString" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "fitEllipsis" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "breakWord" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "wrap" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "balance" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "layout" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "draw" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerPlayerParts" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "facade" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Create `scripts/storymaker-tests/15-glyphs.mjs`:

```js
// Task G6: SM.glyphs, the coverage tables in JS, against the real ui/img/storymaker/glyphs.json.
import assert from 'node:assert/strict';
import { plain, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const data = readJSON('ui/img/storymaker/glyphs.json');
const ids = faces => plain(faces.map(f => f.id));

check('glyphs: a code point goes to the first covering face, and Greek comes from the Greek face, never from Geist', () => {
    const G = t.core().glyphs, idx = G.index(data);
    const faceId = cp => { const i = G.faceOf(idx, cp); return i < 0 ? null : idx.faces[i].id; };
    assert.equal(faceId(0x03C9), 'sm-greek');
    assert.equal(faceId(0x1F00), 'sm-greek-ext');
    assert.equal(faceId(0x0041), 'geist');
    assert.equal(faceId(0x0915), 'sm-deva');
    assert.equal(faceId(0x0627), 'sm-arab');
    assert.equal(faceId(0x6F22), 'sm-cjk');
    assert.equal(faceId(0x0E01), null, 'Thai has no face');
    assert.equal(G.index(null), null);
    assert.equal(G.faceOf(null, 0x41), -1);
});

check('glyphs: needsFor skips marks, spaces and ignorables, and lists each face once', () => {
    const G = t.core().glyphs, idx = G.index(data);
    const cp = (...codes) => String.fromCodePoint(...codes);
    assert.deepEqual(ids(G.needsFor(idx, 'A' + cp(0x03C9) + 'B' + cp(0x03C9))), ['geist', 'sm-greek']);
    assert.deepEqual(ids(G.needsFor(idx, cp(0x0301, 0x200D, 0x3000, 0x2028, 0x0020))), []);
    assert.deepEqual(ids(G.needsFor(idx, cp(0x0E01))), [], 'a character without a face is the job of Validate, not a need');
});

check('glyphs: the family chain, the faces of a language and the key', () => {
    const G = t.core().glyphs, idx = G.index(data);
    assert.deepEqual(plain(G.chain(idx, '$style')), ['SMGreek', '$style', 'SMDeva', 'SMArab', 'SMCJK']);
    for (const lang of ['el', 'EL_gr']) assert.deepEqual(ids(G.facesForLanguage(idx, lang)), ['sm-greek', 'sm-greek-ext'], lang);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'hi-IN')), ['sm-deva']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'ar')), ['sm-arab']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'zh-CN')), ['sm-cjk']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'ja')), ['sm-cjk']);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'de')), []);
    assert.deepEqual(ids(G.facesForLanguage(idx, 'constructor')), [], 'any language string is safe');
    assert.equal(G.key({ id: 'sm-cjk' }), 'font:sm-cjk/400');
});
```

Append to `scripts/storymaker-tests/02-stage-parts-text.mjs`. `assert`, `plain`, `makeContext`, `t`, `check` and `SM` are already in that file.

```js
check('text flow (D26): family lists, code points, kinsoku, CJK wrap, trim set, direction before measuring', () => {
    const cp = (...codes) => String.fromCodePoint(...codes);
    const ell = cp(0x2026), perPoint = s => [...s].length * 10;   // 10 px per code point

    assert.equal(SM.text.fontString(700, 76, ['SMGreek', 'Geist', 'SMCJK']), '700 76px "SMGreek", "Geist", "SMCJK", sans-serif');
    assert.equal(SM.text.fontString(700, 76, 'Storymaker Default'), '700 76px "Storymaker Default", sans-serif');

    // 25 px fits two code points, so five emoji give 2, 2, 1. An emoji is two UTF-16 units and never splits.
    const pair = cp(0x1F600);
    assert.deepEqual(plain(SM.text.breakWord(perPoint, pair.repeat(5), 25)), [pair + pair, pair + pair, pair]);

    // 15 px fits one code point, but ka, virama and ssa stay one cluster: no break before a mark or after U+094D.
    const cluster = cp(0x0915, 0x094D, 0x0937), ka = cp(0x0915);
    assert.deepEqual(plain(SM.text.breakWord(perPoint, cluster + ka, 15)), [cluster, ka]);

    // 40 px fits four. U+3002 may not start a line, so the character before it moves down with it.
    const han = cp(0x6F22), zi = cp(0x5B57), dot = cp(0x3002), open = cp(0x300C);
    assert.deepEqual(plain(SM.text.wrap(perPoint, han + zi + han + zi + dot + han + zi, 40, 5).lines), [han + zi + han, zi + dot + han + zi]);
    // U+300C may not end a line, so it moves down with the character after it.
    assert.deepEqual(plain(SM.text.wrap(perPoint, han + zi + han + open + zi, 40, 5).lines), [han + zi + han, open + zi]);
    // balance adds no space where a CJK line break had none. 40 px is the narrowest width that keeps two lines, so 5 + 3 becomes 4 + 4.
    assert.deepEqual(plain(SM.text.balance(perPoint, [han + zi + han + zi + han, zi + han + zi], 50, 2)), [han + zi + han + zi, han + zi + han + zi]);

    // 25 px fits five units. The first line is aaa, U+FF0C, b. The cut reaches the comma, and the trim set drops it.
    assert.equal(SM.text.wrap(s => s.length * 5, 'aaa' + cp(0xFF0C) + 'bbbbbb cc', 25, 1).lines[0], 'aaa' + ell);
    // Three lines of two emoji, one kept. 25 px fits one emoji and the ellipsis, and the cut drops whole pairs.
    assert.equal(SM.text.wrap(perPoint, pair.repeat(6), 25, 1).lines[0], pair + ell);

    const seen = [];
    const ctx = makeContext(null, { measure: s => { seen.push(ctx.direction); return String(s).length * 10; } });
    const style = t.byId('text.title'), opts = { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' };
    const arabic = SM.text.layout(ctx, cp(0x0645, 0x0631, 0x062D, 0x0628, 0x0627), style, opts);
    assert.equal(arabic.dir, 'rtl');
    assert.ok(seen.length > 0 && seen.every(d => d === 'rtl'), 'the direction is set before the first measure');
    seen.length = 0;
    const latin = SM.text.layout(ctx, 'Hello', style, opts);
    assert.equal(latin.dir, 'ltr');
    assert.ok(seen.length > 0 && seen.every(d => d === 'ltr'), 'the previous text leaves no direction behind');
    ctx.log.length = 0;
    SM.text.draw(ctx, arabic, style, 100, 100, () => '#000000', 1);
    assert.ok(ctx.log.includes('set direction=rtl'), 'draw sets the direction of its layout');
});
```

Append to `scripts/storymaker-tests/06-codescene.mjs`. `assert`, `runScene`, `FakeCanvas`, `catalog`, `files` and `check` are already in that file.

```js
check('code scene frames (D26): each g.text sets the direction of its own text', () => {
    const arabic = String.fromCodePoint(0x0645, 0x0631, 0x062D, 0x0628, 0x0627) + '!';
    const { C, build } = runScene(`export default function scene(api) {
      api.duration(1);
      api.onFrame((t, g) => { g.text(${JSON.stringify(arabic)}, 100, 100); g.text('Hello!', 100, 200); });
    }`, 'dirs');
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', accent: '#f00', text: '#000' }, cat: C.scene.indexCatalog(catalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    const rc = C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
    C.codescene.drawFrame(build(), 0, cv.ctx, rc);
    const drawn = cv.ctx.log.filter(l => l.startsWith('set direction=') || l.startsWith('fillText('));
    assert.deepEqual(drawn, ['set direction=rtl', 'fillText(' + arabic + ',100,100)', 'set direction=ltr', 'fillText(Hello!,100,200)']);
});
```

- [ ] **Step 3: Run it**

```powershell
node scripts/test-storymaker-player.mjs
```

Expected: exit 1 with five new `FAIL` lines. The three `glyphs:` checks fail because `t.core().glyphs` is undefined. The `text flow (D26)` check fails on the first `fontString` assertion, because an array is not a family list yet. The `code scene frames (D26)` check fails because `g.text` logs no `set direction=` line yet.

- [ ] **Step 4: Implement**

Create `ui/js/storymaker/glyphs.js`:

```js
// Storymaker player: glyph coverage in JS, read from the catalog's coverage file (catalog.glyph_data, schema
// storymaker-glyphs/1). It answers which fallback face draws a code point, which faces a text needs, the family chain for
// ctx.font and the faces a code scene gets for its language. Only the tables of SM.ranges decide, never ICU. Classic script;
// publishes StorymakerCore.glyphs.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var LANGUAGE_FACES = { el: ['sm-greek', 'sm-greek-ext'], hi: ['sm-deva'], ar: ['sm-arab'], zh: ['sm-cjk'], ja: ['sm-cjk'] };

    // decode(d): d is [start0, len0, gap1, len1, ...] with gap = start - previous end and len = end - start. The result is the
    // flat list [start0, end0, start1, end1, ...]. An end is inclusive.
    function decode(d) {
        var out = [], prev = 0;
        for (var i = 0; i + 1 < d.length; i += 2) {
            var start = prev + d[i], end = start + d[i + 1];
            out.push(start, end);
            prev = end;
        }
        return out;
    }

    function covers(ranges, cp) {
        var lo = 0, hi = ranges.length / 2 - 1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (cp < ranges[mid * 2]) hi = mid - 1;
            else if (cp > ranges[mid * 2 + 1]) lo = mid + 1;
            else return true;
        }
        return false;
    }

    // index(glyphData) = {faces, ranges}, in fallback order, or null without glyph data.
    function index(glyphData) {
        if (!glyphData || !Array.isArray(glyphData.faces)) return null;
        return {
            faces: glyphData.faces,
            ranges: glyphData.faces.map(function (face) { return decode(face && Array.isArray(face.d) ? face.d : []); })
        };
    }

    // faceOf(idx, cp) = the index of the first covering face, -1 when none.
    function faceOf(idx, cp) {
        if (!idx) return -1;
        for (var i = 0; i < idx.ranges.length; i++) if (covers(idx.ranges[i], cp)) return i;
        return -1;
    }

    // needsFor(idx, text) = the faces a text draws from, each once. Marks add no need, because compile may not normalize.
    function needsFor(idx, text) {
        var out = [], R = SM.ranges, s = String(text === undefined || text === null ? '' : text);
        if (!idx) return out;
        for (var i = 0; i < s.length;) {
            var cp = s.codePointAt(i);
            i += cp > 0xFFFF ? 2 : 1;
            if (R.isDefaultIgnorable(cp) || R.isMark(cp) || R.isSpace(cp)) continue;
            var at = faceOf(idx, cp);
            if (at >= 0 && out.indexOf(idx.faces[at]) < 0) out.push(idx.faces[at]);
        }
        return out;
    }

    // chain(idx, styleFamily) = the family list for ctx.font, with the style family in the place of Geist.
    function chain(idx, styleFamily) {
        var out = [];
        if (!idx) return out;
        idx.faces.forEach(function (face) {
            var family = face.family === 'Geist' ? styleFamily : face.family;
            if (out.indexOf(family) < 0) out.push(family);
        });
        return out;
    }

    // facesForLanguage(idx, lang): the same base rule as Go's LanguageBase, because the validator accepts any language string.
    function facesForLanguage(idx, lang) {
        if (!idx) return [];
        var base = String(lang === undefined || lang === null ? '' : lang).toLowerCase().split(/[-_]/)[0];
        var wanted = Object.prototype.hasOwnProperty.call(LANGUAGE_FACES, base) ? LANGUAGE_FACES[base] : [];
        return idx.faces.filter(function (face) { return wanted.indexOf(face.id) >= 0; });
    }

    function key(face) { return 'font:' + face.id + '/400'; }

    SM.glyphs = { index: index, faceOf: faceOf, needsFor: needsFor, chain: chain, facesForLanguage: facesForLanguage, key: key };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

**`scripts/storymaker-bundle-parts.js`.** This file has CRLF line endings in the checkout, so the old text is one line. It adds the part in the place frozen section 4.1 gives it.

Old text:

```js
  'ui/js/storymaker/text.js',
```

New text:

```js
  'ui/js/storymaker/glyphs.js',
  'ui/js/storymaker/text.js',
```

**`ui/js/storymaker/text.js`.** This file has CRLF line endings in the checkout, so each old text below is one line. M13 added `balance`, the shrink loop and the memo. It leaves these lines as they are quoted here.

Edit 1. Old text:

```js
    var LINE_HEIGHT = 1.22;
```

New text:

```js
    var LINE_HEIGHT = 1.22;
    // Trimmed before an ellipsis, besides ASCII punctuation: Arabic and CJK punctuation and the ellipsis itself.
    var TRIM_CODES = [0x060C, 0x061B, 0x061F, 0x06D4, 0x2026, 0x3001, 0x3002, 0xFF01, 0xFF0C, 0xFF1A, 0xFF1B, 0xFF1F];
```

Edit 2. Old text:

```js
        return String(weight || 400) + ' ' + px + 'px "' + String(family || 'sans-serif').replace(/"/g, '') + '", sans-serif';
```

New text:

```js
        // family is one name or a fallback list. Each name is quoted, and sans-serif comes last.
        var names = (Array.isArray(family) ? family : [family]).filter(Boolean);
        if (!names.length) names = ['sans-serif'];
        var list = names.map(function (name) { return '"' + String(name).replace(/"/g, '') + '"'; }).join(', ');
        return String(weight || 400) + ' ' + px + 'px ' + list + ', sans-serif';
```

Edit 3. Old text:

```js
    function fitEllipsis(measure, line, maxWidth) {
```

New text:

```js
    function trimEnd(line) {
        var end = line.length;
        while (end > 0 && (TRIM_CODES.indexOf(line.charCodeAt(end - 1)) >= 0 || /[\s.,;:!?-]/.test(line.charAt(end - 1)))) end--;
        return line.slice(0, end);
    }

    // dropLast(text): text without its last code point, so a surrogate pair never splits.
    function dropLast(text) {
        var cut = text.length - 1, low = text.charCodeAt(cut), high = text.charCodeAt(cut - 1);
        if (low >= 0xDC00 && low <= 0xDFFF && high >= 0xD800 && high <= 0xDBFF) cut--;
        return text.slice(0, cut);
    }

    function fitEllipsis(measure, line, maxWidth) {
```

Edit 4. Old text:

```js
        var text = line.replace(/[\s.,;:!?-]+$/, '');
```

New text:

```js
        var text = trimEnd(line);
```

Edit 5. `wrap` always passes a line wider than `maxWidth`, so the trim must also run after each cut, or the new trim set has no effect.

Old text:

```js
        while (text.length > 0 && measure(text + ELLIPSIS) > maxWidth) text = text.slice(0, -1);
```

New text:

```js
        while (text.length > 0 && measure(text + ELLIPSIS) > maxWidth) text = trimEnd(dropLast(text));
```

Edit 6. Old text:

```js
    // breakWord(measure, word, maxWidth): splits a word that is wider than a line into pieces that fit.
```

New text:

```js
    // breakWord(measure, word, maxWidth): splits a word that is wider than a line into pieces that fit. It iterates code points
    // and breaks only where SM.ranges.canBreakAfter allows, at the last allowed point before the line would overflow.
```

Edit 7. Old text:

```js
        var pieces = [], current = '';
```

New text:

```js
        var pieces = [], current = '', breakAt = 0, prev = -1;
```

Edit 8. Old text:

```js
        for (var i = 0; i < word.length; i++) {
```

New text:

```js
        for (var i = 0; i < word.length;) {
```

Edit 9. Old text:

```js
            if (current && measure(current + word[i]) > maxWidth) { pieces.push(current); current = word[i]; } else current += word[i];
```

New text:

```js
            var cp = word.codePointAt(i), ch = String.fromCodePoint(cp);
            i += ch.length;
            if (current && SM.ranges.canBreakAfter(prev, cp)) breakAt = current.length;
            if (breakAt > 0 && measure(current + ch) > maxWidth) {
                pieces.push(current.slice(0, breakAt));
                current = current.slice(breakAt);
                breakAt = 0;
            }
            current += ch;
            prev = cp;
```

Edit 10. Old text:

```js
    // wrap(measure, text, maxWidth, maxLines) = {lines, truncated}. measure(str) returns the width at the target font.
```

New text:

```js
    // cjkParts(word): a token split at every SM.ranges.cjkBreak. A token without CJK stays whole.
    function cjkParts(word) {
        var parts = [], current = '', prev = -1;
        for (var i = 0; i < word.length;) {
            var cp = word.codePointAt(i), ch = String.fromCodePoint(cp);
            i += ch.length;
            if (current && SM.ranges.cjkBreak(prev, cp)) { parts.push(current); current = ''; }
            current += ch;
            prev = cp;
        }
        if (current) parts.push(current);
        return parts;
    }

    // wrap(measure, text, maxWidth, maxLines) = {lines, truncated}. measure(str) returns the width at the target font.
```

Edit 11. Old text:

```js
                var parts = measure(word) > maxWidth ? breakWord(measure, word, maxWidth) : [word];
```

New text:

```js
                var parts = [];
                cjkParts(word).forEach(function (piece) {
                    (measure(piece) > maxWidth ? breakWord(measure, piece, maxWidth) : [piece]).forEach(function (p) { parts.push(p); });
                });
```

Edit 12. `layoutDir` wraps `layout`, so the direction is set ahead of every measure, on a memo hit or miss.

Old text:

```js
    function roundRect(ctx, x, y, w, h, r) {
```

New text:

```js
    // layoutDir(ctx, text, style, opts): layout with the direction of the text, from its first strong character. Setting it first
    // means canvas state never leaks from one text to the next, so workers cannot diverge. style.align stays physical for RTL.
    function layoutDir(ctx, text, style, opts) {
        var dir = SM.ranges.dirOf(String(text === undefined || text === null ? '' : text));
        ctx.direction = dir;
        var lay = layout(ctx, text, style, opts);
        lay.dir = dir;
        return lay;
    }

    function roundRect(ctx, x, y, w, h, r) {
```

Edit 13. Old text:

```js
        if (!(alpha > 0)) return;
```

New text:

```js
        if (!(alpha > 0)) return;
        ctx.direction = lay.dir || 'ltr';
```

Edit 14. This publishes the wrapper, and `breakWord` for the test. The old text is part of the `SM.text = {` line, so match only this part. M13 added `balance` elsewhere on that line.

Old text:

```js
layout: layout,
```

New text:

```js
breakWord: breakWord, layout: layoutDir,
```

Edit 15. M13's `balance` re-joins its lines with a space. `wrap` now breaks between CJK characters, so that join would add a space where the break had none.

Old text:

```js
    // balance(measure, lines, maxWidth, maxLines): the same text, re-wrapped at the narrowest width (by bisection) that does not
```

New text:

```js
    // rejoin(lines): the lines as one text, with a space only where the break replaced one. A break at SM.ranges.cjkBreak had none.
    // The lines cannot show a real space between two CJK characters at a break, so that space goes.
    function rejoin(lines) {
        return lines.reduce(function (text, line) {
            var last = text.slice(dropLast(text).length).codePointAt(0);
            return text + (SM.ranges.cjkBreak(last, line.codePointAt(0)) ? '' : ' ') + line;
        });
    }

    // balance(measure, lines, maxWidth, maxLines): the same text, re-wrapped at the narrowest width (by bisection) that does not
```

Edit 16. Old text:

```js
        var joined = lines.join(' '), best = lines, hi = maxWidth, lo = 0;
```

New text:

```js
        var joined = rejoin(lines), best = lines, hi = maxWidth, lo = 0;
```

Edit 17. A CJK text has no spaces left after Edit 16, so splitting at spaces alone would see one long word, and the floor of the bisection would pass `maxWidth`. The floor is now the widest part that `wrap` keeps whole. A Latin word is one part, as before.

Old text:

```js
        joined.split(' ').forEach(function (word) { var w = measure(word); if (w > lo) lo = w; });
```

New text:

```js
        joined.split(' ').forEach(function (word) {
            cjkParts(word).forEach(function (part) { var w = measure(part); if (w > lo) lo = w; });
        });
```

**`ui/js/storymaker/codescene.js`.** This file has CRLF line endings in the checkout, so the old text is one line. Without this edit, `g.text` draws with whatever direction the last text left on the canvas, so a scrubbed preview frame can differ from the same frame in a sequential export. The `ctx.save()` and `ctx.restore()` around `onFrame` already keep the value from leaking past the hook.

Old text:

```js
                ctx.fillText(String(str), num(x, 'x'), num(y, 'y'));
```

New text:

```js
                // Set per call, so the direction never depends on what an earlier draw left on the canvas.
                ctx.direction = SM.ranges.dirOf(String(str));
                ctx.fillText(String(str), num(x, 'x'), num(y, 'y'));
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and `test:storymaker` ends with `0 failed`. The pins at `scripts/storymaker-tests/02-stage-parts-text.mjs:83` and `:89` and `scripts/storymaker-tests/11-real-catalog.mjs:317` still hold, because a single family keeps its old form and a Latin word without a mark keeps its old breaks.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

A PARTIAL answer on the new files needs a grep confirmation. Put it in an extra `-m` ahead of the trailer.

```powershell
git add ui/js/storymaker/glyphs.js ui/js/storymaker/text.js ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/15-glyphs.mjs scripts/storymaker-tests/02-stage-parts-text.mjs scripts/storymaker-tests/06-codescene.mjs
git commit -m "feat(storymaker): glyph coverage in the player and text flow for CJK, Devanagari and RTL" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task G7: Fallback faces in host and app
**Batch:** 2B  **Depends on:** G1
**Files:**
Modify: `ui/js/storymaker/host-core.js`
Modify: `ui/js/storymaker/worker-player.js`
Modify: `ui/js/storymaker/worker-scene.js`
Modify: `ui/js/storymaker/host-app.js`
Modify: `ui/js/desktop/apps/storymaker-assets.js`
Modify: `ui/js/desktop/apps/storymaker-api.js`
Modify: `ui/js/storymaker/host.bundle.js` (generated by `npm run build:ui`)
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/07-workers.mjs`
Test: `scripts/storymaker-tests/08-host-app.mjs`
Test: `ui/desktop_storymaker_api_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout for `checkMessage`, `mountApp`, `fontDescriptor`, `handle` (both `Runtime` types), `createAssets` and `createApi`, one command per symbol:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "checkMessage" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "mountApp" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "fontDescriptor" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handle" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createAssets" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createApi" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to `scripts/storymaker-tests/07-workers.mjs`. `client`, `font`, `s3Bundle`, `plain`, `InProcessWorker`, `until`, `assert` and `check` are already in that file.

```js
check('workers: FontFace gets the weight range and the unicode range, and an empty range is omitted', async () => {
    const greek = { name: 'sm-greek', family: 'SMGreek', weight: 400, weight_range: '300 700', unicode_range: 'U+0370-03FF', data: new ArrayBuffer(8) };
    const cjk = { name: 'sm-cjk', family: 'SMCJK', weight: 400, weight_range: '400', unicode_range: '', data: new ArrayBuffer(8) };
    for (const role of ['player', 'scene']) {
        const w = new InProcessWorker(role, role === 'scene' ? s3Bundle : ''), c = client(w);
        await until(() => c.hello === role, 'hello');
        const reply = await c.call({ type: 'assets', assets: {}, fonts: [greek, cjk, font] });
        assert.equal(reply.ok, true, role);
        assert.deepEqual(plain(w.context.fonts.faces.map(f => f.desc)), [
            { weight: '300 700', unicodeRange: 'U+0370-03FF' }, { weight: '400' }, { weight: '400' }
        ], role);
        w.terminate();
    }
});
```

Append to `scripts/storymaker-tests/08-host-app.mjs`. `makeApp`, `assetsOf`, `files`, `firewall`, `catalog`, `plain`, `until`, `assert` and `check` are already in that file.

```js
check('host app: an oversized font passes only with a face of the last accepted load, and the total spans load and assets', async () => {
    const MIB = 1048576;
    const cjkFace = { id: 'sm-cjk', family: 'SMCJK', file: '/fonts/qr/NotoSansCJKsc.otf', weight: '400', unicode_range: '', max_bytes: 17 * MIB, d: [32, 94] };
    const withFaces = { ...catalog, glyph_data: { schema: 'storymaker-glyphs/1', faces: [cjkFace] } };
    const nine = new ArrayBuffer(9 * MIB);   // over the 8 MiB cap, under the 17 MiB of the face
    const a = makeApp(), ok = a.H.checkMessage;
    const loadOf = (cat, name, family) => ({ sm: 1, type: 'load', story: firewall, catalog: cat, fonts: [{ name, family, weight: 400, data: nine }] });
    const fontAnswer = (key, data = nine) => ({ sm: 1, type: 'assets', req: 'r', assets: { [key]: data } });

    // load: the message's own glyph data decides
    assert.match(ok(loadOf(withFaces, 'default', 'Storymaker Default')), /oversized/);
    assert.equal(ok(loadOf(withFaces, 'sm-cjk', 'SMCJK')), null);
    assert.match(ok(loadOf(catalog, 'sm-cjk', 'SMCJK')), /oversized/, 'without faces an oversized font is refused');

    // assets: the faces of the last accepted load decide
    const held = { faces: [cjkFace], fontBytes: 0 };
    assert.match(ok(fontAnswer('font:default/400'), held), /oversized/);
    assert.equal(ok(fontAnswer('font:sm-cjk/400'), held), null);
    assert.match(ok(fontAnswer('font:sm-cjk/400')), /oversized/, 'a missing loaded means no faces');
    // 8 MiB + 1 byte is the limit of this face, so 9 MiB does not fit
    assert.match(ok(fontAnswer('font:sm-cjk/400'), { faces: [{ ...cjkFace, max_bytes: 8 * MIB + 1 }], fontBytes: 0 }), /oversized/);
    // max_bytes never lifts the limit above the 18 MiB of maxFallbackFontBytes
    assert.match(ok(fontAnswer('font:sm-cjk/400', new ArrayBuffer(19 * MIB)), { faces: [{ ...cjkFace, max_bytes: 40 * MIB }], fontBytes: 0 }), /oversized/);
    // the 32 MiB total: 23 + 9 = 32 fits, 24 + 9 = 33 does not
    assert.equal(ok(fontAnswer('font:sm-cjk/400'), { faces: [cjkFace], fontBytes: 23 * MIB }), null);
    assert.match(ok(fontAnswer('font:sm-cjk/400'), { faces: [cjkFace], fontBytes: 24 * MIB }), /too large/);

    // the state: set by a load, grown by an assets answer, cleared by a reset
    a.send(a.loadMsg({ catalog: withFaces, assets: {}, fonts: [{ name: 'sm-cjk', family: 'SMCJK', weight: 400, data: nine }] }));
    assert.deepEqual(plain(a.app.state.loaded.faces.map(f => f.id)), ['sm-cjk']);
    assert.equal(a.app.state.loaded.fontBytes, 9 * MIB, 'the load counts its fonts');
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const all = assetsOf(files); all['font:default/400'] = new ArrayBuffer(16);
    a.send({ type: 'assets', req: a.sent('need_assets')[0].req, assets: all, missing: [] });
    assert.equal(a.app.state.loaded.fontBytes, 9 * MIB + 16, 'the assets answer adds the 16 bytes of its font');
    await until(() => a.sent('time').length >= 1, 'the first frame');

    // onMessage passes the state: the same answer is fine before the reset and refused after it
    a.send({ type: 'assets', req: 'late', assets: { 'font:sm-cjk/400': nine } });
    assert.equal(a.sent('error').length, 0);
    a.send({ type: 'reset' });
    assert.deepEqual(plain(a.app.state.loaded), { faces: [], fontBytes: 0 });
    a.send({ type: 'assets', req: 'late', assets: { 'font:sm-cjk/400': nine } });
    assert.equal(a.sent('error').at(-1).code, 'bad_message');
    assert.match(a.sent('error').at(-1).message, /oversized/);

    // the descriptor carries the ranges of a face and leaves a catalog font alone
    const desc = a.H.Engine.prototype.fontDescriptor(withFaces, 'font:sm-cjk/400', nine);
    assert.deepEqual([desc.name, desc.family, desc.weight, desc.weight_range, desc.unicode_range], ['sm-cjk', 'SMCJK', 400, '400', '']);
    const base = a.H.Engine.prototype.fontDescriptor(withFaces, 'font:default/400', nine);
    assert.deepEqual([base.family, base.weight_range, base.unicode_range], ['Storymaker Default', undefined, undefined]);
});
```

**`ui/desktop_storymaker_api_test.go`, first edit.** The catalog folder holds the real coverage file since G1.

Old text:

```js
  const staticFiles = { '/img/storymaker/catalog.json': catalogText };
```

New text:

```js
  const staticFiles = { '/img/storymaker/catalog.json': catalogText, '/img/storymaker/glyphs.json': fs.readFileSync(path.join(path.dirname(catalogFile), 'glyphs.json'), 'utf8') };
```

**`ui/desktop_storymaker_api_test.go`, second edit.**

Old text:

```js
    if (url.indexOf('/fonts/') === 0) return Promise.resolve(new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 }));
```

New text:

```js
    if (url === '/fonts/qr/NotoSansCJKsc.otf') return Promise.resolve(new Response(new Uint8Array(9 * 1048576), { status: 200 }));
    if (url.indexOf('/fonts/') === 0) return Promise.resolve(new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 }));
```

**`ui/desktop_storymaker_api_test.go`, third edit.** The 9 MiB fake is over the default cap of 8 MiB and under the 17825792 of the real `sm-cjk` face.

Old text:

```js
  console.log('storymaker app api ok');
```

New text:

```js
  // fallback faces: the coverage file travels on the catalog, and a face resolves with its own byte limit
  eq(prepared.catalog.glyph_data.schema, 'storymaker-glyphs/1', 'the coverage file is attached to the catalog');
  const faces = await assets.resolve(['font:sm-cjk/400', 'font:sm-greek/400', 'font:sm-cjk/700', 'font:sm-nope/400'], story, id);
  eq(Object.keys(faces.assets).sort(), ['font:sm-cjk/400', 'font:sm-greek/400'], 'both faces resolve');
  eq(faces.assets['font:sm-cjk/400'].byteLength, 9 * 1048576, 'the CJK face passes over 8 MiB through its max_bytes');
  eq(faces.missing.sort(), ['font:sm-cjk/700', 'font:sm-nope/400'], 'a face answers only at weight 400');
  await rejects(S.createApi({ fetch: staticFetch }).fontBuffer('/fonts/qr/NotoSansCJKsc.otf'), err => eq(err.message, 'The font is too large.', 'the default cap stays 8 MiB'), 'font cap');
  console.log('storymaker app api ok');
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
node scripts/test-storymaker-player.mjs
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected:
- The player run exits 1 with two new `FAIL` lines. `workers: FontFace` fails because the descriptor holds only `{weight:'400'}`. `host app: an oversized font` fails because `sm-cjk` is refused at 9 MiB.
- The Go test fails with `Cannot read properties of undefined (reading 'schema')`, because `loadCatalog` does not attach `glyph_data` yet.

- [ ] **Step 4: Implement**

The JS files below have CRLF line endings in the checkout, so each old text is one line.

**`ui/js/storymaker/host-core.js`.**

Old text:

```js
        return { name: name, family: family, weight: weight, data: data };
```

New text:

```js
        var out = { name: name, family: family, weight: weight, data: data };
        var faces = catalog && catalog.glyph_data && catalog.glyph_data.faces;
        (Array.isArray(faces) ? faces : []).forEach(function (f) {
            if (!f || f.id !== name) return;
            out.family = f.family;
            out.weight_range = f.weight;
            out.unicode_range = f.unicode_range;
        });
        return out;
```

**`ui/js/storymaker/worker-player.js`.** Edit 1 of 2 in this task.

Old text:

```js
                        var face = new env.FontFace(f.family, f.data, { weight: String(f.weight) });
```

New text:

```js
                        var desc = { weight: f.weight_range || String(f.weight) };
                        if (f.unicode_range) desc.unicodeRange = f.unicode_range;
                        var face = new env.FontFace(f.family, f.data, desc);
```

**`ui/js/storymaker/worker-scene.js`.** Edit 2 of 2 in this task, the same text as edit 1.

Old text:

```js
                        var face = new env.FontFace(f.family, f.data, { weight: String(f.weight) });
```

New text:

```js
                        var desc = { weight: f.weight_range || String(f.weight) };
                        if (f.unicode_range) desc.unicodeRange = f.unicode_range;
                        var face = new env.FontFace(f.family, f.data, desc);
```

**`ui/js/storymaker/host-app.js`.**

Edit 1. Old text:

```js
    var CAPS = {
```

New text:

```js
    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
```

Edit 2. Old text:

```js
    // checkMessage(msg) = null when the message fits the schema, else a short reason.
```

New text:

```js
    // facesOf(catalog): the fallback faces of a catalog's glyph data. Only these may name a font over maxFontBytes.
    function facesOf(catalog) {
        var data = isObject(catalog) ? catalog.glyph_data : null;
        return isObject(data) && data.schema === 'storymaker-glyphs/1' && Array.isArray(data.faces) ? data.faces : [];
    }

    // fontLimit(name, faces): the byte limit of one font. A face with that id lifts it to its max_bytes, at most maxFallbackFontBytes.
    function fontLimit(name, faces) {
        var limit = CAPS.maxFontBytes;
        faces.forEach(function (face) {
            if (isObject(face) && face.id === name && typeof face.max_bytes === 'number' && face.max_bytes > limit) limit = Math.min(face.max_bytes, CAPS.maxFallbackFontBytes);
        });
        return limit;
    }

    // checkMessage(msg, loaded) = null when the message fits the schema, else a short reason. loaded = {faces, fontBytes} comes
    // from the last accepted load, because an assets message carries no catalog. A missing loaded means no faces and 0 bytes.
```

Edit 3. Old text:

```js
    function checkMessage(msg) {
```

New text:

```js
    function checkMessage(msg, loaded) {
```

Edit 4. Old text:

```js
        var i, k, total = 0;
```

New text:

```js
        var i, k, total = 0;
        var heldFaces = loaded && Array.isArray(loaded.faces) ? loaded.faces : [], heldBytes = loaded && loaded.fontBytes > 0 ? loaded.fontBytes : 0;
```

Edit 5. Old text:

```js
                    for (i = 0; i < msg.fonts.length; i++) {
```

New text:

```js
                    var loadFaces = facesOf(msg.catalog).concat(heldFaces);
                    for (i = 0; i < msg.fonts.length; i++) {
```

Edit 6. Old text:

```js
                        if (f.data.byteLength > CAPS.maxFontBytes) return 'load.fonts has an oversized font';
```

New text:

```js
                        if (f.weight_range !== undefined && (typeof f.weight_range !== 'string' || f.weight_range.length > 16)) return 'load.fonts has a bad entry';
                        if (f.unicode_range !== undefined && (typeof f.unicode_range !== 'string' || f.unicode_range.length > 256)) return 'load.fonts has a bad entry';
                        if (f.data.byteLength > fontLimit(f.name, loadFaces)) return 'load.fonts has an oversized font';
```

Edit 7. Old text:

```js
                for (k in msg.assets) if (!KEY_RE.test(k)) return 'assets.assets has a bad key';
```

New text:

```js
                for (k in msg.assets) if (!KEY_RE.test(k)) return 'assets.assets has a bad key';
                for (k in msg.assets) {
                    if (k.indexOf('font:') !== 0) continue;
                    if (!isBuffer(msg.assets[k])) return 'assets.assets has a bad font';
                    if (msg.assets[k].byteLength > fontLimit(k.slice(5).replace(/\/\d+$/, ''), heldFaces)) return 'assets.assets has an oversized font';
                    total += msg.assets[k].byteLength;
                }
                if (heldBytes + total > CAPS.maxFontTotal) return 'assets.assets is too large';
```

Edit 8. Old text:

```js
        var state = { engine: null, report: null, frame: 0, playing: false, loadToken: 0, busy: false, shown: -1, lastTime: -1e9, pendingAssets: {}, nextAsset: 1 };
```

New text:

```js
        var state = { engine: null, report: null, frame: 0, playing: false, loadToken: 0, busy: false, shown: -1, lastTime: -1e9, pendingAssets: {}, nextAsset: 1, loaded: { faces: [], fontBytes: 0 } };
```

Edit 9. Old text:

```js
            state.pendingAssets = {};
```

New text:

```js
            state.pendingAssets = {};
            state.loaded = { faces: [], fontBytes: 0 };
```

Edit 10. Old text:

```js
                var engine = H.createEngine(env);
```

New text:

```js
                state.loaded = { faces: facesOf(msg.catalog), fontBytes: (msg.fonts || []).reduce(function (sum, f) { return sum + f.data.byteLength; }, 0) };
                var engine = H.createEngine(env);
```

Edit 11. Old text:

```js
            env.clearTimer(p.timer);
```

New text:

```js
            env.clearTimer(p.timer);
            Object.keys(msg.assets).forEach(function (k) { if (k.indexOf('font:') === 0) state.loaded.fontBytes += msg.assets[k].byteLength; });
```

Edit 12. Old text:

```js
            var why = checkMessage(event.data);
```

New text:

```js
            var why = checkMessage(event.data, state.loaded);
```

**`ui/js/desktop/apps/storymaker-api.js`.**

Old text:

```js
            async fontBuffer(file) {
```

New text:

```js
            // maxBytes is the byte limit of a fallback face; the default stays 8 MiB
            async fontBuffer(file, maxBytes) {
```

Old text:

```js
                if (buffer.byteLength > 8 * 1048576) throw S.errors.normalize({ message: 'The font is too large.' });
```

New text:

```js
                if (buffer.byteLength > (maxBytes > 0 ? maxBytes : 8 * 1048576)) throw S.errors.normalize({ message: 'The font is too large.' });
```

**`ui/js/desktop/apps/storymaker-assets.js`.**

Edit 1. Old text:

```js
    const KEY_RE = /^(catalog|font|bundle):(\S{1,230})$/;
```

New text:

```js
    const KEY_RE = /^(catalog|font|bundle):(\S{1,230})$/;
    const FONT_FILE_RE = /^\/fonts\/(qr\/)?[A-Za-z0-9_.-]+$/;
```

Edit 2. The rejection handler of the existing `.then` now covers both fetches, so a failed coverage file is not cached.

Old text:

```js
                catalogPromise = api.catalogJSON('catalog.json').then(raw => {
```

New text:

```js
                catalogPromise = api.catalogJSON('catalog.json').then(async raw => {
                    // the coverage file travels on the catalog as glyph_data: in memory only, never in catalog.json
                    if (raw && raw.glyphs && typeof raw.glyphs.file === 'string') raw.glyph_data = await api.catalogJSON(raw.glyphs.file);
                    return raw;
                }).then(raw => {
```

Edit 3. Old text:

```js
            return (raw.fonts || []).filter(font => font && font.name === name && typeof font.file === 'string' && /^\/fonts\/[A-Za-z0-9_.-]+$/.test(font.file));
```

New text:

```js
            return (raw.fonts || []).filter(font => font && font.name === name && typeof font.file === 'string' && FONT_FILE_RE.test(font.file));
```

Edit 4. Old text:

```js
        function codeSceneIds(story) {
```

New text:

```js
        // glyphFace(raw, id): the fallback face of the coverage file, with its file and max_bytes. Its key is always font:<id>/400.
        function glyphFace(raw, id) {
            const faces = raw.glyph_data && Array.isArray(raw.glyph_data.faces) ? raw.glyph_data.faces : [];
            return faces.find(face => face && face.id === id && typeof face.file === 'string' && FONT_FILE_RE.test(face.file));
        }

        function codeSceneIds(story) {
```

Edit 5. Old text:

```js
                        const face = parts && fontFaces(raw, parts[1]).find(font => String(font.weight) === parts[2]);
```

New text:

```js
                        const face = parts && (fontFaces(raw, parts[1]).find(font => String(font.weight) === parts[2]) || (parts[2] === '400' && glyphFace(raw, parts[1])));
```

Edit 6. Old text:

```js
                        assets[key] = await cached(key, () => api.fontBuffer(face.file));
```

New text:

```js
                        assets[key] = await cached(key, () => api.fontBuffer(face.file, face.max_bytes));
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
npm run build:ui
npm run check:ui
npm run test:storymaker
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected: `check:ui` passes, `test:storymaker` ends with `0 failed`, and the Go test passes.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

```powershell
git add ui/js/storymaker/host-core.js ui/js/storymaker/worker-player.js ui/js/storymaker/worker-scene.js ui/js/storymaker/host-app.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js ui/js/desktop/apps/storymaker-assets.js ui/js/desktop/apps/storymaker-api.js scripts/storymaker-tests/07-workers.mjs scripts/storymaker-tests/08-host-app.mjs ui/desktop_storymaker_api_test.go
git commit -m "feat(storymaker): fallback faces from app to host, font caps across load and assets" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

