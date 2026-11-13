[← Plan index](README.md) · Spec: [plans/2026-11-13-easydrag-w1a-spec.md](../2026-11-13-easydrag-w1a-spec.md)

## Part D: Rail, icons and motion (UI-15, JY-01)

**Goal.** The editor rail grows from 48 to 64 px and shows a short caption under every icon. Every rail and header icon comes from one meaning-to-glyph map in which no glyph has two meanings ("Start" and "Gruppen" stop sharing `grid`). One motion gate, `ED.delight`, decides whether anything may move. "Lebendigkeit" (Normal or Ruhig) is one entry in the editor's "⋯" menu. No animation runs in an idle editor or home page, and under Ruhig nothing animates at all.

**Shell and worktree.** Git Bash. Every command starts with `cd <worktree> && …`. Work in a worktree made from main; never edit, stage or commit in the main checkout.

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1a-rail -b feat/easydrag-w1a-rail main
mkdir -p C:/disposable/easydrag-w1a/keys D:/tmp/go-build D:/tmp/easydrag-w1a-shots
```

Expected: `Preparing worktree (new branch 'feat/easydrag-w1a-rail')` and `HEAD is now at <sha>`. In this plan `WT` means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-rail`.

**What this part needs from the others.**
- Part A's `--ed-font-label` token. It may not have landed. The CSS below writes `var(--ed-font-label, 11px)`, so it works either way; Part B's font sweep then turns the fallback into the plain token (note for B: `desktop-app-easydrag.css`, rule `.ed-rail-caption`).
- Part A deletes the `.ed-run-dot--*` and `.ed-status--*` families, which hold two of the animations audited in D.4. D.4's gate is one rule that does not touch those lines, so A and D merge without conflict.
- The locale files are shared with A and C. Change them only through the key files below and `C:/disposable/easydrag-w1a/locale-apply.mjs`. After a rebase conflict in `ui/lang/easydrag/*.json`, take main's files and run the key files again.

**Facts the plan relies on (checked on main 9492395de on 2026-11-13).**
- The rail is a flex column; it does not change shape on narrow screens. The `.ed-rail { order: -2; }` rule (CSS around line 906) only places it left of the outline. The spec's idea that the rail "is a row at 390 px" is wrong. The rail stays a 64 px column at every width, which takes 16 % of a 390 px screen (D.2 checks 1440, 900 and 390 px).
- There are 7 rail buttons: `palette`, `home`, `templates`, `groups` (`easydrag-editor.js:72-75`), then `outline` and `focus` (inserted before the spacer by `easydrag-workspace.js:83-87`), then after the spacer `keys` (`easydrag-editor.js:77`).
- `core.icon` draws from `ICONS` in `easydrag-core.js:8-87` (78 glyphs, 78 different drawings, 24×24 view box, stroke 1.8).
- The shared fixture shell (`ui/testdata/aurora-fixture.js:67`) sets `'windows.animations': false`, so `document.body.dataset.animations` is `'false'` in every EasyDrag browser test. The new gate therefore reads `calm` there, whatever the `Motion` option says. D.4's test sets `data-animations` to `true` itself.
- `document.getAnimations()` also lists finished animations whose `fill-mode` is `both`/`forwards` (the `ed-rise` and `ed-pop` rules use `both`). The test counts only animations whose `playState` is `running`.
- A literal-key test (`TestEasyDragUIKeysExistInAllLocales`, `ui/desktop_easydrag_test.go:189`) rejects a key that is built as `'easydrag.ui.rail_' + id` unless the test lists the family. This part therefore writes every rail key as a full literal in `easydrag-icons.js`, so `desktop_easydrag_test.go` needs no edit except the module list.

**Baseline, before any change.**

```
cd WT && npm run test:easydrag:t0
```

Expected: every job `ok`, the last lines `T0 passed`. Record the total seconds.

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-shots/d-baseline.txt | grep -c -- '--- PASS'
grep -c -- '--- SKIP' D:/tmp/easydrag-w1a-shots/d-baseline.txt
```

Run this with the Bash sandbox disabled (Chrome cannot start from a test binary under the repo; see the memory note `reference_chrome_launch_from_agent.md`). Expected: 41 PASS and 2 SKIP, in about 5 to 6 minutes. If the counts differ, write the real counts into the final report and use them as this part's baseline. T0 uses only Node built-ins; if the first run reports a missing module, copy `node_modules` from the main checkout (memory note `reference_worktree_node_modules.md`).

**Shared helper for line edits.** Several tasks change existing lines of `easydrag-editor.js` (896 lines, a ratchet: existing lines only), `easydrag-workspace.js` and `easydrag.js`. The helper below applies a list of `[file, line, old, new]` edits. It fails if a line does not contain `old` exactly once, so a drifted line number stops the work instead of damaging a file. Create it once (Task D.1, Step 1).

---

### Task D.1: The icon map, `easydrag-icons.js`

**Files:**
- Create: `ui/js/desktop/apps/easydrag-icons.js`
- Create: `scripts/test-easydrag-icons.mjs`
- Create: `C:/disposable/easydrag-w1a/d-line-edits.mjs` (helper, not committed)
- Create: `C:/disposable/easydrag-w1a/d1-edits.mjs` (not committed)
- Modify: `ui/js/desktop/apps/easydrag-core.js:76-77` (one new glyph line)
- Modify: `ui/js/desktop/apps/easydrag-editor.js:53,57-58,60-62,64,66-70,72-75,77` (icon calls only)
- Modify: `ui/js/desktop/apps/easydrag-workspace.js:83-84` (icon calls only)
- Modify: `ui/js/desktop/core/module-loader.js:285` (after core)
- Modify: `ui/desktop_easydrag_test.go:12` (module list)
- Modify: `scripts/test-easydrag-extra3.mjs:9` (harness module list; others only if the T0 run names them)
- Test: `node scripts/test-easydrag-icons.mjs`

**The meaning table (decision for every glyph that the rail and the editor header use today).**

| Meaning | Glyph | Used by today | Change |
|---|---|---|---|
| `palette` | `plus` | rail "Schritte" (`editor.js:72`) | none |
| `home` | `home` | rail "Start" (`editor.js:73`, was `grid`) | `grid` to `home` |
| `templates` | `sparkles` | rail "Vorlagen" (`:74`) | none |
| `groups` | `group-frame` (new) | rail "Gruppen" (`:75`, was `grid`) | `grid` to the new glyph |
| `outline` | `list` | rail "Liste" (`workspace.js:83`) | none |
| `focus` | `fit` | rail "Fokus" (`workspace.js:84`) | none |
| `keys` | `keyboard` | rail "Tasten" (`editor.js:77`) | none |
| `back` | `arrow-left` | header back button (`:53`), run-view exit (`:70`) | none |
| `runs` | `history` | header Runs (`:57`), run-view banner (`:66`) | none |
| `test` | `flask` | header Test (`:58`), re-run in the banner (`:69`) | none |
| `publish` | `rocket` | header Publish (`:61`) | none |
| `run` | `play` | header Run now (`:62`) | none |
| `stop` | `player-stop` | header Cancel (`:60`), banner stop (`:68`) | none |
| `approve` | `check` | banner "approval review" (`:67`) | none |
| `more` | `dots` | header "⋯" (`:64`) | none |
| `settings` | `settings` | not on the rail or header today; listed because the spec names it | none |

The glyph `stop` (a filled square) stays unmapped: `player-stop` is what the cancel buttons use, and two near-identical squares would be two glyphs for one meaning. The footer's issue icons (`alert`, `info`, `check` in `easydrag-header.js:193-195`) are status marks and belong to Part A's disc; they are not in this map.

**The groups glyph (decision).** "Start" gets the existing `home` glyph, which no rail button uses. "Gruppen" gets one new glyph, `group-frame`: a dashed rounded frame with a small box inside. It is the owner-approved drawing from the rail mock (`gen_rail.py`, option A), and it is drawn like the group frame that the canvas shows around grouped steps. No existing unused glyph fits: `folder` means a folder of flows on the start page, `grid` means the block library (`easydrag-home.js:125`) and the overview, and `pin`, `star`, `filter`, `checkbox`, `drag` mean something else. The glyph is added to `ICONS` in the same style (24×24, stroke only, `1.5×` the mock's 16×16 coordinates). Part E's lab scene can still show `group-frame` next to `folder` before the owner signs off; the decision costs one line to reverse.

- [ ] **Step 1: Create the line-edit helper**

Write `C:/disposable/easydrag-w1a/d-line-edits.mjs` with the Write tool:

```js
#!/usr/bin/env node
// Applies [file, line, old, new] edits to files under ui/js/desktop/apps/ (run from the worktree root):
//   node C:/disposable/easydrag-w1a/d-line-edits.mjs <edits.mjs> [--check]
// Each edit must find `old` exactly once on that line, or the whole run stops before writing anything.
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const spec = process.argv[2];
if (!spec) throw new Error('usage: d-line-edits.mjs <edits.mjs> [--check]');
const edits = (await import(pathToFileURL(path.resolve(spec)).href)).default;
const apps = path.resolve('ui/js/desktop/apps');
const files = new Map();
for (const [file, line, from, to] of edits) {
    if (!files.has(file)) files.set(file, fs.readFileSync(path.join(apps, file), 'utf8').split('\n'));
    const lines = files.get(file);
    const text = lines[line - 1];
    if (text === undefined) throw new Error(file + ':' + line + ' does not exist');
    const parts = text.split(from);
    if (parts.length !== 2) throw new Error(file + ':' + line + ' must contain exactly once: ' + from + ' (found ' + (parts.length - 1) + ')');
    lines[line - 1] = parts.join(to);
}
if (process.argv.includes('--check')) { console.log('check ok: ' + edits.length + ' edits'); process.exit(0); }
for (const [file, lines] of files) fs.writeFileSync(path.join(apps, file), lines.join('\n'));
console.log('applied ' + edits.length + ' edits in ' + files.size + ' files');
```

- [ ] **Step 2: Write the failing test**

Create `scripts/test-easydrag-icons.mjs`:

```js
#!/usr/bin/env node
// W1a D.1: one meaning, one glyph. The icon map covers every rail and header icon, every glyph it names
// exists in ICONS, no glyph has two meanings, and the rail and the editor header read their icons
// through ED.icons.of (no direct core.icon call is left in editor.js or workspace.js).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' },
    localStorage: { length: 0, key: () => null, getItem: () => null, setItem() {}, removeItem() {} },
    setTimeout: () => 1, clearTimeout: () => {}
});
for (const file of ['easydrag-core.js', 'easydrag-icons.js']) vm.runInContext(read(file), box, { filename: file });
const { core, icons } = box.window.EasyDrag;

// Every glyph that a meaning names exists, and every meaning has its own glyph.
const entries = Object.entries(icons.MEANINGS);
assert.ok(entries.length >= 15, 'the map covers the rail and the header');
for (const [meaning, glyph] of entries) assert.ok(Object.prototype.hasOwnProperty.call(core.ICONS, glyph), meaning + ' names a glyph that ICONS does not have: ' + glyph);
const owner = new Map();
for (const [meaning, glyph] of entries) {
    assert.ok(!owner.has(glyph), 'glyph ' + glyph + ' has two meanings: ' + owner.get(glyph) + ' and ' + meaning);
    owner.set(glyph, meaning);
}
// Two glyph names must not hide one drawing either.
const drawings = new Map();
for (const [name, body] of Object.entries(core.ICONS)) {
    assert.ok(!drawings.has(body), 'ICONS draws ' + name + ' and ' + drawings.get(body) + ' alike');
    drawings.set(body, name);
}

// Start and Gruppen no longer share the grid glyph.
assert.notEqual(icons.MEANINGS.home, icons.MEANINGS.groups);
assert.equal(icons.MEANINGS.home, 'home');
assert.equal(icons.MEANINGS.groups, 'group-frame');
assert.notEqual(icons.of('home'), icons.of('groups'));

// of() draws through core.icon, passes a class on, and falls back to core's fallback glyph for an unknown meaning.
assert.equal(icons.of('more'), core.icon('dots'));
assert.ok(icons.of('run', 'x-extra').includes('class="ed-icon x-extra"'));
assert.equal(icons.of('no-such-meaning'), core.icon('tool'));

// The sources read through the map: every id is a known meaning, and the two rail files keep no direct core.icon call.
let calls = 0;
for (const file of fs.readdirSync(apps).filter(name => /^easydrag.*\.js$/.test(name))) {
    for (const match of read(file).matchAll(/ED\.icons\.of\(\s*'([^']+)'/g)) {
        calls++;
        assert.ok(Object.prototype.hasOwnProperty.call(icons.MEANINGS, match[1]), file + ': unknown icon meaning ' + match[1]);
    }
}
assert.ok(calls >= 17, 'editor.js and workspace.js call ED.icons.of (found ' + calls + ')');
for (const file of ['easydrag-editor.js', 'easydrag-workspace.js']) {
    assert.ok(!/\bcore\.icon\(/.test(read(file)), file + ' still calls core.icon directly');
}
console.log('PASS icon map: ' + entries.length + ' meanings, one glyph each, ' + calls + ' call sites');
```

- [ ] **Step 3: Run it and watch it fail**

```
cd WT && node scripts/test-easydrag-icons.mjs
```

Expected: it fails with `ENOENT … easydrag-icons.js` (the module does not exist yet).

- [ ] **Step 4: IMPACT before the edits**

The edits change markup strings inside `create` of `easydrag-editor.js` and `easydrag-workspace.js`, not their logic. Run:

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "create" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Several `create` functions exist, so the answer may be ambiguous or UNKNOWN. Treat UNKNOWN as unresolved and confirm the callers with `grep -rn "ED.editor.create\|ED.workspace.create" ui/js/desktop/apps`. Expected callers: `easydrag.js` (`showFlow`) for the editor and `easydrag-editor.js` for the workspace. Warn on HIGH or CRITICAL in the report, and continue: the change is an icon swap.

- [ ] **Step 5: Add the glyph and write the module**

In `ui/js/desktop/apps/easydrag-core.js`, add the new glyph directly after the `grid` entry (line 76). The old and new text of the line after it must stay unchanged; insert one line:

```
        'group-frame': '<rect x="3.75" y="5.25" width="16.5" height="13.5" rx="3" stroke-dasharray="3.3 2.4"/><rect x="7.5" y="9" width="4.5" height="3.75" rx=".9"/>',
```

Verify: `grep -n "'group-frame'" ui/js/desktop/apps/easydrag-core.js` prints one line, between `'grid'` and `'external'`.

Create `ui/js/desktop/apps/easydrag-icons.js`:

```js
// EasyDrag icon map: one meaning, one glyph. The rail and the editor header draw their icons here, so a
// glyph never carries two meanings (the start page and the groups used to share "grid"). The glyph
// drawings stay in easydrag-core.js ICONS; this module only says which meaning uses which glyph and
// owns the short captions under the rail icons.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const core = ED.core;

    // MEANINGS maps a meaning to the name of one glyph of core.ICONS. Keep it one-to-one: the Node test
    // fails when two meanings name the same glyph.
    const MEANINGS = Object.freeze({
        palette: 'plus',
        home: 'home',
        templates: 'sparkles',
        groups: 'group-frame',
        outline: 'list',
        focus: 'fit',
        keys: 'keyboard',
        back: 'arrow-left',
        runs: 'history',
        test: 'flask',
        publish: 'rocket',
        run: 'play',
        stop: 'player-stop',
        approve: 'check',
        more: 'dots',
        settings: 'settings'
    });

    // RAIL maps each rail button to the locale key of its caption. The keys are literals on purpose: the
    // locale tests find literal keys, and a key built from a prefix would need its own family list.
    const RAIL = Object.freeze({
        palette: 'easydrag.ui.rail_palette',
        home: 'easydrag.ui.rail_home',
        templates: 'easydrag.ui.rail_templates',
        groups: 'easydrag.ui.rail_groups',
        outline: 'easydrag.ui.rail_outline',
        focus: 'easydrag.ui.rail_focus',
        keys: 'easydrag.ui.rail_keys'
    });

    // of returns the icon markup of a meaning. An unknown meaning draws core's fallback glyph, like core.icon.
    function of(meaning, cls) {
        const glyph = Object.prototype.hasOwnProperty.call(MEANINGS, meaning) ? MEANINGS[meaning] : '';
        return core.icon(glyph, cls);
    }

    // caption returns the short text under a rail icon. The button keeps its long name in aria-label and
    // title, so the caption is hidden from screen readers (it would read the name twice).
    function caption(t, id) {
        return '<span class="ed-rail-caption" aria-hidden="true">' + core.esc(t(RAIL[id])) + '</span>';
    }

    ED.icons = { MEANINGS, RAIL, of, caption };
})();
```

`core.icon('')` finds no glyph and draws `ICONS.tool`, which is what the test expects for an unknown meaning.

- [ ] **Step 6: Register the module in the three places**

1. `ui/js/desktop/core/module-loader.js`, directly after the line `'/js/desktop/apps/easydrag-core.js',` (line 285):

```
                '/js/desktop/apps/easydrag-icons.js',
```

2. `ui/desktop_easydrag_test.go:12`: in `easyDragModules`, change `"easydrag-core.js", "easydrag-durable.js",` to `"easydrag-core.js", "easydrag-icons.js", "easydrag-durable.js",`.

3. `scripts/test-easydrag-extra3.mjs:9`: it loads the editor modules in a vm. Change `const MODULES = ['easydrag-core.js', 'easydrag-durable.js',` to `const MODULES = ['easydrag-core.js', 'easydrag-icons.js', 'easydrag-durable.js',`. Other harness lists (`extra2`, `extra4`) do not load the editor or workspace; if the T0 run in Step 8 fails with `ED.icons` undefined in another script, add `'easydrag-icons.js'` after `'easydrag-core.js'` in that script's list in the same way.

Check that the bundle step is not involved: `grep -n "easydrag" scripts/build-ui-bundles.js` prints nothing, so no `npm run build:ui` and no `node_modules` copy is needed for this part.

- [ ] **Step 7: Move the rail and header icons onto the map**

Create `C:/disposable/easydrag-w1a/d1-edits.mjs`:

```js
// Rail and header icon calls move onto ED.icons.of. Lines are those of main 9492395de.
export default [
    ['easydrag-editor.js', 53, "core.icon('arrow-left')", "ED.icons.of('back')"],
    ['easydrag-editor.js', 57, "core.icon('history')", "ED.icons.of('runs')"],
    ['easydrag-editor.js', 58, "core.icon('flask')", "ED.icons.of('test')"],
    ['easydrag-editor.js', 60, "core.icon('player-stop')", "ED.icons.of('stop')"],
    ['easydrag-editor.js', 61, "core.icon('rocket')", "ED.icons.of('publish')"],
    ['easydrag-editor.js', 62, "core.icon('play')", "ED.icons.of('run')"],
    ['easydrag-editor.js', 64, "core.icon('dots')", "ED.icons.of('more')"],
    ['easydrag-editor.js', 66, "core.icon('history')", "ED.icons.of('runs')"],
    ['easydrag-editor.js', 67, "core.icon('check')", "ED.icons.of('approve')"],
    ['easydrag-editor.js', 68, "core.icon('player-stop')", "ED.icons.of('stop')"],
    ['easydrag-editor.js', 69, "core.icon('flask')", "ED.icons.of('test')"],
    ['easydrag-editor.js', 70, "core.icon('arrow-left')", "ED.icons.of('back')"],
    ['easydrag-editor.js', 72, "core.icon('plus')", "ED.icons.of('palette')"],
    ['easydrag-editor.js', 73, "core.icon('grid')", "ED.icons.of('home')"],
    ['easydrag-editor.js', 74, "core.icon('sparkles')", "ED.icons.of('templates')"],
    ['easydrag-editor.js', 75, "core.icon('grid')", "ED.icons.of('groups')"],
    ['easydrag-editor.js', 77, "core.icon('keyboard')", "ED.icons.of('keys')"],
    ['easydrag-workspace.js', 83, "core.icon('list')", "ED.icons.of('outline')"],
    ['easydrag-workspace.js', 84, "core.icon('fit')", "ED.icons.of('focus')"]
];
```

Run it. If the line numbers moved on main, the helper stops with the file, the line and the missing text; fix the numbers (never the old or new text) from `grep -n "core.icon(" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js` and run again.

```
cd WT && node C:/disposable/easydrag-w1a/d-line-edits.mjs C:/disposable/easydrag-w1a/d1-edits.mjs
```

Expected: `applied 19 edits in 2 files`.

Proof that no old form remains and the file lengths are unchanged:

```
cd WT && grep -c "core.icon(" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js; wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js
```

Expected: both counts `0` (grep exits 1, that is fine), and `896` and `126` lines.

- [ ] **Step 8: Run the tests**

```
cd WT && node scripts/test-easydrag-icons.mjs
```

Expected: `PASS icon map: 16 meanings, one glyph each, 19 call sites`.

```
cd WT && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && node scripts/test-desktop-icon-resolver.mjs && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
```

Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, the icon-resolver line, and `ok  aurago/ui`. The resolver test reads `core.icon('…')` calls only, so it does not see `ED.icons.of`; the new Node test covers those.

- [ ] **Step 9: Commit**

```
cd WT && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_test.go
```

Expected: no output from either command. Write the message to `C:/disposable/easydrag-w1a/d1-commit.txt` with the Write tool:

```
feat(easydrag): one icon map, one glyph per meaning

The rail and the editor header now draw their icons through ED.icons.of.
The map has 16 meanings and no glyph carries two: Start uses the existing
home glyph, Gruppen gets the new group-frame glyph (a dashed frame, the
shape the canvas draws around grouped steps), so they no longer share grid.
A Node test checks the one-to-one map and that editor.js and workspace.js
keep no direct core.icon call.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd WT && git add ui/js/desktop/apps/easydrag-icons.js ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-icons.mjs scripts/test-easydrag-extra3.mjs && git commit -F C:/disposable/easydrag-w1a/d1-commit.txt && git log --oneline -1
```

Expected: one commit; if Step 6 touched another harness script, add it to `git add`.

---

### Task D.2: The rail with captions

**Files:**
- Create: `C:/disposable/easydrag-w1a/keys/d-2-rail.json` (key file, not committed)
- Create: `C:/disposable/easydrag-w1a/d2-edits.mjs` (not committed)
- Create: `scripts/test-easydrag-rail.mjs`
- Create: `ui/desktop_easydrag_rail_browser_test.go`
- Modify: `ui/lang/easydrag/*.json` (16 files, through the helper)
- Modify: `ui/js/desktop/apps/easydrag-editor.js:72-75,77` (existing lines only)
- Modify: `ui/js/desktop/apps/easydrag-workspace.js:83-84`
- Modify: `ui/css/desktop-app-easydrag.css:573,575,577` (rail rules)
- Test: `node scripts/test-easydrag-rail.mjs`, `go test ./ui -run TestEasyDragRailCaptionsBrowser`

**Captions (the full rail button list).** Every caption is at most 10 characters in every locale, so it fits the 56 px button at 11 px; the CSS still ellipsizes a longer text, and the button keeps its full name in `title` and `aria-label`.

| Button (cmd or file) | Position | Long name (tooltip, unchanged) | Caption de | Caption en |
|---|---|---|---|---|
| `palette` | top | "Bausteine (Strg+K)" | Schritte | Steps |
| `home` | top | "Alle Flows" | Start | Home |
| `templates` | top | "Vorlagen" | Vorlagen | Templates |
| `groups` | top | "Visuelle Gruppen" | Gruppen | Groups |
| `outline` (workspace) | top | "Schritte dieses Flows" | Liste | List |
| `focus` (workspace) | top | "Ausgewählten Pfad fokussieren" | Fokus | Focus |
| `keys` | after the spacer | "Tastenkürzel (?)" | Tasten | Keys |

- [ ] **Step 1: Write the failing Node test**

Create `scripts/test-easydrag-rail.mjs`:

```js
#!/usr/bin/env node
// W1a D.2: every rail button has a caption key (in all 16 locales, short) and an icon-map entry.
// The buttons are found in the sources, so a new rail button without a caption fails here.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' },
    localStorage: { length: 0, key: () => null, getItem: () => null, setItem() {}, removeItem() {} },
    setTimeout: () => 1, clearTimeout: () => {}
});
for (const file of ['easydrag-core.js', 'easydrag-icons.js']) vm.runInContext(read(file), box, { filename: file });
const { icons } = box.window.EasyDrag;

// The rail buttons the sources build: ED.icons.caption(<t>, '<id>') in editor.js and workspace.js.
const ids = [];
for (const file of ['easydrag-editor.js', 'easydrag-workspace.js']) {
    for (const match of read(file).matchAll(/ED\.icons\.caption\(\s*[A-Za-z.]+,\s*'([a-z-]+)'\s*\)/g)) ids.push(match[1]);
}
assert.deepEqual([...ids].sort(), Object.keys(icons.RAIL).sort(), 'the rail buttons in the sources are exactly the ids of ED.icons.RAIL');
assert.equal(new Set(ids).size, ids.length, 'no rail button is built twice');
assert.equal(ids.length, 7);

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
for (const id of ids) {
    assert.ok(Object.prototype.hasOwnProperty.call(icons.MEANINGS, id), id + ' has no icon-map entry');
    assert.equal(icons.RAIL[id], 'easydrag.ui.rail_' + id);
    for (const lang of LOCALES) {
        const words = JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
        const text = words[icons.RAIL[id]];
        assert.equal(typeof text, 'string', lang + ' lacks ' + icons.RAIL[id]);
        assert.ok(text.trim().length > 0, lang + ' has an empty ' + icons.RAIL[id]);
        assert.ok(Array.from(text).length <= 10, lang + ' ' + icons.RAIL[id] + ' is too long for the 56 px button: ' + text);
    }
}

// caption() escapes the text and hides it from screen readers (the button keeps its aria-label).
const html = icons.caption(key => (key === 'easydrag.ui.rail_home' ? '<b>"x"</b>' : key), 'home');
assert.equal(html, '<span class="ed-rail-caption" aria-hidden="true">&lt;b&gt;&quot;x&quot;&lt;/b&gt;</span>');
console.log('PASS rail captions: ' + ids.length + ' buttons, ' + LOCALES.length + ' locales');
```

Run it:

```
cd WT && node scripts/test-easydrag-rail.mjs
```

Expected: it fails with `AssertionError … the rail buttons in the sources are exactly the ids of ED.icons.RAIL` (the sources call no `ED.icons.caption` yet; `ids` is empty).

- [ ] **Step 2: Write the key file and apply it**

Create `C:/disposable/easydrag-w1a/keys/d-2-rail.json` with the Write tool. All 16 locales for all 7 keys; the new keys go after `easydrag.ui.rail_label` in the order shown.

```json
{
  "easydrag.ui.rail_palette": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Kroky", "da": "Trin", "de": "Schritte", "el": "Βήματα", "en": "Steps", "es": "Pasos", "fr": "Étapes", "hi": "चरण",
    "it": "Passi", "ja": "手順", "nl": "Stappen", "no": "Trinn", "pl": "Kroki", "pt": "Passos", "sv": "Steg", "zh": "步骤"
  },
  "easydrag.ui.rail_home": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Domů", "da": "Start", "de": "Start", "el": "Αρχική", "en": "Home", "es": "Inicio", "fr": "Accueil", "hi": "होम",
    "it": "Home", "ja": "ホーム", "nl": "Start", "no": "Start", "pl": "Start", "pt": "Início", "sv": "Start", "zh": "首页"
  },
  "easydrag.ui.rail_templates": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Šablony", "da": "Skabelon", "de": "Vorlagen", "el": "Πρότυπα", "en": "Templates", "es": "Modelos", "fr": "Modèles", "hi": "टेम्पलेट",
    "it": "Modelli", "ja": "雛形", "nl": "Sjablonen", "no": "Maler", "pl": "Szablony", "pt": "Modelos", "sv": "Mallar", "zh": "模板"
  },
  "easydrag.ui.rail_groups": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Skupiny", "da": "Grupper", "de": "Gruppen", "el": "Ομάδες", "en": "Groups", "es": "Grupos", "fr": "Groupes", "hi": "समूह",
    "it": "Gruppi", "ja": "グループ", "nl": "Groepen", "no": "Grupper", "pl": "Grupy", "pt": "Grupos", "sv": "Grupper", "zh": "分组"
  },
  "easydrag.ui.rail_outline": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Seznam", "da": "Liste", "de": "Liste", "el": "Λίστα", "en": "List", "es": "Lista", "fr": "Liste", "hi": "सूची",
    "it": "Elenco", "ja": "リスト", "nl": "Lijst", "no": "Liste", "pl": "Lista", "pt": "Lista", "sv": "Lista", "zh": "列表"
  },
  "easydrag.ui.rail_focus": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Fokus", "da": "Fokus", "de": "Fokus", "el": "Εστίαση", "en": "Focus", "es": "Foco", "fr": "Focus", "hi": "फ़ोकस",
    "it": "Focus", "ja": "フォーカス", "nl": "Focus", "no": "Fokus", "pl": "Fokus", "pt": "Foco", "sv": "Fokus", "zh": "聚焦"
  },
  "easydrag.ui.rail_keys": {
    "_after": "easydrag.ui.rail_label",
    "cs": "Klávesy", "da": "Taster", "de": "Tasten", "el": "Πλήκτρα", "en": "Keys", "es": "Teclas", "fr": "Touches", "hi": "कुंजियाँ",
    "it": "Tasti", "ja": "キー", "nl": "Toetsen", "no": "Taster", "pl": "Klawisze", "pt": "Teclas", "sv": "Tangenter", "zh": "快捷键"
  }
}
```

```
cd WT && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/d-2-rail.json && grep -c '"easydrag.ui.rail_' ui/lang/easydrag/de.json ui/lang/easydrag/zh.json
```

Expected: the helper reports the files written; each grep count is `8` (`rail_label` plus the 7 new keys). Run it twice to see it is idempotent (`git diff --stat` does not change on the second run).

- [ ] **Step 3: Add the captions to the rail buttons**

Create `C:/disposable/easydrag-w1a/d2-edits.mjs`:

```js
// Each rail button gets its caption span after the icon. Lines are those after Task D.1.
export default [
    ['easydrag-editor.js', 72, "ED.icons.of('palette') + '</button>'", "ED.icons.of('palette') + ED.icons.caption(t, 'palette') + '</button>'"],
    ['easydrag-editor.js', 73, "ED.icons.of('home') + '</button>'", "ED.icons.of('home') + ED.icons.caption(t, 'home') + '</button>'"],
    ['easydrag-editor.js', 74, "ED.icons.of('templates') + '</button>'", "ED.icons.of('templates') + ED.icons.caption(t, 'templates') + '</button>'"],
    ['easydrag-editor.js', 75, "ED.icons.of('groups') + '</button>'", "ED.icons.of('groups') + ED.icons.caption(t, 'groups') + '</button>'"],
    ['easydrag-editor.js', 77, "ED.icons.of('keys') + '</button>'", "ED.icons.of('keys') + ED.icons.caption(t, 'keys') + '</button>'"],
    ['easydrag-workspace.js', 83, "ED.icons.of('outline') + '</button>'", "ED.icons.of('outline') + ED.icons.caption(ed.t, 'outline') + '</button>'"],
    ['easydrag-workspace.js', 84, "ED.icons.of('focus') + '</button>'", "ED.icons.of('focus') + ED.icons.caption(ed.t, 'focus') + '</button>'"]
];
```

```
cd WT && node C:/disposable/easydrag-w1a/d-line-edits.mjs C:/disposable/easydrag-w1a/d2-edits.mjs && grep -c "ED.icons.caption(" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js && wc -l ui/js/desktop/apps/easydrag-editor.js
```

Expected: `applied 7 edits in 2 files`, counts `5` (the five editor lines hold one call each) and `2`, and `896 ui/js/desktop/apps/easydrag-editor.js`. `grep -c` counts lines, so the editor count is 5.

- [ ] **Step 4: Run the Node tests**

```
cd WT && node scripts/test-easydrag-rail.mjs && node scripts/test-easydrag-icons.mjs && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragUIKeysExistInAllLocales' -count=1 2>&1 | tail -2
```

Expected: `PASS rail captions: 7 buttons, 16 locales`, the D.1 PASS line, and `ok  aurago/ui`. If the locale test names `easydrag.ui.rail_*` as unused, the literal table in `easydrag-icons.js` is not scanned: check that `easydrag-icons.js` is in `easyDragModules` (Task D.1, Step 6).

- [ ] **Step 5: The rail CSS**

Three existing rules change and one is added, all in `ui/css/desktop-app-easydrag.css`. Apply each edit with the Edit tool and the exact strings below.

1. Line 573, in `.ed-rail { … }`: replace `width: 48px; flex: none; padding: 8px 0;` with `width: 64px; flex: none; padding: 8px 0; overflow-x: hidden; overflow-y: auto; scrollbar-width: none;`. The rail stays a column at every width. A short window scrolls the rail instead of clipping a button.

2. Line 575: replace

```
.ed-rail .ed-icon-btn { width: 34px; height: 34px; border-radius: var(--vd-mat-btn-radius, 10px); }
```

with

```
.ed-rail .ed-icon-btn { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; flex: none; width: 56px; height: auto; min-height: 48px; padding: 6px 2px; border-radius: var(--vd-mat-btn-radius, 10px); }
.ed-rail .ed-icon { width: 18px; height: 18px; }
.ed-rail-caption { max-width: 100%; font-size: var(--ed-font-label, 11px); line-height: 1.1; color: inherit; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
```

The caption inherits the button colour, so the pressed state (`--ed-selected-text`), the disabled state and the forced-colors rule (`Highlight`/`HighlightText`, CSS around line 1537) apply to it without further rules. `--ed-font-label` is Part A's token; the `11px` fallback keeps the rule valid before A has landed.

Proof of the three changes:

```
cd WT && grep -n "width: 64px; flex: none\|\.ed-rail \.ed-icon-btn { display: flex\|^\.ed-rail-caption" ui/css/desktop-app-easydrag.css && grep -c "width: 48px; flex: none\|width: 34px; height: 34px; border-radius: var(--vd-mat-btn-radius" ui/css/desktop-app-easydrag.css
```

Expected: three matching lines, then `0` (the old forms are gone).

- [ ] **Step 6: Write the browser test (T1)**

Create `ui/desktop_easydrag_rail_browser_test.go`. It opens the seeded flow and measures every rail button at 1440, 900 and 390 px. The screenshot goes to the artifact directory. Failures name the viewport, the button and the measured numbers.

```go
package ui

import (
	"testing"
)

// TestEasyDragRailCaptionsBrowser checks the labelled rail: seven buttons, 64 px wide, each with an
// unclipped caption under its icon that stays inside its button, at 1440, 900 and 390 px (the width
// where the editor is a phone layout). The rail is a column at every width, so nothing may scroll.
func TestEasyDragRailCaptionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-rail button').length===7`)

	measure := `()=>{
		const rail=document.querySelector('.ed-rail'), rr=rail.getBoundingClientRect(), out=[];
		const buttons=[...rail.querySelectorAll('button')];
		buttons.forEach(b=>{
			const id=b.dataset.edCmd||b.getAttribute('aria-label');
			const cap=b.querySelector('.ed-rail-caption'), icon=b.querySelector('.ed-icon');
			if(!cap||!cap.textContent.trim()){out.push(id+': no caption');return;}
			if(getComputedStyle(cap).display==='none')out.push(id+': caption hidden');
			const r=cap.getBoundingClientRect(), br=b.getBoundingClientRect(), ir=icon.getBoundingClientRect();
			if(cap.scrollWidth>cap.clientWidth)out.push(id+': caption clipped, scrollWidth '+cap.scrollWidth+' > clientWidth '+cap.clientWidth);
			if(r.left<br.left-0.5||r.right>br.right+0.5||r.bottom>br.bottom+0.5)out.push(id+': caption leaves its button');
			if(r.top<ir.bottom-0.5)out.push(id+': caption overlaps its icon');
			if(br.left<rr.left-0.5||br.right>rr.right+0.5)out.push(id+': button leaves the rail');
			if(br.height<44)out.push(id+': button is only '+Math.round(br.height)+' px tall');
			if(parseFloat(getComputedStyle(cap).fontSize)<10.9)out.push(id+': caption font is '+getComputedStyle(cap).fontSize);
		});
		if(rail.scrollHeight>rail.clientHeight+1)out.push('the rail scrolls: '+rail.scrollHeight+' > '+rail.clientHeight);
		return JSON.stringify({n:buttons.length,width:Math.round(rr.width),problems:out});
	}`

	for _, vp := range []struct {
		name          string
		width, height int
		mobile        bool
	}{{"1440", 1440, 900, false}, {"900", 900, 700, false}, {"390", 390, 844, true}} {
		p.MustSetViewport(vp.width, vp.height, 1, vp.mobile)
		p.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,250))))`)
		got := p.MustEval(measure).Str()
		if got != `{"n":7,"width":64,"problems":[]}` {
			t.Fatalf("rail at %s px: %s", vp.name, got)
		}
		s.shot("rail-captions-" + vp.name)
	}
	s.noRawKeys("rail")
	s.failOnPageErrors("rail")
}
```

Run it and see it pass (the work of Steps 3 and 5 is already in place). To see it fail for the right reason first, run it once with the CSS from Step 5 stashed: `git stash push ui/css/desktop-app-easydrag.css`, run, then `git stash pop`. Expected failure: `rail at 1440 px: {"n":7,"width":48,"problems":[…]}`.

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run 'TestEasyDragRailCaptionsBrowser' -count=1 -timeout 10m -v 2>&1 | tail -12
```

Run with the Bash sandbox disabled. Expected: `--- PASS: TestEasyDragRailCaptionsBrowser` and three `wrote …rail-captions-<width>.png` lines. Open the 1440 and 390 screenshots with the Read tool and look at them: the captions sit under the icons, the pressed "Schritte" button is highlighted, and nothing touches the canvas edge.

What each outcome means:
- `caption clipped` for a button: a caption is wider than 56 px in German. Shorten the de text in the key file (and re-run the helper), do not widen the button.
- `the rail scrolls` at 900×700: the editor window is shorter than 7 buttons. Check the window height first; if it is real, reduce `.ed-rail { gap }` from 4px to 2px, not the button height (touch targets stay 44 px or more).
- `width` is not 64: the `box-sizing: border-box` rule (CSS line 256) did not reach the rail; check for a competing `.ed-rail` rule with `grep -n "^\.ed-rail {" ui/css/desktop-app-easydrag.css`.

- [ ] **Step 7: Existing tests that touch the rail**

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run 'TestDesktopEasyDragBrowser|TestEasyDragGroups' -count=1 -timeout 15m -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
```

Expected: every listed test `PASS`. They select rail buttons by `data-ed-cmd` or `aria-label` (`desktop_easydrag_browser_test.go:183,190`, `desktop_easydrag_groups_browser_test.go:43,106`), both of which the buttons keep. A test that fails on a position or a size of a rail button gets its number updated here, with the cause named in the commit.

- [ ] **Step 8: Gates and commit**

```
cd WT && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_rail_browser_test.go
```

Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, and no output from vet and gofmt. Write `C:/disposable/easydrag-w1a/d2-commit.txt`:

```
feat(easydrag): 64 px rail with a caption under every icon

The editor rail grows from 48 to 64 px. Each of its seven buttons (Schritte,
Start, Vorlagen, Gruppen, Liste, Fokus, Tasten) shows a short caption at the
label size, from the new keys easydrag.ui.rail_<id> in all 16 locales. The
tooltip and aria-label keep the long name; the caption is hidden from screen
readers. The rail stays a column at every width and scrolls on a very short
window. A Node test finds every rail button in the sources and requires a
caption key and an icon-map entry; a browser test measures the captions at
1440, 900 and 390 px.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd WT && git add ui/lang/easydrag ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-workspace.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-rail.mjs ui/desktop_easydrag_rail_browser_test.go && git commit -F C:/disposable/easydrag-w1a/d2-commit.txt && git log --oneline -1
```

Expected: one commit with 16 locale files, 2 JS files, the CSS file, the Node test and the Go test.

---

### Task D.3: `easydrag-delight.js` and "Lebendigkeit"

**Files:**
- Create: `ui/js/desktop/apps/easydrag-delight.js`
- Create: `scripts/test-easydrag-delight.mjs`
- Create: `C:/disposable/easydrag-w1a/keys/d-3-liveliness.json` (not committed)
- Create: `C:/disposable/easydrag-w1a/d3-edits.mjs` (not committed)
- Modify: `ui/lang/easydrag/*.json` (through the helper)
- Modify: `ui/js/desktop/apps/easydrag-editor.js:457` (existing line)
- Modify: `ui/js/desktop/apps/easydrag.js:224` (existing line)
- Modify: `ui/js/desktop/core/module-loader.js` (after `easydrag-icons.js`)
- Modify: `ui/desktop_easydrag_test.go:12` (module list)
- Modify: `scripts/test-easydrag-extra3.mjs:9` (harness list, if T0 names it)
- Test: `node scripts/test-easydrag-delight.mjs`

**Design.**
- `level()` returns `'calm'` or `'normal'`. It is `calm` when any of three things holds: the user chose Ruhig (`localStorage['aurago.easydrag.liveliness'] === 'calm'`), the system asks for reduced motion (`matchMedia('(prefers-reduced-motion: reduce)')`), or the desktop turned animations off (`document.body.dataset.animations === 'false'`, the attribute that `desktop-foundation.js:729` sets from the setting `windows.animations`; `animationsEnabled()` at line 714 reads the same attribute and the same media query).
- `choice()` returns what the user picked (`'normal'` or `'calm'`, default `'normal'`), so the menu can show it even when the system forces calm.
- `allow(kind)` is true only when `level()` is `normal`. `kind` names the motion (`'run'`, `'drag'`, `'insert'`, `'loading'`); today every kind follows the level, so the argument documents the call site and keeps room for a per-kind rule.
- `bind(root)` sets `data-motion` on a `.ed-app` root and keeps it current. Three watchers run once per page: the media query's `change` event, a `MutationObserver` on `body[data-animations]`, and the `storage` event (another tab changed the choice). A root that is no longer in the document is dropped at the next refresh, so there is no dispose hook to wire.
- The setting never touches `model.version` (it lives in `localStorage` only).
- The menu entry is one parent item "Lebendigkeit" with the two choices "Normal" and "Ruhig" as its submenu; the current choice carries a check mark. It goes into the "⋯" menu of the flow editor and of the block editor (`moreMenu`, `easydrag-editor.js:428`; Step 5). The home page has no app-level menu today (only a per-flow row menu, `easydrag-home.js:678`); its entry waits for W1f.

- [ ] **Step 1: Write the failing Node test**

Create `scripts/test-easydrag-delight.mjs`:

```js
#!/usr/bin/env node
// W1a D.3: the motion gate. allow() is false when the user chose Ruhig, when the system asks for reduced
// motion, and when the desktop turned animations off; data-motion on a bound root follows all three.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const KEY = 'aurago.easydrag.liveliness';
const source = fs.readFileSync(new URL('easydrag-delight.js', apps), 'utf8');

// load runs a fresh module in its own page stub. state.reduced and state.animations stand for the system
// and the desktop setting; change() calls the listeners that the module registered, like the browser.
function load(start = {}) {
    const store = new Map(start.stored ? [[KEY, start.stored]] : []);
    const state = { reduced: !!start.reduced, animations: start.animations };
    const listeners = [];
    const body = { dataset: {} };
    Object.defineProperty(body.dataset, 'animations', { get: () => state.animations, enumerable: true });
    const box = vm.createContext({
        window: {
            matchMedia: query => ({ matches: state.reduced && query.includes('prefers-reduced-motion'), addEventListener: (name, fn) => { if (name === 'change') listeners.push(fn); } }),
            addEventListener() {}
        },
        document: { body }, console, Map, Set,
        localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } }
    });
    vm.runInContext(source, box, { filename: 'easydrag-delight.js' });
    const root = { isConnected: true, attrs: {}, getAttribute(name) { return this.attrs[name] ?? null; }, setAttribute(name, value) { this.attrs[name] = value; } };
    return { delight: box.window.EasyDrag.delight, state, store, root, change: () => listeners.forEach(fn => fn()) };
}

// Nothing asks for calm: normal, and everything may move.
let h = load({ animations: 'true' });
assert.equal(h.delight.level(), 'normal');
assert.equal(h.delight.choice(), 'normal');
for (const kind of ['run', 'drag', 'insert', 'loading']) assert.equal(h.delight.allow(kind), true, kind + ' is allowed in normal');

// Each of the three conditions alone makes allow() false for every kind.
h = load({ animations: 'true', stored: 'calm' });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.choice(), 'calm');
assert.equal(h.delight.allow('run'), false, 'Ruhig blocks motion');

h = load({ animations: 'true', reduced: true });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.choice(), 'normal', 'the system setting does not change the stored choice');
assert.equal(h.delight.allow('run'), false, 'reduced motion blocks motion');

h = load({ animations: 'false' });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.allow('drag'), false, 'data-animations=false blocks motion');
h = load({});
assert.equal(h.delight.level(), 'normal', 'a body without data-animations is not calm');

// setLevel stores the choice per device and ignores anything but the two values.
h = load({ animations: 'true' });
assert.equal(h.delight.setLevel('calm'), 'calm');
assert.equal(h.store.get(KEY), 'calm');
assert.equal(h.delight.allow('run'), false);
assert.equal(h.delight.setLevel('lively'), 'calm', 'an unknown value changes nothing');
assert.equal(h.store.get(KEY), 'calm');
assert.equal(h.delight.setLevel('normal'), 'normal');
assert.equal(h.delight.allow('run'), true);

// data-motion on a bound root follows the three conditions as they change.
h = load({ animations: 'true' });
const unbind = h.delight.bind(h.root);
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.delight.setLevel('calm');
assert.equal(h.root.getAttribute('data-motion'), 'calm');
h.delight.setLevel('normal');
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.state.reduced = true; h.change();
assert.equal(h.root.getAttribute('data-motion'), 'calm', 'the media query change reaches the root');
h.state.reduced = false; h.change();
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.state.animations = 'false'; h.delight.refresh();
assert.equal(h.root.getAttribute('data-motion'), 'calm', 'the desktop setting reaches the root');
h.state.animations = 'true'; h.delight.refresh();
assert.equal(h.root.getAttribute('data-motion'), 'normal');
unbind();
h.delight.setLevel('calm');
assert.equal(h.root.getAttribute('data-motion'), 'normal', 'an unbound root is left alone');
const gone = { isConnected: false, attrs: {}, getAttribute() { return null; }, setAttribute() { throw new Error('a disconnected root must not be written'); } };
h.delight.bind(gone);
h.delight.setLevel('normal');

// The menu entry: one parent item with two choices; the current one carries the check mark.
h = load({ animations: 'true', stored: 'calm' });
const words = { 'easydrag.ui.liveliness': 'Lebendigkeit', 'easydrag.ui.liveliness_normal': 'Normal', 'easydrag.ui.liveliness_calm': 'Ruhig' };
const items = h.delight.menuItems(key => words[key] || key);
assert.equal(items.length, 1);
assert.equal(items[0].label, 'Lebendigkeit');
assert.deepEqual(items[0].items.map(item => item.label), ['Normal', 'Ruhig']);
assert.deepEqual(items[0].items.map(item => !!item.shortcutHint), [false, true], 'the check mark marks the current choice');
items[0].items[0].action();
assert.equal(h.store.get(KEY), 'normal');
assert.equal(h.delight.menuItems(key => key)[0].items[0].shortcutHint.length > 0, true);
console.log('PASS motion gate: three conditions, data-motion binding, menu entry');
```

Run it:

```
cd WT && node scripts/test-easydrag-delight.mjs
```

Expected: it fails with `ENOENT … easydrag-delight.js`.

- [ ] **Step 2: Write the module**

Create `ui/js/desktop/apps/easydrag-delight.js`:

```js
// EasyDrag motion gate ("Lebendigkeit"). One place decides whether anything may move. The level is
// "calm" when the user chose Ruhig, when the system asks for reduced motion, or when the desktop turned
// animations off; otherwise "normal". Each .ed-app root carries data-motion="calm|normal", and the
// stylesheet runs animations only under data-motion="normal". The choice is per device (localStorage) and
// never part of a flow document, so it never bumps model.version.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const KEY = 'aurago.easydrag.liveliness';
    const CHOICES = ['normal', 'calm'];
    const roots = new Set();
    let watching = false;

    // choice is what the user picked; normal is the default.
    function choice() {
        try { return localStorage.getItem(KEY) === 'calm' ? 'calm' : 'normal'; } catch (err) { return 'normal'; }
    }

    function systemCalm() {
        try { return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches); } catch (err) { return false; }
    }

    // desktopOff reads the attribute that desktop-foundation.js sets from the setting windows.animations.
    function desktopOff() {
        const body = document.body;
        return !!(body && body.dataset && body.dataset.animations === 'false');
    }

    // level is 'calm' or 'normal'.
    function level() {
        return choice() === 'calm' || systemCalm() || desktopOff() ? 'calm' : 'normal';
    }

    // allow says whether a motion of this kind may run. kind names the motion ('run', 'drag', 'insert',
    // 'loading'); every kind follows the level today.
    function allow(kind) {
        return level() === 'normal';
    }

    // refresh writes the current level to every bound root and forgets roots that left the document.
    function refresh() {
        const value = level();
        roots.forEach(root => {
            if (!root.isConnected) { roots.delete(root); return; }
            if (root.getAttribute('data-motion') !== value) root.setAttribute('data-motion', value);
        });
        return value;
    }

    // watch starts the three listeners once: the media query, the desktop attribute and other tabs.
    function watch() {
        if (watching) return;
        watching = true;
        try {
            const query = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
            if (query && typeof query.addEventListener === 'function') query.addEventListener('change', refresh);
            else if (query && typeof query.addListener === 'function') query.addListener(refresh);
            if (typeof MutationObserver === 'function' && document.body) new MutationObserver(refresh).observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
            if (typeof window.addEventListener === 'function') window.addEventListener('storage', event => { if (!event.key || event.key === KEY) refresh(); });
        } catch (err) { /* a missing browser feature leaves the level as of the last refresh */ }
    }

    // bind sets data-motion on a .ed-app root and keeps it current; the returned function unbinds it.
    function bind(root) {
        roots.add(root);
        watch();
        refresh();
        return () => { roots.delete(root); };
    }

    // setLevel stores the user's choice ('normal' or 'calm') and returns the level now in effect.
    function setLevel(value) {
        if (!CHOICES.includes(value)) return level();
        try { localStorage.setItem(KEY, value); } catch (err) { /* storage may be blocked: the choice then lasts until reload */ }
        return refresh();
    }

    // menuItems returns the "Lebendigkeit" entry of the editor's more menu: one item with two choices,
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
})();
```

- [ ] **Step 3: Run the Node test**

```
cd WT && node scripts/test-easydrag-delight.mjs
```

Expected: `PASS motion gate: three conditions, data-motion binding, menu entry`. If the last assertion fails on `shortcutHint.length`, the current choice in that run was `normal` after `items[0].items[0].action()`; the check mark must sit on the first item.

- [ ] **Step 4: Register the module and the locale keys**

1. `module-loader.js`: directly after `'/js/desktop/apps/easydrag-icons.js',` add `'/js/desktop/apps/easydrag-delight.js',` (same indent).
2. `ui/desktop_easydrag_test.go:12`: `"easydrag-core.js", "easydrag-icons.js", "easydrag-durable.js",` becomes `"easydrag-core.js", "easydrag-icons.js", "easydrag-delight.js", "easydrag-durable.js",`.
3. `scripts/test-easydrag-extra3.mjs:9`: `'easydrag-icons.js', 'easydrag-durable.js',` becomes `'easydrag-icons.js', 'easydrag-delight.js', 'easydrag-durable.js',`. The same one-token insert goes into any other harness list that the T0 run names in Step 8.

Create `C:/disposable/easydrag-w1a/keys/d-3-liveliness.json`:

```json
{
  "easydrag.ui.liveliness": {
    "cs": "Živost", "da": "Livlighed", "de": "Lebendigkeit", "el": "Ζωντάνια", "en": "Liveliness", "es": "Vivacidad", "fr": "Animation", "hi": "जीवंतता",
    "it": "Vivacità", "ja": "動きの量", "nl": "Levendigheid", "no": "Livlighet", "pl": "Żywość", "pt": "Vivacidade", "sv": "Livlighet", "zh": "动态效果"
  },
  "easydrag.ui.liveliness_normal": {
    "cs": "Normální", "da": "Normal", "de": "Normal", "el": "Κανονική", "en": "Normal", "es": "Normal", "fr": "Normale", "hi": "सामान्य",
    "it": "Normale", "ja": "標準", "nl": "Normaal", "no": "Normal", "pl": "Normalna", "pt": "Normal", "sv": "Normal", "zh": "正常"
  },
  "easydrag.ui.liveliness_calm": {
    "cs": "Klidná", "da": "Rolig", "de": "Ruhig", "el": "Ήρεμη", "en": "Calm", "es": "Tranquila", "fr": "Calme", "hi": "शांत",
    "it": "Calma", "ja": "静か", "nl": "Rustig", "no": "Rolig", "pl": "Spokojna", "pt": "Calma", "sv": "Lugn", "zh": "安静"
  }
}
```

```
cd WT && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/d-3-liveliness.json && grep -c '"easydrag.ui.liveliness' ui/lang/easydrag/de.json ui/lang/easydrag/ja.json
```

Expected: each count `3`.

- [ ] **Step 5: IMPACT, then wire the root and the menu**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "moreMenu" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "render" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

`moreMenu` is local to `create` in `easydrag-editor.js` (called at line 694 only); `render` in `easydrag.js` has many same-named functions in the index. If the result is UNKNOWN, confirm with `grep -n "moreMenu(" ui/js/desktop/apps/easydrag-editor.js` (two lines: the definition and the call at 694) and `grep -rn "EasyDragApp.render\|\.render(container" ui/js/desktop/apps/easydrag.js`. Both edits add one expression to an existing line and cannot change a return value. Report any HIGH or CRITICAL result.

Create `C:/disposable/easydrag-w1a/d3-edits.mjs`:

```js
// The root gets data-motion, and the editor's more menu gets the Lebendigkeit entry (the entry sits
// before the separator that precedes "delete"). Existing lines only: editor.js stays at 896 lines.
export default [
    ['easydrag.js', 224, 'container.appendChild(root);', 'container.appendChild(root); if (ED.delight) ED.delight.bind(root);'],
    ['easydrag-editor.js', 457, '{ separator: true },', '...(ED.delight ? ED.delight.menuItems(t) : []), { separator: true },'],
    // The block editor's menu (the isBlock branch of moreMenu) gets the same entry, after "open draft".
    ['easydrag-editor.js', 433, 'action: () => app.openBlock(ed.flow.id) },', 'action: () => app.openBlock(ed.flow.id) }, ...(ED.delight ? ED.delight.menuItems(t) : []),']
];
```

```
cd WT && node C:/disposable/easydrag-w1a/d-line-edits.mjs C:/disposable/easydrag-w1a/d3-edits.mjs && wc -l ui/js/desktop/apps/easydrag-editor.js && sed -n 457p ui/js/desktop/apps/easydrag-editor.js | cut -c1-120
```

Expected: `applied 3 edits in 2 files`, `896 …`, and the line starts with `...(ED.delight ? ED.delight.menuItems(t) : []), { separator: true },`. The block menu is the `isBlock` branch of `moreMenu`, `easydrag-editor.js:431-435`; the edit is on line 433. Old: `{ icon: 'pencil', label: t('easydrag.ui.block_open_draft'), action: () => app.openBlock(ed.flow.id) },`. New: the same text followed by ` ...(ED.delight ? ED.delight.menuItems(t) : []),`. Check: `grep -c "ED.delight.menuItems(t)" ui/js/desktop/apps/easydrag-editor.js` prints `2`. The helper insists that `{ separator: true },` appears once on line 457; the other separator (line 443) has no trailing comma and is not touched.

- [ ] **Step 6: Run the gates**

```
cd WT && node scripts/test-easydrag-delight.mjs && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
```

Expected: the PASS line, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok  aurago/ui`. `TestEasyDragUIKeysExistInAllLocales` now sees the three `liveliness` literals and the 16 locale files carry them. `TestEasyDragAssetsLoadInDependencyOrder` passes only if the module-loader order and the Go list agree (icons and delight right after core in both).

- [ ] **Step 7: Commit**

Write `C:/disposable/easydrag-w1a/d3-commit.txt`:

```
feat(easydrag): motion gate and the Lebendigkeit setting

ED.delight decides whether anything may move. The level is calm when the
user chose Ruhig, when the system asks for reduced motion or when the
desktop's data-animations is false, and normal otherwise. Every .ed-app
root gets data-motion="calm|normal" and keeps it current when any of the
three changes. The choice is per device in localStorage and never touches
model.version. "Lebendigkeit" is one entry with two choices in the editor's
more menu, with keys in all 16 locales.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd WT && git add ui/js/desktop/apps/easydrag-delight.js ui/js/desktop/apps/easydrag.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/lang/easydrag scripts/test-easydrag-delight.mjs scripts/test-easydrag-extra3.mjs && git commit -F C:/disposable/easydrag-w1a/d3-commit.txt && git log --oneline -1
```

---

### Task D.4: Zero idle animations

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css` (after the keyframes, line 1396: one gate block)
- Create: `ui/desktop_easydrag_motion_browser_test.go`
- Test: `go test ./ui -run TestEasyDragMotionIsCalmWhenIdle`

**The audit.** Method: `grep -n "@keyframes\|animation\s*:" ui/css/desktop-app-easydrag.css` gives 8 keyframes and 15 declarations; `grep -n "\.animate(\|setInterval\|animationend\|getAnimations" ui/js/desktop/apps/easydrag-*.js` finds no JavaScript animation (no Web Animations call, no interval, no animation listener). The `requestAnimationFrame` calls (`core.js:511`, `detail.js:307,374,871`, `editor.js:851`, `forms.js:408`, `palette.js:289,494`) each run once, to add a class that starts a CSS transition or to focus a field; none loops. The canvas class `is-animating` (a pan or zoom transition) is set only while a view change runs.

| # | CSS line | Selector and keyframes | Starts when | Runs while idle? | Decision |
|---|---|---|---|---|---|
| 1 | 306 | `.ed-btn.is-busy .ed-icon` `ed-spin` 1 s infinite | Test or Run is under way (`header.js:118`, `run-stop.js:103`) | No | Stays. Calm: static icon |
| 2 | 414 | `.ed-spinner` `ed-spin` 0.8 s infinite | The window's shell shows "loading" (`easydrag.js:43`) until the app is ready | No (gone once loaded) | Stays. Calm: static ring (accent arc), which is the static glyph |
| 3 | 465 | `.ed-flow-card` `ed-rise` 200 ms, fill both | A card is rendered on the start page | No: ends after 200 ms | Stays |
| 4 | 547 | `.ed-run-dot--running, --queued` `ed-pulse` 1.2 s infinite | The last run or a card's run is running or queued | No (a live run) | Stays. Part A deletes this family; skip the row if A landed |
| 5 | 597 | `.ed-save--saving .ed-icon` `ed-spin` infinite | A save request is in flight | No | Stays. Calm: static icon |
| 6 | 696 | `.ed-node.is-new` `ed-pop` 360 ms | A card was just inserted; the class is removed after 400 ms (`canvas.js:531`) | No | Stays |
| 7 | 718 | `.ed-status--running, --queued` `ed-pulse` 1 s infinite | A step is running | No (a live run) | Stays. Part A deletes this family; skip the row if A landed |
| 8 | 720 | `.ed-node.status-running` `ed-glow` 1.4 s infinite | A step is running | No | Stays. Calm: the card keeps the static blue border, the "static running mark" |
| 9 | 792 | `.ed-edge.is-drop-target .ed-wire` `ed-dash` 0.6 s infinite | A palette item is dragged over a wire (`wires.js:307`) | No | Stays |
| 10 | 798 | `.ed-edge.is-flowing .ed-wire` `ed-dash` 0.5 s infinite | The next step is running (`runs.js:41`) | No | Stays. Calm: static dashes |
| 11 | 802 | `.ed-wire-preview` `ed-dash` 0.6 s infinite | The user drags a new wire; the path is `hidden` otherwise and `.ed-app [hidden] { display: none !important; }` (CSS line 257) stops it | No | Stays |
| 12 | 815 | `.ed-empty .ed-empty-card` `ed-rise` 320 ms | An empty flow is shown | No: ends after 320 ms | Stays |
| 13 | 1040 | `.ed-field.is-flash` `ed-flash` 1.2 s | The user jumps to a field; removed after 1.2 s (`forms.js:541`) | No | Stays |
| 14 | 1140 | `.ed-popover` `ed-rise` 120 ms | A popover opens | No | Stays |
| 15 | 1155 | `.ed-drawer` `ed-slide-in` 200 ms | The run drawer opens | No | Stays |

Keyframes: `ed-spin` (rows 1, 2, 5), `ed-pulse` (4, 7), `ed-dash` (9, 10, 11), `ed-rise` (3, 12, 14), `ed-slide-in` (15), `ed-pop` (6), `ed-glow` (8), `ed-flash` (13). The `@media (prefers-reduced-motion: reduce)` rule and the `data-animations="false"` rule (CSS 1440-1449) stay as a safety net.

**Result of the audit.** The nine infinite declarations (rows 1, 2, 4, 5, 7, 8, 9, 10 and 11) all start from an event: a run, a save, a drag, loading. The six one-shot animations end within 1.2 s. No animation runs in an idle editor or an idle home page, so no animation is removed in this task. The test below is the proof, and it names any animation that disagrees with this table.

**The gate (one rule instead of fifteen edits).** The spec keys every animation to `[data-motion="normal"]`. The same effect, with one rule, is to switch off every animation under any other value. The rule also covers an animation that Part A or a later sub-project adds, and it does not touch the two families that Part A deletes (no merge conflict). Before `ED.delight` has bound a root (or when the module did not load), `data-motion` is absent, which also reads as calm: the safe direction.

- [ ] **Step 1: Write the failing browser test**

Create `ui/desktop_easydrag_motion_browser_test.go`. The shared helper sets reduced motion unless `Motion` is true, so the test opts out, and it also forces the media query to "no-preference" so that a Windows animation setting cannot make the run vacuous. The shared fixture shell sets `data-animations` to `false`, so the test sets it to `true` first.

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// easyDragRunningAnimations lists the animations that run inside .ed-app right now ("name on element"
// joined by " | "). A finished animation with fill-mode both stays in getAnimations(), so only
// playState "running" counts.
const easyDragRunningAnimations = `()=>{
	const app=document.querySelector('.ed-app');
	return document.getAnimations().filter(a=>a.playState==='running' && a.effect && a.effect.target && app.contains(a.effect.target))
		.map(a=>(a.animationName||a.transitionProperty||a.constructor.name)+' on '+a.effect.target.tagName+'.'+String(a.effect.target.getAttribute('class')||'').split(' ')[0]).join(' | ');
}`

// easyDragSettle waits long enough for every one-shot entrance (the longest is 360 ms) to end.
func easyDragSettle(page *rod.Page) {
	page.MustEval(`()=>new Promise(r=>setTimeout(()=>requestAnimationFrame(()=>requestAnimationFrame(r)),1000))`)
}

// TestEasyDragMotionIsCalmWhenIdle checks the motion gate in the real editor (spec 6.4):
//  1. normal motion, idle editor: nothing runs;
//  2. normal motion, a step running: something runs (so the first check is not vacuous);
//  3. Ruhig, chosen through the editor's more menu, the same step still running: nothing runs.
func TestEasyDragMotionIsCalmWhenIdle(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page

	// The helper leaves the media query alone; say "no preference" explicitly, and turn the desktop's
	// animations on (the fixture shell sets windows.animations to false).
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelector('[data-ed-state]').textContent.length>0`)
	s.wait(`()=>document.querySelector('.ed-app').dataset.motion==='normal'`)
	if !p.MustEval(`()=>!matchMedia('(prefers-reduced-motion: reduce)').matches && document.body.dataset.animations==='true'`).Bool() {
		t.Fatal("the page still asks for reduced motion; the idle check would be vacuous")
	}

	// 1. Idle editor, normal motion.
	easyDragSettle(p)
	if got := p.MustEval(easyDragRunningAnimations).Str(); got != "" {
		t.Fatalf("idle editor, normal motion: running animations: %s", got)
	}

	// 2. A step runs, normal motion: at least one animation runs.
	p.MustEval(`()=>{edFixture.state.delay=600000}`)
	s.fit()
	p.KeyActions().Press(input.ControlLeft).Type(input.Enter).MustDo()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]') && document.querySelectorAll('.ed-modal .ed-effects li').length>=1`)
	p.MustEval(`()=>{const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node.status-running').length===1`)
	easyDragSettle(p)
	if got := p.MustEval(easyDragRunningAnimations).Str(); got == "" {
		t.Fatal("normal motion with a running step: no animation runs; the gate test would be vacuous")
	} else {
		t.Logf("normal motion, running step: %s", got)
	}

	// 3. Ruhig through the real menu entry; the step keeps running, nothing moves.
	p.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-context-menu')`)
	clicked := p.MustEval(`()=>{
		const parent=[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(b=>b.textContent.includes('Lebendigkeit'));
		const calm=[...document.querySelectorAll('.vd-context-submenu-popover .vd-context-item')].find(b=>b.textContent.trim().startsWith('Ruhig'));
		if(!parent||!calm)return 'the Lebendigkeit entry or its Ruhig choice is missing from the more menu';
		calm.click();
		return 'ok';
	}`).Str()
	if clicked != "ok" {
		t.Fatal(clicked)
	}
	s.wait(`()=>document.querySelector('.ed-app').dataset.motion==='calm' && localStorage.getItem('aurago.easydrag.liveliness')==='calm'`)
	easyDragSettle(p)
	if p.MustEval(`()=>document.querySelectorAll('.ed-node.status-running').length`).Int() != 1 {
		t.Fatal("the step stopped running; the Ruhig check would be vacuous")
	}
	if got := p.MustEval(easyDragRunningAnimations).Str(); got != "" {
		t.Fatalf("Ruhig with a running step: running animations: %s", got)
	}
	s.failOnPageErrors("motion")
}
```

- [ ] **Step 2: Run it and watch it fail for the right reason**

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run 'TestEasyDragMotionIsCalmWhenIdle' -count=1 -timeout 10m -v 2>&1 | tail -12
```

Run with the Bash sandbox disabled. Expected: the test reaches phase 3 and fails with `Ruhig with a running step: running animations: ed-glow on DIV.ed-node | …`. Phases 1 and 2 pass: the idle editor is already quiet (the audit's finding) and the running step animates. The gate does not exist yet, so Ruhig only changes the attribute.

What each other outcome means:
- Phase 1 fails (`idle editor, normal motion: running animations: <name> on <element>`): an animation runs while idle, against the audit. Find its row in the table by name. If the element is a real "something is happening" state that the fixture reaches by itself (a queued run, a saving icon that never ends), bind it to that state; if it is decorative and runs for no event, remove its declaration, and list it as removed in the commit and the report. Never loosen the test.
- Phase 2 fails with `no animation runs`: `data-animations` or the media query did not take. Read the log line before it: `matchMedia` and `body.dataset.animations` are printed by the guard in the test.
- Phase 3 stops at `the Lebendigkeit entry … is missing`: the submenu was not rendered. Open the menu in the screenshot (`s.shot("motion-more-menu")` can be added before the click) and check that the editor's "⋯" button is `[data-ed-cmd="more"]` and `ED.delight` is bound (`edFixture`'s page has `window.EasyDrag.delight`).

- [ ] **Step 3: Add the gate**

In `ui/css/desktop-app-easydrag.css`, directly after the `@keyframes ed-flash { … }` line (line 1396) and before `@media (pointer: coarse) {` add:

```css

/* Lebendigkeit: one gate for all motion. ED.delight sets data-motion="normal" on .ed-app when nothing asks
   for calm. Under any other value (Ruhig, reduced motion, desktop animations off, or no value yet) no
   animation runs: a running step keeps its static blue border and disc, a spinner stays a static ring or
   icon, flowing wires keep their dashes. Transitions are not affected; they end by themselves. */
.ed-app:not([data-motion="normal"]) *, .ed-app:not([data-motion="normal"]) *::before, .ed-app:not([data-motion="normal"]) *::after {
    animation: none !important;
}
.ed-app:not([data-motion="normal"]) .ed-edge.is-flowing .ed-wire { stroke-dasharray: none; }
```

The first rule has the specificity of `.ed-app:not([...]) *` plus `!important`, so it wins over every `animation:` declaration in the file (and over Part A's). The second rule gives a flowing wire the same static look as under the existing reduced-motion rule.

Proof that the gate sits after every animation and that nothing else changed in the animation rules:

```
cd WT && grep -n "data-motion" ui/css/desktop-app-easydrag.css && git diff --stat ui/css/desktop-app-easydrag.css
```

Expected: the two `data-motion` rules (and the comment line) and one file changed with only added lines.

- [ ] **Step 4: Run the test again**

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run 'TestEasyDragMotionIsCalmWhenIdle' -count=1 -timeout 10m -v 2>&1 | tail -8
```

Expected: `--- PASS: TestEasyDragMotionIsCalmWhenIdle` and a log line `normal motion, running step: ed-glow on DIV.ed-node` (plus any other running animation, such as `ed-dash on path.ed-wire`).

- [ ] **Step 5: Full EasyDrag browser run**

Most existing browser tests run with reduced motion and `data-animations` false, so the gate leaves them in the calm state they always had. Run everything:

```
cd WT && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-shots/d-final.txt | grep -c -- '--- PASS'
grep -c -- '--- SKIP' D:/tmp/easydrag-w1a-shots/d-final.txt; grep -c -- '--- FAIL' D:/tmp/easydrag-w1a-shots/d-final.txt
```

Expected: the baseline's PASS count plus 2 (the rail test and the motion test), the same SKIP count, and `0` FAIL. With the baseline of 41 PASS and 2 SKIP that is 43 PASS, 2 SKIP, 0 FAIL. Write the real numbers into the final report.

- [ ] **Step 6: Gates and commit**

```
cd WT && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && npm run test:easydrag:t0 2>&1 | tail -3 && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_motion_browser_test.go
```

Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `T0 passed`, and no output from vet and gofmt. If T0 names a harness script that cannot find `ED.icons` or `ED.delight`, add the two module names after `'easydrag-core.js'` in that script's list and run T0 again.

Write `C:/disposable/easydrag-w1a/d4-commit.txt`:

```
feat(easydrag): no animation runs unless motion is normal

One stylesheet gate switches off every animation under .ed-app unless
data-motion is "normal", so Ruhig, reduced motion and desktop animations
off all stop them together; a running step keeps its static blue border
and spinners stay a static ring or icon. The audit of the 8 keyframes and
15 animation declarations found that every infinite one starts from an
event (a run, a save, a drag, loading) and the rest end within 1.2 s, so
none is removed. TestEasyDragMotionIsCalmWhenIdle proves it in the real
editor: idle editor quiet, a running step animates under Normal, and
nothing moves under Ruhig, which it chooses through the more menu.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd WT && git add ui/css/desktop-app-easydrag.css ui/desktop_easydrag_motion_browser_test.go && git commit -F C:/disposable/easydrag-w1a/d4-commit.txt && git log --oneline -4
```

Expected: four commits on `feat/easydrag-w1a-rail` (D.1 to D.4). Nothing is rebased, amended or pushed here; the coordinator rebases before landing, and a locale conflict is solved by taking main's files and running `d-2-rail.json` and `d-3-liveliness.json` again.

---

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §6.1 rail 64 px, caption under each icon at `--ed-font-label`, 7 keys in 16 locales | D.2 |
| §6.1 tooltips keep the long name | D.2 (aria-label and title untouched; the caption is `aria-hidden`) |
| §6.1 narrow widths: captions fit at 900 and 390 px | D.2 (the browser test at 1440, 900 and 390 px) |
| §6.2 `ED.icons`, `MEANINGS`, `of()`, no new glyph family | D.1 |
| §6.2 "Start" and "Gruppen" stop sharing `grid` | D.1 (`home` and the new `group-frame`) |
| §6.2 rail and header read icons through the map | D.1 (19 call sites, grep proof) |
| §6.3 `ED.delight` `allow`, `level`, `setLevel`, per-device storage | D.3 |
| §6.3 `data-motion` on `.ed-app`, kept current | D.3 (bind, media query, observer, storage event) |
| §6.3 Lebendigkeit menu entry, keys in 16 locales | D.3 |
| §6.3 audit of 8 keyframes and the `animation` declarations | D.4 (table) |
| §6.3 every animation keyed to `data-motion="normal"`; Ruhig stops all | D.4 (one gate rule; see deviation 2) |
| §6.3 Ruhig: static running mark and static spinners | D.4 (gate; the static border and ring remain) |
| §6.4 Node: icon map has no glyph with two meanings | D.1 |
| §6.4 Node: every rail button has a caption key and a map entry | D.2 |
| §6.4 Node: `allow()` is false under each of the three conditions | D.3 |
| §6.4 Browser: `TestEasyDragMotionIsCalmWhenIdle` (idle, Ruhig + running, Normal + running) | D.4 |
| §6.4 Browser: rail at 1440 and 900 px unclipped | D.2 |
| §10 "The rail is 64 px with a caption under every icon. No glyph carries two meanings." | D.1, D.2 |
| §10 "Zero animations run in an idle editor, and Ruhig removes every animation while a step runs." | D.4 |
| §11 risk "removing an idle animation removes a cue" | D.4: nothing is removed |

### Deviations from the spec

1. **The rail is not a row at 390 px.** The spec says the rail "moves below 900 px (CSS around 906) and is a row at 390 px". There is no such rule: line 906 is `.ed-rail { order: -2; }`, which only puts the rail left of the outline panel. The rail is a column at every width, so the plan keeps it a 64 px column and the browser test measures it at 390 px as well (captions unclipped, 64 px wide, no scrolling). The rail takes 16 % of a 390 px screen; whether that is acceptable on a phone is for the Wave 1 sessions (decision 2 of the owner, retiring the rail).
2. **The motion gate is one negative rule, not edits to 15 declarations.** `.ed-app:not([data-motion="normal"]) … { animation: none !important; }` has the effect the spec asks for (every animation runs only under `normal`) and also covers animations that other parts add later. Rewriting each declaration would conflict with Part A, which deletes two of the animated families. A root without `data-motion` (module not loaded) is calm.
3. **No animation is removed.** The audit found that all nine infinite animations are event-driven and the six one-shot ones end within 1.2 s. The test is the evidence; the plan carries a decision rule for the case where it names a surprise.
4. **`ED.delight` has more members than the contract names:** `choice()`, `bind(root)`, `refresh()` and `menuItems(t)`. The contract members (`allow`, `level`, `setLevel`, the storage key, `data-motion`) are exactly as agreed.
5. **`ED.icons` has `RAIL` and `caption(t, id)`** in addition to `of` and `MEANINGS`. The rail keys are written out as literals so that the locale test needs no new family entry.
6. **The new glyph `group-frame`** is added to `ICONS` (the spec allows it when no existing glyph fits). Justification is in Task D.1.
7. **`TestEasyDragMotionIsCalmWhenIdle` changes the page's `data-animations` to `true`.** The shared fixture shell sets `windows.animations` to `false`, which means that every existing EasyDrag browser test has run in the calm state; the spec's "Normal" case needs the opposite. The test also forces the media query to `no-preference`, because the Windows animation setting flips it (memory note `reference_windows_motion_browser_tests.md`).
8. **The test counts only `playState === 'running'`.** `getAnimations()` keeps finished `fill: both` animations, so "empty list" in the spec is read as "nothing running".

### Decisions needed

None open. The coordinator settled the four questions on 2026-11-13:

1. **Lebendigkeit entry:** added to the block editor's menu now (Task D.3, Step 5, line 433). The home page entry waits for W1f.
2. **Groups glyph:** `group-frame` stays.
3. **Native-speaker check of the short captions** (Japanese "雛形", and the Hindi, Greek and Danish forms) is a follow-up note and does not block this part.
4. **Stale queued or running pulse:** left as it is; W1c decides.
