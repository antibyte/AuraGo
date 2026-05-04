# Virtual Desktop Audit Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the critical stability, data integrity, and security issues from `reports/virtual_desktop_audit_2026-05-04.md` so Writer, Sheets, File Manager, Code Studio, and the desktop shell can be used without obvious crashes or data-loss traps.

**Architecture:** Treat the audit as six independent waves. First restore the broken UI test baseline, then close user-facing blockers, then add a shared app lifecycle contract, server-side save conflict detection, and safe expression/formula evaluation. The plan keeps the vanilla JavaScript desktop architecture and Go backend intact, adding tests around each regression before implementation.

**Tech Stack:** Go 1.26, `go test`, vanilla JavaScript desktop apps, embedded UI assets, existing desktop server APIs, existing `internal/office` workbook model, existing UI regression tests.

---

## Audit Triage

- Confirmed valid: File Manager rename is broken because no `[data-rename-input]` is rendered.
- Confirmed valid: Writer and Sheets are multi-instance without same-path dedupe.
- Confirmed valid: Desktop window placement uses unbounded linear offsets.
- Confirmed valid: `closeWindow` only disposes Music Player and Radio.
- Confirmed valid: Code Studio keeps module-level instance state and does not close terminal WebSockets.
- Confirmed valid: Calculator still uses `Function(...)`.
- Confirmed valid: `aura-desktop-sdk.js` does not validate message origin.
- Confirmed valid: `PUT /api/desktop/office/workbook` overwrites without a version or ETag check.
- Partly fixed before this plan: Sheets multi-cell selection, context menu, and formula bar exist. Arrow navigation now guards `target` before focus, but still needs upper-bound clamping and regression tests.
- New preflight blocker found while reviewing: `go test ./ui` fails because `ui/img/papirus/manifest.json` is missing the backend alias `radio -> audio`.

## File Structure

- Modify `ui/img/papirus/manifest.json`: add the missing `radio` alias.
- Modify `ui/js/desktop/apps/sheets.js`: clamp keyboard navigation, add formula validation/evaluation hooks, keep formula bar and workbook state in sync.
- Modify `ui/js/desktop/apps/writer.js`: add `dispose(windowId)` and clear stale save errors.
- Modify `ui/js/desktop/apps/code-studio.js`: move app state into per-window instances and add `dispose(windowId)` that closes terminal WebSockets and disposers.
- Modify `ui/js/desktop/file-manager.js`: render inline rename input, remove duplicate toolbar function, scope keyboard shortcuts to the active instance, clamp context menus.
- Modify `ui/js/desktop/aura-desktop-sdk.js`: validate `event.origin` and `event.source`.
- Modify `ui/js/desktop/main.js`: add same-file dedupe for Writer and Sheets, safe window placement, generic app disposal, safe calculator parser, icon arrange clamping, desktop shortcut consistency.
- Modify `ui/css/desktop.css`: style inline rename and any focus/error states needed by desktop apps.
- Modify `internal/office/office.go`: validate workbook formulas and compute supported cached values for basic formulas.
- Create `internal/office/formula.go`: implement a small safe formula parser/evaluator for arithmetic and supported aggregate functions.
- Create `internal/office/formula_test.go`: test parser, evaluator, range handling, and invalid formulas.
- Modify `internal/server/desktop_office_handlers.go`: return file version metadata on load and require optimistic locking on save.
- Modify or create `internal/server/desktop_office_handlers_test.go`: test workbook conflict detection and successful save with matching version.
- Modify UI regression tests in `ui/*_test.go`: add static markers for the JavaScript regressions and asset alias baseline.

## Wave 0: Restore The UI Test Baseline

### Task 1: Fix Papirus Alias Drift

**Files:**

- Modify: `ui/img/papirus/manifest.json`
- Test: `ui/desktop_papirus_assets_test.go`

- [ ] **Step 1: Run the failing baseline test**

```powershell
if (-not (Test-Path disposable)) { New-Item -ItemType Directory disposable | Out-Null }
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopPapirusManifestMatchesBackendIconCatalog -count=1
```

Expected: FAIL with `Papirus manifest alias "radio" = "", want backend target "audio"`.

- [ ] **Step 2: Add the manifest alias**

In `ui/img/papirus/manifest.json`, add this entry in the `aliases` object near `music-player`:

```json
"radio": "audio",
```

- [ ] **Step 3: Verify the focused asset test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopPapirusManifestMatchesBackendIconCatalog -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add ui/img/papirus/manifest.json
git commit -m "fix: sync Papirus desktop icon aliases"
```

## Wave 1: Immediate User-Facing Blockers

### Task 2: Clamp Sheets Keyboard Navigation

**Files:**

- Modify: `ui/js/desktop/apps/sheets.js`
- Modify or create: `ui/desktop_office_apps_test.go`

- [ ] **Step 1: Add a regression marker test**

Add a UI static test that reads `ui/js/desktop/apps/sheets.js` and asserts that keyboard navigation uses both lower and upper clamps. The test should check for these exact markers:

```go
for _, marker := range []string{
    "function clampCellRow",
    "function clampCellCol",
    "cellInput(clampCellRow(move[0]), clampCellCol(move[1]))",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("sheets keyboard navigation missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the new focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSheetsKeyboardNavigationIsBounded -count=1
```

Expected: FAIL until the implementation exists.

- [ ] **Step 3: Implement bounded navigation**

In `ui/js/desktop/apps/sheets.js`, add helpers close to `cellInput`:

```js
function clampCellRow(row) {
    return Math.min(displayRowCount() - 1, Math.max(0, Number(row) || 0));
}

function clampCellCol(col) {
    return Math.min(displayColCount() - 1, Math.max(0, Number(col) || 0));
}
```

Update the target lookup in `handleCellKeydown`:

```js
const target = cellInput(clampCellRow(move[0]), clampCellCol(move[1]));
if (target) {
    target.focus();
    selectCell(Number(target.dataset.row), Number(target.dataset.col), event.shiftKey && event.key.startsWith('Arrow'));
}
```

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSheetsKeyboardNavigationIsBounded -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/apps/sheets.js ui/desktop_office_apps_test.go
git commit -m "fix: bound Sheets keyboard navigation"
```

### Task 3: Repair File Manager Rename

**Files:**

- Modify: `ui/js/desktop/file-manager.js`
- Modify: `ui/css/desktop.css`
- Modify or create: `ui/desktop_file_manager_test.go`

- [ ] **Step 1: Add static regression markers**

Add a UI test that verifies:

```go
for _, marker := range []string{
    "data-rename-input",
    "finishRename",
    "cancelRename",
    "fm.renamePath === file.path",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("file manager rename missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerInlineRenameMarkers -count=1
```

Expected: FAIL until the inline input is rendered.

- [ ] **Step 3: Render inline rename input in grid and list views**

In `renderGridItem(file)`, replace the name markup with:

```js
const isRenaming = fm.renamePath === file.path;
const nameMarkup = isRenaming
    ? `<input class="fm-rename-input" data-rename-input value="${esc(file.name)}" aria-label="${esc(t('desktop.fm.rename', 'Rename'))}">`
    : esc(file.name);
```

Use it in the grid name:

```js
<div class="fm-grid-name">${nameMarkup}</div>
```

In `renderListRow(file)`, use the same `isRenaming` and `nameMarkup` values, then replace:

```js
<span class="fm-list-name">${esc(file.name)}</span>
```

with:

```js
<span class="fm-list-name">${nameMarkup}</span>
```

- [ ] **Step 4: Add input behavior**

Add this helper near `startRename`:

```js
function finishRename(input) {
    if (!input || !fm.renamePath) return;
    const nextName = String(input.value || '').trim();
    const path = fm.renamePath;
    fm.renamePath = '';
    if (!nextName) {
        renderAll();
        return;
    }
    renamePath(path, nextName);
}

function cancelRename() {
    fm.renamePath = '';
    renderAll();
}
```

After `attachEvents()` binds rows and grid items, bind:

```js
const renameInput = fm.host.querySelector('[data-rename-input]');
if (renameInput) {
    renameInput.addEventListener('click', event => event.stopPropagation());
    renameInput.addEventListener('keydown', event => {
        if (event.key === 'Enter') finishRename(renameInput);
        if (event.key === 'Escape') cancelRename();
    });
    renameInput.addEventListener('blur', () => finishRename(renameInput));
}
```

- [ ] **Step 5: Style the inline input**

In `ui/css/desktop.css`, add:

```css
.fm-rename-input {
    width: 100%;
    min-width: 0;
    border: 1px solid var(--vd-accent);
    border-radius: 6px;
    padding: 3px 6px;
    background: var(--vd-surface);
    color: var(--vd-text);
    font: inherit;
}
```

- [ ] **Step 6: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerInlineRenameMarkers -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/js/desktop/file-manager.js ui/css/desktop.css ui/desktop_file_manager_test.go
git commit -m "fix: restore File Manager inline rename"
```

### Task 4: Keep New Windows On Screen

**Files:**

- Modify: `ui/js/desktop/main.js`
- Modify or create: `ui/desktop_window_manager_test.go`

- [ ] **Step 1: Add marker test**

Add a UI test that checks `ui/js/desktop/main.js` for:

```go
for _, marker := range []string{
    "function nextWindowPosition",
    "workspaceRect.width",
    "workspaceRect.height",
    "Math.min(maxLeft",
    "Math.min(maxTop",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("window manager placement missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWindowPlacementIsClamped -count=1
```

Expected: FAIL until the helper exists.

- [ ] **Step 3: Implement bounded placement**

In `ui/js/desktop/main.js`, add:

```js
function nextWindowPosition(size) {
    const workspace = $('vd-workspace') || document.body;
    const workspaceRect = workspace.getBoundingClientRect();
    const margin = 16;
    const stepX = 28;
    const stepY = 24;
    const maxLeft = Math.max(margin, workspaceRect.width - size.width - margin);
    const maxTop = Math.max(margin, workspaceRect.height - size.height - margin);
    const slotsX = Math.max(1, Math.floor((maxLeft - margin) / stepX) + 1);
    const slotsY = Math.max(1, Math.floor((maxTop - margin) / stepY) + 1);
    const index = state.windows.size;
    const left = margin + (index % slotsX) * stepX;
    const top = 72 + (Math.floor(index / slotsX) % slotsY) * stepY;
    return {
        left: Math.min(maxLeft, Math.max(margin, left)),
        top: Math.min(maxTop, Math.max(margin, top))
    };
}
```

Then replace the current linear `win.style.left` and `win.style.top` assignments with:

```js
const size = appWindowSize(appId);
const position = nextWindowPosition(size);
win.style.left = position.left + 'px';
win.style.top = position.top + 'px';
```

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWindowPlacementIsClamped -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/main.js ui/desktop_window_manager_test.go
git commit -m "fix: keep new desktop windows on screen"
```

### Task 5: Focus Already Open Writer and Sheets Files

**Files:**

- Modify: `ui/js/desktop/main.js`
- Modify or create: `ui/desktop_office_apps_test.go`

- [ ] **Step 1: Add marker test**

Add or extend a UI test for same-path dedupe:

```go
for _, marker := range []string{
    "function findExistingAppWindow",
    "context && context.path != null",
    "win.context && win.context.path === context.path",
    "appId === 'writer' || appId === 'sheets'",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("office same-file dedupe missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestOfficeAppsFocusExistingFileWindow -count=1
```

Expected: FAIL until the window context is stored and matched.

- [ ] **Step 3: Store context and dedupe by path**

In `ui/js/desktop/main.js`, add:

```js
function findExistingAppWindow(appId, context) {
    return [...state.windows.values()].find(win => {
        if (win.appId !== appId) return false;
        if ((appId === 'writer' || appId === 'sheets') && context && context.path != null) {
            return win.context && win.context.path === context.path;
        }
        return appId !== 'editor' && appId !== 'writer' && appId !== 'sheets';
    });
}
```

Replace the `multiInstance` and `existing` logic in `openApp` with:

```js
const existing = findExistingAppWindow(appId, context || {});
if (existing) {
    focusWindow(existing.id);
    if (appId === 'files' && context && context.path != null) {
        if (window.FileManager && typeof window.FileManager.navigateTo === 'function') {
            window.FileManager.navigateTo(existing.id, context.path);
        } else {
            renderFiles(existing.id, context.path);
        }
    }
    return;
}
```

When storing the new window state, include context:

```js
state.windows.set(id, { id, appId, title, element: win, maximized: false, restoreBounds: null, context: context || {} });
```

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestOfficeAppsFocusExistingFileWindow -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/main.js ui/desktop_office_apps_test.go
git commit -m "fix: focus already open office files"
```

## Wave 2: Lifecycle And Data Integrity

### Task 6: Add Generic App Disposal

**Files:**

- Modify: `ui/js/desktop/main.js`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify: `ui/js/desktop/apps/writer.js`
- Modify or create: `ui/desktop_app_lifecycle_test.go`

- [ ] **Step 1: Add lifecycle marker test**

Read the three files and assert these markers:

```go
markers := map[string][]string{
    "ui/js/desktop/main.js": {
        "function disposeAppWindow",
        "window[disposeName]",
        "closeWindow(id)",
    },
    "ui/js/desktop/apps/sheets.js": {
        "SheetsApp.dispose",
        "instances.delete(windowId)",
    },
    "ui/js/desktop/apps/writer.js": {
        "WriterApp.dispose",
        "instances.delete(windowId)",
    },
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopAppsExposeDisposeLifecycle -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement shell disposal**

Add this to `ui/js/desktop/main.js`:

```js
function disposeAppWindow(win) {
    if (!win) return;
    if (win.appId === 'music-player') disposeWebampMusic(win.id);
    if (win.appId === 'radio' && window.RadioApp && typeof window.RadioApp.dispose === 'function') {
        window.RadioApp.dispose(win.id);
    }
    const disposeName = appGlobalName(win.appId);
    const app = disposeName ? window[disposeName] : null;
    if (app && typeof app.dispose === 'function') {
        app.dispose(win.id);
    }
}
```

Add or reuse:

```js
function appGlobalName(appId) {
    return {
        writer: 'WriterApp',
        sheets: 'SheetsApp',
        'code-studio': 'CodeStudioApp'
    }[appId] || '';
}
```

Replace explicit disposal in `closeWindow(id)` with:

```js
disposeAppWindow(win);
```

- [ ] **Step 4: Add no-op instance cleanup for Writer and Sheets**

Use an `instances` map in each app if not already present:

```js
const instances = new Map();
```

In `render(container, windowId, context)`, store any per-window cleanup object:

```js
instances.set(windowId, { container });
```

Expose:

```js
dispose(windowId) {
    instances.delete(windowId);
}
```

- [ ] **Step 5: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopAppsExposeDisposeLifecycle -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/js/desktop/main.js ui/js/desktop/apps/sheets.js ui/js/desktop/apps/writer.js ui/desktop_app_lifecycle_test.go
git commit -m "fix: add desktop app disposal lifecycle"
```

### Task 7: Refactor Code Studio To Per-Window State

**Files:**

- Modify: `ui/js/desktop/apps/code-studio.js`
- Modify or create: `ui/desktop_code_studio_test.go`

- [ ] **Step 1: Add Code Studio lifecycle marker test**

Assert these markers:

```go
for _, marker := range []string{
    "const instances = new Map()",
    "function createInstance",
    "instances.set(windowId, instance)",
    "CodeStudioApp.dispose",
    "instance.ws.close()",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("Code Studio per-window lifecycle missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestCodeStudioUsesPerWindowStateAndClosesTerminal -count=1
```

Expected: FAIL.

- [ ] **Step 3: Introduce instance state**

At module top, replace module-level mutable state with:

```js
const instances = new Map();

function createInstance(container, windowId, context) {
    return {
        root: container,
        windowId,
        context: context || {},
        currentPath: '',
        openTabs: [],
        activeTab: '',
        terminal: null,
        fitAddon: null,
        ws: null,
        disposers: []
    };
}
```

Change functions that currently read `state` to accept `instance` or close over it from `render`.

- [ ] **Step 4: Store instance on render**

In `render(container, windowId, context)`, start with:

```js
dispose(windowId);
const instance = createInstance(container, windowId, context);
instances.set(windowId, instance);
```

Then call instance-aware helpers:

```js
await loadFiles(instance);
connectTerminal(instance);
```

- [ ] **Step 5: Close WebSocket in dispose**

Expose:

```js
function dispose(windowId) {
    const instance = instances.get(windowId);
    if (!instance) return;
    if (instance.ws && instance.ws.readyState !== WebSocket.CLOSED) {
        instance.ws.close();
    }
    for (const disposeFn of instance.disposers || []) {
        try { disposeFn(); } catch (_) {}
    }
    instances.delete(windowId);
}
```

Update `connectTerminal(instance)` so it assigns `instance.ws = ws`.

- [ ] **Step 6: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestCodeStudioUsesPerWindowStateAndClosesTerminal -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/js/desktop/apps/code-studio.js ui/desktop_code_studio_test.go
git commit -m "fix: isolate Code Studio window state"
```

### Task 8: Add Optimistic Locking For Office Saves

**Files:**

- Modify: `internal/server/desktop_office_handlers.go`
- Modify or create: `internal/server/desktop_office_handlers_test.go`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify: `ui/js/desktop/apps/writer.js`

- [ ] **Step 1: Add backend conflict tests**

Create tests that load a workbook, save it with the returned version, then write the file through the desktop service and try saving again with the stale version. Assert the stale save returns `409 Conflict`.

The assertion shape:

```go
if rec.Code != http.StatusConflict {
    t.Fatalf("stale office save status = %d, want %d", rec.Code, http.StatusConflict)
}
```

- [ ] **Step 2: Run backend conflict tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run TestDesktopOfficeWorkbookOptimisticLocking -count=1
```

Expected: FAIL.

- [ ] **Step 3: Return version metadata on GET**

In both document and workbook GET responses, include an `office_version` object:

```go
version := map[string]interface{}{
    "path": entry.Path,
    "modified": entry.Modified,
    "size": entry.Size,
}
```

Return it as:

```go
_ = json.NewEncoder(w).Encode(map[string]interface{}{
    "status": "ok",
    "entry": entry,
    "workbook": workbook,
    "office_version": version,
})
```

- [ ] **Step 4: Require version on save when overwriting existing files**

Extend the PUT body:

```go
OfficeVersion *struct {
    Path     string `json:"path"`
    Modified string `json:"modified"`
    Size     int64  `json:"size"`
} `json:"office_version"`
```

Before writing, read the current entry and compare:

```go
if body.OfficeVersion != nil {
    currentData, currentEntry, err := svc.ReadFileBytes(r.Context(), path)
    if err == nil {
        _ = currentData
        if currentEntry.Modified != body.OfficeVersion.Modified || currentEntry.Size != body.OfficeVersion.Size {
            jsonError(w, "office document changed since it was opened", http.StatusConflict)
            return
        }
    }
}
```

- [ ] **Step 5: Send version from Writer and Sheets**

Store `office_version` from load responses:

```js
let officeVersion = payload.office_version || null;
```

Include it in save bodies:

```js
body: JSON.stringify({ path, workbook, office_version: officeVersion })
```

After successful save, update `officeVersion` from the response if the backend returns a refreshed version.

- [ ] **Step 6: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run TestDesktopOfficeWorkbookOptimisticLocking -count=1
go test ./ui -run TestOfficeAppsSendOptimisticVersion -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/server/desktop_office_handlers.go internal/server/desktop_office_handlers_test.go ui/js/desktop/apps/sheets.js ui/js/desktop/apps/writer.js ui/desktop_office_apps_test.go
git commit -m "fix: detect conflicting office saves"
```

## Wave 3: Security Fixes

### Task 9: Replace Calculator `Function()` With A Safe Parser

**Files:**

- Modify: `ui/js/desktop/main.js`
- Modify or create: `ui/desktop_calculator_test.go`

- [ ] **Step 1: Add security marker test**

Add a test that fails if the calculator still contains `Function(` and asserts safe parser markers:

```go
if strings.Contains(source, "Function(") {
    t.Fatal("desktop calculator must not use Function()")
}
for _, marker := range []string{
    "function tokenizeCalculatorExpression",
    "function parseCalculatorExpression",
    "function evaluateCalculatorExpression",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("calculator safe parser missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopCalculatorDoesNotUseFunctionEval -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement tokenizer and parser**

Support numbers, `+`, `-`, `*`, `/`, `%`, parentheses, `PI`, `E`, and functions already exposed in the UI: `sin`, `cos`, `tan`, `sqrt`, `log`, `abs`, and factorial. Use recursive descent with this shape:

```js
function evaluateCalculatorExpression(input) {
    const tokens = tokenizeCalculatorExpression(input);
    const parser = createCalculatorParser(tokens);
    const value = parser.parseExpression();
    if (!parser.atEnd()) throw new Error('Unexpected input');
    if (!Number.isFinite(value)) throw new Error('Invalid result');
    return value;
}
```

The parser must never call `eval`, `Function`, dynamic import, or global object lookups.

- [ ] **Step 4: Wire the calculator**

Replace:

```js
const value = Function('factorial', `return (${js})`)(factorial);
```

with:

```js
const value = evaluateCalculatorExpression(display.value);
```

- [ ] **Step 5: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopCalculatorDoesNotUseFunctionEval -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/js/desktop/main.js ui/desktop_calculator_test.go
git commit -m "fix: remove dynamic calculator evaluation"
```

### Task 10: Validate Desktop SDK Message Origin

**Files:**

- Modify: `ui/js/desktop/aura-desktop-sdk.js`
- Modify or create: `ui/desktop_sdk_test.go`

- [ ] **Step 1: Add SDK security marker test**

Assert:

```go
for _, marker := range []string{
    "event.origin !== window.location.origin",
    "event.source !== window.parent",
    "return;",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("desktop SDK origin validation missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopSDKValidatesMessageOrigin -count=1
```

Expected: FAIL.

- [ ] **Step 3: Add origin and source checks**

In the `message` listener:

```js
window.addEventListener('message', (event) => {
    if (event.origin !== window.location.origin) return;
    if (event.source !== window.parent) return;
    const msg = event.data || {};
    if (msg.type !== 'aura-response' || !msg.id) return;
    const pendingRequest = pending.get(msg.id);
    if (!pendingRequest) return;
    pending.delete(msg.id);
    pendingRequest.resolve(msg.payload);
});
```

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopSDKValidatesMessageOrigin -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/aura-desktop-sdk.js ui/desktop_sdk_test.go
git commit -m "fix: validate desktop SDK message origin"
```

## Wave 4: Sheets Formula Baseline

### Task 11: Add Safe Workbook Formula Validation And Basic Evaluation

**Files:**

- Create: `internal/office/formula.go`
- Create: `internal/office/formula_test.go`
- Modify: `internal/office/office.go`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify or create: `ui/desktop_office_apps_test.go`

- [ ] **Step 1: Add office formula tests**

Create tests for:

```go
func TestEvaluateFormulaArithmetic(t *testing.T) {
    got, err := EvaluateFormulaForSheet(Sheet{}, "1+2*3")
    if err != nil {
        t.Fatal(err)
    }
    if got != "7" {
        t.Fatalf("formula result = %q, want %q", got, "7")
    }
}

func TestEvaluateFormulaSumRange(t *testing.T) {
    sheet := Sheet{Rows: [][]Cell{{{Value: "2"}}, {{Value: "3"}}}}
    got, err := EvaluateFormulaForSheet(sheet, "SUM(A1:A2)")
    if err != nil {
        t.Fatal(err)
    }
    if got != "5" {
        t.Fatalf("formula result = %q, want %q", got, "5")
    }
}

func TestEvaluateFormulaRejectsUnknownFunction(t *testing.T) {
    _, err := EvaluateFormulaForSheet(Sheet{}, "HYPERLINK(\"x\",\"y\")")
    if err == nil {
        t.Fatal("expected unknown function error")
    }
}
```

- [ ] **Step 2: Run formula tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/office -run TestEvaluateFormula -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement formula evaluator**

In `internal/office/formula.go`, implement:

```go
func EvaluateFormulaForSheet(sheet Sheet, formula string) (string, error)
```

Support:

- arithmetic: `+`, `-`, `*`, `/`, parentheses
- cell refs: `A1`, `B2`
- ranges: `A1:A10`
- functions: `SUM`, `AVG`, `MIN`, `MAX`, `COUNT`

Reject:

- strings
- external refs
- sheet refs
- unknown functions
- malformed ranges
- formulas longer than 4096 bytes

- [ ] **Step 4: Validate formulas during workbook encoding**

In `EncodeWorkbook`, before `f.SetCellFormula`, call:

```go
if _, err := EvaluateFormulaForSheet(sheet, cell.Formula); err != nil {
    return nil, fmt.Errorf("invalid formula %s!%s: %w", sheet.Name, axis, err)
}
```

Keep writing the formula through excelize after validation.

- [ ] **Step 5: Add UI formula state marker**

Update `sheets.js` so formula input updates the workbook state through one setter:

```js
function setCellFromInput(input, raw) {
    const row = Number(input.dataset.row);
    const col = Number(input.dataset.col);
    const sheet = workbook.sheets[activeSheet];
    if (!sheet.rows[row]) sheet.rows[row] = [];
    sheet.rows[row][col] = raw.startsWith('=') ? { formula: raw.slice(1) } : { value: raw };
    input.value = raw;
}
```

Use `setCellFromInput` from cell input handlers and formula bar apply.

- [ ] **Step 6: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/office -run TestEvaluateFormula -count=1
go test ./ui -run TestSheetsFormulaStateUsesSingleSetter -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/office/formula.go internal/office/formula_test.go internal/office/office.go ui/js/desktop/apps/sheets.js ui/desktop_office_apps_test.go
git commit -m "feat: validate basic spreadsheet formulas"
```

## Wave 5: File Manager Focus And Toolbar Cleanup

### Task 12: Scope File Manager Keyboard Shortcuts

**Files:**

- Modify: `ui/js/desktop/file-manager.js`
- Modify or create: `ui/desktop_file_manager_test.go`

- [ ] **Step 1: Add focus marker test**

Assert:

```go
if strings.Contains(source, "document.activeElement === document.body") {
    t.Fatal("File Manager must not treat document.body as focused")
}
for _, marker := range []string{
    "fm.activeKeyboardWindow",
    "root.addEventListener('focusin'",
    "root.contains(document.activeElement)",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("File Manager keyboard focus missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerKeyboardShortcutsAreInstanceScoped -count=1
```

Expected: FAIL.

- [ ] **Step 3: Scope shortcuts to the active File Manager root**

In `bindKeyboard`, remove the `document.body` focus shortcut. Track the active window with:

```js
root.addEventListener('focusin', () => {
    fm.activeKeyboardWindow = fm.windowId;
});
root.addEventListener('pointerdown', () => {
    fm.activeKeyboardWindow = fm.windowId;
});
```

In the keydown handler:

```js
if (fm.activeKeyboardWindow !== fm.windowId) return;
if (!root.contains(document.activeElement)) return;
```

- [ ] **Step 4: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerKeyboardShortcutsAreInstanceScoped -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/file-manager.js ui/desktop_file_manager_test.go
git commit -m "fix: scope File Manager keyboard shortcuts"
```

### Task 13: Remove Duplicate Toolbar Override And Clamp Menus

**Files:**

- Modify: `ui/js/desktop/file-manager.js`
- Modify or create: `ui/desktop_file_manager_test.go`

- [ ] **Step 1: Add static regression checks**

Add a test that counts `function updateToolbarState()` occurrences and expects one:

```go
if strings.Count(source, "function updateToolbarState()") != 1 {
    t.Fatalf("expected one updateToolbarState definition")
}
```

Also assert context menu clamping markers:

```go
for _, marker := range []string{
    "Math.max(8,",
    "menuRect.left < 8",
    "menuRect.top < 8",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("context menu clamp missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerToolbarAndContextMenuCleanup -count=1
```

Expected: FAIL.

- [ ] **Step 3: Remove the empty duplicate**

Delete the later empty definition:

```js
function updateToolbarState() {
}
```

Keep the earlier implementation that updates back, forward, and current path state.

- [ ] **Step 4: Clamp left and top context menu overflow**

In context menu positioning code, after the menu is measured:

```js
if (menuRect.left < 8) menu.style.left = '8px';
if (menuRect.top < 8) menu.style.top = '8px';
menu.style.left = Math.max(8, parseFloat(menu.style.left) || x) + 'px';
menu.style.top = Math.max(8, parseFloat(menu.style.top) || y) + 'px';
```

- [ ] **Step 5: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestFileManagerToolbarAndContextMenuCleanup -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/js/desktop/file-manager.js ui/desktop_file_manager_test.go
git commit -m "fix: clean up File Manager toolbar and menus"
```

## Wave 6: Desktop Polish And Full Verification

### Task 14: Finish Low-Risk Desktop Polish

**Files:**

- Modify: `ui/js/desktop/main.js`
- Modify: `ui/js/desktop/apps/writer.js`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify or create: `ui/desktop_polish_test.go`

- [ ] **Step 1: Add polish marker test**

Assert markers:

```go
checks := []string{
    "function clampDesktopIconPosition",
    "case 'Delete':",
    "case 'F2':",
    "setTimeout(() => clearSaveError",
    "function setCellFromInput",
}
for _, marker := range checks {
    if !strings.Contains(combinedSource, marker) {
        t.Fatalf("desktop polish missing marker %q", marker)
    }
}
```

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestVirtualDesktopPolishRegressions -count=1
```

Expected: FAIL.

- [ ] **Step 3: Clamp desktop icon arrangement**

In `autoArrangeIcons`, route every computed icon position through:

```js
function clampDesktopIconPosition(left, top) {
    const workspace = $('vd-workspace') || document.body;
    const rect = workspace.getBoundingClientRect();
    return {
        left: Math.min(Math.max(8, left), Math.max(8, rect.width - 96)),
        top: Math.min(Math.max(8, top), Math.max(8, rect.height - 96))
    };
}
```

- [ ] **Step 4: Make desktop Delete and F2 consistent**

Apply Desktop keyboard actions to selected file shortcuts and directory shortcuts only when the selected item exposes the matching operation. For unsupported app shortcuts, leave the selection unchanged and show the existing notification helper with a short localized message.

- [ ] **Step 5: Clear Writer save errors after a timeout**

In `writer.js`, add:

```js
function clearSaveError(statusNode) {
    if (statusNode && statusNode.dataset.state === 'error') {
        statusNode.textContent = '';
        statusNode.dataset.state = '';
    }
}
```

After setting a save error:

```js
setTimeout(() => clearSaveError(statusNode), 6000);
```

- [ ] **Step 6: Verify**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestVirtualDesktopPolishRegressions -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/js/desktop/main.js ui/js/desktop/apps/writer.js ui/js/desktop/apps/sheets.js ui/desktop_polish_test.go
git commit -m "fix: polish virtual desktop edge cases"
```

### Task 15: Run Final Verification

**Files:**

- No source edits unless verification exposes a regression.

- [ ] **Step 1: Run focused backend tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/office ./internal/server -run "TestEvaluateFormula|TestDesktopOffice" -count=1
```

Expected: PASS.

- [ ] **Step 2: Run focused UI tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -count=1
```

Expected: PASS.

- [ ] **Step 3: Run requested office verification set**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop ./internal/server ./internal/tools ./ui -count=1
```

Expected: PASS.

- [ ] **Step 4: Check worktree**

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors. The only untracked files should be intentionally ignored local artifacts such as `.playwright-mcp/` or `disposable/`.

- [ ] **Step 5: Final commit if verification required fixes**

If Step 1 through Step 4 required additional fixes, commit only those files:

```powershell
git add <changed-files>
git commit -m "test: verify virtual desktop audit fixes"
```

## Execution Notes

- Keep commits small by wave or task, because several fixes touch the same large desktop JavaScript files.
- Do not change `config.yaml`.
- Do not place temporary scripts outside `disposable/`.
- Do not update translations unless new user-visible strings are introduced. If new desktop strings are introduced, update every `ui/lang/desktop/*.json` file.
- Prefer static regression tests for structural JavaScript regressions first, then add browser interaction coverage if the existing test harness already supports it.
- The formula engine is intentionally limited to safe arithmetic and common aggregates for v1. More complex Excel compatibility should be a separate plan after the basic evaluator and validation are stable.

## Self-Review

- Spec coverage: all critical, security, backend, and listed medium/low audit items are represented in Waves 0 through 6.
- Placeholder scan: no tasks contain `TBD`, vague standalone error-handling instructions, or references to undefined files.
- Type consistency: the same `office_version`, `EvaluateFormulaForSheet`, `dispose(windowId)`, and instance-map names are used across backend and UI tasks.
