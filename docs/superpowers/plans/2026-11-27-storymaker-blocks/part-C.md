# Part C: Player blocks, env.board and the board area

> Generated from the drafted and tested commits f9eb9c256..0831f87f9. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task C1: the player indexes and loads icon data

**Files:**
- Modify: `scripts/storymaker-test-support.mjs`
- Modify: `scripts/storymaker-tests/08-host-app.mjs`
- Create: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/desktop/apps/storymaker-assets.js`
- Modify: `ui/js/storymaker/host-app.js`
- Modify: `ui/js/storymaker/host-render.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/scene.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/08-host-app.mjs`, replace:

```js
    // Each range is two values in d, so 120002 values are 60001 ranges, one over maxGlyphRanges.
    assert.match(ok(glyphs({ schema, faces: [face(new Array(120002).fill(1))] })), /ranges/, 'maxGlyphRanges is 60000');
    assert.equal(ok(glyphs(readRepoJSON('ui/img/storymaker/glyphs.json'))), null, 'the shipped coverage file fits the caps');
});

check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
```

with:

```js
    // Each range is two values in d, so 120002 values are 60001 ranges, one over maxGlyphRanges.
    assert.match(ok(glyphs({ schema, faces: [face(new Array(120002).fill(1))] })), /ranges/, 'maxGlyphRanges is 60000');
    assert.equal(ok(glyphs(readRepoJSON('ui/img/storymaker/glyphs.json'))), null, 'the shipped coverage file fits the caps');

    const icons = data => load({ catalog: { ...catalog, icon_data: data } });
    const icon = (id, paths = ['M0 0']) => ({ id, paths, tags: [] });
    assert.match(ok(icons({ schema: 'x', icons: [] })), /icon_data/, 'another icon schema is refused');
    assert.equal(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu')] })), null, 'a valid icon passes');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('cpu')] })), /icon_data/, 'an id without the icon. prefix is refused');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', new Array(41).fill('M0 0'))] })), /icon_data/, 'at most 40 paths');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', ['M' + '0'.repeat(2000)])] })), /icon_data/, 'at most 2000 characters per path');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: new Array(1001).fill(icon('icon.cpu')) })), /icon_data/, 'at most 1000 icons');
    assert.equal(ok(icons(readRepoJSON('ui/img/storymaker/icons/lucide.json'))), null, 'the shipped icon file fits the caps');
});

check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
```

Create `scripts/storymaker-tests/33-blocks.mjs`:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;

check('blocks: indexCatalog indexes the icons', () => {
    const idx = t.core().scene.indexCatalog(t.catalog);
    assert.ok(idx.icons['icon.cpu'].paths.length > 0);
    assert.ok(Array.isArray(idx.icons['icon.cpu'].tags));
    assert.deepEqual(Object.keys(t.core().scene.indexCatalog({ entries: [] }).icons), []);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: blocks: indexCatalog indexes the icons failed (icons undefined); the host-app icon_data refusals had no check

- [ ] **Step 3: Implement**

In `scripts/storymaker-test-support.mjs`, replace:

```js
export const plain = value => JSON.parse(JSON.stringify(value));

export const MINI = 'internal/storymaker/testdata/catalog-mini/';
export const miniCatalog = () => readJSON(MINI + 'catalog.json');
export const miniFiles = () => ({
    rig: readJSON(MINI + 'rig/human.json'),
    props: { 'props/lock.json': readJSON(MINI + 'props/lock.json'), 'props/shield.json': readJSON(MINI + 'props/shield.json') },
```

with:

```js
export const plain = value => JSON.parse(JSON.stringify(value));

export const MINI = 'internal/storymaker/testdata/catalog-mini/';
// icon_data travels on the catalog in memory only, like glyph_data (the host loads it from the catalog's icons file)
export const miniCatalog = () => ({ ...readJSON(MINI + 'catalog.json'), icon_data: readJSON(MINI + 'icons.json') });
export const miniFiles = () => ({
    rig: readJSON(MINI + 'rig/human.json'),
    props: { 'props/lock.json': readJSON(MINI + 'props/lock.json'), 'props/shield.json': readJSON(MINI + 'props/shield.json') },
```

In `ui/js/desktop/apps/storymaker-assets.js`, replace:

```js
                catalogPromise = api.catalogJSON('catalog.json').then(async raw => {
                    // the coverage file travels on the catalog as glyph_data: in memory only, never in catalog.json
                    if (raw && raw.glyphs && typeof raw.glyphs.file === 'string') raw.glyph_data = await api.catalogJSON(raw.glyphs.file);
                    return raw;
                }).then(raw => {
                    if (!raw || !['storymaker-catalog/1', 'storymaker-catalog/2'].includes(raw.schema) || !Array.isArray(raw.entries)) throw S.errors.normalize({ message: 'The catalog has an unknown format.' });
```

with:

```js
                catalogPromise = api.catalogJSON('catalog.json').then(async raw => {
                    // the coverage file travels on the catalog as glyph_data: in memory only, never in catalog.json
                    if (raw && raw.glyphs && typeof raw.glyphs.file === 'string') raw.glyph_data = await api.catalogJSON(raw.glyphs.file);
                    if (raw && raw.icons && typeof raw.icons.file === 'string') raw.icon_data = await api.catalogJSON(raw.icons.file);
                    return raw;
                }).then(raw => {
                    if (!raw || !['storymaker-catalog/1', 'storymaker-catalog/2'].includes(raw.schema) || !Array.isArray(raw.entries)) throw S.errors.normalize({ message: 'The catalog has an unknown format.' });
```

In `ui/js/storymaker/host-app.js`, replace:

```js
    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
```

with:

```js
    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000, maxIcons: 1000, maxIconPaths: 40, maxIconPathChars: 2000,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
```

In `ui/js/storymaker/host-app.js`, replace:

```js
        return null;
    }

    // facesOf(catalog): the fallback faces of a catalog's glyph data. Only these may name a font over maxFontBytes.
    function facesOf(catalog) {
        var data = isObject(catalog) ? catalog.glyph_data : null;
```

with:

```js
        return null;
    }

    // checkIconData(data) = null when catalog.icon_data has the schema string and fits its caps, else a short reason.
    function checkIconData(data) {
        if (!isObject(data) || data.schema !== 'storymaker-icons/1') return 'load.catalog.icon_data needs the schema string';
        if (!Array.isArray(data.icons) || data.icons.length > CAPS.maxIcons) return 'load.catalog.icon_data has bad icons or too many icons';
        for (var i = 0; i < data.icons.length; i++) {
            var icon = data.icons[i];
            if (!isObject(icon) || typeof icon.id !== 'string' || !/^icon.[a-z0-9-]{1,60}$/.test(icon.id)) return 'load.catalog.icon_data has a bad icon id';
            if (!Array.isArray(icon.paths) || icon.paths.length > CAPS.maxIconPaths) return 'load.catalog.icon_data has bad or too many paths';
            for (var k = 0; k < icon.paths.length; k++) {
                if (typeof icon.paths[k] !== 'string' || icon.paths[k].length > CAPS.maxIconPathChars) return 'load.catalog.icon_data has a bad path';
            }
            if (icon.fills !== undefined && (!Array.isArray(icon.fills) || icon.fills.length > CAPS.maxIconPaths)) return 'load.catalog.icon_data has bad fills';
        }
        return null;
    }

    // facesOf(catalog): the fallback faces of a catalog's glyph data. Only these may name a font over maxFontBytes.
    function facesOf(catalog) {
        var data = isObject(catalog) ? catalog.glyph_data : null;
```

In `ui/js/storymaker/host-app.js`, replace:

```js
                    var badGlyphs = checkGlyphData(msg.catalog.glyph_data);
                    if (badGlyphs) return badGlyphs;
                }
                if (msg.audio_info !== undefined) {
                    var badAudio = checkAudioInfo(msg.audio_info);
                    if (badAudio) return badAudio;
```

with:

```js
                    var badGlyphs = checkGlyphData(msg.catalog.glyph_data);
                    if (badGlyphs) return badGlyphs;
                }
                if (msg.catalog.icon_data !== undefined) {
                    var badIcons = checkIconData(msg.catalog.icon_data);
                    if (badIcons) return badIcons;
                }
                if (msg.audio_info !== undefined) {
                    var badAudio = checkAudioInfo(msg.audio_info);
                    if (badAudio) return badAudio;
```

In `ui/js/storymaker/host-render.js`, replace:

```js
                if (glyphFile.indexOf('..') >= 0 || glyphFile.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + glyphFile);
                catalog.glyph_data = await getJSON('glyphs', assetsBase + glyphFile);
            }
            // Fonts are lazy: load.fonts stays empty and requestAssets fetches a face when compile needs it. URLs come from boot.fonts
            // only (the catalog's file paths are AuraGo paths), and each buffer is fetched once.
            var fontByKey = {}, fontFetches = {};
```

with:

```js
                if (glyphFile.indexOf('..') >= 0 || glyphFile.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + glyphFile);
                catalog.glyph_data = await getJSON('glyphs', assetsBase + glyphFile);
            }
            if (catalog && catalog.icons && catalog.icons.file) {
                var iconFile = String(catalog.icons.file);
                if (iconFile.indexOf('..') >= 0 || iconFile.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + iconFile);
                catalog.icon_data = await getJSON('icons', assetsBase + iconFile);
            }
            // Fonts are lazy: load.fonts stays empty and requestAssets fetches a face when compile needs it. URLs come from boot.fonts
            // only (the catalog's file paths are AuraGo paths), and each buffer is fetched once.
            var fontByKey = {}, fontFetches = {};
```

In `ui/js/storymaker/scene.js`, replace:

```js
    function indexCatalog(catalog) {
        var idx = {
            version: (catalog && catalog.version) || '', byId: {}, poses: {}, gestures: {}, expressions: {}, cycles: {},
            textStyles: {}, transitions: {}, mouths: {}, palettes: {}, fonts: (catalog && catalog.fonts) || [],
            stage: (catalog && catalog.stage) || null, rig: (catalog && catalog.rig) || null
        };
        var entries = (catalog && catalog.entries) || [];
```

with:

```js
    function indexCatalog(catalog) {
        var idx = {
            version: (catalog && catalog.version) || '', byId: {}, poses: {}, gestures: {}, expressions: {}, cycles: {},
            textStyles: {}, transitions: {}, mouths: {}, palettes: {}, icons: {}, fonts: (catalog && catalog.fonts) || [],
            stage: (catalog && catalog.stage) || null, rig: (catalog && catalog.rig) || null
        };
        var entries = (catalog && catalog.entries) || [];
```

In `ui/js/storymaker/scene.js`, replace:

```js
            else if (e.kind === 'transition') idx.transitions[bare] = e;
            else if (e.kind === 'mouth') idx.mouths[bare] = e;
        }
        var pals = (catalog && catalog.palettes) || [];
        for (var p = 0; p < pals.length; p++) idx.palettes[pals[p].name || String(pals[p].id).replace(/^palette\./, '')] = pals[p];
        return idx;
```

with:

```js
            else if (e.kind === 'transition') idx.transitions[bare] = e;
            else if (e.kind === 'mouth') idx.mouths[bare] = e;
        }
        // icons: id -> {paths, fills, tags} from catalog.icon_data (in memory only, like glyph_data)
        var icons = catalog && catalog.icon_data && Array.isArray(catalog.icon_data.icons) ? catalog.icon_data.icons : [];
        for (var n = 0; n < icons.length; n++) {
            if (icons[n] && icons[n].id) idx.icons[icons[n].id] = { paths: icons[n].paths || [], fills: icons[n].fills || [], tags: icons[n].tags || [] };
        }
        var pals = (catalog && catalog.palettes) || [];
        for (var p = 0; p < pals.length; p++) idx.palettes[pals[p].name || String(pals[p].id).replace(/^palette\./, '')] = pals[p];
        return idx;
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 272 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-test-support.mjs scripts/storymaker-tests/08-host-app.mjs scripts/storymaker-tests/33-blocks.mjs ui/js/desktop/apps/storymaker-assets.js ui/js/storymaker/host-app.js ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js ui/js/storymaker/scene.js
git commit -m "storymaker: the player indexes and loads icon data"
```

### Task C2: blocks.js model, areas and serialize/revive

**Files:**
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Create: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/scene.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;

check('blocks: indexCatalog indexes the icons', () => {
    const idx = t.core().scene.indexCatalog(t.catalog);
    assert.ok(idx.icons['icon.cpu'].paths.length > 0);
    assert.ok(Array.isArray(idx.icons['icon.cpu'].tags));
    assert.deepEqual(Object.keys(t.core().scene.indexCatalog({ entries: [] }).icons), []);
});
```

with:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();
const cat = SM.scene.indexCatalog(t.catalog);

check('blocks: indexCatalog indexes the icons', () => {
    const idx = SM.scene.indexCatalog(t.catalog);
    assert.ok(idx.icons['icon.cpu'].paths.length > 0);
    assert.ok(Array.isArray(idx.icons['icon.cpu'].tags));
    assert.deepEqual(Object.keys(SM.scene.indexCatalog({ entries: [] }).icons), []);
});

// ---- C2: model, areas, check, create, serialize and revive

const B = SM.blocks;
const stageOf = (aspect, env = null) => SM.stage.create({ aspect, env });
const BOARD_ENV = { id: 'env.board', kind: 'environment', drawn: 'board' };
const inside = (a, S, eps = 1e-6) => a.x >= S.x - eps && a.y >= S.y - eps && a.x + a.w <= S.x + S.w + eps && a.y + a.h <= S.y + S.h + eps;
const nodes = n => Array.from({ length: n }, (_, i) => ({ id: 'n' + i, label: 'Node ' + i }));
const diagramDef = (n, extra = {}) => ({ id: 'diagram-1', kind: 'diagram', diagram: { layout: 'flow', nodes: nodes(n) }, ...extra });

check('blocks: area gives the safe box, the board rect or the fallback, and null for an unknown name', () => {
    const S = SM.stage.safeBox('16:9'), st = stageOf('16:9'), office = t.byId('env.office');
    assert.deepEqual(plain(B.area(st, null, 'day', 'full')), { ...S, surface: 'panel' });
    const rect = B.BOARD_RECTS['env.office'], board = B.area(st, office, 'day', 'board');
    near(board.x, rect.x + 0.03 * rect.w, 1e-6);
    near(board.w, 0.94 * rect.w, 1e-6);
    near(board.y, rect.y + 0.03 * rect.h, 1e-6);
    near(board.h, 0.94 * rect.h, 1e-6);
    assert.ok(inside(board, S), 'the board inset lies inside the safe box');
    assert.equal(board.surface, 'light');
    assert.equal(B.area(st, { id: 'env.classroom' }, 'night', 'board').surface, 'dark');
    const plainBoard = B.area(st, t.byId('env.plain-stage'), 'day', 'board');
    assert.deepEqual(plain(plainBoard), { x: 960, y: S.y, w: S.x + S.w - 960, h: S.h, surface: 'panel' }, 'the fallback is the right half of the safe box');
    assert.deepEqual(plain(B.area(st, BOARD_ENV, 'day', 'board')), { ...S, surface: 'light' });
    assert.equal(B.area(st, BOARD_ENV, 'night', 'full').surface, 'dark');
    assert.equal(B.area(st, null, 'day', 'nowhere'), null);
    const corner = B.area(st, null, 'day', { x: 0, y: 0 });
    assert.ok(corner && inside(corner, S), 'a point area is shifted into the safe box');
    assert.equal(B.isBoard({ env: BOARD_ENV }), true);
    assert.equal(B.isBoard({ env: office }), false);
    assert.equal(B.isBoard(null), false);
});

check('blocks: in 9:16 the areas lie inside the safe box, left before right, and the board falls back to top', () => {
    const S = SM.stage.safeBox('9:16'), st = stageOf('9:16');
    for (const name of B.AREAS) assert.ok(inside(B.area(st, null, 'day', name), S), name);
    const left = B.area(st, null, 'day', 'left'), right = B.area(st, null, 'day', 'right');
    assert.ok(left.x + left.w < right.x);
    assert.deepEqual(plain(B.area(st, null, 'day', 'board')), plain(B.area(st, null, 'day', 'top')));
    const sq = stageOf('1:1');
    assert.deepEqual(plain(B.area(sq, null, 'day', 'board')), plain(B.area(sq, null, 'day', 'top')));
});

check('blocks: check refuses a 6-node flow, a negative pie value and an unknown icon, and names the fix', () => {
    const flow = B.check(diagramDef(6), cat);
    assert.equal(flow.field, 'diagram.nodes');
    assert.match(flow.message, /2 to 5/);
    const pie = B.check({ id: 'c1', kind: 'chart', chart: { type: 'pie', labels: ['a', 'b'], values: [3, -1] } }, cat);
    assert.equal(pie.field, 'chart.values');
    assert.match(pie.message, /0 or more/);
    const icon = B.check({ id: 'i1', kind: 'icon', icon: 'icon.no-such-thing' }, cat);
    assert.equal(icon.field, 'icon');
    assert.match(icon.message, /icon\.no-such-thing/);
    assert.match(icon.message, /search/i);
    assert.equal(B.check(diagramDef(5), cat), null);
    assert.equal(B.check({ id: 'i1', kind: 'icon', icon: 'icon.cpu', label: 'CPU' }, cat), null);
    assert.equal(B.check({ id: 'd', kind: 'diagram', diagram: { layout: 'flow', nodes: [{ id: 'a', label: 'A' }, { id: 'a', label: 'B' }] } }, cat).field, 'diagram.nodes');
    assert.equal(B.check({ id: 'c', kind: 'chart', chart: { type: 'counter', labels: ['x'], values: [1, 2] } }, cat).field, 'chart.values');
    assert.equal(B.check({ id: 'l', kind: 'list', text: 'a\nb\nc\nd\ne' }, cat).field, 'text');
    assert.equal(B.check({ id: 'h', kind: 'headline', text: 'x'.repeat(81) }, cat).field, 'text');
    assert.equal(B.check(diagramDef(2, { reveals: [1] }), cat).field, 'reveals');
    assert.equal(B.revealSeconds(diagramDef(3)), 0.5 + 0.6 * 3);
    assert.equal(B.revealSeconds({ kind: 'icon' }), 0.8);
    assert.equal(B.revealSeconds({ kind: 'chart' }), 1.2);
    assert.equal(B.revealSeconds({ kind: 'list', text: 'a\nb' }), 1);
    assert.equal(B.revealSeconds({ kind: 'headline', text: 'Ein *wichtiges* Wort' }), 0.4 + 0.15 * 3);
});

check('blocks: create, serialize and revive give an equal state at t = 0, 1, 2', () => {
    const def = diagramDef(3, { area: { x: 100, y: 200, w: 800, h: 400, surface: 'panel' }, t0: 1, dur: 2, entry: 'draw', reveals: [1, 1.5, 2], hidden: true });
    const e = B.create(def);
    assert.equal(e.type, 'block');
    assert.equal(e.x.init, 500);
    assert.equal(e.y.init, 400);
    assert.equal(e.alpha.init, 0);
    assert.equal(e.baseScale, 1);
    e.alpha.add(1, 1.5, 1, 'ease-out');
    e.x.add(1, 1.5, 500, 'ease-out', 400);
    e.marks.add(1.2, 1.8, { node: 'n1', style: 'pulse' });
    e.highlights.add(1.9, 2.4, { style: 'outline' });
    const cam = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(cam, 0.5, 1.5, 1.4, 'ease-in-out');
    SM.camera.addShake(cam, 1, 1.5, 10);
    const wire = JSON.parse(JSON.stringify(B.serialize([SM.scene.createText({ id: 't', text: 'x', x: 0, y: 0 }), e], cam)));
    assert.equal(wire.v, 1);
    assert.equal(wire.blocks.length, 1);
    assert.equal(B.serialize([], cam), null);
    const back = B.revive(wire, cat);
    assert.equal(back.entities.length, 1);
    for (const time of [0, 1, 1.25, 2]) {
        assert.deepEqual(plain(SM.scene.evalEntity(back.entities[0], time)), plain(SM.scene.evalEntity(e, time)), 'entity at ' + time);
        assert.deepEqual(plain(SM.camera.at(back.camera, time)), plain(SM.camera.at(cam, time)), 'camera at ' + time);
    }
    const state = SM.scene.evalEntity(e, 1.25);
    assert.equal(state.block.kind, 'diagram');
    assert.equal(state.time, 1.25);
    assert.equal(state.mark.node, 'n1');
    assert.equal(state.mark.style, 'pulse');
    assert.ok(SM.scene.entityEnd(e) >= 3, 'the end covers t0 + dur');
});

check('blocks: revive refuses 33 blocks, another version, an unknown ease, a long string and a missing number', () => {
    const def = diagramDef(2, { area: { x: 0, y: 0, w: 400, h: 300, surface: 'panel' }, t0: 0, dur: 1 });
    const wire = () => JSON.parse(JSON.stringify(B.serialize([B.create(def)], null)));
    const one = wire();
    assert.notEqual(B.revive(one, cat), null);
    assert.equal(B.revive({ ...one, blocks: new Array(33).fill(one.blocks[0]) }, cat), null);
    assert.equal(B.revive({ ...one, v: 2 }, cat), null);
    const bad = wire();
    bad.blocks[0].alpha.segs.push({ t0: 0, t1: 1, from: 0, to: 1, ease: 'wobble', log: false });
    assert.equal(B.revive(bad, cat), null);
    const long = wire();
    long.blocks[0].block.diagram.nodes[0].label = 'x'.repeat(201);
    assert.equal(B.revive(long, cat), null);
    const nan = wire();
    nan.blocks[0].t0 = null;
    assert.equal(B.revive(nan, cat), null);
    assert.equal(B.revive(null, cat), null);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: SM.blocks undefined (5 new blocks checks failed)

- [ ] **Step 3: Implement**

In `scripts/storymaker-bundle-parts.js`, replace:

```js
  'ui/js/storymaker/plan.js',
  'ui/js/storymaker/motion.js',
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

with:

```js
  'ui/js/storymaker/plan.js',
  'ui/js/storymaker/motion.js',
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

Create `ui/js/storymaker/blocks.js`:

```js
// Storymaker player: blocks. A block is a drawn explainer element (an icon, a diagram, a chart, a headline or a list) in one
// area of the frame. This file holds the model: the areas and the board rectangles, check and create, the reveal timing,
// serialize and revive for code scenes. blocks-draw.js paints them. A block is an entity of type 'block' whose tracks (x, y,
// alpha, scale, rot) are filled by the storyboard beats or by the code-scene API; its drawing is a pure function of the
// evaluated state and the scene time. Classic script; publishes StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var BOARD_ID = 'env.board';
    var AREAS = ['full', 'left', 'right', 'top', 'center', 'board'];
    var KINDS = ['icon', 'diagram', 'chart', 'headline', 'list'];
    var ICON_STYLES = ['line', 'badge', 'card'];
    var LAYOUTS = { flow: [2, 5], cycle: [3, 6], hub: [3, 7], stack: [2, 5], compare: [2, 2] };
    var CHARTS = { bar: [1, 8], line: [2, 24], pie: [2, 6], counter: [1, 1] };
    var ENTRIES = ['draw', 'pop', 'fade', 'slide', 'none'];
    var SURFACES = ['panel', 'light', 'dark'];
    var LIST_MAX = 4, POINTS_MAX = 3, MAX_BLOCKS = 32, MAX_SEGS = 64, MAX_MARKS = 64, MAX_STRING = 200;
    var LABEL_MAX = 40, HEADLINE_MAX = 80, CHART_LABEL_MAX = 24, CHART_UNIT_MAX = 8, CHART_TITLE_MAX = 40;
    var ITEM_SPAN = 0.5, WORD_SPAN = 0.35, FLOW_PERIOD = 1.6;
    var SLUG_RE = /^[a-z0-9][a-z0-9-]{0,47}$/;
    // The eases a track may carry: the six public ones and the internal curves that SM.ease knows (see ease.js).
    var EASES = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out', 'quart-out', 'expo-out', 'spring'];

    // The boards of the raster environments, in design pixels of the 1920x1080 frame. Measured on the delivered masters
    // (ui/img/storymaker/textures/environments, 1672x941, scaled to the design frame) by decoding the pixels: the first row
    // and column on each side that still has the colour of the board surface, taken as the median of three scan lines.
    //   office-day.png     pinboard   left 809, top 121, right 1451, bottom 432 (seed 810, 120, 1452, 432)
    //   classroom-day.png  chalkboard left 515, top 104, right 1535, bottom 442 (seed 511, 101, 1536, 445)
    // office-night.png is the same board in the dark: mid brown (luminance 0.18), so dark ink reads at 3.1:1 and light ink at
    // 4.1:1. The night tone of the office is therefore dark. Every other environment has no board in its master.
    var BOARD_RECTS = {
        'env.office': { x: 809, y: 121, w: 642, h: 311, tone: { day: 'light', night: 'dark' } },
        'env.classroom': { x: 515, y: 104, w: 1020, h: 338, tone: { day: 'dark', night: 'dark' } }
    };

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function finite(v) { return typeof v === 'number' && isFinite(v); }
    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function own(map, key) { return Object.prototype.hasOwnProperty.call(map, key) ? map[key] : undefined; }
    function isBoard(sc) { return !!(sc && sc.env && sc.env.drawn === 'board'); }

    // ---- areas

    // area(stage, env, variant, at) = {x, y, w, h, surface} in design pixels, or null for an unknown name. at is a name of
    // AREAS or {x, y} in percent of the safe area.
    function area(stage, env, variant, at) {
        var S = SM.stage.safeBox(stage.aspect), v = variant === 'night' ? 'night' : 'day';
        var onBoard = !!(env && env.drawn === 'board'), panel = onBoard ? (v === 'night' ? 'dark' : 'light') : 'panel';
        function box(x, y, w, h, surface) { return { x: x, y: y, w: w, h: h, surface: surface || panel }; }
        if (at && typeof at === 'object') {
            if (!finite(at.x) || !finite(at.y)) return null;
            var w = 0.45 * S.w, h = 0.5 * S.h, p = stage.safePoint(at.x, at.y);
            return box(clamp(p.x - w / 2, S.x, S.x + S.w - w), clamp(p.y - h / 2, S.y, S.y + S.h - h), w, h);
        }
        switch (at) {
            case 'full': return box(S.x, S.y, S.w, S.h);
            case 'left': return box(S.x, S.y, 0.48 * S.w, S.h);
            case 'right': return box(S.x + 0.52 * S.w, S.y, 0.48 * S.w, S.h);
            case 'top': return box(S.x, S.y, S.w, 0.45 * S.h);
            case 'center': return box(S.x + 0.2 * S.w, S.y + 0.1 * S.h, 0.6 * S.w, 0.8 * S.h);
            case 'board': break;
            default: return null;
        }
        if (onBoard) return box(S.x, S.y, S.w, S.h);
        var rect = env ? own(BOARD_RECTS, env.id) : undefined, vis = stage.vis || { x: 0, y: 0, w: SM.stage.DESIGN_W, h: SM.stage.DESIGN_H };
        if (rect) {
            // the board inset by 3 % per side, cut to the safe box and to the slice that the format shows
            var x0 = Math.max(rect.x + 0.03 * rect.w, S.x, vis.x), y0 = Math.max(rect.y + 0.03 * rect.h, S.y, vis.y);
            var x1 = Math.min(rect.x + 0.97 * rect.w, S.x + S.w, vis.x + vis.w), y1 = Math.min(rect.y + 0.97 * rect.h, S.y + S.h, vis.y + vis.h);
            if (x1 > x0 && y1 > y0 && (x1 - x0) * (y1 - y0) >= 0.5 * rect.w * rect.h) return box(x0, y0, x1 - x0, y1 - y0, rect.tone[v]);
        }
        if (stage.aspect !== '16:9') return area(stage, env, v, 'top');
        var left = Math.max(S.x, vis.x + vis.w / 2);
        return box(left, S.y, S.x + S.w - left, S.h);
    }

    // ---- check

    function problem(field, message) { return { field: field, message: message }; }

    function lines(text) {
        return typeof text === 'string' ? text.split('\n').map(function (l) { return l.replace(/\s+$/, ''); }).filter(function (l) { return l.length > 0; }) : [];
    }

    function wordCount(text) { return String(text || '').replace(/\*/g, ' ').split(/\s+/).filter(Boolean).length; }

    function itemCount(b) {
        if (b.kind === 'diagram') return b.diagram && Array.isArray(b.diagram.nodes) ? b.diagram.nodes.length : 0;
        if (b.kind === 'list') return lines(b.text).length;
        if (b.kind === 'headline') return wordCount(b.text);
        return 1;
    }

    function checkIcon(id, field, cat) {
        if (typeof id !== 'string' || !cat || !cat.icons || !own(cat.icons, id)) {
            return problem(field, 'Unknown icon "' + String(id).slice(0, 60) + '". Search storymaker_catalog with kind icon and use an exact id such as icon.cpu.');
        }
        return null;
    }

    function checkLabel(label, field, max) {
        if (typeof label !== 'string' || label.length < 1 || label.length > max) return problem(field, 'A label has 1 to ' + max + ' characters. Shorten it.');
        return null;
    }

    function checkDiagram(d, cat) {
        if (!isObject(d)) return problem('diagram', 'A diagram block needs diagram {layout, nodes}.');
        var range = own(LAYOUTS, d.layout);
        if (!range) return problem('diagram.layout', 'Unknown diagram layout "' + String(d.layout).slice(0, 20) + '". Use ' + Object.keys(LAYOUTS).join(', ') + '.');
        var list = d.nodes;
        if (!Array.isArray(list) || list.length < range[0] || list.length > range[1]) {
            return problem('diagram.nodes', 'A ' + d.layout + ' diagram takes ' + range[0] + ' to ' + range[1] + ' nodes, got ' + (Array.isArray(list) ? list.length : 0) + '. Change the number of nodes or the layout.');
        }
        var seen = {};
        for (var i = 0; i < list.length; i++) {
            var n = list[i], at = 'diagram.nodes[' + i + ']';
            if (!isObject(n)) return problem('diagram.nodes', 'Node ' + i + ' must be an object {id, label}.');
            if (typeof n.id !== 'string' || !SLUG_RE.test(n.id)) return problem('diagram.nodes', 'Node ' + i + ' needs an id made of lowercase letters, digits and dashes, for example "srv".');
            if (seen[n.id]) return problem('diagram.nodes', 'The node id "' + n.id + '" is used twice. Give every node its own id.');
            seen[n.id] = true;
            var bad = checkLabel(n.label, at + '.label', LABEL_MAX);
            if (bad) return bad;
            if (n.icon !== undefined && n.icon !== null && n.icon !== '') { bad = checkIcon(n.icon, at + '.icon', cat); if (bad) return bad; }
            if (n.points !== undefined) {
                if (!Array.isArray(n.points) || n.points.length > POINTS_MAX) return problem(at + '.points', 'A node has at most ' + POINTS_MAX + ' points.');
                for (var k = 0; k < n.points.length; k++) {
                    if (typeof n.points[k] !== 'string' || n.points[k].length < 1 || n.points[k].length > LABEL_MAX) return problem(at + '.points', 'A point has 1 to ' + LABEL_MAX + ' characters. Shorten it.');
                }
            }
        }
        return null;
    }

    function checkChart(c) {
        if (!isObject(c)) return problem('chart', 'A chart block needs chart {type, labels, values}.');
        var range = own(CHARTS, c.type);
        if (!range) return problem('chart.type', 'Unknown chart type "' + String(c.type).slice(0, 20) + '". Use ' + Object.keys(CHARTS).join(', ') + '.');
        if (!Array.isArray(c.labels) || !Array.isArray(c.values) || c.labels.length !== c.values.length) return problem('chart.values', 'labels and values need the same length.');
        if (c.values.length < range[0] || c.values.length > range[1]) return problem('chart.values', 'A ' + c.type + ' chart takes ' + range[0] + ' to ' + range[1] + ' values, got ' + c.values.length + '.');
        var sum = 0;
        for (var i = 0; i < c.values.length; i++) {
            if (!finite(c.values[i])) return problem('chart.values', 'Every value must be a finite number.');
            if (c.type === 'pie' && c.values[i] < 0) return problem('chart.values', 'A pie value must be 0 or more, got ' + c.values[i] + '.');
            sum += c.values[i];
            if (typeof c.labels[i] !== 'string' || c.labels[i].length > CHART_LABEL_MAX) return problem('chart.labels', 'A chart label has at most ' + CHART_LABEL_MAX + ' characters.');
        }
        if (c.type === 'pie' && !(sum > 0)) return problem('chart.values', 'A pie needs a sum above 0.');
        if (c.unit !== undefined && (typeof c.unit !== 'string' || c.unit.length > CHART_UNIT_MAX)) return problem('chart.unit', 'The unit has at most ' + CHART_UNIT_MAX + ' characters.');
        if (c.title !== undefined && (typeof c.title !== 'string' || c.title.length > CHART_TITLE_MAX)) return problem('chart.title', 'The title has at most ' + CHART_TITLE_MAX + ' characters.');
        return null;
    }

    // check(def, cat) = null, or {field, message} for the first problem; the message names the fix. cat is SM.scene.indexCatalog.
    function check(def, cat) {
        if (!isObject(def)) return problem('block', 'A block is an object.');
        if (typeof def.id !== 'string' || !def.id || def.id.length > 64) return problem('id', 'A block needs an id of 1 to 64 characters.');
        if (KINDS.indexOf(def.kind) < 0) return problem('kind', 'Unknown block kind "' + String(def.kind).slice(0, 20) + '". Use ' + KINDS.join(', ') + '.');
        var bad = null, n = itemCount(def);
        if (def.kind === 'icon') {
            bad = checkIcon(def.icon, 'icon', cat);
            if (!bad && def.style !== undefined && ICON_STYLES.indexOf(def.style) < 0) bad = problem('style', 'Unknown icon style "' + String(def.style).slice(0, 20) + '". Use ' + ICON_STYLES.join(', ') + '.');
            if (!bad && def.label !== undefined && def.label !== '') bad = checkLabel(def.label, 'label', LABEL_MAX);
        } else if (def.kind === 'diagram') bad = checkDiagram(def.diagram, cat);
        else if (def.kind === 'chart') bad = checkChart(def.chart);
        else if (def.kind === 'headline') {
            if (typeof def.text !== 'string' || !def.text.trim() || def.text.length > HEADLINE_MAX) bad = problem('text', 'A headline has 1 to ' + HEADLINE_MAX + ' characters. Shorten it.');
        } else {
            var rows = lines(def.text);
            if (!rows.length || rows.length > LIST_MAX) bad = problem('text', 'A list has 1 to ' + LIST_MAX + ' lines, got ' + rows.length + '. Split the list.');
            for (var r = 0; !bad && r < rows.length; r++) if (rows[r].length > LABEL_MAX) bad = problem('text', 'A list line has at most ' + LABEL_MAX + ' characters. Shorten line ' + (r + 1) + '.');
        }
        if (bad) return bad;
        if (def.reveals !== undefined && def.reveals !== null) {
            if (!Array.isArray(def.reveals) || def.reveals.length !== n || !def.reveals.every(finite)) return problem('reveals', 'reveals needs one finite number per node or line (' + n + ').');
        }
        if (def.area !== undefined && !(isObject(def.area) && finite(def.area.x) && finite(def.area.y) && def.area.w > 0 && def.area.h > 0 && isFinite(def.area.w) && isFinite(def.area.h))) {
            return problem('area', 'area needs finite x, y, w and h from SM.blocks.area.');
        }
        return null;
    }

    // revealSeconds(def): the default entry span of a block in seconds, before the rhythm factor.
    function revealSeconds(def) {
        switch (def.kind) {
            case 'diagram': return 0.5 + 0.6 * itemCount(def);
            case 'chart': return 1.2;
            case 'headline': return 0.4 + 0.15 * wordCount(def.text);
            case 'list': return 0.5 * itemCount(def);
            default: return 0.8;
        }
    }

    // itemProgress(st, k): 0..1 reveal of item k of an evaluated block state (a node, a line or a word; k = 0 for icon and chart).
    function itemProgress(st, k) {
        var kind = st.block.kind, p;
        if (kind === 'icon' || kind === 'chart') p = st.dur > 0 ? (st.time - st.t0) / st.dur : (st.time >= st.t0 ? 1 : 0);
        else {
            var n = Math.max(1, itemCount(st.block)), full = kind === 'headline' ? WORD_SPAN : ITEM_SPAN, start, span;
            if (st.reveals && finite(st.reveals[k])) { start = st.reveals[k]; span = full; }
            else { start = st.t0 + k * st.dur / n; span = Math.min(full, st.dur / n); }
            p = span > 0 ? (st.time - start) / span : (st.time >= start ? 1 : 0);
        }
        return SM.ease('ease-out', clamp(p, 0, 1));
    }

    // ---- create

    var DEF_ONLY = { area: 1, t0: 1, dur: 1, entry: 1, reveals: 1, hidden: 1 };

    // make(block, geometry): the entity. The block field is a plain copy of the definition without the timing fields.
    function make(block, g) {
        var e = SM.scene.base('block', block.id, g.x, g.y);
        e.block = block;
        e.w = g.w;
        e.h = g.h;
        e.surface = SURFACES.indexOf(g.surface) >= 0 ? g.surface : 'panel';
        e.t0 = g.t0;
        e.dur = g.dur;
        e.entry = ENTRIES.indexOf(g.entry) >= 0 ? g.entry : 'fade';
        e.reveals = g.reveals ? g.reveals.slice() : null;
        e.marks = new SM.timeline.Intervals();
        e.highlights = new SM.timeline.Intervals();
        e.baseScale = 1;
        if (g.hidden) e.alpha.init = 0;
        return e;
    }

    // create(def) = the entity of type 'block' for a definition (see the seams document); def.area comes from area().
    function create(def) {
        if (!isObject(def) || !isObject(def.area)) throw new TypeError('SM.blocks.create needs a definition with an area from SM.blocks.area.');
        var block = {};
        Object.keys(def).forEach(function (k) { if (!DEF_ONLY[k]) block[k] = JSON.parse(JSON.stringify(def[k])); });
        var t0 = finite(def.t0) ? def.t0 : 0, dur = finite(def.dur) ? def.dur : revealSeconds(def), a = def.area;
        return make(block, { x: a.x + a.w / 2, y: a.y + a.h / 2, w: a.w, h: a.h, surface: a.surface, t0: t0, dur: dur < 0 ? 0 : dur, entry: def.entry, reveals: def.reveals, hidden: def.hidden === true });
    }

    // ---- serialize and revive

    function trackOut(track) {
        track.finalize();
        return { init: track.init, segs: track.segs.map(function (s) { return { t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: !!s.log }; }) };
    }

    function listOut(list, keys) {
        return list.list.map(function (it) {
            var o = { t0: it.t0, t1: it.t1 };
            keys.forEach(function (k) { o[k] = it.data[k]; });
            return o;
        });
    }

    // serialize(entities, camera) = a plain payload of the block entities and the camera, or null without blocks.
    function serialize(entities, camera) {
        var list = (entities || []).filter(function (e) { return e && e.type === 'block'; });
        if (!list.length) return null;
        var cam = null;
        if (camera) cam = { zoom: trackOut(camera.zoom), cx: trackOut(camera.cx), cy: trackOut(camera.cy), shakes: (camera.shakes || []).map(function (s) { return { t0: s.t0, t1: s.t1, amp: s.amp }; }) };
        return {
            v: 1, camera: cam,
            blocks: list.map(function (e) {
                return {
                    id: e.id, block: e.block, w: e.w, h: e.h, surface: e.surface, t0: e.t0, dur: e.dur, entry: e.entry, reveals: e.reveals,
                    x: trackOut(e.x), y: trackOut(e.y), alpha: trackOut(e.alpha), scale: trackOut(e.scale), rot: trackOut(e.rot),
                    marks: listOut(e.marks, ['node', 'style']), highlights: listOut(e.highlights, ['style'])
                };
            })
        };
    }

    // sane(value, depth): false for a non-finite number, a string over MAX_STRING characters or a structure that is too deep.
    function sane(value, depth) {
        if (typeof value === 'number') return isFinite(value);
        if (typeof value === 'string') return value.length <= MAX_STRING;
        if (value === null || typeof value === 'boolean' || value === undefined) return true;
        if (depth > 8 || typeof value !== 'object') return false;
        return Object.keys(value).every(function (k) { return k.length <= MAX_STRING && sane(value[k], depth + 1); });
    }

    function trackIn(plain, T) {
        if (!isObject(plain) || !finite(plain.init) || !Array.isArray(plain.segs) || plain.segs.length > MAX_SEGS) return null;
        var track = new T.Track(plain.init);
        for (var i = 0; i < plain.segs.length; i++) {
            var s = plain.segs[i];
            if (!isObject(s) || !finite(s.t0) || !finite(s.t1) || !finite(s.from) || !finite(s.to) || EASES.indexOf(s.ease) < 0) return null;
            track.segs.push({ t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: s.log === true, order: i });
        }
        return track;
    }

    function listIn(plain, keys, into) {
        if (!Array.isArray(plain) || plain.length > MAX_MARKS) return false;
        for (var i = 0; i < plain.length; i++) {
            var m = plain[i];
            if (!isObject(m) || !finite(m.t0) || !finite(m.t1)) return false;
            var data = {};
            keys.forEach(function (k) { data[k] = m[k]; });
            into.add(m.t0, m.t1, data);
        }
        return true;
    }

    function reviveBlock(p, cat) {
        var T = SM.timeline;
        if (!isObject(p) || !isObject(p.block) || p.block.id !== p.id || check(p.block, cat)) return null;
        if (!finite(p.w) || !finite(p.h) || !(p.w > 0) || !(p.h > 0) || !finite(p.t0) || !finite(p.dur) || p.dur < 0) return null;
        if (SURFACES.indexOf(p.surface) < 0 || ENTRIES.indexOf(p.entry) < 0) return null;
        if (p.reveals !== null && p.reveals !== undefined && (!Array.isArray(p.reveals) || p.reveals.length !== itemCount(p.block) || !p.reveals.every(finite))) return null;
        var tracks = {};
        for (var k = 0; k < 5; k++) {
            var name = ['x', 'y', 'alpha', 'scale', 'rot'][k];
            tracks[name] = trackIn(p[name], T);
            if (!tracks[name]) return null;
        }
        var e = make(p.block, { x: tracks.x.init, y: tracks.y.init, w: p.w, h: p.h, surface: p.surface, t0: p.t0, dur: p.dur, entry: p.entry, reveals: p.reveals, hidden: false });
        Object.keys(tracks).forEach(function (name) { e[name] = tracks[name]; });
        return listIn(p.marks, ['node', 'style'], e.marks) && listIn(p.highlights, ['style'], e.highlights) ? e : null;
    }

    // revive(plain, cat) = {entities, camera} from a payload of serialize, or null when anything in it is invalid.
    function revive(plain, cat) {
        if (!isObject(plain) || plain.v !== 1 || !Array.isArray(plain.blocks) || plain.blocks.length > MAX_BLOCKS || !sane(plain, 0)) return null;
        var T = SM.timeline, camera = null, entities = [];
        if (plain.camera !== null && plain.camera !== undefined) {
            var c = plain.camera;
            if (!isObject(c) || !Array.isArray(c.shakes) || c.shakes.length > MAX_MARKS) return null;
            camera = { zoom: trackIn(c.zoom, T), cx: trackIn(c.cx, T), cy: trackIn(c.cy, T), shakes: [] };
            if (!camera.zoom || !camera.cx || !camera.cy) return null;
            for (var s = 0; s < c.shakes.length; s++) {
                var sh = c.shakes[s];
                if (!isObject(sh) || !finite(sh.t0) || !finite(sh.t1) || !finite(sh.amp)) return null;
                camera.shakes.push({ t0: sh.t0, t1: sh.t1, amp: sh.amp });
            }
        }
        for (var i = 0; i < plain.blocks.length; i++) {
            var e = reviveBlock(plain.blocks[i], cat);
            if (!e) return null;
            entities.push(e);
        }
        return { entities: entities, camera: camera };
    }

    SM.blocks = {
        BOARD_ID: BOARD_ID, AREAS: AREAS, KINDS: KINDS, ICON_STYLES: ICON_STYLES, LAYOUTS: LAYOUTS, CHARTS: CHARTS, LIST_MAX: LIST_MAX,
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/scene.js`, replace:

```js
            s.style = e.style;
            var cap = e.captions ? SM.captions.at(e.captions, t * 1000) : null;
            s.lines = e.captions ? (cap ? cap.lines : null) : e.lines;
        } else if (e.type === 'shape') {
            s.kind = e.kind;
            s.w = e.w.at(t);
```

with:

```js
            s.style = e.style;
            var cap = e.captions ? SM.captions.at(e.captions, t * 1000) : null;
            s.lines = e.captions ? (cap ? cap.lines : null) : e.lines;
        } else if (e.type === 'block') {
            // a drawn explainer block (SM.blocks): the definition, its box, its timing and the node mark running now
            s.block = e.block;
            s.w = e.w;
            s.h = e.h;
            s.surface = e.surface;
            s.t0 = e.t0;
            s.dur = e.dur;
            s.entry = e.entry;
            s.reveals = e.reveals;
            s.time = t;
            var mk = e.marks ? e.marks.active(t) : null;
            s.mark = mk ? { node: mk.data.node, style: mk.data.style, u: mk.t1 > mk.t0 ? (t - mk.t0) / (mk.t1 - mk.t0) : 1 } : null;
        } else if (e.type === 'shape') {
            s.kind = e.kind;
            s.w = e.w.at(t);
```

In `ui/js/storymaker/scene.js`, replace:

```js
            var lists = [e.gestures.list, e.walks.list];
            for (var k = 0; k < lists.length; k++) for (var i = 0; i < lists[k].length; i++) if (lists[k][i].t1 > end) end = lists[k][i].t1;
        }
        var marks = e.highlights ? e.highlights.list : [];
        for (var m = 0; m < marks.length; m++) if (marks[m].t1 > end) end = marks[m].t1;
        return end;
    }

    SM.scene = {
        indexCatalog: indexCatalog, sfxEntry: sfxEntry, createActor: createActor, createProp: createProp, createText: createText,
        createShape: createShape, createBubble: createBubble, evalEntity: evalEntity, entityEnd: entityEnd
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
            var lists = [e.gestures.list, e.walks.list];
            for (var k = 0; k < lists.length; k++) for (var i = 0; i < lists[k].length; i++) if (lists[k][i].t1 > end) end = lists[k][i].t1;
        }
        var marks = (e.highlights ? e.highlights.list : []).concat(e.marks ? e.marks.list : []);
        for (var m = 0; m < marks.length; m++) if (marks[m].t1 > end) end = marks[m].t1;
        if (e.type === 'block') {
            if (e.t0 + e.dur > end) end = e.t0 + e.dur;
            for (var r = 0; e.reveals && r < e.reveals.length; r++) if (e.reveals[r] + SM.blocks.ITEM_SPAN > end) end = e.reveals[r] + SM.blocks.ITEM_SPAN;
        }
        return end;
    }

    SM.scene = {
        indexCatalog: indexCatalog, sfxEntry: sfxEntry, base: base, createActor: createActor, createProp: createProp, createText: createText,
        createShape: createShape, createBubble: createBubble, evalEntity: evalEntity, entityEnd: entityEnd
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 277 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-bundle-parts.js scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js ui/js/storymaker/scene.js
git commit -m "storymaker: blocks.js model, areas and serialize/revive"
```

### Task C3: icon painting

**Files:**
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Create: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

with:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain, FakeCanvas, digest, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    assert.equal(B.revive(nan, cat), null);
    assert.equal(B.revive(null, cat), null);
});
```

with:

```js
    assert.equal(B.revive(nan, cat), null);
    assert.equal(B.revive(null, cat), null);
});

// ---- C3: icon painting

const PALETTE = Object.fromEntries(Object.entries(t.catalog.palettes[0].tokens));
const ICONS = readJSON('ui/img/storymaker/icons/lucide.json');
const realCat = SM.scene.indexCatalog({ ...t.catalog, icon_data: ICONS });
const rcOf = (W, H, aspect = '16:9', c = cat) => SM.render.makeContext({ aspect, fps: 30, palette: PALETTE, cat: c, fontFamilies: { default: 'Geist' }, style: { font: 'default' } }, {}, W, H);
// paint(def, time, {W, H, aspect, cat, area}): the call log of one block at one time
function paint(def, time, o = {}) {
    const W = o.W || 1920, H = o.H || 1080, aspect = o.aspect || '16:9';
    const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect, o.cat || cat);
    const area = o.area || B.area(stage, null, 'day', 'full');
    const e = B.create({ t0: 0, dur: 1, ...def, area });
    if (o.setup) o.setup(e);
    const st = SM.scene.evalEntity(e, time), cv = new FakeCanvas(W, H);
    B.draw(cv.ctx, st, rc, stage, stage.baseWindow());
    return cv.ctx.log;
}
// the time at which the eased progress of a 1 s block that starts at 0 is p (ease-out, see itemProgress)
const timeFor = p => 1 - Math.cbrt(1 - p);

check('blocks: every icon draws, one stroke per path and one fill per fill, and the same time gives the same digest', () => {
    for (const icon of ICONS.icons) {
        const log = paint({ id: 'i', kind: 'icon', icon: icon.id, style: 'line' }, 5, { cat: realCat });
        assert.equal(log.filter(l => l.startsWith('stroke(path(')).length, icon.paths.length, icon.id);
        assert.equal(log.filter(l => l.startsWith('fill(path(')).length, (icon.fills || []).length, icon.id);
        assert.ok(log.includes('set lineCap=round') && log.includes('set lineJoin=round'));
    }
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5);
    assert.ok(log.includes('set lineWidth=2'), 'the line width is 2 on the 24 grid, scaled by size / 24');
    assert.ok(log.some(l => l.startsWith('scale(')), 'the grid is scaled to the icon size');
    assert.equal(digest(paint({ id: 'i', kind: 'icon', icon: 'icon.cpu' }, 0.4)), digest(paint({ id: 'i', kind: 'icon', icon: 'icon.cpu' }, 0.4)));
});

check('blocks: entry draw reveals the strokes with a dash and ends without one', () => {
    const def = { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' };
    assert.ok(paint(def, timeFor(0.5)).includes('setLineDash([48,96])'));
    assert.ok(paint(def, 5).includes('setLineDash([])'));
    assert.ok(!paint(def, 5).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'));
    assert.equal(paint(def, 0).filter(l => l.startsWith('stroke(')).length, 0, 'nothing is drawn before the reveal starts');
});

check('blocks: the icon styles line, badge and card draw differently, the card prints its label', () => {
    const logs = ['line', 'badge', 'card'].map(style => paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style, label: 'CPU' }, 5));
    assert.equal(new Set(logs.map(digest)).size, 3);
    assert.ok(logs[2].some(l => l.startsWith('fillText(CPU,0,')), 'the card label is centred');
    assert.ok(!logs[0].some(l => l.startsWith('fillText')), 'the line style prints no label');
    assert.ok(logs[1].some(l => l.startsWith('arc(')), 'the badge is a circle');
    assert.ok(logs[2].some(l => l.startsWith('arcTo(')), 'the card has rounded corners');
    const panel = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5);
    assert.ok(panel.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a panel surface draws the palette background behind the block');
    const light = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5, { area: { ...B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full'), surface: 'light' } });
    assert.ok(!light.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a board surface draws no panel');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.draw is not a function (3 new icon checks failed)

- [ ] **Step 3: Implement**

In `scripts/storymaker-bundle-parts.js`, replace:

```js
  'ui/js/storymaker/motion.js',
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

with:

```js
  'ui/js/storymaker/motion.js',
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/blocks-draw.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

Create `ui/js/storymaker/blocks-draw.js`:

```js
// Storymaker player: block painters. draw(ctx, st, rc, stage, win) paints one evaluated block state (see blocks.js) in the
// design frame: an optional panel, then the icon, diagram, chart, headline or list. Every painter works from a plan: the box
// it occupies and the smallest label it prints, both computed from the state alone, so measure() and draw() agree. All strokes
// are round and scaled from the 24 grid of the icons. Nothing here reads a clock or a random source: a frame is a pure
// function of the state and the scene time. Classic script; adds draw and plan to StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks;

    var DASH = 96;   // the dash that reveals a stroke: a dash of DASH * p and a gap of DASH, in 24-grid units for icons
    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = {}, pathCount = 0;

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function r3(v) { return Math.round(v * 1000) / 1000; }

    function pathOf(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 2000) { pathCache = {}; pathCount = 0; }
            p = pathCache[d] = new Path2D(d);
            pathCount++;
        }
        return p;
    }

    // ---- colours: ink, accent, soft and the colour of a line on an accent fill, by surface (see the seams document)
    function colours(rc, surface) {
        var p = rc.palette || {}, dark = surface === 'dark';
        return {
            ink: dark ? '#f4f5eb' : (p.text || '#1f2a44'), accent: dark ? (p.accent2 || '#36cfc9') : (p.accent || '#ff7a45'),
            accent2: dark ? (p.accent || '#ff7a45') : (p.accent2 || '#36cfc9'),
            soft: dark ? 'rgba(255,255,255,0.10)' : (p.bg2 || '#ffe0a3'), bg: p.bg || '#fff7e6', onAccent: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6'),
            panel: p.bg || '#fff7e6'
        };
    }

    // context(ctx, st, rc, stage): what every painter needs. base and floor are the label font and its smallest size in design pixels.
    function context(ctx, st, rc, stage) {
        return {
            ctx: ctx, st: st, rc: rc, stage: stage, col: colours(rc, st.surface), family: SM.render.familyOf(rc, { font: 'default' }),
            alpha: clamp(st.alpha, 0, 1), base: B.LABEL_BASE * rc.H / stage.scale, floor: B.LABEL_MIN * Math.min(rc.W, rc.H) / stage.scale
        };
    }

    // ---- text
    function setFont(c, weight, px) { c.ctx.font = SM.text.fontString(weight, px, c.family); }
    function widthOf(c, s, weight, px) { setFont(c, weight, px); return c.ctx.measureText(s).width; }

    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out;
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
            if (!out.truncated || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        return { px: px, lines: out.lines };
    }

    // printLines(c, lines, x, y, px, weight, color, align): the lines centred on y, 1.2 lines high.
    function printLines(c, lines, x, y, px, weight, color, align) {
        var ctx = c.ctx;
        setFont(c, weight, px);
        ctx.textAlign = align || 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = color;
        for (var i = 0; i < lines.length; i++) {
            ctx.direction = SM.ranges.dirOf(lines[i]);
            ctx.fillText(lines[i], x, y + (i - (lines.length - 1) / 2) * px * 1.2);
        }
    }

    // ---- icons

    // glyph(c, icon, cx, cy, size, color, p): the strokes of an icon on the 24 grid, line width 2 * size / 24, revealed to p.
    function glyph(c, icon, cx, cy, size, color, p) {
        if (!icon || !(p > 0.001)) return;
        var ctx = c.ctx, fills = icon.fills || [];
        ctx.save();
        ctx.translate(cx - size / 2, cy - size / 2);
        ctx.scale(size / 24, size / 24);
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        ctx.lineWidth = 2;
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.setLineDash(p < 1 ? [r3(DASH * p), DASH] : []);
        for (var i = 0; i < icon.paths.length; i++) {
            var path = pathOf(icon.paths[i]);
            if (fills.indexOf(icon.paths[i]) >= 0) { ctx.globalAlpha = c.alpha * p; ctx.fill(path); ctx.globalAlpha = c.alpha; }
            ctx.stroke(path);
        }
        ctx.restore();
    }

    function iconPlan(c) {
        var st = c.st, b = st.block, ctx = c.ctx, icon = c.rc.cat && c.rc.cat.icons ? c.rc.cat.icons[b.icon] : null;
        var m = Math.min(st.w, st.h), style = B.ICON_STYLES.indexOf(b.style) >= 0 ? b.style : (b.label ? 'card' : 'badge');
        var p = st.entry === 'draw' ? B.itemProgress(st, 0) : 1;
        if (style === 'line') {
            var s = 0.6 * m;
            return { box: { x: -s / 2, y: -s / 2, w: s, h: s }, labelPx: 0, paint: function () { glyph(c, icon, 0, 0, s, c.col.ink, p); } };
        }
        if (style === 'badge') {
            var d = 0.72 * m;
            return {
                box: { x: -d / 2, y: -d / 2, w: d, h: d }, labelPx: 0,
                paint: function () {
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(0, 0, d / 2, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha;
                    glyph(c, icon, 0, 0, d * 0.52, c.col.onAccent, p);
                }
            };
        }
        // card: the icon on top, the label below. The card narrows until it fits the area.
        var cw = Math.min(st.w * 0.9, 0.72 * m), g, lab = null;
        for (var pass = 0; pass < 3; pass++) {
            var pad = 0.12 * cw, gs = 0.5 * cw, gap = 0.08 * cw;
            lab = b.label ? fit(c, b.label, 600, cw * 0.86, 3, Math.max(c.base, cw * 0.13)) : null;
            var labH = lab ? lab.lines.length * lab.px * 1.2 : 0;
            g = { pad: pad, gs: gs, gap: gap, labH: labH, h: lab ? pad * 2 + gs + gap + labH : cw };
            if (g.h <= st.h * 0.96) break;
            cw *= st.h * 0.96 / g.h;
        }
        var box = { x: -cw / 2, y: -g.h / 2, w: cw, h: g.h };
        return {
            box: box, labelPx: lab ? lab.px : 0,
            paint: function () {
                SM.text.roundRect(ctx, box.x, box.y, box.w, box.h, 0.08 * cw);
                ctx.globalAlpha = c.alpha * p;
                ctx.fillStyle = c.col.soft;
                ctx.fill();
                ctx.lineWidth = Math.max(3, g.gs / 24);
                ctx.strokeStyle = c.col.ink;
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.stroke();
                ctx.globalAlpha = c.alpha;
                var top = box.y + (lab ? g.pad : (box.h - g.gs) / 2);
                glyph(c, icon, 0, top + g.gs / 2, g.gs, c.col.ink, p);
                if (lab) {
                    ctx.globalAlpha = c.alpha * p;
                    printLines(c, lab.lines, 0, top + g.gs + g.gap + g.labH / 2, lab.px, 600, c.col.ink, 'center');
                    ctx.globalAlpha = c.alpha;
                }
            }
        };
    }

    var PLANS = { icon: iconPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
    function plan(ctx, st, rc, stage) {
        var make = st && st.block ? PLANS[st.block.kind] : null;
        return make ? make(context(ctx, st, rc, stage)) : null;
    }

    // panel: the rounded panel behind a block on a surface of its own: the content box plus padding, cut to the area.
    function paintPanel(c, box) {
        var st = c.st, ctx = c.ctx, pad = Math.max(16, 0.05 * Math.min(st.w, st.h));
        var x0 = Math.max(box.x - pad, -st.w / 2), y0 = Math.max(box.y - pad, -st.h / 2);
        var x1 = Math.min(box.x + box.w + pad, st.w / 2), y1 = Math.min(box.y + box.h + pad, st.h / 2);
        SM.text.roundRect(ctx, x0, y0, x1 - x0, y1 - y0, PANEL_RADIUS * Math.min(st.w, st.h));
        ctx.globalAlpha = c.alpha * PANEL_ALPHA;
        ctx.fillStyle = c.col.panel;
        ctx.fill();
        ctx.globalAlpha = c.alpha;
    }

    // draw(ctx, st, rc, stage, win): paints one live block state through the camera window win.
    function draw(ctx, st, rc, stage, win) {
        if (!st || !st.block || !(st.alpha > 0)) return;
        var c = context(ctx, st, rc, stage), pl = PLANS[st.block.kind] ? PLANS[st.block.kind](c) : null;
        if (!pl) return;
        var R = SM.rig, m = R.mul(stage.matrix(win), R.mul(R.translate(st.x, st.y), R.mul(R.rotate(st.rot), [st.scale, 0, 0, st.scale, 0, 0])));
        ctx.save();
        ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);
        ctx.globalAlpha = c.alpha;
        if (st.surface === 'panel') paintPanel(c, pl.box);
        pl.paint();
        ctx.restore();
    }

    B.draw = draw;
    B.plan = plan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 280 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-bundle-parts.js scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: icon painting"
```

### Task C4: diagram layouts, arrows, flow dots, node highlight

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const light = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5, { area: { ...B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full'), surface: 'light' } });
    assert.ok(!light.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a board surface draws no panel');
});
```

with:

```js
    const light = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5, { area: { ...B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full'), surface: 'light' } });
    assert.ok(!light.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a board surface draws no panel');
});

// ---- C4: diagram layouts, arrows, flow dots, node highlight

const overlap = (a, b) => a.x < b.x + b.w - 1e-6 && b.x < a.x + a.w - 1e-6 && a.y < b.y + b.h - 1e-6 && b.y < a.y + a.h - 1e-6;
const flowNodes = n => Array.from({ length: n }, (_, i) => ({ id: 'n' + i, label: 'Node ' + i, icon: i % 2 ? 'icon.server' : 'icon.cpu' }));
const diagram = (layout, n, extra = {}) => ({ id: 'd1', kind: 'diagram', diagram: { layout, nodes: flowNodes(n), flow: false }, ...extra });

check('blocks: layoutNodes keeps the boxes inside the area and apart, hub 0 in the centre, stack 0 lowest', () => {
    const cases = [['flow', 5], ['cycle', 6], ['hub', 7], ['stack', 5], ['compare', 2], ['flow', 2], ['cycle', 3], ['hub', 3]];
    const areas = [['16:9', 'full'], ['16:9', 'right'], ['9:16', 'full'], ['9:16', 'top'], ['1:1', 'center']];
    for (const [aspect, name] of areas) {
        const stage = SM.stage.create({ aspect }), area = B.area(stage, null, 'day', name);
        for (const [layout, n] of cases) {
            const e = B.create({ ...diagram(layout, n), area }), boxes = B.layoutNodes(SM.scene.evalEntity(e, 0)), label = layout + n + ' in ' + aspect + ' ' + name;
            assert.equal(boxes.length, n, label);
            boxes.forEach((b, i) => {
                assert.ok(inside(b, area, 1e-6), label + ' box ' + i + ' inside');
                assert.ok(b.w > 0 && b.h > 0);
                for (let j = 0; j < i; j++) assert.ok(!overlap(b, boxes[j]), label + ' boxes ' + i + ' and ' + j + ' apart');
            });
            if (layout === 'hub') near(boxes[0].x + boxes[0].w / 2, area.x + area.w / 2, 1e-6, label), near(boxes[0].y + boxes[0].h / 2, area.y + area.h / 2, 1e-6, label);
            if (layout === 'stack') assert.ok(boxes.slice(1).every(b => b.y < boxes[0].y), label + ' node 0 is the lowest');
        }
    }
});

check('blocks: itemProgress of a node is 0 before its reveal and 1 after reveal + ITEM_SPAN', () => {
    const area = { x: 0, y: 0, w: 800, h: 400, surface: 'panel' };
    const e = B.create({ ...diagram('flow', 3), area, t0: 1, dur: 3, reveals: [1, 2, 3.5] });
    const p = (k, time) => B.itemProgress(SM.scene.evalEntity(e, time), k);
    [1, 2, 3.5].forEach((r, k) => {
        assert.equal(p(k, r - 0.01), 0);
        assert.equal(p(k, r), 0);
        assert.ok(p(k, r + B.ITEM_SPAN / 2) > 0 && p(k, r + B.ITEM_SPAN / 2) < 1);
        assert.equal(p(k, r + B.ITEM_SPAN), 1);
    });
    const spread = B.create({ ...diagram('flow', 3), area, t0: 1, dur: 3 });
    const q = (k, time) => B.itemProgress(SM.scene.evalEntity(spread, time), k);
    assert.equal(q(1, 1.99), 0, 'without reveals node k starts at t0 + k * dur / n');
    assert.ok(q(1, 2.2) > 0);
    assert.equal(q(2, 3.5), 1);
});

check('blocks: flow dots are periodic, move with time and stop being drawn once the diagram is not flowing', () => {
    const run = (time, flow = true, setup) => digest(paint(diagram('flow', 3, { diagram: { layout: 'flow', flow, nodes: flowNodes(3) } }), time, { setup }));
    assert.equal(run(4), run(4));
    assert.notEqual(run(4), run(4.4));
    assert.equal(run(4), run(4 + B.FLOW_PERIOD));
    assert.notEqual(run(4), run(4, false));
    assert.equal(run(4, false), run(4.4, false), 'a still diagram does not change with time');
});

check('blocks: an active mark changes only the marked node, and labels never shrink below the floor', () => {
    const base = paint(diagram('flow', 3), 9), mark = style => paint(diagram('flow', 3), 9, { setup: e => e.marks.add(0, 20, { node: 'n1', style }) });
    for (const style of ['pulse', 'outline', 'arrow']) {
        const marked = mark(style);
        assert.notEqual(digest(marked), digest(base), style);
        assert.equal(marked.filter(l => l.startsWith('fillText')).join('\n'), base.filter(l => l.startsWith('fillText')).join('\n'), style + ' prints the same labels');
    }
    const outline = mark('outline');
    let at = 0;
    for (const line of base) { at = outline.indexOf(line, at); assert.ok(at >= 0, 'the unmarked drawing is a subsequence of the marked one'); at++; }
    assert.ok(outline.length > base.length);
    // 5 nodes with 40-character labels in the narrow left area of a 9:16 frame: the font shrinks to the floor and stops there
    const stage = SM.stage.create({ aspect: '9:16', width: 1080, height: 1920 }), floor = B.LABEL_MIN * 1080 / stage.scale;
    const long = Array.from({ length: 5 }, (_, i) => ({ id: 'n' + i, label: 'Ein sehr langer Knotenname Nummer ' + i + ' hier' }));
    const log = paint({ id: 'd', kind: 'diagram', diagram: { layout: 'flow', nodes: long } }, 9, { W: 1080, H: 1920, aspect: '9:16', area: { ...B.area(stage, null, 'day', 'left') } });
    const sizes = log.filter(l => l.startsWith('set font=')).map(l => parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]));
    assert.ok(sizes.length > 0 && sizes.every(s => s >= floor - 1e-6), 'no label below ' + floor + ', got ' + Math.min(...sizes));
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.layoutNodes is not a function; flow dots and node marks drew nothing (3 new diagram checks failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out;
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
            if (!out.truncated || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        return { px: px, lines: out.lines };
```

with:

```js
    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out, words = String(text).split(/s+/);
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
            // a word that does not fit its line is broken by wrap; shrinking is better than a broken word
            var broken = words.some(function (w) { return c.ctx.measureText(w).width > maxW; });
            if ((!out.truncated && !broken) || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        return { px: px, lines: out.lines };
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        };
    }

    var PLANS = { icon: iconPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

with:

```js
        };
    }

    // ---- diagrams

    // edge(a, b, gap) = [x1, y1, x2, y2]: the arrow between two boxes along their centres, trimmed to the box edges plus gap.
    function edge(a, b, gap) {
        var ax = a.x + a.w / 2, ay = a.y + a.h / 2, dx = b.x + b.w / 2 - ax, dy = b.y + b.h / 2 - ay, len = Math.hypot(dx, dy);
        function exit(box) { return Math.min(dx ? box.w / 2 / Math.abs(dx) : 1e9, dy ? box.h / 2 / Math.abs(dy) : 1e9); }
        var t0 = exit(a) + gap / len, t1 = 1 - exit(b) - gap / len;
        return t1 - t0 > 0.02 ? [ax + dx * t0, ay + dy * t0, ax + dx * t1, ay + dy * t1] : null;
    }

    // arrow(c, seg, lw, p, color): a line that grows from its start to p of its length, with a filled head at the tip.
    function arrow(c, seg, lw, p, color) {
        var ctx = c.ctx, x1 = seg[0], y1 = seg[1], a = Math.atan2(seg[3] - y1, seg[2] - seg[0]), co = Math.cos(a), si = Math.sin(a);
        var x2 = x1 + (seg[2] - x1) * p, y2 = y1 + (seg[3] - y1) * p, hs = 4.4 * lw * Math.min(1, p * 2);
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.lineWidth = lw;
        ctx.beginPath();
        ctx.moveTo(x1, y1);
        ctx.lineTo(x2 - co * hs * 0.5, y2 - si * hs * 0.5);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(x2, y2);
        ctx.lineTo(x2 - co * hs - si * hs * 0.5, y2 - si * hs + co * hs * 0.5);
        ctx.lineTo(x2 - co * hs + si * hs * 0.5, y2 - si * hs - co * hs * 0.5);
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
    }

    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icons = c.rc.cat && c.rc.cat.icons, icon = node.icon && icons ? icons[node.icon] : null, m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; o.fit = fit(c, node.label, 700, box.w - pad * 2 - (icon ? o.gs + pad : 0), 2, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
            o.mode = 'top';
            o.fit = fit(c, node.label, 600, box.w - pad * 2, 2, px);
            o.gs = Math.min(0.46 * box.w, 0.5 * box.h, box.h - pad * 2.6 - o.fit.lines.length * o.fit.px * 1.2);
            // no room for the icon above the label: the label alone
            if (o.gs < 0.3 * box.h) { o.gs = 0; o.mode = 'plain'; o.icon = null; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        }
        else { o.mode = 'plain'; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        if (compare) {
            var pts = (node.points || []).map(function (s) { return fit(c, s, 500, box.w - pad * 3.2, 2, Math.max(c.floor, o.fit.px * 0.8)); });
            o.points = pts;
        }
        return o;
    }

    function paintNode(c, o, node, p, lw, accent, mark) {
        var ctx = c.ctx, b = o.box, cx = b.x + b.w / 2, cy = b.y + b.h / 2, draw = c.st.entry === 'draw', ink = accent ? c.col.onAccent : c.col.ink;
        var k = (mark && mark.style === 'pulse' ? 1 + 0.08 * Math.sin(4 * Math.PI * mark.u) : 1) * (draw ? 1 : 0.92 + 0.08 * p);
        ctx.save();
        ctx.translate(cx, cy);
        ctx.scale(k, k);
        ctx.translate(-cx, -cy);
        SM.text.roundRect(ctx, b.x, b.y, b.w, b.h, 0.1 * Math.min(b.w, b.h));
        ctx.globalAlpha = c.alpha * p;
        ctx.fillStyle = accent ? c.col.accent : c.col.soft;
        ctx.fill();
        ctx.globalAlpha = draw ? c.alpha : c.alpha * p;
        ctx.strokeStyle = c.col.ink;
        ctx.lineWidth = lw;
        ctx.setLineDash(draw && p < 1 ? [r3(2 * (b.w + b.h) * p), 2 * (b.w + b.h)] : []);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.globalAlpha = c.alpha * p;
        var f = o.fit, lh = f.lines.length * f.px * 1.2, top = b.y + o.pad, gs = o.gs;
        if (o.mode === 'top') {
            var y0 = cy - (gs + o.pad * 0.6 + lh) / 2;
            glyph(c, o.icon, cx, y0 + gs / 2, gs, ink, draw ? p : 1);
            printLines(c, f.lines, cx, y0 + gs + o.pad * 0.6 + lh / 2, f.px, 600, ink, 'center');
        } else if (o.mode === 'side') {
            glyph(c, o.icon, b.x + o.pad + gs / 2, cy, gs, ink, draw ? p : 1);
            printLines(c, f.lines, b.x + o.pad * 2 + gs + (b.w - o.pad * 3 - gs) / 2, cy, f.px, 600, ink, 'center');
        } else if (o.mode === 'plain') printLines(c, f.lines, cx, cy, f.px, 600, ink, 'center');
        else {
            var head = Math.max(gs, lh), x0 = b.x + o.pad + (o.icon ? gs + o.pad : 0), y = top + head / 2;
            if (o.icon) glyph(c, o.icon, b.x + o.pad + gs / 2, y, gs, c.col.accent, draw ? p : 1);
            printLines(c, f.lines, x0 + (b.x + b.w - o.pad - x0) / 2, y, f.px, 700, ink, 'center');
            y = top + head + o.pad * 0.5;
            ctx.strokeStyle = c.col.ink;
            ctx.lineWidth = lw * 0.5;
            ctx.beginPath();
            ctx.moveTo(b.x + o.pad, y);
            ctx.lineTo(b.x + b.w - o.pad, y);
            ctx.stroke();
            o.points.forEach(function (pt) {
                var ph = pt.lines.length * pt.px * 1.2;
                y += o.pad * 0.5 + ph / 2;
                ctx.fillStyle = c.col.accent;
                ctx.beginPath();
                ctx.arc(b.x + o.pad + pt.px * 0.2, y, pt.px * 0.18, 0, Math.PI * 2);
                ctx.fill();
                printLines(c, pt.lines, b.x + o.pad + pt.px * 0.7, y, pt.px, 500, ink, 'left');
                y += ph / 2;
            });
        }
        ctx.globalAlpha = c.alpha;
        if (mark && mark.style === 'outline') {
            ctx.strokeStyle = c.col.accent;
            ctx.lineWidth = lw * 1.6;
            SM.text.roundRect(ctx, b.x - lw * 1.4, b.y - lw * 1.4, b.w + lw * 2.8, b.h + lw * 2.8, 0.1 * Math.min(b.w, b.h) + lw);
            ctx.stroke();
        } else if (mark && mark.style === 'arrow') {
            var tip = b.y - lw * 3 - lw * 2 * Math.sin(4 * Math.PI * mark.u);
            ctx.fillStyle = c.col.accent;
            ctx.beginPath();
            ctx.moveTo(cx, tip);
            ctx.lineTo(cx - lw * 3, tip - lw * 5);
            ctx.lineTo(cx + lw * 3, tip - lw * 5);
            ctx.closePath();
            ctx.fill();
        }
        ctx.restore();
    }

    function diagramPlan(c) {
        var st = c.st, d = st.block.diagram, n = d.nodes.length, L = B.layout(d.layout, n, st.w, st.h), compare = d.layout === 'compare';
        var m = Math.min.apply(null, L.boxes.map(function (b) { return Math.min(b.w, b.h); }));
        var px = clamp(0.11 * L.boxes[0].w, c.base, c.base * 1.7), items, i;
        items = d.nodes.map(function (node, k) { return nodeLayout(c, node, L.boxes[k], px, compare); });
        for (i = 0, px = Infinity; i < n; i++) px = Math.min(px, items[i].fit.px);
        items = d.nodes.map(function (node, k) { return nodeLayout(c, node, L.boxes[k], px, compare); });
        var x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
        L.boxes.forEach(function (b) { x0 = Math.min(x0, b.x); y0 = Math.min(y0, b.y); x1 = Math.max(x1, b.x + b.w); y1 = Math.max(y1, b.y + b.h); });
        var lw = Math.max(3, Math.min(0.045 * m, 12) / 2);
        return {
            box: { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }, labelPx: px,
            paint: function () {
                var ctx = c.ctx, flow = d.flow === true;
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                L.links.forEach(function (lk) {
                    var seg = edge(L.boxes[lk[0]], L.boxes[lk[1]], lw * 2), p = B.itemProgress(st, lk[1]);
                    if (!seg || !(p > 0)) return;
                    ctx.globalAlpha = c.alpha;
                    arrow(c, seg, lw, p, c.col.ink);
                    if (!flow || p < 1) return;
                    for (var j = 0; j < 2; j++) {
                        var f = (Math.round((st.time / B.FLOW_PERIOD + j / 2) * 1e6) / 1e6) % 1, hs = 4.4 * lw;
                        var len = Math.hypot(seg[2] - seg[0], seg[3] - seg[1]), u = Math.max(0, len - hs) / len;
                        ctx.globalAlpha = c.alpha * Math.sin(Math.PI * f);
                        ctx.fillStyle = c.col.accent;
                        ctx.beginPath();
                        ctx.arc(seg[0] + (seg[2] - seg[0]) * u * f, seg[1] + (seg[3] - seg[1]) * u * f, lw * 1.1, 0, Math.PI * 2);
                        ctx.fill();
                    }
                });
                d.nodes.forEach(function (node, k) {
                    var p = B.itemProgress(st, k), mark = st.mark && st.mark.node === node.id ? st.mark : null;
                    if (p > 0) paintNode(c, items[k], node, p, lw, d.layout === 'hub' && k === 0, mark);
                });
            }
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        return SM.ease('ease-out', clamp(p, 0, 1));
    }

    // ---- create

    var DEF_ONLY = { area: 1, t0: 1, dur: 1, entry: 1, reveals: 1, hidden: 1 };
```

with:

```js
        return SM.ease('ease-out', clamp(p, 0, 1));
    }

    // ---- diagram layouts

    // ring(n, W, H, size, from): n boxes of size x size * 1.1 on the ellipse that fills the area, from the top, clockwise.
    function ring(n, W, H, s, first, step, into) {
        var bw = s * 1.1, rx = (W - bw) / 2, ry = (H - s) / 2;
        for (var i = 0; i < n; i++) {
            var a = -Math.PI / 2 + (first + i) * step;
            into.push({ x: rx * Math.cos(a) - bw / 2, y: ry * Math.sin(a) - s / 2, w: bw, h: s });
        }
    }

    // layout(layout, n, W, H) = {boxes: [{x, y, w, h}], links: [[from, to]]}: the node boxes around the centre of a W x H area at scale
    // 1 (x and y are the top left corner), and the arrows between them. The boxes never overlap and never leave the area.
    function layout(kind, n, W, H) {
        var pad = 0.04 * Math.min(W, H), boxes = [], links = [], m, i, nw, nh, s;
        W -= 2 * pad;   // the layout keeps a margin to the area, so outlines and highlights stay inside it
        H -= 2 * pad;
        m = Math.min(W, H);
        if (kind === 'flow' && W >= H) {
            nw = Math.min(0.665 * H, W / (n + 0.45 * (n - 1)));
            nh = Math.min(0.7 * H, nw * 1.1);
            var left = -(n * nw + 0.45 * nw * (n - 1)) / 2;
            for (i = 0; i < n; i++) { boxes.push({ x: left + i * 1.45 * nw, y: -nh / 2, w: nw, h: nh }); if (i) links.push([i - 1, i]); }
        } else if (kind === 'flow') {
            nh = Math.min(H / (n + 0.45 * (n - 1)), 0.5 * W);
            nw = Math.min(0.92 * W, nh * 2.6);
            var top = -(n * nh + 0.45 * nh * (n - 1)) / 2;
            for (i = 0; i < n; i++) { boxes.push({ x: -nw / 2, y: top + i * 1.45 * nh, w: nw, h: nh }); if (i) links.push([i - 1, i]); }
        } else if (kind === 'stack') {
            nh = Math.min(H / (n + 0.12 * (n - 1)), 0.26 * H);
            nw = Math.min(0.92 * W, nh * 5.5);
            var total = n * nh + 0.12 * nh * (n - 1);
            for (i = 0; i < n; i++) boxes.push({ x: -nw / 2, y: total / 2 - nh - i * 1.12 * nh, w: nw, h: nh });
        } else if (kind === 'cycle') {
            ring(n, W, H, m * ({ 3: 0.28, 4: 0.26, 5: 0.24 }[n] || 0.21), 0, 2 * Math.PI / n, boxes);
            for (i = 0; i < n; i++) links.push([i, (i + 1) % n]);
        } else if (kind === 'hub') {
            s = m * 0.26;
            boxes.push({ x: -s * 0.55, y: -s / 2, w: s * 1.1, h: s });
            ring(n - 1, W, H, m * ({ 2: 0.24, 3: 0.24, 4: 0.22, 5: 0.22 }[n - 1] || 0.2), 0, 2 * Math.PI / (n - 1), boxes);
            for (i = 1; i < n; i++) links.push([0, i]);
        } else {
            var gap = Math.max(24, 0.05 * W);
            nw = Math.min((W - gap) / 2, 1.25 * H);
            nh = Math.min(H, nw * 1.3);
            boxes.push({ x: -gap / 2 - nw, y: -nh / 2, w: nw, h: nh }, { x: gap / 2, y: -nh / 2, w: nw, h: nh });
        }
        return { boxes: boxes, links: links };
    }

    // layoutNodes(st) = [{id, x, y, w, h}]: the node boxes of an evaluated diagram state in design pixels at scale 1.
    function layoutNodes(st) {
        var d = st.block.diagram, l = layout(d.layout, d.nodes.length, st.w, st.h);
        return d.nodes.map(function (n, i) { var b = l.boxes[i]; return { id: n.id, x: st.x + b.x, y: st.y + b.y, w: b.w, h: b.h }; });
    }

    // ---- create

    var DEF_ONLY = { area: 1, t0: 1, dur: 1, entry: 1, reveals: 1, hidden: 1 };
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, layout: layout, layoutNodes: layoutNodes
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 284 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: diagram layouts, arrows, flow dots, node highlight"
```

### Task C5: charts

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const sizes = log.filter(l => l.startsWith('set font=')).map(l => parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]));
    assert.ok(sizes.length > 0 && sizes.every(s => s >= floor - 1e-6), 'no label below ' + floor + ', got ' + Math.min(...sizes));
});
```

with:

```js
    const sizes = log.filter(l => l.startsWith('set font=')).map(l => parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]));
    assert.ok(sizes.length > 0 && sizes.every(s => s >= floor - 1e-6), 'no label below ' + floor + ', got ' + Math.min(...sizes));
});

// ---- C5: charts

const chartDef = (type, values, extra = {}) => ({ id: 'c1', kind: 'chart', chart: { type, labels: values.map((_, i) => 'L' + i), values, ...extra } });

check('blocks: bar heights are proportional to the values at the end and half as high at u = 0.5', () => {
    const def = chartDef('bar', [2, 4, 8], { unit: '%' });
    const heights = time => paint(def, time).filter(l => l.startsWith('fillRect(')).map(l => parseFloat(l.slice(9, -1).split(',')[3]));
    const full = heights(5), half = heights(timeFor(0.5));
    assert.equal(full.length, 3);
    near(full[1] / full[0], 2, 1e-3);
    near(full[2] / full[0], 4, 1e-3);
    full.forEach((h, i) => near(half[i], h * 0.5, 0.01, 'bar ' + i));
    assert.ok(paint(def, 5).some(l => l.startsWith('fillText(8%,')), 'the value is printed with its unit');
    assert.equal(heights(0).reduce((a, b) => a + b, 0), 0, 'nothing has grown at u = 0');
});

check('blocks: a counter shows the eased value rounded to its decimals, plus the unit', () => {
    const def = { ...chartDef('counter', [3.5], { unit: 'ms', title: 'Zeit' }) };
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full');
    const st = SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), timeFor(0.5)), p = B.itemProgress(st, 0);
    near(p, 0.5, 1e-9);
    const texts = time => paint(def, time).filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.ok(texts(timeFor(0.5)).includes((3.5 * p).toFixed(1) + ' ms'), texts(timeFor(0.5)).join('|'));
    assert.ok(texts(5).includes('3.5 ms'));
    assert.ok(paint(chartDef('counter', [45], { unit: '%' }), 5).some(l => l.startsWith('fillText(45%,')), 'a percent sign follows without a space');
});

check('blocks: a pie is a donut of arcs with an inner radius, a line chart reveals with a dash', () => {
    const arcs = paint(chartDef('pie', [3, 2, 1]), 5).filter(l => l.startsWith('arc(') && !l.includes(',0,6.283'));
    const radii = new Set(arcs.map(l => l.slice(4, -1).split(',')[2]));
    assert.equal(radii.size, 2, 'an outer and an inner radius');
    const [a, b] = [...radii].map(Number).sort((x, y) => x - y);
    assert.ok(a > 0 && a < b);
    assert.equal(arcs.length, 6, 'two arcs per slice');
    assert.ok(paint(chartDef('pie', [3, 2, 1]), 5).some(l => l.startsWith('fillText(L0 50%,')), 'the legend carries the share');
    const line = time => paint(chartDef('line', [1, 3, 2, 5]), time).filter(l => l.startsWith('setLineDash('));
    const mid = line(timeFor(0.5));
    assert.ok(mid.some(l => l !== 'setLineDash([])'), 'a partial dash at u = 0.5');
    assert.ok(line(5).every(l => l === 'setLineDash([])'), 'no dash at the end');
    assert.ok(paint(chartDef('line', [1, 3, 2, 5]), 5).filter(l => l.startsWith('lineTo(')).length >= 3, 'the polyline is drawn');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: bar, counter, pie and line charts drew nothing (3 new chart checks failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
            ink: dark ? '#f4f5eb' : (p.text || '#1f2a44'), accent: dark ? (p.accent2 || '#36cfc9') : (p.accent || '#ff7a45'),
            accent2: dark ? (p.accent || '#ff7a45') : (p.accent2 || '#36cfc9'),
            soft: dark ? 'rgba(255,255,255,0.10)' : (p.bg2 || '#ffe0a3'), bg: p.bg || '#fff7e6', onAccent: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6'),
            panel: p.bg || '#fff7e6'
        };
    }
```

with:

```js
            ink: dark ? '#f4f5eb' : (p.text || '#1f2a44'), accent: dark ? (p.accent2 || '#36cfc9') : (p.accent || '#ff7a45'),
            accent2: dark ? (p.accent || '#ff7a45') : (p.accent2 || '#36cfc9'),
            soft: dark ? 'rgba(255,255,255,0.10)' : (p.bg2 || '#ffe0a3'), bg: p.bg || '#fff7e6', onAccent: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6'),
            panel: p.bg || '#fff7e6', base: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6')
        };
    }
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

with:

```js
        };
    }

    // ---- charts

    function numText(v) { return Math.abs(v - Math.round(v)) < 1e-9 ? String(Math.round(v)) : String(Math.round(v * 100) / 100); }
    function unitText(u) { return u ? (u.charAt(0) === '%' || u.charCodeAt(0) === 176 ? u : ' ' + u) : ''; }
    function decimals(v) { var s = String(Math.abs(v)), i = s.indexOf('.'); return i < 0 || s.indexOf('e') >= 0 ? 0 : Math.min(3, s.length - i - 1); }

    // fitSet(c, texts, weight, maxW, maxLines, px0) = {px, lines: [[line]]}: one font size for all texts, so the labels of a chart match.
    function fitSet(c, texts, weight, maxW, maxLines, px0) {
        var px = px0;
        texts.forEach(function (t) { px = Math.min(px, fit(c, t, weight, maxW, maxLines, px0).px); });
        return { px: px, lines: texts.map(function (t) { return fit(c, t, weight, maxW, maxLines, px).lines; }) };
    }

    // axisPlan: bar and line charts on a zero baseline. The labels sit under the baseline, the values above the marks.
    function axisPlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, line = ch.type === 'line';
        var step = Math.ceil(n / 8), cell = (f.x1 - f.x0) / n, shown = [], i;
        for (i = 0; i < n; i += step) shown.push(i);
        var lab = fitSet(c, shown.map(function (k) { return String(ch.labels[k]); }), 600, cell * step * 0.92, 2, c.base);
        var maxLines = Math.max.apply(null, lab.lines.map(function (l) { return l.length; })), valPx = lab.px * 0.9;
        var py0 = f.top + valPx * 1.8, py1 = f.y1 - maxLines * lab.px * 1.2 - lab.px * 0.6;
        var lo = Math.min(0, Math.min.apply(null, vals)), hi = Math.max(0, Math.max.apply(null, vals));
        if (hi === lo) hi = lo + 1;
        var yOf = function (v) { return py0 + (hi - v) / (hi - lo) * (py1 - py0); }, yz = yOf(0), xOf = function (k) { return f.x0 + cell * (k + 0.5); };
        var value = function (k, y, a) {
            ctx.globalAlpha = c.alpha * a;
            printLines(c, [numText(vals[k]) + f.unit], xOf(k), vals[k] >= 0 ? y - valPx * 0.8 : y + valPx * 0.8, valPx, 700, c.col.ink, 'center');
        };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.globalAlpha = c.alpha;
                ctx.strokeStyle = c.col.ink;
                ctx.lineWidth = f.lw;
                ctx.beginPath();
                ctx.moveTo(f.x0, yz);
                ctx.lineTo(f.x1, yz);
                ctx.stroke();
                shown.forEach(function (k, j) { printLines(c, lab.lines[j], xOf(k), py1 + lab.px * 0.6 + lab.lines[j].length * lab.px * 0.6, lab.px, 600, c.col.ink, 'center'); });
                if (!line) {
                    var bw = Math.min(cell * 0.58, 0.2 * st.h);
                    vals.forEach(function (v, k) {
                        var hgt = Math.abs(yz - yOf(v)) * p;
                        ctx.globalAlpha = c.alpha;
                        ctx.fillStyle = c.col.accent;
                        ctx.fillRect(xOf(k) - bw / 2, v >= 0 ? yz - hgt : yz, bw, hgt);
                        value(k, v >= 0 ? yz - hgt : yz + hgt, clamp((p - 0.6) / 0.4, 0, 1));
                    });
                    return;
                }
                var total = 0;
                for (var k = 1; k < n; k++) total += Math.hypot(xOf(k) - xOf(k - 1), yOf(vals[k]) - yOf(vals[k - 1]));
                ctx.globalAlpha = c.alpha;
                ctx.strokeStyle = c.col.accent;
                ctx.lineWidth = f.lw * 1.6;
                ctx.setLineDash(p < 1 ? [r3(total * p), r3(total)] : []);
                ctx.beginPath();
                vals.forEach(function (v, j) { if (j) ctx.lineTo(xOf(j), yOf(v)); else ctx.moveTo(xOf(0), yOf(v)); });
                ctx.stroke();
                ctx.setLineDash([]);
                vals.forEach(function (v, j) {
                    var a = clamp(p * (n - 1) - j + 1, 0, 1);
                    if (!(a > 0) || n > 12) return;
                    ctx.globalAlpha = c.alpha * a;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(xOf(j), yOf(v), f.lw * 1.8, 0, Math.PI * 2);
                    ctx.fill();
                    if (n <= 6) value(j, yOf(v) - f.lw * 2, a);
                });
            }
        };
    }

    // piePlan: a donut with a legend; the slices sweep clockwise from the top while p grows.
    function piePlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, sum = 0;
        vals.forEach(function (v) { sum += v; });
        var wide = f.x1 - f.x0 > (f.y1 - f.top) * 1.35, R = wide ? Math.min((f.y1 - f.top) / 2, (f.x1 - f.x0) * 0.3) : Math.min((f.x1 - f.x0) / 2, (f.y1 - f.top) * 0.36);
        var rows = ch.labels.map(function (l, k) { return String(l) + ' ' + Math.round(100 * vals[k] / sum) + '%'; });
        var lab = fitSet(c, rows, 600, wide ? f.x1 - f.x0 - R * 2.45 - c.base * 1.4 : f.x1 - f.x0 - c.base * 1.4, 1, c.base), rowH = lab.px * 1.9;
        // the donut and its legend are centred as one group
        var rowsW = Math.max.apply(null, rows.map(function (s) { return widthOf(c, s, 600, lab.px); })) + lab.px * 1.3;
        var cx = wide ? f.x0 + (f.x1 - f.x0 - R * 2.45 - rowsW) / 2 + R : (f.x0 + f.x1) / 2, cy = wide ? (f.top + f.y1) / 2 : f.top + R, r = R * 0.58;
        var legendX = wide ? cx + R * 1.45 : (f.x0 + f.x1 - rowsW) / 2, legendTop = wide ? 0 : cy + R + R * 0.2;
        var top = wide ? cy - n * rowH / 2 : legendTop, tint = function (k) { return [c.col.accent, c.col.accent2, c.col.ink][k % 3]; };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                var a0 = -Math.PI / 2, sweep = a0 + 2 * Math.PI * p;
                vals.forEach(function (v, k) {
                    var a1 = a0 + 2 * Math.PI * v / sum, end = Math.min(a1, sweep);
                    if (end > a0) {
                        ctx.globalAlpha = c.alpha * (k < 3 ? 1 : 0.55);
                        ctx.fillStyle = tint(k);
                        ctx.strokeStyle = c.col.base;
                        ctx.lineWidth = f.lw * 0.8;
                        ctx.lineJoin = 'round';
                        ctx.beginPath();
                        ctx.arc(cx, cy, R, a0, end);
                        ctx.arc(cx, cy, r, end, a0, true);
                        ctx.closePath();
                        ctx.fill();
                        ctx.stroke();
                    }
                    var show = clamp((p - 0.3) / 0.7, 0, 1), y = top + rowH * (k + 0.5);
                    ctx.globalAlpha = c.alpha * show * (k < 3 ? 1 : 0.55);
                    ctx.fillStyle = tint(k);
                    SM.text.roundRect(ctx, legendX, y - lab.px * 0.4, lab.px * 0.8, lab.px * 0.8, lab.px * 0.2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha * show;
                    printLines(c, lab.lines[k], legendX + lab.px * 1.3, y, lab.px, 600, c.col.ink, 'left');
                    a0 = a1;
                });
            }
        };
    }

    // counterPlan: one big number that counts up to its value.
    function counterPlan(c, f) {
        var st = c.st, ch = st.block.chart, v = ch.values[0], p = B.itemProgress(st, 0), whole = v.toFixed(decimals(v)) + unitText(ch.unit);
        var cap = ch.labels[0] ? fit(c, String(ch.labels[0]), 600, f.x1 - f.x0, 2, c.base * 1.2) : null, ctx = c.ctx;
        var px = Math.min(0.42 * (f.y1 - f.top), 0.9 * (f.x1 - f.x0) / Math.max(1, whole.length * 0.62)), mid = (f.top + f.y1) / 2 - (cap ? cap.px * 0.7 : 0);
        return {
            box: f.box, labelPx: cap ? cap.px : 0,
            paint: function () {
                f.title();
                ctx.globalAlpha = c.alpha;
                printLines(c, [(v * p).toFixed(decimals(v)) + unitText(ch.unit)], 0, mid, px, 700, c.col.accent, 'center');
                if (cap) printLines(c, cap.lines, 0, mid + px * 0.62 + cap.px * 1.1, cap.px, 600, c.col.ink, 'center');
            }
        };
    }

    function chartPlan(c) {
        var st = c.st, ch = st.block.chart, pad = 0.06 * Math.min(st.w, st.h), f = { x0: -st.w / 2 + pad, x1: st.w / 2 - pad, y1: st.h / 2 - pad };
        var title = ch.title ? fit(c, ch.title, 700, f.x1 - f.x0, 1, c.base * 1.25) : null;
        f.top = -st.h / 2 + pad + (title ? title.px * 1.6 : 0);
        f.box = { x: -st.w / 2, y: -st.h / 2, w: st.w, h: st.h };
        f.lw = Math.max(3, Math.min(st.w, st.h) * 0.008);
        f.unit = unitText(ch.unit);
        f.title = function () { if (title) printLines(c, title.lines, 0, -st.h / 2 + pad + title.px * 0.6, title.px, 700, c.col.ink, 'center'); };
        var plan = ch.type === 'pie' ? piePlan(c, f) : (ch.type === 'counter' ? counterPlan(c, f) : axisPlan(c, f));
        if (title) plan.labelPx = Math.min(plan.labelPx || title.px, title.px);
        return plan;
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 287 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: charts"
```

### Task C6: headline and list

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    assert.ok(line(5).every(l => l === 'setLineDash([])'), 'no dash at the end');
    assert.ok(paint(chartDef('line', [1, 3, 2, 5]), 5).filter(l => l.startsWith('lineTo(')).length >= 3, 'the polyline is drawn');
});
```

with:

```js
    assert.ok(line(5).every(l => l === 'setLineDash([])'), 'no dash at the end');
    assert.ok(paint(chartDef('line', [1, 3, 2, 5]), 5).filter(l => l.startsWith('lineTo(')).length >= 3, 'the polyline is drawn');
});

// ---- C6: headline and list

check('blocks: parseHeadline finds the words and the marked word', () => {
    assert.deepEqual(plain(B.parseHeadline('Ein *wichtiges* Wort')), { words: ['Ein', 'wichtiges', 'Wort'], mark: 1 });
    assert.deepEqual(plain(B.parseHeadline('Ohne Markierung')), { words: ['Ohne', 'Markierung'], mark: -1 });
    assert.equal(B.parseHeadline('  *Eins*  ').mark, 0);
    assert.equal(B.check({ id: 'h', kind: 'headline', text: ' * ' }, cat).field, 'text', 'a headline without a word is refused');
});

check('blocks: headline words rise in order and the marked word has an accent marker behind it', () => {
    const def = { id: 'h', kind: 'headline', text: 'Ein *wichtiges* Wort', dur: 1.5 };
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'top'), e = B.create({ ...def, area, t0: 0 });
    const prog = time => [0, 1, 2].map(k => B.itemProgress(SM.scene.evalEntity(e, time), k));
    const mid = prog(0.6);
    assert.ok(mid[0] >= mid[1] && mid[1] >= mid[2] && mid[0] > mid[2], 'in order: ' + mid.join(', '));
    assert.deepEqual(prog(0), [0, 0, 0]);
    assert.deepEqual(prog(5), [1, 1, 1]);
    const log = paint(def, 5), at = text => log.findIndex(l => l.startsWith('fillText(' + text + ','));
    assert.deepEqual(['Ein', 'wichtiges', 'Wort'].map(at).every(i => i >= 0), true);
    assert.ok(at('Ein') < at('wichtiges') && at('wichtiges') < at('Wort'));
    const rect = log.findIndex(l => l.startsWith('fillRect(')), fill = log.lastIndexOf('set fillStyle=' + PALETTE.accent, at('wichtiges'));
    assert.ok(rect >= 0 && rect < at('wichtiges') && fill >= 0 && fill < rect, 'the accent rect comes before the marked word');
    assert.equal(paint({ ...def, text: 'Ohne Marker' }, 5).filter(l => l.startsWith('fillRect(')).length, 0);
    assert.equal(paint(def, 0).filter(l => l.startsWith('fillText(')).length, 0, 'no word is printed before its turn');
});

check('blocks: a list reveals its lines in order', () => {
    const def = { id: 'l', kind: 'list', text: 'Laden\nRechnen\nAntworten\nSpeichern', dur: 2 };
    const texts = time => paint(def, time).filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts(5), ['Laden', 'Rechnen', 'Antworten', 'Speichern']);
    assert.deepEqual(texts(0), []);
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'right'), e = B.create({ ...def, area, t0: 0 });
    const prog = time => [0, 1, 2, 3].map(k => B.itemProgress(SM.scene.evalEntity(e, time), k));
    const mid = prog(0.8);
    assert.ok(mid[0] >= mid[1] && mid[1] >= mid[2] && mid[2] >= mid[3] && mid[0] > mid[3], mid.join(', '));
    assert.deepEqual(texts(0.3), ['Laden']);
    assert.equal(paint(def, 5).filter(l => l.startsWith('arc(')).length, 4, 'one bullet per line');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.parseHeadline is not a function; headline words, marker and list lines were not drawn (3 new checks failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        return plan;
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

with:

```js
        return plan;
    }

    // ---- headline and list

    // headlinePlan: the words wrap into rows, centred; each word rises into place in its turn, the marked word gets a marker.
    function headlinePlan(c) {
        var st = c.st, hp = B.parseHeadline(st.block.text), words = hp.words, ctx = c.ctx, px = Math.min(c.base * 2.4, st.h * 0.4), rows, ww;
        var rtl = SM.ranges.dirOf(st.block.text) === 'rtl', maxW = st.w * 0.94;
        for (;;) {
            ww = words.map(function (w) { return widthOf(c, w, 700, px); });
            var space = widthOf(c, ' ', 700, px), row = null;
            rows = [];
            words.forEach(function (w, k) {
                if (!row || row.w + space + ww[k] > maxW) rows.push(row = { ix: [], w: -space });
                row.ix.push(k);
                row.w += space + ww[k];
            });
            if ((rows.length <= 3 && rows.every(function (r) { return r.w <= maxW; }) && rows.length * px * 1.25 <= st.h) || px <= c.floor * 1.5) break;
            px = Math.max(c.floor * 1.5, px * 0.94);
        }
        var lh = px * 1.25, width = Math.max.apply(null, rows.map(function (r) { return r.w; })), height = rows.length * lh;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var space = widthOf(c, ' ', 700, px);
                rows.forEach(function (r, i) {
                    var x = -r.w / 2, y = -height / 2 + lh * (i + 0.5), order = rtl ? r.ix.slice().reverse() : r.ix;
                    order.forEach(function (k) {
                        var p = B.itemProgress(st, k), cx = x + ww[k] / 2;
                        x += ww[k] + space;
                        if (!(p > 0)) return;
                        if (k === hp.mark) {
                            ctx.globalAlpha = c.alpha * 0.5 * p;
                            ctx.fillStyle = c.col.accent;
                            ctx.fillRect(cx - ww[k] / 2 - px * 0.1, y + px * 0.02, (ww[k] + px * 0.2) * p, px * 0.46);
                        }
                        ctx.globalAlpha = c.alpha * p;
                        printLines(c, [words[k]], cx, y + (1 - p) * px * 0.35, px, 700, c.col.ink, 'center');
                    });
                });
            }
        };
    }

    // listPlan: up to four lines with an accent bullet, each sliding in from the left in order.
    function listPlan(c) {
        var st = c.st, rows = B.lines(st.block.text), n = rows.length, ctx = c.ctx;
        var lab = fitSet(c, rows, 600, st.w * 0.9 - c.base, 2, clamp(st.h / (n * 2), c.base, c.base * 1.8)), px = lab.px;
        var heights = lab.lines.map(function (l) { return l.length * px * 1.2 + px * 0.7; }), height = heights.reduce(function (a, b) { return a + b; }, 0);
        var width = Math.max.apply(null, lab.lines.map(function (l) { return Math.max.apply(null, l.map(function (s) { return widthOf(c, s, 600, px); })); })) + px * 0.9;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = (1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(-width / 2 + px * 0.2 + dx, mid, px * 0.2, 0, Math.PI * 2);
                    ctx.fill();
                    printLines(c, lines, -width / 2 + px * 0.9 + dx, mid, px, 600, c.col.ink, 'left');
                });
            }
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan, headline: headlinePlan, list: listPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        return typeof text === 'string' ? text.split('\n').map(function (l) { return l.replace(/\s+$/, ''); }).filter(function (l) { return l.length > 0; }) : [];
    }

    function wordCount(text) { return String(text || '').replace(/\*/g, ' ').split(/\s+/).filter(Boolean).length; }

    function itemCount(b) {
        if (b.kind === 'diagram') return b.diagram && Array.isArray(b.diagram.nodes) ? b.diagram.nodes.length : 0;
        if (b.kind === 'list') return lines(b.text).length;
        if (b.kind === 'headline') return wordCount(b.text);
        return 1;
    }
```

with:

```js
        return typeof text === 'string' ? text.split('\n').map(function (l) { return l.replace(/\s+$/, ''); }).filter(function (l) { return l.length > 0; }) : [];
    }

    // parseHeadline(text) = {words, mark}: the words of a headline; the word inside asterisks is the marked one (mark is its index, or -1).
    function parseHeadline(text) {
        var words = [], mark = -1;
        String(text || '').split(/\s+/).filter(Boolean).forEach(function (token) {
            var word = token.replace(/\*/g, '');
            if (!word) return;
            if (mark < 0 && token.indexOf('*') >= 0) mark = words.length;
            words.push(word);
        });
        return { words: words, mark: mark };
    }

    function itemCount(b) {
        if (b.kind === 'diagram') return b.diagram && Array.isArray(b.diagram.nodes) ? b.diagram.nodes.length : 0;
        if (b.kind === 'list') return lines(b.text).length;
        if (b.kind === 'headline') return parseHeadline(b.text).words.length;
        return 1;
    }
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        } else if (def.kind === 'diagram') bad = checkDiagram(def.diagram, cat);
        else if (def.kind === 'chart') bad = checkChart(def.chart);
        else if (def.kind === 'headline') {
            if (typeof def.text !== 'string' || !def.text.trim() || def.text.length > HEADLINE_MAX) bad = problem('text', 'A headline has 1 to ' + HEADLINE_MAX + ' characters. Shorten it.');
        } else {
            var rows = lines(def.text);
            if (!rows.length || rows.length > LIST_MAX) bad = problem('text', 'A list has 1 to ' + LIST_MAX + ' lines, got ' + rows.length + '. Split the list.');
```

with:

```js
        } else if (def.kind === 'diagram') bad = checkDiagram(def.diagram, cat);
        else if (def.kind === 'chart') bad = checkChart(def.chart);
        else if (def.kind === 'headline') {
            if (typeof def.text !== 'string' || !parseHeadline(def.text).words.length || def.text.length > HEADLINE_MAX) bad = problem('text', 'A headline has 1 to ' + HEADLINE_MAX + ' characters. Shorten it.');
        } else {
            var rows = lines(def.text);
            if (!rows.length || rows.length > LIST_MAX) bad = problem('text', 'A list has 1 to ' + LIST_MAX + ' lines, got ' + rows.length + '. Split the list.');
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        switch (def.kind) {
            case 'diagram': return 0.5 + 0.6 * itemCount(def);
            case 'chart': return 1.2;
            case 'headline': return 0.4 + 0.15 * wordCount(def.text);
            case 'list': return 0.5 * itemCount(def);
            default: return 0.8;
        }
```

with:

```js
        switch (def.kind) {
            case 'diagram': return 0.5 + 0.6 * itemCount(def);
            case 'chart': return 1.2;
            case 'headline': return 0.4 + 0.15 * parseHeadline(def.text).words.length;
            case 'list': return 0.5 * itemCount(def);
            default: return 0.8;
        }
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, layout: layout, layoutNodes: layoutNodes
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 290 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: headline and list"
```

### Task C7: block measurement

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    assert.deepEqual(texts(0.3), ['Laden']);
    assert.equal(paint(def, 5).filter(l => l.startsWith('arc(')).length, 4, 'one bullet per line');
});
```

with:

```js
    assert.deepEqual(texts(0.3), ['Laden']);
    assert.equal(paint(def, 5).filter(l => l.startsWith('arc(')).length, 4, 'one bullet per line');
});

// ---- C7: measurement

check('blocks: measure gives a box inside the area and a label of at least LABEL_MIN, halved at scale 0.5', () => {
    const defs = [
        { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'card', label: 'Prozessor' },
        diagram('flow', 3), diagram('cycle', 4), diagram('hub', 5), diagram('stack', 3), diagram('compare', 2),
        chartDef('bar', [2, 4, 8]), chartDef('pie', [3, 2, 1]), chartDef('line', [1, 3, 2]), chartDef('counter', [45], { unit: '%' }),
        { id: 'h', kind: 'headline', text: 'Ein *wichtiges* Wort' }, { id: 'l', kind: 'list', text: 'Laden\nRechnen\nAntworten' }
    ];
    for (const [aspect, W, H] of [['16:9', 1920, 1080], ['9:16', 1080, 1920]]) {
        const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect), win = stage.baseWindow();
        for (const name of ['full', 'right']) {
            const area = B.area(stage, null, 'day', name);
            for (const def of defs) {
                const label = def.kind + (def.diagram ? ' ' + def.diagram.layout : '') + ' ' + aspect + ' ' + name;
                const measureAt = scale => {
                    const e = B.create({ ...def, area, t0: 0, dur: 1 });
                    e.scale.init = scale;
                    return B.measure(new FakeCanvas(W, H).ctx, SM.scene.evalEntity(e, 5), rc, stage, win);
                };
                const m = measureAt(1), half = measureAt(0.5);
                assert.ok(inside(m.box, area, 1e-6), label + ' box inside the area: ' + JSON.stringify(m.box));
                assert.ok(m.box.w > 0 && m.box.h > 0, label);
                if (m.labelPx > 0) {
                    assert.ok(m.labelPx >= B.LABEL_MIN * Math.min(W, H) - 1e-6, label + ' label ' + m.labelPx);
                    near(half.labelPx, m.labelPx / 2, 1e-6, label + ' halves');
                    near(half.box.w, m.box.w / 2, 1e-6, label);
                } else assert.ok(def.kind === 'icon' || def.kind === 'chart', label + ' has a label');
            }
        }
    }
    const stage = SM.stage.create({ aspect: '16:9' }), rc = rcOf(1920, 1080), area = B.area(stage, null, 'day', 'right');
    const text = def => B.measure(new FakeCanvas(1920, 1080).ctx, SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), 5), rc, stage, stage.baseWindow()).box;
    assert.ok(text({ id: 'h', kind: 'headline', text: 'Kurz' }).w < area.w * 0.5, 'a headline box is the text bounds, not the area');
    assert.ok(text({ id: 'l', kind: 'list', text: 'Eins\nZwei' }).h < area.h * 0.5, 'a list box is the text bounds, not the area');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.measure is not a function (1 new check failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks.js`, replace:

```js
        return make(block, { x: a.x + a.w / 2, y: a.y + a.h / 2, w: a.w, h: a.h, surface: a.surface, t0: t0, dur: dur < 0 ? 0 : dur, entry: def.entry, reveals: def.reveals, hidden: def.hidden === true });
    }

    // ---- serialize and revive

    function trackOut(track) {
```

with:

```js
        return make(block, { x: a.x + a.w / 2, y: a.y + a.h / 2, w: a.w, h: a.h, surface: a.surface, t0: t0, dur: dur < 0 ? 0 : dur, entry: def.entry, reveals: def.reveals, hidden: def.hidden === true });
    }

    // measure(ctx, st, rc, stage, win) = {box, labelPx} of an evaluated block state: the box its drawing occupies in design pixels at
    // the block's position and scale (the text bounds of a headline or a list, the nodes of a diagram, the area of a chart) and the
    // smallest label in canvas pixels, 0 without a label. It draws nothing. It is null for an unknown kind.
    function measure(ctx, st, rc, stage, win) {
        var pl = SM.blocks.plan(ctx, st, rc, stage), k = st.scale;
        if (!pl) return null;
        return { box: { x: st.x + pl.box.x * k, y: st.y + pl.box.y * k, w: pl.box.w * k, h: pl.box.h * k }, labelPx: pl.labelPx * stage.scale * k };
    }

    // ---- serialize and revive

    function trackOut(track) {
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes, measure: measure
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 291 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: block measurement"
```

### Task C8: render and raster draw blocks

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/raster.js`
- Modify: `ui/js/storymaker/render.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    assert.ok(text({ id: 'h', kind: 'headline', text: 'Kurz' }).w < area.w * 0.5, 'a headline box is the text bounds, not the area');
    assert.ok(text({ id: 'l', kind: 'list', text: 'Eins\nZwei' }).h < area.h * 0.5, 'a list box is the text bounds, not the area');
});
```

with:

```js
    assert.ok(text({ id: 'h', kind: 'headline', text: 'Kurz' }).w < area.w * 0.5, 'a headline box is the text bounds, not the area');
    assert.ok(text({ id: 'l', kind: 'list', text: 'Eins\nZwei' }).h < area.h * 0.5, 'a list box is the text bounds, not the area');
});

// ---- C8: render and raster draw blocks

check('blocks: drawScene paints a block after the shapes and before the texts, and a block can carry an outline highlight', () => {
    const stage = SM.stage.create({ aspect: '16:9', width: 1920, height: 1080 }), rc = rcOf(1920, 1080), area = B.area(stage, null, 'day', 'center');
    const sceneOf = extra => {
        const block = B.create({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw', area, t0: 0, dur: 1 });
        if (extra) extra(block);
        return {
            id: 's1', env: null, variant: 'day', camera: SM.camera.create(1, 960, 540), stages: {},
            entities: [SM.scene.createText({ id: 't', text: 'Hallo', x: 960, y: 900 }), block, SM.scene.createShape({ id: 'r', kind: 'rect', x: 300, y: 300, w: 100, h: 100, color: '$accent' })]
        };
    };
    const run = (time, extra) => { const cv = new FakeCanvas(1920, 1080); SM.render.drawScene(cv.ctx, sceneOf(extra), time, rc); return cv.ctx.log; };
    const log = run(1), at = prefix => log.findIndex(l => l.startsWith(prefix));
    const shape = at('fillRect(-50,-50,100,100)'), block = at('stroke(path('), text = at('fillText(Hallo');
    assert.ok(shape >= 0 && block > shape && text > block, 'shape ' + shape + ', block ' + block + ', text ' + text);
    assert.equal(digest(run(1)), digest(run(1)));
    assert.notEqual(digest(run(0.2)), digest(run(1)), 'the reveal changes the drawing');
    const marked = run(1, e => e.highlights.add(0, 5, { style: 'outline' }));
    assert.ok(marked.some(l => l.startsWith('strokeRect(')), 'an outline highlight frames the block');
    assert.equal(run(1, e => e.alpha.init = 0).filter(l => l.startsWith('stroke(path(')).length, 0, 'a block with alpha 0 is not drawn');
});

check('blocks: cacheKey only follows the scene time while the block animates', () => {
    const area = { x: 0, y: 0, w: 800, h: 400, surface: 'panel' };
    const key = (def, time) => B.cacheKey(SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), time));
    const icon = { id: 'i', kind: 'icon', icon: 'icon.cpu' };
    assert.notEqual(key(icon, 0.3), key(icon, 0.6), 'during the reveal');
    assert.equal(key(icon, 3), key(icon, 4), 'after the reveal');
    const flow = { ...diagram('flow', 3), diagram: { layout: 'flow', flow: true, nodes: flowNodes(3) } };
    assert.notEqual(key(flow, 3), key(flow, 4), 'flow dots keep moving');
    assert.equal(key(diagram('flow', 3), 3), key(diagram('flow', 3), 4));
    const e = B.create({ ...diagram('flow', 3), area, t0: 0, dur: 1 });
    e.marks.add(2, 6, { node: 'n1', style: 'pulse' });
    assert.notEqual(B.cacheKey(SM.scene.evalEntity(e, 3)), B.cacheKey(SM.scene.evalEntity(e, 4)), 'a running mark');
    assert.notEqual(key(icon, 3), key({ ...icon, icon: 'icon.server' }, 3));
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: no block painting in drawScene (the log had no stroke(path) call); B.cacheKey is not a function

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks.js`, replace:

```js
        return { box: { x: st.x + pl.box.x * k, y: st.y + pl.box.y * k, w: pl.box.w * k, h: pl.box.h * k }, labelPx: pl.labelPx * stage.scale * k };
    }

    // ---- serialize and revive

    function trackOut(track) {
```

with:

```js
        return { box: { x: st.x + pl.box.x * k, y: st.y + pl.box.y * k, w: pl.box.w * k, h: pl.box.h * k }, labelPx: pl.labelPx * stage.scale * k };
    }

    // cacheKey(st): a string that changes when the drawing of a block state changes. The scene time is part of it only while the block
    // animates: its reveal runs, a node mark is active or the diagram flows. The raster overlay redraws only when the key changes.
    function cacheKey(st) {
        var end = st.t0 + st.dur;
        for (var k = 0; st.reveals && k < st.reveals.length; k++) if (st.reveals[k] + ITEM_SPAN > end) end = st.reveals[k] + ITEM_SPAN;
        var moving = st.time < end || !!st.mark || !!(st.block.diagram && st.block.diagram.flow === true);
        return JSON.stringify([st.id, st.x, st.y, st.alpha, st.scale, st.rot, st.block, st.w, st.h, st.surface, st.t0, st.dur, st.entry, st.reveals, st.mark, st.highlight || null, moving ? st.time : 0]);
    }

    // ---- serialize and revive

    function trackOut(track) {
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes, measure: measure
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes, measure: measure, cacheKey: cacheKey
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/raster.js`, replace:

```js
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && s.alpha > 0; }).forEach(prop);
        this.nodes.forEach(function (node, key) { node.visible = self.used.has(key); });
        var overlay = this.overlayContext;
        var visibleOverlay = states.filter(function (s) { return s.alpha > 0 && (s.type === 'shape' || s.type === 'text' && s.text || s.type === 'bubble' && s.lines && s.lines.length || s.highlight && s.highlight.style !== 'pulse'); });
        var owners = new Set(visibleOverlay.filter(function (s) { return s.type === 'bubble'; }).map(function (s) { return s.owner; }));
        var inputs = visibleOverlay.concat(states.filter(function (s) { return owners.has(s.id); })).map(function (s) { var plain = Object.assign({}, s); delete plain.entity; return plain; });
        var overlayKey = visibleOverlay.length ? JSON.stringify([sc.id, cam, inputs]) : '';
        if (this.overlayKey !== overlayKey) {
            // Reset backing pixels only when contents change; transparent CanvasSource uploads must invalidate old pixels.
            this.overlay.width = rc.W;
            states.filter(function (s) { return s.type === 'shape' && s.alpha > 0; }).forEach(function (s) { SM.render.drawShapeEntity(overlay,s,rc,camera); });
            states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { SM.render.drawText(overlay,s,rc,stage,base); });
            states.filter(function (s) { return s.type === 'bubble'; }).forEach(function (s) { SM.render.drawBubble(overlay,s,states,rc,stage,win); });
            states.forEach(function (s) { SM.render.drawHighlight(overlay,s,rc,stage,base,camera); });
```

with:

```js
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && s.alpha > 0; }).forEach(prop);
        this.nodes.forEach(function (node, key) { node.visible = self.used.has(key); });
        var overlay = this.overlayContext;
        var visibleOverlay = states.filter(function (s) { return s.alpha > 0 && (s.type === 'shape' || s.type === 'block' || s.type === 'text' && s.text || s.type === 'bubble' && s.lines && s.lines.length || s.highlight && s.highlight.style !== 'pulse'); });
        var owners = new Set(visibleOverlay.filter(function (s) { return s.type === 'bubble'; }).map(function (s) { return s.owner; }));
        var inputs = visibleOverlay.concat(states.filter(function (s) { return owners.has(s.id); })).map(function (s) { if (s.type === 'block') return SM.blocks.cacheKey(s); var plain = Object.assign({}, s); delete plain.entity; return plain; });
        var overlayKey = visibleOverlay.length ? JSON.stringify([sc.id, cam, inputs]) : '';
        if (this.overlayKey !== overlayKey) {
            // Reset backing pixels only when contents change; transparent CanvasSource uploads must invalidate old pixels.
            this.overlay.width = rc.W;
            states.filter(function (s) { return s.type === 'shape' && s.alpha > 0; }).forEach(function (s) { SM.render.drawShapeEntity(overlay,s,rc,camera); });
            states.filter(function (s) { return s.type === 'block' && s.alpha > 0; }).forEach(function (s) { SM.blocks.draw(overlay,s,rc,stage,win); });
            states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { SM.render.drawText(overlay,s,rc,stage,base); });
            states.filter(function (s) { return s.type === 'bubble'; }).forEach(function (s) { SM.render.drawBubble(overlay,s,states,rc,stage,win); });
            states.forEach(function (s) { SM.render.drawHighlight(overlay,s,rc,stage,base,camera); });
```

In `ui/js/storymaker/render.js`, replace:

```js

    // entityBox(st, rc, stage): the box of an actor or a prop in design pixels, or null when its art is missing.
    function entityBox(st, rc, stage) {
        var depth = stage.depthScale(st.y, st.depth), h, w;
        if (st.type === 'prop') {
            var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
```

with:

```js

    // entityBox(st, rc, stage): the box of an actor or a prop in design pixels, or null when its art is missing.
    function entityBox(st, rc, stage) {
        if (st.type === 'block') return null;   // the box of a block comes from SM.blocks.measure, which needs a context
        var depth = stage.depthScale(st.y, st.depth), h, w;
        if (st.type === 'prop') {
            var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
```

In `ui/js/storymaker/render.js`, replace:

```js
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            box = textBox(ctx, st, rc, stage, bw);
            unit = stage.scale;
        } else {
            box = entityBox(st, rc, stage);
            setM(ctx, Mv);
```

with:

```js
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            box = textBox(ctx, st, rc, stage, bw);
            unit = stage.scale;
        } else if (st.type === 'block') {
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            var measured = SM.blocks.measure(ctx, st, rc, stage, null);
            box = measured && measured.box;
            setM(ctx, Mv);
        } else {
            box = entityBox(st, rc, stage);
            setM(ctx, Mv);
```

In `ui/js/storymaker/render.js`, replace:

```js
            .forEach(function (s) { drawActor(ctx, s, t, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'shape' && live(s); }).forEach(function (s) { drawShapeEntity(ctx, s, rc, Mv); });
        var tokens = envTokens(sc, rc);
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
        states.filter(function (s) { return s.type === 'bubble'; }).forEach(function (s) { drawBubble(ctx, s, states, rc, stage, win); });
```

with:

```js
            .forEach(function (s) { drawActor(ctx, s, t, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'shape' && live(s); }).forEach(function (s) { drawShapeEntity(ctx, s, rc, Mv); });
        states.filter(function (s) { return s.type === 'block' && live(s); }).forEach(function (s) { SM.blocks.draw(ctx, s, rc, stage, win); });
        var tokens = envTokens(sc, rc);
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
        states.filter(function (s) { return s.type === 'bubble'; }).forEach(function (s) { drawBubble(ctx, s, states, rc, stage, win); });
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 293 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js ui/js/storymaker/raster.js ui/js/storymaker/render.js
git commit -m "storymaker: render and raster draw blocks"
```

### Task C9: env.board in the catalog, direction and Go

**Files:**
- Modify: `assets/storymaker/lib/raster-catalog.mjs`
- Modify: `assets/storymaker/lib/raster-names.mjs`
- Modify: `assets/storymaker/lib/raster-validate.mjs`
- Modify: `assets/storymaker/test/catalog.test.mjs`
- Modify: `internal/storymaker/catalog_test.go`
- Modify: `internal/storymaker/data/env_direction.json`
- Modify: `internal/storymaker/studio_direction.go`
- Modify: `internal/storymaker/studio_direction_data.go`
- Modify: `internal/storymaker/studio_direction_test.go`
- Modify: `internal/storymaker/testdata/catalog-mini/catalog.json`
- Create: `internal/storymaker/testdata/catalog-mini/environments/board.json`
- Modify: `scripts/storymaker-test-support.mjs`
- Modify: `scripts/storymaker-tests/00-fixtures.mjs`
- Modify: `scripts/storymaker-tests/23-raster-contract.mjs`
- Modify: `ui/img/storymaker/catalog.json` (generated)
- Create: `ui/img/storymaker/environments/board.json` (generated)
- Create: `ui/img/storymaker/thumbs/env.board.png` (generated)
- Modify: `ui/lang/storymaker_catalog/cs.json` (generated)
- Modify: `ui/lang/storymaker_catalog/da.json` (generated)
- Modify: `ui/lang/storymaker_catalog/de.json` (generated)
- Modify: `ui/lang/storymaker_catalog/el.json` (generated)
- Modify: `ui/lang/storymaker_catalog/en.json` (generated)
- Modify: `ui/lang/storymaker_catalog/es.json` (generated)
- Modify: `ui/lang/storymaker_catalog/fr.json` (generated)
- Modify: `ui/lang/storymaker_catalog/hi.json` (generated)
- Modify: `ui/lang/storymaker_catalog/it.json` (generated)
- Modify: `ui/lang/storymaker_catalog/ja.json` (generated)
- Modify: `ui/lang/storymaker_catalog/nl.json` (generated)
- Modify: `ui/lang/storymaker_catalog/no.json` (generated)
- Modify: `ui/lang/storymaker_catalog/pl.json` (generated)
- Modify: `ui/lang/storymaker_catalog/pt.json` (generated)
- Modify: `ui/lang/storymaker_catalog/sv.json` (generated)
- Modify: `ui/lang/storymaker_catalog/zh.json` (generated)
- Modify: `ui/storymaker_raster_catalog_test.go`

- [ ] **Step 1: Write the failing test**

In `assets/storymaker/test/catalog.test.mjs`, replace:

```js
    assert.ok(at('far-right') - at('right') >= 230, `${id}: the right and far-right actors overlap`);
  }
});
```

with:

```js
    assert.ok(at('far-right') - at('right') >= 230, `${id}: the right and far-right actors overlap`);
  }
});

test('env.board is a drawn environment with a full, a top and a board spot, both token maps and a flat thumbnail', () => {
  const built = buildRasterOutputs();
  const entry = built.manifest.entries.find((e) => e.id === 'env.board');
  assert.equal(entry.kind, 'environment');
  assert.equal(entry.drawn, 'board');
  assert.equal(entry.file, 'environments/board.json');
  for (const spot of ['full', 'top', 'board', 'left', 'center', 'right']) assert.ok(entry.spots[spot], spot);
  assert.deepEqual(entry.variants, ['day', 'night']);
  assert.ok(entry.tags.includes('board') && entry.tags.includes('diagram'));
  const data = JSON.parse(built.files.get('environments/board.json'));
  assert.equal(data.drawn, 'board');
  assert.deepEqual(data.variants, { day: [], night: [] });
  assert.ok(data.tokens.day.text && data.tokens.night.text && data.tokens.night.bg);
  const thumb = decodePNG(built.files.get('thumbs/env.board.png'));
  assert.deepEqual([thumb.width, thumb.height], [256, 144]);
  assert.equal(built.manifest.entries.filter((e) => e.kind === 'environment').length, ENVIRONMENT_IDS.length + 1);
  assert.deepEqual(validateRasterOutputs(built), []);
});
```

In `internal/storymaker/catalog_test.go`, replace:

```go
		"transition":  {"cut", "fade", "slide-left", "wipe"},
		"palette":     {"sunny"},
		"font":        {"default"},
		"environment": {"env.plain-stage", "env.office", "env.abstract-tech"},
		"trans":       {"cut", "fade", "slide-left", "wipe"}, // alias for the id prefix
	}
	for kind, want := range cases {
```

with:

```go
		"transition":  {"cut", "fade", "slide-left", "wipe"},
		"palette":     {"sunny"},
		"font":        {"default"},
		"environment": {"env.plain-stage", "env.office", "env.abstract-tech", "env.board"},
		"trans":       {"cut", "fade", "slide-left", "wipe"}, // alias for the id prefix
	}
	for kind, want := range cases {
```

In `internal/storymaker/catalog_test.go`, replace:

```go
	}
	err := c.VerifyFiles(fsys, "img/storymaker")
	// Rig 1, font 1, 23 thumbs and 5 entry files give 30 (S5). The sfx entries have no thumb, and their
	// other takes add whoosh 1, pop 1 and typing 2, so 34.
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "34 missing files") {
		t.Fatalf("missing files not reported: %v", err)
	}
	for _, e := range c.Entries() {
```

with:

```go
	}
	err := c.VerifyFiles(fsys, "img/storymaker")
	// Rig 1, font 1, 23 thumbs and 5 entry files give 30 (S5). The sfx entries have no thumb, and their
	// other takes add whoosh 1, pop 1 and typing 2, so 34. The drawn board adds its file and its thumb, so 36.
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "36 missing files") {
		t.Fatalf("missing files not reported: %v", err)
	}
	for _, e := range c.Entries() {
```

In `internal/storymaker/studio_direction_test.go`, replace:

```go
		if got := tagMatches(words, tag); got != want {
			t.Errorf("tagMatches(%q) = %v, want %v", tag, got, want)
		}
	}
}
```

with:

```go
		if got := tagMatches(words, tag); got != want {
			t.Errorf("tagMatches(%q) = %v, want %v", tag, got, want)
		}
	}
}

// The board is a drawn backdrop for blocks, not a place for a story: the direction never ranks it, while the catalog lists it.
func TestRankEnvironmentsLeavesTheBoardOut(t *testing.T) {
	cat := loadReal(t)
	if !slices.Contains(cat.Names("environment"), BoardEnvironmentID) {
		t.Fatalf("the catalog lists no %s", BoardEnvironmentID)
	}
	s := loadAgentFixture(t, "agent-firewall-run")
	ranked := rankEnvironments(&s, cat)
	if len(ranked) != len(cat.Names("environment"))-1 || slices.Contains(ranked, BoardEnvironmentID) {
		t.Errorf("ranked = %v", ranked)
	}
}
```

In `internal/storymaker/testdata/catalog-mini/catalog.json`, replace:

```json
  {"id":"env.plain-stage","kind":"environment","name":"env.plain-stage","name_key":"storymaker.catalog.env_plain-stage","tags":["plain","stage","neutral"],"thumb":"thumbs/env.plain-stage.svg","usage_example":{"environment":"env.plain-stage"},"file":"environments/plain-stage.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}},"floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"far-right":{"x":62,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88}}},"1:1":{"focus_x":50}}},
  {"id":"env.office","kind":"environment","name":"env.office","name_key":"storymaker.catalog.env_office","tags":["office","work","desk"],"thumb":"thumbs/env.office.svg","usage_example":{"environment":"env.office"},"file":"environments/office.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"desk":{"x":36,"y":84,"face":"right"},"door":{"x":90,"y":82,"face":"left"},"board":{"x":70,"y":60},"window":{"x":18,"y":82}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":40},"1:1":{"focus_x":45}}},
  {"id":"env.abstract-tech","kind":"environment","name":"env.abstract-tech","name_key":"storymaker.catalog.env_abstract-tech","tags":["tech","abstract","network"],"thumb":"thumbs/env.abstract-tech.svg","usage_example":{"environment":"env.abstract-tech"},"file":"environments/abstract-tech.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":70,"y":58}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50},"1:1":{"focus_x":50}}},
  {"id":"pose.stand","kind":"pose","name":"stand","name_key":"storymaker.catalog.pose_stand","tags":["stand"],"thumb":"thumbs/pose.stand.svg","usage_example":{"pose":"stand"},"bones":{}},
  {"id":"pose.typing","kind":"pose","name":"typing","name_key":"storymaker.catalog.pose_typing","tags":["typing","computer","sit"],"thumb":"thumbs/pose.typing.svg","usage_example":{"pose":"typing"},"bones":{"upper_leg_l":-80,"lower_leg_l":80,"upper_leg_r":-80,"lower_leg_r":80,"upper_arm_l":-30,"lower_arm_l":-70,"upper_arm_r":-30,"lower_arm_r":-70},"hands":{"l":"open","r":"open"},"seated":true},
  {"id":"gesture.point","kind":"gesture","name":"point","name_key":"storymaker.catalog.gesture_point","tags":["point","show"],"thumb":"thumbs/gesture.point.svg","usage_example":{"do":"gesture","who":"anna","gesture":"point"},"duration":1.2,"keys":[{"t":0,"bones":{}},{"t":0.3,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":0.8,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":1,"bones":{}}]},
```

with:

```json
  {"id":"env.plain-stage","kind":"environment","name":"env.plain-stage","name_key":"storymaker.catalog.env_plain-stage","tags":["plain","stage","neutral"],"thumb":"thumbs/env.plain-stage.svg","usage_example":{"environment":"env.plain-stage"},"file":"environments/plain-stage.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}},"floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"far-right":{"x":62,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88}}},"1:1":{"focus_x":50}}},
  {"id":"env.office","kind":"environment","name":"env.office","name_key":"storymaker.catalog.env_office","tags":["office","work","desk"],"thumb":"thumbs/env.office.svg","usage_example":{"environment":"env.office"},"file":"environments/office.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"desk":{"x":36,"y":84,"face":"right"},"door":{"x":90,"y":82,"face":"left"},"board":{"x":70,"y":60},"window":{"x":18,"y":82}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":40},"1:1":{"focus_x":45}}},
  {"id":"env.abstract-tech","kind":"environment","name":"env.abstract-tech","name_key":"storymaker.catalog.env_abstract-tech","tags":["tech","abstract","network"],"thumb":"thumbs/env.abstract-tech.svg","usage_example":{"environment":"env.abstract-tech"},"file":"environments/abstract-tech.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":70,"y":58}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50},"1:1":{"focus_x":50}}},
  {"id":"env.board","kind":"environment","name":"env.board","name_key":"storymaker.catalog.env_board","tags":["board","paper","slate","diagram","chart","whiteboard"],"thumb":"thumbs/env.board.svg","usage_example":{"environment":"env.board"},"file":"environments/board.json","resources":[],"drawn":"board","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"full":{"x":50,"y":50},"top":{"x":50,"y":25},"board":{"x":50,"y":50}},"floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88},"far-right":{"x":62,"y":88}}},"1:1":{"focus_x":50,"spots":{"far-left":{"x":25,"y":88},"far-right":{"x":75,"y":88}}}}},
  {"id":"pose.stand","kind":"pose","name":"stand","name_key":"storymaker.catalog.pose_stand","tags":["stand"],"thumb":"thumbs/pose.stand.svg","usage_example":{"pose":"stand"},"bones":{}},
  {"id":"pose.typing","kind":"pose","name":"typing","name_key":"storymaker.catalog.pose_typing","tags":["typing","computer","sit"],"thumb":"thumbs/pose.typing.svg","usage_example":{"pose":"typing"},"bones":{"upper_leg_l":-80,"lower_leg_l":80,"upper_leg_r":-80,"lower_leg_r":80,"upper_arm_l":-30,"lower_arm_l":-70,"upper_arm_r":-30,"lower_arm_r":-70},"hands":{"l":"open","r":"open"},"seated":true},
  {"id":"gesture.point","kind":"gesture","name":"point","name_key":"storymaker.catalog.gesture_point","tags":["point","show"],"thumb":"thumbs/gesture.point.svg","usage_example":{"do":"gesture","who":"anna","gesture":"point"},"duration":1.2,"keys":[{"t":0,"bones":{}},{"t":0.3,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":0.8,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":1,"bones":{}}]},
```

Create `internal/storymaker/testdata/catalog-mini/environments/board.json`:

```json
{"viewbox":[0,0,1920,1080],"drawn":"board","variants":{"day":[],"night":[]},"tokens":{"day":{"text":"#1f2a44"},"night":{"text":"#f4f5eb","bg":"#1f2a44","bg2":"#2c3a5a"}}}
```

In `scripts/storymaker-tests/00-fixtures.mjs`, replace:

```js
    assert.deepEqual(firewall.scenes.map(s => s.id), ['s1', 's2', 's3']);
    assert.equal(firewall.scenes[2].kind, 'code');
    assert.match(read('internal/storymaker/testdata/scenes/s3.js'), /^export default function scene\(api\) \{\r?\n  api\.duration\(5\);/);
    assert.equal(catalog.schema, 'storymaker-catalog/1'); assert.equal(catalog.version, 'mini00000000'); assert.equal(catalog.entries.length, 26); // 23 with the speech styles, plus sfx.whoosh, sfx.pop, sfx.typing
    assert.deepEqual(catalog.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
    for (const e of catalog.entries) if (e.file && !e.file.endsWith('.wav')) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
    assert.equal(readJSON(MINI + catalog.rig.file).id, catalog.rig.id);
```

with:

```js
    assert.deepEqual(firewall.scenes.map(s => s.id), ['s1', 's2', 's3']);
    assert.equal(firewall.scenes[2].kind, 'code');
    assert.match(read('internal/storymaker/testdata/scenes/s3.js'), /^export default function scene\(api\) \{\r?\n  api\.duration\(5\);/);
    assert.equal(catalog.schema, 'storymaker-catalog/1'); assert.equal(catalog.version, 'mini00000000'); assert.equal(catalog.entries.length, 27); // 23 with the speech styles, plus sfx.whoosh, sfx.pop, sfx.typing and env.board
    assert.deepEqual(catalog.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
    for (const e of catalog.entries) if (e.file && !e.file.endsWith('.wav')) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
    assert.equal(readJSON(MINI + catalog.rig.file).id, catalog.rig.id);
```

In `scripts/storymaker-tests/00-fixtures.mjs`, replace:

```js
    }
    for (const [file, env] of Object.entries(files.envs)) {
        assert.deepEqual(env.viewbox, [0, 0, 1920, 1080], file);
        assert.ok(env.layers.length >= 3 && env.layers[0].parallax === 0 && env.layers.at(-1).parallax === 1, file + ': sky at parallax 0, floor at 1');
        for (const l of env.layers) { assert.equal(typeof l.name, 'string'); checkShapes(l.shapes, file + '/' + l.name); }
        assert.deepEqual(Object.keys(env.tokens.day).sort(), Object.keys(env.tokens.night).sort(), file + ': day and night tokens match');
```

with:

```js
    }
    for (const [file, env] of Object.entries(files.envs)) {
        assert.deepEqual(env.viewbox, [0, 0, 1920, 1080], file);
        if (env.drawn === 'board') { assert.deepEqual(env.variants, { day: [], night: [] }, file + ': a drawn environment has no layers'); assert.ok(env.tokens.day.text && env.tokens.night.text, file); continue; }
        assert.ok(env.layers.length >= 3 && env.layers[0].parallax === 0 && env.layers.at(-1).parallax === 1, file + ': sky at parallax 0, floor at 1');
        for (const l of env.layers) { assert.equal(typeof l.name, 'string'); checkShapes(l.shapes, file + '/' + l.name); }
        assert.deepEqual(Object.keys(env.tokens.day).sort(), Object.keys(env.tokens.night).sort(), file + ': day and night tokens match');
```

In `scripts/storymaker-tests/23-raster-contract.mjs`, replace:

```js

t.check('production catalog: all legacy IDs survive as aliases of three 3D pets, twelve places and eighty props',()=>{
    const catalog=realCatalog();assert.equal(catalog.schema,'storymaker-catalog/2');
    for(const [kind,count]of Object.entries({character:3,environment:12,prop:80}))assert.equal(catalog.entries.filter(e=>e.kind===kind).length,count);
    const ids=new Set(catalog.entries.flatMap(e=>[e.id,...(e.aliases||[])]));for(const e of legacyCatalog().entries)assert.ok(ids.has(e.id),'Missing legacy ID '+e.id);
    for(const language of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
        const names=readJSON('ui/lang/storymaker_catalog/'+language+'.json');for(const e of catalog.entries)assert.ok(names[e.name_key],language+'/'+e.name_key);
```

with:

```js

t.check('production catalog: all legacy IDs survive as aliases of three 3D pets, twelve places and eighty props',()=>{
    const catalog=realCatalog();assert.equal(catalog.schema,'storymaker-catalog/2');
    for(const [kind,count]of Object.entries({character:3,environment:13,prop:80}))assert.equal(catalog.entries.filter(e=>e.kind===kind).length,count);
    const ids=new Set(catalog.entries.flatMap(e=>[e.id,...(e.aliases||[])]));for(const e of legacyCatalog().entries)assert.ok(ids.has(e.id),'Missing legacy ID '+e.id);
    for(const language of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
        const names=readJSON('ui/lang/storymaker_catalog/'+language+'.json');for(const e of catalog.entries)assert.ok(names[e.name_key],language+'/'+e.name_key);
```

In `ui/storymaker_raster_catalog_test.go`, replace:

```go
				Tokens map[string]map[string]string
			}
			smReadJSON(t, smStr(e, "file"), &d)
			for _, variant := range []string{"day", "night"} {
				layers := d.Variants[variant]
				if len(layers) < 2 || d.Tokens[variant]["text"] == "" {
					t.Errorf("missing depth or text tokens: %s/%s", smStr(e, "id"), variant)
				}
```

with:

```go
				Tokens map[string]map[string]string
			}
			smReadJSON(t, smStr(e, "file"), &d)
			drawn := smStr(e, "drawn")
			for _, variant := range []string{"day", "night"} {
				layers := d.Variants[variant]
				if drawn == "board" {
					// a drawn environment has no bitmap layers: the player paints it
					if len(layers) != 0 || d.Tokens[variant]["text"] == "" || smStr(e, "id") != "env.board" {
						t.Errorf("invalid drawn environment: %s/%s", smStr(e, "id"), variant)
					}
					continue
				}
				if len(layers) < 2 || d.Tokens[variant]["text"] == "" {
					t.Errorf("missing depth or text tokens: %s/%s", smStr(e, "id"), variant)
				}
```

In `ui/storymaker_raster_catalog_test.go`, replace:

```go
			}
		}
	}
	if props != 80 || envs != 12 {
		t.Errorf("got %d props and %d environments", props, envs)
	}
}
```

with:

```go
			}
		}
	}
	if props != 80 || envs != 13 {
		t.Errorf("got %d props and %d environments", props, envs)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestRankEnvironmentsLeavesTheBoardOut|TestEnvironmentGuideCoversTheCatalog|TestDirection)' -count=1; node --test assets/storymaker/test/catalog.test.mjs; node scripts/build-storymaker-catalog.mjs --check; go test ./ui -run '(?i)storymaker' -count=1`
Expected: undefined: BoardEnvironmentID (build failed); the env.board catalog test failed

- [ ] **Step 3: Implement**

In `assets/storymaker/lib/raster-catalog.mjs`, replace:

```js
    nature:['sun','moon','tree','leaf','flower','water-drop','wind-turbine','solar-panel','battery','earth'],
    symbols:['check','cross','warning','idea','question','arrow-right','gear','chart-bar','chart-line','puzzle']
};
export const CHARACTER_IDS=MODEL_CHARACTERS.map(c=>c.id);
export const ENVIRONMENT_IDS=['plain-stage','office','home-office','abstract-tech','server-room','classroom','laboratory','workshop','medical-office','living-room','city-street','park'];
export function buildRasterOutputs({prototype=false}={}){
```

with:

```js
    nature:['sun','moon','tree','leaf','flower','water-drop','wind-turbine','solar-panel','battery','earth'],
    symbols:['check','cross','warning','idea','question','arrow-right','gear','chart-bar','chart-line','puzzle']
};
// boardThumbnail(bg, fg): the 256x144 thumbnail of env.board: the palette background with the grain of the player's drawBoard.
// The grain is 900 dots from the seeded generator of seed.js (FNV-1a over the seed, then mulberry32) at 0.8 % alpha, so no pixel
// moves by more than 2 of 255.
function boardThumbnail(bg,fg){
    const W=256,H=144,pixels=Buffer.alloc(W*H*4),rgb=hex=>[1,3,5].map(i=>parseInt(hex.slice(i,i+2),16));
    const back=rgb(bg),front=rgb(fg);
    for(let i=0;i<W*H;i++)pixels.set([...back,255],i*4);
    let a=2166136261;for(const ch of 'board-grain'){a^=ch.charCodeAt(0);a=Math.imul(a,16777619);}a>>>=0;
    const random=()=>{a=(a+0x6D2B79F5)|0;let t=Math.imul(a^(a>>>15),1|a);t=(t+Math.imul(t^(t>>>7),61|t))^t;return((t^(t>>>14))>>>0)/4294967296;};
    for(let i=0;i<900;i++){
        const x=Math.floor(random()*W),y=Math.floor(random()*H);random();
        for(let c=0;c<3;c++)pixels[(y*W+x)*4+c]=Math.round(back[c]+(front[c]-back[c])*0.008);
    }
    return{width:W,height:H,pixels};
}
export const CHARACTER_IDS=MODEL_CHARACTERS.map(c=>c.id);
export const ENVIRONMENT_IDS=['plain-stage','office','home-office','abstract-tech','server-room','classroom','laboratory','workshop','medical-office','living-room','city-street','park'];
export function buildRasterOutputs({prototype=false}={}){
```

In `assets/storymaker/lib/raster-catalog.mjs`, replace:

```js
        const file=`environments/${id}.json`;put(file,{viewbox:[0,0,1920,1080],variants,tokens});
        entries.push(entry('env.'+id,'environment',{file,resources,thumb:thumb('env.'+id,day,[0,0,day.width,day.height]),spots,floor:old.floor,variants:['day','night'],aspects:old.aspects,usage_example:{environment:'env.'+id}}));
    }
    const reference=images.values().next().value;
    if(!reference)throw new Error('No completed character available');
    for(const old of legacy.manifest.entries){
```

with:

```js
        const file=`environments/${id}.json`;put(file,{viewbox:[0,0,1920,1080],variants,tokens});
        entries.push(entry('env.'+id,'environment',{file,resources,thumb:thumb('env.'+id,day,[0,0,day.width,day.height]),spots,floor:old.floor,variants:['day','night'],aspects:old.aspects,usage_example:{environment:'env.'+id}}));
    }
    // env.board is a drawn environment: it has no bitmap, the player paints a calm board (light paper by day, dark slate by night)
    // behind full-frame blocks. It sits after the bitmap environments and is not part of ENVIRONMENT_IDS.
    {
        const sunny=legacy.manifest.palettes.find(p=>p.name==='sunny')?.tokens||{bg:'#fff7e6',fg:'#1f2a44'};
        put('environments/board.json',{viewbox:[0,0,1920,1080],drawn:'board',variants:{day:[],night:[]},tokens:{day:{text:'#1f2a44'},night:{text:'#f4f5eb',bg:'#1f2a44',bg2:'#2c3a5a'}}});
        put('thumbs/env.board.png',encodePNG(boardThumbnail(sunny.bg,sunny.fg)));
        entries.push(entry('env.board','environment',{
            tags:['board','paper','slate','diagram','chart','whiteboard',...NAMES['env.board']],thumb:'thumbs/env.board.png',usage_example:{environment:'env.board'},
            file:'environments/board.json',resources:[],drawn:'board',
            spots:{'far-left':{x:12,y:88},left:{x:30,y:88},center:{x:50,y:88},right:{x:70,y:88},'far-right':{x:88,y:88},full:{x:50,y:50},top:{x:50,y:25},board:{x:50,y:50}},
            floor:{y_near:96,y_far:70,scale_near:1.05,scale_far:0.8},variants:['day','night'],
            aspects:{'9:16':{focus_x:50,spots:{'far-left':{x:38,y:88},left:{x:42,y:88},right:{x:58,y:88},'far-right':{x:62,y:88}}},'1:1':{focus_x:50,spots:{'far-left':{x:25,y:88},'far-right':{x:75,y:88}}}}
        }));
    }
    const reference=images.values().next().value;
    if(!reference)throw new Error('No completed character available');
    for(const old of legacy.manifest.entries){
```

In `assets/storymaker/lib/raster-names.mjs`, replace:

```js
    "Peça de quebra-cabeças",
    "Pusselbit",
    "拼图块"
  ]
};
```

with:

```js
    "Peça de quebra-cabeças",
    "Pusselbit",
    "拼图块"
  ],
  "env.board": [
    "Tabule",
    "Tavle",
    "Tafel",
    "Πίνακας",
    "Board",
    "Pizarra",
    "Tableau",
    "बोर्ड",
    "Lavagna",
    "ボード",
    "Bord",
    "Tavle",
    "Tablica",
    "Quadro",
    "Tavla",
    "展板"
  ]
};
```

In `assets/storymaker/lib/raster-validate.mjs`, replace:

```js
 const errors=[],check=(ok,message)=>{if(!ok)errors.push(message);};
 check(manifest.schema==='storymaker-catalog/2','Wrong raster schema');
 check(manifest.version===versionOf(files),'Wrong catalog version');
 const expected={character:CHARACTER_IDS,environment:ENVIRONMENT_IDS,prop:Object.values(PROP_PACKS).flat()};
 for(const [kind,ids]of Object.entries(expected))check(entries.filter(e=>e.kind===kind).length===ids.length,`Wrong ${kind} count`);
 const delivery=new Map(manifest.files.map(f=>[f.path,f]));
 check(delivery.size===manifest.files.length,'Duplicate delivery file');
```

with:

```js
 const errors=[],check=(ok,message)=>{if(!ok)errors.push(message);};
 check(manifest.schema==='storymaker-catalog/2','Wrong raster schema');
 check(manifest.version===versionOf(files),'Wrong catalog version');
 const expected={character:CHARACTER_IDS,environment:[...ENVIRONMENT_IDS,'board'],prop:Object.values(PROP_PACKS).flat()};
 for(const [kind,ids]of Object.entries(expected))check(entries.filter(e=>e.kind===kind).length===ids.length,`Wrong ${kind} count`);
 const delivery=new Map(manifest.files.map(f=>[f.path,f]));
 check(delivery.size===manifest.files.length,'Duplicate delivery file');
```

In `assets/storymaker/lib/raster-validate.mjs`, replace:

```js
   for(const alias of e.aliases||[]){check(/^char\.[a-z0-9-]+$/.test(alias)&&!ids.has(alias),`Invalid character alias: ${alias}`);ids.add(alias);}
  }
  if(e.kind==='prop')region(json(e.file).image,e.id);
  if(e.kind==='environment')for(const variant of ['day','night']){
   const layers=json(e.file).variants[variant];check(layers?.length>=2,`Missing depth layers: ${e.id}/${variant}`);
   for(const layer of layers||[])region(layer.image,e.id+'/'+variant);
  }
```

with:

```js
   for(const alias of e.aliases||[]){check(/^char\.[a-z0-9-]+$/.test(alias)&&!ids.has(alias),`Invalid character alias: ${alias}`);ids.add(alias);}
  }
  if(e.kind==='prop')region(json(e.file).image,e.id);
  if(e.kind==='environment'&&e.drawn==='board'){
   const d=json(e.file);check(e.id==='env.board'&&d.drawn==='board','Unknown drawn environment: '+e.id);
   for(const variant of ['day','night'])check(Array.isArray(d.variants?.[variant])&&d.variants[variant].length===0&&d.tokens?.[variant]?.text,'Bad drawn environment variant: '+e.id+'/'+variant);
  }else if(e.kind==='environment')for(const variant of ['day','night']){
   const layers=json(e.file).variants[variant];check(layers?.length>=2,`Missing depth layers: ${e.id}/${variant}`);
   for(const layer of layers||[])region(layer.image,e.id+'/'+variant);
  }
```

In `internal/storymaker/data/env_direction.json`, replace:

```json
{
  "env.plain-stage": {
    "about": "Neutral empty stage. Use it only when nothing else fits.",
    "tags": []
```

with:

```json
{
  "env.board": {
    "about": "A calm drawn board (light paper by day, dark slate by night) for full-frame diagrams, charts, icons and headlines.",
    "tags": ["board", "tafel", "diagramm", "diagram", "chart"]
  },
  "env.plain-stage": {
    "about": "Neutral empty stage. Use it only when nothing else fits.",
    "tags": []
```

In `internal/storymaker/studio_direction.go`, replace:

```go
}

// rankEnvironments orders the catalog environments for this film: each tag that matches a word of the title or of the
// narrations scores 2, a base environment scores 1 more, plain-stage comes last, and ties rotate by a hash of the title so
// that two films about the same thing but with different titles do not both start on the same environment.
func rankEnvironments(story *Story, cat *Catalog) []string {
	ids := cat.Names("environment")
	if len(ids) == 0 {
		return nil
	}
```

with:

```go
}

// rankEnvironments orders the catalog environments for this film: each tag that matches a word of the title or of the
// narrations scores 2, a base environment scores 1 more, plain-stage comes last, the drawn board is left out, and ties rotate by a hash of the title so
// that two films about the same thing but with different titles do not both start on the same environment.
func rankEnvironments(story *Story, cat *Catalog) []string {
	ids := make([]string, 0, 16)
	for _, id := range cat.Names("environment") {
		if id != BoardEnvironmentID {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
```

In `internal/storymaker/studio_direction_data.go`, replace:

```go
// plainStageID is the neutral environment that the direction uses only when nothing else exists.
const plainStageID = "env.plain-stage"

// EnvDirection is the curated guide of one catalog environment. About is one line for the model, Tags are the words
// (English and German, matched as whole words or as prefixes of at least 4 letters) that mean the environment fits a
// topic, and Base marks an environment that fits many topics.
```

with:

```go
// plainStageID is the neutral environment that the direction uses only when nothing else exists.
const plainStageID = "env.plain-stage"

// BoardEnvironmentID is the drawn board for full-frame blocks. The catalog lists it, but the direction never ranks it as a place.
const BoardEnvironmentID = "env.board"

// EnvDirection is the curated guide of one catalog environment. About is one line for the model, Tags are the words
// (English and German, matched as whole words or as prefixes of at least 4 letters) that mean the environment fits a
// topic, and Base marks an environment that fits many topics.
```

In `scripts/storymaker-test-support.mjs`, replace:

```js
    envs: {
        'environments/plain-stage.json': readJSON(MINI + 'environments/plain-stage.json'),
        'environments/office.json': readJSON(MINI + 'environments/office.json'),
        'environments/abstract-tech.json': readJSON(MINI + 'environments/abstract-tech.json')
    }
});
export const story = name => readJSON('internal/storymaker/testdata/stories/' + name + '.json');
```

with:

```js
    envs: {
        'environments/plain-stage.json': readJSON(MINI + 'environments/plain-stage.json'),
        'environments/office.json': readJSON(MINI + 'environments/office.json'),
        'environments/abstract-tech.json': readJSON(MINI + 'environments/abstract-tech.json'),
        'environments/board.json': readJSON(MINI + 'environments/board.json')
    }
});
export const story = name => readJSON('internal/storymaker/testdata/stories/' + name + '.json');
```

- [ ] **Step 4: Rebuild the generated files**

Run: node scripts/build-storymaker-catalog.mjs, then node scripts/build-storymaker-catalog.mjs --check (git checkout LF-only phantom diffs of unchanged thumbs)

- [ ] **Step 5: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestRankEnvironmentsLeavesTheBoardOut|TestEnvironmentGuideCoversTheCatalog|TestDirection)' -count=1; node --test assets/storymaker/test/catalog.test.mjs; node scripts/build-storymaker-catalog.mjs --check; go test ./ui -run '(?i)storymaker' -count=1`
Expected: ok aurago/internal/storymaker; 24 node tests pass (icons, catalog, environments); catalog 70c092519ae1 is up to date; ok aurago/ui

- [ ] **Step 6: Commit**

```bash
git add assets/storymaker/lib/raster-catalog.mjs assets/storymaker/lib/raster-names.mjs assets/storymaker/lib/raster-validate.mjs assets/storymaker/test/catalog.test.mjs internal/storymaker/catalog_test.go internal/storymaker/data/env_direction.json internal/storymaker/studio_direction.go internal/storymaker/studio_direction_data.go internal/storymaker/studio_direction_test.go internal/storymaker/testdata/catalog-mini/catalog.json internal/storymaker/testdata/catalog-mini/environments/board.json scripts/storymaker-test-support.mjs scripts/storymaker-tests/00-fixtures.mjs scripts/storymaker-tests/23-raster-contract.mjs ui/img/storymaker/catalog.json ui/img/storymaker/environments/board.json ui/img/storymaker/thumbs/env.board.png ui/lang/storymaker_catalog/cs.json ui/lang/storymaker_catalog/da.json ui/lang/storymaker_catalog/de.json ui/lang/storymaker_catalog/el.json ui/lang/storymaker_catalog/en.json ui/lang/storymaker_catalog/es.json ui/lang/storymaker_catalog/fr.json ui/lang/storymaker_catalog/hi.json ui/lang/storymaker_catalog/it.json ui/lang/storymaker_catalog/ja.json ui/lang/storymaker_catalog/nl.json ui/lang/storymaker_catalog/no.json ui/lang/storymaker_catalog/pl.json ui/lang/storymaker_catalog/pt.json ui/lang/storymaker_catalog/sv.json ui/lang/storymaker_catalog/zh.json ui/storymaker_raster_catalog_test.go
git commit -m "storymaker: env.board in the catalog, direction and Go"
```

### Task C10: the board backdrop is drawn for storyboard and code scenes

**Files:**
- Modify: `scripts/storymaker-tests/06-codescene.mjs`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/host-core.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/plan.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/raster.js`
- Modify: `ui/js/storymaker/render.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: extra && extra.aspect || '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
```

with:

```js
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: extra && extra.aspect || '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe, environment: plan.scenes[sceneId].environment, variant: plan.scenes[sceneId].variant };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
```

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
    const drawn = cv.ctx.log.filter(l => l.startsWith('set direction=') || l.startsWith('fillText('));
    assert.deepEqual(drawn, ['set direction=rtl', 'fillText(' + arabic + ',100,100)', 'set direction=ltr', 'fillText(Hello!,100,200)']);
});
```

with:

```js
    const drawn = cv.ctx.log.filter(l => l.startsWith('set direction=') || l.startsWith('fillText('));
    assert.deepEqual(drawn, ['set direction=rtl', 'fillText(' + arabic + ',100,100)', 'set direction=ltr', 'fillText(Hello!,100,200)']);
});

check('code scene on env.board draws the board backdrop and asks for board.json; another environment keeps the palette backdrop', () => {
    const source = 'export default function scene(api) { api.duration(2); api.onFrame((t, g) => { g.rect(1, 1, 2, 2); }); }';
    const frame = (environment, variant) => {
        const story = { ...firewall, scenes: [{ id: 's', kind: 'code', environment, variant }] };
        const { C, build } = runScene(source, 'board', { story, id: 's' });
        const built = build(), cv = new FakeCanvas(1920, 1080);
        const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', fg: '#123456', accent: '#f00', text: '#000' }, cat: C.scene.indexCatalog(catalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
        C.codescene.drawFrame(built, 0.5, cv.ctx, C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080));
        return { built, log: cv.ctx.log };
    };
    const night = frame('env.board', 'night');
    assert.ok(plain(night.built.needs).includes('catalog:environments/board.json'));
    assert.equal(night.built.variant, 'night');
    assert.ok(night.log.includes('set fillStyle=#123456'), 'the night board is palette.fg');
    assert.ok(night.log.includes('set globalAlpha=0.008'), 'with its grain');
    assert.ok(night.log.indexOf('fillRect(1,1,2,2)') > night.log.indexOf('set globalAlpha=0.008'), 'the onFrame drawing comes over the board');
    const day = frame('env.board', 'day');
    assert.ok(day.log.includes('set fillStyle=#fff'), 'the day board is palette.bg');
    const office = frame('env.office', 'day');
    assert.ok(!office.log.includes('set globalAlpha=0.008'));
    assert.equal(office.built.env, null);
    assert.ok(!plain(office.built.needs).includes('catalog:environments/board.json'));
});
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain, FakeCanvas, digest, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

with:

```js
// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain, FakeCanvas, digest, readJSON, renderAssets } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    assert.notEqual(B.cacheKey(SM.scene.evalEntity(e, 3)), B.cacheKey(SM.scene.evalEntity(e, 4)), 'a running mark');
    assert.notEqual(key(icon, 3), key({ ...icon, icon: 'icon.server' }, 3));
});
```

with:

```js
    assert.notEqual(B.cacheKey(SM.scene.evalEntity(e, 3)), B.cacheKey(SM.scene.evalEntity(e, 4)), 'a running mark');
    assert.notEqual(key(icon, 3), key({ ...icon, icon: 'icon.server' }, 3));
});

// ---- C10: the board backdrop

check('blocks: drawBoard fills the day board with palette.bg and the night board with palette.fg, with 900 grain dots at alpha 0.008', () => {
    const run = (variant, W = 1920, H = 1080) => { const cv = new FakeCanvas(W, H); B.drawBoard(cv.ctx, { variant, palette: PALETTE, W, H }); return cv.ctx.log; };
    const day = run('day'), night = run('night');
    assert.equal(day[day.indexOf('fillRect(0,0,1920,1080)') - 1], 'set fillStyle=' + PALETTE.bg);
    assert.equal(night[night.indexOf('fillRect(0,0,1920,1080)') - 1], 'set fillStyle=' + PALETTE.fg);
    assert.ok(day.includes('set fillStyle=' + PALETTE.fg) && night.includes('set fillStyle=' + PALETTE.bg), 'the dots take the other colour');
    for (const log of [day, night]) {
        const grain = log.slice(log.indexOf('set globalAlpha=0.008'));
        assert.equal(log.filter(l => l.startsWith('arc(')).length, 900);
        assert.equal(grain.filter(l => l.startsWith('set globalAlpha=')).join(), 'set globalAlpha=0.008,set globalAlpha=1', 'every grain dot is drawn at alpha 0.008');
        assert.equal(grain.filter(l => l === 'fill()').length, 900);
    }
    assert.equal(digest(run('day')), digest(run('day')));
    assert.notEqual(digest(run('day')), digest(run('night')));
    const radius = log => Math.max(...log.filter(l => l.startsWith('arc(')).map(l => parseFloat(l.slice(4, -1).split(',')[2])));
    near(radius(run('day', 960, 540)), radius(day) / 2, 0.01, 'the dot radius follows the width');
});

check('blocks: a storyboard scene on env.board draws the board over the backdrop, one on env.office does not', () => {
    const tl = (env, variant) => SM.compile({ scenes: [{ id: 'a', silent: true, environment: env, variant, beats: [{ do: 'show', text: 'Hallo', at: 'center', entry: 'none' }] }] }, {}, t.catalog).timeline;
    const run = (env, variant) => { const cv = new FakeCanvas(1920, 1080); SM.renderFrame(tl(env, variant), 10, cv.ctx, renderAssets()); return cv.ctx.log; };
    const board = run('env.board', 'day'), full = log => log.flatMap((l, i) => l === 'fillRect(0,0,1920,1080)' ? [i] : []);
    assert.equal(full(board).length, 2, 'the backdrop and the board');
    assert.equal(board[full(board)[1] - 1], 'set fillStyle=' + PALETTE.bg);
    assert.ok(board.includes('set globalAlpha=0.008'));
    assert.ok(board.some(l => l.startsWith('fillText(Hallo,')), 'the text is drawn over the board');
    const night = run('env.board', 'night');
    assert.equal(night[full(night)[1] - 1], 'set fillStyle=' + PALETTE.fg);
    assert.ok(!run('env.office', 'day').includes('set globalAlpha=0.008'));
    const plan = SM.planTiming({ scenes: [{ id: 'a' }, { id: 'b', environment: 'env.board', variant: 'night' }, { id: 'c', kind: 'code' }, { id: 'd', environment: 'env.office' }] }, {}, t.catalog).scenes;
    assert.deepEqual(['a', 'b', 'c', 'd'].map(id => [plan[id].environment, plan[id].variant]), [['env.plain-stage', 'day'], ['env.board', 'night'], ['env.board', 'day'], ['env.office', 'day']], 'a scene keeps the environment of the one before it');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.drawBoard is not a function; no board fill in a storyboard scene or in a code scene built on env.board (3 new checks failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
            SM.text.roundRect(ctx, b.x - lw * 1.4, b.y - lw * 1.4, b.w + lw * 2.8, b.h + lw * 2.8, 0.1 * Math.min(b.w, b.h) + lw);
            ctx.stroke();
        } else if (mark && mark.style === 'arrow') {
            var tip = b.y - lw * 3 - lw * 2 * Math.sin(4 * Math.PI * mark.u);
            ctx.fillStyle = c.col.accent;
            ctx.beginPath();
            ctx.moveTo(cx, tip);
            ctx.lineTo(cx - lw * 3, tip - lw * 5);
            ctx.lineTo(cx + lw * 3, tip - lw * 5);
            ctx.closePath();
            ctx.fill();
        }
```

with:

```js
            SM.text.roundRect(ctx, b.x - lw * 1.4, b.y - lw * 1.4, b.w + lw * 2.8, b.h + lw * 2.8, 0.1 * Math.min(b.w, b.h) + lw);
            ctx.stroke();
        } else if (mark && mark.style === 'arrow') {
            var tip = b.y - lw * 3 - lw * 2 * Math.sin(4 * Math.PI * mark.u);   // the arrow bobs like the one of drawHighlight
            ctx.fillStyle = c.col.accent;
            ctx.beginPath();
            ctx.moveTo(cx, tip);
            ctx.lineTo(cx - lw * 4.5, tip - lw * 8);
            ctx.lineTo(cx + lw * 4.5, tip - lw * 8);
            ctx.closePath();
            ctx.fill();
        }
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        ctx.restore();
    }

    B.draw = draw;
    B.plan = plan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        ctx.restore();
    }

    // drawBoard(ctx, {variant, palette, W, H}): the board of env.board over the whole canvas: palette.bg by day, palette.fg by night,
    // with 900 faint dots of the other colour. The dots come from a seeded generator and sit at alpha 0.008, so no pixel moves by more
    // than 2 of 255 and an empty board still reads as one flat colour to the first-frame probe.
    function drawBoard(ctx, o) {
        var p = o.palette || {}, night = o.variant === 'night', W = o.W, H = o.H, random = SM.seed.random('board-grain');
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        ctx.fillStyle = night ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6');
        ctx.fillRect(0, 0, W, H);
        ctx.fillStyle = night ? (p.bg || '#fff7e6') : (p.fg || '#1f2a44');
        ctx.globalAlpha = 0.008;
        for (var i = 0; i < 900; i++) {
            var x = random() * W, y = random() * H, r = (0.6 + 1.0 * random()) * W / 1920;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, Math.PI * 2);
            ctx.fill();
        }
        ctx.globalAlpha = 1;
    }

    B.draw = draw;
    B.drawBoard = drawBoard;
    B.plan = plan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/codescene.js`, replace:

```js
        var state = { duration: null, entities: [], onFrame: null, needs: {}, issues: [], counter: 0, unsupportedSeen: {} };
        var cues = opts.cues || {};
        var rigFile = opts.catalog && opts.catalog.rig && opts.catalog.rig.file || '';
        var glyphIdx = opts.catalog && opts.catalog.glyph_data ? SM.glyphs.index(opts.catalog.glyph_data) : null;
        if (glyphIdx) SM.glyphs.facesForLanguage(glyphIdx, opts.language).forEach(function (face) { state.needs[SM.glyphs.key(face)] = true; });
```

with:

```js
        var state = { duration: null, entities: [], onFrame: null, needs: {}, issues: [], counter: 0, unsupportedSeen: {} };
        var cues = opts.cues || {};
        var rigFile = opts.catalog && opts.catalog.rig && opts.catalog.rig.file || '';
        // a code scene on env.board is drawn over the board; every other environment keeps the palette backdrop
        var boardEnv = opts.environment === SM.blocks.BOARD_ID ? cat.byId[SM.blocks.BOARD_ID] || null : null;
        if (boardEnv && boardEnv.file) state.needs['catalog:' + boardEnv.file] = true;
        var glyphIdx = opts.catalog && opts.catalog.glyph_data ? SM.glyphs.index(opts.catalog.glyph_data) : null;
        if (glyphIdx) SM.glyphs.facesForLanguage(glyphIdx, opts.language).forEach(function (face) { state.needs[SM.glyphs.key(face)] = true; });
```

In `ui/js/storymaker/codescene.js`, replace:

```js
        var motionEnd = SM.camera.end(cam);
        state.entities.forEach(function (e) { var end = S.entityEnd(e); if (end > motionEnd) motionEnd = end; });
        var duration = state.duration !== null ? state.duration : Math.min(MAX_DURATION, Math.max(1, motionEnd + 0.5));
        return { duration: duration, explicit: state.duration !== null, entities: state.entities, camera: cam, onFrame: state.onFrame, needs: SM.textures.expand(opts.catalog, Object.keys(state.needs).sort()), issues: state.issues, stage: stage };
    }

    // ---- the restricted 2D facade passed to api.onFrame(fn)
```

with:

```js
        var motionEnd = SM.camera.end(cam);
        state.entities.forEach(function (e) { var end = S.entityEnd(e); if (end > motionEnd) motionEnd = end; });
        var duration = state.duration !== null ? state.duration : Math.min(MAX_DURATION, Math.max(1, motionEnd + 0.5));
        return { duration: duration, explicit: state.duration !== null, entities: state.entities, camera: cam, onFrame: state.onFrame, needs: SM.textures.expand(opts.catalog, Object.keys(state.needs).sort()), issues: state.issues, stage: stage, env: boardEnv, variant: opts.variant === 'night' ? 'night' : 'day' };
    }

    // ---- the restricted 2D facade passed to api.onFrame(fn)
```

In `ui/js/storymaker/codescene.js`, replace:

```js

    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: null, variant: 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
```

with:

```js

    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: built.env || null, variant: built.variant || 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
```

In `ui/js/storymaker/host-core.js`, replace:

```js
                sceneId: id, index: p.plan.scenes[id].index, catalog: p.msg.catalog, aspect: p.plan.aspect, width: p.plan.width, height: p.plan.height,
                fps: p.plan.fps, cues: p.plan.scenes[id].cues, palette: p.plan.palette, families: p.plan.fontFamilies, styleFont: p.plan.styleFont,
                safe: p.msg.catalog && p.msg.catalog.stage && p.msg.catalog.stage.safe || null,
                language: p.plan.language, fontChain: p.plan.fontChain
            };
            var started = env.now();
            await channel.hello(env.limits.sceneBuildDeadlineMs);
```

with:

```js
                sceneId: id, index: p.plan.scenes[id].index, catalog: p.msg.catalog, aspect: p.plan.aspect, width: p.plan.width, height: p.plan.height,
                fps: p.plan.fps, cues: p.plan.scenes[id].cues, palette: p.plan.palette, families: p.plan.fontFamilies, styleFont: p.plan.styleFont,
                safe: p.msg.catalog && p.msg.catalog.stage && p.msg.catalog.stage.safe || null,
                language: p.plan.language, fontChain: p.plan.fontChain,
                environment: p.plan.scenes[id].environment, variant: p.plan.scenes[id].variant
            };
            var started = env.now();
            await channel.hello(env.limits.sceneBuildDeadlineMs);
```

In `ui/js/storymaker/plan.js`, replace:

```js
    function planTiming(story, audioInfo, catalog, opts) {
        var cat = SM.scene.indexCatalog(catalog), issues = [];
        var fmt = normaliseFormat(story, opts);
        var plans = planAll(story, audioInfo || {}, cat, fmt, issues), scenes = {}, glyphIdx = glyphIndex(catalog);
        plans.forEach(function (p) {
            var s = story.scenes[p.index];
            scenes[s.id] = {
                index: p.index, kind: s.kind === 'code' ? 'code' : 'storyboard', transition_frames: p.tf,
                narration_start: p.narration.start, narration_seconds: p.narration.seconds, cues: p.narration.cues
            };
        });
```

with:

```js
    function planTiming(story, audioInfo, catalog, opts) {
        var cat = SM.scene.indexCatalog(catalog), issues = [];
        var fmt = normaliseFormat(story, opts);
        var plans = planAll(story, audioInfo || {}, cat, fmt, issues), scenes = {}, glyphIdx = glyphIndex(catalog), envPrev = null;
        plans.forEach(function (p) {
            var s = story.scenes[p.index];
            envPrev = s.environment || envPrev || 'env.plain-stage';   // the rule of compile.js: a scene keeps the environment before it
            scenes[s.id] = {
                index: p.index, kind: s.kind === 'code' ? 'code' : 'storyboard', environment: envPrev, variant: s.variant === 'night' ? 'night' : 'day', transition_frames: p.tf,
                narration_start: p.narration.start, narration_seconds: p.narration.seconds, cues: p.narration.cues
            };
        });
```

In `ui/js/storymaker/raster.js`, replace:

```js
        this.overlayTexture = new (V().Texture)({ source: new (V().CanvasSource)({ resource: this.overlay, autoGarbageCollect: false }) });
        this.overlaySprite = new (V().Sprite)(this.overlayTexture);
        this.root.addChild(this.overlaySprite);
    }
    Drawer.prototype.texture = function (image) {
        var key = image.file + ':' + (image.rect || []).join(',');
```

with:

```js
        this.overlayTexture = new (V().Texture)({ source: new (V().CanvasSource)({ resource: this.overlay, autoGarbageCollect: false }) });
        this.overlaySprite = new (V().Sprite)(this.overlayTexture);
        this.root.addChild(this.overlaySprite);
        // the board of env.board sits under every other node: a second canvas, drawn again only when its inputs change
        this.underlay = owner.env.makeCanvas(canvas.width, canvas.height);
        this.underlayContext = this.underlay.getContext('2d');
        this.underlayTexture = new (V().Texture)({ source: new (V().CanvasSource)({ resource: this.underlay, autoGarbageCollect: false }) });
        this.underlaySprite = new (V().Sprite)(this.underlayTexture);
        this.underlaySprite.zIndex = -1;
        this.underlaySprite.visible = false;
        this.root.addChild(this.underlaySprite);
    }
    Drawer.prototype.texture = function (image) {
        var key = image.file + ':' + (image.rect || []).join(',');
```

In `ui/js/storymaker/raster.js`, replace:

```js
        var self = this, stage = SM.render.stageFor(sc, rc), cam = SM.camera.at(sc.camera, time);
        var win = stage.cameraWindow(cam), base = stage.baseWindow(), camera = stage.matrix(win), z = 0;
        this.used.clear(); this.root.sortableChildren = true;
        if (this.models) this.models.instances.forEach(function (v) { v.sprite.visible = false; });
        this.renderer.background.color = rc.palette.bg;
        var environment = sc.env && this.owner.files[sc.env.file];
```

with:

```js
        var self = this, stage = SM.render.stageFor(sc, rc), cam = SM.camera.at(sc.camera, time);
        var win = stage.cameraWindow(cam), base = stage.baseWindow(), camera = stage.matrix(win), z = 0;
        this.used.clear(); this.root.sortableChildren = true;
        var board = SM.blocks.isBoard(sc);
        this.underlaySprite.visible = board;
        if (board) {
            var boardKey = JSON.stringify([sc.variant, rc.palette.bg, rc.palette.fg, rc.W, rc.H]);
            if (this.underlayKey !== boardKey) {
                this.underlay.width = rc.W;
                SM.blocks.drawBoard(this.underlayContext, { variant: sc.variant, palette: rc.palette, W: rc.W, H: rc.H });
                this.underlayTexture.source.update(); this.underlayKey = boardKey;
            }
        }
        if (this.models) this.models.instances.forEach(function (v) { v.sprite.visible = false; });
        this.renderer.background.color = rc.palette.bg;
        var environment = sc.env && this.owner.files[sc.env.file];
```

In `ui/js/storymaker/raster.js`, replace:

```js
        if (this.models) { this.models.dispose(); this.models = null; }
        this.root.destroy({ children: true });
        Array.from(this.textures.keys()).filter(function (k) { return k.endsWith(':source'); }).forEach(function (k) { this.dropTexture(k.slice(0,-7)); }, this);
        this.overlayTexture.destroy(true); this.renderer.destroy(); this.nodes.clear();
        this.overlay.width = this.overlay.height = this.canvas.width = this.canvas.height = 1;
    };
    async function get(owner, width, height) {
        var generation = owner.graphicsGeneration;
```

with:

```js
        if (this.models) { this.models.dispose(); this.models = null; }
        this.root.destroy({ children: true });
        Array.from(this.textures.keys()).filter(function (k) { return k.endsWith(':source'); }).forEach(function (k) { this.dropTexture(k.slice(0,-7)); }, this);
        this.overlayTexture.destroy(true); this.underlayTexture.destroy(true); this.renderer.destroy(); this.nodes.clear();
        this.overlay.width = this.overlay.height = this.underlay.width = this.underlay.height = this.canvas.width = this.canvas.height = 1;
    };
    async function get(owner, width, height) {
        var generation = owner.graphicsGeneration;
```

In `ui/js/storymaker/render.js`, replace:

```js
        var stage = stageFor(sc, rc);
        var cam = SM.camera.at(sc.camera, t), win = stage.cameraWindow(cam), bw = stage.baseWindow(), Mv = stage.matrix(win);
        drawBackdrop(ctx, rc);
        drawEnvironment(ctx, sc, rc, win, bw, Mv);
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; s.scale *= pulseOf(s); return s; });
        var live = function (s) { return s.alpha > 0; };
```

with:

```js
        var stage = stageFor(sc, rc);
        var cam = SM.camera.at(sc.camera, t), win = stage.cameraWindow(cam), bw = stage.baseWindow(), Mv = stage.matrix(win);
        drawBackdrop(ctx, rc);
        if (SM.blocks.isBoard(sc)) SM.blocks.drawBoard(ctx, { variant: sc.variant, palette: rc.palette, W: rc.W, H: rc.H });
        drawEnvironment(ctx, sc, rc, win, bw, Mv);
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; s.scale *= pulseOf(s); return s; });
        var live = function (s) { return s.alpha > 0; };
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 296 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/06-codescene.mjs scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/codescene.js ui/js/storymaker/host-core.js ui/js/storymaker/host.bundle.js ui/js/storymaker/plan.js ui/js/storymaker/player.bundle.js ui/js/storymaker/raster.js ui/js/storymaker/render.js
git commit -m "storymaker: the board backdrop is drawn for storyboard and code scenes"
```

### Task C11: text fitting splits at whitespace; no regex lost its backslash

**Files:**
- Modify: `scripts/storymaker-tests/08-host-app.mjs`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/host-app.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/08-host-app.mjs`, replace:

```js
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', new Array(41).fill('M0 0'))] })), /icon_data/, 'at most 40 paths');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', ['M' + '0'.repeat(2000)])] })), /icon_data/, 'at most 2000 characters per path');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: new Array(1001).fill(icon('icon.cpu')) })), /icon_data/, 'at most 1000 icons');
    assert.equal(ok(icons(readRepoJSON('ui/img/storymaker/icons/lucide.json'))), null, 'the shipped icon file fits the caps');
});
```

with:

```js
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', new Array(41).fill('M0 0'))] })), /icon_data/, 'at most 40 paths');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', ['M' + '0'.repeat(2000)])] })), /icon_data/, 'at most 2000 characters per path');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: new Array(1001).fill(icon('icon.cpu')) })), /icon_data/, 'at most 1000 icons');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('iconXcpu')] })), /icon_data/, 'the dot of the icon. prefix is literal');
    assert.equal(ok(icons(readRepoJSON('ui/img/storymaker/icons/lucide.json'))), null, 'the shipped icon file fits the caps');
});
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const plan = SM.planTiming({ scenes: [{ id: 'a' }, { id: 'b', environment: 'env.board', variant: 'night' }, { id: 'c', kind: 'code' }, { id: 'd', environment: 'env.office' }] }, {}, t.catalog).scenes;
    assert.deepEqual(['a', 'b', 'c', 'd'].map(id => [plan[id].environment, plan[id].variant]), [['env.plain-stage', 'day'], ['env.board', 'night'], ['env.board', 'day'], ['env.office', 'day']], 'a scene keeps the environment of the one before it');
});
```

with:

```js
    const plan = SM.planTiming({ scenes: [{ id: 'a' }, { id: 'b', environment: 'env.board', variant: 'night' }, { id: 'c', kind: 'code' }, { id: 'd', environment: 'env.office' }] }, {}, t.catalog).scenes;
    assert.deepEqual(['a', 'b', 'c', 'd'].map(id => [plan[id].environment, plan[id].variant]), [['env.plain-stage', 'day'], ['env.board', 'night'], ['env.board', 'day'], ['env.office', 'day']], 'a scene keeps the environment of the one before it');
});

// ---- C11: text fitting splits at whitespace

check('blocks: a long card label shrinks instead of breaking mid-word', () => {
    const area = { x: 0, y: 0, w: 400, h: 400, surface: 'panel' };
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'card', label: 'Warteschlangen' }, 5, { area });
    const texts = log.filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts, ['Warteschlangen'], 'the whole word is on one line: ' + texts.join('|'));
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: a card label "Warteschlangen" printed as Warteschlange|n (fit split at the letter s); icon id iconXcpu passed checkIconData

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out, words = String(text).split(/s+/);
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
```

with:

```js
    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out, words = String(text).split(/\s+/);
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
```

In `ui/js/storymaker/host-app.js`, replace:

```js
        if (!Array.isArray(data.icons) || data.icons.length > CAPS.maxIcons) return 'load.catalog.icon_data has bad icons or too many icons';
        for (var i = 0; i < data.icons.length; i++) {
            var icon = data.icons[i];
            if (!isObject(icon) || typeof icon.id !== 'string' || !/^icon.[a-z0-9-]{1,60}$/.test(icon.id)) return 'load.catalog.icon_data has a bad icon id';
            if (!Array.isArray(icon.paths) || icon.paths.length > CAPS.maxIconPaths) return 'load.catalog.icon_data has bad or too many paths';
            for (var k = 0; k < icon.paths.length; k++) {
                if (typeof icon.paths[k] !== 'string' || icon.paths[k].length > CAPS.maxIconPathChars) return 'load.catalog.icon_data has a bad path';
```

with:

```js
        if (!Array.isArray(data.icons) || data.icons.length > CAPS.maxIcons) return 'load.catalog.icon_data has bad icons or too many icons';
        for (var i = 0; i < data.icons.length; i++) {
            var icon = data.icons[i];
            if (!isObject(icon) || typeof icon.id !== 'string' || !/^icon\.[a-z0-9-]{1,60}$/.test(icon.id)) return 'load.catalog.icon_data has a bad icon id';
            if (!Array.isArray(icon.paths) || icon.paths.length > CAPS.maxIconPaths) return 'load.catalog.icon_data has bad or too many paths';
            for (var k = 0; k < icon.paths.length; k++) {
                if (typeof icon.paths[k] !== 'string' || icon.paths[k].length > CAPS.maxIconPathChars) return 'load.catalog.icon_data has a bad path';
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: label stays one line; iconXcpu is refused; sweep of all regex literals in smb-a..smb-c found only these two

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/08-host-app.mjs scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/host-app.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: text fitting splits at whitespace; no regex lost its backslash"
```

### Task C12: pie legend, list and compare fit narrow areas

**Files:**
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Create: `ui/js/storymaker/blocks-plans.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const texts = log.filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts, ['Warteschlangen'], 'the whole word is on one line: ' + texts.join('|'));
});
```

with:

```js
    const texts = log.filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts, ['Warteschlangen'], 'the whole word is on one line: ' + texts.join('|'));
});

// ---- C12: pie legend, list and compare fit narrow areas

// fitted(def, aspect, at) = {area, plan, texts}: the plan box and every painted text as {s, x, y, px} in block coordinates
function fitted(def, aspect, at) {
    const W = aspect === '9:16' ? 1080 : 1920, H = aspect === '9:16' ? 1920 : 1080;
    const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect), area = B.area(stage, null, 'day', at);
    const st = SM.scene.evalEntity(B.create({ t0: 0, dur: 1, ...def, area }), 5), cv = new FakeCanvas(W, H);
    const pl = B.plan(cv.ctx, st, rc, stage);
    B.draw(cv.ctx, st, rc, stage, stage.baseWindow());
    let px = 0;
    const texts = [];
    for (const l of cv.ctx.log) {
        if (l.startsWith('set font=')) px = parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]);
        const m = /^fillText\((.*),(-?[\d.]+),(-?[\d.]+)\)$/.exec(l);
        if (m) texts.push({ s: m[1], x: +m[2], y: +m[3], px });
    }
    return { area, st, plan: pl, texts };
}
const insideArea = (r, what) => {
    const { box } = r.plan, hw = r.area.w / 2, hh = r.area.h / 2;
    assert.ok(box.x >= -hw - 1e-6 && box.y >= -hh - 1e-6 && box.x + box.w <= hw + 1e-6 && box.y + box.h <= hh + 1e-6, what + ': plan box ' + JSON.stringify(box));
    for (const x of r.texts) assert.ok(x.y - x.px / 2 >= -hh - 0.5 && x.y + x.px / 2 <= hh + 0.5, what + ': text "' + x.s + '" at y ' + x.y + ' (' + x.px + 'px) leaves +-' + hh);
};
const CASES = [['16:9', 'right'], ['9:16', 'top']];

check('blocks: pie legends fit the area for 4 and 6 slices at 16:9 right and 9:16 top', () => {
    for (const [aspect, at] of CASES) for (const n of [4, 6]) {
        const values = Array.from({ length: n }, (_, i) => i + 1);
        insideArea(fitted(chartDef('pie', values, { labels: values.map((_, i) => 'Anteil ' + i) }), aspect, at), 'pie ' + n + ' ' + aspect + ' ' + at);
    }
});

check('blocks: a list of two-line items stays in the area, at its label floor or above', () => {
    const text = ['Erster Punkt mit einem langen Satz', 'Zweiter Punkt mit einem langen Satz', 'Dritter Punkt mit einem langen Satz', 'Vierter Punkt mit einem langen Satz'].join('\n');
    for (const [aspect, at] of CASES) {
        const r = fitted({ id: 'l', kind: 'list', text }, aspect, at);
        insideArea(r, 'list ' + aspect + ' ' + at);
        assert.ok(r.plan.labelPx > 0);
    }
});

check('blocks: a compare diagram with two-line headers and three two-line points keeps its text inside each node', () => {
    const node = (id, k) => ({ id, label: 'Kopfzeile ' + k + ' mit mehr Text', points: ['Erster langer Punkt ' + k, 'Zweiter langer Punkt ' + k, 'Dritter langer Punkt ' + k] });
    for (const [aspect, at] of CASES) {
        const r = fitted({ id: 'd', kind: 'diagram', diagram: { layout: 'compare', nodes: [node('a', 1), node('b', 2)] } }, aspect, at);
        insideArea(r, 'compare ' + aspect + ' ' + at);
        const boxes = B.layout('compare', 2, r.area.w, r.area.h).boxes;
        for (const x of r.texts) {
            const b = boxes.find(o => x.x >= o.x && x.x <= o.x + o.w);
            assert.ok(b && x.y - x.px / 2 >= b.y - 0.5 && x.y + x.px / 2 <= b.y + b.h + 0.5, 'compare ' + aspect + ': "' + x.s + '" y ' + x.y + ' outside its node ' + JSON.stringify(b));
        }
    }
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: pie legend text at y 450 outside +-401 (16:9 right), list box 819 high in an 803 area, compare text below its node at 9:16 top

- [ ] **Step 3: Implement**

In `scripts/storymaker-bundle-parts.js`, replace:

```js
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/blocks-draw.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

with:

```js
  'ui/js/storymaker/bubble.js',
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/blocks-draw.js',
  'ui/js/storymaker/blocks-plans.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icons = c.rc.cat && c.rc.cat.icons, icon = node.icon && icons ? icons[node.icon] : null, m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; o.fit = fit(c, node.label, 700, box.w - pad * 2 - (icon ? o.gs + pad : 0), 2, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
            o.mode = 'top';
```

with:

```js
    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icons = c.rc.cat && c.rc.cat.icons, icon = node.icon && icons ? icons[node.icon] : null, m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; return compareLayout(c, node, o, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
            o.mode = 'top';
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
            if (o.gs < 0.3 * box.h) { o.gs = 0; o.mode = 'plain'; o.icon = null; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        }
        else { o.mode = 'plain'; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        if (compare) {
            var pts = (node.points || []).map(function (s) { return fit(c, s, 500, box.w - pad * 3.2, 2, Math.max(c.floor, o.fit.px * 0.8)); });
            o.points = pts;
        }
        return o;
    }

    function paintNode(c, o, node, p, lw, accent, mark) {
        var ctx = c.ctx, b = o.box, cx = b.x + b.w / 2, cy = b.y + b.h / 2, draw = c.st.entry === 'draw', ink = accent ? c.col.onAccent : c.col.ink;
        var k = (mark && mark.style === 'pulse' ? 1 + 0.08 * Math.sin(4 * Math.PI * mark.u) : 1) * (draw ? 1 : 0.92 + 0.08 * p);
```

with:

```js
            if (o.gs < 0.3 * box.h) { o.gs = 0; o.mode = 'plain'; o.icon = null; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        }
        else { o.mode = 'plain'; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        return o;
    }

    // compareLayout(c, node, o, px): the header and the points of a compare node, shrunk together (down to the label floor) until
    // the padding, the header, the divider and every point fit the node height.
    function compareLayout(c, node, o, px) {
        var box = o.box, pad = o.pad, hw = box.w - pad * 2 - (o.icon ? o.gs + pad : 0);
        for (;;) {
            o.fit = fit(c, node.label, 700, hw, 2, px);
            o.points = (node.points || []).map(function (s) { return fit(c, s, 500, box.w - pad * 3.2, 2, Math.max(c.floor, o.fit.px * 0.8)); });
            var need = pad * 2 + Math.max(o.gs, o.fit.lines.length * o.fit.px * 1.2);
            o.points.forEach(function (pt) { need += pad * 0.5 + pt.lines.length * pt.px * 1.2; });
            if (need <= box.h || o.fit.px <= c.floor) return o;
            px = Math.max(c.floor, o.fit.px * 0.94);
        }
    }

    function paintNode(c, o, node, p, lw, accent, mark) {
        var ctx = c.ctx, b = o.box, cx = b.x + b.w / 2, cy = b.y + b.h / 2, draw = c.st.entry === 'draw', ink = accent ? c.col.onAccent : c.col.ink;
        var k = (mark && mark.style === 'pulse' ? 1 + 0.08 * Math.sin(4 * Math.PI * mark.u) : 1) * (draw ? 1 : 0.92 + 0.08 * p);
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        };
    }

    // piePlan: a donut with a legend; the slices sweep clockwise from the top while p grows.
    function piePlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, sum = 0;
        vals.forEach(function (v) { sum += v; });
        var wide = f.x1 - f.x0 > (f.y1 - f.top) * 1.35, R = wide ? Math.min((f.y1 - f.top) / 2, (f.x1 - f.x0) * 0.3) : Math.min((f.x1 - f.x0) / 2, (f.y1 - f.top) * 0.36);
        var rows = ch.labels.map(function (l, k) { return String(l) + ' ' + Math.round(100 * vals[k] / sum) + '%'; });
        var lab = fitSet(c, rows, 600, wide ? f.x1 - f.x0 - R * 2.45 - c.base * 1.4 : f.x1 - f.x0 - c.base * 1.4, 1, c.base), rowH = lab.px * 1.9;
        // the donut and its legend are centred as one group
        var rowsW = Math.max.apply(null, rows.map(function (s) { return widthOf(c, s, 600, lab.px); })) + lab.px * 1.3;
        var cx = wide ? f.x0 + (f.x1 - f.x0 - R * 2.45 - rowsW) / 2 + R : (f.x0 + f.x1) / 2, cy = wide ? (f.top + f.y1) / 2 : f.top + R, r = R * 0.58;
        var legendX = wide ? cx + R * 1.45 : (f.x0 + f.x1 - rowsW) / 2, legendTop = wide ? 0 : cy + R + R * 0.2;
        var top = wide ? cy - n * rowH / 2 : legendTop, tint = function (k) { return [c.col.accent, c.col.accent2, c.col.ink][k % 3]; };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                var a0 = -Math.PI / 2, sweep = a0 + 2 * Math.PI * p;
                vals.forEach(function (v, k) {
                    var a1 = a0 + 2 * Math.PI * v / sum, end = Math.min(a1, sweep);
                    if (end > a0) {
                        ctx.globalAlpha = c.alpha * (k < 3 ? 1 : 0.55);
                        ctx.fillStyle = tint(k);
                        ctx.strokeStyle = c.col.base;
                        ctx.lineWidth = f.lw * 0.8;
                        ctx.lineJoin = 'round';
                        ctx.beginPath();
                        ctx.arc(cx, cy, R, a0, end);
                        ctx.arc(cx, cy, r, end, a0, true);
                        ctx.closePath();
                        ctx.fill();
                        ctx.stroke();
                    }
                    var show = clamp((p - 0.3) / 0.7, 0, 1), y = top + rowH * (k + 0.5);
                    ctx.globalAlpha = c.alpha * show * (k < 3 ? 1 : 0.55);
                    ctx.fillStyle = tint(k);
                    SM.text.roundRect(ctx, legendX, y - lab.px * 0.4, lab.px * 0.8, lab.px * 0.8, lab.px * 0.2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha * show;
                    printLines(c, lab.lines[k], legendX + lab.px * 1.3, y, lab.px, 600, c.col.ink, 'left');
                    a0 = a1;
                });
            }
        };
    }

    // counterPlan: one big number that counts up to its value.
    function counterPlan(c, f) {
        var st = c.st, ch = st.block.chart, v = ch.values[0], p = B.itemProgress(st, 0), whole = v.toFixed(decimals(v)) + unitText(ch.unit);
```

with:

```js
        };
    }

    // counterPlan: one big number that counts up to its value.
    function counterPlan(c, f) {
        var st = c.st, ch = st.block.chart, v = ch.values[0], p = B.itemProgress(st, 0), whole = v.toFixed(decimals(v)) + unitText(ch.unit);
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        f.lw = Math.max(3, Math.min(st.w, st.h) * 0.008);
        f.unit = unitText(ch.unit);
        f.title = function () { if (title) printLines(c, title.lines, 0, -st.h / 2 + pad + title.px * 0.6, title.px, 700, c.col.ink, 'center'); };
        var plan = ch.type === 'pie' ? piePlan(c, f) : (ch.type === 'counter' ? counterPlan(c, f) : axisPlan(c, f));
        if (title) plan.labelPx = Math.min(plan.labelPx || title.px, title.px);
        return plan;
    }
```

with:

```js
        f.lw = Math.max(3, Math.min(st.w, st.h) * 0.008);
        f.unit = unitText(ch.unit);
        f.title = function () { if (title) printLines(c, title.lines, 0, -st.h / 2 + pad + title.px * 0.6, title.px, 700, c.col.ink, 'center'); };
        var plan = ch.type === 'pie' ? B.piePlan(c, f) : (ch.type === 'counter' ? counterPlan(c, f) : axisPlan(c, f));
        if (title) plan.labelPx = Math.min(plan.labelPx || title.px, title.px);
        return plan;
    }
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        };
    }

    // listPlan: up to four lines with an accent bullet, each sliding in from the left in order.
    function listPlan(c) {
        var st = c.st, rows = B.lines(st.block.text), n = rows.length, ctx = c.ctx;
        var lab = fitSet(c, rows, 600, st.w * 0.9 - c.base, 2, clamp(st.h / (n * 2), c.base, c.base * 1.8)), px = lab.px;
        var heights = lab.lines.map(function (l) { return l.length * px * 1.2 + px * 0.7; }), height = heights.reduce(function (a, b) { return a + b; }, 0);
        var width = Math.max.apply(null, lab.lines.map(function (l) { return Math.max.apply(null, l.map(function (s) { return widthOf(c, s, 600, px); })); })) + px * 0.9;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = (1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(-width / 2 + px * 0.2 + dx, mid, px * 0.2, 0, Math.PI * 2);
                    ctx.fill();
                    printLines(c, lines, -width / 2 + px * 0.9 + dx, mid, px, 600, c.col.ink, 'left');
                });
            }
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan, headline: headlinePlan, list: listPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

with:

```js
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan, headline: headlinePlan, list: function (c) { return B.listPlan(c); } };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        ctx.globalAlpha = 1;
    }

    B.draw = draw;
    B.drawBoard = drawBoard;
    B.plan = plan;
```

with:

```js
        ctx.globalAlpha = 1;
    }

    // the helpers that blocks-plans.js builds the pie and list plans from
    B.kit = { fit: fit, fitSet: fitSet, printLines: printLines, widthOf: widthOf, clamp: clamp };
    B.draw = draw;
    B.drawBoard = drawBoard;
    B.plan = plan;
```

Create `ui/js/storymaker/blocks-plans.js`:

```js
// Storymaker player: block plans of the pie chart and the list. They sit in their own part because blocks-draw.js is at its line
// budget. Each plan fits the area of the block: the pie picks the donut size and the legend position from the free width and
// height, the list shrinks its font until every line fits. Same contract as the plans of blocks-draw.js: plan(c, f) returns
// {box, labelPx, paint}. Classic script; adds piePlan and listPlan to StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks, K = B.kit;

    var ROW = 1.6;   // legend row height in label heights

    // pieGeometry(c, rows, W, H) = {px, R, side, rw}: the legend font, the donut radius and the legend position (beside the donut or
    // below it) for a W x H area. The font shrinks, down to the label floor, until the donut gets a third of the short side.
    function pieGeometry(c, rows, W, H) {
        var px = c.base, short = Math.min(W, H), n = rows.length, g;
        for (;;) {
            var legH = n * ROW * px, rw = 0, gapB = 0.4 * px, gapS = 0.9 * px;
            rows.forEach(function (s) { rw = Math.max(rw, K.widthOf(c, s, 600, px)); });
            rw += px * 1.3;
            var below = rw <= W ? Math.min(W / 2, (H - legH - gapB) / 2) : 0, beside = legH <= H ? Math.min(H / 2, (W - rw - gapS) / 2) : 0;
            g = { px: px, rw: rw, legH: legH, gapB: gapB, gapS: gapS, side: beside > below, R: Math.max(beside, below) };
            if (g.R >= 0.3 * short || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        g.R = Math.max(g.R, 0.1 * short);
        return g;
    }

    // piePlan: a donut with a legend; the slices sweep clockwise from the top while p grows.
    function piePlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, sum = 0;
        vals.forEach(function (v) { sum += v; });
        var rows = ch.labels.map(function (l, k) { return String(l) + ' ' + Math.round(100 * vals[k] / sum) + '%'; });
        var W = f.x1 - f.x0, H = f.y1 - f.top, g = pieGeometry(c, rows, W, H), R = g.R, r = R * 0.58;
        var lab = K.fitSet(c, rows, 600, g.side ? W - R * 2 - g.gapS : W, 1, g.px), rowH = ROW * lab.px;
        // the donut and its legend are centred as one group
        var group = g.side ? 2 * R + g.gapS + g.rw : W, cx, cy, legendX, top;
        if (g.side) {
            cx = f.x0 + (W - group) / 2 + R;
            cy = (f.top + f.y1) / 2;
            legendX = cx + R + g.gapS;
            top = cy - n * rowH / 2;
        } else {
            cy = f.top + (H - (2 * R + g.gapB + n * rowH)) / 2 + R;
            cx = (f.x0 + f.x1) / 2;
            legendX = (f.x0 + f.x1 - g.rw) / 2;
            top = cy + R + g.gapB;
        }
        var tint = function (k) { return [c.col.accent, c.col.accent2, c.col.ink][k % 3]; };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                var a0 = -Math.PI / 2, sweep = a0 + 2 * Math.PI * p;
                vals.forEach(function (v, k) {
                    var a1 = a0 + 2 * Math.PI * v / sum, end = Math.min(a1, sweep);
                    if (end > a0) {
                        ctx.globalAlpha = c.alpha * (k < 3 ? 1 : 0.55);
                        ctx.fillStyle = tint(k);
                        ctx.strokeStyle = c.col.base;
                        ctx.lineWidth = f.lw * 0.8;
                        ctx.lineJoin = 'round';
                        ctx.beginPath();
                        ctx.arc(cx, cy, R, a0, end);
                        ctx.arc(cx, cy, r, end, a0, true);
                        ctx.closePath();
                        ctx.fill();
                        ctx.stroke();
                    }
                    var show = K.clamp((p - 0.3) / 0.7, 0, 1), y = top + rowH * (k + 0.5);
                    ctx.globalAlpha = c.alpha * show * (k < 3 ? 1 : 0.55);
                    ctx.fillStyle = tint(k);
                    SM.text.roundRect(ctx, legendX, y - lab.px * 0.4, lab.px * 0.8, lab.px * 0.8, lab.px * 0.2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha * show;
                    K.printLines(c, lab.lines[k], legendX + lab.px * 1.3, y, lab.px, 600, c.col.ink, 'left');
                    a0 = a1;
                });
            }
        };
    }

    // listPlan: up to four lines with an accent bullet, each sliding in from the left in order. The font shrinks, down to the label
    // floor, until the lines fit the height of the area.
    function listPlan(c) {
        var st = c.st, rows = B.lines(st.block.text), n = rows.length, ctx = c.ctx, maxW = st.w * 0.9 - c.base;
        var px0 = K.clamp(st.h / (n * 2), c.floor, c.base * 1.8), lab, px, heights, height;
        for (;;) {
            lab = K.fitSet(c, rows, 600, maxW, 2, px0);
            px = lab.px;
            heights = lab.lines.map(function (l) { return l.length * px * 1.2 + px * 0.7; });
            height = heights.reduce(function (a, b) { return a + b; }, 0);
            if (height <= st.h || px <= c.floor) break;
            px0 = Math.max(c.floor, px * 0.94);
        }
        var width = Math.max.apply(null, lab.lines.map(function (l) { return Math.max.apply(null, l.map(function (s) { return K.widthOf(c, s, 600, px); })); })) + px * 0.9;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = (1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(-width / 2 + px * 0.2 + dx, mid, px * 0.2, 0, Math.PI * 2);
                    ctx.fill();
                    K.printLines(c, lines, -width / 2 + px * 0.9 + dx, mid, px, 600, c.col.ink, 'left');
                });
            }
        };
    }

    B.piePlan = piePlan;
    B.listPlan = listPlan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: pie, list and compare plans and painted text stay inside the area (and compare text inside its node) at 16:9 right and 9:16 top

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-bundle-parts.js scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/blocks-plans.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: pie legend, list and compare fit narrow areas"
```

### Task C13: untrusted block payloads and icon lookups are bounded

**Files:**
- Modify: `scripts/storymaker-tests/08-host-app.mjs`
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/host-app.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/08-host-app.mjs`, replace:

```js
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', ['M' + '0'.repeat(2000)])] })), /icon_data/, 'at most 2000 characters per path');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: new Array(1001).fill(icon('icon.cpu')) })), /icon_data/, 'at most 1000 icons');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('iconXcpu')] })), /icon_data/, 'the dot of the icon. prefix is literal');
    assert.equal(ok(icons(readRepoJSON('ui/img/storymaker/icons/lucide.json'))), null, 'the shipped icon file fits the caps');
});
```

with:

```js
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.cpu', ['M' + '0'.repeat(2000)])] })), /icon_data/, 'at most 2000 characters per path');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: new Array(1001).fill(icon('icon.cpu')) })), /icon_data/, 'at most 1000 icons');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('iconXcpu')] })), /icon_data/, 'the dot of the icon. prefix is literal');
    const long = 'M' + '0'.repeat(1999);
    assert.equal(ok(icons({ schema: 'storymaker-icons/1', icons: [icon('icon.a', new Array(40).fill(long))] })), null, 'a full icon passes');
    assert.match(ok(icons({ schema: 'storymaker-icons/1', icons: Array.from({ length: 14 }, (_, i) => icon('icon.a' + i, new Array(40).fill(long))) })), /icon_data/, 'the paths of all icons together stay under 1 MB');
    assert.equal(ok(icons(readRepoJSON('ui/img/storymaker/icons/lucide.json'))), null, 'the shipped icon file fits the caps');
});
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
            const b = boxes.find(o => x.x >= o.x && x.x <= o.x + o.w);
            assert.ok(b && x.y - x.px / 2 >= b.y - 0.5 && x.y + x.px / 2 <= b.y + b.h + 0.5, 'compare ' + aspect + ': "' + x.s + '" y ' + x.y + ' outside its node ' + JSON.stringify(b));
        }
    }
});
```

with:

```js
            const b = boxes.find(o => x.x >= o.x && x.x <= o.x + o.w);
            assert.ok(b && x.y - x.px / 2 >= b.y - 0.5 && x.y + x.px / 2 <= b.y + b.h + 0.5, 'compare ' + aspect + ': "' + x.s + '" y ' + x.y + ' outside its node ' + JSON.stringify(b));
        }
    }
});

// ---- C13: untrusted block payloads and icon lookups are bounded

check('blocks: revive refuses a segment with t1 < t0, unsorted segments, log ease from 0, and a __proto__ key; it keeps only known block fields', () => {
    const def = diagramDef(2, { area: { x: 0, y: 0, w: 400, h: 300, surface: 'panel' }, t0: 0, dur: 1 });
    const wire = () => JSON.parse(JSON.stringify(B.serialize([B.create(def)], null)));
    const seg = (t0, t1, from, to, log = false) => ({ t0, t1, from, to, ease: 'linear', log });
    const withSegs = segs => { const w = wire(); w.blocks[0].alpha.segs = segs; return B.revive(w, cat); };
    assert.notEqual(withSegs([seg(0, 1, 0, 1), seg(1, 2, 1, 0)]), null, 'sorted segments pass');
    assert.equal(withSegs([seg(2, 1, 0, 1)]), null, 't1 < t0');
    assert.equal(withSegs([seg(1, 2, 0, 1), seg(0, 1, 1, 0)]), null, 'segments out of order');
    assert.equal(withSegs([seg(0, 1, 0, 1, true)]), null, 'log ease from 0');
    assert.equal(withSegs([seg(0, 1, 1, -2, true)]), null, 'log ease to a negative value');
    assert.notEqual(withSegs([seg(0, 1, 1, 2, true)]), null, 'log ease between positive values passes');
    const proto = wire();
    proto.blocks[0].block = JSON.parse(JSON.stringify(proto.blocks[0].block).slice(0, -1) + ',"__proto__":{"x":1}}');
    assert.ok(Object.keys(proto.blocks[0].block).includes('__proto__'), 'the payload really has an own __proto__ key');
    assert.equal(B.revive(proto, cat), null, 'a __proto__ key');
    const extra = wire();
    extra.blocks[0].block.secret = { deep: 1 };
    extra.blocks[0].block.diagram.nodes[0].extra = 5;
    const back = B.revive(extra, cat);
    assert.notEqual(back, null);
    assert.deepEqual(Object.keys(back.entities[0].block).sort(), ['diagram', 'id', 'kind'], 'unknown block fields are dropped');
    assert.deepEqual(Object.keys(back.entities[0].block.diagram.nodes[0]).sort(), ['id', 'label'], 'unknown node fields are dropped');
});

check('blocks: an icon path named like an Object property draws as a path', () => {
    const odd = SM.scene.indexCatalog({ ...t.catalog, icon_data: { schema: 'storymaker-icons/1', icons: [{ id: 'icon.odd', paths: ['constructor', 'toString'], tags: [] }] } });
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.odd', style: 'line' }, 5, { cat: odd });
    assert.deepEqual(log.filter(l => l.startsWith('stroke(')), ['stroke(path(constructor))', 'stroke(path(toString))']);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: revive accepted t1 < t0, unsorted segments, log ease from 0 and a __proto__ key; icon path "constructor" printed as a function; icon_data over 1 MB of path text passed

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js

    var DASH = 96;   // the dash that reveals a stroke: a dash of DASH * p and a gap of DASH, in 24-grid units for icons
    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = {}, pathCount = 0;

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function r3(v) { return Math.round(v * 1000) / 1000; }
```

with:

```js

    var DASH = 96;   // the dash that reveals a stroke: a dash of DASH * p and a gap of DASH, in 24-grid units for icons
    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = Object.create(null), pathCount = 0;

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function r3(v) { return Math.round(v * 1000) / 1000; }
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    function pathOf(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 2000) { pathCache = {}; pathCount = 0; }
            p = pathCache[d] = new Path2D(d);
            pathCount++;
        }
        return p;
    }

    // ---- colours: ink, accent, soft and the colour of a line on an accent fill, by surface (see the seams document)
    function colours(rc, surface) {
        var p = rc.palette || {}, dark = surface === 'dark';
```

with:

```js
    function pathOf(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 2000) { pathCache = Object.create(null); pathCount = 0; }
            p = pathCache[d] = new Path2D(d);
            pathCount++;
        }
        return p;
    }

    // iconOf(c, id): the icon of the catalog under that id, an own entry only, so an id like "constructor" finds nothing.
    function iconOf(c, id) {
        var icons = c.rc.cat && c.rc.cat.icons;
        return icons && typeof id === 'string' && Object.prototype.hasOwnProperty.call(icons, id) ? icons[id] : null;
    }

    // ---- colours: ink, accent, soft and the colour of a line on an accent fill, by surface (see the seams document)
    function colours(rc, surface) {
        var p = rc.palette || {}, dark = surface === 'dark';
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    }

    function iconPlan(c) {
        var st = c.st, b = st.block, ctx = c.ctx, icon = c.rc.cat && c.rc.cat.icons ? c.rc.cat.icons[b.icon] : null;
        var m = Math.min(st.w, st.h), style = B.ICON_STYLES.indexOf(b.style) >= 0 ? b.style : (b.label ? 'card' : 'badge');
        var p = st.entry === 'draw' ? B.itemProgress(st, 0) : 1;
        if (style === 'line') {
```

with:

```js
    }

    function iconPlan(c) {
        var st = c.st, b = st.block, ctx = c.ctx, icon = iconOf(c, b.icon);
        var m = Math.min(st.w, st.h), style = B.ICON_STYLES.indexOf(b.style) >= 0 ? b.style : (b.label ? 'card' : 'badge');
        var p = st.entry === 'draw' ? B.itemProgress(st, 0) : 1;
        if (style === 'line') {
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js

    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icons = c.rc.cat && c.rc.cat.icons, icon = node.icon && icons ? icons[node.icon] : null, m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; return compareLayout(c, node, o, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
```

with:

```js

    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icon = iconOf(c, node.icon), m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; return compareLayout(c, node, o, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        if (typeof value === 'string') return value.length <= MAX_STRING;
        if (value === null || typeof value === 'boolean' || value === undefined) return true;
        if (depth > 8 || typeof value !== 'object') return false;
        return Object.keys(value).every(function (k) { return k.length <= MAX_STRING && sane(value[k], depth + 1); });
    }

    function trackIn(plain, T) {
        if (!isObject(plain) || !finite(plain.init) || !Array.isArray(plain.segs) || plain.segs.length > MAX_SEGS) return null;
        var track = new T.Track(plain.init);
        for (var i = 0; i < plain.segs.length; i++) {
            var s = plain.segs[i];
            if (!isObject(s) || !finite(s.t0) || !finite(s.t1) || !finite(s.from) || !finite(s.to) || EASES.indexOf(s.ease) < 0) return null;
            track.segs.push({ t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: s.log === true, order: i });
        }
        return track;
```

with:

```js
        if (typeof value === 'string') return value.length <= MAX_STRING;
        if (value === null || typeof value === 'boolean' || value === undefined) return true;
        if (depth > 8 || typeof value !== 'object') return false;
        return Object.keys(value).every(function (k) { return k.length <= MAX_STRING && k !== '__proto__' && k !== 'constructor' && k !== 'prototype' && sane(value[k], depth + 1); });
    }

    // cleanBlock(b): a copy of a checked block with the known fields of its kind only, so a payload cannot smuggle other keys in.
    var KIND_KEYS = { icon: ['icon', 'style', 'label'], diagram: [], chart: [], headline: ['text'], list: ['text'] };
    function pick(src, keys) {
        var o = {};
        keys.forEach(function (k) { if (Object.prototype.hasOwnProperty.call(src, k) && src[k] !== undefined) o[k] = JSON.parse(JSON.stringify(src[k])); });
        return o;
    }
    function cleanBlock(b) {
        var o = pick(b, ['id', 'kind'].concat(KIND_KEYS[b.kind]));
        if (b.kind === 'diagram') {
            o.diagram = pick(b.diagram, ['layout', 'flow']);
            o.diagram.nodes = b.diagram.nodes.map(function (n) { return pick(n, ['id', 'label', 'icon', 'points']); });
        }
        if (b.kind === 'chart') o.chart = pick(b.chart, ['type', 'labels', 'values', 'unit', 'title']);
        return o;
    }

    function trackIn(plain, T) {
        if (!isObject(plain) || !finite(plain.init) || !Array.isArray(plain.segs) || plain.segs.length > MAX_SEGS) return null;
        var track = new T.Track(plain.init), last = -Infinity;
        for (var i = 0; i < plain.segs.length; i++) {
            var s = plain.segs[i];
            if (!isObject(s) || !finite(s.t0) || !finite(s.t1) || !finite(s.from) || !finite(s.to) || EASES.indexOf(s.ease) < 0) return null;
            // the rules of Track.add: t1 >= t0, sorted by t0, and a log ease only between positive values
            if (s.t1 < s.t0 || s.t0 < last || (s.log === true && !(s.from > 0 && s.to > 0))) return null;
            last = s.t0;
            track.segs.push({ t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: s.log === true, order: i });
        }
        return track;
```

In `ui/js/storymaker/blocks.js`, replace:

```js
            tracks[name] = trackIn(p[name], T);
            if (!tracks[name]) return null;
        }
        var e = make(p.block, { x: tracks.x.init, y: tracks.y.init, w: p.w, h: p.h, surface: p.surface, t0: p.t0, dur: p.dur, entry: p.entry, reveals: p.reveals, hidden: false });
        Object.keys(tracks).forEach(function (name) { e[name] = tracks[name]; });
        return listIn(p.marks, ['node', 'style'], e.marks) && listIn(p.highlights, ['style'], e.highlights) ? e : null;
    }
```

with:

```js
            tracks[name] = trackIn(p[name], T);
            if (!tracks[name]) return null;
        }
        var e = make(cleanBlock(p.block), { x: tracks.x.init, y: tracks.y.init, w: p.w, h: p.h, surface: p.surface, t0: p.t0, dur: p.dur, entry: p.entry, reveals: p.reveals, hidden: false });
        Object.keys(tracks).forEach(function (name) { e[name] = tracks[name]; });
        return listIn(p.marks, ['node', 'style'], e.marks) && listIn(p.highlights, ['style'], e.highlights) ? e : null;
    }
```

In `ui/js/storymaker/host-app.js`, replace:

```js
    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000, maxIcons: 1000, maxIconPaths: 40, maxIconPathChars: 2000,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
```

with:

```js
    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000, maxIcons: 1000, maxIconPaths: 40, maxIconPathChars: 2000, maxIconTotalChars: 1048576,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
```

In `ui/js/storymaker/host-app.js`, replace:

```js
    function checkIconData(data) {
        if (!isObject(data) || data.schema !== 'storymaker-icons/1') return 'load.catalog.icon_data needs the schema string';
        if (!Array.isArray(data.icons) || data.icons.length > CAPS.maxIcons) return 'load.catalog.icon_data has bad icons or too many icons';
        for (var i = 0; i < data.icons.length; i++) {
            var icon = data.icons[i];
            if (!isObject(icon) || typeof icon.id !== 'string' || !/^icon\.[a-z0-9-]{1,60}$/.test(icon.id)) return 'load.catalog.icon_data has a bad icon id';
            if (!Array.isArray(icon.paths) || icon.paths.length > CAPS.maxIconPaths) return 'load.catalog.icon_data has bad or too many paths';
            for (var k = 0; k < icon.paths.length; k++) {
                if (typeof icon.paths[k] !== 'string' || icon.paths[k].length > CAPS.maxIconPathChars) return 'load.catalog.icon_data has a bad path';
            }
            if (icon.fills !== undefined && (!Array.isArray(icon.fills) || icon.fills.length > CAPS.maxIconPaths)) return 'load.catalog.icon_data has bad fills';
        }
        return null;
```

with:

```js
    function checkIconData(data) {
        if (!isObject(data) || data.schema !== 'storymaker-icons/1') return 'load.catalog.icon_data needs the schema string';
        if (!Array.isArray(data.icons) || data.icons.length > CAPS.maxIcons) return 'load.catalog.icon_data has bad icons or too many icons';
        var chars = 0;   // the paths of all icons together stay under maxIconTotalChars, like Go's maxIconsBytes
        for (var i = 0; i < data.icons.length; i++) {
            var icon = data.icons[i];
            if (!isObject(icon) || typeof icon.id !== 'string' || !/^icon\.[a-z0-9-]{1,60}$/.test(icon.id)) return 'load.catalog.icon_data has a bad icon id';
            if (!Array.isArray(icon.paths) || icon.paths.length > CAPS.maxIconPaths) return 'load.catalog.icon_data has bad or too many paths';
            for (var k = 0; k < icon.paths.length; k++) {
                if (typeof icon.paths[k] !== 'string' || icon.paths[k].length > CAPS.maxIconPathChars) return 'load.catalog.icon_data has a bad path';
                chars += icon.paths[k].length;
            }
            if (chars > CAPS.maxIconTotalChars) return 'load.catalog.icon_data has too many path characters';
            if (icon.fills !== undefined && (!Array.isArray(icon.fills) || icon.fills.length > CAPS.maxIconPaths)) return 'load.catalog.icon_data has bad fills';
        }
        return null;
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: revive refuses all of them and rebuilds the block from its known fields; path cache and icon lookups use own entries; checkIconData caps the summed path characters at 1048576

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/08-host-app.mjs scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/blocks.js ui/js/storymaker/host-app.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: untrusted block payloads and icon lookups are bounded"
```

### Task C14: icon strokes reveal evenly; list items slide in from the left

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks-draw.js`
- Modify: `ui/js/storymaker/blocks-plans.js`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js

check('blocks: entry draw reveals the strokes with a dash and ends without one', () => {
    const def = { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' };
    assert.ok(paint(def, timeFor(0.5)).includes('setLineDash([48,96])'));
    assert.ok(paint(def, 5).includes('setLineDash([])'));
    assert.ok(!paint(def, 5).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'));
    assert.equal(paint(def, 0).filter(l => l.startsWith('stroke(')).length, 0, 'nothing is drawn before the reveal starts');
```

with:

```js

check('blocks: entry draw reveals the strokes with a dash and ends without one', () => {
    const def = { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' };
    assert.ok(paint(def, timeFor(0.5)).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'), 'a partial dash while the stroke grows');
    assert.ok(paint(def, 5).includes('setLineDash([])'));
    assert.ok(!paint(def, 5).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'));
    assert.equal(paint(def, 0).filter(l => l.startsWith('stroke(')).length, 0, 'nothing is drawn before the reveal starts');
```

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.odd', style: 'line' }, 5, { cat: odd });
    assert.deepEqual(log.filter(l => l.startsWith('stroke(')), ['stroke(path(constructor))', 'stroke(path(toString))']);
});
```

with:

```js
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.odd', style: 'line' }, 5, { cat: odd });
    assert.deepEqual(log.filter(l => l.startsWith('stroke(')), ['stroke(path(constructor))', 'stroke(path(toString))']);
});

// ---- C14: even stroke reveal, list items slide in from the left

check('blocks: pathLength measures lines exactly and curves and arcs approximately', () => {
    const len = d => B.pathLength(d);
    near(len('M0 0L10 0'), 10, 1e-9);
    near(len('M0 0l-3-4'), 5, 1e-9, 'relative, packed signs');
    near(len('M0 0h10v10z'), 20 + Math.hypot(10, 10), 1e-9, 'h, v and the closing line');
    near(len('M0 0L1.5.5'), Math.hypot(1.5, 0.5), 1e-9, 'packed decimals');
    const circle = len('M0 10A10 10 0 1 0 20 10A10 10 0 1 0 0 10Z');
    assert.ok(Math.abs(circle - 62.8) <= 0.05 * 62.8, 'circle ' + circle);
    const curve = len('M0 0C0 10 10 10 10 0');
    assert.ok(curve > 10 && curve < 30, 'cubic ' + curve);
    assert.equal(len('M0 0A5 5 0 01 10 0'), len('M0 0A5 5 0 0 1 10 0'), 'packed arc flags');
    assert.equal(len(''), 0);
    assert.equal(len('constructor'), 0);
    assert.equal(len(null), 0);
});

check('blocks: every stroke of an icon is revealed by the same share of its own length', () => {
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' }, timeFor(0.5), { cat: realCat });
    const strokes = [];
    log.forEach((l, i) => {
        const m = /^stroke\(path\((.*)\)\)$/.exec(l);
        if (m) strokes.push({ d: m[1], dash: log[i - 1].slice(13, -2).split(',').map(Number) });
    });
    assert.ok(strokes.length >= 3, 'the icon has several strokes');
    for (const s of strokes) {
        const len = B.pathLength(s.d);
        near(s.dash[1], len, 0.001, s.d);
        near(s.dash[0], len * 0.5, 0.001, s.d);
    }
    assert.ok(new Set(strokes.map(s => s.dash[1])).size > 1, 'the strokes differ in length, so do their dashes');
});

check('blocks: list items slide in from the left', () => {
    const def = { id: 'l', kind: 'list', text: 'Laden\nRechnen', dur: 2 };
    const xOf = (time, s) => parseFloat(paint(def, time).find(l => l.startsWith('fillText(' + s + ',')).slice(10 + s.length).split(',')[0]);
    assert.ok(xOf(timeFor(0.3), 'Laden') < xOf(5, 'Laden'), 'the first line is left of its final place while it slides');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: B.pathLength missing, every icon stroke used a fixed dash of 96 units, list items at p < 1 sat right of their final x

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks;

    var DASH = 96;   // the dash that reveals a stroke: a dash of DASH * p and a gap of DASH, in 24-grid units for icons
    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = Object.create(null), pathCount = 0;
```

with:

```js
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks;

    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = Object.create(null), pathCount = 0;
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js

    // ---- icons

    // glyph(c, icon, cx, cy, size, color, p): the strokes of an icon on the 24 grid, line width 2 * size / 24, revealed to p.
    function glyph(c, icon, cx, cy, size, color, p) {
        if (!icon || !(p > 0.001)) return;
        var ctx = c.ctx, fills = icon.fills || [];
```

with:

```js

    // ---- icons

    // glyph(c, icon, cx, cy, size, color, p): the strokes of an icon on the 24 grid, line width 2 * size / 24, each revealed to p of its length.
    function glyph(c, icon, cx, cy, size, color, p) {
        if (!icon || !(p > 0.001)) return;
        var ctx = c.ctx, fills = icon.fills || [];
```

In `ui/js/storymaker/blocks-draw.js`, replace:

```js
        ctx.lineWidth = 2;
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.setLineDash(p < 1 ? [r3(DASH * p), DASH] : []);
        for (var i = 0; i < icon.paths.length; i++) {
            var path = pathOf(icon.paths[i]);
            if (fills.indexOf(icon.paths[i]) >= 0) { ctx.globalAlpha = c.alpha * p; ctx.fill(path); ctx.globalAlpha = c.alpha; }
            ctx.stroke(path);
        }
```

with:

```js
        ctx.lineWidth = 2;
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        for (var i = 0; i < icon.paths.length; i++) {
            var path = pathOf(icon.paths[i]), len = Math.max(B.pathLength(icon.paths[i]), 0.01);
            // a dash of p of the stroke's own length and a gap of its whole length: every stroke of the icon finishes together
            ctx.setLineDash(p < 1 ? [r3(len * p), r3(len)] : []);
            if (fills.indexOf(icon.paths[i]) >= 0) { ctx.globalAlpha = c.alpha * p; ctx.fill(path); ctx.globalAlpha = c.alpha; }
            ctx.stroke(path);
        }
```

In `ui/js/storymaker/blocks-plans.js`, replace:

```js
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = (1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
```

with:

```js
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = -(1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        return JSON.stringify([st.id, st.x, st.y, st.alpha, st.scale, st.rot, st.block, st.w, st.h, st.surface, st.t0, st.dur, st.entry, st.reveals, st.mark, st.highlight || null, moving ? st.time : 0]);
    }

    // ---- serialize and revive

    function trackOut(track) {
```

with:

```js
        return JSON.stringify([st.id, st.x, st.y, st.alpha, st.scale, st.rot, st.block, st.w, st.h, st.surface, st.t0, st.dur, st.entry, st.reveals, st.mark, st.highlight || null, moving ? st.time : 0]);
    }

    // ---- path length

    var lengthCache = Object.create(null), lengthCount = 0;
    var ARGS = { M: 2, L: 2, H: 1, V: 1, C: 6, S: 4, Q: 4, T: 2, A: 7 };

    // pathLength(d) = the approximate length of an SVG path string in its own units: lines exactly, cubic and quadratic curves by the
    // mean of their chord and control polygon, arcs by their chord and radius. It reveals every stroke of an icon in step. Cached.
    function pathLength(d) {
        if (typeof d !== 'string') return 0;
        if (lengthCache[d] !== undefined) return lengthCache[d];
        var s = d, n = s.length, i = 0, total = 0, cmd = '', x = 0, y = 0, sx = 0, sy = 0, bad = false;
        function skip() { while (i < n && (s.charCodeAt(i) <= 32 || s.charCodeAt(i) === 44)) i++; }
        function digits() { var c; while (i < n && (c = s.charCodeAt(i)) >= 48 && c <= 57) i++; }
        function num() {
            skip();
            var a = i, j;
            if (s.charAt(i) === '-' || s.charAt(i) === '+') i++;
            digits();
            if (s.charAt(i) === '.') { i++; digits(); }
            if (i > a && (s.charAt(i) === 'e' || s.charAt(i) === 'E')) {
                j = i + 1;
                if (s.charAt(j) === '-' || s.charAt(j) === '+') j++;
                if (s.charCodeAt(j) >= 48 && s.charCodeAt(j) <= 57) { i = j; digits(); }
            }
            return i > a ? parseFloat(s.substring(a, i)) : NaN;
        }
        function flag() { skip(); var c = s.charAt(i); if (c === '0' || c === '1') { i++; return +c; } return NaN; }
        while (i < n && !bad) {
            skip();
            if (i >= n) break;
            var ch = s.charAt(i), up;
            if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
                cmd = ch;
                i++;
                if (ch === 'Z' || ch === 'z') { total += Math.hypot(sx - x, sy - y); x = sx; y = sy; continue; }
            }
            up = cmd.toUpperCase();
            if (!ARGS[up]) break;
            var a = [], rel = cmd !== up, k, v;
            for (k = 0; k < ARGS[up]; k++) {
                v = up === 'A' && (k === 3 || k === 4) ? flag() : num();
                if (v !== v) { bad = true; break; }
                a.push(v);
            }
            if (bad) break;
            var ox = rel ? x : 0, oy = rel ? y : 0, nx = x, ny = y, chord, poly;
            if (up === 'H') nx = ox + a[0];
            else if (up === 'V') ny = oy + a[0];
            else { nx = ox + a[ARGS[up] - 2]; ny = oy + a[ARGS[up] - 1]; }
            chord = Math.hypot(nx - x, ny - y);
            if (up === 'M') { sx = nx; sy = ny; cmd = rel ? 'l' : 'L'; }
            else if (up === 'C') { poly = Math.hypot(ox + a[0] - x, oy + a[1] - y) + Math.hypot(a[2] - a[0], a[3] - a[1]) + Math.hypot(nx - ox - a[2], ny - oy - a[3]); total += (chord + poly) / 2; }
            else if (up === 'S') { poly = Math.hypot(ox + a[0] - x, oy + a[1] - y) + Math.hypot(nx - ox - a[0], ny - oy - a[1]); total += (chord + poly) / 2; }
            else if (up === 'Q') { poly = Math.hypot(ox + a[0] - x, oy + a[1] - y) + Math.hypot(nx - ox - a[0], ny - oy - a[1]); total += (2 * chord + poly) / 3; }
            else if (up === 'A') {
                var r = (Math.abs(a[0]) + Math.abs(a[1])) / 2, half = chord / 2;
                if (!(r > 0)) total += chord;
                else if (half >= r) total += Math.PI * half;
                else total += r * (a[3] === 1 ? 2 * Math.PI - 2 * Math.asin(half / r) : 2 * Math.asin(half / r));
            } else total += chord;
            x = nx;
            y = ny;
        }
        if (lengthCount > 2000) { lengthCache = Object.create(null); lengthCount = 0; }
        lengthCount++;
        lengthCache[d] = total;
        return total;
    }

    // ---- serialize and revive

    function trackOut(track) {
```

In `ui/js/storymaker/blocks.js`, replace:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes, measure: measure, cacheKey: cacheKey
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes, measure: measure, cacheKey: cacheKey, pathLength: pathLength
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: pathLength("M0 0L10 0") is 10 and a circle of r 10 is within 5% of 62.8; each stroke is revealed by p of its own length; list items start left of their place

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks-draw.js ui/js/storymaker/blocks-plans.js ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: icon strokes reveal evenly; list items slide in from the left"
```

