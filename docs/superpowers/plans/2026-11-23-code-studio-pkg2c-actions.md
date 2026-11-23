# Code Studio package 2c: LSP actions — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the language server actions to the Code Studio editor:
- **Find references** (Shift+F12). `textDocument/references` fills a new **References** tab in the bottom panel. Results are grouped by file and show line previews. A click opens the location.
- **Rename symbol** (F2 in the editor; F2 in the file tree still renames files). A small box at the symbol collects the new name. Then `textDocument/rename` runs, the preview dialog lists files and change counts, and the edit is applied.
- **Quick fixes** (Ctrl+. and a lightbulb in the gutter on lines with LSP problems). `textDocument/codeAction` uses the line's diagnostics. A keyboard menu at the cursor lists the actions.
  - An edit that stays in the current file applies directly as one undo step.
  - An edit that touches other files goes through the preview dialog.
  - A command goes to `workspace/executeCommand`.
- **One WorkspaceEdit applier** for rename, quick fixes and server-sent `workspace/applyEdit`:
  - Open tabs change in their buffer. The change is unsaved and undoable, and split views get one dispatch.
  - Closed files are read, changed and written with `PUT /api/code-studio/file`, then `lspNotifyFilesChanged` runs.
  - If some files fail, the dialog lists them. The rest stays applied.
  - Read-only mode refuses all edits.

There is no backend change and no Go change.

**Spec (source of truth, decisions are final):** `docs/superpowers/specs/2026-11-23-code-studio-ide-package-2-lsp-design.md`, section 3 "Features and keys", "Error handling" and "Testing".

**Architecture:**
1. `ui/js/desktop/apps/code-studio/lsp-edits.js`: a pure UMD module with the global `CodeStudioLSPEdits`. It has no DOM and no studio state. It turns a WorkspaceEdit into per-file change lists through `CodeStudioLSPProtocol.pathForUri`. It applies LSP TextEdits to text: UTF-16 positions, sorted, applied from the end, overlaps rejected. It also groups reference locations and builds line previews. Node test: `lsp-edits.test.cjs`.
2. `ui/js/desktop/apps/code-studio/lsp-actions.js`: a fragment in the shared Code Studio IIFE. It holds:
   - the applier and the preview/result dialog
   - the References tab, the rename box, the quick-fix menu and the lightbulb gutter
   - one entry in `lspActionExtensionFactories` (keymap plus gutter)
   - one `lspOnServerRequest('workspace/applyEdit', ...)` handler
3. Integration:
   - `panel.js` learns the `references` view. Its tab joins the panel on first use, so the panel keeps its three standard tabs until then.
   - `shortcuts.js` lists the keys and exposes three palette commands.
   - `command-palette.js` shows those commands.
   - `scripts/build-ui-bundles.js` adds both files after `lsp.js`.

**Tech stack:** Vanilla JS fragments concatenated by `scripts/build-ui-bundles.js`, CodeMirror 6 (`keymap`, `Prec`, `gutter`, `GutterMarker`, `isolateHistory`, transactions with `sequential`), `@codemirror/lsp-client` 6.3.1 through plan 2B's wrappers, Node `node:test` + `node:vm`, flat i18n JSON (16 desktop locales).

---

## Conventions for this plan (repo rules)

- **Shared checkout.** Other agent sessions commit in the same checkout.
  - Never run `git add` or `git commit` in the shared index.
  - Commit each task with `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh <message-file> <repo-relative paths...>`. Run it with the Bash tool. It commits through a temporary index, prints `committed <sha> (parent <sha>)`, then syncs only those paths in the shared index.
  - The generated `ui/js/desktop/bundles/code-studio.bundle.js` is committed together with its sources.
  - Commit messages end with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Message files go to `C:\disposable\code-studio-pkg2c\` (the disposable folder, never the repo).
  - Nothing is pushed.
- **Foreign edits.** Before editing shared files, and always before `npm run build:ui`, run `git status --short -- <paths>` as stated in each task.
  - If a path in scope shows changes that are not yours, stop and report.
  - `npm run build:ui` rebuilds every bundle. Commit only `ui/js/desktop/bundles/code-studio.bundle.js`. Do not commit or revert other generated bundles; they come from other sessions' sources.
- **GitNexus (repo CLAUDE.md).**
  - Before editing an existing function, run impact: MCP `impact({target: "<symbol>", direction: "upstream"})`, or from the repo root `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`. Report callers, processes and risk, and warn on HIGH or CRITICAL.
  - `risk: UNKNOWN` is unresolved. Confirm callers with Grep; each task lists the callers it expects.
  - Before every commit, run MCP `detect_changes({scope: "all"})` or `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .`. A `partial: true` or `truncated: true` result is not a clean check, so re-run it. Other sessions' uncommitted files will appear; report them, do not touch them.
- **Write tool and `\u`.** The Write tool decodes `\u` escapes. The code below contains none: it uses `String.fromCharCode` / `String.fromCodePoint` where an escape would be needed, and literal UTF-8 for `…`, `▸`, `▾` and the translations. Copy it as it stands.
- **Ownership.**
  - Do not touch Go files, `lsp.js` or `lsp-protocol.js`; plan 2B owns them. The contract names below are the only coupling.
  - `problems.js` is edited only by the conditional Task 6 Step 6.
- **Anchors decide, line numbers do not.** Plan 2B lands before this plan and may have touched `shortcuts.js`, `command-palette.js` or `problems.js`. If an anchor is not found verbatim, put the new code at the same logical place and say so in your report.
- **Shell.**
  - Windows. PowerShell for `node`, `npm` and `go`; Bash only for `commit-paths.sh` and the one CSS check.
  - If `go test` fails with `no such tool "vet"`, prefix `$env:GOTOOLCHAIN='go1.27.4';`.
  - Browser tests start Chrome. Run them with the PowerShell tool, sandbox off, and `$env:GOTMPDIR='D:\tmp\go-build'` (repo-built binaries elsewhere cannot start Chrome).
  - Before a browser run, check for leftover headless rod Chromes (`Temp\rod\user-data`) and ask the user before stopping any.
- **Tests are lean.** The only new test file is `lsp-edits.test.cjs`. This plan adds no browser test; the package's single browser test belongs to 2B. Task 6 runs existing tests.

## The 2B → 2C contract this plan consumes

Code Studio modules are concatenated into one closure, so these are plain functions visible to later files.

| From 2B | Used for |
| --- | --- |
| `CodeStudioLSPProtocol.languageForPath(path)` | which server a tab belongs to |
| `CodeStudioLSPProtocol.uriForPath(path)` / `pathForUri(uri)` | request URIs; mapping edit and reference URIs to `/workspace/...` (null = refuse) |
| `CodeStudioLSPProtocol.offsetToPosition(doc, offset)` | request positions (UTF-16) |
| `lspEnsureClient(language)` → `Promise<LSPClient\|null>` | `client.serverCapabilities` (renameProvider/prepareProvider, codeActionProvider.resolveProvider) |
| `lspClientState(language)` | only act when `'ready'`, otherwise show "No language server is ready for this file." |
| `lspRequest(language, method, params, timeoutMs)` | every request, with `timeoutMs = 15000`; rejects `Error('timeout')` |
| `lspOnServerRequest(method, handler)` | `workspace/applyEdit` |
| `lspNotifyFilesChanged([{path, type}])` | after writing closed files (`type: 2`) |
| `lspActionExtensionFactories` | `push(cm => [keymap, lightbulb gutter])` at load time |
| `replaceProblemsForFile` (problems.js) | only checked for existence (2B landed) |

2C provides `CodeStudioLSPEdits` (lsp-edits.js). Its contract members are `workspaceEditToFileChanges(edit)` and `applyTextEdits(text, edits)`. It adds `textEditsToChanges`, `planTextEdits`, `referencesToGroups` and `referencePreview`.

## Contract notes (checked by Task 1; ask the controller to align plan 2B)

1. **Client capabilities.** Without these, servers send no `workspace/applyEdit`, no versioned `documentChanges`, no `prepareRename`, and only bare commands as code actions. lsp-client 6.3.1 declares none of them by default (`clientCapabilities` in dist/index.js). 2B must merge them into its `LSPClient` config, for example as an extension entry `{ clientCapabilities: { ... } }`:
   ```js
   {
       workspace: { applyEdit: true, workspaceEdit: { documentChanges: true } },
       textDocument: {
           rename: { prepareSupport: true },
           codeAction: {
               codeActionLiteralSupport: { codeActionKind: { valueSet: ['', 'quickfix', 'refactor', 'refactor.extract', 'refactor.inline', 'refactor.rewrite', 'source', 'source.organizeImports', 'source.fixAll'] } },
               isPreferredSupport: true,
               disabledSupport: true,
               dataSupport: true,
               resolveSupport: { properties: ['edit'] }
           }
       }
   }
   ```
   If they are missing, Task 1 stops: the fix belongs in `lsp.js`.
2. **Client timeout.** `LSPClient` cuts every request at `config.timeout` (default 3000 ms). For `lspRequest(..., 15000)` to mean 15 s, 2B must set `timeout` to at least 15000 and enforce shorter budgets in `lspRequest` itself. Task 1 checks this; it stops if the timeout is lower.
3. **Raw diagnostics for code actions.** `normalizeProblem` keeps only file/line/column/severity/message/source. gopls and clangd match quick fixes by the exact diagnostic range and message, and typescript-language-server needs the diagnostic `code`.
   - If 2B's `diagnosticsToProblems` attaches the server diagnostic as `problem.lsp`, this plan sends it unchanged.
   - Otherwise it rebuilds a diagnostic: start, word range, severity, message, and `code` if present. That works for fewer fixes.
   - Task 1 reports which case applies. Task 6 Step 6 makes `normalizeProblem` keep `problem.lsp` when 2B attaches it but `normalizeProblem` would drop it.
4. **Problem sources.** The spec fixes the LSP problem sources as `gopls`, `typescript`, `pyright` and `clangd` (`LSP_ACT_SOURCES`). Task 1 warns if 2B uses other names.

## Behaviour decisions (read before coding)

1. **Keys.**
   - Shift+F12, F2 and Mod-. are bound through `lspActionExtensionFactories`, so only in editors whose file has a language server. They use `Prec.high`.
   - The file tree's F2 is a row `keydown` handler (sidebar.js) and is not affected.
   - Every action runs inside the Code Studio window that owns the view. It is found by walking `instances` for a tab whose views include it, because keymaps and gutters run outside the studio's instance context.
2. **Requests.**
   - All requests use `lspRequest(language, method, params, 15000)`, called inside the owning window (`runWithInstance`).
   - After every `await`, work continues through `runWithInstance(target, ...)`.
   - A timeout (`'timeout'` or the client's "Request timed out") shows "The language server did not answer in time. Try again." Other errors show "Language server error: …". The editor stays usable.
3. **References.**
   - The References tab and pane are inserted into the bottom panel the first time they are needed. The existing browser test expects exactly three tabs at start.
   - Results are de-duplicated and grouped by path, sorted by path, line and character.
   - Line previews come from the open buffer, or from `GET /api/code-studio/file` for at most 60 files, 6 at a time.
   - Results outside `/workspace` (for example the Go standard library) are counted in a note, not listed.
   - A click opens the file and puts the cursor on the exact position.
   - The pane re-renders only when its data changes, because `renderBottomPanel` runs on every problems update.
4. **Rename.**
   - If `renameProvider` is false, show "This symbol cannot be renamed."
   - If `renameProvider.prepareProvider` is set, `textDocument/prepareRename` runs first. A null answer means the symbol cannot be renamed. A `Range` or `{range, placeholder}` sets the box text. `{defaultBehavior}` keeps the word at the cursor.
   - The box is prefilled and selected. Enter renames; Escape or a click elsewhere cancels.
   - The rename always shows the preview dialog.
5. **Applier.**
   - The plan comes from `CodeStudioLSPEdits.workspaceEditToFileChanges`. When `documentChanges` is present it wins over `changes`.
   - Create, rename and delete file operations are never executed. They are listed as "not applied", together with text edits for files those operations touch, URIs outside the workspace, and malformed edits.
   - **Preview:**
     - Always for rename.
     - For quick fixes and server edits, unless the edit is a single file equal to the file the user is in and nothing is skipped.
   - **Open tab with a live CodeMirror view:** `planTextEdits` turns each batch into ascending changes, and one `view.dispatch(...specs)` runs on `tab.view` (the history owner).
     - The specs carry `userEvent: 'input.lsp'`, `isolateHistory.of('full')` on the first spec, and `sequential: true` on later batches.
     - A linked split follows through `syncLinkedView`, so there is exactly one dispatch.
     - The change is unsaved and one undo step.
   - **Textarea fallback:** `setValue`.
   - **Open tab without a live view:** the change goes into `tab.editorSnapshot.state` plus `updateTabContent`. If that tab has unsaved changes it is refused ("unsaved changes in a hidden tab; save it and try again"), because the server may have seen the saved file.
   - **Stale check:** every open file's document is captured when the request goes out (`docsBefore`). A buffer that changed since then is skipped as "changed while the language server was working".
   - **Closed file:**
     - It is read with `GET`; text containing U+FFFD is skipped.
     - `planTextEdits` produces the new text, which is written with `apiClient.writeFile` (`PUT`).
     - A tab opened during the read takes the change in its buffer. A tab opened after the write is reloaded with `reloadCleanTabFromDisk`.
     - A 413 answer reuses "larger than the file size limit".
   - **After the loop:**
     - `lspNotifyFilesChanged(written.map(path => ({path, type: 2})))` and `scheduleGitRefresh()` run, then `renderTabs()` and `renderStatus()`.
     - Without failures, the dialog closes and the status bar flashes "Applied N changes in M files.". It adds the existing "open files … are not saved yet" note when more than one file changed and some were buffers.
     - With failures, the dialog (the preview one, or a new one) switches to a result view that lists the failed and skipped parts. The other files stay applied.
   - **Read-only:** refused with "Read-only: code changes are turned off." A second edit while one is running is refused (`lspActBusy`).
6. **Quick fixes.**
   - **Range:** the selection for Ctrl+., the whole line for the lightbulb.
   - **Diagnostics:** LSP-source problems of that line in LSP shape. Use `problem.lsp` when 2B keeps it, otherwise rebuild it (see contract note 3).
   - **Menu order:** preferred actions first, then quick fixes, other actions, refactorings and source actions. Disabled actions show greyed with their reason. Arrow keys, Home, End, Enter, Escape and Tab work.
   - **Lazy actions:** an action with `data` but no edit and no command is resolved with `codeAction/resolve` when the server supports it.
   - **Order:** the edit is applied first, then the command runs (protocol order).
   - **Commands:** a bare Command goes to `workspace/executeCommand`. A server `workspace/applyEdit` during the command lands in the applier. A timeout of the command while such an edit was previewed is not reported again.
7. **Server `workspace/applyEdit`.** The same applier runs, with `originPath` = the active file and the title = `params.label` or "Apply these changes?". The answer is `{applied: true}` or `{applied: false, failureReason}`.
8. **Names.** Every top-level name in lsp-actions.js starts with `lspAct` or `LSP_ACT_`, and instance fields are `lspActReferences`, `lspActBusy` and `lspActServerEdits`. A duplicate `const` in the shared closure would break the whole bundle, and 2B also uses the `lsp` prefix. Exceptions, checked against the other files by Task 1: `ensureReferencesPanelParts`, `renderReferencesPanel`, `clearReferences`, `lspFindReferencesCommand`, `lspRenameSymbolCommand`, `lspQuickFixCommand` and the global `CodeStudioLSPEdits`.
9. **Strings.**
   - 35 new keys go into all 16 locales, inserted after `"codeStudio.formatOutputSource"`.
   - Reused keys: `desktop.cancel`, `desktop.close`, `codeStudio.problemWorkspace`, `codeStudio.replaceSkippedTooLarge`, `codeStudio.replaceSkippedEncoding`, `codeStudio.replaceUnsavedNote`.
   - German uses informal address.

## File structure

| File | Change | Task |
| --- | --- | --- |
| `C:\disposable\code-studio-pkg2c\check-2b.mjs` | Create (outside the repo, not committed) | 1 |
| `ui/js/desktop/apps/code-studio/lsp-edits.js` | Create | 1 |
| `ui/js/desktop/apps/code-studio/lsp-edits.test.cjs` | Create | 1 |
| `C:\disposable\code-studio-pkg2c\locales-2c.mjs` | Create (not committed) | 2 |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | Modify (35 keys) | 2 |
| `ui/js/desktop/apps/code-studio/lsp-actions.js` | Create (Task 3), append (Task 4) | 3, 4 |
| `ui/css/code-studio.css` | Append `/* === IDE package 2C: LSP actions === */` | 5 |
| `ui/js/desktop/apps/code-studio/panel.js` | Modify (References view) | 6 |
| `ui/js/desktop/apps/code-studio/shortcuts.js` | Modify (shortcut sheet, command table) | 6 |
| `ui/js/desktop/apps/code-studio/command-palette.js` | Modify (3 commands) | 6 |
| `ui/js/desktop/apps/code-studio/problems.js` | Modify only if Task 1 says so (`normalizeProblem` keeps `lsp`) | 6 |
| `scripts/build-ui-bundles.js` | Modify (2 parts after `lsp.js`) | 6 |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Generated | 6 |

---

### Task 1: Check that plan 2B landed, then the pure edit helpers and their Node test

**Files:**
- Create (not committed): `C:\disposable\code-studio-pkg2c\check-2b.mjs`
- Create: `ui/js/desktop/apps/code-studio/lsp-edits.js`
- Create: `ui/js/desktop/apps/code-studio/lsp-edits.test.cjs`

- [ ] **Step 1: Check the shared tree is clean in scope**

Run (PowerShell, repo root): `git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/css/code-studio.css`
Expected: no output. If anything is listed, stop and report: plan 2B or another session has uncommitted work there.

- [ ] **Step 2: Create `C:\disposable\code-studio-pkg2c\check-2b.mjs` with this complete content**

```js
// Plan 2C, Task 1: is plan 2B (Code Studio LSP client core) on main with the names of the 2B -> 2C contract?
// Usage: node C:\disposable\code-studio-pkg2c\check-2b.mjs C:\Users\Andi\Documents\repo\AuraGo
// FAIL lines mean: stop and report to the controller. WARN lines name their follow-up.
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import vm from 'node:vm';

const repo = process.argv[2] || 'C:/Users/Andi/Documents/repo/AuraGo';
const dir = path.join(repo, 'ui', 'js', 'desktop', 'apps', 'code-studio');
let failures = 0;
const ok = text => console.log('OK   ' + text);
const warn = text => console.log('WARN ' + text);
const fail = text => { failures += 1; console.log('FAIL ' + text); };
const read = async file => { try { return await readFile(file, 'utf8'); } catch (_) { return null; } };
const declares = (source, name) => new RegExp('(?:(?:async\\s+)?function\\s+' + name + '\\s*\\(|(?:const|let|var)\\s+' + name + '\\s*=)').test(source || '');
const body = (source, name) => {
    const start = (source || '').indexOf('function ' + name + '(');
    if (start < 0) return '';
    const end = source.indexOf('\n    }\n', start);
    return end < 0 ? source.slice(start) : source.slice(start, end);
};

const lsp = await read(path.join(dir, 'lsp.js'));
const protocolSource = await read(path.join(dir, 'lsp-protocol.js'));
const problems = (await read(path.join(dir, 'problems.js'))) || '';
const bundles = (await read(path.join(repo, 'scripts', 'build-ui-bundles.js'))) || '';

// 1. Functions and the factory list of lsp.js.
if (lsp === null) {
    fail('ui/js/desktop/apps/code-studio/lsp.js is missing: plan 2B has not landed');
} else {
    for (const name of ['lspEnsureClient', 'lspClientState', 'lspRequest', 'lspOnServerRequest', 'lspNotifyFilesChanged', 'lspViewForPath', 'lspTouch']) {
        if (declares(lsp, name)) ok('lsp.js declares ' + name);
        else fail('lsp.js does not declare ' + name);
    }
    if (/(?:const|let|var)\s+lspActionExtensionFactories\s*=/.test(lsp)) ok('lsp.js declares lspActionExtensionFactories');
    else fail('lsp.js does not declare lspActionExtensionFactories');
    // 2. Client capabilities that rename, quick fixes and workspace/applyEdit need (plan 2C, contract note 1).
    const capabilities = [
        ['workspace.applyEdit', /applyEdit\s*:\s*true/],
        ['workspace.workspaceEdit.documentChanges', /documentChanges\s*:\s*true/],
        ['textDocument.rename.prepareSupport', /prepareSupport\s*:\s*true/],
        ['textDocument.codeAction.codeActionLiteralSupport', /codeActionLiteralSupport/]
    ];
    for (const [name, pattern] of capabilities) {
        if (pattern.test(lsp)) ok('lsp.js declares the client capability ' + name);
        else fail('lsp.js does not declare the client capability ' + name + ' (plan 2C contract note 1)');
    }
    // 3. The client timeout must not cut the 15 s requests (contract note 2).
    const timeout = /\btimeout\s*:\s*([0-9_]+)\b/.exec(lsp);
    if (timeout && Number(timeout[1].replace(/_/g, '')) >= 15000) ok('LSPClient timeout ' + timeout[1]);
    else warn('no literal LSPClient timeout >= 15000: read the LSPClient construction in lsp.js; stop and report if its timeout is below 15000');
}

// 4. The pure protocol module, loaded like the Node tests load it.
let protocol = null;
if (protocolSource === null) {
    fail('ui/js/desktop/apps/code-studio/lsp-protocol.js is missing: plan 2B has not landed');
} else {
    try {
        const mod = { exports: {} };
        const windowObject = {};
        vm.runInNewContext(protocolSource, { module: mod, exports: mod.exports, window: windowObject, console }, { filename: 'lsp-protocol.js' });
        protocol = mod.exports && typeof mod.exports.pathForUri === 'function' ? mod.exports : windowObject.CodeStudioLSPProtocol;
    } catch (err) {
        fail('lsp-protocol.js does not load under node: ' + err.message);
    }
    if (!protocol) fail('lsp-protocol.js exports no CodeStudioLSPProtocol');
    else {
        for (const name of ['languageForPath', 'uriForPath', 'pathForUri', 'diagnosticsToProblems', 'positionToOffset', 'offsetToPosition']) {
            if (typeof protocol[name] === 'function') ok('CodeStudioLSPProtocol.' + name);
            else fail('CodeStudioLSPProtocol.' + name + ' is missing');
        }
        if (typeof protocol.uriForPath === 'function' && typeof protocol.pathForUri === 'function') {
            const back = protocol.pathForUri(protocol.uriForPath('/workspace/src/main.go'));
            if (back === '/workspace/src/main.go') ok('uriForPath and pathForUri round-trip');
            else fail('pathForUri(uriForPath("/workspace/src/main.go")) gave ' + JSON.stringify(back));
        }
        if (typeof protocol.diagnosticsToProblems === 'function') {
            const sample = protocol.diagnosticsToProblems('/workspace/a.go', [{ range: { start: { line: 1, character: 2 }, end: { line: 1, character: 5 } }, severity: 1, message: 'boom', code: 'E1', source: 'compiler' }], 'gopls');
            console.log('INFO diagnosticsToProblems sample: ' + JSON.stringify(sample));
            const keepsRaw = Array.isArray(sample) && sample[0] && sample[0].lsp && sample[0].lsp.range;
            if (keepsRaw) ok('diagnosticsToProblems keeps the server diagnostic as problem.lsp');
            else warn('diagnosticsToProblems does not keep problem.lsp: quick fixes send rebuilt diagnostics (contract note 3); report it, not a stop');
            if (keepsRaw && body(problems, 'replaceProblemsForFile').includes('normalizeProblem') && !body(problems, 'normalizeProblem').includes('lsp')) {
                warn('normalizeProblem drops problem.lsp: apply Task 6 Step 6');
            }
        }
    }
}

// 5. problems.js and the bundle list.
if (declares(problems, 'replaceProblemsForFile')) ok('problems.js declares replaceProblemsForFile');
else fail('problems.js does not declare replaceProblemsForFile: plan 2B has not landed');
const studioStart = bundles.indexOf("out: 'ui/js/desktop/bundles/code-studio.bundle.js'");
const studioParts = studioStart < 0 ? '' : bundles.slice(studioStart, bundles.indexOf('\n    ]', studioStart));
const lspIndex = studioParts.indexOf("'ui/js/desktop/apps/code-studio/lsp.js'");
const shortcutsIndex = studioParts.indexOf("'ui/js/desktop/apps/code-studio/shortcuts.js'");
if (lspIndex < 0) fail('lsp.js is not a part of the code-studio bundle');
else if (shortcutsIndex >= 0 && lspIndex > shortcutsIndex) fail('lsp.js sits after shortcuts.js, outside the shared closure');
else ok('lsp.js is a code-studio bundle part inside the shared closure');
if (studioParts.includes('lsp-edits.js') || studioParts.includes('lsp-actions.js')) warn('lsp-edits.js or lsp-actions.js is already a bundle part: Task 6 Step 7 must not add it twice');

// 6. Problem source names (contract note 4).
for (const source of ['gopls', 'typescript', 'pyright', 'clangd']) {
    const quoted = new RegExp("['\"]" + source + "['\"]");
    if (quoted.test(lsp || '') || quoted.test(protocolSource || '')) ok('problem source ' + source + ' appears in lsp.js or lsp-protocol.js');
    else warn('problem source ' + source + ' not found: put the source names lsp.js passes to diagnosticsToProblems into LSP_ACT_SOURCES (Task 3 Step 1)');
}

// 7. The names plan 2C declares must be free in the other Code Studio files.
const reserved = /\b(?:function\s+|const\s+|let\s+|var\s+|class\s+)(lspAct\w*|LSP_ACT_\w*|ensureReferencesPanelParts|renderReferencesPanel|clearReferences|lspFindReferencesCommand|lspRenameSymbolCommand|lspQuickFixCommand|CodeStudioLSPEdits)\b/;
let clash = false;
for (const name of (await readdir(dir)).filter(file => file.endsWith('.js') && file !== 'lsp-edits.js' && file !== 'lsp-actions.js')) {
    const match = reserved.exec((await read(path.join(dir, name))) || '');
    if (match) {
        clash = true;
        fail(name + ' already declares ' + match[1] + ', a name plan 2C needs');
    }
}
if (!clash) ok('the plan 2C names are free in the other Code Studio files');

console.log(failures ? 'PLAN 2B CONTRACT: ' + failures + ' FAIL line(s) - stop and report' : 'PLAN 2B CONTRACT: OK');
process.exitCode = failures ? 1 : 0;
```

- [ ] **Step 3: Run the check**

Run: `node C:\disposable\code-studio-pkg2c\check-2b.mjs C:\Users\Andi\Documents\repo\AuraGo`
Expected: no `FAIL` line, and the last line is `PLAN 2B CONTRACT: OK`, exit code 0.
- Any `FAIL`: stop. Report the lines to the controller; plan 2B is not landed or misses a contract part.
- `WARN` about the timeout: read the `LSPClient` construction in `lsp.js`. Stop and report if its `timeout` is below 15000.
- `WARN` about a problem source: note 2B's names; Task 3 Step 1 uses them.
- `WARN normalizeProblem drops problem.lsp`: note it; Task 6 Step 6 applies.
- `WARN diagnosticsToProblems does not keep problem.lsp`: note it for the report. It is not a stop.

- [ ] **Step 4: Create `ui/js/desktop/apps/code-studio/lsp-edits.js` with this complete content**

```js
/* Code Studio IDE package 2C: the pure half of the language server actions. Nothing here touches the DOM or the
   studio state, so the Code Studio bundle and lsp-edits.test.cjs (node) load the same file. It turns an LSP
   WorkspaceEdit into per-file change lists, applies LSP TextEdits to plain text and groups reference locations for
   the References panel. Positions are LSP { line, character } with UTF-16 characters, the unit of JavaScript strings
   and of CodeMirror offsets. Paths come from CodeStudioLSPProtocol.pathForUri (lsp-protocol.js, plan 2B); nothing
   outside /workspace passes. */
(function (root, factory) {
    'use strict';
    const api = factory(() => {
        if (root && root.CodeStudioLSPProtocol) return root.CodeStudioLSPProtocol;
        if (typeof CodeStudioLSPProtocol !== 'undefined') return CodeStudioLSPProtocol;
        return null;
    });
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioLSPEdits = api;
})(typeof window !== 'undefined' ? window : null, getProtocol => {
    'use strict';

    const WORKSPACE_PREFIX = '/workspace/';

    function editError(code) {
        const error = new Error('LSP text edit rejected: ' + code);
        error.editError = code;
        return error;
    }

    // '/workspace/...' for a URI inside the workspace, null for anything else (other roots, empty, '.' or '..' parts).
    function workspacePath(uri) {
        if (typeof uri !== 'string' || !uri) return null;
        const protocol = getProtocol();
        const path = protocol && typeof protocol.pathForUri === 'function' ? protocol.pathForUri(uri) : null;
        if (typeof path !== 'string' || !path.startsWith(WORKSPACE_PREFIX)) return null;
        if (path.slice(1).split('/').some(part => part === '' || part === '.' || part === '..')) return null;
        return path;
    }

    // Offsets where lines start. LSP ends a line at \n, \r\n or a lone \r.
    function lineStarts(text) {
        const starts = [0];
        for (let index = 0; index < text.length; index += 1) {
            const code = text.charCodeAt(index);
            if (code === 13) {
                if (text.charCodeAt(index + 1) === 10) index += 1;
                starts.push(index + 1);
            } else if (code === 10) {
                starts.push(index + 1);
            }
        }
        return starts;
    }

    // Offset just before the line break of a line; the end of the text for the last line.
    function lineEnd(text, starts, line) {
        if (line + 1 >= starts.length) return text.length;
        let end = starts[line + 1];
        if (text.charCodeAt(end - 1) === 10) end -= 1;
        if (end > starts[line] && text.charCodeAt(end - 1) === 13) end -= 1;
        return end;
    }

    // LSP position to an offset. A character past the line end means the line end (LSP 3.17); the line after the
    // last one, at character 0, means the end of the text.
    function offsetAt(text, starts, position) {
        const line = position && position.line;
        const character = position && position.character;
        if (!Number.isInteger(line) || !Number.isInteger(character) || line < 0 || character < 0) throw editError('invalid');
        if (line >= starts.length) {
            if (line === starts.length && character === 0) return text.length;
            throw editError('range');
        }
        return Math.min(starts[line] + character, lineEnd(text, starts, line));
    }

    // TextEdits to CodeMirror style changes: ascending, offsets into the original text. Inserts at one position merge
    // in array order, and they may precede one replacement at the same start. Overlapping edits are rejected.
    function textEditsToChanges(text, edits) {
        const source = String(text == null ? '' : text);
        if (!Array.isArray(edits)) throw editError('invalid');
        const starts = lineStarts(source);
        const list = edits.map((edit, index) => {
            if (!edit || !edit.range || typeof edit.newText !== 'string') throw editError('invalid');
            const from = offsetAt(source, starts, edit.range.start);
            const to = offsetAt(source, starts, edit.range.end);
            if (to < from) throw editError('range');
            return { from, to, insert: edit.newText, index };
        });
        list.sort((a, b) => a.from - b.from || Number(a.to > a.from) - Number(b.to > b.from) || a.index - b.index);
        const changes = [];
        list.forEach(change => {
            const last = changes[changes.length - 1];
            if (last && change.from < last.to) throw editError('overlap');
            if (last && last.from === change.from && last.to === last.from) {
                last.to = change.to;
                last.insert += change.insert;
                return;
            }
            changes.push({ from: change.from, to: change.to, insert: change.insert });
        });
        return changes;
    }

    // Applies ascending, non-overlapping changes from the end of the text, so earlier offsets stay valid.
    function applyChanges(text, changes) {
        const parts = [];
        let cursor = text.length;
        for (let index = changes.length - 1; index >= 0; index -= 1) {
            const change = changes[index];
            parts.push(text.slice(change.to, cursor), change.insert);
            cursor = change.from;
        }
        parts.push(text.slice(0, cursor));
        return parts.reverse().join('');
    }

    function applyTextEdits(text, edits) {
        const source = String(text == null ? '' : text);
        return applyChanges(source, textEditsToChanges(source, edits));
    }

    // Batches of one file (several TextDocumentEdits for it) apply one after the other. Every step's offsets refer to
    // the text the previous step produced, which a CodeMirror transaction spec marks with sequential: true.
    function planTextEdits(text, batches) {
        let current = String(text == null ? '' : text);
        const steps = [];
        (Array.isArray(batches) ? batches : []).forEach(batch => {
            const changes = textEditsToChanges(current, batch);
            if (!changes.length) return;
            steps.push(changes);
            current = applyChanges(current, changes);
        });
        return { text: current, steps };
    }

    function isTextEdit(edit) {
        return !!edit && !!edit.range && !!edit.range.start && !!edit.range.end && typeof edit.newText === 'string';
    }

    // A WorkspaceEdit as { files, skipped }.
    // files: [{ path, uri, version, batches, edits, count }] in the order the edit names them. edits lists every
    // TextEdit of the file (for counts and previews); batches keeps the TextDocumentEdits apart for planTextEdits.
    // skipped: [{ uri, path, reason }] with reason 'create', 'rename' or 'delete' (file operations are not supported),
    // 'depends' (a text edit for a file such an operation touches), 'outside' (not under /workspace) or 'invalid'.
    // documentChanges win over changes, as the protocol says.
    function workspaceEditToFileChanges(edit) {
        const files = [];
        const skipped = [];
        const byPath = new Map();
        if (!edit || typeof edit !== 'object') return { files, skipped };
        const skip = (uri, reason, extra) => {
            skipped.push(Object.assign({ uri: typeof uri === 'string' ? uri : '', path: workspacePath(uri) || '', reason }, extra || {}));
        };
        const add = (uri, edits, version) => {
            const path = workspacePath(uri);
            if (!path) {
                skip(uri, 'outside');
                return;
            }
            if (!Array.isArray(edits) || !edits.every(isTextEdit)) {
                skip(uri, 'invalid');
                return;
            }
            if (!edits.length) return;
            let file = byPath.get(path);
            if (!file) {
                file = { path, uri, version: Number.isInteger(version) ? version : null, batches: [], edits: [], count: 0 };
                byPath.set(path, file);
                files.push(file);
            }
            file.batches.push(edits.slice());
            file.edits.push(...edits);
            file.count += edits.length;
        };
        if (Array.isArray(edit.documentChanges)) {
            const touched = new Set();
            edit.documentChanges.forEach(change => {
                if (!change || typeof change !== 'object') {
                    skip('', 'invalid');
                    return;
                }
                if (change.kind === 'create' || change.kind === 'delete') {
                    touched.add(change.uri);
                    skip(change.uri, change.kind);
                    return;
                }
                if (change.kind === 'rename') {
                    touched.add(change.oldUri);
                    touched.add(change.newUri);
                    skip(change.oldUri, 'rename', { newPath: workspacePath(change.newUri) || '' });
                    return;
                }
                const document = change.textDocument;
                if (!document || typeof document.uri !== 'string') {
                    skip('', 'invalid');
                    return;
                }
                if (touched.has(document.uri)) {
                    skip(document.uri, 'depends');
                    return;
                }
                add(document.uri, change.edits, document.version);
            });
        } else if (edit.changes && typeof edit.changes === 'object') {
            Object.keys(edit.changes).forEach(uri => add(uri, edit.changes[uri], null));
        }
        return { files, skipped };
    }

    function wholeNumber(value) {
        const number = Math.floor(Number(value));
        return Number.isFinite(number) && number > 0 ? number : 0;
    }

    // Reference locations grouped by file: { groups: [{ path, uri, items: [{ line, character, endLine, endCharacter }] }],
    // outside, total }. Files sort by path, references by position, duplicates collapse. outside counts the results
    // that are not under /workspace (the standard library, for example).
    function referencesToGroups(locations) {
        const groups = [];
        const byPath = new Map();
        const seen = new Set();
        let outside = 0;
        (Array.isArray(locations) ? locations : []).forEach(location => {
            if (!location || typeof location.uri !== 'string' || !location.range || !location.range.start) return;
            const path = workspacePath(location.uri);
            if (!path) {
                outside += 1;
                return;
            }
            const start = location.range.start;
            const end = location.range.end || start;
            const item = {
                line: wholeNumber(start.line),
                character: wholeNumber(start.character),
                endLine: wholeNumber(end.line),
                endCharacter: wholeNumber(end.character)
            };
            const key = [path, item.line, item.character, item.endLine, item.endCharacter].join('|');
            if (seen.has(key)) return;
            seen.add(key);
            let group = byPath.get(path);
            if (!group) {
                group = { path, uri: location.uri, items: [] };
                byPath.set(path, group);
                groups.push(group);
            }
            group.items.push(item);
        });
        groups.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
        groups.forEach(group => group.items.sort((a, b) => a.line - b.line || a.character - b.character));
        return { groups, outside, total: groups.reduce((sum, group) => sum + group.items.length, 0) };
    }

    // The line of a reference for the panel: indentation dropped, a long prefix shortened, the range split out.
    // Returns { before, match, after }, or null when the line does not exist.
    function referencePreview(text, item, maxLength) {
        const source = String(text == null ? '' : text);
        const starts = lineStarts(source);
        if (!item || !Number.isInteger(item.line) || item.line < 0 || item.line >= starts.length) return null;
        const lineText = source.slice(starts[item.line], lineEnd(source, starts, item.line));
        const indent = lineText.length - lineText.replace(/^\s+/, '').length;
        const from = Math.min(Math.max(Number(item.character) || 0, indent), lineText.length);
        const to = item.endLine === item.line ? Math.min(Math.max(Number(item.endCharacter) || 0, from), lineText.length) : lineText.length;
        const limit = maxLength > 0 ? maxLength : 160;
        let before = lineText.slice(indent, from);
        if (before.length > 40) before = '…' + before.slice(-39);
        const match = lineText.slice(from, to);
        const after = lineText.slice(to, to + Math.max(0, limit - before.length - match.length));
        return { before, match, after };
    }

    return { workspaceEditToFileChanges, textEditsToChanges, applyTextEdits, planTextEdits, referencesToGroups, referencePreview };
});
```

- [ ] **Step 5: Create `ui/js/desktop/apps/code-studio/lsp-edits.test.cjs` with this complete content**

```js
// Unit tests for ui/js/desktop/apps/code-studio/lsp-edits.js (Code Studio IDE package 2C).
// Run: node --test ui/js/desktop/apps/code-studio/lsp-edits.test.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": the UMD sources run in a vm with a fake module, like notes-index.test.cjs.
// lsp-edits.js maps URIs through plan 2B's CodeStudioLSPProtocol, so that module is loaded first.
function loadUmd(file, windowObject) {
    const mod = { exports: {} };
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, file), 'utf8'), {
        module: mod, exports: mod.exports, window: windowObject, console
    }, { filename: file });
    return mod.exports;
}

const protocolWindow = {};
const protocolExports = loadUmd('lsp-protocol.js', protocolWindow);
const Protocol = protocolExports && typeof protocolExports.pathForUri === 'function' ? protocolExports : protocolWindow.CodeStudioLSPProtocol;
assert.equal(typeof (Protocol && Protocol.pathForUri), 'function', 'lsp-protocol.js (plan 2B) must provide pathForUri');
const editsWindow = { CodeStudioLSPProtocol: Protocol };
const E = loadUmd('lsp-edits.js', editsWindow);
const json = value => JSON.parse(JSON.stringify(value));
const uri = file => Protocol.uriForPath('/workspace/' + file);
const at = (line, character) => ({ line, character });
const edit = (startLine, startChar, endLine, endChar, newText) => ({ range: { start: at(startLine, startChar), end: at(endLine, endChar) }, newText });
const editCode = code => err => !!err && err.editError === code;

test('the module registers on window and as a CommonJS export', () => {
    assert.equal(editsWindow.CodeStudioLSPEdits, E);
    for (const name of ['workspaceEditToFileChanges', 'applyTextEdits', 'textEditsToChanges', 'planTextEdits', 'referencesToGroups', 'referencePreview']) {
        assert.equal(typeof E[name], 'function', name);
    }
});

test('changes: one entry per file with its edits and their count', () => {
    const result = json(E.workspaceEditToFileChanges({ changes: {
        [uri('a.go')]: [edit(0, 0, 0, 1, 'x'), edit(2, 0, 2, 1, 'y')],
        [uri('pkg/b.go')]: [edit(1, 4, 1, 7, 'z')]
    } }));
    assert.deepEqual(result.skipped, []);
    assert.deepEqual(result.files.map(file => [file.path, file.count, file.batches.length]), [['/workspace/a.go', 2, 1], ['/workspace/pkg/b.go', 1, 1]]);
    assert.equal(result.files[0].uri, uri('a.go'));
    assert.deepEqual(result.files[1].edits, [edit(1, 4, 1, 7, 'z')]);
});

test('documentChanges win over changes and keep the document version', () => {
    const result = json(E.workspaceEditToFileChanges({
        changes: { [uri('old.go')]: [edit(0, 0, 0, 0, 'x')] },
        documentChanges: [{ textDocument: { uri: uri('a.go'), version: 3 }, edits: [edit(0, 0, 0, 1, 'y')] }]
    }));
    assert.deepEqual(result.files.map(file => [file.path, file.version, file.count]), [['/workspace/a.go', 3, 1]]);
});

test('a file named twice in documentChanges keeps two ordered batches', () => {
    const first = [edit(0, 0, 0, 1, 'A')];
    const second = [edit(0, 2, 0, 3, 'B')];
    const result = json(E.workspaceEditToFileChanges({ documentChanges: [
        { textDocument: { uri: uri('a.go'), version: 1 }, edits: first },
        { textDocument: { uri: uri('a.go'), version: 2 }, edits: second }
    ] }));
    assert.equal(result.files.length, 1);
    assert.deepEqual(result.files[0].batches, [first, second]);
    assert.equal(result.files[0].count, 2);
});

test('file operations are skipped with a reason, and so are edits of the files they touch', () => {
    const result = json(E.workspaceEditToFileChanges({ documentChanges: [
        { kind: 'create', uri: uri('new.go') },
        { textDocument: { uri: uri('new.go'), version: null }, edits: [edit(0, 0, 0, 0, 'package x\n')] },
        { kind: 'rename', oldUri: uri('x.go'), newUri: uri('y.go') },
        { kind: 'delete', uri: uri('z.go') },
        { textDocument: { uri: uri('a.go'), version: 1 }, edits: [edit(0, 0, 0, 1, 'b')] }
    ] }));
    assert.deepEqual(result.files.map(file => file.path), ['/workspace/a.go']);
    assert.deepEqual(result.skipped.map(item => [item.reason, item.path]), [
        ['create', '/workspace/new.go'],
        ['depends', '/workspace/new.go'],
        ['rename', '/workspace/x.go'],
        ['delete', '/workspace/z.go']
    ]);
});

test('edits outside the workspace and malformed edits are skipped; empty edit lists are ignored', () => {
    const result = json(E.workspaceEditToFileChanges({ changes: {
        'file:///etc/passwd': [edit(0, 0, 0, 0, 'x')],
        'file:///workspace/../etc/hosts': [edit(0, 0, 0, 0, 'x')],
        [uri('a.go')]: [{ range: { start: at(0, 0), end: at(0, 1) }, text: 'no newText' }],
        [uri('b.go')]: []
    } }));
    assert.deepEqual(result.files, []);
    assert.deepEqual(result.skipped.map(item => item.reason), ['outside', 'outside', 'invalid']);
    assert.deepEqual(json(E.workspaceEditToFileChanges(null)), { files: [], skipped: [] });
});

test('applyTextEdits applies unsorted edits from the end of the text', () => {
    const text = 'let a = 1;\nlet b = a + a;\n';
    const edits = [edit(1, 12, 1, 13, 'count'), edit(0, 4, 0, 5, 'count'), edit(1, 8, 1, 9, 'count')];
    assert.equal(E.applyTextEdits(text, edits), 'let count = 1;\nlet b = count + count;\n');
});

test('positions count UTF-16 code units, as LSP and JavaScript strings do', () => {
    const smile = String.fromCodePoint(0x1F600);
    assert.equal(smile.length, 2);
    assert.equal(E.applyTextEdits('x' + smile + 'y = 1', [edit(0, 3, 0, 4, 'z')]), 'x' + smile + 'z = 1');
});

test('CRLF and a lone CR end lines; a character past the line end clamps to it', () => {
    const text = 'one\r\ntwo\rthree';
    assert.equal(E.applyTextEdits(text, [edit(1, 0, 1, 3, 'TWO')]), 'one\r\nTWO\rthree');
    assert.equal(E.applyTextEdits(text, [edit(0, 99, 0, 99, '.')]), 'one.\r\ntwo\rthree');
    assert.equal(E.applyTextEdits(text, [edit(2, 99, 2, 99, '!')]), 'one\r\ntwo\rthree!');
});

test('inserts at one position keep their order and may precede one replacement', () => {
    assert.equal(E.applyTextEdits('ab', [edit(0, 1, 0, 1, 'X'), edit(0, 1, 0, 1, 'Y')]), 'aXYb');
    assert.equal(E.applyTextEdits('ab', [edit(0, 1, 0, 2, 'Z'), edit(0, 1, 0, 1, 'I')]), 'aIZ');
});

test('overlapping, out-of-range and malformed edits are rejected', () => {
    assert.throws(() => E.applyTextEdits('abcdef', [edit(0, 0, 0, 3, 'x'), edit(0, 2, 0, 4, 'y')]), editCode('overlap'));
    assert.throws(() => E.applyTextEdits('a\nb', [edit(5, 0, 5, 0, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('a\nb', [edit(2, 1, 2, 1, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('abc', [edit(0, 2, 0, 1, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('abc', [edit(-1, 0, 0, 1, 'x')]), editCode('invalid'));
    assert.throws(() => E.applyTextEdits('abc', [{ range: { start: at(0, 0), end: at(0, 1) } }]), editCode('invalid'));
    assert.equal(E.applyTextEdits('a\nb', [edit(2, 0, 2, 0, 'X')]), 'a\nbX', 'the line after the last one means the end of the text');
});

test('textEditsToChanges gives ascending offsets for one CodeMirror transaction', () => {
    assert.deepEqual(json(E.textEditsToChanges('foo bar\nfoo', [edit(1, 0, 1, 3, 'baz'), edit(0, 0, 0, 3, 'baz')])), [
        { from: 0, to: 3, insert: 'baz' },
        { from: 8, to: 11, insert: 'baz' }
    ]);
});

test('planTextEdits applies batches one after the other', () => {
    const plan = json(E.planTextEdits('a b', [[edit(0, 0, 0, 1, 'aaa')], [edit(0, 4, 0, 5, 'B')]]));
    assert.equal(plan.text, 'aaa B');
    assert.deepEqual(plan.steps, [[{ from: 0, to: 1, insert: 'aaa' }], [{ from: 4, to: 5, insert: 'B' }]]);
});

test('references are grouped by file, sorted and de-duplicated; results outside the workspace are counted', () => {
    const loc = (file, line, character, length) => ({
        uri: file.startsWith('file:') ? file : uri(file),
        range: { start: at(line, character), end: at(line, character + length) }
    });
    const result = json(E.referencesToGroups([
        loc('b.go', 4, 1, 3),
        loc('a.go', 9, 0, 3),
        loc('a.go', 2, 5, 3),
        loc('a.go', 2, 5, 3),
        loc('file:///usr/local/go/src/fmt/print.go', 10, 5, 7),
        { uri: uri('c.go') },
        null
    ]));
    assert.deepEqual(result.groups.map(group => [group.path, group.items.map(item => item.line)]), [['/workspace/a.go', [2, 9]], ['/workspace/b.go', [4]]]);
    assert.deepEqual(result.groups[0].items[0], { line: 2, character: 5, endLine: 2, endCharacter: 8 });
    assert.equal(result.outside, 1);
    assert.equal(result.total, 3);
});

test('reference previews drop the indentation and split the line around the range', () => {
    const text = 'package main\n\n\tfmt.Println(x)\n';
    assert.deepEqual(json(E.referencePreview(text, { line: 2, character: 5, endLine: 2, endCharacter: 12 })), { before: 'fmt.', match: 'Println', after: '(x)' });
    const long = 'x'.repeat(50) + 'name()';
    const preview = json(E.referencePreview(long, { line: 0, character: 50, endLine: 0, endCharacter: 54 }));
    assert.equal(preview.before, String.fromCodePoint(0x2026) + 'x'.repeat(39));
    assert.equal(preview.match, 'name');
    assert.equal(preview.after, '()');
    assert.equal(E.referencePreview(text, { line: 9, character: 0, endLine: 9, endCharacter: 1 }), null);
});
```

- [ ] **Step 6: Run the test and the syntax check**

Run: `node --test ui/js/desktop/apps/code-studio/lsp-edits.test.cjs`
Expected: `ℹ tests 16`, `ℹ pass 16`, `ℹ fail 0`, exit code 0.

Run: `node --check ui/js/desktop/apps/code-studio/lsp-edits.js`
Expected: no output, exit code 0.

Run: `node scripts/build-ui-bundles.js --check`
Expected: every line starts with `Checked`, exit code 0. `lsp-edits.js` is not a bundle part yet.

- [ ] **Step 7: Graph check.** Run `detect-changes --scope all` (see Conventions) and report the result.

- [ ] **Step 8: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-1.txt`:
```
feat(code-studio): pure LSP edit helpers for references, rename and quick fixes

IDE package 2C, task 1. lsp-edits.js turns an LSP WorkspaceEdit into per-file
change lists (file operations and paths outside /workspace are refused),
applies TextEdits with UTF-16 positions from the end of the text, rejects
overlaps and groups reference locations with line previews. Node test:
lsp-edits.test.cjs.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-1.txt ui/js/desktop/apps/code-studio/lsp-edits.js ui/js/desktop/apps/code-studio/lsp-edits.test.cjs`
Expected: `committed <sha> (parent <sha>)` and no status lines for the two paths.

---

### Task 2: Strings for all 16 desktop languages

**Files:**
- Create (not committed): `C:\disposable\code-studio-pkg2c\locales-2c.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

Shape of the locale files:
- Each file is a flat JSON object with 2-space indentation and literal UTF-8.
- Every file has `"codeStudio.formatOutputSource"` (package 1A), which is not the last entry.
- The script inserts the 35 new lines directly after it, so key order, indentation and line endings (detected per file) stay as they are.
- `tr()` fills `{{name}}`.

- [ ] **Step 1: Check the locale files are clean**

Run: `git status --short -- ui/lang/desktop`
Expected: no output. If a locale file is modified, stop and report. Committing it would take another session's keys along.

- [ ] **Step 2: Create `C:\disposable\code-studio-pkg2c\locales-2c.mjs` with this complete content**

```js
// Plan 2C (Code Studio LSP actions): adds 35 strings to the 16 desktop locale files.
// The new lines go right after "codeStudio.formatOutputSource", so key order, indentation and line endings stay.
// Usage: node C:\disposable\code-studio-pkg2c\locales-2c.mjs C:\Users\Andi\Documents\repo\AuraGo
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const repo = process.argv[2] || 'C:/Users/Andi/Documents/repo/AuraGo';
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const anchorKey = '"codeStudio.formatOutputSource":';

const strings = {
    'codeStudio.panelReferences': {
        cs: "Odkazy",
        da: "Referencer",
        de: "Verweise",
        el: "Αναφορές",
        en: "References",
        es: "Referencias",
        fr: "Références",
        hi: "संदर्भ",
        it: "Riferimenti",
        ja: "参照",
        nl: "Verwijzingen",
        no: "Referanser",
        pl: "Odwołania",
        pt: "Referências",
        sv: "Referenser",
        zh: "引用"
    },
    'codeStudio.lspFindReferences': {
        cs: "Najít všechny odkazy",
        da: "Find alle referencer",
        de: "Alle Verweise suchen",
        el: "Εύρεση όλων των αναφορών",
        en: "Find All References",
        es: "Buscar todas las referencias",
        fr: "Rechercher toutes les références",
        hi: "सभी संदर्भ खोजें",
        it: "Trova tutti i riferimenti",
        ja: "すべての参照を検索",
        nl: "Alle verwijzingen zoeken",
        no: "Finn alle referanser",
        pl: "Znajdź wszystkie odwołania",
        pt: "Localizar todas as referências",
        sv: "Hitta alla referenser",
        zh: "查找所有引用"
    },
    'codeStudio.lspRenameSymbol': {
        cs: "Přejmenovat symbol",
        da: "Omdøb symbol",
        de: "Symbol umbenennen",
        el: "Μετονομασία συμβόλου",
        en: "Rename Symbol",
        es: "Cambiar nombre del símbolo",
        fr: "Renommer le symbole",
        hi: "सिंबल का नाम बदलें",
        it: "Rinomina simbolo",
        ja: "シンボルの名前を変更",
        nl: "Symbool hernoemen",
        no: "Gi symbolet nytt navn",
        pl: "Zmień nazwę symbolu",
        pt: "Mudar o nome do símbolo",
        sv: "Byt namn på symbol",
        zh: "重命名符号"
    },
    'codeStudio.lspQuickFix': {
        cs: "Rychlá oprava...",
        da: "Hurtig rettelse...",
        de: "Schnelle Korrektur...",
        el: "Γρήγορη διόρθωση...",
        en: "Quick Fix...",
        es: "Corrección rápida...",
        fr: "Correction rapide...",
        hi: "त्वरित सुधार...",
        it: "Correzione rapida...",
        ja: "クイック修正...",
        nl: "Snelle oplossing...",
        no: "Hurtigretting...",
        pl: "Szybka poprawka...",
        pt: "Correção rápida...",
        sv: "Snabbkorrigering...",
        zh: "快速修复..."
    },
    'codeStudio.renameFile': {
        cs: "Přejmenovat soubor",
        da: "Omdøb fil",
        de: "Datei umbenennen",
        el: "Μετονομασία αρχείου",
        en: "Rename File",
        es: "Cambiar nombre del archivo",
        fr: "Renommer le fichier",
        hi: "फ़ाइल का नाम बदलें",
        it: "Rinomina file",
        ja: "ファイル名を変更",
        nl: "Bestand hernoemen",
        no: "Gi filen nytt navn",
        pl: "Zmień nazwę pliku",
        pt: "Mudar o nome do ficheiro",
        sv: "Byt namn på fil",
        zh: "重命名文件"
    },
    'codeStudio.lspReferencesEmpty': {
        cs: "Umístěte kurzor na symbol a stisknutím Shift+F12 zobrazíte jeho odkazy.",
        da: "Placer markøren på et symbol, og tryk på Shift+F12 for at vise dets referencer.",
        de: "Setze den Cursor auf ein Symbol und drücke Shift+F12, um seine Verweise aufzulisten.",
        el: "Τοποθετήστε τον δρομέα σε ένα σύμβολο και πατήστε Shift+F12 για να δείτε τις αναφορές του.",
        en: "Place the cursor on a symbol and press Shift+F12 to list its references.",
        es: "Coloca el cursor sobre un símbolo y pulsa Shift+F12 para ver sus referencias.",
        fr: "Placez le curseur sur un symbole et appuyez sur Shift+F12 pour lister ses références.",
        hi: "किसी सिंबल पर कर्सर रखें और उसके संदर्भ देखने के लिए Shift+F12 दबाएँ।",
        it: "Posiziona il cursore su un simbolo e premi Shift+F12 per elencarne i riferimenti.",
        ja: "シンボルにカーソルを置き、Shift+F12 を押すと参照が一覧表示されます。",
        nl: "Zet de cursor op een symbool en druk op Shift+F12 om de verwijzingen te tonen.",
        no: "Plasser markøren på et symbol og trykk Shift+F12 for å vise referansene.",
        pl: "Umieść kursor na symbolu i naciśnij Shift+F12, aby wyświetlić jego odwołania.",
        pt: "Coloque o cursor num símbolo e prima Shift+F12 para listar as suas referências.",
        sv: "Placera markören på en symbol och tryck på Shift+F12 för att visa dess referenser.",
        zh: "将光标放在符号上，然后按 Shift+F12 列出其引用。"
    },
    'codeStudio.lspReferencesLoading': {
        cs: "Hledají se odkazy na {{symbol}}...",
        da: "Finder referencer til {{symbol}}...",
        de: "Verweise auf {{symbol}} werden gesucht...",
        el: "Αναζήτηση αναφορών για {{symbol}}...",
        en: "Finding references to {{symbol}}...",
        es: "Buscando referencias a {{symbol}}...",
        fr: "Recherche des références à {{symbol}}...",
        hi: "{{symbol}} के संदर्भ खोजे जा रहे हैं...",
        it: "Ricerca dei riferimenti a {{symbol}}...",
        ja: "{{symbol}} の参照を検索中...",
        nl: "Verwijzingen naar {{symbol}} zoeken...",
        no: "Finner referanser til {{symbol}}...",
        pl: "Wyszukiwanie odwołań do {{symbol}}...",
        pt: "A procurar referências a {{symbol}}...",
        sv: "Söker referenser till {{symbol}}...",
        zh: "正在查找 {{symbol}} 的引用..."
    },
    'codeStudio.lspReferencesNone': {
        cs: "Pro {{symbol}} nebyly nalezeny žádné odkazy.",
        da: "Ingen referencer fundet til {{symbol}}.",
        de: "Keine Verweise auf {{symbol}} gefunden.",
        el: "Δεν βρέθηκαν αναφορές για {{symbol}}.",
        en: "No references found for {{symbol}}.",
        es: "No se encontraron referencias a {{symbol}}.",
        fr: "Aucune référence trouvée pour {{symbol}}.",
        hi: "{{symbol}} के लिए कोई संदर्भ नहीं मिला।",
        it: "Nessun riferimento trovato per {{symbol}}.",
        ja: "{{symbol}} の参照は見つかりませんでした。",
        nl: "Geen verwijzingen naar {{symbol}} gevonden.",
        no: "Fant ingen referanser til {{symbol}}.",
        pl: "Nie znaleziono odwołań do {{symbol}}.",
        pt: "Não foram encontradas referências a {{symbol}}.",
        sv: "Inga referenser till {{symbol}} hittades.",
        zh: "未找到 {{symbol}} 的引用。"
    },
    'codeStudio.lspReferencesSummary': {
        cs: "{{count}} odkazů na {{symbol}} v {{files}} souborech",
        da: "{{count}} referencer til {{symbol}} i {{files}} filer",
        de: "{{count}} Verweise auf {{symbol}} in {{files}} Dateien",
        el: "{{count}} αναφορές για {{symbol}} σε {{files}} αρχεία",
        en: "{{count}} references to {{symbol}} in {{files}} files",
        es: "{{count}} referencias a {{symbol}} en {{files}} archivos",
        fr: "{{count}} références à {{symbol}} dans {{files}} fichiers",
        hi: "{{files}} फ़ाइलों में {{symbol}} के {{count}} संदर्भ",
        it: "{{count}} riferimenti a {{symbol}} in {{files}} file",
        ja: "{{files}} 個のファイルに {{symbol}} の参照が {{count}} 件",
        nl: "{{count}} verwijzingen naar {{symbol}} in {{files}} bestanden",
        no: "{{count}} referanser til {{symbol}} i {{files}} filer",
        pl: "{{count}} odwołań do {{symbol}} w {{files}} plikach",
        pt: "{{count}} referências a {{symbol}} em {{files}} ficheiros",
        sv: "{{count}} referenser till {{symbol}} i {{files}} filer",
        zh: "{{symbol}} 在 {{files}} 个文件中有 {{count}} 处引用"
    },
    'codeStudio.lspReferencesOutside': {
        cs: "Dalších {{count}} výsledků leží mimo pracovní prostor a nejsou uvedeny.",
        da: "{{count}} resultater mere ligger uden for arbejdsområdet og vises ikke.",
        de: "{{count}} weitere Treffer liegen außerhalb des Arbeitsbereichs und werden nicht aufgeführt.",
        el: "{{count}} ακόμη αποτελέσματα βρίσκονται εκτός του χώρου εργασίας και δεν εμφανίζονται.",
        en: "{{count}} more results are outside the workspace and not listed.",
        es: "Otros {{count}} resultados están fuera del espacio de trabajo y no se muestran.",
        fr: "{{count}} autres résultats se trouvent hors de l'espace de travail et ne sont pas listés.",
        hi: "{{count}} और परिणाम वर्कस्पेस से बाहर हैं और सूची में नहीं हैं।",
        it: "Altri {{count}} risultati sono fuori dall'area di lavoro e non sono elencati.",
        ja: "ほかに {{count}} 件の結果はワークスペース外にあるため表示されません。",
        nl: "Nog {{count}} resultaten liggen buiten de werkruimte en worden niet getoond.",
        no: "{{count}} flere treff ligger utenfor arbeidsområdet og vises ikke.",
        pl: "Kolejne wyniki ({{count}}) leżą poza obszarem roboczym i nie są wyświetlane.",
        pt: "Mais {{count}} resultados estão fora da área de trabalho e não são listados.",
        sv: "{{count}} ytterligare träffar ligger utanför arbetsytan och visas inte.",
        zh: "另有 {{count}} 个结果位于工作区之外，未列出。"
    },
    'codeStudio.lspNotReady': {
        cs: "Pro tento soubor není připraven žádný jazykový server.",
        da: "Ingen sprogserver er klar til denne fil.",
        de: "Für diese Datei ist kein Sprachserver bereit.",
        el: "Δεν υπάρχει έτοιμος διακομιστής γλώσσας για αυτό το αρχείο.",
        en: "No language server is ready for this file.",
        es: "No hay ningún servidor de lenguaje listo para este archivo.",
        fr: "Aucun serveur de langage n'est prêt pour ce fichier.",
        hi: "इस फ़ाइल के लिए कोई भाषा सर्वर तैयार नहीं है।",
        it: "Nessun server di linguaggio è pronto per questo file.",
        ja: "このファイルに使える言語サーバーの準備ができていません。",
        nl: "Er is geen taalserver gereed voor dit bestand.",
        no: "Ingen språkserver er klar for denne filen.",
        pl: "Żaden serwer językowy nie jest gotowy dla tego pliku.",
        pt: "Nenhum servidor de linguagem está pronto para este ficheiro.",
        sv: "Ingen språkserver är redo för den här filen.",
        zh: "此文件没有就绪的语言服务器。"
    },
    'codeStudio.lspTimeout': {
        cs: "Jazykový server neodpověděl včas. Zkuste to znovu.",
        da: "Sprogserveren svarede ikke i tide. Prøv igen.",
        de: "Der Sprachserver hat nicht rechtzeitig geantwortet. Versuch es noch einmal.",
        el: "Ο διακομιστής γλώσσας δεν απάντησε εγκαίρως. Δοκιμάστε ξανά.",
        en: "The language server did not answer in time. Try again.",
        es: "El servidor de lenguaje no respondió a tiempo. Inténtalo de nuevo.",
        fr: "Le serveur de langage n'a pas répondu à temps. Réessayez.",
        hi: "भाषा सर्वर ने समय पर जवाब नहीं दिया। फिर से कोशिश करें।",
        it: "Il server di linguaggio non ha risposto in tempo. Riprova.",
        ja: "言語サーバーが時間内に応答しませんでした。もう一度お試しください。",
        nl: "De taalserver antwoordde niet op tijd. Probeer het opnieuw.",
        no: "Språkserveren svarte ikke i tide. Prøv igjen.",
        pl: "Serwer językowy nie odpowiedział na czas. Spróbuj ponownie.",
        pt: "O servidor de linguagem não respondeu a tempo. Tente novamente.",
        sv: "Språkservern svarade inte i tid. Försök igen.",
        zh: "语言服务器未及时响应。请重试。"
    },
    'codeStudio.lspRequestFailed': {
        cs: "Chyba jazykového serveru: {{message}}",
        da: "Fejl i sprogserveren: {{message}}",
        de: "Fehler des Sprachservers: {{message}}",
        el: "Σφάλμα διακομιστή γλώσσας: {{message}}",
        en: "Language server error: {{message}}",
        es: "Error del servidor de lenguaje: {{message}}",
        fr: "Erreur du serveur de langage : {{message}}",
        hi: "भाषा सर्वर त्रुटि: {{message}}",
        it: "Errore del server di linguaggio: {{message}}",
        ja: "言語サーバーのエラー: {{message}}",
        nl: "Fout van de taalserver: {{message}}",
        no: "Feil fra språkserveren: {{message}}",
        pl: "Błąd serwera językowego: {{message}}",
        pt: "Erro do servidor de linguagem: {{message}}",
        sv: "Fel från språkservern: {{message}}",
        zh: "语言服务器错误：{{message}}"
    },
    'codeStudio.lspWorking': {
        cs: "Čeká se na jazykový server...",
        da: "Venter på sprogserveren...",
        de: "Warte auf den Sprachserver...",
        el: "Αναμονή για τον διακομιστή γλώσσας...",
        en: "Waiting for the language server...",
        es: "Esperando al servidor de lenguaje...",
        fr: "En attente du serveur de langage...",
        hi: "भाषा सर्वर की प्रतीक्षा की जा रही है...",
        it: "In attesa del server di linguaggio...",
        ja: "言語サーバーを待っています...",
        nl: "Wachten op de taalserver...",
        no: "Venter på språkserveren...",
        pl: "Oczekiwanie na serwer językowy...",
        pt: "A aguardar o servidor de linguagem...",
        sv: "Väntar på språkservern...",
        zh: "正在等待语言服务器..."
    },
    'codeStudio.lspNoSymbol': {
        cs: "Na pozici kurzoru není žádný symbol.",
        da: "Der er intet symbol ved markøren.",
        de: "Am Cursor steht kein Symbol.",
        el: "Δεν υπάρχει σύμβολο στη θέση του δρομέα.",
        en: "There is no symbol at the cursor.",
        es: "No hay ningún símbolo en el cursor.",
        fr: "Il n'y a aucun symbole sous le curseur.",
        hi: "कर्सर पर कोई सिंबल नहीं है।",
        it: "Non c'è alcun simbolo sotto il cursore.",
        ja: "カーソル位置にシンボルがありません。",
        nl: "Er staat geen symbool bij de cursor.",
        no: "Det er ikke noe symbol ved markøren.",
        pl: "W miejscu kursora nie ma symbolu.",
        pt: "Não há nenhum símbolo no cursor.",
        sv: "Det finns ingen symbol vid markören.",
        zh: "光标处没有符号。"
    },
    'codeStudio.lspCannotRename': {
        cs: "Tento symbol nelze přejmenovat.",
        da: "Dette symbol kan ikke omdøbes.",
        de: "Dieses Symbol kann nicht umbenannt werden.",
        el: "Αυτό το σύμβολο δεν μπορεί να μετονομαστεί.",
        en: "This symbol cannot be renamed.",
        es: "No se puede cambiar el nombre de este símbolo.",
        fr: "Ce symbole ne peut pas être renommé.",
        hi: "इस सिंबल का नाम नहीं बदला जा सकता।",
        it: "Questo simbolo non può essere rinominato.",
        ja: "このシンボルの名前は変更できません。",
        nl: "Dit symbool kan niet worden hernoemd.",
        no: "Dette symbolet kan ikke få nytt navn.",
        pl: "Nie można zmienić nazwy tego symbolu.",
        pt: "Não é possível mudar o nome deste símbolo.",
        sv: "Den här symbolen kan inte byta namn.",
        zh: "无法重命名此符号。"
    },
    'codeStudio.lspRenameLabel': {
        cs: "Nový název",
        da: "Nyt navn",
        de: "Neuer Name",
        el: "Νέο όνομα",
        en: "New name",
        es: "Nuevo nombre",
        fr: "Nouveau nom",
        hi: "नया नाम",
        it: "Nuovo nome",
        ja: "新しい名前",
        nl: "Nieuwe naam",
        no: "Nytt navn",
        pl: "Nowa nazwa",
        pt: "Novo nome",
        sv: "Nytt namn",
        zh: "新名称"
    },
    'codeStudio.lspReadOnly': {
        cs: "Jen pro čtení: změny kódu jsou vypnuté.",
        da: "Skrivebeskyttet: kodeændringer er slået fra.",
        de: "Nur lesen: Codeänderungen sind ausgeschaltet.",
        el: "Μόνο για ανάγνωση: οι αλλαγές κώδικα είναι απενεργοποιημένες.",
        en: "Read-only: code changes are turned off.",
        es: "Solo lectura: los cambios de código están desactivados.",
        fr: "Lecture seule : les modifications du code sont désactivées.",
        hi: "केवल पढ़ने के लिए: कोड बदलाव बंद हैं।",
        it: "Sola lettura: le modifiche al codice sono disattivate.",
        ja: "読み取り専用: コードの変更はオフです。",
        nl: "Alleen lezen: codewijzigingen staan uit.",
        no: "Skrivebeskyttet: kodeendringer er slått av.",
        pl: "Tylko odczyt: zmiany kodu są wyłączone.",
        pt: "Só leitura: as alterações ao código estão desligadas.",
        sv: "Skrivskyddat: kodändringar är avstängda.",
        zh: "只读：代码更改已关闭。"
    },
    'codeStudio.lspNoChanges': {
        cs: "Není co měnit.",
        da: "Intet at ændre.",
        de: "Nichts zu ändern.",
        el: "Δεν υπάρχει τίποτα για αλλαγή.",
        en: "Nothing to change.",
        es: "No hay nada que cambiar.",
        fr: "Rien à modifier.",
        hi: "बदलने के लिए कुछ नहीं है।",
        it: "Niente da modificare.",
        ja: "変更するものはありません。",
        nl: "Niets te wijzigen.",
        no: "Ingenting å endre.",
        pl: "Nie ma nic do zmiany.",
        pt: "Nada para alterar.",
        sv: "Inget att ändra.",
        zh: "没有需要更改的内容。"
    },
    'codeStudio.lspQuickFixNone': {
        cs: "Zde nejsou k dispozici žádné rychlé opravy.",
        da: "Ingen hurtige rettelser her.",
        de: "Hier gibt es keine schnellen Korrekturen.",
        el: "Δεν υπάρχουν γρήγορες διορθώσεις εδώ.",
        en: "No quick fixes available here.",
        es: "No hay correcciones rápidas disponibles aquí.",
        fr: "Aucune correction rapide disponible ici.",
        hi: "यहाँ कोई त्वरित सुधार उपलब्ध नहीं है।",
        it: "Nessuna correzione rapida disponibile qui.",
        ja: "ここで使えるクイック修正はありません。",
        nl: "Hier zijn geen snelle oplossingen beschikbaar.",
        no: "Ingen hurtigrettinger tilgjengelig her.",
        pl: "Brak szybkich poprawek w tym miejscu.",
        pt: "Não há correções rápidas disponíveis aqui.",
        sv: "Inga snabbkorrigeringar finns här.",
        zh: "此处没有可用的快速修复。"
    },
    'codeStudio.lspPreviewTitle': {
        cs: "Použít tyto změny?",
        da: "Anvend disse ændringer?",
        de: "Diese Änderungen übernehmen?",
        el: "Εφαρμογή αυτών των αλλαγών;",
        en: "Apply these changes?",
        es: "¿Aplicar estos cambios?",
        fr: "Appliquer ces modifications ?",
        hi: "ये बदलाव लागू करें?",
        it: "Applicare queste modifiche?",
        ja: "これらの変更を適用しますか？",
        nl: "Deze wijzigingen toepassen?",
        no: "Bruke disse endringene?",
        pl: "Zastosować te zmiany?",
        pt: "Aplicar estas alterações?",
        sv: "Tillämpa de här ändringarna?",
        zh: "应用这些更改？"
    },
    'codeStudio.lspPreviewRename': {
        cs: "Přejmenovat {{from}} na {{to}}",
        da: "Omdøb {{from}} til {{to}}",
        de: "{{from}} in {{to}} umbenennen",
        el: "Μετονομασία {{from}} σε {{to}}",
        en: "Rename {{from}} to {{to}}",
        es: "Cambiar el nombre de {{from}} a {{to}}",
        fr: "Renommer {{from}} en {{to}}",
        hi: "{{from}} का नाम बदलकर {{to}} करें",
        it: "Rinomina {{from}} in {{to}}",
        ja: "{{from}} を {{to}} に名前変更",
        nl: "{{from}} hernoemen naar {{to}}",
        no: "Endre navn fra {{from}} til {{to}}",
        pl: "Zmień nazwę {{from}} na {{to}}",
        pt: "Mudar o nome de {{from}} para {{to}}",
        sv: "Byt namn på {{from}} till {{to}}",
        zh: "将 {{from}} 重命名为 {{to}}"
    },
    'codeStudio.lspPreviewSummary': {
        cs: "{{count}} změn v {{files}} souborech",
        da: "{{count}} ændringer i {{files}} filer",
        de: "{{count}} Änderungen in {{files}} Dateien",
        el: "{{count}} αλλαγές σε {{files}} αρχεία",
        en: "{{count}} changes in {{files}} files",
        es: "{{count}} cambios en {{files}} archivos",
        fr: "{{count}} modifications dans {{files}} fichiers",
        hi: "{{files}} फ़ाइलों में {{count}} बदलाव",
        it: "{{count}} modifiche in {{files}} file",
        ja: "{{files}} 個のファイルで {{count}} 件の変更",
        nl: "{{count}} wijzigingen in {{files}} bestanden",
        no: "{{count}} endringer i {{files}} filer",
        pl: "{{count}} zmian w {{files}} plikach",
        pt: "{{count}} alterações em {{files}} ficheiros",
        sv: "{{count}} ändringar i {{files}} filer",
        zh: "{{files}} 个文件中的 {{count}} 处更改"
    },
    'codeStudio.lspPreviewOpenTab': {
        cs: "otevřená karta, zůstane neuložená",
        da: "åben fane, forbliver ikke gemt",
        de: "offener Tab, bleibt ungespeichert",
        el: "ανοιχτή καρτέλα, μένει χωρίς αποθήκευση",
        en: "open tab, stays unsaved",
        es: "pestaña abierta, queda sin guardar",
        fr: "onglet ouvert, reste non enregistré",
        hi: "खुला टैब, बिना सहेजे रहेगा",
        it: "scheda aperta, resta non salvata",
        ja: "開いているタブ、未保存のまま",
        nl: "open tabblad, blijft onopgeslagen",
        no: "åpen fane, forblir ulagret",
        pl: "otwarta karta, pozostaje niezapisana",
        pt: "separador aberto, fica por guardar",
        sv: "öppen flik, förblir osparad",
        zh: "已打开的标签页，保持未保存"
    },
    'codeStudio.lspPreviewDisk': {
        cs: "zapíše se na disk",
        da: "skrives til disken",
        de: "wird direkt gespeichert",
        el: "εγγράφεται στον δίσκο",
        en: "written to disk",
        es: "se escribe en el disco",
        fr: "écrit sur le disque",
        hi: "डिस्क पर लिखा जाएगा",
        it: "scritto su disco",
        ja: "ディスクに書き込み",
        nl: "wordt naar schijf geschreven",
        no: "skrives til disken",
        pl: "zapisany na dysku",
        pt: "escrito no disco",
        sv: "skrivs till disken",
        zh: "写入磁盘"
    },
    'codeStudio.lspPreviewSkipped': {
        cs: "Tyto části se nepoužijí:",
        da: "Disse dele anvendes ikke:",
        de: "Diese Teile werden nicht übernommen:",
        el: "Αυτά τα μέρη δεν εφαρμόζονται:",
        en: "These parts are not applied:",
        es: "Estas partes no se aplican:",
        fr: "Ces parties ne sont pas appliquées :",
        hi: "ये हिस्से लागू नहीं होंगे:",
        it: "Queste parti non vengono applicate:",
        ja: "次の部分は適用されません:",
        nl: "Deze delen worden niet toegepast:",
        no: "Disse delene brukes ikke:",
        pl: "Te części nie zostaną zastosowane:",
        pt: "Estas partes não são aplicadas:",
        sv: "De här delarna tillämpas inte:",
        zh: "以下部分不会应用："
    },
    'codeStudio.lspPreviewApply': {
        cs: "Použít",
        da: "Anvend",
        de: "Übernehmen",
        el: "Εφαρμογή",
        en: "Apply",
        es: "Aplicar",
        fr: "Appliquer",
        hi: "लागू करें",
        it: "Applica",
        ja: "適用",
        nl: "Toepassen",
        no: "Bruk",
        pl: "Zastosuj",
        pt: "Aplicar",
        sv: "Tillämpa",
        zh: "应用"
    },
    'codeStudio.lspResultSummary': {
        cs: "Použito {{count}} změn v {{files}} souborech.",
        da: "Anvendte {{count}} ændringer i {{files}} filer.",
        de: "{{count}} Änderungen in {{files}} Dateien übernommen.",
        el: "Εφαρμόστηκαν {{count}} αλλαγές σε {{files}} αρχεία.",
        en: "Applied {{count}} changes in {{files}} files.",
        es: "Se aplicaron {{count}} cambios en {{files}} archivos.",
        fr: "{{count}} modifications appliquées dans {{files}} fichiers.",
        hi: "{{files}} फ़ाइलों में {{count}} बदलाव लागू किए गए।",
        it: "Applicate {{count}} modifiche in {{files}} file.",
        ja: "{{files}} 個のファイルに {{count}} 件の変更を適用しました。",
        nl: "{{count}} wijzigingen toegepast in {{files}} bestanden.",
        no: "Brukte {{count}} endringer i {{files}} filer.",
        pl: "Zastosowano {{count}} zmian w {{files}} plikach.",
        pt: "Aplicadas {{count}} alterações em {{files}} ficheiros.",
        sv: "Tillämpade {{count}} ändringar i {{files}} filer.",
        zh: "已在 {{files}} 个文件中应用 {{count}} 处更改。"
    },
    'codeStudio.lspResultFailed': {
        cs: "Tyto soubory nebyly změněny:",
        da: "Disse filer blev ikke ændret:",
        de: "Diese Dateien wurden nicht geändert:",
        el: "Αυτά τα αρχεία δεν άλλαξαν:",
        en: "These files were not changed:",
        es: "Estos archivos no se cambiaron:",
        fr: "Ces fichiers n'ont pas été modifiés :",
        hi: "ये फ़ाइलें नहीं बदली गईं:",
        it: "Questi file non sono stati modificati:",
        ja: "次のファイルは変更されませんでした:",
        nl: "Deze bestanden zijn niet gewijzigd:",
        no: "Disse filene ble ikke endret:",
        pl: "Te pliki nie zostały zmienione:",
        pt: "Estes ficheiros não foram alterados:",
        sv: "De här filerna ändrades inte:",
        zh: "以下文件未更改："
    },
    'codeStudio.lspSkipFileOperation': {
        cs: "{{path}}: vytváření, přejmenování ani mazání souborů není podporováno",
        da: "{{path}}: oprettelse, omdøbning eller sletning af filer understøttes ikke",
        de: "{{path}}: Dateien anlegen, umbenennen oder löschen wird nicht unterstützt",
        el: "{{path}}: η δημιουργία, μετονομασία ή διαγραφή αρχείων δεν υποστηρίζεται",
        en: "{{path}}: creating, renaming or deleting files is not supported",
        es: "{{path}}: no se admite crear, renombrar ni eliminar archivos",
        fr: "{{path}} : la création, le renommage ou la suppression de fichiers n'est pas pris en charge",
        hi: "{{path}}: फ़ाइलें बनाना, उनका नाम बदलना या हटाना समर्थित नहीं है",
        it: "{{path}}: creare, rinominare o eliminare file non è supportato",
        ja: "{{path}}: ファイルの作成、名前変更、削除には対応していません",
        nl: "{{path}}: bestanden aanmaken, hernoemen of verwijderen wordt niet ondersteund",
        no: "{{path}}: oppretting, navneendring eller sletting av filer støttes ikke",
        pl: "{{path}}: tworzenie, zmiana nazwy ani usuwanie plików nie jest obsługiwane",
        pt: "{{path}}: criar, mudar o nome ou eliminar ficheiros não é suportado",
        sv: "{{path}}: att skapa, byta namn på eller ta bort filer stöds inte",
        zh: "{{path}}：不支持创建、重命名或删除文件"
    },
    'codeStudio.lspSkipOutside': {
        cs: "{{path}}: mimo pracovní prostor",
        da: "{{path}}: uden for arbejdsområdet",
        de: "{{path}}: außerhalb des Arbeitsbereichs",
        el: "{{path}}: εκτός του χώρου εργασίας",
        en: "{{path}}: outside the workspace",
        es: "{{path}}: fuera del espacio de trabajo",
        fr: "{{path}} : hors de l'espace de travail",
        hi: "{{path}}: वर्कस्पेस से बाहर",
        it: "{{path}}: fuori dall'area di lavoro",
        ja: "{{path}}: ワークスペース外",
        nl: "{{path}}: buiten de werkruimte",
        no: "{{path}}: utenfor arbeidsområdet",
        pl: "{{path}}: poza obszarem roboczym",
        pt: "{{path}}: fora da área de trabalho",
        sv: "{{path}}: utanför arbetsytan",
        zh: "{{path}}：位于工作区之外"
    },
    'codeStudio.lspSkipInvalid': {
        cs: "{{path}}: server poslal neplatnou změnu",
        da: "{{path}}: serveren sendte en ugyldig ændring",
        de: "{{path}}: der Server hat eine ungültige Änderung gesendet",
        el: "{{path}}: ο διακομιστής έστειλε μη έγκυρη αλλαγή",
        en: "{{path}}: the server sent an invalid change",
        es: "{{path}}: el servidor envió un cambio no válido",
        fr: "{{path}} : le serveur a envoyé une modification non valide",
        hi: "{{path}}: सर्वर ने अमान्य बदलाव भेजा",
        it: "{{path}}: il server ha inviato una modifica non valida",
        ja: "{{path}}: サーバーが無効な変更を送信しました",
        nl: "{{path}}: de server stuurde een ongeldige wijziging",
        no: "{{path}}: serveren sendte en ugyldig endring",
        pl: "{{path}}: serwer wysłał nieprawidłową zmianę",
        pt: "{{path}}: o servidor enviou uma alteração inválida",
        sv: "{{path}}: servern skickade en ogiltig ändring",
        zh: "{{path}}：服务器发送了无效的更改"
    },
    'codeStudio.lspFailStale': {
        cs: "{{path}}: změněno, zatímco jazykový server pracoval",
        da: "{{path}}: ændret, mens sprogserveren arbejdede",
        de: "{{path}}: geändert, während der Sprachserver gearbeitet hat",
        el: "{{path}}: άλλαξε ενώ εργαζόταν ο διακομιστής γλώσσας",
        en: "{{path}}: changed while the language server was working",
        es: "{{path}}: cambió mientras trabajaba el servidor de lenguaje",
        fr: "{{path}} : modifié pendant que le serveur de langage travaillait",
        hi: "{{path}}: भाषा सर्वर के काम करते समय बदल गई",
        it: "{{path}}: modificato mentre il server di linguaggio lavorava",
        ja: "{{path}}: 言語サーバーの処理中に変更されました",
        nl: "{{path}}: gewijzigd terwijl de taalserver bezig was",
        no: "{{path}}: endret mens språkserveren arbeidet",
        pl: "{{path}}: zmieniono w trakcie pracy serwera językowego",
        pt: "{{path}}: alterado enquanto o servidor de linguagem trabalhava",
        sv: "{{path}}: ändrades medan språkservern arbetade",
        zh: "{{path}}：在语言服务器处理期间已更改"
    },
    'codeStudio.lspFailEdits': {
        cs: "{{path}}: server poslal překrývající se změny nebo změny mimo rozsah",
        da: "{{path}}: serveren sendte overlappende ændringer eller ændringer uden for filen",
        de: "{{path}}: der Server hat überlappende Änderungen oder Änderungen außerhalb der Datei gesendet",
        el: "{{path}}: ο διακομιστής έστειλε αλληλοεπικαλυπτόμενες αλλαγές ή αλλαγές εκτός ορίων",
        en: "{{path}}: the server sent overlapping or out-of-range changes",
        es: "{{path}}: el servidor envió cambios superpuestos o fuera de rango",
        fr: "{{path}} : le serveur a envoyé des modifications qui se chevauchent ou hors limites",
        hi: "{{path}}: सर्वर ने ओवरलैप होने वाले या सीमा से बाहर के बदलाव भेजे",
        it: "{{path}}: il server ha inviato modifiche sovrapposte o fuori intervallo",
        ja: "{{path}}: サーバーが重複した変更または範囲外の変更を送信しました",
        nl: "{{path}}: de server stuurde overlappende wijzigingen of wijzigingen buiten het bestand",
        no: "{{path}}: serveren sendte overlappende endringer eller endringer utenfor filen",
        pl: "{{path}}: serwer wysłał nakładające się zmiany lub zmiany poza zakresem",
        pt: "{{path}}: o servidor enviou alterações sobrepostas ou fora do intervalo",
        sv: "{{path}}: servern skickade överlappande ändringar eller ändringar utanför filen",
        zh: "{{path}}：服务器发送了重叠或超出范围的更改"
    },
    'codeStudio.lspFailHiddenUnsaved': {
        cs: "{{path}}: neuložené změny ve skryté kartě; uložte ji a zkuste to znovu",
        da: "{{path}}: ikke-gemte ændringer i en skjult fane; gem den, og prøv igen",
        de: "{{path}}: ungespeicherte Änderungen in einem verdeckten Tab; speichere ihn und versuch es noch einmal",
        el: "{{path}}: μη αποθηκευμένες αλλαγές σε κρυφή καρτέλα. Αποθηκεύστε την και δοκιμάστε ξανά",
        en: "{{path}}: unsaved changes in a hidden tab; save it and try again",
        es: "{{path}}: cambios sin guardar en una pestaña oculta; guárdala e inténtalo de nuevo",
        fr: "{{path}} : modifications non enregistrées dans un onglet masqué ; enregistrez-le et réessayez",
        hi: "{{path}}: छिपे टैब में बिना सहेजे बदलाव हैं; उसे सहेजें और फिर से कोशिश करें",
        it: "{{path}}: modifiche non salvate in una scheda nascosta; salvala e riprova",
        ja: "{{path}}: 非表示のタブに未保存の変更があります。保存してからもう一度お試しください",
        nl: "{{path}}: niet-opgeslagen wijzigingen in een verborgen tabblad; sla het op en probeer het opnieuw",
        no: "{{path}}: ulagrede endringer i en skjult fane; lagre den og prøv igjen",
        pl: "{{path}}: niezapisane zmiany w ukrytej karcie; zapisz ją i spróbuj ponownie",
        pt: "{{path}}: alterações por guardar num separador oculto; guarde-o e tente novamente",
        sv: "{{path}}: osparade ändringar i en dold flik; spara den och försök igen",
        zh: "{{path}}：隐藏的标签页中有未保存的更改；请保存后重试"
    }
};

const placeholders = value => (value.match(/\{\{\w+\}\}/g) || []).sort().join(',');
const formalGerman = [' Sie ', ' Ihre ', ' Ihnen '];

for (const lang of langs) {
    const file = path.join(repo, 'ui', 'lang', 'desktop', lang + '.json');
    const text = await readFile(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const existing = JSON.parse(text);
    const lines = text.split(eol);
    const index = lines.findIndex(line => line.trimStart().startsWith(anchorKey));
    if (index < 0) throw new Error(`${lang}: anchor ${anchorKey} not found`);
    if (!lines[index].trimEnd().endsWith(',')) throw new Error(`${lang}: the anchor is the last entry`);
    const indent = lines[index].match(/^\s*/)[0];
    const added = [];
    for (const [key, values] of Object.entries(strings)) {
        const value = values[lang];
        if (typeof value !== 'string' || !value.trim()) throw new Error(`${lang}: ${key} has no translation`);
        if (placeholders(value) !== placeholders(values.en)) throw new Error(`${lang}: ${key} placeholders differ from English`);
        if (lang === 'de' && formalGerman.some(word => (' ' + value + ' ').includes(word))) throw new Error(`de: ${key} uses formal address`);
        if (Object.prototype.hasOwnProperty.call(existing, key)) {
            if (existing[key] !== value) throw new Error(`${lang}: ${key} already exists with another text`);
            continue;
        }
        added.push(`${indent}${JSON.stringify(key)}: ${JSON.stringify(value)},`);
    }
    if (added.length) {
        lines.splice(index + 1, 0, ...added);
        const next = lines.join(eol);
        const parsed = JSON.parse(next);
        for (const [key, values] of Object.entries(strings)) {
            if (parsed[key] !== values[lang]) throw new Error(`${lang}: ${key} did not land`);
        }
        await writeFile(file, next, 'utf8');
    }
    console.log(`${lang}: added ${added.length} keys`);
}
```

- [ ] **Step 3: Run the script**

Run: `node C:\disposable\code-studio-pkg2c\locales-2c.mjs C:\Users\Andi\Documents\repo\AuraGo`
Expected: 16 lines `<lang>: added 35 keys`, exit code 0. A second run prints `added 0 keys` for every file.

- [ ] **Step 4: Check that the locale files still load in the Go tests**

Run: `go test ./ui -run '(?i)^(TestCodeStudioStatusAndSearchCap)$' -count=1 -v`
Expected: `--- PASS: TestCodeStudioStatusAndSearchCap`.

- [ ] **Step 5: Graph check** (`detect-changes --scope all`).

- [ ] **Step 6: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-2.txt`:
```
feat(code-studio): strings for LSP references, rename and quick fixes

IDE package 2C, task 2: 35 codeStudio.* strings (References tab, rename box,
edit preview and result dialog, quick fixes, skip and failure reasons) in all
16 desktop languages, inserted after codeStudio.formatOutputSource.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-2.txt ui/lang/desktop/cs.json ui/lang/desktop/da.json ui/lang/desktop/de.json ui/lang/desktop/el.json ui/lang/desktop/en.json ui/lang/desktop/es.json ui/lang/desktop/fr.json ui/lang/desktop/hi.json ui/lang/desktop/it.json ui/lang/desktop/ja.json ui/lang/desktop/nl.json ui/lang/desktop/no.json ui/lang/desktop/pl.json ui/lang/desktop/pt.json ui/lang/desktop/sv.json ui/lang/desktop/zh.json`
Expected: `committed <sha> (parent <sha>)`.

---

### Task 3: The WorkspaceEdit applier, the preview dialog and `workspace/applyEdit`

**Files:**
- Create: `ui/js/desktop/apps/code-studio/lsp-actions.js` (first half; it is not a bundle part until Task 6)

- [ ] **Step 1: Create `ui/js/desktop/apps/code-studio/lsp-actions.js` with this complete content**

The file is a fragment of the shared IIFE (4-space indentation like its siblings). If Task 1 warned about problem source names, write 2B's names into `LSP_ACT_SOURCES` instead of the four spec names.

```js
    // IDE package 2C: language server actions in the Code Studio editor.
    // - Find references (Shift+F12) fills a References tab in the bottom panel, grouped by file with line previews.
    // - Rename (F2 in the editor) asks for the new name in a small box at the symbol and previews the edit.
    // - Quick fixes (Ctrl+. or the lightbulb in the gutter) list the server's code actions for the line.
    // Every edit goes through one applier: open tabs change in their editor buffer as one undoable step and stay
    // unsaved; closed files are read and written through the file API. The pure helpers are in lsp-edits.js
    // (CodeStudioLSPEdits). The connection, the requests and the editor hook come from lsp.js (plan 2B). Every
    // top-level name here starts with lspAct or LSP_ACT_, so nothing clashes with lsp.js in the shared bundle closure;
    // the hooks that panel.js and shortcuts.js call (ensureReferencesPanelParts, renderReferencesPanel, clearReferences
    // and the three palette commands) are the only exceptions.

    const LSP_ACT_TIMEOUT_MS = 15000;
    // Problems published by a language server carry the server's name as their source (lsp.js, spec section 3).
    const LSP_ACT_SOURCES = new Set(['gopls', 'typescript', 'pyright', 'clangd']);
    const LSP_ACT_SEVERITY = { error: 1, warning: 2, info: 3 };
    const LSP_ACT_INVALID_TEXT = String.fromCharCode(0xfffd);
    const LSP_ACT_PREVIEW_FILES = 60;
    const LSP_ACT_BULB_SVG = '<svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">'
        + '<path d="M8 1.5a4.5 4.5 0 0 0-2.62 8.16c.39.28.62.72.62 1.2v.64h4v-.64c0-.48.23-.92.62-1.2A4.5 4.5 0 0 0 8 1.5z"/>'
        + '<path d="M6 12.5h4v.75c0 .69-.56 1.25-1.25 1.25h-1.5C6.56 14.5 6 13.94 6 13.25z"/></svg>';

    // --- Shared helpers ---

    function lspActTabShows(tab, view) {
        return !!tab && !!view && (tab.view === view || tab.secondaryView === view || (tab.views || []).includes(view));
    }

    // The Code Studio window that shows a view. Keymaps, gutters and palette commands may run outside the studio's
    // instance context, so the owner is found through the tabs of every open window.
    function lspActInstanceForView(view) {
        for (const instance of instances.values()) {
            if ((instance.openTabs || []).some(tab => lspActTabShows(tab, view))) return instance;
        }
        return null;
    }

    // Starts an action for a view inside its window's context and returns true when the view belongs to a window.
    // The action is async; a failure ends as a status notice.
    function lspActStart(view, action) {
        const instance = lspActInstanceForView(view);
        if (!instance) return false;
        let pending;
        try {
            pending = runWithInstance(instance, () => action(view));
        } catch (err) {
            pending = Promise.reject(err);
        }
        Promise.resolve(pending).catch(err => {
            if (isLiveInstance(instance)) runWithInstance(instance, () => lspActReportError(err));
        });
        return true;
    }

    // The file tab, path, language and URI of an editor view whose file has a language server.
    function lspActContext(view) {
        if (!view || !view.state || !view.state.doc) return null;
        const tab = state.openTabs.find(item => isSourceTab(item) && lspActTabShows(item, view));
        const language = tab ? CodeStudioLSPProtocol.languageForPath(tab.path) : null;
        return language ? { tab, path: tab.path, language, uri: CodeStudioLSPProtocol.uriForPath(tab.path) } : null;
    }

    function lspActOpenTab(path) {
        return state.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
    }

    // The view that owns the undo history comes first: a split of one file links its second view to tab.view.
    function lspActLiveView(tab) {
        const views = [tab.view, tab.secondaryView].filter(Boolean);
        return views.find(view => view.state && view.state.doc) || views.find(view => typeof view.setValue === 'function') || null;
    }

    // The document of an open tab as an edit would see it: the live editor, a saved editor state or the plain content.
    function lspActDocKey(tab) {
        const view = lspActLiveView(tab);
        if (view && view.state && view.state.doc) return view.state.doc;
        if (view && typeof view.getValue === 'function') return view.getValue();
        const snapshot = tab.editorSnapshot;
        if (snapshot && snapshot.state && snapshot.state.doc) return snapshot.state.doc;
        return String(tab.content || '');
    }

    function lspActSameDoc(before, now) {
        if (before === now) return true;
        if (before == null || now == null) return false;
        return (typeof before === 'string' || typeof now === 'string') && String(before) === String(now);
    }

    // What every open file looked like when a request went out; an edit for a buffer that changed since is stale.
    function lspActCaptureDocs() {
        return new Map(state.openTabs.filter(isSourceTab).map(tab => [tab, lspActDocKey(tab)]));
    }

    // LSP position to a CodeMirror offset, clamped into the document.
    function lspActOffset(doc, position) {
        const lineIndex = Math.min(Math.max(0, Math.floor(Number(position && position.line) || 0)), doc.lines - 1);
        const line = doc.line(lineIndex + 1);
        return line.from + Math.min(Math.max(0, Math.floor(Number(position && position.character) || 0)), line.length);
    }

    function lspActErrorText(err) {
        if (!err) return '';
        if (typeof err === 'string') return err;
        return String(err.message || err.code || err);
    }

    // lspRequest rejects with Error('timeout'); the client's own timer says "Request timed out".
    function lspActIsTimeout(err) {
        const text = lspActErrorText(err);
        return text === 'timeout' || /timed out/i.test(text);
    }

    function lspActReportError(err) {
        flashStatus(lspActIsTimeout(err)
            ? tr('codeStudio.lspTimeout', 'The language server did not answer in time. Try again.')
            : tr('codeStudio.lspRequestFailed', 'Language server error: {{message}}', { message: lspActErrorText(err) }), 5000);
    }

    function lspActClearStatus(target) {
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            if (target.statusTimer) clearTimeout(target.statusTimer);
            target.statusTimer = null;
            target.statusMessage = '';
            renderStatus();
        });
    }

    // The window's client for a language, or null with a notice when its server is not ready.
    async function lspActClient(target, language) {
        let client = null;
        if (runWithInstance(target, () => lspClientState(language)) === 'ready') {
            try {
                client = await runWithInstance(target, () => lspEnsureClient(language));
            } catch (_) {
                client = null;
            }
        }
        if (!isLiveInstance(target)) return null;
        if (!client) runWithInstance(target, () => flashStatus(tr('codeStudio.lspNotReady', 'No language server is ready for this file.')));
        return client;
    }

    // Requests with the 15 s budget of references and rename, sent from the window that asks.
    function lspActRequest(target, language, method, params) {
        return runWithInstance(target, () => lspRequest(language, method, params, LSP_ACT_TIMEOUT_MS));
    }

    function lspActReadOnlyText() {
        return tr('codeStudio.lspReadOnly', 'Read-only: code changes are turned off.');
    }

    function lspActLabel(path, fallback) {
        if (path) return searchResultLabel(path);
        return String(fallback || '') || tr('codeStudio.problemWorkspace', 'Workspace');
    }

    function lspActSkipMessage(item) {
        const label = lspActLabel(item.path, item.uri);
        if (item.reason === 'outside') return tr('codeStudio.lspSkipOutside', '{{path}}: outside the workspace', { path: label });
        if (item.reason === 'invalid') return tr('codeStudio.lspSkipInvalid', '{{path}}: the server sent an invalid change', { path: label });
        return tr('codeStudio.lspSkipFileOperation', '{{path}}: creating, renaming or deleting files is not supported', { path: label });
    }

    // The server answers 413 for files above its size cap; lsp-edits.js marks rejected edits with editError.
    function lspActFailure(label, err) {
        if (err && err.status === 413) return tr('codeStudio.replaceSkippedTooLarge', '{{path}}: larger than the file size limit, skipped', { path: label });
        if (err && err.editError) return tr('codeStudio.lspFailEdits', '{{path}}: the server sent overlapping or out-of-range changes', { path: label });
        return label + ': ' + lspActErrorText(err);
    }

    // --- Preview and result dialog ---

    // Lists the files of an edit with their change counts before anything is applied, and after Apply the files that
    // failed. Mounted on the body like the other Code Studio dialogs; Escape and the backdrop cancel the preview.
    // options: { title, files, skipped } for the preview, or { title, result: { summary, note, failures } }.
    // Returns { decision, showResult, close }; decision resolves to true on Apply.
    function lspActDialog(target, options) {
        const settings = options || {};
        const title = settings.title || tr('codeStudio.lspPreviewTitle', 'Apply these changes?');
        const overlay = document.createElement('div');
        overlay.className = 'cs-modal-backdrop';
        document.body.appendChild(overlay);
        const releaseFocus = trapDialogFocus(overlay);
        let mode = 'preview';
        let closed = false;
        let unregister = () => {};
        let resolveDecision = () => {};
        const decision = new Promise(resolve => { resolveDecision = resolve; });
        const onKey = event => {
            if (event.key !== 'Escape' || mode === 'busy') return;
            event.preventDefault();
            event.stopPropagation();
            close();
        };
        function close() {
            if (closed) return;
            closed = true;
            resolveDecision(false);
            unregister();
            overlay.removeEventListener('keydown', onKey);
            overlay.remove();
            releaseFocus();
        }
        const list = (items, className) => (items.length
            ? `<ul class="${className}">${items.map(text => `<li>${esc(text)}</li>`).join('')}</ul>`
            : '');
        const frame = (body, actions) => `<div class="cs-modal cs-lsp-dialog" role="dialog" aria-modal="true" aria-label="${esc(title)}">
            <h3 class="cs-lsp-dialog-title" title="${esc(title)}">${esc(title)}</h3>
            ${body}
            <div class="cs-modal-actions">${actions}</div>
        </div>`;
        function showPreview() {
            const files = settings.files || [];
            const skipped = settings.skipped || [];
            const total = files.reduce((sum, file) => sum + file.count, 0);
            const rows = files.map(file => {
                const open = runWithInstance(target, () => !!lspActOpenTab(file.path));
                const where = open
                    ? tr('codeStudio.lspPreviewOpenTab', 'open tab, stays unsaved')
                    : tr('codeStudio.lspPreviewDisk', 'written to disk');
                return `<li><span class="cs-lsp-dialog-path" title="${esc(file.path)}">${esc(searchResultLabel(file.path))}</span><em class="cs-replace-count">${esc(file.count)}</em><span class="cs-lsp-dialog-where${open ? ' is-open' : ''}">${esc(where)}</span></li>`;
            }).join('');
            const skippedPart = skipped.length
                ? `<p class="cs-lsp-dialog-note">${esc(tr('codeStudio.lspPreviewSkipped', 'These parts are not applied:'))}</p>${list(skipped, 'cs-lsp-dialog-skipped')}`
                : '';
            overlay.innerHTML = frame(
                `<p class="cs-lsp-dialog-summary">${esc(tr('codeStudio.lspPreviewSummary', '{{count}} changes in {{files}} files', { count: total, files: files.length }))}</p><ul class="cs-lsp-dialog-files">${rows}</ul>${skippedPart}`,
                `<button type="button" class="cs-button" data-lsp-cancel>${buttonIcon('x', 'X')}<span>${esc(tr('desktop.cancel', 'Cancel'))}</span></button><button type="button" class="cs-button primary" data-lsp-apply>${buttonIcon('check-square', 'Y')}<span>${esc(tr('codeStudio.lspPreviewApply', 'Apply'))}</span></button>`
            );
            overlay.querySelector('[data-lsp-cancel]').addEventListener('click', close);
            const apply = overlay.querySelector('[data-lsp-apply]');
            apply.addEventListener('click', () => {
                mode = 'busy';
                overlay.querySelectorAll('button').forEach(button => { button.disabled = true; });
                resolveDecision(true);
            });
            apply.focus();
        }
        function showResult(result) {
            if (closed) return;
            mode = 'result';
            const failures = result.failures || [];
            const failedPart = failures.length
                ? `<p class="cs-lsp-dialog-note is-error">${esc(tr('codeStudio.lspResultFailed', 'These files were not changed:'))}</p>${list(failures, 'cs-lsp-dialog-skipped')}`
                : '';
            overlay.innerHTML = frame(
                `<p class="cs-lsp-dialog-summary">${esc(result.summary || '')}</p>${result.note ? `<p class="cs-lsp-dialog-note">${esc(result.note)}</p>` : ''}${failedPart}`,
                `<button type="button" class="cs-button primary" data-lsp-close>${buttonIcon('check-square', 'Y')}<span>${esc(tr('desktop.close', 'Close'))}</span></button>`
            );
            const button = overlay.querySelector('[data-lsp-close]');
            button.addEventListener('click', close);
            button.focus();
        }
        runWithInstance(target, () => { unregister = registerDisposer(close); });
        overlay.addEventListener('keydown', onKey);
        overlay.addEventListener('mousedown', event => {
            if (event.target === overlay && mode !== 'busy') close();
        });
        if (settings.result) showResult(settings.result);
        else showPreview();
        return { decision, showResult, close };
    }

    // --- WorkspaceEdit applier ---

    // Applies a WorkspaceEdit. options: { mode: 'rename' | 'fix' | 'server', title, originPath, docsBefore, focusView }.
    // A rename always shows the preview. Other edits show it when they touch another file than originPath (the file
    // the user is in) or carry parts that cannot be applied. Resolves to { applied, cancelled, reason }.
    async function lspActApplyWorkspaceEdit(target, edit, options) {
        const settings = options || {};
        if (!canWrite(target)) {
            if (isLiveInstance(target)) runWithInstance(target, () => flashStatus(lspActReadOnlyText()));
            return { applied: false, reason: 'Code Studio is read-only.' };
        }
        if (target.lspActBusy) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...')));
            return { applied: false, reason: 'Another edit is being applied.' };
        }
        const plan = CodeStudioLSPEdits.workspaceEditToFileChanges(edit);
        const skipped = plan.skipped.map(lspActSkipMessage);
        if (!plan.files.length) {
            if (skipped.length) {
                runWithInstance(target, () => lspActDialog(target, { title: settings.title, result: {
                    summary: tr('codeStudio.lspResultSummary', 'Applied {{count}} changes in {{files}} files.', { count: 0, files: 0 }),
                    failures: skipped
                } }));
                return { applied: false, reason: 'The edit only contains changes that cannot be applied.' };
            }
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return { applied: true };
        }
        const direct = settings.mode !== 'rename' && !skipped.length && plan.files.length === 1
            && !!settings.originPath && plan.files[0].path === settings.originPath;
        target.lspActBusy = true;
        let dialog = null;
        let report;
        try {
            if (!direct) {
                dialog = runWithInstance(target, () => lspActDialog(target, { title: settings.title, files: plan.files, skipped }));
                const confirmed = await dialog.decision;
                if (!confirmed || !canWrite(target)) {
                    dialog.close();
                    if (confirmed && isLiveInstance(target)) runWithInstance(target, () => flashStatus(lspActReadOnlyText()));
                    return { applied: false, cancelled: true, reason: 'The user cancelled the edit.' };
                }
            }
            report = await lspActApplyFiles(target, plan.files, settings);
        } finally {
            target.lspActBusy = false;
        }
        report.failures = skipped.concat(report.failures);
        if (isLiveInstance(target)) runWithInstance(target, () => lspActFinish(target, report, settings, dialog));
        else if (dialog) dialog.close();
        return report.failures.length ? { applied: false, reason: report.failures.join('\n') } : { applied: true };
    }

    // Files one after the other: one file's failure never stops the others, and what was applied stays applied.
    async function lspActApplyFiles(target, files, settings) {
        const report = { count: 0, files: 0, buffers: 0, written: [], failures: [] };
        for (const file of files) {
            if (!isLiveInstance(target)) break;
            const label = searchResultLabel(file.path);
            let outcome;
            try {
                outcome = await lspActApplyFile(target, file, label, settings);
            } catch (err) {
                outcome = { ok: false, message: lspActFailure(label, err) };
            }
            if (outcome.ok) {
                report.count += file.count;
                report.files += 1;
                if (outcome.buffer) report.buffers += 1;
                else report.written.push(file.path);
            } else {
                report.failures.push(outcome.message);
            }
        }
        return report;
    }

    // An open tab takes the change in its buffer. A closed file is read, changed and written back with PUT.
    async function lspActApplyFile(target, file, label, settings) {
        const open = runWithInstance(target, () => lspActOpenTab(file.path));
        if (open) return runWithInstance(target, () => lspActApplyToTab(open, file, label, settings));
        let content;
        try {
            const result = await apiClient.file(file.path);
            content = result && typeof result.content === 'string' ? result.content : '';
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        if (!isLiveInstance(target)) return { ok: false, message: label };
        // A tab opened while the file was read takes the change in its buffer instead.
        const opened = runWithInstance(target, () => lspActOpenTab(file.path));
        if (opened) return runWithInstance(target, () => lspActApplyToTab(opened, file, label, settings));
        // Text with U+FFFD is not valid UTF-8; writing it back would corrupt the file.
        if (content.includes(LSP_ACT_INVALID_TEXT)) {
            return { ok: false, message: tr('codeStudio.replaceSkippedEncoding', '{{path}}: not plain UTF-8 text, skipped', { path: label }) };
        }
        let next;
        try {
            next = CodeStudioLSPEdits.planTextEdits(content, file.batches).text;
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        if (!canWrite(target)) return { ok: false, message: label + ': ' + lspActReadOnlyText() };
        try {
            await apiClient.writeFile(file.path, next);
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        // A tab opened after the write re-reads the file while its buffer is untouched.
        const late = isLiveInstance(target) ? runWithInstance(target, () => lspActOpenTab(file.path)) : null;
        if (late) await reloadCleanTabFromDisk(target, late);
        return { ok: true, buffer: false };
    }

    // Open files change in their buffer and stay unsaved.
    // - A live editor gets one transaction: one undo step that never joins typing. A split of the same file follows
    //   through its linked view, so the change is dispatched once, on tab.view.
    // - The textarea fallback takes the new text.
    // - A tab without a live editor changes its saved editor state, which the next render restores together with the
    //   undo history. With unsaved changes it is refused: the server may have computed the edit on the saved file.
    function lspActApplyToTab(tab, file, label, settings) {
        if (!canWrite()) return { ok: false, message: label + ': ' + lspActReadOnlyText() };
        const stale = { ok: false, message: tr('codeStudio.lspFailStale', '{{path}}: changed while the language server was working', { path: label }) };
        if (tab.pathMutation) return stale;
        if (settings.docsBefore && settings.docsBefore.has(tab) && !lspActSameDoc(settings.docsBefore.get(tab), lspActDocKey(tab))) return stale;
        try {
            const view = lspActLiveView(tab);
            if (view && view.state && view.state.doc) {
                const plan = CodeStudioLSPEdits.planTextEdits(view.state.doc.toString(), file.batches);
                if (plan.steps.length) view.dispatch(...lspActSpecs(plan.steps));
                return { ok: true, buffer: true };
            }
            if (view && typeof view.setValue === 'function') {
                view.setValue(CodeStudioLSPEdits.planTextEdits(view.getValue(), file.batches).text);
                return { ok: true, buffer: true };
            }
            if (tab.modified) {
                return { ok: false, message: tr('codeStudio.lspFailHiddenUnsaved', '{{path}}: unsaved changes in a hidden tab; save it and try again', { path: label }) };
            }
            const snapshot = tab.editorSnapshot && tab.editorSnapshot.state && tab.editorSnapshot.state.doc ? tab.editorSnapshot : null;
            const plan = CodeStudioLSPEdits.planTextEdits(snapshot ? snapshot.state.doc.toString() : String(tab.content || ''), file.batches);
            if (snapshot && plan.steps.length) snapshot.state = snapshot.state.update(...lspActSpecs(plan.steps)).state;
            updateTabContent(tab, plan.text);
            return { ok: true, buffer: true };
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
    }

    // Transaction specs for the steps of planTextEdits: later steps refer to the text the earlier ones produced.
    function lspActSpecs(steps) {
        const cm = state.cmModule;
        return steps.map((changes, index) => {
            const spec = { changes, userEvent: 'input.lsp' };
            if (index > 0) spec.sequential = true;
            else if (cm && cm.isolateHistory) spec.annotations = cm.isolateHistory.of('full');
            return spec;
        });
    }

    function lspActFinish(target, report, settings, dialog) {
        if (report.written.length) {
            lspNotifyFilesChanged(report.written.map(path => ({ path, type: 2 })));
            scheduleGitRefresh();
        }
        renderTabs();
        renderStatus();
        const summary = tr('codeStudio.lspResultSummary', 'Applied {{count}} changes in {{files}} files.', { count: report.count, files: report.files });
        const note = report.buffers && report.files > 1
            ? tr('codeStudio.replaceUnsavedNote', '{{count}} open files were changed in the editor and are not saved yet.', { count: report.buffers })
            : '';
        if (report.failures.length) {
            const result = { summary, note, failures: report.failures };
            if (dialog) dialog.showResult(result);
            else lspActDialog(target, { title: settings.title, result });
            return;
        }
        if (dialog) dialog.close();
        flashStatus(note ? summary + ' ' + note : summary);
        const view = settings.focusView;
        if (view && view.dom && view.dom.isConnected) view.focus();
    }

    // --- Server-sent workspace/applyEdit ---

    // gopls and typescript-language-server send workspace/applyEdit while they run a command. It goes through the same
    // applier and previews when it touches another file than the active one.
    async function lspActServerApplyEdit(_language, params) {
        const target = isLiveInstance(state) ? state : currentInstance();
        if (!target) return { applied: false, failureReason: 'Code Studio is not open.' };
        target.lspActServerEdits = (target.lspActServerEdits || 0) + 1;
        const origin = runWithInstance(target, () => {
            const tab = activeTab();
            return isSourceTab(tab) ? tab.path : '';
        });
        const title = params && typeof params.label === 'string' && params.label.trim() ? params.label : '';
        const outcome = await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, params && params.edit, { mode: 'server', title, originPath: origin }));
        return outcome.applied ? { applied: true } : { applied: false, failureReason: outcome.reason || 'The edit was not applied.' };
    }

    lspOnServerRequest('workspace/applyEdit', lspActServerApplyEdit);
```

- [ ] **Step 2: Syntax check**

Run: `node --check ui/js/desktop/apps/code-studio/lsp-actions.js`
Expected: no output, exit code 0.

- [ ] **Step 3: Graph check** (`detect-changes --scope all`).

- [ ] **Step 4: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-3.txt`:
```
feat(code-studio): WorkspaceEdit applier and preview dialog for LSP actions

IDE package 2C, task 3: open tabs change in their buffer as one undoable,
unsaved step (one dispatch per tab, split views follow); closed files are
read, edited and written through the file API, then lspNotifyFilesChanged.
The preview lists files and change counts; failed files are listed after
Apply while the rest stays applied. Server workspace/applyEdit uses the same
applier. Not a bundle part yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-3.txt ui/js/desktop/apps/code-studio/lsp-actions.js`
Expected: `committed <sha> (parent <sha>)`.

---

### Task 4: References, rename, quick fixes, lightbulb and the editor hook

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/lsp-actions.js` (append)

- [ ] **Step 1: Append to the end of `lsp-actions.js`, after the line `    lspOnServerRequest('workspace/applyEdit', lspActServerApplyEdit);`**

```js

    // --- Floating boxes at the cursor (rename box and quick-fix menu) ---

    // Closes open Code Studio menus the way core.js does, so only one floating box is open at a time.
    function lspActCloseMenus() {
        document.querySelectorAll('.cs-context-menu').forEach(menu => {
            if (typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup();
            else menu.remove();
        });
    }

    // Puts a fixed box under a document position, kept inside the viewport.
    function lspActPlace(element, view, pos, gap) {
        const coords = view.coordsAtPos(pos) || view.contentDOM.getBoundingClientRect();
        const rect = element.getBoundingClientRect();
        element.style.left = Math.max(8, Math.min(coords.left, window.innerWidth - rect.width - 8)) + 'px';
        element.style.top = Math.max(8, Math.min(coords.bottom + gap, window.innerHeight - rect.height - 8)) + 'px';
    }

    // --- Find references (Shift+F12) and the References tab of the bottom panel ---

    async function lspActFindReferences(view) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        const head = view.state.selection.main.head;
        const word = view.state.wordAt(head);
        if (!word) {
            flashStatus(tr('codeStudio.lspNoSymbol', 'There is no symbol at the cursor.'));
            return;
        }
        const symbol = view.state.doc.sliceString(word.from, word.to);
        const position = CodeStudioLSPProtocol.offsetToPosition(view.state.doc, head);
        const client = await lspActClient(target, ctx.language);
        if (!client) return;
        const request = Symbol('references');
        runWithInstance(target, () => {
            target.lspActReferences = { request, status: 'loading', symbol, groups: [], total: 0, outside: 0, message: '' };
            setActivePanel('references');
        });
        let result;
        try {
            result = await lspActRequest(target, ctx.language, 'textDocument/references', {
                textDocument: { uri: ctx.uri },
                position,
                context: { includeDeclaration: true }
            });
        } catch (err) {
            lspActUpdateReferences(target, request, {
                status: 'error',
                message: lspActIsTimeout(err)
                    ? tr('codeStudio.lspTimeout', 'The language server did not answer in time. Try again.')
                    : tr('codeStudio.lspRequestFailed', 'Language server error: {{message}}', { message: lspActErrorText(err) })
            });
            return;
        }
        if (!lspActReferencesCurrent(target, request)) return;
        const grouped = CodeStudioLSPEdits.referencesToGroups(result);
        const groups = grouped.groups.map(group => ({
            path: group.path,
            label: searchResultLabel(group.path),
            collapsed: false,
            items: group.items.map(item => Object.assign({ preview: null }, item))
        }));
        await lspActFillPreviews(target, groups);
        lspActUpdateReferences(target, request, { status: 'ready', groups, total: grouped.total, outside: grouped.outside });
    }

    function lspActReferencesCurrent(target, request) {
        return isLiveInstance(target) && !!target.lspActReferences && target.lspActReferences.request === request;
    }

    function lspActUpdateReferences(target, request, changes) {
        if (!lspActReferencesCurrent(target, request)) return;
        runWithInstance(target, () => {
            target.lspActReferences = Object.assign({}, target.lspActReferences, changes);
            renderReferencesPanel();
        });
    }

    // Line previews come from the open buffer of a file, otherwise from the file API, six files at a time and at most
    // LSP_ACT_PREVIEW_FILES files; the others keep their line numbers.
    async function lspActFillPreviews(target, groups) {
        const pending = [];
        groups.forEach(group => {
            const text = runWithInstance(target, () => {
                const tab = lspActOpenTab(group.path);
                return tab ? editorValue(tab) : null;
            });
            if (text !== null) lspActSetPreviews(group, text);
            else if (pending.length < LSP_ACT_PREVIEW_FILES) pending.push(group);
        });
        for (let index = 0; index < pending.length; index += 6) {
            await Promise.all(pending.slice(index, index + 6).map(async group => {
                try {
                    const result = await apiClient.file(group.path);
                    if (result && typeof result.content === 'string') lspActSetPreviews(group, result.content);
                } catch (_) {
                    // A file that cannot be read keeps its line numbers without a preview.
                }
            }));
            if (!isLiveInstance(target)) return;
        }
    }

    function lspActSetPreviews(group, text) {
        group.items.forEach(item => { item.preview = CodeStudioLSPEdits.referencePreview(text, item, 160); });
    }

    // The References tab joins the bottom panel when it is first needed, so the panel keeps its three standard views
    // until then. renderBottomPanel (panel.js) calls this before it marks the active view.
    function ensureReferencesPanelParts(panel) {
        if (!panel || (!state.lspActReferences && state.activePanel !== 'references')) return;
        if (panel.querySelector('[data-panel-tab="references"]')) return;
        const tabs = panel.querySelector('[data-panel-tabs]');
        const body = panel.querySelector('.cs-panel-body');
        if (!tabs || !body) return;
        const tab = document.createElement('button');
        tab.type = 'button';
        tab.className = 'cs-panel-tab';
        tab.setAttribute('role', 'tab');
        tab.setAttribute('aria-selected', 'false');
        tab.dataset.panelTab = 'references';
        tab.innerHTML = `<span>${esc(tr('codeStudio.panelReferences', 'References'))}</span>`;
        tab.addEventListener('click', bind(() => setActivePanel('references')));
        tabs.insertBefore(tab, tabs.querySelector('[data-panel-tab="terminal"]'));
        const pane = document.createElement('div');
        pane.className = 'cs-panel-pane';
        pane.dataset.panelPane = 'references';
        pane.setAttribute('data-references-panel', '');
        pane.hidden = true;
        body.insertBefore(pane, body.querySelector('[data-panel-pane="terminal"]'));
    }

    // Re-renders only when the result changed (renderBottomPanel runs on every problems update) or when forced.
    function renderReferencesPanel(force) {
        const pane = shellPart('[data-references-panel]');
        if (!pane) return;
        const data = state.lspActReferences || null;
        if (!force && pane.lspActShown === data) return;
        pane.lspActShown = data;
        const message = (text, extraClass) => `<div class="cs-empty${extraClass ? ' ' + extraClass : ''}">${esc(text)}</div>`;
        if (!data) {
            pane.innerHTML = message(tr('codeStudio.lspReferencesEmpty', 'Place the cursor on a symbol and press Shift+F12 to list its references.'));
            return;
        }
        if (data.status === 'loading') {
            pane.innerHTML = message(tr('codeStudio.lspReferencesLoading', 'Finding references to {{symbol}}...', { symbol: data.symbol }));
            return;
        }
        if (data.status === 'error') {
            pane.innerHTML = message(data.message, 'cs-references-error');
            return;
        }
        const outside = data.outside
            ? `<span class="cs-references-outside">${esc(tr('codeStudio.lspReferencesOutside', '{{count}} more results are outside the workspace and not listed.', { count: data.outside }))}</span>`
            : '';
        if (!data.total) {
            pane.innerHTML = message(tr('codeStudio.lspReferencesNone', 'No references found for {{symbol}}.', { symbol: data.symbol }))
                + (outside ? `<div class="cs-references-summary">${outside}</div>` : '');
            return;
        }
        const summary = `<div class="cs-references-summary"><span>${esc(tr('codeStudio.lspReferencesSummary', '{{count}} references to {{symbol}} in {{files}} files', { count: data.total, symbol: data.symbol, files: data.groups.length }))}</span>${outside}</div>`;
        const groups = data.groups.map((group, groupIndex) => {
            const rows = group.collapsed ? '' : group.items.map((item, index) => {
                const preview = item.preview;
                const code = preview ? `${esc(preview.before)}<mark>${esc(preview.match)}</mark>${esc(preview.after)}` : '';
                return `<button type="button" class="cs-reference" data-ref="${groupIndex}|${index}" title="${esc(group.label)}:${esc(item.line + 1)}"><em class="cs-search-line">${esc(item.line + 1)}</em><code>${code}</code></button>`;
            }).join('');
            return `<div class="cs-references-group${group.collapsed ? ' collapsed' : ''}">
                <button type="button" class="cs-references-file" data-ref-group="${groupIndex}" aria-expanded="${group.collapsed ? 'false' : 'true'}" title="${esc(group.path)}">
                    <span class="cs-problems-caret" aria-hidden="true">${group.collapsed ? '▸' : '▾'}</span>
                    <strong>${esc(group.label)}</strong>
                    <span class="cs-references-count">${esc(group.items.length)}</span>
                </button>${rows}
            </div>`;
        }).join('');
        pane.innerHTML = summary + groups;
        pane.querySelectorAll('[data-ref-group]').forEach(button => {
            button.addEventListener('click', bind(() => {
                const index = Number(button.dataset.refGroup);
                const group = data.groups[index];
                if (!group) return;
                group.collapsed = !group.collapsed;
                renderReferencesPanel(true);
                const again = pane.querySelector(`[data-ref-group="${index}"]`);
                if (again) again.focus();
            }));
        });
        pane.querySelectorAll('[data-ref]').forEach(button => {
            button.addEventListener('click', bind(() => {
                const [groupIndex, index] = button.dataset.ref.split('|').map(Number);
                const group = data.groups[groupIndex];
                const item = group && group.items[index];
                if (item) lspActOpenLocation(group.path, item.line, item.character);
            }));
        });
    }

    function clearReferences() {
        state.lspActReferences = null;
        renderReferencesPanel(true);
    }

    // Opens a reference: the file's tab comes forward (or opens) and the cursor lands on the reference.
    async function lspActOpenLocation(path, line, character) {
        const target = state;
        if (!isLiveInstance(target)) return;
        try {
            await runAsyncStep(target, () => openFile(path));
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = activeTab();
            const view = tab && tab.path === normalizeCodeStudioPath(path) ? focusedEditorView(tab) : null;
            if (!view || !view.state || !view.state.doc || !state.cmModule || !state.cmModule.EditorView) return;
            const pos = lspActOffset(view.state.doc, { line, character });
            view.dispatch({ selection: { anchor: pos }, effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' }) });
            view.focus();
            renderStatus();
        });
    }

    // --- Rename symbol (F2 in the editor; F2 in the file tree still renames files) ---

    async function lspActRename(view) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        if (!canWrite(target)) {
            flashStatus(lspActReadOnlyText());
            return;
        }
        const client = await lspActClient(target, ctx.language);
        if (!client || !view.dom.isConnected) return;
        const capabilities = client.serverCapabilities || null;
        const provider = capabilities ? capabilities.renameProvider : true;
        if (!provider) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspCannotRename', 'This symbol cannot be renamed.')));
            return;
        }
        const doc = view.state.doc;
        const head = view.state.selection.main.head;
        const position = CodeStudioLSPProtocol.offsetToPosition(doc, head);
        const word = view.state.wordAt(head);
        let range = word ? { from: word.from, to: word.to } : null;
        let current = word ? doc.sliceString(word.from, word.to) : '';
        if (typeof provider === 'object' && provider.prepareProvider) {
            let prepared;
            try {
                prepared = await lspActRequest(target, ctx.language, 'textDocument/prepareRename', { textDocument: { uri: ctx.uri }, position });
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
                return;
            }
            if (!isLiveInstance(target) || view.state.doc !== doc) return;
            if (!prepared) {
                runWithInstance(target, () => flashStatus(tr('codeStudio.lspCannotRename', 'This symbol cannot be renamed.')));
                return;
            }
            // prepareRename answers a Range, { range, placeholder } or { defaultBehavior: true } (keep the word).
            const preparedRange = prepared.range && prepared.range.start ? prepared.range : (prepared.start && prepared.end ? prepared : null);
            if (preparedRange) {
                range = { from: lspActOffset(doc, preparedRange.start), to: lspActOffset(doc, preparedRange.end) };
                current = typeof prepared.placeholder === 'string' ? prepared.placeholder : doc.sliceString(range.from, range.to);
            }
        }
        if (!range || !current) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoSymbol', 'There is no symbol at the cursor.')));
            return;
        }
        const newName = await runWithInstance(target, () => lspActRenameInput(target, view, range.from, current));
        if (!newName || newName === current || !isLiveInstance(target) || !view.dom.isConnected || view.state.doc !== doc) return;
        const docsBefore = runWithInstance(target, lspActCaptureDocs);
        runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...'), LSP_ACT_TIMEOUT_MS));
        let edit;
        try {
            edit = await lspActRequest(target, ctx.language, 'textDocument/rename', { textDocument: { uri: ctx.uri }, position, newName });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        lspActClearStatus(target);
        if (!edit) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return;
        }
        await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, edit, {
            mode: 'rename',
            title: tr('codeStudio.lspPreviewRename', 'Rename {{from}} to {{to}}', { from: current, to: newName }),
            originPath: ctx.path,
            docsBefore,
            focusView: view
        }));
    }

    // The new name goes into a small box at the symbol: Enter renames, Escape or a click elsewhere cancels.
    // Resolves to the trimmed name or null.
    function lspActRenameInput(target, view, pos, value) {
        return new Promise(resolve => {
            lspActCloseMenus();
            const label = tr('codeStudio.lspRenameLabel', 'New name');
            const box = document.createElement('div');
            box.className = 'cs-context-menu cs-lsp-rename-box';
            box.setAttribute('role', 'dialog');
            box.setAttribute('aria-label', label);
            box.innerHTML = `<input type="text" value="${esc(value)}" aria-label="${esc(label)}" title="${esc(label)}" autocomplete="off" spellcheck="false" autocapitalize="off" enterkeyhint="done">`;
            document.body.appendChild(box);
            lspActPlace(box, view, pos, 4);
            const input = box.querySelector('input');
            let done = false;
            let unregister = () => {};
            const finish = result => {
                if (done) return;
                done = true;
                unregister();
                box.remove();
                if (view.dom.isConnected) view.focus();
                resolve(result);
            };
            box.__codeStudioCleanup = () => finish(null);
            runWithInstance(target, () => { unregister = registerDisposer(() => finish(null)); });
            input.addEventListener('keydown', event => {
                if (event.isComposing) return;
                if (event.key === 'Enter') {
                    event.preventDefault();
                    event.stopPropagation();
                    finish(input.value.trim() || null);
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    finish(null);
                }
            });
            input.addEventListener('blur', () => setTimeout(() => finish(null), 0));
            input.focus();
            input.select();
        });
    }

    // --- Quick fixes (Ctrl+. and the lightbulb): the server's code actions for the line ---

    // LSP diagnostics of one line for the codeAction context. A problem that kept the server's own diagnostic
    // (problem.lsp, see the contract notes) sends it unchanged; otherwise the diagnostic is rebuilt from the problem.
    function lspActLineDiagnostics(path, doc, lineNumber) {
        return problemsForFile(path)
            .filter(problem => LSP_ACT_SOURCES.has(problem.source))
            .filter(problem => {
                const range = problem.lsp && problem.lsp.range;
                const first = range && range.start ? range.start.line + 1 : problem.line;
                const last = range && range.end ? range.end.line + 1 : first;
                return first <= lineNumber && lineNumber <= last;
            })
            .map(problem => lspActProblemDiagnostic(problem, doc));
    }

    function lspActProblemDiagnostic(problem, doc) {
        const raw = problem.lsp;
        if (raw && raw.range && raw.range.start && raw.range.end && typeof raw.message === 'string') return raw;
        const line = Math.max(0, (Number(problem.line) || 1) - 1);
        const text = line < doc.lines ? doc.line(line + 1).text : '';
        const character = Math.min(Math.max(0, (Number(problem.column) || 1) - 1), text.length);
        const word = /^[\p{L}\p{N}_$]+/u.exec(text.slice(character));
        const end = { line, character: Math.min(text.length, character + (word ? word[0].length : 1)) };
        const diagnostic = { range: { start: { line, character }, end }, severity: LSP_ACT_SEVERITY[problem.severity] || 1, message: String(problem.message || '') };
        if (problem.code !== undefined && problem.code !== null && problem.code !== '') diagnostic.code = problem.code;
        return diagnostic;
    }

    // Commands and code actions in one list: preferred actions first, then quick fixes, other actions, refactorings and
    // source actions, each group in the server's order.
    function lspActNormalizeActions(result) {
        const rank = kind => {
            if (kind === 'quickfix' || kind.startsWith('quickfix.')) return 0;
            if (kind.startsWith('refactor')) return 2;
            if (kind.startsWith('source')) return 3;
            return 1;
        };
        return (Array.isArray(result) ? result : [])
            .filter(item => item && typeof item.title === 'string' && item.title.trim())
            .map((item, index) => {
                const command = typeof item.command === 'string';
                return {
                    raw: item,
                    command,
                    title: item.title,
                    kind: command ? '' : String(item.kind || ''),
                    preferred: !command && item.isPreferred === true,
                    disabled: !command && item.disabled ? String(item.disabled.reason || item.title) : '',
                    index
                };
            })
            .sort((a, b) => (Number(b.preferred) - Number(a.preferred)) || (rank(a.kind) - rank(b.kind)) || (a.index - b.index));
    }

    // wholeLine: the lightbulb asks for the whole line; Ctrl+. asks for the selection.
    async function lspActQuickFixes(view, pos, wholeLine) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        if (!canWrite(target)) {
            flashStatus(lspActReadOnlyText());
            return;
        }
        const client = await lspActClient(target, ctx.language);
        if (!client || !view.dom.isConnected) return;
        const doc = view.state.doc;
        const line = doc.lineAt(Math.min(Math.max(0, pos), doc.length));
        const selection = view.state.selection.main;
        const from = wholeLine ? line.from : selection.from;
        const to = wholeLine ? line.to : selection.to;
        const range = { start: CodeStudioLSPProtocol.offsetToPosition(doc, from), end: CodeStudioLSPProtocol.offsetToPosition(doc, to) };
        const diagnostics = runWithInstance(target, () => lspActLineDiagnostics(ctx.path, doc, line.number));
        const docsBefore = runWithInstance(target, lspActCaptureDocs);
        runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...'), LSP_ACT_TIMEOUT_MS));
        let result;
        try {
            result = await lspActRequest(target, ctx.language, 'textDocument/codeAction', {
                textDocument: { uri: ctx.uri },
                range,
                context: { diagnostics, triggerKind: 1 }
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        lspActClearStatus(target);
        const actions = lspActNormalizeActions(result);
        if (!actions.length) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspQuickFixNone', 'No quick fixes available here.')));
            return;
        }
        if (!view.dom.isConnected) return;
        const run = Object.assign({}, ctx, { client, view, docsBefore });
        runWithInstance(target, () => lspActShowFixMenu(target, view, wholeLine ? line.from : selection.head, actions, action => {
            let pending;
            try {
                pending = runWithInstance(target, () => lspActRunCodeAction(target, run, action));
            } catch (err) {
                pending = Promise.reject(err);
            }
            Promise.resolve(pending).catch(err => {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            });
        }));
    }

    // The quick-fix menu at the cursor: arrow keys, Home and End move, Enter runs, Escape, Tab or a click elsewhere closes.
    function lspActShowFixMenu(target, view, pos, actions, onPick) {
        lspActCloseMenus();
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu cs-lsp-fix-menu';
        menu.setAttribute('role', 'menu');
        menu.setAttribute('aria-label', tr('codeStudio.lspQuickFix', 'Quick Fix...'));
        menu.innerHTML = actions.map((action, index) => {
            const kind = action.kind && action.kind !== 'quickfix' ? `<kbd>${esc(action.kind)}</kbd>` : '';
            return `<button type="button" role="menuitem" tabindex="-1" data-fix-index="${index}"${action.disabled ? ' disabled' : ''} title="${esc(action.disabled || action.title)}"><span>${esc(action.title)}</span>${kind}</button>`;
        }).join('');
        document.body.appendChild(menu);
        lspActPlace(menu, view, pos, 2);
        const items = Array.from(menu.querySelectorAll('button:not(:disabled)'));
        let closed = false;
        let unregister = () => {};
        const onOutside = event => {
            if (!menu.contains(event.target)) close(false);
        };
        function close(refocus) {
            if (closed) return;
            closed = true;
            unregister();
            document.removeEventListener('mousedown', onOutside, true);
            menu.remove();
            if (refocus && view.dom.isConnected) view.focus();
        }
        const move = step => {
            if (!items.length) return;
            const index = items.indexOf(document.activeElement);
            const next = index < 0 ? (step > 0 ? 0 : items.length - 1) : (index + step + items.length) % items.length;
            items[next].focus();
        };
        menu.__codeStudioCleanup = () => close(false);
        runWithInstance(target, () => { unregister = registerDisposer(() => close(false)); });
        menu.addEventListener('keydown', event => {
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault();
                move(event.key === 'ArrowDown' ? 1 : -1);
            } else if (event.key === 'Home' || event.key === 'End') {
                event.preventDefault();
                if (items.length) items[event.key === 'Home' ? 0 : items.length - 1].focus();
            } else if (event.key === 'Escape' || event.key === 'Tab') {
                event.preventDefault();
                event.stopPropagation();
                close(true);
            }
        });
        menu.querySelectorAll('[data-fix-index]').forEach(button => {
            button.addEventListener('click', () => {
                const action = actions[Number(button.dataset.fixIndex)];
                close(true);
                if (action && !action.disabled) onPick(action);
            });
        });
        setTimeout(() => {
            if (!closed) document.addEventListener('mousedown', onOutside, true);
        }, 0);
        if (items.length) {
            items[0].focus();
        } else {
            menu.tabIndex = -1;
            menu.focus();
        }
    }

    // A code action applies its edit first and then runs its command, as the protocol orders it. A plain command goes
    // straight to workspace/executeCommand; the server answers with workspace/applyEdit when it changes files.
    async function lspActRunCodeAction(target, ctx, action) {
        if (!isLiveInstance(target)) return;
        if (action.command) {
            await lspActExecuteCommand(target, ctx.language, action.raw);
            return;
        }
        let item = action.raw;
        const provider = ctx.client && ctx.client.serverCapabilities ? ctx.client.serverCapabilities.codeActionProvider : null;
        if (!item.edit && !item.command && item.data !== undefined && provider && typeof provider === 'object' && provider.resolveProvider) {
            try {
                item = (await lspActRequest(target, ctx.language, 'codeAction/resolve', item)) || item;
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
                return;
            }
            if (!isLiveInstance(target)) return;
        }
        const command = item.command && typeof item.command.command === 'string' ? item.command : null;
        if (!item.edit && !command) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return;
        }
        if (item.edit) {
            const outcome = await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, item.edit, {
                mode: 'fix',
                title: item.title,
                originPath: ctx.path,
                docsBefore: ctx.docsBefore,
                focusView: ctx.view
            }));
            if (!outcome.applied) return;
        }
        if (command) await lspActExecuteCommand(target, ctx.language, command);
    }

    async function lspActExecuteCommand(target, language, command) {
        const before = target.lspActServerEdits || 0;
        const params = { command: command.command };
        if (Array.isArray(command.arguments)) params.arguments = command.arguments;
        try {
            await lspActRequest(target, language, 'workspace/executeCommand', params);
        } catch (err) {
            // While the server waits for a previewed workspace/applyEdit the command itself may time out; that edit
            // reports its own result.
            if (!isLiveInstance(target) || (lspActIsTimeout(err) && (target.lspActServerEdits || 0) !== before)) return;
            runWithInstance(target, () => lspActReportError(err));
        }
    }

    // --- Lightbulb gutter and the editor hook ---

    let lspActBulbType = null;
    let lspActBulbBase = null;

    // GutterMarker comes from the dynamically imported CodeMirror bundle, so the subclass is built on first use.
    function lspActBulbClass(cm) {
        if (lspActBulbType && lspActBulbBase === cm.GutterMarker) return lspActBulbType;
        lspActBulbBase = cm.GutterMarker;
        lspActBulbType = class LspActBulbMarker extends cm.GutterMarker {
            eq(other) {
                return other instanceof LspActBulbMarker;
            }

            toDOM() {
                const element = document.createElement('span');
                element.className = 'cs-lsp-bulb';
                element.title = tr('codeStudio.lspQuickFix', 'Quick Fix...') + ' (Ctrl+.)';
                element.innerHTML = LSP_ACT_BULB_SVG;
                return element;
            }
        };
        return lspActBulbType;
    }

    // Line numbers with language server problems in the file a view shows, cached per problem list. A view that is not
    // attached to its tab yet (the gutter runs while the editor is built) asks for a few more updates.
    function lspActBulbLines(view, cache) {
        if (!cache.instance || !isLiveInstance(cache.instance)) cache.instance = lspActInstanceForView(view);
        if (!cache.instance) {
            if (!cache.retry && cache.retries < 3) {
                cache.retry = true;
                cache.retries += 1;
                setTimeout(() => {
                    cache.retry = false;
                    if (view.dom.isConnected) {
                        try { view.dispatch({}); } catch (_) {}
                    }
                }, 0);
            }
            return null;
        }
        return runWithInstance(cache.instance, () => {
            const tab = state.openTabs.find(item => isSourceTab(item) && lspActTabShows(item, view));
            if (!tab) return null;
            if (cache.problems !== state.problems || cache.path !== tab.path) {
                cache.problems = state.problems;
                cache.path = tab.path;
                cache.lines = new Set(problemsForFile(tab.path).filter(problem => LSP_ACT_SOURCES.has(problem.source)).map(problem => problem.line));
            }
            return cache.lines;
        });
    }

    function lspActBulbGutter(cm) {
        if (typeof cm.gutter !== 'function' || typeof cm.GutterMarker !== 'function') return [];
        const Marker = lspActBulbClass(cm);
        const marker = new Marker();
        const cache = { instance: null, problems: null, path: '', lines: null, retry: false, retries: 0 };
        const hasBulb = (view, from) => {
            const lines = lspActBulbLines(view, cache);
            return !!lines && lines.has(view.state.doc.lineAt(from).number);
        };
        return [cm.gutter({
            class: 'cs-lsp-bulb-gutter',
            lineMarker: (view, line) => (hasBulb(view, line.from) ? marker : null),
            lineMarkerChange: update => update.docChanged || update.transactions.length > 0,
            initialSpacer: () => marker,
            domEventHandlers: {
                mousedown: (view, line, event) => {
                    if (!hasBulb(view, line.from)) return false;
                    event.preventDefault();
                    return lspActStart(view, editor => lspActQuickFixes(editor, line.from, true));
                }
            }
        })];
    }

    // lsp.js appends these extensions to every editor whose file has a language server (contract: lspActionExtensionFactories).
    function lspActEditorExtensions(cm) {
        if (!cm || !cm.keymap) return [];
        const keys = cm.keymap.of([
            { key: 'Shift-F12', preventDefault: true, run: view => lspActStart(view, lspActFindReferences) },
            { key: 'F2', preventDefault: true, run: view => lspActStart(view, lspActRename) },
            { key: 'Mod-.', preventDefault: true, run: view => lspActStart(view, editor => lspActQuickFixes(editor, editor.state.selection.main.head, false)) }
        ]);
        return [cm.Prec && typeof cm.Prec.high === 'function' ? cm.Prec.high(keys) : keys, lspActBulbGutter(cm)];
    }

    lspActionExtensionFactories.push(lspActEditorExtensions);

    // --- Command palette entries (studioCommandTable in shortcuts.js) ---

    function lspActPaletteAction(action) {
        const tab = activeTab();
        const view = isSourceTab(tab) ? focusedEditorView(tab) : null;
        if (!view || !lspActContext(view)) {
            flashStatus(tr('codeStudio.lspNotReady', 'No language server is ready for this file.'));
            return;
        }
        view.focus();
        lspActStart(view, action);
    }

    function lspFindReferencesCommand() {
        lspActPaletteAction(lspActFindReferences);
    }

    function lspRenameSymbolCommand() {
        lspActPaletteAction(lspActRename);
    }

    function lspQuickFixCommand() {
        lspActPaletteAction(editor => lspActQuickFixes(editor, editor.state.selection.main.head, false));
    }
```

- [ ] **Step 2: Syntax check**

Run: `node --check ui/js/desktop/apps/code-studio/lsp-actions.js`
Expected: no output, exit code 0.

- [ ] **Step 3: Graph check** (`detect-changes --scope all`).

- [ ] **Step 4: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-4.txt`:
```
feat(code-studio): find references, rename symbol and quick fixes in the editor

IDE package 2C, task 4: Shift+F12 fills a References tab (grouped by file,
line previews, click opens the location), F2 asks for the new name in a box
at the symbol (prepareRename when the server offers it) and previews the
rename, Ctrl+. and a gutter lightbulb list code actions (edit, then command;
codeAction/resolve and workspace/executeCommand). Registered through
lspActionExtensionFactories. Not a bundle part yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-4.txt ui/js/desktop/apps/code-studio/lsp-actions.js`
Expected: `committed <sha> (parent <sha>)`.

---

### Task 5: Styles for the References tab, the dialog, the rename box, the quick-fix menu and the lightbulb

**Files:**
- Modify: `ui/css/code-studio.css` (append only)

Every element below uses the existing Code Studio tokens. The dialog, the rename box and the menu are mounted on the body. They use `.cs-modal-backdrop` / `.cs-context-menu`, so the token block at the top of the file already covers them.

- [ ] **Step 1: Check the stylesheet is clean**

Run: `git status --short -- ui/css/code-studio.css`
Expected: no output, otherwise stop and report.

- [ ] **Step 2: Append to the end of `ui/css/code-studio.css`**

```css

/* === IDE package 2C: LSP actions === */
/* References tab of the bottom panel: a summary line, one header per file and one row per reference. */
.cs-references-summary {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 14px;
    flex: 0 0 auto;
    padding: 6px 12px;
    border-bottom: 1px solid var(--cs-line);
    color: var(--cs-muted);
    font-size: 11px;
}

.cs-references-outside {
    font-style: italic;
}

.cs-empty.cs-references-error {
    color: var(--cs-danger);
}

.cs-references-group {
    display: flex;
    flex-direction: column;
}

.cs-references-file {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    padding: 5px 12px;
    border: 0;
    background: transparent;
    color: var(--cs-text);
    font: inherit;
    font-size: 12px;
    text-align: left;
    cursor: pointer;
}

.cs-references-file strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
}

.cs-references-count {
    flex: 0 0 auto;
    min-width: 18px;
    margin-left: auto;
    padding: 0 6px;
    border-radius: 9px;
    background: var(--cs-accent-soft);
    color: var(--cs-text);
    font-size: 10px;
    line-height: 16px;
    text-align: center;
    font-variant-numeric: tabular-nums;
}

.cs-reference {
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
    padding: 3px 12px 3px 30px;
    border: 0;
    background: transparent;
    color: var(--cs-text);
    font: inherit;
    text-align: left;
    cursor: pointer;
}

.cs-references-file:hover,
.cs-reference:hover {
    background: var(--cs-hover);
}

.cs-references-file:focus-visible,
.cs-reference:focus-visible,
.cs-lsp-fix-menu button:focus-visible {
    outline: 2px solid var(--cs-accent-border);
    outline-offset: -2px;
}

.cs-reference .cs-search-line {
    flex: 0 0 auto;
    min-width: 32px;
    color: var(--cs-muted);
    font-size: 11px;
    font-style: normal;
    text-align: right;
}

.cs-reference code {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: pre;
    color: var(--cs-text);
    font: 12px/1.4 var(--cs-mono-font);
}

.cs-reference mark {
    border-radius: 2px;
    background: var(--cs-accent-soft);
    color: inherit;
    box-shadow: inset 0 -1px 0 var(--cs-accent-border);
}

/* Rename box at the symbol and quick-fix menu at the cursor; both reuse the context-menu card and its tokens. */
.cs-context-menu.cs-lsp-rename-box {
    min-width: 220px;
    padding: 4px;
}

.cs-lsp-rename-box input {
    width: 100%;
    height: 28px;
    padding: 0 8px;
    border: 1px solid var(--cs-accent-border);
    border-radius: var(--vd-mat-field-radius, var(--ds-radius-sm));
    color: var(--cs-text);
    background: var(--vd-mat-field, var(--cs-panel-soft));
    box-shadow: var(--vd-mat-field-shadow, none);
    font: 12px/1.4 var(--cs-mono-font);
}

.cs-lsp-rename-box input:focus {
    outline: none;
}

.cs-context-menu.cs-lsp-fix-menu {
    max-width: min(520px, calc(100vw - 16px));
    max-height: min(320px, calc(100vh - 16px));
    overflow: auto;
}

.cs-lsp-fix-menu button:focus {
    background: var(--vd-mat-hover, var(--cs-accent-soft));
}

/* Preview of a multi-file edit and the list of files that were not changed. */
.cs-modal.cs-lsp-dialog {
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: min(560px, calc(100vw - 32px));
    max-height: calc(100vh - 64px);
}

.cs-lsp-dialog-title {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--cs-text);
    font-size: 14px;
    font-weight: 600;
}

.cs-modal .cs-lsp-dialog-summary,
.cs-modal .cs-lsp-dialog-note {
    color: var(--cs-muted);
    font-size: 12px;
}

.cs-modal .cs-lsp-dialog-note.is-error {
    color: var(--cs-danger);
}

.cs-lsp-dialog-files,
.cs-lsp-dialog-skipped {
    display: grid;
    gap: 2px;
    min-height: 0;
    max-height: 40vh;
    margin: 0;
    padding: 4px;
    overflow: auto;
    border: 1px solid var(--cs-line);
    border-radius: var(--cs-radius-sm);
    background: var(--vd-mat-well, transparent);
    list-style: none;
}

.cs-lsp-dialog-files li {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding: 3px 6px;
    font-size: 12px;
}

.cs-lsp-dialog-path {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--cs-text);
    font-family: var(--cs-mono-font);
}

.cs-lsp-dialog-where {
    flex: 0 0 auto;
    color: var(--cs-muted);
    font-size: 11px;
}

.cs-lsp-dialog-where.is-open {
    font-style: italic;
}

.cs-lsp-dialog-skipped li {
    padding: 3px 6px;
    color: var(--cs-text);
    font-size: 12px;
    overflow-wrap: anywhere;
}

/* Lightbulb gutter: lines with language-server problems; a click opens the quick fixes. */
.cs-lsp-bulb-gutter .cm-gutterElement {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 2px;
}

.cs-lsp-bulb {
    display: inline-flex;
    color: var(--cs-warning);
    cursor: pointer;
}

.cs-lsp-bulb svg {
    display: block;
}
```

- [ ] **Step 3: Brace balance check**

Run (Bash, repo root): `node -e 'const s=require("fs").readFileSync("ui/css/code-studio.css","utf8");const o=(s.match(/{/g)||[]).length,c=(s.match(/}/g)||[]).length;console.log(o===c?"balanced":"UNBALANCED "+o+" "+c);process.exit(o===c?0:1)'`
Expected: `balanced`.

- [ ] **Step 4: Graph check** (`detect-changes --scope all`).

- [ ] **Step 5: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-5.txt`:
```
style(code-studio): References tab, edit preview, quick-fix menu and lightbulb

IDE package 2C, task 5: styles for the References panel rows, the rename box
and the quick-fix menu (context-menu card), the edit preview/result dialog
and the gutter lightbulb, all on the existing Code Studio tokens.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-5.txt ui/css/code-studio.css`
Expected: `committed <sha> (parent <sha>)`.

---

### Task 6: Wire into the panel, the shortcut sheet and the palette, build and verify

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/panel.js`
- Modify: `ui/js/desktop/apps/code-studio/shortcuts.js`
- Modify: `ui/js/desktop/apps/code-studio/command-palette.js`
- Modify (only if Task 1 printed `WARN normalizeProblem drops problem.lsp`): `ui/js/desktop/apps/code-studio/problems.js`
- Modify: `scripts/build-ui-bundles.js`
- Generated: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Check for foreign edits**

Run: `git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js`
Expected: no output, otherwise stop and report.

- [ ] **Step 2: Impact analysis before editing**

Run `impact` (upstream) for `renderBottomPanel`, `wireBottomPanelEvents`, `setActivePanel`, `activePanelView`, `showShortcutOverlay`, `studioCommandTable` and `getCommands`. Report callers, processes and risk; warn on HIGH or CRITICAL. If a result is UNKNOWN, confirm with Grep. Expected callers:
- `renderBottomPanel`: `core.js` `renderShell` and the run-terminal path (`focusRunTerminal`); `panel.js` `setActivePanel`, `togglePanel` and the close action in `wireBottomPanelEvents`; `problems.js` `setProblems`, `addProblems` and `runDiagnostics`. The edit adds one guarded call at the start (it does nothing until references were requested) and one render call that returns early when nothing changed.
- `wireBottomPanelEvents`: `core.js` `renderShell`. The edit adds one `else if` to the clear action.
- `setActivePanel` / `activePanelView` (through `PANEL_VIEWS`):
  - `shortcuts.js`: Ctrl+Shift+M, Ctrl+Shift+U, Ctrl+J and `studioCommandTable`
  - `core.js`: window menus and the status-bar problems click
  - `replace.js` `finishProjectReplace`
  - `panel.js` `togglePanel`
  - new: `lsp-actions.js`

  Adding `'references'` only makes that one more value valid.
- `showShortcutOverlay`: `shortcuts.js` `?` key, `core.js` help menu, the palette through `studioCommandTable`. The edit changes only the item list.
- `studioCommandTable`: `shortcuts.js` `exposedCommand`. The edit adds three entries.
- `getCommands`: `command-palette.js` `getAllItems`. The edit adds three entries.

- [ ] **Step 3: `panel.js`: the References view**

Replace:
```js
    const PANEL_VIEWS = ['problems', 'output', 'terminal'];
```
With:
```js
    // IDE package 2C: 'references' is the References tab of lsp-actions.js; it joins the panel on first use.
    const PANEL_VIEWS = ['problems', 'output', 'references', 'terminal'];
```

Replace:
```js
        const panel = shellPart('[data-panel]');
        if (!panel) return;
        panel.hidden = !state.panelVisible;
```
With:
```js
        const panel = shellPart('[data-panel]');
        if (!panel) return;
        ensureReferencesPanelParts(panel); // IDE package 2C
        panel.hidden = !state.panelVisible;
```

Replace:
```js
        renderProblemsPanel();
        renderOutputPanel();
        if (active === 'terminal') renderTerminal();
```
With:
```js
        renderProblemsPanel();
        renderOutputPanel();
        renderReferencesPanel(); // IDE package 2C
        if (active === 'terminal') renderTerminal();
```

Replace:
```js
                    if (activePanelView() === 'problems') clearProblems();
                    else if (activePanelView() === 'output') clearOutput();
```
With:
```js
                    if (activePanelView() === 'problems') clearProblems();
                    else if (activePanelView() === 'output') clearOutput();
                    else if (activePanelView() === 'references') clearReferences(); // IDE package 2C
```

`core.js` `loadState` still accepts only the three standard panels, so a saved `'references'` comes back as the terminal. That is intended: references are not persisted.

- [ ] **Step 4: `shortcuts.js`: the shortcut sheet and the command table**

In `showShortcutOverlay`, Editing section, replace:
```js
                { label: tr('codeStudio.openToSide', 'Open to the Side'), keys: 'Ctrl+P, Ctrl+Enter' }
            ]},
```
With:
```js
                { label: tr('codeStudio.openToSide', 'Open to the Side'), keys: 'Ctrl+P, Ctrl+Enter' },
                // IDE package 2C: language server actions in the editor
                { label: tr('codeStudio.lspFindReferences', 'Find All References'), keys: 'Shift+F12' },
                { label: tr('codeStudio.lspRenameSymbol', 'Rename Symbol'), keys: 'F2' },
                { label: tr('codeStudio.lspQuickFix', 'Quick Fix...'), keys: 'Ctrl+.' }
            ]},
```
If plan 2B already appended items to the end of this list (for example F12), put the three items after 2B's last item instead, adding the comma.

In the Explorer section, replace:
```js
                { label: tr('codeStudio.rename', 'Rename'), keys: 'F2' },
```
With:
```js
                { label: tr('codeStudio.renameFile', 'Rename File'), keys: 'F2' },
```

In `studioCommandTable`, replace:
```js
            showShortcutOverlay,
            collapseAllDirectories
        };
```
With:
```js
            showShortcutOverlay,
            collapseAllDirectories,
            // IDE package 2C: language server actions for the command palette
            lspFindReferencesCommand,
            lspRenameSymbolCommand,
            lspQuickFixCommand
        };
```
If 2B added entries after `collapseAllDirectories`, append these three after 2B's last entry.

- [ ] **Step 5: `command-palette.js`: three commands**

In `getCommands`, replace:
```js
            { id: 'format-document', label: tr('codeStudio.formatDocument', 'Format Document'), shortcut: 'Shift+Alt+F', icon: 'edit', run: 'formatActiveDocument', disabled: !sourceTab || state.readOnly },
```
With:
```js
            { id: 'format-document', label: tr('codeStudio.formatDocument', 'Format Document'), shortcut: 'Shift+Alt+F', icon: 'edit', run: 'formatActiveDocument', disabled: !sourceTab || state.readOnly },
            // IDE package 2C: language server actions on the focused editor (without a ready server they show a notice)
            { id: 'lsp-references', label: tr('codeStudio.lspFindReferences', 'Find All References'), shortcut: 'Shift+F12', icon: 'search', run: 'lspFindReferencesCommand', disabled: !sourceTab },
            { id: 'lsp-rename', label: tr('codeStudio.lspRenameSymbol', 'Rename Symbol'), shortcut: 'F2', icon: 'edit', run: 'lspRenameSymbolCommand', disabled: !sourceTab || state.readOnly },
            { id: 'lsp-quick-fix', label: tr('codeStudio.lspQuickFix', 'Quick Fix...'), shortcut: 'Ctrl+.', icon: 'info', run: 'lspQuickFixCommand', disabled: !sourceTab || state.readOnly },
```
If 2B inserted its own lines right after `format-document`, insert these after 2B's lines.

- [ ] **Step 6 (only if Task 1 printed `WARN normalizeProblem drops problem.lsp`): `problems.js` keeps the server diagnostic**

First run `impact` (upstream) for `normalizeProblem`. Expected callers: `setProblems`, `addProblems`, `checkActiveDocument`, `parseDiagnosticOutput`, and 2B's `replaceProblemsForFile`. Then make `normalizeProblem` keep `problem.lsp` without changing any other field. In the current form, replace:
```js
    function normalizeProblem(problem) {
        const severity = ['error', 'warning', 'info'].includes(problem.severity) ? problem.severity : 'error';
        return {
```
With:
```js
    function normalizeProblem(problem) {
        const severity = ['error', 'warning', 'info'].includes(problem.severity) ? problem.severity : 'error';
        const normalized = {
```
And replace the end of the function:
```js
            source: String(problem.source || '')
        };
    }
```
With:
```js
            source: String(problem.source || '')
        };
        // IDE package 2C: keep the server's own diagnostic for textDocument/codeAction (lsp-actions.js).
        if (problem.lsp && typeof problem.lsp === 'object') normalized.lsp = problem.lsp;
        return normalized;
    }
```
If 2B changed the object fields, keep 2B's fields. The only additions are `const normalized =`, the `lsp` line and `return normalized;`. Add `ui/js/desktop/apps/code-studio/problems.js` to the commit in Step 12.

- [ ] **Step 7: Add both files to the code-studio bundle (`scripts/build-ui-bundles.js`)**

Replace:
```js
      'ui/js/desktop/apps/code-studio/lsp.js',
```
With:
```js
      'ui/js/desktop/apps/code-studio/lsp.js',
      // IDE package 2C: pure WorkspaceEdit helpers, then references, rename, quick fixes and the edit applier.
      'ui/js/desktop/apps/code-studio/lsp-edits.js',
      'ui/js/desktop/apps/code-studio/lsp-actions.js',
```
Both parts must sit after `lsp.js` (its `lspActionExtensionFactories` and `lspOnServerRequest` exist before the load-time registration runs) and before `shortcuts.js`, which closes the shared closure. Task 1 checked this.

- [ ] **Step 8: Build**

Run: `git status --short -- ui/js/desktop/apps/code-studio`
Expected: exactly your own edits, ` M ui/js/desktop/apps/code-studio/command-palette.js`, ` M ui/js/desktop/apps/code-studio/panel.js`, ` M ui/js/desktop/apps/code-studio/shortcuts.js`, plus ` M ui/js/desktop/apps/code-studio/problems.js` if Step 6 applied. Anything else: stop and report.

Run: `npm run build:ui`
Expected: a line `Built ui/js/desktop/bundles/code-studio.bundle.js (<n> parts)`, where `<n>` is two more than before this task.

Run: `node scripts/build-ui-bundles.js --check`
Expected: every line starts with `Checked`, exit code 0.

Run: `node --check ui/js/desktop/bundles/code-studio.bundle.js`
Expected: no output, exit code 0. This catches a duplicate `const` in the shared closure.

- [ ] **Step 9: Tests**

Run: `node --test ui/js/desktop/apps/code-studio/lsp-edits.test.cjs`
Expected: `ℹ fail 0`.

Run: `go test ./ui -run '(?i)^(TestDesktopUsedI18nKeysExistInAllLanguages|TestCodeStudioStatusAndSearchCap)$' -count=1 -v`
Expected: `--- PASS: TestDesktopUsedI18nKeysExistInAllLanguages` and `--- PASS: TestCodeStudioStatusAndSearchCap`. The first test checks every `tr('codeStudio.…')` key of lsp-actions.js in all 16 languages.

Find plan 2B's LSP browser test with Grep: pattern `func TestDesktopCodeStudio\w*(Lsp|LSP)\w*` in `ui/*_test.go`. Then run it together with the existing IDE panel test (PowerShell, sandbox off; check for leftover rod Chromes first):
`$env:GOTMPDIR='D:\tmp\go-build'; go test ./ui -run '(?i)^(TestDesktopCodeStudioIDEPanelBrowser|<2B test name>)$' -count=1 -v -timeout 15m`
Expected: both `--- PASS` lines.
- `TestDesktopCodeStudioIDEPanelBrowser` proves the bundle loads with lsp-actions.js (the load-time registration did not throw) and the panel still starts with its three tabs.
- 2B's test proves that the factory extensions (keymap and lightbulb gutter) mount in an LSP editor without breaking hover, diagnostics or F12.
- If a failure is unrelated to the References tab or the bundle load, report it and do not fix 2B's code.

- [ ] **Step 10: Manual smoke (optional; with the locally built image from plan 2A).** See "Manual acceptance" below. Report what was and was not checked.

- [ ] **Step 11: Graph check.** Run `detect-changes --scope all` and report the result. A `partial` or `truncated` result is not a clean check.

- [ ] **Step 12: Commit**

Write `C:\disposable\code-studio-pkg2c\msg-6.txt`:
```
feat(code-studio): wire LSP actions into the panel, shortcuts and command palette

IDE package 2C, task 6: the bottom panel learns the References view (its tab
joins on first use), the shortcut sheet lists Shift+F12, F2 (rename symbol in
the editor, rename file in the tree) and Ctrl+., the command palette offers
Find All References, Rename Symbol and Quick Fix, and lsp-edits.js plus
lsp-actions.js join the code-studio bundle after lsp.js.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
Run (Bash): `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2c/msg-6.txt ui/js/desktop/apps/code-studio/panel.js ui/js/desktop/apps/code-studio/shortcuts.js ui/js/desktop/apps/code-studio/command-palette.js scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js`
If Step 6 applied, add `ui/js/desktop/apps/code-studio/problems.js` to the path list.
Expected: `committed <sha> (parent <sha>)`. Do not commit other generated bundles.

---

## Manual acceptance (the user's, after Task 6)

Use the local image with the four servers (plan 2A) and a small sample project per language.

1. **Go, references.** Put the cursor on a function used in two files and press Shift+F12.
   - The References tab appears with both files, line numbers and highlighted previews.
   - Clicking a row opens the file at the reference.
   - A standard-library symbol shows the "outside the workspace" note.
2. **Go, rename.** Press F2 on a type used in an open tab and in a closed file. Type a new name and press Enter.
   - The preview lists both files with counts: "open tab, stays unsaved" and "written to disk".
   - Apply: the open tab shows the unsaved dot, and Ctrl+Z undoes the whole rename in one step. The closed file is changed on disk (Git panel shows it).
3. **Cancel paths.** Escape in the rename box and Cancel in the preview change nothing.
4. **Quick fix, one file.** An unused import shows a lightbulb.
   - Clicking it or pressing Ctrl+. opens the menu; arrow keys and Enter work.
   - A one-file fix applies directly, and one Ctrl+Z undoes it.
5. **TypeScript, other file.** An "add missing import" or "move to file" style fix that touches another file shows the preview first.
6. **Failure path.** Make one target file unreadable or too large. Rename: the result view lists that file and the others stay applied.
7. **Split view.** With the same file in both panes, a rename applies once and both panes show it.
8. **Read-only desktop.** No LSP actions are available: LSP is off, and the palette entries for rename and quick fix are disabled.
