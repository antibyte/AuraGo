# Virtual Desktop Comprehensive Audit Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` for implementation waves with independent write scopes, or `superpowers:executing-plans` when working sequentially. Track progress by changing each `- [ ]` item to `- [x]` only after its verification command passes.

**Goal:** Resolve the critical and high-risk findings from `reports/virtual_desktop_comprehensive_audit_2026-05-07.md`, then apply the highest-value UX, theme, i18n, and performance optimizations without destabilizing the virtual desktop.

**Architecture:** Keep the existing Go backend, SQLite desktop data model, embedded vanilla JavaScript SPA, and split desktop module files. Fix backend security and cancellation first, then app lifecycle leaks and per-window state, then z-index/theme/i18n polish, and only then performance refactors such as asynchronous script loading and render batching.

**Tech Stack:** Go 1.26, `go test`, PowerShell, vanilla JavaScript, CSS variables, existing embedded UI assets, existing desktop SSE/WebSocket APIs.

---

## Audit Reconciliation

- Source report date: 2026-05-07.
- Report baseline: commit `07557b64`.
- Current head already contains follow-up work not reflected in the report, including App Manager work, circular menu restoration, Fruity dock window content repair, and widget auto-size improvements.
- The plan therefore begins with a small triage pass to mark already-fixed items and avoid reworking current code.
- Header/table mismatch in the report is harmless for execution: it says `189 issues` in the header and `178+` in the executive table. Prioritize the named critical/high findings over the aggregate count.
- Do not attempt all findings in one commit. Use small commits per task or wave, because several fixes touch shared desktop shell files.

## Global Rules For This Plan

- Do not edit `config.yaml`.
- Put temporary scripts only in `disposable/`, then remove them before final verification unless they are intentionally ignored local helpers.
- Use English for code comments and project docs.
- When adding user-visible strings, update every `ui/lang/desktop/*.json` file.
- Prefer focused tests before broad tests.
- Use `apply_patch` for manual source edits.
- Never replace current user work or revert unrelated changes.
- Commit each completed wave with a descriptive commit message.

## Verification Setup

- [ ] Ensure the disposable cache directory exists before running tests:

```powershell
if (-not (Test-Path disposable)) { New-Item -ItemType Directory disposable | Out-Null }
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
```

- [ ] At the end of every wave, run:

```powershell
git diff --check
git status --short
```

- [ ] For backend changes, run the focused package tests first, then:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop ./internal/server -count=1
```

- [ ] For UI/static asset changes, run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -count=1
```

- [ ] Before claiming the whole plan complete, run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./... -count=1
```

## Wave 0: Current-State Triage

**Purpose:** Convert the audit report into a current, actionable checklist before changing production code.

**Files:**

- Read: `reports/virtual_desktop_comprehensive_audit_2026-05-07.md`
- Read: `internal/desktop/service.go`
- Read: `internal/server/desktop_handlers.go`
- Read: `internal/server/desktop_looper_handlers.go`
- Read: `internal/server/looper_service.go`
- Read: `ui/js/desktop/core/module-loader.js`
- Read: `ui/js/desktop/core/window-shell-runtime.js`
- Read: `ui/js/desktop/file-manager/core-render.js`
- Read: `ui/js/desktop/file-manager/lifecycle-export.js`
- Read: `ui/js/desktop/apps/code-studio/core-shell-files.js`
- Read: `ui/js/desktop/apps/writer.js`
- Read: `ui/js/desktop/apps/radio.js`
- Read: `ui/js/desktop/apps/sheets.js`
- Read: `ui/js/desktop/apps/looper.js`
- Read: `ui/js/desktop/apps/camera.js`
- Create temporary triage notes only under `reports/` if needed.

- [ ] Re-run quick source searches for each critical finding and note whether it still exists.

```powershell
rg -n "EvalSymlinks|Loopback|open\('GET'|function runAsyncStep|setInterval\(|window\.FileManager|const fm|let fm|handleLooperRun|SetCancelFn|SetRunning" internal ui -S
```

- [ ] Create `reports/virtual_desktop_audit_triage_2026-05-07.md` only if the implementation session needs a scratch matrix. Keep it in `reports/` because reports are gitignored.
- [ ] Mark any already-fixed item as verified in the scratch matrix with the current commit SHA and source evidence.
- [ ] Do not commit the scratch report unless project policy changes; `reports/` is intentionally ignored.

## Wave 1: Backend Security And Cancellation

**Purpose:** Close path traversal, unbounded copy, request-size, and agent cancellation risks before UI polish.

**Files:**

- Modify: `internal/desktop/service.go`
- Modify: `internal/desktop/service_test.go`
- Modify: `internal/server/desktop_handlers.go`
- Modify: `internal/server/desktop_apps_handler.go` if desktop app POST/PATCH/DELETE bodies are parsed there
- Modify: `internal/server/desktop_widgets_handler.go` if widget visibility/config bodies are parsed there
- Modify: `internal/agent/agent_loop.go` or the file that defines `Loopback`
- Modify tests near existing desktop handler tests, creating focused tests only where needed

### Task 1: Replace `ResolvePath` Symlink Validation

- [ ] Add failing tests in `internal/desktop/service_test.go` for broken symlinks, symlinked parent directories, and symlink loops.
- [ ] Test that a broken symlink inside the workspace cannot be used as an intermediate path for writes or copies.
- [ ] Test that a symlink pointing outside the workspace is rejected even if the final target path does not exist yet.
- [ ] Implement component-by-component path validation using `os.Lstat`.
- [ ] For every component that is a symlink, resolve it relative to its containing directory, normalize it, and verify the resolved absolute path remains inside the workspace.
- [ ] Treat broken symlinks as invalid for all operations that traverse into the symlink.
- [ ] Preserve normal access to valid files and directories inside the workspace.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop -run "Test.*ResolvePath|Test.*Symlink" -count=1
```

### Task 2: Harden Directory Copy

- [ ] Add tests for circular symlink copy attempts and oversized/deep directory copy attempts.
- [ ] Define conservative copy limits in `internal/desktop/service.go`, for example max depth, max entries, and max total bytes.
- [ ] Ensure `CopyPath` never follows unsafe symlinks and uses the same path validation contract as `ResolvePath`.
- [ ] Return user-facing errors that explain when a copy is blocked by safety limits.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop -run "Test.*Copy" -count=1
```

### Task 3: Propagate Request Context Into Desktop Chat

- [ ] Locate the current `agent.Loopback` definition and all call sites.
- [ ] Add `LoopbackContext(ctx context.Context, ...)` or change `Loopback` to accept a context if the call graph is small.
- [ ] Use `r.Context()` in both synchronous desktop chat and streaming desktop chat handlers.
- [ ] Ensure a client disconnect cancels LLM work, broker streaming, and goroutine cleanup.
- [ ] Add handler tests with a canceled context where practical, or static tests that assert desktop chat handlers no longer use `context.Background()`.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run "Test.*Desktop.*Chat|Test.*Loopback" -count=1
```

### Task 4: Add Desktop API Body Limits

- [ ] Add `http.MaxBytesReader` to desktop JSON handlers that accept app, widget, looper, office, or file operation bodies.
- [ ] Use endpoint-appropriate limits: small for visibility toggles, moderate for app manifests/settings, larger only where file content is expected.
- [ ] Add tests that oversized JSON bodies return `413 Request Entity Too Large` or a consistent JSON error.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/server -run "Test.*BodyLimit|Test.*Desktop.*Oversized" -count=1
```

### Task 5: Commit Wave 1

- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop ./internal/server -count=1
git diff --check
```

- [ ] Commit:

```powershell
git add internal/desktop internal/server internal/agent
git commit -m "fix: harden virtual desktop backend safety"
```

## Wave 2: Looper Atomic Execution

**Purpose:** Prevent concurrent Looper runs and cancellation races.

**Files:**

- Modify: `internal/desktop/looper.go`
- Modify: `internal/server/looper_service.go`
- Modify: `internal/server/desktop_looper_handlers.go`
- Modify or create: `internal/desktop/looper_test.go`
- Modify or create: `internal/server/desktop_looper_handlers_test.go`

- [ ] Add a Looper state test that launches two concurrent start attempts and asserts exactly one succeeds.
- [ ] Add a handler or service test for two rapid `/run` requests if the existing test harness supports it.
- [ ] Implement a mutex-protected `TryStart` or `StartRun` method that checks `Running`, sets the cancel function, initializes run metadata, and returns a conflict error atomically.
- [ ] Ensure cancel functions cannot be overwritten by a second run attempt.
- [ ] Keep `State()` read-only and do not use it as the authoritative run gate.
- [ ] Update `handleLooperRun` and `looper_service.Execute` to rely on the atomic start path.
- [ ] Ensure cancellation clears state once and only once.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop ./internal/server -run "Test.*Looper.*Concurrent|Test.*Looper.*Run|Test.*Looper.*Cancel" -count=1
```

- [ ] Commit:

```powershell
git add internal/desktop/looper.go internal/desktop/looper_test.go internal/server/looper_service.go internal/server/desktop_looper_handlers.go internal/server/desktop_looper_handlers_test.go
git commit -m "fix: make Looper runs atomic"
```

## Wave 3: Desktop App Lifecycle And Window Isolation

**Purpose:** Stop leaks, blank/failed app cascades, and cross-window state corruption.

**Files:**

- Modify: `ui/js/desktop/core/window-shell-runtime.js`
- Modify: `ui/js/desktop/core/state.js` if lifecycle state belongs there
- Modify: `ui/js/desktop/core/window-rendering.js` if app render dispatch lives there
- Modify: `ui/js/desktop/file-manager/core-render.js`
- Modify: `ui/js/desktop/file-manager/actions-input.js`
- Modify: `ui/js/desktop/file-manager/lifecycle-export.js`
- Modify: `ui/js/desktop/apps/code-studio/core-shell-files.js`
- Modify: `ui/js/desktop/apps/writer.js`
- Modify: `ui/js/desktop/apps/radio.js`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify: `ui/js/desktop/apps/looper.js`
- Modify: `ui/js/desktop/apps/camera.js`
- Modify or create UI static tests in `ui/`

### Task 1: Standardize `dispose(windowId)`

- [ ] Add or extend a UI static lifecycle test that asserts the shell maps all disposable apps:

```go
for _, marker := range []string{
    "FileManager",
    "WriterApp",
    "SheetsApp",
    "CodeStudioApp",
    "RadioApp",
    "LooperApp",
    "CameraApp",
    ".dispose(win.id)",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("desktop lifecycle missing marker %q", marker)
    }
}
```

- [ ] Update `disposeAppWindow` so it calls the mapped global `dispose(windowId)` for File Manager, Writer, Sheets, Code Studio, Radio, Looper, Camera, Music Player, and any embedded/generated app hook already supported by the shell.
- [ ] Make missing app globals non-fatal.
- [ ] Wrap each dispose call in a small `try/catch` that logs but does not block window closing.
- [ ] Ensure closing a window removes context menus, overlays, SSE handles, XHRs, WebSockets, timers, and per-window maps.

### Task 2: Fix Widget Runtime Leaks

- [ ] Add `state.widgetCleanups` or a similar cleanup registry.
- [ ] Before `renderWidgets()` replaces widget DOM, run and clear every registered widget cleanup.
- [ ] Register clock widget `clearInterval`.
- [ ] Register all widget `ResizeObserver.disconnect()` calls.
- [ ] Ensure rebuilt widgets still resize after the cleanup change.
- [ ] Add static tests for cleanup markers:

```go
for _, marker := range []string{
    "clearWidgetRuntime",
    "clearInterval",
    "disconnect()",
    "widgetCleanups",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("widget runtime cleanup missing marker %q", marker)
    }
}
```

### Task 3: Convert File Manager To Per-Window Instances

- [ ] Replace module-level singleton File Manager state with `const instances = new Map()`.
- [ ] Create an instance in `render(container, windowId, context)`.
- [ ] Pass the instance into helpers or bind helpers to the instance context.
- [ ] Make `navigateTo(windowId, path)` look up the correct instance.
- [ ] Add `dispose(windowId)` to remove listeners, overlays, pending prompts, context menus, and instance state.
- [ ] Confirm two File Manager windows can show different folders without state bleed.
- [ ] Add static tests for:

```go
for _, marker := range []string{
    "const instances = new Map()",
    "function createInstance",
    "instances.set(windowId",
    "function dispose(windowId)",
    "instances.delete(windowId)",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("file manager instance lifecycle missing marker %q", marker)
    }
}
```

### Task 4: Fix Code Studio Async State Restoration

- [ ] Add a focused static test that checks `runAsyncStep` awaits the callback before restoring state.
- [ ] Change `runAsyncStep(instance, fn)` to:

```js
async function runAsyncStep(instance, fn) {
    const previous = state;
    state = instance;
    try {
        return await fn(instance);
    } finally {
        state = previous;
    }
}
```

- [ ] Verify every caller that depends on the current state still awaits `runAsyncStep`.

### Task 5: Complete App-Specific Disposal

- [ ] Writer: destroy Quill if an official destroy API exists in the bundled version; otherwise remove toolbar/root event listeners, clear autosave timers, and delete the instance map entry.
- [ ] Radio: pause audio, clear timers, clear Media Session metadata, and reset media session action handlers to `null`.
- [ ] Looper: close SSE, clear reconnect timers, remove DOM listeners, and cancel in-flight UI work where possible.
- [ ] Camera: abort in-flight XHR, stop media tracks if camera capture uses streams, clear button listeners, and remove instances.
- [ ] Sheets: remove context menus, keyboard listeners, and selection listeners for the closed window.
- [ ] Chat renderer/lightbox if owned by desktop windows: remove delegated lightbox/key handlers on dispose.

### Task 6: Add App Render Error Boundaries

- [ ] Wrap each app renderer call so one failed app module displays a localized error panel inside that window instead of blanking the desktop shell.
- [ ] Add a console error with app id and window id for debugging.
- [ ] Ensure render errors do not prevent the window from closing.
- [ ] Add static test markers:

```go
for _, marker := range []string{
    "renderAppError",
    "try {",
    "catch (err)",
    "appId",
} {
    if !strings.Contains(source, marker) {
        t.Fatalf("app render boundary missing marker %q", marker)
    }
}
```

### Task 7: Commit Wave 3

- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestDesktop.*Lifecycle|TestFileManager.*Instance|TestCodeStudio.*Async|TestWidget.*Cleanup|TestDesktop.*Render" -count=1
go test ./ui -count=1
git diff --check
```

- [ ] Commit:

```powershell
git add ui/js/desktop ui/*.go ui/css
git commit -m "fix: stabilize virtual desktop app lifecycle"
```

## Wave 4: Desktop Interaction Safety

**Purpose:** Prevent keyboard shortcuts, long-press listeners, and bootstrap reloads from interrupting user input.

**Files:**

- Modify: `ui/js/desktop/core/window-shell-runtime.js`
- Modify: `ui/js/desktop/core/input-handlers.js` or the file containing desktop keyboard shortcuts
- Modify: `ui/js/desktop/core/bootstrap.js` or the file containing `loadBootstrap`
- Modify: File Manager input/context files as needed
- Modify or create UI static tests in `ui/`

- [ ] Fix `settingBool` so boolean `false`, string `"false"`, number `0`, and missing values are handled intentionally.
- [ ] Guard desktop Delete/F2 shortcuts so they do not fire while focus is inside `input`, `textarea`, `select`, or `[contenteditable="true"]`.
- [ ] Track and remove context-menu keydown listeners when menus close or are externally removed.
- [ ] Ensure `wireLongPress` registers a bounded click-suppression listener and does not accumulate duplicate capture handlers on repeated renders.
- [ ] Debounce or deduplicate `loadBootstrap` calls so a pending reload does not rebuild the desktop in the middle of drag, resize, inline rename, Writer editing, or Sheets editing.
- [ ] Prefer targeted DOM updates for simple visibility/config changes where that is cheaper than full desktop rebuild.
- [ ] Add tests for the markers:

```go
markers := []string{
    "isEditableTarget",
    "contenteditable",
    "settingBool",
    "bootstrapReloadPromise",
    "removeEventListener('keydown'",
}
```

- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestDesktop.*Keyboard|TestDesktop.*Bootstrap|TestDesktop.*ContextMenu|TestDesktop.*SettingBool" -count=1
go test ./ui -count=1
```

- [ ] Commit:

```powershell
git add ui/js/desktop ui/*.go
git commit -m "fix: protect desktop interactions during edits"
```

## Wave 5: Theme, Z-Index, And Responsive UX

**Purpose:** Make standard and Fruity desktop modes visually consistent in light and dark themes, and stop menus from hiding behind overlays.

**Files:**

- Modify: `ui/css/desktop.css`
- Modify: `ui/css/code-studio.css`
- Modify: `ui/css/radio.css`
- Modify: CSS for Writer, Sheets, Calculator, System Info, and shared desktop variables if they live elsewhere
- Modify: `ui/desktop_theme_test.go` or create a new static CSS test

### Task 1: Establish A Z-Index Scale

- [ ] Define named z-index CSS custom properties in the desktop root, for example:

```css
:root {
    --vd-z-desktop: 0;
    --vd-z-widget: 10;
    --vd-z-window: 100;
    --vd-z-dock: 400;
    --vd-z-menu: 700;
    --vd-z-modal: 800;
    --vd-z-context-menu: 900;
    --vd-z-toast: 1000;
}
```

- [ ] Replace hardcoded context-menu/modal/toast z-index values where practical.
- [ ] Ensure `.vd-context-menu`, `.fm-context-menu`, Sheets context menu, Code Studio context menu, and office menus are above modals that allow contextual actions.
- [ ] Keep destructive confirmation modals above normal menus.
- [ ] Add a static CSS test that asserts the named z-index variables and key selectors use them.

### Task 2: Add Standard Light Theme Coverage

- [ ] Add `[data-theme="light"]` or root variable overrides for standard desktop mode, not only Fruity mode.
- [ ] Convert Code Studio, Radio, Calculator, System Info, Writer, and Sheets hardcoded dark colors to desktop variables or explicit light/dark overrides.
- [ ] Preserve good contrast in dark mode.
- [ ] Ensure Writer's editing surface is fully white in light mode, not just a narrow strip.
- [ ] Ensure Writer icon buttons use theme-aware icon color variables and are not too dark on dark surfaces.
- [ ] Ensure Office/Sheets surfaces do not become hardcoded white in dark mode unless the document canvas is intentionally paper-like.
- [ ] Add static CSS tests for hardcoded color regressions in the audited files.

### Task 3: Responsive And Mobile Fixes

- [ ] Add safe-area support for maximized windows:

```css
top: env(safe-area-inset-top, 0px);
```

- [ ] Fix small touch targets in desktop widgets, app cards, and context menus where the audit identified them.
- [ ] Stabilize File Manager grid row heights and long filename wrapping.
- [ ] Ensure large Sheets grids do not force unusable horizontal viewport overflow on mobile.

### Task 4: Visual Verification

- [ ] If a local dev server is running, use the in-app browser or Playwright-style screenshots to verify:
    - standard desktop light theme
    - standard desktop dark theme
    - Fruity desktop light theme
    - Fruity desktop dark theme
    - Writer editor in light mode
    - File Manager modal with context menu
    - Code Studio and Radio in light mode
- [ ] If browser automation is unavailable, document the skipped visual check and run static CSS tests.

### Task 5: Commit Wave 5

- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestDesktop.*Theme|TestDesktop.*ZIndex|TestDesktop.*Css" -count=1
go test ./ui -count=1
git diff --check
```

- [ ] Commit:

```powershell
git add ui/css ui/*.go
git commit -m "fix: align virtual desktop themes and layering"
```

## Wave 6: i18n Cleanup

**Purpose:** Remove English fallbacks and placeholder drift from desktop UI strings.

**Files:**

- Modify: `ui/lang/desktop/cs.json`
- Modify: `ui/lang/desktop/da.json`
- Modify: `ui/lang/desktop/de.json`
- Modify: `ui/lang/desktop/el.json`
- Modify: `ui/lang/desktop/en.json`
- Modify: `ui/lang/desktop/es.json`
- Modify: `ui/lang/desktop/fr.json`
- Modify: `ui/lang/desktop/hi.json`
- Modify: `ui/lang/desktop/it.json`
- Modify: `ui/lang/desktop/ja.json`
- Modify: `ui/lang/desktop/nl.json`
- Modify: `ui/lang/desktop/no.json`
- Modify: `ui/lang/desktop/pl.json`
- Modify: `ui/lang/desktop/pt.json`
- Modify: `ui/lang/desktop/sv.json`
- Modify: `ui/lang/desktop/zh.json`
- Modify: `ui/js/desktop/apps/sheets.js`
- Modify: `ui/js/desktop/apps/looper.js`
- Modify: `ui/js/desktop/apps/launchpad.js` or wherever Launchpad strings live
- Modify or create i18n tests in `ui/`

- [ ] Create a temporary scanner under `disposable/` if needed to compare `t('desktop...')` keys against every desktop language JSON file.
- [ ] Add the missing Sheets keys to all language files:
    - `desktop.sheets_clear_range`
    - `desktop.sheets_insert_row_above`
    - `desktop.sheets_insert_row_below`
    - `desktop.sheets_insert_col_left`
    - `desktop.sheets_insert_col_right`
    - `desktop.sheets_delete_rows`
    - `desktop.sheets_delete_columns`
- [ ] Translate the settings description keys currently copied from English in non-English files.
- [ ] Translate folder names, wallpaper names, and icon catalog descriptions flagged by the report.
- [ ] Standardize the Looper iteration placeholder style. Pick the existing project convention and update code and translations together.
- [ ] Replace hardcoded Looper notification title strings with translation keys.
- [ ] Replace hardcoded Launchpad placeholder/category strings with translation keys.
- [ ] Add or extend tests that assert every desktop language file contains the same key set and placeholder set.
- [ ] Delete any temporary scanner from `disposable/` if it was created and is no longer needed.
- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestDesktop.*I18n|TestDesktop.*Translation|TestDesktop.*Language" -count=1
go test ./ui -count=1
```

- [ ] Commit:

```powershell
git add ui/lang/desktop ui/js/desktop ui/*.go
git commit -m "fix: complete virtual desktop translations"
```

## Wave 7: Performance Optimizations

**Purpose:** Remove avoidable main-thread stalls and excessive rebuild work after correctness and lifecycle cleanup are stable.

**Files:**

- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/js/desktop/main.js`
- Modify: `ui/js/desktop/code-studio.js`
- Modify: `ui/js/desktop/file-manager.js`
- Modify: `ui/js/desktop/core/window-shell-runtime.js`
- Modify File Manager and Sheets rendering files as needed
- Modify or create UI static tests in `ui/`

### Task 1: Replace Synchronous XHR Safely

- [ ] Do not async-load each current script part independently unless every part is syntactically standalone.
- [ ] Preserve the current ordered concatenation behavior first: fetch all parts asynchronously, concatenate in order, then evaluate once.
- [ ] Return a Promise from `loadScriptParts`.
- [ ] Update callers (`main.js`, `code-studio.js`, `file-manager.js`, and any other split module shell) to wait for the Promise or dispatch a ready event before dependent initialization runs.
- [ ] Add error UI/logging for failed part loads.
- [ ] Add a static test that forbids `xhr.open('GET', part, false)`.
- [ ] Add a static test that requires a Promise-returning loader and ordered concatenation marker.

### Task 2: Batch Hot DOM Updates

- [ ] Use `requestAnimationFrame` for window drag and resize style writes.
- [ ] Batch chat streaming text updates with `requestAnimationFrame`.
- [ ] Avoid repeated layout reads and writes in the same pointermove.
- [ ] Add marker tests for drag/resize RAF scheduling.

### Task 3: Cache And Target Desktop Data Updates

- [ ] Cache `allApps()` results and invalidate only when the app list or visibility changes.
- [ ] Deduplicate rapid `loadBootstrap` requests.
- [ ] For app visibility, widget visibility, and settings updates, update affected DOM sections where practical instead of full `innerHTML` rebuilds.
- [ ] Ensure hidden apps remain available to the App Manager and search logic as required by the App Manager design.

### Task 4: Improve Large Lists And Grids

- [ ] Add File Manager virtual scrolling or incremental rendering for large directories.
- [ ] Add a threshold before virtualization so small folders keep simple rendering.
- [ ] For Sheets, avoid full table rebuilds on single-cell edits and selection changes.
- [ ] Add static or unit-level tests around the new virtualization thresholds if browser tests are not available.

### Task 5: Commit Wave 7

- [ ] Run:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestDesktop.*ModuleLoader|TestDesktop.*Performance|TestDesktop.*RAF|TestFileManager.*Virtual|TestSheets.*Render" -count=1
go test ./ui -count=1
git diff --check
```

- [ ] Commit:

```powershell
git add ui/js/desktop ui/*.go
git commit -m "perf: optimize virtual desktop rendering"
```

## Wave 8: Final Regression Sweep

**Purpose:** Prove the whole desktop still works after the high-risk waves.

- [ ] Run focused backend packages:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./internal/desktop ./internal/server -count=1
```

- [ ] Run focused UI package:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -count=1
```

- [ ] Run all tests:

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./... -count=1
```

- [ ] Start the local app if frontend verification is requested or if a dev server is already part of the current workflow.
- [ ] Verify manually or with browser automation:
    - Desktop loads without blank windows.
    - Circular menu is visible.
    - Fruity dock scroll arrows appear when apps overflow.
    - App Manager can hide/show dock and start menu entries.
    - File Manager supports two independent windows.
    - Writer has readable icons and a full white writing surface in light mode.
    - Desktop widgets fit their content without inner scrollbars for common built-in widgets.
    - Context menus appear above modals.
    - Looper rejects concurrent runs.
    - Closing apps releases timers/SSE/WebSockets in repeated open/close cycles.
- [ ] Run:

```powershell
git diff --check
git status --short
```

- [ ] Commit any final verification-only fixes:

```powershell
git add <changed-files>
git commit -m "test: verify virtual desktop comprehensive audit fixes"
```

## Suggested Parallelization

- Wave 1 and Wave 2 can be separate backend workstreams after Wave 0.
- Wave 3 should be one coordinated frontend lifecycle workstream because File Manager, app disposal, and render boundaries touch shared shell behavior.
- Wave 5 theme work can run in parallel with Wave 6 i18n after Wave 3 is merged.
- Wave 7 performance should wait until Wave 3 and Wave 4 are merged, because loader and render batching changes amplify lifecycle bugs if done too early.

## Risk Register

- Async module loading is the riskiest optimization because current script chunks may not be syntactically standalone. Preserve ordered concatenation first.
- File Manager per-window state is a large refactor. Keep tests marker-based and manually verify two windows.
- Quill disposal depends on the bundled Quill version. If there is no destroy API, remove owned listeners and DOM references explicitly.
- Theme fixes can accidentally create low-contrast controls. Use visual verification in both standard and Fruity modes.
- Translation updates touch many files. Use a scanner to avoid missing keys or placeholder mismatches.

## Completion Criteria

- All critical report items are either fixed or explicitly marked as already fixed with current-source evidence.
- No backend path traversal or broken-symlink bypass remains in desktop file operations.
- Desktop chat and Looper work honor cancellation and cannot leak unbounded goroutines after disconnects.
- File Manager, Writer, Sheets, Code Studio, Radio, Looper, and Camera have reliable per-window cleanup.
- Context menus, modals, and toasts follow a single z-index scale.
- Standard and Fruity desktops have coherent light and dark theme support.
- Desktop translation key and placeholder tests pass for every language file.
- Synchronous XHR is removed or isolated behind a safe compatibility path with tests.
- `go test ./... -count=1` passes, or every remaining failure is documented as unrelated with exact output and owner.
