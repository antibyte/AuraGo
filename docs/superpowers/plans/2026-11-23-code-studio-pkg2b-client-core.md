# Code Studio package 2b: LSP client core — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship section 3 of the approved spec `docs/superpowers/specs/2026-11-23-code-studio-ide-package-2-lsp-design.md` for the client core:
- the `@codemirror/lsp-client` vendor bundle
- `lsp-protocol.js` and `lsp.js`
- diagnostics into the Problems pipeline
- hover, completion and signature help
- go to definition with F12 and Ctrl/Cmd+Click
- the status bar item and the "Code intelligence (LSP)" setting

The plan implements the "2b -> 2c" API of the contract exactly, including the controller's alignment for plan 2c (client capabilities, timeouts, raw diagnostics, name prefixes, no new bottom-panel tab). It uses the "2a -> 2b" bridge as specified.

**Architecture:** The code-studio fragments share one IIFE (`core.js` opens it, `shortcuts.js` closes it). Two new fragments go before `shortcuts.js`:
1. `lsp-protocol.js`: a UMD pure module, `window.CodeStudioLSPProtocol`, tested under node.
2. `lsp.js`: the connection manager.

Each Code Studio window keeps `instance.lsp.connections[language]`, one connection per language. A connection owns:
- a WebSocket to `/api/code-studio/lsp?lang=`
- a stable transport facade
- one `LSPClient` per session

The transport handles four things:
- It answers server-to-client requests itself, from the `lspOnServerRequest` registry. Otherwise 6.3.1 would answer MethodNotFound.
- It adds `workspaceFolders` to `initialize`.
- It re-opens every tracked document right after `initialized`.
- It carries plan 2c's own requests (`lspRequest`), with per-call timers.

The client config merges the capabilities plan 2c needs, because 6.3.1 declares none of them:
- `workspace.applyEdit`
- `workspaceEdit.documentChanges`
- `rename.prepareSupport`
- the codeAction literal, resolve and data support

The client timeout is 15 s, so 2c's references and rename survive. `lspRequest` enforces its own default of 3 s.

A `Workspace` subclass keeps one LSP document per open tab, not per view. This matters because Code Studio rebuilds every view on each tab switch. Split views of one file share the document. `displayFile` opens or activates the tab.

Editors get the LSP extensions in their own compartment (`view.csLsp`). Diagnostics flow through `replaceProblemsForFile` with the raw LSP diagnostic kept as `problem.lsp`.

**Tech Stack:**
- Vanilla JS (ES2020) in the concatenated code-studio bundle.
- CodeMirror 6 plus `@codemirror/lsp-client` 6.3.1 through the rollup vendor bundle (`marked` 15 nested under it).
- Node `node:test`.
- go-rod browser test with a gorilla WebSocket fake LSP endpoint.

---

## Notes for the executor

- **Shared checkout.** Other agent sessions commit in the same checkout.
  - Never run `git add`, `git commit`, `git stash`, `git checkout -- <file>` or `git reset` on the shared index or work tree.
  - Commit each task with `bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh <message-file> <repo-relative paths...>`.
  - Write the message file with the Write tool to `C:\disposable\code-studio-pkg2b\msg-taskN.txt` (create the folder once). Pass it as `C:/disposable/code-studio-pkg2b/msg-taskN.txt`.
  - Every message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - Expected script output: `committed <sha> (parent <sha>)`, then an empty `git status --short` for the committed paths.
  - If the script is missing (the scratchpad was cleared), stop and ask.
- **Generated files.** They are committed together with their sources:
  - `ui/js/vendor/codemirror-bundle.esm.js`
  - `ui/js/desktop/bundles/code-studio.bundle.js`

  A rebuilt bundle can pick up other sessions' uncommitted source edits. Before every build, run:
  ```
  git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
  ```
  Only this task's own files may show up. If anything else shows up, stop and report it.

  `npm run build:ui` rewrites every bundle. Commit only `code-studio.bundle.js`. Leave other bundles that changed because of foreign source edits untouched (no restore, no commit), and mention them in the report. In `npm run build:ui -- --check`, only the `code-studio.bundle.js` line matters.
- **GitNexus (CLAUDE.md).**
  - Before editing an existing function, run from the repo root and report the callers and the risk:
    ```
    node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .
    ```
    MCP works too: `impact({target, direction:"upstream"})`.
  - On HIGH or CRITICAL, write a warning into the task report. Never waive it with `riskSharedAxes`.
  - `UNKNOWN` is unresolved. Confirm with Grep in `ui/js/desktop/apps/code-studio`.
  - Before every commit, run:
    ```
    node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .
    ```
    MCP: `detect_changes({scope:"all"})`. `partial`/`truncated` means re-run. Other sessions' edits show up too. Check this task's symbols and any unexpected HIGH or CRITICAL.
- **Go commands.**
  - Prefix every Go command with `GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp'` (backslashes).
  - Create the folder once: `mkdir -p /c/tmp/go-build-lsp`.
  - Browser tests also need `AURAGO_RUN_BROWSER_SMOKE=1`. Run them from PowerShell with the sandbox off. If rod reports "Failed to get the debug url", use `GOTMPDIR=D:\tmp\go-build`.
- **Write tool escapes.** No code here uses `\u` escapes; non-ASCII is literal or `String.fromCodePoint`. Copy `\\`, `\n`, `\d`, `\.`, `\{`, `\/` and `\0` exactly.
- **npm.** `node_modules` is shared.
  - Run only the single `npm install` in Task 1.
  - Never run `npm prune`, `npm ci`, `npm dedupe` or `npm audit fix`.
  - No Internet assets at runtime.
- **Scope.**
  - No Go server files (plan 2a owns `internal/server`).
  - No references, rename, quick fixes or LSP formatting (plan 2c). No `languageServerExtensions()`.
  - 2b binds only F12, Ctrl/Cmd+Click and Ctrl+Shift+Space.
  - **2b adds no bottom-panel tab.** The existing browser test expects exactly three tabs at start; 2c inserts References lazily.
- **Name prefixes (shared bundle closure; a duplicate `const` breaks the whole bundle).**
  - Every 2b top-level name starts with `lsp`, `Lsp` or `LSP_`, plus `replaceProblemsForFile` in `problems.js`.
  - 2b must not define any name starting with `lspAct` or `LSP_ACT_`, nor `ensureReferencesPanelParts`, `renderReferencesPanel`, `clearReferences`, `lspFindReferencesCommand`, `lspRenameSymbolCommand`, `lspQuickFixCommand` or `CodeStudioLSPEdits`. Plan 2c owns these.
  - The one exception is the contract array `lspActionExtensionFactories`. 2b declares it, and 2c only pushes into it and must not redeclare it.
- **Timeouts.**
  - `LSPClient` `timeout` is 15000 ms, so plan 2c's references and rename (15 s) survive. This also covers `initialize` on a cold container.
  - The library's own hover, completion, signature and definition requests therefore also wait up to 15 s; the controller accepted this.
  - `lspRequest` enforces its own timer: default 3000 ms, rejecting `Error('timeout')`. 2c passes 15000 for references and rename.
- **Lean tests.** New: `lsp-protocol.test.cjs` and one browser test. Existing static tests are run where they verify a step. The asset export list is extended.
- **Known limitation.** A definition outside `/workspace` (Go standard library) cannot be opened by the file API; the jump does nothing.

## Contract recap (what this plan delivers to 2c)

| Item | Behaviour |
| --- | --- |
| `CodeStudioLSPProtocol.languageForPath(path)` | `'go'`, `'typescript'`, `'python'`, `'cpp'` or `null` |
| `uriForPath(path)` / `pathForUri(uri)` | `file:///workspace/...` with percent-encoded segments and back. Anything outside `/workspace`, dot segments, `%2F`, query or fragment gives `null`. |
| `diagnosticsToProblems(path, diagnostics, source)` | Severity 1..4 maps to error, warning, info, info. Missing severity is error. Positions become 1-based `line`, `column`, `endLine`, `endColumn`. **`problem.lsp`** holds the original diagnostic (`range`, `message`, and `severity`, `code`, `source`, `data` when present). |
| `positionToOffset(doc, pos)` / `offsetToPosition(doc, offset)` | LSP `{line, character}` in UTF-16 units, clamped. `doc` is a CodeMirror `Text`. |
| Extras | `languageIdForPath`, `sourceForLanguage`, `ROOT_URI`, `LANGUAGES`, `SOURCES` (`gopls`, `typescript`, `pyright`, `clangd`) |
| `normalizeProblem` (problems.js) | Keeps `problem.lsp` when present. `replaceProblemsForFile(file, source, problems)` keeps it too. |
| Client capabilities | Merged into the 6.3.1 defaults: `workspace.applyEdit`, `workspace.workspaceEdit.documentChanges`, `textDocument.rename.prepareSupport`, `textDocument.codeAction` (literal kinds, isPreferred, disabled, data, resolveSupport `edit`), plus `workspace.configuration`, `didChangeWatchedFiles` (no dynamic registration) and `synchronization.didSave` |
| `lspEnsureClient(language)` | Resolves the client once ready. Resolves `null` when LSP is off, read-only, not installed, too many, or error. |
| `lspClientState(language)` | One of `'off'`, `'starting'`, `'ready'`, `'disconnected'`, `'not-installed'`, `'too-many'`, `'error'` |
| `lspRequest(language, method, params, timeoutMs = 3000)` | Calls `client.sync()` first. Uses its own id space (`cs-N`) and its own timer. Rejects `Error('timeout')` and sends `$/cancelRequest`. |
| `lspOnServerRequest(method, handler)` | `handler(language, params)` returns a result or a Promise. It runs synchronously with `state` set to the window; capture `state` before awaiting. The defaults cover `workspace/configuration`, `window/workDoneProgress/create`, `client/registerCapability`, `window/showMessageRequest`, plus `client/unregisterCapability`. |
| `lspNotifyFilesChanged(changes)` | `[{path, type}]`. Relative paths are normalized. Sends `didSave` for changed open documents and `didChangeWatchedFiles` to each path's ready client. Project files without a language go to every ready client. |
| `lspActionExtensionFactories` | Called as `factory(cm, { language, path, tab })`. The results join the LSP compartment of every LSP-enabled editor. |
| `lspViewForPath(path)` / `lspTouch(language)` | Focused view of the open tab or `null`. `lspTouch` resets the idle timer and reconnects an idle-closed client. |

## File Structure

**Created**

| File | Purpose |
| --- | --- |
| `ui/js/desktop/apps/code-studio/lsp-protocol.js` | Pure UMD module `window.CodeStudioLSPProtocol` |
| `ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs` | Node test of the pure module |
| `ui/js/desktop/apps/code-studio/lsp.js` | Connection manager, transport, Workspace subclass, client config and capabilities, editor extensions, sanitizer, status markup, contract API |
| `ui/desktop_code_studio_lsp_browser_test.go` | The one browser test with a scripted fake LSP endpoint |
| `C:\disposable\code-studio-pkg2b\locales-2b.mjs` | One-off locale merge script; not committed |

**Modified**

| File | Change |
| --- | --- |
| `package.json`, `package-lock.json` | `@codemirror/lsp-client` 6.3.1 (by npm) |
| `scripts/codemirror-entry.js`, `scripts/build-codemirror.js` | LSP exports, `Text`, `ChangeSet`; package list |
| `ui/js/vendor/codemirror-bundle.esm.js` | Generated |
| `ui/code_studio_assets_test.go` | Export list covers the new names |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | 10 new `codeStudio.*` keys |
| `ui/js/desktop/apps/code-studio/problems.js` | `replaceProblemsForFile`; `normalizeProblem` keeps end positions and `lsp`; whole-range squiggles; `checkDocumentContent` skips the bracket check for a ready server |
| `ui/js/desktop/apps/code-studio/editor.js` | `createCodeMirrorEditor`: LSP compartment (`view.csLsp`, `view.csLspActive`) |
| `ui/js/desktop/apps/code-studio/settings.js` | `codeIntelligence` setting, dialog switch, live apply |
| `ui/js/desktop/apps/code-studio/core.js` | Status item and click in `renderStatus`; `applyStudioPolicy`; `saveTab` notifies |
| `ui/js/desktop/apps/code-studio/replace.js` | `projectReplaceInFile` notifies after writing a closed file |
| `ui/js/desktop/apps/code-studio/git.js` | Discard and branch switch notify |
| `scripts/build-ui-bundles.js` | Adds `lsp-protocol.js` and `lsp.js` before `shortcuts.js` |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Generated |
| `ui/css/code-studio.css` | Appended block `/* === IDE package 2b: LSP client core === */` |
| `ui/desktop_code_studio_browser_test.go` | Fixture backend: `lspHandler` field and `/api/code-studio/lsp` route (default: close 4404) |

---

### Task 1: Vendor bundle: `@codemirror/lsp-client` 6.3.1

**Files:**
- Modify: `package.json`, `package-lock.json` (by npm), `scripts/codemirror-entry.js`, `scripts/build-codemirror.js`, `ui/code_studio_assets_test.go`
- Generate: `ui/js/vendor/codemirror-bundle.esm.js`

- [ ] **Step 1: Check for foreign edits and record the bundle size**

```
git status --short -- package.json package-lock.json scripts/codemirror-entry.js scripts/build-codemirror.js ui/js/vendor/codemirror-bundle.esm.js ui/code_studio_assets_test.go
node -e "console.log(require('fs').statSync('ui/js/vendor/codemirror-bundle.esm.js').size)"
```
Expected: the status prints nothing (stop otherwise), and the second command prints a byte count. Note it.

- [ ] **Step 2: Install (shared `node_modules`)**

```
npm install --save-exact @codemirror/lsp-client@6.3.1
```
Expected: `added N packages` (about 5: lsp-client, nested `marked` 15, `vscode-languageserver-protocol`, `vscode-jsonrpc`, `vscode-languageserver-types`). No errors.

- [ ] **Step 3: Verify the tree and the manifest diff**

```
npm ls @codemirror/lsp-client marked @codemirror/view @codemirror/state @codemirror/language @codemirror/autocomplete @codemirror/lint @lezer/highlight
git diff --stat -- package.json package-lock.json
git diff -- package.json
```
Expected:
- `@codemirror/lsp-client@6.3.1` is a direct dependency.
- Under it, `marked@15.0.12` sits next to the root `marked@18.1.1`.
- Every `@codemirror/*` and `@lezer/highlight` under it says `deduped`. A nested copy of `@codemirror/view` or `@codemirror/state` would break the editor; stop if one appears.
- No `invalid` or `extraneous` on these packages.
- `package.json` gains exactly `"@codemirror/lsp-client": "6.3.1",` between `@codemirror/lint` and `@codemirror/search`.
- `package-lock.json` only adds entries. If npm rewrote unrelated versions, stop and report.

- [ ] **Step 4: Export the pieces from `scripts/codemirror-entry.js`**

Replace:
```
export { EditorState, Prec, Compartment } from '@codemirror/state';
```
With:
```
export { EditorState, Prec, Compartment, Text, ChangeSet } from '@codemirror/state';
```

Replace:
```
export { lintKeymap, linter, lintGutter, setDiagnostics } from '@codemirror/lint';
```
With:
```
export { lintKeymap, linter, lintGutter, setDiagnostics } from '@codemirror/lint';
// Code Studio IDE package 2b: the LSP client core (ui/js/desktop/apps/code-studio/lsp.js). References, rename and
// formatting of the library stay out; package 2c brings its own UI for them.
export {
  LSPClient,
  LSPPlugin,
  Workspace,
  serverCompletion,
  hoverTooltips,
  signatureHelp,
  showSignatureHelp,
  serverDiagnostics,
  jumpToDefinition,
  jumpToDefinitionKeymap,
} from '@codemirror/lsp-client';
```
The `Alt+Click` change needs no export. `EditorView.clickAddsSelectionRange`, `domEventHandlers` and `exceptionSink` are static members of `EditorView`.

- [ ] **Step 5: Extend the package list in `scripts/build-codemirror.js`**

Replace:
```
  '@codemirror/lint',
];
```
With:
```
  '@codemirror/lint',
  '@codemirror/lsp-client',
];
```

- [ ] **Step 6: Check the new export names in `ui/code_studio_assets_test.go`**

Impact first: `impact "TestCodeMirrorBundleExportsYAMLAndLint" --direction upstream` (a test; no callers expected).

Replace:
```go
		"sql", "java", "php", "xml", "shell", "dockerFile", "toml",
	} {
```
With:
```go
		"sql", "java", "php", "xml", "shell", "dockerFile", "toml",
		// Code Studio IDE package 2b: the LSP client core and the state pieces its workspace needs.
		"LSPClient", "LSPPlugin", "Workspace", "serverCompletion", "hoverTooltips", "signatureHelp", "showSignatureHelp",
		"serverDiagnostics", "jumpToDefinition", "jumpToDefinitionKeymap", "Text", "ChangeSet",
	} {
```

- [ ] **Step 7: Build and verify (offline)**

```
npm run build:codemirror
node -e "const s=require('fs').readFileSync('ui/js/vendor/codemirror-bundle.esm.js','utf8');console.log(s.length,(s.match(/class EditorView \{/g)||[]).length,(s.match(/class EditorState \{/g)||[]).length)"
GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./ui -count=1 -v -run '(?i)^Test(CodeMirrorBundleExportsYAMLAndLint|CodeStudioEditorBundleIsEmbedded|ShippedWebAssetsLoadNothingExternal)$'
```
Expected:
- The build prints `Including 24 CodeMirror packages` and `CodeMirror bundle written to ui/js/vendor/codemirror-bundle.esm.js`.
- The node line prints `<size> 1 1`.
- Growth is about 110 to 180 KB over Step 1: lsp-client about 60 KB, marked 15 about 45 to 60 KB, small view and highlight additions. More than 300 KB means a duplicate package; stop.
- 3 `--- PASS` lines.

- [ ] **Step 8: Commit**

Run `detect-changes --scope all`. Message `msg-task1.txt`:
```
feat(code-studio): bundle @codemirror/lsp-client 6.3.1 for code intelligence

Adds the official CodeMirror LSP client (marked 15 stays nested under it) and
exports the client core, Text and ChangeSet for Code Studio IDE package 2b.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task1.txt package.json package-lock.json scripts/codemirror-entry.js scripts/build-codemirror.js ui/js/vendor/codemirror-bundle.esm.js ui/code_studio_assets_test.go
```

---

### Task 2: Pure protocol module `lsp-protocol.js` and its node test

**Files:**
- Create: `ui/js/desktop/apps/code-studio/lsp-protocol.js`, `ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs`

The module is not in the bundle yet; Task 5 adds it.

- [ ] **Step 1: Create `ui/js/desktop/apps/code-studio/lsp-protocol.js`**

```js
/* Code Studio LSP protocol helpers (IDE package 2b): language mapping, workspace URIs, LSP diagnostics to Code
   Studio problems and LSP positions. Pure functions without DOM access. The same file runs inside the code-studio
   bundle (window.CodeStudioLSPProtocol) and under node (lsp-protocol.test.cjs). */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioLSPProtocol = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    var ROOT_URI = 'file:///workspace';
    var LANGUAGES = ['go', 'typescript', 'python', 'cpp'];
    // One problem source per server, distinct from the container checks (go build, tsc, py_compile, gcc, ...).
    var SOURCES = { go: 'gopls', typescript: 'typescript', python: 'pyright', cpp: 'clangd' };
    var LANGUAGE_BY_EXTENSION = {
        go: 'go',
        ts: 'typescript', tsx: 'typescript', js: 'typescript', jsx: 'typescript', mjs: 'typescript', cjs: 'typescript',
        py: 'python',
        c: 'cpp', cc: 'cpp', cpp: 'cpp', cxx: 'cpp', h: 'cpp', hpp: 'cpp'
    };
    // LSP language IDs. typescript-language-server takes the script kind (TS, TSX, JS, JSX) from them.
    var LANGUAGE_ID_BY_EXTENSION = {
        go: 'go',
        ts: 'typescript', tsx: 'typescriptreact', js: 'javascript', jsx: 'javascriptreact', mjs: 'javascript', cjs: 'javascript',
        py: 'python',
        c: 'c', h: 'c', cc: 'cpp', cpp: 'cpp', cxx: 'cpp', hpp: 'cpp'
    };
    // LSP DiagnosticSeverity 1 Error, 2 Warning, 3 Information, 4 Hint; the Problems panel knows error, warning, info.
    var SEVERITIES = { 1: 'error', 2: 'warning', 3: 'info', 4: 'info' };

    function own(map, key) {
        return Object.prototype.hasOwnProperty.call(map, key);
    }

    function extensionOf(path) {
        var name = String(path == null ? '' : path).split('/').pop();
        var dot = name.lastIndexOf('.');
        return dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
    }

    function languageForPath(path) {
        var ext = extensionOf(path);
        return own(LANGUAGE_BY_EXTENSION, ext) ? LANGUAGE_BY_EXTENSION[ext] : null;
    }

    function languageIdForPath(path) {
        var ext = extensionOf(path);
        return own(LANGUAGE_ID_BY_EXTENSION, ext) ? LANGUAGE_ID_BY_EXTENSION[ext] : null;
    }

    function sourceForLanguage(language) {
        return own(SOURCES, language) ? SOURCES[language] : '';
    }

    // An absolute container path below /workspace without dot segments, or null.
    function cleanPath(path) {
        var value = typeof path === 'string' ? path : '';
        if (!value || value.charAt(0) !== '/' || value.indexOf('\\') !== -1 || value.indexOf('\0') !== -1) return null;
        var parts = value.split('/').filter(function (part) { return part !== ''; });
        if (!parts.length || parts[0] !== 'workspace') return null;
        for (var i = 0; i < parts.length; i++) {
            if (parts[i] === '.' || parts[i] === '..') return null;
        }
        return '/' + parts.join('/');
    }

    // encodeURIComponent plus !'()*, the way vscode-uri (TypeScript and Pyright servers) writes file URIs.
    function encodeSegment(segment) {
        return encodeURIComponent(segment).replace(/[!'()*]/g, function (ch) {
            return '%' + ch.charCodeAt(0).toString(16).toUpperCase();
        });
    }

    function uriForPath(path) {
        var clean = cleanPath(path);
        return clean ? 'file://' + clean.split('/').map(encodeSegment).join('/') : null;
    }

    function pathForUri(uri) {
        var match = /^file:\/\/(?:localhost)?(\/[^?#]*)$/i.exec(typeof uri === 'string' ? uri : '');
        if (!match) return null;
        var segments = match[1].split('/');
        for (var i = 0; i < segments.length; i++) {
            var decoded;
            try {
                decoded = decodeURIComponent(segments[i]);
            } catch (err) {
                return null;
            }
            if (decoded.indexOf('/') !== -1) return null;
            segments[i] = decoded;
        }
        return cleanPath(segments.join('/'));
    }

    function count(value) {
        var number = Math.floor(Number(value));
        return isFinite(number) && number > 0 ? number : 0;
    }

    // The diagnostic as the server sent it: quick fixes (plan 2c) send it back in the textDocument/codeAction context.
    function rawDiagnostic(item, start, end) {
        var raw = {
            range: {
                start: { line: start.line, character: start.character },
                end: { line: end.line, character: end.character }
            },
            message: item.message
        };
        if (item.severity !== undefined) raw.severity = item.severity;
        if (item.code !== undefined) raw.code = item.code;
        if (item.source !== undefined) raw.source = item.source;
        if (item.data !== undefined) raw.data = item.data;
        return raw;
    }

    function diagnosticsToProblems(path, diagnostics, source) {
        var file = String(path == null ? '' : path);
        var origin = String(source == null ? '' : source);
        var problems = [];
        (Array.isArray(diagnostics) ? diagnostics : []).forEach(function (item) {
            if (!item || !item.range || !item.range.start) return;
            var message = String(item.message == null ? '' : item.message).trim();
            if (!message) return;
            var start = item.range.start;
            var end = item.range.end || start;
            problems.push({
                file: file,
                line: count(start.line) + 1,
                column: count(start.character) + 1,
                endLine: count(end.line) + 1,
                endColumn: count(end.character) + 1,
                severity: own(SEVERITIES, item.severity) ? SEVERITIES[item.severity] : 'error',
                message: message,
                source: origin,
                lsp: rawDiagnostic(item, start, end)
            });
        });
        return problems;
    }

    // LSP positions count UTF-16 code units, like CodeMirror offsets, so no conversion beyond the line lookup.
    function positionToOffset(doc, position) {
        if (!doc || !position) return 0;
        var line = Math.floor(Number(position.line));
        if (!isFinite(line) || line < 0) return 0;
        if (line >= doc.lines) return doc.length;
        var info = doc.line(line + 1);
        var character = Math.floor(Number(position.character));
        if (!isFinite(character) || character < 0) character = 0;
        return info.from + Math.min(character, info.to - info.from);
    }

    function offsetToPosition(doc, offset) {
        if (!doc) return { line: 0, character: 0 };
        var value = Math.floor(Number(offset));
        if (!isFinite(value) || value < 0) value = 0;
        if (value > doc.length) value = doc.length;
        var info = doc.lineAt(value);
        return { line: info.number - 1, character: value - info.from };
    }

    return {
        ROOT_URI: ROOT_URI,
        LANGUAGES: LANGUAGES.slice(),
        SOURCES: { go: SOURCES.go, typescript: SOURCES.typescript, python: SOURCES.python, cpp: SOURCES.cpp },
        languageForPath: languageForPath,
        languageIdForPath: languageIdForPath,
        sourceForLanguage: sourceForLanguage,
        uriForPath: uriForPath,
        pathForUri: pathForUri,
        diagnosticsToProblems: diagnosticsToProblems,
        positionToOffset: positionToOffset,
        offsetToPosition: offsetToPosition
    };
});
```

- [ ] **Step 2: Create `ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs`**

```js
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module, like notes-index.test.cjs.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'lsp-protocol.js'), 'utf8'), { module: mod, window: {} });
const P = mod.exports;
// Objects from the vm realm have another Object.prototype, so deep comparisons go through JSON.
const json = value => JSON.parse(JSON.stringify(value));

// A stand-in for CodeMirror's Text: lines, length, line(n) and lineAt(offset) with number, from, to, text.
function fakeText(content) {
    const lines = content.split('\n');
    const starts = [];
    let pos = 0;
    for (const line of lines) {
        starts.push(pos);
        pos += line.length + 1;
    }
    return {
        length: content.length,
        lines: lines.length,
        line(n) {
            const text = lines[n - 1];
            return { number: n, from: starts[n - 1], to: starts[n - 1] + text.length, text };
        },
        lineAt(offset) {
            let n = lines.length;
            while (n > 1 && starts[n - 1] > offset) n -= 1;
            return this.line(n);
        }
    };
}

test('languageForPath maps the four servers and nothing else', () => {
    assert.equal(P.languageForPath('/workspace/cmd/main.go'), 'go');
    for (const ext of ['ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs']) assert.equal(P.languageForPath('/workspace/web/app.' + ext), 'typescript');
    assert.equal(P.languageForPath('/workspace/tool.PY'), 'python');
    for (const ext of ['c', 'cc', 'cpp', 'cxx', 'h', 'hpp']) assert.equal(P.languageForPath('/workspace/src/x.' + ext), 'cpp');
    for (const name of ['/workspace/README.md', '/workspace/Makefile', '/workspace/.bashrc', '/workspace/lib.rs', '', null]) {
        assert.equal(P.languageForPath(name), null, String(name));
    }
    assert.deepEqual(json(P.LANGUAGES), ['go', 'typescript', 'python', 'cpp']);
    assert.equal(P.ROOT_URI, 'file:///workspace');
});

test('languageIdForPath and sourceForLanguage', () => {
    assert.equal(P.languageIdForPath('/workspace/a.go'), 'go');
    assert.equal(P.languageIdForPath('/workspace/a.ts'), 'typescript');
    assert.equal(P.languageIdForPath('/workspace/a.tsx'), 'typescriptreact');
    assert.equal(P.languageIdForPath('/workspace/a.jsx'), 'javascriptreact');
    assert.equal(P.languageIdForPath('/workspace/a.mjs'), 'javascript');
    assert.equal(P.languageIdForPath('/workspace/a.py'), 'python');
    assert.equal(P.languageIdForPath('/workspace/a.h'), 'c');
    assert.equal(P.languageIdForPath('/workspace/a.hpp'), 'cpp');
    assert.equal(P.languageIdForPath('/workspace/a.txt'), null);
    assert.deepEqual(['go', 'typescript', 'python', 'cpp'].map(P.sourceForLanguage), ['gopls', 'typescript', 'pyright', 'clangd']);
    assert.deepEqual(json(P.SOURCES), { go: 'gopls', typescript: 'typescript', python: 'pyright', cpp: 'clangd' });
    assert.equal(P.sourceForLanguage('rust'), '');
});

test('uriForPath percent-encodes segments and stays inside /workspace', () => {
    assert.equal(P.uriForPath('/workspace'), 'file:///workspace');
    assert.equal(P.uriForPath('/workspace/cmd/main.go'), 'file:///workspace/cmd/main.go');
    assert.equal(P.uriForPath("/workspace/my dir/it's #1 (copy).go"), 'file:///workspace/my%20dir/it%27s%20%231%20%28copy%29.go');
    assert.equal(P.uriForPath('/workspace/' + String.fromCodePoint(0xe4) + '.py'), 'file:///workspace/%C3%A4.py');
    assert.equal(P.uriForPath('//workspace///a.go'), 'file:///workspace/a.go');
    for (const bad of ['/etc/passwd', '/workspacex/a.go', '/workspace/../etc', '/workspace/./a.go', 'workspace/a.go', '/workspace/a\\b.go', '', null]) {
        assert.equal(P.uriForPath(bad), null, String(bad));
    }
});

test('pathForUri decodes workspace URIs and rejects everything else', () => {
    assert.equal(P.pathForUri('file:///workspace/my%20dir/it%27s%20%231.go'), "/workspace/my dir/it's #1.go");
    assert.equal(P.pathForUri('file://localhost/workspace/a.go'), '/workspace/a.go');
    assert.equal(P.pathForUri('FILE:///workspace/a.go'), '/workspace/a.go');
    assert.equal(P.pathForUri(P.uriForPath('/workspace/x y/z.ts')), '/workspace/x y/z.ts');
    for (const bad of [
        'file:///etc/passwd', 'file:///workspace/../etc/passwd', 'file:///workspace/a%2Fb.go', 'file:///workspace/%E0%A4%A.go',
        'https://example.com/workspace/a.go', 'file:///workspace/a.go?x=1', 'file:///C:/workspace/a.go', 'untitled:Untitled-1', '', null
    ]) {
        assert.equal(P.pathForUri(bad), null, String(bad));
    }
});

test('diagnosticsToProblems maps severity, 1-based positions, the source and keeps the raw diagnostic', () => {
    const problems = P.diagnosticsToProblems('/workspace/main.go', [
        { range: { start: { line: 8, character: 1 }, end: { line: 8, character: 5 } }, severity: 1, code: 'UndeclaredName', message: 'undefined: fmtx', source: 'compiler', data: { fix: 1 } },
        { range: { start: { line: 0, character: 0 }, end: { line: 0, character: 7 } }, severity: 2, message: '  unused import  ' },
        { range: { start: { line: 3, character: 2 }, end: { line: 4, character: 0 } }, severity: 3, code: 2304, message: 'info' },
        { range: { start: { line: 5, character: 4 }, end: { line: 5, character: 4 } }, severity: 4, message: 'hint' },
        { range: { start: { line: 6, character: 0 }, end: { line: 6, character: 1 } }, message: 'no severity' },
        { range: { start: { line: 7, character: 0 } }, severity: 1, message: '' },
        { message: 'no range' },
        null
    ], 'gopls');
    const range = (a, b, c, d) => ({ start: { line: a, character: b }, end: { line: c, character: d } });
    assert.deepEqual(json(problems), [
        { file: '/workspace/main.go', line: 9, column: 2, endLine: 9, endColumn: 6, severity: 'error', message: 'undefined: fmtx', source: 'gopls',
            lsp: { range: range(8, 1, 8, 5), message: 'undefined: fmtx', severity: 1, code: 'UndeclaredName', source: 'compiler', data: { fix: 1 } } },
        { file: '/workspace/main.go', line: 1, column: 1, endLine: 1, endColumn: 8, severity: 'warning', message: 'unused import', source: 'gopls',
            lsp: { range: range(0, 0, 0, 7), message: '  unused import  ', severity: 2 } },
        { file: '/workspace/main.go', line: 4, column: 3, endLine: 5, endColumn: 1, severity: 'info', message: 'info', source: 'gopls',
            lsp: { range: range(3, 2, 4, 0), message: 'info', severity: 3, code: 2304 } },
        { file: '/workspace/main.go', line: 6, column: 5, endLine: 6, endColumn: 5, severity: 'info', message: 'hint', source: 'gopls',
            lsp: { range: range(5, 4, 5, 4), message: 'hint', severity: 4 } },
        { file: '/workspace/main.go', line: 7, column: 1, endLine: 7, endColumn: 2, severity: 'error', message: 'no severity', source: 'gopls',
            lsp: { range: range(6, 0, 6, 1), message: 'no severity' } }
    ]);
    assert.deepEqual(json(P.diagnosticsToProblems('/workspace/a.py', 'not a list', 'pyright')), []);
});

test('positionToOffset and offsetToPosition use UTF-16 units and clamp', () => {
    // Line 1 starts with a surrogate pair: the emoji takes two UTF-16 units, as in LSP and CodeMirror.
    const doc = fakeText('ab\n' + String.fromCodePoint(0x1F600) + 'x\n\nend');
    assert.equal(doc.length, 11);
    assert.equal(P.positionToOffset(doc, { line: 1, character: 2 }), 5);
    assert.deepEqual(json(P.offsetToPosition(doc, 5)), { line: 1, character: 2 });
    assert.equal(P.positionToOffset(doc, { line: 0, character: 99 }), 2);
    assert.equal(P.positionToOffset(doc, { line: 99, character: 0 }), 11);
    assert.equal(P.positionToOffset(doc, { line: -1, character: 3 }), 0);
    assert.equal(P.positionToOffset(doc, { line: 3, character: 3 }), 11);
    assert.deepEqual(json(P.offsetToPosition(doc, 0)), { line: 0, character: 0 });
    assert.deepEqual(json(P.offsetToPosition(doc, 7)), { line: 2, character: 0 });
    assert.deepEqual(json(P.offsetToPosition(doc, 999)), { line: 3, character: 3 });
    assert.deepEqual(json(P.offsetToPosition(doc, -4)), { line: 0, character: 0 });
    for (let offset = 0; offset <= doc.length; offset += 1) {
        assert.equal(P.positionToOffset(doc, P.offsetToPosition(doc, offset)), offset, 'round trip ' + offset);
    }
});
```

- [ ] **Step 3: Run the node test**

```
node --test ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs
```
Expected: `# tests 6`, `# pass 6`, `# fail 0`.

- [ ] **Step 4: Commit**

Run `detect-changes --scope all`. Message `msg-task2.txt`:
```
feat(code-studio): add LSP protocol helpers with a node test

Pure module for the LSP client core: language mapping, workspace file URIs,
publishDiagnostics to problems (with the raw diagnostic for code actions)
and UTF-16 positions.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task2.txt ui/js/desktop/apps/code-studio/lsp-protocol.js ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs
```

---

### Task 3: Locale strings (all 16 languages)

**Files:**
- Create (not committed): `C:\disposable\code-studio-pkg2b\locales-2b.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

The script inserts the 10 keys as one block right after the last `codeStudio.*` line, as plan 1A did. It keeps the BOM, the line endings, the indentation and the order. It skips existing keys and checks placeholders against English. `codeStudio.lspStatus` is a format string, so most languages share it; French adds the space before the colon, and Japanese and Chinese use the full-width colon.

- [ ] **Step 1: Create `C:\disposable\code-studio-pkg2b\locales-2b.mjs`** (create the folder first)

```js
// Adds the Code Studio IDE package 2b keys (LSP client core) to the 16 desktop locale files.
// Usage: node C:\disposable\code-studio-pkg2b\locales-2b.mjs C:\Users\Andi\Documents\repo\AuraGo
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2];
if (!root) throw new Error('usage: node locales-2b.mjs <repo root>');
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

const KEYS = {
  'codeStudio.settingsCodeIntelligence': {
    cs: "Inteligence kódu (LSP)", da: "Kodeintelligens (LSP)", de: "Code-Intelligenz (LSP)", el: "Ευφυΐα κώδικα (LSP)",
    en: "Code intelligence (LSP)", es: "Inteligencia de código (LSP)", fr: "Intelligence du code (LSP)", hi: "कोड इंटेलिजेंस (LSP)",
    it: "Intelligenza del codice (LSP)", ja: "コードインテリジェンス (LSP)", nl: "Code-intelligentie (LSP)", no: "Kodeintelligens (LSP)",
    pl: "Inteligencja kodu (LSP)", pt: "Inteligência de código (LSP)", sv: "Kodintelligens (LSP)", zh: "代码智能 (LSP)"
  },
  'codeStudio.lspStatus': {
    cs: "{{server}}: {{state}}", da: "{{server}}: {{state}}", de: "{{server}}: {{state}}", el: "{{server}}: {{state}}",
    en: "{{server}}: {{state}}", es: "{{server}}: {{state}}", fr: "{{server}} : {{state}}", hi: "{{server}}: {{state}}",
    it: "{{server}}: {{state}}", ja: "{{server}}：{{state}}", nl: "{{server}}: {{state}}", no: "{{server}}: {{state}}",
    pl: "{{server}}: {{state}}", pt: "{{server}}: {{state}}", sv: "{{server}}: {{state}}", zh: "{{server}}：{{state}}"
  },
  'codeStudio.lspStarting': {
    cs: "Spouští se...", da: "Starter...", de: "Startet...", el: "Εκκίνηση...",
    en: "Starting...", es: "Iniciando...", fr: "Démarrage...", hi: "शुरू हो रहा है...",
    it: "Avvio...", ja: "起動中...", nl: "Starten...", no: "Starter...",
    pl: "Uruchamianie...", pt: "A iniciar...", sv: "Startar...", zh: "正在启动..."
  },
  'codeStudio.lspReady': {
    cs: "Připraveno", da: "Klar", de: "Bereit", el: "Έτοιμο",
    en: "Ready", es: "Listo", fr: "Prêt", hi: "तैयार",
    it: "Pronto", ja: "準備完了", nl: "Gereed", no: "Klar",
    pl: "Gotowy", pt: "Pronto", sv: "Redo", zh: "就绪"
  },
  'codeStudio.lspIdle': {
    cs: "Nečinný", da: "Inaktiv", de: "Im Leerlauf", el: "Αδρανές",
    en: "Idle", es: "Inactivo", fr: "Inactif", hi: "निष्क्रिय",
    it: "Inattivo", ja: "アイドル", nl: "Inactief", no: "Inaktiv",
    pl: "Bezczynny", pt: "Inativo", sv: "Vilande", zh: "空闲"
  },
  'codeStudio.lspDisconnected': {
    cs: "Odpojeno", da: "Afbrudt", de: "Getrennt", el: "Αποσυνδέθηκε",
    en: "Disconnected", es: "Desconectado", fr: "Déconnecté", hi: "डिस्कनेक्ट हो गया",
    it: "Disconnesso", ja: "切断されました", nl: "Verbinding verbroken", no: "Frakoblet",
    pl: "Rozłączono", pt: "Desligado", sv: "Frånkopplad", zh: "已断开连接"
  },
  'codeStudio.lspNotInstalled': {
    cs: "Jazykový server není nainstalován — znovu sestavte image Code Studia",
    da: "Sprogserveren er ikke installeret — byg Code Studio-imaget igen",
    de: "Sprachserver nicht installiert — Code-Studio-Image neu bauen",
    el: "Ο διακομιστής γλώσσας δεν είναι εγκατεστημένος — δημιουργήστε ξανά την εικόνα του Code Studio",
    en: "Language server not installed — rebuild the Code Studio image",
    es: "Servidor de lenguaje no instalado — vuelve a compilar la imagen de Code Studio",
    fr: "Serveur de langage non installé — reconstruisez l'image Code Studio",
    hi: "भाषा सर्वर इंस्टॉल नहीं है — Code Studio इमेज फिर से बनाएँ",
    it: "Server di linguaggio non installato — ricompila l'immagine di Code Studio",
    ja: "言語サーバーがインストールされていません — Code Studio イメージを再ビルドしてください",
    nl: "Taalserver niet geïnstalleerd — bouw de Code Studio-image opnieuw",
    no: "Språkserveren er ikke installert — bygg Code Studio-bildet på nytt",
    pl: "Serwer języka nie jest zainstalowany — przebuduj obraz Code Studio",
    pt: "Servidor de linguagem não instalado — reconstrua a imagem do Code Studio",
    sv: "Språkservern är inte installerad — bygg om Code Studio-avbildningen",
    zh: "未安装语言服务器 — 请重新构建 Code Studio 镜像"
  },
  'codeStudio.lspTooMany': {
    cs: "Příliš mnoho jazykových serverů", da: "For mange sprogservere", de: "Zu viele Sprachserver", el: "Πάρα πολλοί διακομιστές γλώσσας",
    en: "Too many language servers", es: "Demasiados servidores de lenguaje", fr: "Trop de serveurs de langage", hi: "बहुत अधिक भाषा सर्वर",
    it: "Troppi server di linguaggio", ja: "言語サーバーが多すぎます", nl: "Te veel taalservers", no: "For mange språkservere",
    pl: "Zbyt wiele serwerów języka", pt: "Demasiados servidores de linguagem", sv: "För många språkservrar", zh: "语言服务器过多"
  },
  'codeStudio.lspError': {
    cs: "Chyba — klikněte pro restart", da: "Fejl — klik for at genstarte", de: "Fehler — zum Neustart klicken", el: "Σφάλμα — κάντε κλικ για επανεκκίνηση",
    en: "Error — click to restart", es: "Error — haz clic para reiniciar", fr: "Erreur — cliquez pour redémarrer", hi: "त्रुटि — फिर से शुरू करने के लिए क्लिक करें",
    it: "Errore — fai clic per riavviare", ja: "エラー — クリックして再起動", nl: "Fout — klik om opnieuw te starten", no: "Feil — klikk for å starte på nytt",
    pl: "Błąd — kliknij, aby uruchomić ponownie", pt: "Erro — clique para reiniciar", sv: "Fel — klicka för att starta om", zh: "错误 — 点击重新启动"
  },
  'codeStudio.lspRestartHint': {
    cs: "Kliknutím restartujete jazykový server", da: "Klik for at genstarte sprogserveren", de: "Klicken, um den Sprachserver neu zu starten",
    el: "Κάντε κλικ για επανεκκίνηση του διακομιστή γλώσσας", en: "Click to restart the language server",
    es: "Haz clic para reiniciar el servidor de lenguaje", fr: "Cliquez pour redémarrer le serveur de langage",
    hi: "भाषा सर्वर फिर से शुरू करने के लिए क्लिक करें", it: "Fai clic per riavviare il server di linguaggio",
    ja: "クリックして言語サーバーを再起動", nl: "Klik om de taalserver opnieuw te starten", no: "Klikk for å starte språkserveren på nytt",
    pl: "Kliknij, aby ponownie uruchomić serwer języka", pt: "Clique para reiniciar o servidor de linguagem",
    sv: "Klicka för att starta om språkservern", zh: "点击重新启动语言服务器"
  }
};

const placeholders = text => (String(text).match(/\{\{\w+\}\}/g) || []).sort().join(',');
for (const [key, values] of Object.entries(KEYS)) {
  for (const lang of LANGS) {
    if (typeof values[lang] !== 'string' || !values[lang].trim()) throw new Error(`${key}: missing ${lang}`);
    if (placeholders(values[lang]) !== placeholders(values.en)) throw new Error(`${key}: ${lang} placeholders differ from en`);
  }
}

for (const lang of LANGS) {
  const file = join(root, 'ui', 'lang', 'desktop', `${lang}.json`);
  const raw = readFileSync(file, 'utf8');
  const bom = raw.charCodeAt(0) === 0xFEFF ? raw.slice(0, 1) : '';
  const text = bom ? raw.slice(1) : raw;
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const existing = JSON.parse(text);
  const lines = text.split(eol);
  let last = -1;
  lines.forEach((line, index) => { if (/^\s*"codeStudio\.[^"]+"\s*:/.test(line)) last = index; });
  if (last < 0) throw new Error(`${lang}: no codeStudio.* key found`);
  const indent = /^\s*/.exec(lines[last])[0];
  const additions = Object.keys(KEYS)
    .filter(key => !Object.prototype.hasOwnProperty.call(existing, key))
    .map(key => `${indent}${JSON.stringify(key)}: ${JSON.stringify(KEYS[key][lang])}`);
  if (!additions.length) {
    console.log(`${lang}: nothing to add`);
    continue;
  }
  const lastHadComma = /,\s*$/.test(lines[last]);
  if (!lastHadComma) lines[last] = lines[last].replace(/\s*$/, ',');
  const block = additions.map((line, index) => (index < additions.length - 1 || lastHadComma ? line + ',' : line));
  lines.splice(last + 1, 0, ...block);
  const next = lines.join(eol);
  const parsed = JSON.parse(next);
  for (const key of Object.keys(KEYS)) {
    const want = Object.prototype.hasOwnProperty.call(existing, key) ? existing[key] : KEYS[key][lang];
    if (parsed[key] !== want) throw new Error(`${lang}: ${key} was not written`);
  }
  writeFileSync(file, bom + next, 'utf8');
  console.log(`${lang}: added ${additions.length} keys`);
}
```

- [ ] **Step 2: Check for foreign edits, run once**

```
git status --short -- ui/lang/desktop
node C:/disposable/code-studio-pkg2b/locales-2b.mjs C:/Users/Andi/Documents/repo/AuraGo
```
Expected: the status prints nothing (stop otherwise), then 16 lines `<lang>: added 10 keys`.

- [ ] **Step 3: Verify that the locale files parse**

```
GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./ui -count=1 -v -run '(?i)^TestDesktopCodeStudio(ShellI18n|CopyI18n|CursorI18n)$'
```
Expected: 3 `--- PASS` lines.

- [ ] **Step 4: Commit**

Run `detect-changes --scope all`. Message `msg-task3.txt`:
```
feat(code-studio): add LSP client locale strings

Status, restart hint and the "Code intelligence (LSP)" setting in all 16
desktop languages.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task3.txt ui/lang/desktop/cs.json ui/lang/desktop/da.json ui/lang/desktop/de.json ui/lang/desktop/el.json ui/lang/desktop/en.json ui/lang/desktop/es.json ui/lang/desktop/fr.json ui/lang/desktop/hi.json ui/lang/desktop/it.json ui/lang/desktop/ja.json ui/lang/desktop/nl.json ui/lang/desktop/no.json ui/lang/desktop/pl.json ui/lang/desktop/pt.json ui/lang/desktop/sv.json ui/lang/desktop/zh.json
```

---

### Task 4: Problems: `replaceProblemsForFile`, raw diagnostics and whole-range squiggles

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/problems.js`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Impact check.** Run `impact` upstream for `normalizeProblem` and `applyDiagnosticsToTab`. Expected callers:
  - `normalizeProblem`: `setProblems`, `addProblems`, `checkActiveDocument`, `parseDiagnosticOutput`
  - `applyDiagnosticsToTab`: `applyDiagnosticsToOpenTabs`

  Report the risk.

- [ ] **Step 2: Keep the end and the raw LSP diagnostic in `normalizeProblem`**

Replace:
```js
    function normalizeProblem(problem) {
        const severity = ['error', 'warning', 'info'].includes(problem.severity) ? problem.severity : 'error';
        return {
            file: String(problem.file || ''),
            line: Math.max(1, Number(problem.line) || 1),
            column: Math.max(1, Number(problem.column) || 1),
            severity,
            message: String(problem.message || '').trim(),
            source: String(problem.source || '')
        };
    }
```
With:
```js
    function normalizeProblem(problem) {
        const severity = ['error', 'warning', 'info'].includes(problem.severity) ? problem.severity : 'error';
        const normalized = {
            file: String(problem.file || ''),
            line: Math.max(1, Number(problem.line) || 1),
            column: Math.max(1, Number(problem.column) || 1),
            severity,
            message: String(problem.message || '').trim(),
            source: String(problem.source || '')
        };
        // IDE package 2b: language servers report where a finding ends; the squiggle then covers the whole range.
        const endLine = Number(problem.endLine);
        const endColumn = Number(problem.endColumn);
        if (Number.isInteger(endLine) && Number.isInteger(endColumn)
            && (endLine > normalized.line || (endLine === normalized.line && endColumn > normalized.column))) {
            normalized.endLine = endLine;
            normalized.endColumn = endColumn;
        }
        // IDE package 2b: the original LSP diagnostic travels with the problem, for textDocument/codeAction (plan 2c).
        if (problem.lsp && typeof problem.lsp === 'object') normalized.lsp = problem.lsp;
        return normalized;
    }
```

- [ ] **Step 3: Add `replaceProblemsForFile` after `clearProblems`**

Replace:
```js
    function clearProblems() {
        setProblems([]);
    }
```
With:
```js
    function clearProblems() {
        setProblems([]);
    }

    // IDE package 2b: a language server always publishes the complete list of one file, so its findings for that file
    // replace the earlier ones of the same source; other files and sources stay. List, squiggles and status counters
    // keep coming from state.problems.
    function replaceProblemsForFile(file, source, problems) {
        const path = String(file || '');
        const origin = String(source || '');
        const current = state.problems || [];
        const others = current.filter(problem => !(problem.file === path && problem.source === origin));
        const mine = (problems || []).map(problem => ({ ...problem, file: path, source: origin }));
        if (!mine.length && others.length === current.length) return;
        setProblems([...others, ...mine]);
    }
```

- [ ] **Step 4: Paint the whole range in `applyDiagnosticsToTab`**

Replace:
```js
            const from = Math.min(docLine.to, docLine.from + Math.max(0, problem.column - 1));
            const to = Math.min(docLine.to, Math.max(from + 1, docLine.from + Math.max(0, problem.column - 1)));
            return {
```
With:
```js
            const from = Math.min(docLine.to, docLine.from + Math.max(0, problem.column - 1));
            let to = Math.min(docLine.to, Math.max(from + 1, docLine.from + Math.max(0, problem.column - 1)));
            // IDE package 2b: a finding with an end (language servers) is painted over its whole range.
            if (problem.endLine) {
                const endDocLine = doc.line(Math.min(Math.max(lineNumber, problem.endLine), doc.lines));
                to = Math.max(to, Math.min(endDocLine.to, endDocLine.from + Math.max(0, problem.endColumn - 1)));
            }
            return {
```

- [ ] **Step 5: Foreign-edit check, build, parse check**

```
git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
npm run build:ui
npm run build:ui -- --check
node -e "new (require('vm').Script)(require('fs').readFileSync('ui/js/desktop/bundles/code-studio.bundle.js','utf8'),{filename:'code-studio.bundle.js'})"
```
Expected:
- Only ` M ui/js/desktop/apps/code-studio/problems.js` before the build.
- `Built ui/js/desktop/bundles/code-studio.bundle.js (18 parts)` and `Checked ... (18 parts)`.
- The vm parse prints nothing.

- [ ] **Step 6: Commit**

Run `detect-changes --scope all`. Message `msg-task4.txt`:
```
feat(code-studio): replace problems per file and source, paint whole ranges

replaceProblemsForFile lets a language server publish one file's findings;
problems keep their end position and the raw LSP diagnostic (problem.lsp)
and underline their whole range.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task4.txt ui/js/desktop/apps/code-studio/problems.js ui/js/desktop/bundles/code-studio.bundle.js
```

---

### Task 5: The LSP client core `lsp.js` and the bundle list

**Files:**
- Create: `ui/js/desktop/apps/code-studio/lsp.js`
- Modify: `scripts/build-ui-bundles.js`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

How it works:
- **States and sessions.** The contract states are `off`, `starting`, `ready`, `disconnected`, `not-installed`, `too-many` and `error`. A session is one `LSPClient`. It survives reconnects and the idle close. It ends on 4404, on 4429, after the fifth failed reconnect, on a restart, and when LSP is switched off.
- **Session-ending close codes.** 4404 gives `not-installed` with no reconnect. 4429 gives `too-many`; focusing an editor of that language retries, at most every 2 s.
- **Other closes.** Any other close reconnects after 1, 2, 5, 10 and 10 s, then gives `error`. Each reconnect re-sends `didOpen` for every document and clears that language's LSP problems first.
- **Idle close.** After 10 minutes without edits or requests, the socket closes and the state becomes `off`. The client and the editor plugins stay. The next `client.sync()` reconnects: library requests and autoSync go through `Workspace.syncFiles`, as do `lspRequest`, `lspEnsureClient` and `lspTouch`.
- **Capabilities.** The client config carries the capabilities that plan 2c needs, merged through an extensions entry `{ clientCapabilities }` (6.3.1 merges every extension's `clientCapabilities`).
- **Timeouts.** The client `timeout` is 15000 ms, so 2c's references and rename survive. The library's own hover, completion, signature and definition requests also get 15 s; the controller accepted this. `lspRequest` keeps its own 3 s default timer.
- **6.3.1 internal.** The plan relies on one: `client.connected` is true once a transport exists.

No editor uses `lspEditorExtensions` yet (Task 6).

- [ ] **Step 1: Create `ui/js/desktop/apps/code-studio/lsp.js`**

```js
    // IDE package 2b: the LSP client core of Code Studio.
    //
    // One @codemirror/lsp-client LSPClient per language and window talks to a language server inside the container
    // through the bridge GET /api/code-studio/lsp?lang=<language> (plan 2a). The bridge carries one JSON-RPC message
    // per text frame and closes with 4404 (not installed), 4429 (too many sessions), 4500 (start failed) or 1011
    // (server exited).
    //
    // instance.lsp.connections[language] holds one connection ("conn") in one of the contract states:
    // off | starting | ready | disconnected | not-installed | too-many | error. A session is one LSPClient. It starts
    // with the first editor of its language, survives reconnects (1, 2, 5, 10, 10 s) and the 10-minute idle close,
    // and ends on 4404, on 4429, after the last failed reconnect, on a restart and when LSP is switched off.
    //
    // Editors carry the LSP extensions in a compartment of their own (view.csLsp), attached once the session's client
    // has connected, so nothing waits on a client that never started. The Workspace subclass keeps one LSP document
    // per open tab: split views of one file share it, and tab switches (which rebuild every view) neither close nor
    // reopen documents.
    //
    // Plan 2c builds on lspEnsureClient, lspClientState, lspRequest, lspOnServerRequest, lspNotifyFilesChanged,
    // lspActionExtensionFactories, lspViewForPath and lspTouch. Names starting with lspAct / LSP_ACT_ belong to plan 2c;
    // the only one declared here is the contract array lspActionExtensionFactories.

    const LSP_PROTOCOL = window.CodeStudioLSPProtocol;
    const LSP_LANGUAGES = ['go', 'typescript', 'python', 'cpp'];
    const LSP_SERVER_NAMES = { go: 'gopls', typescript: 'TypeScript', python: 'Pyright', cpp: 'clangd' };
    // Default budget of lspRequest (hover- and completion-sized requests of plan 2c).
    const LSP_REQUEST_TIMEOUT_MS = 3000;
    // The LSPClient's own timeout: 15 s, so references and rename of plan 2c survive. It also covers initialize on a
    // cold container. The library's hover, completion, signature and definition requests share it.
    const LSP_CLIENT_TIMEOUT_MS = 15000;
    const LSP_IDLE_MS = 10 * 60 * 1000;
    // Five reconnect attempts after an unexpected close, then the state is "error".
    const LSP_BACKOFF_MS = [1000, 2000, 5000, 10000, 10000];
    const LSP_TOO_MANY_RETRY_GAP_MS = 2000;
    // The exact text 6.3.1 sends after the initialize answer (JSON.stringify keeps the key order).
    const LSP_INITIALIZED_MESSAGE = JSON.stringify({ jsonrpc: '2.0', method: 'initialized', params: {} });
    const LSP_INITIALIZE_REQUEST = /^\{"jsonrpc":"2\.0","id":\d+,"method":"initialize",/;
    const LSP_REQUEST_MESSAGE = /^\{"jsonrpc":"2\.0","id":/;
    const LSP_QUIET_ERRORS = /^(Request timed out|Client not connected|Language server disconnected)/;
    // RequestCancelled, ContentModified and ServerCancelled are routine for hover and completion.
    const LSP_QUIET_ERROR_CODES = [-32800, -32801, -32802];
    // Merged into the 6.3.1 client capabilities, which declare none of these. applyEdit, documentChanges, prepareRename
    // and the code action support serve plan 2c. Servers that can watch files keep doing it themselves (no dynamic
    // registration); Code Studio's own writes are reported with didChangeWatchedFiles either way.
    const LSP_CLIENT_CAPABILITIES = {
        workspace: {
            configuration: true,
            applyEdit: true,
            workspaceEdit: { documentChanges: true },
            didChangeWatchedFiles: { dynamicRegistration: false }
        },
        textDocument: {
            synchronization: { didSave: true },
            rename: { prepareSupport: true },
            codeAction: {
                codeActionLiteralSupport: {
                    codeActionKind: {
                        valueSet: ['', 'quickfix', 'refactor', 'refactor.extract', 'refactor.inline', 'refactor.rewrite', 'source',
                            'source.organizeImports', 'source.fixAll']
                    }
                },
                isPreferredSupport: true,
                disabledSupport: true,
                dataSupport: true,
                resolveSupport: { properties: ['edit'] }
            }
        }
    };
    // The files a branch switch changed: HEAD@{1} is where HEAD pointed before the switch.
    const LSP_BRANCH_DIFF_COMMAND = "git -C /workspace -c core.quotePath=false diff --name-status --no-renames 'HEAD@{1}' HEAD";
    const LSP_SANITIZE_DROP = new Set(['SCRIPT', 'STYLE', 'IFRAME', 'FRAME', 'FRAMESET', 'OBJECT', 'EMBED', 'APPLET', 'SVG', 'MATH',
        'TEMPLATE', 'IMG', 'PICTURE', 'SOURCE', 'VIDEO', 'AUDIO', 'TRACK', 'CANVAS', 'LINK', 'META', 'BASE', 'FORM', 'INPUT', 'BUTTON',
        'TEXTAREA', 'SELECT', 'OPTION', 'NOSCRIPT', 'DIALOG']);
    const LSP_SANITIZE_KEEP = new Set(['P', 'BR', 'HR', 'PRE', 'CODE', 'SPAN', 'DIV', 'EM', 'STRONG', 'B', 'I', 'U', 'S', 'DEL', 'INS',
        'SUB', 'SUP', 'KBD', 'SMALL', 'UL', 'OL', 'LI', 'BLOCKQUOTE', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'TABLE', 'THEAD', 'TBODY', 'TR',
        'TH', 'TD', 'A']);
    const LSP_SERVER_REQUEST_HANDLERS = new Map();
    // Plan 2c pushes factories here at load time; see lspEditorExtensions.
    const lspActionExtensionFactories = [];
    let lspRequestSequence = 0;
    let lspWorkspaceClassCache = null;

    // --- Availability and connections ------------------------------------------------------

    function lspAvailable(instance, cm) {
        return !!instance && isLiveInstance(instance) && !instance.readOnly && !!LSP_PROTOCOL
            && instance.editorType === 'codemirror' && !!cm && typeof cm.LSPClient === 'function'
            && typeof cm.serverCompletion === 'function' && typeof WebSocket === 'function'
            && editorSettings().codeIntelligence !== false;
    }

    function lspConnectionOf(instance, language) {
        return instance && instance.lsp && instance.lsp.connections[language] || null;
    }

    // The connection of a language; with create it is made on first use and its session started when none runs.
    function lspConnection(instance, language, create) {
        let conn = lspConnectionOf(instance, language);
        if (!conn && create) {
            if (!instance.lsp) {
                instance.lsp = { connections: {} };
                addDisposer(instance, () => lspShutdownInstance(instance));
            }
            conn = {
                instance, language, state: 'off', client: null, socket: null, synced: false, attached: false,
                retries: 0, retryTimer: 0, idleTimer: 0, lastRetryAt: 0, waiters: [], pending: new Map(), transport: null
            };
            conn.transport = lspCreateTransport(conn);
            instance.lsp.connections[language] = conn;
        }
        if (conn && create && conn.state === 'off' && !conn.client) lspStart(conn, false);
        return conn;
    }

    function lspStart(conn, reconfigure) {
        const instance = conn.instance;
        if (!lspAvailable(instance, instance.cmModule)) return;
        lspClearTimers(conn);
        conn.retries = 0;
        conn.attached = false;
        try {
            conn.client = lspCreateClient(conn);
        } catch (err) {
            console.warn('[lsp ' + conn.language + '] client setup failed', err);
            conn.client = null;
            conn.state = 'error';
            lspScheduleStatus(instance);
            return;
        }
        conn.state = 'starting';
        lspOpenSocket(conn);
        lspScheduleStatus(instance);
        if (reconfigure) lspReconfigureViews(instance, conn.language);
    }

    function lspCreateClient(conn) {
        const cm = conn.instance.cmModule;
        const WorkspaceClass = lspWorkspaceClass(cm);
        return new cm.LSPClient({
            rootUri: LSP_PROTOCOL.ROOT_URI,
            timeout: LSP_CLIENT_TIMEOUT_MS,
            sanitizeHTML: lspSanitizeHTML,
            workspace: client => new WorkspaceClass(client, conn),
            notificationHandlers: {
                // Returning true replaces the handler of serverDiagnostics(): findings go to the Problems list,
                // which paints the squiggles of every open tab.
                'textDocument/publishDiagnostics': (client, params) => {
                    lspHandleDiagnostics(conn, client, params);
                    return true;
                },
                'window/showMessage': (_client, params) => {
                    lspHandleShowMessage(conn, params);
                    return true;
                },
                'window/logMessage': (_client, params) => {
                    if (params && params.type === 1) console.warn('[lsp ' + conn.language + '] ' + String(params.message || ''));
                    return true;
                }
            },
            // serverDiagnostics() brings autoSync (didChange 500 ms after typing) as its editor extension; the plain
            // object merges the capabilities above into the client's initialize request.
            extensions: [cm.serverDiagnostics(), { clientCapabilities: LSP_CLIENT_CAPABILITIES }]
        });
    }

    function lspOpenSocket(conn) {
        const instance = conn.instance;
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        let socket;
        try {
            socket = new WebSocket(protocol + '//' + location.host + '/api/code-studio/lsp?lang=' + encodeURIComponent(conn.language));
        } catch (_) {
            conn.socket = null;
            lspSocketClosed(conn, 1006);
            return;
        }
        conn.socket = socket;
        conn.synced = false;
        socket.onopen = bindInstance(instance, () => {
            if (conn.socket !== socket || !conn.client) return;
            const client = conn.client;
            // The findings of the previous server are stale; the new one publishes its own.
            lspClearLanguageProblems(conn);
            client.connect(conn.transport);
            client.initializing.then(bindInstance(instance, () => {
                if (conn.socket === socket && conn.client === client) lspBecameReady(conn);
            }), bindInstance(instance, () => {
                if (conn.socket !== socket) return;
                try { socket.close(4000, 'initialize failed'); } catch (_) {}
            }));
        });
        socket.onmessage = event => {
            if (conn.socket === socket) lspReceive(conn, event.data);
        };
        socket.onerror = () => {};
        socket.onclose = bindInstance(instance, event => {
            if (conn.socket === socket) lspSocketClosed(conn, event.code);
        });
    }

    function lspBecameReady(conn) {
        conn.state = 'ready';
        conn.retries = 0;
        lspTouchConnection(conn);
        lspDropSyntaxProblems(conn);
        lspResolveWaiters(conn, conn.client);
        lspScheduleStatus(conn.instance);
        if (!conn.attached) {
            conn.attached = true;
            lspReconfigureViews(conn.instance, conn.language);
        }
    }

    function lspSocketClosed(conn, code) {
        conn.socket = null;
        conn.synced = false;
        lspRejectPending(conn, 'disconnected');
        if (conn.client) conn.client.disconnect();
        if (code === 4404) {
            lspEndSession(conn, 'not-installed');
            return;
        }
        if (code === 4429) {
            lspEndSession(conn, 'too-many');
            return;
        }
        if (conn.retries >= LSP_BACKOFF_MS.length) {
            lspEndSession(conn, 'error');
            return;
        }
        const instance = conn.instance;
        const delay = LSP_BACKOFF_MS[conn.retries];
        conn.retries += 1;
        conn.state = 'disconnected';
        lspScheduleStatus(instance);
        conn.retryTimer = setTimeout(bindInstance(instance, () => {
            conn.retryTimer = 0;
            if (conn.state !== 'disconnected' || !conn.client) return;
            if (!lspAvailable(instance, instance.cmModule)) {
                lspEndSession(conn, 'off');
                return;
            }
            lspOpenSocket(conn);
        }), delay);
    }

    function lspEndSession(conn, nextState, reconfigure = true) {
        lspClearTimers(conn);
        lspCloseSocket(conn, 1000, 'closed');
        const client = conn.client;
        conn.client = null;
        conn.attached = false;
        if (client) {
            try { client.disconnect(); } catch (_) {}
        }
        conn.state = nextState;
        lspClearLanguageProblems(conn);
        lspResolveWaiters(conn, null);
        lspScheduleStatus(conn.instance);
        if (reconfigure) lspReconfigureViews(conn.instance, conn.language);
    }

    function lspCloseSocket(conn, code, reason) {
        const socket = conn.socket;
        conn.socket = null;
        conn.synced = false;
        lspRejectPending(conn, 'disconnected');
        if (!socket) return;
        socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null;
        try { socket.close(code, reason); } catch (_) {}
    }

    function lspClearTimers(conn) {
        if (conn.retryTimer) clearTimeout(conn.retryTimer);
        if (conn.idleTimer) clearTimeout(conn.idleTimer);
        conn.retryTimer = 0;
        conn.idleTimer = 0;
    }

    // Activity: resets the idle timer and reconnects an idle-closed client.
    function lspTouchConnection(conn) {
        if (!conn) return;
        if (conn.state === 'off' && conn.client) {
            if (!lspAvailable(conn.instance, conn.instance.cmModule)) return;
            conn.state = 'starting';
            lspOpenSocket(conn);
            lspScheduleStatus(conn.instance);
        }
        if (!['starting', 'ready', 'disconnected'].includes(conn.state)) return;
        if (conn.idleTimer) clearTimeout(conn.idleTimer);
        conn.idleTimer = setTimeout(() => lspIdleClose(conn), LSP_IDLE_MS);
    }

    // Ten minutes without edits or requests: the socket closes, so the container hold ends and idle shutdown works.
    function lspIdleClose(conn) {
        conn.idleTimer = 0;
        const client = conn.client;
        if (!isLiveInstance(conn.instance) || !client || !client.connected) return;
        if (!['starting', 'ready', 'disconnected'].includes(conn.state)) return;
        const linked = conn.state !== 'disconnected';
        if (conn.retryTimer) clearTimeout(conn.retryTimer);
        conn.retryTimer = 0;
        conn.retries = 0;
        lspCloseSocket(conn, 1000, 'idle');
        if (linked) client.disconnect();
        conn.state = 'off';
        lspResolveWaiters(conn, null);
        lspScheduleStatus(conn.instance);
    }

    function lspResolveWaiters(conn, client) {
        const waiters = conn.waiters.splice(0);
        waiters.forEach(resolve => resolve(client));
    }

    function lspRejectPending(conn, reason) {
        conn.pending.forEach(entry => {
            clearTimeout(entry.timer);
            entry.reject(new Error(reason));
        });
        conn.pending.clear();
    }

    function lspSettlePending(conn, message) {
        const entry = conn.pending.get(message.id);
        conn.pending.delete(message.id);
        clearTimeout(entry.timer);
        if (message.error) {
            const error = new Error(String(message.error.message || 'request failed'));
            error.code = message.error.code;
            error.data = message.error.data;
            entry.reject(error);
        } else {
            entry.resolve(message.result === undefined ? null : message.result);
        }
    }

    function lspScheduleStatus(instance) {
        if (!instance || instance.lspStatusTimer) return;
        instance.lspStatusTimer = setTimeout(() => {
            instance.lspStatusTimer = 0;
            if (isLiveInstance(instance)) runWithInstance(instance, () => { if (studioRoot()) renderStatus(); });
        }, 0);
    }

    // Swaps the LSP compartment of the open editors of one language (null: all languages).
    function lspReconfigureViews(instance, language) {
        if (!isLiveInstance(instance) || !LSP_PROTOCOL) return;
        runWithInstance(instance, () => {
            const cm = state.cmModule;
            if (!cm) return;
            state.openTabs.forEach(tab => {
                if (!isSourceTab(tab) || (language && LSP_PROTOCOL.languageForPath(tab.path) !== language)) return;
                (tab.views || []).forEach(view => {
                    if (!view || !view.csLsp) return;
                    const next = lspEditorExtensions(cm, tab);
                    if (!next.length && !view.csLspActive) return;
                    view.csLspActive = next.length > 0;
                    try {
                        view.dispatch({ effects: view.csLsp.reconfigure(next) });
                    } catch (err) {
                        console.warn('[lsp] editor reconfigure failed', err);
                    }
                });
            });
        });
    }

    // The settings switch and the read-only policy: turns LSP off (every session ends) or on (editors start it).
    function lspApplyAvailability() {
        const instance = state;
        if (!instance) return;
        const available = lspAvailable(instance, instance.cmModule);
        if (instance.lspAvailableApplied === available) return;
        instance.lspAvailableApplied = available;
        if (!available && instance.lsp) Object.values(instance.lsp.connections).forEach(conn => lspEndSession(conn, 'off', false));
        lspReconfigureViews(instance, null);
    }

    function lspShutdownInstance(instance) {
        const lsp = instance && instance.lsp;
        if (lsp) {
            Object.values(lsp.connections).forEach(conn => {
                lspClearTimers(conn);
                lspCloseSocket(conn, 1000, 'closed');
                if (conn.client) {
                    try { conn.client.disconnect(); } catch (_) {}
                }
                conn.client = null;
                conn.attached = false;
                conn.state = 'off';
                lspResolveWaiters(conn, null);
            });
        }
        if (instance && instance.lspStatusTimer) clearTimeout(instance.lspStatusTimer);
        if (instance) instance.lspStatusTimer = 0;
    }

    // Status bar click: a new session with a new client and a new server process.
    function lspRestart(language) {
        const instance = state;
        const conn = lspConnectionOf(instance, language);
        if (!conn || !lspAvailable(instance, instance.cmModule)) return;
        lspEndSession(conn, 'off', false);
        lspStart(conn, true);
    }

    // 4429: the next focus of an editor of that language tries again.
    function lspRetryTooMany(instance, language) {
        const conn = lspConnectionOf(instance, language);
        if (!conn || conn.state !== 'too-many' || Date.now() - conn.lastRetryAt < LSP_TOO_MANY_RETRY_GAP_MS) return;
        conn.lastRetryAt = Date.now();
        setTimeout(bindInstance(instance, () => {
            if (conn.state === 'too-many' && lspAvailable(instance, instance.cmModule)) lspStart(conn, true);
        }), 0);
    }

    // --- Transport --------------------------------------------------------------------------

    // One stable transport per connection; the socket behind it changes with every reconnect.
    function lspCreateTransport(conn) {
        const handlers = [];
        return {
            send(message) {
                let text = String(message);
                if (LSP_INITIALIZE_REQUEST.test(text)) text = lspPatchInitialize(text);
                const isRequest = LSP_REQUEST_MESSAGE.test(text);
                if (!lspSocketOpen(conn)) {
                    // A request fails at once (the client rejects it); a notification for a closed socket is dropped.
                    if (isRequest) throw new Error('Language server disconnected');
                    return;
                }
                try {
                    conn.socket.send(text);
                } catch (err) {
                    if (isRequest) throw err;
                    return;
                }
                if (text === LSP_INITIALIZED_MESSAGE) lspOpenDocuments(conn);
            },
            subscribe(handler) {
                if (!handlers.includes(handler)) handlers.push(handler);
            },
            unsubscribe(handler) {
                const index = handlers.indexOf(handler);
                if (index >= 0) handlers.splice(index, 1);
            },
            deliver(text) {
                handlers.slice().forEach(handler => {
                    try {
                        handler(text);
                    } catch (err) {
                        console.warn('[lsp ' + conn.language + '] message handling failed', err);
                    }
                });
            }
        };
    }

    // 6.3.1 sends rootUri only; the single workspace folder is added here.
    function lspPatchInitialize(text) {
        try {
            const message = JSON.parse(text);
            message.params = Object.assign({}, message.params, { workspaceFolders: [{ uri: LSP_PROTOCOL.ROOT_URI, name: 'workspace' }] });
            return JSON.stringify(message);
        } catch (_) {
            return text;
        }
    }

    function lspSocketOpen(conn) {
        return !!conn.socket && conn.socket.readyState === 1;
    }

    function lspSendRaw(conn, payload) {
        if (!lspSocketOpen(conn)) return false;
        try {
            conn.socket.send(typeof payload === 'string' ? payload : JSON.stringify(payload));
            return true;
        } catch (_) {
            return false;
        }
    }

    // Server-to-client requests (id and method) never reach the LSPClient, which would answer MethodNotFound.
    // Answers to lspRequest (string ids "cs-N") settle here too; everything else goes to the client.
    function lspReceive(conn, data) {
        if (typeof data !== 'string' || !data) return;
        let message;
        try {
            message = JSON.parse(data);
        } catch (_) {
            return;
        }
        if (!message || typeof message !== 'object' || Array.isArray(message)) return;
        const hasId = message.id !== undefined && message.id !== null;
        if (hasId && typeof message.method === 'string') {
            lspAnswerServerRequest(conn, message);
            return;
        }
        if (hasId && conn.pending.has(message.id)) {
            lspSettlePending(conn, message);
            return;
        }
        conn.transport.deliver(data);
    }

    function lspAnswerServerRequest(conn, message) {
        const reply = payload => lspSendRaw(conn, Object.assign({ jsonrpc: '2.0', id: message.id }, payload));
        const fail = err => reply({ error: { code: -32603, message: String(err && err.message ? err.message : err) } });
        const handler = LSP_SERVER_REQUEST_HANDLERS.get(message.method);
        if (!handler) {
            reply({ error: { code: -32601, message: 'Method not implemented: ' + message.method } });
            return;
        }
        let result;
        try {
            result = runWithInstance(conn.instance, () => handler(conn.language, message.params));
        } catch (err) {
            fail(err);
            return;
        }
        Promise.resolve(result).then(value => reply({ result: value === undefined ? null : value }), fail);
    }

    // Runs right after "initialized" went out: every tracked document is opened on the (new) server with its current
    // text, before any request that waited for the connection is sent.
    function lspOpenDocuments(conn) {
        const client = conn.client;
        const workspace = client && client.workspace;
        if (!workspace) return;
        const cm = conn.instance.cmModule;
        try {
            workspace.prune();
            for (const file of workspace.files) {
                const view = file.getView();
                const tab = workspace.tabFor(file.path);
                if (view) {
                    file.doc = view.state.doc;
                } else if (tab) {
                    file.doc = lspTextOf(cm, tab.content);
                    file.contentRef = tab.content;
                }
                file.version += 1;
                lspClearFilePlugins(cm, file);
                lspSendRaw(conn, {
                    jsonrpc: '2.0',
                    method: 'textDocument/didOpen',
                    params: { textDocument: { uri: file.uri, languageId: file.languageId, version: file.version, text: file.doc.toString() } }
                });
            }
        } catch (err) {
            console.warn('[lsp ' + conn.language + '] reopening documents failed', err);
        }
        conn.synced = true;
    }

    // --- Workspace ----------------------------------------------------------------------------

    // One LSP document per open file tab. version and doc are what the server has; views are the live editors of the
    // tab that carry this client's plugin (two in a split of the same file).
    class LspWorkspaceFile {
        constructor(workspace, uri, path, languageId, doc) {
            this.workspace = workspace;
            this.uri = uri;
            this.path = path;
            this.languageId = languageId;
            this.version = 0;
            this.doc = doc;
            this.views = new Set();
            this.contentRef = null;
        }

        getView(main) {
            if (main && this.views.has(main)) return main;
            const tab = this.workspace.tabFor(this.path);
            const focused = tab && (tab.focusedView || tab.view);
            if (focused && this.views.has(focused)) return focused;
            const first = this.views.values().next();
            return first.done ? null : first.value;
        }
    }

    // The bundle loads CodeMirror lazily, so the subclass is made on first use.
    function lspWorkspaceClass(cm) {
        if (lspWorkspaceClassCache && lspWorkspaceClassCache.base === cm.Workspace) return lspWorkspaceClassCache.cls;
        class CodeStudioLSPWorkspace extends cm.Workspace {
            constructor(client, conn) {
                super(client);
                this.conn = conn;
                this.files = [];
                this.pruneTimer = 0;
            }

            // A restart gives the connection a new client; the old workspace then stays silent.
            get live() {
                return this.conn.client === this.client;
            }

            get synced() {
                return this.live && this.conn.synced;
            }

            tabFor(path) {
                return this.conn.instance.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
            }

            // Servers may spell a URI differently (percent-encoding); the canonical form is the key.
            getFile(uri) {
                const path = LSP_PROTOCOL.pathForUri(uri);
                const canonical = path ? LSP_PROTOCOL.uriForPath(path) : String(uri || '');
                return this.files.find(file => file.uri === canonical) || null;
            }

            syncFiles() {
                if (this.live) lspTouchConnection(this.conn);
                if (!this.synced) return [];
                const updates = [];
                for (const file of this.files) {
                    const update = lspSyncWorkspaceFile(this, file);
                    if (update) updates.push(update);
                }
                return updates;
            }

            openFile(uri, languageId, view) {
                const known = this.getFile(uri);
                if (known) {
                    known.views.add(view);
                    // A view rebuilt from a snapshot can hold edits the server has not seen yet.
                    if (this.synced && !view.state.doc.eq(known.doc)) setTimeout(() => { if (this.synced) this.client.sync(); }, 0);
                    return;
                }
                const path = LSP_PROTOCOL.pathForUri(uri);
                if (!path) return;
                const file = new LspWorkspaceFile(this, uri, path, languageId, view.state.doc);
                file.views.add(view);
                this.files.push(file);
                if (this.synced) this.client.didOpen(file);
            }

            // Every tab switch rebuilds the views; the document stays open on the server until its tab closes.
            closeFile(uri, view) {
                const file = this.getFile(uri);
                if (!file) return;
                file.views.delete(view);
                file.contentRef = null;
                if (!this.pruneTimer) this.pruneTimer = setTimeout(() => { this.pruneTimer = 0; this.prune(); }, 0);
            }

            prune() {
                this.files = this.files.filter(file => {
                    if (file.views.size || this.tabFor(file.path)) return true;
                    if (this.synced) this.client.didClose(file.uri);
                    return false;
                });
            }

            // Documents are (re)opened by lspOpenDocuments right after "initialized".
            connected() {}

            disconnected() {}

            displayFile(uri) {
                return lspDisplayFile(this.conn.instance, uri);
            }
        }
        lspWorkspaceClassCache = { base: cm.Workspace, cls: CodeStudioLSPWorkspace };
        return CodeStudioLSPWorkspace;
    }

    // Cross-file jumps: open the tab (or activate it) and hand back its focused view. openFile is core.js's.
    async function lspDisplayFile(instance, uri) {
        const path = LSP_PROTOCOL.pathForUri(uri);
        if (!path || !isLiveInstance(instance)) return null;
        try {
            await runAsyncStep(instance, () => openFile(path));
        } catch (_) {
            return null;
        }
        if (!isLiveInstance(instance)) return null;
        const view = runWithInstance(instance, () => lspViewForPath(path));
        if (view) view.focus();
        return view;
    }

    // One file's update for client.sync(): the plugin's own changes when its base is what the server has, otherwise
    // (rebuilt view, buffer replaced while no view was mounted) the whole text as one replacement.
    function lspSyncWorkspaceFile(workspace, file) {
        const cm = workspace.conn.instance.cmModule;
        const view = file.getView();
        const plugin = view && cm.LSPPlugin.get(view);
        let doc;
        let changes;
        if (plugin) {
            doc = view.state.doc;
            if (doc === file.doc) {
                lspClearFilePlugins(cm, file);
                return null;
            }
            const base = plugin.syncedDoc;
            if (base === file.doc || base.eq(file.doc)) {
                if (plugin.unsyncedChanges.empty) {
                    file.doc = doc;
                    lspClearFilePlugins(cm, file);
                    return null;
                }
                changes = plugin.unsyncedChanges;
            } else if (doc.eq(file.doc)) {
                file.doc = doc;
                lspClearFilePlugins(cm, file);
                return null;
            } else {
                changes = cm.ChangeSet.of([{ from: 0, to: file.doc.length, insert: doc }], file.doc.length);
            }
        } else {
            const tab = workspace.tabFor(file.path);
            if (!tab || tab.content === file.contentRef) return null;
            file.contentRef = tab.content;
            const text = String(tab.content == null ? '' : tab.content);
            if (text === file.doc.toString()) return null;
            doc = lspTextOf(cm, text);
            changes = cm.ChangeSet.of([{ from: 0, to: file.doc.length, insert: doc }], file.doc.length);
        }
        const prevDoc = file.doc;
        file.doc = doc;
        file.version += 1;
        lspClearFilePlugins(cm, file);
        return { file, prevDoc, changes };
    }

    function lspTextOf(cm, content) {
        return cm.Text.of(String(content == null ? '' : content).split(/\r\n?|\n/));
    }

    function lspClearFilePlugins(cm, file) {
        file.views.forEach(view => {
            const plugin = cm.LSPPlugin.get(view);
            if (plugin) plugin.clear();
        });
    }

    // --- Diagnostics and messages -------------------------------------------------------------

    function lspHandleDiagnostics(conn, client, params) {
        if (conn.client !== client || !params || !isLiveInstance(conn.instance)) return;
        const path = LSP_PROTOCOL.pathForUri(params.uri);
        if (!path) return;
        const file = client.workspace.getFile(params.uri);
        // Findings for an older version of an open document would land on the wrong lines; a newer list follows.
        if (file && typeof params.version === 'number' && params.version !== file.version) return;
        const source = LSP_PROTOCOL.sourceForLanguage(conn.language);
        runWithInstance(conn.instance, () => replaceProblemsForFile(path, source, LSP_PROTOCOL.diagnosticsToProblems(path, params.diagnostics, source)));
    }

    function lspHandleShowMessage(conn, params) {
        if (!params || (params.type !== 1 && params.type !== 2) || !isLiveInstance(conn.instance)) return;
        runWithInstance(conn.instance, () => flashStatus(LSP_SERVER_NAMES[conn.language] + ': ' + String(params.message || ''), 6000));
    }

    function lspClearLanguageProblems(conn) {
        if (!isLiveInstance(conn.instance)) return;
        const source = LSP_PROTOCOL.sourceForLanguage(conn.language);
        runWithInstance(conn.instance, () => {
            const current = state.problems || [];
            const kept = current.filter(problem => problem.source !== source);
            if (kept.length !== current.length) setProblems(kept);
        });
    }

    // A ready server reports syntax errors itself; the bracket check's findings for its files would be duplicates.
    function lspDropSyntaxProblems(conn) {
        if (!isLiveInstance(conn.instance)) return;
        runWithInstance(conn.instance, () => {
            const current = state.problems || [];
            const kept = current.filter(problem => !(problem.source === 'syntax' && LSP_PROTOCOL.languageForPath(problem.file) === conn.language));
            if (kept.length !== current.length) setProblems(kept);
        });
    }

    function lspServesPath(path) {
        const language = LSP_PROTOCOL ? LSP_PROTOCOL.languageForPath(path) : null;
        return !!language && lspClientState(language) === 'ready';
    }

    // --- Editor extensions --------------------------------------------------------------------

    // The content of view.csLsp for one tab. Empty while LSP is off, read-only, not a server language, or the
    // session's client never connected; only a focus handler while the server limit is reached.
    function lspEditorExtensions(cm, tab) {
        const instance = state;
        const available = lspAvailable(instance, cm);
        if (instance) instance.lspAvailableApplied = available;
        if (!available || !isSourceTab(tab)) return [];
        const language = LSP_PROTOCOL.languageForPath(tab.path);
        const uri = language ? LSP_PROTOCOL.uriForPath(tab.path) : null;
        if (!uri) return [];
        const conn = lspConnection(instance, language, true);
        if (conn.state === 'too-many') {
            return [cm.EditorView.domEventHandlers({ focus: () => { lspRetryTooMany(instance, language); return false; } })];
        }
        if (!conn.client || !conn.client.connected) return [];
        const extras = [];
        lspActionExtensionFactories.forEach(factory => {
            try {
                const result = factory(cm, { language, path: tab.path, tab });
                if (result) extras.push(result);
            } catch (err) {
                console.warn('[lsp] action extension failed', err);
            }
        });
        return [
            conn.client.plugin(uri, LSP_PROTOCOL.languageIdForPath(tab.path)),
            cm.serverCompletion(),
            cm.hoverTooltips(),
            // Signature help on ( and , plus Ctrl+Shift+Space (Cmd+Shift+Space on macOS).
            cm.signatureHelp(),
            cm.Prec.high(cm.keymap.of(cm.jumpToDefinitionKeymap)),
            cm.Prec.high(cm.EditorView.domEventHandlers({ mousedown: (event, view) => lspModClick(instance, language, event, view) })),
            cm.Prec.high(cm.EditorView.clickAddsSelectionRange.of(event => lspAddsCursor(instance, language, event))),
            cm.EditorView.exceptionSink.of(lspExceptionSink),
            extras
        ];
    }

    function lspIsMac() {
        return /Mac|iPhone|iPad/.test(navigator.platform || '');
    }

    // Ctrl+Click (Cmd+Click on macOS) jumps to the definition while the file's server is ready.
    function lspModClick(instance, language, event, view) {
        if (event.button !== 0 || event.altKey || event.shiftKey || !(lspIsMac() ? event.metaKey : event.ctrlKey)) return false;
        const conn = lspConnectionOf(instance, language);
        if (!conn || conn.state !== 'ready') return false;
        const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
        if (pos == null) return false;
        event.preventDefault();
        view.dispatch({ selection: { anchor: pos } });
        view.focus();
        instance.cmModule.jumpToDefinition(view);
        return true;
    }

    // While the server is ready, extra cursors move to Alt+Click, as in VS Code; otherwise the default stays.
    function lspAddsCursor(instance, language, event) {
        const conn = lspConnectionOf(instance, language);
        if (conn && conn.state === 'ready') return event.altKey;
        return lspIsMac() ? event.metaKey : event.ctrlKey;
    }

    // Timeouts and cancelled requests of hover and completion are routine while a server starts or is busy.
    function lspExceptionSink(error) {
        const message = String(error && error.message ? error.message : error);
        if (LSP_QUIET_ERRORS.test(message) || (error && LSP_QUIET_ERROR_CODES.includes(error.code))) console.warn('[lsp] ' + message);
        else console.error(error);
    }

    // Allowlist sanitizer for the Markdown HTML of hover, completion and signature docs. Markup outside the list is
    // unwrapped to its text, active content is dropped, and only class (highlighting) and http(s) links survive.
    // Parsing happens in an inert <template>, so nothing loads or runs on the way.
    function lspSanitizeHTML(html) {
        const template = document.createElement('template');
        template.innerHTML = String(html == null ? '' : html);
        lspSanitizeNode(template.content);
        const box = document.createElement('div');
        box.appendChild(template.content);
        return box.innerHTML;
    }

    function lspSanitizeNode(parent) {
        Array.from(parent.childNodes).forEach(node => {
            if (node.nodeType === 3) return;
            if (node.nodeType !== 1) {
                node.remove();
                return;
            }
            const tag = String(node.nodeName).toUpperCase();
            if (LSP_SANITIZE_DROP.has(tag)) {
                node.remove();
                return;
            }
            lspSanitizeNode(node);
            if (!LSP_SANITIZE_KEEP.has(tag)) {
                node.replaceWith(...Array.from(node.childNodes));
                return;
            }
            Array.from(node.attributes).forEach(attribute => {
                const name = attribute.name.toLowerCase();
                const keep = name === 'class' || (tag === 'A' && name === 'href' && /^https?:\/\//i.test(attribute.value.trim()));
                if (!keep) node.removeAttribute(attribute.name);
            });
            if (tag === 'A' && node.hasAttribute('href')) {
                node.setAttribute('target', '_blank');
                node.setAttribute('rel', 'noopener noreferrer');
            }
        });
    }

    // --- Status bar ---------------------------------------------------------------------------

    function lspStateText(stateName) {
        switch (stateName) {
            case 'starting': return tr('codeStudio.lspStarting', 'Starting...');
            case 'ready': return tr('codeStudio.lspReady', 'Ready');
            case 'disconnected': return tr('codeStudio.lspDisconnected', 'Disconnected');
            case 'not-installed': return tr('codeStudio.lspNotInstalled', 'Language server not installed — rebuild the Code Studio image');
            case 'too-many': return tr('codeStudio.lspTooMany', 'Too many language servers');
            case 'error': return tr('codeStudio.lspError', 'Error — click to restart');
            default: return tr('codeStudio.lspIdle', 'Idle');
        }
    }

    // The active file's server: state dot plus label; a click restarts it (renderStatus wires the click).
    function lspStatusMarkup(tab) {
        const instance = state;
        if (!isSourceTab(tab) || !lspAvailable(instance, instance && instance.cmModule)) return '';
        const language = LSP_PROTOCOL.languageForPath(tab.path);
        const conn = language ? lspConnectionOf(instance, language) : null;
        if (!conn || (conn.state === 'off' && !conn.client)) return '';
        const label = tr('codeStudio.lspStatus', '{{server}}: {{state}}', { server: LSP_SERVER_NAMES[language], state: lspStateText(conn.state) });
        const hint = tr('codeStudio.lspRestartHint', 'Click to restart the language server');
        return `<span class="cs-status-item cs-status-lsp" data-clickable data-status-lsp="${esc(language)}" data-lsp-state="${esc(conn.state)}" title="${esc(label + '\n' + hint)}"><span class="cs-lsp-dot" data-lsp-state="${esc(conn.state)}"></span><span class="cs-status-lsp-label">${esc(label)}</span></span>`;
    }

    // --- Code Studio writes ---------------------------------------------------------------------

    // After a branch switch: the changed files from git (open tabs as the fallback) go to the ready servers.
    function lspNotifyBranchSwitch() {
        const instance = state;
        if (!instance || !instance.lsp || !LSP_LANGUAGES.some(language => lspClientState(language) === 'ready')) return;
        const fallback = instance.openTabs.filter(isSourceTab).map(tab => ({ path: tab.path, type: 2 }));
        Promise.resolve(apiClient.exec(LSP_BRANCH_DIFF_COMMAND, WORKSPACE_ROOT, 30)).then(result => {
            if (!isLiveInstance(instance)) return;
            const changes = lspParseNameStatus(result && result.output);
            runWithInstance(instance, () => lspNotifyFilesChanged(changes.concat(fallback)));
        }, () => {
            if (isLiveInstance(instance)) runWithInstance(instance, () => lspNotifyFilesChanged(fallback));
        });
    }

    function lspParseNameStatus(output) {
        const types = { A: 1, M: 2, T: 2, D: 3 };
        return String(output || '').split('\n')
            .map(line => /^([AMTD])\t([^"].*)$/.exec(line.replace(/\r$/, '')))
            .filter(Boolean)
            .map(match => ({ path: match[2], type: types[match[1]] }));
    }

    // --- Contract API for plan 2c ---------------------------------------------------------------

    function lspEnsureClient(language) {
        const instance = state;
        if (!LSP_LANGUAGES.includes(language) || !lspAvailable(instance, instance && instance.cmModule)) return Promise.resolve(null);
        const conn = lspConnection(instance, language, true);
        lspTouchConnection(conn);
        if (conn.state === 'ready' && conn.client) return Promise.resolve(conn.client);
        if (conn.state !== 'starting' && conn.state !== 'disconnected') return Promise.resolve(null);
        return new Promise(resolve => conn.waiters.push(resolve));
    }

    function lspClientState(language) {
        const instance = state;
        const conn = lspConnectionOf(instance, language);
        if (!conn || !lspAvailable(instance, instance.cmModule)) return 'off';
        return conn.state;
    }

    // Own id space and own timer: the default 3 s budget holds even though the LSPClient itself waits up to 15 s.
    async function lspRequest(language, method, params, timeoutMs = LSP_REQUEST_TIMEOUT_MS) {
        const instance = state;
        const client = await lspEnsureClient(language);
        const conn = lspConnectionOf(instance, language);
        if (!client || !conn || conn.client !== client || !isLiveInstance(instance)) throw new Error('unavailable');
        lspTouchConnection(conn);
        client.sync();
        const id = 'cs-' + (++lspRequestSequence);
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                if (!conn.pending.delete(id)) return;
                lspSendRaw(conn, { jsonrpc: '2.0', method: '$/cancelRequest', params: { id } });
                reject(new Error('timeout'));
            }, Math.max(1, Number(timeoutMs) || LSP_REQUEST_TIMEOUT_MS));
            conn.pending.set(id, { resolve, reject, timer });
            // The didChange that sync() queued goes out first: both wait on the same resolved promise.
            client.initializing.then(() => {
                if (!conn.pending.has(id)) return;
                if (!lspSendRaw(conn, { jsonrpc: '2.0', id, method, params })) {
                    clearTimeout(timer);
                    conn.pending.delete(id);
                    reject(new Error('disconnected'));
                }
            }, () => {
                if (!conn.pending.delete(id)) return;
                clearTimeout(timer);
                reject(new Error('disconnected'));
            });
        });
    }

    function lspOnServerRequest(method, handler) {
        if (typeof method === 'string' && method && typeof handler === 'function') LSP_SERVER_REQUEST_HANDLERS.set(method, handler);
    }

    function lspNotifyFilesChanged(changes) {
        const instance = state;
        if (!instance || !instance.lsp || !LSP_PROTOCOL) return;
        const ready = LSP_LANGUAGES.map(language => lspConnectionOf(instance, language))
            .filter(conn => conn && conn.state === 'ready' && conn.synced && conn.client);
        if (!ready.length) return;
        const events = [];
        (Array.isArray(changes) ? changes : []).forEach(change => {
            const path = change ? normalizeCodeStudioPath(change.path) : WORKSPACE_ROOT;
            const uri = path !== WORKSPACE_ROOT ? LSP_PROTOCOL.uriForPath(path) : null;
            if (!uri) return;
            events.push({ uri, language: LSP_PROTOCOL.languageForPath(path), type: [1, 2, 3].includes(change.type) ? change.type : 2 });
        });
        ready.forEach(conn => {
            // Source files go to their own server; project files (go.mod, package.json, tsconfig.json, ...) to every one.
            const mine = events.filter(event => !event.language || event.language === conn.language);
            if (!mine.length) return;
            lspTouchConnection(conn);
            conn.client.sync();
            mine.forEach(event => {
                if (event.type === 2 && conn.client.workspace.getFile(event.uri)) {
                    conn.client.notification('textDocument/didSave', { textDocument: { uri: event.uri } });
                }
            });
            conn.client.notification('workspace/didChangeWatchedFiles', { changes: mine.map(event => ({ uri: event.uri, type: event.type })) });
        });
    }

    function lspViewForPath(path) {
        if (!state) return null;
        const wanted = normalizeCodeStudioPath(path);
        const tab = state.openTabs.find(item => isSourceTab(item) && item.path === wanted);
        const view = tab && (tab.focusedView || tab.view);
        return view && view.state ? view : null;
    }

    function lspTouch(language) {
        lspTouchConnection(lspConnectionOf(state, language));
    }

    // Server-to-client requests the page answers itself; plan 2c adds workspace/applyEdit.
    lspOnServerRequest('workspace/configuration', (_language, params) => (params && Array.isArray(params.items) ? params.items.map(() => null) : []));
    lspOnServerRequest('window/workDoneProgress/create', () => null);
    lspOnServerRequest('client/registerCapability', () => null);
    lspOnServerRequest('client/unregisterCapability', () => null);
    lspOnServerRequest('window/showMessageRequest', (language, params) => {
        if (params && params.message) flashStatus(LSP_SERVER_NAMES[language] + ': ' + String(params.message), 6000);
        return null;
    });
```

- [ ] **Step 2: Add the two fragments to the bundle list in `scripts/build-ui-bundles.js`**

Replace:
```
      'ui/js/desktop/apps/code-studio/settings.js',
      'ui/js/desktop/apps/code-studio/format.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```
With:
```
      'ui/js/desktop/apps/code-studio/settings.js',
      'ui/js/desktop/apps/code-studio/format.js',
      // IDE package 2b: the pure protocol helpers first (window.CodeStudioLSPProtocol), then the client core.
      // Package 2c adds lsp-edits.js and lsp-actions.js right after lsp.js: lsp-actions.js pushes into
      // lspActionExtensionFactories at load time, so it must come after lsp.js and before shortcuts.js.
      'ui/js/desktop/apps/code-studio/lsp-protocol.js',
      'ui/js/desktop/apps/code-studio/lsp.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```

- [ ] **Step 3: Foreign-edit check, build, parse check, i18n check, name check**

```
git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
npm run build:ui
npm run build:ui -- --check
node -e "new (require('vm').Script)(require('fs').readFileSync('ui/js/desktop/bundles/code-studio.bundle.js','utf8'),{filename:'code-studio.bundle.js'})"
node -e "const s=require('fs').readFileSync('ui/js/desktop/apps/code-studio/lsp.js','utf8');const bad=(s.match(/^    (?:const|let|function|async function|class) (lspAct\w*|LSP_ACT_\w*|ensureReferencesPanelParts|renderReferencesPanel|clearReferences|CodeStudioLSPEdits)\b/gm)||[]).filter(m=>!/lspActionExtensionFactories/.test(m));console.log(bad.length?bad.join(','):'names ok')"
GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./ui -count=1 -v -run '(?i)^(TestDesktopUsedI18nKeysExistInAllLanguages|TestCodeStudioUsesPerWindowStateAndClosesTerminal)$'
```
Expected:
- The status lists only ` M scripts/build-ui-bundles.js` and `?? ui/js/desktop/apps/code-studio/lsp.js`.
- `Built ... (20 parts)` and `Checked ... (20 parts)`.
- The vm parse prints nothing.
- The name check prints `names ok`.
- 2 `--- PASS` lines.

- [ ] **Step 4: Commit**

Run `detect-changes --scope all`. Message `msg-task5.txt`:
```
feat(code-studio): add the LSP client core

lsp.js: one LSPClient per language and window over the /api/code-studio/lsp
bridge, the client capabilities plan 2c needs, a 15 s client timeout with a
3 s default for lspRequest, server-request registry, idle close and reconnect
backoff, a Workspace that keeps one LSP document per tab, diagnostics into the
Problems list, the hover sanitizer and the API plan 2c builds on.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task5.txt ui/js/desktop/apps/code-studio/lsp.js scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
```

---

### Task 6: Editor integration and the syntax-check handover

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/editor.js` (`createCodeMirrorEditor`), `ui/js/desktop/apps/code-studio/problems.js` (`checkDocumentContent`)
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Impact check.** Run `impact` upstream for `createCodeMirrorEditor` and `checkDocumentContent`. Expected callers:
  - `createCodeMirrorEditor`: `createEditorView` (from `renderEditor`/`renderSplitPanes`). Likely HIGH; warn in the report. The edit is additive.
  - `checkDocumentContent`: `checkActiveDocument`

- [ ] **Step 2: Add the LSP compartment in `createCodeMirrorEditor`**

Replace:
```js
        // Blame gutter of git-history.js, switched per view without rebuilding the editor.
        const blame = new cm.Compartment();
        const extensions = [
```
With:
```js
        // Blame gutter of git-history.js, switched per view without rebuilding the editor.
        const blame = new cm.Compartment();
        // IDE package 2b: the LSP extensions of lsp.js (view.csLsp). The settings switch, the read-only policy and the
        // session state swap them on the open editor.
        const lspCompartment = new cm.Compartment();
        const lspInitial = lspEditorExtensions(cm, tab);
        const extensions = [
```

Replace:
```js
            languageExtension(cm, tab.language),
            cm.keymap && cm.keymap.of([
```
With:
```js
            languageExtension(cm, tab.language),
            lspCompartment.of(lspInitial),
            cm.keymap && cm.keymap.of([
```

Replace:
```js
        view.csBlame = blame;
        view.csLight = light;
```
With:
```js
        view.csBlame = blame;
        view.csLsp = lspCompartment;
        view.csLspActive = lspInitial.length > 0;
        view.csLight = light;
```

- [ ] **Step 3: Hand the syntax check over to a ready server in `checkDocumentContent`**

Replace:
```js
        const language = tab.language || languageForPath(tab.path);
        if (language === 'json') return checkJson(code);
        if (BRACKET_LANGUAGES.includes(language)) return checkBrackets(code);
        return [];
    }
```
With:
```js
        const language = tab.language || languageForPath(tab.path);
        if (language === 'json') return checkJson(code);
        // IDE package 2b: a ready language server reports syntax errors itself; the bracket check would duplicate them.
        if (lspServesPath(tab.path)) return [];
        if (BRACKET_LANGUAGES.includes(language)) return checkBrackets(code);
        return [];
    }
```

- [ ] **Step 4: Foreign-edit check, build, parse check**

```
git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
npm run build:ui
npm run build:ui -- --check
node -e "new (require('vm').Script)(require('fs').readFileSync('ui/js/desktop/bundles/code-studio.bundle.js','utf8'),{filename:'code-studio.bundle.js'})"
```
Expected:
- Only `editor.js` and `problems.js` before the build.
- `Built` and `Checked ... (20 parts)`.
- The vm parse prints nothing.

- [ ] **Step 5: Commit**

Run `detect-changes --scope all`. Message `msg-task6.txt`:
```
feat(code-studio): attach LSP extensions to source editors

Each CodeMirror view gets an LSP compartment (completion, hover, signature
help, F12, Ctrl/Cmd+Click with Alt+Click cursors). A ready server takes over
the bracket check of its files.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task6.txt ui/js/desktop/apps/code-studio/editor.js ui/js/desktop/apps/code-studio/problems.js ui/js/desktop/bundles/code-studio.bundle.js
```

---

### Task 7: Status bar item, settings switch, read-only policy and CSS

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/settings.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/css/code-studio.css`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

No bottom-panel tab is added: the panel keeps its three tabs, and 2c inserts References lazily.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `normalizeEditorSettings`, `openEditorSettings`, `applyEditorSettingsToOpenEditors`, `renderStatus` and `applyStudioPolicy`. `renderStatus` is likely HIGH; warn in the report. The edits are additive.

- [ ] **Step 2: The setting in `normalizeEditorSettings` (`settings.js`)**

Replace:
```js
            insertFinalNewline: value.insertFinalNewline === true,
            formatOnSave: value.formatOnSave === true
        };
```
With:
```js
            insertFinalNewline: value.insertFinalNewline === true,
            formatOnSave: value.formatOnSave === true,
            // IDE package 2b: code intelligence through the language servers; on unless switched off.
            codeIntelligence: value.codeIntelligence !== false
        };
```

- [ ] **Step 3: Apply the switch live (`applyEditorSettingsToOpenEditors`)**

Replace:
```js
                }));
            }
            if (studioRoot()) renderStatus();
        }));
    }
```
With:
```js
                }));
            }
            lspApplyAvailability(); // IDE package 2b: the "Code intelligence (LSP)" switch
            if (studioRoot()) renderStatus();
        }));
    }
```

- [ ] **Step 4: The switch in the dialog (`openEditorSettings`)**

Replace:
```js
                ${checkbox('formatOnSave', tr('codeStudio.settingsFormatOnSave', 'Format on save'))}
```
With:
```js
                ${checkbox('formatOnSave', tr('codeStudio.settingsFormatOnSave', 'Format on save'))}
                ${checkbox('codeIntelligence', tr('codeStudio.settingsCodeIntelligence', 'Code intelligence (LSP)'))}
```
The existing change handler stores other checkboxes as `{ [key]: control.checked === true }`.

- [ ] **Step 5: Status item and its click in `renderStatus` (`core.js`)**

Replace:
```js
        if (lang) rightItems.push(`<span class="cs-status-item" title="${esc(tr('codeStudio.language', 'Language'))}">${esc(lang)}</span>`);
```
With:
```js
        if (lang) rightItems.push(`<span class="cs-status-item" title="${esc(tr('codeStudio.language', 'Language'))}">${esc(lang)}</span>`);
        const lspItem = lspStatusMarkup(tab); // IDE package 2b: the active file's language server
        if (lspItem) rightItems.push(lspItem);
```

Replace:
```js
        const branch = statusEl.querySelector('[data-status-branch]');
        if (branch) branch.addEventListener('click', bind(() => {
            const rect = branch.getBoundingClientRect();
            showGitBranchMenu(rect.left, rect.top - 4);
        }));
    }
```
With:
```js
        const branch = statusEl.querySelector('[data-status-branch]');
        if (branch) branch.addEventListener('click', bind(() => {
            const rect = branch.getBoundingClientRect();
            showGitBranchMenu(rect.left, rect.top - 4);
        }));
        const lspStatus = statusEl.querySelector('[data-status-lsp]');
        if (lspStatus) lspStatus.addEventListener('click', bind(() => lspRestart(lspStatus.dataset.statusLsp))); // IDE package 2b
    }
```

- [ ] **Step 6: Read-only turns LSP off (`applyStudioPolicy`, `core.js`)**

Replace:
```js
            if (view.textarea) view.textarea.readOnly = readOnly;
        }));
        if (readOnly) {
```
With:
```js
            if (view.textarea) view.textarea.readOnly = readOnly;
        }));
        lspApplyAvailability(); // IDE package 2b: a read-only desktop turns LSP off; the bridge would refuse it anyway
        if (readOnly) {
```

- [ ] **Step 7: Append to the end of `ui/css/code-studio.css`**

```css

/* === IDE package 2b: LSP client core === */
.code-studio-statusbar .cs-status-lsp {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 26em;
}

.cs-lsp-dot {
    flex: 0 0 auto;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--cs-muted);
}

.cs-lsp-dot[data-lsp-state="ready"] {
    background: var(--cs-success);
}

.cs-lsp-dot[data-lsp-state="starting"],
.cs-lsp-dot[data-lsp-state="disconnected"] {
    background: var(--cs-warning);
    animation: cs-pulse 1.2s ease-in-out infinite;
}

.cs-lsp-dot[data-lsp-state="not-installed"],
.cs-lsp-dot[data-lsp-state="too-many"],
.cs-lsp-dot[data-lsp-state="error"] {
    background: var(--cs-danger);
}

.cs-status-lsp-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* Narrow windows keep the dot; the state stays in the tooltip. */
@media (max-width: 900px) {
    .cs-status-lsp-label {
        display: none;
    }
}

@media (prefers-reduced-motion: reduce) {
    .cs-lsp-dot[data-lsp-state] {
        animation: none;
    }
}

/* Hover, completion details and signature help from the language servers. */
.code-studio .cm-tooltip .cm-lsp-documentation {
    max-width: min(560px, 70vw);
    max-height: 320px;
    overflow: auto;
    padding: 6px 10px;
    font-family: var(--cs-ui-font);
    font-size: 12px;
    line-height: 1.45;
}

.code-studio .cm-lsp-documentation p,
.code-studio .cm-lsp-documentation pre {
    margin: 4px 0;
}

.code-studio .cm-lsp-documentation pre,
.code-studio .cm-lsp-documentation code {
    font-family: var(--cs-mono-font);
    font-size: 12px;
}

.code-studio .cm-lsp-documentation pre {
    white-space: pre-wrap;
    word-break: break-word;
}

.code-studio .cm-lsp-documentation a {
    color: var(--cs-accent);
}

.code-studio .cm-lsp-signature-tooltip {
    font-size: 12px;
}

.code-studio .cm-lsp-signature-tooltip .cm-lsp-active-parameter {
    color: var(--cs-accent);
}
```

- [ ] **Step 8: Foreign-edit check, build, checks**

```
git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js ui/css/code-studio.css
npm run build:ui
npm run build:ui -- --check
node -e "new (require('vm').Script)(require('fs').readFileSync('ui/js/desktop/bundles/code-studio.bundle.js','utf8'),{filename:'code-studio.bundle.js'})"
GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./ui -count=1 -v -run '(?i)^(TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopCodeStudioShellI18n|TestShippedWebAssetsLoadNothingExternal)$'
```
Expected:
- Only `settings.js`, `core.js` and `code-studio.css` before the build.
- `Built` and `Checked ... (20 parts)`.
- The vm parse prints nothing.
- 3 `--- PASS` lines.

- [ ] **Step 9: Commit**

Run `detect-changes --scope all`. Message `msg-task7.txt`:
```
feat(code-studio): LSP status item, settings switch and read-only policy

The status bar shows the active file's server (starting, ready, idle,
disconnected, not installed, too many, error); a click restarts it. The
editor settings get "Code intelligence (LSP)", on by default, and a
read-only desktop turns LSP off.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task7.txt ui/js/desktop/apps/code-studio/settings.js ui/js/desktop/apps/code-studio/core.js ui/css/code-studio.css ui/js/desktop/bundles/code-studio.bundle.js
```

---

### Task 8: Report Code Studio's own writes to the servers

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/core.js` (`saveTab`), `ui/js/desktop/apps/code-studio/replace.js` (`projectReplaceInFile`), `ui/js/desktop/apps/code-studio/git.js` (`runGitPathAction`, `changeGitBranch`)
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Impact check.** Run `impact` upstream for `saveTab`, `projectReplaceInFile`, `runGitPathAction` and `changeGitBranch`. Warn if HIGH. The edits are additive, and `lspNotifyFilesChanged` returns at once without a ready server.

- [ ] **Step 2: Save (`saveTab`, `core.js`)**

Replace:
```js
            return runWithInstance(target, () => {
                scheduleGitRefresh();
                tab.modified = editorValue(tab) !== content;
```
With:
```js
            return runWithInstance(target, () => {
                scheduleGitRefresh();
                lspNotifyFilesChanged([{ path, type: 2 }]); // IDE package 2b: didSave + didChangeWatchedFiles
                tab.modified = editorValue(tab) !== content;
```

- [ ] **Step 3: Project-wide replace of a closed file (`projectReplaceInFile`, `replace.js`)**

Open tabs change in the buffer; the normal sync carries those edits.

Replace:
```js
        try {
            await apiClient.writeFile(file.path, plan.content);
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        const late = isLiveInstance(target) ? runWithInstance(target, () => projectReplaceTab(file.path)) : null;
```
With:
```js
        try {
            await apiClient.writeFile(file.path, plan.content);
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        // IDE package 2b: the language servers re-read the closed file the replace just wrote.
        if (isLiveInstance(target)) runWithInstance(target, () => lspNotifyFilesChanged([{ path: file.path, type: 2 }]));
        const late = isLiveInstance(target) ? runWithInstance(target, () => projectReplaceTab(file.path)) : null;
```

- [ ] **Step 4: Git discard (`runGitPathAction`, `git.js`)**

Replace:
```js
        if (!isLiveInstance(target)) return false;
        runWithInstance(target, () => {
            refreshGitStatus();
            if (action === 'discard') refreshGitWorkspaceView();
        });
        return true;
```
With:
```js
        if (!isLiveInstance(target)) return false;
        runWithInstance(target, () => {
            if (action === 'discard') {
                // IDE package 2b: the language servers re-read the restored files; a discarded untracked file is gone.
                const untracked = new Set(gitChangeGroups().changes.filter(gitChangeUntracked).map(change => change.path));
                lspNotifyFilesChanged(list.map(path => ({ path, type: untracked.has(path) ? 3 : 2 })));
            }
            refreshGitStatus();
            if (action === 'discard') refreshGitWorkspaceView();
        });
        return true;
```

- [ ] **Step 5: Branch switch (`changeGitBranch`, `git.js`)**

Replace:
```js
                flashStatus(message, 4000);
                refreshGitStatus();
                if (action !== 'delete') refreshGitWorkspaceView();
            });
```
With:
```js
                flashStatus(message, 4000);
                refreshGitStatus();
                if (action !== 'delete') refreshGitWorkspaceView();
                if (action === 'switch') lspNotifyBranchSwitch(); // IDE package 2b
            });
```

- [ ] **Step 6: Foreign-edit check, build, parse check**

```
git status --short -- ui/js/desktop/apps/code-studio scripts/build-ui-bundles.js ui/js/desktop/bundles/code-studio.bundle.js
npm run build:ui
npm run build:ui -- --check
node -e "new (require('vm').Script)(require('fs').readFileSync('ui/js/desktop/bundles/code-studio.bundle.js','utf8'),{filename:'code-studio.bundle.js'})"
```
Expected:
- Only `core.js`, `replace.js` and `git.js` before the build.
- `Built` and `Checked ... (20 parts)`.
- The vm parse prints nothing.

- [ ] **Step 7: Commit**

Run `detect-changes --scope all`. Message `msg-task8.txt`:
```
feat(code-studio): report Code Studio writes to the language servers

Save, project-wide replace of closed files, git discard and branch switch
send didSave and workspace/didChangeWatchedFiles to the ready servers.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task8.txt ui/js/desktop/apps/code-studio/core.js ui/js/desktop/apps/code-studio/replace.js ui/js/desktop/apps/code-studio/git.js ui/js/desktop/bundles/code-studio.bundle.js
```

---

### Task 9: The browser test with a scripted fake LSP endpoint

**Files:**
- Modify: `ui/desktop_code_studio_browser_test.go` (fixture backend: `lspHandler` field and `/api/code-studio/lsp` route)
- Create: `ui/desktop_code_studio_lsp_browser_test.go`

Without a handler, the route closes with 4404, so every other Code Studio browser test sees a quiet "not installed".

- [ ] **Step 1: Impact check.** Run `impact` upstream for `register` (method of `codeStudioFixtureBackend`). Expected: `newCodeStudioBrowser` and through it every Code Studio browser test. Warn if HIGH. The route is additive.

- [ ] **Step 2: Add the handler field to `codeStudioFixtureBackend`**

Replace:
```go
	// searchTruncated makes the fake search answer like the server at its 200-result cap.
	searchTruncated bool

	mu          sync.Mutex
```
With:
```go
	// searchTruncated makes the fake search answer like the server at its 200-result cap.
	searchTruncated bool
	// lspHandler serves /api/code-studio/lsp after the upgrade (IDE package 2b). Nil closes with 4404 like an image
	// without language servers. Set it under mu.
	lspHandler func(conn *websocket.Conn, lang string)

	mu          sync.Mutex
```

- [ ] **Step 3: Add the route before the `/api/` catch-all**

Replace:
```go
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]interface{}{}) })
}
```
With:
```go
	// IDE package 2b: the language server bridge, one JSON-RPC message per text frame.
	mux.HandleFunc("/api/code-studio/lsp", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		handler := b.lspHandler
		b.mu.Unlock()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if handler != nil {
			handler(conn, r.URL.Query().Get("lang"))
			return
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4404, "language server not installed"), time.Now().Add(time.Second))
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]interface{}{}) })
}
```

- [ ] **Step 4: Create `ui/desktop_code_studio_lsp_browser_test.go`**

````go
package ui

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/gorilla/websocket"
)

// fakeCodeStudioLSP plays gopls behind the Code Studio bridge: one JSON-RPC message per WebSocket text frame.
type fakeCodeStudioLSP struct {
	mu         sync.Mutex
	langs      []string
	methods    []string
	replies    []string
	initParams string
	opened     []string
}

func fakeLSPRange(startLine, startCharacter, endLine, endCharacter int) map[string]interface{} {
	return map[string]interface{}{
		"start": map[string]int{"line": startLine, "character": startCharacter},
		"end":   map[string]int{"line": endLine, "character": endCharacter},
	}
}

func (f *fakeCodeStudioLSP) serve(conn *websocket.Conn, lang string) {
	f.mu.Lock()
	f.langs = append(f.langs, lang)
	f.mu.Unlock()
	send := func(message map[string]interface{}) {
		message["jsonrpc"] = "2.0"
		if data, err := json.Marshal(message); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	}
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		f.mu.Lock()
		if msg.Method != "" {
			f.methods = append(f.methods, msg.Method)
		} else if len(msg.ID) > 0 {
			f.replies = append(f.replies, string(payload))
		}
		f.mu.Unlock()
		switch msg.Method {
		case "initialize":
			f.mu.Lock()
			f.initParams = string(msg.Params)
			f.mu.Unlock()
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{
				"capabilities": map[string]interface{}{"textDocumentSync": 2, "hoverProvider": true, "definitionProvider": true},
				"serverInfo":   map[string]string{"name": "fake-gopls"},
			}})
		case "initialized":
			// A server-to-client request: the page must answer it itself (lspOnServerRequest default).
			send(map[string]interface{}{"id": "reg-1", "method": "client/registerCapability", "params": map[string]interface{}{"registrations": []interface{}{}}})
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI        string `json:"uri"`
					LanguageID string `json:"languageId"`
					Version    int    `json:"version"`
					Text       string `json:"text"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			doc := params.TextDocument
			f.mu.Lock()
			f.opened = append(f.opened, doc.URI+"|"+doc.LanguageID+"|"+doc.Text)
			f.mu.Unlock()
			send(map[string]interface{}{"method": "textDocument/publishDiagnostics", "params": map[string]interface{}{
				"uri":     doc.URI,
				"version": doc.Version,
				"diagnostics": []interface{}{map[string]interface{}{
					"range": fakeLSPRange(8, 1, 8, 5), "severity": 1, "code": "UndeclaredName", "source": "compiler", "message": "undefined: fmtx",
				}},
			}})
		case "textDocument/hover":
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{"contents": map[string]string{
				"kind":  "markdown",
				"value": "```go\nfunc greet(name string) string\n```\n\nGreet says hello. <img src=\"https://example.invalid/x.png\" onerror=\"window.lspXSS=1\"><script>window.lspXSS=2</script>",
			}}})
		case "textDocument/definition":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{"uri": params.TextDocument.URI, "range": fakeLSPRange(2, 5, 2, 10)}})
		default:
			if len(msg.ID) > 0 && msg.Method != "" {
				send(map[string]interface{}{"id": msg.ID, "result": nil})
			}
		}
	}
}

// Opening a .go file starts the go connection; a diagnostic reaches the Problems panel, the hover tooltip shows
// sanitized server Markdown and F12 moves the cursor to the definition.
func TestDesktopCodeStudioLSPBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	fake := &fakeCodeStudioLSP{}
	const source = "package main\n\nfunc greet(name string) string {\n\treturn \"hi \" + name\n}\n\nfunc main() {\n\tprintln(greet(\"x\"))\n\tfmtx.Println()\n}\n"
	backend.mu.Lock()
	backend.dirs["/workspace/lsp"] = true
	backend.files["/workspace/lsp/main.go"] = source
	backend.lspHandler = fake.serve
	backend.mu.Unlock()
	browserErrors := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }

	opened, err := page.Eval(`async()=>{
		await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);
		const id=csState().windowId;
		await CodeStudioApp.openFile('/workspace/lsp/main.go',true,id);
		await csWait(()=>csState().lsp&&csState().lsp.connections.go&&csState().lsp.connections.go.state==='ready',15000);
		await csWait(()=>csState().problems.some(p=>p.source==='gopls'),10000);
		await CodeStudioApp.command('setActivePanel',['problems'],id);
		await csWait(()=>csRoot().querySelector('[data-status-lsp="go"]')&&csRoot().querySelector('.cm-lintRange-error'),5000);
		const problem=csState().problems.find(p=>p.source==='gopls');
		return {
			panel:csRoot().querySelector('[data-problems-panel]').textContent.includes('undefined: fmtx'),
			message:problem.message,line:problem.line,column:problem.column,endColumn:problem.endColumn,
			lspSource:problem.lsp?problem.lsp.source:'',lspCode:problem.lsp?problem.lsp.code:'',lspLine:problem.lsp?problem.lsp.range.start.line:-1,
			panelTabs:csRoot().querySelectorAll('[data-panel-tab]').length,
			status:csRoot().querySelector('[data-status-lsp="go"]').dataset.lspState,
			squiggle:csRoot().querySelector('.cm-lintRange-error').textContent
		};
	}`)
	if err != nil {
		t.Fatalf("Go file did not reach a ready language server with a diagnostic: %v; browser errors: %s", err, browserErrors())
	}
	got := opened.Value
	if !got.Get("panel").Bool() || got.Get("message").Str() != "undefined: fmtx" || got.Get("line").Int() != 9 ||
		got.Get("column").Int() != 2 || got.Get("endColumn").Int() != 6 || got.Get("status").Str() != "ready" ||
		got.Get("squiggle").Str() != "fmtx" || got.Get("lspSource").Str() != "compiler" || got.Get("lspCode").Str() != "UndeclaredName" ||
		got.Get("lspLine").Int() != 8 || got.Get("panelTabs").Int() != 3 {
		t.Fatalf("diagnostic, raw diagnostic, squiggle, status or panel tabs wrong: %s; browser errors: %s", got.JSON("", ""), browserErrors())
	}

	point, err := page.Eval(`()=>{
		const view=csState().openTabs[csState().activeTabIndex].view;
		const line=view.state.doc.line(8);
		const c=view.coordsAtPos(line.from+line.text.indexOf('greet')+2);
		return c?[c.left+1,(c.top+c.bottom)/2]:[];
	}`)
	if err != nil || len(point.Value.Arr()) != 2 {
		t.Fatalf("no screen position for greet: %v", err)
	}
	page.Mouse.MustMoveTo(4, 4)
	page.Mouse.MustMoveTo(point.Value.Arr()[0].Num(), point.Value.Arr()[1].Num())
	hover, err := page.Eval(`async()=>{
		const tip=await csWait(()=>document.querySelector('.cm-lsp-hover-tooltip'),8000);
		return {text:tip.textContent,unsafe:!!tip.querySelector('img,script,[onerror]'),clean:window.lspXSS===undefined};
	}`)
	if err != nil {
		t.Fatalf("hover tooltip did not show: %v; browser errors: %s", err, browserErrors())
	}
	h := hover.Value
	if text := h.Get("text").Str(); !strings.Contains(text, "func greet(name string) string") || !strings.Contains(text, "Greet says hello.") ||
		h.Get("unsafe").Bool() || !h.Get("clean").Bool() {
		t.Fatalf("hover content wrong or unsanitized: %s", h.JSON("", ""))
	}

	if _, err := page.Eval(`()=>{
		const view=csState().openTabs[csState().activeTabIndex].view;
		const line=view.state.doc.line(8);
		view.focus();
		view.dispatch({selection:{anchor:line.from+line.text.indexOf('greet')+1}});
		return true;
	}`); err != nil {
		t.Fatalf("placing the cursor failed: %v", err)
	}
	page.Keyboard.MustType(input.F12)
	if _, err := page.Eval(`async()=>{
		await csWait(()=>{
			const view=csState().openTabs[csState().activeTabIndex].view;
			const head=view.state.selection.main.head,line=view.state.doc.lineAt(head);
			return line.number===3&&head-line.from===5;
		},8000);
		return true;
	}`); err != nil {
		t.Fatalf("F12 did not move the cursor to the definition: %v; browser errors: %s", err, browserErrors())
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if strings.Join(fake.langs, ",") != "go" {
		t.Fatalf("language server sockets = %v, want one for go", fake.langs)
	}
	var init struct {
		RootURI          string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"workspaceFolders"`
		Capabilities struct {
			Workspace struct {
				Configuration bool `json:"configuration"`
				ApplyEdit     bool `json:"applyEdit"`
				WorkspaceEdit struct {
					DocumentChanges bool `json:"documentChanges"`
				} `json:"workspaceEdit"`
			} `json:"workspace"`
			TextDocument struct {
				Rename struct {
					PrepareSupport bool `json:"prepareSupport"`
				} `json:"rename"`
				CodeAction struct {
					CodeActionLiteralSupport struct {
						CodeActionKind struct {
							ValueSet []string `json:"valueSet"`
						} `json:"codeActionKind"`
					} `json:"codeActionLiteralSupport"`
					DataSupport    bool `json:"dataSupport"`
					ResolveSupport struct {
						Properties []string `json:"properties"`
					} `json:"resolveSupport"`
				} `json:"codeAction"`
				Hover struct {
					ContentFormat []string `json:"contentFormat"`
				} `json:"hover"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(fake.initParams), &init); err != nil {
		t.Fatalf("initialize params: %v (%s)", err, fake.initParams)
	}
	caps := init.Capabilities
	if init.RootURI != "file:///workspace" || len(init.WorkspaceFolders) != 1 || init.WorkspaceFolders[0].URI != "file:///workspace" ||
		!caps.Workspace.Configuration || !caps.Workspace.ApplyEdit || !caps.Workspace.WorkspaceEdit.DocumentChanges ||
		!caps.TextDocument.Rename.PrepareSupport || !caps.TextDocument.CodeAction.DataSupport ||
		strings.Join(caps.TextDocument.CodeAction.ResolveSupport.Properties, ",") != "edit" ||
		!strings.Contains(","+strings.Join(caps.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet, ",")+",", ",quickfix,") ||
		len(caps.TextDocument.Hover.ContentFormat) == 0 {
		t.Fatalf("initialize params = %s", fake.initParams)
	}
	if len(fake.opened) == 0 || fake.opened[0] != "file:///workspace/lsp/main.go|go|"+source {
		t.Fatalf("didOpen = %q", fake.opened)
	}
	answered := false
	for _, reply := range fake.replies {
		if strings.Contains(reply, `"id":"reg-1"`) && strings.Contains(reply, `"result":null`) {
			answered = true
		}
	}
	if !answered {
		t.Fatalf("client/registerCapability was not answered by the page: replies=%q", fake.replies)
	}
	methods := "," + strings.Join(fake.methods, ",") + ","
	for _, want := range []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/hover", "textDocument/definition"} {
		if !strings.Contains(methods, ","+want+",") {
			t.Fatalf("server never saw %s: %s", want, methods)
		}
	}
	if errs := browserErrors(); errs != "[]" {
		t.Fatalf("browser errors: %s", errs)
	}
}
````

The `Hover.ContentFormat` check proves that the merge kept the 6.3.1 defaults. The `panelTabs == 3` check guards the "no new bottom-panel tab" rule.

- [ ] **Step 5: Format and run the test** (PowerShell, sandbox off)

The test serves the embedded UI, so it uses the bundles built in Tasks 1 to 8.
```
gofmt -w ui/desktop_code_studio_browser_test.go ui/desktop_code_studio_lsp_browser_test.go
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:GOTOOLCHAIN='go1.27.4'; $env:GOTMPDIR='C:\tmp\go-build-lsp'; go test ./ui -run '(?i)CodeStudioLSP' -count=1 -v
```
Expected: `--- PASS: TestDesktopCodeStudioLSPBrowser`, then `ok  	aurago/ui`.

- [ ] **Step 6: Commit**

Run `detect-changes --scope all`. Message `msg-task9.txt`:
```
test(code-studio): browser test for diagnostics, hover and F12 over a fake LSP

The Code Studio fixture backend gets the /api/code-studio/lsp route (4404
without a handler). The new test scripts a gopls-like endpoint and checks the
Problems panel (with the raw diagnostic), the sanitized hover tooltip, F12,
the initialize parameters and capabilities, and the page's answer to a
server request.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
bash C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/c4dd6092-7ce9-4f53-87af-53da26bef5b6/scratchpad/commit-paths.sh C:/disposable/code-studio-pkg2b/msg-task9.txt ui/desktop_code_studio_browser_test.go ui/desktop_code_studio_lsp_browser_test.go
```

---

### Task 10: Final verification (no new code unless something fails)

- [ ] **Step 1: Node test and static Go checks**

```
node --test ui/js/desktop/apps/code-studio/lsp-protocol.test.cjs
npm run build:ui -- --check
GOTOOLCHAIN=go1.27.4 GOTMPDIR='C:\tmp\go-build-lsp' go test ./ui -count=1 -run '(?i)^(TestCodeStudio|TestCodeMirrorBundleExportsYAMLAndLint|TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopCodeStudio(Shell|Copy|Cursor)I18n|TestShippedWebAssetsLoadNothingExternal)'
```
Expected:
- `# pass 6`, `# fail 0`.
- `Checked ui/js/desktop/bundles/code-studio.bundle.js (20 parts)`.
- `ok  	aurago/ui`.

- [ ] **Step 2: Code Studio browser regression run** (PowerShell, sandbox off; about 5 to 10 minutes)

```
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:GOTOOLCHAIN='go1.27.4'; $env:GOTMPDIR='C:\tmp\go-build-lsp'; go test ./ui -run '(?i)^TestDesktopCodeStudio' -count=1 -timeout 30m
```
Expected: `ok  	aurago/ui`. The existing test that expects three bottom-panel tabs must still pass.

If a test fails, look at the LSP status item first: the narrow layout hides the label below 900 px, and `lspScheduleStatus` can overwrite a transient `renderStatus(message)`. Fix forward in the task's files, rebuild and commit the same way.

- [ ] **Step 3: Graph review of the whole package**

```
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref <sha of main before Task 1> --repo .
```
Expected: the changed symbols are this plan's:
- `normalizeProblem`, `applyDiagnosticsToTab`, `checkDocumentContent`, `replaceProblemsForFile`
- `createCodeMirrorEditor`, `normalizeEditorSettings`, `openEditorSettings`, `applyEditorSettingsToOpenEditors`
- `renderStatus`, `applyStudioPolicy`, `saveTab`, `projectReplaceInFile`, `runGitPathAction`, `changeGitBranch`
- the new lsp functions and the fixture `register`

Report any HIGH or CRITICAL entries. Nothing is pushed.

## Contract coverage checklist (for plan 2c)

- [ ] `lsp-protocol.js` exports `languageForPath`, `uriForPath`, `pathForUri`, `diagnosticsToProblems` (with `problem.lsp`), `positionToOffset` and `offsetToPosition` (plus the extras). Problem sources are exactly `gopls`, `typescript`, `pyright` and `clangd`.
- [ ] `normalizeProblem` and `replaceProblemsForFile` keep `problem.lsp`.
- [ ] `lsp.js` declares `lspEnsureClient`, `lspClientState`, `lspRequest` (own 3 s default timer), `lspOnServerRequest`, `lspNotifyFilesChanged`, `lspActionExtensionFactories`, `lspViewForPath` and `lspTouch` at the top level of the code-studio IIFE.
- [ ] `LSPClient` `timeout` is 15000 ms. The client capabilities include `workspace.applyEdit`, `workspaceEdit.documentChanges`, `rename.prepareSupport` and the full `codeAction` block, merged through `{ clientCapabilities }`.
- [ ] The four default server-request handlers are registered. `workspace/applyEdit` is left for 2c.
- [ ] No 2b top-level name starts with `lspAct` or `LSP_ACT_` except the contract array `lspActionExtensionFactories`. None of `ensureReferencesPanelParts`, `renderReferencesPanel`, `clearReferences`, `lspFindReferencesCommand`, `lspRenameSymbolCommand`, `lspQuickFixCommand` or `CodeStudioLSPEdits` is defined in 2b.
- [ ] No bottom-panel tab is added; the browser test asserts three tabs.
- [ ] The bundle list leaves the slot for `lsp-edits.js` and `lsp-actions.js` right after `lsp.js`.
- [ ] 2b binds F12, Ctrl/Cmd+Click (Alt+Click cursors while ready) and Ctrl+Shift+Space only.
