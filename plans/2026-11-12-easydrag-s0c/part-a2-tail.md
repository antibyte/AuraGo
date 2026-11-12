[← Plan index](README.md) · Spec: [plans/2026-11-12-easydrag-s0c-spec.md](../2026-11-12-easydrag-s0c-spec.md)

## Part A2: defect tail (spec items A2 to A7)

Part A2 continues Part A1 (start failures) in the same worktree. It fixes the stop hint (A2), the file.write card (A3), the schedule help (A4), the target sizes (A5), the plural forms (A6) and the evidence closings (A7). Every task ends in one commit.

**Shell and worktree.** Git Bash. Every command starts with `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && …`, because the working directory resets between calls. The worktree is on branch `feat/easydrag-s0c-defects` and already holds the commits of A1.1 to A1.3. Go temp directory: `GOTMPDIR='D:\tmp\go-build'` for `internal/flows` and `ui` (this part never runs `internal/server` tests). Never edit, stage or commit in the main checkout `C:/Users/Andi/Documents/repo/AuraGo`.

**Preamble (run once, before Task A2).**

- [ ] **Step 0.1: the worktree is clean**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git status --short && git branch --show-current
```
Expected: no status lines, then `feat/easydrag-s0c-defects`. If a line shows, stop and ask the coordinator; do not stash or reset.

- [ ] **Step 0.2: the JS baseline**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
```
Expected: the first command ends with `all EasyDrag checks passed`, the second prints `PASS render baseline (unchanged)`. If either fails here, the failure is not yours: report it to the coordinator before you start.

**Facts all tasks rely on (verified on main b52103d51).**

- Locale files: `ui/lang/easydrag/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`, 1,555 keys each, flat objects, two-space indent, one trailing newline (`JSON.stringify(obj, null, 2) + '\n'` reproduces each file byte for byte). The key order is mostly sorted with appended blocks, so a key goes next to its siblings, not at a fixed alphabetical place. `TestCatalogTranslationsAreComplete` (`internal/flows/catalog_i18n_test.go:34`) fails when a locale lacks an `en` key, has a key `en` lacks, or has other `{placeholder}` names than `en` for the same key. It is the guard for every locale step below.
- Placeholders are filled by `t(key, { name: value })`, which replaces `{name}`; the Node test harness `t` in `scripts/test-easydrag.mjs:48` returns `key-without-prefix:value1,value2`.
- The line ratchet: `easydrag-editor.js` is 896 lines, `easydrag-canvas.js` 928. Canvas must not grow; the editor takes only a change inside an existing line.

**One helper for all locale steps (create it in Task A2, reuse it in A3, A4, A6 and A7.2).** It lives outside the repository.

---

### Task A2: the stop hint ("durch Stopp beendet")

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-testing.js:312` (new pure function `stoppedBy` and the test panel hint, around `:285-295`)
- Modify: `ui/js/desktop/apps/easydrag-run-drawer.js:180-188` (`applyRunView`)
- Modify: `ui/js/desktop/apps/easydrag-editor.js:159-160` (`renderBanner`, inside one existing line)
- Modify: `ui/lang/easydrag/*.json` (16 files, one new key)
- Create: `scripts/test-easydrag-stop-hint.mjs`
- Create (outside the repo): `C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs`, `C:/disposable/easydrag-s0c/keys-a2-stop.json`

**What is true today (read it, do not change it).** The spec records the engine behaviour in §3.2; this task confirms it and fits the client function to the real record.

- A `logic.stop` node returns `ExecResult.Stop` (`internal/flows/nodes_logic.go:521-545`). The engine stores the node's step as an ordinary `StepRecord` with status `success` for both stop choices (`internal/flows/engine_state.go:659-672`: the `default:` branch sets `s.status[n.ID] = StepSuccess` and then `s.stop = d.result.Stop`). A stop with status "error" ends the run as `error` / `FLOW_STOPPED`; its step is still `success`. So the **run's** status must be checked as well as the step's: a stop step alone does not mean "ended by a stop with success".
- Steps not reached are closed as `skipped` (`closeOpenNodes`, `engine_state.go:776-799`), steps cut off with effects carry `effect_uncertain` (already shown as "unklar": `easydrag.ui.step_effect_uncertain`). A step that fails inside the stop's grace makes the run `error` (`conclude`, `:801-823`).
- JSON shape of a step: `{ node_id, node_key, status, effect_uncertain?, … }` (`internal/flows/run_types.go:69-90`). `GET runs/{id}?include=doc` answers `{ run, steps: [...], doc }`; the test panel holds the same data as `ed.run` (`status`, `steps: Map<node_id, step>`).

**Design.** `stoppedBy(detail, model)` lives in `easydrag-testing.js`: it is a plain IIFE module that both surfaces already load before `easydrag-run-drawer.js` (`module-loader.js:308` and `:315`) and that exports its helpers on `ED.testing`. No new module, so no loader and no harness-list change.

- `detail`: `{ run: { status }, steps: [step…], doc? }`. `model`: the editor model (`model.node(id)`, optional `model.info(type)`).
- Returns `{ step, node, label }` or `null`. A step counts when its status is `success`, **the run's status is `success`**, and its node has type `logic.stop`. The node comes from `detail.doc.nodes` when `detail.doc` exists; when `detail.doc` is missing it comes from `model.node(step.node_id)` (same id, and the type must be `logic.stop`); when neither knows the node the answer is `null`.
- Where the drawer shows it: the drawer's own list rows carry no steps, so the hint goes into the run view the drawer opens (`openRunView` → `enterRunView` → `applyRunView`). `applyRunView` stores the finished hint text in `ed.runView.stopHint`; `renderBanner` (the run view's header line, a status region) appends it. In the test result panel the hint is a `<p class="ed-hint">` above the timeline.

- [ ] **Step 1: IMPACT for the functions this task changes**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && for s in createResults applyRunView renderBanner; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|status|message)"'; done
```
Result on main: `createResults` LOW (7 impacted); `applyRunView` and `renderBanner` answer "ambiguous" (several same-named symbols; the walkable ones LOW to MEDIUM, max 17 and 18 impacted). Callers: `createResults` is called from the editor and `easydrag-blocks.js:124`; `applyRunView` only through `rc.applyRunView` in `enterRunView`; `renderBanner` from the header render. All three changes only add output; none changes a signature. If a result says HIGH or CRITICAL, warn the coordinator before editing. The index may report `"status": "behind"`; that is expected, because the index is of main.

- [ ] **Step 2: create the locale helper and the key file**

Write `C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs` with the Write tool:

```js
#!/usr/bin/env node
// Usage (cwd = the worktree): node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs <spec.json>
// spec: { "<key>": "<one text for every locale>" | { "_after": "<existing key>", "cs": "...", ..., "zh": "..." } }
// A key that exists is overwritten in place. A new key goes right after "_after" when given, else
// before the first key of its namespace (the first two dot parts) that sorts after it, else after
// the namespace's last key. The files stay in canonical form (two-space indent, trailing newline).
import fs from 'node:fs';

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const dir = 'ui/lang/easydrag/';
const spec = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const textOf = (value, lang) => (typeof value === 'string' ? value : value[lang]);

for (const [key, value] of Object.entries(spec)) {
    const missing = LOCALES.filter(l => typeof textOf(value, l) !== 'string' || !textOf(value, l).trim());
    if (missing.length) throw new Error(key + ' has no text for ' + missing.join(', '));
}
for (const lang of LOCALES) {
    const file = dir + lang + '.json';
    const raw = fs.readFileSync(file, 'utf8');
    if (JSON.stringify(JSON.parse(raw), null, 2) + '\n' !== raw) throw new Error(file + ' is not in canonical form');
    const entries = Object.entries(JSON.parse(raw));
    const inserted = new Set();
    for (const [key, value] of Object.entries(spec)) {
        const text = textOf(value, lang);
        const at = entries.findIndex(([k]) => k === key);
        if (at >= 0) { entries[at][1] = text; continue; }
        let pos = -1;
        if (value && value._after) {
            const after = entries.findIndex(([k]) => k === value._after);
            if (after < 0) throw new Error(file + ': _after ' + value._after + ' does not exist');
            pos = after + 1;
            while (pos < entries.length && inserted.has(entries[pos][0])) pos++;
        } else {
            const ns = key.split('.').slice(0, 2).join('.') + '.';
            pos = entries.findIndex(([k]) => k.startsWith(ns) && k > key);
            if (pos < 0) {
                for (let i = entries.length - 1; i >= 0; i--) if (entries[i][0].startsWith(ns)) { pos = i + 1; break; }
            }
            if (pos < 0) pos = entries.length;
        }
        entries.splice(pos, 0, [key, text]);
        inserted.add(key);
    }
    fs.writeFileSync(file, JSON.stringify(Object.fromEntries(entries), null, 2) + '\n');
}
console.log('keys written to ' + LOCALES.length + ' locale files: ' + Object.keys(spec).join(', '));
```

Write `C:/disposable/easydrag-s0c/keys-a2-stop.json`:

```json
{
  "easydrag.ui.run_stopped_by": {
    "cs": "Ukončeno krokem Stop: {step}",
    "da": "Afsluttet af stoptrin: {step}",
    "de": "Durch Stopp beendet: {step}",
    "el": "Τερματίστηκε από βήμα διακοπής: {step}",
    "en": "Ended by a stop step: {step}",
    "es": "Finalizada por un paso de parada: {step}",
    "fr": "Terminé par une étape d’arrêt : {step}",
    "hi": "स्टॉप चरण से समाप्त: {step}",
    "it": "Terminata da un passaggio di arresto: {step}",
    "ja": "停止ステップで終了: {step}",
    "nl": "Beëindigd door een stopstap: {step}",
    "no": "Avsluttet av et stopptrinn: {step}",
    "pl": "Zakończono krokiem zatrzymania: {step}",
    "pt": "Terminada por um passo de paragem: {step}",
    "sv": "Avslutad av ett stoppsteg: {step}",
    "zh": "由停止步骤结束：{step}"
  }
}
```

- [ ] **Step 3: write the failing test `scripts/test-easydrag-stop-hint.mjs`**

```js
#!/usr/bin/env node
// A2 (S0c): stoppedBy names the logic.stop step that ended a successful run, and nothing else.
// The stop step itself is "success" for both stop choices (engine_state.go), so the run's own
// status decides: a stop with status "error" and a failure inside the stop's grace end the run
// as "error" and get no hint.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console });
const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
const { stoppedBy } = box.window.EasyDrag.testing;

const TRIGGER = 'n_trigger1', WORK = 'n_workwork', STOP = 'n_stopstop';
const doc = { nodes: [
    { id: TRIGGER, type: 'trigger.manual', label: 'Start' },
    { id: WORK, type: 'web.search', label: 'Suche' },
    { id: STOP, type: 'logic.stop', label: 'Fertig' }
] };
const step = (id, status) => ({ node_id: id, status });
const model = { node: id => doc.nodes.find(n => n.id === id) || null, info: type => (type === 'logic.stop' ? { label: 'Stopp' } : null) };

// 1. A stop run: the stop step succeeded, the rest was cut off or skipped, the run is a success.
const stopRun = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success'), step(WORK, 'skipped')], doc };
const hit = stoppedBy(stopRun, model);
assert.equal(hit.step.node_id, STOP, 'the hint names the stop step');
assert.equal(hit.label, 'Fertig', 'with the label of its node in the run document');
assert.equal(stoppedBy(Object.assign({}, stopRun, { doc: { nodes: [{ id: STOP, type: 'logic.stop' }] } }), model).label, 'Stopp', 'a node without a label takes the type label');

// 2. A stop run with a failed step after the stop: the run is an error, no hint. Same for a stop
// with status "error" (FLOW_STOPPED), whose step is a success as well.
const failedAfter = { run: { status: 'error' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success'), step(WORK, 'error')], doc };
assert.equal(stoppedBy(failedAfter, model), null, 'a failure after the stop is not hidden');
assert.equal(stoppedBy({ run: { status: 'error' }, steps: [step(STOP, 'success')], doc }, model), null, 'a stop that ends the run as error shows no hint');

// 3. An ordinary run: no stop step ran (the stop node was skipped, or the flow has none).
const ordinary = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(WORK, 'success'), step(STOP, 'skipped')], doc };
assert.equal(stoppedBy(ordinary, model), null, 'a skipped stop node is no hint');
const noStopDoc = { nodes: doc.nodes.filter(n => n.id !== STOP) };
assert.equal(stoppedBy({ run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(WORK, 'success')], doc: noStopDoc }, model), null, 'a flow without a stop node');
assert.equal(stoppedBy({ run: { status: 'success' }, steps: [step(STOP, 'success')], doc: noStopDoc }, model), null, 'a document that has no such node: the model is not asked');

// 4. The document is missing (a pruned version): fall back to the editor model, else no hint.
const pruned = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success')] };
assert.equal(stoppedBy(pruned, model).label, 'Fertig', 'the model node with the same id and type stands in');
assert.equal(stoppedBy(pruned, { node: () => null }), null, 'the model does not know the node');
assert.equal(stoppedBy(pruned, { node: () => ({ id: STOP, type: 'web.search' }) }), null, 'the model node has another type');
assert.equal(stoppedBy(pruned, null), null, 'no model at all');
assert.equal(stoppedBy(null, model), null, 'no detail');
assert.equal(stoppedBy({ run: { status: 'success' } }, model), null, 'no steps');
console.log('easydrag stop hint: ok');
```

- [ ] **Step 4: run it and see it fail for the right reason**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag-stop-hint.mjs 2>&1 | head -6
```
Expected: it stops with `TypeError: stoppedBy is not a function` (the module does not export it yet).

- [ ] **Step 5: implement `stoppedBy` in `easydrag-testing.js`**

Edit `ui/js/desktop/apps/easydrag-testing.js`. Replace the last line before the closing `})();` (the export line, currently line 312):

old:
```js
    ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects, simulationDefault, localSample };
```
new:
```js
    // stoppedBy returns the logic.stop step that ended a successful run, or null (S0c A2). The
    // stop step is a success for both stop choices, so the run's own status decides: a stop with
    // status "error" and a failure inside the stop's grace end the run as an error and get no hint.
    // detail is { run: { status }, steps: [step], doc? } as GET runs/{id}?include=doc answers it.
    // The node comes from detail.doc; a run whose document is gone (a pruned version) takes the
    // model node with the same id, and a node that is nowhere known gives null. The result is
    // { step, node, label }; label is the node's own label, else its type's label, else the type.
    function stoppedBy(detail, model) {
        const run = detail && detail.run;
        if (!run || run.status !== 'success' || !Array.isArray(detail.steps)) return null;
        const docNodes = detail.doc && Array.isArray(detail.doc.nodes) ? detail.doc.nodes : null;
        for (const step of detail.steps) {
            if (!step || step.status !== 'success') continue;
            const node = docNodes ? docNodes.find(n => n && n.id === step.node_id) : (model && typeof model.node === 'function' ? model.node(step.node_id) : null);
            if (!node || node.type !== 'logic.stop') continue;
            const info = model && typeof model.info === 'function' ? model.info(node.type) : null;
            return { step, node, label: node.label || (info && info.label) || node.type };
        }
        return null;
    }
    ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects, simulationDefault, localSample, stoppedBy };
```

- [ ] **Step 6: run the test and see it pass**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag-stop-hint.mjs
```
Expected: `easydrag stop hint: ok`.

- [ ] **Step 7: show the hint in the test result panel**

In `ui/js/desktop/apps/easydrag-testing.js`, `createResults`' `render`. After the line `const failed = steps.find(step => step.status === 'error');` add one line, and after the `(simulation ? '<p class="ed-hint">' …` markup line add one markup line.

old (one line, unique):
```js
            const failed = steps.find(step => step.status === 'error');
```
new:
```js
            const failed = steps.find(step => step.status === 'error');
            const stopped = stoppedBy({ run, steps }, ed.model);
```
old (the markup line; it starts with 16 spaces):
```js
                (simulation ? '<p class="ed-hint">' + esc(t(ed.isBlock ? 'easydrag.ui.block_simulation_hint' : 'easydrag.ui.simulation_hint')) + '</p>' : '') +
```
new (same line, then the new one):
```js
                (simulation ? '<p class="ed-hint">' + esc(t(ed.isBlock ? 'easydrag.ui.block_simulation_hint' : 'easydrag.ui.simulation_hint')) + '</p>' : '') +
                (stopped ? '<p class="ed-hint" data-ed-stopped-by>' + esc(t('easydrag.ui.run_stopped_by', { step: stopped.label })) + '</p>' : '') +
```
`run` is `ed.run` (`status`, `steps` Map, read at the top of `render`); `steps` is the sorted array built five lines above.

- [ ] **Step 8: show the hint in the run view opened from the drawer**

`ui/js/desktop/apps/easydrag-run-drawer.js`, `applyRunView` (line 186):

old:
```js
            if (ed.runView) ed.runView.data = runData(detail.run, detail.steps || []);
```
new:
```js
            if (ed.runView) {
                ed.runView.data = runData(detail.run, detail.steps || []);
                // The hint of a run a stop step ended (S0c A2); renderBanner appends it to the banner text.
                const stopped = ED.testing && ED.testing.stoppedBy(detail, ed.model);
                ed.runView.stopHint = stopped ? t('easydrag.ui.run_stopped_by', { step: stopped.label }) : '';
            }
```
`ed.runView` is replaced by `enterRunView` for every run and set to `null` on exit, so the hint never outlives its run. `ed.model` is already the model of the viewed document when `applyRunView` runs (`enterRunView` sets it first).

`ui/js/desktop/apps/easydrag-editor.js`, `renderBanner` (around line 160). One existing line changes; the file keeps its 896 lines:

old:
```js
            if (node.textContent !== text) node.textContent = text;
```
new:
```js
            const shown = text + (ed.runView.stopHint ? ' · ' + ed.runView.stopHint : ''); if (node.textContent !== shown) node.textContent = shown;
```
(`renderBanner` returns early when `ed.runView` is null, so `ed.runView.stopHint` is safe here.)

- [ ] **Step 9: the locale key**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs C:/disposable/easydrag-s0c/keys-a2-stop.json && git diff --stat -- ui/lang/easydrag | tail -1
```
Expected: `keys written to 16 locale files: easydrag.ui.run_stopped_by`, then `16 files changed, 16 insertions(+)`.

- [ ] **Step 10: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && wc -l ui/js/desktop/apps/easydrag-editor.js && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCatalogTranslationsAreComplete' -count=1
```
Expected: `all EasyDrag checks passed`; `PASS render baseline (unchanged)`; `896 ui/js/desktop/apps/easydrag-editor.js`; `ok  	aurago/internal/flows`. If `test-easydrag.mjs` shows a `FAIL` line for the test panel or the run view, read its `got … want …` detail: a changed `innerHTML` of the results panel means the hint appeared in a fixture whose steps include a stop; update that one expectation only if the new hint is the sole difference.

- [ ] **Step 11: commit**

Write `C:/disposable/easydrag-s0c/msg-a2.txt` with the Write tool:
```
feat(easydrag): show "ended by a stop step" for a run a stop node ended

stoppedBy(detail, model) returns the logic.stop step of a successful run. The
run view (opened from the run drawer) and the test result panel show the hint
with the step's label. A stop with status error and a failure inside the stop's
grace end the run as an error and get no hint; a run whose document was pruned
falls back to the editor model. No engine change (owner decision 1).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add ui/js/desktop/apps/easydrag-testing.js ui/js/desktop/apps/easydrag-run-drawer.js ui/js/desktop/apps/easydrag-editor.js scripts/test-easydrag-stop-hint.mjs ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-s0c/msg-a2.txt && git status --short
```
Expected: one commit, `git status --short` empty. (`ui/lang/easydrag/*.json` expands to the 16 locale files.)

---

### Task A3: file.write shows "überschreibt" on the card

**Files:**
- Modify: `internal/flows/node.go:68-72` (`Option` gets `CardLabelKey`)
- Modify: `internal/flows/nodes_logic.go:57-59` (new helper `cardOption` next to `option`)
- Modify: `internal/flows/catalog.go:112-115` and `:354-356` (`OptionInfo.CardLabel`, resolved in `describeParam`)
- Modify: `internal/flows/templates_builtin.go:342-344` (`CatalogI18nKeys` lists the card keys)
- Modify: `internal/flows/catalog_actions_docs.go:392-393` (only file.write's `if_exists`)
- Modify: `ui/js/desktop/apps/easydrag-canvas.js:351` (one existing line; the file stays at 928 lines)
- Modify: `ui/lang/easydrag/*.json` (16 files: 3 new card keys, `easydrag.node.file_write.summary` changed)
- Create: `internal/flows/catalog_actions_docs_default_test.go`
- Create (outside the repo): `C:/disposable/easydrag-s0c/keys-a3-card.json`

**Facts.**
- `if_exists` has the values `overwrite`, `unique` and `fail` (not "rename"); its option labels are `easydrag.option.if_exists_overwrite`, `…_unique` and `…_fail` ("Überschreiben", "Beide behalten (neuer Name)", "Mit Fehler abbrechen"). The card words are the **short** forms of the same three values: "überschreibt", "neuer Name", "bricht ab".
- The node-types API sends the output of `describeParam` (`catalog.go:348-358`): each option becomes `OptionInfo{Value, Label}`, the label resolved by `localize(tr, o.LabelKey, o.Label)`. That function is the "handler that resolves option labels"; there is no separate HTTP handler logic.
- `CatalogI18nKeys` (`templates_builtin.go:328`) lists every key the catalog uses; `TestCatalogTranslationsAreComplete` requires them in `en.json`, and requires all other locales to match `en`'s key set. Listing the card keys there is what makes the test cover them.
- The card's second line is the node type's summary template; `summary()` in `easydrag-canvas.js:376-380` replaces `{name}` with `displayValue(n, i, name, max)`, and `displayValue` already falls back to the param's default when the node has no value (`:347`).

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && for s in displayValue describeParam CatalogI18nKeys fileWriteDef; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|status)"'; done
```
Result on main: `displayValue` LOW (4 impacted; its only caller is `summary`), `describeParam` LOW (4; its caller at `catalog.go:325` and itself for nested fields). `CatalogI18nKeys` reports `UNKNOWN` with 0 callers because the index cannot see its test callers: confirm with `grep -rn "CatalogI18nKeys" internal cmd | grep -v "^internal/flows/templates_builtin.go"` (expect the catalog i18n tests and the server's locale check, if any), and treat it as unresolved until the full `go test ./internal/flows/...` below is green. `fileWriteDef` is a catalog registration function; its only change is two option constructors.

- [ ] **Step 2: write the failing Go tests**

Create `internal/flows/catalog_actions_docs_default_test.go`:
```go
package flows

import "testing"

// file.write keeps its default "overwrite" (owner decision 2 of the S0c spec). Two things pin it:
// the parameter's declared default, and the behaviour of a node that sets no if_exists: it writes
// over an existing file, with one write call and no probe for the name.
func TestFileWriteDefaultsToOverwrite(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	var spec *ParamSpec
	for i := range def.Params {
		if def.Params[i].Name == "if_exists" {
			spec = &def.Params[i]
		}
	}
	if spec == nil || spec.Default != "overwrite" {
		t.Fatalf("if_exists spec = %+v, want default %q", spec, "overwrite")
	}

	tools := existsFake(func(string) bool { return true }) // the file already exists
	res, err := execDef(def, map[string]any{"path": "a.txt", "content": "x"}, &Services{Tools: tools})
	if err != nil {
		t.Fatalf("a node without if_exists must overwrite an existing file, got %v", err)
	}
	calls := tools.allCalls()
	writes, probes := opsOf(calls, "write_file"), opsOf(calls, "stat")
	if len(calls) != 1 || len(writes) != 1 || writes[0] != "a.txt" || len(probes) != 0 {
		t.Fatalf("calls = %d, writes = %q, probes = %q; want one write of a.txt and no probe", len(calls), writes, probes)
	}
	if got := res.Output["file"].(map[string]any)["path"]; got != "a.txt" {
		t.Fatalf("output path = %v, want a.txt (no renamed copy)", got)
	}
}

// Only file.write's if_exists carries card labels; they resolve like the option labels and are
// sent as card_label. Without a translation the card falls back to nothing (the client then uses
// the option label).
func TestFileWriteOptionsCarryCardLabels(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	tr := func(key string) string { return "T:" + key }
	for _, p := range def.Params {
		if p.Name != "if_exists" {
			continue
		}
		info := describeParam(p, tr)
		want := map[string]string{
			"overwrite": "T:easydrag.option.if_exists_overwrite.card",
			"unique":    "T:easydrag.option.if_exists_unique.card",
			"fail":      "T:easydrag.option.if_exists_fail.card",
		}
		if len(info.Options) != len(want) {
			t.Fatalf("options = %+v", info.Options)
		}
		for _, o := range info.Options {
			if o.CardLabel != want[o.Value] || o.Label == "" {
				t.Errorf("option %s: card label %q, label %q; want card label %q", o.Value, o.CardLabel, o.Label, want[o.Value])
			}
		}
	}
	// Every other option in the catalog keeps no card label.
	reg := catalogRegistry(t, fullEnv())
	for _, d := range reg.All() {
		if d.Type == TypeFileWrite {
			continue
		}
		for _, p := range d.Params {
			for _, o := range p.Options {
				if o.CardLabelKey != "" {
					t.Errorf("%s.%s option %s has a card label key", d.Type, p.Name, o.Value)
				}
			}
		}
	}
}
```
`lookupDef`, `docRegistry`, `existsFake`, `opsOf` and `execDef` are the helpers the neighbouring tests use (`catalog_helpers_test.go:113-134`, `catalog_actions_docs_hardening_test.go:29-60`); `catalogRegistry` and `fullEnv` are used by `catalog_i18n_test.go:35`.

- [ ] **Step 3: run them and see them fail for the stated reason**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestFileWriteDefaultsToOverwrite|TestFileWriteOptionsCarryCardLabels' -count=1 2>&1 | head -12
```
Expected: a compile failure naming `o.CardLabel undefined (type OptionInfo has no field or method CardLabel)` and `o.CardLabelKey undefined`. (`TestFileWriteDefaultsToOverwrite` would pass on its own once it compiles: it pins today's behaviour, so it must stay green through the whole task.)

- [ ] **Step 4: implement the Go side**

`internal/flows/node.go`:
old:
```go
type Option struct {
	Value    string `json:"value"`
	LabelKey string `json:"label_key,omitempty"`
	Label    string `json:"label,omitempty"`
}
```
new:
```go
type Option struct {
	Value    string `json:"value"`
	LabelKey string `json:"label_key,omitempty"`
	Label    string `json:"label,omitempty"`
	// CardLabelKey is an optional short word for the node card's second line (the control keeps
	// LabelKey). Only file.write's if_exists sets it; the node-types API sends it as card_label.
	CardLabelKey string `json:"card_label_key,omitempty"`
}
```

`internal/flows/nodes_logic.go`, after `option`:
```go
// cardOption is option with a short card word under "<label key>.card".
func cardOption(value, key string) Option {
	o := option(value, key)
	o.CardLabelKey = o.LabelKey + ".card"
	return o
}
```

`internal/flows/catalog.go`:
old:
```go
type OptionInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
```
new:
```go
type OptionInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
	// CardLabel is the short word the node card shows for this value; empty when the option has none.
	CardLabel string `json:"card_label,omitempty"`
}
```
and in `describeParam`:
old:
```go
		info.Options = append(info.Options, OptionInfo{Value: o.Value, Label: firstNonEmpty(localize(tr, o.LabelKey, o.Label), o.Value)})
```
new:
```go
		oi := OptionInfo{Value: o.Value, Label: firstNonEmpty(localize(tr, o.LabelKey, o.Label), o.Value)}
		if o.CardLabelKey != "" {
			oi.CardLabel = localize(tr, o.CardLabelKey, "")
		}
		info.Options = append(info.Options, oi)
```
(`localize` returns the key itself when there is no translation; for a card label that is wrong, because the card would show a raw key. Guard it: after the call add `if oi.CardLabel == o.CardLabelKey { oi.CardLabel = "" }`.) Write the block as:
```go
		oi := OptionInfo{Value: o.Value, Label: firstNonEmpty(localize(tr, o.LabelKey, o.Label), o.Value)}
		if o.CardLabelKey != "" {
			if card := localize(tr, o.CardLabelKey, ""); card != o.CardLabelKey {
				oi.CardLabel = card
			}
		}
		info.Options = append(info.Options, oi)
```

`internal/flows/templates_builtin.go`, in `addParam`:
old:
```go
		for _, o := range p.Options {
			add(o.LabelKey)
		}
```
new:
```go
		for _, o := range p.Options {
			add(o.LabelKey)
			add(o.CardLabelKey)
		}
```

`internal/flows/catalog_actions_docs.go` (`fileWriteDef`):
old:
```go
			Options: []Option{option("overwrite", "if_exists_overwrite"), option("unique", "if_exists_unique"), option("fail", "if_exists_fail")}},
```
new:
```go
			Options: []Option{cardOption("overwrite", "if_exists_overwrite"), cardOption("unique", "if_exists_unique"), cardOption("fail", "if_exists_fail")}},
```
Check that the line you replace is the one inside `fileWriteDef` (`grep -n 'option("overwrite", "if_exists_overwrite")' internal/flows/*.go` must show exactly one hit, in `catalog_actions_docs.go`; `ifExistsOps` elsewhere does not use `option`).

- [ ] **Step 5: the locale keys (card words and the summary)**

Write `C:/disposable/easydrag-s0c/keys-a3-card.json`:
```json
{
  "easydrag.option.if_exists_overwrite.card": {
    "_after": "easydrag.option.if_exists_overwrite",
    "cs": "přepíše", "da": "overskriver", "de": "überschreibt", "el": "αντικαθιστά", "en": "overwrites", "es": "sobrescribe",
    "fr": "écrase", "hi": "ओवरराइट करता है", "it": "sovrascrive", "ja": "上書き", "nl": "overschrijft", "no": "overskriver",
    "pl": "nadpisuje", "pt": "substitui", "sv": "skriver över", "zh": "覆盖"
  },
  "easydrag.option.if_exists_unique.card": {
    "_after": "easydrag.option.if_exists_unique",
    "cs": "nový název", "da": "nyt navn", "de": "neuer Name", "el": "νέο όνομα", "en": "new name", "es": "nombre nuevo",
    "fr": "nouveau nom", "hi": "नया नाम", "it": "nuovo nome", "ja": "新しい名前", "nl": "nieuwe naam", "no": "nytt navn",
    "pl": "nowa nazwa", "pt": "novo nome", "sv": "nytt namn", "zh": "新名称"
  },
  "easydrag.option.if_exists_fail.card": {
    "_after": "easydrag.option.if_exists_fail",
    "cs": "přeruší", "da": "afbryder", "de": "bricht ab", "el": "διακόπτει", "en": "aborts", "es": "cancela",
    "fr": "interrompt", "hi": "रुक जाता है", "it": "si interrompe", "ja": "中止", "nl": "stopt", "no": "avbryter",
    "pl": "przerywa", "pt": "cancela", "sv": "avbryter", "zh": "中止"
  },
  "easydrag.node.file_write.summary": "{path} · {if_exists}"
}
```
The last entry is a plain string, so all 16 locales get the same text; the key exists, so the helper replaces it in place (line 62 of each file).
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs C:/disposable/easydrag-s0c/keys-a3-card.json && git diff --stat -- ui/lang/easydrag | tail -1 && grep -n 'file_write.summary' ui/lang/easydrag/de.json ui/lang/easydrag/ja.json
```
Expected: `16 files changed, 64 insertions(+), 16 deletions(-)` (3 new keys and one replaced line per file), and both grep lines show `"{path} · {if_exists}"`.

- [ ] **Step 6: `displayValue` prefers the card word (canvas.js must not grow)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && wc -l ui/js/desktop/apps/easydrag-canvas.js
```
Expected: `928 ui/js/desktop/apps/easydrag-canvas.js`. Note the number.

`ui/js/desktop/apps/easydrag-canvas.js:351`:
old:
```js
                if (opt) return opt.label;
```
new:
```js
                if (opt) return opt.card_label || opt.label;
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && wc -l ui/js/desktop/apps/easydrag-canvas.js
```
Expected: still `928`.

- [ ] **Step 7: run the tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestFileWriteDefaultsToOverwrite|TestFileWriteOptionsCarryCardLabels|TestCatalogTranslationsAreComplete' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: three `--- PASS` lines and `ok  	aurago/internal/flows`.

- [ ] **Step 8: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows/ && gofmt -l internal/flows/node.go internal/flows/nodes_logic.go internal/flows/catalog.go internal/flows/templates_builtin.go internal/flows/catalog_actions_docs.go internal/flows/catalog_actions_docs_default_test.go && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... -count=1 2>&1 | tail -3 && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: `go vet` and `gofmt -l` print nothing; the test run ends `ok  	aurago/internal/flows` (and any sub-packages `ok`); `all EasyDrag checks passed`; `PASS render baseline (unchanged)`. If the baseline script fails, read its diff: the only acceptable change is a file.write card whose second line gains the overwrite word; then run `AURAGO_EASYDRAG_RENDER_BASELINE=update node scripts/test-easydrag-render-baseline.mjs` (the script reads that variable, line 147), re-run without it, and add `scripts/easydrag-render-baseline.json` to the commit. Any other change is a regression: stop and report it.

- [ ] **Step 9: commit**

Write `C:/disposable/easydrag-s0c/msg-a3.txt`:
```
feat(easydrag): show what file.write does when the file exists on its card

Option gets an optional card label key; describeParam resolves it like the label
and sends card_label. Only file.write's if_exists sets it ("überschreibt",
"neuer Name", "bricht ab" and the 15 other locales). displayValue prefers
card_label by changing its existing line, so canvas.js does not grow. The
summary becomes "{path} · {if_exists}". TestFileWriteDefaultsToOverwrite pins
the default (owner decision 2).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/flows/node.go internal/flows/nodes_logic.go internal/flows/catalog.go internal/flows/templates_builtin.go internal/flows/catalog_actions_docs.go internal/flows/catalog_actions_docs_default_test.go ui/js/desktop/apps/easydrag-canvas.js ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-s0c/msg-a3.txt && git status --short
```
Expected: one commit; `git status --short` empty (if Step 8 updated the render baseline, add `scripts/easydrag-render-baseline.json` to the `git add`).

---

### Task A4: schedule help (minutes, hours, day)

**Files:**
- Modify: `internal/flows/catalog_schedule.go:37-45` (`scheduleParams`: three `HelpKey`s)
- Modify: `ui/lang/easydrag/*.json` (16 files, 3 new keys)
- Create (outside the repo): `C:/disposable/easydrag-s0c/keys-a4-schedule.json`

**Facts.** `ParamSpec` already has `HelpKey` (`easydrag.help.file_write_path` uses it); `describeParam` resolves it into `ParamInfo.Help`, and `easydrag-forms.js:369` shows it as the info icon with a tooltip. `CatalogI18nKeys` already adds `p.HelpKey`, so `TestCatalogTranslationsAreComplete` covers the new keys (the spec's "no new test"). The divisor lists come from `scheduleInterval` (`catalog_schedule.go:137-157`: the interval must divide the period, so 60 gives 1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30 and 24 gives 1, 2, 3, 4, 6, 8, 12). The server's English error stays as the detail behind the field's issue (not changed).

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "scheduleParams" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"'
```
Result on main: LOW, 7 impacted (the schedule trigger and timer definitions that read the params). The change adds a field value only.

- [ ] **Step 2: confirm the divisor lists by running the real code (not by trusting the text)**

Run the existing schedule tests and read the divisor lists they pin:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestScheduleToCron' -count=1 -v 2>&1 | grep -E '^(ok|FAIL)' | head -2; grep -n 'must divide' internal/flows/catalog_schedule_test.go | head -3
```
Expected: `ok`, and two lines from `catalog_schedule_test.go:196-197`: `minutes must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30)` and `hours must divide 24 (1, 2, 3, 4, 6, 8, 12)`. If the listed divisors differ from the three lists in the help text below, fix the text, not the code.

- [ ] **Step 3: the help keys in the catalog**

`internal/flows/catalog_schedule.go` (`scheduleParams`), three params:

old:
```go
		{Name: "minutes", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_minutes", Required: true, Default: 15.0, VisibleIf: when("interval_minutes")},
		{Name: "hours", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_hours", Required: true, Default: 1.0, VisibleIf: when("interval_hours")},
```
new:
```go
		{Name: "minutes", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_minutes", HelpKey: "easydrag.help.schedule_minutes", Required: true, Default: 15.0, VisibleIf: when("interval_minutes")},
		{Name: "hours", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_hours", HelpKey: "easydrag.help.schedule_hours", Required: true, Default: 1.0, VisibleIf: when("interval_hours")},
```
old:
```go
		{Name: "day", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_day", Required: true, Default: 1.0, VisibleIf: when("monthly")},
```
new:
```go
		{Name: "day", Kind: ParamNumber, LabelKey: "easydrag.param.schedule_day", HelpKey: "easydrag.help.schedule_day", Required: true, Default: 1.0, VisibleIf: when("monthly")},
```

- [ ] **Step 4: run the i18n test and see it fail (keys not in the locales yet)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCatalogTranslationsAreComplete' -count=1 2>&1 | head -5
```
Expected: `FAIL`, with `en.json misses easydrag.help.schedule_minutes` (and the hours and day keys).

- [ ] **Step 5: the locale keys**

Write `C:/disposable/easydrag-s0c/keys-a4-schedule.json`:
```json
{
  "easydrag.help.schedule_minutes": {
    "cs": "Povolené hodnoty: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 nebo 30. Interval musí dělit hodinu beze zbytku.",
    "da": "Tilladte værdier: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 eller 30. Intervallet skal gå op i timen.",
    "de": "Erlaubt sind 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 oder 30. Der Abstand muss die Stunde ohne Rest teilen.",
    "el": "Επιτρεπτές τιμές: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 ή 30. Το διάστημα πρέπει να διαιρεί την ώρα ακριβώς.",
    "en": "Allowed values: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 or 30. The interval must divide the hour evenly.",
    "es": "Valores permitidos: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 o 30. El intervalo debe dividir la hora de forma exacta.",
    "fr": "Valeurs autorisées : 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 ou 30. L’intervalle doit diviser l’heure sans reste.",
    "hi": "मान्य मान: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 या 30। अंतराल घंटे को बिना शेष के बाँटना चाहिए।",
    "it": "Valori consentiti: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 o 30. L’intervallo deve dividere l’ora senza resto.",
    "ja": "指定できる値は 1、2、3、4、5、6、10、12、15、20、30 です。間隔は 1 時間を割り切れる必要があります。",
    "nl": "Toegestane waarden: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 of 30. Het interval moet het uur zonder rest delen.",
    "no": "Tillatte verdier: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 eller 30. Intervallet må gå opp i timen.",
    "pl": "Dozwolone wartości: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 lub 30. Interwał musi dzielić godzinę bez reszty.",
    "pt": "Valores permitidos: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 ou 30. O intervalo tem de dividir a hora sem resto.",
    "sv": "Tillåtna värden: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 eller 30. Intervallet måste gå jämnt upp i timmen.",
    "zh": "可用值：1、2、3、4、5、6、10、12、15、20 或 30。间隔必须能整除一小时。"
  },
  "easydrag.help.schedule_hours": {
    "cs": "Povolené hodnoty: 1, 2, 3, 4, 6, 8 nebo 12. Interval musí dělit den beze zbytku.",
    "da": "Tilladte værdier: 1, 2, 3, 4, 6, 8 eller 12. Intervallet skal gå op i døgnet.",
    "de": "Erlaubt sind 1, 2, 3, 4, 6, 8 oder 12. Der Abstand muss den Tag ohne Rest teilen.",
    "el": "Επιτρεπτές τιμές: 1, 2, 3, 4, 6, 8 ή 12. Το διάστημα πρέπει να διαιρεί το 24ωρο ακριβώς.",
    "en": "Allowed values: 1, 2, 3, 4, 6, 8 or 12. The interval must divide the day evenly.",
    "es": "Valores permitidos: 1, 2, 3, 4, 6, 8 o 12. El intervalo debe dividir el día de forma exacta.",
    "fr": "Valeurs autorisées : 1, 2, 3, 4, 6, 8 ou 12. L’intervalle doit diviser la journée sans reste.",
    "hi": "मान्य मान: 1, 2, 3, 4, 6, 8 या 12। अंतराल दिन को बिना शेष के बाँटना चाहिए।",
    "it": "Valori consentiti: 1, 2, 3, 4, 6, 8 o 12. L’intervallo deve dividere il giorno senza resto.",
    "ja": "指定できる値は 1、2、3、4、6、8、12 です。間隔は 1 日を割り切れる必要があります。",
    "nl": "Toegestane waarden: 1, 2, 3, 4, 6, 8 of 12. Het interval moet de dag zonder rest delen.",
    "no": "Tillatte verdier: 1, 2, 3, 4, 6, 8 eller 12. Intervallet må gå opp i døgnet.",
    "pl": "Dozwolone wartości: 1, 2, 3, 4, 6, 8 lub 12. Interwał musi dzielić dobę bez reszty.",
    "pt": "Valores permitidos: 1, 2, 3, 4, 6, 8 ou 12. O intervalo tem de dividir o dia sem resto.",
    "sv": "Tillåtna värden: 1, 2, 3, 4, 6, 8 eller 12. Intervallet måste gå jämnt upp i dygnet.",
    "zh": "可用值：1、2、3、4、6、8 或 12。间隔必须能整除一天。"
  },
  "easydrag.help.schedule_day": {
    "cs": "Pokud měsíc tento den nemá (29, 30 nebo 31), běh se v tom měsíci přeskočí. Nepřesouvá se na poslední den měsíce.",
    "da": "Hvis en måned ikke har denne dag (29, 30 eller 31), springes kørslen over i den måned. Den flyttes ikke til månedens sidste dag.",
    "de": "Hat ein Monat diesen Tag nicht (29, 30 oder 31), fällt der Lauf in diesem Monat aus. Er rückt nicht auf den letzten Tag des Monats vor.",
    "el": "Αν ένας μήνας δεν έχει αυτή την ημέρα (29, 30 ή 31), η εκτέλεση παραλείπεται εκείνο τον μήνα. Δεν μετακινείται στην τελευταία ημέρα του μήνα.",
    "en": "A month without this day (29, 30 or 31) skips the run. It does not move to the last day of the month.",
    "es": "Si un mes no tiene ese día (29, 30 o 31), la ejecución se omite ese mes. No se traslada al último día del mes.",
    "fr": "Si un mois n’a pas ce jour (29, 30 ou 31), l’exécution est sautée ce mois-là. Elle n’est pas reportée au dernier jour du mois.",
    "hi": "यदि किसी महीने में यह दिन नहीं है (29, 30 या 31), तो उस महीने रन छोड़ दिया जाता है। वह महीने के आख़िरी दिन पर नहीं खिसकता।",
    "it": "Se un mese non ha quel giorno (29, 30 o 31), l’esecuzione viene saltata in quel mese. Non si sposta all’ultimo giorno del mese.",
    "ja": "その日がない月（29、30、31 日）では、その月の実行はスキップされます。月末日には移動しません。",
    "nl": "Heeft een maand deze dag niet (29, 30 of 31), dan wordt de uitvoering in die maand overgeslagen. Hij schuift niet door naar de laatste dag van de maand.",
    "no": "Hvis en måned ikke har denne dagen (29, 30 eller 31), hoppes kjøringen over den måneden. Den flyttes ikke til månedens siste dag.",
    "pl": "Jeśli miesiąc nie ma tego dnia (29, 30 lub 31), uruchomienie w tym miesiącu jest pomijane. Nie przesuwa się na ostatni dzień miesiąca.",
    "pt": "Se um mês não tiver esse dia (29, 30 ou 31), a execução é ignorada nesse mês. Não passa para o último dia do mês.",
    "sv": "Om en månad saknar dagen (29, 30 eller 31) hoppas körningen över den månaden. Den flyttas inte till månadens sista dag.",
    "zh": "如果某个月没有这一天（29、30 或 31 日），该月的运行会被跳过，不会顺延到当月最后一天。"
  }
}
```
These are inserted in the `easydrag.help.` namespace (no `_after`): the helper puts each before the first help key that sorts after it.
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs C:/disposable/easydrag-s0c/keys-a4-schedule.json && git diff --stat -- ui/lang/easydrag | tail -1
```
Expected: `16 files changed, 48 insertions(+)`.

- [ ] **Step 6: verify the day rule is true before the text ships**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && grep -n 'monthly' internal/flows/catalog_schedule.go | head -5 && grep -n 'monthly\|31\|last day' internal/flows/catalog_schedule_test.go | head -8
```
Expected: the monthly mode builds `<minute> <hour> <day> * *` (standard cron, where day 31 simply does not match in a 30-day month). If the code or a test says the run moves to the last day, stop and report: the text in `schedule_day` would be wrong, and the owner's decision needs checking.

- [ ] **Step 7: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows/ && gofmt -l internal/flows/catalog_schedule.go && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCatalogTranslationsAreComplete|TestScheduleToCron|TestTemplatesAreValidInEveryLocale' -count=1 2>&1 | tail -2 && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: nothing from vet or gofmt; `ok  	aurago/internal/flows`; `all EasyDrag checks passed`; `PASS render baseline (unchanged)`.

- [ ] **Step 8: commit**

Write `C:/disposable/easydrag-s0c/msg-a4.txt`:
```
feat(easydrag): explain the valid schedule intervals and the day-of-month rule

The schedule trigger's minutes, hours and day fields get help text in all 16
locales: the intervals that divide the hour (1, 2, 3, 4, 5, 6, 10, 12, 15, 20,
30) and the day (1, 2, 3, 4, 6, 8, 12), and that a month without day 29, 30 or
31 skips the run instead of moving it to the last day. The server's English
error stays as the detail behind the field's issue.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/flows/catalog_schedule.go ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-s0c/msg-a4.txt && git status --short
```
Expected: one commit, clean status.

---

### Task A5: target sizes (U9, V4)

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css:356` (`.ed-seg--small button`), `:753-755` (`.ed-port-add`), the comments above `:370-371` and `:468-469`

CSS only. The browser test that measures every control lands in Part B (its target-size task, after A5 is on main); this task states the exception list exactly as that test reads it.

**The exception list (Part B reads exactly this).** A control at or above 24×24 CSS px passes; a control below that passes only when it matches one of these selectors (`Element.matches`). Inline links inside running text are exempt (WCAG 2.5.8). Two entries, nothing else:

| Selector | Reason |
|---|---|
| `.ed-switch--small` | the visible switch is 30×17 px, its `::before` hit area is 34×25 px (U9) |
| `.ed-flow-select input` | the 20×20 px checkbox sits inside the 44×44 px `.ed-flow-select` label, which is the target |

Part B should carry the list as Go data of the same shape, in this order:
```go
var easyDragTargetSizeExceptions = []struct{ Selector, Reason string }{
	{".ed-switch--small", "the visible switch is 30x17, its ::before hit area is 34x25"},
	{".ed-flow-select input", "the 20x20 checkbox sits in the 44x44 label that is the target"},
}
```
If Part B's measurement finds a third control below 24 px in the main editor fixture, Part B fixes it in the CSS in its own commit after Part A has landed (the README decides this); it does not add an exception on its own.

- [ ] **Step 1: IMPACT**

CSS has no symbol graph. List the users of both selectors instead:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && grep -rn "ed-seg--small\|ed-port-add" ui/js/desktop/apps/*.js | cut -c1-140 | head -12; grep -n "ed-port-add\|ed-seg--small" ui/css/desktop-app-easydrag.css | cut -c1-120
```
Expected: `.ed-seg--small` is used by the run drawer's filter (`easydrag-run-drawer.js:120`) and a few segmented controls; `.ed-port-add` is the "add a step" button on a card port (`data-ed-port-add`). Both phone rules (`:1396-1403`, 44 px) stay as they are and still win on a phone.

- [ ] **Step 2: raise the two controls**

`ui/css/desktop-app-easydrag.css`:

old:
```css
.ed-seg--small button { min-height: 22px; padding: 2px 10px; font-size: 12px; }
```
new:
```css
.ed-seg--small button { min-height: 24px; padding: 2px 10px; font-size: 12px; }
```
old:
```css
    position: absolute; left: 14px; top: 50%; width: 22px; height: 22px; padding: 0; margin-top: -11px;
```
new:
```css
    position: absolute; left: 14px; top: 50%; width: 24px; height: 24px; padding: 0; margin-top: -12px;
```
The line is the first line of the `.ed-port-add` rule (line 754); the 13 px icon inside stays.

- [ ] **Step 3: write the two exceptions as comments**

old:
```css
/* U9: the small switch keeps its look; a pseudo element widens its target to 34 × 25 px */
```
new:
```css
/* U9: the small switch keeps its look; a pseudo element widens its target to 34 × 25 px.
   Target-size exception (S0c A5, WCAG 2.5.8): `.ed-switch--small` is listed in the browser
   target-size test because the visible switch is 30 × 17 px and the hit area is the ::before. */
```
old:
```css
.ed-flow-select input { width: 20px; height: 20px; margin: 0; accent-color: var(--ed-accent); }
```
new:
```css
/* Target-size exception (S0c A5, WCAG 2.5.8): `.ed-flow-select input` is listed in the browser
   target-size test because the 20 × 20 px checkbox sits inside the 44 × 44 px label above, which is the target. */
.ed-flow-select input { width: 20px; height: 20px; margin: 0; accent-color: var(--ed-accent); }
```

- [ ] **Step 4: check the result and the CSS-reading tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && grep -n "min-height: 24px; padding: 2px 10px\|width: 24px; height: 24px; padding: 0; margin-top: -12px\|S0c A5" ui/css/desktop-app-easydrag.css | cut -c1-110 && node scripts/test-easydrag.mjs 2>&1 | tail -1 && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -2
```
Expected: four matching lines (the two raised rules and the two comments); `all EasyDrag checks passed`; `ok  	aurago/ui` (the browser tests skip because `AURAGO_RUN_BROWSER_SMOKE` is unset). Visual check is not part of this task: Part B's target-size test is the proof, and the 2 px growth fits the existing paddings.

- [ ] **Step 5: commit**

Write `C:/disposable/easydrag-s0c/msg-a5.txt`:
```
fix(easydrag): raise two controls to the 24 px target size

.ed-seg--small buttons are 24 px high and .ed-port-add is 24 x 24 px (WCAG
2.5.8, findings U9 and V4). The two documented exceptions, .ed-switch--small
(hit area 34 x 25 px) and .ed-flow-select input (inside a 44 x 44 px label),
are recorded as comments; the browser target-size test of the tiers part reads
the same list.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add ui/css/desktop-app-easydrag.css && git commit -F C:/disposable/easydrag-s0c/msg-a5.txt && git status --short
```
Expected: one commit, clean status.

---

### Task A6: "1 Schritte" plurals

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-core.js:142-146` and `:552` (new `pluralKey`, next to `stepsText`)
- Modify: `ui/js/desktop/apps/easydrag-groups.js:34` (`group_member_count`, `group_edges_count`, `group_errors`)
- Modify: `ui/js/desktop/apps/easydrag-publish.js:288` (`versions_stats`)
- Modify: `ui/lang/easydrag/*.json` (16 files, 5 new keys)
- Create: `scripts/test-easydrag-plurals.mjs`
- Create (outside the repo): `C:/disposable/easydrag-s0c/keys-a6-plurals.json`

**Design.** `stepsText(t, keyOne, keyMany, names)` picks the singular key when the count is exactly one (`easydrag-core.js:144-146`) and counts with two separate keys because languages count differently. `pluralKey(base, count, suffix = '_one')` makes the same choice for a key pair `base` / `base + suffix`. Both existing keys carry the `easydrag.ui.` prefix, so the new keys are `easydrag.ui.group_member_count_one`, `easydrag.ui.versions_stats_one` and, because the second text holds two counts, `easydrag.ui.versions_stats_edge_one`:

- `versions_stats_one`: the step count is 1 (the connection count then reads `{edges}`, normally 0);
- `versions_stats_edge_one`: the connection count is 1 and the step count is more than 1 (the common two-step flow "2 steps · 1 connection");
- `versions_stats` stays the plural text. One step and one connection cannot occur together.
- The group header's count line (`countText` in `easydrag-groups.js:31-35`) holds two more noun-after-number texts with the same defect, so they go in now: `easydrag.ui.group_edges_count_one` ("1 boundary connection") and `easydrag.ui.group_errors_one` ("1 error"). Their singular texts have no placeholder (the number is spelled "1"). `group_running`, `group_success` and `group_disabled` are verbs or adjectives ("1 running"), so they read correctly with one and stay as they are.
- **Survey of the other count texts** (every `en.json` value with `{count}`, checked while planning): the colon forms ("Added: {count}", "Items: {count}", "Copied: {count}", "Hints ({count})", "Local blocks ({count})") read correctly with one. Noun-after-number texts with the same defect that this task does **not** fix, because there are more than two of them and they sit in other features: `easydrag.ui.folder_flows` ("{count} flows"), `easydrag.ui.cred_purge_versions` ("{count} old versions will be deleted."), `easydrag.ui.cred_purge_pinned` ("{count} versions are still used …"; the verb changes too). They are listed in the final report as a Wave 1 follow-up, to be fixed with `core.pluralKey` the same way.

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && for s in stepsText countText; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|status)"'; done
```
`stepsText` is not changed (LOW, one caller `tell`); `pluralKey` is new. `countText` is the local function in `easydrag-groups.js` (it may answer "ambiguous"; its only job is to build the group header's count line, called from `paintCounts`). Confirm the call sites of the two keys: `grep -rn "group_member_count\|group_edges_count\|group_errors\|versions_stats" ui/js scripts | cut -c1-140` must show exactly `easydrag-groups.js:34` and `easydrag-publish.js:288` (the plurals test, once written, also matches the first pattern).

- [ ] **Step 2: write the failing test `scripts/test-easydrag-plurals.mjs`**

```js
#!/usr/bin/env node
// A6 (S0c): "1 Schritte" is wrong. group_member_count and versions_stats get singular variants
// through pluralKey, the choice stepsText makes (one name or several). The test checks the
// selection, the real texts of all 16 locales (singular and plural, same placeholders) and that
// the two call sites use the selection.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const store = new Map();
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' },
    localStorage: { get length() { return store.size; }, key: i => Array.from(store.keys())[i] ?? null, getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
    setTimeout: () => 1, clearTimeout: () => {}
});
vm.runInContext(fs.readFileSync(new URL('easydrag-core.js', apps), 'utf8'), box, { filename: 'easydrag-core.js' });
const core = box.window.EasyDrag.core;

assert.equal(core.pluralKey('k', 1), 'k_one', 'one is singular');
assert.equal(core.pluralKey('k', 2), 'k', 'two is plural');
assert.equal(core.pluralKey('k', 0), 'k', 'zero is plural');
assert.equal(core.pluralKey('k', 1, '_edge_one'), 'k_edge_one', 'a named variant');

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = lang => JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
const fill = (text, vars) => text.replace(/\{([a-z_]+)\}/g, (m, name) => String(vars[name]));
const placeholders = text => (text.match(/\{[a-z_]+\}/g) || []).sort().join(' ');
const GROUP = 'easydrag.ui.group_member_count', STATS = 'easydrag.ui.versions_stats';
const EDGES = 'easydrag.ui.group_edges_count', ERRORS = 'easydrag.ui.group_errors';
// The same choice easydrag-publish.js makes inline.
const statsKey = (nodes, edges) => (nodes === 1 ? core.pluralKey(STATS, 1) : core.pluralKey(STATS, edges, '_edge_one'));

const de = words('de');
const group = n => fill(de[core.pluralKey(GROUP, n)], { count: n });
assert.equal(group(1), '1 Schritt', 'German singular of the group count');
assert.equal(group(2), '2 Schritte', 'German plural of the group count');
assert.equal(fill(de[core.pluralKey(EDGES, 1)], { count: 1 }), '1 äußere Verbindung', 'German singular of the boundary count');
assert.equal(fill(de[core.pluralKey(EDGES, 3)], { count: 3 }), '3 äußere Verbindungen', 'German plural of the boundary count');
assert.equal(fill(de[core.pluralKey(ERRORS, 1)], { count: 1 }), '1 Fehler', 'German singular of the error count');
assert.equal(fill(de[core.pluralKey(ERRORS, 2)], { count: 2 }), '2 Fehler', 'German plural of the error count');
const stats = (nodes, edges) => fill(de[statsKey(nodes, edges)], { nodes, edges });
assert.equal(stats(1, 0), '1 Schritt · 0 Verbindungen', 'one step');
assert.equal(stats(2, 1), '2 Schritte · 1 Verbindung', 'one connection');
assert.equal(stats(3, 4), '3 Schritte · 4 Verbindungen', 'plural both');
const en = words('en');
assert.equal(fill(en[core.pluralKey(GROUP, 1)], { count: 1 }), '1 step');
assert.equal(fill(en[core.pluralKey(GROUP, 5)], { count: 5 }), '5 steps');

for (const lang of LOCALES) {
    const w = words(lang);
    for (const [one, many] of [[GROUP + '_one', GROUP], [STATS + '_one', STATS], [STATS + '_edge_one', STATS], [EDGES + '_one', EDGES], [ERRORS + '_one', ERRORS]]) {
        assert.ok(typeof w[one] === 'string' && w[one].trim(), lang + ' lacks ' + one);
        assert.ok(w[one].trim() !== '', lang + ' ' + one + ' is empty');
        // The placeholders of a variant are the plural's placeholders without the count that the variant spells out.
        for (const name of placeholders(w[one]).split(' ').filter(Boolean)) assert.ok(placeholders(w[many]).includes(name), lang + ' ' + one + ' uses ' + name + ', which ' + many + ' does not');
    }
}

// The call sites go through the selection.
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');
for (const key of [GROUP, EDGES, ERRORS]) assert.ok(read('easydrag-groups.js').includes("core.pluralKey('" + key + "'"), 'easydrag-groups.js selects the singular of ' + key);
assert.ok(read('easydrag-publish.js').includes("core.pluralKey('easydrag.ui.versions_stats'"), 'easydrag-publish.js selects the singular');
console.log('easydrag plurals: ok');
```

- [ ] **Step 3: run it and see it fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag-plurals.mjs 2>&1 | head -5
```
Expected: `TypeError: core.pluralKey is not a function`.

- [ ] **Step 4: add `pluralKey` to the core**

`ui/js/desktop/apps/easydrag-core.js`, after `stepsText`:
old:
```js
    function stepsText(t, keyOne, keyMany, names) {
        return names.length === 1 ? t(keyOne, { name: names[0] }) : t(keyMany, { count: names.length });
    }
```
new:
```js
    function stepsText(t, keyOne, keyMany, names) {
        return names.length === 1 ? t(keyOne, { name: names[0] }) : t(keyMany, { count: names.length });
    }

    // pluralKey picks the key of a count text: base + suffix when the count is exactly one, else
    // base. It is the choice stepsText makes (one or several), for texts that keep their {count}.
    function pluralKey(base, count, suffix) {
        return count === 1 ? base + (suffix || '_one') : base;
    }
```
and the export line (line 552, now shifted by 7):
old:
```js
        ICONS, icon, esc, tr, clamp, debounce, announcer, say, stepsText, removeConnection, frame, emitter, bag, el, isEditable, isMod, shortcut, IS_MAC,
```
new:
```js
        ICONS, icon, esc, tr, clamp, debounce, announcer, say, stepsText, pluralKey, removeConnection, frame, emitter, bag, el, isEditable, isMod, shortcut, IS_MAC,
```

- [ ] **Step 5: use it in the two call sites**

`ui/js/desktop/apps/easydrag-groups.js:34` (inside `countText`). Three elements of the returned array change; the rest of the line (running, success, disabled, the filter and the join) stays as it is. Use the Edit tool with exactly the old text as `old_string` (it is unique):
old:
```js
            return [ed.t('easydrag.ui.group_member_count', { count: item.members.length }), ed.t('easydrag.ui.group_edges_count', { count: boundaryCounts.get(item.id) || 0 }), errors ? ed.t('easydrag.ui.group_errors', { count: errors }) : '',
```
new:
```js
            return [ed.t(core.pluralKey('easydrag.ui.group_member_count', item.members.length), { count: item.members.length }), ed.t(core.pluralKey('easydrag.ui.group_edges_count', boundaryCounts.get(item.id) || 0), { count: boundaryCounts.get(item.id) || 0 }), errors ? ed.t(core.pluralKey('easydrag.ui.group_errors', errors), { count: errors }) : '',
```

`ui/js/desktop/apps/easydrag-publish.js:288`:
old:
```js
                        esc(t('easydrag.ui.versions_stats', { nodes: v.node_count || 0, edges: v.edge_count || 0 })) + '</span></div>' +
```
new:
```js
                        esc(t(Number(v.node_count) === 1 ? core.pluralKey('easydrag.ui.versions_stats', 1) : core.pluralKey('easydrag.ui.versions_stats', Number(v.edge_count), '_edge_one'), { nodes: v.node_count || 0, edges: v.edge_count || 0 })) + '</span></div>' +
```

- [ ] **Step 6: the locale keys**

Write `C:/disposable/easydrag-s0c/keys-a6-plurals.json`:
```json
{
  "easydrag.ui.group_member_count_one": {
    "_after": "easydrag.ui.group_member_count",
    "cs": "1 krok", "da": "1 trin", "de": "1 Schritt", "el": "1 βήμα", "en": "1 step", "es": "1 paso",
    "fr": "1 étape", "hi": "1 चरण", "it": "1 passaggio", "ja": "1 ステップ", "nl": "1 stap", "no": "1 trinn",
    "pl": "1 krok", "pt": "1 passo", "sv": "1 steg", "zh": "1 个步骤"
  },
  "easydrag.ui.group_edges_count_one": {
    "_after": "easydrag.ui.group_edges_count",
    "cs": "1 vnější propojení", "da": "1 forbindelse på tværs", "de": "1 äußere Verbindung", "el": "1 εξωτερική σύνδεση", "en": "1 boundary connection", "es": "1 conexión externa",
    "fr": "1 connexion externe", "hi": "1 बाहरी कनेक्शन", "it": "1 collegamento esterno", "ja": "外部接続 1 件", "nl": "1 externe verbinding", "no": "1 forbindelse ut av gruppen",
    "pl": "Połączenia zewnętrzne: 1", "pt": "1 ligação externa", "sv": "1 extern anslutning", "zh": "1 条外部连接"
  },
  "easydrag.ui.group_errors_one": {
    "_after": "easydrag.ui.group_errors",
    "cs": "1 chyba", "da": "1 fejl", "de": "1 Fehler", "el": "1 σφάλμα", "en": "1 error", "es": "1 error",
    "fr": "1 erreur", "hi": "1 त्रुटि", "it": "1 errore", "ja": "エラー 1 件", "nl": "1 fout", "no": "1 feil",
    "pl": "Błędy: 1", "pt": "1 erro", "sv": "1 fel", "zh": "1 个错误"
  },
  "easydrag.ui.versions_stats_one": {
    "_after": "easydrag.ui.versions_stats",
    "cs": "1 krok · {edges} spojení",
    "da": "1 trin · {edges} forbindelser",
    "de": "1 Schritt · {edges} Verbindungen",
    "el": "1 βήμα · {edges} συνδέσεις",
    "en": "1 step · {edges} connections",
    "es": "1 paso · {edges} conexiones",
    "fr": "1 étape · {edges} connexions",
    "hi": "1 चरण · {edges} कनेक्शन",
    "it": "1 passaggio · {edges} connessioni",
    "ja": "1 ステップ · {edges} 接続",
    "nl": "1 stap · {edges} verbindingen",
    "no": "1 trinn · {edges} koblinger",
    "pl": "1 krok · {edges} połączeń",
    "pt": "1 passo · {edges} ligações",
    "sv": "1 steg · {edges} kopplingar",
    "zh": "1 个步骤 · {edges} 条连接"
  },
  "easydrag.ui.versions_stats_edge_one": {
    "_after": "easydrag.ui.versions_stats_one",
    "cs": "{nodes} kroků · 1 spojení",
    "da": "{nodes} trin · 1 forbindelse",
    "de": "{nodes} Schritte · 1 Verbindung",
    "el": "{nodes} βήματα · 1 σύνδεση",
    "en": "{nodes} steps · 1 connection",
    "es": "{nodes} pasos · 1 conexión",
    "fr": "{nodes} étapes · 1 connexion",
    "hi": "{nodes} चरण · 1 कनेक्शन",
    "it": "{nodes} passaggi · 1 connessione",
    "ja": "{nodes} ステップ · 1 接続",
    "nl": "{nodes} stappen · 1 verbinding",
    "no": "{nodes} trinn · 1 kobling",
    "pl": "{nodes} kroków · 1 połączenie",
    "pt": "{nodes} passos · 1 ligação",
    "sv": "{nodes} steg · 1 koppling",
    "zh": "{nodes} 个步骤 · 1 条连接"
  }
}
```
`_after` of the third key is the second new key; the helper inserts them in order (it skips keys it inserted in the same run).
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs C:/disposable/easydrag-s0c/keys-a6-plurals.json && git diff --stat -- ui/lang/easydrag | tail -1
```
Expected: `16 files changed, 80 insertions(+)`.

- [ ] **Step 7: the placeholder rule for the variants**

`TestCatalogTranslationsAreComplete` compares each locale's placeholders with `en` for the same key. The texts above keep the same names in every locale (`{edges}` only, `{nodes}` only, none for the group variant), so it passes. Run the checks:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag-plurals.mjs && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCatalogTranslationsAreComplete' -count=1 2>&1 | tail -1
```
Expected: `easydrag plurals: ok`, then `ok  	aurago/internal/flows`.

- [ ] **Step 8: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && wc -l ui/js/desktop/apps/easydrag-groups.js ui/js/desktop/apps/easydrag-publish.js ui/js/desktop/apps/easydrag-core.js | tail -4
```
Expected: `all EasyDrag checks passed`; `PASS render baseline (unchanged)`; the modules stay far below 900 lines. If a groups test (`test-easydrag-groups.mjs` or the groups block of `test-easydrag.mjs`) compares the count line, the harness `t` returns `group_member_count_one:3`-style keys: a group of exactly one member now reads `group_member_count_one:1`; update that one expectation.

- [ ] **Step 9: commit**

Write `C:/disposable/easydrag-s0c/msg-a6.txt`:
```
fix(easydrag): say "1 Schritt" instead of "1 Schritte"

group_member_count, group_edges_count, group_errors and versions_stats get
singular variants in all 16 locales, chosen by core.pluralKey, the same
one-or-several choice stepsText makes. versions_stats has two counts, so it has
a variant for one step and one for one connection ("2 Schritte · 1 Verbindung").

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-groups.js ui/js/desktop/apps/easydrag-publish.js scripts/test-easydrag-plurals.mjs ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-s0c/msg-a6.txt && git status --short
```
Expected: one commit, clean status.

---

### Task A7.1: guide and double-scrub comments

**Files:**
- Modify: `internal/flows/AGENTS.md:831`
- Modify: `internal/flows/credentials.go:658-684` (`RedactFlow`'s doc area; the function around the scrub loop)
- Modify: `internal/flows/local_pack.go:421` and `:447-448` (`sanitizeLocalPackFlow`)

Comment and documentation changes only; no behaviour changes.

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && for s in RedactCredentials sanitizeLocalPackFlow; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"'; done
```
Result on main: `RedactCredentials` MEDIUM (13 impacted: export, packs, run view, child history), `sanitizeLocalPackFlow` LOW (5). Only comments change, so no behaviour moves; report the MEDIUM to the coordinator in the final note and run the full package tests below.

- [ ] **Step 2: AGENTS.md names no template count**

`internal/flows/AGENTS.md:831`:
old:
```
- Templates (`TemplateFlow(id, tr)`, listed by `Templates()`): six, all free of lint warnings. Their texts are
```
new:
```
- Templates (`TemplateFlow(id, tr)`, listed by `Templates()`; the list lives in `templates_builtin.go`, which changes more often than this guide, so no count is named here): all free of lint warnings. Their texts are
```
Check that the line wraps sensibly and nothing else in the file counts templates: `grep -n "\bsix\b\|twelve" internal/flows/AGENTS.md` must print nothing.

- [ ] **Step 3: the comment at the first scrub site**

`internal/flows/credentials.go`, in the function that holds the lines `group.Title = security.ScrubSecrets(group.Title)` (around `:669-684`; confirm the function name with `grep -n "^func RedactCredentials" internal/flows/credentials.go`). Put this comment directly above `if c.Groups != nil {`:

old:
```go
	if c.Groups != nil {
		for i := range *c.Groups {
			group := &(*c.Groups)[i]
			group.Title = security.ScrubSecrets(group.Title)
```
new:
```go
	// Defence in depth: local packs scrub the same texts again in sanitizeLocalPackFlow
	// (local_pack.go), after this function ran. Keep both; a change to what is scrubbed here
	// must be mirrored there, and the other way round.
	if c.Groups != nil {
		for i := range *c.Groups {
			group := &(*c.Groups)[i]
			group.Title = security.ScrubSecrets(group.Title)
```

- [ ] **Step 4: the comment at the second scrub site**

`internal/flows/local_pack.go`, in `sanitizeLocalPackFlow`, above the loop that scrubs node labels and notes (around `:421`):

old:
```go
	for i := range redacted.Nodes {
		node := &redacted.Nodes[i]
		node.Label = security.ScrubSecrets(node.Label)
```
new:
```go
	// Defence in depth: RedactCredentials (credentials.go) already scrubbed notes, group texts
	// and credential parameters on the copy it returned. The scrub here repeats it for the
	// texts a pack stores, and also covers labels, the name and the description. Keep both;
	// a change to what is scrubbed here must be mirrored there, and the other way round.
	for i := range redacted.Nodes {
		node := &redacted.Nodes[i]
		node.Label = security.ScrubSecrets(node.Label)
```
Before you write the comment, read the lines just above (`redacted, err := RedactCredentials(…)` or equivalent, around `:415-417`) and check that the sentence "already scrubbed notes, group texts and credential parameters" is what the code does; if `redacted` comes from another function, name that one and fix the sentence.

- [ ] **Step 5: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows/ && gofmt -l internal/flows/credentials.go internal/flows/local_pack.go && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'Redact|LocalPack|Credential' -count=1 2>&1 | tail -2
```
Expected: nothing from vet and gofmt; `ok  	aurago/internal/flows`.

- [ ] **Step 6: commit**

Write `C:/disposable/easydrag-s0c/msg-a7-1.txt`:
```
docs(flows): name no template count; mark the two scrub sites as one pair

The flows guide no longer says how many built-in templates exist
(templates_builtin.go changes more often than the guide). RedactCredentials and
sanitizeLocalPackFlow scrub the same texts twice on purpose (defence in depth);
a comment at each names the other.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/flows/AGENTS.md internal/flows/credentials.go internal/flows/local_pack.go && git commit -F C:/disposable/easydrag-s0c/msg-a7-1.txt && git status --short
```
Expected: one commit, clean status.

---

### Task A7.2: the "Vergangener Lauf" tooltip names the start time

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-header.js:51-53` (`runNowTitle`)
- Modify: `scripts/test-easydrag-extra5.mjs:836-841` (one extra assertion)
- Modify: `ui/lang/easydrag/*.json` (16 files, 1 new key)
- Create (outside the repo): `C:/disposable/easydrag-s0c/keys-a7-runview.json`

**Facts.** In the run view the header's state chip says "Vergangener Lauf" (`stateOf`, `easydrag-header.js:24`, `state_run_view`); the disabled Run now button's tooltip says the same (`runNowTitle`, `:52`). The banner below the header already names the run and its start time (`run_view_banner`). The chip stays; the tooltip names the start time. The run view's start time is formatted by `core.fmt.dateTime(iso)` (the banner and the run list use the same call; `seconds=true` only where two runs of one minute must be told apart). Existing test: `scripts/test-easydrag-extra5.mjs:836-841` expects the title `state_run_view` for a run view **without** `started_at`; keep that behaviour (fallback to the old text), so the test stays valid, and add one case with a start time. The chip-count test `test-easydrag-ux-batch5.mjs:578-593` counts `state_run_view` in the state chip and the revision line, not the title, and is unaffected.

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "runNowTitle" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"'
```
Result on main: **HIGH**, 16 impacted. Warn the coordinator in the final note. The reason is the fan-out of the header paint (`paintRevision` and everything that calls the header render), not the function itself: the change keeps the signature and the return type (a string), and changes the text only when `ed.runView` is set and has a start time. The existing `ux the run view keeps Run now visible and disabled` assertion and the full Node suite are the guard.

- [ ] **Step 2: add the failing assertion**

`scripts/test-easydrag-extra5.mjs`, after the existing `eq('ux the run view keeps Run now visible and disabled', …)` (line 839-841):

old:
```js
            [false, true, 'state_run_view']);

        draft.dispose();
```
new:
```js
            [false, true, 'state_run_view']);
        // A run view that knows its start time names it in the tooltip instead of repeating the chip (S0c A7).
        ahead.ed.runView = { run: { id: 'r9', status: 'success', revision: 2, started_at: '2026-10-25T10:00:00Z' } };
        ahead.ed.bus.emit('enabled');
        const timedBtn = cmdBtn(ahead.el, 'run-now');
        const timedTitle = timedBtn.getAttribute('title') || timedBtn.title;
        eq('ux the run view tooltip names the start time, not the chip text again',
            [timedTitle.startsWith('state_run_view_started:'), timedTitle.length > 'state_run_view_started:'.length, timedTitle === 'state_run_view'],
            [true, true, false]);

        draft.dispose();
```

- [ ] **Step 3: run it and see it fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag.mjs 2>&1 | grep -E "FAIL|run view tooltip" | head -4
```
Expected: `FAIL ux the run view tooltip names the start time, not the chip text again — got [false,false,true] want [true,true,false]`.

- [ ] **Step 4: implement**

`ui/js/desktop/apps/easydrag-header.js`, `runNowTitle`:
old:
```js
            if (ed.runView) return t('easydrag.ui.state_run_view');
```
new:
```js
            if (ed.runView) {
                // The chip beside it says "Past run"; the tooltip names which run, by its start time.
                const started = core.fmt.dateTime(ed.runView.run && ed.runView.run.started_at);
                return started ? t('easydrag.ui.state_run_view_started', { time: started }) : t('easydrag.ui.state_run_view');
            }
```
(`core` is `ED.core`, defined at the top of `create`, line 12.) Note the `ed.runView` guard in `runNowTitle` comes after `state.hint` is checked; do not move it.

- [ ] **Step 5: the locale key**

Write `C:/disposable/easydrag-s0c/keys-a7-runview.json`:
```json
{
  "easydrag.ui.state_run_view_started": {
    "_after": "easydrag.ui.state_run_view",
    "cs": "Běh z {time}",
    "da": "Kørsel fra {time}",
    "de": "Lauf vom {time}",
    "el": "Εκτέλεση της {time}",
    "en": "Run of {time}",
    "es": "Ejecución del {time}",
    "fr": "Exécution du {time}",
    "hi": "{time} का रन",
    "it": "Esecuzione del {time}",
    "ja": "{time} の実行",
    "nl": "Uitvoering van {time}",
    "no": "Kjøring fra {time}",
    "pl": "Uruchomienie z {time}",
    "pt": "Execução de {time}",
    "sv": "Körning från {time}",
    "zh": "{time} 的运行"
  }
}
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/add-locale-keys-a2.mjs C:/disposable/easydrag-s0c/keys-a7-runview.json && git diff --stat -- ui/lang/easydrag | tail -1
```
Expected: `16 files changed, 16 insertions(+)`.

- [ ] **Step 6: run and gate**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node scripts/test-easydrag.mjs 2>&1 | grep -E "FAIL|run view tooltip|all EasyDrag" && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCatalogTranslationsAreComplete' -count=1 2>&1 | tail -1
```
Expected: `ok   ux the run view tooltip names the start time, not the chip text again`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok  	aurago/internal/flows`.

- [ ] **Step 7: commit**

Write `C:/disposable/easydrag-s0c/msg-a7-2.txt`:
```
fix(easydrag): the run view's Run now tooltip names the run's start time

The state chip and the disabled Run now button both said "Vergangener Lauf" in
the run view. The tooltip now reads "Lauf vom {time}" (all 16 locales) and
falls back to the old text when the run has no start time.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add ui/js/desktop/apps/easydrag-header.js scripts/test-easydrag-extra5.mjs ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-s0c/msg-a7-2.txt && git status --short
```
Expected: one commit, clean status.

---

### Task A7.3: ledger evidence (verification only, no commit)

This task changes no file. Re-run the checks below in the worktree to confirm the evidence is still true at the head of Part A, then hand the table to the coordinator. **No part edits the spec or the roadmap:** the coordinator copies the table into the ledger (spec §7) on main when the part lands, together with the other parts' numbers.

- [ ] **Step 1: the named tests exist and pass**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && grep -rn "^func TestFB4MissionUpdatesAreDebounced\|^func TestFB4SSECacheScrubsAnEventOnce\|^func TestFB4FlowStreamsAreCappedPerClient\|^func TestFB4MissionLastRunIsLeftOutWhenZero" internal | cut -d: -f1,2,3 && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run 'TestFB4MissionLastRunIsLeftOutWhenZero' -count=1 2>&1 | tail -1 && go test ./internal/server -run 'TestFB4MissionUpdatesAreDebounced|TestFB4SSECacheScrubsAnEventOnce|TestFB4FlowStreamsAreCappedPerClient' -count=1 2>&1 | tail -1
```
Expected: four lines (file:line:func, as in the table), then `ok  	aurago/internal/tools`, then `ok  	aurago/internal/server`. The `internal/server` run deliberately sets **no** `GOTMPDIR` (a temp directory on D: breaks a cross-drive `filepath.Rel` there). If the `internal/server` test takes more than a few minutes to compile, report it; do not skip it silently.

- [ ] **Step 2: the evidence table (verified on main b52103d51 while planning; re-checked by Step 1)**

| Ledger row | Claim | Evidence found | Verdict |
|---|---|---|---|
| 4 | mission_update debounce | `internal/server/mission_update_debounce.go:11` `const missionUpdateDebounce = 250 * time.Millisecond`; the debouncer folds calls (leading and trailing edge, `fire`/`arm`). Test `TestFB4MissionUpdatesAreDebounced` at `internal/server/mission_update_debounce_fb4_test.go:8` (leading edge sends once, trailing edge sends the folded call and opens a new window) | closed |
| 5 | SSE redaction per (run, seq) with a per-client cap | `internal/server/flows_sse_cache.go:35-65` `scrubbedEvent` caches the scrubbed JSON under `flowSSEKey{RunID, Seq}`, bounded by `flowSSECacheEntries`/`flowSSECacheBytes`; `flowStreamsPerClient = 8` at `internal/server/flows_handlers_runs.go:381`, enforced in `acquireFor` (`:404`). Tests `TestFB4SSECacheScrubsAnEventOnce` at `internal/server/flows_sse_fb4_test.go:10` (second stream scrubs 0 more times, same bytes) and `TestFB4FlowStreamsAreCappedPerClient` at `:29` (eighth stream of one address passes, ninth refused, other address passes, release gives the slot back) | closed |
| 7 | `LastRun` omitted when zero | `internal/tools/missions_v2.go:157` `LastRun time.Time \`json:"last_run,omitzero"\``; test `TestFB4MissionLastRunIsLeftOutWhenZero` at `internal/tools/missions_v2_fb4_test.go:51` (a mission that never ran has no `last_run`, one that ran has it) | closed |
| 8 | docked palette restored after the run view | `easydrag-editor.js:245-246` (`exitRunView` calls `palette.restoreDocked()`), `easydrag-palette.js:257-260` (`restoreDocked` reopens by the stored choice once docked). Covered by `scripts/test-easydrag-ux-batch5.mjs:427` "b5 leaving the run view shows the docked palette by the stored choice, also after a narrow start": narrow start closed, hidden in the run view, back when docked on exit, a floating palette stays closed; it runs inside `node scripts/test-easydrag.mjs` as `ok   b5 narrow start: closed, hidden in the run view, back when docked on exit; …`. **No new test needed.** | closed |
| 14 | the agent manual describes `read_only` and `allow_publish` as they behave | `prompts/tools_manuals/manage_missions.md:21` (the flow bullet; spec's "20-21" is line 21 on main) says: with `flows.agent.read_only` on, `run` and `run_now` on a flow mission are refused while `list` and `history` work; it stops only direct starts, not flows the owner's own triggers start; the agent cannot publish flows. `internal/config/flows.go:25-33` (`FlowsAgentConfig`): `ReadOnly` refuses run and run_now on a flow mission, read on every call, does not stop owner triggers; `AllowPublish` has no effect, the agent cannot publish. The dispatch that enforces it: `internal/agent/agent_dispatch_comm.go:1411` (`targetIsFlow && cfg.Flows.Agent.ReadOnly && (run \|\| run_now)`). The manual does not spell the name `allow_publish`; it states the behaviour (cannot publish), which is what the config comment says | closed |

- [ ] **Step 3: report**

The final report to the coordinator carries: the table above, with the pass/fail of Step 1; the IMPACT results of every task (`displayValue` LOW 4, `describeParam` LOW 4, `createResults` LOW 7, `scheduleParams` LOW 7, `sanitizeLocalPackFlow` LOW 5, `RedactCredentials` MEDIUM 13, `runNowTitle` HIGH 16 with the reason in A7.2, `applyRunView` and `renderBanner` ambiguous with walkable results LOW to MEDIUM, `CatalogI18nKeys` UNKNOWN resolved by the full flows test run); that the ledger row 1 hands "stopped" to Wave 1 (FD-08): *a run-level reason "stopped" whose word is "durch Stopp beendet", rendered by every surface that shows a run status*; and the A5 exception list. Nothing in the spec or roadmap is edited by this part.

---

## Coverage: spec item to task

| Spec item | Task |
|---|---|
| A2 / §3.2 stop hint (`stoppedBy`, drawer and test panel, 16 locales, Node test) | A2 |
| A3 / §3.3 file.write card (`card_label`, canvas line, summary, default test) | A3 |
| A4 / §3.4 schedule help | A4 |
| A5 / §3.5 target sizes, exception list | A5 |
| A6 / §3.6 plurals | A6 |
| A7 / §3.7 AGENTS.md count, double-scrub comments | A7.1 |
| A7 / §3.7 "Vergangener Lauf" tooltip | A7.2 |
| A7 / §3.7 and §7 ledger rows 4, 5, 7, 8, 14 evidence | A7.3 |
| §7 row 1 follow-up (status-map entry for Wave 1) | A7.3 report |

## Deviations from the spec

1. **A2, where the drawer's hint appears.** The spec says the run drawer shows the hint. The drawer's own list rows hold no steps (`GET runs` returns run records only), so the hint cannot be computed there without one request per row. It is shown in the run view the drawer opens (the banner line), which is the drawer's detail surface. `stoppedBy` takes the run record **and** requires the run status `success`, which the spec's wording ("a successful run") implies but its function description left out: the stop step is `success` even for a stop with status "error" (`engine_state.go:659-672`), so the step alone is not enough.
2. **A2, signature.** `stoppedBy(detail, model)` returns `{ step, node, label }` (not the bare step), so both surfaces share the label rule (node label, type label, type). It lives in `easydrag-testing.js` (exported on `ED.testing`), not in a new module.
3. **A3, option values.** The spec and brief name the second value "rename"; the real values are `overwrite`, `unique` and `fail`. The card keys are `easydrag.option.if_exists_{overwrite,unique,fail}.card`. A second Go test (`TestFileWriteOptionsCarryCardLabels`) pins the new `card_label` behaviour, because the spec's single default test does not reach it. `CatalogI18nKeys` lists the card keys so the existing locale-completeness test covers them.
4. **A6, three keys instead of two.** `versions_stats` holds two counts, so one `_one` variant would still print "2 Schritte · 1 Verbindungen" (the commonest small flow). The part adds `versions_stats_one` (one step) and `versions_stats_edge_one` (one connection) next to `group_member_count_one`. By the coordinator's decision it also covers `group_edges_count_one` and `group_errors_one` (same header line, same defect); three further keys (`folder_flows`, `cred_purge_versions`, `cred_purge_pinned`) are a Wave 1 follow-up. The selection is a new core function `pluralKey` (the same one-or-several choice as `stepsText`), because `stepsText` takes names, not a count text.
5. **A7.2, fallback.** The tooltip falls back to the old "Vergangener Lauf" text when the viewed run has no start time, so the existing `ux the run view keeps Run now visible and disabled` assertion stays valid.
6. **A5, further small controls** are fixed by Part B in its own commit after A has landed (README decision).
7. **A7, row 8** needs no new test: `scripts/test-easydrag-ux-batch5.mjs:427` covers it.

## Points that need a decision, with a recommendation

- **Accepted by the coordinator:** the drawer/run-view hint is not covered by a DOM test. The Node harness catalog (`scripts/test-easydrag.mjs:96`) has no `logic.stop` type, and the spec's test is the pure `stoppedBy` function. Recommendation: accept this (the integration is two short call sites, guarded by the full Node suite and the render baseline), and let Part B's or D's browser run look at a stop run once; if the owner wants more, add a `logic.stop` entry to the harness `types` map and a banner assertion modelled on `b5 V1` (`test-easydrag-ux-batch5.mjs:578`).
- **Decided by the coordinator:** `group_edges_count` and `group_errors` are in A6; `folder_flows`, `cred_purge_versions` and `cred_purge_pinned` are a Wave 1 follow-up (listed in the final report).
- **Other locale files are not sorted strictly.** The helper inserts next to siblings (`_after`) or inside the key's namespace, which matches how the files grow today (sorted core, appended blocks). If the owner wants a strict sort, that is a separate, repo-wide change.
