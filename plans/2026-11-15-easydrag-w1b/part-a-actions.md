## Part A: The action registry

Spec: `plans/2026-11-15-easydrag-w1b-spec.md` §1 decisions 4 and 7, §3, §6, §8. Part A gives every action one name, one shortcut and one reason for being unavailable. It adds `ui/js/desktop/apps/easydrag-actions.js` (`ED.actions`), moves the eleven menu and button surfaces onto it, routes the registry's shortcuts through one keyboard path, generates the shortcut table from it, and adds the lint and the browser test that keep the surfaces in agreement.

**Shell and worktree.** Git Bash. Worktree `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions`, branch `feat/easydrag-w1b-actions`, cut from main when the part starts. Every command starts with `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && …`. Never edit, stage or commit in the main checkout. Go commands carry `GOTOOLCHAIN=go1.27.4` and `GOTMPDIR='D:\tmp\go-build'` (all Go work here is package `ui`). Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots` and the Bash sandbox off.

**Depends on.** W1a is on main (status map, kit, icon map, format). Part C (budgets) lands first and touches no file of this part; Part B runs in parallel and shares the locale files and, in other functions, `easydrag-editor.js`, `easydrag-home.js` and `easydrag-dialogs.js` (see "Shared files" below).

**How the edits are made.** The worktree has CRLF line endings, the repository LF, and the Write and Edit tools mangle some escapes. Every multi-site migration is therefore a small Node patch script that replaces an exact old snippet by an exact new snippet and fails loudly when the old snippet is not found exactly once (or when a replaced block has a different line count than the plan says). The helper `lib-patch.mjs` (A.0) and the scripts live in `C:/disposable/easydrag-w1b/`; run a script from the worktree root. A script is run once: if it fails half-way, `git checkout -- <the files it names>` and run it again after fixing the cause. Each patch script lists its old snippets in full, so the "old form" of every site is in the script; the greps after each script prove that no old form remains.

**IMPACT** (before changing a function; the plan names them per task):
```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```
Warn the coordinator on HIGH or CRITICAL. UNKNOWN is unresolved: confirm the callers with `grep -rn`. GitNexus cannot index a worktree, so the graph is of main; the per-task grep steps are the check on the worktree.

**Every commit body ends with** `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Write the message file with the Write tool into `C:/disposable/easydrag-w1b/`, commit with `git commit -F`. Never amend, rebase or push inside a task.

**Escapes.** No code or key file below contains a backslash-u sequence (the Write and Edit tools would decode it); the minus sign and the en dash are `String.fromCodePoint(0x2212)` and `(0x2013)`, and the check mark `(0x2713)`. Keep it so.

### The design in one page

**Rows.** `ED.actions.ROWS` is a frozen array of rows `{ id, label, icon, glyph, shortcut, scope, risk, needs, menu }`:
- `id` is `group.name`; `label` a literal `easydrag.ui.` key; `shortcut` canonical (`Ctrl+Shift+Z`, `Delete`, `Shift+1`, `+`, `-`, `D`) or `''`; `scope` is `editor` or `canvas`; `risk` is `safe`, `undoable` or `confirm`; `needs` lists the shared checks.
- `icon` is the desktop menu icon name that the menus use today (`save`, `trash`, `check-square` …). `glyph` is the `core.ICONS` name for the in-app icon buttons (the card tools, Undo and Redo), or `''`. Deviation, see below: the spec says "an alias from the W1a icon map"; that map has 16 one-to-one meanings and cannot name the card tools' glyphs, and the desktop menus draw desktop icon names.
- `menu` is the id the item has in the desktop's window menus (`run`, `delete-sel`, `select-all` …). The desktop uses it as the action key, and many Node and browser tests find items by it; it stays. A row without one uses its own id.

**Shared checks** (`needs`, run in order; each cause has exactly one reason key, so the text is the same on every surface):

| Check | Cause | Reason key (`easydrag.ui.`) | Reads |
|---|---|---|---|
| `writable` | a past run is open | `reason_run_view` | `ed.runView` |
| | read-only mode | `reason_readonly` | `ed.readonly` / the host's `readonly` |
| | agent permissions or a stale list lock the flows | `reason_locked` | host fact `locked` (home: `createOff()` without read-only) |
| `unlocked` | the same two last causes, not the run view | as above | for whole-flow actions a past run does not stop (duplicate, delete, new, import) |
| `free` | a dialog, a restore offer, a desktop context menu is open | `reason_dialog_open`, `reason_restoring` | host facts `dialog`, `restoring` |
| `quiet` | quick-add is open (edits wait for it) | `reason_dialog_open` | `ed.quickAdd` |
| `selection` | no step is selected and the context names none | `reason_no_selection` | `ed.selection.size`, `ctx.node` |
| `published` | the flow has no published version | `reason_not_published` | `ed.flow.live`, or `ctx.flow.published` |
| `active` | published but paused | `reason_paused` | `ed.flowEnabled`, or `ctx.flow.enabled` |
| `idle` | a run is in progress | `reason_running` | host fact `running` (`runs.isRunning()`) |
| `steps2` | fewer than two steps | `reason_two_steps` | `ed.model.doc.nodes.length` |

A bound action adds its own check (`check(ctx)` returns a reason key or nothing) with its own reasons: `reason_nothing_to_undo`, `reason_nothing_to_redo`, `reason_nothing_to_paste`, `reason_palette_hidden`, `reason_no_steps`, `reason_no_output`, `reason_export_unsaved`, `reason_stop_unavailable`.

**Hosts, binding and context.** The object an action works on is its host: the editor state `ed`, the start page's `actionHost`, or the shell instance `inst` of the start page menu. `ED.actions.bind(id, impl, host)` stores the owner's logic per host (a `WeakMap`), so several editor windows never share bindings; without a host the binding is global (the Lebendigkeit choices, which are per device). `impl` is `{ run(ctx), check(ctx), label(ctx), show(ctx), checked(ctx), accepts(event, where) }`: `label` returns another label key (only `flow.test` and `flow.delete` in a block, `step.toggle` by the step's state, `view.search` "Add a step" while one step is selected, `flow.new` in the block library); `show` false leaves the action out of every menu and toolbar; `checked` draws the check mark; `accepts` false hands a keystroke to the next handler. A host provides `t` and may provide `actionFacts()`, which adds the facts of the moment (`restoring`, `dialog`, `running`, `locked`). A context is `{ host, flow?, node?, x?, y?, keyed? }`.

**Surfaces.**
- `items(ids, ctx)`: desktop menu entries. `'-'` is a separator; separators never lead, trail or double; an unbound or `show()`-false action is left out; `disabledHint` carries the reason; `danger` is set for `undoable` and `confirm` rows; a row in `DESKTOP_KEYS` (`flow.save`, `flow.test`, `view.search`) carries `shortcut` (the shell dispatches it), every other row `shortcutHint` (display only); `ctx.keyed` gives `labelKey` instead of `label` (window menus: the desktop translates).
- `button(id, ctx)`: `{ 'aria-label', title, 'aria-disabled', 'aria-description' }`, `null` when unbound. `paint(el, id, ctx, opts)` writes it onto an element; `attrString(attrs)` renders it as markup.
- `run(id, ctx)` re-checks the state and runs only when enabled (returns `false` when refused). Every click path (menu entry, tool, header button) and every shortcut ends in it.
- `key(event, where)` is the one keyboard path (A.5). `table(layout, ctx)` builds the shortcut table (A.5). `observe(fn)` lets the browser test count runs.

### Decisions the code forced

1. **Task order differs from the suggested cut.** The editing and zoom actions are owned by `easydrag-interact.js` (undo, copy, paste, delete, duplicate, select all, toggle, connect, zoom, auto layout all call its closures), so the registry bindings for them live there. The Edit and View window menus (A.3) need those bindings, so the interact surfaces (card tools, card menu, canvas menu) come first (A.2) and the editor's window menus and "more" menu second (A.3). `interact.js` is 765 lines after A.2 (753 today) and 750 after A.5; the "only shorter" rule holds at the end of the part and at every commit from A.5 on.
2. **Keyboard scopes follow today's code, not the spec's prose.** Today the Edit shortcuts (Ctrl+Z, Ctrl+C, Delete …) act only while the canvas, a card or the page has the focus (`handleKey` is behind `onCanvas`). They are `canvas` scope rows, not `editor` scope. Only Ctrl+S, Ctrl+Enter, Ctrl+K, Ctrl+F and Alt+Shift+F are editor-wide. See the table in A.5.
3. **Added rows** (spec §3.2 allows this): `flow.open_draft` (the block editor's "Open current draft"), and `flow.new`, `flow.import`, `flow.reload` (the start page's window menu, which `easydrag.js` builds with `labelKey:` today and the lint would otherwise have to exempt).
4. **Added checks:** `unlocked` (see the table) and `quiet`. Reason: duplicating and deleting a flow, and Export, work in a past run today (the editor's `duplicateFlow` says so); `quiet` keeps "edits wait for quick-add" without making Save and Test wait for it.
5. **Added helpers:** `paint`, `attrString`, `label`, `display`, `table`, `observe`, `TOOLS`, `REASONS`, `CHECKS`. The contract names (`ROWS`, `bind`, `state`, `run`, `items`, `button`, `key`, `row`) are exactly as agreed with Parts B and C.

**Shared files.** Part B edits `easydrag-editor.js` at line 477 (`repair_mission_done`), `easydrag-home.js` at `runNow` (659) and 99 (search `maxlength`), `easydrag-dialogs.js` at 219-226 (name and description limits, `flow_max_run_hint`) and the locale files. This part edits `onKeyDown`, `setMenus`, `moreMenu`, the template and bindings in the editor; `cardMenu` in home; `SHORTCUTS`, `kbd`, `shortcutRows` in dialogs. All edits are local to those functions. On a rebase conflict in a locale file take main's file and run the key files again in task order (`a-1`, `a-7`).

---

### Task A.0: Preamble (run once, before A.1)

- [ ] **Step 0.1: verify main**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git log --oneline -5 && git status --short | head -8
for f in editor interact home dialogs canvas header history delight; do printf "%s: " $f; wc -l < ui/js/desktop/apps/easydrag-$f.js; done
grep -n "function onKeyDown\|function handleKey\|function setMenus\|function moreMenu\|function openNodeMenu\|function openCanvasMenu\|function cardMenu\|function runNodeTool\|function toolButton\|function menuItems\|function shortcutRows" ui/js/desktop/apps/easydrag-{editor,interact,home,dialogs,canvas,delight}.js
ls ui/js/desktop/apps/easydrag-actions.js scripts/test-easydrag-actions.mjs ui/desktop_easydrag_actions_test.go 2>&1 | head -3
```
Expected: main at 065b6fca8 or later; the other sessions' uncommitted edits (`ui/AGENTS.md`, media files, `ui/theme_optics_*`) are not yours, ignore them; line counts editor 896, interact 753, home 889, dialogs 309, canvas 928 (header 223, history 83, delight 95); every function above found once; the last command reports "No such file" three times. For each outcome:
  - A line count is lower than the list: another session or Part C/B landed an edit in that file. Re-read the sites the later tasks name (each task re-verifies its old snippets, a patch script stops when one moved) and use the new count as that file's baseline for every "expected lines" number below (the rule is "not longer than the baseline", the numbers are for orientation).
  - `easydrag-actions.js` exists: another session started this part. Stop and tell the coordinator.

- [ ] **Step 0.2: create the worktree**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1b-actions -b feat/easydrag-w1b-actions main && cd .worktrees/easydrag-w1b-actions && git branch --show-current && git log --oneline -1
```
Expected: `feat/easydrag-w1b-actions` and main's head commit.

- [ ] **Step 0.3: scratch directories and the patch helper**

```
mkdir -p C:/disposable/easydrag-w1b/patches D:/tmp/go-build D:/tmp/easydrag-w1b-shots
```
Write `C:/disposable/easydrag-w1b/lib-patch.mjs` with the Write tool:

```js
// Exact-text edit helper for the W1b Part A patch scripts. edit() replaces a snippet that must occur exactly
// `count` times (default once) and keeps the file's line endings (the worktree has CRLF, the repository LF).
import fs from 'node:fs';

export function edit(file, oldText, newText, count = 1) {
    const raw = fs.readFileSync(file, 'utf8');
    const crlf = raw.includes('\r\n');
    const fit = text => (crlf ? text.replace(/\r?\n/g, '\r\n') : text.replace(/\r\n/g, '\n'));
    const before = fit(oldText);
    const found = raw.split(before).length - 1;
    if (found !== count) throw new Error(file + ': expected ' + count + ' match(es) of ' + JSON.stringify(oldText.slice(0, 80)) + ', found ' + found);
    fs.writeFileSync(file, raw.split(before).join(fit(newText)));
}

// lines counts the lines of a file the way wc -l does.
export function lines(file) {
    return (fs.readFileSync(file, 'utf8').match(/\n/g) || []).length;
}

// replaceBlock replaces the lines from the one containing `from` up to, not including, the one containing
// `before` (each must match exactly one line) with `text`, and checks how many lines went (`expected`).
export function replaceBlock(file, from, before, text, expected) {
    const raw = fs.readFileSync(file, 'utf8');
    const crlf = raw.includes('\r\n');
    const rows = raw.split(crlf ? '\r\n' : '\n');
    const at = (needle) => {
        const hits = rows.map((row, i) => (row.includes(needle) ? i : -1)).filter(i => i >= 0);
        if (hits.length !== 1) throw new Error(file + ': ' + JSON.stringify(needle) + ' matches ' + hits.length + ' lines');
        return hits[0];
    };
    const start = at(from);
    const end = at(before);
    if (end <= start) throw new Error(file + ': ' + JSON.stringify(before) + ' is not after ' + JSON.stringify(from));
    if (end - start !== expected) throw new Error(file + ': the block has ' + (end - start) + ' lines, expected ' + expected);
    rows.splice(start, end - start, ...(text === '' ? [] : text.replace(/\n$/, '').split('\n')));
    fs.writeFileSync(file, rows.join(crlf ? '\r\n' : '\n'));
}
```
The helper keeps each file's line endings. `edit(file, old, new, count)` needs exactly `count` matches (default one). `replaceBlock(file, from, before, text, expected)` replaces the lines from the one containing `from` up to, not including, the one containing `before` (each matches exactly one line) and checks that exactly `expected` lines go.

- [ ] **Step 0.4: node_modules and baseline T0**

The EasyDrag modules are not in a UI bundle (`grep -c easydrag scripts/build-ui-bundles.js` is 0), so no `npm run build:ui` and no `node_modules` copy is needed.
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' npm run test:easydrag:t0 2>&1 | tail -8
```
Expected: every job `ok`/`PASS`, exit 0. A failure here is not yours: report it before you start.

- [ ] **Step 0.5: where the menu ids the tests rely on come from**

The window-menu item ids that tests find items by (`run`, `delete`, `delete-sel`, `select-all`, `home`, `export`, `extract-selection`, `paste`, `save`, `reload` …) are kept as the rows' `menu` field. Confirm none is missing from the table you write in A.1:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && grep -rhoE "i(tem)?\.id === '[a-z-]+'" scripts/test-easydrag*.mjs | sort -u
```
Expected ids: `delete`, `delete-sel`, `export`, `extract-selection`, `home`, `paste`, `reload`, `run`, `save`, `select-all`, `versions` (and, through other patterns, `test` and `search`). Every one is a `menu` value in the A.1 table (`flow.delete` `delete`, `edit.delete` `delete-sel`, `flow.export` `export`, `edit.extract` `extract-selection`, `flow.home` `home`, `edit.paste` `paste`, `flow.reload` `reload`, `flow.run_now` `run`, `flow.save` `save`, `edit.select_all` `select-all`, `flow.versions` `versions`, `flow.test` `test`, `view.search` `search`). An id that is not: add it as that row's `menu` value, do not change the test.

---

### Task A.1: `easydrag-actions.js`, its Node test, the registration, the reason keys

**Files:**
- Create: `ui/js/desktop/apps/easydrag-actions.js`
- Create: `scripts/test-easydrag-actions.mjs`
- Create (by the coordinator, from this plan's `keys/` folder): `plans/2026-11-15-easydrag-w1b/keys/a-1.json`
- Modify: `ui/js/desktop/core/module-loader.js` (easydrag list, after `easydrag-icons.js`, around line 290)
- Modify: `ui/desktop_easydrag_test.go:11-16` (`easyDragModules`)
- Modify: `scripts/test-easydrag.mjs:30`, `scripts/test-easydrag-extra.mjs:9`, `-extra2.mjs:9`, `-extra3.mjs:9`, `-extra4.mjs:11-12` (the module lists that the Node harnesses load)
- Modify: `ui/lang/easydrag/*.json` (16 files, through the key file only)

**Texts.** The key file `a-1.json` adds 20 keys: the reasons the shared checks and the bound checks return, the label `export_email` (the desktop's own "Email …" texts, so the editor's mail entry has an `easydrag.ui.` label) and `key_delete` (the localized name of the Delete key). en and de:

| Key (`easydrag.ui.`) | en | de |
|---|---|---|
| `reason_run_view` | A past run is open, so nothing can be changed. | Im Lauf-Rückblick kann nichts geändert werden. |
| `reason_readonly` | Read-only mode is on. | Der Nur-Lese-Modus ist an. |
| `reason_locked` | Locked by the agent permissions, or until the list refreshes. | Durch die Agenten-Rechte gesperrt oder bis die Liste neu geladen ist. |
| `reason_dialog_open` | Close the open dialog first. | Schließe zuerst den offenen Dialog. |
| `reason_restoring` | Answer the restore offer first. | Beantworte zuerst die Frage zur Wiederherstellung. |
| `reason_no_selection` | Select a step first. | Wähle zuerst einen Schritt. |
| `reason_not_published` | Possible only after publishing. | Erst nach dem Veröffentlichen möglich. |
| `reason_paused` | The flow is paused. | Der Flow ist pausiert. |
| `reason_running` | A run is in progress. | Ein Lauf läuft gerade. |
| `reason_two_steps` | Needs at least two steps. | Braucht mindestens zwei Schritte. |
| `reason_nothing_to_paste` | Nothing to paste. | Nichts zum Einfügen. |
| `reason_nothing_to_undo` | Nothing to undo. | Nichts rückgängig zu machen. |
| `reason_nothing_to_redo` | Nothing to redo. | Nichts zu wiederholen. |
| `reason_palette_hidden` | The step list is hidden while a past run is open. | Im Lauf-Rückblick ist die Schrittliste ausgeblendet. |
| `reason_no_steps` | The flow has no steps yet. | Der Flow hat noch keine Schritte. |
| `reason_no_output` | This step has no output. | Dieser Schritt hat keinen Ausgang. |
| `reason_export_unsaved` | Unsaved changes cannot be exported in read-only mode. | Ungespeicherte Änderungen lassen sich im Nur-Lese-Modus nicht exportieren. |
| `reason_stop_unavailable` | Stopping is not possible right now. | Das Stoppen ist gerade nicht möglich. |
| `export_email` | Email … | Per E-Mail … |
| `key_delete` | Del | Entf |

Two reasons differ from the spec's table on purpose, both by reading the code (voice rule 7, "only what the code guarantees"): `reason_locked` also covers a stale list (`createOff()` is true for a stale list, FLOWS_DISABLED, lost authorisation and revoked writes, not only the agent permissions), and `reason_restoring` names the restore offer (the editor's `restoring` flag is the emergency-copy offer, not a version restore). The other 14 locales are in the key file, in the register of each locale's existing texts.

- [ ] **Step 1.1: write the failing Node test**

Create `scripts/test-easydrag-actions.mjs` (it runs on its own, like the motion test; T0 picks it up by its name):

```js
#!/usr/bin/env node
// W1b Part A: the action registry (easydrag-actions.js). Every row has a label key in all 16 locales, an icon,
// a valid risk and scope and a shortcut no other row of its scope uses; every cause of a shared check has a
// reason in all 16 locales; state() refuses and run() does nothing when a check fails; items() marks risky
// rows danger, leaves out what nobody bound and drops stray separators; key() fires an action once.
// Legacy assert: the arrays and objects come out of the vm context, so a strict deepEqual would compare prototypes.
import assert from 'node:assert';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const lang = new URL('../ui/lang/easydrag/', import.meta.url);
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = Object.fromEntries(LOCALES.map(l => [l, JSON.parse(fs.readFileSync(new URL(l + '.json', lang), 'utf8'))]));

const box = vm.createContext({ window: {}, navigator: { platform: 'Linux' }, document: {}, crypto: webcrypto, console, Intl, Date, Set, Map, WeakMap });
for (const file of ['easydrag-core.js', 'easydrag-actions.js']) vm.runInContext(fs.readFileSync(new URL(file, apps), 'utf8'), box, { filename: file });
const ED = box.window.EasyDrag;
const A = ED.actions;

// 1. The table.
const seen = new Set();
const keysByScope = { editor: new Map(), canvas: new Map() };
for (const row of A.ROWS) {
    assert.match(row.id, /^(flow|edit|step|view)\.[a-z_]+$/, row.id + ' is not group.name');
    assert.ok(!seen.has(row.id), row.id + ' twice');
    seen.add(row.id);
    for (const l of LOCALES) assert.ok(words[l][row.label], row.id + ': ' + row.label + ' is missing in ' + l);
    assert.ok(row.icon, row.id + ' has no icon');
    assert.ok(!row.glyph || Object.prototype.hasOwnProperty.call(ED.core.ICONS, row.glyph), row.id + ': glyph ' + row.glyph + ' is not a core icon');
    assert.ok(['safe', 'undoable', 'confirm'].includes(row.risk), row.id + ' risk');
    assert.ok(['editor', 'canvas'].includes(row.scope), row.id + ' scope');
    row.needs.forEach(need => assert.equal(typeof A.CHECKS[need], 'function', row.id + ': unknown check ' + need));
    if (row.shortcut) {
        const taken = keysByScope[row.scope].get(row.shortcut);
        assert.equal(taken, undefined, row.id + ' and ' + taken + ' share ' + row.shortcut + ' in the ' + row.scope + ' scope');
        keysByScope[row.scope].set(row.shortcut, row.id);
    }
}
assert.deepEqual(A.ROWS.filter(r => r.risk !== 'safe').map(r => r.id + ':' + r.risk), ['flow.delete:confirm', 'edit.cut:undoable', 'edit.delete:undoable']);
assert.ok(Object.isFrozen(A.ROWS) && Object.isFrozen(A.ROWS[0]), 'the table is frozen');
assert.deepEqual([...A.TOOLS.map(([tool, id]) => tool + ':' + id + ':' + A.row(id).glyph)], ['add:step.add_next:plus', 'connect:step.connect:link', 'test:step.test:flask', 'disable:step.toggle:eye-off', 'duplicate:edit.duplicate:copy', 'delete:edit.delete:trash'], 'the card tools and their glyphs');
// the shortcuts the editor shows today, spelled canonically
assert.deepEqual(['flow.save', 'flow.test', 'view.search', 'view.find_step', 'edit.undo', 'edit.redo', 'edit.delete', 'view.zoom_fit', 'view.zoom_reset', 'view.shortcuts', 'edit.auto_layout']
    .map(id => A.row(id).shortcut), ['Ctrl+S', 'Ctrl+Enter', 'Ctrl+K', 'Ctrl+F', 'Ctrl+Z', 'Ctrl+Shift+Z', 'Delete', 'Shift+1', 'Ctrl+0', '?', 'Alt+Shift+F']);

// 2. Every cause has a reason in all 16 locales; so do the reasons that bound actions return and the key name of Delete.
const reasonKeys = Object.values(A.REASONS).concat(['nothing_to_paste', 'nothing_to_undo', 'nothing_to_redo', 'palette_hidden', 'no_steps', 'no_output', 'export_unsaved', 'stop_unavailable'].map(k => 'easydrag.ui.reason_' + k), ['easydrag.ui.key_delete', 'easydrag.ui.export_email']);
for (const key of reasonKeys) for (const l of LOCALES) assert.ok(words[l][key] && words[l][key].trim(), key + ' is missing in ' + l);
assert.equal(new Set(Object.values(A.REASONS)).size, Object.keys(A.REASONS).length, 'one reason per cause');

// 3. state, run, items, button and key on a stub host.
const t = key => words.en[key] || key;
function makeHost(extra) {
    return Object.assign({ t, readonly: false, runView: null, selection: new Set(['a']), quickAdd: null, flow: { live: {} }, flowEnabled: true, model: { doc: { nodes: [{}, {}] } } }, extra || {});
}
function bindAll(host, calls) {
    const note = id => () => { calls.push(id); };
    ['flow.save', 'flow.test', 'flow.run_now', 'flow.delete', 'edit.undo', 'edit.cut', 'edit.copy', 'edit.delete', 'edit.paste', 'view.zoom_fit', 'view.zoom_in', 'view.zoom_out', 'step.open', 'step.toggle', 'edit.auto_layout']
        .forEach(id => A.bind(id, { run: note(id) }, host));
}
{
    const calls = [];
    const host = makeHost();
    bindAll(host, calls);
    const c = { host };
    assert.deepEqual(A.state('flow.save', c), { enabled: true, reason: '' });
    assert.equal(A.state('flow.stop', c).enabled, false, 'an action nobody bound is unavailable');
    A.run('flow.save', c);
    assert.deepEqual(calls, ['flow.save']);

    // writable: a past run, read-only mode, a locked list; the reasons are the one text per cause
    host.runView = {};
    assert.deepEqual(A.state('flow.save', c), { enabled: false, reason: words.en['easydrag.ui.reason_run_view'] });
    assert.equal(A.run('flow.save', c), false);
    assert.deepEqual(calls, ['flow.save'], 'run does nothing when a check fails');
    host.runView = null; host.readonly = true;
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_readonly']);
    assert.equal(A.state('edit.copy', c).enabled, true, 'Copy has no writable check: it works in read-only mode');
    assert.equal(A.state('edit.cut', c).reason, words.en['easydrag.ui.reason_readonly']);
    host.readonly = false; host.actionFacts = () => ({ locked: true });
    assert.equal(A.state('flow.delete', c).reason, words.en['easydrag.ui.reason_locked']);
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_locked']);
    // free: a dialog or the restore offer
    host.actionFacts = () => ({ dialog: true });
    assert.equal(A.state('edit.delete', c).reason, words.en['easydrag.ui.reason_dialog_open']);
    assert.equal(A.state('flow.save', c).reason, words.en['easydrag.ui.reason_dialog_open']);
    host.actionFacts = null;
    host.quickAdd = {};
    assert.equal(A.state('edit.delete', c).reason, words.en['easydrag.ui.reason_dialog_open'], 'edits wait for quick-add');
    assert.equal(A.state('flow.save', c).enabled, true, 'a save does not');
    host.quickAdd = null;
    host.actionFacts = () => ({ restoring: true });
    assert.equal(A.state('flow.save', c).reason, words.en['easydrag.ui.reason_restoring']);
    // idle, steps2, selection, published, active
    host.actionFacts = () => ({ running: true });
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_running']);
    host.actionFacts = null;
    host.model.doc.nodes.length = 1;
    assert.equal(A.state('edit.auto_layout', c).reason, words.en['easydrag.ui.reason_two_steps']);
    host.selection.clear();
    assert.equal(A.state('edit.cut', c).reason, words.en['easydrag.ui.reason_no_selection']);
    assert.equal(A.state('edit.cut', { host, node: 'n1' }).enabled, true, 'a card names its step');
    host.flow.live = null;
    assert.equal(A.state('flow.run_now', c).reason, words.en['easydrag.ui.reason_not_published']);
    host.flow.live = {}; host.flowEnabled = false;
    assert.equal(A.state('flow.run_now', c).reason, words.en['easydrag.ui.reason_paused']);
    assert.equal(A.state('flow.run_now', { host, flow: { published: true, enabled: true } }).enabled, true, 'a start page card brings its own flow');
    host.flowEnabled = true;

    // a bound check adds its own reason, label and show
    A.bind('step.test', { run() {}, check: () => 'easydrag.ui.reason_no_output', label: () => 'easydrag.ui.block_simulate', show: () => true }, host);
    assert.equal(A.state('step.test', { host, node: 'n1' }).reason, words.en['easydrag.ui.reason_no_output']);
    assert.equal(A.label('step.test', c), words.en['easydrag.ui.block_simulate']);

    // items: ids to menu entries
    host.selection.add('a');
    const out = A.items(['-', 'flow.save', '-', '-', 'flow.stop', '-', 'edit.cut', 'edit.copy', 'view.zoom_fit', 'flow.delete', '-'], c);
    assert.deepEqual(out.map(e => (e.separator ? '|' : e.id)), ['save', '|', 'cut', 'copy', 'zoom-fit', 'delete'], 'unbound rows, doubled, leading and trailing separators are dropped');
    const byId = Object.fromEntries(out.filter(e => !e.separator).map(e => [e.id, e]));
    assert.equal(byId.cut.danger, true, 'undoable is danger');
    assert.equal(byId.delete.danger, true, 'confirm is danger');
    assert.equal(byId.copy.danger, undefined);
    assert.equal(byId.save.shortcut, 'Ctrl+S', 'a shell shortcut is a real shortcut');
    assert.equal(byId.save.shortcutHint, undefined);
    assert.equal(byId.cut.shortcutHint, 'Ctrl+X', 'other shortcuts are only drawn');
    assert.equal(byId.cut.shortcut, undefined);
    assert.equal(byId.save.label, 'Save now');
    const keyed = A.items(['flow.save'], { host, keyed: true })[0];
    assert.deepEqual([keyed.labelKey, keyed.label], ['easydrag.ui.save_now', undefined], 'a window menu entry carries the label key');
    host.readonly = true;
    const ro = A.items(['edit.cut', 'edit.copy', 'edit.delete'], c);
    assert.deepEqual(ro.map(e => [e.disabled, e.disabledHint]), [[true, words.en['easydrag.ui.reason_readonly']], [false, undefined], [true, words.en['easydrag.ui.reason_readonly']]]);
    assert.equal(ro[2].shortcutHint, 'Del');
    ro[0].action();
    assert.equal(calls.includes('edit.cut'), false, 'a disabled entry does nothing when it is clicked');
    host.readonly = false;

    // button: aria-disabled with the reason, never disabled
    const on = A.button('edit.delete', c);
    assert.deepEqual(on, { 'aria-label': 'Delete', title: 'Delete (Del)' });
    host.readonly = true;
    const off = A.button('edit.delete', c);
    assert.equal(off['aria-disabled'], 'true');
    assert.equal(off['aria-description'], words.en['easydrag.ui.reason_readonly']);
    assert.ok(off.title.startsWith('Delete (Del)') && off.title.endsWith(words.en['easydrag.ui.reason_readonly']));
    assert.equal(A.button('flow.stop', c), null, 'an unbound action has no button');
    host.readonly = false;
    assert.equal(A.attrString({ title: 'a "b"' }), ' title="a &quot;b&quot;"');

    // display: the named Delete, the real minus
    assert.equal(A.display('Delete', t), 'Del');
    assert.equal(A.display('-', t), String.fromCodePoint(0x2212));
    assert.equal(A.display('Ctrl+Shift+Z', t), 'Ctrl+Shift+Z');
}

// 4. key: one handler, once; scope and text-field rules.
{
    const calls = [];
    const host = makeHost();
    bindAll(host, calls);
    A.bind('edit.undo', { run: () => { calls.push('edit.undo'); }, check: () => 'easydrag.ui.reason_nothing_to_undo' }, host);
    A.bind('edit.redo', { run: () => { calls.push('edit.redo'); } }, host);
    A.bind('step.open', { run: () => { calls.push('step.open'); }, accepts: (event, where) => !where.control }, host);
    let observed = [];
    const stop = A.observe(id => observed.push(id));
    const press = (k, extra, where) => {
        const event = Object.assign({ key: k, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra);
        const took = A.key(event, Object.assign({ host, onCanvas: true }, where));
        return { took, prevented: event.defaultPrevented };
    };
    assert.deepEqual(press('s', { ctrlKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('s', { ctrlKey: true }, { editable: true }), { took: true, prevented: true }, 'Ctrl+S works from a text field');
    assert.deepEqual(press('Enter', { ctrlKey: true }, { editable: true, quickAdd: true }), { took: true, prevented: true });
    assert.deepEqual(press('z', { ctrlKey: true }, { editable: true }), { took: false, prevented: false }, 'a text field keeps its own undo');
    assert.deepEqual(press('z', { ctrlKey: true }, { quickAdd: true }), { took: false, prevented: false }, 'edits wait for quick-add');
    assert.deepEqual(press('z', { ctrlKey: true }, { onCanvas: false }), { took: false, prevented: false }, 'canvas keys need the canvas');
    assert.deepEqual(press('Z', { ctrlKey: true, shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('y', { ctrlKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('z', { ctrlKey: true }), { took: true, prevented: true }, 'a refused action still takes the key');
    assert.deepEqual(press('Delete'), { took: true, prevented: true });
    assert.deepEqual(press('Backspace'), { took: true, prevented: true });
    assert.deepEqual(press('1', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('!', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('=', {}), { took: true, prevented: true });
    assert.deepEqual(press('+', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('-', {}), { took: true, prevented: true });
    assert.deepEqual(press('d', {}), { took: true, prevented: true });
    assert.deepEqual(press('D', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('Enter', {}, { control: true }), { took: false, prevented: false }, 'Enter on a button inside the canvas keeps its default');
    assert.deepEqual(press('Enter', {}), { took: true, prevented: true });
    assert.deepEqual(press('f', { altKey: true, shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('f', { altKey: true, shiftKey: true }, { editable: true }), { took: false, prevented: false }, 'Alt+Shift+F needs the focus outside a text field');
    assert.deepEqual(press('c', { ctrlKey: true }), { took: true, prevented: true });
    host.selection.clear();
    assert.deepEqual(press('c', { ctrlKey: true }), { took: false, prevented: false }, 'Copy with nothing selected leaves the browser its own copy');
    host.selection.add('a');
    assert.deepEqual(press('d', { ctrlKey: true }), { took: false, prevented: false }, 'an action nobody bound takes no key');
    assert.deepEqual(press('x', {}), { took: false, prevented: false });
    assert.deepEqual(press('s', { metaKey: true }), { took: false, prevented: false }, 'Cmd is no Ctrl here (isMod is ctrlKey off macOS)');
    assert.deepEqual(observed, ['flow.save', 'flow.save', 'flow.test', 'edit.redo', 'edit.redo', 'edit.delete', 'edit.delete', 'view.zoom_fit', 'view.zoom_fit', 'view.zoom_in', 'view.zoom_in', 'view.zoom_out', 'step.toggle', 'step.toggle', 'step.open', 'edit.auto_layout', 'edit.copy']);
    stop();
    assert.deepEqual(calls.filter(id => id === 'edit.undo'), [], 'a refused undo never ran');
    observed = [];
    press('s', { ctrlKey: true });
    assert.deepEqual(observed, [], 'observe stops');

    // the table: actions with their shortcut, gestures as written
    const layout = [['g', ['edit.undo', 'flow.home', ['Space', 'easydrag.ui.keys_pan'], 'view.search']]];
    assert.deepEqual(A.table(layout, { host }), [['g', [['Ctrl+Z', 'easydrag.ui.undo'], ['Space', 'easydrag.ui.keys_pan'], ['Ctrl+K', 'easydrag.ui.keys_search']]]]);
    A.bind('view.search', { run() {}, label: () => 'easydrag.ui.keys_add' }, host);
    assert.equal(A.table(layout, { host })[0][1][2][1], 'easydrag.ui.keys_add');
}

// 5. A global binding serves every host; a host binding wins.
{
    const host = makeHost();
    A.bind('view.liveliness_normal', { run() {}, checked: () => true });
    A.bind('view.liveliness_calm', { run() {}, checked: () => false });
    const [parent] = A.items(['view.liveliness'], { host });
    assert.equal(parent.label, 'Liveliness');
    assert.deepEqual(parent.items.map(e => [e.id, e.checked, e.shortcutHint]), [['liveliness-normal', true, String.fromCodePoint(0x2713)], ['liveliness-calm', false, '']]);
}
console.log('PASS action registry: ' + A.ROWS.length + ' rows');
```

- [ ] **Step 1.2: run it and see it fail for the right reason**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-actions.mjs 2>&1 | head -6
```
Expected: `ENOENT … easydrag-actions.js` (the module does not exist yet).

- [ ] **Step 1.3: apply the key file**

The coordinator has committed `plans/2026-11-15-easydrag-w1b/keys/a-1.json` to main; if the worktree was cut before that, `git merge --ff-only main` brings it (it touches only `plans/`). Validate it, then apply:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node -e "const j=JSON.parse(require('fs').readFileSync('plans/2026-11-15-easydrag-w1b/keys/a-1.json','utf8'));const L='cs da de el en es fr hi it ja nl no pl pt sv zh'.split(' ');const bad=Object.entries(j).filter(([k,v])=>L.some(l=>typeof v[l]!=='string'||!v[l].trim()));console.log(Object.keys(j).length,'keys, incomplete:',bad.length)"
node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/a-1.json
git diff --stat -- ui/lang/easydrag | tail -3
```
Expected: `20 keys, incomplete: 0`; `a-1.json: 20 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`; 16 files changed with 20 insertions each.

- [ ] **Step 1.4: write the module**

Create `ui/js/desktop/apps/easydrag-actions.js`:

```js
// EasyDrag action registry. One table of every action (label, icon, shortcut, risk and the shared checks)
// and the logic the owning modules bind to it. Menus, card tools, header buttons, the shortcut table and
// the keyboard list action ids and read this module, so an action has one name, one shortcut and one
// reason for being unavailable on every surface.
//
// A host is the object an action works on: the editor state (ed), the start page or the shell instance.
// The host carries t (the translator), readonly, and optionally actionFacts(): extra facts of the moment.
// A context is { host, ...extras }: extras are flow (the card's flow on the start page), node (the card a
// card menu or tool belongs to), x and y (a point), keyed (window menu entries carry label keys), cached (read the
// host's facts once per turn: a render of many cards), and for keys the facts of the keystroke (see key).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MARK = String.fromCodePoint(0x2713);
    const MINUS = String.fromCodePoint(0x2212);
    const DASH = String.fromCodePoint(0x2013);
    const RISKS = ['safe', 'undoable', 'confirm'];
    const SCOPES = ['editor', 'canvas'];

    // Row: [id, label key, menu icon, glyph (core.ICONS name for in-app icon buttons, or ''), shortcut, scope,
    // risk, shared checks, legacy window-menu id (or '')]. The label keys are literals on purpose: the locale
    // tests find literal keys.
    const TABLE = [
        ['flow.home', 'easydrag.ui.back_home', 'list', 'arrow-left', '', 'editor', 'safe', [], 'home'],
        ['flow.save', 'easydrag.ui.save_now', 'save', '', 'Ctrl+S', 'editor', 'safe', ['writable', 'free'], 'save'],
        ['flow.test', 'easydrag.ui.test_draft', 'flask', '', 'Ctrl+Enter', 'editor', 'safe', ['writable', 'free', 'idle'], 'test'],
        ['flow.run_now', 'easydrag.ui.run_now', 'play', '', '', 'editor', 'safe', ['writable', 'free', 'published', 'active'], 'run'],
        ['flow.stop', 'easydrag.ui.run_cancel', 'stop', '', '', 'editor', 'safe', [], ''],
        ['flow.publish', 'easydrag.ui.publish', 'upload', '', '', 'editor', 'safe', ['writable', 'free'], 'publish'],
        ['flow.active', 'easydrag.ui.active', 'check', '', '', 'editor', 'safe', ['writable'], ''],
        ['flow.versions', 'easydrag.ui.versions_title', 'history', '', '', 'editor', 'safe', ['free'], 'versions'],
        ['flow.settings', 'easydrag.ui.flow_settings', 'settings', '', '', 'editor', 'safe', ['free'], 'settings'],
        ['flow.organize', 'easydrag.ui.organize', 'folder', '', '', 'editor', 'safe', ['writable', 'free'], ''],
        ['flow.describe', 'easydrag.ui.authoring_title', 'sparkles', '', '', 'editor', 'safe', ['writable', 'free', 'idle'], ''],
        ['flow.repair_mission', 'easydrag.ui.repair_mission', 'link', '', '', 'editor', 'safe', ['writable', 'free'], ''],
        ['flow.duplicate', 'easydrag.ui.home_duplicate', 'copy', '', '', 'editor', 'safe', ['unlocked'], ''],
        ['flow.export', 'easydrag.ui.home_export', 'download', '', '', 'editor', 'safe', [], 'export'],
        ['flow.export_pack', 'easydrag.ui.pack_export', 'download', '', '', 'editor', 'safe', ['free'], 'export-pack'],
        ['flow.export_email', 'easydrag.ui.export_email', 'mail', '', '', 'editor', 'safe', [], ''],
        ['flow.mission_control', 'easydrag.ui.home_mission_control', 'list', '', '', 'editor', 'safe', [], ''],
        ['flow.delete', 'easydrag.ui.home_delete', 'trash', '', '', 'editor', 'confirm', ['unlocked', 'free'], 'delete'],
        ['flow.open', 'easydrag.ui.home_open', 'edit', '', '', 'editor', 'safe', [], ''],
        ['flow.open_draft', 'easydrag.ui.block_open_draft', 'pencil', '', '', 'editor', 'safe', [], ''],
        ['flow.new', 'easydrag.ui.home_new', 'plus', '', '', 'editor', 'safe', ['unlocked'], 'new'],
        ['flow.import', 'easydrag.ui.home_import', 'upload', '', '', 'editor', 'safe', ['unlocked'], 'import'],
        ['flow.reload', 'easydrag.ui.reload', 'refresh', '', '', 'editor', 'safe', [], 'reload'],
        ['edit.undo', 'easydrag.ui.undo', 'undo', 'undo', 'Ctrl+Z', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'undo'],
        ['edit.redo', 'easydrag.ui.redo', 'redo', 'redo', 'Ctrl+Shift+Z', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'redo'],
        ['edit.cut', 'easydrag.ui.cut', 'scissors', '', 'Ctrl+X', 'canvas', 'undoable', ['writable', 'free', 'quiet', 'selection'], 'cut'],
        ['edit.copy', 'easydrag.ui.copy', 'copy', '', 'Ctrl+C', 'canvas', 'safe', ['free', 'quiet', 'selection'], 'copy'],
        ['edit.paste', 'easydrag.ui.paste', 'clipboard', '', 'Ctrl+V', 'canvas', 'safe', ['writable', 'free', 'quiet'], 'paste'],
        ['edit.duplicate', 'easydrag.ui.duplicate', 'copy', 'copy', 'Ctrl+D', 'canvas', 'safe', ['writable', 'free', 'quiet', 'selection'], 'duplicate'],
        ['edit.delete', 'easydrag.ui.delete', 'trash', 'trash', 'Delete', 'canvas', 'undoable', ['writable', 'free', 'quiet'], 'delete-sel'],
        ['edit.select_all', 'easydrag.ui.select_all', 'check-square', '', 'Ctrl+A', 'canvas', 'safe', ['free', 'quiet'], 'select-all'],
        ['edit.extract', 'easydrag.ui.extraction_title', 'grid', '', '', 'canvas', 'safe', ['writable', 'free', 'quiet', 'selection'], 'extract-selection'],
        ['edit.auto_layout', 'easydrag.ui.auto_layout', 'sparkles', '', 'Alt+Shift+F', 'editor', 'safe', ['writable', 'free', 'steps2'], 'auto-layout'],
        ['edit.group', 'easydrag.ui.group_create', 'grid', '', '', 'canvas', 'safe', ['writable'], ''],
        ['edit.add_step', 'easydrag.ui.menu_add_node', 'plus', '', '', 'canvas', 'safe', ['writable'], ''],
        ['step.open', 'easydrag.ui.menu_open_node', 'edit', '', 'Enter', 'canvas', 'safe', [], ''],
        ['step.test', 'easydrag.ui.node_test', 'play', 'flask', '', 'canvas', 'safe', ['writable', 'idle', 'selection'], ''],
        ['step.toggle', 'easydrag.ui.keys_disable', 'pause', 'eye-off', 'D', 'canvas', 'safe', ['writable', 'selection'], ''],
        ['step.connect', 'easydrag.ui.keys_connect', 'link', 'link', 'C', 'canvas', 'safe', ['writable', 'selection'], ''],
        ['step.add_next', 'easydrag.ui.port_add', 'plus', 'plus', '', 'canvas', 'safe', ['writable'], ''],
        ['step.align_horizontal', 'easydrag.ui.align_horizontal', 'sliders', '', '', 'canvas', 'safe', ['writable'], ''],
        ['step.align_vertical', 'easydrag.ui.align_vertical', 'sliders', '', '', 'canvas', 'safe', ['writable'], ''],
        ['view.zoom_in', 'easydrag.ui.zoom_in', 'zoom-in', '', '+', 'canvas', 'safe', [], 'zoom-in'],
        ['view.zoom_out', 'easydrag.ui.zoom_out', 'zoom-out', '', '-', 'canvas', 'safe', [], 'zoom-out'],
        ['view.zoom_fit', 'easydrag.ui.zoom_fit', 'maximize', '', 'Shift+1', 'canvas', 'safe', [], 'zoom-fit'],
        ['view.zoom_reset', 'easydrag.ui.keys_zoom_reset', 'maximize', '', 'Ctrl+0', 'canvas', 'safe', [], ''],
        ['view.find_step', 'easydrag.ui.find_step', 'search', '', 'Ctrl+F', 'editor', 'safe', ['free'], 'find-step'],
        ['view.search', 'easydrag.ui.keys_search', 'search', '', 'Ctrl+K', 'editor', 'safe', ['free'], 'search'],
        ['view.palette', 'easydrag.ui.palette_title', 'sidebar', '', '', 'editor', 'safe', [], 'palette'],
        ['view.runs', 'easydrag.ui.runs_title', 'history', '', '', 'editor', 'safe', [], 'runs'],
        ['view.shortcuts', 'easydrag.ui.keys_title', 'help', '', '?', 'canvas', 'safe', ['free'], 'keys'],
        ['view.liveliness', 'easydrag.ui.liveliness', 'sparkles', '', '', 'editor', 'safe', [], ''],
        ['view.liveliness_normal', 'easydrag.ui.liveliness_normal', 'play', '', '', 'editor', 'safe', [], 'liveliness-normal'],
        ['view.liveliness_calm', 'easydrag.ui.liveliness_calm', 'pause', '', '', 'editor', 'safe', [], 'liveliness-calm']
    ];

    const ROWS = Object.freeze(TABLE.map(([id, label, icon, glyph, shortcut, scope, risk, needs, menu]) =>
        Object.freeze({ id, label, icon, glyph, shortcut, scope, risk, needs: Object.freeze(needs.slice()), menu })));
    const BY_ID = new Map(ROWS.map(row => [row.id, row]));

    // REASONS: each cause of a shared check has one reason key, so the text is the same on every surface.
    const REASONS = Object.freeze({
        run_view: 'easydrag.ui.reason_run_view',
        readonly: 'easydrag.ui.reason_readonly',
        locked: 'easydrag.ui.reason_locked',
        dialog: 'easydrag.ui.reason_dialog_open',
        restoring: 'easydrag.ui.reason_restoring',
        selection: 'easydrag.ui.reason_no_selection',
        published: 'easydrag.ui.reason_not_published',
        paused: 'easydrag.ui.reason_paused',
        running: 'easydrag.ui.reason_running',
        steps2: 'easydrag.ui.reason_two_steps'
    });

    // The shared checks. Each reads the facts of the host (see facts) and the context, and returns a reason key
    // or ''. writable: nothing may be changed (a past run is open, read-only mode, the flows are locked).
    // unlocked: the same without the run view, for actions on the whole flow that a past run does not stop.
    // free: no dialog, context menu or restore offer is open. quiet: quick-add is closed (edits wait for it). selection: a step is selected (or the context names one).
    const CHECKS = Object.freeze({
        writable: f => (f.runView ? REASONS.run_view : f.readonly ? REASONS.readonly : f.locked ? REASONS.locked : ''),
        unlocked: f => (f.readonly ? REASONS.readonly : f.locked ? REASONS.locked : ''),
        free: f => (f.dialog ? REASONS.dialog : f.restoring ? REASONS.restoring : ''),
        quiet: f => (f.quickAdd ? REASONS.dialog : ''),
        selection: (f, c) => (c.node || f.selection > 0 ? '' : REASONS.selection),
        published: (f, c) => ((c.flow ? c.flow.published : f.published) ? '' : REASONS.published),
        active: (f, c) => {
            const published = c.flow ? c.flow.published : f.published;
            const enabled = c.flow ? c.flow.enabled : f.enabled;
            return published && !enabled ? REASONS.paused : '';
        },
        idle: f => (f.running ? REASONS.running : ''),
        steps2: f => (f.steps >= 2 ? '' : REASONS.steps2)
    });

    // Shell-dispatched shortcuts: the desktop's window-menu dispatch fires them (the menu item carries
    // `shortcut`); the desktop's own search would take Ctrl+K otherwise. Every other shortcut is only displayed
    // by the menus (`shortcutHint`) and fired by key().
    const DESKTOP_KEYS = new Set(['flow.save', 'flow.test', 'view.search']);
    // These work from a text field as well (they never type a character). Every other key needs the focus
    // outside a text field.
    const TEXT_KEYS = new Set(['flow.save', 'flow.test', 'view.search', 'view.find_step']);
    // Ctrl+C with nothing selected is not taken from the browser (its own copy goes on).
    const PASS_WHEN_REFUSED = new Set(['edit.copy']);
    const ALIASES = [['edit.redo', 'Ctrl+Y']];
    const SUBMENUS = { 'view.liveliness': ['view.liveliness_normal', 'view.liveliness_calm'] };
    const MARKED = new Set(['view.liveliness_normal', 'view.liveliness_calm']);

    // TOOLS: the card tools in the order the card draws them: [data-ed-node-tool value, action id].
    const TOOLS = Object.freeze([['add', 'step.add_next'], ['connect', 'step.connect'], ['test', 'step.test'], ['disable', 'step.toggle'], ['duplicate', 'edit.duplicate'], ['delete', 'edit.delete']]);

    const hosts = new WeakMap();
    const globals = new Map();
    let observer = null;

    // bind attaches what an action does: impl is { run(ctx), check(ctx), label(ctx), show(ctx), checked(ctx),
    // accepts(event, where) }. check returns a reason key or nothing, label an other label key or nothing, show
    // false leaves the action out of menus and toolbars, checked draws a check mark, accepts false lets a key
    // pass to the next handler. Without a host the binding is global (the same for every host).
    function bind(id, impl, host) {
        if (!BY_ID.has(id)) throw new Error('unknown action ' + id);
        if (!host) { globals.set(id, impl); return; }
        let map = hosts.get(host);
        if (!map) { map = new Map(); hosts.set(host, map); }
        map.set(id, impl);
    }

    function implOf(host, id) {
        const map = host && hosts.get(host);
        return (map && map.get(id)) || globals.get(id) || null;
    }

    function row(id) { return BY_ID.get(id) || null; }

    // facts are read fresh, except for a caller that draws many buttons in one go (ctx.cached): they are kept for the turn of the
    // event loop. run() and key() never use the kept ones.
    const kept = new WeakMap();
    let epoch = 0;
    let ticking = false;

    function facts(host, cached) {
        const old = cached && host ? kept.get(host) : null;
        if (old && old.epoch === epoch) return old.value;
        const value = readFacts(host);
        if (cached && host) {
            kept.set(host, { epoch, value });
            if (!ticking) { ticking = true; Promise.resolve().then(() => { epoch++; ticking = false; }); }
        }
        return value;
    }

    // readFacts reads what the shared checks need from a host. The defaults come from the editor state; a host
    // overrides or adds with actionFacts() (restoring, dialog, running, locked ...).
    function readFacts(host) {
        const h = host || {};
        const nodes = h.model && h.model.doc && h.model.doc.nodes;
        const base = {
            readonly: !!h.readonly, runView: !!h.runView, locked: false, restoring: false, dialog: false, quickAdd: !!h.quickAdd, running: false,
            selection: h.selection ? h.selection.size : 0, steps: nodes ? nodes.length : 0,
            published: !!(h.flow && h.flow.live), enabled: !!h.flowEnabled
        };
        return typeof h.actionFacts === 'function' ? Object.assign(base, h.actionFacts()) : base;
    }

    function translator(c) { return c.t || (c.host && c.host.t) || (key => key); }

    function reasonOf(c, key) { return key ? { enabled: false, reason: translator(c)(key) } : null; }

    // state says whether an action may run now, and if not, why (a translated text).
    function state(id, ctx) {
        const c = ctx || {};
        const r = BY_ID.get(id);
        const impl = r && implOf(c.host, id);
        if (!impl) return { enabled: false, reason: '' };
        const f = facts(c.host, !!c.cached);
        for (const need of r.needs) {
            const refused = reasonOf(c, CHECKS[need](f, c));
            if (refused) return refused;
        }
        return reasonOf(c, impl.check ? impl.check(c) : '') || { enabled: true, reason: '' };
    }

    // run checks the state again and runs the action only when it is enabled. It returns false when the
    // action is refused, and what the action returns otherwise.
    function run(id, ctx) {
        const c = ctx || {};
        if (!state(id, c).enabled) return false;
        if (observer) observer(id);
        return implOf(c.host, id).run(c);
    }

    // observe lets a test count the runs; fn(id) is called right before an enabled action runs.
    function observe(fn) { observer = typeof fn === 'function' ? fn : null; return () => { if (observer === fn) observer = null; }; }

    function labelKey(id, c) {
        const impl = implOf(c.host, id);
        return (impl && impl.label && impl.label(c)) || BY_ID.get(id).label;
    }

    function label(id, ctx) {
        const c = ctx || {};
        return translator(c)(labelKey(id, c));
    }

    // display is the shortcut as the surfaces draw it: ⌘ and ⇧ on macOS, the localized name of Delete.
    function display(shortcut, t) {
        if (!shortcut) return '';
        if (shortcut === 'Delete') return t ? t('easydrag.ui.key_delete') : 'Del';
        if (shortcut === '-') return MINUS;
        return ED.core ? ED.core.shortcut(shortcut) : shortcut;
    }

    function shown(id, impl, c) { return !!impl && !(impl.show && !impl.show(c)); }

    // named gives a menu entry its words: the translated label, or with ctx.keyed the label key, which the desktop
    // translates when it draws the menu (window menus follow a language change that way).
    function named(entry, id, c) {
        if (c.keyed) entry.labelKey = labelKey(id, c); else entry.label = label(id, c);
        return entry;
    }

    function entryOf(id, c) {
        const r = BY_ID.get(id);
        if (!r) throw new Error('unknown action ' + id);
        if (SUBMENUS[id]) {
            const kids = items(SUBMENUS[id], c);
            return kids.length ? named({ id: id.replace('.', '-'), icon: r.icon, items: kids }, id, c) : null;
        }
        const impl = implOf(c.host, id);
        if (!shown(id, impl, c)) return null;
        const st = state(id, c);
        const keys = display(r.shortcut, translator(c));
        const entry = named({ id: r.menu || r.id, icon: r.icon, disabled: !st.enabled, action: () => run(id, c) }, id, c);
        if (!st.enabled && st.reason) entry.disabledHint = st.reason;
        if (keys) { if (DESKTOP_KEYS.has(id)) entry.shortcut = r.shortcut; else entry.shortcutHint = keys; }
        if (r.risk !== 'safe') entry.danger = true;
        if (impl.checked) {
            entry.checked = !!impl.checked(c);
            if (MARKED.has(id)) entry.shortcutHint = entry.checked ? MARK : '';
        }
        return entry;
    }

    // items turns action ids into desktop menu entries (showContextMenu and setWindowMenus format; ctx.keyed: labelKey
    // instead of label, for window menus). '-' is a
    // separator; an action nobody bound here, or one that says show() is false, is left out, and a separator
    // never leads, trails or doubles.
    function items(ids, ctx) {
        const c = ctx || {};
        const out = [];
        ids.forEach(id => {
            if (id === '-') {
                if (out.length && !out[out.length - 1].separator) out.push({ separator: true, type: 'separator' });
                return;
            }
            const entry = entryOf(id, c);
            if (entry) out.push(entry);
        });
        while (out.length && out[out.length - 1].separator) out.pop();
        return out;
    }

    // button returns the attributes of an icon button: aria-label, a title of the label and the shortcut, and
    // when the action is unavailable aria-disabled="true" with the reason in the title and in aria-description
    // (an aria-disabled button stays focusable, so a keyboard user learns why). null: not bound here.
    function button(id, ctx) {
        const c = ctx || {};
        const r = BY_ID.get(id);
        const impl = r && implOf(c.host, id);
        if (!shown(id, impl, c)) return null;
        const st = state(id, c);
        const name = label(id, c);
        const keys = display(r.shortcut, translator(c));
        const attrs = { 'aria-label': name, title: name + (keys ? ' (' + keys + ')' : '') };
        if (!st.enabled) {
            attrs['aria-disabled'] = 'true';
            if (st.reason) { attrs.title += ' ' + DASH + ' ' + st.reason; attrs['aria-description'] = st.reason; }
        }
        return attrs;
    }

    // paint writes button(id, ctx) onto an element. The accessible name is written only with opts.name (an icon
    // button); a button with visible text keeps its text as its name.
    function paint(el, id, ctx, opts) {
        const attrs = button(id, ctx);
        if (!el || !attrs) return false;
        Object.keys(attrs).forEach(name => { if (name !== 'aria-label' || (opts && opts.name)) el.setAttribute(name, attrs[name]); });
        if (!attrs['aria-disabled']) { el.removeAttribute('aria-disabled'); el.removeAttribute('aria-description'); }
        return true;
    }

    // attrString renders an attribute object as markup (for modules that build innerHTML).
    function attrString(attrs) {
        return Object.keys(attrs).map(name => ' ' + name + '="' + ED.core.esc(attrs[name]) + '"').join('');
    }

    // parse reads a canonical shortcut ("Ctrl+Shift+Z", "Shift+1", "+", "D") into modifiers and a key.
    function parse(shortcut) {
        const m = { ctrl: false, shift: false, alt: false, key: '' };
        let rest = shortcut;
        for (;;) {
            if (rest.startsWith('Ctrl+')) { m.ctrl = true; rest = rest.slice(5); }
            else if (rest.startsWith('Shift+')) { m.shift = true; rest = rest.slice(6); }
            else if (rest.startsWith('Alt+')) { m.alt = true; rest = rest.slice(4); }
            else break;
        }
        m.key = rest.toLowerCase();
        return m;
    }

    // The keystrokes, most specific first (Ctrl+Shift+Z before Ctrl+Z).
    const KEYS = ROWS.filter(r => r.shortcut).map(r => ({ row: r, m: parse(r.shortcut) }))
        .concat(ALIASES.map(([id, shortcut]) => ({ row: BY_ID.get(id), m: parse(shortcut) })))
        .sort((a, b) => (b.m.shift + b.m.alt) - (a.m.shift + a.m.alt));

    // matches compares a keydown with a parsed shortcut. A modifier the shortcut does not name is ignored,
    // except Ctrl (the platform's Cmd or Ctrl, ED.core.isMod): a plain key never fires with it held.
    function matches(m, event) {
        if (m.ctrl !== ED.core.isMod(event)) return false;
        if (m.shift && !event.shiftKey) return false;
        if (m.alt && !event.altKey) return false;
        const key = String(event.key).toLowerCase();
        if (m.key === '+') return key === '+' || key === '=';
        if (m.key === '1' && m.shift) return key === '1' || key === '!';
        if (m.key === 'delete') return key === 'delete' || key === 'backspace';
        return key === m.key;
    }

    // key is the one keyboard path for registry shortcuts. where: { host, editable (the target is a text field),
    // quickAdd, onCanvas (the focus is on the canvas, a card or the page), control (the target is a button, link or
    // field inside the canvas) }. It returns true when it took the keystroke: the browser's default is prevented
    // and the action ran once, or was refused with its reason. The shell fires its own shortcuts first and prevents
    // them; the editor does not call key for a prevented event.
    function key(event, where) {
        const w = where || {};
        if (!event || typeof event.key !== 'string') return false;
        for (const entry of KEYS) {
            if (!matches(entry.m, event)) continue;
            const r = entry.row;
            const impl = implOf(w.host, r.id);
            if (!impl) continue;
            if (r.scope === 'editor' ? w.editable && !TEXT_KEYS.has(r.id) : w.editable || w.quickAdd || !w.onCanvas) continue;
            if (impl.accepts && !impl.accepts(event, w)) continue;
            if (!state(r.id, w).enabled && PASS_WHEN_REFUSED.has(r.id)) return false;
            event.preventDefault();
            run(r.id, w);
            return true;
        }
        return false;
    }

    // table builds the shortcut table: layout is [[group key, [action id or [shortcut, label key]]]]. A literal
    // pair is a gesture that is no action (arrows, Space, Esc). Actions without a shortcut are left out.
    function table(layout, ctx) {
        const c = ctx || {};
        return layout.map(([group, entries]) => [group, entries.map(entry => {
            if (Array.isArray(entry)) return entry;
            const r = BY_ID.get(entry);
            return r && r.shortcut ? [r.shortcut, labelKey(entry, c)] : null;
        }).filter(Boolean)]);
    }

    ED.actions = { ROWS, REASONS, CHECKS, TOOLS, bind, state, run, items, button, paint, attrString, key, row, label, display, table, observe };
})();
```

The module is plain data and functions: it needs `ED.core` only inside `display` and `matches` (called at run time), so it loads right after `easydrag-icons.js` and before every module that binds or reads it.

- [ ] **Step 1.5: run the test and see it pass**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-actions.mjs
```
Expected: `PASS action registry: 54 rows`. If a row's label key is reported missing, the key file was not applied (Step 1.3). If a `menu` id or shortcut assertion fails, the table in the module differs from the spec's list: fix the table, not the test.

- [ ] **Step 1.6: register the module in the three places and the Node harness lists**

Write `C:/disposable/easydrag-w1b/patches/a1-patch.mjs`:

```js
// W1b A.1: register easydrag-actions.js in the three places (module loader, Go list, Node harness lists).
// Run from the worktree root: node C:/disposable/easydrag-w1b/a1-patch.mjs
import { edit } from '../lib-patch.mjs';

edit('ui/js/desktop/core/module-loader.js', `                '/js/desktop/apps/easydrag-icons.js',
`, `                '/js/desktop/apps/easydrag-icons.js',
                '/js/desktop/apps/easydrag-actions.js',
`);
edit('ui/desktop_easydrag_test.go', `"easydrag-icons.js", "easydrag-delight.js"`, `"easydrag-icons.js", "easydrag-actions.js", "easydrag-delight.js"`);
// The Node harnesses that load modules: the registry goes in right after the status module (before delight, which binds to it).
for (const file of ['test-easydrag-extra.mjs', 'test-easydrag-extra2.mjs', 'test-easydrag-extra3.mjs']) edit('scripts/' + file, `'easydrag-status.js', `, `'easydrag-status.js', 'easydrag-actions.js', `);
edit('scripts/test-easydrag-extra4.mjs', `'easydrag-status.js', `, `'easydrag-status.js', 'easydrag-actions.js', `, 2);
edit('scripts/test-easydrag.mjs', `for (const file of ['easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-library-pause.js',`, `for (const file of ['easydrag-actions.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-library-pause.js',`);
console.log('A.1 patch applied');
```
Run it, then prove the registration:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a1-patch.mjs
grep -c "easydrag-actions.js" ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag.mjs scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)TestEasyDragAssetsLoadInDependencyOrder' -count=1 2>&1 | tail -3
```
Expected counts 1, 1, 1, 1, 1, 1, 2 (extra4 has two lists); the Go test `ok`. The Go test fails with "assets miss easydrag-actions.js" when the loader line is missing and with an order message when the module sits after one that needs it.

- [ ] **Step 1.7: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_test.go
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no vet output, no gofmt output. (`node scripts/test-easydrag.mjs` runs about three minutes: it runs every extra script.)

- [ ] **Step 1.8: commit**

Write `C:/disposable/easydrag-w1b/a1-message.txt`:
```
feat(easydrag): action registry module with shared checks and reasons

easydrag-actions.js holds one row per action (label, icon, shortcut, scope,
risk, shared checks), the shared checks with one reason text per cause, and
bind, state, run, items, button, paint, key and table. Nothing uses it yet.
Registered in the module loader, the Go order list and the Node harness lists.
Twenty new locale keys: the reasons (en, de and 14 locales) and export_email
and key_delete.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && git add ui/js/desktop/apps/easydrag-actions.js scripts/test-easydrag-actions.mjs ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag.mjs scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1b/a1-message.txt && git show --stat HEAD | tail -4
```
Expected: one commit; 9 source files and 16 locale files.

---

### Task A.2: card tools, card menu and canvas menu through the registry; the editing actions bound in `interact`

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-interact.js` (`runNodeTool` 458-479, `openNodeMenu` 526-553, `openCanvasMenu` 555-568; new action block before `storedFragment();` at 726)
- Modify: `ui/js/desktop/apps/easydrag-canvas.js` (`cardMarkup` 477-484, `toolButton` 494-498)
- Modify: `ui/js/desktop/apps/easydrag-extraction.js` (one bind line after 196)
- Modify: `ui/css/desktop-app-easydrag.css` (two rules after line 853; the file has CRLF endings, the patch helper keeps them)
- Modify: `scripts/test-easydrag-extra.mjs` (one expected count: the bus now has one more subscription)

**Design.**
- **Bindings in `interact`.** The functions the actions run (`undo`, `redo`, `copySelection`, `pasteAt`, `removeSelection`, `duplicateSteps`, `toggleSteps`, `selectAll`) are closures of `interact.create`, so it binds `edit.*`, `step.*` and `view.zoom_*` on the editor state `ed` (`ED.actions.bind(id, impl, ed)`). The Edit and View window menus of A.3 use these bindings. The block editor gets the same bindings; `step.test` is `show: () => !ed.isBlock`, and `edit.extract` is bound by the extraction module, which exists only in a flow editor, so each is left out of a block's menus without a special case.
- **Context.** A card menu or tool passes `{ host: ed, node: id }`; the menu selects its card first (as today). A tool runs `runNodeTool`: it checks the state for its card, selects the card unless it is the only one selected, then runs the registry action, which acts on the selection like the card menu does. Today a tool acts on its own card whatever else is selected; after this task selecting the card first makes that the same thing.
- **Repaint.** A tool is painted when its card is built. The one state that changes later is `idle` (Test this step while a run goes), so `interact` repaints the selected cards' tools on the editor's `run` event. `run()` checks again at click time, so a tool that looks stale does nothing wrong.
- **Paste's "nothing to paste".** Paste reads the system clipboard asynchronously, so its availability is a conservative check: unavailable only when there is neither a fresh stored fragment nor a clipboard API.
- **Canvas menu.** `edit.paste` and `edit.add_step` get the pointer from the context (`x`, `y`, client coordinates): the canvas menu pastes and adds at the pointer, the Edit menu (A.3) at the view centre, as today. "Einfügen" replaces "Hier einfügen".
- **No key change yet.** `handleKey` keeps all its keys in this task; A.5 moves them.

- [ ] **Step 2.1: impact**

Run the IMPACT command for `openNodeMenu`, `openCanvasMenu`, `runNodeTool`, `cardMarkup` and `toolButton`. Expected: callers inside `easydrag-interact.js` and `easydrag-canvas.js` only (the menus are opened by `onContextMenu` and the long press, `cardMarkup` by `renderNode`). If the graph says UNKNOWN, confirm with `grep -rn "openNodeMenu\|openCanvasMenu\|runNodeTool\|toolButton" ui/js scripts ui/*.go`: only the two files and no test.

- [ ] **Step 2.2: write and run the patch**

Write `C:/disposable/easydrag-w1b/patches/a2-patch.mjs`:

```js
// W1b A.2: the card tools, the card menu and the canvas menu go through the action registry; the editing actions are bound in interact.
// Run from the worktree root: node C:/disposable/easydrag-w1b/a2-patch.mjs
import { edit } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-interact.js ────────────────────────────────────────────────────────

const interact = apps + 'easydrag-interact.js';

// 1. A card tool selects its card and runs the same action as the card menu.
edit(interact, `        function runNodeTool(action, id) {
            if (readonly()) return;
            if (action === 'add') {
                const n = ed.model.node(id);
                const outs = n ? ed.model.outputs(n) : [];
                const card = canvas.nodeEl(id);
                if (!n || !outs.length || !card) return;
                const rect = card.getBoundingClientRect();
                ed.bus.emit('quick-add', { x: rect.right + 8, y: rect.top, from: { node: id, port: outs[0] } });
            } else if (action === 'connect') ed.bus.emit('connect-picker', { nodeId: id });
            else if (action === 'test') ed.bus.emit('node-test', { nodeId: id });
            else if (action === 'disable') toggleSteps([id]);
            else if (action === 'duplicate') duplicateSteps([id]);
            else if (action === 'delete') removeSteps([id]);
        }
`, `        // runNodeTool: a card tool selects its card (unless it is the only one selected) and runs the registry action
        // the card menu runs; run() checks the state again, so a tool that looks unavailable does nothing.
        function runNodeTool(tool, id) {
            const found = ED.actions.TOOLS.find(([name]) => name === tool);
            if (!found || !ED.actions.state(found[1], { host: ed, node: id }).enabled) return;
            if (!(ed.selection.size === 1 && ed.selection.has(id))) select([id]);
            ED.actions.run(found[1], { host: ed, node: id });
        }
`);

// 2. The card menu and the canvas menu list action ids.
edit(interact, `        function openNodeMenu(id, x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            if (!ed.selection.has(id)) select([id]);
            const n = ed.model.node(id);
            const ro = readonly();
            const items = [
                { icon: 'edit', label: t('easydrag.ui.menu_open_node'), action: () => ed.bus.emit('open-detail', { nodeId: id }) },
                ...(!ed.isBlock ? [{ icon: 'play', label: t('easydrag.ui.node_test'), disabled: ro, action: () => ed.bus.emit('node-test', { nodeId: id }) }] : []),
                { separator: true },
                { icon: 'copy', label: t('easydrag.ui.node_duplicate'), disabled: ro, action: () => duplicateSteps(Array.from(ed.selection)) },
                { icon: 'copy', label: t('easydrag.ui.menu_copy'), action: () => copySelection() },
                ...(ed.groups ? [{ icon: 'grid', label: t('easydrag.ui.group_create'), disabled: ro, action: () => ed.groups.createFromSelection() }] : []),
                ...(!ed.isBlock && ED.extraction ? [{ icon: 'grid', label: t('easydrag.ui.extraction_title'), disabled: ro, action: () => ed.bus.emit('extract-selection') }] : []),
                { icon: 'pause', label: n && n.settings.disabled ? t('easydrag.ui.node_enable') : t('easydrag.ui.node_disable'), disabled: ro, action: () => toggleSteps(Array.from(ed.selection)) }
            ];
            if (ed.selection.size >= 2) {
                items.push(
                    { separator: true },
                    { icon: 'sliders', label: t('easydrag.ui.align_horizontal'), disabled: ro, action: () => canvas.alignSelection('horizontal') },
                    { icon: 'sliders', label: t('easydrag.ui.align_vertical'), disabled: ro, action: () => canvas.alignSelection('vertical') }
                );
            }
            items.push(
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.node_delete'), disabled: ro, action: () => removeSelection() }
            );
            ed.ctx.showContextMenu(x, y, items);
        }

        function openCanvasMenu(x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            const ro = readonly();
            ed.ctx.showContextMenu(x, y, [
                { icon: 'plus', label: t('easydrag.ui.menu_add_node'), disabled: ro, action: () => ed.bus.emit('quick-add', { x, y }) },
                { icon: 'copy', label: t('easydrag.ui.menu_paste'), disabled: ro, action: () => pasteAt(canvas.clientToWorld(x, y)) },
                ...(!ed.isBlock && ED.extraction ? [{ icon: 'grid', label: t('easydrag.ui.extraction_title'), disabled: ro || !ed.selection.size, action: () => ed.bus.emit('extract-selection') }] : []),
                { separator: true },
                { icon: 'search', label: t('easydrag.ui.find_step'), disabled: !ed.model.doc.nodes.length, action: () => ed.bus.emit('find-step') },
                { icon: 'sparkles', label: t('easydrag.ui.auto_layout'), disabled: ro || ed.model.doc.nodes.length < 2, action: () => canvas.autoLayout() },
                { icon: 'list', label: t('easydrag.ui.menu_select_all'), action: selectAll },
                { icon: 'refresh', label: t('easydrag.ui.zoom_fit'), action: () => canvas.fit({ animate: true }) }
            ]);
        }
`, `        function openNodeMenu(id, x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            if (!ed.selection.has(id)) select([id]);
            ed.ctx.showContextMenu(x, y, ED.actions.items(['step.open', 'step.test', '-', 'edit.duplicate', 'edit.copy', 'edit.group', 'edit.extract', 'step.toggle', '-',
                'step.align_horizontal', 'step.align_vertical', '-', 'edit.delete'], { host: ed, node: id }));
        }

        // The canvas menu pastes and adds at the pointer (x and y are client coordinates).
        function openCanvasMenu(x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            ed.ctx.showContextMenu(x, y, ED.actions.items(['edit.add_step', 'edit.paste', 'edit.extract', '-', 'view.find_step', 'edit.auto_layout', 'edit.select_all', 'view.zoom_fit'], { host: ed, x, y }));
        }
`);

// 3. What the editing actions do is bound here, where their functions live.
edit(interact, `        // An expired fragment goes as soon as an editor opens, not only at the next paste.
`, `        // ── actions ─────────────────────────────────────────────────────────────

        // The registry (easydrag-actions.js) holds every action's label, shortcut and reasons; the editing actions
        // are bound here. The context's node is the card a menu or tool belongs to, else the one selected step.
        const stepOf = c => (c && c.node) || (ed.selection.size === 1 ? Array.from(ed.selection)[0] : '');
        const noStep = c => (stepOf(c) ? '' : 'easydrag.ui.reason_no_selection');
        const grouped = () => !!(ed.groups && ed.groups.active);
        const bindAction = (id, impl) => ED.actions.bind(id, impl, ed);
        bindAction('edit.undo', { run: () => undo(), check: () => (ed.model.canUndo() ? '' : 'easydrag.ui.reason_nothing_to_undo') });
        bindAction('edit.redo', { run: () => redo(), check: () => (ed.model.canRedo() ? '' : 'easydrag.ui.reason_nothing_to_redo') });
        bindAction('edit.cut', { run: () => { if (copySelection()) removeSelection({ quiet: true }); } });
        bindAction('edit.copy', { run: () => copySelection() });
        bindAction('edit.paste', {
            run: c => pasteAt(c.x === undefined ? null : canvas.clientToWorld(c.x, c.y)),
            check: () => (navigator.clipboard && navigator.clipboard.readText) || storedFragment() ? '' : 'easydrag.ui.reason_nothing_to_paste'
        });
        bindAction('edit.duplicate', { run: () => duplicateSteps(Array.from(ed.selection)) });
        bindAction('edit.delete', { run: () => removeSelection(), accepts: () => !grouped(), check: () => (ed.selection.size || ed.selectedEdge ? '' : 'easydrag.ui.reason_no_selection') });
        bindAction('edit.select_all', { run: () => selectAll() });
        bindAction('edit.group', { run: () => ed.groups.createFromSelection(), show: () => !!ed.groups });
        bindAction('edit.add_step', { run: c => ed.bus.emit('quick-add', { x: c.x, y: c.y }) });
        bindAction('edit.auto_layout', { run: () => canvas.autoLayout() });
        bindAction('step.open', { run: c => ed.bus.emit('open-detail', { nodeId: stepOf(c) }), check: noStep, accepts: (event, where) => !grouped() && !where.control });
        bindAction('step.connect', { run: c => ed.bus.emit('connect-picker', { nodeId: stepOf(c) }), check: noStep });
        bindAction('step.toggle', {
            run: () => toggleSteps(Array.from(ed.selection)),
            label: c => { const n = ed.model.node(stepOf(c)); return n ? (n.settings.disabled ? 'easydrag.ui.node_enable' : 'easydrag.ui.node_disable') : ''; }
        });
        bindAction('step.test', { run: c => ed.bus.emit('node-test', { nodeId: stepOf(c) }), show: () => !ed.isBlock });
        bindAction('step.add_next', {
            run: c => {
                const id = stepOf(c), card = canvas.nodeEl(id), n = ed.model.node(id), outs = n ? ed.model.outputs(n) : [];
                if (!card || !outs.length) return;
                const rect = card.getBoundingClientRect();
                ed.bus.emit('quick-add', { x: rect.right + 8, y: rect.top, from: { node: id, port: outs[0] } });
            },
            check: c => { const n = ed.model.node(stepOf(c)); return n && ed.model.outputs(n).length ? '' : 'easydrag.ui.reason_no_output'; }
        });
        bindAction('step.align_horizontal', { run: () => canvas.alignSelection('horizontal'), show: () => ed.selection.size >= 2 });
        bindAction('step.align_vertical', { run: () => canvas.alignSelection('vertical'), show: () => ed.selection.size >= 2 });
        bindAction('view.zoom_in', { run: () => canvas.zoomBy(1.2) });
        bindAction('view.zoom_out', { run: () => canvas.zoomBy(1 / 1.2) });
        bindAction('view.zoom_fit', { run: () => canvas.fit({ animate: true }) });
        bindAction('view.zoom_reset', { run: () => { const s = canvas.size(); canvas.setView(G.zoomAt(ed.view, 1, { x: s.w / 2, y: s.h / 2 }), { animate: true }); } });
        // A run starting or ending changes which tools may act (Test this step): the selected cards repaint theirs.
        bag.add(ed.bus.on('run', () => ed.selection.forEach(id => {
            const card = canvas.nodeEl(id);
            if (card) card.querySelectorAll('[data-ed-node-tool]').forEach(button => ED.actions.TOOLS.filter(([tool]) => tool === button.dataset.edNodeTool).forEach(([, action]) => ED.actions.paint(button, action, { host: ed, node: id }, { name: true })));
        })));

        // An expired fragment goes as soon as an editor opens, not only at the next paste.
`);

// ── easydrag-canvas.js ──────────────────────────────────────────────────────────

const canvas = apps + 'easydrag-canvas.js';
edit(canvas, `                toolButton('add', 'plus', t('easydrag.ui.port_add')) +
                toolButton('connect', 'link', t('easydrag.ui.keys_connect')) +
                (ed.isBlock ? '' : toolButton('test', 'flask', t('easydrag.ui.node_test'))) +
                toolButton('disable', 'eye-off', n.settings.disabled ? t('easydrag.ui.node_enable') : t('easydrag.ui.node_disable')) +
                toolButton('duplicate', 'copy', t('easydrag.ui.node_duplicate')) +
                toolButton('delete', 'trash', t('easydrag.ui.node_delete')) + '</div>';
`, `                ED.actions.TOOLS.map(([tool, action]) => toolButton(tool, action, n.id)).join('') + '</div>';
`);
edit(canvas, `        // Tool buttons start outside the tab order: one stop per action on every card would bury
        // the canvas. syncTools makes the selected card's tools tab stops.
        function toolButton(id, iconName, label) {
            return '<button type="button" class="ed-node-tool" tabindex="-1" data-ed-node-tool="' + id + '" title="' + esc(label) + '" aria-label="' + esc(label) + '">' + core.icon(iconName) + '</button>';
        }
`, `        // Tool buttons start outside the tab order: one stop per action on every card would bury
        // the canvas. syncTools makes the selected card's tools tab stops. Each tool is a registry action: its
        // label, shortcut and the reason it is unavailable come from ED.actions.button (null: not bound here).
        function toolButton(tool, action, id) {
            const attrs = ED.actions.button(action, { host: ed, node: id, cached: true });
            return attrs ? '<button type="button" class="ed-node-tool" tabindex="-1" data-ed-node-tool="' + tool + '"' + ED.actions.attrString(attrs) + '>' + core.icon(ED.actions.row(action).glyph) + '</button>' : '';
        }
`);

// ── easydrag-extraction.js: the owner of "Extract as a block" binds it ──────────

edit(apps + 'easydrag-extraction.js', `        bag.add(ed.bus.on('extract-selection', open));
`, `        bag.add(ed.bus.on('extract-selection', open));
        ED.actions.bind('edit.extract', { run: () => open() }, ed);
`);

// ── desktop-app-easydrag.css: an unavailable tool ───────────────────────────────

edit('ui/css/desktop-app-easydrag.css', `.ed-node-tool[data-ed-node-tool="delete"]:hover { color: var(--ed-danger); }
`, `.ed-node-tool[data-ed-node-tool="delete"]:hover { color: var(--ed-danger); }
.ed-node-tool[aria-disabled="true"] { opacity: 0.4; cursor: default; }
.ed-node-tool[aria-disabled="true"]:hover { background: transparent; color: var(--ed-muted); }
`);

// The editor's bus gets one more subscription (the tools repaint on a run), which the dispose test counts.
edit('scripts/test-easydrag-extra.mjs', `[[true, true, 10], origin, [], 0, 0, 0, { observed: 1, disconnected: 1 }]`, `[[true, true, 11], origin, [], 0, 0, 0, { observed: 1, disconnected: 1 }]`);

console.log('A.2 patch applied');
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a2-patch.mjs
```
Expected: `A.2 patch applied`. A "expected 1 match, found 0" error names the snippet that moved: re-read that site, fix the old text in the script to the site's current text (keep the new text), `git checkout -- <the files>` and run again.

- [ ] **Step 2.3: prove no old form remains**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
grep -n "label: t('easydrag.ui\|showContextMenu(x, y, \[\|toolButton('" ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js
grep -n "menu_copy\|menu_paste\|menu_select_all\|node_duplicate\|node_delete" ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js
grep -c "ED.actions" ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-extraction.js
wc -l ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js
```
Expected: the two greps print nothing; the counts (lines that mention `ED.actions`) are 7 / 4 / 1 (interact has the action block, canvas the tool markup and its button, extraction its one bind); `wc -l` says 765 for interact (753 before: temporarily above the baseline, A.5 takes it to 750) and 925 for canvas (928 before: stays at or below 928).

- [ ] **Step 2.4: run the Node checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-actions.mjs && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: `PASS action registry: 54 rows`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. This task changes only one existing expectation (`c1d04 dispose …` counts 11 bus subscriptions instead of 10: `interact` subscribes to the editor's `run` event once; the same test still checks that dispose leaves 0).

If a card-tool test fails (`scripts/test-easydrag-extra*.mjs` search the tools by `data-ed-node-tool`): a harness that builds a canvas without `interact` has no bound tools, so its cards draw none. That is by design (an unbound action has no button); give that harness's editor `ED.interact.create(ed, canvas, wires)` before the first render, as the editor does, and do not weaken the test.

- [ ] **Step 2.5: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-t0.mjs --node-only 2>&1 | tail -4
```
Expected: every Node job `ok`. Write `C:/disposable/easydrag-w1b/a2-message.txt`:
```
feat(easydrag): card tools, card menu and canvas menu use the action registry

The editing and zoom actions are bound in easydrag-interact.js. The card menu
and the canvas menu list action ids and render through ED.actions.items, the
card tools through ED.actions.button (aria-disabled with the reason, never
disabled) and run through ED.actions.run, which checks again. A tool selects its
card first and runs the same action as the card menu. Paste in the canvas menu now
reads "Einfuegen" like every other surface (the label key moves to the registry).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && git add ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-extraction.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-extra.mjs && git commit -F C:/disposable/easydrag-w1b/a2-message.txt && git show --stat HEAD | tail -7
```
Expected: one commit, five files.

---

### Task A.3: window menus, the "more" menu and the Lebendigkeit submenu through the registry; the flow and view actions bound in the editor

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (`runNowState` import 129; `moreMenu` 428-460; the window-menu section 484-561)
- Modify: `ui/js/desktop/apps/easydrag-delight.js` (`menuItems` 79-92, export line 94)
- Modify: `scripts/test-easydrag-delight.mjs`, `scripts/test-easydrag-extra3.mjs`, `scripts/test-easydrag-extra5.mjs`, `scripts/test-easydrag-extra6.mjs` (expected values that change with the registry, listed below)

**Design.**
- **Bindings.** The flow and view actions are owned by the editor (`saveNow`, `test`, `publishNow`, `runLiveNow`, `exportFlow`, `deleteFlow`, `duplicateFlow`, `repairMission`, `togglePalette`, `searchPalette`, the run drawer, the dialogs). The editor binds them on `ed`, in one block that replaces the window-menu section. A block editor gets the same block: what a block has not is not bound (`flow.run_now`, `flow.settings`, `flow.export`, `flow.duplicate`, `flow.active`, `view.runs`, `flow.organize`, `flow.describe`, `flow.export_email`, `flow.repair_mission`), so the five lines that filtered the block's menus by id (`['run', 'runs', 'settings', 'export']`, the `block_simulate` relabel) disappear. `flow.test` and `flow.delete` carry a bound `label` in a block ("Simulate block", "Delete block").
- **Facts.** `ed.actionFacts` supplies `restoring`, `dialog` (an `.ed-modal-backdrop` that is not closing, or a desktop context menu: the shell runs the window-menu shortcuts before `onKeyDown` can see that menu, which `unlessModal` guarded) and `running` (`runs.isRunning()`). The `unlessModal` and `unlessOverlay` wrappers and `runNowState` are replaced by the registry's `free`, `quiet`, `published` and `active` checks.
- **Window menus.** `setMenus` calls `ED.actions.items(ids, { host: ed, keyed: true })` for Flow, Edit and View. `keyed` makes the entries carry `labelKey`, as today, so the desktop translates them when it draws them.
- **Shortcuts in the menus.** The window menu keeps `shortcut` (the shell dispatches) for Ctrl+S, Ctrl+Enter, Ctrl+K only, as today; every other shortcut is a `shortcutHint`. Find step (Ctrl+F) and Arrange (Alt+Shift+F) now show their key, which the menu left out although `onKeyDown` handled them.
- **Labels.** One label per action. The Flow menu said "Testen" (`test`), the "more" menu and the header "Entwurf testen" (`test_draft`): the registry uses `test_draft` everywhere (the header text does not change). The block's Flow menu used the flow label "Löschen"; the "more" menu "Baustein löschen": the block uses the latter everywhere. The delight submenu keeps its two choices; its check mark is the registry's `checked` mark.
- **Block versions.** The block's Flow-menu "Versionen" called `publish.openVersions()`, its "more" menu `ED.blocks.versions(ed, app)`; the registry runs the latter on both.

- [ ] **Step 3.1: impact**

Run the IMPACT command for `setMenus`, `moreMenu`, `menuItems`, `runNowState`, `unlessModal`, `unlessOverlay`. Expected: `setMenus` is called inside `easydrag-editor.js` (events, `enterRunView`, `exitRunView`) and by the shell's tests; `menuItems` only by the editor; `runNowState` by the editor and the header. Confirm with `grep -rn "menuItems\|runNowState\|unlessModal\|unlessOverlay" ui/js scripts`: besides the sites in this task and `easydrag-header.js` (A.6), the delight test.

- [ ] **Step 3.2: write and run the patch**

Write `C:/disposable/easydrag-w1b/patches/a3-patch.mjs`:

```js
// W1b A.3: the editor's window menus and the header "more" menu list action ids; the flow and view actions are bound here.
// Run from the worktree root: node C:/disposable/easydrag-w1b/a3-patch.mjs
import { edit, replaceBlock } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';
const editor = apps + 'easydrag-editor.js';

// The header no longer hands runNowState to the menus (the registry's checks replace it).
edit(editor, `        const { renderHeader, renderSaveState, renderIssues, renderLastRun, runNowState } = headerView;
`, `        const { renderHeader, renderSaveState, renderIssues, renderLastRun } = headerView;
`);

// 1. The "more" menu: two lists of ids. A narrow header hides Runs and Test: the menu offers them first.
replaceBlock(editor, '        function moreMenu(anchor) {', '        async function repairMission() {', `        const MORE_MENU = ['flow.run_now', 'flow.versions', 'flow.settings', 'flow.organize', 'flow.describe', 'flow.repair_mission', 'flow.duplicate', 'flow.export',
            'flow.export_pack', 'flow.export_email', 'flow.mission_control', 'view.liveliness', '-', 'flow.delete'];
        const BLOCK_MORE_MENU = ['flow.versions', 'flow.open_draft', 'view.liveliness', 'flow.delete'];

        function moreMenu(anchor) {
            const r = anchor.getBoundingClientRect();
            const moved = !isBlock && headerNarrow() ? ['view.runs', 'flow.test', '-'] : [];
            ctx.showContextMenu(r.left, r.bottom + 4, ED.actions.items(isBlock ? BLOCK_MORE_MENU : moved.concat(MORE_MENU), { host: ed }));
        }

`, 34);

// 2. The window menus and the action bindings.
replaceBlock(editor, '// ── window menus', '        const setMenusSoon = core.debounce(setMenus, 120);', `        // ── actions and window menus ────────────────────────────────────────────

        // The registry (easydrag-actions.js) holds every action's label, shortcut and reasons. The editing and zoom
        // actions are bound in easydrag-interact.js, the flow and view actions here. The desktop runs a window-menu
        // item's shortcut before onKeyDown sees the key, past onKeyDown's guards, and prevents it: Ctrl+S, Ctrl+Enter
        // and Ctrl+K are real menu shortcuts (Ctrl+K so that the desktop's search does not take it) and do nothing
        // while a dialog, the restore offer or a desktop context menu is open (the shell runs them before onKeyDown
        // can see that menu: the registry's "free" check reads actionFacts). Edits also wait for quick-add ("quiet").
        // Every other shortcut is only drawn by the menus and fired by ED.actions.key in onKeyDown. The step
        // inspector is not a dialog.
        function modalOpen() { return !!el.querySelector('.ed-modal-backdrop:not(.is-closing)'); }
        function contextMenuOpen() { return !!document.querySelector('.vd-context-menu'); }
        ed.actionFacts = () => ({ restoring, dialog: modalOpen() || contextMenuOpen(), running: runs.isRunning() });

        const bind = (id, impl) => ED.actions.bind(id, impl, ed);
        const noPalette = () => (ed.runView ? 'easydrag.ui.reason_palette_hidden' : '');
        bind('flow.home', { run: () => goHome() });
        bind('flow.save', { run: () => saveNow() });
        bind('flow.test', { run: () => test(), label: () => (isBlock ? 'easydrag.ui.block_simulate' : '') });
        bind('flow.stop', { run: () => runs.cancel(), check: () => (ed.run?.durableStatusError || el.querySelector('[data-ed-cmd="cancel"]').getAttribute('aria-busy') === 'true' ? 'easydrag.ui.reason_stop_unavailable' : '') });
        bind('flow.publish', { run: () => publishNow() });
        bind('flow.versions', { run: () => (isBlock ? ED.blocks.versions(ed, app) : publish.openVersions()) });
        bind('flow.export_pack', { run: () => exportLocalPack(), check: () => (ed.readonly && ed.saver.isDirty() ? 'easydrag.ui.reason_export_unsaved' : '') });
        bind('flow.mission_control', { run: () => ctx.openApp && ctx.openApp('mission-control') });
        bind('flow.delete', { run: () => deleteFlow(), label: () => (isBlock ? 'easydrag.ui.block_delete' : '') });
        bind('view.find_step', { run: () => ED.dialogs.findStep(ed, canvas, interact), check: () => (ed.model.doc.nodes.length ? '' : 'easydrag.ui.reason_no_steps') });
        // Ctrl+K adds a step from one selected step, and searches otherwise (searchPalette).
        bind('view.search', { run: () => searchPalette(), check: noPalette, label: () => (ed.selection.size === 1 && !ed.quickAdd ? 'easydrag.ui.keys_add' : '') });
        bind('view.palette', { run: () => togglePalette(), check: noPalette, checked: () => palette.isOpen() });
        bind('view.shortcuts', { run: () => ED.dialogs.shortcuts(ed) });
        if (isBlock) bind('flow.open_draft', { run: () => app.openBlock(ed.flow.id) });
        else {
            bind('flow.run_now', { run: () => runLiveNow() });
            bind('flow.active', { run: () => publish.setActive(!ed.flowEnabled) });
            bind('flow.settings', { run: () => ED.dialogs.flowSettings(ed) });
            bind('flow.duplicate', { run: () => duplicateFlow() });
            bind('flow.export', { run: () => exportFlow() });
            bind('flow.export_email', { run: () => emailBinding.activate(), show: () => !!emailBinding, check: () => (ed.readonly ? 'easydrag.ui.reason_readonly' : '') });
            bind('flow.repair_mission', { run: () => repairMission(), show: () => ed.publishIncomplete === 'FLOW_MISSION_MISSING' || ed.liveIssues.some(issue => issue.code === 'FLOW_MISSION_MISSING') });
            bind('view.runs', { run: () => { runs.toggleDrawer(); renderHeader(); setMenus(); }, checked: () => runs.drawerOpen() });
            if (ED.metadata) bind('flow.organize', { run: () => ED.metadata.organization(Object.assign({}, ed, { readonly: !!(ed.readonly || ed.runView), canWrite: () => !disposed && !ed.readonly && !ed.runView }), ed.flow.id) });
            if (authoring) bind('flow.describe', { run: () => authoring.open() });
        }

        // An action that is not bound here (Run now in a block, Extract in a block) is left out of the menus.
        const FLOW_MENU = ['flow.home', '-', 'flow.save', 'flow.test', 'flow.run_now', 'flow.publish', 'flow.versions', '-', 'flow.settings', 'flow.export', 'flow.export_pack', 'flow.delete'];
        const EDIT_MENU = ['edit.undo', 'edit.redo', '-', 'edit.cut', 'edit.copy', 'edit.paste', 'edit.duplicate', 'edit.extract', 'edit.delete', '-', 'edit.select_all', 'edit.auto_layout'];
        const VIEW_MENU = ['view.zoom_in', 'view.zoom_out', 'view.zoom_fit', '-', 'view.find_step', 'view.search', 'view.palette', 'view.runs', '-', 'view.shortcuts'];

        function setMenus() {
            if (disposed || typeof ctx.setWindowMenus !== 'function') return;
            const c = { host: ed, keyed: true };
            ctx.setWindowMenus(ed.windowId, [
                { id: 'flow', labelKey: 'easydrag.ui.menu_flow', items: ED.actions.items(FLOW_MENU, c) },
                { id: 'edit', labelKey: 'easydrag.ui.menu_edit', items: ED.actions.items(EDIT_MENU, c) },
                { id: 'view', labelKey: 'easydrag.ui.menu_view', items: ED.actions.items(VIEW_MENU, c) }
            ]);
        }
`, 78);

// 3. The "Lebendigkeit" choices are registry actions: delight binds them, the registry draws the submenu.
const delight = apps + 'easydrag-delight.js';
edit(delight, `    // menuItems returns the "Lebendigkeit" entry of the editor's more menu: one item with two choices,
    // and a check mark on the current one.
    function menuItems(t) {
        const current = choice();
        const mark = String.fromCodePoint(0x2713);
        const pick = value => () => { setLevel(value); };
        return [{
            icon: 'sparkles', label: t('easydrag.ui.liveliness'),
            items: [
                { id: 'liveliness-normal', icon: 'play', label: t('easydrag.ui.liveliness_normal'), shortcutHint: current === 'normal' ? mark : '', action: pick('normal') },
                { id: 'liveliness-calm', icon: 'pause', label: t('easydrag.ui.liveliness_calm'), shortcutHint: current === 'calm' ? mark : '', action: pick('calm') }
            ]
        }];
    }

    ED.delight = { allow, level, choice, setLevel, bind, refresh, menuItems };
`, `    // The "Lebendigkeit" entry of the editor's more menu is the registry's view.liveliness submenu: two choices with
    // a check mark on the current one. The choice is per device, so the binding is global (no host).
    if (ED.actions) {
        ED.actions.bind('view.liveliness_normal', { run: () => { setLevel('normal'); }, checked: () => choice() === 'normal' });
        ED.actions.bind('view.liveliness_calm', { run: () => { setLevel('calm'); }, checked: () => choice() === 'calm' });
    }

    ED.delight = { allow, level, choice, setLevel, bind, refresh };
`);

// 4. The motion test's menu part: it reads the registry's submenu.
const delightTest = 'scripts/test-easydrag-delight.mjs';
edit(delightTest, `    vm.runInContext(source, box, { filename: 'easydrag-delight.js' });
`, `    vm.runInContext(actionsSource, box, { filename: 'easydrag-actions.js' });
    vm.runInContext(source, box, { filename: 'easydrag-delight.js' });
`);
edit(delightTest, `const source = fs.readFileSync(new URL('easydrag-delight.js', apps), 'utf8');
`, `const source = fs.readFileSync(new URL('easydrag-delight.js', apps), 'utf8');
const actionsSource = fs.readFileSync(new URL('easydrag-actions.js', apps), 'utf8');
`);
edit(delightTest, `const items = h.delight.menuItems(key => words[key] || key);
assert.equal(items.length, 1);
assert.equal(items[0].label, 'Lebendigkeit');
assert.deepEqual([...items[0].items.map(item => item.label)], ['Normal', 'Ruhig']);
assert.deepEqual([...items[0].items.map(item => !!item.shortcutHint)], [false, true], 'the check mark marks the current choice');
items[0].items[0].action();
assert.equal(h.store.get(KEY), 'normal');
assert.equal(h.delight.menuItems(key => key)[0].items[0].shortcutHint.length > 0, true);
`, `const menuOf = t => h.actions.items(['view.liveliness'], { host: { t } });
const items = menuOf(key => words[key] || key);
assert.equal(items.length, 1);
assert.equal(items[0].label, 'Lebendigkeit');
assert.deepEqual([...items[0].items.map(item => item.label)], ['Normal', 'Ruhig']);
assert.deepEqual([...items[0].items.map(item => !!item.shortcutHint)], [false, true], 'the check mark marks the current choice');
items[0].items[0].action();
assert.equal(h.store.get(KEY), 'normal');
assert.equal(menuOf(key => key)[0].items[0].shortcutHint.length > 0, true);
`);
edit(delightTest, `    return { delight: box.window.EasyDrag.delight, state, store, root, change: () => listeners.forEach(fn => fn()) };`,
    `    return { delight: box.window.EasyDrag.delight, actions: box.window.EasyDrag.actions, state, store, root, change: () => listeners.forEach(fn => fn()) };`);

// 5. Tests that read the editor's menus. The window menus now draw the shortcut of Find and Arrange (Ctrl+F, Alt+Shift+F) and the
// named Delete; Paste is off in this harness because nothing is copied and it has no clipboard (the registry says so); a paused
// flow says reason_paused; Run now says why it is off for a flow that is not published. The a1008 test starts the editor switched on
// and lets the server refuse the live start (the refusal path), because a switched-off flow is now refused before the request.
edit('scripts/test-easydrag-extra3.mjs', `                ['undo=Ctrl+Z', 'redo=Ctrl+Shift+Z', 'cut=Ctrl+X', 'copy=Ctrl+C', 'paste=Ctrl+V', 'duplicate=Ctrl+D', 'delete-sel=Del', 'select-all=Ctrl+A',
                    'zoom-in=+', 'zoom-out=−', 'zoom-fit=Shift+1', 'keys=?'],`, `                ['undo=Ctrl+Z', 'redo=Ctrl+Shift+Z', 'cut=Ctrl+X', 'copy=Ctrl+C', 'paste=Ctrl+V', 'duplicate=Ctrl+D', 'delete-sel=key_delete', 'select-all=Ctrl+A', 'auto-layout=Alt+Shift+F',
                    'zoom-in=+', 'zoom-out=−', 'zoom-fit=Shift+1', 'find-step=Ctrl+F', 'keys=?'],`);

edit('scripts/test-easydrag-extra3.mjs', `[true, [true, false, false, false, false, false, false]], 0, false]`, `[true, [true, false, false, true, false, false, false]], 0, false]`);
edit('scripts/test-easydrag-extra5.mjs', `[[true, 'error_flow_disabled'], [true, 'error_flow_disabled']]`, `[[true, 'reason_paused'], [true, 'reason_paused']]`, 2);
edit('scripts/test-easydrag-extra6.mjs', `            if (req.method === 'PUT') return { draft_revision: 3, issues: [] };
            return undefined;
        });
        const editor = openEditor(h, { enabled: false, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });`, `            if (req.method === 'PUT') return { draft_revision: 3, issues: [] };
            return undefined;
        });
        const editor = openEditor(h, { enabled: true, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });`);

// The Go browser test that pauses the flow elsewhere reads the registry's reason in the Flow menu and in the more menu.
edit('ui/desktop_easydrag_browser_test.go', `item.disabledHint===ed.t('easydrag.ui.error_flow_disabled')`, `item.disabledHint===ed.t('easydrag.ui.reason_paused')`);
edit('ui/desktop_easydrag_browser_test.go', `b.title===ed.t('easydrag.ui.error_flow_disabled')`, `b.title===ed.t('easydrag.ui.reason_paused')`);

console.log('A.3 patch applied');
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a3-patch.mjs
```
Expected: `A.3 patch applied`. The two `replaceBlock` calls check that exactly 34 and 78 lines go (the `moreMenu` function with its blank line; the window-menu section from its heading to `setMenusSoon`); if the count differs, the section was edited since this plan: look at `git diff main -- ui/js/desktop/apps/easydrag-editor.js`, adjust the number, and check that the block still starts and ends where the script says.

- [ ] **Step 3.3: prove no old form remains**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
grep -n "unlessModal\|unlessOverlay\|runNowState\|menuItems\|labelKey: 'easydrag.ui.save_now'\|shortcutHint\|shortcut:" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-delight.js
grep -n "label: t('easydrag.ui" ui/js/desktop/apps/easydrag-editor.js
wc -l ui/js/desktop/apps/easydrag-editor.js
```
Expected: the first grep prints nothing; the second prints only the modal's own `actions: [{ id: 'cancel', label: …` line of `repairMission` (a dialog button, not a menu entry); `wc -l` says 852 (896 before). `runNowState` still exists in `easydrag-header.js` until A.6.

- [ ] **Step 3.4: run the checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-actions.mjs && node scripts/test-easydrag-delight.mjs | tail -1 && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: `PASS action registry: 54 rows`, `PASS motion gate: …`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`.

The script updates these expectations, each for one stated cause (nothing else may change; if another test fails, the cause is a regression and is fixed in the code, not in the test):
  - `scripts/test-easydrag-delight.mjs`: the submenu is read from `ED.actions.items(['view.liveliness'], { host: { t } })`; the test loads `easydrag-actions.js` before `easydrag-delight.js`.
  - `c1d10 only Ctrl+S, Ctrl+Enter and Ctrl+K are dispatched …` (`extra3`): the hint list gains `auto-layout=Alt+Shift+F` and `find-step=Ctrl+F` (shown now) and `delete-sel` reads `key_delete` (the named Delete). The same test still asserts that no window-menu item carries a `label` (every label is a key).
  - `c1d07 the detail view is not modal …` (`extra3`): Paste is unavailable in this harness (nothing was copied and there is no clipboard API: `reason_nothing_to_paste`).
  - `ff2 I1 …` and `ff2 switching the flow off here …` (`extra5`): the reason of a paused flow is `reason_paused`.
  - `a1008 runs pause does not block the test POST …` (`extra6`): the editor now starts switched on and the server refuses the live start; a switched-off flow is refused before any request (the registry's `active` check), the server's refusal path is what this test covers.

- [ ] **Step 3.5: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-t0.mjs --node-only 2>&1 | tail -4
```
Write `C:/disposable/easydrag-w1b/a3-message.txt`:
```
feat(easydrag): window menus and the more menu use the action registry

The editor binds the flow and view actions (save, test, publish, run now,
versions, settings, export, delete, find, search, palette, runs, shortcuts) and
lists action ids for the Flow, Edit and View menus and the header more menu. An
action a block editor does not have is not bound and so not listed. The facts of
the moment (restore offer, open dialog or context menu, run in progress) feed
the registry's shared checks, which replace unlessModal, unlessOverlay and
runNowState. The Lebendigkeit choices are registry actions bound by easydrag-delight.
Find step and Arrange show their shortcut.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && git add ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-delight.js scripts/test-easydrag-delight.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra5.mjs scripts/test-easydrag-extra6.mjs && git commit -F C:/disposable/easydrag-w1b/a3-message.txt && git show --stat HEAD | tail -8
```
Expected: one commit, six files.

---

### Task A.4: the start page's card menu and window menu through the registry

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-home.js` (`cardMenu` 677-694)
- Modify: `ui/js/desktop/apps/easydrag.js` (`setHomeMenus` 74-85, `bindHomeActions` new, the instance creation around 236)
- Modify: `scripts/test-easydrag-extra5.mjs` (one expected pair)

**Design.**
- **The home card menu uses the `flow.*` rows with the card's flow as context.** The start page creates `actionHost = { t, readonly: !!app.readonly, actionFacts: () => ({ locked: createOff() && !app.readonly }) }` and binds `flow.open`, `flow.organize` (only when `hasOrganization`), `flow.run_now`, `flow.duplicate`, `flow.export`, `flow.export_pack` (only when `packUI` exists), `flow.mission_control`, `flow.delete` on it. The context is `{ host: actionHost, flow: f }`, `f` being the card's flow record (`id`, `name`, `published`, `enabled`).
- **The mapping of today's conditions.** `locked = createOff()` becomes the registry's `writable` / `unlocked` (cause `locked`, or `readonly` when the desktop is read-only); `!f.published` the `published` check; `paused = published && !enabled` the `active` check; `runsPending.has(f.id)` a bound `check` returning `reason_running` ("A run is in progress"), because the request that starts the run is the run in progress for this card. Each cause now has a reason text; the home had only the stale-list text (`home_stale_locked`, still used by the card grid) and the paused hint.
- **Run now's read-only and unpublished cases** now say why (`reason_readonly`, `reason_not_published`); before they were silently off.
- **The start page's window menu** (New, Import, Reload; `easydrag.js`) builds entries with `labelKey:` today, so the lint (A.7) either exempts it or the rows exist. It gets rows `flow.new`, `flow.import`, `flow.reload`, bound on the shell instance `inst` (`bindHomeActions`, once per window). `blocked = readonly || createOff()` becomes `unlocked` over the facts `readonly` and `locked`; "New" reads "New block" in the block library (a bound label); Import is left out there (a bound `show`).

- [ ] **Step 4.1: impact**

Run the IMPACT command for `cardMenu` and `setHomeMenus`. Expected: `cardMenu` is called by the card's menu button and the context-menu handler inside `easydrag-home.js`; `setHomeMenus` by `easydrag.js` (`showHome`, `showFlow`'s home route). Confirm with `grep -rn "cardMenu\|setHomeMenus" ui/js scripts`.

- [ ] **Step 4.2: write and run the patch**

Write `C:/disposable/easydrag-w1b/patches/a4-patch.mjs`:

```js
// W1b A.4: the start page's card menu and the start page's window menu list registry actions.
// Run from the worktree root: node C:/disposable/easydrag-w1b/a4-patch.mjs
import { edit, replaceBlock } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// 1. The card menu: the card's flow is the context; the registry says why an entry is off.
replaceBlock(apps + 'easydrag-home.js', '        function cardMenu(f, x, y) {', '        // focusTemplates moves to the existing template section.', `        // The card menu lists registry actions with the card's flow as context (easydrag-actions.js). The host's readonly is the
        // desktop's, locked is createOff without it; a paused or unpublished flow and a pending run-now are checks of run_now.
        const actionHost = { t, readonly: !!app.readonly, actionFacts: () => ({ locked: createOff() && !app.readonly }) };
        const bindAction = (id, impl) => ED.actions.bind(id, impl, actionHost);
        bindAction('flow.open', { run: c => { clearSelection(); app.openFlow(c.flow.id); } });
        if (hasOrganization) bindAction('flow.organize', { run: c => ED.metadata.organization(metadataEnv(), c.flow.id, reload) });
        bindAction('flow.run_now', { run: c => runNow(c.flow), check: c => (runsPending.has(c.flow.id) ? 'easydrag.ui.reason_running' : '') });
        bindAction('flow.duplicate', { run: c => duplicate(c.flow.id) });
        bindAction('flow.export', { run: c => download(c.flow.id, c.flow.name) });
        if (packUI) bindAction('flow.export_pack', { run: c => packUI.exportStored(c.flow.id, c.flow.name) });
        bindAction('flow.mission_control', { run: () => ctx.openApp && ctx.openApp('mission-control') });
        bindAction('flow.delete', { run: c => remove(c.flow) });

        function cardMenu(f, x, y) {
            if (!f || typeof ctx.showContextMenu !== 'function') return;
            ctx.showContextMenu(x, y, ED.actions.items(['flow.open', 'flow.organize', 'flow.run_now', 'flow.duplicate', 'flow.export', 'flow.export_pack', 'flow.mission_control', '-', 'flow.delete'], { host: actionHost, flow: f }));
        }

`, 19);

// 2. The start page's window menu (New, Import, Reload): rows flow.new, flow.import and flow.reload.
const shell = apps + 'easydrag.js';
replaceBlock(shell, '    function setHomeMenus(inst) {', '    function clickHome(inst, selector) {', `    function setHomeMenus(inst) {
        if (typeof inst.ctx.setWindowMenus !== 'function') return;
        inst.ctx.setWindowMenus(inst.windowId, [{ id: 'flow', labelKey: 'easydrag.ui.menu_flow', items: ED.actions.items(['flow.new', 'flow.import', '-', 'flow.reload'], { host: inst }) }]);
    }

    // bindHomeActions binds the start page's window-menu actions. The shell instance is their host: New and Import are
    // off when the desktop is read-only (readonly) or the list came back FLOWS_DISABLED (createOff, the registry's locked).
    function bindHomeActions(inst) {
        inst.actionFacts = () => ({ readonly: !!inst.ctx.readonly, locked: !!(inst.screen && typeof inst.screen.createOff === 'function' && inst.screen.createOff()) });
        const bind = (id, impl) => ED.actions.bind(id, impl, inst);
        bind('flow.new', { run: () => clickHome(inst, '[data-ed-new]'), label: () => (inst.blockHome ? 'easydrag.ui.block_new' : '') });
        bind('flow.import', { run: () => clickHome(inst, '[data-ed-import]'), show: () => !inst.blockHome });
        bind('flow.reload', { run: () => inst.screen && inst.screen.reload && inst.screen.reload() });
    }

`, 13);
edit(shell, `        instances.set(windowId, inst);
`, `        bindHomeActions(inst);
        instances.set(windowId, inst);
`);

// The start page's Run now says why it is off: a paused flow reason_paused, a flow that is not published reason_not_published.
edit('scripts/test-easydrag-extra5.mjs', `['f2', true, 'error_flow_disabled'], ['f3', true, '']]`, `['f2', true, 'reason_paused'], ['f3', true, 'reason_not_published']]`);

console.log('A.4 patch applied');
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a4-patch.mjs
```
Expected: `A.4 patch applied`. The block counts (19 lines of `cardMenu` with its blank line; 13 lines of `setHomeMenus` with its blank line) are checked by the script.

- [ ] **Step 4.3: prove no old form remains, and the line rule**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
grep -n "label: t('easydrag.ui\|const locked = createOff()\|const paused = \|labelKey: inst.blockHome" ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag.js
grep -c "ED.actions" ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag.js
wc -l ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag.js
```
Expected: the first grep prints only the cancel button of a modal in `easydrag-home.js` (`actions: [{ id: 'cancel', label: …`, not a menu entry); counts 2 and 2; `wc -l` says 888 for `easydrag-home.js` (889 before: one line shorter, as the part requires) and 277 for `easydrag.js` (274 before).

- [ ] **Step 4.4: run the checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. One expectation changes, for one cause: `ff2 the Paused filter lists published flows that are off; Run now is off with its hint …` (`extra5`): a paused flow's Run now says `reason_paused` (it said `error_flow_disabled`, "The flow is paused. Switch it to Active first."), and a flow that was never published says `reason_not_published` (it said nothing). The patch script updates that line.

- [ ] **Step 4.5: gates and commit**

Write `C:/disposable/easydrag-w1b/a4-message.txt`:
```
feat(easydrag): start page card menu and window menu use the action registry

The card menu lists flow.* action ids with the card's flow as context. The
card's agent-permission lock, a paused flow, an unpublished flow and a pending
run-now are the registry's checks, each with its reason text. The start page's
window menu (New, Import, Reload) lists flow.new, flow.import and flow.reload,
bound once per window on the shell instance.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-t0.mjs --node-only 2>&1 | tail -3 && git add ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag.js scripts/test-easydrag-extra5.mjs && git commit -F C:/disposable/easydrag-w1b/a4-message.txt && git show --stat HEAD | tail -5
```
Expected: one commit, three files.

---

### Task A.5: `ED.actions.key` and the generated shortcut table

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (`onKeyDown` 721-752)
- Modify: `ui/js/desktop/apps/easydrag-interact.js` (`handleKey` 664-697)
- Modify: `ui/js/desktop/apps/easydrag-dialogs.js` (`SHORTCUTS`, `kbd`, `shortcutRows` 8-54; the row template in `shortcuts` 60; the export line 308)
- Modify: `scripts/test-easydrag-extra3.mjs` (the shortcut-table test)

**Step 5.1 is a record, not a change.** Before the change, write down what each key does today, from the code (the editor's `onKeyDown`, interact's `handleKey`, the shell's `handleWindowMenuShortcut` at `ui/js/desktop/core/menus-and-routing.js:1525`, which is registered before the editor's listener on the same `document`). The table below is that record, taken from the code on main at 065b6fca8; Step 5.1 re-derives it with the commands given, so a change since then shows up.

Notation: "shell" is the desktop's window-menu dispatch (a menu item's `shortcut` field; it skips a disabled item and, except for Ctrl+S, a key typed in a text field, then runs the item's action and prevents the key, so `onKeyDown` returns at its first line). "onKeyDown" and "handleKey" are the editor and interact. "Canvas focus" is `onCanvas`: the focus is in the canvas, on the page body or on the editor root. "Text" is `core.isEditable(target)`.

| Key | Who fires it today | In a text field | Dialog open | Desktop context menu open | Read-only / run view |
|---|---|---|---|---|---|
| Ctrl+S | shell (menu `shortcut`; `unlessModal(saveNow)`); onKeyDown when the shell skipped it (disabled item, restore offer) | works (the shell allows it in fields) | prevented, nothing saved | prevented, nothing saved | `saveNow` refuses; key prevented |
| Ctrl+Enter | shell (`unlessModal(test)`, item `disabled: ro`); onKeyDown (`test()`) from a text field or when the item is disabled | works (onKeyDown) | prevented (Chrome), nothing runs | nothing runs | `startTest` refuses; key prevented |
| Ctrl+K | shell (`unlessModal(searchPalette)`, item disabled in the run view, so the shell's own search does not take it); onKeyDown from a text field | works (onKeyDown) | prevented, nothing | nothing | `searchPalette` returns in the run view; key prevented |
| Ctrl+F | onKeyDown only (no menu shortcut; the menu showed no key) | works | prevented, nothing | nothing | works (opens the find dialog) |
| Alt+Shift+F | onKeyDown only | not handled (not prevented) | nothing | nothing | `autoLayout` refuses |
| ? | onKeyDown only, canvas focus, not in quick-add | not handled | nothing | nothing | works |
| Esc in a run view | onKeyDown (exit the run view), not in a text field, not in quick-add | not handled | nothing | nothing | exits |
| Esc otherwise | handleKey (clears the selection) | not handled | nothing | nothing | clears |
| Ctrl+Z, Ctrl+Shift+Z, Ctrl+Y | handleKey (menu `shortcutHint`) | native undo (not handled) | nothing | nothing | no-op, key prevented |
| Ctrl+A | handleKey | native | nothing | nothing | selects all |
| Ctrl+C | handleKey; not prevented when nothing is selected | native | nothing | nothing | copies (allowed) |
| Ctrl+X | handleKey: copies, then removes | native | nothing | nothing | **copies, removes nothing** |
| Ctrl+V | handleKey | native | nothing | nothing | refuses |
| Ctrl+D | handleKey | native | nothing | nothing | refuses, key prevented |
| Ctrl+0 | handleKey (no menu entry) | native | nothing | nothing | works |
| Delete, Backspace | handleKey (an active group takes them first) | native | nothing | nothing | refuses |
| Shift+1 (also `!`), +, =, - | handleKey | native | nothing | nothing | works |
| D | handleKey, with a selection | native | nothing | nothing | refuses |
| C | handleKey, with one selected step | native | nothing | nothing | refuses |
| Enter | handleKey, with one selected step (an active group takes it first); a button inside the canvas keeps Enter | native | nothing | nothing | works |
| Arrows, Shift+arrows, Space, Tab | handleKey (stay there; no action) | native | nothing | nothing | arrows work; nudge refuses |

"Nothing" means the key is not handled: in a dialog `onKeyDown` returns before any registry key (it prevents only Ctrl+S, Ctrl+K, Ctrl+F and Ctrl+Enter there), and with a desktop context menu open it returns as well. "Canvas focus" applies to every row from `?` down except Esc in a run view. `ed.quickAdd` open stops every key from `Ctrl+Z` on down (not Ctrl+S, Ctrl+Enter, Ctrl+K, Ctrl+F, Alt+Shift+F).

**Decision, per key: one path fires it.**
- **The shell fires** Ctrl+S, Ctrl+Enter and Ctrl+K, because their window-menu items are real shortcuts (Ctrl+K so that the desktop's search does not take the key; `flow.save`, `flow.test`, `view.search` are `DESKTOP_KEYS` in the registry, whose menu items carry `shortcut`). The shell prevents the key, so `onKeyDown` and `ED.actions.key` never see it: it fires once. `ED.actions.key` also matches them (a text field, a disabled item or a restore offer made the shell skip them, and `onKeyDown` handled them then); it runs only for an event the shell did not prevent.
- **`ED.actions.key` fires every other registry shortcut**, from `onKeyDown`, and the menus only display them (`shortcutHint`). `handleKey` keeps Space, Esc, arrows, Shift+arrows, Tab and the active group's Delete and Enter, which are no actions.
- **Scopes follow the table.** `editor` scope (works from a text field where the table says so): `flow.save`, `flow.test`, `view.search`, `view.find_step` (these four also from a text field), `edit.auto_layout` (not from a text field). `canvas` scope (canvas focus, no text field, no quick-add): every other row with a shortcut.
- **Behaviour is kept.** The matcher takes `+` or `=`, Shift+1 or `!`, Delete or Backspace, Ctrl+Y as redo, ignores Shift and Alt where the shortcut does not name them (as handleKey does) and never fires a plain key with Ctrl held. A refused action still takes its key (a read-only Ctrl+Z is prevented, as before); Ctrl+C with nothing selected passes (as before). `accepts` keeps an active group's Delete and Enter and a button's Enter with `handleKey`.
- **Differences (listed in "Deviations from the spec").** Ctrl+X in read-only mode no longer copies; D, C and Enter without a (single) selection, and Ctrl+D, are now taken and prevented where they were not (no visible effect); Ctrl+F in a flow without steps is unavailable (`reason_no_steps`); Ctrl+Enter, Ctrl+S, Ctrl+K and Ctrl+F stay available under quick-add as today.

**Display.** Shortcuts are drawn through `ED.actions.display` (`core.shortcut`, so ⌘ and ⇧ on macOS; the named Delete: "Del", "Entf" in German, "Supr" in Spanish; the real minus sign). The window menu shows "Ctrl+Z" for Undo and so on, as today.

**The table ("Tasten") is generated.** `LAYOUT` in `easydrag-dialogs.js` names the action ids of the three groups (canvas, edit, flow); `ED.actions.table` adds each row's shortcut and label (so "Ctrl+K" reads "Add a step" while one step is selected, from `view.search`'s bound label), and the gestures that are no action (arrows, Shift+arrows, Space, Esc) stay as written pairs. "+ / −" becomes two rows ("Zoom in", "Zoom out"). `ED.dialogs.SHORTCUTS` stays an export (a getter) for the tests.

- [ ] **Step 5.1: re-derive the table**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
sed -n '/function onKeyDown/,/bag.listen(document, .keydown., onKeyDown)/p' ui/js/desktop/apps/easydrag-editor.js
sed -n '/function handleKey(event)/,/function handleKeyUp/p' ui/js/desktop/apps/easydrag-interact.js
sed -n '/function handleWindowMenuShortcut/,/^    }/p;/function editableShortcutTarget/,/^    }/p' ui/js/desktop/core/menus-and-routing.js
```
Compare with the table. Expected: the same keys and guards (the `onKeyDown` text is the one shown in A.5's patch script as old code; `handleKey` still holds the mod keys because A.2 did not touch them). A difference means the table above is out of date: correct the table in this task's commit message and use the new behaviour in the registry rows (scope, `TEXT_KEYS`, `PASS_WHEN_REFUSED`) before going on; if the difference is a behaviour change that is not stated here, stop and ask the coordinator (it is a design decision for the owner).

- [ ] **Step 5.2: run the keyboard checks before the change (they are the regression net)**

The registry test of A.1 already pins `ED.actions.key` (scope, text fields, quick-add, aliases, passes, once). The editor-level checks are the existing `scripts/test-easydrag-extra*.mjs` suite, which presses these keys on the canvas, and the browser test of A.8. Run the suite on the unchanged keyboard code to see that it is green; after the patch the same checks must stay green (this task adds no key test of its own, because the existing ones already press every key):
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag.mjs 2>&1 | tail -2
```
Expected: `all EasyDrag checks passed`.

- [ ] **Step 5.3: impact**

Run the IMPACT command for `onKeyDown`, `handleKey`, `shortcutRows` and `kbd`. Expected: `onKeyDown` is registered by the editor only; `handleKey` is called by `onKeyDown` and the tests; `shortcutRows` and `kbd` by `shortcuts` in `easydrag-dialogs.js`. Confirm with `grep -rn "handleKey\|shortcutRows\|SHORTCUTS" ui/js scripts ui/*.go`.

- [ ] **Step 5.4: write and run the patch**

Write `C:/disposable/easydrag-w1b/patches/a5-patch.mjs`:

```js
// W1b A.5: ED.actions.key is the one keyboard path for registry shortcuts; the shortcut table is generated from the registry.
// Run from the worktree root: node C:/disposable/easydrag-w1b/a5-patch.mjs
import { edit, replaceBlock } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-interact.js: the keys that are actions leave handleKey ────────────────────────────────
// Space, Escape, the arrows and the active group's Delete and Enter stay (they are no action).

const interact = apps + 'easydrag-interact.js';
edit(interact, `        // handleKey processes canvas shortcuts; it returns true when the event was used.
`, `        // handleKey processes the canvas keys that are no action (Space, Escape, the arrows, the active group's Delete
        // and Enter); the actions' shortcuts are ED.actions.key's. It returns true when the event was used.
`);
edit(interact, `            const mod = core.isMod(event);
            const one = ed.selection.size === 1 ? Array.from(ed.selection)[0] : null;
            if (key === ' ' && !event.repeat) { spaceDown = true; el.classList.add('is-space'); return true; }
            if (mod && key.toLowerCase() === 'z' && !event.shiftKey) { undo(); return true; }
            if (mod && (key.toLowerCase() === 'y' || (key.toLowerCase() === 'z' && event.shiftKey))) { redo(); return true; }
            if (mod && key.toLowerCase() === 'a') { selectAll(); return true; }
            if (mod && key.toLowerCase() === 'c') return copySelection();
            if (mod && key.toLowerCase() === 'x') { if (copySelection()) removeSelection({ quiet: true }); return true; }
            if (mod && key.toLowerCase() === 'v') { pasteAt(null); return true; }
            if (mod && key.toLowerCase() === 'd') { if (!readonly() && ed.selection.size) duplicateSteps(Array.from(ed.selection)); return true; }
            if (mod && key === '0') { const s = canvas.size(); canvas.setView(G.zoomAt(ed.view, 1, { x: s.w / 2, y: s.h / 2 }), { animate: true }); return true; }
            if (mod) return false;
`, `            const one = ed.selection.size === 1 ? Array.from(ed.selection)[0] : null;
            if (key === ' ' && !event.repeat) { spaceDown = true; el.classList.add('is-space'); return true; }
            if (core.isMod(event)) return false;
`);
edit(interact, `            if (key === 'Delete' || key === 'Backspace') { removeSelection(); return true; }
            if (key === 'Escape') { select([]); wires.highlight(null); return true; }
            if (key === '!' || (key === '1' && event.shiftKey)) { canvas.fit({ animate: true }); return true; }
            if (key === '+' || key === '=') { canvas.zoomBy(1.2); return true; }
            if (key === '-') { canvas.zoomBy(1 / 1.2); return true; }
            if ((key === 'd' || key === 'D') && ed.selection.size) { if (!readonly()) toggleSteps(Array.from(ed.selection)); return true; }
            if (key === 'Enter' && one) { ed.bus.emit('open-detail', { nodeId: one }); return true; }
            if ((key === 'c' || key === 'C') && one && !readonly()) { ed.bus.emit('connect-picker', { nodeId: one }); return true; }
`, `            if (key === 'Escape') { select([]); wires.highlight(null); return true; }
`);

// ── easydrag-editor.js: onKeyDown hands the registry's shortcuts to ED.actions.key ───────────────────

edit(apps + 'easydrag-editor.js', `            if (mod && key === 's') { if (!restoring) saveNow(); return; }
            if (mod && event.key === 'Enter') { event.preventDefault(); test(); return; }
            if (mod && key === 'k') { event.preventDefault(); searchPalette(); return; }
            if (mod && key === 'f') { event.preventDefault(); ED.dialogs.findStep(ed, canvas, interact); return; }
            if (event.altKey && event.shiftKey && key === 'f' && !core.isEditable(target)) { event.preventDefault(); canvas.autoLayout(); return; }
            if (core.isEditable(target) || ed.quickAdd) return;
            const active = document.activeElement;
            const onCanvas = canvas.el.contains(active) || active === document.body || active === el;
            if (event.key === '?' && onCanvas) { event.preventDefault(); ED.dialogs.shortcuts(ed); return; }
            if (event.key === 'Escape' && ed.runView) { event.preventDefault(); exitRunView(); return; }
`, `            // The registry's shortcuts (ED.actions.key) keep the rules each key had: Ctrl+S, Ctrl+Enter, Ctrl+K and Ctrl+F also
            // work from a text field, Alt+Shift+F from outside one, and the rest only while the canvas or a card has the focus
            // and quick-add is closed. A refused action (read-only, a run in progress ...) still takes its key.
            const active = document.activeElement;
            const onCanvas = canvas.el.contains(active) || active === document.body || active === el;
            const control = !!(event.target && event.target !== canvas.el && event.target.closest && event.target.closest('button, a, input, textarea, select'));
            if (ED.actions.key(event, { host: ed, editable: core.isEditable(target), quickAdd: !!ed.quickAdd, onCanvas, control })) return;
            if (core.isEditable(target) || ed.quickAdd) return;
            if (event.key === 'Escape' && ed.runView) { event.preventDefault(); exitRunView(); return; }
`);

// ── easydrag-dialogs.js: the shortcut table is the registry's ─────────────────────────────────────────

const dialogs = apps + 'easydrag-dialogs.js';
replaceBlock(dialogs, '    // SHORTCUTS lists [keys, label key] per group', '    function shortcuts(ed) {', `    // LAYOUT is the shortcut table ("Tasten"): the registry's actions that have a shortcut, in the dialog's groups, and the
    // gestures that are no action (arrows, Space, Esc), written out. ED.actions.table adds the keys and the labels.
    const LAYOUT = [
        ['easydrag.ui.keys_group_canvas', ['step.open', 'step.connect', 'step.toggle', 'edit.delete',
            ['← → ↑ ↓', 'easydrag.ui.keys_navigate'], ['Shift+← → ↑ ↓', 'easydrag.ui.keys_nudge'], ['Space', 'easydrag.ui.keys_pan'],
            'view.zoom_fit', 'view.zoom_in', 'view.zoom_out', 'view.zoom_reset']],
        ['easydrag.ui.keys_group_edit', ['edit.undo', 'edit.redo', 'edit.copy', 'edit.cut', 'edit.paste', 'edit.duplicate', 'edit.select_all']],
        ['easydrag.ui.keys_group_flow', ['flow.save', 'flow.test', 'view.search', 'view.find_step', 'edit.auto_layout', 'view.shortcuts', ['Esc', 'easydrag.ui.keys_escape']]]
    ];

    // kbd draws one key per part of keys: "+" joins keys ("Ctrl+S"), but a leading "+" is a key
    // itself ("+" alone must not become an empty key). ED.actions.display names Delete and draws ⌘ and ⇧ on macOS.
    function kbd(esc, keys, t) {
        const text = ED.actions.display(keys, t);
        return (text.startsWith('+') ? [text] : text.split('+')).map(k => '<kbd>' + esc(k) + '</kbd>').join('<span class="ed-kbd-plus">+</span>');
    }

    // Ctrl+K reads "Add a step" while one step is selected: view.search's bound label.
    function shortcutRows(ed) { return ED.actions.table(LAYOUT, { host: ed }); }

`, 48);
edit(dialogs, `'<div class="ed-keys-row"><dt>' + kbd(esc, keys) + '</dt>`, `'<div class="ed-keys-row"><dt>' + kbd(esc, keys, t) + '</dt>`);
edit(dialogs, `    ED.dialogs = { shortcuts, findStep, connectPicker, flowSettings, conflict, restore, SHORTCUTS };`,
    `    ED.dialogs = { shortcuts, findStep, connectPicker, flowSettings, conflict, restore, get SHORTCUTS() { return ED.actions.table(LAYOUT, {}); } };`);

// ── tests that read the table ────────────────────────────────────────────────────────────────────────

edit('scripts/test-easydrag-extra3.mjs', `        eq('c1d07 the shortcuts dialog draws "+ / −" as one key and no empty key',
            [kbds.includes('<kbd>+ / −</kbd>'), kbds.includes('<kbd></kbd>'), kbds.includes('<kbd>Ctrl</kbd>'), kbds.includes('<kbd>S</kbd>')], [true, false, true, true]);`,
`        eq('c1d07 the shortcuts dialog draws "+" and "−" as keys of their own and no empty key',
            [kbds.includes('<kbd>+</kbd>'), kbds.includes('<kbd>−</kbd>'), kbds.includes('<kbd></kbd>'), kbds.includes('<kbd>Ctrl</kbd>'), kbds.includes('<kbd>S</kbd>')], [true, true, false, true, true]);`);

// The interact harness of test-easydrag-extra.mjs calls handleKey directly. The keys that are actions now come through
// ED.actions.key (as the editor's onKeyDown sends them), so the harness gets press(), which does the same.
const extra = 'scripts/test-easydrag-extra.mjs';
edit(extra, `            key: (k, target, extra) => Object.assign({ key: k, target: target || canvas.el, repeat: false, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }, extra),
`, `            key: (k, target, extra) => Object.assign({ key: k, target: target || canvas.el, repeat: false, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }, extra),
            // press sends a key the way onKeyDown does: the registry first, then handleKey.
            press(event) {
                const target = event.target || canvas.el;
                const where = { host: ed, editable: ED.core.isEditable(target), quickAdd: false, onCanvas: true, control: target !== canvas.el && !!target.closest && !!target.closest('button, a, input, textarea, select') };
                return ED.actions.key(Object.assign({ preventDefault() {} }, event), where) || interact.handleKey(event);
            },
`);
edit(extra, `interact.handleKey(key('Enter')), h.details.length], [true, true, true, 1]);`, `h.press(key('Enter')), h.details.length], [true, true, true, 1]);`);
edit(extra, `const inField = interact.handleKey(key('Delete', field));`, `const inField = h.press(key('Delete', field));`);
edit(extra, `        h.interact.handleKey(h.key('a', h.el, { ctrlKey: true }));`, `        h.press(h.key('a', h.el, { ctrlKey: true }));`);

console.log('A.5 patch applied');
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a5-patch.mjs
```
Expected: `A.5 patch applied` (the dialogs block is checked to be 48 lines).

- [ ] **Step 5.5: prove no old form remains, and the line rule**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
grep -n "key.toLowerCase() === '[zyacxvd]'\|key === '+'\|key === 'Delete'\|key === 'Enter' && one\|const SHORTCUTS\|keys_connect'\]" ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-dialogs.js
grep -n "mod && event.key === 'Enter'\|mod && key === 'k'\|mod && key === 'f'\|event.altKey && event.shiftKey" ui/js/desktop/apps/easydrag-editor.js
wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-dialogs.js
```
Expected: the first grep prints only the group branch's `key === 'Delete'` line inside `handleKey` (the active group's Delete and Enter, which stay) and nothing else; the second prints nothing; `wc -l`: editor 851 (896 at baseline), interact 750 (753 before), dialogs 281 (309 before). All three are now shorter than the baseline.

- [ ] **Step 5.6: run the checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-actions.mjs && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: `PASS action registry: 54 rows`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. One test changes, for one cause: `c1d07 the shortcuts dialog draws "+ / −" as one key …` (`extra3`) now expects the keys `+` and `−` as keys of their own (the table has one row per zoom action). The `a1008 runs shortcuts drop Tab add and keep Ctrl+K plus connect` test (`extra6`) stays as it is: `SHORTCUTS` still lists `Ctrl+K` with `keys_search`, `C`, and no `Tab`, and the dialog still shows `keys_add` for one selected step and `keys_search` while quick-add is open.

- [ ] **Step 5.6b: keys read by the old labels**

The table now has one label per action. The keys `keys_open`, `keys_delete`, `keys_fit`, `keys_zoom` and `keys_help` are no longer used by any table row; A.7 deletes them. Check that nothing else reads them yet:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && grep -rn "keys_open\|keys_delete\|keys_fit\|keys_zoom'\|keys_help" ui/js scripts ui/*.go | grep -v "ui/js/desktop/bundles"
```
Expected: no output.

- [ ] **Step 5.7: gates and commit**

Write `C:/disposable/easydrag-w1b/a5-message.txt`:
```
feat(easydrag): one keyboard path for registry shortcuts, shortcut table from the registry

ED.actions.key fires every registry shortcut from the editor's onKeyDown with
the rules each key had: Ctrl+S, Ctrl+Enter, Ctrl+K and Ctrl+F also from a text
field, Alt+Shift+F outside one, the rest only with the canvas focused and quick-add
closed. The desktop still fires Ctrl+S, Ctrl+Enter and Ctrl+K (menu shortcuts) and
prevents them, so each key fires once. handleKey keeps Space, Escape, the arrows
and the active group's Delete and Enter. The "Tasten" table is generated from
the registry plus the gestures that are no action; "+ / -" is two rows. Ctrl+X no
longer copies in read-only mode (the Edit menu already refused it).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-t0.mjs --node-only 2>&1 | tail -3 && git add ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-dialogs.js scripts/test-easydrag-extra3.mjs && git commit -F C:/disposable/easydrag-w1b/a5-message.txt && git show --stat HEAD | tail -6
```
Expected: one commit, four files.

---

### Task A.6: header buttons, Undo and Redo take label, title and reason from the registry

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-header.js` (`runNowState` 36-41, `runNowTitle` 54-65 removed; `paintActions` new; `renderHeader` button block 88-125; the return line)
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (the header template 56-64, the overlay observer 565-569)
- Modify: `ui/js/desktop/apps/easydrag-history.js` (markup 19-22, `setOff` and `sync` 37-51, the click handler 65-70)
- Modify: `ui/css/desktop-app-easydrag.css` (seven rules around lines 320-337 and 418; CRLF kept by the helper)
- Modify: `scripts/test-easydrag-extra5.mjs`, `scripts/test-easydrag-extra7.mjs`, `ui/desktop_easydrag_blocks_browser_test.go` (readers of the old `disabled` and titles, listed below)

**Design.**
- **Markup stays.** The header template keeps its buttons, classes and `data-ed-cmd` values (W1c rebuilds the header). Their visible words come from the registry (`lab('flow.test')` = `esc(t(ED.actions.row(id).label))`), and `title`, `aria-disabled` and `aria-description` are painted by `paintActions` in `renderHeader`.
- **`aria-disabled`, not `disabled`.** An unavailable Test, Stop, Publish, Run now, the Active switch, Undo or Redo carries `aria-disabled="true"`, its reason in the title and in `aria-description`. The click handler already ignores `aria-disabled`, and `ED.actions.run` checks the state again, so a click or Enter on such a button does nothing. The CSS gives `[aria-disabled="true"]` the look of `:disabled` and removes the hover highlight.
- **Repaint on overlays.** A dialog or quick-add changes `free` and `quiet`; the editor's existing overlay observer repaints the header buttons and Undo/Redo as it already repaints the menus.
- **What goes.** `runNowState` and `runNowTitle`: the registry's checks (`writable`, `published`, `active`) and reasons replace them. The run view's Run now tooltip no longer names the run's start time ("Lauf vom …"); the banner above the canvas names it. "⋯" is a menu opener, not an action: it keeps its label key `more` and has no row.
- **Not changed:** the banner's own Stop, Re-run and Exit buttons, and the rail's palette button (not in the spec's list).

- [ ] **Step 6.1: impact**

Run the IMPACT command for `renderHeader`, `runNowState`, `runNowTitle` and `sync` (history). Expected: `renderHeader` is called by the editor (events, run view) and returned through `headerView`; `runNowState` by the editor (gone since A.3) and the header; `sync` by the history module and the editor's observer. Confirm with `grep -rn "runNowState\|runNowTitle\|history.sync\|headerView" ui/js scripts`.

- [ ] **Step 6.2: write and run the patch**

Write `C:/disposable/easydrag-w1b/patches/a6-patch.mjs`:

```js
// W1b A.6: the header buttons (Test, Stop, Publish, Run now, the Active switch) and Undo and Redo take their label, title and
// reason from the registry; their markup stays (W1c rebuilds the header). An unavailable button says aria-disabled, not disabled.
// Run from the worktree root: node C:/disposable/easydrag-w1b/a6-patch.mjs
import { edit, replaceBlock } from '../lib-patch.mjs';

const apps = 'ui/js/desktop/apps/';

// ── easydrag-header.js ────────────────────────────────────────────────────────────────────────────

const header = apps + 'easydrag-header.js';

// 1. runNowState and runNowTitle go: the registry's checks (writable, published, active) say why Run now is off.
replaceBlock(header, '        // runNowState: Run now starts the published version', '        // revisionText is the persistent draft-versus-live line.', '', 9);
replaceBlock(header, '        function runNowTitle() {', '        function paintRevision() {', `        // paintActions draws the action buttons from the registry: the title (label and shortcut), and for an unavailable button
        // aria-disabled with its reason in the title and in aria-description. A button never says "disabled": a keyboard user can
        // still reach it and read why, and the click handler (and ED.actions.run) refuse it.
        function paintActions() {
            const c = { host: ed };
            [['test', 'flow.test'], ['cancel', 'flow.stop'], ['publish', 'flow.publish'], ['run-now', 'flow.run_now'], ['active', 'flow.active']].forEach(([cmd, id]) => {
                const button = el.querySelector('[data-ed-cmd="' + cmd + '"]');
                if (button) ED.actions.paint(button, id, c);
            });
        }

`, 12);

// 2. renderHeader: no more disabled and title writes.
edit(header, `            if (isBlock) {
                btn('test').querySelector('span').textContent = t('easydrag.ui.block_simulate');
                btn('test').title = t('easydrag.ui.block_simulate');
                btn('runs').hidden = true; btn('active').hidden = true;`, `            if (isBlock) {
                btn('test').querySelector('span').textContent = ED.actions.label('flow.test', { host: ed });
                btn('runs').hidden = true; btn('active').hidden = true;`);
edit(header, `            btn('test').disabled = ro || running;
            btn('test').classList.toggle('is-busy', running);`, `            btn('test').classList.toggle('is-busy', running);`);
edit(header, `            btn('cancel').disabled = !!ed.run?.durableStatusError || btn('cancel').getAttribute('aria-busy') === 'true';
            btn('publish').disabled = ro;
            btn('publish').classList.toggle('has-changes', s.cls !== 'ok' && !ro);`, `            btn('publish').classList.toggle('has-changes', s.cls !== 'ok' && !ro);`);
edit(header, `            const runBtn = btn('run-now');
            if (runBtn) {
                const run = runNowState();
                const reason = runNowTitle();
                runBtn.hidden = isBlock || !ed.flow.live;
                runBtn.disabled = !!run.disabled;
                if (reason && ed.flow.live) { runBtn.title = reason; runBtn.setAttribute('title', reason); }
                else { runBtn.title = ''; runBtn.removeAttribute('title'); }
            }
            const sw = btn('active');
            sw.setAttribute('aria-checked', String(!!ed.flowEnabled));
            sw.disabled = !!ed.readonly || !!ed.runView;
`, `            btn('run-now').hidden = isBlock || !ed.flow.live;
            btn('active').setAttribute('aria-checked', String(!!ed.flowEnabled));
            paintActions();
`);
edit(header, `        return { renderHeader, renderSaveState, renderIssues, renderLastRun, runNowState };`, `        return { renderHeader, renderSaveState, renderIssues, renderLastRun, paintActions };`);

// ── easydrag-editor.js: the markup takes its words from the registry; the overlays repaint the buttons ──

const editor = apps + 'easydrag-editor.js';
edit(editor, `        const el = core.el('<div class="ed-editor">' +`, `        const lab = id => esc(t(ED.actions.row(id).label));
        const el = core.el('<div class="ed-editor">' +`);
edit(editor, `            '<button type="button" class="ed-btn" data-ed-cmd="test" title="' + esc(t('easydrag.ui.test_draft') + ' (' + core.shortcut('Ctrl+Enter') + ')') + '">' + ED.icons.of('test') + '<span>' + esc(t('easydrag.ui.test_draft')) + '</span></button>' +`,
    `            '<button type="button" class="ed-btn" data-ed-cmd="test">' + ED.icons.of('test') + '<span>' + lab('flow.test') + '</span></button>' +`);
edit(editor, `            '<button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" title="' + esc(t('easydrag.ui.run_cancel')) + '" aria-label="' + esc(t('easydrag.ui.run_cancel')) + '" hidden>' + ED.icons.of('stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-cmd="publish">' + ED.icons.of('publish') + '<span>' + esc(t('easydrag.ui.publish')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-cmd="run-now" hidden>' + ED.icons.of('run') + '<span>' + esc(t('easydrag.ui.run_now')) + '</span></button>' +
            '<button type="button" class="ed-switch-btn" role="switch" aria-checked="false" data-ed-cmd="active" title="' + esc(t('easydrag.ui.active')) + '" aria-label="' + esc(t('easydrag.ui.active')) + '"><span class="ed-switch" aria-hidden="true"><span></span></span><span>' + esc(t('easydrag.ui.active')) + '</span></button>' +`,
    `            '<button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" aria-label="' + lab('flow.stop') + '" hidden>' + ED.icons.of('stop') + '<span>' + lab('flow.stop') + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-cmd="publish">' + ED.icons.of('publish') + '<span>' + lab('flow.publish') + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-cmd="run-now" hidden>' + ED.icons.of('run') + '<span>' + lab('flow.run_now') + '</span></button>' +
            '<button type="button" class="ed-switch-btn" role="switch" aria-checked="false" data-ed-cmd="active" aria-label="' + lab('flow.active') + '"><span class="ed-switch" aria-hidden="true"><span></span></span><span>' + lab('flow.active') + '</span></button>' +`);
edit(editor, `            const overlays = new MutationObserver(() => setMenusSoon());`, `            const overlays = new MutationObserver(() => { setMenusSoon(); headerView.paintActions(); history.sync(true); });`);

// ── easydrag-history.js: Undo and Redo are the registry's edit.undo and edit.redo ─────────────────

const history = apps + 'easydrag-history.js';
edit(history, `        const hint = key => ' (' + core.shortcut(key) + ')';
        host.setAttribute('aria-label', t('easydrag.ui.history_label'));
        host.innerHTML = '<button type="button" class="ed-icon-btn" data-ed-history="undo" aria-label="' + esc(t('easydrag.ui.undo')) + '" title="' + esc(t('easydrag.ui.undo') + hint('Ctrl+Z')) + '">' + core.icon('undo') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-history="redo" aria-label="' + esc(t('easydrag.ui.redo')) + '" title="' + esc(t('easydrag.ui.redo') + hint('Ctrl+Shift+Z')) + '">' + core.icon('redo') + '</button>';
`, `        host.setAttribute('aria-label', t('easydrag.ui.history_label'));
        host.innerHTML = '<button type="button" class="ed-icon-btn" data-ed-history="undo">' + core.icon('undo') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-history="redo">' + core.icon('redo') + '</button>';
`);
edit(history, `        // Disabled buttons would drop the focus to the page: the buttons say aria-disabled instead,
        // and a click on them does nothing.
        function setOff(button, off) {
            if (off) button.setAttribute('aria-disabled', 'true');
            else button.removeAttribute('aria-disabled');
        }

        // sync follows the model shown: off in the run view and while there is nothing to undo or
        // redo; a read-only desktop shows no group at all.
        function sync() {
            const ro = !!(ed.readonly || ed.runView);
            host.hidden = !!ed.readonly;
            setOff(undoBtn, ro || !ed.model.canUndo());
            setOff(redoBtn, ro || !ed.model.canRedo());
        }
`, `        // sync follows the registry's edit.undo and edit.redo: label, shortcut, and aria-disabled with the reason (a disabled
        // button would drop the focus to the page); a read-only desktop shows no group at all. A drag moves the model on every
        // frame: the buttons are painted only when what they say may have changed (the editor forces it when an overlay opens or closes).
        let shown = '';
        function sync(force) {
            host.hidden = !!ed.readonly;
            const now = [ed.readonly, !!ed.runView, ed.model.canUndo(), ed.model.canRedo()].join();
            if (!force && now === shown) return;
            shown = now;
            ED.actions.paint(undoBtn, 'edit.undo', { host: ed }, { name: true });
            ED.actions.paint(redoBtn, 'edit.redo', { host: ed }, { name: true });
        }
`);
edit(history, `            if (b.dataset.edHistory === 'undo') opts.undo(); else opts.redo();
            sync();`, `            ED.actions.run(b.dataset.edHistory === 'undo' ? 'edit.undo' : 'edit.redo', { host: ed });
            sync();`);

// ── desktop-app-easydrag.css: an unavailable header button looks like a disabled one ────────────────

const css = 'ui/css/desktop-app-easydrag.css';
const off = ':not(:disabled):not([aria-disabled="true"])';
edit(css, `.ed-btn:hover:not(:disabled) {`, `.ed-btn:hover${off} {`);
edit(css, `.ed-btn:active:not(:disabled) {`, `.ed-btn:active${off} {`);
edit(css, `.ed-btn:disabled { opacity: 0.5; cursor: not-allowed; }`, `.ed-btn:disabled, .ed-btn[aria-disabled="true"] { opacity: 0.5; cursor: not-allowed; }`);
edit(css, `.ed-btn--primary:hover:not(:disabled) {`, `.ed-btn--primary:hover${off} {`);
edit(css, `.ed-btn--danger:hover:not(:disabled) {`, `.ed-btn--danger:hover${off} {`);
edit(css, `.ed-btn--ghost:hover:not(:disabled) {`, `.ed-btn--ghost:hover${off} {`);
edit(css, `.ed-switch-btn:disabled { opacity: 0.5; cursor: not-allowed; }`, `.ed-switch-btn:disabled, .ed-switch-btn[aria-disabled="true"] { opacity: 0.5; cursor: not-allowed; }`);

// ── tests that read the header buttons ─────────────────────────────────────────────────────────────
// A header button is unavailable by aria-disabled, never disabled; its title is the label, the shortcut and the reason
// ("Run now – reason_paused" with the key stub of the Node harness; the dash is the en dash).
const extra5 = 'scripts/test-easydrag-extra5.mjs';
edit(extra5, `        const before = [!btn('stop-viewed').hidden, btn('test').disabled, btn('publish').disabled, btn('active').disabled, !!(del && del.disabled)];`,
    `        const off = button => button.getAttribute('aria-disabled') === 'true';
        const before = [!btn('stop-viewed').hidden, off(btn('test')), off(btn('publish')), off(btn('active')), !!(del && del.disabled)];`);
edit(extra5, `            [liveBtn.hidden, liveBtn.disabled, liveBtn.getAttribute('title'), on.editor.el.querySelector('[data-ed-revision]').textContent,`,
    `            [liveBtn.hidden, liveBtn.getAttribute('aria-disabled') === 'true', liveBtn.getAttribute('title'), on.editor.el.querySelector('[data-ed-revision]').textContent,`);
edit(extra5, `            [false, false, null, 'revision_live:1', 1]);`, `            [false, false, 'run_now', 'revision_live:1', 1]);`);
edit(extra5, `            [pausedBtn.hidden, pausedBtn.disabled, pausedBtn.getAttribute('title') || pausedBtn.title,`,
    `            [pausedBtn.hidden, pausedBtn.getAttribute('aria-disabled') === 'true', pausedBtn.getAttribute('title'),`);
edit(extra5, `            [false, true, 'error_flow_disabled', 0]);`, `            [false, true, 'run_now ' + String.fromCodePoint(0x2013) + ' reason_paused', 0]);`);
edit(extra5, `            [roBtn.hidden, roBtn.disabled, roBtn.getAttribute('title') || roBtn.title],
            [false, true, 'run_readonly']);`, `            [roBtn.hidden, roBtn.getAttribute('aria-disabled') === 'true', roBtn.getAttribute('title')],
            [false, true, 'run_now ' + String.fromCodePoint(0x2013) + ' reason_readonly']);`);
edit(extra5, `            [viewBtn.hidden, viewBtn.disabled, viewBtn.getAttribute('title') || viewBtn.title],
            [false, true, 'state_run_view']);`, `            [viewBtn.hidden, viewBtn.getAttribute('aria-disabled') === 'true', viewBtn.getAttribute('title')],
            [false, true, 'run_now ' + String.fromCodePoint(0x2013) + ' reason_run_view']);`);
edit(extra5, `        const timedBtn = cmdBtn(ahead.el, 'run-now');
        const timedTitle = timedBtn.getAttribute('title') || timedBtn.title;
        eq('ux the run view tooltip names the start time, not the chip text again',
            [timedTitle.startsWith('state_run_view_started:'), timedTitle.length > 'state_run_view_started:'.length, timedTitle === 'state_run_view'],
            [true, true, false]);`, `        const timedBtn = cmdBtn(ahead.el, 'run-now');
        const timedTitle = timedBtn.getAttribute('title');
        eq('ux the run view tooltip gives the registry reason, not the chip text again (the start time is in the banner)',
            [timedTitle.endsWith('reason_run_view'), timedTitle === 'state_run_view', timedBtn.getAttribute('aria-description')],
            [true, false, 'reason_run_view']);`);
const extra7 = 'scripts/test-easydrag-extra7.mjs';
edit(extra7, `            const running = testBtn.disabled;`, `            const off = button => button.getAttribute('aria-disabled') === 'true';
            const running = off(testBtn);`);
edit(extra7, `            for (; used < rounds && testBtn.disabled; used++) {`, `            for (; used < rounds && off(testBtn); used++) {`);
edit(extra7, `const row = [running, testBtn.disabled, editor.ed.run.status,`, `const row = [running, off(testBtn), editor.ed.run.status,`);
edit('ui/desktop_easydrag_blocks_browser_test.go', `document.querySelector('[data-ed-cmd="publish"]').disabled && document.querySelector('[data-ed-cmd="test"]').disabled`,
    `document.querySelector('[data-ed-cmd="publish"]').getAttribute('aria-disabled')==='true' && document.querySelector('[data-ed-cmd="test"]').getAttribute('aria-disabled')==='true'`);

console.log('A.6 patch applied');
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1b/patches/a6-patch.mjs
```
Expected: `A.6 patch applied`.

- [ ] **Step 6.3: prove no old form remains**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
grep -n "runNowState\|runNowTitle\|\.disabled = \|setOff\|hint('Ctrl" ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-history.js ui/js/desktop/apps/easydrag-editor.js
grep -nE 'data-ed-cmd="(test|cancel|publish|run-now|active)"[^>]* title=' ui/js/desktop/apps/easydrag-editor.js
grep -c "ED.actions" ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-history.js ui/js/desktop/apps/easydrag-editor.js
wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-history.js
```
Expected: the first two greps print nothing; the counts (lines that mention `ED.actions`) are 3 / 3 / 9; `wc -l`: editor 852 (896 at baseline), header 200 (223), history 79 (83).

- [ ] **Step 6.4: run the checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag.mjs 2>&1 | tail -3 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_blocks_browser_test.go ui/desktop_easydrag_browser_test.go
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no vet or gofmt output. The patch updates these expectations, each for one stated cause:
  - `merge read-only desktop …` and `a1023 F5 …` (two tests) read the buttons by `aria-disabled`, not `disabled`.
  - `ux a published flow shows Run now …`: the title is now the label (`run_now`) instead of none.
  - `ux a paused live flow …`, `ux read-only still shows Run now …`, `ux the run view keeps Run now …`: the title is the label plus the reason (`run_now – reason_paused`, `– reason_readonly`, `– reason_run_view`), the state `aria-disabled`.
  - `ux the run view tooltip names the start time …` becomes "gives the registry reason, not the chip text again": the start time is no longer in the button's title.
  - `desktop_easydrag_blocks_browser_test.go`: the block editor's Publish and Test are read by `aria-disabled`.
If the render baseline changes, something in a card or the header re-renders more often than before: find the cause (the tools are painted from `ED.actions.button` with `cached: true`, so they must not add `renderNode` calls); do not update the baseline.

- [ ] **Step 6.5: gates and commit**

Write `C:/disposable/easydrag-w1b/a6-message.txt`:
```
feat(easydrag): header buttons and Undo/Redo take label, title and reason from the registry

Test, Stop, Publish, Run now, the Active switch, Undo and Redo get their words
from the action registry and say why they are unavailable in the title and in
aria-description; an unavailable one carries aria-disabled and never disabled.
The header markup is unchanged. runNowState and runNowTitle go: the registry's
published, active and writable checks replace them. The overlay observer repaints
the buttons with the menus. The run view's Run now tooltip no longer names the
run's start time (the banner does).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node scripts/test-easydrag-t0.mjs --node-only 2>&1 | tail -3 && git add ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-history.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-extra5.mjs scripts/test-easydrag-extra7.mjs ui/desktop_easydrag_blocks_browser_test.go && git commit -F C:/disposable/easydrag-w1b/a6-message.txt && git show --stat HEAD | tail -9
```
Expected: one commit, seven files.

---

### Task A.7: retire the duplicate label keys; the Go lint `TestEasyDragMenusUseTheActionRegistry`

**Files:**
- Create: `ui/desktop_easydrag_actions_test.go`
- Create (by the coordinator, from this plan's `keys/` folder): `plans/2026-11-15-easydrag-w1b/keys/a-7.json`
- Modify: `ui/lang/easydrag/*.json` (16 files, through the key file only)

**What goes.** Eleven keys that no code uses any more, because every surface reads the registry's one label per action: `menu_copy`, `menu_paste` ("Hier einfügen"; "Einfügen" replaces it), `menu_select_all`, `node_duplicate`, `node_delete` (duplicates of `copy`, `paste`, `select_all`, `duplicate`, `delete`), `keys_open`, `keys_delete`, `keys_fit`, `keys_zoom`, `keys_help` (the table's own spellings of "Open", "Delete", "Show everything", "Zoom in", "Keyboard shortcuts"), and `test` (the Flow menu's "Testen"; every surface says `test_draft`). The key file `a-7.json` is `{ "_delete": [ … ] }` with the full key names.

**The lint.** It reads every EasyDrag module except `easydrag-actions.js` and `easydrag-workspace.js` (the panel width menu, which offers pixel sizes and is the one exception, asserted to open exactly one menu) and fails on:
  - a `shortcut:` or `shortcutHint:` property anywhere (menu entries get their shortcut from the registry);
  - a line with `label:` or `labelKey:` and a function `action:` or a `disabledHint:`, or, with an `icon:`, a literal `items:` (a menu entry or submenu with its own words; a kit toast's `action: {` object is not a menu entry);
  - a `showContextMenu(` or `setWindowMenus(` call whose arguments do not contain `ED.actions.items(`.
Comment lines are skipped. It also asserts that it saw at least five menu calls (the lint must see the menus it guards).

- [ ] **Step 7.1: write the lint and run it (it passes: A.2 to A.5 migrated every menu)**

Create `ui/desktop_easydrag_actions_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// W1b Part A, the menu lint. EasyDrag menus list action ids and render through ED.actions.items, so a label, a
// shortcut and the reason an entry is off live in easydrag-actions.js only. This test fails when another EasyDrag module
//   - gives a menu entry a `shortcut` or `shortcutHint` of its own,
//   - builds a menu entry (a line with `label:` or `labelKey:` and `action:`, `disabledHint:` or, with an `icon:`, a literal `items:`), or
//   - opens a context menu or sets window menus with anything but ED.actions.items(...).
//
// The one exception is the panel width menu of easydrag-workspace.js: it offers pixel sizes, not actions.
func TestEasyDragMenusUseTheActionRegistry(t *testing.T) {
	keyField := regexp.MustCompile(`\bshortcut(?:Hint)?\s*:`)
	labelField := regexp.MustCompile(`\blabel(?:Key)?\s*:`)
	// A menu entry's action is a function; a toast's action is an object ("action: {").
	entryField := regexp.MustCompile(`\baction\s*:\s*[(A-Za-z_$]|\bdisabledHint\s*:`)
	itemsField := regexp.MustCompile(`\bitems\s*:`)
	iconField := regexp.MustCompile(`\bicon\s*:`)
	menuCall := regexp.MustCompile(`\b(?:showContextMenu|setWindowMenus)\(`)
	calls := 0
	for _, name := range easyDragModules {
		if name == "easydrag-actions.js" || name == "easydrag-workspace.js" {
			continue
		}
		text := strings.ReplaceAll(readDesktopAssetText(t, "js/desktop/apps/"+name), "\r\n", "\n")
		for n, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if keyField.MatchString(line) {
				t.Errorf("%s:%d gives a menu entry its own shortcut: the registry draws shortcuts\n\t%s", name, n+1, strings.TrimSpace(line))
			}
			if labelField.MatchString(line) && (entryField.MatchString(line) || itemsField.MatchString(line) && iconField.MatchString(line) && !strings.Contains(line, ".items(")) {
				t.Errorf("%s:%d builds a menu entry with its own label: list the action id and render through ED.actions.items\n\t%s", name, n+1, strings.TrimSpace(line))
			}
		}
		for _, at := range menuCall.FindAllStringIndex(text, -1) {
			calls++
			args := easyDragCallArguments(text, at[1]-1)
			if !strings.Contains(args, "ED.actions.items(") {
				line := 1 + strings.Count(text[:at[0]], "\n")
				t.Errorf("%s:%d opens a menu without ED.actions.items(...) in its arguments", name, line)
			}
		}
	}
	// The lint must see the menus it guards: editor (window menus, more menu), interact (card and canvas menu), home (card menu)
	// and the shell (start page window menu).
	if calls < 5 {
		t.Errorf("the lint found %d menu calls, expected at least 5 (window menus, more menu, card menu, canvas menu, start page menus, home card menu)", calls)
	}
	workspace := readDesktopAssetText(t, "js/desktop/apps/easydrag-workspace.js")
	if got := len(menuCall.FindAllStringIndex(workspace, -1)); got != 1 {
		t.Errorf("easydrag-workspace.js opens %d menus: the panel width menu is the one exception", got)
	}
}

// easyDragCallArguments returns the argument text of the call whose "(" is at open: up to and including the matching ")".
// It skips string literals, so a parenthesis inside a label does not count.
func easyDragCallArguments(text string, open int) string {
	depth := 0
	for i := open; i < len(text); i++ {
		switch c := text[i]; c {
		case '\'', '"', '`':
			for i++; i < len(text) && text[i] != c; i++ {
				if text[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[open : i+1]
			}
		}
	}
	return text[open:]
}
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)TestEasyDragMenusUseTheActionRegistry' -count=1 -v 2>&1 | tail -4
```
Expected: `--- PASS: TestEasyDragMenusUseTheActionRegistry`. A failure names `file:line` and the line: a menu entry that A.2 to A.5 did not migrate. Migrate it (list its action id; if the action has no row, add the row in `easydrag-actions.js` with its label key, and bind it); do not exempt it.

- [ ] **Step 7.2: prove the lint bites**

Put one violation in, see the failure, take it out again:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions
node -e "const fs=require('fs');const f='ui/js/desktop/apps/easydrag-delight.js';fs.writeFileSync(f,fs.readFileSync(f,'utf8').replace('    ED.delight = {','    const probe = [{ icon: \'x\', label: \'Probe\', action: () => 1 }];\n    ED.delight = {'))"
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)TestEasyDragMenusUseTheActionRegistry' -count=1 2>&1 | head -5
git checkout -- ui/js/desktop/apps/easydrag-delight.js && git status --short ui/js | head -3
```
Expected: the test fails with `easydrag-delight.js:NN builds a menu entry with its own label`; after the checkout, `git status` shows no change to the delight file (it was committed in A.3; the checkout restores that version).

- [ ] **Step 7.3: confirm the eleven keys are unused**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && for k in test menu_copy menu_paste menu_select_all node_duplicate node_delete keys_open keys_delete keys_fit keys_zoom keys_help; do printf "%s: " $k; grep -rn "easydrag\.ui\.$k['\"]" ui/js scripts ui/*.go internal --include=*.js --include=*.mjs --include=*.go | grep -v "ui/js/desktop/bundles" | wc -l; done
```
Expected: `0` for every key. A key with a count above 0 stays: take it out of `a-7.json` and tell the coordinator which surface still reads it.

- [ ] **Step 7.4: apply the key file**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/a-7.json && git diff --stat -- ui/lang/easydrag | tail -2
node scripts/test-easydrag-actions.mjs && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragUIKeysExistInAllLocales|TestEasyDragMenusUseTheActionRegistry)' -count=1 2>&1 | tail -3
```
Expected: `a-7.json: 0 keys set, 11 deleted, in 16 files of ui/lang/easydrag/`; 16 files changed with 11 deletions each; `PASS action registry: 54 rows`; `ok`.

- [ ] **Step 7.5: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_actions_test.go && node scripts/test-easydrag.mjs 2>&1 | tail -2
```
Expected: no vet or gofmt output, `all EasyDrag checks passed`. Write `C:/disposable/easydrag-w1b/a7-message.txt`:
```
test(easydrag): menu lint for the action registry; retire eleven duplicate keys

TestEasyDragMenusUseTheActionRegistry fails when an EasyDrag module outside
easydrag-actions.js gives a menu entry its own label or shortcut, or opens a menu
without ED.actions.items (the panel width menu of easydrag-workspace.js is the
one exception). The keys menu_copy, menu_paste, menu_select_all, node_duplicate,
node_delete, keys_open, keys_delete, keys_fit, keys_zoom, keys_help and test leave
all 16 locales: no code uses them since every surface reads the registry's label.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && git add ui/desktop_easydrag_actions_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1b/a7-message.txt && git show --stat HEAD | tail -3
```
Expected: one commit, 17 files.

---

### Task A.8: the browser test `TestEasyDragActionsAgreeBrowser`

**Files:**
- Create: `ui/desktop_easydrag_actions_browser_test.go`

**What it does.** In four states (normal with one step selected; nothing selected; a run in progress; a past run open) it opens every surface that exists in that state and reads it into one list of `{surface, id, label, keys, enabled, reason}`: the window menus (from `aurora.state.windowMenus`), the "more" menu, the card menu, the canvas menu (context-menu DOM, mapped to registry rows by their item id), the selected card's tools, the header buttons (Test, Stop, Publish, Run now, Active, Undo, Redo), and the shortcut table. Then it fails when
  - an action shows a different label, availability or reason on two surfaces;
  - a surface shows a shortcut other than `ED.actions.display(row.shortcut)`, or the table's label differs from the registry's;
  - an unavailable entry has no reason;
  - an action is listed on fewer surfaces than the test's `need` table says (so the test cannot pass by finding nothing).
It also pins one fact per state (Undo says there is nothing to undo; Cut and Duplicate say to select a step; Test and Test this step say a run is going while Stop is on; Save and Duplicate say a past run is open). After the four states it presses every registry shortcut once on the focused canvas (a keydown on `.ed-canvas`; `ED.actions.observe` counts the registry's runs) and requires the count to equal what `ED.actions.state` said (one when enabled, none when refused). It finally opens the start page's card menu and checks that each entry carries the registry's label and that every unavailable one has a reason.

The test is long because it drives the real editor; it adds one browser test to the suite (about 40 s).

- [ ] **Step 8.1: write the test**

Create `ui/desktop_easydrag_actions_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// easyDragActionsHelper installs window.__acts: collectors that read every surface an action appears on (the window menus, the
// "more" menu, the card menu, the canvas menu, the card tools, the header buttons and the shortcut table) into one list of
// {surface, id, label, keys, enabled, reason}, and analyse(), which compares them. An entry's id is the registry row it maps to
// (a menu item id is the row's id or its legacy window-menu id).
const easyDragActionsHelper = `()=>{
	const ED=window.EasyDrag, A=ED.actions;
	const strip=x=>String(x==null?'':x).replace(/\s+/g,' ').trim();
	const rowOf=id=>A.row(id)||A.ROWS.find(r=>r.menu===id)||null;
	const ed=()=>edFixture.editor();
	const list=[];
	const push=(surface,row,label,keys,enabled,reason)=>{if(row)list.push({surface,id:row.id,label:strip(label),keys:strip(keys),enabled:!!enabled,reason:strip(reason)});};
	const titleKeys=(title,label)=>{const m=/^ \(([^)]*)\)/.exec(strip(title).slice(label.length));return m?m[1]:'';};
	window.__acts={
		windowMenus(){
			const menus=aurora.state.windowMenus.get(ed().windowId).renderedMenus.filter(m=>m.id!=='window');
			const visit=item=>{
				if(item.type==='separator')return;
				if(item.type==='submenu'){item.items.forEach(visit);return;}
				push('window',rowOf(item.id),item.label||ed().t(item.labelKey),item.shortcut||item.shortcutHint,!item.disabled,item.disabled?item.disabledHint:'');
			};
			menus.forEach(m=>m.items.forEach(visit));
		},
		context(surface){
			document.querySelectorAll('.vd-context-menu [data-context-action]').forEach(b=>{
				const kbd=b.querySelector('.vd-context-shortcut');
				push(surface,rowOf(b.dataset.contextAction.split('/').pop()),b.querySelector('.vd-context-label').textContent,kbd?kbd.textContent:'',!b.disabled,b.disabled?b.getAttribute('title'):'');
			});
		},
		tools(){
			const card=document.querySelector('.ed-node.is-selected');
			if(!card)return;
			card.querySelectorAll('[data-ed-node-tool]').forEach(b=>{
				const found=A.TOOLS.find(([tool])=>tool===b.dataset.edNodeTool);
				const label=b.getAttribute('aria-label');
				push('tool',rowOf(found[1]),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true'&&!b.disabled,b.getAttribute('aria-description'));
			});
		},
		header(){
			const ids={test:'flow.test',cancel:'flow.stop',publish:'flow.publish','run-now':'flow.run_now',active:'flow.active'};
			document.querySelectorAll('.ed-head [data-ed-cmd]').forEach(b=>{
				const id=ids[b.dataset.edCmd];
				if(!id||b.hidden||!b.getClientRects().length)return;
				const label=strip(b.textContent);
				push('header',rowOf(id),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true'&&!b.disabled,b.getAttribute('aria-description'));
			});
			document.querySelectorAll('.ed-head [data-ed-history]').forEach(b=>{
				if(!b.getClientRects().length)return;
				const label=b.getAttribute('aria-label');
				push('header',rowOf(b.dataset.edHistory==='undo'?'edit.undo':'edit.redo'),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true',b.getAttribute('aria-description'));
			});
		},
		table(){
			const dialog=ED.dialogs.shortcuts(ed());
			dialog.el.parentNode.querySelectorAll('.ed-keys-row').forEach(r=>{
				const keys=[...r.querySelectorAll('dt kbd')].map(k=>k.textContent).join('+');
				const row=A.ROWS.find(x=>x.shortcut&&A.display(x.shortcut,ed().t)===keys);
				if(row)list.push({surface:'table',id:row.id,label:strip(r.querySelector('dd').textContent),keys,enabled:true,reason:''});
			});
			dialog.close(null);
		},
		reset(){list.length=0;},
		// analyse compares the entries of one state: the same label, displayed shortcut, availability and reason on every
		// surface that lists an action, a reason on every unavailable entry. need maps an action id to the fewest surfaces it
		// must appear on.
		analyse(need){
			const problems=[], byId=new Map();
			list.forEach(e=>{
				const row=A.row(e.id), marked=e.id.startsWith('view.liveliness');
				const want=A.display(row.shortcut,ed().t);
				if(!marked&&e.surface!=='table'&&e.keys!==want)problems.push(e.surface+' '+e.id+': shortcut "'+e.keys+'", the registry says "'+want+'"');
				if(e.surface==='table'&&e.label!==strip(A.label(e.id,{host:ed()})))problems.push('table '+e.id+': label "'+e.label+'"');
				if(e.surface!=='table'&&!e.enabled&&!e.reason)problems.push(e.surface+' '+e.id+': unavailable without a reason');
				if(!byId.has(e.id))byId.set(e.id,[]);
				byId.get(e.id).push(e);
			});
			byId.forEach((entries,id)=>{
				const live=entries.filter(e=>e.surface!=='table');
				live.slice(1).forEach(e=>{
					if(e.label!==live[0].label)problems.push(id+': label "'+e.label+'" on '+e.surface+', "'+live[0].label+'" on '+live[0].surface);
					if(e.enabled!==live[0].enabled)problems.push(id+': '+(e.enabled?'available':'unavailable')+' on '+e.surface+', '+(live[0].enabled?'available':'unavailable')+' on '+live[0].surface);
					if(e.reason!==live[0].reason)problems.push(id+': reason "'+e.reason+'" on '+e.surface+', "'+live[0].reason+'" on '+live[0].surface);
				});
			});
			Object.keys(need).forEach(id=>{const n=new Set((byId.get(id)||[]).map(e=>e.surface)).size;if(n<need[id])problems.push(id+' is listed on '+n+' surfaces, expected at least '+need[id]);});
			const out={problems,compared:byId.size,facts:Object.fromEntries([...byId].map(([id,entries])=>[id,entries.filter(e=>e.surface!=='table').map(e=>e.enabled?'on':e.reason)[0]||'']))};
			list.length=0;
			return JSON.stringify(out);
		}
	};
}`

type easyDragActionsReport struct {
	Problems []string          `json:"problems"`
	Compared int               `json:"compared"`
	Facts    map[string]string `json:"facts"`
}

// actionsMenu opens a context menu (by a click or a JS expression), collects it as a surface and closes it.
func (s *easyDragSmoke) actionsMenu(surface, open string) {
	s.t.Helper()
	s.page.MustEval(open)
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action]')`)
	s.page.MustEval(`(surface)=>window.__acts.context(surface)`, surface)
	s.page.MustEval(`()=>aurora.closeContextMenu()`)
	s.wait(`()=>!document.querySelector('.vd-context-menu')`)
}

// actionsAgree collects the surfaces of one state and fails on the first disagreement. A surface is one of
// window, more, card, canvas, tools, header, table. need maps action ids to the fewest surfaces they must appear on.
func (s *easyDragSmoke) actionsAgree(state string, surfaces []string, need map[string]int) easyDragActionsReport {
	s.t.Helper()
	p := s.page
	p.MustEval(`()=>window.__acts.reset()`)
	for _, surface := range surfaces {
		switch surface {
		case "window":
			p.MustEval(`()=>window.__acts.windowMenus()`)
		case "more":
			s.actionsMenu("more", `()=>document.querySelector('[data-ed-cmd="more"]').click()`)
		case "card":
			s.actionsMenu("card", `()=>{const c=document.querySelector('.ed-node.is-selected')||document.querySelector('.ed-node');const r=c.getBoundingClientRect();c.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.left+20,clientY:r.top+20}));}`)
		case "canvas":
			s.actionsMenu("canvas", `()=>{const c=document.querySelector('.ed-canvas');const r=c.getBoundingClientRect();c.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.left+6,clientY:r.bottom-6}));}`)
		case "tools":
			p.MustEval(`()=>window.__acts.tools()`)
		case "header":
			p.MustEval(`()=>window.__acts.header()`)
		case "table":
			p.MustEval(`()=>window.__acts.table()`)
			s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
		default:
			s.t.Fatalf("unknown surface %q", surface)
		}
	}
	needJSON, err := json.Marshal(need)
	if err != nil {
		s.t.Fatal(err)
	}
	var report easyDragActionsReport
	if err := json.Unmarshal([]byte(p.MustEval(`(need)=>window.__acts.analyse(JSON.parse(need))`, string(needJSON)).Str()), &report); err != nil {
		s.t.Fatal(err)
	}
	if len(report.Problems) > 0 {
		s.t.Fatalf("%s: the surfaces disagree:\n\t%s", state, strings.Join(report.Problems, "\n\t"))
	}
	s.t.Logf("%s: %d actions compared on %v", state, report.Compared, surfaces)
	return report
}

// TestEasyDragActionsAgreeBrowser opens every surface of the action registry in four states (normal, nothing selected, a run in
// progress, a past run open) and checks that each action shows the same label, displayed shortcut, availability and reason on
// every surface that lists it, that every unavailable entry carries a reason, and that each registry shortcut fires its action once.
func TestEasyDragActionsAgreeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	p.MustEval(easyDragActionsHelper)
	select1 := `()=>{const ed=edFixture.editor();const id=ed.model.doc.nodes.find(n=>!ed.model.info(n.type).trigger).id;ed.bus.emit('focus-node',id);}`
	s.settle()

	// 1. Normal: the published, switched-on flow with one step selected. Every surface is open.
	p.MustEval(select1)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	normal := s.actionsAgree("normal", []string{"window", "more", "card", "canvas", "tools", "header", "table"}, map[string]int{
		"edit.duplicate": 4, "edit.delete": 4, "step.test": 2, "step.toggle": 3, "flow.test": 3, "flow.run_now": 3, "edit.undo": 3, "edit.select_all": 3, "view.zoom_fit": 3,
	})
	if normal.Facts["flow.test"] != "on" || normal.Facts["edit.duplicate"] != "on" {
		t.Fatalf("normal: Test and Duplicate must be available, got %v", normal.Facts)
	}
	if !strings.Contains(normal.Facts["edit.undo"], "Nichts") {
		t.Fatalf("normal: Undo has nothing to undo and says so, got %q", normal.Facts["edit.undo"])
	}

	// 2. Nothing selected.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.selection.clear();ed.bus.emit('selection',ed.selection);}`)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===0`)
	s.settle()
	none := s.actionsAgree("nothing selected", []string{"window", "more", "canvas", "header", "table"}, map[string]int{
		"edit.cut": 1, "edit.copy": 1, "edit.duplicate": 1, "edit.extract": 2, "flow.test": 3, "view.find_step": 3,
	})
	if !strings.Contains(none.Facts["edit.cut"], "Schritt") || !strings.Contains(none.Facts["edit.duplicate"], "Schritt") {
		t.Fatalf("nothing selected: Cut and Duplicate say to select a step first, got %v", none.Facts)
	}

	// 3. A run in progress: Test is off ("Ein Lauf läuft gerade."), Stop is on, edits stay on.
	p.MustEval(`()=>{edFixture.state.delay=2500;}`)
	p.MustEval(select1)
	p.MustEval(`()=>window.EasyDrag.actions.run('flow.test',{host:edFixture.editor()})`)
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const stop=document.querySelector('[data-ed-cmd="cancel"]');return !!stop && !stop.hidden && !document.querySelector('.ed-modal-backdrop')}`)
	p.MustEval(select1)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	running := s.actionsAgree("run in progress", []string{"window", "more", "card", "canvas", "tools", "header", "table"}, map[string]int{
		"flow.test": 3, "step.test": 2, "flow.stop": 1, "edit.duplicate": 4,
	})
	if !strings.Contains(running.Facts["flow.test"], "Lauf") || !strings.Contains(running.Facts["step.test"], "Lauf") || running.Facts["flow.stop"] != "on" {
		t.Fatalf("run in progress: Test and Test this step say a run is going and Stop is on, got %v", running.Facts)
	}
	p.MustElement(`[data-ed-cmd="cancel"]`).MustClick()
	s.wait(`()=>{const stop=document.querySelector('[data-ed-cmd="cancel"]');return !stop || stop.hidden}`)
	p.MustEval(`()=>{edFixture.state.delay=140;}`)

	// 4. A past run open: nothing can be changed, and every entry says so.
	p.MustEval(`()=>{const id=edFixture.addWaitingRun();Array.from(window.EasyDragApp._instances.values())[0].screen.showRun(id);}`)
	s.wait(`()=>!document.querySelector('.ed-runview-banner').hidden`)
	s.settle()
	view := s.actionsAgree("past run", []string{"window", "more", "card", "canvas", "header", "table"}, map[string]int{
		"flow.save": 2, "flow.test": 3, "edit.duplicate": 2, "step.test": 1, "view.palette": 1,
	})
	if !strings.Contains(view.Facts["flow.save"], "Lauf-Rückblick") || !strings.Contains(view.Facts["edit.duplicate"], "Lauf-Rückblick") {
		t.Fatalf("past run: Save and Duplicate say a past run is open, got %v", view.Facts)
	}
	p.MustElement(`[data-ed-cmd="exit-run-view"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	s.settle()

	// 5. Each registry shortcut fires its action once, or is refused with its reason (nothing runs). The key goes to the canvas, as
	// a user's would; observe() counts the runs of the registry.
	ids := p.MustEval(`()=>window.EasyDrag.actions.ROWS.filter(r=>r.shortcut).map(r=>r.id)`).Arr()
	fired := 0
	for _, raw := range ids {
		id := raw.Str()
		p.MustEval(select1)
		s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1 && !document.querySelector('.ed-modal-backdrop') && !document.querySelector('.ed-detail-backdrop') && !document.querySelector('.ed-quick')`)
		got := p.MustEval(`(id)=>{
			const ED=window.EasyDrag, A=ED.actions, ed=edFixture.editor(), row=A.row(id);
			const node=ed.model.doc.nodes[1].id;
			if(id==='edit.undo'||id==='edit.redo'){ed.model.moveNodes([node],40,0);if(id==='edit.redo')ed.model.undo();}
			const canvas=document.querySelector('.ed-canvas');canvas.focus();
			const expected=A.state(id,{host:ed}).enabled?1:0;
			const parts=row.shortcut.split('+');let key=parts[parts.length-1];
			if(row.shortcut==='+'){key='+';}
			const shift=parts.includes('Shift')||['+','?'].includes(row.shortcut), alt=parts.includes('Alt'), ctrl=parts.includes('Ctrl');
			if(row.shortcut==='Shift+1')key='!';
			if(key.length===1&&(shift&&/[a-z]/i.test(key)))key=key.toUpperCase();else if(key.length===1&&/[A-Z]/.test(key))key=key.toLowerCase();
			let count=0;const off=A.observe(()=>count++);
			canvas.dispatchEvent(new KeyboardEvent('keydown',{key,ctrlKey:ctrl,shiftKey:shift,altKey:alt,bubbles:true,cancelable:true,composed:true}));
			off();
			return JSON.stringify({id,expected,count});
		}`, id).Str()
		var res struct {
			ID       string `json:"id"`
			Expected int    `json:"expected"`
			Count    int    `json:"count"`
		}
		if err := json.Unmarshal([]byte(got), &res); err != nil {
			t.Fatal(err)
		}
		if res.Count != res.Expected {
			t.Fatalf("shortcut of %s: ran %d times, expected %d", id, res.Count, res.Expected)
		}
		fired += res.Count
		// What the shortcut opened (a dialog, the step view, quick-add) closes with Escape before the next one.
		for i := 0; i < 4 && p.MustEval(`()=>!!(document.querySelector('.ed-modal-backdrop')||document.querySelector('.ed-detail-backdrop')||document.querySelector('.ed-quick'))`).Bool(); i++ {
			p.Keyboard.MustType(input.Escape)
			p.MustEval(`()=>new Promise(r=>setTimeout(r,200))`)
		}
		s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && !document.querySelector('.ed-detail-backdrop') && !document.querySelector('.ed-quick')`)
	}
	if fired < 17 {
		t.Fatalf("only %d of %d registry shortcuts fired an action", fired, len(ids))
	}
	s.t.Logf("shortcuts: %d of %d fired once, the rest were refused with their reason", fired, len(ids))

	// 6. The start page's card menu: the card's flow is the context; its entries carry the registry's labels.
	p.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustEval(`()=>document.querySelector('[data-ed-card-menu]').click()`)
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action]')`)
	home := p.MustEval(`()=>{
		const A=window.EasyDrag.actions, t=Array.from(window.EasyDragApp._instances.values())[0].t, out=[];
		document.querySelectorAll('.vd-context-menu [data-context-action]').forEach(b=>{
			const row=A.row(b.dataset.contextAction)||A.ROWS.find(r=>r.menu===b.dataset.contextAction);
			const label=b.querySelector('.vd-context-label').textContent;
			if(!row)out.push('unknown entry '+b.dataset.contextAction);
			else if(label!==t(row.label))out.push(row.id+': label "'+label+'", the registry says "'+t(row.label)+'"');
			if(b.disabled&&!b.getAttribute('title'))out.push((row&&row.id)+': unavailable without a reason');
		});
		return JSON.stringify({problems:out,count:document.querySelectorAll('.vd-context-menu [data-context-action]').length});
	}`).Str()
	var homeReport struct {
		Problems []string `json:"problems"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(home), &homeReport); err != nil {
		t.Fatal(err)
	}
	if len(homeReport.Problems) > 0 || homeReport.Count < 7 {
		t.Fatalf("start page card menu: %d entries, problems %v", homeReport.Count, homeReport.Problems)
	}
	p.MustEval(`()=>aurora.closeContextMenu()`)
	s.failOnPageErrors("actions agree")
}
```

- [ ] **Step 8.2: run it**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' gofmt -l ui/desktop_easydrag_actions_browser_test.go
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)TestEasyDragActionsAgreeBrowser' -count=1 -timeout 10m -v 2>&1 | tail -20
```
Run it with the Bash sandbox off (Chrome cannot start from a test binary under the repo). Expected: `--- PASS: TestEasyDragActionsAgreeBrowser` with the log lines `normal: N actions compared on [window more card canvas tools header table]`, the same for the other three states, and `shortcuts: 21 of 21 fired once …` (a refused shortcut counts as "refused with its reason": the line says how many fired). The rod page has a 150 s deadline: if the test times out, re-run it alone once before counting a failure.

What a failure means, and what to do. The test reports `the surfaces disagree` with one line per problem; each names the action id and the two surfaces.
  - **A label differs** (`flow.test: label "…" on header, "…" on window`): a surface still reads its own key. Find it with `grep -n "easydrag.ui.<key>" ui/js/desktop/apps/*.js` and route it through `ED.actions`.
  - **A reason or availability differs**: two surfaces evaluate different contexts. The card menu and the card tools use `{ host: ed, node }`, the menus `{ host: ed }`. A difference between "a selected step" and "the menu's step" means the selection changed between the surfaces (the test selects once per state), so check that the opening of a menu did not change the selection.
  - **`unavailable without a reason`**: a bound `check` returned a key that has no text (`t` returns the key itself), or a shared check has no reason key. Look at `state(id, ctx).reason`.
  - **`is listed on N surfaces`**: a surface did not open (look at the screenshots in `D:/tmp/easydrag-w1b-shots`), or the action is not listed there; fix the list in `FLOW_MENU`, `EDIT_MENU`, `VIEW_MENU`, `MORE_MENU`, or the card and canvas menu lists. Do not lower `need`.
  - **`shortcut of X: ran 0 times, expected 1`**: the keystroke never reached `ED.actions.key` (a dialog or quick-add from the previous shortcut was still open: the test closes them with Escape and waits; add the missing selector to its list), or `where` gating refused it. **`ran 2 times`**: a double dispatch: the shell and `onKeyDown` both fired it; the shell's item must carry `shortcut` only for `flow.save`, `flow.test` and `view.search`.

- [ ] **Step 8.3: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_actions_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)TestEasyDragBrowserTestsUseTheSmokeHelper' -count=1 2>&1 | tail -2
```
Expected: no vet or gofmt output, and the guard `TestEasyDragBrowserTestsUseTheSmokeHelper` `ok` (the test opens its page through `startEasyDragSmoke`). Write `C:/disposable/easydrag-w1b/a8-message.txt`:
```
test(easydrag): browser test that every surface agrees on every action

TestEasyDragActionsAgreeBrowser opens the window menus, the more menu, the card
and canvas menus, the card tools, the header buttons and the shortcut table in
four states (normal, nothing selected, a run in progress, a past run open) and
checks the same label, displayed shortcut, availability and reason everywhere, a
reason on every unavailable entry, that each registry shortcut fires its action
once, and the start page's card menu.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && git add ui/desktop_easydrag_actions_browser_test.go && git commit -F C:/disposable/easydrag-w1b/a8-message.txt && git show --stat HEAD | tail -2
```
Expected: one commit, one file.

- [ ] **Step 8.4: the part's final run (before the coordinator lands it)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-actions && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' npm run test:easydrag:t0 2>&1 | tail -8
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1b-actions-full.txt | grep -c -- "^--- PASS"; grep -- "^--- FAIL" D:/tmp/easydrag-w1b-actions-full.txt; tail -2 D:/tmp/easydrag-w1b-actions-full.txt
wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-dialogs.js ui/js/desktop/apps/easydrag-canvas.js
```
Expected: T0 all jobs `ok`; the full browser run (about 17 minutes) shows the baseline `--- PASS` count plus one (the new top-level test; 120 PASS lines today, so 122 with its subtests none: count the top-level `--- PASS` lines, 60 today, 61 with this part), 2 SKIP, 0 FAIL, including the W1a lab with zero findings (the lab opens the editor in 16 contexts; an `aria-disabled` tool or header button must not break its target-size and contrast checks: if the lab reports a finding on a button this part touched, fix the CSS in `desktop-app-easydrag.css`, not the lab); the line counts: editor 852, interact 750, home 888, dialogs 281 (each shorter than its baseline 896, 753, 889, 309), canvas 925 (at most 928). Part C's budget test, once landed, counts the header's controls: this part adds none and removes none.

---

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §3.2 the table: rows, fields, the 11 groups of actions | A.1 (`TABLE` in `easydrag-actions.js`; four rows added, see Deviations) |
| §3.3 shared checks with one reason per cause; reason keys in 16 locales | A.1 (`CHECKS`, `REASONS`, key file `a-1`); `unlocked` and `quiet` are additions |
| §3.3 `edit.copy` has no `writable` check | A.1 (row `needs`; Node test), A.2 (card menu), A.3 (Edit menu), A.8 (all surfaces) |
| §3.4 `bind`, `state`, `run`, `items`, `button`, `row` | A.1 |
| §3.4 `key`: one keyboard path per shortcut, today's per-key rules kept | A.5 (the per-key table, the decision per shortcut, `key`, `TEXT_KEYS`, `PASS_WHEN_REFUSED`) |
| §3.4 display through `core.shortcut`, localized key names | A.1 (`display`, key `key_delete`), A.5 |
| §3.4 the shortcut table generated from the registry plus gestures | A.5 (`LAYOUT`, `table`) |
| §3.5 risk: `danger` for `edit.delete`, `edit.cut`, `flow.delete` | A.1 (rows, `items`), A.2 (card menu, canvas), A.3 ("more" menu), A.4 (home card menu); window menus see Decision 1 |
| §3.6 window menus, "more", Lebendigkeit through `items` | A.3 |
| §3.6 card menu, canvas menu, card tools (a tool selects its card, then runs the menu's action) | A.2 |
| §3.6 home card menu through the `flow.*` rows with the card's flow as context | A.4 |
| §3.6 header buttons take label, title, reason from `button`; markup stays | A.6 |
| §3.6 keyboard handlers route registry shortcuts through `key`; navigation stays in `handleKey` | A.5 |
| §3.6 one label per action; duplicate keys leave all 16 locales | A.2 to A.6 (call sites), A.7 (key file `a-7`) |
| §3.6 line rule (editor, interact, home, dialogs only shorter; canvas at most 928) | `wc -l` check in A.2 (canvas 925), A.3 (editor 852), A.4 (home 888), A.5 (interact 750, dialogs 281), A.6 (editor 852), A.8 (all five) |
| §3.7 T0 node: rows, reasons, `state`/`run` refuse, `items` danger | A.1 (`scripts/test-easydrag-actions.mjs`) |
| §3.7 T0 Go lint | A.7 (`TestEasyDragMenusUseTheActionRegistry`) |
| §3.7 browser test, four states, every surface, each shortcut fires once | A.8 (`TestEasyDragActionsAgreeBrowser`) |
| §8 "Delete and cut are red; deleting a flow is red and asks first" | A.1 (risk), A.2 to A.4; the existing confirmation dialogs stay; window menubar: Decision 1 |
| §8 "Copy works in read-only mode on every surface" | A.1 (Node test), A.8 |
| §9 risk "`aria-disabled` buttons still react" | A.6 (click handler, `run` re-check; Node tests) |

### Deviations from the spec

1. **Rows carry a desktop `icon` and a `glyph`, not an alias from the W1a icon map.** The map has 16 one-to-one meanings (and `palette` already owns `plus`), so it cannot name the card tools' glyphs (`plus`, `link`, `flask`, `eye-off`, `copy`, `trash`); the desktop menus draw desktop icon names (`save`, `check-square` …), not `core.ICONS` glyphs. `icon` keeps what the menus draw today, `glyph` is checked against `ED.core.ICONS` by the Node test. `ED.icons.MEANINGS` and its one-to-one test are untouched.
2. **A `menu` field on rows** (the window-menu item id: `run`, `delete-sel`, `select-all` …). The desktop uses it as the action key, and about twenty Node and browser tests find menu items by it. Without it every one of them would change for no gain.
3. **Rows beyond the spec's list** (spec §3.2 allows rows the plan finds): `flow.open_draft`, `flow.new`, `flow.import`, `flow.reload`. The start page's window menu (New, Import, Reload) is a menu today and builds `labelKey:` entries in `easydrag.js`; the block editor's "more" menu has "Open current draft". Not leaving an action outside the registry is the spec's own rule. The home hero buttons and the bulk bar stay out, as the spec says.
4. **Two checks beyond the spec's table:** `unlocked` (read-only and locked, but not the run view) and `quiet` (quick-add closed). `duplicateFlow` documents that a past run may be duplicated, deleting and exporting work there too, and Save and Test have always worked under quick-add while the edits wait for it. Without them the registry would have changed those behaviours.
5. **More helpers than the contract lists:** `paint`, `attrString`, `label`, `display`, `table`, `observe`, `TOOLS`, `REASONS`, `CHECKS`; `items(ids, ctx)` accepts `ctx.keyed`; a context may carry `cached`. The contract names `ROWS`, `bind`, `state`, `run`, `items`, `button`, `key`, `row` are exact, and `bind(id, impl)` takes the host as a third argument because several editor windows (and the start page) must not share bindings.
6. **Two reason texts are not the spec's:** `reason_locked` ("Locked by the agent permissions, or until the list refreshes." / "Durch die Agenten-Rechte gesperrt oder bis die Liste neu geladen ist.") because `createOff()` is also true for a stale list, FLOWS_DISABLED and lost authorisation; `reason_restoring` ("Answer the restore offer first.") because the editor's `restoring` flag is the emergency-copy offer, not a version restore. Voice rule 7.
7. **Task order:** interact before the editor's menus (the Edit and View menus need the `edit.*` bindings, which live in `interact`). `easydrag-interact.js` is 765 lines after A.2 (baseline 753) and 750 after A.5; the rule holds at the end of the part.
8. **Scope of the Edit shortcuts is `canvas`, as today**, not `editor` (the spec's table lists the shortcuts, its prose says "anywhere in the editor except text fields"). Today Ctrl+Z, Ctrl+C, Delete … act only while the canvas, a card or the page has the focus. See Decision 2.
9. **"⋯" has no row** (it opens a menu; it is not an action). Its label stays the key `more`. The header buttons that are actions (Test, Stop, Publish, Run now, Active, Undo, Redo) are registry-driven.
10. **`danger` red is drawn only by context menus** (card, canvas, "more", home card menu). The desktop's window menus (`normalizeWindowMenuItems`) do not support it; adding it is a change of the desktop core and a rebuild of `main.bundle.js`, which the spec does not ask for. `items()` sets `danger` on every entry; the window menu ignores it. Decision 1.
11. **Where today's surfaces disagreed, the stricter condition wins** (the spec's rule), except Copy. In effect:
    - Ctrl+X in read-only mode no longer copies (it copied and removed nothing; the Edit menu already refused Cut).
    - The Flow menu's Test is unavailable while a run is going (the "more" menu and the header already said so).
    - Find step is unavailable in a flow without steps on every surface (only the canvas menu said so; Ctrl+F opened an empty dialog).
    - Flow-menu items are unavailable, with a reason, while a dialog is open or the restore offer waits (they did nothing silently; Edit items already were).
    - Paste is unavailable when nothing can be pasted (no stored fragment and no clipboard API); in a browser the clipboard API exists, so it is available as before.
    - Run now says why it is off in read-only mode, in a past run and for a flow that was never published (it said nothing there); the paused text is `reason_paused` ("The flow is paused.") instead of `error_flow_disabled`. The run view's Run now tooltip no longer names the run's start time; the banner does.
    - Flow-menu "Versions" in a block editor opens the block's version list, as its "more" menu did.
    - A key that the registry takes and the action refuses is still prevented (Ctrl+D and Enter, D and C without a selection used to fall through; nothing visible happens either way).
12. **Labels unified** (one per action): Test reads "Entwurf testen" / "Test draft" everywhere (the Flow menu said "Testen"); a block's delete reads "Baustein löschen" on every surface; Paste reads "Einfügen" (the canvas menu said "Hier einfügen"); the shortcut table uses the menu labels ("Open", "Show everything", "Delete", "Keyboard shortcuts" for the old "Open the step", "Show the whole flow", "Delete the selection", "Show these shortcuts"); "+ / −" is two rows; Delete is drawn "Del" / "Entf". The menus show "Add a step" for Ctrl+K while one step is selected (the table did already).
13. **Tests changed, each for the cause named in its task:** `c1d04` dispose (11 bus subscriptions), `c1d04` key checks (harness gets `press`), `c1d10`, `c1d07` detail view, the shortcut-table test, `ff2` (three), `a1008` runs pause, `ux` Run now (five), `merge read-only desktop`, `a1023 F5` (two), the motion test, and the Go browser tests for the paused menu and the block header. No test was weakened: each one reads the surface the way the registry now writes it.

### Decisions needed

1. **Red for Delete and Cut in the window menubar.** The registry marks them `danger` and the context menus draw them red; the menubar does not. Recommendation: leave it; make it a small separate desktop change (`danger` in `normalizeWindowMenuItems` and `renderWindowMenuItems`, a CSS rule, a rebuild of `main.bundle.js`) once Parts A, B and C have landed.
2. **Should the Edit shortcuts work with the focus elsewhere in the editor** (on a rail or header button)? Today they do not, and the registry keeps that. Recommendation: keep it; revisit with E4's selection bar, which brings its own focus rules.
3. **Paste's availability check** is a heuristic (a stored fragment or a clipboard API). Recommendation: keep it (conservative: a browser always has the API). Alternative: no check, Paste always available.
4. **The wording of `reason_locked`** is longer than the spec's "Durch die Agenten-Rechte gesperrt." Recommendation: keep the accurate text.

### Could not verify here

- The Go lint and the browser test were written and checked for syntax (`gofmt`) but not run: the plan's Go package is only built in the worktree. The lint's logic was run as a Node port on the patched tree (zero violations, six menu calls; on main it reports 105). The first run of the browser test may need the small selector or wait fixes that Step 8.2 describes.
- The Node suite (`node scripts/test-easydrag.mjs`, `test-easydrag-actions.mjs`, the motion test) was run on a scratch copy with the patches applied; the numbers in the tasks come from that copy. T0's Go steps and the browser run were not run.
- The CSS rules for `[aria-disabled="true"]` were not looked at in a browser; the W1a lab (16 contexts, zero findings) checks target size and contrast on the header and the tools and is the gate in Step 8.4.
- GitNexus impact was not run (the index is of main; the commands and the expected callers are in each task).
