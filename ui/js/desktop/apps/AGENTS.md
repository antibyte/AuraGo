# Desktop App Modules - Child DOX Contract

## Camera

- A photo or video handoff reuses the desktop upload. The request sends `unique=1` and `If-None-Match: *`, then keeps the returned `Pictures/` or `Videos/` path. The same capture is not uploaded again. A closed window drops a late result.
- Photo opens Pixel with `forceNew: true`. Video passes `importPath` and `importToken` once. Those fields are not stored on the window session context, so a restored window does not import again.
- Video Studio asks for an existing project, a new project, or cancel before import. Cancel leaves the file in `Videos`. Import uses one `Idempotency-Key`, the media library only, and the existing dirty-save guards. A second handoff does not replace an open choice or unrelated dialog. Escape follows the same cancellation path as the dialog buttons.
- While Video Studio loads, the window record retains one transient offer outside its session context; rendering consumes it once.
- Verify `TestDesktopCameraHandoffBrowser`, `TestDesktopVideoStudioCameraImportBrowser`, `TestDesktopCameraHandoffIntegrationBrowser` and `TestDesktopVideoStudioStartMenuBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1`.

## Pixel

Pixel is the image editor (`window.AuraPixelApp` is its namespace, `window.PixelApp = { render, dispose }` its entry point).
`module-loader.js` key `pixel` loads the styles `pixel.css`, `desktop-stickers.css`, `pixel-stickers.css`, `pixel-layers.css`,
`pixel-filters.css`, `pixel-generators.css`, `pixel-cutout.css`, `pixel-selection.css`, `pixel-tools-pro.css` and
`pixel-smart.css`, and the scripts in the order given in the table below. Older notes in this file that say "layers (max
20)", describe a canvas-only history, or give the old 17-tool, 21-filter or 29-filter counts are obsolete. The current
limits are 64 layers and 60 history steps, and the catalogue holds 80 filters and 43 generators.

### Module structure

The loader starts with `shared/sticker-catalog.js` and `shared/sticker-library.js`, then:

| Layer | Files | Role |
|---|---|---|
| State and constants | `pixel-state.js` | `MAX_HISTORY` 60, `MAX_LAYERS` 64, `BLEND_MODES`, `IMAGE_EXTS` (includes `ora`), tool SVGs, tool groups, default tool shortcuts, the shortcut normaliser (`normalizeShortcut`, `shortcutFromEvent`) |
| Core | `pixel-core.js` | Registries (`registerModule`, `registerPanel`, `registerTool`, `registerMenuItems`, `registerCommand`), layers and masks, composition with blend modes and clipping, history with shared snapshots, selection masks, `commitLayerPixels`, `runJob`, overlays, events |
| Libraries (DOM-free) | `pixel-lib-core.js` (`AuraPixelLib` helpers, core ops), `pixel-lib-fx.js` (filters, histogram, auto tone, fade), `pixel-lib-gen.js` (`GENERATORS` catalog and renderers), `pixel-lib-cutout.js` (classic matting, colour key, refine, decontaminate, blur), `pixel-lib-inpaint.js` (PatchMatch fill) | Pure functions on `{ width, height, data }` images. They load in a window, in the worker and in a node `vm` |
| Worker | `pixel-worker.js` | Classic Worker, created lazily by `runJob`; NOT listed in the loader. It `importScripts` every `pixel-lib-*.js` (`TestPixelWorkerImportsEveryLib` keeps the list complete). Protocol `{ id, op, args }` → `{ id, progress }` / `{ id, result }` / `{ id, error }` |
| Shell | `pixel.js` (windows, instances, wiring, dispose), `pixel-view.js` (markup, the parameter-controls kit `defaultParams` / `buildParamControls` / `wireParamControls`, built-in panels, tools and menus), `pixel-events.js` (pointer and key dispatch, shortcuts modal, context menu) | The shell. Built-in tools, panels and menus are registered in `pixel-view.js`, and the brush tools in `pixel-brush.js` |
| Legacy paths | `pixel-canvas.js`, `pixel-tools.js`, `pixel-actions.js` | Pre-core composition, file I/O, the provider AI calls (`/api/pixel/generate`, `enhance`, `remove-bg`, `upscale`). Newer modules override parts of them; do not add features there |
| Feature modules | `pixel-brush.js` (stroke engine, `toolUi`, Brush tab), `pixel-tools-pro.js` (shapes, gradient, bucket, retouch, hand, zoom, grids), `pixel-transform.js` (move, free transform, crop), `pixel-selection.js` (selection tools, Select menu, `selectionKit`), `pixel-layers.js` (Layers dock, Layer menu), `pixel-filters.js` (Filters and Adjust tabs, Filter menu), `pixel-generators.js` (Generate tab and menu), `pixel-cutout.js` (Cutout tab, AI, classic and colour-key engines, cutout brush), `pixel-smart.js` (content-aware fill, remove-object tool, Colors tab, trim, auto-crop, canvas size), `pixel-commands.js` (command palette, fuzzy matcher, synonyms), `pixel-ora.js` (OpenRaster write and read, Save and Open glue), `pixel-stickers.js` (sticker layers) | Feature modules call `Pixel.installX`/`registerModule` and register panels, tools, menu items and commands. They do not edit the shell. A module whose installer throws is logged and skipped, so a test must catch it |

Pixel scripts are classic scripts in the `window.AuraPixelApp` namespace (`Pixel`). Module installers run once per window,
after the built-in installers and before the markup exists. DOM work belongs in a panel `mount` or in `r.on('ready', …)`.

### Contracts

- **QR insertion.** `pixel-qrcode.js` registers immediately after `pixel-core.js`
  and adds Generate/menu-palette entries. It lazily opens shared key `qr`, captures
  the document, layers, history entry and selection, and rejects stale results.
  Insert a normal named document-sized raster layer with `commitLayerPixels`
  (`respectSelection:false`, `history:false`), then one `pushHistory`; existing
  layers and the selection descriptor survive. Placement uses bounded pixel
  dimensions/offsets and the preview aspect ratio. Existing transforms and
  OpenRaster own later editing/persistence; never add a parametric QR layer type.
  Decode the scaled PNG and original preview with the shared scanner before
  creating the layer; their complete payloads must match. Reject unreadable sizes
  without changing layers/history and recheck document and cancellation after
  verification. Commit the same verified pixels without resampling again.
  Wrap the existing `disposeCore` to abort the dialog before core cleanup.

- **Layers.** `layer = { id, name, canvas, visible, opacity, blend, lockPixels, lockAlpha, lockPosition, maskCanvas,
  maskEnabled, clip, rev, maskRev, sticker?, stickerImage? }`. Every layer owns a document-sized canvas. Create layers with
  `createLayer`, mark them with `markLayerDirty(layer, { mask })`, read through `getActiveReadCtx()` and write through
  `getActiveCtx()`. Do not reintroduce a null background canvas.
- **Pixel changes.** Route every pixel change through `commitLayerPixels(layer, source, { label, respectSelection, mask,
  history })`. It applies the feathered selection, refuses `lockPixels`, keeps the alpha under `lockAlpha` and writes
  history. The exception is an incremental stroke tool, which draws into the stroke buffer (`brush.js` `openSession`) and
  calls `pushHistory` on release.
- **History.** `pushHistory(label)` stores `{ label, width, height, activeLayerId, layers: [{ meta, snap, maskSnap }],
  selection }`. A `snap = { rev, canvas }` is shared with the previous entry while the layer's `rev` is unchanged.
  `restoreHistory` copies snapshots back into fresh canvases. Undo, redo and jump go through it. `HISTORY_BYTE_BUDGET` caps
  the unique snapshot memory at 768 MiB.
- **Selection.** `r.selection` is a descriptor (`rect`, `ellipse` or `mask`). Assign a new object instead of mutating one.
  `getSelectionMask()` returns `{ alpha, x, y, w, h }` (cached). `setSelectionMask(alpha, { mode })` takes `replace`, `add`,
  `subtract` or `intersect`.
- **Long work.** Anything per pixel × radius, iterative, or a preview goes through `r.runJob(op, args, { key, signal,
  onProgress })`. A second job with the same `key` replaces the first, which rejects with `AbortError`. The pool has two
  workers; `Worker` missing means a synchronous fallback. Ops are registered with `AuraPixelLib.registerOp(name, fn)`.
- **Registries.** `Pixel.registerPanel({ id, labelKey, fallback, order, dock, build, mount, onShow, onHide })`,
  `Pixel.registerTool({ id, group, svg, shortcut, labelKey, fallback, raster, options, wireOptions, onDown, onMove, onUp,
  onHover, onKey, onActivate, onDeactivate, overlay })`, `Pixel.registerMenuItems(menuId, items, { order })` for the menus
  `file`, `edit`, `image`, `layer`, `select`, `filter`, `generate`, `view` and `ai`, and `Pixel.registerCommand({ id,
  labelKey, fallback, keywords, group, shortcut, run, enabled })`. Re-registering an id replaces the definition.
- **Shortcuts.** A shortcut is a property of a menu item (`shortcut` or `shortcuts`), a tool (`shortcut`) or a command.
  Ctrl and Meta are the same modifier, letters are case-folded, and the newest enabled command wins. Shortcuts are ignored
  inside inputs, after `defaultPrevented`, and in windows that are not the active Pixel window.
- **Strings.** `t('pixel.key', 'English fallback')` (or `tx(r, key, fallback)` in the modules). Every key must exist in all 16
  `ui/lang/desktop/*.json` files (`TestDesktopUsedI18nKeysExistInAllLanguages`).
- **Parameter schemas.** `r.defaultParams`, `r.buildParamControls` and `r.wireParamControls` take fields `{ key, type,
  labelKey, fallback, default, min, max, step, unit, options, showIf }`. Types are `range`, `int`, `select`, `segmented`,
  `bool`, `color`, `angle`, `seed`, `point`, `gradient` and `custom`.
- **OpenRaster.** `Pixel.ora.write(layers, w, h)` and `Pixel.ora.read(buffer, { maxLayers })` in `pixel-ora.js`. The server
  (`internal/server/pixel_ora.go`) validates every archive again. Keep its limits and the client's in step: 32 MiB per file,
  128 entries, 512 MiB uncompressed, and 64 layers when opening.

### Server and AI models

- Routes are in `internal/server/server_routes.go` (`/api/pixel/config`, `generate`, `enhance`, `remove-bg`, `upscale`,
  `save`, `cutout/status`, `cutout/prepare`, `cutout`). Handlers are in `pixel_handlers.go`, `pixel_cutout.go`,
  `pixel_ora.go` and `pixel_files.go`.
- The Cutout tab is the only path that downloads AI models, and only after the user clicks **Download model**. The matting
  package is `internal/pixelmatte` (`internal/pixelmatte/AGENTS.md`). The classic engine and colour key run in the worker.

### File ownership

- Shared (change with care, run the full Pixel suite): `pixel.js`, `pixel-state.js`, `pixel-core.js`, `pixel-view.js`,
  `pixel-events.js`, `pixel-lib-core.js`, `pixel-worker.js`, `ui/js/desktop/core/module-loader.js` (key `pixel`).
- Feature areas own their files: layers → `pixel-layers.js`; filters → `pixel-filters.js`, `pixel-lib-fx.js`; generators →
  `pixel-generators.js`, `pixel-lib-gen.js`; cutout → `pixel-cutout.js`, `pixel-lib-cutout.js`; selection →
  `pixel-selection.js`; brush and retouch → `pixel-brush.js`, `pixel-tools-pro.js`; transform → `pixel-transform.js`;
  intelligence → `pixel-smart.js`, `pixel-commands.js`, `pixel-lib-inpaint.js`; project files → `pixel-ora.js`; stickers →
  `pixel-stickers.js`; QR insertion → `pixel-qrcode.js`. Styles are
  `ui/css/pixel*.css`, one per feature area; QR chrome uses shared lazy
  `ui/css/desktop-qr.css`.
- Files that other apps read: `menus-and-routing.js` (`desktopIsImageFile`, bundled into
  `ui/js/desktop/bundles/main.bundle.js`) and `editor-filemenu.js` (`isPixelImageFile`). After editing either, run
  `npm run build:ui` and `npm run check:ui`.

### Tests

- `npm run test:pixel` runs `node scripts/test-pixel-all.mjs`, which runs every `scripts/test-pixel-*.mjs` and reports
  all failures at the end. `test-pixel-core.mjs` is a fake-DOM harness that loads every script and runs `render()`, so it
  catches module install errors. Run a single area with `node scripts/test-pixel-<area>.mjs` (areas: `core`, `lib`,
  `tools`, `layers`, `selection`, `fx`, `gen`, `cutout`, `inpaint`, `ora`). Syntax-check edited files with `node --check`.
- `go test ./ui -run '(TestPixel|TestDesktopPixel)' -count=1` runs the Go source-contract tests in `ui/desktop_pixel_*_test.go`
  (wiring, loader order, registrations, strings, CSS rules). Both name prefixes are used.
- `go test ./internal/server -run TestPixel -count=1` and `go test ./internal/pixelmatte -count=1` cover the server.
- Do not run the full `./ui` suite for Pixel changes. The browser smoke is manual.

## Infra

- Load order (module-loader `infra`): `infra-model.js` (pure helpers: viewport math, geometry, regions, drop rules,
  guests, filters, scene builder, layout intents, history), `infra-lanes.js` (Topology lanes, auto layout; `window.InfraAppLanes`,
  `app.lanes`), `infra-store.js` (state, API, polling and visibility,
  layout queue, undo), `infra-canvas.js` (keyed map rendering), `infra-gestures.js` (pointer and keyboard),
  `infra-library.js`, `infra-inspector.js`, `infra-table.js`, `infra-dialogs.js`, then `infra.js` (composition root).
  Styles: `desktop-app-infra.css` (shell, panels, dialogs) and `desktop-app-infra-canvas.css` (map). Every file stays
  below 1100 lines and no file touches the DOM at load time (the Node test runs them in a vm).
- World coordinates: an asset placement is the centre of its 216×64 card; a place placement is the top-left of its
  region plus `w`/`h`. Cards sit at `left`/`top` world px inside `[data-scene]`, which carries the only transform
  (`translate(x, y) scale(z)`, zoom 0.25–2).
- Every position, parent or size change goes through `store.dispatch` with an `InfraAppStore.commands` object. The
  store keeps one `PUT /layout` in flight per view, coalesces later moves, rebases once on 409 (fresh snapshot, same
  intents) and leaves a second failure unsaved behind `[data-action="retry-layout"]`. A conflict clears that view's
  history. Placing from the library is undoable when the server supports `unplace` (`features.layoutParent`); the
  revert queues `{unplace: true}` intents, which the queue sends as `{id, version, unplace: true}`. A command with a
  `partial()` hook (`setParent`, which writes node by node) keeps a history entry for the nodes it wrote before it failed.
  The connection dialog reads the stored version of the connection at Save and reloads on a 409, like the inline form.
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
- Full load order: `infra-model.js`, `infra-lanes.js`, `infra-store.js`, `infra-picker.js`, `infra-canvas.js`, `infra-gestures.js`,
  `infra-library.js`, `infra-cleanup.js`, `infra-inspector-forms.js`, `infra-inspector.js`, `infra-table.js`,
  `infra-dialogs.js`, `infra-import.js`, `infra-menus.js`, `infra.js`. The module-loader list and the file list of
  `scripts/test-infra-ui.mjs` must match it.
- Plan C modules: `infra-picker.js` (ARIA combobox, popover and inline; options carry `data-app-target`,
  `data-node-option`, `data-place-option`), `infra-cleanup.js` (Cleanup/Ignored from `InfraAppModel.infraCleanup`,
  dismissals in `infra.dismissed.v1`), `infra-inspector-forms.js` (Details/Sources forms; all four tab panels are
  rendered and inactive ones hidden; app bindings live in the draft), `infra-import.js` (non-modal import sheet,
  `POST /import/batch`, `discoveryRelevance` groups; `InfraAppImport.dropItems` builds the items of a drop: the placement is
  `pinned: true` like a drag, and a discovery bound to an asset that is already placed in the view sends neither placement nor
  `parent_id`, because an import never enters the undo history; the asset is selected and revealed where it stands),
  `infra-menus.js` (context menus through `ctx.showContextMenu`,
  the inspector's more menu, the shortcut sheet and the legend).
- Snapshots always ask `?include_ignored=1`; ignored sources never count in the ledger and appear only under
  Ignored. Ignore, remove-and-ignore, engine hosts and batch import go through the store, which reloads the
  snapshot and says so on an uncertain outcome. `unplace` intents make library placing undoable (only when the
  snapshot has `source_hosts`). Inspector tab per kind persists in `infra.inspector.v1`, List sort in `infra.table.v1`.
- Menus: items are built when the menu opens, writes carry `disabledHint` in read-only windows, and every item hands
  the focus back to its anchor before it acts (the import sheet remembers that element). Shift+F10 and the
  context-menu key open the same menu; `?` (shortcut sheet) and those keys ignore text fields. Do not add global
  Delete or Ctrl A handlers: the table and the import sheet keep their own.
- Selections made with the List checkboxes carry `state.selectionOrigin === 'check'`: in the List view the inspector
  stays closed for them and the bulk bar is the surface (below 1100 px the inspector is an overlay whose scrim would
  cover it). Every other `store.select` resets the origin. Clicking an asset's name still opens its panel.
- Design QA batch D-fix-1 (amendments A1 to A16):
  - Cards are 216x64 (`CARD_W`, `CARD_H`); a stacked card lands 72 px away (`freeSlot` gap 8), `compactGrid` cells are
    240x96 and Arrange/Topology fall back to a 252x160 grid from (126, 100). The status glyph and the power dot ride on the icon.
  - Semantic zoom lives in `InfraAppModel.zoomLevels(zoom, previousForm)`; `paintLevels` in infra-canvas.js applies it once
    per level change. Classes on `[data-stage]`: `is-compact` (below 0.65, kept until 0.68), `is-tiny` (below 0.4),
    `is-no-handles` (below 0.5), `is-zoom-low` (edge labels, below 0.75), `is-form-switching` (the fade, only while
    `app.animationsEnabled()`). Names, region headers and lane labels are counter-scaled with a transform whose factor is the
    CSS variable `--infra-name-k`, `--infra-header-k` or `--infra-lane-k` on `[data-scene]` (12, 13 and 13 px on screen at
    the least); never change their font size per frame. `--infra-ring` keeps a compact icon's status ring visible. The
    compact form changes no geometry.
  - The minimap is a 168x112 panel without text; `model.sceneFits` hides it while the scene fits. Its toggle sits in the
    tools cluster; the shown count is the toggle's `title` and the cluster's `aria-description`.
  - A connection label that would sit on a card gets `is-label-clash` (`model.edgeLabelClash`) and shows on hover or
    selection only; labels are hidden with `visibility` so they can be measured.
  - Command bar: `fitFilterChips` shows the chips that fit and a `+N` chip (`data-filter-overflow`) that opens the filter
    popover, the search keeps 220 px from 760 px up; below 560 px the search is the `open-search` icon and opens over the bar
    (`data-search-open`, `app.focusSearch()`, which Ctrl K calls and which keeps the event from the desktop's Spotlight).
  - `model.sourceLabels` numbers several instances of a source kind ("Docker 1"); the instance id only goes into a title.
    `model.APP_LINK_TARGETS` is the one list of linkable apps.
- Topology lanes (Plan D, amended by D-fix-2, A17 to A27). The pure helpers live in `infra-lanes.js` (`window.InfraAppLanes`,
  `app.lanes`); the model no longer knows lanes (`buildScene` returns no `lanes`, its `bounds` are the drawn cards only).
  - Lanes: four fixed world bands, `LANE_H` = 384 px from y = 0 (internet, network, compute, clients), `laneAt(y)` =
    clamp(floor(y / 384), 0, 3). They are drawn as swimlane stripes in `[data-lanes]` (the first scene layer,
    `aria-hidden`, pointer-events none): static elements 100000 px wide (`LANE_EXTENT` = 50000 each side of x = 0), an
    alternating neutral fill and a separator in `--infra-outline`. They never take part in bounds, fit, the minimap or hit
    tests. Labels are sticky: `paintView` calls `paintLaneLabels`, which sets `--infra-lane-x` (and `--infra-lane-w`) on
    the lanes layer once per frame so the pill sits 12 px from the viewport's left edge; `--infra-lane-k` counter-scales it.
  - A card saved `pinned: true` is in lane `laneAt(y)`; any other card is in `topologyLane(node)`: the network kind, the icon
    (`LANE_ICONS`, real ids of the icon catalog: servers, storage, NVR, database, UPS and PDU are compute, VPN and firewall
    network, modem-ont internet), words in the role, "has guests", else clients. Auto columns sit `LANE_CELL_W` = 240 px
    apart; a lane has at most two rows (centres at lane top + 96 and + 240, `LANE_ROW_H` = 144), columns per row =
    max(8, ceil(n / 2)), every row centred on x = 0.
  - Order inside a lane (A20): `topologyLayout` starts from the connection order (breadth first over Ethernet, fiber and Wi-Fi)
    and reorders each lane by the barycentre of the neighbours in the other lanes (one sweep down, one up; a card whose links
    stay in its lane borrows their key, unconnected cards go last, ties keep the connection order). `ordering: 'connection'`
    gives the plain order. It is deterministic; the Node test pins the crossing counts of the lab fixture (11 and 5).
  - Placements: Topology ignores a saved `pinned: false` placement (`store.effectivePlacement` returns null for it, A22): it is a
    leftover of the retired layered Arrange. The auto layout never writes: `topologyAutoPositions` slots every eligible asset
    without a placement (not a place, guest or hostless VM/container). The canvas memoizes it (`autoPositions()`,
    `isAutoPlaced(id)`); the library tree and the inspector read it. In Topology the library has no To place section and the
    placed one is titled "In topology". Cards the auto layout moves glide (`is-gliding`, `startGlide`) only while
    `app.animationsEnabled()`.
  - Connections (A19, A28): `topologyPaths(lines, cardAt, ids)` is the one source of every Topology path. Between lanes it is a
    cubic from the bottom edge of the upper card to the top edge of the lower one (control offset max(48, dy / 2)). Inside a
    lane it is the straight segment between the facing sides while no other card is in the way (a drawn card whose box, padded
    by 4 px, touches the segment; `ids` lists every drawn card, so pinned cards count by position and direct neighbours
    stay straight); otherwise it is an arc above the row from the top edge of the left card to that of the right one, rising
    clamp(24 + 0.12 dx, 32, 56) px. The connections on one card edge spread over the middle 60% in slots; on a top edge the
    arcs to the left come first, then the curves arriving from the lane above, then the arcs to the right (each group by the
    other end's x), so arcs never cross arrivals. A slot is kept for every connection of the view, so nothing moves when a
    hidden one appears (A29). The result carries the label anchor `mx, my` (the path's midpoint; `edgeLabelClash` reads it)
    for the A5 clash rule. The canvas draws it for the line, the hit area and the selection, and redraws all of them while a
    card is dragged. Cross-lane curves do not avoid cards of the lanes between. The Map view stays straight.
  - Gestures: a drag or keyboard move highlights its lane (`setLaneTarget`) and announces a lane change. The first move of
    an auto-laid card undoes back to the automatic layout (`{unplace: true}` in `previousIntent`; without unplace support it
    is not undoable). Arrange in Topology (A22) is no layout: it hands the selection, or every card with a saved pin, back to the
    automatic layout as one undoable unplace save (`arrangeIds`, `removeFromMap`); it reads "Arrange automatically" and is
    disabled, with a reason title, without `store.features.unplace` (menu item, window menu and the inspector button).
  - Fit keeps 104 px below the cards (`FIT_PADDING`, `fitView` takes a padding object) so the tools and the names of compact
    cards stay clear. The compact icon is counter-scaled by `--infra-icon-k` (A24: `zoomLevels().icon`).
  - The empty-result action names its cause (A26: `clear-search`, `reset-filters` or `clear-search-filters` on the no-match
    box) and a library section that a query or filter emptied says "No matches". `fitSearchBadge` hides the Ctrl K badge when
    the placeholder would not fit next to it (A27).
  - A query or filter whose matches are all off the map does not say "No asset matches": the box counts them
    (`infra_no_match_unplaced[_one]`) and offers a primary `show-match` action beside the cause's clear action. The count, the
    asset revealed come from the library's own rows, `store.unplacedMatches(view)`: nothing in Topology; in the Map the To place
    rows (`store.toPlace`, `model.toPlaceNodes`: every unplaced asset that is no guest and no hostless vm or container, plus a host
    whose guest matches, since a guest has no row of its own) by name, then the Needs attention rows (`store.attention`, which the
    library section uses too) of hostless guests that are not on the map (`model.unplacedAttention`). Show match, and Enter in the
    search when no placed match exists (`revealUnplacedMatch`), open the first of them in the inspector and show its library row
    selected and in view, in whichever section lists it (`library.showRow`); they never place it. Anything else that matches
    nothing the library lists (a hostless guest with nothing to attend to, a place in Topology) keeps `infra_no_match` without that
    action. Never count or reveal from a second predicate.
- Design QA batch D-fix-3 (A28 to A31; A28 is in the Connections bullet above):
  - Which connections show (A29/A30) is one pure answer, `model.edgeState({view, kind, id, ends}, {selection, selectedEdge,
    near, dimmed})` -> `{off, linked, faded, dimmed}`, and `paintEdges` (infra-canvas.js) only toggles the classes `is-off`,
    `is-linked`, `is-faded`, `is-dimmed` on the connection groups: no scene render, so a hover costs no layout. `ends` holds
    the two cards of a line and the two assets the connection joins (a guest rides in its host's card). The canvas keeps
    `hoverId` (pointerover on a card, guest chip, host badge or region header) and `focusId` (keyboard focus only, tested with
    `:focus-visible`, so a clicked card does not keep its connections open); both are the `near` set.
  - Topology draws ethernet, fiber, wifi and the membership kinds (`member_of`, `runs_on`) always; `powered_by` and `depends_on`
    are `is-off` (opacity 0, no hit area) until an end is selected, hovered or focused, or the connection itself is selected.
    The Map draws every kind. Hidden connections keep their path and their slots on the card edges, so nothing moves when one
    appears. The inspector's Connections tab lists all of them.
  - A selection (cards, guests, a place or one connection) makes the connections that touch it `is-linked` (accent stroke,
    moved last in `[data-edge-layer]` so they paint above) and the other visible ones `is-faded` (35 %); a hover only reveals.
    A connection with a dimmed (filtered out) end is `is-dimmed` (28 %, like the cards), whether or not it is drawn. The
    opacity change is a transition only while `app.animationsEnabled()` and reduced motion is not asked for.
  - Scene paint order (A31): `.infra-lane-label` has `z-index: 1`, so the sticky lane labels paint above the connection layer
    (the `svg` has no z-index) and below the cards (z-index 1, later in the scene); nothing but z-index decides this.
  - `.infra-canvas-empty` and `.infra-canvas-nomatch` are centred boxes on their own surface (`--vd-theme-panel-bg-strong`, the surface of the tools and the minimap, with
    the theme blur, as the minimap) above the scene; only their buttons take the pointer.
- Visual review fixes (Plan D Task 10/11, screenshots in `reports/infra-ui-review/plan-d/`):
  - Status colours: `--infra-ok/warn/bad` stay the raw tokens for dots and strokes. Text and rings use `--infra-ok-text`,
    `--infra-warn-text`, `--infra-bad-text` and `--infra-ok-ring`, `--infra-warn-ring`, `--infra-bad-ring`, which are
    `light-dark(color-mix(token, --vd-text), token)`: the dark schemes keep the token (the danger text and the link are only
    lightened a little), the light schemes darken it, because only the default theme and Minimal ship light variants of
    `--ds-color-*-fg` (Fruity light, Windows XP and Amiga inherit the dark values: a theme-token follow-up). Accent used as a
    text colour goes through `--infra-link`. Never use the raw tokens as text colour.
  - Surfaces that float over the canvas (tools, lane labels, minimap, empty and no-match boxes) use `--vd-theme-panel-bg-strong`;
    `--vd-theme-overlay-bg` is a modal scrim (the scrim and the dialog backdrop only).
  - The List's table view is a container (`infra-table`): fixed px column widths, the name takes the rest, and Hardware, then
    Power and Source, then Place, then Type, then Address drop below 1142, 1011, 807, 657 and 507 px so the List never scrolls
    sideways. Spacer rows have no colspan (a colspan would keep the hidden columns' width). An empty inventory or a query with no
    match shows `[data-table-empty]`.
  - While the import sheet is open (`data-import-open`), the tools cluster moves right of it. Compact card names sit on a chip in
    the app colour so connections leaving the bottom edge do not strike through them.
  - Field labels are never capitalised by CSS. The inspector header stacks the kind and the breadcrumbs without a separator.
    Its action button (`.infra-head-pills .infra-button`) has no fixed height: a label too long for the 360 px panel wraps to
    centred lines inside the button (`TestDesktopInfraHeaderActionFitsBrowser` checks all 16 locales).
- Focus return (Plan D Task 7): `app.focusReturnTarget(el)` names where focus goes back when a panel opened from `el` closes (a
  menu or popover item hands over to the trigger of its menu through `MENU_TRIGGERS`; anything outside the window, or nothing,
  hands over to the map, or to the active view button in the List) and `app.restoreFocus(el)` focuses it, falling back to the
  same place when `el` is gone or hidden. The dialog host remembers it in `openDialog` and restores it from the dialog's
  `close` event (skipped while another dialog is already open); the import sheet does the same with its own opener. The narrow
  library sheet (`openLibrarySheet` / `closeLibrarySheet`) focuses its first row when it opens and its toggle when it closes
  (toggle, scrim, Escape inside it; an inspector opening over it closes it without moving focus); the toggle's
  `aria-expanded` follows the sheet below 760 px and the docked library above. Live regions: only `[data-announce]` polite,
  `[data-layout-chip]`, the draft and review banners (`role="status"`) and the inline errors (`role="alert"`); `[data-live]`
  has none, so a poll never speaks. A failing poll reports once (`loadSnapshot`, `explicit` always reports). The only
  `role="toolbar"` is the canvas tools, with a roving tab stop and arrows, Home and End.

## Docker Manager

- Load order (module-loader `docker-manager`): `docker-manager-model.js` (pure: container status and problem derivation,
  grouping by compose project, search and state filters, the multi-host merge, host health, the host form rules,
  error normalisation), `docker-manager-store.js` (every request through the desktop `api()` against `/api/docker-manager`,
  one request in flight per key, per-host container cache, polling only while the window is visible),
  `docker-manager-views.js` (HTML builders: rail, banners, table, empty states, detail drawer),
  `docker-manager-dialogs.js` (one native `<dialog>` with a view stack: host manager, add/edit form with Test connection,
  import from nest, confirmations), then `docker-manager.js` (composition root: skeleton, focus, keyboard, context menu,
  window menus, toasts). Style: `css/desktop-app-docker-manager.css`. Every file stays below 1100 lines and no file touches
  the DOM at load (`scripts/test-docker-manager-ui.mjs` runs them in a vm). Phase 1 shows Containers only; Stacks, Images,
  Volumes, Networks and Templates come in later phases, and no placeholder is rendered for them.
- The app appears only while Docker is enabled (`docker` capability, `desktopCapabilities` in `internal/server`). `ctx.readonly`
  (read-only desktop) hides container actions and host editing. `docker.readonly` hides the container actions only (host management
  stays): every `GET /hosts` answer carries `docker_readonly` (read per request), `loadHosts` sets and clears
  `state.dockerReadonly` from it, and a `read_only` 403 still sets it at once. The `aurago:desktop-policy` event counts only its
  `readonly` flag, never `enabled`. Docker disabled answers 503 `docker_disabled` on every route and shows one
  explanatory state with a link to `/config#docker`.
- Repaints compare the new HTML with the last one and skip equal markup; controls carry `data-focus-id` and `keepFocus`
  restores the focus on the same control, so a poll never steals it. Do not re-render the table from a timer. The table is a
  `treegrid` with a roving tab stop: arrows move, Left/Right collapse and expand a group, Enter opens the drawer, Delete asks to
  remove, Escape closes the drawer, `/` focuses the search, Shift+F10 opens the row menu.
- A failed container list keeps the last containers of that host and shows a banner with the reason code
  (`dns`, `refused`, `timeout`, `tls`, `auth`, `ssh_host_key`, `socket_missing`, `api_too_old`, `unknown`, texts
  `docker_manager.reason_*`); the browser giving up after 35 s counts as `host_unreachable` with reason `timeout`. A host
  status whose `checked_at` is the zero time (`0001-01-01…`) is "not checked yet", never a date. `GET /hosts` answers from cached
  statuses and refreshes stale ones in the background, so it is polled together with the container lists.
- Secrets are write-only. An edited host with stored credentials shows "stored" and a Replace action instead of empty secret
  fields; blank fields keep the stored value, the TLS PEM fields are sent only when typed (the server merges them one by one),
  and Test connection posts the draft with the host's `id` so the server reuses the stored secrets. A stored SSH secret follows the
  endpoint it was saved for: when the address (trimmed, case-insensitive, no IPv6 brackets), the effective port (empty = 22) or the
  SSH user changes, or the kind becomes SSH, the server refuses to reuse it (`invalid_host`, field `ssh_secret`) and the form
  (`storedSecretUsable`) asks for it again up front and says why; TLS material is kept. The dialog content is emptied whenever the
  view stack is empty, so a typed password or PEM key never stays in the closed `<dialog>`; `docker-manager.ui.v1` holds prefs only.
  Plain TCP carries a visible warning. Names are unique ignoring case and "Local" is reserved (checked in the form, enforced by the server).
- Protected containers (lock icon) ask before stop, restart, pause, kill and remove. The server never confirms the first four;
  a protected remove answers 409 `container_protected_confirmation_required` with `owner` and `message`, the dialog shows that
  message and the request is repeated with `?confirm=protected` (plus `&force=1` when forcing).
- Host colours are the eight `data-host-color` values mapped to theme tokens in the stylesheet; no literal colours, no
  uppercase labels, motion only inside `prefers-reduced-motion: no-preference` on `body:not([data-animations="false"])` (the
  Node test guards this like the Infra test). A modal `<dialog>` sits in the top layer, so toasts raised while it is open are
  moved inside it.
- Strings: `desktop.app_docker_manager` and `docker_manager.*` in all 16 `lang/desktop/*.json` files, local helper `t` with the
  `docker_manager.` prefix (registered in `desktopLocalI18nHelpers`; the key families built at run time are in
  `desktopDynamicI18nPrefixes` in `ui/i18n_lint_test.go`). A new string needs its key in all 16 files with the same
  `{{placeholders}}`; `TestDockerManagerLocalesAreComplete` and the Node test check this.
- Verify: `node scripts/test-docker-manager-ui.mjs` (or `npm run test:docker-manager`),
  `go test ./ui -run DockerManager -count=1`, and
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopDockerManagerBrowser -count=1`. After editing a core file
  (`module-loader.js`, `menus-and-routing.js`, `desktop-foundation.js`, `window-shell-runtime.js`) run `npm run build:ui` and
  `npm run check:ui`. API contract: `internal/server` (`/api/docker-manager`), host registry: `internal/dockerhost`.

## Synth Studio

- `synth-studio` is a native Creative app with the `synthStudio.*` locale prefix.
  Keep its eighty presets (five groups of sixteen), 128 synthesized GM programs and
  percussion bank local. See `documentation/synth-studio.md` for supported music
  formats, controls and resource limits.
- `synth-studio-workspace.js` owns workbench tabs, resizing, templates and local
  view preferences. Parameter gestures update existing audio buses and commit one
  undo step; cancellation restores the initial value. Structural edits wait for
  the next bar without restarting transport; cancel obsolete reserved voices even
  inside look-ahead. Stop/deadline takes priority over queued changes. Verify scheduler tests
  and responsive browser flows when changing these contracts.
- `synth-studio-music.js` owns deterministic scale/chord/arp/variation/rhythm
  transforms; `synth-studio-compose.js` owns preview/apply and held live inputs.
  Preview/cancel never changes the project; Apply commits ordinary notes once.
  Format v2 stores harmony, bar-range scenes and track automation; all automation
  counts toward the event/byte limits. v1 validation preserves v1. Upgrade in
  memory and require a create-only v2 copy before enabling autosave.
- `synth-studio-performance.js` owns pure scene expansion, sustain-to-duration
  conversion, automation reduction and take limits. `synth-studio-stage.js` owns
  scene pads, XY/manual override, point editing and temporary take recovery.
  Takes preserve their source and become distinct unsaved projects. Bound them by
  64 bars and the same renderer/project budgets before scheduling; keep captured
  material on stop or failure. Store drafts through OfficeSession with conditional
  removal, and start recovery only after initial project loading completes.
- Tone/reverb automation uses arrangement ticks, native AudioParam smoothing and
  at most 20 capture samples/second plus gesture endpoints. Manual input suppresses
  that parameter's automation until release. Scene changes release held/sustained
  notes, chase controllers and retain effect tails; Stop cancels future actions.
- Model validation owns project trust boundaries. Notes/controllers use clip-local
  ticks; clips use arrangement ticks. Live playback and offline WAV rendering
  share the native Web Audio implementation. Never schedule musical timing from
  animation frames or copy an upstream live-only synthesizer into offline export.
  `synth-studio-voices.js` (patches, mix buses, voices, controllers) loads before
  `synth-studio-audio.js` (project timeline, live transport, WAV render) in the
  `module-loader.js` asset list and the browser fixtures.
- Reuse OfficeSession drafts/write queues and the conditional Desktop file API.
  Preserve the latest dirty revision on failures; fence asynchronous file, MIDI
  and render results after a project change, permission revocation or disposal.
  A reopened project is paused. Desktop minimization keeps playback; browser-tab
  hiding pauses it. Closing releases notes, input ports, timers and audio nodes.
- MIDI is input-only, explicitly permission-gated with SysEx disabled. The Desktop
  document alone receives `midi=(self)`; untrusted sandboxed app frames deny MIDI.
  Read-only policy changes stop recording and block all project mutations.
- Verify the model/storage checks, `TestDesktopSynthStudioBrowser`,
  `TestDesktopSynthStudioAudioBrowser` and
  `TestSynthStudioMIDIPolicyOnlyAllowsDesktop`. Keep all sixteen locales and both
  Desktop themes usable, including compact library/sound-panel controls.

## AuraGolf

- `minigolf` is a native Games app. `minigolf.js` owns window instances and lazy
  imports the isolated bundle from `js/vendor/minigolf/`. Build it with
  `node scripts/build-minigolf.mjs`; never reuse another app's renderer.
- Scene, fixed-step Rapier physics, round model, views and audio have separate
  modules. Visible obstacles use physics poses. Keep one active ball, transparent
  non-colliding opponents, true cup openings and collision geometry from the
  exported Blender scene. Authoring sources live in `assets/minigolf/` (README and
  `kit/AGENTS.md`); regeneration is deterministic: `generate.py -- --only kits,courses`
  (plus `--course N` or `--only previews`) under Blender 5.2.2. On Windows rerun a
  courses build that hits a transient export lock until it reports `Wrote 18 courses`,
  then validate with `node scripts/test-minigolf-assets.mjs`.
- UI modules: `minigolf-shared.js` is DOM- and three-free (palette,
  `MAX_NAME_LENGTH` = 24 code points, `POWER_STOPS`, `FLOOR_SURFACES` for physics
  and the map, `createQualityGovernor`). `minigolf-views.js` builds the shell,
  imports `-hud`, `-menus`, `-scorecard` and `-tutorial`, and merges the menus and
  scorecard functions into its view; `-hud` imports `-minimap`. UI factories receive
  `tr`, `esc` and `icon` as arguments. Register each new `tr` module in
  `desktopLocalI18nHelpers` (`ui/i18n_lint_test.go`).
- P2 renderer modules: `minigolf-looks.js` (pure data: `LOOKS`, `TIERS`,
  `lookOf()` reading the manifest `world`, `placementMatrix()`),
  `minigolf-camera.js` (pure intro-flight maths), `minigolf-sky.js` (per-look sky
  dome, moon stars and Earth, `createEnvironment()` PMREM), `minigolf-water.js`
  (open disc and the `water-shore` material), `minigolf-props.js` (kit materials
  from glTF extras, `buildProps()` instancing, the shared shader clock) and
  `minigolf-fx.js` (fixed particle pools, ball trail, rings, fireworks).
  `minigolf-scene.js` stays the owner: it fetches and caches kits and environments,
  loads courses, runs the composer and exposes the public API. New modules import
  only `three`, `minigolf-shared.js` and each other; the pure parts are covered by
  `scripts/test-minigolf-look.mjs`.
- P3 physics modules, DOM- and three-free so Node tests cover them. `minigolf-collision.js`
  (`readColliders(glb)`, `colliderDigest(groups)`) reads physics meshes from the course GLB:
  mesh nodes with the extra `aurago_collider` (`grass|sand|wood|stone|metal`), world
  transform through TRS/matrix and the parent chain in the term order of
  `export.glb_fixed_colliders`, bit-identical Float32 vertices welded per surface in triangle
  order, collapsed triangles dropped (their vertices stay), groups in that surface order as
  typed arrays. The digest (contract V1 in the P3 plan) must equal the Python
  `glb_collision`. `minigolf-mechanics.js` imports nothing: obstacle poses
  (`rotate|slide|spin|gate|boulder`), `validateObstacle|Mechanic` and the zone logic
  (bumper kick, boost, teleport, cannon) that the live step and `predict()` share through a ball
  adapter. Physics accepts typed mesh arrays, `course.gravity` (finite, 0 < g <= 20, default
  9.81), `metal` rails (restitution 0.80), obstacle parts `box|ball|cylinder` and `mechanics`.
  Only `rest|cup|water|out` end a shot; `impact`, `bumper`, `boost`, `teleport` and `cannon`
  only report, and `predict()` returns `zone: 'teleport'|'cannon'|null`. Verify with
  `scripts/test-minigolf-collision.mjs` and the mechanics, loop prototype and seam cases in
  `scripts/test-minigolf-physics.mjs`.
- P3 scene and game: `overviewFor(course)` (`minigolf-camera.js`, pure) returns the overview
  pose, `shadowHalf`, `waterRadius` and `fogScale`. An island half-axis up to 5.5 m keeps the
  fixed (10, 13, 14) view of the tee/cup midpoint (shadow ±9, water 60, fog scale 1). A larger
  course is framed by its playable area, the plan box of `path` and every `corridors` line plus
  2.5 m on each side: the camera looks at the box centre from `clamp(3.4 · half extent, 21.6, 60)`
  m (a course without routes falls back to the island centre and half-axis). Shadow ±max(9, island
  half-axis + 1.5) and water radius max(60, 5 · island half-axis) still cover the whole island.
  `fogScale = min(1, 21.56 / distance)` multiplies the look's `FogExp2` density in `applyLook`, so
  the fog over the lane at the overview distance equals the P2 value (4.5 % pirate, 8.7 % temple).
  The scene follows the pose (orbit `maxDistance`, camera `far`, sun target at
  the island centre). `scene.load(course)` always reads the colliders from the GLB (manifest v2 has no
  `course.colliders`; an unreadable GLB or one without a `grass` collision throws `course_collision`) and resolves to
  `{ colliders }`; the game passes `{ ...course, colliders }` to physics and the lane map. Nodes with
  the extra `aurago_spin: { axis, speed }` turn about their local axis unless motion is reduced.
  `boost`, `teleport`, `cannon` and `bumper` events only play sound and effects (`spark` bursts,
  `scene.resetTrail?.()` on teleport, an extra `impact` burst on a bumper kick) and never end `busy`.
  Developer route search lives in `scripts/lib/minigolf-routes.mjs` (`loadCourses`, `runShot`,
  `progress`/`remaining` along `corridors`/`path`, `findRoute`, `timedRoute`, `replayRoute`) with the
  CLI `scripts/minigolf-route-search.mjs [--dir] [--course id|number|look-NN] [--json]`.
- `settings(values, track, focus)`, `audioSettings(settings, track)` and
  `credits(tracks)` in `minigolf-menus.js` are the stable entry points for the
  settings, sound and credits panels.
- The first-putt introduction lives in `minigolf-tutorial.js`: `tutorialNext()` and
  `tutorialView()` are pure, and `createCoach()` renders its single bubble.
- Controls cover keyboard, mouse and touch: held arrow keys scale with frame time
  (Shift keeps fine steps), a one-finger Aim drag putts from anywhere, and
  optional persisted settings add a four-degree aim assist, per-turn camera
  framing behind the ball and roll following — all still usable under reduced
  motion. Visuals add a dashed aim line with a color-graded power ring, penalty
  rest markers, name tags on ghost balls, a lane map with a persisted toggle, and
  the animated scorecard with grade badges, on top of the P2 look below.
- P2 look: `LOOKS` (exposure, sky, fog, sun, rim, environment intensity, bloom,
  water mode, FX palette) drives each world. The renderer uses
  `THREE.NeutralToneMapping` with per-look exposure, no hemisphere light, and
  `scene.environment` from the look's PMREM (kept per look for the scene's
  lifetime). The sun shadow camera covers ±9 m around the island (wider for large P3
  islands, see `overviewFor` below). `TIERS` (read
  through `tierConfig(tier())`) sets pixel ratio, shadows, bloom, clouds, twinkle,
  water motion and glints, scatter, motion, particle caps and the trail; the
  governor still chooses `auto`. Low has no shadow maps and no composer: a blob
  shadow sits under the ball instead.
- Kits and course data: `props-<look>.glb` is fetched once per look per scene
  (`kitFor`, in parallel with the course fetch). A failed fetch resolves to null and
  the course plays without props. The kit owns the geometry; `buildProps()` makes one
  `InstancedMesh` per prop primitive with cloned, patched materials. A course switch
  disposes the props, sky, water and course tree; `dispose()` also releases the
  PMREM targets, pending kit trees and effects. Manifest use: `world` is the look
  (`lookOf()`: `pirate|temple|moon`, anything else falls back to `pirate`; the manifest has no `look`
  field), `path` drives the intro flight
  (no path, no flight), `props` are `[name, x, y, z, yaw, scale]` placements, and
  `worlds[].preview` feeds the menu cards. The game does not read `kits`; the scene
  reads `island` for framing. The game accepts only manifest `version` 2 with 18
  courses, `course-NN.glb` models and a string `collision.digest` per course; there is no v1 branch and no
  legacy world id (`garden|coast|night`) anywhere: `LEGACY_LOOK`, `WORLD_ALIAS` and the old icons and accents are gone.
- Node contract: `aurago_hidden` on the `rail-wood|stone|metal` collision nodes hides them at
  load (the collision stays in physics). `rail-trim` is plain decor. `water-shore` takes the
  shore material (`TEXCOORD_0.x` is the distance to the shore; animated foam). Only nodes
  with `aurago_spin` turn (teleporter rings, radar arm); the temple sun wheel is a static
  prop and the scene has no windmill pivot. `flag` waves on every tier unless reduced.
- Material extras (`patchMaterial`): `aurago_kind` picks the response. `foliage` and
  `cloth` with `aurago_sway` bend with height, scaled by the `golfMotion` switch;
  `emissive` and `glow` multiply their emission by `COLOR_0` and take
  `aurago_flicker`; `glass` loses its transmission (transparent at 0.55 opacity).
  `aurago_strength` is not read at runtime; the exported emission is used.
- Effects: `burst(type, position, speed, info)` types are `strike`, `impact`, `water`,
  `cup`, `win` and `spark` (18 glow particles in the look's palette, the P3 mechanic flash); `info.surface` (`sand`) selects the sand cloud on a strike. The
  rolling sand plume follows `update(dt, snapshot)` and its optional
  `snapshot.surface`. The pools are fixed buffers capped by `TIERS.particles`
  (confetti: half); nothing allocates per frame. The ball trail is 28 points, on
  medium and high only, and shows once the ball moves faster than 0.6 m/s.
- Intro flight: `createIntro()` follows the path at 2.2 m behind and 3.2 m above,
  looking 2.5 m ahead, for 0.35 s per metre clamped to 2.5–4.5 s, and then settles
  on the overview pose. `scene.skipIntro()` ends it (returns true when one was
  running): pointer down on the canvas, wheel, orbit start, a strike burst and
  `setReducedMotion(true)` call it. `focusBall()` ends a running flight as well, and
  `minigolf-game.js` calls `skipIntro` on any keydown without consuming the key. A
  `frameTurn` queued during the flight runs after it.
- Reduced motion (OS/Desktop preference, `data-animations="false"` or the checkbox)
  skips the flight, stops particles, fireworks, sway, flicker, spin, flags and
  water and sky animation, and keeps the gold ring static. The ball
  trail stays. Low tier also drops scatter props, sway, flicker and spin, the
  composer (bloom) and the trail, and freezes the water.
- `stats()` also reports `look`, `props` (placed instances), `env` (environment
  set), `intro` (flight running) and `trail` (visible trail points). `triangles` and
  `calls` count the whole frame: `renderer.info.autoReset` is false and the counters
  reset once per frame. Before P2 they covered only the last composer pass, so only
  frame rates compare with older runs.
- Saves contain settled shots only. Pause, minimization and hidden tabs stop
  simulation/audio; closing releases GPU, WASM, audio, observers and listeners.
  Respect reduced motion while retaining gameplay obstacle movement.
- Storage: `round.v2` keeps the saved round with each player's colour; `best.v2`
  holds `set`, `rounds` (full and worlds only) and `holes` (per course, any mode);
  `settings.v1` holds `tutorial` (0–2, where 2 hides the introduction), `showMap`
  and the other fields. `normalizeSettings()` is the only place that defines
  settings fields, and `saveSettings()` keeps unknown stored fields. The v1 round
  and best keys are removed without migration, with a one-time notice when no v2
  progress exists.
- World ids are `pirate` (holes 1-6), `temple` (7-12) and `moon` (13-18); `WORLD_ICONS`, the accent
  tables in `css/desktop-app-minigolf.css` and the locale keys `minigolf.<worldId>` cover exactly these three
  (the P1/P2 keys `minigolf.garden|coast|night` are removed from all 16 locales). Optional card previews come from manifest
  `worlds: [{ id, preview }]`; the game accepts only relative image paths (webp,
  png, jpg, avif) without `..`.
- Styling uses `--mg-*` tokens aliased to Desktop `--vd-*` theme tokens, with no
  theme-name selectors. UI timers advance with the `frame()` loop, never
  `setTimeout`. `data-reduced` on the app root disables transitions and animations.
- While the panel is `menu` the footer `.mg-controls` is not rendered (CSS on
  `data-panel`; the stage resizes through its observers). `showMenu()` calls
  `hud.clearToasts()` first, so later toasts such as `progress_reset` still show.
  Taken colour swatches carry a strike line besides the dimming. With two players
  the podium renders two steps centred as a pair (`data-count="2"`).
- `inspect()` is additive: `hud` (`meter`, `bannerCount`, `banner`, `toasts`,
  `map`), `tutorial` (current step) and `menuStep` (menu panel only).
- Scene helpers `frameTurn`, `skipIntro()`, `setFollow`, `setAimCamera`, `setRestMark`,
  `panAt`, `screenPosition()`, `cancelDrag()`, the optional `info` argument of
  `burst()` and `stats().tier` are optional-callable (the game calls them with `?.`).
  `cancelDrag()` restores a changed aim from before the press and reports `onAim` with
  `phase: 'cancel'`.
- Audio is the `minigolf-audio.js` facade over `minigolf-audio-engine.js` (context,
  buses, limiter, voices, loops, reverb), `minigolf-audio-music.js` (crossfades,
  ducking) and `minigolf-audio-synth.js` (UI blips, ambience, impulse responses).
  AuraGolf must not load Synth Studio. Create no `AudioContext` and make no `fetch`
  before a user gesture (`unlock()`). Persisted zero volumes stay zero.
- Scene files are `ui/3d/minigolf/course-NN.glb`, `props-<look>.glb`,
  `previews/<look>.webp` and `manifest.json` (contracts in `assets/minigolf/README.md`
  and `kit/AGENTS.md`). A missing preview falls back to the drawn placeholder.
- Audio comes only from `ui/3d/minigolf/audio/`, generated from
  `assets/minigolf/audio/sources.json` by `scripts/build-minigolf-audio.mjs`.
  `--check` verifies SHA-256, CC0, the budgets (SFX ≤ 2 MiB, music ≤ 6 MiB,
  manifest ≤ 128 KB) and stray files. Never hand-edit `manifest.json`; change the
  recipe and rebuild. `REQUIRED_GROUPS` must equal the manifest SFX groups.
- `pause()` suspends the context and freezes `position`. `setMusic('menu'|world)`
  and `setAmbient(world)` take the world id as is (unknown ids select the menu track or no ambience). `cup`,
  `cannon` and `win` duck the music. `effect('cannon'|'boost'|'teleport')` exists
  and is played by the game from the matching physics events.
- The browser test's zero-tolerance underrun assertion can flake under heavy
  machine load; rerun once before suspecting a regression.
- Validate `npm run test:minigolf` (model, `scripts/test-minigolf-look.mjs`, UI,
  physics, asset, course and audio checks), `npm run test:synth-studio`,
  `node scripts/build-minigolf-audio.mjs --check`, `node scripts/build-minigolf.mjs --check`
  (6 MiB cap, pinned Three.js 0.186.1 and Rapier 0.21.0), `TestDesktopMinigolf*`,
  `TestFrontend_StaticI18nKeysExistInEnglishBundle` and the desktop bundle checks.
- `TestDesktopMinigolfBrowser` needs `AURAGO_RUN_BROWSER_SMOKE=1`. This machine's
  headless Chrome reports `prefers-reduced-motion: reduce`, so the smoke emulates
  `no-preference` only for its first intro check, resets it, and then requires that
  later loads under reduced motion start no flight. It loads the finale of each world of manifest v2
  (`pirate-06`, `temple-12`, `moon-18`; `AURAGO_MINIGOLF_ALL_COURSES=1` loads all 18) through the menus
  (`graphics.look` equals the world id, props and environment present) and checks that the ball rests on its tee
  floor, which proves that the collision came from the GLB; the world cards are `full,pirate,temple,moon,practice`. `AURAGO_MINIGOLF_BENCHMARK=1`
  logs 240-frame 1920×1080 measurements per tier (run with `go test -v`).
  Browser/GPU measurements and human listening remain separate acceptance steps.
  Keep `minigolf.*` in all sixteen Desktop locales, including the
  `follow_ball`/`aim_camera`/`magnet`/`hole_map` and score-grade keys; settings
  persist `follow`/`aimCamera`/`magnet`/`tutorial`/`showMap` additively.
  See `documentation/minigolf.md`.

## Newspaper

- `newspaper.js` owns one window reader for today's immutable edition, articles, archive and preferences. When only an earlier issue exists, keep it readable and offer creation of today's issue; do not offer today's correction flow on older issues. Closing the window cancels only UI work; the server owns ongoing research and delivery.
- Preferences include section-bound RSS feeds. A new revision can carry an optional correction note; the reader displays it on the published issue without changing older revisions.
- Show configured research capabilities and actionable setup reasons separately from last-attempt errors. Preserve optional run counters (candidates/read pages/accepted stories) and missing-topic labels after completion or failure. Escape topic labels like all other external text; older run JSON without counters must remain readable. Config exposes `max_searches` with default 32 and range 1–64. Verify `TestNewspaperResearchTranslations` and `TestNewspaperConfigResearchBrowser` alongside the reader matrix.
- Escape all source text and allow only safe HTTP(S) source links. Show actual publication and source times, partial coverage, single-source disclosure, run progress and delivery uncertainty. Keep the same issue when moving between views and preserve reading position.
- Show localized email-confirmation bounce suppression, rejection, configuration failure, uncertainty and cooldown messages from structured API codes. Send code first saves the selected sender account and recipient address, without saving unrelated draft preferences; never send through a stale stored account. Do not render raw AgentMail response text or repeat a blocked send automatically.
- Load `desktop-app-newspaper.css` lazily, use both Desktop themes, all 16 `newspaper.*` locale entries and the Papirus/WhiteSur Newspaper icons. Verify with `TestDesktopNewspaperBrowser` and `TestNewspaperTranslations`.

## Personal Radio

- `personal-radio-player.js`, `personal-radio-runtime.js`,
  `personal-radio-settings.js` and `personal-radio.js` load in that order.
  Register `personalRadio` in the module loader's `APP_I18N_SECTIONS`; the
  desktop shell does not initially embed this app's translation prefix.
  Window disposal cancels only window work. The singleton desktop runtime owns
  playback, listener heartbeat and the mini control until explicit stop or
  pagehide; reopening must not create a second player.
- Show startup immediately in the central player with an explicit current task,
  actual ready tracks/minutes against both requirements, and the automatic-start
  explanation. Keep this visible throughout preparation, including after the
  opening. Do not use the small reserve/job label as the only startup feedback.
  Expose registry search, preparation and failure states. A zero minute reserve
  means automatic start with the required prepared track count; show available
  minutes without an invented 30-minute target or division by zero. Explain that
  matching existing music plays while new fitting music is produced in background.
- Use one Web Audio clock, bounded PCM windows and two prepared segments for
  initial music playback. A typed opening may play alone before `music_ready`,
  without priming/bypassing music preparation. A window that starts late plays
  from the elapsed offset; a fully elapsed window is dropped. A failed fetch
  still fails that segment. Fence asynchronous work by generation and server epoch.
  Never claim airtime from a server timer. Expired speech must not resume.
  A heartbeat rejected as `radio_device_busy` stops the local player, releases
  the desktop media session and dispatches `personal-radio-stopped`.
- Blend music into and out of speech on that clock. Keep speech clear above the
  ducked music; every third eligible moderation may retain a quiet outgoing music
  bed for up to 4.5 seconds. During overlaps, the newer segment owns the title
  and playback position.
- Keep Standard/Fruity tokens, reduced motion and all 16 locale dictionaries.
  The original Radio app, Noisemaker and Webamp retain their own lifecycles.
- Verify `TestPersonalRadioBrowser`, `TestPersonalRadioAudioContinuityBrowser`,
  `TestPersonalRadioVoiceMixBrowser`
  and `TestPersonalRadioTranslations`. Backend: `internal/personalradio/AGENTS.md`.

## RTL-SDR

- `rtl-sdr-runtime.js`, `rtl-sdr-panel.js`, `rtl-sdr-scope.js` and `rtl-sdr.js`
  load in that order. `rtl-sdr.js` resolves local `t('key')` calls under
  `rtlSdr.`; the built-in app name uses `desktop.app_rtl_sdr`. Keep both in all
  16 desktop locales. The panel and scope modules receive translated text and
  never call `t()` themselves; the static i18n checker resolves the app-local
  prefix for `rtl-sdr.js` only.
- The receive view is a graphite receiver front panel: station memory, display
  (status indicators, seven-segment frequency, station text, signal meter, band
  scope with waterfall), tuning column and key strip. It fits the default
  1000x760 window without scrolling; narrower windows stack the sections and
  scroll. Recordings, schedules and receiver setup are views of the same chassis.
- `s.tuning` is the single source of the pending tuning. Every control commits
  into it and `controls()` repaints all widgets. Mode keys and AGC/stereo are
  native radio/checkbox inputs, the four small knobs wrap native range inputs,
  and the gain knob steps through the tuner's reported gain list by index.
  A manual frequency or mode change drops the station label.
- Frequency digits are created once and repainted, so focus survives tuning.
  They accept wheel, arrow keys and typed digits. While the panel itself
  scrolls, the wheel only turns the control that has focus. The waterfall keeps
  its history across view changes and shifts old rows with the tuned frequency.
- The singleton runtime owns the desktop mini control. Windows report their
  visibility through `RTLSDRRuntime.present(id, visible)`; the mini control
  shows only while listening without a visible receiver window.
- Verify with `TestRTLSDRTranslations`,
  `TestFrontend_StaticI18nKeysExistInEnglishBundle` and the opt-in
  `TestRTLSDRDesktopBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`). Its
  `AURAGO_RTLSDR_SCREENSHOT` path also receives `-fruity` and `-compact` views.

## Bluetooth

- `bluetooth-views.js` precedes `bluetooth.js`; strings live under `bluetooth.*`
  plus `desktop.app_bluetooth` in all 16 desktop locales and load through
  `APP_I18N_SECTIONS`. Views use literal translation keys so the static i18n
  checker sees every call.
- Hardware-bound built-in apps declare `Requires` in their manifest; the
  server's `CapabilityProvider` decides visibility at read time and a
  capability change broadcasts `desktop_changed` with
  `operation: "app_availability"`. An open window is never closed by a
  capability change; it shows its unavailable state and resumes.
- The app never trusts event payloads: every `aurago:bluetooth-change` with a
  different revision re-fetches `GET /api/bluetooth/status`. Fetches never
  overlap. Server-owned state (scan, visibility, running operations) survives
  closing the window.
- Device actions post `wait: false`; progress and errors come back as
  `operation`/`error` codes on the device and are shown from
  `bluetooth.error_<CODE>` keys, never from English server text.
- Pairing questions render in an in-window dialog with a 20 s countdown;
  passkey/PIN fields validate locally. With the app closed, the shell shows a
  "Pairing request" notification.
- Verify `TestBluetoothTranslations`,
  `TestFrontend_StaticI18nKeysExistInEnglishBundle` and the opt-in
  `TestDesktopBluetoothBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`, screenshots via
  `AURAGO_BLUETOOTH_SCREENSHOT`). Backend: `internal/bluetooth/AGENTS.md`.

## Detective
- `detective-views.js` precedes `detective.js`; both use the native Desktop theme
  tokens and `desktop-app-detective.css`. Register the built-in `detective` app
  with the existing search icon. All sixteen locales use `desktop.detective_*`.
- `DetectiveApp` owns a per-window instance with cancellable polling. Disposal
  aborts that window's UI controller only; it does not stop the run. Research
  remains owned by the server. While a case is open, one refresh chain reads
  `GET …/state?after=` (live view plus new events in one round trip) and the case
  list, every 3 seconds while the case is queued, running or waiting, every 10
  seconds otherwise (`v.pollDelay`). A hidden document does no work at all; the
  `visibilitychange` listener polls once as soon as the window is visible again.
  With the research form open only the first list load happens, never a poll.
  Source text and the report render again only when that data changes. A missing
  report and `latest_revision` 0 are the same revision, so an empty report stays in
  place.
  An open source disclosure, a report text selection, scroll position, and a
  focused answer keep their text through a status-only poll. The reason
  paragraph tracks the live reason. `waiting_for_user` shows `.dt-answer` on
  the next poll and removes it when the run leaves that status; a focused
  answer keeps its text and focus while that status remains. Render evidence
  and structured report blocks as escaped text. Only HTTP(S) source links are
  clickable; private integration receipts never masquerade as website links.
  Source disclosures expose `data-id`. Activity items expose `data-event`, and a
  poll does not append an event id that is already in the list. A full load
  stores `latestSourceID` only after the loaded sources match that id, and an
  absent source list becomes an empty list so the next poll does not refetch it.
- Stop stays enabled for an active case when Detective or Virtual Desktop is
  read-only, and when the integration is enabled without a ready runner
  (`v.canStop(caps)` needs only `enabled`; Stop is a server state write). Finish,
  Start, Continue, Deepen, Delete and New stay disabled. A draft offers Start and
  the effort control, not Deepen. Continue does not post `effort`. Start and
  Deepen do. Disabled actions name their reason from `caps.block_reason`
  (`disabled`, `read_only`, `initializing`) through `desktop.detective_block_*`.
- The revision selection is view state: `knownReports`/`chosenReport` in
  `detective-views.js` merge current reports, history and the pinned report, and a
  revision that is not in that set falls back to the newest one. A provisional revision (the
  partial a failed continue, deepen or update keeps) never becomes the default:
  `chosenReport` prefers the newest non-provisional revision, the selector marks it,
  and the report tab shows `desktop.detective_provisional_notice` with a button to the
  last full revision. `resetArchive()`
  clears `state.revision` (case switch, "New", and after creating a case, which also
  opens the report tab), so a stale selection can never leave the export bar without
  a report. Test it without a DOM in `scripts/test-ui-regressions.mjs`.
- Idempotency keys use `crypto.randomUUID` in a secure context and fall back to
  `crypto.getRandomValues` on a plain-HTTP LAN install (`Math.random` only without
  any crypto object), so Start/Continue never throw; the key stays under the
  server's 128-character limit.
- The case payload carries source metadata only. A source disclosure starts with a
  `<blockquote aria-busy="true">…</blockquote>` and the `toggle` listener fetches
  `GET …/source?id=` once, caches the text per case (`excerpts`, cleared by
  `resetArchive`), and fills that blockquote in place — no redraw, so the open
  disclosure and the scroll position survive. Archive pages already carry their
  excerpts and render inline.
- Export links name the revision on screen. A live poll rebuilds the export bar
  when the visible revision changes or when the revision set grows or shrinks,
  even if the chosen number stays the same. A poll that leaves both unchanged
  does not rebuild the bar. Autor
  opens a new, create-only DOCX copy. No private model continuation is returned
  to the UI.
- API refusals carry `code` in `err.body.code`; `v.errorText(tr, err)` shows
  `desktop.detective_error_<code>` and keeps the server text for an unknown code.
  `v.formats(caps)` orders the export links: with `caps.pdf_scripts === false`
  (the built-in PDF writer's truth) Word comes before PDF and the PDF link gets
  `aria-describedby` pointing at the `pdf_hint` note, so the limit is visible
  before the download fails.
- Answers and token counts are visible: `startLocked` writes an `answer` event, so the
  activity tab shows the question and the reply after the window was closed, and
  `v.tokenText(usage, tr)` renders `desktop.detective_tokens{,_in,_out,_cached}` in the
  status line, the case list total (`caseTokens`) and the automatic panel (usage of the
  last run). No pricing logic lives here; the budget tracker owns costs. Zero counts
  render nothing.
- Verify `TestDetectiveTranslations`, `TestDesktopDetectiveBrowser` and the
  Detective config section. Backend contracts: `internal/detective/AGENTS.md`.
- Automatic Update settings belong to each case (also available on creation):
  integer hours/days, append/rewrite and a separate update effort. Preserve unsaved
  settings, focus, selected revision, source disclosure and scroll across polls.
  Pause remains available in readonly mode; enabling/changing settings does not.
  Show due/checked/changed times and question/error/permission status. History and
  source pages load on demand; append ancestry loads incrementally. Each export
  stays bound to the selected revision, including all its ancestor additions.

## Local Wikipedia
- `local-wikipedia-views.js` precedes `local-wikipedia.js`; both use
  `desktop-app-local-wikipedia.css` and `--vd-theme-*` tokens (no per-theme
  overrides). The builtin `local-wikipedia` app ("Wikipedia", icon `book`,
  category `office`) requires the `local_wikipedia` capability. All sixteen
  locales own `desktop.app_local_wikipedia` and `desktop.local_wikipedia_*`;
  views use literal keys.
- `LocalWikipediaApp` keeps one instance per window with an `AbortController`;
  disposal aborts requests, clears the 200 ms suggestion debounce and the status
  poll (1 s while the edition loads, 3 s while downloading/verifying, 15 s while
  unavailable, 5 s after a failed poll while reading, 5 min while readable) and
  blanks the frame. Only `status.readable` decides whether content is shown.
- The article iframe loads `/api/desktop/local-wikipedia/content/<path>` with
  `sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"`,
  matching the server CSP (`script-src 'none'`, `object-src 'none'`, `base-uri 'none'`,
  `connect-src 'none'`): no ZIM script runs. Same-origin access only reads the
  title and the `aurago-local-wikipedia-error` marker and adopts links: `ping`
  and `attributionsrc` go, external `http(s)` links get `target="_blank"
  rel="noopener noreferrer"`, `mailto:` stays, other schemes and same-origin
  links outside the content route lose their `href`. The app owns back/forward
  history (200 entries; app navigations use `location.replace`, entries are
  recorded on `load`, a redirect leaves one entry); paths with `.`/`..` segments
  are never addressed.
- Results and suggestions render escaped text only; Enter without an active
  suggestion searches; the toolbar is one tab stop (arrows, Home, End). Admin
  actions open `/config#local_wikipedia` in a new tab only when the status
  reports `can_manage`. Result titles and error text keep text contrast: no
  plain accent or coral text (too light on Fruity light).
- Verify `TestLocalWikipediaTranslations`, `TestLocalWikipediaViewHelpers`,
  `TestDesktopLocalWikipediaModuleContract`, `TestDesktopLocalWikipediaRegistration`
  and, with `AURAGO_RUN_BROWSER_SMOKE=1`, `TestDesktopLocalWikipediaBrowser` (fake
  API) and `TestDesktopLocalWikipediaShellBrowser` (real shell through
  `desktopAuditBrowserWithAPI`, 4.5:1 text contrast; screenshots in
  `reports/local-wikipedia*/`). Server contract: `internal/localwiki/AGENTS.md`.

## Retro Games
- `retro-games-views.js`, `retro-games-player.js` and `retro-games.js` load in this
  order with `desktop-app-retro-games.css`. The builtin `retro-games` app ("Retro
  Games", icon `arcade` → `run`, category `games`) requires the `retro_games`
  capability. Strings are `desktop.app_retro_games` and 81 `desktop.retro_games_*`
  keys in all sixteen locales.
- The app owns a scoped dark skin (`.rg-*`, neon browser + cabinet player) that is
  identical under Standard and Fruity; fonts are the bundled Press Start 2P and
  Px437 IBM VGA ("Retro Games VGA").
- No `innerHTML` in the three files: archive.org titles and makers reach the DOM
  only via `textContent`/attributes. Thumbnails load from
  `https://archive.org/services/img/<id>` with `referrerpolicy="no-referrer"`.
- The emulator frame is AuraGo's own player page, never archive.org's `/embed/` page (that one starts
  every emulator muted, is cut off in a small frame and fires `load` long before the emulator runs). Its
  URL is `PLAYER_BASE + encodeURIComponent(id)` plus `&muted=0|1&sound=<label>` (`playerURL(id, muted,
  soundLabel)`; the label is `desktop.retro_games_click_for_sound`, rendered by the page as text), with
  exactly `sandbox="allow-scripts allow-pointer-lock"` and `allow="fullscreen; gamepad; autoplay"` (no
  clipboard, popups, forms or top navigation). The page (`internal/server/retro_games_player.go`) runs
  archive.org's Emularity loader. Without `allow-same-origin` (the page's CSP carries the same sandbox) the
  frame has an opaque origin: archive.org's code cannot read AuraGo cookies, storage, DOM or APIs and can
  only talk to archive.org hosts. Never add `allow-same-origin`, `allow-top-navigation` or `allow-popups`,
  and never point the frame at anything but the player route or `about:blank`. Back and window close
  (`dispose`) point the frame at `about:blank` and remove it. The player mount is the only place that
  frames it; the "open on archive.org" button still opens `https://archive.org/details/<id>` in a new tab.
- Message protocol (`{ rg: 1, ... }` both ways, `postMessage` with target `'*'` because the origin is
  opaque). Page -> app: `{ type: 'state', state: 'loading' | 'running' | 'failed' }` and
  `{ type: 'audio', blocked: boolean }` (the page shows its own click-for-sound pill; the app ignores it).
  App -> page: `{ type: 'mute', muted: boolean }` (the page also accepts `{ type: 'focus' }`; the app does
  not send it). `mount` registers exactly one `message` listener and `destroy` removes it. It accepts
  only `event.source === frame.contentWindow` with `data.rg === 1`, and then only `PLAYER_STATES`
  (`loading`/`running`/`failed`) and a boolean `blocked`: archive.org's code runs in the same realm as the
  page and can post any `{ rg: 1 }` message, so the content is untrusted, nothing from it is rendered or
  logged, and a flood or out-of-order states must change nothing. `phase` enforces that: `idle` -> `loading`
  (counted once) -> `running`; `failed` ends it from `idle` or `loading` (a script that cannot load fails
  the page before it reports `loading`) and is final until the next start; after `running` it is ignored,
  because a game that ran never shows "could not be started".
- Every start (mount and each restart) builds a FRESH iframe (`makeFrame()`, `start()`) and replaces the
  old element. Re-navigating the same iframe (`about:blank`, then the player URL) does not stop the old
  document: it stays alive and keeps posting until the new response commits, and its messages pass
  `event.source === frame.contentWindow` because the window object is the same (measured: dozens of stale
  messages up to ~300 ms after the restart, a stale `failed` then stuck over the running game). A removed
  iframe's window no longer matches `frame.contentWindow`, so what it still had queued is dropped. Never
  restart by changing the `src` of the existing frame; `destroy()` still unloads (`about:blank`, remove).
- States drive one bottom strip (`.rg-overlay`, `pointer-events: none`, only the hint buttons opt back
  in, so the game stays clickable): the loading bar (`desktop.retro_games_player_loading`) from mount
  until `running`; `failed` or no `running` within `STALL_MS` = 60 s show the hint with restart, open on
  archive.org and mark as not starting plus `desktop.retro_games_player_failed` or
  `desktop.retro_games_player_stalled` (a late `running` retires the stall hint). The frame's `load` event
  and window focus/blur say nothing about the emulator and are not used. `mount(..., { stallMs })` shortens
  the delay for tests only; they wrap `RetroGamesPlayer.mount`.
- Sound: the mute button (🔊/🔇) sits in the marquee actions. Its label (`aria-label` and `title`) names
  what a click does: `desktop.retro_games_sound_off` while sound plays, `desktop.retro_games_sound_on`
  while muted. It has NO `aria-pressed` (a pressed button labelled "Sound on" reads as the opposite); the
  icon and the `.is-muted` class show the state. The choice is kept in
  `localStorage` key `aurago.retroGames.muted` (read and write in try/catch; blocked storage means sound
  on), goes into the player URL (`muted=1`, also after a restart) and is posted to the running page on
  click. A click before the page listens is repeated once the page reports `loading` (`urlMuted`).
- The keyboard belongs to the game: `mount` focuses the frame (never Back), and restart, fullscreen,
  favorite, broken, mute and the hint's restart/mark-broken buttons return the focus to it through
  `focusFrame()` (the external link opens a tab and leaves it); the page moves it on to its canvas. Our
  buttons stay in the tab order.
- Launch argument `game` (validated `^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`) opens the
  player directly: `openApp('retro-games', { game })`, the existing-window hook
  `RetroGamesApp.open`, and the `open_app` desktop event (agent tool `retro_games`).
- Refused library changes get their own notice through `ERROR_NOTICES` in `handleError`:
  409 `library_full` -> `desktop.retro_games_library_full`, 403 `desktop_readonly` ->
  `desktop.retro_games_readonly` (favorites, broken markers and the sync button). A failed
  toggle reverts, shows the notice and reloads the library and the current list. Recording a
  play never shows `desktop_readonly` (the game still runs).
- `showDisabled` keeps one slow passive poll (`status?passive=1`, every 30 s, on the shared
  `st.pollTimer`, so there is only ever one timer and `dispose` clears it). When the status
  answers enabled again, `reenable` clears the disabled state and runs the first-open path
  (library, status, current view). Never start a sync from that poll, and never touch
  `st.pollTimer` from `refreshStatus` while `st.disabled`.
- The config page's "Check index" uses `status?passive=1` too: a plain status request starts
  the first index download.
- Verify `TestDesktopRetroGamesTranslations`, `TestDesktopRetroGamesRegistration`,
  `TestDesktopRetroGamesViewsNeverRenderHTML`, `TestDesktopRetroGamesPlayerFrameIsLockedDown`,
  `TestDesktopRetroGamesPlayerTrustsOnlyItsOwnFrame`, `TestDesktopRetroGamesPlayerGivesEveryStartAFreshFrame`,
  `TestDesktopRetroGamesPlayerHasAMuteToggle`,
  `TestDesktopRetroGamesAppLifecycle`, `TestDesktopRetroGamesPlayerGivesTheGameTheKeyboard`,
  `TestDesktopRetroGamesStalledHintNeverCoversTheFrame` and, with `AURAGO_RUN_BROWSER_SMOKE=1`,
  `TestDesktopRetroGamesBrowser`, `TestDesktopRetroGamesPlayerBrowser`,
  `TestDesktopRetroGamesPlayerBrowserFailure`, `TestDesktopRetroGamesPlayerBrowserHostileMessages`,
  `TestDesktopRetroGamesPlayerBrowserRestartIgnoresTheOldPage` (holds the player route and makes zero-delay
  timers microtasks, so the old-document race is certain instead of lucky) and
  `TestDesktopRetroGamesBrowserStalledHint`. They drive the real shell (`desktopAuditBrowserWith`) with the
  REAL player page (`server.ServeRetroGamesPlayerUngated`, headers included); only archive.org's
  `browserfs.min.js` and `loader.js` are hijacked (`page.HijackRequests()`: an empty script and a fake
  `IALoader` that draws a test pattern, can be held back, fail or post junk). A sandboxed frame is
  out-of-process by default and its requests never reach the page-level router, so these tests launch
  Chrome with site isolation off (`disable-site-isolation-trials`, `disable-features=...IsolateOrigins,
  IsolateSandboxedIframes`); reach into the frame with `el.MustFrame()`. Screenshots land in
  `reports/retro-games/`. Server contract: `internal/retrogames/AGENTS.md`.

## Network Cameras person detection
- `network-cameras-detection-views.js` (constants, pure helpers and string-only
  markup builders, no DOM; `window.NetworkCamerasDetectionViews`),
  `network-cameras-detection-zones.js` (camera tab controller and zone editor;
  `window.NetworkCamerasDetectionZones`) and `network-cameras-detection.js`
  (detection mode runtime and settings dialog) load in this order before
  `network-cameras.js`, with `desktop-app-network-cameras-detection.css`. Public
  API: `window.NetworkCamerasDetection = { mount, unmount, cameraSettingsMarkup,
  bindCameraSettings, validateCameraSettings }`; each file stays below the
  1100-line budget.
- `network-cameras.js` stores the mode `detections` (`coerceMode`), mounts into
  `[data-role="detection"]` from `draw` (`syncDetection`), unmounts with
  `{ keepState: true }` when the window is hidden or another mode is shown, and
  forgets the state in `dispose`. The manage modal shows General | Person
  detection tabs to admins (both panels rendered once, toggled with `hidden`);
  `PATCH streams/<id>` carries `detection` only when the tab changed it, after
  `commitDrawing()` (closes a zone being drawn; under 3 points it blocks the
  save with `zone_unfinished`) and `validateCameraSettings`. The server MERGES
  that object per key and rejects `null` and unknown keys, so `cameraPayload`
  sends exactly the C5 keys with the full `zones` list. `detection_enabled` is
  top-level in app/state and copied onto the stream for `cam_global_off`.
- Launch contexts `{event_id, camera_id}` (person detection toasts, kept by the
  notification center) open mode `detections` on that event: `render` reads
  them from its context, `open(windowId, context)` handles an open window and
  is called from `openApp` in `core/window-shell-runtime.js` (main bundle:
  `npm run build:ui`). `mount(…, { launch })` sets the camera filter and loads
  `events/<id>` even when the event is not on the loaded page. A 404 /
  `event_not_found` on `events/<id>` (the change feed has no tombstones)
  drops the event, closes the detail and shows `detection.event_gone`.
- The camera tab shows `detection.cam_restart_hint` under the detection switch
  (`aria-describedby`): while go2rtc runs, toggling recreates its container (a
  stopped go2rtc stays stopped).
- API base `/api/desktop/integrations/go2rtc/detection/`. The first mount of a
  window runs `state?since=0` (a snapshot: status + arm log; its events are
  ignored, they come paged from `events?…` with `before`), then
  `state?since=<cursor>` every 5 s only while mounted; a remount with kept state
  resumes the cycle. A settings save or "Activate" clears `lastPollAt`, so the
  remount from `reloadAppState` polls at once. The settings dialog and
  "Activate" need `detection_settings` from `GET app/state` and send the FULL
  object (`PUT settings` replaces); they never save guessed values.
- Ids are int64 around 1.8e12: read them only through `idOf()` (safe-integer
  check, no float parsing). Times are ms UTC; `next_change` shows in the browser
  zone, while the strip (when the zones differ) and the schedule editor show the
  server zone from `ArmState.time_zone`/`utc_offset` (`serverZone()`; a UTC
  server reads "UTC").
- Zone editor: points snap within 0.015 of a frame edge (the server maps with
  0.01); polygons keep the default nonzero fill rule (`evenodd` is forbidden);
  a zone under 0.01 % of the frame or with collinear points warns
  `zone_flat`; names are trimmed only when edited and carry `aria-invalid`
  over 48 runes; test results show localized texts, never server messages.
- Escaping: server strings only via `ctx.esc` or `textContent`; `innerHTML` only
  for region/skeleton strings, the zone SVG and the zone list (regression-
  tested); SVG from numbers; the test image only as `data:image/jpeg;base64,…`.
  No `alert/confirm/prompt`; deletion uses `ctx.confirmDialog`.
- Accessibility: `aria-pressed` arm and day buttons, ↑/↓/Home/End between
  timeline rows, ←/→ on the focused detail, Enter/Esc/Backspace in the zone
  stage, list-based zone rename/select/delete, one `aria-live` region, focus
  kept across re-renders via `data-focus-key`.
- Strings: `desktop.network_cameras.tab_general` and
  `desktop.network_cameras.detection.*` (including the server's alert
  keys, whose single-brace Go placeholders such as `{camera}` never go through
  the JS `{{…}}` interpolation) in all sixteen locales.
- Verify: `node scripts/test-ui-regressions.mjs` (four "Network Cameras person
  detection" contracts), `go test ./ui/ -run TestDesktopNetworkCamerasDetection`
  and, with `AURAGO_RUN_BROWSER_SMOKE=1`,
  `TestDesktopNetworkCamerasDetectionBrowser`,
  `TestDesktopNetworkCamerasZoneEditorBrowser`,
  `TestDesktopNetworkCamerasDetectionActivateBrowser`,
  `TestDesktopNetworkCamerasDetectionLaunchBrowser` (fake API with the real C4/C6
  shapes, browser in Europe/Berlin against a UTC server; screenshots in
  `reports/network-cameras-detection/`).

## Looper
- `looper-monitor.js` precedes `looper.js`. The monitor owns the run view
  (`createRunView`), the log-delta merge (`mergeStatus`) and the history views;
  `looper.js` owns the form, presets, drafts and the action bar.
- `/api/desktop/looper/status` sends the whole state first and afterwards only
  log entries appended since (`logs_from` = absolute index of the first sent
  entry, `log_total`, `run_id`). A message without `logs_from` is a full
  replacement. The client merges by absolute index and never rebuilds the
  timeline: new entries are appended in place, finished rounds fold away, and
  following the newest entry stops as soon as the user scrolls up.
- While a run is active the editor is replaced by the summary card
  (`is-focus`); `Edit loop` brings the fields back and is disabled while a run
  is executing. Preset switches are blocked during a run. Unsaved edits show the
  `vd-looper-dirty` dot, are confirmed before being discarded and are kept as a
  draft in `localStorage` (`aurago.looper.draft.v1`, always wrapped in try/catch).
- A pending pause (`pause_requested`) disables the pause button and says so;
  Stop on a paused run discards it (the server files it as `stopped`). A review
  without a usable score is a `failed` evaluate entry, never score 0.
- Chart, gauge and hero use theme tokens; keep the grid rows explicit so the
  action bar cannot absorb the free row. Every new string needs all 16 locales.
- The unused run view shows the decorative transparent `img/looper-empty.png`
  illustration with the existing empty-state copy. It shares the monitor's idle
  predicate and disappears for running, paused and finished runs; keep it responsive.
- Costs come from the budget tracker's model rates (`EstimateCost`); `cost_approximate`
  marks a fallback price. Starting or resuming under an exhausted budget answers
  402 with `code: "budget_exceeded"`, and a running loop pauses itself with
  `pause_reason: "budget"`. API errors carry `code` in `err.body.code` (the shared
  `api()` helper sets no `status`), never match on the English message.
- The active run is checkpointed to `desktop_looper_active` after every round and
  restored paused (`pause_reason: "interrupted"`) on the next start; a graceful
  shutdown keeps the checkpoint instead of filing the run as stopped. A window
  that finds an active run with an empty editor adopts `GET /api/desktop/looper/active`;
  Resume with an empty editor sends `{}` and the server uses the stored settings.
  `LooperRunner.checkpoint` only writes while the run is running or paused and shares
  `checkpointMu` with the removal in `persistFinishedRun`, so a discarded run cannot be
  brought back by a late write. `init()` waits for the first status (max 1.5 s) before
  `restoreDraft()`, otherwise a stale draft would shadow the settings of a restored run.
- History records keep their run settings (`config` on `GET .../runs/{id}`), which
  drive "Load into editor" and "Run again"; older records have none.
- The desktop opens a window only when a `virtual_desktop_event` arrives over SSE.
  Looper tool calls run with `looperDesktopBroker`, which forwards just those events
  and drops all other tool feedback; `open_app`/`open_widget` pass only in the finish
  step, because an early Writer window would merely be refocused there, still showing
  an old draft (`TestLooperFinishOpensTheResultInTheDesktop`).
- Verify `TestDesktopLooperUIContract`, `TestDesktopLooperCostI18n`,
  `TestDesktopLooperDurationI18n`, `TestDesktopLooperActionButtonsShareConsistentStyle`
  and the opt-in `TestDesktopLooperBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`).
  Backend: `internal/desktop/looper.go`, `internal/server/looper_service.go`.

## EasyDrag
- Twenty scripts (`easydrag-*.js` and `easydrag.js`) share `window.EasyDrag` and load in the
  order of `DESKTOP_APP_ASSETS.easydrag` (`core/module-loader.js`), `easydrag.js` last.
  `window.EasyDragApp = { render, open, dispose }`; `open` re-routes an existing window by
  `flowId`/`flow_id` and `runId`/`run_id`, and `{section: 'home'}` (Mission Control's New flow)
  shows the start page once the editor's `leave()` allowed it.
- Pure modules (template, model, geometry, start-page preview, shortcut table) run in Node; the
  template filters mirror `internal/flows`. Template errors keep Go's English message (the parity
  checks compare it) and carry `err.code` (`TPL_*`, `tplError`) with `err.params` ({name},
  {text}); the field preview shows `easydrag.ui.tpl_error_*` (`TPL_ERRORS` in
  `easydrag-forms.js`), an error without a known code the generic error, never the message. Every model command is one undo step. Pans and zooms
  change only `ed.view`, stored per flow on the device (`aurago.easydrag.view.<id>`): the model
  has no viewport command, so they never bump `model.version`, save, or count as unpublished
  changes; a document's own `viewport` (old drafts, imports) is kept as it came and not read.
- State words match Mission Control and the missions page: `state_draft` reads "Not published yet"
  (never published), `state_inactive` and `home_filter_inactive` read "Paused" (published,
  switched off; the filter lists only those). "Draft" is the editable version. The MC hint
  `desktop.mc_flow_cancel_in_easydrag` uses EasyDrag's Stop.
- The editor follows `flows_changed` for its flow (`flow_id`): `enabled`/`published` re-read the
  record (`refreshRecord`: chip, Active switch, Run now), `deleted` goes home with a notice
  (`goneElsewhere`). An event without an id and reason `deleted` or `enabled` is "maybe mine": the
  record is read again and `FLOW_NOT_FOUND` means deleted. Every delete (editor, start-page card,
  elsewhere, a broadcast on the start page) calls `core.forgetFlow(id)`, which drops the four
  per-flow keys (`core.FLOW_KEYS`: emergency copy, view, effects-ok, test-trigger).
- `flows_changed` with reason `checked` (no flow id) follows the server's start-up check: the start page reloads and
  the editor re-reads its record. `live_issues` shows `state_live_outdated` (`.ed-chip--risk`, `live_outdated_hint`
  as title and `.ed-sr-only` text) on the card and in the header, the latter only while `ed.flow.live_revision` is
  the revision the issues came with; `timers_running: false` shows the `timers_stopped_*` banner (`.ed-home-stale`
  look) on the start page and a `.ed-chip--warn` in the header.
- The key-value editor (`ED.fields.keyValue`) compares trimmed keys; while two rows share one it emits nothing and
  marks them `aria-invalid` with `aria-describedby` on its one `[data-ed-kv-error]` message. An unedited value keeps
  its JSON type (an object is cloned); an edited value is the field's text.
- Below 900 px (`@container`) the header hides the text of Stop and of the Active switch, visually
  only (the `.ed-sr-only` pattern, not `display: none`). Both carry the same words as `aria-label`
  and `title`, so the visible label is part of the name; Stop's name follows its text to "Stopping…"
  (`setStopPending`). A new header button that loses its text needs the same.
- Run now is off while a published flow is paused (`runNowState`), in the ⋯ menu, the Flow menu
  and a card's menu, with `disabledHint` (`error_flow_disabled`), a menu-item field the desktop
  draws as the disabled item's tooltip; a stale click gets the server's 409 `FLOW_DISABLED`.
- Undo affordances live in `easydrag-history.js` (`ED.history.create(ed, {host, toastHost, undo, redo, focusCanvas})`): the header group `[data-ed-history-host]` holds Undo/Redo (`data-ed-history`, `aria-disabled`, never `disabled`, hidden on a read-only desktop) and `.ed-toast` offers Undo after `interact.removeSteps` emits `steps-deleted` ({names}); the toast is no live region (the canvas announced the delete), closes after 8 s or on any other model change, and its Undo acts only on the version right after the delete. No delete confirmation.
- At an editor width of 560 px or less (`@container`), Runs, Test and Run now are hidden in the header and `moreMenu` lists Runs and Test first (`headerNarrow`). The state chip goes, but the Paused chip (`[data-ed-paused]`), "Republish needed" (`[data-ed-live-outdated]`) and the timers warning (`[data-ed-timers-off]`) stay: the CSS exception lists all three. A skipped Run now (202 `{status: "skipped"}`) notifies `run_now_skipped`. The run view hides the revision line and a "Saved" chip; its banner offers `rerun-viewed` (Test again with this data: the run's `trigger_data` prefills the test dialog, which still lists effects and asks; nothing is remembered unless ticked). `canRerun` hides that button for stub trigger data (the store keeps oversized data as `{_preview}` or `{_preview: "", _omitted}`: a test would run real effects on the stub) and while a run is going (the viewed run is active, or a live run is parked: `runs.hasParked()`); a click that still finds a run going keeps the run view and notifies `run_going`. `GET runs/{id}?include=doc` without `doc` carries `doc_missing` (`version_pruned` shows `run_view_pruned`).
- The JSON editor (`fields.jsonEditor`) keeps `aria-invalid` live on every input but writes the `role="alert"` text only when the error appears and never rewrites identical text (a screen reader would read it on every keystroke); change and blur announce a standing error afresh.
- The editor builds in two steps: `create` makes the shell and `build(app, loaded, opts, guard)` fills it. A build aborted by a dispose, a failure or a newer open cleans up what it made so far.
- Run events repaint in place without markup: `runs.js` logs touched steps in `ed.run.changes`, the canvas (`paintCard`) and wires (`onRun`, `syncPill`) follow that log; a new run object repaints all states. `ed.renderStats` ({cards, paints, wires}) is guarded by the b5 F6 check (300 steps, 1000 events: no card render). `model.incoming/outgoing/edge` use an adjacency index rebuilt on `docGen` and return copies.
- Storage: the clipboard fragment is stored as `{at, frag}` and expires after 24 h (unstamped counts as expired); refused emergency writes back off 0.5 to 8 s and stop after 6 (flushCopy/flush/dispose still write); the port tip is shown until `aurago.easydrag.hint.ports`. Quick-add chooses on `click` (a press only keeps focus).
- DOM modules get the editor state `ed` and talk over `ed.bus`, never through other modules'
  DOM. Focus moves synchronously when a popover or dialog opens; `.ed-app [hidden]` forces
  `display: none`. `ED.canvas.portLabel(t, node, port)` is the one port label. An edge whose
  source port the step does not have is drawn dashed under the card (`.ed-edge.is-broken`)
  and stays selectable; a missing step draws nothing. The server still reports
  `EDGE_PORT_INVALID`.
- `ai.decision` uses `dynamic_outputs: "decision_choices"`: route ports are keyed by stable
  option ID (`choice_<id>`) and move with their option; evaluate ports stay `out`/`uncertain`.
  Keep `model.outputs` and `canvas.portLabel` in sync, and never key a choice wire by its row
  number. The nested question and image-reference forms preserve order, stable IDs, templates
  for instructions/media references, and the server caps; the JSON state field can switch to a
  template. Its detail error-policy picker follows `error_modes` from the catalog.
  Converting `score` to `choice` removes option values while retaining IDs, labels and wires.
- The step inspector (`ED.detail`, `.ed-detail`, `role="region"`) is a non-modal pane appended to
  `.ed-body`. It is not portaled, not `aria-modal`, and has no focus trap or outside-click close.
  Input, Parameters and Output are tabs at every width. Below 760 px (`PANE_BREAK`, class
  `is-inspector`) the graph and the inspector take turns. EasyDrag is not a mobile wide window.
  `appWindowMinSize` is 360×480. The responsive check also uses a 320 px viewport. Supported
  appearances there are Standard, Fruity light and Fruity dark. There is no Standard-light theme.
- The Settings pane of the inspector ends with Connections (`[data-ed-connections]`, `renderConnections`
  in `easydrag-detail.js`): the step's incoming wires first, then the outgoing ones, each as "From/To
  {step}" with the port the wire leaves (`ED.canvas.portLabel`). It is the keyboard way to what the
  canvas shows on wire hover only: Remove (`core.removeConnection`) and Insert a step (the same
  quick-add as the wire's insert button, `ED.palette.requestInsert`, which takes a `restoreFocus`
  callback so the focus returns to the row, not the canvas). Both buttons name both ends in
  `aria-label`. A read-only desktop and the run view list the wires without buttons. The list redraws
  on every model change while the Settings tab shows (a signature of the rows keeps typing from
  redrawing it), also for an undo that touches no step; a redraw keeps the focus in the list (same wire,
  else the row now at that place, else the heading `[data-ed-conns-title]`).
- Keyboard model of the canvas: Tab, Space and Enter keep ordinary control behavior, so the canvas
  is not a focus trap and Tab does not add a step. The canvas is one tab stop and the zoom bar
  follows. With five steps and no selection the browser smoke test counts five stops (canvas plus
  four zoom buttons). A selected card's six tools join that order (`syncTools`; `tabindex="-1"`
  otherwise), eleven stops, and `.ed-node:focus-within` shows a focused tool's toolbar even on an
  unselected card. Arrow keys move the selection (announced through `core.announcer`). Enter opens
  the inspector. Delete/Backspace, D and Ctrl+D act on the selection. C opens the connect picker.
  Card tools are Add, Connect, Test step, Disable, Duplicate and Delete. Add, a port's Add button
  and a wire dropped in empty space open quick-add; Add uses the first outgoing port. Tapping a
  port without dragging opens the picker with that side and port fixed, without changing the
  document or undo/redo history. The picker lists every compatible endpoint and does not guess a side. Ctrl+K, with exactly
  one selected step that has an outgoing port and while the draft can change, opens quick-add
  already wired from that first outgoing port. Otherwise Ctrl+K focuses palette search. It does
  nothing in the run view, and it does not run when a pending field flush fails. Run events and
  hints change badges, run status and the error box in place (`paintState`); markup, tools
  included, is rebuilt only when the step itself changes, so a focused tool keeps focus during a
  run. A rebuild hands that focus to the new button with the same `data-ed-node-tool` while the
  card is selected, else to the canvas. D, Ctrl+D and Del remain keys; Test step is also in the
  inspector (Enter). The screen-reader step list (`.ed-node-list` in `section.ed-node-region`, `canvas.listRegion`,
  appended to `.ed-body` beside the canvas: `role="application"` would keep browse mode out of it) duplicates
  the arrow keys, so its buttons are `tabindex="-1"`. User actions are announced in the canvas's live region
  (`canvas.announce`, `ed.announce`, `core.say`; WCAG 4.1.3): steps added (palette, quick-add,
  paste, duplicate), deleted, turned off or on, moved by Shift+Arrow (one text 500 ms after the last
  key, never for a pointer drag), undo and redo (only when there was something to undo), a removed
  connection (wire button, Delete, inspector), a started test or live run and each step that fails
  during a live run (once per step). One step gives its name, several the count, as separate keys
  (`announce_*_one` takes {name}, `announce_*_many` {count}: languages count differently). Not
  announced: autosave (the save chip has its own live region), a cut (its copy says "Copied"), the
  inspector's switch for a step turned off (a switch speaks its own state), steps that went well.
  Do not add announcements to the model's change descriptor; announce from the user-action site.
  Accessibility acceptance requires screen-reader and user
  task checks, not screenshots alone. The minimap patches selection and run status on existing
  rects (`minimapMarks` / `selectionTick`) without rebuilding geometry. Performance acceptance
  requires measured p95 interaction latency at most 100 ms with the hardware, browser and
  document size recorded. Screen changes move the focus: leaving
  the run view to the canvas, the start page to New flow and then to the card of the flow just
  left (not for Mission Control's New flow or the templates link), the shell's error card to Retry.
- Home has four list states. A successful list shows the cards. A later refresh failure keeps that
  list in memory only, shows the stale banner, and turns creates off until retry (`staleError`).
  An empty successful list offers templates, New and Import. The first load, `FLOWS_DISABLED`, or
  auth loss (`authLoss`: every 401 and 403, plus `unauthorized`, `forbidden`, `permission_denied` and `desktop_scope_required`; not `desktop_readonly`) replaces
  the grid and does not keep protected cards; auth loss also clears templates. Nothing for those
  failures is written to storage. Create, import, duplicate, delete, toggle and run follow
  `createOff`. Opening a flow stays available when the list is only read-only.
- Strings: `easydrag.ui.*`, `easydrag.decision.*`, `easydrag.availability.*` in
  `ui/lang/easydrag/<16>.json` (shared with the server's catalog keys); built keys (`core.tr`)
  belong to a family of `TestEasyDragUIKeysExistInAllLocales`.
- `createApi` (`easydrag-core.js`) errors carry `err.body.code`, shown as
  `easydrag.ui.error_<code>`. SSE (`/runs/{id}/events?after=<seq>`) is idempotent and
  reconnects at once on `event: resync`. Only `FLOWS_DISABLED` (flows switched off) gets the
  lock card with "Open settings" (`easydrag.js` `showError`, the start page's `errorCard`); the
  other 503 codes (`FLOW_MISSION_CONTROL_UNAVAILABLE`, `FLOW_RUNNER_STOPPED`,
  `FLOW_VAULT_UNAVAILABLE`, `FLOW_REQUEST_CANCELLED`) show their own sentence with Try again
  (`a1008` checks in `scripts/test-easydrag-extra5.mjs`).
- Test effects (`ED.runs.effects`): each step that runs counts with its catalog effects (default
  params) and those of its real params from `GET publish-preview` (`CollectEffects`). Without
  the preview, or with a running step whose catalog entry is risky without effects (a failed
  effects hook), the dialog says so and never starts silently; an edit after the flush is checked
  again before Run. Run confirms the effects shown (`ed.effectsConfirmed`; with "Don't ask again"
  also `aurago.easydrag.effects-ok.<id>`) only once the test POST succeeded: a refused or failed
  test confirms nothing and the next attempt asks again (`a1008` checks in
  `scripts/test-easydrag-extra6.mjs`). The dialog warns that those effects are real
  (`easydrag.ui.effects_real_stop`). Stopping does not roll back steps that already ran. A step
  test counts what the engine runs (`engine_state.go` `run`, `fireTrigger`, `collectReady`); keep
  the two in step.
- `startTest` and `runLive` return without posting while the start dialog is open (`starting`),
  a start POST is in flight (`startPost`), or `isRunning()` (a live run in this editor, not a
  run view and not a stream that was given up). `trackStart` returns that in-flight promise
  instead of posting again. An uncertain result is not sent again. Autosave timing is unchanged.
- Runs that have not ended (`ED.runs.isActive`) get Stop in the drawer and the run-view banner
  (`runs.stopRun`: any run but a test asks first, 409 `FLOW_RUN_FINISHED` refreshes quietly, a
  run no stream here shows is asked for after 1 to 16 s); `run_finished` refreshes both
  (`runs.runFinished`: 250 ms, at most 1 s). The drawer (`aside.ed-drawer`) is appended to
  `.ed-body`, below the header and the run-view banner. The secret field deletes the chosen
  secret and warns with `used_by`.
- Saver: 1 s after the last change; network errors, 5xx, 429 and `FLOWS_DISABLED` go `offline`
  (retry 5 s, doubling to 60 s), `PERMANENT_CODES` and a 4xx without a code go `failed` (a retry
  button), `FLOW_INVALID` waits for the next change. The emergency copy is written at most once
  per 500 ms while changes keep coming (a drag); a held-back change is written by a later change,
  `saver.flushCopy()` (the interact bus event `gesture-end`, `pagehide`, a `visibilitychange`
  to hidden), the next save, `flush()`, `dispose()`, and at the latest by a trailing write
  500 ms later, so a crash loses less than 500 ms; never once the draft is saved. A failed
  `storage.set` leaves the copy pending (`copyPending` stays, `copyAt` does not move) so those
  same paths try again. `dispose` writes before it marks itself disposed when the model is newer
  than the saved version, including while an older save is still in flight. A refused write on
  dispose is tried once more immediately and arms no timer; a timer already waiting is cleared
  with the saver. Model change
  sets and `model.node` look ids up in Maps (`node` rebuilds its Map after each write of the
  node list); `test-easydrag-extra5.mjs` guards a 200-step drag and the 100-to-400-step scaling.
- Run view: `ed.model` is the stored run's document there, `ed.draftModel` always the draft. A
  dropped duplicate step is announced once, on the draft model only, with the existing
  `easydrag.ui.issue_node_id_duplicate` (also after a restore). The
  hints (`publish.refreshIssues`) validate the draft, wait while the run view shows and run again
  on exit; a live run that starts meanwhile is parked (`runs.attach`) and followed on exit. The
  issues foot button is `aria-disabled` in the run view and `openIssues` returns at once there;
  leaving the view renders the button again so it works.
- Dialog holds (`holdAction`, a 429's Retry-After) are kept per button and end when the dialog
  closes; a tree drag's document listeners (`dragend`, `drop`) end with the drag or the next one.
- `flushPendingEdits` runs before save, test, publish, switching steps, the run view and leave.
  It writes the debounced note, calls `form.flush()`, and commits the settings data name. Template
  `flush` writes the open edit and leaves the caret, suggestion list and `is-editing` in place;
  it returns false while an IME composition owns the text. `form.flush` also calls native
  `edCommit` on plain, number and JSON controls, including fields nested in the form. Nested
  decision inputs that have no `edCommit` commit on change, and only the focused one is flushed
  this way. Invalid JSON, a number outside its range, an IME composition, or a data name the
  model refuses stays in the control and blocks the action. The typed text is not replaced.
- Window menus pass canonical keys ("Ctrl+S"); the shell dispatches a `shortcut` item before the
  editor sees the key, a `shortcutHint` ("?") is only drawn. Mod+S is always prevented in the
  editor. It saves only when no real EasyDrag dialog (`.ed-modal-backdrop`) and no
  `.vd-context-menu` is open. The inspector is not that dialog: Ctrl+S, Ctrl+Enter and Ctrl+K
  still run. `unlessModal` waits for a real dialog or a context menu. `unlessOverlay` also waits
  for quick-add and the restore offer. Under a real dialog, Ctrl+Enter, Ctrl+K and Ctrl+F are
  prevented (the browser would take Ctrl+K to the address bar). Text fields keep native undo and
  IME. The start page disables New and Import while `createOff` is true (read-only desktop,
  `FLOWS_DISABLED`, a stale list, or a lost session). Reload stays enabled. A later list that
  succeeds enables New and Import again unless the desktop is read-only.
- `editor.leave()` aborts a drag, runs `flushPendingEdits` (false keeps the editor open),
  flushes the saver and shares one in-flight promise. `pagehide` and a hidden page flush a
  pending note before the emergency copy. Sessions
  and notification contexts keep `flowId` only when it matches `^flow_[a-z0-9]{10}$`.
- Opening: a stored view `{cx, cy, zoom}` per flow (editors wider than 560 px), else the readable
  fit (zoom ≥ 0.8, trigger first); below zoom 0.7 cards show labels only. Under 900 px the
  palette floats over the canvas; the run view keeps it hidden and inert. Do not add a portal
  or a pinned wide window for this layout.
- Other surfaces: Mission Control (`MissionControlTriggers.isFlow`/`isUnpublishedFlow`/
  `upcomingRun`), `ui/js/missions/main.js`, `ui/cfg/flows.js` and the dashboard's cron list
  (`managed_by: easydrag`). User docs: manual chapter 24.
- Verify: `node scripts/test-easydrag.mjs` (with `-extra.mjs` to `-extra7.mjs` and `-ux-batch5.mjs`),
  `npm run test:mission-control`, `npm run test:dashboard-cron`, `npm run test:missions-page`,
  `go test ./ui -run 'EasyDrag|MissionControlShowsFlow|StandaloneMissionsPage'` and the opt-in
  browser tests `TestDesktopEasyDragBrowser`, `TestDesktopEasyDragDecisionBrowser` and
  `TestDesktopEasyDragUXBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`;
  `ui/desktop_easydrag_browser_test.go`, `ui/desktop_easydrag_decision_browser_test.go`,
  `ui/desktop_easydrag_ux_browser_test.go`; screenshots in `reports/easydrag/`). Strings stay in
  all 16 `ui/lang/easydrag/<locale>.json` files. Backend: `internal/flows`,
  `internal/server/flows_*.go`. EasyDrag source stays owned by the parent UI contract.

## CYD Creator
- Builtin app `cyd-creator` (category dev, `Requires: cyd_creator`, opens maximized). It must stay a builtin
  loaded into the `/desktop` document, never an iframe app: flashing needs Web Serial there. Server contracts
  (HTTP API, SSE, step and message codes): `internal/server/AGENTS.md` (CYD Creator) and
  `internal/cydcreator/AGENTS.md`. The `inst` object and the 4a↔4b seam are also specified in the local plan
  overview `docs/superpowers/plans/2026-10-10-cyd-creator-overview.md` (git-ignored).
- Modules share `window.CYDCreatorParts` and load in the order of `DESKTOP_APP_ASSETS['cyd-creator']`:
  core, api, events, the ten designer modules of plan 4b (directly after events, above the "plan 4b"
  marker comment), code, dock, agent, home, the modules of plan 5 below the "plan 5" marker comment,
  `cyd-creator-app.js` last. Keep both marker comments. The shell finds
  `window.CYDCreatorApp = { render, dispose, instances }` through `appGlobalName`. Each shell module stays
  below 1100 lines and starts with a purpose comment, `'use strict'` and `window.CYDCreatorParts`
  (`checkCYDCreatorModule`; `knownOversizedContinuations` does not exempt them; `app.js` is near 900 lines).
- `cyd-creator-app.js` builds one `inst` per window and composes dock, code, agent, home and, when
  `P.designer` exists, the designer. `inst.flushAll()` → Promise<boolean> saves layout and code. `inst.api` is
  `Object.create(P.api)` whose `startJob` flushes first and throws the save error when that fails (override
  single methods, never replace `P.api`). `inst.uploadAsset(file)` (not `inst.api.uploadAsset`) flushes the
  layout first and then re-reads it. `inst.confirm` and every `P.dialog.open` pass `owner` (a `P.util.bag`),
  so dialogs close with their window or project; `inst.notify` stays safe after dispose. Dock, code, agent and
  designer exist per project and are disposed in `teardownProject`; app.js checks `inst.projectSeq` before
  async work touches the window.
- Designer seam (plan 4b): `inst.layoutStore` exists only when `P.model.createStore(project, {limit: 300,
  coalesceMs: 1200})` exists; only store changes with origin local, undo or redo autosave, and none while
  read-only. The designer is created only when `P.designer.create` and the store both exist and gets
  `mountScreens`, `mountPalette`, `mountCanvas` and `mountInspector`. Reveals go only through the bus event
  `designer:reveal`. For the designer the bus also carries `lock:changed(locked, jobId)`, `mode:changed`
  and `layout:issues`; it may call `inst.assetUrl(id)` and `inst.code.ensureHook({name, event})`.
- `P.api` wraps every endpoint and throws `ApiError {status, code, message, details}` (status 0: the
  request never reached AuraGo); `P.errors.message(t, err)` shows `cyd_creator.error.<CODE>` (`CYD_BUSY`
  with `details.reason` `publication_review` has its own sentence; `CYD_FILE_LIMIT` and `CYD_BUSY` `export` use
  the plan 5 transfer strings, "CYD Creator flash, monitor and devices").
- Event stream (`cyd-creator-events.js`): resumes with `?after=<last seq>`, coalesces the backlog until
  `stream_ready` and reconnects with 1–30 s backoff (±20 % jitter); parts listen to `sse:<type>` or the
  semantic bus events of the `inst` contract. An event without an id line (the browser keeps the previous
  `lastEventId`) counts as seq 0. `stream_ready` carries `{after}`; with `reload: true` the cursor moves to
  `after` and the window reloads through the internal `reloadProject(inst, {keepEdits: true})`: unsaved layout
  edits stay and meet the new version through the conflict choice (`inst.reloadProject()` always reloads the
  layout). A stream cut off during the catch-up resumes from the catch-up's
  start (`resumeFrom`, not `after`). Each failure in a row emits `events:failed(n)`; after 3 the app probes
  `GET /projects/{id}`: 404 returns to the start screen, 401, `unauthorized`, `desktop_scope_required` and
  `CYD_DISABLED` stop the stream with a banner and Retry (`data-cc-stream-stopped`,
  `data-cc-action="reconnect"`), anything else keeps reconnecting. The first `stream_ready` after a failure
  re-reads the summary: a server restart ends in-memory state (the publication fence) without an event.
- Publication review: a project with `publish_review` shows the danger banner (`data-cc-publish-review`,
  `cyd_creator.error.publication_review`) and Build stays disabled; a layout or code save refused for
  `publication_review` re-reads the summary, so the banner appears.
- `P.autosave` serialises saves (1 s debounce, If-Match version for the layout, sha for files). A 409
  asks Keep editing / Load saved version / Overwrite and never overwrites silently; while that choice is
  open, `flush()` returns it instead of sending the stale version again. 423 waits for the unlock;
  network/408/429/5xx and `CYD_BUSY` without a reason or with `file_open` retry after 5 s doubling to 60 s,
  `CYD_BUSY` `publication_review` fails at once. Home and window close flush first and ask
  before changes are lost. The layout autosave also saves layouts with validation issues (the designer
  applies edits without a gate; the server answers with `issues`, the shell emits `layout:issues`); only a
  422 for an unparseable layout counts as failed. The project lock (`lock_changed`) makes layout and code
  read-only.
- Read-only: `inst.readonly` is the desktop policy (`ctx.readonly`, then `aurago:desktop-policy` events on
  `document`) or `caps.readonly`; a change re-renders toolbar, code editor, agent panel and start screen and
  emits `readonly:changed(bool)`. Read-only viewers can still cancel a job or a build (the server allows the
  cancel routes in read-only mode), so never gate a Cancel button on `inst.readonly`.
- The agent panel never moves the focus or switches the right panel by itself; a running job only shows
  the dot on its tab. Without plan 4b (`P.designer` absent) Design mode shows a placeholder. Job steps and
  system chat lines render `cyd_creator.step.<code>` / `cyd_creator.msg.<code>` with their `vars` (server
  table `internal/cydcreator/texts.go`; an `error_code` var shows that error's sentence), else the server's
  English `text`.
- Code uses full `cyd_creator.*` key literals (the i18n lint tests read them); strings live in
  `ui/lang/cyd_creator/<16>.json`. Plan 4b owns the families `theme.*`, `board.status_*`, `widget.*`,
  `group.*`, `prop.*`, `event.*`, `action.*`, `transition.*`, `issue.*` and `designer.*`; shell code reads
  them with `t.or(key, fallback)` / `t.has` (the dock falls back to the English Issue message). `t(key,
  vars)` fills `{{name}}` and `{name}`. Shell CSS classes stay distinct from the designer's global ones
  (`.cydc-tag`, `.cydc-pill`, `.cydc-form-*` here; `.cydc-badge`, `.cydc-chip`, `.cydc-field*` belong to 4b), and the
  designer rules live at the end of the stylesheet in the section that starts with
  `/* ==== CYD Creator designer (plan 4b) — begin` (the theme-token test stops there). No
  alert/confirm/prompt: `P.dialog` (core) and `inst.confirm` render inside the window, and one shared key
  listener lets only the newest dialog in the focused window handle Escape and Tab.
- Config section `ui/cfg/cyd_creator.js` (strings in `ui/lang/config/cyd_creator/`): `cyd_creator.model` is a
  provider ID from the shared provider dropdown ("" = main LLM); its sidebar icon is slot 123 of the 11×12
  sprite (`ui/AGENTS.md`).
- Verify: `npm run test:cyd-creator-shell`, `go test ./ui -run '(?i)CYDCreator'` and, with
  `AURAGO_RUN_BROWSER_SMOKE=1`, `TestDesktopCYDCreatorShellBrowser` (screenshots in `reports/cyd-creator/`,
  backend `ui/testdata/cyd-creator-fixture.js`, whose EventSource keeps `lastEventId` like a browser) and
  `TestConfigCYDCreatorSectionBrowser` (the real config page, no screenshots).

## CYD Creator designer

- `cyd-creator-icons.js` loads before the inspector and provides the image/button
  library through `inst.iconCatalog` and `inst.importIcon`, with the same flush,
  project-switch and autosave protection as image upload. Snapshot a selection
  before awaiting import. Commit assignment as one undoable transaction and
  retain imported assets during concurrent local edits. Lazy previews use
  RGB565 PNGs or alpha masks; animate only the selection, respect reduced
  motion, and dispose observers with the dialog. Names/filters/errors belong
  to all 16 CYD locales. Browser test: `TestDesktopCYDCreatorIconsBrowser`.
- The eleven designer modules load right after `cyd-creator-events.js` in this order (`cydDesignerScripts`,
  `TestCYDCreatorDesignerModulesLoadInOrder`): `cyd-creator-geometry`, `-model`, `-render` (pure, run in Node),
  `-canvas`, `-interact`, `-palette`, `-inspector-fields`, `-icons`, `-inspector`, `-screens`, `-designer` last; each stays
  below 1100 lines (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`). `P.designer.create(inst)` returns
  `{mountCanvas, mountPalette, mountScreens, mountInspector, reveal, dispose, state}` (`state`: the internal
  designer object, read-only, for tests and diagnostics). Snap and grid persist in `localStorage`
  `aurago.cyd-creator.designer.view`; the clipboard is shared by all designer windows of the document and stores
  the source project id. Pasting into a different project (`sanitizeForeign`) drops catalog asset props whose id
  is not in that project, and with them `icon_frames`, `icon_animate` and `icon_origin`; drops `icon_frames` that
  are present but not exactly six known ids, and `icon_animate` with them; and removes `goto_screen` bindings
  `findScreen` cannot resolve. A paste that removed something announces `designer.paste_adjusted`. The same
  project, or a missing id, keeps the copied tree.
- Every edit is one `inst.layoutStore.change` through `d.commands.run`/`transact` with ops from `P.model.apply`;
  the store keeps frozen snapshots, at most 16 MiB of history and the origins `local`/`undo`/`redo`/`remote`. The
  ops are the structural part of Go `ApplyOps` (`internal/cydcreator/patch.go`: fresh ids, catalog default sizes,
  `goto_screen` names stored as ids, `tab` only under a tabview, `set_settings` takes settings keys only) plus the
  designer-only `set_project`, `reorder_widget`, `reorder_screen` and a last-screen guard. No validation gate (the
  server reports issues after the save); only a structurally impossible op throws (`ModelError`
  `CYD_INVALID_LAYOUT`, shown as `designer.op_failed`). New widgets come from `P.model.newWidget`/`cloneSubtree`:
  the server decodes layouts strictly (no unknown fields, integers only).
- Shell bus: an identical `layout:issues` list is ignored (a re-render would reset a name being typed and make
  gesture previews flicker); `lock:changed(locked, jobId)` and `readonly:changed(bool)` end a running gesture and
  re-render the parts (read-only = `inst.readonly || inst.isLocked()`, read at each use); `mode:changed` to `code`
  ends a gesture (the hidden canvas keeps its zoom and runs no keys); `designer:reveal` → `reveal({screen,
  widget})` opens the tab pages on the path, scrolls and flashes. Parts talk over the designer bus `d.bus`
  (`selection`, `screen`, `project`, `view`, `issues`, `lock`). A store change emits `project` `{before, label,
  origin, screenChanged}`; when it took the active screen away (deleted, Wi-Fi off), `screen` fires first and
  `screenChanged: true` lets a part that renders on both render once. Non-local changes end a gesture, remote
  ones flash the changed widgets.
- Uploads go only through `inst.uploadAsset(file)` (the static test forbids `P.api.`, `inst.api.`, `layoutSaver`,
  `innerHTML` and `insertAdjacentHTML` in every designer module). It saves pending edits, the server adds the asset
  to the layout and the window replaces the store remotely (`resetHistory: false`) before it answers; the
  inspector then sets the prop (an image also takes the asset's size) and pushes the asset only when it is missing
  (an edit during the upload kept the window's copy; the If-Match conflict choice follows). The browser refuses
  what the server would: only PNG/JPEG (magic bytes) or SVG (rasterised to PNG at the widget's size), at most 2 MiB.
- Gestures preview in the DOM and commit one change on release; `pointercancel`, lost capture, window blur, a
  screen switch, the lock and non-local changes cancel them. Coordinates are device pixels: the screen scales with
  `transform: scale(zoom)` (integer 1–4, images pixelated), the overlay does not, and children are relative to
  `P.model.contentBox` (as `validate.go` bounds them: the whole panel, a tabview minus its tab bar). Committed
  move, nudge, resize position and place (`align`/`distribute`) run through `P.geometry.clampInto` into that
  box: the size stays, a widget larger than the frame sticks to its top-left, and a group stays rigid through
  `clampDelta`. Snap runs first; an axis the clamp moves drops that axis's guide. The marquee and Ctrl+A use
  `P.model.visibleIds` (tree order, hidden widgets and their children out, only the active tab page) and
  absolute rects. Multi-select z-order follows each parent list, not click order: front is the unselected
  widgets then the selected ones, back is the reverse, and forward or backward is one shared step.
  `reorderIndexes` emits `reorder_widget` ops that stay valid under splice-out-then-insert; an unchanged order
  emits nothing. Locked widgets stay in the z-selection. Read-only:
  palette entries, inspector fieldsets and screen tools are disabled, no resize handles, commands return early;
  screens still open and layers still select.
- Keys: `interact.handleKey(e)` runs a designer shortcut and returns true when it used the key (Delete/Backspace,
  arrows nudge 1/8 px, Ctrl+Z/Y/Shift+Z/C/X/V/D/A, `+` `-` `0` zoom, Escape); `interact.gestureKey(e)` takes the
  keys while a gesture runs (Escape cancels, other designer keys are swallowed). The canvas listens itself;
  palette, screens panel and layer list forward their keys and on true call `preventDefault()` and
  `stopPropagation()`. Enter, Delete, Backspace and F2 outside form fields never leave the canvas (toolbar
  included), the palette or the screens panel, and the inspector stops Enter, Delete and F2 without
  Ctrl/Cmd/Alt; a button's native Enter click still runs (no `preventDefault()`). They keep the desktop from
  opening, deleting or renaming its selected icons; the desktop itself ignores keys from app windows since main's
  3ff6eed26, so after that lands this is belt-and-braces. Ctrl/Cmd+S, Alt combinations and Tab bubble on. The
  canvas is one tab stop with a live announcer and a shortcut hint; the layer list reaches every widget with a
  roving tab stop (ArrowUp/Down, Home/End win over nudging); Enter/Space on a palette entry adds the widget in the
  middle; F2 renames and Alt+ArrowUp/Down moves the focused screen. The browser smoke presses Enter, F2 and Delete
  (on the toolbar also Backspace) in the canvas, its toolbar, palette, layers, screens and inspector and checks
  that none reaches the document.
- Names match `^[a-z][a-z0-9_]{0,31}$`, are unique and not reserved (`P.model.RESERVED_NAMES`, prefixes and the
  `_series` suffix mirror `validate.go`); proposals (`btn_1` …) skip reserved names. The inspector keeps an
  invalid name visible with its message and stores only valid names.
- Layout values are untrusted (agent-written or imported), so `P.render` validates its input itself: colours reach
  CSS only through `hexOr` (`#RRGGBB`, `validate.go` `colorPattern`), numbers clamp to the catalog and
  `validate.go` ranges, widget sides to 1–2048 (`maxWidgetSide`), strings and lists to their limits, enums to
  their options; texts are text nodes. In the canvas, widget visuals come only from `P.render.widget(spec, w,
  theme, assetUrl, {activeTab, screenTitle, settings})`, the screen colour only through `P.render.colors`, and
  widget boxes reach CSS as finite integers clamped like the renderer clamps them. Device colours come only from
  `inst.caps.themes` and style overrides (no hex literal in render, canvas or interact). The Node test reads
  `limits.go`, `validate.go`, `ids.go` and `assets.go` and pins the mirrored values: `P.model` `LIMITS`,
  `NAME_RE`, `RESERVED_NAMES`/`_PREFIXES`/`_SUFFIXES`, `TRANSITIONS`, `ACTIONS`, `ORIENTATIONS`, `WIFI_MODES`;
  render's `HEX`, `ELEMENT_ID`, `MAX_SIDE`, `MAX_TEXT`; canvas `MAX_COORD`; `P.geometry.MAX_SIDE`;
  `P.designerInspector.LIMITS`; `P.designerFields.HEX` and `MAX_UPLOAD_BYTES`. Hand mirrors without a source
  check: render's inline style clamps (the values of the pinned `LIMITS.style`), `P.model.STYLE_KEYS` and the op
  semantics of `patch.go` (behaviour tests only).
- CSS: the designer section ends at `/* ==== CYD Creator designer (plan 4b) — end`; keep that end marker, plan
  5's CSS test requires it before its own section. `TestCYDCreatorDesignerStaticContracts/css` checks: every
  selector starts at `.cydc-app` or the body-level `.cydc-drag-ghost`; literal colours (hex, `rgb()`/`hsl()`, named)
  stand only in the token block `.cydc-app, .cydc-drag-ghost {…}` (marks over the device preview that read on every
  device theme); every `var()` in the section is a `--cc-*` token the stylesheet defines, and the file has no
  `--ccd-*`; `.cydc-canvas-stage` padding = `PAD` (24), `.cydc-device-frame` padding = `BEZEL` (14) and
  `.cydc-device-label` margin-top + min-height = `LABEL_H` (the `cyd-creator-canvas.js` constants `fitZoom`
  subtracts); non-active screen rows stay on one line and their tools change only `visibility` on hover or focus;
  no rule sets the dock's height (`cyd-creator-dock.js` limits only its first, unsaved Design-mode height to
  30 %); the `@container cydc-app (max-width: 820px)` rule shows `.cydc-left` again in Design mode. Conventions
  without a test: status text through the `*-ink` tokens, solid fills `--cc-accent-deep`/`--cc-on-accent`; at
  560 px the left column becomes a strip above the canvas (Code mode keeps the shell layout); `@media
  (forced-colors: active)` keeps the device preview's own colours and marks selection, handles, pressed tools and
  chips and the shell's Design/Code switch with `Highlight`.
- Preview fonts `CYD Montserrat` 500 and `CYD JetBrains Mono` 400 (latin and latin-ext `woff2`, fontsource 5.3.0)
  live in `ui/fonts/` with their OFL texts (`Montserrat-LICENSE.txt`, `JetBrainsMono-LICENSE.txt`), are listed in
  `THIRD_PARTY_LIBRARIES.md` §6 and are declared in the designer section (loaded only with the app); Inter comes
  from `fonts/fonts.css`. `TestCYDCreatorDesignerFontsVendored` checks files, licences and inventory rows.
- Strings: `cyd_creator.designer.*`, the families `widget.`, `group.`, `prop.`, `event.`, `action.`,
  `transition.`, `theme.`, `board.status_`, `issue.` and `designer.align_`, `designer.z_`, `designer.font_`,
  `designer.wifi_`, `designer.opt_<prop>_<option>`; orientation labels reuse plan 4a's `cyd_creator.orientation.*`.
  `TestCYDCreatorDesignerKeysExistInAllLocales` takes the literal keys and `'prefix' +` families from the designer
  modules and the family members from the Go catalog, checks all 16 locales (English placeholders, German without
  the formal address) and runs in reverse: a family no designer module builds fails, and so does any key under the
  designer-owned prefixes (`designer.`, `widget.`, `group.`, `prop.`, `event.`, `action.`, `transition.`,
  `theme.`, `board.status_`, `issue.`) that no designer module uses, so plan 5 modules add no keys there. Every
  literal `t.or(key, 'fallback')` must equal `en.json`: change both together.
- Verify: `npm run test:cyd-creator-designer` (pure modules, then the DOM parts with the real designer on a fake
  DOM), `go test ./ui -run '(?i)CYDCreatorDesigner'` (`TestCYDCreatorDesignerStaticContracts`, load order, keys,
  fonts, capabilities fixture) and, with `AURAGO_RUN_BROWSER_SMOKE=1`, `TestDesktopCYDCreatorDesignerBrowser` (real
  core and designer on `ui/testdata/cyd-creator-designer-fixture.js`, both `prefers-reduced-motion` modes,
  screenshots in `designer/<motion>/` under `AURAGO_BROWSER_ARTIFACT_DIR` or `reports/cyd-creator/`) and
  `TestDesktopCYDCreatorShellBrowser` (the real shell: a palette add autosaves with If-Match). After a catalog,
  theme, font, symbol, library, language or classic-CYD board change regenerate
  `ui/testdata/cyd_creator_designer_caps.json` with
  `go test ./ui -run TestCYDCreatorDesignerCapsFixtureMatchesCatalog -args -update-cyd-designer-caps`.

## CYD Creator flash, monitor and devices
- Plan 5's five modules `cyd-creator-serial`, `-flasher`, `-monitor`, `-devices`, `-transfer` load in this order
  below the "plan 5" marker comment, between `cyd-creator-home.js` and `cyd-creator-app.js`
  (`cydCreatorFlashModules`, also in `cydCreatorShellModules`). `app` attaches the monitor right after the dock
  (`inst.monitor = P.monitor.attach(inst)`, disposed with `inst.projectBag`), before the agent panel exists: the
  monitor looks `inst.agent` up when "Attach log" is used. Toolbar Flash, OTA and Export and the start screen's
  Import ZIP card show only when their module loaded.
- Web Serial (`P.serial`): `P.serial.manager` is the page's single owner of the port (a port opens once per
  page). Owners are `flasher` (exclusive) and `monitor` (revocable: a flash makes the monitor close the port
  first, and the monitor reopens it afterwards). It covers CYD Creator only: a port Quick Connect holds is
  `CYD_FLASH_PORT_BUSY`. `support()` returns `{ok, reason}` with reason `readonly` (read-only desktop), `disabled`
  (`virtual_desktop.serial_browser_enabled` off, admin-only, or not known yet), `insecure`, `unsupported` (no
  `navigator.serial`), `framed` (not the top-level `/desktop` document) or `policy` (Permissions-Policy). The
  desktop policy (decision D2, like Quick Connect) comes from `aurago:desktop-policy` events on `document`
  (`serial_browser_enabled`, `readonly`) and, while unknown, one `GET /api/desktop/bootstrap` (events win;
  `manager.ensure()` waits for it); a change re-renders an idle flash dialog and ends a monitor session it no
  longer allows. `manager.request()` (`requestPort`) must be the first await of a click (user activation). The
  reset pulses RTS for 100 ms with DTR (IO0/BOOT) released: BOOT held 5 s during the splash erases the Wi-Fi
  credentials.
- Flasher (`P.flasher.open(inst)`): esptool-js 0.7.0 is imported lazily from
  `/js/vendor/esptool-js/esptool-js.min.mjs` (`P.flasher.setLoader` injects a fake). It offers succeeded builds
  with merged.bin (default: the newest), downloads the images and checks each against the build record's SHA-256
  and `X-CYD-SHA256` before anything is written. The build decides the AuraGo link: its image contains the
  heartbeat path literal `/api/cyd-creator/device/heartbeat` (only link firmware has it). When it does and the user
  may mint (not read-only, OTA on), the flasher mints a device after the download (`P.api.mintDevice`) and writes
  the blob at its `config_offset`, which must equal the board's `caps.boards[].config_offset`. Install modes
  (decision D1): Automatic (default) keeps the settings only when this browser's board record (`localStorage`
  `aurago.cydcreator.boards.v1`, per project and MAC: chip, board profile, minted device; server device rows have
  no MAC) matches the connected board's MAC, chip and board profile and the partition table on the board (MD5 of
  0x8000, 3 KiB, read back) equals merged.bin's; else it is a full install. Keep settings writes firmware.bin at
  0x10000 and erases otadata (8 KiB 0xFF at 0xE000, so app0 boots even after an OTA into app1); Full install
  writes merged.bin at 0x0 (wipes touch calibration and Wi-Fi) and drops the board's records before writing.
  Every flash rewrites the cydcc config partition (the blob, else 4 KiB 0xFF at `config_offset`), so no earlier
  token survives. Rates 921600 → 460800 → 115200; a wrong chip, a lost board, a busy port or missing Web Serial
  end at once; esptool verifies every image by MD5 (`md5Hex`). With the monitor toggle on, esptool ends in the stub
  (`no_reset_stub`) and the monitor connects and resets the board through RTS (`handOff`; when it cannot, the
  flasher resets once). After a success `aurago.cydcreator.flashed.v1` remembers the build per project (backtrace
  decoding), the board record is written and the board's previous device of this project is revoked (its token is
  no longer on the board). A failed flash revokes the device it minted unless the blob was already transferred
  (`blobWritten`). Escape and the backdrop cannot close the dialog while a flash runs; a window closed after the
  first write lets the flash finish and hard-reset. `device_url` and `tls_pin` refusals show
  `flash.error_device_url` and `flash.error_tls_pin` (decision D4: set `cyd_creator.ota.public_url`). Without Web
  Serial the dialog says why (`flash.unsupported_<reason>`) and offers only "Download image" (merged.bin at 0x0).
- Monitor (`P.monitor`): dock tab `monitor` (order 1, `cyd_creator.dock.monitor`), states disconnected, connected,
  flashing and lost; 5000-line ring buffer, timestamps, filter, pause, clear, reset, baud rates 9600–921600
  (default 115200; baud and timestamps remembered in `aurago.cydcreator.monitor.v1`). `Backtrace:` lines get
  "Decode" (`POST builds/{id}/decode-backtrace` with the build last flashed from this browser, else the newest
  green one); "Attach log" hands at most 16 KiB of the selected or visible lines to `inst.agent.attachLog`.
- OTA dialog (`P.devices.open`): devices with state (pending, online, offline), board, build, last seen, target,
  IP, RSSI and the translated last error (`rolled_back[: …]` with `devices.rollback_hint`: re-picking the failed
  build is reset until the board restarts; `failed[: …]`); rename, "Update to" a succeeded build with firmware.bin
  for the device's board, remove (asks first, revokes). Live `device:changed`, a refresh every 30 s while open and
  `readonly:changed` re-render it; read-only users see no actions; banners show OTA off, the device URL
  (`caps.ota.public_url`) and a plain `http://` warning. The device wrappers live on `P.api` (`listDevices`,
  `mintDevice`, `updateDevice`, `revokeDevice`); both dialogs pass `owner: inst.projectBag || inst.bag`.
- Transfer (`P.transfer`): Export flushes (`inst.flushAll()`), fetches `GET projects/{id}/export` and saves it
  under the `Content-Disposition` name; Import ZIP picks a file (`P.transfer.pickFile`, replaceable in tests),
  posts it to `/import` (`P.api.importZip`) and the start screen opens the new project. `CYD_IMPORT_INVALID` shows
  `transfer.import_<reason>` (`transfer.limit_asset_bytes` / `limit_app_files` for the size limits); other errors
  go to `P.errors.message`, which maps `CYD_FILE_LIMIT` by `details.limit` to `transfer.limit_<asset_bytes|
  export_bytes|app_files>` (`file_bytes`, `files` and `total_bytes` share `app_files`; `{{size}}` = `details.max`)
  and `CYD_BUSY` `export` to `transfer.busy_export`.
- Strings: `cyd_creator.flash.*`, `monitor.*`, `devices.*`, `transfer.*` and `dock.monitor` in all 16
  `ui/lang/cyd_creator` files; `TestCYDCreatorFlashKeysExistInAllLocales` lists the members of every dynamic family
  (`flash.unsupported_`, `flash.phase_`, `monitor.status_`, `devices.state_`, `transfer.limit_`, `transfer.busy_`,
  `transfer.import_`); errors reuse 4a's `error.<CODE>`.
- CSS: the section between `/* ==== CYD Creator flash, monitor and devices (plan 5) — begin` and `… — end` (both
  with U+2014) follows the designer's end marker; `TestCYDCreatorFlashStylesheetSection` allows no literal colour
  or colour function, only `--cc-*` tokens the stylesheet defines and only selectors scoped to `.cydc-app`.
- Verify: `npm run test:cyd-creator-flash`, `go test ./ui -run '(?i)CYDCreator'` (plan 5 adds the module, offset,
  vendor, wiring, stylesheet, devices/transfer and key tests) and, with `AURAGO_RUN_BROWSER_SMOKE=1`,
  `TestDesktopCYDCreatorFlashBrowser` (fixture `ui/testdata/cyd-creator-flash-fixture.js`: fake `navigator.serial`
  and esptool). After an esptool-js change: `npm run build:browser-vendor -- --check`.

## RuView

- `ruview.js` owns one Desktop window. Overview and Devices poll status and known boards together while the window is visible, pause when it is hidden, and do not overlap requests. An idle tab waits for that snapshot before showing board ages. A tab switch during a refresh paints the last snapshot and does not start another request. The first summary value stays the service state. A failed status or node read keeps the previous board list. A successful status is still applied when the board read fails.
- Devices name CSI and RSSI and show a relative age. An offline RSSI is labelled as the last value for the same receiver. A measurement time the reply omitted stays blank. Each board can open the wizard with that node ID. Disabled actions share one visible reason. Rendering never starts a runtime or enables UDP.
- The upstream iframe sandbox is `allow-scripts` only. `relay.mjs` posts reviewed API calls and sensing sockets to this window and replies with target origin `*`; the frame accepts a reply only from this window and cannot open its own connections. Serial, camera, and microphone stay off the frame. Backend status and recording acknowledgement: `internal/ruview/AGENTS.md`. Verify `node scripts/ruview/browser.test.mjs`.

## Purpose

This subtree owns built-in virtual desktop app modules that are loaded lazily by
`ui/js/desktop/core/module-loader.js`.

- `ziro-designer.js` owns the `store_ui=ziro-designer` wrapper; `ziro-files.js`
  owns bounded Shared project reads and conditional binary uploads. Load them
  through the Store renderer, including its stylesheet. Only the parent owns
  API credentials, file versions, dialogs and Shared paths. Bind a MessageChannel
  to the exact Store iframe, origin and random document challenge, and revoke
  it on navigation/disposal. Keep version/draft state per window and preserve
  partial-save progress; a cancelled recovery must retain the original draft.
  Once bound to a project, New/Open launch a separate Desktop window to isolate
  pending editor callbacks; Save As retains the current editor's file set.
  Snapshot every mounted editor before explicit Save/Ctrl+S and before closing.
  Match dirty state to the snapshot generation so an older acknowledgement
  cannot hide later edits. The upstream image adapter owns strict serialization,
  binary/new-file preservation and local drafts. Read `documentation/ziro-designer.md`.
  Verify `node scripts/test-ziro.mjs`, `node scripts/test-ziro-bridge.mjs` and
  `node scripts/test-ziro-browser.mjs` against the built local image.

- `tresor.js` owns setup, recovery confirmation, note/file workflows, encrypted note autosave/draft recovery, the five-minute idle lock and window disposal. Desktop close awaits saves and reports failures. Explicit, idle and pagehide locks immediately clear decrypted state even during failed/pending writes; only completed ciphertext drafts survive. Late operations cannot alter a newer session. Commit note selection only after loading/decryption succeeds, and keep both draft fields in state across redraws. `tresor-crypto.js` derives Argon2id keys with locally copied `argon2id@1.0.1` WASM, wipes its entire workspace and encrypts metadata and bodies with Web Crypto AES-256-GCM. Never send passwords, recovery keys or plaintext to `/api/desktop/tresor` or persistent browser storage. The door artwork is decorative; forms stay keyboard-accessible and reduced motion disables the opening effect. Verify `TestDesktopTresorBrowser` (including `checkTresorFailureRecovery`) and `npm run test:tresor-crypto`.

- Game Maker's existing asset browser includes sprite packs and individual 3D
  models. `game-maker-studio-models.js` owns one disposable viewer per modal,
  sharing the pinned local 0.186.1 runtime helper. Cancel pending fetches and
  release rigs, controls, shadow maps and renderer on replacement/close/dispose.
  Render only on interaction or while an animation is playing and visible.
  Selection uses up to 64 `model_asset_ids`, separate from sprite pack selection.
  Keep all labels in the sixteen desktop locales and preserve both import flows.

Shell chrome helpers live in the main desktop bundle (not lazy apps):
`core/sound-runtime.js` (opt-in synthesized UI sounds; lazy
`bundles/desktop-sounds.bundle.js` with `sound/synth-core.js` and five theme
modules), `core/screensaver-runtime.js` (opt-in idle detection and wake
handling; the lazy overlay and scenes live in `js/desktop/screensavers/`,
see its child contract), `core/session-runtime.js` (session restore, dock pins, recent files, default
apps), `core/spaces-runtime.js` (three virtual desktops / Spaces v1: window
`spaceId`, hide-without-dispose, session snapshot v2, taskbar pager, Ctrl+Alt
arrows; disabled on compact viewport), `core/shell-chrome-runtime.js`
(notification center, clock popup, hold window switcher, shortcuts overlay),
`core/spotlight-runtime.js` (Ctrl+K mixed search), and
`core/window-shell-runtime.js` (widget frames, standalone widgets,
`openApp`). Widget-frame and standalone-widget empty-state load
failures use `desktop.load_failed`. Standalone Webamp notifications
map `desktop.winamp_unsupported`. Weather HTTP throws the sentinel
`HTTP` without a status; the catch interpolates
`desktop.weather_load_error` with `desktop.weather_network_error`
only. Widget persist toasts (location, auto-size, bounds) use
`desktop.widget_update_failed`. Styles:
`ui/css/desktop-chrome.css` (bundled into `desktop-shell.bundle.css`).
Persisted keys: `windows.restore_session`, `appearance.dock_pins`,
`session.windows` (snapshot v2 with `activeSpaceId` and per-window `spaceId`
plus optional `alwaysOnTop`),
and `files.default_apps` via `/api/desktop/settings`.
The shell owns its settings writer; the pet runtime's private writer is not
available to session, dock or default-app helpers. Enabling session restore
captures current windows immediately; `pagehide` flushes pending geometry with
a keepalive request. Startup restores before opening an `?app=` deep link,
preserving separate instances and the normal bounds of maximized windows.
Verify actual save/reload behavior with `TestDesktopSessionRestoreBrowser`.

### Desktop UI sound contract

- Opt-in via `sound.enabled` (default off). Persisted keys:
  `sound.theme`, `sound.volume`, `sound.windows`, `sound.notifications`,
  `sound.navigation`, `sound.files` (`internal/desktop/types.go`).
- Shell API: `desktopSound(eventId)` only. No playback when disabled, tab hidden,
  session restore, category off, or before the first user gesture unlocks Web
  Audio. Preview in Settings may render/play without changing the saved theme.
- Event vocabulary (19 ids, all five themes): `window.*`, `notify.*`, `menu.*`,
  `space.switch`, `dialog.*`, `file.*`. Hooks live in window shell/interactions,
  notifications, menus, spaces, dialogs and file drops — no business-logic changes.
- Offline render once per theme into cached `AudioBuffer`s; master gain uses
  perceptual `volume²`, category gains, compressor and voice rate limits.
- Settings app section `sound` exposes master toggle, theme cards with preview,
  volume range and category toggles. Keep `desktop.settings_sound_*` in all 16
  desktop locales. Saving `sound.volume` must not rebuild the Settings pane
  (that resets scroll). Other setting saves restore `.vd-settings-pane` scroll.
  `sound-symbolic` maps to the speaker mini icon; theme-SVG fallbacks use a
  currentColor mask so they are not black on dark chrome.
- Verify with `TestDesktopSoundRuntimeMarkers`, `TestDesktopSoundHookMarkers`,
  `TestDesktopSoundThemeEvents`, `TestDesktopSoundTranslations`,
  `TestDesktopSoundSettingsUIMarkers`, `TestDesktopSettingsSymbolicIconAssetsStayCompact`,
  service settings tests, and opt-in `TestDesktopSoundBrowser`
  (`AURAGO_RUN_BROWSER_SMOKE=1`).

Resize handles in `core/window-interactions-runtime.js` honor the shell's
per-window minimum width/height, including the fixed opposite edge on west/north drags.
Free titlebar/menubar space supports dragging and double-click maximizing;
buttons and menu popovers remain excluded from those gestures.

### Store app setup

- PrintGuard setup in `software-store.js` uses its admin config/test endpoints,
  CPU-first install, defined acceleration/network choices and explicit off/read/
  control grants. Never prefill tokens or test unsaved drafts. Clear credentials
  and ignore late requests on close/disposal; preserve focus trapping and status
  announcements. Show active compute, CPU fallback and pending runtime changes.
  Missing deployment prerequisites disable Install and block form submission.
  Keep their messages visible during edits, link to Server config and allow a
  fresh check. A valid network choice clears only the network prerequisite.
  Keep all `desktop.store.pg_*` texts in sixteen locales and the upstream icon /
  license under `img/desktop/store/`. Verify `TestPrintGuard*` and bundle `--check`.

- Store operation failures remain as escaped, accessible text on the app card
  until retry, including when install rollback removes the installed record.
  Notify before refreshing Desktop bootstrap; refresh failures must not hide
  the original operation error. Polling errors must remain visible as well.
- `software-store.js` adds `Einrichten` for installed `gods-eye-view` only.
  GET/PUT `/api/desktop/store/apps/gods-eye-view/config` returns flags/origins,
  never saved keys. Empty fields preserve, checkboxes explicitly delete, and
  pending changes remain visibly inactive after a failed container replacement.
  Keep the dialog keyboard accessible, focus trapped and disposed on app close. Every Store modal close clears credential values and attributes, removes its overlay, and ignores late asynchronous responses.
- Installation preselects the current AuraGo origin. The dialog accepts further
  exact HTTP(S) origins and explains optional LAN access/provider quota use,
  browser-visible Google/Cesium credentials, and secure-context microphone
  permission. Keep `desktop.store.gev_*` in all 16 Desktop locale files.
- `quickconnect-launchpad-chat.js` uses the existing sandboxed container-app
  frame, adds microphone only for this app, passes `aurago_lang` to the managed
  notice and keeps external-open disabled. No new proxy path or auto-start is
  introduced. Logo/license live under `img/desktop/store/gods-eye-view.*`.
  Papirus and WhiteSur register `gods-eye-view` with copies of that SVG carrying
  its MIT notice; keep them synchronized with the backend preferred-icon list.
- Verify `TestGodsEyeDesktopTranslations`, bundle `--check`, and opt-in
  `TestGodsEyeDesktopBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` and
  `AURAGO_GEV_BROWSER=1`: reviewed image on 127.0.0.1:14173, allowed frame origin
  `http://127.0.0.1:18099`. The test exercises the globe, free live data, controls,
  resizing, close/reopen and key-free setup retrieval. Real provider acceptance
  remains separate from image adapter tests with synthetic responses.

### Spaces v1 contract

- Exactly three spaces (`1`, `2`, `3`); no create/delete in v1.
- Normal windows get `spaceId` from the active space at open time; session
  restore reads stored `spaceId` (fallback space `1`).
- Space switch hides other windows (`vd-space-hidden` / `data-space-hidden`) but
  does not dispose them; minimize and space-hide stay independent.
- Desktop icons, widgets, and gadgets stay global across spaces. Wallpaper is
  per space (`appearance.wallpaper_by_space`) and falls back to
  `appearance.wallpaper`; compact viewport keeps one shared wallpaper.
- Taskbar/dock pins stay global; running window buttons and Ctrl+Tab switcher
  list only the active space.
- Fruity lists every user-facing app with `dock_visible !== false`, with pins
  first and no fixed item limit. Hidden apps appear temporarily only while
  running on the active space. Keep hover-label headroom inside `.vd-dock-scroll`
  so its vertical overflow clip cannot leave label fragments above the dock.
- `findExistingAppWindow` prefers the current space; a match in another space
  triggers `switchSpace` then focus (no duplicate window).
- Compact viewport (`isCompactViewport()`) keeps single-space behavior and hides
  the pager.

### Spaces overview contract

- UI label is **Flächenübersicht / Spaces overview** — never reuse the Mission
  Control app name.
- Exactly three columns (`1`, `2`, `3`); overview does not create spaces.
- Window cards reuse `windowPreviewMarkup()` from the taskbar thumbnail helper;
  minimized windows stay visible as dimmed cards.
- Click column background switches space; click card switches, focuses, and
  closes; drag card to another column calls `moveWindowToSpace()` and re-renders.
- Shortcuts: `Ctrl+Alt+ArrowUp` and `F3` toggle; `Ctrl+Alt+ArrowDown` closes;
  `Ctrl+Alt+ArrowLeft/Right` keep cycling spaces.
- Pager: short click switches; ~400ms hold or second click on the active space
  opens overview.
- Show Desktop uses `toggleShowDesktop()` peek/restore for visible windows on the
  active space only; peek set clears on focus or `openApp()`.
- Compact viewport disables overview and keeps legacy single-window Show Desktop.
- Snap left/right entries in the window context menu call `applyWindowSnap()`.

### Window always-on-top contract

- Normal windows can pin above other windows via the window context menu.
  Pet and SIP gadgets keep their own always-on-top settings; they are not
  windows and stay siblings above `#vd-window-layer`.
- On-top windows use Z-band `200000` inside the window layer. Focusing a
  normal window must not jump that band. `normalizeWindowZIndexes` keeps both
  bands. Gadgets stay out of this band.
- Always-on-top stays space-scoped. Hidden on other spaces. Not global.
- Session snapshot v2 stores `alwaysOnTop` additively. No version bump to 3.

### Widget config contract

- Sticky notes are individual `type: sticky-note` records. Their manager and
  context-menu deletion uses `desktop.sticky_delete` / `sticky_delete_msg` to
  explain that only this note and its content are removed; creation remains
  available. Reserve `widget_delete_permanent` for custom widget registrations.

- `builtin-printer` is hidden by default and added through the widget drawer.
  It uses the shared 320px widget width, remembers the selected configured ID in browser storage,
  polls read-only status every 30 seconds while visible, and shows progress,
  available estimated remaining time, layers, nozzle/bed temperatures and filename.
  Elegoo `TotalTicks - CurrentTicks` is seconds; absent timing is never estimated
  from progress. Text uses `desktop.widget_printer_*` in all 16 desktop locales.

- `builtin-fritzbox` (`core/widget-fritzbox-runtime.js` plus the pure SVG/format
  helpers in `core/widget-fritzbox-charts.js`) is hidden by default, uses the
  shared 320px width at 300px height and reads only
  `GET /api/desktop/fritzbox/overview?sections=...`. Pages (connection, traffic,
  devices, telephony) are a pager with `role="tablist"` dots, arrows,
  Arrow/Home/End keys, horizontal wheel/trackpad gestures and touch/pen swipe;
  the last page persists in `localStorage` key `aurago.desktop.fritzbox.page`.
  The widget card captures every pointer on `pointerdown` so it can be moved,
  therefore a mouse never starts a swipe and swipe move/up/cancel handling
  listens on `window` in the capture phase (removed in the cleanup); a swipe
  aborts when the card enters `vd-dragging` or a long press triggers. Never
  attach the swipe's `pointerup` to the viewport alone. Sections whose capability
  is off are dropped; `fritzbox_disabled` shows `widget_fritzbox_disabled_hint`
  and stops polling. Polling: connection every 5 s while visible (keeps the
  traffic history continuous on every page), devices/telephony every 60 s plus
  a targeted refresh when their page opens with data older than 20 s, system
  every 5 min; `visibilitychange` pauses, `AbortController`
  cancels in-flight requests, and `registerWidgetCleanup` releases timers,
  observer and listeners. Traffic history is a client-only ring buffer
  (`fritzMergeMonitorSamples`, max 180 samples) seeded from the router online
  monitor; no backend sampler exists. All router text (host names, caller
  names, numbers, IPs, SSIDs) renders via `textContent`; `innerHTML` receives
  only the static shell, numeric SVG markup and trusted glyphs. Compact mode
  (`is-compact`) engages below 280px via `ResizeObserver`. Strings use
  `desktop.widget_fritzbox_*` plus reused `desktop.copy`, `desktop.copied`,
  `desktop.retry`, `desktop.load_failed`, `desktop.system_info_updated` and
  the sysmon byte/uptime formatters in all 16 desktop locales.
- Fritz!Box overview errors belong to individual sections. Preserve a failed
  section across unrelated partial polls and clear it only after its own
  successful read or capability removal. Name the affected sections in the
  banner; a telephony/device failure must not mark fresh connection data stale
  or claim the router is unreachable. Request failures use a neutral data-load
  message. Verify the partial-error and recovery cases in
  `TestDesktopFritzBoxWidgetBrowser`.

- All widget cards, including sticky notes and generated iframe widgets, use
  `widgetWidth()` (320px, reduced only for a narrower workspace). Content resize
  changes height only. Stored legacy widths must not override the shared width.
  `snapWidgetPosition()` applies an 8px grid during drag and restore, with edges
  rounded inward after workspace clamping. Default stacks stay on that grid;
  drag persistence stores the displayed position and dimensions.

- Weather location lives in `widget.Config.location` (`{ lat, lon, name, country }`).
  `localStorage` key `vd-weather-location` is a one-time import only. After a
  successful POST upsert, stop using it as the source of truth.
- Auto-size persists as `widget.Config.auto_size`. Default is on when the flag
  is missing. Top-level `auto_size` is dropped by the Go `Widget` struct — do
  not add a Go field unless Config cannot round-trip.
- POST `/api/desktop/widgets` replaces the entire `config_json`. Always send a
  merged full config. PATCH stays visibility-only.
- The widget context menu owns the auto-size toggle (`desktop.widget_auto_size`).
  Readonly denies the mutation. Weather location saves use `skipReload` so the
  card does not remount.
- Weather chrome and WMO labels use `desktop.weather_*` keys in all 16 desktop
  locales. Do not hardcode English weather UI or WMO labels.
- Builtin widget catalog titles are UI-only via `widgetDisplayTitle`. Stored
  `widget.Title` stays the seed. Do not send the translated title in POST/PATCH.
- Widget chrome errors and the sysmon host label use `desktop.quickchat_error`,
  `desktop.widget_update_failed`, and `desktop.system_info_host`. Weather
  HTTP throws the sentinel `HTTP` without a status. The weather catch
  interpolates `desktop.weather_load_error` with
  `desktop.weather_network_error` only. Persist toasts for location,
  auto-size, and bounds use `desktop.widget_update_failed`. Do not dump
  `err.message` or an HTTP status there. Do not hardcode English there.
- Sysmon uptime units and weather wind speed use
  `desktop.system_info_uptime_days_hours`,
  `desktop.system_info_uptime_hours_minutes`,
  `desktop.system_info_uptime_minutes`, and `desktop.weather_wind_kmh`. Do not
  hardcode `d`/`h`/`m` or `km/h`.
- Sysmon memory, disk, and network sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not
  hardcode `B`/`KiB`/`MiB`/`GiB`/`TiB` there. Leave the `/s` rate suffix.
- The opt-in `builtin-meshcore` widget (`core/widget-meshcore-runtime.js`)
  reads only `GET /api/meshcore/messenger/bootstrap` (conversations, status,
  enabled) and refreshes on the metadata-only `aurago:meshcore-change`
  document event plus a 30 s visibility-gated poll; all listeners and timers
  are released via `registerWidgetCleanup`. Rows open the `meshcore` app with
  a validated 64-hex `conversation_id`. A protected conversation always shows
  the lock placeholder, including when a preview string is present, and is
  never revealed; radio text renders via `textContent` only. Strings use `desktop.widget_meshcore_*` plus reused
  `desktop.meshcore_*` keys in all 16 desktop locales.
- The System Info app reuses `hours_minutes` and `minutes`, and uses
  `desktop.system_info_uptime_days_hours_minutes` when days are present.
  Sysmon stays without minutes in the days/hours form.
- System Info network totals use `desktop.system_info_network_io` with
  `{{sent}}` and `{{recv}}`. Do not hardcode English `up / down`.
- System Info history-chart dataset labels reuse `desktop.system_info_cpu`,
  `desktop.system_info_memory`, and `desktop.system_info_disk`. Gauge
  dataset labels use `desktop.system_info_used` and
  `desktop.system_info_free`. Do not hardcode English CPU/Memory/Disk or
  Used/Free there. Leave Chart legend and tooltip off.
- System Info memory, disk, and network sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not
  hardcode `B`/`KiB`/`MiB`/`GiB`/`TiB` there. Leave File Manager and
  OpenSCAD byte formatters unchanged.
- Log Viewer file-list sizes use `desktop.bytes`, `desktop.kib`,
  `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not hardcode
  `B`/`KiB`/`MiB` there.
- Sheets search match counts use `desktop.sheets_match_count` with
  `{{current}}` and `{{total}}`. Do not hardcode English `of` there.
- Writer search match counts use `desktop.writer_match_count` with
  `{{current}}` and `{{total}}`. The search-close tooltip uses
  `desktop.close`. Do not hardcode `1/5` or `Esc` there.
- Pet fallback `aria-label` uses `desktop.pet_aria_label`. After a pet
  loads, keep `display_name` / `id`. Pet Picker scale text uses
  `desktop.pet_scale_value` with `{{value}}`. Do not hardcode
  `Desktop pet` or `1.0x` there.
- Radio station click counts use `desktop.radio_compact_thousands` and
  `desktop.radio_compact_millions` with `{{count}}`. MediaSession title
  fallback uses `desktop.app_radio`; album uses `desktop.radio_album`.
  Pass `t` into `updateMediaSession`. Radio `t` stays key-only;
  interpolate via `.replace`. Do not hardcode `K`/`M`, `Radio`, or
  `AuraGo Radio` there.
- Mission Control formatting rules live in the "Mission Control contract"
  section. Keep SIP/Noisemaker duration formatting unchanged.
  System World shows localized timestamps; compact uptime reuses
  `desktop.system_info_uptime_days_hours`, `desktop.system_info_uptime_hours_minutes`
  and `desktop.system_info_uptime_minutes`. Budget uses `desktop.looper_cost`;
  success rate and identity rows use `sysworld.panel.success_rate` and
  `sysworld.panel.id`. Interpolation goes through `inst.ctx.t`.
- People birthday countdowns in the sidebar, cards, list, and detail
  use `desktop.people_today`, `desktop.people_tomorrow`, and
  `desktop.people_days_until_birthday`. Do not hardcode `d` or `days`
  there.
- People KG toggle, active label, and card badge use
  `desktop.people_kg`. Do not hardcode `KG` there.
- Virtual Computers duration and expiry-day labels use
  `desktop.virtual_computers_duration_*` and
  `desktop.virtual_computers_expiry_days`. Volume TTL options use
  `formatDuration` for 1/7/30 days. Do not hardcode `s`/`min`/`h`/`d`
  there. Leave `tx` as a key-only helper.
- Virtual Computers volume sizes use `desktop.bytes`, `desktop.kib`,
  `desktop.mib`, and `desktop.gib`. Do not hardcode `KB`/`MB`/`GB` there.
- The New computer dialog shows the per-machine Network choice for both Python
  and Desktop templates. When the server reports Internet capability, default
  to Internet and allow Offline; otherwise explain why only Offline is available.
  Send the explicit selection on launch and keep labels in all 16 Desktop locales.
- Looper log durations use `desktop.looper_duration_ms` and
  `desktop.looper_duration_s`. Do not hardcode `ms`/`s` there. Leave SIP
  and Noisemaker `formatDuration` unchanged.
- OpenSCAD, Homepage Studio, and Noisemaker elapsed busy times use
  `desktop.noisemaker_progress_elapsed` with `{{seconds}}`. Do not
  hardcode `s` there. OpenSCAD `t` stays key-only; interpolate via
  `ctx.t`.
- Looper status cost and token labels use `desktop.looper_cost`,
  `desktop.looper_cost_under`, and `desktop.looper_tokens`. Sysworld
  HUD budget reuses `desktop.looper_cost` only. Do not hardcode `$` /
  `<$0.01` / `tok` there.
- Zipper status uses `zipper.selected` and `zipper.compressed_size`.
  Zipper sizes use `desktop.bytes`, `desktop.kib`, `desktop.mib`,
  `desktop.gib`, and `desktop.tib`. The open-dialog filter uses
  `desktop.file_dialog_zip`. Do not hardcode `selected`,
  `compressed`, `ZIP Archives`, or `B`/`KiB`/`MiB` there. Zipper
  `t` stays key-only.   Pixel open-dialog filter uses
  `desktop.file_dialog_images`. Pixel save-dialog filters use
  `desktop.file_dialog_png` and `desktop.file_dialog_jpeg` (the server validates
  the actual PNG/JPEG data). WebP remains an export format. Do not hardcode `Images`,
  `PNG Image`, `JPEG Image`, or `WebP Image` there. The OpenRaster filter of the
  open and save dialogs uses `desktop.file_dialog_ora`; the Pixel save dialog lists
  it first only when the document has several layers or masks.
  Pixel `t` stays key-only. Leave File Manager and
  OpenSCAD byte formatters unchanged.
- Quick Connect SFTP status uses `desktop.qc_sftp_items`. SFTP sizes
  use `desktop.bytes`, `desktop.kib`, `desktop.mib`, `desktop.gib`,
  and `desktop.tib`. Do not hardcode `items` or `B`/`KiB`/`MiB`/`GiB`
  there.
- Quick Connect SFTP navigation uses one `createSFTPNavigator` per open panel
  (`quickconnect-sftp-navigator.js`, bundled before `menus-and-routing.js`). A newer listing aborts the
  older request, the shown path changes only after a successful listing, rows carry their absolute
  `data-path` for every action, and closing the panel or window disposes the navigator. Verify with
  `npm run test:ui-regressions`.
- `quickconnect-serial.js` owns the serial session controller: profiles, bounded
  in-memory RX/TX capture, ANSI/hex display and the browser/host connection
  lifecycle. `quickconnect-serial-model.js` (profile schema, hex codec, error
  keys), `-views.js` (markup) and `-transport.js` (control signals, teardown)
  precede it in `desktopMainParts`, all before the Quick Connect shell. Serial
  tests load the parts in that order through `readQuickConnectSerialSources`.
  Profiles use the versioned `quick_connect.serial_profiles` setting;
  payload bytes never reach logs, persistence or an LLM. See
  `documentation/quick-connect-serial.md` and `internal/desktop/AGENTS.md`.
- Quick Connect has one active connection per window. Serial, SSH and VNC switches
  invalidate old callbacks before cleanup; the Files tab remains SSH-only. Cancel
  pending device choices and close late opens after policy/auth/window disposal.
  Browser serial requires a trusted user gesture and always uses the browser's
  picker. No automatic reconnect. Embedded apps deny `serial` in their frame policy.
- Quick Connect lists inventory entries by explicit `ssh`/`vnc` protocol; device
  types never imply remote access. An explicit `device_id` launch may retain a
  `none` target with connection disabled. Keep the synthetic AuraGo host and serial
  profiles. Verify `TestDesktopQuickConnectOnlyListsRemoteDevicesBrowser` in both themes.
- Keep serial writes ordered, capture and render queues bounded, initial DTR/RTS
  off, and clear Break after 250 ms including cleanup races. Device traffic never
  counts as user activity. Verify `TestQuickConnectSerialTranslations`, serial
  browser tests, existing Quick Connect/SFTP checks and both themes at narrow and
  wide window sizes. USB adapter acceptance is a separate hardware check.
- Quick Connect synthetic AuraGo host uses `desktop.qc_aurago_host`
  and `desktop.qc_aurago_host_description`. Detect the host by
  `id === '__aurago-host__'`, matching IP, or the English sentinel
  `aurago host`. Do not hardcode `AuraGo Host` or
  `Current AuraGo web host` there. Shared `t(k, p)` interpolates
  `{{name}}`; call these keys without a second argument.
- Calculator backspace labels use `desktop.calc_back`. Do not hardcode
  English `Back` there.
- Calculator display maps the parser sentinel `Invalid expression` to
  `desktop.calc_invalid_expression`. Do not change the throw messages.
- Code Studio agent markdown copy buttons use `desktop.copy` and
  `desktop.copied`. Do not hardcode English Copy/Copied there.
- Code Studio status cursor text uses `codeStudio.cursorPosition` with
  `{{line}}` and `{{column}}`. Sidebar file sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do
  not hardcode `Ln`/`Col` or `B`/`KiB`/`MiB` there.
- Code Studio terminal tabs and the xterm welcome line use
  `codeStudio.shell_n` with `{{n}}` plus `codeStudio.title`. The zen
  exit tooltip reuses `codeStudio.exitZen`. A line-mode fallback labels
  the terminal status with `codeStudio.terminalLineMode`. Do not hardcode
  English `Shell N`, `Code Studio - Shell`, `Line mode`, or
  `Exit Zen Mode (Esc)` there.
- Shell new-file and new-folder prompt defaults use
  `desktop.new_file_default` and `desktop.new_folder`. Do not hardcode
  `untitled.txt` or `New Folder` in those prompts. Leave editor path
  fallbacks as `untitled.txt`.
- File Manager new-file template labels use `desktop.fm.new_file_kind_*`
  and `desktop.fm.new_file_template_label`. ZIP/rename success toasts use
  `desktop.fm.zip_created`, `desktop.fm.zip_extracted`, and
  `desktop.fm.batch_rename_success`. Preview and Quick Look chrome use
  `desktop.fm.preview_loading`, `desktop.fm.preview_unavailable`,
  `desktop.fm.quick_look_close`, and `desktop.fm.quick_look_error`.   The new-folder
  prompt default uses `desktop.fm.new_folder`. The new-file prompt and
  template filename default use `desktop.new_file_default`. Do not
  hardcode `new-file.txt` there.
- `renderAppError` shows `desktop.app_error_title` plus `err.message` or
  `desktop.app_error_fallback`. Do not hardcode English `Error` there.
- Calendar, Todo, and Gallery empty-state load failures use
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Quick Connect device-list, generated-app host, and People content
  empty-state load failures reuse `desktop.load_failed`. Do not dump
  raw `err.message` there. People `t` takes `(context, key)`.
- Editor fallback file-list empty-state load failures reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Webamp unsupported-browser errors use
  `desktop.winamp_unsupported`. `notifyError` maps the English
  sentinel `Webamp is not supported in this browser.` and the
  localized throw; other load failures reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
  Leave the Webamp skin unchanged.
- Widget-frame and standalone-widget empty-state load failures reuse
  `desktop.load_failed`. Standalone Webamp notifications map the
  same unsupported sentinel and reuse `desktop.load_failed` for
  other load failures. Do not dump raw `err.message` there.
- Missing Agent Chat / Live Speech renderers use
  `desktop.app_error_renderer_missing` with `{{app}}`. Do not hardcode
  English "renderer is not loaded" strings.
- Agent Chat and Live Speech missing-host throws reuse
  `desktop.load_failed`. `renderAppError` may show that localized
  message. Do not hardcode English `Desktop chat window content is
  not available` or `Live Speech window content is not available`.
  Agent Chat uses `desktopText(key)` with no second argument. Live
  Speech `text` stays `(key, fallback)`; call this key without a
  fallback.
- Agent Chat generic request errors use `desktop.chat_request_failed`.
  Homepage Studio chat-stream failures reuse the same key, including
  the missing stream-parser throw. Chat live-stream fallbacks use
  `desktop.chat_live_stream`. Unknown document-format badges use
  `desktop.chat_document_format_unknown`. Do not hardcode English
  `Request failed`, `Chat stream parser not loaded`, `Live stream`,
  or `FILE` there.
- Viewer missing-markdown-it errors show `viewer.error` only. Do
  not hardcode English `markdown-it not loaded` there.
- Pixel image-decode failures throw `pixel.error_load`. Pixel `t`
  stays key-only. Do not hardcode English `Failed to load image`.
- Teevee catalog timeouts and HTTP failures show
  `desktop.teevee_catalog_error`. `fetchJSON` throws the sentinel
  `iptv-org HTTP` without a status and must not call `t()` (it is
  module-level). `loadCatalog` must not dump `err.message`. Call
  Teevee `t(key)` with no second argument. Do not hardcode English
  `Catalog request timed out` or `iptv-org HTTP ` plus a status.
  Leave playback `formatPlaybackError`.
- Viewer 3D missing-STLLoader errors throw and map to
  `viewer.error`. Map the English sentinel
  `Three.js STLLoader is unavailable`. Other init failures may
  still use `viewer.error` plus `err.message`.
- Game Maker missing-modals throws use
  `game_maker.modules_load_failed` via `state.context.t(key)` with
  no placeholder map. Do not change `fail()` itself. Do not
  hardcode English `Game Maker Studio modules failed to load`.
- Radio catalog HTTP throws the sentinel `Radio Browser HTTP`
  without a status. `loadActive` and `searchStations` show
  `desktop.radio_catalog_error`. Playback `play()` shows
  `desktop.radio_error` only, including the player toggle.
  Radio `t` stays key-only. Do not dump `err.message` in those
  catalog catches or the play catches.
- People save/delete notifies and Notes `notifyError` use
  `desktop.request_failed`. Keep Notes rename conflicts on
  `desktop.notes_rename_exists` via `notesCode`. People uses
  `t(context, key)`. Notes uses `state.t(key)` with no fallback
  string. Do not dump `err.message` there except the rename
  conflict.
- Pet Picker load, activate, settings, and import notifies use
  `desktop.request_failed`. Call Pet `t(key)` with no fallback
  string as the second argument. Leave `desktop.pet_import_invalid`
  for a bad ZIP name. Leave `pet-runtime.js` setting toasts.
- Viewer and Sheets missing print-frame errors use
  `desktop.print_failed`. Viewer still prefixes `viewer.error`
  plus the throw message. Sheets notifies and returns.
  Call `t(key)` with no fallback string. Do not hardcode English
  `print frame unavailable`.
- Notes, Writer, Sheets and Viewer share `AuraDesktopPrint` from
  `core/print-runtime.js`. Set iframe sandbox before insertion to
  `allow-same-origin allow-modals`, never allow scripts; only the parent prints.
  Preserve sanitization, await fonts/images, bind the frame to its owner signal,
  and translate timeout errors at the caller. Browser regression:
  `TestDesktopPrintSandboxBrowser`.
- Store container-app frame errors, terminal-preview frame errors,
  store start toasts, and external-open notifications reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Generated-app iframe title fallback uses
  `desktop.embed_frame_title`. Host SDK error fallback uses
  `desktop.embed_bridge_failed`. Do not hardcode English
  `Aura Desktop app` or `Desktop bridge request failed` there.
  Leave `aura-desktop-sdk.js` last-resort English when the parent
  sends no error text.
- Store terminal-preview module load and in-preview stylesheet/script
  loads use `desktop.store_terminal_load_failed`. Wrap AuraLazyAssets
  and the fallback `onerror` so the asset URL does not leak. Call
  `t(key)` with no second argument. Leave
  `desktop.store_terminal_module_unavailable` for a loaded module
  without `render`.
- Host clipboard throws use `desktop.clipboard_read_unavailable` and
  `desktop.clipboard_write_unavailable`. Do not hardcode English
  `Clipboard read/write is not available` there. Leave other SDK
  throws and `aura-desktop-sdk.js` last-resort English unchanged.
- Chess result-modal fallbacks use `desktop.chess_new_game` and
  `desktop.ok`. Pass `t` into `createChessFx`. Callers may still
  pass localized labels. Do not hardcode English `New game` or
  `OK` there.
- Chess opponent-move toasts and status use
  `desktop.chess_agent_unavailable`, `desktop.chess_agent_no_move`,
  `desktop.chess_engine_unavailable`, `desktop.chess_engine_no_move`,
  `desktop.chess_engine_worker_failed`, `desktop.chess_engine_timeout`,
  `desktop.chess_engine_illegal`, `desktop.chess_opponent_illegal`,
  or `desktop.chess_move_failed`. Map `chessCode` or known English
  sentinels. Do not show raw Stockfish, worker, HTTP, or
  `err.message` text there.
- Chess vendor-load status uses `desktop.chess_load_failed`.
  Do not show raw vendor `err.message` there. Leave the hint
  path and opponent-error mapper unchanged. The missing-template
  HTML fallback reuses the same key via `ctx.t` and `ctx.esc`.
  Do not hardcode English `Chess UI failed to load.` there.
- Homepage Studio local webhost name fallback uses
  `homepage_studio.default_name`. Do not hardcode English `Homepage`
  there.

### Trash restore contract

- Restore is File Manager only. The desktop trash icon keeps Open, Empty, and
  Properties — never a Restore action (no single target).
- Restore only moves paths under `trash/…` through the batch `/api/desktop/trash`
  operation. Ordinary legacy entries restore to `Desktop/<name>`. Notes under
  `Trash/Notes/<uuid>/<subpath>` restore to `Documents/Notes/<subpath>`.
  No origin is guessed for legacy ordinary entries. Conflicts require the shared
  Replace / Keep copy / Cancel dialog; replacement rechecks the observed version.
- Trash drops from desktop icons and File Manager preflight the entire selection
  server-side before moving the first entry; Notes roots and ancestors are protected.
- Readonly denies restore and empty-trash mutations. Delete inside Trash stays
  a permanent DELETE.
- Shell callbacks `restoreFromTrash` and `emptyTrash` are injected from
  `menus-and-routing.js`. Empty Trash in the File Manager empty-folder menu
  calls that callback; do not reimplement emptying in the File Manager.

### Gallery contract

- The Gallery is a lazy app. `module-loader.js` `DESKTOP_APP_ASSETS.gallery`
  loads `/css/desktop-app-gallery.css` and, in this order,
  `gallery-library.js` (`window.GalleryLibrary`: item model, prefs
  `aurago.desktop.gallery.v1`, `TILE_SIZES`, `countLabel`), `gallery-view.js`
  (`window.GalleryView`: shell/card/section/info HTML), `gallery-menus.js`
  (`window.GalleryMenus`), `gallery-lightbox.js` (`window.GalleryLightbox`)
  and `gallery.js` (`window.GalleryApp.render(host, windowId, ctx)` /
  `dispose(windowId)`). Keep every module below the desktop JS line budget.
- `planning-gallery-music.js` only delegates: `renderGallery` builds the ctx via
  `galleryAppContext(context)` (`t`, `esc`, `api`, `iconMarkup`, `fmtBytes`,
  `notify`, `mediaPreviewURL`, `mediaDownloadURL`, `mediaPreviewKind`,
  `readonly`, `pageSize`, `animationsEnabled`, window-menu and context-menu
  callbacks, `confirmDialog`, `promptDialog`, `settingBool`, `desktopSound`,
  `openApp`, `downloadMediaPath`, `afterFileChange`). `openMediaLightbox` is
  the shared image/video/audio viewer; `openMediaPreview` in
  `editor-filemenu.js` routes media there and keeps `openLegacyMediaPreview`
  for documents/iframes.
- Every window-menu and context-menu item must carry a stable `id` (`open`,
  `download`, `sort-<sort>`, `tab-photos`, ...). `normalizeWindowMenuItems`
  derives action keys from `id`, so items without one collide. Context menus
  do not render `checked`; `withCheckIcons` mirrors the checked state into the
  `check`/`square` icon.
- Interaction: plain click opens the lightbox, Ctrl/Cmd-click, Shift-click,
  the tile checkbox or selection mode select; arrow keys/Home/End navigate,
  Space toggles, Enter opens, Delete deletes, Ctrl/Cmd-A selects, Escape clears.
  Lightbox: arrow keys/PageUp/PageDown/Home/End navigate, `+`/`-`/`0`/`1`
  zoom, `I` info, `D` download, `F2` rename, Delete deletes, Space toggles the
  slideshow, Escape closes; chrome auto-hides via `is-idle`, and backdrop
  clicks within 450 ms of opening are ignored (tile double-click).
- Readonly omits rename/delete controls and shows the readonly hint instead of
  rendering disabled buttons. There is no upload because media mounts are
  server-side read-only.
- `gallery-metadata.js` loads after `gallery-library.js` and before the view.
  Descriptions/tags use the media registry via `/api/desktop/gallery/metadata`;
  category keyword rules use the validated `gallery.categories` Desktop setting.
  Keep optimistic version checks, rooted image reads, admin-only explicit vision,
  vision budget accounting and Desktop revocation gates. Analyze all snapshots
  the complete Photos library (not the filtered grid), warns above 20 photos,
  runs sequentially and cancels on Stop/disposal/revocation. Preserve prompts,
  source tags and unsaved editor drafts. Verify `TestGalleryMetadataBrowser`,
  `TestDesktopGallery*`, `TestGallery*` and all 16 locales.
- Width/height/duration are derived client-side from loaded `<img>`/`<video>`
  elements; `FileEntry` carries none. Live refresh listens to `desktop_changed`
  SSE payload paths, polls every 45 s and reloads on `visibilitychange`.
- Count strings use the `_one` singular convention through
  `GalleryLibrary.countLabel(t, key, count, vars)`:
  `desktop.gallery_item_count(_one)`, `desktop.gallery_status_summary(_one)`
  with `{{size}}`, `desktop.gallery_status_results(_one)` and
  `desktop.gallery_delete_failed(_one)`. Keep all `desktop.gallery_*` keys in
  every `ui/lang/desktop/*.json`.
- `.vd-gallery [hidden]` / `.vd-lightbox [hidden]` force `display: none`
  because author display rules otherwise beat the UA hidden default. Broken
  tiles (`.is-broken`) hide media, play glyph and badge.
- Verify with `go test ./ui -run 'Gallery|WindowMenu|ContextMenu'` and
  `go test ./internal/desktop -run RecursiveCacheInvalidated` (recursive list
  cache invalidation after desktop mutations). See
  `documentation/desktop-gallery.md` for the user-facing description.

### Video Studio contract

- Video Studio is a lazy app. Load `desktop-app-video-studio.css` and `desktop-stickers.css`, then
  `shared/sticker-catalog.js`, `shared/sticker-library.js`,
  `video-studio-icons.js`, `video-studio-time.js`, `video-studio-effects.js` (DOM-free registry mirror and shared
  math), `video-studio-picture.js` (DOM-free colour and picture math), `video-studio-anim.js` (DOM-free in/out animation math), `video-studio-scope-math.js` (DOM-free scope math), `video-studio-media.js`, `video-studio-gl.js`
  (WebGL2 helper), `video-studio-shaders.js`,
  `video-studio-compositor.js` (GL painter), `video-studio-painter2d.js` (2D painter), `video-studio-preview.js`,
  `video-studio-timeline.js`,
  `video-studio-ruler.js` (in/out shade, marker flags, overview strip),
  `video-studio-edit.js` (DOM-free edit operations),
  `video-studio-keyframe-edit.js` (DOM-free keyframe and ramp operations),
  `video-studio-navigation.js` (DOM-free shuttle, edit points and track order),
  `video-studio-source-edit.js` (DOM-free source rules),
  `video-studio-folders.js` (DOM-free media folder model),
  `video-studio-inspector.js`, `video-studio-colorwheel.js` (SVG colour wheel: model, markup, pointer and keys), then the app parts
  `video-studio-core.js` (constants, requests, model, history, saving,
  conflicts, drafts), `-library.js`, `-effects-library.js` (effects library tab, LUT drops and removal), `-artwork.js` (titles, stickers),
  `-jobs.js` (jobs, refresh, export/AI dialogs), `-stage.js` (popovers,
  drawer, timeline height, transform box, field edits), `-mask.js` (mask overlay and handles), `-editing.js`
  (selection, clipboard, tools, edit commands and shortcuts), `-keyframes.js`
  (keyframe controls, strip, speed lane and shortcuts), `-inspector-effects.js`
  (Effects section, crop and flip edits, LUT picker, stack keys), `-source.js`
  (source monitor), `-tracks.js` (track commands, targets, heights, solo),
  `-markers.js` (marker commands and popover), `-transport.js` (JKL, in/out,
  loop and navigation keys), `-rangerender.js` (render range exactly and its compare overlay), `-scopes.js` (scopes panel: transport toggle, dock, readback and drawing), `-derive.js`
  (reverse, freeze frame and export frame as server jobs; the edit is applied
  by the `onSucceeded` hook of `createJob`/`pollJob`, always directly before
  `-projects.js`), `-projects.js` (settings and new-project dialogs),
  `-mediabin.js` (media bin actions: folder dialogs, item and folder menus,
  removal, drops onto folders, library metadata) and
  `-menus.js` (context menus), and `video-studio.js` last (42 Video Studio scripts after the two shared
  sticker scripts, so 44 in all; the same order in `module-loader.js`, both browser fixtures and
  `TestVideoStudioNavigationLoadOrder`; lists only get insertions). The parts share
  `window.VideoStudioParts`: each registers its functions with
  `Object.assign(P, …)` and calls the others through call-time wrappers, so
  every file stays below the 1100-line budget; the shell exposes
  `VideoStudioApp.render(host, id, ctx)` and `dispose(id)`. The preview owns its media elements and shared frame clock
  and creates only `<video>`/`<audio>`/`Image` (the node test asserts it);
  dispose stops local playback, timers, uploads (XHR), thumbnail extraction and
  requests but does not cancel server-owned jobs.
- Editing (phase 1a): `VideoStudioEdit` (`video-studio-edit.js`, DOM-free) changes a project in place and
  returns `true` or a reason (`locked`, `media`, `bounds`, `nothing`, `limit`, `kind`, `transition`); the app
  runs operations through `applyEdit` (one `mutate` undo step; a reason shows an info toast). Overwrite is the
  default drop and move, Ctrl inserts; locked tracks never change; ripple delete closes only time that is empty
  on every affected track. The multi-selection is `selectionSet` + `selectionPrimary` and counts only while the
  primary equals `selectedClipId` (code that sets `selectedClipId` directly collapses it); `mutate` restores
  both. Tools V/T/B: the trim tool rolls at a cut (Shift: ripple), ripples a free edge and slips the body
  (Alt: slides); the blade cuts one track (Shift: all unlocked). Context menus use `openMenu` in
  `video-studio-menus.js` (`role="menu"`, arrows, Home/End, Escape returns focus). Node checks:
  `go test ./ui -run TestVideoStudioEdit`.
- Clips and derived media (phase 1b): link groups are the clips with one `link` id (`linkGroup`, `withLinks`,
  `linkedEdges`; edges on the same frame belong together, Alt unlinks). Speed changes go through `setSpeed` only;
  stills keep speed 1. Every split (blade, overwrite, insert) follows the blade rule: right parts split off one group
  at one frame share a new link id, a lone right part is unlinked, so no split grows a group; `finish` (reason `link`)
  and `validTimeline` refuse a group above `MAX_LINK_CLIPS` (16, Go's `MaxLinkClips`). A linked blade or split cuts a
  group whole or not at all (`transition`). Paste remaps link ids and clears `audio_detached` on a video copy pasted
  without its audio clip; `canReattach`/`reattachAudio` also give an orphaned detached clip (no audio clip of its
  asset in its group or overlapping it) its sound back. Disabled clips are never drawn, played or mixed, and a
  transition applies only when the clip after it is enabled. Derived jobs (`video-studio-derive.js`) call
  `createJob(…, onSucceeded)`: the hook waits in `s.jobHooks` (by job id, epoch-checked) until a `refreshProject` that
  applied the server project (it returns `true`; a timeline drag defers it to `onDragEnd` via `pendingRefresh`) and the
  new asset is in `s.project`; it re-finds its clips by id and never assumes they did not move. Reverse requests the
  union of the source ranges of the clip's same-asset group members (`E.reverseRange`, limit 300 s; also
  `clipCapabilities.reversible`; `P.reverseFits` also checks `E.reverseRamped`, so Reverse is disabled for ramped clips
  and their same-asset partners). `reverseClips` keeps the reversed clip (the anchor) in place and mirrors the other
  same-asset members around it (`start' = anchor.start + anchor.end − end`, offset mirrored within the union), so linked
  picture and sound stay in sync; a moved member that would overlap a clip, touch a transition or leave the project
  refuses with reason `room` (`reverseNoRoom`), before the job and when applying; freeze remembers the asset and
  source frame (`E.sourceFrameAt`) and refuses when the clip shows another frame at the cut. Still jobs (freeze and
  export frame) send the clip position (`stillPayload`: `offset`, `clip_frame`, `speed`) with that `frame`, so the
  server samples at the clip's speed like the renderer. Reverse sends `name: E.reversedName(display name, localized
  videoStudio.reversedSuffix)`, which toggles the suffix so a reversed copy reversed again gets its plain name back.
  Both edits are tried on a
  copy before the job starts. Load order: `derive` loads directly before `projects` (`…-transport.js, -derive.js,
  -projects.js, -menus.js, video-studio.js`). The inspector's label swatches share `data-action="clip-label"` and
  differ by `data-value`, which `focusKey` remembers so a re-render keeps focus on the clicked swatch.
  Node checks: `go test ./ui -run 'TestVideoStudioClipModelChecks|TestVideoStudioEditLinks'`.
- Navigation and tracks (phase 1c): `video-studio-navigation.js` is DOM-free (`VideoStudioNavigation`: the JKL
  shuttle controller, edit points, marker steps, loop ranges, overview geometry, track order). Transport keys live in
  one table, `navCommand` in `video-studio-transport.js`; edit keys stay in `editCommand` (`-editing.js`). Every project
  change goes through `mutate` (one undo step); targets, heights, the loop and solo are view state (targets and heights
  per viewer and project in localStorage, solo session only). The preview takes signed rates (`setRate`), a loop range
  (`setLoop`) and audio solo (`setSolo`); `pause()` resets the rate to 1x. Markers (`markers`, in/out) are validated by
  `validateMarkers` in Go and `validMarkers` in JS; markers never render. Array order is paint order: `arraySideFor`
  is the one place that maps "above/below" to array positions for visual and audio tracks. The timeline draws clips only
  inside the visible window (`laneWindow`); the rubber band selects drawn clips. A frame-rate change keeps markers and
  in/out (`rescaleProject`), is refused when a clip would pass 60 minutes (`rescaleFits`), and the playhead keeps its
  time through it and through undo and redo (`frameAtRate`). Node checks:
  `go test ./ui -run 'TestVideoStudio(Navigation|Ruler|Marker|Track|Rate)'`.
- Source monitor (phase 1d): `VideoStudioSourceEdit` (`video-studio-source-edit.js`, DOM-free) plans the 3-point
  placement (`plan`) and the clips it places (`placements`: linked pair, picture-only or sound-only); the source
  part (`video-studio-source.js`) mounts the item as a second `VideoStudioPreview` instance over a one-clip project,
  so playback elements come from preview.js (`stopAtEnd` keeps an item from wrapping). Source edits run through
  `applyEdit`. In source mode `handleKeys` routes every key to `P.handleSourceKeys` and stops; Escape returns to the
  timeline. The position slider `[data-source-scrub]` is exempt from the text-field guard for every key but its own
  (arrows, Home, End, Page Up/Down), and a mouse scrub hands focus back to `.vs-app` (pointerup/change); an action that
  disables its own button (Clear in and out) does the same through `keepKeyFocus`, since keys are ignored on `<body>`.
  Marks are per item and viewer in
  `aurago.videoStudio.sourceMarks.<projectId>` (seconds, at most 200 entries). The picture and sound handles set
  `application/x-video-studio-part`; the timeline drop passes it as the fifth argument of `onDropAsset`, and
  `addAssetToTrack` hands such a drop to `P.dropSourcePart`, which places only on the lane dropped on. The handles are
  buttons too: a click, Enter or Space (`activatePart`) runs `sourceEdit(s, 'overwrite', part)` at the 3-point position on
  the matching target track, Ctrl/Cmd (click or Enter) inserts; a drag never counts as that click. The ResizeObserver
  watches `.vs-app` as well as the preview stage, because the stage is hidden in source mode and the narrow drawer's
  "widening closes it" logic depends on the app width. Node checks:
  `go test ./ui -run TestVideoStudioSource`.
- Media and projects (phase 1e): `VideoStudioFolders` (`video-studio-folders.js`, DOM-free) holds the folder rules
  (`normalizeFolder`, 1 to 4 segments of 1 to 64 runes, no control characters or `.`/`..`), the tree, usage counts, the
  list sort and the folder and name edits; edits return `true` or a reason. The bin (`video-studio-library.js`: media bar,
  grid, list, folder tiles, poster fallback) and its actions (`video-studio-mediabin.js`: dialogs, item and folder menus,
  removal, drops, library metadata) run their edits through `folderEdit` (one `mutate` undo step). Names and folders are
  project-owned: `keepOwnedAssetFields` keeps the local values in `adoptServerMedia` and `refreshProject`. While a removal
  deletes files, `s.mediaRemoving` (a counter) makes `mutate` and `undo` refuse and disables undo/redo; afterwards
  `P.forgetMedia` strips the deleted media and its clips from the project and its history. The list view is an ARIA
  table: `[data-assets]` gets `role="table"` named after the open folder, the header row holds `role="columnheader"`
  cells with the sort buttons (`aria-sort` only on the sorted column), and item, folder and upload rows are
  `role="row"` with `role="cell"` children (the thumbnail is `aria-hidden`); the grid has no table roles. The project
  manager and versions live in `video-studio-projects.js`; a restore needs `If-Match` and a clean editor. The export
  dialog offers the in/out range when both points are set (`P.inOut(s)` clamped to the enabled length, as Go's
  `EnabledDuration`) and sends `range {start, end}`; `exportName(s, job)` adds ` (in-out)` for a range result. Node
  checks: `go test ./ui -run TestVideoStudioFolderModel`.
- Preview engine, proxies and range check (phase 2a): `video-studio-preview.js` keeps the clock, media sync and audio
  and paints through a painter (`kind`, `canvas`, `frame(scene)`, `read(rect)`, `readScaled(width, height)` (2f), `release(clipId)`, `pending()` (2c), `dispose()`):
  `VideoStudioCompositor` (WebGL2; `video-studio-compositor.js` with `VideoStudioGL` and `VideoStudioShaders`, named
  stages `sourceTexture`, `clipPasses`, `placeClip`, `compositeClip`, `drawTransition`, `present`) or
  `VideoStudioPainter2D`. `describeFrame` builds the scene (visual tracks in paint order, items with
  `VideoStudioEffects.clipState`, one transition pair per layer). The painter is chosen only by `options.painter`
  (default `'2d'`; the stage passes `P.previewEngine(s)`, i.e. `'auto'` unless the hidden setting
  `aurago.videoStudio.previewEngine` is `2d`; the source monitor passes `'2d'`), never by probing `getContext`. Every
  fallback (no WebGL2, failed `create`, three `webglcontextlost` within 60 s, `fallback(reason)`) swaps in
  `canvas.cloneNode(false)`: never cache the canvas, re-query `[data-preview]`, hang overlays on `.vs-preview-mat`. The
  persistent notice `[data-preview-notice]` (`P.syncPreviewNotice`) shows `previewFallback` while the 2D painter
  draws, or `previewTransitionFallback` while the GL painter draws and `preview.failedPrograms()` lists a transition
  program (that transition then renders as a hard cut at p ≥ 0.5); otherwise, and with no project open, it is hidden.
  Smokes read pixels through `capture(rect)`, `settle(ms)`, `painterKind()` and
  `VideoStudioApp.capturePreview(id, rect)`, never `getContext('2d')`. Hold-last-frame is GL-only; the 2D painter keeps
  the earlier geometry. Paused seeks stay at frame starts (Task 13 found no off-by-one frames), so the preview adds no
  frame-centre seeking. `VideoStudioEffects` (DOM-free) mirrors the Go registry (`BUILTIN`, parity-tested), adopts
  `status.registry`/`unavailable`, feeds the inspector's transition buttons and `validTimeline`, and holds
  `fadeBlackWeights`, the measured fade-through-black curve (not linear; the GL shader takes it as `u_weights`).
  An image is ready (`Item.ready`) only when `naturalWidth && naturalHeight && complete !== false`, and
  `VideoStudioGL` refuses an incomplete image: a half-loaded PNG used to upload as black and never refresh
  (`TestDesktopVideoStudioSlowImageBrowser`).
  Proxies (`video-studio-jobs.js`): `P.queueProxy`/`P.pumpProxies` keep at most one queued or running `preview` job
  per project; `P.previewMediaURL` plays the proxy for Auto/Half/Quarter once ready and originals for Full or after a
  failure; the media cache key includes the URL; `visibleJobs` hides `preview` and `range_render`, and `loadJobs` does
  not refresh the project for them; library cards carry `[data-proxy-state]` (in the list view the badge sits after
  the file name in the ellipsized name cell, so a very long name can clip it: known minor issue).
  Range check (`video-studio-rangerender.js`): `P.rangeRenderRange(s)` (both in/out points clamped to `P.enabledEnd`,
  the helper the export dialog shares; else 5 s around the playhead; more than 60 s is `{error: 'too_long'}`, notice
  `rangeTooLong`, never clamped); `P.startRangeRender(s, range?)` saves with `P.saveUntilClean`, posts
  `{kind: 'range_render', range}` with `If-Match` and polls without toasts; the overlay `[data-range-overlay]`
  (`data-range-state`, `data-range-view`) sits in `.vs-preview-mat` with `<video data-range-video>` (same-origin URL
  only) and the actions `range-view` (`data-view`), `range-retry`, `range-close`. Rendered → Live seeks to
  `start + round(video.currentTime · fps)`; Live → Rendered puts the video at the playhead. Escape
  (`P.handleRangeKeys`, after dialogs, popovers and the narrow drawer) and Close cancel a running render; dispose and
  project switches close it with `keepJob`, so server jobs outlive the editor as before. Range results come back with
  alphabetically ordered JSON keys, so tests compare fields, not strings. Node checks:
  `go test ./ui -run 'TestVideoStudio(Effects|Canonical|Preview|Navigation|Proxy|RangeRender)|TestDesktopVideoStudioPreviewMediaLifecycle' -count=1`;
  browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopVideoStudio -count=1 -timeout 30m` (GL tests,
  the fallback test and the goldens use `newWebGLSmokeBrowser`; missing WebGL2 there fails) and
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run VideoStudioBrowser -count=1 -timeout 20m`. Both default
  smoke launchers on the development machine expose WebGL2, so a smoke that is not about the painter reads
  `painterKind()` instead of assuming one; the UI fixture also runs a forced-2D pass through the hidden setting. Never
  call `restoreContext()` in the same microtask as `webglcontextlost` (Chrome ignores it): wait a task first, and give
  every event wait a timeout.
- Keyframes and speed ramps (phase 2b), DOM-free part: `VideoStudioKeyframeEdit` (`video-studio-keyframe-edit.js`,
  directly after `-edit.js`) holds every keyframe and ramp operation, each returning `true` or a `VideoStudioEdit.REASON`:
  `keyFrame`, `atPlayhead`, `setAnimated`, `toggleKeyframe`, `setValues`, `neighbor`, `moveKeys`, `removeKeys`,
  `setEasing`, `copyKeys`, `pasteKeys`, `rampPreset`, `setRamp`, `applyRampPreset`, `addRampPoint`, `moveRampPoint`
  and `removeRampPoint`. The app runs them through `applyEdit`.
  - The shared math is in `VideoStudioEffects` (`ease`, `sample`, `sourceAt`/`timelineAt`, `rampPieces`, `clipSource`,
    `sourceExtent`, `speedAt`, `headShift`, `meanSpeed`, …; parity topics `easing`, `keyframes`, `ramp`).
  - Keyframe `f` is a source frame (the content anchor), so keys are never remapped on trims, split, slip, slide or
    speed changes. Head edits go through `headShift`.
  - `slip` and reverse refuse ramped clips with reason `ramp` (notice `editRamp`), and `setSpeed` replaces a ramp. On a
    ramped clip `setSpeed` keeps `round(sourceExtent / speed) + 1` frames; `'none'` is `setSpeed(1)`, which also resets
    a constant speed.
  - Presets start from `speedOf(clip)` (1 once ramped). By default they keep the source range: the duration whose
    ramped extent is closest, found by bisection. They then ripple the track like `setSpeed`; `keepDuration` keeps the
    length.
- Keyframes and speed ramps (phase 2b), UI part: `video-studio-keyframes.js` (DOM, directly after `-editing.js`) holds
  the inspector controls, the shortcuts and the strip.
  - Inspector controls: `[data-kf-path][data-kf-action="stopwatch|key|prev|next"]` (`P.keyframeView`,
    `P.handleKeyframeClick`, `P.syncKeyframeControls`; auto-key through `setValues`). A live drag whose auto-key is
    refused restores the clip in place (`liveValues` in `-stage.js`).
  - Shortcuts: `P.keyframeCommand`, `P.isKeyframeShortcut` and `P.handleKeyframeKeys` (Alt+K, Alt+↑/↓, matched by
    `event.code`). `handleKeyframeKeys` runs after the text-field guard, so the keys also work while a button has focus;
    inside number inputs Alt+↑ still steps the field natively.
  - Strip rendering: `timelineOptions.stripAfter` calls `P.keyframeStrip`, which draws `[data-kf-strip=<clipId>]` after
    the selected visual clip's track row while the clip has keyframes or a ramp, or `s.kfBoard` holds copied keys. The
    rows are `.vs-kf-row[data-kf-row="summary|<path>|speed"]` and carry no `data-track-row` and no `.vs-track-lane`, so
    every timeline gesture (drag, band, drop, blade, context menu) passes them by. A key at the very first lane pixel
    is half hidden under the sticky row head (known cosmetic issue).
  - Strip geometry comes from `VideoStudioKeyframes.stripModel(project, clip, pps, view)`, which places keys at
    `timelineAt` and flags keys outside the clip.
  - Strip input: `P.wireKeyframeStrip` (listeners on the timeline root with `s.abort.signal`) handles the diamonds
    `button[data-kf-key][data-kf-path][data-kf-f]`; a summary diamond has `data-kf-path="summary"`. The selection is
    `s.kfStrip`. Drags move in timeline frames with playhead snap, and arrows move keys. Del/Backspace stop the event,
    so the clip stays. The menu goes through `openMenu`, with `menuitemradio` easings. The handler also covers the
    chevron `[data-kf-action="expand"]` and the ramp points `button[data-ramp-point]` (log-scale y, clamped to the
    clip; drags keep the last valid state). The strip restores focus after its own re-renders, because the timeline's
    `focusSelector` does not know the diamonds.
  - Animation section: `[data-ramp-preset]`, `[data-ramp-keep]`, `[data-action="ramp-edit"]` (`P.applyRampPreset`), and
    the notice `[data-ramp-notice]` (`rampMuted`, also in Sound). While ramped, the speed field is empty with the
    placeholder `speedRampActive`. The clip badge is `.vs-clip-ramp`, and the timeline filmstrip samples `clipSource`
    (`video-studio-library.js`).
  - Preview: `clipState` evaluates keyframes for both painters, media sync uses `sourceAt`/`speedAt`, and linked audio of
    ramped video-track clips is muted (`rules.audioAllowed`).
  - Node checks: `go test ./ui -run 'TestVideoStudio(Keyframe|Ramp|Effects|Canonical|ValidTimeline|Preview|Navigation)' -count=1`.
    Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopVideoStudioKeyframesBrowser -count=1 -timeout 20m`.
- Colour and picture (phase 2c), DOM-free part: `VideoStudioPicture` (`video-studio-picture.js`, directly after
  `-effects.js`) holds the math of Go `picture_math.go` and `lut.go`: `whiteBalance`, `wheel`, `colorCoefficients`,
  `applyColor`, `saturationMatrix`, `cropRect`, `mirroredCrop`, `vignetteAngle`, `blurSigma`, `parseCube`, `sampleLUT`,
  `unitDomain`, `decodedSize`, `CAS_P0`/`CAS_P1` (= Go `casPeakP0`/`casPeakP1`), `capSize` and `passUniforms` (parity
  topics `color`, `wheels`, `picture`, `lut`).
  - `VideoStudioEffects` mirrors the 2c registry rows and Go's `validateCrop`/`validateEffects` (`validPicture`, called by
    `validTimeline`). `clipState` evaluates crop, flips and the `on` entries (`effects: [{index, id, values}]`).
  - `VideoStudioKeyframeEdit` adds the stack operations `addEffect`, `removeEffect`, `moveEffect`, `setEffectOn`,
    `resetEffect`, `setEffectParams`, `setCrop` and `setFlip`; every keyframe operation accepts `crop.*` and
    `effects.<i>.<param>`. Paste attributes has the groups `basePicture` (crop, flips; 2d adds blend and mask) and `effects`.
- Colour and picture (phase 2c), UI part:
  - Effects section (`video-studio-inspector-effects.js`, directly after `-keyframes.js`; `VideoStudioInspectorEffects`,
    `P.wireEffectStack`, `P.handleEffectStackKeys`, `P.effectCommand`, `P.addEffect`, `P.uploadLUT`): "Add effect" by
    category through `openMenu`, entries with collapse, on/off (`role="switch"`), reset, remove and drag reorder.
    Alt+↑/↓ on a focused entry header moves the entry and stops the event (`P.handleEffectStackKeys` runs before
    `P.handleKeyframeKeys`; elsewhere Alt+↑/↓ still jumps keyframes). Unavailable effects are badged `No export`, and
    the inspector shows `effectsCropAnimatedHint` while the crop has keyframes. The section shows for every visual
    clip, titles included; the crop block, the flips and crop mode skip titles (`clip.text`).
  - Effect key selection: selected `effects.<i>.<param>` diamonds in the keyframe strip are dropped by
    `P.dropEffectKeySelection(s)` (`video-studio-core.js`) when an edit moves, removes or replaces stack entries
    (`P.mutate`), and always on undo and redo (`P.undo`), since the stack indexes in those ids no longer match.
  - Colour wheels (`video-studio-colorwheel.js`, directly after `-inspector.js`; `VideoStudioColorWheel`): SVG,
    `role="slider"` with value text, arrows 0.01 (Shift 0.1), double-click and Delete (or Backspace) reset.
  - Crop handles (`video-studio-stage.js`): `P.setCropMode`, `P.updateCropBox` (called by `updateTransform`),
    `P.wireCropBox` and `P.handleCropKeys` (in `handleKeys` after `P.handleRangeKeys`). The overlay `[data-crop-box]`
    holds `[data-crop-source]`, `[data-crop-rect]` and eight `[data-crop-handle]` buttons; the inspector toggle is
    `[data-action="crop-mode"]`. A drag commits once through `setCrop` with the changed values only, and each arrow key
    is one edit. The math (`P.cropGeometry`, `P.cropDrag`, `P.cropDragDelta`) is Node-tested. The source size comes from
    `VideoStudioPicture.decodedSize` with the size the preview's `mediaSize(clipId)` reports, so the overlay of a
    rotated phone video shows its upright aspect.
  - Typed crop and box values next to an animated side: when one side of a crop pair (l/r, t/b) or a box pair
    (x/width, y/height) is animated, a typed value on either side is clamped to the room its animated partner leaves at
    the playhead (`fitAnimatedPair` in `video-studio-keyframe-edit.js`). The static field of an animated path is never written.
    A typed key next to a static partner stays as typed, because keys are checked per parameter only.
  - Effects library (`video-studio-effects-library.js`, directly after `-library.js`; `VideoStudioEffectsLibrary`,
    `P.renderEffectsLibrary`, `P.wireEffectsLibrary`): tab `effects` with `[data-effects-library]`, items
    `[data-effect-item]` (`<effect id>` or `look:<id>`), drag MIME `application/x-aurago-vs-effect` with a delegated drop
    on `[data-timeline]` (wired after `T().attach`), and a double-click or keyboard click (Enter or Space) that adds to
    the selected clip. Unavailable items are `aria-disabled` and greyed out.
  - LUT media: `.cube` imports by extension, the LUT filter chip shows only while LUTs exist, a LUT card dropped on a clip
    adds a lut effect (`P.dropLUTAsset`), effects that use a LUT count in `usageCounts`, and removing a LUT removes those
    effects in the same undo step (`removeLUTRefs` in `removeMediaFrom`; confirm copy `removeLUTUsedCopy`). `stripAssets`
    and `clipsUse` treat lut effect refs like clips. `dropRemovedMedia` (media removed in another window) treats a LUT
    that an effect still uses as a conflict instead of removing those effects.
  - Preview: the GL compositor runs `clipPasses` (crop/flip pass, effect passes, LUT textures through `loadLUT`);
    `pending()` counts LUT loads and keeps `settle` (through `painterIdle`) waiting; a failed effect program raises
    `previewEffectFallback`. The 2D painter crops and flips only.
  - Node checks: `go test ./ui -run 'TestVideoStudio(EffectStack|ColorWheel|EffectsLibrary|Effects|Picture|Canonical|KeyframeStrip|CropHandle|Painter2D|RemoveLUT)' -count=1`.
    Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopVideoStudioEffects(GL)?Browser' -count=1 -timeout 30m`.
- Compositing (phase 2d), DOM-free part:
  - `VideoStudioEffects` mirrors the 2d registry rows (`blend`, `mask.*`, the 14 blend modes, `chroma_key`) and Go's
    `validateBlend`/`validateMask` (`validPicture`). It adds `blendModes()`, `blendAvailable(id)`, `maskAnimated(clip)`
    and `MASK_PATHS`.
  - `clipState` gives `blend` (`'normal'` when absent) and `mask` (`null` or the eight values at the frame).
    `animatablePaths` lists `mask.*` only while the clip has a mask.
  - `VideoStudioPicture` holds the real-valued compositing forms of Go `compositing_math.go` (parity topics `blend`,
    `mask`, `key`): `BLEND_SWAPPED`, `blendChannel`, `compositeRGB`, `maskFrame`, `maskAt`, `keyColor`, `keyAlpha`,
    `despill`, `overlapBlend`, `passUniforms('chroma_key', …)`, `maskUniforms` and `boxRect`.
  - `VideoStudioKeyframeEdit` adds `setBlend`, `setMask`, `removeMask` (with its keys) and `pasteMask`. `setMask` takes
    any subset of the values (`shape`, `invert` and the six numbers, clamped to the registry ranges); animated paths get
    a key at the playhead, and a clip without a mask starts from the registry defaults. The internal `writeParams` accepts
    colour params (lowercased `#rrggbb`). `VideoStudioEdit`'s `ATTRIBUTE_GROUPS.basePicture` is crop, flips, blend and
    mask.
- Compositing (phase 2d), UI part:
  - Picture section (`video-studio-inspector.js`):
    - `select[data-blend]`: unavailable modes are disabled, but the clip's own mode stays selected. `[data-blend-hint]`
      shows `blendTransitionHint` while a transition joins two modes (`blendJoins`).
    - The mask block (`.vs-mask`, `maskMarkup`): `[data-mask-set="none|rect|ellipse"]` and
      `[data-mask-field="x|y|w|h|rotation|feather"]` with keyframe tools (`MASK_FIELDS`: percent of the box for x, y,
      width and height, degrees, percent of the box height for feather), then `[data-mask-invert]` and
      `[data-action="mask-mode"]` (`maskModeButton`).
    - Titles and stickers take both.
  - Edits (`video-studio-inspector-effects.js`), one undo step each:
    - `P.compositingChange`: the blend list, the mask fields (clamped to the registry range) and a chroma_key entry's
      colour input (`input[type="color"][data-fx-param]`);
    - `P.effectCommand`: the mask shape (`[data-mask-set]`), Invert (`[data-mask-invert]`) and the key presets
      (`[data-fx-key-preset="green|blue"]`, `KEY_PRESETS`).

    `VideoStudioInspectorEffects.addEntry` puts a new `chroma_key` at stack index 0 for Add effect and the effects
    library. `P.syncEffectControls` keeps the mask fields and the mask-mode button in step with the playhead.
  - Mask handles (`video-studio-mask.js`, directly after `-stage.js`):
    - Parts: `P.setMaskMode`, `P.updateMaskBox` (called by `updateTransform`), `P.wireMaskBox` and `P.handleMaskKeys`
      (in `handleKeys` after `P.handleCropKeys`). The DOM-free math `P.maskGeometry`, `P.maskDrag`, `P.maskDragDelta`
      and `P.maskNudge` is Node-tested.
    - The overlay `[data-mask-box]` is built once in `.vs-preview-mat`. It holds `[data-mask-shape]` (value `rect` or
      `ellipse`) and the `[data-mask-handle]` buttons `nw`, `n`, `ne`, `e`, `se`, `s`, `sw`, `w`, `rotate` and `feather`.
    - Mask mode needs a mask, hides the transform box, and ends when the clip, its selection, the playhead, the lock or
      the mask goes away. It ends crop mode and is ended by it; entering it closes the narrow inspector drawer.
    - A drag moves only the overlay until pointerup, which commits once through `setMask` with the changed values.
      Each arrow-key press is one `setMask` (`P.maskNudge`). `Enter` and `Escape` leave; `Delete` and `Backspace` are
      swallowed in the overlay.
  - Preview:
    - `clipPasses` runs the `KEY` pass (the chroma key). `placeClip` draws `PLACE_MASKED` for a clip with a mask (law L
      per fragment in box px, `maskUniforms` and `boxRect`). `MASK` is the GLSL `vsMask(p)` that `PLACE_MASKED` inlines.
      The masked program reports `('effect', 'mask')` once when it fails.
    - Non-normal modes and shared-mode transitions composite through `blendLayer(blendProgram(mode), layer, target)`,
      which copies back into the target (`BLEND_HEAD`, the mode's body in `BLENDS`, `BLEND_MAIN` in `shaders.js`).
      `transitionMode` is `overlapBlend`. A blended clip still goes through `placeClip`, so its mask works.
    - A failed blend program reports `('effect', 'blend:<mode>')` and the clip falls back to Normal. The preview notice
      for a failed effect is `previewEffectFallback`.
    - The 2D painter draws none of them; mask mode still shows the outline, because the overlay is DOM.
  - Node checks: `go test ./ui -run 'TestVideoStudio(Compositing|MaskHandles|Effects|Picture|Canonical|EffectStack|EffectsLibrary|KeyframeStrip|NavigationLoadOrder)' -count=1`.
    Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopVideoStudioCompositing(GL)?Browser$' -count=1 -timeout 30m`.
- Transitions and animation (phase 2e), DOM-free part:
  - `VideoStudioAnim` (`video-studio-anim.js`, directly after `-picture.js`) holds the in/out animation math of Go
    `anim.go` (parity topic `anim`):
    - constants: `SECONDS`, `SLIDE`, `POP`, `BLUR_RADIUS` and `BLUR_NODE`;
    - functions: `frameRange`, `spans`, `at`, `blurMasks`, `box`, `revealPx`, `valid` (Go `validateAnim`), `rescale` and
      `blurUniforms`.

    `time.js` calls `rescale` on a frame-rate change.
  - `VideoStudioEffects` mirrors the 24 transitions and the 8 animation presets. It adds:
    - `animPresets()`, `animAvailable(id)` and `transitionGroups()`;
    - `clipState().anim` (null at rest);
    - the optional 5th `placement` argument. The GL compositor passes it; the 2D painter applies `state.anim` itself, and
      the transform, crop and mask overlays keep the keyed box;
    - `wipeEdge(type, index, duration, width)`, which takes the overlap frame and returns null for every type but the
      two 2a wipes.
  - D1 rule: no transition parameter is computed outside `VideoStudioPicture.transitionParams`.
    - It is float32 through `Math.fround` and equals Go `transitionFrameAt`; the parity topic `xfade` is exact.
    - GL gets the parameters only through `transitionUniforms(type, index, duration, {width, height})`.
    - The scene's `transition` carries `index` and `duration` (`VideoStudioPreview.rules.transitionPair`), and painters
      never derive a parameter from `progress`.
  - Edits:
    - `VideoStudioKeyframeEdit.setAnim(project, clipId, side, values)` and `pasteAnim(project, source, clip)`.
    - `VideoStudioEdit.ATTRIBUTE_GROUPS.animation` is `anim_in` and `anim_out`.
    - `piece()` keeps `anim_in` only on the piece that keeps the clip's start, and `anim_out` only on the one that keeps
      its end. Freeze frame gives the still none; detach gives the audio clip none.
- Transitions and animation (phase 2e), UI part:
  - Animation section (`video-studio-inspector.js`, every visual clip; the speed ramp part stays video-only):
    - `select[data-anim-preset="in|out"]`;
    - `input[type="range"][data-anim-duration="in|out"]` (frames, with the output in seconds);
    - the clamp hint `[data-anim-clamped="in|out"]`.

    The edits are `P.animChange` and `P.liveAnimDuration` in `video-studio-inspector-effects.js`, one undo step per
    commit.
  - Transition picker (`transitionSection`):
    - the summary `[data-action="transition-picker"]` (aria-expanded) and the inline panel `[data-tx-picker]`, with
      `none` and one `role="group"` per `transitionGroups()` category;
    - buttons `button[data-transition][aria-pressed]`; unavailable ones are disabled with `transitionUnavailable`, but
      the current one stays enabled;
    - the open state is view state `s.transitionPickerOpen` (`P.setTransitionPicker` in `video-studio.js`). A choice
      and `Esc` close it and return focus to the summary; Esc is handled before the narrow drawer. It also closes when
      another clip is shown;
    - `VideoStudioInspector.transitionLength(project, clip, next)` gives the slider's step, its maximum and the default
      length.
  - Icons and timeline:
    - `VideoStudioIcons.transition(id, size)` is a 30×18 SVG per registry id: outgoing #4f5fd6, incoming #c66f31, the
      through colours, and white direction strokes. It has no gradients, ids or references, and unknown ids get the A|B
      split.
    - The timeline's transition tooltip reads the registry `label_key`.
  - Effects library:
    - The transition groups follow the effect groups: items `transition:<id>`, drag MIME
      `application/x-aurago-vs-transition`.
    - Dragging over a clip marks its nearer cut (`is-cut-drop-before`, `is-cut-drop-after`) when the neighbour touches
      or overlaps (`VideoStudioEffectsLibrary.cutAt`). A drop elsewhere shows `effectsLibraryDropOnCut`.
    - Drops, double-clicks and Enter/Space go through `P.addTransitionTo(s, clipId, type)`. It calls
      `P.applyTransition(s, type, frames, clipId)`, whose 4th parameter is the outgoing clip, which the edit selects.
  - Preview:
    - `video-studio-shaders.js` holds the 20 new transition programs.
    - PLACE and PLACE_MASKED read `v_box` and `u_reveal`, and `placeClip` always sets both.
    - The blur_in cascade (`blurPasses`, `blurCascade`) runs after the effects: two BLUR pairs at S and S/4, then two
      `ANIM_MIX` passes. It reports `('effect', 'anim:blur_in')` when the cascade fails.
    - The 2D painter draws offset, scale, opacity and the wipe, but no blur. It draws Dissolve, Through black and the two
      2a wipes as in the export, and the 20 transitions of 2e as a dissolve.
  - Node checks: `go test ./ui -run 'TestVideoStudio(Anim|Transition|Effects|EffectsLibrary|Canonical|Picture|Shaders|Painter|Preview|NavigationLoadOrder)' -count=1`.
    Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopVideoStudio(TransitionsGL|AnimGL|Anim)Browser$' -count=1 -timeout 30m`.
- Scopes and polish (phase 2f), DOM-free part:
  - `VideoStudioScopeMath` (`video-studio-scope-math.js`, directly after `-anim.js`; index §4.26.3–§4.26.4):
    - constants `SIZE` (256 × 144), `HISTOGRAM`, `WAVEFORM`, `VECTORSCOPE`, `READ_INTERVAL_MS` (100), `KINDS` and
      `COLORS`;
    - functions `luma`, `chroma`, `skinAngle`, `analyse`, `histogramImage`, `waveformImage`, `vectorscopeImage`,
      `graticule`, `due` and `prefs`;
    - rules: BT.709 float luma, Cb/Cr scaled to ±0.5, the skin line on the NTSC I axis mapped into Cb/Cr, log
      intensity, fixed colours, no text in the canvases.
  - `VideoStudioPicture.EFFECT_CAP_LONG` (1280) and `effectSize(width, height, canvas, drawn, cap)` (O2, index §4.26.8).
    The target is the smallest of the canvas size, the drawn size and the cap on the long edge, never below 1 px. With
    `drawn` null and `cap` 0 it equals `capSize`.
  - `VideoStudioInspector.pickerMove(lists, current, key)`: the transition picker's arrow, Home and End rules over the
    measured columns. Disabled types are skipped and the moves never wrap.
- Scopes and polish (phase 2f), painters and preview:
  - Both painters have `readScaled(width, height) -> Promise<{width, height, data} | null>`: a point-sampled, top-down
    RGBA8 copy of the last frame, 1..1024 px per side. It never paints and never changes the canvas.
    - GL draws the composite through `scope-sample` (`VideoStudioShaders.SCOPE_SAMPLE`) into a pooled target and reads
      it with `ctx.readAsync(target, rect)` (`video-studio-gl.js`).
    - `readAsync` keeps one pack buffer per context generation and size, one fence per read, and one read in flight. It
      polls on the next animation frame or a 16 ms timer, and resolves null on a context loss, after 3000 ms or while
      another read is pending.
    - Nothing reads the WebGL canvas, and nothing reads back synchronously (spike S1: 11–32 ms stalls on iGPUs).
    - The 2D painter reads a reused `willReadFrequently` small canvas with smoothing off. Its own canvas is never
      `willReadFrequently`.
    - `pending()` still counts LUT loads only.
  - Preview:
    - mount option `onPaint()`: called last in every paint, inside try/catch; it must not paint;
    - mount option `effectCap()`: copied into `scene.effectCap`, 0 without the option;
    - instance method `readScaled(width, height)`. `capture(rect)` still paints first.
  - O2:
    - the stage passes `effectCap: () => P.previewEffectCap(s)` (`video-studio-core.js`: 0 at full, else
      `EFFECT_CAP_LONG`);
    - `clipPasses` sizes its targets with `effectSize(…, canvasSize(), drawn, cap)`. `drawn` (`passDrawn`) is placement's
      draw rect at rest for a static crop and a static width and height, and is used only while the cap is on;
    - PICTURE does the downscale, so `u_size` and the blur σ follow the target;
    - full quality and the golden engine page (no option) render exactly as before.
- Scopes and polish (phase 2f), UI part:
  - Scopes panel (`video-studio-scopes.js`, directly after `-rangerender.js`). `P.wireScopes` runs once from
    `appRender`, after `P.wireMaskBox`, and inserts:
    - the toggle `[data-action="scopes"]` (`aria-expanded`, `aria-controls`) before `[data-action="range-render"]`;
    - the dock `.vs-scopes-dock[data-stage="timeline"]` after the timeline stage, holding
      `section.vs-scopes[data-scopes]` (`role="region"`).
    The section holds the kind buttons `[data-scope="histogram|waveform|vectorscope"]` (`aria-pressed`; the last shown
    kind stays pressed), the focusable tile row `.vs-scopes-tiles`, and one `figure[data-scope-tile]` per kind with a
    `canvas[data-scope-canvas]` (`role="img"`).
    - View state `s.scopes = {open, show, last, pending, dirty}`; preference `aurago.videoStudio.scopes`, read through
      `prefs`.
    - Parts: `P.syncScopes` (from `renderUI`), `P.setScopesOpen` and `P.toggleScope`; `P.handleScopeClick` (in
      `handleClick` after `P.handleRangeClick`) and `P.handleScopeKeys` (Esc, in `handleKeys` after `P.handleRangeKeys`);
      `P.scopesAfterPaint` (the `onPaint` hook), `P.readScopes` and `P.drawScopes`.
  - Readback rule: a closed panel or source mode reads nothing. While playing, a paint reads at most every
    `READ_INTERVAL_MS` (`due`); while paused, every paint reads. One read is in flight: a due paint during it sets
    `dirty`, and one trailing read of the last frame follows; a paint that is not due is dropped. Opening the panel
    reads at once.
  - Every 2f call in `video-studio.js` is guarded (`if (P.wireScopes) …`), because the camera handoff browser tests
    load shorter script lists.
  - Transition picker keys: `P.handleTransitionPickerKeys(s, event)` (`video-studio-inspector-effects.js`) runs in
    `handleKeys` directly after the picker's Esc line. It measures the columns (the buttons that share the first
    button's `offsetTop`), calls `pickerMove`, focuses the result and prevents the default. Choosing a type from None
    takes the length slider's value, whether or not the slider is disabled.
  - Library and labels: a transition drop or double-click without a picture clip shows `effectsLibraryTransitionNeedsClip`
    (`NEEDS_CUT` in `video-studio-effects-library.js`); effects keep `effectsLibraryNeedsClip`. Every word of a transition
    label has at most 13 characters, except in ja and zh (`TestVideoStudioTransitionLabelsFitThePicker`).
    `animClamped` has U+00A0 before the unit in every locale that has a space there (`\xa0` in `animClampText`'s
    fallback).
  - Job failures and media: `jobFailure` maps `render_too_many_luts` to `tooManyLUTsAtOnce`; `removeMediaFrom` calls
    `removeLUTRefs` alone, and `mutate` drops the strip's effect-key selection when the stack changes.
  - Node checks: `go test ./ui -run 'TestVideoStudio(ScopeMath|ScopesPanel|ScopesLocales|Painter2DReadScaled|PreviewEffectCap|PictureUniforms|TransitionLabels|TransitionPicker|AnimInspector|EffectsLibraryTransitions|EffectKeySelection|RangeRender|Proxy|NavigationLoadOrder)' -count=1`.
    Browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopVideoStudio(ScopesBrowser|AnimBrowser|EffectsGLBrowser)' -count=1 -timeout 30m`.
    Preview benchmark (reported, never asserted): `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./ui -run '^TestDesktopVideoStudioPreviewBenchmarkOptIn$' -count=1 -v -timeout 60m`.
    - Variables: `AURAGO_VS_BENCH_LAUNCHERS` (default `gpu,swiftshader`), `AURAGO_VS_BENCH_BROWSER`,
      `AURAGO_VS_BENCH_GPU_FLAGS` (`force_low_power_gpu` for the local iGPU; Chrome's default adapter on the 2f
      measurement machine is that UHD chip, and `force_high_performance_gpu` selects the Arc B580),
      `AURAGO_VS_BENCH_REPS` (default 3), `AURAGO_VS_BENCH_SCENARIOS` (a comma list of scenario names; default all eight).
    - The Renoir container recipe is in plan index §6.
- Layout: toolbar (project popover with switch/rename/new, save chip
  `[data-action="save"]`, undo/redo, format `[data-canvas]`, tasks popover with
  `[data-jobs]`, shortcuts, export), tabbed library (media/text/stickers/effects/AI;
  all panels stay in the DOM), preview with transport, contextual inspector,
  resizable timeline (`--vs-timeline-h`, per-viewer `localStorage`). Icons come
  from `VideoStudioIcons`, never from the shell's `iconMarkup`. Notices are a
  toast in `[data-notice]` (info hides after 5 s, a download toast after 15 s,
  errors stay). An open dialog or the narrow drawer makes everything behind it
  `inert` (`syncInert`, driven by a MutationObserver on both dialog hosts); Tab
  cycles inside, Escape in the conflict dialog is "Keep editing", and a kept
  conflict shows in the save chip until resolved. Finished probe
  jobs are not listed; a finished render announces once with a download named
  `<project>.mp4` (`<project> (in-out).mp4` for a range export). A cancel answered with 409 `job_finished` (ended, or already
  saving its result) is not an error: the list reloads and polling reports it.
- `video-studio-media.js` extracts up to eight thumbnails per video with one
  hidden `<video>` and waveform peaks (`decodeAudioData`, ≤80 MB) sequentially;
  only the editor's own artwork names (`title-<Date.now()>.png`,
  `sticker-<built-in name>.png`) are hidden from the media bin. Key names and
  decimal seconds follow the UI language. Strings with `{{count}}` go through
  `P.trCount(s, key, count, one, other)`: the `<key>_one` string for exactly one (the Desktop convention of
  `GalleryLibrary.countLabel`), the key itself for every other count. Each such key has `_one` in all sixteen
  locales; Czech and Polish plurals stay count-neutral (`Položky: {{count}}`), since one/other cannot express
  their 2–4 forms. Checks: `go test ./ui -run 'TestVideoStudio(CountStrings|TrCount)'`.
  The inspector shows seconds (`m:ss.cc`, input also `4,5`) and percent; the
  model stays in integer frames at the project rate and 0..1 boxes. Its fine-tune fields toggle with
  `hidden`, not `<details>`: closed `<details>` content still reports client
  rects and broke the drawer focus trap.
- A project loaded without tracks gets Video 1, Overlay 1 and Audio 1 once (no
  undo step). Placing media creates a missing track kind. The timeline lists
  visual tracks front-most first, then audio; array order stays the paint order.
- Persist the canonical project with strong `If-Match` ETags. Imports change the
  server copy's asset list and ETag, so a 412 save first reads the server copy:
  when its content (everything but `assets`) equals the last saved project, adopt
  its assets and the observed ETag and retry once (`adoptServerMedia`). Any other
  412/428 must present Reload, Replace latest or Keep editing; never retry a real
  conflict without an explicit choice and the observed latest ETag. Media removed in
  another window (missing from `GET …/media`, which also lists unprobed imports) leaves
  the local copy and its undo snapshots when no clip uses it: in `adoptServerMedia`, in a
  save answered 400 `project_asset_invalid` (`forgetServerRemovedMedia`, one retry) and in
  `recoverDraft`. A clip of removed media is a conflict, never a retry loop; Replace latest
  drops removed media with its clips (`forgetMedia`, notice `conflictMediaDropped`) and
  Reload discards the local draft. The canonical
  form normalizes `text_style` like the server's omitempty JSON (no false/0/empty
  values, sorted keys), or every refresh of a title project reports a conflict. Bind imports, jobs,
  project refreshes and draft recovery to the captured project ID and epoch.
  Window close awaits saving until clean or blocked by a visible failure or
  conflict choice.
- Timing goes through `window.VideoStudioTime` only (`rate`, `formatTime`, `parseTime`, `shortClock`,
  `formatSeconds(frames, project, locale)`, `framesAt`, `maxFrames`, `rescaleProject`); no part keeps an `FPS`
  constant. The inspector's `formatSeconds(frames, locale, project)` wrapper keeps its old argument order.
  Thumbnails are cached in seconds (`{time, url}`), waveform peaks per second. Changing the frame rate in
  Project settings rescales clip edges in one undo step after a confirm; durations are clamped by the source extent
  rule with the clip's speed. The preview canvas backing store
  follows the per-viewer quality (`aurago.videoStudio.previewQuality`: auto = long edge ≤ 1920 px, full, half,
  quarter); the transform box stays in CSS pixels. Every quality but full caps the GL effect passes at 1280 px on the
  long edge and sizes them to the drawn picture (`P.previewEffectCap`, 2f O2); full is the exact preview.
- Timing and transitions use integer frames at the project rate (24/25/30/50/60, NTSC ×1000/1001). The outgoing clip owns an
  exact overlap with the next clip on its track; transitions apply on change.
  `setTransition` moves the following clip and every later clip on the track
  together; `repairTransitions` runs after every edit (mutate, recordChange,
  dragClip) so a transition follows the actual overlap or disappears without one.
  Timeline gestures capture the pointer on the timeline container; a lost capture
  or window blur cancels them.
  Titles are PNGs cropped to their text block and positioned with the clip box
  like static stickers (both separate image overlays). Titles and stickers look
  for a free unlocked overlay track (`findOverlaySlot`) before their artwork is
  uploaded, adding an overlay track when allowed. Re-applying title artwork keeps
  the box center and the user's scale. The reference box is the one the clip shows at the playhead (`clipState`,
  clamped to the clip, so keyframed values count); the new box is written through `VideoStudioKeyframeEdit.setValues`,
  so animated values get a key. Preserve editable text/style metadata and
  use only same-origin staged media URLs.
- Bound preview media to the active/near clips during playback and release clips
  that leave that window. Preserve both players for an actual transition overlap;
  backward seeks must rehydrate released media and dispose must release all media.
  Dissolves, wipes and fade through black compose the clip pair in a track layer like the export's xfade
  (`VideoStudioPreview.rules.transitionPair` includes `black`; the 2D painter mixes out·w.out + in·w.inc + opaque
  black·w.black with `fadeBlackWeights`), so fade through black darkens the tracks below too, in both painters.
  Empty areas are black.
- A title's PNG is derived from its text and style. Edits mark the clip stale and
  start a per-clip 900 ms debounce (`markTextStale`), independent of the selection.
  Commit generated artwork only if its project, clip, text, normalized style and
  source asset still match and both the per-clip and the global Apply revision are
  current. The commit amends the current state without its own undo step; the text
  edit's pre-edit snapshot lives on `s.textBefore` (never on DOM nodes) and enters
  history when the field commits or another edit starts. Undo/Redo bump the global
  revision, cancel debounces and regenerate restored titles whose PNG key no longer
  matches (`resyncArtwork`). Results arriving during a drag wait for its end
  (`retryArtwork`); close, project switch and export await `flushArtwork`. While a
  title field or slider is being edited, re-renders only update the status line.
  Await PNG rendering before upload. Style comparison must ignore JSON key order
  and omitted default values. `refreshProject` waits for a running save and reads
  again if a save started meanwhile.
- Keep editor shortcuts off native controls: Space on buttons/links remains native,
  default-prevented events and unknown Ctrl/Meta/Alt shortcuts are ignored, while
  drawer Escape/Tab focus handling runs before control guards. Ctrl+Z remains
  available for editor Undo without intercepting text-field Undo.
- Offer/send AI aspect ratio only when the provider supports it (MiniMax currently
  does not). Localize stable job error codes; a terminal
  `external_status_unknown` warning takes precedence over retry-safe failure text,
  but a running job must not show that warning.
- Keep the media bin, preview and inspector responsive; below 920 px the
  Inspector control opens a keyboard-dismissable drawer without hiding the
  timeline. Width queries name their container: `vs-app` for the app breakpoints (1180, 920, 650, 420 px) and
  `vs-center` for the transport drop-outs, which measure the preview column (above 1180 px it is the app width
  minus 592 px). A query never styles its own container, so the breakpoints set `--vs-lib-w`, `--vs-insp-w` and
  `--vs-head-w` on `.vs-workspace` and `.vs-timeline-panel`, never on `.vs-app`. The library is the `vs-library`
  container: a rail of 260 px or less (medium and narrow windows) stacks the import buttons, shows the filters two by
  two, keeps the media bar on one line with icon view buttons (`aria-label` carries the name), uses two grid columns
  and a two-line list that never scrolls sideways (name; duration and the sorted column, from `[data-assets]
  [data-sort]` and the header cells' `data-column`). The wide rail keeps the full list, which scrolls sideways. On
  phones (app 650 px or less) the library is a strip below the preview, 46 % of the workspace and at most 224 px. The
  import buttons, view switch and folder actions share one row (`.vs-media-bar` is `display: contents` in the media
  panel's grid; labels end in an ellipsis instead of overflowing); search, filters and the count stay hidden. Grid,
  list and the Text and Stickers panels scroll vertically only. The list shows name, duration and the sorted column
  (a strip of 560 px or more also resolution and size) in fixed columns shared with its header, switched by
  `[data-assets][data-sort]`; like the narrow rail, it sorts only by visible columns. Below 600 px the breadcrumb
  shows only inside a folder, on its own line. Use
  the real Standard/Fruity theme tokens and all sixteen Desktop locale dictionaries. Verify UI behavior with
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopVideoStudioBrowser$'` (its phone pass checks 390x844
  and 650x900 in both themes and saves `phone-*.png` to `AURAGO_VIDEO_STUDIO_SCREENSHOTS`),
  `go test ./ui -run 'TestVideoStudio(Helpers|TimelineEdits)$'` and the real-FFmpeg
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestVideoStudioBrowserRealExport$'`.
- Without a project (zero projects or the feature switched off),
  `.vs-preview-stage.vs-no-project` hides the canvas and preview placeholder and
  gives the empty-project card the whole stage; `vs-stage` height container
  queries compact it so New project stays visible in a ~135px stage. `renderUI`
  clears the state once a project loads. `TestDesktopVideoStudioStartMenuBrowser`
  asserts the card is visibly hit-testable at 1280x800 and 1910x760 in both themes.

### Mission Control contract

- Mission Control is a master-detail workbench composed of
  `mission-control-schedule.js` (pure cron builder/describer),
  `mission-control-triggers.js` (grouped trigger catalog, picker, config
  panel), `mission-control-menus.js` (inline SVG icons, window + context menu
  builders), `mission-control-list.js`, `mission-control-detail.js`,
  `mission-control-editor.js` and the shell `mission-control.js`. Load order in
  `module-loader.js` is helpers first, shell last; there is no modal module.
  Keep `window.MissionControlApp = { render, dispose }` and every module below
  the desktop JS line budget.
- Mission Control UI text lives under `desktop.mc_*` (all 16 locales,
  `TestDesktopMissionControlTranslationsCoverAllLocales`); trigger labels and
  hints reuse `missions.*`, with leading legacy emoji stripped through
  `MissionControlTriggers.label`. Priorities use the emoji-free
  `desktop.mc_priority_*`, never `missions.form_priority_*`. Window menus use
  `desktop.menu_file` / `desktop.menu_view`; the trigger min-interval uses
  `desktop.rel_time_seconds` with `{{count}}`. Menu `icon` values are desktop
  icon keys (mini/Papirus); the inline `ICONS` map is for in-app markup only.
- The shell persists filter/sort/list width/collapsed state under
  `aurago.desktop.mission-control.prefs`, switches to single-pane below 720 px,
  registers a `beforeClose` dirty guard through `setWindowBeforeClose`, and
  scopes document shortcuts with `isActive()`. Local running missions can be
  cancelled via `POST /api/missions/v2/{id}/cancel`; `next_run` comes from the
  mission payload. The editor re-validates live after the first failed save.
- `.vd-mc [hidden]` forces `display: none` because module display rules
  otherwise beat the UA hidden default; keep the stylesheet theme-native
  (`--vd-theme-*`, `--vd-accent`) and free of dark-only literals. The colour
  reset is `.vd-mc :where(button) { color: inherit }` at (0,1,0) so
  single-class button colours (link, back, tab, quick action, danger) apply,
  while `.vd-mc button { font: inherit }` keeps (0,1,1); accent and danger
  text on buttons and links uses `--mc-accent-ink` / `--mc-danger-ink`, never
  the raw accent or `#e5484d`.
- Verify with `go test ./ui -run 'MissionControl|RelTime'`,
  `npm run test:mission-control` (schedule, plus flow missions on a stub DOM in
  `scripts/test-mission-control-flows.mjs`), `npm run test:dashboard-cron` and
  the opt-in `TestDesktopMissionControlBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`).

### App theme bridge contract

- Everyday apps Writer/Sheets, Todo/Calendar, Settings, Calculator, Chat, File
  Manager, Quick Connect, Mission Control, Software Store, Gallery, Notes,
  Viewer, Looper, Cheater, People, Launchpad, Zipper, Pixel, Log Viewer,
  System Info, Pet Picker, Radio, Camera, Code Studio, Network Cameras,
  Noisemaker, Live Speech, Homepage Studio, Game Maker, OpenSCAD, the Webamp
  launcher, Chess chrome, Nasscad, Mailer and Infra read `--vd-theme-*` for chrome,
  panels, controls, borders, and shadows. Infra's two stylesheets are held to that by a Node guard in
  `scripts/test-infra-ui.mjs` (spec §12): no hex or literal colour functions outside `color-mix()`, no named colours, no
  `--vd-font` (use `--vd-font-ui`), no uppercase or monospace labels, no `[data-theme…]` selectors, and every `transition` or
  `animation` sits inside `@media (prefers-reduced-motion: no-preference)` on selectors that start with
  `body:not([data-animations="false"])`.
- Fixed accents: an app that paints text on the raw accent exposes a root
  token that a fixed-accent skin overrides in an App tokens rule of its
  skin file (`.vs-app` `--vs-accent-strong`/`--vs-accent-hover`,
  `.code-studio` `--cs-accent-contrast`, dark by default); Mission Control
  mixes `--mc-accent-ink` (30 % accent) and `--mc-danger-ink` (60 % danger,
  45 % under `[data-scheme="light"]`) with `--vd-text` instead and needs no
  skin rule.
- Apps read the colour scheme from `document.body.dataset.scheme` (`light` or `dark`), never the theme id, and observe `data-scheme` together with `data-theme` and `data-fruity-mode` to re-theme live (Code Studio editor, CYD Creator code, Layerling, Leafy). The Fruity check (`dataset.theme === 'fruity'` with `fruityMode`) is only a fallback for harnesses that set `data-theme` directly. Verify `TestDesktopThemeRegistryShellUsesCapabilities`.
- Calculator programmer display (`.vd-calc-prog-display`) and history chrome
  use `--vd-theme-panel-bg` / `--vd-theme-border` / `--vd-theme-muted`. HEX,
  DEC, OCT, and BIN labels stay muted. Do not put `.vd-calc-base button.active`
  or `.vd-calc-tabs button.active` in the control `!important` bridge.
- Writer page content may stay on a light paper surface (`--vd-editor-page-bg`);
  toolbars, status bars, and sheet chrome follow the active desktop theme.
- Chat user bubbles may keep an accent wash (`--vd-theme-accent-soft`); agent
  bubbles use theme panel material instead of fixed white glass.
- Quick Connect xterm/VNC viewports and the built-in Terminal screen may stay
  on a dark terminal surface (`#0d1117` / `#0f172a`); toolbars follow theme.
- Gallery thumbnails (`.vd-gallery-thumb`), the info-panel preview
  (`.vd-gallery-info-preview`) and the lightbox stage (`.vd-lightbox-stage`)
  may stay dark for media contrast; card chrome, toolbar, info panel and the
  lightbox bar follow theme.
- Cheater code blocks keep a dark readable code surface (`--cheater-code-bg` /
  `--cheater-code-fg`); app chrome uses `--vd-theme-*` through `--cheater-*`.
- People status badges keep semantic colors (`#e8a020` / `#6495ed` / `#32cd32`);
  danger stays `#e74c3c`. Accent-on-white remains on primary People buttons.
- Zipper local `--zipper-*` aliases map to `--vd-theme-*`; the in-app preview
  overlay uses `--vd-theme-panel-bg`.
- Pixel canvas, checkerboard, and image viewport stay a dark work surface
  (`#0e1117`); toolbars, rails, panels, and dialogs follow theme via
  `--pixel-*` aliases.
- Log Viewer level colors and log-line semantics stay readable; toolbar,
  sidebar, and pane chrome use theme tokens. System Info gauge and chart
  accents may stay.
- Radio owns a theme-independent wood/champagne-metal receiver skin, including
  the existing window chrome scoped to `.vd-window[data-app-id="radio"]`.
  Material assets live under `ui/img/radio/`; retain real shell menus/resize
  handlers. Tuning selects current results without playing until activation.
  Late catalog/stream results and disposal must not resume playback. Favorites
  remain available when empty; meters animate only during playback and respect
  reduced motion. Verify with `TestDesktopRadioBrowser`.
- RTL-SDR owns a theme-independent graphite receiver skin scoped under
  `.sdr-app` with local `--sdr-*` tokens and its own label/display faces. Keep
  all `.sdr-*` out of the theme bridge; the shell window chrome stays native.
- Camera viewport stays black (`#000`) for live preview; toolbar and controls
  use `--cam-*` aliases mapped to `--vd-theme-*`. Error banner keeps semantic
  danger colors.
- Code Studio editor/terminal surfaces follow `--cs-*` aliases mapped to
  `--vd-theme-*`; CodeMirror theme selection stays in `code-studio/editor.js`.
  Editor text defaults to 12px in CodeMirror and the textarea fallback; saved
  zoom preferences remain authoritative and View > Reset Zoom restores 12px.
- Network Cameras live tiles and detail video stay dark viewports (`#05080d` /
  `#030509`); toolbar, cards, and modal chrome use `--nc-*` aliases mapped to
  `--vd-theme-*`. Online/offline and danger badges stay semantic.
- Noisemaker keeps the brand pink/purple gradient (`--nm-accent-2`) on create
  and play controls; chrome, library cards, and the player bar use `--nm-*`
  aliases mapped to `--vd-theme-*`. Cover play overlays may stay dark.
- Live Speech canvas FX stay decorative; the lab panel and realtime surface
  use `--vd-theme-panel-bg`.
- Homepage Studio preview letterbox (`.vd-hp-preview-zone`) and the website
  paper panel (`#f8fafc`) stay work surfaces; chrome uses `--hp-*` aliases
  mapped to `--vd-theme-*`. Icon filters stay light/dark aware. History type
  colors stay semantic.
- Game Maker preview checkerboard and iframe stay dark (`#0d0e14` / `#050509`);
  chrome uses `--gm-*` aliases mapped to `--vd-theme-*`. Brand purple/teal
  (`--gm-accent`, `--gm-accent-2`) and status badges stay.
- OpenSCAD chrome uses `--oscad-*` aliases mapped to `--vd-theme-*`. The 3D
  preview letterbox and panel stay dark work surfaces (`#071018` / `#050a10`);
  `light-preview` may flip the panel to `#f2f6f8`. Warm/danger and accent-on
  `#061014` stay semantic. Icon filters stay light/dark aware. Do not put
  `.oscad-primary` in the control `!important` bridge.
- Webamp launcher chrome uses `--vd-theme-*`. The embedded Winamp player skin
  stays authentic and stays out of this bridge.
- TeeVee is a theme-independent wood/metal CRT receiver. Its visual source is
  `documentation/assets/teevee-retro-reference.png`; retain the real shell menus
  and window actions. Keep all `.teevee-*` out of the theme bridge.
  CRT bezel/tube proportions follow the reference at every window size; fit the
  initial receiver bounds proportionally to the desktop, then enforce the
  1140x540 minimum so the left sidebar stays visible, including session restores.
  Only desktops smaller than that minimum may use the compact layout; the tube
  remains proportional independently of window bounds.
  Free header space, including the decorative model logo, stays draggable.
  Hide the on-tube fullscreen button two seconds after the first `playing` event
  per stream; tube hover or keyboard focus reveals it. Reset the timer on source
  reset/stop/disposal, and preserve this behavior with reduced motion enabled.
  Material and icon provenance lives in `ui/img/teevee/README.md`. CRT and glass
  have separate, persisted View switches. The one existing video element owns decoding/audio;
  `teevee-crt.js` owns only rendering. Blocked texture access or WebGL failure
  preserves native playback with a labelled basic filter. Never force CORS or
  globally proxy all streams to enable effects. Explicit reconnect applies only
  to the current station. Cancel renderer work while hidden/minimized and dispose
  callbacks, observers and GPU resources. Keep `playbackID` and catalog guards.
- Chess chrome uses `--chess-*` aliases mapped to `--vd-theme-*`. The wood
  frame (`--chess-board-frame*`) and felt (`--chess-felt`) stay the board
  surface. Warn/danger/good stay semantic.
- Nasscad shell uses `--vd-theme-app-bg`; the bundled iframe viewport stays
  `#111318`.
- Wikipedia (`local-wikipedia`) chrome reads `--vd-theme-*`; the article iframe
  stays a white reading surface (`#ffffff`, light color scheme) in every theme.
- Sysworld HUD uses `--sw-*` aliases mapped to `--vd-theme-*`. The 3D canvas
  and vignette stay a dark work surface (`#020208`). Brand cyan
  (`--sw-accent`), event/tone semantics, and `.sw-btn.active` stay. Do not put
  `.sw-btn.active` in the control `!important` bridge.
- Galaxa, Quake, and SIP-phone hardware chrome remain excluded.

### Desktop Phase 3 contract

- Standard-theme taskbar window buttons and Fruity dock app buttons show a
  hover/focus thumbnail preview (`.vd-taskbar-thumbnail`) for windows on the
  active space only. Compact viewport and coarse-pointer layouts disable
  previews. Dock hover must not call `findExistingAppWindow` (that switches
  spaces). Win/Meta+Arrow snaps the active window; Ctrl+Alt+Arrow keeps
  cycling spaces.
- Thumbnails clone DOM window content when possible; iframe-heavy windows show
  a live-window fallback instead of a blank capture.
- `AuraDesktopMediaSession` owns OS media handlers/metadata. Registrations are
  owner-bound; disposal releases only that owner. Personal Radio has priority
  over Webamp, then the latest active Radio/TeeVee/RTL-SDR player. Metadata
  refreshes do not steal ownership. Radio direct streams use native no-CORS
  audio; background TeeVee playback remains intentional. No global volume control.
- Chess disposes its AudioContext and scheduled tones. Galaxa cancels RAF while
  inactive and restarts at most one loop on visibility/focus return. Gallery
  errors use translated messages; Detective renders invalid URLs as text.

- `galaxa-*.js` implements Galaxa Deluxe, a modular Canvas 2D arcade shooter
  with procedural audio, biomed progression, parry/super combat, and persistent
  meta-progression.
- `log-viewer*.js` implements Log Viewer, a first-party desktop app for
  browsing and tailing AuraGo log files. Load order is
  `log-viewer-filters.js` then `log-viewer.js`. Exposes
  `window.LogViewerApp = { render, dispose }` and
  `window.LogViewerFilters = { create }`. Every window owns its
  `EventSource` (or 2s tail poll fallback), ring buffer, keyboard
  handlers, and timers and must close them in `dispose`. Styles are
  scoped under `.vd-logviewer`. The virtualized log scroller must keep
  native incremental wheel movement in both directions; disable browser
  scroll anchoring where rows are replaced. Visible strings use
  `desktop.app_log_viewer` plus `desktop.log_viewer_*` in all 16
  `ui/lang/desktop/*.json` files. File-list sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`.
  Download is hidden when the desktop
  is readonly; the backend also returns HTTP 403 for
  `/api/desktop/logs/download` in that mode.
- `chess*.js` implements Chess, a desktop chess app using `cm-chessboard`,
  `chess.js`, a local Stockfish WebWorker, and the optional AuraGo agent move
  endpoint. Features three opponent modes (Computer, Agent, Local 2P),
  optional chess clocks (3/5/10 min), captured-material tray, material balance
  bar, board-skin selector (green/blue/wood/classic, persisted in
  `aurago.desktop.chess.boardSkin`), move-evaluation panel, hint engine
  (Stockfish), move-history click-to-review with first/prev/next/last scrubber,
  last-move and check highlights, resign/draw confirmation modals with cancel,
  game-over result overlay with win confetti, CSS move/capture/check/thinking
  effects (`prefers-reduced-motion` aware), and Web Audio synthesized
  move/capture/check/castle/promote/game-over sounds. Split across `chess.js`
  (core game loop), `chess-fx.js` (template, effects, audio, skin helpers and
  the promotion dialog), `chess-engine.js` (Stockfish worker bridge),
  `chess-agent.js` (AuraGo agent move API client) and `chess-3d.js` (the
  optional 3D board).
- `chess-3d.js` owns the optional 3D board mode of the Chess app. It renders
  the live position with the original AuraGo piece kit from `/3d/chess/v1/`
  through the vendored global Three.js, GLTFLoader and OrbitControls, and
  exposes `window.createChess3D` plus `window.ChessView3D` (view switch, sync
  and disposal). The module never owns game state: `chess.js` owns the
  position, the turn, the board skin and the orientation, and the 3D view
  mirrors the snapshot it is given. Piece reconciliation keeps pieces that
  still match their square and code, retires every other previous piece by
  entry identity (a taken piece shares its target square with its captor, so a
  square-keyed sweep leaks it), animates the mover onto its target and swaps a
  promoted pawn for the promoted piece. Player intent (select, move) is reported
  back to `chess.js`, which applies the move through the regular player-move
  path including the promotion dialog. The 2D board stays the default and
  stays mounted; switching modes hides one surface and shows the other, and
  the 3D view is disposed with the window. A missing WebGL context or a failed
  asset load shows a localized note instead of a blank canvas. Verify with
  `go test ./ui/ -run TestDesktopChess` and `node scripts/test-chess-assets.mjs`.
- Chess layout is one centered board column: the view switch and material score
  share the pane head row, the board sits in a single square box, and the status
  ribbon closes the column below it. The head and the ribbon are capped by
  `--chess-column`. The board box fits the pane height (`container-type: size` on
  the shell plus `100cqh` in the frame width), so a tall window gets a bigger
  board and a short one never clips it. The 2D board is that square box; the 3D
  board is a perspective scene and takes the whole board area
  (`.vd-chess.is-3d`), so a wide pane lets it spread into the free width. The
  right sidebar widens with the window (`clamp(340px, 22vw, 420px)`), folds its
  settings away into a disclosure, pairs the open fields in two columns and
  scrolls instead of clipping when the window gets short. The panels carry their
  own display rule, so the tab switch needs the explicit `[hidden]` reset.
  Never let a compact row stretch across the full pane and never size the board
  from the browser viewport (`vh`).
- `writer.js` owns Autor's native DOCX document, menus and lifecycle;
  `writer-session.js` owns revision-bound autosave and IndexedDB recovery;
  `writer-panels.js` owns formatting, search, review, AI and snapshot printing.
  The Apache-2.0 core and local fonts are pinned to 2.29.0 and generated by
  `scripts/build-writer-vendor.js`. Do not add Pro packages or Quill to Autor.
  Opening a DOCX that a Writer window already shows (Files, the agent's
  `open_in_app`) focuses it and calls `reloadIfChanged()`: the window reloads
  when the file's ETag moved since load or the last save, and unsaved edits are
  never replaced (conflict notice with Save As and reopen instead). Markdown/HTML/TXT open as a new
  import copy, so their windows never match a re-open. Verify
  `TestDesktopWriterReloadsChangedFileOnReopen` (`AURAGO_RUN_BROWSER_SMOKE=1`).
- The Calendar is three continuations inside the shared Desktop IIFE, bundled
  in this order after `core/media-keys-runtime.js`: `calendar-views.js` (date
  math, `Intl` formatting, pure HTML renderers for month grid, week/day time
  grid, agenda, mini month, sidebar agenda and skeletons), `calendar-editor.js`
  (appointment editor dialog, quick-peek popover, recurring creation, contact
  participant picker) and `calendar.js` (per-window session state, incremental
  `paintCalendar`, navigation, keyboard, drag/drop, snackbar with undo, window
  menus and cleanup). None of them is loaded lazily. Styles live in
  `ui/css/desktop-app-calendar.css`, registered for `calendar` in
  `module-loader.js`; `desktop-app-planning.css` owns only Todo and the music
  player and must not regain `.vd-calendar-*` rules.
- Calendar contracts: views are `day`, `week`, `month`, `agenda`; view,
  sidebar and completed/cancelled filters persist in localStorage
  `aurago.desktop.calendar.prefs`. Rendering is incremental per session (no
  full shell re-render); loading uses skeletons. A newer list request wins,
  and an older response does not paint. While the document is visible, the
  open calendar reloads at the same 60-second cadence as the current-time
  line. The first load error shows `desktop.load_failed` and a retry button
  only. Reschedule (drag/drop, 15-minute snap in time columns, time kept on
  month cells), status changes and deletes are optimistic with rollback and
  an inline `[data-cal-snackbar]` undo; they never call
  `showDesktopNotification`. Delete commits when the undo snack ends, and
  undo keeps the same appointment id. Overdue is server-derived: moving an
  overdue appointment into the future sends `status: 'upcoming'`. The agenda
  keeps overdue days that are before today. Reminder presets map to
  `notification_at`; enabling "wake the agent" forces a reminder. Repeat
  create sends one `appointments` array. The server inserts 1 to 30 rows in
  one transaction. A single object POST is unchanged. A read-only desktop
  disables create, drag, status and delete, and shows `desktop.cal_readonly`.
  Single-click on a day/slot opens the editor; clicking an event opens the
  peek popover inside `.vd-calendar-shell`. Keyboard: `T`/`Home` today,
  `N` new, `D`/`W`/`M`/`A` views, `/` search, arrows/PageUp/PageDown
  navigate, arrows on month cells move the roving selection, `Esc` clears
  search. Searching switches to the agenda view and restores the previous
  view when cleared. All shell clicks go through one delegated handler in
  `wireCalendarShell` that matches `data-cal-*` selectors with `closest`, so
  the shell element itself must never carry one of those attributes: the
  active view is exposed as `data-cal-mode` and `data-cal-view` belongs to
  the toolbar tabs only. The editor is appended to `document.body` as a
  modal; a document-level capture `keydown` closes it on `Escape` regardless
  of focus and is removed in `closeCalendarEditor`. Closing a window
  disposes only that session's editor. The clock popup matches the local
  calendar day. The stylesheet keeps `[hidden]` authoritative inside
  `.vd-calendar-shell`, `.vd-calendar-editor` and `.vd-calendar-peek`
  because several fields use flex/grid classes. Verify with
  `go test ./ui -run TestDesktopCalendar` (the browser flow needs
  `AURAGO_RUN_BROWSER_SMOKE=1`).
- `cheater*.js` implements the Cheater app, a cheat-sheet manager with a
  textarea-based Markdown editor, live preview, Markdown toolbar, command
  palette (spotlight), and attachments side panel.
- `sheets.js` owns Tabellen's Autor-style chrome, document lifecycle and native
  Univer OSS 1.0.4 instance. `sheets-data.js` owns localized inputs, CSV and
  templates; `sheets-panels.js` owns formatting, data, search, AI and print;
  `sheets-charts.js` renders five editable Chart.js chart types on the sheet.
- `code-studio/*.js` implements Code Studio, a full IDE with file explorer,
  CodeMirror editor, terminal, search, agent chat, Git integration, a synced
  split editor, and keyboard shortcuts. Split across `core.js`
  (state management, API client, lifecycle, shell, tabs, status bar, dialogs,
  shared context menu), `sidebar.js` (keyboard-navigable file tree, path
  crumbs, tree context menu), `editor.js` (CodeMirror/textarea views),
  `terminal.js` (xterm.js sessions, ResizeObserver fit), `search.js`
  (search-in-files), `agent.js` (agent chat, markdown, diff preview),
  `git.js` (Git panel, closable diff view, commit), `panels.js` (two synced
  split panes), `shortcuts.js` (keyboard shortcuts, command table,
  window.CodeStudioApp), and `command-palette.js` (separate IIFE).
- `sysworld*.js` implements System World's Blender-authored data metropolis:
  seven fixed districts, live read-only inspector, entity search, map, explicit
  tour and street-level WASD/touch exploration. The isolated Three.js 0.186.1
  ESM renderer does not replace the legacy global used by other apps. The
  moon is a sky-dome disc, never a billboard quad. Opens
  maximized; existing dashboard/KG/mission APIs and shared SSE supply live data.
  Persistent 24-hour history remains a subsequent stage in
  `documentation/system-world-plan.md`.
- `looper.js` plus `looper-monitor.js` implement Looper v2. See the Looper
  contract in the Child DOX Index.
- `game-maker-studio-api.js`, `game-maker-studio-preview.js`,
  `game-maker-studio-modals.js`, and `game-maker-studio.js` implement Game
  Maker Studio: a project library, 2D/3D creation dialog with idea chips,
  bounded agent progress (job banner with phase, elapsed time, and repair-pass
  indicator, a phase stepper, and a job feed under the steps outside both panes), result cards for ready/failed jobs,
  revision history, change requests, and a live game preview in one maximized
  desktop window.
- `live-speech.js` mounts the shared realtime-speech panel on the desktop in a
  compact window (preset 440×520, min 340×460 in
  `window-shell-runtime.js`; panel mounted with `compact: true`).
  The desktop header's speaker control lives beside the FX toggle and opens
  the shared audio settings dialog; pass its `[data-live-speech-audio-controls]`
  host through the mount `audioControls` option.
  The shared panel places its animated persona beside wrapping, scrollable
  captions; small windows scroll vertically. Avatar disposal uses the existing
  panel unmount and preserves `keepSession`. The decorative FX remain separate
  from the avatar and never control its mouth from microphone levels.
  OpenAI, xAI, and Gemini stay on their existing streaming adapters. Speech
  Lab is a keyless `local_s2s` profile that transcribes and speaks through
  the managed or external s2s container. The app shows `/api/speech-lab/status`
  and may start a managed container through `/api/speech-lab/deployment/start`.
  `live-speech-fx.js` loads before `live-speech.js` and renders the
  audio-reactive background canvas (`window.LiveSpeechFX.create`, per-window
  instance with `setEnabled`/`dispose`, FX toggle persisted under
  `aurago.desktop.livespeech.fx`). Reactivity comes from the runtime `level`
  event (mic RMS emitted by `RealtimeAudioGate`) and the optional adapter
  output taps `getOutputLevel()` / `getOutputSpectrum()` (PCMPlayer for
  Gemini/xAI, zero-gain MediaStream tap for OpenAI, MediaElement tap for
  Speech Lab); the FX must keep its pooled particles, DPR cap, and
  `prefers-reduced-motion` static fallback.
- `sip-phone.js` implements the Phone app, an iPhone-inspired SIP softphone
  rendered as a realistic device (brushed titanium frame, separate mute/
  volume/power hardware buttons, glossy Dynamic Island, live status-bar
  clock, signal/battery indicators, glass screen glare, aurora mesh
  wallpaper) on an ambient stage with a light halo and floor shadow. The
  screen hosts five tab views (Favorites, Recents, Contacts, Keypad, Settings)
  above a glass tab bar, plus a full-screen active-call takeover with
  contact-hue avatars and incoming-call answer/decline actions. The Contacts
  tab lists AuraGo address book entries (`/api/contacts`) that carry a phone or
  mobile number, one tap-to-dial row per number with a client-side search
  filter. The app can also run
  windowless as a floating desktop gadget (see `sip-phone-gadget-runtime.js`
  in the Child DOX Index).
- `pixel*.js` implements the Pixel image editor. Its module structure, contracts, file ownership and test commands are in
  `## Pixel` at the top of this file. The sticker contracts below still apply to Pixel sticker layers.

## Ownership

Owned by this subtree. Backend integration lives in `internal/server/` and app
registration lives in `internal/desktop/types.go`.

## Local Contracts

- MeshCore loads `meshcore-device.js` before `meshcore.js`. The companion module
  owns Device/Settings pages and reception/contact details. Settings use topic
  navigation (a native select in narrow windows); inactive forms stay mounted so
  drafts survive section/page changes and refresh. Show disabled reasons and
  readback conflicts/partial results; keep uncertain-state reconciliation visible
  above every section. Radio edits require a before/after confirmation. The Device
  overview separates identity, capacity, radio/features and timestamped diagnostics;
  unavailable values remain unknown. Local diagnostics poll only on the visible
  Device page, at least 30 seconds apart; remote diagnostics start only from
  explicit contact actions and poll bounded server jobs. Settings drafts and
  diagnostics never enter browser storage. Keep device flags/favorites distinct
  from Messenger favorites and agent trust. Dispose timers, requests and dialogs;
  verify both themes and narrow windows with `TestDesktopMeshCoreDeviceBrowser`.
  Firmware/API/privacy contracts are owned by `internal/meshcore/AGENTS.md`.

- Agent Chat displays transient typed `llm_route` metadata through the shared
  `AuraLLMRouteBadge` helper. Update the originating turn's badge on fallback,
  render model/provider strings as text, and keep metadata out of answer bubbles
  and persisted chat history. Runtime routing is owned by `internal/agent`.

- Built-in app load order is defined in `ui/js/desktop/core/module-loader.js`.
- `calendar-views.js`, `calendar-editor.js` and `calendar.js` are the Calendar
  source of truth. `desktopMainParts` in `scripts/build-ui-bundles.js` must
  keep exactly that order, after `core/media-keys-runtime.js` and before
  `core/menus-and-routing.js`, so `renderCalendar` and its helpers stay inside
  the shared Desktop runtime closure without duplication.
- Game Maker Studio loads in the order `game-maker-studio-api.js`,
  `game-maker-studio-activity.js` (`window.GameMakerStudioActivity`: per-window
  background progress terminal),
  `game-maker-studio-preview.js` (`window.GameMakerStudioPreview`: loading
  overlay, stale badge, fullscreen, new-tab), `game-maker-studio-modals.js`
  (`window.GameMakerStudioModals`: shared modal lifecycle, media toggles,
  skills and revisions; the revisions list shows `changed_paths`, at most 40,
  and `game_maker.revision_paths_truncated` when `truncated` is set;
  confirmation stays shell-mediated),
  `game-maker-studio-assets.js` (`window.GameMakerStudioAssets`: offline pack
  catalog, selection, sprite/animation previews), then
  `game-maker-studio.js`.
- Game Maker Studio lays out by container, not by viewport: `.gm-studio` is the
  `gm-studio` inline-size container and `.gm-preview-pane` the `gm-preview`
  container. Every width rule exists as `@container` and as its `@media` twin,
  because fixtures render panes outside the shell; change both together. Below
  760px both rules show `[data-gm-steps]` and give `.gm-workspace` one column.
  The step bar is row 1, `[data-gm-job-feed]` is row 2, and the panes share
  row 3. A hidden feed uses `display: none`, so its row collapses. On a wide
  grid the feed spans both columns as a content-sized first row; the panes
  stay on the flexible second row so the conversation keeps most of the agent pane. `.gm-studio` starts at `data-gm-step="agent"`. That
  step hides `.gm-preview-pane`; `preview` hides `.gm-agent-pane`. The bar uses
  `display:none` above 760px, not the `hidden` attribute, and those pane rules
  stay inside the 760px queries so a wide window keeps both panes. The buttons
  are `game_maker.step_agent` and `game_maker.step_preview`. `playNow` selects
  the preview. A phase event that enters `validating` selects it once, as does
  opening a project whose active job is already validating. A repeated
  validating event does not return there after the visible bar's
  `confirmLeaveValidating` confirmation. Below 980px the library is hidden, so
  the project select and `.gm-narrow-new` in the agent pane head are the only
  way to switch or create. Keep the library button
  the first `[data-gm-action="new"]` in document order and update all of them in
  `applyCapabilities`. Only `.gm-conversation` may stretch in the agent pane;
  banner and notice come and go. Dialogs size against the Studio window, pin
  header and footer with offsets of `-var(--gm-modal-pad)`, and `modalError`
  inserts above the footer. Signal colours use the `--gm-tone-*` variables, never
  fixed light tints. Failure cards show `game_maker.failure_hint_<cause>` and
  keep the backend text inside `details`. Use `error_code` from the job when
  it is present; English substring matching remains only for older jobs that
  have no code. A plan card (`plan_summary`) and a check-result card render
  from `latest_job` so a reload does not depend on the SSE backlog. Omit the
  check line unless both counts are present, and omit repair passes at nil or
  0. When `usage.calls` is present, success and failure cards add one usage
  line from that object. Omit it when calls are missing or zero. Tokens and
  price use the localized unknown phrase when the field is absent. Do not add
  a second usage stream in the client. Library states use `game_maker.status_<status>`. Verify with
  `TestGameMakerStudioLayoutBrowser` (`GAMEMAKER_STUDIO_REPORTS` writes
  screenshots).
- Model qualification and continuation actions belong to the start and retry
  dialogs plus the failure card. Both dialogs show the passive
  `GET /api/game-maker/model-limits` line (`game_maker.model_limits_line`) under
  the provider/model fields, warn with `game_maker.model_limits_warning` when a
  known route reports `below_minimum`, and mark the field group with
  `data-gm-model-state[data-below-minimum]`. The probe is debounced (350 ms),
  ignores late responses and degrades to "unknown" on failure; only an explicit
  confirmation sends `allow_small_model: true`, and a server
  `model_context_too_small` answer replaces the dialog error with
  `game_maker.small_model_blocked`. When the same response reports `small_model`,
  both dialogs show `game_maker.compact_mode_title` and
  `game_maker.compact_mode_body`. That flag is separate from `below_minimum`:
  a route can be compact while it still clears the 16384 context floor.
  The failure card renders translated help for `latest_job.open_checks` (at most
  six). `required_controls` and `required_stages` use their own strings; any
  other id uses `game_maker.check_help_generic`. A missing translation falls
  back to that generic string. `data-gm-apply-check` writes the matching
  `game_maker.check_prompt_*` into the change field and does not start a job.
  Expected and observed values stay in the collapsed technical details, off the headline. The model-facing
  `next_action` stays inside the technical `details` as
  `data-gm-model-instruction`. The card offers
  `data-gm-retry-model` (resume with the chosen provider/model, which becomes the
  project default) and `data-gm-extend-repairs` only for a failed job whose
  `repair_passes` is above zero and below six, carrying `used + 3` as
  `max_repair_passes`. Both resume with empty asset arrays, and the card says
  so with `game_maker.retry_keeps_bindings`. They never reset unrelated fields.
  Elapsed time reads `started_at` from the job or the active-job capability and
  falls back to the local clock only when that time is missing.
  Verify `TestGameMakerFailureCardBrowser`.
- Game Maker Studio exposes `window.GameMakerStudioApp = { render, dispose,
  instances }`. Every window owns and closes its EventSource, preview iframe,
  channel ID, diagnostics, modal handlers, job-elapsed and busy-poll timers,
  document-level overflow-menu listeners, and `message` listener.
- Voxel is the third creation choice (`dimension: 3d, variant: voxel`), with
  localized ideas/badges/capabilities. These fields stay immutable after creation.
  `game-maker-studio-preview.js` forwards only load/save/reset from the exact
  current frame/channel/project/published revision, using a parent-only grant.
  Reject oversized/unknown messages; expose conflicts and keep drafts/validation
  temporary. Flush before restore mutations and before project/frame replacement;
  failed flushes require explicit discard through the existing shell confirmation.
  External published revisions must pass that same resolution before replacing a
  Voxel frame. Restore controls are disabled with the localized read-only reason
  when `capabilities.allow_edit` is false.
- Voxel new-window play uses verified `game-maker-player.html` and
  `game-maker-player.js` with the same bridge and opaque iframe. Keep credentials
  out of iframe messages, source URLs and exported games. A play-state GET 409
  marks the revision-bound grant stale; recovery refreshes the trusted host on
  initial-load races and the explicit Load latest action. Same-revision save CAS
  conflicts still recover in place through Load latest. Verify
  `TestGameMakerVoxelBridgeBrowser` alongside the existing Game Maker UI tests.
- Scene diagnostics use the current preview channel and parent-window binding.
  The toggle exposes boundaries, colliders, IDs and routes only in Studio; ZIP
  exports never enable the overlay. Keep rule evidence separate from startup
  status and display unverified custom gameplay honestly. The shared scene and
  mechanics helpers use the existing engine loop, pause and disposal owners.
  Verify with `GAMEMAKER_BUILDER_BROWSER=1` and `TestBuilderBrowser`, including
  actual inputs, repeated resource counts, exported games and narrow layouts.
- Each project remembers its last asset selection in this browser
  (`localStorage` key `aurago.game-maker.asset-selection`, with an in-memory
  fallback). The selection line also says `game_maker.assets_browser_only`:
  it applies only in this browser and only to an ordinary change. The next
  change request sends that selection again until the user
  clears it. Try again sends empty `asset_pack_ids`, `model_asset_ids` and
  `asset_selections`, and no presentation, so an accepted binding stays. An
  empty selection lets the agent choose. Selecting packs never starts a job.
  A new-game dialog holds the open project's selection until create succeeds
  or the dialog is cancelled. Opening the asset browser from that dialog
  stores the draft on the open project when the browser closes. The asset
  browser still owns its AbortController and animation timer, released on
  modal replacement/close and disposal. Metadata
  stays English for agents; controls and pack titles cover all 16 locales.
  Preview backgrounds are CSS only: PNGs contain genuine alpha, no painted
  checkerboard.
- Modular pack previews default to a complete assembly. Parts use metadata
  coordinates and shared animation time; selecting an individual sprite exits
  assembly mode. Reuse the window's existing preview timer and cleanup path.
- Game Maker previews must use `sandbox="allow-scripts allow-pointer-lock"` without
  `allow-same-origin` (`allowfullscreen` on the iframe is permitted).
  Preview HTTP responses also send CSP `sandbox allow-scripts allow-pointer-lock` so "open in
  new tab" cannot become a first-party AuraGo origin.
  Accept diagnostics only from the instance iframe when `event.source`, the
  random channel ID, the fixed source marker, and the bounded event type all
  match. Forward bounded, deduplicated reports through the authenticated
  `preview-report` API using the parent-held preview token; never grant the
  iframe API credentials. Reports bind to a specific validation build. Forward
  at most five sanitized dist/game.js line/column frames; strip other fields,
  paths and invalid coordinates. The backend owns source mapping. Verify with
  `TestGameMakerDiagnosticFramesBrowser` and `AURAGO_RUN_BROWSER_SMOKE=1`.
  Include current-preview errors as untrusted diagnostics in the next change request.
  Readiness requires the injected boot's `boot: true, visible: true` report;
  game-authored ready messages alone never qualify browser validation.
  Full gameplay grants contain bounded scenarios. Send them only to this iframe;
  forward at most 16 observations and two 700,000-character PNG data URLs through
  the existing authenticated parent report route. Never accept JavaScript test
  expressions or an iframe-authored pass verdict. Validation and advisory image
  results use localized SSE activity; planning/model deltas stay internal until
  the server publishes a verified revision.
  `game-maker-studio-preview.js` owns the message bridge and its report limits;
  the app supplies its instance state and diagnostic callback.
  Clear the visible diagnostics, error badge, and next-request diagnostics when
  replacing a preview or opening a project. Late responses from an old preview
  must not repopulate them; a ready message never clears current-run errors.
  Stop validation reports when the grant expires or its job reaches a terminal
  state, including late API failures. Published-game runtime diagnostics remain
  visible locally.
- Because the preview sandbox is opaque, game diagnostics must
  `postMessage(..., "*")` (never `location.origin`, which is the string
  `"null"`). The parent still validates source/channel/`event.source`.
  Arm the loading overlay before assigning `iframe.src`, clear it on
  `ready` or iframe `load`, and rely on the server-injected preview boot
  script to hide leftover in-game `Loading…` HUD pills for agent-rewritten
  `index.html` files.
- Game Maker job progress UX: the job banner, phase stepper, and result
  cards are driven by the project-scoped SSE stream; `capabilities.active_job`
  marks the single globally running job (library spinner, disabled change
  form plus hint in other projects) and is polled only while another project
  is busy.
- The preview placeholder's adventure terminal consumes only public phase,
  allowlisted tool names, file/asset updates and check status events. Never feed
  it model deltas, reasoning, arguments or diagnostic bodies. Render values as
  bounded text, deduplicate replay IDs and scope them to the current job/project.
  Keep the existing icon/copy stationary. Limit pending lines to 32 and DOM rows
  to 24; suspend typing while hidden, offscreen, inactive or showing a game iframe.
  Reduced motion reveals complete lines without a blinking cursor. Project
  changes and disposal clear its timeout and history; disposal also disconnects
  intersection/resize observers and visibility/media listeners.
  Verify with `TestGameMakerActivityTerminalBrowser`; optional screenshots use
  `AURAGO_GAME_TERMINAL_SCREENSHOTS` under ignored `reports/`.
- Allowlisted `model_progress` statuses show waiting, receiving, retry and code
  recovery without exposing model text. Failed/cancelled runs retain a static
  terminal transcript with no animation timer. Recreate the activity controller
  whenever deleting a project replaces the shell, so later jobs target the new
  preview node. Keep all 16 desktop locales aligned.
- Render terminal text at 16px (12px on narrow views) in local Press Start 2P with full opacity,
  warm yellow fill and a narrow dark outline. Do not use gray fill, white outlines, text strokes or scanlines
  over glyphs; keep the stage-card fade localized so surrounding lines stay legible.
- EventSource open restores status even without replayed events; stale callbacks
  cannot affect another project or disposed window. The server ends the persisted
  backlog with `event: stream_ready` (no id). Until that marker, keep the latest
  `job_status` or `phase` and any gameplay status, and do not append historical
  file, asset, diagnostic, validation, preview or revision activity. After the
  marker, events apply live. A native reconnect keeps its cursor and does not
  treat new events as backlog. Terminal job status survives reconnects and
  project refreshes. Cancellation shows its server reason and retry action, with
  no active phase. Verify with `TestGameMakerEventsReconnectBrowser`.
- The ready-job line `game_maker.gameplay_unverified` is shown only when
  `gameplay_status` is `unverified` on a non-voxel 3D project.
- A non-empty workspace drift draws a badge on the preview and a hint under
  the change form. Preview replacement keeps that badge in the shell. The
  badge action `adopt` confirms, then posts `adopt_workspace: true` with no
  prompt and no model. Voxel adopt flushes through `preparePreviewReplacement`
  first and stops when that returns false. The badge and hint stay hidden
  when the drift payload is not three arrays or the count is zero. The adopt
  action also stays hidden while a job is active or editing is off. While
  phase is `validating`, the banner tells the user to keep this project's
  preview visible. Switching
  projects or closing the window confirms first; declining leaves the project
  open. The sources button keeps `data-gm-action="skills"` and the dialog says
  it does not turn generation on or off.
- Game Maker retry buttons submit `resume: true`,
  `validate_restored_draft: true` and an empty `prompt` directly through the
  existing job-start flow, even after reopening the app. The server stores the
  continuation sentence; the button label is not a user message. Preserve unsent
  editor text, lock immediately against duplicate requests, and retain normal
  permission and global-job gates. Private reasoning stays server-side, outside
  the progress terminal and chat. Load, rename and request failures stay in the
  diagnostics and notice; they do not set the job status to failed.
- Game Maker visible strings use `game_maker.*` plus
  `desktop.app_game_maker_studio` in all 16 `ui/lang/desktop/*.json` files.
  Missing skills/revisions modals throw `game_maker.modules_load_failed`.
  Destructive actions use shell-provided dialogs and never native browser
  dialogs.
- Galaxa modules attach to the shared `window.GalaxaCore` (GC) namespace and
  expose `GC.create<Name>(ctx)` factories that augment a per-instance `ctx`
  created via `Object.create(GC)` in `galaxa-deluxe.js`.
- Galaxa load order is defined under the `galaxa-deluxe` entry.
  `galaxa-constants.js` and `galaxa-tweens.js` must load before factory modules.
  Split modules (soft budget **≤1000 lines** per file): `galaxa-entities-{core,
  spawning, behaviors, combat, weapons}.js`, `galaxa-render-{effects, stage, hud,
  world}.js`, `galaxa-audio-{core, sfx, music}.js`, `galaxa-enemy-motion.js`,
  `galaxa-enemy-fx.js`.
  Orchestrator glue stays in `galaxa-entities.js`, `galaxa-render.js`, and
  `galaxa-audio.js`.
- `galaxa-campaign.js` owns seeded run randomness, the pause-aware game
  scheduler, 30-stage sector waves and six sector-boss encounters. Classic,
  Mirror and Daily use `GC.getStagePlan`; Boss Rush uses those six bosses.
  Gauntlet remains twelve waves; Endless and Hyperdrive retain their spawners.
  Sector bosses use `sectorBoss` and three HP phases; legacy `boss` enemies
  remain formation commanders. Route all damage through `ctx.damageEnemy`
  and finalize sector-boss rewards/destruction once in `updateCampaign`.
- `galaxa-sprites.js` preloads and validates local `img/galaxa/atlas.json`
  and PNG sheets before the title screen. All atlas requests use the page's
  BuildVersion; fixed art revisions fail the resource server's version check.
  Verify with `node scripts/test-galaxa-assets.mjs`. Animation rectangles, durations,
  loops and anchors belong in that manifest; never restore inline pixel
  definitions or fixed frame counts. Collision geometry is independent of art.
  Asteroid animations opt into `blendFrames`; adjacent anchored frames, including
  the loop boundary, blend on one reusable scratch canvas without adding phases
  to the sprite cache. Preserve opacity and nearest-neighbor scaling. Verify
  `TestGalaxaAsteroidAnimationBrowser` for continuity, Reduced Motion and cleanup.
  Sprite canvases have a 768-entry cap. Asset failures expose localized retry.
- Galaxa background order is sector scenery, stars, planets, then gameplay/HUD.
  Planet frame blending uses one reusable logical-size canvas and preserves body
  opacity; dim the colors rather than the alpha so stars cannot show through.
  Keep the two-sheet limit and verify `TestGalaxaPlanetOcclusionBrowser`.
- `ctx.stepFrame` runs a fixed 60 Hz simulation independently of rendering.
  Use `ctx.scheduleGame`, not wall-clock timers, for combat transitions.
  `resetRun` resets the run generation and seeded RNG; delayed work cannot
  cross run/disposal boundaries. Desktop `isActive` gates all controls;
  focus loss pauses and releases held input. The 540x720 field fits the
  available viewport at DPR 1/2 without cropping; touch has Parry/Super buttons.
- New gameplay audio is synthesized. Separate music/SFX buses feed the
  master compressor; SFX have a 32-voice priority cap and cached noise.
  Music is scheduled against the audio clock with 120 ms lookahead and
  bar-boundary transitions. Keep `img/audio/galaga.mp3` unchanged for title
  and demo, and stop it before synthesized gameplay. Persist volume zero.
- Arcade acceptance: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run
  'Galaxa|AdaptiveMusic' -count=1`; the 30-minute real-time browser run also
  needs `AURAGO_GALAXA_SOAK_SECONDS=1800` and Go `-timeout 35m`.
  `AURAGO_BROWSER_ARTIFACT_DIR` optionally saves screenshots/audio/metrics.
- Enemy movement visuals live in `galaxa-enemy-motion.js` with per-type presets
  in `GC.ENEMY_MOTION_FX` (pulse scale, dive trails, blink invisibility).
- Enemy-side effects live in `galaxa-enemy-fx.js` (`GC.createEnemyFx`, called from
  `galaxa-entities.js`): charge-up telegraph glints, feint/formation-shuffle blink
  rings, rage auras, rapid-kill lightning chains, per-hit impact pops, plus the
  smoldering crash-dive state (`SMOLDER`, tuned by `GC.SMOLDER`): hit enemies
  that tumble toward the player trailing smoke/embers and detonate after a seeded
  delay. It is
  drawn by `ctx.drawEnemyFxWorld(c)` between `fxDrawWorld` and `renderDangers`, and
  ticked by `ctx.updateEnemyFx(dt)` from `updateExp`. Pools are bounded by
  `GC.FX_CAPS` (`telegraph`, `enemyMarks`, `chain`, `smolder`) via `ctx.settings.particles`,
  honour `ctx.settings.reducedMotion`/`prefers-reduced-motion`, and are cleared
  whenever the game leaves `PLAYING`/`STAGE_INTRO` so no mark freezes over menus.
- Enemy unpredictability (quirks, feints, squad/revenge dives, dive path
  profiles) lives in `galaxa-entities-behaviors.js` with tuning in
  `galaxa-constants.js` (`GC.ENEMY_QUIRKS`, `GC.QUIRK_CHANCE`, `GC.FEINT`,
  `GC.SQUAD_DIVE`, `GC.REVENGE_DIVE`, `GC.FORMATION_SHUFFLE`, `GC.DIVE_PATHS`).
  Quirks are assigned at both spawn sites through `ctx.assignEnemyQuirk` and are
  disabled on easy difficulty. The new `FEINT` state aborts a dive and can never
  kill by body contact. Every gameplay decision (quirk roll, dive path, squad and
  revenge picks) must use the seeded `ctx.random`; only visual randomness may use
  `Math.random`, so fixed-step runs stay deterministic. Revenge dives are
  reported by `ctx.onEnemyKilled` from `registerKill`; rapid kills also spawn
  `ctx.fxKillChain`, and `damageEnemy` spawns the hit pop once per damage window.
- Weapon power-ups (rare/legendary): `rocket_launcher` / `mega_rocket` (homing
  salvo), `mine_layer` / `mega_mine_layer` (player mines via `ctx.pushPlayerMine`,
  cap `GC.PLAYER_MINE_MAX`), and instant `megabomb`. Mirror shots use
  `ctx.mirrorDuplicateBullets(fromIdx)` for rockets and normal fire. SFX:
  `rocketLaunch`, `rocketHit`, `mineDrop`, `mineExplode`, `megabomb`.
  `collectPU` tracks every pickup in `collectedPU` at entry for `power_collector`.
- Game mode logic lives in `galaxa-modes.js` (`GC.createModes(ctx)`): `gauntlet`
  (12 curated waves, no shop, 3 lives), `hyperdrive` (endless speed ramp +
  rotating modifiers; spawning/HP scaling matches `endless`), `mirror`
  (permanent horizontal mirror + 50% ghost damage; delayed enemy bullets mirror
  on spawn). Export `ctx.modesRestoreTimeScale()` for hitstop/slow-mo/continue/
  bullet-time recovery. Settings `mode` cycle plays `modeSelect` once via
  `GC.applySettingsInput`; daily challenge remains title hotkey `D`.
- Adaptive biome layers in `galaxa-adaptive-music.js` attach to the active base
  theme from `ctx.modesGetBaseMusicTheme`, not hardcoded `gameplay`.
- New Galaxa constants (biomes, super defs, parry tuning, explosion profiles)
  are added to `galaxa-constants.js`, not duplicated in game logic files.
- Galaxa visible UI strings use `galaxa.*` keys in all
  `ui/lang/desktop/*.json` files and must not rely on inline fallback text.
- Chess exposes `window.ChessApp = { render, dispose }`; every desktop window
  instance must own and clean up its own `chess.js` game, `cm-chessboard`
  board, Stockfish worker, Agent client, event handlers, and pending search
  token state.
- Chess loads `ui/js/vendor/chess-vendor.esm.js` with dynamic `import()` from
  `chess.js`; the lazy loader remains classic-script based.
- Chess engine code must load Stockfish only from
  `/js/vendor/stockfish/stockfish-19-lite-single.js` and browser-side agent
  moves must call `/api/desktop/chess/agent-move`.
- Chess visible UI strings use `desktop.*` keys in all
  `ui/lang/desktop/*.json` files.
- Cheater exposes `window.CheaterApp = { render, dispose, openSheet,
  openCreateModal, formatRelativeShort }`; every desktop window instance owns
  its own save debounce timer, preview debounce timer, polling timer, and
  AbortController for in-flight saves.
- Cheater editor uses a stable `<textarea>` source (NOT `contenteditable`) so
  cursor, selection, and native undo stay intact. Live preview is rendered into
  a separate `.cheater-preview` panel via `window.marked`, sanitized with
  `window.DOMPurify`, and highlighted with `window.hljs`.
- Cheater view modes (`edit`/`split`/`preview`) are persisted per-window in
  `localStorage` under `cheater.viewMode`.
- Cheater toolbar is a separate `cheater-toolbar.js` module exposing
  `window.CheaterToolbar.mount(state, slot)`; toolbar buttons use
  `textarea.setRangeText` to stay caret-safe. Do not inline the toolbar into
  `cheater.js`.
- Cheater visible UI strings use `cheater.*` keys in all
  `ui/lang/desktop/*.json` files.
- Cheater tags are persisted through the `/api/cheatsheets` JSON API and must
  remain part of list normalization, creation, search, and card rendering.
- Cheater attachment uploads use `multipart/form-data` to
  `/api/cheatsheets/{id}/attachments`; client validation stays aligned with
  backend limits: `.txt`/`.md`, 1 MiB upload size, and 25,000 text characters
  per sheet.
- Sheets exposes `window.SheetsApp = { render, dispose, instances }`. Each
  window owns its engine/worker, requests, panels, chart canvases, operation
  journal and save queue. Load OfficeSession, data, panels and charts before
  sheets.js. The local vendor build imports only Apache-2.0 Univer OSS 1.0.4.
- Load `sheets-images.js` after charts and before `sheets.js`.
  `SheetsImages.create(state)` owns per-window image nodes in the existing charts
  overlay. `aux.images` stores ID, sheet ID/A1 anchor, offsets, dimensions, MIME,
  embedded data, alt text and readonly state on the same native auxiliary undo
  command as charts and print. Row/column operations transform/remove anchors in
  that transaction; sheet renames preserve stable IDs. Unsupported pictures stay
  readonly/opaque and unsafe unknown drawing references lock structural edits.
  Sheet copies give managed images new IDs and the copied sheet's ID in the
  native copy undo/redo transaction. Check aggregate image limits before copying;
  preserve existing chart references and reject unsupported structural copies.
- Sheets QR insertion captures document revision/signal and selected sheet/range
  before loading the shared dialog. Images preserve aspect ratio while dragging
  or resizing; arrows move one pixel, Shift+arrows resize four pixels, Delete
  removes. Existing snapshots, drafts, conditional writes, Save As, recovery and
  print retain image bytes; CSV warns about omitted images. Dispose removes image
  nodes and pointer listeners. Verify `TestDesktopSheetsImagesBrowser`,
  `TestDesktopSheetsQRDialogBrowser` (actual dialog/selection and XLSX QR decode) and the
  existing Sheets engine/app/recovery/versioned-assets browser checks.
- Resolve the engine import and formula worker through `AuraLazyAssets.versionedURL`
  so both match the desktop build, including after a cached pre-1.0 resource set.
  Verify `TestDesktopSheetsVersionedAssetsBrowser` alongside the app/recovery checks.
- Failed workbook loads release the editor/save queue and show New, Open and
  Retry actions with an error status. A confirmed 404 clears the shell's stored
  path; temporary failures retain it for retry. New uses a fresh create-only
  path and never recreates the missing file. Verify `TestDesktopSheetsLoadRecoveryBrowser`.
- Use the engine's formula, selection, clipboard, structural-reference and undo
  APIs. Do not restore the removed HTML grid or browser formula evaluator.
  Expand shared formulas before persistence, preserve forced strings, and parse
  native edits through the same locale-aware path as the formula bar.
- Current engine selection takes priority over a pinned toolbar fallback.
  Await clipboard commands; format disjoint selections as one native command.
  Formula recalculation events must not dirty the workbook.
- PATCH editor-v2 uses ETag preconditions and an original XLSX source. Keep the
  structure journal until its own save succeeds; stale responses cannot clear
  newer edits. Save As and recovery copies preserve the original source bytes.
  Shared OfficeSession autosaves at 800 ms / five seconds and owns IndexedDB
  drafts separately for Writer and Sheets. Dispose all document resources.
- Use native chart/print mutations on the same undo stack. Charts remain real
  OOXML charts; unsupported chart types and workbook references are preserved
  and guarded. AI only proposes bounded changes to the explicit selection;
  apply once through native undo, and reject stale revisions.
- The sheet stays light in every desktop theme. Use shared desktop menu/icons,
  16 desktop locales, local fonts and a <900px overlay inspector. Print/export
  captures current edits and waits for calculation, without moving selection.
- Writer exposes `window.WriterApp = { render, dispose, instances }`. Each window
  owns its editor, abort controllers, panels, draft and serial save queue.
  Autosave waits 800 ms, with a five-second ceiling during continuous typing.
  Acknowledgements cover only the captured revision. Suspend the queue during
  Save As and rename. Native writes require ETag preconditions. Save As probes
  the destination: a missing file uses `If-None-Match: *`, and an existing file
  is replaced only after confirmation with that file's `If-Match`. A failed copy
  retries the chosen destination. Rename sends `If-None-Match: *` on
  `PATCH /api/desktop/file` and then adopts the ETag from GET of the new path.
  An existing name stays put and uses `desktop.fm.paste_exists`. A conflict
  notice offers Save As and reopen; reopen confirms before discarding local
  edits. Retry stays for load failures and transient saves of the open file.
  A successful save clears conflict and save notices. Format paper recognizes
  Letter and A4 in both orientations; other sizes show a non-writing custom
  option. Format indent, hex color, highlight and image fields follow the
  selection. Keep the async close guard installed until cleanup and never
  replace failed loads with blank content.
- Writer Insert QR captures the native retained selection, document identity and
  package revision before lazy loading. Native image insertion requires a caret:
  collapse to the captured anchor and retain selected text. Pass generated PNG
  through the existing `insertImage`/native image command with requested physical
  millimetres converted to points independently of export pixels. Readonly and
  suggestion-mode restrictions apply again at insertion; document replacement
  closes the dialog. Keep native undo, serial saves, DOCX/print/export and recovery.
- Writer exposes New/Open after load failures and clears the persisted window
  path only on a confirmed 404. Transient errors retain it for Retry. Verify
  `TestDesktopFileLoadRecoveryBrowser` alongside the Sheets recovery check.
- Writer pointer selection must preserve the viewport, including clicks near its
  edges after toolbar focus. Core 2.29.0 supplies this behavior upstream; keep
  keyboard/programmatic reveal and drag edge autoscroll enabled. Retain the
  browser regressions when rebuilding or upgrading the core.
- Writer opens new, loaded and recovered documents at the first paragraph with
  the viewport at the top. Restore typing focus only in the active editable
  window. Verify `TestDesktopWriterAppBrowser` and the Writer shell matrix.
- Writer font-size controls display points and convert to integer half-points at
  the command boundary, including command availability checks. Reject invalid
  inputs before editing. Map app actions to existing shared icon keys; action IDs
  are not automatically icon names.
- Writer visible UI strings use `desktop.writer_*` keys in all
  `ui/lang/desktop/*.json` files. New keys require translations across all 16
  supported languages.
- Writer search uses the core's semantic matches and replace commands, preserving
  document formatting and undo. Review uses public EditorModule/store contracts.
  Structural commands without reliable tracked revisions stay disabled while
  tracking. AI suggestions bind to the source revision and require explicit apply.
  Print and exports serialize the current editor, never stale server content.
  Unknown package parts and authored font names must survive native saves.
- Code Studio exposes `window.CodeStudioApp = { render, dispose, state, instances,
  api, command, knownFiles, loadState, saveState, refreshFiles, openFile, openPath,
  openFileFromDialog, saveCurrentFile, uploadFile, downloadFile }`. All
  non-command-palette modules share a single IIFE closure; `core.js` opens the
  IIFE, `shortcuts.js` closes it. Function declarations are hoisted across the
  entire IIFE scope. All `const`/`let` declarations must stay in `core.js` (the
  first module in the bundle load order).
- The command palette drives Code Studio only through
  `CodeStudioApp.command(name, args)` (table in `shortcuts.js`) and lists tree
  files from `CodeStudioApp.knownFiles()`; never dispatch synthetic key events
  or click hidden buttons from the palette.
- Code Studio binds shared `state` only during synchronous work. Async handlers
  capture their instance, reject disposed/superseded requests, and explicitly
  bind UI updates after awaits. Never leave `state` switched while awaiting.
- Save, Save All and Run share a per-tab serialized save queue. A completion
  clears dirty state only for the saved buffer; later edits remain intact.
  Rename/delete wait for pending saves and block new saves until the path change
  completes. Run retains its document and terminal session and stops after a
  failed/superseded save or a removed/renamed document.
- Code Studio AI suggestions retain their window, tab identity, path, revision,
  range and action intent. Only a single code block from Refactor/Comments may
  replace source; Apply revalidates the original document and uses one editor
  transaction. Other output and stale suggestions stay copyable. Cancelled or
  superseded replies cannot change messages, busy state or pending suggestions.
- Code Studio receives the office app context (`officeAppContext` in
  `menus-and-routing.js`): `setWindowBeforeClose`, `showContextMenu`,
  `promptDialog`, `confirmDialog`, `notify`. Modified tabs block tab close,
  close-others/all and window close until the user confirms with
  `codeStudio.unsavedClosePrompt` / `unsavedTabsPrompt` / `unsavedWindowPrompt`.
- `.code-studio-body` and `.code-studio-main` are flex layouts; hidden
  sidebar/agent/git/terminal panels use `display: none`. Never model panel
  visibility with collapsed grid tracks (zen mode and the Git panel must keep
  the editor visible).
- Explorer rows are `role="treeitem"` with `tabindex="0"`: arrows, Home/End,
  Enter, F2 and Delete work; the active tab is highlighted; right-click opens
  the shared context menu. New file/folder/upload target `targetDirectory()`
  (the selected, expanded folder) and reload only that directory via
  `reloadTreeDirectory`. Typing `?` inside inputs or the editor must not open
  the shortcut overlay; Ctrl+= / Ctrl+- / Ctrl+0 and Ctrl+wheel zoom the editor.
- Split Right/Down renders two synced CodeMirror views of the active tab
  (`tab.view`, `tab.secondaryView`, `tab.views`); `renderEditor` re-applies the
  split on tab changes. `destroyTabView` retains editor state, selection and
  scroll while disposing view DOM/listeners; closing a tab releases snapshots.
  CodeMirror presentation is reconfigured through a compartment. Only the
  primary pane owns history; secondary undo/redo commands use that history.
  AI context uses the last focused pane's selection.
- Initial and reused Code Studio launches use `openPath`, resolving directories
  before reading files. `openFile` remains the file-specific API. Launch errors
  use the existing Code Studio status/error surface.
- The Code Studio terminal refits through a ResizeObserver and sends
  `{"type":"resize","cols","rows"}` as a text frame. Keystrokes are binary
  UTF-8 frames. Closing a tab or the window sends `{"type":"end"}` before
  `close()`. A server text frame `{"type":"line"}` sets
  `codeStudio.terminalLineMode` on the status label and is not written into
  xterm. The welcome line stays. The active session owns terminal/WebSocket/fit
  aliases. F5 and Run save through the tab queue, then write the command and a
  newline as one binary frame to the session active when Run started. They do
  not call `POST /api/code-studio/exec`. The terminal Stop button sends byte
  `0x03` only to that session. Bytes for a session that has already closed are
  discarded. A file without a runner (`codeStudio.noRunner`) does not write to
  the terminal. Runners are `go`, `python`, `.js`/`.mjs`, `.c`, and `rust`.
- Browser acceptance: `TestDesktopCodeStudioBrowser` renders the real shell,
  bundle, CodeMirror and xterm against an in-memory backend (no Docker) with
  `AURAGO_RUN_BROWSER_SMOKE=1`; set `AURAGO_BROWSER_ARTIFACT_DIR` for
  screenshots.
- Code Studio bundle load order in `scripts/build-ui-bundles.js` must be:
  core.js, sidebar.js, editor.js, terminal.js, search.js, agent.js, git.js,
  panels.js, shortcuts.js, command-palette.js.
- Code Studio visible UI strings use `codeStudio.*` keys in all
  `ui/lang/desktop/*.json` files.
- Code Studio New File uses the file API's `create_only` mode; a conflict leaves
  the existing disk file and open editor unchanged and uses the shared localized
  `desktop.fm.paste_exists` message. Save retains overwrite semantics.
- Code Studio Git commands run via Docker exec in the container workspace (`/workspace`).
  Git API endpoints are in `internal/server/code_studio_handlers.go`.
  The panel checkboxes start unchecked and the commit sends those paths with
  `add_all: false`. A commit with no selection and no staged change stays on
  `codeStudio.commitNeedsSelection`. HTTP 409 `git_identity_required` shows
  name and email fields and posts `/api/code-studio/git/identity`. Branch
  create and switch use POST.
- Startup shows the container `state` and `error` in the loading view, and the
  error view when startup reports an error. The status bar shows
  `codeStudio.readOnly` when the desktop is read-only, `codeStudio.networkOff`
  when `network` is `none`, and `codeStudio.idleUntil` once a running container
  has `last_activity` and `auto_stop_minutes` above zero. Search shows
  `codeStudio.resultsTruncated` when the response sets `truncated`.
- Code Studio recognizes C source/header files and reuses the bundled CodeMirror
  C/C++ parser. Run compiles `.c` as C17 with GCC into a private temporary
  directory, executes only a successful build and cleans up afterward. Rust
  uses that same `mktemp` directory and `"$cs_run_dir/program"`. Headers,
  TypeScript, TSX, JSX, and other files stay editable and do not run. With no
  restored tabs or launch path, open available `hello.go`, `hello.py` and
  `hello.c` samples as tabs.
  Verify `TestDesktopCodeStudioC` and the CodeContainer sample/runtime tests.
- System World loads `sysworld-data.js`, `sysworld-hud.js`, `sysworld-controls.js`,
  then `sysworld.js`.
  The first two expose `window.SysWorld.data/createHud`; the entry owns per-window
  instances and exports `SysWorldApp.render/dispose/inspect`. It imports
  versioned `/js/vendor/system-world/city.esm.js` only when opened.
- `sysworld-scene.js` is build input, not a classic lazy script. Rebuild with
  `node scripts/build-system-world.js` after changes; `--check` verifies exact
  output. Three.js and matching addons stay pinned to MIT 0.186.1.
- System World visible UI strings use `sysworld.*` keys in all
  `ui/lang/desktop/*.json` files (section registered as `'system-world':
  ['sysworld']` in `APP_I18N_SECTIONS`); the dock/start name uses
  `desktop.app_system_world`.
- SIP Phone exposes `window.SipPhoneApp = { render, dispose }`; every window
  instance owns its runtime subscription, 1-second clock/duration timer, tab
  state (`keypad`/`favorites`/`recents`/`contacts`/`settings`), address-book
  search state, long-press `0`→`+`
  handling, and its Web Audio keypad-tone context (DTMF hold-to-play
  feedback, closed on dispose). All call media flows through
  `window.SipPhoneRuntime`; the app
  must keep the `data-sip-phone*` hooks, observer-disabled controls, and the
  always-enabled hangup control asserted in `ui/desktop_sip_phone_test.go`,
  and must not introduce voicemail/mailbox UI.
- The floating phone gadget (`ui/js/desktop/core/sip-phone-gadget-runtime.js`,
  part of the desktop main bundle) mounts the same app windowless on a
  fixed, draggable layer inside `#vd-workspace` (sibling of
  `#vd-window-layer`, so its z-index is validly compared against
  `--vd-z-window`) under the reserved instance id
  `sip-phone-gadget`. It exposes `window.SipPhoneGadget = { init, sync }`,
  renders the app into a 400×830 stage scaled via
  `.vd-sip-phone-gadget-scale`, drags only from the status bar / Dynamic
  Island / hardware buttons / device frame, and persists
  `phone_gadget.enabled|position_x|position_y|always_on_top` through
  `/api/desktop/settings`. `applyDesktopSettings()` re-syncs the gadget, the
  Settings app owns the `phone_gadget.enabled` toggle, and the gadget
  context menu offers open-in-window, always-on-top, and remove. The layer
  hides below 640 px viewport width.
- SIP Phone visible UI strings use `desktop.sip_phone_*` keys in all 16
  `ui/lang/sip_phone/*.json` files (identical key sets); the dock/start name
  uses `desktop.app_sip_phone` in `ui/lang/desktop/*.json`.
- SIP keypad digits, long-press `0` to `+`, and backspace update the number
  without focusing its text field, so touch dialing does not summon the OS
  keyboard. Direct field editing remains available. Verify
  `TestDesktopSIPPhoneKeypadFocusBrowser`.
- Pixel's load order, module structure and registries are described in `## Pixel` at the top of this file. The loader
  (`module-loader.js`, key `pixel`) loads the sticker catalog and library first, then the Pixel scripts and styles. Modules
  attach through the registries and `bindRuntime` (no eval); shared state lives on the runtime object created by `pixel.js`.
- Pixel exposes `window.PixelApp = { render, dispose }`. Every window instance owns its layers, history, selection overlay,
  worker jobs and document-level key handlers, all removed on dispose.
- Pixel filters live in the `Pixel.FILTERS` catalog (`pixel-filters.js`, with the pixel work in `pixel-lib-fx.js`). Selecting a
  filter shows a live preview on a layer preview (`setLayerPreview`). Apply commits through `commitLayerPixels`, and leaving the
  Filters tab resets the preview. Legacy filter ids stay valid.
- Pixel visible UI strings use `pixel.*` and shared `stickers.*` keys in all 16
  `ui/lang/desktop/*.json` files.
- The shared sticker catalog contains 100 stable IDs, ten categories and ten styles.
  Video Studio, Pixel and Mailer use versioned local 192px thumbnails and 1024px RGBA PNG originals under
  `ui/img/stickers`; originals load only on insertion. `assets/stickers/production.json`
  records the individual Imagegen prompts; `node scripts/build-stickers.mjs --check`
  checks the generated catalog, files and translations. There is no sticker REST API.
- Mailer loads `mailer-stickers.js` before its composer and reuses the shared picker
  and all sixteen `stickers.*` locales. The existing draft upload/save path stores
  each selected PNG privately with an inline Content-ID; Markdown and outgoing HTML
  reference `cid:`, never an AuraGo asset URL. Only the editor resolves catalog
  thumbnails locally. Display stickers at 192px and preserve their original PNGs.
  Map the insertion bookmark through edits while loading, retain failed uploads for
  explicit retry, and let send/close await the upload queue. Disposal aborts fetches
  and uploads; late responses cannot touch another composer. Foreign HTML requires
  the existing explicit conversion; read-only and queued drafts disable insertion.
  Verify `scripts/test-mailer-stickers.mjs`, `scripts/test-mailer-browser.mjs`, and
  `TestStickerDraftAndMIMERoundTrip` / `TestAgentMailStickerWireFormat`.
- The Mailer surface is theme-native. `desktop-app-mailer.css` resolves local
  `--mail-*` aliases from the `--vd-theme-*` bridge (`--vd-theme-app-bg`,
  `--vd-theme-panel-bg`, `--vd-theme-chrome-bg`, `--vd-theme-control-bg`,
  `--vd-theme-border`, `--vd-theme-muted`, `--vd-theme-radius-*`,
  `--vd-theme-shadow-*`) plus `--vd-text` and `--vd-accent`; the legacy chat
  tokens (`--surface-color`, `--accent-color`, `--text-color`, `--text-muted`)
  are gone. Accent tints are mixed from `--mail-accent` with `color-mix()`, never
  from the fixed `--vd-accent-r/g/b` fallback. Mail HTML bodies keep their light
  paper surface; only the surrounding chrome follows the theme.
- The Mailer is split into `mailer.js` (window shell, list, reader, lifecycle),
  `mailer-views.js` (pure rendering helpers: `safeMailHTML`, `initials`,
  `parseSender`, `formatDate`, `formatBytes`, avatar, empty state, skeleton),
  `mailer-compose.js`, `mailer-settings.js`, `mailer-stickers.js` and
  `mailer-api.js`. The loader order is `mailer-api` → `mailer-views` →
  `mailer-stickers` → `mailer-compose` → `mailer-settings` → `mailer`; the views
  module must load before the shell because the shell delegates to it.
  `window.MailerApp` keeps `{ render, dispose, safeMailHTML }`.
- Mailer layout stays container driven: `container-name: mailer` with the
  1099 px and 719 px app-width breakpoints and `container-name: mailer-compose`
  for the composer. Viewport media queries are forbidden. The reader is an
  overlay pane below 719 px, and the list panel stays the flex child.
- The reader hands the whole pane to the message body. `.vd-mailer-reader` is a
  flex column, the message stack and the message grow into the free height, and
  the mail frame is an absolutely positioned iframe inside a relative wrap, so
  the body fills the column instead of floating in a fixed 210px strip. The
  reader itself only scrolls when the subject header, the sender rows and the
  footer need more room than the pane; a long mail scrolls inside its frame.
  The automation report (`.vd-mailer-rule-runs`) is capped and scrolls inside
  its own box, so it can never squeeze the mail body out of the reader.
- Mailer surfaces use the shared desktop app conventions: focus-visible rings,
  styled thin scrollbars, 140 ms hover feedback, `prefers-reduced-motion`
  fallbacks, sender avatars with a stable tone per name, and an unread marker on
  the avatar instead of an extra grid child.
- A thread row shows one person. `threadIdentity` picks the sender of the
  latest message and, in a sent folder, its recipient. The `participants` list
  of a thread mixes recipients and the sender, so it must never reach
  `initials` or the avatar: the letters of one address only. The reader header
  shows the roster as `mailer.participants_count` chip.
- The reader actions are grouped, never one flat pile: response actions
  (`reply`, `reply_all`, `forward`) keep their label, utility and file actions
  (`print`, `eml`, `archive`, `trash`, `delete`) collapse into icon buttons
  with `title` and `aria-label`, and every action carries its `vd-mailer-icon`.
  `readerActionButton` is the only builder for them. The back button belongs
  to the overlay reader below 719 px and stays hidden in the three-pane layout.
- Video Studio imports originals through the existing project media importer before
  adding a four-second overlay at the playhead, at 24% of the shorter canvas edge.
  Recheck project identity/epoch after asynchronous work and keep busy/locked overlay
  handling. Catalog filenames and the six legacy names remain recognizable artwork.
- Pixel sticker layers keep an immutable source image and `{id, matrix}` affine metadata.
  Canvas picking follows visible layer pixels from front to back, including opacity and
  masks, and activates the clicked sticker before its transform gesture. Transparent
  regions let lower stickers be selected; scale and rotation handles retain priority.
  Every transformation redraws the original. History and duplicates preserve metadata;
  whole-image transforms left-multiply it and also transform raster masks. Pixel edits,
  filters, adjustments, masks and selection mutations require explicit rasterization.
  Adjustment snapshots belong to the active raster layer, including after rasterization.
  Merge/flatten create raster layers; undo restores objects. `renderComposition` is shared
  by preview, save and export, including visibility, masks, opacity and alpha. Saved files
  are flat; the live session stays editable. Document revisions and abort controllers fence
  late insertion; disposal releases listeners, pointer capture and pending loads.
- Run `scripts/test-desktop-stickers.mjs` with the existing `PLAYWRIGHT_MODULE` /
  `PLAYWRIGHT_CHANNEL` overrides for browser checks; `TestDesktopStickerAssets` decodes
  every original and thumbnail, and `TestRenderBundledSticker1080p` performs an FFmpeg export.
- System World keeps one browser data subscription/polling set across windows.
  Reuse `system_metrics` SSE, with REST bootstrap/fallback; never poll the OS per
  window. Closing the last subscriber aborts requests and removes timers/SSE.
  Preserve labelled stale values, measured zeroes and configured/unknown states;
  enabled integration flags do not prove connectivity. Older REST responses or
  failures must not replace newer SSE samples. No raw tool arguments, prompts
  or issue details enter the bounded 60-event feed.
- The hologram artifact feed in `sysworld.js` polls GET
  `/api/desktop/system-world/memory-artifacts` (same-origin, `no-store`, 10 s
  abort) every 24 s only while the city is visible and not in map mode; failures
  back off 90 s and HTTP 429 keeps the normal interval. Until the first live
  response, and after failures, the archive's own localized counters feed the
  hologram (`source: 'local'`). Close clears the timer and aborts the request.
  `inspect()` exposes `artifacts.{source,count,failures,polling}`, never text.
- `sysworld-life.js` owns exactly five decorative white ThreeDee robots, rounded
  street routes, shared assets/soft hover lights and state-driven district rings.
  Normalize the GLB face axis (+X) to route-forward (+Z) before cloning;
  keep faces aligned with travel on straights and rounded turns.
  Only fresh actual error/running states or bounded recent events animate district
  signals; stale/unknown stays neutral. The operations signal uses private material
  clones so its red pulse cannot recolor other buildings. Rebind after LOD swaps.
  The effective System World motion preference freezes residents and pulses.
  Paused patrols render their accepted collision pose without submitting route
  adjustments; suspension may rebase the navigator but must not move the actor.
- `sysworld-navigation.js` steers those robots. Routes are right-turn block
  circuits sampled by arc length with 7.5 m corner handles; robots drive the
  right-hand lane (`2.2`) and may use `0/-2.2/±3.9`. Obstacles are circles
  derived from kit bounds (`obstaclesFrom(placements)`: tram, server rack,
  lamp pole, planter) expanded by the 1.3 m robot radius. Every frame checks
  lane probes up to 18 m ahead and trajectory conflicts up to 12 m; robots
  sidestep only away from where oncoming traffic is heading, yield to the
  robot closer to a shared crossing, brake behind slower leaders, and after
  2.5 s blocked they reverse direction (`state: 'turn'`, 4 s cooldown). Lane
  changes are velocity-limited to the current speed so heading always follows
  travel; stopped robots square to the street. These are movement intentions:
  `sysworld-traffic.js` validates every final pose before rendering. Spawns scan
  for a clear lane spot using the normalized exported model envelope.
- `sysworld-hologram.js` projects the memory archive hologram (roof 21 m):
  additive cone, base glow, rings, wireframe core, GPU motes, one billboard main
  panel and two orbiting fragment panels. Text is drawn with canvas `fillText`
  only and never interpreted as markup; `sanitizeArtifacts` trims to 24 entries
  and 140 characters. Panels cycle randomly every 4.5–7.5 s (12–15 s under
  reduced motion) with glitch/fade transitions. `stats()` reports counts,
  source and switch counters, never text.
- `sysworld-atmosphere.js` owns the sky dome (dusk gradient, weather-driven fbm
  clouds, Milky Way, meteors, city light dome, aurora), 1400 twinkling stars,
  moon, animated sea with shore foam and city-light reflections, sea mist, 500
  dust motes, the rotating spire beacon (faster while the agent is busy),
  instanced lamp cones, three night searchlights (always leaning away from the
  island) and up to 96 blinking aviation lights on skyline roofs. The sky disc
  follows its own path (behind the skyline at dusk/night); the key light keeps
  its front-lit path so glass facades never mirror a backlight into the camera.
  `sysworld-cinema.js` is the final `ShaderPass`: grade/split-tone, S-curve,
  edge chromatic aberration, anamorphic streaks, sun/moon shafts limited to a
  tight radius (no depth, so windows must not become shaft sources), vignette,
  grain and tour-only letterboxing. One shared `time` uniform advances only while
  animated, so hidden windows and reduced motion freeze the layer. Low tier hides
  mist/dust/lamp cones/searchlights and uses fewer cloud octaves; the post pass
  runs on high/ultra only. `sysworld-weather.js` adds soft lightning in rain
  (animated, outdoors only, 9–25 s apart, never a strobe) and reports thunder
  delay through `onThunder`. Clamp every
  `pow()` base: multisampled edge extrapolation yields NaN otherwise, and bloom
  smears one NaN over the whole frame.
- `sysworld-surfaces.js` gives the UV-less kit surface detail: six generated tileable maps
  (`ui/3d/system-world/textures/v1/`, R albedo / G roughness / B height) projected triplanar
  through `onBeforeCompile` on materials matched by name (`city.road|stone|graphite|titanium|
  bronze|ceramic|leaf` and the island ground). Static geometry uses world space; moving or
  articulated assets (tram, robots, doors, lifts, animated installations, drone clones) use
  object space so detail never slides. Bump uses screen-space derivatives of a metre-scaled
  height and fades out between 35 and 140 m. Low = no detail and no download, medium =
  albedo/roughness, high/ultra = bump and anisotropy. Maps load independently behind neutral
  placeholders; failures retry on the next quality change. Rain wetness eases (instant without
  motion); puddles form per material threshold on up-facing ground, reflect the fog colour and
  stay dry under the three pavilions. `city.ivory|warm` panes (four-vertex components) switch
  individually by time of day; larger lit parts and horizontal fixtures only dim. Pane light
  is decorative, never telemetry. Static walls darken towards their foot and the quay wall
  carries a wet waterline (world space only). Verify `node scripts/test-system-world-surfaces.mjs`.
- Look by time of day: bloom and exposure rise towards night; the day keeps thin fog, low
  sea mist and restrained ambient fill for readable shadows. The skyline placements are
  seeded (jitter, gaps, turned towers, taller towards the middle) plus a lower far row; they
  stay scenery outside navigation. `sysworld-atmosphere.js` also draws a faint distant
  coastline (fixed haze tint, not scene fog) behind and beside the city, open towards the
  camera side, with twinkling settlement lights at night.
- `sysworld-drones.js` flies up to six service-drone patrols (two on low) on closed Catmull-Rom
  loops with spinning rotors, navigation lights and banking; the template is the
  cached kit GLB and instances share geometry.
- `sysworld-scene.js` renders through `SceneCapturePass` (multisampled HDR scene
  target resolved and NaN-guarded into plain composer buffers) → bloom → output
  → atmosphere post. Never let bloom blend into a multisampled buffer.
- City radio waves consume typed `agent_action` starts and executed results, plus
  co-agent progress. Exact tool names map to districts; unknown tools, previews,
  blocked proposals and metric snapshots must not invent building-to-building traffic.
  Keep only route/state/tool-name metadata, never arguments, result/error text or
  session content. Deduplicate action states in a bounded map and clear it on close.
  Unknown, deferred and needs-setup results close the tracked action without
  emitting successful execution traffic.
  `sysworld-life.js` shares twelve wave packets and six route curves on its existing
  RAF. Coalesce bursts, expire by wall time, discard hidden/map/reduced-motion starts,
  and never replay retained events when a window opens or resumes. Robot geometry
  and the shared legacy Three.js remain unchanged.
- `sysworld-audio.js` owns quiet native Web Audio synthesis. Sound is opt-in,
  persisted, gesture-unlocked, volume-bounded and fades/suspends when the app is
  hidden, unfocused or in map mode. Close releases oscillators, nodes and AudioContexts.
  The soundscape is a sub drone, a detuned saw pad gliding through four chords
  (16 s) into a generated convolution reverb, air with wind gusts and at most 12
  transient events (swooshes, chimes, day birds, night harbour horn, thunder).
  Event/chord timers run only while audible and `quiet()` releases every voice on
  inactivity. `setMood` (busy/day/evening/weather from the scene) opens the pad
  filter. Do not add a `DynamicsCompressorNode`: its automatic makeup gain lifts
  the mix above the browser test's 0.05 RMS voice ceiling.
  `sysworld-voice.js` adds transient tower speech to that same opt-in mixer: one
  cancellable POST `/api/desktop/system-world/voice`, 20–40 seconds of quiet after
  each short phrase, with 60-second failure backoff. Camera pose updates through
  the existing scene RAF; no extra render loop. Distance to the tower attenuates
  the complete dry/85-ms echo/0.85-second stereo-room mix while leaving speech
  audible across the road grid. Normalize each decoded
  phrase by voiced-window RMS with a bounded input gain so quiet TTS backends remain
  intelligible; retain low voice fundamentals and a calibrated room impulse instead
  of browser-dependent convolver normalization. Limit peaks before distance gain
  and stereo placement. Master volume remains bounded to 35%; no media cache,
  raw text diagnostics, automatic retries without backoff or browser-TTS fallback.
  Hide, focus loss, map, mute/zero-volume and close abort requests, stop playback,
  release phrase nodes/tails and prevent late fetch/decode responses from replaying.
  Verify with `node scripts/test-system-world-voice.mjs` and the real-shell city test.
- Rendering owns one RAF per visible window. Minimize, Spaces, document hiding
  and map mode stop it. Close aborts loaders and frees GPU resources, listeners
  and observers. Context loss falls back to the usable map. Keep all models,
  local materials and licenses build-versioned; no remote textures or services.
- System World 2 adds `sysworld-experience/exploration/weather/effects.js` to the
  same renderer build and RAF. The v2 Blender manifest has 33 designs, three LODs,
  articulated clips and navigation metadata. Keep the complete app payload below
  48 MiB and first display below 12 MiB; exterior shells load with the city and
  interior furnishings load by proximity. Five original
  robots plus 19 new residents are the high-tier cap (eight total on low).
- `sysworld-traffic.js` owns one shared collision world for both resident groups,
  trams, drones, moving freight and the visitor. Fixed 1/60-second steps consume
  at most six steps per frame; discarded hidden time is never replayed. Resolve
  all intentions from the same snapshot, with swept circles/boxes and vertical
  intervals, iterating after rejections. Priority never pushes a stopped actor.
  Followers yield to their leader; trams retain their rails. Only a passenger's
  own vehicle is excluded. `sysworld-colliders.js` preserves ground apertures
  separately from upper building envelopes. Moving doors wait for clear panels.
  Trams accelerate and brake before obstacles/stops; yielding residents wait
  outside the rail corridor until the complete vehicle has passed. Guides
  retain their destination while blocked and resume when a route becomes free.
  Camera flights validate each segment, use nearby clear exits before climbing
  above roofs, and wait or stop when blocked. Manual orbit input cancels a flight.
- `sysworld-places/society/signals/machinery.js` own a bounded waypoint graph,
  reserved work slots, courier pickup/delivery, maintenance, archive exchanges,
  visitor greetings/guides and six local installation demonstrations. Guides
  wait for the visitor; the camera remains user-controlled. Residents clear
  approaching rails; conflicting drones use distinct vertical escape decisions.
  Cancel, hide, replay and close release reservations and packets. Local exchanges
  are labelled City life, never successful system actions. Eight reusable local
  wave packets show direction, receipt and reply; genuine building waves retain
  their existing twelve-packet metadata contract. No new endpoints or LLM calls.
- New installation geometry starts at LOD2 and upgrades by proximity (55/32 m),
  within the quality cap. Colliders use LOD0 envelopes/exported navigation data
  at every quality. Machinery particles are bounded and omitted on low; reduced
  motion freezes demonstrations and presents a localized quiet-state message.
  Effect envelopes fade within their existing lifetime, clear expired live colors
  and freeze under reduced motion; verify with the expansion script.
  Diagnostics expose bounded counts, actor poses, blockers and effect provenance,
  never content or invented telemetry. Verify shared movement with
  `node scripts/test-system-world-traffic.mjs` and real exported controllers with
  `node scripts/test-system-world-living.mjs` (`SYSTEM_WORLD_SIM_SECONDS=3600`
  for the extended run).
- `sysworld-layout.js` owns roads, clear building plots, station platforms and
  exposed surface heights. Tram rails and motion share `streetCurve`; test the
  swept vehicle body through bends. Do not use overshooting splines across plots.
  Quay, ground and interior floors must not be coplanar. Upper navigation is
  restricted to the rendered guarded gallery and docked lift platform. Verify
  `node scripts/test-system-world-layout.mjs` against actual exported GLBs and
  the browser expansion round including leaving, traversing and returning from
  the upper floor; lift arrival alone does not prove a usable landing.
- Sky deck and viewers (`sysworld-experience.js`, `skyDeck` in `sysworld-layout.js`):
  the open east lift of the agent tower travels 71.32 m (mirrors `SKY_TRAVEL` in
  `build_expansion.py`) to the 72.2 m crown terrace. Up there only the ring between
  `inner` and `outer` is walkable, plus the bridge while the cab is docked. The
  agent's collision uses stepped `upperSolids` (`sysworld-exploration.js`) instead of
  its bounding box, so shaft and terrace stay free. Rides take 15 s, calls 7 s; idle
  trips happen only while nobody is within 30 m; reduced motion makes all of them
  instant. Viewers on galleries and deck move the camera to their pivot (deck: a
  virtual objective beyond the balustrade), clamp yaw/pitch to their field, zoom by
  wheel or +/-, hide their model while in use and report the reticle's district via
  `onScope`; the HUD draws lens, reticle and a live tag. Escape leaves a viewer
  before it leaves street mode. Verify with the layout script and the expansion round.
- `sysworld-controls.js` owns destinations, discoveries, environment, three audio
  buses, the typed terminal and 24-hour replay. UI hints must not mutate the DOM
  every frame. Replay immediately disables system actions and private memory feeds;
  missing history shows a gap, never substituted live values. Confirm stop/cancel/
  restart with the concrete target and lock before opening the confirmation dialog.
  Accepted actions remain pending until a newer matching state confirms the result.
  Opening an encounter, terminal or discovery releases this city's pointer lock.
  Close/Escape returns focus to the canvas; encounter updates preserve keyboard
  focus and the chosen guide destination. Reserve space for walking controls and
  hide keyboard-only interaction hints on touch devices. Interaction prompts yield
  to the open exploration panel. Verify with the living browser scenario.
- Server `internal/systemworld` stores bounded telemetry in the Desktop database.
  `/api/desktop/system-world/{snapshot,history,events,entity,actions}` requires admin
  scope; reuse existing services/write gates. The shared ten-second metrics worker
  owns collection, not windows. No prompts, reasoning, tool results, credentials or
  memory excerpts belong in history. Detailed contracts and acceptance commands:
  `documentation/system-world-2.md`.
- Quality `auto/low/medium/high/ultra` persists under
  `aurago.desktop.sysworld.quality`. LODs are separately fetched and cached;
  instanced scenery uses shared geometry/materials, distant towers always LOD2.
  Quality changes rebuild instances after assets finish without moving focus.
  Auto uses frame-time hysteresis; shadow maps update only after geometry/tier
  changes. No per-frame geometry or material creation.
- District placement and entity IDs stay fixed through data refresh. User
  navigation cancels the explicit tour; inactivity never takes the camera.
  Motion `auto/on/off` persists under `aurago.desktop.sysworld.motion`. Default
  `auto` honors reduced motion and Desktop animation settings; explicit `on`
  enables this city's animation without changing global preferences, and `off`
  freezes it. The exploration selector controls the existing simulation; its
  summary visibly labels a paused city. Reapply the effective preference after
  asynchronous city loading. Verify `AURAGO_SYSTEM_WORLD_MOTION=1` with the city
  browser test (all moving groups, pause, persistence and system changes).
  Effective reduced motion suppresses camera flights/tours and the decorative
  pulse. Street mode owns WASD only while its canvas is
  focused; pointer lock is explicit and Escape/blur/close release control.
- HUD presentation: district labels are ranked (selected, hovered, error, running,
  then distance with hysteresis) and never overlap each other or HUD panels; a
  label cut by a panel's lower edge slides down its building by at most 90 px.
  Panel and label rectangles come from a ResizeObserver, never from per-frame
  layout reads; label state and transforms are written only on change. Hover is
  a raycast throttled to 80 ms (ring, cursor, highlighted label and rail entry);
  the selection ring takes the district's state colour. Loading shows real byte
  progress (tier LODs from the manifest plus surface textures); the first display
  fades the canvas in with a one-time opening glide. Camera teleports dip through
  a short fade, but `city.setMode` is never delayed. The tour shows a lower third
  per station. Photo mode (camera button or `H`; `H`/Escape leave it before the
  canvas sees the key) hides the whole interface and saves the composed frame
  right after `composer.render()` in the same task, without
  `preserveDrawingBuffer`. CPU/RAM sparklines keep 60 live samples and never
  ingest replay data. The street compass (north is -z) redraws only after a 1.5°
  turn or a 1.5 m step. Reduced motion skips the glide, fades and caption motion.
  The city browser matrix checks label overlap, hover, compass, tour caption and
  the photo PNG.
- The map is an SVG plan drawn from `NS.map`/`NS.districts`, mirrored constants of
  `sysworld-layout.js` and the scene's districts (`node
  scripts/test-system-world-layout.mjs` keeps them in step): water, quay, streets,
  blocks in state colours with counts, tram loop and stops, pavilions, drone pad,
  legend and the last camera position. It needs no WebGL and stays the
  context-loss fallback; the exploration control hides in map mode. At 760 px and
  below the rail is an icon column whose toggle opens a drawer with search
  (choosing an entry or Escape closes it and clears the query), and the inspector
  is a bottom sheet beside the rail that yields to street, tour, the open
  exploration panel and the drawer. The browser matrix fails when the exploration
  control, rail, inspector or fixed bars overlap at any tested size.
- Verify `node scripts/test-system-world.mjs` (includes the navigation
  blockade/sidestep units and a 60-minute five-robot simulation), `node
  scripts/build-system-world.js --check`, focused Sysworld Go tests, `go test
  ./internal/server -run SystemWorld`, and the real-shell browser matrix:
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SYSTEM_WORLD_MATRIX=1 go test ./ui
  -run '^TestDesktopAuroraBrowser$'`. The browser run mocks the artifact feed,
  asserts live hologram/atmosphere/drone stats without text leaks, captures
  Three.js console errors, and fails when the canvas renders flat (blank
  post-processing). Screenshot/performance reports stay ignored.

- Terminal uses xterm 6 with the WebGL addon. CRT capture includes every canvas
  below `.xterm-screen`, including the unnamed WebGL text canvas. Preserve the
  drawing buffer and usable DOM/CSS fallback after context loss.
- Sheets 1.0 runs write interceptors sequentially; localized input parsing runs
  after the format parsers and before the terminal interceptor (priority -0.5),
  so the native formatter cannot reinterpret numeric values.
  Read function metadata from `engine-formula.functionList`. Browser
  input tests wait for the loading overlay and emit both clicks of a double click.

## Work Guidance

- A local translation helper that prepends a prefix
  (`tr = key => ctx.t('desktop.gog.' + key)`) is registered in
  `desktopLocalI18nHelpers` (`ui/i18n_lint_test.go`) and called with literal
  keys. The static checks resolve those calls in every locale and skip the
  bare prefix. Verify `TestFrontend_StaticI18nKeysExistInEnglishBundle` and
  `TestDesktopUsedI18nKeysExistInAllLanguages`.
- Files exceeding 1100 lines must be added to `knownOversizedContinuations` in
  `ui/desktop_js_line_budget_test.go`; use the map there as the current
  source of truth for oversized continuation files.
- Performance-sensitive Galaxa rendering respects the `ctx.settings.particles`
  setting (`low`/`medium`/`high`); particle/trail caps must scale accordingly.
- Galaxa audio uses Web Audio API synthesis only (no sample files). New SFX
  must check `ctx.G.muted` and respect `ctx.G.vol`.
- Juice pass (2026-08): `shootTyped`, `puCollectRarity`, `weaponArm`,
  `bossKillFanfare`, `megaComboStinger`, `stageClearFanfare`, plus
  `fxMuzzleSparks` / `fxBossKillSetPiece` / `fxMegaCombo` /
  `fxStageClearSetPiece`; signature FX honor `FX_CAPS` and
  `prefers-reduced-motion`.
- Combat-juice pack (2026-09) adds `superReady` / `heartbeat` / `multiKill`
  SFX (mute-guarded) plus a shimmer layer on `respawn`; `FX_SUPER_READY_DUR`,
  `FX_LASTLIFE_INTERVAL`, `FX_MULTIKILL_WINDOW/COUNT/HITSTOP` constants live in
  `galaxa-constants.js`.   `registerKill(x, y)` tracks the
  `multiKillCount`/`multiKillWindow` cluster and fires `fxMultiKill` once per
  cluster; `superReadyFired` (reset in `startSuper` and below 100 % meter)
  gates the one-shot super-ready cue in `updateFX`. Menu states (SHOP, evo
  choice) tick `updateFX` so leftover gameplay FX decay instead of freezing
  as artifacts over the menu panels.
- Enemy-behaviour pass (2026-10) adds `feintWhistle`, `squadDive`, `rageGrowl`,
  `shuffleRumble`, `smolderIgnite` and `smolderBlast` SFX (mute-guarded) plus
  the `galaxa-enemy-fx.js` effects
  and the quirk tables listed above. Keep quirk rolls, dive-path picks and
  squad/revenge/smolder selection on the seeded `ctx.random` (only the particle
  seeds may use `Math.random`, because particles never enter the simulation
  signature); smolder divers steer toward the player and detonate centrally in
  `smolderDetonate`. New enemy FX belongs in
  `galaxa-enemy-fx.js`, not in `galaxa-fx.js` (which is near the line budget).
- Galaxa canvas resource caches (`cachedRadialGradient`, `spriteAtlasCache`,
  `ensureNebulaCanvas`) must be reused; see
  `ui/desktop_runtime_performance_test.go` for enforced markers.
- Keep Chess split across `chess.js`, `chess-fx.js`, `chess-engine.js`, and
  `chess-agent.js`; do not fold worker, API bridge, template, or FX/audio
  helpers into the main app file. `chess-fx.js` must load before `chess.js`.
- Keep Cheater split across `cheater.js`, `cheater-toolbar.js`,
  `cheater-spotlight.js`, `cheater-templates.js`, and `cheater-attachments.js`;
  do not fold the toolbar, spotlight, or attachment logic into the main app
  file.
- Keep Sheets lifecycle, data conversion, panels and charts in their four
  existing modules; reuse the shared OfficeSession queue and native engine.
- Keep Code Studio split across `core.js`, `sidebar.js`, `editor.js`,
  `terminal.js`, `search.js`, `agent.js`, `git.js`, `panels.js`, `shortcuts.js`,
  and `command-palette.js`; do not fold domain modules into core.js.
- Keep System World's data, HUD, scene, life, navigation, hologram, atmosphere,
  drones, audio and lifecycle in their owned files. Do not merge renderer
  dependencies or data polling into the Desktop shell.
- Keep OpenSCAD split across `openscad.js`, `openscad-editor.js`, and
  `openscad-defines.js`; do not fold the CodeMirror editor or defines slider
  logic into the main app file.
- Keep Homepage Studio split across `homepage-studio.js`,
  `homepage-studio-preview.js`, `homepage-studio-sites.js`, and
  `homepage-studio-history.js`; do not fold the preview chrome, sites, or
  history panels into the main app file. Load order in
  `module-loader.js`: preview, sites, history, then `homepage-studio.js`.
- OpenSCAD exposes `window.OpenSCADApp = { render, dispose }`. Every window
  instance owns its draft timer, SSE listeners, editor, and preview resources.
- The OpenSCAD STL preview keeps one scene record per `renderSTL` with its own `AbortController`; a late
  download for a replaced scene is ignored and never touches the current record. PNG, SVG, PDF, empty and
  download-only previews call `cleanupPreview` before replacing the panel. Verify
  `TestDesktopOpenSCADSTLPreviewIgnoresStaleLoads`.
- OpenSCAD uses a preview-first workbench layout: a slim header bar with
  primary actions (render, generate, cancel, download, save) and toggle
  buttons for the three panels; the main grid is
  `[inspector | splitter | preview | splitter | parameters]` with explicit
  grid tracks so the preview keeps its column when panels collapse. The left
  inspector column hosts the Source/Files/Log tabs plus an auto-hiding
  issues bar (hidden when empty) and is resizable/collapsible; the center
  preview zone is edge-to-edge with floating glass overlays (title chip,
  viewport toolbar pill, status pill) so the viewport loses no space to
  chrome; the right parameter sidebar holds exports with a selected-count
  badge, segmented Render/Preview mode, timeout, and defines sliders; the
  agent panel is an absolute slide-over (never squeezes the preview) hosting
  the streaming chat transcript, a composer, and an apply-changes bar that
  stages agent-proposed source instead of silently overwriting.
- OpenSCAD viewport controls include zoom in/out, perspective/orthographic
  projection, shaded/wireframe shading, auto-rotate (disabled under
  `prefers-reduced-motion`), dark/light background, grid/axes, fit view,
  double-click-to-reset, and fullscreen on the whole preview zone (overlays
  stay visible). The busy overlay is scoped to the preview zone (editor
  stays usable) and shows elapsed seconds.
- OpenSCAD 3D preview uses a gradient canvas-texture background, a
  ShadowMaterial contact-shadow plane plus a model-sized grid grounded at
  the model's bounding box, `framePreviewCamera` bounding-sphere fit
  targeting the box center, and a `ResizeObserver` that keeps renderer size
  and camera aspect in sync (no stretched canvas on window/fullscreen
  resize). `cleanupPreview` disposes geometry, materials, background
  texture, and disconnects the observer.
- OpenSCAD keyboard shortcuts (Ctrl+Enter render, Esc cancel, F fit view,
  Ctrl+S save draft) are attached in `wireKeyboardShortcuts` and removed
  in `dispose`; splitters support pointer drag, double-click to collapse,
  and ArrowLeft/ArrowRight resizing.
- OpenSCAD drafts persist per `windowId` under
  `aurago.desktop.openscad.draft.<windowId>` and include additive viewport
  preferences (projection, shading, auto-rotate), panel collapse state, and
  inspector/sidebar widths.
- OpenSCAD toolbar icons use themed manifest icons (`sliders`, `cube`,
  `mesh`, `contrast` were added for this app to `ui/img/papirus`,
  `ui/img/whitesur`, and the backend icon catalog in
  `internal/desktop/types.go`); button icons are retinted via CSS
  `--oscad-icon-filter` so they stay legible on dark glass.
- OpenSCAD result events: a present `window_id` still selects that window
  only. A missing `window_id` is ignored whenever more than one OpenSCAD
  window exists, including a busy window. OpenSCAD does not listen for
  `window` `message` events. Preview URLs must match
  `/api/openscad/jobs/oscad-…/files/…`. SVG preview uses an image element,
  and PDF preview uses a sandboxed iframe. The more-exports row includes AMF.
- OpenSCAD readonly mode disables CodeMirror/`textarea` editing, defines
  inputs, and the agent prompt. Readonly mode does not apply `source_scad`
  into the editor.
- OpenSCAD visible UI strings use `desktop.openscad.*` keys in all
  `ui/lang/desktop/*.json` files.
- `homepage-studio.js`, `homepage-studio-preview.js`,
  `homepage-studio-sites.js`, and `homepage-studio-history.js` implement
  Homepage Studio as a preview-first workbench: slim header (brand, status
  pill with server/fallback/tunnel state, target selector, panel toggles),
  main grid `[chat | splitter | preview | splitter | inspector]` with
  explicit tracks, collapsible/resizable chat and inspector panels, a
  floating viewport chrome (URL pill, desktop/tablet/mobile device
  segmented, refresh, external, fullscreen), a site/drift pill and an
  agent-busy pill with elapsed time, and an inspector with Sites (managed
  site cards, drift badges, deploy targets/deployments/remote observations,
  reconcile) and History (search, type filter, offset pagination) tabs.
  The welcome hero offers localized prompt suggestion chips.
- Homepage Studio chrome uses the shared `--vd-theme-*` materials and Desktop
  accent in `css/desktop-app-homepage-studio.css`. Keep panels, empty/loading
  previews, controls and history badges legible in Standard and Fruity light/dark;
  avoid a separate gradient palette. The website iframe retains its own colors.
- Homepage Studio exposes `window.HomepageStudioApp = { render, dispose }`;
  every window instance owns its AbortControllers, busy/persist timers,
  listeners, and sub-module instances and must release them in `dispose`.
  Workbench state persists per `windowId` under
  `aurago.desktop.homepage.draft.<windowId>` (target, device, panel
  widths/collapse, inspector tab, history filters, selected site).
- Homepage Studio URL validation and preview sandboxing
  (`safeExternalURL`, `firstPreviewURL`, `homepageStatusPreviewURL`,
  `updatePreviewUrl`, `showPreview`, `refreshPreview`; iframe
  `allow-scripts allow-forms` + `referrerPolicy no-referrer`, never
  `allow-same-origin`) stay in `homepage-studio.js` — they are pinned by
  `ui/security_lint_test.go` and must not move into sub-modules.
- `homepage-studio-preview.js` (`window.HomepageStudioPreview { create }`)
  owns the preview chrome **and the workbench layout**: the device switcher, the
  fullscreen control, the panel splitters (pointer drag, double-click collapse,
  ArrowLeft/ArrowRight), the collapsed state, and the scaled device viewport
  (the previewed page renders at the real device width — desktop 1280 px, tablet
  768 px, mobile 390 px — and is scaled into the pane through
  `.vd-hp-preview-frame`, `--hp-device-w`, `--hp-device-h`,
  `--hp-device-scale`); the sites panel
  (`window.HomepageStudioSites { create, DRIFT_KEYS, driftStatus, targets }`)
  owns the site cards, the drift enum and the deployment target collection
  (`targets.collect/add/normalize/external`), and the history panel
  (`window.HomepageStudioHistory { create }`) receives their dependencies
  via a deps object like `NoisemakerLibrary`.
- `homepage-studio-annotate.js` (`window.HomepageStudioAnnotate { create,
  hitTest, overlapping, normalizeBox, tabOrder, composeMessage,
  annotationPayload, cropLayout, MAX_MARKS }`) owns annotate mode. It injects
  the toolbar button (`data-hp-annotate`), the snapshot layer inside
  `.vd-hp-preview-frame` (scaled with the device), the mode bar and the mark
  popover on `.vd-hp-preview-panel`, and the mark chips before the chat form;
  styles live in `css/desktop-app-homepage-studio-annotate.css`. Snapshots come
  from `POST /api/desktop/integrations/homepage/annotation-snapshot`
  (`internal/server/homepage_annotation_handlers.go`, capture in
  `internal/tools/homepage_annotation_capture.go`). The client sends site id,
  target, path, viewport and `preview_url` (the URL its iframe shows). The
  server uses `preview_url` only for deployed targets, and only when it matches
  a URL registered for a managed site; otherwise it answers
  `preview_unavailable`. The capture Chrome runs through a per-capture filtering
  forward proxy (`internal/security/capture_proxy.go`, `--proxy-server` and
  `--proxy-bypass-list=<-loopback>`). The proxy dials only the local preview
  origin `127.0.0.1:<WebServerPort>` directly and checks every other destination
  with the SSRF rules. It caps 16 concurrent connections, 600 connections and
  64 MiB per capture. `GuardRodPageSSRF` is unchanged. Captures use a
  desktop-style viewport at every width (`Mobile: false`). Keyboard: Tab and
  Shift+Tab step through `tabOrder`; Tab past the last element goes on to Path,
  Recapture and Done, Shift+Tab before the first element leaves the snapshot,
  and neither wraps. Escape closes the popover, then the mode, and stops
  propagation so the studio's Escape handler does not run. `takeForSend` returns
  the message with the localized "Marks:" list (user comments only),
  `context.annotations` (element details, rendered as untrusted external data by
  `buildHomepageStudioAgentContext`) and `context.image_base64` (numbered crops,
  at most 1000 × 1600 px and 1.4 MB of base64, below the 2 MiB chat body limit).
  If the image cannot be used (too large, a provider that needs public image
  URLs, or a failed vision fallback), the turn is still sent and the prompt says
  so. On a text-only provider, the vision-fallback description of the marks
  image enters the prompt only as untrusted external data
  (`homepage_image_analysis`). Marks are not part of the draft. They are
  dropped when the target changes, and for a deployed target also when the site
  or the preview URL changes. The site pill renderer lives in the sites module
  (`renderSitePill`), which keeps `homepage-studio.js` under the 1100-line
  budget.
- Homepage Studio accessibility contract: the chat log is a transcript, not a
  live region — it carries `aria-busy` and **no** `aria-live`; a dedicated
  `role="status"` node announces the finished message, the agent start and the
  agent completion exactly once; the elapsed counter stays `aria-hidden`. The
  inspector tabs are a `role="tablist"` with `role="tab"` children, arrow-key
  navigation and a roving `tabindex`; the site list is a `role="listbox"` with
  `role="option"` cards that keep the keyboard focus across re-renders; the
  preview iframe always carries a `title`; the panel splitters expose
  `aria-valuenow`/`aria-valuemin`/`aria-valuemax`; the device switcher is a
  `role="radiogroup"`.
- The shared translator `t(key, params)` treats its **second argument as a
  placeholder map**, so a string fallback never applies. Homepage Studio uses
  the local helper `tr(key, fallback)` (defined in `homepage-studio.js`, handed
  to the sub-modules through the deps object) for every dynamically built key
  (`homepage_studio.sites_drift_*`, `history_type_*`, `history_filter_*`,
  `sites_origin_*`, `sites_target_*`, `sites_outcome_*`).
- Homepage Studio honors `context.readonly` (composer, suggestion chips,
  reconcile, and history delete disabled). Destructive history deletes use
  the shell `confirmDialog` passed by `menus-and-routing.js`; native
  `alert`/`confirm`/`prompt` are forbidden in all four modules.
- Homepage Studio loads its site list **once per status load** and refreshes
  only the detail of the selected site; the target resolver reuses the sites
  panel list and its cached detail instead of issuing a second request per
  site. The status pill describes the **selected target** (container running,
  cloud target reachable, cloud target unreachable), not only the local
  container.
- Homepage Studio visible UI strings use `homepage_studio.*` keys in all
  16 `ui/lang/desktop/*.json` files. The local webhost name fallback
  uses `homepage_studio.default_name`. Chat-stream failures reuse
  `desktop.chat_request_failed`.
- Keep Writer's document lifecycle, save queue and panels in their existing three
  modules; load session and panels before writer.js. Rebuild and check local vendor
  assets with `node scripts/build-writer-vendor.js [--check]`.
- Build/check Sheets with `node scripts/build-sheets-vendor.js [--check]`.
  Both the main engine and worker register AVG as the AVERAGE compatibility
  alias. English formula syntax is independent from localized input/display.
  The worker's cycle guard uses the public dependency graph and runtime result
  API to mark cycles and their dependents as #REF!, instead of Univer's default
  one-iteration numbers. Keep graph emission enabled and test self/cross-cell
  cycles, unaffected formulas and recovery after breaking the cycle.
- Rebuild chess vendor assets with `npm run build:chess-vendor` after changing
  vendored chess package versions or copied Stockfish assets.

## Verification

- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestQRDialogWriterPixelBrowser -count=1`
  covers the shared real-core dialog and decodes QR media after native DOCX/ORA
  save/reopen, plus undo/redo, selection fencing, all sixteen locales and viewport
  cases. `node scripts/test-pixel-qrcode.mjs` covers raster insertion, bounds,
  readonly, document changes and disposal; `npm run test:pixel` includes it.
  Shared QR use and limitations are documented in
  [QR codes](../../../../documentation/qr-code.md).

- `node scripts/test-sheets-data.mjs`, `node scripts/test-writer-session.mjs`
- `node scripts/build-sheets-vendor.js --check`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopSheets(Engine|App)Browser' -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SHEETS_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1` (18 theme/density/size screenshots)

- `go test ./ui/ -run TestDesktopFeeling`
- `go test ./ui/ -run TestDesktopWidgetConfigPersistence`
- `go test ./ui/ -run TestDesktopWeatherWidgetI18n`
- `go test ./ui/ -run TestDesktopVirtualComputersDurationI18n`
- `go test ./ui/ -run TestDesktopCalculatorBackI18n`
- `go test ./ui/ -run 'TestDesktopMissionControl|TestMissionControlDispose'`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopMissionControlBrowser -count=1`
- `node scripts/test-mission-control-schedule.mjs`
- `node scripts/test-mission-control-flows.mjs` (or `npm run test:mission-control` for both)
- `node scripts/test-dashboard-cronjobs.mjs` (`npm run test:dashboard-cron`)
- `go test ./ui/ -run TestDesktopFileManagerTemplateI18n`
- `go test ./ui/ -run TestDesktopWidgetDisplayTitle`
- `go test ./ui/ -run 'LineBudget|GalaxaMode|DesktopAppAssets|AdaptiveMusic'`
- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go test ./ui/ -run TestGalaxaDeluxeCachesCanvasResources`
- `go test ./ui/ -run TestVirtualDesktopJSUsesSemanticChunkNames`
- `go test ./ui/ -run "TestDesktopChess|TestDesktopAppsExposeDisposeLifecycle|TestDesktopAppAssetsRegistry"`
- `go test ./ui/ -run "TestDesktopCheater"`
- `go test ./ui/ -run "TestDesktopSheets"`
- `go test ./ui/ -run TestDesktopSheetsMatchCountI18n`
- `go test ./ui/ -run TestDesktopPeopleDaysI18n`
- `go test ./ui/ -run TestDesktopElapsedSecondsI18n`
- `go test ./ui/ -run TestDesktopShellPromptDefaultsI18n`
- `go test ./ui/ -run TestDesktopChatFallbackI18n`
- `go test ./ui/ -run TestDesktopFileManagerNewFileDefaultI18n`
- `go test ./ui/ -run TestDesktopWriterSearchI18n`
- `node scripts/test-writer-session.mjs`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopWriter(App|Engine)Browser'`
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_WRITER_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$'`
- `go test ./ui/ -run TestDesktopRelTimeI18n`
- `go test ./ui/ -run TestDesktopPetChromeI18n`
- `go test ./ui/ -run TestDesktopRadioCompactI18n`
- `go test ./ui/ -run TestDesktopRadioMediaSessionI18n`
- `go test ./ui/ -run TestDesktopPeopleKgI18n`
- `go test ./ui/ -run TestDesktopSysworldHudUptimeI18n`
- `go test ./ui/ -run TestDesktopSysworldHudMoneyI18n`
- `go test ./ui/ -run TestDesktopSystemInfoGaugeI18n`
- `go test ./ui/ -run TestDesktopEmbedFrameI18n`
- `go test ./ui/ -run TestDesktopChessResultI18n`
- `go test ./ui/ -run TestDesktopChessErrorI18n`
- `go test ./ui/ -run TestDesktopChessLoadI18n`
- `go test ./ui/ -run TestDesktopChessTemplateI18n`
- `go test ./ui/ -run TestDesktopSysworldSuccessRateI18n`
- `go test ./ui/ -run TestDesktopSysworldPanelIdI18n`
- `go test ./ui/ -run TestDesktopHomepageStudioFallbackI18n`
- `go test ./ui/ -run TestDesktopZipperFilterI18n`
- `go test ./ui/ -run TestDesktopPixelOpenFilterI18n`
- `go test ./ui/ -run TestDesktopPixelSaveFilterI18n`
- `go test ./ui/ -run TestDesktopTerminal`
- `go test ./ui/ -run TestDesktopCodeStudioShellI18n`
- `go test ./ui/ -run TestDesktopQcAuragoHostI18n`
- `go test ./ui/ -run TestDesktopHostErrorI18n`
- `go test ./ui/ -run TestDesktopLoadFailedI18n`
- `go test ./ui/ -run TestDesktopAppAssetsRegistry`
- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go test ./ui/ -run TestDesktopLooper`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopLooperBrowser`
- `go build ./cmd/aurago`

## Child DOX Index

- `looper.js` / `looper-monitor.js` own Looper v2 (`window.LooperApp` +
  `window.LooperMonitor`). A loop is goal + work + evaluate + optional finish.
  Visible controls are rounds (1–50), target score (50–100), stall rounds
  (0–10, 0 off), provider and model. No context mode, confidence or truncation
  UI. Layout is three columns from 820 px (list | editor | run/history) with
  one footer action bar; below that, compact tabs Setup / Run / History.
  The loop completes when the score reaches the target; evaluator `done`
  is logged only. Review is tool-free JSON against the work report; a failed
  or invalid review logs score 0 and the loop continues. Monitor shows
  `error`, a pending current step, the sparkline, timeline and last 20 saved
  runs. Status SSE
  is `/api/desktop/looper/status`; start/resume stay admin POST. Shell must
  pass `promptDialog` and `confirmDialog`. Readonly disables start/save/delete
  and form edits. Keep `desktop.looper_*` in all 16 desktop locales, including
  cost/token/duration keys shared with System World. No emoji in the app
  sources. Verify `TestDesktopLooper*`, `TestDesktopAuditedI18n*`, bundle
  `--check`, and opt-in `TestDesktopLooperBrowser`
  (`AURAGO_RUN_BROWSER_SMOKE=1`). Backend: `internal/desktop/looper*.go`,
  `internal/server/looper_service.go`, `desktop_looper_handlers.go`.
  No child DOX file needed.

- `ha-switchboard.js` exposes `window.HASwitchboardApp.render/dispose`. The
  scoped `css/ha-switchboard.css` retains the real shell controls and uses the
  original local walnut/three-pose silver atlas (SVG clips remove its background).
  The SVG dial shares a 240-degree scale/needle pivot and keeps labels below the
  sweep. Size-container queries shrink cabinet chrome and levers for short windows;
  a single row must fit at 1300 x 540/620/700 in every theme. Keep entity IDs in
  plaque/switch tooltips so similarly named HA entities can be distinguished.
  Use the existing admin-only `/api/desktop/home-assistant/{entities,states,switch}`
  routes and shared validated `ha_switchboard.board` setting, never browser HA
  credentials or a second connection setup. The native dialog searches locally,
  preserves failed-save drafts and detects changed selections before saving.
  Poll every five seconds only while visible, cancel obsolete reads, pause in
  inactive Spaces, and release requests/timers/listeners on dispose. Explicit
  on/off writes remain pending until a subsequent state read confirms the target;
  never retry a write automatically. Keep missing entities visible, escape HA
  names, respect desktop/HA read-only and service capabilities, preserve keyboard
  switches and reduced motion. Verify `TestHASwitchboardBrowser` with
  `AURAGO_RUN_BROWSER_SMOKE=1`, `TestHASwitchboardLocales`, focused backend
  `TestHASwitchboard|TestHAContext`, bundle `--check` and pinned asset validation.
  Implementation and acceptance details: `documentation/ha-switchboard-plan.md`.

- `meshcore.js`: native Messenger (`window.MeshCoreApp.render/dispose/openConversation`),
  using `/api/meshcore/messenger/` and the existing Companion manager. Owns
  direct/channel conversations, protected-text reveal, contact/channel dialogs,
  native BarcodeDetector import and the existing QRCode renderer. No direct
  hardware connection or agent-tool sending. Labeled Messages/Device/Settings
  navigation stays visible on all pages. Theme styles live in
  `css/desktop-app-meshcore.css`; the 300px list switches to single-pane navigation
  below 700px. Every selector stays scoped under `.vd-meshcore` because Mission
  Control reuses `mc-*` class names. Scoped `--mc-*` tokens provide teal surfaces
  on a dot-grid canvas, per-theme variants, tinted hash-hue avatars with contact
  type badges, relative non-sticky day dividers, SNR signal bars and skeleton
  loading; Geist Mono renders keys, times and radio values. Healthy send states
  are icon-only with a screen-reader label; failed/uncertain states keep a
  visible label. Inline
  SVG icons use `icon()`/`iconEl()` without external assets. History search opens
  on demand; closing it clears the query. Distinguish empty conversations from
  filtered results and provide filter reset. Hide the composer until a conversation
  is selected; grow its input with the draft and show packet preview for multipart
  messages. Messages group by direction/origin within a 300 s gap via
  `mc-group-start/mid/end`; day changes break groups. Message actions
  (reveal/copy/retry) reveal via `opacity` on hover/focus-within, never via
  `pointer-events`, `visibility` or `display`; they remain visible on touch/narrow
  layouts. They live in `.mc-message-actions`, floating beside the bubble so
  hidden actions never shift the meta line, and render inline on touch/narrow. Preserve ArrowUp/ArrowDown roving focus through list refreshes. Esc
  closes details/search before leaving the chat, unless a dialog is open. Drafts
  and pending send IDs are local per device/conversation; invitation keys never
  enter browser storage. Persistent request IDs reconcile HTTP retries; explicit
  resend warns about duplicates. Abort all requests and remove document
  listeners/timers on dispose. Session/notification context contains only a
  validated conversation ID. All 16 desktop locales must include
  `desktop.meshcore_*`. Verify `TestDesktopMeshCoreBrowser` in standard and fruity
  light/dark themes at wide and narrow widths.
  New channels default to the hashtag type so a name such as `#bot` uses its derived key; the invitation field also accepts `#bot` as shorthand when the name is empty. Selecting Public fixes the channel name to `Public` and restores the draft name when switching back.

- `file-manager/` (under `ui/js/desktop/file-manager/`, bundled to
  `file-manager.bundle.js`) - File Manager restore and empty-trash menus follow
  the Trash restore contract above. New-file templates, new-file default
  names, and ZIP/rename success toasts follow the i18n keys in Local
  Contracts. No child DOX file needed.
- `galaxa-modes.js` - Game mode contracts (`gauntlet`, `hyperdrive`, `mirror`)
  and hooks (`modesOnRunStart`, `modesOnStageStart`, `modesShouldOpenShop`,
  `modesGetBaseMusicTheme`). Settings mode cycle reads/writes `settings.mode`.
  Achievements: `gauntlet_clear`, `hyper_survivor`, `mirror_master`.
- `galaxa-fx.js` retains capped particle/ring producers and `fxDrawBack`.
  Atlas explosions, impacts, shields and parries render through the common
  world renderer. Decorative effects precede `renderDangers`; HUD uses a
  fixed transform after camera shake. Do not restore removed overlay/ghost
  drawing passes or ordinary-hit hitstop. Respect particles, shake and
  reduced-motion settings. No child DOX file needed.
- `galaxa-enemy-fx.js` owns the enemy-side FX (telegraph glints, feint/shuffle
  rings, rage auras, kill chains, hit pops) via `GC.createEnemyFx`, drawn between
  `fxDrawWorld` and `renderDangers` and ticked from `updateExp`. Keep its pools
  bounded by `GC.FX_CAPS` and clear them outside `PLAYING`/`STAGE_INTRO`.
  No child DOX file needed.
- `writer.js`, `writer-session.js`, `writer-panels.js` - Native DOCX Autor,
  revision-aware persistence, document panels and isolated AI suggestions.
  `desktop-app-writer.css` owns chrome only; the engine owns document geometry.
  Public table merge/split operations live in `scripts/writer-table-ops.js`.
  Exposes `window.WriterApp`. No child DOX file needed.
- `pet-picker.js` - Pet catalog, scale/enabled/always-on-top settings, and
  ZIP import. Scale text uses `desktop.pet_scale_value`. Load,
  activate, settings, and import notifies use
  `desktop.request_failed`. Invalid ZIP names stay on
  `desktop.pet_import_invalid`. Exposes `window.PetPickerApp`.
  The companion shell runtime `core/pet-runtime.js` uses
  `desktop.pet_aria_label` for the fallback sprite label. No child
  DOX file needed.
- `radio.js` - Station browser and player. Click counts use
  `desktop.radio_compact_thousands` and `desktop.radio_compact_millions`.
  MediaSession title fallback uses `desktop.app_radio`; album uses
  `desktop.radio_album`. Catalog HTTP throws the sentinel
  `Radio Browser HTTP` without a status; `loadActive` and
  `searchStations` show `desktop.radio_catalog_error`. Playback
  `play()` and the player toggle show `desktop.radio_error` only
  and must not dump `err.message`. Radio `t` stays key-only.
  Exposes `window.RadioApp`. No child DOX file needed.
- `people.js` - Address-book app. KG toggle, active label, and card badge
  use `desktop.people_kg`. Content empty-state load failures use
  `desktop.load_failed` via `t(inst.context, key)`. Save and delete
  notifies use `desktop.request_failed`. Exposes
  `window.PeopleApp`. No child DOX file needed.
- `chess.js` / `chess-fx.js` - Chess app and board FX. Result-modal
  fallbacks use `desktop.chess_new_game` and `desktop.ok`; pass `t`
  into `createChessFx`. Opponent-move errors use
  `desktop.chess_*` codes via `formatOpponentError`. Vendor-load
  status and the missing-template fallback use
  `desktop.chess_load_failed`. Exposes
  `window.ChessApp` and `window.createChessFx`. No child DOX file
  needed.
- `calendar.js` / `calendar-views.js` / `calendar-editor.js` - Calendar
  session shell, pure view renderers and editor/peek continuations bundled
  inside the shared Desktop IIFE (see Local Contracts for order and behavior).
  Empty-state load failures use `desktop.load_failed`. No child DOX file
  needed.
- `looper.js` / `looper-monitor.js` - Desktop Looper: goal/work/evaluate/optional
  finish. Evaluate is tool-free. A broken review records score 0 and continues.
  Work recovers from `unexpected tool-call text` when the failed step already
  produced tool output; MeshCore still fail-closes on that error. Finish format
  errors keep the completed loop status. Verify
  `go test ./internal/agent ./internal/server -run 'Looper|MinimalLoop|ParseEvaluation'`.
  No child DOX file needed.
- `agent-chat.js` - Desktop Agent Chat. Missing-host throws reuse
  `desktop.load_failed` via `desktopText(key)` with no second
  argument. Loaded lazily. Exposes `window.AgentChatApp`. No child
  DOX file needed. Finish and release the active streaming bubble at each
  `tool_call`; its narration replaces that round's streamed draft instead of
  duplicating it. `final_response` replaces the current final draft. Pending
  scrolls target the newest log item, never an earlier round's bubble.
  Verify with `node scripts/test-ui-regressions.mjs`.
- `live-speech.js` - Desktop Live Speech. Missing-host throws reuse
  `desktop.load_failed` via `text(key)` with no fallback. Loaded
  lazily. Exposes `window.LiveSpeechApp`. No child DOX file needed.
- `galaxa-demo.js` - AI pilot and demo lifecycle; reactive combat AI (aim, fire,
  dodge, collect powerups), menu auto-tap for shop/evo, and game-over
  auto-restart loop. Attaches `ctx.startDemo()` and `ctx.updateDemo(dt)` via
  `GC.createDemo(ctx)`. Uses the `ctx.G.ai` input source merged in
  `galaxa-game.js` when `ctx.G.demoMode` is true. No child DOX file needed.
- `cheater.js` - Cheater app entry: library, editor, create modal, auto-save,
  polling, view-mode toggle. Exposes `window.CheaterApp`. Editor uses a stable
  `<textarea>` source and renders a separate live preview via marked,
  DOMPurify, and hljs. No child DOX file needed.
- `cheater-toolbar.js` - Markdown formatting toolbar (bold, italic, code,
  link, heading, lists, quote, divider) plus shortcut help modal. Mounts into
  the editor toolbar slot via `window.CheaterToolbar.mount(state, slot)`. No
  child DOX file needed.
- `cheater-spotlight.js` - Command-palette overlay with fuzzy search, keyboard
  navigation, delete confirmation, and create-from-query fallback. No child DOX
  file needed.
- `cheater-templates.js` - New-sheet templates (empty, deployment, debug,
  routine, API, backup) returning localized names via `cheater.template.*`
  keys. No child DOX file needed.
- `cheater-attachments.js` - Attachment upload/delete side panel with
  drag-and-drop, multipart `.txt`/`.md` uploads, backend-aligned 1 MiB and
  25,000-character validation, and 5-second undo. No child DOX file needed.
- `calculator.js` implements the Calculator app, a three-mode calculator
  (standard, scientific, programmer) with expression tokenizer/parser, context
  menu for clipboard operations, and window cleanup. Loaded lazily by
  `module-loader.js` as a standalone app. Exposes `window.CalculatorApp`.
  The programmer section starts hidden; mode tabs must fit the content width.
  Verify all three modes in the default and narrow windows with
  `TestDesktopCalculatorLayoutBrowser`.
- Settings icon themes include Tela, Candy, Fluent and Buuf; Tela/Fluent each persist their own folder color. The picker reads generated manifest palettes, with backend parity enforced by `TestDesktopAdditionalIconAssets`. Serialize settings saves so fast selections keep their order; failed saves restore the previous state. Theme/color/scheme changes update open decorative icons without replacing controls or focus.
- `settings.js` implements the Settings app, a virtual desktop configuration
  panel with sidebar navigation, global search, hamburger menu on mobile,
  and full desktop shell re-render on changes (icons, widgets, start menu,
  start button). Loaded lazily by `module-loader.js`. Exposes
  `window.SettingsApp`. Info and editable rows share inset spacing; bounded
  control columns must not let long provider/model names squeeze their labels.
  Keep full workspace paths wrappable and Info values at normal text weight.
  The theme picker is a card grid (`renderDesktopThemes`, `data-desktop-theme`)
  built from the `DESKTOP_THEMES` registry. A pick atomically saves the theme,
  matching icon family, wallpaper for all spaces (clearing per-space overrides),
  enabled theme cursors and matching sound bank in one settings PUT. Sound and
  motion master switches remain unchanged. A failed save preserves the complete
  previous selection. Later individual customizations survive reloads.
  Verify `TestDesktopExperienceThemeAtomicSelectionBrowser`.
  The light/dark row (`appearance.fruity_mode`)
  shows only for themes with `schemeSetting`, and the accent row only for
  `accent: 'user'`. Verify `TestDesktopThemeSweepBrowser`.
- `camera.js` implements the Camera app (device webcam): photo mode with
  optional self-timer countdown, mirror toggle and rule-of-thirds grid, and
  MediaRecorder-based video mode with a centered REC badge. Live video, photo
  preview and clip preview stack absolutely inside the viewport (never in-flow
  flex siblings — that squeezed each into one half); the `[hidden]` guard in
  `camera.css` must keep beating author display rules. Photo previews offer
  retake/save (`Pictures`)/download/clipboard-copy/send-to-agent and opening in
  Pixel; clip previews offer retake/save (`Videos`)/download and Video Studio
  handoff (see Camera above). Narrow action rows wrap controls and labels while
  retaining 44px targets. A session-only recents strip (max 12, data-URL thumbnails)
  re-opens captures. Camera network access is limited to `/api/desktop/upload`
  and `/api/desktop/chat/stream`; preview state does not persist across windows. `dispose()` must stop tracks, cancel countdown/record clocks,
  stop the recorder (discarding the clip), revoke the clip object URL and
  detach `devicechange`. Loaded lazily by `module-loader.js` and exposes
  `window.CameraApp { render, dispose }`. Visible strings use
  `desktop.camera_*` keys in all `ui/lang/desktop/*.json` locales. No child
  DOX file needed.
- `network-cameras.js` implements the Network Cameras app with a bounded
  snapshot grid, one selected live viewer, an optional four-stream live grid,
  administrator-only ONVIF/manual setup and stream management, and cleanup on
  minimize or close. It must use AuraGo viewer/thumbnail APIs only, must never
  receive or persist camera credentials, and stores only non-sensitive grid
  mode, selected-stream and non-sensitive main/substream role preferences. Loaded lazily by `module-loader.js` and
  exposes `window.NetworkCamerasApp { render, dispose, open }`. `network-cameras-media.js`
  owns shared media settings, role switching, diagnostics, incremental telemetry
  and deliberate push-to-talk. Load it before the app. Visible windows poll state
  every 5 seconds with at most one request; unchanged structures retain iframes,
  inputs and modal state. Revisions reload only that camera. Hold/release,
  pointer cancellation, blur, hiding and disposal stop microphone tracks, peer,
  heartbeat and the identity-bound server lease. Never start the microphone on a
  status update. Verify `TestDesktopNetworkCamerasMediaBrowser` in Standard/Fruity. An empty visible-ID
  set means no thumbnail requests; the no-`IntersectionObserver` fallback must
  explicitly mark all cards visible, and focus mode must stop grid polling.
  HTTP 202 mutation responses are saved partial successes: close the dialog,
  refresh state, and show the localized reconciliation warning. True save
  failures keep retryable manual sources and setup tokens in window memory.
  The third mode `detections` and the manage modal's Person detection tab are
  delegated to `window.NetworkCamerasDetection` (section "Network Cameras
  person detection"). Toasts go through `notify(state, message, type)`:
  `ctx.notify` needs `{title, message, type}`, never a bare string.
- `noisemaker.js` — Noisemaker shell (`window.NoisemakerApp = { render, dispose }`):
  workbench layout (create pane | splitter | library, player bar), prefs in
  `aurago.desktop.noisemaker.prefs`, API calls to `/api/desktop/noisemaker/*`
  (state, enhance, generate, tracks, PATCH/DELETE tracks/{id}), window +
  context menus, compact mode (< 860 px). Loads after the four sub-modules
  below (see `module-loader.js`). The shell is the only module that talks to
  the network; it must call only `/api/desktop/noisemaker/*` and never
  receives provider credentials. Capability gating (music disabled, no LLM,
  no cover AI, lyrics unsupported) is driven by `/api/desktop/noisemaker/state`;
  a disabled integration renders the onboarding card instead of the workbench.
  A failed state request renders a retryable connection error, never setup;
  background refresh failures preserve the current capabilities and workbench.
  Tracks are server-paginated (`limit`/`offset`/`q`/`favorites=1`, newest
  first); favorites are the `favorite` media tag toggled via PATCH. HTTP 200
  `{status:error}` track pages throw and leave the current list in place; toasts
  use the server message or `desktop.noisemaker_error_unknown` (never on
  `AbortError`). `needmore` runs `continueQueue`: enqueue remaining loaded
  tracks via `queueHas` (library order), else wait for a library load that is
  already running (`tracksInFlight`) and re-check, else fetch the next page;
  `cancelPendingAutoplay` only when nothing more can arrive. Toasts go through
  the shell's `toast()` helper, which builds the `{ title, message, type,
  appId }` payload `showDesktopNotification` expects. Compact windows (`is-compact`, < 860 px) apply the same narrow
  player, list-row and Now-Playing rules as the 720 px viewport fallback.
  Volume `change` events still save prefs; the menubar rebuilds only when
  shuffle, repeat, visualizer or muted change. Visible UI strings use
  `desktop.noisemaker_*` keys plus `desktop.app_noisemaker` in all
  `ui/lang/desktop/*.json` files.
- `noisemaker-menus.js` — `window.NoisemakerMenus = { windowMenus(m),
  trackContextItems(m, track), libraryContextItems(m), withCheckIcons }`;
  pure builders over a model `{ t, tFull, readonly, s, actions }`.
  `t(key, params, fallback)` expects full keys (`desktop.noisemaker_*`),
  `tFull` is an alias; all five modules pass full keys so the static i18n key
  lint covers them.
- `noisemaker-library.js` — `window.NoisemakerLibrary = { create(deps),
  formatDuration, formatDate }`; grid/list views, favorites filter, search,
  multi-select, load-more; emits `play, enqueue, favorite, delete, template,
  download, contextmenu, create, loadmore, search, filter, view, selection`.
- `noisemaker-player.js` — `window.NoisemakerPlayer = { create(deps) }`; single
  `<audio>`, queue with shuffle/repeat/autoplay, Web-Audio visualizer,
  Now-Playing view; exposes `queueHas` and `cancelPendingAutoplay`;
  `pendingAutoplay` clears on play/setQueue/clearQueue/stopAudio. Volume slider
  `input` updates audio/UI only; `change` and mute emit prefs. Visualizer
  availability is queryable at mount; Now Playing shares `.is-viz-off`.
  Emits `state, change, favorite, delete, template, download, expand,
  needmore, error, visualizer-unavailable`.
- `noisemaker-create.js` — `window.NoisemakerCreate = { create(deps), PRESETS }`;
  Simple/Custom modes, genre presets, AI enhance, local ACE-Step controls,
  progress + result card; the root carries the current mode as
  `data-nm-create-mode`; the segment buttons carry
  `data-nm-mode="simple|custom"`. Emits `generate, change, mode, play-result,
  show-in-library, new-song`.
  Read-only mode blocks generation and enhancement at both UI and action entry
  points. Validate retained local controls in both modes; show the cover toggle
  in both modes and required lyrics when the local model has no language model.
  Enhancement results may only replace the unchanged field of the latest
  request. `TestDesktopNoisemakerAuditBrowser` covers these flows and compact
  list geometry in Standard and Fruity themes.
- `editor-filemenu.js` implements `renderFiles`, file management helpers and the inline text
  editor with window menus (file, edit, agent, help). Fallback file-list
  empty-state load failures use `desktop.load_failed`. Bundled in the
  main shell bundle (`desktopMainParts` in `build-ui-bundles.js`) because
  it is referenced directly by the desktop foundation runtime.
  Failed text-file loads retain Retry/New/Open and block editing/saving the
  unloaded document. A 404 starts an empty buffer only for explicit creation;
  restored/opened files keep load intent through both shell entry points.
  Clear their persisted path only on confirmed 404s, retaining temporary failures
  for Retry. Verify `TestDesktopFileLoadRecoveryBrowser`.
- `planning-gallery-music.js` - Planner/todo, gallery and Webamp music.
  Bundled in the main shell. The date field shows the civil day and omits
  an unchanged `due_date`. Loads keep a generation and refresh every 60
  seconds while the window is visible. Read-only mode disables edits and
  shows `desktop.todo_readonly`. An empty filter uses `desktop.todo_empty`.
  The three columns stack on the todo frame's inline size, not the viewport.
  Mutation errors use `desktop.request_failed`. Todo and Gallery empty-state
  load failures use `desktop.load_failed`. Webamp unsupported-browser errors
  use `desktop.winamp_unsupported`; launcher `notifyError` maps the
  English sentinel and reuses `desktop.load_failed` for other
  load failures. Leave the Webamp skin unchanged. No child DOX
  file needed.
- `quickconnect-launchpad-chat.js` - Quick Connect (`renderQuickConnect`
  device list and session chrome), store/launchpad and generated-app
  host. The synthetic AuraGo host card uses `desktop.qc_aurago_host` and
  `desktop.qc_aurago_host_description`; device-list empty-state load
  failures use `desktop.load_failed`. Store terminal-preview load
  failures use `desktop.store_terminal_load_failed`. Generated-app
  host empty-state, store container-app frame errors, start toasts,
  and external-open notifications use `desktop.load_failed`. Bundled
  in the main shell. No child DOX file needed.
- `store-terminal-preview.js` - CommandCode console-plus-preview
  host. CommandCode stays visible above an initially hidden shell drawer.
  Terminal toggles reuse its live sessions; Plus adds a shell in the same
  container working directory (`/workspace`). Only tab close, explicit restart
  and window disposal close shell sockets. Clipboard, focus and status remain
  session-scoped; hiding restores CommandCode focus. Keep all 16 locale labels,
  narrow-window layout and `TestDesktopStoreTerminalDrawerBrowser` aligned.
  Frame empty-state and start-toast failures reuse
  `desktop.load_failed`. Stylesheet and script loads wrap
  AuraLazyAssets and fallback `onerror` with
  `desktop.store_terminal_load_failed` so the asset URL does not
  leak. Loaded lazily. Exposes
  `window.StoreTerminalPreviewApp`. No child DOX file needed.
- `sheets.js` - Native workbook host and lifecycle. Exposes
  `window.SheetsApp`. No child DOX file needed.
- `sheets-data.js` - Typed localized inputs, CSV preview/import, templates and
  native locale supplementation. Exposes `window.SheetsData`.
- `sheets-panels.js` - Format/data/chart/AI/search/navigation/print panels.
  Exposes `window.SheetsPanels`.
- `sheets-charts.js` - Chart.js overlay, sheet anchoring and undoable chart
  operations. Exposes `window.SheetsCharts`.
- `code-studio/core.js` - Code Studio core: state management, API client, path
  utilities, lifecycle (render/dispose), shell markup, toolbar, tabs, breadcrumbs,
  status bar, file operations, window menus. Zen-mode exit title reuses
  `codeStudio.exitZen`. Opens the shared IIFE. No child DOX
  file needed.
- `code-studio/sidebar.js` - File explorer: keyboard-navigable tree view,
  path crumbs and header tools, expand/collapse, drag & drop upload into the
  hovered folder, file actions (rename/delete/download), tree context menu,
  active-file highlight, activity bar. No child DOX file needed.
- `code-studio/editor.js` - CodeMirror and textarea editor views, linked-view
  sync for split panes, Ctrl+wheel zoom, syntax highlighting integration. No
  child DOX file needed.
- `code-studio/terminal.js` - Terminal sessions with xterm.js, WebSocket
  connection, multi-session management. Tab names and the xterm welcome
  line use `codeStudio.shell_n` plus `codeStudio.title`. No child DOX
  file needed.
- `code-studio/search.js` - Search-in-files panel with grep, result navigation.
  No child DOX file needed.
- `code-studio/agent.js` - Agent chat panel, SSE streaming, diff preview,
  code actions (explain/comments/tests/refactor), markdown rendering. No child
  DOX file needed.
- `code-studio/git.js` - Git panel: branch display, change list with
  checkboxes, diff view, selected-path commit, local identity fields, recent
  log. No child DOX file needed.
- `code-studio/panels.js` - Split editor (right/down) with two synced views of
  the active tab and a resizable divider. No child DOX file needed.
- `code-studio/shortcuts.js` - Keyboard shortcuts, shortcut overlay, command
  table, exposed API, `window.CodeStudioApp` assignment. Closes the shared
  IIFE. No child DOX file needed.
- `code-studio/command-palette.js` - Command palette overlay with fuzzy search
  over commands, open/recent/tree files, keyboard navigation; executes through
  `CodeStudioApp.command`. Separate IIFE. No child DOX file needed.
- `sysworld.js` - Per-window lifecycle, lazy ESM loading, visibility, menus,
  selection, bounded neighbourhood requests and the visibility-gated memory
  artifact feed for the hologram. Exposes `window.SysWorldApp`.
- `sysworld-data.js` - Shared read-only REST/SSE data lifecycle and stable entity
  records with source timestamp/status; no persistent history store.
- `sysworld-scene.js` - Isolated Three.js city renderer, GLB LOD cache,
  instancing, PBR/PMREM, shadows, `SceneCapturePass` + bloom + output +
  atmosphere post chain, camera modes and disposal. Build to
  `ui/js/vendor/system-world/`; never classic-script load this source.
- `sysworld-hud.js` - Theme-native, localized HTML metrics with CPU/RAM sparklines,
  district navigation with state badges, entity search, inspector, SVG plan map, street
  controls and compass, tour lower third, loading progress, photo mode and
  decluttered projected district labels.
- `sysworld-life.js` - Shared robot assets, street routes, hover lights and district
  status effects; imports only into the city bundle.
- `sysworld-navigation.js` - Pure robot steering: arc-length routes, lanes,
  obstacle circles, conflict prediction, yielding, U-turns and separation.
  Node-testable without Three.js.
- `sysworld-hologram.js` - Memory archive hologram: cone, rings, motes and
  canvas-text panels cycling sanitized artifacts.
- `sysworld-atmosphere.js` - Sky, aurora, stars, moon, sea, mist, dust, spire
  beacon, lamp cones and the vignette/grain post pass.
- `sysworld-drones.js` - Three patrolling service drones on closed spline loops.
- `sysworld-audio.js` - Gesture-unlocked opt-in ambient audio and shared master mixer.
- `sysworld-voice.js` - Transient TTS phrase playback, spatial echo/reverb and cancellation.
  These modules need no additional child DOX.
- `openscad-editor.js` - CodeMirror editor integration for SCAD source with
  syntax highlighting (using javascript()), error line highlighting, fallback
  textarea, and `revealLine(line)` for jumping to an issue. Exposes
  `window.OpenSCADEditor { create, parse, revealLine }`. The `parse` function
  extracts line-numbered errors from OpenSCAD stderr output. No child DOX
  file needed.
- `openscad-defines.js` - Parametric define slider panel: parses name=value
  pairs, renders numeric values as range sliders (with negative-value support)
  plus number inputs, text values as plain inputs, and per-row reset/remove
  buttons with an add-define control. Exposes
  `window.OpenSCADDefines { parse, render, toText }`. No child DOX file needed.
- `homepage-studio.js`, `homepage-studio-preview.js`,
  `homepage-studio-sites.js`, `homepage-studio-history.js` - Homepage Studio
  workbench: assistant chat with suggestion chips, preview-first viewport
  with device switcher and fullscreen, Sites inspector (drift badges,
  deployments, reconcile) and History inspector (search/filter/pagination,
  shell-dialog deletes). URL validation and the iframe sandbox contract stay
  pinned in `homepage-studio.js`. Local webhost name fallback uses
  `homepage_studio.default_name`; chat-stream failures and the
  missing parser throw reuse `desktop.chat_request_failed`. Exposes `window.HomepageStudioApp` plus
  `HomepageStudioPreview`/`HomepageStudioSites`/`HomepageStudioHistory`
  factories. No child DOX file needed.
- `log-viewer-filters.js` / `log-viewer.js` - Log Viewer: file sidebar,
  virtualized tail list, level/search filters, dedicated per-window
  EventSource to `/api/desktop/logs/stream`, readonly-gated download.
  Exposes `window.LogViewerFilters` then `window.LogViewerApp`. No child
  DOX file needed.
- `mailer-views.js` / `mailer.js` / `mailer-compose.js` / `mailer-settings.js` /
  `mailer-stickers.js` / `mailer-api.js` - Mailer: theme-native three-pane
  mail client (folder rail | message list | reader). `mailer-views.js` owns the
  pure rendering helpers (mail HTML sandbox, sender parsing, avatar tones,
  empty state, skeleton) and loads before the shell; `mailer.js` owns the
  window lifecycle, list, reader and `window.MailerApp`
  (`{ render, dispose, safeMailHTML }`). Styles live in
  `css/desktop-app-mailer.css` and resolve `--mail-*` from the `--vd-theme-*`
  bridge; the reader is an overlay pane below 719 px and the layout stays
  container driven. Sticker drafts keep their `cid:` contract. No child DOX
  file needed.
- `noisemaker-menus.js` - Noisemaker window/context menu builders. Exposes
  `window.NoisemakerMenus`; pure builders over `{ t, tFull, readonly, s,
  actions }`. Loads before `noisemaker.js`. No child DOX file needed.
- `noisemaker-library.js` - Noisemaker library grid/list, favorites, search,
  multi-select and pagination. Exposes
  `window.NoisemakerLibrary { create, formatDuration, formatDate }`. No child
  DOX file needed.
- `noisemaker-player.js` - Noisemaker player bar, queue, visualizer and Now
  Playing view. Exposes `window.NoisemakerPlayer { create }`. No child DOX
  file needed.
- `noisemaker-create.js` - Noisemaker song creation (Simple/Custom modes,
  presets, ACE-Step controls). Exposes
  `window.NoisemakerCreate { create, PRESETS }`. No child DOX file needed.
- `noisemaker.js` - Noisemaker workbench shell orchestrating create pane,
  library and player. Exposes `window.NoisemakerApp { render, dispose }`;
  loads after the four sub-modules. No child DOX file needed.
- `sip-phone.js` - iPhone-inspired SIP softphone: device chassis with glossy
  Dynamic Island and status bar, separate `.sip-phone-hw-*` hardware buttons,
  black screen bezel, aurora mesh wallpaper, `.sip-phone-glare` glass
  reflection, and ambient stage halo/floor shadow. Five tab views
  (Favorites, Recents, Contacts, Keypad, Settings) with glass tab bar; the
  Contacts tab renders the AuraGo address book (entries with phone/mobile,
  one dial row per number, client-side search filter, refresh on tab
  activation), active-call
  takeover with answer/decline for inbound ringing calls, contact-hue
  avatars, and frameless small-window fallback (hides hardware buttons,
  glare, and stage effects). Styling lives in
  `ui/css/desktop-app-sip-phone.css`. Exposes `window.SipPhoneApp`. No child
  DOX file needed.
- `sip-phone-gadget-runtime.js` (core, main bundle) - Floating phone gadget:
  mounts `sip-phone.js` windowless on a draggable body-level layer
  (`#vd-sip-phone-gadget`, scaled 400×830 stage, instance id
  `sip-phone-gadget`). Drag handles are the status bar, Dynamic Island,
  hardware buttons, and device frame; right-click opens a shell context menu
  (open in window, always on top, remove from desktop). Settings keys
  `phone_gadget.*` (defaults in `desktop-foundation.js`, toggle in the
  Settings app), layer CSS in `ui/css/desktop-sip-phone-shell.css`, gadget
  overrides in `ui/css/desktop-app-sip-phone.css`. Exposes
  `window.SipPhoneGadget { init, sync }`. The per-frame audio-visualization
  writer must skip gadget-hosted phones (the viz is `display:none` there and
  writes would invalidate the whole scaled subtree, visibly flickering the
  screen) and only write custom properties when a rounded level changed.
  Active-call texts (party name/URI, status, button and volume labels) must
  wrap inside the screen via `overflow-wrap` and never bleed past the device
  edges. No child DOX file needed.
- `zipper.js` - Zip archive browser: list/create/extract, desktop and host
  file drops, breadcrumb navigation inside an archive. Double-click, Enter, or
  File → Open preview a member without extracting the whole zip. Images, text,
  audio, and video render in an in-app overlay from
  `GET /api/desktop/archive/entry`. PDF, Markdown, and Office files open Viewer
  with `{ path, archiveEntry, forceNew: true }`; STL opens Viewer 3D the same
  way. Executables and other blocked types stay closed. Visible strings use
  `zipper.*` in all 16 `ui/lang/desktop/*.json` files. The open-dialog
  filter uses `desktop.file_dialog_zip`. Exposes
  `window.ZipperApp`. No child DOX file needed.
- `viewer.js` / `viewer-3d.js` - Viewer and STL viewer accept optional
  `archiveEntry` with the zip `path` and `forceNew: true`. Archive members
  load from `/api/desktop/viewer/content?path=&entry=` or
  `/api/desktop/archive/entry`; Viewer hides Edit for archive members.
  Missing markdown-it shows `viewer.error` only. Viewer 3D missing
  STLLoader throws and maps `viewer.error`. Missing print-frame
  errors throw `desktop.print_failed`; the print catch still
  prefixes `viewer.error`. No child DOX file needed.
- `teevee.js` - IPTV player; `teevee-catalog.js` owns catalog loading,
  stream identities, search and display labels. Catalog HTTP throws the
  sentinel `iptv-org HTTP` without a status. `fetchJSON` must
  not call `t()`. `loadCatalog` shows `desktop.teevee_catalog_error`.
  Loaded lazily after `teevee-crt.js` and `teevee-catalog.js` (which exposes
  `window.AuraTeeVeeCatalog`). Exposes `window.TeeVeeApp`. The real-shell
  `TestDesktopTeeVeeBrowser` covers video, HLS/AES, origin fallback, controls and
  lifecycle; opt in with `AURAGO_RUN_BROWSER_SMOKE=1`. No child DOX file needed.
- `game-maker-studio.js` - Game Maker Studio shell. The shell hosts
  `[data-gm-job-feed]` directly under `[data-gm-steps]`, outside both panes.
  `GameMakerStudioActivity` paints the phase, the shared elapsed time and the
  last eight public events, and hides the feed while `.gm-build-terminal` is
  connected. Assets and Export stay visible buttons. Sources, Knowledge,
  Revisions, Code Studio, Rename and Delete live only in `.gm-more`, which
  stays visible at every width. Knowledge is enabled only when a project is
  open and `window.AuraGoLearnings` exists. The sources item stays enabled.
  Export stays disabled until the open project has a revision. Missing
  skills/revisions modals throw `game_maker.modules_load_failed`
  via `state.context.t(key)`. Exposes `window.GameMakerStudioApp`.
  Its optional screenshot-review strip has an explicit, height-bounded grid row;
  status text and thumbnails must not squeeze the live game into a narrow band.
  Verify wide/narrow layout and stale-review cleanup with
  `TestGameMakerVisualStripLayoutBrowser` / `TestGameMakerManualVisualLifecycleBrowser`.
  Manual review renews its credential for the captured revision without replacing
  the frame or its save/channel grant; changed revisions and late responses fail
  closed. Replay and live job events may replace the current job only with a newer
  queued job; old terminal/preview events cannot finish a newer job. Project-only
  revision events stay valid. Verify `scripts/test-ui-regressions.mjs`.
  No child DOX file needed.
- `pixel*.js`, `pixel-lib-*.js` (incl. `pixel-worker.js`) - Pixel image editor, structure in `## Pixel`:
  shell and core, the DOM-free libraries (filters, generators, cutout, inpaint), the worker, and the feature
  modules for layers, selections, brush and retouch, transform, filters, generators, cutout, intelligence
  (command palette, content-aware fill, colour panel) and OpenRaster projects. The open-dialog filter uses
  `desktop.file_dialog_images`. Save-dialog filters use `desktop.file_dialog_png`, `desktop.file_dialog_jpeg`
  and `desktop.file_dialog_ora`. Image-decode failures throw `pixel.error_load`. Exposes `window.PixelApp`.
  No child DOX file needed.
- `terminal.js` - Standalone workspace terminal and Retro-Net dialer. Exactly
  one live socket per window: the shell (`/api/code-studio/terminal`) **or** a
  Retro-Net session (`/api/desktop/retronet/connect?entry=<id>&cols&rows`, in
  `terminal-retronet-session.js`; never a host or port). With `retronet_enabled`
  (bootstrap, updated by `aurago:desktop-policy`), not read-only and no `path`
  context, it opens on the directory (`terminal-retronet-directory.js`, drawn
  on the alternate screen with auto-wrap off; entry `00` = shell, last entry in
  `aurago.desktop.terminal.retronet.last`); otherwise, and after a directory
  403, it behaves as the plain shell. `Ctrl+]` (never forwarded to a service)
  and the toolbar return to the directory. Opening the directory, a dial or the
  shell calls `resetScreen()` (`term.reset()` plus a queued RIS that shows the
  cursor); a result keeps the session output and only restores the normal
  screen, modes and mouse reporting (`PLAIN_SCREEN`).
  Dialing runs the modem sequence and the optional baud throttle from
  `terminal-modem.js` (`aurago.desktop.terminal.baud`, default off) in parallel
  with the socket and shows `CONNECT` only after both finished; output before
  `CONNECT` is buffered up to the throttle's queue limit
  (`TerminalModem.MAX_QUEUE_BYTES`, 1 MiB), more skips the animation; results print
  the Hayes code plus `desktop.terminal_retronet_result_<reason>` (also the
  browser-only `local_hangup` and `lost`), and keys in the first 400 ms after a
  result are ignored. Screen geometry lives in `terminal-geometry.js`
  (`window.TerminalGeometry`: `MODES`, `create` → `set(mode)`/`mode`/`refit`/`dispose`;
  modes `fit`, `vt`, `bbs`, `c64`; terminal.js switches only through `setGeometry(next)`).
  The directory and the shell use `fit`. Every Retro-Net session gets a pinned classic screen:
  `bbs` entries 80×25, `c64` entries 40×25, Telnet `world` entries and all SSH entries (catalog
  and own) the 80×24 VT screen `vt`. The grid is pinned synchronously, so the session URL
  carries it (the server's first NAWS and SSH PTY size) and no `resize` frame ever follows
  (`TerminalRetroNetSession` has no resize); `onTermResize` only redraws the directory.
  `bbs` entries use a fixed 80×25 grid with the vendored `Aura VGA` font
  (Px437 IBM VGA 8x16, CC BY-SA 4.0, `ui/fonts/Px437-LICENSE.txt`), `c64`
  entries 40×25 with the vendored `Aura C64` font (C64 Pro Mono by Style,
  `ui/fonts/C64_Pro_Mono-STYLE.woff`, unmodified under its original name,
  licence and hashes in `ui/fonts/C64_Pro_Mono-LICENSE.txt`; the licence allows
  `@font-face` embedding only in software provided to end users free of
  charge, so never link or offer the
  `.woff` as a download); both fonts are credited in `THIRD_PARTY_LIBRARIES.md`.
  Pinned grids set `data-terminal-geometry`, wait for their font (then change
  the size once so xterm measures the loaded font) before the first fit and
  centre the grid with letterbox padding inside a glass border. Retro styles
  give the `.xterm` `clamp(16px, 3%, 30px)`, which CSS resolves against the
  width on every side (the shell keeps it); `vt` and `bbs` use a per-axis
  border instead: `GLASS_X` 2 % of the glass width left and right, `GLASS_Y`
  3 % of its height top and bottom, `GLASS_MIN` 12 px at the least, never more
  than the style's padding and none in modern. It keeps the grid's corners
  inside the glass's rounded corners (an ellipse of 5 % × 7 %) and the CRT's
  1.8 % edge fade. `c64` keeps the style's padding. `bbs` and `c64` keep line
  height 1 (VGA blocks come from the font) and no scrollback. `bbs` takes the
  largest size in 0.5 px steps, down from the size its em cell allows, that
  fits the glass. `c64` (`pixels: 8`) scales
  by whole device pixels: font size 8k/DPR for the largest k whose grid fits,
  so a C64 pixel is k device px. The WebGL renderer floors the measured
  advance in device px: a width one px short gets `letterSpacing` 1 (the DOM
  renderer after a lost WebGL context does not floor; its refit drops it
  again). The rounded font box (ceiled) is never corrected: `lineHeight`
  stays 1, any height mismatch tries k−1. At DPR 2 odd k always fails (cells
  of 42/26 px), so a 960×720 window gets a 640×400 C64 screen at DPR 1 and 2;
  with no exact k (tiny glass; DPR 1.25 below 24 px, DPR 1.5 below 24 px
  cells) it falls back to the 0.5 px steps. Sizes come from
  `offsetWidth`/`offsetHeight` (window animations transform the box); a refit
  at the same k keeps the spacing, so it changes no xterm option.
  `c64` uses xterm's alpha glyph atlas (`allowTransparency`; the opaque atlas
  antialiases for LCD and a glyph's fringe overwrites the previous cell's edge
  column) and refits on a DPR change (`matchMedia('(resolution: …dppx)')`).
  Known limitation: the browser's greyscale antialiasing (seen in Chrome on
  Windows, also on a plain canvas) softens vertical glyph edges with a faint
  one-px halo on each side; block art stays seamless.
  `vt` (classic services assume a VT100; towel.blinkenlights.nl draws 19-row
  frames that a shorter screen scrolls into stale rows) has no font of its
  own: it takes the style's font family (loaded at the style's size; the
  style's line height is not used), keeps the 1000-line scrollback (MUD
  players read back) with xterm's 14 px scrollbar in a strip right of the
  grid, taken from the right letterbox where that is wide enough (the grid
  stays centred in the glass; a narrower letterbox moves the grid left by the
  difference), and its letterbox shows the style's background (xterm's
  viewport, or the CRT's theme background). Font: the largest 0.5 px step whose grid fits
  at line height 1, i.e. whose 80 columns fit the width, or whose rows fit the
  height where they do not at line height 1 (a wide, short glass: the sides
  take the rest; the 960×720 window and a maximized one are such glass for
  Share Tech Mono). The search starts at the current size and moves to the
  size the measured cells scale to, between the largest size that fit and the
  smallest that did not; after a fit it tries one step more (whole device px
  cells keep their size over some 0.5 px steps). `MIN_FONT_SIZE` 6 at the
  least, clipped below that. The search measures at the current line height,
  so rows do not collapse and spread again on screen: xterm's cell is
  `floor(base × lineHeight)` device px, so the font's box `base` is
  `ceil(cell / lineHeight)`. Rows: then spaced like a CRT raster to fill the
  height: the largest device px cell height whose 24 rows fit, as
  `lineHeight = (cell + 0.5) / base` (xterm floors the product), at most
  `MAX_LINE_HEIGHT` 2, where a tall glass keeps a letterbox above and below;
  a height xterm does not give is retried a px lower, twice, else line height
  1 stays. Only while the WebGL addon is loaded (`canSpace`, `!!canvasAddon`
  in terminal.js): its custom glyphs (`customGlyphs`, default on) draw block
  elements and box drawing over the whole spaced cell, so they still join
  (text glyphs sit centred in the cell). xterm's DOM renderer (modern style,
  a lost WebGL context) draws them from the font, so `vt` keeps line height 1
  there; the context-loss refit drops the spacing. `ceiling` keeps the last
  fit (glass, font, spacing allowed, size, line height, grid): an unchanged
  glass whose cells measure the same takes one probe and writes nothing
  (every option written re-measures and rebuilds the WebGL atlas). A resize or
  style change changes font and line height, never 80×24; the style select
  stays enabled and a new style re-pins with its font. xterm 6.0 syncs its
  viewport (scrollbar, scroll position) only on buffer resize and scroll
  events, so after a fit that changed the cell the scroll position would stay
  in the old cell height and the next wheel tick would jump by
  `ydisp × (1 − old / new)` lines: the geometry re-syncs it the way xterm does
  after a buffer switch (`term._core._viewport`: `_latestYDisp` cleared, then
  `queueSync()`; private, guarded).
  `fit` uses the fit addon at the style's font. A pinned grid stays through
  the result; `set('fit')` (directory, shell, policy switch, directory 403)
  restores the style's font, size and line height, which are never changed or
  saved. The directory redraws for any grid. Retro sessions set
  `convertEol=false`, the shell restores `true`.
  C64 calls put on the `commodore64` style with `persist=false`
  (never saved; dispose drops it), lock the style select (disabled, `title`
  and an `aria-describedby` screen-reader note with
  `desktop.terminal_retronet_style_locked`), switch to a blinking block cursor
  on a black screen and paint the letterbox in the C64 border colour `#706DEB`.
  The look stays through the result and is left only through `setGeometry()`
  (directory, shell, policy switch, directory 403), which restores the saved
  style, cursor and select. `C64_FRESH_SCREEN` (SGR reset, clear, cursor to row
  25) goes out right before the first board byte, matching the server's PETSCII
  model; the coordinator writes nothing during the call. After CONNECT the
  Telnet notice and the C64 key hint (`TerminalText.wrapHints`) stay for
  `C64_HINT_HOLD_MS` (2 s) while board output waits; a key (colour keys too,
  forwarded) or a full queue ends the hold early, a local hang-up drops the
  held output and a remote end shows it before the result. Colour keys
  (`attachCustomKeyEventHandler`, live C64 calls only): Ctrl+Shift+1–8,
  Alt+Shift+1–8 and Ctrl+Shift+9/0 by `event.code` send `ESC[64;n~`
  (`TerminalRetroNetSession.c64ColorKey`), prevent the browser default and never
  reach xterm. BEL in a C64 call beeps through `TerminalAudio.beep` under the
  key-click rules, at most one tone per 150 ms and only on a running
  AudioContext. The gap counts from the tone itself: the first BEL creates
  the context, whose constructor can block the page for 100-200 ms, and BELs
  parsed behind that stall add no tone. The directory marks `c64` entries `[C]` (never checked) and
  explains the marker in its help when one is listed.
  `world` entries edit lines locally while `echo.remote` is false and suppress
  local echo and history while `echo.hidden` is true. Input leaves in UTF-8
  frames of at most 16 KiB (server read limit 64 KiB). The host-key prompt
  takes the localized letters first, then ASCII Y/N (Greek uses Latin Y/N).
  Admins manage own entries in `terminal-retronet-entries.js` (modal,
  `retronet.entries` setting): it re-reads the directory and merges before
  saving, and also refuses `localhost` names; type `c64` always stores charset
  `petscii` and hides the charset row. `terminal-text.js`
  (`window.TerminalText`: `cellWidth`, `cells`, `fitToCells(value, width, pad)`,
  `printable`, `sentences(parts)`, `wrapHints(value, width)`,
  `wrapWords(value, width)`) is the single display-width source (vendored
  xterm's Unicode 6 table); `printable` strips control characters
  from API-provided and translated strings (names, descriptions, fingerprints, host,
  result text) before the coordinator or directory writes them. The coordinator's dim
  lines (Telnet notice, results) wrap between words at `term.cols - 1` through
  `wrapWords` (never cut); key hints pack through `wrapHints`; the CONNECT
  announcement joins its parts through `sentences` (a full stop only where
  sentence punctuation, CJK included, is missing). The coordinator, directory
  and entry editor call `t(key)` without params and fill `{{name}}`
  themselves (the shared `t()` uses `replaceAll`, where `$&`/`` $` ``/`$'`
  in names would be patterns). The service's own byte
  stream passes unchanged so ANSI works. Modem sounds follow the key-click
  rules. Style catalog in `terminal-styles.js`
  (`window.TerminalStyles`: `ids`, `normalize`, `load`, `save`, `profile`,
  `applyXterm`, `effectControls`, `loadEffects`, `saveEffects`, `resetEffects`).
  IDs: `modern`, `amber`, `green`, `apple2`, `commodore64`,
  `ibm3278`, `vintage`, `mono-green`, `transparent-green`, `color-monitor`.
  Persist `aurago.desktop.terminal.style` and audio mute
  `aurago.desktop.terminal.audioMuted`. Retro styles use vendored
  xterm 6 with its WebGL addon, original WebGL CRT in `terminal-crt.js`
  (`window.TerminalCrt.create` → `setProfile`/`setEnabled`/`resize`/`dispose`/`usesFallback`;
  captures every `.xterm-screen` canvas at its CSS offset and scale;
  output is capped at DPR 1.25 and 30 fps, never stretches text to fill the tube),
  CSS bezels, and Web Audio key-clicks in `terminal-audio.js`
  (`window.TerminalAudio.create` → `setProfile`/`setMuted`/`playKey`/`beep`/`dispose`).
  Load order (`module-loader.js`, mirrored by the browser fixtures): xterm.css,
  desktop-app-terminal.css, then the 13 scripts `xterm.min.js`,
  `xterm-addon-fit.min.js`, `xterm-addon-webgl.min.js`, `terminal-styles.js`,
  `terminal-crt.js`, `terminal-audio.js`, `terminal-text.js`,
  `terminal-geometry.js`, `terminal-modem.js`, `terminal-retronet-directory.js`,
  `terminal-retronet-session.js`, `terminal-retronet-entries.js`, `terminal.js`.
  The baud throttle (`TerminalModem.createThrottle`) counts units: one per code
  point outside escapes and one per complete escape sequence; a synchronized
  block `ESC[?2026h`...`ESC[?2026l` (the PETSCII repaints) is one unit; an
  unterminated block ends as one unit once it holds 64 KiB, or 1 s after the
  throttle stalled on it; the queue is scanned incrementally (each byte once)
  and CAN/SUB cancel an escape. Scope is this app only. Reduced motion
  and `dataset.animations === 'false'` disable flicker, burn-in, animated grain, and audio.
  The native Effects dialog applies bounded sliders to the existing renderer,
  without recreating xterm or its socket. Preferences live per style in
  `aurago.desktop.terminal.effects.v1`; malformed/blocked storage falls back
  to presets, and reset affects only the selected style. Overall intensity zero
  restores unwarped source output and removes reflection. Brightness, bloom,
  scanlines, curvature, afterglow, phosphor mask, vignette, glass reflection,
  noise, flicker, jitter, rolling interference and color fringing are independent.
  Reduced motion also pauses jitter/interference; static adjustments remain.
  Glass reflection is a pointer-transparent CSS layer, available in fallback.
  CSS fallback supports intensity/brightness/bloom/scanlines/vignette/reflection;
  the dialog disables unsupported effects and explains the limitation. Dispose
  closes the dialog and releases its media-query/mutation observers. Zero burn
  removes temporal persistence even when instantaneous bloom remains enabled.
  Retro appearance follows cool-retro-term's luminous phosphor, scanlines,
  subtly curved glass and recessed bezel using original rendering code. Keep
  profile curvature gentle so text rows remain nearly straight. Share Tech
  Mono is embedded as `Aura Terminal`; pixel profiles retain Press Start 2P.
  Additive bloom and decaying persistence share a half-resolution blurred
  source buffer; never feed warped output back into the source. The native
  xterm layer stays interactive and is visually hidden only after a WebGL frame.
  Housing materials, seams, vents and localized wear live in the app CSS;
  older Apple II/Vintage cases show more wear. Decorative hardware is hidden
  from accessibility and pointer input. Its LED follows the existing localized
  socket status via `data-terminal-state`; Modern stays frameless. Keep compact
  cases inside the app without changing xterm's measured screen padding.
  Browser verification is
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopTerminalRetro(Net(C64)?)?Browser' -count=1 -v`
  (styles/CRT, Retro-Net and the C64 call, each in both motion modes; the
  Retro-Net run also checks the 80×24 VT screen: 80×24 in the session URL,
  rows filling at least 95 % of the height inside the glass border at 960×720
  and 1600×994, a tall 600×840 window taking its font from the width with
  spaced rows, a pixel readback of full and half blocks, a vertical and a
  horizontal line across spaced rows without a gap (also at DPR 1.25), line
  height 1 with rows edge to edge after a lost WebGL context and in the modern
  style, one wheel tick in the scrollback moving as far after a refit as
  before, a larger font in the larger window, towel-style 19-row frames
  without stale rows, a style change, no resize frame and the style font after
  leaving; the C64
  grid also at DPR 1.25 and 1.5, each with a pixel readback of half-block art
  from xterm's WebGL canvas that fails on seams; all use
  `ui/desktop_terminal_harness_test.go`, the Retro-Net fixture is
  `ui/testdata/terminal-retronet-fixture.js` plus
  `ui/testdata/terminal-retronet-c64-fixture.js`, which keeps the C64 hint
  hold's timer callback for manual release so hold checks need no deadline).
  Retro-Net module behaviour: `npm run test:terminal-retronet` (five Node
  scripts: modem, geometry, directory, session, entries; also run by
  `TestDesktopTerminalBehaviourScripts`). Static contracts, the C64 font and
  the Node scripts: `go test ./ui -run TestDesktopTerminal -count=1` (the
  browser tests skip without `AURAGO_RUN_BROWSER_SMOKE=1`).
  WebGL/canvas failure uses CSS fallback and keeps the WebSocket. Style
  changes wait for `document.fonts.load` before changing xterm options or
  creating the canvas renderer, then fit once. Ignore stale font completions
  after another style selection or disposal. Exposes
  `window.TerminalApp = { render, dispose }` with a per-window instances Map.
  The standalone window opens at 960x720 (4:3 CRT framing), scaling both axes
  together on smaller desktops; saved session bounds, user resizing and mobile
  maximization remain unchanged. Browser coverage delays the `commodore64`
  style's pixel font (Press Start 2P) on first load and checks its measured
  glyph width as well as constrained opening sizes.
  Visible strings use `desktop.terminal_*` plus `desktop.terminal_style*`,
  `desktop.terminal_audio*` and `desktop.terminal_retronet_*` (including one
  description per catalog entry) in all 16 desktop locales. Package contracts:
  `internal/retronet/AGENTS.md`; operator docs: `documentation/retro-net.md`.
  No child DOX file needed.
- Notes uses the compact Autor-style shell with library, outline/info/search/AI panels,
  global Fruity menus and a responsive flowing note surface. Markdown stays authoritative
  under Documents/Notes. notes.js owns each window, versioned /api/desktop/notes I/O,
  explicit IndexedDB recovery and the shared OfficeSession close/save lifecycle.
- notes-editor.js uses local Milkdown/Crepe/Kit 7.22.3 (MIT), public ProseMirror APIs
  and the existing CodeMirror bundle. Rich editing is default; unsupported syntax and
  notes above 200k characters remain in source mode. notes-frontmatter.js preserves
  unknown fields and line endings while updating tags. No obsolete NotesList or
  NotesToolbar runtime remains. Load order: frontmatter, writer-session, editor, entry.
- The notes.meta.json sidecar keeps version/pinned/sort/last_note. Full-text search is
  server-side, shared with desktop_notes; no browser 500-file index. Relative attachments
  and link destinations survive moves; user trash preserves original folder paths.
  Attachment uploads use `If-None-Match: *` even with unique filenames.
  A missing sidecar returns 404 through the real Notes handler so first use can
  create it conditionally; wrapped filesystem errors must retain that status.
- Agent Notes access is list/search/read/create only in native APIs, including
  agent-created notes. Permitted local execution follows the sandbox setting;
  disabled isolation or explicit unsafe fallback can bypass native file guards.
  Active isolation still checks Notes write-path overlap. See documentation/desktop-notes.md.
- Keep .vd-notes-app and .vd-notes-toolbar for the common theme bridge. Use the shared
  icon renderer and notes/writer/common translations in all 16 Desktop locales.
  Validate TestDesktopNotesAppBrowser, the AURAGO_NOTES_MATRIX shell fixture, vendor
  --check and OfficeSession tests. The API, permission and packaging contracts are in
  documentation/desktop-notes.md. No child DOX file needed.

## Tuya camera setup
- `js/go2rtc-tuya.js` owns the shared account/setup dialog used by Config and Network Cameras; load it before the camera modules with `css/go2rtc-tuya.css`. Strings use `common.go2rtc_tuya.*` in all 16 common locales. Config adopts committed account/camera metadata without replacing other drafts.
- Entered credentials exist only in dialog drafts until save or disposal. Discovery selections are opaque; account checks test saved inputs explicitly. Probe each selected HD/SD quality; failed HD offers an explicit SD choice. Cancel/close/window disposal abort requests, drop late results and clear drafts.
- Tuya role settings hide RTSP transport/manual source controls. Talk is disabled without a confirmed source backchannel; reconcile `source_type` and `talk_supported` changes in the toolbar, stopping active talk if capability disappears. State refresh never recreates a microphone or unchanged viewer. Verify `TestGo2RTCTuya*Browser`, existing media browser smokes and `node scripts/test-ui-regressions.mjs`.

## Camera source types
- Shared module `js/go2rtc-source-form.js` + `css/go2rtc-source-form.css`, loaded by Network Cameras (module-loader) and the config page (`config.html`). Strings use `common.go2rtc_source.*` in all 16 common locales; the catalog comes from `GET /api/go2rtc/source-types` (`internal/config/go2rtc_source_types.go`).
- Secrets and built URLs live only in memory (form state) and are restored into inputs via DOM properties after redraws; they never appear in markup, attributes, localStorage or preferences. Tuya stays in its own dialog.
- Test: `node scripts/test-ui-regressions.mjs` ("go2rtc source form builds URLs").

### Homepage Studio managed website evidence

- Render locations, provenance, check timestamps and grouped automatic reasons from the shared managed-site model. Escape all ledger strings and retain the safe URL filter.
- Stored URLs do not prove liveness; render explicit target state. Keep Homepage labels translated in all sixteen Desktop locales.
- Verify Homepage source/locale tests and the deterministic browser timeline fixture.
- Notifications: a published revision, `no_changes`, a question and a failed run arrive as
  a Desktop notification with `appId: "detective"` and `context.case_id`. The shell's
  `openApp` calls `window.DetectiveApp.open(existing.id, context)` (window-shell-runtime.js,
  main bundle) and `selectCase` loads that case, resets the archive and opens the report
  tab; a fresh window takes `ctx.context.case_id`/`ctx.launch.case_id`. Test:
  `TestDesktopDetectiveOpensTheNotifiedCase`.- Accessibility: the case title is the view's `h1`, the report title `h2` and report
  blocks `h3`. Tabs are a real `role="tablist"` with `role="tab"`, `aria-selected` and
  `aria-controls="dt-panel"`; the content area is the single `role="tabpanel"`. Only the
  status/phase line (`<div aria-live="polite">`) is a live region, not the whole status
  section. Deletion requires `ctx.confirmDialog`; there is no `window.confirm` fallback.
  The built-in icon stays `search`: it already denotes research, and a distinct glyph
  would need new artwork in both icon families for a cosmetic change (audit 4.23 kept
  the shared key deliberately). `TestDesktopDetectiveAccessibilityMarkers` pins all of it.
