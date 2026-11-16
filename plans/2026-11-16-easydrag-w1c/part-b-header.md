## Part B: The header (pill, popover, primary button, Stop slot)

**Goal.** The editor header gets one pill that says where the flow stands, a popover behind it that holds what left the header (Active, the next run, Run now, Runs, the action that finishes a partial publish), one primary button for the next step, and a fixed-width slot for the Stop that Part C turns into the one Stop. The header budget is enforced at 8 controls. Spec: `plans/2026-11-16-easydrag-w1c-spec.md` §4 (decisions 1 to 3 of §1 apply).

**Shell and worktree.** Git Bash. Every command starts with `cd <worktree> && ...`. `W` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-header`, branch `feat/easydrag-w1c-header`, cut from main. Part C follows in the same worktree, after B.6. Go commands carry `GOTOOLCHAIN=go1.27.4`; `GOTMPDIR='D:\tmp\go-build'` for `./ui`. Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots`, `GOTMPDIR='D:\tmp\go-build'` and the Bash sandbox off.

**Depends on.** W1a and W1b are on main. Part B writes against Part A's API contract (below) and does not wait for it: B.1 adds the fixture fields itself when they are not on main yet. Part A lands first; if its fixture task lands after B.1, whoever lands second merges the two fixture edits (same names; see Deviations).

**The contract B reads** (top level of the answer of `GET /api/desktop/flows/{id}`, beside `enabled`, and of the save answer beside `draft_revision`): `next_run` `{at, node}` (GET only), `tested` (boolean), `changes` `{steps, connections}` (only for a flow with a live version). Fixture: `edFixture.state.tested`, `.changes`, `.nextRun`.

**How the edits are made.** The worktree has CRLF line endings, the repository LF, and the Write and Edit tools mangle some escapes. Edits to existing files are therefore Node patch scripts (`import { edit, replaceBlock } from '../lib-patch.mjs'`; the helper is `C:/disposable/easydrag-w1c/lib-patch.mjs`, the scripts live in `C:/disposable/easydrag-w1c/patches/`) or edit groups for `C:/disposable/easydrag-w1c/apply-plan-edits.mjs`. Both keep each file's line endings and fail when an old text is not found exactly once (`replaceBlock` also when the block has another line count). A script is run once: if it fails half-way, `git checkout -- <the files it names>`, fix the cause and run it again. New files are written with the Write tool. Run scripts from the worktree root: `cd W && node C:/disposable/easydrag-w1c/patches/<name>.mjs`.

**IMPACT** (the index is stale: it knows `renderHeader` as three symbols, one of them in a file that no longer has it; confirm with `grep -rn`). Run before the tasks that change the function; B did it for these on 2026-11-16:

| Function | Result | Callers (confirmed by grep) |
|---|---|---|
| `refreshRecord` (editor) | LOW, 4 impacted | the flows-changed handler, `repairMission`, `ed.bus` handlers |
| `headerNarrow` (editor) | LOW, 3 impacted | `moreMenu` only |
| `paintActions` (header) | 4 candidates, max LOW (3) | `renderHeader`, the editor's overlay observer |
| `renderHeader` (header) | ambiguous (3 symbols), UNKNOWN | called by the editor's bus handlers and by the saver's `onSaved`; `grep -n "renderHeader" ui/js/desktop/apps/*.js` shows nothing else |

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

**Every commit body ends with** `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Write the message file with the Write tool into `C:/disposable/easydrag-w1c/`, commit with `git commit -F`. Never amend, rebase or push inside a task.

**Escapes.** No code or key file here contains a backslash-u sequence; the dash is `String.fromCodePoint(0x2013)`.

**Gates before each commit** (JS or CSS): `node scripts/test-easydrag.mjs` ends with `all EasyDrag checks passed`; `node scripts/test-easydrag-render-baseline.mjs` ends with `PASS render baseline (unchanged)` (watch its wall time: 15 to 25 s). Go: `gofmt -l ui/` lists none of the files of the task, `go vet ./ui` is silent, and `GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short` ends with `ok`.

### The design in one page

- **Facts** (`easydrag-flow-state.js`, `ED.flowState`): `take(ed, answer, {get, revision})` copies `tested`, `changes` and (for a GET) `next_run` onto `ed.flow`; `isTested(ed)` is true when the flag is for the exact revision on screen and nothing is unsaved, or when the editor's own `ed.testEvidence` is a success for that revision; `changeCount(ed)` is steps plus connections; `refresh(ed)` reads the flow again (the popover does that when it opens, so the next run is fresh).
- **Pill** (`easydrag-pill.js`, `ED.pill`): `describe` picks one state in a fixed order: a run (own or viewed, disc running), a past run (no disc), a partial publish (blocked), a missing Mission Control link (blocked), a live version that fails the checks (blocked), stopped timers (uncertain), paused (paused), live with a changed draft, live, never published (not run). Texts are the status words and `pill_*` keys; the version reads `vN`.
- **Popover**: `ED.kit.popover(anchor, {content, label, onClose})` (kit) holds the entries `[data-ed-pop]` (`active`, `run-now`, `runs`, `publish-finish`, `repair`); each runs through `ED.actions.run`, so refused entries show the registry's reason.
- **Primary**: `ED.pill.primaryOf(ed)`: none for a clean live flow, a read-only desktop and a past run; Publish for a partial publish; Test while the draft is not tested; Publish (with the count, plural) once it is. One filled button at most: it carries `data-ed-primary`.
- **Row**: Back, name, pill on the left; Stop slot, Test, Publish, Undo and Redo, More on the right.

---

### Task B.0: Preamble (run once, before B.1)

- [ ] **Step 0.1: verify main and Part A**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git log --oneline -5 && git status --short | grep -i easydrag
for f in editor header kit format saver; do printf "%s: " $f; wc -l < ui/js/desktop/apps/easydrag-$f.js; done
grep -n "tested\|nextRun\|next_run" ui/testdata/easydrag-fixture.js | head
ls ui/js/desktop/apps/easydrag-flow-state.js ui/js/desktop/apps/easydrag-pill.js scripts/test-easydrag-header.mjs 2>&1 | head -3
```
Expected: main at d87930dfc or later; no EasyDrag file modified in the main checkout; line counts editor 859, header 200, kit 259, format 174, saver 390 or more; the `grep` finds nothing when Part A has not landed (B.1 then adds the fixture fields); the last command reports "No such file" three times. For each outcome:
  - The fixture already has `tested`, `nextRun` and `changes`: Part A landed. Open the fixture and check that GET answers and save answers carry the fields at the top level of the answer (beside `enabled` and `draft_revision`). If they sit elsewhere, change the three reads in `take` (B.1) and say so in the commit body. Skip `b1-fixture.mjs` in B.1.
  - A file of the last command exists: another session started this part. Stop and tell the coordinator.

- [ ] **Step 0.2: create the worktree and the scratch directories**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1c-header -b feat/easydrag-w1c-header main && cd .worktrees/easydrag-w1c-header && git branch --show-current && git log --oneline -1
mkdir -p C:/disposable/easydrag-w1c/patches D:/tmp/go-build D:/tmp/easydrag-w1c-shots
ls plans/2026-11-16-easydrag-w1c/keys/
```
Expected: `feat/easydrag-w1c-header` and main's head commit; the last command lists `b-pill.json  b-primary.json` (the coordinator commits the key files to main before this part starts; if one is missing, stop and ask; do not write it by hand). The helper `C:/disposable/easydrag-w1c/lib-patch.mjs` and `apply-plan-edits.mjs` must exist (`ls C:/disposable/easydrag-w1c/`).

- [ ] **Step 0.3: baseline**

```
cd W && npm run test:easydrag:t0
```
Expected: no FAIL line (about 90 to 200 s). Note the time. Line counts after Part B (the line rule: editor at most 896): editor 847, header 157, pill 193, flow-state 85, kit 319, format 185; `interact`, `home`, `dialogs` and `canvas` are untouched (753, 888, 281, 925).

---

### Task B.1: The editor keeps `next_run`, `tested` and `changes`

**Files:**
- Create: `ui/js/desktop/apps/easydrag-flow-state.js`, `scripts/test-easydrag-header.mjs`
- Modify: `ui/js/desktop/apps/easydrag-saver.js:232` (the save answer reaches `onSaved`), `ui/js/desktop/apps/easydrag-editor.js` (bind after `ed`, `onSaved`, `refreshRecord`), `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go:11`, `scripts/test-easydrag-extra3.mjs` (module list, `openEditor` takes `facts`), `scripts/test-easydrag.mjs` (runs the new checks), `ui/testdata/easydrag-fixture.js` (only without Part A)
- Test: `scripts/test-easydrag-header.mjs`

- [ ] **Step 1: write the module**

Write `ui/js/desktop/apps/easydrag-flow-state.js`:

```js
// EasyDrag flow facts from the server (W1c): when the flow runs next (ed.flow.next_run), whether its
// exact draft revision passed a test or a simulation (ed.flow.tested) and how much the draft differs
// from the live version (ed.flow.changes). The editor takes them from GET /flows/{id} and from every
// save answer; the header's pill and primary button read them through the helpers below.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const count = value => (Number.isFinite(Number(value)) && Number(value) > 0 ? Math.floor(Number(value)) : 0);

    // take copies the facts of a server answer onto ed.flow. An answer is GET /flows/{id} ({flow, enabled, next_run?, tested?,
    // changes?}) or a save answer ({draft_revision, tested?, changes?}). opts.revision is the draft revision the answer describes
    // (the flag "tested" is true for that revision only); opts.get says that the answer is a complete read, so a missing next_run
    // means "none". A save answer says nothing about the schedule: next_run stays as it was.
    function take(ed, answer, opts) {
        const o = opts || {};
        const flow = ed.flow;
        if (!flow || !answer || typeof answer !== 'object') return;
        flow.tested = answer.tested === true;
        const changes = answer.changes;
        if (changes && typeof changes === 'object') flow.changes = { steps: count(changes.steps), connections: count(changes.connections) };
        else delete flow.changes;
        if (o.get || Object.prototype.hasOwnProperty.call(answer, 'next_run')) {
            const next = answer.next_run;
            if (next && typeof next.at === 'string' && Number.isFinite(Date.parse(next.at))) flow.next_run = { at: next.at, node: typeof next.node === 'string' ? next.node : '' };
            else delete flow.next_run;
        }
        ed.flowFactsRevision = Number.isInteger(o.revision) ? o.revision : count(flow.draft_revision);
    }

    // currentRevision is the draft revision the editor shows: the saver's, else the record's.
    function currentRevision(ed) {
        return ed.saver && Number.isInteger(ed.saver.revision) ? ed.saver.revision : count(ed.flow && ed.flow.draft_revision);
    }

    // dirty: the draft has an edit the server has not seen, so nothing it says about "tested" applies.
    function dirty(ed) { return !!(ed.saver && typeof ed.saver.isDirty === 'function' && ed.saver.isDirty()); }

    // isTested: the exact draft revision on screen passed a test or a simulation. The server's flag counts for the revision it
    // was read for; the editor's own evidence (ed.testEvidence, set when a test or simulation of this session ends) counts at
    // once, before the next save or read brings the flag. A failed run is no evidence.
    function isTested(ed) {
        if (!ed.flow || dirty(ed)) return false;
        const revision = currentRevision(ed);
        if (ed.flow.tested === true && ed.flowFactsRevision === revision) return true;
        const evidence = ed.testEvidence;
        return !!evidence && Number(evidence.revision) === revision && ED.status.of(evidence.status, 'run') === 'ok';
    }

    // changeCount is the number of changes since the live version (steps and connections together), or null when the server
    // sent none (a flow that was never published, an older server).
    function changeCount(ed) {
        const changes = ed.flow && ed.flow.changes;
        return changes ? count(changes.steps) + count(changes.connections) : null;
    }

    // refresh reads the flow again and takes its facts: the popover asks for the next run when it opens. A read that answers for
    // another revision than the one on screen, or after the editor closed, is dropped. It resolves true when the facts changed hands.
    async function refresh(ed) {
        if (!ed.flow || !ed.api || typeof ed.api.get !== 'function') return false;
        let res;
        try { res = await ed.api.get(ed.flow.id); } catch (err) { return false; }
        if (!res || !res.flow || ed.root.isConnected === false || ed.runView) return false;
        const revision = count(res.flow.draft_revision);
        if (revision !== currentRevision(ed)) return false;
        take(ed, res, { get: true, revision });
        ed.bus.emit('flow-facts', ed.flow);
        return true;
    }

    // bind keeps the facts honest between answers: a restored version replaces the draft, so the old flag and count no longer apply.
    // loaded is the first GET answer.
    function bind(ed, bag, loaded) {
        take(ed, loaded, { get: true, revision: count(loaded && loaded.flow && loaded.flow.draft_revision) });
        bag.add(ed.bus.on('restored', res => {
            if (!res || !res.flow) return;
            ed.flow.tested = false;
            delete ed.flow.changes;
            ed.flowFactsRevision = count(res.flow.draft_revision);
        }));
    }

    ED.flowState = { take, bind, refresh, isTested, changeCount, dirty };
})();
```

- [ ] **Step 2: write the patch scripts**

Write `C:/disposable/easydrag-w1c/patches/b1-facts.mjs`:

```js
// B.1: the editor keeps next_run, tested and changes on ed.flow. Run from the worktree root.
import { edit } from '../lib-patch.mjs';

const SAVER = 'ui/js/desktop/apps/easydrag-saver.js';
const EDITOR = 'ui/js/desktop/apps/easydrag-editor.js';

// The saver hands the whole save answer to onSaved.
edit(SAVER, `            notify(o.onSaved, { revision, issues: res.issues || [] });`,
`            notify(o.onSaved, { revision, issues: res.issues || [], answer: res });`);
edit(SAVER, `    //   onSaved({revision, issues})`, `    //   onSaved({revision, issues, answer})   answer is the whole save answer (tested, changes ...)`);

// The editor binds the facts once, takes them from every save answer and from every re-read of the record.
edit(EDITOR, `        bag.add(() => { if (ed.packExportControl) ed.packExportControl.abort(); });
`, `        bag.add(() => { if (ed.packExportControl) ed.packExportControl.abort(); });
        ED.flowState.bind(ed, bag, loaded);
`);
edit(EDITOR, `            onSaved: ({ revision }) => { ed.flow.draft_revision = revision; contentDirty = ed.saver.isDirty(); renderHeader(); },`,
`            onSaved: ({ revision, answer }) => { ed.flow.draft_revision = revision; ED.flowState.take(ed, answer, { revision }); contentDirty = ed.saver.isDirty(); renderHeader(); },`);
edit(EDITOR, `            ed.flowEnabled = !!res.enabled;
            renderHeader();`, `            ed.flowEnabled = !!res.enabled;
            ED.flowState.take(ed, res, { get: true, revision: Number(res.flow.draft_revision) });
            renderHeader();`);

// The module loads before the header.
edit('ui/js/desktop/core/module-loader.js', `                '/js/desktop/apps/easydrag-extraction.js',
`, `                '/js/desktop/apps/easydrag-extraction.js',
                '/js/desktop/apps/easydrag-flow-state.js',
`);
edit('ui/desktop_easydrag_test.go', `"easydrag-extraction.js", "easydrag-header.js",`, `"easydrag-extraction.js", "easydrag-flow-state.js", "easydrag-header.js",`);
edit('scripts/test-easydrag-extra3.mjs', `'easydrag-extraction.js', 'easydrag-header.js',`, `'easydrag-extraction.js', 'easydrag-flow-state.js', 'easydrag-header.js',`);
console.log('b1-facts: done');
```

Write `C:/disposable/easydrag-w1c/patches/b1-extra3.mjs` (the c1d07 sandbox hands extra GET fields to the editor it opens) and `C:/disposable/easydrag-w1c/patches/b1-runner.mjs` (the full run calls the new checks):

```js
// B.1: openEditor of the c1d07 sandbox hands extra answer fields (opts.facts) to the editor, as the server's GET would.
import { edit } from '../lib-patch.mjs';

edit('scripts/test-easydrag-extra3.mjs', `        const editor = h.ED.editor.create(app, { flow, enabled: !!o.enabled, issues: [] }, {});`,
`        const editor = h.ED.editor.create(app, Object.assign({ flow, enabled: !!o.enabled, issues: [] }, o.facts), {});`);
edit('scripts/test-easydrag-extra3.mjs', `    // opts: flow (fields over the default record), enabled, readonly (a read-only desktop).`,
`    // opts: flow (fields over the default record), enabled, readonly (a read-only desktop), facts (more fields of the GET answer: next_run, tested, changes).`);
console.log('b1-extra3: done');
```

```js
// B.1: test-easydrag.mjs runs the header checks on the c1d07 sandbox.
import { edit } from '../lib-patch.mjs';

edit('scripts/test-easydrag.mjs', `// ── S0.0: render counters on seeded S/M/XL flows`, `// ── W1c B: the header, its pill and the popover (test-easydrag-header.mjs, on the c1d07 sandbox) ──
await (await import('./test-easydrag-header.mjs')).run({ apps, eq, guardAsync, settle, sandbox: editorHarness.sandbox, openEditor: editorHarness.openEditor });
// ── S0.0: render counters on seeded S/M/XL flows`);
console.log('b1-runner: done');
```

Only when Part A has not landed (Step 0.1), write `C:/disposable/easydrag-w1c/patches/b1-fixture.mjs`:

```js
// B.1 (only when Part A has not landed): the fixture answers tested, changes and next_run from edFixture.state.tested,
// .changes and .nextRun. Part A's fixture task uses the same names; whoever lands second merges. Run from the worktree root.
import { edit } from '../lib-patch.mjs';

const FILE = 'ui/testdata/easydrag-fixture.js';

edit(FILE, `        delay: 140, disabled: false,`, `        tested: false, changes: null, nextRun: null,
        delay: 140, disabled: false,`);
edit(FILE, `    // broadcast sends flows_changed through the desktop's event handler,`,
`    // factsOf are the W1c facts of a flow answer: tested, changes (only for a flow with a live version) and next_run, from state.
    function factsOf(rec, withSchedule) {
        const out = { tested: !!state.tested };
        if (rec.live && state.changes) out.changes = clone(state.changes);
        if (withSchedule && state.nextRun) out.next_run = clone(state.nextRun);
        return out;
    }

    // broadcast sends flows_changed through the desktop's event handler,`);
edit(FILE, `            if (method === 'GET') return reply({ flow: rec, enabled: entry.enabled, issues: validate(rec.draft, 'draft') });`,
`            if (method === 'GET') return reply(Object.assign({ flow: rec, enabled: entry.enabled, issues: validate(rec.draft, 'draft') }, factsOf(rec, true)));`);
edit(FILE, `            return reply({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') });
        }
        if (action === 'publish-preview'`, `            return reply(Object.assign({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') }, factsOf(rec, false)));
        }
        if (action === 'publish-preview'`);
console.log('b1-fixture: done');
```

- [ ] **Step 3: write the failing test**

Write `scripts/test-easydrag-header.mjs` (the skeleton; later tasks add checks at the marker through edit groups):

```js
// W1c Part B checks (the editor header): the facts the editor keeps from the server (B.1), the kit popover (B.2), the pill and
// its popover (B.3, B.4) and the primary button (B.5). They run on the c1d07 sandbox (test-easydrag-extra3.mjs returns sandbox
// and openEditor); test-easydrag.mjs calls run(env) with its helpers and counts the failures. Alone this file only has to import.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';

// flowDoc is a flow start -> alpha.
function flowDoc() {
    return {
        schema: 1, name: 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} }
        ],
        edges: [{ id: 'e1', source: { node: T1, port: 'out' }, target: { node: A, port: 'in' } }]
    };
}

export async function run(env) {
    const { apps, eq, guardAsync, settle, sandbox, openEditor } = env;

    // @@APPEND@@
}
```

Add the B.1 checks (edit group `b1-test`):

<!-- edits:b1-test -->
**Edit** `scripts/test-easydrag-header.mjs`

Old:
```text
    // @@APPEND@@
```

New:
```text
    // ── B.1: the facts of the server ──

    // recordOf is the flow record the stub server answers with: published as v2, the draft at revision `rev`.
    const recordOf = rev => ({ id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: rev, published_draft_revision: 2, live: flowDoc(), live_revision: 2 });

    await guardAsync('w1c B.1 the editor keeps next_run, tested and changes from the read and from every save', async () => {
        let rev = 3;
        const get = { tested: true, changes: { steps: 2, connections: 1 }, next_run: { at: '2026-11-17T07:00:00+01:00', node: T1 } };
        const put = { tested: false, changes: { steps: 3, connections: 1 } };
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return Object.assign({ flow: recordOf(rev), enabled: true, issues: [] }, get);
            if (req.method === 'PUT') { rev = 4; return Object.assign({ draft_revision: 4, issues: [] }, put); }
            return undefined;
        });
        const editor = openEditor(h, { flow: recordOf(3), enabled: true, facts: get });
        await settle();
        const ED = h.ED, flow = () => editor.ed.flow;
        const first = [flow().tested, flow().changes, flow().next_run, ED.flowState.isTested(editor.ed), ED.flowState.changeCount(editor.ed)];
        // An edit makes the draft untested at once (before the save), and the save answer brings the new facts; it says nothing of the schedule.
        editor.ed.model.setFlow({ description: 'changed' });
        const dirtyTested = ED.flowState.isTested(editor.ed);
        h.runTimers(1000);
        await settle();
        const saved = [flow().tested, flow().changes, flow().next_run, ED.flowState.isTested(editor.ed), ED.flowState.changeCount(editor.ed), editor.ed.saver.state];
        // A re-read for revision 4 that says "tested" counts; one for another revision does not.
        get.tested = true; get.changes = { steps: 5, connections: 0 }; delete get.next_run;
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'checked' } });
        await settle();
        const reread = [flow().tested, flow().changes, flow().next_run === undefined, ED.flowState.isTested(editor.ed)];
        rev = 9;
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'checked' } });
        await settle();
        const elsewhere = [ED.flowState.isTested(editor.ed)];
        // A restored version replaces the draft: the old flag and count go.
        editor.ed.bus.emit('restored', { flow: Object.assign(recordOf(10), { draft: flowDoc() }), issues: [] });
        await settle();
        const restored = [flow().tested, flow().changes === undefined, ED.flowState.isTested(editor.ed)];
        eq('w1c B.1 the facts follow the read, the save, a re-read and a restore', [first, dirtyTested, saved, reread, elsewhere, restored, h.logged], [
            [true, { steps: 2, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, true, 3],
            false,
            [false, { steps: 3, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, false, 4, 'saved'],
            [true, { steps: 5, connections: 0 }, true, true],
            [false],
            [false, true, false],
            []
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.1 the editor\'s own test evidence counts for the exact revision, and a failed run does not', async () => {
        const ED = sandbox(() => undefined).ED;
        const ed = ok => ({ flow: { tested: false, draft_revision: 5 }, flowFactsRevision: 5, saver: { revision: 5, isDirty: () => false }, testEvidence: { revision: 5, status: ok } });
        const other = Object.assign(ed('success'), { testEvidence: { revision: 4, status: 'success' } });
        const dirty = Object.assign(ed('success'), { saver: { revision: 5, isDirty: () => true } });
        const staleFlag = { flow: { tested: true, draft_revision: 5 }, flowFactsRevision: 4, saver: { revision: 5, isDirty: () => false } };
        eq('w1c B.1 isTested: success of this revision, a failure, another revision, an unsaved edit, a flag for an older revision',
            [ED.flowState.isTested(ed('success')), ED.flowState.isTested(ed('error')), ED.flowState.isTested(other), ED.flowState.isTested(dirty), ED.flowState.isTested(staleFlag)],
            [true, false, false, false, false]);
        eq('w1c B.1 changeCount adds steps and connections and is null without the server\'s count',
            [ED.flowState.changeCount({ flow: { changes: { steps: 2, connections: 3 } } }), ED.flowState.changeCount({ flow: {} }), ED.flowState.changeCount({ flow: { changes: { steps: 'x' } } })],
            [5, null, 0]);
    });

    // @@APPEND@@
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/patches/b1-extra3.mjs && node C:/disposable/easydrag-w1c/patches/b1-runner.mjs
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b1-test
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B\|^FAIL"
```
Expected: the three `FAIL w1c ... threw` lines (`ED.flowState` is not defined: the module is not loaded yet).

- [ ] **Step 4: wire the module and run the check**

```
cd W && node C:/disposable/easydrag-w1c/patches/b1-facts.mjs
cd W && ls ui/testdata/easydrag-fixture.js && node C:/disposable/easydrag-w1c/patches/b1-fixture.mjs   # only without Part A
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B\|^FAIL\|all EasyDrag"
```
Expected: `b1-facts: done`, `b1-fixture: done`, then `ok   w1c B.1 the facts follow the read, the save, a re-read and a restore`, `ok   w1c B.1 isTested ...`, `ok   w1c B.1 changeCount ...` and `all EasyDrag checks passed`, no `FAIL`. Check what moved: `git diff --stat` lists the saver, the editor (3 lines more), the loader, the Go list, extra3, test-easydrag.mjs, the fixture and the two new files. Then the gates: `node scripts/test-easydrag-render-baseline.mjs`, and the Go static test of the module list:

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
```
Expected: `ok`.

- [ ] **Step 5: commit**

Write `C:/disposable/easydrag-w1c/b1-msg.txt`:

```text
feat(easydrag): the editor keeps next_run, tested and changes

W1c Part B.1. ED.flowState takes the three facts of the server from GET
/flows/{id} and from every save answer (the saver now hands the answer to
onSaved) and from every re-read of the record; isTested counts the server's
flag only for the revision on screen with nothing unsaved, or the editor's own
successful test of that revision; changeCount is steps plus connections. The
fixture answers the three fields from edFixture.state.tested, .changes and
.nextRun (only when Part A had not landed).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-flow-state.js ui/js/desktop/apps/easydrag-saver.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/testdata/easydrag-fixture.js scripts/test-easydrag-extra3.mjs scripts/test-easydrag.mjs scripts/test-easydrag-header.mjs && git commit -F C:/disposable/easydrag-w1c/b1-msg.txt && git log --oneline -1
```

---

### Task B.2: `ED.kit.popover`

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-kit.js` (new function, export)
- Test: `scripts/test-easydrag-header.mjs`

- [ ] **Step 1: add the failing checks** (edit group `b2-test`):

<!-- edits:b2-test -->
**Edit** `scripts/test-easydrag-header.mjs`

Old:
```text
    // @@APPEND@@
```

New:
```text
    // ── B.2: the kit popover ──

    await guardAsync('w1c B.2 ED.kit.popover: focus moves in; Escape, a press outside and a focus change close it; Escape gives the focus back', async () => {
        const h = sandbox(() => undefined);
        const { ED } = h, doc = h.dom.document, El = h.dom.El;
        const anchor = new El('button', {});
        const outside = new El('button', {});
        h.body.appendChild(anchor);
        h.body.appendChild(outside);
        const closed = [];
        const open = () => ED.kit.popover(anchor, {
            content: ED.core.el('<div><button type="button" data-ed-pop="a">A</button><button type="button" data-ed-pop="b">B</button></div>'),
            label: 'Status', onClose: info => closed.push(info.restore)
        });
        let pop = open();
        const opened = [pop.el.getAttribute('role'), pop.el.getAttribute('aria-label'), h.body.contains(pop.el), doc.activeElement === pop.el.querySelector('[data-ed-pop="a"]'), pop.isOpen()];
        const key = pop.el.fire('keydown', { key: 'Escape' });
        const byEscape = [h.body.contains(pop.el), doc.activeElement === anchor, closed.slice(), key.defaultPrevented, key.stopped, pop.isOpen()];
        pop = open();
        h.fireDoc('pointerdown', { target: anchor });
        const onAnchor = [pop.isOpen()];
        h.fireDoc('pointerdown', { target: pop.el.querySelector('[data-ed-pop="b"]') });
        const inside = [pop.isOpen()];
        outside.focus();
        h.fireDoc('pointerdown', { target: outside });
        const byPress = [pop.isOpen(), h.body.contains(pop.el), doc.activeElement === outside, closed.slice()];
        pop = open();
        pop.el.fire('focusout', { relatedTarget: pop.el.querySelector('[data-ed-pop="b"]') });
        const withinFocus = [pop.isOpen()];
        pop.el.fire('focusout', { relatedTarget: outside });
        const byFocus = [pop.isOpen(), closed.slice()];
        pop.close(true);
        eq('w1c B.2 popover', [opened, byEscape, onAnchor, inside, byPress, withinFocus, byFocus, h.docListeners.pointerdown.length, h.logged], [
            ['dialog', 'Status', true, true, true],
            [false, true, [true], true, true, false],
            [true], [true],
            [false, false, true, [true, false]],
            [true],
            [false, [true, false, false]],
            0, []
        ]);
    });

    await guardAsync('w1c B.2 ED.kit.popover.update swaps the content and keeps the focus on the same control', async () => {
        const h = sandbox(() => undefined);
        const { ED } = h, doc = h.dom.document;
        const anchor = new h.dom.El('button', {});
        h.body.appendChild(anchor);
        const pop = ED.kit.popover(anchor, { content: ED.core.el('<div><button type="button" data-ed-pop="a">A</button><button type="button" data-ed-pop="b">B</button></div>') });
        pop.el.querySelector('[data-ed-pop="b"]').focus();
        pop.update(ED.core.el('<div><button type="button" data-ed-pop="a">A2</button><button type="button" data-ed-pop="b">B2</button></div>'));
        eq('w1c B.2 popover update', [doc.activeElement === pop.el.querySelector('[data-ed-pop="b"]'), pop.el.querySelectorAll('button').length, h.body.contains(pop.el)], [true, 2, true]);
        pop.close(false);
    });

    // @@APPEND@@
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b2-test
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.2\|^FAIL"
```
Expected: two `FAIL w1c B.2 ...` lines (`ED.kit.popover is not a function`).

- [ ] **Step 2: implement**

Write `C:/disposable/easydrag-w1c/patches/b2-kit.mjs`:

```js
// B.2: ED.kit.popover. Run from the worktree root: node C:/disposable/easydrag-w1c/patches/b2-kit.mjs
import { edit } from '../lib-patch.mjs';

const FILE = 'ui/js/desktop/apps/easydrag-kit.js';

edit(FILE, `    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash, limit };`,
`    // popover anchors a small dialog panel to a control (W1c): opts {content (an element, or markup), label (the dialog's name),
    // className, onClose({restore})}. It sits under the anchor inside the nearest .ed-editor (else the body), left-aligned to the
    // anchor and kept inside that host. Focus moves to the first button in it (else the panel); Escape closes it and gives the focus
    // back to the anchor; a press outside the panel and the anchor, and focus that moves elsewhere, close it without moving the focus.
    // Returns {el, close(restoreFocus), update(content), isOpen()}. update swaps the content and keeps the focus on the same
    // [data-ed-pop] control. Nothing animates on close: the panel is removed at once.
    function popover(anchor, opts) {
        const o = opts || {};
        const host = (anchor.closest && anchor.closest('.ed-editor')) || document.body;
        const el = ED.core.el('<div class="ed-popover ed-popover--anchored" role="dialog" tabindex="-1"></div>');
        if (o.label) el.setAttribute('aria-label', text(o.label));
        if (o.className) o.className.split(/\\s+/).filter(Boolean).forEach(name => el.classList.add(name));
        let open = true;
        function fill(content) {
            el.innerHTML = '';
            if (content && typeof content === 'object') el.appendChild(content); else el.innerHTML = text(content);
        }
        function place() {
            const h = host.getBoundingClientRect(), a = anchor.getBoundingClientRect();
            const left = Math.max(8, Math.min(a.left - h.left, h.width - 368));
            el.style.left = left + 'px';
            el.style.top = Math.max(8, a.bottom - h.top + 6) + 'px';
        }
        function close(restore) {
            if (!open) return;
            open = false;
            document.removeEventListener('pointerdown', outside, true);
            el.remove();
            if (restore && typeof anchor.focus === 'function') anchor.focus();
            if (typeof o.onClose === 'function') o.onClose({ restore: !!restore });
        }
        function outside(event) {
            if (!el.contains(event.target) && !anchor.contains(event.target)) close(false);
        }
        el.addEventListener('keydown', event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            close(true);
        });
        el.addEventListener('focusout', event => {
            const to = event.relatedTarget;
            if (to && !el.contains(to) && !anchor.contains(to)) close(false);
        });
        function update(content) {
            const was = document.activeElement && el.contains(document.activeElement) ? document.activeElement.getAttribute('data-ed-pop') : null;
            fill(content);
            place();
            const again = was ? el.querySelector('[data-ed-pop="' + was + '"]') : null;
            if (again && typeof again.focus === 'function') again.focus();
        }
        fill(o.content);
        host.appendChild(el);
        place();
        document.addEventListener('pointerdown', outside, true);
        const first = el.querySelector('button');
        (first || el).focus();
        return { el, close, update, isOpen: () => open };
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash, limit, popover };`);
console.log('b2-kit: popover added to ' + FILE);
```

```
cd W && node C:/disposable/easydrag-w1c/patches/b2-kit.mjs && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B\|^FAIL\|all EasyDrag"
```
Expected: `ok   w1c B.2 popover`, `ok   w1c B.2 popover update`, `all EasyDrag checks passed`. The popover returns `{el, close, update, isOpen}`; the contract of the brief is `{el, close}`, the two others are additions of this part. It closes at once on every path (no animation), so nothing waits on a closing class.

- [ ] **Step 3: gates and commit**

```
cd W && node scripts/test-easydrag-render-baseline.mjs && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
```
Write `C:/disposable/easydrag-w1c/b2-msg.txt`:

```text
feat(easydrag): kit popover for anchored panels

W1c Part B.2. ED.kit.popover(anchor, {content, label, onClose}) puts a dialog
panel under its anchor inside the editor; focus moves in, Escape closes it
and gives the focus back, a press outside and a focus change close it, and
update swaps the content and keeps the focus on the same entry.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-kit.js scripts/test-easydrag-header.mjs && git commit -F C:/disposable/easydrag-w1c/b2-msg.txt && git log --oneline -1
```

---

### Task B.3: The pill and its popover replace the chips and three buttons

The pill replaces the state chips and the revision line; its popover is the new home of Active, Run now and Runs. They land in one commit: the popover is the only place the three buttons go to, and the tests that press them move with them. (The brief suggested B.3 for the pill and B.4 for the popover; two commits would leave a commit without Runs and Run now.)

**Files:**
- Create: `ui/js/desktop/apps/easydrag-pill.js`, `scripts/easydrag-header-test-helper.mjs`, `ui/desktop_easydrag_header_helpers_test.go`
- Modify: `ui/js/desktop/apps/easydrag-format.js` (`nextRun`), `easydrag-header.js`, `easydrag-editor.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go`, `ui/css/desktop-app-easydrag.css`, `ui/lang/easydrag/*.json` (9 keys), the tests listed below
- Test: `scripts/test-easydrag-header.mjs`

**Keys** (key file `plans/2026-11-16-easydrag-w1c/keys/b-pill.json`; en and de here, the other 14 locales in the key file; the live word of each locale is the one `revision_live` already uses):

| Key | en | de |
|---|---|---|
| `pill_live` | Live v{n} | Live v{n} |
| `pill_live_changed` | Live v{n} · draft changed | Live v{n} · Entwurf geändert |
| `pill_paused` | Paused · v{n} | Pausiert · v{n} |
| `pill_draft` | Draft | Entwurf |
| `pill_link_missing` | Link missing | Verknüpfung fehlt |
| `pill_label` | Flow status | Flow-Status |
| `pill_next_run` | Next run | Nächster Lauf |
| `pill_next_none` | No run scheduled | Kein geplanter Lauf |
| `pill_finish_publish` | Finish publishing | Veröffentlichen abschließen |

Existing keys the pill reuses: `state_publish_incomplete`, `state_live_outdated`, `timers_stopped_title`, `live_outdated_hint`, `timers_stopped_text`, `state_run_view`, `state_inactive`, `flow_active` words through the registry, and the status word "Running". (The spec's "Veröffentlichen unvollständig" is the existing de text "Veröffentlichung unvollständig"; the reused key keeps one text for the state everywhere.)

- [ ] **Step 1: apply the keys** and check them

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/b-pill.json
cd W && node -e "for (const l of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) { const j = require('./ui/lang/easydrag/' + l + '.json'); const k = Object.keys(j).filter(x => x.startsWith('easydrag.ui.pill_')); if (k.length !== 9) throw new Error(l + ' ' + k.length); } console.log('16 locales, 9 pill keys')"
```
Expected: `...b-pill.json: 9 keys set, 0 deleted, in 16 files of ui/lang/easydrag/` and `16 locales, 9 pill keys`.

- [ ] **Step 2: write the new files**

Write `ui/js/desktop/apps/easydrag-pill.js`:

```js
// EasyDrag header pill and primary button (W1c). One pill says where the flow stands: a status disc and one short text,
// picked in a fixed order (a run, a past run, a problem, a pause, then the version state). Its popover holds what left the
// header: the Active switch, the next run, Run now, Runs and the action that finishes a partial publish. The primary button
// names the next step: Test, or Publish with its change count, or nothing for a clean live flow. Every control goes through
// the action registry (easydrag-actions.js); the words come from the status map and the easydrag.ui.pill_* keys.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MISSION_MISSING = 'FLOW_MISSION_MISSING';
    // ENTRIES maps a popover entry to its registry action.
    const ENTRIES = { active: 'flow.active', 'run-now': 'flow.run_now', runs: 'view.runs', 'publish-finish': 'flow.publish', repair: 'flow.repair_mission' };

    const missionMissing = ed => ed.publishIncomplete === MISSION_MISSING || (ed.liveIssues || []).some(issue => issue.code === MISSION_MISSING);
    // outdated: the published version no longer passes the server's checks (and the issue is not the missing Mission Control entry).
    const outdated = ed => !ed.runView && !!ed.flow.live && (ed.liveIssues || []).some(issue => issue.code !== MISSION_MISSING) && Number(ed.flow.live_revision) === ed.liveIssuesRevision;
    // changed: a live flow whose draft differs from the live version, or has an edit the server has not seen yet.
    const changed = ed => !!ed.flow.live && (ed.flow.draft_revision !== ed.flow.published_draft_revision || ED.flowState.dirty(ed));

    // describe picks the one state of the pill. f: {running (a run of this flow is going, its own or the one viewed),
    // partialText(code) (the words of a partial publish)}. It returns {kind, status (an ED.status value, '' for no disc), key
    // (the text key; none for a running flow, whose word is the status word), numbered (the text names the live version, {n}), hint}.
    function describe(ed, f) {
        const block = !!ed.isBlock;
        if (f.running) return { kind: 'running', status: 'running', key: '', hint: '' };
        if (ed.runView) return { kind: 'past', status: '', key: 'easydrag.ui.state_run_view', hint: '' };
        if (ed.publishIncomplete && ed.publishIncomplete !== MISSION_MISSING) return { kind: 'partial', status: 'blocked', key: 'easydrag.ui.state_publish_incomplete', hint: f.partialText(ed.publishIncomplete) };
        if (missionMissing(ed)) return { kind: 'link', status: 'blocked', key: 'easydrag.ui.pill_link_missing', hint: f.partialText(MISSION_MISSING) };
        if (outdated(ed)) return { kind: 'outdated', status: 'blocked', key: 'easydrag.ui.state_live_outdated', hint: ed.t('easydrag.ui.live_outdated_hint') };
        if (!block && ed.timersRunning === false) return { kind: 'timers', status: 'uncertain', key: 'easydrag.ui.timers_stopped_title', hint: ed.t('easydrag.ui.timers_stopped_text') };
        if (!block && ed.flow.live && !ed.flowEnabled) return { kind: 'paused', status: 'paused', key: 'easydrag.ui.pill_paused', numbered: true, hint: '' };
        if (changed(ed)) return { kind: 'changed', status: 'ok', key: 'easydrag.ui.pill_live_changed', numbered: true, hint: '' };
        if (ed.flow.live) return { kind: 'live', status: 'ok', key: 'easydrag.ui.pill_live', numbered: true, hint: '' };
        return { kind: 'draft', status: 'not_run', key: 'easydrag.ui.pill_draft', hint: '' };
    }

    function wordOf(ed, d) {
        if (!d.key) return ED.status.word(ed.t, d.status);
        return d.numbered ? ed.t(d.key, { n: Number(ed.flow.live_revision) || 0 }) : ed.t(d.key);
    }

    // primaryOf names the next step. {kind: 'test'} (the draft is not tested yet), {kind: 'publish', count} (tested, with changes;
    // count is null when the server sent none or the flow was never published) or {kind: 'none'}. A partial publish offers Publish
    // again, as it always did; a read-only desktop and a past run offer nothing.
    function primaryOf(ed) {
        if (ed.readonly || ed.runView) return { kind: 'none', count: null };
        if (ed.publishIncomplete) return { kind: 'publish', count: null };
        if (ed.flow.live && !changed(ed)) return { kind: 'none', count: null };
        if (!ED.flowState.isTested(ed)) return { kind: 'test', count: null };
        const count = ed.flow.live ? ED.flowState.changeCount(ed) : null;
        return { kind: 'publish', count: count > 0 ? count : null };
    }

    function markup() {
        return '<button type="button" class="ed-status-pill" data-ed-pill data-ed-revisions aria-haspopup="dialog" aria-expanded="false">' +
            '<span class="ed-status-pill-disc" data-ed-pill-disc></span><span class="ed-status-pill-text" data-ed-pill-text></span></button>';
    }

    // create builds the pill renderer of one editor. host: {ed, el, runs, publish}.
    function create(host) {
        const { ed, el, runs, publish } = host;
        const { t, esc } = ed;
        const anchor = el.querySelector('[data-ed-pill]');
        const disc = anchor.querySelector('[data-ed-pill-disc]');
        const label = anchor.querySelector('[data-ed-pill-text]');
        const c = { host: ed };
        let pop = null;
        let shown = '';

        const facts = () => ({
            running: runs.isRunning() || !!(ed.runView && ed.runView.run && ED.runs.isActive(ed.runView.run.status)),
            partialText: code => publish.partialText(code)
        });

        // The popover is built as {html, texts}: the markup holds the structure and the attributes, the words are set as text after
        // the nodes exist (no escaping to forget), and the pair is compared with the last one so an unchanged popover is left alone.
        function entry(parts, texts, id, name, glyph, text) {
            const attrs = ED.actions.button(id, c);
            if (!attrs) return;
            delete attrs['aria-label'];
            const reason = attrs['aria-disabled'] ? attrs['aria-description'] || '' : '';
            parts.push('<button type="button" class="ed-btn ed-btn--ghost ed-pop-item" data-ed-pop="' + name + '"' + ED.actions.attrString(attrs) + '>' + glyph +
                '<span data-ed-fill="' + name + '"></span>' + (reason ? '<small class="ed-pop-reason" data-ed-fill="' + name + '-reason"></small>' : '') + '</button>');
            texts[name] = text || ED.actions.label(id, c);
            if (reason) texts[name + '-reason'] = reason;
        }

        function switchEntry(parts, texts) {
            const attrs = ED.actions.button('flow.active', c);
            if (!attrs) return;
            delete attrs['aria-label'];
            const reason = attrs['aria-disabled'] ? attrs['aria-description'] || '' : '';
            parts.push('<button type="button" class="ed-switch-btn ed-pop-item" role="switch" aria-checked="' + (ed.flowEnabled ? 'true' : 'false') + '" data-ed-pop="active"' + ED.actions.attrString(attrs) +
                '><span class="ed-switch" aria-hidden="true"><span></span></span><span data-ed-fill="active"></span>' + (reason ? '<small class="ed-pop-reason" data-ed-fill="active-reason"></small>' : '') + '</button>');
            texts.active = ED.actions.label('flow.active', c);
            if (reason) texts['active-reason'] = reason;
        }

        function nextRunText() {
            if (!ed.flow.live) return t('easydrag.ui.pill_next_none');
            if (!ed.flowEnabled) return t('easydrag.ui.state_inactive');
            return ed.flow.next_run ? ED.format.nextRun(ed.flow.next_run.at) : t('easydrag.ui.pill_next_none');
        }

        // content is the popover's markup and words. Blocks have no Active switch, next run or Run now.
        function content() {
            const d = describe(ed, facts());
            const texts = { state: wordOf(ed, d) };
            const parts = ['<div class="ed-pop-body"><p class="ed-pop-state">' + (d.status ? ED.status.disc(d.status, 14, { t, hidden: true }) : '') + '<strong data-ed-fill="state"></strong></p>'];
            if (d.hint) { parts.push('<p class="ed-hint" data-ed-fill="hint"></p>'); texts.hint = d.hint; }
            if (!ed.isBlock) {
                switchEntry(parts, texts);
                const title = ed.flow.live && ed.flowEnabled && ed.flow.next_run ? ' title="' + esc(ED.format.date(ed.flow.next_run.at)) + '"' : '';
                parts.push('<p class="ed-pop-row"><span class="ed-pop-label" data-ed-fill="next-label"></span><span class="ed-pop-value" data-ed-next-run data-ed-fill="next"' + title + '></span></p>');
                texts['next-label'] = t('easydrag.ui.pill_next_run');
                texts.next = nextRunText();
                if (ed.flow.live) entry(parts, texts, 'flow.run_now', 'run-now', ED.icons.of('run'));
                entry(parts, texts, 'view.runs', 'runs', ED.icons.of('runs'));
            }
            if (!ed.runView) {
                if (missionMissing(ed)) entry(parts, texts, 'flow.repair_mission', 'repair', ED.core.icon('link'));
                else if (ed.publishIncomplete || outdated(ed)) entry(parts, texts, 'flow.publish', 'publish-finish', ED.icons.of('publish'), t('easydrag.ui.pill_finish_publish'));
            }
            parts.push('</div>');
            return { html: parts.join(''), texts };
        }

        function render(made) {
            const node = ED.core.el(made.html);
            Object.keys(made.texts).forEach(name => {
                const target = node.querySelector('[data-ed-fill="' + name + '"]');
                if (target) target.textContent = made.texts[name];
            });
            return node;
        }

        // paint draws the pill; an open popover follows when its content changed.
        function paint() {
            const d = describe(ed, facts());
            const word = wordOf(ed, d);
            if (label.textContent !== word) label.textContent = word;
            const mark = d.status ? ED.status.disc(d.status, 14, { t, hidden: true }) : '';
            if (disc.dataset.mark !== mark) { disc.dataset.mark = mark; disc.innerHTML = mark; }
            anchor.setAttribute('data-ed-pill-kind', d.kind);
            anchor.setAttribute('data-status', d.status || 'none');
            anchor.setAttribute('title', d.hint ? word + ' ' + String.fromCodePoint(0x2013) + ' ' + d.hint : word);
            // The revision facts the tests and the stylesheet read: draft (never published), newer (draft ahead of live) or live.
            anchor.dataset.kind = !ed.flow.live ? 'draft' : ed.flow.draft_revision !== ed.flow.published_draft_revision ? 'newer' : 'live';
            anchor.dataset.draft = String(Number(ed.flow.draft_revision) || 0);
            anchor.dataset.live = String(Number(ed.flow.live_revision) || 0);
            anchor.dataset.paused = !ed.isBlock && ed.flow.live && !ed.flowEnabled ? 'true' : 'false';
            if (pop) {
                const made = content();
                const key = JSON.stringify(made);
                if (key !== shown) { shown = key; pop.update(render(made)); }
            }
        }

        function close(restore) { if (pop) pop.close(restore); }

        // onClick: one handler for every entry. The Active switch stays open; any other entry closes the popover and gives the
        // focus back to the pill first, so a dialog it opens returns the focus there.
        function onClick(event) {
            const b = event.target.closest && event.target.closest('[data-ed-pop]');
            if (!b || b.getAttribute('aria-disabled') === 'true') return;
            const id = ENTRIES[b.getAttribute('data-ed-pop')];
            if (!id) return;
            if (id !== 'flow.active') close(true);
            ED.actions.run(id, c);
        }

        function open() {
            if (pop) { pop.close(true); return; }
            const made = content();
            shown = JSON.stringify(made);
            anchor.setAttribute('aria-expanded', 'true');
            pop = ED.kit.popover(anchor, {
                content: render(made), label: t('easydrag.ui.pill_label'),
                onClose: () => { pop = null; anchor.setAttribute('aria-expanded', 'false'); }
            });
            pop.el.setAttribute('data-ed-pill-popover', '');
            pop.el.addEventListener('click', onClick);
            // The next run is read when the popover opens: the schedule may have changed since the editor loaded.
            ED.flowState.refresh(ed);
        }

        anchor.addEventListener('click', open);

        return { paint, open, close, isOpen: () => !!pop, dispose() { close(false); } };
    }

    ED.pill = { markup, create, describe, primaryOf, missionMissing, outdated, changed };
})();
```

Write `scripts/easydrag-header-test-helper.mjs` (the name does not match `test-easydrag*.mjs`, so T0 imports it from the checks and does not run it alone):

```js
// Helpers of the Node checks that act on the editor header's pill (W1c). The name does not match test-easydrag*.mjs: the T0 run
// imports it from the checks and does not run it alone.

// pillText is what the pill says.
export const pillText = editor => editor.el.querySelector('[data-ed-pill-text]').textContent;

// pillKind is the state the pill shows: running, past, partial, link, outdated, timers, paused, changed, live or draft.
export const pillKind = editor => editor.el.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind');

// popEntry opens the pill's popover when it is closed and returns its entry data-ed-pop="name" (active, run-now, runs,
// publish-finish, repair), or null when the popover has none. The popover stays open until an entry closes it.
export function popEntry(editor, name) {
    if (!editor.el.querySelector('[data-ed-pill-popover]')) editor.el.querySelector('[data-ed-pill]').fire('click');
    return editor.el.querySelector('[data-ed-pill-popover] [data-ed-pop="' + name + '"]');
}
```

Write `ui/desktop_easydrag_header_helpers_test.go`:

```go
package ui

// Helpers of the EasyDrag browser tests that act on the editor header's pill (W1c). The pill's popover holds what left the
// header: the Active switch, Run now, Runs and the actions that finish a publish.

// pillEntry opens the pill's popover when it is closed and clicks its entry: active, run-now, runs, publish-finish or repair. The
// popover stays open after Active (the switch acts in place) and closes after every other entry.
func (s *easyDragSmoke) pillEntry(name string) {
	s.t.Helper()
	if !s.page.MustEval(`()=>!!document.querySelector('[data-ed-pill-popover]')`).Bool() {
		s.page.MustElement(`[data-ed-pill]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	}
	s.page.MustElement(`[data-ed-pill-popover] [data-ed-pop="` + name + `"]`).MustClick()
}
```

- [ ] **Step 3: add the failing checks** (edit group `b3-test`):

<!-- edits:b3-test -->
**Edit** `scripts/test-easydrag-header.mjs`

Old:
```text
    // @@APPEND@@
```

New:
```text
    // ── B.3: the pill and its popover ──

    // pillOf reads the pill: [kind, status, text].
    const pillOf = editor => {
        const b = editor.el.querySelector('[data-ed-pill]');
        return [b.getAttribute('data-ed-pill-kind'), b.getAttribute('data-status'), b.querySelector('[data-ed-pill-text]').textContent];
    };
    // show sets what the editor knows and paints the header again (the bus event of a switch does that).
    const show = (editor, patch, flow) => {
        Object.assign(editor.ed, patch);
        Object.assign(editor.ed.flow, flow || {});
        editor.ed.bus.emit('enabled');
        return pillOf(editor);
    };
    // publishedFlow opens f1 as v2 with a stub server: world.enabled and world.next are what GET answers; the switch and Run now answer too.
    const publishedFlow = (world, opts) => {
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return Object.assign({ flow: recordOf(2), enabled: world.enabled, issues: [] }, world.next ? { next_run: { at: world.next, node: T1 } } : {});
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/enabled') { world.enabled = !!req.body.enabled; return { enabled: world.enabled }; }
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run') return { run_id: 'r9' };
            if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
            return undefined;
        });
        const editor = openEditor(h, Object.assign({ flow: recordOf(2), enabled: world.enabled }, opts));
        return { h, editor };
    };

    await guardAsync('w1c B.3 the pill says one thing, in the fixed order: run, past run, problem, pause, version', async () => {
        const world = { enabled: true };
        const { h, editor } = publishedFlow(world);
        await settle();
        const live = { status: 'success', revision: 2, started_at: '2026-11-16T10:00:00Z' };
        const rows = [];
        rows.push(['live', pillOf(editor)]);
        rows.push(['draft ahead', show(editor, {}, { draft_revision: 5 })]);
        rows.push(['paused', show(editor, { flowEnabled: false })]);
        rows.push(['timers over paused', show(editor, { timersRunning: false })]);
        rows.push(['outdated over timers', show(editor, { liveIssues: [{ code: 'FLOW_SCHEDULE_INVALID' }], liveIssuesRevision: 2 })]);
        rows.push(['link over outdated', show(editor, { liveIssues: [{ code: 'FLOW_SCHEDULE_INVALID' }, { code: 'FLOW_MISSION_MISSING' }], liveIssuesRevision: 2 })]);
        rows.push(['partial over link', show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' })]);
        rows.push(['past run over problems', show(editor, { runView: { run: live } })]);
        rows.push(['a running viewed run over everything', show(editor, { runView: { run: Object.assign({}, live, { status: 'running' }) } })]);
        rows.push(['mission missing alone', show(editor, { runView: null, liveIssues: [], publishIncomplete: 'FLOW_MISSION_MISSING' })]);
        rows.push(['all clear', show(editor, { publishIncomplete: '', timersRunning: true, flowEnabled: true }, { draft_revision: 2 })]);
        const draft = openEditor(sandbox(() => undefined));
        await settle();
        rows.push(['never published', pillOf(draft)]);
        draft.dispose();
        const dsc = editor.el.querySelector('[data-ed-pill-disc]');
        eq('w1c B.3 pill rows (kind, disc, text)', rows, [
            ['live', ['live', 'ok', 'pill_live:2']],
            ['draft ahead', ['changed', 'ok', 'pill_live_changed:2']],
            ['paused', ['paused', 'paused', 'pill_paused:2']],
            ['timers over paused', ['timers', 'uncertain', 'timers_stopped_title']],
            ['outdated over timers', ['outdated', 'blocked', 'state_live_outdated']],
            ['link over outdated', ['link', 'blocked', 'pill_link_missing']],
            ['partial over link', ['partial', 'blocked', 'state_publish_incomplete']],
            ['past run over problems', ['past', 'none', 'state_run_view']],
            ['a running viewed run over everything', ['running', 'running', 'running']],
            ['mission missing alone', ['link', 'blocked', 'pill_link_missing']],
            ['all clear', ['live', 'ok', 'pill_live:2']],
            ['never published', ['draft', 'not_run', 'pill_draft']]
        ]);
        eq('w1c B.3 the pill draws a status disc (none for a past run), is a button with a popover, and the old chips and revision text are gone', [
            dsc.html.includes('data-status="ok"'), editor.el.querySelector('[data-ed-pill]').localName, editor.el.querySelector('[data-ed-pill]').getAttribute('aria-haspopup'),
            editor.el.querySelectorAll('[data-ed-state] .ed-chip').length, !!editor.el.querySelector('[data-ed-revision]'), h.logged], [true, 'button', 'dialog', 0, false, []]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the pill tooltip carries the hint of a problem, the revision facts stay on the pill', async () => {
        const { editor } = publishedFlow({ enabled: true });
        await settle();
        const pill = editor.el.querySelector('[data-ed-pill]');
        const plain = [pill.getAttribute('title'), pill.dataset.kind, pill.dataset.draft, pill.dataset.live, pill.dataset.paused];
        show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        const partial = pill.getAttribute('title');
        show(editor, { publishIncomplete: '', flowEnabled: false }, { draft_revision: 5 });
        const paused = [pill.dataset.kind, pill.dataset.draft, pill.dataset.paused];
        eq('w1c B.3 pill title and revision facts', [plain, partial, paused], [
            ['pill_live:2', 'live', '2', '2', 'false'],
            'state_publish_incomplete ' + String.fromCodePoint(0x2013) + ' error_flow_publish_incomplete',
            ['newer', '5', 'true']
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the popover: state, Active, next run, Run now and Runs; the next run is read when it opens', async () => {
        const world = { enabled: true, next: null };
        const { h, editor } = publishedFlow(world);
        await settle();
        const ED = h.ED, pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const entries = () => Array.from(pop().querySelectorAll('[data-ed-pop]')).map(b => b.getAttribute('data-ed-pop'));
        const gets = () => h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/f1').length;
        const getsBefore = gets();
        world.next = '2026-11-17T12:00:00Z';
        pill.fire('click');
        const opened = [!!pop(), pill.getAttribute('aria-expanded'), pop().getAttribute('role'), h.dom.document.activeElement === pop().querySelector('[data-ed-pop="active"]'), entries(),
            pop().querySelector('[data-ed-next-run]').textContent, pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked')];
        await settle();
        // The read that came with the open brought the next run; the popover shows it as weekday and time, and keeps the focus.
        const next = pop().querySelector('[data-ed-next-run]').textContent;
        const nextOk = /^Tue/.test(next) && /\d:\d\d/.test(next);
        const later = ED.format.nextRun('2026-11-27T12:00:00Z', new Date('2026-11-16T10:00:00Z'));
        const soon = ED.format.nextRun('2026-11-17T12:00:00Z', new Date('2026-11-16T10:00:00Z'));
        eq('w1c B.3 popover contents and the read on open', [opened, gets() - getsBefore, nextOk, /Nov/.test(later) && !/Tue|Fri/.test(later), /^Tue/.test(soon), ED.format.nextRun('nope'), editor.ed.flow.next_run.at, h.dom.document.activeElement === pop().querySelector('[data-ed-pop="active"]')], [
            [true, 'true', 'dialog', true, ['active', 'run-now', 'runs'], 'pill_next_none', 'true'], 1, true, true, true, '', '2026-11-17T12:00:00Z', true
        ]);
        pill.fire('click');
        eq('w1c B.3 a second click on the pill closes the popover and gives the focus back', [!!pop(), pill.getAttribute('aria-expanded'), h.dom.document.activeElement === pill], [false, 'false', true]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the popover says paused, none and the reasons of the registry', async () => {
        const rows = [];
        const paused = publishedFlow({ enabled: false, next: '2026-11-17T12:00:00Z' });
        await settle();
        paused.editor.el.querySelector('[data-ed-pill]').fire('click');
        const p = paused.editor.el.querySelector('[data-ed-pill-popover]');
        rows.push(['paused', p.querySelector('[data-ed-next-run]').textContent, p.querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'),
            p.querySelector('[data-ed-pop="run-now"]').getAttribute('aria-disabled'), p.querySelector('[data-ed-pop="run-now"]').getAttribute('aria-description'),
            p.querySelector('[data-ed-pop="run-now"]').querySelector('.ed-pop-reason') !== null]);
        paused.editor.dispose();
        const ro = openEditor(sandbox(() => undefined), { readonly: true, flow: recordOf(2), enabled: true });
        await settle();
        ro.el.querySelector('[data-ed-pill]').fire('click');
        const r = ro.el.querySelector('[data-ed-pill-popover]');
        rows.push(['read-only', r.querySelector('[data-ed-pop="active"]').getAttribute('aria-disabled'), r.querySelector('[data-ed-pop="active"]').getAttribute('aria-description'),
            r.querySelector('[data-ed-pop="runs"]').getAttribute('aria-disabled')]);
        ro.dispose();
        const draft = openEditor(sandbox(() => undefined));
        await settle();
        draft.el.querySelector('[data-ed-pill]').fire('click');
        const d = draft.el.querySelector('[data-ed-pill-popover]');
        rows.push(['never published', d.querySelector('[data-ed-next-run]').textContent, !!d.querySelector('[data-ed-pop="run-now"]'), !!d.querySelector('[data-ed-pop="active"]'), !!d.querySelector('[data-ed-pop="runs"]')]);
        draft.dispose();
        eq('w1c B.3 popover reasons', rows, [
            ['paused', 'state_inactive', 'false', 'true', 'reason_paused', true],
            ['read-only', 'true', 'reason_readonly', null],
            ['never published', 'pill_next_none', false, true, true]
        ]);
    });

    await guardAsync('w1c B.3 the entries act through the registry: Active stays open, Run now and Runs close the popover first', async () => {
        const world = { enabled: true };
        const { h, editor } = publishedFlow(world);
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const posts = url => h.requests.filter(r => r.method === 'POST' && r.url === url).length;
        pill.fire('click');
        pop().querySelector('[data-ed-pop="active"]').fire('click');
        await settle();
        const afterActive = [!!pop(), world.enabled, pop() && pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'), pillOf(editor)[0], posts('/api/desktop/flows/f1/enabled')];
        pop().querySelector('[data-ed-pop="active"]').fire('click');
        await settle();
        const backOn = [world.enabled, pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'), pillOf(editor)[0]];
        pop().querySelector('[data-ed-pop="run-now"]').fire('click');
        await settle();
        const afterRun = [!!pop(), posts('/api/desktop/flows/f1/run'), h.dom.document.activeElement === pill];
        pill.fire('click');
        pop().querySelector('[data-ed-pop="runs"]').fire('click');
        await settle();
        const afterRuns = [!!pop(), !!editor.el.querySelector('.ed-drawer')];
        stop();
        eq('w1c B.3 popover actions', [afterActive, backOn, afterRun, afterRuns, ran, h.logged], [
            [true, false, 'false', 'paused', 1], [true, 'true', 'live'], [false, 1, true], [false, true],
            ['flow.active', 'flow.active', 'flow.run_now', 'view.runs'], []
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a refused entry does nothing, and Escape closes the popover and returns the focus', async () => {
        const { h, editor } = publishedFlow({ enabled: false });
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        pill.fire('click');
        editor.el.querySelector('[data-ed-pill-popover] [data-ed-pop="run-now"]').fire('click');
        await settle();
        const refused = [!!editor.el.querySelector('[data-ed-pill-popover]'), ran.slice(), h.requests.filter(r => r.url === '/api/desktop/flows/f1/run').length];
        const key = editor.el.querySelector('[data-ed-pill-popover]').fire('keydown', { key: 'Escape' });
        stop();
        eq('w1c B.3 refused entry and Escape', [refused, !!editor.el.querySelector('[data-ed-pill-popover]'), h.dom.document.activeElement === pill, key.defaultPrevented, pill.getAttribute('aria-expanded')],
            [[true, [], 0], false, true, true, 'false']);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a partial publish and a missing link name the action that finishes them', async () => {
        const { h, editor } = publishedFlow({ enabled: true });
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const list = () => Array.from(pop().querySelectorAll('[data-ed-pop]')).map(b => b.getAttribute('data-ed-pop'));
        const rows = [];
        show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        pill.fire('click');
        rows.push(['partial', list(), pop().querySelector('[data-ed-fill="publish-finish"]').textContent, !!pop().querySelector('.ed-hint')]);
        pop().querySelector('[data-ed-pop="publish-finish"]').fire('click');
        await settle();
        rows.push(['finish', !!pop(), ran.slice()]);
        // The popover follows a change while it is open: the link goes missing, so Repair replaces Finish.
        pill.fire('click');
        show(editor, { publishIncomplete: 'FLOW_MISSION_MISSING' });
        rows.push(['missing', list()]);
        pop().querySelector('[data-ed-pop="repair"]').fire('click');
        await settle();
        rows.push(['repair', !!pop(), ran.slice()]);
        stop();
        eq('w1c B.3 completing actions', rows, [
            ['partial', ['active', 'run-now', 'runs', 'publish-finish'], 'pill_finish_publish', true],
            ['finish', false, ['flow.publish']],
            ['missing', ['active', 'run-now', 'runs', 'repair']],
            ['repair', false, ['flow.publish', 'flow.repair_mission']]
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a block has no Active, next run or Run now: its popover shows the state only', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h, { block: true, flow: { live: flowDoc(), live_revision: 3, published_draft_revision: 3 } });
        await settle();
        const kind = pillOf(editor);
        editor.el.querySelector('[data-ed-pill]').fire('click');
        const pop = editor.el.querySelector('[data-ed-pill-popover]');
        eq('w1c B.3 block pill and popover', [kind, pop.querySelectorAll('[data-ed-pop]').length, !!pop.querySelector('[data-ed-next-run]'), h.logged], [['live', 'ok', 'pill_live:3'], 0, false, []]);
        editor.dispose();
    });

    // @@APPEND@@
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b3-test
```

- [ ] **Step 4: write and run the patch scripts**

Write `C:/disposable/easydrag-w1c/patches/b3-format.mjs` (the next run, short):

```js
// B.3: ED.format.nextRun. Run from the worktree root.
import { edit } from '../lib-patch.mjs';

const FILE = 'ui/js/desktop/apps/easydrag-format.js';

edit(FILE, `    // list joins items with the language's "and"`,
`    // nextRun says when a flow runs next, short: the weekday and the time inside the next six days ("Mo., 07:00"), the day, the
    // month and the time after that ("17. Nov., 07:00"). The time shown is the browser's. now is for tests; an invalid value gives ''.
    function nextRun(value, now) {
        const d = toDate(value);
        if (!d) return '';
        const from = now instanceof Date ? now : new Date();
        const soon = d.getTime() - from.getTime() < 6 * 86400000;
        const opts = soon ? { weekday: 'short', hour: '2-digit', minute: '2-digit' } : { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' };
        try { return intl('DateTimeFormat', locale(), opts).format(d); } catch (err) { return d.toISOString(); }
    }

    // list joins items with the language's "and"`);

edit(FILE, `    ED.format = { locale, category, key, number, plural, duration, date, relative, list,`,
`    ED.format = { locale, category, key, number, plural, duration, date, relative, nextRun, list,`);
console.log('b3-format: nextRun added to ' + FILE);
```

Write `C:/disposable/easydrag-w1c/patches/b3-pill.mjs` (registration, header.js, editor.js, CSS):

```js
// B.3: the pill and its popover replace the state chips, the revision text and the Runs, Run now and Active buttons of the header.
// Run from the worktree root.
import { edit, replaceBlock } from '../lib-patch.mjs';

const HEADER = 'ui/js/desktop/apps/easydrag-header.js';
const EDITOR = 'ui/js/desktop/apps/easydrag-editor.js';
const CSS = 'ui/css/desktop-app-easydrag.css';

// ── registration: the module loads after the flow state and before the header ──
edit('ui/js/desktop/core/module-loader.js', `                '/js/desktop/apps/easydrag-flow-state.js',
`, `                '/js/desktop/apps/easydrag-flow-state.js',
                '/js/desktop/apps/easydrag-pill.js',
`);
edit('ui/desktop_easydrag_test.go', `"easydrag-flow-state.js", "easydrag-header.js",`, `"easydrag-flow-state.js", "easydrag-pill.js", "easydrag-header.js",`);
edit('scripts/test-easydrag-extra3.mjs', `'easydrag-flow-state.js', 'easydrag-header.js',`, `'easydrag-flow-state.js', 'easydrag-pill.js', 'easydrag-header.js',`);

// ── header.js ──
edit(HEADER, `// EasyDrag editor header and footer: the flow name, the state, revision, live and timer chips,
// the action buttons, the save state, the issues count and the last run. easydrag-editor.js
// creates it once per editor and keeps the run view (renderBanner), the commands and the bus wiring.`,
`// EasyDrag editor header and footer: the flow name, the pill (easydrag-pill.js: the state, its popover),
// the action buttons, the save state, the issues count and the last run. easydrag-editor.js
// creates it once per editor and keeps the run view (renderBanner), the commands and the bus wiring.`);
edit(HEADER, `    // palette, publish, nameInput, renderBanner(), isDisposed(), lastRecord()}; t, esc and ctx come from ed.`,
`    // palette, publish, nameInput, bag, renderBanner(), isDisposed(), lastRecord()}; t, esc and ctx come from ed.`);
replaceBlock(HEADER, `// stateOf names the flow's state`, `// paintActions draws the action buttons`,
`        // The pill (the flow's state in one text, and its popover with Active, the next run, Run now and Runs) is easydrag-pill.js.
        const pill = ED.pill.create({ ed, el, runs, publish });

`, 23);
replaceBlock(HEADER, `function paintRevision() {`, `function renderHeader() {`, ``, 34);
edit(HEADER, `            [['test', 'flow.test'], ['cancel', 'flow.stop'], ['publish', 'flow.publish'], ['run-now', 'flow.run_now'], ['active', 'flow.active']].forEach(([cmd, id]) => {
                const button = el.querySelector('[data-ed-cmd="' + cmd + '"]');
                if (button) ED.actions.paint(button, id, c);
            });`,
`            [['test', 'flow.test'], ['cancel', 'flow.stop'], ['publish', 'flow.publish']].forEach(([cmd, id]) => {
                const button = el.querySelector('[data-ed-cmd="' + cmd + '"]');
                if (button) ED.actions.paint(button, id, c);
            });
            // The pill's popover shows the same reasons (a dialog or a menu opening or closing changes them).
            pill.paint();`);
replaceBlock(HEADER, `const s = stateOf();`, `const ro = !!(ed.readonly || ed.runView);`,
`            syncSaveChip();
`, 6);
edit(HEADER, `                btn('runs').hidden = true; btn('active').hidden = true;
`, ``);
edit(HEADER, `            btn('publish').classList.toggle('has-changes', s.cls !== 'ok' && !ro);
            btn('publish').classList.toggle('ed-btn--primary', !!ed.flow.live && !ro);
            btn('run-now').hidden = isBlock || !ed.flow.live;
            btn('active').setAttribute('aria-checked', String(!!ed.flowEnabled));
            paintActions();
            btn('runs').setAttribute('aria-pressed', String(runs.drawerOpen()));
`, `            btn('publish').classList.toggle('has-changes', (!ed.flow.live || ED.pill.changed(ed) || !!ed.publishIncomplete) && !ro);
            btn('publish').classList.toggle('ed-btn--primary', !!ed.flow.live && !ro);
            paintActions();
`);
edit(HEADER, `            node.title = s === 'failed' ? '' : title;
`, `            node.title = s === 'failed' ? '' : title;
            if (!host.isDisposed()) pill.paint();
`);
edit(HEADER, `        return { renderHeader, renderSaveState, renderIssues, renderLastRun, paintActions };`,
`        // The facts of the server (next run, tested, changes) arrive after the first paint: the pill follows them.
        host.bag.add(ed.bus.on('flow-facts', renderHeader));

        return { renderHeader, renderSaveState, renderIssues, renderLastRun, paintActions, dispose() { pill.dispose(); } };`);

// ── editor.js ──
edit(EDITOR, `<span class="ed-state" data-ed-state></span><span class="ed-revision" data-ed-revision data-ed-revisions></span>`,
`<span class="ed-state" data-ed-state>' + ED.pill.markup() + '</span>`);
edit(EDITOR, `            '<button type="button" class="ed-btn ed-btn--ghost" data-ed-cmd="runs" aria-pressed="false">' + ED.icons.of('runs') + '<span>' + esc(t('easydrag.ui.runs_title')) + '</span></button>' +
`, ``);
edit(EDITOR, `            '<button type="button" class="ed-btn" data-ed-cmd="run-now" hidden>' + ED.icons.of('run') + '<span>' + lab('flow.run_now') + '</span></button>' +
            '<button type="button" class="ed-switch-btn" role="switch" aria-checked="false" data-ed-cmd="active" aria-label="' + lab('flow.active') + '"><span class="ed-switch" aria-hidden="true"><span></span></span><span>' + lab('flow.active') + '</span></button>' +
`, ``);
edit(EDITOR, `            ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner,`, `            ed, el, isBlock, canvas, runs, palette, publish, nameInput, bag, renderBanner,`);
edit(EDITOR, `            else if (cmd === 'runs') { runs.toggleDrawer(); renderHeader(); setMenus(); }
`, ``);
edit(EDITOR, `            else if (cmd === 'run-now') runLiveNow();
            else if (cmd === 'active') publish.setActive(!ed.flowEnabled);
`, ``);
edit(EDITOR, `[ed.childHistory, ed.runtime, extraction, blockTracker, authoring, workspace, wires, canvas, palette, runs, publish, history]`,
`[ed.childHistory, ed.runtime, extraction, blockTracker, authoring, workspace, wires, canvas, palette, runs, publish, history, headerView]`);

// ── CSS ──
edit(CSS, `.ed-state { display: flex; gap: 6px; flex: none; }
`, `.ed-state { display: flex; gap: 6px; flex: 0 1 auto; min-width: 0; max-width: 100%; }
/* the status pill (easydrag-pill.js): a status disc and one short text; its popover holds what left the header */
.ed-status-pill {
    display: inline-flex; align-items: center; gap: 6px; max-width: min(20rem, 100%); min-width: 0; min-height: 28px; padding: 3px 10px 3px 8px;
    border: 1px solid var(--ed-border); border-radius: 999px; background: var(--ed-control); color: var(--ed-text);
    font: inherit; font-size: var(--ed-font-meta); font-weight: 600; line-height: 1.2; cursor: pointer;
}
.ed-status-pill:hover { background: var(--ed-control-hover); border-color: var(--ed-border-strong); }
.ed-status-pill[aria-expanded="true"] { background: var(--ed-selected); border-color: var(--ed-selected-color); color: var(--ed-selected-text); }
.ed-status-pill[data-status="blocked"], .ed-status-pill[data-status="uncertain"] { border-color: color-mix(in srgb, var(--ed-warn) 55%, var(--ed-border)); }
.ed-status-pill-disc { display: inline-flex; flex: none; }
.ed-status-pill-disc:empty { display: none; }
.ed-status-pill-text { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
`);
edit(CSS, `    /* the state chip goes, Paused stays (U13): a paused flow must be told apart on a phone too */
    .ed-state > :not([data-ed-paused], [data-ed-live-outdated], [data-ed-timers-off]) { display: none; }
    .ed-state > [data-ed-live-outdated], .ed-state > [data-ed-timers-off] { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
`, `    /* the pill stays at every width and cuts its text with an ellipsis; "Paused" leads its text (U13): a paused flow is told apart on a phone too */
`);
edit(CSS, `.ed-popover header h3 {`, `/* the pill's popover (kit popover): the state, the Active switch, the next run, Run now, Runs and the action that finishes a publish */
.ed-popover--anchored { width: min(340px, calc(100% - 16px)); }
.ed-pop-body { display: grid; gap: 6px; }
.ed-pop-state { display: flex; align-items: center; gap: 8px; margin: 2px 4px; }
.ed-pop-state strong { min-width: 0; overflow-wrap: anywhere; }
.ed-pop-body > .ed-hint { margin: 0 4px; }
.ed-pop-row { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; margin: 0 4px; }
.ed-pop-label { color: var(--ed-muted); }
.ed-pop-value { font-weight: 600; font-variant-numeric: tabular-nums; text-align: right; }
.ed-pop-item { width: 100%; justify-content: flex-start; flex-wrap: wrap; text-align: left; }
.ed-pop-reason { flex: 1 0 100%; padding-left: 22px; font-weight: 400; color: var(--ed-muted); font-size: var(--ed-font-meta); white-space: normal; }
.ed-popover header h3 {`);
console.log('b3-pill: done');
```

```
cd W && node C:/disposable/easydrag-w1c/patches/b3-format.mjs && node C:/disposable/easydrag-w1c/patches/b3-pill.mjs
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-header.js
cd W && grep -n "stateOf\|paintRevision\|liveChip\|timersChip\|data-ed-revision\b\|data-ed-cmd=\"runs\"\|data-ed-cmd=\"run-now\"\|data-ed-cmd=\"active\"" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-header.js
```
Expected: both scripts print `done`; the editor has 855 lines or fewer (baseline 859, rule 896), the header about 143; the `grep` finds no old form in the editor or the header (the shortcut and menu code do not use these hooks).

- [ ] **Step 5: move the tests that read the chips or press the three buttons**

The checks below read the pill (`pillText`, `pillKind`) and press popover entries (`popEntry`) instead. Nothing is weakened: every old expectation keeps its meaning; where a state is now one text instead of two chips, the expectation names the one text; where a control moved, the check opens the popover first (the stub has no MutationObserver, so the one check that outlives a closed dialog repaints the editor the way the browser's observer does: `a1008`).

| File | Checks changed | What changed |
|---|---|---|
| `scripts/test-easydrag-blocks.mjs` | `blocks editor hides activation and history` | asks the popover (null entries) |
| `scripts/test-easydrag-extra3.mjs` | `chipState`, `c1d07 a partial publish ...` | reads the pill (kind, text, tooltip); the stub flow is switched on so the pill shows the version state, not the pause |
| `scripts/test-easydrag-extra5.mjs` | `chips`, `ff2 I1 ...`, `ff2 M1 ...`, merge read-only, the `ux ...` checks | pill text; `popEntry` for Active and Run now; the read-counts are relative to the popover's own read |
| `scripts/test-easydrag-extra6.mjs` | `a1008 runs one test ...`, `a1008 runs stop covers ...` | `popEntry` for Run now and Runs |
| `scripts/test-easydrag-extra7.mjs` | `a1023 F5`, `a1023b U1` (names), `a1023b U3` | `popEntry`; Active left the header, so its name check goes |
| `scripts/test-easydrag-flagship.mjs` | two checks | `popEntry` for Runs |
| `scripts/test-easydrag-live-state.mjs` | `a1025 R2b`, `a1025 R4` | pill kind `outdated` and `timers` instead of chips |
| `scripts/test-easydrag-render-baseline.mjs` | `runEvent` | `popEntry` for Run now |
| `scripts/test-easydrag-ux-batch5.mjs` | `b5 G2`, `b5 F6`, `b5 rerun`, `b5 U4 U6`, `b5 V1`, `b5 V4 U13`, the 560 px rule check | `popEntry`; pill text; the rule check says no 560 px rule hides the pill |
| `ui/desktop_easydrag_browser_test.go` | journey, paused-elsewhere, stop-waiting-runs, failed-run-view | `s.pillEntry("runs")` (six sites), pill attributes instead of chips |
| `ui/desktop_easydrag_approval_browser_test.go`, `..._durable_...`, `..._voice_...` | Runs, Run now, Active | `s.pillEntry` |
| `ui/desktop_easydrag_ux_browser_test.go` | phone paused chip, header names | pill kind; Active no longer in the header |
| `ui/desktop_easydrag_blocks_browser_test.go` | block hides activation and history | the buttons are absent |

Write `C:/disposable/easydrag-w1c/patches/b3-tests.mjs` and `b3-go.mjs`:

```js
// B.3: the Node checks that read the state chips, the revision line or press Runs, Run now and Active move to the pill and its
// popover. Run from the worktree root, after b3-pill.mjs.
import { edit } from '../lib-patch.mjs';

const IMPORT = `import { popEntry, pillText, pillKind } from './easydrag-header-test-helper.mjs';\n`;
const withImport = file => edit(file, `import fs from 'node:fs';\n`, `import fs from 'node:fs';\n` + IMPORT);
const DASH = `' ' + String.fromCodePoint(0x2013) + ' '`;

// ── blocks ──
edit('scripts/test-easydrag-blocks.mjs', `export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {`, IMPORT + `export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {`);
edit('scripts/test-easydrag-blocks.mjs', `        eq('blocks editor hides activation and history', [editor.el.querySelector('[data-ed-cmd="active"]').hidden, editor.el.querySelector('[data-ed-cmd="runs"]').hidden], [true, true]);`,
`        eq('blocks editor offers no activation, next run, Run now or history: its popover shows the state only',
            [popEntry(editor, 'active'), popEntry(editor, 'runs'), popEntry(editor, 'run-now'), editor.el.querySelector('[data-ed-next-run]')], [null, null, null, null]);`);

// ── extra3: the state chip became the pill ──
edit('scripts/test-easydrag-extra3.mjs', `// chipState reads the editor's state chip: [colour, text, title, Publish disabled].
function chipState(editor) {
    const m = /<span class="ed-chip ed-chip--(\\w+)"(?: title="([^"]*)")?>([^<]*)<\\/span>/.exec(editor.el.querySelector('[data-ed-state]').html);
    return m ? [m[1], m[3], m[2] === undefined ? null : m[2], editor.el.querySelector('[data-ed-cmd="publish"]').disabled] : null;
}`, `// chipState reads the editor's pill: [kind, text, tooltip, Publish disabled].
function chipState(editor) {
    const pill = editor.el.querySelector('[data-ed-pill]');
    return [pill.getAttribute('data-ed-pill-kind'), pill.querySelector('[data-ed-pill-text]').textContent, pill.getAttribute('title'), editor.el.querySelector('[data-ed-cmd="publish"]').disabled];
}`);
edit('scripts/test-easydrag-extra3.mjs', `            ['accent', 'state_changes', null, false],
            ['warn', 'state_publish_incomplete', 'error_flow_publish_incomplete', false],
            ['warn', 'state_publish_incomplete', 'error_flow_publish_incomplete', false],
            ['ok', 'state_published', null, false]`, `            ['changed', 'pill_live_changed:2', 'pill_live_changed:2', false],
            ['partial', 'state_publish_incomplete', 'state_publish_incomplete' + ${DASH} + 'error_flow_publish_incomplete', false],
            ['partial', 'state_publish_incomplete', 'state_publish_incomplete' + ${DASH} + 'error_flow_publish_incomplete', false],
            ['live', 'pill_live:4', 'pill_live:4', false]`);
// The stub's flow is switched on, so the pill shows the version state and not the pause.
edit('scripts/test-easydrag-extra3.mjs', `return { flow: record(liveRevision), enabled: false, issues: [] };`, `return { flow: record(liveRevision), enabled: true, issues: [] };`);
edit('scripts/test-easydrag-extra3.mjs', `        const editor = openEditor(h, { flow: { published_draft_revision: 2, live: flowDoc(), live_revision: 2 } });
        await settle();
        const states = [chipState(editor)];`, `        const editor = openEditor(h, { enabled: true, flow: { published_draft_revision: 2, live: flowDoc(), live_revision: 2 } });
        await settle();
        const states = [chipState(editor)];`);

// ── extra5 ──
withImport('scripts/test-easydrag-extra5.mjs');
edit('scripts/test-easydrag-extra5.mjs', `function chips(editor) {
    return Array.from(editor.el.querySelector('[data-ed-state]').html.matchAll(/<span class="ed-chip ed-chip--\\w+"[^>]*>([^<]*)<\\/span>/g)).map(m => m[1]);`,
`function chips(editor) {
    return [pillText(editor)];`);
edit('scripts/test-easydrag-extra5.mjs', `        const rows = [['open', chips(editor), runNow(h, editor), gets(h)]];`,
`        // The popover stays open: its read of the next run is part of the baseline of the counts.
        popEntry(editor, 'active');
        const base = gets(h);
        const rows = [['open', chips(editor), runNow(h, editor), gets(h) - base]];`);
edit('scripts/test-easydrag-extra5.mjs', `        rows.push(['paused here', chips(editor), runNow(h, editor), gets(h), editor.el.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')]);`,
`        rows.push(['paused here', chips(editor), runNow(h, editor), gets(h) - base, popEntry(editor, 'active').getAttribute('aria-checked')]);`);
edit('scripts/test-easydrag-extra5.mjs', `        rows.push(['other', gets(h)]);`, `        rows.push(['other', gets(h) - base]);`);
edit('scripts/test-easydrag-extra5.mjs', `        rows.push(['maybe mine', chips(editor), runNow(h, editor), gets(h), editor.el.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')]);`,
`        rows.push(['maybe mine', chips(editor), runNow(h, editor), gets(h) - base, popEntry(editor, 'active').getAttribute('aria-checked')]);`);
edit('scripts/test-easydrag-extra5.mjs', `            ['open', ['state_published'], [[false, ''], [false, '']], 0],
            ['paused here', ['state_published', 'state_inactive'], [[true, 'reason_paused'], [true, 'reason_paused']], 1, 'false'],
            ['other', 1],
            ['maybe mine', ['state_published'], [[false, ''], [false, '']], 2, 'true']`,
`            ['open', ['pill_live:1'], [[false, ''], [false, '']], 0],
            ['paused here', ['pill_paused:1'], [[true, 'reason_paused'], [true, 'reason_paused']], 1, 'false'],
            ['other', 1],
            ['maybe mine', ['pill_live:1'], [[false, ''], [false, '']], 2, 'true']`);
edit('scripts/test-easydrag-extra5.mjs', `        s.editor.el.querySelector('[data-ed-cmd="active"]').fire('click');`, `        popEntry(s.editor, 'active').fire('click');`);
edit('scripts/test-easydrag-extra5.mjs', `            [false, ['state_published', 'state_inactive'], [[true, 'reason_paused'], [true, 'reason_paused']], []]);`,
`            [false, ['pill_paused:1'], [[true, 'reason_paused'], [true, 'reason_paused']], []]);`);
edit('scripts/test-easydrag-extra5.mjs', `            [[0, true, ['state_published'], 'saved', false, true, false], 1, 1, []]);`, `            [[0, true, ['pill_live:1'], 'saved', false, true, false], 1, 1, []]);`);
edit('scripts/test-easydrag-extra5.mjs', `        const before = [!btn('stop-viewed').hidden, off(btn('test')), off(btn('publish')), off(btn('active')), !!(del && del.disabled)];`,
`        const before = [!btn('stop-viewed').hidden, off(btn('test')), off(btn('publish')), off(popEntry(editor, 'active')), !!(del && del.disabled)];`);
edit('scripts/test-easydrag-extra5.mjs', `                cmdBtn(draft.el, 'test').getAttribute('title').includes('test_draft'), cmdBtn(draft.el, 'run-now').hidden,
                draft.el.querySelector('[data-ed-revision]').textContent],
            [true, 'ed-foot-item ed-save ed-save--saved', false, true, true, '']);`,
`                cmdBtn(draft.el, 'test').getAttribute('title').includes('test_draft'), popEntry(draft, 'run-now') === null,
                pillText(draft)],
            [true, 'ed-foot-item ed-save ed-save--saved', false, true, true, 'pill_draft']);`);
edit('scripts/test-easydrag-extra5.mjs', `        const liveBtn = cmdBtn(on.editor.el, 'run-now');
        liveBtn.fire('click');
        await settle();
        eq('ux a published flow shows Run now and starts it',
            [liveBtn.hidden, liveBtn.getAttribute('aria-disabled') === 'true', liveBtn.getAttribute('title'), on.editor.el.querySelector('[data-ed-revision]').textContent,
                on.h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows/f1/run').length],
            [false, false, 'run_now', 'revision_live:1', 1]);`,
`        const liveBtn = popEntry(on.editor, 'run-now');
        liveBtn.fire('click');
        await settle();
        eq('ux a published flow shows Run now in the popover, starts it, and the pill says it is running',
            [!liveBtn, liveBtn.getAttribute('aria-disabled') === 'true', liveBtn.getAttribute('title'), pillText(on.editor),
                on.h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows/f1/run').length],
            [false, false, 'run_now', 'running', 1]);`);
edit('scripts/test-easydrag-extra5.mjs', `        const pausedBtn = cmdBtn(paused.editor.el, 'run-now');
        pausedBtn.fire('click');
        await settle();
        eq('ux a paused live flow keeps Run now visible and disabled with the reason',
            [pausedBtn.hidden,`, `        const pausedBtn = popEntry(paused.editor, 'run-now');
        pausedBtn.fire('click');
        await settle();
        eq('ux a paused live flow keeps Run now in the popover, disabled with the reason',
            [!pausedBtn,`);
edit('scripts/test-easydrag-extra5.mjs', `        const roBtn = cmdBtn(roEditor.el, 'run-now');
        eq('ux read-only still shows Run now, disabled, with its reason',
            [roBtn.hidden,`, `        const roBtn = popEntry(roEditor, 'run-now');
        eq('ux read-only still shows Run now in the popover, disabled, with its reason',
            [!roBtn,`);
edit('scripts/test-easydrag-extra5.mjs', `        eq('ux revision names a newer draft, and a partial publish keeps its chip',
            [ahead.el.querySelector('[data-ed-revision]').textContent, chips(ahead)],
            ['revision_ahead:2', ['state_publish_incomplete']]);`,
`        eq('ux the pill knows the draft is newer than live, and a partial publish says so',
            [ahead.el.querySelector('[data-ed-pill]').dataset.kind, chips(ahead)],
            ['newer', ['state_publish_incomplete']]);`);
edit('scripts/test-easydrag-extra5.mjs', `        const viewBtn = cmdBtn(ahead.el, 'run-now');
        eq('ux the run view keeps Run now visible and disabled',
            [viewBtn.hidden,`, `        const viewBtn = popEntry(ahead, 'run-now');
        eq('ux the run view keeps Run now in the popover, disabled',
            [!viewBtn,`);
edit('scripts/test-easydrag-extra5.mjs', `        const timedBtn = cmdBtn(ahead.el, 'run-now');`, `        const timedBtn = popEntry(ahead, 'run-now');`);

// ── extra6, extra7 ──
withImport('scripts/test-easydrag-extra6.mjs');
// The stub has no MutationObserver: a closed dialog repaints the popover here, as it does in the browser.
edit('scripts/test-easydrag-extra6.mjs', `        const runNow = () => editor.el.querySelector('[data-ed-cmd="run-now"]');`, `        const runNow = () => { editor.ed.bus.emit('enabled'); return popEntry(editor, 'run-now'); };`);
edit('scripts/test-easydrag-extra6.mjs', `        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click');`, `        popEntry(editor, 'runs').fire('click');`);
withImport('scripts/test-easydrag-extra7.mjs');
edit('scripts/test-easydrag-extra7.mjs', `            editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');`, `            popEntry(editor, 'run-now').fire('click');`);
edit('scripts/test-easydrag-extra7.mjs', `        await say('start a live run', () => s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click'));`, `        await say('start a live run', () => popEntry(s.editor, 'run-now').fire('click'));`);
edit('scripts/test-easydrag-extra7.mjs', `        eq('a1023b U1 the header hides the text of Stop, the Active switch and More, and spares four buttons',
            [spared, buttons.map(cmd)], [['publish', 'run-now', 'runs', 'test'], ['runs', 'test', 'cancel', 'publish', 'run-now', 'active', 'more']]);`,
`        eq('a1023b U1 the header hides the text of Stop and More, and spares the buttons the stylesheet names',
            [spared, buttons.map(cmd)], [['publish', 'run-now', 'runs', 'test'], ['test', 'cancel', 'publish', 'more']]);`);
edit('scripts/test-easydrag-extra7.mjs', `            buttons.filter(b => ['cancel', 'active'].includes(cmd(b))).map(b => [cmd(b), b.getAttribute('aria-label'), b.getAttribute('title')]),
            [['cancel', 'run_cancel', 'run_cancel'], ['active', 'active', 'active']]);`,
`            buttons.filter(b => ['cancel'].includes(cmd(b))).map(b => [cmd(b), b.getAttribute('aria-label'), b.getAttribute('title')]),
            [['cancel', 'run_cancel', 'run_cancel']]);`);

// ── flagship, live state, render baseline ──
edit('scripts/test-easydrag-flagship.mjs', `export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {`, IMPORT + `export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {`);
edit('scripts/test-easydrag-flagship.mjs', `        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click'); await settle();`, `        popEntry(editor, 'runs').fire('click'); await settle();`);
edit('scripts/test-easydrag-flagship.mjs', `        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click');
        await settle();`, `        popEntry(editor, 'runs').fire('click');
        await settle();`);
withImport('scripts/test-easydrag-live-state.mjs');
edit('scripts/test-easydrag-live-state.mjs', `        const chip = () => !!editor.el.querySelector('[data-ed-state] [data-ed-live-outdated]');`, `        const chip = () => pillKind(editor) === 'outdated';`);
edit('scripts/test-easydrag-live-state.mjs', `        const chip = () => !!editor.el.querySelector('[data-ed-state] [data-ed-timers-off]');`, `        const chip = () => pillKind(editor) === 'timers';`);
withImport('scripts/test-easydrag-render-baseline.mjs');
edit('scripts/test-easydrag-render-baseline.mjs', `            s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');`, `            popEntry(s.editor, 'run-now').fire('click');`);

// ── ux batch 5 ──
withImport('scripts/test-easydrag-ux-batch5.mjs');
edit('scripts/test-easydrag-ux-batch5.mjs', `        const said = await s.heard(() => s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click'));`, `        const said = await s.heard(() => popEntry(s.editor, 'run-now').fire('click'));`);
edit('scripts/test-easydrag-ux-batch5.mjs', `        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click');
        const drawerGroup`, `        popEntry(editor, 'runs').fire('click');
        const drawerGroup`);
edit('scripts/test-easydrag-ux-batch5.mjs', `            const state = s.editor.el.querySelector('[data-ed-state]').html;
            const rev = s.editor.el.querySelector('[data-ed-revisions]');
            const save = s.editor.el.querySelector('[data-ed-save]');
            return [(state.match(/state_run_view/g) || []).length + (rev.textContent === 'state_run_view' ? 1 : 0), !!rev.hidden, !!save.hidden];`,
`            const save = s.editor.el.querySelector('[data-ed-save]');
            return [pillText(s.editor) === 'state_run_view' ? 1 : 0, !!save.hidden];`);
edit('scripts/test-easydrag-ux-batch5.mjs', `        eq('b5 V1 draft: no Past run; run view: once, revision line and Saved hidden; back: as before',
            [before, inView, head(), s.h.logged], [[0, true, false], [1, true, true], [0, true, false], []]);`,
`        eq('b5 V1 draft: no Past run; run view: the pill says it and Saved is hidden; back: as before',
            [before, inView, head(), s.h.logged], [[0, false], [1, true], [0, false], []]);`);
edit('scripts/test-easydrag-ux-batch5.mjs', `        const paused = s.editor.el.querySelector('[data-ed-state]').html.includes('data-ed-paused');`, `        const paused = pillKind(s.editor) === 'paused';`);
edit('scripts/test-easydrag-ux-batch5.mjs', `                narrow.includes('.ed-state > :not([data-ed-paused], [data-ed-live-outdated], [data-ed-timers-off]) { display: none; }'), narrow.includes('.ed-state { display: none; }'), s.h.logged],
            [['run_now', 'versions_title'], ['runs_title', 'test_draft', '|', 'run_now'], true, 1, true, true, true, false, []]);`,
`                narrow.includes('.ed-state > :not('), narrow.includes('.ed-state { display: none; }'), s.h.logged],
            [['run_now', 'versions_title'], ['runs_title', 'test_draft', '|', 'run_now'], true, 1, true, true, false, false, []]);`);
const CHAIN = "{ enabled: true, flow: { draft: chain, live: chain, live_revision: 2, published_draft_revision: 3 } });";
edit('scripts/test-easydrag-ux-batch5.mjs', `            ${CHAIN}
        s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');`, `            ${CHAIN}
        popEntry(s.editor, 'run-now').fire('click');`);
edit('scripts/test-easydrag-ux-batch5.mjs', `        }, ${CHAIN}
        s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');`, `        }, ${CHAIN}
        popEntry(s.editor, 'run-now').fire('click');`);
edit('scripts/test-easydrag-ux-batch5.mjs', `        }, ${CHAIN}
        s2.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');`, `        }, ${CHAIN}
        popEntry(s2.editor, 'run-now').fire('click');`);
edit('scripts/test-easydrag-ux-batch5.mjs', `    await guardAsync('b5 phone the state chips Republish needed and the timers warning stay visible at 560 px', async () => {
        const sheet = css();
        const rule = /@container \\(max-width: 560px\\) \\{[\\s\\S]*?\\.ed-state > :not\\(([^)]*)\\) \\{ display: none; \\}/.exec(sheet);
        eq('b5 phone the hide rule excepts paused, live-outdated and timers-off', rule && rule[1], '[data-ed-paused], [data-ed-live-outdated], [data-ed-timers-off]');
    });`, `    await guardAsync('b5 phone the pill with Republish needed, the timers warning and Paused stays visible at 560 px', async () => {
        const sheet = css();
        const narrow = sheet.slice(sheet.indexOf('@container (max-width: 560px)')).split(/\\r?\\n\\}\\r?\\n/)[0];
        eq('b5 phone no 560 px rule hides the pill or its parent', /\\.ed-status-pill[^{]*\\{[^}]*display: none/.test(narrow) || /\\.ed-state[^{]*\\{[^}]*display: none/.test(narrow), false);
    });`);
console.log('b3-tests: done');
```

```js
// B.3: the Go browser tests that read the state chips or press Runs, Run now and Active move to the pill and its popover.
// Run from the worktree root, after b3-pill.mjs.
import { edit } from '../lib-patch.mjs';

const BROWSER = 'ui/desktop_easydrag_browser_test.go';
const RUNS = 'page.MustElement(`[data-ed-cmd="runs"]`).MustClick()';

// ── the journey (browser_test) ──
edit(BROWSER, `const rev=head.querySelector('[data-ed-revision]');`, `const rev=head.querySelector('[data-ed-pill-text]');`);
edit(BROWSER, `document.querySelector('[data-ed-state] .ed-chip').getBoundingClientRect()`, `document.querySelector('[data-ed-pill]').getBoundingClientRect()`);
edit(BROWSER, `!!document.querySelector('[data-ed-state] .ed-chip--ok')}`, `document.querySelector('[data-ed-pill]').getAttribute('data-status')==='ok'}`);
edit(BROWSER, `const chips=[...document.querySelectorAll('[data-ed-state] .ed-chip')].map(c=>c.textContent);return ed.flowEnabled===false && chips.includes(ed.t('easydrag.ui.state_inactive')) && document.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')==='false' &&`,
    `return ed.flowEnabled===false && document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='paused' &&`);
edit(BROWSER, `document.querySelectorAll('[data-ed-state] .ed-chip').length===1`, `document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='live'`);
edit(BROWSER, `!!document.querySelector('[data-ed-state] .ed-chip--warn')`, `document.querySelector('[data-ed-pill]').getAttribute('data-status')==='blocked'`);
edit(BROWSER, RUNS, `s.pillEntry("runs")`, 6);

// ── the other browser tests ──
edit('ui/desktop_easydrag_approval_browser_test.go', 'p.MustElement(`[data-ed-cmd="runs"]`).MustClick()', `s.pillEntry("runs")`);
edit('ui/desktop_easydrag_durable_browser_test.go', 'p.MustElement(`[data-ed-cmd="run-now"]`).MustClick()', `s.pillEntry("run-now")`);
edit('ui/desktop_easydrag_voice_browser_test.go', 'page.MustElement(`[data-ed-cmd="active"]`).MustClick()', `s.pillEntry("active")`);
edit('ui/desktop_easydrag_ux_browser_test.go', `[]string{"cancel", "active", "more"}`, `[]string{"cancel", "more"}`);
edit('ui/desktop_easydrag_ux_browser_test.go', `document.querySelector('[data-ed-state] [data-ed-paused]')`, `document.querySelector('[data-ed-pill][data-ed-pill-kind="paused"]')`, 2);
edit('ui/desktop_easydrag_blocks_browser_test.go', `!document.querySelector('[data-ed-cmd="active"]').getClientRects().length && !document.querySelector('[data-ed-cmd="runs"]').getClientRects().length`,
    `!document.querySelector('[data-ed-cmd="active"]') && !document.querySelector('[data-ed-cmd="runs"]')`);
edit('ui/desktop_easydrag_version_words_test.go', `"revision_live", "revision_ahead",`, `"revision_live", "revision_ahead", "pill_live", "pill_live_changed", "pill_paused",`);
console.log('b3-go: done');
```

```
cd W && node C:/disposable/easydrag-w1c/patches/b3-tests.mjs && node C:/disposable/easydrag-w1c/patches/b3-go.mjs
cd W && gofmt -l ui/desktop_easydrag_header_helpers_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_approval_browser_test.go ui/desktop_easydrag_durable_browser_test.go ui/desktop_easydrag_voice_browser_test.go ui/desktop_easydrag_ux_browser_test.go ui/desktop_easydrag_blocks_browser_test.go ui/desktop_easydrag_version_words_test.go
cd W && grep -rn 'data-ed-cmd="\(runs\|run-now\|active\)"\|data-ed-state\] \|data-ed-revision\b' scripts/test-easydrag*.mjs ui/desktop_easydrag_*_test.go | cut -c1-140
```
Expected: `b3-tests: done`, `b3-go: done`; `gofmt -l` prints nothing; the last `grep` finds only `ui/desktop_easydrag_browser_test.go:43`, `ui/desktop_easydrag_motion_browser_test.go:56` (`[data-ed-state]` text, still valid: the pill sits in it), `ui/desktop_easydrag_budget_browser_test.go:134` (B.6) and the absence assertions of the blocks tests.

- [ ] **Step 6: run everything**

```
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.3\|^FAIL\|all EasyDrag"
cd W && node scripts/test-easydrag-render-baseline.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
```
Expected: ten `ok   w1c B.3 ...` lines, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok` for the Go run (the key-literal test finds the nine new keys in 16 locales; the version-words lint keeps `{n}` in the three pill texts).

- [ ] **Step 7: commit**

Write `C:/disposable/easydrag-w1c/b3-msg.txt`:

```text
feat(easydrag): header pill with a popover for Active, next run, Run now and Runs

W1c Part B.3. One pill (status disc and one text, fixed order: a run, a past
run, a problem, a pause, the version) replaces the state chips and the
revision line; its popover (ED.kit.popover) holds the Active switch, the next
run (ED.format.nextRun, read again when it opens), Run now, Runs and the
action that finishes a partial publish. Every entry runs through the action
registry, so a refused entry shows its reason. Runs, Run now and the Active
switch leave the header; the checks that read the chips or pressed them move
to the pill and the popover. Nine pill_* keys in 16 locales.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-pill.js ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-format.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/css/desktop-app-easydrag.css ui/lang/easydrag ui/desktop_easydrag_header_helpers_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_approval_browser_test.go ui/desktop_easydrag_durable_browser_test.go ui/desktop_easydrag_voice_browser_test.go ui/desktop_easydrag_ux_browser_test.go ui/desktop_easydrag_blocks_browser_test.go ui/desktop_easydrag_version_words_test.go scripts/easydrag-header-test-helper.mjs scripts/test-easydrag-header.mjs scripts/test-easydrag-blocks.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra5.mjs scripts/test-easydrag-extra6.mjs scripts/test-easydrag-extra7.mjs scripts/test-easydrag-flagship.mjs scripts/test-easydrag-live-state.mjs scripts/test-easydrag-render-baseline.mjs scripts/test-easydrag-ux-batch5.mjs && git commit -F C:/disposable/easydrag-w1c/b3-msg.txt && git log --oneline -1
```

---

### Task B.4: The primary button and the secondary Test

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-header.js` (`paintPrimary`), `ui/css/desktop-app-easydrag.css` (the change dot goes), `ui/lang/easydrag/*.json` (`publish_with_changes`), `ui/desktop_easydrag_plural_test.go`, the Go browser tests that publish
- Modify: `ui/desktop_easydrag_header_helpers_test.go` (`markTested`)
- Test: `scripts/test-easydrag-header.mjs`

**Key** (key file `b-primary.json`, plural forms: `locale-apply-forms.mjs`, because the shared helper cannot add `_few` for cs and pl only):

| Key | en | de |
|---|---|---|
| `publish_with_changes` (other) | Publish · {count} changes | Veröffentlichen · {count} Änderungen |
| `publish_with_changes_one` | Publish · {count} change | Veröffentlichen · {count} Änderung |

cs and pl also have `_few`, pl `_many`; ja and zh carry `_one` equal to the bare text (the plural test asks for it).

- [ ] **Step 1: add the failing checks** (edit group `b4-test`):

<!-- edits:b4-test -->
**Edit** `scripts/test-easydrag-header.mjs`

Old:
```text
    // @@APPEND@@
```

New:
```text
    // ── B.4: the primary button ──

    // primaryOf opens f1 with the given facts and says [which button is filled, the Publish label, whether Publish shows].
    const primaryRow = async (opts) => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h, opts);
        await settle();
        const filled = editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd'));
        const test = editor.el.querySelector('[data-ed-cmd="test"]'), pub = editor.el.querySelector('[data-ed-cmd="publish"]');
        const row = [filled, pub.hidden === true, pub.querySelector('span').textContent, test.className.includes('ed-btn--primary'), pub.className.includes('ed-btn--primary'), h.logged];
        return { h, editor, row };
    };
    const live = { live: flowDoc(), live_revision: 2, published_draft_revision: 2 };

    await guardAsync('w1c B.4 the primary button follows the states of the table: Test, Publish with its count, or none', async () => {
        const rows = [];
        const cases = [
            ['never published, untested', {}],
            ['never published, tested', { facts: { tested: true } }],
            ['changed, untested', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live) }],
            ['changed, tested, three changes', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true, changes: { steps: 2, connections: 1 } } }],
            ['changed, tested, one change', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true, changes: { steps: 1, connections: 0 } } }],
            ['changed, tested, no count', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true } }],
            ['clean live', { enabled: true, flow: Object.assign({ draft_revision: 2 }, live), facts: { tested: true } }],
            ['read-only', { enabled: true, readonly: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true } }]
        ];
        for (const [name, opts] of cases) {
            const r = await primaryRow(opts);
            rows.push([name, r.row]);
            r.editor.dispose();
        }
        eq('w1c B.4 primary rows [filled, Publish hidden, Publish text, Test filled, Publish filled, log]', rows, [
            ['never published, untested', [['test'], true, 'publish', true, false, []]],
            ['never published, tested', [['publish'], false, 'publish', false, true, []]],
            ['changed, untested', [['test'], true, 'publish', true, false, []]],
            ['changed, tested, three changes', [['publish'], false, 'publish_with_changes:3', false, true, []]],
            ['changed, tested, one change', [['publish'], false, 'publish_with_changes_one:1', false, true, []]],
            ['changed, tested, no count', [['publish'], false, 'publish', false, true, []]],
            ['clean live', [[], true, 'publish', false, false, []]],
            ['read-only', [[], true, 'publish', false, false, []]]
        ]);
    });

    await guardAsync('w1c B.4 a failed test leaves Test primary; a passed one, an edit, a save and a restore move the button', async () => {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [], tested: false, changes: { steps: 1, connections: 0 } } : undefined));
        const editor = openEditor(h, { enabled: true, flow: Object.assign({ draft_revision: 3 }, live) });
        await settle();
        const filled = () => editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd')).join();
        const rows = [filled()];
        // The editor's own evidence: a failed run is no test, a successful one of this revision is.
        editor.ed.testEvidence = { revision: 3, status: 'error' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled());
        editor.ed.testEvidence = { revision: 3, status: 'success' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled());
        // An edit makes the draft untested at once; its save brings revision 4, which the evidence does not cover.
        editor.ed.model.setFlow({ description: 'changed' });
        rows.push(filled());
        h.runTimers(1000);
        await settle();
        rows.push(filled(), editor.el.querySelector('[data-ed-cmd="publish"]').hidden);
        // Evidence for revision 4 brings Publish with the count of the save answer.
        editor.ed.testEvidence = { revision: 4, status: 'success' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled(), editor.el.querySelector('[data-ed-cmd="publish"]').querySelector('span').textContent);
        editor.ed.bus.emit('restored', { flow: Object.assign({ id: 'f1', draft: flowDoc(), draft_revision: 9, published_draft_revision: 2 }, live), issues: [] });
        await settle();
        rows.push(filled());
        eq('w1c B.4 primary follows evidence, edits, saves and a restore', rows, ['test', 'test', 'publish', 'test', 'test', true, 'publish', 'publish_with_changes_one:1', 'test']);
        eq('w1c B.4 no log', h.logged, []);
        editor.dispose();
    });

    await guardAsync('w1c B.4 a partial publish offers Publish again, a past run offers nothing, Test stays beside the primary and says why when it cannot run', async () => {
        const { h, editor } = await primaryRow({ enabled: true, flow: Object.assign({ draft_revision: 2 }, live) });
        const filled = () => editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd')).join();
        const rows = [filled()];
        Object.assign(editor.ed, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        editor.ed.bus.emit('enabled');
        rows.push(filled());
        editor.ed.runView = { run: { id: 'r9', status: 'success', revision: 2 } };
        editor.ed.bus.emit('enabled');
        const test = editor.el.querySelector('[data-ed-cmd="test"]');
        rows.push(filled(), test.getAttribute('aria-disabled'), test.getAttribute('aria-description'));
        eq('w1c B.4 partial publish, past run', rows, ['', 'publish', '', 'true', 'reason_run_view']);
        editor.dispose();
    });

    // @@APPEND@@
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b4-test
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.4\|^FAIL"
```
Expected: `FAIL w1c B.4 ...` lines (no `[data-ed-primary]` exists yet).

- [ ] **Step 2: key, scripts, test helper**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply-forms.mjs plans/2026-11-16-easydrag-w1c/keys/b-primary.json
```
Expected: `...b-primary.json: 1 count keys, 0 deleted, in 16 files of ui/lang/easydrag/`.

Write `C:/disposable/easydrag-w1c/patches/b4-primary.mjs` and `b4-go.mjs`:

```js
// B.4: the primary button (Test, or Publish with its change count, or none) and the secondary Test. Run from the worktree root.
import { edit, replaceBlock } from '../lib-patch.mjs';

const HEADER = 'ui/js/desktop/apps/easydrag-header.js';
const CSS = 'ui/css/desktop-app-easydrag.css';

// ── header.js: paintPrimary replaces the chip-era class toggles ──
edit(HEADER, `        function renderHeader() {
            // A save answered after dispose must not point the window context at this flow again.`,
`        // paintPrimary draws the one filled button (easydrag-pill.js, primaryOf): Test while the draft is not tested, Publish with the
        // number of changes once it is, none for a clean live flow. Both buttons keep their place in the row; the one that is not
        // wanted is hidden, and only the filled one carries data-ed-primary.
        function paintPrimary() {
            const p = ED.pill.primaryOf(ed);
            const test = el.querySelector('[data-ed-cmd="test"]');
            const pub = el.querySelector('[data-ed-cmd="publish"]');
            test.classList.toggle('ed-btn--primary', p.kind === 'test');
            test.toggleAttribute('data-ed-primary', p.kind === 'test');
            pub.hidden = p.kind !== 'publish';
            pub.classList.toggle('ed-btn--primary', p.kind === 'publish');
            pub.toggleAttribute('data-ed-primary', p.kind === 'publish');
            const text = p.count ? ED.format.plural(t, 'easydrag.ui.publish_with_changes', p.count) : ED.actions.label('flow.publish', { host: ed });
            const span = pub.querySelector('span');
            if (span && span.textContent !== text) span.textContent = text;
        }

        function renderHeader() {
            // A save answered after dispose must not point the window context at this flow again.`);
edit(HEADER, `            const ro = !!(ed.readonly || ed.runView);
            const running = runs.isRunning();`, `            const running = runs.isRunning();`);
edit(HEADER, `            btn('test').classList.toggle('is-busy', running);
            btn('test').classList.toggle('ed-btn--primary', !ed.flow.live && !ro);
            btn('cancel').hidden = !running;
            btn('publish').classList.toggle('has-changes', (!ed.flow.live || ED.pill.changed(ed) || !!ed.publishIncomplete) && !ro);
            btn('publish').classList.toggle('ed-btn--primary', !!ed.flow.live && !ro);
            paintActions();`, `            btn('test').classList.toggle('is-busy', running);
            btn('cancel').hidden = !running;
            paintPrimary();
            paintActions();`);
edit(HEADER, `            if (!host.isDisposed()) pill.paint();
`, `            if (!host.isDisposed()) { paintPrimary(); pill.paint(); }
`);

// ── CSS: the change dot on Publish is gone, the count is in its label ──
edit(CSS, `.ed-btn--primary.has-changes { position: relative; }
.ed-btn--primary.has-changes::after {
    content: ""; position: absolute; top: -3px; right: -3px; width: 9px; height: 9px; border-radius: 50%;
    background: var(--ed-change); box-shadow: 0 0 0 2px var(--ed-chrome);
}
`, ``);
edit(CSS, `    .ed-btn--primary.has-changes::after, .ed-badge--issue, .ed-guide { background-color: Highlight; }`, `    .ed-badge--issue, .ed-guide { background-color: Highlight; }`);

// ── plural lint: the new count text has the plural forms of every locale ──
edit('ui/desktop_easydrag_plural_test.go', `"group_success", "group_disabled", "cred_purge_versions", "cred_purge_pinned", "versions_connections",`,
    `"group_success", "group_disabled", "cred_purge_versions", "cred_purge_pinned", "versions_connections", "publish_with_changes",`);
console.log('b4-primary: done');
```

```js
// B.4: the Go browser tests that publish now say the draft is tested first, and the journey knows that a clean live flow has no
// Publish button. Run from the worktree root, after b4-primary.mjs.
import { edit } from '../lib-patch.mjs';

const PUBLISH = 'p.MustElement(`[data-ed-cmd="publish"]`).MustClick()';

edit('ui/desktop_easydrag_browser_test.go', `!!publish && !publish.hidden && publish.textContent.trim().length>0 && !!rev`, `!!publish && publish.hidden && !!rev`);
edit('ui/desktop_easydrag_browser_test.go', `!pub.disabled && pub.classList.contains('has-changes')`, `!pub.disabled && !pub.hidden`);
edit('ui/desktop_easydrag_browser_test.go', '\tpage.MustElement(`[data-ed-cmd="publish"]`).MustClick()\n\ts.wait(`()=>!!document.querySelector(\'.ed-modal [data-ed-action="publish"]\')`)\n\ts.shot("publish-dialog")',
    '\ts.markTested()\n\tpage.MustElement(`[data-ed-cmd="publish"]`).MustClick()\n\ts.wait(`()=>!!document.querySelector(\'.ed-modal [data-ed-action="publish"]\')`)\n\ts.shot("publish-dialog")');
edit('ui/desktop_easydrag_blocks_browser_test.go', '\tpublish := func(block bool) {\n\t\tt.Helper()\n\t\t' + PUBLISH, '\tpublish := func(block bool) {\n\t\tt.Helper()\n\t\ts.markTested()\n\t\t' + PUBLISH);
edit('ui/desktop_easydrag_credentials_browser_test.go', '\t// The next publish offers the guided purge, once.\n\t' + PUBLISH, '\t// The next publish offers the guided purge, once.\n\ts.markTested()\n\t' + PUBLISH);
edit('ui/desktop_easydrag_metadata_browser_test.go', "edMetadataFixture.state.partialNext=true;}`)\n\t" + PUBLISH, "edMetadataFixture.state.partialNext=true;}`)\n\ts.markTested()\n\t" + PUBLISH);
edit('ui/desktop_easydrag_metadata_browser_test.go', '\t' + PUBLISH + '\n\ts.wait(`()=>!!document.querySelector(\'[data-ed-publish-note]\')`)\n\tp.MustElement(`[data-ed-publish-note]`).MustInput("Block note")',
    '\ts.markTested()\n\t' + PUBLISH + '\n\ts.wait(`()=>!!document.querySelector(\'[data-ed-publish-note]\')`)\n\tp.MustElement(`[data-ed-publish-note]`).MustInput("Block note")');
console.log('b4-go: done');
```

Append to `ui/desktop_easydrag_header_helpers_test.go` (edit group `b4-helper`):

<!-- edits:b4-helper -->
**Append** `ui/desktop_easydrag_header_helpers_test.go`

```text

// markTested says that the open flow's draft passed a test, as the server would after one: the fixture answers tested=true from now
// on (flows), the editor takes the flag at once (flows and blocks), and the header repaints. The primary button then offers Publish.
func (s *easyDragSmoke) markTested() {
	s.t.Helper()
	s.page.MustEval(`()=>{edFixture.state.tested=true;const ed=edFixture.editor();if(!ed)return;ed.flow.tested=true;ed.flowFactsRevision=ed.saver.revision;ed.bus.emit('flow-facts',ed.flow)}`)
}
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b4-helper
cd W && node C:/disposable/easydrag-w1c/patches/b4-primary.mjs && node C:/disposable/easydrag-w1c/patches/b4-go.mjs
cd W && gofmt -l ui/desktop_easydrag_header_helpers_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_blocks_browser_test.go ui/desktop_easydrag_credentials_browser_test.go ui/desktop_easydrag_metadata_browser_test.go ui/desktop_easydrag_plural_test.go
cd W && grep -rn "has-changes" ui/css ui/js scripts ui/*_test.go | head
```
Expected: both `done`; `gofmt -l` prints nothing; the `grep` finds nothing.

What the Go sites do: the browser tests that click Publish now say the draft is tested first (`s.markTested()`), as a user would after a test; they are the journey (after its test run), the blocks journey (the `publish` closure), the credentials move, and the metadata test (twice). A clean live flow has no Publish button, so the journey's header wait says `publish.hidden`; the partial-publish check says `!pub.hidden`.

- [ ] **Step 3: run everything**

```
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.4\|^FAIL\|all EasyDrag"
cd W && node scripts/test-easydrag-render-baseline.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
```
Expected: four `ok   w1c B.4 ...` lines, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, Go `ok` (the plural lint finds `publish_with_changes` with the forms of every locale).

- [ ] **Step 4: commit**

Write `C:/disposable/easydrag-w1c/b4-msg.txt`:

```text
feat(easydrag): one filled button for the next step

W1c Part B.4. Test while the draft is not tested, "Publish · N changes"
(plural forms, steps plus connections) once the server says it is, nothing
for a clean live flow, a read-only desktop and a past run; a partial publish
still offers Publish. Test stays beside the primary. Only one button carries
data-ed-primary. The change dot on Publish goes: the count is in the label.
The browser tests that publish say the draft is tested first.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-header.js ui/css/desktop-app-easydrag.css ui/lang/easydrag ui/desktop_easydrag_plural_test.go ui/desktop_easydrag_header_helpers_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_blocks_browser_test.go ui/desktop_easydrag_credentials_browser_test.go ui/desktop_easydrag_metadata_browser_test.go scripts/test-easydrag-header.mjs && git commit -F C:/disposable/easydrag-w1c/b4-msg.txt && git log --oneline -1
```

---

### Task B.5: The header row, the Stop slot and the narrow layouts

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (markup order, the narrow ⋯ list goes), `ui/css/desktop-app-easydrag.css`, `scripts/test-easydrag-ux-batch5.mjs`, `scripts/test-easydrag-extra7.mjs`
- Test: `scripts/test-easydrag-header.mjs`

- [ ] **Step 1: add the failing checks** (edit group `b5-test`):

<!-- edits:b5-test -->
**Edit** `scripts/test-easydrag-header.mjs`

Old:
```text
    // @@APPEND@@
```

New:
```text
    // ── B.5: the header row ──

    await guardAsync('w1c B.5 the header row: Back, Name and Pill on the left; Stop slot, Test, primary, Undo and Redo, More on the right', async () => {
        const h = sandbox(req => (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test' ? { run_id: 'r1' } : undefined));
        const editor = openEditor(h);
        await settle();
        const head = editor.el.querySelector('.ed-head');
        const kind = c => (c.hasAttribute('data-ed-stop-slot') ? 'stop-slot' : c.hasAttribute('data-ed-history-host') ? 'history' : c.getAttribute('data-ed-cmd') || c.className);
        const row = () => Array.from(editor.el.querySelector('.ed-head-actions').children).map(kind);
        const title = editor.el.querySelector('.ed-title');
        const idle = row();
        const slot = editor.el.querySelector('[data-ed-stop-slot]');
        const stop = slot.querySelector('[data-ed-cmd="cancel"]');
        const idleStop = [stop.hidden, editor.el.querySelector('.ed-head-actions').children[0] === slot];
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        if (dialog) { const run = dialog.el.querySelector('[data-ed-action="run"]'); if (run) { run.fire('click'); await settle(); } }
        const running = [stop.hidden, row(), stop.parentNode === slot];
        eq('w1c B.5 header row order and the Stop slot (idle and while a test runs)', [
            Array.from(head.children).map(c => c.className), title.querySelector('[data-ed-pill]') !== null, title.querySelector('[data-ed-name]') !== null,
            idle, idleStop, running, h.logged], [
            ['ed-icon-btn ed-back', 'ed-title', 'ed-head-actions'], true, true,
            ['stop-slot', 'test', 'publish', 'history', 'more'], [true, true],
            [false, ['stop-slot', 'test', 'publish', 'history', 'more'], true], []]);
        editor.dispose();
    });

    await guardAsync('w1c B.5 the stylesheet reserves the slot, keeps the Stop label at 900 px and names no removed button', async () => {
        const css = fs.readFileSync(path.join(apps, '..', '..', '..', 'css', 'desktop-app-easydrag.css'), 'utf8');
        const slot = /\.ed-stop-slot \{([^}]*)\}/.exec(css);
        const at900 = css.slice(css.indexOf('@container (max-width: 900px)')).split(/\r?\n\}\r?\n/)[0];
        eq('w1c B.5 stylesheet', [!!slot && /flex: none/.test(slot[1]) && /width: var\(--ed-stop-slot-w, 9\.5rem\)/.test(slot[1]),
            /\[data-ed-cmd="cancel"\] > span \{ display: inline/.test(at900), /data-ed-cmd="(runs|run-now|active)"/.test(css), /\.ed-head-history \{[^}]*flex: none/.test(css)], [true, true, false, true]);
    });

    // @@APPEND@@
```
<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-b-header.md b5-test
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.5\|^FAIL"
```
Expected: two `FAIL w1c B.5 ...` lines (the row still has the old order, the stylesheet has no `.ed-stop-slot`).

- [ ] **Step 2: write and run the scripts**

Write `C:/disposable/easydrag-w1c/patches/b5-layout.mjs` and `b5-tests.mjs`:

```js
// B.5: the new header row: Back, Name and Pill on the left; the Stop slot, Test, the primary button, Undo, Redo and More in a right-aligned
// group. Run from the worktree root.
import { edit } from '../lib-patch.mjs';

const EDITOR = 'ui/js/desktop/apps/easydrag-editor.js';
const CSS = 'ui/css/desktop-app-easydrag.css';

// ── editor.js: the markup ──
edit(EDITOR, `            '<div class="ed-head-history" role="group" data-ed-history-host></div>' +
            '<div class="ed-head-actions">' +
`, `            '<div class="ed-head-actions">' +
`);
edit(EDITOR, `            '<button type="button" class="ed-btn" data-ed-cmd="test">' + ED.icons.of('test') + '<span>' + lab('flow.test') + '</span></button>' +
            // Below 900 px the text of Stop and of the Active switch (below) is hidden: the name and the tooltip say the same words.
            '<button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" aria-label="' + lab('flow.stop') + '" hidden>' + ED.icons.of('stop') + '<span>' + lab('flow.stop') + '</span></button>' +
`, `            // The Stop sits in a slot of fixed width: the controls beside it keep their place whether a run is going or not.
            '<span class="ed-stop-slot" data-ed-stop-slot><button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" aria-label="' + lab('flow.stop') + '" hidden>' + ED.icons.of('stop') + '<span>' + lab('flow.stop') + '</span></button></span>' +
            '<button type="button" class="ed-btn" data-ed-cmd="test">' + ED.icons.of('test') + '<span>' + lab('flow.test') + '</span></button>' +
`);
edit(EDITOR, `data-ed-cmd="publish">' + ED.icons.of('publish')`, `data-ed-cmd="publish" hidden>' + ED.icons.of('publish')`);
edit(EDITOR, `            '<button type="button" class="ed-icon-btn" data-ed-cmd="more"`, `            '<div class="ed-head-history" role="group" data-ed-history-host></div>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="more"`);

// ── editor.js: the narrow ⋯ menu no longer carries Runs and Test (Test stays in the header, Runs is in the pill popover) ──
edit(EDITOR, `        // headerNarrow: the editor is 560 px wide or narrower, where the stylesheet hides Runs, Test and
        // Run now in the header (@container max-width 560px) and the ⋯ menu offers them.
        function headerNarrow() {
            const width = el.getBoundingClientRect().width;
            return width > 0 && width <= 560;
        }

`, ``);
edit(EDITOR, `            const moved = !isBlock && headerNarrow() ? ['view.runs', 'flow.test', '-'] : [];
            ctx.showContextMenu(r.left, r.bottom + 4, ED.actions.items(isBlock ? BLOCK_MORE_MENU : moved.concat(MORE_MENU), { host: ed }));`,
`            ctx.showContextMenu(r.left, r.bottom + 4, ED.actions.items(isBlock ? BLOCK_MORE_MENU : MORE_MENU, { host: ed }));`);

// ── CSS ──
edit(CSS, `.ed-head-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; flex: 1 1 100%; justify-content: flex-start; min-width: 0; }
.ed-head-actions .ed-btn { white-space: normal; }`, `/* the right-hand group: Stop slot, Test, the primary button, Undo, Redo, More. It sits at the right end of the row, and below it when the row is full. */
.ed-head-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; flex: 0 1 auto; margin-left: auto; justify-content: flex-end; min-width: 0; }
.ed-head-actions .ed-btn { white-space: normal; }
/* the Stop slot keeps its width whether a run is going or not, so no control beside it moves (the Stop itself fills it) */
.ed-stop-slot { display: inline-flex; align-items: center; justify-content: flex-end; flex: none; width: var(--ed-stop-slot-w, 9.5rem); }
.ed-stop-slot > .ed-btn { width: 100%; min-width: 0; }`);
edit(CSS, `    .ed-head-actions .ed-btn:not([data-ed-cmd="test"]):not([data-ed-cmd="publish"]):not([data-ed-cmd="runs"]):not([data-ed-cmd="run-now"]) > span,
    .ed-switch-btn > span:last-child { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .ed-head-actions .ed-btn[data-ed-cmd="test"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="publish"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="runs"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="run-now"] > span { display: inline; white-space: normal; }
    .ed-head-actions .ed-btn, .ed-head-actions .ed-icon-btn, .ed-head-actions .ed-switch-btn { min-height: 44px; min-width: 44px; }`,
`    .ed-head-actions .ed-btn:not([data-ed-cmd="test"]):not([data-ed-cmd="publish"]):not([data-ed-cmd="cancel"]) > span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .ed-head-actions .ed-btn[data-ed-cmd="test"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="publish"] > span,
    .ed-head-actions .ed-btn[data-ed-cmd="cancel"] > span { display: inline; white-space: normal; }
    .ed-head-actions { flex: 1 1 100%; }
    .ed-head-actions .ed-btn, .ed-head-actions .ed-icon-btn, .ed-status-pill { min-height: 44px; min-width: 44px; }`);
edit(CSS, `    /* Keep the first-test path visible; history and live runs remain in the More menu. */
    .ed-head-actions [data-ed-cmd="runs"], .ed-head-actions [data-ed-cmd="run-now"] { display: none; }
`, `    /* Test and the primary button share the width; the Stop slot and Undo, Redo and More keep their size. */
`);
console.log('b5-layout: done');
```

```js
// B.5: the Node checks that pinned the old header row (the phone's ⋯ menu with Runs and Test, the stylesheet rules that hid Runs and Run now,
// the buttons of .ed-head-actions) follow the new row. Run from the worktree root, after b5-layout.mjs.
import { edit } from '../lib-patch.mjs';

const UX = 'scripts/test-easydrag-ux-batch5.mjs';

edit(UX, `    await guardAsync('b5 V4 U13 a phone header keeps Test visible and offers history in More', async () => {`,
`    await guardAsync('b5 V4 U13 a phone header keeps Test visible, Runs is in the pill popover and More does not grow', async () => {`);
edit(UX, `        items.find(i => i.label === 'runs_title').action();
        const drawer = !!s.editor.el.querySelector('.ed-drawer');
        const dialogs = s.h.dialogs.length;
        await items.find(i => i.label === 'test_draft').action();
        await settle();`, `        popEntry(s.editor, 'runs').fire('click');
        const drawer = !!s.editor.el.querySelector('.ed-drawer');
        const dialogs = s.h.dialogs.length;
        s.editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();`);
edit(UX, `        eq('b5 V4 U13 phone keeps Test visible, both menu shortcuts work and Paused stays visible',
            [wide.slice(0, 2), phone.slice(0, 4), drawer, testOpened, paused,
                narrow.includes('.ed-head-actions [data-ed-cmd="runs"], .ed-head-actions [data-ed-cmd="run-now"] { display: none; }') && !narrow.includes('[data-ed-cmd="test"] { display: none; }'),
                narrow.includes('.ed-state > :not('), narrow.includes('.ed-state { display: none; }'), s.h.logged],
            [['run_now', 'versions_title'], ['runs_title', 'test_draft', '|', 'run_now'], true, 1, true, true, false, false, []]);`,
`        eq('b5 V4 U13 phone: More is the same as wide, Test and the popover entry Runs work and Paused stays visible',
            [wide.slice(0, 2), phone.slice(0, 2), phone.includes('runs_title') || phone.includes('test_draft'), drawer, testOpened, paused,
                narrow.includes('[data-ed-cmd="runs"]') || narrow.includes('[data-ed-cmd="run-now"]') || narrow.includes('[data-ed-cmd="test"] { display: none; }'),
                narrow.includes('.ed-state > :not('), narrow.includes('.ed-state { display: none; }'), s.h.logged],
            [['run_now', 'versions_title'], ['run_now', 'versions_title'], false, true, 1, true, false, false, false, []]);`);

// the stylesheet spares Test, Publish and Stop at 900 px; the row holds the Stop slot, Test, Publish, the history group and More
edit('scripts/test-easydrag-extra7.mjs', `        eq('a1023b U1 the header hides the text of Stop and More, and spares the buttons the stylesheet names',
            [spared, buttons.map(cmd)], [['publish', 'run-now', 'runs', 'test'], ['test', 'cancel', 'publish', 'more']]);`,
`        eq('a1023b U1 the header hides the text of Undo, Redo and More, and spares Stop, Test and Publish',
            [spared, buttons.map(cmd)], [['cancel', 'publish', 'test'], ['cancel', 'test', 'publish', undefined, undefined, 'more']]);`);
console.log('b5-tests: done');
```

```
cd W && node C:/disposable/easydrag-w1c/patches/b5-layout.mjs && node C:/disposable/easydrag-w1c/patches/b5-tests.mjs
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js && grep -n "headerNarrow\|moved.concat\|data-ed-cmd=\"runs\"\|data-ed-cmd=\"run-now\"" ui/js/desktop/apps/easydrag-editor.js ui/css/desktop-app-easydrag.css
```
Expected: both `done`; the editor has 847 lines (rule 896); the `grep` finds nothing. What changed in the layout: the row is Back, name and pill on the left and a right-aligned group (Stop slot, Test, Publish, Undo and Redo, ⋯) that wraps below the left part when the row is full and takes the whole width below 900 px (container query); below 900 px the text of every button but Test, Publish and Stop is hidden (the Stop label stays, as the spec says); the 560 px rule that hid Runs and Run now is gone, and so is the narrow ⋯ list (`headerNarrow`): Runs is in the pill popover at every width, Test stays visible, and the ⋯ menu does not grow. The Stop slot is a box of `var(--ed-stop-slot-w, 9.5rem)`; the Stop button keeps `hidden` while idle and fills the slot while a run goes.

The checks that pinned the old row: `scripts/test-easydrag-extra7.mjs` (`a1023b U1`: which buttons the 900 px rule spares and which buttons the group holds) and `scripts/test-easydrag-ux-batch5.mjs` (`b5 V4 U13`: the phone's ⋯ menu is the wide one, Runs through the popover, Test still opens its dialog, no 560 px rule names a removed button).

- [ ] **Step 3: run everything**

```
cd W && node scripts/test-easydrag.mjs 2>&1 | grep "w1c B.5\|^FAIL\|all EasyDrag"
cd W && node scripts/test-easydrag-render-baseline.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
```
Expected: two `ok   w1c B.5 ...` lines, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, Go `ok` (`TestEasyDragSplitSeams` still finds `ED.header.create({` in the editor and the six header functions in `easydrag-header.js`).

- [ ] **Step 4: look at it** (not covered by a test; the browser run of B.6 measures the widths)

Run the preview harness of `memory/reference_desktop_preview_harness.md` or the browser smoke with `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots` and open `editor-standard.png` at 1440, 900 and 390 px. Check that the pill's text is cut with an ellipsis, not the name; that nothing on the left moves the right group; and that no control overlaps at 390 px (the group takes two rows there: the reserved slot is wide; Part C measures it with the real label).

- [ ] **Step 5: commit**

Write `C:/disposable/easydrag-w1c/b5-msg.txt`:

```text
feat(easydrag): the new header row with a reserved Stop slot

W1c Part B.5. Back, name and pill on the left; Stop slot, Test, the primary
button, Undo, Redo and the more menu in a right-aligned group. The Stop sits
in a slot of fixed width and keeps its behaviour (shown while a run goes), so
the controls beside it do not move; its label stays at every width. The 560
px rule that hid Runs and Run now and the narrow additions to the more menu
go: Runs is in the pill popover, Test stays visible.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-editor.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-ux-batch5.mjs scripts/test-easydrag-extra7.mjs scripts/test-easydrag-header.mjs && git commit -F C:/disposable/easydrag-w1c/b5-msg.txt && git log --oneline -1
```

---

### Task B.6: The budget, the browser tests and the notes

**Files:**
- Create: `ui/desktop_easydrag_header_browser_test.go`
- Modify: `ui/desktop_easydrag_budget_browser_test.go:29,134`, `ui/js/desktop/apps/AGENTS.md`

- [ ] **Step 1: the budget and the notes**

Write `C:/disposable/easydrag-w1c/patches/b6-budget.mjs`:

```js
// B.6: the header budget is enforced at 8, and the notes of the editor follow the new header. Run from the worktree root.
import { edit } from '../lib-patch.mjs';

const BUDGET = 'ui/desktop_easydrag_budget_browser_test.go';

edit(BUDGET, `	// The editor header (.ed-head). W1c rebuilds it and must reach 8.
	{Region: "header", Ceiling: 10, Target: 8, Gate: "W1c"},`, `	// The editor header (.ed-head): Back, name, pill, Test, Undo, Redo and More, and the primary Publish button while there are changes
	// to publish (W1c). Enforced.
	{Region: "header", Ceiling: 8, Target: 8, Gate: "W1c", Enforced: true},`);
edit(BUDGET, `	// The editor of that flow: published, idle and switched on, so Run now is shown.`, `	// The editor of that flow: published, idle and switched on; the pill says so.`);
edit(BUDGET, `const run=document.querySelector('[data-ed-cmd="run-now"]');return !!ed && ed.flowEnabled===true && !!run && !run.hidden}`, `const pill=document.querySelector('[data-ed-pill]');return !!ed && ed.flowEnabled===true && !!pill && pill.getAttribute('data-ed-pill-kind')==='live'}`);

edit('ui/js/desktop/apps/AGENTS.md', `- At an editor width of 560 px or less (\`@container\`), Runs and Run now are hidden in the header, Test stays visible, and \`moreMenu\` lists Runs and Test first (\`headerNarrow\`). The state chip goes, but the Paused chip (\`[data-ed-paused]\`), "Republish needed" (\`[data-ed-live-outdated]\`) and the timers warning (\`[data-ed-timers-off]\`) stay: the CSS exception lists all three.`,
`- The header row (W1c): Back, the name and the pill (\`[data-ed-pill]\`, easydrag-pill.js) on the left; on the right a group of the Stop slot (\`[data-ed-stop-slot]\`, fixed width), Test, the primary button (\`[data-ed-primary]\`: Test, or Publish with the number of changes, or none for a clean live flow), Undo, Redo and ⋯ (8 controls at most; \`TestEasyDragControlBudgetsBrowser\` enforces it). The pill says one thing in a fixed order (a run, a past run, a problem, a pause, the version) and stays at every width, cutting its text with an ellipsis. Its popover (\`ED.kit.popover\`) holds Active, the next run, Run now, Runs and the action that finishes a partial publish; its entries are \`[data-ed-pop]\` and run through the action registry. The server's facts (\`next_run\`, \`tested\`, \`changes\`) sit on \`ed.flow\` (easydrag-flow-state.js): taken from the GET, from every save answer and from every re-read; the popover reads the flow again when it opens.`);
console.log('b6-budget: done');
```

```
cd W && node C:/disposable/easydrag-w1c/patches/b6-budget.mjs && gofmt -l ui/desktop_easydrag_budget_browser_test.go
```
Expected: `b6-budget: done`, no gofmt output. The header row is `Enforced: true` with ceiling = target = 8 (`TestEasyDragControlBudgetTable` accepts an enforced row only when ceiling equals target). The seeded flow is a clean live flow, so the header counts 7 controls (Back, name, pill, Test, Undo, Redo, More); the eighth is Publish while there are changes to publish. The test logs `BUDGET region=header ... lower the ceiling to 7`; the ceiling stays 8 on purpose (the owner's budget is 8 with Publish). The old wait for the Run now button becomes a wait for the pill.

- [ ] **Step 2: the browser tests**

Write `ui/desktop_easydrag_header_browser_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// openSeededEditor opens the seeded published flow and waits until its four steps are drawn and settled.
func (s *easyDragSmoke) openSeededEditor() {
	s.t.Helper()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.wait(`()=>!!document.querySelector('[data-ed-pill]') && document.querySelector('[data-ed-pill-text]').textContent.length>0`)
}

// TestEasyDragHeaderPopoverBrowser opens the pill's popover and closes it with the keyboard and the mouse: Enter opens it and moves the
// focus in, Tab walks its entries, Escape closes it and gives the focus back to the pill, a press outside closes it. The next run the
// fixture reports shows as the weekday and the time.
func TestEasyDragHeaderPopoverBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openSeededEditor()
	p.MustEval(`()=>{edFixture.state.nextRun={at:new Date(Date.now()+36*3600*1000).toISOString(),node:'n_zeitplan'}}`)

	p.MustEval(`()=>document.querySelector('[data-ed-pill]').focus()`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]') && document.activeElement===document.querySelector('[data-ed-pop="active"]') && document.querySelector('[data-ed-pill]').getAttribute('aria-expanded')==='true'`)
	// The popover read the flow when it opened: the fixture's next run is there, in the words of ED.format.
	s.wait(`()=>{const at=edFixture.state.nextRun.at;const v=document.querySelector('[data-ed-next-run]').textContent;return v.length>0 && v===window.EasyDrag.format.nextRun(at)}`)
	s.shot("header-popover")
	s.noRawKeys("header popover")
	p.Keyboard.MustType(input.Tab)
	s.wait(`()=>document.activeElement===document.querySelector('[data-ed-pop="run-now"]')`)
	p.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]') && document.activeElement===document.querySelector('[data-ed-pill]') && document.querySelector('[data-ed-pill]').getAttribute('aria-expanded')==='false'`)

	// A press on the canvas closes it without moving the focus to the pill.
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	p.MustElement(`.ed-canvas`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]')`)
	s.failOnPageErrors("header popover")
}

// TestEasyDragHeaderPrimaryBrowser walks the primary button through its states with the fixture's facts: none for a clean live flow, Test for a
// changed and untested draft, Publish with its change count once the server says the draft is tested, and the dialog (never a direct publish).
func TestEasyDragHeaderPrimaryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openSeededEditor()
	filled := `[...document.querySelectorAll('.ed-head [data-ed-primary]')].map(b=>b.getAttribute('data-ed-cmd')).join()`
	s.wait(`()=>` + filled + `==='' && document.querySelector('[data-ed-cmd="publish"]').hidden && !document.querySelector('[data-ed-cmd="test"]').hidden`)
	s.shot("header-primary-none")

	// A changed draft the server has not seen pass a test: Test is the filled button.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed once')}`)
	s.wait(`()=>` + filled + `==='test' && document.querySelector('[data-ed-cmd="publish"]').hidden && edFixture.editor().saver.state==='saved'`)
	s.shot("header-primary-test")

	// The next save answers "tested" and the count of the changes: three, as the button says.
	p.MustEval(`()=>{edFixture.state.tested=true;edFixture.state.changes={steps:2,connections:1};const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed twice')}`)
	s.wait(`()=>` + filled + `==='publish' && !document.querySelector('[data-ed-cmd="publish"]').hidden && /3/.test(document.querySelector('[data-ed-cmd="publish"]').textContent)`)
	s.shot("header-primary-publish")
	s.noRawKeys("header primary")

	// The primary button opens the full dialog and publishes nothing by itself.
	published := p.MustEval(`()=>edFixture.flow().rec.published_draft_revision`).Int()
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	if got := p.MustEval(`()=>edFixture.flow().rec.published_draft_revision`).Int(); got != published {
		t.Fatalf("the primary button published the draft by itself: revision %d, want %d", got, published)
	}
	p.MustElement(`.ed-modal [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
	s.failOnPageErrors("header primary")
}

// TestEasyDragHeaderPopoverMotionBrowser clicks real popover entries with the desktop's animations on (the W1b lesson: a test with reduced
// motion cannot see an action that does nothing while a menu closes with animation): Active switches the flow off and on in place, Run now
// closes the popover and starts a run, Runs opens the drawer.
func TestEasyDragHeaderPopoverMotionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)
	s.openSeededEditor()

	p.MustElement(`[data-ed-pill]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="active"]`).MustClick()
	s.wait(`()=>edFixture.editor().flowEnabled===false && document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='paused' && document.querySelector('[data-ed-pop="active"]').getAttribute('aria-checked')==='false'`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="active"]`).MustClick()
	s.wait(`()=>edFixture.editor().flowEnabled===true && document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='live'`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="run-now"]`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]') && !!edFixture.editor().run`)
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success'}`)
	s.pillEntry("runs")
	s.wait(`()=>!!document.querySelector('.ed-drawer') && !document.querySelector('[data-ed-pill-popover]')`)
	s.failOnPageErrors("header popover motion")
}
```

- [ ] **Step 3: run the browser tests**

```
cd W && gofmt -l ui/desktop_easydrag_header_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragHeader|TestEasyDragControlBudgetsBrowser' -count=1 -timeout 15m -v 2>&1 | grep -E "^(=== RUN|--- |PASS|FAIL|ok)|BUDGET"
```
Expected: three `--- PASS` for the header tests and `--- PASS: TestEasyDragControlBudgetsBrowser` with `BUDGET region=header count=7 ceiling=8 target=8 gate=W1c` (the count is 7 for the clean flow). For each outcome:
  - the budget count is above 8: a control was added; the log lists the header's items (`CONTROLS region=header ...`); one of them is not meant to be there.
  - `TestEasyDragHeaderPrimaryBrowser` times out waiting for `publish`: the fixture does not answer `tested` (Part A's fixture uses other names): open `ui/testdata/easydrag-fixture.js` and make `edFixture.state.tested` and `.changes` control the answers, as B.1 does.
  - `TestEasyDragHeaderPopoverMotionBrowser` fails on the Run now entry: look at `ed.actionFacts()` in the page: a `vd-context-menu` that is still closing makes the registry refuse (the W1b lesson); the check waits for `.vd-context-menu:not(.vd-context-menu-closing)` to be gone before the pill is pressed.

Then the full run, once, with the W1a lab (zero findings) and no leftover rod Chromes before it:

```
cd W && tasklist //FI "IMAGENAME eq chrome.exe" | head -5
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m 2>&1 | tail -5
```
Expected: `ok` (about 10 to 17 minutes). Failures in tests that press Publish, Runs, Run now or Active are the sites of B.3 and B.4: look at the screenshot named in the failure.

- [ ] **Step 4: T0 and the line counts**

```
cd W && npm run test:easydrag:t0 && wc -l ui/js/desktop/apps/easydrag-{editor,interact,home,dialogs,canvas}.js
```
Expected: no FAIL; editor 847 (at most 896), interact 753, home 888, dialogs 281, canvas 925.

- [ ] **Step 5: commit**

Write `C:/disposable/easydrag-w1c/b6-msg.txt`:

```text
test(easydrag): header budget enforced at 8, popover and primary button in the browser

W1c Part B.6. The header row of the control budget is enforced (ceiling and
target 8). Browser tests: the popover opens and closes by keyboard and by a
press outside, shows the next run in the words of ED.format; the primary
button goes through none, Test and Publish with its count and opens the full
dialog without publishing; with the desktop's animations on, real clicks on
the Active, Run now and Runs entries act. The notes of the apps folder
describe the new header.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/desktop_easydrag_budget_browser_test.go ui/desktop_easydrag_header_browser_test.go ui/js/desktop/apps/AGENTS.md && git commit -F C:/disposable/easydrag-w1c/b6-msg.txt && git log --oneline -1
```

---

### Coverage table

| Spec item | Task |
|---|---|
| §4.1/4.2 layout: Back, name, pill; Stop slot, Test, primary, Undo, Redo, ⋯; pill ellipsis; narrow layouts | B.5 (markup, CSS), B.3 (pill CSS) |
| §4.3 pill states, disc and text, fixed order | B.3 (`describe`, check `pill rows`) |
| §4.4 popover through the action registry; next run by `ED.format`; blocks variant; completing action | B.3 (entries, `format.nextRun`, checks) |
| §4.5 primary states, plural count, always the full dialog, Test beside it, aria-disabled reason | B.4 (checks, browser check in B.6) |
| §4.6 budget enforced at 8 | B.6 |
| §4.7 Node: every pill row, every primary row, popover contents per state | B.3, B.4 |
| §4.7 browser: popover by keyboard, Active and Run now with reasons, budget | B.6 (and B.3's Node reasons) |
| Contract: `ed.flow.next_run`, `tested`, `changes` from GET and every save | B.1 |
| Contract: `ED.kit.popover` | B.2 |
| Contract: hooks `[data-ed-pill]`, `[data-ed-pill-popover]`, `[data-ed-primary]`, `[data-ed-cmd="test"]`, `[data-ed-stop-slot]` | B.3, B.4, B.5 |
| Boundary with C: the empty Stop slot holding today's `[data-ed-cmd="cancel"]` unchanged in behaviour | B.5 |
| Line rule, `TestEasyDragSplitSeams`, module registration | B.1, B.3, B.5 |
| Tests of removed buttons moved without weakening | B.3, B.5 (table) |

### Deviations from the spec and the brief

1. **Tasks.** B.3 holds the pill and its popover together (the brief had B.3 and B.4): the popover is where Runs, Run now and Active went, and the tests that press them move in the same commit. B.4 is the primary button, B.5 the row and the slot, B.6 the budget and the browser tests.
2. **The Stop slot** is a fixed-width box around the button; the button keeps `hidden` while idle (so every Stop check of today still reads `hidden`), not `visibility: hidden`. Its width is a guess (`9.5rem`, `--ed-stop-slot-w`); Part C sets it from the real labels. The Stop label is shown at every width from B.5 on (spec §4.2: "except the Stop label").
3. **"Testen"** is the registry's `flow.test` label ("Entwurf testen", "Test draft"), not a new word; "Baustein simulieren" for blocks is unchanged.
4. **The pill's order** puts a past run right after a run (the old header hid every problem chip in the run view). A running viewed run says "Läuft". The spec table lists "past run" last.
5. **A partial publish** keeps Publish as the primary button (as it always did: "Publish stays offered"); the spec's table has no row for it. The pill popover offers "Veröffentlichen abschließen" too.
6. **Texts:** the partial publish and the stale live version use the existing keys (`state_publish_incomplete`: "Veröffentlichung unvollständig"; `state_live_outdated`), the stopped timers `timers_stopped_title`.
7. **The ⋯ menu** no longer gains Runs and Test in a narrow editor (`headerNarrow` is gone): both are reachable without it. The menu keeps its entries (ceiling 12).
8. **`[data-ed-revisions]`** now sits on the pill (its `data-kind`, `data-draft`, `data-live`, `data-paused` are what `a1008` reads); `revision_live` and `revision_ahead` are unused keys now (left in place; their lint still passes).
9. **`ED.kit.popover`** returns `{el, close, update, isOpen}`; the contract is `{el, close}`.
10. **The count key** is a plural key (`publish_with_changes`) applied with `locale-apply-forms.mjs`, not `locale-apply.mjs`.
11. **The fixture** (B.1, only without Part A): `factsOf` in `ui/testdata/easydrag-fixture.js` with the three state names of the contract. Part A's fixture task must merge with it (same names); the answer shape is the contract's (top level).
12. **The popover's next run** is read when it opens (one GET per open), so a schedule changed since the editor loaded shows; the save answer says nothing of the schedule, so `next_run` stays on `ed.flow` until the next read.
13. **Time zone.** `ED.format.nextRun` shows the browser's local time; the server answers with its own offset. They differ only when the browser is in another zone (UX-13 settles the display with the DST note).

### Decisions needed

1. **Manual chapter 24** (`documentation/manuals/*/24-easydrag.md`, en line 221, de line 251) still describes the old header (state chips, Runs and Active in the header, Runs in the ⋯ menu on a phone). Recommendation: a docs-agent task after Part C lands (the Stop text changes there too); not part of this plan.
2. **Narrow header (390 px and below).** With the slot reserved the right group takes two rows on a phone. Recommendation: accept it for B; Part C measures the slot with the real label and decides whether Undo and Redo move into ⋯ below 560 px (the budget counts them at 1440 px only).
3. **The partial-publish primary** (deviation 5). Recommendation: keep; removing it leaves the finish action only in the popover.
4. **Blocks and "tested".** The contract names the flow API only. A block's `tested` comes from the editor's own evidence (`ed.testEvidence`) unless the block API answers the flag too; after a reload a block is untested until its next simulation. Recommendation: ask Part A for the block answers, or accept.
5. **Unused keys** `revision_live`, `revision_ahead`: delete in a later cleanup together with their lint entries.

### What this part verified, and what it could not

Verified on a scratch copy of main (`git archive`, then every patch script and edit group above, in order): the Node suite `node scripts/test-easydrag.mjs` after B.1, B.3, B.4 and B.5 (0 failures; the one infrastructure failure of the copy is the manual chapter, which the copy lacks); the new checks (21 of them: B.1 3, B.2 2, B.3 10, B.4 4, B.5 2); the key files with `locale-apply.mjs` and `locale-apply-forms.mjs`; `gofmt -e` on every Go file of the part; the key-literal rule, key parity and the lowercase "du" rule as a Node emulation; the editor's line count (847). **Not verified:** the Go tests (the copy has no module and no embedded assets: the static tests and the vet were not run), all browser tests, the CSS in a browser (the pill, the slot and the wrapping are reasoned from the stylesheet), and the Windows CRLF run of the patch scripts (the helper keeps line endings; its tests are W1b's).
