# Part E: Code entry

> Generated from the drafted and tested commits 152499ce2..105ea7ca0. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task E1: api.icon and block handles

**Files:**
- Create: `scripts/storymaker-tests/35-block-api.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

Create `scripts/storymaker-tests/35-block-api.mjs`:

```js
// Batch E: the blocks API of a code scene (api.icon, api.diagram, api.chart, api.headline, api.list) and the board tokens.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { plain, FakeCanvas, loadPlayer } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene } = t;
const SM = t.core();

// The mini catalog has the default font at weight 400 only; the real one also has 600 and 700.
const fonts = [400, 600, 700].map(weight => ({ name: 'default', family: 'Storymaker Default', weight, file: '/fonts/placeholder.woff2', licence: 'OFL-1.1' }));
const fullCatalog = { ...catalog, fonts };

function runScene(source, extra) {
    const ctx = loadPlayer();
    vm.runInContext(bundleScene(source, 'x'), ctx);
    const C = ctx.StorymakerCore, story = extra && extra.story || { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei {2}drei' }] };
    const plan = C.planTiming(story, {}, fullCatalog), id = extra && extra.id || 'm';
    const opts = { sceneId: id, index: plan.scenes[id].index, catalog: fullCatalog, aspect: extra && extra.aspect || '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[id].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe, environment: plan.scenes[id].environment, variant: plan.scenes[id].variant };
    return { C, ctx, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
const build = source => runScene(source).build();
const fail = source => { try { build(source); } catch (e) { return String(e.message); } return ''; };

check('api.icon: a block in the right area with handle methods and the 600 and 700 fonts', () => {
    const built = build(`export default function scene(api) {
      const i = api.icon('icon.cpu', { at: 'right', style: 'card', label: 'CPU', start: 1 });
      i.fadeOut({ start: 3, dur: 1 }).moveTo('left', { start: 0, dur: 1 }).scaleTo(2, { start: 0, dur: 1 });
    }`);
    const [e] = built.entities, a = SM.blocks.area(built.stage, null, 'day', 'right');
    assert.equal(built.entities.length, 1);
    assert.equal(e.type, 'block');
    assert.equal(e.block.kind, 'icon'); assert.equal(e.block.icon, 'icon.cpu'); assert.equal(e.block.style, 'card'); assert.equal(e.block.label, 'CPU');
    assert.equal(e.entry, 'draw'); assert.equal(e.t0, 1); near(e.dur, 0.8, 1e-9);
    near(e.w, a.w, 1e-9); assert.equal(e.surface, 'panel');
    near(e.alpha.at(4), 0, 1e-9); near(e.alpha.at(2), 1, 1e-9);
    near(e.x.at(1), built.stage.resolve('left').x, 1e-6); near(e.scale.at(1), 2, 1e-9);
    assert.ok(plain(built.needs).includes('font:default/600') && plain(built.needs).includes('font:default/700'), plain(built.needs).join());
    const dflt = build("export default function scene(api) { api.icon('icon.cpu'); }").entities[0];
    near(dflt.w, SM.blocks.area(built.stage, null, 'day', 'center').w, 1e-9);
    assert.equal(dflt.entry, 'draw'); assert.equal(dflt.t0, 0);
});

check('api.icon: unknown ids, areas and entries throw a RangeError that names the fix', () => {
    assert.match(fail("export default function scene(api) { api.icon('icon.nope'); }"), /icon\.nope.*storymaker_catalog/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { at: 'nowhere' }); }"), /"nowhere".*full, left, right, top, center, board/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { entry: 'zoom' }); }"), /"zoom".*draw, pop, fade, slide, none/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { start: -1 }); }"), /start must be 0 or more/);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: 2 failed (api.icon is not a function)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
            return h;
        }

        // ---- the API
        var api = {};
        api.duration = function (sec) {
```

with:

```js
            return h;
        }

        // ---- blocks: icon, diagram, chart, headline and list share the storyboard renderer (SM.blocks)
        var BLOCK_ENTRIES = ['draw', 'pop', 'fade', 'slide', 'none'], blockCount = 0;
        function fontNeeds() {
            var font = opts.styleFont || 'default';
            (cat.fonts || []).forEach(function (f) { if (f.name === font && (f.weight === 600 || f.weight === 700)) state.needs['font:' + f.name + '/' + f.weight] = true; });
        }
        // addBlock(def, o, at, entry): o = {at, entry, start, dur, reveals}; the defaults are the area and the entry of the kind.
        function addBlock(def, o, at, entry) {
            o = o || {};
            if (blockCount >= SM.blocks.MAX_BLOCKS) throw new RangeError('A scene has at most ' + SM.blocks.MAX_BLOCKS + ' blocks. Remove some or merge them into one diagram or list.');
            var where = o.at === undefined || o.at === null ? at : o.at;
            var area = SM.blocks.area(stage, boardEnv, opts.variant, where);
            if (!area) throw new RangeError('Unknown area ' + JSON.stringify(where) + '. Use ' + SM.blocks.AREAS.join(', ') + ' or {x, y} in percent.');
            var how = o.entry === undefined ? entry : o.entry;
            if (BLOCK_ENTRIES.indexOf(how) < 0) throw new RangeError('Unknown entry "' + how + '". Use ' + BLOCK_ENTRIES.join(', ') + '.');
            var t = timing({ start: o.start, dur: o.dur }, SM.blocks.revealSeconds(def));
            Object.keys(def).forEach(function (k) { if (def[k] === undefined) delete def[k]; });   // SM.blocks.create copies fields through JSON
            def.id = nextId(def.kind);
            def.area = area;
            def.t0 = t.start;
            def.dur = t.dur;
            def.entry = how;
            if (o.reveals !== undefined) def.reveals = o.reveals;
            var bad = SM.blocks.check(def, cat);
            if (bad) throw new RangeError(bad.message + ' (' + bad.field + ')');
            blockCount++;
            fontNeeds();
            return common(addEntity(SM.blocks.create(def)));
        }

        // ---- the API
        var api = {};
        api.duration = function (sec) {
```

In `ui/js/storymaker/codescene.js`, replace:

```js
            var pos = place(o.at, 'shape');
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.diagram = function () { note('not_supported_yet', 'api.diagram is not supported yet; the diagram is not drawn.'); return common(S.createShape({ id: nextId('diagram'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.tween = function (target, props, o) {
```

with:

```js
            var pos = place(o.at, 'shape');
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.icon = function (id, o) { o = o || {}; return addBlock({ kind: 'icon', icon: id, style: o.style, label: o.label }, o, 'center', 'draw'); };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.diagram = function () { note('not_supported_yet', 'api.diagram is not supported yet; the diagram is not drawn.'); return common(S.createShape({ id: nextId('diagram'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.tween = function (target, props, o) {
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 298 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/35-block-api.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: api.icon and block handles"
```

### Task E2: api.diagram with highlight

**Files:**
- Modify: `scripts/storymaker-tests/35-block-api.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { entry: 'zoom' }); }"), /"zoom".*draw, pop, fade, slide, none/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { start: -1 }); }"), /start must be 0 or more/);
});
```

with:

```js
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { entry: 'zoom' }); }"), /"zoom".*draw, pop, fade, slide, none/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu', { start: -1 }); }"), /start must be 0 or more/);
});

const nodes = n => '[' + Array.from({ length: n }, (_, i) => `{ id: '${String.fromCharCode(97 + i)}', label: 'Node ${i}' }`).join(', ') + ']';

check('api.diagram: cue reveals are stored, highlight adds a node mark, there is no not_supported_yet issue', () => {
    const { C, build: run } = runScene(`export default function scene(api) {
      const d = api.diagram({ layout: 'flow', flow: true, nodes: ${nodes(3)} }, { reveals: [api.cue(1), api.cue(2), 3] });
      const same = d.highlight('b', { start: 3 });
      api.text(String(same === d), 'title');
    }`);
    const built = run(), [e] = built.entities, cues = C.planTiming({ ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei {2}drei' }] }, {}, fullCatalog).scenes.m.cues;
    assert.equal(e.type, 'block'); assert.equal(e.block.kind, 'diagram'); assert.equal(e.block.diagram.flow, true);
    assert.deepEqual(plain(e.reveals), [cues[1], cues[2], 3]);
    assert.equal(e.entry, 'draw'); near(e.dur, 0.5 + 0.6 * 3, 1e-9);
    assert.equal(built.entities[1].text, 'true', 'highlight returns the handle, so calls chain');
    assert.deepEqual(plain(e.marks.list), [{ t0: 3, t1: 4.2, data: { node: 'b', style: 'pulse' } }]);
    assert.deepEqual(plain(built.issues), []);
    const st = C.scene.evalEntity(e, 3.5);
    assert.deepEqual(plain(st.mark.node), 'b');
    assert.ok(C.scene.entityEnd(e) >= 4.2);
});

check('api.diagram: highlight and the layout limits name the fix', () => {
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(2)} }).highlight('zzz'); }`), /"zzz".*a, b/);
    assert.match(fail("export default function scene(api) { api.icon('icon.cpu').highlight('a'); }"), /highlight.*diagram/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(6)} }); }`), /flow diagram takes 2 to 5 nodes, got 6/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'ring', nodes: ${nodes(3)} }); }`), /"ring".*flow, cycle, hub, stack, compare/);
    assert.match(fail(`export default function scene(api) { api.diagram({ layout: 'flow', nodes: ${nodes(2)} }, { reveals: [1] }); }`), /reveals needs one finite number per node/);
    assert.match(fail("export default function scene(api) { api.diagram({ layout: 'flow', nodes: [{ id: 'a', label: 'A', icon: 'icon.nope' }, { id: 'b', label: 'B' }] }); }"), /icon\.nope.*storymaker_catalog/);
    assert.equal(fail(`export default function scene(api) { api.diagram({ nodes: ${nodes(3)} }); }`), '', 'the layout defaults to flow');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: 2 failed (d.highlight is not a function: api.diagram was the placeholder)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
            if (bad) throw new RangeError(bad.message + ' (' + bad.field + ')');
            blockCount++;
            fontNeeds();
            return common(addEntity(SM.blocks.create(def)));
        }

        // ---- the API
```

with:

```js
            if (bad) throw new RangeError(bad.message + ' (' + bad.field + ')');
            blockCount++;
            fontNeeds();
            var e = addEntity(SM.blocks.create(def)), h = common(e);
            // highlight(nodeId, timing): pulses one node of a diagram for 1.2 s (the default)
            h.highlight = function (node, ho) {
                if (e.block.kind !== 'diagram') throw new RangeError('highlight works on a diagram block; this one is a ' + e.block.kind + '.');
                var ids = e.block.diagram.nodes.map(function (n) { return n.id; });
                if (ids.indexOf(node) < 0) throw new RangeError('Unknown node id "' + String(node).slice(0, 40) + '". The node ids of this diagram are: ' + ids.join(', ') + '.');
                var t = timing(ho, 1.2);
                e.marks.add(t.start, t.end, { node: node, style: 'pulse' });
                return h;
            };
            return h;
        }

        // ---- the API
```

In `ui/js/storymaker/codescene.js`, replace:

```js
        };
        api.icon = function (id, o) { o = o || {}; return addBlock({ kind: 'icon', icon: id, style: o.style, label: o.label }, o, 'center', 'draw'); };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.diagram = function () { note('not_supported_yet', 'api.diagram is not supported yet; the diagram is not drawn.'); return common(S.createShape({ id: nextId('diagram'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
```

with:

```js
        };
        api.icon = function (id, o) { o = o || {}; return addBlock({ kind: 'icon', icon: id, style: o.style, label: o.label }, o, 'center', 'draw'); };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        // api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): the layout defaults to flow.
        api.diagram = function (spec, o) {
            var d = spec && typeof spec === 'object' && !Array.isArray(spec) ? Object.assign({}, spec) : spec;
            if (d && d.layout === undefined) d.layout = 'flow';
            return addBlock({ kind: 'diagram', diagram: d }, o, 'full', 'draw');
        };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 300 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/35-block-api.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: api.diagram with highlight"
```

### Task E3: api.chart; the placeholder notes go

**Files:**
- Modify: `scripts/storymaker-tests/06-codescene.mjs`
- Modify: `scripts/storymaker-tests/35-block-api.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
      api.tween(box, { x: 80, scale: 2 }, { start: 0, dur: 2, ease: 'linear' });
      const lock = api.add('prop.lock', { at: 'center', layer: 'back' });
      lock.moveTo('left', { start: 1, dur: 1 }).scaleTo(2, { start: 1, dur: 1 }).rotateTo(90, { start: 0, dur: 1, ease: 'linear' });
      const c = api.chart('bar', { labels: [], values: [] }, { at: 'right' });
      api.tween(c, { grow: 1 }, { start: api.cue(1), dur: 2 });
      api.camera.zoom(1.5, { start: 0, dur: 1 });
    }`, 'more', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build();
    const [text, box, lock] = more.entities;
    assert.equal(more.entities.length, 3, 'the inert chart is not an entity');
    near(text.alpha.at(0.5), 0, 1e-9); near(text.alpha.at(2), 1, 1e-9);
    near(box.x.at(2), 0.8 * 1920, 1e-9); near(box.x.at(1), (0.5 + 0.8) / 2 * 1920, 1e-6); near(box.scale.at(2), 2, 1e-9);
    near(lock.rot.at(0.5), 45, 1e-9); near(lock.scale.at(2), 2, 1e-9);
    assert.equal(lock.layer, 'back');
    assert.deepEqual(plain(more.issues.map(i => i.code)), ['not_supported_yet', 'not_supported_yet'], 'api.chart and the grow tween report not_supported_yet');
    near(more.camera.zoom.at(1), 1.5, 1e-9);
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});
```

with:

```js
      api.tween(box, { x: 80, scale: 2 }, { start: 0, dur: 2, ease: 'linear' });
      const lock = api.add('prop.lock', { at: 'center', layer: 'back' });
      lock.moveTo('left', { start: 1, dur: 1 }).scaleTo(2, { start: 1, dur: 1 }).rotateTo(90, { start: 0, dur: 1, ease: 'linear' });
      const c = api.chart({ type: 'bar', labels: ['a', 'b'], values: [1, 2] }, { at: 'right' });
      api.tween(c, { opacity: 0.5 }, { start: api.cue(1), dur: 0.5 });
      api.camera.zoom(1.5, { start: 0, dur: 1 });
    }`, 'more', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build();
    const [text, box, lock] = more.entities;
    assert.equal(more.entities.length, 4, 'the chart is a block entity');
    near(text.alpha.at(0.5), 0, 1e-9); near(text.alpha.at(2), 1, 1e-9);
    near(box.x.at(2), 0.8 * 1920, 1e-9); near(box.x.at(1), (0.5 + 0.8) / 2 * 1920, 1e-6); near(box.scale.at(2), 2, 1e-9);
    near(lock.rot.at(0.5), 45, 1e-9); near(lock.scale.at(2), 2, 1e-9);
    assert.equal(lock.layer, 'back');
    assert.deepEqual(plain(more.issues), []);
    near(more.camera.zoom.at(1), 1.5, 1e-9);
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});
```

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
    assert.match(err("export default function s(api) { api.duration(9999); }"), /between 0 and 600/);
    assert.match(err("export default function s(api) { api.tween({}, { opacity: 1 }); }"), /needs a handle/);
    assert.match(err("export default function s(api) { const t = api.text('x'); api.tween(t, { weird: 1 }); }"), /Unknown tween property "weird"/);
    assert.match(err("export default function s(api) { const t = api.text('x'); t.fadeIn({ start: 'soon' }); }"), /start must be a number/);
    assert.match(err("export default function s(api) { api.text('x', 'huge'); }"), /Unknown text style "huge"/);
    assert.match(err("export default function s(api) { api.add('env.office'); }"), /api.add takes a character or a prop/);
```

with:

```js
    assert.match(err("export default function s(api) { api.duration(9999); }"), /between 0 and 600/);
    assert.match(err("export default function s(api) { api.tween({}, { opacity: 1 }); }"), /needs a handle/);
    assert.match(err("export default function s(api) { const t = api.text('x'); api.tween(t, { weird: 1 }); }"), /Unknown tween property "weird"/);
    assert.match(err("export default function s(api) { const t = api.text('x'); api.tween(t, { grow: 1 }); }"), /Unknown tween property "grow"/);
    assert.match(err("export default function s(api) { const t = api.text('x'); t.fadeIn({ start: 'soon' }); }"), /start must be a number/);
    assert.match(err("export default function s(api) { api.text('x', 'huge'); }"), /Unknown text style "huge"/);
    assert.match(err("export default function s(api) { api.add('env.office'); }"), /api.add takes a character or a prop/);
```

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
    assert.match(fail("export default function scene(api) { api.diagram({ layout: 'flow', nodes: [{ id: 'a', label: 'A', icon: 'icon.nope' }, { id: 'b', label: 'B' }] }); }"), /icon\.nope.*storymaker_catalog/);
    assert.equal(fail(`export default function scene(api) { api.diagram({ nodes: ${nodes(3)} }); }`), '', 'the layout defaults to flow');
});
```

with:

```js
    assert.match(fail("export default function scene(api) { api.diagram({ layout: 'flow', nodes: [{ id: 'a', label: 'A', icon: 'icon.nope' }, { id: 'b', label: 'B' }] }); }"), /icon\.nope.*storymaker_catalog/);
    assert.equal(fail(`export default function scene(api) { api.diagram({ nodes: ${nodes(3)} }); }`), '', 'the layout defaults to flow');
});

check('api.chart: a counter chart is a block; bad values name the fix', () => {
    const e = build("export default function scene(api) { api.chart({ type: 'counter', labels: ['Users'], values: [42], unit: '%', title: 'Growth' }); }").entities[0];
    assert.equal(e.block.kind, 'chart'); assert.equal(e.block.chart.values[0], 42); assert.equal(e.entry, 'fade'); near(e.dur, 1.2, 1e-9);
    assert.match(fail("export default function scene(api) { api.chart({ type: 'bar', labels: ['a'], values: [1, 2] }); }"), /same length/);
    assert.match(fail("export default function scene(api) { api.chart({ type: 'donut', labels: [], values: [] }); }"), /"donut".*bar, line, pie, counter/);
    assert.match(fail("export default function scene(api) { api.chart('bar', { labels: ['a'], values: [1] }); }"), /chart \{type, labels, values\}/);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: 3 failed (chart still an inert placeholder, grow tween only noted, no chart block)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, safe, add, text, shape, tween, camera, random(seed), cue(n), onFrame(fn); chart and
// diagram exist and report not_supported_yet in phase 1. Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
// Classic script; publishes StorymakerCore.codescene.
```

with:

```js
// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, safe, add, text, shape, icon, diagram, chart, headline, list (blocks, SM.blocks), tween, camera, random(seed), cue(n), onFrame(fn).
// Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
// Classic script; publishes StorymakerCore.codescene.
```

In `ui/js/storymaker/codescene.js`, replace:

```js
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.icon = function (id, o) { o = o || {}; return addBlock({ kind: 'icon', icon: id, style: o.style, label: o.label }, o, 'center', 'draw'); };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        // api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): the layout defaults to flow.
        api.diagram = function (spec, o) {
            var d = spec && typeof spec === 'object' && !Array.isArray(spec) ? Object.assign({}, spec) : spec;
```

with:

```js
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.icon = function (id, o) { o = o || {}; return addBlock({ kind: 'icon', icon: id, style: o.style, label: o.label }, o, 'center', 'draw'); };
        api.chart = function (spec, o) { return addBlock({ kind: 'chart', chart: spec }, o, 'full', 'fade'); };
        // api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): the layout defaults to flow.
        api.diagram = function (spec, o) {
            var d = spec && typeof spec === 'object' && !Array.isArray(spec) ? Object.assign({}, spec) : spec;
```

In `ui/js/storymaker/codescene.js`, replace:

```js
            Object.keys(props).forEach(function (key) {
                var field = Object.prototype.hasOwnProperty.call(TWEEN_PROPS, key) ? TWEEN_PROPS[key] : undefined;
                if (!field) {
                    if (key === 'grow') { note('not_supported_yet', 'The tween property "grow" belongs to charts, which are not supported yet.'); return; }
                    throw new RangeError('Unknown tween property "' + key + '". Use ' + Object.keys(TWEEN_PROPS).join(', ') + '.');
                }
                var to = finite(props[key], key), track = e[field];
```

with:

```js
            Object.keys(props).forEach(function (key) {
                var field = Object.prototype.hasOwnProperty.call(TWEEN_PROPS, key) ? TWEEN_PROPS[key] : undefined;
                if (!field) {
                    throw new RangeError('Unknown tween property "' + key + '". Use ' + Object.keys(TWEEN_PROPS).join(', ') + '.');
                }
                var to = finite(props[key], key), track = e[field];
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 301 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/06-codescene.mjs scripts/storymaker-tests/35-block-api.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: api.chart; the placeholder notes go"
```

### Task E4: api.headline and api.list

**Files:**
- Modify: `scripts/storymaker-tests/35-block-api.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
    assert.match(fail("export default function scene(api) { api.chart({ type: 'donut', labels: [], values: [] }); }"), /"donut".*bar, line, pie, counter/);
    assert.match(fail("export default function scene(api) { api.chart('bar', { labels: ['a'], values: [1] }); }"), /chart \{type, labels, values\}/);
});
```

with:

```js
    assert.match(fail("export default function scene(api) { api.chart({ type: 'donut', labels: [], values: [] }); }"), /"donut".*bar, line, pie, counter/);
    assert.match(fail("export default function scene(api) { api.chart('bar', { labels: ['a'], values: [1] }); }"), /chart \{type, labels, values\}/);
});

const frameLog = (built, C, time, extra = {}) => {
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', fg: '#123456', accent: '#f00', accent2: '#0f0', text: '#000' }, cat: C.scene.indexCatalog(fullCatalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    C.codescene.drawFrame(built, time, cv.ctx, C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080));
    return cv.ctx.log;
};

check('api.headline and api.list: blocks in the full area; lists take at most 4 lines; drawing is the same at the same time', () => {
    const { C, build: run } = runScene(`export default function scene(api) {
      api.headline('Kleine Modelle sind *schnell*');
      api.list(['eins', 'zwei', 'drei'], { at: 'right', reveals: [api.cue(1), api.cue(2), 3] });
    }`);
    const built = run(), [h, l] = built.entities, full = SM.blocks.area(built.stage, null, 'day', 'full');
    assert.equal(h.block.kind, 'headline'); assert.equal(h.block.text, 'Kleine Modelle sind *schnell*'); near(h.w, full.w, 1e-9);
    assert.equal(h.entry, 'fade'); near(h.dur, 0.4 + 0.15 * 4, 1e-9);
    assert.equal(l.block.kind, 'list'); assert.equal(l.block.text, 'eins\nzwei\ndrei'); assert.equal(l.reveals.length, 3);
    assert.match(fail("export default function scene(api) { api.list(['a', 'b', 'c', 'd', 'e']); }"), /1 to 4 lines, got 5/);
    assert.match(fail("export default function scene(api) { api.list('a'); }"), /api\.list needs an array/);
    assert.match(fail("export default function scene(api) { api.headline(''); }"), /headline has 1 to 80/);
    const a = frameLog(built, C, 1.0), b = frameLog(built, C, 1.0);
    assert.ok(a.length > 20 && a.join('\n') === b.join('\n'));
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: 1 failed (api.headline is not a function)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
            if (d && d.layout === undefined) d.layout = 'flow';
            return addBlock({ kind: 'diagram', diagram: d }, o, 'full', 'draw');
        };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
```

with:

```js
            if (d && d.layout === undefined) d.layout = 'flow';
            return addBlock({ kind: 'diagram', diagram: d }, o, 'full', 'draw');
        };
        api.headline = function (text, o) { return addBlock({ kind: 'headline', text: text }, o, 'full', 'fade'); };
        // api.list(items, {at, entry, start, dur, reveals}): items is an array of 1 to 4 strings.
        api.list = function (items, o) {
            if (!Array.isArray(items)) throw new TypeError('api.list needs an array of 1 to ' + SM.blocks.LIST_MAX + ' strings, for example api.list(["first", "second"]).');
            return addBlock({ kind: 'list', text: items.map(String).join('\n') }, o, 'full', 'fade');
        };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 302 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/35-block-api.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: api.headline and api.list"
```

### Task E5: the API card lists the blocks

**Files:**
- Modify: `internal/storymaker/studio_apicard.go`
- Modify: `internal/storymaker/studio_free_packet_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_free_packet_test.go`, replace:

```go
		t.Fatalf("safe_box = %v, want %+v", packet["safe_box"], SafeBoxFor(st.Format.Aspect))
	}
	card, _ := packet["api_card"].(string)
	if !strings.Contains(card, "api.safe()") || !strings.Contains(card, "Palette of this film: $bg #") || len(card) > 1800 {
		t.Fatalf("api_card has %d bytes: %s", len(card), card)
	}
	if next, _ := packet["next"].(string); !strings.HasPrefix(next, "put_code with expected_rev = code_rev") {
```

with:

```go
		t.Fatalf("safe_box = %v, want %+v", packet["safe_box"], SafeBoxFor(st.Format.Aspect))
	}
	card, _ := packet["api_card"].(string)
	if !strings.Contains(card, "api.safe()") || !strings.Contains(card, "Palette of this film: $bg #") || len(card) > 2400 {
		t.Fatalf("api_card has %d bytes: %s", len(card), card)
	}
	if next, _ := packet["next"].(string); !strings.HasPrefix(next, "put_code with expected_rev = code_rev") {
```

In `internal/storymaker/studio_free_packet_test.go`, replace:

```go
		t.Fatalf("code_rev = %v, want the rev of the put_code answer %v", again["code_rev"], put["rev"])
	}
}
```

with:

```go
		t.Fatalf("code_rev = %v, want the rev of the put_code answer %v", again["code_rev"], put["rev"])
	}
}

func TestFreeAPICardNamesTheBlocks(t *testing.T) {
	tokens := map[string]string{"bg": "#fff7e6", "bg2": "#ffe9c2", "fg": "#1f2a44", "accent": "#ff6b35", "accent2": "#2ec4b6", "text": "#1f2a44", "outline": "#1f2a44"}
	card := FreeAPICard(tokens, SafeBox{X: 96, Y: 54, W: 1728, H: 803})
	for _, want := range []string{"api.icon(", "api.diagram(", "api.chart(", "api.headline(", "api.list(", "highlight(nodeId, timing)", "kind icon", "api.safe()", "Palette of this film: $bg #"} {
		if !strings.Contains(card, want) {
			t.Errorf("the API card does not contain %q", want)
		}
	}
	if len(card) > 2400 {
		t.Errorf("the API card has %d bytes, the bound is 2400", len(card))
	}
	t.Logf("api card bytes: %d", len(card))
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeAPICardNamesTheBlocks|TestFreeFillPacketCarriesCodeRevCardAndSafeBox)' -count=1`
Expected: TestFreeAPICardNamesTheBlocks failed (no block lines; the exact insert of the plan put the card at 2920 bytes, so the base lines were tightened)

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_apicard.go`, replace:

```go
	"strings"
)

// freeAPICardText is the code API in short, for the context of a free fill. It must stay within 1800 bytes with the two lines
// FreeAPICard adds; the code reference holds the long form.
const freeAPICardText = `API card for this code scene. Read storymaker_catalog reference topic code only when this card is not enough.
export default function scene(api) { ... } runs once and builds the scene; each frame is a pure function of t.
timing = {start, dur, ease}; ease is linear, ease-in, ease-out, ease-in-out, back-out or bounce-out. api.cue(n) is the second of narration marker {n}.
api.duration(seconds)
api.safe() returns {x, y, w, h}: the drawing area of this format in 1920x1080 design pixels, above the captions.
api.text(text, 'title' or 'label', {at}) returns a handle.
api.shape('rect', 'circle' or 'line', {at, w, h, fill, stroke}) returns a handle; w and h are design pixels.
handle.moveTo(at, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale, timing), rotateTo(degrees, timing)
api.tween(handle, {opacity, x, y, scale, rotation, width, height}, timing)
api.camera.zoom(factor, timing), api.camera.pan(at, timing), api.camera.shake(amount, timing)
api.random('seed') returns a seeded random function: call it while building, never in onFrame.
api.onFrame(function (t, g) { ... }): g.save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path(svgPath, 'fill', 'stroke' or 'both'), text(text, x, y, {size, weight, align, color}). g draws in design pixels.
at is {x, y} in percent of the safe area, or a spot: left, center, right.
Colours: the palette tokens $bg, $bg2, $fg, $accent, $accent2, $text, $outline, or #rrggbb.
Never: api.add, Math.random, Date, timers, imports, the DOM, or state that changes from frame to frame.`

// FreeAPICard is the card of a free fill packet: freeAPICardText, a palette line and a safe_box line, joined by "\n". The
```

with:

```go
	"strings"
)

// freeAPICardText is the code API in short, for the context of a free fill. It must stay within 2400 bytes with the two lines
// FreeAPICard adds; the code reference holds the long form.
const freeAPICardText = `API card. Read storymaker_catalog reference topic code only when this card is not enough.
export default function scene(api) { ... } builds the scene once; each frame is a pure function of t.
timing = {start, dur, ease}; ease is linear, ease-in, ease-out, ease-in-out or back-out. api.cue(n) is the second of narration marker {n}.
api.duration(seconds)
api.safe() returns {x, y, w, h}: the drawing area in 1920x1080 design pixels, above the captions.
api.text(text, 'title' or 'label', {at}) and api.shape('rect', 'circle' or 'line', {at, w, h, fill, stroke}) return handles; w and h are design pixels.
handle.moveTo(at, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale, timing), rotateTo(degrees, timing)
api.tween(handle, {opacity, x, y, scale, rotation, width, height}, timing)
api.camera.zoom(factor, timing), api.camera.pan(at, timing), api.camera.shake(amount, timing)
api.random('seed') returns a seeded random function: call it while building, never in onFrame.
api.onFrame(function (t, g) { ... }): g.save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path(svgPath, 'fill', 'stroke' or 'both'), text(text, x, y, {size, weight, align, color}).
at is left, center, right or {x, y} in percent of the safe area; blocks also take full, top, board.
Blocks return a handle plus highlight(nodeId, timing). Options {at, entry, start, dur}; entry is draw, pop, fade or none.
api.icon(id, {style, label}): style is line, badge or card; find ids with storymaker_catalog operation search, kind icon, one English word.
api.diagram({layout, nodes, flow}, {reveals}): layout is flow (2-5 nodes), cycle (3-6), hub (centre + 2-6), stack (2-5) or compare (2); node {id, label, icon, points (compare, up to 3)}; flow: true animates the arrows.
api.chart({type, labels, values, unit, title}): type is bar (up to 8), line (2-24), pie (2-6) or counter (1 value).
api.headline('One *marked* word') and api.list(['line', ...]) with up to 4 lines; reveals: one start second per node or line, e.g. api.cue(2).
Colours: $bg, $bg2, $fg, $accent, $accent2, $text, $outline or #rrggbb.
Never: api.add, Math.random, Date, timers, imports, the DOM, or state that changes from frame to frame.`

// FreeAPICard is the card of a free fill packet: freeAPICardText, a palette line and a safe_box line, joined by "\n". The
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeAPICardNamesTheBlocks|TestFreeFillPacketCarriesCodeRevCardAndSafeBox)' -count=1`
Expected: both PASS, card 2382 bytes with a 7-token palette line and the safe_box line (bound 2400)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_apicard.go internal/storymaker/studio_free_packet_test.go
git commit -m "storymaker: the API card lists the blocks"
```

### Task E6: the references teach the blocks; the free example uses them

**Files:**
- Modify: `internal/storymaker/reference/code-free.md`
- Modify: `internal/storymaker/reference/code.md`
- Modify: `internal/storymaker/reference/free-motion.js`
- Modify: `internal/storymaker/studio_free_reference_test.go`
- Modify: `scripts/storymaker-tests/35-block-api.mjs`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_free_reference_test.go`, replace:

```go
		if !strings.Contains(text, "api.safe()") {
			t.Errorf("%s does not name api.safe()", name)
		}
	}
}
```

with:

```go
		if !strings.Contains(text, "api.safe()") {
			t.Errorf("%s does not name api.safe()", name)
		}
	}
}

func TestFreeReferenceTeachesTheBlocks(t *testing.T) {
	for name, text := range map[string]string{"code.md": codeReference, "code-free.md": codeReferenceFree} {
		if !strings.Contains(text, "api.diagram") {
			t.Errorf("%s does not teach api.diagram", name)
		}
		if strings.Contains(text, "placeholders") {
			t.Errorf("%s still calls the blocks placeholders", name)
		}
		for _, want := range []string{"## Explainer blocks", "api.icon(", "api.chart(", "api.headline(", "api.list(", "highlight(nodeId, timing)"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", name, want)
			}
		}
	}
	if !strings.Contains(freeMotionExample, "api.diagram(") || !strings.Contains(freeMotionExample, "api.headline(") || !strings.Contains(freeMotionExample, "api.safe()") {
		t.Error("free-motion.js does not use api.headline, api.diagram and api.safe()")
	}
	if lines := strings.Count(freeMotionExample, "\n"); lines >= 40 {
		t.Errorf("free-motion.js has %d lines, want fewer than 40", lines)
	}
	if strings.Count(freeMotionExample, "api.onFrame(") != 1 {
		t.Error("free-motion.js has not exactly one onFrame accent")
	}
}
```

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
// Batch E: the blocks API of a code scene (api.icon, api.diagram, api.chart, api.headline, api.list) and the board tokens.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { plain, FakeCanvas, loadPlayer } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene } = t;
```

with:

```js
// Batch E: the blocks API of a code scene (api.icon, api.diagram, api.chart, api.headline, api.list) and the board tokens.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { root, plain, FakeCanvas, loadPlayer } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene } = t;
```

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
    const a = frameLog(built, C, 1.0), b = frameLog(built, C, 1.0);
    assert.ok(a.length > 20 && a.join('\n') === b.join('\n'));
});
```

with:

```js
    const a = frameLog(built, C, 1.0), b = frameLog(built, C, 1.0);
    assert.ok(a.length > 20 && a.join('\n') === b.join('\n'));
});

check('free-motion.js: the reference example builds in every aspect, its blocks do not overlap and the frames repeat', () => {
    const source = fs.readFileSync(path.join(root, 'internal/storymaker/reference/free-motion.js'), 'utf8');
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const { C, build: run } = runScene(source, { aspect });
        const built = run();
        assert.equal(built.duration, 8);
        assert.deepEqual(plain(built.issues), []);
        assert.deepEqual(plain(built.entities.map(e => e.block.kind)), ['headline', 'diagram']);
        const rc = C.render.makeContext({ aspect, fps: 30, palette: { bg: '#fff', bg2: '#eee', fg: '#123456', accent: '#f00', accent2: '#0f0', text: '#000' }, cat: C.scene.indexCatalog(fullCatalog), fontChain: null, fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe }, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
        const [h, d] = built.entities.map(e => C.blocks.measure(new FakeCanvas(1920, 1080).ctx, C.scene.evalEntity(e, 7), rc, built.stage, built.stage.baseWindow()).box);
        assert.ok(h.y + h.h <= d.y + 1e-6 || d.y + d.h <= h.y, aspect + ': headline ' + JSON.stringify(h) + ' overlaps the diagram ' + JSON.stringify(d));
        for (const time of [0.5, 3, 7]) assert.equal(frameLog(built, C, time).join('\n'), frameLog(run(), C, time).join('\n'));
    }
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeReferenceTeachesTheBlocks|TestFree.*Reference|TestFreeMotionExample)' -count=1; npm run test:storymaker`
Expected: TestFreeReferenceTeachesTheBlocks failed (placeholder text, no api.diagram, 53-line example)

- [ ] **Step 3: Implement**

In `internal/storymaker/reference/code-free.md`, replace:

```markdown

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

api.chart and api.diagram are placeholders and do not render. Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text and the drawing facade for every effect.

## Draw every motif yourself with g.path and shapes
```

with:

```markdown

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text and the drawing facade for every effect.

## Explainer blocks

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board of the scene's environment, or the right half of the frame when it has none.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.

Calls:

- api.icon(id, {at, style, label, entry, start, dur}): id is an exact icon id such as icon.cpu; find it with storymaker_catalog operation search, kind icon and one English word. style is line, badge or card; the label shows on the card style.
- api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): layout is flow (2-5 nodes), cycle (3-6), hub (the first node in the centre, 2-6 around it), stack (2-5, first node at the bottom) or compare (2 columns). A node is {id, label, icon, points}: id is unique and made of lowercase letters, digits and dashes; label has at most 40 characters, keep it to 3 words; icon is optional; points (compare only) has up to 3 short lines. flow: true moves dots along the arrows. reveals holds one start second per node, for example [api.cue(1), api.cue(2)]; without it the nodes appear one after another over dur. highlight(nodeId, {start, dur}) pulses one node for 1.2 s.
- api.chart({type, labels, values, unit, title}, {at, entry, start, dur}): type is bar (1-8 values), line (2-24), pie (2-6, no negative values) or counter (exactly 1 value). labels and values have the same length; a label has at most 24 characters, unit 8, title 40.
- api.headline('One *marked* word', {at, entry, start, dur}): at most 80 characters; the word between asterisks is accented.
- api.list(['line', ...], {at, entry, start, dur, reveals}): 1-4 lines of at most 40 characters; reveals holds one start second per line.

Put a headline in top and a diagram or chart below it instead of one crowded block, and keep onFrame drawing for small accents on top of the blocks.

## Draw every motif yourself with g.path and shapes
```

In `internal/storymaker/reference/code.md`, replace:

```markdown

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

api.chart and api.diagram are placeholders and do not render. Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text, catalog props and the drawing facade for custom effects.

## Create a missing SVG motif
```

with:

```markdown

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text, catalog props and the drawing facade for custom effects.

## Explainer blocks

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board of the scene's environment, or the right half of the frame when it has none.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.

Calls:

- api.icon(id, {at, style, label, entry, start, dur}): id is an exact icon id such as icon.cpu; find it with storymaker_catalog operation search, kind icon and one English word. style is line, badge or card; the label shows on the card style.
- api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): layout is flow (2-5 nodes), cycle (3-6), hub (the first node in the centre, 2-6 around it), stack (2-5, first node at the bottom) or compare (2 columns). A node is {id, label, icon, points}: id is unique and made of lowercase letters, digits and dashes; label has at most 40 characters, keep it to 3 words; icon is optional; points (compare only) has up to 3 short lines. flow: true moves dots along the arrows. reveals holds one start second per node, for example [api.cue(1), api.cue(2)]; without it the nodes appear one after another over dur. highlight(nodeId, {start, dur}) pulses one node for 1.2 s.
- api.chart({type, labels, values, unit, title}, {at, entry, start, dur}): type is bar (1-8 values), line (2-24), pie (2-6, no negative values) or counter (exactly 1 value). labels and values have the same length; a label has at most 24 characters, unit 8, title 40.
- api.headline('One *marked* word', {at, entry, start, dur}): at most 80 characters; the word between asterisks is accented.
- api.list(['line', ...], {at, entry, start, dur, reveals}): 1-4 lines of at most 40 characters; reveals holds one start second per line.

Put a headline in top and a diagram or chart below it instead of one crowded block, and keep onFrame drawing for small accents on top of the blocks.

## Create a missing SVG motif
```

In `internal/storymaker/reference/free-motion.js`, replace:

```js
export default function scene(api) {
    api.duration(8);
    // Cue {1} marks the narrated word where the shield appears; without that marker use a fixed time.
    function cueOr(n, fallback) {
        try {
            return api.cue(n);
```

with:

```js
export default function scene(api) {
    api.duration(8);
    const box = api.safe();
    // Cues {1} and {2} mark the narrated words where the check starts and ends; without markers use fixed times.
    function cueOr(n, fallback) {
        try {
            return api.cue(n);
```

In `internal/storymaker/reference/free-motion.js`, replace:

```js
            return fallback;
        }
    }
    const shown = cueOr(1, 1.5);
    const title = api.text('Jedes Paket wird geprüft', 'label', { at: { x: 50, y: 3 } });
    const rule = api.shape('rect', { at: { x: 50, y: 11 }, w: 40, h: 6, fill: '#d49b36' });
    api.tween(title, { opacity: 1 }, { start: 0, dur: 0.6, ease: 'ease-out', from: { opacity: 0 } });
    api.tween(rule, { width: 420 }, { start: 0.3, dur: 0.8, ease: 'ease-out' });
    api.tween(rule, { opacity: 0.35 }, { start: shown + 1, dur: 1, ease: 'ease-in-out' });
    const shield = 'M0-120L96-84V-6Q96 70 0 122Q-96 70-96-6V-84Z';
    const tick = 'M-38 4L-10 32L42-28';
    const lanes = [-56, 0, 56];
    api.onFrame(function (t, g) {
        const appear = Math.min(1, Math.max(0, (t - shown) / 0.5));
        g.save();
        g.translate(960, 460);
        g.save();
        g.translate(0, 8);
        g.alpha(appear * 0.16);
        g.fill('#183f39');
        g.path(shield);
        g.restore();
        g.alpha(appear);
        g.fill('#eee5cf');
        g.stroke('#4b7067', 6);
        g.path(shield, 'both');
        g.stroke('#28594e', 10);
        g.path(tick, 'stroke');
        g.restore();
        let checked = 0;
        g.save();
        for (let i = 0; i < 3; i++) {
            const u = Math.min(1, Math.max(0, (t - shown - 0.5 - i * 0.7) / 2.4));
            if (u <= 0) continue;
            const x = 740 + u * 440;
            if (x > 960) checked++;
            g.fill(x > 960 ? '#2f8f6b' : '#d49b36');
            g.circle(x, 460 + lanes[i], 12);
        }
        g.restore();
        g.save();
        g.alpha(appear);
        g.text(checked + ' / 3', 960, 700, { size: 30, color: '#28594e' });
        g.restore();
    });
}
```

with:

```js
            return fallback;
        }
    }
    const check = cueOr(1, 2.8);
    const done = cueOr(2, 5);
    api.headline('Jedes Paket wird *geprüft*', { at: 'top', start: 0.2 });
    const flow = api.diagram({
        layout: 'flow',
        flow: true,
        nodes: [
            { id: 'packet', label: 'Paket', icon: 'icon.mail' },
            { id: 'firewall', label: 'Firewall', icon: 'icon.lock' },
            { id: 'network', label: 'Netzwerk', icon: 'icon.server' }
        ]
    }, { at: { x: 50, y: 62 }, reveals: [1, check, done] });
    flow.highlight('firewall', { start: check + 0.4, dur: 1.6 });
    // One small accent: a hairline at the foot of the safe area that fills over the whole scene.
    api.onFrame(function (t, g) {
        const u = Math.min(1, Math.max(0, t / 8));
        g.save();
        g.alpha(0.35);
        g.fill('$text');
        g.rect(box.x, box.y + box.h - 6, box.w, 2);
        g.alpha(1);
        g.fill('$accent');
        g.rect(box.x, box.y + box.h - 8, box.w * u, 6);
        g.restore();
    });
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeReferenceTeachesTheBlocks|TestFree.*Reference|TestFreeMotionExample)' -count=1; npm run test:storymaker`
Expected: Go PASS (5 tests); player tests 303 passed, 0 failed (new check runs free-motion.js in 3 aspects)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/reference/code-free.md internal/storymaker/reference/code.md internal/storymaker/reference/free-motion.js internal/storymaker/studio_free_reference_test.go scripts/storymaker-tests/35-block-api.mjs
git commit -m "storymaker: the references teach the blocks; the free example uses them"
```

### Task E7: code scenes on a board read the board's colour tokens

**Files:**
- Modify: `scripts/storymaker-tests/35-block-api.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/render.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/35-block-api.mjs`, replace:

```js
        for (const time of [0.5, 3, 7]) assert.equal(frameLog(built, C, time).join('\n'), frameLog(run(), C, time).join('\n'));
    }
});
```

with:

```js
        for (const time of [0.5, 3, 7]) assert.equal(frameLog(built, C, time).join('\n'), frameLog(run(), C, time).join('\n'));
    }
});

check('code scenes on env.board resolve $text and $bg from the board tokens, then from the palette', () => {
    const source = "export default function scene(api) { api.duration(2); api.shape('rect', { at: 'center', w: 20, h: 20, fill: '$bg' }); api.onFrame((t, g) => { g.fill('$text'); g.rect(1, 1, 2, 2); g.text('x', 5, 5, { color: '$text' }); g.fill('$accent'); g.rect(3, 3, 2, 2); }); }";
    const fills = (environment, variant) => {
        const story = { ...firewall, scenes: [{ id: 's', kind: 'code', environment, variant }] };
        const { C, build: run } = runScene(source, { story, id: 's' });
        return frameLog(run(), C, 0.5).filter(l => l.startsWith('set fillStyle='));
    };
    const night = fills('env.board', 'night'), day = fills('env.board', 'day'), other = fills('env.office', 'night');
    assert.ok(night.includes('set fillStyle=#f4f5eb'), 'night $text: ' + night.join());
    assert.ok(night.includes('set fillStyle=#1f2a44'), 'night $bg of the shape: ' + night.join());
    assert.ok(night.includes('set fillStyle=#f00'), 'a token the board does not carry comes from the palette');
    assert.ok(day.includes('set fillStyle=#1f2a44') && !day.includes('set fillStyle=#f4f5eb'), 'day $text: ' + day.join());
    assert.ok(day.includes('set fillStyle=#fff'), 'day $bg stays the palette bg: ' + day.join());
    assert.ok(other.includes('set fillStyle=#000') && !other.includes('set fillStyle=#f4f5eb'), 'without env.board $text is the palette text: ' + other.join());
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: 1 failed (night $text resolved to the palette text #000 on env.board)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: built.env || null, variant: built.variant || 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
```

with:

```js
    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: built.env || null, variant: built.variant || 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        // A scene on a board reads the tokens of the board (text, bg, bg2) before the palette: $text is light on the night board.
        var tokens = SM.render.envTokens(sceneLike, rc);
        if (tokens) rc = Object.assign({}, rc, { palette: Object.assign({}, rc.palette, tokens) });
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
```

In `ui/js/storymaker/render.js`, replace:

```js

    SM.render = {
        renderFrame: renderFrame, drawScene: drawScene, rigState: rigState, makeContext: makeContext, familyOf: familyOf,
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf,
        entityBox: entityBox, textBox: textBox, captionBox: captionBox
        , drawShapeEntity: drawShapeEntity, drawBubble: drawBubble, drawHighlight: drawHighlight
    };
```

with:

```js

    SM.render = {
        renderFrame: renderFrame, drawScene: drawScene, rigState: rigState, makeContext: makeContext, familyOf: familyOf,
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf, envTokens: envTokens,
        entityBox: entityBox, textBox: textBox, captionBox: captionBox
        , drawShapeEntity: drawShapeEntity, drawBubble: drawBubble, drawHighlight: drawHighlight
    };
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 304 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/35-block-api.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js ui/js/storymaker/render.js
git commit -m "storymaker: code scenes on a board read the board's colour tokens"
```

### Task E8: the references say where a board area falls back

**Files:**
- Modify: `internal/storymaker/reference/code-free.md`
- Modify: `internal/storymaker/reference/code.md`
- Modify: `internal/storymaker/studio_free_reference_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_free_reference_test.go`, replace:

```go
		if strings.Contains(text, "placeholders") {
			t.Errorf("%s still calls the blocks placeholders", name)
		}
		for _, want := range []string{"## Explainer blocks", "api.icon(", "api.chart(", "api.headline(", "api.list(", "highlight(nodeId, timing)"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", name, want)
			}
```

with:

```go
		if strings.Contains(text, "placeholders") {
			t.Errorf("%s still calls the blocks placeholders", name)
		}
		for _, want := range []string{"## Explainer blocks", "api.icon(", "api.chart(", "api.headline(", "api.list(", "highlight(nodeId, timing)", "on env.board it is the full frame", "to the top in 9:16 and 1:1", "only env.board draws a board"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", name, want)
			}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeReferenceTeachesTheBlocks)' -count=1`
Expected: code.md and code-free.md lack "on env.board it is the full frame", "to the top in 9:16 and 1:1", "only env.board draws a board"

- [ ] **Step 3: Implement**

In `internal/storymaker/reference/code-free.md`, replace:

```markdown

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board of the scene's environment, or the right half of the frame when it has none.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.
```

with:

```markdown

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board area of the environment; on env.board it is the full frame. Elsewhere it falls back to the right half of the safe area in 16:9 and to the top in 9:16 and 1:1. In code scenes only env.board draws a board.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.
```

In `internal/storymaker/reference/code.md`, replace:

```markdown

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board of the scene's environment, or the right half of the frame when it has none.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.
```

with:

```markdown

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board area of the environment; on env.board it is the full frame. Elsewhere it falls back to the right half of the safe area in 16:9 and to the top in 9:16 and 1:1. In code scenes only env.board draws a board.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeReferenceTeachesTheBlocks)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/reference/code-free.md internal/storymaker/reference/code.md internal/storymaker/studio_free_reference_test.go
git commit -m "storymaker: the references say where a board area falls back"
```

