[← Plan index](README.md) · Spec: [plans/2026-11-10-easydrag-s0-spec.md](../2026-11-10-easydrag-s0-spec.md)

## Part 2: Header, home grid and fields splits, headroom ratchet

**Preconditions.** Part 1 is committed on `feat/easydrag-s0`:
- render counters and `scripts/test-easydrag-render-baseline.mjs`;
- runs.js split into `easydrag-run-stream/start/stop/drawer.js`, each loading right before `easydrag-runs.js` in all registries;
- runs.js at 400 lines or fewer.

**Shell.** Every command runs in PowerShell from the worktree root. Start each shell call with `Set-Location C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0;`. GitNexus `impact` is the exception: it uses absolute paths and points at the main checkout.

**Line numbers** are as of main 3da58db83, the same as the spec. Each edit also names its anchor text, because Part 1 and earlier tasks shift lines.

**Test commands used below.**
- `GO-STATIC`: `go test ./ui/ -run '(?i)(EasyDrag)' -count=1`. Without the env var the browser tests skip. Expected: `ok  	aurago/ui`.
- `NODE`: `node scripts/test-easydrag.mjs`. Expected last line: `all EasyDrag checks passed`, exit 0. It runs extra, extra2, extra3 (the c1d07 sandbox), extra4, extra5, extra6, extra7, live-state, kv-editor, flagship, pack and ux-batch5.
- `BASELINE`: `node scripts/test-easydrag-render-baseline.mjs`. Expected PASS, with every count unchanged.
- `BROWSER`: `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE`. Expected: `ok  	aurago/ui`.
  - The gate is `requirePrecisionBrowserSmoke`, in `ui/precision_workspace_browser_test.go:13-18`, which reads `AURAGO_RUN_BROWSER_SMOKE=1`.
  - `startEasyDragSmoke` (`ui/desktop_easydrag_browser_test.go:347`) loads the app through `module-loader.js`. Browser tests therefore have no hard-coded script lists.
  - If one browser check fails, re-run that test alone before treating it as a regression. Windows motion settings can flip reduced motion.
- `IMPACT <names>`: `foreach ($n in <names>) { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $n --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo }`
- `DETECT`: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .`, from the worktree root (spec §9). A `partial` or `truncated` result is not clean; run it again.

**PLAN-CHECK "hard-coded script lists in tests", settled.** These lists load EasyDrag files by name:
- `scripts/test-easydrag-extra.mjs:9-10`
- `scripts/test-easydrag-extra2.mjs:9-10`
- `scripts/test-easydrag-extra3.mjs:9-11` (the c1d07 sandbox that extra5/6/7, live-state, kv-editor, flagship, pack and ux-batch5 reuse)
- `scripts/test-easydrag-extra4.mjs:11-12`
- `scripts/test-easydrag.mjs:28`
- the Go list `easyDragModules` (`ui/desktop_easydrag_test.go:11-16`)

Text pins in moved ranges: none. `extra5:621` pins `'aurago.easydrag.view.'` in editor.js, and that line stays. The email adapter checks (`ui/desktop_email_test.go:16`, `scripts/test-desktop-email-adapters.mjs:90`) read editor.js's `.attach(` block, which does not move.

---

### Task 2.1: easydrag-header.js out of easydrag-editor.js

**Files:**
- Create: `ui/js/desktop/apps/easydrag-header.js`
- Modify: `ui/js/desktop/apps/easydrag-editor.js`:
  - :1-2 header comment;
  - :8 `SAVE_ICONS`;
  - glue inserted before :124 `ed.saver = ED.saver.create({`;
  - remove :136-335 (`// ── header and footer` through the blank line after `renderLastRun`).
- Modify: `ui/js/desktop/core/module-loader.js` :319-320 (between extraction and editor)
- Modify: `ui/desktop_easydrag_test.go`: :15 (`easyDragModules`) and a new test after :107
- Modify: `scripts/test-easydrag-extra3.mjs` :11
- Check: `scripts/test-easydrag-render-baseline.mjs` (Part 1)

**Closure variables the moved code (editor.js:138-334) reads, and how it gets them:**

| editor.js name | how header.js gets it |
|---|---|
| `ed`, `el`, `isBlock`, `nameInput` (:85) | `host.ed`, `host.el`, `host.isBlock`, `host.nameInput` |
| `t`, `esc`, `ctx` | `ed.t`, `ed.esc`, `ed.ctx`, the same objects as `app`'s (editor.js:26, :90) |
| `core` | `ED.core`, inside `create()` |
| `canvas` (:102), `palette` (:106), `runs` (:110), `publish` (:111) | `host.*`. Each is assigned once, before the header is created. |
| `renderBanner` (:341, stays in the editor, run view) | `host.renderBanner`, a hoisted function passed by reference |
| `disposed` (let :50) | `host.isDisposed()` |
| `lastRecord` (let :44, written by the bus at :812) | `host.lastRecord()` |
| `SAVE_ICONS` (:8) | moves into header.js |
| `draftModel` (listed in spec §4) | not read by any moved line, so not passed |

- [ ] **Step 1: Impact.** Run `IMPACT 'stateOf','runNowState','revisionText','runNowTitle','paintRevision','liveChip','timersChip','renderHeader','renderSaveState','syncSaveChip','renderIssues','renderLastRun'`.
  - Read the entries for `ui/js/desktop/apps/easydrag-editor.js`.
  - Expected: callers only inside `build()`, because these are closure-locals. `UNKNOWN` is likely for that reason.
  - Confirm with Grep `\b(stateOf|runNowState|revisionText|runNowTitle|paintRevision|liveChip|timersChip|renderHeader|renderSaveState|syncSaveChip|renderIssues|renderLastRun)\b` over `ui/js/desktop/apps` (glob `easydrag*.js`) and `scripts` (glob `test-easydrag*.mjs`). Expected: hits only in easydrag-editor.js.
  - Also Grep `^\s+(canvas|palette|runs|publish) = ` in editor.js. Expected: one hit each (:102, :106, :110, :111).
  - Report any HIGH or CRITICAL risk before editing.

- [ ] **Step 2: Failing test.** In `ui/desktop_easydrag_test.go`:
  - In `easyDragModules` (:15), change `"easydrag-extraction.js", "easydrag-editor.js",` to `"easydrag-extraction.js", "easydrag-header.js", "easydrag-editor.js",`.
  - After `TestEasyDragAppLifecycle` (after :107), add:
```go
// easyDragSplitSeams pins the S0.1 module splits: each moved function lives only in its new
// module, the module that lost it reaches it through the new module, and the facades other
// modules call keep their exact export line.
var easyDragSplitSeams = []struct {
	file   string
	has    []string
	hasNot []string
}{
	{"easydrag-header.js", []string{"function renderHeader(", "function renderSaveState(", "function syncSaveChip(", "function renderIssues(", "function renderLastRun(", "function runNowState(", "const SAVE_ICONS", "ED.header = { create };"}, nil},
	{"easydrag-editor.js", []string{"ED.header.create({", "function renderBanner(", "ED.editor = { create };"}, []string{"function renderHeader(", "function renderSaveState(", "function stateOf(", "function runNowState(", "const SAVE_ICONS"}},
}

func TestEasyDragSplitSeams(t *testing.T) {
	for _, seam := range easyDragSplitSeams {
		text := readDesktopAssetText(t, "js/desktop/apps/"+seam.file)
		for _, marker := range seam.has {
			if !strings.Contains(text, marker) {
				t.Errorf("%s misses %q", seam.file, marker)
			}
		}
		for _, marker := range seam.hasNot {
			if strings.Contains(text, marker) {
				t.Errorf("%s still has %q, which moved out in S0.1", seam.file, marker)
			}
		}
	}
}
```

- [ ] **Step 3: Run it and see it fail.** Run `GO-STATIC`. Expected FAIL:
  - `easydrag assets miss easydrag-header.js` (TestEasyDragAssetsLoadInDependencyOrder);
  - TestEasyDragAppLifecycle and TestEasyDragSplitSeams fail to read `js/desktop/apps/easydrag-header.js`.

- [ ] **Step 4: Create `ui/js/desktop/apps/easydrag-header.js`.**
```js
// EasyDrag editor header and footer: the flow name, the state, revision, live and timer chips,
// the action buttons, the save state, the issues count and the last run. easydrag-editor.js
// creates it once per editor and keeps the run view (renderBanner), the commands and the bus wiring.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const SAVE_ICONS = { saved: 'check', dirty: 'pencil', saving: 'refresh', invalid: 'alert', offline: 'alert', failed: 'alert', conflict: 'alert' };

    // create(host) builds the header renderers of one editor. host: {ed, el, isBlock, canvas, runs,
    // palette, publish, nameInput, renderBanner(), isDisposed(), lastRecord()}; t, esc and ctx come from ed.
    function create(host) {
        const core = ED.core;
        const { ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner } = host;
        const { t, esc, ctx } = ed;

        // >>> editor.js lines 138-334 go here (Step 5) <<<

        return { renderHeader, renderSaveState, renderIssues, renderLastRun, runNowState };
    }

    ED.header = { create };
})();
```
  The header holds no listeners or timers, so it needs no dispose.

- [ ] **Step 5: Move the code.** Work bottom-up in editor.js.
  1. Cut lines 138-334 verbatim and paste them over the `// >>> ... <<<` line. The range runs from `// stateOf names the flow's state with the words of Mission Control and the missions page:` through the closing `}` of `renderLastRun`. The indentation stays at 8 spaces.
  2. In the pasted code, make exactly three edits:
     - `if (disposed) return;` (was :214) becomes `if (host.isDisposed()) return;`
     - `if (hadFocus && !disposed)` (was :285) becomes `if (hadFocus && !host.isDisposed())`
     - `: lastRecord);` (was :321) becomes `: host.lastRecord());`
  3. In editor.js, delete the now-empty block, from `// ── header and footer ───…` (:136) through the blank line before `// ── run view ───…` (:335).
  4. Insert before `ed.saver = ED.saver.create({` (:124). It must come before the saver, because `onState: renderSaveState` reads the const:
```js
        // The header and the footer (name, state and revision chips, actions, save state, issues,
        // last run) render in easydrag-header.js. The run view's banner stays here; renderHeader calls it.
        const headerView = ED.header.create({
            ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner,
            isDisposed: () => disposed, lastRecord: () => lastRecord
        });
        const { renderHeader, renderSaveState, renderIssues, renderLastRun, runNowState } = headerView;
```
  5. Delete line 8 `const SAVE_ICONS = …;`.
  6. Replace the header comment at lines 1-2 with:
```js
// EasyDrag editor screen: palette + canvas, window menus, keyboard shortcuts, run view and the
// wiring of all modules. The header and footer (name, state, actions, last run, issues, save state) are easydrag-header.js.
```
  - The markers pinned by `ui/desktop_easydrag_test.go:93` stay in editor.js: `ctx.setWindowBeforeClose(ed.windowId, leave)`, `typeof ctx.isActive === 'function' && !ctx.isActive()`, `bag.listen(document, 'aurago:flows-changed'`.
  - No call to these functions runs before the new const. The bus listeners (:809-838), `enterRunView`/`exitRunView`, `setMenus` (:629, :698) and the first render (:1018-1021) all run later.

- [ ] **Step 6: Register.**
  - In `module-loader.js`, between `'/js/desktop/apps/easydrag-extraction.js',` and `'/js/desktop/apps/easydrag-editor.js',`, add the line `'/js/desktop/apps/easydrag-header.js',`.
  - In `scripts/test-easydrag-extra3.mjs` :11, change `'easydrag-extraction.js', 'easydrag-editor.js'` to `'easydrag-extraction.js', 'easydrag-header.js', 'easydrag-editor.js'`.
  - Grep `easydrag-editor.js` in `scripts/test-easydrag-render-baseline.mjs`. If that script has its own module list, insert `'easydrag-header.js'` before `'easydrag-editor.js'` there and add the file to Step 9's `git add`. If it takes its sandbox from test-easydrag-extra3.mjs, nothing changes.

- [ ] **Step 7: Self-check.**
  - Grep `disposed|lastRecord` in easydrag-header.js. Expected: only the doc comment, `host.isDisposed()` twice and `host.lastRecord()` once.
  - `(Get-Content ui/js/desktop/apps/easydrag-editor.js).Length`. Expected about 874, and no more than 900.

- [ ] **Step 8: Run the tests.** `GO-STATIC`, `NODE`, `BASELINE`, `BROWSER`. All pass; the baseline counts are unchanged.

- [ ] **Step 9: Commit.** Run `DETECT` and check that the changed symbols are only the header functions and `build`. Then:
```
git add ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra3.mjs
git commit -m "refactor(easydrag): move the editor header and footer into easydrag-header.js" -m "S0.1: stateOf, runNowState, revisionText, runNowTitle, paintRevision, liveChip, timersChip, renderHeader, renderSaveState, syncSaveChip, renderIssues and renderLastRun move unchanged; the editor passes disposed and lastRecord as getters and keeps renderBanner. TestEasyDragSplitSeams pins the seam." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2.2: easydrag-home-grid.js out of easydrag-home.js

**Files:**
- Create: `ui/js/desktop/apps/easydrag-home-grid.js`
- Modify: `ui/js/desktop/apps/easydrag-home.js`:
  - :372-429: `triggerLabels`, `statusBadge`, `liveBadge`, `lastRun`, `visibleFlows`;
  - :437: `let spoken = '';`;
  - :471-519: `errorCard`, `staleBanner`, `timersBanner`, `emptyCard`, `noMatchCard`, `speak`;
  - :525-588: `cardName`, `render`.
- Modify: `ui/js/desktop/core/module-loader.js` :313-314 (between pack and home)
- Modify: `ui/desktop_easydrag_test.go` :15 and `easyDragSplitSeams`
- Modify: `scripts/test-easydrag-extra3.mjs` :11, `scripts/test-easydrag-extra4.mjs` :12, `scripts/test-easydrag.mjs` :28

**What stays in home.js, and why.**
- The state lets (:433-441, except `spoken`) and `errorIdentity`/`authLoss`/`writesRevoked`/`createOff`/`blockReason` (:443-469) stay, because `reload` (:597-652), `mayChange` (:704), `cardMenu` (:837) and the window menu (export :1045) use them.
- `reportLoaded` (:520-523) stays.
- The click, keydown and contextmenu handlers (:892-960) stay.
- **Spec deviation.** The spec's callbacks (`runNow` :812, `toggle` :827, `cardMenu` :837, `duplicate` :783, `retry` :690, `focusLeftFlow` :676) are targets of that click handler. `render()` never calls them, so they are not passed. The callbacks object holds what render really calls.

**Closure variables the moved code reads, and how it gets them:**

| home.js name | how home-grid.js gets it |
|---|---|
| `flows`, `templates`, `filter`, `sortBy`, `query`, `templateQuery`, `templateCategory`, `prefs`, `favoritesOnly`, `organizationFolder`, `organizationTag`, `loadError`, `staleError`, `timersRunning`, `templateError` (lets) and `selectedFlows` (const Map) | `host.state()` snapshot, which `pull()` reads as the first line of `render()` |
| `libraryControls`, `organizationControls`, `selectionControls`, `createOff`, `blockReason`, `metadataOf`, `metadataMarkup` | `host.*` (hoisted functions of home.js) |
| `app` (catalog in `triggerLabels`), `el`, `grid`, `tplGrid` | `host.*` |
| `previewSVG` (module level in home.js, still exported by `ED.home`) | `host.previewSVG` |
| `t`, `esc` | `app.t`, `app.esc` |
| `core` | `ED.core` |
| `spoken` | moves (only `speak` used it) |

The snapshot is safe: every moved helper runs only inside `render()`. During a render, nothing reassigns a snapshot variable: `libraryControls`, `organizationControls` and `selectionControls` only read them. `selectedFlows.delete` changes the shared Map object.

- [ ] **Step 1: Impact.** Run `IMPACT 'triggerLabels','statusBadge','liveBadge','lastRun','visibleFlows','errorCard','staleBanner','timersBanner','emptyCard','noMatchCard','speak','cardName','render'`.
  - Use the entries for `ui/js/desktop/apps/easydrag-home.js`. `render` matches many files.
  - Expected: callers only inside home.js `create()`, because these are closure-locals.
  - Confirm with Grep `\b(triggerLabels|statusBadge|liveBadge|visibleFlows|errorCard|staleBanner|timersBanner|emptyCard|noMatchCard|cardName)\b` over `ui/js/desktop/apps` (glob `easydrag*.js`). Expected: home.js only.
  - Grep `\brender\(` in home.js. Expected: calls only, no bare references.
  - Report HIGH or CRITICAL before editing.

- [ ] **Step 2: Failing test.**
  - In `easyDragModules`, change `"easydrag-pack.js", "easydrag-home.js",` to `"easydrag-pack.js", "easydrag-home-grid.js", "easydrag-home.js",`.
  - Append two rows to `easyDragSplitSeams`:
```go
	{"easydrag-home-grid.js", []string{"function visibleFlows(", "function cardName(", "function errorCard(", "function emptyCard(", "function speak(", "function render() {", "ED.homeGrid = { create };"}, nil},
	{"easydrag-home.js", []string{"ED.homeGrid.create({", "function render() { gridView.render(); }", "function createOff(", "async function runNow(", "function focusLeftFlow(", "ED.home = { create, previewSVG, pauseOps };"}, []string{"function visibleFlows(", "function cardName(", "function errorCard(", "function statusBadge(", "let spoken"}},
```

- [ ] **Step 3: Run it and see it fail.** Run `GO-STATIC`. Expected FAIL: `easydrag assets miss easydrag-home-grid.js`, plus the read failures for the new file.

- [ ] **Step 4: Create `ui/js/desktop/apps/easydrag-home-grid.js`.**
```js
// EasyDrag start page grid: the flow cards (triggers, state and live badges, last run, favourite,
// switch and menu buttons), the stale and timer banners, the error, empty and no-match cards and
// the template cards. easydrag-home.js keeps the state, the requests and the event handlers.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // create(host) returns {render} for one start page. host: {app, el, grid, tplGrid, previewSVG,
    // libraryControls(), organizationControls(), selectionControls(), createOff(), blockReason(),
    // metadataOf(flow), metadataMarkup(flow), state()}; state() returns the page's current values.
    function create(host) {
        const core = ED.core;
        const { app, el, grid, tplGrid, previewSVG, libraryControls, organizationControls, selectionControls, createOff, blockReason, metadataOf, metadataMarkup } = host;
        const { t, esc } = app;
        // spoken is the grid state announced last: speak says each state once.
        let spoken = '';
        // The page's state, read at the start of every render; the helpers below run only inside render().
        let flows, templates, filter, sortBy, query, templateQuery, templateCategory, prefs, favoritesOnly,
            organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError;
        function pull() {
            ({ flows, templates, filter, sortBy, query, templateQuery, templateCategory, prefs, favoritesOnly,
                organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError } = host.state());
        }

        // >>> home.js lines 372-428 (triggerLabels .. visibleFlows) <<<

        // >>> home.js lines 471-518 (errorCard .. speak) <<<

        // >>> home.js lines 525-587 (cardName, render) <<<

        return { render };
    }

    ED.homeGrid = { create };
})();
```

- [ ] **Step 5: Move the code.** Work bottom-up in home.js.
  1. Cut :525-587, from `// cardName is a flow card's accessible name (U7)` through render's closing `}`. Paste it over the third marker. Add `pull();` as the first statement of `render()`, before `const list = visibleFlows();`. Delete the leftover blank line :588.
  2. In home.js, put the glue where :525 was, after `reportLoaded` and its blank line:
```js
        // The grid render path (flow cards, badges, banners, error, empty and no-match cards,
        // template cards) is easydrag-home-grid.js; it reads this page's state at every render.
        const gridView = ED.homeGrid.create({
            app, el, grid, tplGrid, previewSVG, libraryControls, organizationControls, selectionControls, createOff, blockReason, metadataOf, metadataMarkup,
            state: () => ({ flows, templates, filter, sortBy, query, templateQuery, templateCategory, prefs, favoritesOnly,
                organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError })
        });
        function render() { gridView.render(); }
```
     No `render()` call runs before this const. :148/:149/:346/:369 sit inside functions, `reload` renders after an await, and the first synchronous call is at :1042.
  3. Cut :471-518, from `// errorCard offers the actions of the window's lock card` through speak's closing `}`. Paste it over the second marker. Delete the blank line :519.
  4. Delete :437 `let spoken = '';`.
  5. Cut :372-428, from `function triggerLabels(f) {` through visibleFlows' closing `}`. Paste it over the first marker. Delete the blank line :429.

- [ ] **Step 6: Register.**
  - In `module-loader.js`, add `'/js/desktop/apps/easydrag-home-grid.js',` between the pack and home lines.
  - `extra3.mjs` :11: change `'easydrag-pack.js', 'easydrag-home.js'` to `'easydrag-pack.js', 'easydrag-home-grid.js', 'easydrag-home.js'`.
  - `extra4.mjs` :12: change `'easydrag-library-pause.js', 'easydrag-home.js'` to `'easydrag-library-pause.js', 'easydrag-home-grid.js', 'easydrag-home.js'`.
  - `test-easydrag.mjs` :28: the same change as extra4. home-grid.js has no load-time dependencies.
  - Apply the baseline-script check from Task 2.1 Step 6, with `easydrag-home.js` as the anchor.

- [ ] **Step 7: Self-check.**
  - Grep `\b(ctx|opts|reload|hasOrganization|PAUSE_LIMIT|activePause|mayChange)\b` in easydrag-home-grid.js. Expected: no match.
  - Grep `\b(visibleFlows|cardName|statusBadge|liveBadge|errorCard|emptyCard|noMatchCard|staleBanner|timersBanner|speak|spoken|triggerLabels)\b` in easydrag-home.js. Expected: no match.
  - `(Get-Content ui/js/desktop/apps/easydrag-home.js).Length`. Expected about 886, and no more than 900.

- [ ] **Step 8: Run the tests.** `GO-STATIC`, `NODE`, `BASELINE`, `BROWSER`. All pass; the home checks are in extra5 (:150-760) and ux-batch5 (:649), and the baseline is unchanged.

- [ ] **Step 9: Commit.** Run `DETECT`, then:
```
git add ui/js/desktop/apps/easydrag-home-grid.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs scripts/test-easydrag.mjs
git commit -m "refactor(easydrag): move the start page grid render into easydrag-home-grid.js" -m "S0.1: badges, visibleFlows, the error/empty/no-match cards, cardName and render move unchanged; render reads the page state through a host.state() snapshot and calls back the library, organization and selection controls. ED.home is unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2.3: ED.fieldsShared and easydrag-template-field.js out of easydrag-fields.js

**Files:**
- Create: `ui/js/desktop/apps/easydrag-template-field.js`
- Modify: `ui/js/desktop/apps/easydrag-fields.js`:
  - :1-3 comment;
  - :14 `let fieldCount`;
  - :18-29 `nameAttrs`, `withChange`;
  - :35-51 `composing`, `trimTrailingDots`;
  - :52-305 template field section;
  - :654 and :715 `++fieldCount`.
- Modify: `ui/js/desktop/core/module-loader.js` :295-296 (between palette and fields)
- Modify: `ui/desktop_easydrag_test.go` :13 and `easyDragSplitSeams`
- Modify: `scripts/test-easydrag-extra.mjs` :10 (plus a guard before :675), `scripts/test-easydrag-extra2.mjs` :9, `scripts/test-easydrag-extra3.mjs` :10, `scripts/test-easydrag-extra4.mjs` :12

**Module-level names the moved code reads:**

| fields.js name | where it goes |
|---|---|
| `nameAttrs` :20 | template-field.js; fields.js (`segmented`, `selectBox`) imports it from `ED.fieldsShared` |
| `withChange` :28 | template-field.js; shared, because fields.js (:439, :521, :619, :697, :807) and conditions (:363, :368) use it |
| `composing` :36 | template-field.js; shared, because `jsonEditor` (:777) uses it |
| `fieldCount` :14 | template-field.js. `templateField` keeps `++fieldCount` (:81). `keyValue` (:654) and `jsonEditor` (:715) switch to `nextFieldId()`, so one counter keeps today's id numbers. |
| `trimTrailingDots` :41 | template-field.js; only `templateField` uses it |
| `core()` :16 | redefined in the new file |
| `ED.template` | global, used at call time |

Exports:
- `ED.fieldsShared = { nameAttrs, withChange, composing, nextFieldId }`
- `ED.fieldsTemplate = { templateField, chipsMarkup, sampleAt }`

fields.js destructures both at load, so its `ED.fields` export line (:1009) stays byte for byte the same.

- [ ] **Step 1: Impact.** Run `IMPACT 'nameAttrs','withChange','composing','trimTrailingDots','chipsMarkup','templateField','sampleAt','keyValue','jsonEditor'`.
  - Expected public callers, only through `ED.fields`: forms.js :73 / :173-214 / :269, detail.js :403 / :455, block-fields.js :135, and the Node harnesses.
  - `nameAttrs`, `withChange`, `composing` and `trimTrailingDots` are IIFE-private.
  - Confirm `UNKNOWN` with Grep `ED\.fields\b` over `ui/js` and `scripts`. Expected: only the call sites listed above, plus extra/extra4/extra5/kv-editor/ux-batch5.
  - Report HIGH or CRITICAL for `templateField` before editing. The mitigation: the facade stays unchanged.

- [ ] **Step 2: Failing test and guard.**
  - In `easyDragModules` :13, change `"easydrag-palette.js", "easydrag-fields.js",` to `"easydrag-palette.js", "easydrag-template-field.js", "easydrag-fields.js",`.
  - Append to `easyDragSplitSeams`:
```go
	{"easydrag-template-field.js", []string{"function templateField(", "function chipsMarkup(", "function sampleAt(", "function trimTrailingDots(", "function nameAttrs(", "function withChange(", "ED.fieldsShared = { nameAttrs, withChange, composing, nextFieldId };", "ED.fieldsTemplate = { templateField, chipsMarkup, sampleAt };"}, nil},
	{"easydrag-fields.js", []string{"= ED.fieldsShared;", "= ED.fieldsTemplate;", "ED.fields = { templateField, chipsMarkup, sampleAt, segmented, toggle, selectBox, conditionGroup, cases, fieldList, decisionQuestions, decisionImages, keyValue, jsonEditor, tags, dateTime, fileField, secretRef, OPS, UNARY };"}, []string{"function templateField(", "function nameAttrs(", "function withChange(", "function composing(", "let fieldCount"}},
```
  - Add a guard in `scripts/test-easydrag-extra.mjs`, right before `await guardAsync('c1d05 sample lookups'` (:675). It passes before the split and must still pass after:
```js
    await guardAsync('s0 ED.fields merges its parts', async () => {
        const F = harness().ED.fields;
        const names = ['templateField', 'chipsMarkup', 'sampleAt', 'segmented', 'toggle', 'selectBox', 'conditionGroup', 'cases', 'fieldList', 'decisionQuestions', 'decisionImages', 'keyValue', 'jsonEditor', 'tags', 'dateTime', 'fileField', 'secretRef', 'OPS', 'UNARY'];
        eq('s0 ED.fields keeps its 19 exports, none missing', [Object.keys(F).sort(), names.filter(name => F[name] == null)], [names.slice().sort(), []]);
    });
```

- [ ] **Step 3: Run it and see it fail.** Run `GO-STATIC`. Expected FAIL: `easydrag assets miss easydrag-template-field.js`, plus the read failures. Run `NODE`. Expected: `ok   s0 ED.fields keeps its 19 exports, none missing` and `all EasyDrag checks passed`, because the guard is green before the move.

- [ ] **Step 4: Create `ui/js/desktop/apps/easydrag-template-field.js`.**
```js
// EasyDrag template field: a text with {{…}} references shown as chips, edited with {{
// autocomplete and caret-aware drops, plus the sample lookup its suggestions use. It loads
// before easydrag-fields-conditions.js and easydrag-fields.js and defines ED.fieldsShared,
// the helpers all three share.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // fieldCount numbers the element ids of every parameter editor (suggestion lists here, key-value
    // and JSON errors in easydrag-fields.js): one counter, so the ids stay as before the split.
    let fieldCount = 0;

    function core() { return ED.core; }

    // >>> fields.js lines 18-24 (nameAttrs) <<<

    // >>> fields.js lines 26-28 (withChange) <<<

    // >>> fields.js lines 35-36 (composing) <<<

    // nextFieldId returns the next number for a parameter editor's element ids.
    function nextFieldId() { return ++fieldCount; }

    ED.fieldsShared = { nameAttrs, withChange, composing, nextFieldId };

    // >>> fields.js lines 38-50 (trimTrailingDots) <<<

    // >>> fields.js lines 52-304 (template field section: chipsMarkup, templateField, sampleAt) <<<

    ED.fieldsTemplate = { templateField, chipsMarkup, sampleAt };
})();
```

- [ ] **Step 5: Move the code.** Work bottom-up in fields.js.
  1. :715: change `const errorId = (id || 'ed-json-' + (++fieldCount)) + '-error';` to `const errorId = (id || 'ed-json-' + nextFieldId()) + '-error';`.
  2. :654: change `const errorId = 'ed-kv-error-' + (++fieldCount);` to `const errorId = 'ed-kv-error-' + nextFieldId();`.
  3. Cut :52-304 verbatim (from `// ── template field ───…` through sampleAt's closing `}`) to its marker. Delete the blank line :305.
  4. Cut :38-50 (`trimTrailingDots` with its comment) to its marker. Delete :51.
  5. Cut :35-36 (`composing`) to its marker. Delete :37.
  6. Cut :26-28 (`withChange`) to its marker. Delete :29.
  7. Cut :18-24 (`nameAttrs`) to its marker. Delete :25.
  8. Delete :14 `let fieldCount = 0;`.
  9. After `const SECRET_MAX_BYTES = 4096;` (:13), insert:
```js
    // The template field and the helpers it shares load first (easydrag-template-field.js).
    const { nameAttrs, withChange, composing, nextFieldId } = ED.fieldsShared;
    const { templateField, chipsMarkup, sampleAt } = ED.fieldsTemplate;
```
  10. Replace the comment at :1-3 with:
```js
// EasyDrag parameter editors: the simple controls (segmented, switch, select), the condition
// group and switch cases, and the complex editors (field lists, decisions, key-value, JSON, tags,
// file, secrets, date and time). The template field is easydrag-template-field.js; ED.fields
// re-exports it. Every editor calls env.change(value) on edits.
```

- [ ] **Step 6: Register.**
  - In `module-loader.js`, add `'/js/desktop/apps/easydrag-template-field.js',` between the palette and fields lines.
  - Insert `'easydrag-template-field.js', ` directly before `'easydrag-fields.js'` in `extra.mjs` :10, `extra2.mjs` :9, `extra3.mjs` :10 and `extra4.mjs` :12.
  - Apply the baseline-script check, with `easydrag-fields.js` as the anchor.

- [ ] **Step 7: Self-check.**
  - Grep `\b(objects|blankRow|segmented|selectBox|OPS|UNARY|SECRET_MAX_BYTES)\b` in easydrag-template-field.js. Expected: no match.
  - Grep `fieldCount|function (templateField|chipsMarkup|sampleAt|nameAttrs|withChange|composing|trimTrailingDots)\(` in easydrag-fields.js. Expected: no match.
  - `(Get-Content ui/js/desktop/apps/easydrag-fields.js).Length`. Expected about 729.

- [ ] **Step 8: Run the tests.** `GO-STATIC`, `NODE` (the guard and the c1d05 template-field checks pass), `BASELINE`, `BROWSER`. All pass.

- [ ] **Step 9: Commit.** Run `DETECT`, then:
```
git add ui/js/desktop/apps/easydrag-template-field.js ui/js/desktop/apps/easydrag-fields.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
git commit -m "refactor(easydrag): move the template field into easydrag-template-field.js with ED.fieldsShared" -m "S0.1: chipsMarkup, templateField, sampleAt, trimTrailingDots and withChange move unchanged; nameAttrs, withChange, composing and one shared id counter (nextFieldId) live on ED.fieldsShared. ED.fields keeps its export line; a Node guard checks its 19 exports." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2.4: easydrag-fields-conditions.js; ED.fields becomes a merge

**Files:**
- Create: `ui/js/desktop/apps/easydrag-fields-conditions.js`
- Modify: `ui/js/desktop/apps/easydrag-fields.js`:
  - the comment written in 2.3;
  - :9-11 `OPS`, `UNARY`, `COND_TYPES`;
  - :30-31 `objects`;
  - :33 `blankRow`;
  - :341-419 condition group and cases.
- Modify: `ui/js/desktop/core/module-loader.js` (after template-field)
- Modify: `ui/desktop_easydrag_test.go` :13 and `easyDragSplitSeams`
- Modify: `scripts/test-easydrag-extra.mjs` :10, `extra2.mjs` :9, `extra3.mjs` :10, `extra4.mjs` :12

**Names the moved code (:341-418) reads:**

| name | how conditions.js gets it |
|---|---|
| `OPS`, `UNARY`, `COND_TYPES`, `blankRow` | move in |
| `objects` | moves in. It is exported as `ED.fieldsConditions.objects`, because fields.js still calls it at :424, :477, :525, :582, :590 and :599. |
| `withChange` | `ED.fieldsShared`, at load |
| `templateField` | `ED.fieldsTemplate`, at load |
| `segmented`, `selectBox` | stay in fields.js, which loads later. Local wrappers look them up on `ED.fields` when a condition editor is built. |
| `core()` | redefined |

Export: `ED.fieldsConditions = { conditionGroup, cases, objects, OPS, UNARY }`. fields.js destructures it, so the `ED.fields` line is unchanged and is now the merge of the three parts.

- [ ] **Step 1: Impact.** Run `IMPACT 'conditionGroup','cases','objects','blankRow'`.
  - Expected: `conditionGroup` and `cases` are reached through `ED.fields` from forms.js :202 / :204 and harnesses; `objects` and `blankRow` are private.
  - Confirm `UNKNOWN` with Grep `\b(conditionGroup|cases|OPS|UNARY)\b` in `ui/js/desktop/apps/easydrag*.js`. Expected: fields.js and forms.js only.
  - Report HIGH or CRITICAL first.

- [ ] **Step 2: Failing test.**
  - In `easyDragModules`, change `"easydrag-template-field.js", "easydrag-fields.js",` to `"easydrag-template-field.js", "easydrag-fields-conditions.js", "easydrag-fields.js",`.
  - Append to `easyDragSplitSeams`:
```go
	{"easydrag-fields-conditions.js", []string{"function conditionGroup(", "function cases(", "function objects(", "function blankRow(", "const COND_TYPES", "ED.fieldsConditions = { conditionGroup, cases, objects, OPS, UNARY };"}, nil},
```
  - Replace the `easydrag-fields.js` row with:
```go
	{"easydrag-fields.js", []string{"= ED.fieldsShared;", "= ED.fieldsTemplate;", "= ED.fieldsConditions;", "ED.fields = { templateField, chipsMarkup, sampleAt, segmented, toggle, selectBox, conditionGroup, cases, fieldList, decisionQuestions, decisionImages, keyValue, jsonEditor, tags, dateTime, fileField, secretRef, OPS, UNARY };"}, []string{"function templateField(", "function nameAttrs(", "function withChange(", "function composing(", "let fieldCount", "function conditionGroup(", "function cases(", "function objects(", "function blankRow(", "const OPS ="}},
```

- [ ] **Step 3: Run it and see it fail.** Run `GO-STATIC`. Expected FAIL: `easydrag assets miss easydrag-fields-conditions.js`, plus the read failures.

- [ ] **Step 4: Create `ui/js/desktop/apps/easydrag-fields-conditions.js`.**
```js
// EasyDrag condition editors: the condition group (all or any of rows with left side, operator,
// right side and type) and the switch cases built on it. Loads after easydrag-template-field.js
// and before easydrag-fields.js, which re-exports both through ED.fields.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // >>> fields.js lines 9-11 (OPS, UNARY, COND_TYPES) <<<

    const { withChange } = ED.fieldsShared;
    const { templateField } = ED.fieldsTemplate;

    function core() { return ED.core; }

    // segmented and selectBox stay in easydrag-fields.js, which loads after this file; a condition
    // editor looks them up when it is built.
    function segmented(env, options, value, onChange, aria) { return ED.fields.segmented(env, options, value, onChange, aria); }
    function selectBox(env, options, value, onChange, id, aria) { return ED.fields.selectBox(env, options, value, onChange, id, aria); }

    // >>> fields.js lines 30-31 (objects) <<<

    // >>> fields.js line 33 (blankRow) <<<

    // >>> fields.js lines 341-418 (condition group and cases) <<<

    ED.fieldsConditions = { conditionGroup, cases, objects, OPS, UNARY };
})();
```

- [ ] **Step 5: Move the code.** Work bottom-up in fields.js, finding each block by its anchor text.
  1. Cut the block from `// ── condition group ───…` through the closing `}` of `function cases(` to its marker, then delete the blank line after it.
  2. Cut `function blankRow() {…}` to its marker and delete the blank line after it.
  3. Cut the `// objects keeps the object items…` comment and `function objects(list) {…}` to their marker, and delete the blank line after them.
  4. Cut the three `const OPS` / `const UNARY` / `const COND_TYPES` lines to their marker.
  5. Under the two import lines from 2.3, add:
```js
    const { conditionGroup, cases, objects, OPS, UNARY } = ED.fieldsConditions;
```
  6. Change that import block's comment to `// The template field, the shared helpers and the condition editors load first; ED.fields below merges them.`
  7. Replace the file's top comment with:
```js
// EasyDrag parameter editors: the simple controls (segmented, switch, select) and the complex
// editors (field lists, decisions, key-value, JSON, tags, file, secrets, date and time).
// ED.fields merges them with the template field (easydrag-template-field.js) and the condition
// group and cases (easydrag-fields-conditions.js), which load first. Every editor calls
// env.change(value) on edits.
```

- [ ] **Step 6: Register.**
  - In `module-loader.js`, add `'/js/desktop/apps/easydrag-fields-conditions.js',` between the template-field and fields lines.
  - In `extra.mjs` :10, `extra2.mjs` :9, `extra3.mjs` :10 and `extra4.mjs` :12, change `'easydrag-template-field.js', 'easydrag-fields.js'` to `'easydrag-template-field.js', 'easydrag-fields-conditions.js', 'easydrag-fields.js'`.
  - Apply the baseline-script check, with `easydrag-fields.js` as the anchor.

- [ ] **Step 7: Self-check.**
  - Grep `\b(fieldList|keyValue|jsonEditor|nameAttrs|composing|SECRET_MAX_BYTES)\b` in easydrag-fields-conditions.js. Expected: no match.
  - Grep `function (conditionGroup|cases|objects|blankRow)\(|const (OPS|UNARY|COND_TYPES) =` in easydrag-fields.js. Expected: no match.
  - `(Get-Content ui/js/desktop/apps/easydrag-fields.js).Length`. Expected about 645.

- [ ] **Step 8: Run the tests.** `GO-STATIC`, `NODE` (the s0 facade guard and the detail and forms checks pass), `BASELINE`, `BROWSER`. All pass.

- [ ] **Step 9: Commit.** Run `DETECT`, then:
```
git add ui/js/desktop/apps/easydrag-fields-conditions.js ui/js/desktop/apps/easydrag-fields.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs
git commit -m "refactor(easydrag): move the condition group and cases into easydrag-fields-conditions.js" -m "S0.1: conditionGroup, cases, OPS, UNARY, COND_TYPES, objects and blankRow move unchanged; segmented and selectBox are looked up on ED.fields when a condition editor is built. ED.fields is now the merge of easydrag-template-field.js, easydrag-fields-conditions.js and easydrag-fields.js, with an unchanged export line." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2.5: Headroom ratchet

**Files:** Modify `ui/desktop_easydrag_test.go`: the budget loop at :98-106 inside `TestEasyDragAppLifecycle`, plus new declarations above `TestEasyDragAppLifecycle` (:80).

The large-module list holds the modules above 900 lines after Part 1 and Tasks 2.1-2.4:

| module | lines at 3da58db83 |
|---|---|
| easydrag-canvas.js | 923, plus Part 1's counters |
| easydrag-model.js | 921 |
| easydrag-template.js | 911 |

Every other module is at 900 or below. detail.js is the closest at 878.

"Warn at 900" is read as "above 900": a module with exactly 900 lines meets the acceptance limit ("at most 900") and needs no list entry.

- [ ] **Step 1: Impact.** Run `IMPACT 'TestEasyDragAppLifecycle'`. Expected: no upstream callers, because it is a test entry point.

- [ ] **Step 2: Failing test.** Add above `func TestEasyDragAppLifecycle`. The list starts **empty** on purpose, for this step:
```go
// The EasyDrag headroom ratchet (S0.1). A module above easyDragHeadroomLines is logged as a
// warning and must be on easyDragLargeModules; easyDragLineBudget fails every module;
// easydrag-runs.js, split in S0.1, stays at most easyDragRunsLines.
const (
	easyDragHeadroomLines = 900
	easyDragLineBudget    = 1100
	easyDragRunsLines     = 400
)

// easyDragLargeModules are the modules above the headroom line when the ratchet was set (S0,
// 2026-11). The list only shrinks: no module joins it, and a listed module that falls to the
// line or below fails the test until it is taken off.
var easyDragLargeModules = map[string]bool{}
```
  Replace the loop at :98-106 with:
```go
	seen := map[string]bool{}
	for _, name := range easyDragModules {
		seen[name] = true
		text := readDesktopAssetText(t, "js/desktop/apps/"+name)
		lines := strings.Count(text, "\n")
		switch {
		case lines >= easyDragLineBudget:
			t.Errorf("%s has %d lines, the desktop budget is %d", name, lines, easyDragLineBudget)
		case lines > easyDragHeadroomLines && !easyDragLargeModules[name]:
			t.Errorf("%s has %d lines, above the %d-line headroom line: split it by responsibility (easyDragLargeModules only shrinks)", name, lines, easyDragHeadroomLines)
		case lines > easyDragHeadroomLines:
			t.Logf("warning: %s has %d lines, above the %d-line headroom line (budget %d)", name, lines, easyDragHeadroomLines, easyDragLineBudget)
		case easyDragLargeModules[name]:
			t.Errorf("%s has %d lines, at or below the %d-line headroom line: take it off easyDragLargeModules", name, lines, easyDragHeadroomLines)
		}
		if name == "easydrag-runs.js" && lines > easyDragRunsLines {
			t.Errorf("easydrag-runs.js has %d lines, at most %d after the S0.1 split", lines, easyDragRunsLines)
		}
		if !strings.HasPrefix(strings.TrimSpace(text), "//") || !strings.Contains(text, "'use strict';") {
			t.Errorf("%s must start with a purpose comment and use strict mode", name)
		}
	}
	for name := range easyDragLargeModules {
		if !seen[name] {
			t.Errorf("easyDragLargeModules lists %s, which is no EasyDrag module", name)
		}
	}
```

- [ ] **Step 3: Run it and see it fail.** Run `go test ./ui/ -run '(?i)(TestEasyDragAppLifecycle)' -count=1 -v`.
  - Expected FAIL with exactly three "above the 900-line headroom line" errors: easydrag-canvas.js, easydrag-model.js and easydrag-template.js.
  - Stop if anything else is reported: editor, home, fields or a run part above 900, or runs.js above 400. That means an earlier task or Part 1 is incomplete.

- [ ] **Step 4: Fill in the list.**
```go
var easyDragLargeModules = map[string]bool{
	"easydrag-canvas.js":   true,
	"easydrag-model.js":    true,
	"easydrag-template.js": true,
}
```

- [ ] **Step 5: Run the tests.**
  - Run Step 3's command again. Expected: `--- PASS: TestEasyDragAppLifecycle`, with three `warning:` log lines (canvas, model, template).
  - Then run `GO-STATIC`, `NODE` and `BASELINE`. All pass. Browser tests are not needed, because this task changes Go test code only.

- [ ] **Step 6: Commit.** Run `DETECT`, then:
```
git add ui/desktop_easydrag_test.go
git commit -m "test(easydrag): headroom ratchet for EasyDrag modules" -m "S0.1: a module above 900 lines is logged and must be on easyDragLargeModules (canvas, model, template; the list only shrinks), every module fails at 1,100 lines, and easydrag-runs.js fails above 400." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Part 2 coverage

| Spec item | Task |
|---|---|
| §4 editor.js: header and footer :136-335 into easydrag-header.js; context; renderBanner; markers :92-97 stay | 2.1 |
| §4 home.js: badges / visibleFlows :378-410, error and empty cards :430-533, cardName :527, render :534-588; callbacks object; `ED.home` unchanged | 2.2 (with the callbacks deviation) |
| §4 fields.js: easydrag-template-field.js :52-305 plus withChange :28; `ED.fieldsShared` (nameAttrs) in the first-loaded part | 2.3 |
| §4 fields.js: easydrag-fields-conditions.js :341-419, OPS/UNARY/COND_TYPES, objects, blankRow; load order shared → template-field → conditions → fields; `ED.fields` merge; callers forms/detail/block-fields | 2.4 |
| §4 headroom ratchet (warn at 900, fail at 1,100, shrinking list, runs.js ≤ 400) | 2.5 |
| §4 rules: one commit per module, impact before each moved function, `//` plus `'use strict'`, two registrations, no `build:ui` | Steps 1, 4, 6 and 9 of 2.1-2.4 |
| §4 PLAN-CHECK, hard-coded script lists | Settled in the preamble; Step 6 of each task |
| §4 tests: existing tests pass with only load lists changed; baseline unchanged after each split | Step 8 of 2.1-2.4; the added tests are the seam test and the facade guard only |
| §4 / §14 acceptance: editor, home and fields ≤ 900; facades unchanged; new modules registered | Step 7 line counts; `TestEasyDragSplitSeams`; 2.5 (4 of the 8 new modules are here, the other 4 are in Part 1) |
| §9 commit order: header, home-grid, template-field, fields-conditions, ratchet | 2.1 → 2.5 |
| §10: browser tests only with `-run '(?i)(EasyDrag)'` and `-timeout 30m`; baseline at every commit | `BROWSER`, `BASELINE` |

### Notes for the other parts

1. **Part 1, registries.** Insert the four run parts directly before `easydrag-runs.js` in all of these:
   - module-loader.js;
   - `easyDragModules`;
   - `scripts/test-easydrag-extra3.mjs` :11;
   - `scripts/test-easydrag-extra2.mjs` :10;
   - `scripts/test-easydrag-extra4.mjs` :12. extra2 and extra4 also load runs.js.

   Part 2's edits anchor on the neighbouring names, not on list positions.
2. **Part 1, pinned strings.** `scripts/test-easydrag-extra5.mjs:621-622` pins `'aurago.easydrag.effects-ok.'` and `'aurago.easydrag.test-trigger.'` in `easydrag-runs.js` with `src('easydrag-runs.js').includes(...)`. If `storedEffects` (:389) or the test-trigger key moves into easydrag-run-start.js, Part 1 must update that pin.
3. **Part 1, baseline script.** Ideally `test-easydrag-render-baseline.mjs` takes its sandbox from test-easydrag-extra3.mjs instead of keeping its own list. Then Part 2's list edits carry over and the conditional baseline step in each task does nothing.
4. **Before Task 2.5:** runs.js must be at 400 lines or fewer, and each run part at 900 or fewer.
5. **Parts for S0.2-S0.5, ratchet.** After 2.5, no module may cross 900 lines unless it is canvas.js, model.js or template.js, and those stay below 1,100. Headroom today:
   - detail.js: 22 lines;
   - interact.js: about 165 lines;
   - canvas.js: about 170 lines before 1,100 (for S0.4).

   The list may only shrink. A new module above 900 needs a split, not a list entry.
6. **Moved code that later parts touch:**
   - **Header chips** (state, live-outdated, timers, paused), the Publish button's `has-changes` / `ed-btn--primary` toggles and the save chip are now in `easydrag-header.js`. That is relevant to S0.5's status colours and the Publish dot PLAN-CHECK.
   - **Template-card category dots** (`ed-cat-dot data-cat`) are now in `easydrag-home-grid.js`. The flow-card mini preview (`previewSVG`, `rect data-cat`) stays in home.js.
   - **The `cat-` class on template chips** is now in `easydrag-template-field.js`.
   - Any lint or key scan that walks `easyDragModules` covers the new files.
   - New page state the grid reads must be added to home.js's `state: () => ({…})` object. The grid only sees the snapshot.
7. **New namespaces:**
   - `ED.header` (create);
   - `ED.homeGrid` (create);
   - `ED.fieldsShared` {nameAttrs, withChange, composing, nextFieldId};
   - `ED.fieldsTemplate` {templateField, chipsMarkup, sampleAt};
   - `ED.fieldsConditions` {conditionGroup, cases, objects, OPS, UNARY}.

   Wave 5's PE-15 condition work edits easydrag-fields-conditions.js.
8. **Spec deviations, for the assembler and reviewer:**
   - (a) The header context: `draftModel` is unused. t, esc and ctx come from `ed`. Added: canvas, nameInput, renderBanner, isDisposed() and lastRecord().
   - (b) Direction: `renderHeader` calls `renderBanner`, not the reverse. The banner stays in the editor and is passed in.
   - (c) home-grid's callbacks are what render calls (library, organization and selection controls, createOff, blockReason, metadataOf, metadataMarkup), not the click-handler targets.
   - (d) The home state lets and the auth helpers (:433-469) stay in home.js; only `spoken` moves.
   - (e) `objects` lives in conditions but is exported for fields.js. withChange, composing and the id counter are shared through `ED.fieldsShared`.
9. **detect-changes in the worktree is unverified.** The `DETECT` form (`--repo .` from the worktree root) should be confirmed once at Part 1's first commit and then used the same way in every part.
10. **Review.** Spec §10 asks for one combined S0.1 review, Critical and Important only. It belongs after Task 2.5.
