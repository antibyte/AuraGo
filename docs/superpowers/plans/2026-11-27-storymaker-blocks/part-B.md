# Part B: Icons

> Generated from the drafted and tested commits f8bc11664..f9eb9c256. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task B1: node-to-path converter

**Files:**
- Create: `assets/storymaker/lib/icons.mjs`
- Create: `assets/storymaker/test/icons.test.mjs`

- [ ] **Step 1: Write the failing test**

Create `assets/storymaker/test/icons.test.mjs`:

```js
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
```

- [ ] **Step 2: Run it and see it fail**

Run: `node --test assets/storymaker/test/icons.test.mjs`
Expected: ERR_MODULE_NOT_FOUND for assets/storymaker/lib/icons.mjs

- [ ] **Step 3: Implement**

Create `assets/storymaker/lib/icons.mjs`:

```js
// Lucide icon data for Storymaker: converts icon-nodes.json elements to path data on the 24 grid.
// Reads only icon-nodes.json, tags.json and LICENSE of a lucide-static package; it never runs package code.

import fs from 'node:fs';
import path from 'node:path';

export const LUCIDE_VERSION = '1.82.0';
export const LUCIDE_SOURCE = `lucide-static@${LUCIDE_VERSION}`;
export const LUCIDE_INTEGRITY = 'sha512-Xzzpfkx06hHXjXoLWOk1RWg52OhfaRcIOIK72hCbRty8Jq29WQUV4J4Z9GvjAKDoPxGf7tlcqBbJX4qrr1U8+w==';
export const ICON_SCHEMA = 'storymaker-icons/1';
export const ALLOW_LIST_SCHEMA = 'storymaker-icon-allow-list/1';
export const CATEGORIES = ['devices', 'development', 'connectivity', 'security', 'science', 'medical', 'finance', 'charts', 'communication', 'people', 'nature', 'transport', 'weather', 'files', 'arrows', 'home', 'education', 'general'];
export const PATH_CHARS = /^[MmLlHhVvCcSsQqTtAaZz0-9 .,eE-]+$/;

export const fmt = (n) => String(Math.round(n * 1000) / 1000);

const num = (attrs, key, tag) => {
    const v = Number(attrs[key]);
    if (attrs[key] === undefined || !Number.isFinite(v)) throw new Error(`${tag}: attribute ${key} is missing or not a number`);
    return v;
};

const points = (attrs, tag) => {
    const v = String(attrs.points ?? '').trim().split(/[\s,]+/).map(Number);
    if (v.length < 4 || v.length % 2 || v.some((n) => !Number.isFinite(n))) throw new Error(`${tag}: bad points`);
    const out = [];
    for (let i = 0; i < v.length; i += 2) out.push(`${i ? 'L' : 'M'}${fmt(v[i])} ${fmt(v[i + 1])}`);
    return out.join('');
};

const ellipse = (cx, cy, rx, ry) => `M${fmt(cx - rx)} ${fmt(cy)}A${fmt(rx)} ${fmt(ry)} 0 1 0 ${fmt(cx + rx)} ${fmt(cy)}A${fmt(rx)} ${fmt(ry)} 0 1 0 ${fmt(cx - rx)} ${fmt(cy)}Z`;

function nodeToPath([tag, attrs]) {
    if ('transform' in attrs) throw new Error(`${tag}: transform is not supported`);
    switch (tag) {
        case 'path': return String(attrs.d ?? '').trim();
        case 'circle': {
            const r = num(attrs, 'r', tag);
            return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), r, r);
        }
        case 'ellipse': return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), num(attrs, 'rx', tag), num(attrs, 'ry', tag));
        case 'rect': {
            const x = num(attrs, 'x', tag), y = num(attrs, 'y', tag), w = num(attrs, 'width', tag), h = num(attrs, 'height', tag);
            let rx = attrs.rx === undefined ? undefined : num(attrs, 'rx', tag);
            let ry = attrs.ry === undefined ? undefined : num(attrs, 'ry', tag);
            rx = Math.min(rx ?? ry ?? 0, w / 2);
            ry = Math.min(ry ?? rx ?? 0, h / 2);
            if (!rx && !ry) return `M${fmt(x)} ${fmt(y)}H${fmt(x + w)}V${fmt(y + h)}H${fmt(x)}Z`;
            const a = `A${fmt(rx)} ${fmt(ry)} 0 0 1 `;
            return `M${fmt(x + rx)} ${fmt(y)}H${fmt(x + w - rx)}${a}${fmt(x + w)} ${fmt(y + ry)}V${fmt(y + h - ry)}${a}${fmt(x + w - rx)} ${fmt(y + h)}H${fmt(x + rx)}${a}${fmt(x)} ${fmt(y + h - ry)}V${fmt(y + ry)}${a}${fmt(x + rx)} ${fmt(y)}Z`;
        }
        case 'line': return `M${fmt(num(attrs, 'x1', tag))} ${fmt(num(attrs, 'y1', tag))}L${fmt(num(attrs, 'x2', tag))} ${fmt(num(attrs, 'y2', tag))}`;
        case 'polyline': return points(attrs, tag);
        case 'polygon': return `${points(attrs, tag)}Z`;
        default: throw new Error(`${tag}: element is not supported`);
    }
}

// nodes: [[tag, attrs], ...] -> {paths, fills}; fills are the elements with a fill other than none (drawn filled and stroked).
export function nodesToPaths(nodes) {
    const paths = [], fills = [];
    for (const node of nodes) {
        const d = nodeToPath(node);
        paths.push(d);
        const fill = node[1].fill;
        if (fill !== undefined && fill !== 'none') fills.push(d);
    }
    return { paths, fills };
}

const label = (name) => name.split('-');

// allowList: {schema, source, categories: {name: [ids]}} -> {json, notice, count, problems}
export function buildIconData(pkgDir, allowList) {
    const read = (f) => fs.readFileSync(path.join(pkgDir, f), 'utf8');
    const nodes = JSON.parse(read('icon-nodes.json'));
    const tags = JSON.parse(read('tags.json'));
    const license = read('LICENSE').replace(/\r\n/g, '\n');
    const problems = [];
    const cats = new Map();
    if (allowList.schema !== ALLOW_LIST_SCHEMA) problems.push(`allow-list schema is ${allowList.schema}`);
    for (const cat of CATEGORIES) for (const id of allowList.categories?.[cat] ?? []) {
        if (!cats.has(id)) cats.set(id, []);
        if (!cats.get(id).includes(cat)) cats.get(id).push(cat);
    }
    for (const cat of Object.keys(allowList.categories ?? {})) if (!CATEGORIES.includes(cat)) problems.push(`unknown category ${cat}`);
    const lines = [];
    for (const name of [...cats.keys()].sort()) {
        if (!Object.prototype.hasOwnProperty.call(nodes, name)) { problems.push(`allow-list id ${name} is not in the package`); continue; }
        let conv;
        try { conv = nodesToPaths(nodes[name]); } catch (e) { problems.push(`${name}: ${e.message}`); continue; }
        const icon = { id: `icon.${name}`, paths: conv.paths };
        if (conv.fills.length) icon.fills = conv.fills;
        icon.tags = [...new Set([...(tags[name] ?? []).slice(0, 20), ...label(name), ...cats.get(name)])];
        if (!/^icon\.[a-z0-9-]{1,60}$/.test(icon.id)) problems.push(`${name}: bad id`);
        if (icon.paths.length < 1 || icon.paths.length > 40) problems.push(`${name}: ${icon.paths.length} paths`);
        for (const p of icon.paths) if (p.length > 2000 || !PATH_CHARS.test(p)) problems.push(`${name}: bad path data ${p.slice(0, 40)}`);
        if (icon.tags.length > 32) problems.push(`${name}: ${icon.tags.length} tags`);
        lines.push(JSON.stringify(icon));
    }
    const json = `{"schema":"${ICON_SCHEMA}","source":"${LUCIDE_SOURCE}","grid":24,"stroke":2,"icons":[\n${lines.join(',\n')}\n]}\n`;
    if (Buffer.byteLength(json) > 256 * 1024) problems.push(`lucide.json is ${Buffer.byteLength(json)} bytes`);
    const notice = `Storymaker icons: ${lines.length} icons from Lucide (lucide-static ${LUCIDE_VERSION}, https://lucide.dev), ISC License.\nSome Lucide icons derive from Feather (Cole Bemis), MIT License; both texts follow.\nPackage integrity: ${LUCIDE_INTEGRITY}\nConverted to path data on the 24 grid by scripts/build-storymaker-icons.mjs from icon-nodes.json and tags.json.\n\n${license}`;
    return { json, notice, count: lines.length, problems, license };
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `node --test assets/storymaker/test/icons.test.mjs`
Expected: 2 tests pass

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/icons.mjs assets/storymaker/test/icons.test.mjs
git commit -m "storymaker: node-to-path converter"
```

### Task B2: build script, allow-list and committed icon data

**Files:**
- Modify: `.gitattributes`
- Create: `assets/storymaker/icons/LICENSE.txt` (generated)
- Create: `assets/storymaker/icons/NOTICE.txt` (generated)
- Create: `assets/storymaker/icons/allow-list.json`
- Create: `assets/storymaker/icons/lucide.json` (generated)
- Modify: `assets/storymaker/test/icons.test.mjs`
- Create: `scripts/build-storymaker-icons.mjs`

- [ ] **Step 1: Write the failing test**

In `assets/storymaker/test/icons.test.mjs`, replace:

```js

import test from 'node:test';
import assert from 'node:assert/strict';
import { nodesToPaths, fmt } from '../lib/icons.mjs';

test('every node kind converts to path data on the 24 grid', () => {
  const one = (node) => nodesToPaths([node]).paths[0];
```

with:

```js

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { nodesToPaths, fmt, PATH_CHARS, CATEGORIES, LUCIDE_INTEGRITY } from '../lib/icons.mjs';

const iconsDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'icons');

test('every node kind converts to path data on the 24 grid', () => {
  const one = (node) => nodesToPaths([node]).paths[0];
```

In `assets/storymaker/test/icons.test.mjs`, replace:

```js
  assert.throws(() => nodesToPaths([['path', { d: 'M1 1', transform: 'rotate(5)' }]]), /transform/);
  assert.throws(() => nodesToPaths([['text', {}]]), /text/);
});
```

with:

```js
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
```

- [ ] **Step 2: Run it and see it fail**

Run: `node --test assets/storymaker/test/icons.test.mjs`
Expected: ENOENT for assets/storymaker/icons/lucide.json (3rd test fails, 2 pass)

- [ ] **Step 3: Implement**

In `.gitattributes`, replace:

```text
# Chibi Sensei: the trainer's bytes are hashed into every run; golden files and proof data are compared byte for byte.
internal/chibisensei/trainer/** text eol=lf
internal/chibisensei/testdata/** text eol=lf
```

with:

```text
# Chibi Sensei: the trainer's bytes are hashed into every run; golden files and proof data are compared byte for byte.
internal/chibisensei/trainer/** text eol=lf
internal/chibisensei/testdata/** text eol=lf

assets/storymaker/icons/*.txt text eol=lf
ui/img/storymaker/icons/*.txt text eol=lf
```

Create `assets/storymaker/icons/allow-list.json`:

```json
{"schema":"storymaker-icon-allow-list/1","source":"lucide-static@1.82.0","categories":{
"devices":["cpu","microchip","server","server-cog","server-crash","hard-drive","database","database-zap","database-search","monitor","monitor-smartphone","monitor-play","laptop","smartphone","tablet","tv","keyboard","mouse","printer","router","webcam","camera","headphones","speaker","watch","gamepad-2","joystick","calculator","projector","memory-stick","usb","cable","battery","battery-charging","plug","plug-zap","unplug","power","radio","drone"],
"development":["code","code-xml","square-code","terminal","braces","brackets","bug","git-branch","git-merge","git-pull-request","git-graph","binary","file-code","file-braces","variable","webhook","workflow","blocks","box","boxes","package","puzzle","component","layers","layout-dashboard","app-window","settings","wrench","hammer","sliders-horizontal","rocket","bot","brain-circuit","sparkles","wand-sparkles","container","infinity","refresh-cw","gauge","activity","zap","cloud-cog"],
"connectivity":["globe","earth","wifi","wifi-off","signal","antenna","radio-tower","rss","cast","share-2","send","network","waypoints","route","satellite","satellite-dish","nfc","cloud","cloud-off","cloud-upload","cloud-download","link","unlink","ethernet-port","bluetooth","globe-lock","server-off","arrow-left-right"],
"security":["shield","shield-check","shield-exclamation-point","shield-x","shield-off","shield-user","lock","lock-open","lock-keyhole","key","key-round","fingerprint-pattern","scan-face","eye","eye-off","user-check","user-lock","badge-check","siren","triangle-exclamation-point","circle-exclamation-point","ban","file-lock","folder-lock","vault","id-card","skull","cctv","door-closed","door-open","brick-wall","brick-wall-fire","scan-search"],
"science":["atom","flask-conical","flask-round","beaker","test-tube","test-tubes","microscope","dna","telescope","magnet","orbit","radiation","biohazard","pipette","thermometer","sigma","pi","lightbulb","flame","droplet","wind","brain","cone","cylinder","chart-scatter","virus"],
"medical":["heart","heart-pulse","heart-handshake","stethoscope","pill","pill-bottle","syringe","bandage","hospital","ambulance","cross","bone","accessibility","hand-heart","baby","ear","apple","salad","dumbbell","bed","clipboard-plus","briefcase-medical","tablets","person-standing"],
"finance":["wallet","wallet-cards","credit-card","banknote","coins","piggy-bank","receipt","landmark","dollar-sign","euro","bitcoin","percent","badge-percent","hand-coins","trending-up","trending-down","shopping-cart","shopping-bag","store","tag","tags","gift","handshake","scale","briefcase","building","factory","truck"],
"charts":["chart-bar","chart-bar-increasing","chart-column","chart-column-increasing","chart-line","chart-pie","chart-area","chart-spline","chart-network","chart-gantt","presentation","table","table-2","target","goal","kanban","list-checks","list-ordered","list","funnel"],
"communication":["mail","mail-open","inbox","message-circle","message-square","messages-square","message-circle-question-mark","phone","phone-call","video","mic","megaphone","bell","bell-ring","at-sign","hash","contact","book-user","languages","speech","quote","thumbs-up","thumbs-down","star","flag","bookmark","newspaper","headset","sticky-note","notebook-pen","captions","audio-waveform"],
"people":["user","users","user-plus","user-minus","user-round","users-round","user-cog","user-search","user-star","user-exclamation-point","circle-user","circle-user-round","crown","hand","face-slightly-smiling","face-grinning","face-neutral","face-slightly-frowning","face-angry","shirt","graduation-cap"],
"nature":["sun","moon","snowflake","droplets","waves-horizontal","mountain","mountain-snow","tree-pine","tree-deciduous","trees","tree-palm","leaf","sprout","flower","flower-2","clover","bird","fish","squirrel","rabbit","dog","cat","turtle","shell","feather","paw-print","sunrise","sunset","rainbow","tornado","recycle","wheat","carrot","citrus"],
"transport":["car","car-front","bus","bike","train-front","tram-front","train-track","plane","plane-takeoff","plane-landing","helicopter","ship","sailboat","anchor","fuel","map","map-pin","map-pinned","navigation","compass","milestone","traffic-cone","forklift","tractor","cable-car","ticket"],
"weather":["cloud-rain","cloud-drizzle","cloud-hail","cloud-fog","cloud-moon","cloud-sun","cloud-snow","cloud-lightning","cloudy","thermometer-sun","thermometer-snowflake","umbrella","haze","sun-dim","sun-snow","moon-star"],
"files":["file","file-text","file-image","file-video-camera","file-music","file-spreadsheet","file-chart-column","file-check","file-x","file-plus","file-minus","file-search","file-archive","file-stack","files","folder","folder-open","folder-tree","archive","clipboard","clipboard-check","clipboard-list","notebook","book","book-open","library","paperclip","save","download","upload","trash","copy","sheet","image","film","eighth-notes"],
"arrows":["arrow-right","arrow-left","arrow-up","arrow-down","arrow-up-right","arrow-down-right","arrow-big-right","arrow-right-left","arrow-right-to-line","arrow-down-to-line","arrow-up-from-line","move","move-right","move-horizontal","chevron-right","chevrons-right","corner-down-right","undo-2","rotate-cw","repeat","shuffle","split","merge","expand","shrink","maximize","circle-arrow-right","square-arrow-right","external-link","iteration-cw"],
"home":["house","house-wifi","sofa","armchair","bath","toilet","lamp","lamp-desk","refrigerator","microwave","washing-machine","heater","fan","coffee","utensils","cooking-pot","chef-hat","fence","warehouse","hotel","church","castle","tent","alarm-clock"],
"education":["school","backpack","book-open-check","library-big","pencil","pencil-ruler","pen","pen-tool","ruler","eraser","highlighter","spell-check","shapes","palette","trophy","medal","award","list-todo","square-check","circle-check","question-mark","circle-question-mark"],
"general":["check","x","plus","minus","circle-x","info","exclamation-mark","search","zoom-in","clock","calendar","calendar-check","timer","hourglass","pin","equal","divide","crosshair","focus","scan","grid-3x3","circle","circle-dot","square","triangle","hexagon","diamond","sparkle","gem","party-popper","flag-triangle-right","signpost","loader","badge"]
}}
```

Create `scripts/build-storymaker-icons.mjs`:

```js
// Builds assets/storymaker/icons/{lucide.json,LICENSE.txt,NOTICE.txt} from a lucide-static package and the allow-list.
//   node scripts/build-storymaker-icons.mjs --verify <lucide-static.tgz>     checks the sha512 integrity
//   node scripts/build-storymaker-icons.mjs --package <dir>                  writes the files
//   node scripts/build-storymaker-icons.mjs --check --package <dir>          compares without writing
// The package is data: only icon-nodes.json, tags.json and LICENSE are read.

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildIconData, LUCIDE_INTEGRITY } from '../assets/storymaker/lib/icons.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'assets', 'storymaker', 'icons');

const args = process.argv.slice(2);
const arg = (name) => { const i = args.indexOf(name); return i < 0 ? null : args[i + 1] ?? ''; };
const fail = (msg) => { console.error(msg); process.exit(1); };

if (args.includes('--verify')) {
    const tgz = arg('--verify');
    if (!tgz) fail('--verify needs the path of the tarball');
    const got = `sha512-${crypto.createHash('sha512').update(fs.readFileSync(tgz)).digest('base64')}`;
    if (got !== LUCIDE_INTEGRITY) fail(`integrity mismatch: ${got}`);
    console.log('integrity ok');
    if (!arg('--package')) process.exit(0);
}

const pkg = arg('--package');
if (!pkg) fail('usage: --package <dir> [--check] | --verify <tgz>');

const allowList = JSON.parse(fs.readFileSync(path.join(outDir, 'allow-list.json'), 'utf8'));
const built = buildIconData(pkg, allowList);
if (built.problems.length) fail(`icon build failed:\n${built.problems.join('\n')}`);
const outputs = { 'lucide.json': built.json, 'LICENSE.txt': built.license, 'NOTICE.txt': built.notice };

let stale = 0;
for (const [name, text] of Object.entries(outputs)) {
    const file = path.join(outDir, name);
    if (args.includes('--check')) {
        const have = fs.existsSync(file) ? fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n') : null;
        if (have !== text) { console.error(`stale: assets/storymaker/icons/${name}`); stale++; }
    } else {
        fs.writeFileSync(file, text);
    }
}
if (stale) process.exit(1);
console.log(args.includes('--check') ? `icons up to date (${built.count})` : `wrote ${built.count} icons`);
```

- [ ] **Step 4: Rebuild the generated files**

Run: node scripts/build-storymaker-icons.mjs --verify <lucide-static-1.82.0.tgz> and then --package <unpacked package dir> (see the plan index: Lucide package)

- [ ] **Step 5: Run the test and see it pass**

Run: `node --test assets/storymaker/test/icons.test.mjs`
Expected: 3 tests pass; 516 icons, lucide.json 168937 bytes

- [ ] **Step 6: Commit**

```bash
git add .gitattributes assets/storymaker/icons/LICENSE.txt assets/storymaker/icons/NOTICE.txt assets/storymaker/icons/allow-list.json assets/storymaker/icons/lucide.json assets/storymaker/test/icons.test.mjs scripts/build-storymaker-icons.mjs
git commit -m "storymaker: build script, allow-list and committed icon data"
```

### Task B3: the catalog delivers icons, licence and notice

**Files:**
- Modify: `assets/storymaker/lib/raster-catalog.mjs`
- Modify: `assets/storymaker/lib/raster-validate.mjs`
- Modify: `assets/storymaker/test/catalog.test.mjs`
- Modify: `licences/THIRD_PARTY_LIBRARIES.md`
- Modify: `scripts/build-storymaker-catalog.mjs`
- Modify: `ui/img/storymaker/catalog.json` (generated)
- Create: `ui/img/storymaker/icons/LICENSE.txt` (generated)
- Create: `ui/img/storymaker/icons/NOTICE.txt` (generated)
- Create: `ui/img/storymaker/icons/lucide.json` (generated)

- [ ] **Step 1: Write the failing test**

In `assets/storymaker/test/catalog.test.mjs`, replace:

```js
  assert.equal(r.status, 0, r.stdout + r.stderr);
});

test('every foreground accent bleeds off the frame edge it reaches in its cell', () => {
  // A near-camera accent that reaches the edge of its atlas cell belongs to that edge of the picture: it has to bleed off
  // it. An accent that stops short of the edge floats inside the frame with a visible cut, which is what the stage curtain
```

with:

```js
  assert.equal(r.status, 0, r.stdout + r.stderr);
});

test('the delivered catalog names icons/lucide.json and ships LICENSE.txt and NOTICE.txt', () => {
  const built = buildRasterOutputs();
  assert.deepEqual(built.manifest.icons, { file: 'icons/lucide.json' });
  const listed = new Map(built.manifest.files.map(f => [f.path, f]));
  for (const f of ['icons/lucide.json', 'icons/LICENSE.txt', 'icons/NOTICE.txt']) assert.ok(listed.has(f), f);
  assert.equal(JSON.parse(built.files.get('icons/lucide.json')).schema, 'storymaker-icons/1');
  assert.ok(built.files.get('icons/NOTICE.txt').toString().includes('ISC'));
  assert.deepEqual(validateRasterOutputs(built), []);
  built.manifest.icons = { file: 'icons/missing.json' };
  assert.ok(validateRasterOutputs(built).some(p => p.includes('icons/missing.json')));
});

test('every foreground accent bleeds off the frame edge it reaches in its cell', () => {
  // A near-camera accent that reaches the edge of its atlas cell belongs to that edge of the picture: it has to bleed off
  // it. An accent that stops short of the edge floats inside the frame with a visible cut, which is what the stage curtain
```

- [ ] **Step 2: Run it and see it fail**

Run: `node --test assets/storymaker/test/catalog.test.mjs; node scripts/build-storymaker-catalog.mjs --check; go test ./ui -run '(?i)storymakercatalog' -count=1`
Expected: catalog test fails, manifest has no icons key (1 of 16 fails)

- [ ] **Step 3: Implement**

In `assets/storymaker/lib/raster-catalog.mjs`, replace:

```js
        if(e.kind==='sfx')for(const take of e.takes)put(take.file,legacy.files.get(take.file));
    }
    put('glyphs.json',Buffer.from(legacy.files.get('glyphs.json')));
    const manifest={...legacy.manifest,schema:'storymaker-catalog/2',entries,files:[]};
    for(const [file,value]of files){
        const bytes=Buffer.isBuffer(value)?value:Buffer.from(value),ext=path.extname(file),type=ext==='.png'?'image/png':ext==='.wav'?'audio/wav':ext==='.glb'?'model/gltf-binary':ext==='.txt'?'text/plain':'application/json';
        const meta={path:file,type,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex')};
```

with:

```js
        if(e.kind==='sfx')for(const take of e.takes)put(take.file,legacy.files.get(take.file));
    }
    put('glyphs.json',Buffer.from(legacy.files.get('glyphs.json')));
    for(const name of ['lucide.json','LICENSE.txt','NOTICE.txt'])put('icons/'+name,fs.readFileSync(path.join(root,'assets/storymaker/icons',name)));
    const manifest={...legacy.manifest,schema:'storymaker-catalog/2',icons:{file:'icons/lucide.json'},entries,files:[]};
    for(const [file,value]of files){
        const bytes=Buffer.isBuffer(value)?value:Buffer.from(value),ext=path.extname(file),type=ext==='.png'?'image/png':ext==='.wav'?'audio/wav':ext==='.glb'?'model/gltf-binary':ext==='.txt'?'text/plain':'application/json';
        const meta={path:file,type,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex')};
```

In `assets/storymaker/lib/raster-validate.mjs`, replace:

```js
  check(m&&m.bytes<=8*1048576,`Oversized file: ${file}`);
  if(m?.type==='image/png')check(m.width===bytes.readUInt32BE(16)&&m.height===bytes.readUInt32BE(20),`Incorrect image dimensions: ${file}`);
 }
 const json=file=>JSON.parse(files.get(file));
 function region(image,where){
  const m=delivery.get(image?.file),r=image?.rect;
```

with:

```js
  check(m&&m.bytes<=8*1048576,`Oversized file: ${file}`);
  if(m?.type==='image/png')check(m.width===bytes.readUInt32BE(16)&&m.height===bytes.readUInt32BE(20),`Incorrect image dimensions: ${file}`);
 }
 check(delivery.has(manifest.icons?.file),`Unlisted icons file: ${manifest.icons?.file}`);
 const json=file=>JSON.parse(files.get(file));
 function region(image,where){
  const m=delivery.get(image?.file),r=image?.rect;
```

In `licences/THIRD_PARTY_LIBRARIES.md`, replace:

```markdown
| GSAP 3.15.1 | `ui/js/vendor/storymaker/runtime.js` | Standard No Charge License; see adjacent `LICENSES.txt` and restrictions in `THIRD_PARTY_NOTICES.md` | https://gsap.com/community/standard-license/ |
| @pixi/colord 2.9.6 | Included in the Storymaker PixiJS runtime | MIT; upstream text retained in `storymaker-colord-LICENSE.txt` and the generated runtime `LICENSES.txt` | https://github.com/pixijs/colord |
| three.js 0.186.1 | `ui/js/vendor/three.min.js`; Storymaker's local runtime, including GLTFLoader and SkeletonUtils | MIT; bundled notices retained in `ui/js/vendor/storymaker/LICENSES.txt` | https://threejs.org / https://github.com/mrdoob/three.js |
| GLTFLoader | `ui/js/vendor/GLTFLoader.min.js` | MIT | three.js examples |
| STLLoader | `ui/js/vendor/STLLoader.min.js` | MIT | three.js examples |
| OrbitControls | `ui/js/vendor/OrbitControls.min.js` | MIT | three.js examples |
```

with:

```markdown
| GSAP 3.15.1 | `ui/js/vendor/storymaker/runtime.js` | Standard No Charge License; see adjacent `LICENSES.txt` and restrictions in `THIRD_PARTY_NOTICES.md` | https://gsap.com/community/standard-license/ |
| @pixi/colord 2.9.6 | Included in the Storymaker PixiJS runtime | MIT; upstream text retained in `storymaker-colord-LICENSE.txt` and the generated runtime `LICENSES.txt` | https://github.com/pixijs/colord |
| three.js 0.186.1 | `ui/js/vendor/three.min.js`; Storymaker's local runtime, including GLTFLoader and SkeletonUtils | MIT; bundled notices retained in `ui/js/vendor/storymaker/LICENSES.txt` | https://threejs.org / https://github.com/mrdoob/three.js |
| Lucide icons (lucide-static 1.82.0) | ui/img/storymaker/icons/lucide.json | ISC; Feather-derived icons MIT; texts in adjacent LICENSE.txt and NOTICE.txt | https://lucide.dev |
| GLTFLoader | `ui/js/vendor/GLTFLoader.min.js` | MIT | three.js examples |
| STLLoader | `ui/js/vendor/STLLoader.min.js` | MIT | three.js examples |
| OrbitControls | `ui/js/vendor/OrbitControls.min.js` | MIT | three.js examples |
```

In `scripts/build-storymaker-catalog.mjs`, replace:

```js
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'ui', 'img', 'storymaker');
const langDir = path.join(root, 'ui', 'lang', 'storymaker_catalog');
const SUBDIRS = ['models', 'rig', 'characters', 'textures', 'props', 'environments', 'sfx', 'thumbs'];


function expectedFiles() {
```

with:

```js
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'ui', 'img', 'storymaker');
const langDir = path.join(root, 'ui', 'lang', 'storymaker_catalog');
const SUBDIRS = ['models', 'rig', 'characters', 'textures', 'props', 'environments', 'sfx', 'thumbs', 'icons'];


function expectedFiles() {
```

- [ ] **Step 4: Rebuild the generated files**

Run: node scripts/build-storymaker-catalog.mjs, then node scripts/build-storymaker-catalog.mjs --check (git checkout LF-only phantom diffs of unchanged thumbs)

- [ ] **Step 5: Run the test and see it pass**

Run: `node --test assets/storymaker/test/catalog.test.mjs; node scripts/build-storymaker-catalog.mjs --check; go test ./ui -run '(?i)storymakercatalog' -count=1`
Expected: 16 node tests pass, catalog check up to date (version b4bef9bececf -> 10b1f67cc96a), 8 Go PASS lines

- [ ] **Step 6: Commit**

```bash
git add assets/storymaker/lib/raster-catalog.mjs assets/storymaker/lib/raster-validate.mjs assets/storymaker/test/catalog.test.mjs licences/THIRD_PARTY_LIBRARIES.md scripts/build-storymaker-catalog.mjs ui/img/storymaker/catalog.json ui/img/storymaker/icons/LICENSE.txt ui/img/storymaker/icons/NOTICE.txt ui/img/storymaker/icons/lucide.json
git commit -m "storymaker: the catalog delivers icons, licence and notice"
```

### Task B4: Go loads the icon file

**Files:**
- Modify: `internal/storymaker/catalog.go`
- Modify: `internal/storymaker/catalog_models_test.go`
- Modify: `internal/storymaker/glyphs_test.go`
- Create: `internal/storymaker/icons.go`
- Create: `internal/storymaker/icons_test.go`
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/testdata/catalog-mini/catalog.json`
- Create: `internal/storymaker/testdata/catalog-mini/icons.json`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/catalog_models_test.go`, replace:

```go
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var character map[string]any
	for _, value := range doc["entries"].([]any) {
		entry := value.(map[string]any)
```

with:

```go
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "icons") // this fixture serves catalog.json alone
	var character map[string]any
	for _, value := range doc["entries"].([]any) {
		entry := value.(map[string]any)
```

In `internal/storymaker/glyphs_test.go`, replace:

```go
package storymaker

import (
	"reflect"
	"strings"
	"testing"
```

with:

```go
package storymaker

import (
	"os"
	"reflect"
	"strings"
	"testing"
```

In `internal/storymaker/glyphs_test.go`, replace:

```go
	named := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"glyphs.json"},"rig":{`, 1)
	})
	fsys := fstest.MapFS{"img/catalog.json": {Data: named}, "img/glyphs.json": {Data: []byte(glyphsFixture)}}
	c, err := LoadCatalog(fsys, "img/catalog.json")
	if err != nil {
		t.Fatal(err)
```

with:

```go
	named := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"glyphs.json"},"rig":{`, 1)
	})
	icons, err := os.ReadFile("testdata/catalog-mini/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"img/catalog.json": {Data: named}, "img/glyphs.json": {Data: []byte(glyphsFixture)}, "img/icons.json": {Data: icons}}
	c, err := LoadCatalog(fsys, "img/catalog.json")
	if err != nil {
		t.Fatal(err)
```

Create `internal/storymaker/icons_test.go`:

```go
package storymaker

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadCatalogReadsIcons(t *testing.T) {
	mini := loadMini(t)
	if got := len(mini.IconIDs()); got != 6 {
		t.Fatalf("mini icons = %d, want 6", got)
	}
	cpu, ok := mini.Icon("icon.cpu")
	if !ok || len(cpu.Paths) == 0 || len(cpu.Tags) == 0 {
		t.Fatalf("Icon(icon.cpu) = %+v, %v", cpu, ok)
	}
	if _, ok := mini.Icon("icon.nope"); ok {
		t.Fatal("an unknown icon was found")
	}
	if mini.IconsFile() != "icons.json" {
		t.Fatalf("IconsFile() = %q", mini.IconsFile())
	}
	ids := mini.IconIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("IconIDs not sorted: %v", ids)
		}
	}

	rc := loadReal(t)
	if n := len(rc.IconIDs()); n < 490 || n > 530 {
		t.Fatalf("real icons = %d, want 490..530", n)
	}
	if rc.IconsFile() != "icons/lucide.json" {
		t.Fatalf("real IconsFile() = %q", rc.IconsFile())
	}

	manifest, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	icons, err := os.ReadFile("testdata/catalog-mini/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, from, to, want string }{
		{"duplicate id", `"id":"icon.database"`, `"id":"icon.cpu"`, "icon.cpu"},
		{"unsorted", `"id":"icon.cpu"`, `"id":"icon.zzz"`, "sorted"},
		{"bad id", `"id":"icon.mail"`, `"id":"icon.Mail"`, "icon.Mail"},
		{"bad schema", `storymaker-icons/1`, `storymaker-icons/9`, "schema"},
		{"bad path", `"M12 20v2"`, `"M12 20 <script>"`, "path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(string(icons), tc.from, tc.to, 1)
			if text == string(icons) {
				t.Fatal("the edit changed nothing")
			}
			fsys := fstest.MapFS{"catalog.json": {Data: manifest}, "icons.json": {Data: []byte(text)}}
			_, err := LoadCatalog(fsys, "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadCatalog error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
```

In `internal/storymaker/testdata/catalog-mini/catalog.json`, replace:

```json
{"schema":"storymaker-catalog/1","version":"mini00000000",
 "stage":{"width":1920,"height":1080,"safe":{"x":5,"y":5,"w":90,"h":90}},
 "rig":{"id":"rig.human/1","file":"rig/human.json"},
 "fonts":[{"name":"default","family":"Storymaker Default","weight":400,"file":"/fonts/placeholder.woff2","licence":"OFL-1.1"}],
 "palettes":[{"id":"palette.sunny","name":"sunny","name_key":"storymaker.catalog.palette_sunny","tokens":{"bg":"#fff7e6","bg2":"#ffe0a3","fg":"#1f2a44","accent":"#ff7a45","accent2":"#36cfc9","text":"#1f2a44","outline":"#1f2a44"}}],
 "entries":[
```

with:

```json
{"schema":"storymaker-catalog/1","version":"mini00000000",
 "stage":{"width":1920,"height":1080,"safe":{"x":5,"y":5,"w":90,"h":90}},
 "rig":{"id":"rig.human/1","file":"rig/human.json"},
 "icons":{"file":"icons.json"},
 "fonts":[{"name":"default","family":"Storymaker Default","weight":400,"file":"/fonts/placeholder.woff2","licence":"OFL-1.1"}],
 "palettes":[{"id":"palette.sunny","name":"sunny","name_key":"storymaker.catalog.palette_sunny","tokens":{"bg":"#fff7e6","bg2":"#ffe0a3","fg":"#1f2a44","accent":"#ff7a45","accent2":"#36cfc9","text":"#1f2a44","outline":"#1f2a44"}}],
 "entries":[
```

Create `internal/storymaker/testdata/catalog-mini/icons.json`:

```json
{"schema":"storymaker-icons/1","source":"lucide-static@1.82.0","grid":24,"stroke":2,"icons":[
{"id":"icon.cpu","paths":["M12 20v2","M12 2v2","M17 20v2","M17 2v2","M2 12h2","M2 17h2","M2 7h2","M20 12h2","M20 17h2","M20 7h2","M7 20v2","M7 2v2","M6 4H18A2 2 0 0 1 20 6V18A2 2 0 0 1 18 20H6A2 2 0 0 1 4 18V6A2 2 0 0 1 6 4Z","M9 8H15A1 1 0 0 1 16 9V15A1 1 0 0 1 15 16H9A1 1 0 0 1 8 15V9A1 1 0 0 1 9 8Z"],"tags":["processor","cores","technology","computer","chip","circuit","memory","ram","specs","gigahertz","ghz","cpu","devices"]},
{"id":"icon.database","paths":["M3 5A9 3 0 1 0 21 5A9 3 0 1 0 3 5Z","M3 5V19A9 3 0 0 0 21 19V5","M3 12A9 3 0 0 0 21 12"],"tags":["storage","memory","container","tin","pot","bytes","servers","sql","db","query","sqlite","mysql","backup","export","import","dump","restore","migration","schema","records","database","devices"]},
{"id":"icon.lock","paths":["M5 11H19A2 2 0 0 1 21 13V20A2 2 0 0 1 19 22H5A2 2 0 0 1 3 20V13A2 2 0 0 1 5 11Z","M7 11V7a5 5 0 0 1 10 0v4"],"tags":["lock","padlock","private","privacy","protection","password","secure","security","safety","protected","restricted","encrypted","authorization","read-only","blocked","locked","admin"]},
{"id":"icon.mail","paths":["m22 7-8.991 5.727a2 2 0 0 1-2.009 0L2 7","M4 4H20A2 2 0 0 1 22 6V18A2 2 0 0 1 20 20H4A2 2 0 0 1 2 18V6A2 2 0 0 1 4 4Z"],"tags":["email","message","letter","unread","mail","communication"]},
{"id":"icon.send","paths":["M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z","m21.854 2.147-10.94 10.939"],"tags":["email","message","mail","paper airplane","paper aeroplane","submit","invite","invitation","send","connectivity"]},
{"id":"icon.server","paths":["M4 2H20A2 2 0 0 1 22 4V8A2 2 0 0 1 20 10H4A2 2 0 0 1 2 8V4A2 2 0 0 1 4 2Z","M4 14H20A2 2 0 0 1 22 16V20A2 2 0 0 1 20 22H4A2 2 0 0 1 2 20V16A2 2 0 0 1 4 14Z","M6 6L6.01 6","M6 18L6.01 18"],"tags":["cloud","storage","server","devices"]}
]}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestLoadCatalogReadsIcons)' -count=1`
Expected: undefined: Catalog.IconIDs, Icon, IconsFile (build failed)

- [ ] **Step 3: Implement**

In `internal/storymaker/catalog.go`, replace:

```go

	glyphs     *Glyphs // nil when the manifest names no glyphs file
	glyphsFile string
}

type catalogFile struct {
```

with:

```go

	glyphs     *Glyphs // nil when the manifest names no glyphs file
	glyphsFile string

	icons     []Icon // sorted by ID; empty when the manifest names no icons file
	iconIdx   map[string]int
	iconsFile string
}

type catalogFile struct {
```

In `internal/storymaker/catalog.go`, replace:

```go
	Stage    Stage             `json:"stage"`
	Rig      RigRef            `json:"rig"`
	Glyphs   *glyphsRef        `json:"glyphs"`
	Fonts    []Font            `json:"fonts"`
	Palettes []Palette         `json:"palettes"`
	Entries  []json.RawMessage `json:"entries"`
```

with:

```go
	Stage    Stage             `json:"stage"`
	Rig      RigRef            `json:"rig"`
	Glyphs   *glyphsRef        `json:"glyphs"`
	Icons    *iconsRef         `json:"icons"`
	Fonts    []Font            `json:"fonts"`
	Palettes []Palette         `json:"palettes"`
	Entries  []json.RawMessage `json:"entries"`
```

In `internal/storymaker/catalog.go`, replace:

```go
	if m.Glyphs != nil {
		c.loadGlyphs(fsys, path, m.Glyphs.File, bad)
	}
	c.checkFiles(bad)

	if len(problems) > 0 {
```

with:

```go
	if m.Glyphs != nil {
		c.loadGlyphs(fsys, path, m.Glyphs.File, bad)
	}
	if m.Icons != nil {
		c.loadIcons(fsys, path, m.Icons.File, bad)
	}
	c.checkFiles(bad)

	if len(problems) > 0 {
```

Create `internal/storymaker/icons.go`:

```go
package storymaker

import (
	"encoding/json"
	"io"
	"io/fs"
	"path"
	"regexp"
)

// IconsSchema is the schema of the icon data file (Lucide icons converted to path data on the 24 grid).
const IconsSchema = "storymaker-icons/1"

const maxIconsBytes = 1 << 20

var (
	iconIDPattern   = regexp.MustCompile(`^icon\.[a-z0-9-]{1,60}$`)
	iconPathPattern = regexp.MustCompile(`^[MmLlHhVvCcSsQqTtAaZz0-9 .,eE-]+$`)
)

// Icon is one stroked icon: path strings on a 24 grid. Fills are the elements drawn filled and stroked.
type Icon struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
	Fills []string `json:"fills,omitempty"`
	Tags  []string `json:"tags"`
}

// iconsRef is the manifest key that names the icon data file.
type iconsRef struct {
	File string `json:"file"`
}

type iconsDoc struct {
	Schema string `json:"schema"`
	Icons  []Icon `json:"icons"`
}

// Icon returns the icon with the ID, for example "icon.cpu".
func (c *Catalog) Icon(id string) (Icon, bool) {
	i, ok := c.iconIdx[id]
	if !ok {
		return Icon{}, false
	}
	return c.icons[i], true
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	ids := make([]string, len(c.icons))
	for i, ic := range c.icons {
		ids[i] = ic.ID
	}
	return ids
}

// IconsFile returns the icon data file name, relative to the folder of catalog.json, or "".
func (c *Catalog) IconsFile() string { return c.iconsFile }

func (c *Catalog) loadIcons(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
		bad("icons needs a relative file path")
		return
	}
	full := path.Join(path.Dir(catalogPath), file)
	f, err := fsys.Open(full)
	if err != nil {
		bad("open icons: %v", err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxIconsBytes+1))
	if err != nil {
		bad("read icons %s: %v", full, err)
		return
	}
	if len(data) > maxIconsBytes {
		bad("icons %s is larger than %d bytes", full, maxIconsBytes)
		return
	}
	var doc iconsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		bad("parse icons %s: %v", full, err)
		return
	}
	if doc.Schema != IconsSchema {
		bad("icons %s: schema is %q, want %q", full, doc.Schema, IconsSchema)
		return
	}
	for i, ic := range doc.Icons {
		if !iconIDPattern.MatchString(ic.ID) {
			bad("icons: id %q must match icon.[a-z0-9-]", ic.ID)
			continue
		}
		if i > 0 && doc.Icons[i-1].ID >= ic.ID {
			bad("icons: %s is a duplicate or not sorted after %s", ic.ID, doc.Icons[i-1].ID)
		}
		if n := len(ic.Paths); n < 1 || n > 40 {
			bad("icons: %s has %d paths, want 1 to 40", ic.ID, n)
		}
		for _, p := range append(append([]string(nil), ic.Paths...), ic.Fills...) {
			if len(p) > 2000 || !iconPathPattern.MatchString(p) {
				bad("icons: %s has an invalid path string", ic.ID)
				break
			}
		}
		if len(ic.Tags) > 32 {
			bad("icons: %s has %d tags, want at most 32", ic.ID, len(ic.Tags))
		}
	}
	c.icons, c.iconsFile = doc.Icons, file
	c.iconIdx = make(map[string]int, len(doc.Icons))
	for i, ic := range doc.Icons {
		c.iconIdx[ic.ID] = i
	}
}
```

In `internal/storymaker/render/render.go`, replace:

```go
	if glyphs := cat.GlyphsFile(); glyphs != "" {
		files[glyphs] = true
	}
	for _, e := range cat.Entries() {
		if e.File != "" && e.Kind != "sfx" { // the page never reads WAVs
			files[e.File] = true
```

with:

```go
	if glyphs := cat.GlyphsFile(); glyphs != "" {
		files[glyphs] = true
	}
	if icons := cat.IconsFile(); icons != "" {
		files[icons] = true
	}
	for _, e := range cat.Entries() {
		if e.File != "" && e.Kind != "sfx" { // the page never reads WAVs
			files[e.File] = true
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestLoadCatalogReadsIcons)' -count=1`
Expected: PASS (6 mini icons, 516 real icons, 5 broken-copy subtests); go test ./internal/storymaker/... ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/catalog.go internal/storymaker/catalog_models_test.go internal/storymaker/glyphs_test.go internal/storymaker/icons.go internal/storymaker/icons_test.go internal/storymaker/render/render.go internal/storymaker/testdata/catalog-mini/catalog.json internal/storymaker/testdata/catalog-mini/icons.json
git commit -m "storymaker: Go loads the icon file"
```

### Task B5: catalog search and describe return icons

**Files:**
- Modify: `internal/agent/native_tools_storymaker.go`
- Modify: `internal/storymaker/catalog.go`
- Modify: `internal/storymaker/catalog_test.go`
- Modify: `internal/storymaker/icons.go`
- Modify: `internal/storymaker/icons_test.go`
- Modify: `internal/storymaker/studio_catalog.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/catalog_test.go`, replace:

```go
		limit       int
		want        []string
	}{
		{"padlock", "", 0, []string{"prop.lock"}},                              // tag
		{"lock", "prop", 0, []string{"prop.lock"}},                             // id and tag, kind filter
		{"security", "prop", 0, []string{"prop.lock", "prop.shield"}},          // tie keeps catalog order
		{"prop.shield", "", 0, []string{"prop.shield"}},                        // exact id first
```

with:

```go
		limit       int
		want        []string
	}{
		{"padlock", "", 0, []string{"prop.lock", "icon.lock"}},                 // tag; icons follow the entries
		{"lock", "prop", 0, []string{"prop.lock"}},                             // id and tag, kind filter
		{"security", "prop", 0, []string{"prop.lock", "prop.shield"}},          // tie keeps catalog order
		{"prop.shield", "", 0, []string{"prop.shield"}},                        // exact id first
```

In `internal/storymaker/icons_test.go`, replace:

```go
package storymaker

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
```

with:

```go
package storymaker

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
```

In `internal/storymaker/icons_test.go`, replace:

```go
		})
	}
}
```

with:

```go
		})
	}
}

// newIconStudio returns a Studio over the real catalog in the phase, for the icon search tests.
func newIconStudio(t *testing.T, mode, phase string) *Studio {
	t.Helper()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(context.Background(), "Icons", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Compile: func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		},
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Language: "en", TargetSeconds: 60, Mode: mode})
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(phase, "")
	return studio
}

func iconAnswer(t *testing.T, s *Studio, op string, args map[string]any) map[string]any {
	t.Helper()
	res := s.Call(context.Background(), StudioCall{Tool: ToolCatalog, Operation: op, Args: args})
	var answer map[string]any
	if err := json.Unmarshal(res.JSON, &answer); err != nil {
		t.Fatalf("answer %s: %v", res.JSON, err)
	}
	return answer
}

func TestSearchFindsIcons(t *testing.T) {
	cat := loadReal(t)
	if NormalizeKind("icon") != "icon" || NormalizeKind("Icons") != "icon" {
		t.Fatal("NormalizeKind must accept icon and icons")
	}
	ids := func(es []Entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.ID
		}
		return out
	}
	if got := ids(cat.Search("processor", "icon", 6)); !slices.Contains(got, "icon.cpu") {
		t.Fatalf("Search(processor, icon) = %v", got)
	}
	both := ids(cat.Search("lock", "", 12))
	prop, icon := slices.Index(both, "prop.lock"), slices.Index(both, "icon.lock")
	if prop < 0 || icon < 0 || prop > icon {
		t.Fatalf("Search(lock) = %v, want prop.lock before icon.lock", both)
	}
	if slices.Contains(ids(cat.Search("processor", "prop", 6)), "icon.cpu") {
		t.Fatal("a prop search must not return icons")
	}

	studio := newIconStudio(t, ModeCatalog, PhaseFill)
	found := iconAnswer(t, studio, "search", map[string]any{"query": "processor", "kind": "icon"})
	results, _ := found["results"].([]any)
	var cpu map[string]any
	for _, r := range results {
		if m := r.(map[string]any); m["id"] == "icon.cpu" {
			cpu = m
		}
	}
	if cpu == nil || cpu["kind"] != "icon" {
		t.Fatalf("search answer = %v", found)
	}
	example, _ := cpu["usage_example"].(map[string]any)
	if example["do"] != "show" || example["icon"] != "icon.cpu" || example["style"] != "card" || example["text"] != "Cpu" || example["at"] != "board" || example["entry"] != "draw" {
		t.Fatalf("usage_example = %v", cpu["usage_example"])
	}
	described := iconAnswer(t, studio, "describe", map[string]any{"id": "icon.cpu"})
	entry, _ := described["entry"].(map[string]any)
	if entry["id"] != "icon.cpu" || entry["kind"] != "icon" {
		t.Fatalf("describe answer = %v", described)
	}
	hit := studio.catalogHitOf(cat.Search("server-cog", "icon", 1)[0])
	if !strings.Contains(string(hit.UsageExample), `"text":"Server cog"`) {
		t.Fatalf("label of a dashed name: %s", hit.UsageExample)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestSearchFindsIcons)' -count=1`
Expected: FAIL icons_test.go:110 NormalizeKind must accept icon and icons

- [ ] **Step 3: Implement**

In `internal/agent/native_tools_storymaker.go`, replace:

```go
				"topic":     storymakerEnum("code"),
			}, "operation"))
	}
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx"}
	if !compact || phase != storymaker.PhaseCast {
		kinds = append(kinds, "voice")
	}
```

with:

```go
				"topic":     storymakerEnum("code"),
			}, "operation"))
	}
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx", "icon"}
	if !compact || phase != storymaker.PhaseCast {
		kinds = append(kinds, "voice")
	}
```

In `internal/storymaker/catalog.go`, replace:

```go
	"poses": "pose", "gestures": "gesture", "expr": "expression", "expressions": "expression",
	"mouths": "mouth", "cycles": "cycle", "text": "text_style", "text_styles": "text_style",
	"trans": "transition", "transitions": "transition", "palettes": "palette", "fonts": "font",
}

// NormalizeKind returns the canonical catalog kind for a kind, alias or ID prefix, or "".
```

with:

```go
	"poses": "pose", "gestures": "gesture", "expr": "expression", "expressions": "expression",
	"mouths": "mouth", "cycles": "cycle", "text": "text_style", "text_styles": "text_style",
	"trans": "transition", "transitions": "transition", "palettes": "palette", "fonts": "font",
	"icon": "icon", "icons": "icon",
}

// NormalizeKind returns the canonical catalog kind for a kind, alias or ID prefix, or "".
```

In `internal/storymaker/catalog.go`, replace:

```go
)

// Search finds entries by terms over their id, name and tags. kind restricts the result (a kind,
// alias or ID prefix; an unknown kind finds nothing). limit defaults to 6 and is capped at 12.
// Without terms it lists the entries of the kind in catalog order. Results are sorted by score,
// then by catalog order.
func (c *Catalog) Search(query, kind string, limit int) []Entry {
```

with:

```go
)

// Search finds entries by terms over their id, name and tags. kind restricts the result (a kind,
// alias or ID prefix; an unknown kind finds nothing). Icons answer to kind icon or no kind, after all entries. limit defaults to 6 and is capped at 12.
// Without terms it lists the entries of the kind in catalog order. Results are sorted by score,
// then by catalog order.
func (c *Catalog) Search(query, kind string, limit int) []Entry {
```

In `internal/storymaker/catalog.go`, replace:

```go
		}
		hits = append(hits, scored{e, score, i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
```

with:

```go
		}
		hits = append(hits, scored{e, score, i})
	}
	if kind == "" || kind == "icon" {
		for i, ic := range c.icons {
			e := iconEntry(ic)
			score := 1
			if len(terms) > 0 {
				score = 0
				for _, t := range terms {
					score += termScore(e, t)
				}
				if score == 0 {
					continue
				}
			}
			hits = append(hits, scored{e, score, len(c.entries) + i})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
```

In `internal/storymaker/icons.go`, replace:

```go
	"io/fs"
	"path"
	"regexp"
)

// IconsSchema is the schema of the icon data file (Lucide icons converted to path data on the 24 grid).
```

with:

```go
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// IconsSchema is the schema of the icon data file (Lucide icons converted to path data on the 24 grid).
```

In `internal/storymaker/icons.go`, replace:

```go

// Icon returns the icon with the ID, for example "icon.cpu".
func (c *Catalog) Icon(id string) (Icon, bool) {
	i, ok := c.iconIdx[id]
	if !ok {
		return Icon{}, false
```

with:

```go

// Icon returns the icon with the ID, for example "icon.cpu".
func (c *Catalog) Icon(id string) (Icon, bool) {
	if c == nil {
		return Icon{}, false
	}
	i, ok := c.iconIdx[id]
	if !ok {
		return Icon{}, false
```

In `internal/storymaker/icons.go`, replace:

```go
	return c.icons[i], true
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	ids := make([]string, len(c.icons))
	for i, ic := range c.icons {
		ids[i] = ic.ID
```

with:

```go
	return c.icons[i], true
}

// iconEntry is the search view of an icon: the name is the ID, so a tie keeps catalog entries first.
func iconEntry(ic Icon) Entry {
	return Entry{ID: ic.ID, Kind: "icon", Name: ic.ID, Tags: ic.Tags}
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, len(c.icons))
	for i, ic := range c.icons {
		ids[i] = ic.ID
```

In `internal/storymaker/icons.go`, replace:

```go
}

// IconsFile returns the icon data file name, relative to the folder of catalog.json, or "".
func (c *Catalog) IconsFile() string { return c.iconsFile }

func (c *Catalog) loadIcons(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
```

with:

```go
}

// IconsFile returns the icon data file name, relative to the folder of catalog.json, or "".
func (c *Catalog) IconsFile() string {
	if c == nil {
		return ""
	}
	return c.iconsFile
}

func (c *Catalog) loadIcons(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
```

In `internal/storymaker/icons.go`, replace:

```go
		c.iconIdx[ic.ID] = i
	}
}
```

with:

```go
		c.iconIdx[ic.ID] = i
	}
}

// iconLabel turns the Lucide name of an icon ID into a label: "server-cog" becomes "Server cog".
func iconLabel(id string) string {
	name := strings.ReplaceAll(strings.TrimPrefix(id, "icon."), "-", " ")
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// iconUsageExample is the example a catalog search hit carries for an icon.
func iconUsageExample(id string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Do    string `json:"do"`
		Icon  string `json:"icon"`
		Style string `json:"style"`
		Text  string `json:"text"`
		At    string `json:"at"`
		Entry string `json:"entry"`
	}{"show", id, "card", iconLabel(id), "board", "draw"})
	return data
}
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	catalogVoiceKind          = "voice"
	catalogMissNextAction     = "Search a concrete English concept or synonym in the appropriate kind. Use an alternative only when its meaning fits; never invent an ID."
	catalogPropMissNextAction = "Search one concrete English synonym with kind prop. If no asset fits, read storymaker_catalog reference topic code, then create the missing motif as SVG paths with g.path and combine it with fitting catalog props. Use get_code, put_code and put_scene kind code as documented. Keep speaking-actor scenes and their lip-sync; use off-screen narration for custom illustration scenes. Alternatives are spelling suggestions, not semantic matches."
	catalogKindList           = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx or voice"
)

// catalogModelKinds are the kinds a miss may suggest when the call named no kind.
```

with:

```go
	catalogVoiceKind          = "voice"
	catalogMissNextAction     = "Search a concrete English concept or synonym in the appropriate kind. Use an alternative only when its meaning fits; never invent an ID."
	catalogPropMissNextAction = "Search one concrete English synonym with kind prop. If no asset fits, read storymaker_catalog reference topic code, then create the missing motif as SVG paths with g.path and combine it with fitting catalog props. Use get_code, put_code and put_scene kind code as documented. Keep speaking-actor scenes and their lip-sync; use off-screen narration for custom illustration scenes. Alternatives are spelling suggestions, not semantic matches."
	catalogKindList           = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx, icon or voice"
)

// catalogModelKinds are the kinds a miss may suggest when the call named no kind.
```

In `internal/storymaker/studio_catalog.go`, replace:

```go

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	entry, found := s.deps.Catalog.Lookup(target)
	if !found && kind != "" {
		entry, found = s.deps.Catalog.Lookup(kindPrefixes[kind] + target)
```

with:

```go

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	if ic, ok := s.deps.Catalog.Icon(target); ok && (kind == "" || kind == "icon") {
		return catalogOpAnswer(op, "ok", map[string]any{"entry": s.catalogHitOf(iconEntry(ic))})
	}
	if ic, ok := s.deps.Catalog.Icon("icon." + target); ok && kind == "icon" {
		return catalogOpAnswer(op, "ok", map[string]any{"entry": s.catalogHitOf(iconEntry(ic))})
	}
	entry, found := s.deps.Catalog.Lookup(target)
	if !found && kind != "" {
		entry, found = s.deps.Catalog.Lookup(kindPrefixes[kind] + target)
```

In `internal/storymaker/studio_catalog.go`, replace:

```go

func (s *Studio) catalogHitOf(e Entry) catalogHit {
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if hit.Tags == nil {
		hit.Tags = []string{}
	}
```

with:

```go

func (s *Studio) catalogHitOf(e Entry) catalogHit {
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if e.Kind == "icon" {
		hit.UsageExample = iconUsageExample(e.ID)
	}
	if hit.Tags == nil {
		hit.Tags = []string{}
	}
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
				pool = append(pool, e.ID)
			}
		}
	}
	return catalogOpAnswer(op, "ok", map[string]any{
		"results": []catalogHit{}, "alternatives": closestNames(query, pool, 5), "next_action": nextAction,
```

with:

```go
				pool = append(pool, e.ID)
			}
		}
		if kind == "icon" {
			pool = s.deps.Catalog.IconIDs()
		}
	}
	return catalogOpAnswer(op, "ok", map[string]any{
		"results": []catalogHit{}, "alternatives": closestNames(query, pool, 5), "next_action": nextAction,
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestSearchFindsIcons)' -count=1`
Expected: PASS; go test ./internal/storymaker/... and go test ./internal/agent -run '(?i)storymaker' ok

- [ ] **Step 5: Commit**

```bash
git add internal/agent/native_tools_storymaker.go internal/storymaker/catalog.go internal/storymaker/catalog_test.go internal/storymaker/icons.go internal/storymaker/icons_test.go internal/storymaker/studio_catalog.go
git commit -m "storymaker: catalog search and describe return icons"
```

### Task B6: free mode searches icons only

**Files:**
- Modify: `internal/agent/native_tools_storymaker.go`
- Modify: `internal/agent/native_tools_storymaker_free_test.go`
- Modify: `internal/storymaker/catalog.go`
- Modify: `internal/storymaker/icons.go`
- Modify: `internal/storymaker/studio.go`
- Modify: `internal/storymaker/studio_catalog.go`
- Create: `internal/storymaker/studio_free_icons_test.go`
- Modify: `internal/storymaker/studio_mode_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/agent/native_tools_storymaker_free_test.go`, replace:

```go
		schemas := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, phase, false, names)
		catalog := storymakerToolParams(schemas, storymaker.ToolCatalog)
		operations, _ := storymakerSchemaAt(catalog, "operation")["enum"].([]string)
		if !slices.Equal(operations, []string{"reference"}) {
			t.Errorf("%s catalog operations = %v, want [reference]", phase, operations)
		}
		if storymakerSchemaAt(catalog, "query") != nil || storymakerSchemaAt(catalog, "kind") != nil {
			t.Errorf("%s: the free catalog tool must not offer query or kind", phase)
		}
		if phase == storymaker.PhaseCheck {
			continue
```

with:

```go
		schemas := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, phase, false, names)
		catalog := storymakerToolParams(schemas, storymaker.ToolCatalog)
		operations, _ := storymakerSchemaAt(catalog, "operation")["enum"].([]string)
		want := []string{"reference", "search"}
		if phase == storymaker.PhaseCheck {
			want = []string{"reference"}
		}
		if got := slices.Sorted(slices.Values(operations)); !slices.Equal(got, want) {
			t.Errorf("%s catalog operations = %v, want %v", phase, operations, want)
		}
		kindEnum, _ := storymakerSchemaAt(catalog, "kind")["enum"].([]string)
		if searches := slices.Contains(operations, "search"); searches != (storymakerSchemaAt(catalog, "query") != nil) || searches != slices.Equal(kindEnum, []string{"icon"}) {
			t.Errorf("%s: the free catalog tool offers query and kind icon exactly when it searches, kind enum = %v", phase, kindEnum)
		}
		if phase == storymaker.PhaseCheck {
			continue
```

Create `internal/storymaker/studio_free_icons_test.go`:

```go
package storymaker

import (
	"strings"
	"testing"
)

func TestFreeSearchFindsOnlyIcons(t *testing.T) {
	s := newIconStudio(t, ModeFree, PhaseFill)
	found := iconAnswer(t, s, "search", map[string]any{"kind": "prop", "query": "lock"})
	results, _ := found["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("free search answer = %v", found)
	}
	for _, r := range results {
		if id, _ := r.(map[string]any)["id"].(string); !strings.HasPrefix(id, "icon.") {
			t.Fatalf("free search returned %q, want icons only: %v", id, found)
		}
	}
	first := results[0].(map[string]any)
	example, _ := first["usage_example"].(map[string]any)
	code, _ := example["code"].(string)
	if first["id"] != "icon.lock" || !strings.HasPrefix(code, "api.icon('icon.lock'") || !strings.Contains(code, "label: 'Lock'") {
		t.Fatalf("first free hit = %v", first)
	}
	if _, has := example["do"]; has {
		t.Fatalf("a free usage_example must be code only: %v", example)
	}

	miss := iconAnswer(t, s, "search", map[string]any{"query": "zebra"})
	if empty, _ := miss["results"].([]any); len(empty) != 0 || !strings.Contains(miss["next_action"].(string), "No icon matches.") {
		t.Fatalf("free miss = %v", miss)
	}

	s.SetPhase(PhaseCheck, "")
	refused := iconAnswer(t, s, "search", map[string]any{"query": "lock"})
	if refused["code"] != "operation_not_in_phase" {
		t.Fatalf("free check phase must not search: %v", refused)
	}
}
```

In `internal/storymaker/studio_mode_test.go`, replace:

```go
	return studio, ps, proj.ID
}

func TestFreePhaseTablesDropTheCatalogSearchAndTheCast(t *testing.T) {
	if got := PhaseToolsFor(ModeFree, PhaseCast); len(got) != 0 {
		t.Fatalf("free cast tools = %v, want none", got)
	}
	for _, phase := range []string{PhaseFill, PhaseCheck, PhaseRevise} {
		if got := PhaseOperationsFor(ModeFree, phase, ToolCatalog); !slices.Equal(got, []string{"reference"}) {
			t.Errorf("free %s catalog operations = %v, want [reference]", phase, got)
		}
	}
	if slices.Contains(PhaseOperationsFor(ModeFree, PhaseFill, ToolStory), "set_meta") {
```

with:

```go
	return studio, ps, proj.ID
}

func TestFreePhaseTablesOfferOnlyIconSearchAndNoCast(t *testing.T) {
	if got := PhaseToolsFor(ModeFree, PhaseCast); len(got) != 0 {
		t.Fatalf("free cast tools = %v, want none", got)
	}
	for _, phase := range []string{PhaseFill, PhaseCheck, PhaseRevise} {
		want := []string{"search", "reference"}
		if phase == PhaseCheck {
			want = []string{"reference"}
		}
		if got := PhaseOperationsFor(ModeFree, phase, ToolCatalog); !slices.Equal(got, want) {
			t.Errorf("free %s catalog operations = %v, want %v", phase, got, want)
		}
	}
	if slices.Contains(PhaseOperationsFor(ModeFree, PhaseFill, ToolStory), "set_meta") {
```

In `internal/storymaker/studio_mode_test.go`, replace:

```go
	}
}

func TestFreeStudioRefusesCatalogSearchAndTheCastPhase(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	res := s.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "search", Args: map[string]any{"kind": "prop", "query": "lock"}})
	if got := answerOf(t, res); got["code"] != "operation_not_in_phase" {
		t.Fatalf("catalog search in a free project: %s", res.JSON)
```

with:

```go
	}
}

func TestFreeStudioRefusesCatalogSearchInCheckAndTheCastPhase(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseCheck, "")
	res := s.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "search", Args: map[string]any{"kind": "prop", "query": "lock"}})
	if got := answerOf(t, res); got["code"] != "operation_not_in_phase" {
		t.Fatalf("catalog search in a free project: %s", res.JSON)
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeSearchFindsOnlyIcons)' -count=1; go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: free search refused (operation_not_in_phase, search is not offered); agent free catalog operations [reference], want [reference search]

- [ ] **Step 3: Implement**

In `internal/agent/native_tools_storymaker.go`, replace:

```go

func storymakerCatalogTool(mode, phase string, compact bool) openai.Tool {
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		return tool(storymaker.ToolCatalog,
			"Read the code reference with operation reference and topic code. A free project has no catalog assets.",
			schema(map[string]interface{}{
				"operation": storymakerEnum(storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolCatalog)...),
				"topic":     storymakerEnum("code"),
			}, "operation"))
	}
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx", "icon"}
	if !compact || phase != storymaker.PhaseCast {
```

with:

```go

func storymakerCatalogTool(mode, phase string, compact bool) openai.Tool {
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		operations := storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolCatalog)
		properties := map[string]interface{}{
			"operation": storymakerEnum(operations...),
			"topic":     storymakerEnum("code"),
		}
		description := "Read the code reference with operation reference and topic code. A free project has no catalog assets."
		if slices.Contains(operations, "search") {
			properties["query"] = storymakerTyped("string")
			properties["kind"] = storymakerEnum("icon")
			properties["limit"] = map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12}
			description = "Search icons (kind icon, English query) or read the code reference with operation reference and topic code. A free project has no other catalog assets."
		}
		return tool(storymaker.ToolCatalog, description, schema(properties, "operation"))
	}
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx", "icon"}
	if !compact || phase != storymaker.PhaseCast {
```

In `internal/storymaker/catalog.go`, replace:

```go
	if kind == "" || kind == "icon" {
		for i, ic := range c.icons {
			e := iconEntry(ic)
			score := 1
			if len(terms) > 0 {
				score = 0
				for _, t := range terms {
					score += termScore(e, t)
				}
				if score == 0 {
					continue
				}
			}
			hits = append(hits, scored{e, score, len(c.entries) + i})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
```

with:

```go
	if kind == "" || kind == "icon" {
		for i, ic := range c.icons {
			e := iconEntry(ic)
			score, order := 1, len(c.entries)+len(c.icons)+i
			if len(terms) > 0 {
				score = 0
				for _, t := range terms {
					score += iconTermScore(e, t)
					if t == strings.TrimPrefix(e.ID, "icon.") {
						order = len(c.entries) + i // an exact name leads the icons of its score
					}
				}
				if score == 0 {
					continue
				}
			}
			hits = append(hits, scored{e, score, order})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
```

In `internal/storymaker/icons.go`, replace:

```go
	return Entry{ID: ic.ID, Kind: "icon", Name: ic.ID, Tags: ic.Tags}
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	if c == nil {
```

with:

```go
	return Entry{ID: ic.ID, Kind: "icon", Name: ic.ID, Tags: ic.Tags}
}

// iconTermScore is termScore, but an exact name counts like an exact tag: the term "lock" finds icon.lock before
// icon.lock-open. A catalog entry with an exact tag (prop.lock) still comes first, because it sorts first on a tie.
func iconTermScore(e Entry, term string) int {
	score := termScore(e, term)
	if term == strings.TrimPrefix(e.ID, "icon.") {
		score = max(score, 4)
	}
	return score
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	if c == nil {
```

In `internal/storymaker/icons.go`, replace:

```go
	}{"show", id, "card", iconLabel(id), "board", "draw"})
	return data
}
```

with:

```go
	}{"show", id, "card", iconLabel(id), "board", "draw"})
	return data
}

// iconFreeUsageExample is the example a free project gets: a line of scene code.
func iconFreeUsageExample(id string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Code string `json:"code"`
	}{"api.icon('" + id + "', {style: 'card', label: '" + iconLabel(id) + "', at: 'right'})"})
	return data
}
```

In `internal/storymaker/studio.go`, replace:

```go
		ToolStory: {"outline", "get"},
	},
	PhaseFill: {
		ToolCatalog: {"reference"},
		ToolStory:   {"put_scene", "patch_scene", "dry_run", "get", "get_code", "put_code"},
	},
	PhaseCheck: {
```

with:

```go
		ToolStory: {"outline", "get"},
	},
	PhaseFill: {
		ToolCatalog: {"search", "reference"},
		ToolStory:   {"put_scene", "patch_scene", "dry_run", "get", "get_code", "put_code"},
	},
	PhaseCheck: {
```

In `internal/storymaker/studio.go`, replace:

```go
		ToolCatalog: {"reference"},
	},
	PhaseRevise: {
		ToolCatalog: {"reference"},
		ToolStory:   {"get", "put_scene", "patch_scene", "remove_scene", "move_scene", "set_meta", "dry_run", "get_code", "put_code"},
		ToolCheck:   {"validate"},
	},
```

with:

```go
		ToolCatalog: {"reference"},
	},
	PhaseRevise: {
		ToolCatalog: {"search", "reference"},
		ToolStory:   {"get", "put_scene", "patch_scene", "remove_scene", "move_scene", "set_meta", "dry_run", "get_code", "put_code"},
		ToolCheck:   {"validate"},
	},
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	catalogVoiceKind          = "voice"
	catalogMissNextAction     = "Search a concrete English concept or synonym in the appropriate kind. Use an alternative only when its meaning fits; never invent an ID."
	catalogPropMissNextAction = "Search one concrete English synonym with kind prop. If no asset fits, read storymaker_catalog reference topic code, then create the missing motif as SVG paths with g.path and combine it with fitting catalog props. Use get_code, put_code and put_scene kind code as documented. Keep speaking-actor scenes and their lip-sync; use off-screen narration for custom illustration scenes. Alternatives are spelling suggestions, not semantic matches."
	catalogKindList           = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx, icon or voice"
)
```

with:

```go
	catalogVoiceKind          = "voice"
	catalogMissNextAction     = "Search a concrete English concept or synonym in the appropriate kind. Use an alternative only when its meaning fits; never invent an ID."
	catalogPropMissNextAction = "Search one concrete English synonym with kind prop. If no asset fits, read storymaker_catalog reference topic code, then create the missing motif as SVG paths with g.path and combine it with fitting catalog props. Use get_code, put_code and put_scene kind code as documented. Keep speaking-actor scenes and their lip-sync; use off-screen narration for custom illustration scenes. Alternatives are spelling suggestions, not semantic matches."
	catalogFreeMissNextAction = "No icon matches. Search one concrete English word with kind icon, or draw the motif with g.path."
	catalogKindList           = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx, icon or voice"
)
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	query := catalogOpArg(args, "query")
	id := catalogOpArg(args, "id")
	rawKind := catalogOpArg(args, "kind")
	if strings.EqualFold(rawKind, catalogVoiceKind) {
		return s.catalogVoiceSearch(ctx, op, query, id, catalogOpLimit(args))
	}
```

with:

```go
	query := catalogOpArg(args, "query")
	id := catalogOpArg(args, "id")
	rawKind := catalogOpArg(args, "kind")
	if s.freeMode() {
		rawKind = "icon" // a free project has no other catalog assets
	}
	if strings.EqualFold(rawKind, catalogVoiceKind) {
		return s.catalogVoiceSearch(ctx, op, query, id, catalogOpLimit(args))
	}
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	return catalogOpAnswer(op, "ok", map[string]any{"results": results})
}

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	if ic, ok := s.deps.Catalog.Icon(target); ok && (kind == "" || kind == "icon") {
```

with:

```go
	return catalogOpAnswer(op, "ok", map[string]any{"results": results})
}

// freeMode reports whether the job is a free project.
func (s *Studio) freeMode() bool { return EffectiveMode(s.job.Mode) == ModeFree }

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	if ic, ok := s.deps.Catalog.Icon(target); ok && (kind == "" || kind == "icon") {
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if e.Kind == "icon" {
		hit.UsageExample = iconUsageExample(e.ID)
	}
	if hit.Tags == nil {
		hit.Tags = []string{}
```

with:

```go
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if e.Kind == "icon" {
		hit.UsageExample = iconUsageExample(e.ID)
		if s.freeMode() {
			hit.UsageExample = iconFreeUsageExample(e.ID)
		}
	}
	if hit.Tags == nil {
		hit.Tags = []string{}
```

In `internal/storymaker/studio_catalog.go`, replace:

```go
	if kind == "prop" || (kind == "" && strings.HasPrefix(query, "prop.")) {
		nextAction = catalogPropMissNextAction
	}
	var pool []string
	if s.deps.Catalog != nil {
		for _, e := range s.deps.Catalog.Entries() {
```

with:

```go
	if kind == "prop" || (kind == "" && strings.HasPrefix(query, "prop.")) {
		nextAction = catalogPropMissNextAction
	}
	if s.freeMode() {
		nextAction = catalogFreeMissNextAction
	}
	var pool []string
	if s.deps.Catalog != nil {
		for _, e := range s.deps.Catalog.Entries() {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeSearchFindsOnlyIcons)' -count=1; go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: both PASS; go test ./internal/storymaker/... ok

- [ ] **Step 5: Commit**

```bash
git add internal/agent/native_tools_storymaker.go internal/agent/native_tools_storymaker_free_test.go internal/storymaker/catalog.go internal/storymaker/icons.go internal/storymaker/studio.go internal/storymaker/studio_catalog.go internal/storymaker/studio_free_icons_test.go internal/storymaker/studio_mode_test.go
git commit -m "storymaker: free mode searches icons only"
```

### Task B7: the icon build checks the package it reads; rect ry follows the raw rx

**Files:**
- Modify: `assets/storymaker/lib/icons.mjs`
- Modify: `assets/storymaker/test/icons.test.mjs`
- Modify: `internal/agent/native_tools_storymaker_free_test.go`

- [ ] **Step 1: Write the failing test**

In `assets/storymaker/test/icons.test.mjs`, replace:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { nodesToPaths, fmt, PATH_CHARS, CATEGORIES, LUCIDE_INTEGRITY } from '../lib/icons.mjs';

const iconsDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'icons');
```

with:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { nodesToPaths, buildIconData, fmt, PATH_CHARS, CATEGORIES, LUCIDE_INTEGRITY, LUCIDE_VERSION } from '../lib/icons.mjs';

const iconsDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'icons');
```

In `assets/storymaker/test/icons.test.mjs`, replace:

```js
  assert.ok(notice.includes(`${ids.length} icons`));
  assert.ok(fs.readFileSync(path.join(iconsDir, 'LICENSE.txt'), 'utf8').includes('ISC License'));
});
```

with:

```js
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
```

In `internal/agent/native_tools_storymaker_free_test.go`, replace:

```go
	return out
}

func TestStorymakerFreeSchemasOfferNoCatalogSearchNoCastAndCodeScenesOnly(t *testing.T) {
	names := StorymakerNames{}
	if got := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, storymaker.PhaseCast, false, names); len(got) != 0 {
		t.Fatalf("a free project has no cast phase, got %d tools", len(got))
```

with:

```go
	return out
}

func TestStorymakerFreeSchemasOfferIconSearchNoCastAndCodeScenesOnly(t *testing.T) {
	names := StorymakerNames{}
	if got := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, storymaker.PhaseCast, false, names); len(got) != 0 {
		t.Fatalf("a free project has no cast phase, got %d tools", len(got))
```

- [ ] **Step 2: Run it and see it fail**

Run: `node --test assets/storymaker/test/icons.test.mjs; node scripts/build-storymaker-icons.mjs --check --package C:\disposable\sm-opt\lucide\pkg\package; go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: rect rx 30 in a 20x40 box gave ry 10 instead of 20; a 1.81.0 package was read without a complaint (ENOENT on icon-nodes.json)

- [ ] **Step 3: Implement**

In `assets/storymaker/lib/icons.mjs`, replace:

```js
        case 'ellipse': return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), num(attrs, 'rx', tag), num(attrs, 'ry', tag));
        case 'rect': {
            const x = num(attrs, 'x', tag), y = num(attrs, 'y', tag), w = num(attrs, 'width', tag), h = num(attrs, 'height', tag);
            let rx = attrs.rx === undefined ? undefined : num(attrs, 'rx', tag);
            let ry = attrs.ry === undefined ? undefined : num(attrs, 'ry', tag);
            rx = Math.min(rx ?? ry ?? 0, w / 2);
            ry = Math.min(ry ?? rx ?? 0, h / 2);
            if (!rx && !ry) return `M${fmt(x)} ${fmt(y)}H${fmt(x + w)}V${fmt(y + h)}H${fmt(x)}Z`;
            const a = `A${fmt(rx)} ${fmt(ry)} 0 0 1 `;
            return `M${fmt(x + rx)} ${fmt(y)}H${fmt(x + w - rx)}${a}${fmt(x + w)} ${fmt(y + ry)}V${fmt(y + h - ry)}${a}${fmt(x + w - rx)} ${fmt(y + h)}H${fmt(x + rx)}${a}${fmt(x)} ${fmt(y + h - ry)}V${fmt(y + ry)}${a}${fmt(x + rx)} ${fmt(y)}Z`;
```

with:

```js
        case 'ellipse': return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), num(attrs, 'rx', tag), num(attrs, 'ry', tag));
        case 'rect': {
            const x = num(attrs, 'x', tag), y = num(attrs, 'y', tag), w = num(attrs, 'width', tag), h = num(attrs, 'height', tag);
            // Each radius defaults to the other one as written, then each is clamped to its own half side.
            const rawRx = attrs.rx === undefined ? undefined : num(attrs, 'rx', tag);
            const rawRy = attrs.ry === undefined ? undefined : num(attrs, 'ry', tag);
            const rx = Math.min(rawRx ?? rawRy ?? 0, w / 2);
            const ry = Math.min(rawRy ?? rawRx ?? 0, h / 2);
            if (!rx && !ry) return `M${fmt(x)} ${fmt(y)}H${fmt(x + w)}V${fmt(y + h)}H${fmt(x)}Z`;
            const a = `A${fmt(rx)} ${fmt(ry)} 0 0 1 `;
            return `M${fmt(x + rx)} ${fmt(y)}H${fmt(x + w - rx)}${a}${fmt(x + w)} ${fmt(y + ry)}V${fmt(y + h - ry)}${a}${fmt(x + w - rx)} ${fmt(y + h)}H${fmt(x + rx)}${a}${fmt(x)} ${fmt(y + h - ry)}V${fmt(y + ry)}${a}${fmt(x + rx)} ${fmt(y)}Z`;
```

In `assets/storymaker/lib/icons.mjs`, replace:

```js
// allowList: {schema, source, categories: {name: [ids]}} -> {json, notice, count, problems}
export function buildIconData(pkgDir, allowList) {
    const read = (f) => fs.readFileSync(path.join(pkgDir, f), 'utf8');
    const nodes = JSON.parse(read('icon-nodes.json'));
    const tags = JSON.parse(read('tags.json'));
    const license = read('LICENSE').replace(/\r\n/g, '\n');
    const problems = [];
    const cats = new Map();
    if (allowList.schema !== ALLOW_LIST_SCHEMA) problems.push(`allow-list schema is ${allowList.schema}`);
    for (const cat of CATEGORIES) for (const id of allowList.categories?.[cat] ?? []) {
```

with:

```js
// allowList: {schema, source, categories: {name: [ids]}} -> {json, notice, count, problems}
export function buildIconData(pkgDir, allowList) {
    const read = (f) => fs.readFileSync(path.join(pkgDir, f), 'utf8');
    // The notice names the pinned version and integrity, so the package must be that version.
    const problems = [];
    try {
        const meta = JSON.parse(read('package.json'));
        if (meta.name !== 'lucide-static' || meta.version !== LUCIDE_VERSION) {
            problems.push(`package.json is ${meta.name}@${meta.version}, expected ${LUCIDE_SOURCE}`);
        }
    } catch (e) { problems.push(`package.json cannot be read: ${e.message}`); }
    if (problems.length) return { json: '', notice: '', count: 0, problems, license: '' };
    const nodes = JSON.parse(read('icon-nodes.json'));
    const tags = JSON.parse(read('tags.json'));
    const license = read('LICENSE').replace(/\r\n/g, '\n');
    const cats = new Map();
    if (allowList.schema !== ALLOW_LIST_SCHEMA) problems.push(`allow-list schema is ${allowList.schema}`);
    for (const cat of CATEGORIES) for (const id of allowList.categories?.[cat] ?? []) {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `node --test assets/storymaker/test/icons.test.mjs; node scripts/build-storymaker-icons.mjs --check --package C:\disposable\sm-opt\lucide\pkg\package; go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: 5 node tests pass; icons up to date (516), lucide.json unchanged; fake 1.81.0 package refused; Go PASS

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/icons.mjs assets/storymaker/test/icons.test.mjs internal/agent/native_tools_storymaker_free_test.go
git commit -m "storymaker: the icon build checks the package it reads; rect ry follows the raw rx"
```

