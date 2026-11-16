## Part C: the one Stop

**Goal.** The editor shows exactly one Stop. It is the header's button `[data-ed-cmd="cancel"]` in the reserved slot `[data-ed-stop-slot]`, it stops the run in focus (the viewed run while it can be stopped, else the editor's own run, also while a past run is open), its words ("Stoppen", "Wird gestoppt…") show at every width, and it does not move any other header control. The run-view banner and the runs drawer lose their Stops. Spec: `plans/2026-11-16-easydrag-w1c-spec.md` §5 (decision 4 of §1 applies).

**Shell and worktree.** Git Bash. Every command starts with `cd W && ...`; `W` is `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-header` (branch `feat/easydrag-w1c-header`). Part C runs **in Part B's worktree, after B's last commit**. `P` below means `C:/disposable/easydrag-w1c/patches` (the patch scripts of this part are written there; `lib-patch.mjs` sits one folder up). `PLAN` means `plans/2026-11-16-easydrag-w1c/part-c-stop.md`.

**Depends on Part B.** The boundary this part builds on:
- the header has a slot `<span class="ed-stop-slot" data-ed-stop-slot>` of fixed width (`.ed-stop-slot { ... width: var(--ed-stop-slot-w, 9.5rem); }`, visibility of the content by the `hidden` attribute of the button) that holds today's button `[data-ed-cmd="cancel"]`, which B left as it is: `hidden` while no run is shown, set by `btn('cancel').hidden = !running;` in `renderHeader` (`easydrag-header.js`). B's 900 px rule already spares the Stop's text (`:not([data-ed-cmd="cancel"])`, `> span { display: inline }`) and its stylesheet test pins the slot rule; **B left the slot width as a guess and says Part C sets it from the real labels (C.1 does, Step 4)**;
- B removed the header buttons "Läufe" (`runs`), "Jetzt ausführen" (`run-now`) and "Aktiv" (`active`); they live in the pill popover `[data-ed-pill-popover]` (B's tests open the runs drawer with `s.pillEntry("runs")`);
- B's `renderHeader` still has `const running = runs.isRunning();`, `btn('cancel').hidden = !running;`, `paintPrimary();` and `paintActions();` (Task C.1, Step 1 checks this). In the right-hand group the header takes **two rows below 560 px** (the coordinator accepted that for W1c; Undo and Redo stay in the header), so C.3's position test compares a control with itself at one width, never across widths.

Part A (server data) and D (quick-add) do not touch this part's files. Part C changes no Go package code and no server code; its Go changes are browser tests.

**Facts the plan rests on** (read in the code on main d87930dfc, 2026-11-16; Task C.1, Step 1 re-checks the ones B can move):

| Fact | Where |
|---|---|
| Three Stops today: header `[data-ed-cmd="cancel"]`, banner `[data-ed-cmd="stop-viewed"]`, one `[data-ed-run-stop]` per active drawer row. | `easydrag-editor.js` (markup, `renderBanner`, click handler), `easydrag-run-drawer.js` (`runRow`, click handler) |
| In the run view the viewed run is `ed.run` with `view: true`; `runs.isRunning()` is false, so the header Stop is hidden. A live run of this window is **parked** (`rc.parked = {id, mode}`) behind the view and ends unseen (`watchParked` polls it). | `easydrag-run-drawer.js` `applyRunView`, `easydrag-run-stream.js` `isRunning` |
| The banner's Stop rule (`renderBanner`): hidden when the checkpoint read failed (`ed.run.durableStatusError`); for a durable run it follows the checkpoint's `cancel_available`; for any other run it shows while the run is queued, waiting or running (a stored run that says `durable` but has no checkpoint has none). A read-only desktop keeps it. | `easydrag-editor.js` `renderBanner` |
| The header Stop shows while `runs.isRunning()`; its click calls `runs.cancel()`, which stops `ed.run`; the confirmation (`confirmStop`) runs for every mode but `test`, and durable runs re-read their checkpoint first (`stopRun`). | `easydrag-run-stop.js` |
| `stopButtons(id)` collects the three kinds; `setStopPending(id, on)` paints "Stopping…" on them. | `easydrag-runs.js`, `easydrag-run-stop.js` |
| `flow.stop` is bound in the editor with a `check` (checkpoint read failed, or the button is busy) and has no other availability rule. Only the header paints it. | `easydrag-editor.js`, `easydrag-actions.js` |
| `easydrag-runs.js` may have at most 400 lines (`easyDragRunsLines`, `TestEasyDragAppLifecycle`); `easydrag-editor.js` at most 896 (W1c rule). | `ui/desktop_easydrag_test.go`, brief |
| The new component needs no new module: `easydrag-run-stop.js` is in `module-loader.js`, `easyDragModules` and every Node harness list already. | `module-loader.js:313`, `ui/desktop_easydrag_test.go:11` |

**Baseline (before any change).**

```
cd W && git status --short && git log --oneline -3
cd W && npm run test:easydrag:t0
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-stop.js ui/js/desktop/apps/easydrag-run-drawer.js ui/js/desktop/apps/easydrag-header.js
```

Expected: `git status --short` prints nothing (B committed everything; if it prints files, stop: Part C starts from a clean tree because a failing patch script is undone with `git checkout -- .`); the log's newest commit is B's last; T0 ends without a FAIL line. Measured on main d87930dfc: `859 / 138 / 299 / 213 / 200` lines (editor, runs, run-stop, run-drawer, header); on main plus B.1 to B.6 (scratch copy): `847 / 138 / 299 / 213 / 157`. **Take the printed editor count as the baseline: Part C must end below it, and at or below 896.** On the scratch copy (main plus B plus Part C) the editor ends at 840 (-7), runs at 140, run-stop at 348, drawer at 204, header at 160.

**Patch scripts.** The edits of this part are patch scripts in the W1b style: `import { edit } from '../lib-patch.mjs'`, run from the worktree root. `edit` keeps each file's line endings and requires each old text to occur exactly once. It writes after each edit, so **run a script on a clean tree**; if it stops with `expected 1 match(es) ... found 0`, run `cd W && git checkout -- .`, read the code at that place (B may have moved it), adjust the old text of that one edit so that it keeps its meaning, and run the script again. The scripts that sit on B's work are kept apart (`c-1-header.mjs`, and the Go and test edits of C.2 and C.3) so that only they can need an adjustment.

**Go commands** use `GOTOOLCHAIN=go1.27.4` and, for `./ui`, `GOTMPDIR='D:\tmp\go-build'`. Browser tests: `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots`, the Bash sandbox off. Before a browser run check for leftover headless Chromes (spec §7).

**IMPACT (run before the code of C.1 and C.2 changes).** The index is stale: it files `stopButtons`, `setStopPending` and `runRow` under `easydrag-runs.js`, where they no longer live (they are in `easydrag-run-stop.js` and `easydrag-run-drawer.js`).

```
cd W && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "setStopPending" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Result on 2026-11-16: `stopButtons` LOW (12 impacted, lower bound); `setStopPending` **CRITICAL (90 impacted)**; `runRow` UNKNOWN (no callers resolved); `renderBanner` ambiguous (3 candidates, one per file, max LOW). The CRITICAL figure is the index counting transitively through the run controller, not callers of the function. Confirmed by text search, which is the check that counts here:

```
cd W && git grep -n "setStopPending\|stopButtons\|runRow\|stopRun(" -- ui/js/desktop/apps
```

`setStopPending` is called only inside `easydrag-run-stop.js` (`stopRun`, `settleDurableStop`) and once in `easydrag-run-drawer.js` (`drawRuns`, removed in C.2). `stopButtons` is read in `easydrag-run-stop.js` only. `runRow` is called only by `drawRuns`. `stopRun` is called by `cancel()` (header), the drawer's row click and the editor's `stop-viewed` click, the last two removed in C.2. **Warning to the implementer: HIGH/CRITICAL risk is reported, the text search above bounds it; the change keeps `setStopPending(id, on)` and `stopRun(record)` as they are for the header, so no caller outside the three files changes.**

---

### Task C.1: The Stop component and the header's Stop for the run in focus

**Files:**
- Create: `scripts/test-easydrag-stop.mjs` (Node test of `focusRun`, `paint`, `sync`)
- Create: `plans/2026-11-16-easydrag-w1c/keys/c-1-stopping.json` (key file; the coordinator commits it; the step copies nothing)
- Modify: `ui/js/desktop/apps/easydrag-run-stop.js` (component above `installStop`, `setStopPending`, `cancel`, export), `ui/js/desktop/apps/easydrag-runs.js` (`stopButtons`, `rc.pendingStops`, `ed.parkedRun`, facade `stopPending`), `ui/js/desktop/apps/easydrag-run-drawer.js` (`applyRunView` keeps the parked record), `ui/js/desktop/apps/easydrag-header.js` (`renderHeader`), `ui/css/desktop-app-easydrag.css` (slot width from the real labels, Stop words on one line), `ui/lang/easydrag/*.json` (key file), `scripts/test-easydrag-header.mjs` (B's stylesheet test names the slot width)
- Test: `scripts/test-easydrag-stop.mjs`, `scripts/test-easydrag-extra6.mjs` (a real editor: past run open, own test running)

**What C.1 does.** `ED.runStop.focusRun(ed)` returns the run the Stop stops, as a record `stopRun` takes, or `null`:
1. a run is open in the review and still stoppable (the banner's rule, now in one function) → that run;
2. else a run is open and the editor's own run is parked behind it → the parked run (its record travels, so the confirmation can name its start time and a durable run is recognised);
3. else no review and the own run is active (the rule of `runs.isRunning()`) → the own run;
4. else `null`.

`ED.runStop.paint(button, t, pending, ed)` gives the button its words and busy state; `ED.runStop.sync(ed, button, t, pendingOf)` shows or hides the button for the run in focus and hands the keyboard focus on when the button goes away while it has it (to "Back to draft" in the review, else to the canvas: the banner's Stop did the first, the header's Stop lost its focus). `cancel()` stops `focusRun(ed)`. The banner and the drawer keep their Stops until C.2, so C.1 is green by itself: `setStopPending` paints the header button through `paint` and the others as before.

- [ ] **Step 1: Check B's state and what to do if it differs**

```
cd W && git grep -n "data-ed-stop-slot" -- ui/js/desktop/apps/ ui/css/
cd W && git grep -n "data-ed-cmd=\"cancel\"" -- ui/js/desktop/apps/easydrag-editor.js
cd W && git grep -n "btn('cancel')\|paintActions();\|runs.isRunning()" -- ui/js/desktop/apps/easydrag-header.js
cd W && git grep -c "data-ed-cmd=\"runs\"\|data-ed-cmd=\"run-now\"\|data-ed-cmd=\"active\"" -- ui/js/desktop/apps/easydrag-editor.js
cd W && git grep -n "data-ed-pill-popover" -- ui/js/desktop/apps/ | head -3
```

Expected (B as specified): the slot attribute appears in the editor markup and the CSS; the `cancel` button is inside it (`[data-ed-stop-slot]` encloses the `<button ... data-ed-cmd="cancel" ... hidden>`); the header file shows `btn('cancel').hidden = !running;` once, `paintActions();` once and a `const running = runs.isRunning();` line; the fourth command prints `0` (the three buttons are gone from the markup); the popover hook is found.

What to do when it differs:
- **No `data-ed-stop-slot`, or `cancel` is outside it:** B is not finished. Stop and tell the coordinator; Part C does not add the slot (it is B's, with its CSS width).
- **`btn('cancel').hidden = !running;` is gone or written another way** (for example B moved the visibility into the slot): `c-1-header.mjs` (Step 4) fails on that anchor. Keep this meaning in the edit: for a flow editor the line `ED.runStop.sync(ed, <the cancel button>, t, id => runs.stopPending(id))` replaces whatever hides or shows the button, and runs after `paintActions()`; for a block editor the old line stays. The rest of the part does not depend on the way the hiding is written, **except** that every test below reads `button.hidden` or computed visibility of `[data-ed-cmd="cancel"]`: if B shows and hides the Stop by another means (a class on the slot), say so in the review note and make `sync` set that means too.
- **B's U1 test (`scripts/test-easydrag-extra7.mjs`, "the header hides the text of Undo, Redo and More, and spares Stop, Test and Publish") and its stylesheet test in `scripts/test-easydrag-header.mjs` already expect the Stop to keep its text below 900 px; they need no change from this part, except the slot width in the second (Step 4).

- [ ] **Step 2: Write the failing tests**

Write `scripts/test-easydrag-stop.mjs` with the Write tool:

```js
#!/usr/bin/env node
// W1c C.1: the one Stop. ED.runStop.focusRun names the run the header's Stop stops: the viewed run while it is
// active and can be cancelled (durable runs by their checkpoint), else the editor's own run, also while a past
// run is open (the parked run). paint gives the button its words, sync shows or hides it and hands on the focus.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');

// A button just big enough for paint and sync: attributes, classes, a span child, focus.
function button(text) {
    const attrs = new Map();
    const span = { textContent: text };
    const b = {
        hidden: false, disabled: false, focused: false, span,
        setAttribute: (k, v) => attrs.set(k, String(v)), getAttribute: k => (attrs.has(k) ? attrs.get(k) : null), removeAttribute: k => attrs.delete(k),
        classList: { on: new Set(), toggle(c, on) { if (on) this.on.add(c); else this.on.delete(c); } },
        querySelector: sel => (sel === 'span' ? span : null),
        focus() { box.document.activeElement = b; b.focused = true; }
    };
    return b;
}
const box = vm.createContext({ window: {}, console, document: { activeElement: null }, Set, Map, Object, Array });
for (const file of ['easydrag-durable.js', 'easydrag-run-stop.js']) vm.runInContext(read(file), box, { filename: file });
const ED = box.window.EasyDrag;
const t = key => key.replace('easydrag.ui.', '');

const own = (status, extra) => Object.assign({ id: 'r_own', status, mode: 'test', steps: new Map() }, extra);
const viewed = (status, extra) => Object.assign({ id: 'r_view', status, mode: 'live', started_at: '2026-11-16T10:00:00Z' }, extra);
const host = (patch) => Object.assign({ run: null, runView: null, parkedRun: null }, patch);
const ids = ed => { const f = ED.runStop.focusRun(ed); return f ? [f.id, f.mode, f.status] : null; };

// 1. Nothing runs: no run in focus.
assert.equal(ED.runStop.focusRun(host()), null, 'idle: no run in focus');

// 2. The editor's own run: queued, waiting and running count, a final one does not; a stale run only when durable.
assert.deepEqual(ids(host({ run: own('running') })), ['r_own', 'test', 'running']);
assert.deepEqual(ids(host({ run: own('queued', { mode: 'live' }) })), ['r_own', 'live', 'queued']);
assert.equal(ED.runStop.focusRun(host({ run: own('success') })), null, 'a finished own run has no Stop');
assert.equal(ED.runStop.focusRun(host({ run: own('running', { stale: true }) })), null, 'a run whose stream was given up has no Stop');
assert.deepEqual(ids(host({ run: own('running', { stale: true, durableKnown: true }) })), ['r_own', 'test', 'running'], 'a durable run stays stoppable');
const record = ED.runStop.focusRun(host({ run: own('running', { record: { id: 'r_own', started_at: '2026-11-16T09:00:00Z', durable: true } }) }));
assert.equal(record.started_at, '2026-11-16T09:00:00Z', 'the run record travels with the focus (the confirmation names its start)');
assert.equal(record.durable, true);

// 3. The review of a run that is still active: the viewed run is in focus (it is ed.run too, as a review).
const review = (status, extra, ed) => host(Object.assign({ runView: { run: viewed(status, extra) }, run: own(status, { id: 'r_view', view: true }) }, ed));
assert.deepEqual(ids(review('waiting')), ['r_view', 'live', 'waiting']);
assert.equal(ED.runStop.focusRun(review('success')), null, 'a finished viewed run has no Stop');
assert.equal(ED.runStop.focusRun(review('waiting', { durable: true })), null, 'a durable run without a checkpoint cannot be stopped from here');
const checkpoint = (generation, cancel) => ({ run_id: 'r_view', state: 'waiting', generation, cancel_available: cancel });
assert.deepEqual(ids(review('waiting', { durable: true }, { run: own('waiting', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'waiting' }, durableStatus: checkpoint(2, true) }) })), ['r_view', 'live', 'waiting'], 'a checkpoint that allows the cancel puts the run in focus');
assert.equal(ED.runStop.focusRun(review('waiting', { durable: true }, { run: own('waiting', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'waiting' }, durableStatus: checkpoint(2, false) }) })), null, 'a checkpoint that forbids the cancel takes the run out of focus');
assert.equal(ED.runStop.focusRun(review('waiting', {}, { run: own('waiting', { id: 'r_view', view: true, durableStatusError: true }) })), null, 'a lost checkpoint read takes the proof away');
// A failed run whose legacy checkpoint still offers the cleanup keeps its Stop (the banner's rule).
assert.deepEqual(ids(review('error', { durable: true }, { run: own('error', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'error' }, durableStatus: { run_id: 'r_view', state: 'blocked', generation: 2, cancel_available: true, blocked_code: 'incompatible' } }) })), ['r_view', 'live', 'error']);

// 4. A past run is open and the editor's own run is parked behind it: the Stop stops the own run.
const parked = { id: 'r_own', mode: 'test', record: { id: 'r_own', status: 'running', started_at: '2026-11-16T09:30:00Z' } };
const past = review('success', {}, { parkedRun: parked });
assert.deepEqual(ids(past), ['r_own', 'test', 'running']);
assert.equal(ED.runStop.focusRun(past).started_at, '2026-11-16T09:30:00Z');
// An active viewed run wins over the parked one; the parked run that is the viewed run adds nothing.
assert.deepEqual(ids(review('running', {}, { parkedRun: parked })), ['r_view', 'live', 'running']);
assert.equal(ED.runStop.focusRun(review('success', { id: 'r_own' }, { parkedRun: parked })), null, 'the parked run itself, shown as finished, is nothing to stop');

// 5. paint: the words, the busy state, the name.
const b = button('Stop');
ED.runStop.paint(b, t, true);
assert.deepEqual([b.span.textContent, b.disabled, b.getAttribute('aria-busy'), b.getAttribute('aria-label'), b.getAttribute('title'), b.classList.on.has('is-busy')], ['run_stopping', true, 'true', 'run_stopping', 'run_stopping', true]);
ED.runStop.paint(b, t, false);
assert.deepEqual([b.span.textContent, b.disabled, b.getAttribute('aria-busy'), b.getAttribute('aria-label'), b.classList.on.has('is-busy')], ['run_cancel', false, null, 'run_cancel', false], 'the label comes back');

// 6. sync: shown for a run in focus, hidden without; pending follows the run in focus; the focus is handed on.
const exit = button('Back'), canvas = button('');
const root = { querySelector: sel => (sel === '[data-ed-cmd="exit-run-view"]' ? exit : sel === '.ed-canvas' ? canvas : null) };
const stop = button('Stop');
let ed = host({ root, run: own('running') });
assert.deepEqual([ED.runStop.sync(ed, stop, t, () => false) !== null, stop.hidden, stop.disabled], [true, false, false]);
ED.runStop.sync(ed, stop, t, id => id === 'r_own');
assert.deepEqual([stop.span.textContent, stop.disabled], ['run_stopping', true], 'a stop in flight shows on the Stop of that run');
ED.runStop.sync(host({ root, runView: { run: viewed('waiting') }, run: own('waiting', { id: 'r_view', view: true }) }), stop, t, id => id === 'r_own');
assert.deepEqual([stop.span.textContent, stop.disabled], ['run_cancel', false], 'the pending state of another run does not show');
stop.focus();
ED.runStop.sync(host({ root, runView: { run: viewed('success') }, run: own('success', { id: 'r_view', view: true }) }), stop, t, () => false);
assert.deepEqual([stop.hidden, box.document.activeElement === exit], [true, true], 'a Stop that goes away in the review hands the focus to "Back to draft"');
stop.hidden = false; stop.focus();
ED.runStop.sync(host({ root, run: own('success') }), stop, t, () => false);
assert.deepEqual([stop.hidden, box.document.activeElement === canvas], [true, true], 'else the focus goes to the canvas');
canvas.focused = false;
stop.hidden = false; box.document.activeElement = exit;
ED.runStop.sync(host({ root, run: own('success') }), stop, t, () => false);
assert.equal(canvas.focused, false, 'a focus elsewhere stays');

console.log('ok   the one Stop: focus rules, words, pending, focus hand-over');
```

Run it (fails because `ED.runStop` does not exist):

```
cd W && node scripts/test-easydrag-stop.mjs
```

Expected: `TypeError: Cannot read properties of undefined (reading 'focusRun')`.

Add the editor-level test (a real editor: the own test runs, a past run is opened, the Stop stops the own run, not the past one):

```
cd W && node P/c-1-tests.mjs
```

Expected: `c-1 test edit applied`. The script (written to `P/c-1-tests.mjs`):

```js
// Task C.1 (test): a past run is open while the editor's own test runs; the one Stop stops the own run.
// Run from the worktree root: node C:/disposable/easydrag-w1c/patches/c-1-tests.mjs
import { edit } from '../lib-patch.mjs';

edit('scripts/test-easydrag-extra6.mjs', `    await guardAsync('a1008 runs shortcuts drop Tab add and keep Ctrl+K plus connect', async () => {`, `    // W1c C.1: a past run is open while the editor's own test still runs. The header's Stop stops the own run.
    await guardAsync('w1c the one Stop stops the own run while a past run is open', async () => {
        const at = '2026-11-16T10:00:00Z';
        const h = sandbox(req => {
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test') return { run_id: 'r_own' };
            if (req.method === 'POST' && req.url === '/api/desktop/flows/runs/r_own/cancel') return {};
            if (req.method === 'GET' && req.url === '/api/desktop/flows/runs/r_old?include=doc') return { run: { id: 'r_old', status: 'success', mode: 'test', started_at: at, revision: 1, trigger_node: T1 }, steps: [], doc: flowDoc() };
            return undefined;
        });
        const editor = openEditor(h);
        await settle();
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const stop = () => editor.el.querySelector('[data-ed-cmd="cancel"]');
        const during = !stop().hidden;
        editor.showRun('r_old');
        await settle();
        const reviewing = [!stop().hidden, editor.ed.runView.run.id, h.dialogs.length];
        stop().fire('click');
        await settle();
        eq('w1c the Stop shows during the own run and in the review of a past run, and stops the own run, not the past one',
            [during, reviewing, postsTo(h, '/api/desktop/flows/runs/r_own/cancel'), postsTo(h, '/api/desktop/flows/runs/r_old/cancel')],
            [true, [true, 'r_old', 1], 1, 0]);
        editor.dispose();
    });

    await guardAsync('a1008 runs shortcuts drop Tab add and keep Ctrl+K plus connect', async () => {`);

console.log('c-1 test edit applied');
```

```
cd W && AURAGO_EASYDRAG_TEST_FILTER="w1c the one Stop" node scripts/test-easydrag.mjs 2>&1 | grep "FAIL\|failure"
```

Expected (fails because the Stop is hidden in the review and nothing posts a cancel):

```
FAIL w1c the Stop shows during the own run and in the review of a past run, and stops the own run, not the past one — got [true,[false,"r_old",1],0,0] want [true,[true,"r_old",1],1,0]
1 failure(s)
```

- [ ] **Step 3: Implement the component and its wiring**

Write `P/c-1.mjs` with the Write tool, then run it:

```js
// Task C.1: the one Stop component. Run from the worktree root: node C:/disposable/easydrag-w1c/patches/c-1.mjs
import { edit } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-run-stop.js: the component, the pending state of the one button, cancel() through the run in focus ──
const stop = apps + 'easydrag-run-stop.js';

edit(stop, `(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // installStop installs`, `(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FINAL = ['success', 'error', 'cancelled'];
    const ACTIVE = ['queued', 'waiting', 'running'];

    // ── the one Stop ─────────────────────────────────────────────────────────────
    // The header holds one Stop button (data-ed-cmd="cancel", in the reserved slot). It stops the run in
    // focus: the run the review shows while that run is still active and can be cancelled (a durable
    // run too: its checkpoint says so), else the editor's own run, also while a past run is open (the
    // own run is parked behind the review, ed.parkedRun). focusRun returns that run as a record that
    // stopRun takes, or null: then the slot is empty and keeps its width.

    // viewedStoppable: the rule the banner's Stop used. A durable run asks its checkpoint; a stored
    // run that has no checkpoint but says "durable" cannot be stopped from here; any other run can
    // while it is queued, waiting or running. A lost checkpoint read takes the proof away.
    function viewedStoppable(ed, run) {
        if (ed.run && ed.run.durableStatusError) return false;
        const status = ED.durable && ED.durable.info(ed.run);
        return status ? !!status.cancel_available : !run.durable && ACTIVE.includes(run.status);
    }

    // ownStoppable is the rule of runs.isRunning(): a live run, not a review, not one whose stream was given up.
    function ownStoppable(ed) {
        const run = ed.run;
        if (!run || run.view || FINAL.includes(run.status)) return false;
        return !run.stale || !!(ED.durable && ED.durable.has(run));
    }

    function focusRun(ed) {
        const view = ed.runView && ed.runView.run;
        if (view && viewedStoppable(ed, view)) return view;
        if (view) {
            const parked = ed.parkedRun;
            return parked && parked.id !== view.id ? Object.assign({ id: parked.id, status: 'running', mode: parked.mode || 'test' }, parked.record || {}) : null;
        }
        return ownStoppable(ed) ? Object.assign({ id: ed.run.id, status: ed.run.status, mode: ed.run.mode || 'test' }, ed.run.record || {}) : null;
    }

    // paint gives the Stop its words: "Stop" (always shown, at every width), "Stopping…" while the
    // request is out. The name follows the text, so a screen reader hears what the eye reads.
    function paint(button, t, pending, ed) {
        const name = t(pending ? 'easydrag.ui.run_stopping' : 'easydrag.ui.run_cancel');
        button.disabled = !!pending;
        if (pending) button.setAttribute('aria-busy', 'true'); else button.removeAttribute('aria-busy');
        button.classList.toggle('is-busy', !!pending);
        button.setAttribute('aria-label', name);
        button.setAttribute('title', name);
        const span = button.querySelector('span');
        if (span && span.textContent !== name) span.textContent = name;
        // The registry paints the availability (aria-disabled and its reason) from the same facts, now that the busy state changed.
        if (ed && ED.actions) ED.actions.paint(button, 'flow.stop', { host: ed });
    }

    // sync shows the Stop for the run in focus (or hides it) and paints its pending state; the editor's
    // header calls it on every render. pendingOf(id) tells whether the stop of that run is in flight.
    // A Stop that goes away while it has the focus hands the focus on: "Back to draft" in the review,
    // else the canvas.
    function sync(ed, button, t, pendingOf) {
        const focus = focusRun(ed);
        const hadFocus = button === document.activeElement;
        button.hidden = !focus;
        paint(button, t, !!focus && !!pendingOf(focus.id), ed);
        if (!focus && hadFocus) {
            const next = (ed.runView && ed.root.querySelector('[data-ed-cmd="exit-run-view"]')) || ed.root.querySelector('.ed-canvas');
            if (next && next.focus) next.focus({ preventScroll: true });
        }
        return focus;
    }

    // installStop installs`);

edit(stop, `        async function cancel() {
            if (!ed.run || isFinal(ed.run.status) && !ED.durable?.info(ed.run)?.cancel_available) return;
            const record = Object.assign({ id: ed.run.id, status: ed.run.status, mode: ed.run.mode || 'test' }, ed.run.record || {});
            return stopRun(record);
        }`, `        // cancel is the header Stop: it stops the run in focus (focusRun above).
        async function cancel() {
            const record = ED.runStop.focusRun(ed);
            return record ? stopRun(record) : undefined;
        }`);

edit(stop, `        function setStopPending(id, on) {
            stopButtons(id).forEach(b => {
                b.disabled = !!on;`, `        function setStopPending(id, on) {
            if (on) rc.pendingStops.add(id); else rc.pendingStops.delete(id);
            stopButtons(id).forEach(b => {
                if (b.dataset.edCmd === 'cancel') { ED.runStop.paint(b, t, !!on, ed); return; }
                b.disabled = !!on;`);

edit(stop, `                b.classList.toggle('is-busy', !!on);
                // The header's Stop is named after its text (its text is hidden on narrow headers):
                // the name says "Stopping…" as long as the text does.
                if (b.dataset.edCmd === 'cancel') {
                    const name = on ? t('easydrag.ui.run_stopping') : t('easydrag.ui.run_cancel');
                    b.setAttribute('aria-label', name);
                    b.setAttribute('title', name);
                }
                const span`, `                b.classList.toggle('is-busy', !!on);
                const span`);

edit(stop, `    (ED.runParts = ED.runParts || {}).stop = installStop;`, `    ED.runStop = { focusRun, paint, sync };
    (ED.runParts = ED.runParts || {}).stop = installStop;`);

// ── easydrag-runs.js: the pending ids, the parked run, the header's Stop belongs to the run in focus ──
const runs = apps + 'easydrag-runs.js';

edit(runs, `                if (b.dataset.edCmd === 'cancel') return !!(ed.run && ed.run.id === id && !ed.run.view);`, `                if (b.dataset.edCmd === 'cancel') { const focus = ED.runStop.focusRun(ed); return !!focus && focus.id === id; }`);

edit(runs, `            // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
            stopping: new Set(),`, `            // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
            // pendingStops holds the runs whose stop request is out (the Stop says "Stopping…" for them).
            stopping: new Set(),
            pendingStops: new Set(),`);

edit(runs, `        Object.assign(rc, ED.runParts.stream(rc));`, `        // ed.parkedRun is the live run that a review parked ({id, mode, record}): the one Stop reads it (focusRun).
        Object.defineProperty(ed, 'parkedRun', { configurable: true, get: () => rc.parked });

        Object.assign(rc, ED.runParts.stream(rc));`);

edit(runs, `            isRunning: rc.isRunning, hasParked: () => !!rc.parked,`, `            isRunning: rc.isRunning, hasParked: () => !!rc.parked, stopPending: id => rc.pendingStops.has(id),`);

// ── easydrag-run-drawer.js: a parked run keeps its record (the Stop confirms and checks durability with it) ──
edit(apps + 'easydrag-run-drawer.js', `{ rc.parked = { id: ed.run.id, mode: ed.run.mode }; watchParked(); }`, `{ rc.parked = { id: ed.run.id, mode: ed.run.mode, record: ed.run.record }; watchParked(); }`);

console.log('c-1 JS edits applied');
```

```
cd W && node P/c-1.mjs
```

Expected: `c-1 JS edits applied`. What it does, so that the diff can be read: in `easydrag-run-stop.js` it adds `viewedStoppable`, `ownStoppable`, `focusRun`, `paint`, `sync` above `installStop`, exports them as `ED.runStop`, makes `cancel()` stop `focusRun(ed)`, and lets `setStopPending` record the pending run in `rc.pendingStops` and paint the header button through `paint`. In `easydrag-runs.js` the header branch of `stopButtons` asks `focusRun`, `rc.pendingStops` is new, `ed.parkedRun` is a getter on the editor object (read by `focusRun`; the block editor has none, and `focusRun` is not used for blocks), and the facade gets `stopPending(id)`. In `easydrag-run-drawer.js` the parked run keeps its record.

Why `paint` also calls `ED.actions.paint(button, 'flow.stop', { host: ed })`: the registry's check for `flow.stop` reads `aria-busy`, so when the busy state ends the registry state (`aria-disabled`, its reason) has to be painted again, else the Stop stays `aria-disabled` until the next header render and ignores the next click (the durable test "uncertain Stop is not replayed" found this).

```
cd W && node scripts/test-easydrag-stop.mjs
cd W && AURAGO_EASYDRAG_TEST_FILTER="w1c the one Stop" node scripts/test-easydrag.mjs 2>&1 | grep "FAIL\|failure\|all EasyDrag"
```

Expected: `ok   the one Stop: focus rules, words, pending, focus hand-over`, then `all EasyDrag checks passed` (the editor test reports `ok`).

- [ ] **Step 4: Wire the header, set the slot width (the edits that sit on B's work)**

Write `P/c-1-header.mjs` and run it:

```js
// Task C.1 (header, CSS, slot width): the parts of C.1 that sit on B's work. Run from the worktree root after c-1.mjs:
//   node C:/disposable/easydrag-w1c/patches/c-1-header.mjs
import { edit } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-header.js: the Stop follows the run in focus (after paintActions: the registry paints the title first) ──
edit(apps + 'easydrag-header.js', `            btn('cancel').hidden = !running;`, `            // A block editor's simulation has its own Stop: shown while the simulation runs.
            if (isBlock) btn('cancel').hidden = !running;`);
edit(apps + 'easydrag-header.js', `            paintActions();`, `            paintActions();
            // The one Stop, for the run in focus (easydrag-run-stop.js).
            if (!isBlock) ED.runStop.sync(ed, btn('cancel'), t, id => runs.stopPending(id));`);

// ── CSS: the slot's width from the real labels, and the Stop's words on one line ──
const css = 'ui/css/desktop-app-easydrag.css';
// B's guess was 9.5rem (152 px). The longest words of the Stop in the 16 locales are the "Stopping" texts: hi 15 characters,
// de and pl 14. At the header font (14 px, weight 600) such a text is about 105-112 px; the icon (16), the gap (6), the
// padding (24) and the border (2) add 48, so the button needs about 160 px: 9.5rem is too tight, 10.5rem (168 px) fits.
edit(css, `width: var(--ed-stop-slot-w, 9.5rem); }`, `width: var(--ed-stop-slot-w, 10.5rem); }`);
edit(css, `    .ed-head-actions .ed-btn[data-ed-cmd="publish"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="cancel"] > span { display: inline; white-space: normal; }`, `    .ed-head-actions .ed-btn[data-ed-cmd="publish"] > span { display: inline; white-space: normal; }
    /* The Stop's words stay on one line at every width: the slot holds the longest of them. */
    .ed-head-actions .ed-btn[data-ed-cmd="cancel"] > span { display: inline; white-space: nowrap; }`);
edit(css, `.ed-stop-slot > .ed-btn { width: 100%; min-width: 0; }`, `.ed-stop-slot > .ed-btn { width: 100%; min-width: 0; white-space: nowrap; }`);

// ── B's stylesheet test names the slot's width: it follows ──
edit('scripts/test-easydrag-header.mjs', `/width: var\\(--ed-stop-slot-w, 9\\.5rem\\)/`, `/width: var\\(--ed-stop-slot-w, 10\\.5rem\\)/`);

console.log('c-1 header, CSS and slot width edits applied');
```

```
cd W && node P/c-1-header.mjs
```

Expected: `c-1 header, CSS and slot width edits applied`. The three groups:
- **Header.** `renderHeader` no longer hides the Stop by `!running` for a flow editor: after `paintActions()` it calls `ED.runStop.sync`. It runs after `paintActions()` because the registry paints the button's title first. A block editor keeps `btn('cancel').hidden = !running` (its simulation has its own Stop).
- **CSS and slot width.** B already spares the Stop's text in the 900 px rule. This script (a) sets the slot width from the real labels: B's guess `9.5rem` (152 px) is too tight. The longest Stop words in the 16 locales are the "Stopping" texts (`hi` 15 characters, `de` and `pl` 14; the idle words are at most 9: `pl` "Zatrzymaj"). At the header font (14 px, weight 600) the longest text is about 105-112 px; icon (16), gap (6), padding (24) and border (2) add 48, so the button needs about 160 px; the slot becomes **`10.5rem` (168 px)**. This is an estimate from the font size, not a measurement of every locale; C.3 measures the German words in a browser and `--ed-stop-slot-w` can be raised in one place if a locale clips. (b) keeps the Stop's words on one line (`white-space: nowrap` on the button and on its span in the 900 px rule). (c) changes B's stylesheet test, which pins the slot width, to `10.5rem`.

- [ ] **Step 5: The key file**

`easydrag.ui.run_stopping` reads "Stoppt…" in German today; the spec's words are "Wird gestoppt…". The key is also the word of the banner and the status text of a durable run that is being stopped (`ED.durable.label`), where "Wird gestoppt…" reads as well. The other 15 locales keep their texts; the key file carries all 16 (idempotent).

| Key | en | de |
|---|---|---|
| `easydrag.ui.run_stopping` | Stopping… | Wird gestoppt… |

Write `plans/2026-11-16-easydrag-w1c/keys/c-1-stopping.json` (the coordinator commits it to main before this part starts; if it is missing, ask for it, do not write it by hand). Its content:

```json
{
  "easydrag.ui.run_stopping": {
    "cs": "Zastavování…",
    "da": "Stopper…",
    "de": "Wird gestoppt…",
    "el": "Διακόπτεται…",
    "en": "Stopping…",
    "es": "Deteniendo…",
    "fr": "Arrêt…",
    "hi": "रोका जा रहा है…",
    "it": "Arresto…",
    "ja": "停止しています…",
    "nl": "Stoppen…",
    "no": "Stopper…",
    "pl": "Zatrzymywanie…",
    "pt": "Parando…",
    "sv": "Stoppar…",
    "zh": "正在停止…"
  }
}
```

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/c-1-stopping.json
```

Expected: `plans/2026-11-16-easydrag-w1c/keys/c-1-stopping.json: 1 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`.

Validation of the key file (run once): `node -e "const k=JSON.parse(require('fs').readFileSync('plans/2026-11-16-easydrag-w1c/keys/c-1-stopping.json','utf8'));const L='cs da de el en es fr hi it ja nl no pl pt sv zh'.split(' ');for(const [key,v] of Object.entries(k)){const m=L.filter(l=>!v[l]||!v[l].trim());console.log(key,m.length?'MISSING '+m:'16 ok')}"` prints `easydrag.ui.run_stopping 16 ok`. The text has no placeholder; the lints `TestEasyDragGermanUsesLowercaseDu` and `TestEasyDragLocalesUseLedgerWords` read it (no "Du", no ledger word); it is not a count key, so the plural lists are not touched.

- [ ] **Step 6: Run the gates**

```
cd W && node scripts/test-easydrag.mjs 2>&1 | tail -3
cd W && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-runs.js
```

Expected: the first prints `all EasyDrag checks passed` (on the scratch copy with B: about 2 minutes, 1256 `ok` lines, no FAIL); the second prints `ok   render baseline XL counters equal the baseline` and `PASS render baseline (unchanged)` (4 s and 17 s on two scratch runs; watch the wall time, up to 25 s is normal); the Go run prints `ok  github.com/.../ui` (static tests: seam pins, key lints, plural lists, menu lint); the editor file is unchanged in size in this task, `easydrag-runs.js` stays at most 400 lines (140 on the scratch copy).

- [ ] **Step 7: Commit**

Write the message to `C:/disposable/easydrag-w1c/c1-msg.txt`:

```
feat(easydrag): one Stop component for the run in focus

ED.runStop.focusRun(ed) names the run the header's Stop stops: the viewed
run while it can be stopped (the banner's rule: a durable run by its
checkpoint, any other run while queued, waiting or running), else the
editor's own run, also while a past run is open and the own run is parked
behind it. The Stop shows for that run, says "Stop" at every width and
"Stopping..." while the request is out, and hands its focus on when it goes
away. cancel() stops the run in focus; the confirmation for live and
durable runs is unchanged. The German "Stoppt..." becomes "Wird gestoppt...".

The banner's and the drawer's Stops stay until the next commit.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add scripts/test-easydrag-stop.mjs scripts/test-easydrag-extra6.mjs scripts/test-easydrag-header.mjs ui/js/desktop/apps/easydrag-run-stop.js ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-drawer.js ui/js/desktop/apps/easydrag-header.js ui/css/desktop-app-easydrag.css ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-w1c/c1-msg.txt
```

(The message uses plain ASCII dots where the German text has an ellipsis character; the code has the real "…" through the key file.)

---

### Task C.2: Remove the banner's and the drawer's Stops; move the tests

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (banner markup, `renderBanner`, `stop-viewed` click), `ui/js/desktop/apps/easydrag-runs.js` (`stopButtons`), `ui/js/desktop/apps/easydrag-run-stop.js` (`setStopPending`, two comments), `ui/js/desktop/apps/easydrag-run-drawer.js` (`runRow`, `drawRuns`, `loadRuns` focus, click handler, two constants), `ui/css/desktop-app-easydrag.css` (`.ed-run-item` rules), `ui/lang/easydrag/*.json` (key file `c-2-drawer.json`: `run_stop_label` goes), `documentation/manuals/en/24-easydrag.md`, `documentation/manuals/de/24-easydrag.md`
- Create: `ui/desktop_easydrag_stop_helpers_browser_test.go` (`stopsVisible`, `waitStop`, `clickStop`, `wantStops`)
- Modify tests: `scripts/test-easydrag-durable.mjs`, `-extra4.mjs`, `-extra5.mjs`, `-extra6.mjs`, `ui/desktop_easydrag_browser_test.go`, `ui/desktop_easydrag_durable_browser_test.go`, `ui/desktop_easydrag_actions_browser_test.go`

**Tests that clicked a removed Stop, and where they go** (none is weakened; where an assertion was about the removed button itself it is replaced by the check that the button is gone):

| Test | Was | Now |
|---|---|---|
| `scripts/test-easydrag-durable.mjs` lines 33, 106, 125, 128, 182, 189, 197, 199 (helper `stop`, 8 reads of `stop-viewed`) | banner Stop of the viewed durable run (a real editor) | header `[data-ed-cmd="cancel"]`: same rule, same expectations (`hidden`, `disabled`, counts of `/durable/cancel` posts) |
| `scripts/test-easydrag-extra5.mjs:713-714` (read-only desktop: Stop shows in the run view and posts the cancel; Test, Publish, the switch and Delete stay off) | `btn('stop-viewed')` | `btn('cancel')`, same expectation `[true,true,true,true,true]`, 1 cancel, 0 saves |
| `scripts/test-easydrag-extra6.mjs:725-751` ("a1008 runs stop covers queued waiting and running, test now, live after confirm") | row Stops of the drawer, then the banner Stop | rows open their run (`[data-ed-run]` click), the header Stop stops it. Same cases: queued test stops at once and shows pending, live asks and "keep" posts nothing, "stop" posts, finished run stays quiet (409), a run that ended has no Stop. New in it: no row has a Stop, and the pending state of one run does not show on the Stop of the next run in focus |
| `scripts/test-easydrag-extra4.mjs:281-325, 347` ("c1d15b stop runs") and `:373-449` ("c1d15b stop follow-ups") | row Stops of the drawer, the harness has no header | `stopRow(id)` passes the run the row names to `runs.stopRun` (the call the Stop makes after `focusRun`), `rowOf(id)` is the row; the focus checks now hold for the row. Dropped with their button: the Stop's name with the start time (`run_stop_label`, #8; one Stop has one name) and the "focus stays on the Stop" assertion (#9 now holds for the row) |
| `ui/desktop_easydrag_browser_test.go:719-741` (`stopWaitingRuns`) | row Stop, then banner Stop | row click opens the run, `s.clickStop()` stops it; exactly one Stop checked before and after |
| `ui/desktop_easydrag_durable_browser_test.go:34, 49, 51, 57` | banner `stop-viewed` | `s.clickStop()` and the header `cancel` for the `hidden` checks (the first click happens at 320 px) |
| `ui/desktop_easydrag_actions_browser_test.go:223-237` | header `cancel` `MustClick` | `s.clickStop()` (the race-safe click) |
| `scripts/test-easydrag-extra7.mjs:273-298`, `-blocks.mjs:153`, `ui/desktop_easydrag_ux_browser_test.go:261, 267` | the header `cancel` | **no change**: they read or click the header Stop, which keeps its hook and its behaviour (`headerNames` forces `hidden=false` on the button for a moment; `sync` repaints it on the next render) |

- [ ] **Step 1: Add the browser helpers**

The moved tests and C.3's tests need helpers that count and click the Stop. Write `ui/desktop_easydrag_stop_helpers_browser_test.go` with the Write tool (LF is fine; git normalises):

```go
package ui

import (
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// easyDragVisibleJS defines vis(el) for page scripts: the control is not hidden, has a real size and none of its
// ancestors is display:none, visibility:hidden or fully transparent. The Stop slot keeps its width when idle, so
// "the button exists" and "the button can be seen" differ.
const easyDragVisibleJS = `const vis=el=>{if(!el||el.hidden)return false;const r=el.getBoundingClientRect();if(r.width<2||r.height<2)return false;for(let n=el;n&&n.nodeType===1;n=n.parentElement){const c=getComputedStyle(n);if(c.display==='none'||c.visibility==='hidden'||c.opacity==='0')return false;}return true;};`

// stopsVisible counts the controls of the open editor that a user reads as a Stop: the header's button, the old
// banner and row hooks, and any other button whose text or name is the Stop word or the Stopping word. A
// confirmation dialog's own "Stop" button does not count (it asks, it does not stop by itself).
func (s *easyDragSmoke) stopsVisible() int {
	s.t.Helper()
	return s.page.MustEval(`()=>{` + easyDragVisibleJS + `
		const ed=edFixture.editor(),words=[ed.t('easydrag.ui.run_cancel'),ed.t('easydrag.ui.run_stopping')];
		return [...ed.root.querySelectorAll('button')].filter(b=>!b.closest('.ed-modal-backdrop')&&vis(b)&&(['cancel','stop-viewed'].includes(b.dataset.edCmd)||b.hasAttribute('data-ed-run-stop')||words.includes(b.textContent.trim())||words.includes(b.getAttribute('aria-label')||''))).length;}`).Int()
}

// waitStop waits until the header Stop can be pressed: visible, enabled, not busy, and at the same place in two
// samples in a row (the header repaints on every run event; a click on a button that was just replaced fails with
// "element has no visible shape").
func (s *easyDragSmoke) waitStop() {
	s.t.Helper()
	s.wait(`()=>{` + easyDragVisibleJS + `
		const b=document.querySelector('[data-ed-cmd="cancel"]');
		if(!vis(b)||b.disabled||b.getAttribute('aria-disabled')==='true'||b.getAttribute('aria-busy')==='true'){window.__stopRect='';return false;}
		const r=b.getBoundingClientRect(),key=[r.left,r.top,r.width,r.height].map(Math.round).join(',');
		const same=window.__stopRect===key;window.__stopRect=key;return same;}`)
}

// clickStop clicks the real header Stop once it is ready, and tries again when the page repainted under the click.
func (s *easyDragSmoke) clickStop() {
	s.t.Helper()
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		s.waitStop()
		el, err := s.page.Element(`[data-ed-cmd="cancel"]`)
		if err == nil {
			if err = el.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return
			}
		}
		last = err
		time.Sleep(60 * time.Millisecond)
	}
	s.t.Fatalf("the header Stop could not be clicked: %v", last)
}

// wantStops fails the test unless exactly n Stop controls are visible.
func (s *easyDragSmoke) wantStops(n int, when string) {
	s.t.Helper()
	if got := s.stopsVisible(); got != n {
		s.t.Fatalf("%s: %d Stop controls are visible, want %d", when, got, n)
	}
}
```

`stopsVisible` counts what a user reads as a Stop (the header's, the old banner and row hooks, any button named "Stop"/"Stopping…", not the confirmation dialog's own button), `waitStop` waits for a visible, enabled, not busy button that stood still for two samples, `clickStop` clicks the real button and retries when the page repainted under the click (the W1b landing found "element has no visible shape" on a Stop click), `wantStops(n, when)` fails the test with the count.

```
cd W && gofmt -l ui/desktop_easydrag_stop_helpers_browser_test.go && GOTOOLCHAIN=go1.27.4 go vet ./ui
```

Expected: no output (gofmt) and a clean `go vet`.

- [ ] **Step 2: Move the tests to the header Stop**

The Node test edits (written to `P/c-2-tests.mjs`; the old text of the `extra6` edit is the current test, 35 lines):

```js
// Task C.2 (tests): every test that clicked the banner's Stop or a row's Stop goes through the header's Stop,
// or, where the harness has no editor, through stopRun with the run the row named. Run from the worktree root:
//   node C:/disposable/easydrag-w1c/patches/c-2-tests.mjs
import { edit } from '../lib-patch.mjs';

// ── scripts/test-easydrag-durable.mjs: the viewed durable run is stopped from the header ──
edit('scripts/test-easydrag-durable.mjs', `[data-ed-cmd="stop-viewed"]`, `[data-ed-cmd="cancel"]`, 8);

// ── scripts/test-easydrag-extra5.mjs: a read-only desktop stops the viewed run from the header ──
const extra5 = 'scripts/test-easydrag-extra5.mjs';
edit(extra5, `const before = [!btn('stop-viewed').hidden,`, `const before = [!btn('cancel').hidden,`);
edit(extra5, `        btn('stop-viewed').fire('click');`, `        btn('cancel').fire('click');`);

// ── scripts/test-easydrag-extra4.mjs: no row has a Stop; stopRun is called with the run the row names ──
const extra4 = 'scripts/test-easydrag-extra4.mjs';
edit(extra4, `        const stopOf = id => drawer().querySelector('[data-ed-run-stop="' + id + '"]');`, `        // A row has no Stop of its own (W1c): its click opens the run, and the header's one Stop (the real editor's tests
        // click it) passes the run in focus to stopRun. This harness has no header: stopRow passes the run the row names.
        const stopRow = id => runs.stopRun(listed.find(r => r.id === id));
        const rowOf = id => drawer().querySelector('[data-ed-run="' + id + '"]');`, 2);
edit(extra4, `        const label = stopOf('r_wait') && stopOf('r_wait').getAttribute('aria-label');
        const shown = [!!stopOf('r_wait'), !!stopOf('r_done'), !!stopOf('r_test'), String(label).startsWith('run_stop_label:') && label.length > 'run_stop_label:'.length,
            stopOf('r_wait').localName, stopOf('r_wait').parentNode.className];`, `        const shown = [drawer().querySelectorAll('[data-ed-run-stop], .ed-run-item').length, !!rowOf('r_wait'), !!rowOf('r_done'), !!rowOf('r_test'), rowOf('r_wait').localName];`);
edit(extra4, `        stopOf('r_wait').focus();
        stopOf('r_wait').fire('click');
        await settle();
        const asked`, `        rowOf('r_wait').focus();
        stopRow('r_wait');
        await settle();
        const asked`);
// The second stop is of the row the drawer still shows as waiting: the fixture lists the run as cancelled by now.
edit(extra4, `        stopOf('r_wait').focus();
        stopOf('r_wait').fire('click');
        await settle();
        h.top().querySelector('[data-ed-action="stop"]')`, `        rowOf('r_wait').focus();
        runs.stopRun(run('r_wait', 'waiting', 'live'));
        await settle();
        h.top().querySelector('[data-ed-action="stop"]')`);
edit(extra4, `const stopped = [posted(), lists() - before, !!stopOf('r_wait'),`, `const stopped = [posted(), lists() - before, !!rowOf('r_wait'),`);
edit(extra4, `        eq('c1d15b a waiting live run shows Stop in the drawer; Stop asks first and then posts the cancel and refreshes the list',
            [shown, asked, kept, stopped],
            [[true, false, true, true, 'button', 'ed-run-item'], [true, true, []], [null, []], [['r_wait'], 1, false, 'r_wait', []]]);`, `        eq('c1d15b a waiting live run has no Stop in the drawer; stopping it asks first and then posts the cancel and refreshes the list, the focus stays on its row',
            [shown, asked, kept, stopped],
            [[0, true, true, true, 'button'], [true, true, []], [null, []], [['r_wait'], 1, true, 'r_wait', []]]);`);
edit(extra4, `        stopOf('r_test').fire('click');
        await settle();
        const testStop`, `        stopRow('r_test');
        await settle();
        const testStop`);
edit(extra4, `        stopOf('r_gone').fire('click');`, `        stopRow('r_gone');`);
edit(extra4, `        stopOf('r_lost').fire('click');`, `        stopRow('r_lost');`);
edit(extra4, `        const roShown = !!stopOf('r_test');`, `        const roShown = !!rowOf('r_test') && !drawer().querySelector('[data-ed-run-stop]');`);
edit(extra4, `eq('c1d15b a read-only desktop still shows Stop and posts the cancel',`, `eq('c1d15b a read-only desktop lists the running run without a row Stop, and the stop still posts the cancel',`);
edit(extra4, `        // #8: the name carries the seconds; #7: an agent run asks too.
        const label = stopOf('r_a').getAttribute('aria-label');
        stopOf('r_a').fire('click');`, `        // #7: an agent run asks too. (#8, the seconds in the row Stop's name, went with the row Stop: one Stop has one name.)
        stopRow('r_a');`);
edit(extra4, `        // #9: a reload keeps the focus on the Stop of a run that goes on.
        stopOf('r_a').focus();
        runs.refreshShown();
        await settle();
        const kept = h.dom.document.activeElement === stopOf('r_a');`, `        // #9: a reload keeps the focus on the row of a run that goes on.
        rowOf('r_a').focus();
        runs.refreshShown();
        await settle();
        const kept = h.dom.document.activeElement === rowOf('r_a');`);
edit(extra4, `        stopOf('r_t').fire('click');
        await settle();
        const first`, `        stopRow('r_t');
        await settle();
        const first`);
edit(extra4, `        eq('c1d15b a stop asks for any run but a test, names the run to the second, keeps the focus on a Stop that stays, and follows a run no stream shows',
            [label === 'run_stop_label:' + h.ED.core.fmt.dateTime(at, true), h.ED.core.fmt.dateTime(at, true) !== h.ED.core.fmt.dateTime(at), asked, kept, first, second, [delays(), fetches(), lists() - listsBefore, !!stopOf('r_t')]],
            [true, true, true, true, [[1000], 0, 1], [[2000], 1], [[], 2, 2, false]]);`, `        eq('c1d15b a stop asks for any run but a test, keeps the focus on the row of a run that goes on, and follows a run no stream shows',
            [asked, kept, first, second, [delays(), fetches(), lists() - listsBefore, !!rowOf('r_t')]],
            [true, true, [[1000], 0, 1], [[2000], 1], [[], 2, 2, true]]);`);
edit(extra4, `        stopOf('r_s').fire('click');`, `        stopRow('r_s');`);
edit(extra4, `        stopOf('r_t').fire('click');
        await settle();`, `        stopRow('r_t');
        await settle();`);

// ── scripts/test-easydrag-extra6.mjs: the a1008 runs-stop test goes through the header's Stop ──
edit('scripts/test-easydrag-extra6.mjs', `        const stop = id => editor.el.querySelector('[data-ed-run-stop="' + id + '"]');
        const shown = ['rq', 'rw', 'rr', 'rs'].map(id => !!stop(id));
        stop('rq').fire('click');
        await settle();
        const testPending = [stop('rq').disabled, stop('rq').getAttribute('aria-busy'), stop('rq').querySelector('span').textContent.includes(editor.ed.t('easydrag.ui.run_stopping')),
            h.confirms.length, postsTo(h, '/api/desktop/flows/runs/rq/cancel')];
        stop('rw').fire('click');
        await settle();
        const liveDialog = h.dialogs[h.dialogs.length - 1];
        const beforeConfirm = postsTo(h, '/api/desktop/flows/runs/rw/cancel');
        liveDialog.el.querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        const declined = [postsTo(h, '/api/desktop/flows/runs/rw/cancel'), stop('rw').disabled];
        stop('rw').fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        const liveSent = postsTo(h, '/api/desktop/flows/runs/rw/cancel');
        editor.el.querySelector('[data-ed-run="rr"]').fire('click');
        await settle();
        const viewed = editor.el.querySelector('[data-ed-cmd="stop-viewed"]');
        const viewShown = !!(viewed && !viewed.hidden);
        if (viewed) viewed.fire('click');
        await settle();
        cancelMode = 'finished';
        const notesBefore = h.notes.length;
        stop('rw').fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        hold.resolve({});
        await settle();
        eq('a1008 runs every active run has stop; a test stops immediately and pending; a live stop asks, then a finished run stays quiet',
            [shown, testPending, beforeConfirm, declined, liveSent, viewShown, postsTo(h, '/api/desktop/flows/runs/rr/cancel'), h.notes.length - notesBefore],
            [[true, true, true, false], [true, 'true', true, 0, 1], 0, [0, false], 1, true, 1, 0]);`, `        // A row has no Stop of its own (W1c): a click on it opens the run, and the header's one Stop stops the run in focus.
        const stop = () => editor.el.querySelector('[data-ed-cmd="cancel"]');
        const open = async id => { editor.el.querySelector('[data-ed-run="' + id + '"]').fire('click'); await settle(); };
        const rowStops = editor.el.querySelectorAll('[data-ed-run-stop]').length;
        const shown = [];
        for (const id of ['rq', 'rw', 'rr', 'rs']) { await open(id); shown.push(!!(stop() && !stop().hidden)); }
        await open('rq');
        stop().fire('click');
        await settle();
        const testPending = [stop().disabled, stop().getAttribute('aria-busy'), stop().querySelector('span').textContent.includes(editor.ed.t('easydrag.ui.run_stopping')),
            h.confirms.length, postsTo(h, '/api/desktop/flows/runs/rq/cancel')];
        // The stop of rq is still out; the Stop of the next run in focus is its own.
        await open('rw');
        const other = stop().disabled;
        stop().fire('click');
        await settle();
        const liveDialog = h.dialogs[h.dialogs.length - 1];
        const beforeConfirm = postsTo(h, '/api/desktop/flows/runs/rw/cancel');
        liveDialog.el.querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        const declined = [postsTo(h, '/api/desktop/flows/runs/rw/cancel'), stop().disabled];
        stop().fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        const liveSent = postsTo(h, '/api/desktop/flows/runs/rw/cancel');
        await open('rr');
        const viewShown = !!(stop() && !stop().hidden);
        stop().fire('click');
        await settle();
        cancelMode = 'finished';
        const notesBefore = h.notes.length;
        await open('rw');
        stop().fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        hold.resolve({});
        await settle();
        eq('a1008 runs no row has a Stop; the header Stop shows for every run that has not ended; a test stops immediately and pending; a live stop asks, then a finished run stays quiet',
            [rowStops, shown, testPending, other, beforeConfirm, declined, liveSent, viewShown, postsTo(h, '/api/desktop/flows/runs/rr/cancel'), h.notes.length - notesBefore],
            [0, [true, true, true, false], [true, 'true', true, 0, 1], false, 0, [0, false], 1, true, 1, 0]);`);

console.log('c-2 test edits applied');
```

The Go test edits (`P/c-2-go.mjs`):

```js
// Task C.2 (Go browser tests): the tests that clicked the banner's Stop or a row's Stop click the header's Stop.
// Run from the worktree root: node C:/disposable/easydrag-w1c/patches/c-2-go.mjs
import { edit } from '../lib-patch.mjs';

// ── ui/desktop_easydrag_browser_test.go: stopWaitingRuns ──
const browser = 'ui/desktop_easydrag_browser_test.go';
edit(browser, `// stopWaitingRuns adds two live runs that wait for a slot. One is stopped from its row in the runs
// drawer, the other from the banner of its run view; both ask first.`, `// stopWaitingRuns adds two live runs that wait for a slot. A row in the runs drawer has no Stop: it opens its run, and the
// header's one Stop stops it, after asking. Both runs are stopped that way, the second one with the drawer still open.`);
edit(browser, `	s.wait(\`()=>document.querySelectorAll('.ed-drawer .ed-run-item [data-ed-run-stop]').length===2\`)
	// The Stop button's name contains its visible label and names the run's start time.
	s.wait(fmt.Sprintf(\`()=>{const b=document.querySelector('[data-ed-run-stop="%s"]');const name=b.getAttribute('aria-label').toLowerCase();return name.includes(b.textContent.trim().toLowerCase()) && /\\d/.test(name)}\`, first))
	s.shot("runs-drawer-stop")
	s.noRawKeys("runs drawer stop")
	page.MustElement(fmt.Sprintf(\`[data-ed-run-stop="%s"]\`, first)).MustClick()
	s.wait(\`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')\`)
	s.shot("run-stop-confirm")
	s.noRawKeys("run stop confirm")
	page.MustElement(\`.ed-modal [data-ed-action="stop"]\`).MustClick()
	s.wait(fmt.Sprintf(\`()=>edFixture.state.runs.get('%s').run.status==='cancelled' && !document.querySelector('.ed-modal-backdrop') && !document.querySelector('[data-ed-run-stop="%s"]') && document.activeElement===document.querySelector('[data-ed-run="%s"]')\`, first, first, first))
	// The run view of the other waiting run: Stop in the banner, then the banner shows it stopped.
	// The stopped run's run_finished broadcast redraws the drawer about 250 ms later, so the row
	// is found and clicked in one step (a found row can be detached before a separate click).
	page.MustEval(fmt.Sprintf(\`()=>document.querySelector('.ed-run-row[data-ed-run="%s"]').click()\`, second))
	s.wait(\`()=>{const b=document.querySelector('.ed-runview-banner');return !b.hidden && !b.querySelector('[data-ed-cmd="stop-viewed"]').hidden}\`)
	s.settle()
	s.shot("run-view-stop")
	page.MustElement(\`.ed-runview-banner [data-ed-cmd="stop-viewed"]\`).MustClick()
	s.wait(\`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')\`)
	page.MustElement(\`.ed-modal [data-ed-action="stop"]\`).MustClick()
	s.wait(fmt.Sprintf(\`()=>{const b=document.querySelector('.ed-runview-banner');return edFixture.state.runs.get('%s').run.status==='cancelled' && b.querySelector('[data-ed-cmd="stop-viewed"]').hidden && b.textContent.includes(edFixture.editor().t('easydrag.status.cancelled')) && document.activeElement===b.querySelector('[data-ed-cmd="exit-run-view"]')}\`, second))
	s.shot("run-view-stopped")`, `	s.wait(\`()=>document.querySelectorAll('.ed-drawer .ed-run-row').length>=2\`)
	// A row has no Stop of its own, and with no run in focus no Stop shows at all.
	s.wait(\`()=>!document.querySelector('[data-ed-run-stop], .ed-run-item')\`)
	s.wantStops(0, "runs drawer open, no run in focus")
	s.shot("runs-drawer-running")
	s.noRawKeys("runs drawer running")
	// A click on a running row opens its run; the header's one Stop then stops it, after asking. The row is found and
	// clicked in one step: the drawer is redrawn when a run ends, and a found row can be detached before a separate click.
	page.MustEval(fmt.Sprintf(\`()=>document.querySelector('.ed-run-row[data-ed-run="%s"]').click()\`, first))
	s.wait(fmt.Sprintf(\`()=>{const ed=edFixture.editor();return !document.querySelector('.ed-runview-banner').hidden && !!ed.runView && ed.runView.run.id==='%s'}\`, first))
	s.settle()
	s.waitStop()
	s.wantStops(1, "a running run open in the review, runs drawer open")
	s.shot("run-view-stop")
	s.clickStop()
	s.wait(\`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')\`)
	s.shot("run-stop-confirm")
	s.noRawKeys("run stop confirm")
	page.MustElement(\`.ed-modal [data-ed-action="stop"]\`).MustClick()
	s.wait(fmt.Sprintf(\`()=>{const b=document.querySelector('.ed-runview-banner');return edFixture.state.runs.get('%s').run.status==='cancelled' && !document.querySelector('.ed-modal-backdrop') && b.textContent.includes(edFixture.editor().t('easydrag.status.cancelled')) && document.activeElement===b.querySelector('[data-ed-cmd="exit-run-view"]')}\`, first))
	s.wantStops(0, "the stopped run in the review")
	// The other waiting run, opened from its row while the first run's review is still open: the banner has no Stop,
	// the header's Stop stops it, and the banner then shows it stopped.
	page.MustEval(fmt.Sprintf(\`()=>document.querySelector('.ed-run-row[data-ed-run="%s"]').click()\`, second))
	s.wait(fmt.Sprintf(\`()=>{const ed=edFixture.editor();return !!ed.runView && ed.runView.run.id==='%s'}\`, second))
	s.settle()
	s.waitStop()
	s.wantStops(1, "the second running run open in the review")
	s.shot("run-view-stop-second")
	s.clickStop()
	s.wait(\`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')\`)
	page.MustElement(\`.ed-modal [data-ed-action="stop"]\`).MustClick()
	s.wait(fmt.Sprintf(\`()=>{const b=document.querySelector('.ed-runview-banner');return edFixture.state.runs.get('%s').run.status==='cancelled' && !b.querySelector('[data-ed-cmd="stop-viewed"]') && b.textContent.includes(edFixture.editor().t('easydrag.status.cancelled')) && document.activeElement===b.querySelector('[data-ed-cmd="exit-run-view"]')}\`, second))
	s.wantStops(0, "the second run stopped in the review")
	s.shot("run-view-stopped")`);

// ── ui/desktop_easydrag_durable_browser_test.go: the viewed durable run is stopped from the header ──
const durable = 'ui/desktop_easydrag_durable_browser_test.go';
edit(durable, `	p.MustElement(\`[data-ed-cmd="stop-viewed"]\`).MustClick()`, `	s.clickStop()`, 2);
edit(durable, `!document.querySelector('[data-ed-cmd="stop-viewed"]').hidden`, `!document.querySelector('[data-ed-cmd="cancel"]').hidden`);
edit(durable, `document.querySelector('[data-ed-cmd="stop-viewed"]').hidden`, `document.querySelector('[data-ed-cmd="cancel"]').hidden`);

// ── ui/desktop_easydrag_actions_browser_test.go: the click on Stop waits for a stable button ──
edit('ui/desktop_easydrag_actions_browser_test.go', `	p.MustElement(\`[data-ed-cmd="cancel"]\`).MustClick()`, `	s.clickStop()`);

console.log('c-2 Go test edits applied');
```

```
cd W && node P/c-2-tests.mjs && node P/c-2-go.mjs && gofmt -l ui/
```

Expected: `c-2 test edits applied`, `c-2 Go test edits applied`, and gofmt prints nothing. **The Go edit sits on text B may have changed:** `stopWaitingRuns` opens the runs drawer with `[data-ed-cmd="runs"]` before and after the replaced block; the script does not touch those two lines, so whatever B made of them stays. If an `edit` stops on "found 0", compare with `git show HEAD:ui/desktop_easydrag_browser_test.go` and adjust that one old text.

Run the moved Node tests against the **old** implementation to see them fail for the right reason (the banner and rows still carry Stops):

```
cd W && AURAGO_EASYDRAG_TEST_FILTER="a1008 runs stop" node scripts/test-easydrag.mjs 2>&1 | grep "FAIL\|all EasyDrag"
```

Expected (measured on the scratch copy at the C.1 state):

```
FAIL a1008 runs no row has a Stop; the header Stop shows for every run that has not ended; a test stops immediately and pending; a live stop asks, then a finished run stays quiet — got [3,[true,true,true,false],[true,"true",true,0,1],false,0,[0,false],1,true,1,0] want [0,[true,true,true,false],...]
1 failure(s)
```

The header Stop already works for the opened runs (the rest of the array is equal); only the three row Stops are left. The `extra4` tests fail the same way (`shown` starts with 3 instead of 0).

- [ ] **Step 3: Remove the Stops**

```js
// Task C.2: the banner and the runs drawer lose their Stops; the one Stop is the header's.
// Run from the worktree root: node C:/disposable/easydrag-w1c/patches/c-2.mjs
import { edit } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-editor.js: no stop-viewed in the banner ──
const editor = apps + 'easydrag-editor.js';
edit(editor, `            '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-cmd="stop-viewed" hidden>' + ED.icons.of('stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button>' +
`, ``);
edit(editor, `        // renderBanner names the viewed run and offers Stop while it has not ended (not on a
        // read-only desktop). A Stop that goes away hands its focus to "Back to draft". The text
        // changes only with the run: the banner is a live region, and every header render calls this.`, `        // renderBanner names the viewed run. The banner has no Stop: the header's one Stop stops the
        // viewed run while it can be stopped (easydrag-run-stop.js). The text changes only with the
        // run: the banner is a live region, and every header render calls this.`);
edit(editor, `            const stop = banner.querySelector('[data-ed-cmd="stop-viewed"]');
            const approval`, `            const approval`);
edit(editor, `            // Stop shows on a read-only desktop too (a stop is no write); nothing else does.
            const off = !!ed.run?.durableStatusError || (durableStatus ? !durableStatus.cancel_available : !!(run.durable) || !ED.runs.isActive(run.status));
            if (off && document.activeElement === stop) banner.querySelector('[data-ed-cmd="exit-run-view"]').focus();
            stop.hidden = off;
`, ``);
edit(editor, `            else if (cmd === 'stop-viewed') runs.stopRun(ed.runView && ed.runView.run);
`, ``);

// ── easydrag-runs.js: stopButtons collects only the one button ──
const runs = apps + 'easydrag-runs.js';
edit(runs, `        // stopButtons are every control for this still-active run: drawer, header and run view.
        function stopButtons(id) {
            if (!ed.root || !ed.root.querySelectorAll) return [];
            return Array.from(ed.root.querySelectorAll('[data-ed-run-stop], [data-ed-cmd="cancel"], [data-ed-cmd="stop-viewed"]')).filter(b => {
                if (b.dataset.edRunStop) return b.dataset.edRunStop === String(id);
                if (b.dataset.edCmd === 'cancel') { const focus = ED.runStop.focusRun(ed); return !!focus && focus.id === id; }
                return !!(ed.runView && ed.runView.run && ed.runView.run.id === id);
            });
        }`, `        // stopButtons is the one Stop (the header's) when it belongs to this run: the run in focus.
        function stopButtons(id) {
            if (!ed.root || !ed.root.querySelectorAll) return [];
            const focus = ED.runStop.focusRun(ed);
            return focus && focus.id === id ? Array.from(ed.root.querySelectorAll('[data-ed-cmd="cancel"]')) : [];
        }`);
edit(runs, `and the runs it lists by id (for their Stop buttons).`, `and the runs it lists by id (for showsActive).`);

// ── easydrag-run-stop.js: setStopPending paints the one button ──
const stop = apps + 'easydrag-run-stop.js';
edit(stop, `        // setStopPending shows the stop request on those buttons until it settles.
        function setStopPending(id, on) {
            if (on) rc.pendingStops.add(id); else rc.pendingStops.delete(id);
            stopButtons(id).forEach(b => {
                if (b.dataset.edCmd === 'cancel') { ED.runStop.paint(b, t, !!on, ed); return; }
                b.disabled = !!on;
                if (on) b.setAttribute('aria-busy', 'true');
                else b.removeAttribute('aria-busy');
                b.classList.toggle('is-busy', !!on);
                const span = b.querySelector('span');
                if (!span) return;
                if (on) {
                    if (!b.dataset.edStopLabel) b.dataset.edStopLabel = span.textContent;
                    span.textContent = t('easydrag.ui.run_stopping');
                } else if (b.dataset.edStopLabel) {
                    span.textContent = b.dataset.edStopLabel;
                    delete b.dataset.edStopLabel;
                }
            });
        }`, `        // setStopPending shows the stop request on the one Stop, when it belongs to that run, until it settles.
        // The header paints the same state again when the run in focus changes (ED.runStop.sync).
        function setStopPending(id, on) {
            if (on) rc.pendingStops.add(id); else rc.pendingStops.delete(id);
            stopButtons(id).forEach(b => ED.runStop.paint(b, t, !!on, ed));
        }`);
edit(stop, `// the drawer and the run view (also runs this window did not start), with the polls that follow a`, `// the run view (also runs this window did not start), with the polls that follow a`);
edit(stop, `// run parts that create() in easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.`, `// run parts that create() in easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
// It also owns the one Stop (ED.runStop below): the header's button, for the run in focus.`);
edit(stop, `        // stopRun stops a queued, waiting or running run of the drawer, the header or the run
        // view, also one this window did not start.`, `        // stopRun stops a queued, waiting or running run (the one Stop passes the run in focus), also
        // one this window did not start.`);

// ── easydrag-run-drawer.js: a running row opens its run, like every row ──
const drawer = apps + 'easydrag-run-drawer.js';
edit(drawer, `// EasyDrag run drawer: the list of a flow's runs (filter, search, paging, a Stop per active run,
// compare)`, `// EasyDrag run drawer: the list of a flow's runs (filter, search, paging, compare; a running row opens its
// run, and the header's Stop stops it there)`);
edit(drawer, `        const setStopPending = (...args) => rc.setStopPending(...args);
        const stopRun = (...args) => rc.stopRun(...args);
`, ``);
edit(drawer, `            // A run that has not ended can be stopped from its row, also on a read-only desktop.
            if (!isActive(r.status)) return row;
            return '<div class="ed-run-item">' + row + '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-run-stop="' + esc(r.id) + '" aria-label="' +
                esc(t('easydrag.ui.run_stop_label', { time: core.fmt.dateTime(r.started_at, true) })) + '">' + core.icon('player-stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button></div>';
        }`, `            // A run that has not ended opens like any other: the header's Stop stops it in the review, also on a read-only desktop.
            return row;
        }`);
edit(drawer, `            new Set([...rc.stopping, ...rc.durableStopping]).forEach(id => setStopPending(id, true));
`, ``);
edit(drawer, `            const focusRun = inList ? (active.dataset.edRunStop || active.dataset.edRun || '') : null;
            const onStop = inList && active.dataset.edRunStop !== undefined;
            drawRuns();
            if (focusRun === null) return;
            // The same control when it is still there (the Stop of a run that goes on), else the row.
            const stop = onStop && Array.from(list.querySelectorAll('[data-ed-run-stop]')).find(b => b.dataset.edRunStop === focusRun);
            const target = stop || Array.from(list.querySelectorAll('[data-ed-run]')).find(b => b.dataset.edRun === focusRun) || rc.drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]');`, `            const focusRun = inList ? (active.dataset.edRun || '') : null;
            drawRuns();
            if (focusRun === null) return;
            // The run's row when it is still listed, else the filter.
            const target = Array.from(list.querySelectorAll('[data-ed-run]')).find(b => b.dataset.edRun === focusRun) || rc.drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]');`);
edit(drawer, `                const stop = event.target.closest('[data-ed-run-stop]');
                if (stop) { stopRun(rc.listedRuns.get(stop.dataset.edRunStop)); return; }
`, ``);

// ── CSS: the row wrapper that held the row's Stop is gone ──
edit('ui/css/desktop-app-easydrag.css', `.ed-run-item { display: flex; align-items: center; gap: 4px; }
.ed-run-item > .ed-run-row { flex: 1; min-width: 0; }
.ed-run-item > .ed-btn { flex: none; margin-right: 4px; }
`, ``);

console.log('c-2 JS and CSS edits applied');
```

```
cd W && node P/c-2.mjs
```

Expected: `c-2 JS and CSS edits applied`. It removes: the banner's `stop-viewed` markup, its lines in `renderBanner` and its click branch (`renderBanner` keeps Re-run and Back to draft, and the approval button); the row Stop in `runRow` (a running row is a plain `.ed-run-row` that opens its run, also on a read-only desktop), the click branch, the re-pending of row buttons in `drawRuns`, the "same control" focus logic in `loadRuns` (focus stays on the run's row, else on the filter), the now unused `setStopPending` and `stopRun` constants of the drawer part, and the `.ed-run-item` CSS; `stopButtons` collects only the header button when it belongs to the run in focus; `setStopPending` is three lines.

- [ ] **Step 4: The key file and the manuals**

`easydrag.ui.run_stop_label` ("Stop the run from {time}") named the row Stops; nothing reads it now. The key file deletes it from all 16 locales:

```json
{
  "_delete": ["easydrag.ui.run_stop_label"]
}
```

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/c-2-drawer.json
```

Expected: `plans/2026-11-16-easydrag-w1c/keys/c-2-drawer.json: 0 keys set, 1 deleted, in 16 files of ui/lang/easydrag/`.

The manual chapter 24 (English and German) tells where Stop is (it names the banner and the list). Apply:

```js
// Task C.2 (manuals): the English and German chapter 24 name the one Stop. Run from the worktree root:
//   node C:/disposable/easydrag-w1c/patches/c-2-docs.mjs
import { edit } from '../lib-patch.mjs';

const en = 'documentation/manuals/en/24-easydrag.md';
edit(en, `- **Stop** in the header cancels the test or run you started in this window, without asking (also a run started with *Run now*). Other runs that have not ended are stopped under **Runs** (see [Runs and notifications](#runs-and-notifications)).`, `- **Stop** in the header is the only Stop in the editor. It stops the run in focus: the run you opened in the run view while it has not ended, otherwise the test or run you started in this window (also a run started with *Run now*). A test stops at once; any other run asks first. A run that has not ended is opened under **Runs** (see [Runs and notifications](#runs-and-notifications)) and stopped there with **Stop**.`);
edit(en, `shows **Stop** in the list and in the banner of its run view, also when a trigger or another window started it.`, `opens from the list like any other run; **Stop** in the header stops it there, also when a trigger or another window started it.`);

const de = 'documentation/manuals/de/24-easydrag.md';
edit(de, `- **Stoppen** im Kopf bricht den Test oder Lauf ab, den du in diesem Fenster gestartet hast, ohne Rückfrage (auch einen Lauf, den du mit *Jetzt ausführen* gestartet hast). Andere Läufe, die noch nicht beendet sind, stoppst du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)).`, `- **Stoppen** im Kopf ist das einzige Stoppen im Editor. Es stoppt den Lauf im Fokus: den Lauf, den du in der Laufansicht geöffnet hast, solange er nicht beendet ist, sonst den Test oder Lauf, den du in diesem Fenster gestartet hast (auch einen Lauf mit *Jetzt ausführen*). Ein Test stoppt sofort, jeder andere Lauf fragt vorher nach. Einen Lauf, der noch nicht beendet ist, öffnest du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)) und stoppst ihn dort mit **Stoppen**.`);
edit(de, `zeigt in der Liste und im Banner seiner Ansicht **Stoppen**, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat.`, `öffnet sich aus der Liste wie jeder andere Lauf; **Stoppen** im Kopf stoppt ihn dort, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat.`);

console.log('c-2 manual edits applied');
```

```
cd W && node P/c-2-docs.mjs
```

Expected: `c-2 manual edits applied`. (`cmd/flowsdoc` only generates the trigger and step blocks of these files; T0 runs its drift check.)

- [ ] **Step 5: Prove no old form is left**

```
cd W && git grep -n "stop-viewed\|data-ed-run-stop\|run_stop_label\|ed-run-item\|edStopLabel" -- ui scripts documentation ':!ui/desktop_easydrag_stop_helpers_browser_test.go'
```

Expected: only these lines remain, all of them checks that the old controls are gone: `scripts/test-easydrag-extra4.mjs` (the `[data-ed-run-stop], .ed-run-item` count and `!drawer().querySelector('[data-ed-run-stop]')`), `scripts/test-easydrag-extra6.mjs` (`rowStops`, and the new test's name), `ui/desktop_easydrag_browser_test.go` (the two absence waits in `stopWaitingRuns`). Any other hit is an old form: remove it. (`stopsVisible` in the helper file also names the old hooks on purpose, so a regression under an old hook is counted.)

- [ ] **Step 6: Run the gates**

```
cd W && node scripts/test-easydrag.mjs 2>&1 | tail -3
cd W && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestDesktopEasyDragBrowser|TestDesktopEasyDragDurableBrowser|TestEasyDragActionsAgreeBrowser|TestEasyDragUXBrowser' -count=1 -timeout 30m
```

Expected: `all EasyDrag checks passed` (scratch with B: 1256 ok lines, no FAIL); `PASS render baseline (unchanged)`; vet and gofmt silent; the static Go run `ok`; the editor is 7 lines shorter than before C.2 (the count printed in the baseline minus 7; on the scratch copy 847 to 840) and at most 896; the browser run passes. **Name check:** if `-run` names do not match, list the functions with `git grep -n "^func Test.*EasyDrag" -- ui/*browser_test.go` and use the ones that hold `stopWaitingRuns` (the function is called from the main smoke test), the durable test and the two files above. The browser tests could not be run on the scratch copy (see "What was verified" at the end of the part); this run is the first time they run.

- [ ] **Step 7: Commit**

Write the message to `C:/disposable/easydrag-w1c/c2-msg.txt`:

```
feat(easydrag): the run view and the runs list lose their Stops

The run-view banner keeps "Test again with this data" and "Back to draft";
its Stop is gone. A running row in the runs drawer opens its run like any
other row; the header's one Stop stops it there. stopButtons collects only
that button, setStopPending paints it, and the drawer's row wrapper, the
row Stop's name key (run_stop_label) and its CSS go.

Every test that clicked the banner's or a row's Stop clicks the header
Stop instead (durable, extra5, extra6, the browser tests) or, where the
Node harness has no header, calls stopRun with the run the row names
(extra4). The Go browser tests get one race-safe Stop click and a count of
the visible Stops. The manual chapter 24 (en, de) says where Stop is.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-runs.js ui/js/desktop/apps/easydrag-run-stop.js ui/js/desktop/apps/easydrag-run-drawer.js ui/css/desktop-app-easydrag.css ui/lang/easydrag/*.json documentation/manuals/en/24-easydrag.md documentation/manuals/de/24-easydrag.md scripts/test-easydrag-durable.mjs scripts/test-easydrag-extra4.mjs scripts/test-easydrag-extra5.mjs scripts/test-easydrag-extra6.mjs ui/desktop_easydrag_stop_helpers_browser_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_durable_browser_test.go ui/desktop_easydrag_actions_browser_test.go && git commit -F C:/disposable/easydrag-w1c/c2-msg.txt
```

---

### Task C.3: Browser tests for the one Stop

**Files:**
- Create: `ui/desktop_easydrag_stop_browser_test.go` (four tests: `TestEasyDragOneStopBrowser`, `TestEasyDragStopPositionsBrowser`, `TestEasyDragStopLabelBrowser`, `TestEasyDragStopClicksBrowser`)
- Modify: nothing else (the helpers of C.2 are used; if Step 4 needs a CSS fix it goes into `ui/css/desktop-app-easydrag.css`, which this task then lists in its `git add`)

What the four tests check (spec §5.4):
1. **Exactly one visible Stop** with the own test running (it sits inside `[data-ed-stop-slot]`), with a running run open in the review (the banner has none), and with the runs drawer open while a run is going (no row has one).
2. **Equal positions:** (below 560 px B's header takes two rows, so the comparison is per width: each control against itself at that width, never across widths) the `left` of every header control (`Math.round(x*2)/2`) idle, during a run, while the Stop says "Wird gestoppt…" (forced with `ED.runStop.paint(..., true, ed)`, the widest words the Stop ever shows) and after the run, at 390, 900 and 1440 px. Fonts and two animation frames settle before each measurement. The Stop must lie inside its slot, and no control may appear or disappear (the flow is live and unchanged, so no primary button is expected to change).
3. **Label:** the Stop's `<span>` is not clipped (not `position: absolute`, a real size, inside the button, no overflow, the button inside the viewport) at 320, 760 and 1440 px, for both words.
4. **Real clicks with animations on** (`Motion: true`, reduced motion switched off, as `TestEasyDragActionsMenuClicksBrowser` does): the own test stops with a click; a waiting run open in the review stops through the question, whose dialog closes with an animation.

- [ ] **Step 1: Write the tests**

Write `ui/desktop_easydrag_stop_browser_test.go` with the Write tool:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// openFirstFlow opens the standard fixture's one flow in the editor and waits for its canvas.
func (s *easyDragSmoke) openFirstFlow() {
	s.t.Helper()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
}

// startOwnTest starts a real test of the open flow, each step taking delay milliseconds, and waits until the header
// shows the run (the Stop). The flow has effects, so Test opens in Simulation: the dialog is set to a real test.
func (s *easyDragSmoke) startOwnTest(delay int) {
	s.t.Helper()
	s.page.MustEval(fmt.Sprintf(`()=>{edFixture.state.delay=%d;}`, delay))
	s.page.MustEval(`()=>window.EasyDrag.actions.run('flow.test',{host:edFixture.editor()})`)
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	s.page.MustEval(`()=>{const m=document.querySelector('.ed-modal [data-ed-test-mode]');if(m){m.value='real';m.dispatchEvent(new Event('change',{bubbles:true}));}}`)
	s.page.MustEval(`()=>document.querySelector('.ed-modal [data-ed-action="run"]').click()`)
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && !!edFixture.editor().run && !edFixture.editor().run.view`)
	s.waitStop()
}

// waitNoStop waits until no Stop can be seen: the run ended or was stopped.
func (s *easyDragSmoke) waitNoStop() {
	s.t.Helper()
	s.wait(`()=>{` + easyDragVisibleJS + `return !vis(document.querySelector('[data-ed-cmd="cancel"]'));}`)
}

// showWaitingRun opens a new live run that waits for a slot in the run view and waits for its Stop.
func (s *easyDragSmoke) showWaitingRun() string {
	s.t.Helper()
	id := s.page.MustEval(`()=>edFixture.addWaitingRun()`).Str()
	s.page.MustEval(`id=>Array.from(window.EasyDragApp._instances.values())[0].screen.showRun(id)`, id)
	s.wait(fmt.Sprintf(`()=>{const ed=edFixture.editor();return !document.querySelector('.ed-runview-banner').hidden && !!ed.runView && ed.runView.run.id==='%s'}`, id))
	s.settle()
	s.waitStop()
	return id
}

// leaveRunView ends the run view with its "Back to draft" button.
func (s *easyDragSmoke) leaveRunView() {
	s.t.Helper()
	s.page.MustEval(`()=>document.querySelector('[data-ed-cmd="exit-run-view"]').click()`)
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	s.settle()
}

// TestEasyDragOneStopBrowser: exactly one Stop is visible in the three situations that used to show two or three
// (the own test running, a running run open in the review, the runs drawer open while a run is going), and it is the
// header's, inside its slot.
func TestEasyDragOneStopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	s.wantStops(0, "idle")

	// 1. The own test is running.
	s.startOwnTest(2500)
	s.wantStops(1, "the own test is running")
	if !p.MustEval(`()=>!!document.querySelector('[data-ed-stop-slot] [data-ed-cmd="cancel"]')`).Bool() {
		t.Fatal("the Stop is not inside the Stop slot")
	}
	s.shot("stop-own-test")
	s.clickStop()
	s.waitNoStop()
	s.wantStops(0, "the own test was stopped")

	// 2. A running run is open in the review: the banner has no Stop, the header's one shows.
	s.showWaitingRun()
	s.wantStops(1, "a running run open in the review")
	if p.MustEval(`()=>!!document.querySelector('.ed-runview-banner [data-ed-cmd="stop-viewed"], .ed-runview-banner [data-ed-cmd="cancel"]')`).Bool() {
		t.Fatal("the run view's banner still has a Stop")
	}
	s.shot("stop-run-view")
	s.leaveRunView()
	s.wantStops(0, "back on the draft, nothing running")

	// 3. The runs drawer is open while a run is going: the rows of the running runs have no Stop of their own.
	p.MustEval(`()=>{edFixture.addWaitingRun();edFixture.addWaitingRun();}`)
	s.startOwnTest(2500)
	p.MustEval(`()=>window.EasyDrag.actions.run('view.runs',{host:edFixture.editor()})`)
	s.wait(`()=>document.querySelectorAll('.ed-drawer .ed-run-row').length>=3`)
	s.wait(`()=>!document.querySelector('[data-ed-run-stop], .ed-run-item')`)
	s.wantStops(1, "the runs drawer is open while the own test runs")
	s.shot("stop-drawer-open")
	s.clickStop()
	s.waitNoStop()
	p.MustEval(`()=>window.EasyDrag.actions.run('view.runs',{host:edFixture.editor()})`)
	s.wait(`()=>!document.querySelector('.ed-drawer')`)
	s.failOnPageErrors("one Stop")
}

// headerPositions returns the x position of every header control by name, after the fonts and two animation frames
// have settled. The Stop is listed only while it is visible; its slot always is.
func (s *easyDragSmoke) headerPositions() map[string]float64 {
	s.t.Helper()
	raw := s.page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
		` + easyDragVisibleJS + `
		const head=document.querySelector('.ed-editor > .ed-head')||document.querySelector('.ed-head');
		const out={};
		head.querySelectorAll('button, input, [data-ed-stop-slot]').forEach(el=>{
			const slot=el.hasAttribute('data-ed-stop-slot');
			if(!slot&&!vis(el))return;
			const name=slot?'stop-slot':el.dataset.edCmd||(el.hasAttribute('data-ed-pill')&&'pill')||(el.hasAttribute('data-ed-primary')&&'primary')||(el.dataset.edHistory&&'history-'+el.dataset.edHistory)||(el.hasAttribute('data-ed-name')&&'name')||el.className;
			out[name]=Math.round(el.getBoundingClientRect().left*2)/2;
		});
		return JSON.stringify(out);
	}`).Str()
	var out map[string]float64
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		s.t.Fatalf("header positions: %v (%s)", err, raw)
	}
	return out
}

// sameHeader fails when a control of the header is at another x than idle. The Stop itself is not in the idle list: it
// must lie inside its slot, and the slot must not have moved.
func (s *easyDragSmoke) sameHeader(idle, now map[string]float64, when string) {
	s.t.Helper()
	for name, x := range idle {
		got, ok := now[name]
		if !ok {
			s.t.Errorf("%s: the header control %q is gone", when, name)
			continue
		}
		if got-x > 0.5 || x-got > 0.5 {
			s.t.Errorf("%s: the header control %q moved from x=%v to x=%v", when, name, x, got)
		}
	}
	for name := range now {
		if _, ok := idle[name]; !ok && name != "cancel" {
			s.t.Errorf("%s: the header control %q appeared", when, name)
		}
	}
}

// TestEasyDragStopPositionsBrowser: no header control moves when a run starts, while the Stop says "Stopping…", or
// when the run ends, at 390, 900 and 1440 px. The Stop sits inside its slot.
func TestEasyDragStopPositionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	for _, width := range []int{390, 900, 1440} {
		p.MustSetViewport(width, 900, 1, width < 480)
		s.settle()
		idle := s.headerPositions()
		if _, ok := idle["stop-slot"]; !ok {
			t.Fatalf("%d px: the header has no Stop slot: %v", width, idle)
		}
		s.startOwnTest(500)
		during := s.headerPositions()
		s.sameHeader(idle, during, fmt.Sprintf("%d px, during the run", width))
		if cx, ok := during["cancel"]; !ok || cx < idle["stop-slot"]-0.5 {
			t.Errorf("%d px: the Stop (x=%v) is not inside its slot (x=%v)", width, cx, idle["stop-slot"])
		}
		inside := p.MustEval(`()=>{const b=document.querySelector('[data-ed-cmd="cancel"]').getBoundingClientRect(),s=document.querySelector('[data-ed-stop-slot]').getBoundingClientRect();return b.left>=s.left-0.5&&b.right<=s.right+0.5;}`).Bool()
		if !inside {
			t.Errorf("%d px: the Stop is wider than its slot", width)
		}
		// The words the Stop shows while a stop is out are the longest it ever has: the slot holds them.
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,true,ed);}`)
		s.sameHeader(idle, s.headerPositions(), fmt.Sprintf("%d px, while stopping", width))
		inside = p.MustEval(`()=>{const b=document.querySelector('[data-ed-cmd="cancel"]').getBoundingClientRect(),s=document.querySelector('[data-ed-stop-slot]').getBoundingClientRect();return b.left>=s.left-0.5&&b.right<=s.right+0.5;}`).Bool()
		if !inside {
			t.Errorf("%d px: the Stop is wider than its slot while it says Stopping", width)
		}
		s.shot(fmt.Sprintf("stop-positions-%d", width))
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,false,ed);}`)
		s.waitNoStop()
		s.sameHeader(idle, s.headerPositions(), fmt.Sprintf("%d px, after the run", width))
	}
	s.failOnPageErrors("stop positions")
}

// TestEasyDragStopLabelBrowser: the Stop's words ("Stoppen", and "Wird gestoppt…" while a stop is out) are visible
// and not clipped at 320, 760 and 1440 px.
func TestEasyDragStopLabelBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	for _, width := range []int{320, 760, 1440} {
		p.MustSetViewport(width, 900, 1, width < 480)
		s.settle()
		s.startOwnTest(1500)
		for _, pending := range []bool{false, true} {
			key := "run_cancel"
			if pending {
				key = "run_stopping"
			}
			p.MustEval(`(pending)=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,pending,ed);}`, pending)
			raw := p.MustEval(`async(key)=>{
				await document.fonts.ready;
				await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
				const ed=edFixture.editor(),b=document.querySelector('[data-ed-cmd="cancel"]'),span=b.querySelector('span'),cs=getComputedStyle(span);
				const sr=span.getBoundingClientRect(),br=b.getBoundingClientRect();
				return JSON.stringify({text:span.textContent.trim(),want:ed.t('easydrag.ui.'+key),w:sr.width,h:sr.height,
					inside:sr.left>=br.left-0.5&&sr.right<=br.right+0.5&&sr.top>=br.top-0.5&&sr.bottom<=br.bottom+0.5,
					inViewport:br.left>=0&&br.right<=innerWidth+0.5,overflow:span.scrollWidth>span.clientWidth+1,display:cs.display,clip:cs.clip,position:cs.position});
			}`, key).Str()
			var got struct {
				Text, Want, Display, Clip, Position string
				W, H                                float64
				Inside, InViewport, Overflow        bool
			}
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("%d px: %v (%s)", width, err, raw)
			}
			if got.Text != got.Want || got.W < 8 || got.H < 8 || !got.Inside || !got.InViewport || got.Overflow || got.Display == "none" || got.Position == "absolute" || (got.Clip != "auto" && got.Clip != "") {
				t.Errorf("%d px, %s: the Stop's words are hidden or clipped: %s", width, key, raw)
			}
		}
		s.shot(fmt.Sprintf("stop-label-%d", width))
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,false,ed);}`)
		s.clickStop()
		s.waitNoStop()
	}
	s.failOnPageErrors("stop label")
}

// TestEasyDragStopClicksBrowser clicks the real Stop with desktop animations on: the own test stops at once, and a
// running run open in the review stops after the question, whose dialog closes with an animation.
func TestEasyDragStopClicksBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page
	// The fixture shell sets windows.animations to false; make the desktop animate so the closing class really appears.
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)
	s.openFirstFlow()

	s.startOwnTest(2500)
	s.clickStop()
	s.waitNoStop()
	s.wait(`()=>{const r=edFixture.editor().run;return !!r && r.status==='cancelled'}`)

	id := s.showWaitingRun()
	s.clickStop()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	p.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(fmt.Sprintf(`()=>edFixture.state.runs.get('%s').run.status==='cancelled' && !document.querySelector('.ed-modal-backdrop')`, id))
	s.waitNoStop()
	s.wantStops(0, "the waiting run was stopped")
	s.failOnPageErrors("stop clicks")
}
```

```
cd W && gofmt -l ui/ && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
```

Expected: no output.

- [ ] **Step 2: Run them**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragOneStopBrowser|TestEasyDragStopPositionsBrowser|TestEasyDragStopLabelBrowser|TestEasyDragStopClicksBrowser' -count=1 -v -timeout 15m 2>&1 | grep -E "^(--- |=== RUN|ok|FAIL|PASS)|stop_browser_test"
```

Expected: four `--- PASS` lines and `ok  .../ui`; screenshots `stop-own-test.png`, `stop-run-view.png`, `stop-drawer-open.png`, `stop-positions-{390,900,1440}.png`, `stop-label-{320,760,1440}.png` appear in `D:/tmp/easydrag-w1c-shots`. Look at the three `stop-positions` and `stop-label-320` shots: the Stop is in its slot, nothing overlaps.

- [ ] **Step 3: Red check of the first test**

The count test must fail on the state before C.2 (the banner Stop and the header Stop are visible together). Check out the state of the first Part C commit and run it:

```
cd W && git checkout HEAD~1 -- ui/js ui/css ui/lang && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragOneStopBrowser' -count=1 2>&1 | grep -E "Stop controls|FAIL|ok" ; git checkout HEAD -- ui/js ui/css ui/lang && git status --short
```

Expected: `a running run open in the review: 2 Stop controls are visible, want 1` and `FAIL`; the last `git status --short` prints nothing (the working tree is back at C.2). The positions and label tests have no red check: they need the Stop and the slot, which exist only after B and C.1; they would fail on a header without the slot (`the header has no Stop slot`).

- [ ] **Step 4: If a test fails (what to do)**

- **`the header has no Stop slot` or a control "moved":** the slot's reserved width is B's rule with C.1's `--ed-stop-slot-w` (10.5rem). Measure with the saved screenshot; the slot must hold both words ("Stoppen" and "Wird gestoppt…"). Fix it in B's CSS rule for `[data-ed-stop-slot]` (a `min-width` in `ch` or a measured width), not by hiding the label.
- **The label test fails only at 320 px (`overflow` or `inViewport`):** the right-hand group wraps below the name at that width (the header is `flex-wrap: wrap`), and the Stop slot is wider than the room. Allow the group to shrink its other controls (labels hide, icons stay) before the slot: in `ui/css/desktop-app-easydrag.css` inside `@container (max-width: 560px)` add `.ed-head-actions [data-ed-stop-slot] { flex: none; }` and, if the slot is wider than the container, `max-width: 100%`. The spec keeps the label visible at every width; do not hide it.
- **`element has no visible shape` on a click:** `clickStop` retries three times; if it still fails the Stop is repainted by a render loop: print `edFixture.editor().run.status` and the header's `hidden` state before the click.
- **A measured x differs by 0.5 px:** the tolerance is 0.5; a difference of 1 px or more is real (a changed pill, a scrollbar). Print `during` with `t.Logf` and compare.

- [ ] **Step 5: Run the budget test and the neighbours**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragControlBudgetsBrowser|TestEasyDragActionsAgreeBrowser|TestEasyDragActionsMenuClicksBrowser' -count=1 -timeout 15m 2>&1 | tail -3
```

Expected: `ok`. The header budget (enforced at 8 by B) counts the controls idle, where the Stop is invisible; if it counts the slot's hidden button, B's `countControls` selector is the thing to fix, not this part.

- [ ] **Step 6: Commit**

Write the message to `C:/disposable/easydrag-w1c/c3-msg.txt`:

```
test(easydrag): browser tests for the one Stop

Exactly one visible Stop with the own test running, with a running run
open in the review and with the runs drawer open while a run goes; the x
position of every header control idle, during a run, while the Stop says
"Stopping" and after the run at 390, 900 and 1440 px; the Stop's words not
clipped at 320, 760 and 1440 px; and one test with desktop animations on
that clicks the real Stop (the own test, and a waiting run through its
question).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/desktop_easydrag_stop_browser_test.go && git commit -F C:/disposable/easydrag-w1c/c3-msg.txt
```

(Add `ui/css/desktop-app-easydrag.css` to the `git add` only if Step 4 changed it.)

---

### Part C wrap-up: the full check before the review

```
cd W && npm run test:easydrag:t0
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m
```

Expected: T0 without a FAIL line; the full EasyDrag browser run passes (about 10-17 minutes, the lab with zero findings is part of it). The review of B+C includes a check with desktop animations on (spec §7): `TestEasyDragStopClicksBrowser` is C's part of it; click the Stop by hand with animations on, once with a test and once from a run's review, and look at where the keyboard focus lands (the "Back to draft" button in the review, the canvas otherwise).

## Coverage table (spec item → task)

| Spec item | Task |
|---|---|
| §5.2 `easydrag-run-stop.js` owns the one Stop, keeps `data-ed-cmd="cancel"`, slot | C.1 (`ED.runStop`, header wiring; the slot is B's) |
| §5.2 the run in focus: viewed run while active (durable included), else the own run | C.1 `focusRun`, tests in `test-easydrag-stop.mjs` and `extra6` |
| §5.2 label always visible at every width, "Stoppen" / "Wird gestoppt…", `aria-busy` | C.1 (`paint`, CSS, key file), C.3 label test |
| §5.2 confirmation for live and durable runs, test stops at once | unchanged `stopRun`; moved tests in C.2 (`extra6`, `extra4`, durable) keep it |
| §5.2 `flow.stop` registry covers both cases, reason `reason_stop_unavailable` | C.1 (`paint` repaints the registry state; the check is unchanged: see Deviations) |
| §5.3 the banner's `stop-viewed` goes, Re-run and Back to draft stay | C.2 |
| §5.3 the drawer's row Stops go, a running row opens its run | C.2 |
| §5.3 `stopButtons` collects only the one button | C.2 |
| §5.3 tests that clicked the removed Stops move, none weakened | C.2 (table at the head of the task) |
| §5.4 exactly one visible Stop in three situations | C.3 `TestEasyDragOneStopBrowser` |
| §5.4 equal x positions idle, during, after at 390, 900, 1440 | C.3 `TestEasyDragStopPositionsBrowser` |
| §5.4 label visible and not clipped at 320, 760, 1440 | C.3 `TestEasyDragStopLabelBrowser` |
| §7 one check with `Motion: true` clicking the real Stop | C.3 `TestEasyDragStopClicksBrowser` |
| §9 "exactly one Stop per window in the three situations", "no header control moves" | C.3 |
| Line rule: editor at most 896 | C.2 removes lines (-7 on main); measured in each gate |

## Deviations from the spec

1. **The parked run is a third case of "the run in focus".** The spec says "the viewed run when still active, otherwise the editor's own running run". While a past run is open, `ed.run` is that past run and the own run is parked, so "the editor's own running run" is `ed.parkedRun`. `applyRunView` now stores the parked run's record (`rc.parked.record`) so that the confirmation can name its start time and a durable run is recognised. Without it the Stop would be hidden during a review while a test still runs, and the header would show no Stop for a run that is going.
2. **`flow.stop`'s `check` is unchanged** (checkpoint read failed, or the button is busy). The spec says its reason "stays `reason_stop_unavailable`"; adding "nothing to stop" as a reason made the idle, hidden button carry a reason text in its title (the U1 test caught it) and changed no behaviour: `runs.cancel()` already does nothing without a run in focus.
3. **Two tasks in the spec's order, C.2 splits tests from the helper.** The helper file `ui/desktop_easydrag_stop_helpers_browser_test.go` (new, C.2) holds `stopsVisible`, `waitStop`, `clickStop` and `wantStops` because C.2's moved tests need them before C.3's tests exist; C.3's file keeps only tests and test-local helpers.
4. **`extra4` tests call `stopRun` with the run a row names** instead of clicking the header, because that harness has no editor header. The header path (focus rule, click, pending state, confirmation, durable) is covered by `extra6`, durable, `extra5`, the new Node test and the browser tests; this is a smaller change than building a header into the harness and keeps all case counts. Two of its assertions (the Stop's name with the start time, focus on the row Stop) belonged to the removed button and are replaced as listed in the C.2 table.
5. **The focus after a Stop goes away** moves to "Back to draft" (review) or the canvas. The banner's Stop did the first; the spec does not say. Without it, a keyboard user who pressed the header Stop in a review lands on the page body.
6. **The manual chapter 24 is edited** (en, de): it named the banner and the list as places of Stop. No AGENTS.md line was written: the files are modified by other sessions in the main checkout; the review can add one line (`ED.runStop`) to `ui/js/desktop/apps/AGENTS.md`.
7. **`easydrag.ui.run_stopping` changes in German only** ("Stoppt…" → "Wird gestoppt…", the spec's words). The key is also used by the durable status text; the same words read well there.
8. **The slot width is set in C.1:** `10.5rem` instead of B's guess `9.5rem`, reasoned from the font size and the longest of the 16 locales' words, not measured per locale (Step 4 of C.1 gives the figures).

## Decisions needed

1. **Is `extra4`'s `stopRow` (stopRun with the row's run, no header in the harness) acceptable as "moved to the header Stop"?** Recommendation: yes (Deviation 4). The alternative, mounting a header in the harness, would be a new test fixture of about 40 lines for no new coverage.
2. **Focus hand-over (Deviation 5).** Recommendation: keep it. If the owner prefers the old behaviour (focus lost), drop the `if (!focus && hadFocus)` block in `sync` and the focus assertions in the Node test and `stopWaitingRuns`.
3. **Stop for the parked run (Deviation 1).** Recommendation: keep. The alternative (no Stop in a review that is not about an active run) would hide the only control that can stop the test the user started.
4. **A terminal own durable run with a checkpoint cleanup** (final status, `cancel_available`) could be stopped through the `flow.stop` shortcut or menu before; now only through its review (as the banner did). `cancel()` reaches only the run in focus, and the own run is in focus only while active. Recommendation: accept; the review shows the cleanup, the header's idle state stays quiet.

## What was verified and what was not

**Verified on a scratch copy of main d87930dfc with Part B applied** (`C:/disposable/easydrag-w1c-plan/scratch-bc`: B.1 to B.6 replayed with B's own edit groups and patch scripts, `b1-fixture.mjs` skipped, committed as "B applied"; then this part's scripts in the order printed here):
- All anchors matched on B's code: `c-1-tests`, `c-1`, `c-1-header`, `c-2`, `c-2-tests`, `c-2-go` (including `stopWaitingRuns`, whose drawer-opening line is B's `s.pillEntry("runs")`, untouched), `c-2-docs`, both key files. B's U1 and budget test needed no change; B's stylesheet test needed the slot width.
- `node scripts/test-easydrag.mjs` on the B plus C.1 plus C.2 state: `all EasyDrag checks passed`, 1256 `ok` lines, no FAIL (B's own tests included); `node scripts/test-easydrag-render-baseline.mjs`: `PASS render baseline (unchanged)`; the new Node test prints `ok   the one Stop: focus rules, words, pending, focus hand-over`.
- The failing outputs quoted in C.1 Step 2 and C.2 Step 2 were produced on that copy.
- `gofmt -l` is silent for every Go file of B and C in the copy; `go vet` passes for this part's two Go test files in a stub package against rod v0.116.2 (the copy holds only some of the `ui` Go files, so the whole package cannot be vetted there; the identifiers they define occur nowhere else in `ui`).
- Editor line count: 847 after B, 840 after C.

**Not verified:** every browser test (the moved ones and the four new ones) has not run: they need the real repository and Chrome; the positions, the slot width (10.5rem is an estimate) and the label at 320 px are decided there (C.3 Step 4 says what to do for each failure). The static Go tests of `ui` (`TestEasyDragSplitSeams`, key and plural lints, `TestShippedWebAssetsLoadNothingExternal`) were not run: the repository is too large to copy. C.1 and C.2 add no module, no asset and no count key; the key files change one German text and delete one key that no JS reads any more.
