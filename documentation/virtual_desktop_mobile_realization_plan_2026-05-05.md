# Virtual Desktop Mobile Implementation Plan - 2026-05-05

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the AuraGo virtual desktop usable on phones and small tablets without regressing the existing desktop window workflow.

**Architecture:** Keep the existing vanilla JS shell and CSS structure. Add touch-specific interaction helpers in the desktop shell and file-manager module, then layer compact-viewport CSS and keyboard-safe viewport handling around the current window manager.

**Tech Stack:** Go embedded UI tests, vanilla JavaScript, CSS media queries, embedded translation JSON.

---

## Context

This plan reviews `reports/virtual_desktop_mobile_plan_2026-05-05.md` against the current codebase and turns it into an implementation sequence. The report itself stays in `reports/` and must not be committed.

Current relevant files:

- Modify `ui/js/desktop/main.js`
- Modify `ui/js/desktop/file-manager.js`
- Modify `ui/css/desktop.css`
- Modify `ui/desktop_mobile_layout_test.go`
- Modify `ui/desktop_file_manager_test.go`
- Modify `ui/lang/desktop/*.json` only if new labels or tooltips are introduced

## Plan Validation

Confirmed current defects:

- Desktop icons still require selection plus double-click. `renderIcons()` wires `click` to `selectDesktopIcon()` and `dblclick` to `activateDesktopItem()`.
- Context menus still depend on `contextmenu` events. Desktop icons, widgets, taskbar buttons, titlebars, and file-manager items have no long-press fallback.
- Desktop icon and widget dragging starts immediately on pointer down. That is fine for mouse, but touch needs hold/drag gating so tap and long-press can coexist.
- File-manager items still require double-click to open and single-click to select. Touch users need first tap to open and long-press for selection plus menu.
- Window dragging and resizing are pointer-based but not compact-viewport aware. CSS already forces windows full width below 820px, but JS still tracks them as normal floating windows.
- Start menu currently focuses search immediately after opening. On mobile this can open the virtual keyboard before the user asks for search.
- `workspaceBoundsForWindow()` only reads the window layer size and does not account for `visualViewport` changes when the virtual keyboard is open.
- Context menu rows are below mobile target size: `.vd-context-item` has `min-height: 34px`, and `.fm-context-item` has compact padding.
- File-manager sidebar is fixed at 180px and has no mobile toggle.

Partially stale or already implemented report points:

- `ui/desktop.html` already has a robust viewport tag with `viewport-fit=cover` and `interactive-widget=resizes-content`.
- `ui/css/desktop.css` already has a `max-width: 820px` rule that makes `.vd-window` full viewport and accounts for `env(safe-area-inset-bottom)`.
- `.vd-icon` and `.vd-widget` already use `touch-action: none`, so do not replace those with `touch-action: manipulation`.
- The report says 15 languages, but the desktop locale set currently has 16 files: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.

Corrections to the source report:

- Use a compact viewport predicate such as `window.matchMedia('(max-width: 820px)').matches` for phone layout. Do not use coarse pointer alone, because tablets and touch laptops can still have enough space for floating windows.
- Do not add visible instructional text such as "Hold to open menu" unless there is a real UI surface for it. Prefer `title` and `aria-label` text for new controls.
- Do not call `toggleMaximizeWindow()` after opening every touch window. CSS already forces compact windows full-screen; JS should instead avoid drag/resize work while compact.
- `ensureInputVisible()` should scroll `.vd-window-content` or the nearest scrollable app content, not `.vd-window`, because windows use `overflow: hidden`.
- Keep browser zoom accessibility intact. Do not add `user-scalable=no`.

## Implementation Decisions

- Touch behavior is added as progressive enhancement. Mouse behavior remains single-click select, double-click open, right-click menu, immediate drag.
- On compact viewports, desktop icon tap opens immediately. Long-press opens the context menu. Dragging starts only after a hold plus movement.
- File-manager touch tap opens or navigates immediately. Long-press selects the item and opens its context menu. Keyboard and mouse selection behavior stays unchanged.
- Compact windows are treated as full-screen by CSS. JS guards prevent titlebar drag and resize while compact, and viewport sync keeps focused fields visible above the virtual keyboard.
- The start menu becomes a bottom sheet on compact viewports, and search is not auto-focused on compact viewports.
- The file-manager sidebar becomes collapsible below 820px and hidden by default below 560px. A toolbar button exposes it.
- Add marker-style Go tests first, matching the existing UI test style in this package.

## Wave 1: Baseline Tests And Touch Primitives

### Task 1: Add mobile interaction regression markers

Files:

- Modify `ui/desktop_mobile_layout_test.go`
- Modify `ui/desktop_file_manager_test.go`

- [ ] Add a test in `ui/desktop_mobile_layout_test.go` that reads `js/desktop/main.js` and asserts these markers:
  - `function isCompactViewport()`
  - `function isTouchLikePointer(event)`
  - `function wireLongPress(element, callback, options)`
  - `function shouldOpenOnTap(event)`
  - `function updateViewportMetrics()`
  - `window.visualViewport`
  - `function ensureFocusedControlVisible(event)`
  - `function wireWindowTouchGestures(win, id)`
- [ ] Add CSS marker assertions in `ui/desktop_mobile_layout_test.go`:
  - `.vd-long-press-active`
  - `overscroll-behavior: none`
  - `.vd-window-titlebar`
  - `touch-action: none;`
  - `max-height: 70dvh`
  - `@media (max-width: 560px)`
  - `.fm-sidebar-toggle`
- [ ] Add a test in `ui/desktop_file_manager_test.go` that reads `js/desktop/file-manager.js` and asserts:
  - `function isTouchLikePointer(event)`
  - `function wireLongPress(element, callback, options)`
  - `function openFileItem(path, type)`
  - `function handleSidebarToggle()`
  - `fm.sidebarOpen`
- [ ] Run `go test ./ui`.
  Expected: the new tests fail before implementation with missing marker messages.

### Task 2: Add shared touch helpers in the desktop shell

Files:

- Modify `ui/js/desktop/main.js`

- [ ] Add `isCompactViewport()` near existing layout helpers:
  - Return `window.matchMedia('(max-width: 820px)').matches`.
- [ ] Add `isTouchLikePointer(event)`:
  - Return true for `event.pointerType === 'touch'` or `event.pointerType === 'pen'`.
  - Fall back to `window.matchMedia('(hover: none) and (pointer: coarse)').matches`.
- [ ] Add `shouldOpenOnTap(event)`:
  - Return true only for touch-like pointer events or compact viewport.
- [ ] Add `wireLongPress(element, callback, options)`:
  - Start a timer on touch-like `pointerdown`.
  - Cancel when movement exceeds 10px.
  - Trigger after 600ms.
  - Add `.vd-long-press-active` only while the press is active.
  - Suppress the next click after a successful long-press.
- [ ] Run `go test ./ui`.
  Expected: desktop helper marker failures are reduced, file-manager markers still fail.

### Task 3: Apply touch helpers to desktop icons, widgets, windows, and taskbar

Files:

- Modify `ui/js/desktop/main.js`

- [ ] In `renderIcons()`, keep existing mouse `click` and `dblclick`, but change click handling so touch or compact tap calls `activateDesktopItem(btn)` and stops the selection-only path.
- [ ] Wire long-press on each `.vd-icon` to call `showIconContextMenu()` using the original pointer coordinates.
- [ ] In `wireDraggableIcon()`, keep immediate drag for mouse. For touch-like pointers, arm drag only after a short hold and movement. If long-press fires first, do not drag.
- [ ] Wire long-press on widgets to call `showWidgetContextMenu()`.
- [ ] Wire long-press on taskbar buttons to call `showWindowContextMenu()`.
- [ ] In `wireWindow()`, skip titlebar drag when `isCompactViewport()` is true.
- [ ] Add `wireWindowTouchGestures(win, id)`:
  - On compact viewport, detect swipe down on `.vd-window-titlebar`.
  - If vertical movement is over 80px and dominates horizontal movement, minimize the window.
  - Do not implement swipe-to-close in this pass.
- [ ] Call `wireWindowTouchGestures(win, id)` from `wireWindow()`.
- [ ] Run `go test ./ui`.
  Expected: main shell interaction markers pass.

## Wave 2: Compact Viewport And Virtual Keyboard Behavior

### Task 4: Make compact windows and viewport metrics robust

Files:

- Modify `ui/js/desktop/main.js`
- Modify `ui/css/desktop.css`

- [ ] Add `updateViewportMetrics()` in `main.js`:
  - Set CSS variable `--vd-visual-height` to `window.visualViewport.height + 'px'` when available.
  - Fall back to `window.innerHeight + 'px'`.
- [ ] Call `updateViewportMetrics()` during init.
- [ ] Bind `visualViewport.resize`, `visualViewport.scroll`, and `window.resize` to update metrics.
- [ ] Update compact `.vd-window` CSS height to use `var(--vd-visual-height, 100dvh)`.
- [ ] Add `ensureFocusedControlVisible(event)`:
  - React to focus on `input`, `textarea`, `select`, and contenteditable controls inside `.vd-window`.
  - Use `window.visualViewport.height` to determine if the control is covered.
  - Scroll the closest `.vd-window-content` or app scroll container by the required offset.
- [ ] Register a document-level `focusin` listener during desktop init.
- [ ] In the start button handler, focus `vd-start-search` only when not compact.
- [ ] Run `go test ./ui`.
  Expected: viewport marker tests pass.

### Task 5: Tighten compact CSS and touch targets

Files:

- Modify `ui/css/desktop.css`

- [ ] Add `overscroll-behavior: none` to `.desktop-body` and `.vd-shell`.
- [ ] Add `touch-action: manipulation` to normal buttons and file-manager items, but keep `touch-action: none` on draggable desktop icons, widgets, and window titlebars.
- [ ] Add `.vd-long-press-active` with a subtle scale or brightness feedback that does not shift layout.
- [ ] In `@media (max-width: 820px)`:
  - Set `.vd-window` border radius to 0.
  - Set `.vd-window-titlebar` to 52px height.
  - Set `.vd-window-button` to 44px by 44px.
  - Hide `.vd-resize-handle`.
  - Make `.vd-start-menu` a bottom sheet with full width and `max-height: 70dvh`.
- [ ] Add `.vd-resize-handle::after { content: ''; position: absolute; inset: -12px; }` outside compact media queries.
- [ ] Set `.vd-context-item` and `.fm-context-item` to at least 44px high.
- [ ] Run `go test ./ui`.
  Expected: CSS marker tests pass.

## Wave 3: File Manager Mobile UX

### Task 6: Add touch tap/open and long-press menu to FileManager

Files:

- Modify `ui/js/desktop/file-manager.js`

- [ ] Add `sidebarOpen: false` to `fm`.
- [ ] Add `isTouchLikePointer(event)` and `wireLongPress(element, callback, options)` in this module because it is standalone and can run without shell helpers.
- [ ] Extract item opening from `handleItemDblClick()` into `openFileItem(path, type)`.
- [ ] In `handleItemClick(e)`, if the event is touch-like, call `openFileItem(path, type)` and return without changing selection.
- [ ] In `attachFileItemEvents(root)`, wire long-press for each item to select it and call `handleItemContextMenu()`.
- [ ] Keep `dblclick`, keyboard Enter, Ctrl/Cmd multi-select, Shift range select, drag/drop, and rename behavior unchanged for mouse and keyboard.
- [ ] Run `go test ./ui`.
  Expected: file-manager JS marker tests pass.

### Task 7: Add mobile sidebar toggle to FileManager

Files:

- Modify `ui/js/desktop/file-manager.js`
- Modify `ui/css/desktop.css`
- Modify `ui/lang/desktop/*.json`

- [ ] Add a toolbar icon button with class `.fm-sidebar-toggle` and action `sidebar-toggle`.
- [ ] Add `handleSidebarToggle()` and call it from `handleActionClick()`.
- [ ] Add `data-sidebar-open` to the file-manager root when `fm.sidebarOpen` is true.
- [ ] In CSS below 820px, make the sidebar overlay or collapse without shrinking file content.
- [ ] In CSS below 560px, hide the sidebar by default and show it only when `[data-sidebar-open="true"]`.
- [ ] Add translation key `desktop.fm.toggle_sidebar` to all 16 `ui/lang/desktop/*.json` files.
- [ ] Run `go test ./ui`.
  Expected: translation lint and file-manager marker tests pass.

## Wave 4: Verification And Commit

### Task 8: Run automated verification

Files:

- No source changes unless failures expose a real issue.

- [ ] Run `go test ./ui`.
- [ ] Run `go test ./...` if the UI package passes.
- [ ] If tests fail, fix the smallest relevant issue and rerun the failing package.

### Task 9: Manual mobile smoke test

Files:

- No source changes unless smoke testing exposes a real issue.

- [ ] Start AuraGo locally using the existing development workflow.
- [ ] Test a phone-width viewport around 390x844:
  - Open the virtual desktop.
  - Tap a desktop app icon and confirm it opens.
  - Long-press a desktop icon and confirm the context menu opens.
  - Open the start menu and confirm the keyboard does not appear automatically.
  - Open Files, tap a folder, and confirm it navigates.
  - Long-press a file-manager item and confirm the context menu opens.
  - Focus an input in a desktop app and confirm it remains visible above the virtual keyboard area.
  - Swipe down on a compact window titlebar and confirm it minimizes.
- [ ] Test a desktop viewport around 1440x900:
  - Single-click selects icons.
  - Double-click opens icons and files.
  - Right-click opens context menus.
  - Window drag and resize still work.

### Task 10: Commit

Files:

- Include only source, tests, translations, and this implementation plan.
- Do not stage `reports/` or `disposable/`.

- [ ] Run `git status --short`.
- [ ] Stage the touched versioned files.
- [ ] Run `git diff --cached`.
- [ ] Commit with:

```bash
git commit -m "feat: improve virtual desktop mobile interactions"
```

## Residual Risks

- Marker tests protect structure, not real pointer-event timing. Manual browser testing is required for long-press, swipe, and keyboard behavior.
- Touch timing can feel different across iOS Safari and Android Chrome. Keep thresholds conservative and easy to tune.
- Generated or embedded app iframes may need their own keyboard handling later; this plan only keeps the host window content visible.
- Compact mode is width-based. Large tablets with narrow split-screen windows will use compact behavior, which is intended.

## Self-Review

- Spec coverage: The reviewed report's critical interaction, compact layout, touch target, browser behavior, accessibility, translation, and testing concerns are covered by Waves 1-4.
- Placeholder scan: Every task has concrete files, actions, commands, and expected results.
- Type consistency: Helper names are stable across tests and implementation tasks.
- Scope: Native app/PWA, tablet split-view, stylus-specific behavior, and `main.js` modularization remain out of scope.
