[← Plan index](README.md) · Spec: [plans/2026-11-10-easydrag-s0-spec.md](../2026-11-10-easydrag-s0-spec.md)

## Part 1: Render counters and the runs.js split

**Scope.** Spec §3 (S0.0) and the runs.js part of spec §4 (S0.1). Part 1 has five tasks and five commits, in this order: 1.1 (baseline), 1.2 (stream), 1.3 (start), 1.4 (stop), 1.5 (drawer).

**Root and shell.**
- All paths are relative to the worktree root `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0` (branch `feat/easydrag-s0`).
- Every command block runs in Git Bash (the Bash tool) and starts with a `cd`, because the cwd resets between calls.
- Line numbers are those of main, checked against the main checkout on 2026-11-11. They match the anchors in the brief.

**Scratch folder.** Scratch scripts go in `C:/disposable/easydrag-s0/` (the user's disposable folder). Create them with the Write tool, never with heredocs.

**Commands used in every task.**
- NODE: `node scripts/test-easydrag.mjs`. This is the whole EasyDrag Node suite: extra–extra7, durable, b5, and the baseline from 1.1 on. It exits 1 on a failure.
- FILTERED: `AURAGO_EASYDRAG_TEST_FILTER='<substring>' node scripts/test-easydrag.mjs`. `guardAsync` skips every guard whose name lacks the substring.
- BASELINE: `node scripts/test-easydrag-render-baseline.mjs`. Expected output: `PASS render baseline (unchanged)`.
- GO_STATIC: `go test ./ui/ -run '(?i)(EasyDrag)' -count=1`. Browser tests skip without the env var.
- BROWSER: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m`. The gate is `requirePrecisionBrowserSmoke` in `ui/precision_workspace_browser_test.go:13-18`. It runs once, in Task 1.5.
- IMPACT: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<fn>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`, run from `C:/Users/Andi/Documents/repo/AuraGo`.
  - For an ambiguous name (`render`, `create`, `run`, `cancel`, `finish`, `connect`, `probe`), take the ranked candidate in the named file.
  - Treat `UNKNOWN` as unresolved and confirm with the Grep given in the task.

**PLAN-CHECK: hard-coded script lists in tests.**
- Node harnesses that load `easydrag-runs.js` by name:
  - `scripts/test-easydrag-extra2.mjs:9-10` (`MODULES`)
  - `scripts/test-easydrag-extra3.mjs:9-11` (`MODULES`)
  - `scripts/test-easydrag-extra4.mjs:12` (`UI_MODULES`)
- Content check on runs.js: `scripts/test-easydrag-extra5.mjs:622` reads `easydrag-runs.js` for the keys `'aurago.easydrag.effects-ok.'` and `'aurago.easydrag.test-trigger.'`.
- No browser test (`ui/desktop_easydrag_*browser_test.go`) names an EasyDrag script. They load through module-loader.js.

**Split design (applies to 1.2–1.5).**
- `create(ed, canvas)` in runs.js builds a run-core object `rc`. It holds:
  - `ed, canvas, core, t, esc`;
  - the module helpers (`isFinal, isActive, edgeStates, triggers, realEffects, effects, unknownEffects`);
  - the module limits (`RETRY_MS, RETRY_MAX_MS, MAX_FAILURES, STOP_POLL_MS, PARK_POLL_MS, EFFECTS_KEY, REDACTED, EFFECT_ICONS`);
  - `stopButtons`;
  - every former closure `let`. These are the lets at :50-64, `failedSpoken` :183, `starting` :419, `startPost` :422, `viewRefreshSeq` :870, `listedRuns` and `runsOffset/runsMore/runsLoading/runsQuery` :914-915.
  - It also holds the Sets `stopping/durableStopping/stopPolls` :684-687. Stop and drawer share them: `drawRuns` and `stepsFrom` read them.
- Moved code reads and writes state only as `rc.<name>`. This is a mechanical rewrite in 1.2, with a self-check.
- Each part file registers an install function on `ED.runParts`:
  - `ED.runParts.stream = installStream`, and `start`, `stop`, `drawer` the same way.
  - An install function destructures the core helpers it uses.
  - It declares late-binding wrappers, `const x = (...args) => rc.x(...args);`, for functions of other parts.
  - It returns all its functions. `create()` merges them into rc with `Object.assign(rc, ED.runParts.<part>(rc))`.
- The moved code itself is byte-identical. A scratch mover cuts it, so there is no hand transcription.
- runs.js keeps the module helpers and constants, the core, the bag wiring and the unchanged facade `ED.runs = { create, effects, unknownEffects, triggers, edgeStates, isFinal, isActive }`. The object returned by `create()` keeps its keys and their order.
- Create-time side effects keep their original order. `runReads` is built at the stream install, `runFinished` at the stop install, and the bag lines come last.

---

### Task 1.1: Render counters and the seeded S/M/XL baseline (S0.0)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-canvas.js:54-56` (renderStats init), `:196` (`allRects`), `:656-657` (`drawMinimap`).
- Modify: `ui/js/desktop/apps/easydrag-wires.js:117-118` (`renderFor`).
- Modify: `ui/js/desktop/apps/easydrag-groups.js:63-64` (`render`).
- Modify: `ui/js/desktop/apps/easydrag-interact.js:119-120` (`edgeNear`), `:274-275` (`alignTo`).
- Modify: `scripts/test-easydrag-ux-batch5.mjs:832-833` (the F6 guard asserts the new keys).
- Modify: `scripts/test-easydrag.mjs:16` (header comment) and `:1138` (register the baseline after b5).
- Create: `scripts/test-easydrag-render-baseline.mjs`.
- Create (generated): `scripts/easydrag-render-baseline.json`.

- [ ] **Step 1: GitNexus impact (the six hot paths).**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  for fn in allRects drawMinimap renderFor render edgeNear alignTo; do echo "== $fn"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$fn" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
  ```
  - For `render`, use the candidate in `ui/js/desktop/apps/easydrag-groups.js`.
  - Report callers, processes and risk. Warn the controller on HIGH or CRITICAL before editing. The edits only add a counter.
  - On `UNKNOWN`, Grep `\b(allRects|drawMinimap|renderFor|edgeNear|alignTo)\b` in `ui/js/desktop/apps`. Expected: only canvas.js, wires.js and interact.js.

- [ ] **Step 2: Failing test. The F6 guard asserts the new keys.**
  In `scripts/test-easydrag-ux-batch5.mjs`, insert this line between `            [[0, 1000, true], 298, true, true, []]);` (:832) and `        s.editor.dispose();` (:833):
  ```js
          eq('b5 F6 renderStats carries the S0.0 counters', Object.keys(stats).sort(), ['align', 'allRects', 'cards', 'edgeNear', 'groups', 'minimap', 'paints', 'wireScans', 'wires']);
  ```
  Run:
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  AURAGO_EASYDRAG_TEST_FILTER='b5 F6' node scripts/test-easydrag.mjs
  ```
  Expected:
  - `FAIL b5 F6 renderStats carries the S0.0 counters — got ["cards","paints","wires"] want [...]`
  - then `1 failure(s)` and exit 1.

- [ ] **Step 3: Add the counters (one `++` at function entry each).**
  `ui/js/desktop/apps/easydrag-canvas.js`, replace :54-56:
  ```js
          // renderStats counts card renders (renderNode), state paints (paintState) and wire draws
          // (wires.js); the b5 F6 check reads them to guard the cost of run events.
          if (!ed.renderStats) ed.renderStats = { cards: 0, paints: 0, wires: 0 };
  ```
  with:
  ```js
          // renderStats counts card renders (renderNode), state paints (paintState) and wire draws
          // (wires.js); the b5 F6 check reads them to guard the cost of run events. The S0.0 counters
          // count calls of the other hot paths: wireScans (wires.js renderFor), groups (groups.js
          // render), allRects, minimap (drawMinimap), edgeNear and align (interact.js alignTo).
          // scripts/test-easydrag-render-baseline.mjs pins them; each is one ++ and does nothing else.
          if (!ed.renderStats) ed.renderStats = { cards: 0, paints: 0, wires: 0, wireScans: 0, groups: 0, allRects: 0, minimap: 0, edgeNear: 0, align: 0 };
  ```
  Replace :196:
  ```js
          function allRects() { return ed.groups ? ed.groups.rects() : ed.model.doc.nodes.map(n => G.nodeRect(n.position, outCount(n))); }
  ```
  with:
  ```js
          function allRects() { ed.renderStats.allRects++; return ed.groups ? ed.groups.rects() : ed.model.doc.nodes.map(n => G.nodeRect(n.position, outCount(n))); }
  ```
  Replace :656-657:
  ```js
          function drawMinimap() {
              const rects = allRects();
  ```
  with:
  ```js
          function drawMinimap() {
              ed.renderStats.minimap++;
              const rects = allRects();
  ```
  `ui/js/desktop/apps/easydrag-wires.js`, replace :117-118:
  ```js
          function renderFor(nodeIds) {
              const set = new Set(nodeIds || []);
  ```
  with:
  ```js
          function renderFor(nodeIds) {
              if (ed.renderStats) ed.renderStats.wireScans++;
              const set = new Set(nodeIds || []);
  ```
  `ui/js/desktop/apps/easydrag-groups.js`, replace :63-64:
  ```js
          function render() {
              rebuild(); const present = new Set(groups.map(item => item.id));
  ```
  with:
  ```js
          function render() {
              if (ed.renderStats) ed.renderStats.groups++;
              rebuild(); const present = new Set(groups.map(item => item.id));
  ```
  `ui/js/desktop/apps/easydrag-interact.js`, replace :119-120:
  ```js
          function edgeNear(worldPoint, exceptNode) {
              const byId = new Map(ed.model.doc.nodes.map(n => [n.id, n]));
  ```
  with:
  ```js
          function edgeNear(worldPoint, exceptNode) {
              if (ed.renderStats) ed.renderStats.edgeNear++;
              const byId = new Map(ed.model.doc.nodes.map(n => [n.id, n]));
  ```
  and :274-275:
  ```js
          function alignTo(id, pos, moving) {
              const skip = new Set(moving);
  ```
  with:
  ```js
          function alignTo(id, pos, moving) {
              if (ed.renderStats) ed.renderStats.align++;
              const skip = new Set(moving);
  ```
  - canvas.js always creates `ed.renderStats`, so its counters stay unguarded, like :443 and :517.
  - wires, groups and interact can run without a canvas in some tests, so they keep the `if (ed.renderStats)` guard that wires.js:67 uses.

  Run the Step 2 command again. Expected: `ok   b5 F6 renderStats carries the S0.0 counters`, and both F6 guards pass with exit 0.

- [ ] **Step 4: Create `scripts/test-easydrag-render-baseline.mjs`.**
  Why it works this way:
  - `types` and `miniDom` live only inside `test-easydrag.mjs`. So a direct run starts that runner as a child process, with the guard filter set to `render baseline`.
  - The runner imports this module and calls `run(env)` with the c1d07 sandbox that b5 uses.
  - In the child, `argv[1]` is `test-easydrag.mjs`, so the module does not spawn again. Two modules importing each other would deadlock on top-level await; the child process avoids that.
  ```js
  #!/usr/bin/env node
  // S0.0 render baseline (plans/2026-11-10-easydrag-s0-spec.md section 3). Seeded flows S (8 cards,
  // 7 wires), M (60, 70, 4 groups) and XL (300, 360, 20 groups) each run three scenarios on the c1d07
  // sandbox that the b5 checks use: drag one card for 20 frames, one step-done run event on a step in
  // the middle of the flow, and one card rename. ed.renderStats counts the render work of each
  // scenario; the counts must equal scripts/easydrag-render-baseline.json exactly. A lower count is
  // committed only together with a baseline update whose commit message says why.
  //
  // Run: node scripts/test-easydrag-render-baseline.mjs            compare with the baseline
  //      node scripts/test-easydrag-render-baseline.mjs --update   write the baseline
  // A direct run starts test-easydrag.mjs with AURAGO_EASYDRAG_TEST_FILTER=render baseline (that runner
  // builds the sandbox and calls run(env) below); every full test-easydrag.mjs run compares as well.
  import fs from 'node:fs';
  import path from 'node:path';
  import { spawnSync } from 'node:child_process';
  import { fileURLToPath } from 'node:url';

  const here = path.dirname(fileURLToPath(import.meta.url));
  const BASELINE = path.join(here, 'easydrag-render-baseline.json');
  const KEYS = ['cards', 'paints', 'wires', 'wireScans', 'groups', 'allRects', 'minimap', 'edgeNear', 'align'];
  const FIXTURES = [
      { name: 'S', nodes: 8, edges: 7, groups: 0, seed: 0x5e0d5001 },
      { name: 'M', nodes: 60, edges: 70, groups: 4, seed: 0x5e0d5002 },
      { name: 'XL', nodes: 300, edges: 360, groups: 20, seed: 0x5e0d5003 }
  ];
  const GROUP_COLORS = ['neutral', 'blue', 'green', 'amber', 'rose', 'violet'];
  const BASE32 = 'abcdefghijklmnopqrstuvwxyz234567';

  // mulberry32 is a small seeded PRNG: the same seed gives the same fixture on every machine.
  function mulberry32(seed) {
      let a = seed >>> 0;
      return () => {
          a = (a + 0x6D2B79F5) >>> 0;
          let x = a;
          x = Math.imul(x ^ (x >>> 15), x | 1);
          x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
          return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
      };
  }

  // fixtureDoc builds a flow of spec.nodes cards on a jittered grid: a manual trigger, web searches
  // wired as a random tree plus random extra wires (always from an earlier to a later card, never the
  // same pair twice), and one last card without wires: the drag scenario moves it, so the
  // drop-on-wire search (edgeNear) runs. Groups take disjoint runs of wired cards.
  function fixtureDoc(spec) {
      const rand = mulberry32(spec.seed);
      const pick = n => Math.floor(rand() * n);
      const id = i => 'n_f' + String(i).padStart(7, '0');
      const nodes = [];
      for (let i = 0; i < spec.nodes; i++) {
          const position = { x: (i % 12) * 300 + pick(40), y: Math.floor(i / 12) * 160 + pick(40) };
          nodes.push(i === 0
              ? { id: id(0), key: 'start', type: 'trigger.manual', label: 'Start', position, params: {}, settings: {} }
              : { id: id(i), key: 'step_' + i, type: 'web.search', label: 'Step ' + i, position, params: { query: 'q' + i }, settings: {} });
      }
      const wired = spec.nodes - 1;
      const edges = [];
      const pairs = new Set();
      const wire = (a, b) => {
          pairs.add(a + '>' + b);
          edges.push({ id: 'e_f' + String(edges.length + 1).padStart(5, '0'), source: { node: id(a), port: 'out' }, target: { node: id(b), port: 'in' } });
      };
      for (let b = 1; b < wired; b++) wire(pick(b), b);
      for (let tries = 0; edges.length < spec.edges && tries < 100000; tries++) {
          const a = pick(wired - 1);
          const b = a + 1 + pick(wired - 1 - a);
          if (!pairs.has(a + '>' + b)) wire(a, b);
      }
      const groups = [];
      const span = Math.floor((wired - 1) / Math.max(spec.groups, 1));
      for (let g = 0; g < spec.groups; g++) {
          const first = 1 + g * span;
          const size = 3 + pick(4);
          groups.push({ id: 'g_aaaaaaa' + BASE32[g], title: 'Group ' + (g + 1), color: GROUP_COLORS[g % GROUP_COLORS.length], members: Array.from({ length: size }, (_, k) => id(first + k)) });
      }
      const doc = { schema: 1, name: 'Baseline ' + spec.name, nodes, edges };
      if (groups.length) doc.groups = groups;
      return doc;
  }

  export async function run(env) {
      const { eq, check, guardAsync, settle, sandbox, openEditor } = env;
      // frames runs the queued animation frames; a frame may queue the next, 50 rounds bound it.
      const frames = h => { for (let i = 0; i < 50 && h.frames.size; i++) h.flushFrames(); };
      const settled = async h => { frames(h); await settle(); frames(h); };
      const snap = ed => Object.fromEntries(KEYS.map(key => [key, ed.renderStats[key]]));
      const delta = (a, b) => Object.fromEntries(KEYS.map(key => [key, b[key] - a[key]]));
      const middle = doc => doc.nodes[Math.floor((doc.nodes.length - 1) / 2)].id;

      // open starts the editor for doc on a fresh sandbox: draft and live revision, enabled, so that
      // Run now follows a live run (as in the b5 F6 check). Timers never fire. The view is pinned to
      // zoom 1, so that the automatic-fit floor (READABLE_ZOOM, changed in S0.4) cannot change what a
      // drag reaches or snaps to.
      async function open(doc) {
          const h = sandbox(req => (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run' ? { run_id: 'r1' } : undefined));
          const editor = openEditor(h, { enabled: true, flow: { draft: doc, live: JSON.parse(JSON.stringify(doc)), live_revision: 2, published_draft_revision: 3 } });
          await settled(h);
          editor.ed.view = { x: 0, y: 0, zoom: 1 };
          return { h, editor, ed: editor.ed, canvasEl: editor.el.querySelector('.ed-canvas'), card: id => editor.el.querySelector('[data-node-id="' + id + '"]') };
      }

      // Each scenario opens its own editor and returns the counter deltas of its one action and
      // whether the action took effect, so that a scenario that did nothing cannot pass as cheap.
      const SCENARIOS = {
          async drag(doc) {
              const s = await open(doc);
              const loose = doc.nodes[doc.nodes.length - 1].id;
              const from = s.ed.model.node(loose).position.x;
              const before = snap(s.ed);
              s.card(loose).fire('pointerdown', s.h.pe(1, 100, 100));
              for (let i = 1; i <= 20; i++) {
                  s.canvasEl.fire('pointermove', s.h.pe(1, 100 + i * 12, 100 + i * 5));
                  frames(s.h);
              }
              s.canvasEl.fire('pointerup', s.h.pe(1, 340, 200));
              await settled(s.h);
              const out = { counts: delta(before, snap(s.ed)), acted: s.ed.model.node(loose).position.x !== from };
              s.editor.dispose();
              return out;
          },
          async runEvent(doc) {
              const s = await open(doc);
              s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');
              await settled(s.h);
              const source = s.h.sources[s.h.sources.length - 1];
              const id = middle(doc);
              const before = snap(s.ed);
              if (source) source.emit('event', { seq: 1, type: 'step_finished', node_id: id, step: { node_id: id, status: 'success', duration_ms: 3, item_count: 1 } });
              await settled(s.h);
              const out = { counts: delta(before, snap(s.ed)), acted: !!source && s.card(id).className.includes('status-success') };
              s.editor.dispose();
              return out;
          },
          async rename(doc) {
              const s = await open(doc);
              const id = middle(doc);
              const before = snap(s.ed);
              s.ed.model.setLabel(id, 'Renamed step');
              await settled(s.h);
              const out = { counts: delta(before, snap(s.ed)), acted: s.ed.model.node(id).label === 'Renamed step' };
              s.editor.dispose();
              return out;
          }
      };

      await guardAsync('render baseline S/M/XL counters equal scripts/easydrag-render-baseline.json', async () => {
          const update = process.env.AURAGO_EASYDRAG_RENDER_BASELINE === 'update';
          const counts = {};
          let sane = true;
          for (const spec of FIXTURES) {
              const probe = await open(fixtureDoc(spec));
              const shape = [probe.ed.model.doc.nodes.length, probe.ed.model.doc.edges.length, (probe.ed.model.doc.groups || []).length];
              probe.editor.dispose();
              counts[spec.name] = {};
              const acted = [];
              for (const [name, scenario] of Object.entries(SCENARIOS)) {
                  const out = await scenario(fixtureDoc(spec));
                  counts[spec.name][name] = out.counts;
                  acted.push(out.acted);
              }
              const want = [[spec.nodes, spec.edges, spec.groups], [true, true, true]];
              if (JSON.stringify([shape, acted]) !== JSON.stringify(want)) sane = false;
              eq('render baseline ' + spec.name + ' opens ' + spec.nodes + ' cards, ' + spec.edges + ' wires and ' + spec.groups + ' groups, and drag, run event and rename take effect', [shape, acted], want);
          }
          if (update) {
              if (!sane) { check('render baseline not written: a fixture or a scenario failed', false); return; }
              fs.writeFileSync(BASELINE, JSON.stringify({ spec: 'plans/2026-11-10-easydrag-s0-spec.md section 3', keys: KEYS, counts }, null, 2) + '\n');
              check('render baseline written to ' + path.basename(BASELINE), true);
              return;
          }
          const stored = fs.existsSync(BASELINE) ? JSON.parse(fs.readFileSync(BASELINE, 'utf8')) : null;
          check('render baseline ' + path.basename(BASELINE) + ' exists (node scripts/test-easydrag-render-baseline.mjs --update writes it)', !!stored);
          if (!stored) return;
          for (const spec of FIXTURES) eq('render baseline ' + spec.name + ' counters equal the baseline', counts[spec.name], stored.counts[spec.name]);
      });
  }

  if (path.basename(process.argv[1] || '') === 'test-easydrag-render-baseline.mjs') {
      const update = process.argv.includes('--update');
      const child = spawnSync(process.execPath, [path.join(here, 'test-easydrag.mjs')], {
          env: Object.assign({}, process.env, { AURAGO_EASYDRAG_TEST_FILTER: 'render baseline', AURAGO_EASYDRAG_RENDER_BASELINE: update ? 'update' : 'check' }),
          encoding: 'utf8',
          maxBuffer: 256 * 1024 * 1024
      });
      const lines = String(child.stdout || '').split(/\r?\n/);
      lines.filter(line => line.includes('render baseline') || line.startsWith('FAIL')).forEach(line => console.log(line));
      const passed = lines.filter(line => line.startsWith('ok   render baseline')).length;
      const want = update ? 4 : 7;
      if (child.status !== 0 || lines.some(line => line.startsWith('FAIL')) || passed !== want) {
          console.log(String(child.stderr || '').split(/\r?\n/).slice(-20).join('\n'));
          console.log('FAIL render baseline: exit ' + child.status + ', ' + passed + ' of ' + want + ' checks passed' + (child.error ? ' (' + child.error.message + ')' : ''));
          process.exit(1);
      }
      console.log('PASS render baseline (' + (update ? 'written to ' + path.basename(BASELINE) : 'unchanged') + ')');
  }
  ```

- [ ] **Step 5: Register the baseline in the runner.**
  In `scripts/test-easydrag.mjs`, after line 16 (`// i18n, run-event rendering) on the c1d07 sandbox.`), insert:
  ```js
  // The S0.0 render baseline (test-easydrag-render-baseline.mjs) counts the render work of seeded S, M
  // and XL flows on the c1d07 sandbox and compares it with scripts/easydrag-render-baseline.json.
  ```
  After the b5 line (old :1138, `await (await import('./test-easydrag-ux-batch5.mjs')).run({ ... saverHarness });`), insert:
  ```js
  // ── S0.0: render counters on seeded S/M/XL flows (test-easydrag-render-baseline.mjs, on the c1d07 sandbox) ──
  await (await import('./test-easydrag-render-baseline.mjs')).run({ eq, check, guardAsync, settle, sandbox: editorHarness.sandbox, openEditor: editorHarness.openEditor });
  ```
  Run:
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node scripts/test-easydrag-render-baseline.mjs
  ```
  Expected:
  - three `ok   render baseline S|M|XL opens …` lines;
  - `FAIL render baseline easydrag-render-baseline.json exists (…--update writes it)`;
  - `FAIL render baseline: exit 1, 3 of 7 checks passed`.

- [ ] **Step 6: Write the baseline, then check it.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node scripts/test-easydrag-render-baseline.mjs --update
  node scripts/test-easydrag-render-baseline.mjs
  ```
  - Expected: `PASS render baseline (written to easydrag-render-baseline.json)`, then `PASS render baseline (unchanged)`. The second run is a fresh run against the written counts, which proves the result is deterministic.
  - Open the JSON and check it. It has `counts.S|M|XL.drag|runEvent|rename`, each with the nine keys.
  - In every fixture, `drag.align` is 20 and `drag.edgeNear` is between 20 and 60.
  - If a value is `null` or `NaN`, a counter is missing. Fix it before going on.
  - Do not hand-edit the file.

- [ ] **Step 7: Full tests.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node scripts/test-easydrag.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  ```
  Expected: the Node suite ends without `failure(s)` and exits 0. It now includes the 7 baseline checks. Go prints `ok`.

- [ ] **Step 8: GitNexus detect-changes.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref main --repo C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  ```
  If the runner refuses the worktree path, or answers partial or truncated, report "GitNexus detect-changes: judge by symbols + tests" (the controller decides). Otherwise report the changed symbols and risk, and warn on HIGH or CRITICAL.

- [ ] **Step 9: Commit.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  git add ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-wires.js ui/js/desktop/apps/easydrag-groups.js ui/js/desktop/apps/easydrag-interact.js scripts/test-easydrag-ux-batch5.mjs scripts/test-easydrag.mjs scripts/test-easydrag-render-baseline.mjs scripts/easydrag-render-baseline.json
  git commit -m "test(easydrag): S0.0 render counters and a seeded S/M/XL baseline" -m "Six call counters on ed.renderStats (wireScans, groups, allRects, minimap, edgeNear, align), one ++ each. scripts/test-easydrag-render-baseline.mjs runs drag, run-event and rename scenarios on seeded S/M/XL flows; the counts in scripts/easydrag-render-baseline.json were written with --update and every later S0 commit must reproduce them." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 1.2: Run core and `easydrag-run-stream.js`

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-runs.js` (old line numbers):
  - :46-64: create header and lets become `stopButtons` plus the `rc` literal;
  - :180-184, :417-423, :683-698, :870, :913-916: declarations removed (`stopButtons` :689-697 moves up into the core);
  - :382-383, :895-896: section headings removed;
  - :65-380: the stream moves out;
  - :1077-1091: bag and returned object read from rc.
- Create: `ui/js/desktop/apps/easydrag-run-stream.js`.
- Modify: `ui/js/desktop/core/module-loader.js:309`, `ui/desktop_easydrag_test.go:14`, `scripts/test-easydrag-extra2.mjs:10`, `scripts/test-easydrag-extra3.mjs:11`, `scripts/test-easydrag-extra4.mjs:12`.
- Create (scratch, not committed): `C:/disposable/easydrag-s0/rc-rewrite.mjs`, `C:/disposable/easydrag-s0/move-part.mjs`, `C:/disposable/easydrag-s0/rc-parts-check.mjs`.

- [ ] **Step 1: Failing test. Register the module name first.**
  In `ui/desktop_easydrag_test.go`, replace `"easydrag-runs.js",` with `"easydrag-run-stream.js", "easydrag-runs.js",`.
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  go test ./ui/ -run '(?i)(EasyDragAssetsLoadInDependencyOrder|EasyDragAppLifecycle)' -count=1
  ```
  Expected: FAIL. You see `easydrag assets miss easydrag-run-stream.js` and a read error for the missing file.

- [ ] **Step 2: GitNexus impact (every moved function, plus `stopButtons` and `create`).**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  for fn in create durable noteStep setRun snapshot unknownDurable reconnect refreshDurableShown rootsFrom runData rememberRun closeStream announceFailed attach streamed connect reconnectLater probe streamLost isRunning applyEvent finish stopButtons; do echo "== $fn"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$fn" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
  ```
  - Pick the candidate in `ui/js/desktop/apps/easydrag-runs.js`.
  - Expected callers: inside runs.js, plus the facade users of `attach`, `isRunning` and `create` (editor.js:110 and others). The facade does not change.
  - Warn on HIGH or CRITICAL.
  - On `UNKNOWN`, Grep the name with `\b<fn>\b` in `ui/js` and `scripts`. Closure functions can be reached only inside runs.js or through `runs.<key>`.

- [ ] **Step 3: Create the three scratch tools** (Write tool).
  `C:/disposable/easydrag-s0/rc-rewrite.mjs`:
  ```js
  // EasyDrag S0 task 1.2 (one-off): in create() of easydrag-runs.js, after the run-core literal
  // ("        const rc = {" up to its "        };"), every bare reference to run-core state becomes
  // rc.<name>, and the two section headings ("// ── starting runs", "// ── drawer") go together with
  // the blank line under each. Full-line comments are left alone. Self-check: removing every "rc."
  // gives back the input without the headings. Usage, from the worktree root:
  //   node C:/disposable/easydrag-s0/rc-rewrite.mjs
  import fs from 'node:fs';

  const FILE = 'ui/js/desktop/apps/easydrag-runs.js';
  const NAMES = ['source', 'lastSeq', 'reconnectTimer', 'retryDelay', 'failures', 'failedSpoken', 'parked', 'parkTimer', 'starting', 'startPost', 'stopping', 'durableStopping', 'stopPolls',
      'viewing', 'viewSeq', 'viewRefreshSeq', 'drawer', 'drawerOpener', 'drawerFilter', 'runsQuery', 'runsSeq', 'runsOffset', 'runsMore', 'runsLoading', 'listedRuns', 'disposed'];
  // A name counts when no word character, "$", ".", quote or "-" stands before it, and no word
  // character, "$" or ":" (an object key) follows it.
  const ref = new RegExp('(?<![\\w$.\'"-])(' + NAMES.join('|') + ')(?![\\w$:])', 'g');
  const text = fs.readFileSync(FILE, 'utf8');
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const lines = text.split(/\r?\n/);
  const open = lines.indexOf('        const rc = {');
  const close = lines.findIndex((l, i) => i > open && l === '        };');
  const end = lines.findIndex(l => l.startsWith('    ED.runs = {'));
  if (open < 0 || close < 0 || end < close) throw new Error('markers not found: rc ' + open + ', end of rc ' + close + ', ED.runs ' + end);
  const out = [];
  const expected = [];
  const counts = {};
  for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      const inside = i > close && i < end;
      if (inside && line.trim().startsWith('// ── ')) { if (lines[i + 1] === '') i++; continue; }
      expected.push(line);
      if (!inside || line.trim().startsWith('//')) { out.push(line); continue; }
      out.push(line.replace(ref, (match, name) => { counts[name] = (counts[name] || 0) + 1; return 'rc.' + name; }));
  }
  const strip = list => list.join('\n').replace(/\brc\./g, '');
  if (strip(out) !== strip(expected)) throw new Error('self-check failed: the rewrite changed more than rc. prefixes');
  const unused = NAMES.filter(name => !counts[name]);
  if (unused.length) throw new Error('state never referenced after the core: ' + unused.join(', '));
  fs.writeFileSync(FILE, out.join(eol));
  console.log('rewrote ' + Object.values(counts).reduce((a, b) => a + b, 0) + ' references: ' + JSON.stringify(counts));
  ```
  `C:/disposable/easydrag-s0/move-part.mjs`:
  ```js
  // EasyDrag S0 tasks 1.2-1.5: moves one block of create() in easydrag-runs.js, byte for byte, into a
  // run part. Usage, from the worktree root:
  //   node C:/disposable/easydrag-s0/move-part.mjs <part file> "<first line starts with>" "<stop line starts with>"
  // The block runs from the first line of create() whose trimmed text starts with <first> up to, not
  // including, the next line whose trimmed text starts with <stop>, without its trailing blank lines.
  // The part file must hold the line "        //@@MOVED_BLOCK@@" once; the block replaces it. In
  // easydrag-runs.js the block is replaced by that same placeholder line, for the hand edit after.
  import fs from 'node:fs';

  const RUNS = 'ui/js/desktop/apps/easydrag-runs.js';
  const PLACEHOLDER = '        //@@MOVED_BLOCK@@';
  const [partFile, first, stop] = process.argv.slice(2);
  if (!partFile || !first || !stop) throw new Error('usage: move-part.mjs <part file> <first> <stop>');
  const runsText = fs.readFileSync(RUNS, 'utf8');
  const eol = runsText.includes('\r\n') ? '\r\n' : '\n';
  const runs = runsText.split(/\r?\n/);
  if (runs.includes(PLACEHOLDER)) throw new Error(RUNS + ' still holds a placeholder');
  const create = runs.findIndex(l => l.startsWith('    function create(ed, canvas) {'));
  const from = runs.findIndex((l, i) => i > create && l.trim().startsWith(first));
  const to = from < 0 ? -1 : runs.findIndex((l, i) => i > from && l.trim().startsWith(stop));
  if (create < 0 || from < 0 || to < 0) throw new Error('markers not found: create ' + create + ', first ' + from + ', stop ' + to);
  let end = to;
  while (end > from && runs[end - 1].trim() === '') end--;
  const block = runs.slice(from, end);
  const part = fs.readFileSync(partFile, 'utf8').split(/\r?\n/);
  const slots = part.filter(l => l === PLACEHOLDER).length;
  if (slots !== 1) throw new Error(partFile + ' must hold the placeholder line once, holds it ' + slots + ' times');
  part.splice(part.indexOf(PLACEHOLDER), 1, ...block);
  runs.splice(from, end - from, PLACEHOLDER);
  fs.writeFileSync(partFile, part.join(eol));
  fs.writeFileSync(RUNS, runs.join(eol));
  console.log('moved ' + block.length + ' lines');
  console.log('first: ' + block[0].trim());
  console.log('last:  ' + block[block.length - 1].trim());
  ```
  `C:/disposable/easydrag-s0/rc-parts-check.mjs`:
  ```js
  // EasyDrag S0 tasks 1.2-1.5: static check of the runs split. For easydrag-runs.js and every
  // easydrag-run-*.js that exists:
  //  1. every name of the old create() closure that the file uses is declared in the file (function,
  //     const/let, destructuring, parameter), so no moved code reaches a name that lives elsewhere now;
  //  2. every rc.<function> and every name destructured from rc is put on rc: by the core literal, by
  //     a part's export (its final "return { ... };") or by runs.js's Object.assign(rc, { ... });
  //  3. no bare run-core state name is left (state is reached as rc.<name> only).
  // Usage, from the worktree root: node C:/disposable/easydrag-s0/rc-parts-check.mjs
  import fs from 'node:fs';
  import path from 'node:path';

  const APPS = 'ui/js/desktop/apps';
  const FILES = ['easydrag-run-stream.js', 'easydrag-run-start.js', 'easydrag-run-stop.js', 'easydrag-run-drawer.js', 'easydrag-runs.js'];
  const CORE = ['ed', 'canvas', 'core', 't', 'esc', 'stopButtons', 'isFinal', 'isActive', 'edgeStates', 'triggers', 'realEffects', 'effects', 'unknownEffects',
      'RETRY_MS', 'RETRY_MAX_MS', 'MAX_FAILURES', 'STOP_POLL_MS', 'PARK_POLL_MS', 'EFFECTS_KEY', 'REDACTED', 'EFFECT_ICONS'];
  const FUNCS = ['bag',
      'durable', 'runReads', 'readDurable', 'watchDurable', 'noteStep', 'setRun', 'snapshot', 'unknownDurable', 'reconnect', 'refreshDurableShown', 'rootsFrom', 'runData', 'rememberRun',
      'closeStream', 'announceFailed', 'attach', 'streamed', 'connect', 'reconnectLater', 'probe', 'streamLost', 'isRunning', 'applyEvent', 'finish',
      'storedEffects', 'confirmedEffects', 'confirmEffects', 'uncheckedMarkup', 'effectsMarkup', 'trackStart', 'editsReady', 'startTest', 'openTest',
      'baseRevision', 'run', 'runLive', 'cancel', 'settleFinished', 'setStopPending', 'stopRun', 'settleDurableStop', 'watchParked', 'followStop', 'showsActive', 'confirmStop',
      'runFinished', 'refreshShown', 'refreshView', 'loadLast',
      'triggerLabel', 'runRow', 'matchingRuns', 'drawRuns', 'loadRuns', 'toggleDrawer', 'openRunView', 'stepsFrom', 'applyRunView', 'clearRun'];
  const STATE = ['source', 'lastSeq', 'reconnectTimer', 'retryDelay', 'failures', 'failedSpoken', 'parked', 'parkTimer', 'starting', 'startPost', 'stopping', 'durableStopping', 'stopPolls',
      'viewing', 'viewSeq', 'viewRefreshSeq', 'drawer', 'drawerOpener', 'drawerFilter', 'runsQuery', 'runsSeq', 'runsOffset', 'runsMore', 'runsLoading', 'listedRuns', 'disposed'];
  const KNOWN = CORE.concat(FUNCS);
  const ID = '[A-Za-z_$][\\w$]*';
  // bare finds a name with no word character, "$" or "." before it and no word character or "$"
  // after it, and skips object keys ("name:").
  const bare = list => new RegExp('(?<![\\w$.])(' + list.join('|') + ')(?![\\w$])(?!\\s*:)', 'g');
  // code drops comments and the text of string literals, so that words in them do not count.
  const code = text => text.split(/\r?\n/).filter(l => !l.trim().startsWith('//')).join('\n')
      .replace(/\/\*[\s\S]*?\*\//g, ' ')
      .replace(/'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"/g, "''")
      .replace(/\/\/[^\n]*/g, ' ');
  const names = list => list.split(',').map(part => {
      const m = /(?:^|:)\s*(?:\.\.\.)?\s*([A-Za-z_$][\w$]*)\s*(?:=[^,]*)?$/.exec(part.trim());
      return m ? m[1] : null;
  }).filter(Boolean);
  const files = FILES.filter(name => fs.existsSync(path.join(APPS, name))).map(name => ({ name, src: code(fs.readFileSync(path.join(APPS, name), 'utf8')) }));
  const provided = new Set(CORE);
  for (const f of files) {
      const exp = /return \{([^}]*)\};\s*\}\s*\(ED\.runParts/.exec(f.src);
      if (exp) names(exp[1]).forEach(n => provided.add(n));
      for (const m of f.src.matchAll(/Object\.assign\(rc, \{([^}]*)\}\)/g)) names(m[1]).forEach(n => provided.add(n));
  }
  let bad = 0;
  const fail = msg => { bad++; console.log('FAIL ' + msg); };
  for (const f of files) {
      const declared = new Set();
      for (const m of f.src.matchAll(new RegExp('\\bfunction\\s+(' + ID + ')', 'g'))) declared.add(m[1]);
      for (const m of f.src.matchAll(new RegExp('\\b(?:const|let|var)\\s+(' + ID + ')', 'g'))) declared.add(m[1]);
      for (const m of f.src.matchAll(/\b(?:const|let|var)\s*\{([^}]*)\}/g)) names(m[1]).forEach(n => declared.add(n));
      for (const m of f.src.matchAll(/\bfunction\b[^(]*\(([^)]*)\)/g)) names(m[1]).forEach(n => declared.add(n));
      for (const m of f.src.matchAll(/\(([^()]*)\)\s*=>/g)) names(m[1]).forEach(n => declared.add(n));
      for (const m of f.src.matchAll(new RegExp('(' + ID + ')\\s*=>', 'g'))) declared.add(m[1]);
      const used = new Set(Array.from(f.src.matchAll(bare(KNOWN)), m => m[1]));
      const undeclared = Array.from(used).filter(n => !declared.has(n)).sort();
      if (undeclared.length) fail(f.name + ' uses names it does not declare: ' + undeclared.join(', '));
      const fromRc = [];
      for (const m of f.src.matchAll(/\b(?:const|let)\s*\{([^}]*)\}\s*=\s*rc\s*;/g)) fromRc.push(...names(m[1]));
      for (const m of f.src.matchAll(new RegExp('\\brc\\.(' + ID + ')', 'g'))) if (KNOWN.includes(m[1])) fromRc.push(m[1]);
      const absent = Array.from(new Set(fromRc)).filter(n => !provided.has(n)).sort();
      if (absent.length) fail(f.name + ' takes names from rc that nothing puts there: ' + absent.join(', '));
      const loose = Array.from(new Set(Array.from(f.src.matchAll(bare(STATE)), m => m[1]))).sort();
      if (loose.length) fail(f.name + ' reaches run-core state without rc.: ' + loose.join(', '));
      if (!undeclared.length && !absent.length && !loose.length) console.log('ok   ' + f.name + ' (' + used.size + ' closure names, ' + new Set(fromRc).size + ' from rc)');
  }
  process.exit(bad ? 1 : 0);
  ```

- [ ] **Step 4: Hand edits in `ui/js/desktop/apps/easydrag-runs.js` (Edit tool, before the rewrite).**
  **Edit A.** Replace :46-64:
  ```js
      function create(ed, canvas) {
          const core = ED.core;
          const { t, esc } = ed;
          const bag = core.bag();
          let source = null;
          let lastSeq = 0;
          let reconnectTimer = 0;
          let retryDelay = RETRY_MS;
          let failures = 0;
          // parked is a live run that a run view replaced; clearRun() attaches it again.
          let parked = null;
          let parkTimer = 0;
          let drawer = null;
          let drawerOpener = null;
          let drawerFilter = 'all';
          let runsSeq = 0;
          let viewSeq = 0;
          let viewing = null;
          let disposed = false;
  ```
  with:
  ```js
      function create(ed, canvas) {
          const core = ED.core;
          const { t, esc } = ed;
          const bag = core.bag();

          // stopButtons are every control for this still-active run: drawer, header and run view.
          function stopButtons(id) {
              if (!ed.root || !ed.root.querySelectorAll) return [];
              return Array.from(ed.root.querySelectorAll('[data-ed-run-stop], [data-ed-cmd="cancel"], [data-ed-cmd="stop-viewed"]')).filter(b => {
                  if (b.dataset.edRunStop) return b.dataset.edRunStop === String(id);
                  if (b.dataset.edCmd === 'cancel') return !!(ed.run && ed.run.id === id && !ed.run.view);
                  return !!(ed.runView && ed.runView.run && ed.runView.run.id === id);
              });
          }

          // rc is the run core. It holds the state that the run parts share (easydrag-run-stream.js,
          // -start.js, -stop.js and -drawer.js), the helpers and limits they use and, once a part is
          // installed, that part's functions: a part reaches state and the other parts through rc only.
          const rc = {
              ed, canvas, core, t, esc, stopButtons,
              isFinal, isActive, edgeStates, triggers, realEffects, effects, unknownEffects,
              RETRY_MS, RETRY_MAX_MS, MAX_FAILURES, STOP_POLL_MS, PARK_POLL_MS, EFFECTS_KEY, REDACTED, EFFECT_ICONS,
              // The live run's stream: its EventSource, the last event seen, the reconnect timer, the
              // backoff and the failures in a row without a message.
              source: null,
              lastSeq: 0,
              reconnectTimer: 0,
              retryDelay: RETRY_MS,
              failures: 0,
              // failedSpoken holds the steps of the live run whose failure was announced: a step that
              // reports again (the snapshot of a reconnect) is not announced twice. Steps that went well
              // are not announced at all; the end of the run is (finish).
              failedSpoken: new Set(),
              // parked is a live run that a run view replaced; clearRun() attaches it again.
              parked: null,
              parkTimer: 0,
              // starting is true from a startTest call until its run was posted or its dialog closed:
              // one test at a time, and none while a run is active (it would orphan that run's stream).
              starting: false,
              // startPost is the test or live POST in flight. A second call waits on the same
              // promise and does not send again. An uncertain result is not sent again.
              startPost: null,
              // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
              stopping: new Set(),
              durableStopping: new Set(),
              // stopPolls holds the timers of followStop.
              stopPolls: new Set(),
              // The run view: the run being opened (openRunView) and the numbers of the latest open and
              // of the latest refresh (refreshView), so that late replies cannot replace a newer view.
              viewing: null,
              viewSeq: 0,
              viewRefreshSeq: 0,
              // The run drawer: its element, the control that opened it, its filter and search, its
              // latest request and paging, and the runs it lists by id (for their Stop buttons).
              drawer: null,
              drawerOpener: null,
              drawerFilter: 'all',
              runsQuery: '',
              runsSeq: 0,
              runsOffset: 0,
              runsMore: false,
              runsLoading: false,
              listedRuns: new Map(),
              disposed: false
          };

  ```
  The new text ends with one blank line. The next line stays `        const durable = run => !!(ED.durable && ED.durable.has(run));`.

  **Edit B.** Replace:
  ```js
          }

          // failedSpoken holds the steps of the live run whose failure was announced: a step that
          // reports again (the snapshot of a reconnect) is not announced twice. Steps that went well
          // are not announced at all; the end of the run is (finish).
          let failedSpoken = new Set();

          function announceFailed(nodeId, step) {
  ```
  with:
  ```js
          }

          function announceFailed(nodeId, step) {
  ```
  **Edit C.** Replace:
  ```js
          // starting is true from a startTest call until its run was posted or its dialog closed:
          // one test at a time, and none while a run is active (it would orphan that run's stream).
          let starting = false;
          // startPost is the test or live POST in flight. A second call waits on the same
          // promise and does not send again. An uncertain result is not sent again.
          let startPost = null;

          function trackStart(work) {
  ```
  with:
  ```js
          function trackStart(work) {
  ```
  **Edit D.** Replace, from `        // stopping holds the runs` through the original `stopButtons` function:
  ```js
          // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
          const stopping = new Set();
          const durableStopping = new Set();
          // stopPolls holds the timers of followStop.
          const stopPolls = new Set();

          // stopButtons are every control for this still-active run: drawer, header and run view.
          function stopButtons(id) {
              if (!ed.root || !ed.root.querySelectorAll) return [];
              return Array.from(ed.root.querySelectorAll('[data-ed-run-stop], [data-ed-cmd="cancel"], [data-ed-cmd="stop-viewed"]')).filter(b => {
                  if (b.dataset.edRunStop) return b.dataset.edRunStop === String(id);
                  if (b.dataset.edCmd === 'cancel') return !!(ed.run && ed.run.id === id && !ed.run.view);
                  return !!(ed.runView && ed.runView.run && ed.runView.run.id === id);
              });
          }

          // setStopPending shows the stop request on those buttons until it settles.
  ```
  with:
  ```js
          // setStopPending shows the stop request on those buttons until it settles.
  ```
  **Edit E.** Replace:
  ```js
          // Late replies cannot replace a newer view or undo a terminal state.
          let viewRefreshSeq = 0;
          async function refreshView(runId) {
  ```
  with:
  ```js
          // Late replies cannot replace a newer view or undo a terminal state.
          async function refreshView(runId) {
  ```
  **Edit F.** Replace:
  ```js
          // listedRuns holds the runs the drawer shows, by id (for their Stop buttons).
          let listedRuns = new Map();
          let runsOffset = 0, runsMore = false, runsLoading = false, runsQuery = '';

          function matchingRuns() {
  ```
  with:
  ```js
          function matchingRuns() {
  ```

- [ ] **Step 5: Mechanical rewrite to `rc.<state>`.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node C:/disposable/easydrag-s0/rc-rewrite.mjs
  node --check ui/js/desktop/apps/easydrag-runs.js
  ```
  - Expected: `rewrote N references: {...}`, where all 26 names appear. The script throws otherwise, and also when its self-check fails.
  - `node --check` prints nothing.
  - The two `// ── …` headings are gone.

- [ ] **Step 6: Create the stream part skeleton** (`ui/js/desktop/apps/easydrag-run-stream.js`, Write tool):
  ```js
  // EasyDrag run stream: follows a live run (SSE with reconnect and backoff, durable reads), applies
  // its events to the run state, announces failed steps and loads the stored result when it ends.
  // One of the four run parts that create() in easydrag-runs.js installs on its run core; it loads
  // before easydrag-runs.js.
  (function () {
      'use strict';

      const ED = window.EasyDrag = window.EasyDrag || {};

      // installStream installs the stream part on the run core rc and returns its functions, which
      // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
      // parts are called through rc when they run, after every part is installed.
      function installStream(rc) {
          const { ed, canvas, core, t, isFinal, edgeStates, RETRY_MS, RETRY_MAX_MS, MAX_FAILURES } = rc;
          const settleDurableStop = (...args) => rc.settleDurableStop(...args);
          const watchParked = (...args) => rc.watchParked(...args);
          const loadRuns = (...args) => rc.loadRuns(...args);

          //@@MOVED_BLOCK@@

          return {
              durable, runReads, readDurable, watchDurable, noteStep, setRun, snapshot, unknownDurable, reconnect, refreshDurableShown, rootsFrom, runData, rememberRun,
              closeStream, announceFailed, attach, streamed, connect, reconnectLater, probe, streamLost, isRunning, applyEvent, finish
          };
      }

      (ED.runParts = ED.runParts || {}).stream = installStream;
  })();
  ```

- [ ] **Step 7: Move the stream block.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node C:/disposable/easydrag-s0/move-part.mjs ui/js/desktop/apps/easydrag-run-stream.js "const durable = run =>" "// Real effects are confirmed one by one"
  ```
  - Expected: `moved 311 lines`. That is the old :65-380 minus the 5 lines removed by Edit B.
  - First line: `const durable = run => !!(ED.durable && ED.durable.has(run));`. Last line: `}` (the end of `finish`).

- [ ] **Step 8: Wire runs.js.**
  Replace the placeholder line `        //@@MOVED_BLOCK@@` in runs.js with:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, finish, isRunning, readDurable, rememberRun, rootsFrom, runData, setRun, snapshot, streamed, unknownDurable } = rc;
  ```
  Replace the bag and the returned object. After the rewrite they read exactly:
  ```js
          bag.add(closeStream);
          bag.add(ed.bus.on('refresh-durable-run', refreshDurableShown));
          bag.add(() => runReads.dispose());
          bag.add(() => clearTimeout(rc.parkTimer));
          bag.add(() => { rc.stopPolls.forEach(clearTimeout); rc.stopPolls.clear(); });
          bag.add(() => runFinished.cancel());
          bag.add(() => toggleDrawer(false));
          bag.add(() => { if (ED.compare) ED.compare.close(ed); });

          return {
              startTest, attach, runLive, cancel, stopRun, refreshShown, runFinished, loadLast, toggleDrawer, openRunView, effects, stepsFrom, applyRunView, clearRun,
              isRunning, hasParked: () => !!rc.parked,
              drawerOpen: () => !!rc.drawer,
              dispose() { rc.disposed = true; rc.parked = null; bag.dispose(); }
          };
  ```
  New text:
  ```js
          // The functions that have not moved into a part yet are on rc too: the parts call them through rc.
          Object.assign(rc, {
              storedEffects, confirmedEffects, confirmEffects, uncheckedMarkup, effectsMarkup, trackStart, editsReady, startTest, openTest,
              baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop, runFinished, refreshShown, refreshView, loadLast,
              triggerLabel, runRow, matchingRuns, drawRuns, loadRuns, toggleDrawer, openRunView, stepsFrom, applyRunView, clearRun
          });

          bag.add(rc.closeStream);
          bag.add(ed.bus.on('refresh-durable-run', rc.refreshDurableShown));
          bag.add(() => rc.runReads.dispose());
          bag.add(() => clearTimeout(rc.parkTimer));
          bag.add(() => { rc.stopPolls.forEach(clearTimeout); rc.stopPolls.clear(); });
          bag.add(() => rc.runFinished.cancel());
          bag.add(() => rc.toggleDrawer(false));
          bag.add(() => { if (ED.compare) ED.compare.close(ed); });

          return {
              startTest: rc.startTest, attach: rc.attach, runLive: rc.runLive, cancel: rc.cancel, stopRun: rc.stopRun, refreshShown: rc.refreshShown, runFinished: rc.runFinished,
              loadLast: rc.loadLast, toggleDrawer: rc.toggleDrawer, openRunView: rc.openRunView, effects, stepsFrom: rc.stepsFrom, applyRunView: rc.applyRunView, clearRun: rc.clearRun,
              isRunning: rc.isRunning, hasParked: () => !!rc.parked,
              drawerOpen: () => !!rc.drawer,
              dispose() { rc.disposed = true; rc.parked = null; bag.dispose(); }
          };
  ```
  Keys and their order are unchanged. The functions are the same objects as before.

- [ ] **Step 9: Register the part (it loads before runs.js).**
  - `ui/js/desktop/core/module-loader.js`: replace `                '/js/desktop/apps/easydrag-runs.js',` with two lines:
    ```js
                    '/js/desktop/apps/easydrag-run-stream.js',
                    '/js/desktop/apps/easydrag-runs.js',
    ```
  - `scripts/test-easydrag-extra2.mjs`, `scripts/test-easydrag-extra3.mjs`, `scripts/test-easydrag-extra4.mjs`: replace the single `'easydrag-runs.js'` in each file with `'easydrag-run-stream.js', 'easydrag-runs.js'`.
  - `ui/desktop_easydrag_test.go` was done in Step 1.

- [ ] **Step 10: Tests.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node --check ui/js/desktop/apps/easydrag-runs.js && node --check ui/js/desktop/apps/easydrag-run-stream.js
  node C:/disposable/easydrag-s0/rc-parts-check.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  node scripts/test-easydrag.mjs
  node scripts/test-easydrag-render-baseline.mjs
  ```
  Expected:
  - `ok   easydrag-run-stream.js (…)` and `ok   easydrag-runs.js (…)`, exit 0;
  - Go `ok`;
  - the Node suite exits 0;
  - `PASS render baseline (unchanged)`.
  - runs.js is about 840 lines and the stream part about 339.

- [ ] **Step 11: GitNexus detect-changes.** Same command and rule as Task 1.1 Step 8.

- [ ] **Step 12: Commit.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  git add ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-stream.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
  git commit -m "refactor(easydrag): run core and easydrag-run-stream.js out of easydrag-runs.js" -m "create() builds the run core rc (shared state, helpers, limits, stopButtons); state is reached as rc.<name>. The live-run stream (connect, applyEvent, finish and their helpers) moves verbatim into easydrag-run-stream.js, installed through ED.runParts.stream. ED.runs and the object create() returns are unchanged; the render baseline is unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 1.3: `easydrag-run-start.js` (test dialog and starting tests)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-runs.js`. As of main this is :384-613 (`storedEffects` :389, `confirmEffects` :400, `trackStart` :424, `startTest` :435, `openTest` :448), minus :417-423 already removed. The install and destructure lines and the first line of the remaining-exports block change.
- Create: `ui/js/desktop/apps/easydrag-run-start.js`.
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go:14`, `scripts/test-easydrag-extra2.mjs:10`, `scripts/test-easydrag-extra3.mjs:11`, `scripts/test-easydrag-extra4.mjs:12`, `scripts/test-easydrag-extra5.mjs:622`.

- [ ] **Step 1: Failing test.** In `ui/desktop_easydrag_test.go`, replace `"easydrag-runs.js",` with `"easydrag-run-start.js", "easydrag-runs.js",`.
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  go test ./ui/ -run '(?i)(EasyDragAssetsLoadInDependencyOrder|EasyDragAppLifecycle)' -count=1
  ```
  Expected: FAIL with `easydrag assets miss easydrag-run-start.js`.

- [ ] **Step 2: GitNexus impact.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  for fn in storedEffects confirmedEffects confirmEffects uncheckedMarkup effectsMarkup trackStart editsReady startTest openTest; do echo "== $fn"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$fn" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
  ```
  - Expected: callers inside runs.js; `startTest` is also reached through the facade by editor.js, detail.js and testing.js.
  - Warn on HIGH or CRITICAL.
  - On `UNKNOWN`, Grep `\b(storedEffects|confirmedEffects|confirmEffects|uncheckedMarkup|effectsMarkup|trackStart|editsReady|startTest|openTest)\b` in `ui/js` and `scripts`.

- [ ] **Step 3: Create the skeleton** (`ui/js/desktop/apps/easydrag-run-start.js`):
  ```js
  // EasyDrag run start: the test dialog (trigger, sample data, real-effects warning, Simulation) and
  // starting a test of the saved draft, one at a time. One of the four run parts that create() in
  // easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
  (function () {
      'use strict';

      const ED = window.EasyDrag = window.EasyDrag || {};

      // installStart installs the start part on the run core rc and returns its functions, which
      // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
      // parts are called through rc when they run, after every part is installed.
      function installStart(rc) {
          const { ed, canvas, core, t, esc, triggers, realEffects, effects, unknownEffects, EFFECTS_KEY, REDACTED, EFFECT_ICONS } = rc;
          const isRunning = (...args) => rc.isRunning(...args);
          const rootsFrom = (...args) => rc.rootsFrom(...args);
          const setRun = (...args) => rc.setRun(...args);
          const run = (...args) => rc.run(...args);
          const baseRevision = (...args) => rc.baseRevision(...args);
          const stepsFrom = (...args) => rc.stepsFrom(...args);

          //@@MOVED_BLOCK@@

          return { storedEffects, confirmedEffects, confirmEffects, uncheckedMarkup, effectsMarkup, trackStart, editsReady, startTest, openTest };
      }

      (ED.runParts = ED.runParts || {}).start = installStart;
  })();
  ```
  - The local `const run = stepsFrom(…)` inside the Simulation `apply` callback shadows the wrapper, as it shadowed the stop function before.
  - `revision: baseRevision` now hands `ED.testing.decorate` the wrapper, which returns the same value.

- [ ] **Step 4: Move the block.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node C:/disposable/easydrag-s0/move-part.mjs ui/js/desktop/apps/easydrag-run-start.js "// Real effects are confirmed one by one" "// baseRevision is the draft revision a test runs"
  ```
  - Expected: `moved 223 lines`. That is the old :384-613 minus the 7 lines removed in 1.2 Edit C.
  - First line: `// Real effects are confirmed one by one (sends_message, deletes, …): confirming the effects`. Last line: `}` (the end of `openTest`).

- [ ] **Step 5: Wire runs.js.** Replace:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, finish, isRunning, readDurable, rememberRun, rootsFrom, runData, setRun, snapshot, streamed, unknownDurable } = rc;

          //@@MOVED_BLOCK@@
  ```
  with:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          Object.assign(rc, ED.runParts.start(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, finish, isRunning, readDurable, rememberRun, runData, setRun, snapshot, streamed, trackStart, unknownDurable } = rc;
  ```
  In the remaining-exports block, delete the line:
  ```js
              storedEffects, confirmedEffects, confirmEffects, uncheckedMarkup, effectsMarkup, trackStart, editsReady, startTest, openTest,
  ```

- [ ] **Step 6: Register, and fix the content check.**
  - module-loader.js: replace `                '/js/desktop/apps/easydrag-runs.js',` with:
    ```js
                    '/js/desktop/apps/easydrag-run-start.js',
                    '/js/desktop/apps/easydrag-runs.js',
    ```
  - extra2, extra3, extra4: replace `'easydrag-runs.js'` with `'easydrag-run-start.js', 'easydrag-runs.js'`.
  - `scripts/test-easydrag-extra5.mjs:622`: replace `src('easydrag-runs.js').includes("'aurago.easydrag.test-trigger.'")` with `src('easydrag-run-start.js').includes("'aurago.easydrag.test-trigger.'")`. `EFFECTS_KEY` stays in runs.js, so the `effects-ok` half keeps reading `easydrag-runs.js`.

- [ ] **Step 7: Tests.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node --check ui/js/desktop/apps/easydrag-runs.js && node --check ui/js/desktop/apps/easydrag-run-start.js
  node C:/disposable/easydrag-s0/rc-parts-check.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  node scripts/test-easydrag.mjs
  node scripts/test-easydrag-render-baseline.mjs
  ```
  Expected:
  - three `ok   …` lines from the check;
  - Go `ok`;
  - the Node suite exits 0, including `ok   ff2 M9 forgetFlow knows every per-flow key prefix`;
  - `PASS render baseline (unchanged)`.

- [ ] **Step 8: GitNexus detect-changes.** Same as Task 1.1 Step 8.

- [ ] **Step 9: Commit.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  git add ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-start.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs scripts/test-easydrag-extra5.mjs
  git commit -m "refactor(easydrag): easydrag-run-start.js (test dialog, starting tests) out of easydrag-runs.js" -m "storedEffects, confirmEffects, trackStart, startTest and openTest move verbatim into easydrag-run-start.js, installed through ED.runParts.start. The ff2 M9 key check reads the test-trigger prefix there. ED.runs is unchanged; the render baseline is unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 1.4: `easydrag-run-stop.js` (run, cancel and stop)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-runs.js`. As of main this is :615-893 (`baseRevision` :617, `run` :622, `runLive` :639, `cancel` :665, `settleFinished` :674, `setStopPending` :700, `stopRun` :730, `settleDurableStop` :791, `watchParked` :801, `followStop` :824, `showsActive` :840, `confirmStop` :846, `runFinished` :856, `refreshShown` :862, `refreshView` :871, `loadLast` :884). :683-698 and :870 were already removed. The install and destructure lines and the remaining-exports block change.
- Create: `ui/js/desktop/apps/easydrag-run-stop.js`.
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go:14`, `scripts/test-easydrag-extra2.mjs:10`, `scripts/test-easydrag-extra3.mjs:11`, `scripts/test-easydrag-extra4.mjs:12`.

- [ ] **Step 1: Failing test.** In `ui/desktop_easydrag_test.go`, replace `"easydrag-runs.js",` with `"easydrag-run-stop.js", "easydrag-runs.js",`. Run the Go command from Task 1.3 Step 1. Expected: FAIL with `easydrag assets miss easydrag-run-stop.js`.

- [ ] **Step 2: GitNexus impact.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  for fn in baseRevision run runLive cancel settleFinished setStopPending stopRun settleDurableStop watchParked followStop showsActive confirmStop refreshShown refreshView loadLast; do echo "== $fn"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$fn" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
  ```
  - Expected: callers inside runs.js. `runLive`, `cancel`, `stopRun`, `refreshShown` and `loadLast` are also reached through the facade (editor.js, compare.js).
  - Warn on HIGH or CRITICAL.
  - On `UNKNOWN`, Grep the names in `ui/js` and `scripts`.

- [ ] **Step 3: Create the skeleton** (`ui/js/desktop/apps/easydrag-run-stop.js`):
  ```js
  // EasyDrag run stop: Run now and posting a test run; cancelling and stopping runs from the header,
  // the drawer and the run view (also runs this window did not start), with the polls that follow a
  // stop and a parked run; the run view refresh and the last run for the input tree. One of the four
  // run parts that create() in easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
  (function () {
      'use strict';

      const ED = window.EasyDrag = window.EasyDrag || {};

      // installStop installs the stop part on the run core rc and returns its functions, which
      // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
      // parts are called through rc when they run, after every part is installed.
      function installStop(rc) {
          const { ed, canvas, core, t, esc, isFinal, isActive, stopButtons, STOP_POLL_MS, PARK_POLL_MS } = rc;
          const trackStart = (...args) => rc.trackStart(...args);
          const attach = (...args) => rc.attach(...args);
          const isRunning = (...args) => rc.isRunning(...args);
          const closeStream = (...args) => rc.closeStream(...args);
          const finish = (...args) => rc.finish(...args);
          const readDurable = (...args) => rc.readDurable(...args);
          const snapshot = (...args) => rc.snapshot(...args);
          const setRun = (...args) => rc.setRun(...args);
          const streamed = (...args) => rc.streamed(...args);
          const unknownDurable = (...args) => rc.unknownDurable(...args);
          const rememberRun = (...args) => rc.rememberRun(...args);
          const loadRuns = (...args) => rc.loadRuns(...args);
          const applyRunView = (...args) => rc.applyRunView(...args);

          //@@MOVED_BLOCK@@

          return {
              baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop,
              runFinished, refreshShown, refreshView, loadLast
          };
      }

      (ED.runParts = ED.runParts || {}).stop = installStop;
  })();
  ```

- [ ] **Step 4: Move the block.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node C:/disposable/easydrag-s0/move-part.mjs ui/js/desktop/apps/easydrag-run-stop.js "// baseRevision is the draft revision a test runs" "function triggerLabel(r) {"
  ```
  - Expected: `moved 262 lines`. That is the old :615-893 minus the 17 lines removed in 1.2 Edits D and E.
  - First line: `// baseRevision is the draft revision a test runs: the saver's (it saved just before) or the`. Last line: `}` (the end of `loadLast`).

- [ ] **Step 5: Wire runs.js.** Replace:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          Object.assign(rc, ED.runParts.start(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, finish, isRunning, readDurable, rememberRun, runData, setRun, snapshot, streamed, trackStart, unknownDurable } = rc;

          //@@MOVED_BLOCK@@
  ```
  with:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          Object.assign(rc, ED.runParts.start(rc));
          Object.assign(rc, ED.runParts.stop(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, runData, setRun, setStopPending, stopRun, watchParked } = rc;
  ```
  In the remaining-exports block, delete the line:
  ```js
              baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop, runFinished, refreshShown, refreshView, loadLast,
  ```

- [ ] **Step 6: Register.**
  - module-loader.js: replace `                '/js/desktop/apps/easydrag-runs.js',` with:
    ```js
                    '/js/desktop/apps/easydrag-run-stop.js',
                    '/js/desktop/apps/easydrag-runs.js',
    ```
  - extra2, extra3, extra4: replace `'easydrag-runs.js'` with `'easydrag-run-stop.js', 'easydrag-runs.js'`.

- [ ] **Step 7: Tests.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node --check ui/js/desktop/apps/easydrag-runs.js && node --check ui/js/desktop/apps/easydrag-run-stop.js
  node C:/disposable/easydrag-s0/rc-parts-check.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  node scripts/test-easydrag.mjs
  node scripts/test-easydrag-render-baseline.mjs
  ```
  Expected: four `ok   …` lines, Go `ok`, the Node suite exits 0, and `PASS render baseline (unchanged)`.

- [ ] **Step 8: GitNexus detect-changes.** Same as Task 1.1 Step 8.

- [ ] **Step 9: Commit.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  git add ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-stop.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
  git commit -m "refactor(easydrag): easydrag-run-stop.js (run, cancel, stop) out of easydrag-runs.js" -m "run, runLive, cancel, stopRun, confirmStop, the stop and parked-run polls, refreshView and loadLast move verbatim into easydrag-run-stop.js, installed through ED.runParts.stop; stopButtons, listedRuns and the stopping sets stay on the run core. ED.runs is unchanged; the render baseline is unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 1.5: `easydrag-run-drawer.js` (drawer and run view); runs.js is the core

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-runs.js`:
  - :1-2: header comment;
  - the drawer block, as of main :897-1075 (`triggerLabel` :897, `runRow` :902, `matchingRuns` :917, `drawRuns` :921, `loadRuns` :937, `toggleDrawer` :981, `openRunView` :1028, `stepsFrom` :1050, `applyRunView` :1058, `clearRun` :1068), minus :913-916 already removed;
  - the install lines; the destructure line and the remaining-exports block are removed.
- Create: `ui/js/desktop/apps/easydrag-run-drawer.js`.
- Modify: `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go:14`, `scripts/test-easydrag-extra2.mjs:10`, `scripts/test-easydrag-extra3.mjs:11`, `scripts/test-easydrag-extra4.mjs:12`.

- [ ] **Step 1: Failing test.** In `ui/desktop_easydrag_test.go`, replace `"easydrag-runs.js",` with `"easydrag-run-drawer.js", "easydrag-runs.js",`. Run the Go command from Task 1.3 Step 1. Expected: FAIL with `easydrag assets miss easydrag-run-drawer.js`.

- [ ] **Step 2: GitNexus impact.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo
  for fn in triggerLabel runRow matchingRuns drawRuns loadRuns toggleDrawer openRunView stepsFrom applyRunView clearRun create; do echo "== $fn"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$fn" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
  ```
  - Expected: callers inside runs.js. `toggleDrawer`, `openRunView`, `stepsFrom`, `applyRunView` and `clearRun` are also reached through the facade (detail.js:276, compare.js:65/:108, testing.js:199, editor.js:110/:260/:359/:374/:385).
  - Warn on HIGH or CRITICAL.
  - On `UNKNOWN`, Grep the names in `ui/js` and `scripts`.

- [ ] **Step 3: Create the skeleton** (`ui/js/desktop/apps/easydrag-run-drawer.js`):
  ```js
  // EasyDrag run drawer: the list of a flow's runs (filter, search, paging, a Stop per active run,
  // compare) and the read-only run view of a stored run, which parks a live run until the view ends.
  // One of the four run parts that create() in easydrag-runs.js installs on its run core; it loads
  // before easydrag-runs.js.
  (function () {
      'use strict';

      const ED = window.EasyDrag = window.EasyDrag || {};

      // installDrawer installs the drawer part on the run core rc and returns its functions, which
      // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
      // parts are called through rc when they run, after every part is installed.
      function installDrawer(rc) {
          const { ed, core, t, esc, isFinal, isActive } = rc;
          const closeStream = (...args) => rc.closeStream(...args);
          const runData = (...args) => rc.runData(...args);
          const setRun = (...args) => rc.setRun(...args);
          const attach = (...args) => rc.attach(...args);
          const setStopPending = (...args) => rc.setStopPending(...args);
          const stopRun = (...args) => rc.stopRun(...args);
          const watchParked = (...args) => rc.watchParked(...args);

          //@@MOVED_BLOCK@@

          return { triggerLabel, runRow, matchingRuns, drawRuns, loadRuns, toggleDrawer, openRunView, stepsFrom, applyRunView, clearRun };
      }

      (ED.runParts = ED.runParts || {}).drawer = installDrawer;
  })();
  ```

- [ ] **Step 4: Move the block.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  node C:/disposable/easydrag-s0/move-part.mjs ui/js/desktop/apps/easydrag-run-drawer.js "function triggerLabel(r) {" "// The functions that have not moved into a part yet"
  ```
  - Expected: `moved 175 lines`. That is the old :897-1075 minus the 4 lines removed in 1.2 Edit F.
  - First line: `function triggerLabel(r) {`. Last line: `}` (the end of `clearRun`).

- [ ] **Step 5: Wire runs.js and update its header.** Replace:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          Object.assign(rc, ED.runParts.start(rc));
          Object.assign(rc, ED.runParts.stop(rc));
          // The code that has not moved into a part yet calls the parts' functions by these names.
          const { attach, closeStream, runData, setRun, setStopPending, stopRun, watchParked } = rc;

          //@@MOVED_BLOCK@@

          // The functions that have not moved into a part yet are on rc too: the parts call them through rc.
          Object.assign(rc, {
              triggerLabel, runRow, matchingRuns, drawRuns, loadRuns, toggleDrawer, openRunView, stepsFrom, applyRunView, clearRun
          });
  ```
  with:
  ```js
          Object.assign(rc, ED.runParts.stream(rc));
          Object.assign(rc, ED.runParts.start(rc));
          Object.assign(rc, ED.runParts.stop(rc));
          Object.assign(rc, ED.runParts.drawer(rc));
  ```
  Replace lines 1-2:
  ```js
  // EasyDrag runs: test dialog (trigger, sample data, real-effects warning), live run stream
  // (SSE with reconnect), step and wire states, run drawer and read-only run view, stopping their runs.
  ```
  with:
  ```js
  // EasyDrag runs: the ED.runs facade, the wire states of a run (edgeStates) and the run core.
  // create() builds the core (shared run state, helpers, limits) and installs the four run parts
  // on it: easydrag-run-stream.js (live run stream), easydrag-run-start.js (test dialog),
  // easydrag-run-stop.js (run, cancel, stop) and easydrag-run-drawer.js (run drawer, run view).
  ```
  After this step, `create()` reads exactly as follows (check it against the file):
  - the four lines `const core`, `const { t, esc }`, `const bag`, a blank line;
  - `stopButtons`, then the `rc` literal from 1.2 Edit A, then a blank line;
  - the four `Object.assign(rc, ED.runParts.<part>(rc));` lines (stream, start, stop, drawer), then a blank line;
  - the eight `bag.add(...)` lines and the `return { … }` from 1.2 Step 8;
  - `    }`.

  Lines 3-45 (the module helpers and constants) and `ED.runs = { create, effects, unknownEffects, triggers, edgeStates, isFinal, isActive };` are untouched.

- [ ] **Step 6: Register.**
  - module-loader.js: replace `                '/js/desktop/apps/easydrag-runs.js',` with:
    ```js
                    '/js/desktop/apps/easydrag-run-drawer.js',
                    '/js/desktop/apps/easydrag-runs.js',
    ```
    The list now reads `…easydrag-approvals.js, easydrag-run-stream.js, easydrag-run-start.js, easydrag-run-stop.js, easydrag-run-drawer.js, easydrag-runs.js, easydrag-metadata.js…`.
  - extra2, extra3, extra4: replace `'easydrag-runs.js'` with `'easydrag-run-drawer.js', 'easydrag-runs.js'`.

- [ ] **Step 7: Tests, size and facade.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  for f in easydrag-runs easydrag-run-stream easydrag-run-start easydrag-run-stop easydrag-run-drawer; do node --check ui/js/desktop/apps/$f.js || exit 1; done
  node C:/disposable/easydrag-s0/rc-parts-check.mjs
  wc -l ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-*.js
  grep -c "//@@MOVED_BLOCK@@" ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-*.js
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  node scripts/test-easydrag.mjs
  node scripts/test-easydrag-render-baseline.mjs
  AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m
  ```
  Expected:
  - The check prints five `ok   …` lines.
  - `wc` shows runs.js at about 146 lines (it must be at most 400). The parts come to about stream 339, start 250, stop 299 and drawer 203.
  - Every placeholder count is 0.
  - Go `ok`, the Node suite exits 0, and `PASS render baseline (unchanged)`.
  - The browser run prints `ok`.
    - If Chrome fails to launch with "crashpad access denied", follow `reference_chrome_launch_from_agent.md`.
    - If a browser test fails, bisect the four split commits with the same command.

  Facade check: Grep (`-o`) `\bruns\.[A-Za-z]+` in `ui/js/desktop/apps`. Every name after `runs.` must be one of:
  - the `ED.runs` keys `create`, `effects`, `unknownEffects`, `triggers`, `edgeStates`, `isFinal`, `isActive`;
  - or the returned keys `startTest`, `attach`, `runLive`, `cancel`, `stopRun`, `refreshShown`, `runFinished`, `loadLast`, `toggleDrawer`, `openRunView`, `stepsFrom`, `applyRunView`, `clearRun`, `isRunning`, `hasParked`, `drawerOpen`, `dispose`.

- [ ] **Step 8: GitNexus detect-changes.** Same as Task 1.1 Step 8.

- [ ] **Step 9: Commit.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0
  git add ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-drawer.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
  git commit -m "refactor(easydrag): easydrag-run-drawer.js (drawer, run view); easydrag-runs.js is the run core" -m "The run drawer and the run view (drawRuns, loadRuns, toggleDrawer, openRunView, stepsFrom, applyRunView, clearRun) move verbatim into easydrag-run-drawer.js, installed through ED.runParts.drawer. easydrag-runs.js keeps the module helpers, the run core and the facade (about 146 lines, budget 400). ED.runs and the returned object are unchanged; the render baseline is unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Part 1 coverage

| Spec item | Task |
|---|---|
| §3 six counters `wireScans, groups, allRects, minimap, edgeNear, align`, one `++` at entry | 1.1 Step 3 |
| §3 seeded PRNG fixtures S 8/7/0, M 60/70/4, XL 300/360/20 (fixed seeds, no clock) | 1.1 Step 4 (`mulberry32`, `fixtureDoc`, shape check) |
| §3 scenarios: drag 20 frames, one step-done run event mid-flow, one rename | 1.1 Step 4 (`SCENARIOS`) |
| §3 counts committed as `scripts/easydrag-render-baseline.json` | 1.1 Steps 6 and 9 |
| §3 exact-count ratchet; every later commit reproduces it | 1.1 (check mode, also in every full Node run); BASELINE step in 1.2–1.5 |
| §3 batch5 :812-831 asserts the new keys | 1.1 Step 2 |
| §3 acceptance: baseline commit before any S0.1/S0.4/S0.5 commit | 1.1 is the first commit of Part 1 |
| §4 split by responsibility, one commit per moved module | 1.2 stream, 1.3 start, 1.4 stop, 1.5 drawer |
| §4 impact per moved function (absolute runner, UNKNOWN confirmed by grep) | Step 2 of 1.2–1.5 |
| §4 facades, bus wiring and `ED.*` exports unchanged | 1.2 Step 8 (same keys, same function objects), 1.5 Step 7 facade grep |
| §4 new files start with `//` and `'use strict'` | skeletons in 1.2–1.5 Step 3/6 |
| §4 registration in module-loader `easydrag` list and `easyDragModules`, parts before runs.js | Step 1 and Step 6/9 of 1.2–1.5 |
| §4 PLAN-CHECK hard-coded script lists | intro (found extra2/3/4 lists and extra5:622); fixed in 1.2–1.5 and 1.3 |
| §4 run core holds lets :50-64, `failedSpoken`, `starting`, `startPost`, `viewRefreshSeq`, `listedRuns` and drawer state, `stopButtons` | 1.2 Steps 4–5 (`rc` literal, `rc-rewrite.mjs`) |
| §4 stream :55-381 / start :382-614 / stop :615-894 / drawer :895-1068 | 1.2 / 1.3 / 1.4 / 1.5 |
| §4 runs.js keeps the module helpers, core and wiring; export at :1094 unchanged | 1.5 Step 5 |
| §4 external callers stay valid (detail, compare, testing, editor) | 1.5 Step 7 grep, Node and browser suites |
| §4 tests: all EasyDrag Node, static and browser tests pass; baseline unchanged per split | Test step of 1.2–1.5; browser in 1.5 |
| §14 "counter baseline committed first", "runs.js ≤ 400" | 1.1; 1.5 Step 7 (`wc -l`, about 146) |
| §9 detect-changes before each commit | Step "GitNexus detect-changes" in every task |

Deviations from the spec text, each deliberate:
- The `stopping`, `durableStopping` and `stopPolls` Sets are on the core as well, because drawer code reads them (`drawRuns`, `stepsFrom`).
- The drawer reaches `stopButtons` through `setStopPending`. `stopButtons` is defined in runs.js and put on the core.
- The constants stay at module level in runs.js and reach the parts through rc.
- `clearRun` (:1068-1075) goes with the drawer and run view.
- The two `// ── …` section headings are dropped.
- The baseline runs on the batch5 (c1d07) sandbox through `test-easydrag.mjs`. A direct run starts that runner with the guard filter, so it works standalone.

### Notes for the other parts

**Baseline.**
- Run `node scripts/test-easydrag-render-baseline.mjs` at every S0 commit after 1.1. Expected: `PASS render baseline (unchanged)`.
- Never run `--update` in S0. The spec expects no change; a change needs its own commit that says why.
- Every full `node scripts/test-easydrag.mjs` run also compares the baseline.

**Counters.**
- `ed.renderStats` keys: `cards, paints, wires, wireScans, groups, allRects, minimap, edgeNear, align`.
- `groups` counts calls of groups.js `render()` and `wireScans` counts calls of wires.js `renderFor()`. S0.5's "group re-render" must not add calls in these scenarios.
- The baseline pins the view to zoom 1. S0.4's `READABLE_ZOOM` 0.86 and the pill width must therefore leave the counts unchanged. A difference would be a real behaviour change.

**Commands.**
- Filter guards: `AURAGO_EASYDRAG_TEST_FILTER='<substring>' node scripts/test-easydrag.mjs`.
- Static Go: `go test ./ui/ -run '(?i)(EasyDrag)' -count=1`.
- Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m`.

**Registration state after Part 1.**
- module-loader.js `easydrag` scripts: four new lines (`easydrag-run-stream.js`, `-start`, `-stop`, `-drawer`) directly before `easydrag-runs.js`. Lines after the old :309 shift by 4.
- `easyDragModules` (`ui/desktop_easydrag_test.go:14`) holds the same four names before `"easydrag-runs.js"`.
- Insert Part 2 modules relative to their own neighbours: header before editor, home-grid before home, the fields parts before fields.

**Other hard-coded lists Part 2 must update** (from this part's PLAN-CHECK):
- `scripts/test-easydrag.mjs:28` loads `easydrag-home.js` (and `template`, `model`, `geometry`, `library-pause`, `dialogs`) by name.
- `scripts/test-easydrag-extra.mjs:9`, `scripts/test-easydrag-extra2.mjs:9-10`, `scripts/test-easydrag-extra3.mjs:9-11` and `scripts/test-easydrag-extra4.mjs:11-12` (`MODULES` and `UI_MODULES`) list `easydrag-fields.js`, `easydrag-home.js` and `easydrag-editor.js` where they load them.
- Content checks read module source by name:
  - `scripts/test-easydrag-extra5.mjs:619-625` reads `easydrag-saver.js`, `easydrag-editor.js` (`'aurago.easydrag.view.'`), `easydrag-runs.js`, `easydrag-groups.js` and `easydrag-core.js`.
  - The b5 script reads `easydrag-detail.js` (`read(...)`).
  - Grep `read\('easydrag-|src\('easydrag-` in `scripts/` before moving code out of editor.js, home.js or fields.js.

**Files Part 1 touches** (to avoid conflicts):
- canvas.js :54-56, :196, :656; wires.js :117; groups.js :63; interact.js :119 and :274 (one counter line each);
- `scripts/test-easydrag-ux-batch5.mjs` (one `eq` after :832);
- `scripts/test-easydrag.mjs` (header comment after :16, registration after :1138);
- `scripts/test-easydrag-extra2/3/4.mjs` load lists; `scripts/test-easydrag-extra5.mjs:622`;
- module-loader.js; `ui/desktop_easydrag_test.go:14`.

**Headroom ratchet (Part 2).**
- After 1.5, runs.js is about 146 lines (rule: at most 400).
- The four parts are about 200–340 lines each, all under 900, so none belongs on the over-900 allowlist.
- The budget loop in `TestEasyDragAppLifecycle` iterates `easyDragModules`, so it covers the parts.

**Namespaces.**
- `ED.runParts` (`stream`, `start`, `stop`, `drawer`) is taken.
- `ED.fieldsShared` (spec §4) is free.

**Scratch tools.**
- `C:/disposable/easydrag-s0/move-part.mjs` moves a block of `create()` byte for byte between two marker lines into a placeholder. It hard-codes the runs.js path and the `function create(ed, canvas) {` anchor. Copy it and change those two constants if the editor, home or fields moves want the same safety.
- `C:/disposable/easydrag-s0/rc-parts-check.mjs` is runs-specific.

**GitNexus.**
- Ambiguous names (`render`, `create`, `run`, `cancel`) return ranked candidates; pick the one in the named file.
- `detect-changes` cannot see the worktree unless `--repo` accepts the worktree path. If it cannot, report "judge by symbols + tests" as agreed.
