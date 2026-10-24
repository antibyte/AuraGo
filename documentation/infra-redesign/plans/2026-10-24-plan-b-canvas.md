# Plan B: Infra canvas foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the banded Infra layout with a map-first shell. It has a 44 px command bar, a docked library, a full-bleed pan/zoom canvas with cards, host stacks, regions, connections and a minimap, direct-manipulation gestures with keyboard equivalents, and a store with a coalescing layout queue, one conflict rebase and multi-level undo/redo. Every current safety property stays: CAS, drafts, uncertain outcomes, read-only and policy gating, and aborting fetches on hide.

**Architecture:** Nine vanilla-JS IIFE files in `ui/js/desktop/apps/`, loaded in order by `module-loader.js`. `infra-model.js` holds the pure helpers. Unit tests in `scripts/test-infra-ui.mjs` run them in a Node vm. `infra-store.js` owns state, the API, polling and the layout/undo engine, also tested in the vm against a fake API. The canvas, gestures, library, inspector, table and dialogs modules are each built with `create(app)` around one shared `app` context. The composition root is `infra.js`. Rendering is keyed by id. The scene carries the only transform. Write commands go through `store.dispatch`.

**Tech Stack:** Vanilla ES2020 in IIFEs, no bundler for app files. CSS container queries with theme tokens (`--vd-theme-*`, `--ds-*`). Node `assert` and `vm` for unit tests. Go and rod for the browser tests (`AURAGO_RUN_BROWSER_SMOKE=1`).

---

## 0. Ground rules for every task

**Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-redesign`. In Git Bash this is `/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign`. Every shell command starts with `cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" &&` because the cwd resets between calls. Scratch scripts go in your session scratchpad directory, never in the repo. Below, `$SCRATCH` stands for that absolute path.

**Standard verification (V-core):**
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && node scripts/test-infra-ui.mjs
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && go test ./ui -run '(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopUsedI18nKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix)' -count=1
```
**Browser verification (V-browser):** about 15 min. If Go build space runs out, trim GOCACHE on D: (entries older than 3 days).
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra' -count=1 -timeout 30m
```
**Syntax check for every JS file you created or edited:** `node --check ui/js/desktop/apps/<file>.js`.

**Impact step (before editing an existing function):**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<Symbol>" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers <Symbol>
```
The index is stale, so expect `risk: UNKNOWN`. That is not an all-clear. Confirm with the text search given in the step and report callers and risk in your task notes. If a result says HIGH or CRITICAL, warn before editing.

**Commit step template:**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && git add <only the paths this task touched> && git commit -m "<subject>" -m "<one-paragraph body>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
`partial: true` or `truncated: true` from detect-changes is not a clean result. Re-run it.

**Code conventions:**
- Write `'\x1f'`-style escapes, never `\u` escapes. The Write/Edit tools decode `\u` sequences.
- Every translated string is a literal `t('desktop.infra_…')` call, so both i18n tests can see the key. Placeholders use `.replace('{name}', …)` or `model.fill(text, vars)`.
- Never `t('k', 'fallback')`. Never `t('k') || 'x'`. Never a template-literal key for a new key.
- New files must not touch `document`, `requestAnimationFrame`, `CSS`, `localStorage`, `matchMedia` or `ResizeObserver` at load time. Only `create()`/`attach()` may use them, because the Node test concatenates and runs every file.
- Classes use the `infra-` prefix. Nothing starts with `cc-`. No external URLs. Inline SVG has no `xmlns`.
- JS animation runs only when `document.body.dataset.animations !== 'false'` and `prefers-reduced-motion` is not `reduce`.

**Coordinate contract (new, client-side; Plan D documents it in `documentation/infra.md`):**
- An asset placement `(x, y)` is the centre of its 184×60 card. This matches today's saved data.
- A place placement (site, room, rack, group) `(x, y)` is the top-left corner of its region. `(w, h)` is its size, default 360×240, minimum 200×140.
- The drawn region is the stored rectangle grown to contain its children's rects plus 48 px.

**Plan A dependency guard:**
- The server decodes request bodies with `DisallowUnknownFields`.
- Layout items carry `parent_id`/`w`/`h` only while `store.features.layoutParent` is true. That flag is true when the last snapshot had a `source_hosts` array, which only Plan A sends.
- Nodes are never given `host_id`/`host_inferred` keys the snapshot did not send. Node drafts are PUT back as-is.
- `host_id` is read with `node.host_id || ''`. Without it, `runs_on` edges decide hosting, as today.

**Known limitations Plan B accepts (state them in the review handoff):**
1. **Placing from the library is undoable only against a server that can unplace.** `PUT /layout` items may carry `unplace: true` (with `id` and `version`, no coordinates), which removes that view's placement; the backend plan adds it together with `parent_id`/`w`/`h`, so it is available exactly when `store.features.layoutParent` is true. `placeFromLibrary` then records an undo entry: its `before` holds the asset's previous intent, or `{unplace: true}` when it had no placement in that view. Against an older server the command is dispatched but stays out of history (no API removes a placement). Undoing a drop into a place unplaces the asset but leaves its `parent_id`, because an unplace item carries nothing else. The Task 1 and Task 3 listings below show the first, non-undoable draft; the committed `infra-model.js` and `infra-store.js` are authoritative for `mergeIntent` (an unplace replaces earlier intents and a later placement lifts it), `layoutItems`, `applyLayoutPatch` (a full node without the view drops that placement), `effectivePlacement` (null while an unplace is queued) and `placeFromLibrary`.
2. **Not in Plan B:**
   - Plan C: context menus, the filter chips and popover polish, the full library sections (Cleanup, Ignored, place-row drop targets), inspector tabs, the app-link picker, import relevance groups and the shortcut sheet.
   - Plan D: topology lanes and auto layout.
3. **Guests nest one level.** A guest whose host is itself a guest appears only in that host's shelf.
4. **No visually hidden duplicate list of the canvas.** The library tree and the List view are the accessible lists; cards are labelled buttons inside the `role=application` viewport.

---

## 1. File structure and public APIs

| File | Global | Responsibility |
|---|---|---|
| `ui/js/desktop/apps/infra-model.js` (modify) | `InfraAppModel` | Pure helpers. Existing exports are kept except `sceneDimensions`, `containerFrame` and `atlasDimensions`, which are removed in Task 12 with their tests replaced. New: viewport math, geometry, regions, drop rules, spatial nav, guests, filters, scene builder, layout intents, history, prefs, glyphs. |
| `ui/js/desktop/apps/infra-store.js` (new) | `InfraAppStore` | State, API client, snapshot, polling/visibility, policy, selection, filters, layout queue with rebase, edge writes, undo/redo, commands. |
| `ui/js/desktop/apps/infra-canvas.js` (new) | `InfraAppCanvas` | Canvas DOM. Keyed patching of regions, cards, host stacks, edges. View transform. Minimap. Overlays: empty state, no match, layout chip, tools. Drop and connect highlights. Temporary drag positions. Ghost element. |
| `ui/js/desktop/apps/infra-gestures.js` (new) | `InfraAppGestures` | Pointer, wheel and keyboard on the canvas. Library drags. Marquee. Connect. Region move/resize. Move mode. Arrange. Global shortcuts. |
| `ui/js/desktop/apps/infra-library.js` (new) | `InfraAppLibrary` | Left panel: To place, On the map, Needs a host. Rows, drag sources, `[data-place]`, roving focus, collapse rail, splitter. |
| `ui/js/desktop/apps/infra-inspector.js` (new) | `InfraAppInspector` | The current inspector, ported (forms, drafts, source edit, conflict review). Adds the connection inspector, the multi-selection summary and show/hide on selection. |
| `ui/js/desktop/apps/infra-table.js` (new) | `InfraAppTable` | The current virtualized table, ported. |
| `ui/js/desktop/apps/infra-dialogs.js` (new) | `InfraAppDialogs` | Dialog host, confirm, new asset/place, import, edge dialog, source actions, node actions, delete, connection popover. |
| `ui/js/desktop/apps/infra.js` (rewrite in Task 12) | `InfraApp` | `render(host, windowId, ctx)`, `dispose(windowId)`. Shell markup, command bar, menus/popovers, toasts, announcer, window menus, before-close, subscription routing. |
| `ui/js/desktop/apps/infra-views.js` | — | **Deleted** in Task 12. |
| `ui/css/desktop-app-infra.css` (rewrite in Task 12) | — | Root tokens, shell grid, command bar, menus, library, splitters, inspector, table, dialogs, toasts, responsive rules. |
| `ui/css/desktop-app-infra-canvas.css` (new, Task 5) | — | Canvas, grid, scene, cards, badges, shelves, regions, edges, tools, minimap, chip, empty states, ghost, popover. |
| `ui/js/desktop/core/module-loader.js:27` | — | New script/style list (Task 12). |
| `scripts/test-infra-ui.mjs` | — | File list, vm context (`AbortController`), new pure and store tests, CSS token guard. |
| `ui/desktop_infra_browser_test.go` | — | Fixture: `source_hosts`, layout CAS, parent/size, edge 409/400, `layoutCalls`, `fail409Once`. Main test updated (Task 12). |
| `ui/desktop_infra_canvas_browser_test.go` (new, Task 13) | — | `TestDesktopInfraCanvasGesturesBrowser`. |
| `ui/js/desktop/apps/AGENTS.md` (Infra section) | — | New module boundary rules (Task 12). |
| `ui/lang/desktop/*.json` (16 locales) | — | New `desktop.infra_*` keys in batches A–E. |

### 1.1 Shared `app` context, built by `infra.js` and passed to every `create(app)`

```js
app = {
  ctx, t, esc, windowId, safeId, root, q: selector => root.querySelector(selector),
  model: window.InfraAppModel, store, state: store.state,
  icon: value => `<img src="${esc(model.iconURL(value))}" alt="" loading="lazy" draggable="false">`,
  glyph: name => model.glyph(name),
  labelForKind: kind => t(`desktop.infra_kind_${kind}`),     // existing family keys
  statusLabel: status => t(`desktop.infra_state_${status}`), // existing family keys
  mapView: () => (store.state.view === 'table' ? 'atlas' : store.state.view),
  ensureMapView(),                // switches table -> atlas
  toast(message, kind),           // 'info' | 'success' | 'error'
  announce(text),                 // polite live region, de-duplicated by 30 ms reset
  animationsEnabled(),            // body data-animations !== 'false' && !reduced motion
  // filled in by modules: canvas, gestures, library, inspector, table, dialogs
}
```

### 1.2 Store state shape (`store.state`)

| Group | Fields |
|---|---|
| lifecycle | `disposed, epoch, visible, inView, pollPaused, timer, requests:Set<AbortController>, controller, manifestController, manifestLoaded, manifestLoading` |
| data | `caps:{write,source_edit,control}, nodes[], edges[], sources[], sourceHosts[], candidates[], appTargets[], targetsLoaded, targetsError, credentials[], credentialsLoaded, credentialsLoading, credentialsError, stamp, refreshAt, sync:'syncing'|'live'|'error'|'paused'` |
| ui | `view:'atlas'|'topology'|'table', tool:'select'|'hand'|'connect', filters:{query,kind,source,power}, selection:Set, selected (the single selected node id or ''), selectedEdge, expandedHosts:Set, interacting:number, historyBusy` |
| drafts (owned by inspector/dialogs, unchanged semantics) | `drafts, dirty, draftRevision, nodeReviews, customRows, sourceDrafts, sourceDirty, sourceRevision, sourceReviews, sourceBlocked, sourcePending, actionBlocks, importSelection, importBindings, importDialogOwner, importOpen, dialogGeneration` |
| busy | `busy:Set<string>` (`snapshot`, `layout`, `create`, `import`, `node-<id>`, `source-<id>`, `edge-<id|new>`, `action-…`) |

**LayoutIntent** = `{x, y, pinned?, parent_id?, w?, h?}`.
- An absent `parent_id` means unchanged; `''` clears it.
- Each view lane is `{pending:Map, inflight:Map|null, unsaved:Map, waiters:[], scheduled}`.

### 1.3 Store API (`InfraAppStore.create({ctx, t})` returns `store`)

`state, features:{layoutParent}, subscribe(fn)→unsubscribe, notify(message, kind), setBusy(key, on), request(path, options, epoch), messageFor(error), uncertainMutation(error), isAccessError(error), canWrite(), canEditSource(), canControl(), allowedAction(node, action, source), actionReason(node, source, action), actionBlockKey(nodeId, source, action), nodeById(id), nodeIndex(), childrenOf(id), guests()→{hostOf, guestsOf}, searchableText(node), matches(node), applySnapshot(data, {explicit}), loadSnapshot(explicit), loadTargets(force), loadCredentials(), loadIconManifest(), replaceNode(node), removeNode(id), select(ids, {toggle, additive}), selectEdge(id), clearSelection(), setView(view), setFilters(partial), setTool(tool), toggleHost(id), beginInteraction(), endInteraction(), effectivePlacement(id, view), effectiveParent(id, view), currentIntent(id, view, withParent), layoutState(id, view)→''|'saving'|'unsaved', unsavedCount(view), queueLayout(view, intents)→Promise<boolean>, whenLayoutIdle(view), retryLayout(view), createEdge(fields), updateEdge(id, body), deleteEdgeById(id), dispatch(command)→Promise<boolean>, undo(view), redo(view), canUndo(view), canRedo(view), hasUnsavedWork(), attach({root, win}), abortRequests(), dispose()`.

**Change events** (`subscribe(fn)` receives one object):

| type | payload |
|---|---|
| `snapshot` | `structural, explicit` |
| `layout` | `view, ids?` |
| `layout-status` | `view` |
| `selection` | — |
| `view` | — |
| `filters` | — |
| `tool` | — |
| `expanded` | — |
| `edges` | — |
| `nodes` | — |
| `busy` | `key` |
| `caps` | — |
| `history` | — |
| `notice` | `message, kind` |
| `sync` | `announce` |
| `targets` | — |
| `credentials` | — |
| `manifest` | — |
| `interaction-end` | — |

**Commands** (`InfraAppStore.commands`):

| Factory | Shape | apply | revert |
|---|---|---|---|
| `moveNodes(view, before, after, label)` / `reparent(...)` / `resizePlace(...)` | `{type, view, before:Map<id,Intent>, after:Map<id,Intent>, label, optimistic:true, undoable:true}` | `store.queueLayout(view, after)` | `store.queueLayout(view, before)` |
| `placeFromLibrary(view, after, label)` | same shape; `dispatch` calls its `prepare(store)`, which fills `before` (previous intent, or `{unplace: true}`) and sets `undoable = store.features.layoutParent` | queue `after` | queue `before` (unplace intents) |
| `createEdge(fields)` | `{type:'CreateEdge', fields, edgeId, undoable:true}` | POST, remember `edgeId` | DELETE `edgeId` |
| `deleteEdge(edge)` | `{type:'DeleteEdge', fields, edgeId, undoable:true}` | DELETE | POST `fields` again, remember the new id |

`dispatch` handles the two kinds differently:
- Optimistic (layout) commands are pushed to `histories[view]` immediately.
- Edge commands are pushed after `apply()` resolves `true`.

History depth is 50 per view. A layout 409 clears that view's history.

### 1.4 Module APIs

- `InfraAppCanvas.create(app)` returns:
  - Elements and rendering: `el, viewport, render({force}), renderNow(), sceneData()→{cards, cardById, regions, regionRects, regionDepth, lines, bounds}`.
  - View: `getView(), setView(view, {animate}), fit({animate}), zoomBy(factor, screenPoint?), zoomTo(zoom), centerOn(id, {animate}), ensureVisible(id), clientToWorld(x, y), size()`.
  - Element lookup and live drag: `cardEl(id), regionEl(id), moveTemp(cardPositions:Map, regionRects:Map), clearTemp()`.
  - Highlights and overlays: `setDropTarget(regionId), setConnectTarget(id), showEdgePreview(fromId, point|null), showMarquee(rect|null), createGhost(ids)→{el, move(clientX, clientY, {scale, label, invalid}), remove()}`.
  - Focus and tools: `setRovingCard(id), applyTool(), dispose()`.
- `InfraAppGestures.create(app)` returns `attach(), cancel(), startLibraryDrag(event, ids), placeAtCenter(ids, {moveMode}), enterMoveMode(ids), arrange(), onRootKeydown(event), dispose()`.
- `InfraAppLibrary.create(app)` returns `el, render(), renderSelection(), reveal(id), dispose()`.
- `InfraAppInspector.create(app)` returns `el, paint(), paintIfIdle(), flushStale(), onBusy(key), saveDraft(), focusName(), close({restoreFocus}), dispose()`.
- `InfraAppTable.create(app)` returns `el, render(), dispose()`.
- `InfraAppDialogs.create(app)` returns:
  - Dialogs: `openDialog(title, html, onRender), confirmAction(title, message, opts), openNodeDialog(kind, {title}), openImport(), openEdgeDialog(edgeId, fromId, toId)`.
  - Node and action operations: `deleteNode(id), deleteNodes(ids), openActionMenu(id, source), runAction(id, source, action, dialog), openNodeActions(id)`.
  - Connection popover: `openConnectionPopover({from, to, clientX, clientY}), closeConnectionPopover()`.
  - State: `onBusy(key), ownsDialog(owner), closeOwnedDialog(owner), dispose()`.

### 1.5 Test-pinned DOM hooks

**Kept:**
- `[data-infra-app]`, `[data-search]`.
- `[data-view=atlas|topology|table][aria-pressed]`.
- `[data-stage]`: the `role=application` viewport.
- `[data-scene]`: now `translate(x,y) scale(z)`.
- `[data-nodes]`, `[data-node]` (card `<button>`, `style.left/top` in world px).
- `[data-edges]` (`viewBox="0 0 1 1"`), `[data-visible-count]`, `[data-zoom]`.
- `[data-action=refresh|zoom-in|zoom-out|zoom-reset|arrange|import|add|add-empty|import-empty|close-inspector|add-edge|save-source|review-source|reapply-source-draft|retry-layout]`.
- `[data-inspector]`, `[data-node-field]`, `[data-detail-form]`, `[data-list-add]`, `[data-list-row]`.
- `[data-toggle-host-services]` (now the card badge), `[data-hosted-service]` (now a shelf chip).
- `[data-table-view|data-table-scroll|data-table-body|data-table-total]`, `[data-row]`, `[data-select]`.
- `[data-candidate]`, `[data-import]`, `[data-bind]`, `[data-dialog]`, `[data-new-node]`, `[data-edge-form]`.
- `[data-edit-source]`, `[data-source-field]`, `[data-source-form]`, `[data-source-action]`, `[data-run-action]`, `[data-action-rearm]`, `[data-app-target][data-app]`.
- `.infra-draft-banner`, `.infra-source-row`, `.infra-source-edit`, `.infra-relation-row`, `.infra-live-status(.is-error)`, `[data-empty]`, `[data-place]`.

**New:**
- `[data-library-item]`, `[data-region]`, `[data-region-header]`, `[data-resize]`, `[data-connect-handle]`, `[data-guest-shelf]`, `[data-connect-popover]`, `[data-connect-kind]`, `[data-layout-chip]`.
- `[data-action=toggle-library|add-menu|add-place|fit|redo|undo|filters|toggle-minimap|create-connection|clear-search|place-selected]`.
- `[data-live][data-refreshed-at]`, `[data-tool]`.

---

## Task 0: Baseline (no commit)

**Files:** none.

- [ ] **Step 1: Run V-core on the unmodified branch** and record the result.
- [ ] **Step 2: Run V-browser on the unmodified branch.** Project memory says `TestDesktopInfraAppBrowser` already fails on main.
  - Record the failing step (assertion text and line) in your task notes.
  - Task 12 rewrites most of this test; its final version must pass.
  - If the same pre-existing failure survives into Task 12, hand it to debug-agent. Do not skip it.
- [ ] **Step 3: Confirm the only references to `infra-views` and `desktop-app-infra.css`.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && git grep -n -e "infra-views" -e "desktop-app-infra.css" -- ':!documentation'
  ```
  Expected: `scripts/test-infra-ui.mjs:8`, `ui/js/desktop/core/module-loader.js:27`, and the comment in `infra-model.js:1`.

---

## Task 1: Model helpers (pure) and their tests

**Files:**
- Modify: `ui/js/desktop/apps/infra-model.js`
- Modify: `scripts/test-infra-ui.mjs`

- [ ] **Step 1: Impact check** for `applyLayoutPatch`, `edgeSegment` and `snapshotStamp`, using the impact template.
  - Confirm with `git grep -n -e applyLayoutPatch -e edgeSegment -e snapshotStamp -- ui scripts`.
  - Expected callers: `infra.js` (persistLayout, loadSnapshot), `infra-views.js` (paintStage), the test.
  - All changes are backward compatible for those callers. `edgeSegment` now clips at the 184×60 card border instead of 78 px, which only changes how the old UI draws.

- [ ] **Step 2: Add the new helpers.** Insert this block into `infra-model.js` directly after `const snapshotStamp = …;` (before `window.InfraAppModel = {`). Change the first-line comment to `// Pure Infra helpers. Loaded first; every other infra-*.js file reads window.InfraAppModel.`

```js
    // ---- canvas world -------------------------------------------------------
    // World coordinates are CSS pixels at zoom 1. An asset placement (x, y) is
    // the centre of its 184×60 card; a place placement (x, y) is the top-left
    // corner of its region and (w, h) its size (default 360×240).
    const CARD_W = 184, CARD_H = 60, GRID = 24, MIN_ZOOM = 0.25, MAX_ZOOM = 2;
    const REGION_W = 360, REGION_H = 240, REGION_PAD = 48, REGION_MIN_W = 200, REGION_MIN_H = 140;
    const CARD_HALF = { w: CARD_W / 2, h: CARD_H / 2 };
    const HOSTED_KINDS = new Set(['vm', 'container', 'service']);
    const WIRELESS_ICONS = new Set(['access-point', 'mesh-node']);
    const finite = value => (value === null || value === undefined || value === '' || !Number.isFinite(Number(value)) ? null : Number(value));
    const clampZoom = zoom => (typeof zoom === 'number' && Number.isFinite(zoom) ? Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom)) : 1);
    const toWorld = (view, p) => ({ x: (p.x - view.x) / view.zoom, y: (p.y - view.y) / view.zoom });
    const toScreen = (view, p) => ({ x: p.x * view.zoom + view.x, y: p.y * view.zoom + view.y });
    // zoomAt changes the zoom and keeps the world point under screen fixed.
    const zoomAt = (view, zoom, screen) => {
        const z = clampZoom(zoom);
        const wx = (screen.x - view.x) / view.zoom, wy = (screen.y - view.y) / view.zoom;
        return { x: screen.x - wx * z, y: screen.y - wy * z, zoom: z };
    };
    // fitView shows box inside size with padding, never above maxZoom.
    const fitView = (box, size, padding = 48, maxZoom = 1) => {
        if (!box) return { x: size.w / 2, y: size.h / 2, zoom: 1 };
        const zoom = clampZoom(Math.min((size.w - padding * 2) / Math.max(box.w, 1), (size.h - padding * 2) / Math.max(box.h, 1), maxZoom));
        return { x: (size.w - box.w * zoom) / 2 - box.x * zoom, y: (size.h - box.h * zoom) / 2 - box.y * zoom, zoom };
    };
    // A stored viewport is its world centre and zoom, so it suits any window size.
    const viewCenter = (view, size) => ({ cx: (size.w / 2 - view.x) / view.zoom, cy: (size.h / 2 - view.y) / view.zoom, zoom: view.zoom });
    const viewFromCenter = (stored, size) => {
        if (!stored || typeof stored !== 'object' || !Number.isFinite(stored.cx) || !Number.isFinite(stored.cy)) return null;
        const zoom = clampZoom(stored.zoom);
        return { x: size.w / 2 - stored.cx * zoom, y: size.h / 2 - stored.cy * zoom, zoom };
    };
    const snap = (value, grid = GRID) => Math.round(value / grid) * grid;
    const cardRect = pos => ({ x: Number(pos.x) - CARD_W / 2, y: Number(pos.y) - CARD_H / 2, w: CARD_W, h: CARD_H });
    const intersects = (a, b) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
    const containsPoint = (r, p) => p.x >= r.x && p.x <= r.x + r.w && p.y >= r.y && p.y <= r.y + r.h;
    const normalizeRect = (a, b) => ({ x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(b.x - a.x), h: Math.abs(b.y - a.y) });
    const padRect = (r, pad) => ({ x: r.x - pad, y: r.y - pad, w: r.w + pad * 2, h: r.h + pad * 2 });
    const unionRects = rects => {
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (const r of rects || []) {
            if (!r) continue;
            minX = Math.min(minX, r.x); minY = Math.min(minY, r.y);
            maxX = Math.max(maxX, r.x + r.w); maxY = Math.max(maxY, r.y + r.h);
        }
        return minX === Infinity ? null : { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
    };
    // freeSlot returns want when the card fits there, else the grid point of the
    // nearest ring around it whose card (plus gap) overlaps no obstacle rect.
    const freeSlot = (want, obstacles, { grid = GRID, gap = 12, maxRing = 40 } = {}) => {
        const reach = (maxRing + 2) * grid + CARD_W;
        const near = (obstacles || []).filter(o => Math.abs(o.x + o.w / 2 - want.x) < reach + o.w / 2 && Math.abs(o.y + o.h / 2 - want.y) < reach + o.h / 2).map(o => padRect(o, gap / 2));
        const blocked = point => { const rect = padRect(cardRect(point), gap / 2); return near.some(o => intersects(rect, o)); };
        if (!blocked(want)) return { x: want.x, y: want.y };
        const base = { x: snap(want.x, grid), y: snap(want.y, grid) };
        for (let ring = 0; ring <= maxRing; ring += 1) {
            let best = null, bestDistance = Infinity;
            for (let dy = -ring; dy <= ring; dy += 1) for (let dx = -ring; dx <= ring; dx += 1) {
                if (Math.max(Math.abs(dx), Math.abs(dy)) !== ring) continue;
                const point = { x: base.x + dx * grid, y: base.y + dy * grid };
                if (blocked(point)) continue;
                const distance = Math.hypot(point.x - want.x, point.y - want.y);
                if (distance < bestDistance) { best = point; bestDistance = distance; }
            }
            if (best) return best;
        }
        return { x: want.x, y: want.y };
    };
    // regionRect is the drawn rectangle of a place: its stored rectangle grown to
    // contain childRects plus the 48 px frame padding.
    const regionRect = (placement, childRects = []) => {
        if (!placement || finite(placement.x) == null || finite(placement.y) == null) return null;
        const base = { x: Number(placement.x), y: Number(placement.y), w: finite(placement.w) || REGION_W, h: finite(placement.h) || REGION_H };
        return unionRects([base, ...childRects.filter(Boolean).map(r => padRect(r, REGION_PAD))]);
    };
    // regionLayout draws placed places bottom-up so nested places grow their parents.
    const regionLayout = (placeIds, { parentOf, placementOf, cardRectsByParent = new Map() }) => {
        const ids = new Set(placeIds), rects = new Map(), depth = new Map(), visiting = new Set(), nested = new Map();
        for (const id of ids) {
            const parent = parentOf(id);
            if (parent !== id && ids.has(parent)) { const list = nested.get(parent) || []; list.push(id); nested.set(parent, list); }
        }
        const depthOf = id => {
            let level = 0;
            const seen = new Set([id]);
            for (let parent = parentOf(id); ids.has(parent) && !seen.has(parent); parent = parentOf(parent)) { seen.add(parent); level += 1; }
            return level;
        };
        const visit = id => {
            if (rects.has(id)) return rects.get(id);
            if (visiting.has(id)) return null;
            visiting.add(id);
            const inner = (nested.get(id) || []).map(visit).filter(Boolean);
            const rect = regionRect(placementOf(id), [...(cardRectsByParent.get(id) || []), ...inner]);
            visiting.delete(id);
            if (rect) { rects.set(id, rect); depth.set(id, depthOf(id)); }
            return rect;
        };
        for (const id of ids) visit(id);
        return { rects, depth };
    };
    // regionAt returns the innermost region containing point: deepest first,
    // then the smallest area. exclude holds ids that cannot be targets.
    const regionAt = (point, rects, depth, exclude = new Set()) => {
        let best = '', bestDepth = -1, bestArea = Infinity;
        for (const [id, r] of rects) {
            if (exclude.has(id) || !containsPoint(r, point)) continue;
            const level = depth.get(id) || 0, area = r.w * r.h;
            if (level > bestDepth || (level === bestDepth && area < bestArea)) { best = id; bestDepth = level; bestArea = area; }
        }
        return best;
    };
    // dropParent says what a drop does to parent_id: {} when it stays, else
    // {parent_id}. A parent that is not drawn in this view is never cleared.
    const dropParent = (current, point, rects, depth, exclude = new Set()) => {
        const target = regionAt(point, rects, depth, exclude);
        const parent = current || '';
        if (target === parent) return {};
        if (!target && parent && !rects.has(parent)) return {};
        return { parent_id: target };
    };
    // growRegions returns stored rectangles that must grow so region id (and its
    // ancestors) contain rect plus REGION_PAD.
    const growRegions = (id, rect, { ids, parentOf, placementOf }) => {
        const changes = new Map(), seen = new Set();
        let target = rect, current = id;
        while (current && ids.has(current) && !seen.has(current)) {
            seen.add(current);
            const p = placementOf(current);
            if (!p) break;
            const base = { x: Number(p.x), y: Number(p.y), w: finite(p.w) || REGION_W, h: finite(p.h) || REGION_H };
            const grown = unionRects([base, padRect(target, REGION_PAD)]);
            if (grown.x === base.x && grown.y === base.y && grown.w === base.w && grown.h === base.h) break;
            changes.set(current, grown);
            target = grown;
            current = parentOf(current);
        }
        return changes;
    };
    // spatialNeighbour picks the nearest item in direction dir (left/right/up/down);
    // the off-axis distance counts twice.
    const spatialNeighbour = (from, items, dir) => {
        const axis = dir === 'left' || dir === 'right' ? 'x' : 'y', other = axis === 'x' ? 'y' : 'x';
        const sign = dir === 'right' || dir === 'down' ? 1 : -1;
        let best = '', bestScore = Infinity;
        for (const item of items || []) {
            if (item.id === from.id) continue;
            const primary = (item[axis] - from[axis]) * sign;
            if (primary <= 0) continue;
            const score = primary + Math.abs(item[other] - from[other]) * 2;
            if (score < bestScore || (score === bestScore && String(item.id) < String(best))) { best = item.id; bestScore = score; }
        }
        return best;
    };
    // compactGrid lays count cards out around centre in rows of ceil(sqrt(count)),
    // 216×96 apart (both multiples of the 24 px grid).
    const compactGrid = (count, centre) => {
        const cols = Math.max(1, Math.ceil(Math.sqrt(count))), rows = Math.ceil(count / cols);
        const cellW = CARD_W + 32, cellH = CARD_H + 36;
        const x0 = snap(centre.x - (cols - 1) * cellW / 2), y0 = snap(centre.y - (rows - 1) * cellH / 2);
        return Array.from({ length: count }, (_, i) => ({ x: x0 + (i % cols) * cellW, y: y0 + Math.floor(i / cols) * cellH }));
    };
    const defaultEdgeKind = (a, b) => {
        if ([a, b].some(node => WIRELESS_ICONS.has(node?.icon))) return 'wifi';
        if ([a, b].some(node => node?.kind === 'service')) return 'depends_on';
        return 'ethernet';
    };
    // guestIndex: a vm, container or service belongs to the node named by
    // host_id (server projection, Plan A) or else to the first target of its
    // runs_on edges. Mutual hosting is broken in favour of the smaller id.
    const guestIndex = (nodes, edges) => {
        const byId = new Map((nodes || []).map(node => [node.id, node]));
        const runsOn = new Map();
        for (const edge of edges || []) {
            if (edge.kind !== 'runs_on' || edge.from === edge.to || !byId.has(edge.to)) continue;
            const list = runsOn.get(edge.from) || []; list.push(edge.to); runsOn.set(edge.from, list);
        }
        const hostOf = new Map(), guestsOf = new Map();
        for (const node of nodes || []) {
            if (!HOSTED_KINDS.has(node.kind)) continue;
            const declared = typeof node.host_id === 'string' && node.host_id !== node.id && byId.has(node.host_id) ? node.host_id : '';
            const host = declared || (runsOn.get(node.id) || []).slice().sort()[0] || '';
            if (host) hostOf.set(node.id, host);
        }
        for (const [guest, host] of [...hostOf]) if (hostOf.get(host) === guest && guest > host) hostOf.delete(guest);
        for (const [guest, host] of hostOf) { const list = guestsOf.get(host) || []; list.push(guest); guestsOf.set(host, list); }
        return { hostOf, guestsOf };
    };
    // matchesFilters is the single filter predicate (search text, type, source, power).
    const matchesFilters = (node, filters = {}, textOf = () => '') => {
        if (filters.kind && node.kind !== filters.kind) return false;
        if (filters.source && !(node.sources || []).some(source => source.kind === filters.source)) return false;
        if (filters.power === 'none' && node.power) return false;
        if (filters.power && filters.power !== 'none' && node.power !== filters.power) return false;
        const query = String(filters.query || '').trim().toLocaleLowerCase();
        return !query || textOf(node).includes(query);
    };
    const filtersActive = filters => !!(filters && (filters.kind || filters.source || filters.power || String(filters.query || '').trim()));
    // buildScene computes what the canvas draws for one view: asset cards (a
    // guest rides in its host's card instead), regions for placed places (Map
    // view only) and connection lines between shape borders. runs_on lines that
    // repeat the nesting are left out; a line ending at a guest attaches to the host.
    const buildScene = ({ nodes, edges, view, placementOf, parentOf, guests }) => {
        const atlas = view === 'atlas';
        const cards = [], placeIds = [];
        for (const node of nodes || []) {
            if (guests.hostOf.has(node.id)) continue;
            const p = placementOf(node.id);
            if (!p || finite(p.x) == null || finite(p.y) == null) continue;
            if (isContainer(node.kind)) { if (atlas) placeIds.push(node.id); continue; }
            cards.push({ id: node.id, x: Number(p.x), y: Number(p.y), guests: guests.guestsOf.get(node.id) || [] });
        }
        const cardById = new Map(cards.map(card => [card.id, card]));
        const cardRectsByParent = new Map();
        if (atlas) for (const card of cards) {
            const parent = parentOf(card.id);
            if (!parent) continue;
            const list = cardRectsByParent.get(parent) || []; list.push(cardRect(card)); cardRectsByParent.set(parent, list);
        }
        const { rects, depth } = atlas ? regionLayout(placeIds, { parentOf, placementOf, cardRectsByParent }) : { rects: new Map(), depth: new Map() };
        const endpoint = id => (cardById.has(id) || rects.has(id) ? id : (cardById.has(guests.hostOf.get(id)) ? guests.hostOf.get(id) : ''));
        const shape = id => {
            const card = cardById.get(id);
            if (card) return { x: card.x, y: card.y, half: CARD_HALF };
            const r = rects.get(id);
            return { x: r.x + r.w / 2, y: r.y + r.h / 2, half: { w: r.w / 2, h: r.h / 2 } };
        };
        const lines = [];
        for (const edge of edges || []) {
            if (edge.kind === 'runs_on' && guests.hostOf.get(edge.from) === edge.to) continue;
            const from = endpoint(edge.from), to = endpoint(edge.to);
            if (!from || !to || from === to) continue;
            const a = shape(from), b = shape(to);
            lines.push({ id: edge.id, kind: edge.kind, from, to, ...edgeSegment(a, b, a.half, b.half) });
        }
        const regions = [...rects].map(([id, rect]) => ({ id, rect, depth: depth.get(id) || 0 })).sort((a, b) => a.depth - b.depth || String(a.id).localeCompare(String(b.id)));
        return { cards, cardById, regions, regionRects: rects, regionDepth: depth, lines, bounds: unionRects([...cards.map(cardRect), ...rects.values()]) };
    };
    // ---- layout intents and history -----------------------------------------
    const mergeIntent = (earlier, later) => ({ ...(earlier || {}), ...(later || {}) });
    // layoutItems builds PUT /layout items with the node versions of nodesById.
    // parent_id and w/h go out only when the server supports them (parent).
    const layoutItems = (intents, nodesById, { parent = false, view = 'atlas' } = {}) => {
        const items = [], missing = [];
        for (const [id, intent] of intents) {
            const node = nodesById.get(id);
            if (!node) { missing.push(id); continue; }
            const item = { id, version: node.version, x: Math.round(Number(intent.x)), y: Math.round(Number(intent.y)), pinned: intent.pinned !== false };
            if (parent && intent.parent_id !== undefined) item.parent_id = String(intent.parent_id);
            if (parent && isContainer(node.kind)) {
                const stored = node.placements?.[view] || {};
                const w = finite(intent.w) ?? finite(stored.w), h = finite(intent.h) ?? finite(stored.h);
                if (w != null && h != null) { item.w = Math.round(w); item.h = Math.round(h); }
            }
            items.push(item);
        }
        return { items, missing };
    };
    // rebaseLayout keeps a conflicted batch for nodes that still exist; a parent
    // that vanished or is no longer a place is dropped from the intent.
    const rebaseLayout = (intents, nodesById) => {
        const kept = new Map(), dropped = [];
        for (const [id, intent] of intents) {
            if (!nodesById.has(id)) { dropped.push(id); continue; }
            const next = { ...intent };
            if (next.parent_id) { const parent = nodesById.get(next.parent_id); if (!parent || !isContainer(parent.kind)) delete next.parent_id; }
            kept.set(id, next);
        }
        return { kept, dropped };
    };
    const createHistory = (limit = 50) => {
        let undo = [], redo = [];
        const cap = list => (list.length > limit ? list.slice(list.length - limit) : list);
        return {
            push(entry) { undo = cap([...undo, entry]); redo = []; },
            pushUndo(entry) { undo = cap([...undo, entry]); },
            pushRedo(entry) { redo = cap([...redo, entry]); },
            takeUndo: () => undo.pop() ?? null,
            takeRedo: () => redo.pop() ?? null,
            canUndo: () => undo.length > 0,
            canRedo: () => redo.length > 0,
            clear() { undo = []; redo = []; },
        };
    };
    // ---- small utilities -----------------------------------------------------
    const clampNumber = (value, min, max, fallback) => { const n = Number(value); return value !== null && value !== '' && Number.isFinite(n) ? Math.min(max, Math.max(min, n)) : fallback; };
    const readLayoutPrefs = raw => {
        let data = {};
        try { data = raw ? JSON.parse(raw) : {}; } catch (_) { data = {}; }
        if (!data || typeof data !== 'object') data = {};
        return { libraryWidth: clampNumber(data.libraryWidth, 220, 400, 264), libraryCollapsed: data.libraryCollapsed === true, inspectorWidth: clampNumber(data.inspectorWidth, 320, 520, 360), minimap: data.minimap !== false };
    };
    const fill = (text, vars = {}) => Object.entries(vars).reduce((out, [key, value]) => out.replaceAll(`{${key}}`, String(value)), String(text));
    // normalizeNode fills UI defaults. It never adds fields the snapshot did not
    // send: node PUT bodies are clones of these objects and the server rejects
    // unknown fields.
    const normalizeNode = node => ({ ...node, icon: iconId(node.icon), parent_id: node.parent_id || '', profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] });
    const minimapTransform = (bounds, size, pad = 6) => {
        if (!bounds) return { scale: 1, x: 0, y: 0 };
        const scale = Math.min((size.w - pad * 2) / Math.max(bounds.w, 1), (size.h - pad * 2) / Math.max(bounds.h, 1));
        return { scale, x: (size.w - bounds.w * scale) / 2 - bounds.x * scale, y: (size.h - bounds.h * scale) / 2 - bounds.y * scale };
    };
    const GLYPHS = {
        select: '<path d="M3.5 2.5l9 5.2-4.1 1.1-1.9 3.9z" fill="currentColor" stroke="none"/>',
        hand: '<path d="M5.5 8.5V4.2a1 1 0 0 1 2 0v3.3M7.5 7V3.2a1 1 0 0 1 2 0V7m0-.3V4a1 1 0 0 1 2 0v4.2m0-1.6a1 1 0 0 1 2 0v2.6a4.8 4.8 0 0 1-4.8 4.8H8.4a4 4 0 0 1-3.2-1.6L3 9.6a1 1 0 0 1 1.5-1.2l1 1"/>',
        connect: '<circle cx="4" cy="4" r="2"/><circle cx="12" cy="12" r="2"/><path d="M5.6 5.6l4.8 4.8"/>',
        minus: '<path d="M3 8h10"/>', plus: '<path d="M8 3v10M3 8h10"/>',
        fit: '<path d="M2.5 6V2.5H6M10 2.5h3.5V6M13.5 10v3.5H10M6 13.5H2.5V10"/>',
        undo: '<path d="M5.5 3L2.5 6l3 3"/><path d="M2.5 6h7a3.5 3.5 0 0 1 0 7H6"/>',
        redo: '<path d="M10.5 3l3 3-3 3"/><path d="M13.5 6h-7a3.5 3.5 0 0 0 0 7H10"/>',
        library: '<rect x="2" y="3" width="12" height="10" rx="1.5"/><path d="M6 3v10"/>',
        more: '<circle cx="3.5" cy="8" r="1.1" fill="currentColor"/><circle cx="8" cy="8" r="1.1" fill="currentColor"/><circle cx="12.5" cy="8" r="1.1" fill="currentColor"/>',
        search: '<circle cx="7" cy="7" r="4.5"/><path d="M10.4 10.4L14 14"/>',
        filter: '<path d="M2.5 3h11l-4.2 5.2V13l-2.6-1.3V8.2z"/>',
        minimap: '<rect x="2" y="2" width="12" height="12" rx="1.5"/><rect x="8" y="8" width="4" height="4"/>',
        chevron: '<path d="M4.5 6.5L8 10l3.5-3.5"/>',
        close: '<path d="M4 4l8 8M12 4l-8 8"/>',
    };
    const glyph = name => `<svg class="infra-glyph" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">${GLYPHS[name] || ''}</svg>`;
```

- [ ] **Step 3: Replace `edgeSegment`** (keep `edgeCaption` unchanged):

```js
    // edgeSegment joins two shapes at their borders: each end leaves its centre
    // towards the other and stops at its half-size rectangle. Overlapping shapes
    // keep the centre-to-centre line.
    const borderPoint = (from, to, half) => {
        const dx = to.x - from.x, dy = to.y - from.y;
        if (!dx && !dy) return { x: from.x, y: from.y };
        const scale = Math.min(dx ? half.w / Math.abs(dx) : Infinity, dy ? half.h / Math.abs(dy) : Infinity);
        return { x: from.x + dx * scale, y: from.y + dy * scale };
    };
    const edgeSegment = (a, b, halfA = { w: 92, h: 30 }, halfB = { w: 92, h: 30 }) => {
        const x1 = Number(a.x), y1 = Number(a.y), x2 = Number(b.x), y2 = Number(b.y);
        const start = borderPoint({ x: x1, y: y1 }, { x: x2, y: y2 }, halfA);
        const end = borderPoint({ x: x2, y: y2 }, { x: x1, y: y1 }, halfB);
        if ((end.x - start.x) * (x2 - x1) + (end.y - start.y) * (y2 - y1) <= 0) return { x1, y1, x2, y2 };
        return { x1: start.x, y1: start.y, x2: end.x, y2: end.y };
    };
```

Move the `edgeSegment` definition above `buildScene` because `buildScene` uses it; arrow `const`s must be defined before first call at runtime, and definition order inside the IIFE is enough.

- [ ] **Step 4: Extend `applyLayoutPatch` and `snapshotStamp`.**
  - In `applyLayoutPatch`, after the `node.placements[view] = …` line, add:
    ```js
            // A full node (it carries placements) states its parent; the server
            // omits an empty parent_id, so absence means "no parent".
            if (row.placements && typeof row.placements === 'object') node.parent_id = row.parent_id || '';
    ```
  - In `snapshotStamp`'s `nodePart`, replace `node.parent_id || ''` with `node.parent_id || '', node.host_id || ''`.

- [ ] **Step 5: Export** everything new. Replace the export object with:

```js
    window.InfraAppModel = {
        esc, clone, iconId, iconURL, assetURL, iconLabelKey, sourceActions, sourceIdentity, virtualSlice, sourceState,
        visibleByCollapsed, isPlaced, atlasDimensions, sceneDimensions, validCIDR, validHTTPLink, iconIds: () => [...ICON_LIST],
        acceptsIcon, rememberIcon, versionedAsset, containerKinds: () => [...CONTAINER_KINDS], isContainer, powerLabel,
        containerFrame, edgeSegment, edgeCaption, gridPositions, topologyPositions, snapshotStamp,
        layoutSnapshot, applyLayoutPatch,
        CARD_W, CARD_H, GRID, MIN_ZOOM, MAX_ZOOM, REGION_W, REGION_H, REGION_PAD, REGION_MIN_W, REGION_MIN_H,
        clampZoom, toWorld, toScreen, zoomAt, fitView, viewCenter, viewFromCenter, snap, cardRect, intersects, containsPoint,
        normalizeRect, padRect, unionRects, freeSlot, regionRect, regionLayout, regionAt, dropParent, growRegions,
        spatialNeighbour, compactGrid, defaultEdgeKind, guestIndex, matchesFilters, filtersActive, buildScene,
        mergeIntent, layoutItems, rebaseLayout, createHistory, readLayoutPrefs, fill, normalizeNode, minimapTransform, glyph,
    };
```

- [ ] **Step 6: Update the tests.** In `scripts/test-infra-ui.mjs`:
  - Change the vm context to `{ window, console, setTimeout, clearTimeout, DOMException, TextEncoder, Map, Set, JSON, Math, Date, URL, AbortController }`.
  - Add `const plain = value => JSON.parse(JSON.stringify(value));` after `const model = window.InfraAppModel;`. Objects from the vm have a foreign prototype, so `assert.deepEqual` needs plain copies.
  - Replace the edge-segment block (current lines 133–142) with:

```js
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 400, y: 0 })), { x1: 92, y1: 0, x2: 308, y2: 0 }, 'connections start and end at the card border');
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 0, y: 300 })), { x1: 0, y1: 30, x2: 0, y2: 270 });
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 100, y: 0 })), { x1: 0, y1: 0, x2: 100, y2: 0 }, 'overlapping cards keep the centre line');
assert.equal(model.edgeSegment({ x: 0, y: 0 }, { x: 1000, y: 0 }, undefined, { w: 300, h: 100 }).x2, 700, 'a region end stops at its own border');
```

  Then insert before the final `console.log`:

```js
// ---- canvas viewport --------------------------------------------------------
{
    const view = { x: 40, y: -20, zoom: 0.5 };
    const world = model.toWorld(view, { x: 140, y: 80 });
    assert.deepEqual(plain(world), { x: 200, y: 200 });
    assert.deepEqual(plain(model.toScreen(view, world)), { x: 140, y: 80 });
    const zoomed = model.zoomAt(view, 1, { x: 140, y: 80 });
    assert.equal(zoomed.zoom, 1);
    assert.deepEqual(plain(model.toWorld(zoomed, { x: 140, y: 80 })), { x: 200, y: 200 }, 'zoomAt keeps the world point under the pointer');
    assert.equal(model.clampZoom(9), 2);
    assert.equal(model.clampZoom(0.01), 0.25);
    assert.equal(model.clampZoom(NaN), 1);
    const fitted = model.fitView({ x: 0, y: 0, w: 400, h: 200 }, { w: 1000, h: 800 });
    assert.deepEqual(plain(fitted), { x: 300, y: 300, zoom: 1 }, 'fit never zooms beyond 100 %');
    assert.equal(model.fitView({ x: 0, y: 0, w: 1808, h: 200 }, { w: 1000, h: 800 }).zoom, 0.5, 'fit keeps a 48 px margin');
    const stored = model.viewCenter({ x: 100, y: 50, zoom: 2 }, { w: 800, h: 600 });
    assert.deepEqual(plain(model.viewFromCenter(stored, { w: 800, h: 600 })), { x: 100, y: 50, zoom: 2 });
    assert.equal(model.viewFromCenter({ cx: 'x' }, { w: 1, h: 1 }), null);
    assert.equal(model.snap(35), 24);
    assert.equal(model.snap(37), 48);
    assert.equal(model.snap(-13), -24);
}

// ---- geometry, regions and drops --------------------------------------------
{
    const a = model.cardRect({ x: 0, y: 0 });
    assert.deepEqual(plain(a), { x: -92, y: -30, w: 184, h: 60 });
    const slot = model.freeSlot({ x: 0, y: 0 }, [a]);
    assert.ok(!model.intersects(model.cardRect(slot), a), 'the nudged card no longer overlaps');
    assert.equal(Math.hypot(slot.x, slot.y), 72, 'the nearest free slot is one card height plus the gap away');
    assert.ok(Number.isInteger(slot.x / 24) && Number.isInteger(slot.y / 24), 'a nudged card lands on the grid');
    assert.deepEqual(plain(model.freeSlot({ x: 500, y: 501 }, [a])), { x: 500, y: 501 }, 'a free drop keeps its exact point');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0 })), { x: 0, y: 0, w: 360, h: 240 }, 'a place without size gets 360×240');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0, w: 200, h: 140 }, [model.cardRect({ x: 400, y: 100 })])), { x: 0, y: 0, w: 540, h: 178 }, 'children grow the region by the 48 px frame padding');
    const placements = { site: { x: 0, y: 0, w: 200, h: 140 }, rack: { x: 300, y: 300, w: 200, h: 140 } };
    const parentOf = id => ({ rack: 'site' })[id] || '';
    const layout = model.regionLayout(['site', 'rack'], { parentOf, placementOf: id => placements[id], cardRectsByParent: new Map([['rack', [model.cardRect({ x: 400, y: 380 })]]]) });
    assert.deepEqual(plain(layout.rects.get('rack')), { x: 260, y: 300, w: 280, h: 158 });
    assert.deepEqual(plain(layout.rects.get('site')), { x: 0, y: 0, w: 588, h: 506 }, 'a nested place grows its parent');
    assert.equal(layout.depth.get('rack'), 1);
    assert.equal(model.regionAt({ x: 400, y: 380 }, layout.rects, layout.depth), 'rack', 'hit-testing returns the innermost region');
    assert.equal(model.regionAt({ x: 50, y: 50 }, layout.rects, layout.depth), 'site');
    assert.equal(model.regionAt({ x: 5000, y: 50 }, layout.rects, layout.depth), '');
    assert.equal(model.regionAt({ x: 400, y: 380 }, layout.rects, layout.depth, new Set(['rack'])), 'site', 'excluded regions are skipped');
    assert.deepEqual(plain(model.dropParent('', { x: 400, y: 380 }, layout.rects, layout.depth)), { parent_id: 'rack' });
    assert.deepEqual(plain(model.dropParent('rack', { x: 400, y: 380 }, layout.rects, layout.depth)), {}, 'staying in the same place leaves parent_id unchanged');
    assert.deepEqual(plain(model.dropParent('rack', { x: 5000, y: 5000 }, layout.rects, layout.depth)), { parent_id: '' }, 'leaving every region clears the parent');
    assert.deepEqual(plain(model.dropParent('hidden-room', { x: 5000, y: 5000 }, layout.rects, layout.depth)), {}, 'a parent that is not drawn here is never cleared');
    const growth = model.growRegions('rack', model.cardRect({ x: 700, y: 380 }), { ids: new Set(['site', 'rack']), parentOf, placementOf: id => placements[id] });
    assert.deepEqual(plain(growth.get('rack')), { x: 300, y: 300, w: 540, h: 158 });
    assert.deepEqual(plain(growth.get('site')), { x: 0, y: 0, w: 888, h: 506 }, 'growth propagates to the ancestors');
    const items = [{ id: 'c', x: 0, y: 0 }, { id: 'r', x: 220, y: 10 }, { id: 'far', x: 600, y: 0 }, { id: 'd', x: 0, y: 155 }, { id: 'diag', x: 150, y: 150 }];
    assert.equal(model.spatialNeighbour(items[0], items, 'right'), 'r');
    assert.equal(model.spatialNeighbour(items[0], items, 'down'), 'd');
    assert.equal(model.spatialNeighbour(items[0], items, 'left'), '');
    assert.deepEqual(plain(model.compactGrid(4, { x: 0, y: 0 })), [{ x: -96, y: -48 }, { x: 120, y: -48 }, { x: -96, y: 48 }, { x: 120, y: 48 }]);
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'router' }, { kind: 'device', icon: 'nas' }), 'ethernet');
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'access-point' }, { kind: 'device', icon: 'laptop' }), 'wifi');
    assert.equal(model.defaultEdgeKind({ kind: 'service', icon: 'web-service' }, { kind: 'device', icon: 'nas' }), 'depends_on');
    const mini = model.minimapTransform({ x: 0, y: 0, w: 1000, h: 500 }, { w: 200, h: 130 });
    assert.ok(Math.abs(mini.scale - 0.188) < 1e-9 && Math.abs(mini.x - 6) < 1e-9 && Math.abs(mini.y - 18) < 1e-9, 'the minimap centres the world');
}

// ---- guests, filters and the scene ------------------------------------------
{
    const guestNodes = [{ id: 'host', kind: 'device' }, { id: 'vm', kind: 'vm', host_id: 'host' }, { id: 'svc', kind: 'service' }, { id: 'ct', kind: 'container', host_id: 'gone' }, { id: 'dev', kind: 'device', host_id: 'host' }];
    const guests = model.guestIndex(guestNodes, [{ id: 'e1', kind: 'runs_on', from: 'svc', to: 'vm' }]);
    assert.equal(guests.hostOf.get('vm'), 'host', 'host_id wins');
    assert.equal(guests.hostOf.get('svc'), 'vm', 'without host_id a runs_on edge names the host');
    assert.equal(guests.hostOf.has('ct'), false, 'an unknown host leaves the guest hostless');
    assert.equal(guests.hostOf.has('dev'), false, 'devices are never guests');
    assert.deepEqual(plain(guests.guestsOf.get('host')), ['vm']);
    const textOf = node => String(node.name).toLocaleLowerCase();
    const nas = { name: 'Cold NAS', kind: 'device', power: 'running', sources: [{ kind: 'docker' }] };
    assert.equal(model.matchesFilters(nas, { query: 'cold' }, textOf), true);
    assert.equal(model.matchesFilters(nas, { query: 'warm' }, textOf), false);
    assert.equal(model.matchesFilters(nas, { kind: 'vm' }, textOf), false);
    assert.equal(model.matchesFilters(nas, { source: 'docker', power: 'running' }, textOf), true);
    assert.equal(model.matchesFilters(nas, { power: 'none' }, textOf), false);
    assert.equal(model.filtersActive({ query: '  ' }), false);
    assert.equal(model.filtersActive({ kind: 'vm' }), true);
    const sceneNodes = [
        { id: 'room', kind: 'room', placements: { atlas: { x: 0, y: 0, w: 600, h: 300 } } },
        { id: 'host', kind: 'device', parent_id: 'room', placements: { atlas: { x: 200, y: 150 } } },
        { id: 'svc', kind: 'service', placements: { atlas: { x: 900, y: 900 } } },
        { id: 'nas', kind: 'device', placements: { atlas: { x: 900, y: 150 } } },
        { id: 'loose', kind: 'device', placements: {} },
    ];
    const sceneEdges = [{ id: 'r', kind: 'runs_on', from: 'svc', to: 'host' }, { id: 'l', kind: 'ethernet', from: 'svc', to: 'nas' }];
    const byId = new Map(sceneNodes.map(node => [node.id, node]));
    const input = { nodes: sceneNodes, edges: sceneEdges, placementOf: id => byId.get(id).placements.atlas, parentOf: id => byId.get(id)?.parent_id || '', guests: model.guestIndex(sceneNodes, sceneEdges) };
    const scene = model.buildScene({ ...input, view: 'atlas' });
    assert.deepEqual(plain(scene.cards.map(card => card.id)), ['host', 'nas'], 'guests and unplaced assets are not cards; places are regions');
    assert.deepEqual(plain(scene.cards[0].guests), ['svc']);
    assert.deepEqual(plain(scene.regions.map(region => region.id)), ['room']);
    assert.deepEqual(plain(scene.lines.map(line => [line.id, line.from, line.to])), [['l', 'host', 'nas']], 'runs_on is implied by nesting and a guest line attaches to its host');
    assert.equal(model.buildScene({ ...input, view: 'topology' }).regions.length, 0, 'topology hides regions');
    const normalized = model.normalizeNode({ id: 'n', icon: 'nope' });
    assert.equal(normalized.icon, 'generic-device');
    assert.equal(normalized.parent_id, '');
    assert.equal('host_id' in normalized, false, 'a node never gains fields the node PUT would reject');
}

// ---- layout intents, history and prefs --------------------------------------
{
    assert.deepEqual(plain(model.mergeIntent({ x: 1, y: 1, parent_id: 'room' }, { x: 5, y: 6, pinned: true })), { x: 5, y: 6, parent_id: 'room', pinned: true });
    const nodesById = new Map([['a', { id: 'a', kind: 'device', version: 3, placements: {} }], ['r', { id: 'r', kind: 'room', version: 7, placements: { atlas: { x: 0, y: 0, w: 400, h: 300 } } }]]);
    const intents = new Map([['a', { x: 10.4, y: 20.6, parent_id: 'r' }], ['r', { x: 24, y: 48 }], ['gone', { x: 0, y: 0 }]]);
    const legacy = model.layoutItems(intents, nodesById, { parent: false, view: 'atlas' });
    assert.deepEqual(plain(legacy.items), [{ id: 'a', version: 3, x: 10, y: 21, pinned: true }, { id: 'r', version: 7, x: 24, y: 48, pinned: true }], 'without server support parent and size stay local');
    assert.deepEqual(plain(legacy.missing), ['gone']);
    const extended = model.layoutItems(intents, nodesById, { parent: true, view: 'atlas' });
    assert.deepEqual(plain(extended.items), [{ id: 'a', version: 3, x: 10, y: 21, pinned: true, parent_id: 'r' }, { id: 'r', version: 7, x: 24, y: 48, pinned: true, w: 400, h: 300 }], 'a place always carries its size so the server never drops it');
    const rebased = model.rebaseLayout(new Map([['a', { x: 1, y: 2, parent_id: 'gone' }], ['b', { x: 3, y: 4 }]]), nodesById);
    assert.deepEqual(plain([...rebased.kept]), [['a', { x: 1, y: 2 }]], 'a vanished parent is dropped, the coordinates stay');
    assert.deepEqual(plain(rebased.dropped), ['b']);
    const history = model.createHistory(2);
    history.push('one'); history.push('two'); history.push('three');
    assert.equal(history.takeUndo(), 'three');
    history.pushRedo('three');
    assert.equal(history.canRedo(), true);
    history.push('four');
    assert.equal(history.canRedo(), false, 'a new action clears redo');
    assert.equal(history.takeUndo(), 'four');
    assert.equal(history.takeUndo(), 'two');
    assert.equal(history.takeUndo(), null, 'depth is capped');
    assert.deepEqual(plain(model.readLayoutPrefs('{"libraryWidth":9999,"inspectorWidth":"x","libraryCollapsed":true,"minimap":false}')), { libraryWidth: 400, libraryCollapsed: true, inspectorWidth: 360, minimap: false });
    assert.deepEqual(plain(model.readLayoutPrefs('not json')), { libraryWidth: 264, libraryCollapsed: false, inspectorWidth: 360, minimap: true });
    assert.equal(model.fill('{count} of {total}', { count: 2, total: 5 }), '2 of 5');
    const full = [{ id: 'c', version: 1, parent_id: 'room', placements: {} }];
    model.applyLayoutPatch(full, [{ id: 'c', version: 3, placements: { atlas: { x: 1, y: 2, pinned: true } } }], 'atlas');
    assert.equal(full[0].parent_id, '', 'a full node without parent_id has left its place');
    model.applyLayoutPatch(full, [{ id: 'c', version: 4, parent_id: 'rack', placements: { atlas: { x: 1, y: 2, pinned: true, w: 300, h: 200 } } }], 'atlas');
    assert.equal(full[0].parent_id, 'rack');
    assert.equal(full[0].placements.atlas.w, 300);
    model.applyLayoutPatch(full, [{ id: 'c', version: 5, x: 3, y: 4, pinned: true }], 'atlas');
    assert.equal(full[0].parent_id, 'rack', 'a bare position patch leaves the parent alone');
    const hostedStamp = model.snapshotStamp([{ ...base.nodes[0], host_id: 'h' }], base.edges, base.sources, base.caps);
    assert.notEqual(same, hostedStamp, 'a new host is a structural change');
}
```

- [ ] **Step 7: Verify.**
  - `node --check ui/js/desktop/apps/infra-model.js`, then V-core. Expected: everything passes, and the final line reports the translation key count.
  - Then `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(UnsavedDraftClose|PendingRequestsAbortOnClose)Browser' -count=1 -timeout 15m`. The old UI still works with the clipped `edgeSegment`.
- [ ] **Step 8: Commit.**
  - Paths: `ui/js/desktop/apps/infra-model.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add canvas model helpers for viewport, regions, drops and layout intents`.

---

## Task 2: i18n batch A (store and shared keys) and the insertion script

**Files:**
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
- Modify: `scripts/test-infra-ui.mjs`: add `'edge_exists'` to the `infra_error` family list on line 88.

- [ ] **Step 1: Write the insertion script** to `$SCRATCH/infra-i18n-add.mjs`. Every later i18n task reuses this file.

```js
// Usage: node infra-i18n-add.mjs <repoRoot> <batch.json>
// batch.json: { "en": { "desktop.infra_x": "…" }, "de": { … }, … all 16 locales }
import fs from 'node:fs';
import path from 'node:path';

const [root, batchFile] = process.argv.slice(2);
const batch = JSON.parse(fs.readFileSync(batchFile, 'utf8'));
const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const keys = Object.keys(batch.en || {});
if (!keys.length) throw new Error('batch has no English keys');
for (const locale of locales) {
    const entries = batch[locale];
    if (!entries) throw new Error(`batch lacks ${locale}`);
    for (const key of keys) if (typeof entries[key] !== 'string' || !entries[key].trim()) throw new Error(`${locale} lacks ${key}`);
    const file = path.join(root, 'ui/lang/desktop', `${locale}.json`);
    const text = fs.readFileSync(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const existing = JSON.parse(text);
    const fresh = keys.filter(key => !(key in existing));
    if (!fresh.length) { console.log(`${locale}: nothing new`); continue; }
    const lines = text.split(eol);
    let anchor = -1;
    lines.forEach((line, index) => { if (/^\s*"desktop\.infra_/.test(line)) anchor = index; });
    if (anchor < 0) throw new Error(`${locale}: no desktop.infra_ anchor line`);
    const hadComma = lines[anchor].trimEnd().endsWith(',');
    if (!hadComma) lines[anchor] = lines[anchor].replace(/\s*$/, ',');
    const added = fresh.map((key, i) => `  ${JSON.stringify(key)}: ${JSON.stringify(entries[key])}${i < fresh.length - 1 || hadComma ? ',' : ''}`);
    lines.splice(anchor + 1, 0, ...added);
    const out = lines.join(eol);
    JSON.parse(out);
    fs.writeFileSync(file, out);
    console.log(`${locale}: +${fresh.length}`);
}
```

- [ ] **Step 2: Write `$SCRATCH/infra-i18n-a.json`** with all 16 locales.

  English and German texts are below. Translate the other 14 locales properly:
  - Native phrasing.
  - Placeholders (`{count}`, `{name}`, `{place}`, `{running}`) unchanged.
  - No English prose. `node scripts/test-infra-ui.mjs` rejects identical English text longer than 20 characters that contains spaces.

| key | en | de |
|---|---|---|
| `desktop.infra_layout_dropped` | Some positions were dropped because their assets no longer exist. | Einige Positionen wurden verworfen, weil die Assets nicht mehr existieren. |
| `desktop.infra_layout_unsaved` | Positions not saved | Positionen nicht gespeichert |
| `desktop.infra_sync_stale` | Infra could not refresh. You are seeing the last known state. | Infra konnte nicht aktualisieren. Du siehst den letzten bekannten Stand. |
| `desktop.infra_sync_recovered` | Infra is up to date again. | Infra ist wieder aktuell. |
| `desktop.infra_error_edge_exists` | This connection already exists. | Diese Verbindung gibt es bereits. |
| `desktop.infra_undone` | Undone | Rückgängig gemacht |
| `desktop.infra_redone` | Redone | Wiederhergestellt |

- [ ] **Step 3: Run the script.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && node "$SCRATCH/infra-i18n-add.mjs" . "$SCRATCH/infra-i18n-a.json"
  ```
  Expected: `<locale>: +7` for each of the 16 locales.
- [ ] **Step 4: Add `'edge_exists'`** to the `infra_error` array in `scripts/test-infra-ui.mjs` (line 88).
- [ ] **Step 5: Verify:** V-core, and `git diff --stat` shows exactly 17 files changed.
- [ ] **Step 6: Commit.**
  - Paths: `ui/lang/desktop/*.json`, `scripts/test-infra-ui.mjs`.
  - Subject: `i18n(infra): add layout, sync and connection keys for the store`.

---

## Task 3: Store (`infra-store.js`) and the store tests

**Files:**
- Create: `ui/js/desktop/apps/infra-store.js`
- Modify: `scripts/test-infra-ui.mjs`: file list becomes `['infra-model.js', 'infra-store.js', 'infra-views.js', 'infra.js']`, plus the async tests.

- [ ] **Step 1: Create `infra-store.js`** with exactly this structure.
  - Sections marked **PORT** are copied verbatim from the current `ui/js/desktop/apps/infra.js`, which is unchanged until Task 12. Only the listed edits apply.
  - Everything else is new code given in full.

```js
// Infra window state: API client, snapshot polling and visibility, CAS writes,
// the per-view layout queue with one conflict rebase, and undo/redo. Nothing
// here touches the DOM outside attach(): scripts/test-infra-ui.mjs runs the
// queue against a fake API inside a Node vm.
(function () {
    'use strict';

    const API = '/api/desktop/infra';
    const VIEWS = ['atlas', 'topology'];

    const edgeFields = edge => {
        const fields = { from: edge.from, to: edge.to, kind: edge.kind };
        for (const key of ['label', 'from_port', 'to_port', 'speed']) if (edge[key]) fields[key] = String(edge[key]);
        return fields;
    };
    const layoutCommand = (type, view, before, after, label) => ({
        type, view, before, after, label, optimistic: true, undoable: type !== 'PlaceFromLibrary',
        apply: store => store.queueLayout(view, after),
        revert: store => store.queueLayout(view, before),
    });
    // Layout commands are optimistic: the overlay shows the result at once and
    // history records them at once. PlaceFromLibrary is not undoable because
    // PUT /layout cannot remove a placement.
    const commands = {
        moveNodes: (view, before, after, label = '') => layoutCommand('MoveNodes', view, before, after, label),
        reparent: (view, before, after, label = '') => layoutCommand('Reparent', view, before, after, label),
        resizePlace: (view, before, after, label = '') => layoutCommand('ResizePlace', view, before, after, label),
        placeFromLibrary: (view, after, label = '') => layoutCommand('PlaceFromLibrary', view, new Map(), after, label),
        createEdge: fields => ({
            type: 'CreateEdge', fields: { ...fields }, edgeId: '', undoable: true,
            async apply(store) { const edge = await store.createEdge(this.fields); this.edgeId = edge?.id || ''; return !!this.edgeId; },
            async revert(store) { return store.deleteEdgeById(this.edgeId); },
        }),
        deleteEdge: edge => ({
            type: 'DeleteEdge', fields: edgeFields(edge), edgeId: edge.id, undoable: true,
            async apply(store) { return store.deleteEdgeById(this.edgeId); },
            async revert(store) { const created = await store.createEdge(this.fields); this.edgeId = created?.id || ''; return !!this.edgeId; },
        }),
    };

    function create({ ctx = {}, t: translate } = {}) {
        const model = window.InfraAppModel;
        const t = translate || (key => ctx.t?.(key) || key);
        const listeners = new Set();
        const features = { layoutParent: false };
        const lanes = {}, histories = {};
        for (const view of VIEWS) {
            lanes[view] = { pending: new Map(), inflight: null, unsaved: new Map(), waiters: [], scheduled: false };
            histories[view] = model.createHistory(50);
        }
        const state = {
            disposed: false, epoch: 1, visible: false, inView: true, pollPaused: false, timer: 0,
            requests: new Set(), controller: new AbortController(), manifestController: null, manifestLoaded: false, manifestLoading: false,
            caps: { write: false, source_edit: false, control: false },
            nodes: [], edges: [], sources: [], sourceHosts: [], candidates: [],
            appTargets: [], targetsLoaded: false, targetsError: false,
            credentials: [], credentialsLoaded: false, credentialsLoading: false, credentialsError: false,
            stamp: '', refreshAt: 0, sync: 'syncing',
            view: 'atlas', tool: 'select', filters: { query: '', kind: '', source: '', power: '' },
            selection: new Set(), selected: '', selectedEdge: '', expandedHosts: new Set(), interacting: 0, historyBusy: false,
            drafts: new Map(), dirty: new Set(), draftRevision: new Map(), nodeReviews: new Map(), customRows: new Map(),
            sourceDrafts: new Map(), sourceDirty: new Set(), sourceRevision: new Map(), sourceReviews: new Map(), sourceBlocked: new Set(), sourcePending: new Set(),
            actionBlocks: new Map(), importSelection: new Set(), importBindings: new Map(), importDialogOwner: '', importOpen: false, dialogGeneration: 0,
            busy: new Set(),
        };
        let detach = () => {};
        const emit = change => { if (!state.disposed) for (const fn of [...listeners]) fn(change); };
        const subscribe = fn => { listeners.add(fn); return () => listeners.delete(fn); };
        const notify = (message, kind = 'info') => emit({ type: 'notice', message, kind });
        const setBusy = (key, busy) => { if (busy) state.busy.add(key); else state.busy.delete(key); emit({ type: 'busy', key }); };

        // ---- permissions -----------------------------------------------------
        const canWrite = () => state.caps.write === true && ctx.readonly !== true;
        const canEditSource = () => state.caps.source_edit === true && ctx.readonly !== true;
        const canControl = () => state.caps.control === true && ctx.readonly !== true;
        // PORT infra.js 258-263 (allowedAction, actionReason, actionBlockKey) verbatim.

        // ---- requests --------------------------------------------------------
        // PORT infra.js 94-127 (abortRequests, request) verbatim.
        const messageFor = error => {
            const code = String(error?.body?.error || error?.message || 'generic').toLowerCase().replace(/[^a-z0-9_]/g, '_');
            const key = `desktop.infra_error_${code}`;
            const text = t(key);
            if (text !== key) return text;
            return t(error?.status === 409 ? 'desktop.infra_error_conflict' : 'desktop.infra_error_infra_error');
        };
        // PORT infra.js 136-140 (uncertainMutation, isAccessError) verbatim.

        // ---- derived indexes (keyed by array identity; every change replaces the array)
        const nodeIndexCache = new WeakMap(), childrenCache = new WeakMap(), guestCache = new WeakMap();
        const nodeIndex = () => { let index = nodeIndexCache.get(state.nodes); if (!index) { index = new Map(state.nodes.map(node => [node.id, node])); nodeIndexCache.set(state.nodes, index); } return index; };
        const nodeById = id => nodeIndex().get(id);
        const childrenOf = id => {
            let index = childrenCache.get(state.nodes);
            if (!index) { index = new Map(); for (const node of state.nodes) if (node.parent_id) { const list = index.get(node.parent_id) || []; list.push(node); index.set(node.parent_id, list); } childrenCache.set(state.nodes, index); }
            return index.get(id) || [];
        };
        const guests = () => {
            const cached = guestCache.get(state.nodes);
            if (cached && cached.edges === state.edges) return cached.value;
            const value = model.guestIndex(state.nodes, state.edges);
            guestCache.set(state.nodes, { edges: state.edges, value });
            return value;
        };
        // PORT infra.js 213-236 (searchableCache, searchableText) verbatim.
        const matches = node => model.matchesFilters(node, state.filters, searchableText);

        // ---- snapshot ----------------------------------------------------------
        function setSync(next) {
            const previous = state.sync;
            if (previous === next) return;
            state.sync = next;
            const announce = next === 'error' && previous === 'live' ? t('desktop.infra_sync_stale') : next === 'live' && previous === 'error' ? t('desktop.infra_sync_recovered') : '';
            emit({ type: 'sync', announce });
        }
        function pruneSelection() {
            const removed = [...state.selection].filter(id => !nodeById(id));
            if (!removed.length && (!state.selectedEdge || state.edges.some(edge => edge.id === state.selectedEdge))) return;
            if (removed.some(id => state.dirty.has(id))) notify(t('desktop.infra_remote_removed_draft'), 'error');
            for (const id of removed) state.selection.delete(id);
            if (state.selectedEdge && !state.edges.some(edge => edge.id === state.selectedEdge)) state.selectedEdge = '';
            state.selected = state.selection.size === 1 ? [...state.selection][0] : '';
            emit({ type: 'selection' });
        }
        function applySnapshot(data, { explicit = false } = {}) {
            const previousCaps = JSON.stringify(state.caps);
            state.nodes = (data.nodes || []).map(model.normalizeNode);
            state.edges = Array.isArray(data.edges) ? data.edges : [];
            state.sources = Array.isArray(data.sources) ? data.sources : [];
            features.layoutParent = Array.isArray(data.source_hosts);
            state.sourceHosts = features.layoutParent ? data.source_hosts : [];
            state.caps = { write: false, source_edit: false, control: false, ...(data.capabilities || {}) };
            for (const node of state.nodes) {
                if (state.nodeReviews.has(node.id) && state.dirty.has(node.id)) state.nodeReviews.set(node.id, { latest: model.clone(node) });
                if (!state.dirty.has(node.id)) state.drafts.set(node.id, model.clone(node));
            }
            pruneSelection();
            const stamp = model.snapshotStamp(state.nodes, state.edges, state.sources, state.caps);
            const structural = stamp !== state.stamp;
            state.stamp = stamp;
            state.refreshAt = Date.now();
            emit({ type: 'snapshot', structural, explicit });
            if (previousCaps !== JSON.stringify(state.caps)) emit({ type: 'caps' });
        }
        async function loadSnapshot(explicit = false) {
            if (state.disposed || !state.visible || state.busy.has('snapshot') || (state.pollPaused && !explicit)) return;
            const generation = state.epoch;
            setBusy('snapshot', true);
            try {
                const data = await request(`/snapshot${explicit ? '?refresh=1' : ''}`, {}, generation);
                state.pollPaused = false;
                applySnapshot(data, { explicit });
                setSync('live');
                schedulePoll();
            } catch (error) {
                if (error.name !== 'AbortError') {
                    if (isAccessError(error)) {
                        state.caps = { write: false, source_edit: false, control: false };
                        state.pollPaused = true; abortRequests(); emit({ type: 'caps' });
                        notify(t('desktop.infra_access_unavailable'), 'error'); setSync('paused');
                    } else { notify(messageFor(error), 'error'); setSync('error'); }
                }
            } finally { setBusy('snapshot', false); }
        }
        async function refreshSnapshot() {
            try { applySnapshot(await request('/snapshot')); return true; }
            catch (error) { if (error?.name !== 'AbortError') notify(messageFor(error), 'error'); return false; }
        }
        // PORT infra.js 331-340 (loadTargets): replace paintInspector() with emit({ type: 'targets' }).
        // PORT infra.js 341-353 (loadCredentials): replace both paintInspector() calls with emit({ type: 'credentials' }).
        // PORT infra.js 153-177 (loadIconManifest): replace paintAll() with emit({ type: 'manifest' }).

        const replaceNode = node => {
            const normalized = model.normalizeNode(node);
            const nodes = state.nodes.slice();
            const index = nodes.findIndex(item => item.id === normalized.id);
            if (index < 0) nodes.push(normalized); else nodes[index] = normalized;
            state.nodes = nodes;
            if (!state.dirty.has(normalized.id)) state.drafts.set(normalized.id, model.clone(normalized));
            emit({ type: 'nodes' });
            return normalized;
        };
        const removeNode = id => {
            state.nodes = state.nodes.filter(item => item.id !== id);
            state.edges = state.edges.filter(edge => edge.from !== id && edge.to !== id);
            state.drafts.delete(id); state.dirty.delete(id);
            for (const view of VIEWS) { lanes[view].pending.delete(id); lanes[view].unsaved.delete(id); }
            pruneSelection();
            emit({ type: 'nodes' }); emit({ type: 'edges' });
        };

        // ---- selection, view, filters --------------------------------------------
        function select(ids, { toggle = false, additive = false } = {}) {
            const next = toggle || additive ? new Set(state.selection) : new Set();
            for (const id of ids) { if (toggle && next.has(id)) next.delete(id); else if (nodeById(id)) next.add(id); }
            state.selection = next; state.selectedEdge = '';
            state.selected = next.size === 1 ? [...next][0] : '';
            if (state.selected && !state.dirty.has(state.selected)) state.drafts.set(state.selected, model.clone(nodeById(state.selected)));
            emit({ type: 'selection' });
        }
        const selectEdge = id => { state.selection = new Set(); state.selected = ''; state.selectedEdge = state.edges.some(edge => edge.id === id) ? id : ''; emit({ type: 'selection' }); };
        const clearSelection = () => { if (!state.selection.size && !state.selectedEdge) return; state.selection = new Set(); state.selected = ''; state.selectedEdge = ''; emit({ type: 'selection' }); };
        const setView = view => { if (state.view === view) return; state.view = view; emit({ type: 'view' }); };
        const setFilters = partial => { state.filters = { ...state.filters, ...partial }; emit({ type: 'filters' }); };
        const setTool = tool => { if (state.tool === tool) return; state.tool = tool; emit({ type: 'tool' }); };
        const toggleHost = id => { if (state.expandedHosts.has(id)) state.expandedHosts.delete(id); else state.expandedHosts.add(id); emit({ type: 'expanded' }); };
        const beginInteraction = () => { state.interacting += 1; };
        const endInteraction = () => { state.interacting = Math.max(0, state.interacting - 1); if (!state.interacting) emit({ type: 'interaction-end' }); };

        // ---- layout overlay -------------------------------------------------------
        const effectiveIntent = (view, id) => {
            const lane = lanes[view];
            if (!lane) return null;
            const parts = [lane.unsaved.get(id), lane.inflight?.get(id), lane.pending.get(id)].filter(Boolean);
            return parts.length ? parts.reduce(model.mergeIntent, {}) : null;
        };
        function effectivePlacement(id, view) {
            const node = nodeById(id);
            if (!node) return null;
            const base = node.placements?.[view] || null;
            const intent = effectiveIntent(view, id);
            if (!intent) return base;
            const out = { ...(base || {}), x: intent.x, y: intent.y, pinned: intent.pinned !== false };
            if (intent.w != null && intent.h != null) { out.w = intent.w; out.h = intent.h; }
            return out;
        }
        function effectiveParent(id, view) {
            const intent = effectiveIntent(view, id);
            return intent && intent.parent_id !== undefined ? intent.parent_id : (nodeById(id)?.parent_id || '');
        }
        function currentIntent(id, view, withParent = false) {
            const node = nodeById(id), placement = effectivePlacement(id, view);
            if (!node || !placement) return null;
            const intent = { x: Number(placement.x), y: Number(placement.y), pinned: placement.pinned !== false };
            if (withParent) intent.parent_id = effectiveParent(id, view);
            if (model.isContainer(node.kind)) { intent.w = Number(placement.w) || model.REGION_W; intent.h = Number(placement.h) || model.REGION_H; }
            return intent;
        }
        const layoutState = (id, view) => { const lane = lanes[view]; if (!lane) return ''; if (lane.unsaved.has(id)) return 'unsaved'; return lane.pending.has(id) || lane.inflight?.has(id) ? 'saving' : ''; };
        const unsavedCount = view => lanes[view]?.unsaved.size || 0;
        const laneIdle = lane => !lane.inflight && !lane.pending.size;
        function settle(view) { const lane = lanes[view]; if (!laneIdle(lane)) return; const ok = !lane.unsaved.size; const waiters = lane.waiters.splice(0); waiters.forEach(resolve => resolve(ok)); }
        const whenLayoutIdle = view => { const lane = lanes[view]; return laneIdle(lane) ? Promise.resolve(!lane.unsaved.size) : new Promise(resolve => lane.waiters.push(resolve)); };
        function scheduleFlush(view) {
            const lane = lanes[view];
            if (lane.scheduled) return;
            lane.scheduled = true;
            Promise.resolve().then(() => { lane.scheduled = false; void flush(view); });
        }
        // queueLayout overlays intents at once and saves them in one PUT per view;
        // moves queued while a save is in flight coalesce into the next PUT.
        function queueLayout(view, intents) {
            const lane = lanes[view];
            if (!lane || !intents || !intents.size || !canWrite()) return Promise.resolve(false);
            for (const [id, intent] of intents) {
                const earlier = lane.unsaved.get(id);
                lane.unsaved.delete(id);
                lane.pending.set(id, model.mergeIntent(model.mergeIntent(earlier, lane.pending.get(id)), intent));
            }
            emit({ type: 'layout', view, ids: [...intents.keys()] });
            emit({ type: 'layout-status', view });
            const done = new Promise(resolve => lane.waiters.push(resolve));
            scheduleFlush(view);
            return done;
        }
        function moveToUnsaved(lane, batch) {
            for (const [id, intent] of batch) {
                if (lane.pending.has(id)) lane.pending.set(id, model.mergeIntent(intent, lane.pending.get(id)));
                else lane.unsaved.set(id, model.mergeIntent(lane.unsaved.get(id), intent));
            }
        }
        // alignDraft keeps a dirty draft saveable after our own layout write
        // bumped the node version (and, for a drop, changed its parent).
        function alignDraft(id, previousVersion, intent) {
            const node = nodeById(id), draft = state.drafts.get(id);
            if (!node || !draft) return;
            if (!state.dirty.has(id)) { state.drafts.set(id, model.clone(node)); return; }
            if (draft.version !== previousVersion) return;
            draft.version = node.version;
            if (intent?.parent_id !== undefined) draft.parent_id = node.parent_id;
        }
        function applyLayoutResult(result, view, batch) {
            const snapshot = model.layoutSnapshot(result);
            if (snapshot) { applySnapshot(snapshot, { explicit: true }); return; }
            const rows = Array.isArray(result?.nodes) ? result.nodes : Array.isArray(result) ? result : [];
            const before = new Map(rows.map(row => [row?.id, nodeById(row?.id)?.version]));
            model.applyLayoutPatch(state.nodes, rows, view);
            state.nodes = state.nodes.slice(); // a parent change invalidates the derived indexes
            for (const row of rows) alignDraft(row?.id, before.get(row?.id), batch.get(row?.id));
            emit({ type: 'nodes' });
        }
        async function sendBatch(view, batch) {
            const { items } = model.layoutItems(batch, nodeIndex(), { parent: features.layoutParent, view });
            if (!items.length) return 'ok';
            setBusy('layout', true);
            try {
                const result = await request('/layout', { method: 'PUT', body: { view, nodes: items } });
                applyLayoutResult(result, view, batch);
                return 'ok';
            } catch (error) {
                if (error?.status === 409) return 'conflict';
                if (error?.name !== 'AbortError') notify(messageFor(error), 'error');
                return 'failed';
            } finally { setBusy('layout', false); }
        }
        async function flush(view) {
            const lane = lanes[view];
            if (state.disposed || lane.inflight || !lane.pending.size) return;
            lane.inflight = lane.pending;
            lane.pending = new Map();
            let attempt = 0;
            for (;;) {
                const outcome = canWrite() ? await sendBatch(view, lane.inflight) : 'failed';
                if (outcome === 'conflict' && attempt === 0) {
                    attempt = 1;
                    histories[view].clear();
                    emit({ type: 'history' });
                    if (await refreshSnapshot()) {
                        const { kept, dropped } = model.rebaseLayout(lane.inflight, nodeIndex());
                        if (dropped.length) notify(t('desktop.infra_layout_dropped'), 'info');
                        lane.inflight = kept;
                        if (kept.size) continue;
                        break;
                    }
                }
                if (outcome !== 'ok') moveToUnsaved(lane, lane.inflight);
                break;
            }
            lane.inflight = null;
            emit({ type: 'layout', view });
            emit({ type: 'layout-status', view });
            if (lane.pending.size) scheduleFlush(view); else settle(view);
        }
        function retryLayout(view) {
            const lane = lanes[view];
            if (!lane.unsaved.size) return whenLayoutIdle(view);
            const intents = new Map(lane.unsaved);
            lane.unsaved.clear();
            return queueLayout(view, intents);
        }

        // ---- edges ------------------------------------------------------------
        async function createEdge(fields) {
            const result = await request('/edges', { method: 'POST', body: { id: '', version: 0, ...fields } });
            const edge = result?.edge || result;
            if (!edge?.id) { await refreshSnapshot(); return null; }
            state.edges = [...state.edges.filter(item => item.id !== edge.id), edge];
            emit({ type: 'edges' });
            return edge;
        }
        async function updateEdge(id, body) {
            const result = await request(`/edges/${encodeURIComponent(id)}`, { method: 'PUT', body });
            const edge = result?.edge || result;
            if (edge?.id) { state.edges = state.edges.map(item => (item.id === edge.id ? edge : item)); emit({ type: 'edges' }); }
            else await refreshSnapshot();
            return edge;
        }
        async function deleteEdgeById(id) {
            const edge = state.edges.find(item => item.id === id);
            if (!edge) return true;
            try { await request(`/edges/${encodeURIComponent(id)}?version=${encodeURIComponent(edge.version)}`, { method: 'DELETE' }); }
            catch (error) { if (error?.status !== 404) throw error; }
            state.edges = state.edges.filter(item => item.id !== id);
            if (state.selectedEdge === id) { state.selectedEdge = ''; emit({ type: 'selection' }); }
            emit({ type: 'edges' });
            return true;
        }

        // ---- history ---------------------------------------------------------------
        async function dispatch(command) {
            if (!command || !canWrite()) return false;
            const view = command.view || (state.view === 'table' ? 'atlas' : state.view);
            const history = histories[view];
            if (command.optimistic) {
                const done = command.apply(api);
                if (command.undoable) { history.push(command); emit({ type: 'history' }); }
                return done;
            }
            const ok = await command.apply(api);
            if (ok && command.undoable) { history.push(command); emit({ type: 'history' }); }
            return ok;
        }
        async function step(view, direction) {
            const history = histories[view];
            if (!history || state.historyBusy || !canWrite()) return false;
            const entry = direction === 'undo' ? history.takeUndo() : history.takeRedo();
            if (!entry) return false;
            const forward = direction === 'redo';
            const back = () => (forward ? history.pushRedo(entry) : history.pushUndo(entry));
            const done = () => (forward ? history.pushUndo(entry) : history.pushRedo(entry));
            if (entry.optimistic) { void (forward ? entry.apply(api) : entry.revert(api)); done(); emit({ type: 'history' }); return true; }
            state.historyBusy = true; emit({ type: 'history' });
            try {
                const ok = forward ? await entry.apply(api) : await entry.revert(api);
                if (ok) done(); else back();
                return !!ok;
            } catch (error) { back(); notify(messageFor(error), 'error'); return false; }
            finally { state.historyBusy = false; emit({ type: 'history' }); }
        }
        const canUndo = view => !!histories[view]?.canUndo() && !state.historyBusy;
        const canRedo = view => !!histories[view]?.canRedo() && !state.historyBusy;
        const hasUnsavedWork = () => state.dirty.size > 0 || state.sourceDirty.size > 0 || VIEWS.some(view => !laneIdle(lanes[view]) || lanes[view].unsaved.size > 0);

        // ---- visibility and polling (DOM; never called by the Node test) -----------
        // PORT infra.js 962-973 (visibility, schedulePoll) verbatim, except:
        //   `win` and `state.inView` come from attach(); visibility() takes no arguments.
        function attach({ root, win }) {
            // PORT infra.js 1018-1037 with these edits:
            //  - observe `win` and `root` as there; IntersectionObserver absent => state.inView = true;
            //  - onPolicy: replace `setNotice(t('desktop.infra_readonly_revoked'), 'info'); syncMenus(); paintAll();`
            //    with `notify(t('desktop.infra_readonly_revoked'), 'info'); setSync('paused'); emit({ type: 'caps' });`
            //    and in the enable branch replace `syncMenus();` with `emit({ type: 'caps' });`.
            // Store every listener/observer you create; `detach` removes them all.
            detach = () => { /* disconnect observers, remove the visibilitychange and aurago:desktop-policy listeners */ };
            visibility();
        }
        function dispose() {
            if (state.disposed) return;
            abortRequests();
            state.disposed = true;
            clearTimeout(state.timer); state.timer = 0;
            detach();
            listeners.clear();
            for (const view of VIEWS) lanes[view].waiters.splice(0).forEach(resolve => resolve(false));
        }

        const api = {
            state, features, subscribe, notify, setBusy, request, messageFor, uncertainMutation, isAccessError,
            canWrite, canEditSource, canControl, allowedAction, actionReason, actionBlockKey,
            nodeById, nodeIndex, childrenOf, guests, searchableText, matches,
            applySnapshot, loadSnapshot, loadTargets, loadCredentials, loadIconManifest, replaceNode, removeNode,
            select, selectEdge, clearSelection, setView, setFilters, setTool, toggleHost, beginInteraction, endInteraction,
            effectivePlacement, effectiveParent, currentIntent, layoutState, unsavedCount, queueLayout, whenLayoutIdle, retryLayout,
            createEdge, updateEdge, deleteEdgeById, dispatch, undo: view => step(view, 'undo'), redo: view => step(view, 'redo'),
            canUndo, canRedo, hasUnsavedWork, attach, abortRequests, dispose,
        };
        return api;
    }

    window.InfraAppStore = { create, commands, edgeFields };
})();
```

  Port notes:
  - Inside the ported `request()`, keep the credentials special case and the 20 s abort timer.
  - The ported `abortRequests()` already increments `epoch`, clears the timer, aborts requests and the manifest controller, and replaces `state.controller`.
  - The ported `schedulePoll()` must use the literal `30000` delay. The browser test hooks `window.setTimeout` and counts `delay === 30000`.

- [ ] **Step 2: Add the async store tests** to `scripts/test-infra-ui.mjs`. Update the file list as given under **Files**, then insert before the final `console.log`:

```js
// ---- store: layout queue, rebase, undo --------------------------------------
{
    const InfraStore = window.InfraAppStore;
    const storeNode = (id, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x: 0, y: 0, pinned: true } }, sources: [], ...extra });
    const conflict = () => Object.assign(new Error('conflict'), { status: 409, body: { error: 'conflict' } });
    const fakeApi = routes => {
        const calls = [];
        const api = async (url, options = {}) => {
            const route = `${options.method || 'GET'} ${String(url).replace('/api/desktop/infra', '').split('?')[0]}`;
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            calls.push({ route, body });
            const handler = routes[route];
            if (!handler) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
            return handler(body);
        };
        return { api, calls };
    };
    const newStore = api => InfraStore.create({ ctx: { api }, t: key => key });
    const echo = body => ({ nodes: body.nodes.map(item => storeNode(item.id, { version: item.version + 1, ...(item.parent_id !== undefined ? { parent_id: item.parent_id } : {}), placements: { atlas: { x: item.x, y: item.y, pinned: true } } })) });

    { // coalescing; an old server never receives parent_id
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a'), storeNode('room', { kind: 'room' })], edges: [], sources: [], capabilities: { write: true } });
        store.queueLayout('atlas', new Map([['a', { x: 24, y: 24 }]]));
        const done = store.queueLayout('atlas', new Map([['a', { x: 48, y: 48, parent_id: 'room' }]]));
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48, 'a queued move shows at once');
        assert.equal(await done, true);
        const puts = calls.filter(call => call.route === 'PUT /layout');
        assert.equal(puts.length, 1, 'moves made in the same tick coalesce into one save');
        assert.deepEqual(plain(puts[0].body.nodes), [{ id: 'a', version: 1, x: 48, y: 48, pinned: true }], 'a server without source_hosts never receives parent_id');
        assert.equal(store.nodeById('a').version, 2);
        store.dispose();
    }
    { // one conflict: reload, reapply onto the fresh version, resend once
        let serverVersion = 1, puts = 0;
        const snapshot = () => ({ nodes: [storeNode('a', { version: serverVersion }), storeNode('room', { kind: 'room' })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        const { api, calls } = fakeApi({
            'GET /snapshot': snapshot,
            'PUT /layout': body => { puts += 1; if (puts === 1) { serverVersion = 5; throw conflict(); } return echo(body); },
        });
        const store = newStore(api);
        store.applySnapshot(snapshot());
        await store.dispatch(InfraStore.commands.reparent('atlas', new Map([['a', { x: 0, y: 0, pinned: true, parent_id: '' }]]), new Map([['a', { x: 96, y: 0, pinned: true, parent_id: 'room' }]])));
        await store.whenLayoutIdle('atlas');
        const sent = calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes[0]);
        assert.equal(sent.length, 2, 'a conflict is retried exactly once');
        assert.deepEqual(plain(sent[1]), { id: 'a', version: 5, x: 96, y: 0, pinned: true, parent_id: 'room' }, 'the retry reapplies the intent onto the fresh version');
        assert.equal(calls.filter(call => call.route === 'GET /snapshot').length, 1);
        assert.equal(store.nodeById('a').parent_id, 'room');
        assert.equal(store.unsavedCount('atlas'), 0);
        assert.equal(store.canUndo('atlas'), false, 'a structural conflict clears the view history');
        store.dispose();
    }
    { // a second conflict leaves the position unsaved until Retry
        let fail = true;
        const { api, calls } = fakeApi({
            'GET /snapshot': () => ({ nodes: [storeNode('a')], edges: [], sources: [], source_hosts: [], capabilities: { write: true } }),
            'PUT /layout': body => { if (fail) throw conflict(); return echo(body); },
        });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a')], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        assert.equal(await store.queueLayout('atlas', new Map([['a', { x: 48, y: 0 }]])), false);
        assert.equal(calls.filter(call => call.route === 'PUT /layout').length, 2);
        assert.equal(store.unsavedCount('atlas'), 1, 'a second conflict keeps the position as unsaved');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48, 'the unsaved position stays visible');
        assert.equal(store.hasUnsavedWork(), true);
        fail = false;
        assert.equal(await store.retryLayout('atlas'), true);
        assert.equal(store.unsavedCount('atlas'), 0);
        store.dispose();
    }
    { // undo/redo and draft alignment
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a')], edges: [], sources: [], capabilities: { write: true } });
        store.state.drafts.set('a', { ...store.nodeById('a'), name: 'Draft name' });
        store.state.dirty.add('a');
        await store.dispatch(InfraStore.commands.moveNodes('atlas', new Map([['a', { x: 0, y: 0, pinned: true }]]), new Map([['a', { x: 96, y: 0, pinned: true }]])));
        assert.equal(store.state.drafts.get('a').version, 2, 'our own layout save moves a dirty draft onto the new version');
        assert.equal(store.state.drafts.get('a').name, 'Draft name');
        assert.equal(await store.undo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 0);
        assert.equal(await store.redo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.deepEqual(plain(calls.map(call => call.body.nodes[0].x)), [96, 0, 96]);
        store.dispose();
    }
}
```

- [ ] **Step 3: Verify.** Run `node --check ui/js/desktop/apps/infra-store.js`, then V-core.
  - Expected: all assertions pass, and `node scripts/test-infra-ui.mjs` exits promptly. A hang means an uncleared timer, usually a request timeout that was not cleared in `finally`.
  - The browser suite is unaffected because the store is not loaded by the app yet.
- [ ] **Step 4: Commit.**
  - Paths: `ui/js/desktop/apps/infra-store.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add store with layout queue, conflict rebase and undo history`.

---

## Task 4: i18n batch B (canvas)

**Files:** `ui/lang/desktop/*.json` (16 files).

- [ ] **Step 1: Write `$SCRATCH/infra-i18n-b.json`** with all 16 locales. Translation rules are the same as Task 2 Step 2. Keep shortcut letters such as `(V)` unchanged.

| key | en | de |
|---|---|---|
| `desktop.infra_map_label` | Infrastructure map | Infrastrukturkarte |
| `desktop.infra_map_role` | map | Karte |
| `desktop.infra_map_keys` | Arrow keys move between assets. Enter opens details, Space selects, M moves the selection. | Pfeiltasten wechseln zwischen Assets. Eingabe öffnet Details, Leertaste wählt aus, M verschiebt die Auswahl. |
| `desktop.infra_tools` | Map tools | Kartenwerkzeuge |
| `desktop.infra_tool_select` | Select (V) | Auswählen (V) |
| `desktop.infra_tool_hand` | Pan the map (H) | Karte verschieben (H) |
| `desktop.infra_tool_connect` | Connect assets (C) | Assets verbinden (C) |
| `desktop.infra_zoom_fit` | Fit to window (F) | An Fenster anpassen (F) |
| `desktop.infra_minimap` | Overview map | Übersichtskarte |
| `desktop.infra_empty_title` | Start your homelab map | Starte deine Homelab-Karte |
| `desktop.infra_empty_step_sources` | Connect sources such as Docker, Proxmox or a FRITZ!Box in Desktop settings. | Verbinde Quellen wie Docker, Proxmox oder eine FRITZ!Box in den Desktop-Einstellungen. |
| `desktop.infra_empty_step_import` | Import the discoveries you want to manage. | Importiere die Funde, die du verwalten möchtest. |
| `desktop.infra_empty_step_drag` | Drag assets from the library onto the map. | Ziehe Assets aus der Bibliothek auf die Karte. |
| `desktop.infra_new_asset` | New asset | Neues Asset |
| `desktop.infra_import_discoveries` | Import discoveries… | Funde importieren… |
| `desktop.infra_no_match` | No asset matches | Kein Asset passt |
| `desktop.infra_clear_search` | Clear search | Suche löschen |
| `desktop.infra_retry` | Retry | Erneut versuchen |
| `desktop.infra_guests_summary` | {count} · {running} running | {count} · {running} laufen |
| `desktop.infra_guest_count` | {count} guests | {count} Gäste |
| `desktop.infra_show_guests` | Show guests of {name} | Gäste von {name} anzeigen |
| `desktop.infra_hide_guests` | Hide guests of {name} | Gäste von {name} ausblenden |
| `desktop.infra_guests_of` | Guests of {name} | Gäste von {name} |
| `desktop.infra_more_guests` | +{count} more | +{count} weitere |
| `desktop.infra_drop_into` | Drop into {name} | In {name} ablegen |
| `desktop.infra_drop_on_map` | Place on the map | Auf der Karte ablegen |
| `desktop.infra_region_label` | {name}, {kind}, {count} assets | {name}, {kind}, {count} Assets |

- [ ] **Step 2: Run** `node "$SCRATCH/infra-i18n-add.mjs" . "$SCRATCH/infra-i18n-b.json"` from the worktree root. Expected: `+27` per locale.
- [ ] **Step 3: Verify:** V-core.
- [ ] **Step 4: Commit.** Paths: `ui/lang/desktop/*.json`. Subject: `i18n(infra): add canvas, empty state and guest keys`.

---

## Task 5: Canvas module and canvas stylesheet

**Files:**
- Create: `ui/js/desktop/apps/infra-canvas.js`
- Create: `ui/css/desktop-app-infra-canvas.css`
- Modify: `scripts/test-infra-ui.mjs`: insert `'infra-canvas.js'` after `'infra-store.js'` in the file list, and add the CSS guard.

- [ ] **Step 1: Build the canvas DOM** in `create(app)`, which returns the API in §1.4. `create` builds this markup once. `${S}` is `app.safeId`; every text comes from a literal `t()`.

```html
<section class="infra-canvas" data-canvas aria-label="{t desktop.infra_map_label}">
  <div class="infra-viewport" data-stage tabindex="0" role="application" data-tool="select"
       aria-roledescription="{t desktop.infra_map_role}" aria-label="{t desktop.infra_map_label}" aria-describedby="infra-canvas-help-${S}">
    <div class="infra-canvas-grid" aria-hidden="true"></div>
    <div class="infra-scene" data-scene>
      <div class="infra-regions" data-regions></div>
      <svg class="infra-edges" data-edges width="1" height="1" viewBox="0 0 1 1" overflow="visible" aria-hidden="true" focusable="false">
        <g data-edge-layer></g><path class="infra-edge-preview" data-edge-preview d="" hidden></path>
      </svg>
      <div class="infra-nodes" data-nodes></div>
    </div>
    <div class="infra-marquee" data-marquee hidden></div>
  </div>
  <div class="infra-canvas-empty" data-empty hidden>
    <img src="{assetURL('empty-atlas')}" alt="" draggable="false">
    <h2>{t desktop.infra_empty_title}</h2>
    <ol><li>{t desktop.infra_empty_step_sources}</li><li>{t desktop.infra_empty_step_import}</li><li>{t desktop.infra_empty_step_drag}</li></ol>
    <div class="infra-empty-actions">
      <button type="button" class="infra-button infra-button-primary" data-action="import-empty" disabled>{t desktop.infra_import_discoveries}</button>
      <button type="button" class="infra-button" data-action="add-empty" disabled>{t desktop.infra_new_asset}</button>
    </div>
  </div>
  <div class="infra-canvas-nomatch" data-no-match hidden><strong>{t desktop.infra_no_match}</strong>
    <button type="button" class="infra-button" data-action="clear-search">{t desktop.infra_clear_search}</button></div>
  <div class="infra-canvas-chip" data-layout-chip role="status" hidden>
    <span>{t desktop.infra_layout_unsaved}</span><button type="button" class="infra-button" data-action="retry-layout" disabled>{t desktop.infra_retry}</button></div>
  <div class="infra-tools" role="toolbar" aria-label="{t desktop.infra_tools}" data-tools>
    <button type="button" data-tool="select" aria-pressed="true" tabindex="0" aria-label="{t desktop.infra_tool_select}" title="…">{glyph select}</button>
    <button type="button" data-tool="hand" aria-pressed="false" tabindex="-1" aria-label="{t desktop.infra_tool_hand}" title="…">{glyph hand}</button>
    <button type="button" data-tool="connect" aria-pressed="false" tabindex="-1" aria-label="{t desktop.infra_tool_connect}" title="…">{glyph connect}</button>
    <span class="infra-tools-sep" aria-hidden="true"></span>
    <button type="button" data-action="zoom-out" tabindex="-1" aria-label="{t desktop.infra_zoom_out}" title="…">{glyph minus}</button>
    <button type="button" class="infra-zoom-value" data-action="zoom-reset" tabindex="-1" title="{t desktop.infra_zoom_reset}"><span data-zoom>100%</span></button>
    <button type="button" data-action="zoom-in" tabindex="-1" aria-label="{t desktop.infra_zoom_in}" title="…">{glyph plus}</button>
    <button type="button" data-action="fit" tabindex="-1" aria-label="{t desktop.infra_zoom_fit}" title="…">{glyph fit}</button>
  </div>
  <figure class="infra-minimap" data-minimap>
    <figcaption class="infra-minimap-caption"><span data-visible-count></span>
      <button type="button" class="infra-cmd-icon" data-action="toggle-minimap" aria-pressed="true" aria-label="{t desktop.infra_minimap}" title="…">{glyph minimap}</button></figcaption>
    <svg class="infra-minimap-map" data-minimap-map viewBox="0 0 200 130" role="img" aria-label="{t desktop.infra_minimap}">
      <g data-minimap-regions></g><g data-minimap-cards></g><rect class="infra-minimap-view" data-minimap-view rx="2"></rect></svg>
  </figure>
  <p class="infra-sr-only" id="infra-canvas-help-${S}">{t desktop.infra_map_keys}</p>
</section>
```

  - `.infra-zoom-value` gets `aria-label = t('desktop.infra_zoom_reset') + ' · ' + pct`, updated on every view change.
  - The tools toolbar uses a roving tabindex: ArrowLeft/ArrowRight/Home/End move focus. Clicking a `[data-tool]` calls `store.setTool`. The zoom buttons call `zoomBy(1/1.2)`, `zoomBy(1.2)`, `zoomTo(1)` (around the viewport centre) and `fit({animate:true})`.

- [ ] **Step 2: View management.**
  - **State:** `views = {atlas: null, topology: null}`; the current view is `views[app.mapView()]`.
  - **applyView** (rAF-coalesced via a local `frame(fn)` helper; port `core.frame` from `easydrag-core.js:153-159`) does all of the following:
    - `scene.style.transform = \`translate(${v.x}px, ${v.y}px) scale(${v.zoom})\``.
    - Grid `backgroundSize = \`${24*v.zoom}px ${24*v.zoom}px\``, `backgroundPosition = \`${v.x}px ${v.y}px\``.
    - Viewport class `is-zoom-low` when `zoom < 0.75`.
    - `[data-zoom]` text `Math.round(zoom*100)+'%'` and the zoom-value aria-label.
    - Minimap viewport rect.
    - Persists, debounced 400 ms, to `localStorage['infra.viewport.v1.'+view] = JSON.stringify(model.viewCenter(v, size()))`, inside try/catch.
  - **setView(view, {animate}):** coerce non-finite values to 0, then `zoom = model.clampZoom`. Add class `is-animating` to `[data-scene]` for 220 ms only when `animate && app.animationsEnabled()`.
  - **ensureInitialView()** runs on the first render with `size().w > 0` per view:
    - Restore `model.viewFromCenter(JSON.parse(stored), size())`.
    - If that is null, or no card intersects the visible area, call `setView(model.fitView(scene.bounds, size(), 48, 1))`.
  - **fit:** `fitView(bounds, size, 48, 1)`.
  - **zoomBy(f, point = centre):** `setView(model.zoomAt(v, v.zoom*f, point))`. `zoomTo(z)` is the same with an absolute zoom.
  - **centerOn(id):** centre the card or region at `max(zoom, 0.8)`.
  - **ensureVisible(id):** call `centerOn` only when the card rect is not fully inside the viewport.
  - **clientToWorld:** `model.toWorld(v, {x: clientX - rect.left, y: clientY - rect.top})`.
  - A `ResizeObserver` (if present) keeps the world centre stable on resize: remember `viewCenter` before the size change, then `viewFromCenter` after.

- [ ] **Step 3: Keyed rendering.** `renderNow()`:

```js
        function renderNow() {
            if (store.state.view === 'table' || !viewport.isConnected) return;
            const view = app.mapView();
            last = model.buildScene({ nodes: store.state.nodes, edges: store.state.edges, view,
                placementOf: id => store.effectivePlacement(id, view), parentOf: id => store.effectiveParent(id, view), guests: store.guests() });
            patchRegions(last.regions); patchCards(last.cards); patchLines(last.lines);
            paintStates(); paintOverlays(); ensureInitialView(); minimapFrame.request();
            viewport.classList.toggle('is-readonly', !store.canWrite());
        }
```

  **Cards** are keyed by id in `cardEls: Map<id, {card, badge, shelf, sig, x, y, flags}>`.
  - `cardMeta(node) = node.profile.ip_address || node.profile.role || app.labelForKind(node.kind)`.
  - The signature is:
    ```js
    [node.name, node.kind, model.iconURL(node.icon), model.sourceState(node), node.power||'', cardMeta(node), guestsSig, state.expandedHosts.has(id), store.canWrite()].join('\x1f')
    ```
  - `guestsSig` joins `id:name:power` for every guest.

  Patch algorithm:
  1. Walk the scene cards.
  2. A known id with an equal signature only gets `placeCard(entry, x, y)`. That sets `style.left/top` on the card, badge and shelf, and only when changed.
  3. A known id with a different signature: build the new markup in a `<template>` and `replaceWith` the card. Keep focus if the old card was `document.activeElement`. Replace, insert or remove the badge and shelf right after the card.
  4. Unknown ids are batched into **one** `<template>` built with `innerHTML` and indexed by iterating `template.content.children`: `data-node` gives the card, `data-toggle-host-services` the badge, `data-guest-shelf` the shelf. The batch is then appended to `[data-nodes]` in one call.
  5. Entries missing from the scene are removed.
  6. Never set `innerHTML` on `[data-nodes]` after the first batch.

  Card markup, with `safe = String(id).replace(/[^a-zA-Z0-9_-]/g,'')`:

```html
<button type="button" class="infra-card" data-node="{id}" data-kind="{kind}" data-state="{sourceState}" [data-power="{power}"] tabindex="-1" aria-pressed="false"
        aria-label="{name}, {kind label}, {state label}[, {power label}][, {fill(t desktop.infra_guest_count, count)}]" title="{name}">
  <span class="infra-card-icon">{app.icon(node.icon)}</span>
  <span class="infra-card-text"><strong class="infra-card-name">{name}</strong><small class="infra-card-meta">{cardMeta}</small></span>
  <span class="infra-card-status" aria-hidden="true">{●|◐|○|✕|'' for current|stale|unknown|removed|manual}</span>
  [<i class="infra-card-power" data-power="{power}" aria-hidden="true"></i>]
  [canWrite: <span class="infra-card-handle" data-connect-handle="n" aria-hidden="true"></span> … e, s, w]
</button>
[guests.length:
<button type="button" class="infra-card-badge" data-toggle-host-services="{id}" tabindex="-1" aria-expanded="{expanded}"
        aria-controls="infra-guests-${S}-{safe}" aria-label="{fill(t show_guests|hide_guests, {name})}">
  <span aria-hidden="true">▣</span> {fill(t desktop.infra_guests_summary, {count, running})}</button>
<div class="infra-guest-shelf" id="infra-guests-${S}-{safe}" data-guest-shelf="{id}" role="group" aria-label="{fill(t desktop.infra_guests_of,{name})}" [hidden unless expanded]>
  <button type="button" class="infra-guest-chip" data-hosted-service="{gid}" data-power="{power}" tabindex="-1" title="{guest name}"><i aria-hidden="true"></i><span>{guest name}</span></button> ×(first 8 by name)
  [more: <button type="button" class="infra-guest-more" data-guest-more="{id}" tabindex="-1">{fill(t desktop.infra_more_guests,{count})}</button>]
</div>]
```

  `running` counts guests with `power === 'running'`.

  **Regions** are keyed in `regionEls`.
  - Signature: name, icon URL, kind, direct child count from `store.childrenOf(id).length`, `store.canWrite() && store.features.layoutParent`, depth.
  - Each patch sets `style.left/top/width/height` from `rect`.
  - DOM order follows `last.regions` (shallow first, so nested regions paint above).

```html
<div class="infra-region" data-region="{id}" data-kind="{kind}" data-depth="{depth}">
  <button type="button" class="infra-region-header" data-region-header="{id}" tabindex="-1" aria-pressed="false"
          aria-label="{fill(t desktop.infra_region_label,{name, kind: kind label, count})}">
    <img class="infra-region-icon" src="{iconURL}" alt="" draggable="false"><span class="infra-region-name">{name}</span><span class="infra-region-count" aria-hidden="true">{count}</span></button>
  <span class="infra-region-drop" data-region-drop aria-hidden="true"></span>
  [canWrite && features.layoutParent: <span class="infra-region-handle" data-resize="{nw|n|ne|e|se|s|sw|w}" aria-hidden="true"></span> ×8]
</div>
```

  **Lines** are keyed `<g>` elements in `[data-edge-layer]`.
  - Create them by parsing markup inside `document.createElementNS(edgesSvg.namespaceURI, 'svg')` with `innerHTML`, then moving `firstElementChild`. Do not write the namespace string literally.
  - Signature: kind, caption (`edge.label || model.edgeCaption(edge)`), title. Each patch updates `d` on the hit, line and core paths and `x/y` on the label.
  - Keep `linesByEnd: Map<cardOrRegionId, Set<lineId>>` for temporary moves.

```html
<g class="infra-edge" data-edge="{id}" data-kind="{kind}">
  <path class="infra-edge-hit" d="M x1 y1 L x2 y2"/><path class="infra-edge-line" d="…"/>
  [fiber: <path class="infra-edge-line infra-edge-core" d="…"/>]
  [caption: <text class="infra-edge-label" x="{mid x}" y="{mid y}">{caption}</text>]
  <title>{edge kind label}{ · caption}</title>
</g>
```

  **paintStates()** never rebuilds. It compares cached flags and writes only on change.
  - Card `aria-pressed` = `state.selection.has(id)`.
  - `is-dimmed` = filters active and neither the node nor any of its guests `store.matches`.
  - `is-saving` / `is-unsaved` come from `store.layoutState(id, view)`.
  - Chips: `is-selected` and `is-dimmed`.
  - Region headers: `aria-pressed` plus the region class `is-selected`.
  - Lines: `is-selected` when the id is `state.selectedEdge`.

  **paintOverlays():**
  - `[data-visible-count]` = `t('desktop.infra_visible_count').replace('{count}', undimmedCards)`.
  - `[data-empty]` is visible when the scene has no cards and no regions. Its buttons are disabled when `!store.canWrite()`.
  - `[data-no-match]` is visible when filters are active and cards exist but none match.
  - `[data-layout-chip]` is visible when `store.unsavedCount(view) > 0`. `[data-action="retry-layout"]` is disabled when there is nothing unsaved, `!canWrite`, or `state.busy.has('layout')`.

  **Minimap:**
  - Redraw region and card rects only when a `revision` counter, bumped by renderNow when positions or ids change, differs from the drawn one.
  - Transform is `model.minimapTransform(unionRects([bounds, visibleWorldRect]), {w:200, h:130})`.
  - Selected cards get class `is-selected`.
  - The view rect updates on every applyView.
  - Pointer handling uses pointerdown/pointermove with capture on `[data-minimap-map]`: map the point to world and centre the view there. That gives click-to-jump and drag-to-pan.
  - `toggle-minimap` flips `aria-pressed`, hides the svg, and persists through the app prefs callback `app.setPref('minimap', bool)` (provided by infra.js).

- [ ] **Step 4: Temporary positions and highlights.**
  - `moveTemp(cards, regions)`:
    - Sets `left/top` on the card, badge and shelf for each id in `cards: Map<id,{x,y}>`.
    - Sets `left/top/width/height` for each region rect in `regions: Map<id, rect>`.
    - Recomputes every line in `linesByEnd` of those ids with `model.edgeSegment`, using temporary or scene geometry.
    - Does not touch `last`.
  - `clearTemp()`: re-apply positions from `last`.
  - `setDropTarget(id)`: class `is-drop-target` on that region and text `fill(t('desktop.infra_drop_into'), {name})` in its `[data-region-drop]`. Passing `''` clears it.
  - `setConnectTarget(id)`: class `is-connect-target`.
  - `showEdgePreview(fromId, point)`: path from the border point of the from-card toward `point` (world). `null` hides it.
  - `showMarquee(rect)`: rect in viewport-relative screen px. `null` hides it.
  - `setRovingCard(id)`: that card gets `tabindex=0` and the viewport `-1`. With no id the viewport gets `0`.
  - `createGhost(ids)`:
    - Appends `<div class="infra-drag-ghost" aria-hidden="true">` to `app.root`. Content: first icon and name, plus `+N` when more than one id.
    - `move(clientX, clientY, {scale, label, invalid})` sets `transform: translate(${clientX - appRect.left + 8}px, ${clientY - appRect.top + 8}px) scale(${scale})`, the `data-drop-label` attribute and the `is-invalid` class.
    - `remove()` detaches it.

- [ ] **Step 5: Write `desktop-app-infra-canvas.css`.** Tokens only. No hex outside `color-mix()`. No `rgb(a)(<digit>` literals. Each rule is complete as listed; sizes are exact.

```css
.infra-canvas { position: relative; grid-column: 3; min-width: 0; min-height: 200px; overflow: hidden; background: var(--vd-theme-app-bg); }
.infra-viewport { position: absolute; inset: 0; overflow: hidden; touch-action: none; outline: none; cursor: default; }
.infra-viewport:focus-visible { box-shadow: inset 0 0 0 var(--ds-focus-ring-width) var(--ds-color-border-focus); }
.infra-viewport[data-tool="hand"], .infra-viewport.is-panning { cursor: grab; }
.infra-viewport.is-panning:active { cursor: grabbing; }
.infra-canvas-grid { position: absolute; inset: 0; pointer-events: none; background-image: radial-gradient(circle, color-mix(in srgb, var(--vd-theme-border-strong) 80%, transparent) 1px, transparent 1.3px); background-size: 24px 24px; }
.infra-scene, .infra-regions, .infra-nodes { position: absolute; left: 0; top: 0; width: 0; height: 0; }
.infra-scene { transform-origin: 0 0; will-change: transform; }
.infra-edges { position: absolute; left: 0; top: 0; overflow: visible; pointer-events: none; }
.infra-edge-hit { fill: none; stroke: transparent; stroke-width: 14px; pointer-events: stroke; cursor: pointer; vector-effect: non-scaling-stroke; }
.infra-edge-line { fill: none; stroke: color-mix(in srgb, var(--vd-theme-muted) 70%, var(--vd-text)); stroke-width: 2px; vector-effect: non-scaling-stroke; pointer-events: none; }
.infra-edge[data-kind="wifi"] .infra-edge-line { stroke-dasharray: 6 5; }
.infra-edge[data-kind="fiber"] .infra-edge-line { stroke: var(--ds-color-info-fg); stroke-width: 5px; }
.infra-edge[data-kind="fiber"] .infra-edge-core { stroke: var(--vd-theme-app-bg); stroke-width: 1.6px; }
.infra-edge[data-kind="depends_on"] .infra-edge-line, .infra-edge[data-kind="member_of"] .infra-edge-line { stroke-dasharray: 1.5 4; stroke-linecap: round; }
.infra-edge[data-kind="powered_by"] .infra-edge-line { stroke: var(--ds-color-warning-fg); }
.infra-edge:hover .infra-edge-line, .infra-edge.is-selected .infra-edge-line { stroke: var(--vd-accent); stroke-width: 3px; }
.infra-edge-label { font: 600 12px/1 var(--vd-font-ui); fill: var(--vd-theme-muted); paint-order: stroke; stroke: var(--vd-theme-app-bg); stroke-width: 4px; text-anchor: middle; dominant-baseline: middle; pointer-events: none; }
.infra-viewport.is-zoom-low .infra-edge-label { display: none; }
.infra-viewport.is-zoom-low .infra-edge:hover .infra-edge-label { display: inline; }
.infra-edge-preview { fill: none; stroke: var(--vd-accent); stroke-width: 2px; stroke-dasharray: 4 4; vector-effect: non-scaling-stroke; }
.infra-card { position: absolute; z-index: 1; width: 184px; height: 60px; transform: translate(-50%, -50%); display: grid; grid-template-columns: 36px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 0 12px 0 10px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg); color: var(--vd-text); box-shadow: var(--vd-theme-shadow-panel); font: inherit; text-align: left; cursor: default; --infra-state: transparent; }
.infra-card[data-state="current"] { --infra-state: var(--ds-color-success-fg); }
.infra-card[data-state="stale"] { --infra-state: var(--ds-color-warning-fg); }
.infra-card[data-state="removed"] { --infra-state: var(--ds-color-danger-fg); }
.infra-card[data-state="unknown"] { --infra-state: var(--vd-theme-muted); }
.infra-card-icon { width: 36px; height: 36px; display: grid; place-items: center; border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); box-shadow: 0 0 0 2px var(--infra-state); }
.infra-card-icon img { width: 32px; height: 32px; object-fit: contain; pointer-events: none; }
.infra-card-text { display: grid; min-width: 0; }
.infra-card-name { overflow: hidden; font-size: 16px; font-weight: 600; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; }
.infra-card-meta { overflow: hidden; color: var(--vd-theme-muted); font-size: 14px; line-height: 1.3; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.infra-card-status { align-self: start; margin-top: 8px; color: var(--infra-state); font-size: 12px; }
.infra-card-power { position: absolute; right: 8px; bottom: 8px; width: 8px; height: 8px; border-radius: 50%; background: var(--vd-theme-muted); }
.infra-card-power[data-power="running"], .infra-guest-chip[data-power="running"] i { background: var(--ds-color-success-fg); }
.infra-card-power[data-power="stopped"], .infra-guest-chip[data-power="stopped"] i { background: var(--ds-color-danger-fg); }
.infra-card-power[data-power="paused"], .infra-guest-chip[data-power="paused"] i { background: var(--ds-color-warning-fg); }
.infra-card:hover { border-color: var(--vd-theme-border-strong); transform: translate(-50%, calc(-50% - 1px)); }
.infra-card[aria-pressed="true"] { z-index: 2; outline: 2px solid var(--vd-accent); outline-offset: 1px; box-shadow: 0 0 0 6px var(--vd-theme-accent-soft), var(--vd-theme-shadow-panel); }
.infra-card:focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset); }
.infra-card.is-dimmed, .infra-guest-chip.is-dimmed { opacity: .28; }
.infra-card.is-saving { border-style: dashed; }
.infra-card.is-unsaved { border: 1px dashed var(--ds-color-warning-fg); }
.infra-card.is-connect-target { outline: 2px dashed var(--vd-accent); outline-offset: 3px; }
.infra-card-handle { position: absolute; width: 10px; height: 10px; border: 2px solid var(--vd-theme-panel-bg); border-radius: 50%; background: var(--vd-accent); opacity: 0; cursor: crosshair; }
.infra-card-handle[data-connect-handle="n"] { top: -6px; left: calc(50% - 5px); }
.infra-card-handle[data-connect-handle="s"] { bottom: -6px; left: calc(50% - 5px); }
.infra-card-handle[data-connect-handle="e"] { right: -6px; top: calc(50% - 5px); }
.infra-card-handle[data-connect-handle="w"] { left: -6px; top: calc(50% - 5px); }
.infra-card:hover .infra-card-handle, .infra-card[aria-pressed="true"] .infra-card-handle, .infra-viewport[data-tool="connect"] .infra-card-handle { opacity: 1; }
.infra-card-badge { position: absolute; z-index: 3; transform: translate(-50%, 20px); height: 22px; padding: 0 8px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-panel-bg-strong); color: var(--vd-text); font: 600 13px/20px var(--vd-font-ui); font-variant-numeric: tabular-nums; white-space: nowrap; cursor: pointer; }
.infra-card-badge[aria-expanded="true"] { border-color: var(--vd-theme-accent-border); background: var(--vd-theme-accent-soft); }
.infra-guest-shelf { position: absolute; z-index: 3; transform: translate(-50%, 48px); display: flex; flex-wrap: wrap; gap: 4px; width: max-content; max-width: 280px; padding: 6px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); }
.infra-guest-chip, .infra-guest-more { display: inline-flex; align-items: center; gap: 6px; max-width: 100%; min-height: 28px; padding: 0 8px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); color: var(--vd-text); font-size: 14px; cursor: pointer; }
.infra-guest-chip i { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: var(--vd-theme-muted); }
.infra-guest-chip span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.infra-guest-chip.is-selected { border-color: var(--vd-accent); background: var(--vd-theme-accent-soft); }
.infra-region { position: absolute; border: 1.5px solid color-mix(in srgb, var(--vd-accent) 35%, var(--vd-theme-border)); border-radius: var(--vd-theme-radius-panel); background: color-mix(in srgb, var(--vd-accent) 5%, transparent); pointer-events: none; }
.infra-region[data-depth="1"], .infra-region[data-depth="2"] { background: color-mix(in srgb, var(--vd-accent) 8%, transparent); }
.infra-region-header { position: absolute; left: 8px; top: 8px; max-width: calc(100% - 16px); height: 32px; display: inline-flex; align-items: center; gap: 8px; padding: 0 10px 0 6px; border: 0; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-text); font: 600 16px/1 var(--vd-font-ui); pointer-events: auto; cursor: grab; }
.infra-region-header:hover { background: var(--vd-theme-control-hover); }
.infra-region-icon { width: 22px; height: 22px; object-fit: contain; }
.infra-region-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.infra-region-count { color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; }
.infra-region.is-selected { border-color: var(--vd-accent); box-shadow: 0 0 0 4px var(--vd-theme-accent-soft); }
.infra-region.is-drop-target { border-style: dashed; border-color: var(--vd-accent); background: var(--vd-theme-accent-soft); }
.infra-region-drop { display: none; position: absolute; left: 50%; bottom: 12px; transform: translateX(-50%); padding: 2px 10px; border-radius: var(--ds-radius-full); background: var(--vd-accent); color: var(--vd-accent-contrast); font: 600 14px/1.6 var(--vd-font-ui); white-space: nowrap; }
.infra-region.is-drop-target .infra-region-drop { display: block; }
.infra-region-handle { position: absolute; width: 14px; height: 14px; pointer-events: auto; opacity: 0; }
.infra-region:hover .infra-region-handle, .infra-region.is-selected .infra-region-handle { opacity: 1; }
/* nw/ne/sw/se: corners at -7px with cursor nwse/nesw-resize and a 2px accent border on the two outer sides;
   n/s: left 14px, right 14px, width auto, top/bottom -7px, cursor ns-resize; e/w: top 14px, bottom 14px, height auto, cursor ew-resize. */
.infra-marquee { position: absolute; left: 0; top: 0; border: 1px solid var(--vd-accent); background: var(--vd-theme-accent-soft); pointer-events: none; }
.infra-tools { position: absolute; left: var(--ds-space-3); bottom: var(--ds-space-3); z-index: 6; display: flex; align-items: center; gap: 2px; padding: 4px; border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); }
.infra-tools button { width: 36px; height: 36px; display: grid; place-items: center; border: 0; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-text); cursor: pointer; }
.infra-tools button:hover { background: var(--vd-theme-control-hover); }
.infra-tools button[aria-pressed="true"] { background: var(--vd-theme-accent-soft); box-shadow: inset 0 0 0 1px var(--vd-theme-accent-border); }
.infra-tools button:disabled { opacity: .45; cursor: not-allowed; }
.infra-tools .infra-zoom-value { width: auto; min-width: 56px; padding: 0 6px; font: 600 14px/1 var(--vd-font-ui); font-variant-numeric: tabular-nums; }
.infra-tools-sep { width: 1px; height: 24px; margin: 0 4px; background: var(--vd-theme-border); }
.infra-minimap { position: absolute; right: var(--ds-space-3); bottom: var(--ds-space-3); z-index: 6; margin: 0; padding: 6px; border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); }
.infra-minimap-caption { display: flex; align-items: center; justify-content: space-between; gap: var(--ds-space-2); color: var(--vd-theme-muted); font-size: 14px; }
.infra-minimap-map { display: block; width: 200px; height: 130px; margin-top: 4px; cursor: pointer; touch-action: none; }
.infra-minimap-map .region { fill: none; stroke: var(--vd-theme-accent-border); }
.infra-minimap-map .card { fill: var(--vd-theme-muted); }
.infra-minimap-map .card.is-selected { fill: var(--vd-accent); }
.infra-minimap-view { fill: var(--vd-theme-accent-soft); stroke: var(--vd-accent); stroke-width: 1.5px; vector-effect: non-scaling-stroke; }
.infra-canvas-chip { position: absolute; top: var(--ds-space-3); left: 50%; z-index: 7; transform: translateX(-50%); display: flex; align-items: center; gap: var(--ds-space-2); padding: 4px 4px 4px var(--ds-space-3); border: 1px solid color-mix(in srgb, var(--ds-color-warning-fg) 50%, var(--vd-theme-border)); border-radius: var(--ds-radius-full); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); font-size: 14px; }
.infra-canvas-empty, .infra-canvas-nomatch { position: absolute; inset: 0; z-index: 5; display: grid; place-content: center; justify-items: center; gap: var(--ds-space-3); padding: var(--ds-space-6); text-align: center; pointer-events: none; }
.infra-canvas-empty img { width: 180px; height: 130px; object-fit: contain; }
.infra-canvas-empty h2 { margin: 0; font-size: 20px; font-weight: 650; }
.infra-canvas-empty ol { display: grid; gap: var(--ds-space-1); max-width: 420px; margin: 0; padding-left: 1.4em; color: var(--vd-theme-muted); font-size: 14px; text-align: left; }
.infra-empty-actions, .infra-canvas-nomatch button { display: flex; gap: var(--ds-space-2); pointer-events: auto; }
.infra-drag-ghost { position: absolute; left: 0; top: 0; z-index: 40; display: grid; grid-template-columns: 28px minmax(0, 1fr); align-items: center; gap: 8px; width: 184px; height: 60px; padding: 0 10px; border: 1px solid var(--vd-accent); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-modal); opacity: .92; transform-origin: 0 0; pointer-events: none; }
.infra-drag-ghost[data-drop-label]:not([data-drop-label=""])::after { content: attr(data-drop-label); position: absolute; left: 0; top: calc(100% + 6px); padding: 2px 8px; border-radius: var(--ds-radius-full); background: var(--vd-accent); color: var(--vd-accent-contrast); font: 600 13px/1.6 var(--vd-font-ui); white-space: nowrap; }
.infra-drag-ghost.is-invalid { border-color: var(--ds-color-danger-fg); }
.infra-drag-ghost.is-invalid::after { background: var(--ds-color-danger-fg); }
.infra-app.is-dragging, .infra-app.is-dragging * { cursor: grabbing !important; }
.infra-app.is-drag-invalid, .infra-app.is-drag-invalid * { cursor: not-allowed !important; }
@media (prefers-reduced-motion: no-preference) {
  body:not([data-animations="false"]) .infra-card { transition: box-shadow var(--ds-motion-duration-fast) var(--ds-motion-ease-standard), border-color var(--ds-motion-duration-fast) var(--ds-motion-ease-standard), opacity var(--ds-motion-duration-fast) var(--ds-motion-ease-standard); }
  body:not([data-animations="false"]) .infra-scene.is-animating { transition: transform var(--ds-motion-duration-base) var(--ds-motion-ease-decel); }
}
@media (pointer: coarse) { .infra-card-handle { width: 16px; height: 16px; } .infra-tools button { width: 44px; height: 44px; } }
@container infra (max-width: 759px) { .infra-minimap-map { display: none; } .infra-tools [data-tool="hand"] { display: none; } }
```

  The connection popover styles are added in Task 7 to this same file.

- [ ] **Step 6: Add the CSS token guard** to `scripts/test-infra-ui.mjs` before the final log. Task 12 adds `desktop-app-infra.css` to the list.

```js
for (const file of ['desktop-app-infra-canvas.css']) {
    const css = fs.readFileSync(path.join(root, 'ui/css', file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    const outsideMixes = css.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, '');
    assert.deepEqual(outsideMixes.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], `${file} uses hex colours outside color-mix()`);
    assert.deepEqual(outsideMixes.match(/rgba?\(\s*\d/g) || [], [], `${file} uses literal rgb() colours instead of tokens`);
    assert.ok(!/var\(--vd-font[,)]/.test(css), `${file} uses --vd-font instead of --vd-font-ui`);
    assert.ok(!/copper/i.test(css), `${file} still uses the copper accent`);
}
```

- [ ] **Step 7: Verify.** Run `node --check ui/js/desktop/apps/infra-canvas.js`, then V-core. The canvas is not loaded by the app yet; its first visual check is in Task 12.
- [ ] **Step 8: Commit.**
  - Paths: `ui/js/desktop/apps/infra-canvas.js`, `ui/css/desktop-app-infra-canvas.css`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add keyed canvas renderer with regions, host stacks, edges and minimap`.

---

## Task 6: i18n batch C (panels)

**Files:** `ui/lang/desktop/*.json`.

- [ ] **Step 1: Write `$SCRATCH/infra-i18n-c.json`** with all 16 locales. Rules as in Task 2.

| key | en | de |
|---|---|---|
| `desktop.infra_new_place` | New place | Neuer Ort |
| `desktop.infra_place_on_map` | Place on map | Auf Karte platzieren |
| `desktop.infra_move_hint` | Select it on the map and press M to move it with the arrow keys. | Wähle es auf der Karte aus und drücke M, um es mit den Pfeiltasten zu verschieben. |
| `desktop.infra_connection` | Connection | Verbindung |
| `desktop.infra_connection_new` | New connection | Neue Verbindung |
| `desktop.infra_connection_kind` | Connection type | Verbindungsart |
| `desktop.infra_connection_create` | Create connection | Verbindung erstellen |
| `desktop.infra_connection_created` | Connection created | Verbindung erstellt |
| `desktop.infra_connection_removed` | Connection removed | Verbindung entfernt |
| `desktop.infra_edit_connection` | Edit connection | Verbindung bearbeiten |
| `desktop.infra_remove_connection` | Remove connection | Verbindung entfernen |
| `desktop.infra_remove_selection_confirm` | Remove {count} assets from the Infra inventory? Their external sources will not be changed. | {count} Assets aus dem Infra-Inventar entfernen? Ihre externen Quellen bleiben unverändert. |

- [ ] **Step 2: Run** the script with `infra-i18n-c.json`. Expected: `+12` per locale.
- [ ] **Step 3: Verify:** V-core.
- [ ] **Step 4: Commit.** Paths: `ui/lang/desktop/*.json`. Subject: `i18n(infra): add connection, placement and selection keys`.

---

## Task 7: Inspector, table and dialogs modules (ported) plus the connection popover

**Files:**
- Create: `ui/js/desktop/apps/infra-inspector.js`
- Create: `ui/js/desktop/apps/infra-table.js`
- Create: `ui/js/desktop/apps/infra-dialogs.js`
- Modify: `ui/css/desktop-app-infra-canvas.css` (popover rules)
- Modify: `scripts/test-infra-ui.mjs`: file list `['infra-model.js', 'infra-store.js', 'infra-canvas.js', 'infra-inspector.js', 'infra-table.js', 'infra-dialogs.js', 'infra-views.js', 'infra.js']`

These files are new copies; the old `infra-views.js`/`infra.js` stay untouched until Task 12. Inside each `create(app)`, start with `const { store, state, t, esc, model, q, root, ctx } = app;`.

Ported code needs these name substitutions:
- `nodeById` → `store.nodeById`
- `canWrite/canEditSource/allowedAction/actionReason/actionBlockKey` → the `store.` versions
- `request` → `store.request`
- `messageFor/uncertainMutation` → the `store.` versions
- `setNotice(msg, type)` → `store.notify(msg, type)`, with `'busy'` notices dropped
- `setBusy` → `store.setBusy`
- `loadSnapshot` → `store.loadSnapshot`
- `replaceNode` → `store.replaceNode`
- `groupChildren` → `store.childrenOf`
- `icon` → `app.icon`, `labelForKind` → `app.labelForKind`, `statusLabel` → `app.statusLabel`
- `paintAll()` / `paintStage()` / `paintTable()` / `paintGroups()` / `paintTray()` → nothing (the store events trigger repaints)
- `selectNode(id)` → `store.select([id])`
- `paintInspector()` → `paint()` inside the inspector, `app.inspector.paint()` elsewhere

- [ ] **Step 1: Write `infra-inspector.js`.**
  - Port from `infra-views.js`: lines 168–414 (`profileFields` through `readProfileLists`) and 415–457 (`saveNode`).
  - Port from `infra.js`: lines 690–781 (source-edit flows) and the list add/remove branch of `bindEvents` (877–905).

  `paint()` keeps today's markup, including every pinned hook, `.infra-draft-banner`, the conflict reviews, `.infra-source-row` and `.infra-source-edit`. Changes:
  - **Visibility:**
    - `const node = store.nodeById(state.selected)` decides the panel.
    - With a single node, paint the node inspector.
    - With `state.selectedEdge`, paint the connection inspector (below).
    - With `state.selection.size > 1`, paint the selection summary (below).
    - Otherwise set `panel.hidden = true`, `panel.innerHTML = ''` and `root.dataset.inspectorOpen = 'false'`, then return. Every non-empty branch sets `panel.hidden = false` and `root.dataset.inspectorOpen = 'true'`.
    - There is no empty-state markup any more.
  - **Header:**
    - `<p class="infra-kicker">` becomes `<p class="infra-detail-kind">`, with the same text.
    - `[data-action="close-inspector"]` stays and is visible at every width.
    - `[data-action="node-more"]` calls `app.dialogs.openNodeActions(node.id)`.
  - **Placement section:**
    - Replace the three `[data-place-choice]` buttons and the keyboard hint.
    - When the node has no placement in `app.mapView()`, is not a guest (`!store.guests().hostOf.has(id)`) and `store.canWrite()`: render `<button type="button" class="infra-button" data-action="place-selected">{t desktop.infra_place_on_map}</button>`. Its handler is `app.gestures.placeAtCenter([node.id], { moveMode: true })`.
    - Always render `<p class="infra-hint">{t desktop.infra_move_hint}</p>`.
    - The position label stays `t('desktop.infra_position')`, or `t('desktop.infra_unplaced')` when unplaced.
  - **Hosted services:**
    - Replace the toggle section with a plain list of guests from `store.guests().guestsOf.get(node.id)`:
      ```html
      <section class="infra-hosted-services"><div class="infra-form-heading"><h3>{t desktop.infra_hosted_services} <span>{n}</span></h3></div><div class="infra-component-list">{per guest: <button type="button" data-inspector-guest="{id}">{icon}<span>{name}</span><small>{kind label}</small></button>}</div></section>
      ```
    - Clicking selects the guest.
    - Remove `[data-toggle-host-services]` from the inspector; the card badge is the only toggle.
  - **Relations:**
    - Keep `[data-action="add-edge"]`, which calls `app.dialogs.openEdgeDialog('', node.id)`.
    - Keep `.infra-relation-row` and `[data-edit-edge]`.
  - **Delete:** `[data-action="delete-node"]` calls `app.dialogs.deleteNode(node.id)`.
  - **Source actions:** `[data-source-action]` calls `app.dialogs.openActionMenu(node.id, source)`.
  - **Connection inspector,** when `state.selectedEdge` is set (`from`/`to` are node names, or ids if unknown):
    ```html
    <div class="infra-detail-head"><div class="infra-detail-icon">{app.glyph('connect')}</div><div><p class="infra-detail-kind">{t desktop.infra_connection} · {t(`desktop.infra_edge_${edge.kind}`)}</p><h2>{from} → {to}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="{t desktop.close}" title="…">×</button></div>
    <div class="infra-form-section"><dl class="infra-facts"><dt>{t desktop.infra_relation_from_port}</dt><dd>{from_port||—}</dd><dt>{t desktop.infra_relation_to_port}</dt><dd>{to_port||—}</dd><dt>{t desktop.infra_relation_speed}</dt><dd>{speed||—}</dd><dt>{t desktop.infra_relation_label}</dt><dd>{label||—}</dd></dl>
    {canWrite && !edge.source ? <div class="infra-form-actions"><button type="button" class="infra-button" data-action="edit-edge">{t desktop.infra_edit_connection}</button><button type="button" class="infra-button infra-danger-button" data-action="delete-edge">{t desktop.infra_remove_connection}</button></div> : ''}</div>
    ```
    - `edit-edge` calls `app.dialogs.openEdgeDialog(edge.id)`.
    - `delete-edge` asks `app.dialogs.confirmAction(t('desktop.infra_remove_connection'), t('desktop.infra_remove_relation_confirm'))`. On confirm it calls `store.dispatch(InfraAppStore.commands.deleteEdge(edge))` and announces `t('desktop.infra_connection_removed')`.
  - **Selection summary,** when more than one node is selected:
    - Header h2: `t('desktop.infra_selected_count').replace('{count}', n)`.
    - A `<ul class="infra-selection-kinds">` with one `<li>{kind label} <b>{count}</b></li>` per kind.
    - When canWrite, `[data-action="arrange-selection"]` calls `app.gestures.arrange()` and `[data-action="delete-selection"]` calls `app.dialogs.deleteNodes([...state.selection])`.
  - **Close:**
    - `close({restoreFocus = true})` remembers the closed id and calls `store.clearSelection()`.
    - It then focuses the previously selected `[data-node]` via `app.canvas.cardEl(id)`, or `[data-row]` in table view.
  - **Ported busy handling:**
    - `onBusy(key)` carries the per-key DOM updates of the old `setBusy`, `infra.js` lines 276–288:
      - source save button
      - `[data-detail-form] button[type="submit"]`
      - `[data-edge-form]` submit
    - Dialog-related keys are handled in dialogs.
  - **Stale repaint:**
    - `paintIfIdle()` ports `inspectorBusy()` from `infra.js` lines 986–991. If the inspector is busy, it sets `stale = true`; otherwise it paints.
    - `flushStale()` ports lines 1011–1017; the root calls it on `focusout`.
  - `saveDraft()` calls `q('[data-detail-form]')?.requestSubmit()` when `state.dirty.has(state.selected)`.
  - `focusName()` focuses `[data-node-field="name"]`.
  - **Listeners:**
    - `create` binds one delegated `click` listener on the panel element for `[data-list-add]` and `[data-list-remove]`. This is the ported branch, followed by `paint()` and re-focus.
    - The same listener handles `place-selected`, `arrange-selection`, `delete-selection`, `edit-edge`, `delete-edge`, `reload-targets` (calls `store.loadTargets(true)`) and `[data-inspector-guest]`.
    - Per-paint listeners stay exactly as today, ported from infra-views lines 312–360.

- [ ] **Step 2: Write `infra-table.js`.**
  - Port `paintTable` from `infra-views.js` lines 120–132 into `render()`.
  - Items are `state.nodes.filter(store.matches).sort(name)`. Selection is `state.selection.has(id)`.
  - `[data-select]` click: `store.select([id])`, then focus `[data-row="id"]`.
  - `create` builds the old table section markup (old `infra.js` line 80) without the `infra-kicker` paragraph:
    ```html
    <section class="infra-table-view" data-table-view hidden aria-label="{t desktop.infra_inventory}"><div class="infra-table-head"><h2>{t desktop.infra_inventory}</h2><span data-table-total></span></div><div class="infra-table-scroll" data-table-scroll tabindex="0"><table>…same thead…</table></div></section>
    ```
  - It binds `scroll` (passive) to `render`.
  - Exports `el`, so infra.js inserts the section.

- [ ] **Step 3: Write `infra-dialogs.js`.** Port the following from `infra.js`:
  - `openDialog` (540–551): remove the `<p class="infra-kicker">…workspace…</p>` line.
  - `openNodeDialog`/`createNode` (552–575):
    - The signature becomes `openNodeDialog(kind = 'device', { title } = {})`. The title defaults to `t('desktop.infra_new_node')`; `infra.js` passes `t('desktop.infra_new_place')` for places.
    - On success: `store.replaceNode(node)`, then `store.select([node.id])` if the dialog is still owned, then `closeOwnedDialog`.
  - `loadCandidates`/`paintImportDialog`/`updateImportCount`/`importSelected`/`openImport` (354–363, 584–624, 858–875): unchanged logic, with `state.sources = result.sources || state.sources` kept.
  - `openEdgeDialog`/`saveEdge`/`deleteEdge` (625–664):
    - The signature is `openEdgeDialog(edgeId = '', fromId = state.selected, toId = '')`.
    - The create path dispatches `InfraAppStore.commands.createEdge(fields)` after a client duplicate pre-check. If an edge with the same `from`/`to`/`kind` already exists, notify `t('desktop.infra_error_edge_exists')` as an error and stop.
    - The edit path calls `store.updateEdge(id, body)`.
    - The delete path confirms, then dispatches `InfraAppStore.commands.deleteEdge(edge)`.
    - Errors notify `store.messageFor(error)`.
  - `confirmAction` (665–683): verbatim.
  - `deleteNode` (684–689): on success `store.removeNode(id)`.
  - New `deleteNodes(ids)`:
    - One confirmation: `t('desktop.infra_remove_selection_confirm').replace('{count}', ids.length)`.
    - Then sequential DELETEs, each calling `store.removeNode`.
    - Stop at the first error and notify it.
  - `openActionMenu`/`runAction` (782–836): verbatim, including `request_id`, re-arm and the uncertain handling.
  - `openNodeActions` (837–850): verbatim.
  - `onBusy(key)`: lines 285–289 of the old `setBusy` (`create`, `import`, `edge-*`, `action-*`).
  - Do not port `chooseEdgeEndpoint` or `openQuickActions`.

  **Connection popover** (new): `openConnectionPopover({ from, to, clientX, clientY })`.
  - It closes any previous popover and returns early when `!store.canWrite()` or `from === to`.
  - It appends this to `app.canvas.el`, positioned at the release point relative to `[data-canvas]` and clamped inside it:
    ```html
    <form class="infra-connect-popover" data-connect-popover role="dialog" aria-labelledby="infra-connect-title-${S}">
      <h3 id="infra-connect-title-${S}">{t desktop.infra_connection_new}</h3>
      <p class="infra-connect-ends">{from name} → {to name}</p>
      <label class="infra-field"><span>{t desktop.infra_connection_kind}</span><select name="kind" data-connect-kind>{EDGE_KINDS options with t(`desktop.infra_edge_${kind}`)}</select></label>
      <label class="infra-field"><span>{t desktop.infra_relation_label}</span><input name="label" maxlength="128" autocomplete="off"></label>
      <div class="infra-field-row"><label class="infra-field"><span>{t desktop.infra_relation_from_port}</span><input name="from_port" maxlength="64"></label><label class="infra-field"><span>{t desktop.infra_relation_to_port}</span><input name="to_port" maxlength="64"></label></div>
      <p class="infra-connect-error" data-connect-error role="alert" hidden></p>
      <div class="infra-form-actions"><button type="button" class="infra-button" data-action="cancel-connection">{t desktop.cancel}</button><button type="submit" class="infra-button infra-button-primary" data-action="create-connection">{t desktop.infra_connection_create}</button></div>
    </form>
    ```
  - `[data-connect-kind]` defaults to `model.defaultEdgeKind(fromNode, toNode)`. EDGE_KINDS is `['ethernet','fiber','wifi','member_of','runs_on','depends_on','powered_by']`.
  - Focus moves to the select.
  - **Submit:**
    - Prevent the default action.
    - Build fields with `from`, `to`, `kind` and the non-empty `label`/`from_port`/`to_port`.
    - Duplicate pre-check: show the inline error `t('desktop.infra_error_edge_exists')`.
    - Otherwise disable submit, then `await store.dispatch(InfraAppStore.commands.createEdge(fields))`.
    - On success, close, announce `t('desktop.infra_connection_created')`, and focus the from-card.
    - On error, show `store.messageFor(error)` inline, re-enable submit and keep the popover.
  - **Dismiss:** Esc, `cancel-connection` or a pointerdown outside (document capture listener, removed on close) closes it and returns focus to the from-card.
  - Add these rules to `desktop-app-infra-canvas.css`:
    ```css
    .infra-connect-popover { position: absolute; z-index: 30; width: 300px; display: grid; gap: var(--ds-space-2); padding: var(--ds-space-3); border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-modal); }
    .infra-connect-popover h3 { margin: 0; font-size: 15px; font-weight: 650; }
    .infra-connect-ends { margin: 0; color: var(--vd-theme-muted); font-size: 14px; }
    .infra-connect-error { margin: 0; color: var(--ds-color-danger-fg); font-size: 14px; }
    ```

- [ ] **Step 4: Verify.** Run `node --check` on the three files, then V-core. Expect the line budgets to hold: inspector about 650 lines, dialogs about 500, table about 60.
- [ ] **Step 5: Commit.**
  - Paths: the three JS files, `ui/css/desktop-app-infra-canvas.css`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): port inspector, table and dialogs into modules and add the connection popover`.

---

## Task 8: i18n batch D (library and gestures)

**Files:** `ui/lang/desktop/*.json`.

- [ ] **Step 1: Write `$SCRATCH/infra-i18n-d.json`** with all 16 locales. Rules as in Task 2.

| key | en | de |
|---|---|---|
| `desktop.infra_library` | Library | Bibliothek |
| `desktop.infra_to_place` | To place | Zu platzieren |
| `desktop.infra_on_map` | On the map | Auf der Karte |
| `desktop.infra_needs_host` | Needs a host | Braucht einen Host |
| `desktop.infra_everything_placed` | Everything is on the map | Alles ist auf der Karte |
| `desktop.infra_library_empty_map` | Nothing is on the map yet | Noch nichts auf der Karte |
| `desktop.infra_expand_library` | Show library | Bibliothek einblenden |
| `desktop.infra_collapse_library` | Collapse library | Bibliothek einklappen |
| `desktop.infra_move_mode` | Move mode: arrow keys move, Enter saves, Escape cancels. | Verschiebemodus: Pfeiltasten verschieben, Eingabe speichert, Escape bricht ab. |
| `desktop.infra_move_cancelled` | Move cancelled | Verschieben abgebrochen |
| `desktop.infra_moved` | {count} assets moved | {count} Assets verschoben |
| `desktop.infra_moved_into` | {name} moved into {place} | {name} nach {place} verschoben |
| `desktop.infra_moved_out` | {name} moved out of its place | {name} aus seinem Ort verschoben |
| `desktop.infra_placed` | {name} placed on the map | {name} auf der Karte platziert |
| `desktop.infra_placed_many` | {count} assets placed on the map | {count} Assets auf der Karte platziert |
| `desktop.infra_parent_unsupported` | This server cannot store places yet; only the position was saved. | Dieser Server kann noch keine Orte speichern; nur die Position wurde gespeichert. |
| `desktop.infra_arranged` | Assets arranged | Assets angeordnet |

- [ ] **Step 2: Run** the script with `infra-i18n-d.json`. Expected: `+17` per locale.
- [ ] **Step 3: Verify:** V-core.
- [ ] **Step 4: Commit.** Paths: `ui/lang/desktop/*.json`. Subject: `i18n(infra): add library and gesture keys`.

---

## Task 9: Library module

**Files:**
- Create: `ui/js/desktop/apps/infra-library.js`
- Modify: `scripts/test-infra-ui.mjs`: insert `'infra-library.js'` after `'infra-canvas.js'`.

- [ ] **Step 1: Build the library markup** in `create(app)`, once. `el` is the `<aside>`; the splitter is exposed as `library.splitter`.

```html
<aside class="infra-library" data-library id="infra-library-${S}" aria-label="{t desktop.infra_library}">
  <div class="infra-library-scroll" data-library-scroll>
    <section class="infra-library-section" data-library-section="to-place">
      <h2 class="infra-library-heading"><button type="button" class="infra-library-toggle" data-section-toggle="to-place" aria-expanded="true" aria-controls="infra-lib-to-place-${S}">
        <span class="infra-library-title">{t desktop.infra_to_place}</span><span class="infra-library-count" data-section-count="to-place">0</span></button></h2>
      <ul class="infra-library-list" id="infra-lib-to-place-${S}" data-section-list="to-place"></ul>
      <p class="infra-library-empty" data-section-empty="to-place" hidden>{t desktop.infra_everything_placed}</p>
    </section>
    <section … "on-map" title {t desktop.infra_on_map}, empty text {t desktop.infra_library_empty_map}>
    <section … "needs-host" title {t desktop.infra_needs_host}, hidden when empty, no empty text>
  </div>
  <div class="infra-library-rail" data-library-rail hidden><button type="button" class="infra-cmd-icon" data-action="expand-library" aria-label="{t desktop.infra_expand_library}" title="…">{glyph library}</button></div>
</aside>
<div class="infra-splitter" data-splitter="library" role="separator" aria-orientation="vertical" aria-controls="infra-library-${S}"
     aria-valuemin="220" aria-valuemax="400" aria-valuenow="264" aria-label="{t desktop.infra_resize_library}" tabindex="0"></div>
```

  Note: `desktop.infra_resize_library` arrives in batch E (Task 11). Run Task 11 before this file is first committed, or add this one key to batch D. **Do the latter:** move `desktop.infra_resize_library` and `desktop.infra_resize_inspector` from batch E into batch D before running Task 8. Task 8 then reports `+19`.

- [ ] **Step 2: Sections.** `computeSections()` uses `view = app.mapView()`, `g = store.guests()` and filters `store.matches`.
  - **toPlace:** nodes that meet all of these:
    - not a guest (`!g.hostOf.has(id)`);
    - not a hostless vm/container;
    - no placement in `view`;
    - not a place when `view === 'topology'`;
    - matching the filters.

    Sorted by name with `localeCompare`.
  - **needsHost:** `kind ∈ {vm, container}`, `!g.hostOf.has(id)`, no placement in `view`, matching. Sorted by name.
  - **onMap:** a tree from `app.canvas.sceneData()`:
    - Roots are the regions without a drawn parent, sorted by name.
    - Each region lists its child regions (effective parent) and its cards (`store.effectiveParent(card.id, view) === region.id`).
    - Then the cards without a drawn parent region.
    - A card with guests carries a nested guest list.
    - Filtering keeps a row when it matches or any descendant matches.
  - Counts are the number of rows in each section after filtering.

- [ ] **Step 3: Row markup.**
  - Rendering: `render()` rebuilds each section's `<ul>` with one `innerHTML` assignment, rAF-coalesced.
  - Preserved across a rebuild:
    - `scrollTop` of `[data-library-scroll]`;
    - the focused row, restored by `data-library-select` id;
    - the roving `tabindex`.
  - `renderSelection()` only toggles `aria-selected` on rows.

```html
<li class="infra-library-row" data-library-item="{id}" data-section="{to-place|on-map|needs-host}" [data-draggable="true" for to-place when canWrite] aria-selected="{selected}">
  <button type="button" class="infra-library-row-main" data-library-select="{id}" tabindex="{0 for the roving row, else -1}" title="{name}">
    <img class="infra-library-row-icon" src="{iconURL}" alt="" draggable="false">
    <span class="infra-library-row-text"><span class="infra-library-row-name">{name}</span><span class="infra-library-row-meta">{kind label}{ · ip}</span></span></button>
  [to-place: <button type="button" class="infra-library-place" data-place="{id}" tabindex="-1" aria-label="{t desktop.infra_place_asset with {name}}" title="{same or t desktop.infra_read_only}" {disabled when !canWrite}>{glyph plus}</button>]
  [on-map host with guests: <button type="button" class="infra-library-disclosure" data-library-expand="{id}" tabindex="-1" aria-expanded="{bool}" aria-label="{fill(t show_guests|hide_guests,{name})}">{glyph chevron}</button>]
  [children: <ul class="infra-library-children"> … nested rows … </ul>]
</li>
```

  Guest rows under a host are only rendered when the host is in `expandedLibraryHosts` (a local Set).

- [ ] **Step 4: Behaviour**, all delegated on `el`:
  - **click:**
    - Ignored when `suppressClick` is set; clear the flag.
    - `[data-section-toggle]` toggles `aria-expanded` and the `hidden` attribute of its list.
    - `[data-place]` calls `app.gestures.placeAtCenter([id], {moveMode: true})`.
    - `[data-library-expand]` toggles the guests.
    - `[data-library-select]`:
      - Shift or Ctrl/⌘ toggles the row in the selection (`store.select([id], {toggle: true})`).
      - Otherwise `store.select([id])`.
      - On-map rows also call `app.canvas.ensureVisible(id)`; guest rows reveal their host.
    - `[data-action="expand-library"]` calls `app.setLibraryCollapsed(false)`.
  - **pointerdown** on a `[data-draggable="true"]` row outside `[data-place]` with `button === 0`:
    - `ids` = the To place rows in the selection if this row is selected, otherwise `[id]`.
    - Call `app.gestures.startLibraryDrag(event, ids)`.
    - Gestures calls `library.suppressNextClick()` after a real drag.
  - **keydown** on `[data-library-select]`:
    - ArrowUp/ArrowDown move the roving focus within the visible rows.
    - ArrowRight focuses the row's `[data-place]`, or expands guests.
    - ArrowLeft goes back to the row or collapses.
    - Home/End jump to the ends.
    - Enter selects.
    - `p`/`P` on a To place row calls `placeAtCenter([id], {moveMode: true})`.
  - **Collapsed state:**
    - Read from `root.dataset.libraryCollapsed === 'true'`, set by infra.js.
    - When collapsed, hide `[data-library-scroll]` and unhide `[data-library-rail]`.
  - **reveal(id):**
    - Open the On the map section and expand the host that contains `id` (for `+N more`).
    - Scroll the row into view (`block: 'nearest'`) and focus it.
  - **Splitter:**
    - Pointer drag with capture sets `--infra-library-w` on `root`, clamped 220–400.
    - On pointerup it calls `app.setPref('libraryWidth', px)`.
    - Keys: ArrowLeft/ArrowRight ±16, Home 220, End 400, Enter toggles collapse.
    - `aria-valuenow` stays in sync.

- [ ] **Step 5: Verify.** Run `node --check ui/js/desktop/apps/infra-library.js`, then V-core.
- [ ] **Step 6: Commit.**
  - Paths: `ui/js/desktop/apps/infra-library.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add library panel with to-place, on-map and needs-host sections`.

---

## Task 10: Gestures module

**Files:**
- Create: `ui/js/desktop/apps/infra-gestures.js`
- Modify: `scripts/test-infra-ui.mjs`: insert `'infra-gestures.js'` after `'infra-canvas.js'`. Final order: model, store, canvas, gestures, library, inspector, table, dialogs, views, infra.

- [ ] **Step 1: Pointer capture helper.** Copy `capturePointer` from `ui/js/desktop/apps/easydrag-core.js:451-481` verbatim as a local function. It ends a gesture as cancelled on `pointercancel`, `lostpointercapture`, or a mouse move with no button. Add a local `frame(fn)` rAF coalescer, as in canvas.

- [ ] **Step 2: Drop planning.** Commit code, in full:

```js
        // planDrop turns wanted card centres into before/after intents: optional
        // overlap nudge, drop parent (Map view, server support) and region growth.
        function planDrop(view, wants, { nudge = true } = {}) {
            const scene = canvas.sceneData();
            const parentAware = view === 'atlas' && store.features.layoutParent;
            const moving = new Set(wants.keys());
            const obstacles = scene.cards.filter(card => !moving.has(card.id)).map(model.cardRect);
            const before = new Map(), after = new Map(), moves = [];
            const placementOf = id => after.get(id) || store.effectivePlacement(id, view);
            const ids = new Set(scene.regionRects.keys());
            let hitRegion = '';
            for (const [id, want] of wants) {
                const pos = nudge ? model.freeSlot(want, obstacles) : { x: want.x, y: want.y };
                obstacles.push(model.cardRect(pos));
                const current = store.effectiveParent(id, view);
                const parent = view === 'atlas' ? model.dropParent(current, pos, scene.regionRects, scene.regionDepth) : {};
                if (parent.parent_id) hitRegion = parent.parent_id;
                const parentChange = parentAware && 'parent_id' in parent ? parent : {};
                after.set(id, { x: Math.round(pos.x), y: Math.round(pos.y), pinned: true, ...parentChange });
                before.set(id, store.currentIntent(id, view, 'parent_id' in parentChange));
                moves.push({ id, from: current, to: 'parent_id' in parentChange ? parentChange.parent_id : current });
                const target = 'parent_id' in parentChange ? parentChange.parent_id : current;
                if (!parentAware || !target || !ids.has(target)) continue;
                for (const [regionId, rect] of model.growRegions(target, model.cardRect(pos), { ids, parentOf: rid => store.effectiveParent(rid, view), placementOf })) {
                    if (!before.has(regionId)) before.set(regionId, store.currentIntent(regionId, view, false));
                    after.set(regionId, { ...(after.get(regionId) || {}), x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.w), h: Math.round(rect.h), pinned: true });
                }
            }
            return { before, after, moves, hitRegion, parentAware };
        }
        function commitPlan(view, plan, kind) {
            const name = id => store.nodeById(id)?.name || id;
            const changed = plan.moves.filter(move => move.to !== move.from);
            const label = kind === 'place'
                ? (plan.moves.length === 1 ? t('desktop.infra_placed').replace('{name}', name(plan.moves[0].id)) : t('desktop.infra_placed_many').replace('{count}', plan.moves.length))
                : changed.length === 1 && plan.moves.length === 1
                    ? (changed[0].to ? t('desktop.infra_moved_into').replace('{name}', name(changed[0].id)).replace('{place}', name(changed[0].to)) : t('desktop.infra_moved_out').replace('{name}', name(changed[0].id)))
                    : t('desktop.infra_moved').replace('{count}', plan.moves.length);
            const { commands } = window.InfraAppStore;
            const command = kind === 'place' ? commands.placeFromLibrary(view, plan.after, label)
                : changed.length ? commands.reparent(view, plan.before, plan.after, label) : commands.moveNodes(view, plan.before, plan.after, label);
            void store.dispatch(command);
            app.announce(label);
            if (plan.hitRegion && !plan.parentAware) store.notify(t('desktop.infra_parent_unsupported'), 'info');
        }
```

  `planRegionMove(regionId, delta)`:
  1. Collect the region plus every drawn descendant region and card, via a BFS over the scene using `store.effectiveParent`.
  2. `after` = each card `{x+dx, y+dy, pinned: true}` and each region `{x+dx, y+dy, w, h, pinned: true}`, taken from `store.currentIntent(id, view, false)`.
  3. Before computing the region's own parent, check the server: if `store.features.layoutParent` is false, keep no parent change and skip step 4.
  4. Region parent: `model.dropParent(store.effectiveParent(regionId), {x: newX + 20, y: newY + 18}, rects, depth, new Set([regionId, ...descendantRegionIds]))`.
  5. If the parent changes, grow the new parent with `growRegions(target, movedRect, …)`.
  6. Dispatch `reparent` when the parent changed, else `moveNodes`.

  `planResize(regionId, handle, startRect, dx, dy)`:
  1. Apply the deltas to the edges named by `handle`.
  2. Snap each edge to 24.
  3. Clamp to `REGION_MIN_W`/`REGION_MIN_H`, keeping the opposite edge fixed.
  4. Before = `store.currentIntent(regionId, view, false)`; after = `{x, y, w, h, pinned: true}`.
  5. Dispatch `resizePlace`.
  6. Only allowed when `store.canWrite() && store.features.layoutParent && view === 'atlas'`.

- [ ] **Step 3: Viewport pointer routing.** `attach()` binds, on `canvas.viewport`: `pointerdown`, `pointermove`, `pointerup`, `pointercancel`, `lostpointercapture`, `wheel` with `{passive: false}`, `click`, `dblclick`, `keydown`, `keyup`, `focusin` and `focusout`. All are removed in `dispose()`. Behaviour is contractual:

| Situation on pointerdown | Result |
|---|---|
| a second pointer is down | Pinch. Abort the current gesture as cancelled, then `zoomAt(pinch.view, z*dist/startDist, midpoint)` on every move. Ends when fewer than 2 pointers remain. |
| `button === 1`, Space held, `tool === 'hand'`, or `pointerType === 'touch'` on empty canvas | Pan: `setView({x: start.x + dx, y: start.y + dy, zoom})`, class `is-panning`. |
| target in `[data-toggle-host-services]`, `[data-hosted-service]` or `[data-guest-more]` | Nothing; the click handles it. |
| `!store.canWrite()` | Shift on empty canvas gives a marquee, other empty canvas pans, everything else is left to click. |
| `[data-connect-handle]` on a card, or `tool === 'connect'` on a card | Connect gesture. |
| `[data-resize]` | Region resize. Live: `canvas.moveTemp(new Map(), new Map([[id, rect]]))`. |
| `[data-region-header]` | Region move. Live: moveTemp of the region and its descendants. |
| `[data-node]` (no Shift/Ctrl/⌘) | Card drag. |
| `[data-edge]` | Nothing; the click selects the edge. |
| empty canvas with Shift | Marquee. Ctrl/⌘ additionally keeps the current selection. |
| empty canvas | Pan with `clearOnClick`: no movement at pointerup means `store.clearSelection()`. |

  **Card drag:**
  1. Threshold 4 px.
  2. On start: if the card is not selected, `store.select([id])`. Then `ids` = selected cards that are visible in the scene, `store.beginInteraction()`, class `is-dragging` on `app.root`.
  3. Move: world delta = client delta / zoom. `snapDelta = snap(primary.x0 + dx) - primary.x0` (same for y). Positions = origin + snapDelta. Call `canvas.moveTemp(positions)` (rAF-coalesced) and `canvas.setDropTarget(regionAt(primary position))`. The drop target is shown only in atlas.
  4. Up, not cancelled: `planDrop(view, positions, {nudge: true})` then `commitPlan(view, plan, 'move')`. Then `canvas.clearTemp()`, `endInteraction()`, and `suppressClick = true` (cleared by the next click or after `setTimeout(…, 0)`).
  5. Cancelled (Esc, lostpointercapture, pinch): `clearTemp()` and announce `t('desktop.infra_move_cancelled')`.

  **Marquee:**
  - Screen rect shown with `canvas.showMarquee`.
  - World box = `normalizeRect(toWorld(a), toWorld(b))`.
  - Hits = scene cards whose `cardRect` intersects the box.
  - Selection becomes the hits, or base ∪ hits when Ctrl/⌘ is held. It changes only when the hit set changes (string key compare).

  **Connect:**
  - Threshold 4 px.
  - Each move:
    - target = `document.elementFromPoint(x, y)?.closest('[data-node]')` inside the viewport, if it is not the from-card;
    - `canvas.setConnectTarget(target)`;
    - `showEdgePreview(from, target ? target centre : pointer world)`.
  - Release on a target: `app.dialogs.openConnectionPopover({from, to, clientX, clientY})`.
  - A tap with no movement does nothing; the click still selects.
  - Esc cancels.

  **Wheel:**
  - Ctrl/⌘: `factor = Math.exp(-deltaY * (deltaMode === 1 ? 0.05 : 0.0025))`, zoom at the pointer.
  - Otherwise pan by `deltaX`/`deltaY`; Shift swaps the axes; `deltaMode === 1` multiplies by 16.
  - Always `preventDefault()`.

  **click** (delegated; skipped when `suppressClick`):
  - badge → `store.toggleHost(id)`
  - chip → `store.select([guestId])`
  - `[data-guest-more]` → `app.library.reveal(hostId)`
  - card → Shift or Ctrl/⌘ toggles it in the selection (`store.select([id], {toggle: true})`), otherwise `store.select([id])`
  - region header → same rule for the place
  - edge → `store.selectEdge(id)`

  `.click()` calls from the tests come without pointer events and must work.

  **dblclick on a card:** `store.select([id])`, then `app.inspector.focusName()`.

- [ ] **Step 4: Library drag.** `startLibraryDrag(event, ids)`:
  - Returns when `!store.canWrite()` or a gesture is active.
  - Capture target: `event.currentTarget` (the library `<aside>`).
  - **Threshold:** 5 px. For touch, a mostly vertical first movement (`|dy| > |dx|`) detaches the gesture so the list can scroll.
  - **Start:** `ghost = canvas.createGhost(ids)`, `store.beginInteraction()`, class `is-dragging` on the root.
  - **Move** (rAF-coalesced):
    - Read `hit = document.elementFromPoint(clientX, clientY)`.
    - Over the canvas means `canvas.viewport.contains(hit)`.
    - World point = `canvas.clientToWorld`.
    - Region = in atlas `regionAt(world, rects, depth)`, else `''`; call `canvas.setDropTarget(region)`.
    - `ghost.move(x, y, { scale: over ? zoom : 1, label: over ? (region ? fill(t('desktop.infra_drop_into'), {name}) : t('desktop.infra_drop_on_map')) : '' })`.
  - **Up, over the canvas:**
    - `wants`: a single id uses the world point; several ids use `model.compactGrid(ids.length, world)` in order.
    - Run `planDrop(view, wants, {nudge: true})` and `commitPlan(view, plan, 'place')`.
    - Then `store.select(ids)`.
  - **Always on end:** `ghost.remove()`, `setDropTarget('')`, remove the root classes, `endInteraction()`. After a real drag, call `app.library.suppressNextClick()`.

  **Read-only:** no drag starts. Rows show `cursor: not-allowed` through the CSS on `.infra-app[data-readonly="true"] .infra-library-row`.

- [ ] **Step 5: Keyboard on the canvas.** `keydown` on the viewport; ignore when the target is an input, select, textarea or contenteditable.

| Key | Action |
|---|---|
| Arrow (no move mode) | `next = model.spatialNeighbour(fromCard, scene.cards, dir)`. From the viewport itself, take the selected card, else the first in reading order (sort by y then x). Then `canvas.setRovingCard(next)`, `cardEl(next).focus({preventScroll: true})`, `canvas.ensureVisible(next)`, `announce(card aria-label)`. |
| Enter | Select the focused card or region; the inspector opens through the selection event. |
| Space (keyup, if not used for panning) | Toggle the focused card in the selection. |
| `m`/`M` | `enterMoveMode(selection cards or the focused card)`. |
| Delete/Backspace | `app.dialogs.deleteNodes([...selection])` when canWrite (`deleteNode` for one). |
| Escape | Cancel the gesture or move mode; else clear the selection. |
| Ctrl/⌘+A | Select all undimmed scene cards. |
| `f` | `canvas.fit({animate: true})`. |
| `+`/`=`, `-`, `0` | Zoom in, zoom out, zoom to 1. |
| `v`, `h`, `c` | `store.setTool('select' / 'hand' / 'connect')`. Connect is ignored when read-only. |

  `focusin` on the viewport itself:
  - If no card has focus, focus the selected card or the first card in reading order (`setRovingCard`).
  - With no cards, the viewport keeps focus.

  **Move mode:**
  - `enterMoveMode(ids)` sets `moveMode = {view, ids, origin: Map(id → {x, y}), dx: 0, dy: 0}`, calls `store.beginInteraction()` and announces `t('desktop.infra_move_mode')`.
  - Arrows add ±24, or ±8 with Shift, then call `canvas.moveTemp(origin + delta)`.
  - Enter: `planDrop(view, positions, {nudge: false})`, then `commitPlan(view, plan, 'move')`, then exit.
  - Escape: `clearTemp()`, announce `t('desktop.infra_move_cancelled')`, exit.
  - `focusout` from the viewport commits, the same as Enter.

- [ ] **Step 6: `placeAtCenter`, `arrange` and the global shortcuts.**
  - **`placeAtCenter(ids, {moveMode})`:**
    - Return when `!store.canWrite()`.
    - Call `app.ensureMapView()`.
    - Centre = `canvas.clientToWorld` of the viewport centre.
    - Wants = compactGrid around the centre, then `planDrop(view, wants, {nudge: true})`, `commitPlan(view, plan, 'place')`, `store.select(ids)`.
    - `canvas.renderNow()`, focus the first card.
    - If `moveMode` is set: `enterMoveMode(ids)`.
  - **`arrange()`:**
    - `view = app.mapView()`.
    - Targets = selected scene cards, or else every scene card without a drawn parent region.
    - Obstacles = the other scene cards' positions.
    - Positions = `view === 'topology' ? model.topologyPositions(targets, obstacles) : model.gridPositions(targets, obstacles, store.state.nodes.length)`. Target nodes need `kind` and `name`; pass the node objects.
    - Before = `currentIntent`; after = the positions (pinned as returned).
    - Dispatch `moveNodes(view, before, after, t('desktop.infra_arranged'))` and announce it.
    - Pinned cards are included on purpose; undo covers them.
  - **`onRootKeydown(event)`:**
    - infra.js binds this on `app.root`.
    - Ignored while `[data-dialog]` is open.
    - Ctrl/⌘+K: focus and select `[data-search]`.
    - Ctrl/⌘+Z: `store.undo(view)`. Ctrl/⌘+Shift+Z and Ctrl/⌘+Y: `store.redo(view)`. Both skip typing targets and `preventDefault`. Announce `t('desktop.infra_undone')` / `t('desktop.infra_redone')` when the result is true.
    - Ctrl/⌘+S: `app.inspector.saveDraft()`, with `preventDefault`, even while typing.
    - Escape inside `[data-inspector]` on a non-text target: `app.inspector.close()`.
  - **`cancel()`:** aborts any active capture gesture, ghost or move mode (on policy revoke and dispose).

- [ ] **Step 7: Verify.** Run `node --check ui/js/desktop/apps/infra-gestures.js`, then V-core. Expect under 1100 lines, roughly 800.
- [ ] **Step 8: Commit.**
  - Paths: `ui/js/desktop/apps/infra-gestures.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add canvas gestures, library drag, connect, move mode and shortcuts`.

---

## Task 11: i18n batch E (command bar and shell)

**Files:** `ui/lang/desktop/*.json`.

- [ ] **Step 1: Write `$SCRATCH/infra-i18n-e.json`** with all 16 locales. Rules as in Task 2. The two resize keys already moved to batch D.

| key | en | de |
|---|---|---|
| `desktop.infra_view_map` | Map | Karte |
| `desktop.infra_view_list` | List | Liste |
| `desktop.infra_search_jump` | Search or jump… | Suchen oder springen… |
| `desktop.infra_filters` | Filters | Filter |
| `desktop.infra_redo` | Redo | Wiederholen |
| `desktop.infra_undo_none` | Nothing to undo | Nichts zum Rückgängigmachen |
| `desktop.infra_redo_none` | Nothing to redo | Nichts zum Wiederholen |
| `desktop.infra_add_menu` | More ways to add | Weitere Möglichkeiten zum Hinzufügen |
| `desktop.infra_sync_live` | Live | Live |
| `desktop.infra_sync_syncing` | Syncing… | Synchronisiere… |
| `desktop.infra_sync_error` | Not up to date | Nicht aktuell |
| `desktop.infra_sync_paused` | Paused | Pausiert |

- [ ] **Step 2: Run** the script with `infra-i18n-e.json`. Expected: `+12` per locale.
- [ ] **Step 3: Verify:** V-core.
- [ ] **Step 4: Commit.** Paths: `ui/lang/desktop/*.json`. Subject: `i18n(infra): add command bar and sync keys`.

---

## Task 12: Cutover (composition root, shell CSS, loader, browser tests)

**Files:**
- Rewrite: `ui/js/desktop/apps/infra.js`
- Delete: `ui/js/desktop/apps/infra-views.js`
- Rewrite: `ui/css/desktop-app-infra.css`
- Modify: `ui/js/desktop/core/module-loader.js:27`
- Modify: `ui/js/desktop/apps/infra-model.js` (remove `sceneDimensions`, `containerFrame`, `atlasDimensions`)
- Modify: `scripts/test-infra-ui.mjs`
- Modify: `ui/js/desktop/apps/AGENTS.md`
- Modify: `ui/desktop_infra_browser_test.go`

- [ ] **Step 1: Impact check** for `render`, `dispose` (InfraApp), `create` (InfraAppViews), `sceneDimensions`, `containerFrame`, `atlasDimensions`, `ServeHTTP` (infraAppBrowserFixture) and `TestDesktopInfraAppBrowser`. Confirm callers with:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && git grep -n -e "InfraApp\b" -e "InfraAppViews" -e "sceneDimensions" -e "containerFrame" -e "atlasDimensions" -e "infraAppBrowserFixture" -- ui scripts
  ```
  Expected:
  - `menus-and-routing.js:1644-1645` calls `window.InfraApp.render`, and the desktop foundation calls `dispose`.
  - The removed helpers are used only by `infra-views.js`/`infra.js` and the test.
  - The fixture is used by the three Infra browser test files.

- [ ] **Step 2: Rewrite `infra.js`** (target about 600 lines). `render(host, windowId, ctx)`:
  1. `dispose(windowId)`. Return if there is no host.
  2. Set `t = key => ctx.t?.(key) || key`, `esc = ctx.esc || model.esc`, `safeId` as today.
  3. `store = InfraAppStore.create({ctx, t})`.
  4. Build `host.innerHTML` with the shell (structure below).
  5. Create `app` (§1.1).
  6. Create the modules in this order: canvas, table, library, inspector, dialogs, gestures. Insert `library.el` and `library.splitter`, `canvas.el`, `table.el`, the scrim, the inspector splitter and the inspector `<aside>` into `[data-body]` in that order.
  7. Read prefs: `model.readLayoutPrefs(localStorage['infra.layout.v1'])`.
     - Apply them as `--infra-library-w`, `--infra-inspector-w` and `root.dataset.libraryCollapsed`.
     - Apply the minimap `aria-pressed`/hidden state.
     - `app.setPref(key, value)` merges and persists, debounced 300 ms, in try/catch.
  8. `store.subscribe(route)` (routing table below).
  9. `gestures.attach()`. `syncMenus()`. Register `ctx.setWindowBeforeClose`.
  10. `store.attach({root, win: host.closest('.vd-window')})`.
  11. Initial paint: `paintShell()`, `canvas.render()`, `library.render()`, `table.render()`, `inspector.paint()`.
  12. Register the instance with its dispose.

  Shell markup:

```html
<section class="infra-app" data-infra-app="${safeId}" aria-label="{t desktop.app_infra}" data-inspector-open="false" data-library-collapsed="false" data-library-sheet="closed" data-readonly="false">
  <header class="infra-commandbar" aria-label="{t desktop.infra_actions}">
    <div class="infra-brand"><img class="infra-brand-mark" src="{assetURL('icon')}" alt="" draggable="false"><span class="infra-brand-name">{t desktop.app_infra}</span></div>
    <button type="button" class="infra-cmd-icon" data-action="toggle-library" aria-controls="infra-library-${S}" aria-expanded="true" aria-label="{t desktop.infra_library}" title="…">{glyph library}</button>
    <div class="infra-view-switch" role="group" aria-label="{t desktop.infra_view}">
      <button type="button" data-view="atlas" aria-pressed="true">{t desktop.infra_view_map}</button>
      <button type="button" data-view="topology" aria-pressed="false">{t desktop.infra_topology}</button>
      <button type="button" data-view="table" aria-pressed="false">{t desktop.infra_view_list}</button>
    </div>
    <div class="infra-search">
      <span class="infra-search-icon" aria-hidden="true">{glyph search}</span>
      <input type="search" data-search aria-label="{t desktop.infra_search}" placeholder="{t desktop.infra_search_jump}" autocomplete="off" aria-keyshortcuts="Control+K">
      <kbd class="infra-search-kbd" aria-hidden="true">Ctrl K</kbd>
      <button type="button" class="infra-cmd-icon" data-action="filters" aria-haspopup="true" aria-expanded="false" aria-controls="infra-filters-${S}" aria-label="{t desktop.infra_filters}" title="…">{glyph filter}<span class="infra-filter-dot" data-filter-dot hidden></span></button>
    </div>
    <div class="infra-live-status" data-live data-sync="syncing" data-refreshed-at="0" title=""><i aria-hidden="true"></i><span class="infra-sync-label">{t desktop.infra_sync_syncing}</span></div>
    <div class="infra-history" role="group">
      <button type="button" class="infra-cmd-icon" data-action="undo" aria-disabled="true" aria-keyshortcuts="Control+Z" aria-label="{t desktop.infra_undo}" title="{t desktop.infra_undo_none}">{glyph undo}</button>
      <button type="button" class="infra-cmd-icon" data-action="redo" aria-disabled="true" aria-keyshortcuts="Control+Shift+Z" aria-label="{t desktop.infra_redo}" title="{t desktop.infra_redo_none}">{glyph redo}</button>
    </div>
    <div class="infra-split" role="group">
      <button type="button" class="infra-button infra-button-primary" data-action="add" aria-label="{t desktop.infra_new_asset}" title="…" disabled>{glyph plus}<span class="infra-cmd-label">{t desktop.infra_new_asset}</span></button>
      <button type="button" class="infra-button infra-button-primary infra-split-more" data-action="add-menu" aria-haspopup="menu" aria-expanded="false" aria-controls="infra-add-menu-${S}" aria-label="{t desktop.infra_add_menu}" title="…" disabled>{glyph chevron}</button>
    </div>
    <button type="button" class="infra-cmd-icon" data-action="more" aria-haspopup="menu" aria-expanded="false" aria-controls="infra-more-menu-${S}" aria-label="{t desktop.infra_more_actions}" title="…">{glyph more}</button>
  </header>
  <div class="infra-body" data-body>
    <!-- library aside + splitter, canvas section, table section, then: -->
    <div class="infra-scrim" data-scrim></div>
    <div class="infra-splitter" data-splitter="inspector" role="separator" aria-orientation="vertical" aria-controls="infra-inspector-${S}" aria-valuemin="320" aria-valuemax="520" aria-valuenow="360" aria-label="{t desktop.infra_resize_inspector}" tabindex="0"></div>
    <aside class="infra-inspector" id="infra-inspector-${S}" data-inspector hidden aria-label="{t desktop.infra_details}" tabindex="-1"></aside>
  </div>
  <div class="infra-layer" data-layer>
    <div class="infra-menu" id="infra-add-menu-${S}" data-menu="add" role="menu" hidden>
      <button type="button" role="menuitem" data-action="add-place" disabled>{t desktop.infra_new_place}</button>
      <button type="button" role="menuitem" data-action="import" disabled>{t desktop.infra_import_discoveries}</button>
    </div>
    <div class="infra-menu" id="infra-more-menu-${S}" data-menu="more" role="menu" hidden>
      <button type="button" role="menuitem" data-action="refresh">{t desktop.infra_refresh}</button>
      <button type="button" role="menuitem" data-action="arrange" disabled>{t desktop.infra_arrange}</button>
      <button type="button" role="menuitem" data-action="history-undo" data-overflow-only>{t desktop.infra_undo}</button>
      <button type="button" role="menuitem" data-action="history-redo" data-overflow-only>{t desktop.infra_redo}</button>
    </div>
    <div class="infra-popover infra-filters" id="infra-filters-${S}" data-popover="filters" hidden>
      <!-- the three labelled selects from old infra.js lines 54-56 unchanged: data-kind-filter, data-source-filter, data-power-filter -->
    </div>
  </div>
  <div class="infra-toasts" data-toasts></div>
  <div class="infra-sr-only" data-announce aria-live="polite" aria-atomic="true"></div>
  <dialog class="infra-dialog" data-dialog aria-labelledby="infra-dialog-title-${S}"><form method="dialog"><button class="infra-dialog-close" aria-label="{t desktop.close}">×</button></form><div data-dialog-content></div></dialog>
</section>
```

  Contracts:
  - **Menus and popovers** live in `.infra-layer`, so the command bar's `overflow: hidden` never clips them.
    - `openMenu(name, trigger)`: position at `top = triggerRect.bottom - appRect.top + 6`, `right = appRect.right - triggerRect.right` (for filters: `left = triggerRect.left - appRect.left`). Unhide it, set `aria-expanded="true"` on the trigger, focus the first enabled item.
    - ArrowUp/ArrowDown cycle the items.
    - Escape, an outside pointerdown (document capture, removed on close) or activating an item closes the menu and returns focus to the trigger.
    - Opening one menu closes the others.
  - **Root click delegation:**
    - Ignore clicks inside `[data-inspector]`, `[data-dialog]`, `[data-connect-popover]`, `[data-stage]`, `[data-library]` and `[data-table-view]`. Those modules handle their own.
    - `[data-view]` → `store.setView(view)`.
    - `[data-tool]` → `store.setTool(tool)`.
    - Action mapping:

      | `data-action` | Effect |
      |---|---|
      | `refresh` | `store.loadSnapshot(true)` |
      | `add`, `add-empty` | `dialogs.openNodeDialog('device')` |
      | `add-place` | `dialogs.openNodeDialog('room', {title: t('desktop.infra_new_place')})` |
      | `add-menu` | `openMenu('add')` |
      | `more` | `openMenu('more')` |
      | `filters` | `openMenu('filters')` |
      | `import`, `import-empty` | `dialogs.openImport()` |
      | `arrange` | `gestures.arrange()` |
      | `undo`, `history-undo` | `if (store.canUndo(view)) store.undo(view)` |
      | `redo`, `history-redo` | `if (store.canRedo(view)) store.redo(view)` |
      | `toggle-library` | ≥760 px: `setLibraryCollapsed(!collapsed)`; <760 px: toggle `data-library-sheet` and close the inspector sheet |
      | `retry-layout` | `store.retryLayout(app.mapView())` |
      | `clear-search` | clear `[data-search]` and `store.setFilters({query: ''})` |
      | `zoom-in`, `zoom-out`, `zoom-reset`, `fit`, `toggle-minimap` | canvas |
      | `dismiss-toast` | remove the toast |

    - Width checks use `root.clientWidth`.
  - **Search:**
    - The `input` event is coalesced to one `store.setFilters({query})` per animation frame. This keeps today's search stampede guard.
    - Enter selects the first undimmed card (by reading order), calls `canvas.centerOn(id)` and focuses the card.
    - Escape clears the query.
  - **Filter selects:** `change` → `store.setFilters({kind | source | power})`. `[data-filter-dot]` is unhidden when kind/source/power is set.
  - **Scrim:** a click closes the inspector (`inspector.close()`) and the library sheet.
  - **Toasts:**
    - `app.toast(message, kind)` appends `<div class="infra-toast" data-toast data-kind="{kind}"><span>{message}</span><button type="button" class="infra-toast-close" data-action="dismiss-toast" aria-label="{t desktop.close}">{glyph close}</button></div>`.
    - It de-duplicates by text (replacing the older toast) and keeps at most 3.
    - `success`/`info` toasts auto-dismiss after 4000 ms through tracked timers, cleared on dispose. Never use 30000 here.
    - `error` toasts stay until dismissed. No poll clears them.
    - Every toast text is also passed to `app.announce`.
  - **paintShell():**
    - `[data-view]` `aria-pressed`.
    - `[data-canvas]` hidden in table view; `[data-table-view]` hidden otherwise.
    - `root.dataset.readonly = String(!store.canWrite())`.
    - **Write controls,** disabled when `!store.canWrite()`: `add`, `add-menu`, `add-place`, `import`, `add-empty`, `import-empty`.
    - **Arrange,** disabled when `!canWrite`, when there are no scene cards, or while `state.busy.has('layout')`.
    - **Refresh,** disabled while `state.busy.has('snapshot')`.
    - **History:**
      - Undo/redo `aria-disabled` follow `canUndo`/`canRedo` of `app.mapView()`.
      - Titles: `t('desktop.infra_undo')` or `t('desktop.infra_undo_none')`, and the same pair for redo.
      - The overflow history items are disabled the same way.
    - **Tool and chip:** `[data-tool="connect"]` disabled when read-only, plus the canvas chip state.
    - **Sync indicator `[data-live]`:**
      - `data-sync` = `state.busy.has('snapshot') ? 'syncing' : state.sync`; class `is-error` when the sync state is `'error'` or `'paused'`.
      - Label: `t('desktop.infra_sync_live' | '…syncing' | '…error' | '…paused')`.
      - `data-refreshed-at = String(state.refreshAt)`.
      - Title: `t('desktop.infra_updated').replace('{time}', …)`, then ` · ` and `t('desktop.infra_sources_count')` with `{current}`/`{total}` filled in, or `t('desktop.infra_sources_waiting')`.
  - **Window menus.** `syncMenus()` calls `ctx.setWindowMenus(windowId, [{ id: 'infra', labelKey: 'desktop.app_infra', items: [...] }])` with these items:

    | labelKey | icon | disabled | action |
    |---|---|---|---|
    | `desktop.infra_refresh` | `refresh` | — | `store.loadSnapshot(true)` |
    | separator | | | |
    | `desktop.infra_undo` | — | `!store.canUndo(app.mapView())` | `store.undo(app.mapView())` |
    | `desktop.infra_redo` | — | `!store.canRedo(app.mapView())` | `store.redo(app.mapView())` |
    | separator | | | |
    | `desktop.infra_import_title` | `download` | `!store.canWrite()` | `dialogs.openImport` |
    | `desktop.infra_new_asset` | `plus` | `!store.canWrite()` | `dialogs.openNodeDialog('device')` |
    | `desktop.infra_new_place` | — | `!store.canWrite()` | `dialogs.openNodeDialog('room', {title})` |
    | `desktop.infra_arrange` | `grid` | `!store.canWrite()` | `gestures.arrange()` |
  - **Routing table** for `store.subscribe`:
    - **`snapshot`:**
      - While `state.interacting`: remember `deferred = deferred || (structural || explicit)` and return.
      - Otherwise, if neither `structural` nor `explicit`: `paintShell()` only.
      - Else: `canvas.render()`, `library.render()`, `table.render()`, `paintShell()`, `inspector.paintIfIdle()`.
    - **`interaction-end`:** flush `deferred` as a structural snapshot paint.
    - **`layout`, `nodes`, `edges`, `expanded`:** `canvas.render()`, `library.render()`, `table.render()`. For `nodes`/`edges` also `inspector.paintIfIdle()`.
    - **`layout-status`:** `canvas.render()` (states and chip) and `paintShell()`.
    - **`selection`:** `canvas.render()`, `library.renderSelection()`, `table.render()`, `inspector.paint()`. At <760 px an open inspector closes the library sheet.
    - **`view`:** `paintShell()`, `canvas.render({force: true})`, `library.render()`, `table.render()`, `inspector.paint()`, `syncMenus()`.
    - **`filters`:** `canvas.render()`, `library.render()`, `table.render()`, `paintShell()`.
    - **`tool`:** `canvas.applyTool()`.
    - **`busy`:** `paintShell()`, `inspector.onBusy(key)`, `dialogs.onBusy(key)`.
    - **`caps`:** `gestures.cancel()`, `syncMenus()`, `paintShell()`, `canvas.render({force: true})`, `library.render()`, `inspector.paint()`.
    - **`history`:** `paintShell()`, `syncMenus()`.
    - **`notice`:** `app.toast(message, kind)`.
    - **`sync`:** `paintShell()`; announce when `announce` is set.
    - **`targets`, `credentials`:** `inspector.paintIfIdle()`.
    - **`manifest`:** `canvas.render({force: true})`, `library.render()`.
  - **Root listeners:**
    - `focusout` → `setTimeout(() => inspector.flushStale(), 0)`.
    - `keydown` → `gestures.onRootKeydown`.
    - The inspector splitter: same contract as the library splitter, range 320–520, pref `inspectorWidth`. Dragging left widens.
  - **Before-close:** `ctx.setWindowBeforeClose(windowId, async () => !store.hasUnsavedWork() && !state.busy.has('layout') ? true : dialogs.confirmAction(t('desktop.app_infra'), t('desktop.infra_unsaved_close'), { confirmLabel: t('desktop.infra_discard') }))`.
  - **Dispose** runs once:
    - `gestures.dispose()`, `canvas.dispose()`, `library.dispose()`, `inspector.dispose()`, `dialogs.dispose()` (closes the dialog and popover), `table.dispose()`.
    - Clear the toast and announce timers.
    - `store.dispose()`: aborts requests and the timer, removes observers.
    - `ctx.setWindowBeforeClose(windowId, null)`, `ctx.setWindowMenus(windowId, [])`, `root.replaceChildren()`.
  - `window.InfraApp = { render, dispose }` is unchanged.

- [ ] **Step 3: Delete `infra-views.js`** with `git rm ui/js/desktop/apps/infra-views.js`.

- [ ] **Step 4: Update the module loader.** Line 27 becomes:
  ```js
        infra: { styles: appStyles('/css/desktop-app-infra.css', '/css/desktop-app-infra-canvas.css'), scripts: ['/js/desktop/apps/infra-model.js', '/js/desktop/apps/infra-store.js', '/js/desktop/apps/infra-canvas.js', '/js/desktop/apps/infra-gestures.js', '/js/desktop/apps/infra-library.js', '/js/desktop/apps/infra-inspector.js', '/js/desktop/apps/infra-table.js', '/js/desktop/apps/infra-dialogs.js', '/js/desktop/apps/infra.js'] },
  ```

- [ ] **Step 5: Model cleanup.**
  - Remove `atlasDimensions`, `sceneDimensions` and `containerFrame` from `infra-model.js` and its export object.
  - In `scripts/test-infra-ui.mjs`:
    - Remove `'infra-views.js'` from the file list. The final list is `['infra-model.js', 'infra-store.js', 'infra-canvas.js', 'infra-gestures.js', 'infra-library.js', 'infra-inspector.js', 'infra-table.js', 'infra-dialogs.js', 'infra.js']`.
    - Replace the `sceneDimensions` block (current lines 60–68) with the block below.
    - Delete the `containerFrame` block (current lines 123–131); `regionRect` now pins the 48 px padding.
    - Change the CSS guard list to `['desktop-app-infra.css', 'desktop-app-infra-canvas.css']`.

```js
const placedNodes = [{ id: 'north', kind: 'device', placements: { atlas: { x: -330, y: 85 } } }, { id: 'far', kind: 'device', placements: { atlas: { x: 5200, y: 2910 } } }];
const unplacedNodes = Array.from({ length: 1000 }, (_, index) => ({ id: `unplaced-${index}`, kind: 'device', placements: {} }));
const worldOf = nodes => { const byId = new Map(nodes.map(node => [node.id, node])); return model.buildScene({ nodes, edges: [], view: 'atlas', placementOf: id => byId.get(id).placements.atlas, parentOf: () => '', guests: model.guestIndex(nodes, []) }).bounds; };
assert.deepEqual(plain(worldOf([...placedNodes, ...unplacedNodes])), plain(worldOf(placedNodes)), 'unplaced nodes never change the world bounds');
const world = worldOf(placedNodes);
assert.ok(world.x < -330 && world.x + world.w > 5200 && world.y + world.h > 2910, 'saved negative and far positions stay in the world');
```

- [ ] **Step 6: Rewrite `desktop-app-infra.css`.** Tokens only; the guard in `test-infra-ui.mjs` enforces it. Contents, in this order:

  **(a) Root:**
  ```css
  .infra-app { --infra-library-w: 264px; --infra-inspector-w: 360px; --infra-ok: var(--ds-color-success-fg); --infra-warn: var(--ds-color-warning-fg); --infra-bad: var(--ds-color-danger-fg); --infra-info: var(--ds-color-info-fg); position: relative; width: 100%; height: 100%; min-width: 0; min-height: 340px; display: grid; grid-template-rows: 44px minmax(0, 1fr); overflow: hidden; isolation: isolate; color: var(--vd-text); background: var(--vd-theme-app-bg); font: 14px/1.45 var(--vd-font-ui, system-ui, sans-serif); container: infra / inline-size; }
  .infra-app * { box-sizing: border-box; }
  .infra-app button, .infra-app input, .infra-app select, .infra-app textarea { font: inherit; color: inherit; }
  .infra-app [hidden] { display: none !important; }
  .infra-app :is(button, input, select, textarea, [tabindex="0"]):focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset); }
  .infra-app button:disabled { opacity: .45; cursor: not-allowed; }
  .infra-sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
  .infra-glyph { width: 16px; height: 16px; flex: 0 0 auto; }
  ```

  **(b) Command bar:**
  ```css
  .infra-commandbar { display: flex; align-items: center; gap: var(--ds-space-2); min-width: 0; height: 44px; padding: 0 var(--ds-space-2) 0 var(--ds-space-3); overflow: hidden; border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); white-space: nowrap; }
  .infra-brand { display: flex; flex: 0 0 auto; align-items: center; gap: var(--ds-space-2); }
  .infra-brand-mark { width: 24px; height: 24px; object-fit: contain; }
  .infra-brand-name { font-size: 15px; font-weight: 600; }
  .infra-cmd-icon { position: relative; flex: 0 0 auto; width: 36px; height: 36px; display: inline-grid; place-items: center; padding: 0; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; cursor: pointer; }
  .infra-cmd-icon:hover { background: var(--vd-theme-control-hover); }
  .infra-cmd-icon[aria-disabled="true"] { opacity: .45; cursor: default; }
  .infra-cmd-icon[aria-expanded="true"], .infra-cmd-icon[aria-pressed="true"] { border-color: var(--vd-theme-accent-border); background: var(--vd-theme-accent-soft); }
  .infra-view-switch { display: inline-flex; flex: 0 0 auto; padding: 2px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-control-bg); }
  .infra-view-switch button { height: 30px; padding: 0 12px; border: 0; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-theme-muted); font-size: 14px; font-weight: 600; cursor: pointer; }
  .infra-view-switch button[aria-pressed="true"] { background: var(--vd-theme-panel-bg-strong); color: var(--vd-text); box-shadow: var(--vd-theme-shadow-panel); }
  .infra-search { flex: 1 1 320px; min-width: 0; max-width: 460px; height: 34px; display: flex; align-items: center; gap: var(--ds-space-1); padding: 0 2px 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-control-bg); color: var(--vd-theme-muted); }
  .infra-search:focus-within { border-color: var(--ds-color-border-focus); }
  .infra-search input { flex: 1; min-width: 0; height: 100%; border: 0; outline: 0; background: transparent; color: var(--vd-text); font-size: 14px; }
  .infra-search .infra-cmd-icon { width: 30px; height: 30px; }
  .infra-search-kbd { padding: 3px 5px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-xs); color: var(--vd-theme-muted); font: 600 12px/1 var(--vd-font-ui); }
  .infra-filter-dot { position: absolute; top: 5px; right: 5px; width: 7px; height: 7px; border-radius: 50%; background: var(--vd-accent); }
  .infra-live-status { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 6px; color: var(--vd-theme-muted); font-size: 14px; }
  .infra-live-status > i { width: 8px; height: 8px; border-radius: 50%; background: var(--infra-ok); }
  .infra-live-status[data-sync="syncing"] > i { background: var(--infra-info); }
  .infra-live-status[data-sync="paused"] > i { background: var(--vd-theme-muted); }
  .infra-live-status.is-error > i { background: var(--infra-bad); }
  .infra-history, .infra-split { display: inline-flex; flex: 0 0 auto; }
  .infra-split { margin-left: auto; gap: 1px; }
  .infra-button { height: 34px; display: inline-flex; align-items: center; justify-content: center; gap: 6px; padding: 0 12px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); font-size: 14px; font-weight: 600; cursor: pointer; }
  .infra-button:hover:not(:disabled) { background: var(--vd-theme-control-hover); }
  .infra-button-primary { border-color: transparent; background: var(--vd-accent); color: var(--vd-accent-contrast); }
  .infra-button-primary:hover:not(:disabled) { background: color-mix(in srgb, var(--vd-accent) 86%, var(--vd-text)); }
  .infra-split > .infra-button:first-child { border-radius: var(--ds-radius-sm) 0 0 var(--ds-radius-sm); }
  .infra-split-more { width: 28px; padding: 0; border-radius: 0 var(--ds-radius-sm) var(--ds-radius-sm) 0; }
  .infra-danger-button { color: var(--infra-bad) !important; }
  ```

  **(c) Layer, menus and popovers:**
  ```css
  .infra-layer { position: absolute; inset: 0; z-index: 30; pointer-events: none; }
  .infra-menu, .infra-popover { position: absolute; min-width: 220px; padding: var(--ds-space-1); border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-modal); pointer-events: auto; }
  .infra-menu [role="menuitem"] { display: flex; width: 100%; min-height: 36px; align-items: center; padding: 0 var(--ds-space-3); border: 0; border-radius: var(--ds-radius-sm); background: transparent; font-size: 14px; text-align: left; cursor: pointer; }
  .infra-menu [role="menuitem"]:hover, .infra-menu [role="menuitem"]:focus-visible { background: var(--vd-theme-control-hover); }
  .infra-menu [data-overflow-only] { display: none; }
  .infra-filters { display: grid; gap: var(--ds-space-2); padding: var(--ds-space-3); }
  ```
  Port `.infra-select-label` from the old lines 84–85 with token mapping.

  **(d) Body grid:**
  ```css
  .infra-body { position: relative; min-height: 0; display: grid; grid-template-rows: minmax(0, 1fr); grid-template-columns: var(--infra-library-w) 4px minmax(0, 1fr); }
  .infra-app[data-inspector-open="true"] .infra-body { grid-template-columns: var(--infra-library-w) 4px minmax(0, 1fr) 4px var(--infra-inspector-w); }
  .infra-app[data-library-collapsed="true"] .infra-body { grid-template-columns: 40px 0 minmax(0, 1fr); }
  .infra-app[data-library-collapsed="true"][data-inspector-open="true"] .infra-body { grid-template-columns: 40px 0 minmax(0, 1fr) 4px var(--infra-inspector-w); }
  .infra-library { grid-column: 1; }
  .infra-splitter[data-splitter="library"] { grid-column: 2; }
  .infra-canvas, .infra-table-view { grid-column: 3; }
  .infra-splitter[data-splitter="inspector"] { grid-column: 4; }
  .infra-inspector { grid-column: 5; }
  .infra-app[data-inspector-open="false"] .infra-splitter[data-splitter="inspector"] { display: none; }
  .infra-scrim { display: none; position: absolute; inset: 0; z-index: 7; background: var(--vd-theme-overlay-bg); }
  .infra-splitter { position: relative; z-index: 4; cursor: col-resize; touch-action: none; }
  .infra-splitter::after { content: ''; position: absolute; top: 0; bottom: 0; left: -3px; right: -3px; }
  .infra-splitter:hover, .infra-splitter:focus-visible, .infra-splitter.is-dragging { background: var(--vd-theme-accent-border); }
  ```

  **(e) Library:**
  ```css
  .infra-library { display: flex; flex-direction: column; min-width: 0; min-height: 0; border-right: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }
  .infra-library-scroll { flex: 1; min-height: 0; overflow: auto; padding: var(--ds-space-2); }
  .infra-library-section + .infra-library-section { margin-top: var(--ds-space-3); }
  .infra-library-heading { margin: 0; }
  .infra-library-toggle { display: flex; width: 100%; min-height: 32px; align-items: center; justify-content: space-between; padding: 0 var(--ds-space-2); border: 0; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-theme-muted); cursor: pointer; }
  .infra-library-title { font-size: 13px; font-weight: 600; }
  .infra-library-count { padding: 2px 6px; border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); font: 600 13px/1 var(--vd-font-ui); font-variant-numeric: tabular-nums; }
  .infra-library-list, .infra-library-children { margin: 0; padding: 0; list-style: none; }
  .infra-library-list { display: grid; gap: 2px; margin-top: var(--ds-space-1); }
  .infra-library-children { margin-left: var(--ds-space-4); padding-left: var(--ds-space-2); border-left: 1px solid var(--vd-theme-border); }
  .infra-library-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: var(--ds-space-1); border-radius: var(--ds-radius-sm); touch-action: pan-y; }
  .infra-library-row[aria-selected="true"] > .infra-library-row-main { background: var(--vd-theme-accent-soft); }
  .infra-library-row-main { display: grid; grid-template-columns: 28px minmax(0, 1fr); align-items: center; gap: var(--ds-space-2); min-height: 44px; padding: 4px var(--ds-space-2); border: 0; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
  .infra-library-row[data-draggable="true"] > .infra-library-row-main { cursor: grab; }
  .infra-app[data-readonly="true"] .infra-library-row[data-section="to-place"] > .infra-library-row-main { cursor: not-allowed; }
  .infra-library-row-icon { width: 28px; height: 28px; object-fit: contain; }
  .infra-library-row-text { display: grid; min-width: 0; }
  .infra-library-row-name, .infra-library-row-meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
  .infra-library-row-name { font-weight: 600; }
  .infra-library-row-meta { color: var(--vd-theme-muted); font-variant-numeric: tabular-nums; }
  .infra-library-place, .infra-library-disclosure { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; cursor: pointer; }
  .infra-library-place:hover:not(:disabled), .infra-library-disclosure:hover { background: var(--vd-theme-control-hover); }
  .infra-library-disclosure[aria-expanded="true"] .infra-glyph { transform: rotate(180deg); }
  .infra-library-empty { margin: var(--ds-space-2); color: var(--vd-theme-muted); font-size: 14px; }
  .infra-library-rail { display: flex; flex-direction: column; align-items: center; padding-top: var(--ds-space-2); }
  ```

  **(f) Inspector, table and dialogs:** port the old rules with this token map, and keep the class names the ported markup uses.
  - Map: `--infra-ink` → `var(--vd-text)`, `--infra-muted` → `var(--vd-theme-muted)`, `--infra-line` → `var(--vd-theme-border)`, `--infra-panel` → `var(--vd-theme-panel-bg)`, `--infra-panel-strong` → `var(--vd-theme-panel-bg-strong)`, `--infra-app` → `var(--vd-theme-app-bg)`, `--infra-chrome` → `var(--vd-theme-control-bg)`, `--infra-accent` → `var(--vd-accent)`, `--infra-copper` → `var(--infra-warn)`, `--infra-good` → `var(--infra-ok)`, `#09251f` → `var(--vd-accent-contrast)`, `#77a4da` → `var(--infra-info)`, literal `rgba(...)` shadows → `var(--vd-theme-shadow-panel)` or `var(--vd-theme-shadow-modal)`.
  - Old line ranges to port:
    - lines 173–213: action reason, list editor, network segment, targets, conflict review, app links, service components;
    - lines 222–247: table, state pill;
    - lines 262–318: inspector body, with `.infra-detail-close { display: grid; }` always;
    - lines 319–376: dialogs, candidates, action list, hosted services.
  - Replace every `ui-monospace … uppercase` micro-label (`.infra-form-section h3`, table `th`) with `font: 600 13px/1.3 var(--vd-font-ui); color: var(--vd-theme-muted); text-transform: none; letter-spacing: 0;`.
  - Add `.infra-detail-kind { margin: 0 0 2px; color: var(--vd-theme-muted); font-size: 13px; font-weight: 600; }`.
  - Add `.infra-facts { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 4px var(--ds-space-3); margin: 0; font-size: 14px; } .infra-facts dt { color: var(--vd-theme-muted); } .infra-facts dd { margin: 0; overflow-wrap: anywhere; }`.
  - Add `.infra-inspector { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: auto; border-left: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }`.
  - Add `.infra-table-view { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; }`.

  **(g) Toasts:**
  ```css
  .infra-toasts { position: absolute; left: 50%; bottom: var(--ds-space-4); z-index: 25; transform: translateX(-50%); display: grid; gap: var(--ds-space-2); width: min(480px, calc(100% - 32px)); pointer-events: none; }
  .infra-toast { display: flex; align-items: center; gap: var(--ds-space-2); padding: var(--ds-space-2) var(--ds-space-2) var(--ds-space-2) var(--ds-space-3); border: 1px solid var(--vd-theme-border); border-left: 4px solid var(--infra-info); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-modal); font-size: 14px; pointer-events: auto; }
  .infra-toast > span { flex: 1; }
  .infra-toast[data-kind="success"] { border-left-color: var(--infra-ok); }
  .infra-toast[data-kind="error"] { border-left-color: var(--infra-bad); }
  .infra-toast-close { width: 28px; height: 28px; display: grid; place-items: center; border: 0; border-radius: var(--ds-radius-sm); background: transparent; cursor: pointer; }
  ```

  **(h) Responsive rules:**
  ```css
  @container infra (max-width: 1099px) {
    .infra-cmd-label, .infra-search-kbd, .infra-sync-label { display: none; }
    .infra-split > .infra-button:first-child { width: 36px; padding: 0; }
    .infra-app[data-inspector-open="true"] .infra-body { grid-template-columns: var(--infra-library-w) 4px minmax(0, 1fr); }
    .infra-app[data-library-collapsed="true"][data-inspector-open="true"] .infra-body { grid-template-columns: 40px 0 minmax(0, 1fr); }
    .infra-inspector { position: absolute; top: 0; right: 0; bottom: 0; z-index: 8; width: min(var(--infra-inspector-w), 92%); box-shadow: var(--vd-theme-shadow-modal); }
    .infra-splitter[data-splitter="inspector"] { display: none; }
    .infra-app[data-inspector-open="true"] .infra-scrim { display: block; }
  }
  @container infra (max-width: 759px) {
    .infra-history { display: none; }
    .infra-menu [data-overflow-only] { display: flex; }
    .infra-body, .infra-app[data-inspector-open="true"] .infra-body, .infra-app[data-library-collapsed="true"] .infra-body, .infra-app[data-library-collapsed="true"][data-inspector-open="true"] .infra-body { grid-template-columns: minmax(0, 1fr); }
    .infra-canvas, .infra-table-view { grid-column: 1; }
    .infra-library { position: absolute; top: 0; left: 0; bottom: 0; z-index: 8; display: none; width: min(320px, 92%); box-shadow: var(--vd-theme-shadow-modal); }
    .infra-app[data-library-sheet="open"] .infra-library { display: flex; }
    .infra-app[data-library-sheet="open"] .infra-scrim { display: block; }
    .infra-splitter, .infra-library-rail { display: none !important; }
    .infra-app[data-library-collapsed="true"] .infra-library-scroll { display: block; }
  }
  @container infra (max-width: 559px) {
    .infra-brand, .infra-live-status { display: none; }
    .infra-commandbar { gap: 4px; padding: 0 6px; }
    .infra-cmd-icon { width: 32px; height: 32px; }
    .infra-view-switch button { padding: 0 6px; }
  }
  @media (prefers-reduced-motion: no-preference) { body:not([data-animations="false"]) .infra-inspector { animation: infra-panel-in var(--ds-motion-duration-base) var(--ds-motion-ease-decel); } }
  @keyframes infra-panel-in { from { opacity: 0; transform: translateX(12px); } }
  @media (pointer: coarse) { .infra-cmd-icon, .infra-view-switch button { min-height: 40px; } .infra-button, .infra-library-row-main, .infra-library-place { min-height: 44px; } }
  ```

  **(i) Theme overrides:** delete the old `.desktop-body[data-theme=…]` overrides (old lines 378–388). Radii and blur come from tokens now.

- [ ] **Step 7: Replace the `## Infra` section** of `ui/js/desktop/apps/AGENTS.md` (current lines 106–124) with:

```markdown
## Infra

- Load order (module-loader `infra`): `infra-model.js` (pure helpers: viewport math, geometry, regions, drop rules,
  guests, filters, scene builder, layout intents, history), `infra-store.js` (state, API, polling and visibility,
  layout queue, undo), `infra-canvas.js` (keyed map rendering), `infra-gestures.js` (pointer and keyboard),
  `infra-library.js`, `infra-inspector.js`, `infra-table.js`, `infra-dialogs.js`, then `infra.js` (composition root).
  Styles: `desktop-app-infra.css` (shell, panels, dialogs) and `desktop-app-infra-canvas.css` (map). Every file stays
  below 1100 lines and no file touches the DOM at load time (the Node test runs them in a vm).
- World coordinates: an asset placement is the centre of its 184×60 card; a place placement is the top-left of its
  region plus `w`/`h`. Cards sit at `left`/`top` world px inside `[data-scene]`, which carries the only transform
  (`translate(x, y) scale(z)`, zoom 0.25–2).
- Every position, parent or size change goes through `store.dispatch` with an `InfraAppStore.commands` object. The
  store keeps one `PUT /layout` in flight per view, coalesces later moves, rebases once on 409 (fresh snapshot, same
  intents) and leaves a second failure unsaved behind `[data-action="retry-layout"]`. A conflict clears that view's
  history. Placing from the library is undoable when the server supports `unplace` (`features.layoutParent`); the revert queues `{unplace: true}` intents, which the queue sends as `{id, version, unplace: true}`.
- `PUT /layout` answers `{"nodes": [<changed nodes>]}` and is a patch: merge it through
  `InfraAppModel.applyLayoutPatch` (full nodes also set `parent_id`); only an explicit `snapshot` field replaces the
  atlas.
- The server rejects unknown body fields. `parent_id`, `w` and `h` go out only when the snapshot has `source_hosts`
  (`store.features.layoutParent`); node drafts never gain fields the snapshot did not send.
- Rendering is keyed by id; search and filters dim instead of re-rendering; a snapshot poll repaints the inspector
  only when its draft is clean and focus is outside it; gestures defer snapshot repaints until they end; keep the
  one-repaint-per-frame search guard.
- Verify `node scripts/test-infra-ui.mjs`, the UI line budget, the i18n and asset guards and
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopInfra`. The browser fixture answers `/layout` like the
  server (version CAS, patch reply); never return a full snapshot there.
```

- [ ] **Step 8: Update the fixture** in `ui/desktop_infra_browser_test.go`.
  - Add two fields to `infraAppBrowserFixture`:
    - `layoutCalls []map[string]any`: every PUT /layout body, in order.
    - `fail409Once bool`: the next PUT /layout bumps its first node's version by 5 and answers 409.
  - `snapshot()` returns `{"nodes", "edges", "sources": []any{}, "source_hosts": []any{}, "capabilities"}`.
  - Add the helper:
    ```go
    func infraFixtureNumber(value any) float64 {
    	switch v := value.(type) {
    	case float64:
    		return v
    	case int:
    		return float64(v)
    	case int64:
    		return float64(v)
    	}
    	return 0
    }
    ```
  - In `case path == "/edges" && r.Method == http.MethodPost:`, after decoding, mirror Plan A:
    ```go
    		if edge["from"] == edge["to"] {
    			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    			return
    		}
    		for _, existing := range f.edges {
    			if existing["from"] == edge["from"] && existing["to"] == edge["to"] && existing["kind"] == edge["kind"] {
    				write(http.StatusConflict, map[string]string{"error": "edge_exists"})
    				return
    			}
    		}
    ```
  - Replace the `/layout` case body (current lines 296–321) with:
    ```go
    		var payload struct {
    			View  string           `json:"view"`
    			Nodes []map[string]any `json:"nodes"`
    		}
    		if !decode(&payload) {
    			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    			return
    		}
    		f.layoutCalls = append(f.layoutCalls, map[string]any{"view": payload.View, "nodes": payload.Nodes})
    		byID := map[string]map[string]any{}
    		for _, node := range f.nodes {
    			byID[fmt.Sprint(node["id"])] = node
    		}
    		if f.fail409Once && len(payload.Nodes) > 0 {
    			f.fail409Once = false
    			if node := byID[fmt.Sprint(payload.Nodes[0]["id"])]; node != nil {
    				node["version"] = infraFixtureNumber(node["version"]) + 5 // another client changed it
    			}
    			write(http.StatusConflict, map[string]string{"error": "conflict"})
    			return
    		}
    		for _, change := range payload.Nodes { // the server validates the whole batch first
    			node := byID[fmt.Sprint(change["id"])]
    			if node == nil {
    				write(http.StatusNotFound, map[string]string{"error": "not_found"})
    				return
    			}
    			if infraFixtureNumber(change["version"]) != infraFixtureNumber(node["version"]) {
    				write(http.StatusConflict, map[string]string{"error": "conflict"})
    				return
    			}
    		}
    		patched := make([]map[string]any, 0, len(payload.Nodes))
    		for _, change := range payload.Nodes {
    			node := byID[fmt.Sprint(change["id"])]
    			placements, _ := node["placements"].(map[string]any)
    			if placements == nil {
    				placements = map[string]any{}
    				node["placements"] = placements
    			}
    			placement := map[string]any{"x": change["x"], "y": change["y"], "pinned": change["pinned"]}
    			if w, ok := change["w"]; ok {
    				placement["w"], placement["h"] = w, change["h"]
    			} else if previous, ok := placements[payload.View].(map[string]any); ok && previous["w"] != nil {
    				placement["w"], placement["h"] = previous["w"], previous["h"]
    			}
    			placements[payload.View] = placement
    			if parent, ok := change["parent_id"]; ok {
    				if value, _ := parent.(string); value == "" {
    					delete(node, "parent_id")
    				} else {
    					node["parent_id"] = value
    				}
    			}
    			node["version"] = infraFixtureNumber(node["version"]) + 1
    			patched = append(patched, node)
    		}
    		// Mirror the server: only the nodes this request changed are returned.
    		write(http.StatusOK, map[string]any{"nodes": patched})
    ```

- [ ] **Step 9: Update `TestDesktopInfraAppBrowser` deliberately.** Each change below states why.
  1. Lines 354–356 (empty state). The empty state is now `[data-empty]` with the illustration:
     ```go
     	return !create.hidden&&!create.disabled&&!importExisting.hidden&&!importExisting.disabled&&document.querySelector('[data-empty] img')?.naturalWidth>0;
     ```
  2. Line 379. The imported Docker container has no engine host, so it is listed under *Needs a host*, not *To place*:
     ```go
     	page.MustWait(`()=>!!document.querySelector('[data-library-item="imported-2"]')`)
     ```
  3. Lines 426–435. The old bands are gone, so the readable-layout check moves to the command bar, library and canvas tools. Section titles have a 13 px floor; the rest of the chrome keeps 14 px.
     ```go
     	assertReadableLayout := func(label string) {
     		got := page.MustEval(`()=>{
     			const sizes=[['brand','.infra-brand-name',14],['view switch','.infra-view-switch button',14],['search','.infra-search input',14],['library title','.infra-library-title',13],['library row','.infra-library-row-name',14],['library meta','.infra-library-row-meta',14],['zoom','.infra-zoom-value',14],['visible count','[data-visible-count]',14]];
     			const small=sizes.filter(([,selector,min])=>{const el=document.querySelector(selector);return !el||parseFloat(getComputedStyle(el).fontSize)<min;}).map(([name])=>name);
     			const overflow=['.infra-commandbar','.infra-library','.infra-inspector'].filter(selector=>{const el=document.querySelector(selector);return el&&getComputedStyle(el).display!=='none'&&el.scrollWidth>el.clientWidth+1;});
     			return JSON.stringify({small,overflow});
     		}`).Str()
     		if got != `{"small":[],"overflow":[]}` {
     			t.Fatalf("%s Infra text is too small or a key region overflows: %s", label, got)
     		}
     	}
     	page.MustWait(`()=>!!document.querySelector('[data-library-item]')&&!!document.querySelector('[data-detail-form]')`)
     ```
  4. Lines 447–451 and the screenshot on 452. Narrow windows now open the library as a sheet instead of hiding it:
     ```go
     	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='430px';win.style.height='700px';}`)
     	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth<500&&!!document.querySelector('[data-action="close-inspector"]')`)
     	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]').click()`)
     	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'&&getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
     	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
     	page.MustWait(`()=>{const place=document.querySelector('[data-place="manual-1"]');return getComputedStyle(document.querySelector('.infra-library')).display!=='none'&&!!place&&place.offsetParent!==null;}`)
     	assertReadableLayout("430px")
     	path := infraReviewArtifactPath(t, "430-narrow-library-sheet.png")
     	if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
     		t.Fatal(err)
     	}
     	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
     	page.MustWait(`()=>getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
     ```
  5. Lines 463–467 (`expectedVisible`). Guests and places are not cards:
     ```go
     		expectedVisible := map[int]int{25: 23, 250: 248, 1000: 998}[size]
     ```
     The `page.MustWait` with `[data-visible-count]` stays as it is.
  6. Replace the `size == 25` block (lines 468–531). It covers guests as shelf chips, the region, search dimming, translate+scale zoom and an inspector with no overflow:
     ```go
     		if size == 25 {
     			page.MustWait(`()=>{const images=[...document.querySelectorAll('[data-nodes] img')];return images.length>0&&images.every(image=>image.complete&&image.naturalWidth>0);}`)
     			if got := page.MustEval(`()=>JSON.stringify({cards:document.querySelectorAll('[data-nodes] [data-node]').length,guestCard:!!document.querySelector('[data-node="fixture-node-0001"]'),region:!!document.querySelector('[data-regions] [data-region="fixture-group"]'),viewBox:document.querySelector('[data-edges]').getAttribute('viewBox').slice(0,4),transform:document.querySelector('[data-scene]').style.transform.slice(0,10)})`).Str(); got != `{"cards":23,"guestCard":false,"region":true,"viewBox":"0 0 ","transform":"translate("}` {
     				t.Fatalf("map did not render 23 cards, the group region and a translate+scale scene: %s", got)
     			}
     			if !page.MustEval(`()=>{const button=document.querySelector('[data-toggle-host-services="fixture-node-0002"]'),shelf=button&&document.getElementById(button.getAttribute('aria-controls'));return button?.getAttribute('aria-expanded')==='false'&&shelf?.hidden&&!!shelf.querySelector('[data-hosted-service="fixture-node-0001"]');}`).Bool() {
     				t.Fatal("the host card did not expose a collapsed, accessible guest shelf")
     			}
     			page.MustEval(`()=>document.querySelector('[data-toggle-host-services="fixture-node-0002"]').click()`)
     			page.MustWait(`()=>{const button=document.querySelector('[data-toggle-host-services="fixture-node-0002"]');return button.getAttribute('aria-expanded')==='true'&&!document.getElementById(button.getAttribute('aria-controls')).hidden;}`)
     			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 23 {
     				t.Fatalf("expanding a host must not turn guests into cards; got %d cards", count)
     			}
     			page.MustEval(`()=>document.querySelector('[data-hosted-service="fixture-node-0001"]').click()`)
     			page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Public status'`)
     			if !page.MustEval(`()=>parseFloat(getComputedStyle(document.querySelector('.infra-card-name')).fontSize)>=16&&parseFloat(getComputedStyle(document.querySelector('.infra-card-meta')).fontSize)>=14`).Bool() {
     				t.Fatal("card names must stay at least 16px and their meta line at least 14px")
     			}
     			before := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0003"]');return n.style.left+'|'+n.style.top;}`).Str()
     			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='Cold-store NAS';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
     			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]:not(.is-dimmed)').length===1&&!document.querySelector('[data-node="fixture-node-0003"]').classList.contains('is-dimmed')&&document.querySelectorAll('[data-nodes] [data-node]').length===23`)
     			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
     			page.MustWait(`()=>!document.querySelector('[data-nodes] [data-node].is-dimmed')`)
     			if after := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0003"]');return n.style.left+'|'+n.style.top;}`).Str(); after != before {
     				t.Fatal("filtering moved the asset from its saved world position")
     			}
     			page.MustEval(`()=>{window.infraZoomOf=()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1]);window.infraZoomBefore=infraZoomOf();document.querySelector('[data-action="zoom-in"]').click();}`)
     			page.MustWait(`()=>infraZoomOf()>infraZoomBefore`)
     			page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
     			page.MustWait(`()=>infraZoomOf()===1&&document.querySelector('[data-zoom]').textContent.trim()==='100%'`)
     			page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
     			page.MustWait(`()=>document.querySelector('[data-view="topology"]').getAttribute('aria-pressed')==='true'`)
     			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
     			page.MustWait(`()=>document.querySelector('[data-view="atlas"]').getAttribute('aria-pressed')==='true'`)
     			if !page.MustEval(`()=>{const left=document.querySelector('[data-node="fixture-node-0003"]').getBoundingClientRect(),right=document.querySelector('[data-node="fixture-node-0002"]').getBoundingClientRect();return left.right<=right.left;}`).Bool() {
     				t.Fatal("cards overlap neighbouring assets at their saved positions")
     			}
     			for _, theme := range []string{"standard", "fruity"} {
     				for _, mode := range []string{"light", "dark"} {
     					page.MustEval(`(theme,mode)=>{const settings=auditTest.state.bootstrap.settings;settings['appearance.theme']=theme;settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;auditTest.applyDesktopSettings();}`, theme, mode)
     					page.MustWait(`(theme,mode)=>document.body.dataset.theme===theme&&document.body.dataset.fruityMode===mode`, theme, mode)
     					if theme == "standard" && mode == "dark" || theme == "fruity" && mode == "light" {
     						path := infraReviewArtifactPath(t, theme+"-"+mode+"-wide.png")
     						if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
     							t.Fatal(err)
     						}
     					}
     				}
     			}
     			page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0002"]').click()`)
     			page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Atlas hypervisor'`)
     			if !page.MustEval(`()=>{const root=document.querySelector('.infra-inspector');return root.scrollWidth<=root.clientWidth+1}`).Bool() {
     				t.Fatal("selected node inspector has horizontal overflow")
     			}
     ```
  7. Replace the `size == 250` block (lines 532–538). Saved world coordinates are used as they are; the scene no longer has a width:
     ```go
     		} else if size == 250 {
     			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 248 {
     				t.Fatalf("map has %d cards; want 248 (249 assets minus one guest)", count)
     			}
     			if left := page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0045"]').style.left`).Str(); left != "5390px" {
     				t.Fatalf("a far saved world coordinate changed: %s", left)
     			}
     ```
  8. In the 1000 block (lines 539–589), make these replacements. Keep the inventory, draft and inspector lines 567–588 unchanged.
     - **Lines 540–542** (count). Every asset renders as a card, the region, or a shelf chip:
       ```go
       			if got := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length+document.querySelectorAll('[data-regions] [data-region]').length+document.querySelectorAll('[data-guest-shelf] [data-hosted-service]').length`).Int(); got != 1000 {
       				t.Fatalf("1000-asset map rendered %d cards, regions and guest chips; want 1000", got)
       			}
       ```
     - **Line 549:** `page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===998`)`.
     - **Lines 550–556** (group collapse is gone). Replace them with the keyed-patching check:
       ```go
       			page.MustEval(`()=>{window.infraKeyedCard=document.querySelector('[data-node="fixture-node-0500"]');window.infraStamp=document.querySelector('[data-live]').dataset.refreshedAt;document.querySelector('[data-action="refresh"]').click();}`)
       			page.MustWait(`()=>document.querySelector('[data-live]').dataset.refreshedAt!==window.infraStamp`)
       			if !page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0500"]')===window.infraKeyedCard`).Bool() {
       				t.Fatal("a refresh with unchanged data rebuilt the card instead of patching it")
       			}
       ```
     - **Lines 558–561** (narrow check). The library sheet stays closed and the command bar must not overflow:
       ```go
       			if !page.MustEval(`()=>{const app=document.querySelector('.infra-app'),bar=document.querySelector('.infra-commandbar'),toggle=document.querySelector('[data-action="toggle-library"]');return app.clientWidth<500&&getComputedStyle(document.querySelector('.infra-library')).display==='none'&&getComputedStyle(toggle).display!=='none'&&bar.scrollWidth<=bar.clientWidth+1&&innerWidth>1500;}`).Bool() {
       				t.Fatalf("narrow Infra window did not switch to its compact layout: %s", page.MustEval(`()=>{const bar=document.querySelector('.infra-commandbar');return JSON.stringify({app:document.querySelector('.infra-app').clientWidth,library:getComputedStyle(document.querySelector('.infra-library')).display,bar:[bar.clientWidth,bar.scrollWidth]});}`).Str())
       			}
       ```
     - **Lines 564–566.** Import now lives in the Add menu:
       ```go
       			if !page.MustEval(`()=>{const trigger=document.querySelector('[data-action="add-menu"]');trigger.click();const item=document.querySelector('[data-action="import"]');const ok=!!item&&item.offsetParent!==null&&!item.disabled;trigger.click();return ok&&!!document.querySelector('[data-action="close-inspector"]')?.getAttribute('aria-label');}`).Bool() {
       				t.Fatal("narrow inventory must keep import reachable through the Add menu and a labelled inspector close control")
       			}
       ```
  9. Lines 591–612 (poll timers, hide, policy, close) stay unchanged.

- [ ] **Step 10: Verify.**
  - `node --check` on every infra JS file, then V-core.
  - `node scripts/test-infra-assets.mjs` must pass unchanged.
  - V-browser: all `TestDesktopInfra*` pass, including the regression file, which is unchanged.
  - Open `reports/infra-ui-review/*.png` and check them against §4/§12 of the spec: command bar on one line, canvas fills the window, no hard-coded colours in dark or light. Note what you saw in your task notes.
- [ ] **Step 11: Commit.**
  - Paths: `ui/js/desktop/apps/infra.js`, `ui/js/desktop/apps/infra-model.js`, `ui/js/desktop/apps/infra-views.js` (deleted), `ui/css/desktop-app-infra.css`, `ui/js/desktop/core/module-loader.js`, `scripts/test-infra-ui.mjs`, `ui/js/desktop/apps/AGENTS.md`, `ui/desktop_infra_browser_test.go`.
  - Subject: `feat(infra): switch to the canvas-first shell with command bar, library and keyed map`.

---

## Task 13: Canvas gesture browser test

**Files:** Create `ui/desktop_infra_canvas_browser_test.go`.

- [ ] **Step 1: Write the test.**

```go
package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func infraCanvasFixtureNode(id, kind, name, icon string, placement map[string]any, parent string) map[string]any {
	node := map[string]any{"id": id, "version": 1, "kind": kind, "name": name, "icon": icon,
		"profile":    map[string]any{"tags": []any{}, "interfaces": []any{}, "links": []any{}, "app_bindings": []any{}, "custom": map[string]any{}},
		"placements": map[string]any{}, "sources": []any{}, "available_actions": []any{}}
	if placement != nil {
		node["placements"] = map[string]any{"atlas": placement}
	}
	if parent != "" {
		node["parent_id"] = parent
	}
	return node
}

// TestDesktopInfraCanvasGesturesBrowser covers the canvas wiring: the map owns
// the window, a library row dropped into a region saves parent and position in
// one call, a card dragged out clears its parent, undo/redo replay the move,
// connecting by handle drag creates an edge, a layout conflict is rebased
// once, and keyboard move mode saves one move.
func TestDesktopInfraCanvasGesturesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 400, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-in-room", "device", "Media box", "mini-pc", map[string]any{"x": 640, "y": 300, "pinned": true}, "room-1"),
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-b", "device", "Core switch", "managed-switch", map[string]any{"x": 160, "y": 420, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-unplaced", "device", "Cold NAS", "nas", nil, ""),
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')&&!!document.querySelector('[data-library-item="dev-unplaced"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)

	center := func(selector string) (float64, float64) {
		point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, selector).Arr()
		return point[0].Num(), point[1].Num()
	}
	drag := func(fromX, fromY, toX, toY float64) {
		page.Mouse.MustMoveTo(fromX, fromY).MustDown(proto.InputMouseButtonLeft)
		page.Mouse.MustMoveTo((fromX+toX)/2, (fromY+toY)/2)
		page.Mouse.MustMoveTo(toX, toY)
		page.Mouse.MustUp(proto.InputMouseButtonLeft)
	}
	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	waitLayoutCalls := func(want int) []map[string]any {
		deadline := time.Now().Add(8 * time.Second)
		for {
			if calls := layoutCalls(); len(calls) >= want {
				time.Sleep(250 * time.Millisecond) // a stray extra save would land here
				return layoutCalls()
			}
			if time.Now().After(deadline) {
				t.Fatalf("waited for %d layout saves, got %d", want, len(layoutCalls()))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	itemFor := func(call map[string]any, id string) map[string]any {
		for _, item := range call["nodes"].([]map[string]any) {
			if item["id"] == id {
				return item
			}
		}
		return nil
	}
	snapshotRequests := func() int {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		count := 0
		for _, path := range fixture.requests {
			if path == "/api/desktop/infra/snapshot" {
				count++
			}
		}
		return count
	}

	// The map owns the window.
	if !page.MustEval(`()=>{const app=document.querySelector('.infra-app').getBoundingClientRect(),canvas=document.querySelector('[data-canvas]').getBoundingClientRect();return canvas.height>=app.height*0.75;}`).Bool() {
		t.Fatal("the canvas must own at least 75 % of the window height")
	}

	// A library row dropped into a region saves parent and position in one call.
	start := len(layoutCalls())
	rx, ry := center(`[data-library-item="dev-unplaced"] .infra-library-row-name`)
	gx, gy := center(`[data-region="room-1"]`)
	drag(rx, ry, gx, gy+40)
	calls := waitLayoutCalls(start + 1)[start:]
	if len(calls) != 1 {
		t.Fatalf("placing into a region made %d layout calls; want one", len(calls))
	}
	if item := itemFor(calls[0], "dev-unplaced"); item == nil || item["parent_id"] != "room-1" {
		t.Fatalf("the drop did not save the region as parent: %#v", calls[0])
	}
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-unplaced"]')&&!document.querySelector('[data-place="dev-unplaced"]')`)

	// A card dragged out of every region clears its parent.
	start = len(layoutCalls())
	cx, cy := center(`[data-node="dev-in-room"]`)
	drag(cx, cy, cx+320, cy)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "" {
		t.Fatalf("dragging out of the region did not clear the parent: %#v", calls[0])
	}

	// Undo restores position and parent; redo replays the move.
	start = len(layoutCalls())
	page.MustEval(`()=>document.querySelector('[data-action="undo"]').click()`)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "room-1" || item["x"] != float64(640) {
		t.Fatalf("undo did not restore the saved position and parent: %#v", calls[0])
	}
	page.MustEval(`()=>document.querySelector('[data-action="redo"]').click()`)
	calls = waitLayoutCalls(start + 2)[start+1:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "" {
		t.Fatalf("redo did not replay the move: %#v", calls[0])
	}

	// Dragging a connection handle onto another card offers the connection popover.
	ax, ay := center(`[data-node="dev-a"]`)
	page.Mouse.MustMoveTo(ax, ay)
	hx, hy := center(`[data-node="dev-a"] [data-connect-handle="s"]`)
	bx, by := center(`[data-node="dev-b"]`)
	drag(hx, hy, bx, by)
	page.MustWait(`()=>!!document.querySelector('[data-connect-popover]')`)
	if kind := page.MustEval(`()=>document.querySelector('[data-connect-popover] [data-connect-kind]').value`).Str(); kind != "ethernet" {
		t.Fatalf("router to switch should default to ethernet, got %q", kind)
	}
	page.MustEval(`()=>document.querySelector('[data-connect-popover] [data-action="create-connection"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-connect-popover]')&&!!document.querySelector('[data-edges] [data-edge="edge-1"]')`)
	fixture.mu.Lock()
	edges := append([]map[string]any(nil), fixture.edges...)
	fixture.mu.Unlock()
	if len(edges) != 1 || edges[0]["from"] != "dev-a" || edges[0]["to"] != "dev-b" || edges[0]["kind"] != "ethernet" {
		t.Fatalf("connect gesture saved %#v", edges)
	}

	// A layout conflict reloads once and resends onto the fresh version.
	fixture.mu.Lock()
	fixture.fail409Once = true
	fixture.mu.Unlock()
	snapshotsBefore := snapshotRequests()
	start = len(layoutCalls())
	ax, ay = center(`[data-node="dev-a"]`)
	drag(ax, ay, ax, ay-96)
	calls = waitLayoutCalls(start + 2)[start:]
	if len(calls) != 2 {
		t.Fatalf("a conflict produced %d layout calls; want the original and one rebased resend", len(calls))
	}
	first, second := itemFor(calls[0], "dev-a"), itemFor(calls[1], "dev-a")
	if first == nil || second == nil || second["version"].(float64) != first["version"].(float64)+5 || second["y"] != first["y"] {
		t.Fatalf("the resend must reapply the same intent onto the fresh version: %#v", calls)
	}
	if snapshotRequests() <= snapshotsBefore {
		t.Fatal("the conflict did not reload the snapshot before resending")
	}
	page.MustWait(`()=>document.querySelector('[data-layout-chip]').hidden`)

	// Keyboard move mode: M, two steps right, Enter saves one move.
	page.MustEval(`()=>{const card=document.querySelector('[data-node="dev-b"]');card.click();card.focus();}`)
	start = len(layoutCalls())
	page.Keyboard.MustType(input.KeyM, input.ArrowRight, input.ArrowRight, input.Enter)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-b"); len(calls) != 1 || item == nil || item["x"] != float64(208) || item["y"] != float64(420) {
		t.Fatalf("keyboard move mode saved %s", fmt.Sprint(calls))
	}

	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
```

- [ ] **Step 2: Verify.**
  - `cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign" && go vet ./ui`
  - Then `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfraCanvasGesturesBrowser' -count=1 -timeout 15m` must pass.
  - If the Windows animation setting makes it flaky, reproduce it both ways (project memory: Windows motion vs browser tests).
  - Then run the full V-browser.
- [ ] **Step 3: Commit.**
  - Paths: `ui/desktop_infra_canvas_browser_test.go`.
  - Subject: `test(infra): cover canvas drops, reparenting, undo, connect, rebase and move mode`.

---

## Task 14: Final verification and review handoff (no code)

- [ ] **Step 1: Run everything.**
  - V-core.
  - `node scripts/test-infra-assets.mjs`.
  - `go test ./ui -run '(TestDesktopInfra|TestDesktopUsedI18nKeys|TestFrontend_JS_NoLiteralTranslationFallbacks|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget)' -count=1`.
  - V-browser.
- [ ] **Step 2: Run** `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref "main" --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"` and attach the summary.
- [ ] **Step 3: Hand off for the single combined review** (Critical/Important only). Include:
  - the Task 0 baseline note;
  - the screenshots in `reports/infra-ui-review/`;
  - the known limitations from §0;
  - a confirmation that every file stays under 1100 lines (`wc -l ui/js/desktop/apps/infra*.js`).

Plan C (panels and flows) and Plan D (topology, polish, docs) build on the modules and hooks defined here.
